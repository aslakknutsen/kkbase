package observability

import (
	"fmt"
	"time"
)

// AgentSession represents a complete AI agent diagnostic session
type AgentSession struct {
	ID              string         `json:"id"`
	InitialSymptom  string         `json:"initial_symptom"`
	InitialResource string         `json:"initial_resource,omitempty"`
	EventID         string         `json:"event_id,omitempty"`
	EventSource     string         `json:"event_source,omitempty"`
	EventTimestamp  *time.Time     `json:"event_timestamp,omitempty"`
	ProcessingDelay *time.Duration `json:"processing_delay,omitempty"`
	Status          string         `json:"status"` // "active", "completed", "timeout", "incomplete", "abandoned"
	CreatedAt       time.Time      `json:"created_at"`
	CompletedAt     *time.Time     `json:"completed_at,omitempty"`
	CurrentStage    int            `json:"current_stage"`
	QueryCount      int            `json:"query_count"`
	FindingCount    int            `json:"finding_count"`
	Summary         string         `json:"summary,omitempty"`
}

// Hypothesis represents a versioned hypothesis at a specific investigation stage
type Hypothesis struct {
	ID        string    `json:"id"`
	Stage     int       `json:"stage"`
	Text      string    `json:"text"`
	Status    string    `json:"status"` // "active", "superseded", "confirmed"
	CreatedAt time.Time `json:"created_at"`
}

// QueryExecution represents a single query executed by the agent with reasoning
type QueryExecution struct {
	ID          string                   `json:"id"`
	Query       string                   `json:"query"`
	Reasoning   string                   `json:"reasoning"`
	Params      map[string]interface{}   `json:"params,omitempty"`
	ResultCount int                      `json:"result_count"`
	Results     []map[string]interface{} `json:"results,omitempty"`   // Stored results (if enabled)
	Truncated   bool                     `json:"truncated,omitempty"` // True if results were truncated
	Duration    time.Duration            `json:"duration"`
	ExecutedAt  time.Time                `json:"executed_at"`
	Findings    []string                 `json:"findings"` // IDs of auto-extracted findings
}

// Finding represents a discovered issue (failed service, unhealthy pod, etc.)
type Finding struct {
	ID              string                 `json:"id"`
	Type            string                 `json:"type"`     // "failed_dependency", "unhealthy_pod", "error_spike", "deployment_change"
	Severity        string                 `json:"severity"` // "critical", "warning", "info"
	ResourceID      string                 `json:"resource_id"`
	ResourceType    string                 `json:"resource_type,omitempty"`
	Description     string                 `json:"description"`
	Evidence        map[string]interface{} `json:"evidence,omitempty"`
	DetectionMethod string                 `json:"detection_method"` // "automatic", "agent_recorded"
	DiscoveredAt    time.Time              `json:"discovered_at"`
}

// Recommendation represents an actionable next step identified during investigation
type Recommendation struct {
	ID              string                 `json:"id"`
	Type            string                 `json:"type"`     // "root_cause_fix", "preventive_action", "optimization", "monitoring_improvement", "cleanup"
	Priority        string                 `json:"priority"` // "critical", "high", "medium", "low"
	Title           string                 `json:"title"`
	Description     string                 `json:"description"`
	Rationale       string                 `json:"rationale"`                  // Why this is recommended
	RelatedFindings []string               `json:"related_findings"`           // Finding IDs that led to this recommendation
	ActionItems     []string               `json:"action_items"`               // Step-by-step actions
	EstimatedEffort string                 `json:"estimated_effort,omitempty"` // "minutes", "hours", "days"
	AutomationHint  string                 `json:"automation_hint,omitempty"`  // Hints for automation (e.g., kubectl commands, scripts)
	Tags            []string               `json:"tags,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt       time.Time              `json:"created_at"`
}

// Pattern represents a reusable diagnostic pattern learned from investigations
type Pattern struct {
	ID                    string                 `json:"id"`
	Name                  string                 `json:"name"`
	RootCauseResourceType string                 `json:"root_cause_resource_type"` // e.g., "Service", "Pod", "HTTPRoute"
	RootCauseIssueType    string                 `json:"root_cause_issue_type"`    // e.g., "cascading_failure", "selector_mismatch"
	InvestigationSteps    []string               `json:"investigation_steps"`      // Ordered sequence of steps
	DiagnosisGuidance     string                 `json:"diagnosis_guidance"`       // What to look for
	Recommendations       []string               `json:"recommendations"`          // Generic recommendations
	BundleID              string                 `json:"bundle_id,omitempty"`      // Null for discovered patterns
	Source                string                 `json:"source"`                   // "discovered" or "bundled"
	UsageCount            int                    `json:"usage_count"`              // How many times used
	CreatedAt             time.Time              `json:"created_at"`
	Metadata              map[string]interface{} `json:"metadata,omitempty"`
}

// PatternBundle represents a versioned collection of patterns
type PatternBundle struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Version     string    `json:"version"`
	SourceURL   string    `json:"source_url,omitempty"`
	Description string    `json:"description"`
	ImportedAt  time.Time `json:"imported_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Active      bool      `json:"active"`
}

// BlastZoneSnapshot represents a point-in-time view of the blast zone graph
type BlastZoneSnapshot struct {
	SessionID     string          `json:"session_id"`
	Timestamp     time.Time       `json:"timestamp"`
	TriggerEvent  string          `json:"trigger_event"` // What caused recalculation
	Nodes         []BlastZoneNode `json:"nodes"`
	Edges         []BlastZoneEdge `json:"edges"`
	ImpactRadius  int             `json:"impact_radius"` // Degrees of separation
	AffectedCount int             `json:"affected_count"`
}

// BlastZoneNode represents a node in the blast zone graph
type BlastZoneNode struct {
	ID         string                 `json:"id"`
	Label      string                 `json:"label"`
	Type       string                 `json:"type"`
	Status     string                 `json:"status"` // "failed", "degraded", "healthy"
	Properties map[string]interface{} `json:"properties,omitempty"`
}

// BlastZoneEdge represents an edge in the blast zone graph
type BlastZoneEdge struct {
	Source     string                 `json:"source"`
	Target     string                 `json:"target"`
	Type       string                 `json:"type"`
	Status     string                 `json:"status"` // "failing", "ok"
	Properties map[string]interface{} `json:"properties,omitempty"`
}

// TimelineEvent represents an event in the investigation timeline
type TimelineEvent struct {
	Timestamp time.Time              `json:"timestamp"`
	Type      string                 `json:"type"` // "hypothesis", "query", "finding", "investigation"
	Data      map[string]interface{} `json:"data"`
}

// SessionSummary represents the final summary of a completed investigation
type SessionSummary struct {
	SessionID       string             `json:"session_id"`
	InitialSymptom  string             `json:"initial_symptom"`
	Duration        time.Duration      `json:"duration"`
	TotalQueries    int                `json:"total_queries"`
	TotalFindings   int                `json:"total_findings"`
	FinalHypothesis string             `json:"final_hypothesis"`
	RootCause       string             `json:"root_cause,omitempty"`
	BlastZone       *BlastZoneSnapshot `json:"blast_zone,omitempty"`
	Recommendations []Recommendation   `json:"recommendations"`
	CompletedAt     time.Time          `json:"completed_at"`
}

// ActiveSessionInfo represents summary info for listing active sessions
type ActiveSessionInfo struct {
	ID              string         `json:"id"`
	InitialSymptom  string         `json:"initial_symptom"`
	EventID         string         `json:"event_id,omitempty"`
	EventSource     string         `json:"event_source,omitempty"`
	EventTimestamp  *time.Time     `json:"event_timestamp,omitempty"`
	ProcessingDelay *time.Duration `json:"processing_delay,omitempty"`
	Status          string         `json:"status"`
	CreatedAt       time.Time      `json:"created_at"`
	CompletedAt     *time.Time     `json:"completed_at,omitempty"`
	QueryCount      int            `json:"query_count"`
	FindingCount    int            `json:"finding_count"`
	CurrentStage    int            `json:"current_stage"`
}

// SessionDetail represents complete session data with all related entities
type SessionDetail struct {
	Session           *AgentSession    `json:"session"`
	Hypotheses        []Hypothesis     `json:"hypotheses"`
	Queries           []QueryExecution `json:"queries"`
	Findings          []Finding        `json:"findings"`
	Recommendations   []Recommendation `json:"recommendations"`
	Patterns          []Pattern        `json:"patterns"` // Discovered patterns
	Investigations    []string         `json:"investigations"` // Investigation IDs
	CurrentHypothesis *Hypothesis      `json:"current_hypothesis,omitempty"`
}

// generateSessionID creates a unique session identifier
func generateSessionID() string {
	return fmt.Sprintf("session_%d", time.Now().UnixNano())
}

// generateHypothesisID creates a unique hypothesis identifier
func generateHypothesisID() string {
	return fmt.Sprintf("hypothesis_%d", time.Now().UnixNano())
}

// generateQueryID creates a unique query execution identifier
func generateQueryID() string {
	return fmt.Sprintf("query_%d", time.Now().UnixNano())
}

// generateFindingID creates a unique finding identifier
func generateFindingID() string {
	return fmt.Sprintf("finding_%d", time.Now().UnixNano())
}

// generateRecommendationID creates a unique recommendation identifier
func generateRecommendationID() string {
	return fmt.Sprintf("recommendation_%d", time.Now().UnixNano())
}

// generatePatternID creates a unique pattern identifier
func generatePatternID() string {
	return fmt.Sprintf("pattern_%d", time.Now().UnixNano())
}
