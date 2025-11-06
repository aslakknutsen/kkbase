package gateway

import (
	"context"

	"github.com/kagenti/kkbase/pkg/graph"
	"github.com/kagenti/kkbase/pkg/watchers"
	"go.uber.org/zap"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// GatewayClassHandler handles GatewayClass resources
type GatewayClassHandler struct {
	*watchers.BaseWatcher
	dynamicClient       dynamic.Interface
	relationshipBuilder *RelationshipBuilder
}

// NewGatewayClassHandler creates a new GatewayClass handler
func NewGatewayClassHandler(
	clientset *kubernetes.Clientset,
	dynamicClient dynamic.Interface,
	graphStore graph.GraphStore,
	logger *zap.Logger,
	factory dynamicinformer.DynamicSharedInformerFactory,
) *GatewayClassHandler {
	gvr := gatewayv1.SchemeGroupVersion.WithResource("gatewayclasses")
	informer := factory.ForResource(gvr).Informer()

	handler := &GatewayClassHandler{
		BaseWatcher:         watchers.NewBaseWatcher(graphStore, logger, informer),
		dynamicClient:       dynamicClient,
		relationshipBuilder: NewRelationshipBuilder(clientset, graphStore, logger),
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

// HandleAdd processes a newly added GatewayClass
func (h *GatewayClassHandler) HandleAdd(obj interface{}) {
	gatewayClass, err := watchers.ConvertToTyped[gatewayv1.GatewayClass](obj)

	if err != nil {

		h.Logger.Error("failed to convert to GatewayClass", zap.Error(err))

		return

	}

	h.Logger.Debug("gatewayclass added",
		zap.String("name", gatewayClass.Name),
		zap.String("controller", string(gatewayClass.Spec.ControllerName)),
	)

	ctx := context.Background()

	// Create GatewayClass node
	gatewayClassNode := GatewayClassToGraphNode(gatewayClass)
	if err := h.GraphStore.UpsertNode(ctx, string(gatewayClassNode.Type), gatewayClassNode.ID, gatewayClassNode.Properties); err != nil {
		h.Logger.Error("failed to create gatewayclass node", zap.Error(err), zap.String("name", gatewayClass.Name))
		return
	}
}

// HandleUpdate processes an updated GatewayClass
func (h *GatewayClassHandler) HandleUpdate(oldObj, newObj interface{}) {
	gatewayClass, err := watchers.ConvertToTyped[gatewayv1.GatewayClass](newObj)

	if err != nil {

		h.Logger.Error("failed to convert to GatewayClass", zap.Error(err))

		return

	}

	h.Logger.Debug("gatewayclass updated", zap.String("name", gatewayClass.Name))

	// For GatewayClass, just update the node
	h.HandleAdd(newObj)
}

// HandleDelete processes a deleted GatewayClass
func (h *GatewayClassHandler) HandleDelete(obj interface{}) {
	gatewayClass, err := watchers.ConvertToTyped[gatewayv1.GatewayClass](obj)

	if err != nil {

		h.Logger.Error("failed to convert to GatewayClass", zap.Error(err))

		return

	}

	h.Logger.Debug("gatewayclass deleted", zap.String("name", gatewayClass.Name))

	ctx := context.Background()

	if err := h.GraphStore.DeleteNode(ctx, string(NodeTypeGatewayClass), gatewayClass.Name); err != nil {
		h.Logger.Error("failed to delete gatewayclass node", zap.Error(err), zap.String("name", gatewayClass.Name))
	}
}
