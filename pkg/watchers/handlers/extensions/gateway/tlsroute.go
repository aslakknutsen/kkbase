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
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

// TLSRouteHandler handles TLSRoute resources
type TLSRouteHandler struct {
	*watchers.BaseWatcher
	dynamicClient       dynamic.Interface
	relationshipBuilder *watchers.RelationshipBuilder
}

// NewTLSRouteHandler creates a new TLSRoute handler
func NewTLSRouteHandler(
	dynamicClient dynamic.Interface,
	graphStore graph.GraphStore,
	logger *zap.Logger,
	dynamicInformerFactory dynamicinformer.DynamicSharedInformerFactory,
) *TLSRouteHandler {
	gvr := gatewayv1alpha2.SchemeGroupVersion.WithResource("tlsroutes")
	informer := dynamicInformerFactory.ForResource(gvr).Informer()

	handler := &TLSRouteHandler{
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

// HandleAdd processes a newly added TLSRoute
func (h *TLSRouteHandler) HandleAdd(obj interface{}) {
	unstructuredObj, ok := obj.(*unstructured.Unstructured)
	if !ok {
		h.Logger.Error("unexpected object type", zap.String("type", fmt.Sprintf("%T", obj)))
		return
	}

	tlsRoute := &gatewayv1alpha2.TLSRoute{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.Object, tlsRoute); err != nil {
		h.Logger.Error("failed to convert to TLSRoute", zap.Error(err))
		return
	}

	h.Logger.Debug("tlsroute added",
		zap.String("namespace", tlsRoute.Namespace),
		zap.String("name", tlsRoute.Name),
	)

	ctx := context.Background()

	// Create TLSRoute node
	tlsRouteNode := models.TLSRouteToGraphNode(tlsRoute)
	if err := h.GraphStore.UpsertNode(ctx, string(tlsRouteNode.Type), tlsRouteNode.ID, tlsRouteNode.Properties); err != nil {
		h.Logger.Error("failed to create tlsroute node", zap.Error(err), zap.String("tlsroute", tlsRoute.Name))
		return
	}

	// Create IN_NAMESPACE edge
	if err := h.relationshipBuilder.CreateNamespaceEdge(ctx, models.NodeTypeTLSRoute, tlsRouteNode.ID, tlsRoute.Namespace); err != nil {
		h.Logger.Error("failed to create namespace edge", zap.Error(err))
	}

	// Create ATTACHES_TO edges to parent Gateways
	for _, parentRef := range tlsRoute.Spec.ParentRefs {
		gatewayNamespace := tlsRoute.Namespace
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
			models.NodeTypeTLSRoute,
			tlsRoute.Namespace,
			tlsRoute.Name,
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
	for _, rule := range tlsRoute.Spec.Rules {
		for _, backendRef := range rule.BackendRefs {
			if backendRef.Kind != nil && *backendRef.Kind != "Service" {
				continue
			}

			serviceNamespace := tlsRoute.Namespace
			if backendRef.Namespace != nil {
				serviceNamespace = string(*backendRef.Namespace)
			}
			serviceName := string(backendRef.Name)

			if err := h.relationshipBuilder.CreateRouteForwardsToEdge(
				ctx,
				models.NodeTypeTLSRoute,
				tlsRoute.Namespace,
				tlsRoute.Name,
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

// HandleUpdate processes an updated TLSRoute
func (h *TLSRouteHandler) HandleUpdate(oldObj, newObj interface{}) {
	unstructuredObj, ok := newObj.(*unstructured.Unstructured)
	if !ok {
		h.Logger.Error("unexpected object type", zap.String("type", fmt.Sprintf("%T", newObj)))
		return
	}

	tlsRoute := &gatewayv1alpha2.TLSRoute{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.Object, tlsRoute); err != nil {
		h.Logger.Error("failed to convert to TLSRoute", zap.Error(err))
		return
	}

	h.Logger.Debug("tlsroute updated",
		zap.String("namespace", tlsRoute.Namespace),
		zap.String("name", tlsRoute.Name),
	)

	ctx := context.Background()
	tlsRouteID := models.GetNodeID("TLSRoute", tlsRoute.Namespace, tlsRoute.Name)

	// Delete old edges
	if err := h.GraphStore.DeleteEdgesByNode(ctx, string(models.NodeTypeTLSRoute), tlsRouteID); err != nil {
		h.Logger.Error("failed to delete old tlsroute edges", zap.Error(err))
	}

	// Recreate with new data
	h.HandleAdd(newObj)
}

// HandleDelete processes a deleted TLSRoute
func (h *TLSRouteHandler) HandleDelete(obj interface{}) {
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

	tlsRoute := &gatewayv1alpha2.TLSRoute{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.Object, tlsRoute); err != nil {
		h.Logger.Error("failed to convert to TLSRoute", zap.Error(err))
		return
	}

	h.Logger.Debug("tlsroute deleted",
		zap.String("namespace", tlsRoute.Namespace),
		zap.String("name", tlsRoute.Name),
	)

	ctx := context.Background()

	tlsRouteID := models.GetNodeID("TLSRoute", tlsRoute.Namespace, tlsRoute.Name)
	if err := h.GraphStore.DeleteNode(ctx, string(models.NodeTypeTLSRoute), tlsRouteID); err != nil {
		h.Logger.Error("failed to delete tlsroute node", zap.Error(err), zap.String("tlsroute", tlsRoute.Name))
	}
}
