package handlers

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

// PVCHandler handles PersistentVolumeClaim resources
type PVCHandler struct {
	*watchers.BaseWatcher
	relationshipBuilder *watchers.RelationshipBuilder
}

// NewPVCHandler creates a new PVC handler
func NewPVCHandler(
	clientset *kubernetes.Clientset,
	graphStore graph.GraphStore,
	logger *zap.Logger,
	informerFactory informers.SharedInformerFactory,
) *PVCHandler {
	informer := informerFactory.Core().V1().PersistentVolumeClaims().Informer()

	handler := &PVCHandler{
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

// HandleAdd processes a newly added PVC
func (h *PVCHandler) HandleAdd(obj interface{}) {
	pvc, ok := obj.(*corev1.PersistentVolumeClaim)
	if !ok {
		h.Logger.Error("unexpected object type", zap.String("type", fmt.Sprintf("%T", obj)))
		return
	}

	h.Logger.Debug("pvc added", zap.String("namespace", pvc.Namespace), zap.String("name", pvc.Name))

	ctx := context.Background()

	pvcNode := models.PersistentVolumeClaimToGraphNode(pvc)
	if err := h.GraphStore.UpsertNode(ctx, string(pvcNode.Type), pvcNode.ID, pvcNode.Properties); err != nil {
		h.Logger.Error("failed to create pvc node", zap.Error(err), zap.String("pvc", pvc.Name))
		return
	}

	// Create IN_NAMESPACE edge
	if err := h.relationshipBuilder.CreateNamespaceEdge(ctx, models.NodeTypePersistentVolumeClaim, pvcNode.ID, pvc.Namespace); err != nil {
		h.Logger.Error("failed to create namespace edge", zap.Error(err))
	}

	// Create BOUND_TO edge to PV if bound
	if err := h.relationshipBuilder.CreatePVCPVEdge(ctx, pvc); err != nil {
		h.Logger.Error("failed to create pvc-pv edge", zap.Error(err))
	}
}

// HandleUpdate processes an updated PVC
func (h *PVCHandler) HandleUpdate(oldObj, newObj interface{}) {
	newPVC, ok := newObj.(*corev1.PersistentVolumeClaim)
	if !ok {
		h.Logger.Error("unexpected object type", zap.String("type", fmt.Sprintf("%T", newObj)))
		return
	}

	h.Logger.Debug("pvc updated", zap.String("namespace", newPVC.Namespace), zap.String("name", newPVC.Name))
	h.HandleAdd(newObj)
}

// HandleDelete processes a deleted PVC
func (h *PVCHandler) HandleDelete(obj interface{}) {
	pvc, ok := obj.(*corev1.PersistentVolumeClaim)
	if !ok {
		extracted, err := watchers.SafeGetObject(obj)
		if err != nil {
			h.Logger.Error("failed to extract object", zap.Error(err))
			return
		}
		pvc, ok = extracted.(*corev1.PersistentVolumeClaim)
		if !ok {
			h.Logger.Error("unexpected object type", zap.String("type", fmt.Sprintf("%T", extracted)))
			return
		}
	}

	h.Logger.Debug("pvc deleted", zap.String("namespace", pvc.Namespace), zap.String("name", pvc.Name))

	ctx := context.Background()

	pvcID := models.GetNodeID("PersistentVolumeClaim", pvc.Namespace, pvc.Name)
	if err := h.GraphStore.DeleteNode(ctx, string(models.NodeTypePersistentVolumeClaim), pvcID); err != nil {
		h.Logger.Error("failed to delete pvc node", zap.Error(err), zap.String("pvc", pvc.Name))
	}
}
