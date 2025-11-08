# Kuadrant Handler Architecture: Version-Agnostic Design

## Problem Solved

Kuadrant (and other CRD-based extensions) can have multiple versions with API changes. Hard-coding to specific types causes dependency conflicts and brittle code.

## Solution: Dynamic Field Extraction with CRD Schema Validation

### Key Components

#### 1. `FieldRequirement` - Single Source of Truth
```go
// requirements.go
var AuthPolicyRequirements = []schema.FieldRequirement{
    {
        Name:     "targetRef.kind",
        Required: true,
        Paths:    []string{"spec.targetRef.kind", "spec.target.kind"}, // Version alternatives
    },
}
```

**Serves dual purpose:**
- **Validation**: Checked against CRD schema at registration time
- **Extraction**: Used to extract fields from objects at runtime

#### 2. `FieldExtractor` - Version-Aware Runtime Helper
```go
// Created once per CRD version detected
extractor, err := schema.NewFieldExtractor(crdInfo.Schema, crdInfo.Version, AuthPolicyRequirements)

// Used throughout handler lifetime
targetKind, found, err := extractor.ExtractString(policy, "targetRef.kind")
```

**Benefits:**
- Zero runtime path lookup overhead (validated once at registration)
- Compile-time safety for field names (string constants)
- Clear error messages when fields don't exist

#### 3. RegisterHandlerFactory with CRDInfo
```go
// register.go - Version-agnostic registration
manager.RegisterHandlerFactory(
    watchers.ResourceTypeInfo{
        NodeType:      NodeTypeAuthPolicy,
        Kind:          "AuthPolicy",
        APIGroup:      "kuadrant.io",
        ClusterScoped: false,
    },
    func(crdInfo *watchers.CRDInfo) watchers.ResourceWatcher {
        // crdInfo contains: Name, Group, Version, Kind, Schema
        extractor, err := schema.NewFieldExtractor(
            crdInfo.Schema,       // OpenAPI schema from CRD
            crdInfo.Version,      // Detected storage version
            AuthPolicyRequirements,
        )
        
        if err != nil {
            // Missing required fields - don't register handler
            logger.Error("unsupported version", zap.Error(err))
            return nil  // Handler not created
        }
        
        // Create handler with validated extractor
        return NewAuthPolicyHandler(clientset, dynamicClient, graphStore, logger, factory, extractor)
    },
)
```

**Key Benefits:**
- `RegisterHandlerFactory` internally handles CRD watching via `CRDWatcher`
- Factory function receives `CRDInfo` directly when CRD becomes available
- Returning `nil` from the factory prevents handler registration for unsupported versions
- No manual `WatchCRD()` calls needed - cleaner API

### Information Flow

```
Startup
  ↓
CRDWatcher detects AuthPolicy CRD (v1)
  ↓
Extract OpenAPI schema for v1
  ↓
Pass to handler registration callback:
  - crdInfo.Schema (OpenAPI structure)
  - crdInfo.Version ("v1")
  ↓
Validate requirements against schema:
  - spec.targetRef.kind exists? ✓
  - spec.targetRef.name exists? ✓
  - spec.authScheme exists? ✓
  ↓
Create FieldExtractor with validated paths:
  - "targetRef.kind" → ["spec", "targetRef", "kind"]
  - "targetRef.name" → ["spec", "targetRef", "name"]
  ↓
Pass extractor to handler constructor
  ↓
Handler uses extractor for ALL field access
  - extractor.ExtractString(obj, "targetRef.kind")
  - O(1) lookup using pre-validated path
```

### Example: Version Evolution

**Scenario**: Kuadrant v1beta3 renames `spec.targetRef` → `spec.target`

**No code changes needed!**

```go
// requirements.go (already has fallback)
var AuthPolicyRequirements = []schema.FieldRequirement{
    {
        Name:     "targetRef",
        Required: true,
        Paths:    []string{
            "spec.targetRef",  // v1, v1beta2
            "spec.target",     // v1beta3 - will be found automatically
        },
    },
}
```

**What happens:**
1. CRDWatcher detects v1beta3
2. FieldExtractor tries `spec.targetRef` in schema → not found
3. Tries `spec.target` in schema → found ✓
4. Handler registered with path ["spec", "target"]
5. Runtime extraction works seamlessly

### Startup Log Example

```
INFO CRDWatcher detected CRD name=authpolicies.kuadrant.io version=v1
INFO validating AuthPolicy requirements version=v1
INFO ✓ targetRef.kind found at spec.targetRef.kind
INFO ✓ targetRef.name found at spec.targetRef.name  
INFO ✓ authScheme found at spec.authScheme
INFO registering AuthPolicy handler version=v1 has_authScheme=true
INFO handler registered successfully kind=AuthPolicy version=v1
```

### If Version is Incompatible

```
INFO CRDWatcher detected CRD name=authpolicies.kuadrant.io version=v2alpha1
INFO validating AuthPolicy requirements version=v2alpha1
ERROR ✗ targetRef.kind not found at any known path
ERROR ✗ targetRef.name not found at any known path
ERROR AuthPolicy v2alpha1 missing required fields missing=["targetRef.kind", "targetRef.name"]
WARN AuthPolicy handler NOT registered - version unsupported
```

### Key Advantages

1. **No Dependency Conflicts**: Zero imports of kuadrant-operator types
2. **Validation at Registration**: Fail fast if version unsupported
3. **Zero Duplication**: Requirements serve both validation AND extraction
4. **Performance**: Field paths resolved once, not per-object
5. **Graceful Degradation**: Optional fields can be missing without failure
6. **Clear Diagnostics**: Know exactly what's supported at startup
7. **Future-Proof**: Add version alternatives without code changes
8. **Agent-Friendly**: Core graph structure (IDs, relationships) always available

### Migration Path

To add support for new fields/versions:

1. **Add path alternative** to existing requirement
2. **Or add new optional requirement**
3. **Test** with the new CRD version
4. **No handler code changes needed** (uses logical field names)

### Code Structure

```
pkg/watchers/
├── schema/
│   └── field_extractor.go      # FieldExtractor + FieldRequirement
├── crd_info.go                  # CRDInfo with Schema
└── crd_watcher.go               # Detects CRDs, extracts schemas

pkg/watchers/handlers/extensions/kuadrant/
├── requirements.go              # ALL field requirements (single source of truth)
├── register.go                  # Version-agnostic registration via CRDWatcher
├── authpolicy_dynamic.go        # Uses FieldExtractor
├── ratelimitpolicy_dynamic.go   # Uses FieldExtractor
└── ...                          # Other handlers
```

### Testing Strategy

```go
// Test with real CRD schemas from multiple versions
func TestAuthPolicyExtraction_V1(t *testing.T) {
    schema := loadCRDSchema(t, "testdata/authpolicy-v1.yaml")
    extractor, err := schema.NewFieldExtractor(schema, "v1", AuthPolicyRequirements)
    require.NoError(t, err)
    
    obj := loadTestObject(t, "testdata/authpolicy-v1-example.yaml")
    kind, found, err := extractor.ExtractString(obj, "targetRef.kind")
    require.NoError(t, err)
    require.True(t, found)
    assert.Equal(t, "HTTPRoute", kind)
}

func TestAuthPolicyExtraction_V1Beta3(t *testing.T) {
    schema := loadCRDSchema(t, "testdata/authpolicy-v1beta3.yaml")
    extractor, err := schema.NewFieldExtractor(schema, "v1beta3", AuthPolicyRequirements)
    require.NoError(t, err) // Should work with renamed field
    
    obj := loadTestObject(t, "testdata/authpolicy-v1beta3-example.yaml")
    kind, found, err := extractor.ExtractString(obj, "targetRef.kind")
    require.NoError(t, err)
    require.True(t, found)
    assert.Equal(t, "HTTPRoute", kind)
}
```

## Summary

This architecture:
- ✅ Eliminates dependency version conflicts
- ✅ Validates compatibility at registration time (fail fast)
- ✅ Provides runtime type safety via FieldExtractor
- ✅ Handles API evolution gracefully
- ✅ Zero code duplication between validation and extraction
- ✅ Clear diagnostics for unsupported versions
- ✅ Integrates cleanly with existing CRDWatcher

The Agent doesn't care if v1 uses `spec.authScheme.authentication` while v2 uses `spec.auth.rules` — both produce nodes with similar core properties.

### Indexed Properties

The handlers extract and index key properties for fast querying while storing complete specs for deep diagnostics:

**All Policies:**
- **Basic**: name, namespace, version, labels
- **Target**: target_kind, target_name, target_group (optional)
- **Status**: status_accepted, status_enforced (or status_ready for TLS), status_failed, status_message, observed_generation, status_stale
- **Complete**: spec_json, status_json (for deep diagnostics)

**AuthPolicy & RateLimitPolicy:**
- **Precedence**: policy_type (defaults/overrides/implicit_defaults)
- **Configuration hints**: authentication_configured, limits_count

**DNSPolicy:**
- **Configuration**: has_load_balancing
- **Relationships**: USES_SECRET → provider credential secrets

**TLSPolicy:**
- **Configuration**: has_issuer_ref
- **Status**: status_ready (instead of status_enforced)

## Additional Features

### 1. Build-Time Validation for Known Versions

While we support runtime validation and unknown versions, we also want to catch regressions early for known versions:

```go
// authpolicy_test.go
func TestAuthPolicyFieldRequirements_KnownVersions(t *testing.T) {
    // Test against actual CRD schemas from known releases
    tests := []struct{
        name          string
        version       string
        crdYAML       string
        shouldSupport bool
    }{
        {
            name:          "v1 - full support",
            version:       "v1",
            crdYAML:       authPolicyV1CRD,
            shouldSupport: true,
        },
    }
    
    for _, tt := range tests {
        // Parse CRD and extract schema
        crd := &apiextensionsv1.CustomResourceDefinition{}
        decode([]byte(tt.crdYAML), crd)
        
        // Validate our FieldRequirements against this version
        extractor, err := schema.NewFieldExtractor(
            crd.Schema, 
            tt.version, 
            AuthPolicyFieldRequirements,
        )
        
        if tt.shouldSupport {
            require.NoError(t, err)
        } else {
            require.Error(t, err)
        }
    }
}
```

**Benefits:**
- Catch API changes in known versions during CI/CD
- Document which versions are fully supported
- Prevent accidental breakage of known-good configurations
- Validate field paths match actual CRD schemas

**Implementation:**
- Store CRD YAML fixtures from actual releases in `testdata/`
- Run tests in CI against multiple versions
- Fail build if known versions break

### 2. Full Spec Storage for AI Reasoning

While we extract specific fields for graph relationships, AI agents benefit from having the **complete resource configuration**:

```go
// helpers.go
func storeCompleteResourceSpec(obj *unstructured.Unstructured, properties map[string]interface{}) {
    // Store complete spec as JSON
    if spec, found, _ := unstructured.NestedMap(obj.Object, "spec"); found {
        if specJSON, err := json.Marshal(spec); err == nil {
            properties["spec_json"] = string(specJSON)
        }
    }
    
    // Store complete status as JSON
    if status, found, _ := unstructured.NestedMap(obj.Object, "status"); found {
        if statusJSON, err := json.Marshal(status); err == nil {
            properties["status_json"] = string(statusJSON)
        }
    }
}

// Usage in handler
policyNode.Properties["name"] = policy.GetName()
policyNode.Properties["target_kind"] = targetKind  // Extracted for indexing
storeCompleteResourceSpec(policy, policyNode.Properties)    // Full spec for diagnostics
```

**Use Case Example:**
```cypher
// AI agent diagnosing auth failures
MATCH (ap:AuthPolicy)-[:APPLIES_TO]->(route:HTTPRoute)
WHERE route.name = 'api-gateway'
RETURN ap.name, 
       ap.authentication_count,  // Extracted summary for quick filtering
       ap.spec_json              // Full config for detailed analysis
```

The AI can:
1. Use extracted fields (`authentication_count`, `target_kind`) for quick filtering and graph traversal
2. Access `spec_json` for detailed configuration when diagnosing issues
3. Inspect `status_json` for health/condition information
4. Reason about fields we didn't anticipate extracting

**Storage Pattern:**
- **Extracted fields**: Indexed, used for graph traversal and filtering (e.g., `target_kind`, `target_name`)
- **spec_json**: Full context for AI reasoning, not indexed
- **status_json**: Runtime state for diagnostics

**Trade-offs:**
- ✅ AI has complete context for any field, even those we didn't extract
- ✅ No need to anticipate all useful fields upfront
- ✅ Version-agnostic (works with any field structure)
- ✅ Enables deep diagnostics without schema knowledge
- ⚠️ Slightly larger Neo4j storage (typically 1-5KB per resource)
- ⚠️ AI must parse JSON (minimal overhead for LLMs)

**Example AI Query:**
```
You: Why is authentication failing for the api-gateway route?

Agent queries:
1. Find AuthPolicy for api-gateway
2. Read ap.spec_json to examine authScheme details
3. Discover that the JWT issuer URL is unreachable
4. Check ap.status_json for condition messages
5. Provide specific diagnosis with policy configuration details
```

## Future Improvements

1. **Automated CRD Discovery**: Could auto-generate `FieldRequirement`s from CRD schemas
2. **Migration Guidance**: When a field is missing, suggest which alternative fields exist
3. **Field Deprecation Warnings**: Detect when using deprecated field paths
4. **Performance Optimization**: Cache schema lookups across handler instances
5. **Selective Full Spec Storage**: Make `storeCompleteResourceSpec()` configurable per resource type
6. **Compressed Storage**: Consider gzip compression for large spec_json fields

