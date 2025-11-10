# Watcher Service

The Watcher service is the core component that continuously syncs your Kubernetes cluster state to Neo4j as a knowledge graph.

## What It Does

The watcher:
- **Watches** Kubernetes API for resource changes using SharedInformers
- **Converts** K8s resources to graph nodes (Pod, Service, Deployment, etc.)
- **Creates** edges for relationships (manages, selects, routes-to, etc.)
- **Maintains** real-time synchronization with cluster state
- **Supports** core resources and extensions (Gateway API, Istio, Kuadrant)

## When to Use

Deploy the watcher when you want:

- **Queryable cluster topology** - Explore your cluster as a graph
- **Dependency mapping** - Understand resource relationships
- **Impact analysis** - Assess blast radius of changes
- **Foundation for agents** - Provides the knowledge base for AI diagnostics
- **Custom integrations** - Build tools that need cluster topology

## Architecture

```
┌─────────────────────────────────────────────┐
│         Kubernetes API Server               │
│  ┌──────┐  ┌─────────┐  ┌─────────┐       │
│  │ Pods │  │ Services│  │  Nodes  │  ...  │
│  └──────┘  └─────────┘  └─────────┘       │
└──────────────────┬──────────────────────────┘
                   │ Watch (efficient, streaming)
                   ↓
        ┌──────────────────────┐
        │   Watcher Service    │
        │                      │
        │  ┌────────────────┐ │
        │  │ SharedInformers│ │  Local cache
        │  └────────────────┘ │  Reduces API load
        │                      │
        │  ┌────────────────┐ │
        │  │   Handlers     │ │  Per-resource logic
        │  │  - PodHandler  │ │  Convert → Graph
        │  │  - SvcHandler  │ │  Extract relationships
        │  └────────────────┘ │
        └──────────┬───────────┘
                   │ Bolt Protocol
                   ↓
        ┌──────────────────────┐
        │       Neo4j          │
        │  Knowledge Graph     │
        └──────────────────────┘
```

## Key Features

### Real-Time Synchronization

- **Event-driven**: Responds to Add/Update/Delete events instantly
- **No polling**: Uses Kubernetes watch API for efficiency
- **Resync**: Periodic full sync prevents drift
- **Consistency**: Maintains graph integrity during updates

### Resource Coverage

**Core Kubernetes** (always enabled):
- Compute: Node, Namespace
- Workloads: Pod, Container, Deployment, ReplicaSet, StatefulSet, DaemonSet, Job, CronJob
- Networking: Service, Ingress, Endpoint, NetworkPolicy
- Storage: PV, PVC, StorageClass
- Configuration: ConfigMap, Secret
- Observability: K8sEvent

**Extensions** (optional):
- **Gateway API**: GatewayClass, Gateway, HTTPRoute, GRPCRoute, TCPRoute, UDPRoute, TLSRoute, ReferenceGrant
- **Istio**: VirtualService, DestinationRule, Gateway, ServiceEntry, Sidecar, AuthorizationPolicy, PeerAuthentication
- **Kuadrant**: Kuadrant CR, AuthPolicy, RateLimitPolicy, DNSPolicy, TLSPolicy
- **Custom CRDs**: Extensible handler framework

### Relationship Extraction

Automatically discovers and creates edges:

- **Hierarchical**: Deployment → ReplicaSet → Pod → Container
- **Networking**: Service → Pods (label selector), HTTPRoute → Service
- **Storage**: Pod → PVC → PV, PVC → StorageClass
- **Configuration**: Pod → ConfigMap, Pod → Secret
- **Placement**: Pod → Node
- **Ownership**: Owner references tracked

### Handler System

Pluggable architecture for extensibility:

```go
// Core handlers (always enabled)
pkg/watchers/handlers/core/
├── pod.go
├── service.go
├── deployment.go
└── ...

// Extension handlers (optional)
pkg/watchers/handlers/extensions/
├── gateway/        # Gateway API
├── istio/          # Istio service mesh
├── kuadrant/       # Kuadrant API management
└── custom/         # Your CRDs
```

Each handler:
- Watches specific resource type
- Converts to graph representation
- Extracts relationships
- Handles lifecycle events

## Deployment

The watcher runs as a Kubernetes Deployment with:

- **ServiceAccount** with cluster-wide read permissions
- **ClusterRole** granting `get`, `list`, `watch` on resources
- **ConfigMap** for configuration
- **Secret** for Neo4j credentials
- **Health checks** for liveness and readiness

### Quick Deploy

```bash
kubectl apply -f deploy/rbac.yaml
kubectl apply -f deploy/configmap.yaml
kubectl apply -f deploy/secret.yaml
kubectl apply -f deploy/deployment.yaml
```

See [Deployment Guide](deployment.md) for detailed instructions.

## Configuration

Key configuration options:

| Variable | Purpose | Default |
|----------|---------|---------|
| `NEO4J_URI` | Neo4j connection | `bolt://localhost:7687` |
| `NAMESPACE` | Watch specific namespace | `""` (all) |
| `RESYNC_PERIOD` | Full resync interval | `30s` |
| `LOG_LEVEL` | Logging verbosity | `info` |

See [Configuration Guide](configuration.md) for all options.

## Performance

### Efficiency Features

- **Shared Informers**: Single watch per resource type
- **Local Caching**: Reduces API server load
- **Batch Operations**: Multiple graph updates in transactions
- **Retry Logic**: Exponential backoff for failures
- **Connection Pooling**: Reusable Neo4j connections

### Scalability

- Handles **1000s of resources** efficiently
- Memory: ~50-100MB baseline
- CPU: Minimal (event-driven, not polling)
- API load: Single watch stream per resource type

### Resource Usage

Typical consumption:
```
Memory: 50-100MB baseline + ~1KB per resource
CPU: <0.1 core average, <0.5 core during full sync
Network: Minimal (watch API, not REST polling)
```

## Monitoring

### Health Checks

```bash
# Liveness probe
kubectl exec deployment/kkbase-watcher -- curl -f http://localhost:8080/healthz

# Readiness probe (checks Neo4j connection)
kubectl exec deployment/kkbase-watcher -- curl -f http://localhost:8080/ready
```

### Logs

```bash
# Check sync status
kubectl logs deployment/kkbase-watcher | grep "synced"

# See errors
kubectl logs deployment/kkbase-watcher | grep ERROR

# Watch resource sync
kubectl logs -f deployment/kkbase-watcher | grep -E "Pod|Service"
```

### Metrics (Planned)

Future Prometheus metrics:
- Events processed per resource type
- Graph operations (nodes/edges created/updated/deleted)
- Sync lag per resource type
- Error rates and retry counts

## Extension Support

### Gateway API

Automatically enabled when CRDs are detected:

```bash
# Install Gateway API
kubectl apply -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.2.1/standard-install.yaml

# Watcher automatically detects and starts tracking
```

See [Extensions Guide](extensions.md) for details.

### Istio

Automatically enabled when Istio is installed:

```bash
# Install Istio
istioctl install --set profile=demo

# Watcher automatically detects and starts tracking
```

See [Extensions Guide](extensions.md) for details.

### Kuadrant

Automatically enabled when Kuadrant operator is installed:

```bash
# Install Kuadrant operator (example using Helm)
helm repo add kuadrant https://kuadrant.io/helm-charts/
helm install kuadrant-operator kuadrant/kuadrant-operator

# Watcher automatically detects and starts tracking
```

See [Extensions Guide](extensions.md) for details.

### Custom Resources

Add handlers for your CRDs:

1. Create handler in `pkg/watchers/handlers/extensions/`
2. Implement interface
3. Register handler
4. Deploy

See [Custom Handlers Guide](custom-handlers.md) for step-by-step instructions.

## Troubleshooting

### Watcher Not Starting

```bash
# Check deployment status
kubectl get deployment kkbase-watcher

# Check logs
kubectl logs deployment/kkbase-watcher | tail -50

# Common issues:
# - Neo4j not accessible
# - RBAC permissions missing
# - Invalid configuration
```

### Resources Not Syncing

```bash
# Trigger full resync
kubectl rollout restart deployment/kkbase-watcher

# Verify informers synced
kubectl logs deployment/kkbase-watcher | grep "caches synced"

# Check for specific resource errors
kubectl logs deployment/kkbase-watcher | grep -i "pod"
```

### High Memory Usage

```bash
# Check resource count
kubectl exec deployment/kkbase-watcher -- \
  curl -s localhost:8080/metrics | grep resource_count

# Reduce scope by namespace
kubectl set env deployment/kkbase-watcher NAMESPACE=production

# Increase memory limit
kubectl set resources deployment/kkbase-watcher --limits=memory=512Mi
```

See [Troubleshooting Guide](../../guides/operations/troubleshooting.md) for more solutions.

## Security

### RBAC

The watcher requires read-only access:

```yaml
rules:
- apiGroups: [""]
  resources: ["pods", "services", "nodes", ...]
  verbs: ["get", "list", "watch"]  # Read-only
```

No write permissions needed.

### Network Security

```yaml
# NetworkPolicy example
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: kkbase-watcher-policy
spec:
  podSelector:
    matchLabels:
      app: kkbase-watcher
  policyTypes:
  - Egress
  egress:
  - to:
    - podSelector:
        matchLabels:
          app: neo4j
    ports:
    - protocol: TCP
      port: 7687
  - to:  # Kubernetes API
    - namespaceSelector: {}
    ports:
    - protocol: TCP
      port: 443
```

### Secrets Management

```bash
# Neo4j credentials in Secret (not ConfigMap)
kubectl create secret generic kkbase-watcher-secret \
  --from-literal=NEO4J_PASSWORD=your-secure-password
```

## Best Practices

1. **Start with all namespaces** - Get full cluster view, then filter if needed
2. **Monitor sync status** - Check logs for "caches synced" on startup
3. **Use resync period wisely** - 30-60s is typical, shorter increases API load
4. **Resource limits** - Set appropriate memory limits based on cluster size
5. **Health checks** - Enable readiness probe to prevent traffic during sync
6. **Namespace filtering** - Use for large clusters to reduce memory
7. **Extension opt-in** - Only enable extensions you need

## Documentation

- **[Deployment Guide](deployment.md)** - Step-by-step deployment
- **[Configuration](configuration.md)** - All configuration options
- **[Extensions](extensions.md)** - Gateway API, Istio, and Kuadrant support
- **[Custom Handlers](custom-handlers.md)** - Add support for custom CRDs
- **[Architecture Deep Dive](../../development/deep-dive.md)** - Internal implementation

## Quick Links

- [Getting Started](../../getting-started/)
- [System Architecture](../../ARCHITECTURE.md)
- [Query the Graph](../../guides/querying/)
- [Operations Guide](../../guides/operations/)

