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

// PeerAuthenticationHandler handles Istio PeerAuthentication resources
type PeerAuthenticationHandler struct {
	*watchers.BaseWatcher
	clientset           *kubernetes.Clientset
	dynamicClient       dynamic.Interface
	relationshipBuilder *watchers.RelationshipBuilder
}

// NewPeerAuthenticationHandler creates a new PeerAuthentication handler
func NewPeerAuthenticationHandler(
	clientset *kubernetes.Clientset,
	dynamicClient dynamic.Interface,
	graphStore graph.GraphStore,
	logger *zap.Logger,
	dynamicInformerFactory dynamicinformer.DynamicSharedInformerFactory,
) *PeerAuthenticationHandler {
	gvr := istiosecurityv1.SchemeGroupVersion.WithResource("peerauthentications")
	informer := dynamicInformerFactory.ForResource(gvr).Informer()

	handler := &PeerAuthenticationHandler{
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

// HandleAdd processes a newly added PeerAuthentication
func (h *PeerAuthenticationHandler) HandleAdd(obj interface{}) {
	unstructuredObj, ok := obj.(*unstructured.Unstructured)
	if !ok {
		h.Logger.Error("unexpected object type", zap.String("type", fmt.Sprintf("%T", obj)))
		return
	}

	pa := &istiosecurityv1.PeerAuthentication{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.Object, pa); err != nil {
		h.Logger.Error("failed to convert to PeerAuthentication", zap.Error(err))
		return
	}

	h.Logger.Debug("peerauthentication added",
		zap.String("namespace", pa.Namespace),
		zap.String("name", pa.Name),
	)

	ctx := context.Background()

	// Create PeerAuthentication node
	paNode := models.PeerAuthenticationToGraphNode(pa)
	if err := h.GraphStore.UpsertNode(ctx, string(paNode.Type), paNode.ID, paNode.Properties); err != nil {
		h.Logger.Error("failed to create peerauthentication node", zap.Error(err), zap.String("peerauthentication", pa.Name))
		return
	}

	// Create IN_NAMESPACE edge
	if err := h.relationshipBuilder.CreateNamespaceEdge(ctx, models.NodeTypePeerAuthentication, paNode.ID, pa.Namespace); err != nil {
		h.Logger.Error("failed to create namespace edge", zap.Error(err))
	}

	// Create APPLIES_TO edges to Pods
	selector := make(map[string]string)
	if pa.Spec.Selector != nil && len(pa.Spec.Selector.MatchLabels) > 0 {
		selector = pa.Spec.Selector.MatchLabels
	}

	additionalProps := map[string]interface{}{}
	if pa.Spec.Mtls != nil && pa.Spec.Mtls.Mode.String() != "" {
		additionalProps["mtls_mode"] = pa.Spec.Mtls.Mode.String()
	}

	if err := h.relationshipBuilder.CreateIstioPolicyAppliesToEdge(
		ctx, models.NodeTypePeerAuthentication, pa.Namespace, pa.Name, selector, additionalProps); err != nil {
		h.Logger.Error("failed to create APPLIES_TO edges", zap.Error(err))
	}
}

// HandleUpdate processes an updated PeerAuthentication
func (h *PeerAuthenticationHandler) HandleUpdate(oldObj, newObj interface{}) {
	unstructuredObj, ok := newObj.(*unstructured.Unstructured)
	if !ok {
		h.Logger.Error("unexpected object type", zap.String("type", fmt.Sprintf("%T", newObj)))
		return
	}

	pa := &istiosecurityv1.PeerAuthentication{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.Object, pa); err != nil {
		h.Logger.Error("failed to convert to PeerAuthentication", zap.Error(err))
		return
	}

	h.Logger.Debug("peerauthentication updated",
		zap.String("namespace", pa.Namespace),
		zap.String("name", pa.Name),
	)

	ctx := context.Background()
	paID := models.GetNodeID("PeerAuthentication", pa.Namespace, pa.Name)

	// Delete old edges
	if err := h.GraphStore.DeleteEdgesByNode(ctx, string(models.NodeTypePeerAuthentication), paID); err != nil {
		h.Logger.Error("failed to delete old peerauthentication edges", zap.Error(err))
	}

	// Recreate with new data
	h.HandleAdd(newObj)
}

// HandleDelete processes a deleted PeerAuthentication
func (h *PeerAuthenticationHandler) HandleDelete(obj interface{}) {
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

	pa := &istiosecurityv1.PeerAuthentication{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.Object, pa); err != nil {
		h.Logger.Error("failed to convert to PeerAuthentication", zap.Error(err))
		return
	}

	h.Logger.Debug("peerauthentication deleted",
		zap.String("namespace", pa.Namespace),
		zap.String("name", pa.Name),
	)

	ctx := context.Background()

	paID := models.GetNodeID("PeerAuthentication", pa.Namespace, pa.Name)
	if err := h.GraphStore.DeleteNode(ctx, string(models.NodeTypePeerAuthentication), paID); err != nil {
		h.Logger.Error("failed to delete peerauthentication node", zap.Error(err), zap.String("peerauthentication", pa.Name))
	}
}
