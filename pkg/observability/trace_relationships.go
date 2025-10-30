package observability

import (
	"context"
	"strings"
	"time"

	"github.com/kagenti/kkbase/pkg/graph"
	"github.com/kagenti/kkbase/pkg/models"
	"go.uber.org/zap"
)

// TraceRelationshipBuilder helps build relationships for trace/span resources
type TraceRelationshipBuilder struct {
	graphStore graph.GraphStore
	logger     *zap.Logger
}

// NewTraceRelationshipBuilder creates a new trace relationship builder
func NewTraceRelationshipBuilder(graphStore graph.GraphStore, logger *zap.Logger) *TraceRelationshipBuilder {
	return &TraceRelationshipBuilder{
		graphStore: graphStore,
		logger:     logger,
	}
}

// CreateTraceContainsSpanEdge creates CONTAINS_SPAN edge from Trace to Span
func (trb *TraceRelationshipBuilder) CreateTraceContainsSpanEdge(ctx context.Context, traceID, spanID string) error {
	traceNodeID := models.GetNodeID("Trace", "", traceID)
	return trb.graphStore.UpsertEdge(ctx, "Trace", traceNodeID, "CONTAINS_SPAN", "Span", spanID, nil)
}

// CreateSpanParentEdge creates PARENT_OF edge between spans
func (trb *TraceRelationshipBuilder) CreateSpanParentEdge(ctx context.Context, traceID, parentSpanID, childSpanID string) error {
	parentID := models.GetNodeID("Span", traceID, parentSpanID)
	return trb.graphStore.UpsertEdge(ctx, "Span", parentID, "PARENT_OF", "Span", childSpanID, nil)
}

// CreateSpanOriginatedFromServiceEdge creates ORIGINATED_FROM edge from Span to Service
func (trb *TraceRelationshipBuilder) CreateSpanOriginatedFromServiceEdge(ctx context.Context, spanID, service, namespace string) error {
	if service == "" || namespace == "" {
		trb.logger.Debug("span missing service or namespace info",
			zap.String("service", service),
			zap.String("namespace", namespace),
			zap.String("span_id", spanID))
		return nil
	}

	serviceID := models.GetNodeID("Service", namespace, service)
	if err := trb.graphStore.UpsertEdge(ctx, "Span", spanID, "ORIGINATED_FROM", "Service", serviceID, nil); err != nil {
		trb.logger.Warn("failed to create ORIGINATED_FROM edge - Service may not exist",
			zap.String("span_service", service),
			zap.String("span_namespace", namespace),
			zap.String("expected_service_id", serviceID),
			zap.String("span_id", spanID),
			zap.Error(err))
		return err
	}

	trb.logger.Debug("successfully linked span to service",
		zap.String("span_service", service),
		zap.String("service_id", serviceID))
	return nil
}

// CreateSpanExecutedInPodEdge creates EXECUTED_IN edge from Span to Pod
func (trb *TraceRelationshipBuilder) CreateSpanExecutedInPodEdge(ctx context.Context, spanID, podName, namespace string) error {
	if podName == "" || namespace == "" {
		trb.logger.Debug("span missing pod or namespace info",
			zap.String("pod", podName),
			zap.String("namespace", namespace),
			zap.String("span_id", spanID))
		return nil
	}

	podID := models.GetNodeID("Pod", namespace, podName)
	if err := trb.graphStore.UpsertEdge(ctx, "Span", spanID, "EXECUTED_IN", "Pod", podID, nil); err != nil {
		trb.logger.Warn("failed to create EXECUTED_IN edge - Pod may not exist",
			zap.String("span_pod", podName),
			zap.String("span_namespace", namespace),
			zap.String("expected_pod_id", podID),
			zap.String("span_id", spanID),
			zap.Error(err))
		return err
	}

	trb.logger.Debug("successfully linked span to pod",
		zap.String("span_pod", podName),
		zap.String("pod_id", podID))
	return nil
}

// CreateServiceCallEdge creates runtime CALLS or FAILED_CALL_TO edges between services
func (trb *TraceRelationshipBuilder) CreateServiceCallEdge(ctx context.Context, span TraceSpan) error {
	// Determine target from ServerAddress or URLFull
	targetAddress := span.ServerAddress
	if targetAddress == "" && span.URLFull != "" {
		// Extract server address from full URL if ServerAddress is missing
		targetAddress = trb.extractHostFromURL(span.URLFull)
	}

	if targetAddress == "" {
		return nil
	}

	// Parse server address to extract target service/namespace
	targetService, targetNamespace := trb.parseServerAddress(targetAddress)
	if targetService == "" {
		return nil
	}

	fromServiceID := models.GetNodeID("Service", span.Namespace, span.Service)
	toServiceID := models.GetNodeID("Service", targetNamespace, targetService)

	// Create or update CALLS edge with runtime metrics
	edgeProps := map[string]interface{}{
		"source":        "trace_observed",
		"protocol":      span.Protocol,
		"last_observed": span.StartTime.Format(time.RFC3339),
		"duration_ms":   span.Duration.Seconds() * 1000,
		"status_code":   span.HTTPResponseStatusCode,
		"error":         span.Error,
	}

	edgeType := "CALLS"
	if span.Error {
		edgeType = "FAILED_CALL_TO"
		edgeProps["error_message"] = span.ErrorMessage
		if span.ErrorType != "" {
			edgeProps["error_type"] = span.ErrorType
		}
	}

	return trb.graphStore.UpsertEdge(ctx, "Service", fromServiceID, edgeType, "Service", toServiceID, edgeProps)
}

// parseServerAddress extracts service name and namespace from server address
// e.g., "payment.sf-payments.svc.cluster.local:9090" -> ("payment", "sf-payments")
// e.g., "payment.sf-payments.svc.cluster.local" -> ("payment", "sf-payments")
func (trb *TraceRelationshipBuilder) parseServerAddress(serverAddress string) (service, namespace string) {
	// Split host:port
	host := strings.Split(serverAddress, ":")[0]

	// Parse K8s DNS format: service.namespace.svc.cluster.local
	hostParts := strings.Split(host, ".")
	if len(hostParts) >= 2 {
		return hostParts[0], hostParts[1]
	}

	return "", ""
}

// extractHostFromURL extracts the host portion from a full URL
// e.g., "http://payment.sf-payments.svc.cluster.local:9090/api/v1" -> "payment.sf-payments.svc.cluster.local:9090"
func (trb *TraceRelationshipBuilder) extractHostFromURL(fullURL string) string {
	// Remove protocol
	url := strings.TrimPrefix(fullURL, "http://")
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "grpc://")

	// Split host:port and path
	parts := strings.Split(url, "/")
	if len(parts) > 0 {
		return parts[0]
	}

	return ""
}
