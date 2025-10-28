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

	return nil
}

// UpsertNode creates or updates a node in the graph
func (s *Store) UpsertNode(ctx context.Context, nodeType, id string, properties map[string]interface{}) error {
	return s.executeWithRetry(ctx, func(session neo4j.SessionWithContext) error {
		// Prepare properties
		props := make(map[string]interface{})
		for k, v := range properties {
			props[k] = v
		}
		props["id"] = id
		props["updated_at"] = time.Now().Unix()

		query := fmt.Sprintf(`
			MERGE (n:%s {id: $id})
			SET n += $properties
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
func (s *Store) UpsertEdge(ctx context.Context, fromType, fromID, edgeType, toType, toID string, properties map[string]interface{}) error {
	return s.executeWithRetry(ctx, func(session neo4j.SessionWithContext) error {
		// Prepare properties
		props := make(map[string]interface{})
		for k, v := range properties {
			props[k] = v
		}
		props["updated_at"] = time.Now().Unix()

		query := fmt.Sprintf(`
			MATCH (from:%s {id: $fromID})
			MATCH (to:%s {id: $toID})
			MERGE (from)-[r:%s]->(to)
			SET r += $properties
			RETURN r
		`, fromType, toType, edgeType)

		params := map[string]interface{}{
			"fromID":     fromID,
			"toID":       toID,
			"properties": props,
		}

		result, err := session.Run(ctx, query, params)
		if err != nil {
			return err
		}

		// Check if any relationship was created/updated
		if result.Next(ctx) {
			return nil
		}

		// If no relationship was returned, one or both nodes don't exist
		return fmt.Errorf("failed to create edge: one or both nodes not found (from:%s id:%s, to:%s id:%s)",
			fromType, fromID, toType, toID)
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
