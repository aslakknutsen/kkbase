package llm

import (
	"context"
	"fmt"
	"time"

	agentmcp "github.com/kagenti/kkbase/pkg/agent/mcp"
	"github.com/kagenti/kkbase/pkg/agenttypes"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"
	"google.golang.org/genai"
)

// GeminiClient implements the Client interface using Google Gemini
type GeminiClient struct {
	client    *genai.Client
	model     string
	config    Config
	mcpClient *agentmcp.Client
	logger    *zap.Logger
}

// NewGeminiClient creates a new Gemini LLM client
func NewGeminiClient(config Config, mcpClient *agentmcp.Client, logger *zap.Logger) (*GeminiClient, error) {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  config.APIKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	if config.Model == "" {
		config.Model = "gemini-2.0-flash-exp"
	}

	logger.Info("created Gemini client",
		zap.String("model", config.Model),
		zap.Float32("temperature", config.Temperature))

	return &GeminiClient{
		client:    client,
		model:     config.Model,
		config:    config,
		mcpClient: mcpClient,
		logger:    logger,
	}, nil
}

// InvestigateWithTools performs investigation using Gemini with MCP tool calling
func (c *GeminiClient) InvestigateWithTools(ctx context.Context, event agenttypes.Event) (*agenttypes.InvestigationResult, error) {
	startTime := time.Now()

	// Build initial prompt
	prompt := c.buildEventPrompt(event)

	// Build MCP tools as Gemini function declarations
	tools := c.buildMCPTools()

	// Run agentic loop
	result, err := c.runAgenticLoop(ctx, tools, prompt, event)
	if err != nil {
		return &agenttypes.InvestigationResult{
			Event:    event,
			Status:   "failed",
			Error:    err,
			Duration: time.Since(startTime),
		}, err
	}

	result.Event = event
	result.Status = "completed"
	result.Duration = time.Since(startTime)

	c.logger.Info("investigation completed",
		zap.String("event_id", event.ID),
		zap.Duration("duration", result.Duration),
		zap.Int("recommendations", len(result.Recommendations)))

	return result, nil
}

// buildEventPrompt formats the event for analysis
func (c *GeminiClient) buildEventPrompt(event agenttypes.Event) string {
	// Format additional data
	dataStr := ""
	for k, v := range event.Data {
		dataStr += fmt.Sprintf("  %s: %v\n", k, v)
	}
	if dataStr == "" {
		dataStr = "  (none)"
	}

	return fmt.Sprintf(EventAnalysisPromptTemplate,
		event.Type,
		event.Severity,
		event.Source,
		event.Reason,
		event.Message,
		event.Resource.Name,
		event.Resource.Type,
		event.Resource.Namespace,
		event.Timestamp.Format(time.RFC3339),
		dataStr,
	)
}

// buildMCPTools discovers available tools from MCP server and converts them to Gemini function declarations
func (c *GeminiClient) buildMCPTools() []*genai.Tool {
	ctx := context.Background()

	// Discover tools from MCP server
	mcpTools, err := c.mcpClient.ListTools(ctx)
	if err != nil {
		c.logger.Warn("failed to discover MCP tools, using empty list", zap.Error(err))
		return []*genai.Tool{}
	}

	c.logger.Info("discovered tools from MCP server", zap.Int("count", len(mcpTools)))

	// Convert MCP tools to Gemini function declarations
	functionDeclarations := make([]*genai.FunctionDeclaration, 0, len(mcpTools))
	for _, tool := range mcpTools {
		funcDecl := c.convertMCPToolToGemini(tool)
		if funcDecl != nil {
			functionDeclarations = append(functionDeclarations, funcDecl)
		}
	}

	return []*genai.Tool{{
		FunctionDeclarations: functionDeclarations,
	}}
}

// convertMCPToolToGemini converts an MCP tool definition to a Gemini function declaration
func (c *GeminiClient) convertMCPToolToGemini(tool *mcp.Tool) *genai.FunctionDeclaration {
	if tool == nil {
		return nil
	}

	funcDecl := &genai.FunctionDeclaration{
		Name:        tool.Name,
		Description: tool.Description,
	}

	// Convert InputSchema to Gemini Parameters schema
	if tool.InputSchema != nil {
		if schemaMap, ok := tool.InputSchema.(map[string]interface{}); ok {
			funcDecl.Parameters = c.convertJSONSchemaToGemini(schemaMap)
		} else {
			c.logger.Warn("tool InputSchema is not a map",
				zap.String("tool", tool.Name),
				zap.Any("schema_type", fmt.Sprintf("%T", tool.InputSchema)))
		}
	}

	return funcDecl
}

// convertJSONSchemaToGemini converts a JSON Schema (map) to Gemini Schema
func (c *GeminiClient) convertJSONSchemaToGemini(jsonSchema map[string]interface{}) *genai.Schema {
	schema := &genai.Schema{}

	// Extract type - Gemini SDK expects the type as a string field
	if t, ok := jsonSchema["type"].(string); ok {
		// Store as string in the Type field (SDK handles this)
		schema.Type = genai.Type(t)
	}

	// Extract description
	if desc, ok := jsonSchema["description"].(string); ok {
		schema.Description = desc
	}

	// Extract properties
	if props, ok := jsonSchema["properties"].(map[string]interface{}); ok {
		schema.Properties = make(map[string]*genai.Schema)
		for propName, propDef := range props {
			if propDefMap, ok := propDef.(map[string]interface{}); ok {
				schema.Properties[propName] = c.convertJSONSchemaToGemini(propDefMap)
			}
		}
	}

	// Extract required fields
	if req, ok := jsonSchema["required"].([]interface{}); ok {
		required := make([]string, 0, len(req))
		for _, r := range req {
			if s, ok := r.(string); ok {
				required = append(required, s)
			}
		}
		schema.Required = required
	}

	// Extract items (for arrays)
	if items, ok := jsonSchema["items"].(map[string]interface{}); ok {
		schema.Items = c.convertJSONSchemaToGemini(items)
	}

	// Extract enum values - convert to []string
	if enumVals, ok := jsonSchema["enum"].([]interface{}); ok {
		enumStrings := make([]string, 0, len(enumVals))
		for _, v := range enumVals {
			if s, ok := v.(string); ok {
				enumStrings = append(enumStrings, s)
			}
		}
		schema.Enum = enumStrings
	}

	return schema
}

// buildMCPToolsStatic is the old static implementation kept as fallback
// This can be removed once dynamic discovery is proven stable
func (c *GeminiClient) buildMCPToolsStatic() []*genai.Tool {
	return []*genai.Tool{{
		FunctionDeclarations: []*genai.FunctionDeclaration{
			{
				Name: "start_agent_session",
				Description: "Start a new AI agent diagnostic session for tracking exploratory investigation. " +
					"IMPORTANT: Call this FIRST before any other investigation tools. " +
					"Returns a session ID that must be used in all subsequent query_with_session calls. " +
					"The session automatically tracks hypotheses, queries, findings, and dynamically calculates blast zone.",
				Parameters: &genai.Schema{
					Type: "object",
					Properties: map[string]*genai.Schema{
						"symptom": {
							Type:        "string",
							Description: "Initial symptom being investigated (e.g., 'Orders failing for last 1m', 'Pod CrashLoopBackOff')",
						},
						"initial_resource": {
							Type:        "string",
							Description: "Optional initial resource to investigate (e.g., 'Service/namespace/service-name' or 'Pod/namespace/pod-name')",
						},
					},
					Required: []string{"symptom"},
				},
			},
			{
				Name: "query_with_session",
				Description: "Execute a Cypher query within an active agent session. This automatically: " +
					"1. Records the query and your reasoning " +
					"2. Executes the query against the knowledge graph " +
					"3. Extracts findings from results (failed calls, unhealthy pods, errors) " +
					"4. Links findings to affected resources " +
					"5. Updates session state. " +
					"Use this instead of query_knowledge_graph when working within an investigation session.",
				Parameters: &genai.Schema{
					Type: "object",
					Properties: map[string]*genai.Schema{
						"session_id": {
							Type:        "string",
							Description: "Session ID from start_agent_session",
						},
						"query": {
							Type:        "string",
							Description: "Cypher query to execute (read-only MATCH queries only)",
						},
						"reasoning": {
							Type:        "string",
							Description: "Explanation of why this query is being run and what it seeks to discover",
						},
						"params": {
							Type:        "object",
							Description: "Query parameters as key-value pairs (optional)",
						},
					},
					Required: []string{"session_id", "query", "reasoning"},
				},
			},
			{
				Name: "update_hypothesis",
				Description: "Update the current diagnostic hypothesis for an investigation session. " +
					"Call this at the end of each investigation round when you've refined your understanding of the problem. " +
					"This marks the current investigation stage and triggers blast zone recalculation.",
				Parameters: &genai.Schema{
					Type: "object",
					Properties: map[string]*genai.Schema{
						"session_id": {
							Type:        "string",
							Description: "Session ID from start_agent_session",
						},
						"stage": {
							Type:        "integer",
							Description: "Investigation stage/round number (1, 2, 3, etc.)",
						},
						"text": {
							Type:        "string",
							Description: "Current hypothesis text explaining the suspected root cause",
						},
					},
					Required: []string{"session_id", "stage", "text"},
				},
			},
			{
				Name: "record_recommendation",
				Description: "Record an actionable recommendation based on investigation findings. " +
					"Use this to suggest concrete next steps for resolving the root cause or addressing other issues. " +
					"Recommendations should be specific, actionable, and prioritized.",
				Parameters: &genai.Schema{
					Type: "object",
					Properties: map[string]*genai.Schema{
						"session_id": {
							Type:        "string",
							Description: "Session ID from start_agent_session",
						},
						"type": {
							Type:        "string",
							Description: "Type of recommendation: root_cause_fix, preventive_action, optimization, monitoring_improvement, or cleanup",
						},
						"priority": {
							Type:        "string",
							Description: "Priority level: critical, high, medium, or low",
						},
						"title": {
							Type:        "string",
							Description: "Short title for the recommendation",
						},
						"description": {
							Type:        "string",
							Description: "Detailed description of what should be done",
						},
						"rationale": {
							Type:        "string",
							Description: "Why this recommendation is being made",
						},
						"related_findings": {
							Type:        "array",
							Description: "Finding IDs that support this recommendation",
						},
						"action_items": {
							Type:        "array",
							Description: "Step-by-step action items",
						},
						"estimated_effort": {
							Type:        "string",
							Description: "Estimated time to complete (e.g., '30 minutes', '2 hours')",
						},
						"automation_hint": {
							Type:        "string",
							Description: "Commands or automation suggestions",
						},
					},
					Required: []string{"session_id", "type", "priority", "title", "description", "rationale", "action_items"},
				},
			},
			{
				Name: "complete_agent_session",
				Description: "Mark an agent session as completed and generate final summary. " +
					"IMPORTANT: Call this LAST when the investigation is finished and you've identified the root cause. " +
					"This finalizes the blast zone snapshot and completes any linked investigations.",
				Parameters: &genai.Schema{
					Type: "object",
					Properties: map[string]*genai.Schema{
						"session_id": {
							Type:        "string",
							Description: "Session ID from start_agent_session",
						},
						"summary": {
							Type:        "string",
							Description: "Optional summary of findings and root cause",
						},
					},
					Required: []string{"session_id"},
				},
			},
			{
				Name: "get_structure",
				Description: "Get the complete graph database schema including all node types (labels), " +
					"relationship types, and their properties. Use this to understand what data " +
					"is available before querying.",
				Parameters: &genai.Schema{
					Type: "object",
				},
			},
			{
				Name: "start_investigation",
				Description: "Start a metrics investigation by pulling data from Prometheus for a specific " +
					"Kubernetes resource. Use this when you need to analyze metrics like memory usage, CPU, " +
					"network traffic, etc. The metrics will be stored temporarily for analysis.",
				Parameters: &genai.Schema{
					Type: "object",
					Properties: map[string]*genai.Schema{
						"resource_type": {
							Type:        "string",
							Description: "Type of resource (Pod, Service, Node, etc.)",
						},
						"resource_id": {
							Type:        "string",
							Description: "Full resource ID in format: namespace/name or just name for cluster-scoped resources",
						},
						"symptom": {
							Type:        "string",
							Description: "Symptom being investigated (e.g., 'OOMKilled', 'HighLatency', 'CrashLoopBackOff')",
						},
						"lookback_minutes": {
							Type:        "integer",
							Description: "How many minutes of metrics to pull (5-120, default 15)",
						},
					},
					Required: []string{"resource_type", "resource_id", "symptom"},
				},
			},
		},
	}}
}

// runAgenticLoop runs the agent loop allowing Gemini to call functions iteratively
func (c *GeminiClient) runAgenticLoop(ctx context.Context, tools []*genai.Tool, prompt string, event agenttypes.Event) (*agenttypes.InvestigationResult, error) {
	maxIterations := c.config.MaxIterations
	if maxIterations <= 0 {
		maxIterations = 30 // Fallback default
	}
	var sessionID string // Track session ID throughout investigation

	// Build conversation history (without system message - it goes in the config)
	messages := []*genai.Content{
		{
			Parts: []*genai.Part{{Text: prompt}},
			Role:  "user",
		},
	}

	for i := 0; i < maxIterations; i++ {
		c.logger.Debug("agent iteration",
			zap.Int("iteration", i+1),
			zap.Int("max", maxIterations))

		// Build system instruction (Gemini uses this instead of system role in messages)
		systemInstruction := &genai.Content{
			Parts: []*genai.Part{
				{Text: SystemPrompt},
			},
		}

		// Call Gemini with tools and system instruction
		generateConfig := &genai.GenerateContentConfig{
			Temperature:       &c.config.Temperature,
			MaxOutputTokens:   int32(c.config.MaxTokens),
			Tools:             tools,
			SystemInstruction: systemInstruction,
		}

		resp, err := c.client.Models.GenerateContent(ctx, c.model, messages, generateConfig)
		if err != nil {
			return nil, fmt.Errorf("gemini API error: %w", err)
		}

		// Check response
		if len(resp.Candidates) == 0 {
			return nil, fmt.Errorf("no response candidates from Gemini")
		}

		candidate := resp.Candidates[0]
		if candidate.Content == nil {
			return nil, fmt.Errorf("no content in Gemini response")
		}

		// Add assistant response to conversation
		messages = append(messages, candidate.Content)

		// Check for function calls
		functionCalls := c.extractFunctionCalls(candidate.Content)
		if len(functionCalls) == 0 {
			// No function calls - investigation complete
			return c.extractAnalysis(candidate.Content, event, sessionID)
		}

		// Build function response parts
		responseParts := make([]*genai.Part, 0, len(functionCalls))
		for _, fc := range functionCalls {
			// Execute function calls via MCP
			c.logger.Info("executing function call",
				zap.String("name", fc.Name),
			)

			result := c.executeMCPTool(ctx, fc.Name, fc.Args)

			// Capture session ID from start_agent_session
			if fc.Name == "start_agent_session" {
				if sid, ok := result["session_id"].(string); ok {
					sessionID = sid
					c.logger.Info("captured session ID", zap.String("session_id", sessionID))
				}
			}

			responseParts = append(responseParts, &genai.Part{
				FunctionResponse: &genai.FunctionResponse{
					Name:     fc.Name,
					Response: result,
				},
			})
		}

		// Add function responses to conversation
		messages = append(messages, &genai.Content{
			Parts: responseParts,
			Role:  "function",
		})
	}

	// Max iterations reached - try to force a conclusion
	c.logger.Warn("max iterations reached, forcing conclusion",
		zap.Int("iterations", maxIterations),
		zap.String("session_id", sessionID))

	// If we have a session ID, try to complete it with timeout status
	if sessionID != "" {
		timeoutSummary := fmt.Sprintf(
			"Investigation reached maximum iteration limit (%d) before full completion. "+
				"Please review the findings and hypotheses recorded during the investigation. "+
				"Additional manual investigation may be required.", maxIterations)

		// Complete the session with timeout status using the existing tool
		completeArgs := map[string]interface{}{
			"session_id": sessionID,
			"summary":    timeoutSummary,
			"status":     "timeout",
		}

		c.logger.Info("attempting to complete session with timeout status",
			zap.String("session_id", sessionID))

		c.executeMCPTool(ctx, "complete_agent_session", completeArgs)

		// Return a result indicating timeout
		return &agenttypes.InvestigationResult{
			Event: event,
			Analysis: &agenttypes.Analysis{
				RootCause: "Investigation timeout - max iterations reached",
				ImpactAssessment: fmt.Sprintf(
					"Investigation reached iteration limit after %d steps. "+
						"Partial findings available in session %s.", maxIterations, sessionID),
				Confidence: 0.5,
			},
			Recommendations: []agenttypes.Recommendation{
				{
					Priority:    "high",
					Type:        "manual_investigation",
					Title:       "Complete investigation manually",
					Description: "The automated investigation reached its iteration limit. Review the session findings and complete the investigation manually.",
					Rationale:   "Automated agent was unable to reach a definitive conclusion within the iteration limit.",
				},
			},
			SessionID: sessionID,
		}, nil
	}

	// No session ID - just return error as before
	return nil, fmt.Errorf("exceeded maximum iterations (%d) without reaching conclusion", maxIterations)
}

// functionCall represents a function call from Gemini
type functionCall struct {
	Name string
	Args map[string]interface{}
}

// extractFunctionCalls extracts function calls from Gemini content
func (c *GeminiClient) extractFunctionCalls(content *genai.Content) []functionCall {
	var calls []functionCall

	for _, part := range content.Parts {
		if part.FunctionCall != nil {
			calls = append(calls, functionCall{
				Name: part.FunctionCall.Name,
				Args: part.FunctionCall.Args,
			})
		}
	}

	return calls
}

// executeMCPTool executes an MCP tool and returns the result
// Now handles all tools generically via dynamic discovery
func (c *GeminiClient) executeMCPTool(ctx context.Context, name string, args map[string]interface{}) map[string]interface{} {
	c.logger.Debug("executing MCP tool",
		zap.String("tool", name),
		zap.Any("args", args))

	// Call the MCP server generically for any tool
	mcpResult, err := c.mcpClient.CallTool(ctx, name, args)
	if err != nil {
		c.logger.Warn("MCP tool execution failed",
			zap.String("tool", name),
			zap.Error(err))
		return map[string]interface{}{
			"error": err.Error(),
		}
	}

	// Try to extract structured content
	var result map[string]interface{}
	if mcpResult.StructuredContent != nil {
		if data, ok := mcpResult.StructuredContent.(map[string]interface{}); ok {
			result = data
		} else {
			// Structured content exists but not in expected format
			result = map[string]interface{}{
				"raw_content": mcpResult.StructuredContent,
			}
		}
	} else {
		// No structured content, try to extract from text content
		result = map[string]interface{}{
			"success": true,
		}

		// Include text content if available
		if len(mcpResult.Content) > 0 {
			textParts := []string{}
			for _, content := range mcpResult.Content {
				if textContent, ok := content.(*mcp.TextContent); ok {
					textParts = append(textParts, textContent.Text)
				}
			}
			if len(textParts) > 0 {
				result["message"] = textParts[0] // Use first text part as message
				if len(textParts) > 1 {
					result["details"] = textParts
				}
			}
		}
	}

	c.logger.Debug("MCP tool executed successfully",
		zap.String("tool", name),
		zap.Any("result_keys", getKeys(result)))

	return result
}

// getKeys returns the keys from a map for logging purposes
func getKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// extractAnalysis extracts the final analysis from Gemini's response
// In the new workflow, recommendations are already stored via MCP tools during investigation
func (c *GeminiClient) extractAnalysis(content *genai.Content, event agenttypes.Event, sessionID string) (*agenttypes.InvestigationResult, error) {
	// Extract text from content
	var text string
	for _, part := range content.Parts {
		if part.Text != "" {
			text += part.Text
		}
	}

	if text == "" {
		text = "Investigation completed. See session details for findings and recommendations."
	}

	c.logger.Info("investigation completed",
		zap.String("session_id", sessionID),
		zap.String("summary", text))

	// In the new workflow, all analysis data and recommendations are stored in the session
	// Return a simplified result with the session ID
	return &agenttypes.InvestigationResult{
		Event: event,
		Analysis: &agenttypes.Analysis{
			RootCause:        text,
			ImpactAssessment: "See session details",
			Confidence:       1.0, // Session tracks actual confidence
			RelatedResources: []string{},
		},
		Recommendations: []agenttypes.Recommendation{}, // Stored in session via record_recommendation
		SessionID:       sessionID,
	}, nil
}

// Close closes the Gemini client (no-op for new SDK)
func (c *GeminiClient) Close() error {
	// New SDK doesn't require explicit close
	return nil
}
