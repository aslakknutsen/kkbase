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

	istiosecurityv1 "istio.io/client-go/pkg/apis/security/v1"
)

// RequestAuthenticationHandler handles Istio RequestAuthentication resources
type RequestAuthenticationHandler struct {
	*watchers.BaseWatcher
	clientset           *kubernetes.Clientset
	dynamicClient       dynamic.Interface
	relationshipBuilder *watchers.RelationshipBuilder
}

// NewRequestAuthenticationHandler creates a new RequestAuthentication handler
func NewRequestAuthenticationHandler(
	clientset *kubernetes.Clientset,
	dynamicClient dynamic.Interface,
	graphStore graph.GraphStore,
	logger *zap.Logger,
	dynamicInformerFactory dynamicinformer.DynamicSharedInformerFactory,
) *RequestAuthenticationHandler {
	gvr := istiosecurityv1.SchemeGroupVersion.WithResource("requestauthentications")
	informer := dynamicInformerFactory.ForResource(gvr).Informer()

	handler := &RequestAuthenticationHandler{
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

// HandleAdd processes a newly added RequestAuthentication
func (h *RequestAuthenticationHandler) HandleAdd(obj interface{}) {
	unstructuredObj, ok := obj.(*unstructured.Unstructured)
	if !ok {
		h.Logger.Error("unexpected object type", zap.String("type", fmt.Sprintf("%T", obj)))
		return
	}

	ra := &istiosecurityv1.RequestAuthentication{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.Object, ra); err != nil {
		h.Logger.Error("failed to convert to RequestAuthentication", zap.Error(err))
		return
	}

	h.Logger.Debug("requestauthentication added",
		zap.String("namespace", ra.Namespace),
		zap.String("name", ra.Name),
	)

	ctx := context.Background()

	// Create RequestAuthentication node
	raNode := models.RequestAuthenticationToGraphNode(ra)
	if err := h.GraphStore.UpsertNode(ctx, string(raNode.Type), raNode.ID, raNode.Properties); err != nil {
		h.Logger.Error("failed to create requestauthentication node", zap.Error(err), zap.String("requestauthentication", ra.Name))
		return
	}

	// Create IN_NAMESPACE edge
	if err := h.relationshipBuilder.CreateNamespaceEdge(ctx, models.NodeTypeRequestAuthentication, raNode.ID, ra.Namespace); err != nil {
		h.Logger.Error("failed to create namespace edge", zap.Error(err))
	}

	// Create APPLIES_TO edges to Pods
	selector := make(map[string]string)
	if ra.Spec.Selector != nil && len(ra.Spec.Selector.MatchLabels) > 0 {
		selector = ra.Spec.Selector.MatchLabels
	}

	if err := h.relationshipBuilder.CreateIstioPolicyAppliesToEdge(
		ctx, models.NodeTypeRequestAuthentication, ra.Namespace, ra.Name, selector, nil); err != nil {
		h.Logger.Error("failed to create APPLIES_TO edges", zap.Error(err))
	}
}

// HandleUpdate processes an updated RequestAuthentication
func (h *RequestAuthenticationHandler) HandleUpdate(oldObj, newObj interface{}) {
	unstructuredObj, ok := newObj.(*unstructured.Unstructured)
	if !ok {
		h.Logger.Error("unexpected object type", zap.String("type", fmt.Sprintf("%T", newObj)))
		return
	}

	ra := &istiosecurityv1.RequestAuthentication{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.Object, ra); err != nil {
		h.Logger.Error("failed to convert to RequestAuthentication", zap.Error(err))
		return
	}

	h.Logger.Debug("requestauthentication updated",
		zap.String("namespace", ra.Namespace),
		zap.String("name", ra.Name),
	)

	ctx := context.Background()
	raID := models.GetNodeID("RequestAuthentication", ra.Namespace, ra.Name)

	// Delete old edges
	if err := h.GraphStore.DeleteEdgesByNode(ctx, string(models.NodeTypeRequestAuthentication), raID); err != nil {
		h.Logger.Error("failed to delete old requestauthentication edges", zap.Error(err))
	}

	// Recreate with new data
	h.HandleAdd(newObj)
}

// HandleDelete processes a deleted RequestAuthentication
func (h *RequestAuthenticationHandler) HandleDelete(obj interface{}) {
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

	ra := &istiosecurityv1.RequestAuthentication{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.Object, ra); err != nil {
		h.Logger.Error("failed to convert to RequestAuthentication", zap.Error(err))
		return
	}

	h.Logger.Debug("requestauthentication deleted",
		zap.String("namespace", ra.Namespace),
		zap.String("name", ra.Name),
	)

	ctx := context.Background()

	raID := models.GetNodeID("RequestAuthentication", ra.Namespace, ra.Name)
	if err := h.GraphStore.DeleteNode(ctx, string(models.NodeTypeRequestAuthentication), raID); err != nil {
		h.Logger.Error("failed to delete requestauthentication node", zap.Error(err), zap.String("requestauthentication", ra.Name))
	}
}
