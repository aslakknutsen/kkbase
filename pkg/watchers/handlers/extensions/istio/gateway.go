package istio

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
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	istiov1 "istio.io/client-go/pkg/apis/networking/v1"
)

// IstioGatewayHandler handles Istio Gateway resources
type IstioGatewayHandler struct {
	*watchers.BaseWatcher
	clientset           *kubernetes.Clientset
	dynamicClient       dynamic.Interface
	relationshipBuilder *watchers.RelationshipBuilder
}

// NewIstioGatewayHandler creates a new Istio Gateway handler
func NewIstioGatewayHandler(
	clientset *kubernetes.Clientset,
	dynamicClient dynamic.Interface,
	graphStore graph.GraphStore,
	logger *zap.Logger,
	dynamicInformerFactory dynamicinformer.DynamicSharedInformerFactory,
) *IstioGatewayHandler {
	gvr := istiov1.SchemeGroupVersion.WithResource("gateways")
	informer := dynamicInformerFactory.ForResource(gvr).Informer()

	handler := &IstioGatewayHandler{
		BaseWatcher:         watchers.NewBaseWatcher(graphStore, logger, informer),
		clientset:           clientset,
		dynamicClient:       dynamicClient,
		relationshipBuilder: watchers.NewRelationshipBuilder(clientset, graphStore, logger),
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

// HandleAdd processes a newly added Istio Gateway
func (h *IstioGatewayHandler) HandleAdd(obj interface{}) {
	unstructuredObj, ok := obj.(*unstructured.Unstructured)
	if !ok {
		h.Logger.Error("unexpected object type", zap.String("type", fmt.Sprintf("%T", obj)))
		return
	}

	gateway := &istiov1.Gateway{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.Object, gateway); err != nil {
		h.Logger.Error("failed to convert to Istio Gateway", zap.Error(err))
		return
	}

	h.Logger.Debug("istio gateway added",
		zap.String("namespace", gateway.Namespace),
		zap.String("name", gateway.Name),
	)

	ctx := context.Background()

	// Create Gateway node
	gatewayNode := models.IstioGatewayToGraphNode(gateway)
	if err := h.GraphStore.UpsertNode(ctx, string(gatewayNode.Type), gatewayNode.ID, gatewayNode.Properties); err != nil {
		h.Logger.Error("failed to create istio gateway node", zap.Error(err), zap.String("gateway", gateway.Name))
		return
	}

	// Create IN_NAMESPACE edge
	if err := h.relationshipBuilder.CreateNamespaceEdge(ctx, models.NodeTypeIstioGateway, gatewayNode.ID, gateway.Namespace); err != nil {
		h.Logger.Error("failed to create namespace edge", zap.Error(err))
	}

	// Create SELECTS_PROXY edges to Pods
	if gateway.Spec.Selector != nil && len(gateway.Spec.Selector) > 0 {
		if err := h.relationshipBuilder.CreateIstioGatewaySelectsProxyEdge(ctx, gateway.Namespace, gateway.Name, gateway.Spec.Selector); err != nil {
			h.Logger.Error("failed to create SELECTS_PROXY edges", zap.Error(err))
		}
	}
}

// HandleUpdate processes an updated Istio Gateway
func (h *IstioGatewayHandler) HandleUpdate(oldObj, newObj interface{}) {
	unstructuredObj, ok := newObj.(*unstructured.Unstructured)
	if !ok {
		h.Logger.Error("unexpected object type", zap.String("type", fmt.Sprintf("%T", newObj)))
		return
	}

	gateway := &istiov1.Gateway{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.Object, gateway); err != nil {
		h.Logger.Error("failed to convert to Istio Gateway", zap.Error(err))
		return
	}

	h.Logger.Debug("istio gateway updated",
		zap.String("namespace", gateway.Namespace),
		zap.String("name", gateway.Name),
	)

	ctx := context.Background()
	gatewayID := models.GetNodeID("IstioGateway", gateway.Namespace, gateway.Name)

	// Delete old edges
	if err := h.GraphStore.DeleteEdgesByNode(ctx, string(models.NodeTypeIstioGateway), gatewayID); err != nil {
		h.Logger.Error("failed to delete old istio gateway edges", zap.Error(err))
	}

	// Recreate with new data
	h.HandleAdd(newObj)
}

// HandleDelete processes a deleted Istio Gateway
func (h *IstioGatewayHandler) HandleDelete(obj interface{}) {
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

	gateway := &istiov1.Gateway{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.Object, gateway); err != nil {
		h.Logger.Error("failed to convert to Istio Gateway", zap.Error(err))
		return
	}

	h.Logger.Debug("istio gateway deleted",
		zap.String("namespace", gateway.Namespace),
		zap.String("name", gateway.Name),
	)

	ctx := context.Background()

	gatewayID := models.GetNodeID("IstioGateway", gateway.Namespace, gateway.Name)
	if err := h.GraphStore.DeleteNode(ctx, string(models.NodeTypeIstioGateway), gatewayID); err != nil {
		h.Logger.Error("failed to delete istio gateway node", zap.Error(err), zap.String("gateway", gateway.Name))
	}
}
