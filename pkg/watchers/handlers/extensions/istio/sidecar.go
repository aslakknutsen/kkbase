package istio

import (
	"context"

	"github.com/aslakknutsen/kkbase/pkg/graph"
	"github.com/aslakknutsen/kkbase/pkg/models"
	"github.com/aslakknutsen/kkbase/pkg/watchers"
	"go.uber.org/zap"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	istiov1 "istio.io/client-go/pkg/apis/networking/v1"
)

// SidecarHandler handles Istio Sidecar resources
type SidecarHandler struct {
	*watchers.BaseWatcher
	clientset           *kubernetes.Clientset
	dynamicClient       dynamic.Interface
	relationshipBuilder *RelationshipBuilder
}

// NewSidecarHandler creates a new Sidecar handler
func NewSidecarHandler(
	clientset *kubernetes.Clientset,
	dynamicClient dynamic.Interface,
	graphStore graph.GraphStore,
	logger *zap.Logger,
	factory dynamicinformer.DynamicSharedInformerFactory,
) *SidecarHandler {
	gvr := istiov1.SchemeGroupVersion.WithResource("sidecars")
	informer := factory.ForResource(gvr).Informer()

	handler := &SidecarHandler{
		BaseWatcher:   watchers.NewBaseWatcher(graphStore, logger, informer),
		clientset:     clientset,
		dynamicClient: dynamicClient, relationshipBuilder: NewRelationshipBuilder(clientset, graphStore, logger),
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

// HandleAdd processes a newly added Sidecar
func (h *SidecarHandler) HandleAdd(obj interface{}) {
	sidecar, err := watchers.ConvertToTyped[istiov1.Sidecar](obj)

	if err != nil {

		h.Logger.Error("failed to convert to Sidecar", zap.Error(err))

		return

	}

	h.Logger.Debug("sidecar added",
		zap.String("namespace", sidecar.Namespace),
		zap.String("name", sidecar.Name),
	)

	ctx := context.Background()

	// Create Sidecar node
	sidecarNode := SidecarToGraphNode(sidecar)
	if err := h.GraphStore.UpsertNode(ctx, string(sidecarNode.Type), sidecarNode.ID, sidecarNode.Properties); err != nil {
		h.Logger.Error("failed to create sidecar node", zap.Error(err), zap.String("sidecar", sidecar.Name))
		return
	}

	// Create IN_NAMESPACE edge
	if err := h.relationshipBuilder.base.CreateNamespaceEdge(ctx, NodeTypeSidecar, sidecarNode.ID, sidecar.Namespace); err != nil {
		h.Logger.Error("failed to create namespace edge", zap.Error(err))
	}
}

// HandleUpdate processes an updated Sidecar
func (h *SidecarHandler) HandleUpdate(oldObj, newObj interface{}) {
	sidecar, err := watchers.ConvertToTyped[istiov1.Sidecar](newObj)

	if err != nil {

		h.Logger.Error("failed to convert to Sidecar", zap.Error(err))

		return

	}

	h.Logger.Debug("sidecar updated",
		zap.String("namespace", sidecar.Namespace),
		zap.String("name", sidecar.Name),
	)

	ctx := context.Background()
	sidecarID := models.GetNodeID(NodeTypeSidecar, sidecar.Namespace, sidecar.Name)

	// Delete old edges
	if err := h.GraphStore.DeleteEdgesByNode(ctx, string(NodeTypeSidecar), sidecarID); err != nil {
		h.Logger.Error("failed to delete old sidecar edges", zap.Error(err))
	}

	// Recreate with new data
	h.HandleAdd(newObj)
}

// HandleDelete processes a deleted Sidecar
func (h *SidecarHandler) HandleDelete(obj interface{}) {
	sidecar, err := watchers.ConvertToTyped[istiov1.Sidecar](obj)

	if err != nil {

		h.Logger.Error("failed to convert to Sidecar", zap.Error(err))

		return

	}

	h.Logger.Debug("sidecar deleted",
		zap.String("namespace", sidecar.Namespace),
		zap.String("name", sidecar.Name),
	)

	ctx := context.Background()

	sidecarID := models.GetNodeID(NodeTypeSidecar, sidecar.Namespace, sidecar.Name)
	if err := h.GraphStore.DeleteNode(ctx, string(NodeTypeSidecar), sidecarID); err != nil {
		h.Logger.Error("failed to delete sidecar node", zap.Error(err), zap.String("sidecar", sidecar.Name))
	}
}
