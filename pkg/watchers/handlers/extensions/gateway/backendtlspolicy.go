package gateway

import (
	"context"

	"github.com/kagenti/kkbase/pkg/graph"
	"github.com/kagenti/kkbase/pkg/models"
	"github.com/kagenti/kkbase/pkg/watchers"
	"github.com/kagenti/kkbase/pkg/watchers/handlers/core"
	"go.uber.org/zap"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

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

	// Create APPLIES_TO edges to target Services
	for _, targetRef := range backendTLSPolicy.Spec.TargetRefs {
		if targetRef.Kind == "Service" && (targetRef.Group == "" || targetRef.Group == "core" || targetRef.Group == "v1") {
			targetNamespace := backendTLSPolicy.Namespace // LocalPolicyTargetReference is always in the same namespace
			serviceName := string(targetRef.Name)

			serviceID := models.GetNodeID(core.NodeTypeService, targetNamespace, serviceName)

			if err := h.GraphStore.UpsertEdge(
				ctx,
				string(NodeTypeBackendTLSPolicy),
				backendTLSPolicyNode.ID,
				string(models.EdgeTypeAppliesTo),
				string(core.NodeTypeService),
				serviceID,
				nil,
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
