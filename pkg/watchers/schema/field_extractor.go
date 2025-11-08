package schema

import (
	"fmt"
	"strings"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// FieldRequirement defines a field needed from a CRD
// It serves BOTH as validation criteria AND extraction logic
type FieldRequirement struct {
	Name        string   // Logical name (e.g., "targetRef")
	Description string   // What it's used for
	Required    bool     // Must exist for handler to function
	Paths       []string // JSONPath alternatives to try (e.g., "spec.targetRef", "spec.target")
}

// FieldExtractor validates CRD schema and provides field extraction
type FieldExtractor struct {
	requirements []FieldRequirement
	fieldPaths   map[string][]string // field name -> validated path (from CRD schema)
	version      string
}

// NewFieldExtractor creates an extractor with validated field paths from CRD schema
func NewFieldExtractor(
	crdSchema *apiextensionsv1.JSONSchemaProps,
	version string,
	requirements []FieldRequirement,
) (*FieldExtractor, error) {
	extractor := &FieldExtractor{
		requirements: requirements,
		fieldPaths:   make(map[string][]string),
		version:      version,
	}

	// Validate and discover field paths
	var missingRequired []string
	var missingOptional []string

	for _, req := range requirements {
		found := false

		// Try each alternative path in the CRD schema
		for _, pathStr := range req.Paths {
			path := splitPath(pathStr)
			if fieldExistsInSchema(crdSchema, path) {
				extractor.fieldPaths[req.Name] = path
				found = true
				break
			}
		}

		if !found {
			if req.Required {
				missingRequired = append(missingRequired, req.Name)
			} else {
				missingOptional = append(missingOptional, req.Name)
			}
		}
	}

	// Check if we can function
	if len(missingRequired) > 0 {
		return nil, fmt.Errorf(
			"version %s missing required fields: %v (optional missing: %v)",
			version, missingRequired, missingOptional,
		)
	}

	return extractor, nil
}

// Extract gets a field value from an object using validated paths
func (e *FieldExtractor) Extract(obj *unstructured.Unstructured, fieldName string) (interface{}, bool, error) {
	path, exists := e.fieldPaths[fieldName]
	if !exists {
		return nil, false, fmt.Errorf("field %s not available in version %s", fieldName, e.version)
	}

	return unstructured.NestedFieldNoCopy(obj.Object, path...)
}

// ExtractString is a typed convenience for string fields
func (e *FieldExtractor) ExtractString(obj *unstructured.Unstructured, fieldName string) (string, bool, error) {
	path, exists := e.fieldPaths[fieldName]
	if !exists {
		return "", false, fmt.Errorf("field %s not available in version %s", fieldName, e.version)
	}

	return unstructured.NestedString(obj.Object, path...)
}

// ExtractMap is a typed convenience for map fields
func (e *FieldExtractor) ExtractMap(obj *unstructured.Unstructured, fieldName string) (map[string]interface{}, bool, error) {
	path, exists := e.fieldPaths[fieldName]
	if !exists {
		return nil, false, fmt.Errorf("field %s not available in version %s", fieldName, e.version)
	}

	return unstructured.NestedMap(obj.Object, path...)
}

// ExtractSlice is a typed convenience for slice fields
func (e *FieldExtractor) ExtractSlice(obj *unstructured.Unstructured, fieldName string) ([]interface{}, bool, error) {
	path, exists := e.fieldPaths[fieldName]
	if !exists {
		return nil, false, fmt.Errorf("field %s not available in version %s", fieldName, e.version)
	}

	return unstructured.NestedSlice(obj.Object, path...)
}

// ExtractInt extracts an int64 field from an unstructured object
func (e *FieldExtractor) ExtractInt(obj *unstructured.Unstructured, fieldName string) (int64, bool, error) {
	path, exists := e.fieldPaths[fieldName]
	if !exists {
		return 0, false, fmt.Errorf("field %s not available in version %s", fieldName, e.version)
	}

	return unstructured.NestedInt64(obj.Object, path...)
}

// HasField checks if a field is available (validated during construction)
func (e *FieldExtractor) HasField(fieldName string) bool {
	_, exists := e.fieldPaths[fieldName]
	return exists
}

// GetVersion returns the CRD version this extractor was built for
func (e *FieldExtractor) GetVersion() string {
	return e.version
}

// Helper functions

func splitPath(pathStr string) []string {
	if pathStr == "" {
		return []string{}
	}
	return strings.Split(pathStr, ".")
}

func fieldExistsInSchema(schema *apiextensionsv1.JSONSchemaProps, path []string) bool {
	if len(path) == 0 {
		return true
	}

	current := schema
	for _, segment := range path {
		if current.Properties == nil {
			return false
		}

		prop, exists := current.Properties[segment]
		if !exists {
			return false
		}

		current = &prop
	}

	return true
}
