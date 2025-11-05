package sources

import (
	"context"
	"fmt"
	"time"

	"github.com/kagenti/kkbase/pkg/agenttypes"
	"github.com/kagenti/kkbase/pkg/graph"
	"go.uber.org/zap"
)

// K8sEventsSource polls Neo4j for new Kubernetes events
type K8sEventsSource struct {
	name         string
	events       chan agenttypes.Event
	graphStore   graph.GraphStore
	logger       *zap.Logger
	stopCh       chan struct{}
	lastSeen     time.Time
	pollInterval time.Duration
}

// NewK8sEventsSource creates a new Kubernetes events source
func NewK8sEventsSource(graphStore graph.GraphStore, logger *zap.Logger) *K8sEventsSource {
	return &K8sEventsSource{
		name:         "k8s-events",
		events:       make(chan agenttypes.Event, 100),
		graphStore:   graphStore,
		logger:       logger,
		stopCh:       make(chan struct{}),
		lastSeen:     time.Now().Add(-5 * time.Minute), // Look back 5 minutes initially
		pollInterval: 10 * time.Second,
	}
}

// Name returns the source name
func (s *K8sEventsSource) Name() string {
	return s.name
}

// Start starts polling for K8s events
func (s *K8sEventsSource) Start(ctx context.Context) error {
	s.logger.Info("starting K8s events source",
		zap.Duration("poll_interval", s.pollInterval))

	go s.pollLoop(ctx)
	return nil
}

// Events returns the events channel
func (s *K8sEventsSource) Events() <-chan agenttypes.Event {
	return s.events
}

// Stop stops the source
func (s *K8sEventsSource) Stop() error {
	close(s.stopCh)
	close(s.events)
	return nil
}

// pollLoop continuously polls for new events
func (s *K8sEventsSource) pollLoop(ctx context.Context) {
	ticker := time.NewTicker(s.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("stopping K8s events source")
			return
		case <-s.stopCh:
			s.logger.Info("stopping K8s events source")
			return
		case <-ticker.C:
			s.pollEvents(ctx)
		}
	}
}

// pollEvents queries Neo4j for new events
func (s *K8sEventsSource) pollEvents(ctx context.Context) {
	// Query for events created after lastSeen
	query := `
		MATCH (e:Event)
		WHERE datetime(e.last_timestamp) > datetime($last_seen)
		OPTIONAL MATCH (e)-[:INVOLVES]->(r)
		RETURN e.uid as id,
		       e.type as type,
		       e.reason as reason,
		       e.message as message,
		       e.last_timestamp as timestamp,
		       e.namespace as namespace,
		       e.involved_object_kind as resource_type,
		       e.involved_object_name as resource_name,
		       labels(r)[0] as actual_resource_type
		ORDER BY e.last_timestamp
		LIMIT 100
	`

	params := map[string]interface{}{
		"last_seen": s.lastSeen.Format(time.RFC3339),
	}

	results, err := s.graphStore.Query(ctx, query, params)
	if err != nil {
		s.logger.Error("failed to query events", zap.Error(err))
		return
	}

	if len(results) == 0 {
		return
	}

	s.logger.Debug("found new K8s events", zap.Int("count", len(results)))

	for _, result := range results {
		event := s.convertToAgentEvent(result)
		if event != nil {
			select {
			case s.events <- *event:
				// Update lastSeen
				if event.Timestamp.After(s.lastSeen) {
					s.lastSeen = event.Timestamp
				}
			case <-ctx.Done():
				return
			case <-s.stopCh:
				return
			default:
				s.logger.Warn("events channel full, dropping event")
			}
		}
	}
}

// convertToAgentEvent converts Neo4j result to agent Event
func (s *K8sEventsSource) convertToAgentEvent(result map[string]interface{}) *agenttypes.Event {
	id, _ := result["id"].(string)
	eventType, _ := result["type"].(string)
	reason, _ := result["reason"].(string)
	message, _ := result["message"].(string)
	timestampStr, _ := result["timestamp"].(string)
	namespace, _ := result["namespace"].(string)
	resourceType, _ := result["resource_type"].(string)
	resourceName, _ := result["resource_name"].(string)

	// Parse timestamp
	timestamp, err := time.Parse(time.RFC3339, timestampStr)
	if err != nil {
		s.logger.Warn("failed to parse timestamp", zap.Error(err))
		timestamp = time.Now()
	}

	// Determine severity based on type and reason
	severity := s.determineSeverity(eventType, reason)

	// Build resource ID
	resourceID := fmt.Sprintf("%s/%s/%s", resourceType, namespace, resourceName)

	return &agenttypes.Event{
		ID:        id,
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
		Reason:  reason,
		Message: message,
		Data: map[string]interface{}{
			"event_type": eventType,
		},
		Labels: map[string]string{
			"source": "k8s-events",
		},
	}
}

// determineSeverity maps K8s event types and reasons to severity levels
func (s *K8sEventsSource) determineSeverity(eventType, reason string) agenttypes.Severity {
	// Critical events
	criticalReasons := map[string]bool{
		"OOMKilled":          true,
		"CrashLoopBackOff":   true,
		"ImagePullBackOff":   true,
		"Failed":             true,
		"FailedScheduling":   true,
		"FailedMount":        true,
		"FailedAttachVolume": true,
		"NodeNotReady":       true,
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
	}

	if warningReasons[reason] || eventType == "Warning" {
		return agenttypes.SeverityWarning
	}

	// Default to info
	return agenttypes.SeverityInfo
}
