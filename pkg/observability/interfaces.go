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

// TraceSpan represents a single trace span with full Jaeger compatibility
type TraceSpan struct {
	// Core identifiers
	TraceID  string
	SpanID   string
	ParentID string // Primary parent span ID

	// Operation details
	OperationName string
	Service       string
	Namespace     string // Kubernetes namespace

	// Timing
	StartTime time.Time
	Duration  time.Duration

	// Status
	SpanKind     string // internal, client, server, producer, consumer
	Status       string // OK, ERROR
	Error        bool
	ErrorMessage string

	// Protocol/endpoint information
	Protocol   string // http, grpc, kafka, etc.
	HTTPMethod string
	HTTPPath   string
	HTTPStatus int
	RPCService string
	RPCMethod  string

	// Upstream call details (if this span calls another service)
	UpstreamName     string
	UpstreamURL      string
	UpstreamProtocol string

	// Tags (all attributes as key-value)
	Tags map[string]string

	// References (for complex span relationships)
	References []SpanReference

	// Process metadata
	ProcessID   string
	LibraryName string
}

// SpanReference represents a relationship to another span
type SpanReference struct {
	RefType string // CHILD_OF, FOLLOWS_FROM
	TraceID string
	SpanID  string
}

// Trace represents a complete distributed trace
type Trace struct {
	TraceID       string
	Spans         []TraceSpan
	StartTime     time.Time
	Duration      time.Duration
	RootService   string
	RootOperation string
	SpanCount     int
	ErrorCount    int
	HasErrors     bool
	Services      []string // Unique services involved
	Namespaces    []string // Unique namespaces involved
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
	// GetTraces retrieves traces for specific services in a time range
	GetTraces(ctx context.Context, services []string, startTime, endTime time.Time) ([]Trace, error)

	// GetTraceByID retrieves a specific trace by ID
	GetTraceByID(ctx context.Context, traceID string) (*Trace, error)

	// StreamTraces streams traces as they're discovered (polling or real-time)
	StreamTraces(ctx context.Context, services []string, pollInterval time.Duration) (<-chan Trace, error)

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
