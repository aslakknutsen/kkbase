package neo4j

import (
	"context"
	"fmt"
	"time"

	"github.com/kagenti/kkbase/pkg/graph"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"go.uber.org/zap"
)

// Config holds Neo4j connection configuration
type Config struct {
	URI        string
	Username   string
	Password   string
	Database   string
	MaxRetries int
	RetryDelay time.Duration
}

// Store implements the GraphStore interface for Neo4j
type Store struct {
	driver     neo4j.DriverWithContext
	database   string
	logger     *zap.Logger
	maxRetries int
	retryDelay time.Duration
}

// NewStore creates a new Neo4j graph store
func NewStore(cfg Config, logger *zap.Logger) (graph.GraphStore, error) {
	driver, err := neo4j.NewDriverWithContext(
		cfg.URI,
		neo4j.BasicAuth(cfg.Username, cfg.Password, ""),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Neo4j driver: %w", err)
	}

	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = 3
	}
	if cfg.RetryDelay == 0 {
		cfg.RetryDelay = time.Second
	}

	store := &Store{
		driver:     driver,
		database:   cfg.Database,
		logger:     logger,
		maxRetries: cfg.MaxRetries,
		retryDelay: cfg.RetryDelay,
	}

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := store.HealthCheck(ctx); err != nil {
		driver.Close(context.Background())
		return nil, fmt.Errorf("failed to verify Neo4j connection: %w", err)
	}

	// Create indexes for better performance
	if err := store.createIndexes(ctx); err != nil {
		driver.Close(context.Background())
		return nil, fmt.Errorf("failed to create indexes: %w", err)
	}

	return store, nil
}

// createIndexes creates necessary indexes in Neo4j
func (s *Store) createIndexes(ctx context.Context) error {
	session := s.driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: s.database})
	defer session.Close(ctx)

	nodeTypes := []string{
		"Cluster", "Node", "Pod", "Container", "Deployment", "ReplicaSet",
		"StatefulSet", "DaemonSet", "Service", "Ingress", "Endpoint",
		"NetworkPolicy", "PersistentVolume", "PersistentVolumeClaim",
		"StorageClass", "ConfigMap", "Secret", "K8sEvent", "Namespace",
		"Trace", "Span", "ServiceCall",
	}

	for _, nodeType := range nodeTypes {
		query := fmt.Sprintf("CREATE INDEX IF NOT EXISTS FOR (n:%s) ON (n.id)", nodeType)
		_, err := session.Run(ctx, query, nil)
		if err != nil {
			return fmt.Errorf("failed to create index for %s: %w", nodeType, err)
		}
	}

	// Create index on placeholder field for each node type for efficient querying of placeholder nodes
	for _, nodeType := range nodeTypes {
		placeholderIndexQuery := fmt.Sprintf("CREATE INDEX IF NOT EXISTS FOR (n:%s) ON (n.placeholder)", nodeType)
		_, err := session.Run(ctx, placeholderIndexQuery, nil)
		if err != nil {
			return fmt.Errorf("failed to create placeholder index for %s: %w", nodeType, err)
		}
	}

	return nil
}

// UpsertNode creates or updates a node in the graph
// When updating a placeholder node, it removes the placeholder flag
func (s *Store) UpsertNode(ctx context.Context, nodeType, id string, properties map[string]interface{}) error {
	return s.executeWithRetry(ctx, func(session neo4j.SessionWithContext) error {
		// Prepare properties
		props := make(map[string]interface{})
		for k, v := range properties {
			props[k] = v
		}
		props["id"] = id
		props["updated_at"] = time.Now().Unix()
		// When we have full data, mark as not a placeholder
		props["placeholder"] = false

		query := fmt.Sprintf(`
			MERGE (n:%s {id: $id})
			ON CREATE SET 
				n = $properties,
				n.created_at = timestamp()
			ON MATCH SET 
				n += $properties
		`, nodeType)

		params := map[string]interface{}{
			"id":         id,
			"properties": props,
		}

		_, err := session.Run(ctx, query, params)
		return err
	})
}

// DeleteNode removes a node from the graph
func (s *Store) DeleteNode(ctx context.Context, nodeType, id string) error {
	return s.executeWithRetry(ctx, func(session neo4j.SessionWithContext) error {
		query := fmt.Sprintf(`
			MATCH (n:%s {id: $id})
			DETACH DELETE n
		`, nodeType)

		params := map[string]interface{}{
			"id": id,
		}

		_, err := session.Run(ctx, query, params)
		return err
	})
}

// UpsertEdge creates or updates an edge between two nodes
// Uses MERGE to ensure both nodes exist before creating the relationship
// If nodes don't exist, they are created as placeholders
func (s *Store) UpsertEdge(ctx context.Context, fromType, fromID, edgeType, toType, toID string, properties map[string]interface{}) error {
	return s.executeWithRetry(ctx, func(session neo4j.SessionWithContext) error {
		// Prepare properties
		props := make(map[string]interface{})
		for k, v := range properties {
			props[k] = v
		}
		props["updated_at"] = time.Now().Unix()

		updatedAt := time.Now().Unix()

		// Use MERGE for both nodes to create placeholders if they don't exist
		// This handles out-of-order data arrival gracefully
		query := fmt.Sprintf(`
			MERGE (from:%s {id: $fromID})
			ON CREATE SET 
				from.placeholder = true,
				from.created_at = timestamp(),
				from.updated_at = $updated_at
			MERGE (to:%s {id: $toID})
			ON CREATE SET 
				to.placeholder = true,
				to.created_at = timestamp(),
				to.updated_at = $updated_at
			MERGE (from)-[r:%s]->(to)
			SET r += $properties
			RETURN r
		`, fromType, toType, edgeType)

		params := map[string]interface{}{
			"fromID":     fromID,
			"toID":       toID,
			"properties": props,
			"updated_at": updatedAt,
		}

		result, err := session.Run(ctx, query, params)
		if err != nil {
			return err
		}

		// Consume the result to ensure transaction commits
		if _, err := result.Consume(ctx); err != nil {
			return fmt.Errorf("failed to consume result: %w", err)
		}

		return nil
	})
}

// DeleteEdge removes an edge from the graph
func (s *Store) DeleteEdge(ctx context.Context, edgeID string) error {
	return s.executeWithRetry(ctx, func(session neo4j.SessionWithContext) error {
		query := `
			MATCH ()-[r]->()
			WHERE id(r) = $edgeID
			DELETE r
		`

		params := map[string]interface{}{
			"edgeID": edgeID,
		}

		_, err := session.Run(ctx, query, params)
		return err
	})
}

// DeleteEdgesByNode removes all edges connected to a node
func (s *Store) DeleteEdgesByNode(ctx context.Context, nodeType, nodeID string) error {
	return s.executeWithRetry(ctx, func(session neo4j.SessionWithContext) error {
		query := fmt.Sprintf(`
			MATCH (n:%s {id: $nodeID})-[r]-()
			DELETE r
		`, nodeType)

		params := map[string]interface{}{
			"nodeID": nodeID,
		}

		_, err := session.Run(ctx, query, params)
		return err
	})
}

// DeleteEdgesByTypeAndNode removes specific edge types connected to a node
func (s *Store) DeleteEdgesByTypeAndNode(ctx context.Context, nodeType, nodeID string, edgeTypes []string) error {
	if len(edgeTypes) == 0 {
		return nil
	}

	return s.executeWithRetry(ctx, func(session neo4j.SessionWithContext) error {
		// Build the relationship type filter: r:TYPE1|TYPE2|TYPE3
		typeFilter := edgeTypes[0]
		for i := 1; i < len(edgeTypes); i++ {
			typeFilter += "|" + edgeTypes[i]
		}

		query := fmt.Sprintf(`
			MATCH (n:%s {id: $nodeID})-[r:%s]-()
			DELETE r
		`, nodeType, typeFilter)

		params := map[string]interface{}{
			"nodeID": nodeID,
		}

		_, err := session.Run(ctx, query, params)
		return err
	})
}

// Query executes a custom query against the graph database
func (s *Store) Query(ctx context.Context, query string, params map[string]interface{}) ([]map[string]interface{}, error) {
	var results []map[string]interface{}

	err := s.executeWithRetry(ctx, func(session neo4j.SessionWithContext) error {
		result, err := session.Run(ctx, query, params)
		if err != nil {
			return err
		}

		records, err := result.Collect(ctx)
		if err != nil {
			return err
		}

		for _, record := range records {
			recordMap := make(map[string]interface{})
			for _, key := range record.Keys {
				recordMap[key] = record.Values[0]
			}
			results = append(results, recordMap)
		}

		return nil
	})

	return results, err
}

// HealthCheck verifies the database connection is healthy
func (s *Store) HealthCheck(ctx context.Context) error {
	return s.driver.VerifyConnectivity(ctx)
}

// Close closes the connection to the graph database
func (s *Store) Close() error {
	return s.driver.Close(context.Background())
}

// executeWithRetry executes a function with exponential backoff retry logic
func (s *Store) executeWithRetry(ctx context.Context, fn func(neo4j.SessionWithContext) error) error {
	var lastErr error

	for attempt := 0; attempt <= s.maxRetries; attempt++ {
		if attempt > 0 {
			delay := s.retryDelay * time.Duration(1<<uint(attempt-1))
			s.logger.Debug("retrying Neo4j operation",
				zap.Int("attempt", attempt),
				zap.Duration("delay", delay),
			)

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}

		session := s.driver.NewSession(ctx, neo4j.SessionConfig{
			DatabaseName: s.database,
			AccessMode:   neo4j.AccessModeWrite,
		})

		lastErr = fn(session)
		session.Close(ctx)

		if lastErr == nil {
			return nil
		}

		// Check if error is retryable
		if !isRetryableError(lastErr) {
			return lastErr
		}

		s.logger.Debug("Neo4j operation failed, will retry",
			zap.Error(lastErr),
			zap.Int("attempt", attempt),
		)
	}

	return fmt.Errorf("operation failed after %d retries: %w", s.maxRetries, lastErr)
}

// isRetryableError determines if an error should trigger a retry
func isRetryableError(err error) bool {
	// Check for transient errors, connection issues, etc.
	// For now, we'll retry on all errors except context cancellation
	return err != context.Canceled && err != context.DeadlineExceeded
}

// GetPlaceholderNodes returns all placeholder nodes of a given type
// Useful for diagnostics and monitoring
func (s *Store) GetPlaceholderNodes(ctx context.Context, nodeType string) ([]map[string]interface{}, error) {
	query := fmt.Sprintf(`
		MATCH (n:%s)
		WHERE n.placeholder = true
		RETURN n.id as id, n.updated_at as updated_at, labels(n) as labels
		ORDER BY n.updated_at DESC
		LIMIT 1000
	`, nodeType)

	return s.Query(ctx, query, nil)
}

// CleanupOrphanedPlaceholders removes placeholder nodes that have been
// unresolved for longer than the specified duration and have no relationships
func (s *Store) CleanupOrphanedPlaceholders(ctx context.Context, olderThan time.Duration) error {
	cutoffTime := time.Now().Add(-olderThan).Unix()

	query := `
		MATCH (n)
		WHERE n.placeholder = true 
			AND n.updated_at < $cutoff
			AND NOT (n)-[]-()
		DETACH DELETE n
		RETURN count(n) as deleted_count
	`

	result, err := s.Query(ctx, query, map[string]interface{}{
		"cutoff": cutoffTime,
	})

	if err != nil {
		return fmt.Errorf("failed to cleanup orphaned placeholders: %w", err)
	}

	if len(result) > 0 {
		if deletedCount, ok := result[0]["deleted_count"].(int64); ok {
			s.logger.Info("cleaned up orphaned placeholder nodes",
				zap.Int64("deleted_count", deletedCount))
		}
	}

	return nil
}
