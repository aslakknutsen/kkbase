package testing

import (
	"fmt"
	"reflect"
	"testing"
)

// AssertNodeCreated verifies that a node was created with the expected properties
func AssertNodeCreated(t *testing.T, mock *MockGraphStore, nodeType, id string, expectedProps map[string]interface{}) {
	t.Helper()

	node := mock.FindNode(nodeType, id)
	if node == nil {
		t.Errorf("Expected node not found: type=%s, id=%s", nodeType, id)
		t.Logf("Available nodes:")
		for i, n := range mock.Nodes {
			t.Logf("  [%d] type=%s, id=%s", i, n.NodeType, n.ID)
		}
		return
	}

	// If expectedProps is nil, we don't check properties
	if expectedProps == nil {
		return
	}

	// Check that all expected properties are present and match
	for key, expectedValue := range expectedProps {
		actualValue, exists := node.Properties[key]
		if !exists {
			t.Errorf("Node %s/%s missing expected property: %s", nodeType, id, key)
			continue
		}

		if !reflect.DeepEqual(actualValue, expectedValue) {
			t.Errorf("Node %s/%s property %s mismatch:\n  expected: %v (%T)\n  actual:   %v (%T)",
				nodeType, id, key, expectedValue, expectedValue, actualValue, actualValue)
		}
	}
}

// AssertEdgeCreated verifies that an edge was created with the expected properties
func AssertEdgeCreated(t *testing.T, mock *MockGraphStore, fromType, fromID, edgeType, toType, toID string, expectedProps map[string]interface{}) {
	t.Helper()

	edge := mock.FindEdge(fromType, fromID, edgeType, toType, toID)
	if edge == nil {
		t.Errorf("Expected edge not found: %s/%s -[%s]-> %s/%s", fromType, fromID, edgeType, toType, toID)
		t.Logf("Available edges:")
		for i, e := range mock.Edges {
			t.Logf("  [%d] %s/%s -[%s]-> %s/%s", i, e.FromType, e.FromID, e.EdgeType, e.ToType, e.ToID)
		}
		return
	}

	// If expectedProps is nil, we don't check properties
	if expectedProps == nil {
		return
	}

	// Check that all expected properties are present and match
	for key, expectedValue := range expectedProps {
		actualValue, exists := edge.Properties[key]
		if !exists {
			t.Errorf("Edge %s/%s -[%s]-> %s/%s missing expected property: %s",
				fromType, fromID, edgeType, toType, toID, key)
			continue
		}

		if !reflect.DeepEqual(actualValue, expectedValue) {
			t.Errorf("Edge %s/%s -[%s]-> %s/%s property %s mismatch:\n  expected: %v (%T)\n  actual:   %v (%T)",
				fromType, fromID, edgeType, toType, toID, key, expectedValue, expectedValue, actualValue, actualValue)
		}
	}
}

// AssertNodeCount verifies the number of nodes of a specific type
func AssertNodeCount(t *testing.T, mock *MockGraphStore, nodeType string, expectedCount int) {
	t.Helper()

	nodes := mock.GetNodesByType(nodeType)
	actualCount := len(nodes)

	if actualCount != expectedCount {
		t.Errorf("Node count mismatch for type %s: expected %d, got %d", nodeType, expectedCount, actualCount)
		if actualCount > 0 {
			t.Logf("Actual nodes of type %s:", nodeType)
			for i, node := range nodes {
				t.Logf("  [%d] id=%s", i, node.ID)
			}
		}
	}
}

// AssertEdgeCount verifies the number of edges of a specific type
func AssertEdgeCount(t *testing.T, mock *MockGraphStore, edgeType string, expectedCount int) {
	t.Helper()

	edges := mock.GetEdgesByType(edgeType)
	actualCount := len(edges)

	if actualCount != expectedCount {
		t.Errorf("Edge count mismatch for type %s: expected %d, got %d", edgeType, expectedCount, actualCount)
		if actualCount > 0 {
			t.Logf("Actual edges of type %s:", edgeType)
			for i, edge := range edges {
				t.Logf("  [%d] %s/%s -> %s/%s", i, edge.FromType, edge.FromID, edge.ToType, edge.ToID)
			}
		}
	}
}

// AssertPropertyMatches compares properties with support for nested structures
func AssertPropertyMatches(t *testing.T, actual, expected map[string]interface{}, path string) {
	t.Helper()

	if path == "" {
		path = "root"
	}

	for key, expectedValue := range expected {
		actualValue, exists := actual[key]
		if !exists {
			t.Errorf("Property missing at %s.%s", path, key)
			continue
		}

		currentPath := fmt.Sprintf("%s.%s", path, key)

		switch expectedV := expectedValue.(type) {
		case map[string]interface{}:
			// Recursively check nested maps
			if actualMap, ok := actualValue.(map[string]interface{}); ok {
				AssertPropertyMatches(t, actualMap, expectedV, currentPath)
			} else {
				t.Errorf("Property at %s is not a map: %T", currentPath, actualValue)
			}
		default:
			if !reflect.DeepEqual(actualValue, expectedValue) {
				t.Errorf("Property at %s mismatch:\n  expected: %v (%T)\n  actual:   %v (%T)",
					currentPath, expectedValue, expectedValue, actualValue, actualValue)
			}
		}
	}
}

// AssertTotalNodeCount verifies the total number of nodes created
func AssertTotalNodeCount(t *testing.T, mock *MockGraphStore, expectedCount int) {
	t.Helper()

	actualCount := len(mock.Nodes)
	if actualCount != expectedCount {
		t.Errorf("Total node count mismatch: expected %d, got %d", expectedCount, actualCount)
		t.Logf("All nodes:")
		for i, node := range mock.Nodes {
			t.Logf("  [%d] type=%s, id=%s", i, node.NodeType, node.ID)
		}
	}
}

// AssertTotalEdgeCount verifies the total number of edges created
func AssertTotalEdgeCount(t *testing.T, mock *MockGraphStore, expectedCount int) {
	t.Helper()

	actualCount := len(mock.Edges)
	if actualCount != expectedCount {
		t.Errorf("Total edge count mismatch: expected %d, got %d", expectedCount, actualCount)
		t.Logf("All edges:")
		for i, edge := range mock.Edges {
			t.Logf("  [%d] %s/%s -[%s]-> %s/%s", i, edge.FromType, edge.FromID, edge.EdgeType, edge.ToType, edge.ToID)
		}
	}
}
