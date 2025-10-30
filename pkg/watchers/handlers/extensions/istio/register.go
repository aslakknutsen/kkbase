package istio

import (
	"github.com/kagenti/kkbase/pkg/graph"
	"github.com/kagenti/kkbase/pkg/watchers"
	"go.uber.org/zap"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
)

// RegisterIstioHandlers registers Istio resource handlers with dynamic CRD detection
func RegisterIstioHandlers(
	manager *watchers.Manager,
	dynamicClient dynamic.Interface,
	factory dynamicinformer.DynamicSharedInformerFactory,
	graphStore graph.GraphStore,
	logger *zap.Logger,
) {
	logger.Info("registering Istio handlers with CRD watcher")

	// Register Istio Gateway handler
	manager.RegisterHandlerFactory(
		watchers.ResourceTypeInfo{
			NodeType:      NodeTypeIstioGateway,
			Kind:          "Gateway",
			APIGroup:      "networking.istio.io",
			ClusterScoped: false,
		},
		func() watchers.ResourceWatcher {
			return NewIstioGatewayHandler(nil, dynamicClient, graphStore, logger, factory)
		},
	)

	// Register VirtualService handler
	manager.RegisterHandlerFactory(
		watchers.ResourceTypeInfo{
			NodeType:      NodeTypeVirtualService,
			Kind:          "VirtualService",
			APIGroup:      "networking.istio.io",
			ClusterScoped: false,
		},
		func() watchers.ResourceWatcher {
			return NewVirtualServiceHandler(nil, dynamicClient, graphStore, logger, factory)
		},
	)

	// Register DestinationRule handler
	manager.RegisterHandlerFactory(
		watchers.ResourceTypeInfo{
			NodeType:      NodeTypeDestinationRule,
			Kind:          "DestinationRule",
			APIGroup:      "networking.istio.io",
			ClusterScoped: false,
		},
		func() watchers.ResourceWatcher {
			return NewDestinationRuleHandler(nil, dynamicClient, graphStore, logger, factory)
		},
	)

	// Register ServiceEntry handler
	manager.RegisterHandlerFactory(
		watchers.ResourceTypeInfo{
			NodeType:      NodeTypeServiceEntry,
			Kind:          "ServiceEntry",
			APIGroup:      "networking.istio.io",
			ClusterScoped: false,
		},
		func() watchers.ResourceWatcher {
			return NewServiceEntryHandler(nil, dynamicClient, graphStore, logger, factory)
		},
	)

	// Register Sidecar handler
	manager.RegisterHandlerFactory(
		watchers.ResourceTypeInfo{
			NodeType:      NodeTypeSidecar,
			Kind:          "Sidecar",
			APIGroup:      "networking.istio.io",
			ClusterScoped: false,
		},
		func() watchers.ResourceWatcher {
			return NewSidecarHandler(nil, dynamicClient, graphStore, logger, factory)
		},
	)

	// Register AuthorizationPolicy handler
	manager.RegisterHandlerFactory(
		watchers.ResourceTypeInfo{
			NodeType:      NodeTypeAuthorizationPolicy,
			Kind:          "AuthorizationPolicy",
			APIGroup:      "security.istio.io",
			ClusterScoped: false,
		},
		func() watchers.ResourceWatcher {
			return NewAuthorizationPolicyHandler(nil, dynamicClient, graphStore, logger, factory)
		},
	)

	// Register PeerAuthentication handler
	manager.RegisterHandlerFactory(
		watchers.ResourceTypeInfo{
			NodeType:      NodeTypePeerAuthentication,
			Kind:          "PeerAuthentication",
			APIGroup:      "security.istio.io",
			ClusterScoped: false,
		},
		func() watchers.ResourceWatcher {
			return NewPeerAuthenticationHandler(nil, dynamicClient, graphStore, logger, factory)
		},
	)

	// Register RequestAuthentication handler
	manager.RegisterHandlerFactory(
		watchers.ResourceTypeInfo{
			NodeType:      NodeTypeRequestAuthentication,
			Kind:          "RequestAuthentication",
			APIGroup:      "security.istio.io",
			ClusterScoped: false,
		},
		func() watchers.ResourceWatcher {
			return NewRequestAuthenticationHandler(nil, dynamicClient, graphStore, logger, factory)
		},
	)

	logger.Info("Istio handlers registered with CRD watcher")
}
