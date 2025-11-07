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
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
)

// UDPRouteHandler handles UDPRoute resources
type UDPRouteHandler struct {
	*watchers.BaseWatcher
	dynamicClient       dynamic.Interface
	relationshipBuilder *RelationshipBuilder
}

// NewUDPRouteHandler creates a new UDPRoute handler
func NewUDPRouteHandler(
	clientset *kubernetes.Clientset,
	dynamicClient dynamic.Interface,
	graphStore graph.GraphStore,
	logger *zap.Logger,
	factory dynamicinformer.DynamicSharedInformerFactory,
) *UDPRouteHandler {
	gvr := gatewayv1alpha2.SchemeGroupVersion.WithResource("udproutes")
	informer := factory.ForResource(gvr).Informer()

	handler := &UDPRouteHandler{
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

// HandleAdd processes a newly added UDPRoute
func (h *UDPRouteHandler) HandleAdd(obj interface{}) {
	udpRoute, err := watchers.ConvertToTyped[gatewayv1alpha2.UDPRoute](obj)

	if err != nil {

		h.Logger.Error("failed to convert to UDPRoute", zap.Error(err))

		return

	}

	h.Logger.Debug("udproute added",
		zap.String("namespace", udpRoute.Namespace),
		zap.String("name", udpRoute.Name),
	)

	ctx := context.Background()

	// Create UDPRoute node
	udpRouteNode := UDPRouteToGraphNode(udpRoute)
	if err := h.GraphStore.UpsertNode(ctx, string(udpRouteNode.Type), udpRouteNode.ID, udpRouteNode.Properties); err != nil {
		h.Logger.Error("failed to create udproute node", zap.Error(err), zap.String("udproute", udpRoute.Name))
		return
	}

	// Create IN_NAMESPACE edge
	if err := h.relationshipBuilder.base.CreateNamespaceEdge(ctx, NodeTypeUDPRoute, udpRouteNode.ID, udpRoute.Namespace); err != nil {
		h.Logger.Error("failed to create namespace edge", zap.Error(err))
	}

	// Create ATTACHES_TO edges to parent Gateways
	for _, parentRef := range udpRoute.Spec.ParentRefs {
		gatewayNamespace := udpRoute.Namespace
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
			NodeTypeUDPRoute,
			udpRoute.Namespace,
			udpRoute.Name,
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
	for _, rule := range udpRoute.Spec.Rules {
		for _, backendRef := range rule.BackendRefs {
			if backendRef.Kind != nil && *backendRef.Kind != "Service" {
				continue
			}

			serviceNamespace := udpRoute.Namespace
			if backendRef.Namespace != nil {
				serviceNamespace = string(*backendRef.Namespace)
			}
			serviceName := string(backendRef.Name)

			if err := h.relationshipBuilder.CreateRouteForwardsToEdge(
				ctx,
				NodeTypeUDPRoute,
				udpRoute.Namespace,
				udpRoute.Name,
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

// HandleUpdate processes an updated UDPRoute
func (h *UDPRouteHandler) HandleUpdate(oldObj, newObj interface{}) {
	udpRoute, err := watchers.ConvertToTyped[gatewayv1alpha2.UDPRoute](newObj)

	if err != nil {

		h.Logger.Error("failed to convert to UDPRoute", zap.Error(err))

		return

	}

	h.Logger.Debug("udproute updated",
		zap.String("namespace", udpRoute.Namespace),
		zap.String("name", udpRoute.Name),
	)

	ctx := context.Background()
	udpRouteID := models.GetNodeID(NodeTypeUDPRoute, udpRoute.Namespace, udpRoute.Name)

	// Delete old edges
	if err := h.GraphStore.DeleteEdgesByNode(ctx, string(NodeTypeUDPRoute), udpRouteID); err != nil {
		h.Logger.Error("failed to delete old udproute edges", zap.Error(err))
	}

	// Recreate with new data
	h.HandleAdd(newObj)
}

// HandleDelete processes a deleted UDPRoute
func (h *UDPRouteHandler) HandleDelete(obj interface{}) {
	udpRoute, err := watchers.ConvertToTyped[gatewayv1alpha2.UDPRoute](obj)

	if err != nil {

		h.Logger.Error("failed to convert to UDPRoute", zap.Error(err))

		return

	}

	h.Logger.Debug("udproute deleted",
		zap.String("namespace", udpRoute.Namespace),
		zap.String("name", udpRoute.Name),
	)

	ctx := context.Background()

	udpRouteID := models.GetNodeID(NodeTypeUDPRoute, udpRoute.Namespace, udpRoute.Name)
	if err := h.GraphStore.DeleteNode(ctx, string(NodeTypeUDPRoute), udpRouteID); err != nil {
		h.Logger.Error("failed to delete udproute node", zap.Error(err), zap.String("udproute", udpRoute.Name))
	}
}
