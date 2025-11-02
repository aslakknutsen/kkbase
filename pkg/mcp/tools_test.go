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

		// Mock Query 1: Get node labels
		mockStore.AddQueryResponse(
			"CALL db.labels()",
			[]map[string]interface{}{
				{"label": "Pod"},
				{"label": "Service"},
				{"label": "Node"},
			},
		)

		// Mock Query 2: Get relationship types
		mockStore.AddQueryResponse(
			"CALL db.relationshipTypes()",
			[]map[string]interface{}{
				{"relationshipType": "SCHEDULED_ON"},
				{"relationshipType": "EXPOSES"},
			},
		)

		// Mock Query 3: Get properties for Pod
		mockStore.AddQueryResponse(
			"MATCH (n:Pod)",
			[]map[string]interface{}{
				{"key": "name"},
				{"key": "namespace"},
				{"key": "status"},
			},
		)

		// Mock Query 4: Get properties for Service
		mockStore.AddQueryResponse(
			"MATCH (n:Service)",
			[]map[string]interface{}{
				{"key": "name"},
				{"key": "namespace"},
				{"key": "type"},
			},
		)

		// Mock Query 5: Get properties for Node
		mockStore.AddQueryResponse(
			"MATCH (n:Node)",
			[]map[string]interface{}{
				{"key": "name"},
				{"key": "status"},
			},
		)

		// Mock Query 6: Get properties for SCHEDULED_ON relationship
		mockStore.AddQueryResponse(
			"MATCH ()-[r:SCHEDULED_ON]->()",
			[]map[string]interface{}{
				{"key": "created_at"},
			},
		)

		// Mock Query 7: Get properties for EXPOSES relationship
		mockStore.AddQueryResponse(
			"MATCH ()-[r:EXPOSES]->()",
			[]map[string]interface{}{
				{"key": "port"},
			},
		)

		// Mock Query 8-13: Schema triplets for each node-relationship combination
		// Pod-SCHEDULED_ON
		mockStore.AddQueryResponse(
			"MATCH (a:Pod)-[r:SCHEDULED_ON]->(b)",
			[]map[string]interface{}{
				{"toLabel": "Node"},
			},
		)

		// Pod-EXPOSES
		mockStore.AddQueryResponse(
			"MATCH (a:Pod)-[r:EXPOSES]->(b)",
			[]map[string]interface{}{},
		)

		// Service-SCHEDULED_ON
		mockStore.AddQueryResponse(
			"MATCH (a:Service)-[r:SCHEDULED_ON]->(b)",
			[]map[string]interface{}{},
		)

		// Service-EXPOSES
		mockStore.AddQueryResponse(
			"MATCH (a:Service)-[r:EXPOSES]->(b)",
			[]map[string]interface{}{
				{"toLabel": "Pod"},
			},
		)

		// Node-SCHEDULED_ON
		mockStore.AddQueryResponse(
			"MATCH (a:Node)-[r:SCHEDULED_ON]->(b)",
			[]map[string]interface{}{},
		)

		// Node-EXPOSES
		mockStore.AddQueryResponse(
			"MATCH (a:Node)-[r:EXPOSES]->(b)",
			[]map[string]interface{}{},
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

		// Verify node properties
		if props, ok := output.NodeProperties["Pod"]; !ok || len(props) != 3 {
			t.Errorf("StructureTool() Pod properties = %v, want 3 properties", props)
		}

		if props, ok := output.NodeProperties["Service"]; !ok || len(props) != 3 {
			t.Errorf("StructureTool() Service properties = %v, want 3 properties", props)
		}

		if props, ok := output.NodeProperties["Node"]; !ok || len(props) != 2 {
			t.Errorf("StructureTool() Node properties = %v, want 2 properties", props)
		}

		// Verify relationship properties
		if props, ok := output.RelationshipProperties["SCHEDULED_ON"]; !ok || len(props) != 1 {
			t.Errorf("StructureTool() SCHEDULED_ON properties = %v, want 1 property", props)
		}

		if props, ok := output.RelationshipProperties["EXPOSES"]; !ok || len(props) != 1 {
			t.Errorf("StructureTool() EXPOSES properties = %v, want 1 property", props)
		}
	})

	t.Run("node properties query error", func(t *testing.T) {
		mockStore := kktesting.NewMockGraphStore()
		mockStore.SetQueryError("database connection failed")

		_, err := StructureTool(ctx, mockStore, logger)
		if err == nil {
			t.Error("StructureTool() expected error from node properties query, got nil")
		}
	})
}
