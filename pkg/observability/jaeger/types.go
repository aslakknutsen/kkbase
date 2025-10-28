package jaeger

import "fmt"

// JaegerTracesResponse is the top-level response from /api/traces
type JaegerTracesResponse struct {
	Data   []JaegerTrace `json:"data"`
	Total  int           `json:"total"`
	Limit  int           `json:"limit"`
	Offset int           `json:"offset"`
	Errors []string      `json:"errors"`
}

// JaegerTrace represents a single trace
type JaegerTrace struct {
	TraceID   string                   `json:"traceID"`
	Spans     []JaegerSpan             `json:"spans"`
	Processes map[string]JaegerProcess `json:"processes"`
	Warnings  []string                 `json:"warnings"`
}

// JaegerSpan represents a single span
type JaegerSpan struct {
	TraceID       string            `json:"traceID"`
	SpanID        string            `json:"spanID"`
	OperationName string            `json:"operationName"`
	References    []JaegerReference `json:"references"`
	StartTime     int64             `json:"startTime"` // microseconds
	Duration      int64             `json:"duration"`  // microseconds
	Tags          []JaegerTag       `json:"tags"`
	Logs          []JaegerLog       `json:"logs"`
	ProcessID     string            `json:"processID"`
	Warnings      []string          `json:"warnings"`
}

// JaegerReference represents a span reference
type JaegerReference struct {
	RefType string `json:"refType"` // CHILD_OF, FOLLOWS_FROM
	TraceID string `json:"traceID"`
	SpanID  string `json:"spanID"`
}

// JaegerTag represents a span tag/attribute
type JaegerTag struct {
	Key   string      `json:"key"`
	Type  string      `json:"type"` // string, int64, float64, bool
	Value interface{} `json:"value"`
}

// JaegerLog represents a span log event
type JaegerLog struct {
	Timestamp int64       `json:"timestamp"`
	Fields    []JaegerTag `json:"fields"`
}

// JaegerProcess represents process metadata
type JaegerProcess struct {
	ServiceName string      `json:"serviceName"`
	Tags        []JaegerTag `json:"tags"`
}

// ValueString returns the tag value as a string
func (t JaegerTag) ValueString() string {
	if t.Value == nil {
		return ""
	}
	switch v := t.Value.(type) {
	case string:
		return v
	case float64:
		return fmt.Sprintf("%.0f", v)
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprintf("%v", v)
	}
}
