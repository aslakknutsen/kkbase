# Handler Testing Framework

This package provides a testing framework for testing Kubernetes resource handlers that interact with the graph database. The framework uses a mock GraphStore to record operations and provides assertions to verify expected behavior.

## Overview

The testing framework consists of:

1. **MockGraphStore**: A mock implementation that records all graph operations
2. **Assertion Helpers**: Functions to verify nodes and edges were created correctly
3. **YAML Parsing**: Utilities to parse inline YAML into Kubernetes objects
4. **Expected Types**: Structs for declaring expected test outcomes

## Quick Start

### Basic Test Structure

```go
func TestMyHandler_HandleAdd(t *testing.T) {
    tests := []struct {
        name          string
        inputYAML     string
        expectedNodes []kktesting.ExpectedNode
        expectedEdges []kktesting.ExpectedEdge
    }{
        {
            name: "basic test case",
            inputYAML: `
apiVersion: v1
kind: Service
metadata:
  name: my-service
  namespace: default
spec:
  selector:
    app: myapp`,
            expectedNodes: []kktesting.ExpectedNode{
                {
                    Type: "Service",
                    ID:   "Service/default/my-service",
                    Properties: map[string]interface{}{
                        "name":      "my-service",
                        "namespace": "default",
                    },
                },
            },
            expectedEdges: []kktesting.ExpectedEdge{
                {
                    FromType: "Service",
                    FromID:   "Service/default/my-service",
                    EdgeType: "IN_NAMESPACE",
                    ToType:   "Namespace",
                    ToID:     "default",
                },
            },
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // 1. Parse YAML
            obj, err := kktesting.ParseYAML(tt.inputYAML)
            if err != nil {
                t.Fatalf("Failed to parse YAML: %v", err)
            }

            // 2. Convert to unstructured (handlers expect this)
            unstructuredObj, err := kktesting.ToUnstructured(obj)
            if err != nil {
                t.Fatalf("Failed to convert to unstructured: %v", err)
            }

            // 3. Create mock store
            mockStore := kktesting.NewMockGraphStore()

            // 4. Create handler
            handler := NewMyHandler(mockStore, logger)

            // 5. Execute handler
            handler.HandleAdd(unstructuredObj)

            // 6. Verify expected nodes
            for _, expected := range tt.expectedNodes {
                kktesting.AssertNodeCreated(t, mockStore, 
                    expected.Type, expected.ID, expected.Properties)
            }

            // 7. Verify expected edges
            for _, expected := range tt.expectedEdges {
                kktesting.AssertEdgeCreated(t, mockStore,
                    expected.FromType, expected.FromID,
                    expected.EdgeType,
                    expected.ToType, expected.ToID,
                    expected.Properties)
            }
        })
    }
}
```

## Components

### MockGraphStore

Records all operations performed on the graph store:

```go
mockStore := kktesting.NewMockGraphStore()

// After running handler
fmt.Println("Nodes created:", len(mockStore.Nodes))
fmt.Println("Edges created:", len(mockStore.Edges))

// Query recorded operations
nodes := mockStore.GetNodesByType("Pod")
edges := mockStore.GetEdgesByType("CONTAINS")

// Find specific items
node := mockStore.FindNode("Pod", "Pod/default/my-pod")
edge := mockStore.FindEdge("Pod", "Pod/default/my-pod", "CONTAINS", "Container", "Container/...")

// Reset between tests
mockStore.Reset()
```

### Assertion Helpers

#### AssertNodeCreated

Verifies a node was created with expected properties:

```go
kktesting.AssertNodeCreated(t, mockStore, 
    "Pod",                      // node type
    "Pod/default/my-pod",       // node ID
    map[string]interface{}{     // expected properties (nil = don't check)
        "name": "my-pod",
        "namespace": "default",
    })
```

#### AssertEdgeCreated

Verifies an edge was created with expected properties:

```go
kktesting.AssertEdgeCreated(t, mockStore,
    "Pod",                      // from type
    "Pod/default/my-pod",       // from ID
    "SCHEDULED_ON",             // edge type
    "Node",                     // to type
    "node-1",                   // to ID
    nil)                        // properties (nil = don't check)
```

#### Count Assertions

```go
// Verify count of specific node type
kktesting.AssertNodeCount(t, mockStore, "Pod", 1)

// Verify count of specific edge type
kktesting.AssertEdgeCount(t, mockStore, "CONTAINS", 3)

// Verify total counts
kktesting.AssertTotalNodeCount(t, mockStore, 4)
kktesting.AssertTotalEdgeCount(t, mockStore, 5)
```

### YAML Parsing

Parse inline YAML into Kubernetes objects:

```go
// Generic parsing - works with ANY K8s resource type
httpRoute, err := kktesting.ParseYAMLAs[*gatewayv1.HTTPRoute](yamlString)
pod, err := kktesting.ParseYAMLAs[*corev1.Pod](yamlString)
virtualService, err := kktesting.ParseYAMLAs[*istiov1.VirtualService](yamlString)
gateway, err := kktesting.ParseYAMLAs[*gatewayv1.Gateway](yamlString)

// Low-level untyped parsing (returns runtime.Object)
obj, err := kktesting.ParseYAML(yamlString)

// Convert typed object to unstructured (required for handlers)
unstructuredObj, err := kktesting.ToUnstructured(httpRoute)
```

### Expected Types

Define expected outcomes in test cases:

```go
type ExpectedNode struct {
    Type       string                 // Node type (e.g., "Pod", "Service")
    ID         string                 // Node ID
    Properties map[string]interface{} // nil = don't check properties
}

type ExpectedEdge struct {
    FromType   string                 // Source node type
    FromID     string                 // Source node ID
    EdgeType   string                 // Edge type (e.g., "CONTAINS")
    ToType     string                 // Target node type
    ToID       string                 // Target node ID
    Properties map[string]interface{} // nil = don't check properties
}
```

## Property Matching

Properties can be matched in three ways:

1. **Nil properties**: Don't check properties at all
   ```go
   Properties: nil
   ```

2. **Partial match**: Only check specified properties
   ```go
   Properties: map[string]interface{}{
       "name": "my-pod",
       // Other properties in the node are ignored
   }
   ```

3. **Exact match**: Use `AssertTotalNodeCount` to ensure no extra nodes were created

## Testing Different Handler Operations

### Testing HandleAdd

```go
handler.HandleAdd(unstructuredObj)
kktesting.AssertNodeCreated(t, mockStore, "Pod", "Pod/default/my-pod", ...)
```

### Testing HandleUpdate

```go
handler.HandleUpdate(nil, unstructuredObj)

// Verify old edges were deleted
if len(mockStore.DeletedEdges) != 1 {
    t.Errorf("Expected 1 DeleteEdgesByNode call")
}

// Verify new nodes/edges were created
kktesting.AssertNodeCreated(t, mockStore, ...)
```

### Testing HandleDelete

```go
handler.HandleDelete(unstructuredObj)

// Verify node was deleted
if len(mockStore.DeletedNodes) != 1 {
    t.Errorf("Expected 1 DeleteNode call")
}
```

## Best Practices

1. **Use inline YAML**: More readable than programmatic construction
2. **Test cross-namespace references**: Ensure proper namespace handling
3. **Test edge properties**: Verify weights, labels, and other edge attributes
4. **Test error paths**: Use `mockStore.UpsertNodeErr` to inject errors
5. **Keep tests focused**: One aspect of behavior per test case
6. **Verify total counts**: Ensure no unexpected nodes/edges are created

## Example: Complete Test

See `../../pkg/watchers/handlers/extensions/gateway/httproute_test.go` for a complete example with:
- Simple resource with one relationship
- Resource with multiple backends
- Complex resource with weights and properties
- Cross-namespace reference handling
- Update and delete operations

## Adding Support for New Resource Types

To add parsing support for a new resource type, register the scheme in `yaml_helpers.go`:

```go
import istiov1 "istio.io/client-go/pkg/apis/networking/v1"

func init() {
    _ = istiov1.AddToScheme(testScheme)
}
```

Then use the generic parser directly:

```go
virtualService, err := kktesting.ParseYAMLAs[*istiov1.VirtualService](yamlStr)
deployment, err := kktesting.ParseYAMLAs[*appsv1.Deployment](yamlStr)
```

The generic `ParseYAMLAs` works with any registered Kubernetes resource type.

## Troubleshooting

### "expected unstructured.Unstructured, got *v1.Pod"

You forgot to convert to unstructured before calling the handler:
```go
unstructuredObj, err := kktesting.ToUnstructured(pod)
handler.HandleAdd(unstructuredObj)
```

### "Property type mismatch"

Make sure property types match exactly:
```go
// Wrong: []interface{}{"example.com"}
// Right: []string{"example.com"}
Properties: map[string]interface{}{
    "hostnames": []string{"example.com"},
}
```

### "Node not found"

Check the node ID format. It should match `GetNodeID()` output:
```go
// Namespaced: "Type/namespace/name"
ID: "Pod/default/my-pod"

// Cluster-scoped: "Type/name"
ID: "Node/node-1"
```

## Testing Out-of-Order Data Scenarios

The system uses the Create-on-Write pattern to handle resources that arrive out of order. When testing, you should verify this behavior works correctly.

### What is Out-of-Order Data?

Out-of-order data occurs when a resource references another resource that hasn't been processed yet:
- A Pod references a ConfigMap that hasn't been created
- A Service selector matches Pods that haven't arrived
- A VirtualService routes to a Service not yet in the graph

### Placeholder Node Behavior

When an edge is created between nodes where one or both don't exist yet, the system automatically creates **placeholder nodes** with minimal properties:

```go
// Pod references ConfigMap that doesn't exist yet
// Result: ConfigMap placeholder node is created
{
    "id": "ConfigMap/default/app-config",
    "placeholder": true,
    "created_at": timestamp,
    "updated_at": timestamp
}
```

### Testing Placeholder Creation

#### Test 1: Reference to Non-Existent Resource

```go
func TestPodHandler_PlaceholderConfigMap(t *testing.T) {
    mockStore := kktesting.NewMockGraphStore()
    handler := NewPodHandler(mockStore, logger, relationshipBuilder)
    
    // Pod references ConfigMap that doesn't exist
    pod := &corev1.Pod{
        ObjectMeta: metav1.ObjectMeta{
            Name:      "app-pod",
            Namespace: "default",
        },
        Spec: corev1.PodSpec{
            Volumes: []corev1.Volume{
                {
                    Name: "config-volume",
                    VolumeSource: corev1.VolumeSource{
                        ConfigMap: &corev1.ConfigMapVolumeSource{
                            LocalObjectReference: corev1.LocalObjectReference{
                                Name: "app-config",
                            },
                        },
                    },
                },
            },
        },
    }
    
    unstructuredObj, _ := kktesting.ToUnstructured(pod)
    handler.HandleAdd(unstructuredObj)
    
    // Verify Pod node was created
    kktesting.AssertNodeCreated(t, mockStore, "Pod", "Pod/default/app-pod", nil)
    
    // Verify ConfigMap placeholder was created
    kktesting.AssertNodeCreated(t, mockStore, "ConfigMap", "ConfigMap/default/app-config", 
        map[string]interface{}{
            "placeholder": true,
        })
    
    // Verify USES_CONFIG edge was created
    kktesting.AssertEdgeCreated(t, mockStore, 
        "Pod", "Pod/default/app-pod",
        "USES_CONFIG",
        "ConfigMap", "ConfigMap/default/app-config",
        nil)
}
```

#### Test 2: Placeholder Enrichment

```go
func TestConfigMapHandler_EnrichesPlaceholder(t *testing.T) {
    mockStore := kktesting.NewMockGraphStore()
    handler := NewConfigMapHandler(mockStore, logger)
    
    // First, create a placeholder by having a Pod reference it
    podHandler := NewPodHandler(mockStore, logger, relationshipBuilder)
    pod := createPodReferencingConfigMap("app-pod", "app-config")
    unstructuredPod, _ := kktesting.ToUnstructured(pod)
    podHandler.HandleAdd(unstructuredPod)
    
    // Verify placeholder exists
    placeholderNode := mockStore.FindNode("ConfigMap", "ConfigMap/default/app-config")
    if placeholderNode == nil {
        t.Fatal("Placeholder ConfigMap should exist")
    }
    if placeholderNode.Properties["placeholder"] != true {
        t.Error("ConfigMap should be marked as placeholder")
    }
    
    mockStore.Reset() // Reset to track only the next operation
    
    // Now the actual ConfigMap arrives
    configMap := &corev1.ConfigMap{
        ObjectMeta: metav1.ObjectMeta{
            Name:      "app-config",
            Namespace: "default",
        },
        Data: map[string]string{
            "key": "value",
        },
    }
    
    unstructuredCM, _ := kktesting.ToUnstructured(configMap)
    handler.HandleAdd(unstructuredCM)
    
    // Verify the node was updated with full data
    enrichedNode := mockStore.FindNode("ConfigMap", "ConfigMap/default/app-config")
    if enrichedNode == nil {
        t.Fatal("ConfigMap should exist after enrichment")
    }
    
    // Verify placeholder flag is now false
    if enrichedNode.Properties["placeholder"] != false {
        t.Error("ConfigMap should no longer be marked as placeholder")
    }
    
    // Verify it has real data now
    if enrichedNode.Properties["name"] != "app-config" {
        t.Error("ConfigMap should have full properties")
    }
}
```

#### Test 3: Verify Order Independence

```go
func TestOrderIndependence(t *testing.T) {
    tests := []struct {
        name  string
        order []string // "pod", "configmap"
    }{
        {"pod first, then configmap", []string{"pod", "configmap"}},
        {"configmap first, then pod", []string{"configmap", "pod"}},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            mockStore := kktesting.NewMockGraphStore()
            
            pod := createPod()
            configMap := createConfigMap()
            
            for _, resource := range tt.order {
                if resource == "pod" {
                    handler := NewPodHandler(mockStore, logger, relationshipBuilder)
                    unstructured, _ := kktesting.ToUnstructured(pod)
                    handler.HandleAdd(unstructured)
                } else {
                    handler := NewConfigMapHandler(mockStore, logger)
                    unstructured, _ := kktesting.ToUnstructured(configMap)
                    handler.HandleAdd(unstructured)
                }
            }
            
            // Regardless of order, both nodes should exist
            kktesting.AssertNodeCreated(t, mockStore, "Pod", "Pod/default/app-pod", nil)
            kktesting.AssertNodeCreated(t, mockStore, "ConfigMap", "ConfigMap/default/app-config", 
                map[string]interface{}{
                    "placeholder": false, // Should be enriched
                })
            
            // Edge should exist
            kktesting.AssertEdgeCreated(t, mockStore, 
                "Pod", "Pod/default/app-pod",
                "USES_CONFIG",
                "ConfigMap", "ConfigMap/default/app-config",
                nil)
        })
    }
}
```

### Integration Testing with Neo4j

For integration tests with a real Neo4j instance:

```go
func TestPlaceholderLifecycle_Integration(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }
    
    // Connect to test Neo4j instance
    store, err := neo4j.NewStore(neo4j.Config{
        URI:      "bolt://localhost:7687",
        Username: "neo4j",
        Password: "test",
        Database: "neo4j",
    }, logger)
    if err != nil {
        t.Fatalf("Failed to connect to Neo4j: %v", err)
    }
    defer store.Close()
    
    ctx := context.Background()
    
    // 1. Create edge with non-existent nodes
    err = store.UpsertEdge(ctx, "Pod", "Pod/default/test-pod", 
        "USES_CONFIG", "ConfigMap", "ConfigMap/default/test-config", nil)
    if err != nil {
        t.Fatalf("Failed to create edge: %v", err)
    }
    
    // 2. Query to verify placeholder nodes were created
    results, err := store.Query(ctx, 
        "MATCH (cm:ConfigMap {id: $id}) RETURN cm.placeholder AS placeholder",
        map[string]interface{}{"id": "ConfigMap/default/test-config"})
    if err != nil {
        t.Fatalf("Failed to query: %v", err)
    }
    if len(results) == 0 {
        t.Fatal("Placeholder ConfigMap should exist")
    }
    if results[0]["placeholder"] != true {
        t.Error("ConfigMap should be marked as placeholder")
    }
    
    // 3. Upsert the real ConfigMap
    err = store.UpsertNode(ctx, "ConfigMap", "ConfigMap/default/test-config",
        map[string]interface{}{
            "name":      "test-config",
            "namespace": "default",
            "data":      "key=value",
        })
    if err != nil {
        t.Fatalf("Failed to upsert node: %v", err)
    }
    
    // 4. Verify placeholder flag was cleared
    results, err = store.Query(ctx, 
        "MATCH (cm:ConfigMap {id: $id}) RETURN cm.placeholder AS placeholder, cm.name AS name",
        map[string]interface{}{"id": "ConfigMap/default/test-config"})
    if err != nil {
        t.Fatalf("Failed to query: %v", err)
    }
    if results[0]["placeholder"] != false {
        t.Error("ConfigMap should no longer be marked as placeholder")
    }
    if results[0]["name"] != "test-config" {
        t.Error("ConfigMap should have full properties")
    }
    
    // Cleanup
    store.DeleteNode(ctx, "Pod", "Pod/default/test-pod")
    store.DeleteNode(ctx, "ConfigMap", "ConfigMap/default/test-config")
}
```

### Monitoring Placeholder Metrics in Tests

For development and CI environments, you can query placeholder metrics:

```cypher
// Count placeholder nodes by type
MATCH (n)
WHERE n.placeholder = true
RETURN labels(n)[0] AS Type, count(n) AS Count
ORDER BY Count DESC

// Find old placeholders (> 5 minutes in tests)
MATCH (n)
WHERE n.placeholder = true 
  AND n.updated_at < (timestamp() / 1000) - 300
RETURN labels(n)[0] AS Type, n.id AS ID
```

### Best Practices for Out-of-Order Testing

1. **Test Both Orders**: Always test resource A→B and B→A arrival orders
2. **Verify Placeholder Flags**: Check that `placeholder: true` is set on creation and `placeholder: false` on enrichment
3. **Check Relationships**: Ensure edges exist regardless of node arrival order
4. **Integration Tests**: Use real Neo4j for end-to-end placeholder lifecycle tests
5. **Monitor Cleanup**: In long-running tests, verify orphaned placeholders are eventually cleaned up
6. **Test Failure Scenarios**: What happens if the real resource never arrives?

### Common Pitfalls

1. **Not resetting MockGraphStore**: Call `mockStore.Reset()` between test phases
2. **Checking exact property counts**: Placeholder nodes will have fewer properties than real nodes
3. **Assuming order**: Never write tests that depend on resource processing order
4. **Ignoring cleanup**: Test that orphaned placeholders are eventually removed

