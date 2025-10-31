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
