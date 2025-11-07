package gateway

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
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// HTTPRouteHandler handles HTTPRoute resources
type HTTPRouteHandler struct {
	*watchers.BaseWatcher
	dynamicClient       dynamic.Interface
	relationshipBuilder *RelationshipBuilder
}

// NewHTTPRouteHandler creates a new HTTPRoute handler
func NewHTTPRouteHandler(
	clientset *kubernetes.Clientset,
	dynamicClient dynamic.Interface,
	graphStore graph.GraphStore,
	logger *zap.Logger,
	factory dynamicinformer.DynamicSharedInformerFactory,
) *HTTPRouteHandler {
	gvr := gatewayv1.SchemeGroupVersion.WithResource("httproutes")
	informer := factory.ForResource(gvr).Informer()

	handler := &HTTPRouteHandler{
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

// HandleAdd processes a newly added HTTPRoute
func (h *HTTPRouteHandler) HandleAdd(obj interface{}) {
	httpRoute, err := watchers.ConvertToTyped[gatewayv1.HTTPRoute](obj)
	if err != nil {
		h.Logger.Error("failed to convert to HTTPRoute", zap.Error(err))
		return
	}

	h.Logger.Debug("httproute added",
		zap.String("namespace", httpRoute.Namespace),
		zap.String("name", httpRoute.Name),
	)

	ctx := context.Background()

	// Create HTTPRoute node
	httpRouteNode := HTTPRouteToGraphNode(httpRoute)
	if err := h.GraphStore.UpsertNode(ctx, string(httpRouteNode.Type), httpRouteNode.ID, httpRouteNode.Properties); err != nil {
		h.Logger.Error("failed to create httproute node", zap.Error(err), zap.String("httproute", httpRoute.Name))
		return
	}

	// Create IN_NAMESPACE edge
	if err := h.relationshipBuilder.base.CreateNamespaceEdge(ctx, NodeTypeHTTPRoute, httpRouteNode.ID, httpRoute.Namespace); err != nil {
		h.Logger.Error("failed to create namespace edge", zap.Error(err))
	}

	// Create ATTACHES_TO edges to parent Gateways
	for _, parentRef := range httpRoute.Spec.ParentRefs {
		gatewayNamespace := httpRoute.Namespace
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
			NodeTypeHTTPRoute,
			httpRoute.Namespace,
			httpRoute.Name,
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
	for _, rule := range httpRoute.Spec.Rules {
		if rule.BackendRefs != nil {
			for _, backendRef := range rule.BackendRefs {
				// Only handle Service backends (skip other kinds)
				if backendRef.Kind != nil && *backendRef.Kind != "Service" {
					continue
				}

				serviceNamespace := httpRoute.Namespace
				if backendRef.Namespace != nil {
					serviceNamespace = string(*backendRef.Namespace)
				}
				serviceName := string(backendRef.Name)

				// Check for cross-namespace reference
				if serviceNamespace != httpRoute.Namespace {
					h.Logger.Debug("cross-namespace backend reference detected",
						zap.String("route", httpRoute.Name),
						zap.String("route_namespace", httpRoute.Namespace),
						zap.String("service", serviceName),
						zap.String("service_namespace", serviceNamespace),
					)
					// Note: ReferenceGrant checking would happen here in a full implementation
				}

				if err := h.relationshipBuilder.CreateRouteForwardsToEdge(
					ctx,
					NodeTypeHTTPRoute,
					httpRoute.Namespace,
					httpRoute.Name,
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

// HandleUpdate processes an updated HTTPRoute
func (h *HTTPRouteHandler) HandleUpdate(oldObj, newObj interface{}) {
	httpRoute, err := watchers.ConvertToTyped[gatewayv1.HTTPRoute](newObj)
	if err != nil {
		h.Logger.Error("failed to convert to HTTPRoute", zap.Error(err))
		return
	}

	h.Logger.Debug("httproute updated",
		zap.String("namespace", httpRoute.Namespace),
		zap.String("name", httpRoute.Name),
	)

	ctx := context.Background()
	httpRouteID := models.GetNodeID(NodeTypeHTTPRoute, httpRoute.Namespace, httpRoute.Name)

	// Delete old edges
	if err := h.GraphStore.DeleteEdgesByNode(ctx, string(NodeTypeHTTPRoute), httpRouteID); err != nil {
		h.Logger.Error("failed to delete old httproute edges", zap.Error(err))
	}

	// Recreate with new data
	h.HandleAdd(newObj)
}

// HandleDelete processes a deleted HTTPRoute
func (h *HTTPRouteHandler) HandleDelete(obj interface{}) {
	httpRoute, err := watchers.ConvertToTyped[gatewayv1.HTTPRoute](obj)
	if err != nil {
		h.Logger.Error("failed to convert to HTTPRoute", zap.Error(err))
		return
	}

	h.Logger.Debug("httproute deleted",
		zap.String("namespace", httpRoute.Namespace),
		zap.String("name", httpRoute.Name),
	)

	ctx := context.Background()

	httpRouteID := models.GetNodeID(NodeTypeHTTPRoute, httpRoute.Namespace, httpRoute.Name)
	if err := h.GraphStore.DeleteNode(ctx, string(NodeTypeHTTPRoute), httpRouteID); err != nil {
		h.Logger.Error("failed to delete httproute node", zap.Error(err), zap.String("httproute", httpRoute.Name))
	}
}
