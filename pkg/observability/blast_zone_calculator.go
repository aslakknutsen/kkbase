package observability

import (
	"context"
	"fmt"
	"time"

	"github.com/aslakknutsen/kkbase/pkg/graph"
	"go.uber.org/zap"
)

// BlastZoneCalculator computes the blast zone graph for an investigation session
type BlastZoneCalculator struct {
	graphStore graph.GraphStore
	logger     *zap.Logger
	maxHops    int
	maxNodes   int
}

// NewBlastZoneCalculator creates a new blast zone calculator
func NewBlastZoneCalculator(graphStore graph.GraphStore, logger *zap.Logger) *BlastZoneCalculator {
	return &BlastZoneCalculator{
		graphStore: graphStore,
		logger:     logger,
		maxHops:    3,   // Default: 3 hops
		maxNodes:   200, // Default: limit to 200 nodes
	}
}

// Calculate computes the current blast zone for a session
func (bzc *BlastZoneCalculator) Calculate(ctx context.Context, sessionID string) (*BlastZoneSnapshot, error) {
	bzc.logger.Debug("calculating blast zone",
		zap.String("session_id", sessionID))

	// Step 1: Get all resources affected by findings in this session
	affectedQuery := `
		MATCH (s:AgentSession {id: $session_id})-[:HAS_FINDING]->(f:Finding)-[:AFFECTS]->(affected)
		RETURN DISTINCT affected.id as resource_id, labels(affected)[0] as resource_type, affected.name as name
	`

	affectedResults, err := bzc.graphStore.Query(ctx, affectedQuery, map[string]interface{}{
		"session_id": sessionID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query affected resources: %w", err)
	}

	if len(affectedResults) == 0 {
		bzc.logger.Debug("no affected resources found", zap.String("session_id", sessionID))
		return &BlastZoneSnapshot{
			SessionID:     sessionID,
			Timestamp:     time.Now(),
			TriggerEvent:  "no_findings",
			Nodes:         []BlastZoneNode{},
			Edges:         []BlastZoneEdge{},
			ImpactRadius:  0,
			AffectedCount: 0,
		}, nil
	}

	// Step 2: Build blast zone graph using APOC path expansion
	// Note: This query uses APOC if available, falls back to manual expansion if not
	blastZoneQuery := `
		MATCH (s:AgentSession {id: $session_id})-[:HAS_FINDING]->(f:Finding)-[:AFFECTS]->(affected)
		
		// Try to use APOC for efficient path expansion
		CALL apoc.path.subgraphAll(affected, {
			relationshipFilter: 'CALLS|FAILED_CALL_TO|MANAGES|SELECTS_PODS|FORWARDS_TO|IN_NAMESPACE|IMPLEMENTED_BY',
			minLevel: 0,
			maxLevel: $max_hops,
			limit: $max_nodes
		}) YIELD nodes, relationships
		
		// Determine node status based on findings and failures
		UNWIND nodes as node
		OPTIONAL MATCH (node)-[fail:FAILED_CALL_TO]->()
		OPTIONAL MATCH (f2:Finding)-[:AFFECTS]->(node)
		
		WITH node, fail, f2,
			 CASE 
			   WHEN f2 IS NOT NULL THEN 'failed'
			   WHEN fail IS NOT NULL THEN 'degraded'
			   WHEN (node:Pod AND node.status <> 'Running') THEN 'degraded'
			   ELSE 'healthy'
			 END as status,
			 relationships
		
		RETURN collect(DISTINCT {
			id: node.id,
			label: coalesce(node.name, node.id),
			type: labels(node)[0],
			status: status,
			properties: properties(node)
		}) as nodes,
		[rel IN relationships | {
			source: startNode(rel).id,
			target: endNode(rel).id,
			type: type(rel),
			status: CASE WHEN type(rel) = 'FAILED_CALL_TO' THEN 'failing' ELSE 'ok' END,
			properties: properties(rel)
		}] as edges
	`

	results, err := bzc.graphStore.Query(ctx, blastZoneQuery, map[string]interface{}{
		"session_id": sessionID,
		"max_hops":   bzc.maxHops,
		"max_nodes":  bzc.maxNodes,
	})

	// If APOC is not available, fall back to manual expansion
	if err != nil && (containsString(err.Error(), "apoc") || containsString(err.Error(), "procedure")) {
		bzc.logger.Debug("APOC not available, using fallback query")
		return bzc.calculateWithoutAPOC(ctx, sessionID, affectedResults)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to calculate blast zone: %w", err)
	}

	if len(results) == 0 {
		return bzc.calculateWithoutAPOC(ctx, sessionID, affectedResults)
	}

	// Parse results
	result := results[0]
	nodes := parseBlastZoneNodes(result["nodes"])
	edges := parseBlastZoneEdges(result["edges"])

	snapshot := &BlastZoneSnapshot{
		SessionID:     sessionID,
		Timestamp:     time.Now(),
		TriggerEvent:  "calculation_complete",
		Nodes:         nodes,
		Edges:         edges,
		ImpactRadius:  bzc.maxHops,
		AffectedCount: len(nodes),
	}

	bzc.logger.Info("blast zone calculated",
		zap.String("session_id", sessionID),
		zap.Int("nodes", len(nodes)),
		zap.Int("edges", len(edges)))

	return snapshot, nil
}

// calculateWithoutAPOC performs blast zone calculation without APOC
func (bzc *BlastZoneCalculator) calculateWithoutAPOC(ctx context.Context, sessionID string, affectedResources []map[string]interface{}) (*BlastZoneSnapshot, error) {
	bzc.logger.Debug("calculating blast zone without APOC",
		zap.String("session_id", sessionID))

	// Manual multi-hop expansion with proper edge deduplication
	query := `
		MATCH (s:AgentSession {id: $session_id})-[:HAS_FINDING]->(f:Finding)-[:AFFECTS]->(affected)
		
		// Get affected nodes and 1-3 hop neighbors
		MATCH path = (affected)-[r*0..3]-(connected)
		WHERE ALL(rel IN relationships(path) WHERE 
			type(rel) IN ['CALLS', 'FAILED_CALL_TO', 'MANAGES', 'SELECTS_PODS', 'FORWARDS_TO', 'IN_NAMESPACE', 'IMPLEMENTED_BY'])
		
		WITH DISTINCT connected as node
		LIMIT $max_nodes
		
		// Determine status
		OPTIONAL MATCH (node)-[fail:FAILED_CALL_TO]->()
		OPTIONAL MATCH (f2:Finding)-[:AFFECTS]->(node)
		
		WITH collect(DISTINCT {
			id: node.id,
			label: coalesce(node.name, node.id),
			type: labels(node)[0],
			status: CASE 
			   WHEN f2 IS NOT NULL THEN 'failed'
			   WHEN fail IS NOT NULL THEN 'degraded'
			   WHEN (node:Pod AND node.status <> 'Running') THEN 'degraded'
			   ELSE 'healthy'
			 END,
			properties: properties(node)
		}) as nodes
		
		// Now collect edges between the nodes we found
		MATCH (s:AgentSession {id: $session_id})-[:HAS_FINDING]->(f:Finding)-[:AFFECTS]->(affected)
		MATCH path = (affected)-[r*0..3]-(connected)
		WHERE ALL(rel IN relationships(path) WHERE 
			type(rel) IN ['CALLS', 'FAILED_CALL_TO', 'MANAGES', 'SELECTS_PODS', 'FORWARDS_TO', 'IN_NAMESPACE', 'IMPLEMENTED_BY'])
		
		UNWIND relationships(path) as rel
		WITH DISTINCT rel, nodes
		
		RETURN nodes,
		       collect(DISTINCT {
			source: startNode(rel).id,
			target: endNode(rel).id,
			type: type(rel),
			status: CASE WHEN type(rel) = 'FAILED_CALL_TO' THEN 'failing' ELSE 'ok' END
		}) as edges
	`

	results, err := bzc.graphStore.Query(ctx, query, map[string]interface{}{
		"session_id": sessionID,
		"max_nodes":  bzc.maxNodes,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to calculate blast zone (fallback): %w", err)
	}

	if len(results) == 0 {
		return &BlastZoneSnapshot{
			SessionID:     sessionID,
			Timestamp:     time.Now(),
			TriggerEvent:  "no_data",
			Nodes:         []BlastZoneNode{},
			Edges:         []BlastZoneEdge{},
			ImpactRadius:  0,
			AffectedCount: 0,
		}, nil
	}

	result := results[0]
	nodes := parseBlastZoneNodes(result["nodes"])
	edges := parseBlastZoneEdges(result["edges"])

	snapshot := &BlastZoneSnapshot{
		SessionID:     sessionID,
		Timestamp:     time.Now(),
		TriggerEvent:  "calculation_complete_fallback",
		Nodes:         nodes,
		Edges:         edges,
		ImpactRadius:  bzc.maxHops,
		AffectedCount: len(nodes),
	}

	bzc.logger.Info("blast zone calculated (fallback)",
		zap.String("session_id", sessionID),
		zap.Int("nodes", len(nodes)),
		zap.Int("edges", len(edges)))

	return snapshot, nil
}

// Helper functions

func parseBlastZoneNodes(data interface{}) []BlastZoneNode {
	nodes := []BlastZoneNode{}

	if nodeList, ok := data.([]interface{}); ok {
		for _, nodeData := range nodeList {
			if nodeMap, ok := nodeData.(map[string]interface{}); ok {
				node := BlastZoneNode{
					ID:     getStringField(nodeMap, "id"),
					Label:  getStringField(nodeMap, "label"),
					Type:   getStringField(nodeMap, "type"),
					Status: getStringField(nodeMap, "status"),
				}

				if props, ok := nodeMap["properties"].(map[string]interface{}); ok {
					node.Properties = props
				}

				nodes = append(nodes, node)
			}
		}
	}

	return nodes
}

func parseBlastZoneEdges(data interface{}) []BlastZoneEdge {
	edges := []BlastZoneEdge{}

	if edgeList, ok := data.([]interface{}); ok {
		for _, edgeData := range edgeList {
			if edgeMap, ok := edgeData.(map[string]interface{}); ok {
				edge := parseBlastZoneEdge(edgeMap)
				edges = append(edges, edge)
			}
		}
	}

	return edges
}

func parseBlastZoneEdge(edgeMap map[string]interface{}) BlastZoneEdge {
	edge := BlastZoneEdge{
		Source: getStringField(edgeMap, "source"),
		Target: getStringField(edgeMap, "target"),
		Type:   getStringField(edgeMap, "type"),
		Status: getStringField(edgeMap, "status"),
	}

	if props, ok := edgeMap["properties"].(map[string]interface{}); ok {
		edge.Properties = props
	}

	return edge
}

func getStringField(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok && val != nil {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
