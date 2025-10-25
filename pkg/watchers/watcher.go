package watchers

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/kagenti/kkbase/pkg/graph"
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
}

// Config holds configuration for watchers
type Config struct {
	Clientset    *kubernetes.Clientset
	GraphStore   graph.GraphStore
	Logger       *zap.Logger
	ResyncPeriod time.Duration
	Namespace    string // Empty string for all namespaces
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

	started bool
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
		started:       false,
	}, nil
}

// RegisterHandler registers a resource handler with a given name
func (m *Manager) RegisterHandler(name string, handler ResourceWatcher) {
	m.handlersMu.Lock()
	defer m.handlersMu.Unlock()

	m.handlers[name] = handler
	m.logger.Info("registered handler", zap.String("name", name))
}

// UnregisterHandler removes a resource handler
func (m *Manager) UnregisterHandler(name string) {
	m.handlersMu.Lock()
	defer m.handlersMu.Unlock()

	delete(m.handlers, name)
	m.logger.Info("unregistered handler", zap.String("name", name))
}

// RegisterHandlerFactory registers a handler factory that will be called when a CRD becomes available
func (m *Manager) RegisterHandlerFactory(
	name string,
	group string,
	kind string,
	factory func() ResourceWatcher,
) {
	m.crdWatcher.WatchCRD(group, kind, func(crd *CRDInfo) {
		m.logger.Info("creating handler for CRD",
			zap.String("handler", name),
			zap.String("group", crd.Group),
			zap.String("kind", crd.Kind),
		)

		// Create the handler
		handler := factory()

		// Register it
		m.RegisterHandler(name, handler)

		// If manager is already started, the handler's informer is already running
		// because it was added to the factory during construction
		if m.started {
			m.logger.Info("handler registered dynamically (runtime)", zap.String("handler", name))
		}
	}, nil)
}

// Start starts the manager and all registered handlers
func (m *Manager) Start(ctx context.Context) error {
	m.handlersMu.RLock()
	handlerCount := len(m.handlers)
	m.handlersMu.RUnlock()

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
}

// NewBaseWatcher creates a new base watcher
func NewBaseWatcher(graphStore graph.GraphStore, logger *zap.Logger, informer cache.SharedIndexInformer) *BaseWatcher {
	return &BaseWatcher{
		GraphStore: graphStore,
		Logger:     logger,
		Informer:   informer,
	}
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
