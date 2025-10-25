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

// AuthorizationPolicyHandler handles Istio AuthorizationPolicy resources
type AuthorizationPolicyHandler struct {
	*watchers.BaseWatcher
	clientset           *kubernetes.Clientset
	dynamicClient       dynamic.Interface
	relationshipBuilder *watchers.RelationshipBuilder
}

// NewAuthorizationPolicyHandler creates a new AuthorizationPolicy handler
func NewAuthorizationPolicyHandler(
	clientset *kubernetes.Clientset,
	dynamicClient dynamic.Interface,
	graphStore graph.GraphStore,
	logger *zap.Logger,
	factory dynamicinformer.DynamicSharedInformerFactory,
) *AuthorizationPolicyHandler {
	gvr := istiosecurityv1.SchemeGroupVersion.WithResource("authorizationpolicies")
	informer := factory.ForResource(gvr).Informer()

	handler := &AuthorizationPolicyHandler{
		BaseWatcher:   watchers.NewBaseWatcher(graphStore, logger, informer),
		clientset:     clientset,
		dynamicClient: dynamicClient, relationshipBuilder: watchers.NewRelationshipBuilder(clientset, graphStore, logger),
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

// HandleAdd processes a newly added AuthorizationPolicy
func (h *AuthorizationPolicyHandler) HandleAdd(obj interface{}) {
	policy, err := watchers.ConvertToTyped[istiosecurityv1.AuthorizationPolicy](obj)
	if err != nil {
		h.Logger.Error("failed to convert to AuthorizationPolicy", zap.Error(err))
		return
	}

	h.Logger.Debug("authorizationpolicy added",
		zap.String("namespace", policy.Namespace),
		zap.String("name", policy.Name),
	)

	ctx := context.Background()

	// Create AuthorizationPolicy node
	policyNode := models.AuthorizationPolicyToGraphNode(policy)
	if err := h.GraphStore.UpsertNode(ctx, string(policyNode.Type), policyNode.ID, policyNode.Properties); err != nil {
		h.Logger.Error("failed to create authorizationpolicy node", zap.Error(err), zap.String("policy", policy.Name))
		return
	}

	// Create IN_NAMESPACE edge
	if err := h.relationshipBuilder.CreateNamespaceEdge(ctx, models.NodeTypeAuthorizationPolicy, policyNode.ID, policy.Namespace); err != nil {
		h.Logger.Error("failed to create namespace edge", zap.Error(err))
	}

	// Create APPLIES_TO edges to Pods
	selector := make(map[string]string)
	if policy.Spec.Selector != nil && len(policy.Spec.Selector.MatchLabels) > 0 {
		selector = policy.Spec.Selector.MatchLabels
	}

	additionalProps := map[string]interface{}{}
	if policy.Spec.Action.String() != "" {
		additionalProps["action"] = policy.Spec.Action.String()
	}

	if err := h.relationshipBuilder.CreateIstioPolicyAppliesToEdge(
		ctx, models.NodeTypeAuthorizationPolicy, policy.Namespace, policy.Name, selector, additionalProps); err != nil {
		h.Logger.Error("failed to create APPLIES_TO edges", zap.Error(err))
	}
}

// HandleUpdate processes an updated AuthorizationPolicy
func (h *AuthorizationPolicyHandler) HandleUpdate(oldObj, newObj interface{}) {
	policy, err := watchers.ConvertToTyped[istiosecurityv1.AuthorizationPolicy](newObj)
	if err != nil {
		h.Logger.Error("failed to convert to AuthorizationPolicy", zap.Error(err))
		return
	}

	h.Logger.Debug("authorizationpolicy updated",
		zap.String("namespace", policy.Namespace),
		zap.String("name", policy.Name),
	)

	ctx := context.Background()
	policyID := models.GetNodeID("AuthorizationPolicy", policy.Namespace, policy.Name)

	// Delete old edges
	if err := h.GraphStore.DeleteEdgesByNode(ctx, string(models.NodeTypeAuthorizationPolicy), policyID); err != nil {
		h.Logger.Error("failed to delete old authorizationpolicy edges", zap.Error(err))
	}

	// Recreate with new data
	h.HandleAdd(newObj)
}

// HandleDelete processes a deleted AuthorizationPolicy
func (h *AuthorizationPolicyHandler) HandleDelete(obj interface{}) {
	policy, err := watchers.ConvertToTyped[istiosecurityv1.AuthorizationPolicy](obj)
	if err != nil {
		h.Logger.Error("failed to convert to AuthorizationPolicy", zap.Error(err))
		return
	}

	h.Logger.Debug("authorizationpolicy deleted",
		zap.String("namespace", policy.Namespace),
		zap.String("name", policy.Name),
	)

	ctx := context.Background()

	policyID := models.GetNodeID("AuthorizationPolicy", policy.Namespace, policy.Name)
	if err := h.GraphStore.DeleteNode(ctx, string(models.NodeTypeAuthorizationPolicy), policyID); err != nil {
		h.Logger.Error("failed to delete authorizationpolicy node", zap.Error(err), zap.String("policy", policy.Name))
	}
}
