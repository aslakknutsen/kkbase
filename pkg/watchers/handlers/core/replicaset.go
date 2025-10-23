package core

import (
	"context"
	"fmt"

	"github.com/kagenti/kkbase/pkg/graph"
	"github.com/kagenti/kkbase/pkg/models"
	"github.com/kagenti/kkbase/pkg/watchers"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// ReplicaSetHandler handles ReplicaSet resources
type ReplicaSetHandler struct {
	*watchers.BaseWatcher
	clientset           *kubernetes.Clientset
	relationshipBuilder *watchers.RelationshipBuilder
}

// NewReplicaSetHandler creates a new ReplicaSet handler
func NewReplicaSetHandler(
	clientset *kubernetes.Clientset,
	graphStore graph.GraphStore,
	logger *zap.Logger,
	informerFactory informers.SharedInformerFactory,
) *ReplicaSetHandler {
	informer := informerFactory.Apps().V1().ReplicaSets().Informer()

	handler := &ReplicaSetHandler{
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

// HandleAdd processes a newly added ReplicaSet
func (h *ReplicaSetHandler) HandleAdd(obj interface{}) {
	replicaSet, ok := obj.(*appsv1.ReplicaSet)
	if !ok {
		h.Logger.Error("unexpected object type", zap.String("type", fmt.Sprintf("%T", obj)))
		return
	}

	h.Logger.Debug("replicaset added", zap.String("namespace", replicaSet.Namespace), zap.String("name", replicaSet.Name))

	ctx := context.Background()

	// Create ReplicaSet node
	replicaSetNode := models.ReplicaSetToGraphNode(replicaSet)
	if err := h.GraphStore.UpsertNode(ctx, string(replicaSetNode.Type), replicaSetNode.ID, replicaSetNode.Properties); err != nil {
		h.Logger.Error("failed to create replicaset node", zap.Error(err), zap.String("replicaset", replicaSet.Name))
		return
	}

	// Create IN_NAMESPACE edge
	if err := h.relationshipBuilder.CreateNamespaceEdge(ctx, models.NodeTypeReplicaSet, replicaSetNode.ID, replicaSet.Namespace); err != nil {
		h.Logger.Error("failed to create namespace edge", zap.Error(err))
	}

	// Create owner reference edges (typically to Deployment)
	if ownerRef := models.GetOwnerReference(replicaSet.OwnerReferences); ownerRef != nil {
		if err := h.relationshipBuilder.CreateOwnerEdge(ctx, models.NodeTypeReplicaSet, replicaSetNode.ID, *ownerRef, replicaSet.Namespace); err != nil {
			h.Logger.Error("failed to create owner edge", zap.Error(err))
		}
	}
}

// HandleUpdate processes an updated ReplicaSet
func (h *ReplicaSetHandler) HandleUpdate(oldObj, newObj interface{}) {
	newReplicaSet, ok := newObj.(*appsv1.ReplicaSet)
	if !ok {
		h.Logger.Error("unexpected object type", zap.String("type", fmt.Sprintf("%T", newObj)))
		return
	}

	h.Logger.Debug("replicaset updated", zap.String("namespace", newReplicaSet.Namespace), zap.String("name", newReplicaSet.Name))

	h.HandleAdd(newObj)
}

// HandleDelete processes a deleted ReplicaSet
func (h *ReplicaSetHandler) HandleDelete(obj interface{}) {
	replicaSet, ok := obj.(*appsv1.ReplicaSet)
	if !ok {
		extracted, err := watchers.SafeGetObject(obj)
		if err != nil {
			h.Logger.Error("failed to extract object", zap.Error(err))
			return
		}
		replicaSet, ok = extracted.(*appsv1.ReplicaSet)
		if !ok {
			h.Logger.Error("unexpected object type", zap.String("type", fmt.Sprintf("%T", extracted)))
			return
		}
	}

	h.Logger.Debug("replicaset deleted", zap.String("namespace", replicaSet.Namespace), zap.String("name", replicaSet.Name))

	ctx := context.Background()

	replicaSetID := models.GetNodeID("ReplicaSet", replicaSet.Namespace, replicaSet.Name)
	if err := h.GraphStore.DeleteNode(ctx, string(models.NodeTypeReplicaSet), replicaSetID); err != nil {
		h.Logger.Error("failed to delete replicaset node", zap.Error(err), zap.String("replicaset", replicaSet.Name))
	}
}
