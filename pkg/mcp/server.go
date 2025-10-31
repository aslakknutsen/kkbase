package mcp

import (
	"context"
	"fmt"
	"time"

	"github.com/kagenti/kkbase/pkg/graph"
	"github.com/kagenti/kkbase/pkg/observability"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"
)

// Server represents the MCP server instance
type Server struct {
	mcpServer        *mcp.Server
	store            graph.GraphStore
	logger           *zap.Logger
	metricsProcessor *observability.InvestigationMetricsProcessor
}

// ServerOption is a function that configures the Server
type ServerOption func(*Server)

// WithMetricsProcessor sets the investigation metrics processor for the server
func WithMetricsProcessor(processor *observability.InvestigationMetricsProcessor) ServerOption {
	return func(s *Server) {
		s.metricsProcessor = processor
	}
}

// NewServer creates a new MCP server instance
func NewServer(store graph.GraphStore, logger *zap.Logger, opts ...ServerOption) (*Server, error) {
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

	// Apply options
	for _, opt := range opts {
		opt(s)
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

	// Register investigation tools (only if metrics processor is available)
	toolNames := []string{"query", "structure"}

	if s.metricsProcessor != nil {
		// Register start_investigation tool
		mcp.AddTool(s.mcpServer, &mcp.Tool{
			Name: "start_investigation",
			Description: "Start a new RCA investigation for a Kubernetes resource. This will pull relevant metrics " +
				"from Prometheus for the specified resource and symptom, correlate them with the resource in the " +
				"knowledge graph, and store them temporarily for investigation. Returns an investigation ID that " +
				"must be used to complete the investigation and cleanup metrics.",
		}, func(ctx context.Context, request *mcp.CallToolRequest, input StartInvestigationInput) (*mcp.CallToolResult, any, error) {
			s.logger.Info("starting investigation",
				zap.String("resource_type", input.ResourceType),
				zap.String("resource_id", input.ResourceID),
				zap.String("symptom", input.Symptom),
				zap.Int("lookback_minutes", input.LookbackMinutes))

			lookback := time.Duration(input.LookbackMinutes) * time.Minute
			if input.LookbackMinutes == 0 {
				lookback = 15 * time.Minute // default
			}

			session, err := s.metricsProcessor.StartInvestigation(
				ctx,
				input.ResourceType,
				input.ResourceID,
				input.Symptom,
				lookback,
			)
			if err != nil {
				s.logger.Error("failed to start investigation",
					zap.Error(err),
					zap.String("resource_type", input.ResourceType),
					zap.String("resource_id", input.ResourceID))
				return nil, nil, fmt.Errorf("failed to start investigation: %w", err)
			}

			// Count metrics collected for this investigation
			countQuery := `
				MATCH (m:Metric {investigation_id: $investigation_id})
				RETURN count(m) as count
			`
			countResults, err := s.store.Query(ctx, countQuery, map[string]interface{}{
				"investigation_id": session.ID,
			})

			metricsCollected := 0
			if err == nil && len(countResults) > 0 {
				if count, ok := countResults[0]["count"].(int64); ok {
					metricsCollected = int(count)
				}
			}

			output := StartInvestigationOutput{
				InvestigationID:  session.ID,
				Status:           session.Status,
				ResourceType:     session.ResourceType,
				ResourceID:       session.ResourceID,
				Symptom:          session.Symptom,
				MetricsCollected: metricsCollected,
				Message: fmt.Sprintf("Investigation started successfully. Use investigation ID '%s' to query metrics and complete investigation.",
					session.ID),
			}

			s.logger.Info("investigation started",
				zap.String("investigation_id", session.ID),
				zap.Int("metrics_collected", metricsCollected))

			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{
						Text: output.Message,
					},
				},
			}, output, nil
		})

		// Register complete_investigation tool
		mcp.AddTool(s.mcpServer, &mcp.Tool{
			Name: "complete_investigation",
			Description: "Complete an active investigation and purge all associated metrics from the graph. " +
				"This should be called when the RCA investigation is finished to clean up temporary metric data. " +
				"Returns the number of metric data points purged.",
		}, func(ctx context.Context, request *mcp.CallToolRequest, input CompleteInvestigationInput) (*mcp.CallToolResult, any, error) {
			s.logger.Info("completing investigation",
				zap.String("investigation_id", input.InvestigationID))

			if err := s.metricsProcessor.CompleteInvestigation(ctx, input.InvestigationID); err != nil {
				s.logger.Error("failed to complete investigation",
					zap.Error(err),
					zap.String("investigation_id", input.InvestigationID))
				return nil, nil, fmt.Errorf("failed to complete investigation: %w", err)
			}

			// Query for final status
			query := `
				MATCH (i:Investigation {id: $investigation_id})
				RETURN i.status as status, i.metrics_purged as metrics_purged
			`
			results, err := s.store.Query(ctx, query, map[string]interface{}{
				"investigation_id": input.InvestigationID,
			})

			metricsPurged := 0
			status := "completed"
			if err == nil && len(results) > 0 {
				if mp, ok := results[0]["metrics_purged"].(int64); ok {
					metricsPurged = int(mp)
				}
				if st, ok := results[0]["status"].(string); ok {
					status = st
				}
			}

			output := CompleteInvestigationOutput{
				InvestigationID: input.InvestigationID,
				Status:          status,
				MetricsPurged:   metricsPurged,
				Message: fmt.Sprintf("Investigation '%s' completed successfully. Purged %d metric data points.",
					input.InvestigationID, metricsPurged),
			}

			s.logger.Info("investigation completed",
				zap.String("investigation_id", input.InvestigationID),
				zap.Int("metrics_purged", metricsPurged))

			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{
						Text: output.Message,
					},
				},
			}, output, nil
		})

		// Register get_investigation_status tool
		mcp.AddTool(s.mcpServer, &mcp.Tool{
			Name: "get_investigation_status",
			Description: "Get the current status of an investigation including resource details, symptom, " +
				"timeframes, and current status. Useful for checking investigation progress or retrieving " +
				"investigation details.",
		}, func(ctx context.Context, request *mcp.CallToolRequest, input GetInvestigationStatusInput) (*mcp.CallToolResult, any, error) {
			s.logger.Debug("getting investigation status",
				zap.String("investigation_id", input.InvestigationID))

			query := `
				MATCH (i:Investigation {id: $investigation_id})
				RETURN i.status as status, 
				       i.resource_type as resource_type,
				       i.resource_id as resource_id,
				       i.symptom as symptom,
				       i.start_time as start_time,
				       i.lookback_duration as lookback_duration
			`
			results, err := s.store.Query(ctx, query, map[string]interface{}{
				"investigation_id": input.InvestigationID,
			})
			if err != nil {
				s.logger.Error("failed to query investigation status",
					zap.Error(err),
					zap.String("investigation_id", input.InvestigationID))
				return nil, nil, fmt.Errorf("failed to query investigation: %w", err)
			}

			if len(results) == 0 {
				return nil, nil, fmt.Errorf("investigation not found: %s", input.InvestigationID)
			}

			result := results[0]
			output := GetInvestigationStatusOutput{
				InvestigationID:  input.InvestigationID,
				Status:           getStringField(result, "status"),
				ResourceType:     getStringField(result, "resource_type"),
				ResourceID:       getStringField(result, "resource_id"),
				Symptom:          getStringField(result, "symptom"),
				StartTime:        getStringField(result, "start_time"),
				LookbackDuration: getStringField(result, "lookback_duration"),
			}

			s.logger.Debug("investigation status retrieved",
				zap.String("investigation_id", input.InvestigationID),
				zap.String("status", output.Status))

			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{
						Text: fmt.Sprintf("Investigation '%s' status: %s", input.InvestigationID, output.Status),
					},
				},
			}, output, nil
		})

		toolNames = append(toolNames, "start_investigation", "complete_investigation", "get_investigation_status")
	}

	s.logger.Info("registered MCP tools",
		zap.Strings("tools", toolNames))

	return nil
}

// getStringField safely extracts a string field from query results
func getStringField(result map[string]interface{}, field string) string {
	if val, ok := result[field].(string); ok {
		return val
	}
	return ""
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
