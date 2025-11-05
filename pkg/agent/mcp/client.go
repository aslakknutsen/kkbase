package mcp

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"
)

// Client wraps the MCP SDK client for agent use
type Client struct {
	mcpClient *mcp.Client
	session   *mcp.ClientSession
	logger    *zap.Logger
}

// NewClient creates a new MCP client
func NewClient(baseURL string, logger *zap.Logger) (*Client, error) {
	// Create MCP client using SDK
	mcpClient := mcp.NewClient(
		&mcp.Implementation{
			Name:    "kkbase-agent",
			Version: "1.0.0",
		},
		nil, // No client options needed
	)

	// Create streamable HTTP transport
	transport := &mcp.StreamableClientTransport{
		Endpoint: baseURL,
	}

	// Connect to server
	ctx := context.Background()
	session, err := mcpClient.Connect(ctx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MCP server: %w", err)
	}

	logger.Info("connected to MCP server", zap.String("url", baseURL))

	return &Client{
		mcpClient: mcpClient,
		session:   session,
		logger:    logger,
	}, nil
}

// CallTool calls an MCP tool with the given arguments
func (c *Client) CallTool(ctx context.Context, toolName string, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	result, err := c.session.CallTool(ctx, &mcp.CallToolParams{
		Name:      toolName,
		Arguments: arguments,
	})
	if err != nil {
		c.logger.Error("tool call failed",
			zap.String("tool", toolName),
			zap.Error(err))
		return nil, fmt.Errorf("failed to call tool %s: %w", toolName, err)
	}

	c.logger.Debug("tool call succeeded",
		zap.String("tool", toolName))

	return result, nil
}

// Query executes a Cypher query via MCP
func (c *Client) Query(ctx context.Context, cypherQuery string, params map[string]interface{}) ([]map[string]interface{}, error) {
	result, err := c.CallTool(ctx, "query", map[string]interface{}{
		"query":  cypherQuery,
		"params": params,
	})
	if err != nil {
		return nil, err
	}

	// Extract results from structured content
	if result.StructuredContent != nil {
		if data, ok := result.StructuredContent.(map[string]interface{}); ok {
			if results, ok := data["results"].([]interface{}); ok {
				converted := make([]map[string]interface{}, 0, len(results))
				for _, r := range results {
					if m, ok := r.(map[string]interface{}); ok {
						converted = append(converted, m)
					}
				}
				return converted, nil
			}
		}
	}

	return nil, fmt.Errorf("unexpected response format from query tool")
}

// GetStructure retrieves the graph schema via MCP
func (c *Client) GetStructure(ctx context.Context) (map[string]interface{}, error) {
	result, err := c.CallTool(ctx, "structure", nil)
	if err != nil {
		return nil, err
	}

	if result.StructuredContent != nil {
		if data, ok := result.StructuredContent.(map[string]interface{}); ok {
			return data, nil
		}
	}

	return nil, fmt.Errorf("unexpected response format from structure tool")
}

// StartInvestigation starts a metrics investigation via MCP
func (c *Client) StartInvestigation(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	result, err := c.CallTool(ctx, "start_investigation", args)
	if err != nil {
		return nil, err
	}

	if result.StructuredContent != nil {
		if data, ok := result.StructuredContent.(map[string]interface{}); ok {
			return data, nil
		}
	}

	return nil, fmt.Errorf("unexpected response format from start_investigation tool")
}

// CompleteInvestigation completes a metrics investigation via MCP
func (c *Client) CompleteInvestigation(ctx context.Context, investigationID string) error {
	_, err := c.CallTool(ctx, "complete_investigation", map[string]interface{}{
		"investigation_id": investigationID,
	})
	return err
}

// ListTools retrieves the list of available tools from the MCP server
func (c *Client) ListTools(ctx context.Context) ([]*mcp.Tool, error) {
	result, err := c.session.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		c.logger.Error("failed to list tools", zap.Error(err))
		return nil, fmt.Errorf("failed to list tools: %w", err)
	}

	c.logger.Debug("tools listed successfully", zap.Int("count", len(result.Tools)))
	return result.Tools, nil
}

// Close closes the MCP client connection
func (c *Client) Close() error {
	if c.session != nil {
		return c.session.Close()
	}
	return nil
}
