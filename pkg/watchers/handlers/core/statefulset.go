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

// StatefulSetHandler handles StatefulSet resources
type StatefulSetHandler struct {
	*watchers.BaseWatcher
	clientset           *kubernetes.Clientset
	relationshipBuilder *RelationshipBuilder
}

// NewStatefulSetHandler creates a new StatefulSet handler
func NewStatefulSetHandler(
	clientset *kubernetes.Clientset,
	graphStore graph.GraphStore,
	logger *zap.Logger,
	factory dynamicinformer.DynamicSharedInformerFactory,
) *StatefulSetHandler {
	gvr := schema.GroupVersionResource{
		Group:    "apps",
		Version:  "v1",
		Resource: "statefulsets",
	}
	informer := factory.ForResource(gvr).Informer()

	handler := &StatefulSetHandler{
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

// HandleAdd processes a newly added StatefulSet
func (h *StatefulSetHandler) HandleAdd(obj interface{}) {
	statefulSet, err := watchers.ConvertToTyped[appsv1.StatefulSet](obj)
	if err != nil {
		h.Logger.Error("failed to convert to StatefulSet", zap.Error(err))
		return
	}

	h.Logger.Debug("statefulset added", zap.String("namespace", statefulSet.Namespace), zap.String("name", statefulSet.Name))

	ctx := context.Background()

	// Create StatefulSet node
	statefulSetNode := StatefulSetToGraphNode(statefulSet)
	if err := h.GraphStore.UpsertNode(ctx, string(statefulSetNode.Type), statefulSetNode.ID, statefulSetNode.Properties); err != nil {
		h.Logger.Error("failed to create statefulset node", zap.Error(err), zap.String("statefulset", statefulSet.Name))
		return
	}

	// Create IN_NAMESPACE edge
	if err := h.relationshipBuilder.CreateNamespaceEdge(ctx, NodeTypeStatefulSet, statefulSetNode.ID, statefulSet.Namespace); err != nil {
		h.Logger.Error("failed to create namespace edge", zap.Error(err))
	}
}

// HandleUpdate processes an updated StatefulSet
func (h *StatefulSetHandler) HandleUpdate(oldObj, newObj interface{}) {
	newStatefulSet, err := watchers.ConvertToTyped[appsv1.StatefulSet](newObj)
	if err != nil {
		h.Logger.Error("failed to convert to StatefulSet", zap.Error(err))
		return
	}

	h.Logger.Debug("statefulset updated", zap.String("namespace", newStatefulSet.Namespace), zap.String("name", newStatefulSet.Name))

	h.HandleAdd(newObj)
}

// HandleDelete processes a deleted StatefulSet
func (h *StatefulSetHandler) HandleDelete(obj interface{}) {
	statefulSet, err := watchers.ConvertToTyped[appsv1.StatefulSet](obj)
	if err != nil {
		h.Logger.Error("failed to convert to StatefulSet", zap.Error(err))
		return
	}

	h.Logger.Debug("statefulset deleted", zap.String("namespace", statefulSet.Namespace), zap.String("name", statefulSet.Name))

	ctx := context.Background()

	statefulSetID := models.GetNodeID(NodeTypeStatefulSet, statefulSet.Namespace, statefulSet.Name)
	if err := h.GraphStore.DeleteNode(ctx, string(NodeTypeStatefulSet), statefulSetID); err != nil {
		h.Logger.Error("failed to delete statefulset node", zap.Error(err), zap.String("statefulset", statefulSet.Name))
	}
}
