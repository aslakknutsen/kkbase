# Configuration Reference

kkbase is configured through environment variables, typically provided via Kubernetes ConfigMap and Secret resources.

## Environment Variables

### Neo4j Connection

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `NEO4J_URI` | Neo4j connection URI (bolt:// or neo4j://) | `bolt://localhost:7687` | No |
| `NEO4J_USERNAME` | Neo4j username | `neo4j` | No |
| `NEO4J_PASSWORD` | Neo4j password | - | **Yes** |
| `NEO4J_DATABASE` | Neo4j database name | `neo4j` | No |

#### Examples

**Local Neo4j:**
```yaml
NEO4J_URI: "bolt://neo4j:7687"
NEO4J_USERNAME: "neo4j"
NEO4J_PASSWORD: "changeme"
```

**Neo4j Aura (Cloud):**
```yaml
NEO4J_URI: "neo4j+s://xxxxx.databases.neo4j.io:7687"
NEO4J_USERNAME: "neo4j"
NEO4J_PASSWORD: "your-aura-password"
```

**Custom Database:**
```yaml
NEO4J_DATABASE: "kubernetes-graph"
```

### Kubernetes Configuration

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `KUBECONFIG` | Path to kubeconfig file (local dev only) | - | No |
| `NAMESPACE` | Namespace to watch (empty = all namespaces) | `""` (all) | No |
| `RESYNC_PERIOD` | How often to resync with Kubernetes API | `30s` | No |

#### Examples

**Watch All Namespaces:**
```yaml
NAMESPACE: ""
```

**Watch Single Namespace:**
```yaml
NAMESPACE: "production"
```

**Custom Resync Period:**
```yaml
RESYNC_PERIOD: "60s"  # Every 60 seconds
```

### Logging

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `LOG_LEVEL` | Logging verbosity | `info` | No |

#### Valid Values

- `debug` - Detailed debugging information
- `info` - General informational messages (default)
- `warn` - Warning messages only
- `error` - Error messages only

#### Example

```yaml
LOG_LEVEL: "debug"
```

### Observability Integration (Planned)

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `ENABLE_METRICS` | Enable metrics integration | `false` | No |
| `ENABLE_LOGS` | Enable logs integration | `false` | No |
| `ENABLE_TRACES` | Enable traces integration | `false` | No |

*Note: These features are planned but not yet implemented.*

## Kubernetes Resources

### ConfigMap

Store non-sensitive configuration in a ConfigMap:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: kkbase-watcher-config
  namespace: default
data:
  NEO4J_URI: "bolt://neo4j:7687"
  NEO4J_USERNAME: "neo4j"
  NEO4J_DATABASE: "neo4j"
  NAMESPACE: ""
  RESYNC_PERIOD: "30s"
  LOG_LEVEL: "info"
  ENABLE_METRICS: "false"
  ENABLE_LOGS: "false"
  ENABLE_TRACES: "false"
```

### Secret

Store sensitive data (passwords) in a Secret:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: kkbase-watcher-secret
  namespace: default
type: Opaque
stringData:
  NEO4J_PASSWORD: "your-neo4j-password"
```

**Security Note:** Never commit passwords to source control. Generate them securely:

```bash
# Generate a random password
openssl rand -base64 32
```

### Deployment

Reference ConfigMap and Secret in your Deployment:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: kkbase-watcher
spec:
  template:
    spec:
      containers:
      - name: kkbase-watcher
        image: kkbase-watcher:latest
        envFrom:
        - configMapRef:
            name: kkbase-watcher-config
        env:
        - name: NEO4J_PASSWORD
          valueFrom:
            secretKeyRef:
              name: kkbase-watcher-secret
              key: NEO4J_PASSWORD
```

## Configuration Examples

### Development Environment

Verbose logging, local Neo4j, watch all namespaces:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: kkbase-watcher-config
data:
  NEO4J_URI: "bolt://localhost:7687"
  NEO4J_USERNAME: "neo4j"
  NAMESPACE: ""
  RESYNC_PERIOD: "10s"
  LOG_LEVEL: "debug"
```

### Production Environment

Standard logging, clustered Neo4j, specific namespace:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: kkbase-watcher-config
data:
  NEO4J_URI: "bolt://neo4j-cluster:7687"
  NEO4J_USERNAME: "neo4j"
  NEO4J_DATABASE: "production-k8s"
  NAMESPACE: "production"
  RESYNC_PERIOD: "60s"
  LOG_LEVEL: "info"
```

### Multi-Tenant Environment

Separate database per cluster, watch all namespaces:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: kkbase-watcher-config
data:
  NEO4J_URI: "bolt://neo4j:7687"
  NEO4J_USERNAME: "neo4j"
  NEO4J_DATABASE: "cluster-east-1"
  NAMESPACE: ""
  RESYNC_PERIOD: "30s"
  LOG_LEVEL: "info"
```

## Local Development

For local development outside Kubernetes:

```bash
export NEO4J_URI="bolt://localhost:7687"
export NEO4J_USERNAME="neo4j"
export NEO4J_PASSWORD="changeme"
export KUBECONFIG="$HOME/.kube/config"
export LOG_LEVEL="debug"
export NAMESPACE=""
export RESYNC_PERIOD="30s"

go run ./cmd/watcher
```

## Configuration Validation

kkbase validates configuration on startup:

### Connection Validation

```bash
kubectl logs deployment/kkbase-watcher | grep "connected to Neo4j"
```

Expected output:
```
INFO  successfully connected to Neo4j  uri=bolt://neo4j:7687
```

### Watcher Registration

```bash
kubectl logs deployment/kkbase-watcher | grep "registered"
```

Expected output:
```
INFO  registered all watchers  count=15
```

### Cache Synchronization

```bash
kubectl logs deployment/kkbase-watcher | grep "synced"
```

Expected output:
```
INFO  all caches synced successfully
```

## Troubleshooting Configuration

### Neo4j Connection Failures

**Error:** `failed to connect to Neo4j`

Check:
1. URI is correct (bolt:// or neo4j://)
2. Neo4j service is running: `kubectl get svc neo4j`
3. Password is correct in Secret
4. Network connectivity: `kubectl exec deployment/kkbase-watcher -- nc -zv neo4j 7687`

### RBAC Permission Errors

**Error:** `forbidden: User cannot list resource`

Check:
1. ClusterRole includes required resources: `kubectl describe clusterrole kkbase-watcher`
2. ClusterRoleBinding is correct: `kubectl describe clusterrolebinding kkbase-watcher`

### Invalid Configuration Values

**Error:** `invalid resync period`

Check:
1. RESYNC_PERIOD uses valid duration format: `30s`, `1m`, `1h`
2. LOG_LEVEL is one of: debug, info, warn, error

## Advanced Configuration

### Custom Namespace Labels

Watch only namespaces with specific labels (requires code modification):

```go
// In cmd/watcher/main.go
informerFactory := informers.NewSharedInformerFactoryWithOptions(
    clientset,
    resyncPeriod,
    informers.WithTweakListOptions(func(options *metav1.ListOptions) {
        options.LabelSelector = "environment=production"
    }),
)
```

### Resource Filtering

Filter specific resource types (modify RBAC ClusterRole):

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: kkbase-watcher
rules:
- apiGroups: [""]
  resources: ["pods", "services"]  # Only these resources
  verbs: ["get", "list", "watch"]
```

## Best Practices

1. **Use Secrets for Passwords** - Never store passwords in ConfigMaps
2. **Start with Debug Logging** - Use `LOG_LEVEL: debug` initially, then reduce to `info`
3. **Namespace Filtering** - Watch specific namespaces in large clusters to reduce load
4. **Reasonable Resync** - 30-60 seconds is typical; shorter periods increase API load
5. **Monitor Logs** - Watch startup logs to ensure proper configuration
6. **Backup Configuration** - Keep ConfigMap and Secret definitions in version control (without passwords)

## See Also

- **[Installation Guide](../user-guide/installation.md)** - Step-by-step deployment
- **[Quick Start](../user-guide/quickstart.md)** - Fast setup guide
- **[Architecture](../development/architecture.md)** - How configuration is used internally

