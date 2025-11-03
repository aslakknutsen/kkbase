package mcp

// QueryInput defines the input schema for the query tool
type QueryInput struct {
	Query  string                 `json:"query" jsonschema:"description:Cypher query to execute against the Neo4j knowledge graph"`
	Params map[string]interface{} `json:"params,omitempty" jsonschema:"description:Optional parameters for the Cypher query"`
}

// QueryOutput defines the output schema for the query tool
type QueryOutput struct {
	Results []map[string]interface{} `json:"results" jsonschema:"description:Query results as an array of records"`
	Count   int                      `json:"count" jsonschema:"description:Number of records returned"`
}

// StructureInput defines the input schema for the structure tool (no input required)
type StructureInput struct{}

// StructureOutput defines the output schema for the structure tool
type StructureOutput struct {
	NodeTypes              []string            `json:"node_types" jsonschema:"description:List of all node types (labels) in the graph"`
	NodeProperties         map[string][]string `json:"node_properties" jsonschema:"description:Properties for each node type"`
	RelationshipTypes      []string            `json:"relationship_types" jsonschema:"description:List of all relationship types in the graph"`
	RelationshipProperties map[string][]string `json:"relationship_properties" jsonschema:"description:Properties for each relationship type"`
	SchemaTriplets         []SchemaTriplet     `json:"schema_triplets" jsonschema:"description:From-Relationship-To triplets showing the graph structure"`
}

// SchemaTriplet represents a from-relationship-to pattern in the graph
type SchemaTriplet struct {
	From         string `json:"from" jsonschema:"description:Source node type"`
	Relationship string `json:"relationship" jsonschema:"description:Relationship type"`
	To           string `json:"to" jsonschema:"description:Target node type"`
}

// StartInvestigationInput defines the input for starting a metrics investigation
type StartInvestigationInput struct {
	ResourceType    string `json:"resource_type" jsonschema:"description:Type of Kubernetes resource to investigate (Pod Service Node Deployment StatefulSet DaemonSet)"`
	ResourceID      string `json:"resource_id" jsonschema:"description:Full resource ID (e.g. 'Pod/prod/api-gateway-xyz' or 'Node/worker-1')"`
	Symptom         string `json:"symptom" jsonschema:"description:Symptom being investigated (e.g. 'OOMKilled' 'HighLatency' 'CrashLoopBackOff' 'HighErrorRate' 'NodeNotReady')"`
	LookbackMinutes int    `json:"lookback_minutes" jsonschema:"description:How many minutes back to pull metrics from Prometheus (5-120 default 15)"`
}

// StartInvestigationOutput defines the output from starting an investigation
type StartInvestigationOutput struct {
	InvestigationID  string `json:"investigation_id" jsonschema:"description:Unique ID for this investigation"`
	Status           string `json:"status" jsonschema:"description:Investigation status (typically 'active')"`
	ResourceType     string `json:"resource_type" jsonschema:"description:Type of resource being investigated"`
	ResourceID       string `json:"resource_id" jsonschema:"description:Full resource ID"`
	Symptom          string `json:"symptom" jsonschema:"description:Symptom being investigated"`
	MetricsCollected int    `json:"metrics_collected" jsonschema:"description:Number of metric data points collected"`
	Message          string `json:"message" jsonschema:"description:Human-readable status message"`
}

// CompleteInvestigationInput defines the input for completing an investigation
type CompleteInvestigationInput struct {
	InvestigationID string `json:"investigation_id" jsonschema:"description:Investigation ID to complete and cleanup"`
}

// CompleteInvestigationOutput defines the output from completing an investigation
type CompleteInvestigationOutput struct {
	InvestigationID string `json:"investigation_id" jsonschema:"description:Investigation ID that was completed"`
	Status          string `json:"status" jsonschema:"description:Final status (typically 'completed')"`
	MetricsPurged   int    `json:"metrics_purged" jsonschema:"description:Number of metric data points purged from graph"`
	Message         string `json:"message" jsonschema:"description:Human-readable status message"`
}

// GetInvestigationStatusInput defines the input for querying investigation status
type GetInvestigationStatusInput struct {
	InvestigationID string `json:"investigation_id" jsonschema:"description:Investigation ID to query"`
}

// GetInvestigationStatusOutput defines the output of investigation status query
type GetInvestigationStatusOutput struct {
	InvestigationID  string `json:"investigation_id" jsonschema:"description:Investigation ID"`
	Status           string `json:"status" jsonschema:"description:Current status (active, completed, abandoned)"`
	ResourceType     string `json:"resource_type" jsonschema:"description:Type of resource being investigated"`
	ResourceID       string `json:"resource_id" jsonschema:"description:Full resource ID"`
	Symptom          string `json:"symptom" jsonschema:"description:Symptom being investigated"`
	StartTime        string `json:"start_time" jsonschema:"description:When investigation started (ISO 8601 format)"`
	LookbackDuration string `json:"lookback_duration" jsonschema:"description:How far back metrics were collected"`
}

// Agent Session Tool Types

// StartAgentSessionInput defines the input for starting an agent session
type StartAgentSessionInput struct {
	Symptom         string `json:"symptom" jsonschema:"description:Initial symptom being investigated (e.g. 'Orders failing for last 1m')"`
	InitialResource string `json:"initial_resource,omitempty" jsonschema:"description:Optional initial resource to investigate (e.g. 'Service/sf-orders/order-management')"`
}

// StartAgentSessionOutput defines the output from starting an agent session
type StartAgentSessionOutput struct {
	SessionID string `json:"session_id" jsonschema:"description:Unique session ID for this investigation"`
	Status    string `json:"status" jsonschema:"description:Session status (typically 'active')"`
	Message   string `json:"message" jsonschema:"description:Human-readable status message"`
}

// QueryWithSessionInput defines the input for executing a query within a session
type QueryWithSessionInput struct {
	SessionID string                 `json:"session_id" jsonschema:"description:Session ID to execute query in"`
	Query     string                 `json:"query" jsonschema:"description:Cypher query to execute"`
	Reasoning string                 `json:"reasoning" jsonschema:"description:Explanation of why this query is being run and what it seeks to discover"`
	Params    map[string]interface{} `json:"params,omitempty" jsonschema:"description:Optional parameters for the Cypher query"`
}

// QueryWithSessionOutput defines the output from executing a query with session tracking
type QueryWithSessionOutput struct {
	QueryID      string                   `json:"query_id" jsonschema:"description:Unique ID for this query execution"`
	Results      []map[string]interface{} `json:"results" jsonschema:"description:Query results as an array of records"`
	Count        int                      `json:"count" jsonschema:"description:Number of records returned"`
	Findings     []FindingOutput          `json:"findings" jsonschema:"description:Automatically extracted findings from query results"`
	FindingCount int                      `json:"finding_count" jsonschema:"description:Number of findings extracted"`
}

// FindingOutput represents a discovered issue
type FindingOutput struct {
	FindingID   string                 `json:"finding_id" jsonschema:"description:Unique finding ID"`
	Type        string                 `json:"type" jsonschema:"description:Type of finding (failed_dependency, unhealthy_pod, etc)"`
	Severity    string                 `json:"severity" jsonschema:"description:Severity level (critical, warning, info)"`
	ResourceID  string                 `json:"resource_id" jsonschema:"description:Affected resource ID"`
	Description string                 `json:"description" jsonschema:"description:Human-readable description of the issue"`
	Evidence    map[string]interface{} `json:"evidence,omitempty" jsonschema:"description:Evidence supporting this finding"`
}

// UpdateHypothesisInput defines the input for updating investigation hypothesis
type UpdateHypothesisInput struct {
	SessionID string `json:"session_id" jsonschema:"description:Session ID to update hypothesis for"`
	Stage     int    `json:"stage" jsonschema:"description:Investigation stage/round number (1, 2, 3, etc)"`
	Text      string `json:"text" jsonschema:"description:Current hypothesis text explaining the suspected root cause"`
}

// UpdateHypothesisOutput defines the output from updating hypothesis
type UpdateHypothesisOutput struct {
	HypothesisID     string `json:"hypothesis_id" jsonschema:"description:Unique ID for this hypothesis"`
	Stage            int    `json:"stage" jsonschema:"description:Investigation stage"`
	BlastZoneUpdated bool   `json:"blast_zone_updated" jsonschema:"description:Whether blast zone was recalculated"`
	Message          string `json:"message" jsonschema:"description:Human-readable status message"`
}

// RecordFindingInput defines the input for explicitly recording a finding
type RecordFindingInput struct {
	SessionID   string                 `json:"session_id" jsonschema:"description:Session ID to record finding for"`
	Type        string                 `json:"type" jsonschema:"description:Type of finding (failed_dependency, unhealthy_pod, error_spike, etc)"`
	ResourceID  string                 `json:"resource_id" jsonschema:"description:ID of affected resource"`
	Description string                 `json:"description" jsonschema:"description:Description of the issue discovered"`
	Severity    string                 `json:"severity" jsonschema:"description:Severity level: critical, warning, or info"`
	Evidence    map[string]interface{} `json:"evidence,omitempty" jsonschema:"description:Optional evidence supporting this finding"`
}

// RecordFindingOutput defines the output from recording a finding
type RecordFindingOutput struct {
	FindingID string `json:"finding_id" jsonschema:"description:Unique ID for the recorded finding"`
	Message   string `json:"message" jsonschema:"description:Human-readable status message"`
}

// SpawnInvestigationInput defines the input for spawning a metrics investigation
type SpawnInvestigationInput struct {
	SessionID       string `json:"session_id" jsonschema:"description:Session ID to link investigation to"`
	HypothesisID    string `json:"hypothesis_id,omitempty" jsonschema:"description:Optional hypothesis ID that triggered this investigation"`
	ResourceType    string `json:"resource_type" jsonschema:"description:Type of resource (Pod, Service, Node, etc)"`
	ResourceID      string `json:"resource_id" jsonschema:"description:Full resource ID (e.g. 'Pod/namespace/podname')"`
	Symptom         string `json:"symptom" jsonschema:"description:Symptom to investigate (OOMKilled, HighLatency, etc)"`
	LookbackMinutes int    `json:"lookback_minutes" jsonschema:"description:How many minutes back to pull metrics (5-120)"`
}

// SpawnInvestigationOutput defines the output from spawning an investigation
type SpawnInvestigationOutput struct {
	InvestigationID string `json:"investigation_id" jsonschema:"description:ID of the spawned investigation"`
	SessionID       string `json:"session_id" jsonschema:"description:Parent session ID"`
	Message         string `json:"message" jsonschema:"description:Human-readable status message"`
}

// CompleteAgentSessionInput defines the input for completing an agent session
type CompleteAgentSessionInput struct {
	SessionID string `json:"session_id" jsonschema:"description:Session ID to complete"`
	Summary   string `json:"summary,omitempty" jsonschema:"description:Optional summary of findings and root cause"`
}

// CompleteAgentSessionOutput defines the output from completing a session
type CompleteAgentSessionOutput struct {
	SessionID    string `json:"session_id" jsonschema:"description:Completed session ID"`
	Duration     string `json:"duration" jsonschema:"description:Total investigation duration"`
	QueryCount   int    `json:"query_count" jsonschema:"description:Total queries executed"`
	FindingCount int    `json:"finding_count" jsonschema:"description:Total findings discovered"`
	Message      string `json:"message" jsonschema:"description:Human-readable completion message"`
}
