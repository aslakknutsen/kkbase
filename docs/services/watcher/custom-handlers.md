# Adding Custom Handlers

This guide shows how to extend kkbase to watch custom resources or CRDs.

## Handler Types

### Core Handlers
Located in `pkg/watchers/handlers/core/`, these handle standard Kubernetes resources that are present in every cluster.

**Examples:** Pod, Node, Service, ConfigMap

### Extension Handlers
Located in `pkg/watchers/handlers/extensions/`, these handle optional resources like CRDs or resources from specific operators.

**Examples:** Istio resources, Prometheus CRDs, cert-manager resources

## Creating a New Extension Handler

### Step 1: Create the Handler File

Create `pkg/watchers/handlers/extensions/myresource_handler.go`:

```go
package extensions

import (
	"context"
	
	"github.com/aslakknutsen/kkbase/pkg/graph"
	"github.com/aslakknutsen/kkbase/pkg/models"
	"github.com/aslakknutsen/kkbase/pkg/watchers"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// MyResourceHandler watches MyResource CRD
type MyResourceHandler struct {
	*watchers.BaseWatcher
	clientset           *kubernetes.Clientset
	dynamicClient       dynamic.Interface
	relationshipBuilder *watchers.RelationshipBuilder
}

// NewMyResourceHandler creates a new handler for MyResource
func NewMyResourceHandler(
	clientset *kubernetes.Clientset,
	graphStore graph.GraphStore,
	logger *zap.Logger,
	informerFactory informers.SharedInformerFactory,
) *MyResourceHandler {
	// Create dynamic client for CRDs
	dynamicClient := dynamic.NewForConfigOrDie(clientset.RESTClient().GetConfig())
	
	// Define the GVR (GroupVersionResource) for your CRD
	gvr := schema.GroupVersionResource{
		Group:    "example.com",
		Version:  "v1",
		Resource: "myresources",
	}
	
	// Create dynamic informer
	dynInformerFactory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(
		dynamicClient,
		informerFactory.ResyncPeriod(),
		"", // namespace (empty = all namespaces)
		nil,
	)
	
	informer := dynInformerFactory.ForResource(gvr).Informer()
	
	handler := &MyResourceHandler{
		BaseWatcher:         watchers.NewBaseWatcher(graphStore, logger, informer),
		clientset:           clientset,
		dynamicClient:       dynamicClient,
		relationshipBuilder: watchers.NewRelationshipBuilder(clientset, graphStore, logger),
	}
	
	// Register event handlers
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    handler.HandleAdd,
		UpdateFunc: handler.HandleUpdate,
		DeleteFunc: handler.HandleDelete,
	})
	
	return handler
}

func (h *MyResourceHandler) HandleAdd(obj interface{}) {
	unstructuredObj, ok := obj.(*unstructured.Unstructured)
	if !ok {
		h.Logger.Error("failed to convert object to unstructured")
		return
	}
	
	h.Logger.Debug("myresource added",
		zap.String("namespace", unstructuredObj.GetNamespace()),
		zap.String("name", unstructuredObj.GetName()),
	)
	
	// Convert to graph node
	node := h.convertToGraphNode(unstructuredObj)
	
	// Create node in graph
	ctx := context.Background()
	if err := h.GraphStore.UpsertNode(
		ctx,
		string(node.Type),
		node.ID,
		node.Properties,
	); err != nil {
		h.Logger.Error("failed to create myresource node",
			zap.Error(err),
			zap.String("name", unstructuredObj.GetName()),
		)
		return
	}
	
	// Create edges to related resources
	h.createRelationships(ctx, unstructuredObj)
}

func (h *MyResourceHandler) HandleUpdate(oldObj, newObj interface{}) {
	h.HandleAdd(newObj)
}

func (h *MyResourceHandler) HandleDelete(obj interface{}) {
	unstructuredObj, ok := obj.(*unstructured.Unstructured)
	if !ok {
		h.Logger.Error("failed to convert object to unstructured")
		return
	}
	
	h.Logger.Debug("myresource deleted",
		zap.String("namespace", unstructuredObj.GetNamespace()),
		zap.String("name", unstructuredObj.GetName()),
	)
	
	ctx := context.Background()
	nodeID := models.GetNodeID("MyResource", unstructuredObj.GetNamespace(), unstructuredObj.GetName())
	
	if err := h.GraphStore.DeleteNode(ctx, "MyResource", nodeID); err != nil {
		h.Logger.Error("failed to delete myresource node",
			zap.Error(err),
			zap.String("name", unstructuredObj.GetName()),
		)
	}
}

func (h *MyResourceHandler) convertToGraphNode(obj *unstructured.Unstructured) *models.GraphNode {
	// Extract properties from the unstructured object
	properties := map[string]interface{}{
		"name":      obj.GetName(),
		"namespace": obj.GetNamespace(),
		"uid":       string(obj.GetUID()),
		"created":   obj.GetCreationTimestamp().String(),
	}
	
	// Extract spec fields
	spec, found, _ := unstructured.NestedMap(obj.Object, "spec")
	if found {
		// Add spec fields you care about
		if host, ok := spec["host"].(string); ok {
			properties["host"] = host
		}
	}
	
	// Extract status fields
	status, found, _ := unstructured.NestedMap(obj.Object, "status")
	if found {
		if phase, ok := status["phase"].(string); ok {
			properties["status"] = phase
		}
	}
	
	// Add labels
	if len(obj.GetLabels()) > 0 {
		properties["labels"] = serializeLabels(obj.GetLabels())
	}
	
	return &models.GraphNode{
		Type:       "MyResource", // Define this in models/types.go
		ID:         models.GetNodeID("MyResource", obj.GetNamespace(), obj.GetName()),
		Properties: properties,
	}
}

func (h *MyResourceHandler) createRelationships(ctx context.Context, obj *unstructured.Unstructured) {
	// Example: Create edge to a Service
	// If your CRD references a service, create an edge
	spec, found, _ := unstructured.NestedMap(obj.Object, "spec")
	if !found {
		return
	}
	
	if serviceName, ok := spec["serviceName"].(string); ok {
		fromID := models.GetNodeID("MyResource", obj.GetNamespace(), obj.GetName())
		toID := models.GetNodeID("Service", obj.GetNamespace(), serviceName)
		
		h.GraphStore.UpsertEdge(
			ctx,
			"MyResource",
			fromID,
			"REFERENCES",
			"Service",
			toID,
			nil,
		)
	}
	
	// Create IN_NAMESPACE edge
	h.relationshipBuilder.CreateNamespaceEdge(
		ctx,
		obj.GetNamespace(),
		"MyResource",
		models.GetNodeID("MyResource", obj.GetNamespace(), obj.GetName()),
	)
}

func serializeLabels(labels map[string]string) string {
	// Similar to serializeMap in models/converters.go
	if len(labels) == 0 {
		return ""
	}
	b, _ := json.Marshal(labels)
	return string(b)
}
```

### Step 2: Register the Handler

For extension/CRD handlers, use `RegisterHandlerFactory` with `ResourceTypeInfo`. Create or update `pkg/watchers/handlers/extensions/myresource/register.go`:

```go
package myresource

import (
	"github.com/aslakknutsen/kkbase/pkg/graph"
	"github.com/aslakknutsen/kkbase/pkg/models"
	"github.com/aslakknutsen/kkbase/pkg/watchers"
	"go.uber.org/zap"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
)

// RegisterMyResourceHandler registers the MyResource CRD handler
func RegisterMyResourceHandler(
	manager *watchers.Manager,
	dynamicClient dynamic.Interface,
	factory dynamicinformer.DynamicSharedInformerFactory,
	graphStore graph.GraphStore,
	logger *zap.Logger,
) {
	logger.Info("registering MyResource handler with CRD watcher")

	// Register with ResourceTypeInfo
	manager.RegisterHandlerFactory(
		watchers.ResourceTypeInfo{
			NodeType:      models.NodeTypeMyResource,
			Kind:          "MyResource",
			APIGroup:      "example.com",
			ClusterScoped: false, // Set true if resource is cluster-scoped
		},
		func() watchers.ResourceWatcher {
			return NewMyResourceHandler(
				dynamicClient,
				graphStore,
				logger,
				factory,
			)
		},
	)

	logger.Info("MyResource handler registered with CRD watcher")
}
```

The `ResourceTypeInfo` struct automatically:
- Registers the NodeType metadata in the global registry
- Determines the correct handler name (e.g., "example.com/myresource")
- Ensures cluster-scope is handled correctly throughout the system

### Step 3: Define Node and Edge Types

Update `pkg/models/types.go` to add the NodeType constant:

```go
// Add to NodeType constants
const (
	// ... existing types ...
	NodeTypeMyResource NodeType = "MyResource"
)

// Add to EdgeType constants if you have custom edges
const (
	// ... existing types ...
	EdgeTypeReferences EdgeType = "REFERENCES"
)
```

**Note:** You don't need to manually register the NodeType metadata anywhere. The `RegisterHandlerFactory` call in Step 2 automatically registers the metadata (Kind, APIGroup, ClusterScoped) into the global registry when your handler is registered.

### Step 4: Enable in Main

Update `cmd/watcher/main.go`:

```go
import (
	// ... existing imports ...
	"github.com/aslakknutsen/kkbase/pkg/watchers/handlers/extensions/myresource"
)

func run() error {
	// ... existing code ...
	
	// Create watcher manager
	watcherManager, err := watchers.NewManager(watchers.Config{
		Clientset:    clientset,
		GraphStore:   graphStore,
		Logger:       logger,
		ResyncPeriod: cfg.ResyncPeriod,
		Namespace:    cfg.Namespace,
	}, k8sConfig)
	if err != nil {
		return fmt.Errorf("failed to create watcher manager: %w", err)
	}

	// Get shared factory and clients
	factory := watcherManager.GetFactory()
	dynamicClient := watcherManager.GetDynamicClient()
	
	// Register core handlers (always enabled)
	core.RegisterCoreHandlers(watcherManager, clientset, factory, graphStore, logger)
	
	// Register extension handlers (conditional)
	if cfg.EnableMyResource {
		myresource.RegisterMyResourceHandler(watcherManager, dynamicClient, factory, graphStore, logger)
	}
	
	// Start watchers
	if err := watcherManager.Start(ctx); err != nil {
		return fmt.Errorf("watcher manager failed: %w", err)
	}
	
	// ... rest of the code ...
}
```

### Step 5: Add Configuration

Update `pkg/config/config.go`:

```go
type Config struct {
	// ... existing fields ...
	EnableMyResource bool `env:"ENABLE_MYRESOURCE" envDefault:"false"`
}
```

Update deployment ConfigMap:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: kkbase-watcher-config
data:
  NEO4J_URI: "bolt://neo4j:7687"
  NEO4J_USERNAME: "neo4j"
  LOG_LEVEL: "info"
  ENABLE_MYRESOURCE: "true"  # Enable MyResource handler
```

## Example: Istio VirtualService Handler

Here's a real example from the codebase for watching Istio VirtualServices:

**Registration** (`pkg/watchers/handlers/extensions/istio/register.go`):

```go
package istio

import (
	"github.com/aslakknutsen/kkbase/pkg/graph"
	"github.com/aslakknutsen/kkbase/pkg/models"
	"github.com/aslakknutsen/kkbase/pkg/watchers"
	"go.uber.org/zap"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
)

func RegisterIstioHandlers(
	manager *watchers.Manager,
	dynamicClient dynamic.Interface,
	factory dynamicinformer.DynamicSharedInformerFactory,
	graphStore graph.GraphStore,
	logger *zap.Logger,
) {
	logger.Info("registering Istio handlers with CRD watcher")

	// Register VirtualService handler
	manager.RegisterHandlerFactory(
		watchers.ResourceTypeInfo{
			NodeType:      models.NodeTypeVirtualService,
			Kind:          "VirtualService",
			APIGroup:      "networking.istio.io",
			ClusterScoped: false,
		},
		func() watchers.ResourceWatcher {
			return NewVirtualServiceHandler(nil, dynamicClient, graphStore, logger, factory)
		},
	)
	
	// Register other Istio handlers...
	
	logger.Info("Istio handlers registered with CRD watcher")
}
```

This pattern:
- Uses `RegisterHandlerFactory` for dynamic CRD detection
- Automatically registers NodeType metadata
- Handler is created only when the CRD is detected in the cluster
- Supports multiple Istio versions gracefully

## Testing

Test your handler:

```bash
# Deploy the CRD and create a test resource
kubectl apply -f myresource-crd.yaml
kubectl apply -f myresource-example.yaml

# Check logs
kubectl logs -f deployment/kkbase-watcher

# Query Neo4j
kubectl exec neo4j-0 -- cypher-shell -u neo4j -p changeme \
  "MATCH (n:MyResource) RETURN n"
```

## Best Practices

1. **Handle Missing CRDs Gracefully** - Check if CRD exists before watching
2. **Use Proper GVR** - Verify Group, Version, Resource names
3. **Extract Meaningful Properties** - Only store properties useful for queries
4. **Create Logical Edges** - Connect to related resources
5. **Test Thoroughly** - Test add, update, delete operations
6. **Document** - Add comments explaining CRD-specific logic
7. **Make Optional** - Extension handlers shouldn't break if CRD is missing

## Troubleshooting

### CRD Not Found
```
Error: no matches for kind "MyResource" in version "example.com/v1"
```
Solution: Verify the GVR and ensure CRD is installed.

### Permission Denied
```
Error: forbidden: User cannot list resource "myresources"
```
Solution: Update ClusterRole to include your CRD:
```yaml
- apiGroups: ["example.com"]
  resources: ["myresources"]
  verbs: ["get", "list", "watch"]
```

### Type Assertion Failures
```
Error: failed to convert object to unstructured
```
Solution: Dynamic informers always return `*unstructured.Unstructured`, use type assertion carefully.

