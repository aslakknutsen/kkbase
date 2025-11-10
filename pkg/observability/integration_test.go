package observability

import (
	"context"
	"fmt"
	"testing"
	"time"

	kktesting "github.com/aslakknutsen/kkbase/pkg/testing"
	"go.uber.org/zap"
)

// TestOOMInvestigationWorkflow tests the complete workflow for investigating an OOMKilled pod
func TestOOMInvestigationWorkflow(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	mockStore := kktesting.NewMockGraphStore()

	// Setup: Create Pod node in graph
	podID := "Pod/default/test-pod"
	mockStore.UpsertNode(ctx, "Pod", podID, map[string]interface{}{
		"id":           podID,
		"name":         "test-pod",
		"namespace":    "default",
		"memory_limit": "1Gi",
		"status":       "CrashLoopBackOff",
	})

	// Setup: Generate OOM scenario metrics
	oomMetrics := generateOOMMetrics()
	mockProvider := newTestMockProvider(oomMetrics)

	// Create processor
	processor := NewInvestigationMetricsProcessor(mockStore, mockProvider, logger)

	// Start investigation
	session, err := processor.StartInvestigation(
		ctx,
		"Pod",
		podID,
		"OOMKilled",
		15*time.Minute,
	)

	if err != nil {
		t.Fatalf("Failed to start investigation: %v", err)
	}

	// Verify session was created
	if session.Status != "active" {
		t.Errorf("Expected status 'active', got %s", session.Status)
	}
	if session.Symptom != "OOMKilled" {
		t.Errorf("Expected symptom 'OOMKilled', got %s", session.Symptom)
	}

	// Verify metrics were pulled
	queryHistory := mockProvider.queryHistory
	if len(queryHistory) == 0 {
		t.Error("Expected metrics to be pulled")
	}

	// Verify correct metrics were requested for OOM investigation
	if len(queryHistory) > 0 {
		spec := queryHistory[0]
		expectedMetrics := []string{"container_memory_usage_bytes", "container_memory_working_set_bytes"}
		foundMemoryMetric := false
		for _, name := range spec.MetricNames {
			for _, expected := range expectedMetrics {
				if name == expected {
					foundMemoryMetric = true
					break
				}
			}
		}
		if !foundMemoryMetric {
			t.Error("Expected memory metrics to be requested for OOM investigation")
		}
	}

	// Setup mock for cleanup query
	mockStore.SetQueryResult([]map[string]interface{}{
		{"deleted_count": int64(len(oomMetrics))},
	})

	// Complete investigation
	err = processor.CompleteInvestigation(ctx, session.ID)
	if err != nil {
		t.Fatalf("Failed to complete investigation: %v", err)
	}
}

// TestHighLatencyInvestigation tests investigating high latency issues
func TestHighLatencyInvestigation(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	mockStore := kktesting.NewMockGraphStore()

	// Setup: Create Service and Pod nodes
	serviceID := "Service/prod/api-service"
	podID := "Pod/prod/api-pod-1"

	mockStore.UpsertNode(ctx, "Service", serviceID, map[string]interface{}{
		"id":        serviceID,
		"name":      "api-service",
		"namespace": "prod",
	})

	mockStore.UpsertNode(ctx, "Pod", podID, map[string]interface{}{
		"id":        podID,
		"name":      "api-pod-1",
		"namespace": "prod",
	})

	// Generate latency scenario metrics
	latencyMetrics := generateHighLatencyMetrics()
	mockProvider := newTestMockProvider(latencyMetrics)

	processor := NewInvestigationMetricsProcessor(mockStore, mockProvider, logger)

	// Start investigation
	session, err := processor.StartInvestigation(
		ctx,
		"Service",
		serviceID,
		"HighLatency",
		15*time.Minute,
	)

	if err != nil {
		t.Fatalf("Failed to start investigation: %v", err)
	}

	// Verify latency metrics were requested
	queryHistory := mockProvider.queryHistory
	if len(queryHistory) > 0 {
		spec := queryHistory[0]
		foundLatencyMetric := false
		for _, name := range spec.MetricNames {
			if name == "http_request_duration_seconds" {
				foundLatencyMetric = true
				break
			}
		}
		if !foundLatencyMetric {
			t.Error("Expected latency metrics to be requested")
		}
	}

	// Setup mock for cleanup
	mockStore.SetQueryResult([]map[string]interface{}{
		{"deleted_count": int64(len(latencyMetrics))},
	})

	// Complete investigation
	err = processor.CompleteInvestigation(ctx, session.ID)
	if err != nil {
		t.Fatalf("Failed to complete investigation: %v", err)
	}
}

// TestNodeResourceContention tests investigating node-level resource issues
func TestNodeResourceContention(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	mockStore := kktesting.NewMockGraphStore()

	// Setup: Create Node and multiple Pods
	nodeID := "Node/worker-1"
	mockStore.UpsertNode(ctx, "Node", nodeID, map[string]interface{}{
		"id":   nodeID,
		"name": "worker-1",
	})

	// Create multiple pods on this node
	for i := 1; i <= 3; i++ {
		podID := fmt.Sprintf("Pod/default/pod-%d", i)
		mockStore.UpsertNode(ctx, "Pod", podID, map[string]interface{}{
			"id":        podID,
			"name":      fmt.Sprintf("pod-%d", i),
			"namespace": "default",
		})
	}

	// Generate high CPU metrics
	cpuMetrics := generateHighCPUMetrics()
	mockProvider := newTestMockProvider(cpuMetrics)

	processor := NewInvestigationMetricsProcessor(mockStore, mockProvider, logger)

	// Start investigation
	session, err := processor.StartInvestigation(
		ctx,
		"Node",
		nodeID,
		"NodeNotReady",
		15*time.Minute,
	)

	if err != nil {
		t.Fatalf("Failed to start investigation: %v", err)
	}

	// Verify node metrics were requested
	queryHistory := mockProvider.queryHistory
	if len(queryHistory) > 0 {
		spec := queryHistory[0]
		foundNodeMetric := false
		for _, name := range spec.MetricNames {
			if name == "node_cpu_seconds_total" || name == "node_memory_MemAvailable_bytes" {
				foundNodeMetric = true
				break
			}
		}
		if !foundNodeMetric {
			t.Error("Expected node metrics to be requested")
		}
	}

	// Setup mock for cleanup
	mockStore.SetQueryResult([]map[string]interface{}{
		{"deleted_count": int64(len(cpuMetrics))},
	})

	// Complete investigation
	err = processor.CompleteInvestigation(ctx, session.ID)
	if err != nil {
		t.Fatalf("Failed to complete investigation: %v", err)
	}
}

// TestInvestigationWithoutMetrics tests graceful handling when Prometheus is unavailable
func TestInvestigationWithoutMetrics(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	mockStore := kktesting.NewMockGraphStore()

	// Setup: Create Pod node
	podID := "Pod/default/test-pod"
	mockStore.UpsertNode(ctx, "Pod", podID, map[string]interface{}{
		"id":        podID,
		"name":      "test-pod",
		"namespace": "default",
	})

	// Create processor without metrics provider
	processor := NewInvestigationMetricsProcessor(mockStore, nil, logger)

	// Start investigation (should not fail)
	session, err := processor.StartInvestigation(
		ctx,
		"Pod",
		podID,
		"CrashLoopBackOff",
		15*time.Minute,
	)

	if err != nil {
		t.Fatalf("Investigation should proceed without metrics provider: %v", err)
	}

	if session.Status != "active" {
		t.Errorf("Expected status 'active', got %s", session.Status)
	}

	// Setup mock for cleanup
	mockStore.SetQueryResult([]map[string]interface{}{
		{"deleted_count": int64(0)},
	})

	// Complete investigation (should work even without metrics)
	err = processor.CompleteInvestigation(ctx, session.ID)
	if err != nil {
		t.Fatalf("Failed to complete investigation: %v", err)
	}
}

// Helper functions to generate test metrics

func generateOOMMetrics() []MetricData {
	var metrics []MetricData
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

		metrics = append(metrics, MetricData{
			Name:      "container_memory_usage_bytes",
			Type:      MetricTypeGauge,
			Value:     float64(memoryUsage),
			Timestamp: timestamp,
			Labels:    labels,
			Unit:      "bytes",
		})

		metrics = append(metrics, MetricData{
			Name:      "container_memory_working_set_bytes",
			Type:      MetricTypeGauge,
			Value:     float64(memoryUsage),
			Timestamp: timestamp,
			Labels:    labels,
			Unit:      "bytes",
		})
	}

	return metrics
}

func generateHighLatencyMetrics() []MetricData {
	var metrics []MetricData
	baseTime := time.Now().Add(-15 * time.Minute)

	labels := map[string]string{
		"service":   "api-service",
		"namespace": "prod",
	}

	// Generate latency climbing from 100ms to 5s
	for i := 0; i < 15; i++ {
		timestamp := baseTime.Add(time.Duration(i) * time.Minute)
		latency := 0.1 + (float64(i) * 0.3) // 100ms to 4.5s

		metrics = append(metrics, MetricData{
			Name:      "http_request_duration_seconds",
			Type:      MetricTypeHistogram,
			Value:     latency,
			Timestamp: timestamp,
			Labels:    labels,
			Unit:      "seconds",
		})
	}

	return metrics
}

func generateHighCPUMetrics() []MetricData {
	var metrics []MetricData
	baseTime := time.Now().Add(-15 * time.Minute)

	labels := map[string]string{
		"node": "worker-1",
	}

	// Generate CPU usage at 95-100%
	for i := 0; i < 15; i++ {
		timestamp := baseTime.Add(time.Duration(i) * time.Minute)

		metrics = append(metrics, MetricData{
			Name:      "node_cpu_seconds_total",
			Type:      MetricTypeCounter,
			Value:     0.95 + (float64(i%3) * 0.02), // 95-99% CPU
			Timestamp: timestamp,
			Labels:    labels,
			Unit:      "seconds",
		})

		metrics = append(metrics, MetricData{
			Name:      "node_memory_MemAvailable_bytes",
			Type:      MetricTypeGauge,
			Value:     float64(1_000_000_000 - (i * 50_000_000)), // Declining available memory
			Timestamp: timestamp,
			Labels:    labels,
			Unit:      "bytes",
		})
	}

	return metrics
}
