package agent

import "github.com/aslakknutsen/kkbase/pkg/agenttypes"

// FilterFunc is a function that filters events
type FilterFunc func(agenttypes.Event) bool

// AllowlistFilter creates a filter that only allows events with reasons in the allowlist
func AllowlistFilter(allowlist []string) FilterFunc {
	if len(allowlist) == 0 {
		// Empty allowlist means allow all
		return func(e agenttypes.Event) bool { return true }
	}

	// Build map for O(1) lookup
	allowed := make(map[string]bool, len(allowlist))
	for _, reason := range allowlist {
		allowed[reason] = true
	}

	return func(e agenttypes.Event) bool {
		return allowed[e.Reason]
	}
}

// DenylistFilter creates a filter that blocks events with reasons in the denylist
func DenylistFilter(denylist []string) FilterFunc {
	if len(denylist) == 0 {
		// Empty denylist means allow all
		return func(e agenttypes.Event) bool { return true }
	}

	// Build map for O(1) lookup
	denied := make(map[string]bool, len(denylist))
	for _, reason := range denylist {
		denied[reason] = true
	}

	return func(e agenttypes.Event) bool {
		return !denied[e.Reason]
	}
}

// SeverityFilter creates a filter that only allows events at or above the minimum severity
func SeverityFilter(minSeverity agenttypes.Severity) FilterFunc {
	severityRank := map[agenttypes.Severity]int{
		agenttypes.SeverityInfo:     1,
		agenttypes.SeverityWarning:  2,
		agenttypes.SeverityCritical: 3,
	}

	minRank := severityRank[minSeverity]

	return func(e agenttypes.Event) bool {
		return severityRank[e.Severity] >= minRank
	}
}

// ResourceTypeFilter creates a filter that only allows specific resource types
func ResourceTypeFilter(allowedTypes []string) FilterFunc {
	if len(allowedTypes) == 0 {
		return func(e agenttypes.Event) bool { return true }
	}

	allowed := make(map[string]bool, len(allowedTypes))
	for _, t := range allowedTypes {
		allowed[t] = true
	}

	return func(e agenttypes.Event) bool {
		return allowed[e.Resource.Type]
	}
}
