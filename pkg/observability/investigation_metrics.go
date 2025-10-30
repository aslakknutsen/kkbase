package observability

import (
	"context"
	"fmt"
	"time"

	"github.com/kagenti/kkbase/pkg/graph"
	"go.uber.org/zap"
)

// InvestigationMetricsProcessor handles metric collection for RCA investigations
type InvestigationMetricsProcessor struct {
	graphStore      graph.GraphStore
	metricsProvider MetricsProvider
	catalog         *MetricCatalog
	correlator      *MetricCorrelator
	logger          *zap.Logger
}

// NewInvestigationMetricsProcessor creates a new processor
func NewInvestigationMetricsProcessor(
	graphStore graph.GraphStore,
	metricsProvider MetricsProvider,
	logger *zap.Logger,
) *InvestigationMetricsProcessor {
	return &InvestigationMetricsProcessor{
		graphStore:      graphStore,
		metricsProvider: metricsProvider,
		catalog:         NewMetricCatalog(),
		correlator:      NewMetricCorrelator(graphStore, logger),
		logger:          logger,
	}
}

// StartInvestigation creates an investigation session and pulls relevant metrics
func (imp *InvestigationMetricsProcessor) StartInvestigation(
	ctx context.Context,
	resourceType, resourceID, symptom string,
	lookbackDuration time.Duration,
) (*InvestigationSession, error) {

	session := &InvestigationSession{
		ID:               generateInvestigationID(),
		ResourceType:     resourceType,
		ResourceID:       resourceID,
		Symptom:          symptom,
		StartTime:        time.Now(),
		LookbackDuration: lookbackDuration,
		Status:           "active",
		CreatedAt:        time.Now(),
	}

	imp.logger.Info("starting investigation",
		zap.String("investigation_id", session.ID),
		zap.String("resource_type", resourceType),
		zap.String("resource_id", resourceID),
		zap.String("symptom", symptom),
		zap.Duration("lookback", lookbackDuration))

	// Create Investigation node in graph
	if err := imp.createInvestigationNode(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to create investigation node: %w", err)
	}

	// Pull relevant metrics based on symptom and resource type
	if imp.metricsProvider != nil {
		if err := imp.pullMetricsForInvestigation(ctx, session); err != nil {
			imp.logger.Warn("failed to pull metrics for investigation",
				zap.String("investigation_id", session.ID),
				zap.Error(err))
			// Don't fail - investigation can proceed without metrics
		}
	} else {
		imp.logger.Warn("no metrics provider configured, skipping metric collection",
			zap.String("investigation_id", session.ID))
	}

	return session, nil
}

// pullMetricsForInvestigation queries Prometheus and stores metrics temporarily
func (imp *InvestigationMetricsProcessor) pullMetricsForInvestigation(
	ctx context.Context,
	session *InvestigationSession,
) error {

	// Determine which metrics to pull based on symptom
	metricNames := imp.selectMetricsForSymptom(session.Symptom, session.ResourceType)

	imp.logger.Debug("selected metrics for symptom",
		zap.String("symptom", session.Symptom),
		zap.Strings("metrics", metricNames))

	endTime := session.StartTime
	startTime := endTime.Add(-session.LookbackDuration)

	// Build query spec
	spec := MetricQuerySpec{
		MetricNames:  metricNames,
		ResourceType: session.ResourceType,
		ResourceID:   session.ResourceID,
		StartTime:    startTime,
		EndTime:      endTime,
		StepDuration: 1 * time.Minute, // 1-minute aggregation
	}

	// Query metrics provider
	metrics, err := imp.metricsProvider.QueryMetrics(ctx, spec)
	if err != nil {
		return fmt.Errorf("failed to query metrics: %w", err)
	}

	imp.logger.Info("pulled metrics for investigation",
		zap.String("investigation_id", session.ID),
		zap.Int("metric_count", len(metrics)))

	// Store metrics in graph with investigation context
	storedCount := 0
	for _, metric := range metrics {
		metric.InvestigationID = session.ID
		if err := imp.storeMetric(ctx, metric, session); err != nil {
			imp.logger.Warn("failed to store metric",
				zap.String("metric_name", metric.Name),
				zap.Time("timestamp", metric.Timestamp),
				zap.Error(err))
		} else {
			storedCount++
		}
	}

	imp.logger.Info("stored metrics in graph",
		zap.String("investigation_id", session.ID),
		zap.Int("stored_count", storedCount))

	return nil
}

// selectMetricsForSymptom chooses relevant metrics based on symptom
func (imp *InvestigationMetricsProcessor) selectMetricsForSymptom(symptom, resourceType string) []string {
	// Symptom-driven metric selection
	switch symptom {
	case "OOMKilled", "CrashLoopBackOff":
		return []string{
			"container_memory_usage_bytes",
			"container_memory_working_set_bytes",
			"container_cpu_usage_seconds_total",
		}
	case "HighLatency":
		return []string{
			"http_request_duration_seconds",
			"container_cpu_usage_seconds_total",
			"node_cpu_seconds_total",
		}
	case "HighErrorRate":
		return []string{
			"http_requests_total",
			"container_network_receive_errors_total",
			"container_network_transmit_errors_total",
		}
	case "NodeNotReady":
		return []string{
			"node_cpu_seconds_total",
			"node_memory_MemAvailable_bytes",
			"node_load1",
		}
	default:
		// Default: pull core resource metrics based on resource type
		defs := imp.catalog.GetMetricsForResourceType(resourceType)
		names := make([]string, 0, len(defs))
		for _, def := range defs {
			if def.Category == "resource" {
				names = append(names, def.Name)
			}
		}
		if len(names) > 0 {
			return names
		}
		// Fallback to basic metrics
		return []string{
			"container_memory_usage_bytes",
			"container_cpu_usage_seconds_total",
		}
	}
}

// storeMetric creates a Metric node with investigation context
func (imp *InvestigationMetricsProcessor) storeMetric(
	ctx context.Context,
	metric MetricData,
	session *InvestigationSession,
) error {

	// Generate metric node ID (include investigation ID for uniqueness)
	metricID := fmt.Sprintf("Metric/%s/%s/%d",
		session.ID,
		metric.Name,
		metric.Timestamp.Unix(),
	)

	// Create Metric node
	props := map[string]interface{}{
		"name":             metric.Name,
		"type":             string(metric.Type),
		"value":            metric.Value,
		"timestamp":        metric.Timestamp.Format(time.RFC3339),
		"investigation_id": session.ID,
	}

	// Add unit if present
	if metric.Unit != "" {
		props["unit"] = metric.Unit
	}

	// Store labels as individual properties for easier querying
	for k, v := range metric.Labels {
		props[fmt.Sprintf("label_%s", k)] = v
	}

	if err := imp.graphStore.UpsertNode(ctx, "Metric", metricID, props); err != nil {
		return err
	}

	// Link to Investigation
	investigationNodeID := fmt.Sprintf("Investigation/%s", session.ID)
	if err := imp.graphStore.UpsertEdge(ctx,
		"Investigation", investigationNodeID,
		"HAS_METRIC_EVIDENCE",
		"Metric", metricID,
		map[string]interface{}{
			"collected_at": time.Now().Format(time.RFC3339),
		},
	); err != nil {
		imp.logger.Warn("failed to link metric to investigation", zap.Error(err))
	}

	// Correlate metric to K8s resource
	correlatedResourceType, correlatedResourceID := imp.correlator.FindResourceFromLabels(ctx, metric.Labels)
	if correlatedResourceID != "" {
		if err := imp.graphStore.UpsertEdge(ctx,
			"Metric", metricID,
			"EMITTED_BY",
			correlatedResourceType, correlatedResourceID,
			nil,
		); err != nil {
			imp.logger.Debug("failed to create EMITTED_BY edge",
				zap.String("metric_id", metricID),
				zap.String("resource_type", correlatedResourceType),
				zap.String("resource_id", correlatedResourceID),
				zap.Error(err))
		}
	}

	return nil
}

// CompleteInvestigation marks investigation as complete and purges metrics
func (imp *InvestigationMetricsProcessor) CompleteInvestigation(
	ctx context.Context,
	investigationID string,
) error {

	imp.logger.Info("completing investigation and purging metrics",
		zap.String("investigation_id", investigationID))

	// Update Investigation node status
	investigationNodeID := fmt.Sprintf("Investigation/%s", investigationID)
	if err := imp.graphStore.UpsertNode(ctx, "Investigation", investigationNodeID, map[string]interface{}{
		"status":       "completed",
		"completed_at": time.Now().Format(time.RFC3339),
	}); err != nil {
		return err
	}

	// Purge all metrics associated with this investigation
	query := `
		MATCH (i:Investigation {id: $investigation_id})-[:HAS_METRIC_EVIDENCE]->(m:Metric)
		DETACH DELETE m
		RETURN count(m) as deleted_count
	`

	result, err := imp.graphStore.Query(ctx, query, map[string]interface{}{
		"investigation_id": investigationNodeID,
	})

	if err != nil {
		return fmt.Errorf("failed to purge metrics: %w", err)
	}

	if len(result) > 0 {
		if deletedCount, ok := result[0]["deleted_count"].(int64); ok {
			imp.logger.Info("purged investigation metrics",
				zap.String("investigation_id", investigationID),
				zap.Int64("deleted_count", deletedCount))
		}
	}

	return nil
}

// AbandonInvestigation cleans up a failed/abandoned investigation
func (imp *InvestigationMetricsProcessor) AbandonInvestigation(
	ctx context.Context,
	investigationID string,
) error {
	imp.logger.Info("abandoning investigation",
		zap.String("investigation_id", investigationID))

	// Update status to abandoned
	investigationNodeID := fmt.Sprintf("Investigation/%s", investigationID)
	if err := imp.graphStore.UpsertNode(ctx, "Investigation", investigationNodeID, map[string]interface{}{
		"status":       "abandoned",
		"abandoned_at": time.Now().Format(time.RFC3339),
	}); err != nil {
		return err
	}

	// Purge metrics (same as CompleteInvestigation)
	query := `
		MATCH (i:Investigation {id: $investigation_id})-[:HAS_METRIC_EVIDENCE]->(m:Metric)
		DETACH DELETE m
		RETURN count(m) as deleted_count
	`

	result, err := imp.graphStore.Query(ctx, query, map[string]interface{}{
		"investigation_id": investigationNodeID,
	})

	if err != nil {
		return fmt.Errorf("failed to purge metrics: %w", err)
	}

	if len(result) > 0 {
		if deletedCount, ok := result[0]["deleted_count"].(int64); ok {
			imp.logger.Info("purged abandoned investigation metrics",
				zap.String("investigation_id", investigationID),
				zap.Int64("deleted_count", deletedCount))
		}
	}

	return nil
}

func (imp *InvestigationMetricsProcessor) createInvestigationNode(
	ctx context.Context,
	session *InvestigationSession,
) error {
	nodeID := fmt.Sprintf("Investigation/%s", session.ID)
	props := map[string]interface{}{
		"id":                nodeID,
		"investigation_id":  session.ID,
		"resource_type":     session.ResourceType,
		"resource_id":       session.ResourceID,
		"symptom":           session.Symptom,
		"start_time":        session.StartTime.Format(time.RFC3339),
		"lookback_duration": session.LookbackDuration.String(),
		"status":            session.Status,
		"created_at":        session.CreatedAt.Format(time.RFC3339),
	}

	if err := imp.graphStore.UpsertNode(ctx, "Investigation", nodeID, props); err != nil {
		return err
	}

	// Link Investigation to the resource being investigated
	return imp.graphStore.UpsertEdge(ctx,
		"Investigation", nodeID,
		"INVESTIGATING",
		session.ResourceType, session.ResourceID,
		nil,
	)
}

func generateInvestigationID() string {
	return fmt.Sprintf("inv_%d", time.Now().UnixNano())
}
