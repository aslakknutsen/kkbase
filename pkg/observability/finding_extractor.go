package observability

import (
	"fmt"
	"strings"
)

// FindingExtractor automatically extracts findings from query results
type FindingExtractor struct {
	patterns []FindingPattern
}

// FindingPattern defines a pattern for detecting findings in query results
type FindingPattern struct {
	Name    string
	Matches func(result map[string]interface{}) bool
	Extract func(result map[string]interface{}) *Finding
}

// NewFindingExtractor creates a new finding extractor with default patterns
func NewFindingExtractor() *FindingExtractor {
	return &FindingExtractor{
		patterns: DefaultPatterns(),
	}
}

// ExtractFindings analyzes query results and extracts findings based on patterns
func (fe *FindingExtractor) ExtractFindings(results []map[string]interface{}) []*Finding {
	findings := make([]*Finding, 0)
	seen := make(map[string]bool) // Deduplicate

	for _, result := range results {
		for _, pattern := range fe.patterns {
			if pattern.Matches(result) {
				finding := pattern.Extract(result)
				if finding != nil && !seen[finding.ResourceID+finding.Type] {
					finding.ID = generateFindingID()
					finding.DetectionMethod = "automatic"
					findings = append(findings, finding)
					seen[finding.ResourceID+finding.Type] = true
				}
			}
		}
	}

	return findings
}

// DefaultPatterns returns the default set of finding patterns
func DefaultPatterns() []FindingPattern {
	return []FindingPattern{
		// Pattern 1: Failed service calls
		{
			Name: "failed_service_call",
			Matches: func(r map[string]interface{}) bool {
				// Check for FAILED_CALL_TO relationship or error_message field
				if hasKey(r, "error_message") && r["error_message"] != nil && r["error_message"] != "" {
					return true
				}
				if hasKey(r, "r.error_message") && r["r.error_message"] != nil && r["r.error_message"] != "" {
					return true
				}
				// Check for relationship type
				if relType, ok := r["type(r)"].(string); ok && relType == "FAILED_CALL_TO" {
					return true
				}
				return false
			},
			Extract: func(r map[string]interface{}) *Finding {
				// Try to extract source and target
				source := extractStringField(r, "source.name", "caller", "source")
				target := extractStringField(r, "target.name", "dependency", "target")
				errorMsg := extractStringField(r, "error_message", "r.error_message", "message")
				statusCode := extractIntField(r, "status_code", "r.status_code")

				resourceID := target
				if resourceID == "" {
					resourceID = source
				}

				description := fmt.Sprintf("Failed call from %s to %s", source, target)
				if errorMsg != "" {
					description += fmt.Sprintf(": %s", errorMsg)
				}
				if statusCode > 0 {
					description += fmt.Sprintf(" (HTTP %d)", statusCode)
				}

				return &Finding{
					Type:         "failed_dependency",
					Severity:     "critical",
					ResourceID:   resourceID,
					ResourceType: "Service",
					Description:  description,
					Evidence: map[string]interface{}{
						"source":      source,
						"target":      target,
						"error":       errorMsg,
						"status_code": statusCode,
					},
				}
			},
		},

		// Pattern 2: Unhealthy pods
		{
			Name: "unhealthy_pod",
			Matches: func(r map[string]interface{}) bool {
				// Check if result has pod with non-Running status
				if status, ok := r["status"].(string); ok {
					if status != "Running" && (hasKey(r, "pod_name") || hasKey(r, "p.name")) {
						return true
					}
				}
				if status, ok := r["pod_status"].(string); ok {
					return status != "Running"
				}
				if status, ok := r["p.status"].(string); ok {
					return status != "Running"
				}
				return false
			},
			Extract: func(r map[string]interface{}) *Finding {
				podName := extractStringField(r, "pod_name", "p.name", "name")
				namespace := extractStringField(r, "namespace", "p.namespace")
				status := extractStringField(r, "status", "pod_status", "p.status")

				resourceID := fmt.Sprintf("Pod/%s/%s", namespace, podName)
				if namespace == "" {
					resourceID = fmt.Sprintf("Pod/%s", podName)
				}

				return &Finding{
					Type:         "unhealthy_pod",
					Severity:     determinePodSeverity(status),
					ResourceID:   resourceID,
					ResourceType: "Pod",
					Description:  fmt.Sprintf("Pod %s in %s state", podName, status),
					Evidence: map[string]interface{}{
						"pod":       podName,
						"namespace": namespace,
						"status":    status,
					},
				}
			},
		},

		// Pattern 3: Error spans in traces
		{
			Name: "trace_error_span",
			Matches: func(r map[string]interface{}) bool {
				if status, ok := r["status"].(string); ok && status == "ERROR" {
					return hasKey(r, "service_name") || hasKey(r, "s.service_name")
				}
				if status, ok := r["s.status"].(string); ok && status == "ERROR" {
					return true
				}
				return false
			},
			Extract: func(r map[string]interface{}) *Finding {
				serviceName := extractStringField(r, "service_name", "s.service_name")
				operation := extractStringField(r, "operation_name", "s.operation_name")
				errorMsg := extractStringField(r, "error_message", "s.error_message")

				return &Finding{
					Type:         "error_spike",
					Severity:     "warning",
					ResourceID:   fmt.Sprintf("Service/%s", serviceName),
					ResourceType: "Service",
					Description:  fmt.Sprintf("Error in %s operation %s: %s", serviceName, operation, errorMsg),
					Evidence: map[string]interface{}{
						"service":   serviceName,
						"operation": operation,
						"error":     errorMsg,
					},
				}
			},
		},

		// Pattern 4: OOMKilled events
		{
			Name: "oom_killed",
			Matches: func(r map[string]interface{}) bool {
				reason := extractStringField(r, "reason", "e.reason")
				return reason == "OOMKilled"
			},
			Extract: func(r map[string]interface{}) *Finding {
				podName := extractStringField(r, "involved_object_name", "pod_name")
				namespace := extractStringField(r, "involved_object_namespace", "namespace")
				message := extractStringField(r, "message", "e.message")

				return &Finding{
					Type:         "oom_killed",
					Severity:     "critical",
					ResourceID:   fmt.Sprintf("Pod/%s/%s", namespace, podName),
					ResourceType: "Pod",
					Description:  fmt.Sprintf("Pod %s OOMKilled: %s", podName, message),
					Evidence: map[string]interface{}{
						"pod":       podName,
						"namespace": namespace,
						"message":   message,
					},
				}
			},
		},

		// Pattern 5: High restart counts
		{
			Name: "high_restart_count",
			Matches: func(r map[string]interface{}) bool {
				restartCount := extractIntField(r, "restart_count", "c.restart_count", "restartCount")
				return restartCount >= 5
			},
			Extract: func(r map[string]interface{}) *Finding {
				podName := extractStringField(r, "pod_name", "p.name")
				namespace := extractStringField(r, "namespace", "p.namespace")
				restartCount := extractIntField(r, "restart_count", "c.restart_count", "restartCount")

				return &Finding{
					Type:         "high_restart_count",
					Severity:     "warning",
					ResourceID:   fmt.Sprintf("Pod/%s/%s", namespace, podName),
					ResourceType: "Pod",
					Description:  fmt.Sprintf("Pod %s has high restart count: %d", podName, restartCount),
					Evidence: map[string]interface{}{
						"pod":           podName,
						"namespace":     namespace,
						"restart_count": restartCount,
					},
				}
			},
		},

		// Pattern 6: HTTP 5xx errors
		{
			Name: "http_5xx_error",
			Matches: func(r map[string]interface{}) bool {
				statusCode := extractIntField(r, "status_code", "r.status_code")
				return statusCode >= 500 && statusCode < 600
			},
			Extract: func(r map[string]interface{}) *Finding {
				serviceName := extractStringField(r, "service", "target", "downstream")
				statusCode := extractIntField(r, "status_code", "r.status_code")
				errorMsg := extractStringField(r, "error_message", "r.error_message")

				return &Finding{
					Type:         "http_error",
					Severity:     "critical",
					ResourceID:   fmt.Sprintf("Service/%s", serviceName),
					ResourceType: "Service",
					Description:  fmt.Sprintf("HTTP %d error from %s: %s", statusCode, serviceName, errorMsg),
					Evidence: map[string]interface{}{
						"status_code": statusCode,
						"error":       errorMsg,
						"service":     serviceName,
					},
				}
			},
		},
	}
}

// Helper functions

func hasKey(m map[string]interface{}, key string) bool {
	_, ok := m[key]
	return ok
}

func extractStringField(m map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if val, ok := m[key]; ok && val != nil {
			if str, ok := val.(string); ok && str != "" {
				return str
			}
		}
	}
	return ""
}

func extractIntField(m map[string]interface{}, keys ...string) int {
	for _, key := range keys {
		if val, ok := m[key]; ok && val != nil {
			switch v := val.(type) {
			case int:
				return v
			case int64:
				return int(v)
			case float64:
				return int(v)
			}
		}
	}
	return 0
}

func determinePodSeverity(status string) string {
	status = strings.ToLower(status)
	switch status {
	case "crashloopbackoff", "error", "failed":
		return "critical"
	case "pending", "unknown":
		return "warning"
	default:
		return "info"
	}
}
