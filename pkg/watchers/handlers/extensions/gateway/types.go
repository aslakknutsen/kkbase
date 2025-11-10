package gateway

import "github.com/aslakknutsen/kkbase/pkg/models"

// Gateway API NodeTypes
// These constants are co-located with Gateway API handlers conceptually,
// but placed in models/gateway to avoid circular dependencies with converters.
// They represent node types as strings that match models.NodeType.
const (
	NodeTypeGatewayClass     models.NodeType = "GatewayClass"
	NodeTypeGateway          models.NodeType = "Gateway"
	NodeTypeHTTPRoute        models.NodeType = "HTTPRoute"
	NodeTypeGRPCRoute        models.NodeType = "GRPCRoute"
	NodeTypeTCPRoute         models.NodeType = "TCPRoute"
	NodeTypeUDPRoute         models.NodeType = "UDPRoute"
	NodeTypeTLSRoute         models.NodeType = "TLSRoute"
	NodeTypeReferenceGrant   models.NodeType = "ReferenceGrant"
	NodeTypeBackendTLSPolicy models.NodeType = "BackendTLSPolicy"
)

// Gateway API EdgeTypes
// These constants represent edge types (relationships) that are specific to Gateway API resources.
// Shared edge types (used across multiple handler packages) remain in models.EdgeType.
const (
	EdgeTypeImplementedBy models.EdgeType = "IMPLEMENTED_BY"
	EdgeTypeForwardsTo    models.EdgeType = "FORWARDS_TO"
	EdgeTypeUsesTLSFrom   models.EdgeType = "USES_TLS_FROM"
	EdgeTypePermittedBy   models.EdgeType = "PERMITTED_BY"
	EdgeTypeAllowsRouteTo models.EdgeType = "ALLOWS_ROUTE_TO"
)
