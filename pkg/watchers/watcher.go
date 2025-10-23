package watchers

import (
	"context"
	"fmt"
	"time"

	"github.com/kagenti/kkbase/pkg/graph"
	"go.uber.org/zap"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// ResourceWatcher defines the interface for watching Kubernetes resources
type ResourceWatcher interface {
	// Start begins watching the resource
	Start(ctx context.Context) error

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

// Manager manages multiple resource watchers
type Manager struct {
	config          Config
	watchers        []ResourceWatcher
	informerFactory informers.SharedInformerFactory
	logger          *zap.Logger
}

// NewManager creates a new watcher manager
func NewManager(config Config) *Manager {
	if config.ResyncPeriod == 0 {
		config.ResyncPeriod = 30 * time.Second
	}

	var informerFactory informers.SharedInformerFactory
	if config.Namespace == "" {
		informerFactory = informers.NewSharedInformerFactory(config.Clientset, config.ResyncPeriod)
	} else {
		informerFactory = informers.NewSharedInformerFactoryWithOptions(
			config.Clientset,
			config.ResyncPeriod,
			informers.WithNamespace(config.Namespace),
		)
	}

	return &Manager{
		config:          config,
		watchers:        []ResourceWatcher{},
		informerFactory: informerFactory,
		logger:          config.Logger,
	}
}

// RegisterWatcher registers a new resource watcher
func (m *Manager) RegisterWatcher(watcher ResourceWatcher) {
	m.watchers = append(m.watchers, watcher)
}

// Start starts all registered watchers
func (m *Manager) Start(ctx context.Context) error {
	m.logger.Info("starting watcher manager", zap.Int("watcher_count", len(m.watchers)))

	// Start all watchers
	for i, watcher := range m.watchers {
		m.logger.Debug("starting watcher", zap.Int("index", i))
		if err := watcher.Start(ctx); err != nil {
			return fmt.Errorf("failed to start watcher %d: %w", i, err)
		}
	}

	// Start the informer factory
	m.informerFactory.Start(ctx.Done())

	// Wait for caches to sync
	m.logger.Info("waiting for caches to sync")
	synced := m.informerFactory.WaitForCacheSync(ctx.Done())
	for informerType, isSynced := range synced {
		if !isSynced {
			return fmt.Errorf("failed to sync cache for %v", informerType)
		}
	}

	m.logger.Info("all caches synced successfully")
	return nil
}

// GetInformerFactory returns the shared informer factory
func (m *Manager) GetInformerFactory() informers.SharedInformerFactory {
	return m.informerFactory
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

// Start starts the base watcher
func (b *BaseWatcher) Start(ctx context.Context) error {
	// The informer will be started by the factory
	return nil
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
