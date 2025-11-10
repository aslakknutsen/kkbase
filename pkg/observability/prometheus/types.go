package prometheus

import (
	"fmt"
	"strconv"
	"time"

	"github.com/aslakknutsen/kkbase/pkg/observability"
)

// PrometheusResponse represents the top-level response from Prometheus API
type PrometheusResponse struct {
	Status    string         `json:"status"` // "success" or "error"
	Data      PrometheusData `json:"data"`
	Error     string         `json:"error,omitempty"`
	ErrorType string         `json:"errorType,omitempty"`
}

// PrometheusData contains the query result data
type PrometheusData struct {
	ResultType string             `json:"resultType"` // "matrix", "vector", "scalar", "string"
	Result     []PrometheusResult `json:"result"`
}

// PrometheusResult represents a single time series result
type PrometheusResult struct {
	Metric PrometheusMetric `json:"metric"`           // Label set
	Value  []interface{}    `json:"value,omitempty"`  // For instant queries: [timestamp, "value"]
	Values [][]interface{}  `json:"values,omitempty"` // For range queries: [[timestamp, "value"], ...]
}

// PrometheusMetric contains the metric labels
type PrometheusMetric map[string]string

// ParseTimestamp parses a Prometheus timestamp (seconds since epoch, as float64)
func ParseTimestamp(ts interface{}) (time.Time, error) {
	var seconds float64
	switch v := ts.(type) {
	case float64:
		seconds = v
	case int64:
		seconds = float64(v)
	case int:
		seconds = float64(v)
	default:
		return time.Time{}, fmt.Errorf("invalid timestamp type: %T", ts)
	}
	return time.Unix(int64(seconds), 0), nil
}

// ParseValue parses a Prometheus metric value (string representation of float)
func ParseValue(val interface{}) (float64, error) {
	switch v := val.(type) {
	case string:
		return strconv.ParseFloat(v, 64)
	case float64:
		return v, nil
	case int64:
		return float64(v), nil
	case int:
		return float64(v), nil
	default:
		return 0, fmt.Errorf("invalid value type: %T", val)
	}
}

// ConvertToMetricData converts Prometheus result to MetricData
func (pr *PrometheusResult) ConvertToMetricData(metricName string) ([]observability.MetricData, error) {
	var metrics []observability.MetricData

	// Extract metric type from labels if available
	metricType := observability.MetricTypeUnknown
	if typeLabel, ok := pr.Metric["__type__"]; ok {
		metricType = observability.MetricType(typeLabel)
	}

	// Convert labels to map[string]string
	labels := make(map[string]string)
	for k, v := range pr.Metric {
		if k != "__name__" && k != "__type__" { // Exclude internal labels
			labels[k] = v
		}
	}

	// Use metric name from labels if available, otherwise use provided name
	if name, ok := pr.Metric["__name__"]; ok && name != "" {
		metricName = name
	}

	// Handle range query results (multiple values)
	if len(pr.Values) > 0 {
		for _, valuePoint := range pr.Values {
			if len(valuePoint) != 2 {
				continue
			}
			timestamp, err := ParseTimestamp(valuePoint[0])
			if err != nil {
				continue
			}
			value, err := ParseValue(valuePoint[1])
			if err != nil {
				continue
			}
			metrics = append(metrics, observability.MetricData{
				Name:      metricName,
				Type:      metricType,
				Value:     value,
				Timestamp: timestamp,
				Labels:    labels,
			})
		}
	} else if len(pr.Value) == 2 {
		// Handle instant query result (single value)
		timestamp, err := ParseTimestamp(pr.Value[0])
		if err != nil {
			return nil, err
		}
		value, err := ParseValue(pr.Value[1])
		if err != nil {
			return nil, err
		}
		metrics = append(metrics, observability.MetricData{
			Name:      metricName,
			Type:      metricType,
			Value:     value,
			Timestamp: timestamp,
			Labels:    labels,
		})
	}

	return metrics, nil
}
