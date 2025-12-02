package mcp

import (
	"context"
	"fmt"

	"github.com/aslakknutsen/kkbase/pkg/observability"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"
)

// registerAgentSessionTools registers all agent investigation session tools
func (s *Server) registerAgentSessionTools(sessionManager *observability.AgentSessionManager, broadcaster *NotificationBroadcaster) error {
	if sessionManager == nil {
		s.logger.Info("agent session manager not available, skipping agent session tools")
		return nil
	}

	// Tool 1: start_agent_session
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name: "start_agent_session",
		Description: "Start a new AI agent diagnostic session for tracking exploratory investigation. " +
			"Returns a session ID that should be included in all subsequent tool calls (query_with_session, update_hypothesis, etc). " +
			"The session automatically tracks hypotheses, queries, findings, and dynamically calculates blast zone as the investigation progresses.",
	}, func(ctx context.Context, request *mcp.CallToolRequest, input StartAgentSessionInput) (*mcp.CallToolResult, any, error) {
		s.logger.Info("starting agent session",
			zap.String("symptom", input.Symptom),
			zap.String("event_id", input.EventID),
			zap.String("event_source", input.EventSource))

		session, err := sessionManager.CreateSession(ctx, input.Symptom, input.InitialResource, input.EventID, input.EventSource, input.EventTimestamp)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to start agent session: %w", err)
		}

		// Find patterns matching the symptom
		patterns, err := sessionManager.FindPatternsBySymptom(ctx, input.Symptom)
		if err != nil {
			s.logger.Warn("failed to find patterns by symptom", zap.Error(err))
			patterns = []observability.Pattern{} // Continue with empty patterns
		}

		// Mark patterns as presented and convert to output format
		suggestedPatterns := make([]PatternInfo, 0, len(patterns))
		for _, pattern := range patterns {
			// Mark pattern as presented
			if err := sessionManager.MarkPatternPresented(ctx, session.ID, pattern.ID); err != nil {
				s.logger.Warn("failed to mark pattern as presented",
					zap.String("pattern_id", pattern.ID),
					zap.Error(err))
			}

			suggestedPatterns = append(suggestedPatterns, PatternInfo{
				ID:                    pattern.ID,
				Name:                  pattern.Name,
				RootCauseResourceType: pattern.RootCauseResourceType,
				RootCauseIssueType:    pattern.RootCauseIssueType,
				InvestigationSteps:    pattern.InvestigationSteps,
				DiagnosisGuidance:     pattern.DiagnosisGuidance,
				UsageCount:            pattern.UsageCount,
			})
		}

		// Emit notification
		if broadcaster != nil {
			broadcaster.EmitSessionCreated(session.ID, input.Symptom)
		}

		output := StartAgentSessionOutput{
			SessionID:         session.ID,
			Status:            "active",
			SuggestedPatterns: suggestedPatterns,
			Message: fmt.Sprintf("Agent session started successfully. Use session_id '%s' in query_with_session calls. "+
				"Update hypothesis with update_hypothesis as your investigation progresses.", session.ID),
		}

		// Build response text with pattern information
		responseText := fmt.Sprintf("🔬 Agent Session Started\n\nSession ID: %s\nInitial Symptom: %s\nStatus: %s\n\n",
			session.ID, input.Symptom, output.Status)

		if len(suggestedPatterns) > 0 {
			responseText += fmt.Sprintf("📚 %d Suggested Pattern(s) Found:\n\n", len(suggestedPatterns))
			for i, p := range suggestedPatterns {
				responseText += fmt.Sprintf("%d. %s (used %d times)\n", i+1, p.Name, p.UsageCount)
				responseText += fmt.Sprintf("   Expected: %s + %s\n", p.RootCauseResourceType, p.RootCauseIssueType)
				responseText += fmt.Sprintf("   Guidance: %s\n", p.DiagnosisGuidance)
				if len(p.InvestigationSteps) > 0 {
					responseText += "   Steps:\n"
					for j, step := range p.InvestigationSteps {
						responseText += fmt.Sprintf("     %d. %s\n", j+1, step)
					}
				}
				responseText += "\n"
			}
			responseText += "Use mark_pattern_used tool if a pattern helps guide your investigation.\n\n"
		}

		responseText += "Next steps:\n" +
			"1. Use query_with_session to execute Cypher queries\n" +
			"2. Use update_hypothesis to record your diagnostic hypothesis\n" +
			"3. Findings will be automatically extracted from query results\n" +
			"4. Blast zone will be calculated dynamically as findings emerge"

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: responseText,
				},
			},
		}, output, nil
	})

	// Tool 2: query_with_session
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name: "query_with_session",
		Description: "Execute a Cypher query within an active agent session. This automatically:\n" +
			"1. Records the query and your reasoning\n" +
			"2. Executes the query against the knowledge graph\n" +
			"3. Extracts findings from results (failed calls, unhealthy pods, errors)\n" +
			"4. Links findings to affected resources\n" +
			"5. Updates session state\n\n" +
			"Use this instead of the regular 'query' tool when working within an investigation session.",
	}, func(ctx context.Context, request *mcp.CallToolRequest, input QueryWithSessionInput) (*mcp.CallToolResult, any, error) {
		s.logger.Info("executing query in session",
			zap.String("session_id", input.SessionID),
			zap.String("reasoning", input.Reasoning))

		// Validate query is read-only
		if err := ValidateReadOnlyQuery(input.Query); err != nil {
			return nil, nil, err
		}

		// Execute query with session tracking
		queryExec, results, findings, err := sessionManager.ExecuteQuery(
			ctx, input.SessionID, input.Query, input.Reasoning, input.Params,
		)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to execute query: %w", err)
		}

		// Convert findings to output format
		findingOutputs := make([]FindingOutput, len(findings))
		for i, finding := range findings {
			findingOutputs[i] = FindingOutput{
				FindingID:   finding.ID,
				Type:        finding.Type,
				Severity:    finding.Severity,
				ResourceID:  finding.ResourceID,
				Description: finding.Description,
				Evidence:    finding.Evidence,
			}
		}

		output := QueryWithSessionOutput{
			QueryID:      queryExec.ID,
			Results:      results,
			Count:        len(results),
			Findings:     findingOutputs,
			FindingCount: len(findings),
		}

		// Emit notifications
		if broadcaster != nil {
			broadcaster.EmitQueryExecuted(input.SessionID, queryExec.ID, len(findings))

			// Emit finding notifications
			for _, finding := range findings {
				broadcaster.EmitFindingDiscovered(input.SessionID, finding.ID, finding.Type, finding.Severity)
			}
		}

		// Format output text
		resultText := fmt.Sprintf("Query executed successfully\n\nQuery ID: %s\nResults: %d records\n", queryExec.ID, len(results))

		if len(findings) > 0 {
			resultText += fmt.Sprintf("\n🔍 Automatically extracted %d finding(s):\n", len(findings))
			for i, finding := range findings {
				resultText += fmt.Sprintf("%d. [%s] %s - %s\n", i+1, finding.Severity, finding.Type, finding.Description)
			}
		} else {
			resultText += "\nNo issues detected in results.\n"
		}

		// Add sample results
		if len(results) > 0 && len(results) <= 5 {
			resultText += "\nResults:\n"
			for i, result := range results {
				resultText += fmt.Sprintf("\nResult %d:\n", i+1)
				for key, value := range result {
					resultText += fmt.Sprintf("  %s: %v\n", key, value)
				}
			}
		} else if len(results) > 5 {
			resultText += fmt.Sprintf("\nShowing first 5 of %d results:\n", len(results))
			for i := 0; i < 5; i++ {
				resultText += fmt.Sprintf("\nResult %d:\n", i+1)
				for key, value := range results[i] {
					resultText += fmt.Sprintf("  %s: %v\n", key, value)
				}
			}
			resultText += fmt.Sprintf("\n... and %d more results\n", len(results)-5)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: resultText,
				},
			},
		}, output, nil
	})

	// Tool 3: update_hypothesis
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name: "update_hypothesis",
		Description: "Update the current diagnostic hypothesis for an investigation session. " +
			"This marks the current investigation stage and triggers blast zone recalculation. " +
			"Call this at the end of each investigation round when you've refined your understanding of the problem. " +
			"Previous hypotheses are marked as 'superseded' while the new one becomes 'active'.",
	}, func(ctx context.Context, request *mcp.CallToolRequest, input UpdateHypothesisInput) (*mcp.CallToolResult, any, error) {
		s.logger.Info("updating hypothesis",
			zap.String("session_id", input.SessionID),
			zap.Int("stage", input.Stage))

		hypothesis, err := sessionManager.UpdateHypothesis(ctx, input.SessionID, input.Stage, input.Text)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to update hypothesis: %w", err)
		}

		// Emit notifications
		if broadcaster != nil {
			broadcaster.EmitHypothesisUpdated(input.SessionID, input.Stage, input.Text)
			broadcaster.EmitBlastZoneUpdated(input.SessionID, 0, 0) // Counts will be updated when blast zone is calculated
		}

		output := UpdateHypothesisOutput{
			HypothesisID:     hypothesis.ID,
			Stage:            input.Stage,
			BlastZoneUpdated: true,
			Message:          fmt.Sprintf("Hypothesis updated for stage %d. Blast zone recalculated.", input.Stage),
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("💡 Hypothesis Updated\n\nStage: %d\nHypothesis: %s\n\nBlast zone has been recalculated based on current findings.",
						input.Stage, input.Text),
				},
			},
		}, output, nil
	})

	// Tool 4: record_finding
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name: "record_finding",
		Description: "Explicitly record a finding discovered during investigation. " +
			"Use this when you synthesize an insight that wasn't automatically detected by query_with_session. " +
			"For example: correlating timeline events, identifying deployment changes, or making connections between multiple queries. " +
			"Findings recorded this way are marked as 'agent_recorded' (vs 'automatic' for auto-detected findings).",
	}, func(ctx context.Context, request *mcp.CallToolRequest, input RecordFindingInput) (*mcp.CallToolResult, any, error) {
		s.logger.Info("recording finding",
			zap.String("session_id", input.SessionID),
			zap.String("type", input.Type))

		finding := &observability.Finding{
			Type:        input.Type,
			ResourceID:  input.ResourceID,
			Description: input.Description,
			Severity:    input.Severity,
			Evidence:    input.Evidence,
		}

		err := sessionManager.RecordFinding(ctx, input.SessionID, finding)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to record finding: %w", err)
		}

		// Emit notification
		if broadcaster != nil {
			broadcaster.EmitFindingDiscovered(input.SessionID, finding.ID, finding.Type, finding.Severity)
		}

		output := RecordFindingOutput{
			FindingID: finding.ID,
			Message:   fmt.Sprintf("Finding recorded: %s", finding.Description),
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("📌 Finding Recorded\n\nType: %s\nSeverity: %s\nResource: %s\nDescription: %s\n\nThis agent-recorded finding has been added to the session.",
						input.Type, input.Severity, input.ResourceID, input.Description),
				},
			},
		}, output, nil
	})

	// Tool 5: record_recommendation
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name: "record_recommendation",
		Description: "Record an actionable recommendation based on investigation findings. " +
			"Use this to suggest concrete next steps for resolving the root cause or addressing " +
			"other issues discovered during investigation. Recommendations should be specific, " +
			"actionable, and prioritized. Include related finding IDs to show the evidence supporting this recommendation.",
	}, func(ctx context.Context, request *mcp.CallToolRequest, input RecordRecommendationInput) (*mcp.CallToolResult, any, error) {
		s.logger.Info("recording recommendation",
			zap.String("session_id", input.SessionID),
			zap.String("type", input.Type),
			zap.String("priority", input.Priority))

		recommendation := &observability.Recommendation{
			Type:            input.Type,
			Priority:        input.Priority,
			Title:           input.Title,
			Description:     input.Description,
			Rationale:       input.Rationale,
			RelatedFindings: input.RelatedFindings,
			ActionItems:     input.ActionItems,
			EstimatedEffort: input.EstimatedEffort,
			AutomationHint:  input.AutomationHint,
			Tags:            input.Tags,
			Metadata:        input.Metadata,
		}

		err := sessionManager.RecordRecommendation(ctx, input.SessionID, recommendation)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to record recommendation: %w", err)
		}

		// Emit notification
		if broadcaster != nil {
			broadcaster.EmitRecommendationRecorded(input.SessionID, recommendation.ID, recommendation.Priority, recommendation.Type)
		}

		output := RecordRecommendationOutput{
			RecommendationID: recommendation.ID,
			Message:          fmt.Sprintf("Recommendation recorded: %s", recommendation.Title),
		}

		// Format priority emoji
		priorityEmoji := map[string]string{
			"critical": "🔴",
			"high":     "🟠",
			"medium":   "🟡",
			"low":      "🟢",
		}[input.Priority]

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("%s Recommendation Recorded\n\nPriority: %s\nType: %s\nTitle: %s\n\nDescription: %s\n\nRationale: %s\n\nAction Items:\n%s\n\nThis recommendation has been linked to %d finding(s).",
						priorityEmoji, input.Priority, input.Type, input.Title,
						input.Description, input.Rationale,
						formatActionItems(input.ActionItems),
						len(input.RelatedFindings)),
				},
			},
		}, output, nil
	})

	// Tool 5.5: record_pattern
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name: "record_pattern",
		Description: "Record a diagnostic pattern discovered during investigation. " +
			"Use this to capture reusable knowledge about symptom → investigation → diagnosis → resolution. " +
			"Patterns are matched strictly on resource_type + issue_type.",
	}, func(ctx context.Context, request *mcp.CallToolRequest, input RecordPatternInput) (*mcp.CallToolResult, any, error) {
		s.logger.Info("recording pattern",
			zap.String("session_id", input.SessionID),
			zap.String("name", input.Name),
			zap.String("resource_type", input.RootCauseResourceType))

		pattern, err := sessionManager.RecordPattern(ctx, input.SessionID, &observability.Pattern{
			Name:                  input.Name,
			RootCauseResourceType: input.RootCauseResourceType,
			RootCauseIssueType:    input.RootCauseIssueType,
			SymptomKeywords:       input.SymptomKeywords,
			InvestigationSteps:    input.InvestigationSteps,
			DiagnosisGuidance:     input.DiagnosisGuidance,
			Recommendations:       input.Recommendations,
			Metadata:              input.Metadata,
		})

		if err != nil {
			return nil, nil, fmt.Errorf("failed to record pattern: %w", err)
		}

		output := RecordPatternOutput{
			PatternID: pattern.ID,
			Status:    "recorded",
			Message: fmt.Sprintf("Pattern recorded: %s (matches: %s + %s)",
				pattern.Name, pattern.RootCauseResourceType, pattern.RootCauseIssueType),
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("✓ Pattern Recorded\n\nID: %s\nName: %s\nMatch Key: %s + %s\nSteps: %d",
						pattern.ID, pattern.Name, pattern.RootCauseResourceType,
						pattern.RootCauseIssueType, len(pattern.InvestigationSteps)),
				},
			},
		}, output, nil
	})

	// Tool 6: spawn_investigation
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name: "spawn_investigation",
		Description: "Spawn a metrics investigation session linked to the current agent session. " +
			"Use this when you need to examine metrics data (CPU, memory, network, etc.) for a specific resource. " +
			"This creates an Investigation node and pulls historical metrics from Prometheus. " +
			"The investigation is linked to the agent session and optionally to the current hypothesis.",
	}, func(ctx context.Context, request *mcp.CallToolRequest, input SpawnInvestigationInput) (*mcp.CallToolResult, any, error) {
		s.logger.Info("spawning investigation",
			zap.String("session_id", input.SessionID),
			zap.String("resource", input.ResourceID))

		investigationID, err := sessionManager.SpawnInvestigation(
			ctx,
			input.SessionID,
			input.HypothesisID,
			input.ResourceType,
			input.ResourceID,
			input.Symptom,
			input.LookbackMinutes,
		)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to spawn investigation: %w", err)
		}

		// Emit notification
		if broadcaster != nil {
			broadcaster.EmitInvestigationSpawned(input.SessionID, investigationID, input.ResourceID)
		}

		output := SpawnInvestigationOutput{
			InvestigationID: investigationID,
			SessionID:       input.SessionID,
			Message:         fmt.Sprintf("Metrics investigation spawned: %s", investigationID),
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("📊 Metrics Investigation Spawned\n\nInvestigation ID: %s\nResource: %s (%s)\nSymptom: %s\nLookback: %d minutes\n\n"+
						"Query metrics with:\nMATCH (m:Metric {investigation_id: '%s'}) RETURN m",
						investigationID, input.ResourceID, input.ResourceType, input.Symptom, input.LookbackMinutes, investigationID),
				},
			},
		}, output, nil
	})

	// Tool 6: complete_agent_session
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name: "complete_agent_session",
		Description: "Mark an agent session as completed and generate final summary. " +
			"Call this when the investigation is finished and you've identified the root cause. " +
			"This finalizes the blast zone snapshot, completes any linked investigations, and generates a summary report. " +
			"Optionally specify a status (e.g., 'timeout' if iteration limit reached, 'incomplete' for partial results).",
	}, func(ctx context.Context, request *mcp.CallToolRequest, input CompleteAgentSessionInput) (*mcp.CallToolResult, any, error) {
		s.logger.Info("completing agent session",
			zap.String("session_id", input.SessionID),
			zap.String("status", input.Status))

		// Pass status to CompleteSession (defaults to "completed" if empty)
		summary, err := sessionManager.CompleteSession(ctx, input.SessionID, input.Summary, input.Status)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to complete session: %w", err)
		}

		// Emit notification
		if broadcaster != nil {
			broadcaster.EmitSessionCompleted(input.SessionID, summary.TotalFindings, summary.TotalQueries)
		}

		// Determine final status for display
		finalStatus := input.Status
		if finalStatus == "" {
			finalStatus = "completed"
		}

		output := CompleteAgentSessionOutput{
			SessionID:    summary.SessionID,
			Status:       finalStatus,
			Duration:     summary.Duration.String(),
			QueryCount:   summary.TotalQueries,
			FindingCount: summary.TotalFindings,
			Message:      fmt.Sprintf("Agent session completed with status: %s", finalStatus),
		}

		statusEmoji := "✅"
		if finalStatus == "timeout" {
			statusEmoji = "⏱️"
		} else if finalStatus == "incomplete" {
			statusEmoji = "⚠️"
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("%s Investigation Complete (%s)\n\nSession ID: %s\nDuration: %s\nQueries Executed: %d\nFindings Discovered: %d\n\nInitial Symptom: %s\nFinal Hypothesis: %s\n\nRoot Cause: %s",
						statusEmoji, finalStatus, summary.SessionID, summary.Duration, summary.TotalQueries, summary.TotalFindings,
						summary.InitialSymptom, summary.FinalHypothesis, summary.RootCause),
				},
			},
		}, output, nil
	})

	// Tool 7: get_patterns
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name: "get_patterns",
		Description: "Query patterns by resource type, issue type, or symptom keywords. " +
			"Patterns provide investigation guidance based on previous successful investigations. " +
			"This creates PRESENTED_PATTERN relationships for tracking.",
	}, func(ctx context.Context, request *mcp.CallToolRequest, input GetPatternsInput) (*mcp.CallToolResult, any, error) {
		s.logger.Info("querying patterns",
			zap.String("session_id", input.SessionID),
			zap.String("resource_type", input.ResourceType),
			zap.String("issue_type", input.IssueType))

		var patterns []observability.Pattern
		var err error

		// Query by type if both provided
		if input.ResourceType != "" && input.IssueType != "" {
			patterns, err = sessionManager.FindPatternsByType(ctx, input.ResourceType, input.IssueType)
		} else if len(input.SymptomKeywords) > 0 {
			// Query by symptom keywords
			allPatterns := []observability.Pattern{}
			for _, keyword := range input.SymptomKeywords {
				matched, e := sessionManager.FindPatternsBySymptom(ctx, keyword)
				if e == nil {
					allPatterns = append(allPatterns, matched...)
				}
			}
			patterns = allPatterns
		} else {
			return nil, nil, fmt.Errorf("must provide either resource_type+issue_type or symptom_keywords")
		}

		if err != nil {
			return nil, nil, fmt.Errorf("failed to query patterns: %w", err)
		}

		// Mark patterns as presented
		patternInfos := make([]PatternInfo, 0, len(patterns))
		for _, pattern := range patterns {
			if err := sessionManager.MarkPatternPresented(ctx, input.SessionID, pattern.ID); err != nil {
				s.logger.Warn("failed to mark pattern as presented",
					zap.String("pattern_id", pattern.ID),
					zap.Error(err))
			}

			patternInfos = append(patternInfos, PatternInfo{
				ID:                    pattern.ID,
				Name:                  pattern.Name,
				RootCauseResourceType: pattern.RootCauseResourceType,
				RootCauseIssueType:    pattern.RootCauseIssueType,
				InvestigationSteps:    pattern.InvestigationSteps,
				DiagnosisGuidance:     pattern.DiagnosisGuidance,
				UsageCount:            pattern.UsageCount,
			})
		}

		output := GetPatternsOutput{
			Patterns: patternInfos,
			Count:    len(patternInfos),
			Message:  fmt.Sprintf("Found %d pattern(s)", len(patternInfos)),
		}

		// Format response text
		responseText := fmt.Sprintf("📚 Found %d Pattern(s)\n\n", len(patternInfos))
		if len(patternInfos) == 0 {
			responseText += "No patterns match your query. Consider recording a new pattern if you successfully resolve this issue.\n"
		} else {
			for i, p := range patternInfos {
				responseText += fmt.Sprintf("%d. %s (used %d times)\n", i+1, p.Name, p.UsageCount)
				responseText += fmt.Sprintf("   Root Cause: %s + %s\n", p.RootCauseResourceType, p.RootCauseIssueType)
				responseText += fmt.Sprintf("   Guidance: %s\n", p.DiagnosisGuidance)
				if len(p.InvestigationSteps) > 0 {
					responseText += "   Investigation Steps:\n"
					for j, step := range p.InvestigationSteps {
						responseText += fmt.Sprintf("     %d. %s\n", j+1, step)
					}
				}
				responseText += "\n"
			}
			responseText += "If a pattern helps, use mark_pattern_used to track its effectiveness.\n"
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: responseText,
				},
			},
		}, output, nil
	})

	// Tool 8: mark_pattern_used
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name: "mark_pattern_used",
		Description: "Mark a pattern as helpful during investigation. " +
			"This creates a USED_PATTERN relationship and increments the pattern's usage count. " +
			"Use this when a pattern successfully guides your investigation.",
	}, func(ctx context.Context, request *mcp.CallToolRequest, input MarkPatternUsedInput) (*mcp.CallToolResult, any, error) {
		s.logger.Info("marking pattern as used",
			zap.String("session_id", input.SessionID),
			zap.String("pattern_id", input.PatternID))

		err := sessionManager.MarkPatternUsed(ctx, input.SessionID, input.PatternID, input.Notes)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to mark pattern as used: %w", err)
		}

		output := MarkPatternUsedOutput{
			PatternID: input.PatternID,
			Message:   "Pattern marked as used successfully",
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: fmt.Sprintf("✓ Pattern Marked as Used\n\nPattern ID: %s\n\nThis pattern has been recorded as helpful for this investigation.\nThe usage count has been incremented to help future investigations.",
						input.PatternID),
				},
			},
		}, output, nil
	})

	s.logger.Info("registered agent session tools",
		zap.Strings("tools", []string{
			"start_agent_session",
			"query_with_session",
			"update_hypothesis",
			"record_finding",
			"record_recommendation",
			"spawn_investigation",
			"complete_agent_session",
			"get_patterns",
			"mark_pattern_used",
		}))

	return nil
}

// formatActionItems formats action items as a numbered list
func formatActionItems(items []string) string {
	if len(items) == 0 {
		return "  (none specified)"
	}
	result := ""
	for i, item := range items {
		result += fmt.Sprintf("  %d. %s\n", i+1, item)
	}
	return result
}
