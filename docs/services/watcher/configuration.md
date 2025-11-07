# Watcher Configuration

Configuration reference for the kkbase Watcher service.

## Configuration Method

The watcher is configured through environment variables, typically provided via Kubernetes ConfigMap and Secret.

## Environment Variables

###Neo4j Connection

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `NEO4J_URI` | Neo4j connection URI (bolt:// or neo4j://) | `bolt://localhost:7687` | No |
| `NEO4J_USERNAME` | Neo4j username | `neo4j` | No |
| `NEO4J_PASSWORD` | Neo4j password | - | **Yes** |
| `NEO4J_DATABASE` | Neo4j database name | `neo4j` | No |

**Examples**:

```yaml
# Local Neo4j
NEO4J_URI: "bolt://neo4j:7687"
NEO4J_USERNAME: "neo4j"
NEO4J_PASSWORD: "changeme"

# Neo4j Aura (Cloud)
NEO4J_URI: "neo4j+s://xxxxx.databases.neo4j.io:7687"
NEO4J_USERNAME: "neo4j"
NEO4J_PASSWORD: "your-aura-password"

# Custom Database
NEO4J_DATABASE: "kubernetes-prod"
```

### Kubernetes Configuration

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `KUBECONFIG` | Path to kubeconfig file (local dev only) | - | No |
| `NAMESPACE` | Namespace to watch (empty = all namespaces) | `""` (all) | No |
| `RESYNC_PERIOD` | How often to resync with Kubernetes API | `30s` | No |

**Examples**:

```yaml
# Watch all namespaces
NAMESPACE: ""

# Watch single namespace
NAMESPACE: "production"

# Custom resync period
RESYNC_PERIOD: "60s"  # Every 60 seconds
```

### Logging

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `LOG_LEVEL` | Logging verbosity | `info` | No |

**Valid Values**: `debug`, `info`, `warn`, `error`

**Examples**:

```yaml
# Debug logging (verbose)
LOG_LEVEL: "debug"

# Production logging
LOG_LEVEL: "info"

# Minimal logging
LOG_LEVEL: "error"
```

### Observability Integration

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `ENABLE_TRACES` | Enable distributed tracing (Jaeger) | `false` | No |
| `JAEGER_QUERY_URL` | Jaeger Query API URL | `http://localhost:16686` | No |
| `JAEGER_POLL_INTERVAL` | How often to poll for new traces | `30s` | No |
| `JAEGER_LOOKBACK_WINDOW` | How far back to look for traces | `5m` | No |
| `JAEGER_SPAN_RETENTION` | How long to keep spans in graph | `1h` | No |

**Jaeger Integration Example**:

```yaml
ENABLE_TRACES: "true"
JAEGER_QUERY_URL: "http://jaeger-query.observability:16686"
JAEGER_POLL_INTERVAL: "30s"
JAEGER_LOOKBACK_WINDOW: "5m"
JAEGER_SPAN_RETENTION: "24h"
```

**Notes**:
- Watcher discovers services from graph and queries Jaeger for traces
- Spans older than retention are auto-cleaned every 10 minutes
- Only CLIENT and PRODUCER spans generate CALLS/FAILED_CALL_TO edges
- All spans create ORIGINATED_FROM relationships to Services

## Kubernetes Resources

### ConfigMap

Store non-sensitive configuration:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: kkbase-watcher-config
  namespace: default
data:
  # Neo4j
  NEO4J_URI: "bolt://neo4j:7687"
  NEO4J_USERNAME: "neo4j"
  NEO4J_DATABASE: "neo4j"
  
  # Kubernetes
  NAMESPACE: ""
  RESYNC_PERIOD: "30s"
  LOG_LEVEL: "info"
  
  # Observability (optional)
  ENABLE_TRACES: "true"
  JAEGER_QUERY_URL: "http://jaeger-query:16686"
  JAEGER_POLL_INTERVAL: "30s"
  JAEGER_LOOKBACK_WINDOW: "5m"
  JAEGER_SPAN_RETENTION: "24h"
```

### Secret

Store sensitive data (passwords):

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: kkbase-watcher-secret
  namespace: default
type: Opaque
stringData:
  NEO4J_PASSWORD: "your-secure-password"
```

**Security Note**: Never commit passwords to source control.

### Deployment

Reference ConfigMap and Secret:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: kkbase-watcher
spec:
  template:
    spec:
      containers:
      - name: watcher
        image: kkbase-watcher:latest
        
        # Load all non-sensitive config
        envFrom:
        - configMapRef:
            name: kkbase-watcher-config
        
        # Load password from Secret
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

### Multi-Cluster Environment

Separate database per cluster:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: kkbase-watcher-config
data:
  NEO4J_URI: "bolt://neo4j:7687"
  NEO4J_USERNAME: "neo4j"
  NEO4J_DATABASE: "cluster-us-west-2"
  NAMESPACE: ""
  RESYNC_PERIOD: "30s"
  LOG_LEVEL: "info"
```

### With Tracing Integration

Full observability stack:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: kkbase-watcher-config
data:
  # Neo4j
  NEO4J_URI: "bolt://neo4j:7687"
  NEO4J_USERNAME: "neo4j"
  
  # Kubernetes
  NAMESPACE: ""
  RESYNC_PERIOD: "30s"
  LOG_LEVEL: "info"
  
  # Distributed Tracing
  ENABLE_TRACES: "true"
  JAEGER_QUERY_URL: "http://jaeger-query.observability:16686"
  JAEGER_POLL_INTERVAL: "30s"
  JAEGER_LOOKBACK_WINDOW: "5m"
  JAEGER_SPAN_RETENTION: "24h"
```

## Local Development

For local development outside Kubernetes:

```bash
# Create .env file
cat > .env <<EOF
# Neo4j
NEO4J_URI=bolt://localhost:7687
NEO4J_USERNAME=neo4j
NEO4J_PASSWORD=changeme

# Kubernetes
KUBECONFIG=$HOME/.kube/config
NAMESPACE=""
RESYNC_PERIOD=30s

# Logging
LOG_LEVEL=debug

# Tracing (optional)
ENABLE_TRACES=true
JAEGER_QUERY_URL=http://localhost:16686
EOF

# Load and run
source .env
go run ./cmd/watcher
```

## Configuration Validation

### Startup Checks

Watcher validates configuration on startup:

```bash
kubectl logs deployment/kkbase-watcher | head -20
```

Expected output:
```
INFO  Loading configuration from environment
INFO  successfully connected to Neo4j  uri=bolt://neo4j:7687
INFO  registered all watchers  count=15
INFO  watcher started successfully
INFO  all caches synced successfully
```

### Connection Test

```bash
# Test Neo4j connectivity
kubectl exec deployment/kkbase-watcher -- nc -zv neo4j 7687

# Check health
kubectl exec deployment/kkbase-watcher -- curl -f http://localhost:8080/healthz

# Check readiness
kubectl exec deployment/kkbase-watcher -- curl -f http://localhost:8080/ready
```

### Trace Processing Test

```bash
# Check trace integration logs
kubectl logs deployment/kkbase-watcher | grep -i trace
```

Expected (if ENABLE_TRACES=true):
```
INFO  started Jaeger trace polling
INFO  starting trace stream  service_count=5
DEBUG processed trace  trace_id=xxx
```

## Troubleshooting

### Neo4j Connection Failed

**Error**: `failed to connect to Neo4j`

**Check**:
1. URI is correct (bolt:// or neo4j://)
2. Neo4j service is running: `kubectl get svc neo4j`
3. Password is correct in Secret
4. Network connectivity: `kubectl exec deployment/kkbase-watcher -- nc -zv neo4j 7687`

### Invalid Configuration Value

**Error**: `invalid resync period`

**Solution**: Use valid Go duration format:
- `30s` (30 seconds)
- `1m` (1 minute)
- `1h` (1 hour)

### RBAC Permission Errors

**Error**: `forbidden: User cannot list resource`

**Solution**: Verify ClusterRole includes required resources:

```bash
kubectl describe clusterrole kkbase-watcher
kubectl describe clusterrolebinding kkbase-watcher
```

### Trace Processing Not Working

**Symptom**: No spans in Neo4j, no trace logs

**Check**:
1. `ENABLE_TRACES` is `"true"` (string, not boolean)
2. Jaeger URL is accessible: `kubectl exec deployment/kkbase-watcher -- curl -s http://jaeger-query:16686/api/services`
3. Check logs: `kubectl logs deployment/kkbase-watcher | grep -i trace`

## Performance Tuning

### Resync Period

Balance between freshness and API load:

```yaml
# More API load, fresher data
RESYNC_PERIOD: "10s"

# Recommended for most clusters
RESYNC_PERIOD: "30s"

# Lower API load for large clusters
RESYNC_PERIOD: "60s"
```

### Namespace Filtering

Reduce memory and processing for large clusters:

```yaml
# Watch only production
NAMESPACE: "production"

# Watch only specific namespaces (requires multiple deployments)
# Deploy separate watchers with different NAMESPACE values
```

### Log Level

Adjust based on needs:

```yaml
# Development - see everything
LOG_LEVEL: "debug"

# Production - standard
LOG_LEVEL: "info"

# Quiet - errors only
LOG_LEVEL: "error"
```

## Best Practices

1. **Use Secrets for passwords** - Never store in ConfigMaps
2. **Start with debug logging** - Use `LOG_LEVEL: debug` initially, then reduce
3. **Namespace filtering in production** - Reduce load in large clusters
4. **Reasonable resync** - 30-60s typical; shorter increases API load
5. **Monitor logs** - Watch startup for "caches synced" message
6. **Version control** - Store ConfigMap/Secret definitions (without passwords)
7. **Test changes** - Update ConfigMap, restart deployment, verify logs

## See Also

- **[Deployment Guide](deployment.md)** - Step-by-step deployment
- **[Complete Configuration Reference](../../reference/configuration.md)** - All services
- **[Operations Guide](../../guides/operations/)** - Monitoring and troubleshooting
- **[Architecture](../../ARCHITECTURE.md)** - System overview

