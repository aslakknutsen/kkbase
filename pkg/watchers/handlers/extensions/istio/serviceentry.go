package istio

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

	istiov1 "istio.io/client-go/pkg/apis/networking/v1"
)

// ServiceEntryHandler handles Istio ServiceEntry resources
type ServiceEntryHandler struct {
	*watchers.BaseWatcher
	clientset           *kubernetes.Clientset
	dynamicClient       dynamic.Interface
	relationshipBuilder *watchers.RelationshipBuilder
}

// NewServiceEntryHandler creates a new ServiceEntry handler
func NewServiceEntryHandler(
	clientset *kubernetes.Clientset,
	dynamicClient dynamic.Interface,
	graphStore graph.GraphStore,
	logger *zap.Logger,
	factory dynamicinformer.DynamicSharedInformerFactory,
) *ServiceEntryHandler {
	gvr := istiov1.SchemeGroupVersion.WithResource("serviceentries")
	informer := factory.ForResource(gvr).Informer()

	handler := &ServiceEntryHandler{
		BaseWatcher:   watchers.NewBaseWatcher(graphStore, logger, informer),
		clientset:     clientset,
		dynamicClient: dynamicClient, relationshipBuilder: watchers.NewRelationshipBuilder(clientset, graphStore, logger),
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

// HandleAdd processes a newly added ServiceEntry
func (h *ServiceEntryHandler) HandleAdd(obj interface{}) {
	serviceEntry, err := watchers.ConvertToTyped[istiov1.ServiceEntry](obj)

	if err != nil {

		h.Logger.Error("failed to convert to ServiceEntry", zap.Error(err))

		return

	}

	h.Logger.Debug("serviceentry added",
		zap.String("namespace", serviceEntry.Namespace),
		zap.String("name", serviceEntry.Name),
	)

	ctx := context.Background()

	// Create ServiceEntry node
	seNode := models.ServiceEntryToGraphNode(serviceEntry)
	if err := h.GraphStore.UpsertNode(ctx, string(seNode.Type), seNode.ID, seNode.Properties); err != nil {
		h.Logger.Error("failed to create serviceentry node", zap.Error(err), zap.String("serviceentry", serviceEntry.Name))
		return
	}

	// Create IN_NAMESPACE edge (if namespaced)
	if serviceEntry.Namespace != "" {
		if err := h.relationshipBuilder.CreateNamespaceEdge(ctx, models.NodeTypeServiceEntry, seNode.ID, serviceEntry.Namespace); err != nil {
			h.Logger.Error("failed to create namespace edge", zap.Error(err))
		}
	}
}

// HandleUpdate processes an updated ServiceEntry
func (h *ServiceEntryHandler) HandleUpdate(oldObj, newObj interface{}) {
	serviceEntry, err := watchers.ConvertToTyped[istiov1.ServiceEntry](newObj)

	if err != nil {

		h.Logger.Error("failed to convert to ServiceEntry", zap.Error(err))

		return

	}

	h.Logger.Debug("serviceentry updated",
		zap.String("namespace", serviceEntry.Namespace),
		zap.String("name", serviceEntry.Name),
	)

	ctx := context.Background()
	seID := models.GetNodeID("ServiceEntry", serviceEntry.Namespace, serviceEntry.Name)

	// Delete old edges
	if err := h.GraphStore.DeleteEdgesByNode(ctx, string(models.NodeTypeServiceEntry), seID); err != nil {
		h.Logger.Error("failed to delete old serviceentry edges", zap.Error(err))
	}

	// Recreate with new data
	h.HandleAdd(newObj)
}

// HandleDelete processes a deleted ServiceEntry
func (h *ServiceEntryHandler) HandleDelete(obj interface{}) {
	serviceEntry, err := watchers.ConvertToTyped[istiov1.ServiceEntry](obj)

	if err != nil {

		h.Logger.Error("failed to convert to ServiceEntry", zap.Error(err))

		return

	}

	h.Logger.Debug("serviceentry deleted",
		zap.String("namespace", serviceEntry.Namespace),
		zap.String("name", serviceEntry.Name),
	)

	ctx := context.Background()

	seID := models.GetNodeID("ServiceEntry", serviceEntry.Namespace, serviceEntry.Name)
	if err := h.GraphStore.DeleteNode(ctx, string(models.NodeTypeServiceEntry), seID); err != nil {
		h.Logger.Error("failed to delete serviceentry node", zap.Error(err), zap.String("serviceentry", serviceEntry.Name))
	}
}
