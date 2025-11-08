package watchers

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/kagenti/kkbase/pkg/graph"
	"github.com/kagenti/kkbase/pkg/models"
	"go.uber.org/zap"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

// ResourceWatcher defines the interface for watching Kubernetes resources
type ResourceWatcher interface {
	// HandleAdd processes a newly added resource
	HandleAdd(obj interface{})

	// HandleUpdate processes an updated resource
	HandleUpdate(oldObj, newObj interface{})

	// HandleDelete processes a deleted resource
	HandleDelete(obj interface{})

	// Start starts the informer for this watcher
	// Returns true if the informer was started, false if already running or not applicable
	Start(ctx context.Context) bool
}

// ResourceTypeInfo contains metadata about a resource type
type ResourceTypeInfo struct {
	NodeType      models.NodeType
	Kind          string
	APIGroup      string
	ClusterScoped bool
}

// Config holds configuration for watchers
type Config struct {
	Clientset    *kubernetes.Clientset
	GraphStore   graph.GraphStore
	Logger       *zap.Logger
	ResyncPeriod time.Duration
	Namespace    string // Empty string for all namespaces
}

// deriveHandlerName derives a unique handler name from ResourceTypeInfo
func deriveHandlerName(typeInfo ResourceTypeInfo) string {
	if typeInfo.APIGroup == "" {
		return strings.ToLower(typeInfo.Kind)
	}
	return strings.ToLower(typeInfo.APIGroup + "/" + typeInfo.Kind)
}

// Manager manages multiple resource watchers with dynamic client support
type Manager struct {
	config        Config
	restConfig    *rest.Config
	dynamicClient dynamic.Interface
	factory       dynamicinformer.DynamicSharedInformerFactory
	crdWatcher    *CRDWatcher
	logger        *zap.Logger

	// Track registered handlers
	handlers   map[string]ResourceWatcher
	handlersMu sync.RWMutex

	// Track handlers registered before Start() - these have informers started by factory
	coreHandlers map[string]bool

	started bool
	ctx     context.Context // Store the context for starting dynamic informers
}

// NewManager creates a new watcher manager with dynamic client support
func NewManager(config Config, restConfig *rest.Config) (*Manager, error) {
	if config.ResyncPeriod == 0 {
		config.ResyncPeriod = 30 * time.Second
	}

	// Create dynamic client
	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	// Create dynamic informer factory
	var factory dynamicinformer.DynamicSharedInformerFactory
	if config.Namespace == "" {
		factory = dynamicinformer.NewDynamicSharedInformerFactory(dynamicClient, config.ResyncPeriod)
	} else {
		factory = dynamicinformer.NewFilteredDynamicSharedInformerFactory(
			dynamicClient,
			config.ResyncPeriod,
			config.Namespace,
			nil,
		)
	}

	// Create CRD watcher
	crdWatcher := NewCRDWatcher(restConfig, dynamicClient, factory, config.Logger)

	return &Manager{
		config:        config,
		restConfig:    restConfig,
		dynamicClient: dynamicClient,
		factory:       factory,
		crdWatcher:    crdWatcher,
		logger:        config.Logger,
		handlers:      make(map[string]ResourceWatcher),
		coreHandlers:  make(map[string]bool),
		started:       false,
	}, nil
}

// RegisterHandler registers a resource handler with type metadata
// Used for core resources that are always available (no CRD detection needed)
func (m *Manager) RegisterHandler(typeInfo ResourceTypeInfo, handler ResourceWatcher) {
	m.handlersMu.Lock()
	defer m.handlersMu.Unlock()

	// Register the NodeType metadata
	models.RegisterNodeType(models.NodeTypeMetadata{
		Type:          typeInfo.NodeType,
		ClusterScoped: typeInfo.ClusterScoped,
		Kind:          typeInfo.Kind,
		APIGroup:      typeInfo.APIGroup,
	})

	// Derive handler name from type info
	name := deriveHandlerName(typeInfo)

	m.handlers[name] = handler

	// Track if this is a core handler (registered before Start())
	if !m.started {
		m.coreHandlers[name] = true
	}

	m.logger.Info("registered handler",
		zap.String("name", name),
		zap.String("kind", typeInfo.Kind),
		zap.String("node_type", string(typeInfo.NodeType)),
		zap.Bool("cluster_scoped", typeInfo.ClusterScoped),
	)
}

// UnregisterHandler removes a resource handler
func (m *Manager) UnregisterHandler(name string) {
	m.handlersMu.Lock()
	defer m.handlersMu.Unlock()

	delete(m.handlers, name)
	m.logger.Info("unregistered handler", zap.String("name", name))
}

// registerHandlerOnly is an internal method that registers only the handler
// without registering type metadata (used when type is already registered)
func (m *Manager) registerHandlerOnly(name string, handler ResourceWatcher) {
	m.handlersMu.Lock()
	defer m.handlersMu.Unlock()

	m.handlers[name] = handler

	// Track if this is a core handler
	if !m.started {
		m.coreHandlers[name] = true
	}
}

// RegisterHandlerFactory registers a handler factory for CRD-based resources
// The handler will be created when the CRD becomes available
// The factory function receives CRDInfo (including schema and version) to enable version-agnostic handlers
func (m *Manager) RegisterHandlerFactory(
	typeInfo ResourceTypeInfo,
	factory func(crdInfo *CRDInfo) ResourceWatcher,
) {
	// Register the NodeType metadata immediately (even before CRD is available)
	models.RegisterNodeType(models.NodeTypeMetadata{
		Type:          typeInfo.NodeType,
		ClusterScoped: typeInfo.ClusterScoped,
		Kind:          typeInfo.Kind,
		APIGroup:      typeInfo.APIGroup,
	})

	// Derive handler name
	name := deriveHandlerName(typeInfo)

	// Watch for the CRD
	m.crdWatcher.WatchCRD(typeInfo.APIGroup, typeInfo.Kind, func(crd *CRDInfo) {
		// Check if handler is already registered (prevent duplicates)
		m.handlersMu.RLock()
		_, exists := m.handlers[name]
		m.handlersMu.RUnlock()

		if exists {
			m.logger.Debug("handler already registered, skipping",
				zap.String("handler", name),
				zap.String("group", crd.Group),
				zap.String("kind", crd.Kind),
			)
			return
		}

		m.logger.Info("creating handler for CRD",
			zap.String("handler", name),
			zap.String("group", crd.Group),
			zap.String("kind", crd.Kind),
			zap.String("node_type", string(typeInfo.NodeType)),
			zap.String("version", crd.Version),
		)

		// Create the handler, passing CRDInfo with schema and version
		handler := factory(crd)

		if handler == nil {
			m.logger.Warn("handler factory returned nil - version not supported or validation failed",
				zap.String("handler", name),
				zap.String("version", crd.Version),
			)
			return
		}

		// Register it (without type info since we already registered it above)
		m.registerHandlerOnly(name, handler)

		// If manager is already started, manually start the informer
		if m.started {
			m.logger.Info("handler registered dynamically (runtime), starting informer",
				zap.String("handler", name),
			)

			// Start the handler's informer
			if started := handler.Start(m.ctx); started {
				m.logger.Info("dynamic handler informer started and synced",
					zap.String("handler", name),
				)
			} else {
				m.logger.Warn("failed to start dynamic handler informer",
					zap.String("handler", name),
				)
			}
		}
	}, nil)

	m.logger.Info("registered handler factory",
		zap.String("name", name),
		zap.String("group", typeInfo.APIGroup),
		zap.String("kind", typeInfo.Kind),
		zap.String("node_type", string(typeInfo.NodeType)),
		zap.Bool("cluster_scoped", typeInfo.ClusterScoped),
	)
}

// Start starts the manager and all registered handlers
func (m *Manager) Start(ctx context.Context) error {
	m.handlersMu.RLock()
	handlerCount := len(m.handlers)
	m.handlersMu.RUnlock()

	// Store context for dynamic handler registration
	m.ctx = ctx

	m.logger.Info("starting dynamic watcher manager", zap.Int("handler_count", handlerCount))

	// Start CRD watcher first (it will watch for CRDs and trigger handler registration)
	if err := m.crdWatcher.Start(ctx); err != nil {
		return fmt.Errorf("failed to start CRD watcher: %w", err)
	}

	// Start the dynamic informer factory
	m.factory.Start(ctx.Done())

	// Wait for caches to sync
	m.logger.Info("waiting for caches to sync")
	synced := m.factory.WaitForCacheSync(ctx.Done())

	// Check if all expected informers synced
	syncedCount := 0
	for gvr, isSynced := range synced {
		if isSynced {
			syncedCount++
		} else {
			m.logger.Warn("failed to sync cache for resource", zap.String("gvr", gvr.String()))
		}
	}

	m.logger.Info("caches synced",
		zap.Int("synced_count", syncedCount),
		zap.Int("handler_count", handlerCount),
	)

	m.started = true

	// Start informers for any handlers that were registered during cache sync
	// These are extension handlers (not core) that got registered while waiting for sync
	m.handlersMu.RLock()
	currentHandlerCount := len(m.handlers)
	extensionHandlers := make(map[string]ResourceWatcher)
	for name, handler := range m.handlers {
		if !m.coreHandlers[name] {
			extensionHandlers[name] = handler
		}
	}
	m.handlersMu.RUnlock()

	if len(extensionHandlers) > 0 {
		m.logger.Info("starting informers for extension handlers registered during sync",
			zap.Int("core_handlers", len(m.coreHandlers)),
			zap.Int("total_handlers", currentHandlerCount),
			zap.Int("extension_handlers", len(extensionHandlers)),
		)

		for name, handler := range extensionHandlers {
			m.logger.Info("starting informer for extension handler",
				zap.String("handler", name),
			)

			if started := handler.Start(m.ctx); started {
				m.logger.Info("extension handler informer started and synced",
					zap.String("handler", name),
				)
			} else {
				m.logger.Warn("failed to start extension handler informer",
					zap.String("handler", name),
				)
			}
		}
	}

	return nil
}

// GetFactory returns the dynamic informer factory
func (m *Manager) GetFactory() dynamicinformer.DynamicSharedInformerFactory {
	return m.factory
}

// GetDynamicClient returns the dynamic client
func (m *Manager) GetDynamicClient() dynamic.Interface {
	return m.dynamicClient
}

// GetCRDWatcher returns the CRD watcher
func (m *Manager) GetCRDWatcher() *CRDWatcher {
	return m.crdWatcher
}

// BaseWatcher provides common functionality for all watchers
type BaseWatcher struct {
	GraphStore graph.GraphStore
	Logger     *zap.Logger
	Informer   cache.SharedIndexInformer

	started bool
	mu      sync.Mutex
}

// NewBaseWatcher creates a new base watcher
func NewBaseWatcher(graphStore graph.GraphStore, logger *zap.Logger, informer cache.SharedIndexInformer) *BaseWatcher {
	return &BaseWatcher{
		GraphStore: graphStore,
		Logger:     logger,
		Informer:   informer,
	}
}

// Start starts the informer for this watcher
func (b *BaseWatcher) Start(ctx context.Context) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Already started
	if b.started {
		return false
	}

	// No informer to start
	if b.Informer == nil {
		return false
	}

	// Start the informer in a goroutine
	go b.Informer.Run(ctx.Done())

	// Wait for cache to sync
	if !cache.WaitForCacheSync(ctx.Done(), b.Informer.HasSynced) {
		return false
	}

	b.started = true
	return true
}

// SafeGetObject safely extracts an object from the informer
func SafeGetObject(obj interface{}) (interface{}, error) {
	var object interface{}
	var ok bool

	// Check if the object is a DeletedFinalStateUnknown
	if tombstone, isTombstone := obj.(cache.DeletedFinalStateUnknown); isTombstone {
		object = tombstone.Obj
		ok = true
	} else {
		object = obj
		ok = true
	}

	if !ok || object == nil {
		return nil, fmt.Errorf("error decoding object, invalid type")
	}

	return object, nil
}
