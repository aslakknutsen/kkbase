package prometheus

import (
	"context"
	"fmt"
	"time"

	"github.com/kagenti/kkbase/pkg/observability"
)

// MockProvider is a mock implementation of MetricsProvider for testing
type MockProvider struct {
	metrics      []observability.MetricData
	queryHistory []observability.MetricQuerySpec
	shouldError  bool
	errorMessage string
	queryDelay   time.Duration
}

// NewMockProvider creates a new mock provider with empty data
func NewMockProvider() *MockProvider {
	return &MockProvider{
		metrics:      []observability.MetricData{},
		queryHistory: []observability.MetricQuerySpec{},
	}
}

// NewMockProviderWithData creates a mock provider with predefined data
func NewMockProviderWithData(metrics []observability.MetricData) *MockProvider {
	return &MockProvider{
		metrics:      metrics,
		queryHistory: []observability.MetricQuerySpec{},
	}
}

// NewMockProviderForScenario creates a mock provider for common test scenarios
func NewMockProviderForScenario(scenario string) *MockProvider {
	var metrics []observability.MetricData

	switch scenario {
	case "OOMKilled":
		metrics = GenerateOOMScenario()
	case "HighCPU":
		metrics = GenerateHighCPUScenario()
	case "HighLatency":
		metrics = GenerateLatencyScenario()
	case "Healthy":
		metrics = GenerateHealthyScenario()
	default:
		metrics = []observability.MetricData{}
	}

	return &MockProvider{
		metrics:      metrics,
		queryHistory: []observability.MetricQuerySpec{},
	}
}

// GetMetrics retrieves metrics for a specific resource
func (m *MockProvider) GetMetrics(
	ctx context.Context,
	resourceType, resourceID string,
	startTime, endTime time.Time,
) ([]observability.MetricData, error) {

	if m.shouldError {
		return nil, fmt.Errorf("%s", m.errorMessage)
	}

	if m.queryDelay > 0 {
		time.Sleep(m.queryDelay)
	}

	// Filter metrics by time range
	var result []observability.MetricData
	for _, metric := range m.metrics {
		if (metric.Timestamp.Equal(startTime) || metric.Timestamp.After(startTime)) &&
			(metric.Timestamp.Equal(endTime) || metric.Timestamp.Before(endTime)) {
			result = append(result, metric)
		}
	}

	return result, nil
}

// QueryMetrics retrieves metrics with flexible query specification
func (m *MockProvider) QueryMetrics(
	ctx context.Context,
	spec observability.MetricQuerySpec,
) ([]observability.MetricData, error) {

	// Record query for test assertions
	m.queryHistory = append(m.queryHistory, spec)

	if m.shouldError {
		return nil, fmt.Errorf("%s", m.errorMessage)
	}

	if m.queryDelay > 0 {
		time.Sleep(m.queryDelay)
	}

	// Filter metrics by spec
	var result []observability.MetricData
	for _, metric := range m.metrics {
		// Check time range
		if (metric.Timestamp.Equal(spec.StartTime) || metric.Timestamp.After(spec.StartTime)) &&
			(metric.Timestamp.Equal(spec.EndTime) || metric.Timestamp.Before(spec.EndTime)) {

			// Check metric name
			matchesName := false
			for _, name := range spec.MetricNames {
				if metric.Name == name {
					matchesName = true
					break
				}
			}
			if len(spec.MetricNames) == 0 || matchesName {
				result = append(result, metric)
			}
		}
	}

	return result, nil
}

// StreamMetrics streams real-time metrics
func (m *MockProvider) StreamMetrics(
	ctx context.Context,
	resourceType, resourceID string,
) (<-chan observability.MetricData, error) {

	if m.shouldError {
		return nil, fmt.Errorf("%s", m.errorMessage)
	}

	ch := make(chan observability.MetricData)
	go func() {
		defer close(ch)
		for _, metric := range m.metrics {
			select {
			case <-ctx.Done():
				return
			case ch <- metric:
			}
		}
	}()

	return ch, nil
}

// Close closes the provider connection
func (m *MockProvider) Close() error {
	if m.shouldError {
		return fmt.Errorf("%s", m.errorMessage)
	}
	return nil
}

// SetError configures the mock to return errors
func (m *MockProvider) SetError(shouldError bool, message string) {
	m.shouldError = shouldError
	m.errorMessage = message
}

// SetDelay configures a delay for query operations (for timeout testing)
func (m *MockProvider) SetDelay(delay time.Duration) {
	m.queryDelay = delay
}

// GetQueryHistory returns the recorded query specifications
func (m *MockProvider) GetQueryHistory() []observability.MetricQuerySpec {
	return m.queryHistory
}

// ClearQueryHistory clears the query history
func (m *MockProvider) ClearQueryHistory() {
	m.queryHistory = []observability.MetricQuerySpec{}
}

// AddMetric adds a single metric to the mock data
func (m *MockProvider) AddMetric(metric observability.MetricData) {
	m.metrics = append(m.metrics, metric)
}

// GenerateOOMScenario generates metrics showing memory climbing to limit
func GenerateOOMScenario() []observability.MetricData {
	var metrics []observability.MetricData
	baseTime := time.Now().Add(-15 * time.Minute)

	labels := map[string]string{
		"pod":       "test-pod",
		"namespace": "default",
		"container": "app",
	}

	// Generate memory usage climbing from 500MB to 1GB (limit)
	for i := 0; i < 15; i++ {
		timestamp := baseTime.Add(time.Duration(i) * time.Minute)
		memoryUsage := 500_000_000 + (i * 35_000_000) // Climbing from 500MB to 990MB

		metrics = append(metrics, observability.MetricData{
			Name:      "container_memory_usage_bytes",
			Type:      observability.MetricTypeGauge,
			Value:     float64(memoryUsage),
			Timestamp: timestamp,
			Labels:    labels,
			Unit:      "bytes",
		})

		metrics = append(metrics, observability.MetricData{
			Name:      "container_memory_working_set_bytes",
			Type:      observability.MetricTypeGauge,
			Value:     float64(memoryUsage),
			Timestamp: timestamp,
			Labels:    labels,
			Unit:      "bytes",
		})
	}

	return metrics
}

// GenerateHighCPUScenario generates metrics showing CPU at 100%
func GenerateHighCPUScenario() []observability.MetricData {
	var metrics []observability.MetricData
	baseTime := time.Now().Add(-15 * time.Minute)

	labels := map[string]string{
		"pod":       "test-pod",
		"namespace": "default",
		"container": "app",
	}

	// Generate CPU usage at 100% (1.0 cores)
	for i := 0; i < 15; i++ {
		timestamp := baseTime.Add(time.Duration(i) * time.Minute)

		metrics = append(metrics, observability.MetricData{
			Name:      "container_cpu_usage_seconds_total",
			Type:      observability.MetricTypeCounter,
			Value:     0.95 + (float64(i%3) * 0.02), // 95-99% CPU
			Timestamp: timestamp,
			Labels:    labels,
			Unit:      "seconds",
		})
	}

	return metrics
}

// GenerateLatencyScenario generates metrics showing increasing latency
func GenerateLatencyScenario() []observability.MetricData {
	var metrics []observability.MetricData
	baseTime := time.Now().Add(-15 * time.Minute)

	labels := map[string]string{
		"service":   "api-service",
		"namespace": "prod",
	}

	// Generate latency climbing from 100ms to 5s
	for i := 0; i < 15; i++ {
		timestamp := baseTime.Add(time.Duration(i) * time.Minute)
		latency := 0.1 + (float64(i) * 0.3) // 100ms to 4.5s

		metrics = append(metrics, observability.MetricData{
			Name:      "http_request_duration_seconds",
			Type:      observability.MetricTypeHistogram,
			Value:     latency,
			Timestamp: timestamp,
			Labels:    labels,
			Unit:      "seconds",
		})
	}

	return metrics
}

// GenerateHealthyScenario generates normal, healthy metrics
func GenerateHealthyScenario() []observability.MetricData {
	var metrics []observability.MetricData
	baseTime := time.Now().Add(-15 * time.Minute)

	labels := map[string]string{
		"pod":       "test-pod",
		"namespace": "default",
		"container": "app",
	}

	// Generate stable, healthy metrics
	for i := 0; i < 15; i++ {
		timestamp := baseTime.Add(time.Duration(i) * time.Minute)

		// Stable memory around 300MB
		metrics = append(metrics, observability.MetricData{
			Name:      "container_memory_usage_bytes",
			Type:      observability.MetricTypeGauge,
			Value:     300_000_000 + float64(i%5*1_000_000), // 300MB ± 5MB
			Timestamp: timestamp,
			Labels:    labels,
			Unit:      "bytes",
		})

		// Low CPU around 20%
		metrics = append(metrics, observability.MetricData{
			Name:      "container_cpu_usage_seconds_total",
			Type:      observability.MetricTypeCounter,
			Value:     0.15 + (float64(i%3) * 0.05), // 15-25% CPU
			Timestamp: timestamp,
			Labels:    labels,
			Unit:      "seconds",
		})
	}

	return metrics
}
