package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/kagenti/kkbase/pkg/agent/mcp"
	"github.com/kagenti/kkbase/pkg/agenttypes"
	"go.uber.org/zap"
	"google.golang.org/genai"
)

// GeminiClient implements the Client interface using Google Gemini
type GeminiClient struct {
	client    *genai.Client
	model     string
	config    Config
	mcpClient *mcp.Client
	logger    *zap.Logger
}

// NewGeminiClient creates a new Gemini LLM client
func NewGeminiClient(config Config, mcpClient *mcp.Client, logger *zap.Logger) (*GeminiClient, error) {
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

	// Build system instruction
	systemContent := &genai.Content{
		Parts: []*genai.Part{
			{Text: SystemPrompt},
		},
		Role: "system",
	}

	// Build MCP tools as Gemini function declarations
	tools := c.buildMCPTools()

	// Run agentic loop
	result, err := c.runAgenticLoop(ctx, systemContent, tools, prompt, event)
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

// buildMCPTools creates Gemini function declarations for MCP tools
func (c *GeminiClient) buildMCPTools() []*genai.Tool {
	return []*genai.Tool{{
		FunctionDeclarations: []*genai.FunctionDeclaration{
			{
				Name: "query_knowledge_graph",
				Description: "Execute a read-only Cypher query against the Kubernetes knowledge graph stored in Neo4j. " +
					"Use this to understand resource topology, relationships, and current state. " +
					"Returns JSON results from the query.",
				Parameters: &genai.Schema{
					Type: "object",
					Properties: map[string]*genai.Schema{
						"query": {
							Type:        "string",
							Description: "Cypher query to execute (read-only MATCH queries only)",
						},
						"params": {
							Type:        "object",
							Description: "Query parameters as key-value pairs (optional)",
						},
					},
					Required: []string{"query"},
				},
			},
			{
				Name: "get_structure",
				Description: "Get the complete graph database schema including all node types (labels), " +
					"relationship types, and their properties. Use this first to understand what data " +
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
func (c *GeminiClient) runAgenticLoop(ctx context.Context, systemContent *genai.Content, tools []*genai.Tool, prompt string, event agenttypes.Event) (*agenttypes.InvestigationResult, error) {
	maxIterations := 10

	// Build conversation history
	messages := []*genai.Content{
		systemContent,
		{
			Parts: []*genai.Part{{Text: prompt}},
			Role:  "user",
		},
	}

	for i := 0; i < maxIterations; i++ {
		c.logger.Debug("agent iteration",
			zap.Int("iteration", i+1),
			zap.Int("max", maxIterations))

		// Call Gemini with tools
		generateConfig := &genai.GenerateContentConfig{
			Temperature:     &c.config.Temperature,
			MaxOutputTokens: int32(c.config.MaxTokens),
			Tools:           tools,
		}

		resp, err := c.client.Models.GenerateContent(ctx, c.model, messages, generateConfig)
		if err != nil {
			return nil, fmt.Errorf("Gemini API error: %w", err)
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
			// No function calls - extract final analysis
			return c.extractAnalysis(candidate.Content, event)
		}

		// Execute function calls via MCP
		c.logger.Info("executing function calls",
			zap.Int("count", len(functionCalls)))

		// Build function response parts
		responseParts := make([]*genai.Part, 0, len(functionCalls))
		for _, fc := range functionCalls {
			result := c.executeMCPTool(ctx, fc.Name, fc.Args)
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
func (c *GeminiClient) executeMCPTool(ctx context.Context, name string, args map[string]interface{}) map[string]interface{} {
	c.logger.Debug("executing MCP tool",
		zap.String("tool", name),
		zap.Any("args", args))

	var result map[string]interface{}
	var err error

	switch name {
	case "query_knowledge_graph":
		query, _ := args["query"].(string)
		params, _ := args["params"].(map[string]interface{})
		results, err := c.mcpClient.Query(ctx, query, params)
		if err != nil {
			result = map[string]interface{}{
				"error": err.Error(),
			}
		} else {
			result = map[string]interface{}{
				"results": results,
				"count":   len(results),
			}
		}

	case "get_structure":
		result, err = c.mcpClient.GetStructure(ctx)
		if err != nil {
			result = map[string]interface{}{
				"error": err.Error(),
			}
		}

	case "start_investigation":
		result, err = c.mcpClient.StartInvestigation(ctx, args)
		if err != nil {
			result = map[string]interface{}{
				"error": err.Error(),
			}
		}

	default:
		result = map[string]interface{}{
			"error": fmt.Sprintf("unknown tool: %s", name),
		}
	}

	c.logger.Debug("MCP tool result",
		zap.String("tool", name),
		zap.Bool("success", err == nil))

	return result
}

// extractAnalysis extracts the final analysis from Gemini's response
func (c *GeminiClient) extractAnalysis(content *genai.Content, event agenttypes.Event) (*agenttypes.InvestigationResult, error) {
	// Extract text from content
	var text string
	for _, part := range content.Parts {
		if part.Text != "" {
			text += part.Text
		}
	}

	if text == "" {
		return nil, fmt.Errorf("no text content in final response")
	}

	// Try to extract JSON from the text
	jsonStr := extractJSON(text)
	if jsonStr == "" {
		// If no JSON found, create a basic analysis from the text
		return &agenttypes.InvestigationResult{
			Event: event,
			Analysis: &agenttypes.Analysis{
				RootCause:        text,
				ImpactAssessment: "Unable to assess",
				Confidence:       0.5,
				RelatedResources: []string{},
			},
			Recommendations: []agenttypes.Recommendation{},
		}, nil
	}

	// Parse JSON
	var analysisData struct {
		RootCause        string   `json:"root_cause"`
		ImpactAssessment string   `json:"impact_assessment"`
		Confidence       float32  `json:"confidence"`
		RelatedResources []string `json:"related_resources"`
		Recommendations  []struct {
			Action       string `json:"action"`
			Description  string `json:"description"`
			RiskLevel    string `json:"risk_level"`
			AutoApproved bool   `json:"auto_approved"`
		} `json:"recommendations"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &analysisData); err != nil {
		c.logger.Warn("failed to parse analysis JSON, using text as-is", zap.Error(err))
		return &agenttypes.InvestigationResult{
			Event: event,
			Analysis: &agenttypes.Analysis{
				RootCause:        text,
				ImpactAssessment: "Unable to assess",
				Confidence:       0.5,
				RelatedResources: []string{},
			},
			Recommendations: []agenttypes.Recommendation{},
		}, nil
	}

	// Convert to result
	recommendations := make([]agenttypes.Recommendation, len(analysisData.Recommendations))
	for i, rec := range analysisData.Recommendations {
		recommendations[i] = agenttypes.Recommendation{
			Action:       rec.Action,
			Description:  rec.Description,
			RiskLevel:    rec.RiskLevel,
			AutoApproved: rec.AutoApproved,
		}
	}

	return &agenttypes.InvestigationResult{
		Event: event,
		Analysis: &agenttypes.Analysis{
			RootCause:        analysisData.RootCause,
			ImpactAssessment: analysisData.ImpactAssessment,
			Confidence:       analysisData.Confidence,
			RelatedResources: analysisData.RelatedResources,
		},
		Recommendations: recommendations,
	}, nil
}

// extractJSON attempts to extract JSON from text (handling markdown code blocks)
func extractJSON(text string) string {
	// Try to find JSON in markdown code blocks
	if strings.Contains(text, "```json") {
		start := strings.Index(text, "```json")
		if start != -1 {
			start += 7 // len("```json")
			end := strings.Index(text[start:], "```")
			if end != -1 {
				return strings.TrimSpace(text[start : start+end])
			}
		}
	}

	// Try to find JSON object directly
	start := strings.Index(text, "{")
	if start != -1 {
		end := strings.LastIndex(text, "}")
		if end != -1 && end > start {
			return text[start : end+1]
		}
	}

	return ""
}

// Close closes the Gemini client (no-op for new SDK)
func (c *GeminiClient) Close() error {
	// New SDK doesn't require explicit close
	return nil
}
