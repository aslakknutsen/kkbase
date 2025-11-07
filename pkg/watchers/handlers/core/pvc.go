package core

import (
	"context"

	"github.com/kagenti/kkbase/pkg/graph"
	"github.com/kagenti/kkbase/pkg/models"
	"github.com/kagenti/kkbase/pkg/watchers"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// PVCHandler handles PersistentVolumeClaim resources
type PVCHandler struct {
	*watchers.BaseWatcher
	relationshipBuilder *RelationshipBuilder
}

// NewPVCHandler creates a new PVC handler
func NewPVCHandler(
	clientset *kubernetes.Clientset,
	graphStore graph.GraphStore,
	logger *zap.Logger,
	factory dynamicinformer.DynamicSharedInformerFactory,
) *PVCHandler {
	gvr := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "persistentvolumeclaims",
	}
	informer := factory.ForResource(gvr).Informer()

	handler := &PVCHandler{
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

// HandleAdd processes a newly added PVC
func (h *PVCHandler) HandleAdd(obj interface{}) {
	pvc, err := watchers.ConvertToTyped[corev1.PersistentVolumeClaim](obj)
	if err != nil {
		h.Logger.Error("conversion failed", zap.Error(err))
		return
	}

	h.Logger.Debug("pvc added", zap.String("namespace", pvc.Namespace), zap.String("name", pvc.Name))

	ctx := context.Background()

	pvcNode := PersistentVolumeClaimToGraphNode(pvc)
	if err := h.GraphStore.UpsertNode(ctx, string(pvcNode.Type), pvcNode.ID, pvcNode.Properties); err != nil {
		h.Logger.Error("failed to create pvc node", zap.Error(err), zap.String("pvc", pvc.Name))
		return
	}

	// Create IN_NAMESPACE edge
	if err := h.relationshipBuilder.CreateNamespaceEdge(ctx, NodeTypePersistentVolumeClaim, pvcNode.ID, pvc.Namespace); err != nil {
		h.Logger.Error("failed to create namespace edge", zap.Error(err))
	}

	// Create BOUND_TO edge to PV if bound
	if err := h.relationshipBuilder.CreatePVCPVEdge(ctx, pvc); err != nil {
		h.Logger.Error("failed to create pvc-pv edge", zap.Error(err))
	}

	// Create PROVISIONED_BY edge to StorageClass
	if err := h.relationshipBuilder.CreatePVCStorageClassEdge(ctx, pvc); err != nil {
		h.Logger.Error("failed to create pvc-storageclass edge", zap.Error(err))
	}
}

// HandleUpdate processes an updated PVC
func (h *PVCHandler) HandleUpdate(oldObj, newObj interface{}) {
	newPVC, err := watchers.ConvertToTyped[corev1.PersistentVolumeClaim](newObj)
	if err != nil {
		h.Logger.Error("conversion failed", zap.Error(err))
		return
	}

	h.Logger.Debug("pvc updated", zap.String("namespace", newPVC.Namespace), zap.String("name", newPVC.Name))
	h.HandleAdd(newObj)
}

// HandleDelete processes a deleted PVC
func (h *PVCHandler) HandleDelete(obj interface{}) {
	pvc, err := watchers.ConvertToTyped[corev1.PersistentVolumeClaim](obj)
	if err != nil {
		h.Logger.Error("failed to convert to PersistentVolumeClaim", zap.Error(err))
		return
	}

	h.Logger.Debug("pvc deleted", zap.String("namespace", pvc.Namespace), zap.String("name", pvc.Name))

	ctx := context.Background()

	pvcID := models.GetNodeID(NodeTypePersistentVolumeClaim, pvc.Namespace, pvc.Name)
	if err := h.GraphStore.DeleteNode(ctx, string(NodeTypePersistentVolumeClaim), pvcID); err != nil {
		h.Logger.Error("failed to delete pvc node", zap.Error(err), zap.String("pvc", pvc.Name))
	}
}
