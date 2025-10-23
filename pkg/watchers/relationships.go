package watchers

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

// RelationshipBuilder helps build relationships between resources
type RelationshipBuilder struct {
	clientset  *kubernetes.Clientset
	graphStore graph.GraphStore
	logger     *zap.Logger
}

// NewRelationshipBuilder creates a new relationship builder
func NewRelationshipBuilder(clientset *kubernetes.Clientset, graphStore graph.GraphStore, logger *zap.Logger) *RelationshipBuilder {
	return &RelationshipBuilder{
		clientset:  clientset,
		graphStore: graphStore,
		logger:     logger,
	}
}

// CreateOwnerEdge creates an edge based on owner reference
func (rb *RelationshipBuilder) CreateOwnerEdge(ctx context.Context, childType models.NodeType, childID string, ownerRef metav1.OwnerReference, namespace string) error {
	var parentType models.NodeType
	var edgeType models.EdgeType

	switch ownerRef.Kind {
	case "Deployment":
		parentType = models.NodeTypeDeployment
		edgeType = models.EdgeTypeManages
	case "ReplicaSet":
		parentType = models.NodeTypeReplicaSet
		edgeType = models.EdgeTypeManages
	case "StatefulSet":
		parentType = models.NodeTypeStatefulSet
		edgeType = models.EdgeTypeManages
	case "DaemonSet":
		parentType = models.NodeTypeDaemonSet
		edgeType = models.EdgeTypeManages
	default:
		rb.logger.Debug("unknown owner kind", zap.String("kind", ownerRef.Kind))
		return nil
	}

	parentID := models.GetNodeID(ownerRef.Kind, namespace, ownerRef.Name)
	return rb.graphStore.UpsertEdge(
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
		if err := rb.graphStore.UpsertEdge(
			ctx,
			string(models.NodeTypePod),
			podID,
			string(models.EdgeTypeScheduledOn),
			string(models.NodeTypeNode),
			pod.Spec.NodeName,
			nil,
		); err != nil {
			return fmt.Errorf("failed to create SCHEDULED_ON edge: %w", err)
		}
	}

	// Create IN_NAMESPACE edge
	if err := rb.graphStore.UpsertEdge(
		ctx,
		string(models.NodeTypePod),
		podID,
		string(models.EdgeTypeInNamespace),
		string(models.NodeTypeNamespace),
		pod.Namespace,
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

		if err := rb.graphStore.UpsertEdge(
			ctx,
			string(models.NodeTypePod),
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

			if err := rb.graphStore.UpsertEdge(
				ctx,
				string(models.NodeTypePod),
				podID,
				string(models.EdgeTypeMounts),
				string(models.NodeTypePersistentVolumeClaim),
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

			if err := rb.graphStore.UpsertEdge(
				ctx,
				string(models.NodeTypePod),
				podID,
				string(models.EdgeTypeUsesConfig),
				string(models.NodeTypeConfigMap),
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

			if err := rb.graphStore.UpsertEdge(
				ctx,
				string(models.NodeTypePod),
				podID,
				string(models.EdgeTypeUsesSecret),
				string(models.NodeTypeSecret),
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

				if err := rb.graphStore.UpsertEdge(
					ctx,
					string(models.NodeTypePod),
					podID,
					string(models.EdgeTypeUsesConfig),
					string(models.NodeTypeConfigMap),
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

				if err := rb.graphStore.UpsertEdge(
					ctx,
					string(models.NodeTypePod),
					podID,
					string(models.EdgeTypeUsesSecret),
					string(models.NodeTypeSecret),
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
	pods, err := rb.clientset.CoreV1().Pods(service.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: selector.String(),
	})
	if err != nil {
		return fmt.Errorf("failed to list pods for service %s: %w", service.Name, err)
	}

	// Create edges to matching pods
	for _, pod := range pods.Items {
		podID := models.GetNodeID("Pod", pod.Namespace, pod.Name)

		if err := rb.graphStore.UpsertEdge(
			ctx,
			string(models.NodeTypeService),
			serviceID,
			string(models.EdgeTypeSelectsPods),
			string(models.NodeTypePod),
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

	return rb.graphStore.UpsertEdge(
		ctx,
		string(models.NodeTypePersistentVolumeClaim),
		pvcID,
		string(models.EdgeTypeBoundTo),
		string(models.NodeTypePersistentVolume),
		pvc.Spec.VolumeName,
		nil,
	)
}

// CreatePVStorageClassEdge creates PROVISIONED_BY edge from PV to StorageClass
func (rb *RelationshipBuilder) CreatePVStorageClassEdge(ctx context.Context, pv *corev1.PersistentVolume) error {
	if pv.Spec.StorageClassName == "" {
		return nil
	}

	return rb.graphStore.UpsertEdge(
		ctx,
		string(models.NodeTypePersistentVolume),
		pv.Name,
		string(models.EdgeTypeProvisionedBy),
		string(models.NodeTypeStorageClass),
		pv.Spec.StorageClassName,
		nil,
	)
}

// CreateIngressServiceEdges creates ROUTES_TO edges from Ingress to Services
func (rb *RelationshipBuilder) CreateIngressServiceEdges(ctx context.Context, namespace, ingressName string, serviceName string) error {
	ingressID := models.GetNodeID("Ingress", namespace, ingressName)
	serviceID := models.GetNodeID("Service", namespace, serviceName)

	return rb.graphStore.UpsertEdge(
		ctx,
		string(models.NodeTypeIngress),
		ingressID,
		string(models.EdgeTypeRoutesTo),
		string(models.NodeTypeService),
		serviceID,
		nil,
	)
}

// CreateEventInvolvedObjectEdge creates INVOLVES edge from Event to involved object
func (rb *RelationshipBuilder) CreateEventInvolvedObjectEdge(ctx context.Context, event *corev1.Event) error {
	// Event IDs include kind to avoid collisions
	eventID := fmt.Sprintf("Event/%s/%s/%s", event.Namespace, event.InvolvedObject.Name, event.Name)

	var objectType models.NodeType
	switch event.InvolvedObject.Kind {
	case "Pod":
		objectType = models.NodeTypePod
	case "Node":
		objectType = models.NodeTypeNode
	case "Deployment":
		objectType = models.NodeTypeDeployment
	case "ReplicaSet":
		objectType = models.NodeTypeReplicaSet
	case "Service":
		objectType = models.NodeTypeService
	case "PersistentVolumeClaim":
		objectType = models.NodeTypePersistentVolumeClaim
	case "PersistentVolume":
		objectType = models.NodeTypePersistentVolume
	default:
		rb.logger.Debug("unknown involved object kind", zap.String("kind", event.InvolvedObject.Kind))
		return nil
	}

	objectNamespace := event.InvolvedObject.Namespace
	if objectNamespace == "" {
		objectNamespace = event.Namespace
	}

	// Use the Kind from the event's involved object
	objectID := models.GetNodeID(event.InvolvedObject.Kind, objectNamespace, event.InvolvedObject.Name)

	return rb.graphStore.UpsertEdge(
		ctx,
		string(models.NodeTypeK8sEvent),
		eventID,
		string(models.EdgeTypeInvolves),
		string(objectType),
		objectID,
		nil,
	)
}

// CreateNamespaceEdge creates IN_NAMESPACE edge for namespaced resources
func (rb *RelationshipBuilder) CreateNamespaceEdge(ctx context.Context, resourceType models.NodeType, resourceID, namespace string) error {
	return rb.graphStore.UpsertEdge(
		ctx,
		string(resourceType),
		resourceID,
		string(models.EdgeTypeInNamespace),
		string(models.NodeTypeNamespace),
		namespace,
		nil,
	)
}
