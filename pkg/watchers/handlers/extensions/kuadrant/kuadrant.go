package kuadrant

import (
	"context"

	"github.com/aslakknutsen/kkbase/pkg/graph"
	"github.com/aslakknutsen/kkbase/pkg/models"
	"github.com/aslakknutsen/kkbase/pkg/watchers"
	"github.com/aslakknutsen/kkbase/pkg/watchers/schema"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8sschema "k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// KuadrantFieldRequirements defines the required and optional fields for Kuadrant CR
var KuadrantFieldRequirements = []schema.FieldRequirement{
	// Kuadrant CR has minimal requirements - mostly status
	{
		Name:        "status",
		Description: "Operational status",
		Required:    false,
		Paths:       []string{"status"},
	},
}

// KuadrantHandler handles Kuadrant CR resources (version-agnostic)
type KuadrantHandler struct {
	*watchers.BaseWatcher
	clientset           *kubernetes.Clientset
	dynamicClient       dynamic.Interface
	relationshipBuilder *RelationshipBuilder
	extractor           *schema.FieldExtractor // May be nil if schema validation failed
}

// NewKuadrantHandler creates a new version-agnostic Kuadrant handler
func NewKuadrantHandler(
	clientset *kubernetes.Clientset,
	dynamicClient dynamic.Interface,
	graphStore graph.GraphStore,
	logger *zap.Logger,
	factory dynamicinformer.DynamicSharedInformerFactory,
	extractor *schema.FieldExtractor, // May be nil
) *KuadrantHandler {
	// Use a default version if extractor is nil
	version := "v1beta1"
	if extractor != nil {
		version = extractor.GetVersion()
	}

	gvr := k8sschema.GroupVersionResource{
		Group:    "kuadrant.io",
		Version:  version,
		Resource: "kuadrants",
	}
	informer := factory.ForResource(gvr).Informer()

	handler := &KuadrantHandler{
		BaseWatcher:         watchers.NewBaseWatcher(graphStore, logger, informer),
		clientset:           clientset,
		dynamicClient:       dynamicClient,
		relationshipBuilder: NewRelationshipBuilder(clientset, graphStore, logger),
		extractor:           extractor,
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

// HandleAdd processes a newly added Kuadrant CR
func (h *KuadrantHandler) HandleAdd(obj interface{}) {
	kuadrant, err := watchers.ConvertToTyped[unstructured.Unstructured](obj)
	if err != nil {
		h.Logger.Error("failed to convert to Unstructured", zap.Error(err))
		return
	}

	h.Logger.Debug("kuadrant added",
		zap.String("namespace", kuadrant.GetNamespace()),
		zap.String("name", kuadrant.GetName()),
	)

	ctx := context.Background()

	// Create Kuadrant node
	kuadrantNode := &models.GraphNode{
		Type: NodeTypeKuadrant,
		ID:   models.GetNodeID(NodeTypeKuadrant, kuadrant.GetNamespace(), kuadrant.GetName()),
		Properties: map[string]interface{}{
			"name":      kuadrant.GetName(),
			"namespace": kuadrant.GetNamespace(),
			"version":   kuadrant.GetAPIVersion(),
		},
	}

	// Extract optional fields if extractor is available
	if h.extractor != nil {
		if h.extractor.HasField("spec.clusterManaged") {
			if clusterManaged, found, _ := h.extractor.ExtractString(kuadrant, "spec.clusterManaged"); found {
				kuadrantNode.Properties["cluster_managed"] = clusterManaged
			}
		}
	}

	if labels := kuadrant.GetLabels(); len(labels) > 0 {
		kuadrantNode.Properties["labels"] = serializeMap(labels)
	}

	// Store complete spec and status
	storeCompleteResourceSpec(kuadrant, kuadrantNode.Properties)

	if err := h.GraphStore.UpsertNode(ctx, string(kuadrantNode.Type), kuadrantNode.ID, kuadrantNode.Properties); err != nil {
		h.Logger.Error("failed to create kuadrant node", zap.Error(err))
		return
	}

	// Create IN_NAMESPACE edge
	if err := h.relationshipBuilder.base.CreateNamespaceEdge(ctx, NodeTypeKuadrant, kuadrantNode.ID, kuadrant.GetNamespace()); err != nil {
		h.Logger.Error("failed to create namespace edge", zap.Error(err))
	}

	// Create MANAGES edges to Authorino and Limitador services
	// Discover services by label (app=authorino, app=limitador)
	if authorinoNs, authorinoName, found := h.relationshipBuilder.FindServiceByLabel(ctx, "app=authorino"); found {
		if err := h.relationshipBuilder.CreateKuadrantManagesServiceEdge(
			ctx,
			kuadrant.GetNamespace(),
			kuadrant.GetName(),
			authorinoNs,
			authorinoName,
		); err != nil {
			h.Logger.Error("failed to create MANAGES edge to Authorino",
				zap.Error(err),
				zap.String("authorino_service", authorinoName),
			)
		}
	}

	if limitadorNs, limitadorName, found := h.relationshipBuilder.FindServiceByLabel(ctx, "app=limitador"); found {
		if err := h.relationshipBuilder.CreateKuadrantManagesServiceEdge(
			ctx,
			kuadrant.GetNamespace(),
			kuadrant.GetName(),
			limitadorNs,
			limitadorName,
		); err != nil {
			h.Logger.Error("failed to create MANAGES edge to Limitador",
				zap.Error(err),
				zap.String("limitador_service", limitadorName),
			)
		}
	}
}

// HandleUpdate processes an updated Kuadrant CR
func (h *KuadrantHandler) HandleUpdate(oldObj, newObj interface{}) {
	kuadrant, err := watchers.ConvertToTyped[unstructured.Unstructured](newObj)
	if err != nil {
		h.Logger.Error("failed to convert to Unstructured", zap.Error(err))
		return
	}

	h.Logger.Debug("kuadrant updated",
		zap.String("namespace", kuadrant.GetNamespace()),
		zap.String("name", kuadrant.GetName()),
	)

	ctx := context.Background()
	kuadrantID := models.GetNodeID(NodeTypeKuadrant, kuadrant.GetNamespace(), kuadrant.GetName())

	if err := h.GraphStore.DeleteEdgesByNode(ctx, string(NodeTypeKuadrant), kuadrantID); err != nil {
		h.Logger.Error("failed to delete old edges", zap.Error(err))
	}

	h.HandleAdd(newObj)
}

// HandleDelete processes a deleted Kuadrant CR
func (h *KuadrantHandler) HandleDelete(obj interface{}) {
	kuadrant, err := watchers.ConvertToTyped[unstructured.Unstructured](obj)
	if err != nil {
		h.Logger.Error("failed to convert to Unstructured", zap.Error(err))
		return
	}

	h.Logger.Debug("kuadrant deleted",
		zap.String("namespace", kuadrant.GetNamespace()),
		zap.String("name", kuadrant.GetName()),
	)

	ctx := context.Background()
	kuadrantID := models.GetNodeID(NodeTypeKuadrant, kuadrant.GetNamespace(), kuadrant.GetName())

	if err := h.GraphStore.DeleteNode(ctx, string(NodeTypeKuadrant), kuadrantID); err != nil {
		h.Logger.Error("failed to delete kuadrant node", zap.Error(err))
	}
}

