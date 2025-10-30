package core

// Core Kubernetes NodeTypes
// These constants are co-located with core handlers conceptually,
// but placed in models/core to avoid circular dependencies with converters.
// They represent node types as strings that match models.NodeType.
const (
	// Compute & Infrastructure
	NodeTypeNode      = "Node"
	NodeTypeNamespace = "Namespace"

	// Workloads
	NodeTypePod         = "Pod"
	NodeTypeDeployment  = "Deployment"
	NodeTypeReplicaSet  = "ReplicaSet"
	NodeTypeStatefulSet = "StatefulSet"
	NodeTypeDaemonSet   = "DaemonSet"

	// Networking
	NodeTypeService       = "Service"
	NodeTypeIngress       = "Ingress"
	NodeTypeEndpoint      = "Endpoint"
	NodeTypeNetworkPolicy = "NetworkPolicy"

	// Storage
	NodeTypePersistentVolume      = "PersistentVolume"
	NodeTypePersistentVolumeClaim = "PersistentVolumeClaim"
	NodeTypeStorageClass          = "StorageClass"

	// Configuration
	NodeTypeConfigMap = "ConfigMap"
	NodeTypeSecret    = "Secret"

	// Observability
	NodeTypeK8sEvent = "K8sEvent"
)
