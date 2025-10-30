package gateway

// Gateway API NodeTypes
// These constants are co-located with Gateway API handlers conceptually,
// but placed in models/gateway to avoid circular dependencies with converters.
// They represent node types as strings that match models.NodeType.
const (
	NodeTypeGatewayClass     = "GatewayClass"
	NodeTypeGateway          = "Gateway"
	NodeTypeHTTPRoute        = "HTTPRoute"
	NodeTypeGRPCRoute        = "GRPCRoute"
	NodeTypeTCPRoute         = "TCPRoute"
	NodeTypeUDPRoute         = "UDPRoute"
	NodeTypeTLSRoute         = "TLSRoute"
	NodeTypeReferenceGrant   = "ReferenceGrant"
	NodeTypeBackendTLSPolicy = "BackendTLSPolicy"
)

// Gateway API EdgeTypes
// These constants represent edge types (relationships) that are specific to Gateway API resources.
// Shared edge types (used across multiple handler packages) remain in models.EdgeType.
const (
	EdgeTypeImplementedBy = "IMPLEMENTED_BY"
	EdgeTypeForwardsTo    = "FORWARDS_TO"
	EdgeTypeUsesTLSFrom   = "USES_TLS_FROM"
	EdgeTypePermittedBy   = "PERMITTED_BY"
	EdgeTypeAllowsRouteTo = "ALLOWS_ROUTE_TO"
)
