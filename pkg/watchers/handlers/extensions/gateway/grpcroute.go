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
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// GRPCRouteHandler handles GRPCRoute resources
type GRPCRouteHandler struct {
	*watchers.BaseWatcher
	dynamicClient       dynamic.Interface
	relationshipBuilder *watchers.RelationshipBuilder
}

// NewGRPCRouteHandler creates a new GRPCRoute handler
func NewGRPCRouteHandler(
	dynamicClient dynamic.Interface,
	graphStore graph.GraphStore,
	logger *zap.Logger,
	dynamicInformerFactory dynamicinformer.DynamicSharedInformerFactory,
) *GRPCRouteHandler {
	gvr := gatewayv1.SchemeGroupVersion.WithResource("grpcroutes")
	informer := dynamicInformerFactory.ForResource(gvr).Informer()

	handler := &GRPCRouteHandler{
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

// HandleAdd processes a newly added GRPCRoute
func (h *GRPCRouteHandler) HandleAdd(obj interface{}) {
	unstructuredObj, ok := obj.(*unstructured.Unstructured)
	if !ok {
		h.Logger.Error("unexpected object type", zap.String("type", fmt.Sprintf("%T", obj)))
		return
	}

	grpcRoute := &gatewayv1.GRPCRoute{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.Object, grpcRoute); err != nil {
		h.Logger.Error("failed to convert to GRPCRoute", zap.Error(err))
		return
	}

	h.Logger.Debug("grpcroute added",
		zap.String("namespace", grpcRoute.Namespace),
		zap.String("name", grpcRoute.Name),
	)

	ctx := context.Background()

	// Create GRPCRoute node
	grpcRouteNode := models.GRPCRouteToGraphNode(grpcRoute)
	if err := h.GraphStore.UpsertNode(ctx, string(grpcRouteNode.Type), grpcRouteNode.ID, grpcRouteNode.Properties); err != nil {
		h.Logger.Error("failed to create grpcroute node", zap.Error(err), zap.String("grpcroute", grpcRoute.Name))
		return
	}

	// Create IN_NAMESPACE edge
	if err := h.relationshipBuilder.CreateNamespaceEdge(ctx, models.NodeTypeGRPCRoute, grpcRouteNode.ID, grpcRoute.Namespace); err != nil {
		h.Logger.Error("failed to create namespace edge", zap.Error(err))
	}

	// Create ATTACHES_TO edges to parent Gateways
	for _, parentRef := range grpcRoute.Spec.ParentRefs {
		gatewayNamespace := grpcRoute.Namespace
		if parentRef.Namespace != nil {
			gatewayNamespace = string(*parentRef.Namespace)
		}
		gatewayName := string(parentRef.Name)

		var sectionName *string
		if parentRef.SectionName != nil {
			sn := string(*parentRef.SectionName)
			sectionName = &sn
		}

		if err := h.relationshipBuilder.CreateRouteAttachesToEdge(
			ctx,
			models.NodeTypeGRPCRoute,
			grpcRoute.Namespace,
			grpcRoute.Name,
			gatewayNamespace,
			gatewayName,
			sectionName,
		); err != nil {
			h.Logger.Error("failed to create ATTACHES_TO edge",
				zap.Error(err),
				zap.String("gateway", gatewayName),
			)
		}
	}

	// Create FORWARDS_TO edges to backend Services
	for _, rule := range grpcRoute.Spec.Rules {
		if rule.BackendRefs != nil {
			for _, backendRef := range rule.BackendRefs {
				if backendRef.Kind != nil && *backendRef.Kind != "Service" {
					continue
				}

				serviceNamespace := grpcRoute.Namespace
				if backendRef.Namespace != nil {
					serviceNamespace = string(*backendRef.Namespace)
				}
				serviceName := string(backendRef.Name)

				if err := h.relationshipBuilder.CreateRouteForwardsToEdge(
					ctx,
					models.NodeTypeGRPCRoute,
					grpcRoute.Namespace,
					grpcRoute.Name,
					serviceNamespace,
					serviceName,
					backendRef.Weight,
				); err != nil {
					h.Logger.Error("failed to create FORWARDS_TO edge",
						zap.Error(err),
						zap.String("service", serviceName),
					)
				}
			}
		}
	}
}

// HandleUpdate processes an updated GRPCRoute
func (h *GRPCRouteHandler) HandleUpdate(oldObj, newObj interface{}) {
	unstructuredObj, ok := newObj.(*unstructured.Unstructured)
	if !ok {
		h.Logger.Error("unexpected object type", zap.String("type", fmt.Sprintf("%T", newObj)))
		return
	}

	grpcRoute := &gatewayv1.GRPCRoute{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.Object, grpcRoute); err != nil {
		h.Logger.Error("failed to convert to GRPCRoute", zap.Error(err))
		return
	}

	h.Logger.Debug("grpcroute updated",
		zap.String("namespace", grpcRoute.Namespace),
		zap.String("name", grpcRoute.Name),
	)

	ctx := context.Background()
	grpcRouteID := models.GetNodeID("GRPCRoute", grpcRoute.Namespace, grpcRoute.Name)

	// Delete old edges
	if err := h.GraphStore.DeleteEdgesByNode(ctx, string(models.NodeTypeGRPCRoute), grpcRouteID); err != nil {
		h.Logger.Error("failed to delete old grpcroute edges", zap.Error(err))
	}

	// Recreate with new data
	h.HandleAdd(newObj)
}

// HandleDelete processes a deleted GRPCRoute
func (h *GRPCRouteHandler) HandleDelete(obj interface{}) {
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

	grpcRoute := &gatewayv1.GRPCRoute{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.Object, grpcRoute); err != nil {
		h.Logger.Error("failed to convert to GRPCRoute", zap.Error(err))
		return
	}

	h.Logger.Debug("grpcroute deleted",
		zap.String("namespace", grpcRoute.Namespace),
		zap.String("name", grpcRoute.Name),
	)

	ctx := context.Background()

	grpcRouteID := models.GetNodeID("GRPCRoute", grpcRoute.Namespace, grpcRoute.Name)
	if err := h.GraphStore.DeleteNode(ctx, string(models.NodeTypeGRPCRoute), grpcRouteID); err != nil {
		h.Logger.Error("failed to delete grpcroute node", zap.Error(err), zap.String("grpcroute", grpcRoute.Name))
	}
}
