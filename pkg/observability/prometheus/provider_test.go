package prometheus

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/kagenti/kkbase/pkg/observability"
	"go.uber.org/zap"
)

func TestProvider_QueryRange_Success(t *testing.T) {
	// Create mock Prometheus server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.URL.Path != "/api/v1/query_range" {
			t.Errorf("Expected path /api/v1/query_range, got %s", r.URL.Path)
		}

		query := r.URL.Query().Get("query")
		if query == "" {
			t.Error("Expected query parameter")
		}

		// Return mock response
		response := PrometheusResponse{
			Status: "success",
			Data: PrometheusData{
				ResultType: "matrix",
				Result: []PrometheusResult{
					{
						Metric: PrometheusMetric{
							"__name__":  "container_memory_usage_bytes",
							"pod":       "test-pod",
							"namespace": "default",
							"container": "app",
						},
						Values: [][]interface{}{
							{float64(time.Now().Unix()), "500000000"},
							{float64(time.Now().Unix() + 60), "550000000"},
						},
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	logger := zap.NewNop()
	provider := NewProvider(server.URL, logger)

	ctx := context.Background()
	query := "container_memory_usage_bytes{pod=\"test-pod\"}"
	start := time.Now().Add(-10 * time.Minute)
	end := time.Now()
	step := 1 * time.Minute

	metrics, err := provider.queryRange(ctx, query, start, end, step)

	if err != nil {
		t.Fatalf("queryRange failed: %v", err)
	}

	if len(metrics) != 2 {
		t.Errorf("Expected 2 metric data points, got %d", len(metrics))
	}

	// Verify metric properties
	if len(metrics) > 0 {
		if metrics[0].Name != "container_memory_usage_bytes" {
			t.Errorf("Expected metric name 'container_memory_usage_bytes', got %s", metrics[0].Name)
		}
		if metrics[0].Labels["pod"] != "test-pod" {
			t.Errorf("Expected pod label 'test-pod', got %s", metrics[0].Labels["pod"])
		}
		if metrics[0].Value != 500000000 {
			t.Errorf("Expected value 500000000, got %f", metrics[0].Value)
		}
	}
}

func TestProvider_QueryRange_Error(t *testing.T) {
	// Create mock Prometheus server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := PrometheusResponse{
			Status:    "error",
			ErrorType: "bad_data",
			Error:     "invalid query",
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	logger := zap.NewNop()
	provider := NewProvider(server.URL, logger)

	ctx := context.Background()
	query := "invalid{query}"
	start := time.Now().Add(-10 * time.Minute)
	end := time.Now()
	step := 1 * time.Minute

	_, err := provider.queryRange(ctx, query, start, end, step)

	if err == nil {
		t.Error("Expected error from invalid query")
	}
}

func TestProvider_QueryRange_HTTPError(t *testing.T) {
	// Create mock Prometheus server that returns HTTP error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	logger := zap.NewNop()
	provider := NewProvider(server.URL, logger)

	ctx := context.Background()
	query := "container_memory_usage_bytes"
	start := time.Now().Add(-10 * time.Minute)
	end := time.Now()
	step := 1 * time.Minute

	_, err := provider.queryRange(ctx, query, start, end, step)

	if err == nil {
		t.Error("Expected error from HTTP 500")
	}
}

func TestProvider_QueryMetrics(t *testing.T) {
	// Create mock Prometheus server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := PrometheusResponse{
			Status: "success",
			Data: PrometheusData{
				ResultType: "matrix",
				Result: []PrometheusResult{
					{
						Metric: PrometheusMetric{
							"__name__":  "container_memory_usage_bytes",
							"pod":       "test-pod",
							"namespace": "default",
						},
						Values: [][]interface{}{
							{float64(time.Now().Unix()), "500000000"},
						},
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	logger := zap.NewNop()
	provider := NewProvider(server.URL, logger)

	ctx := context.Background()
	spec := observability.MetricQuerySpec{
		MetricNames:  []string{"container_memory_usage_bytes"},
		ResourceType: "Pod",
		ResourceID:   "Pod/default/test-pod",
		StartTime:    time.Now().Add(-10 * time.Minute),
		EndTime:      time.Now(),
		StepDuration: 1 * time.Minute,
		Labels: map[string]string{
			"namespace": "default",
			"pod":       "test-pod",
		},
	}

	metrics, err := provider.QueryMetrics(ctx, spec)

	if err != nil {
		t.Fatalf("QueryMetrics failed: %v", err)
	}

	if len(metrics) == 0 {
		t.Error("Expected at least one metric")
	}
}

func TestProvider_GetMetrics_Pod(t *testing.T) {
	// Create mock Prometheus server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := PrometheusResponse{
			Status: "success",
			Data: PrometheusData{
				ResultType: "matrix",
				Result:     []PrometheusResult{},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	logger := zap.NewNop()
	provider := NewProvider(server.URL, logger)

	ctx := context.Background()
	metrics, err := provider.GetMetrics(
		ctx,
		"Pod",
		"Pod/default/test-pod",
		time.Now().Add(-10*time.Minute),
		time.Now(),
	)

	if err != nil {
		t.Fatalf("GetMetrics failed: %v", err)
	}

	// Even with no data, should not error
	if metrics == nil {
		t.Error("Expected metrics array, got nil")
	}
}

func TestProvider_GetMetrics_UnsupportedType(t *testing.T) {
	logger := zap.NewNop()
	provider := NewProvider("http://localhost:9090", logger)

	ctx := context.Background()
	_, err := provider.GetMetrics(
		ctx,
		"UnsupportedType",
		"UnsupportedType/test",
		time.Now().Add(-10*time.Minute),
		time.Now(),
	)

	if err == nil {
		t.Error("Expected error for unsupported resource type")
	}
}

func TestBuildPromQLForResource(t *testing.T) {
	logger := zap.NewNop()
	provider := NewProvider("http://localhost:9090", logger)

	tests := []struct {
		name        string
		metricName  string
		labels      map[string]string
		expectRate  bool
		expectLabel bool
	}{
		{
			name:        "Counter metric with rate",
			metricName:  "container_cpu_usage_seconds_total",
			labels:      map[string]string{"pod": "test-pod"},
			expectRate:  true,
			expectLabel: true,
		},
		{
			name:        "Gauge metric without rate",
			metricName:  "container_memory_usage_bytes",
			labels:      map[string]string{"pod": "test-pod"},
			expectRate:  false,
			expectLabel: true,
		},
		{
			name:        "Metric without labels",
			metricName:  "node_cpu_seconds_total",
			labels:      map[string]string{},
			expectRate:  true,
			expectLabel: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := provider.buildPromQLForResource(tt.metricName, tt.labels)

			if query == "" {
				t.Error("Expected non-empty query")
			}

			if tt.expectRate {
				if len(query) < 5 || query[:5] != "rate(" {
					t.Errorf("Expected query to start with 'rate(', got %s", query)
				}
			}

			if tt.expectLabel {
				found := false
				for k := range tt.labels {
					if len(query) > len(k) {
						for i := 0; i < len(query)-len(k); i++ {
							if query[i:i+len(k)] == k {
								found = true
								break
							}
						}
					}
				}
				if !found {
					t.Errorf("Expected query to contain labels, got %s", query)
				}
			}
		})
	}
}

func TestParseResourceID(t *testing.T) {
	tests := []struct {
		name              string
		resourceID        string
		expectedNamespace string
		expectedName      string
	}{
		{
			name:              "Namespaced resource",
			resourceID:        "Pod/default/test-pod",
			expectedNamespace: "default",
			expectedName:      "test-pod",
		},
		{
			name:              "Cluster-scoped resource",
			resourceID:        "Node/worker-1",
			expectedNamespace: "",
			expectedName:      "worker-1",
		},
		{
			name:              "Malformed ID",
			resourceID:        "invalid",
			expectedNamespace: "",
			expectedName:      "invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			namespace, name := parseResourceID(tt.resourceID)

			if namespace != tt.expectedNamespace {
				t.Errorf("Expected namespace '%s', got '%s'", tt.expectedNamespace, namespace)
			}
			if name != tt.expectedName {
				t.Errorf("Expected name '%s', got '%s'", tt.expectedName, name)
			}
		})
	}
}

func TestExtractMetricNameFromQuery(t *testing.T) {
	tests := []struct {
		name         string
		query        string
		expectedName string
	}{
		{
			name:         "Simple metric",
			query:        "container_memory_usage_bytes",
			expectedName: "container_memory_usage_bytes",
		},
		{
			name:         "Metric with labels",
			query:        "container_memory_usage_bytes{pod=\"test\"}",
			expectedName: "container_memory_usage_bytes",
		},
		{
			name:         "Rate function",
			query:        "rate(container_cpu_usage_seconds_total[1m])",
			expectedName: "container_cpu_usage_seconds_total[1m]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name := extractMetricNameFromQuery(tt.query)

			if name == "" {
				t.Error("Expected non-empty metric name")
			}
		})
	}
}

func TestProvider_Close(t *testing.T) {
	logger := zap.NewNop()
	provider := NewProvider("http://localhost:9090", logger)

	err := provider.Close()
	if err != nil {
		t.Errorf("Close should not error: %v", err)
	}
}

func TestProvider_StreamMetrics_NotImplemented(t *testing.T) {
	logger := zap.NewNop()
	provider := NewProvider("http://localhost:9090", logger)

	ctx := context.Background()
	_, err := provider.StreamMetrics(ctx, "Pod", "Pod/default/test-pod")

	if err == nil {
		t.Error("Expected error for unimplemented StreamMetrics")
	}
}
