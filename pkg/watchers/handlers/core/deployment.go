package core

import (
	"context"

	"github.com/kagenti/kkbase/pkg/graph"
	"github.com/kagenti/kkbase/pkg/models"
	"github.com/kagenti/kkbase/pkg/watchers"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// DeploymentHandler handles Deployment resources
type DeploymentHandler struct {
	*watchers.BaseWatcher
	clientset           *kubernetes.Clientset
	relationshipBuilder *watchers.RelationshipBuilder
}

// NewDeploymentHandler creates a new Deployment handler
func NewDeploymentHandler(
	clientset *kubernetes.Clientset,
	graphStore graph.GraphStore,
	logger *zap.Logger,
	factory dynamicinformer.DynamicSharedInformerFactory,
) *DeploymentHandler {
	gvr := schema.GroupVersionResource{
		Group:    "apps",
		Version:  "v1",
		Resource: "deployments",
	}
	informer := factory.ForResource(gvr).Informer()

	handler := &DeploymentHandler{
		BaseWatcher:         watchers.NewBaseWatcher(graphStore, logger, informer),
		clientset:           clientset,
		relationshipBuilder: watchers.NewRelationshipBuilder(clientset, graphStore, logger),
	}

	_, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    handler.HandleAdd,
		UpdateFunc: handler.HandleUpdate,
		DeleteFunc: handler.HandleDelete,
	})
	if err != nil {
		logger.Error("failed to add event handler", zap.Error(err))
	}

	return handler
}

// HandleAdd processes a newly added Deployment
func (h *DeploymentHandler) HandleAdd(obj interface{}) {
	deployment, err := watchers.ConvertToTyped[appsv1.Deployment](obj)
	if err != nil {
		h.Logger.Error("failed to convert to Deployment", zap.Error(err))
		return
	}

	h.Logger.Debug("deployment added", zap.String("namespace", deployment.Namespace), zap.String("name", deployment.Name))

	ctx := context.Background()

	// Create Deployment node
	deploymentNode := models.DeploymentToGraphNode(deployment)
	if err := h.GraphStore.UpsertNode(ctx, string(deploymentNode.Type), deploymentNode.ID, deploymentNode.Properties); err != nil {
		h.Logger.Error("failed to create deployment node", zap.Error(err), zap.String("deployment", deployment.Name))
		return
	}

	// Create IN_NAMESPACE edge
	if err := h.relationshipBuilder.CreateNamespaceEdge(ctx, models.NodeTypeDeployment, deploymentNode.ID, deployment.Namespace); err != nil {
		h.Logger.Error("failed to create namespace edge", zap.Error(err))
	}
}

// HandleUpdate processes an updated Deployment
func (h *DeploymentHandler) HandleUpdate(oldObj, newObj interface{}) {
	newDeployment, err := watchers.ConvertToTyped[appsv1.Deployment](newObj)
	if err != nil {
		h.Logger.Error("failed to convert to Deployment", zap.Error(err))
		return
	}

	h.Logger.Debug("deployment updated", zap.String("namespace", newDeployment.Namespace), zap.String("name", newDeployment.Name))

	h.HandleAdd(newObj)
}

// HandleDelete processes a deleted Deployment
func (h *DeploymentHandler) HandleDelete(obj interface{}) {
	deployment, err := watchers.ConvertToTyped[appsv1.Deployment](obj)
	if err != nil {
		h.Logger.Error("failed to convert to Deployment", zap.Error(err))
		return
	}

	h.Logger.Debug("deployment deleted", zap.String("namespace", deployment.Namespace), zap.String("name", deployment.Name))

	ctx := context.Background()

	deploymentID := models.GetNodeID("Deployment", deployment.Namespace, deployment.Name)
	if err := h.GraphStore.DeleteNode(ctx, string(models.NodeTypeDeployment), deploymentID); err != nil {
		h.Logger.Error("failed to delete deployment node", zap.Error(err), zap.String("deployment", deployment.Name))
	}
}
