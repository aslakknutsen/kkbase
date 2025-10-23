package models

import (
	"encoding/json"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	istiov1 "istio.io/client-go/pkg/apis/networking/v1"
	istiosecurityv1 "istio.io/client-go/pkg/apis/security/v1"
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

// GetNodeID generates a unique identifier for a Kubernetes resource
// Format: kind/namespace/name for namespaced resources, kind/name for cluster-scoped
func GetNodeID(kind, namespace, name string) string {
	if namespace == "" {
		return fmt.Sprintf("%s/%s", kind, name)
	}
	return fmt.Sprintf("%s/%s/%s", kind, namespace, name)
}

// NodeToGraphNode converts a Kubernetes Node to a graph node
func NodeToGraphNode(node *corev1.Node) *GraphNode {
	properties := map[string]interface{}{
		"name":   node.Name,
		"status": getNodeStatus(node),
	}

	// Add conditions
	for _, condition := range node.Status.Conditions {
		properties[string(condition.Type)] = string(condition.Status)
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

	return NewGraphNode(NodeTypeNode, node.Name, properties)
}

// PodToGraphNode converts a Kubernetes Pod to a graph node
func PodToGraphNode(pod *corev1.Pod) *GraphNode {
	properties := map[string]interface{}{
		"name":      pod.Name,
		"namespace": pod.Namespace,
		"status":    string(pod.Status.Phase),
		"node_name": pod.Spec.NodeName,
	}

	if pod.Status.PodIP != "" {
		properties["ip"] = pod.Status.PodIP
	}

	if pod.Status.HostIP != "" {
		properties["host_ip"] = pod.Status.HostIP
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

	// Add owner references
	if len(pod.OwnerReferences) > 0 {
		owners := make([]string, len(pod.OwnerReferences))
		for i, owner := range pod.OwnerReferences {
			owners[i] = fmt.Sprintf("%s/%s", owner.Kind, owner.Name)
		}
		properties["owners"] = owners
	}

	return NewGraphNode(NodeTypePod, GetNodeID("Pod", pod.Namespace, pod.Name), properties)
}

// ContainerToGraphNode converts a container spec to a graph node
func ContainerToGraphNode(pod *corev1.Pod, container corev1.Container, containerStatus *corev1.ContainerStatus) *GraphNode {
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

	return NewGraphNode(NodeTypeContainer, containerID, properties)
}

// DeploymentToGraphNode converts a Kubernetes Deployment to a graph node
func DeploymentToGraphNode(deployment *appsv1.Deployment) *GraphNode {
	properties := map[string]interface{}{
		"name":               deployment.Name,
		"namespace":          deployment.Namespace,
		"desired_replicas":   *deployment.Spec.Replicas,
		"available_replicas": deployment.Status.AvailableReplicas,
		"ready_replicas":     deployment.Status.ReadyReplicas,
		"updated_replicas":   deployment.Status.UpdatedReplicas,
	}

	if deployment.Spec.Strategy.Type != "" {
		properties["strategy"] = string(deployment.Spec.Strategy.Type)
	}

	// Add labels and selectors
	if len(deployment.Labels) > 0 {
		properties["labels"] = serializeMap(deployment.Labels)
	}
	if deployment.Spec.Selector != nil {
		properties["selector"] = serializeMap(deployment.Spec.Selector.MatchLabels)
	}

	return NewGraphNode(NodeTypeDeployment, GetNodeID("Deployment", deployment.Namespace, deployment.Name), properties)
}

// ReplicaSetToGraphNode converts a Kubernetes ReplicaSet to a graph node
func ReplicaSetToGraphNode(replicaSet *appsv1.ReplicaSet) *GraphNode {
	properties := map[string]interface{}{
		"name":             replicaSet.Name,
		"namespace":        replicaSet.Namespace,
		"desired_replicas": *replicaSet.Spec.Replicas,
		"current_replicas": replicaSet.Status.Replicas,
		"ready_replicas":   replicaSet.Status.ReadyReplicas,
	}

	// Add labels and selectors
	if len(replicaSet.Labels) > 0 {
		properties["labels"] = serializeMap(replicaSet.Labels)
	}
	if replicaSet.Spec.Selector != nil {
		properties["selector"] = serializeMap(replicaSet.Spec.Selector.MatchLabels)
	}

	// Add owner references
	if len(replicaSet.OwnerReferences) > 0 {
		owners := make([]string, len(replicaSet.OwnerReferences))
		for i, owner := range replicaSet.OwnerReferences {
			owners[i] = fmt.Sprintf("%s/%s", owner.Kind, owner.Name)
		}
		properties["owners"] = owners
	}

	return NewGraphNode(NodeTypeReplicaSet, GetNodeID("ReplicaSet", replicaSet.Namespace, replicaSet.Name), properties)
}

// StatefulSetToGraphNode converts a Kubernetes StatefulSet to a graph node
func StatefulSetToGraphNode(statefulSet *appsv1.StatefulSet) *GraphNode {
	properties := map[string]interface{}{
		"name":             statefulSet.Name,
		"namespace":        statefulSet.Namespace,
		"desired_replicas": *statefulSet.Spec.Replicas,
		"current_replicas": statefulSet.Status.Replicas,
		"ready_replicas":   statefulSet.Status.ReadyReplicas,
	}

	// Add labels and selectors
	if len(statefulSet.Labels) > 0 {
		properties["labels"] = serializeMap(statefulSet.Labels)
	}
	if statefulSet.Spec.Selector != nil {
		properties["selector"] = serializeMap(statefulSet.Spec.Selector.MatchLabels)
	}

	return NewGraphNode(NodeTypeStatefulSet, GetNodeID("StatefulSet", statefulSet.Namespace, statefulSet.Name), properties)
}

// DaemonSetToGraphNode converts a Kubernetes DaemonSet to a graph node
func DaemonSetToGraphNode(daemonSet *appsv1.DaemonSet) *GraphNode {
	properties := map[string]interface{}{
		"name":              daemonSet.Name,
		"namespace":         daemonSet.Namespace,
		"desired_scheduled": daemonSet.Status.DesiredNumberScheduled,
		"current_scheduled": daemonSet.Status.CurrentNumberScheduled,
		"number_ready":      daemonSet.Status.NumberReady,
		"number_available":  daemonSet.Status.NumberAvailable,
	}

	// Add labels and selectors
	if len(daemonSet.Labels) > 0 {
		properties["labels"] = serializeMap(daemonSet.Labels)
	}
	if daemonSet.Spec.Selector != nil {
		properties["selector"] = serializeMap(daemonSet.Spec.Selector.MatchLabels)
	}

	return NewGraphNode(NodeTypeDaemonSet, GetNodeID("DaemonSet", daemonSet.Namespace, daemonSet.Name), properties)
}

// ServiceToGraphNode converts a Kubernetes Service to a graph node
func ServiceToGraphNode(service *corev1.Service) *GraphNode {
	properties := map[string]interface{}{
		"name":      service.Name,
		"namespace": service.Namespace,
		"type":      string(service.Spec.Type),
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

	return NewGraphNode(NodeTypeService, GetNodeID("Service", service.Namespace, service.Name), properties)
}

// IngressToGraphNode converts a Kubernetes Ingress to a graph node
func IngressToGraphNode(ingress *networkingv1.Ingress) *GraphNode {
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

	return NewGraphNode(NodeTypeIngress, GetNodeID("Ingress", ingress.Namespace, ingress.Name), properties)
}

// NetworkPolicyToGraphNode converts a Kubernetes NetworkPolicy to a graph node
func NetworkPolicyToGraphNode(networkPolicy *networkingv1.NetworkPolicy) *GraphNode {
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

	return NewGraphNode(NodeTypeNetworkPolicy, GetNodeID("NetworkPolicy", networkPolicy.Namespace, networkPolicy.Name), properties)
}

// PersistentVolumeToGraphNode converts a Kubernetes PersistentVolume to a graph node
func PersistentVolumeToGraphNode(pv *corev1.PersistentVolume) *GraphNode {
	properties := map[string]interface{}{
		"name":   pv.Name,
		"status": string(pv.Status.Phase),
	}

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

	return NewGraphNode(NodeTypePersistentVolume, pv.Name, properties)
}

// PersistentVolumeClaimToGraphNode converts a Kubernetes PersistentVolumeClaim to a graph node
func PersistentVolumeClaimToGraphNode(pvc *corev1.PersistentVolumeClaim) *GraphNode {
	properties := map[string]interface{}{
		"name":      pvc.Name,
		"namespace": pvc.Namespace,
		"status":    string(pvc.Status.Phase),
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

	return NewGraphNode(NodeTypePersistentVolumeClaim, GetNodeID("PersistentVolumeClaim", pvc.Namespace, pvc.Name), properties)
}

// StorageClassToGraphNode converts a Kubernetes StorageClass to a graph node
func StorageClassToGraphNode(sc *storagev1.StorageClass) *GraphNode {
	properties := map[string]interface{}{
		"name":        sc.Name,
		"provisioner": sc.Provisioner,
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

	return NewGraphNode(NodeTypeStorageClass, sc.Name, properties)
}

// ConfigMapToGraphNode converts a Kubernetes ConfigMap to a graph node
func ConfigMapToGraphNode(cm *corev1.ConfigMap) *GraphNode {
	properties := map[string]interface{}{
		"name":      cm.Name,
		"namespace": cm.Namespace,
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

	return NewGraphNode(NodeTypeConfigMap, GetNodeID("ConfigMap", cm.Namespace, cm.Name), properties)
}

// SecretToGraphNode converts a Kubernetes Secret to a graph node
func SecretToGraphNode(secret *corev1.Secret) *GraphNode {
	properties := map[string]interface{}{
		"name":      secret.Name,
		"namespace": secret.Namespace,
		"type":      string(secret.Type),
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

	return NewGraphNode(NodeTypeSecret, GetNodeID("Secret", secret.Namespace, secret.Name), properties)
}

// EventToGraphNode converts a Kubernetes Event to a graph node
func EventToGraphNode(event *corev1.Event) *GraphNode {
	eventID := fmt.Sprintf("Event/%s/%s/%s", event.Namespace, event.InvolvedObject.Name, event.Name)

	properties := map[string]interface{}{
		"name":      event.Name,
		"namespace": event.Namespace,
		"reason":    event.Reason,
		"message":   event.Message,
		"type":      event.Type,
		"count":     event.Count,
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

	return NewGraphNode(NodeTypeK8sEvent, eventID, properties)
}

// NamespaceToGraphNode converts a Kubernetes Namespace to a graph node
func NamespaceToGraphNode(namespace *corev1.Namespace) *GraphNode {
	properties := map[string]interface{}{
		"name":   namespace.Name,
		"status": string(namespace.Status.Phase),
	}

	// Add labels
	if len(namespace.Labels) > 0 {
		properties["labels"] = serializeMap(namespace.Labels)
	}

	return NewGraphNode(NodeTypeNamespace, namespace.Name, properties)
}

// Helper function to get node status
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

// GetOwnerReference extracts owner reference information
func GetOwnerReference(ownerRefs []metav1.OwnerReference) *metav1.OwnerReference {
	for i := range ownerRefs {
		if ownerRefs[i].Controller != nil && *ownerRefs[i].Controller {
			return &ownerRefs[i]
		}
	}
	if len(ownerRefs) > 0 {
		return &ownerRefs[0]
	}
	return nil
}

// Gateway API Converters

// GatewayClassToGraphNode converts a Gateway API GatewayClass to a graph node
func GatewayClassToGraphNode(gatewayClass *gatewayv1.GatewayClass) *GraphNode {
	properties := map[string]interface{}{
		"name":            gatewayClass.Name,
		"controller_name": string(gatewayClass.Spec.ControllerName),
	}

	if gatewayClass.Spec.Description != nil {
		properties["description"] = *gatewayClass.Spec.Description
	}

	// Add status conditions
	if len(gatewayClass.Status.Conditions) > 0 {
		for _, condition := range gatewayClass.Status.Conditions {
			if condition.Type == string(gatewayv1.GatewayClassConditionStatusAccepted) {
				properties["accepted"] = string(condition.Status)
				if condition.Message != "" {
					properties["status_message"] = condition.Message
				}
				break
			}
		}
	}

	// Add labels
	if len(gatewayClass.Labels) > 0 {
		properties["labels"] = serializeMap(gatewayClass.Labels)
	}

	return NewGraphNode(NodeTypeGatewayClass, gatewayClass.Name, properties)
}

// GatewayToGraphNode converts a Gateway API Gateway to a graph node
func GatewayToGraphNode(gateway *gatewayv1.Gateway) *GraphNode {
	properties := map[string]interface{}{
		"name":               gateway.Name,
		"namespace":          gateway.Namespace,
		"gateway_class_name": string(gateway.Spec.GatewayClassName),
	}

	// Add listeners information
	if len(gateway.Spec.Listeners) > 0 {
		listeners := make([]map[string]interface{}, len(gateway.Spec.Listeners))
		for i, listener := range gateway.Spec.Listeners {
			listenerInfo := map[string]interface{}{
				"name":     string(listener.Name),
				"port":     int(listener.Port),
				"protocol": string(listener.Protocol),
			}
			if listener.Hostname != nil {
				listenerInfo["hostname"] = string(*listener.Hostname)
			}
			if listener.TLS != nil {
				listenerInfo["tls_mode"] = string(*listener.TLS.Mode)
				if len(listener.TLS.CertificateRefs) > 0 {
					certRefs := make([]string, len(listener.TLS.CertificateRefs))
					for j, certRef := range listener.TLS.CertificateRefs {
						namespace := gateway.Namespace
						if certRef.Namespace != nil {
							namespace = string(*certRef.Namespace)
						}
						certRefs[j] = fmt.Sprintf("%s/%s", namespace, string(certRef.Name))
					}
					listenerInfo["certificate_refs"] = certRefs
				}
			}
			listeners[i] = listenerInfo
		}
		if b, err := json.Marshal(listeners); err == nil {
			properties["listeners"] = string(b)
		}
	}

	// Add addresses
	if len(gateway.Status.Addresses) > 0 {
		addresses := make([]string, len(gateway.Status.Addresses))
		for i, addr := range gateway.Status.Addresses {
			addresses[i] = addr.Value
		}
		properties["addresses"] = addresses
	}

	// Add status conditions
	if len(gateway.Status.Conditions) > 0 {
		for _, condition := range gateway.Status.Conditions {
			if condition.Type == string(gatewayv1.GatewayConditionAccepted) {
				properties["accepted"] = string(condition.Status)
			} else if condition.Type == string(gatewayv1.GatewayConditionProgrammed) {
				properties["programmed"] = string(condition.Status)
			}
		}
	}

	// Add labels
	if len(gateway.Labels) > 0 {
		properties["labels"] = serializeMap(gateway.Labels)
	}

	return NewGraphNode(NodeTypeGateway, GetNodeID("Gateway", gateway.Namespace, gateway.Name), properties)
}

// HTTPRouteToGraphNode converts a Gateway API HTTPRoute to a graph node
func HTTPRouteToGraphNode(httpRoute *gatewayv1.HTTPRoute) *GraphNode {
	properties := map[string]interface{}{
		"name":      httpRoute.Name,
		"namespace": httpRoute.Namespace,
	}

	// Add hostnames
	if len(httpRoute.Spec.Hostnames) > 0 {
		hostnames := make([]string, len(httpRoute.Spec.Hostnames))
		for i, hostname := range httpRoute.Spec.Hostnames {
			hostnames[i] = string(hostname)
		}
		properties["hostnames"] = hostnames
	}

	// Add parent refs (gateways this route attaches to)
	if len(httpRoute.Spec.ParentRefs) > 0 {
		parentRefs := make([]map[string]interface{}, len(httpRoute.Spec.ParentRefs))
		for i, parentRef := range httpRoute.Spec.ParentRefs {
			ref := map[string]interface{}{
				"name": string(parentRef.Name),
			}
			if parentRef.Namespace != nil {
				ref["namespace"] = string(*parentRef.Namespace)
			} else {
				ref["namespace"] = httpRoute.Namespace
			}
			if parentRef.SectionName != nil {
				ref["section_name"] = string(*parentRef.SectionName)
			}
			parentRefs[i] = ref
		}
		if b, err := json.Marshal(parentRefs); err == nil {
			properties["parent_refs"] = string(b)
		}
	}

	// Add rules summary
	if len(httpRoute.Spec.Rules) > 0 {
		properties["rule_count"] = len(httpRoute.Spec.Rules)

		// Serialize rules
		rules := make([]map[string]interface{}, len(httpRoute.Spec.Rules))
		for i, rule := range httpRoute.Spec.Rules {
			ruleInfo := map[string]interface{}{}

			// Add matches
			if len(rule.Matches) > 0 {
				matches := make([]map[string]interface{}, len(rule.Matches))
				for j, match := range rule.Matches {
					matchInfo := map[string]interface{}{}
					if match.Path != nil {
						matchInfo["path_type"] = string(*match.Path.Type)
						if match.Path.Value != nil {
							matchInfo["path_value"] = *match.Path.Value
						}
					}
					matches[j] = matchInfo
				}
				ruleInfo["matches"] = matches
			}

			// Add backend refs
			if len(rule.BackendRefs) > 0 {
				backends := make([]string, len(rule.BackendRefs))
				for j, backend := range rule.BackendRefs {
					namespace := httpRoute.Namespace
					if backend.Namespace != nil {
						namespace = string(*backend.Namespace)
					}
					backends[j] = fmt.Sprintf("%s/%s", namespace, string(backend.Name))
				}
				ruleInfo["backends"] = backends
			}

			rules[i] = ruleInfo
		}
		if b, err := json.Marshal(rules); err == nil {
			properties["rules"] = string(b)
		}
	}

	// Add status
	if len(httpRoute.Status.Parents) > 0 {
		for _, parent := range httpRoute.Status.Parents {
			for _, condition := range parent.Conditions {
				if condition.Type == string(gatewayv1.RouteConditionAccepted) {
					properties["accepted"] = string(condition.Status)
					break
				}
			}
		}
	}

	// Add labels
	if len(httpRoute.Labels) > 0 {
		properties["labels"] = serializeMap(httpRoute.Labels)
	}

	return NewGraphNode(NodeTypeHTTPRoute, GetNodeID("HTTPRoute", httpRoute.Namespace, httpRoute.Name), properties)
}

// GRPCRouteToGraphNode converts a Gateway API GRPCRoute to a graph node
func GRPCRouteToGraphNode(grpcRoute *gatewayv1.GRPCRoute) *GraphNode {
	properties := map[string]interface{}{
		"name":      grpcRoute.Name,
		"namespace": grpcRoute.Namespace,
	}

	// Add hostnames
	if len(grpcRoute.Spec.Hostnames) > 0 {
		hostnames := make([]string, len(grpcRoute.Spec.Hostnames))
		for i, hostname := range grpcRoute.Spec.Hostnames {
			hostnames[i] = string(hostname)
		}
		properties["hostnames"] = hostnames
	}

	// Add parent refs
	if len(grpcRoute.Spec.ParentRefs) > 0 {
		parentRefs := make([]map[string]interface{}, len(grpcRoute.Spec.ParentRefs))
		for i, parentRef := range grpcRoute.Spec.ParentRefs {
			ref := map[string]interface{}{
				"name": string(parentRef.Name),
			}
			if parentRef.Namespace != nil {
				ref["namespace"] = string(*parentRef.Namespace)
			} else {
				ref["namespace"] = grpcRoute.Namespace
			}
			parentRefs[i] = ref
		}
		if b, err := json.Marshal(parentRefs); err == nil {
			properties["parent_refs"] = string(b)
		}
	}

	// Add rules summary
	if len(grpcRoute.Spec.Rules) > 0 {
		properties["rule_count"] = len(grpcRoute.Spec.Rules)
	}

	// Add labels
	if len(grpcRoute.Labels) > 0 {
		properties["labels"] = serializeMap(grpcRoute.Labels)
	}

	return NewGraphNode(NodeTypeGRPCRoute, GetNodeID("GRPCRoute", grpcRoute.Namespace, grpcRoute.Name), properties)
}

// TCPRouteToGraphNode converts a Gateway API TCPRoute to a graph node
func TCPRouteToGraphNode(tcpRoute *gatewayv1alpha2.TCPRoute) *GraphNode {
	properties := map[string]interface{}{
		"name":      tcpRoute.Name,
		"namespace": tcpRoute.Namespace,
	}

	// Add parent refs
	if len(tcpRoute.Spec.ParentRefs) > 0 {
		parentRefs := make([]map[string]interface{}, len(tcpRoute.Spec.ParentRefs))
		for i, parentRef := range tcpRoute.Spec.ParentRefs {
			ref := map[string]interface{}{
				"name": string(parentRef.Name),
			}
			if parentRef.Namespace != nil {
				ref["namespace"] = string(*parentRef.Namespace)
			} else {
				ref["namespace"] = tcpRoute.Namespace
			}
			parentRefs[i] = ref
		}
		if b, err := json.Marshal(parentRefs); err == nil {
			properties["parent_refs"] = string(b)
		}
	}

	// Add rules summary
	if len(tcpRoute.Spec.Rules) > 0 {
		properties["rule_count"] = len(tcpRoute.Spec.Rules)
	}

	// Add labels
	if len(tcpRoute.Labels) > 0 {
		properties["labels"] = serializeMap(tcpRoute.Labels)
	}

	return NewGraphNode(NodeTypeTCPRoute, GetNodeID("TCPRoute", tcpRoute.Namespace, tcpRoute.Name), properties)
}

// UDPRouteToGraphNode converts a Gateway API UDPRoute to a graph node
func UDPRouteToGraphNode(udpRoute *gatewayv1alpha2.UDPRoute) *GraphNode {
	properties := map[string]interface{}{
		"name":      udpRoute.Name,
		"namespace": udpRoute.Namespace,
	}

	// Add parent refs
	if len(udpRoute.Spec.ParentRefs) > 0 {
		parentRefs := make([]map[string]interface{}, len(udpRoute.Spec.ParentRefs))
		for i, parentRef := range udpRoute.Spec.ParentRefs {
			ref := map[string]interface{}{
				"name": string(parentRef.Name),
			}
			if parentRef.Namespace != nil {
				ref["namespace"] = string(*parentRef.Namespace)
			} else {
				ref["namespace"] = udpRoute.Namespace
			}
			parentRefs[i] = ref
		}
		if b, err := json.Marshal(parentRefs); err == nil {
			properties["parent_refs"] = string(b)
		}
	}

	// Add rules summary
	if len(udpRoute.Spec.Rules) > 0 {
		properties["rule_count"] = len(udpRoute.Spec.Rules)
	}

	// Add labels
	if len(udpRoute.Labels) > 0 {
		properties["labels"] = serializeMap(udpRoute.Labels)
	}

	return NewGraphNode(NodeTypeUDPRoute, GetNodeID("UDPRoute", udpRoute.Namespace, udpRoute.Name), properties)
}

// TLSRouteToGraphNode converts a Gateway API TLSRoute to a graph node
func TLSRouteToGraphNode(tlsRoute *gatewayv1alpha2.TLSRoute) *GraphNode {
	properties := map[string]interface{}{
		"name":      tlsRoute.Name,
		"namespace": tlsRoute.Namespace,
	}

	// Add hostnames
	if len(tlsRoute.Spec.Hostnames) > 0 {
		hostnames := make([]string, len(tlsRoute.Spec.Hostnames))
		for i, hostname := range tlsRoute.Spec.Hostnames {
			hostnames[i] = string(hostname)
		}
		properties["hostnames"] = hostnames
	}

	// Add parent refs
	if len(tlsRoute.Spec.ParentRefs) > 0 {
		parentRefs := make([]map[string]interface{}, len(tlsRoute.Spec.ParentRefs))
		for i, parentRef := range tlsRoute.Spec.ParentRefs {
			ref := map[string]interface{}{
				"name": string(parentRef.Name),
			}
			if parentRef.Namespace != nil {
				ref["namespace"] = string(*parentRef.Namespace)
			} else {
				ref["namespace"] = tlsRoute.Namespace
			}
			parentRefs[i] = ref
		}
		if b, err := json.Marshal(parentRefs); err == nil {
			properties["parent_refs"] = string(b)
		}
	}

	// Add rules summary
	if len(tlsRoute.Spec.Rules) > 0 {
		properties["rule_count"] = len(tlsRoute.Spec.Rules)
	}

	// Add labels
	if len(tlsRoute.Labels) > 0 {
		properties["labels"] = serializeMap(tlsRoute.Labels)
	}

	return NewGraphNode(NodeTypeTLSRoute, GetNodeID("TLSRoute", tlsRoute.Namespace, tlsRoute.Name), properties)
}

// ReferenceGrantToGraphNode converts a Gateway API ReferenceGrant to a graph node
func ReferenceGrantToGraphNode(referenceGrant *gatewayv1beta1.ReferenceGrant) *GraphNode {
	properties := map[string]interface{}{
		"name":      referenceGrant.Name,
		"namespace": referenceGrant.Namespace,
	}

	// Add from references (who can reference)
	if len(referenceGrant.Spec.From) > 0 {
		froms := make([]map[string]interface{}, len(referenceGrant.Spec.From))
		for i, from := range referenceGrant.Spec.From {
			fromInfo := map[string]interface{}{
				"group":     string(from.Group),
				"kind":      string(from.Kind),
				"namespace": string(from.Namespace),
			}
			froms[i] = fromInfo
		}
		if b, err := json.Marshal(froms); err == nil {
			properties["from"] = string(b)
		}
	}

	// Add to references (what can be referenced)
	if len(referenceGrant.Spec.To) > 0 {
		tos := make([]map[string]interface{}, len(referenceGrant.Spec.To))
		for i, to := range referenceGrant.Spec.To {
			toInfo := map[string]interface{}{
				"group": string(to.Group),
				"kind":  string(to.Kind),
			}
			if to.Name != nil {
				toInfo["name"] = string(*to.Name)
			}
			tos[i] = toInfo
		}
		if b, err := json.Marshal(tos); err == nil {
			properties["to"] = string(b)
		}
	}

	// Add labels
	if len(referenceGrant.Labels) > 0 {
		properties["labels"] = serializeMap(referenceGrant.Labels)
	}

	return NewGraphNode(NodeTypeReferenceGrant, GetNodeID("ReferenceGrant", referenceGrant.Namespace, referenceGrant.Name), properties)
}

// =============== Istio Converters ===============

// IstioGatewayToGraphNode converts an Istio Gateway to a graph node
func IstioGatewayToGraphNode(gateway *istiov1.Gateway) *GraphNode {
	properties := map[string]interface{}{
		"name":      gateway.Name,
		"namespace": gateway.Namespace,
		"uid":       string(gateway.UID),
		"created":   gateway.CreationTimestamp.Format("2006-01-02T15:04:05Z"),
	}

	// Add labels
	if len(gateway.Labels) > 0 {
		properties["labels"] = serializeMap(gateway.Labels)
	}

	// Add annotations
	if len(gateway.Annotations) > 0 {
		properties["annotations"] = serializeMap(gateway.Annotations)
	}

	// Add selector
	if gateway.Spec.Selector != nil && len(gateway.Spec.Selector) > 0 {
		properties["selector"] = serializeMap(gateway.Spec.Selector)
	}

	// Add servers (serialized JSON)
	if len(gateway.Spec.Servers) > 0 {
		serversJSON, err := json.Marshal(gateway.Spec.Servers)
		if err == nil {
			properties["servers"] = string(serversJSON)
		}
	}

	return NewGraphNode(NodeTypeIstioGateway, GetNodeID("IstioGateway", gateway.Namespace, gateway.Name), properties)
}

// VirtualServiceToGraphNode converts an Istio VirtualService to a graph node
func VirtualServiceToGraphNode(vs *istiov1.VirtualService) *GraphNode {
	properties := map[string]interface{}{
		"name":      vs.Name,
		"namespace": vs.Namespace,
		"uid":       string(vs.UID),
		"created":   vs.CreationTimestamp.Format("2006-01-02T15:04:05Z"),
	}

	// Add labels
	if len(vs.Labels) > 0 {
		properties["labels"] = serializeMap(vs.Labels)
	}

	// Add annotations
	if len(vs.Annotations) > 0 {
		properties["annotations"] = serializeMap(vs.Annotations)
	}

	// Add hosts
	if len(vs.Spec.Hosts) > 0 {
		hostsJSON, err := json.Marshal(vs.Spec.Hosts)
		if err == nil {
			properties["hosts"] = string(hostsJSON)
		}
	}

	// Add gateways
	if len(vs.Spec.Gateways) > 0 {
		gatewaysJSON, err := json.Marshal(vs.Spec.Gateways)
		if err == nil {
			properties["gateways"] = string(gatewaysJSON)
		}
	}

	// Add HTTP routes (serialized JSON)
	if len(vs.Spec.Http) > 0 {
		httpRoutesJSON, err := json.Marshal(vs.Spec.Http)
		if err == nil {
			properties["http_routes"] = string(httpRoutesJSON)
		}
	}

	// Add TCP routes (serialized JSON)
	if len(vs.Spec.Tcp) > 0 {
		tcpRoutesJSON, err := json.Marshal(vs.Spec.Tcp)
		if err == nil {
			properties["tcp_routes"] = string(tcpRoutesJSON)
		}
	}

	// Add TLS routes (serialized JSON)
	if len(vs.Spec.Tls) > 0 {
		tlsRoutesJSON, err := json.Marshal(vs.Spec.Tls)
		if err == nil {
			properties["tls_routes"] = string(tlsRoutesJSON)
		}
	}

	return NewGraphNode(NodeTypeVirtualService, GetNodeID("VirtualService", vs.Namespace, vs.Name), properties)
}

// DestinationRuleToGraphNode converts an Istio DestinationRule to a graph node
func DestinationRuleToGraphNode(dr *istiov1.DestinationRule) *GraphNode {
	properties := map[string]interface{}{
		"name":      dr.Name,
		"namespace": dr.Namespace,
		"uid":       string(dr.UID),
		"created":   dr.CreationTimestamp.Format("2006-01-02T15:04:05Z"),
	}

	// Add labels
	if len(dr.Labels) > 0 {
		properties["labels"] = serializeMap(dr.Labels)
	}

	// Add annotations
	if len(dr.Annotations) > 0 {
		properties["annotations"] = serializeMap(dr.Annotations)
	}

	// Add host
	if dr.Spec.Host != "" {
		properties["host"] = dr.Spec.Host
	}

	// Add subsets (serialized JSON)
	if len(dr.Spec.Subsets) > 0 {
		subsetsJSON, err := json.Marshal(dr.Spec.Subsets)
		if err == nil {
			properties["subsets"] = string(subsetsJSON)
		}
	}

	// Add traffic policy (serialized JSON)
	if dr.Spec.TrafficPolicy != nil {
		trafficPolicyJSON, err := json.Marshal(dr.Spec.TrafficPolicy)
		if err == nil {
			properties["traffic_policy"] = string(trafficPolicyJSON)
		}
	}

	return NewGraphNode(NodeTypeDestinationRule, GetNodeID("DestinationRule", dr.Namespace, dr.Name), properties)
}

// ServiceEntryToGraphNode converts an Istio ServiceEntry to a graph node
func ServiceEntryToGraphNode(se *istiov1.ServiceEntry) *GraphNode {
	properties := map[string]interface{}{
		"name":      se.Name,
		"namespace": se.Namespace,
		"uid":       string(se.UID),
		"created":   se.CreationTimestamp.Format("2006-01-02T15:04:05Z"),
	}

	// Add labels
	if len(se.Labels) > 0 {
		properties["labels"] = serializeMap(se.Labels)
	}

	// Add annotations
	if len(se.Annotations) > 0 {
		properties["annotations"] = serializeMap(se.Annotations)
	}

	// Add hosts
	if len(se.Spec.Hosts) > 0 {
		hostsJSON, err := json.Marshal(se.Spec.Hosts)
		if err == nil {
			properties["hosts"] = string(hostsJSON)
		}
	}

	// Add location
	if se.Spec.Location.String() != "" {
		properties["location"] = se.Spec.Location.String()
	}

	// Add resolution
	if se.Spec.Resolution.String() != "" {
		properties["resolution"] = se.Spec.Resolution.String()
	}

	// Add endpoints (serialized JSON)
	if len(se.Spec.Endpoints) > 0 {
		endpointsJSON, err := json.Marshal(se.Spec.Endpoints)
		if err == nil {
			properties["endpoints"] = string(endpointsJSON)
		}
	}

	return NewGraphNode(NodeTypeServiceEntry, GetNodeID("ServiceEntry", se.Namespace, se.Name), properties)
}

// SidecarToGraphNode converts an Istio Sidecar to a graph node
func SidecarToGraphNode(sidecar *istiov1.Sidecar) *GraphNode {
	properties := map[string]interface{}{
		"name":      sidecar.Name,
		"namespace": sidecar.Namespace,
		"uid":       string(sidecar.UID),
		"created":   sidecar.CreationTimestamp.Format("2006-01-02T15:04:05Z"),
	}

	// Add labels
	if len(sidecar.Labels) > 0 {
		properties["labels"] = serializeMap(sidecar.Labels)
	}

	// Add annotations
	if len(sidecar.Annotations) > 0 {
		properties["annotations"] = serializeMap(sidecar.Annotations)
	}

	// Add workload selector
	if sidecar.Spec.WorkloadSelector != nil && len(sidecar.Spec.WorkloadSelector.Labels) > 0 {
		workloadSelectorJSON, err := json.Marshal(sidecar.Spec.WorkloadSelector)
		if err == nil {
			properties["workload_selector"] = string(workloadSelectorJSON)
		}
	}

	// Add egress hosts
	if len(sidecar.Spec.Egress) > 0 {
		var egressHosts []string
		for _, egress := range sidecar.Spec.Egress {
			egressHosts = append(egressHosts, egress.Hosts...)
		}
		if len(egressHosts) > 0 {
			egressHostsJSON, err := json.Marshal(egressHosts)
			if err == nil {
				properties["egress_hosts"] = string(egressHostsJSON)
			}
		}
	}

	// Add ingress listeners (serialized JSON)
	if len(sidecar.Spec.Ingress) > 0 {
		ingressJSON, err := json.Marshal(sidecar.Spec.Ingress)
		if err == nil {
			properties["ingress_listeners"] = string(ingressJSON)
		}
	}

	// Add egress listeners (serialized JSON)
	if len(sidecar.Spec.Egress) > 0 {
		egressJSON, err := json.Marshal(sidecar.Spec.Egress)
		if err == nil {
			properties["egress_listeners"] = string(egressJSON)
		}
	}

	return NewGraphNode(NodeTypeSidecar, GetNodeID("Sidecar", sidecar.Namespace, sidecar.Name), properties)
}

// AuthorizationPolicyToGraphNode converts an Istio AuthorizationPolicy to a graph node
func AuthorizationPolicyToGraphNode(policy *istiosecurityv1.AuthorizationPolicy) *GraphNode {
	properties := map[string]interface{}{
		"name":      policy.Name,
		"namespace": policy.Namespace,
		"uid":       string(policy.UID),
		"created":   policy.CreationTimestamp.Format("2006-01-02T15:04:05Z"),
	}

	// Add labels
	if len(policy.Labels) > 0 {
		properties["labels"] = serializeMap(policy.Labels)
	}

	// Add annotations
	if len(policy.Annotations) > 0 {
		properties["annotations"] = serializeMap(policy.Annotations)
	}

	// Add selector
	if policy.Spec.Selector != nil && len(policy.Spec.Selector.MatchLabels) > 0 {
		selectorJSON, err := json.Marshal(policy.Spec.Selector)
		if err == nil {
			properties["selector"] = string(selectorJSON)
		}
	}

	// Add action
	if policy.Spec.Action.String() != "" {
		properties["action"] = policy.Spec.Action.String()
	}

	// Add rules (serialized JSON)
	if len(policy.Spec.Rules) > 0 {
		rulesJSON, err := json.Marshal(policy.Spec.Rules)
		if err == nil {
			properties["rules"] = string(rulesJSON)
		}
	}

	return NewGraphNode(NodeTypeAuthorizationPolicy, GetNodeID("AuthorizationPolicy", policy.Namespace, policy.Name), properties)
}

// PeerAuthenticationToGraphNode converts an Istio PeerAuthentication to a graph node
func PeerAuthenticationToGraphNode(pa *istiosecurityv1.PeerAuthentication) *GraphNode {
	properties := map[string]interface{}{
		"name":      pa.Name,
		"namespace": pa.Namespace,
		"uid":       string(pa.UID),
		"created":   pa.CreationTimestamp.Format("2006-01-02T15:04:05Z"),
	}

	// Add labels
	if len(pa.Labels) > 0 {
		properties["labels"] = serializeMap(pa.Labels)
	}

	// Add annotations
	if len(pa.Annotations) > 0 {
		properties["annotations"] = serializeMap(pa.Annotations)
	}

	// Add selector
	if pa.Spec.Selector != nil && len(pa.Spec.Selector.MatchLabels) > 0 {
		selectorJSON, err := json.Marshal(pa.Spec.Selector)
		if err == nil {
			properties["selector"] = string(selectorJSON)
		}
	}

	// Add mtls mode
	if pa.Spec.Mtls != nil && pa.Spec.Mtls.Mode.String() != "" {
		properties["mtls_mode"] = pa.Spec.Mtls.Mode.String()
	}

	// Add port level mtls (serialized JSON)
	if pa.Spec.PortLevelMtls != nil && len(pa.Spec.PortLevelMtls) > 0 {
		portLevelMtlsJSON, err := json.Marshal(pa.Spec.PortLevelMtls)
		if err == nil {
			properties["port_level_mtls"] = string(portLevelMtlsJSON)
		}
	}

	return NewGraphNode(NodeTypePeerAuthentication, GetNodeID("PeerAuthentication", pa.Namespace, pa.Name), properties)
}

// RequestAuthenticationToGraphNode converts an Istio RequestAuthentication to a graph node
func RequestAuthenticationToGraphNode(ra *istiosecurityv1.RequestAuthentication) *GraphNode {
	properties := map[string]interface{}{
		"name":      ra.Name,
		"namespace": ra.Namespace,
		"uid":       string(ra.UID),
		"created":   ra.CreationTimestamp.Format("2006-01-02T15:04:05Z"),
	}

	// Add labels
	if len(ra.Labels) > 0 {
		properties["labels"] = serializeMap(ra.Labels)
	}

	// Add annotations
	if len(ra.Annotations) > 0 {
		properties["annotations"] = serializeMap(ra.Annotations)
	}

	// Add selector
	if ra.Spec.Selector != nil && len(ra.Spec.Selector.MatchLabels) > 0 {
		selectorJSON, err := json.Marshal(ra.Spec.Selector)
		if err == nil {
			properties["selector"] = string(selectorJSON)
		}
	}

	// Add JWT rules (serialized JSON)
	if len(ra.Spec.JwtRules) > 0 {
		jwtRulesJSON, err := json.Marshal(ra.Spec.JwtRules)
		if err == nil {
			properties["jwt_rules"] = string(jwtRulesJSON)
		}
	}

	return NewGraphNode(NodeTypeRequestAuthentication, GetNodeID("RequestAuthentication", ra.Namespace, ra.Name), properties)
}
