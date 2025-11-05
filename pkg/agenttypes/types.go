package agenttypes

import "time"

// EventType represents the type of event
type EventType string

const (
	EventTypeK8sEvent        EventType = "k8s_event"
	EventTypePrometheusAlert EventType = "prometheus_alert"
	EventTypeCustom          EventType = "custom"
)

// Severity represents event severity level
type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityWarning  Severity = "warning"
	SeverityInfo     Severity = "info"
)

// ResourceRef references a Kubernetes resource
type ResourceRef struct {
	Type      string // Pod, Service, Node, etc.
	Namespace string
	Name      string
	ID        string // Full ID like "Pod/namespace/name"
}

// Event represents an event that requires investigation
type Event struct {
	ID        string
	Type      EventType
	Source    string
	Severity  Severity
	Timestamp time.Time
	Resource  ResourceRef
	Reason    string
	Message   string
	Data      map[string]interface{}
	Labels    map[string]string
}

// ProcessedEvent is an event that has been filtered and prioritized
type ProcessedEvent struct {
	Event    Event
	Priority int
}

// InvestigationResult represents the outcome of an agent investigation
type InvestigationResult struct {
	SessionID       string
	Event           Event
	Analysis        *Analysis
	Recommendations []Recommendation
	Status          string
	Error           error
	Duration        time.Duration
}

// Analysis represents the LLM's analysis of an event
type Analysis struct {
	RootCause        string
	ImpactAssessment string
	Confidence       float32
	RelatedResources []string
}

// Recommendation represents an actionable recommendation
type Recommendation struct {
	Action       string
	Description  string
	RiskLevel    string
	AutoApproved bool
}
