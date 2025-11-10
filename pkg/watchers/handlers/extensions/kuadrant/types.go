package kuadrant

import "github.com/aslakknutsen/kkbase/pkg/models"

// Kuadrant NodeTypes
// These constants represent node types for Kuadrant resources including
// the operator management CR and all policy CRDs.
const (
	NodeTypeKuadrant         models.NodeType = "Kuadrant"
	NodeTypeAuthPolicy       models.NodeType = "AuthPolicy"
	NodeTypeRateLimitPolicy  models.NodeType = "RateLimitPolicy"
	NodeTypeDNSPolicy        models.NodeType = "DNSPolicy"
	NodeTypeTLSPolicy        models.NodeType = "TLSPolicy"
)

// Kuadrant EdgeTypes
// These constants represent edge types (relationships) specific to Kuadrant resources.
// Shared edge types (used across multiple handler packages) remain in models.EdgeType.
const (
	EdgeTypeAppliesTo   models.EdgeType = "APPLIES_TO"
	EdgeTypeEnforcedBy  models.EdgeType = "ENFORCED_BY"
	EdgeTypeManagedBy   models.EdgeType = "MANAGED_BY"
)

