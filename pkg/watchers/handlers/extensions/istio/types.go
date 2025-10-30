package istio

// Istio NodeTypes
// These constants are co-located with Istio handlers conceptually,
// but placed in models/istio to avoid circular dependencies with converters.
// They represent node types as strings that match models.NodeType.
const (
	// Traffic Management
	NodeTypeIstioGateway    = "IstioGateway"
	NodeTypeVirtualService  = "VirtualService"
	NodeTypeDestinationRule = "DestinationRule"
	NodeTypeServiceEntry    = "ServiceEntry"
	NodeTypeSidecar         = "Sidecar"

	// Security
	NodeTypeAuthorizationPolicy   = "AuthorizationPolicy"
	NodeTypePeerAuthentication    = "PeerAuthentication"
	NodeTypeRequestAuthentication = "RequestAuthentication"
)
