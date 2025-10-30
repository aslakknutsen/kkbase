package mcp

import (
	"fmt"
	"regexp"
	"strings"
)

// writeOperationPatterns defines regex patterns for detecting write operations in Cypher queries
var writeOperationPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bCREATE\b`),
	regexp.MustCompile(`(?i)\bDELETE\b`),
	regexp.MustCompile(`(?i)\bSET\b`),
	regexp.MustCompile(`(?i)\bMERGE\b`),
	regexp.MustCompile(`(?i)\bREMOVE\b`),
	regexp.MustCompile(`(?i)\bDROP\b`),
	regexp.MustCompile(`(?i)\bDETACH\s+DELETE\b`),
}

// ValidateReadOnlyQuery checks if a Cypher query contains write operations
func ValidateReadOnlyQuery(query string) error {
	// Trim whitespace and normalize
	query = strings.TrimSpace(query)

	if query == "" {
		return fmt.Errorf("query cannot be empty")
	}

	// Check for write operations
	for _, pattern := range writeOperationPatterns {
		if pattern.MatchString(query) {
			operation := pattern.FindString(query)
			return fmt.Errorf("write operation detected: %s (only read-only queries are allowed)", strings.ToUpper(operation))
		}
	}

	return nil
}
