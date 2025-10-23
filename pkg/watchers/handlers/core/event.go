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
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// EventHandler handles K8s Event resources
type EventHandler struct {
	*watchers.BaseWatcher
	relationshipBuilder *watchers.RelationshipBuilder
}

// NewEventHandler creates a new Event handler
func NewEventHandler(
	clientset *kubernetes.Clientset,
	graphStore graph.GraphStore,
	logger *zap.Logger,
	informerFactory informers.SharedInformerFactory,
) *EventHandler {
	informer := informerFactory.Core().V1().Events().Informer()

	handler := &EventHandler{
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

// HandleAdd processes a newly added Event
func (h *EventHandler) HandleAdd(obj interface{}) {
	event, ok := obj.(*corev1.Event)
	if !ok {
		h.Logger.Error("unexpected object type", zap.String("type", fmt.Sprintf("%T", obj)))
		return
	}

	h.Logger.Debug("event added",
		zap.String("namespace", event.Namespace),
		zap.String("name", event.Name),
		zap.String("reason", event.Reason))

	ctx := context.Background()

	eventNode := models.EventToGraphNode(event)
	if err := h.GraphStore.UpsertNode(ctx, string(eventNode.Type), eventNode.ID, eventNode.Properties); err != nil {
		h.Logger.Error("failed to create event node", zap.Error(err), zap.String("event", event.Name))
		return
	}

	// Create INVOLVES edge to the involved object
	if err := h.relationshipBuilder.CreateEventInvolvedObjectEdge(ctx, event); err != nil {
		h.Logger.Error("failed to create event-object edge", zap.Error(err))
	}
}

// HandleUpdate processes an updated Event
func (h *EventHandler) HandleUpdate(oldObj, newObj interface{}) {
	newEvent, ok := newObj.(*corev1.Event)
	if !ok {
		h.Logger.Error("unexpected object type", zap.String("type", fmt.Sprintf("%T", newObj)))
		return
	}

	h.Logger.Debug("event updated",
		zap.String("namespace", newEvent.Namespace),
		zap.String("name", newEvent.Name),
		zap.String("reason", newEvent.Reason))

	h.HandleAdd(newObj)
}

// HandleDelete processes a deleted Event
func (h *EventHandler) HandleDelete(obj interface{}) {
	event, ok := obj.(*corev1.Event)
	if !ok {
		extracted, err := watchers.SafeGetObject(obj)
		if err != nil {
			h.Logger.Error("failed to extract object", zap.Error(err))
			return
		}
		event, ok = extracted.(*corev1.Event)
		if !ok {
			h.Logger.Error("unexpected object type", zap.String("type", fmt.Sprintf("%T", extracted)))
			return
		}
	}

	h.Logger.Debug("event deleted",
		zap.String("namespace", event.Namespace),
		zap.String("name", event.Name))

	ctx := context.Background()

	eventID := fmt.Sprintf("%s/%s/%s", event.Namespace, event.InvolvedObject.Name, event.Name)
	if err := h.GraphStore.DeleteNode(ctx, string(models.NodeTypeK8sEvent), eventID); err != nil {
		h.Logger.Error("failed to delete event node", zap.Error(err), zap.String("event", event.Name))
	}
}
