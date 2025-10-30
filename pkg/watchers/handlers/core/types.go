package core

import "github.com/kagenti/kkbase/pkg/models"

// Core Kubernetes NodeTypes
// These constants are co-located with core handlers conceptually,
// but placed in models/core to avoid circular dependencies with converters.
// They represent node types as strings that match models.NodeType.
const (
	// Compute & Infrastructure
	NodeTypeNode      models.NodeType = "Node"
	NodeTypeNamespace models.NodeType = "Namespace"

	// Workloads
	NodeTypePod         models.NodeType = "Pod"
	NodeTypeDeployment  models.NodeType = "Deployment"
	NodeTypeReplicaSet  models.NodeType = "ReplicaSet"
	NodeTypeStatefulSet models.NodeType = "StatefulSet"
	NodeTypeDaemonSet   models.NodeType = "DaemonSet"

	// Networking
	NodeTypeService       models.NodeType = "Service"
	NodeTypeIngress       models.NodeType = "Ingress"
	NodeTypeEndpoint      models.NodeType = "Endpoint"
	NodeTypeNetworkPolicy models.NodeType = "NetworkPolicy"

	// Storage
	NodeTypePersistentVolume      models.NodeType = "PersistentVolume"
	NodeTypePersistentVolumeClaim models.NodeType = "PersistentVolumeClaim"
	NodeTypeStorageClass          models.NodeType = "StorageClass"

	// Configuration
	NodeTypeConfigMap models.NodeType = "ConfigMap"
	NodeTypeSecret    models.NodeType = "Secret"

	// Observability
	NodeTypeK8sEvent models.NodeType = "K8sEvent"
)

// Core Kubernetes EdgeTypes
// These constants represent edge types (relationships) that are specific to core K8s resources.
// Shared edge types (used across multiple handler packages) remain in models.EdgeType.
const (
	// Structural & Hierarchical
	EdgeTypeManages     models.EdgeType = "MANAGES"
	EdgeTypeContains    models.EdgeType = "CONTAINS"
	EdgeTypeScheduledOn models.EdgeType = "SCHEDULED_ON"

	// Networking
	EdgeTypeSelectsPods models.EdgeType = "SELECTS_PODS"

	// Storage
	EdgeTypeMounts        models.EdgeType = "MOUNTS"
	EdgeTypeBoundTo       models.EdgeType = "BOUND_TO"
	EdgeTypeProvisionedBy models.EdgeType = "PROVISIONED_BY"

	// Configuration
	EdgeTypeUsesConfig models.EdgeType = "USES_CONFIG"

	// Observability
	EdgeTypeInvolves models.EdgeType = "INVOLVES"
)
