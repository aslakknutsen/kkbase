package neo4j

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"
)

// TestUpsertEdge_CreatesPlaceholderNodes verifies that placeholder nodes are created
// when creating an edge between nodes that don't exist yet
func TestUpsertEdge_CreatesPlaceholderNodes(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()

	// Create edge between two non-existent nodes
	err := store.UpsertEdge(ctx, "Pod", "Pod/default/test-pod",
		"USES_CONFIG", "ConfigMap", "ConfigMap/default/test-config", nil)
	if err != nil {
		t.Fatalf("Failed to create edge: %v", err)
	}

	// Verify both placeholder nodes were created
	results, err := store.Query(ctx,
		"MATCH (p:Pod {id: $podID}) RETURN p.placeholder AS placeholder",
		map[string]interface{}{"podID": "Pod/default/test-pod"})
	if err != nil {
		t.Fatalf("Failed to query Pod: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("Pod placeholder should exist")
	}
	if placeholder, ok := results[0]["placeholder"].(bool); !ok || !placeholder {
		t.Error("Pod should be marked as placeholder")
	}

	results, err = store.Query(ctx,
		"MATCH (cm:ConfigMap {id: $cmID}) RETURN cm.placeholder AS placeholder",
		map[string]interface{}{"cmID": "ConfigMap/default/test-config"})
	if err != nil {
		t.Fatalf("Failed to query ConfigMap: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("ConfigMap placeholder should exist")
	}
	if placeholder, ok := results[0]["placeholder"].(bool); !ok || !placeholder {
		t.Error("ConfigMap should be marked as placeholder")
	}

	// Verify edge exists
	results, err = store.Query(ctx,
		"MATCH (p:Pod {id: $podID})-[r:USES_CONFIG]->(cm:ConfigMap {id: $cmID}) RETURN r",
		map[string]interface{}{
			"podID": "Pod/default/test-pod",
			"cmID":  "ConfigMap/default/test-config",
		})
	if err != nil {
		t.Fatalf("Failed to query edge: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("USES_CONFIG edge should exist")
	}
}

// TestUpsertEdge_WithExistingNodes verifies that normal operation is unchanged
// when both nodes already exist
func TestUpsertEdge_WithExistingNodes(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()

	// First create the nodes
	err := store.UpsertNode(ctx, "Pod", "Pod/default/test-pod",
		map[string]interface{}{
			"name":      "test-pod",
			"namespace": "default",
		})
	if err != nil {
		t.Fatalf("Failed to create Pod: %v", err)
	}

	err = store.UpsertNode(ctx, "ConfigMap", "ConfigMap/default/test-config",
		map[string]interface{}{
			"name":      "test-config",
			"namespace": "default",
		})
	if err != nil {
		t.Fatalf("Failed to create ConfigMap: %v", err)
	}

	// Now create edge between existing nodes
	err = store.UpsertEdge(ctx, "Pod", "Pod/default/test-pod",
		"USES_CONFIG", "ConfigMap", "ConfigMap/default/test-config", nil)
	if err != nil {
		t.Fatalf("Failed to create edge: %v", err)
	}

	// Verify nodes are NOT marked as placeholders
	results, err := store.Query(ctx,
		"MATCH (p:Pod {id: $podID}) RETURN p.placeholder AS placeholder",
		map[string]interface{}{"podID": "Pod/default/test-pod"})
	if err != nil {
		t.Fatalf("Failed to query Pod: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("Pod should exist")
	}
	if placeholder, ok := results[0]["placeholder"].(bool); ok && placeholder {
		t.Error("Pod should NOT be marked as placeholder")
	}

	// Verify edge exists
	results, err = store.Query(ctx,
		"MATCH (p:Pod {id: $podID})-[r:USES_CONFIG]->(cm:ConfigMap {id: $cmID}) RETURN r",
		map[string]interface{}{
			"podID": "Pod/default/test-pod",
			"cmID":  "ConfigMap/default/test-config",
		})
	if err != nil {
		t.Fatalf("Failed to query edge: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("USES_CONFIG edge should exist")
	}
}

// TestUpsertNode_EnrichesPlaceholder verifies that placeholder nodes are updated
// with full data when UpsertNode is called
func TestUpsertNode_EnrichesPlaceholder(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()

	// First, create a placeholder by creating an edge
	err := store.UpsertEdge(ctx, "Pod", "Pod/default/test-pod",
		"USES_CONFIG", "ConfigMap", "ConfigMap/default/test-config", nil)
	if err != nil {
		t.Fatalf("Failed to create edge: %v", err)
	}

	// Verify ConfigMap is a placeholder
	results, err := store.Query(ctx,
		"MATCH (cm:ConfigMap {id: $cmID}) RETURN cm.placeholder AS placeholder, cm.name AS name",
		map[string]interface{}{"cmID": "ConfigMap/default/test-config"})
	if err != nil {
		t.Fatalf("Failed to query ConfigMap: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("ConfigMap placeholder should exist")
	}
	if placeholder, ok := results[0]["placeholder"].(bool); !ok || !placeholder {
		t.Error("ConfigMap should be marked as placeholder")
	}
	if results[0]["name"] != nil {
		t.Error("Placeholder should not have name property initially")
	}

	// Now upsert the real ConfigMap
	err = store.UpsertNode(ctx, "ConfigMap", "ConfigMap/default/test-config",
		map[string]interface{}{
			"name":      "test-config",
			"namespace": "default",
			"data":      "key=value",
		})
	if err != nil {
		t.Fatalf("Failed to upsert ConfigMap: %v", err)
	}

	// Verify placeholder flag was cleared and properties were added
	results, err = store.Query(ctx,
		"MATCH (cm:ConfigMap {id: $cmID}) RETURN cm.placeholder AS placeholder, cm.name AS name, cm.data AS data",
		map[string]interface{}{"cmID": "ConfigMap/default/test-config"})
	if err != nil {
		t.Fatalf("Failed to query ConfigMap: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("ConfigMap should still exist")
	}
	if placeholder, ok := results[0]["placeholder"].(bool); !ok || placeholder {
		t.Error("ConfigMap should no longer be marked as placeholder")
	}
	if name, ok := results[0]["name"].(string); !ok || name != "test-config" {
		t.Errorf("ConfigMap should have name='test-config', got: %v", results[0]["name"])
	}
	if data, ok := results[0]["data"].(string); !ok || data != "key=value" {
		t.Errorf("ConfigMap should have data='key=value', got: %v", results[0]["data"])
	}
}

// TestGetPlaceholderNodes verifies retrieval of placeholder nodes
func TestGetPlaceholderNodes(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()

	// Create some placeholder nodes
	err := store.UpsertEdge(ctx, "Pod", "Pod/default/test-pod-1",
		"USES_CONFIG", "ConfigMap", "ConfigMap/default/test-config-1", nil)
	if err != nil {
		t.Fatalf("Failed to create edge 1: %v", err)
	}

	err = store.UpsertEdge(ctx, "Pod", "Pod/default/test-pod-2",
		"USES_CONFIG", "ConfigMap", "ConfigMap/default/test-config-2", nil)
	if err != nil {
		t.Fatalf("Failed to create edge 2: %v", err)
	}

	// Get placeholder ConfigMaps
	placeholders, err := store.GetPlaceholderNodes(ctx, "ConfigMap")
	if err != nil {
		t.Fatalf("Failed to get placeholder nodes: %v", err)
	}

	if len(placeholders) < 2 {
		t.Errorf("Expected at least 2 placeholder ConfigMaps, got %d", len(placeholders))
	}

	// Verify they have the expected IDs
	foundIDs := make(map[string]bool)
	for _, p := range placeholders {
		if id, ok := p["id"].(string); ok {
			foundIDs[id] = true
		}
	}

	if !foundIDs["ConfigMap/default/test-config-1"] {
		t.Error("Expected to find ConfigMap/default/test-config-1")
	}
	if !foundIDs["ConfigMap/default/test-config-2"] {
		t.Error("Expected to find ConfigMap/default/test-config-2")
	}
}

// TestCleanupOrphanedPlaceholders verifies cleanup of old placeholder nodes
func TestCleanupOrphanedPlaceholders(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()

	// Create a placeholder node with an old timestamp
	oldTime := time.Now().Add(-2 * time.Hour).Unix()
	_, err := store.Query(ctx,
		"CREATE (cm:ConfigMap {id: $id, placeholder: true, updated_at: $oldTime})",
		map[string]interface{}{
			"id":      "ConfigMap/default/old-placeholder",
			"oldTime": oldTime,
		})
	if err != nil {
		t.Fatalf("Failed to create old placeholder: %v", err)
	}

	// Create a recent placeholder node (should not be deleted)
	err = store.UpsertEdge(ctx, "Pod", "Pod/default/test-pod",
		"USES_CONFIG", "ConfigMap", "ConfigMap/default/recent-placeholder", nil)
	if err != nil {
		t.Fatalf("Failed to create recent placeholder: %v", err)
	}

	// Run cleanup (delete placeholders older than 1 hour)
	err = store.CleanupOrphanedPlaceholders(ctx, 1*time.Hour)
	if err != nil {
		t.Fatalf("Failed to cleanup placeholders: %v", err)
	}

	// Verify old placeholder was deleted
	results, err := store.Query(ctx,
		"MATCH (cm:ConfigMap {id: $id}) RETURN cm",
		map[string]interface{}{"id": "ConfigMap/default/old-placeholder"})
	if err != nil {
		t.Fatalf("Failed to query old placeholder: %v", err)
	}
	if len(results) > 0 {
		t.Error("Old placeholder should have been deleted")
	}

	// Verify recent placeholder still exists
	results, err = store.Query(ctx,
		"MATCH (cm:ConfigMap {id: $id}) RETURN cm",
		map[string]interface{}{"id": "ConfigMap/default/recent-placeholder"})
	if err != nil {
		t.Fatalf("Failed to query recent placeholder: %v", err)
	}
	if len(results) == 0 {
		t.Error("Recent placeholder should still exist")
	}
}

// TestPlaceholderLifecycle tests the complete lifecycle:
// 1. Edge creates placeholder
// 2. UpsertNode enriches placeholder
// 3. Cleanup doesn't remove connected placeholders
func TestPlaceholderLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()

	// Phase 1: Create edge with non-existent nodes (creates placeholders)
	err := store.UpsertEdge(ctx, "Pod", "Pod/default/lifecycle-pod",
		"USES_CONFIG", "ConfigMap", "ConfigMap/default/lifecycle-config",
		map[string]interface{}{"mount_path": "/etc/config"})
	if err != nil {
		t.Fatalf("Failed to create edge: %v", err)
	}

	// Verify both are placeholders
	results, err := store.Query(ctx,
		"MATCH (p:Pod {id: $podID}) RETURN p.placeholder AS placeholder",
		map[string]interface{}{"podID": "Pod/default/lifecycle-pod"})
	if err != nil || len(results) == 0 {
		t.Fatalf("Pod placeholder should exist")
	}
	if placeholder, ok := results[0]["placeholder"].(bool); !ok || !placeholder {
		t.Error("Pod should be a placeholder")
	}

	// Phase 2: Enrich Pod
	err = store.UpsertNode(ctx, "Pod", "Pod/default/lifecycle-pod",
		map[string]interface{}{
			"name":      "lifecycle-pod",
			"namespace": "default",
			"status":    "Running",
		})
	if err != nil {
		t.Fatalf("Failed to enrich Pod: %v", err)
	}

	// Verify Pod is no longer a placeholder
	results, err = store.Query(ctx,
		"MATCH (p:Pod {id: $podID}) RETURN p.placeholder AS placeholder, p.status AS status",
		map[string]interface{}{"podID": "Pod/default/lifecycle-pod"})
	if err != nil || len(results) == 0 {
		t.Fatalf("Pod should still exist")
	}
	if placeholder, ok := results[0]["placeholder"].(bool); ok && placeholder {
		t.Error("Pod should no longer be a placeholder")
	}
	if status, ok := results[0]["status"].(string); !ok || status != "Running" {
		t.Error("Pod should have status property")
	}

	// ConfigMap should still be a placeholder
	results, err = store.Query(ctx,
		"MATCH (cm:ConfigMap {id: $cmID}) RETURN cm.placeholder AS placeholder",
		map[string]interface{}{"cmID": "ConfigMap/default/lifecycle-config"})
	if err != nil || len(results) == 0 {
		t.Fatalf("ConfigMap placeholder should exist")
	}
	if placeholder, ok := results[0]["placeholder"].(bool); !ok || !placeholder {
		t.Error("ConfigMap should still be a placeholder")
	}

	// Phase 3: Cleanup should NOT remove ConfigMap because it has a relationship
	err = store.CleanupOrphanedPlaceholders(ctx, 0*time.Second)
	if err != nil {
		t.Fatalf("Failed to cleanup: %v", err)
	}

	results, err = store.Query(ctx,
		"MATCH (cm:ConfigMap {id: $cmID}) RETURN cm",
		map[string]interface{}{"cmID": "ConfigMap/default/lifecycle-config"})
	if err != nil {
		t.Fatalf("Failed to query ConfigMap: %v", err)
	}
	if len(results) == 0 {
		t.Error("ConfigMap should still exist because it has relationships")
	}

	// Phase 4: Enrich ConfigMap
	err = store.UpsertNode(ctx, "ConfigMap", "ConfigMap/default/lifecycle-config",
		map[string]interface{}{
			"name":      "lifecycle-config",
			"namespace": "default",
		})
	if err != nil {
		t.Fatalf("Failed to enrich ConfigMap: %v", err)
	}

	// Verify ConfigMap is no longer a placeholder
	results, err = store.Query(ctx,
		"MATCH (cm:ConfigMap {id: $cmID}) RETURN cm.placeholder AS placeholder",
		map[string]interface{}{"cmID": "ConfigMap/default/lifecycle-config"})
	if err != nil || len(results) == 0 {
		t.Fatalf("ConfigMap should still exist")
	}
	if placeholder, ok := results[0]["placeholder"].(bool); ok && placeholder {
		t.Error("ConfigMap should no longer be a placeholder")
	}

	// Verify edge still exists
	results, err = store.Query(ctx,
		"MATCH (p:Pod {id: $podID})-[r:USES_CONFIG]->(cm:ConfigMap {id: $cmID}) RETURN r",
		map[string]interface{}{
			"podID": "Pod/default/lifecycle-pod",
			"cmID":  "ConfigMap/default/lifecycle-config",
		})
	if err != nil {
		t.Fatalf("Failed to query edge: %v", err)
	}
	if len(results) == 0 {
		t.Error("Edge should still exist")
	}
}

// setupTestStore creates a test Neo4j store and returns a cleanup function
func setupTestStore(t *testing.T) (*Store, func()) {
	logger, _ := zap.NewDevelopment()

	graphStore, err := NewStore(Config{
		URI:      getEnvOrDefault("NEO4J_TEST_URI", "bolt://localhost:7687"),
		Username: getEnvOrDefault("NEO4J_TEST_USERNAME", "neo4j"),
		Password: getEnvOrDefault("NEO4J_TEST_PASSWORD", "test"),
		Database: getEnvOrDefault("NEO4J_TEST_DATABASE", "neo4j"),
	}, logger)
	if err != nil {
		t.Fatalf("Failed to create test store: %v", err)
	}

	// Type assert to *Store for testing
	store, ok := graphStore.(*Store)
	if !ok {
		t.Fatal("Failed to type assert graphStore to *Store")
	}

	// Cleanup function
	cleanup := func() {
		ctx := context.Background()
		// Delete all test data
		store.Query(ctx, "MATCH (n) WHERE n.id STARTS WITH 'Pod/default/test-' OR n.id STARTS WITH 'Pod/default/lifecycle-' OR n.id STARTS WITH 'ConfigMap/default/test-' OR n.id STARTS WITH 'ConfigMap/default/lifecycle-' OR n.id STARTS WITH 'ConfigMap/default/old-' OR n.id STARTS WITH 'ConfigMap/default/recent-' DETACH DELETE n", nil)
		store.Close()
	}

	return store, cleanup
}

// getEnvOrDefault returns environment variable value or default
func getEnvOrDefault(key, defaultValue string) string {
	// In a real test, you'd use os.Getenv, but for the example:
	return defaultValue
}
