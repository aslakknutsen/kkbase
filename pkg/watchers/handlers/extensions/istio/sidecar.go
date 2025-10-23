package istio

import (
	"context"
	"fmt"

	"github.com/kagenti/kkbase/pkg/graph"
	"github.com/kagenti/kkbase/pkg/models"
	"github.com/kagenti/kkbase/pkg/watchers"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	istiov1 "istio.io/client-go/pkg/apis/networking/v1"
)

// SidecarHandler handles Istio Sidecar resources
type SidecarHandler struct {
	*watchers.BaseWatcher
	clientset           *kubernetes.Clientset
	dynamicClient       dynamic.Interface
	relationshipBuilder *watchers.RelationshipBuilder
}

// NewSidecarHandler creates a new Sidecar handler
func NewSidecarHandler(
	clientset *kubernetes.Clientset,
	dynamicClient dynamic.Interface,
	graphStore graph.GraphStore,
	logger *zap.Logger,
	dynamicInformerFactory dynamicinformer.DynamicSharedInformerFactory,
) *SidecarHandler {
	gvr := istiov1.SchemeGroupVersion.WithResource("sidecars")
	informer := dynamicInformerFactory.ForResource(gvr).Informer()

	handler := &SidecarHandler{
		BaseWatcher:         watchers.NewBaseWatcher(graphStore, logger, informer),
		clientset:           clientset,
		dynamicClient:       dynamicClient,
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

// HandleAdd processes a newly added Sidecar
func (h *SidecarHandler) HandleAdd(obj interface{}) {
	unstructuredObj, ok := obj.(*unstructured.Unstructured)
	if !ok {
		h.Logger.Error("unexpected object type", zap.String("type", fmt.Sprintf("%T", obj)))
		return
	}

	sidecar := &istiov1.Sidecar{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.Object, sidecar); err != nil {
		h.Logger.Error("failed to convert to Sidecar", zap.Error(err))
		return
	}

	h.Logger.Debug("sidecar added",
		zap.String("namespace", sidecar.Namespace),
		zap.String("name", sidecar.Name),
	)

	ctx := context.Background()

	// Create Sidecar node
	sidecarNode := models.SidecarToGraphNode(sidecar)
	if err := h.GraphStore.UpsertNode(ctx, string(sidecarNode.Type), sidecarNode.ID, sidecarNode.Properties); err != nil {
		h.Logger.Error("failed to create sidecar node", zap.Error(err), zap.String("sidecar", sidecar.Name))
		return
	}

	// Create IN_NAMESPACE edge
	if err := h.relationshipBuilder.CreateNamespaceEdge(ctx, models.NodeTypeSidecar, sidecarNode.ID, sidecar.Namespace); err != nil {
		h.Logger.Error("failed to create namespace edge", zap.Error(err))
	}
}

// HandleUpdate processes an updated Sidecar
func (h *SidecarHandler) HandleUpdate(oldObj, newObj interface{}) {
	unstructuredObj, ok := newObj.(*unstructured.Unstructured)
	if !ok {
		h.Logger.Error("unexpected object type", zap.String("type", fmt.Sprintf("%T", newObj)))
		return
	}

	sidecar := &istiov1.Sidecar{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.Object, sidecar); err != nil {
		h.Logger.Error("failed to convert to Sidecar", zap.Error(err))
		return
	}

	h.Logger.Debug("sidecar updated",
		zap.String("namespace", sidecar.Namespace),
		zap.String("name", sidecar.Name),
	)

	ctx := context.Background()
	sidecarID := models.GetNodeID("Sidecar", sidecar.Namespace, sidecar.Name)

	// Delete old edges
	if err := h.GraphStore.DeleteEdgesByNode(ctx, string(models.NodeTypeSidecar), sidecarID); err != nil {
		h.Logger.Error("failed to delete old sidecar edges", zap.Error(err))
	}

	// Recreate with new data
	h.HandleAdd(newObj)
}

// HandleDelete processes a deleted Sidecar
func (h *SidecarHandler) HandleDelete(obj interface{}) {
	unstructuredObj, ok := obj.(*unstructured.Unstructured)
	if !ok {
		extracted, err := watchers.SafeGetObject(obj)
		if err != nil {
			h.Logger.Error("failed to extract object", zap.Error(err))
			return
		}
		unstructuredObj, ok = extracted.(*unstructured.Unstructured)
		if !ok {
			h.Logger.Error("unexpected object type", zap.String("type", fmt.Sprintf("%T", extracted)))
			return
		}
	}

	sidecar := &istiov1.Sidecar{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.Object, sidecar); err != nil {
		h.Logger.Error("failed to convert to Sidecar", zap.Error(err))
		return
	}

	h.Logger.Debug("sidecar deleted",
		zap.String("namespace", sidecar.Namespace),
		zap.String("name", sidecar.Name),
	)

	ctx := context.Background()

	sidecarID := models.GetNodeID("Sidecar", sidecar.Namespace, sidecar.Name)
	if err := h.GraphStore.DeleteNode(ctx, string(models.NodeTypeSidecar), sidecarID); err != nil {
		h.Logger.Error("failed to delete sidecar node", zap.Error(err), zap.String("sidecar", sidecar.Name))
	}
}
