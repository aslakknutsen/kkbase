package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/aslakknutsen/kkbase/pkg/agenttypes"
	"go.uber.org/zap"
)

// CustomWebhook receives generic webhook events
type CustomWebhook struct {
	name   string
	events chan agenttypes.Event
	logger *zap.Logger
}

// NewCustomWebhook creates a new custom webhook source
// The mux parameter is used to register the webhook handler
func NewCustomWebhook(mux *http.ServeMux, logger *zap.Logger) *CustomWebhook {
	source := &CustomWebhook{
		name:   "custom-webhook",
		events: make(chan agenttypes.Event, 100),
		logger: logger,
	}

	mux.HandleFunc("/webhook/custom", source.handleWebhook)

	return source
}

// Name returns the source name
func (w *CustomWebhook) Name() string {
	return w.name
}

// Start starts the webhook server
func (w *CustomWebhook) Start(ctx context.Context) error {
	w.logger.Info("custom webhook ready")
	return nil
}

// Events returns the events channel
func (w *CustomWebhook) Events() <-chan agenttypes.Event {
	return w.events
}

// Stop stops the webhook server
func (w *CustomWebhook) Stop() error {
	close(w.events)
	return nil
}

// CustomEventPayload represents a generic event payload
type CustomEventPayload struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Severity  string                 `json:"severity"`
	Resource  CustomResourceRef      `json:"resource"`
	Reason    string                 `json:"reason"`
	Message   string                 `json:"message"`
	Timestamp string                 `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
	Labels    map[string]string      `json:"labels"`
}

// CustomResourceRef represents a resource reference
type CustomResourceRef struct {
	Type      string `json:"type"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

// handleWebhook processes incoming custom webhooks
func (w *CustomWebhook) handleWebhook(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload CustomEventPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		w.logger.Warn("failed to decode webhook payload", zap.Error(err))
		http.Error(rw, "invalid payload", http.StatusBadRequest)
		return
	}

	w.logger.Info("received custom webhook",
		zap.String("id", payload.ID),
		zap.String("type", payload.Type),
		zap.String("resource", payload.Resource.Name))

	// Validate required fields
	if payload.Resource.Type == "" || payload.Resource.Name == "" {
		w.logger.Warn("invalid payload: missing required fields")
		http.Error(rw, "missing required fields: resource.type, resource.name", http.StatusBadRequest)
		return
	}

	event := w.convertToAgentEvent(payload)
	select {
	case w.events <- event:
		w.logger.Debug("forwarded custom event", zap.String("id", event.ID))
	default:
		w.logger.Warn("events channel full, dropping event")
	}

	rw.WriteHeader(http.StatusOK)
	json.NewEncoder(rw).Encode(map[string]string{
		"status": "accepted",
		"id":     event.ID,
	})
}

// convertToAgentEvent converts custom payload to agent Event
func (w *CustomWebhook) convertToAgentEvent(payload CustomEventPayload) agenttypes.Event {
	// Parse timestamp
	timestamp := time.Now()
	if payload.Timestamp != "" {
		if ts, err := time.Parse(time.RFC3339, payload.Timestamp); err == nil {
			timestamp = ts
		}
	}

	// Map severity
	severity := agenttypes.SeverityInfo
	switch payload.Severity {
	case "critical":
		severity = agenttypes.SeverityCritical
	case "warning":
		severity = agenttypes.SeverityWarning
	}

	// Build resource ID
	resourceID := fmt.Sprintf("%s/%s/%s",
		payload.Resource.Type,
		payload.Resource.Namespace,
		payload.Resource.Name)

	// Generate ID if not provided
	eventID := payload.ID
	if eventID == "" {
		eventID = fmt.Sprintf("custom-%s-%d",
			payload.Resource.Name,
			timestamp.Unix())
	}

	// Initialize data if nil
	data := payload.Data
	if data == nil {
		data = make(map[string]interface{})
	}

	// Initialize labels if nil
	labels := payload.Labels
	if labels == nil {
		labels = make(map[string]string)
	}
	labels["source"] = "custom-webhook"

	return agenttypes.Event{
		ID:        eventID,
		Type:      agenttypes.EventTypeCustom,
		Source:    "custom",
		Severity:  severity,
		Timestamp: timestamp,
		Resource: agenttypes.ResourceRef{
			Type:      payload.Resource.Type,
			Namespace: payload.Resource.Namespace,
			Name:      payload.Resource.Name,
			ID:        resourceID,
		},
		Reason:  payload.Reason,
		Message: payload.Message,
		Data:    data,
		Labels:  labels,
	}
}
