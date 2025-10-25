package gateway

import (
	"github.com/kagenti/kkbase/pkg/graph"
	"github.com/kagenti/kkbase/pkg/watchers"
	"go.uber.org/zap"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
)

// RegisterGatewayAPIHandlers registers Gateway API resource handlers with dynamic CRD detection
func RegisterGatewayAPIHandlers(
	manager *watchers.Manager,
	dynamicClient dynamic.Interface,
	factory dynamicinformer.DynamicSharedInformerFactory,
	graphStore graph.GraphStore,
	logger *zap.Logger,
) {
	logger.Info("registering Gateway API handlers with CRD watcher")

	// Register GatewayClass handler
	manager.RegisterHandlerFactory(
		"gatewayclass",
		"gateway.networking.k8s.io",
		"GatewayClass",
		func() watchers.ResourceWatcher {
			return NewGatewayClassHandler(dynamicClient, graphStore, logger, factory)
		},
	)

	// Register Gateway handler
	manager.RegisterHandlerFactory(
		"gateway",
		"gateway.networking.k8s.io",
		"Gateway",
		func() watchers.ResourceWatcher {
			return NewGatewayHandler(dynamicClient, graphStore, logger, factory)
		},
	)

	// Register HTTPRoute handler
	manager.RegisterHandlerFactory(
		"httproute",
		"gateway.networking.k8s.io",
		"HTTPRoute",
		func() watchers.ResourceWatcher {
			return NewHTTPRouteHandler(dynamicClient, graphStore, logger, factory)
		},
	)

	// Register GRPCRoute handler
	manager.RegisterHandlerFactory(
		"grpcroute",
		"gateway.networking.k8s.io",
		"GRPCRoute",
		func() watchers.ResourceWatcher {
			return NewGRPCRouteHandler(dynamicClient, graphStore, logger, factory)
		},
	)

	// Register TCPRoute handler
	manager.RegisterHandlerFactory(
		"tcproute",
		"gateway.networking.k8s.io",
		"TCPRoute",
		func() watchers.ResourceWatcher {
			return NewTCPRouteHandler(dynamicClient, graphStore, logger, factory)
		},
	)

	// Register UDPRoute handler
	manager.RegisterHandlerFactory(
		"udproute",
		"gateway.networking.k8s.io",
		"UDPRoute",
		func() watchers.ResourceWatcher {
			return NewUDPRouteHandler(dynamicClient, graphStore, logger, factory)
		},
	)

	// Register TLSRoute handler
	manager.RegisterHandlerFactory(
		"tlsroute",
		"gateway.networking.k8s.io",
		"TLSRoute",
		func() watchers.ResourceWatcher {
			return NewTLSRouteHandler(dynamicClient, graphStore, logger, factory)
		},
	)

	// Register ReferenceGrant handler
	manager.RegisterHandlerFactory(
		"referencegrant",
		"gateway.networking.k8s.io",
		"ReferenceGrant",
		func() watchers.ResourceWatcher {
			return NewReferenceGrantHandler(dynamicClient, graphStore, logger, factory)
		},
	)

	// Register BackendTLSPolicy handler
	manager.RegisterHandlerFactory(
		"backendtlspolicy",
		"gateway.networking.k8s.io",
		"BackendTLSPolicy",
		func() watchers.ResourceWatcher {
			return NewBackendTLSPolicyHandler(dynamicClient, graphStore, logger, factory)
		},
	)

	logger.Info("Gateway API handlers registered with CRD watcher")
}
