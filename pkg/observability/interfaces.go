package observability

import (
	"context"
	"time"
)

// MetricData represents a single metric data point
type MetricData struct {
	Name      string
	Value     float64
	Timestamp time.Time
	Labels    map[string]string
	Source    string // e.g., "pod/namespace/podname/container/containername"
}

// LogEntry represents a single log entry
type LogEntry struct {
	Message   string
	Level     string // INFO, WARN, ERROR, DEBUG
	Timestamp time.Time
	Source    string
	Labels    map[string]string
}

// TraceSpan represents a single trace span
type TraceSpan struct {
	TraceID   string
	SpanID    string
	ParentID  string
	Name      string
	StartTime time.Time
	EndTime   time.Time
	Duration  time.Duration
	Service   string
	Tags      map[string]string
}

// MetricsProvider defines the interface for metrics collection
type MetricsProvider interface {
	// GetMetrics retrieves metrics for a specific resource
	GetMetrics(ctx context.Context, resourceType, resourceID string, startTime, endTime time.Time) ([]MetricData, error)

	// StreamMetrics streams real-time metrics
	StreamMetrics(ctx context.Context, resourceType, resourceID string) (<-chan MetricData, error)

	// Close closes the metrics provider connection
	Close() error
}

// LogsProvider defines the interface for log collection
type LogsProvider interface {
	// GetLogs retrieves logs for a specific resource
	GetLogs(ctx context.Context, resourceType, resourceID string, startTime, endTime time.Time) ([]LogEntry, error)

	// StreamLogs streams real-time logs
	StreamLogs(ctx context.Context, resourceType, resourceID string) (<-chan LogEntry, error)

	// QueryLogs executes a query against the log system
	QueryLogs(ctx context.Context, query string) ([]LogEntry, error)

	// Close closes the logs provider connection
	Close() error
}

// TracesProvider defines the interface for distributed tracing
type TracesProvider interface {
	// GetTraces retrieves traces for a specific service or operation
	GetTraces(ctx context.Context, service string, startTime, endTime time.Time) ([]TraceSpan, error)

	// GetTraceByID retrieves a specific trace by ID
	GetTraceByID(ctx context.Context, traceID string) ([]TraceSpan, error)

	// Close closes the traces provider connection
	Close() error
}

// Registry manages observability providers
type Registry struct {
	metricsProvider MetricsProvider
	logsProvider    LogsProvider
	tracesProvider  TracesProvider
}

// NewRegistry creates a new observability registry
func NewRegistry() *Registry {
	return &Registry{}
}

// RegisterMetricsProvider registers a metrics provider
func (r *Registry) RegisterMetricsProvider(provider MetricsProvider) {
	r.metricsProvider = provider
}

// RegisterLogsProvider registers a logs provider
func (r *Registry) RegisterLogsProvider(provider LogsProvider) {
	r.logsProvider = provider
}

// RegisterTracesProvider registers a traces provider
func (r *Registry) RegisterTracesProvider(provider TracesProvider) {
	r.tracesProvider = provider
}

// GetMetricsProvider returns the registered metrics provider
func (r *Registry) GetMetricsProvider() MetricsProvider {
	return r.metricsProvider
}

// GetLogsProvider returns the registered logs provider
func (r *Registry) GetLogsProvider() LogsProvider {
	return r.logsProvider
}

// GetTracesProvider returns the registered traces provider
func (r *Registry) GetTracesProvider() TracesProvider {
	return r.tracesProvider
}

// Close closes all registered providers
func (r *Registry) Close() error {
	if r.metricsProvider != nil {
		if err := r.metricsProvider.Close(); err != nil {
			return err
		}
	}
	if r.logsProvider != nil {
		if err := r.logsProvider.Close(); err != nil {
			return err
		}
	}
	if r.tracesProvider != nil {
		if err := r.tracesProvider.Close(); err != nil {
			return err
		}
	}
	return nil
}
