package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/kagenti/kkbase/pkg/agent/mcp"
	"github.com/kagenti/kkbase/pkg/agenttypes"
	"github.com/kagenti/kkbase/pkg/graph"
	"github.com/kagenti/kkbase/pkg/llm"
	"github.com/kagenti/kkbase/pkg/observability"
	"go.uber.org/zap"
)

// Agent is the core investigation agent
type Agent struct {
	id             string
	geminiClient   *llm.GeminiClient
	mcpClient      *mcp.Client
	graphStore     graph.GraphStore
	sessionManager *observability.AgentSessionManager
	logger         *zap.Logger
}

// NewAgent creates a new investigation agent
func NewAgent(
	id string,
	geminiClient *llm.GeminiClient,
	mcpClient *mcp.Client,
	graphStore graph.GraphStore,
	sessionManager *observability.AgentSessionManager,
	logger *zap.Logger,
) *Agent {
	return &Agent{
		id:             id,
		geminiClient:   geminiClient,
		mcpClient:      mcpClient,
		graphStore:     graphStore,
		sessionManager: sessionManager,
		logger:         logger.With(zap.String("agent_id", id)),
	}
}

// Investigate investigates an event using LLM with MCP tools
func (a *Agent) Investigate(ctx context.Context, event agenttypes.Event) agenttypes.InvestigationResult {
	start := time.Now()

	a.logger.Info("starting investigation",
		zap.String("event_id", event.ID),
		zap.String("reason", event.Reason),
		zap.String("resource", event.Resource.ID))

	// Step 1: Start agent session
	session, err := a.sessionManager.CreateSession(ctx, event.Reason, event.Resource.ID)
	if err != nil {
		a.logger.Error("failed to create session", zap.Error(err))
		return agenttypes.InvestigationResult{
			Event:    event,
			Status:   "failed",
			Error:    fmt.Errorf("failed to create session: %w", err),
			Duration: time.Since(start),
		}
	}

	a.logger.Info("agent session started", zap.String("session_id", session.ID))

	// Step 2: Use LLM with MCP function calling for investigation
	result, err := a.geminiClient.InvestigateWithTools(ctx, event)
	if err != nil {
		a.logger.Error("investigation failed", zap.Error(err))
		// Complete session with error
		a.sessionManager.CompleteSession(ctx, session.ID, fmt.Sprintf("Investigation failed: %v", err))
		return agenttypes.InvestigationResult{
			SessionID: session.ID,
			Event:     event,
			Status:    "failed",
			Error:     err,
			Duration:  time.Since(start),
		}
	}

	// Step 3: Store recommendations in Neo4j
	if len(result.Recommendations) > 0 {
		if err := a.storeRecommendations(ctx, session.ID, result.Recommendations); err != nil {
			a.logger.Warn("failed to store recommendations", zap.Error(err))
			// Non-fatal, continue
		}
	}

	// Step 4: Complete session with summary
	summary := fmt.Sprintf("Root Cause: %s | Confidence: %.2f | Recommendations: %d",
		result.Analysis.RootCause,
		result.Analysis.Confidence,
		len(result.Recommendations))

	_, err = a.sessionManager.CompleteSession(ctx, session.ID, summary)
	if err != nil {
		a.logger.Warn("failed to complete session", zap.Error(err))
		// Non-fatal
	}

	a.logger.Info("investigation completed successfully",
		zap.String("session_id", session.ID),
		zap.Float32("confidence", result.Analysis.Confidence),
		zap.Int("recommendations", len(result.Recommendations)),
		zap.Duration("duration", time.Since(start)))

	result.SessionID = session.ID
	result.Status = "completed"
	result.Duration = time.Since(start)

	return *result
}

// storeRecommendations stores recommendations in the knowledge graph
func (a *Agent) storeRecommendations(ctx context.Context, sessionID string, recommendations []agenttypes.Recommendation) error {
	// Create recommendation nodes and link them to the agent session
	for i, rec := range recommendations {
		query := `
			MATCH (s:AgentSession {id: $session_id})
			CREATE (r:Recommendation {
				id: $rec_id,
				action: $action,
				description: $description,
				risk_level: $risk_level,
				auto_approved: $auto_approved,
				created_at: datetime()
			})
			CREATE (s)-[:HAS_RECOMMENDATION]->(r)
			RETURN r.id as id
		`

		params := map[string]interface{}{
			"session_id":    sessionID,
			"rec_id":        fmt.Sprintf("%s-rec-%d", sessionID, i),
			"action":        rec.Action,
			"description":   rec.Description,
			"risk_level":    rec.RiskLevel,
			"auto_approved": rec.AutoApproved,
		}

		_, err := a.graphStore.Query(ctx, query, params)
		if err != nil {
			a.logger.Error("failed to store recommendation",
				zap.Error(err),
				zap.String("session_id", sessionID),
				zap.Int("recommendation_index", i))
			return fmt.Errorf("failed to store recommendation %d: %w", i, err)
		}
	}

	a.logger.Debug("stored recommendations",
		zap.String("session_id", sessionID),
		zap.Int("count", len(recommendations)))

	return nil
}
