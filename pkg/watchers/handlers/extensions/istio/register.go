package istio

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

	istiov1 "istio.io/client-go/pkg/apis/networking/v1"
	istiosecurityv1 "istio.io/client-go/pkg/apis/security/v1"
)

// IstioAvailability tracks which Istio resources are available
type IstioAvailability struct {
	Gateway               bool
	VirtualService        bool
	DestinationRule       bool
	ServiceEntry          bool
	Sidecar               bool
	AuthorizationPolicy   bool
	PeerAuthentication    bool
	RequestAuthentication bool
}

// CheckIstioAvailability checks which Istio CRDs are installed
func CheckIstioAvailability(config *rest.Config, logger *zap.Logger) *IstioAvailability {
	availability := &IstioAvailability{}

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		logger.Warn("failed to create discovery client for Istio check", zap.Error(err))
		return availability
	}

	// Check for Istio networking resources (v1)
	networkingResources, err := discoveryClient.ServerResourcesForGroupVersion(istiov1.SchemeGroupVersion.String())
	if err == nil && networkingResources != nil {
		for _, resource := range networkingResources.APIResources {
			switch resource.Kind {
			case "Gateway":
				availability.Gateway = true
			case "VirtualService":
				availability.VirtualService = true
			case "DestinationRule":
				availability.DestinationRule = true
			case "ServiceEntry":
				availability.ServiceEntry = true
			case "Sidecar":
				availability.Sidecar = true
			}
		}
	}

	// Check for Istio security resources (v1)
	securityResources, err := discoveryClient.ServerResourcesForGroupVersion(istiosecurityv1.SchemeGroupVersion.String())
	if err == nil && securityResources != nil {
		for _, resource := range securityResources.APIResources {
			switch resource.Kind {
			case "AuthorizationPolicy":
				availability.AuthorizationPolicy = true
			case "PeerAuthentication":
				availability.PeerAuthentication = true
			case "RequestAuthentication":
				availability.RequestAuthentication = true
			}
		}
	}

	logger.Info("Istio availability",
		zap.Bool("gateway", availability.Gateway),
		zap.Bool("virtualservice", availability.VirtualService),
		zap.Bool("destinationrule", availability.DestinationRule),
		zap.Bool("serviceentry", availability.ServiceEntry),
		zap.Bool("sidecar", availability.Sidecar),
		zap.Bool("authorizationpolicy", availability.AuthorizationPolicy),
		zap.Bool("peerauthentication", availability.PeerAuthentication),
		zap.Bool("requestauthentication", availability.RequestAuthentication),
	)

	return availability
}

// RegisterIstioHandlers registers Istio resource handlers
// It checks for CRD availability and only registers handlers for installed CRDs
func RegisterIstioHandlers(
	registry *handlers.Registry,
	config *rest.Config,
	logger *zap.Logger,
) {
	// Check which Istio resources are available
	availability := CheckIstioAvailability(config, logger)

	// If no Istio resources are available, skip registration
	if !availability.Gateway && !availability.VirtualService && !availability.DestinationRule &&
		!availability.ServiceEntry && !availability.Sidecar &&
		!availability.AuthorizationPolicy && !availability.PeerAuthentication && !availability.RequestAuthentication {
		logger.Info("Istio CRDs not found, skipping Istio handlers registration")
		return
	}

	logger.Info("registering Istio handlers")

	// Create dynamic client for CRD watching
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		logger.Error("failed to create dynamic client for Istio", zap.Error(err))
		return
	}

	// Register Istio Gateway handler
	if availability.Gateway {
		registry.Register(&handlers.HandlerRegistration{
			Name:        "istio-gateway",
			Description: "Watches Istio Gateway resources",
			Category:    "istio",
			Required:    false,
			Factory: func(clientset *kubernetes.Clientset, graphStore graph.GraphStore, logger *zap.Logger, informerFactory informers.SharedInformerFactory) watchers.ResourceWatcher {
				dynamicInformerFactory := dynamicinformer.NewDynamicSharedInformerFactory(dynamicClient, 0)
				return NewIstioGatewayHandler(clientset, dynamicClient, graphStore, logger, dynamicInformerFactory)
			},
		})
	}

	// Register VirtualService handler
	if availability.VirtualService {
		registry.Register(&handlers.HandlerRegistration{
			Name:        "virtualservice",
			Description: "Watches Istio VirtualService resources",
			Category:    "istio",
			Required:    false,
			Factory: func(clientset *kubernetes.Clientset, graphStore graph.GraphStore, logger *zap.Logger, informerFactory informers.SharedInformerFactory) watchers.ResourceWatcher {
				dynamicInformerFactory := dynamicinformer.NewDynamicSharedInformerFactory(dynamicClient, 0)
				return NewVirtualServiceHandler(clientset, dynamicClient, graphStore, logger, dynamicInformerFactory)
			},
		})
	}

	// Register DestinationRule handler
	if availability.DestinationRule {
		registry.Register(&handlers.HandlerRegistration{
			Name:        "destinationrule",
			Description: "Watches Istio DestinationRule resources",
			Category:    "istio",
			Required:    false,
			Factory: func(clientset *kubernetes.Clientset, graphStore graph.GraphStore, logger *zap.Logger, informerFactory informers.SharedInformerFactory) watchers.ResourceWatcher {
				dynamicInformerFactory := dynamicinformer.NewDynamicSharedInformerFactory(dynamicClient, 0)
				return NewDestinationRuleHandler(clientset, dynamicClient, graphStore, logger, dynamicInformerFactory)
			},
		})
	}

	// Register ServiceEntry handler
	if availability.ServiceEntry {
		registry.Register(&handlers.HandlerRegistration{
			Name:        "serviceentry",
			Description: "Watches Istio ServiceEntry resources",
			Category:    "istio",
			Required:    false,
			Factory: func(clientset *kubernetes.Clientset, graphStore graph.GraphStore, logger *zap.Logger, informerFactory informers.SharedInformerFactory) watchers.ResourceWatcher {
				dynamicInformerFactory := dynamicinformer.NewDynamicSharedInformerFactory(dynamicClient, 0)
				return NewServiceEntryHandler(clientset, dynamicClient, graphStore, logger, dynamicInformerFactory)
			},
		})
	}

	// Register Sidecar handler
	if availability.Sidecar {
		registry.Register(&handlers.HandlerRegistration{
			Name:        "sidecar",
			Description: "Watches Istio Sidecar resources",
			Category:    "istio",
			Required:    false,
			Factory: func(clientset *kubernetes.Clientset, graphStore graph.GraphStore, logger *zap.Logger, informerFactory informers.SharedInformerFactory) watchers.ResourceWatcher {
				dynamicInformerFactory := dynamicinformer.NewDynamicSharedInformerFactory(dynamicClient, 0)
				return NewSidecarHandler(clientset, dynamicClient, graphStore, logger, dynamicInformerFactory)
			},
		})
	}

	// Register AuthorizationPolicy handler
	if availability.AuthorizationPolicy {
		registry.Register(&handlers.HandlerRegistration{
			Name:        "authorizationpolicy",
			Description: "Watches Istio AuthorizationPolicy resources",
			Category:    "istio",
			Required:    false,
			Factory: func(clientset *kubernetes.Clientset, graphStore graph.GraphStore, logger *zap.Logger, informerFactory informers.SharedInformerFactory) watchers.ResourceWatcher {
				dynamicInformerFactory := dynamicinformer.NewDynamicSharedInformerFactory(dynamicClient, 0)
				return NewAuthorizationPolicyHandler(clientset, dynamicClient, graphStore, logger, dynamicInformerFactory)
			},
		})
	}

	// Register PeerAuthentication handler
	if availability.PeerAuthentication {
		registry.Register(&handlers.HandlerRegistration{
			Name:        "peerauthentication",
			Description: "Watches Istio PeerAuthentication resources",
			Category:    "istio",
			Required:    false,
			Factory: func(clientset *kubernetes.Clientset, graphStore graph.GraphStore, logger *zap.Logger, informerFactory informers.SharedInformerFactory) watchers.ResourceWatcher {
				dynamicInformerFactory := dynamicinformer.NewDynamicSharedInformerFactory(dynamicClient, 0)
				return NewPeerAuthenticationHandler(clientset, dynamicClient, graphStore, logger, dynamicInformerFactory)
			},
		})
	}

	// Register RequestAuthentication handler
	if availability.RequestAuthentication {
		registry.Register(&handlers.HandlerRegistration{
			Name:        "requestauthentication",
			Description: "Watches Istio RequestAuthentication resources",
			Category:    "istio",
			Required:    false,
			Factory: func(clientset *kubernetes.Clientset, graphStore graph.GraphStore, logger *zap.Logger, informerFactory informers.SharedInformerFactory) watchers.ResourceWatcher {
				dynamicInformerFactory := dynamicinformer.NewDynamicSharedInformerFactory(dynamicClient, 0)
				return NewRequestAuthenticationHandler(clientset, dynamicClient, graphStore, logger, dynamicInformerFactory)
			},
		})
	}
}
