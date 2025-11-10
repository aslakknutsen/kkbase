package core

import (
	"context"

	"github.com/aslakknutsen/kkbase/pkg/graph"
	"github.com/aslakknutsen/kkbase/pkg/watchers"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// PVHandler handles PersistentVolume resources
type PVHandler struct {
	*watchers.BaseWatcher
	relationshipBuilder *RelationshipBuilder
}

// NewPVHandler creates a new PV handler
func NewPVHandler(
	clientset *kubernetes.Clientset,
	graphStore graph.GraphStore,
	logger *zap.Logger,
	factory dynamicinformer.DynamicSharedInformerFactory,
) *PVHandler {
	gvr := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "persistentvolumes",
	}
	informer := factory.ForResource(gvr).Informer()

	handler := &PVHandler{
		BaseWatcher:         watchers.NewBaseWatcher(graphStore, logger, informer),
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

// HandleAdd processes a newly added PV
func (h *PVHandler) HandleAdd(obj interface{}) {
	pv, err := watchers.ConvertToTyped[corev1.PersistentVolume](obj)
	if err != nil {
		h.Logger.Error("conversion failed", zap.Error(err))
		return
	}

	h.Logger.Debug("pv added", zap.String("name", pv.Name))

	ctx := context.Background()

	pvNode := PersistentVolumeToGraphNode(pv)
	if err := h.GraphStore.UpsertNode(ctx, string(pvNode.Type), pvNode.ID, pvNode.Properties); err != nil {
		h.Logger.Error("failed to create pv node", zap.Error(err), zap.String("pv", pv.Name))
		return
	}

	// Create PROVISIONED_BY edge to StorageClass
	if err := h.relationshipBuilder.CreatePVStorageClassEdge(ctx, pv); err != nil {
		h.Logger.Error("failed to create pv-storageclass edge", zap.Error(err))
	}
}

// HandleUpdate processes an updated PV
func (h *PVHandler) HandleUpdate(oldObj, newObj interface{}) {
	newPV, err := watchers.ConvertToTyped[corev1.PersistentVolume](newObj)
	if err != nil {
		h.Logger.Error("conversion failed", zap.Error(err))
		return
	}

	h.Logger.Debug("pv updated", zap.String("name", newPV.Name))
	h.HandleAdd(newObj)
}

// HandleDelete processes a deleted PV
func (h *PVHandler) HandleDelete(obj interface{}) {
	pv, err := watchers.ConvertToTyped[corev1.PersistentVolume](obj)
	if err != nil {
		h.Logger.Error("failed to convert to PersistentVolume", zap.Error(err))
		return
	}

	h.Logger.Debug("pv deleted", zap.String("name", pv.Name))

	ctx := context.Background()

	if err := h.GraphStore.DeleteNode(ctx, string(NodeTypePersistentVolume), pv.Name); err != nil {
		h.Logger.Error("failed to delete pv node", zap.Error(err), zap.String("pv", pv.Name))
	}
}
