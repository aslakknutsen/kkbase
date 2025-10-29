package testing

import (
	"context"
	"sync"
	"time"
)

// NodeCall represents a recorded UpsertNode call
type NodeCall struct {
	NodeType   string
	ID         string
	Properties map[string]interface{}
}

// EdgeCall represents a recorded UpsertEdge call
type EdgeCall struct {
	FromType   string
	FromID     string
	EdgeType   string
	ToType     string
	ToID       string
	Properties map[string]interface{}
}

// DeleteNodeCall represents a recorded DeleteNode call
type DeleteNodeCall struct {
	NodeType string
	ID       string
}

// DeleteEdgeCall represents a recorded DeleteEdgesByNode call
type DeleteEdgeCall struct {
	NodeType string
	NodeID   string
}

// MockGraphStore is a mock implementation of graph.GraphStore that records all operations
type MockGraphStore struct {
	mu           sync.RWMutex
	Nodes        []NodeCall
	Edges        []EdgeCall
	DeletedNodes []DeleteNodeCall
	DeletedEdges []DeleteEdgeCall

	// Optional error injection for testing error paths
	UpsertNodeErr  error
	UpsertEdgeErr  error
	DeleteNodeErr  error
	DeleteEdgeErr  error
	QueryErr       error
	HealthCheckErr error
}

// NewMockGraphStore creates a new mock graph store
func NewMockGraphStore() *MockGraphStore {
	return &MockGraphStore{
		Nodes:        make([]NodeCall, 0),
		Edges:        make([]EdgeCall, 0),
		DeletedNodes: make([]DeleteNodeCall, 0),
		DeletedEdges: make([]DeleteEdgeCall, 0),
	}
}

// Reset clears all recorded operations
func (m *MockGraphStore) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Nodes = make([]NodeCall, 0)
	m.Edges = make([]EdgeCall, 0)
	m.DeletedNodes = make([]DeleteNodeCall, 0)
	m.DeletedEdges = make([]DeleteEdgeCall, 0)
}

// UpsertNode records a node upsert operation
func (m *MockGraphStore) UpsertNode(ctx context.Context, nodeType, id string, properties map[string]interface{}) error {
	if m.UpsertNodeErr != nil {
		return m.UpsertNodeErr
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Deep copy properties to avoid mutation issues
	propsCopy := make(map[string]interface{})
	for k, v := range properties {
		propsCopy[k] = v
	}

	m.Nodes = append(m.Nodes, NodeCall{
		NodeType:   nodeType,
		ID:         id,
		Properties: propsCopy,
	})

	return nil
}

// DeleteNode records a node deletion operation
func (m *MockGraphStore) DeleteNode(ctx context.Context, nodeType, id string) error {
	if m.DeleteNodeErr != nil {
		return m.DeleteNodeErr
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.DeletedNodes = append(m.DeletedNodes, DeleteNodeCall{
		NodeType: nodeType,
		ID:       id,
	})

	return nil
}

// UpsertEdge records an edge upsert operation
func (m *MockGraphStore) UpsertEdge(ctx context.Context, fromType, fromID, edgeType, toType, toID string, properties map[string]interface{}) error {
	if m.UpsertEdgeErr != nil {
		return m.UpsertEdgeErr
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Deep copy properties to avoid mutation issues
	var propsCopy map[string]interface{}
	if properties != nil {
		propsCopy = make(map[string]interface{})
		for k, v := range properties {
			propsCopy[k] = v
		}
	}

	m.Edges = append(m.Edges, EdgeCall{
		FromType:   fromType,
		FromID:     fromID,
		EdgeType:   edgeType,
		ToType:     toType,
		ToID:       toID,
		Properties: propsCopy,
	})

	return nil
}

// DeleteEdge records an edge deletion operation (by edge ID)
func (m *MockGraphStore) DeleteEdge(ctx context.Context, edgeID string) error {
	if m.DeleteEdgeErr != nil {
		return m.DeleteEdgeErr
	}
	// For simplicity, we don't track individual edge deletions
	return nil
}

// DeleteEdgesByNode records a deletion of all edges connected to a node
func (m *MockGraphStore) DeleteEdgesByNode(ctx context.Context, nodeType, nodeID string) error {
	if m.DeleteEdgeErr != nil {
		return m.DeleteEdgeErr
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.DeletedEdges = append(m.DeletedEdges, DeleteEdgeCall{
		NodeType: nodeType,
		NodeID:   nodeID,
	})

	return nil
}

// DeleteEdgesByTypeAndNode records a deletion of specific edge types connected to a node
func (m *MockGraphStore) DeleteEdgesByTypeAndNode(ctx context.Context, nodeType, nodeID string, edgeTypes []string) error {
	if m.DeleteEdgeErr != nil {
		return m.DeleteEdgeErr
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Record the deletion (same as DeleteEdgesByNode for mock purposes)
	m.DeletedEdges = append(m.DeletedEdges, DeleteEdgeCall{
		NodeType: nodeType,
		NodeID:   nodeID,
	})

	return nil
}

// Query is a no-op for the mock
func (m *MockGraphStore) Query(ctx context.Context, query string, params map[string]interface{}) ([]map[string]interface{}, error) {
	if m.QueryErr != nil {
		return nil, m.QueryErr
	}
	return nil, nil
}

// Close is a no-op for the mock
func (m *MockGraphStore) Close() error {
	return nil
}

// HealthCheck is a no-op for the mock
func (m *MockGraphStore) HealthCheck(ctx context.Context) error {
	if m.HealthCheckErr != nil {
		return m.HealthCheckErr
	}
	return nil
}

// GetPlaceholderNodes returns an empty list for the mock
// In tests, this is typically not needed as we're focused on recording operations
func (m *MockGraphStore) GetPlaceholderNodes(ctx context.Context, nodeType string) ([]map[string]interface{}, error) {
	// For mock purposes, return empty list
	// Tests can check recorded Nodes if needed
	return []map[string]interface{}{}, nil
}

// CleanupOrphanedPlaceholders is a no-op for the mock
// In tests, we don't need to simulate placeholder cleanup
func (m *MockGraphStore) CleanupOrphanedPlaceholders(ctx context.Context, olderThan time.Duration) error {
	// For mock purposes, do nothing
	return nil
}

// Helper methods for querying recorded operations

// GetNodesByType returns all recorded nodes of a specific type
func (m *MockGraphStore) GetNodesByType(nodeType string) []NodeCall {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []NodeCall
	for _, node := range m.Nodes {
		if node.NodeType == nodeType {
			result = append(result, node)
		}
	}
	return result
}

// GetEdgesByType returns all recorded edges of a specific type
func (m *MockGraphStore) GetEdgesByType(edgeType string) []EdgeCall {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []EdgeCall
	for _, edge := range m.Edges {
		if edge.EdgeType == edgeType {
			result = append(result, edge)
		}
	}
	return result
}

// FindNode finds a node by type and ID
func (m *MockGraphStore) FindNode(nodeType, id string) *NodeCall {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, node := range m.Nodes {
		if node.NodeType == nodeType && node.ID == id {
			return &node
		}
	}
	return nil
}

// FindEdge finds an edge by its components
func (m *MockGraphStore) FindEdge(fromType, fromID, edgeType, toType, toID string) *EdgeCall {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, edge := range m.Edges {
		if edge.FromType == fromType &&
			edge.FromID == fromID &&
			edge.EdgeType == edgeType &&
			edge.ToType == toType &&
			edge.ToID == toID {
			return &edge
		}
	}
	return nil
}
