package core

import (
	"context"

	"github.com/kagenti/kkbase/pkg/graph"
	"github.com/kagenti/kkbase/pkg/models"
	"github.com/kagenti/kkbase/pkg/watchers"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// DaemonSetHandler handles DaemonSet resources
type DaemonSetHandler struct {
	*watchers.BaseWatcher
	clientset           *kubernetes.Clientset
	relationshipBuilder *RelationshipBuilder
}

// NewDaemonSetHandler creates a new DaemonSet handler
func NewDaemonSetHandler(
	clientset *kubernetes.Clientset,
	graphStore graph.GraphStore,
	logger *zap.Logger,
	factory dynamicinformer.DynamicSharedInformerFactory,
) *DaemonSetHandler {
	gvr := schema.GroupVersionResource{
		Group:    "apps",
		Version:  "v1",
		Resource: "daemonsets",
	}
	informer := factory.ForResource(gvr).Informer()

	handler := &DaemonSetHandler{
		BaseWatcher:         watchers.NewBaseWatcher(graphStore, logger, informer),
		clientset:           clientset,
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

// HandleAdd processes a newly added DaemonSet
func (h *DaemonSetHandler) HandleAdd(obj interface{}) {
	daemonSet, err := watchers.ConvertToTyped[appsv1.DaemonSet](obj)
	if err != nil {
		h.Logger.Error("failed to convert to DaemonSet", zap.Error(err))
		return
	}

	h.Logger.Debug("daemonset added", zap.String("namespace", daemonSet.Namespace), zap.String("name", daemonSet.Name))

	ctx := context.Background()

	// Create DaemonSet node
	daemonSetNode := DaemonSetToGraphNode(daemonSet)
	if err := h.GraphStore.UpsertNode(ctx, string(daemonSetNode.Type), daemonSetNode.ID, daemonSetNode.Properties); err != nil {
		h.Logger.Error("failed to create daemonset node", zap.Error(err), zap.String("daemonset", daemonSet.Name))
		return
	}

	// Create IN_NAMESPACE edge
	if err := h.relationshipBuilder.CreateNamespaceEdge(ctx, NodeTypeDaemonSet, daemonSetNode.ID, daemonSet.Namespace); err != nil {
		h.Logger.Error("failed to create namespace edge", zap.Error(err))
	}
}

// HandleUpdate processes an updated DaemonSet
func (h *DaemonSetHandler) HandleUpdate(oldObj, newObj interface{}) {
	newDaemonSet, err := watchers.ConvertToTyped[appsv1.DaemonSet](newObj)
	if err != nil {
		h.Logger.Error("failed to convert to DaemonSet", zap.Error(err))
		return
	}

	h.Logger.Debug("daemonset updated", zap.String("namespace", newDaemonSet.Namespace), zap.String("name", newDaemonSet.Name))

	h.HandleAdd(newObj)
}

// HandleDelete processes a deleted DaemonSet
func (h *DaemonSetHandler) HandleDelete(obj interface{}) {
	daemonSet, err := watchers.ConvertToTyped[appsv1.DaemonSet](obj)
	if err != nil {
		h.Logger.Error("failed to convert to DaemonSet", zap.Error(err))
		return
	}

	h.Logger.Debug("daemonset deleted", zap.String("namespace", daemonSet.Namespace), zap.String("name", daemonSet.Name))

	ctx := context.Background()

	daemonSetID := models.GetNodeID(NodeTypeDaemonSet, daemonSet.Namespace, daemonSet.Name)
	if err := h.GraphStore.DeleteNode(ctx, string(NodeTypeDaemonSet), daemonSetID); err != nil {
		h.Logger.Error("failed to delete daemonset node", zap.Error(err), zap.String("daemonset", daemonSet.Name))
	}
}
