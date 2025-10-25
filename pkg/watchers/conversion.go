package watchers

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
)

// ConvertToTyped converts an unstructured object to a typed object
// This is a generic helper for all dynamic handlers to convert from
// unstructured.Unstructured to their concrete types (e.g., corev1.Pod, gatewayv1.HTTPRoute)
func ConvertToTyped[T any](obj interface{}) (*T, error) {
	// Handle tombstone objects (from delete events)
	if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		obj = tombstone.Obj
	}

	unstructuredObj, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return nil, fmt.Errorf("expected unstructured.Unstructured, got %T", obj)
	}

	typed := new(T)
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.Object, typed); err != nil {
		return nil, fmt.Errorf("failed to convert to typed object: %w", err)
	}

	return typed, nil
}
