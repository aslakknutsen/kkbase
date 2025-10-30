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

// Istio EdgeTypes
// These constants represent edge types (relationships) that are specific to Istio resources.
// Shared edge types (used across multiple handler packages) remain in models.EdgeType.
const (
	// Traffic Management
	EdgeTypeSelectsProxy      = "SELECTS_PROXY"
	EdgeTypeRoutesTrafficFor  = "ROUTES_TRAFFIC_FOR"
	EdgeTypeRoutesToSubset    = "ROUTES_TO_SUBSET"
	EdgeTypeDefinesPolicyFor  = "DEFINES_POLICY_FOR"
	EdgeTypeSelectsSubsetPods = "SELECTS_SUBSET_PODS"
)
