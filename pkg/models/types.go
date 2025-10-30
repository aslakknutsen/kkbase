package models

import (
	"fmt"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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

// Shared NodeTypes (not specific to any handler package)
// Handler-specific types are defined in their respective packages:
// - pkg/watchers/handlers/core/types.go (core Kubernetes resources)
// - pkg/watchers/handlers/extensions/gateway/types.go (Gateway API)
// - pkg/watchers/handlers/extensions/istio/types.go (Istio)
const (
	// Infrastructure

	// Derived from other resources (not a K8s resource itself)
	NodeTypeContainer NodeType = "Container"

	// Observability (from traces/monitoring, not K8s resources)
	NodeTypeMetric      NodeType = "Metric"
	NodeTypeLogEntry    NodeType = "LogEntry"
	NodeTypeTrace       NodeType = "Trace"
	NodeTypeSpan        NodeType = "Span"
	NodeTypeServiceCall NodeType = "ServiceCall"
)

// GetNodeID generates a unique identifier for a Kubernetes resource
// Format: kind/namespace/name for namespaced resources, kind/name for cluster-scoped
func GetNodeID(kind, namespace, name string) string {
	if namespace == "" {
		return fmt.Sprintf("%s/%s", kind, name)
	}
	return fmt.Sprintf("%s/%s/%s", kind, namespace, name)
}

// GetOwnerReference extracts the controller owner reference from a list
// Returns the first controller owner if found, otherwise the first owner, or nil
func GetOwnerReference(ownerRefs []metav1.OwnerReference) *metav1.OwnerReference {
	for i := range ownerRefs {
		if ownerRefs[i].Controller != nil && *ownerRefs[i].Controller {
			return &ownerRefs[i]
		}
	}
	if len(ownerRefs) > 0 {
		return &ownerRefs[0]
	}
	return nil
}

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
