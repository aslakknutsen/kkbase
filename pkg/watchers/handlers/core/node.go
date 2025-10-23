package core

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

// NodeHandler handles Node resources
type NodeHandler struct {
	*watchers.BaseWatcher
}

// NewNodeHandler creates a new Node handler
func NewNodeHandler(
	graphStore graph.GraphStore,
	logger *zap.Logger,
	informerFactory informers.SharedInformerFactory,
) *NodeHandler {
	informer := informerFactory.Core().V1().Nodes().Informer()

	handler := &NodeHandler{
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

// HandleAdd processes a newly added Node
func (h *NodeHandler) HandleAdd(obj interface{}) {
	node, ok := obj.(*corev1.Node)
	if !ok {
		h.Logger.Error("unexpected object type", zap.String("type", fmt.Sprintf("%T", obj)))
		return
	}

	h.Logger.Debug("node added", zap.String("name", node.Name))

	ctx := context.Background()

	// Create Node node
	nodeNode := models.NodeToGraphNode(node)
	if err := h.GraphStore.UpsertNode(ctx, string(nodeNode.Type), nodeNode.ID, nodeNode.Properties); err != nil {
		h.Logger.Error("failed to create node", zap.Error(err), zap.String("node", node.Name))
		return
	}
}

// HandleUpdate processes an updated Node
func (h *NodeHandler) HandleUpdate(oldObj, newObj interface{}) {
	newNode, ok := newObj.(*corev1.Node)
	if !ok {
		h.Logger.Error("unexpected object type", zap.String("type", fmt.Sprintf("%T", newObj)))
		return
	}

	h.Logger.Debug("node updated", zap.String("name", newNode.Name))

	// For updates, we can reuse the add logic
	h.HandleAdd(newObj)
}

// HandleDelete processes a deleted Node
func (h *NodeHandler) HandleDelete(obj interface{}) {
	node, ok := obj.(*corev1.Node)
	if !ok {
		extracted, err := watchers.SafeGetObject(obj)
		if err != nil {
			h.Logger.Error("failed to extract object", zap.Error(err))
			return
		}
		node, ok = extracted.(*corev1.Node)
		if !ok {
			h.Logger.Error("unexpected object type", zap.String("type", fmt.Sprintf("%T", extracted)))
			return
		}
	}

	h.Logger.Debug("node deleted", zap.String("name", node.Name))

	ctx := context.Background()

	if err := h.GraphStore.DeleteNode(ctx, string(models.NodeTypeNode), node.Name); err != nil {
		h.Logger.Error("failed to delete node", zap.Error(err), zap.String("node", node.Name))
	}
}
