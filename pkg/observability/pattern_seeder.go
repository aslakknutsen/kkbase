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
	Patterns []PatternSeed `json:"patterns"`
}

// PatternSeed represents a pattern to be seeded
type PatternSeed struct {
	Name                  string   `json:"name"`
	RootCauseResourceType string   `json:"root_cause_resource_type"`
	RootCauseIssueType    string   `json:"root_cause_issue_type"`
	SymptomKeywords       []string `json:"symptom_keywords"`
	InvestigationSteps    []string `json:"investigation_steps"`
	DiagnosisGuidance     string   `json:"diagnosis_guidance"`
	Recommendations       []string `json:"recommendations"`
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
		zap.Int("pattern_count", len(seedFile.Patterns)))

	createdCount := 0
	updatedCount := 0

	// Create or update patterns in graph
	for i, seed := range seedFile.Patterns {
		// Check if pattern with same match key exists
		existingQuery := `
			MATCH (p:Pattern {
				root_cause_resource_type: $resource_type,
				root_cause_issue_type: $issue_type,
				source: 'bundled'
			})
			RETURN p.id as id, p.name as name, p.usage_count as usage_count
			LIMIT 1
		`

		existingResults, err := graphStore.Query(ctx, existingQuery, map[string]interface{}{
			"resource_type": seed.RootCauseResourceType,
			"issue_type":    seed.RootCauseIssueType,
		})

		if err != nil {
			logger.Error("failed to check for existing pattern",
				zap.Int("index", i),
				zap.String("name", seed.Name),
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

			if err := updatePattern(ctx, graphStore, existingID, &seed, int(existingUsageCount)); err != nil {
				logger.Error("failed to update pattern",
					zap.Int("index", i),
					zap.String("name", seed.Name),
					zap.String("pattern_id", existingID),
					zap.Error(err))
				continue
			}

			logger.Info("updated bundled pattern",
				zap.String("pattern_id", existingID),
				zap.String("name", seed.Name),
				zap.String("match_key", seed.RootCauseResourceType+"+"+seed.RootCauseIssueType))
			updatedCount++
		} else {
			// Create new pattern
			pattern := &Pattern{
				ID:                    generatePatternID(),
				Name:                  seed.Name,
				RootCauseResourceType: seed.RootCauseResourceType,
				RootCauseIssueType:    seed.RootCauseIssueType,
				SymptomKeywords:       seed.SymptomKeywords,
				InvestigationSteps:    seed.InvestigationSteps,
				DiagnosisGuidance:     seed.DiagnosisGuidance,
				Recommendations:       seed.Recommendations,
				Source:                "bundled",
				UsageCount:            0,
				CreatedAt:             time.Now(),
			}

			if err := createPattern(ctx, graphStore, pattern); err != nil {
				logger.Error("failed to create pattern",
					zap.Int("index", i),
					zap.String("name", seed.Name),
					zap.Error(err))
				continue
			}

			logger.Info("created bundled pattern",
				zap.String("pattern_id", pattern.ID),
				zap.String("name", pattern.Name),
				zap.String("match_key", pattern.RootCauseResourceType+"+"+pattern.RootCauseIssueType))
			createdCount++
		}
	}

	logger.Info("successfully synced initial patterns",
		zap.Int("created", createdCount),
		zap.Int("updated", updatedCount),
		zap.Int("total", len(seedFile.Patterns)))

	return nil
}

// createPattern creates a pattern node in the graph
func createPattern(ctx context.Context, graphStore graph.GraphStore, pattern *Pattern) error {
	// Marshal arrays to JSON
	keywordsJSON, _ := json.Marshal(pattern.SymptomKeywords)
	stepsJSON, _ := json.Marshal(pattern.InvestigationSteps)
	recsJSON, _ := json.Marshal(pattern.Recommendations)
	metadataJSON, _ := json.Marshal(pattern.Metadata)

	query := `
		CREATE (p:Pattern {
			id: $id,
			name: $name,
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
		RETURN p
	`

	params := map[string]interface{}{
		"id":            pattern.ID,
		"name":          pattern.Name,
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

	_, err := graphStore.Query(ctx, query, params)
	if err != nil {
		return fmt.Errorf("failed to create pattern: %w", err)
	}

	return nil
}

// updatePattern updates an existing bundled pattern while preserving usage_count
func updatePattern(ctx context.Context, graphStore graph.GraphStore, patternID string, seed *PatternSeed, usageCount int) error {
	// Marshal arrays to JSON
	keywordsJSON, _ := json.Marshal(seed.SymptomKeywords)
	stepsJSON, _ := json.Marshal(seed.InvestigationSteps)
	recsJSON, _ := json.Marshal(seed.Recommendations)

	query := `
		MATCH (p:Pattern {id: $id})
		SET p.name = $name,
			p.symptom_keywords = $keywords,
			p.investigation_steps = $steps,
			p.diagnosis_guidance = $diagnosis,
			p.recommendations = $recs,
			p.updated_at = datetime()
		RETURN p
	`

	params := map[string]interface{}{
		"id":        patternID,
		"name":      seed.Name,
		"keywords":  string(keywordsJSON),
		"steps":     string(stepsJSON),
		"diagnosis": seed.DiagnosisGuidance,
		"recs":      string(recsJSON),
	}

	_, err := graphStore.Query(ctx, query, params)
	if err != nil {
		return fmt.Errorf("failed to update pattern: %w", err)
	}

	return nil
}
