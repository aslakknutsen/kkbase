package models

// NodeType represents the type of a graph node
type NodeType string

// Node types as defined in the knowledge graph schema
const (
	// Compute & Hardware
	NodeTypeCluster NodeType = "Cluster"
	NodeTypeNode    NodeType = "Node"

	// Workloads
	NodeTypePod         NodeType = "Pod"
	NodeTypeContainer   NodeType = "Container"
	NodeTypeDeployment  NodeType = "Deployment"
	NodeTypeReplicaSet  NodeType = "ReplicaSet"
	NodeTypeStatefulSet NodeType = "StatefulSet"
	NodeTypeDaemonSet   NodeType = "DaemonSet"

	// Networking
	NodeTypeService       NodeType = "Service"
	NodeTypeIngress       NodeType = "Ingress"
	NodeTypeEndpoint      NodeType = "Endpoint"
	NodeTypeNetworkPolicy NodeType = "NetworkPolicy"

	// Storage
	NodeTypePersistentVolume      NodeType = "PersistentVolume"
	NodeTypePersistentVolumeClaim NodeType = "PersistentVolumeClaim"
	NodeTypeStorageClass          NodeType = "StorageClass"

	// Configuration
	NodeTypeConfigMap NodeType = "ConfigMap"
	NodeTypeSecret    NodeType = "Secret"

	// Observability
	NodeTypeMetric   NodeType = "Metric"
	NodeTypeLogEntry NodeType = "LogEntry"
	NodeTypeTrace    NodeType = "Trace"
	NodeTypeK8sEvent NodeType = "K8sEvent"

	// Other
	NodeTypeNamespace NodeType = "Namespace"
)

// EdgeType represents the type of a graph edge (relationship)
type EdgeType string

// Edge types as defined in the knowledge graph schema
const (
	// Structural & Hierarchical
	EdgeTypeManages     EdgeType = "MANAGES"
	EdgeTypeContains    EdgeType = "CONTAINS"
	EdgeTypeScheduledOn EdgeType = "SCHEDULED_ON"
	EdgeTypePartOf      EdgeType = "PART_OF"
	EdgeTypeInNamespace EdgeType = "IN_NAMESPACE"

	// Networking
	EdgeTypeSelectsPods EdgeType = "SELECTS_PODS"
	EdgeTypeHasEndpoint EdgeType = "HAS_ENDPOINT"
	EdgeTypeRoutesTo    EdgeType = "ROUTES_TO"
	EdgeTypeAffectedBy  EdgeType = "AFFECTED_BY"

	// Storage
	EdgeTypeMounts        EdgeType = "MOUNTS"
	EdgeTypeBoundTo       EdgeType = "BOUND_TO"
	EdgeTypeProvisionedBy EdgeType = "PROVISIONED_BY"

	// Configuration
	EdgeTypeUsesConfig EdgeType = "USES_CONFIG"
	EdgeTypeUsesSecret EdgeType = "USES_SECRET"

	// Observability
	EdgeTypeEmits     EdgeType = "EMITS"
	EdgeTypeGenerates EdgeType = "GENERATES"
	EdgeTypeInvolves  EdgeType = "INVOLVES"

	// Dynamic relationships
	EdgeTypeCommunicatesWith EdgeType = "COMMUNICATES_WITH"
	EdgeTypeDependsOn        EdgeType = "DEPENDS_ON"
)

// GraphNode represents a node in the knowledge graph
type GraphNode struct {
	Type       NodeType
	ID         string
	Properties map[string]interface{}
}

// GraphEdge represents an edge in the knowledge graph
type GraphEdge struct {
	Type       EdgeType
	FromType   NodeType
	FromID     string
	ToType     NodeType
	ToID       string
	Properties map[string]interface{}
}

// NewGraphNode creates a new graph node
func NewGraphNode(nodeType NodeType, id string, properties map[string]interface{}) *GraphNode {
	if properties == nil {
		properties = make(map[string]interface{})
	}
	return &GraphNode{
		Type:       nodeType,
		ID:         id,
		Properties: properties,
	}
}

// NewGraphEdge creates a new graph edge
func NewGraphEdge(edgeType EdgeType, fromType NodeType, fromID string, toType NodeType, toID string, properties map[string]interface{}) *GraphEdge {
	if properties == nil {
		properties = make(map[string]interface{})
	}
	return &GraphEdge{
		Type:       edgeType,
		FromType:   fromType,
		FromID:     fromID,
		ToType:     toType,
		ToID:       toID,
		Properties: properties,
	}
}
