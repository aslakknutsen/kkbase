package istio

import (
	"context"

	"github.com/aslakknutsen/kkbase/pkg/graph"
	"github.com/aslakknutsen/kkbase/pkg/models"
	"github.com/aslakknutsen/kkbase/pkg/watchers"
	"go.uber.org/zap"
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
	relationshipBuilder *RelationshipBuilder
}

// NewRequestAuthenticationHandler creates a new RequestAuthentication handler
func NewRequestAuthenticationHandler(
	clientset *kubernetes.Clientset,
	dynamicClient dynamic.Interface,
	graphStore graph.GraphStore,
	logger *zap.Logger,
	factory dynamicinformer.DynamicSharedInformerFactory,
) *RequestAuthenticationHandler {
	gvr := istiosecurityv1.SchemeGroupVersion.WithResource("requestauthentications")
	informer := factory.ForResource(gvr).Informer()

	handler := &RequestAuthenticationHandler{
		BaseWatcher:   watchers.NewBaseWatcher(graphStore, logger, informer),
		clientset:     clientset,
		dynamicClient: dynamicClient, relationshipBuilder: NewRelationshipBuilder(clientset, graphStore, logger),
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
	requestAuthentication, err := watchers.ConvertToTyped[istiosecurityv1.RequestAuthentication](obj)
	if err != nil {
		h.Logger.Error("failed to convert to RequestAuthentication", zap.Error(err))
		return
	}

	h.Logger.Debug("requestauthentication added",
		zap.String("namespace", requestAuthentication.Namespace),
		zap.String("name", requestAuthentication.Name),
	)

	ctx := context.Background()

	// Create RequestAuthentication node
	raNode := RequestAuthenticationToGraphNode(requestAuthentication)
	if err := h.GraphStore.UpsertNode(ctx, string(raNode.Type), raNode.ID, raNode.Properties); err != nil {
		h.Logger.Error("failed to create requestauthentication node", zap.Error(err), zap.String("requestauthentication", requestAuthentication.Name))
		return
	}

	// Create IN_NAMESPACE edge
	if err := h.relationshipBuilder.base.CreateNamespaceEdge(ctx, NodeTypeRequestAuthentication, raNode.ID, requestAuthentication.Namespace); err != nil {
		h.Logger.Error("failed to create namespace edge", zap.Error(err))
	}

	// Create APPLIES_TO edges to Pods
	selector := make(map[string]string)
	if requestAuthentication.Spec.Selector != nil && len(requestAuthentication.Spec.Selector.MatchLabels) > 0 {
		selector = requestAuthentication.Spec.Selector.MatchLabels
	}

	if err := h.relationshipBuilder.CreateIstioPolicyAppliesToEdge(
		ctx, NodeTypeRequestAuthentication, requestAuthentication.Namespace, requestAuthentication.Name, selector, nil); err != nil {
		h.Logger.Error("failed to create APPLIES_TO edges", zap.Error(err))
	}
}

// HandleUpdate processes an updated RequestAuthentication
func (h *RequestAuthenticationHandler) HandleUpdate(oldObj, newObj interface{}) {
	requestAuthentication, err := watchers.ConvertToTyped[istiosecurityv1.RequestAuthentication](newObj)
	if err != nil {
		h.Logger.Error("failed to convert to RequestAuthentication", zap.Error(err))
		return
	}

	h.Logger.Debug("requestauthentication updated",
		zap.String("namespace", requestAuthentication.Namespace),
		zap.String("name", requestAuthentication.Name),
	)

	ctx := context.Background()
	raID := models.GetNodeID(NodeTypeRequestAuthentication, requestAuthentication.Namespace, requestAuthentication.Name)

	// Delete old edges
	if err := h.GraphStore.DeleteEdgesByNode(ctx, string(NodeTypeRequestAuthentication), raID); err != nil {
		h.Logger.Error("failed to delete old requestauthentication edges", zap.Error(err))
	}

	// Recreate with new data
	h.HandleAdd(newObj)
}

// HandleDelete processes a deleted RequestAuthentication
func (h *RequestAuthenticationHandler) HandleDelete(obj interface{}) {
	requestAuthentication, err := watchers.ConvertToTyped[istiosecurityv1.RequestAuthentication](obj)
	if err != nil {
		h.Logger.Error("failed to convert to RequestAuthentication", zap.Error(err))
		return
	}

	h.Logger.Debug("requestauthentication deleted",
		zap.String("namespace", requestAuthentication.Namespace),
		zap.String("name", requestAuthentication.Name),
	)

	ctx := context.Background()

	raID := models.GetNodeID(NodeTypeRequestAuthentication, requestAuthentication.Namespace, requestAuthentication.Name)
	if err := h.GraphStore.DeleteNode(ctx, string(NodeTypeRequestAuthentication), raID); err != nil {
		h.Logger.Error("failed to delete requestauthentication node", zap.Error(err), zap.String("requestauthentication", requestAuthentication.Name))
	}
}
