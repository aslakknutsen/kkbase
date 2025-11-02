package mcp

import (
	"context"
	"fmt"

	"github.com/kagenti/kkbase/pkg/graph"
	"go.uber.org/zap"
)

// QueryTool executes a read-only Cypher query against the graph database
func QueryTool(ctx context.Context, store graph.GraphStore, logger *zap.Logger, input QueryInput) (*QueryOutput, error) {
	// Validate query is read-only
	if err := ValidateReadOnlyQuery(input.Query); err != nil {
		logger.Warn("rejected write operation in query",
			zap.Error(err),
			zap.String("query", input.Query))
		return nil, err
	}

	// Execute query
	logger.Debug("executing cypher query",
		zap.String("query", input.Query),
		zap.Any("params", input.Params))

	results, err := store.Query(ctx, input.Query, input.Params)
	if err != nil {
		logger.Error("query execution failed",
			zap.Error(err),
			zap.String("query", input.Query))
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	logger.Info("query executed successfully",
		zap.Int("result_count", len(results)),
		zap.String("query", input.Query))

	return &QueryOutput{
		Results: results,
		Count:   len(results),
	}, nil
}

// StructureTool returns the complete graph schema including node types, relationship types, and their properties
func StructureTool(ctx context.Context, store graph.GraphStore, logger *zap.Logger) (*StructureOutput, error) {
	logger.Debug("fetching graph structure")

	output := &StructureOutput{
		NodeProperties:         make(map[string][]string),
		RelationshipProperties: make(map[string][]string),
		SchemaTriplets:         make([]SchemaTriplet, 0),
	}

	// Query 1: Get all node types and their properties in a single query
	nodePropsQuery := `
		MATCH (n)
		WITH labels(n)[0] AS NodeType, n
		UNWIND keys(n) AS Property
		RETURN NodeType, collect(DISTINCT Property) AS Properties
		ORDER BY NodeType
	`

	nodePropsResult, err := store.Query(ctx, nodePropsQuery, nil)
	if err != nil {
		logger.Error("failed to fetch node types and properties", zap.Error(err))
		return nil, fmt.Errorf("failed to fetch node types and properties: %w", err)
	}

	// Process node types and properties
	for _, record := range nodePropsResult {
		nodeType, nodeTypeOk := record["NodeType"].(string)
		properties, propsOk := record["Properties"].([]interface{})

		if nodeTypeOk && propsOk && nodeType != "" {
			output.NodeTypes = append(output.NodeTypes, nodeType)

			// Convert properties from []interface{} to []string
			propStrings := make([]string, 0, len(properties))
			for _, prop := range properties {
				if propStr, ok := prop.(string); ok {
					propStrings = append(propStrings, propStr)
				}
			}
			output.NodeProperties[nodeType] = propStrings
		}
	}

	// Query 2: Get all relationship types and their properties in a single query
	relPropsQuery := `
		MATCH ()-[r]->()
		WITH type(r) AS RelationshipType, r
		UNWIND keys(r) AS Property
		RETURN RelationshipType, collect(DISTINCT Property) AS Properties
		ORDER BY RelationshipType
	`

	relPropsResult, err := store.Query(ctx, relPropsQuery, nil)
	if err != nil {
		logger.Error("failed to fetch relationship types and properties", zap.Error(err))
		return nil, fmt.Errorf("failed to fetch relationship types and properties: %w", err)
	}

	// Process relationship types and properties
	for _, record := range relPropsResult {
		relType, relTypeOk := record["RelationshipType"].(string)
		properties, propsOk := record["Properties"].([]interface{})

		if relTypeOk && propsOk {
			output.RelationshipTypes = append(output.RelationshipTypes, relType)

			// Convert properties from []interface{} to []string
			propStrings := make([]string, 0, len(properties))
			for _, prop := range properties {
				if propStr, ok := prop.(string); ok {
					propStrings = append(propStrings, propStr)
				}
			}
			output.RelationshipProperties[relType] = propStrings
		}
	}

	// Query 3: Get schema triplets (from-relationship-to patterns)
	tripletsQuery := `
		MATCH (a)-[r]->(b)
		RETURN DISTINCT labels(a)[0] AS FromNodeType, 
			type(r) AS RelationshipType, 
			labels(b)[0] AS ToNodeType
		ORDER BY FromNodeType, RelationshipType, ToNodeType
	`

	tripletsResult, err := store.Query(ctx, tripletsQuery, nil)
	if err != nil {
		logger.Error("failed to fetch schema triplets", zap.Error(err))
		return nil, fmt.Errorf("failed to fetch schema triplets: %w", err)
	}

	// Process schema triplets
	for _, record := range tripletsResult {
		fromNodeType, fromOk := record["FromNodeType"].(string)
		relType, relOk := record["RelationshipType"].(string)
		toNodeType, toOk := record["ToNodeType"].(string)

		if fromOk && relOk && toOk && fromNodeType != "" && toNodeType != "" {
			output.SchemaTriplets = append(output.SchemaTriplets, SchemaTriplet{
				From:         fromNodeType,
				Relationship: relType,
				To:           toNodeType,
			})
		}
	}

	logger.Info("graph structure fetched successfully",
		zap.Int("node_types", len(output.NodeTypes)),
		zap.Int("relationship_types", len(output.RelationshipTypes)),
		zap.Int("triplets", len(output.SchemaTriplets)))

	return output, nil
}
