package jaeger

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/aslakknutsen/kkbase/pkg/observability"
	"go.uber.org/zap"
)

// Provider implements TracesProvider for Jaeger Query API
type Provider struct {
	baseURL    string
	httpClient *http.Client
	logger     *zap.Logger
}

// NewProvider creates a new Jaeger traces provider
func NewProvider(baseURL string, logger *zap.Logger) *Provider {
	return &Provider{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

// GetTraces retrieves traces for specific services in a time range
func (p *Provider) GetTraces(ctx context.Context, services []string, startTime, endTime time.Time) ([]observability.Trace, error) {
	var allTraces []observability.Trace

	// If no services specified, we can't query efficiently
	if len(services) == 0 {
		p.logger.Debug("no services specified for trace query")
		return allTraces, nil
	}

	// Query each service
	for _, service := range services {
		traces, err := p.queryServiceTraces(ctx, service, startTime, endTime)
		if err != nil {
			p.logger.Warn("failed to query traces for service",
				zap.String("service", service),
				zap.Error(err))
			continue
		}
		allTraces = append(allTraces, traces...)
	}

	return allTraces, nil
}

// queryServiceTraces queries Jaeger API for a specific service
func (p *Provider) queryServiceTraces(ctx context.Context, service string, startTime, endTime time.Time) ([]observability.Trace, error) {
	// Build query URL
	params := url.Values{}
	params.Set("service", service)
	params.Set("start", fmt.Sprintf("%d", startTime.UnixMicro()))
	params.Set("end", fmt.Sprintf("%d", endTime.UnixMicro()))
	params.Set("limit", "100") // Adjustable

	queryURL := fmt.Sprintf("%s/api/traces?%s", p.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", queryURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to query Jaeger: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Jaeger API error: %d - %s", resp.StatusCode, string(body))
	}

	// Parse response
	var jaegerResp JaegerTracesResponse
	if err := json.NewDecoder(resp.Body).Decode(&jaegerResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert to our model
	traces := make([]observability.Trace, 0, len(jaegerResp.Data))
	for _, jTrace := range jaegerResp.Data {
		trace := p.convertTrace(jTrace)
		traces = append(traces, trace)
	}

	p.logger.Debug("retrieved traces",
		zap.String("service", service),
		zap.Int("count", len(traces)))

	return traces, nil
}

// GetTraceByID retrieves a specific trace by ID
func (p *Provider) GetTraceByID(ctx context.Context, traceID string) (*observability.Trace, error) {
	queryURL := fmt.Sprintf("%s/api/traces/%s", p.baseURL, traceID)

	req, err := http.NewRequestWithContext(ctx, "GET", queryURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to query Jaeger: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("trace not found: %s", traceID)
	}

	var jaegerResp JaegerTracesResponse
	if err := json.NewDecoder(resp.Body).Decode(&jaegerResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(jaegerResp.Data) == 0 {
		return nil, fmt.Errorf("trace not found: %s", traceID)
	}

	trace := p.convertTrace(jaegerResp.Data[0])
	return &trace, nil
}

// StreamTraces streams traces by polling at intervals
func (p *Provider) StreamTraces(ctx context.Context, services []string, pollInterval time.Duration) (<-chan observability.Trace, error) {
	traceChan := make(chan observability.Trace, 100)

	go func() {
		defer close(traceChan)

		ticker := time.NewTicker(pollInterval)
		defer ticker.Stop()

		// Start from lookback window
		lastPoll := time.Now().Add(-5 * time.Minute)

		for {
			select {
			case <-ctx.Done():
				p.logger.Info("stopping trace stream")
				return

			case <-ticker.C:
				now := time.Now()

				traces, err := p.GetTraces(ctx, services, lastPoll, now)
				if err != nil {
					p.logger.Error("failed to poll traces", zap.Error(err))
					continue
				}

				p.logger.Debug("polled traces",
					zap.Int("count", len(traces)),
					zap.Time("start", lastPoll),
					zap.Time("end", now))

				for _, trace := range traces {
					select {
					case traceChan <- trace:
					case <-ctx.Done():
						return
					}
				}

				lastPoll = now
			}
		}
	}()

	return traceChan, nil
}

// convertTrace converts Jaeger trace format to our model
func (p *Provider) convertTrace(jTrace JaegerTrace) observability.Trace {
	spans := make([]observability.TraceSpan, 0, len(jTrace.Spans))

	var minStart int64 = 0
	var maxEnd int64 = 0
	errorCount := 0
	servicesMap := make(map[string]bool)
	namespacesMap := make(map[string]bool)

	for _, jSpan := range jTrace.Spans {
		span := p.convertSpan(jSpan, jTrace.Processes)
		spans = append(spans, span)

		// Track metrics
		if minStart == 0 || jSpan.StartTime < minStart {
			minStart = jSpan.StartTime
		}
		endTime := jSpan.StartTime + jSpan.Duration
		if endTime > maxEnd {
			maxEnd = endTime
		}

		if span.Error {
			errorCount++
		}

		servicesMap[span.Service] = true
		if span.Namespace != "" {
			namespacesMap[span.Namespace] = true
		}
	}

	// Find root span
	rootSpan := spans[0]
	for _, span := range spans {
		if span.ParentID == "" {
			rootSpan = span
			break
		}
	}

	// Build unique lists
	services := make([]string, 0, len(servicesMap))
	for svc := range servicesMap {
		services = append(services, svc)
	}
	namespaces := make([]string, 0, len(namespacesMap))
	for ns := range namespacesMap {
		namespaces = append(namespaces, ns)
	}

	return observability.Trace{
		TraceID:       jTrace.TraceID,
		Spans:         spans,
		StartTime:     time.UnixMicro(minStart),
		Duration:      time.Duration(maxEnd-minStart) * time.Microsecond,
		RootService:   rootSpan.Service,
		RootOperation: rootSpan.OperationName,
		SpanCount:     len(spans),
		ErrorCount:    errorCount,
		HasErrors:     errorCount > 0,
		Services:      services,
		Namespaces:    namespaces,
	}
}

// convertSpan converts a Jaeger span to our model using OpenTelemetry 1.21+ conventions
func (p *Provider) convertSpan(jSpan JaegerSpan, processes map[string]JaegerProcess) observability.TraceSpan {
	span := observability.TraceSpan{
		TraceID:       jSpan.TraceID,
		SpanID:        jSpan.SpanID,
		OperationName: jSpan.OperationName,
		StartTime:     time.UnixMicro(jSpan.StartTime),
		Duration:      time.Duration(jSpan.Duration) * time.Microsecond,
		Tags:          make(map[string]string),
		ProcessID:     jSpan.ProcessID,
	}

	// Get process info (service name, namespace, K8s metadata)
	if proc, ok := processes[jSpan.ProcessID]; ok {
		span.Service = proc.ServiceName
		for _, tag := range proc.Tags {
			switch tag.Key {
			case "service.namespace", "k8s.namespace.name", "namespace":
				span.Namespace = tag.ValueString()
			case "otel.library.name":
				span.LibraryName = tag.ValueString()
			case "k8s.pod.name":
				span.K8sPodName = tag.ValueString()
			case "k8s.node.name":
				span.K8sNodeName = tag.ValueString()
			case "service.instance.id":
				span.ServiceInstanceID = tag.ValueString()
			case "service.version":
				span.ServiceVersion = tag.ValueString()
			}
		}
	}

	// Parse references to find parent
	for _, ref := range jSpan.References {
		if ref.RefType == "CHILD_OF" {
			span.ParentID = ref.SpanID
		}
		span.References = append(span.References, observability.SpanReference{
			RefType: ref.RefType,
			TraceID: ref.TraceID,
			SpanID:  ref.SpanID,
		})
	}

	// Parse tags using OpenTelemetry 1.21+ semantic conventions
	for _, tag := range jSpan.Tags {
		tagValue := tag.ValueString()
		span.Tags[tag.Key] = tagValue

		// Extract fields based on OTel conventions
		switch tag.Key {
		// Span metadata
		case "span.kind":
			span.SpanKind = tagValue
		case "otel.status_code":
			span.Status = tagValue
		case "error":
			span.Error = (tagValue == "true")
		case "error.type":
			span.ErrorType = tagValue
			if tagValue != "" {
				span.Error = true
			}
		case "otel.status_description":
			span.ErrorMessage = tagValue

		// Namespace fallback
		case "service.namespace", "k8s.namespace.name", "namespace":
			if span.Namespace == "" {
				span.Namespace = tagValue
			}

		// HTTP attributes (OpenTelemetry 1.21+ conventions)
		case "http.request.method_original", "http.request.method":
			span.HTTPRequestMethod = tagValue
		case "http.response.status_code":
			if tag.Type == "int64" {
				if statusCode, ok := tag.Value.(float64); ok {
					span.HTTPResponseStatusCode = int(statusCode)
				}
			}
		case "url.path":
			span.URLPath = tagValue
		case "url.scheme":
			span.URLScheme = tagValue
		case "url.full":
			span.URLFull = tagValue

		// Network attributes
		case "network.protocol.name":
			span.NetworkProtocolName = tagValue
		case "network.protocol.version":
			span.NetworkProtocolVersion = tagValue
		case "network.transport":
			span.NetworkTransport = tagValue

		// Server/Client addressing
		case "server.address":
			span.ServerAddress = tagValue
		case "server.port":
			if tag.Type == "int64" {
				if port, ok := tag.Value.(float64); ok {
					span.ServerPort = int(port)
				}
			}
		case "client.address":
			span.ClientAddress = tagValue

		// RPC attributes
		case "rpc.system":
			span.RPCSystem = tagValue
		case "rpc.service":
			span.RPCService = tagValue
		case "rpc.method":
			span.RPCMethod = tagValue
		case "rpc.grpc.status_code":
			if tag.Type == "int64" {
				if statusCode, ok := tag.Value.(float64); ok {
					span.RPCGRPCStatusCode = int(statusCode)
				}
			}

		// User agent
		case "user_agent.original":
			span.UserAgent = tagValue
		}
	}

	// Derive Protocol field for backward compatibility
	if span.NetworkProtocolName != "" {
		span.Protocol = span.NetworkProtocolName
	} else if span.RPCSystem != "" {
		span.Protocol = span.RPCSystem
	} else if span.HTTPRequestMethod != "" {
		span.Protocol = "http"
	}

	// Log warning if namespace is missing (common issue)
	if span.Namespace == "" && span.Service != "" {
		p.logger.Debug("span missing namespace information",
			zap.String("service", span.Service),
			zap.String("span_id", span.SpanID),
			zap.String("operation", span.OperationName))
	}

	return span
}

// Close closes the provider
func (p *Provider) Close() error {
	p.httpClient.CloseIdleConnections()
	return nil
}
