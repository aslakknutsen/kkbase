package models

import "sync"

// NodeType represents the type of a graph node
type NodeType string

// NodeTypeMetadata contains metadata about a node type
type NodeTypeMetadata struct {
	Type          NodeType
	ClusterScoped bool
	Kind          string
	APIGroup      string
}

// Global registry with thread-safe access
var (
	nodeTypeRegistry  = make(map[NodeType]NodeTypeMetadata)
	kindToNodeTypeMap = make(map[string]NodeType)
	registryMu        sync.RWMutex
)

// RegisterNodeType registers a node type with its metadata
func RegisterNodeType(meta NodeTypeMetadata) {
	registryMu.Lock()
	defer registryMu.Unlock()

	nodeTypeRegistry[meta.Type] = meta
	kindToNodeTypeMap[meta.Kind] = meta.Type
}

// IsClusterScoped returns true if the node type is cluster-scoped
func (nt NodeType) IsClusterScoped() bool {
	registryMu.RLock()
	defer registryMu.RUnlock()

	meta, ok := nodeTypeRegistry[nt]
	if !ok {
		return false // default to namespaced
	}
	return meta.ClusterScoped
}

// ToKind returns the Kubernetes Kind string for this NodeType
func (nt NodeType) ToKind() string {
	registryMu.RLock()
	defer registryMu.RUnlock()

	meta, ok := nodeTypeRegistry[nt]
	if !ok {
		return string(nt) // fallback to the NodeType string itself
	}
	return meta.Kind
}

// NodeTypeFromKind converts a Kubernetes Kind string to a NodeType
// Returns the NodeType and a boolean indicating if the kind is recognized
func NodeTypeFromKind(kind string) (NodeType, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()

	nodeType, ok := kindToNodeTypeMap[kind]
	return nodeType, ok
}

// NodeTypeFromString converts a string to a NodeType
// This is useful for deserializing from database or API
func NodeTypeFromString(s string) (NodeType, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()

	// First try direct match as NodeType
	nt := NodeType(s)
	if _, ok := nodeTypeRegistry[nt]; ok {
		return nt, true
	}

	// Try as Kind string
	nodeType, ok := kindToNodeTypeMap[s]
	return nodeType, ok
}

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

	// Gateway API
	NodeTypeGatewayClass     NodeType = "GatewayClass"
	NodeTypeGateway          NodeType = "Gateway"
	NodeTypeHTTPRoute        NodeType = "HTTPRoute"
	NodeTypeGRPCRoute        NodeType = "GRPCRoute"
	NodeTypeTCPRoute         NodeType = "TCPRoute"
	NodeTypeUDPRoute         NodeType = "UDPRoute"
	NodeTypeTLSRoute         NodeType = "TLSRoute"
	NodeTypeReferenceGrant   NodeType = "ReferenceGrant"
	NodeTypeBackendTLSPolicy NodeType = "BackendTLSPolicy"

	// Istio - Traffic Management
	NodeTypeIstioGateway    NodeType = "IstioGateway"
	NodeTypeVirtualService  NodeType = "VirtualService"
	NodeTypeDestinationRule NodeType = "DestinationRule"
	NodeTypeServiceEntry    NodeType = "ServiceEntry"
	NodeTypeSidecar         NodeType = "Sidecar"

	// Istio - Security
	NodeTypeAuthorizationPolicy   NodeType = "AuthorizationPolicy"
	NodeTypePeerAuthentication    NodeType = "PeerAuthentication"
	NodeTypeRequestAuthentication NodeType = "RequestAuthentication"

	// Storage
	NodeTypePersistentVolume      NodeType = "PersistentVolume"
	NodeTypePersistentVolumeClaim NodeType = "PersistentVolumeClaim"
	NodeTypeStorageClass          NodeType = "StorageClass"

	// Configuration
	NodeTypeConfigMap NodeType = "ConfigMap"
	NodeTypeSecret    NodeType = "Secret"

	// Observability
	NodeTypeMetric      NodeType = "Metric"
	NodeTypeLogEntry    NodeType = "LogEntry"
	NodeTypeTrace       NodeType = "Trace"
	NodeTypeSpan        NodeType = "Span"
	NodeTypeServiceCall NodeType = "ServiceCall"
	NodeTypeK8sEvent    NodeType = "K8sEvent"

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

	// Gateway API
	EdgeTypeImplementedBy EdgeType = "IMPLEMENTED_BY"
	EdgeTypeAttachesTo    EdgeType = "ATTACHES_TO"
	EdgeTypeForwardsTo    EdgeType = "FORWARDS_TO"
	EdgeTypeUsesTLSFrom   EdgeType = "USES_TLS_FROM"
	EdgeTypePermittedBy   EdgeType = "PERMITTED_BY"
	EdgeTypeAllowsRouteTo EdgeType = "ALLOWS_ROUTE_TO"

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

	// Trace relationships
	EdgeTypeContainsSpan   EdgeType = "CONTAINS_SPAN"
	EdgeTypeParentOf       EdgeType = "PARENT_OF"
	EdgeTypeOriginatedFrom EdgeType = "ORIGINATED_FROM"
	EdgeTypeObservedCallTo EdgeType = "OBSERVED_CALL_TO"
	EdgeTypeCalls          EdgeType = "CALLS"
	EdgeTypeFailedCallTo   EdgeType = "FAILED_CALL_TO"
	EdgeTypeExecutedIn     EdgeType = "EXECUTED_IN"

	// Dynamic relationships
	EdgeTypeCommunicatesWith EdgeType = "COMMUNICATES_WITH"
	EdgeTypeDependsOn        EdgeType = "DEPENDS_ON"

	// Istio relationships
	EdgeTypeSelectsProxy      EdgeType = "SELECTS_PROXY"
	EdgeTypeRoutesTrafficFor  EdgeType = "ROUTES_TRAFFIC_FOR"
	EdgeTypeRoutesToSubset    EdgeType = "ROUTES_TO_SUBSET"
	EdgeTypeDefinesPolicyFor  EdgeType = "DEFINES_POLICY_FOR"
	EdgeTypeSelectsSubsetPods EdgeType = "SELECTS_SUBSET_PODS"
	EdgeTypeAppliesTo         EdgeType = "APPLIES_TO"
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
