package observability

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aslakknutsen/kkbase/pkg/graph"
	"go.uber.org/zap"
)

//go:embed patterns/*.json
var patternsFS embed.FS

// PatternSeedFile represents the structure of the initial-patterns.json file
type PatternSeedFile struct {
	SchemaVersion string        `json:"schema_version"`
	Description   string        `json:"description"`
	Patterns      []PatternSeed `json:"patterns"`
}

// PatternSeed represents a pattern to be seeded (supports both Tier 1 and Tier 2)
type PatternSeed struct {
	// Common fields
	Tier            int      `json:"tier"` // 1 = triage, 2 = root cause
	Name            string   `json:"name"`
	Description     string   `json:"description,omitempty"`
	SymptomKeywords []string `json:"symptom_keywords"`

	// Tier 1 specific fields (triage patterns)
	DiscriminatingQueries     []DiscriminatingQuerySeed `json:"discriminating_queries,omitempty"`
	DecisionLogic             map[string]string         `json:"decision_logic,omitempty"`
	InitialInvestigationSteps []string                  `json:"initial_investigation_steps,omitempty"`

	// Tier 2 specific fields (root cause patterns)
	RootCauseResourceType string   `json:"root_cause_resource_type,omitempty"`
	RootCauseIssueType    string   `json:"root_cause_issue_type,omitempty"`
	InvestigationSteps    []string `json:"investigation_steps,omitempty"`
	DiagnosisGuidance     string   `json:"diagnosis_guidance,omitempty"`
	Recommendations       []string `json:"recommendations,omitempty"`
}

// DiscriminatingQuerySeed represents a query used in Tier 1 patterns to narrow down the root cause
type DiscriminatingQuerySeed struct {
	Name            string `json:"name"`
	Query           string `json:"query"`
	Condition       string `json:"condition"`
	SuggestsPattern string `json:"suggests_pattern"`
}

// SeedInitialPatterns loads and seeds/updates initial patterns from embedded file
func SeedInitialPatterns(ctx context.Context, graphStore graph.GraphStore, logger *zap.Logger) error {
	logger.Info("loading initial patterns from embedded file")

	// Load patterns from embedded JSON file
	data, err := patternsFS.ReadFile("patterns/initial-patterns.json")
	if err != nil {
		return fmt.Errorf("failed to read embedded patterns file: %w", err)
	}

	var seedFile PatternSeedFile
	if err := json.Unmarshal(data, &seedFile); err != nil {
		return fmt.Errorf("failed to parse patterns JSON: %w", err)
	}

	logger.Info("loaded patterns from embedded file",
		zap.String("schema_version", seedFile.SchemaVersion),
		zap.Int("pattern_count", len(seedFile.Patterns)))

	createdCount := 0
	updatedCount := 0
	tier1Count := 0
	tier2Count := 0

	// Create or update patterns in graph
	for i, seed := range seedFile.Patterns {
		var existingQuery string
		var existingParams map[string]interface{}
		var matchKey string

		// Different matching logic for Tier 1 vs Tier 2 patterns
		if seed.Tier == 1 {
			// Tier 1: Match by tier + name
			existingQuery = `
				MATCH (p:Pattern {
					tier: $tier,
					name: $name,
					source: 'bundled'
				})
				RETURN p.id as id, p.name as name, p.usage_count as usage_count
				LIMIT 1
			`
			existingParams = map[string]interface{}{
				"tier": seed.Tier,
				"name": seed.Name,
			}
			matchKey = fmt.Sprintf("tier%d:%s", seed.Tier, seed.Name)
			tier1Count++
		} else {
			// Tier 2: Match by resource_type + issue_type (existing behavior)
			existingQuery = `
				MATCH (p:Pattern {
					root_cause_resource_type: $resource_type,
					root_cause_issue_type: $issue_type,
					source: 'bundled'
				})
				RETURN p.id as id, p.name as name, p.usage_count as usage_count
				LIMIT 1
			`
			existingParams = map[string]interface{}{
				"resource_type": seed.RootCauseResourceType,
				"issue_type":    seed.RootCauseIssueType,
			}
			matchKey = seed.RootCauseResourceType + "+" + seed.RootCauseIssueType
			tier2Count++
		}

		existingResults, err := graphStore.Query(ctx, existingQuery, existingParams)

		if err != nil {
			logger.Error("failed to check for existing pattern",
				zap.Int("index", i),
				zap.String("name", seed.Name),
				zap.Int("tier", seed.Tier),
				zap.Error(err))
			continue
		}

		if len(existingResults) > 0 {
			// Update existing bundled pattern
			existingID := existingResults[0]["id"].(string)
			existingUsageCount := int64(0)
			if uc, ok := existingResults[0]["usage_count"].(int64); ok {
				existingUsageCount = uc
			}

			if err := updatePatternFromSeed(ctx, graphStore, existingID, &seed, int(existingUsageCount)); err != nil {
				logger.Error("failed to update pattern",
					zap.Int("index", i),
					zap.String("name", seed.Name),
					zap.String("pattern_id", existingID),
					zap.Error(err))
				continue
			}

			logger.Debug("updated bundled pattern",
				zap.String("pattern_id", existingID),
				zap.String("name", seed.Name),
				zap.Int("tier", seed.Tier),
				zap.String("match_key", matchKey))
			updatedCount++
		} else {
			// Create new pattern from seed
			if err := createPatternFromSeed(ctx, graphStore, &seed); err != nil {
				logger.Error("failed to create pattern",
					zap.Int("index", i),
					zap.String("name", seed.Name),
					zap.Int("tier", seed.Tier),
					zap.Error(err))
				continue
			}

			logger.Debug("created bundled pattern",
				zap.String("name", seed.Name),
				zap.Int("tier", seed.Tier),
				zap.String("match_key", matchKey))
			createdCount++
		}
	}

	logger.Info("successfully synced initial patterns",
		zap.Int("created", createdCount),
		zap.Int("updated", updatedCount),
		zap.Int("tier1_count", tier1Count),
		zap.Int("tier2_count", tier2Count),
		zap.Int("total", len(seedFile.Patterns)))

	return nil
}

// createPatternFromSeed creates a pattern node in the graph from a seed
func createPatternFromSeed(ctx context.Context, graphStore graph.GraphStore, seed *PatternSeed) error {
	// Marshal arrays to JSON
	keywordsJSON, _ := json.Marshal(seed.SymptomKeywords)
	stepsJSON, _ := json.Marshal(seed.InvestigationSteps)
	recsJSON, _ := json.Marshal(seed.Recommendations)
	discriminatingQueriesJSON, _ := json.Marshal(seed.DiscriminatingQueries)
	decisionLogicJSON, _ := json.Marshal(seed.DecisionLogic)
	initialStepsJSON, _ := json.Marshal(seed.InitialInvestigationSteps)

	query := `
		CREATE (p:Pattern {
			id: $id,
			tier: $tier,
			name: $name,
			description: $description,
			root_cause_resource_type: $resource_type,
			root_cause_issue_type: $issue_type,
			symptom_keywords: $keywords,
			discriminating_queries: $discriminating_queries,
			decision_logic: $decision_logic,
			initial_investigation_steps: $initial_steps,
			investigation_steps: $steps,
			diagnosis_guidance: $diagnosis,
			recommendations: $recs,
			source: $source,
			usage_count: $usage_count,
			created_at: datetime($created_at)
		})
		RETURN p
	`

	params := map[string]interface{}{
		"id":                     generatePatternID(),
		"tier":                   seed.Tier,
		"name":                   seed.Name,
		"description":            seed.Description,
		"resource_type":          seed.RootCauseResourceType,
		"issue_type":             seed.RootCauseIssueType,
		"keywords":               string(keywordsJSON),
		"discriminating_queries": string(discriminatingQueriesJSON),
		"decision_logic":         string(decisionLogicJSON),
		"initial_steps":          string(initialStepsJSON),
		"steps":                  string(stepsJSON),
		"diagnosis":              seed.DiagnosisGuidance,
		"recs":                   string(recsJSON),
		"source":                 "bundled",
		"usage_count":            0,
		"created_at":             time.Now().Format(time.RFC3339),
	}

	_, err := graphStore.Query(ctx, query, params)
	if err != nil {
		return fmt.Errorf("failed to create pattern: %w", err)
	}

	return nil
}

// updatePatternFromSeed updates an existing bundled pattern while preserving usage_count
func updatePatternFromSeed(ctx context.Context, graphStore graph.GraphStore, patternID string, seed *PatternSeed, usageCount int) error {
	// Marshal arrays to JSON
	keywordsJSON, _ := json.Marshal(seed.SymptomKeywords)
	stepsJSON, _ := json.Marshal(seed.InvestigationSteps)
	recsJSON, _ := json.Marshal(seed.Recommendations)
	discriminatingQueriesJSON, _ := json.Marshal(seed.DiscriminatingQueries)
	decisionLogicJSON, _ := json.Marshal(seed.DecisionLogic)
	initialStepsJSON, _ := json.Marshal(seed.InitialInvestigationSteps)

	query := `
		MATCH (p:Pattern {id: $id})
		SET p.tier = $tier,
			p.name = $name,
			p.description = $description,
			p.symptom_keywords = $keywords,
			p.discriminating_queries = $discriminating_queries,
			p.decision_logic = $decision_logic,
			p.initial_investigation_steps = $initial_steps,
			p.investigation_steps = $steps,
			p.diagnosis_guidance = $diagnosis,
			p.recommendations = $recs,
			p.updated_at = datetime()
		RETURN p
	`

	params := map[string]interface{}{
		"id":                     patternID,
		"tier":                   seed.Tier,
		"name":                   seed.Name,
		"description":            seed.Description,
		"keywords":               string(keywordsJSON),
		"discriminating_queries": string(discriminatingQueriesJSON),
		"decision_logic":         string(decisionLogicJSON),
		"initial_steps":          string(initialStepsJSON),
		"steps":                  string(stepsJSON),
		"diagnosis":              seed.DiagnosisGuidance,
		"recs":                   string(recsJSON),
	}

	_, err := graphStore.Query(ctx, query, params)
	if err != nil {
		return fmt.Errorf("failed to update pattern: %w", err)
	}

	return nil
}

// createPattern creates a pattern node in the graph (for runtime-discovered patterns)
func createPattern(ctx context.Context, graphStore graph.GraphStore, pattern *Pattern) error {
	// Marshal arrays to JSON
	keywordsJSON, _ := json.Marshal(pattern.SymptomKeywords)
	stepsJSON, _ := json.Marshal(pattern.InvestigationSteps)
	recsJSON, _ := json.Marshal(pattern.Recommendations)
	metadataJSON, _ := json.Marshal(pattern.Metadata)
	discriminatingQueriesJSON, _ := json.Marshal(pattern.DiscriminatingQueries)
	decisionLogicJSON, _ := json.Marshal(pattern.DecisionLogic)
	initialStepsJSON, _ := json.Marshal(pattern.InitialInvestigationSteps)

	query := `
		CREATE (p:Pattern {
			id: $id,
			tier: $tier,
			name: $name,
			description: $description,
			root_cause_resource_type: $resource_type,
			root_cause_issue_type: $issue_type,
			symptom_keywords: $keywords,
			discriminating_queries: $discriminating_queries,
			decision_logic: $decision_logic,
			initial_investigation_steps: $initial_steps,
			investigation_steps: $steps,
			diagnosis_guidance: $diagnosis,
			recommendations: $recs,
			source: $source,
			usage_count: $usage_count,
			created_at: datetime($created_at),
			metadata: $metadata
		})
		RETURN p
	`

	// Default to Tier 2 for discovered patterns
	tier := pattern.Tier
	if tier == 0 {
		tier = 2
	}

	params := map[string]interface{}{
		"id":                     pattern.ID,
		"tier":                   tier,
		"name":                   pattern.Name,
		"description":            pattern.Description,
		"resource_type":          pattern.RootCauseResourceType,
		"issue_type":             pattern.RootCauseIssueType,
		"keywords":               string(keywordsJSON),
		"discriminating_queries": string(discriminatingQueriesJSON),
		"decision_logic":         string(decisionLogicJSON),
		"initial_steps":          string(initialStepsJSON),
		"steps":                  string(stepsJSON),
		"diagnosis":              pattern.DiagnosisGuidance,
		"recs":                   string(recsJSON),
		"source":                 pattern.Source,
		"usage_count":            pattern.UsageCount,
		"created_at":             pattern.CreatedAt.Format(time.RFC3339),
		"metadata":               string(metadataJSON),
	}

	_, err := graphStore.Query(ctx, query, params)
	if err != nil {
		return fmt.Errorf("failed to create pattern: %w", err)
	}

	return nil
}
