package gateway

import (
	"context"

	"github.com/aslakknutsen/kkbase/pkg/graph"
	"github.com/aslakknutsen/kkbase/pkg/models"
	"github.com/aslakknutsen/kkbase/pkg/watchers/handlers/core"
	coretypes "github.com/aslakknutsen/kkbase/pkg/watchers/handlers/core"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
)

// RelationshipBuilder helps build relationships for Gateway API resources
type RelationshipBuilder struct {
	base *core.RelationshipBuilder
}

// NewRelationshipBuilder creates a new gateway relationship builder
func NewRelationshipBuilder(clientset *kubernetes.Clientset, graphStore graph.GraphStore, logger *zap.Logger) *RelationshipBuilder {
	return &RelationshipBuilder{
		base: core.NewRelationshipBuilder(clientset, graphStore, logger),
	}
}

// CreateGatewayImplementedByEdge creates IMPLEMENTED_BY edge from Gateway to GatewayClass
func (rb *RelationshipBuilder) CreateGatewayImplementedByEdge(ctx context.Context, gatewayNamespace, gatewayName, gatewayClassName string) error {
	gatewayID := models.GetNodeID(NodeTypeGateway, gatewayNamespace, gatewayName)
	gatewayClassID := models.GetNodeID(NodeTypeGatewayClass, "", gatewayClassName)

	return rb.base.GraphStore.UpsertEdge(
		ctx,
		string(NodeTypeGateway),
		gatewayID,
		string(EdgeTypeImplementedBy),
		string(NodeTypeGatewayClass),
		gatewayClassID,
		nil,
	)
}

// CreateGatewayTLSEdge creates USES_TLS_FROM edge from Gateway to Secret
func (rb *RelationshipBuilder) CreateGatewayTLSEdge(ctx context.Context, gatewayNamespace, gatewayName, secretNamespace, secretName, listenerName string) error {
	gatewayID := models.GetNodeID(NodeTypeGateway, gatewayNamespace, gatewayName)
	secretID := models.GetNodeID(coretypes.NodeTypeSecret, secretNamespace, secretName)

	properties := map[string]interface{}{
		"listener_name": listenerName,
	}

	return rb.base.GraphStore.UpsertEdge(
		ctx,
		string(NodeTypeGateway),
		gatewayID,
		string(EdgeTypeUsesTLSFrom),
		string(coretypes.NodeTypeSecret),
		secretID,
		properties,
	)
}

// CreateRouteAttachesToEdge creates ATTACHES_TO edge from Route to Gateway
func (rb *RelationshipBuilder) CreateRouteAttachesToEdge(ctx context.Context, routeType models.NodeType, routeNamespace, routeName, gatewayNamespace, gatewayName string, sectionName *string, statusProps map[string]interface{}) error {
	routeID := models.GetNodeID(routeType, routeNamespace, routeName)
	gatewayID := models.GetNodeID(NodeTypeGateway, gatewayNamespace, gatewayName)

	properties := map[string]interface{}{}
	if sectionName != nil {
		properties["section_name"] = *sectionName
	}

	// Add per-parent status properties
	for k, v := range statusProps {
		properties[k] = v
	}

	return rb.base.GraphStore.UpsertEdge(
		ctx,
		string(routeType),
		routeID,
		string(models.EdgeTypeAttachesTo),
		string(NodeTypeGateway),
		gatewayID,
		properties,
	)
}

// CreateRouteForwardsToEdge creates FORWARDS_TO edge from Route to Service
func (rb *RelationshipBuilder) CreateRouteForwardsToEdge(ctx context.Context, routeType models.NodeType, routeNamespace, routeName, serviceNamespace, serviceName string, weight *int32) error {
	routeID := models.GetNodeID(routeType, routeNamespace, routeName)
	serviceID := models.GetNodeID(coretypes.NodeTypeService, serviceNamespace, serviceName)

	properties := map[string]interface{}{}
	if weight != nil {
		properties["weight"] = *weight
	}

	return rb.base.GraphStore.UpsertEdge(
		ctx,
		string(routeType),
		routeID,
		string(EdgeTypeForwardsTo),
		string(coretypes.NodeTypeService),
		serviceID,
		properties,
	)
}

// CreateRoutePermittedByEdge creates PERMITTED_BY edge from Route to ReferenceGrant
func (rb *RelationshipBuilder) CreateRoutePermittedByEdge(ctx context.Context, routeType models.NodeType, routeNamespace, routeName, grantNamespace, grantName string) error {
	routeID := models.GetNodeID(routeType, routeNamespace, routeName)
	grantID := models.GetNodeID(NodeTypeReferenceGrant, grantNamespace, grantName)

	return rb.base.GraphStore.UpsertEdge(
		ctx,
		string(routeType),
		routeID,
		string(EdgeTypePermittedBy),
		string(NodeTypeReferenceGrant),
		grantID,
		nil,
	)
}

// CreateReferenceGrantAllowsEdge creates ALLOWS_ROUTE_TO edge from ReferenceGrant to Service
func (rb *RelationshipBuilder) CreateReferenceGrantAllowsEdge(ctx context.Context, grantNamespace, grantName, serviceNamespace, serviceName string) error {
	grantID := models.GetNodeID(NodeTypeReferenceGrant, grantNamespace, grantName)
	serviceID := models.GetNodeID(coretypes.NodeTypeService, serviceNamespace, serviceName)

	return rb.base.GraphStore.UpsertEdge(
		ctx,
		string(NodeTypeReferenceGrant),
		grantID,
		string(EdgeTypeAllowsRouteTo),
		string(coretypes.NodeTypeService),
		serviceID,
		nil,
	)
}
