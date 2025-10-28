package core

import (
	"github.com/kagenti/kkbase/pkg/graph"
	"github.com/kagenti/kkbase/pkg/watchers"
	"go.uber.org/zap"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/kubernetes"
)

// RegisterCoreHandlers registers all core Kubernetes resource handlers
func RegisterCoreHandlers(
	manager *watchers.Manager,
	clientset *kubernetes.Clientset,
	factory dynamicinformer.DynamicSharedInformerFactory,
	graphStore graph.GraphStore,
	logger *zap.Logger,
) {
	// Namespace must be registered first as other resources depend on it
	manager.RegisterHandler("namespace", NewNamespaceHandler(graphStore, logger, factory))

	// Nodes
	manager.RegisterHandler("node", NewNodeHandler(graphStore, logger, factory))

	// Workload resources
	manager.RegisterHandler("deployment", NewDeploymentHandler(clientset, graphStore, logger, factory))
	manager.RegisterHandler("replicaset", NewReplicaSetHandler(clientset, graphStore, logger, factory))
	manager.RegisterHandler("statefulset", NewStatefulSetHandler(clientset, graphStore, logger, factory))
	manager.RegisterHandler("daemonset", NewDaemonSetHandler(clientset, graphStore, logger, factory))
	manager.RegisterHandler("pod", NewPodHandler(clientset, graphStore, logger, factory))

	// Networking resources
	manager.RegisterHandler("service", NewServiceHandler(clientset, graphStore, logger, factory))

	// Storage resources
	manager.RegisterHandler("persistentvolume", NewPVHandler(clientset, graphStore, logger, factory))
	manager.RegisterHandler("persistentvolumeclaim", NewPVCHandler(clientset, graphStore, logger, factory))
	manager.RegisterHandler("storageclass", NewStorageClassHandler(clientset, graphStore, logger, factory))

	// Configuration resources
	manager.RegisterHandler("configmap", NewConfigMapHandler(clientset, graphStore, logger, factory))
	manager.RegisterHandler("secret", NewSecretHandler(clientset, graphStore, logger, factory))

	// Observability resources
	manager.RegisterHandler("event", NewEventHandler(clientset, graphStore, logger, factory))

}
