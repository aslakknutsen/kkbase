package mcp

import (
	"context"
	"testing"
	"time"

	"github.com/kagenti/kkbase/pkg/observability"
	kktesting "github.com/kagenti/kkbase/pkg/testing"
	"go.uber.org/zap/zaptest"
)

func TestNewServer_WithoutMetricsProcessor(t *testing.T) {
	// Setup
	mockStore := kktesting.NewMockGraphStore()
	logger := zaptest.NewLogger(t)

	// Create server without metrics processor
	server, err := NewServer(mockStore, logger)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer server.Close()

	// Verify server was created
	if server == nil {
		t.Fatal("Expected server to be created")
	}

	// Verify basic fields
	if server.store != mockStore {
		t.Error("Expected store to be set correctly")
	}
	if server.logger != logger {
		t.Error("Expected logger to be set correctly")
	}
	if server.metricsProcessor != nil {
		t.Error("Expected metricsProcessor to be nil")
	}
}

func TestNewServer_WithMetricsProcessor(t *testing.T) {
	// Setup
	mockStore := kktesting.NewMockGraphStore()
	logger := zaptest.NewLogger(t)

	// Create a mock metrics processor
	mockProvider := &mockMetricsProvider{}
	metricsProcessor := observability.NewInvestigationMetricsProcessor(
		mockStore,
		mockProvider,
		logger,
	)

	// Create server with metrics processor
	server, err := NewServer(mockStore, logger, WithMetricsProcessor(metricsProcessor))
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer server.Close()

	// Verify server was created with metrics processor
	if server == nil {
		t.Fatal("Expected server to be created")
	}
	if server.metricsProcessor == nil {
		t.Error("Expected metricsProcessor to be set")
	}
}

func TestNewServer_NilStore(t *testing.T) {
	logger := zaptest.NewLogger(t)

	// Should return error with nil store
	_, err := NewServer(nil, logger)
	if err == nil {
		t.Fatal("Expected error when creating server with nil store")
	}
}

func TestNewServer_NilLogger(t *testing.T) {
	mockStore := kktesting.NewMockGraphStore()

	// Should return error with nil logger
	_, err := NewServer(mockStore, nil)
	if err == nil {
		t.Fatal("Expected error when creating server with nil logger")
	}
}

// mockMetricsProvider is a simple mock for testing
type mockMetricsProvider struct{}

func (m *mockMetricsProvider) GetMetrics(ctx context.Context, resourceType, resourceID string, startTime, endTime time.Time) ([]observability.MetricData, error) {
	return []observability.MetricData{}, nil
}

func (m *mockMetricsProvider) QueryMetrics(ctx context.Context, spec observability.MetricQuerySpec) ([]observability.MetricData, error) {
	return []observability.MetricData{}, nil
}

func (m *mockMetricsProvider) StreamMetrics(ctx context.Context, resourceType, resourceID string) (<-chan observability.MetricData, error) {
	ch := make(chan observability.MetricData)
	close(ch)
	return ch, nil
}

func (m *mockMetricsProvider) Close() error {
	return nil
}
