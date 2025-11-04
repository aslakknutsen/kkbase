package observability

import (
	"context"
	"fmt"
	"time"

	"github.com/kagenti/kkbase/pkg/graph"
	"github.com/kagenti/kkbase/pkg/models"
	"go.uber.org/zap"
)

// AgentSessionManager manages all agent investigation session operations
type AgentSessionManager struct {
	graphStore       graph.GraphStore
	findingExtractor *FindingExtractor
	blastZoneCalc    *BlastZoneCalculator
	invProcessor     *InvestigationMetricsProcessor // Link to existing metrics system
	logger           *zap.Logger
}

// NewAgentSessionManager creates a new agent session manager
func NewAgentSessionManager(
	graphStore graph.GraphStore,
	invProcessor *InvestigationMetricsProcessor,
	logger *zap.Logger,
) *AgentSessionManager {
	return &AgentSessionManager{
		graphStore:       graphStore,
		findingExtractor: NewFindingExtractor(),
		blastZoneCalc:    NewBlastZoneCalculator(graphStore, logger),
		invProcessor:     invProcessor,
		logger:           logger,
	}
}

// CreateSession creates a new agent investigation session
func (asm *AgentSessionManager) CreateSession(ctx context.Context, symptom, initialResource string) (*AgentSession, error) {
	session := &AgentSession{
		ID:              generateSessionID(),
		InitialSymptom:  symptom,
		InitialResource: initialResource,
		Status:          "active",
		CreatedAt:       time.Now(),
		CurrentStage:    0,
		QueryCount:      0,
		FindingCount:    0,
	}

	asm.logger.Info("creating agent session",
		zap.String("session_id", session.ID),
		zap.String("symptom", symptom))

	// Create session node in Neo4j
	query := `
		CREATE (s:AgentSession {
			id: $id,
			initial_symptom: $symptom,
			initial_resource: $initial_resource,
			status: $status,
			created_at: datetime($created_at),
			current_stage: $stage,
			query_count: 0,
			finding_count: 0,
			placeholder: false
		})
		RETURN s
	`

	_, err := asm.graphStore.Query(ctx, query, map[string]interface{}{
		"id":               session.ID,
		"symptom":          symptom,
		"initial_resource": initialResource,
		"status":           "active",
		"created_at":       session.CreatedAt.Format(time.RFC3339),
		"stage":            0,
	})
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
func (asm *AgentSessionManager) CompleteSession(ctx context.Context, sessionID, summary string) (*SessionSummary, error) {
	completedAt := time.Now()

	// Update session status
	updateQuery := `
		MATCH (s:AgentSession {id: $session_id})
		SET s.status = 'completed',
			s.completed_at = datetime($completed_at),
			s.summary = $summary
		RETURN s.created_at as created_at, s.query_count as query_count, s.finding_count as finding_count
	`

	results, err := asm.graphStore.Query(ctx, updateQuery, map[string]interface{}{
		"session_id":   sessionID,
		"completed_at": completedAt.Format(time.RFC3339),
		"summary":      summary,
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
		Investigations:    investigations,
		CurrentHypothesis: currentHypothesis,
	}, nil
}

// GetActiveSessions retrieves all active sessions
func (asm *AgentSessionManager) GetActiveSessions(ctx context.Context) ([]ActiveSessionInfo, error) {
	query := `
		MATCH (s:AgentSession {status: 'active'})
		RETURN s.id as id,
			   s.initial_symptom as symptom,
			   s.created_at as created_at,
			   s.query_count as query_count,
			   s.finding_count as finding_count,
			   s.current_stage as current_stage
		ORDER BY s.created_at DESC
	`

	results, err := asm.graphStore.Query(ctx, query, nil)
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

	// Sort by timestamp (would need proper time parsing and sorting)
	// For now return unsorted - frontend can sort

	return events, nil
}

// Helper methods

func (asm *AgentSessionManager) storeQueryExecution(ctx context.Context, sessionID string, queryExec *QueryExecution) error {
	query := `
		MATCH (s:AgentSession {id: $session_id})
		CREATE (q:QueryExecution {
			id: $id,
			query: $query_text,
			reasoning: $reasoning,
			result_count: $result_count,
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

		query := QueryExecution{}

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

func parseActiveSessionInfo(result map[string]interface{}) ActiveSessionInfo {
	info := ActiveSessionInfo{}

	if id, ok := result["id"].(string); ok {
		info.ID = id
	}
	if symptom, ok := result["symptom"].(string); ok {
		info.InitialSymptom = symptom
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

	return info
}

func parseTimelineEvent(result map[string]interface{}) TimelineEvent {
	event := TimelineEvent{
		Data: make(map[string]interface{}),
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
