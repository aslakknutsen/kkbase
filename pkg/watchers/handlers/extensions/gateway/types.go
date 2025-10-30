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
