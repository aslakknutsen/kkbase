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

	istiov1 "istio.io/client-go/pkg/apis/networking/v1"
)

// DestinationRuleHandler handles Istio DestinationRule resources
type DestinationRuleHandler struct {
	*watchers.BaseWatcher
	clientset           *kubernetes.Clientset
	dynamicClient       dynamic.Interface
	relationshipBuilder *watchers.RelationshipBuilder
}

// NewDestinationRuleHandler creates a new DestinationRule handler
func NewDestinationRuleHandler(
	clientset *kubernetes.Clientset,
	dynamicClient dynamic.Interface,
	graphStore graph.GraphStore,
	logger *zap.Logger,
	factory dynamicinformer.DynamicSharedInformerFactory,
) *DestinationRuleHandler {
	gvr := istiov1.SchemeGroupVersion.WithResource("destinationrules")
	informer := factory.ForResource(gvr).Informer()

	handler := &DestinationRuleHandler{
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

// HandleAdd processes a newly added DestinationRule
func (h *DestinationRuleHandler) HandleAdd(obj interface{}) {
	destinationRule, err := watchers.ConvertToTyped[istiov1.DestinationRule](obj)

	if err != nil {

		h.Logger.Error("failed to convert to DestinationRule", zap.Error(err))

		return

	}

	h.Logger.Debug("destinationrule added",
		zap.String("namespace", destinationRule.Namespace),
		zap.String("name", destinationRule.Name),
		zap.String("host", destinationRule.Spec.Host),
	)

	ctx := context.Background()

	// Create DestinationRule node
	drNode := models.DestinationRuleToGraphNode(destinationRule)
	if err := h.GraphStore.UpsertNode(ctx, string(drNode.Type), drNode.ID, drNode.Properties); err != nil {
		h.Logger.Error("failed to create destinationrule node", zap.Error(err), zap.String("destinationrule", destinationRule.Name))
		return
	}

	// Create IN_NAMESPACE edge
	if err := h.relationshipBuilder.CreateNamespaceEdge(ctx, models.NodeTypeDestinationRule, drNode.ID, destinationRule.Namespace); err != nil {
		h.Logger.Error("failed to create namespace edge", zap.Error(err))
	}

	// Create DEFINES_POLICY_FOR edge to Service
	if destinationRule.Spec.Host != "" {
		svcNs, svcName := parseServiceHost(destinationRule.Spec.Host, destinationRule.Namespace)
		if svcName != "" {
			if err := h.relationshipBuilder.CreateDestinationRuleDefinesPolicyForEdge(ctx, destinationRule.Namespace, destinationRule.Name, svcNs, svcName, destinationRule.Spec.Host); err != nil {
				h.Logger.Debug("failed to create DEFINES_POLICY_FOR edge",
					zap.Error(err),
					zap.String("host", destinationRule.Spec.Host),
				)
			}
		}
	}

	// Create SELECTS_SUBSET_PODS edges for each subset
	for _, subset := range destinationRule.Spec.Subsets {
		if len(subset.Labels) > 0 {
			// Determine target namespace from host
			targetNs, _ := parseServiceHost(destinationRule.Spec.Host, destinationRule.Namespace)
			if err := h.relationshipBuilder.CreateDestinationRuleSelectsSubsetPodsEdge(
				ctx, destinationRule.Namespace, destinationRule.Name, subset.Name, subset.Labels, targetNs); err != nil {
				h.Logger.Error("failed to create SELECTS_SUBSET_PODS edges",
					zap.Error(err),
					zap.String("subset", subset.Name),
				)
			}
		}
	}
}

// HandleUpdate processes an updated DestinationRule
func (h *DestinationRuleHandler) HandleUpdate(oldObj, newObj interface{}) {
	destinationRule, err := watchers.ConvertToTyped[istiov1.DestinationRule](newObj)

	if err != nil {

		h.Logger.Error("failed to convert to DestinationRule", zap.Error(err))

		return

	}

	h.Logger.Debug("destinationrule updated",
		zap.String("namespace", destinationRule.Namespace),
		zap.String("name", destinationRule.Name),
	)

	ctx := context.Background()
	drID := models.GetNodeID("DestinationRule", destinationRule.Namespace, destinationRule.Name)

	// Delete old edges
	if err := h.GraphStore.DeleteEdgesByNode(ctx, string(models.NodeTypeDestinationRule), drID); err != nil {
		h.Logger.Error("failed to delete old destinationrule edges", zap.Error(err))
	}

	// Recreate with new data
	h.HandleAdd(newObj)
}

// HandleDelete processes a deleted DestinationRule
func (h *DestinationRuleHandler) HandleDelete(obj interface{}) {
	destinationRule, err := watchers.ConvertToTyped[istiov1.DestinationRule](obj)

	if err != nil {

		h.Logger.Error("failed to convert to DestinationRule", zap.Error(err))

		return

	}

	h.Logger.Debug("destinationrule deleted",
		zap.String("namespace", destinationRule.Namespace),
		zap.String("name", destinationRule.Name),
	)

	ctx := context.Background()

	drID := models.GetNodeID("DestinationRule", destinationRule.Namespace, destinationRule.Name)
	if err := h.GraphStore.DeleteNode(ctx, string(models.NodeTypeDestinationRule), drID); err != nil {
		h.Logger.Error("failed to delete destinationrule node", zap.Error(err), zap.String("destinationrule", destinationRule.Name))
	}
}
