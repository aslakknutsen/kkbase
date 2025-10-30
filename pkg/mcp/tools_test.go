package mcp

import (
	"context"
	"testing"

	kktesting "github.com/kagenti/kkbase/pkg/testing"
	"go.uber.org/zap"
)

func TestQueryTool(t *testing.T) {
	logger := zap.NewNop()
	ctx := context.Background()

	t.Run("successful query", func(t *testing.T) {
		mockStore := kktesting.NewMockGraphStore()
		mockStore.SetQueryResult([]map[string]interface{}{
			{"name": "pod-1", "namespace": "default"},
			{"name": "pod-2", "namespace": "kube-system"},
		})

		input := QueryInput{
			Query:  "MATCH (p:Pod) RETURN p.name as name, p.namespace as namespace",
			Params: nil,
		}

		output, err := QueryTool(ctx, mockStore, logger, input)
		if err != nil {
			t.Fatalf("QueryTool() error = %v", err)
		}

		if output.Count != 2 {
			t.Errorf("QueryTool() count = %d, want 2", output.Count)
		}

		if len(output.Results) != 2 {
			t.Errorf("QueryTool() results length = %d, want 2", len(output.Results))
		}
	})

	t.Run("query with parameters", func(t *testing.T) {
		mockStore := kktesting.NewMockGraphStore()
		mockStore.SetQueryResult([]map[string]interface{}{
			{"name": "pod-1", "namespace": "default"},
		})

		input := QueryInput{
			Query: "MATCH (p:Pod) WHERE p.namespace = $namespace RETURN p",
			Params: map[string]interface{}{
				"namespace": "default",
			},
		}

		output, err := QueryTool(ctx, mockStore, logger, input)
		if err != nil {
			t.Fatalf("QueryTool() error = %v", err)
		}

		if output.Count != 1 {
			t.Errorf("QueryTool() count = %d, want 1", output.Count)
		}
	})

	t.Run("rejected write operation", func(t *testing.T) {
		mockStore := kktesting.NewMockGraphStore()

		input := QueryInput{
			Query:  "CREATE (p:Pod {name: 'test'}) RETURN p",
			Params: nil,
		}

		_, err := QueryTool(ctx, mockStore, logger, input)
		if err == nil {
			t.Error("QueryTool() expected error for write operation, got nil")
		}
	})

	t.Run("query execution error", func(t *testing.T) {
		mockStore := kktesting.NewMockGraphStore()
		mockStore.SetQueryError("query execution failed")

		input := QueryInput{
			Query:  "MATCH (p:Pod) RETURN p",
			Params: nil,
		}

		_, err := QueryTool(ctx, mockStore, logger, input)
		if err == nil {
			t.Error("QueryTool() expected error from query execution, got nil")
		}
	})

	t.Run("empty query", func(t *testing.T) {
		mockStore := kktesting.NewMockGraphStore()

		input := QueryInput{
			Query:  "",
			Params: nil,
		}

		_, err := QueryTool(ctx, mockStore, logger, input)
		if err == nil {
			t.Error("QueryTool() expected error for empty query, got nil")
		}
	})
}

func TestStructureTool(t *testing.T) {
	logger := zap.NewNop()
	ctx := context.Background()

	t.Run("successful structure fetch", func(t *testing.T) {
		mockStore := kktesting.NewMockGraphStore()

		// Mock labels query
		mockStore.AddQueryResponse(
			"CALL db.labels()",
			[]map[string]interface{}{
				{"label": "Pod"},
				{"label": "Service"},
				{"label": "Node"},
			},
		)

		// Mock relationship types query
		mockStore.AddQueryResponse(
			"CALL db.relationshipTypes()",
			[]map[string]interface{}{
				{"relationshipType": "SCHEDULED_ON"},
				{"relationshipType": "EXPOSES"},
			},
		)

		// Mock properties queries for each node type
		mockStore.AddQueryResponse(
			"Pod",
			[]map[string]interface{}{
				{"key": "name"},
				{"key": "namespace"},
				{"key": "status"},
			},
		)

		mockStore.AddQueryResponse(
			"Service",
			[]map[string]interface{}{
				{"key": "name"},
				{"key": "namespace"},
				{"key": "type"},
			},
		)

		mockStore.AddQueryResponse(
			"Node",
			[]map[string]interface{}{
				{"key": "name"},
				{"key": "status"},
			},
		)

		// Mock relationship properties
		mockStore.AddQueryResponse(
			"SCHEDULED_ON",
			[]map[string]interface{}{
				{"key": "created_at"},
			},
		)

		mockStore.AddQueryResponse(
			"EXPOSES",
			[]map[string]interface{}{
				{"key": "port"},
			},
		)

		// Mock triplets query (matches "MATCH (a)-[r]->(b)")
		mockStore.AddQueryResponse(
			"MATCH (a)-[r]->(b)",
			[]map[string]interface{}{
				{"fromLabel": "Pod", "relType": "SCHEDULED_ON", "toLabel": "Node"},
				{"fromLabel": "Service", "relType": "EXPOSES", "toLabel": "Pod"},
			},
		)

		output, err := StructureTool(ctx, mockStore, logger)
		if err != nil {
			t.Fatalf("StructureTool() error = %v", err)
		}

		if len(output.NodeTypes) != 3 {
			t.Errorf("StructureTool() node types count = %d, want 3", len(output.NodeTypes))
		}

		if len(output.RelationshipTypes) != 2 {
			t.Errorf("StructureTool() relationship types count = %d, want 2", len(output.RelationshipTypes))
		}

		if len(output.SchemaTriplets) != 2 {
			t.Errorf("StructureTool() triplets count = %d, want 2", len(output.SchemaTriplets))
		}
	})

	t.Run("labels query error", func(t *testing.T) {
		mockStore := kktesting.NewMockGraphStore()
		mockStore.SetQueryError("database connection failed")

		_, err := StructureTool(ctx, mockStore, logger)
		if err == nil {
			t.Error("StructureTool() expected error from labels query, got nil")
		}
	})
}
