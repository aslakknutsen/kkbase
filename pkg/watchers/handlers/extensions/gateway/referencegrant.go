package gateway

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
	"k8s.io/client-go/tools/cache"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

// ReferenceGrantHandler handles ReferenceGrant resources
type ReferenceGrantHandler struct {
	*watchers.BaseWatcher
	dynamicClient       dynamic.Interface
	relationshipBuilder *watchers.RelationshipBuilder
}

// NewReferenceGrantHandler creates a new ReferenceGrant handler
func NewReferenceGrantHandler(
	dynamicClient dynamic.Interface,
	graphStore graph.GraphStore,
	logger *zap.Logger,
	dynamicInformerFactory dynamicinformer.DynamicSharedInformerFactory,
) *ReferenceGrantHandler {
	gvr := gatewayv1beta1.SchemeGroupVersion.WithResource("referencegrants")
	informer := dynamicInformerFactory.ForResource(gvr).Informer()

	handler := &ReferenceGrantHandler{
		BaseWatcher:         watchers.NewBaseWatcher(graphStore, logger, informer),
		dynamicClient:       dynamicClient,
		relationshipBuilder: watchers.NewRelationshipBuilder(nil, graphStore, logger),
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

// HandleAdd processes a newly added ReferenceGrant
func (h *ReferenceGrantHandler) HandleAdd(obj interface{}) {
	unstructuredObj, ok := obj.(*unstructured.Unstructured)
	if !ok {
		h.Logger.Error("unexpected object type", zap.String("type", fmt.Sprintf("%T", obj)))
		return
	}

	referenceGrant := &gatewayv1beta1.ReferenceGrant{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.Object, referenceGrant); err != nil {
		h.Logger.Error("failed to convert to ReferenceGrant", zap.Error(err))
		return
	}

	h.Logger.Debug("referencegrant added",
		zap.String("namespace", referenceGrant.Namespace),
		zap.String("name", referenceGrant.Name),
	)

	ctx := context.Background()

	// Create ReferenceGrant node
	referenceGrantNode := models.ReferenceGrantToGraphNode(referenceGrant)
	if err := h.GraphStore.UpsertNode(ctx, string(referenceGrantNode.Type), referenceGrantNode.ID, referenceGrantNode.Properties); err != nil {
		h.Logger.Error("failed to create referencegrant node", zap.Error(err), zap.String("referencegrant", referenceGrant.Name))
		return
	}

	// Create IN_NAMESPACE edge
	if err := h.relationshipBuilder.CreateNamespaceEdge(ctx, models.NodeTypeReferenceGrant, referenceGrantNode.ID, referenceGrant.Namespace); err != nil {
		h.Logger.Error("failed to create namespace edge", zap.Error(err))
	}

	// Create ALLOWS_ROUTE_TO edges to target Services
	// ReferenceGrant allows resources in "from" namespaces to reference resources in this namespace ("to")
	for _, to := range referenceGrant.Spec.To {
		// Only handle Service targets for now
		if to.Kind == "Service" && (to.Group == "" || to.Group == "core" || to.Group == "v1") {
			// If a specific name is provided, create edge to that service
			if to.Name != nil {
				serviceName := string(*to.Name)
				if err := h.relationshipBuilder.CreateReferenceGrantAllowsEdge(
					ctx,
					referenceGrant.Namespace,
					referenceGrant.Name,
					referenceGrant.Namespace,
					serviceName,
				); err != nil {
					h.Logger.Error("failed to create ALLOWS_ROUTE_TO edge",
						zap.Error(err),
						zap.String("service", serviceName),
					)
				}
			}
			// Note: If no name is specified, the grant allows access to all Services in the namespace
			// We could query and create edges to all services, but that would be expensive
			// Instead, we just track the grant and routes can reference it
		}
	}
}

// HandleUpdate processes an updated ReferenceGrant
func (h *ReferenceGrantHandler) HandleUpdate(oldObj, newObj interface{}) {
	unstructuredObj, ok := newObj.(*unstructured.Unstructured)
	if !ok {
		h.Logger.Error("unexpected object type", zap.String("type", fmt.Sprintf("%T", newObj)))
		return
	}

	referenceGrant := &gatewayv1beta1.ReferenceGrant{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.Object, referenceGrant); err != nil {
		h.Logger.Error("failed to convert to ReferenceGrant", zap.Error(err))
		return
	}

	h.Logger.Debug("referencegrant updated",
		zap.String("namespace", referenceGrant.Namespace),
		zap.String("name", referenceGrant.Name),
	)

	ctx := context.Background()
	referenceGrantID := models.GetNodeID("ReferenceGrant", referenceGrant.Namespace, referenceGrant.Name)

	// Delete old edges
	if err := h.GraphStore.DeleteEdgesByNode(ctx, string(models.NodeTypeReferenceGrant), referenceGrantID); err != nil {
		h.Logger.Error("failed to delete old referencegrant edges", zap.Error(err))
	}

	// Recreate with new data
	h.HandleAdd(newObj)
}

// HandleDelete processes a deleted ReferenceGrant
func (h *ReferenceGrantHandler) HandleDelete(obj interface{}) {
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

	referenceGrant := &gatewayv1beta1.ReferenceGrant{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.Object, referenceGrant); err != nil {
		h.Logger.Error("failed to convert to ReferenceGrant", zap.Error(err))
		return
	}

	h.Logger.Debug("referencegrant deleted",
		zap.String("namespace", referenceGrant.Namespace),
		zap.String("name", referenceGrant.Name),
	)

	ctx := context.Background()

	referenceGrantID := models.GetNodeID("ReferenceGrant", referenceGrant.Namespace, referenceGrant.Name)
	if err := h.GraphStore.DeleteNode(ctx, string(models.NodeTypeReferenceGrant), referenceGrantID); err != nil {
		h.Logger.Error("failed to delete referencegrant node", zap.Error(err), zap.String("referencegrant", referenceGrant.Name))
	}
}
