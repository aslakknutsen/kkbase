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
	factory dynamicinformer.DynamicSharedInformerFactory,
) *NodeHandler {
	gvr := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "nodes",
	}
	informer := factory.ForResource(gvr).Informer()

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
	node, err := watchers.ConvertToTyped[corev1.Node](obj)
	if err != nil {
		h.Logger.Error("failed to convert to Node", zap.Error(err))
		return
	}

	h.Logger.Debug("node added", zap.String("name", node.Name))

	ctx := context.Background()

	// Create Node node
	nodeNode := NodeToGraphNode(node)
	if err := h.GraphStore.UpsertNode(ctx, string(nodeNode.Type), nodeNode.ID, nodeNode.Properties); err != nil {
		h.Logger.Error("failed to create node", zap.Error(err), zap.String("node", node.Name))
		return
	}
}

// HandleUpdate processes an updated Node
func (h *NodeHandler) HandleUpdate(oldObj, newObj interface{}) {
	newNode, err := watchers.ConvertToTyped[corev1.Node](newObj)
	if err != nil {
		h.Logger.Error("failed to convert to Node", zap.Error(err))
		return
	}

	h.Logger.Debug("node updated", zap.String("name", newNode.Name))

	// For updates, we can reuse the add logic
	h.HandleAdd(newObj)
}

// HandleDelete processes a deleted Node
func (h *NodeHandler) HandleDelete(obj interface{}) {
	node, err := watchers.ConvertToTyped[corev1.Node](obj)
	if err != nil {
		h.Logger.Error("failed to convert to Node", zap.Error(err))
		return
	}

	h.Logger.Debug("node deleted", zap.String("name", node.Name))

	ctx := context.Background()

	nodeID := models.GetNodeID("Node", "", node.Name)
	if err := h.GraphStore.DeleteNode(ctx, string(NodeTypeNode), nodeID); err != nil {
		h.Logger.Error("failed to delete node", zap.Error(err), zap.String("node", node.Name))
	}
}
