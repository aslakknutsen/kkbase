package kuadrant

import (
	"context"

	"github.com/kagenti/kkbase/pkg/graph"
	"github.com/kagenti/kkbase/pkg/models"
	"github.com/kagenti/kkbase/pkg/watchers/handlers/core"
	coretypes "github.com/kagenti/kkbase/pkg/watchers/handlers/core"
	gatewaytypes "github.com/kagenti/kkbase/pkg/watchers/handlers/extensions/gateway"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
)

// RelationshipBuilder helps build relationships for Kuadrant resources
type RelationshipBuilder struct {
	base *core.RelationshipBuilder
}

// NewRelationshipBuilder creates a new kuadrant relationship builder
func NewRelationshipBuilder(clientset *kubernetes.Clientset, graphStore graph.GraphStore, logger *zap.Logger) *RelationshipBuilder {
	return &RelationshipBuilder{
		base: core.NewRelationshipBuilder(clientset, graphStore, logger),
	}
}

// CreatePolicyAppliesToEdge creates APPLIES_TO edge from Policy to Gateway or HTTPRoute
// This handles the spec.targetRef field in Kuadrant policies
func (rb *RelationshipBuilder) CreatePolicyAppliesToEdge(
	ctx context.Context,
	policyType models.NodeType,
	policyNamespace, policyName string,
	targetGroup, targetKind, targetNamespace, targetName string,
) error {
	policyID := models.GetNodeID(policyType, policyNamespace, policyName)

	// Determine target node type based on kind
	var targetNodeType models.NodeType
	switch targetKind {
	case "Gateway":
		targetNodeType = gatewaytypes.NodeTypeGateway
	case "HTTPRoute":
		targetNodeType = gatewaytypes.NodeTypeHTTPRoute
	default:
		// For future extensibility, handle other route types
		targetNodeType = models.NodeType(targetKind)
	}

	targetID := models.GetNodeID(targetNodeType, targetNamespace, targetName)

	properties := map[string]interface{}{
		"target_kind": targetKind,
	}
	if targetGroup != "" {
		properties["target_group"] = targetGroup
	}

	return rb.base.GraphStore.UpsertEdge(
		ctx,
		string(policyType),
		policyID,
		string(EdgeTypeAppliesTo),
		string(targetNodeType),
		targetID,
		properties,
	)
}

// CreatePolicyManagedByEdge creates MANAGED_BY edge from Policy to Kuadrant CR
func (rb *RelationshipBuilder) CreatePolicyManagedByEdge(
	ctx context.Context,
	policyType models.NodeType,
	policyNamespace, policyName string,
	kuadrantNamespace, kuadrantName string,
) error {
	policyID := models.GetNodeID(policyType, policyNamespace, policyName)
	kuadrantID := models.GetNodeID(NodeTypeKuadrant, kuadrantNamespace, kuadrantName)

	return rb.base.GraphStore.UpsertEdge(
		ctx,
		string(policyType),
		policyID,
		string(EdgeTypeManagedBy),
		string(NodeTypeKuadrant),
		kuadrantID,
		nil,
	)
}

// TODO: CreatePolicyEnforcedByEdge creates ENFORCED_BY edge from Policy to enforcement Service
// This requires investigating the Kuadrant operator source code to understand how policies
// discover their enforcement services (Authorino for AuthPolicy, Limitador for RateLimitPolicy).
//
// Possible discovery mechanisms to investigate:
// - Kuadrant CR contains explicit service references
// - Label selectors (e.g., app.kubernetes.io/component=authorino)
// - Naming conventions (e.g., "authorino" service in same namespace)
// - ConfigMap-based discovery
// - Operator-managed Service objects with known names
//
// Source to investigate:
// - github.com/Kuadrant/kuadrant-operator/api/v1beta1/kuadrant_types.go
// - github.com/Kuadrant/kuadrant-operator/controllers/*_controller.go
//
// Once the mechanism is understood, implement:
//
// func (rb *RelationshipBuilder) CreatePolicyEnforcedByEdge(
//     ctx context.Context,
//     policyType models.NodeType,
//     policyNamespace, policyName string,
//     serviceNamespace, serviceName string,
// ) error {
//     policyID := models.GetNodeID(policyType, policyNamespace, policyName)
//     serviceID := models.GetNodeID(coretypes.NodeTypeService, serviceNamespace, serviceName)
//
//     return rb.base.GraphStore.UpsertEdge(
//         ctx,
//         string(policyType),
//         policyID,
//         string(EdgeTypeEnforcedBy),
//         string(coretypes.NodeTypeService),
//         serviceID,
//         nil,
//     )
// }

// CreateKuadrantManagesServiceEdge creates MANAGES edge from Kuadrant CR to Service
// This links the Kuadrant operator to its managed components (Authorino, Limitador)
func (rb *RelationshipBuilder) CreateKuadrantManagesServiceEdge(
	ctx context.Context,
	kuadrantNamespace, kuadrantName string,
	serviceNamespace, serviceName string,
) error {
	kuadrantID := models.GetNodeID(NodeTypeKuadrant, kuadrantNamespace, kuadrantName)
	serviceID := models.GetNodeID(coretypes.NodeTypeService, serviceNamespace, serviceName)

	return rb.base.GraphStore.UpsertEdge(
		ctx,
		string(NodeTypeKuadrant),
		kuadrantID,
		string(models.EdgeTypeManages),
		string(coretypes.NodeTypeService),
		serviceID,
		nil,
	)
}

