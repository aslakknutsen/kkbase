package observability

import (
	"context"
	"fmt"

	"github.com/aslakknutsen/kkbase/pkg/graph"
	"github.com/aslakknutsen/kkbase/pkg/models"
	"go.uber.org/zap"
)

// MetricCorrelator matches metrics to K8s resources via labels
type MetricCorrelator struct {
	graphStore graph.GraphStore
	logger     *zap.Logger
}

// NewMetricCorrelator creates a new correlator
func NewMetricCorrelator(graphStore graph.GraphStore, logger *zap.Logger) *MetricCorrelator {
	return &MetricCorrelator{
		graphStore: graphStore,
		logger:     logger,
	}
}

// FindResourceFromLabels matches metric labels to resources in graph
// Returns (resourceType, resourceID) if found, or ("", "") if no match
func (mc *MetricCorrelator) FindResourceFromLabels(
	ctx context.Context,
	labels map[string]string,
) (string, string) {

	// Try different label combinations in priority order

	// 1. Container-level (most specific)
	if container, hasContainer := labels["container"]; hasContainer {
		if podName, hasPod := labels["pod"]; hasPod {
			if ns, hasNS := labels["namespace"]; hasNS && container != "" && container != "POD" {
				containerID := models.GetNodeID("Container", ns, fmt.Sprintf("%s/%s", podName, container))
				if mc.resourceExists(ctx, "Container", containerID) {
					mc.logger.Debug("correlated metric to container",
						zap.String("container_id", containerID),
						zap.String("pod", podName),
						zap.String("namespace", ns))
					return "Container", containerID
				}
			}
		}
	}

	// 2. Pod-level
	if podName, hasPod := labels["pod"]; hasPod {
		if ns, hasNS := labels["namespace"]; hasNS {
			podID := models.GetNodeID("Pod", ns, podName)
			if mc.resourceExists(ctx, "Pod", podID) {
				mc.logger.Debug("correlated metric to pod",
					zap.String("pod_id", podID),
					zap.String("pod", podName),
					zap.String("namespace", ns))
				return "Pod", podID
			}
		}
	}

	// 3. Service-level (common in application metrics)
	if svcName, hasSvc := labels["service"]; hasSvc {
		if ns, hasNS := labels["namespace"]; hasNS {
			svcID := models.GetNodeID("Service", ns, svcName)
			if mc.resourceExists(ctx, "Service", svcID) {
				mc.logger.Debug("correlated metric to service",
					zap.String("service_id", svcID),
					zap.String("service", svcName),
					zap.String("namespace", ns))
				return "Service", svcID
			}
		}
	}

	// 4. Node-level (using 'node' label)
	if nodeName, hasNode := labels["node"]; hasNode {
		nodeID := models.GetNodeID("Node", "", nodeName)
		if mc.resourceExists(ctx, "Node", nodeID) {
			mc.logger.Debug("correlated metric to node",
				zap.String("node_id", nodeID),
				zap.String("node", nodeName))
			return "Node", nodeID
		}
	}

	// 5. Alternative: 'instance' label (used by some node exporters)
	if instance, hasInstance := labels["instance"]; hasInstance {
		// Try to extract node name from instance (e.g., "node-1:9100" -> "node-1")
		nodeName := instance
		// Simple heuristic: remove port if present
		if idx := len(instance) - 1; idx > 0 {
			for i := len(instance) - 1; i >= 0; i-- {
				if instance[i] == ':' {
					nodeName = instance[:i]
					break
				}
			}
		}
		nodeID := models.GetNodeID("Node", "", nodeName)
		if mc.resourceExists(ctx, "Node", nodeID) {
			mc.logger.Debug("correlated metric to node via instance label",
				zap.String("node_id", nodeID),
				zap.String("instance", instance),
				zap.String("node", nodeName))
			return "Node", nodeID
		}
	}

	mc.logger.Debug("could not correlate metric to any resource",
		zap.Any("labels", labels))
	return "", ""
}

// resourceExists checks if a resource node exists in the graph
func (mc *MetricCorrelator) resourceExists(ctx context.Context, nodeType, nodeID string) bool {
	query := fmt.Sprintf(`
		MATCH (n:%s {id: $id})
		RETURN count(n) > 0 as exists
	`, nodeType)

	result, err := mc.graphStore.Query(ctx, query, map[string]interface{}{
		"id": nodeID,
	})

	if err != nil {
		mc.logger.Debug("resource existence check failed",
			zap.String("node_type", nodeType),
			zap.String("node_id", nodeID),
			zap.Error(err))
		return false
	}

	if len(result) > 0 {
		if exists, ok := result[0]["exists"].(bool); ok {
			return exists
		}
	}

	return false
}
