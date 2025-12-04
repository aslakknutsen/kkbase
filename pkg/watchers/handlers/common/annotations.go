package common

import (
	"encoding/json"
)

// annotationDenylist contains annotations that should be filtered out before storing.
// These annotations contain stale or redundant data that could confuse diagnostic agents.
var annotationDenylist = map[string]struct{}{
	// Contains the full resource spec from the last kubectl apply, which may not
	// match the current state if the resource was modified by other means.
	"kubectl.kubernetes.io/last-applied-configuration": {},
}

// FilterAnnotations removes problematic annotations from the map.
// Returns a new map with only the annotations that should be stored.
func FilterAnnotations(annotations map[string]string) map[string]string {
	if len(annotations) == 0 {
		return nil
	}

	filtered := make(map[string]string)
	for key, value := range annotations {
		if _, denied := annotationDenylist[key]; !denied {
			filtered[key] = value
		}
	}

	if len(filtered) == 0 {
		return nil
	}
	return filtered
}

// SerializeAnnotations filters and serializes annotations to JSON for Neo4j storage.
// Returns empty string if no annotations remain after filtering.
func SerializeAnnotations(annotations map[string]string) string {
	filtered := FilterAnnotations(annotations)
	if len(filtered) == 0 {
		return ""
	}
	b, err := json.Marshal(filtered)
	if err != nil {
		return ""
	}
	return string(b)
}

