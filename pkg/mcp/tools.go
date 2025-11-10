package mcp

import (
	"context"
	"fmt"

	"github.com/aslakknutsen/kkbase/pkg/graph"
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

	// Get all node labels using efficient Neo4j procedure
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

	// Get all relationship types using efficient Neo4j procedure
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

	// Get node properties for each label (sample first 100 nodes)
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

	// Get relationship properties for each type (sample first 100 relationships)
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

	// Get schema triplets by checking specific combinations efficiently
	// This is faster than a full graph scan on large databases
	for _, nodeType := range output.NodeTypes {
		for _, relType := range output.RelationshipTypes {
			// Check what target node types exist for this combination
			tripletQuery := fmt.Sprintf(`
				MATCH (a:%s)-[r:%s]->(b)
				RETURN DISTINCT labels(b)[0] AS toLabel
				LIMIT 10
			`, nodeType, relType)

			tripletResult, err := store.Query(ctx, tripletQuery, nil)
			if err != nil {
				logger.Debug("failed to fetch triplet",
					zap.String("from", nodeType),
					zap.String("rel", relType),
					zap.Error(err))
				continue
			}

			for _, record := range tripletResult {
				if toLabel, ok := record["toLabel"].(string); ok && toLabel != "" {
					output.SchemaTriplets = append(output.SchemaTriplets, SchemaTriplet{
						From:         nodeType,
						Relationship: relType,
						To:           toLabel,
					})
				}
			}
		}
	}

	logger.Info("graph structure fetched successfully",
		zap.Int("node_types", len(output.NodeTypes)),
		zap.Int("relationship_types", len(output.RelationshipTypes)),
		zap.Int("triplets", len(output.SchemaTriplets)))

	return output, nil
}
