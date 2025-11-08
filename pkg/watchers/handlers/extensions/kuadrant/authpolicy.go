package kuadrant

import (
	"context"

	"github.com/kagenti/kkbase/pkg/graph"
	"github.com/kagenti/kkbase/pkg/models"
	"github.com/kagenti/kkbase/pkg/watchers"
	"github.com/kagenti/kkbase/pkg/watchers/schema"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8sschema "k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// AuthPolicyFieldRequirements defines the required and optional fields for AuthPolicy CRDs
var AuthPolicyFieldRequirements = []schema.FieldRequirement{
	{
		Name:        "targetRef",
		Description: "Policy attachment target (Gateway/HTTPRoute)",
		Required:    true,
		Paths:       []string{"spec.targetRef"},
	},
	{
		Name:        "targetRef.kind",
		Description: "Kind of target resource",
		Required:    true,
		Paths:       []string{"spec.targetRef.kind"},
	},
	{
		Name:        "targetRef.name",
		Description: "Name of target resource",
		Required:    true,
		Paths:       []string{"spec.targetRef.name"},
	},
	{
		Name:        "targetRef.group",
		Description: "Group of target resource",
		Required:    false,
		Paths:       []string{"spec.targetRef.group"},
	},
	{
		Name:        "authentication",
		Description: "Authentication and authorization rules",
		Required:    false,
		Paths:       []string{"spec.authentication"},
	},
	{
		Name:        "defaults",
		Description: "Default policy rules (can be overridden)",
		Required:    false,
		Paths:       []string{"spec.defaults"},
	},
	{
		Name:        "overrides",
		Description: "Override policy rules (cannot be overridden)",
		Required:    false,
		Paths:       []string{"spec.overrides"},
	},
	{
		Name:        "status.conditions",
		Description: "Policy health and enforcement status",
		Required:    false,
		Paths:       []string{"status.conditions"},
	},
	{
		Name:        "status.observedGeneration",
		Description: "Last observed generation for staleness detection",
		Required:    false,
		Paths:       []string{"status.observedGeneration"},
	},
}

// AuthPolicyHandler handles Kuadrant AuthPolicy resources (version-agnostic)
type AuthPolicyHandler struct {
	*watchers.BaseWatcher
	clientset           *kubernetes.Clientset
	dynamicClient       dynamic.Interface
	relationshipBuilder *RelationshipBuilder
	extractor           *schema.FieldExtractor // Validated field paths for this CRD version
}

// NewAuthPolicyHandler creates a new version-agnostic AuthPolicy handler
func NewAuthPolicyHandler(
	clientset *kubernetes.Clientset,
	dynamicClient dynamic.Interface,
	graphStore graph.GraphStore,
	logger *zap.Logger,
	factory dynamicinformer.DynamicSharedInformerFactory,
	extractor *schema.FieldExtractor,
) *AuthPolicyHandler {
	// Use the version from the extractor
	gvr := k8sschema.GroupVersionResource{
		Group:    "kuadrant.io",
		Version:  extractor.GetVersion(),
		Resource: "authpolicies",
	}
	informer := factory.ForResource(gvr).Informer()

	handler := &AuthPolicyHandler{
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

// HandleAdd processes a newly added AuthPolicy
func (h *AuthPolicyHandler) HandleAdd(obj interface{}) {
	policy, err := watchers.ConvertToTyped[unstructured.Unstructured](obj)
	if err != nil {
		h.Logger.Error("failed to convert to Unstructured", zap.Error(err))
		return
	}

	h.Logger.Debug("authpolicy added",
		zap.String("namespace", policy.GetNamespace()),
		zap.String("name", policy.GetName()),
		zap.String("version", policy.GetAPIVersion()),
	)

	ctx := context.Background()

	// Extract targetRef using validated paths
	targetKind, _, err := h.extractor.ExtractString(policy, "targetRef.kind")
	if err != nil {
		h.Logger.Error("failed to extract targetRef.kind", zap.Error(err))
		return
	}

	targetName, _, err := h.extractor.ExtractString(policy, "targetRef.name")
	if err != nil {
		h.Logger.Error("failed to extract targetRef.name", zap.Error(err))
		return
	}

	// Group is optional
	targetGroup, _, _ := h.extractor.ExtractString(policy, "targetRef.group")

	// Create AuthPolicy node
	policyNode := &models.GraphNode{
		Type: NodeTypeAuthPolicy,
		ID:   models.GetNodeID(NodeTypeAuthPolicy, policy.GetNamespace(), policy.GetName()),
		Properties: map[string]interface{}{
			"name":             policy.GetName(),
			"namespace":        policy.GetNamespace(),
			"version":          policy.GetAPIVersion(),
			"target_kind":      targetKind,
			"target_name":      targetName,
			"target_namespace": policy.GetNamespace(), // Local policy
		},
	}

	if targetGroup != "" {
		policyNode.Properties["target_group"] = targetGroup
	}

	// Extract optional fields
	if h.extractor.HasField("authentication") {
		if _, found, _ := h.extractor.ExtractMap(policy, "authentication"); found {
			// v1 structure: spec.authentication contains auth rules
			policyNode.Properties["authentication_configured"] = true
		}
	}

	// Extract policy precedence information
	hasDefaults := false
	hasOverrides := false

	if h.extractor.HasField("defaults") {
		if _, found, _ := h.extractor.ExtractMap(policy, "defaults"); found {
			hasDefaults = true
		}
	}

	if h.extractor.HasField("overrides") {
		if _, found, _ := h.extractor.ExtractMap(policy, "overrides"); found {
			hasOverrides = true
		}
	}

	if hasDefaults {
		policyNode.Properties["policy_type"] = "defaults"
	} else if hasOverrides {
		policyNode.Properties["policy_type"] = "overrides"
	} else {
		policyNode.Properties["policy_type"] = "implicit_defaults"
	}

	// Extract status conditions for quick diagnostics
	if h.extractor.HasField("status.conditions") {
		if conditions, found, _ := h.extractor.ExtractSlice(policy, "status.conditions"); found {
			// Find key condition types
			for _, cond := range conditions {
				if condMap, ok := cond.(map[string]interface{}); ok {
					condType, _ := condMap["type"].(string)
					status, _ := condMap["status"].(string)

					switch condType {
					case "Accepted":
						policyNode.Properties["status_accepted"] = (status == "True")
					case "Enforced":
						policyNode.Properties["status_enforced"] = (status == "True")
					case "Failed":
						policyNode.Properties["status_failed"] = (status == "True")
						if msg, ok := condMap["message"].(string); ok && status == "True" {
							policyNode.Properties["status_message"] = msg
						}
					}
				}
			}
		}
	}

	// Extract observedGeneration for staleness detection
	if h.extractor.HasField("status.observedGeneration") {
		if obsGen, found, _ := h.extractor.ExtractInt(policy, "status.observedGeneration"); found {
			policyNode.Properties["observed_generation"] = obsGen
			// Compare with metadata.generation to detect stale status
			if policy.GetGeneration() != obsGen {
				policyNode.Properties["status_stale"] = true
			}
		}
	}

	// Add labels
	if labels := policy.GetLabels(); len(labels) > 0 {
		policyNode.Properties["labels"] = serializeMap(labels)
	}

	// Store complete spec and status
	storeCompleteResourceSpec(policy, policyNode.Properties)

	if err := h.GraphStore.UpsertNode(ctx, string(policyNode.Type), policyNode.ID, policyNode.Properties); err != nil {
		h.Logger.Error("failed to create authpolicy node", zap.Error(err))
		return
	}

	// Create IN_NAMESPACE edge
	if err := h.relationshipBuilder.base.CreateNamespaceEdge(ctx, NodeTypeAuthPolicy, policyNode.ID, policy.GetNamespace()); err != nil {
		h.Logger.Error("failed to create namespace edge", zap.Error(err))
	}

	// Create APPLIES_TO edge to target (Gateway or HTTPRoute)
	targetNamespace := policy.GetNamespace() // Local policy

	if err := h.relationshipBuilder.CreatePolicyAppliesToEdge(
		ctx,
		NodeTypeAuthPolicy,
		policy.GetNamespace(),
		policy.GetName(),
		targetGroup,
		targetKind,
		targetNamespace,
		targetName,
	); err != nil {
		h.Logger.Error("failed to create APPLIES_TO edge",
			zap.Error(err),
			zap.String("target_kind", targetKind),
			zap.String("target_name", targetName),
		)
	}

	// TODO: Create ENFORCED_BY edge to Authorino service
	// This requires understanding how Kuadrant discovers Authorino instances
	// See relationships.go for investigation notes
}

// HandleUpdate processes an updated AuthPolicy
func (h *AuthPolicyHandler) HandleUpdate(oldObj, newObj interface{}) {
	policy, err := watchers.ConvertToTyped[unstructured.Unstructured](newObj)
	if err != nil {
		h.Logger.Error("failed to convert to Unstructured", zap.Error(err))
		return
	}

	h.Logger.Debug("authpolicy updated",
		zap.String("namespace", policy.GetNamespace()),
		zap.String("name", policy.GetName()),
	)

	ctx := context.Background()
	policyID := models.GetNodeID(NodeTypeAuthPolicy, policy.GetNamespace(), policy.GetName())

	// Delete old edges
	if err := h.GraphStore.DeleteEdgesByNode(ctx, string(NodeTypeAuthPolicy), policyID); err != nil {
		h.Logger.Error("failed to delete old authpolicy edges", zap.Error(err))
	}

	// Recreate with new data
	h.HandleAdd(newObj)
}

// HandleDelete processes a deleted AuthPolicy
func (h *AuthPolicyHandler) HandleDelete(obj interface{}) {
	policy, err := watchers.ConvertToTyped[unstructured.Unstructured](obj)
	if err != nil {
		h.Logger.Error("failed to convert to Unstructured", zap.Error(err))
		return
	}

	h.Logger.Debug("authpolicy deleted",
		zap.String("namespace", policy.GetNamespace()),
		zap.String("name", policy.GetName()),
	)

	ctx := context.Background()

	policyID := models.GetNodeID(NodeTypeAuthPolicy, policy.GetNamespace(), policy.GetName())
	if err := h.GraphStore.DeleteNode(ctx, string(NodeTypeAuthPolicy), policyID); err != nil {
		h.Logger.Error("failed to delete authpolicy node", zap.Error(err))
	}
}
