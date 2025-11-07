# MCP Server Configuration

Configuration reference for the kkbase MCP Server.

## Configuration Method

The MCP server is configured through environment variables, typically provided via Kubernetes ConfigMap and Secret.

## Environment Variables

### Neo4j Connection

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `NEO4J_URI` | Neo4j connection URI | `bolt://localhost:7687` | No |
| `NEO4J_USERNAME` | Neo4j username | `neo4j` | No |
| `NEO4J_PASSWORD` | Neo4j password | - | **Yes** |
| `NEO4J_DATABASE` | Neo4j database name | `neo4j` | No |

**Examples**:

```yaml
# Local Neo4j
NEO4J_URI: "bolt://neo4j:7687"

# Neo4j Aura (Cloud)
NEO4J_URI: "neo4j+s://xxxxx.databases.neo4j.io:7687"

# Custom database
NEO4J_DATABASE: "kubernetes-prod"
```

### MCP Server

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `MCP_PORT` | HTTP server port | `8080` | No |
| `MCP_ENABLED` | Enable MCP server (integrated mode) | `false` | No |

**Examples**:

```yaml
# Standalone mode (always enabled)
MCP_PORT: "8080"

# Integrated mode (must enable)
MCP_ENABLED: "true"
MCP_PORT: "8080"

# Custom port
MCP_PORT: "8081"
```

### Prometheus Integration

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `PROMETHEUS_URL` | Prometheus server URL | - | No |

**When configured**, enables investigation tools:
- `spawn_investigation`
- `get_investigation_status`
- `complete_investigation`

**Example**:

```yaml
# Enable metrics investigation tools
PROMETHEUS_URL: "http://prometheus-kube-prometheus-prometheus.monitoring.svc:9090"
```

**Note**: Without Prometheus, only core and agent session tools are available.

### Logging

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `LOG_LEVEL` | Logging verbosity | `info` | No |

**Valid Values**: `debug`, `info`, `warn`, `error`

**Examples**:

```yaml
# Debug logging
LOG_LEVEL: "debug"

# Production logging
LOG_LEVEL: "info"
```

## Kubernetes Resources

### ConfigMap (Standalone Mode)

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: kkbase-mcp-server-config
  namespace: default
data:
  # Neo4j
  NEO4J_URI: "bolt://neo4j:7687"
  NEO4J_USERNAME: "neo4j"
  NEO4J_DATABASE: "neo4j"
  
  # MCP Server
  MCP_PORT: "8080"
  LOG_LEVEL: "info"
  
  # Prometheus (optional)
  PROMETHEUS_URL: "http://prometheus.monitoring.svc:9090"
```

### ConfigMap (Integrated Mode)

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: kkbase-config
  namespace: default
data:
  # Neo4j
  NEO4J_URI: "bolt://neo4j:7687"
  NEO4J_USERNAME: "neo4j"
  NEO4J_DATABASE: "neo4j"
  
  # Watcher
  NAMESPACE: ""
  RESYNC_PERIOD: "30s"
  
  # MCP Server (enable for integrated mode)
  MCP_ENABLED: "true"
  MCP_PORT: "8080"
  LOG_LEVEL: "info"
  
  # Prometheus (optional)
  PROMETHEUS_URL: "http://prometheus.monitoring.svc:9090"
```

### Secret

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: kkbase-mcp-server-secret
  namespace: default
type: Opaque
stringData:
  NEO4J_PASSWORD: "your-secure-password"
```

### Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: kkbase-mcp-server
spec:
  template:
    spec:
      containers:
      - name: mcp-server
        image: kkbase-mcp-server:latest
        
        # Load all non-sensitive config
        envFrom:
        - configMapRef:
            name: kkbase-mcp-server-config
        
        # Load password from Secret
        env:
        - name: NEO4J_PASSWORD
          valueFrom:
            secretKeyRef:
              name: kkbase-mcp-server-secret
              key: NEO4J_PASSWORD
```

## Configuration Examples

### Development Environment

Local Neo4j, debug logging, Prometheus disabled:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: kkbase-mcp-server-config
data:
  NEO4J_URI: "bolt://localhost:7687"
  NEO4J_USERNAME: "neo4j"
  MCP_PORT: "8080"
  LOG_LEVEL: "debug"
```

### Production Environment

Clustered Neo4j, standard logging, Prometheus enabled:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: kkbase-mcp-server-config
data:
  NEO4J_URI: "bolt://neo4j-cluster:7687"
  NEO4J_USERNAME: "neo4j"
  NEO4J_DATABASE: "production-k8s"
  MCP_PORT: "8080"
  LOG_LEVEL: "info"
  PROMETHEUS_URL: "http://prometheus-kube-prometheus-prometheus.monitoring.svc:9090"
```

### Multi-Cluster Environment

Separate database per cluster:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: kkbase-mcp-server-config
data:
  NEO4J_URI: "bolt://neo4j:7687"
  NEO4J_USERNAME: "neo4j"
  NEO4J_DATABASE: "cluster-us-east-1"
  MCP_PORT: "8080"
  LOG_LEVEL: "info"
  PROMETHEUS_URL: "http://prometheus.monitoring.svc:9090"
```

### Integrated Mode

Combined watcher + MCP server:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: kkbase-config
data:
  # Neo4j
  NEO4J_URI: "bolt://neo4j:7687"
  NEO4J_USERNAME: "neo4j"
  
  # Watcher
  NAMESPACE: ""
  RESYNC_PERIOD: "30s"
  
  # MCP Server
  MCP_ENABLED: "true"    # Enable integrated MCP server
  MCP_PORT: "8080"
  
  # Shared
  LOG_LEVEL: "info"
  PROMETHEUS_URL: "http://prometheus.monitoring.svc:9090"
```

## Local Development

For local development outside Kubernetes:

```bash
# Create .env file
cat > .env <<EOF
# Neo4j
NEO4J_URI=bolt://localhost:7687
NEO4J_USERNAME=neo4j
NEO4J_PASSWORD=password123
NEO4J_DATABASE=neo4j

# MCP Server
MCP_PORT=8080
LOG_LEVEL=debug

# Prometheus (optional)
PROMETHEUS_URL=http://localhost:9090
EOF

# Load and run
source .env
go run ./cmd/mcp-server
```

## Tool Availability by Configuration

| Tools | Configuration Required |
|-------|----------------------|
| **Core Tools** | Always available |
| `query` | Always |
| `structure` | Always |
| **Agent Session Tools** | Always available |
| `start_agent_session` | Always |
| `update_hypothesis` | Always |
| `query_with_session` | Always |
| `record_finding` | Always |
| `record_recommendation` | Always |
| `complete_agent_session` | Always |
| **Investigation Tools** | Requires `PROMETHEUS_URL` |
| `spawn_investigation` | When Prometheus configured |
| `get_investigation_status` | When Prometheus configured |
| `complete_investigation` | When Prometheus configured |
| **Dashboard Tools** | Always available |
| `get_active_sessions` | Always |
| `get_session_details` | Always |
| `get_blast_zone` | Always |
| `get_session_timeline` | Always |

## Configuration Validation

### Startup Checks

MCP server validates configuration on startup:

```bash
kubectl logs deployment/kkbase-mcp-server | head -20
```

Expected output:

```
INFO  Loading configuration from environment
INFO  Connected to Neo4j successfully  uri=bolt://neo4j:7687
INFO  MCP server listening  port=8080
INFO  Embedded frontend enabled
INFO  Metrics integration enabled  # If PROMETHEUS_URL set
INFO  Registered MCP tools  count=15
```

### Health Check

```bash
kubectl exec deployment/kkbase-mcp-server -- \
  curl -f http://localhost:8080/health

# Expected: {"status":"healthy","service":"kkbase-mcp"}
```

### Tool List Check

```bash
kubectl exec deployment/kkbase-mcp-server -- \
  curl -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'

# Should return list of available tools
```

## Troubleshooting

### Neo4j Connection Failed

**Error**: `failed to connect to Neo4j`

**Check**:
1. URI is correct (bolt:// or neo4j://)
2. Neo4j is running: `kubectl get pods -l app=neo4j`
3. Password is correct
4. Network: `kubectl exec deployment/kkbase-mcp-server -- nc -zv neo4j 7687`

### Port Already in Use

**Error**: `bind: address already in use`

**Solution**: Change MCP_PORT

```yaml
# Use different port
MCP_PORT: "8081"
```

Update port forward:

```bash
kubectl port-forward svc/kkbase-mcp-server 8081:8081
```

### Investigation Tools Not Available

**Symptom**: `spawn_investigation` tool missing from tools/list

**Solution**: Configure Prometheus URL

```yaml
data:
  PROMETHEUS_URL: "http://prometheus-kube-prometheus-prometheus.monitoring.svc:9090"
```

Verify:

```bash
# Test Prometheus connectivity
kubectl exec deployment/kkbase-mcp-server -- \
  curl -s http://prometheus-kube-prometheus-prometheus.monitoring.svc:9090/api/v1/status/config

# Check logs
kubectl logs deployment/kkbase-mcp-server | grep -i prometheus

# Expected: "metrics integration enabled"
```

### Dashboard Not Loading

**Symptom**: HTTP 404 on http://localhost:8080/

**Solution**: Frontend not embedded. Rebuild with frontend:

```bash
cd frontend
npm run build

# Copy to cmd/mcp-server/frontend/dist/
# Rebuild Docker image
```

Or use pre-built image with embedded frontend.

### Invalid Configuration Value

**Error**: `invalid port number`

**Solution**: Use valid port (1-65535)

```yaml
MCP_PORT: "8080"  # Must be string in YAML
```

## Performance Tuning

### Resource Limits

Adjust based on usage:

```yaml
# Low usage (<10 concurrent sessions)
resources:
  limits:
    memory: "512Mi"
    cpu: "1000m"
  requests:
    memory: "256Mi"
    cpu: "500m"

# Medium usage (10-50 concurrent sessions)
resources:
  limits:
    memory: "1Gi"
    cpu: "2000m"
  requests:
    memory: "512Mi"
    cpu: "1000m"

# High usage (>50 concurrent sessions)
resources:
  limits:
    memory: "2Gi"
    cpu: "4000m"
  requests:
    memory: "1Gi"
    cpu: "2000m"
```

### Log Level

Adjust for performance vs visibility:

```yaml
# Development - see everything (slower)
LOG_LEVEL: "debug"

# Production - standard (recommended)
LOG_LEVEL: "info"

# High-performance - minimal logging
LOG_LEVEL: "warn"
```

### Connection Pooling

Neo4j driver automatically manages connection pool. No configuration needed.

## Security

### Secrets Management

Never commit passwords to source control:

```bash
# Generate secure password
openssl rand -base64 32

# Create secret
kubectl create secret generic kkbase-mcp-server-secret \
  --from-literal=NEO4J_PASSWORD="$(openssl rand -base64 32)"
```

### TLS Configuration

For Neo4j with TLS:

```yaml
NEO4J_URI: "neo4j+s://neo4j.example.com:7687"  # Use neo4j+s:// for TLS
```

### Network Policies

Restrict MCP server access (see [Deployment Guide](deployment.md))

## Best Practices

1. **Use Secrets for passwords** - Never in ConfigMaps
2. **Start with debug logging** - Reduce after validation
3. **Enable Prometheus** - Unlock investigation tools
4. **Monitor startup logs** - Verify all connections
5. **Test tool list** - Confirm expected tools available
6. **Version control** - Store ConfigMaps (without passwords)
7. **Resource limits** - Set appropriate for cluster size

## See Also

- **[Deployment Guide](deployment.md)** - Step-by-step deployment
- **[Tools Reference](tools-reference.md)** - Complete API documentation
- **[Complete Configuration Reference](../../reference/configuration.md)** - All services
- **[Operations Guide](../../guides/operations/)** - Monitoring and troubleshooting

