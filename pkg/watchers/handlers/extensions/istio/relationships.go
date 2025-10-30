package istio

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/kagenti/kkbase/pkg/graph"
	"github.com/kagenti/kkbase/pkg/models"
	"github.com/kagenti/kkbase/pkg/watchers/handlers/core"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
)

// RelationshipBuilder helps build relationships for Istio resources
type RelationshipBuilder struct {
	base *core.RelationshipBuilder
}

// NewRelationshipBuilder creates a new istio relationship builder
func NewRelationshipBuilder(clientset *kubernetes.Clientset, graphStore graph.GraphStore, logger *zap.Logger) *RelationshipBuilder {
	return &RelationshipBuilder{
		base: core.NewRelationshipBuilder(clientset, graphStore, logger),
	}
}

// CreateIstioGatewaySelectsProxyEdge creates edges from an Istio Gateway to proxy Pods based on selector
func (rb *RelationshipBuilder) CreateIstioGatewaySelectsProxyEdge(ctx context.Context, gatewayNamespace, gatewayName string, selector map[string]string) error {
	if rb.base.Clientset == nil {
		rb.base.Logger.Warn("clientset is nil, skipping pod resolution")
		return nil
	}

	if len(selector) == 0 {
		rb.base.Logger.Debug("empty selector, skipping")
		return nil
	}

	// List all pods matching the selector
	labelSelector := labels.SelectorFromSet(selector)
	pods, err := rb.base.Clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector.String(),
	})
	if err != nil {
		return fmt.Errorf("failed to list pods with selector %v: %w", selector, err)
	}

	gatewayID := models.GetNodeID("IstioGateway", gatewayNamespace, gatewayName)
	selectorLabelsJSON, _ := json.Marshal(selector)

	for _, pod := range pods.Items {
		podID := models.GetNodeID("Pod", pod.Namespace, pod.Name)
		if err := rb.base.GraphStore.UpsertEdge(
			ctx,
			string(NodeTypeIstioGateway),
			gatewayID,
			string(EdgeTypeSelectsProxy),
			string(core.NodeTypePod),
			podID,
			map[string]interface{}{
				"selector_labels": string(selectorLabelsJSON),
			},
		); err != nil {
			rb.base.Logger.Error("failed to create SELECTS_PROXY edge",
				zap.Error(err),
				zap.String("gateway", gatewayName),
				zap.String("pod", pod.Name),
			)
		}
	}

	return nil
}

// CreateVirtualServiceAttachesToEdge creates an edge from a VirtualService to an Istio Gateway
func (rb *RelationshipBuilder) CreateVirtualServiceAttachesToEdge(ctx context.Context, vsNamespace, vsName, gatewayNamespace, gatewayName string) error {
	vsID := models.GetNodeID("VirtualService", vsNamespace, vsName)
	gatewayID := models.GetNodeID("IstioGateway", gatewayNamespace, gatewayName)

	return rb.base.GraphStore.UpsertEdge(
		ctx,
		string(NodeTypeVirtualService),
		vsID,
		string(models.EdgeTypeAttachesTo),
		string(NodeTypeIstioGateway),
		gatewayID,
		map[string]interface{}{
			"gateway_ref": gatewayName,
		},
	)
}

// CreateVirtualServiceRoutesTrafficForEdge creates an edge from a VirtualService to a Service
func (rb *RelationshipBuilder) CreateVirtualServiceRoutesTrafficForEdge(ctx context.Context, vsNamespace, vsName, svcNamespace, svcName, host string) error {
	vsID := models.GetNodeID("VirtualService", vsNamespace, vsName)
	svcID := models.GetNodeID("Service", svcNamespace, svcName)

	return rb.base.GraphStore.UpsertEdge(
		ctx,
		string(NodeTypeVirtualService),
		vsID,
		string(EdgeTypeRoutesTrafficFor),
		string(core.NodeTypeService),
		svcID,
		map[string]interface{}{
			"host": host,
		},
	)
}

// CreateVirtualServiceRoutesToSubsetEdge creates an edge from a VirtualService to a DestinationRule
func (rb *RelationshipBuilder) CreateVirtualServiceRoutesToSubsetEdge(ctx context.Context, vsNamespace, vsName, drNamespace, drName, subsetName string, weight int32) error {
	vsID := models.GetNodeID("VirtualService", vsNamespace, vsName)
	drID := models.GetNodeID("DestinationRule", drNamespace, drName)

	properties := map[string]interface{}{
		"subset_name": subsetName,
	}
	if weight > 0 {
		properties["weight"] = weight
	}

	return rb.base.GraphStore.UpsertEdge(
		ctx,
		string(NodeTypeVirtualService),
		vsID,
		string(EdgeTypeRoutesToSubset),
		string(NodeTypeDestinationRule),
		drID,
		properties,
	)
}

// CreateDestinationRuleDefinesPolicyForEdge creates an edge from a DestinationRule to a Service
func (rb *RelationshipBuilder) CreateDestinationRuleDefinesPolicyForEdge(ctx context.Context, drNamespace, drName, svcNamespace, svcName, host string) error {
	drID := models.GetNodeID("DestinationRule", drNamespace, drName)
	svcID := models.GetNodeID("Service", svcNamespace, svcName)

	return rb.base.GraphStore.UpsertEdge(
		ctx,
		string(NodeTypeDestinationRule),
		drID,
		string(EdgeTypeDefinesPolicyFor),
		string(core.NodeTypeService),
		svcID,
		map[string]interface{}{
			"host": host,
		},
	)
}

// CreateDestinationRuleSelectsSubsetPodsEdge creates edges from a DestinationRule to Pods based on subset labels
func (rb *RelationshipBuilder) CreateDestinationRuleSelectsSubsetPodsEdge(ctx context.Context, drNamespace, drName, subsetName string, subsetLabels map[string]string, targetNamespace string) error {
	if rb.base.Clientset == nil {
		rb.base.Logger.Warn("clientset is nil, skipping pod resolution")
		return nil
	}

	if len(subsetLabels) == 0 {
		rb.base.Logger.Debug("empty subset labels, skipping")
		return nil
	}

	// List all pods matching the subset labels in the target namespace
	labelSelector := labels.SelectorFromSet(subsetLabels)
	pods, err := rb.base.Clientset.CoreV1().Pods(targetNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector.String(),
	})
	if err != nil {
		return fmt.Errorf("failed to list pods with selector %v: %w", subsetLabels, err)
	}

	drID := models.GetNodeID("DestinationRule", drNamespace, drName)
	subsetLabelsJSON, _ := json.Marshal(subsetLabels)

	for _, pod := range pods.Items {
		podID := models.GetNodeID("Pod", pod.Namespace, pod.Name)
		if err := rb.base.GraphStore.UpsertEdge(
			ctx,
			string(NodeTypeDestinationRule),
			drID,
			string(EdgeTypeSelectsSubsetPods),
			string(core.NodeTypePod),
			podID,
			map[string]interface{}{
				"subset_name":   subsetName,
				"subset_labels": string(subsetLabelsJSON),
			},
		); err != nil {
			rb.base.Logger.Error("failed to create SELECTS_SUBSET_PODS edge",
				zap.Error(err),
				zap.String("destination_rule", drName),
				zap.String("subset", subsetName),
				zap.String("pod", pod.Name),
			)
		}
	}

	return nil
}

// CreateIstioPolicyAppliesToEdge creates edges from an Istio security policy to Pods based on selector
func (rb *RelationshipBuilder) CreateIstioPolicyAppliesToEdge(ctx context.Context, policyType models.NodeType, policyNamespace, policyName string, selector map[string]string, additionalProps map[string]interface{}) error {
	if rb.base.Clientset == nil {
		rb.base.Logger.Warn("clientset is nil, skipping pod resolution")
		return nil
	}

	// If selector is empty, the policy applies to all pods in the namespace
	var pods *corev1.PodList
	var err error

	if len(selector) == 0 {
		// Empty selector means all pods in namespace
		pods, err = rb.base.Clientset.CoreV1().Pods(policyNamespace).List(ctx, metav1.ListOptions{})
	} else {
		// List pods matching the selector
		labelSelector := labels.SelectorFromSet(selector)
		pods, err = rb.base.Clientset.CoreV1().Pods(policyNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: labelSelector.String(),
		})
	}

	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}

	policyID := models.GetNodeID(string(policyType), policyNamespace, policyName)

	properties := make(map[string]interface{})
	if len(selector) > 0 {
		selectorLabelsJSON, _ := json.Marshal(selector)
		properties["selector_labels"] = string(selectorLabelsJSON)
	}
	// Merge additional properties
	for k, v := range additionalProps {
		properties[k] = v
	}

	for _, pod := range pods.Items {
		podID := models.GetNodeID("Pod", pod.Namespace, pod.Name)
		if err := rb.base.GraphStore.UpsertEdge(
			ctx,
			string(policyType),
			policyID,
			string(models.EdgeTypeAppliesTo),
			string(core.NodeTypePod),
			podID,
			properties,
		); err != nil {
			rb.base.Logger.Error("failed to create APPLIES_TO edge",
				zap.Error(err),
				zap.String("policy_type", string(policyType)),
				zap.String("policy", policyName),
				zap.String("pod", pod.Name),
			)
		}
	}

	return nil
}
