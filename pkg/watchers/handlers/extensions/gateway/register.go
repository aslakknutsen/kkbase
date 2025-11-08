package gateway

import (
	"github.com/kagenti/kkbase/pkg/graph"
	"github.com/kagenti/kkbase/pkg/watchers"
	"go.uber.org/zap"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/kubernetes"
)

// RegisterGatewayAPIHandlers registers Gateway API resource handlers with dynamic CRD detection
func RegisterGatewayAPIHandlers(
	manager *watchers.Manager,
	clientset *kubernetes.Clientset,
	dynamicClient dynamic.Interface,
	factory dynamicinformer.DynamicSharedInformerFactory,
	graphStore graph.GraphStore,
	logger *zap.Logger,
) {
	logger.Info("registering Gateway API handlers with CRD watcher")

	// Register GatewayClass handler
	manager.RegisterHandlerFactory(
		watchers.ResourceTypeInfo{
			NodeType:      NodeTypeGatewayClass,
			Kind:          "GatewayClass",
			APIGroup:      "gateway.networking.k8s.io",
			ClusterScoped: true,
		},
		func(_ *watchers.CRDInfo) watchers.ResourceWatcher {
			return NewGatewayClassHandler(clientset, dynamicClient, graphStore, logger, factory)
		},
	)

	// Register Gateway handler
	manager.RegisterHandlerFactory(
		watchers.ResourceTypeInfo{
			NodeType:      NodeTypeGateway,
			Kind:          "Gateway",
			APIGroup:      "gateway.networking.k8s.io",
			ClusterScoped: false,
		},
		func(_ *watchers.CRDInfo) watchers.ResourceWatcher {
			return NewGatewayHandler(clientset, dynamicClient, graphStore, logger, factory)
		},
	)

	// Register HTTPRoute handler
	manager.RegisterHandlerFactory(
		watchers.ResourceTypeInfo{
			NodeType:      NodeTypeHTTPRoute,
			Kind:          "HTTPRoute",
			APIGroup:      "gateway.networking.k8s.io",
			ClusterScoped: false,
		},
		func(_ *watchers.CRDInfo) watchers.ResourceWatcher {
			return NewHTTPRouteHandler(clientset, dynamicClient, graphStore, logger, factory)
		},
	)

	// Register GRPCRoute handler
	manager.RegisterHandlerFactory(
		watchers.ResourceTypeInfo{
			NodeType:      NodeTypeGRPCRoute,
			Kind:          "GRPCRoute",
			APIGroup:      "gateway.networking.k8s.io",
			ClusterScoped: false,
		},
		func(_ *watchers.CRDInfo) watchers.ResourceWatcher {
			return NewGRPCRouteHandler(clientset, dynamicClient, graphStore, logger, factory)
		},
	)

	// Register TCPRoute handler
	manager.RegisterHandlerFactory(
		watchers.ResourceTypeInfo{
			NodeType:      NodeTypeTCPRoute,
			Kind:          "TCPRoute",
			APIGroup:      "gateway.networking.k8s.io",
			ClusterScoped: false,
		},
		func(_ *watchers.CRDInfo) watchers.ResourceWatcher {
			return NewTCPRouteHandler(clientset, dynamicClient, graphStore, logger, factory)
		},
	)

	// Register UDPRoute handler
	manager.RegisterHandlerFactory(
		watchers.ResourceTypeInfo{
			NodeType:      NodeTypeUDPRoute,
			Kind:          "UDPRoute",
			APIGroup:      "gateway.networking.k8s.io",
			ClusterScoped: false,
		},
		func(_ *watchers.CRDInfo) watchers.ResourceWatcher {
			return NewUDPRouteHandler(clientset, dynamicClient, graphStore, logger, factory)
		},
	)

	// Register TLSRoute handler
	manager.RegisterHandlerFactory(
		watchers.ResourceTypeInfo{
			NodeType:      NodeTypeTLSRoute,
			Kind:          "TLSRoute",
			APIGroup:      "gateway.networking.k8s.io",
			ClusterScoped: false,
		},
		func(_ *watchers.CRDInfo) watchers.ResourceWatcher {
			return NewTLSRouteHandler(clientset, dynamicClient, graphStore, logger, factory)
		},
	)

	// Register ReferenceGrant handler
	manager.RegisterHandlerFactory(
		watchers.ResourceTypeInfo{
			NodeType:      NodeTypeReferenceGrant,
			Kind:          "ReferenceGrant",
			APIGroup:      "gateway.networking.k8s.io",
			ClusterScoped: false,
		},
		func(_ *watchers.CRDInfo) watchers.ResourceWatcher {
			return NewReferenceGrantHandler(clientset, dynamicClient, graphStore, logger, factory)
		},
	)

	// Register BackendTLSPolicy handler
	manager.RegisterHandlerFactory(
		watchers.ResourceTypeInfo{
			NodeType:      NodeTypeBackendTLSPolicy,
			Kind:          "BackendTLSPolicy",
			APIGroup:      "gateway.networking.k8s.io",
			ClusterScoped: false,
		},
		func(_ *watchers.CRDInfo) watchers.ResourceWatcher {
			return NewBackendTLSPolicyHandler(clientset, dynamicClient, graphStore, logger, factory)
		},
	)

	logger.Info("Gateway API handlers registered with CRD watcher")
}
