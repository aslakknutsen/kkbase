package core

import (
	"encoding/json"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	storagev1 "k8s.io/api/storage/v1"

	"github.com/aslakknutsen/kkbase/pkg/models"
	"github.com/aslakknutsen/kkbase/pkg/watchers/handlers/common"
)

// serializeMap converts a map to JSON string for Neo4j storage
func serializeMap(m map[string]string) string {
	if len(m) == 0 {
		return ""
	}
	b, err := json.Marshal(m)
	if err != nil {
		return ""
	}
	return string(b)
}

// NodeToGraphNode converts a Kubernetes Node to a graph node
func NodeToGraphNode(node *corev1.Node) *models.GraphNode {
	properties := map[string]interface{}{
		"name":       node.Name,
		"status":     getNodeStatus(node),
		"created_at": node.CreationTimestamp.Unix(),
	}

	// Extract conditions as booleans with status_ prefix, messages and reasons
	if len(node.Status.Conditions) > 0 {
		for _, condition := range node.Status.Conditions {
			switch condition.Type {
			case corev1.NodeReady:
				properties["status_ready"] = (condition.Status == corev1.ConditionTrue)
				if condition.Message != "" {
					properties["status_ready_message"] = condition.Message
				}
				if condition.Reason != "" {
					properties["status_ready_reason"] = condition.Reason
				}
			case corev1.NodeMemoryPressure:
				properties["status_memory_pressure"] = (condition.Status == corev1.ConditionTrue)
				if condition.Message != "" {
					properties["status_memory_pressure_message"] = condition.Message
				}
				if condition.Reason != "" {
					properties["status_memory_pressure_reason"] = condition.Reason
				}
			case corev1.NodeDiskPressure:
				properties["status_disk_pressure"] = (condition.Status == corev1.ConditionTrue)
				if condition.Message != "" {
					properties["status_disk_pressure_message"] = condition.Message
				}
				if condition.Reason != "" {
					properties["status_disk_pressure_reason"] = condition.Reason
				}
			case corev1.NodePIDPressure:
				properties["status_pid_pressure"] = (condition.Status == corev1.ConditionTrue)
				if condition.Message != "" {
					properties["status_pid_pressure_message"] = condition.Message
				}
				if condition.Reason != "" {
					properties["status_pid_pressure_reason"] = condition.Reason
				}
			case corev1.NodeNetworkUnavailable:
				properties["status_network_unavailable"] = (condition.Status == corev1.ConditionTrue)
				if condition.Message != "" {
					properties["status_network_unavailable_message"] = condition.Message
				}
				if condition.Reason != "" {
					properties["status_network_unavailable_reason"] = condition.Reason
				}
			}
		}
	}

	// Add capacity
	if node.Status.Capacity != nil {
		properties["cpu_capacity"] = node.Status.Capacity.Cpu().String()
		properties["memory_capacity"] = node.Status.Capacity.Memory().String()
	}

	// Add addresses
	for _, addr := range node.Status.Addresses {
		if addr.Type == corev1.NodeInternalIP {
			properties["internal_ip"] = addr.Address
		} else if addr.Type == corev1.NodeExternalIP {
			properties["external_ip"] = addr.Address
		}
	}

	// Add labels as JSON string
	if len(node.Labels) > 0 {
		properties["labels"] = serializeMap(node.Labels)
	}

	// Add annotations
	if annotations := common.SerializeAnnotations(node.Annotations); annotations != "" {
		properties["annotations"] = annotations
	}

	return models.NewGraphNode(models.NodeType(NodeTypeNode), models.GetNodeID(NodeTypeNode, "", node.Name), properties)
}

// PodToGraphNode converts a Kubernetes Pod to a graph node
func PodToGraphNode(pod *corev1.Pod) *models.GraphNode {
	properties := map[string]interface{}{
		"name":       pod.Name,
		"namespace":  pod.Namespace,
		"status":     string(pod.Status.Phase),
		"node_name":  pod.Spec.NodeName,
		"created_at": pod.CreationTimestamp.Unix(),
	}

	if pod.Status.PodIP != "" {
		properties["ip"] = pod.Status.PodIP
	}

	if pod.Status.HostIP != "" {
		properties["host_ip"] = pod.Status.HostIP
	}

	// Extract pod conditions as booleans with messages and reasons
	if len(pod.Status.Conditions) > 0 {
		for _, condition := range pod.Status.Conditions {
			switch condition.Type {
			case corev1.PodScheduled:
				properties["status_pod_scheduled"] = (condition.Status == corev1.ConditionTrue)
				if condition.Message != "" {
					properties["status_pod_scheduled_message"] = condition.Message
				}
				if condition.Reason != "" {
					properties["status_pod_scheduled_reason"] = condition.Reason
				}
			case corev1.ContainersReady:
				properties["status_containers_ready"] = (condition.Status == corev1.ConditionTrue)
				if condition.Message != "" {
					properties["status_containers_ready_message"] = condition.Message
				}
				if condition.Reason != "" {
					properties["status_containers_ready_reason"] = condition.Reason
				}
			case corev1.PodInitialized:
				properties["status_initialized"] = (condition.Status == corev1.ConditionTrue)
				if condition.Message != "" {
					properties["status_initialized_message"] = condition.Message
				}
				if condition.Reason != "" {
					properties["status_initialized_reason"] = condition.Reason
				}
			case corev1.PodReady:
				properties["status_ready"] = (condition.Status == corev1.ConditionTrue)
				if condition.Message != "" {
					properties["status_ready_message"] = condition.Message
				}
				if condition.Reason != "" {
					properties["status_ready_reason"] = condition.Reason
				}
			}
		}
	}

	// Add resource requests and limits
	var totalCPURequest, totalMemoryRequest int64
	var totalCPULimit, totalMemoryLimit int64
	for _, container := range pod.Spec.Containers {
		if container.Resources.Requests != nil {
			totalCPURequest += container.Resources.Requests.Cpu().MilliValue()
			totalMemoryRequest += container.Resources.Requests.Memory().Value()
		}
		if container.Resources.Limits != nil {
			totalCPULimit += container.Resources.Limits.Cpu().MilliValue()
			totalMemoryLimit += container.Resources.Limits.Memory().Value()
		}
	}

	if totalCPURequest > 0 {
		properties["cpu_request"] = totalCPURequest
	}
	if totalMemoryRequest > 0 {
		properties["memory_request"] = totalMemoryRequest
	}
	if totalCPULimit > 0 {
		properties["cpu_limit"] = totalCPULimit
	}
	if totalMemoryLimit > 0 {
		properties["memory_limit"] = totalMemoryLimit
	}

	// Add labels
	if len(pod.Labels) > 0 {
		properties["labels"] = serializeMap(pod.Labels)
	}

	// Add annotations
	if annotations := common.SerializeAnnotations(pod.Annotations); annotations != "" {
		properties["annotations"] = annotations
	}

	// Add owner references
	if len(pod.OwnerReferences) > 0 {
		owners := make([]string, len(pod.OwnerReferences))
		for i, owner := range pod.OwnerReferences {
			owners[i] = fmt.Sprintf("%s/%s", owner.Kind, owner.Name)
		}
		properties["owners"] = owners
	}

	return models.NewGraphNode(models.NodeType(NodeTypePod), models.GetNodeID(NodeTypePod, pod.Namespace, pod.Name), properties)
}

// ContainerToGraphNode converts a container spec to a graph node
func ContainerToGraphNode(pod *corev1.Pod, container corev1.Container, containerStatus *corev1.ContainerStatus) *models.GraphNode {
	containerID := fmt.Sprintf("Container/%s/%s/%s", pod.Namespace, pod.Name, container.Name)

	properties := map[string]interface{}{
		"name":  container.Name,
		"image": container.Image,
	}

	if containerStatus != nil {
		properties["image_id"] = containerStatus.ImageID
		properties["ready"] = containerStatus.Ready
		properties["restart_count"] = containerStatus.RestartCount

		if containerStatus.State.Running != nil {
			properties["state"] = "Running"
		} else if containerStatus.State.Waiting != nil {
			properties["state"] = "Waiting"
			properties["reason"] = containerStatus.State.Waiting.Reason
		} else if containerStatus.State.Terminated != nil {
			properties["state"] = "Terminated"
			properties["exit_code"] = containerStatus.State.Terminated.ExitCode
			properties["reason"] = containerStatus.State.Terminated.Reason
		}
	}

	// Add ports
	if len(container.Ports) > 0 {
		ports := make([]int32, len(container.Ports))
		for i, port := range container.Ports {
			ports[i] = port.ContainerPort
		}
		properties["ports"] = ports
	}

	return models.NewGraphNode(models.NodeTypeContainer, containerID, properties)
}

// DeploymentToGraphNode converts a Kubernetes Deployment to a graph node
func DeploymentToGraphNode(deployment *appsv1.Deployment) *models.GraphNode {
	properties := map[string]interface{}{
		"name":               deployment.Name,
		"namespace":          deployment.Namespace,
		"desired_replicas":   *deployment.Spec.Replicas,
		"available_replicas": deployment.Status.AvailableReplicas,
		"ready_replicas":     deployment.Status.ReadyReplicas,
		"updated_replicas":   deployment.Status.UpdatedReplicas,
		"created_at":         deployment.CreationTimestamp.Unix(),
	}

	if deployment.Spec.Strategy.Type != "" {
		properties["strategy"] = string(deployment.Spec.Strategy.Type)
	}

	// Extract observedGeneration for staleness detection
	properties["observed_generation"] = deployment.Status.ObservedGeneration
	if deployment.Generation != deployment.Status.ObservedGeneration {
		properties["status_stale"] = true
	}

	// Extract status conditions as booleans with messages and reasons
	if len(deployment.Status.Conditions) > 0 {
		for _, condition := range deployment.Status.Conditions {
			switch condition.Type {
			case appsv1.DeploymentAvailable:
				properties["status_available"] = (condition.Status == "True")
				if condition.Message != "" {
					properties["status_available_message"] = condition.Message
				}
				if condition.Reason != "" {
					properties["status_available_reason"] = condition.Reason
				}
			case appsv1.DeploymentProgressing:
				properties["status_progressing"] = (condition.Status == "True")
				if condition.Message != "" {
					properties["status_progressing_message"] = condition.Message
				}
				if condition.Reason != "" {
					properties["status_progressing_reason"] = condition.Reason
				}
			case appsv1.DeploymentReplicaFailure:
				properties["status_replica_failure"] = (condition.Status == "True")
				if condition.Message != "" {
					properties["status_replica_failure_message"] = condition.Message
				}
				if condition.Reason != "" {
					properties["status_replica_failure_reason"] = condition.Reason
				}
			}
		}
	}

	// Add labels and selectors
	if len(deployment.Labels) > 0 {
		properties["labels"] = serializeMap(deployment.Labels)
	}
	if deployment.Spec.Selector != nil {
		properties["selector"] = serializeMap(deployment.Spec.Selector.MatchLabels)
	}

	// Add annotations
	if annotations := common.SerializeAnnotations(deployment.Annotations); annotations != "" {
		properties["annotations"] = annotations
	}

	return models.NewGraphNode(models.NodeType(NodeTypeDeployment), models.GetNodeID(NodeTypeDeployment, deployment.Namespace, deployment.Name), properties)
}

// ReplicaSetToGraphNode converts a Kubernetes ReplicaSet to a graph node
func ReplicaSetToGraphNode(replicaSet *appsv1.ReplicaSet) *models.GraphNode {
	properties := map[string]interface{}{
		"name":             replicaSet.Name,
		"namespace":        replicaSet.Namespace,
		"desired_replicas": *replicaSet.Spec.Replicas,
		"current_replicas": replicaSet.Status.Replicas,
		"ready_replicas":   replicaSet.Status.ReadyReplicas,
		"created_at":       replicaSet.CreationTimestamp.Unix(),
	}

	// Extract observedGeneration for staleness detection
	properties["observed_generation"] = replicaSet.Status.ObservedGeneration
	if replicaSet.Generation != replicaSet.Status.ObservedGeneration {
		properties["status_stale"] = true
	}

	// Extract status conditions as booleans with messages and reasons
	if len(replicaSet.Status.Conditions) > 0 {
		for _, condition := range replicaSet.Status.Conditions {
			switch condition.Type {
			case appsv1.ReplicaSetReplicaFailure:
				properties["status_replica_failure"] = (condition.Status == "True")
				if condition.Message != "" {
					properties["status_replica_failure_message"] = condition.Message
				}
				if condition.Reason != "" {
					properties["status_replica_failure_reason"] = condition.Reason
				}
			}
		}
	}

	// Add labels and selectors
	if len(replicaSet.Labels) > 0 {
		properties["labels"] = serializeMap(replicaSet.Labels)
	}
	if replicaSet.Spec.Selector != nil {
		properties["selector"] = serializeMap(replicaSet.Spec.Selector.MatchLabels)
	}

	// Add annotations
	if annotations := common.SerializeAnnotations(replicaSet.Annotations); annotations != "" {
		properties["annotations"] = annotations
	}

	// Add owner references
	if len(replicaSet.OwnerReferences) > 0 {
		owners := make([]string, len(replicaSet.OwnerReferences))
		for i, owner := range replicaSet.OwnerReferences {
			owners[i] = fmt.Sprintf("%s/%s", owner.Kind, owner.Name)
		}
		properties["owners"] = owners
	}

	return models.NewGraphNode(models.NodeType(NodeTypeReplicaSet), models.GetNodeID(NodeTypeReplicaSet, replicaSet.Namespace, replicaSet.Name), properties)
}

// StatefulSetToGraphNode converts a Kubernetes StatefulSet to a graph node
func StatefulSetToGraphNode(statefulSet *appsv1.StatefulSet) *models.GraphNode {
	properties := map[string]interface{}{
		"name":             statefulSet.Name,
		"namespace":        statefulSet.Namespace,
		"desired_replicas": *statefulSet.Spec.Replicas,
		"current_replicas": statefulSet.Status.Replicas,
		"ready_replicas":   statefulSet.Status.ReadyReplicas,
		"created_at":       statefulSet.CreationTimestamp.Unix(),
	}

	// Extract observedGeneration for staleness detection
	properties["observed_generation"] = statefulSet.Status.ObservedGeneration
	if statefulSet.Generation != statefulSet.Status.ObservedGeneration {
		properties["status_stale"] = true
	}

	// Extract status conditions as booleans with messages and reasons
	if len(statefulSet.Status.Conditions) > 0 {
		for _, condition := range statefulSet.Status.Conditions {
			// StatefulSet doesn't have standard condition type constants
			if string(condition.Type) == "ReplicasReady" {
				properties["status_replicas_ready"] = (condition.Status == "True")
				if condition.Message != "" {
					properties["status_replicas_ready_message"] = condition.Message
				}
				if condition.Reason != "" {
					properties["status_replicas_ready_reason"] = condition.Reason
				}
			}
		}
	}

	// Add labels and selectors
	if len(statefulSet.Labels) > 0 {
		properties["labels"] = serializeMap(statefulSet.Labels)
	}
	if statefulSet.Spec.Selector != nil {
		properties["selector"] = serializeMap(statefulSet.Spec.Selector.MatchLabels)
	}

	// Add annotations
	if annotations := common.SerializeAnnotations(statefulSet.Annotations); annotations != "" {
		properties["annotations"] = annotations
	}

	return models.NewGraphNode(models.NodeType(NodeTypeStatefulSet), models.GetNodeID(NodeTypeStatefulSet, statefulSet.Namespace, statefulSet.Name), properties)
}

// DaemonSetToGraphNode converts a Kubernetes DaemonSet to a graph node
func DaemonSetToGraphNode(daemonSet *appsv1.DaemonSet) *models.GraphNode {
	properties := map[string]interface{}{
		"name":              daemonSet.Name,
		"namespace":         daemonSet.Namespace,
		"desired_scheduled": daemonSet.Status.DesiredNumberScheduled,
		"current_scheduled": daemonSet.Status.CurrentNumberScheduled,
		"number_ready":      daemonSet.Status.NumberReady,
		"number_available":  daemonSet.Status.NumberAvailable,
		"created_at":        daemonSet.CreationTimestamp.Unix(),
	}

	// Extract observedGeneration for staleness detection
	properties["observed_generation"] = daemonSet.Status.ObservedGeneration
	if daemonSet.Generation != daemonSet.Status.ObservedGeneration {
		properties["status_stale"] = true
	}

	// Extract status conditions as booleans with messages and reasons
	if len(daemonSet.Status.Conditions) > 0 {
		for _, condition := range daemonSet.Status.Conditions {
			// DaemonSet doesn't have standard condition type constants, check string
			if condition.Type == "Available" {
				properties["status_available"] = (condition.Status == "True")
				if condition.Message != "" {
					properties["status_available_message"] = condition.Message
				}
				if condition.Reason != "" {
					properties["status_available_reason"] = condition.Reason
				}
			}
		}
	}

	// Add labels and selectors
	if len(daemonSet.Labels) > 0 {
		properties["labels"] = serializeMap(daemonSet.Labels)
	}
	if daemonSet.Spec.Selector != nil {
		properties["selector"] = serializeMap(daemonSet.Spec.Selector.MatchLabels)
	}

	// Add annotations
	if annotations := common.SerializeAnnotations(daemonSet.Annotations); annotations != "" {
		properties["annotations"] = annotations
	}

	return models.NewGraphNode(models.NodeType(NodeTypeDaemonSet), models.GetNodeID(NodeTypeDaemonSet, daemonSet.Namespace, daemonSet.Name), properties)
}

// ServiceToGraphNode converts a Kubernetes Service to a graph node
func ServiceToGraphNode(service *corev1.Service) *models.GraphNode {
	properties := map[string]interface{}{
		"name":       service.Name,
		"namespace":  service.Namespace,
		"type":       string(service.Spec.Type),
		"created_at": service.CreationTimestamp.Unix(),
	}

	if service.Spec.ClusterIP != "" {
		properties["cluster_ip"] = service.Spec.ClusterIP
	}

	// Add ports as JSON string
	if len(service.Spec.Ports) > 0 {
		ports := make([]map[string]interface{}, len(service.Spec.Ports))
		for i, port := range service.Spec.Ports {
			ports[i] = map[string]interface{}{
				"name":        port.Name,
				"port":        port.Port,
				"target_port": port.TargetPort.String(),
				"protocol":    string(port.Protocol),
			}
		}
		if b, err := json.Marshal(ports); err == nil {
			properties["ports"] = string(b)
		}
	}

	// Add selector as JSON string
	if len(service.Spec.Selector) > 0 {
		properties["selector"] = serializeMap(service.Spec.Selector)
	}

	// Add labels
	if len(service.Labels) > 0 {
		properties["labels"] = serializeMap(service.Labels)
	}

	// Add annotations
	if annotations := common.SerializeAnnotations(service.Annotations); annotations != "" {
		properties["annotations"] = annotations
	}

	return models.NewGraphNode(models.NodeType(NodeTypeService), models.GetNodeID(NodeTypeService, service.Namespace, service.Name), properties)
}

// IngressToGraphNode converts a Kubernetes Ingress to a graph node
func IngressToGraphNode(ingress *networkingv1.Ingress) *models.GraphNode {
	properties := map[string]interface{}{
		"name":      ingress.Name,
		"namespace": ingress.Namespace,
	}

	// Add rules
	if len(ingress.Spec.Rules) > 0 {
		hosts := make([]string, len(ingress.Spec.Rules))
		for i, rule := range ingress.Spec.Rules {
			hosts[i] = rule.Host
		}
		properties["hosts"] = hosts
	}

	// Add labels
	if len(ingress.Labels) > 0 {
		properties["labels"] = serializeMap(ingress.Labels)
	}

	// Add annotations
	if annotations := common.SerializeAnnotations(ingress.Annotations); annotations != "" {
		properties["annotations"] = annotations
	}

	return models.NewGraphNode(models.NodeType(NodeTypeIngress), models.GetNodeID(NodeTypeIngress, ingress.Namespace, ingress.Name), properties)
}

// NetworkPolicyToGraphNode converts a Kubernetes NetworkPolicy to a graph node
func NetworkPolicyToGraphNode(networkPolicy *networkingv1.NetworkPolicy) *models.GraphNode {
	properties := map[string]interface{}{
		"name":      networkPolicy.Name,
		"namespace": networkPolicy.Namespace,
	}

	// Add policy types
	if len(networkPolicy.Spec.PolicyTypes) > 0 {
		policyTypes := make([]string, len(networkPolicy.Spec.PolicyTypes))
		for i, pt := range networkPolicy.Spec.PolicyTypes {
			policyTypes[i] = string(pt)
		}
		properties["policy_types"] = policyTypes
	}

	// Add pod selector as JSON string
	if networkPolicy.Spec.PodSelector.Size() > 0 {
		properties["pod_selector"] = serializeMap(networkPolicy.Spec.PodSelector.MatchLabels)
	}

	// Add labels
	if len(networkPolicy.Labels) > 0 {
		properties["labels"] = serializeMap(networkPolicy.Labels)
	}

	// Add annotations
	if annotations := common.SerializeAnnotations(networkPolicy.Annotations); annotations != "" {
		properties["annotations"] = annotations
	}

	return models.NewGraphNode(models.NodeType(NodeTypeNetworkPolicy), models.GetNodeID(NodeTypeNetworkPolicy, networkPolicy.Namespace, networkPolicy.Name), properties)
}

// PersistentVolumeToGraphNode converts a Kubernetes PersistentVolume to a graph node
func PersistentVolumeToGraphNode(pv *corev1.PersistentVolume) *models.GraphNode {
	properties := map[string]interface{}{
		"name":       pv.Name,
		"status":     string(pv.Status.Phase),
		"created_at": pv.CreationTimestamp.Unix(),
	}

	// Note: PersistentVolume doesn't have status.conditions in the API

	if pv.Spec.Capacity != nil {
		if storage := pv.Spec.Capacity.Storage(); storage != nil {
			properties["capacity"] = storage.String()
		}
	}

	// Add access modes
	if len(pv.Spec.AccessModes) > 0 {
		accessModes := make([]string, len(pv.Spec.AccessModes))
		for i, mode := range pv.Spec.AccessModes {
			accessModes[i] = string(mode)
		}
		properties["access_modes"] = accessModes
	}

	if pv.Spec.StorageClassName != "" {
		properties["storage_class"] = pv.Spec.StorageClassName
	}

	// Add labels
	if len(pv.Labels) > 0 {
		properties["labels"] = serializeMap(pv.Labels)
	}

	// Add annotations
	if annotations := common.SerializeAnnotations(pv.Annotations); annotations != "" {
		properties["annotations"] = annotations
	}

	return models.NewGraphNode(models.NodeType(NodeTypePersistentVolume), models.GetNodeID(NodeTypePersistentVolume, "", pv.Name), properties)
}

// PersistentVolumeClaimToGraphNode converts a Kubernetes PersistentVolumeClaim to a graph node
func PersistentVolumeClaimToGraphNode(pvc *corev1.PersistentVolumeClaim) *models.GraphNode {
	properties := map[string]interface{}{
		"name":       pvc.Name,
		"namespace":  pvc.Namespace,
		"status":     string(pvc.Status.Phase),
		"created_at": pvc.CreationTimestamp.Unix(),
	}

	// Extract PVC conditions as booleans with messages and reasons
	if len(pvc.Status.Conditions) > 0 {
		for _, condition := range pvc.Status.Conditions {
			switch condition.Type {
			case corev1.PersistentVolumeClaimResizing:
				properties["status_resizing"] = (condition.Status == corev1.ConditionTrue)
				if condition.Message != "" {
					properties["status_resizing_message"] = condition.Message
				}
				if condition.Reason != "" {
					properties["status_resizing_reason"] = condition.Reason
				}
			case corev1.PersistentVolumeClaimFileSystemResizePending:
				properties["status_filesystem_resize_pending"] = (condition.Status == corev1.ConditionTrue)
				if condition.Message != "" {
					properties["status_filesystem_resize_pending_message"] = condition.Message
				}
				if condition.Reason != "" {
					properties["status_filesystem_resize_pending_reason"] = condition.Reason
				}
			}
		}
	}

	if pvc.Spec.Resources.Requests != nil {
		if storage := pvc.Spec.Resources.Requests.Storage(); storage != nil {
			properties["requested_storage"] = storage.String()
		}
	}

	// Add access modes
	if len(pvc.Spec.AccessModes) > 0 {
		accessModes := make([]string, len(pvc.Spec.AccessModes))
		for i, mode := range pvc.Spec.AccessModes {
			accessModes[i] = string(mode)
		}
		properties["access_modes"] = accessModes
	}

	if pvc.Spec.StorageClassName != nil {
		properties["storage_class"] = *pvc.Spec.StorageClassName
	}

	if pvc.Spec.VolumeName != "" {
		properties["volume_name"] = pvc.Spec.VolumeName
	}

	// Add labels
	if len(pvc.Labels) > 0 {
		properties["labels"] = serializeMap(pvc.Labels)
	}

	// Add annotations
	if annotations := common.SerializeAnnotations(pvc.Annotations); annotations != "" {
		properties["annotations"] = annotations
	}

	return models.NewGraphNode(models.NodeType(NodeTypePersistentVolumeClaim), models.GetNodeID(NodeTypePersistentVolumeClaim, pvc.Namespace, pvc.Name), properties)
}

// StorageClassToGraphNode converts a Kubernetes StorageClass to a graph node
func StorageClassToGraphNode(sc *storagev1.StorageClass) *models.GraphNode {
	properties := map[string]interface{}{
		"name":        sc.Name,
		"provisioner": sc.Provisioner,
		"created_at":  sc.CreationTimestamp.Unix(),
	}

	if sc.VolumeBindingMode != nil {
		properties["binding_mode"] = string(*sc.VolumeBindingMode)
	}

	if sc.ReclaimPolicy != nil {
		properties["reclaim_policy"] = string(*sc.ReclaimPolicy)
	}

	// Add labels
	if len(sc.Labels) > 0 {
		properties["labels"] = serializeMap(sc.Labels)
	}

	// Add annotations
	if annotations := common.SerializeAnnotations(sc.Annotations); annotations != "" {
		properties["annotations"] = annotations
	}

	return models.NewGraphNode(models.NodeType(NodeTypeStorageClass), models.GetNodeID(NodeTypeStorageClass, "", sc.Name), properties)
}

// ConfigMapToGraphNode converts a Kubernetes ConfigMap to a graph node
func ConfigMapToGraphNode(cm *corev1.ConfigMap) *models.GraphNode {
	properties := map[string]interface{}{
		"name":       cm.Name,
		"namespace":  cm.Namespace,
		"created_at": cm.CreationTimestamp.Unix(),
	}

	if len(cm.Data) > 0 {
		dataKeys := make([]string, 0, len(cm.Data))
		for key := range cm.Data {
			dataKeys = append(dataKeys, key)
		}
		properties["data_keys"] = dataKeys
	}

	// Add labels
	if len(cm.Labels) > 0 {
		properties["labels"] = serializeMap(cm.Labels)
	}

	// Add annotations
	if annotations := common.SerializeAnnotations(cm.Annotations); annotations != "" {
		properties["annotations"] = annotations
	}

	return models.NewGraphNode(models.NodeType(NodeTypeConfigMap), models.GetNodeID(NodeTypeConfigMap, cm.Namespace, cm.Name), properties)
}

// SecretToGraphNode converts a Kubernetes Secret to a graph node
func SecretToGraphNode(secret *corev1.Secret) *models.GraphNode {
	properties := map[string]interface{}{
		"name":       secret.Name,
		"namespace":  secret.Namespace,
		"type":       string(secret.Type),
		"created_at": secret.CreationTimestamp.Unix(),
	}

	if len(secret.Data) > 0 {
		dataKeys := make([]string, 0, len(secret.Data))
		for key := range secret.Data {
			dataKeys = append(dataKeys, key)
		}
		properties["data_keys"] = dataKeys
	}

	// Add labels
	if len(secret.Labels) > 0 {
		properties["labels"] = serializeMap(secret.Labels)
	}

	// Add annotations
	if annotations := common.SerializeAnnotations(secret.Annotations); annotations != "" {
		properties["annotations"] = annotations
	}

	return models.NewGraphNode(models.NodeType(NodeTypeSecret), models.GetNodeID(NodeTypeSecret, secret.Namespace, secret.Name), properties)
}

// EventToGraphNode converts a Kubernetes Event to a graph node
func EventToGraphNode(event *corev1.Event) *models.GraphNode {
	eventID := fmt.Sprintf("Event/%s/%s/%s", event.Namespace, event.InvolvedObject.Name, event.Name)

	properties := map[string]interface{}{
		"name":       event.Name,
		"namespace":  event.Namespace,
		"reason":     event.Reason,
		"message":    event.Message,
		"type":       event.Type,
		"count":      event.Count,
		"created_at": event.CreationTimestamp.Unix(),
	}

	if !event.FirstTimestamp.IsZero() {
		properties["first_timestamp"] = event.FirstTimestamp.Unix()
	}
	if !event.LastTimestamp.IsZero() {
		properties["last_timestamp"] = event.LastTimestamp.Unix()
	}

	// Add involved object information
	properties["involved_object_kind"] = event.InvolvedObject.Kind
	properties["involved_object_name"] = event.InvolvedObject.Name
	if event.InvolvedObject.Namespace != "" {
		properties["involved_object_namespace"] = event.InvolvedObject.Namespace
	}

	return models.NewGraphNode(models.NodeType(NodeTypeK8sEvent), eventID, properties)
}

// NamespaceToGraphNode converts a Kubernetes Namespace to a graph node
func NamespaceToGraphNode(namespace *corev1.Namespace) *models.GraphNode {
	properties := map[string]interface{}{
		"name":       namespace.Name,
		"status":     string(namespace.Status.Phase),
		"created_at": namespace.CreationTimestamp.Unix(),
	}

	// Extract Namespace conditions as booleans with messages and reasons (deletion-related conditions)
	if len(namespace.Status.Conditions) > 0 {
		for _, condition := range namespace.Status.Conditions {
			switch condition.Type {
			case corev1.NamespaceDeletionDiscoveryFailure:
				properties["status_deletion_discovery_failure"] = (condition.Status == corev1.ConditionTrue)
				if condition.Message != "" {
					properties["status_deletion_discovery_failure_message"] = condition.Message
				}
				if condition.Reason != "" {
					properties["status_deletion_discovery_failure_reason"] = condition.Reason
				}
			case corev1.NamespaceDeletionContentFailure:
				properties["status_deletion_content_failure"] = (condition.Status == corev1.ConditionTrue)
				if condition.Message != "" {
					properties["status_deletion_content_failure_message"] = condition.Message
				}
				if condition.Reason != "" {
					properties["status_deletion_content_failure_reason"] = condition.Reason
				}
			case corev1.NamespaceDeletionGVParsingFailure:
				properties["status_deletion_gv_parsing_failure"] = (condition.Status == corev1.ConditionTrue)
				if condition.Message != "" {
					properties["status_deletion_gv_parsing_failure_message"] = condition.Message
				}
				if condition.Reason != "" {
					properties["status_deletion_gv_parsing_failure_reason"] = condition.Reason
				}
			case corev1.NamespaceContentRemaining:
				properties["status_content_remaining"] = (condition.Status == corev1.ConditionTrue)
				if condition.Message != "" {
					properties["status_content_remaining_message"] = condition.Message
				}
				if condition.Reason != "" {
					properties["status_content_remaining_reason"] = condition.Reason
				}
			case corev1.NamespaceFinalizersRemaining:
				properties["status_finalizers_remaining"] = (condition.Status == corev1.ConditionTrue)
				if condition.Message != "" {
					properties["status_finalizers_remaining_message"] = condition.Message
				}
				if condition.Reason != "" {
					properties["status_finalizers_remaining_reason"] = condition.Reason
				}
			}
		}
	}

	// Add labels
	if len(namespace.Labels) > 0 {
		properties["labels"] = serializeMap(namespace.Labels)
	}

	// Add annotations
	if annotations := common.SerializeAnnotations(namespace.Annotations); annotations != "" {
		properties["annotations"] = annotations
	}

	return models.NewGraphNode(models.NodeType(NodeTypeNamespace), models.GetNodeID(NodeTypeNamespace, "", namespace.Name), properties)
}

// getNodeStatus is a helper function to get node status
func getNodeStatus(node *corev1.Node) string {
	for _, condition := range node.Status.Conditions {
		if condition.Type == corev1.NodeReady {
			if condition.Status == corev1.ConditionTrue {
				return "Ready"
			}
			return "NotReady"
		}
	}
	return "Unknown"
}
