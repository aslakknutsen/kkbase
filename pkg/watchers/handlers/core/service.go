package core

import (
	"context"
	"fmt"

	"github.com/kagenti/kkbase/pkg/graph"
	"github.com/kagenti/kkbase/pkg/models"
	"github.com/kagenti/kkbase/pkg/watchers"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// ServiceHandler handles Service resources
type ServiceHandler struct {
	*watchers.BaseWatcher
	clientset           *kubernetes.Clientset
	relationshipBuilder *watchers.RelationshipBuilder
}

// NewServiceHandler creates a new Service handler
func NewServiceHandler(
	clientset *kubernetes.Clientset,
	graphStore graph.GraphStore,
	logger *zap.Logger,
	informerFactory informers.SharedInformerFactory,
) *ServiceHandler {
	informer := informerFactory.Core().V1().Services().Informer()

	handler := &ServiceHandler{
		BaseWatcher:         watchers.NewBaseWatcher(graphStore, logger, informer),
		clientset:           clientset,
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

// HandleAdd processes a newly added Service
func (h *ServiceHandler) HandleAdd(obj interface{}) {
	service, ok := obj.(*corev1.Service)
	if !ok {
		h.Logger.Error("unexpected object type", zap.String("type", fmt.Sprintf("%T", obj)))
		return
	}

	h.Logger.Debug("service added", zap.String("namespace", service.Namespace), zap.String("name", service.Name))

	ctx := context.Background()

	// Create Service node
	serviceNode := models.ServiceToGraphNode(service)
	if err := h.GraphStore.UpsertNode(ctx, string(serviceNode.Type), serviceNode.ID, serviceNode.Properties); err != nil {
		h.Logger.Error("failed to create service node", zap.Error(err), zap.String("service", service.Name))
		return
	}

	// Create IN_NAMESPACE edge
	if err := h.relationshipBuilder.CreateNamespaceEdge(ctx, models.NodeTypeService, serviceNode.ID, service.Namespace); err != nil {
		h.Logger.Error("failed to create namespace edge", zap.Error(err))
	}

	// Create SELECTS_PODS edges
	if err := h.relationshipBuilder.CreateServicePodEdges(ctx, service); err != nil {
		h.Logger.Error("failed to create service-pod edges", zap.Error(err))
	}
}

// HandleUpdate processes an updated Service
func (h *ServiceHandler) HandleUpdate(oldObj, newObj interface{}) {
	newService, ok := newObj.(*corev1.Service)
	if !ok {
		h.Logger.Error("unexpected object type", zap.String("type", fmt.Sprintf("%T", newObj)))
		return
	}

	h.Logger.Debug("service updated", zap.String("namespace", newService.Namespace), zap.String("name", newService.Name))

	// For services, we need to delete old edges and recreate them
	ctx := context.Background()
	serviceID := models.GetNodeID("Service", newService.Namespace, newService.Name)

	// Delete old edges
	if err := h.GraphStore.DeleteEdgesByNode(ctx, string(models.NodeTypeService), serviceID); err != nil {
		h.Logger.Error("failed to delete old service edges", zap.Error(err))
	}

	// Recreate with new data
	h.HandleAdd(newObj)
}

// HandleDelete processes a deleted Service
func (h *ServiceHandler) HandleDelete(obj interface{}) {
	service, ok := obj.(*corev1.Service)
	if !ok {
		extracted, err := watchers.SafeGetObject(obj)
		if err != nil {
			h.Logger.Error("failed to extract object", zap.Error(err))
			return
		}
		service, ok = extracted.(*corev1.Service)
		if !ok {
			h.Logger.Error("unexpected object type", zap.String("type", fmt.Sprintf("%T", extracted)))
			return
		}
	}

	h.Logger.Debug("service deleted", zap.String("namespace", service.Namespace), zap.String("name", service.Name))

	ctx := context.Background()

	serviceID := models.GetNodeID("Service", service.Namespace, service.Name)
	if err := h.GraphStore.DeleteNode(ctx, string(models.NodeTypeService), serviceID); err != nil {
		h.Logger.Error("failed to delete service node", zap.Error(err), zap.String("service", service.Name))
	}
}
