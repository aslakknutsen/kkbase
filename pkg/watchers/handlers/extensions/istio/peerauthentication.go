package istio

import (
	"context"

	"github.com/kagenti/kkbase/pkg/graph"
	"github.com/kagenti/kkbase/pkg/models"
	"github.com/kagenti/kkbase/pkg/watchers"
	"go.uber.org/zap"
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
	relationshipBuilder *RelationshipBuilder
}

// NewPeerAuthenticationHandler creates a new PeerAuthentication handler
func NewPeerAuthenticationHandler(
	clientset *kubernetes.Clientset,
	dynamicClient dynamic.Interface,
	graphStore graph.GraphStore,
	logger *zap.Logger,
	factory dynamicinformer.DynamicSharedInformerFactory,
) *PeerAuthenticationHandler {
	gvr := istiosecurityv1.SchemeGroupVersion.WithResource("peerauthentications")
	informer := factory.ForResource(gvr).Informer()

	handler := &PeerAuthenticationHandler{
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

// HandleAdd processes a newly added PeerAuthentication
func (h *PeerAuthenticationHandler) HandleAdd(obj interface{}) {
	peerAuthentication, err := watchers.ConvertToTyped[istiosecurityv1.PeerAuthentication](obj)
	if err != nil {
		h.Logger.Error("failed to convert to PeerAuthentication", zap.Error(err))
		return
	}

	h.Logger.Debug("peerauthentication added",
		zap.String("namespace", peerAuthentication.Namespace),
		zap.String("name", peerAuthentication.Name),
	)

	ctx := context.Background()

	// Create PeerAuthentication node
	paNode := PeerAuthenticationToGraphNode(peerAuthentication)
	if err := h.GraphStore.UpsertNode(ctx, string(paNode.Type), paNode.ID, paNode.Properties); err != nil {
		h.Logger.Error("failed to create peerauthentication node", zap.Error(err), zap.String("peerauthentication", peerAuthentication.Name))
		return
	}

	// Create IN_NAMESPACE edge
	if err := h.relationshipBuilder.base.CreateNamespaceEdge(ctx, NodeTypePeerAuthentication, paNode.ID, peerAuthentication.Namespace); err != nil {
		h.Logger.Error("failed to create namespace edge", zap.Error(err))
	}

	// Create APPLIES_TO edges to Pods
	selector := make(map[string]string)
	if peerAuthentication.Spec.Selector != nil && len(peerAuthentication.Spec.Selector.MatchLabels) > 0 {
		selector = peerAuthentication.Spec.Selector.MatchLabels
	}

	additionalProps := map[string]interface{}{}
	if peerAuthentication.Spec.Mtls != nil && peerAuthentication.Spec.Mtls.Mode.String() != "" {
		additionalProps["mtls_mode"] = peerAuthentication.Spec.Mtls.Mode.String()
	}

	if err := h.relationshipBuilder.CreateIstioPolicyAppliesToEdge(
		ctx, NodeTypePeerAuthentication, peerAuthentication.Namespace, peerAuthentication.Name, selector, additionalProps); err != nil {
		h.Logger.Error("failed to create APPLIES_TO edges", zap.Error(err))
	}
}

// HandleUpdate processes an updated PeerAuthentication
func (h *PeerAuthenticationHandler) HandleUpdate(oldObj, newObj interface{}) {
	peerAuthentication, err := watchers.ConvertToTyped[istiosecurityv1.PeerAuthentication](newObj)
	if err != nil {
		h.Logger.Error("failed to convert to PeerAuthentication", zap.Error(err))
		return
	}

	h.Logger.Debug("peerauthentication updated",
		zap.String("namespace", peerAuthentication.Namespace),
		zap.String("name", peerAuthentication.Name),
	)

	ctx := context.Background()
	paID := models.GetNodeID(NodeTypePeerAuthentication, peerAuthentication.Namespace, peerAuthentication.Name)

	// Delete old edges
	if err := h.GraphStore.DeleteEdgesByNode(ctx, string(NodeTypePeerAuthentication), paID); err != nil {
		h.Logger.Error("failed to delete old peerauthentication edges", zap.Error(err))
	}

	// Recreate with new data
	h.HandleAdd(newObj)
}

// HandleDelete processes a deleted PeerAuthentication
func (h *PeerAuthenticationHandler) HandleDelete(obj interface{}) {
	peerAuthentication, err := watchers.ConvertToTyped[istiosecurityv1.PeerAuthentication](obj)
	if err != nil {
		h.Logger.Error("failed to convert to PeerAuthentication", zap.Error(err))
		return
	}

	h.Logger.Debug("peerauthentication deleted",
		zap.String("namespace", peerAuthentication.Namespace),
		zap.String("name", peerAuthentication.Name),
	)

	ctx := context.Background()

	paID := models.GetNodeID(NodeTypePeerAuthentication, peerAuthentication.Namespace, peerAuthentication.Name)
	if err := h.GraphStore.DeleteNode(ctx, string(NodeTypePeerAuthentication), paID); err != nil {
		h.Logger.Error("failed to delete peerauthentication node", zap.Error(err), zap.String("peerauthentication", peerAuthentication.Name))
	}
}
