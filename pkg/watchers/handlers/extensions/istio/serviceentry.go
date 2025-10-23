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

// ServiceEntryHandler handles Istio ServiceEntry resources
type ServiceEntryHandler struct {
	*watchers.BaseWatcher
	clientset           *kubernetes.Clientset
	dynamicClient       dynamic.Interface
	relationshipBuilder *watchers.RelationshipBuilder
}

// NewServiceEntryHandler creates a new ServiceEntry handler
func NewServiceEntryHandler(
	clientset *kubernetes.Clientset,
	dynamicClient dynamic.Interface,
	graphStore graph.GraphStore,
	logger *zap.Logger,
	dynamicInformerFactory dynamicinformer.DynamicSharedInformerFactory,
) *ServiceEntryHandler {
	gvr := istiov1.SchemeGroupVersion.WithResource("serviceentries")
	informer := dynamicInformerFactory.ForResource(gvr).Informer()

	handler := &ServiceEntryHandler{
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

// HandleAdd processes a newly added ServiceEntry
func (h *ServiceEntryHandler) HandleAdd(obj interface{}) {
	unstructuredObj, ok := obj.(*unstructured.Unstructured)
	if !ok {
		h.Logger.Error("unexpected object type", zap.String("type", fmt.Sprintf("%T", obj)))
		return
	}

	se := &istiov1.ServiceEntry{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.Object, se); err != nil {
		h.Logger.Error("failed to convert to ServiceEntry", zap.Error(err))
		return
	}

	h.Logger.Debug("serviceentry added",
		zap.String("namespace", se.Namespace),
		zap.String("name", se.Name),
	)

	ctx := context.Background()

	// Create ServiceEntry node
	seNode := models.ServiceEntryToGraphNode(se)
	if err := h.GraphStore.UpsertNode(ctx, string(seNode.Type), seNode.ID, seNode.Properties); err != nil {
		h.Logger.Error("failed to create serviceentry node", zap.Error(err), zap.String("serviceentry", se.Name))
		return
	}

	// Create IN_NAMESPACE edge (if namespaced)
	if se.Namespace != "" {
		if err := h.relationshipBuilder.CreateNamespaceEdge(ctx, models.NodeTypeServiceEntry, seNode.ID, se.Namespace); err != nil {
			h.Logger.Error("failed to create namespace edge", zap.Error(err))
		}
	}
}

// HandleUpdate processes an updated ServiceEntry
func (h *ServiceEntryHandler) HandleUpdate(oldObj, newObj interface{}) {
	unstructuredObj, ok := newObj.(*unstructured.Unstructured)
	if !ok {
		h.Logger.Error("unexpected object type", zap.String("type", fmt.Sprintf("%T", newObj)))
		return
	}

	se := &istiov1.ServiceEntry{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.Object, se); err != nil {
		h.Logger.Error("failed to convert to ServiceEntry", zap.Error(err))
		return
	}

	h.Logger.Debug("serviceentry updated",
		zap.String("namespace", se.Namespace),
		zap.String("name", se.Name),
	)

	ctx := context.Background()
	seID := models.GetNodeID("ServiceEntry", se.Namespace, se.Name)

	// Delete old edges
	if err := h.GraphStore.DeleteEdgesByNode(ctx, string(models.NodeTypeServiceEntry), seID); err != nil {
		h.Logger.Error("failed to delete old serviceentry edges", zap.Error(err))
	}

	// Recreate with new data
	h.HandleAdd(newObj)
}

// HandleDelete processes a deleted ServiceEntry
func (h *ServiceEntryHandler) HandleDelete(obj interface{}) {
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

	se := &istiov1.ServiceEntry{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.Object, se); err != nil {
		h.Logger.Error("failed to convert to ServiceEntry", zap.Error(err))
		return
	}

	h.Logger.Debug("serviceentry deleted",
		zap.String("namespace", se.Namespace),
		zap.String("name", se.Name),
	)

	ctx := context.Background()

	seID := models.GetNodeID("ServiceEntry", se.Namespace, se.Name)
	if err := h.GraphStore.DeleteNode(ctx, string(models.NodeTypeServiceEntry), seID); err != nil {
		h.Logger.Error("failed to delete serviceentry node", zap.Error(err), zap.String("serviceentry", se.Name))
	}
}
