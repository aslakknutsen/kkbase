package core

import (
	"context"

	"github.com/kagenti/kkbase/pkg/graph"
	"github.com/kagenti/kkbase/pkg/watchers"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
)

// NamespaceHandler handles Namespace resources
type NamespaceHandler struct {
	*watchers.BaseWatcher
}

// NewNamespaceHandler creates a new Namespace handler
func NewNamespaceHandler(
	graphStore graph.GraphStore,
	logger *zap.Logger,
	factory dynamicinformer.DynamicSharedInformerFactory,
) *NamespaceHandler {
	gvr := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "namespaces",
	}
	informer := factory.ForResource(gvr).Informer()

	handler := &NamespaceHandler{
		BaseWatcher: watchers.NewBaseWatcher(graphStore, logger, informer),
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

// HandleAdd processes a newly added Namespace
func (h *NamespaceHandler) HandleAdd(obj interface{}) {
	namespace, err := watchers.ConvertToTyped[corev1.Namespace](obj)
	if err != nil {
		h.Logger.Error("failed to convert to Namespace", zap.Error(err))
		return
	}

	h.Logger.Debug("namespace added", zap.String("name", namespace.Name))

	ctx := context.Background()

	namespaceNode := NamespaceToGraphNode(namespace)
	if err := h.GraphStore.UpsertNode(ctx, string(namespaceNode.Type), namespaceNode.ID, namespaceNode.Properties); err != nil {
		h.Logger.Error("failed to create namespace node", zap.Error(err), zap.String("namespace", namespace.Name))
		return
	}
}

// HandleUpdate processes an updated Namespace
func (h *NamespaceHandler) HandleUpdate(oldObj, newObj interface{}) {
	newNamespace, err := watchers.ConvertToTyped[corev1.Namespace](newObj)
	if err != nil {
		h.Logger.Error("failed to convert to Namespace", zap.Error(err))
		return
	}

	h.Logger.Debug("namespace updated", zap.String("name", newNamespace.Name))
	h.HandleAdd(newObj)
}

// HandleDelete processes a deleted Namespace
func (h *NamespaceHandler) HandleDelete(obj interface{}) {
	namespace, err := watchers.ConvertToTyped[corev1.Namespace](obj)
	if err != nil {
		h.Logger.Error("failed to convert to Namespace", zap.Error(err))
		return
	}

	h.Logger.Debug("namespace deleted", zap.String("name", namespace.Name))

	ctx := context.Background()

	if err := h.GraphStore.DeleteNode(ctx, string(NodeTypeNamespace), namespace.Name); err != nil {
		h.Logger.Error("failed to delete namespace node", zap.Error(err), zap.String("namespace", namespace.Name))
	}
}
