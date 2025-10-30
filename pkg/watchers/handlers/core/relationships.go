package core

import (
	"context"
	"fmt"

	"github.com/kagenti/kkbase/pkg/graph"
	"github.com/kagenti/kkbase/pkg/models"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
)

// RelationshipBuilder helps build relationships for core Kubernetes resources
type RelationshipBuilder struct {
	Clientset  *kubernetes.Clientset
	GraphStore graph.GraphStore
	Logger     *zap.Logger
}

// NewRelationshipBuilder creates a new core relationship builder
func NewRelationshipBuilder(clientset *kubernetes.Clientset, graphStore graph.GraphStore, logger *zap.Logger) *RelationshipBuilder {
	return &RelationshipBuilder{
		GraphStore: graphStore,
		Clientset:  clientset,
		Logger:     logger,
	}
}

// CreateOwnerEdge creates an edge based on owner reference
// CreateOwnerEdge creates an edge based on owner reference
// This is a generic method used across all resource types
func (rb *RelationshipBuilder) CreateOwnerEdge(ctx context.Context, childType models.NodeType, childID string, ownerRef metav1.OwnerReference, namespace string) error {
	// Convert the owner kind to a node type
	parentType, ok := models.NodeTypeFromKind(ownerRef.Kind)
	if !ok {
		rb.Logger.Debug("unknown owner kind", zap.String("kind", ownerRef.Kind))
		return nil
	}

	// Use MANAGES edge type for owner relationships
	edgeType := models.EdgeTypeManages

	// Determine if the owner is cluster-scoped (no namespace)
	ownerNamespace := namespace
	if parentType.IsClusterScoped() {
		ownerNamespace = ""
	}

	parentID := models.GetNodeID(ownerRef.Kind, ownerNamespace, ownerRef.Name)
	return rb.GraphStore.UpsertEdge(
		ctx,
		string(parentType),
		parentID,
		string(edgeType),
		string(childType),
		childID,
		nil,
	)
}

// CreatePodSchedulingEdges creates edges for pod scheduling
func (rb *RelationshipBuilder) CreatePodSchedulingEdges(ctx context.Context, pod *corev1.Pod) error {
	podID := models.GetNodeID("Pod", pod.Namespace, pod.Name)

	// Create SCHEDULED_ON edge to Node
	if pod.Spec.NodeName != "" {
		nodeID := models.GetNodeID("Node", "", pod.Spec.NodeName)
		if err := rb.GraphStore.UpsertEdge(
			ctx,
			string(NodeTypePod),
			podID,
			string(models.EdgeTypeScheduledOn),
			string(NodeTypeNode),
			nodeID,
			nil,
		); err != nil {
			return fmt.Errorf("failed to create SCHEDULED_ON edge: %w", err)
		}
	}

	// Create IN_NAMESPACE edge
	namespaceID := models.GetNodeID("Namespace", "", pod.Namespace)
	if err := rb.GraphStore.UpsertEdge(
		ctx,
		string(NodeTypePod),
		podID,
		string(models.EdgeTypeInNamespace),
		string(NodeTypeNamespace),
		namespaceID,
		nil,
	); err != nil {
		return fmt.Errorf("failed to create IN_NAMESPACE edge: %w", err)
	}

	return nil
}

// CreatePodContainerEdges creates CONTAINS edges from Pod to Containers
func (rb *RelationshipBuilder) CreatePodContainerEdges(ctx context.Context, pod *corev1.Pod) error {
	podID := models.GetNodeID("Pod", pod.Namespace, pod.Name)

	for _, container := range pod.Spec.Containers {
		containerID := fmt.Sprintf("Container/%s/%s/%s", pod.Namespace, pod.Name, container.Name)

		if err := rb.GraphStore.UpsertEdge(
			ctx,
			string(NodeTypePod),
			podID,
			string(models.EdgeTypeContains),
			string(models.NodeTypeContainer),
			containerID,
			nil,
		); err != nil {
			return fmt.Errorf("failed to create CONTAINS edge for container %s: %w", container.Name, err)
		}
	}

	return nil
}

// CreatePodVolumeEdges creates edges for pod volumes
func (rb *RelationshipBuilder) CreatePodVolumeEdges(ctx context.Context, pod *corev1.Pod) error {
	podID := models.GetNodeID("Pod", pod.Namespace, pod.Name)

	for _, volume := range pod.Spec.Volumes {
		// Handle PersistentVolumeClaim
		if volume.PersistentVolumeClaim != nil {
			pvcID := models.GetNodeID("PersistentVolumeClaim", pod.Namespace, volume.PersistentVolumeClaim.ClaimName)

			if err := rb.GraphStore.UpsertEdge(
				ctx,
				string(NodeTypePod),
				podID,
				string(models.EdgeTypeMounts),
				string(NodeTypePersistentVolumeClaim),
				pvcID,
				map[string]interface{}{
					"volume_name": volume.Name,
				},
			); err != nil {
				return fmt.Errorf("failed to create MOUNTS edge for PVC %s: %w", volume.PersistentVolumeClaim.ClaimName, err)
			}
		}

		// Handle ConfigMap
		if volume.ConfigMap != nil {
			configMapID := models.GetNodeID("ConfigMap", pod.Namespace, volume.ConfigMap.Name)

			if err := rb.GraphStore.UpsertEdge(
				ctx,
				string(NodeTypePod),
				podID,
				string(models.EdgeTypeUsesConfig),
				string(NodeTypeConfigMap),
				configMapID,
				map[string]interface{}{
					"volume_name": volume.Name,
				},
			); err != nil {
				return fmt.Errorf("failed to create USES_CONFIG edge for ConfigMap %s: %w", volume.ConfigMap.Name, err)
			}
		}

		// Handle Secret
		if volume.Secret != nil {
			secretID := models.GetNodeID("Secret", pod.Namespace, volume.Secret.SecretName)

			if err := rb.GraphStore.UpsertEdge(
				ctx,
				string(NodeTypePod),
				podID,
				string(models.EdgeTypeUsesSecret),
				string(NodeTypeSecret),
				secretID,
				map[string]interface{}{
					"volume_name": volume.Name,
				},
			); err != nil {
				return fmt.Errorf("failed to create USES_SECRET edge for Secret %s: %w", volume.Secret.SecretName, err)
			}
		}
	}

	// Handle environment variables from ConfigMaps and Secrets
	for _, container := range pod.Spec.Containers {
		for _, envFrom := range container.EnvFrom {
			if envFrom.ConfigMapRef != nil {
				configMapID := models.GetNodeID("ConfigMap", pod.Namespace, envFrom.ConfigMapRef.Name)

				if err := rb.GraphStore.UpsertEdge(
					ctx,
					string(NodeTypePod),
					podID,
					string(models.EdgeTypeUsesConfig),
					string(NodeTypeConfigMap),
					configMapID,
					map[string]interface{}{
						"container": container.Name,
						"env_from":  true,
					},
				); err != nil {
					return fmt.Errorf("failed to create USES_CONFIG edge for ConfigMap %s: %w", envFrom.ConfigMapRef.Name, err)
				}
			}

			if envFrom.SecretRef != nil {
				secretID := models.GetNodeID("Secret", pod.Namespace, envFrom.SecretRef.Name)

				if err := rb.GraphStore.UpsertEdge(
					ctx,
					string(NodeTypePod),
					podID,
					string(models.EdgeTypeUsesSecret),
					string(NodeTypeSecret),
					secretID,
					map[string]interface{}{
						"container": container.Name,
						"env_from":  true,
					},
				); err != nil {
					return fmt.Errorf("failed to create USES_SECRET edge for Secret %s: %w", envFrom.SecretRef.Name, err)
				}
			}
		}
	}

	return nil
}

// CreateServicePodEdges creates SELECTS_PODS edges from Service to Pods
func (rb *RelationshipBuilder) CreateServicePodEdges(ctx context.Context, service *corev1.Service) error {
	serviceID := models.GetNodeID("Service", service.Namespace, service.Name)

	// If no selector, nothing to do
	if len(service.Spec.Selector) == 0 {
		return nil
	}

	// Convert selector to label selector
	selector := labels.SelectorFromSet(service.Spec.Selector)

	// List pods matching the selector
	pods, err := rb.Clientset.CoreV1().Pods(service.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: selector.String(),
	})
	if err != nil {
		return fmt.Errorf("failed to list pods for service %s: %w", service.Name, err)
	}

	// Create edges to matching pods
	for _, pod := range pods.Items {
		podID := models.GetNodeID("Pod", pod.Namespace, pod.Name)

		if err := rb.GraphStore.UpsertEdge(
			ctx,
			string(NodeTypeService),
			serviceID,
			string(models.EdgeTypeSelectsPods),
			string(NodeTypePod),
			podID,
			nil,
		); err != nil {
			return fmt.Errorf("failed to create SELECTS_PODS edge to pod %s: %w", pod.Name, err)
		}
	}

	return nil
}

// CreatePVCPVEdge creates BOUND_TO edge from PVC to PV
func (rb *RelationshipBuilder) CreatePVCPVEdge(ctx context.Context, pvc *corev1.PersistentVolumeClaim) error {
	if pvc.Spec.VolumeName == "" {
		return nil
	}

	pvcID := models.GetNodeID("PersistentVolumeClaim", pvc.Namespace, pvc.Name)
	pvID := models.GetNodeID("PersistentVolume", "", pvc.Spec.VolumeName)

	return rb.GraphStore.UpsertEdge(
		ctx,
		string(NodeTypePersistentVolumeClaim),
		pvcID,
		string(models.EdgeTypeBoundTo),
		string(NodeTypePersistentVolume),
		pvID,
		nil,
	)
}

// CreatePVStorageClassEdge creates PROVISIONED_BY edge from PV to StorageClass
func (rb *RelationshipBuilder) CreatePVStorageClassEdge(ctx context.Context, pv *corev1.PersistentVolume) error {
	if pv.Spec.StorageClassName == "" {
		return nil
	}

	pvID := models.GetNodeID("PersistentVolume", "", pv.Name)
	storageClassID := models.GetNodeID("StorageClass", "", pv.Spec.StorageClassName)

	return rb.GraphStore.UpsertEdge(
		ctx,
		string(NodeTypePersistentVolume),
		pvID,
		string(models.EdgeTypeProvisionedBy),
		string(NodeTypeStorageClass),
		storageClassID,
		nil,
	)
}

// CreatePVCStorageClassEdge creates PROVISIONED_BY edge from PVC to StorageClass
func (rb *RelationshipBuilder) CreatePVCStorageClassEdge(ctx context.Context, pvc *corev1.PersistentVolumeClaim) error {
	if pvc.Spec.StorageClassName == nil || *pvc.Spec.StorageClassName == "" {
		return nil
	}

	pvcID := models.GetNodeID("PersistentVolumeClaim", pvc.Namespace, pvc.Name)
	storageClassID := models.GetNodeID("StorageClass", "", *pvc.Spec.StorageClassName)

	return rb.GraphStore.UpsertEdge(
		ctx,
		string(NodeTypePersistentVolumeClaim),
		pvcID,
		string(models.EdgeTypeProvisionedBy),
		string(NodeTypeStorageClass),
		storageClassID,
		nil,
	)
}

// CreateIngressServiceEdges creates ROUTES_TO edges from Ingress to Services
func (rb *RelationshipBuilder) CreateIngressServiceEdges(ctx context.Context, namespace, ingressName string, serviceName string) error {
	ingressID := models.GetNodeID("Ingress", namespace, ingressName)
	serviceID := models.GetNodeID("Service", namespace, serviceName)

	return rb.GraphStore.UpsertEdge(
		ctx,
		string(NodeTypeIngress),
		ingressID,
		string(models.EdgeTypeRoutesTo),
		string(NodeTypeService),
		serviceID,
		nil,
	)
}

// CreateEventInvolvedObjectEdge creates INVOLVES edge from Event to involved object
func (rb *RelationshipBuilder) CreateEventInvolvedObjectEdge(ctx context.Context, event *corev1.Event) error {
	// Event IDs include kind to avoid collisions
	eventID := fmt.Sprintf("Event/%s/%s/%s", event.Namespace, event.InvolvedObject.Name, event.Name)

	objectType, ok := models.NodeTypeFromKind(event.InvolvedObject.Kind)
	if !ok {
		rb.Logger.Debug("unknown involved object kind", zap.String("kind", event.InvolvedObject.Kind))
		return nil
	}

	// For cluster-scoped resources, don't use a namespace in the ID
	objectNamespace := ""
	if !objectType.IsClusterScoped() {
		objectNamespace = event.InvolvedObject.Namespace
		if objectNamespace == "" {
			objectNamespace = event.Namespace
		}
	}

	// Use the Kind from the event's involved object
	objectID := models.GetNodeID(event.InvolvedObject.Kind, objectNamespace, event.InvolvedObject.Name)

	return rb.GraphStore.UpsertEdge(
		ctx,
		string(NodeTypeK8sEvent),
		eventID,
		string(models.EdgeTypeInvolves),
		string(objectType),
		objectID,
		nil,
	)
}

// CreateNamespaceEdge creates IN_NAMESPACE edge for namespaced resources
func (rb *RelationshipBuilder) CreateNamespaceEdge(ctx context.Context, resourceType models.NodeType, resourceID, namespace string) error {
	namespaceID := models.GetNodeID("Namespace", "", namespace)
	return rb.GraphStore.UpsertEdge(
		ctx,
		string(resourceType),
		resourceID,
		string(models.EdgeTypeInNamespace),
		string(NodeTypeNamespace),
		namespaceID,
		nil,
	)
}
