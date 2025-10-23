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
	dynamicInformerFactory dynamicinformer.DynamicSharedInformerFactory,
) *DestinationRuleHandler {
	gvr := istiov1.SchemeGroupVersion.WithResource("destinationrules")
	informer := dynamicInformerFactory.ForResource(gvr).Informer()

	handler := &DestinationRuleHandler{
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

// HandleAdd processes a newly added DestinationRule
func (h *DestinationRuleHandler) HandleAdd(obj interface{}) {
	unstructuredObj, ok := obj.(*unstructured.Unstructured)
	if !ok {
		h.Logger.Error("unexpected object type", zap.String("type", fmt.Sprintf("%T", obj)))
		return
	}

	dr := &istiov1.DestinationRule{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.Object, dr); err != nil {
		h.Logger.Error("failed to convert to DestinationRule", zap.Error(err))
		return
	}

	h.Logger.Debug("destinationrule added",
		zap.String("namespace", dr.Namespace),
		zap.String("name", dr.Name),
		zap.String("host", dr.Spec.Host),
	)

	ctx := context.Background()

	// Create DestinationRule node
	drNode := models.DestinationRuleToGraphNode(dr)
	if err := h.GraphStore.UpsertNode(ctx, string(drNode.Type), drNode.ID, drNode.Properties); err != nil {
		h.Logger.Error("failed to create destinationrule node", zap.Error(err), zap.String("destinationrule", dr.Name))
		return
	}

	// Create IN_NAMESPACE edge
	if err := h.relationshipBuilder.CreateNamespaceEdge(ctx, models.NodeTypeDestinationRule, drNode.ID, dr.Namespace); err != nil {
		h.Logger.Error("failed to create namespace edge", zap.Error(err))
	}

	// Create DEFINES_POLICY_FOR edge to Service
	if dr.Spec.Host != "" {
		svcNs, svcName := parseServiceHost(dr.Spec.Host, dr.Namespace)
		if svcName != "" {
			if err := h.relationshipBuilder.CreateDestinationRuleDefinesPolicyForEdge(ctx, dr.Namespace, dr.Name, svcNs, svcName, dr.Spec.Host); err != nil {
				h.Logger.Debug("failed to create DEFINES_POLICY_FOR edge",
					zap.Error(err),
					zap.String("host", dr.Spec.Host),
				)
			}
		}
	}

	// Create SELECTS_SUBSET_PODS edges for each subset
	for _, subset := range dr.Spec.Subsets {
		if len(subset.Labels) > 0 {
			// Determine target namespace from host
			targetNs, _ := parseServiceHost(dr.Spec.Host, dr.Namespace)
			if err := h.relationshipBuilder.CreateDestinationRuleSelectsSubsetPodsEdge(
				ctx, dr.Namespace, dr.Name, subset.Name, subset.Labels, targetNs); err != nil {
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
	unstructuredObj, ok := newObj.(*unstructured.Unstructured)
	if !ok {
		h.Logger.Error("unexpected object type", zap.String("type", fmt.Sprintf("%T", newObj)))
		return
	}

	dr := &istiov1.DestinationRule{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.Object, dr); err != nil {
		h.Logger.Error("failed to convert to DestinationRule", zap.Error(err))
		return
	}

	h.Logger.Debug("destinationrule updated",
		zap.String("namespace", dr.Namespace),
		zap.String("name", dr.Name),
	)

	ctx := context.Background()
	drID := models.GetNodeID("DestinationRule", dr.Namespace, dr.Name)

	// Delete old edges
	if err := h.GraphStore.DeleteEdgesByNode(ctx, string(models.NodeTypeDestinationRule), drID); err != nil {
		h.Logger.Error("failed to delete old destinationrule edges", zap.Error(err))
	}

	// Recreate with new data
	h.HandleAdd(newObj)
}

// HandleDelete processes a deleted DestinationRule
func (h *DestinationRuleHandler) HandleDelete(obj interface{}) {
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

	dr := &istiov1.DestinationRule{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.Object, dr); err != nil {
		h.Logger.Error("failed to convert to DestinationRule", zap.Error(err))
		return
	}

	h.Logger.Debug("destinationrule deleted",
		zap.String("namespace", dr.Namespace),
		zap.String("name", dr.Name),
	)

	ctx := context.Background()

	drID := models.GetNodeID("DestinationRule", dr.Namespace, dr.Name)
	if err := h.GraphStore.DeleteNode(ctx, string(models.NodeTypeDestinationRule), drID); err != nil {
		h.Logger.Error("failed to delete destinationrule node", zap.Error(err), zap.String("destinationrule", dr.Name))
	}
}
