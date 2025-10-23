package gateway

import (
	"context"
	"fmt"

	"github.com/kagenti/kkbase/pkg/graph"
	"github.com/kagenti/kkbase/pkg/models"
	"github.com/kagenti/kkbase/pkg/watchers"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// GatewayClassHandler handles GatewayClass resources
type GatewayClassHandler struct {
	*watchers.BaseWatcher
	dynamicClient       dynamic.Interface
	relationshipBuilder *watchers.RelationshipBuilder
}

// NewGatewayClassHandler creates a new GatewayClass handler
func NewGatewayClassHandler(
	dynamicClient dynamic.Interface,
	graphStore graph.GraphStore,
	logger *zap.Logger,
	dynamicInformerFactory dynamicinformer.DynamicSharedInformerFactory,
) *GatewayClassHandler {
	gvr := gatewayv1.SchemeGroupVersion.WithResource("gatewayclasses")
	informer := dynamicInformerFactory.ForResource(gvr).Informer()

	handler := &GatewayClassHandler{
		BaseWatcher:         watchers.NewBaseWatcher(graphStore, logger, informer),
		dynamicClient:       dynamicClient,
		relationshipBuilder: watchers.NewRelationshipBuilder(nil, graphStore, logger),
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
	unstructuredObj, ok := obj.(*unstructured.Unstructured)
	if !ok {
		h.Logger.Error("unexpected object type", zap.String("type", fmt.Sprintf("%T", obj)))
		return
	}

	gatewayClass := &gatewayv1.GatewayClass{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.Object, gatewayClass); err != nil {
		h.Logger.Error("failed to convert to GatewayClass", zap.Error(err))
		return
	}

	h.Logger.Debug("gatewayclass added",
		zap.String("name", gatewayClass.Name),
		zap.String("controller", string(gatewayClass.Spec.ControllerName)),
	)

	ctx := context.Background()

	// Create GatewayClass node
	gatewayClassNode := models.GatewayClassToGraphNode(gatewayClass)
	if err := h.GraphStore.UpsertNode(ctx, string(gatewayClassNode.Type), gatewayClassNode.ID, gatewayClassNode.Properties); err != nil {
		h.Logger.Error("failed to create gatewayclass node", zap.Error(err), zap.String("name", gatewayClass.Name))
		return
	}
}

// HandleUpdate processes an updated GatewayClass
func (h *GatewayClassHandler) HandleUpdate(oldObj, newObj interface{}) {
	unstructuredObj, ok := newObj.(*unstructured.Unstructured)
	if !ok {
		h.Logger.Error("unexpected object type", zap.String("type", fmt.Sprintf("%T", newObj)))
		return
	}

	gatewayClass := &gatewayv1.GatewayClass{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.Object, gatewayClass); err != nil {
		h.Logger.Error("failed to convert to GatewayClass", zap.Error(err))
		return
	}

	h.Logger.Debug("gatewayclass updated", zap.String("name", gatewayClass.Name))

	// For GatewayClass, just update the node
	h.HandleAdd(newObj)
}

// HandleDelete processes a deleted GatewayClass
func (h *GatewayClassHandler) HandleDelete(obj interface{}) {
	unstructuredObj, ok := obj.(*unstructured.Unstructured)
	if !ok {
		extracted, err := watchers.SafeGetObject(obj)
		if err != nil {
			h.Logger.Error("failed to extract object", zap.Error(err))
			return
		}
		unstructuredObj, ok = extracted.(*unstructured.Unstructured)
		if !ok {
			h.Logger.Error("unexpected object type", zap.String("type", fmt.Sprintf("%T", extracted)))
			return
		}
	}

	gatewayClass := &gatewayv1.GatewayClass{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.Object, gatewayClass); err != nil {
		h.Logger.Error("failed to convert to GatewayClass", zap.Error(err))
		return
	}

	h.Logger.Debug("gatewayclass deleted", zap.String("name", gatewayClass.Name))

	ctx := context.Background()

	if err := h.GraphStore.DeleteNode(ctx, string(models.NodeTypeGatewayClass), gatewayClass.Name); err != nil {
		h.Logger.Error("failed to delete gatewayclass node", zap.Error(err), zap.String("name", gatewayClass.Name))
	}
}
