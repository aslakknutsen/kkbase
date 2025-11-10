package prometheus

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/aslakknutsen/kkbase/pkg/observability"
	"go.uber.org/zap"
)

// Provider implements MetricsProvider for Prometheus Query API
type Provider struct {
	baseURL    string
	httpClient *http.Client
	logger     *zap.Logger
}

// NewProvider creates a new Prometheus metrics provider
func NewProvider(baseURL string, logger *zap.Logger) *Provider {
	return &Provider{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

// GetMetrics retrieves metrics for a specific resource (simplified interface)
func (p *Provider) GetMetrics(
	ctx context.Context,
	resourceType, resourceID string,
	startTime, endTime time.Time,
) ([]observability.MetricData, error) {

	// Parse resourceID to extract namespace and name
	namespace, name := parseResourceID(resourceID)

	// Build basic query spec
	spec := observability.MetricQuerySpec{
		ResourceType: resourceType,
		ResourceID:   resourceID,
		StartTime:    startTime,
		EndTime:      endTime,
		StepDuration: 1 * time.Minute,
		Labels: map[string]string{
			"namespace": namespace,
		},
	}

	// Add resource-specific labels
	switch resourceType {
	case "Pod":
		spec.Labels["pod"] = name
		spec.MetricNames = []string{
			"container_memory_usage_bytes",
			"container_cpu_usage_seconds_total",
		}
	case "Node":
		spec.Labels["node"] = name
		spec.MetricNames = []string{
			"node_cpu_seconds_total",
			"node_memory_MemAvailable_bytes",
		}
	case "Service":
		spec.Labels["service"] = name
		spec.MetricNames = []string{
			"http_requests_total",
			"http_request_duration_seconds",
		}
	default:
		return nil, fmt.Errorf("unsupported resource type: %s", resourceType)
	}

	return p.QueryMetrics(ctx, spec)
}

// QueryMetrics retrieves metrics with flexible query specification
func (p *Provider) QueryMetrics(
	ctx context.Context,
	spec observability.MetricQuerySpec,
) ([]observability.MetricData, error) {

	var allMetrics []observability.MetricData

	// Query each metric name
	for _, metricName := range spec.MetricNames {
		// Build PromQL query
		query := p.buildPromQLForResource(metricName, spec.Labels)

		p.logger.Debug("querying Prometheus",
			zap.String("metric", metricName),
			zap.String("query", query),
			zap.Time("start", spec.StartTime),
			zap.Time("end", spec.EndTime))

		// Execute range query
		metrics, err := p.queryRange(ctx, query, spec.StartTime, spec.EndTime, spec.StepDuration)
		if err != nil {
			p.logger.Warn("failed to query metric",
				zap.String("metric", metricName),
				zap.Error(err))
			continue // Continue with other metrics
		}

		allMetrics = append(allMetrics, metrics...)
	}

	return allMetrics, nil
}

// StreamMetrics streams real-time metrics (polling-based implementation)
func (p *Provider) StreamMetrics(
	ctx context.Context,
	resourceType, resourceID string,
) (<-chan observability.MetricData, error) {
	// For now, return an error as streaming is not yet implemented
	return nil, fmt.Errorf("streaming metrics not yet implemented")
}

// Close closes the provider connection (no-op for HTTP client)
func (p *Provider) Close() error {
	return nil
}

// queryRange executes a range query against Prometheus
func (p *Provider) queryRange(
	ctx context.Context,
	query string,
	start, end time.Time,
	step time.Duration,
) ([]observability.MetricData, error) {

	// Build query URL
	params := url.Values{}
	params.Set("query", query)
	params.Set("start", fmt.Sprintf("%d", start.Unix()))
	params.Set("end", fmt.Sprintf("%d", end.Unix()))
	params.Set("step", fmt.Sprintf("%ds", int(step.Seconds())))

	queryURL := fmt.Sprintf("%s/api/v1/query_range?%s", p.baseURL, params.Encode())

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", queryURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Execute request
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to query Prometheus: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check HTTP status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Prometheus returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	metrics, err := p.parsePrometheusResponse(body, query)
	if err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return metrics, nil
}

// parsePrometheusResponse parses the Prometheus API response
func (p *Provider) parsePrometheusResponse(body []byte, query string) ([]observability.MetricData, error) {
	var response PrometheusResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	// Check for errors in response
	if response.Status != "success" {
		return nil, fmt.Errorf("Prometheus error: %s (%s)", response.Error, response.ErrorType)
	}

	// Extract metric name from query (simple heuristic)
	metricName := extractMetricNameFromQuery(query)

	// Convert results to MetricData
	var metrics []observability.MetricData
	for _, result := range response.Data.Result {
		convertedMetrics, err := result.ConvertToMetricData(metricName)
		if err != nil {
			p.logger.Warn("failed to convert result",
				zap.Error(err))
			continue
		}
		metrics = append(metrics, convertedMetrics...)
	}

	return metrics, nil
}

// buildPromQLForResource builds a PromQL query with label filters
func (p *Provider) buildPromQLForResource(metricName string, labels map[string]string) string {
	// Start with metric name
	query := metricName

	// Add label filters
	if len(labels) > 0 {
		var labelFilters []string
		for k, v := range labels {
			if v != "" {
				labelFilters = append(labelFilters, fmt.Sprintf(`%s="%s"`, k, v))
			}
		}
		if len(labelFilters) > 0 {
			query = fmt.Sprintf("%s{%s}", metricName, strings.Join(labelFilters, ","))
		}
	}

	// Add rate() for counter metrics
	if strings.HasSuffix(metricName, "_total") || strings.HasSuffix(metricName, "_seconds_total") {
		query = fmt.Sprintf("rate(%s[1m])", query)
	}

	return query
}

// parseResourceID extracts namespace and name from resource ID
// Format: "Kind/namespace/name" or "Kind/name" (for cluster-scoped)
func parseResourceID(resourceID string) (namespace, name string) {
	parts := strings.Split(resourceID, "/")
	if len(parts) == 3 {
		// Namespaced: "Pod/prod/api-gateway"
		return parts[1], parts[2]
	} else if len(parts) == 2 {
		// Cluster-scoped: "Node/node-1"
		return "", parts[1]
	}
	// Fallback
	return "", resourceID
}

// extractMetricNameFromQuery extracts the metric name from a PromQL query
// This is a simple heuristic and may not work for complex queries
func extractMetricNameFromQuery(query string) string {
	// Remove rate() or other functions
	query = strings.TrimPrefix(query, "rate(")
	query = strings.TrimPrefix(query, "histogram_quantile(0.95,")
	query = strings.TrimSuffix(query, ")")

	// Extract metric name before {
	if idx := strings.Index(query, "{"); idx > 0 {
		return query[:idx]
	}

	// If no labels, return the whole query (cleaned)
	return strings.TrimSpace(query)
}
