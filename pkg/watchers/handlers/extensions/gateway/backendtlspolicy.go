package gateway

import (
	"context"

	"github.com/aslakknutsen/kkbase/pkg/graph"
	"github.com/aslakknutsen/kkbase/pkg/models"
	"github.com/aslakknutsen/kkbase/pkg/watchers"
	"github.com/aslakknutsen/kkbase/pkg/watchers/handlers/core"
	"go.uber.org/zap"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// extractAncestorStatusProps extracts status properties for BackendTLSPolicy ancestors
// BackendTLSPolicy is in v1 and uses PolicyAncestorStatus for per-ancestor status
func extractAncestorStatusProps(policy *gatewayv1.BackendTLSPolicy) map[string]interface{} {
	props := make(map[string]interface{})

	// Extract conditions from all ancestors
	// BackendTLSPolicy has per-ancestor status in Status.Ancestors
	for _, ancestor := range policy.Status.Ancestors {

		// Extract conditions for this ancestor
		for _, condition := range ancestor.Conditions {
			switch string(condition.Type) {
			case "Accepted":
				if condition.Status == "True" {
					props["status_accepted"] = true
				} else if _, exists := props["status_accepted"]; !exists {
					props["status_accepted"] = false
				}
				if condition.Message != "" && props["status_accepted_message"] == nil {
					props["status_accepted_message"] = condition.Message
				}
				if condition.Reason != "" && props["status_accepted_reason"] == nil {
					props["status_accepted_reason"] = string(condition.Reason)
				}
			case "ResolvedRefs":
				if condition.Status == "True" {
					props["status_resolved_refs"] = true
				} else if _, exists := props["status_resolved_refs"]; !exists {
					props["status_resolved_refs"] = false
				}
				if condition.Message != "" && props["status_resolved_refs_message"] == nil {
					props["status_resolved_refs_message"] = condition.Message
				}
				if condition.Reason != "" && props["status_resolved_refs_reason"] == nil {
					props["status_resolved_refs_reason"] = string(condition.Reason)
				}
			}
		}
	}

	return props
}

// BackendTLSPolicyHandler handles BackendTLSPolicy resources
type BackendTLSPolicyHandler struct {
	*watchers.BaseWatcher
	dynamicClient       dynamic.Interface
	relationshipBuilder *RelationshipBuilder
}

// NewBackendTLSPolicyHandler creates a new BackendTLSPolicy handler
func NewBackendTLSPolicyHandler(
	clientset *kubernetes.Clientset,
	dynamicClient dynamic.Interface,
	graphStore graph.GraphStore,
	logger *zap.Logger,
	factory dynamicinformer.DynamicSharedInformerFactory,
) *BackendTLSPolicyHandler {
	gvr := gatewayv1.SchemeGroupVersion.WithResource("backendtlspolicies")
	informer := factory.ForResource(gvr).Informer()

	handler := &BackendTLSPolicyHandler{
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

// HandleAdd processes a newly added BackendTLSPolicy
func (h *BackendTLSPolicyHandler) HandleAdd(obj interface{}) {
	backendTLSPolicy, err := watchers.ConvertToTyped[gatewayv1.BackendTLSPolicy](obj)

	if err != nil {
		h.Logger.Error("failed to convert to BackendTLSPolicy", zap.Error(err))
		return
	}

	h.Logger.Debug("backendtlspolicy added",
		zap.String("namespace", backendTLSPolicy.Namespace),
		zap.String("name", backendTLSPolicy.Name),
	)

	ctx := context.Background()

	// Create BackendTLSPolicy node
	backendTLSPolicyNode := BackendTLSPolicyToGraphNode(backendTLSPolicy)
	if err := h.GraphStore.UpsertNode(ctx, string(backendTLSPolicyNode.Type), backendTLSPolicyNode.ID, backendTLSPolicyNode.Properties); err != nil {
		h.Logger.Error("failed to create backendtlspolicy node", zap.Error(err), zap.String("backendtlspolicy", backendTLSPolicy.Name))
		return
	}

	// Create IN_NAMESPACE edge
	if err := h.relationshipBuilder.base.CreateNamespaceEdge(ctx, NodeTypeBackendTLSPolicy, backendTLSPolicyNode.ID, backendTLSPolicy.Namespace); err != nil {
		h.Logger.Error("failed to create namespace edge", zap.Error(err))
	}

	// Create APPLIES_TO edges to target Services with per-ancestor status
	for _, targetRef := range backendTLSPolicy.Spec.TargetRefs {
		if targetRef.Kind == "Service" && (targetRef.Group == "" || targetRef.Group == "core" || targetRef.Group == "v1") {
			targetNamespace := backendTLSPolicy.Namespace // LocalPolicyTargetReference is always in the same namespace
			serviceName := string(targetRef.Name)

			serviceID := models.GetNodeID(core.NodeTypeService, targetNamespace, serviceName)

			// Extract per-ancestor status (aggregated across all ancestors)
			statusProps := extractAncestorStatusProps(backendTLSPolicy)

			if err := h.GraphStore.UpsertEdge(
				ctx,
				string(NodeTypeBackendTLSPolicy),
				backendTLSPolicyNode.ID,
				string(models.EdgeTypeAppliesTo),
				string(core.NodeTypeService),
				serviceID,
				statusProps,
			); err != nil {
				h.Logger.Error("failed to create APPLIES_TO edge",
					zap.Error(err),
					zap.String("service", serviceName),
				)
			}
		}
	}
}

// HandleUpdate processes an updated BackendTLSPolicy
func (h *BackendTLSPolicyHandler) HandleUpdate(oldObj, newObj interface{}) {
	backendTLSPolicy, err := watchers.ConvertToTyped[gatewayv1.BackendTLSPolicy](newObj)

	if err != nil {
		h.Logger.Error("failed to convert to BackendTLSPolicy", zap.Error(err))
		return
	}

	h.Logger.Debug("backendtlspolicy updated",
		zap.String("namespace", backendTLSPolicy.Namespace),
		zap.String("name", backendTLSPolicy.Name),
	)

	ctx := context.Background()
	backendTLSPolicyID := models.GetNodeID(NodeTypeBackendTLSPolicy, backendTLSPolicy.Namespace, backendTLSPolicy.Name)

	// Delete old edges
	if err := h.GraphStore.DeleteEdgesByNode(ctx, string(NodeTypeBackendTLSPolicy), backendTLSPolicyID); err != nil {
		h.Logger.Error("failed to delete old backendtlspolicy edges", zap.Error(err))
	}

	// Recreate with new data
	h.HandleAdd(newObj)
}

// HandleDelete processes a deleted BackendTLSPolicy
func (h *BackendTLSPolicyHandler) HandleDelete(obj interface{}) {
	backendTLSPolicy, err := watchers.ConvertToTyped[gatewayv1.BackendTLSPolicy](obj)

	if err != nil {
		h.Logger.Error("failed to convert to BackendTLSPolicy", zap.Error(err))
		return
	}

	h.Logger.Debug("backendtlspolicy deleted",
		zap.String("namespace", backendTLSPolicy.Namespace),
		zap.String("name", backendTLSPolicy.Name),
	)

	ctx := context.Background()

	backendTLSPolicyID := models.GetNodeID(NodeTypeBackendTLSPolicy, backendTLSPolicy.Namespace, backendTLSPolicy.Name)
	if err := h.GraphStore.DeleteNode(ctx, string(NodeTypeBackendTLSPolicy), backendTLSPolicyID); err != nil {
		h.Logger.Error("failed to delete backendtlspolicy node", zap.Error(err), zap.String("backendtlspolicy", backendTLSPolicy.Name))
	}
}
