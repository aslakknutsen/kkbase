package core

import (
	"github.com/kagenti/kkbase/pkg/graph"
	"github.com/kagenti/kkbase/pkg/watchers"
	"github.com/kagenti/kkbase/pkg/watchers/handlers"
	"go.uber.org/zap"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
)

// RegisterCoreHandlers registers all core Kubernetes resource handlers
func RegisterCoreHandlers(registry *handlers.Registry) {
	// Namespace must be registered first as other resources depend on it
	registry.Register(&handlers.HandlerRegistration{
		Name:        "namespace",
		Description: "Watches Kubernetes Namespaces",
		Category:    "core",
		Required:    true,
		Factory: func(clientset *kubernetes.Clientset, graphStore graph.GraphStore, logger *zap.Logger, informerFactory informers.SharedInformerFactory) watchers.ResourceWatcher {
			return NewNamespaceHandler(graphStore, logger, informerFactory)
		},
	})

	// Nodes
	registry.Register(&handlers.HandlerRegistration{
		Name:        "node",
		Description: "Watches Kubernetes Nodes",
		Category:    "core",
		Required:    true,
		Factory: func(clientset *kubernetes.Clientset, graphStore graph.GraphStore, logger *zap.Logger, informerFactory informers.SharedInformerFactory) watchers.ResourceWatcher {
			return NewNodeHandler(graphStore, logger, informerFactory)
		},
	})

	// Workload resources
	registry.Register(&handlers.HandlerRegistration{
		Name:        "deployment",
		Description: "Watches Kubernetes Deployments",
		Category:    "workloads",
		Required:    false,
		Factory: func(clientset *kubernetes.Clientset, graphStore graph.GraphStore, logger *zap.Logger, informerFactory informers.SharedInformerFactory) watchers.ResourceWatcher {
			return NewDeploymentHandler(clientset, graphStore, logger, informerFactory)
		},
	})

	registry.Register(&handlers.HandlerRegistration{
		Name:        "replicaset",
		Description: "Watches Kubernetes ReplicaSets",
		Category:    "workloads",
		Required:    false,
		Factory: func(clientset *kubernetes.Clientset, graphStore graph.GraphStore, logger *zap.Logger, informerFactory informers.SharedInformerFactory) watchers.ResourceWatcher {
			return NewReplicaSetHandler(clientset, graphStore, logger, informerFactory)
		},
	})

	registry.Register(&handlers.HandlerRegistration{
		Name:        "pod",
		Description: "Watches Kubernetes Pods",
		Category:    "workloads",
		Required:    true,
		Factory: func(clientset *kubernetes.Clientset, graphStore graph.GraphStore, logger *zap.Logger, informerFactory informers.SharedInformerFactory) watchers.ResourceWatcher {
			return NewPodHandler(clientset, graphStore, logger, informerFactory)
		},
	})

	// Networking resources
	registry.Register(&handlers.HandlerRegistration{
		Name:        "service",
		Description: "Watches Kubernetes Services",
		Category:    "networking",
		Required:    false,
		Factory: func(clientset *kubernetes.Clientset, graphStore graph.GraphStore, logger *zap.Logger, informerFactory informers.SharedInformerFactory) watchers.ResourceWatcher {
			return NewServiceHandler(clientset, graphStore, logger, informerFactory)
		},
	})

	// Storage resources
	registry.Register(&handlers.HandlerRegistration{
		Name:        "persistentvolume",
		Description: "Watches Kubernetes PersistentVolumes",
		Category:    "storage",
		Required:    false,
		Factory: func(clientset *kubernetes.Clientset, graphStore graph.GraphStore, logger *zap.Logger, informerFactory informers.SharedInformerFactory) watchers.ResourceWatcher {
			return NewPVHandler(clientset, graphStore, logger, informerFactory)
		},
	})

	registry.Register(&handlers.HandlerRegistration{
		Name:        "persistentvolumeclaim",
		Description: "Watches Kubernetes PersistentVolumeClaims",
		Category:    "storage",
		Required:    false,
		Factory: func(clientset *kubernetes.Clientset, graphStore graph.GraphStore, logger *zap.Logger, informerFactory informers.SharedInformerFactory) watchers.ResourceWatcher {
			return NewPVCHandler(clientset, graphStore, logger, informerFactory)
		},
	})

	// Configuration resources
	registry.Register(&handlers.HandlerRegistration{
		Name:        "configmap",
		Description: "Watches Kubernetes ConfigMaps",
		Category:    "configuration",
		Required:    false,
		Factory: func(clientset *kubernetes.Clientset, graphStore graph.GraphStore, logger *zap.Logger, informerFactory informers.SharedInformerFactory) watchers.ResourceWatcher {
			return NewConfigMapHandler(clientset, graphStore, logger, informerFactory)
		},
	})

	// Observability resources
	registry.Register(&handlers.HandlerRegistration{
		Name:        "event",
		Description: "Watches Kubernetes Events",
		Category:    "observability",
		Required:    false,
		Factory: func(clientset *kubernetes.Clientset, graphStore graph.GraphStore, logger *zap.Logger, informerFactory informers.SharedInformerFactory) watchers.ResourceWatcher {
			return NewEventHandler(clientset, graphStore, logger, informerFactory)
		},
	})
}

// DefaultLogger returns a logger for when one isn't provided
func defaultLogger() *zap.Logger {
	logger, _ := zap.NewProduction()
	return logger
}
