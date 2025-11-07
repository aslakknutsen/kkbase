# Extending kkbase

Extend kkbase with new handlers, MCP tools, and functionality.

## Adding Handlers

### Overview

Handlers watch Kubernetes resources and sync them to Neo4j as graph nodes and relationships.

### Step-by-Step

See detailed guide: [Custom Handlers](../services/watcher/custom-handlers.md)

Quick summary:

1. **Create handler file**: `pkg/watchers/handlers/myresource_handler.go`
2. **Implement Handler interface**:
   ```go
   type Handler interface {
       Type() string
       Process(event Event) error
   }
   ```
3. **Register handler**: In `handlers/registry.go`
4. **Add to ConfigMap**: Enable in watcher configuration
5. **Update RBAC**: Add permissions in `rbac.yaml`

### Example

```go
package handlers

type MyResourceHandler struct {
    graphStore graph.Store
}

func (h *MyResourceHandler) Type() string {
    return "MyResource"
}

func (h *MyResourceHandler) Process(event Event) error {
    // Convert resource to graph nodes/edges
    // Write to Neo4j via graphStore
    return nil
}
```

## Adding MCP Tools

### Overview

MCP tools expose functionality to AI agents via the MCP protocol.

### Step-by-Step

1. **Define tool** in `pkg/mcp/tools.go`:
   ```go
   {
       Name: "my_tool",
       Description: "Does something useful",
       InputSchema: json.RawMessage(`{...}`),
   }
   ```

2. **Implement handler** in `pkg/mcp/server.go`:
   ```go
   case "my_tool":
       return s.handleMyTool(params.Arguments)
   ```

3. **Add handler method**:
   ```go
   func (s *Server) handleMyTool(args json.RawMessage) (interface{}, error) {
       // Parse arguments
       // Execute logic
       // Return result
   }
   ```

4. **Add tests** in `pkg/mcp/tools_test.go`

5. **Update documentation**:
   - Add to [Tools Reference](../services/mcp-server/tools-reference.md)

### Example

```go
func (s *Server) handleMyTool(args json.RawMessage) (interface{}, error) {
    var input struct {
        Param1 string `json:"param1"`
    }
    if err := json.Unmarshal(args, &input); err != nil {
        return nil, err
    }

    // Execute logic
    result := doSomething(input.Param1)

    return result, nil
}
```

## Modifying Graph Schema

### Adding Node Types

1. **Update handler** to create new node type
2. **Define properties** in node creation
3. **Update schema doc**: [Graph Schema](../reference/graph-schema.md)

Example:
```go
node := map[string]interface{}{
    "id":   "MyResource/namespace/name",
    "name": resource.Name,
    "type": "MyResource",
    // ... other properties
}
err := h.graphStore.CreateNode("MyResource", node)
```

### Adding Relationship Types

1. **Update handler** to create relationships
2. **Define relationship properties**
3. **Update schema doc**

Example:
```go
err := h.graphStore.CreateRelationship(
    fromID, toID,
    "MY_RELATIONSHIP",
    map[string]interface{}{"weight": 1},
)
```

## Adding Agent Capabilities

### Custom Investigation Logic

Extend agent investigation engine:

1. **Add prompt templates** in `pkg/llm/prompts.go`
2. **Add investigation patterns**
3. **Update agent workflow**

### Custom Reporters

Add new reporting channels:

1. **Implement Reporter interface**
2. **Register reporter**
3. **Configure in agent

**

## Adding Frontend Features

### Dashboard Components

1. **Create component**: `frontend/src/components/MyComponent.tsx`
2. **Add to App**: Import and use in `App.tsx`
3. **Style**: Update CSS or Tailwind classes

### SSE Events

Listen for new event types:

1. **Update MCP server** to emit events
2. **Update frontend** `mcpObserver.ts` to handle events
3. **Update components** to respond

## Adding Prometheus Metrics

### In Go Services

```go
import "github.com/prometheus/client_golang/prometheus"

var myMetric = prometheus.NewCounterVec(
    prometheus.CounterOpts{
        Name: "kkbase_my_metric_total",
        Help: "My custom metric",
    },
    []string{"label"},
)

func init() {
    prometheus.MustRegister(myMetric)
}

// Use in code
myMetric.WithLabelValues("value").Inc()
```

## Testing Extensions

### Unit Tests

```go
func TestMyHandler(t *testing.T) {
    mockStore := &MockGraphStore{}
    handler := NewMyResourceHandler(mockStore)

    err := handler.Process(event)
    assert.NoError(t, err)
}
```

### Integration Tests

```go
// +build integration

func TestMyHandlerIntegration(t *testing.T) {
    // Test with real Neo4j
}
```

## Best Practices

1. **Follow Go conventions** - Standard naming, formatting
2. **Write tests** - Unit and integration tests
3. **Update documentation** - Keep docs in sync
4. **Handle errors** - Proper error handling and logging
5. **Use interfaces** - For testability
6. **Add logging** - Use structured logging
7. **Consider idempotency** - Handlers should be idempotent

## Examples in Codebase

### Existing Handlers

Study these for patterns:
- `pod_handler.go` - Basic handler
- `service_handler.go` - Relationships
- `deployment_handler.go` - Hierarchies
- `virtualservice_handler.go` - CRDs

### Existing MCP Tools

Study these for patterns:
- `query` - Simple tool
- `query_with_session` - Complex tool with side effects
- `spawn_investigation` - Async operations

## See Also

- [Development README](README.md)
- [Building Guide](building.md)
- [Deep Dive](deep-dive.md)
- [Custom Handlers Guide](../services/watcher/custom-handlers.md)
- [Handler Development](adding-handlers.md) (archive)

