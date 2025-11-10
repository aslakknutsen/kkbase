package observability

import (
	"context"
	"testing"
	"time"

	kktesting "github.com/aslakknutsen/kkbase/pkg/testing"
	"go.uber.org/zap"
)

// testMockProvider is a simple mock provider for testing (avoids import cycle)
type testMockProvider struct {
	metrics      []MetricData
	queryHistory []MetricQuerySpec
}

func newTestMockProvider(metrics []MetricData) *testMockProvider {
	return &testMockProvider{
		metrics:      metrics,
		queryHistory: []MetricQuerySpec{},
	}
}

func (m *testMockProvider) GetMetrics(ctx context.Context, resourceType, resourceID string, startTime, endTime time.Time) ([]MetricData, error) {
	var result []MetricData
	for _, metric := range m.metrics {
		if (metric.Timestamp.Equal(startTime) || metric.Timestamp.After(startTime)) &&
			(metric.Timestamp.Equal(endTime) || metric.Timestamp.Before(endTime)) {
			result = append(result, metric)
		}
	}
	return result, nil
}

func (m *testMockProvider) QueryMetrics(ctx context.Context, spec MetricQuerySpec) ([]MetricData, error) {
	m.queryHistory = append(m.queryHistory, spec)
	var result []MetricData
	for _, metric := range m.metrics {
		if (metric.Timestamp.Equal(spec.StartTime) || metric.Timestamp.After(spec.StartTime)) &&
			(metric.Timestamp.Equal(spec.EndTime) || metric.Timestamp.Before(spec.EndTime)) {
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

func (m *testMockProvider) StreamMetrics(ctx context.Context, resourceType, resourceID string) (<-chan MetricData, error) {
	ch := make(chan MetricData)
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

func (m *testMockProvider) Close() error {
	return nil
}

// generateTestMetrics creates sample metrics for testing
func generateTestMetrics() []MetricData {
	baseTime := time.Now().Add(-15 * time.Minute)
	labels := map[string]string{
		"pod":       "test-pod",
		"namespace": "default",
		"container": "app",
	}

	var metrics []MetricData
	for i := 0; i < 5; i++ {
		metrics = append(metrics, MetricData{
			Name:      "container_memory_usage_bytes",
			Type:      MetricTypeGauge,
			Value:     float64(500000000 + i*10000000),
			Timestamp: baseTime.Add(time.Duration(i) * time.Minute),
			Labels:    labels,
			Unit:      "bytes",
		})
	}
	return metrics
}

func TestInvestigationMetricsProcessor_StartInvestigation(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	mockStore := kktesting.NewMockGraphStore()
	mockProvider := newTestMockProvider(generateTestMetrics())

	processor := NewInvestigationMetricsProcessor(mockStore, mockProvider, logger)

	session, err := processor.StartInvestigation(
		ctx,
		"Pod",
		"Pod/default/test-pod",
		"CrashLoopBackOff",
		15*time.Minute,
	)

	if err != nil {
		t.Fatalf("Failed to start investigation: %v", err)
	}

	if session == nil {
		t.Fatal("Expected session, got nil")
	}

	if session.Status != "active" {
		t.Errorf("Expected status 'active', got %s", session.Status)
	}

	if session.ResourceType != "Pod" {
		t.Errorf("Expected resource type 'Pod', got %s", session.ResourceType)
	}

	if session.Symptom != "CrashLoopBackOff" {
		t.Errorf("Expected symptom 'CrashLoopBackOff', got %s", session.Symptom)
	}

	// Verify Investigation node was created
	if len(mockStore.Nodes) == 0 {
		t.Error("Expected Investigation node to be created")
	}
}

func TestInvestigationMetricsProcessor_SelectMetricsForSymptom(t *testing.T) {
	logger := zap.NewNop()
	mockStore := kktesting.NewMockGraphStore()
	processor := NewInvestigationMetricsProcessor(mockStore, nil, logger)

	tests := []struct {
		name               string
		symptom            string
		resourceType       string
		expectedMetrics    []string
		expectAtLeastCount int
	}{
		{
			name:               "OOMKilled symptom",
			symptom:            "OOMKilled",
			resourceType:       "Pod",
			expectedMetrics:    []string{"container_memory_usage_bytes", "container_memory_working_set_bytes"},
			expectAtLeastCount: 2,
		},
		{
			name:               "HighLatency symptom",
			symptom:            "HighLatency",
			resourceType:       "Service",
			expectedMetrics:    []string{"http_request_duration_seconds"},
			expectAtLeastCount: 1,
		},
		{
			name:               "HighErrorRate symptom",
			symptom:            "HighErrorRate",
			resourceType:       "Pod",
			expectedMetrics:    []string{"http_requests_total"},
			expectAtLeastCount: 1,
		},
		{
			name:               "NodeNotReady symptom",
			symptom:            "NodeNotReady",
			resourceType:       "Node",
			expectedMetrics:    []string{"node_cpu_seconds_total", "node_memory_MemAvailable_bytes"},
			expectAtLeastCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metrics := processor.selectMetricsForSymptom(tt.symptom, tt.resourceType)

			if len(metrics) < tt.expectAtLeastCount {
				t.Errorf("Expected at least %d metrics, got %d", tt.expectAtLeastCount, len(metrics))
			}

			// Check that expected metrics are present
			for _, expected := range tt.expectedMetrics {
				found := false
				for _, metric := range metrics {
					if metric == expected {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected metric '%s' not found in results: %v", expected, metrics)
				}
			}
		})
	}
}

func TestInvestigationMetricsProcessor_CompleteInvestigation(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	mockStore := kktesting.NewMockGraphStore()
	mockProvider := newTestMockProvider(generateTestMetrics())

	// Setup mock to return deleted count
	mockStore.SetQueryResult([]map[string]interface{}{
		{"deleted_count": int64(15)},
	})

	processor := NewInvestigationMetricsProcessor(mockStore, mockProvider, logger)

	// Start investigation first
	session, err := processor.StartInvestigation(
		ctx,
		"Pod",
		"Pod/default/test-pod",
		"CrashLoopBackOff",
		15*time.Minute,
	)
	if err != nil {
		t.Fatalf("Failed to start investigation: %v", err)
	}

	// Complete the investigation
	err = processor.CompleteInvestigation(ctx, session.ID)
	if err != nil {
		t.Fatalf("Failed to complete investigation: %v", err)
	}

	// Verify investigation completed without error (mockStore doesn't track query history)
}

func TestInvestigationMetricsProcessor_AbandonInvestigation(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	mockStore := kktesting.NewMockGraphStore()
	mockProvider := newTestMockProvider(generateTestMetrics())

	// Setup mock to return deleted count
	mockStore.SetQueryResult([]map[string]interface{}{
		{"deleted_count": int64(10)},
	})

	processor := NewInvestigationMetricsProcessor(mockStore, mockProvider, logger)

	// Start investigation first
	session, err := processor.StartInvestigation(
		ctx,
		"Pod",
		"Pod/default/test-pod",
		"HighLatency",
		15*time.Minute,
	)
	if err != nil {
		t.Fatalf("Failed to start investigation: %v", err)
	}

	// Abandon the investigation
	err = processor.AbandonInvestigation(ctx, session.ID)
	if err != nil {
		t.Fatalf("Failed to abandon investigation: %v", err)
	}

	// Verify investigation abandoned without error (mockStore doesn't track query history)
}

func TestInvestigationMetricsProcessor_WithoutMetricsProvider(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	mockStore := kktesting.NewMockGraphStore()

	// Create processor without metrics provider
	processor := NewInvestigationMetricsProcessor(mockStore, nil, logger)

	session, err := processor.StartInvestigation(
		ctx,
		"Pod",
		"Pod/default/test-pod",
		"CrashLoopBackOff",
		15*time.Minute,
	)

	// Should not fail even without provider
	if err != nil {
		t.Fatalf("Investigation should not fail without metrics provider: %v", err)
	}

	if session == nil {
		t.Fatal("Expected session, got nil")
	}

	if session.Status != "active" {
		t.Errorf("Expected status 'active', got %s", session.Status)
	}
}

func TestInvestigationMetricsProcessor_MetricStorage(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	mockStore := kktesting.NewMockGraphStore()
	mockProvider := newTestMockProvider(generateTestMetrics())

	processor := NewInvestigationMetricsProcessor(mockStore, mockProvider, logger)

	_, err := processor.StartInvestigation(
		ctx,
		"Pod",
		"Pod/default/test-pod",
		"OOMKilled",
		15*time.Minute,
	)

	if err != nil {
		t.Fatalf("Failed to start investigation: %v", err)
	}

	// Verify that metrics were pulled and stored
	history := mockProvider.queryHistory
	if len(history) == 0 {
		t.Error("Expected at least one query to metrics provider")
	}

	// Verify metrics query spec
	if len(history) > 0 {
		spec := history[0]
		if spec.ResourceType != "Pod" {
			t.Errorf("Expected resource type 'Pod', got %s", spec.ResourceType)
		}
		if spec.ResourceID != "Pod/default/test-pod" {
			t.Errorf("Expected resource ID 'Pod/default/test-pod', got %s", spec.ResourceID)
		}
		if len(spec.MetricNames) == 0 {
			t.Error("Expected metric names in spec")
		}
	}
}

func TestGenerateInvestigationID(t *testing.T) {
	id1 := generateInvestigationID()
	time.Sleep(1 * time.Millisecond)
	id2 := generateInvestigationID()

	if id1 == id2 {
		t.Error("Expected unique investigation IDs")
	}

	if id1 == "" {
		t.Error("Expected non-empty investigation ID")
	}
}
