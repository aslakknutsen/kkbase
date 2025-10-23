package gateway

import (
	"github.com/kagenti/kkbase/pkg/graph"
	"github.com/kagenti/kkbase/pkg/watchers"
	"github.com/kagenti/kkbase/pkg/watchers/handlers"
	"go.uber.org/zap"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

// GatewayAPIAvailability tracks which Gateway API resources are available
type GatewayAPIAvailability struct {
	GatewayClass   bool
	Gateway        bool
	HTTPRoute      bool
	GRPCRoute      bool
	TCPRoute       bool
	UDPRoute       bool
	TLSRoute       bool
	ReferenceGrant bool
}

// CheckGatewayAPIAvailability checks which Gateway API CRDs are installed
func CheckGatewayAPIAvailability(config *rest.Config, logger *zap.Logger) *GatewayAPIAvailability {
	availability := &GatewayAPIAvailability{}

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		logger.Warn("failed to create discovery client for Gateway API check", zap.Error(err))
		return availability
	}

	// Check for v1 resources (stable API)
	v1Resources, err := discoveryClient.ServerResourcesForGroupVersion(gatewayv1.GroupVersion.String())
	if err == nil && v1Resources != nil {
		for _, resource := range v1Resources.APIResources {
			switch resource.Kind {
			case "GatewayClass":
				availability.GatewayClass = true
			case "Gateway":
				availability.Gateway = true
			case "HTTPRoute":
				availability.HTTPRoute = true
			case "GRPCRoute":
				availability.GRPCRoute = true
			}
		}
	}

	// Check for v1alpha2 resources (experimental API)
	v1alpha2Resources, err := discoveryClient.ServerResourcesForGroupVersion(gatewayv1alpha2.GroupVersion.String())
	if err == nil && v1alpha2Resources != nil {
		for _, resource := range v1alpha2Resources.APIResources {
			switch resource.Kind {
			case "TCPRoute":
				availability.TCPRoute = true
			case "UDPRoute":
				availability.UDPRoute = true
			case "TLSRoute":
				availability.TLSRoute = true
			}
		}
	}

	// Check for v1beta1 resources
	v1beta1Resources, err := discoveryClient.ServerResourcesForGroupVersion(gatewayv1beta1.GroupVersion.String())
	if err == nil && v1beta1Resources != nil {
		for _, resource := range v1beta1Resources.APIResources {
			switch resource.Kind {
			case "ReferenceGrant":
				availability.ReferenceGrant = true
			}
		}
	}

	logger.Info("Gateway API availability",
		zap.Bool("gatewayclass", availability.GatewayClass),
		zap.Bool("gateway", availability.Gateway),
		zap.Bool("httproute", availability.HTTPRoute),
		zap.Bool("grpcroute", availability.GRPCRoute),
		zap.Bool("tcproute", availability.TCPRoute),
		zap.Bool("udproute", availability.UDPRoute),
		zap.Bool("tlsroute", availability.TLSRoute),
		zap.Bool("referencegrant", availability.ReferenceGrant),
	)

	return availability
}

// RegisterGatewayAPIHandlers registers Gateway API resource handlers
// It checks for CRD availability and only registers handlers for installed CRDs
func RegisterGatewayAPIHandlers(
	registry *handlers.Registry,
	config *rest.Config,
	logger *zap.Logger,
) {
	// Check which Gateway API resources are available
	availability := CheckGatewayAPIAvailability(config, logger)

	// If no Gateway API resources are available, skip registration
	if !availability.GatewayClass && !availability.Gateway && !availability.HTTPRoute {
		logger.Info("Gateway API CRDs not found, skipping Gateway API handlers registration")
		return
	}

	logger.Info("registering Gateway API handlers")

	// Create dynamic client for CRD watching
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		logger.Error("failed to create dynamic client for Gateway API", zap.Error(err))
		return
	}

	// Register GatewayClass handler
	if availability.GatewayClass {
		registry.Register(&handlers.HandlerRegistration{
			Name:        "gatewayclass",
			Description: "Watches Gateway API GatewayClass resources",
			Category:    "gateway-api",
			Required:    false,
			Factory: func(clientset *kubernetes.Clientset, graphStore graph.GraphStore, logger *zap.Logger, informerFactory informers.SharedInformerFactory) watchers.ResourceWatcher {
				dynamicInformerFactory := dynamicinformer.NewDynamicSharedInformerFactory(dynamicClient, 0)
				return NewGatewayClassHandler(dynamicClient, graphStore, logger, dynamicInformerFactory)
			},
		})
	}

	// Register Gateway handler
	if availability.Gateway {
		registry.Register(&handlers.HandlerRegistration{
			Name:        "gateway",
			Description: "Watches Gateway API Gateway resources",
			Category:    "gateway-api",
			Required:    false,
			Factory: func(clientset *kubernetes.Clientset, graphStore graph.GraphStore, logger *zap.Logger, informerFactory informers.SharedInformerFactory) watchers.ResourceWatcher {
				dynamicInformerFactory := dynamicinformer.NewDynamicSharedInformerFactory(dynamicClient, 0)
				return NewGatewayHandler(dynamicClient, graphStore, logger, dynamicInformerFactory)
			},
		})
	}

	// Register HTTPRoute handler
	if availability.HTTPRoute {
		registry.Register(&handlers.HandlerRegistration{
			Name:        "httproute",
			Description: "Watches Gateway API HTTPRoute resources",
			Category:    "gateway-api",
			Required:    false,
			Factory: func(clientset *kubernetes.Clientset, graphStore graph.GraphStore, logger *zap.Logger, informerFactory informers.SharedInformerFactory) watchers.ResourceWatcher {
				dynamicInformerFactory := dynamicinformer.NewDynamicSharedInformerFactory(dynamicClient, 0)
				return NewHTTPRouteHandler(dynamicClient, graphStore, logger, dynamicInformerFactory)
			},
		})
	}

	// Register GRPCRoute handler
	if availability.GRPCRoute {
		registry.Register(&handlers.HandlerRegistration{
			Name:        "grpcroute",
			Description: "Watches Gateway API GRPCRoute resources",
			Category:    "gateway-api",
			Required:    false,
			Factory: func(clientset *kubernetes.Clientset, graphStore graph.GraphStore, logger *zap.Logger, informerFactory informers.SharedInformerFactory) watchers.ResourceWatcher {
				dynamicInformerFactory := dynamicinformer.NewDynamicSharedInformerFactory(dynamicClient, 0)
				return NewGRPCRouteHandler(dynamicClient, graphStore, logger, dynamicInformerFactory)
			},
		})
	}

	// Register TCPRoute handler
	if availability.TCPRoute {
		registry.Register(&handlers.HandlerRegistration{
			Name:        "tcproute",
			Description: "Watches Gateway API TCPRoute resources",
			Category:    "gateway-api",
			Required:    false,
			Factory: func(clientset *kubernetes.Clientset, graphStore graph.GraphStore, logger *zap.Logger, informerFactory informers.SharedInformerFactory) watchers.ResourceWatcher {
				dynamicInformerFactory := dynamicinformer.NewDynamicSharedInformerFactory(dynamicClient, 0)
				return NewTCPRouteHandler(dynamicClient, graphStore, logger, dynamicInformerFactory)
			},
		})
	}

	// Register UDPRoute handler
	if availability.UDPRoute {
		registry.Register(&handlers.HandlerRegistration{
			Name:        "udproute",
			Description: "Watches Gateway API UDPRoute resources",
			Category:    "gateway-api",
			Required:    false,
			Factory: func(clientset *kubernetes.Clientset, graphStore graph.GraphStore, logger *zap.Logger, informerFactory informers.SharedInformerFactory) watchers.ResourceWatcher {
				dynamicInformerFactory := dynamicinformer.NewDynamicSharedInformerFactory(dynamicClient, 0)
				return NewUDPRouteHandler(dynamicClient, graphStore, logger, dynamicInformerFactory)
			},
		})
	}

	// Register TLSRoute handler
	if availability.TLSRoute {
		registry.Register(&handlers.HandlerRegistration{
			Name:        "tlsroute",
			Description: "Watches Gateway API TLSRoute resources",
			Category:    "gateway-api",
			Required:    false,
			Factory: func(clientset *kubernetes.Clientset, graphStore graph.GraphStore, logger *zap.Logger, informerFactory informers.SharedInformerFactory) watchers.ResourceWatcher {
				dynamicInformerFactory := dynamicinformer.NewDynamicSharedInformerFactory(dynamicClient, 0)
				return NewTLSRouteHandler(dynamicClient, graphStore, logger, dynamicInformerFactory)
			},
		})
	}

	// Register ReferenceGrant handler
	if availability.ReferenceGrant {
		registry.Register(&handlers.HandlerRegistration{
			Name:        "referencegrant",
			Description: "Watches Gateway API ReferenceGrant resources",
			Category:    "gateway-api",
			Required:    false,
			Factory: func(clientset *kubernetes.Clientset, graphStore graph.GraphStore, logger *zap.Logger, informerFactory informers.SharedInformerFactory) watchers.ResourceWatcher {
				dynamicInformerFactory := dynamicinformer.NewDynamicSharedInformerFactory(dynamicClient, 0)
				return NewReferenceGrantHandler(dynamicClient, graphStore, logger, dynamicInformerFactory)
			},
		})
	}
}
