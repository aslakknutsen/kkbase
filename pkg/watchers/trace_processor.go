package watchers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/kagenti/kkbase/pkg/graph"
	"github.com/kagenti/kkbase/pkg/models"
	"github.com/kagenti/kkbase/pkg/observability"
	"go.uber.org/zap"
)

// TraceProcessor processes traces and updates the graph
type TraceProcessor struct {
	graphStore      graph.GraphStore
	logger          *zap.Logger
	spanRetention   time.Duration
	lastCleanup     time.Time
	cleanupInterval time.Duration
}

// NewTraceProcessor creates a new trace processor
func NewTraceProcessor(graphStore graph.GraphStore, logger *zap.Logger, spanRetention time.Duration) *TraceProcessor {
	return &TraceProcessor{
		graphStore:      graphStore,
		logger:          logger,
		spanRetention:   spanRetention,
		lastCleanup:     time.Now(),
		cleanupInterval: 10 * time.Minute,
	}
}

// ProcessTrace processes a single trace and creates graph nodes/edges
func (tp *TraceProcessor) ProcessTrace(ctx context.Context, trace observability.Trace) error {
	// Create Trace node (lightweight aggregation)
	traceNode := models.NewGraphNode(
		models.NodeTypeTrace,
		fmt.Sprintf("Trace/%s", trace.TraceID),
		map[string]interface{}{
			"trace_id":          trace.TraceID,
			"start_time":        trace.StartTime.Format(time.RFC3339),
			"duration_ms":       trace.Duration.Seconds() * 1000,
			"root_operation":    trace.RootOperation,
			"root_service":      trace.RootService,
			"span_count":        trace.SpanCount,
			"error_count":       trace.ErrorCount,
			"has_errors":        trace.HasErrors,
			"services_involved": strings.Join(trace.Services, ","),
		},
	)

	if err := tp.graphStore.UpsertNode(ctx, string(traceNode.Type), traceNode.ID, traceNode.Properties); err != nil {
		return fmt.Errorf("failed to create trace node: %w", err)
	}

	// Process each span
	for _, span := range trace.Spans {
		if err := tp.processSpan(ctx, trace.TraceID, span); err != nil {
			tp.logger.Warn("failed to process span",
				zap.String("trace_id", trace.TraceID),
				zap.String("span_id", span.SpanID),
				zap.Error(err))
		}
	}

	// Periodically cleanup old spans
	if time.Since(tp.lastCleanup) > tp.cleanupInterval {
		go tp.cleanupOldSpans(context.Background())
		tp.lastCleanup = time.Now()
	}

	return nil
}

// processSpan processes a single span
func (tp *TraceProcessor) processSpan(ctx context.Context, traceID string, span observability.TraceSpan) error {
	spanID := fmt.Sprintf("Span/%s/%s", traceID, span.SpanID)

	// Create Span node with OpenTelemetry 1.21+ attributes
	spanProps := map[string]interface{}{
		"span_id":           span.SpanID,
		"trace_id":          span.TraceID,
		"parent_span_id":    span.ParentID,
		"operation_name":    span.OperationName,
		"service_name":      span.Service,
		"service_namespace": span.Namespace,
		"start_time":        span.StartTime.Format(time.RFC3339),
		"duration_us":       span.Duration.Microseconds(),
		"duration_ms":       span.Duration.Seconds() * 1000,
		"span_kind":         span.SpanKind,
		"status":            span.Status,
		"error":             span.Error,
		"protocol":          span.Protocol,
	}

	// Add HTTP/URL attributes
	if span.HTTPRequestMethod != "" {
		spanProps["http_request_method"] = span.HTTPRequestMethod
		spanProps["http_response_status_code"] = span.HTTPResponseStatusCode
		spanProps["url_path"] = span.URLPath
		spanProps["url_scheme"] = span.URLScheme
		if span.URLFull != "" {
			spanProps["url_full"] = span.URLFull
		}
	}

	// Add network attributes
	if span.NetworkProtocolName != "" {
		spanProps["network_protocol_name"] = span.NetworkProtocolName
		if span.NetworkProtocolVersion != "" {
			spanProps["network_protocol_version"] = span.NetworkProtocolVersion
		}
		if span.NetworkTransport != "" {
			spanProps["network_transport"] = span.NetworkTransport
		}
	}

	// Add server/client addressing
	if span.ServerAddress != "" {
		spanProps["server_address"] = span.ServerAddress
		if span.ServerPort > 0 {
			spanProps["server_port"] = span.ServerPort
		}
	}
	if span.ClientAddress != "" {
		spanProps["client_address"] = span.ClientAddress
	}

	// Add RPC attributes
	if span.RPCService != "" {
		spanProps["rpc_system"] = span.RPCSystem
		spanProps["rpc_service"] = span.RPCService
		spanProps["rpc_method"] = span.RPCMethod
		if span.RPCGRPCStatusCode > 0 {
			spanProps["rpc_grpc_status_code"] = span.RPCGRPCStatusCode
		}
	}

	// Add error attributes
	if span.ErrorMessage != "" {
		spanProps["error_message"] = span.ErrorMessage
	}
	if span.ErrorType != "" {
		spanProps["error_type"] = span.ErrorType
	}

	// Add user agent
	if span.UserAgent != "" {
		spanProps["user_agent"] = span.UserAgent
	}

	// Add Kubernetes metadata
	if span.K8sPodName != "" {
		spanProps["k8s_pod_name"] = span.K8sPodName
	}
	if span.K8sNodeName != "" {
		spanProps["k8s_node_name"] = span.K8sNodeName
	}
	if span.ServiceInstanceID != "" {
		spanProps["service_instance_id"] = span.ServiceInstanceID
	}
	if span.ServiceVersion != "" {
		spanProps["service_version"] = span.ServiceVersion
	}

	if err := tp.graphStore.UpsertNode(ctx, "Span", spanID, spanProps); err != nil {
		return fmt.Errorf("failed to create span node: %w", err)
	}

	// Create CONTAINS_SPAN edge from Trace to Span
	traceNodeID := fmt.Sprintf("Trace/%s", traceID)
	if err := tp.graphStore.UpsertEdge(ctx, "Trace", traceNodeID, "CONTAINS_SPAN", "Span", spanID, nil); err != nil {
		return fmt.Errorf("failed to create CONTAINS_SPAN edge: %w", err)
	}

	// Create PARENT_OF edge if parent exists
	if span.ParentID != "" {
		parentSpanID := fmt.Sprintf("Span/%s/%s", traceID, span.ParentID)
		if err := tp.graphStore.UpsertEdge(ctx, "Span", parentSpanID, "PARENT_OF", "Span", spanID, nil); err != nil {
			tp.logger.Debug("failed to create PARENT_OF edge", zap.Error(err))
		}
	}

	// Link span to K8s Service (ORIGINATED_FROM)
	if span.Service != "" && span.Namespace != "" {
		serviceID := fmt.Sprintf("Service/%s/%s", span.Namespace, span.Service)
		if err := tp.graphStore.UpsertEdge(ctx, "Span", spanID, "ORIGINATED_FROM", "Service", serviceID, nil); err != nil {
			tp.logger.Warn("failed to create ORIGINATED_FROM edge - Service may not exist",
				zap.String("span_service", span.Service),
				zap.String("span_namespace", span.Namespace),
				zap.String("expected_service_id", serviceID),
				zap.String("span_id", spanID),
				zap.Error(err))
		} else {
			tp.logger.Debug("successfully linked span to service",
				zap.String("span_service", span.Service),
				zap.String("service_id", serviceID))
		}
	} else {
		tp.logger.Debug("span missing service or namespace info",
			zap.String("service", span.Service),
			zap.String("namespace", span.Namespace),
			zap.String("span_id", span.SpanID))
	}

	// Create runtime service call edge (CALLS or FAILED_CALL_TO)
	// Check if this span represents a client call to another service
	if span.ServerAddress != "" || span.URLFull != "" {
		if err := tp.createServiceCallEdge(ctx, span); err != nil {
			tp.logger.Debug("failed to create service call edge", zap.Error(err))
		}
	}

	return nil
}

// createServiceCallEdge creates runtime CALLS or FAILED_CALL_TO edges between services
func (tp *TraceProcessor) createServiceCallEdge(ctx context.Context, span observability.TraceSpan) error {
	// Determine target from ServerAddress or URLFull
	targetAddress := span.ServerAddress
	if targetAddress == "" && span.URLFull != "" {
		// Extract server address from full URL if ServerAddress is missing
		targetAddress = tp.extractHostFromURL(span.URLFull)
	}

	if targetAddress == "" {
		return nil
	}

	// Parse server address to extract target service/namespace
	targetService, targetNamespace := tp.parseServerAddress(targetAddress)
	if targetService == "" {
		return nil
	}

	fromServiceID := fmt.Sprintf("Service/%s/%s", span.Namespace, span.Service)
	toServiceID := fmt.Sprintf("Service/%s/%s", targetNamespace, targetService)

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

	return tp.graphStore.UpsertEdge(ctx, "Service", fromServiceID, edgeType, "Service", toServiceID, edgeProps)
}

// parseServerAddress extracts service name and namespace from server address
// e.g., "payment.sf-payments.svc.cluster.local:9090" -> ("payment", "sf-payments")
// e.g., "payment.sf-payments.svc.cluster.local" -> ("payment", "sf-payments")
func (tp *TraceProcessor) parseServerAddress(serverAddress string) (service, namespace string) {
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
func (tp *TraceProcessor) extractHostFromURL(fullURL string) string {
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

// cleanupOldSpans deletes Span nodes older than retention period
func (tp *TraceProcessor) cleanupOldSpans(ctx context.Context) {
	cutoffTime := time.Now().Add(-tp.spanRetention)

	tp.logger.Info("cleaning up old spans",
		zap.Time("cutoff", cutoffTime),
		zap.Duration("retention", tp.spanRetention))

	// Cypher query to delete old spans
	query := `
		MATCH (s:Span)
		WHERE datetime(s.start_time) < datetime($cutoff)
		DETACH DELETE s
		RETURN count(s) as deleted_count
	`

	result, err := tp.graphStore.Query(ctx, query, map[string]interface{}{
		"cutoff": cutoffTime.Format(time.RFC3339),
	})

	if err != nil {
		tp.logger.Error("failed to cleanup old spans", zap.Error(err))
		return
	}

	if len(result) > 0 {
		if deletedCount, ok := result[0]["deleted_count"].(int64); ok {
			tp.logger.Info("cleaned up old spans",
				zap.Int64("deleted_count", deletedCount))
		}
	}
}
