# kkbase Architecture

## Overview

kkbase is a Kubernetes Knowledge Graph Watcher that continuously syncs cluster state to a Neo4j graph database, creating a living model of your infrastructure.

## Core Architecture

### Handler Registry Pattern

The system uses a pluggable handler registry pattern that separates core Kubernetes resources from optional extensions:

```
pkg/watchers/handlers/
├── registry.go              # Handler registration and management
├── core/                    # Core K8s resource handlers
│   ├── register.go          # Core handler registration
│   ├── namespace.go
│   ├── node.go
│   ├── pod.go
│   ├── deployment.go
│   ├── replicaset.go
│   ├── service.go
│   ├── pv.go
│   ├── pvc.go
│   ├── configmap.go
│   └── event.go
└── extensions/              # Optional/CRD handlers
    ├── README.md            # Extension development guide
    └── register.go          # Extension handler registration
```

### Handler Categories

Handlers are organized into logical categories:

1. **core** - Essential cluster resources (Node, Namespace)
2. **workloads** - Pod controllers (Deployment, ReplicaSet, Pod)
3. **networking** - Network resources (Service, Ingress)
4. **storage** - Persistent storage (PV, PVC, StorageClass)
5. **configuration** - Config and secrets (ConfigMap, Secret)
6. **observability** - Events and monitoring
7. **extensions** - CRDs and add-on resources (Istio, Prometheus, etc.)

### Resource Identification

All Kubernetes resources are identified using the format:

```
kind/namespace/name     # For namespaced resources
kind/name               # For cluster-scoped resources
```

Examples:
- `Pod/default/nginx-abc123`
- `Service/kube-system/metrics-server`
- `Node/worker-node-1`
- `Container/default/nginx-abc123/app`

This ensures globally unique identifiers that clearly represent the resource hierarchy.

## Component Layers

### 1. Graph Database Layer (`pkg/graph/`)

Abstraction layer for graph database operations:

```go
type GraphStore interface {
    UpsertNode(ctx, nodeType, id, properties) error
    DeleteNode(ctx, nodeType, id) error
    UpsertEdge(ctx, fromType, fromID, edgeType, toType, toID, properties) error
    DeleteEdge(ctx, edgeID) error
    DeleteEdgesByNode(ctx, nodeType, id) error
    HealthCheck(ctx) error
}
```

**Implementation:**
- `neo4j/neo4j.go` - Neo4j driver with connection pooling, retries, and health checks

#### Create-on-Write Pattern: Handling Out-of-Order Data

Kubernetes resources often reference each other (e.g., a Pod references a ConfigMap), but these references may arrive out of order during the initial sync or cluster events. The Create-on-Write pattern ensures relationships are never lost.

**The Problem:**
- Pod arrives first, referencing ConfigMap `app-config`
- ConfigMap `app-config` hasn't been processed yet
- Without special handling, the relationship would be dropped

**The Solution: MERGE with Placeholder Nodes**

When creating an edge, both nodes are created using Cypher's `MERGE` operation:

```cypher
MERGE (from:Pod {id: $fromID})
ON CREATE SET 
    from.placeholder = true,
    from.created_at = timestamp(),
    from.updated_at = $updated_at
MERGE (to:ConfigMap {id: $toID})
ON CREATE SET 
    to.placeholder = true,
    to.created_at = timestamp(),
    to.updated_at = $updated_at
MERGE (from)-[r:USES_CONFIG]->(to)
SET r += $properties
```

**Placeholder Lifecycle:**

1. **Creation**: Node is created with `placeholder: true` when referenced but not yet seen
2. **Enrichment**: When full object data arrives, `UpsertNode` updates all properties and sets `placeholder: false`
3. **Cleanup**: Orphaned placeholders (older than 1 hour with no relationships) are automatically removed

**Monitoring:**

The system monitors placeholder nodes every 20 minutes and logs:
- Count of placeholder nodes per type
- Cleanup of orphaned placeholders

**Query Considerations:**

When querying the graph, you may want to filter placeholders:

```cypher
// Exclude placeholder nodes
MATCH (s:Service)
WHERE s.placeholder <> true OR s.placeholder IS NULL
RETURN s

// Include placeholder status in results
MATCH (s:Service)
RETURN s.id, s.name, COALESCE(s.placeholder, false) as is_placeholder
```

**Benefits:**
- Idempotent operations
- No data loss from out-of-order events
- Self-healing: placeholders automatically enriched when data arrives
- Observable: metrics track unresolved references

### 2. Models Layer (`pkg/models/`)

Defines the graph schema:

**Node Types:**
- Compute: `Node`, `Cluster`
- Workloads: `Pod`, `Container`, `Deployment`, `ReplicaSet`, `StatefulSet`, `DaemonSet`, `Job`
- Networking: `Service`, `Ingress`, `Endpoint`, `NetworkPolicy`
- Storage: `PersistentVolume`, `PersistentVolumeClaim`, `StorageClass`
- Configuration: `ConfigMap`, `Secret`, `Namespace`
- Observability: `K8sEvent`

**Edge Types:**
- `SCHEDULED_ON` - Pod → Node
- `MANAGES` - Deployment → ReplicaSet → Pod
- `CONTAINS` - Pod → Container
- `SELECTS_PODS` - Service → Pod
- `ROUTES_TO` - Ingress → Service
- `BOUND_TO` - PVC → PV
- `MOUNTS` - Pod → PVC
- `USES_CONFIG` - Pod → ConfigMap
- `USES_SECRET` - Pod → Secret
- `IN_NAMESPACE` - Resource → Namespace
- `INVOLVES` - Event → Resource

### 3. Watcher Framework (`pkg/watchers/`)

Generic watching infrastructure built on `client-go`:

**Manager** - Coordinates multiple watchers:
- Creates `SharedInformerFactory` for efficient resource watching
- Manages watcher lifecycle
- Handles graceful shutdown

**BaseWatcher** - Common functionality for all watchers:
- Graph store access
- Logging
- Informer management

**RelationshipBuilder** - Extracts and creates edges:
- Label selector resolution (Service → Pods)
- Owner reference traversal (Deployment → ReplicaSet → Pod)
- Volume mount parsing (Pod → PVC/ConfigMap)
- Environment variable resolution (Pod → ConfigMap/Secret)

### 4. Handler Layer (`pkg/watchers/handlers/`)

Each handler watches a specific resource type and:
1. Converts K8s objects to graph nodes
2. Extracts relationships and creates edges
3. Handles Add/Update/Delete events
4. Maintains graph consistency

**Handler Interface:**
```go
type ResourceWatcher interface {
    Start(ctx context.Context) error
}
```

Each handler implements:
- `HandleAdd(obj interface{})`
- `HandleUpdate(oldObj, newObj interface{})`
- `HandleDelete(obj interface{})`

### 5. Configuration (`pkg/config/`)

Configuration via environment variables:

- `NEO4J_URI` - Neo4j connection URI
- `NEO4J_USERNAME` - Database username
- `NEO4J_PASSWORD` - Database password
- `LOG_LEVEL` - Logging level (debug, info, warn, error)
- `RESYNC_PERIOD` - Informer resync interval
- `NAMESPACE` - Namespace to watch (empty = all)

## Data Flow

```
┌─────────────┐
│ Kubernetes  │
│ API Server  │
└──────┬──────┘
       │ watch
       ↓
┌──────────────┐
│   Informer   │  (client-go SharedInformer)
│    Cache     │
└──────┬───────┘
       │ events
       ↓
┌──────────────┐
│   Handler    │  (PodHandler, ServiceHandler, etc.)
│              │
│ • Convert    │  K8s Object → GraphNode
│ • Extract    │  Relationships → GraphEdges
└──────┬───────┘
       │
       ↓
┌──────────────┐
│  GraphStore  │  (Neo4j implementation)
│              │
│ • UpsertNode │
│ • UpsertEdge │
└──────┬───────┘
       │
       ↓
  ┌─────────┐
  │  Neo4j  │
  │Database │
  └─────────┘
```

## Extensibility

### Adding New Handlers

1. **For Core Resources**: Add to `pkg/watchers/handlers/core/`
2. **For Extensions**: Add to `pkg/watchers/handlers/extensions/`

Example extension handler registration:

```go
// pkg/watchers/handlers/extensions/istio.go
func RegisterIstioHandlers(registry *handlers.Registry) {
    registry.Register(&handlers.HandlerRegistration{
        Name:        "virtualservice",
        Description: "Watches Istio VirtualService resources",
        Category:    "extensions",
        Required:    false,
        Factory: func(clientset, graphStore, logger, informerFactory) watchers.ResourceWatcher {
            return NewVirtualServiceHandler(clientset, graphStore, logger, informerFactory)
        },
    })
}
```

Enable in `cmd/watcher/main.go`:

```go
core.RegisterCoreHandlers(handlerRegistry)

if cfg.EnableIstio {
    extensions.RegisterIstioHandlers(handlerRegistry)
}
```

### Conditional Handler Registration

Filter handlers by category, name, or custom criteria:

```go
// Only core and workload handlers
watchers := handlerRegistry.InstantiateFiltered(
    clientset, graphStore, logger, informerFactory,
    func(reg *handlers.HandlerRegistration) bool {
        return reg.Category == "core" || reg.Category == "workloads"
    },
)
```

## Deployment

### In-Cluster Deployment

The watcher runs as a Deployment with:
- **ServiceAccount** with cluster-wide read permissions
- **ClusterRole** granting `get`, `list`, `watch` on all resource types
- **ConfigMap** for configuration
- **Secret** for Neo4j credentials
- **Health checks** on `/healthz` (liveness) and `/ready` (readiness)

### Neo4j Deployment

Neo4j is deployed via Helm:
```bash
helm install neo4j neo4j/neo4j \
  --set neo4j.name=neo4j \
  --set neo4j.password=changeme \
  --set volumes.data.mode=defaultStorageClass
```

## Observability

### Health Checks

- **`/healthz`** - Liveness probe (is the process running?)
- **`/ready`** - Readiness probe (can it connect to Neo4j?)

### Logging

Structured logging with `zap`:
- Request tracing
- Resource context (namespace, name, kind)
- Error stack traces
- Performance metrics

### Metrics (Planned)

Future Prometheus metrics:
- Events processed per resource type
- Graph operations (nodes/edges created/updated/deleted)
- Sync lag per resource type
- Error rates and retry counts

## Performance

### Efficiency Features

1. **Shared Informers** - Single watch per resource type across all namespaces
2. **Local Caching** - Informers maintain in-memory cache
3. **Batch Operations** - Multiple graph updates in single transaction
4. **Retry Logic** - Exponential backoff for failed operations
5. **Connection Pooling** - Reusable Neo4j connections

### Scalability

- Handles clusters with 1000s of resources
- Low memory footprint (~50-100MB baseline)
- Minimal API server load (uses watch, not poll)
- Horizontal scaling (future): Multiple instances with leader election

## Security

- **RBAC**: Minimal permissions (read-only cluster access)
- **Secrets**: Neo4j credentials stored in Kubernetes Secrets
- **TLS**: Supports TLS for Neo4j connections
- **No privilege escalation**: Runs as non-root user

## Future Enhancements

1. **Dynamic CRD Discovery** - Automatically detect and watch new CRDs
2. **Observability Integration** - Prometheus, Grafana, Loki
3. **Graph Analytics** - Built-in queries for common patterns
4. **Change Detection** - Track resource changes over time
5. **Multi-Cluster** - Federated graphs across clusters
6. **AI Agent Interface** - Natural language query interface

