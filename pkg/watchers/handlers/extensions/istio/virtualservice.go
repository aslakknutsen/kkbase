package istio

import (
	"context"
	"fmt"
	"strings"

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

// VirtualServiceHandler handles Istio VirtualService resources
type VirtualServiceHandler struct {
	*watchers.BaseWatcher
	clientset           *kubernetes.Clientset
	dynamicClient       dynamic.Interface
	relationshipBuilder *watchers.RelationshipBuilder
}

// NewVirtualServiceHandler creates a new VirtualService handler
func NewVirtualServiceHandler(
	clientset *kubernetes.Clientset,
	dynamicClient dynamic.Interface,
	graphStore graph.GraphStore,
	logger *zap.Logger,
	dynamicInformerFactory dynamicinformer.DynamicSharedInformerFactory,
) *VirtualServiceHandler {
	gvr := istiov1.SchemeGroupVersion.WithResource("virtualservices")
	informer := dynamicInformerFactory.ForResource(gvr).Informer()

	handler := &VirtualServiceHandler{
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

// HandleAdd processes a newly added VirtualService
func (h *VirtualServiceHandler) HandleAdd(obj interface{}) {
	unstructuredObj, ok := obj.(*unstructured.Unstructured)
	if !ok {
		h.Logger.Error("unexpected object type", zap.String("type", fmt.Sprintf("%T", obj)))
		return
	}

	vs := &istiov1.VirtualService{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.Object, vs); err != nil {
		h.Logger.Error("failed to convert to VirtualService", zap.Error(err))
		return
	}

	h.Logger.Debug("virtualservice added",
		zap.String("namespace", vs.Namespace),
		zap.String("name", vs.Name),
	)

	ctx := context.Background()

	// Create VirtualService node
	vsNode := models.VirtualServiceToGraphNode(vs)
	if err := h.GraphStore.UpsertNode(ctx, string(vsNode.Type), vsNode.ID, vsNode.Properties); err != nil {
		h.Logger.Error("failed to create virtualservice node", zap.Error(err), zap.String("virtualservice", vs.Name))
		return
	}

	// Create IN_NAMESPACE edge
	if err := h.relationshipBuilder.CreateNamespaceEdge(ctx, models.NodeTypeVirtualService, vsNode.ID, vs.Namespace); err != nil {
		h.Logger.Error("failed to create namespace edge", zap.Error(err))
	}

	// Create ATTACHES_TO edges to Istio Gateways
	for _, gateway := range vs.Spec.Gateways {
		// Parse gateway reference (format: namespace/name or just name)
		gatewayNs, gatewayName := parseGatewayRef(gateway, vs.Namespace)
		if err := h.relationshipBuilder.CreateVirtualServiceAttachesToEdge(ctx, vs.Namespace, vs.Name, gatewayNs, gatewayName); err != nil {
			h.Logger.Debug("failed to create ATTACHES_TO edge",
				zap.Error(err),
				zap.String("gateway", gateway),
			)
		}
	}

	// Create ROUTES_TRAFFIC_FOR edges to Services based on hosts
	for _, host := range vs.Spec.Hosts {
		// Parse service host (format: service.namespace.svc.cluster.local or just service)
		svcNs, svcName := parseServiceHost(host, vs.Namespace)
		if svcName != "" {
			if err := h.relationshipBuilder.CreateVirtualServiceRoutesTrafficForEdge(ctx, vs.Namespace, vs.Name, svcNs, svcName, host); err != nil {
				h.Logger.Debug("failed to create ROUTES_TRAFFIC_FOR edge",
					zap.Error(err),
					zap.String("host", host),
				)
			}
		}
	}

	// Create ROUTES_TO_SUBSET edges to DestinationRules for HTTP routes
	for _, httpRoute := range vs.Spec.Http {
		for _, dest := range httpRoute.Route {
			if dest.Destination != nil && dest.Destination.Subset != "" {
				// The destination rule will be in the same namespace or the service's namespace
				destNs, destService := parseServiceHost(dest.Destination.Host, vs.Namespace)
				weight := int32(0)
				if dest.Weight != 0 {
					weight = dest.Weight
				}
				if err := h.relationshipBuilder.CreateVirtualServiceRoutesToSubsetEdge(
					ctx, vs.Namespace, vs.Name, destNs, destService, dest.Destination.Subset, weight); err != nil {
					h.Logger.Debug("failed to create ROUTES_TO_SUBSET edge",
						zap.Error(err),
						zap.String("destination", dest.Destination.Host),
						zap.String("subset", dest.Destination.Subset),
					)
				}
			}
		}
	}
}

// HandleUpdate processes an updated VirtualService
func (h *VirtualServiceHandler) HandleUpdate(oldObj, newObj interface{}) {
	unstructuredObj, ok := newObj.(*unstructured.Unstructured)
	if !ok {
		h.Logger.Error("unexpected object type", zap.String("type", fmt.Sprintf("%T", newObj)))
		return
	}

	vs := &istiov1.VirtualService{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.Object, vs); err != nil {
		h.Logger.Error("failed to convert to VirtualService", zap.Error(err))
		return
	}

	h.Logger.Debug("virtualservice updated",
		zap.String("namespace", vs.Namespace),
		zap.String("name", vs.Name),
	)

	ctx := context.Background()
	vsID := models.GetNodeID("VirtualService", vs.Namespace, vs.Name)

	// Delete old edges
	if err := h.GraphStore.DeleteEdgesByNode(ctx, string(models.NodeTypeVirtualService), vsID); err != nil {
		h.Logger.Error("failed to delete old virtualservice edges", zap.Error(err))
	}

	// Recreate with new data
	h.HandleAdd(newObj)
}

// HandleDelete processes a deleted VirtualService
func (h *VirtualServiceHandler) HandleDelete(obj interface{}) {
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

	vs := &istiov1.VirtualService{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.Object, vs); err != nil {
		h.Logger.Error("failed to convert to VirtualService", zap.Error(err))
		return
	}

	h.Logger.Debug("virtualservice deleted",
		zap.String("namespace", vs.Namespace),
		zap.String("name", vs.Name),
	)

	ctx := context.Background()

	vsID := models.GetNodeID("VirtualService", vs.Namespace, vs.Name)
	if err := h.GraphStore.DeleteNode(ctx, string(models.NodeTypeVirtualService), vsID); err != nil {
		h.Logger.Error("failed to delete virtualservice node", zap.Error(err), zap.String("virtualservice", vs.Name))
	}
}

// parseGatewayRef parses a gateway reference in format "namespace/name" or just "name"
func parseGatewayRef(ref, defaultNamespace string) (string, string) {
	parts := strings.Split(ref, "/")
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return defaultNamespace, ref
}

// parseServiceHost parses a service host in format "service.namespace.svc.cluster.local" or "service"
func parseServiceHost(host, defaultNamespace string) (string, string) {
	// Skip wildcard or external hosts
	if strings.HasPrefix(host, "*") || strings.Contains(host, ".com") || strings.Contains(host, ".net") || strings.Contains(host, ".org") {
		return "", ""
	}

	parts := strings.Split(host, ".")
	if len(parts) >= 2 {
		// service.namespace or service.namespace.svc.cluster.local
		return parts[1], parts[0]
	}
	// Just service name
	return defaultNamespace, host
}
