package kuadrant

import (
	"github.com/aslakknutsen/kkbase/pkg/graph"
	"github.com/aslakknutsen/kkbase/pkg/watchers"
	"github.com/aslakknutsen/kkbase/pkg/watchers/schema"
	"go.uber.org/zap"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/kubernetes"
)

// RegisterKuadrantHandlers registers version-agnostic Kuadrant handlers
// Handlers are created dynamically when CRDs become available
func RegisterKuadrantHandlers(
	manager *watchers.Manager,
	clientset *kubernetes.Clientset,
	dynamicClient dynamic.Interface,
	factory dynamicinformer.DynamicSharedInformerFactory,
	graphStore graph.GraphStore,
	logger *zap.Logger,
) {
	logger.Info("registering Kuadrant handlers (version-agnostic)")

	// Register AuthPolicy handler factory
	manager.RegisterHandlerFactory(
		watchers.ResourceTypeInfo{
			NodeType:      NodeTypeAuthPolicy,
			Kind:          "AuthPolicy",
			APIGroup:      "kuadrant.io",
			ClusterScoped: false,
		},
		func(crdInfo *watchers.CRDInfo) watchers.ResourceWatcher {
			extractor, err := schema.NewFieldExtractor(crdInfo.Schema, crdInfo.Version, AuthPolicyFieldRequirements)
			if err != nil {
				logger.Error("AuthPolicy CRD schema validation failed",
					zap.String("version", crdInfo.Version),
					zap.Error(err),
				)
				return nil
			}

			logger.Info("creating AuthPolicy handler",
				zap.String("version", crdInfo.Version),
				zap.Bool("has_authScheme", extractor.HasField("authScheme")),
			)

			return NewAuthPolicyHandler(clientset, dynamicClient, graphStore, logger, factory, extractor)
		},
	)

	// Register RateLimitPolicy handler factory
	manager.RegisterHandlerFactory(
		watchers.ResourceTypeInfo{
			NodeType:      NodeTypeRateLimitPolicy,
			Kind:          "RateLimitPolicy",
			APIGroup:      "kuadrant.io",
			ClusterScoped: false,
		},
		func(crdInfo *watchers.CRDInfo) watchers.ResourceWatcher {
			extractor, err := schema.NewFieldExtractor(crdInfo.Schema, crdInfo.Version, RateLimitPolicyFieldRequirements)
			if err != nil {
				logger.Error("RateLimitPolicy CRD schema validation failed",
					zap.String("version", crdInfo.Version),
					zap.Error(err),
				)
				return nil
			}

			logger.Info("creating RateLimitPolicy handler",
				zap.String("version", crdInfo.Version),
				zap.Bool("has_limits", extractor.HasField("limits")),
			)

			return NewRateLimitPolicyHandler(clientset, dynamicClient, graphStore, logger, factory, extractor)
		},
	)

	// Register DNSPolicy handler factory
	manager.RegisterHandlerFactory(
		watchers.ResourceTypeInfo{
			NodeType:      NodeTypeDNSPolicy,
			Kind:          "DNSPolicy",
			APIGroup:      "kuadrant.io",
			ClusterScoped: false,
		},
		func(crdInfo *watchers.CRDInfo) watchers.ResourceWatcher {
			extractor, err := schema.NewFieldExtractor(crdInfo.Schema, crdInfo.Version, DNSPolicyFieldRequirements)
			if err != nil {
				logger.Error("DNSPolicy CRD schema validation failed",
					zap.String("version", crdInfo.Version),
					zap.Error(err),
				)
				return nil
			}

			logger.Info("creating DNSPolicy handler", zap.String("version", crdInfo.Version))

			return NewDNSPolicyHandler(clientset, dynamicClient, graphStore, logger, factory, extractor)
		},
	)

	// Register TLSPolicy handler factory
	manager.RegisterHandlerFactory(
		watchers.ResourceTypeInfo{
			NodeType:      NodeTypeTLSPolicy,
			Kind:          "TLSPolicy",
			APIGroup:      "kuadrant.io",
			ClusterScoped: false,
		},
		func(crdInfo *watchers.CRDInfo) watchers.ResourceWatcher {
			extractor, err := schema.NewFieldExtractor(crdInfo.Schema, crdInfo.Version, TLSPolicyFieldRequirements)
			if err != nil {
				logger.Error("TLSPolicy CRD schema validation failed",
					zap.String("version", crdInfo.Version),
					zap.Error(err),
				)
				return nil
			}

			logger.Info("creating TLSPolicy handler", zap.String("version", crdInfo.Version))

			return NewTLSPolicyHandler(clientset, dynamicClient, graphStore, logger, factory, extractor)
		},
	)

	// Register Kuadrant CR handler factory
	manager.RegisterHandlerFactory(
		watchers.ResourceTypeInfo{
			NodeType:      NodeTypeKuadrant,
			Kind:          "Kuadrant",
			APIGroup:      "kuadrant.io",
			ClusterScoped: false,
		},
		func(crdInfo *watchers.CRDInfo) watchers.ResourceWatcher {
			extractor, err := schema.NewFieldExtractor(crdInfo.Schema, crdInfo.Version, KuadrantFieldRequirements)
			if err != nil {
				logger.Warn("Kuadrant CR schema validation failed - creating handler anyway",
					zap.String("version", crdInfo.Version),
					zap.Error(err),
				)
				// Kuadrant CR is mostly metadata, proceed with nil extractor
				extractor = nil
			}

			logger.Info("creating Kuadrant CR handler", zap.String("version", crdInfo.Version))

			return NewKuadrantHandler(clientset, dynamicClient, graphStore, logger, factory, extractor)
		},
	)

	logger.Info("Kuadrant handler registration complete")
}
