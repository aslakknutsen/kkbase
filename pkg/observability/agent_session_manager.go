package observability

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/aslakknutsen/kkbase/pkg/config"
	"github.com/aslakknutsen/kkbase/pkg/graph"
	"github.com/aslakknutsen/kkbase/pkg/models"
	"go.uber.org/zap"
)

// AgentSessionManager manages all agent investigation session operations
type AgentSessionManager struct {
	graphStore       graph.GraphStore
	findingExtractor *FindingExtractor
	blastZoneCalc    *BlastZoneCalculator
	invProcessor     *InvestigationMetricsProcessor // Link to existing metrics system
	config           *config.Config
	logger           *zap.Logger
}

// NewAgentSessionManager creates a new agent session manager
func NewAgentSessionManager(
	graphStore graph.GraphStore,
	invProcessor *InvestigationMetricsProcessor,
	cfg *config.Config,
	logger *zap.Logger,
) *AgentSessionManager {
	return &AgentSessionManager{
		graphStore:       graphStore,
		findingExtractor: NewFindingExtractor(),
		blastZoneCalc:    NewBlastZoneCalculator(graphStore, logger),
		invProcessor:     invProcessor,
		config:           cfg,
		logger:           logger,
	}
}

// CreateSession creates a new agent investigation session
func (asm *AgentSessionManager) CreateSession(ctx context.Context, symptom, initialResource, eventID, eventSource, eventTimestamp string) (*AgentSession, error) {
	session := &AgentSession{
		ID:              generateSessionID(),
		InitialSymptom:  symptom,
		InitialResource: initialResource,
		EventID:         eventID,
		EventSource:     eventSource,
		Status:          "active",
		CreatedAt:       time.Now(),
		CurrentStage:    0,
		QueryCount:      0,
		FindingCount:    0,
	}

	// Parse and set event timestamp if provided
	if eventTimestamp != "" {
		parsedTime, err := time.Parse(time.RFC3339, eventTimestamp)
		if err == nil {
			session.EventTimestamp = &parsedTime
			// Calculate processing delay
			delay := session.CreatedAt.Sub(parsedTime)
			session.ProcessingDelay = &delay
		}
	}

	asm.logger.Info("creating agent session",
		zap.String("session_id", session.ID),
		zap.String("symptom", symptom),
		zap.String("event_id", eventID),
		zap.String("event_source", eventSource))

	// Create session node in Neo4j
	query := `
		CREATE (s:AgentSession {
			id: $id,
			initial_symptom: $symptom,
			initial_resource: $initial_resource,
			event_id: $event_id,
			event_source: $event_source,
			event_timestamp: datetime($event_timestamp),
			status: $status,
			created_at: datetime($created_at),
			current_stage: $stage,
			query_count: 0,
			finding_count: 0,
			placeholder: false
		})
		RETURN s
	`

	params := map[string]interface{}{
		"id":               session.ID,
		"symptom":          symptom,
		"initial_resource": initialResource,
		"event_id":         eventID,
		"event_source":     eventSource,
		"status":           "active",
		"created_at":       session.CreatedAt.Format(time.RFC3339),
		"stage":            0,
	}

	// Only add event_timestamp if it was provided and parsed successfully
	if session.EventTimestamp != nil {
		params["event_timestamp"] = session.EventTimestamp.Format(time.RFC3339)
	} else {
		params["event_timestamp"] = nil
	}

	_, err := asm.graphStore.Query(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("failed to create session node: %w", err)
	}

	asm.logger.Info("agent session created", zap.String("session_id", session.ID))
	return session, nil
}

// ExecuteQuery executes a query within a session with automatic finding extraction
func (asm *AgentSessionManager) ExecuteQuery(
	ctx context.Context,
	sessionID, query, reasoning string,
	params map[string]interface{},
) (*QueryExecution, []map[string]interface{}, []*Finding, error) {

	startTime := time.Now()

	// Execute the query
	results, err := asm.graphStore.Query(ctx, query, params)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to execute query: %w", err)
	}

	duration := time.Since(startTime)

	// Extract findings from results (automatic detection)
	findings := asm.findingExtractor.ExtractFindings(results)

	// Create QueryExecution record
	queryExec := &QueryExecution{
		ID:          generateQueryID(),
		Query:       query,
		Reasoning:   reasoning,
		Params:      params,
		ResultCount: len(results),
		Duration:    duration,
		ExecutedAt:  time.Now(),
		Findings:    make([]string, len(findings)),
	}

	// Store results if enabled (dev mode)
	if asm.config.StoreQueryResults {
		if len(results) <= 100 {
			queryExec.Results = results
			queryExec.Truncated = false
		} else {
			queryExec.Results = results[:100]
			queryExec.Truncated = true
		}
	}

	// Store findings
	for i, finding := range findings {
		finding.DiscoveredAt = queryExec.ExecutedAt
		err := asm.storeFinding(ctx, sessionID, queryExec.ID, finding)
		if err != nil {
			asm.logger.Warn("failed to store finding",
				zap.String("session_id", sessionID),
				zap.Error(err))
		} else {
			queryExec.Findings[i] = finding.ID
		}
	}

	// Store query execution
	err = asm.storeQueryExecution(ctx, sessionID, queryExec)
	if err != nil {
		asm.logger.Warn("failed to store query execution",
			zap.String("session_id", sessionID),
			zap.Error(err))
	}

	// Update session query count
	asm.updateSessionQueryCount(ctx, sessionID)

	asm.logger.Info("query executed in session",
		zap.String("session_id", sessionID),
		zap.String("query_id", queryExec.ID),
		zap.Int("results", len(results)),
		zap.Int("findings", len(findings)),
		zap.Duration("duration", duration))

	return queryExec, results, findings, nil
}

// UpdateHypothesis updates the current hypothesis and triggers blast zone recalculation
func (asm *AgentSessionManager) UpdateHypothesis(ctx context.Context, sessionID string, stage int, text string) (*Hypothesis, error) {
	hypothesis := &Hypothesis{
		ID:        generateHypothesisID(),
		Stage:     stage,
		Text:      text,
		Status:    "active",
		CreatedAt: time.Now(),
	}

	asm.logger.Info("updating hypothesis",
		zap.String("session_id", sessionID),
		zap.Int("stage", stage))

	// Mark previous hypotheses as superseded
	supersedQuery := `
		MATCH (s:AgentSession {id: $session_id})-[:HAS_HYPOTHESIS]->(h:Hypothesis {status: 'active'})
		SET h.status = 'superseded'
	`
	_, _ = asm.graphStore.Query(ctx, supersedQuery, map[string]interface{}{
		"session_id": sessionID,
	})

	// Create new hypothesis
	createQuery := `
		MATCH (s:AgentSession {id: $session_id})
		CREATE (h:Hypothesis {
			id: $id,
			stage: $stage,
			text: $text,
			status: 'active',
			created_at: datetime($created_at),
			placeholder: false
		})
		CREATE (s)-[:HAS_HYPOTHESIS]->(h)
		SET s.current_stage = $stage
		RETURN h
	`

	_, err := asm.graphStore.Query(ctx, createQuery, map[string]interface{}{
		"session_id": sessionID,
		"id":         hypothesis.ID,
		"stage":      stage,
		"text":       text,
		"created_at": hypothesis.CreatedAt.Format(time.RFC3339),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create hypothesis: %w", err)
	}

	// Check if hypothesis implies need for metrics investigation
	if asm.invProcessor != nil && shouldSpawnInvestigation(text) {
		resourceType, resourceID, symptom := extractInvestigationParams(text)
		if resourceType != "" && resourceID != "" {
			asm.logger.Info("hypothesis suggests metrics investigation",
				zap.String("session_id", sessionID),
				zap.String("resource", resourceID))

			_, err := asm.invProcessor.StartInvestigation(
				ctx, resourceType, resourceID, symptom, 15*time.Minute,
			)
			if err == nil {
				// Link investigation to hypothesis
				// (Implementation in spawn investigation method)
			}
		}
	}

	// Recalculate blast zone (requirement from plan)
	_, err = asm.blastZoneCalc.Calculate(ctx, sessionID)
	if err != nil {
		asm.logger.Warn("failed to recalculate blast zone",
			zap.String("session_id", sessionID),
			zap.Error(err))
	}

	asm.logger.Info("hypothesis updated",
		zap.String("session_id", sessionID),
		zap.String("hypothesis_id", hypothesis.ID))

	return hypothesis, nil
}

// RecordFinding explicitly records a finding (agent-recorded, not automatic)
func (asm *AgentSessionManager) RecordFinding(ctx context.Context, sessionID string, finding *Finding) error {
	finding.ID = generateFindingID()
	finding.DetectionMethod = "agent_recorded"
	finding.DiscoveredAt = time.Now()

	err := asm.storeFinding(ctx, sessionID, "", finding)
	if err != nil {
		return fmt.Errorf("failed to record finding: %w", err)
	}

	// Update session finding count
	asm.updateSessionFindingCount(ctx, sessionID)

	asm.logger.Info("finding recorded",
		zap.String("session_id", sessionID),
		zap.String("finding_id", finding.ID),
		zap.String("type", finding.Type))

	return nil
}

// SpawnInvestigation spawns a metrics investigation linked to the session
func (asm *AgentSessionManager) SpawnInvestigation(
	ctx context.Context,
	sessionID, hypothesisID, resourceType, resourceID, symptom string,
	lookbackMinutes int,
) (string, error) {

	if asm.invProcessor == nil {
		return "", fmt.Errorf("investigation processor not available")
	}

	lookback := time.Duration(lookbackMinutes) * time.Minute
	invSession, err := asm.invProcessor.StartInvestigation(
		ctx, resourceType, resourceID, symptom, lookback,
	)
	if err != nil {
		return "", fmt.Errorf("failed to start investigation: %w", err)
	}

	// Link investigation to session and optionally to hypothesis
	linkQuery := `
		MATCH (s:AgentSession {id: $session_id})
		MATCH (i:Investigation {id: $inv_id})
		CREATE (s)-[:SPAWNED_INVESTIGATION]->(i)
	`
	linkParams := map[string]interface{}{
		"session_id": sessionID,
		"inv_id":     models.GetNodeID("Investigation", "", invSession.ID),
	}

	if hypothesisID != "" {
		linkQuery += `
		WITH s, i
		MATCH (h:Hypothesis {id: $hypothesis_id})
		CREATE (h)-[:TRIGGERED_INVESTIGATION]->(i)
		`
		linkParams["hypothesis_id"] = hypothesisID
	}

	_, err = asm.graphStore.Query(ctx, linkQuery, linkParams)
	if err != nil {
		asm.logger.Warn("failed to link investigation to session",
			zap.String("session_id", sessionID),
			zap.String("investigation_id", invSession.ID),
			zap.Error(err))
	}

	asm.logger.Info("investigation spawned",
		zap.String("session_id", sessionID),
		zap.String("investigation_id", invSession.ID),
		zap.String("resource", resourceID))

	return invSession.ID, nil
}

// CompleteSession marks a session as completed and generates summary
func (asm *AgentSessionManager) CompleteSession(ctx context.Context, sessionID, summary, status string) (*SessionSummary, error) {
	completedAt := time.Now()

	// Default to "completed" if no status provided
	if status == "" {
		status = "completed"
	}

	// Update session status
	updateQuery := `
		MATCH (s:AgentSession {id: $session_id})
		SET s.status = $status,
			s.completed_at = datetime($completed_at),
			s.summary = $summary
		RETURN s.created_at as created_at, s.query_count as query_count, s.finding_count as finding_count
	`

	results, err := asm.graphStore.Query(ctx, updateQuery, map[string]interface{}{
		"session_id":   sessionID,
		"completed_at": completedAt.Format(time.RFC3339),
		"summary":      summary,
		"status":       status,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to complete session: %w", err)
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	// Get session details
	session, err := asm.GetSession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session details: %w", err)
	}

	// Get final blast zone
	blastZone, _ := asm.blastZoneCalc.Calculate(ctx, sessionID)

	// Build summary
	sessionSummary := &SessionSummary{
		SessionID:       sessionID,
		InitialSymptom:  session.Session.InitialSymptom,
		Duration:        completedAt.Sub(session.Session.CreatedAt),
		TotalQueries:    session.Session.QueryCount,
		TotalFindings:   session.Session.FindingCount,
		FinalHypothesis: "",
		RootCause:       summary,
		BlastZone:       blastZone,
		CompletedAt:     completedAt,
	}

	if session.CurrentHypothesis != nil {
		sessionSummary.FinalHypothesis = session.CurrentHypothesis.Text
	}

	asm.logger.Info("session completed",
		zap.String("session_id", sessionID),
		zap.Duration("duration", sessionSummary.Duration))

	return sessionSummary, nil
}

// CalculateBlastZone calculates the blast zone for a session
func (asm *AgentSessionManager) CalculateBlastZone(ctx context.Context, sessionID string) (*BlastZoneSnapshot, error) {
	return asm.blastZoneCalc.Calculate(ctx, sessionID)
}

// GetSession retrieves complete session details
func (asm *AgentSessionManager) GetSession(ctx context.Context, sessionID string) (*SessionDetail, error) {
	// Get session
	sessionQuery := `
		MATCH (s:AgentSession {id: $session_id})
		RETURN s
	`
	sessionResults, err := asm.graphStore.Query(ctx, sessionQuery, map[string]interface{}{
		"session_id": sessionID,
	})
	if err != nil || len(sessionResults) == 0 {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	session := parseAgentSession(sessionResults[0]["s"])

	// Get hypotheses
	hypQuery := `
		MATCH (s:AgentSession {id: $session_id})-[:HAS_HYPOTHESIS]->(h:Hypothesis)
		RETURN h
		ORDER BY h.stage DESC
	`
	hypResults, _ := asm.graphStore.Query(ctx, hypQuery, map[string]interface{}{
		"session_id": sessionID,
	})
	hypotheses := parseHypotheses(hypResults)

	// Get queries
	queryQuery := `
		MATCH (s:AgentSession {id: $session_id})-[:EXECUTED_QUERY]->(q:QueryExecution)
		RETURN q
		ORDER BY q.executed_at DESC
		LIMIT 50
	`
	queryResults, _ := asm.graphStore.Query(ctx, queryQuery, map[string]interface{}{
		"session_id": sessionID,
	})
	queries := parseQueryExecutions(queryResults)

	// Get findings
	findingQuery := `
		MATCH (s:AgentSession {id: $session_id})-[:HAS_FINDING]->(f:Finding)
		RETURN f
		ORDER BY f.discovered_at DESC
	`
	findingResults, _ := asm.graphStore.Query(ctx, findingQuery, map[string]interface{}{
		"session_id": sessionID,
	})
	findings := parseFindings(findingResults)

	// Get linked investigations
	invQuery := `
		MATCH (s:AgentSession {id: $session_id})-[:SPAWNED_INVESTIGATION]->(i:Investigation)
		RETURN i.id as investigation_id
	`
	invResults, _ := asm.graphStore.Query(ctx, invQuery, map[string]interface{}{
		"session_id": sessionID,
	})
	investigations := make([]string, len(invResults))
	for i, result := range invResults {
		if invID, ok := result["investigation_id"].(string); ok {
			investigations[i] = invID
		}
	}

	// Get recommendations
	recommendations, err := asm.GetRecommendations(ctx, sessionID)
	if err != nil {
		asm.logger.Warn("failed to get recommendations",
			zap.String("session_id", sessionID),
			zap.Error(err))
		recommendations = []Recommendation{}
	}

	// Get patterns related to this session (presented, used, or discovered)
	// Query each relationship type separately for simplicity
	patterns := []Pattern{}

	// Get presented patterns
	presentedQuery := `
		MATCH (s:AgentSession {id: $session_id})-[:PRESENTED_PATTERN]->(p:Pattern)
		RETURN p, 'presented' as relationship_type
	`
	presentedResults, err := asm.graphStore.Query(ctx, presentedQuery, map[string]interface{}{
		"session_id": sessionID,
	})
	if err != nil {
		asm.logger.Debug("no presented patterns or query failed",
			zap.String("session_id", sessionID),
			zap.Error(err))
	} else if len(presentedResults) > 0 {
		patterns = append(patterns, parsePatternsWithRelationship(presentedResults)...)
	}

	// Get used patterns
	usedQuery := `
		MATCH (s:AgentSession {id: $session_id})-[:USED_PATTERN]->(p:Pattern)
		RETURN p, 'used' as relationship_type
	`
	usedResults, err := asm.graphStore.Query(ctx, usedQuery, map[string]interface{}{
		"session_id": sessionID,
	})
	if err != nil {
		asm.logger.Debug("no used patterns or query failed",
			zap.String("session_id", sessionID),
			zap.Error(err))
	} else if len(usedResults) > 0 {
		patterns = append(patterns, parsePatternsWithRelationship(usedResults)...)
	}

	// Get discovered patterns
	discoveredQuery := `
		MATCH (s:AgentSession {id: $session_id})-[:DISCOVERED_PATTERN]->(p:Pattern)
		RETURN p, 'discovered' as relationship_type
	`
	discoveredResults, err := asm.graphStore.Query(ctx, discoveredQuery, map[string]interface{}{
		"session_id": sessionID,
	})
	if err != nil {
		asm.logger.Debug("no discovered patterns or query failed",
			zap.String("session_id", sessionID),
			zap.Error(err))
	} else if len(discoveredResults) > 0 {
		patterns = append(patterns, parsePatternsWithRelationship(discoveredResults)...)
	}

	asm.logger.Debug("loaded patterns for session",
		zap.String("session_id", sessionID),
		zap.Int("pattern_count", len(patterns)))

	var currentHypothesis *Hypothesis
	for i := range hypotheses {
		if hypotheses[i].Status == "active" {
			currentHypothesis = &hypotheses[i]
			break
		}
	}

	return &SessionDetail{
		Session:           session,
		Hypotheses:        hypotheses,
		Queries:           queries,
		Findings:          findings,
		Recommendations:   recommendations,
		Patterns:          patterns,
		Investigations:    investigations,
		CurrentHypothesis: currentHypothesis,
	}, nil
}

// GetActiveSessions retrieves all active sessions and recently completed ones
func (asm *AgentSessionManager) GetActiveSessions(ctx context.Context) ([]ActiveSessionInfo, error) {
	retentionMinutes := asm.config.CompletedSessionRetentionMinutes

	query := `
		MATCH (s:AgentSession)
		WHERE s.status = 'active' 
		   OR datetime(s.completed_at) > datetime() - duration({minutes: $retention_minutes})
		RETURN s.id as id,
			   s.initial_symptom as symptom,
			   s.event_id as event_id,
			   s.event_source as event_source,
			   s.event_timestamp as event_timestamp,
			   s.status as status,
			   s.created_at as created_at,
			   s.completed_at as completed_at,
			   s.query_count as query_count,
			   s.finding_count as finding_count,
			   s.current_stage as current_stage
		ORDER BY 
			CASE WHEN s.status = 'active' THEN 0 ELSE 1 END,
			COALESCE(s.completed_at, s.created_at) DESC
	`

	results, err := asm.graphStore.Query(ctx, query, map[string]interface{}{
		"retention_minutes": retentionMinutes,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get active sessions: %w", err)
	}

	sessions := make([]ActiveSessionInfo, len(results))
	for i, result := range results {
		sessions[i] = parseActiveSessionInfo(result)
	}

	return sessions, nil
}

// GetTimeline retrieves the chronological timeline of session events
func (asm *AgentSessionManager) GetTimeline(ctx context.Context, sessionID string) ([]TimelineEvent, error) {
	// This is a complex query that unions different event types
	// For simplicity, we'll query each type and merge

	events := make([]TimelineEvent, 0)

	// Get hypotheses
	hypQuery := `
		MATCH (s:AgentSession {id: $session_id})-[:HAS_HYPOTHESIS]->(h:Hypothesis)
		RETURN 'hypothesis' as type, h.created_at as timestamp, h as data
	`
	hypResults, _ := asm.graphStore.Query(ctx, hypQuery, map[string]interface{}{
		"session_id": sessionID,
	})
	for _, result := range hypResults {
		events = append(events, parseTimelineEvent(result))
	}

	// Get queries
	queryQuery := `
		MATCH (s:AgentSession {id: $session_id})-[:EXECUTED_QUERY]->(q:QueryExecution)
		RETURN 'query' as type, q.executed_at as timestamp, q as data
	`
	queryResults, _ := asm.graphStore.Query(ctx, queryQuery, map[string]interface{}{
		"session_id": sessionID,
	})
	for _, result := range queryResults {
		events = append(events, parseTimelineEvent(result))
	}

	// Get findings
	findingQuery := `
		MATCH (s:AgentSession {id: $session_id})-[:HAS_FINDING]->(f:Finding)
		RETURN 'finding' as type, f.discovered_at as timestamp, f as data
	`
	findingResults, _ := asm.graphStore.Query(ctx, findingQuery, map[string]interface{}{
		"session_id": sessionID,
	})
	for _, result := range findingResults {
		events = append(events, parseTimelineEvent(result))
	}

	// Sort events chronologically by timestamp
	sort.Slice(events, func(i, j int) bool {
		return events[i].Timestamp.Before(events[j].Timestamp)
	})

	return events, nil
}

// Helper methods

func (asm *AgentSessionManager) storeQueryExecution(ctx context.Context, sessionID string, queryExec *QueryExecution) error {
	// Serialize results to JSON if present
	var resultsJSON string
	if len(queryExec.Results) > 0 {
		resultsBytes, _ := json.Marshal(queryExec.Results)
		resultsJSON = string(resultsBytes)
	}

	query := `
		MATCH (s:AgentSession {id: $session_id})
		CREATE (q:QueryExecution {
			id: $id,
			query: $query_text,
			reasoning: $reasoning,
			result_count: $result_count,
			results: $results,
			truncated: $truncated,
			duration_ms: $duration_ms,
			executed_at: datetime($executed_at),
			placeholder: false
		})
		CREATE (s)-[:EXECUTED_QUERY {sequence: s.query_count + 1}]->(q)
		RETURN q
	`

	_, err := asm.graphStore.Query(ctx, query, map[string]interface{}{
		"session_id":   sessionID,
		"id":           queryExec.ID,
		"query_text":   queryExec.Query,
		"reasoning":    queryExec.Reasoning,
		"result_count": queryExec.ResultCount,
		"results":      resultsJSON,
		"truncated":    queryExec.Truncated,
		"duration_ms":  queryExec.Duration.Milliseconds(),
		"executed_at":  queryExec.ExecutedAt.Format(time.RFC3339),
	})
	return err
}

func (asm *AgentSessionManager) storeFinding(ctx context.Context, sessionID, queryID string, finding *Finding) error {
	query := `
		MATCH (s:AgentSession {id: $session_id})
		CREATE (f:Finding {
			id: $id,
			type: $type,
			severity: $severity,
			resource_id: $resource_id,
			resource_type: $resource_type,
			description: $description,
			detection_method: $detection_method,
			discovered_at: datetime($discovered_at),
			placeholder: false
		})
		CREATE (s)-[:HAS_FINDING]->(f)
	`

	params := map[string]interface{}{
		"session_id":       sessionID,
		"id":               finding.ID,
		"type":             finding.Type,
		"severity":         finding.Severity,
		"resource_id":      finding.ResourceID,
		"resource_type":    finding.ResourceType,
		"description":      finding.Description,
		"detection_method": finding.DetectionMethod,
		"discovered_at":    finding.DiscoveredAt.Format(time.RFC3339),
	}

	if queryID != "" {
		query += `
		WITH s, f
		MATCH (q:QueryExecution {id: $query_id})
		CREATE (q)-[:DISCOVERED]->(f)
		`
		params["query_id"] = queryID
	}

	// Link finding to affected resource if it exists
	query += `
	WITH s, f
	OPTIONAL MATCH (r) WHERE r.id = $resource_id
	FOREACH (_ IN CASE WHEN r IS NOT NULL THEN [1] ELSE [] END |
		CREATE (f)-[:AFFECTS]->(r)
	)
	RETURN f
	`

	_, err := asm.graphStore.Query(ctx, query, params)
	return err
}

func (asm *AgentSessionManager) updateSessionQueryCount(ctx context.Context, sessionID string) {
	query := `
		MATCH (s:AgentSession {id: $session_id})
		SET s.query_count = s.query_count + 1
	`
	_, _ = asm.graphStore.Query(ctx, query, map[string]interface{}{
		"session_id": sessionID,
	})
}

func (asm *AgentSessionManager) updateSessionFindingCount(ctx context.Context, sessionID string) {
	query := `
		MATCH (s:AgentSession {id: $session_id})
		SET s.finding_count = s.finding_count + 1
	`
	_, _ = asm.graphStore.Query(ctx, query, map[string]interface{}{
		"session_id": sessionID,
	})
}

// RecordRecommendation stores a recommendation for a session
func (asm *AgentSessionManager) RecordRecommendation(
	ctx context.Context,
	sessionID string,
	recommendation *Recommendation,
) error {
	// Generate ID if not set
	if recommendation.ID == "" {
		recommendation.ID = generateRecommendationID()
	}

	if recommendation.CreatedAt.IsZero() {
		recommendation.CreatedAt = time.Now()
	}

	asm.logger.Info("recording recommendation",
		zap.String("session_id", sessionID),
		zap.String("recommendation_id", recommendation.ID),
		zap.String("type", recommendation.Type),
		zap.String("priority", recommendation.Priority))

	// Marshal arrays/objects to JSON
	actionItemsJSON, _ := json.Marshal(recommendation.ActionItems)
	tagsJSON, _ := json.Marshal(recommendation.Tags)
	metadataJSON, _ := json.Marshal(recommendation.Metadata)

	// Build query - handle empty related findings
	query := `
		MATCH (s:AgentSession {id: $session_id})
		CREATE (r:Recommendation {
			id: $id,
			type: $type,
			priority: $priority,
			title: $title,
			description: $description,
			rationale: $rationale,
			action_items: $action_items,
			estimated_effort: $estimated_effort,
			automation_hint: $automation_hint,
			tags: $tags,
			metadata: $metadata,
			created_at: datetime($created_at)
		})
		CREATE (s)-[:HAS_RECOMMENDATION]->(r)
	`

	params := map[string]interface{}{
		"session_id":       sessionID,
		"id":               recommendation.ID,
		"type":             recommendation.Type,
		"priority":         recommendation.Priority,
		"title":            recommendation.Title,
		"description":      recommendation.Description,
		"rationale":        recommendation.Rationale,
		"action_items":     string(actionItemsJSON),
		"estimated_effort": recommendation.EstimatedEffort,
		"automation_hint":  recommendation.AutomationHint,
		"tags":             string(tagsJSON),
		"metadata":         string(metadataJSON),
		"created_at":       recommendation.CreatedAt.Format(time.RFC3339),
	}

	// Add BASED_ON relationships if there are related findings
	if len(recommendation.RelatedFindings) > 0 {
		query += `
		WITH r
		UNWIND $related_findings AS finding_id
		MATCH (f:Finding {id: finding_id})
		CREATE (r)-[:BASED_ON]->(f)
		`
		params["related_findings"] = recommendation.RelatedFindings
	}

	query += `
		RETURN r
	`

	_, err := asm.graphStore.Query(ctx, query, params)
	if err != nil {
		return fmt.Errorf("failed to create recommendation: %w", err)
	}

	asm.logger.Info("recommendation recorded", zap.String("recommendation_id", recommendation.ID))
	return nil
}

// RecordPattern stores a diagnostic pattern discovered during investigation
func (asm *AgentSessionManager) RecordPattern(
	ctx context.Context,
	sessionID string,
	pattern *Pattern,
) (*Pattern, error) {
	// Generate ID if not set
	if pattern.ID == "" {
		pattern.ID = generatePatternID()
	}

	if pattern.CreatedAt.IsZero() {
		pattern.CreatedAt = time.Now()
	}

	// Set source as discovered
	pattern.Source = "discovered"
	pattern.UsageCount = 0

	asm.logger.Info("recording pattern",
		zap.String("session_id", sessionID),
		zap.String("pattern_id", pattern.ID),
		zap.String("name", pattern.Name),
		zap.String("match_key", pattern.RootCauseResourceType+"+"+pattern.RootCauseIssueType))

	// Check if session has used existing patterns
	usedPatternsQuery := `
		MATCH (s:AgentSession {id: $session_id})-[:USED_PATTERN]->(p:Pattern)
		RETURN p.id as id, p.name as name
	`
	usedResults, err := asm.graphStore.Query(ctx, usedPatternsQuery, map[string]interface{}{
		"session_id": sessionID,
	})

	if err == nil && len(usedResults) > 0 {
		asm.logger.Warn("session already used existing pattern(s) - consider if new pattern is necessary",
			zap.String("session_id", sessionID),
			zap.Int("used_pattern_count", len(usedResults)))
		// Don't block, just warn
	}

	// Check for existing pattern with same match key (strict)
	existingQuery := `
		MATCH (p:Pattern {
			root_cause_resource_type: $resource_type,
			root_cause_issue_type: $issue_type
		})
		RETURN p.id as id, p.name as name, p.usage_count as usage_count
		LIMIT 1
	`

	existingResults, err := asm.graphStore.Query(ctx, existingQuery, map[string]interface{}{
		"resource_type": pattern.RootCauseResourceType,
		"issue_type":    pattern.RootCauseIssueType,
	})

	if err == nil && len(existingResults) > 0 {
		asm.logger.Info("pattern already exists with same match key",
			zap.String("existing_id", existingResults[0]["id"].(string)),
			zap.String("existing_name", existingResults[0]["name"].(string)))
		// Still record but log that duplicate exists
	}

	// Discovered patterns are always Tier 2 (root cause patterns)
	// Tier 1 (triage) patterns are only created via bundled patterns
	if pattern.Tier == 0 {
		pattern.Tier = 2
	}

	// Marshal arrays to JSON
	stepsJSON, _ := json.Marshal(pattern.InvestigationSteps)
	recsJSON, _ := json.Marshal(pattern.Recommendations)
	keywordsJSON, _ := json.Marshal(pattern.SymptomKeywords)
	metadataJSON, _ := json.Marshal(pattern.Metadata)

	// Create pattern node and link to session
	query := `
		MATCH (s:AgentSession {id: $session_id})
		CREATE (p:Pattern {
			id: $id,
			tier: $tier,
			name: $name,
			description: $description,
			root_cause_resource_type: $resource_type,
			root_cause_issue_type: $issue_type,
			symptom_keywords: $keywords,
			investigation_steps: $steps,
			diagnosis_guidance: $diagnosis,
			recommendations: $recs,
			source: $source,
			usage_count: $usage_count,
			created_at: datetime($created_at),
			metadata: $metadata
		})
		CREATE (s)-[:DISCOVERED_PATTERN]->(p)
		RETURN p
	`

	params := map[string]interface{}{
		"session_id":    sessionID,
		"id":            pattern.ID,
		"tier":          pattern.Tier,
		"name":          pattern.Name,
		"description":   pattern.Description,
		"resource_type": pattern.RootCauseResourceType,
		"issue_type":    pattern.RootCauseIssueType,
		"keywords":      string(keywordsJSON),
		"steps":         string(stepsJSON),
		"diagnosis":     pattern.DiagnosisGuidance,
		"recs":          string(recsJSON),
		"source":        pattern.Source,
		"usage_count":   pattern.UsageCount,
		"created_at":    pattern.CreatedAt.Format(time.RFC3339),
		"metadata":      string(metadataJSON),
	}

	_, err = asm.graphStore.Query(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("failed to store pattern: %w", err)
	}

	asm.logger.Info("pattern recorded", zap.String("pattern_id", pattern.ID))
	return pattern, nil
}

// FindPatternsBySymptom searches for patterns matching symptom keywords
// Returns Tier 1 (triage) patterns first, then Tier 2 (root cause) patterns
func (asm *AgentSessionManager) FindPatternsBySymptom(ctx context.Context, symptom string) ([]Pattern, error) {
	// Query all patterns with symptom_keywords, ordering by tier first (Tier 1 before Tier 2)
	query := `
		MATCH (p:Pattern)
		WHERE p.symptom_keywords IS NOT NULL AND p.symptom_keywords <> '[]'
		RETURN p
		ORDER BY p.tier ASC, p.usage_count DESC, p.created_at DESC
	`

	results, err := asm.graphStore.Query(ctx, query, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to query patterns: %w", err)
	}

	patterns := parsePatterns(results)

	// Filter patterns by keyword matching in Go
	// Check if any keyword appears in the symptom text (case-insensitive)
	matchedPatterns := []Pattern{}
	symptomLower := strings.ToLower(symptom)

	for _, pattern := range patterns {
		for _, keyword := range pattern.SymptomKeywords {
			if strings.Contains(symptomLower, strings.ToLower(keyword)) {
				matchedPatterns = append(matchedPatterns, pattern)
				break // Only match once per pattern
			}
		}
	}

	// Count by tier for logging
	tier1Count := 0
	tier2Count := 0
	for _, p := range matchedPatterns {
		if p.Tier == 1 {
			tier1Count++
		} else {
			tier2Count++
		}
	}

	asm.logger.Info("found patterns by symptom",
		zap.String("symptom", symptom),
		zap.Int("matched_count", len(matchedPatterns)),
		zap.Int("tier1_count", tier1Count),
		zap.Int("tier2_count", tier2Count))

	return matchedPatterns, nil
}

// FindPatternsByType searches for patterns by exact resource type and issue type
func (asm *AgentSessionManager) FindPatternsByType(ctx context.Context, resourceType, issueType string) ([]Pattern, error) {
	query := `
		MATCH (p:Pattern {
			root_cause_resource_type: $resource_type,
			root_cause_issue_type: $issue_type
		})
		RETURN p
		ORDER BY p.usage_count DESC, p.created_at DESC
	`

	results, err := asm.graphStore.Query(ctx, query, map[string]interface{}{
		"resource_type": resourceType,
		"issue_type":    issueType,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query patterns by type: %w", err)
	}

	patterns := parsePatterns(results)

	asm.logger.Info("found patterns by type",
		zap.String("resource_type", resourceType),
		zap.String("issue_type", issueType),
		zap.Int("count", len(patterns)))

	return patterns, nil
}

// MarkPatternPresented creates a PRESENTED_PATTERN relationship
func (asm *AgentSessionManager) MarkPatternPresented(ctx context.Context, sessionID, patternID string) error {
	query := `
		MATCH (s:AgentSession {id: $session_id})
		MATCH (p:Pattern {id: $pattern_id})
		MERGE (s)-[r:PRESENTED_PATTERN {presented_at: datetime()}]->(p)
		RETURN r
	`

	_, err := asm.graphStore.Query(ctx, query, map[string]interface{}{
		"session_id": sessionID,
		"pattern_id": patternID,
	})

	if err != nil {
		return fmt.Errorf("failed to mark pattern as presented: %w", err)
	}

	asm.logger.Info("pattern marked as presented",
		zap.String("session_id", sessionID),
		zap.String("pattern_id", patternID))

	return nil
}

// MarkPatternUsed creates a USED_PATTERN relationship and increments usage count
func (asm *AgentSessionManager) MarkPatternUsed(ctx context.Context, sessionID, patternID, notes string) error {
	query := `
		MATCH (s:AgentSession {id: $session_id})
		MATCH (p:Pattern {id: $pattern_id})
		MERGE (s)-[r:USED_PATTERN]->(p)
		ON CREATE SET 
			r.used_at = datetime(),
			r.notes = $notes
		ON MATCH SET
			r.used_at = datetime(),
			r.notes = $notes
		WITH p
		SET p.usage_count = p.usage_count + 1
		RETURN p.usage_count as new_count
	`

	results, err := asm.graphStore.Query(ctx, query, map[string]interface{}{
		"session_id": sessionID,
		"pattern_id": patternID,
		"notes":      notes,
	})

	if err != nil {
		return fmt.Errorf("failed to mark pattern as used: %w", err)
	}

	var newCount int64
	if len(results) > 0 {
		if count, ok := results[0]["new_count"].(int64); ok {
			newCount = count
		}
	}

	asm.logger.Info("pattern marked as used",
		zap.String("session_id", sessionID),
		zap.String("pattern_id", patternID),
		zap.Int64("new_usage_count", newCount))

	return nil
}

// GetRecommendations retrieves all recommendations for a session
func (asm *AgentSessionManager) GetRecommendations(ctx context.Context, sessionID string) ([]Recommendation, error) {
	query := `
		MATCH (s:AgentSession {id: $session_id})-[:HAS_RECOMMENDATION]->(r:Recommendation)
		OPTIONAL MATCH (r)-[:BASED_ON]->(f:Finding)
		WITH r, collect(f.id) AS related_findings
		RETURN r, related_findings
		ORDER BY 
			CASE r.priority 
				WHEN 'critical' THEN 1
				WHEN 'high' THEN 2
				WHEN 'medium' THEN 3
				WHEN 'low' THEN 4
				ELSE 5
			END,
			r.created_at DESC
	`

	results, err := asm.graphStore.Query(ctx, query, map[string]interface{}{
		"session_id": sessionID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get recommendations: %w", err)
	}

	recommendations := make([]Recommendation, 0, len(results))
	for _, result := range results {
		rec := parseRecommendationFromResult(result)
		recommendations = append(recommendations, rec)
	}

	return recommendations, nil
}

// parseRecommendationFromResult helper function
func parseRecommendationFromResult(result map[string]interface{}) Recommendation {
	rMap := result["r"].(map[string]interface{})

	var actionItems []string
	if aiStr, ok := rMap["action_items"].(string); ok && aiStr != "" {
		json.Unmarshal([]byte(aiStr), &actionItems)
	}

	var tags []string
	if tagsStr, ok := rMap["tags"].(string); ok && tagsStr != "" {
		json.Unmarshal([]byte(tagsStr), &tags)
	}

	var metadata map[string]interface{}
	if metaStr, ok := rMap["metadata"].(string); ok && metaStr != "" {
		json.Unmarshal([]byte(metaStr), &metadata)
	}

	var relatedFindings []string
	if rf, ok := result["related_findings"].([]interface{}); ok {
		for _, f := range rf {
			if fStr, ok := f.(string); ok && fStr != "" {
				relatedFindings = append(relatedFindings, fStr)
			}
		}
	}

	// Parse created_at timestamp - handle both string and time.Time
	var createdAt time.Time
	if createdAtStr, ok := rMap["created_at"].(string); ok {
		createdAt, _ = time.Parse(time.RFC3339, createdAtStr)
	} else if createdAtTime, ok := rMap["created_at"].(time.Time); ok {
		createdAt = createdAtTime
	}

	return Recommendation{
		ID:              rMap["id"].(string),
		Type:            rMap["type"].(string),
		Priority:        rMap["priority"].(string),
		Title:           rMap["title"].(string),
		Description:     rMap["description"].(string),
		Rationale:       rMap["rationale"].(string),
		RelatedFindings: relatedFindings,
		ActionItems:     actionItems,
		EstimatedEffort: getStringOrEmpty(rMap, "estimated_effort"),
		AutomationHint:  getStringOrEmpty(rMap, "automation_hint"),
		Tags:            tags,
		Metadata:        metadata,
		CreatedAt:       createdAt,
	}
}

// getStringOrEmpty helper function for optional string fields
func getStringOrEmpty(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// Parsing helpers

func parseAgentSession(data interface{}) *AgentSession {
	props, ok := data.(map[string]interface{})
	if !ok {
		return &AgentSession{}
	}

	session := &AgentSession{}

	if id, ok := props["id"].(string); ok {
		session.ID = id
	}
	if symptom, ok := props["initial_symptom"].(string); ok {
		session.InitialSymptom = symptom
	}
	if resource, ok := props["initial_resource"].(string); ok {
		session.InitialResource = resource
	}
	if eventID, ok := props["event_id"].(string); ok {
		session.EventID = eventID
	}
	if eventSource, ok := props["event_source"].(string); ok {
		session.EventSource = eventSource
	}
	if status, ok := props["status"].(string); ok {
		session.Status = status
	}
	if stage, ok := props["current_stage"].(int64); ok {
		session.CurrentStage = int(stage)
	}
	if queryCount, ok := props["query_count"].(int64); ok {
		session.QueryCount = int(queryCount)
	}
	if findingCount, ok := props["finding_count"].(int64); ok {
		session.FindingCount = int(findingCount)
	}
	if summary, ok := props["summary"].(string); ok {
		session.Summary = summary
	}

	// Parse created_at
	if createdAt, ok := props["created_at"].(string); ok {
		if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
			session.CreatedAt = t
		}
	} else if createdAt, ok := props["created_at"].(time.Time); ok {
		session.CreatedAt = createdAt
	}

	// Parse event_timestamp (optional)
	if eventTimestamp, ok := props["event_timestamp"].(string); ok {
		if t, err := time.Parse(time.RFC3339, eventTimestamp); err == nil {
			session.EventTimestamp = &t
			// Calculate processing delay
			delay := session.CreatedAt.Sub(t)
			session.ProcessingDelay = &delay
		}
	} else if eventTimestamp, ok := props["event_timestamp"].(time.Time); ok {
		session.EventTimestamp = &eventTimestamp
		// Calculate processing delay
		delay := session.CreatedAt.Sub(eventTimestamp)
		session.ProcessingDelay = &delay
	}

	// Parse completed_at (optional)
	if completedAt, ok := props["completed_at"].(string); ok {
		if t, err := time.Parse(time.RFC3339, completedAt); err == nil {
			session.CompletedAt = &t
		}
	} else if completedAt, ok := props["completed_at"].(time.Time); ok {
		session.CompletedAt = &completedAt
	}

	return session
}

func parseHypotheses(results []map[string]interface{}) []Hypothesis {
	hypotheses := make([]Hypothesis, 0, len(results))

	for _, result := range results {
		var props map[string]interface{}

		// Handle both direct map and nested "h" key
		if h, ok := result["h"].(map[string]interface{}); ok {
			props = h
		} else {
			props = result
		}

		hypothesis := Hypothesis{}

		if id, ok := props["id"].(string); ok {
			hypothesis.ID = id
		}
		if stage, ok := props["stage"].(int64); ok {
			hypothesis.Stage = int(stage)
		}
		if text, ok := props["text"].(string); ok {
			hypothesis.Text = text
		}
		if status, ok := props["status"].(string); ok {
			hypothesis.Status = status
		}

		// Parse created_at
		if createdAt, ok := props["created_at"].(string); ok {
			if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
				hypothesis.CreatedAt = t
			}
		} else if createdAt, ok := props["created_at"].(time.Time); ok {
			hypothesis.CreatedAt = createdAt
		}

		hypotheses = append(hypotheses, hypothesis)
	}

	return hypotheses
}

func parseQueryExecutions(results []map[string]interface{}) []QueryExecution {
	queries := make([]QueryExecution, 0, len(results))

	for _, result := range results {
		var props map[string]interface{}

		// Handle both direct map and nested "q" key
		if q, ok := result["q"].(map[string]interface{}); ok {
			props = q
		} else {
			props = result
		}

		query := QueryExecution{
			Findings: make([]string, 0), // Initialize to empty slice, never nil
		}

		if id, ok := props["id"].(string); ok {
			query.ID = id
		}
		if queryText, ok := props["query"].(string); ok {
			query.Query = queryText
		}
		if reasoning, ok := props["reasoning"].(string); ok {
			query.Reasoning = reasoning
		}
		if resultCount, ok := props["result_count"].(int64); ok {
			query.ResultCount = int(resultCount)
		}

		// Parse duration
		if durationMs, ok := props["duration_ms"].(int64); ok {
			query.Duration = time.Duration(durationMs) * time.Millisecond
		}

		// Parse executed_at
		if executedAt, ok := props["executed_at"].(string); ok {
			if t, err := time.Parse(time.RFC3339, executedAt); err == nil {
				query.ExecutedAt = t
			}
		} else if executedAt, ok := props["executed_at"].(time.Time); ok {
			query.ExecutedAt = executedAt
		}

		// Parse findings (array of finding IDs)
		if findings, ok := props["findings"].([]interface{}); ok {
			query.Findings = make([]string, 0, len(findings))
			for _, f := range findings {
				if fID, ok := f.(string); ok {
					query.Findings = append(query.Findings, fID)
				}
			}
		}

		// Parse results JSON if present
		if resultsJSON, ok := props["results"].(string); ok && resultsJSON != "" {
			var results []map[string]interface{}
			if err := json.Unmarshal([]byte(resultsJSON), &results); err == nil {
				query.Results = results
			}
		}

		// Parse truncated flag
		if truncated, ok := props["truncated"].(bool); ok {
			query.Truncated = truncated
		}

		queries = append(queries, query)
	}

	return queries
}

func parseFindings(results []map[string]interface{}) []Finding {
	findings := make([]Finding, 0, len(results))

	for _, result := range results {
		var props map[string]interface{}

		// Handle both direct map and nested "f" key
		if f, ok := result["f"].(map[string]interface{}); ok {
			props = f
		} else {
			props = result
		}

		finding := Finding{}

		if id, ok := props["id"].(string); ok {
			finding.ID = id
		}
		if findingType, ok := props["type"].(string); ok {
			finding.Type = findingType
		}
		if severity, ok := props["severity"].(string); ok {
			finding.Severity = severity
		}
		if resourceID, ok := props["resource_id"].(string); ok {
			finding.ResourceID = resourceID
		}
		if resourceType, ok := props["resource_type"].(string); ok {
			finding.ResourceType = resourceType
		}
		if description, ok := props["description"].(string); ok {
			finding.Description = description
		}
		if detectionMethod, ok := props["detection_method"].(string); ok {
			finding.DetectionMethod = detectionMethod
		}

		// Parse evidence (optional map)
		if evidence, ok := props["evidence"].(map[string]interface{}); ok {
			finding.Evidence = evidence
		}

		// Parse discovered_at
		if discoveredAt, ok := props["discovered_at"].(string); ok {
			if t, err := time.Parse(time.RFC3339, discoveredAt); err == nil {
				finding.DiscoveredAt = t
			}
		} else if discoveredAt, ok := props["discovered_at"].(time.Time); ok {
			finding.DiscoveredAt = discoveredAt
		}

		findings = append(findings, finding)
	}

	return findings
}

// parsePatternFromProps extracts a Pattern from a map of properties
func parsePatternFromProps(props map[string]interface{}) Pattern {
	pattern := Pattern{}

	// Common fields
	if id, ok := props["id"].(string); ok {
		pattern.ID = id
	}
	if tier, ok := props["tier"].(int64); ok {
		pattern.Tier = int(tier)
	}
	if name, ok := props["name"].(string); ok {
		pattern.Name = name
	}
	if description, ok := props["description"].(string); ok {
		pattern.Description = description
	}
	if resourceType, ok := props["root_cause_resource_type"].(string); ok {
		pattern.RootCauseResourceType = resourceType
	}
	if issueType, ok := props["root_cause_issue_type"].(string); ok {
		pattern.RootCauseIssueType = issueType
	}
	if source, ok := props["source"].(string); ok {
		pattern.Source = source
	}
	if usageCount, ok := props["usage_count"].(int64); ok {
		pattern.UsageCount = int(usageCount)
	}
	if bundleID, ok := props["bundle_id"].(string); ok {
		pattern.BundleID = bundleID
	}
	if guidance, ok := props["diagnosis_guidance"].(string); ok {
		pattern.DiagnosisGuidance = guidance
	}

	// Parse JSON arrays - common
	if keywordsJSON, ok := props["symptom_keywords"].(string); ok && keywordsJSON != "" {
		json.Unmarshal([]byte(keywordsJSON), &pattern.SymptomKeywords)
	}
	if metadataJSON, ok := props["metadata"].(string); ok && metadataJSON != "" {
		json.Unmarshal([]byte(metadataJSON), &pattern.Metadata)
	}

	// Parse JSON arrays - Tier 1 specific
	if discriminatingQueriesJSON, ok := props["discriminating_queries"].(string); ok && discriminatingQueriesJSON != "" {
		json.Unmarshal([]byte(discriminatingQueriesJSON), &pattern.DiscriminatingQueries)
	}
	if decisionLogicJSON, ok := props["decision_logic"].(string); ok && decisionLogicJSON != "" {
		json.Unmarshal([]byte(decisionLogicJSON), &pattern.DecisionLogic)
	}
	if initialStepsJSON, ok := props["initial_investigation_steps"].(string); ok && initialStepsJSON != "" {
		json.Unmarshal([]byte(initialStepsJSON), &pattern.InitialInvestigationSteps)
	}

	// Parse JSON arrays - Tier 2 specific
	if stepsJSON, ok := props["investigation_steps"].(string); ok && stepsJSON != "" {
		json.Unmarshal([]byte(stepsJSON), &pattern.InvestigationSteps)
	}
	if recsJSON, ok := props["recommendations"].(string); ok && recsJSON != "" {
		json.Unmarshal([]byte(recsJSON), &pattern.Recommendations)
	}

	// Parse timestamps
	if createdAt, ok := props["created_at"].(string); ok {
		if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
			pattern.CreatedAt = t
		}
	} else if createdAt, ok := props["created_at"].(time.Time); ok {
		pattern.CreatedAt = createdAt
	}

	return pattern
}

func parsePatterns(results []map[string]interface{}) []Pattern {
	patterns := make([]Pattern, 0, len(results))

	for _, result := range results {
		var props map[string]interface{}

		if p, ok := result["p"].(map[string]interface{}); ok {
			props = p
		} else {
			props = result
		}

		pattern := parsePatternFromProps(props)
		patterns = append(patterns, pattern)
	}

	return patterns
}

func parsePatternsWithRelationship(results []map[string]interface{}) []Pattern {
	// Use a map to deduplicate patterns (a pattern might have multiple relationships)
	patternMap := make(map[string]Pattern)

	for _, result := range results {
		var props map[string]interface{}

		if p, ok := result["p"].(map[string]interface{}); ok {
			props = p
		} else {
			continue
		}

		pattern := parsePatternFromProps(props)

		// Set relationship type directly from query result
		if relationshipType, ok := result["relationship_type"].(string); ok {
			pattern.RelationshipType = relationshipType
		}

		// Store pattern in map (will keep the highest priority relationship type if duplicates)
		// Priority: discovered > used > presented
		if existingPattern, exists := patternMap[pattern.ID]; exists {
			// Keep the higher priority relationship
			if shouldReplaceRelationship(existingPattern.RelationshipType, pattern.RelationshipType) {
				patternMap[pattern.ID] = pattern
			}
		} else {
			patternMap[pattern.ID] = pattern
		}
	}

	// Convert map to slice
	patterns := make([]Pattern, 0, len(patternMap))
	for _, pattern := range patternMap {
		patterns = append(patterns, pattern)
	}

	return patterns
}

// shouldReplaceRelationship determines if a new relationship type should replace the existing one
// Priority: discovered > used > presented
func shouldReplaceRelationship(existingType, newType string) bool {
	priority := map[string]int{
		"discovered": 3,
		"used":       2,
		"presented":  1,
		"":           0,
	}

	return priority[newType] > priority[existingType]
}

func parseActiveSessionInfo(result map[string]interface{}) ActiveSessionInfo {
	info := ActiveSessionInfo{}

	if id, ok := result["id"].(string); ok {
		info.ID = id
	}
	if symptom, ok := result["symptom"].(string); ok {
		info.InitialSymptom = symptom
	}
	if eventID, ok := result["event_id"].(string); ok {
		info.EventID = eventID
	}
	if eventSource, ok := result["event_source"].(string); ok {
		info.EventSource = eventSource
	}
	if status, ok := result["status"].(string); ok {
		info.Status = status
	}
	if queryCount, ok := result["query_count"].(int64); ok {
		info.QueryCount = int(queryCount)
	}
	if findingCount, ok := result["finding_count"].(int64); ok {
		info.FindingCount = int(findingCount)
	}
	if currentStage, ok := result["current_stage"].(int64); ok {
		info.CurrentStage = int(currentStage)
	}

	// Parse created_at timestamp
	if createdAt, ok := result["created_at"].(string); ok {
		if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
			info.CreatedAt = t
		}
	} else if createdAt, ok := result["created_at"].(time.Time); ok {
		info.CreatedAt = createdAt
	}

	// Parse event_timestamp (optional)
	if eventTimestamp, ok := result["event_timestamp"].(string); ok {
		if t, err := time.Parse(time.RFC3339, eventTimestamp); err == nil {
			info.EventTimestamp = &t
			// Calculate processing delay
			delay := info.CreatedAt.Sub(t)
			info.ProcessingDelay = &delay
		}
	} else if eventTimestamp, ok := result["event_timestamp"].(time.Time); ok {
		info.EventTimestamp = &eventTimestamp
		// Calculate processing delay
		delay := info.CreatedAt.Sub(eventTimestamp)
		info.ProcessingDelay = &delay
	}

	// Parse completed_at timestamp (optional)
	if completedAt, ok := result["completed_at"].(string); ok {
		if t, err := time.Parse(time.RFC3339, completedAt); err == nil {
			info.CompletedAt = &t
		}
	} else if completedAt, ok := result["completed_at"].(time.Time); ok {
		info.CompletedAt = &completedAt
	}

	return info
}

func parseTimelineEvent(result map[string]interface{}) TimelineEvent {
	event := TimelineEvent{
		Data: make(map[string]interface{}), // Always initialize to empty map, never nil
	}

	// Get event type
	if eventType, ok := result["type"].(string); ok {
		event.Type = eventType
	}

	// Parse timestamp
	if timestamp, ok := result["timestamp"].(string); ok {
		if t, err := time.Parse(time.RFC3339, timestamp); err == nil {
			event.Timestamp = t
		}
	} else if timestamp, ok := result["timestamp"].(time.Time); ok {
		event.Timestamp = timestamp
	}

	// Parse data (the actual event content)
	if data, ok := result["data"].(map[string]interface{}); ok {
		event.Data = data
	}
	// If data parsing fails, Data remains as the empty map initialized above

	return event
}

// Helper functions for hypothesis analysis

func shouldSpawnInvestigation(hypothesisText string) bool {
	// Simple keyword matching - would be more sophisticated in production
	keywords := []string{"memory", "cpu", "oom", "resource", "metrics", "performance"}
	for _, keyword := range keywords {
		for i := 0; i <= len(hypothesisText)-len(keyword); i++ {
			if hypothesisText[i:i+len(keyword)] == keyword {
				return true
			}
		}
	}
	return false
}

func extractInvestigationParams(hypothesisText string) (resourceType, resourceID, symptom string) {
	// Simplified extraction - would use NLP or regex in production
	// For now, return empty to disable automatic investigation spawning
	return "", "", ""
}
