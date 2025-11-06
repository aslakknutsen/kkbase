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

// TCPRouteHandler handles TCPRoute resources
type TCPRouteHandler struct {
	*watchers.BaseWatcher
	dynamicClient       dynamic.Interface
	relationshipBuilder *RelationshipBuilder
}

// NewTCPRouteHandler creates a new TCPRoute handler
func NewTCPRouteHandler(
	clientset *kubernetes.Clientset,
	dynamicClient dynamic.Interface,
	graphStore graph.GraphStore,
	logger *zap.Logger,
	factory dynamicinformer.DynamicSharedInformerFactory,
) *TCPRouteHandler {
	gvr := gatewayv1alpha2.SchemeGroupVersion.WithResource("tcproutes")
	informer := factory.ForResource(gvr).Informer()

	handler := &TCPRouteHandler{
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

// HandleAdd processes a newly added TCPRoute
func (h *TCPRouteHandler) HandleAdd(obj interface{}) {
	tcpRoute, err := watchers.ConvertToTyped[gatewayv1alpha2.TCPRoute](obj)

	if err != nil {

		h.Logger.Error("failed to convert to TCPRoute", zap.Error(err))

		return

	}

	h.Logger.Debug("tcproute added",
		zap.String("namespace", tcpRoute.Namespace),
		zap.String("name", tcpRoute.Name),
	)

	ctx := context.Background()

	// Create TCPRoute node
	tcpRouteNode := TCPRouteToGraphNode(tcpRoute)
	if err := h.GraphStore.UpsertNode(ctx, string(tcpRouteNode.Type), tcpRouteNode.ID, tcpRouteNode.Properties); err != nil {
		h.Logger.Error("failed to create tcproute node", zap.Error(err), zap.String("tcproute", tcpRoute.Name))
		return
	}

	// Create IN_NAMESPACE edge
	if err := h.relationshipBuilder.base.CreateNamespaceEdge(ctx, NodeTypeTCPRoute, tcpRouteNode.ID, tcpRoute.Namespace); err != nil {
		h.Logger.Error("failed to create namespace edge", zap.Error(err))
	}

	// Create ATTACHES_TO edges to parent Gateways
	for _, parentRef := range tcpRoute.Spec.ParentRefs {
		gatewayNamespace := tcpRoute.Namespace
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
			NodeTypeTCPRoute,
			tcpRoute.Namespace,
			tcpRoute.Name,
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
	for _, rule := range tcpRoute.Spec.Rules {
		for _, backendRef := range rule.BackendRefs {
			if backendRef.Kind != nil && *backendRef.Kind != "Service" {
				continue
			}

			serviceNamespace := tcpRoute.Namespace
			if backendRef.Namespace != nil {
				serviceNamespace = string(*backendRef.Namespace)
			}
			serviceName := string(backendRef.Name)

			if err := h.relationshipBuilder.CreateRouteForwardsToEdge(
				ctx,
				NodeTypeTCPRoute,
				tcpRoute.Namespace,
				tcpRoute.Name,
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

// HandleUpdate processes an updated TCPRoute
func (h *TCPRouteHandler) HandleUpdate(oldObj, newObj interface{}) {
	tcpRoute, err := watchers.ConvertToTyped[gatewayv1alpha2.TCPRoute](newObj)

	if err != nil {

		h.Logger.Error("failed to convert to TCPRoute", zap.Error(err))

		return

	}

	h.Logger.Debug("tcproute updated",
		zap.String("namespace", tcpRoute.Namespace),
		zap.String("name", tcpRoute.Name),
	)

	ctx := context.Background()
	tcpRouteID := models.GetNodeID("TCPRoute", tcpRoute.Namespace, tcpRoute.Name)

	// Delete old edges
	if err := h.GraphStore.DeleteEdgesByNode(ctx, string(NodeTypeTCPRoute), tcpRouteID); err != nil {
		h.Logger.Error("failed to delete old tcproute edges", zap.Error(err))
	}

	// Recreate with new data
	h.HandleAdd(newObj)
}

// HandleDelete processes a deleted TCPRoute
func (h *TCPRouteHandler) HandleDelete(obj interface{}) {
	tcpRoute, err := watchers.ConvertToTyped[gatewayv1alpha2.TCPRoute](obj)

	if err != nil {

		h.Logger.Error("failed to convert to TCPRoute", zap.Error(err))

		return

	}

	h.Logger.Debug("tcproute deleted",
		zap.String("namespace", tcpRoute.Namespace),
		zap.String("name", tcpRoute.Name),
	)

	ctx := context.Background()

	tcpRouteID := models.GetNodeID("TCPRoute", tcpRoute.Namespace, tcpRoute.Name)
	if err := h.GraphStore.DeleteNode(ctx, string(NodeTypeTCPRoute), tcpRouteID); err != nil {
		h.Logger.Error("failed to delete tcproute node", zap.Error(err), zap.String("tcproute", tcpRoute.Name))
	}
}
