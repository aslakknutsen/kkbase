package observability_test

import (
	"context"
	"fmt"
	"time"

	"github.com/kagenti/kkbase/pkg/observability"
	kktesting "github.com/kagenti/kkbase/pkg/testing"
	"go.uber.org/zap"
)

// simpleMockProvider is a minimal mock for examples (avoids import cycles)
type simpleMockProvider struct{}

func (s *simpleMockProvider) GetMetrics(ctx context.Context, resourceType, resourceID string, startTime, endTime time.Time) ([]observability.MetricData, error) {
	return []observability.MetricData{}, nil
}

func (s *simpleMockProvider) QueryMetrics(ctx context.Context, spec observability.MetricQuerySpec) ([]observability.MetricData, error) {
	// Return sample data for examples
	var metrics []observability.MetricData
	baseTime := spec.StartTime
	for i := 0; i < 5; i++ {
		metrics = append(metrics, observability.MetricData{
			Name:      "container_memory_usage_bytes",
			Type:      observability.MetricTypeGauge,
			Value:     float64(500000000 + i*10000000),
			Timestamp: baseTime.Add(time.Duration(i) * time.Minute),
			Labels: map[string]string{
				"pod":       "test-pod",
				"namespace": "prod",
			},
			Unit: "bytes",
		})
	}
	return metrics, nil
}

func (s *simpleMockProvider) StreamMetrics(ctx context.Context, resourceType, resourceID string) (<-chan observability.MetricData, error) {
	ch := make(chan observability.MetricData)
	close(ch)
	return ch, nil
}

func (s *simpleMockProvider) Close() error {
	return nil
}

// ExampleInvestigationMetricsProcessor_StartInvestigation demonstrates how to start
// an investigation for a pod experiencing OOMKilled errors.
func ExampleInvestigationMetricsProcessor_StartInvestigation() {
	// Setup logger
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	// Setup graph store (use mock for this example)
	// In production, use: neo4j.NewNeo4jStore("bolt://localhost:7687", "neo4j", "password")
	graphStore := kktesting.NewMockGraphStore()
	defer graphStore.Close()

	// Setup Prometheus provider (use mock for this example)
	// In production, use: prometheus.NewProvider("http://localhost:9090", logger)
	promProvider := &simpleMockProvider{}
	defer promProvider.Close()

	// Create investigation processor
	processor := observability.NewInvestigationMetricsProcessor(
		graphStore,
		promProvider,
		logger,
	)

	// Start investigation for an OOMKilled pod
	session, err := processor.StartInvestigation(
		context.Background(),
		"Pod",                      // Resource type
		"Pod/prod/api-gateway-xyz", // Resource ID (format: Kind/Namespace/Name)
		"OOMKilled",                // Symptom
		15*time.Minute,             // Look back 15 minutes for metrics
	)
	if err != nil {
		logger.Fatal("Failed to start investigation", zap.Error(err))
	}

	fmt.Printf("Investigation started: %s\n", session.ID)
	fmt.Printf("Status: %s\n", session.Status)
	fmt.Printf("Investigating: %s\n", session.ResourceID)

	// Output format (example):
	// Investigation started: inv_1234567890
	// Status: active
	// Investigating: Pod/prod/api-gateway-xyz
}

// ExampleInvestigationMetricsProcessor_CompleteInvestigation demonstrates how to
// complete an investigation and purge temporary metrics from the graph.
func ExampleInvestigationMetricsProcessor_CompleteInvestigation() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	graphStore := kktesting.NewMockGraphStore()
	defer graphStore.Close()

	promProvider := &simpleMockProvider{}
	defer promProvider.Close()

	processor := observability.NewInvestigationMetricsProcessor(
		graphStore,
		promProvider,
		logger,
	)

	// Start investigation
	session, _ := processor.StartInvestigation(
		context.Background(),
		"Pod",
		"Pod/prod/api-gateway-xyz",
		"CrashLoopBackOff",
		15*time.Minute,
	)

	// ... Perform RCA queries using the graph ...

	// Always complete investigation to cleanup metrics
	err := processor.CompleteInvestigation(context.Background(), session.ID)
	if err != nil {
		logger.Error("Failed to complete investigation", zap.Error(err))
	}

	fmt.Println("Investigation completed and metrics purged")

	// Output:
	// Investigation completed and metrics purged
}

// ExampleInvestigationMetricsProcessor_withDefer demonstrates the recommended
// pattern using defer to ensure cleanup happens even if errors occur.
func ExampleInvestigationMetricsProcessor_withDefer() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	graphStore := kktesting.NewMockGraphStore()
	defer graphStore.Close()

	promProvider := &simpleMockProvider{}
	defer promProvider.Close()

	processor := observability.NewInvestigationMetricsProcessor(
		graphStore,
		promProvider,
		logger,
	)

	session, err := processor.StartInvestigation(
		context.Background(),
		"Service",
		"Service/prod/api-service",
		"HighLatency",
		30*time.Minute,
	)
	if err != nil {
		logger.Fatal("Failed to start investigation", zap.Error(err))
	}

	// Ensure cleanup happens even if subsequent operations fail
	defer func() {
		if err := processor.CompleteInvestigation(context.Background(), session.ID); err != nil {
			logger.Error("Failed to cleanup investigation", zap.Error(err))
		}
	}()

	// ... Perform RCA analysis ...
	// If any error occurs, defer will still execute cleanup

	fmt.Println("RCA completed, cleanup will happen automatically")

	// Output:
	// RCA completed, cleanup will happen automatically
}

// ExampleMetricCatalog_Register demonstrates how to add custom metrics to the catalog.
func ExampleMetricCatalog_Register() {
	// Create a new metric catalog
	catalog := observability.NewMetricCatalog()

	// Define a custom metric
	customMetric := observability.MetricDefinition{
		Name:              "custom_queue_depth",
		Type:              observability.MetricTypeGauge,
		Description:       "Number of items in processing queue",
		Unit:              "items",
		PromQLTemplate:    `custom_queue_depth{namespace="{{.Namespace}}", pod="{{.PodName}}"}`,
		ApplicableToTypes: []string{"Pod", "Service"},
		Category:          "performance",
	}

	// Register the custom metric
	catalog.Register(customMetric)

	// Verify registration
	metric, exists := catalog.Get("custom_queue_depth")
	if exists {
		fmt.Printf("Registered metric: %s\n", metric.Name)
		fmt.Printf("Type: %s\n", metric.Type)
		fmt.Printf("Category: %s\n", metric.Category)
	}

	// Output:
	// Registered metric: custom_queue_depth
	// Type: gauge
	// Category: performance
}

// ExampleMetricCatalog_GetMetricsForResourceType demonstrates how to get all
// metrics applicable to a specific resource type.
func ExampleMetricCatalog_GetMetricsForResourceType() {
	catalog := observability.NewMetricCatalog()

	// Get all metrics for Pods
	podMetrics := catalog.GetMetricsForResourceType("Pod")

	fmt.Printf("Found %d metrics for Pods:\n", len(podMetrics))
	for _, metric := range podMetrics {
		fmt.Printf("- %s (%s)\n", metric.Name, metric.Category)
	}

	// Output will vary based on registered metrics, example:
	// Found 6 metrics for Pods:
	// - container_memory_usage_bytes (resource)
	// - container_memory_working_set_bytes (resource)
	// - container_cpu_usage_seconds_total (resource)
	// - container_network_receive_errors_total (error)
	// - container_network_transmit_errors_total (error)
	// - http_request_duration_seconds (performance)
}

// ExampleMetricCorrelator_FindResourceFromLabels demonstrates how metrics are
// automatically correlated to Kubernetes resources using labels.
func ExampleMetricCorrelator_FindResourceFromLabels() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	graphStore := kktesting.NewMockGraphStore()
	defer graphStore.Close()

	correlator := observability.NewMetricCorrelator(graphStore, logger)

	// Metric labels from Prometheus
	labels := map[string]string{
		"pod":       "api-gateway-xyz",
		"namespace": "prod",
		"container": "app",
	}

	// Find matching resource in graph
	resourceType, resourceID := correlator.FindResourceFromLabels(
		context.Background(),
		labels,
	)

	if resourceType != "" {
		fmt.Printf("Correlated to: %s\n", resourceType)
		fmt.Printf("Resource ID: %s\n", resourceID)
	}

	// Output:
	// Correlated to: Container
	// Resource ID: Container/prod/api-gateway-xyz/app
}

// ExamplePrometheusProvider demonstrates how to query Prometheus directly.
func ExamplePrometheusProvider() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	// Create Prometheus provider (use mock for this example)
	// In production, use: prometheus.NewProvider("http://localhost:9090", logger)
	provider := &simpleMockProvider{}
	defer provider.Close()

	// Query metrics for a specific pod
	metrics, err := provider.GetMetrics(
		context.Background(),
		"Pod",
		"Pod/default/test-pod",
		time.Now().Add(-10*time.Minute),
		time.Now(),
	)

	if err != nil {
		logger.Error("Failed to query metrics", zap.Error(err))
		return
	}

	fmt.Printf("Retrieved %d metric data points\n", len(metrics))
	for i, metric := range metrics {
		if i >= 3 { // Show only first 3
			fmt.Println("...")
			break
		}
		fmt.Printf("- %s: %.2f %s at %s\n",
			metric.Name,
			metric.Value,
			metric.Unit,
			metric.Timestamp.Format("15:04:05"))
	}

	// Output (example):
	// Retrieved 20 metric data points
	// - container_memory_usage_bytes: 524288000 bytes at 14:30:00
	// - container_memory_usage_bytes: 526336000 bytes at 14:31:00
	// - container_cpu_usage_seconds_total: 0.45 seconds at 14:30:00
	// ...
}

// ExamplePrometheusProvider_queryMetrics demonstrates advanced metric querying
// with flexible specifications.
func ExamplePrometheusProvider_queryMetrics() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	provider := &simpleMockProvider{}
	defer provider.Close()

	// Build detailed query specification
	spec := observability.MetricQuerySpec{
		MetricNames: []string{
			"container_memory_usage_bytes",
			"container_cpu_usage_seconds_total",
		},
		ResourceType: "Pod",
		ResourceID:   "Pod/prod/api-gateway",
		StartTime:    time.Now().Add(-30 * time.Minute),
		EndTime:      time.Now(),
		StepDuration: 1 * time.Minute, // 1-minute resolution
		Labels: map[string]string{
			"namespace": "prod",
			"pod":       "api-gateway",
		},
	}

	metrics, err := provider.QueryMetrics(context.Background(), spec)
	if err != nil {
		logger.Error("Failed to query metrics", zap.Error(err))
		return
	}

	// Group by metric name
	metricsByName := make(map[string]int)
	for _, metric := range metrics {
		metricsByName[metric.Name]++
	}

	fmt.Printf("Retrieved metrics:\n")
	for name, count := range metricsByName {
		fmt.Printf("- %s: %d data points\n", name, count)
	}

	// Output (example):
	// Retrieved metrics:
	// - container_memory_usage_bytes: 30 data points
	// - container_cpu_usage_seconds_total: 30 data points
}

// ExampleHighLatencyInvestigation demonstrates a complete workflow for investigating
// high latency in a service.
func ExampleHighLatencyInvestigation() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	graphStore := kktesting.NewMockGraphStore()
	defer graphStore.Close()

	promProvider := &simpleMockProvider{}
	defer promProvider.Close()

	processor := observability.NewInvestigationMetricsProcessor(
		graphStore,
		promProvider,
		logger,
	)

	// 1. Start investigation
	session, err := processor.StartInvestigation(
		context.Background(),
		"Service",
		"Service/prod/api-service",
		"HighLatency",
		30*time.Minute,
	)
	if err != nil {
		logger.Fatal("Failed to start investigation", zap.Error(err))
	}
	defer processor.CompleteInvestigation(context.Background(), session.ID)

	// 2. Query graph for latency metrics
	query := `
		MATCH (inv:Investigation {investigation_id: $inv_id})
		-[:HAS_METRIC_EVIDENCE]->(m:Metric)
		WHERE m.name = 'http_request_duration_seconds'
		RETURN 
		  datetime(m.timestamp) as time,
		  m.value * 1000 as latency_ms
		ORDER BY time ASC
	`

	results, err := graphStore.Query(context.Background(), query, map[string]interface{}{
		"inv_id": "Investigation/" + session.ID,
	})
	if err != nil {
		logger.Error("Query failed", zap.Error(err))
		return
	}

	// 3. Analyze results
	fmt.Printf("Latency analysis for service:\n")
	for i, row := range results {
		if i >= 5 { // Show first 5
			fmt.Printf("... (%d more data points)\n", len(results)-5)
			break
		}
		fmt.Printf("- %v: %.2f ms\n", row["time"], row["latency_ms"])
	}

	// Output (example):
	// Latency analysis for service:
	// - 2025-01-15 14:30:00: 125.50 ms
	// - 2025-01-15 14:31:00: 230.75 ms
	// - 2025-01-15 14:32:00: 445.20 ms
	// - 2025-01-15 14:33:00: 523.80 ms
	// - 2025-01-15 14:34:00: 612.45 ms
	// ... (25 more data points)
}
