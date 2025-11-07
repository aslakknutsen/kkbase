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

// ConfigMapHandler handles ConfigMap resources
type ConfigMapHandler struct {
	*watchers.BaseWatcher
	relationshipBuilder *RelationshipBuilder
}

// NewConfigMapHandler creates a new ConfigMap handler
func NewConfigMapHandler(
	clientset *kubernetes.Clientset,
	graphStore graph.GraphStore,
	logger *zap.Logger,
	factory dynamicinformer.DynamicSharedInformerFactory,
) *ConfigMapHandler {
	gvr := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "configmaps",
	}
	informer := factory.ForResource(gvr).Informer()

	handler := &ConfigMapHandler{
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

// HandleAdd processes a newly added ConfigMap
func (h *ConfigMapHandler) HandleAdd(obj interface{}) {
	configMap, err := watchers.ConvertToTyped[corev1.ConfigMap](obj)
	if err != nil {
		h.Logger.Error("conversion failed", zap.Error(err))
		return
	}

	h.Logger.Debug("configmap added", zap.String("namespace", configMap.Namespace), zap.String("name", configMap.Name))

	ctx := context.Background()

	configMapNode := ConfigMapToGraphNode(configMap)
	if err := h.GraphStore.UpsertNode(ctx, string(configMapNode.Type), configMapNode.ID, configMapNode.Properties); err != nil {
		h.Logger.Error("failed to create configmap node", zap.Error(err), zap.String("configmap", configMap.Name))
		return
	}

	// Create IN_NAMESPACE edge
	if err := h.relationshipBuilder.CreateNamespaceEdge(ctx, NodeTypeConfigMap, configMapNode.ID, configMap.Namespace); err != nil {
		h.Logger.Error("failed to create namespace edge", zap.Error(err))
	}
}

// HandleUpdate processes an updated ConfigMap
func (h *ConfigMapHandler) HandleUpdate(oldObj, newObj interface{}) {
	newConfigMap, err := watchers.ConvertToTyped[corev1.ConfigMap](newObj)
	if err != nil {
		h.Logger.Error("conversion failed", zap.Error(err))
		return
	}

	h.Logger.Debug("configmap updated", zap.String("namespace", newConfigMap.Namespace), zap.String("name", newConfigMap.Name))
	h.HandleAdd(newObj)
}

// HandleDelete processes a deleted ConfigMap
func (h *ConfigMapHandler) HandleDelete(obj interface{}) {
	configMap, err := watchers.ConvertToTyped[corev1.ConfigMap](obj)
	if err != nil {
		h.Logger.Error("failed to convert to ConfigMap", zap.Error(err))
		return
	}

	h.Logger.Debug("configmap deleted", zap.String("namespace", configMap.Namespace), zap.String("name", configMap.Name))

	ctx := context.Background()

	configMapID := models.GetNodeID(NodeTypeConfigMap, configMap.Namespace, configMap.Name)
	if err := h.GraphStore.DeleteNode(ctx, string(NodeTypeConfigMap), configMapID); err != nil {
		h.Logger.Error("failed to delete configmap node", zap.Error(err), zap.String("configmap", configMap.Name))
	}
}
