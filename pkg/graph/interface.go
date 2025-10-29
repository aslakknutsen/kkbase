package graph

import (
	"context"
	"time"
)

// GraphStore defines the interface for interacting with the knowledge graph database
type GraphStore interface {
	// UpsertNode creates or updates a node in the graph
	UpsertNode(ctx context.Context, nodeType, id string, properties map[string]interface{}) error

	// DeleteNode removes a node from the graph
	DeleteNode(ctx context.Context, nodeType, id string) error

	// UpsertEdge creates or updates an edge between two nodes
	UpsertEdge(ctx context.Context, fromType, fromID, edgeType, toType, toID string, properties map[string]interface{}) error

	// DeleteEdge removes an edge from the graph
	DeleteEdge(ctx context.Context, edgeID string) error

	// DeleteEdgesByNode removes all edges connected to a node
	DeleteEdgesByNode(ctx context.Context, nodeType, nodeID string) error

	// DeleteEdgesByTypeAndNode removes specific edge types connected to a node
	DeleteEdgesByTypeAndNode(ctx context.Context, nodeType, nodeID string, edgeTypes []string) error

	// Query executes a custom query against the graph database
	Query(ctx context.Context, query string, params map[string]interface{}) ([]map[string]interface{}, error)

	// Close closes the connection to the graph database
	Close() error

	// HealthCheck verifies the database connection is healthy
	HealthCheck(ctx context.Context) error

	// GetPlaceholderNodes returns placeholder nodes for diagnostics
	GetPlaceholderNodes(ctx context.Context, nodeType string) ([]map[string]interface{}, error)

	// CleanupOrphanedPlaceholders removes old placeholder nodes with no relationships
	CleanupOrphanedPlaceholders(ctx context.Context, olderThan time.Duration) error
}
