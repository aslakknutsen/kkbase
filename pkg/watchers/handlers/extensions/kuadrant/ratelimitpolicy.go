package kuadrant

import (
	"context"

	"github.com/aslakknutsen/kkbase/pkg/graph"
	"github.com/aslakknutsen/kkbase/pkg/models"
	"github.com/aslakknutsen/kkbase/pkg/watchers"
	"github.com/aslakknutsen/kkbase/pkg/watchers/handlers/common"
	"github.com/aslakknutsen/kkbase/pkg/watchers/schema"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8sschema "k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// RateLimitPolicyFieldRequirements defines the required and optional fields for RateLimitPolicy CRDs
var RateLimitPolicyFieldRequirements = []schema.FieldRequirement{
	{
		Name:        "targetRef",
		Description: "Policy attachment target",
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
		Name:        "limits",
		Description: "Rate limit definitions",
		Required:    false,
		Paths:       []string{"spec.limits"},
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

// RateLimitPolicyHandler handles Kuadrant RateLimitPolicy resources (version-agnostic)
type RateLimitPolicyHandler struct {
	*watchers.BaseWatcher
	clientset           *kubernetes.Clientset
	dynamicClient       dynamic.Interface
	relationshipBuilder *RelationshipBuilder
	extractor           *schema.FieldExtractor
}

// NewRateLimitPolicyHandler creates a new version-agnostic RateLimitPolicy handler
func NewRateLimitPolicyHandler(
	clientset *kubernetes.Clientset,
	dynamicClient dynamic.Interface,
	graphStore graph.GraphStore,
	logger *zap.Logger,
	factory dynamicinformer.DynamicSharedInformerFactory,
	extractor *schema.FieldExtractor,
) *RateLimitPolicyHandler {
	gvr := k8sschema.GroupVersionResource{
		Group:    "kuadrant.io",
		Version:  extractor.GetVersion(),
		Resource: "ratelimitpolicies",
	}
	informer := factory.ForResource(gvr).Informer()

	handler := &RateLimitPolicyHandler{
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

// HandleAdd processes a newly added RateLimitPolicy
func (h *RateLimitPolicyHandler) HandleAdd(obj interface{}) {
	policy, err := watchers.ConvertToTyped[unstructured.Unstructured](obj)
	if err != nil {
		h.Logger.Error("failed to convert to Unstructured", zap.Error(err))
		return
	}

	h.Logger.Debug("ratelimitpolicy added",
		zap.String("namespace", policy.GetNamespace()),
		zap.String("name", policy.GetName()),
	)

	ctx := context.Background()

	// Extract targetRef
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

	targetGroup, _, _ := h.extractor.ExtractString(policy, "targetRef.group")

	// Create RateLimitPolicy node
	policyNode := &models.GraphNode{
		Type: NodeTypeRateLimitPolicy,
		ID:   models.GetNodeID(NodeTypeRateLimitPolicy, policy.GetNamespace(), policy.GetName()),
		Properties: map[string]interface{}{
			"name":             policy.GetName(),
			"namespace":        policy.GetNamespace(),
			"version":          policy.GetAPIVersion(),
			"target_kind":      targetKind,
			"target_name":      targetName,
			"target_namespace": policy.GetNamespace(),
		},
	}

	if targetGroup != "" {
		policyNode.Properties["target_group"] = targetGroup
	}

	// Extract limits (optional)
	if h.extractor.HasField("limits") {
		if limits, found, _ := h.extractor.ExtractMap(policy, "limits"); found {
			policyNode.Properties["limits_count"] = len(limits)
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

	// Extract status conditions with per-condition messages and reasons
	statusProps := make(map[string]interface{})
	if h.extractor.HasField("status.conditions") {
		if conditions, found, _ := h.extractor.ExtractSlice(policy, "status.conditions"); found {
			// Find key condition types
			for _, cond := range conditions {
				if condMap, ok := cond.(map[string]interface{}); ok {
					condType, _ := condMap["type"].(string)
					status, _ := condMap["status"].(string)
					message, _ := condMap["message"].(string)
					reason, _ := condMap["reason"].(string)

					switch condType {
					case "Accepted":
						policyNode.Properties["status_accepted"] = (status == "True")
						statusProps["status_accepted"] = (status == "True")
						if message != "" {
							policyNode.Properties["status_accepted_message"] = message
							statusProps["status_accepted_message"] = message
						}
						if reason != "" {
							policyNode.Properties["status_accepted_reason"] = reason
							statusProps["status_accepted_reason"] = reason
						}
					case "Enforced":
						policyNode.Properties["status_enforced"] = (status == "True")
						statusProps["status_enforced"] = (status == "True")
						if message != "" {
							policyNode.Properties["status_enforced_message"] = message
							statusProps["status_enforced_message"] = message
						}
						if reason != "" {
							policyNode.Properties["status_enforced_reason"] = reason
							statusProps["status_enforced_reason"] = reason
						}
					case "Failed":
						policyNode.Properties["status_failed"] = (status == "True")
						statusProps["status_failed"] = (status == "True")
						if message != "" {
							policyNode.Properties["status_failed_message"] = message
							statusProps["status_failed_message"] = message
						}
						if reason != "" {
							policyNode.Properties["status_failed_reason"] = reason
							statusProps["status_failed_reason"] = reason
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

	if labels := policy.GetLabels(); len(labels) > 0 {
		policyNode.Properties["labels"] = serializeMap(labels)
	}

	// Add annotations
	if annotations := common.SerializeAnnotations(policy.GetAnnotations()); annotations != "" {
		policyNode.Properties["annotations"] = annotations
	}

	// Store complete spec and status
	storeCompleteResourceSpec(policy, policyNode.Properties)

	if err := h.GraphStore.UpsertNode(ctx, string(policyNode.Type), policyNode.ID, policyNode.Properties); err != nil {
		h.Logger.Error("failed to create ratelimitpolicy node", zap.Error(err))
		return
	}

	// Create IN_NAMESPACE edge
	if err := h.relationshipBuilder.base.CreateNamespaceEdge(ctx, NodeTypeRateLimitPolicy, policyNode.ID, policy.GetNamespace()); err != nil {
		h.Logger.Error("failed to create namespace edge", zap.Error(err))
	}

	// Create APPLIES_TO edge with per-target status
	if err := h.relationshipBuilder.CreatePolicyAppliesToEdge(
		ctx,
		NodeTypeRateLimitPolicy,
		policy.GetNamespace(),
		policy.GetName(),
		targetGroup,
		targetKind,
		policy.GetNamespace(),
		targetName,
		statusProps,
	); err != nil {
		h.Logger.Error("failed to create APPLIES_TO edge", zap.Error(err))
	}

	// Create ENFORCED_BY edge to Limitador service
	// Discover Limitador service by label (app=limitador)
	if limitadorNs, limitadorName, found := h.relationshipBuilder.FindServiceByLabel(ctx, "app=limitador"); found {
		if err := h.relationshipBuilder.CreatePolicyEnforcedByEdge(
			ctx,
			NodeTypeRateLimitPolicy,
			policy.GetNamespace(),
			policy.GetName(),
			limitadorNs,
			limitadorName,
		); err != nil {
			h.Logger.Error("failed to create ENFORCED_BY edge",
				zap.Error(err),
				zap.String("limitador_service", limitadorName),
				zap.String("limitador_namespace", limitadorNs),
			)
		}
	} else {
		h.Logger.Debug("limitador service not found, skipping ENFORCED_BY edge",
			zap.String("policy", policy.GetName()),
		)
	}
}

// HandleUpdate processes an updated RateLimitPolicy
func (h *RateLimitPolicyHandler) HandleUpdate(oldObj, newObj interface{}) {
	policy, err := watchers.ConvertToTyped[unstructured.Unstructured](newObj)
	if err != nil {
		h.Logger.Error("failed to convert to Unstructured", zap.Error(err))
		return
	}

	h.Logger.Debug("ratelimitpolicy updated",
		zap.String("namespace", policy.GetNamespace()),
		zap.String("name", policy.GetName()),
	)

	ctx := context.Background()
	policyID := models.GetNodeID(NodeTypeRateLimitPolicy, policy.GetNamespace(), policy.GetName())

	if err := h.GraphStore.DeleteEdgesByNode(ctx, string(NodeTypeRateLimitPolicy), policyID); err != nil {
		h.Logger.Error("failed to delete old edges", zap.Error(err))
	}

	h.HandleAdd(newObj)
}

// HandleDelete processes a deleted RateLimitPolicy
func (h *RateLimitPolicyHandler) HandleDelete(obj interface{}) {
	policy, err := watchers.ConvertToTyped[unstructured.Unstructured](obj)
	if err != nil {
		h.Logger.Error("failed to convert to Unstructured", zap.Error(err))
		return
	}

	h.Logger.Debug("ratelimitpolicy deleted",
		zap.String("namespace", policy.GetNamespace()),
		zap.String("name", policy.GetName()),
	)

	ctx := context.Background()
	policyID := models.GetNodeID(NodeTypeRateLimitPolicy, policy.GetNamespace(), policy.GetName())

	if err := h.GraphStore.DeleteNode(ctx, string(NodeTypeRateLimitPolicy), policyID); err != nil {
		h.Logger.Error("failed to delete ratelimitpolicy node", zap.Error(err))
	}
}
