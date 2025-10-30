package observability

import (
	"testing"
)

func TestMetricCatalog_Registration(t *testing.T) {
	catalog := NewMetricCatalog()

	// Test that core metrics are registered
	coreMetrics := catalog.GetAllMetrics()
	if len(coreMetrics) == 0 {
		t.Error("Expected core metrics to be registered, got none")
	}

	// Test registering a custom metric
	customMetric := MetricDefinition{
		Name:              "custom_metric_total",
		Type:              MetricTypeCounter,
		Description:       "Custom test metric",
		Unit:              "count",
		PromQLTemplate:    `custom_metric_total{namespace="{{.Namespace}}"}`,
		ApplicableToTypes: []string{"Pod"},
		Category:          "custom",
	}

	catalog.Register(customMetric)

	// Verify it was added
	retrieved, exists := catalog.Get("custom_metric_total")
	if !exists {
		t.Error("Expected custom metric to be registered")
	}
	if retrieved.Name != "custom_metric_total" {
		t.Errorf("Expected metric name 'custom_metric_total', got %s", retrieved.Name)
	}
}

func TestMetricCatalog_GetByResourceType(t *testing.T) {
	catalog := NewMetricCatalog()

	tests := []struct {
		name         string
		resourceType string
		expectCount  int
	}{
		{
			name:         "Pod metrics",
			resourceType: "Pod",
			expectCount:  6, // Based on registered core metrics
		},
		{
			name:         "Node metrics",
			resourceType: "Node",
			expectCount:  3,
		},
		{
			name:         "Container metrics",
			resourceType: "Container",
			expectCount:  3,
		},
		{
			name:         "NonExistent type",
			resourceType: "NonExistent",
			expectCount:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metrics := catalog.GetMetricsForResourceType(tt.resourceType)
			if len(metrics) < tt.expectCount {
				t.Errorf("Expected at least %d metrics for %s, got %d",
					tt.expectCount, tt.resourceType, len(metrics))
			}
		})
	}
}

func TestMetricCatalog_GetByCategory(t *testing.T) {
	catalog := NewMetricCatalog()

	tests := []struct {
		name        string
		category    string
		expectCount int
	}{
		{
			name:        "Resource metrics",
			category:    "resource",
			expectCount: 7,
		},
		{
			name:        "Performance metrics",
			category:    "performance",
			expectCount: 2,
		},
		{
			name:        "Error metrics",
			category:    "error",
			expectCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metrics := catalog.GetMetricsByCategory(tt.category)
			if len(metrics) < tt.expectCount {
				t.Errorf("Expected at least %d metrics for category %s, got %d",
					tt.expectCount, tt.category, len(metrics))
			}
		})
	}
}

func TestMetricCatalog_Get(t *testing.T) {
	catalog := NewMetricCatalog()

	tests := []struct {
		name         string
		metricName   string
		shouldExist  bool
		expectedType MetricType
	}{
		{
			name:         "Existing gauge metric",
			metricName:   "container_memory_usage_bytes",
			shouldExist:  true,
			expectedType: MetricTypeGauge,
		},
		{
			name:         "Existing counter metric",
			metricName:   "container_cpu_usage_seconds_total",
			shouldExist:  true,
			expectedType: MetricTypeCounter,
		},
		{
			name:         "Non-existing metric",
			metricName:   "does_not_exist",
			shouldExist:  false,
			expectedType: MetricTypeUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metric, exists := catalog.Get(tt.metricName)
			if exists != tt.shouldExist {
				t.Errorf("Expected exists=%v for %s, got %v",
					tt.shouldExist, tt.metricName, exists)
			}
			if exists && metric.Type != tt.expectedType {
				t.Errorf("Expected type %s, got %s", tt.expectedType, metric.Type)
			}
		})
	}
}

func TestMetricCatalog_GetMetricNames(t *testing.T) {
	catalog := NewMetricCatalog()

	names := catalog.GetMetricNames()
	if len(names) == 0 {
		t.Error("Expected metric names, got none")
	}

	// Check for expected metrics
	expectedMetrics := []string{
		"container_memory_usage_bytes",
		"container_cpu_usage_seconds_total",
		"node_cpu_seconds_total",
	}

	for _, expected := range expectedMetrics {
		found := false
		for _, name := range names {
			if name == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected metric %s not found in names", expected)
		}
	}
}

func TestMetricCatalog_CoreMetricsProperties(t *testing.T) {
	catalog := NewMetricCatalog()

	// Test that all core metrics have required properties
	allMetrics := catalog.GetAllMetrics()

	for _, metric := range allMetrics {
		if metric.Name == "" {
			t.Error("Metric with empty name found")
		}
		if metric.Type == "" {
			t.Errorf("Metric %s has empty type", metric.Name)
		}
		if metric.Description == "" {
			t.Errorf("Metric %s has empty description", metric.Name)
		}
		if len(metric.ApplicableToTypes) == 0 {
			t.Errorf("Metric %s has no applicable types", metric.Name)
		}
		if metric.Category == "" {
			t.Errorf("Metric %s has empty category", metric.Name)
		}
	}
}
