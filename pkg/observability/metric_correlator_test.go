package observability

import (
	"context"
	"testing"

	kktesting "github.com/aslakknutsen/kkbase/pkg/testing"
	"go.uber.org/zap"
)

func TestMetricCorrelator_FindResourceFromLabels_Pod(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	mockStore := kktesting.NewMockGraphStore()

	// Setup mock to return pod exists
	mockStore.SetQueryResult([]map[string]interface{}{
		{"exists": true},
	})

	correlator := NewMetricCorrelator(mockStore, logger)

	labels := map[string]string{
		"pod":       "test-pod",
		"namespace": "default",
	}

	resourceType, resourceID := correlator.FindResourceFromLabels(ctx, labels)

	if resourceType != "Pod" {
		t.Errorf("Expected resource type 'Pod', got %s", resourceType)
	}
	if resourceID != "Pod/default/test-pod" {
		t.Errorf("Expected resource ID 'Pod/default/test-pod', got %s", resourceID)
	}
}

func TestMetricCorrelator_FindResourceFromLabels_Container(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	mockStore := kktesting.NewMockGraphStore()

	// Setup mock to return container exists
	mockStore.SetQueryResult([]map[string]interface{}{
		{"exists": true},
	})

	correlator := NewMetricCorrelator(mockStore, logger)

	labels := map[string]string{
		"pod":       "test-pod",
		"namespace": "default",
		"container": "app-container",
	}

	resourceType, resourceID := correlator.FindResourceFromLabels(ctx, labels)

	if resourceType != "Container" {
		t.Errorf("Expected resource type 'Container', got %s", resourceType)
	}
	expectedID := "Container/default/test-pod/app-container"
	if resourceID != expectedID {
		t.Errorf("Expected resource ID '%s', got %s", expectedID, resourceID)
	}
}

func TestMetricCorrelator_FindResourceFromLabels_Service(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	mockStore := kktesting.NewMockGraphStore()

	// Setup mock to return service exists
	mockStore.SetQueryResult([]map[string]interface{}{
		{"exists": true},
	})

	correlator := NewMetricCorrelator(mockStore, logger)

	labels := map[string]string{
		"service":   "api-service",
		"namespace": "prod",
	}

	resourceType, resourceID := correlator.FindResourceFromLabels(ctx, labels)

	if resourceType != "Service" {
		t.Errorf("Expected resource type 'Service', got %s", resourceType)
	}
	if resourceID != "Service/prod/api-service" {
		t.Errorf("Expected resource ID 'Service/prod/api-service', got %s", resourceID)
	}
}

func TestMetricCorrelator_FindResourceFromLabels_Node(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	mockStore := kktesting.NewMockGraphStore()

	// Setup mock to return node exists
	mockStore.SetQueryResult([]map[string]interface{}{
		{"exists": true},
	})

	correlator := NewMetricCorrelator(mockStore, logger)

	labels := map[string]string{
		"node": "worker-node-1",
	}

	resourceType, resourceID := correlator.FindResourceFromLabels(ctx, labels)

	if resourceType != "Node" {
		t.Errorf("Expected resource type 'Node', got %s", resourceType)
	}
	if resourceID != "Node/worker-node-1" {
		t.Errorf("Expected resource ID 'Node/worker-node-1', got %s", resourceID)
	}
}

func TestMetricCorrelator_FindResourceFromLabels_InstanceLabel(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	mockStore := kktesting.NewMockGraphStore()

	// Setup mock to return node exists
	mockStore.SetQueryResult([]map[string]interface{}{
		{"exists": true},
	})

	correlator := NewMetricCorrelator(mockStore, logger)

	// Test with instance label (common in node exporter metrics)
	labels := map[string]string{
		"instance": "worker-node-1:9100",
	}

	resourceType, resourceID := correlator.FindResourceFromLabels(ctx, labels)

	if resourceType != "Node" {
		t.Errorf("Expected resource type 'Node', got %s", resourceType)
	}
	// Should strip port from instance label
	if resourceID != "Node/worker-node-1" {
		t.Errorf("Expected resource ID 'Node/worker-node-1', got %s", resourceID)
	}
}

func TestMetricCorrelator_FindResourceFromLabels_PODContainer(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	mockStore := kktesting.NewMockGraphStore()

	// POD container should be filtered out (empty string), so should fall back to pod
	// Setup mock to return pod exists
	mockStore.SetQueryResult([]map[string]interface{}{
		{"exists": true},
	})

	correlator := NewMetricCorrelator(mockStore, logger)

	labels := map[string]string{
		"pod":       "test-pod",
		"namespace": "default",
		"container": "POD", // Infrastructure container
	}

	resourceType, _ := correlator.FindResourceFromLabels(ctx, labels)

	// Should fall back to Pod since POD container is filtered out
	if resourceType != "Pod" {
		t.Errorf("Expected resource type 'Pod', got %s", resourceType)
	}
}

func TestMetricCorrelator_FindResourceFromLabels_NoMatch(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	mockStore := kktesting.NewMockGraphStore()

	// Setup mock to return no resource exists
	mockStore.SetQueryResult([]map[string]interface{}{
		{"exists": false},
	})

	correlator := NewMetricCorrelator(mockStore, logger)

	labels := map[string]string{
		"pod":       "nonexistent-pod",
		"namespace": "default",
	}

	resourceType, resourceID := correlator.FindResourceFromLabels(ctx, labels)

	if resourceType != "" {
		t.Errorf("Expected empty resource type, got %s", resourceType)
	}
	if resourceID != "" {
		t.Errorf("Expected empty resource ID, got %s", resourceID)
	}
}

func TestMetricCorrelator_FindResourceFromLabels_EmptyLabels(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	mockStore := kktesting.NewMockGraphStore()

	correlator := NewMetricCorrelator(mockStore, logger)

	labels := map[string]string{}

	resourceType, resourceID := correlator.FindResourceFromLabels(ctx, labels)

	if resourceType != "" {
		t.Errorf("Expected empty resource type, got %s", resourceType)
	}
	if resourceID != "" {
		t.Errorf("Expected empty resource ID, got %s", resourceID)
	}
}

func TestMetricCorrelator_FindResourceFromLabels_Priority(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	mockStore := kktesting.NewMockGraphStore()

	// Setup mock to return true for all queries
	mockStore.SetQueryResult([]map[string]interface{}{
		{"exists": true},
	})

	correlator := NewMetricCorrelator(mockStore, logger)

	// Labels that could match multiple resource types
	labels := map[string]string{
		"pod":       "test-pod",
		"namespace": "default",
		"container": "app",
		"service":   "test-service",
	}

	resourceType, _ := correlator.FindResourceFromLabels(ctx, labels)

	// Should prioritize Container (most specific)
	if resourceType != "Container" {
		t.Errorf("Expected resource type 'Container' (highest priority), got %s", resourceType)
	}
}
