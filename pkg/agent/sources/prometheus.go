package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/kagenti/kkbase/pkg/agenttypes"
	"go.uber.org/zap"
)

// AlertmanagerWebhook receives Prometheus Alertmanager webhooks
type AlertmanagerWebhook struct {
	name   string
	events chan agenttypes.Event
	logger *zap.Logger
}

// NewAlertmanagerWebhook creates a new Alertmanager webhook source
// The mux parameter is used to register the webhook handler
func NewAlertmanagerWebhook(mux *http.ServeMux, logger *zap.Logger) *AlertmanagerWebhook {
	source := &AlertmanagerWebhook{
		name:   "prometheus-alertmanager",
		events: make(chan agenttypes.Event, 100),
		logger: logger,
	}

	mux.HandleFunc("/webhook/alertmanager", source.handleWebhook)

	return source
}

// Name returns the source name
func (a *AlertmanagerWebhook) Name() string {
	return a.name
}

// Start starts the webhook server
func (a *AlertmanagerWebhook) Start(ctx context.Context) error {
	a.logger.Info("alertmanager webhook ready")
	return nil
}

// Events returns the events channel
func (a *AlertmanagerWebhook) Events() <-chan agenttypes.Event {
	return a.events
}

// Stop stops the webhook server
func (a *AlertmanagerWebhook) Stop() error {
	close(a.events)
	return nil
}

// AlertmanagerPayload represents the Alertmanager webhook payload
type AlertmanagerPayload struct {
	Version  string  `json:"version"`
	GroupKey string  `json:"groupKey"`
	Status   string  `json:"status"`
	Alerts   []Alert `json:"alerts"`
}

// Alert represents a single alert from Alertmanager
type Alert struct {
	Status      string            `json:"status"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	StartsAt    time.Time         `json:"startsAt"`
	EndsAt      time.Time         `json:"endsAt"`
}

// handleWebhook processes incoming webhooks from Alertmanager
func (a *AlertmanagerWebhook) handleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload AlertmanagerPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		a.logger.Warn("failed to decode webhook payload", zap.Error(err))
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	a.logger.Info("received alertmanager webhook",
		zap.String("status", payload.Status),
		zap.Int("alerts", len(payload.Alerts)))

	// Process alerts
	for _, alert := range payload.Alerts {
		// Only process firing alerts
		if alert.Status != "firing" {
			continue
		}

		event := a.convertToAgentEvent(alert)
		select {
		case a.events <- event:
			a.logger.Debug("forwarded alert as event", zap.String("alert", alert.Labels["alertname"]))
		default:
			a.logger.Warn("events channel full, dropping alert")
		}
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

// convertToAgentEvent converts an Alertmanager alert to agent Event
func (a *AlertmanagerWebhook) convertToAgentEvent(alert Alert) agenttypes.Event {
	alertname := alert.Labels["alertname"]
	severity := a.mapSeverity(alert.Labels["severity"])

	// Extract resource information from labels
	namespace := alert.Labels["namespace"]
	podName := alert.Labels["pod"]
	serviceName := alert.Labels["service"]
	nodeName := alert.Labels["node"]

	// Determine resource type and name
	resourceType := "Unknown"
	resourceName := ""
	if podName != "" {
		resourceType = "Pod"
		resourceName = podName
	} else if serviceName != "" {
		resourceType = "Service"
		resourceName = serviceName
	} else if nodeName != "" {
		resourceType = "Node"
		resourceName = nodeName
		namespace = "" // Nodes are cluster-scoped
	}

	resourceID := fmt.Sprintf("%s/%s/%s", resourceType, namespace, resourceName)
	if namespace == "" {
		resourceID = fmt.Sprintf("%s/%s", resourceType, resourceName)
	}

	// Generate event ID
	eventID := fmt.Sprintf("prom-%s-%d", alertname, alert.StartsAt.Unix())

	return agenttypes.Event{
		ID:        eventID,
		Type:      agenttypes.EventTypePrometheusAlert,
		Source:    "prometheus-alertmanager",
		Severity:  severity,
		Timestamp: alert.StartsAt,
		Resource: agenttypes.ResourceRef{
			Type:      resourceType,
			Namespace: namespace,
			Name:      resourceName,
			ID:        resourceID,
		},
		Reason:  alertname,
		Message: alert.Annotations["description"],
		Data: map[string]interface{}{
			"summary":     alert.Annotations["summary"],
			"runbook_url": alert.Annotations["runbook_url"],
		},
		Labels: alert.Labels,
	}
}

// mapSeverity maps Prometheus severity to agent severity
func (a *AlertmanagerWebhook) mapSeverity(severity string) agenttypes.Severity {
	switch severity {
	case "critical", "page":
		return agenttypes.SeverityCritical
	case "warning", "warn":
		return agenttypes.SeverityWarning
	default:
		return agenttypes.SeverityInfo
	}
}
