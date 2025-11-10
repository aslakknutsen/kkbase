package sources

import (
	"context"
	"fmt"
	"time"

	"github.com/aslakknutsen/kkbase/pkg/agenttypes"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

// K8sEventsSource watches Kubernetes events from the API
type K8sEventsSource struct {
	name            string
	events          chan agenttypes.Event
	clientset       *kubernetes.Clientset
	config          *rest.Config
	informer        cache.SharedIndexInformer
	informerFactory dynamicinformer.DynamicSharedInformerFactory
	logger          *zap.Logger
	stopCh          chan struct{}
	namespace       string
	resyncPeriod    time.Duration
}

// NewK8sEventsSource creates a new Kubernetes events source that watches the K8s API
func NewK8sEventsSource(
	clientset *kubernetes.Clientset,
	config *rest.Config,
	namespace string,
	resyncPeriod time.Duration,
	logger *zap.Logger,
) *K8sEventsSource {
	source := &K8sEventsSource{
		name:         "k8s-events",
		events:       make(chan agenttypes.Event, 100),
		clientset:    clientset,
		config:       config,
		logger:       logger,
		stopCh:       make(chan struct{}),
		namespace:    namespace,
		resyncPeriod: resyncPeriod,
	}

	return source
}

// Name returns the source name
func (s *K8sEventsSource) Name() string {
	return s.name
}

// Start starts watching for K8s events
func (s *K8sEventsSource) Start(ctx context.Context) error {
	s.logger.Info("starting k8s events source - initializing informer")

	// Create dynamic client
	dynamicClient, err := dynamic.NewForConfig(s.config)
	if err != nil {
		return fmt.Errorf("failed to create dynamic client: %w", err)
	}

	// Create dynamic informer factory
	s.informerFactory = dynamicinformer.NewFilteredDynamicSharedInformerFactory(
		dynamicClient,
		s.resyncPeriod,
		s.namespace,
		nil,
	)

	// Get informer for events
	gvr := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "events",
	}
	s.informer = s.informerFactory.ForResource(gvr).Informer()

	// Register event handlers
	_, err = s.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    s.handleAdd,
		UpdateFunc: s.handleUpdate,
		DeleteFunc: s.handleDelete,
	})
	if err != nil {
		return fmt.Errorf("failed to add event handler: %w", err)
	}

	// Start the informer factory
	s.informerFactory.Start(s.stopCh)

	s.logger.Info("k8s events source started - watching Kubernetes API")
	return nil
}

// Events returns the events channel
func (s *K8sEventsSource) Events() <-chan agenttypes.Event {
	return s.events
}

// Stop stops the source and shuts down the informer
func (s *K8sEventsSource) Stop() error {
	s.logger.Info("stopping k8s events source")
	close(s.stopCh)

	// Wait for informer to stop (it listens on stopCh)
	if s.informerFactory != nil {
		s.informerFactory.Shutdown()
	}

	close(s.events)
	return nil
}

// handleAdd processes a newly added event
func (s *K8sEventsSource) handleAdd(obj interface{}) {
	event, err := convertToK8sEvent(obj)
	if err != nil {
		s.logger.Error("failed to convert event", zap.Error(err))
		return
	}

	// Only process Warning and Error events
	if event.Type != "Warning" && event.Type != "Error" {
		return
	}

	s.logger.Debug("k8s event added",
		zap.String("namespace", event.Namespace),
		zap.String("name", event.Name),
		zap.String("reason", event.Reason),
		zap.String("type", event.Type))

	agentEvent := s.convertToAgentEvent(event)
	select {
	case s.events <- agentEvent:
		s.logger.Debug("forwarded k8s event", zap.String("id", agentEvent.ID))
	case <-s.stopCh:
		return
	default:
		s.logger.Warn("events channel full, dropping event",
			zap.String("event", event.Name))
	}
}

// handleUpdate processes an updated event
func (s *K8sEventsSource) handleUpdate(oldObj, newObj interface{}) {
	oldEvent, err := convertToK8sEvent(oldObj)
	if err != nil {
		s.logger.Error("failed to convert old event", zap.Error(err))
		return
	}

	newEvent, err := convertToK8sEvent(newObj)
	if err != nil {
		s.logger.Error("failed to convert new event", zap.Error(err))
		return
	}

	// Only process if count increased (event occurred again)
	if newEvent.Count > oldEvent.Count {
		// Only process Warning and Error events
		if newEvent.Type != "Warning" && newEvent.Type != "Error" {
			return
		}

		s.logger.Debug("k8s event updated",
			zap.String("namespace", newEvent.Namespace),
			zap.String("name", newEvent.Name),
			zap.String("reason", newEvent.Reason),
			zap.Int32("old_count", oldEvent.Count),
			zap.Int32("new_count", newEvent.Count))

		agentEvent := s.convertToAgentEvent(newEvent)
		select {
		case s.events <- agentEvent:
			s.logger.Debug("forwarded updated k8s event", zap.String("id", agentEvent.ID))
		case <-s.stopCh:
			return
		default:
			s.logger.Warn("events channel full, dropping event",
				zap.String("event", newEvent.Name))
		}
	}
}

// handleDelete processes a deleted event (usually not interesting for agent)
func (s *K8sEventsSource) handleDelete(obj interface{}) {
	event, err := convertToK8sEvent(obj)
	if err != nil {
		s.logger.Error("failed to convert deleted event", zap.Error(err))
		return
	}

	s.logger.Debug("k8s event deleted",
		zap.String("namespace", event.Namespace),
		zap.String("name", event.Name))
}

// convertToAgentEvent converts a K8s Event to an agent Event
func (s *K8sEventsSource) convertToAgentEvent(event *corev1.Event) agenttypes.Event {
	// Determine severity based on type and reason
	severity := s.determineSeverity(event.Type, event.Reason)

	// Use last timestamp or first timestamp
	timestamp := event.LastTimestamp.Time
	if timestamp.IsZero() {
		timestamp = event.FirstTimestamp.Time
	}
	if timestamp.IsZero() {
		timestamp = time.Now()
	}

	// Build resource ID
	namespace := event.InvolvedObject.Namespace
	if namespace == "" {
		namespace = event.Namespace
	}

	resourceType := event.InvolvedObject.Kind
	resourceName := event.InvolvedObject.Name
	resourceID := fmt.Sprintf("%s/%s/%s", resourceType, namespace, resourceName)

	// Generate event ID
	eventID := fmt.Sprintf("k8s-event-%s-%s-%d",
		namespace,
		event.Name,
		timestamp.Unix())

	return agenttypes.Event{
		ID:        eventID,
		Type:      agenttypes.EventTypeK8sEvent,
		Source:    "kubernetes",
		Severity:  severity,
		Timestamp: timestamp,
		Resource: agenttypes.ResourceRef{
			Type:      resourceType,
			Namespace: namespace,
			Name:      resourceName,
			ID:        resourceID,
		},
		Reason:  event.Reason,
		Message: event.Message,
		Data: map[string]interface{}{
			"event_type":  event.Type,
			"count":       event.Count,
			"first_time":  event.FirstTimestamp.Unix(),
			"last_time":   event.LastTimestamp.Unix(),
			"object_uid":  string(event.InvolvedObject.UID),
			"object_kind": event.InvolvedObject.Kind,
		},
		Labels: map[string]string{
			"source":     "k8s-events",
			"event_type": event.Type,
		},
	}
}

// determineSeverity maps K8s event types and reasons to severity levels
func (s *K8sEventsSource) determineSeverity(eventType, reason string) agenttypes.Severity {
	// Critical events
	criticalReasons := map[string]bool{
		"OOMKilled":              true,
		"CrashLoopBackOff":       true,
		"ImagePullBackOff":       true,
		"ErrImagePull":           true,
		"Failed":                 true,
		"FailedScheduling":       true,
		"FailedMount":            true,
		"FailedAttachVolume":     true,
		"NodeNotReady":           true,
		"Evicted":                true,
		"FailedKillPod":          true,
		"FailedCreatePodSandBox": true,
	}

	if criticalReasons[reason] || eventType == "Error" {
		return agenttypes.SeverityCritical
	}

	// Warning events
	warningReasons := map[string]bool{
		"BackOff":           true,
		"Unhealthy":         true,
		"FailedHealthCheck": true,
		"ProbeWarning":      true,
		"Pulling":           false, // Don't treat pulling as warning
		"Created":           false,
		"Started":           false,
	}

	if val, exists := warningReasons[reason]; exists && val {
		return agenttypes.SeverityWarning
	}

	if eventType == "Warning" {
		return agenttypes.SeverityWarning
	}

	// Default to info
	return agenttypes.SeverityInfo
}

// convertToK8sEvent converts an interface{} to a *corev1.Event
func convertToK8sEvent(obj interface{}) (*corev1.Event, error) {
	event, ok := obj.(*corev1.Event)
	if !ok {
		return nil, fmt.Errorf("expected *corev1.Event, got %T", obj)
	}
	return event, nil
}
