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

// Core Kubernetes EdgeTypes
// These constants represent edge types (relationships) that are specific to core K8s resources.
// Shared edge types (used across multiple handler packages) remain in models.EdgeType.
const (
	// Structural & Hierarchical
	EdgeTypeManages     = "MANAGES"
	EdgeTypeContains    = "CONTAINS"
	EdgeTypeScheduledOn = "SCHEDULED_ON"

	// Networking
	EdgeTypeSelectsPods = "SELECTS_PODS"

	// Storage
	EdgeTypeMounts        = "MOUNTS"
	EdgeTypeBoundTo       = "BOUND_TO"
	EdgeTypeProvisionedBy = "PROVISIONED_BY"

	// Configuration
	EdgeTypeUsesConfig = "USES_CONFIG"

	// Observability
	EdgeTypeInvolves = "INVOLVES"
)
