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
		"istio-gateway",
		"networking.istio.io",
		"Gateway",
		func() watchers.ResourceWatcher {
			return NewIstioGatewayHandler(nil, dynamicClient, graphStore, logger, factory)
		},
	)

	// Register VirtualService handler
	manager.RegisterHandlerFactory(
		"virtualservice",
		"networking.istio.io",
		"VirtualService",
		func() watchers.ResourceWatcher {
			return NewVirtualServiceHandler(nil, dynamicClient, graphStore, logger, factory)
		},
	)

	// Register DestinationRule handler
	manager.RegisterHandlerFactory(
		"destinationrule",
		"networking.istio.io",
		"DestinationRule",
		func() watchers.ResourceWatcher {
			return NewDestinationRuleHandler(nil, dynamicClient, graphStore, logger, factory)
		},
	)

	// Register ServiceEntry handler
	manager.RegisterHandlerFactory(
		"serviceentry",
		"networking.istio.io",
		"ServiceEntry",
		func() watchers.ResourceWatcher {
			return NewServiceEntryHandler(nil, dynamicClient, graphStore, logger, factory)
		},
	)

	// Register Sidecar handler
	manager.RegisterHandlerFactory(
		"sidecar",
		"networking.istio.io",
		"Sidecar",
		func() watchers.ResourceWatcher {
			return NewSidecarHandler(nil, dynamicClient, graphStore, logger, factory)
		},
	)

	// Register AuthorizationPolicy handler
	manager.RegisterHandlerFactory(
		"authorizationpolicy",
		"security.istio.io",
		"AuthorizationPolicy",
		func() watchers.ResourceWatcher {
			return NewAuthorizationPolicyHandler(nil, dynamicClient, graphStore, logger, factory)
		},
	)

	// Register PeerAuthentication handler
	manager.RegisterHandlerFactory(
		"peerauthentication",
		"security.istio.io",
		"PeerAuthentication",
		func() watchers.ResourceWatcher {
			return NewPeerAuthenticationHandler(nil, dynamicClient, graphStore, logger, factory)
		},
	)

	// Register RequestAuthentication handler
	manager.RegisterHandlerFactory(
		"requestauthentication",
		"security.istio.io",
		"RequestAuthentication",
		func() watchers.ResourceWatcher {
			return NewRequestAuthenticationHandler(nil, dynamicClient, graphStore, logger, factory)
		},
	)

	logger.Info("Istio handlers registered with CRD watcher")
}
