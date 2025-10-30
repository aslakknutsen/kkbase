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

	// Get all node labels
	labelsQuery := "CALL db.labels() YIELD label RETURN label ORDER BY label"
	labelsResult, err := store.Query(ctx, labelsQuery, nil)
	if err != nil {
		logger.Error("failed to fetch node labels", zap.Error(err))
		return nil, fmt.Errorf("failed to fetch node labels: %w", err)
	}

	for _, record := range labelsResult {
		if label, ok := record["label"].(string); ok {
			output.NodeTypes = append(output.NodeTypes, label)
		}
	}

	// Get all relationship types
	relTypesQuery := "CALL db.relationshipTypes() YIELD relationshipType RETURN relationshipType ORDER BY relationshipType"
	relTypesResult, err := store.Query(ctx, relTypesQuery, nil)
	if err != nil {
		logger.Error("failed to fetch relationship types", zap.Error(err))
		return nil, fmt.Errorf("failed to fetch relationship types: %w", err)
	}

	for _, record := range relTypesResult {
		if relType, ok := record["relationshipType"].(string); ok {
			output.RelationshipTypes = append(output.RelationshipTypes, relType)
		}
	}

	// Get node properties for each label
	for _, label := range output.NodeTypes {
		propsQuery := fmt.Sprintf(`
			MATCH (n:%s)
			WITH n LIMIT 100
			UNWIND keys(n) AS key
			RETURN DISTINCT key
			ORDER BY key
		`, label)

		propsResult, err := store.Query(ctx, propsQuery, nil)
		if err != nil {
			logger.Warn("failed to fetch properties for node type",
				zap.String("node_type", label),
				zap.Error(err))
			continue
		}

		properties := make([]string, 0)
		for _, record := range propsResult {
			if key, ok := record["key"].(string); ok {
				properties = append(properties, key)
			}
		}
		output.NodeProperties[label] = properties
	}

	// Get relationship properties for each type
	for _, relType := range output.RelationshipTypes {
		propsQuery := fmt.Sprintf(`
			MATCH ()-[r:%s]->()
			WITH r LIMIT 100
			UNWIND keys(r) AS key
			RETURN DISTINCT key
			ORDER BY key
		`, relType)

		propsResult, err := store.Query(ctx, propsQuery, nil)
		if err != nil {
			logger.Warn("failed to fetch properties for relationship type",
				zap.String("relationship_type", relType),
				zap.Error(err))
			continue
		}

		properties := make([]string, 0)
		for _, record := range propsResult {
			if key, ok := record["key"].(string); ok {
				properties = append(properties, key)
			}
		}
		output.RelationshipProperties[relType] = properties
	}

	// Get schema triplets (from-relationship-to patterns)
	tripletsQuery := `
		MATCH (a)-[r]->(b)
		WITH DISTINCT labels(a) AS fromLabels, type(r) AS relType, labels(b) AS toLabels
		UNWIND fromLabels AS fromLabel
		UNWIND toLabels AS toLabel
		RETURN DISTINCT fromLabel, relType, toLabel
		ORDER BY fromLabel, relType, toLabel
	`

	tripletsResult, err := store.Query(ctx, tripletsQuery, nil)
	if err != nil {
		logger.Error("failed to fetch schema triplets", zap.Error(err))
		return nil, fmt.Errorf("failed to fetch schema triplets: %w", err)
	}

	for _, record := range tripletsResult {
		fromLabel, fromOk := record["fromLabel"].(string)
		relType, relOk := record["relType"].(string)
		toLabel, toOk := record["toLabel"].(string)

		if fromOk && relOk && toOk {
			output.SchemaTriplets = append(output.SchemaTriplets, SchemaTriplet{
				From:         fromLabel,
				Relationship: relType,
				To:           toLabel,
			})
		}
	}

	logger.Info("graph structure fetched successfully",
		zap.Int("node_types", len(output.NodeTypes)),
		zap.Int("relationship_types", len(output.RelationshipTypes)),
		zap.Int("triplets", len(output.SchemaTriplets)))

	return output, nil
}
