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

// SecretHandler handles Secret resources
type SecretHandler struct {
	*watchers.BaseWatcher
	clientset           *kubernetes.Clientset
	relationshipBuilder *RelationshipBuilder
}

// NewSecretHandler creates a new Secret handler
func NewSecretHandler(
	clientset *kubernetes.Clientset,
	graphStore graph.GraphStore,
	logger *zap.Logger,
	factory dynamicinformer.DynamicSharedInformerFactory,
) *SecretHandler {
	gvr := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "secrets",
	}
	informer := factory.ForResource(gvr).Informer()

	handler := &SecretHandler{
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

// HandleAdd processes a newly added Secret
func (h *SecretHandler) HandleAdd(obj interface{}) {
	secret, err := watchers.ConvertToTyped[corev1.Secret](obj)
	if err != nil {
		h.Logger.Error("conversion failed", zap.Error(err))
		return
	}

	h.Logger.Debug("secret added",
		zap.String("namespace", secret.Namespace),
		zap.String("name", secret.Name),
		zap.String("type", string(secret.Type)),
	)

	ctx := context.Background()

	// Create Secret node
	secretNode := SecretToGraphNode(secret)
	if err := h.GraphStore.UpsertNode(ctx, string(secretNode.Type), secretNode.ID, secretNode.Properties); err != nil {
		h.Logger.Error("failed to create secret node", zap.Error(err), zap.String("secret", secret.Name))
		return
	}

	// Create IN_NAMESPACE edge
	if err := h.relationshipBuilder.CreateNamespaceEdge(ctx, NodeTypeSecret, secretNode.ID, secret.Namespace); err != nil {
		h.Logger.Error("failed to create namespace edge", zap.Error(err))
	}
}

// HandleUpdate processes an updated Secret
func (h *SecretHandler) HandleUpdate(oldObj, newObj interface{}) {
	newSecret, err := watchers.ConvertToTyped[corev1.Secret](newObj)
	if err != nil {
		h.Logger.Error("conversion failed", zap.Error(err))
		return
	}

	h.Logger.Debug("secret updated",
		zap.String("namespace", newSecret.Namespace),
		zap.String("name", newSecret.Name),
	)

	ctx := context.Background()
	secretID := models.GetNodeID("Secret", newSecret.Namespace, newSecret.Name)

	// Delete old edges
	if err := h.GraphStore.DeleteEdgesByNode(ctx, string(NodeTypeSecret), secretID); err != nil {
		h.Logger.Error("failed to delete old secret edges", zap.Error(err))
	}

	// Recreate with new data
	h.HandleAdd(newObj)
}

// HandleDelete processes a deleted Secret
func (h *SecretHandler) HandleDelete(obj interface{}) {
	secret, err := watchers.ConvertToTyped[corev1.Secret](obj)
	if err != nil {
		h.Logger.Error("failed to convert to Secret", zap.Error(err))
		return
	}

	h.Logger.Debug("secret deleted",
		zap.String("namespace", secret.Namespace),
		zap.String("name", secret.Name),
	)

	ctx := context.Background()

	secretID := models.GetNodeID("Secret", secret.Namespace, secret.Name)
	if err := h.GraphStore.DeleteNode(ctx, string(NodeTypeSecret), secretID); err != nil {
		h.Logger.Error("failed to delete secret node", zap.Error(err), zap.String("secret", secret.Name))
	}
}
