package kuadrant

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kagenti/kkbase/pkg/watchers/schema"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"
)

// TestDNSPolicyFieldRequirements_KnownVersions validates that our field requirements
// work against known CRD schema versions at build time
//
// CRD fixtures are from official Kuadrant releases:
// - dnspolicy-v1-crd.yaml: Kuadrant v1.3.0
func TestDNSPolicyFieldRequirements_KnownVersions(t *testing.T) {
	// Register CRD types
	_ = apiextensionsv1.AddToScheme(scheme.Scheme)

	tests := []struct {
		name            string
		version         string
		crdFile         string
		shouldSupport   bool
		expectedMissing []string
	}{
		{
			name:          "v1 - full support",
			version:       "v1",
			crdFile:       "dnspolicy-v1-crd.yaml",
			shouldSupport: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Load CRD from testdata
			crdPath := filepath.Join("testdata", tt.crdFile)
			crdYAML, err := os.ReadFile(crdPath)
			if err != nil {
				t.Fatalf("failed to read CRD file %s: %v", crdPath, err)
			}

			// Parse CRD
			crd := &apiextensionsv1.CustomResourceDefinition{}
			decode := serializer.NewCodecFactory(scheme.Scheme).UniversalDeserializer().Decode
			_, _, err = decode(crdYAML, nil, crd)
			if err != nil {
				t.Fatalf("failed to parse CRD YAML: %v", err)
			}

			// Find the version in the CRD
			var versionSchema *apiextensionsv1.JSONSchemaProps
			for _, version := range crd.Spec.Versions {
				if version.Name == tt.version {
					versionSchema = version.Schema.OpenAPIV3Schema
					break
				}
			}

			if versionSchema == nil {
				t.Fatalf("version %s not found in CRD", tt.version)
			}

			// Create field extractor and validate
			extractor, err := schema.NewFieldExtractor(versionSchema, tt.version, DNSPolicyFieldRequirements)

			if tt.shouldSupport {
				if err != nil {
					t.Errorf("expected version %s to be supported, but got error: %v", tt.version, err)
				}
			} else {
				if err == nil {
					t.Errorf("expected version %s to NOT be supported, but validation passed", tt.version)
				}
				// Optionally check for specific missing fields
				if len(tt.expectedMissing) > 0 {
					for _, missing := range tt.expectedMissing {
						if !contains(err.Error(), missing) {
							t.Errorf("expected missing field '%s' in error, got: %v", missing, err)
						}
					}
				}
			}

			// Additional validation: check that HasField works correctly
			if tt.shouldSupport && extractor != nil {
				// Required fields must exist
				requiredFields := []string{"targetRef.kind", "targetRef.name"}
				for _, field := range requiredFields {
					if !extractor.HasField(field) {
						t.Errorf("required field '%s' not found in version %s", field, tt.version)
					}
				}
			}
		})
	}
}

