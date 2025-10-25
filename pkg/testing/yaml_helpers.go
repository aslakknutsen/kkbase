package testing

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
	gatewayv1alpha3 "sigs.k8s.io/gateway-api/apis/v1alpha3"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
)

var (
	// testScheme is a runtime scheme with all necessary types registered
	testScheme = runtime.NewScheme()
	// testCodecFactory for decoding YAML
	testCodecFactory serializer.CodecFactory
)

func init() {
	// Register core k8s types
	_ = scheme.AddToScheme(testScheme)

	// Register Gateway API types
	_ = gatewayv1.Install(testScheme)
	_ = gatewayv1beta1.Install(testScheme)
	_ = gatewayv1alpha2.Install(testScheme)
	_ = gatewayv1alpha3.Install(testScheme)

	testCodecFactory = serializer.NewCodecFactory(testScheme)
}

// ParseYAML parses a YAML string into a Kubernetes object
func ParseYAML(yamlStr string) (runtime.Object, error) {
	decode := testCodecFactory.UniversalDeserializer().Decode
	obj, _, err := decode([]byte(yamlStr), nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decode YAML: %w", err)
	}
	return obj, nil
}

// ParseYAMLAs is a generic parser that works for any Kubernetes resource type
// Example: httpRoute, err := ParseYAMLAs[*gatewayv1.HTTPRoute](yamlStr)
func ParseYAMLAs[T runtime.Object](yamlStr string) (T, error) {
	var zero T
	obj, err := ParseYAML(yamlStr)
	if err != nil {
		return zero, err
	}

	typed, ok := obj.(T)
	if !ok {
		return zero, fmt.Errorf("expected %T, got %T", zero, obj)
	}

	return typed, nil
}

// ToUnstructured converts a typed Kubernetes object to unstructured
// This is useful for testing handlers that expect unstructured input from informers
func ToUnstructured(obj runtime.Object) (*unstructured.Unstructured, error) {
	unstructuredMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, fmt.Errorf("failed to convert to unstructured: %w", err)
	}
	return &unstructured.Unstructured{Object: unstructuredMap}, nil
}
