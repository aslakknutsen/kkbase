package core

import (
	"github.com/aslakknutsen/kkbase/pkg/graph"
	"github.com/aslakknutsen/kkbase/pkg/models"
	"github.com/aslakknutsen/kkbase/pkg/watchers"
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
	manager.RegisterHandler(
		watchers.ResourceTypeInfo{
			NodeType:      NodeTypeNamespace,
			Kind:          "Namespace",
			APIGroup:      "",
			ClusterScoped: true,
		},
		NewNamespaceHandler(graphStore, logger, factory),
	)

	// Nodes
	manager.RegisterHandler(
		watchers.ResourceTypeInfo{
			NodeType:      NodeTypeNode,
			Kind:          "Node",
			APIGroup:      "",
			ClusterScoped: true,
		},
		NewNodeHandler(graphStore, logger, factory),
	)

	// Workload resources
	manager.RegisterHandler(
		watchers.ResourceTypeInfo{
			NodeType:      NodeTypeDeployment,
			Kind:          "Deployment",
			APIGroup:      "apps",
			ClusterScoped: false,
		},
		NewDeploymentHandler(clientset, graphStore, logger, factory),
	)

	manager.RegisterHandler(
		watchers.ResourceTypeInfo{
			NodeType:      NodeTypeReplicaSet,
			Kind:          "ReplicaSet",
			APIGroup:      "apps",
			ClusterScoped: false,
		},
		NewReplicaSetHandler(clientset, graphStore, logger, factory),
	)

	manager.RegisterHandler(
		watchers.ResourceTypeInfo{
			NodeType:      NodeTypeStatefulSet,
			Kind:          "StatefulSet",
			APIGroup:      "apps",
			ClusterScoped: false,
		},
		NewStatefulSetHandler(clientset, graphStore, logger, factory),
	)

	manager.RegisterHandler(
		watchers.ResourceTypeInfo{
			NodeType:      NodeTypeDaemonSet,
			Kind:          "DaemonSet",
			APIGroup:      "apps",
			ClusterScoped: false,
		},
		NewDaemonSetHandler(clientset, graphStore, logger, factory),
	)

	manager.RegisterHandler(
		watchers.ResourceTypeInfo{
			NodeType:      NodeTypePod,
			Kind:          "Pod",
			APIGroup:      "",
			ClusterScoped: false,
		},
		NewPodHandler(clientset, graphStore, logger, factory),
	)

	// Networking resources
	manager.RegisterHandler(
		watchers.ResourceTypeInfo{
			NodeType:      NodeTypeService,
			Kind:          "Service",
			APIGroup:      "",
			ClusterScoped: false,
		},
		NewServiceHandler(clientset, graphStore, logger, factory),
	)

	// Storage resources
	manager.RegisterHandler(
		watchers.ResourceTypeInfo{
			NodeType:      NodeTypePersistentVolume,
			Kind:          "PersistentVolume",
			APIGroup:      "",
			ClusterScoped: true,
		},
		NewPVHandler(clientset, graphStore, logger, factory),
	)

	manager.RegisterHandler(
		watchers.ResourceTypeInfo{
			NodeType:      NodeTypePersistentVolumeClaim,
			Kind:          "PersistentVolumeClaim",
			APIGroup:      "",
			ClusterScoped: false,
		},
		NewPVCHandler(clientset, graphStore, logger, factory),
	)

	manager.RegisterHandler(
		watchers.ResourceTypeInfo{
			NodeType:      NodeTypeStorageClass,
			Kind:          "StorageClass",
			APIGroup:      "storage.k8s.io",
			ClusterScoped: true,
		},
		NewStorageClassHandler(clientset, graphStore, logger, factory),
	)

	// Configuration resources
	manager.RegisterHandler(
		watchers.ResourceTypeInfo{
			NodeType:      NodeTypeConfigMap,
			Kind:          "ConfigMap",
			APIGroup:      "",
			ClusterScoped: false,
		},
		NewConfigMapHandler(clientset, graphStore, logger, factory),
	)

	manager.RegisterHandler(
		watchers.ResourceTypeInfo{
			NodeType:      NodeTypeSecret,
			Kind:          "Secret",
			APIGroup:      "",
			ClusterScoped: false,
		},
		NewSecretHandler(clientset, graphStore, logger, factory),
	)

	// Observability resources
	manager.RegisterHandler(
		watchers.ResourceTypeInfo{
			NodeType:      NodeTypeK8sEvent,
			Kind:          "Event",
			APIGroup:      "",
			ClusterScoped: false,
		},
		NewEventHandler(clientset, graphStore, logger, factory),
	)

	// Container is special - not a real K8s resource, but we register its type
	models.RegisterNodeType(models.NodeTypeMetadata{
		Type:          models.NodeTypeContainer,
		Kind:          "Container",
		APIGroup:      "",
		ClusterScoped: false,
	})
}
