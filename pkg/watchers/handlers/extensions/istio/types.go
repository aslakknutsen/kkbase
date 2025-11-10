package istio

import "github.com/aslakknutsen/kkbase/pkg/models"

// Istio NodeTypes
// These constants are co-located with Istio handlers conceptually,
// but placed in models/istio to avoid circular dependencies with converters.
// They represent node types as strings that match models.NodeType.
const (
	// Traffic Management
	NodeTypeIstioGateway    models.NodeType = "IstioGateway"
	NodeTypeVirtualService  models.NodeType = "VirtualService"
	NodeTypeDestinationRule models.NodeType = "DestinationRule"
	NodeTypeServiceEntry    models.NodeType = "ServiceEntry"
	NodeTypeSidecar         models.NodeType = "Sidecar"

	// Security
	NodeTypeAuthorizationPolicy   models.NodeType = "AuthorizationPolicy"
	NodeTypePeerAuthentication    models.NodeType = "PeerAuthentication"
	NodeTypeRequestAuthentication models.NodeType = "RequestAuthentication"
)

// Istio EdgeTypes
// These constants represent edge types (relationships) that are specific to Istio resources.
// Shared edge types (used across multiple handler packages) remain in models.EdgeType.
const (
	// Traffic Management
	EdgeTypeSelectsProxy      models.EdgeType = "SELECTS_PROXY"
	EdgeTypeRoutesTrafficFor  models.EdgeType = "ROUTES_TRAFFIC_FOR"
	EdgeTypeRoutesToSubset    models.EdgeType = "ROUTES_TO_SUBSET"
	EdgeTypeDefinesPolicyFor  models.EdgeType = "DEFINES_POLICY_FOR"
	EdgeTypeSelectsSubsetPods models.EdgeType = "SELECTS_SUBSET_PODS"
)
