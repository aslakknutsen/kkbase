package core

import (
	"context"
	"fmt"

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

// PodHandler handles Pod resources
type PodHandler struct {
	*watchers.BaseWatcher
	clientset           *kubernetes.Clientset
	relationshipBuilder *RelationshipBuilder
}

// NewPodHandler creates a new Pod handler
func NewPodHandler(
	clientset *kubernetes.Clientset,
	graphStore graph.GraphStore,
	logger *zap.Logger,
	factory dynamicinformer.DynamicSharedInformerFactory,
) *PodHandler {
	gvr := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "pods",
	}
	informer := factory.ForResource(gvr).Informer()

	handler := &PodHandler{
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

// HandleAdd processes a newly added Pod
func (h *PodHandler) HandleAdd(obj interface{}) {
	pod, err := watchers.ConvertToTyped[corev1.Pod](obj)
	if err != nil {
		h.Logger.Error("failed to convert to Pod", zap.Error(err))
		return
	}

	h.Logger.Debug("pod added", zap.String("namespace", pod.Namespace), zap.String("name", pod.Name))

	ctx := context.Background()

	// Create Pod node
	podNode := PodToGraphNode(pod)
	if err := h.GraphStore.UpsertNode(ctx, string(podNode.Type), podNode.ID, podNode.Properties); err != nil {
		h.Logger.Error("failed to create pod node", zap.Error(err), zap.String("pod", pod.Name))
		return
	}

	// Create Container nodes
	for i, container := range pod.Spec.Containers {
		var containerStatus *corev1.ContainerStatus
		if i < len(pod.Status.ContainerStatuses) {
			containerStatus = &pod.Status.ContainerStatuses[i]
		}

		containerNode := ContainerToGraphNode(pod, container, containerStatus)
		if err := h.GraphStore.UpsertNode(ctx, string(containerNode.Type), containerNode.ID, containerNode.Properties); err != nil {
			h.Logger.Error("failed to create container node", zap.Error(err), zap.String("container", container.Name))
			continue
		}
	}

	// Create relationships
	if err := h.relationshipBuilder.CreatePodSchedulingEdges(ctx, pod); err != nil {
		h.Logger.Error("failed to create pod scheduling edges", zap.Error(err))
	}

	if err := h.relationshipBuilder.CreatePodContainerEdges(ctx, pod); err != nil {
		h.Logger.Error("failed to create pod container edges", zap.Error(err))
	}

	if err := h.relationshipBuilder.CreatePodVolumeEdges(ctx, pod); err != nil {
		h.Logger.Error("failed to create pod volume edges", zap.Error(err))
	}

	// Create edges to matching Services
	if err := h.relationshipBuilder.CreatePodToServiceEdges(ctx, pod); err != nil {
		h.Logger.Error("failed to create pod-service edges", zap.Error(err))
	}

	// Create owner reference edges
	if ownerRef := models.GetOwnerReference(pod.OwnerReferences); ownerRef != nil {
		if err := h.relationshipBuilder.CreateOwnerEdge(ctx, NodeTypePod, podNode.ID, *ownerRef, pod.Namespace); err != nil {
			h.Logger.Error("failed to create owner edge", zap.Error(err))
		}
	}
}

// HandleUpdate processes an updated Pod
func (h *PodHandler) HandleUpdate(oldObj, newObj interface{}) {
	newPod, err := watchers.ConvertToTyped[corev1.Pod](newObj)
	if err != nil {
		h.Logger.Error("failed to convert to Pod", zap.Error(err))
		return
	}

	h.Logger.Debug("pod updated", zap.String("namespace", newPod.Namespace), zap.String("name", newPod.Name))

	// For updates, we can reuse the add logic as it uses MERGE (upsert)
	h.HandleAdd(newObj)
}

// HandleDelete processes a deleted Pod
func (h *PodHandler) HandleDelete(obj interface{}) {
	pod, err := watchers.ConvertToTyped[corev1.Pod](obj)
	if err != nil {
		h.Logger.Error("failed to convert to Pod", zap.Error(err))
		return
	}

	h.Logger.Debug("pod deleted", zap.String("namespace", pod.Namespace), zap.String("name", pod.Name))

	ctx := context.Background()

	// Delete Container nodes
	for _, container := range pod.Spec.Containers {
		containerID := fmt.Sprintf("Container/%s/%s/%s", pod.Namespace, pod.Name, container.Name)
		if err := h.GraphStore.DeleteNode(ctx, string(models.NodeTypeContainer), containerID); err != nil {
			h.Logger.Error("failed to delete container node", zap.Error(err), zap.String("container", container.Name))
		}
	}

	// Delete Pod node
	podID := models.GetNodeID(NodeTypePod, pod.Namespace, pod.Name)
	if err := h.GraphStore.DeleteNode(ctx, string(NodeTypePod), podID); err != nil {
		h.Logger.Error("failed to delete pod node", zap.Error(err), zap.String("pod", pod.Name))
	}
}
