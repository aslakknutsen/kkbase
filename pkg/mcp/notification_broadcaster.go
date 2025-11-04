package mcp

import (
	"net/http"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"
)

// NotificationBroadcaster manages broadcasting MCP notifications to connected clients via SSE
type NotificationBroadcaster struct {
	servers        map[string]*mcp.Server // Map of client ID to server instance (legacy)
	sseBroadcaster *SSEBroadcaster        // SSE broadcaster for push notifications
	mu             sync.RWMutex
	logger         *zap.Logger
}

// NewNotificationBroadcaster creates a new notification broadcaster
func NewNotificationBroadcaster(logger *zap.Logger) *NotificationBroadcaster {
	return &NotificationBroadcaster{
		servers:        make(map[string]*mcp.Server),
		sseBroadcaster: NewSSEBroadcaster(logger),
		logger:         logger,
	}
}

// RegisterClient registers an MCP server instance for a client
func (nb *NotificationBroadcaster) RegisterClient(clientID string, server *mcp.Server) {
	nb.mu.Lock()
	defer nb.mu.Unlock()

	nb.servers[clientID] = server
	nb.logger.Debug("client registered for notifications",
		zap.String("client_id", clientID),
		zap.Int("total_clients", len(nb.servers)))
}

// UnregisterClient removes a client from the broadcast list
func (nb *NotificationBroadcaster) UnregisterClient(clientID string) {
	nb.mu.Lock()
	defer nb.mu.Unlock()

	delete(nb.servers, clientID)
	nb.logger.Debug("client unregistered from notifications",
		zap.String("client_id", clientID),
		zap.Int("total_clients", len(nb.servers)))
}

// Emit broadcasts a notification to all connected clients via SSE
func (nb *NotificationBroadcaster) Emit(method string, params map[string]interface{}) {
	// Broadcast via SSE to all connected dashboard clients
	nb.sseBroadcaster.Broadcast(method, params)

	nb.logger.Info("notification broadcasted",
		zap.String("method", method),
		zap.Int("sse_connections", nb.sseBroadcaster.ConnectionCount()),
		zap.Any("params", params))
}

// EmitSessionCreated emits agent_session/created notification
func (nb *NotificationBroadcaster) EmitSessionCreated(sessionID, symptom string) {
	nb.Emit("agent_session/created", map[string]interface{}{
		"session_id": sessionID,
		"symptom":    symptom,
	})
}

// EmitQueryExecuted emits agent_session/query_executed notification
func (nb *NotificationBroadcaster) EmitQueryExecuted(sessionID, queryID string, findingCount int) {
	nb.Emit("agent_session/query_executed", map[string]interface{}{
		"session_id":    sessionID,
		"query_id":      queryID,
		"finding_count": findingCount,
	})
}

// EmitHypothesisUpdated emits agent_session/hypothesis_updated notification
func (nb *NotificationBroadcaster) EmitHypothesisUpdated(sessionID string, stage int, text string) {
	nb.Emit("agent_session/hypothesis_updated", map[string]interface{}{
		"session_id": sessionID,
		"stage":      stage,
		"text":       text,
	})
}

// EmitFindingDiscovered emits agent_session/finding_discovered notification
func (nb *NotificationBroadcaster) EmitFindingDiscovered(sessionID, findingID, findingType, severity string) {
	nb.Emit("agent_session/finding_discovered", map[string]interface{}{
		"session_id":   sessionID,
		"finding_id":   findingID,
		"finding_type": findingType,
		"severity":     severity,
	})
}

// EmitRecommendationRecorded emits agent_session/recommendation_recorded notification
func (nb *NotificationBroadcaster) EmitRecommendationRecorded(sessionID, recommendationID, priority, recType string) {
	nb.Emit("agent_session/recommendation_recorded", map[string]interface{}{
		"session_id":        sessionID,
		"recommendation_id": recommendationID,
		"priority":          priority,
		"type":              recType,
	})
}

// EmitBlastZoneUpdated emits agent_session/blast_zone_updated notification
func (nb *NotificationBroadcaster) EmitBlastZoneUpdated(sessionID string, nodeCount, edgeCount int) {
	nb.Emit("agent_session/blast_zone_updated", map[string]interface{}{
		"session_id": sessionID,
		"node_count": nodeCount,
		"edge_count": edgeCount,
	})
}

// EmitInvestigationSpawned emits agent_session/investigation_spawned notification
func (nb *NotificationBroadcaster) EmitInvestigationSpawned(sessionID, investigationID, resourceID string) {
	nb.Emit("agent_session/investigation_spawned", map[string]interface{}{
		"session_id":       sessionID,
		"investigation_id": investigationID,
		"resource_id":      resourceID,
	})
}

// EmitSessionCompleted emits agent_session/completed notification
func (nb *NotificationBroadcaster) EmitSessionCompleted(sessionID string, findingCount, queryCount int) {
	nb.Emit("agent_session/completed", map[string]interface{}{
		"session_id":    sessionID,
		"finding_count": findingCount,
		"query_count":   queryCount,
	})
}

// ClientCount returns the number of registered clients
func (nb *NotificationBroadcaster) ClientCount() int {
	return nb.sseBroadcaster.ConnectionCount()
}

// GetSSEHandler returns the HTTP handler for SSE connections
func (nb *NotificationBroadcaster) GetSSEHandler() http.HandlerFunc {
	return nb.sseBroadcaster.HandleSSE()
}
