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

// ConfigMapHandler handles ConfigMap resources
type ConfigMapHandler struct {
	*watchers.BaseWatcher
	relationshipBuilder *watchers.RelationshipBuilder
}

// NewConfigMapHandler creates a new ConfigMap handler
func NewConfigMapHandler(
	clientset *kubernetes.Clientset,
	graphStore graph.GraphStore,
	logger *zap.Logger,
	informerFactory informers.SharedInformerFactory,
) *ConfigMapHandler {
	informer := informerFactory.Core().V1().ConfigMaps().Informer()

	handler := &ConfigMapHandler{
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

// HandleAdd processes a newly added ConfigMap
func (h *ConfigMapHandler) HandleAdd(obj interface{}) {
	configMap, ok := obj.(*corev1.ConfigMap)
	if !ok {
		h.Logger.Error("unexpected object type", zap.String("type", fmt.Sprintf("%T", obj)))
		return
	}

	h.Logger.Debug("configmap added", zap.String("namespace", configMap.Namespace), zap.String("name", configMap.Name))

	ctx := context.Background()

	configMapNode := models.ConfigMapToGraphNode(configMap)
	if err := h.GraphStore.UpsertNode(ctx, string(configMapNode.Type), configMapNode.ID, configMapNode.Properties); err != nil {
		h.Logger.Error("failed to create configmap node", zap.Error(err), zap.String("configmap", configMap.Name))
		return
	}

	// Create IN_NAMESPACE edge
	if err := h.relationshipBuilder.CreateNamespaceEdge(ctx, models.NodeTypeConfigMap, configMapNode.ID, configMap.Namespace); err != nil {
		h.Logger.Error("failed to create namespace edge", zap.Error(err))
	}
}

// HandleUpdate processes an updated ConfigMap
func (h *ConfigMapHandler) HandleUpdate(oldObj, newObj interface{}) {
	newConfigMap, ok := newObj.(*corev1.ConfigMap)
	if !ok {
		h.Logger.Error("unexpected object type", zap.String("type", fmt.Sprintf("%T", newObj)))
		return
	}

	h.Logger.Debug("configmap updated", zap.String("namespace", newConfigMap.Namespace), zap.String("name", newConfigMap.Name))
	h.HandleAdd(newObj)
}

// HandleDelete processes a deleted ConfigMap
func (h *ConfigMapHandler) HandleDelete(obj interface{}) {
	configMap, ok := obj.(*corev1.ConfigMap)
	if !ok {
		extracted, err := watchers.SafeGetObject(obj)
		if err != nil {
			h.Logger.Error("failed to extract object", zap.Error(err))
			return
		}
		configMap, ok = extracted.(*corev1.ConfigMap)
		if !ok {
			h.Logger.Error("unexpected object type", zap.String("type", fmt.Sprintf("%T", extracted)))
			return
		}
	}

	h.Logger.Debug("configmap deleted", zap.String("namespace", configMap.Namespace), zap.String("name", configMap.Name))

	ctx := context.Background()

	configMapID := models.GetNodeID("ConfigMap", configMap.Namespace, configMap.Name)
	if err := h.GraphStore.DeleteNode(ctx, string(models.NodeTypeConfigMap), configMapID); err != nil {
		h.Logger.Error("failed to delete configmap node", zap.Error(err), zap.String("configmap", configMap.Name))
	}
}
