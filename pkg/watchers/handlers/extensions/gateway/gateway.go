package gateway

import (
	"context"

	"github.com/kagenti/kkbase/pkg/graph"
	"github.com/kagenti/kkbase/pkg/models"
	"github.com/kagenti/kkbase/pkg/watchers"
	"go.uber.org/zap"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// GatewayHandler handles Gateway resources
type GatewayHandler struct {
	*watchers.BaseWatcher
	dynamicClient       dynamic.Interface
	relationshipBuilder *RelationshipBuilder
}

// NewGatewayHandler creates a new Gateway handler
func NewGatewayHandler(
	dynamicClient dynamic.Interface,
	graphStore graph.GraphStore,
	logger *zap.Logger,

	factory dynamicinformer.DynamicSharedInformerFactory,
) *GatewayHandler {
	gvr := gatewayv1.SchemeGroupVersion.WithResource("gateways")
	informer := factory.ForResource(gvr).Informer()

	handler := &GatewayHandler{
		BaseWatcher:   watchers.NewBaseWatcher(graphStore, logger, informer),
		dynamicClient: dynamicClient, relationshipBuilder: NewRelationshipBuilder(nil, graphStore, logger),
	}

	_, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    handler.HandleAdd,
		UpdateFunc: handler.HandleUpdate,
		DeleteFunc: handler.HandleDelete,
	})
	if err != nil {
		logger.Error("failed to add event handler", zap.Error(err))
	}

	return handler
}

// HandleAdd processes a newly added Gateway
func (h *GatewayHandler) HandleAdd(obj interface{}) {
	gateway, err := watchers.ConvertToTyped[gatewayv1.Gateway](obj)
	if err != nil {
		h.Logger.Error("failed to convert to Gateway", zap.Error(err))
		return
	}

	h.Logger.Debug("gateway added",
		zap.String("namespace", gateway.Namespace),
		zap.String("name", gateway.Name),
		zap.String("gatewayClass", string(gateway.Spec.GatewayClassName)),
	)

	ctx := context.Background()

	// Create Gateway node
	gatewayNode := GatewayToGraphNode(gateway)
	if err := h.GraphStore.UpsertNode(ctx, string(gatewayNode.Type), gatewayNode.ID, gatewayNode.Properties); err != nil {
		h.Logger.Error("failed to create gateway node", zap.Error(err), zap.String("gateway", gateway.Name))
		return
	}

	// Create IN_NAMESPACE edge
	if err := h.relationshipBuilder.base.CreateNamespaceEdge(ctx, NodeTypeGateway, gatewayNode.ID, gateway.Namespace); err != nil {
		h.Logger.Error("failed to create namespace edge", zap.Error(err))
	}

	// Create IMPLEMENTED_BY edge to GatewayClass
	if err := h.relationshipBuilder.CreateGatewayImplementedByEdge(ctx, gateway.Namespace, gateway.Name, string(gateway.Spec.GatewayClassName)); err != nil {
		h.Logger.Error("failed to create IMPLEMENTED_BY edge", zap.Error(err))
	}

	// Create USES_TLS_FROM edges to Secrets for TLS listeners
	for _, listener := range gateway.Spec.Listeners {
		if listener.TLS != nil && len(listener.TLS.CertificateRefs) > 0 {
			for _, certRef := range listener.TLS.CertificateRefs {
				secretNamespace := gateway.Namespace
				if certRef.Namespace != nil {
					secretNamespace = string(*certRef.Namespace)
				}
				secretName := string(certRef.Name)

				if err := h.relationshipBuilder.CreateGatewayTLSEdge(
					ctx,
					gateway.Namespace,
					gateway.Name,
					secretNamespace,
					secretName,
					string(listener.Name),
				); err != nil {
					h.Logger.Error("failed to create USES_TLS_FROM edge",
						zap.Error(err),
						zap.String("secret", secretName),
						zap.String("listener", string(listener.Name)),
					)
				}
			}
		}
	}
}

// HandleUpdate processes an updated Gateway
func (h *GatewayHandler) HandleUpdate(oldObj, newObj interface{}) {
	gateway, err := watchers.ConvertToTyped[gatewayv1.Gateway](newObj)

	if err != nil {

		h.Logger.Error("failed to convert to Gateway", zap.Error(err))

		return

	}

	h.Logger.Debug("gateway updated",
		zap.String("namespace", gateway.Namespace),
		zap.String("name", gateway.Name),
	)

	ctx := context.Background()
	gatewayID := models.GetNodeID("Gateway", gateway.Namespace, gateway.Name)

	// Delete old edges
	if err := h.GraphStore.DeleteEdgesByNode(ctx, string(NodeTypeGateway), gatewayID); err != nil {
		h.Logger.Error("failed to delete old gateway edges", zap.Error(err))
	}

	// Recreate with new data
	h.HandleAdd(newObj)
}

// HandleDelete processes a deleted Gateway
func (h *GatewayHandler) HandleDelete(obj interface{}) {
	gateway, err := watchers.ConvertToTyped[gatewayv1.Gateway](obj)

	if err != nil {

		h.Logger.Error("failed to convert to Gateway", zap.Error(err))

		return

	}

	h.Logger.Debug("gateway deleted",
		zap.String("namespace", gateway.Namespace),
		zap.String("name", gateway.Name),
	)

	ctx := context.Background()

	gatewayID := models.GetNodeID("Gateway", gateway.Namespace, gateway.Name)
	if err := h.GraphStore.DeleteNode(ctx, string(NodeTypeGateway), gatewayID); err != nil {
		h.Logger.Error("failed to delete gateway node", zap.Error(err), zap.String("gateway", gateway.Name))
	}
}
