package kuadrant

import (
	"encoding/json"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// serializeMap converts a map to JSON string for Neo4j storage
func serializeMap(m map[string]string) string {
	if len(m) == 0 {
		return ""
	}
	b, _ := json.Marshal(m)
	return string(b)
}

// storeCompleteResourceSpec stores the complete spec and status as JSON properties.
// This enables downstream systems to access the full resource configuration,
// not just the fields we explicitly extract for graph relationships.
//
// Use case: Diagnostic tools can inspect the complete resource specification
// even if we only extracted summary fields (e.g., authentication_count).
//
// Storage format:
//   - spec_json: Complete spec as JSON string
//   - status_json: Complete status as JSON string (if present)
func storeCompleteResourceSpec(obj *unstructured.Unstructured, properties map[string]interface{}) {
	// Store complete spec for diagnostic tools
	if spec, found, _ := unstructured.NestedMap(obj.Object, "spec"); found {
		if specJSON, err := json.Marshal(spec); err == nil {
			properties["spec_json"] = string(specJSON)
		}
	}

	// Store complete status for diagnostic tools (health, conditions, etc.)
	if status, found, _ := unstructured.NestedMap(obj.Object, "status"); found {
		if statusJSON, err := json.Marshal(status); err == nil {
			properties["status_json"] = string(statusJSON)
		}
	}
}
