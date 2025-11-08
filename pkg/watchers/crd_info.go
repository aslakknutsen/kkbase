package watchers

import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

// CRDInfo represents information about a CRD including its schema
type CRDInfo struct {
	Name    string
	Group   string
	Version string // Storage version
	Kind    string
	Schema  *apiextensionsv1.JSONSchemaProps // OpenAPI schema for the storage version
}

