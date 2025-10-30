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
