package agent

import (
	"context"
	"time"

	"github.com/kagenti/kkbase/pkg/agent/mcp"
	"github.com/kagenti/kkbase/pkg/agenttypes"
	"github.com/kagenti/kkbase/pkg/llm"
	"go.uber.org/zap"
)

// Agent is the core investigation agent
type Agent struct {
	id           string
	geminiClient *llm.GeminiClient
	mcpClient    *mcp.Client
	logger       *zap.Logger
}

// NewAgent creates a new investigation agent
func NewAgent(
	id string,
	geminiClient *llm.GeminiClient,
	mcpClient *mcp.Client,
	logger *zap.Logger,
) *Agent {
	return &Agent{
		id:           id,
		geminiClient: geminiClient,
		mcpClient:    mcpClient,
		logger:       logger.With(zap.String("agent_id", id)),
	}
}

// Investigate investigates an event using LLM with MCP tools
// The LLM now manages the entire session lifecycle via MCP tools:
// - start_agent_session to begin
// - query_with_session for investigation
// - record_recommendation for each recommendation
// - complete_agent_session to finish
func (a *Agent) Investigate(ctx context.Context, event agenttypes.Event) agenttypes.InvestigationResult {
	start := time.Now()

	a.logger.Info("starting investigation",
		zap.String("event_id", event.ID),
		zap.String("reason", event.Reason),
		zap.String("resource", event.Resource.ID))

	// Use LLM with MCP function calling for investigation
	// The LLM will manage the session lifecycle through MCP tools
	result, err := a.geminiClient.InvestigateWithTools(ctx, event)
	if err != nil {
		a.logger.Error("investigation failed", zap.Error(err))
		return agenttypes.InvestigationResult{
			Event:    event,
			Status:   "failed",
			Error:    err,
			Duration: time.Since(start),
		}
	}

	a.logger.Info("investigation completed successfully",
		zap.String("session_id", result.SessionID),
		zap.Duration("duration", time.Since(start)))

	result.Status = "completed"
	result.Duration = time.Since(start)

	return *result
}
