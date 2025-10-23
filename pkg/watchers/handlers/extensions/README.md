# Extension Handlers

This directory contains handlers for non-core Kubernetes resources, typically from:
- Custom Resource Definitions (CRDs)
- Operators and add-ons (Istio, Prometheus, cert-manager, etc.)
- Platform-specific resources

## Adding a New Extension Handler

### 1. Create the Handler

Create a new file `<resource>_handler.go`:

```go
package extensions

import (
	"context"
	"fmt"

	"github.com/kagenti/kkbase/pkg/graph"
	"github.com/kagenti/kkbase/pkg/models"
	"github.com/kagenti/kkbase/pkg/watchers"
	"go.uber.org/zap"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// ExampleCRDHandler handles ExampleCRD resources
type ExampleCRDHandler struct {
	*watchers.BaseWatcher
	clientset           *kubernetes.Clientset
	relationshipBuilder *watchers.RelationshipBuilder
}

// NewExampleCRDHandler creates a new handler
func NewExampleCRDHandler(
	clientset *kubernetes.Clientset,
	graphStore graph.GraphStore,
	logger *zap.Logger,
	informerFactory informers.SharedInformerFactory,
) *ExampleCRDHandler {
	// Use a dynamic informer for CRDs
	// informer := dynamicInformerFactory.ForResource(schema.GroupVersionResource{
	//     Group: "example.com",
	//     Version: "v1",
	//     Resource: "examplecrds",
	// }).Informer()
	
	handler := &ExampleCRDHandler{
		// BaseWatcher: watchers.NewBaseWatcher(graphStore, logger, informer),
		clientset:           clientset,
		relationshipBuilder: watchers.NewRelationshipBuilder(clientset, graphStore, logger),
	}

	// Register event handlers
	// informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
	//     AddFunc:    handler.HandleAdd,
	//     UpdateFunc: handler.HandleUpdate,
	//     DeleteFunc: handler.HandleDelete,
	// })

	return handler
}

func (h *ExampleCRDHandler) HandleAdd(obj interface{}) {
	// Convert obj to your CRD type
	// Extract relevant properties
	// Create graph node
	// Create edges to related resources
}

func (h *ExampleCRDHandler) HandleUpdate(oldObj, newObj interface{}) {
	// Update graph node
}

func (h *ExampleCRDHandler) HandleDelete(obj interface{}) {
	// Delete graph node
}
```

### 2. Register the Handler

Create or update `register.go`:

```go
package extensions

import (
	"github.com/kagenti/kkbase/pkg/watchers/handlers"
)

func RegisterExtensionHandlers(registry *handlers.Registry) {
	registry.Register(&handlers.HandlerRegistration{
		Name:        "examplecrd",
		Description: "Watches ExampleCRD resources from example.com/v1",
		Category:    "extensions",
		Required:    false, // Extension handlers are usually optional
		Factory: func(clientset, graphStore, logger, informerFactory) {
			return NewExampleCRDHandler(clientset, graphStore, logger, informerFactory)
		},
	})
}
```

### 3. Enable in Main

In `cmd/watcher/main.go`, conditionally register extension handlers:

```go
// Register core handlers (always enabled)
core.RegisterCoreHandlers(handlerRegistry)

// Register extension handlers (can be conditional)
if cfg.EnableExtensions {
    extensions.RegisterExtensionHandlers(handlerRegistry)
}
```

## Example Extensions

### Istio Resources
- `VirtualService`, `DestinationRule`, `Gateway`
- Track service mesh configuration and routing

### Prometheus Resources
- `ServiceMonitor`, `PrometheusRule`, `Alertmanager`
- Track observability configuration

### cert-manager Resources
- `Certificate`, `Issuer`, `ClusterIssuer`
- Track TLS certificate management

### Argo CD Resources
- `Application`, `AppProject`
- Track GitOps deployments

## Best Practices

1. **Make handlers optional** - Extension handlers should not fail if CRDs aren't installed
2. **Check for CRD existence** - Verify CRD is available before watching
3. **Use dynamic informers** - For CRDs, use `dynamicinformer.DynamicSharedInformerFactory`
4. **Document relationships** - Clearly document what edges your handler creates
5. **Handle version changes** - CRDs can have multiple versions

## Configuration

Extension handlers can be enabled/disabled via:
- Environment variables: `ENABLE_ISTIO_HANDLERS=true`
- ConfigMap settings
- Command-line flags

Example:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: kkbase-watcher-config
data:
  ENABLE_ISTIO_HANDLERS: "true"
  ENABLE_PROMETHEUS_HANDLERS: "true"
  ENABLE_CERT_MANAGER_HANDLERS: "false"
```

