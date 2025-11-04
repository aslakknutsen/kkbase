package mcp

import (
	"context"
	"fmt"
	"time"

	"github.com/kagenti/kkbase/pkg/graph"
	"github.com/kagenti/kkbase/pkg/models"
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
	sessionManager   *observability.AgentSessionManager
	broadcaster      *NotificationBroadcaster
}

// ServerOption is a function that configures the Server
type ServerOption func(*Server)

// WithMetricsProcessor sets the investigation metrics processor for the server
func WithMetricsProcessor(processor *observability.InvestigationMetricsProcessor) ServerOption {
	return func(s *Server) {
		s.metricsProcessor = processor
	}
}

// WithAgentSessionManager sets the agent session manager for the server
func WithAgentSessionManager(manager *observability.AgentSessionManager) ServerOption {
	return func(s *Server) {
		s.sessionManager = manager
	}
}

// WithBroadcaster sets the notification broadcaster for the server
func WithBroadcaster(broadcaster *NotificationBroadcaster) ServerOption {
	return func(s *Server) {
		s.broadcaster = broadcaster
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

		// Format results as readable text with JSON fallback for large datasets
		var resultText string
		if len(results) == 0 {
			resultText = "Query returned 0 results"
		} else if len(results) <= 10 {
			// For small result sets, format as readable text
			resultText = fmt.Sprintf("Query returned %d results:\n\n", len(results))
			for i, result := range results {
				resultText += fmt.Sprintf("Result %d:\n", i+1)
				for key, value := range result {
					resultText += fmt.Sprintf("  %s: %v\n", key, value)
				}
				resultText += "\n"
			}
		} else {
			// For large result sets, provide summary + sample
			resultText = fmt.Sprintf("Query returned %d results. Showing first 5:\n\n", len(results))
			for i := 0; i < 5 && i < len(results); i++ {
				resultText += fmt.Sprintf("Result %d:\n", i+1)
				for key, value := range results[i] {
					resultText += fmt.Sprintf("  %s: %v\n", key, value)
				}
				resultText += "\n"
			}
			resultText += fmt.Sprintf("... and %d more results (use LIMIT in query to refine)\n", len(results)-5)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: resultText,
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

		// Format detailed output as text
		text := fmt.Sprintf(
			"Graph Schema Overview:\n"+
				"- Node Types: %d\n"+
				"- Relationship Types: %d\n"+
				"- Schema Triplets: %d\n\n",
			len(output.NodeTypes),
			len(output.RelationshipTypes),
			len(output.SchemaTriplets),
		)

		// Show node types with properties (limit to first 15 for readability)
		text += "=== NODE TYPES ===\n"
		maxNodeTypes := 50
		if len(output.NodeTypes) > maxNodeTypes {
			text += fmt.Sprintf("Showing %d of %d node types:\n\n", maxNodeTypes, len(output.NodeTypes))
		}
		for i, nodeType := range output.NodeTypes {
			if i >= maxNodeTypes {
				text += fmt.Sprintf("... and %d more node types\n\n", len(output.NodeTypes)-maxNodeTypes)
				break
			}
			text += fmt.Sprintf("%d. %s\n", i+1, nodeType)
			if props, ok := output.NodeProperties[nodeType]; ok && len(props) > 0 {
				text += fmt.Sprintf("   Properties (%d): ", len(props))
				// Show first 8 properties inline
				displayProps := props
				if len(props) > 8 {
					displayProps = props[:8]
					text += fmt.Sprintf("%s, ... (+%d more)\n", formatList(displayProps), len(props)-8)
				} else {
					text += fmt.Sprintf("%s\n", formatList(displayProps))
				}
			}
			text += "\n"
		}

		// Show relationship types
		text += "=== RELATIONSHIP TYPES ===\n"
		for i, relType := range output.RelationshipTypes {
			text += fmt.Sprintf("%d. %s", i+1, relType)
			if props, ok := output.RelationshipProperties[relType]; ok && len(props) > 0 {
				text += fmt.Sprintf(" (properties: %s)", formatList(props))
			}
			text += "\n"
		}
		text += "\n"

		// Show schema triplets (top 25)
		text += "=== SCHEMA TRIPLETS (Graph Structure) ===\n"
		maxTriplets := 50
		if len(output.SchemaTriplets) > maxTriplets {
			text += fmt.Sprintf("Showing %d of %d relationships:\n", maxTriplets, len(output.SchemaTriplets))
		}
		for i, triplet := range output.SchemaTriplets {
			if i >= maxTriplets {
				text += fmt.Sprintf("... and %d more relationships\n", len(output.SchemaTriplets)-maxTriplets)
				break
			}
			text += fmt.Sprintf("  %s -[%s]-> %s\n", triplet.From, triplet.Relationship, triplet.To)
		}

		text += "\nUse the query tool with Cypher to explore the data in detail.\n"

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

			// Get breakdown of metrics by name
			breakdownQuery := `
			MATCH (m:Metric {investigation_id: $investigation_id})
			RETURN m.metric_name as metric_name, count(m) as data_points
			ORDER BY data_points DESC
		`
			breakdownResults, err := s.store.Query(ctx, breakdownQuery, map[string]interface{}{
				"investigation_id": session.ID,
			})

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

			// Format detailed output message
			detailedMessage := fmt.Sprintf("Investigation started: %s\n\n", session.ID)
			detailedMessage += fmt.Sprintf("Resource: %s (%s)\n", session.ResourceID, session.ResourceType)
			detailedMessage += fmt.Sprintf("Symptom: %s\n", session.Symptom)
			detailedMessage += fmt.Sprintf("Lookback: %v\n", lookback)
			detailedMessage += fmt.Sprintf("Status: %s\n\n", session.Status)

			if metricsCollected > 0 {
				detailedMessage += fmt.Sprintf("✓ Metrics Collected: %d data points\n", metricsCollected)
				if err == nil && len(breakdownResults) > 0 {
					detailedMessage += "\nMetrics Breakdown:\n"
					for i, result := range breakdownResults {
						if i >= 10 {
							detailedMessage += fmt.Sprintf("  ... and %d more metric types\n", len(breakdownResults)-10)
							break
						}
						metricName := ""
						dataPoints := int64(0)
						if mn, ok := result["metric_name"].(string); ok {
							metricName = mn
						}
						if dp, ok := result["data_points"].(int64); ok {
							dataPoints = dp
						}
						detailedMessage += fmt.Sprintf("  - %s: %d points\n", metricName, dataPoints)
					}
				}
				detailedMessage += "\nQuery metrics with:\n"
				detailedMessage += fmt.Sprintf("  MATCH (m:Metric {investigation_id: '%s'})\n", session.ID)
				detailedMessage += "  WHERE m.metric_name = 'container_memory_working_set_bytes'\n"
				detailedMessage += "  RETURN m.timestamp, m.value ORDER BY m.timestamp\n"
			} else {
				detailedMessage += "⚠ WARNING: No metrics collected!\n\n"
				detailedMessage += "Possible causes:\n"
				detailedMessage += "  - Prometheus is not configured (check PROMETHEUS_URL)\n"
				detailedMessage += "  - No data available for this resource in Prometheus\n"
				detailedMessage += "  - Resource ID may be incorrect\n"
				detailedMessage += fmt.Sprintf("  - Time window may be too narrow (current: %v)\n", lookback)
			}

			detailedMessage += fmt.Sprintf("\nUse complete_investigation('%s') when done to cleanup.\n", session.ID)

			s.logger.Info("investigation started",
				zap.String("investigation_id", session.ID),
				zap.Int("metrics_collected", metricsCollected))

			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{
						Text: detailedMessage,
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
				"investigation_id": models.GetNodeID("Investigation", "", input.InvestigationID),
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
				"investigation_id": models.GetNodeID("Investigation", "", input.InvestigationID),
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

			// Count metrics for this investigation
			metricsCountQuery := `
			MATCH (m:Metric {investigation_id: $investigation_id})
			RETURN count(m) as metric_count
		`
			metricsResults, err := s.store.Query(ctx, metricsCountQuery, map[string]interface{}{
				"investigation_id": input.InvestigationID,
			})
			metricCount := 0
			if err == nil && len(metricsResults) > 0 {
				if count, ok := metricsResults[0]["metric_count"].(int64); ok {
					metricCount = int(count)
				}
			}

			// Format detailed status message
			statusMessage := fmt.Sprintf("Investigation Status: %s\n\n", input.InvestigationID)
			statusMessage += fmt.Sprintf("Status: %s\n", output.Status)
			statusMessage += fmt.Sprintf("Resource: %s (%s)\n", output.ResourceID, output.ResourceType)
			statusMessage += fmt.Sprintf("Symptom: %s\n", output.Symptom)
			statusMessage += fmt.Sprintf("Started: %s\n", output.StartTime)
			statusMessage += fmt.Sprintf("Lookback Duration: %s\n", output.LookbackDuration)
			statusMessage += fmt.Sprintf("Metrics Collected: %d data points\n", metricCount)

			if metricCount > 0 {
				statusMessage += "\nTo query metrics:\n"
				statusMessage += fmt.Sprintf("  MATCH (m:Metric {investigation_id: '%s'})\n", input.InvestigationID)
				statusMessage += "  RETURN m.metric_name, m.timestamp, m.value\n"
				statusMessage += "  ORDER BY m.timestamp\n"
			}

			if output.Status == "active" {
				statusMessage += fmt.Sprintf("\nRemember to call complete_investigation('%s') when done.\n", input.InvestigationID)
			}

			s.logger.Debug("investigation status retrieved",
				zap.String("investigation_id", input.InvestigationID),
				zap.String("status", output.Status))

			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{
						Text: statusMessage,
					},
				},
			}, output, nil
		})

		toolNames = append(toolNames, "start_investigation", "complete_investigation", "get_investigation_status")
	}

	// Register agent session tools (if session manager available)
	if s.sessionManager != nil {
		if err := s.registerAgentSessionTools(s.sessionManager, s.broadcaster); err != nil {
			s.logger.Warn("failed to register agent session tools", zap.Error(err))
		}

		if err := s.registerAgentSessionResources(s.sessionManager); err != nil {
			s.logger.Warn("failed to register agent session resources", zap.Error(err))
		}
	}

	s.logger.Info("registered MCP tools",
		zap.Strings("tools", toolNames))

	return nil
}

// formatList formats a string slice as a comma-separated list
func formatList(items []string) string {
	if len(items) == 0 {
		return ""
	}
	if len(items) == 1 {
		return items[0]
	}
	result := ""
	for i, item := range items {
		if i > 0 {
			result += ", "
		}
		result += item
	}
	return result
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
