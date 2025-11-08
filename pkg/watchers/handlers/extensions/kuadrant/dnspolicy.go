package kuadrant

import (
	"context"

	"github.com/kagenti/kkbase/pkg/graph"
	"github.com/kagenti/kkbase/pkg/models"
	"github.com/kagenti/kkbase/pkg/watchers"
	"github.com/kagenti/kkbase/pkg/watchers/handlers/core"
	"github.com/kagenti/kkbase/pkg/watchers/schema"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8sschema "k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// DNSPolicyFieldRequirements defines the required and optional fields for DNSPolicy CRDs
var DNSPolicyFieldRequirements = []schema.FieldRequirement{
	{
		Name:        "targetRef",
		Description: "Policy attachment target (Gateway only)",
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
		Name:        "loadBalancing",
		Description: "DNS load balancing strategy",
		Required:    false,
		Paths:       []string{"spec.loadBalancing"},
	},
	{
		Name:        "healthCheck",
		Description: "Health check configuration",
		Required:    false,
		Paths:       []string{"spec.healthCheck"},
	},
	{
		Name:        "providerRefs",
		Description: "References to DNS provider credential secrets",
		Required:    false,
		Paths:       []string{"spec.providerRefs"},
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

// DNSPolicyHandler handles Kuadrant DNSPolicy resources (version-agnostic)
type DNSPolicyHandler struct {
	*watchers.BaseWatcher
	clientset           *kubernetes.Clientset
	dynamicClient       dynamic.Interface
	relationshipBuilder *RelationshipBuilder
	extractor           *schema.FieldExtractor
}

// NewDNSPolicyHandler creates a new version-agnostic DNSPolicy handler
func NewDNSPolicyHandler(
	clientset *kubernetes.Clientset,
	dynamicClient dynamic.Interface,
	graphStore graph.GraphStore,
	logger *zap.Logger,
	factory dynamicinformer.DynamicSharedInformerFactory,
	extractor *schema.FieldExtractor,
) *DNSPolicyHandler {
	gvr := k8sschema.GroupVersionResource{
		Group:    "kuadrant.io",
		Version:  extractor.GetVersion(),
		Resource: "dnspolicies",
	}
	informer := factory.ForResource(gvr).Informer()

	handler := &DNSPolicyHandler{
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

// HandleAdd processes a newly added DNSPolicy
func (h *DNSPolicyHandler) HandleAdd(obj interface{}) {
	policy, err := watchers.ConvertToTyped[unstructured.Unstructured](obj)
	if err != nil {
		h.Logger.Error("failed to convert to Unstructured", zap.Error(err))
		return
	}

	h.Logger.Debug("dnspolicy added",
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

	// Create DNSPolicy node
	policyNode := &models.GraphNode{
		Type: NodeTypeDNSPolicy,
		ID:   models.GetNodeID(NodeTypeDNSPolicy, policy.GetNamespace(), policy.GetName()),
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

	// Extract loadBalancing (optional)
	if h.extractor.HasField("loadBalancing") {
		if lb, found, _ := h.extractor.ExtractMap(policy, "loadBalancing"); found && len(lb) > 0 {
			policyNode.Properties["has_load_balancing"] = true
		}
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

	if labels := policy.GetLabels(); len(labels) > 0 {
		policyNode.Properties["labels"] = serializeMap(labels)
	}

	// Store complete spec and status
	storeCompleteResourceSpec(policy, policyNode.Properties)

	if err := h.GraphStore.UpsertNode(ctx, string(policyNode.Type), policyNode.ID, policyNode.Properties); err != nil {
		h.Logger.Error("failed to create dnspolicy node", zap.Error(err))
		return
	}

	// Create IN_NAMESPACE edge
	if err := h.relationshipBuilder.base.CreateNamespaceEdge(ctx, NodeTypeDNSPolicy, policyNode.ID, policy.GetNamespace()); err != nil {
		h.Logger.Error("failed to create namespace edge", zap.Error(err))
	}

	// Create APPLIES_TO edge (typically to Gateway)
	if err := h.relationshipBuilder.CreatePolicyAppliesToEdge(
		ctx,
		NodeTypeDNSPolicy,
		policy.GetNamespace(),
		policy.GetName(),
		targetGroup,
		targetKind,
		policy.GetNamespace(),
		targetName,
	); err != nil {
		h.Logger.Error("failed to create APPLIES_TO edge", zap.Error(err))
	}

	// Create USES_SECRET edges to DNS provider credentials
	if h.extractor.HasField("providerRefs") {
		if providerRefs, found, _ := h.extractor.ExtractSlice(policy, "providerRefs"); found {
			for _, ref := range providerRefs {
				if refMap, ok := ref.(map[string]interface{}); ok {
					if secretName, ok := refMap["name"].(string); ok {
						// DNS provider secrets are in the same namespace as the policy
						secretID := models.GetNodeID(core.NodeTypeSecret, policy.GetNamespace(), secretName)

						if err := h.GraphStore.UpsertEdge(
							ctx,
							string(NodeTypeDNSPolicy),
							policyNode.ID,
							string(models.EdgeTypeUsesSecret),
							string(core.NodeTypeSecret),
							secretID,
							map[string]interface{}{
								"purpose": "dns_provider_credentials",
							},
						); err != nil {
							h.Logger.Error("failed to create USES_SECRET edge",
								zap.Error(err),
								zap.String("secret", secretName),
							)
						}
					}
				}
			}
		}
	}
}

// HandleUpdate processes an updated DNSPolicy
func (h *DNSPolicyHandler) HandleUpdate(oldObj, newObj interface{}) {
	policy, err := watchers.ConvertToTyped[unstructured.Unstructured](newObj)
	if err != nil {
		h.Logger.Error("failed to convert to Unstructured", zap.Error(err))
		return
	}

	h.Logger.Debug("dnspolicy updated",
		zap.String("namespace", policy.GetNamespace()),
		zap.String("name", policy.GetName()),
	)

	ctx := context.Background()
	policyID := models.GetNodeID(NodeTypeDNSPolicy, policy.GetNamespace(), policy.GetName())

	if err := h.GraphStore.DeleteEdgesByNode(ctx, string(NodeTypeDNSPolicy), policyID); err != nil {
		h.Logger.Error("failed to delete old edges", zap.Error(err))
	}

	h.HandleAdd(newObj)
}

// HandleDelete processes a deleted DNSPolicy
func (h *DNSPolicyHandler) HandleDelete(obj interface{}) {
	policy, err := watchers.ConvertToTyped[unstructured.Unstructured](obj)
	if err != nil {
		h.Logger.Error("failed to convert to Unstructured", zap.Error(err))
		return
	}

	h.Logger.Debug("dnspolicy deleted",
		zap.String("namespace", policy.GetNamespace()),
		zap.String("name", policy.GetName()),
	)

	ctx := context.Background()
	policyID := models.GetNodeID(NodeTypeDNSPolicy, policy.GetNamespace(), policy.GetName())

	if err := h.GraphStore.DeleteNode(ctx, string(NodeTypeDNSPolicy), policyID); err != nil {
		h.Logger.Error("failed to delete dnspolicy node", zap.Error(err))
	}
}
