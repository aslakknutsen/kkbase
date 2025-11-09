package kuadrant

import (
	"context"

	"github.com/kagenti/kkbase/pkg/graph"
	"github.com/kagenti/kkbase/pkg/models"
	coretypes "github.com/kagenti/kkbase/pkg/watchers/handlers/core"
	gatewaytypes "github.com/kagenti/kkbase/pkg/watchers/handlers/extensions/gateway"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// RelationshipBuilder helps build relationships for Kuadrant resources
type RelationshipBuilder struct {
	base      *coretypes.RelationshipBuilder
	clientset *kubernetes.Clientset
	logger    *zap.Logger
}

// NewRelationshipBuilder creates a new kuadrant relationship builder
func NewRelationshipBuilder(clientset *kubernetes.Clientset, graphStore graph.GraphStore, logger *zap.Logger) *RelationshipBuilder {
	return &RelationshipBuilder{
		base:      coretypes.NewRelationshipBuilder(clientset, graphStore, logger),
		clientset: clientset,
		logger:    logger,
	}
}

// CreatePolicyAppliesToEdge creates APPLIES_TO edge from Policy to Gateway or HTTPRoute
// This handles the spec.targetRef field in Kuadrant policies
func (rb *RelationshipBuilder) CreatePolicyAppliesToEdge(
	ctx context.Context,
	policyType models.NodeType,
	policyNamespace, policyName string,
	targetGroup, targetKind, targetNamespace, targetName string,
	statusProps map[string]interface{},
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

	// Add per-target status properties
	for k, v := range statusProps {
		properties[k] = v
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

// FindServiceByLabel searches cluster-wide for a Service with the given label selector
// Returns the first matching service found, or nil if none exist
func (rb *RelationshipBuilder) FindServiceByLabel(ctx context.Context, labelSelector string) (namespace, name string, found bool) {
	services, err := rb.clientset.CoreV1().Services("").List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		rb.logger.Error("failed to list services by label",
			zap.String("label_selector", labelSelector),
			zap.Error(err),
		)
		return "", "", false
	}

	if len(services.Items) == 0 {
		return "", "", false
	}

	// Return the first service found
	// If multiple exist, they should all be equivalent enforcement services
	svc := services.Items[0]
	return svc.Namespace, svc.Name, true
}

// CreatePolicyEnforcedByEdge creates ENFORCED_BY edge from Policy to enforcement Service
// This links policies to their enforcement services (Authorino for AuthPolicy, Limitador for RateLimitPolicy)
func (rb *RelationshipBuilder) CreatePolicyEnforcedByEdge(
	ctx context.Context,
	policyType models.NodeType,
	policyNamespace, policyName string,
	serviceNamespace, serviceName string,
) error {
	policyID := models.GetNodeID(policyType, policyNamespace, policyName)
	serviceID := models.GetNodeID(coretypes.NodeTypeService, serviceNamespace, serviceName)

	return rb.base.GraphStore.UpsertEdge(
		ctx,
		string(policyType),
		policyID,
		string(EdgeTypeEnforcedBy),
		string(coretypes.NodeTypeService),
		serviceID,
		nil,
	)
}

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
