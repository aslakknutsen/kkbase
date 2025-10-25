package gateway

import (
	"testing"

	"github.com/kagenti/kkbase/pkg/models"
	kktesting "github.com/kagenti/kkbase/pkg/testing"
	"github.com/kagenti/kkbase/pkg/watchers"
	"go.uber.org/zap"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func TestHTTPRouteHandler_HandleAdd(t *testing.T) {
	tests := []struct {
		name          string
		inputYAML     string
		expectedNodes []kktesting.ExpectedNode
		expectedEdges []kktesting.ExpectedEdge
	}{
		{
			name: "simple httproute with one gateway",
			inputYAML: `
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: example-route
  namespace: default
spec:
  parentRefs:
  - name: example-gateway
  hostnames:
  - "example.com"`,
			expectedNodes: []kktesting.ExpectedNode{
				{
					Type: "HTTPRoute",
					ID:   "HTTPRoute/default/example-route",
					Properties: map[string]interface{}{
						"name":      "example-route",
						"namespace": "default",
						"hostnames": []string{"example.com"},
					},
				},
			},
			expectedEdges: []kktesting.ExpectedEdge{
				{
					FromType:   "HTTPRoute",
					FromID:     "HTTPRoute/default/example-route",
					EdgeType:   "IN_NAMESPACE",
					ToType:     "Namespace",
					ToID:       "default",
					Properties: nil,
				},
				{
					FromType:   "HTTPRoute",
					FromID:     "HTTPRoute/default/example-route",
					EdgeType:   "ATTACHES_TO",
					ToType:     "Gateway",
					ToID:       "Gateway/default/example-gateway",
					Properties: nil,
				},
			},
		},
		{
			name: "httproute with backend services",
			inputYAML: `
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: backend-route
  namespace: default
spec:
  parentRefs:
  - name: my-gateway
  rules:
  - backendRefs:
    - name: service-a
      kind: Service
      port: 8080
    - name: service-b
      kind: Service
      port: 9090`,
			expectedNodes: []kktesting.ExpectedNode{
				{
					Type: "HTTPRoute",
					ID:   "HTTPRoute/default/backend-route",
					Properties: map[string]interface{}{
						"name":      "backend-route",
						"namespace": "default",
					},
				},
			},
			expectedEdges: []kktesting.ExpectedEdge{
				{
					FromType:   "HTTPRoute",
					FromID:     "HTTPRoute/default/backend-route",
					EdgeType:   "IN_NAMESPACE",
					ToType:     "Namespace",
					ToID:       "default",
					Properties: nil,
				},
				{
					FromType:   "HTTPRoute",
					FromID:     "HTTPRoute/default/backend-route",
					EdgeType:   "ATTACHES_TO",
					ToType:     "Gateway",
					ToID:       "Gateway/default/my-gateway",
					Properties: nil,
				},
				{
					FromType:   "HTTPRoute",
					FromID:     "HTTPRoute/default/backend-route",
					EdgeType:   "FORWARDS_TO",
					ToType:     "Service",
					ToID:       "Service/default/service-a",
					Properties: nil,
				},
				{
					FromType:   "HTTPRoute",
					FromID:     "HTTPRoute/default/backend-route",
					EdgeType:   "FORWARDS_TO",
					ToType:     "Service",
					ToID:       "Service/default/service-b",
					Properties: nil,
				},
			},
		},
		{
			name: "httproute with multiple gateways and weighted backends",
			inputYAML: `
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: complex-route
  namespace: prod
spec:
  parentRefs:
  - name: gateway-1
  - name: gateway-2
    sectionName: https
  hostnames:
  - "api.example.com"
  - "www.example.com"
  rules:
  - backendRefs:
    - name: primary-service
      kind: Service
      port: 8080
      weight: 80
    - name: canary-service
      kind: Service
      port: 8080
      weight: 20`,
			expectedNodes: []kktesting.ExpectedNode{
				{
					Type: "HTTPRoute",
					ID:   "HTTPRoute/prod/complex-route",
					Properties: map[string]interface{}{
						"name":      "complex-route",
						"namespace": "prod",
						"hostnames": []string{"api.example.com", "www.example.com"},
					},
				},
			},
			expectedEdges: []kktesting.ExpectedEdge{
				{
					FromType:   "HTTPRoute",
					FromID:     "HTTPRoute/prod/complex-route",
					EdgeType:   "IN_NAMESPACE",
					ToType:     "Namespace",
					ToID:       "prod",
					Properties: nil,
				},
				{
					FromType:   "HTTPRoute",
					FromID:     "HTTPRoute/prod/complex-route",
					EdgeType:   "ATTACHES_TO",
					ToType:     "Gateway",
					ToID:       "Gateway/prod/gateway-1",
					Properties: nil,
				},
				{
					FromType: "HTTPRoute",
					FromID:   "HTTPRoute/prod/complex-route",
					EdgeType: "ATTACHES_TO",
					ToType:   "Gateway",
					ToID:     "Gateway/prod/gateway-2",
					Properties: map[string]interface{}{
						"section_name": "https",
					},
				},
				{
					FromType: "HTTPRoute",
					FromID:   "HTTPRoute/prod/complex-route",
					EdgeType: "FORWARDS_TO",
					ToType:   "Service",
					ToID:     "Service/prod/primary-service",
					Properties: map[string]interface{}{
						"weight": int32(80),
					},
				},
				{
					FromType: "HTTPRoute",
					FromID:   "HTTPRoute/prod/complex-route",
					EdgeType: "FORWARDS_TO",
					ToType:   "Service",
					ToID:     "Service/prod/canary-service",
					Properties: map[string]interface{}{
						"weight": int32(20),
					},
				},
			},
		},
		{
			name: "httproute with cross-namespace references",
			inputYAML: `
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: cross-ns-route
  namespace: app-namespace
spec:
  parentRefs:
  - name: shared-gateway
    namespace: gateway-namespace
  rules:
  - backendRefs:
    - name: backend-service
      namespace: backend-namespace
      kind: Service
      port: 8080`,
			expectedNodes: []kktesting.ExpectedNode{
				{
					Type: "HTTPRoute",
					ID:   "HTTPRoute/app-namespace/cross-ns-route",
					Properties: map[string]interface{}{
						"name":      "cross-ns-route",
						"namespace": "app-namespace",
					},
				},
			},
			expectedEdges: []kktesting.ExpectedEdge{
				{
					FromType:   "HTTPRoute",
					FromID:     "HTTPRoute/app-namespace/cross-ns-route",
					EdgeType:   "IN_NAMESPACE",
					ToType:     "Namespace",
					ToID:       "app-namespace",
					Properties: nil,
				},
				{
					FromType:   "HTTPRoute",
					FromID:     "HTTPRoute/app-namespace/cross-ns-route",
					EdgeType:   "ATTACHES_TO",
					ToType:     "Gateway",
					ToID:       "Gateway/gateway-namespace/shared-gateway",
					Properties: nil,
				},
				{
					FromType:   "HTTPRoute",
					FromID:     "HTTPRoute/app-namespace/cross-ns-route",
					EdgeType:   "FORWARDS_TO",
					ToType:     "Service",
					ToID:       "Service/backend-namespace/backend-service",
					Properties: nil,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse the YAML input
			httpRoute, err := kktesting.ParseYAMLAs[*gatewayv1.HTTPRoute](tt.inputYAML)
			if err != nil {
				t.Fatalf("Failed to parse HTTPRoute YAML: %v", err)
			}

			// Convert to unstructured (handlers expect this from informers)
			unstructuredObj, err := kktesting.ToUnstructured(httpRoute)
			if err != nil {
				t.Fatalf("Failed to convert to unstructured: %v", err)
			}

			// Create mock graph store
			mockStore := kktesting.NewMockGraphStore()

			// Create logger
			logger := zap.NewNop()

			// Create handler with mock store
			// Note: We create a minimal handler without the full informer setup
			handler := &HTTPRouteHandler{
				BaseWatcher:         watchers.NewBaseWatcher(mockStore, logger, nil),
				relationshipBuilder: watchers.NewRelationshipBuilder(nil, mockStore, logger),
			}

			// Execute the handler
			handler.HandleAdd(unstructuredObj)

			// Verify expected nodes
			for _, expected := range tt.expectedNodes {
				kktesting.AssertNodeCreated(t, mockStore, expected.Type, expected.ID, expected.Properties)
			}

			// Verify total node count
			kktesting.AssertTotalNodeCount(t, mockStore, len(tt.expectedNodes))

			// Verify expected edges
			for _, expected := range tt.expectedEdges {
				kktesting.AssertEdgeCreated(t, mockStore,
					expected.FromType, expected.FromID,
					expected.EdgeType,
					expected.ToType, expected.ToID,
					expected.Properties)
			}

			// Verify total edge count
			kktesting.AssertTotalEdgeCount(t, mockStore, len(tt.expectedEdges))
		})
	}
}

func TestHTTPRouteHandler_HandleUpdate(t *testing.T) {
	inputYAML := `
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: update-route
  namespace: default
spec:
  parentRefs:
  - name: gateway-1
  rules:
  - backendRefs:
    - name: service-a
      kind: Service`

	// Parse the YAML input
	httpRoute, err := kktesting.ParseYAMLAs[*gatewayv1.HTTPRoute](inputYAML)
	if err != nil {
		t.Fatalf("Failed to parse HTTPRoute YAML: %v", err)
	}

	// Convert to unstructured
	unstructuredObj, err := kktesting.ToUnstructured(httpRoute)
	if err != nil {
		t.Fatalf("Failed to convert to unstructured: %v", err)
	}

	// Create mock graph store
	mockStore := kktesting.NewMockGraphStore()

	// Create logger
	logger := zap.NewNop()

	// Create handler with mock store
	handler := &HTTPRouteHandler{
		BaseWatcher:         watchers.NewBaseWatcher(mockStore, logger, nil),
		relationshipBuilder: watchers.NewRelationshipBuilder(nil, mockStore, logger),
	}

	// Execute the handler update (which calls HandleAdd internally)
	handler.HandleUpdate(nil, unstructuredObj)

	// Verify that DeleteEdgesByNode was called
	if len(mockStore.DeletedEdges) != 1 {
		t.Errorf("Expected 1 DeleteEdgesByNode call, got %d", len(mockStore.DeletedEdges))
	}

	if len(mockStore.DeletedEdges) > 0 {
		deleted := mockStore.DeletedEdges[0]
		expectedNodeID := models.GetNodeID("HTTPRoute", "default", "update-route")
		if deleted.NodeID != expectedNodeID {
			t.Errorf("Expected deleted edges for node %s, got %s", expectedNodeID, deleted.NodeID)
		}
	}

	// Verify that nodes and edges were recreated
	kktesting.AssertNodeCreated(t, mockStore, "HTTPRoute", "HTTPRoute/default/update-route", map[string]interface{}{
		"name":      "update-route",
		"namespace": "default",
	})

	kktesting.AssertEdgeCount(t, mockStore, "IN_NAMESPACE", 1)
	kktesting.AssertEdgeCount(t, mockStore, "ATTACHES_TO", 1)
	kktesting.AssertEdgeCount(t, mockStore, "FORWARDS_TO", 1)
}

func TestHTTPRouteHandler_HandleDelete(t *testing.T) {
	inputYAML := `
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: delete-route
  namespace: default
spec:
  parentRefs:
  - name: gateway-1`

	// Parse the YAML input
	httpRoute, err := kktesting.ParseYAMLAs[*gatewayv1.HTTPRoute](inputYAML)
	if err != nil {
		t.Fatalf("Failed to parse HTTPRoute YAML: %v", err)
	}

	// Convert to unstructured
	unstructuredObj, err := kktesting.ToUnstructured(httpRoute)
	if err != nil {
		t.Fatalf("Failed to convert to unstructured: %v", err)
	}

	// Create mock graph store
	mockStore := kktesting.NewMockGraphStore()

	// Create logger
	logger := zap.NewNop()

	// Create handler with mock store
	handler := &HTTPRouteHandler{
		BaseWatcher:         watchers.NewBaseWatcher(mockStore, logger, nil),
		relationshipBuilder: watchers.NewRelationshipBuilder(nil, mockStore, logger),
	}

	// Execute the handler delete
	handler.HandleDelete(unstructuredObj)

	// Verify that DeleteNode was called
	if len(mockStore.DeletedNodes) != 1 {
		t.Errorf("Expected 1 DeleteNode call, got %d", len(mockStore.DeletedNodes))
	}

	if len(mockStore.DeletedNodes) > 0 {
		deleted := mockStore.DeletedNodes[0]
		expectedNodeID := models.GetNodeID("HTTPRoute", "default", "delete-route")
		if deleted.ID != expectedNodeID {
			t.Errorf("Expected deleted node ID %s, got %s", expectedNodeID, deleted.ID)
		}
		if deleted.NodeType != "HTTPRoute" {
			t.Errorf("Expected deleted node type HTTPRoute, got %s", deleted.NodeType)
		}
	}
}
