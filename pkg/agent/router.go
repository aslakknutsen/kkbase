package agent

import (
	"time"

	"github.com/aslakknutsen/kkbase/pkg/agenttypes"
	"github.com/aslakknutsen/kkbase/pkg/config"
	"go.uber.org/zap"
)

// EventRouter filters and prioritizes events
type EventRouter struct {
	deduplicator *Deduplicator
	filters      []FilterFunc
	logger       *zap.Logger
}

// NewEventRouter creates a new event router
func NewEventRouter(cfg *config.Config, logger *zap.Logger) *EventRouter {
	// Create deduplicator with 5 minute TTL
	dedup := NewDeduplicator(5 * time.Minute)

	// Build filters
	filters := []FilterFunc{
		// Apply allowlist filter if configured
		AllowlistFilter(cfg.EventFilterAllowlist),
		// Apply denylist filter if configured
		DenylistFilter(cfg.EventFilterDenylist),
		// Only process warnings and criticals by default
		SeverityFilter(agenttypes.SeverityWarning),
	}

	return &EventRouter{
		deduplicator: dedup,
		filters:      filters,
		logger:       logger,
	}
}

// Process filters and processes an event
// Returns (accepted, processedEvent, error)
func (r *EventRouter) Process(event agenttypes.Event) (bool, *agenttypes.ProcessedEvent, error) {
	// Step 1: Deduplicate
	if r.deduplicator.IsDuplicate(event) {
		r.logger.Debug("duplicate event filtered",
			zap.String("event_id", event.ID),
			zap.String("reason", event.Reason))
		return false, nil, nil
	}

	// Step 2: Apply filters
	for _, filter := range r.filters {
		if !filter(event) {
			r.logger.Debug("event filtered out",
				zap.String("event_id", event.ID),
				zap.String("reason", event.Reason))
			return false, nil, nil
		}
	}

	// Step 3: Calculate priority
	priority := r.calculatePriority(event)

	processed := &agenttypes.ProcessedEvent{
		Event:    event,
		Priority: priority,
	}

	r.logger.Info("event accepted for processing",
		zap.String("event_id", event.ID),
		zap.String("reason", event.Reason),
		zap.String("severity", string(event.Severity)),
		zap.Int("priority", priority))

	return true, processed, nil
}

// calculatePriority assigns a priority score to an event
func (r *EventRouter) calculatePriority(event agenttypes.Event) int {
	score := 0

	// Base score on severity
	switch event.Severity {
	case agenttypes.SeverityCritical:
		score += 100
	case agenttypes.SeverityWarning:
		score += 50
	case agenttypes.SeverityInfo:
		score += 10
	}

	// Boost priority for specific critical reasons
	criticalReasons := map[string]int{
		"OOMKilled":        50,
		"CrashLoopBackOff": 40,
		"Failed":           30,
		"ImagePullBackOff": 25,
		"NodeNotReady":     45,
	}

	if boost, ok := criticalReasons[event.Reason]; ok {
		score += boost
	}

	// Boost for production namespaces
	productionNamespaces := map[string]bool{
		"production": true,
		"prod":       true,
		"default":    true,
	}

	if productionNamespaces[event.Resource.Namespace] {
		score += 20
	}

	return score
}
