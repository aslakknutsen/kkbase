# Refactoring Summary: ResourceWatcher.Start() Method

## Problem
Extension handlers (Gateway API, Istio) were not receiving events because their informers were never started.

## Root Cause
Handlers were registered **during cache sync** (before `m.started = true`), so the code to manually start informers never executed. Core handlers worked because they were registered before `Start()` and their informers were started by `factory.Start()`.

## Solution Evolution

### Initial Approach (Reflection-Based)
Used reflection to extract the embedded `BaseWatcher` from handlers and manually start informers:
```go
// Bad: fragile, not idiomatic Go
func (m *Manager) startInformerForHandler(handler ResourceWatcher, name string) {
    handlerVal := reflect.ValueOf(handler)
    // ... reflection magic to find BaseWatcher
    // ... start informer
}
```

### Final Approach (Interface-Based) ✅
Added `Start()` method to the `ResourceWatcher` interface:

```go
type ResourceWatcher interface {
    HandleAdd(obj interface{})
    HandleUpdate(oldObj, newObj interface{})
    HandleDelete(obj interface{})
    Start(ctx context.Context) bool  // NEW!
}
```

## Changes Made

### 1. Updated ResourceWatcher Interface
Added `Start()` method that returns `true` if the informer was started successfully.

### 2. Implemented Start() in BaseWatcher
```go
type BaseWatcher struct {
    GraphStore graph.GraphStore
    Logger     *zap.Logger
    Informer   cache.SharedIndexInformer
    
    started bool      // Track if already started
    mu      sync.Mutex // Protect concurrent access
}

func (b *BaseWatcher) Start(ctx context.Context) bool {
    b.mu.Lock()
    defer b.mu.Unlock()
    
    if b.started || b.Informer == nil {
        return false
    }
    
    go b.Informer.Run(ctx.Done())
    if !cache.WaitForCacheSync(ctx.Done(), b.Informer.HasSynced) {
        return false
    }
    
    b.started = true
    return true
}
```

### 3. Simplified Manager Code
Replaced reflection-based code with clean interface calls:

**Before:**
```go
m.startInformerForHandler(handler, name)  // Uses reflection
```

**After:**
```go
if started := handler.Start(m.ctx); started {
    m.logger.Info("informer started and synced")
} else {
    m.logger.Warn("failed to start informer")
}
```

### 4. Removed Reflection Import
No longer need `reflect` package since we use the interface method directly.

## Benefits

### ✅ Type Safety
- Compile-time checking ensures all handlers implement `Start()`
- No runtime reflection errors

### ✅ Explicitness
- Clear contract: handlers must know how to start themselves
- Self-documenting code

### ✅ Maintainability
- Easy to understand and debug
- No "magic" reflection code

### ✅ Flexibility
- Handlers can customize their start behavior if needed
- BaseWatcher provides default implementation

### ✅ Thread Safety
- Mutex prevents concurrent start attempts
- Idempotent: calling `Start()` multiple times is safe

## Handler Implementation

Since all handlers embed `BaseWatcher`, they automatically get the `Start()` implementation:

```go
type HTTPRouteHandler struct {
    *watchers.BaseWatcher  // Embeds Start() automatically
    dynamicClient       dynamic.Interface
    relationshipBuilder *watchers.RelationshipBuilder
}
```

Handlers can override `Start()` if they need custom behavior, but the default implementation works for all current handlers.

## Testing

- ✅ All existing tests pass
- ✅ Extension handlers (Gateway API, Istio) now receive events
- ✅ Core handlers continue to work as before
- ✅ No RBAC errors (after adding missing `backendtlspolicies` permission)

## Deployment

1. Rebuild watcher binary
2. Apply updated RBAC (includes `backendtlspolicies`)
3. Deploy/restart

## Verification

After deployment, logs show handlers receiving events:
```json
{"level":"debug","msg":"gateway added","namespace":"default","name":"ecommerce-gateway"}
{"level":"debug","msg":"httproute added","namespace":"frontend","name":"web"}
{"level":"debug","msg":"referencegrant added","namespace":"frontend","name":"ecommerce-to-frontend"}
```

## Lessons Learned

1. **Interfaces > Reflection**: When possible, use explicit interfaces rather than reflection
2. **Start Early**: Consider lifecycle methods (Start, Stop) when designing handler interfaces
3. **RBAC Matters**: Don't forget to grant permissions for CRD resources!
4. **Timing is Everything**: Be aware of when handlers register vs when factory starts

## Related Files

- `pkg/watchers/watcher.go` - Main changes
- `deploy/rbac.yaml` - Added backendtlspolicies permission
- All handlers in `pkg/watchers/handlers/*` - Automatically get Start() via embedding

