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
	informerFactory informers.SharedInformerFactory,
) *NamespaceHandler {
	informer := informerFactory.Core().V1().Namespaces().Informer()

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
	namespace, ok := obj.(*corev1.Namespace)
	if !ok {
		h.Logger.Error("unexpected object type", zap.String("type", fmt.Sprintf("%T", obj)))
		return
	}

	h.Logger.Debug("namespace added", zap.String("name", namespace.Name))

	ctx := context.Background()

	namespaceNode := models.NamespaceToGraphNode(namespace)
	if err := h.GraphStore.UpsertNode(ctx, string(namespaceNode.Type), namespaceNode.ID, namespaceNode.Properties); err != nil {
		h.Logger.Error("failed to create namespace node", zap.Error(err), zap.String("namespace", namespace.Name))
		return
	}
}

// HandleUpdate processes an updated Namespace
func (h *NamespaceHandler) HandleUpdate(oldObj, newObj interface{}) {
	newNamespace, ok := newObj.(*corev1.Namespace)
	if !ok {
		h.Logger.Error("unexpected object type", zap.String("type", fmt.Sprintf("%T", newObj)))
		return
	}

	h.Logger.Debug("namespace updated", zap.String("name", newNamespace.Name))
	h.HandleAdd(newObj)
}

// HandleDelete processes a deleted Namespace
func (h *NamespaceHandler) HandleDelete(obj interface{}) {
	namespace, ok := obj.(*corev1.Namespace)
	if !ok {
		extracted, err := watchers.SafeGetObject(obj)
		if err != nil {
			h.Logger.Error("failed to extract object", zap.Error(err))
			return
		}
		namespace, ok = extracted.(*corev1.Namespace)
		if !ok {
			h.Logger.Error("unexpected object type", zap.String("type", fmt.Sprintf("%T", extracted)))
			return
		}
	}

	h.Logger.Debug("namespace deleted", zap.String("name", namespace.Name))

	ctx := context.Background()

	if err := h.GraphStore.DeleteNode(ctx, string(models.NodeTypeNamespace), namespace.Name); err != nil {
		h.Logger.Error("failed to delete namespace node", zap.Error(err), zap.String("namespace", namespace.Name))
	}
}
