package core

import (
	"context"

	"github.com/kagenti/kkbase/pkg/graph"
	"github.com/kagenti/kkbase/pkg/models"
	"github.com/kagenti/kkbase/pkg/watchers"
	"go.uber.org/zap"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// StorageClassHandler handles StorageClass resources
type StorageClassHandler struct {
	*watchers.BaseWatcher
	relationshipBuilder *watchers.RelationshipBuilder
}

// NewStorageClassHandler creates a new StorageClass handler
func NewStorageClassHandler(
	clientset *kubernetes.Clientset,
	graphStore graph.GraphStore,
	logger *zap.Logger,
	factory dynamicinformer.DynamicSharedInformerFactory,
) *StorageClassHandler {
	gvr := schema.GroupVersionResource{
		Group:    "storage.k8s.io",
		Version:  "v1",
		Resource: "storageclasses",
	}
	informer := factory.ForResource(gvr).Informer()

	handler := &StorageClassHandler{
		BaseWatcher:         watchers.NewBaseWatcher(graphStore, logger, informer),
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

// HandleAdd processes a newly added StorageClass
func (h *StorageClassHandler) HandleAdd(obj interface{}) {
	storageClass, err := watchers.ConvertToTyped[storagev1.StorageClass](obj)
	if err != nil {
		h.Logger.Error("conversion failed", zap.Error(err))
		return
	}

	h.Logger.Debug("storageclass added", zap.String("name", storageClass.Name))

	ctx := context.Background()

	storageClassNode := models.StorageClassToGraphNode(storageClass)
	if err := h.GraphStore.UpsertNode(ctx, string(storageClassNode.Type), storageClassNode.ID, storageClassNode.Properties); err != nil {
		h.Logger.Error("failed to create storageclass node", zap.Error(err), zap.String("storageclass", storageClass.Name))
		return
	}
}

// HandleUpdate processes an updated StorageClass
func (h *StorageClassHandler) HandleUpdate(oldObj, newObj interface{}) {
	newStorageClass, err := watchers.ConvertToTyped[storagev1.StorageClass](newObj)
	if err != nil {
		h.Logger.Error("conversion failed", zap.Error(err))
		return
	}

	h.Logger.Debug("storageclass updated", zap.String("name", newStorageClass.Name))
	h.HandleAdd(newObj)
}

// HandleDelete processes a deleted StorageClass
func (h *StorageClassHandler) HandleDelete(obj interface{}) {
	storageClass, err := watchers.ConvertToTyped[storagev1.StorageClass](obj)
	if err != nil {
		h.Logger.Error("failed to convert to StorageClass", zap.Error(err))
		return
	}

	h.Logger.Debug("storageclass deleted", zap.String("name", storageClass.Name))

	ctx := context.Background()

	if err := h.GraphStore.DeleteNode(ctx, string(models.NodeTypeStorageClass), storageClass.Name); err != nil {
		h.Logger.Error("failed to delete storageclass node", zap.Error(err), zap.String("storageclass", storageClass.Name))
	}
}
