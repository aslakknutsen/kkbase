package handlers

import (
	"context"

	"github.com/kagenti/kkbase/pkg/graph"
	"github.com/kagenti/kkbase/pkg/watchers"
	"go.uber.org/zap"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
)

// HandlerFactory is a function that creates a new resource handler
type HandlerFactory func(
	clientset *kubernetes.Clientset,
	graphStore graph.GraphStore,
	logger *zap.Logger,
	informerFactory informers.SharedInformerFactory,
) watchers.ResourceWatcher

// HandlerRegistration contains metadata about a handler
type HandlerRegistration struct {
	Name        string
	Description string
	Category    string // "core", "networking", "storage", "observability", "extension"
	Factory     HandlerFactory
	Required    bool // If true, watcher fails to start if handler fails
}

// Registry manages handler registrations
type Registry struct {
	handlers map[string]*HandlerRegistration
	logger   *zap.Logger
}

// NewRegistry creates a new handler registry
func NewRegistry(logger *zap.Logger) *Registry {
	return &Registry{
		handlers: make(map[string]*HandlerRegistration),
		logger:   logger,
	}
}

// Register adds a handler to the registry
func (r *Registry) Register(reg *HandlerRegistration) {
	if _, exists := r.handlers[reg.Name]; exists {
		r.logger.Warn("handler already registered, overwriting", zap.String("name", reg.Name))
	}
	r.handlers[reg.Name] = reg
	r.logger.Debug("registered handler",
		zap.String("name", reg.Name),
		zap.String("category", reg.Category),
		zap.Bool("required", reg.Required),
	)
}

// Get retrieves a handler registration by name
func (r *Registry) Get(name string) (*HandlerRegistration, bool) {
	reg, exists := r.handlers[name]
	return reg, exists
}

// List returns all registered handlers
func (r *Registry) List() []*HandlerRegistration {
	handlers := make([]*HandlerRegistration, 0, len(r.handlers))
	for _, reg := range r.handlers {
		handlers = append(handlers, reg)
	}
	return handlers
}

// ListByCategory returns handlers filtered by category
func (r *Registry) ListByCategory(category string) []*HandlerRegistration {
	handlers := make([]*HandlerRegistration, 0)
	for _, reg := range r.handlers {
		if reg.Category == category {
			handlers = append(handlers, reg)
		}
	}
	return handlers
}

// InstantiateAll creates handler instances for all registered handlers
func (r *Registry) InstantiateAll(
	clientset *kubernetes.Clientset,
	graphStore graph.GraphStore,
	logger *zap.Logger,
	informerFactory informers.SharedInformerFactory,
) []watchers.ResourceWatcher {
	watchers := make([]watchers.ResourceWatcher, 0, len(r.handlers))

	for name, reg := range r.handlers {
		r.logger.Info("instantiating handler",
			zap.String("name", name),
			zap.String("category", reg.Category),
		)

		watcher := reg.Factory(clientset, graphStore, logger, informerFactory)
		watchers = append(watchers, watcher)
	}

	return watchers
}

// InstantiateFiltered creates handler instances for handlers matching a filter function
func (r *Registry) InstantiateFiltered(
	clientset *kubernetes.Clientset,
	graphStore graph.GraphStore,
	logger *zap.Logger,
	informerFactory informers.SharedInformerFactory,
	filter func(*HandlerRegistration) bool,
) []watchers.ResourceWatcher {
	watchers := make([]watchers.ResourceWatcher, 0)

	for name, reg := range r.handlers {
		if !filter(reg) {
			r.logger.Debug("skipping handler (filtered out)",
				zap.String("name", name),
			)
			continue
		}

		r.logger.Info("instantiating handler",
			zap.String("name", name),
			zap.String("category", reg.Category),
		)

		watcher := reg.Factory(clientset, graphStore, logger, informerFactory)
		watchers = append(watchers, watcher)
	}

	return watchers
}

// Start is a convenience method to instantiate and start all handlers
func (r *Registry) Start(
	ctx context.Context,
	manager *watchers.Manager,
	clientset *kubernetes.Clientset,
	graphStore graph.GraphStore,
	logger *zap.Logger,
	informerFactory informers.SharedInformerFactory,
) error {
	handlers := r.InstantiateAll(clientset, graphStore, logger, informerFactory)

	for _, handler := range handlers {
		manager.RegisterWatcher(handler)
	}

	return manager.Start(ctx)
}
