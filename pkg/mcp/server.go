package mcp

import (
	"context"
	"fmt"

	"github.com/kagenti/kkbase/pkg/graph"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"
)

// Server represents the MCP server instance
type Server struct {
	mcpServer *mcp.Server
	store     graph.GraphStore
	logger    *zap.Logger
}

// NewServer creates a new MCP server instance
func NewServer(store graph.GraphStore, logger *zap.Logger) (*Server, error) {
	if store == nil {
		return nil, fmt.Errorf("graph store cannot be nil")
	}
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}

	// Create MCP server
	mcpServer := mcp.NewServer(
		&mcp.Implementation{
			Name:    "kkbase-mcp",
			Version: "1.0.0",
		},
		&mcp.ServerOptions{},
	)

	s := &Server{
		mcpServer: mcpServer,
		store:     store,
		logger:    logger,
	}

	// Register tools
	if err := s.registerTools(); err != nil {
		return nil, fmt.Errorf("failed to register tools: %w", err)
	}

	logger.Info("MCP server initialized",
		zap.String("name", "kkbase-mcp"),
		zap.String("version", "1.0.0"))

	return s, nil
}

// registerTools registers all available MCP tools
func (s *Server) registerTools() error {
	// Register query tool
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name: "query",
		Description: "Execute a read-only Cypher query against the Kubernetes knowledge graph stored in Neo4j. " +
			"This tool allows querying nodes (Pods, Services, Deployments, etc.) and their relationships. " +
			"Only SELECT/MATCH operations are allowed; write operations (CREATE, DELETE, SET, MERGE) are rejected.",
	}, func(ctx context.Context, request *mcp.CallToolRequest, input QueryInput) (*mcp.CallToolResult, any, error) {
		// Validate query is read-only
		if err := ValidateReadOnlyQuery(input.Query); err != nil {
			s.logger.Warn("rejected write operation in query",
				zap.Error(err),
				zap.String("query", input.Query))
			return nil, nil, err
		}

		// Execute query
		s.logger.Debug("executing cypher query",
			zap.String("query", input.Query),
			zap.Any("params", input.Params))

		results, err := s.store.Query(ctx, input.Query, input.Params)
		if err != nil {
			s.logger.Error("query execution failed",
				zap.Error(err),
				zap.String("query", input.Query))
			return nil, nil, fmt.Errorf("failed to execute query: %w", err)
		}

		s.logger.Info("query executed successfully",
			zap.Int("result_count", len(results)),
			zap.String("query", input.Query))

		output := QueryOutput{
			Results: results,
			Count:   len(results),
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("Query returned %d results", len(results)),
				},
			},
		}, output, nil
	})

	// Register structure tool
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name: "structure",
		Description: "Get a complete overview of the graph database schema, including all node types (labels), " +
			"relationship types, their properties, and the from-relationship-to triplets that describe " +
			"the graph structure. This is useful for understanding what data is available and how to query it.",
	}, func(ctx context.Context, request *mcp.CallToolRequest, input StructureInput) (*mcp.CallToolResult, any, error) {
		s.logger.Debug("fetching graph structure")

		output, err := StructureTool(ctx, s.store, s.logger)
		if err != nil {
			return nil, nil, err
		}

		// Format output as text
		text := fmt.Sprintf(
			"Graph Schema Overview:\n"+
				"- Node Types: %d\n"+
				"- Relationship Types: %d\n"+
				"- Schema Triplets: %d\n\n"+
				"Use the query tool with Cypher to explore the data.",
			len(output.NodeTypes),
			len(output.RelationshipTypes),
			len(output.SchemaTriplets),
		)

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: text,
				},
			},
		}, output, nil
	})

	s.logger.Info("registered MCP tools",
		zap.Strings("tools", []string{"query", "structure"}))

	return nil
}

// GetMCPServer returns the underlying MCP server instance
func (s *Server) GetMCPServer() *mcp.Server {
	return s.mcpServer
}

// Close closes the MCP server and releases resources
func (s *Server) Close() error {
	s.logger.Info("closing MCP server")
	// The SDK server doesn't have explicit Close method
	// Resources are managed by the HTTP server lifecycle
	return nil
}
