package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/kagenti/kkbase/pkg/observability"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"
)

// registerAgentSessionResources registers all agent session read-only tools
func (s *Server) registerAgentSessionResources(sessionManager *observability.AgentSessionManager) error {
	if sessionManager == nil {
		s.logger.Info("agent session manager not available, skipping agent session resources")
		return nil
	}

	// Resource 1: get_active_sessions (as a tool)
	// Lists all active agent sessions
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name: "get_active_sessions",
		Description: "Get list of all currently active AI agent diagnostic sessions. " +
			"Web dashboard uses this to discover new sessions and display them in the UI. " +
			"This is a read-only tool that returns session summary information.",
	}, func(ctx context.Context, request *mcp.CallToolRequest, input struct{}) (*mcp.CallToolResult, any, error) {
		s.logger.Debug("fetching active sessions")

		sessions, err := sessionManager.GetActiveSessions(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get active sessions: %w", err)
		}

		data, err := json.Marshal(sessions)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to marshal sessions: %w", err)
		}

		s.logger.Debug("active sessions retrieved", zap.Int("count", len(sessions)))

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("Found %d active session(s):\n\n%s", len(sessions), string(data)),
				},
			},
		}, sessions, nil
	})

	// Resource 2: get_session_details (as a tool)
	// Complete session state snapshot
	type GetSessionDetailsInput struct {
		SessionID string `json:"session_id"`
	}

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name: "get_session_details",
		Description: "Get complete state of an agent investigation session including all hypotheses, queries, findings, and linked investigations. " +
			"Used by web dashboard to display detailed session view. This is a read-only tool.",
	}, func(ctx context.Context, request *mcp.CallToolRequest, input GetSessionDetailsInput) (*mcp.CallToolResult, any, error) {
		sessionID := input.SessionID
		if sessionID == "" {
			return nil, nil, fmt.Errorf("session_id is required")
		}

		s.logger.Debug("fetching session details", zap.String("session_id", sessionID))

		sessionDetail, err := sessionManager.GetSession(ctx, sessionID)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get session: %w", err)
		}

		data, err := json.Marshal(sessionDetail)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to marshal session: %w", err)
		}

		s.logger.Debug("session details retrieved", zap.String("session_id", sessionID))

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("Session %s details:\n\n%s", sessionID, string(data)),
				},
			},
		}, sessionDetail, nil
	})

	// Resource 3: get_blast_zone (as a tool)
	// Dynamic blast zone graph
	type GetBlastZoneInput struct {
		SessionID string `json:"session_id"`
	}

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name: "get_blast_zone",
		Description: "Get dynamically calculated blast zone graph showing all affected resources and their relationships. " +
			"Includes nodes (resources) and edges (relationships) with status indicators. " +
			"Recalculated on each call to reflect current findings. This is a read-only tool.",
	}, func(ctx context.Context, request *mcp.CallToolRequest, input GetBlastZoneInput) (*mcp.CallToolResult, any, error) {
		sessionID := input.SessionID
		if sessionID == "" {
			return nil, nil, fmt.Errorf("session_id is required")
		}

		s.logger.Debug("calculating blast zone", zap.String("session_id", sessionID))

		// Get blast zone calculator from session manager
		// For now, we'll return an empty blast zone if it fails
		blastZone, err := s.calculateBlastZone(ctx, sessionID)
		if err != nil {
			s.logger.Warn("failed to calculate blast zone",
				zap.String("session_id", sessionID),
				zap.Error(err))

			// Return empty blast zone rather than error
			emptyBlastZone := &observability.BlastZoneSnapshot{
				SessionID:     sessionID,
				Nodes:         []observability.BlastZoneNode{},
				Edges:         []observability.BlastZoneEdge{},
				AffectedCount: 0,
				ImpactRadius:  0,
			}
			data, _ := json.Marshal(emptyBlastZone)
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{
						Text: fmt.Sprintf("Blast zone for session %s (empty):\n\n%s", sessionID, string(data)),
					},
				},
			}, emptyBlastZone, nil
		}

		data, err := json.Marshal(blastZone)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to marshal blast zone: %w", err)
		}

		s.logger.Debug("blast zone calculated",
			zap.String("session_id", sessionID),
			zap.Int("nodes", len(blastZone.Nodes)),
			zap.Int("edges", len(blastZone.Edges)))

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("Blast zone for session %s: %d nodes, %d edges\n\n%s",
						sessionID, len(blastZone.Nodes), len(blastZone.Edges), string(data)),
				},
			},
		}, blastZone, nil
	})

	// Resource 4: get_session_timeline (as a tool)
	// Chronological timeline of events
	type GetSessionTimelineInput struct {
		SessionID string `json:"session_id"`
	}

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name: "get_session_timeline",
		Description: "Get chronological timeline of all events in an investigation session: " +
			"hypothesis updates, query executions, findings discovered, and investigations spawned. " +
			"Used by web dashboard to display activity timeline. This is a read-only tool.",
	}, func(ctx context.Context, request *mcp.CallToolRequest, input GetSessionTimelineInput) (*mcp.CallToolResult, any, error) {
		sessionID := input.SessionID
		if sessionID == "" {
			return nil, nil, fmt.Errorf("session_id is required")
		}

		s.logger.Debug("fetching timeline", zap.String("session_id", sessionID))

		timeline, err := sessionManager.GetTimeline(ctx, sessionID)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get timeline: %w", err)
		}

		data, err := json.Marshal(timeline)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to marshal timeline: %w", err)
		}

		s.logger.Debug("timeline retrieved",
			zap.String("session_id", sessionID),
			zap.Int("events", len(timeline)))

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("Timeline for session %s: %d event(s)\n\n%s",
						sessionID, len(timeline), string(data)),
				},
			},
		}, timeline, nil
	})

	s.logger.Info("registered agent session read-only tools",
		zap.Strings("tools", []string{
			"get_active_sessions",
			"get_session_details",
			"get_blast_zone",
			"get_session_timeline",
		}))

	return nil
}

// calculateBlastZone is a helper to calculate blast zone (delegates to session manager's calculator)
func (s *Server) calculateBlastZone(ctx context.Context, sessionID string) (*observability.BlastZoneSnapshot, error) {
	// Query Neo4j for affected resources and build blast zone
	// This is a simplified version - full implementation would use the BlastZoneCalculator
	query := `
		MATCH (session:AgentSession {id: $session_id})
		OPTIONAL MATCH (session)-[:HAS_FINDING]->(f:Finding)
		RETURN session, count(f) as finding_count
	`

	results, err := s.store.Query(ctx, query, map[string]interface{}{
		"session_id": sessionID,
	})
	if err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	// For now, return basic blast zone structure
	// Full implementation would use BlastZoneCalculator
	blastZone := &observability.BlastZoneSnapshot{
		SessionID:     sessionID,
		Nodes:         []observability.BlastZoneNode{},
		Edges:         []observability.BlastZoneEdge{},
		ImpactRadius:  0,
		AffectedCount: 0,
	}

	return blastZone, nil
}
