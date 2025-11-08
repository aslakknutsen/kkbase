# Kuadrant CRD Test Fixtures

This directory contains CRD (CustomResourceDefinition) YAML files used for build-time validation testing.

## Purpose

These fixtures enable testing our `FieldRequirement` definitions against known CRD schema versions to:
- Catch regressions when updating handlers
- Document which versions are fully supported
- Validate field paths match actual schemas
- Prevent accidental breakage during refactoring

## File Naming Convention

- `{resource}-{version}-crd.yaml`: Complete CRD definition for a specific version
- Example: `authpolicy-v1-crd.yaml` contains the v1 CRD for AuthPolicy

## Adding New Fixtures

To test a new version:

1. Extract the CRD from the Kuadrant release:
   ```bash
   # From kuadrant-operator repository
   kubectl kustomize config/crd > authpolicy-v1beta2-crd.yaml
   ```

2. Add the file to this directory

3. Add a test case in the corresponding `*_test.go` file:
   ```go
   {
       name:          "v1beta2 - full support",
       version:       "v1beta2",
       crdFile:       "authpolicy-v1beta2-crd.yaml",
       shouldSupport: true,
   },
   ```

## CRD Sources

These CRDs are from official Kuadrant releases:

| File | Version | Source |
|------|---------|--------|
| `authpolicy-v1-crd.yaml` | v1 (from v1.3.0) | [kuadrant-operator v1.3.0](https://github.com/Kuadrant/kuadrant-operator/blob/v1.3.0/config/crd/bases/kuadrant.io_authpolicies.yaml) |
| `ratelimitpolicy-v1-crd.yaml` | v1 (from v1.3.0) | [kuadrant-operator v1.3.0](https://github.com/Kuadrant/kuadrant-operator/blob/v1.3.0/config/crd/bases/kuadrant.io_ratelimitpolicies.yaml) |
| `dnspolicy-v1-crd.yaml` | v1 (from v1.3.0) | [kuadrant-operator v1.3.0](https://github.com/Kuadrant/kuadrant-operator/blob/v1.3.0/config/crd/bases/kuadrant.io_dnspolicies.yaml) |
| `tlspolicy-v1-crd.yaml` | v1 (from v1.3.0) | [kuadrant-operator v1.3.0](https://github.com/Kuadrant/kuadrant-operator/blob/v1.3.0/config/crd/bases/kuadrant.io_tlspolicies.yaml) |
| `kuadrant-v1-crd.yaml` | v1 (from v1.3.0) | [kuadrant-operator v1.3.0](https://github.com/Kuadrant/kuadrant-operator/blob/v1.3.0/config/crd/bases/kuadrant.io_kuadrants.yaml) |

**Download Commands:**
```bash
# AuthPolicy v1 from Kuadrant v1.3.0
curl -sSL "https://raw.githubusercontent.com/Kuadrant/kuadrant-operator/v1.3.0/config/crd/bases/kuadrant.io_authpolicies.yaml" \
  -o testdata/authpolicy-v1-crd.yaml

# RateLimitPolicy v1 from Kuadrant v1.3.0
curl -sSL "https://raw.githubusercontent.com/Kuadrant/kuadrant-operator/v1.3.0/config/crd/bases/kuadrant.io_ratelimitpolicies.yaml" \
  -o testdata/ratelimitpolicy-v1-crd.yaml

# DNSPolicy v1 from Kuadrant v1.3.0
curl -sSL "https://raw.githubusercontent.com/Kuadrant/kuadrant-operator/v1.3.0/config/crd/bases/kuadrant.io_dnspolicies.yaml" \
  -o testdata/dnspolicy-v1-crd.yaml

# TLSPolicy v1 from Kuadrant v1.3.0
curl -sSL "https://raw.githubusercontent.com/Kuadrant/kuadrant-operator/v1.3.0/config/crd/bases/kuadrant.io_tlspolicies.yaml" \
  -o testdata/tlspolicy-v1-crd.yaml

# Kuadrant CR v1 from Kuadrant v1.3.0
curl -sSL "https://raw.githubusercontent.com/Kuadrant/kuadrant-operator/v1.3.0/config/crd/bases/kuadrant.io_kuadrants.yaml" \
  -o testdata/kuadrant-v1-crd.yaml
```

## Notes

- These are **complete, official CRDs** from Kuadrant releases
- They include all fields, validations, and metadata from the actual releases
- Update fixtures when:
  - Testing against a new Kuadrant version
  - Adding new required fields to handlers
  - Validating compatibility with different API versions

