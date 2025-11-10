# Kubernetes Deployment Manifests

This directory contains Kubernetes manifests for deploying kkbase in various configurations.

## Quick Start

### Standard Deployment (Watcher Only)

```bash
kubectl create namespace kkbase
kubectl apply -f rbac.yaml
kubectl apply -f configmap.yaml
kubectl apply -f secret.yaml
kubectl apply -f deployment.yaml
kubectl apply -f service.yaml
```

## Deployment Modes

### Mode 1: Standalone (Watcher + Separate MCP Server)

**Recommended for production** - Separate deployments provide isolation and independent scaling.

```bash
# Deploy watcher
kubectl apply -f rbac.yaml
kubectl apply -f configmap.yaml
kubectl apply -f secret.yaml
kubectl apply -f deployment.yaml
kubectl apply -f service.yaml

# Deploy standalone MCP server
kubectl apply -f mcp-server-deployment.yaml
kubectl apply -f mcp-server-service.yaml
```

**Access:**
- Watcher health: `http://kkbase-watcher:8080/healthz`
- MCP server: `http://kkbase-mcp-server:8080/mcp`

### Mode 2: Integrated (Watcher + MCP in One Pod)

**Good for development/demos** - Single deployment, simpler to manage.

```bash
kubectl apply -f rbac.yaml
kubectl apply -f configmap.yaml
kubectl apply -f secret.yaml
kubectl apply -f deployment-integrated.yaml
kubectl apply -f service-integrated.yaml
```

**Access:**
- Watcher health: `http://kkbase-watcher:8080/healthz`
- MCP server: `http://kkbase-watcher:8081/mcp`

## Files Reference

### Common Files (Both Modes)

| File | Description |
|------|-------------|
| `rbac.yaml` | ServiceAccount, ClusterRole, ClusterRoleBinding for K8s API access |
| `configmap.yaml` | Configuration (Neo4j connection, namespace, etc.) |
| `secret.yaml` | Secrets (Neo4j password) |

### Watcher-Only Files

| File | Description |
|------|-------------|
| `deployment.yaml` | Watcher deployment (no MCP) |
| `service.yaml` | Watcher service (health endpoints only) |

### Standalone MCP Files

| File | Description |
|------|-------------|
| `mcp-server-deployment.yaml` | Standalone MCP server deployment |
| `mcp-server-service.yaml` | MCP server service |

### Integrated Mode Files

| File | Description |
|------|-------------|
| `deployment-integrated.yaml` | Watcher with MCP enabled (MCP_ENABLED=true) |
| `service-integrated.yaml` | Service with both health and MCP ports |

### Optional Files

| File | Description |
|------|-------------|
| `mcp-server-ingress.yaml` | Ingress for external access (requires TLS/auth) |

## Configuration

### ConfigMap (`configmap.yaml`)

Edit these values for your environment:

```yaml
NEO4J_URI: "bolt://neo4j:7687"
NEO4J_USERNAME: "neo4j"
NEO4J_DATABASE: "neo4j"
NAMESPACE: ""  # Empty = all namespaces, or specify one
RESYNC_PERIOD: "30s"
LOG_LEVEL: "info"
```

### Secret (`secret.yaml`)

Set your Neo4j password:

```bash
# Edit secret.yaml and base64 encode your password
echo -n "your-password" | base64
```

Or create the secret directly:

```bash
kubectl create secret generic kkbase-watcher-secret \
  --from-literal=NEO4J_PASSWORD=your-password \
  -n kkbase
```

## External Access (Optional)

### Expose MCP Server with Ingress

**⚠️ IMPORTANT:** Never expose MCP server to the internet without proper security!

1. **Enable TLS:**
   ```bash
   # Create TLS secret
   kubectl create secret tls mcp-tls-secret \
     --cert=path/to/cert.pem \
     --key=path/to/key.pem \
     -n kkbase
   ```

2. **Enable Authentication:**
   ```bash
   # Create basic auth secret
   htpasswd -c auth myuser
   kubectl create secret generic mcp-basic-auth \
     --from-file=auth \
     -n kkbase
   ```

3. **Deploy Ingress:**
   ```bash
   # Edit mcp-server-ingress.yaml with your domain
   kubectl apply -f mcp-server-ingress.yaml
   ```

### Port Forward (Development)

For local development without Ingress:

```bash
# Standalone mode
kubectl port-forward svc/kkbase-mcp-server 8080:8080 -n kkbase

# Integrated mode
kubectl port-forward svc/kkbase-watcher 8081:8081 -n kkbase
```

Then access at `http://localhost:8080/mcp` or `http://localhost:8081/mcp`

## Scaling

### Standalone Mode - Independent Scaling

```bash
# Scale watcher
kubectl scale deployment kkbase-watcher --replicas=1 -n kkbase

# Scale MCP server based on AI agent load
kubectl scale deployment kkbase-mcp-server --replicas=3 -n kkbase
```

### Integrated Mode - Unified Scaling

```bash
# Both watcher and MCP scale together
kubectl scale deployment kkbase-watcher --replicas=2 -n kkbase
```

## Resource Requirements

### Watcher Only
- **Requests:** 100m CPU, 128Mi memory
- **Limits:** 500m CPU, 512Mi memory

### Standalone MCP Server
- **Requests:** 100m CPU, 128Mi memory
- **Limits:** 500m CPU, 512Mi memory

### Integrated Mode (Watcher + MCP)
- **Requests:** 150m CPU, 256Mi memory
- **Limits:** 1000m CPU, 1Gi memory

Adjust based on your cluster size and query load.

## Health Checks

### Watcher

- **Liveness:** `GET /healthz` on port 8080
- **Readiness:** `GET /ready` on port 8080 (checks Neo4j connectivity)

### MCP Server

- **Liveness:** `GET /health` on port 8080 (standalone) or 8081 (integrated)
- **Readiness:** `GET /health` on port 8080 (standalone) or 8081 (integrated)

## Troubleshooting

### Check Logs

```bash
# Watcher logs
kubectl logs -f deployment/kkbase-watcher -n kkbase

# MCP server logs (standalone)
kubectl logs -f deployment/kkbase-mcp-server -n kkbase

# Integrated mode logs
kubectl logs -f deployment/kkbase-watcher -n kkbase --tail=100
```

### Check Service Endpoints

```bash
# List endpoints
kubectl get endpoints -n kkbase

# Describe service
kubectl describe svc kkbase-mcp-server -n kkbase
```

### Test Connectivity

```bash
# From a pod in the cluster
kubectl run -it --rm debug --image=curlimages/curl --restart=Never -n kkbase -- \
  curl http://kkbase-mcp-server:8080/health

# Or for integrated mode
kubectl run -it --rm debug --image=curlimages/curl --restart=Never -n kkbase -- \
  curl http://kkbase-watcher:8081/health
```

### Common Issues

**MCP server not responding:**
- Check Neo4j is accessible: `kubectl exec -it deployment/kkbase-mcp-server -n kkbase -- curl -v http://neo4j:7687`
- Check logs for connection errors
- Verify secret has correct Neo4j password

**Pod not starting:**
- Check image pull status: `kubectl describe pod <pod-name> -n kkbase`
- Verify RBAC permissions: `kubectl auth can-i list pods --as=system:serviceaccount:kkbase:kkbase-watcher`

**High memory usage:**
- Reduce query load or increase memory limits
- Consider scaling replicas instead of increasing per-pod limits

## Security Best Practices

1. **Network Policies:** Restrict access to MCP server
   ```yaml
   apiVersion: networking.k8s.io/v1
   kind: NetworkPolicy
   metadata:
     name: mcp-server-policy
   spec:
     podSelector:
       matchLabels:
         app: kkbase-mcp-server
     ingress:
       - from:
         - podSelector:
             matchLabels:
               app: authorized-client
   ```

2. **RBAC:** Watcher needs cluster-wide read access (already configured in `rbac.yaml`)

3. **Secrets:** Use external secret management (Vault, External Secrets Operator)

4. **TLS:** Always use TLS when exposing MCP externally

5. **Authentication:** Add OAuth 2.1 or API key authentication for production

6. **Monitoring:** Monitor query patterns for suspicious activity

## Monitoring

### Prometheus Metrics (Future)

The deployment is instrumented for Prometheus scraping:

```yaml
annotations:
  prometheus.io/scrape: "true"
  prometheus.io/port: "8080"
  prometheus.io/path: "/metrics"
```

### Key Metrics to Monitor

- Query execution time
- Query failure rate
- Connection pool utilization
- Memory and CPU usage
- Request rate and latency

## Upgrade Strategy

### Rolling Update (Default)

Kubernetes will automatically perform rolling updates:

```bash
kubectl set image deployment/kkbase-mcp-server \
  mcp-server=quay.io/aslakknutsen/kkbase-mcp-server:v2.0.0 \
  -n kkbase
```

### Blue-Green Deployment

For zero-downtime upgrades with rollback capability:

1. Deploy new version with different name
2. Test thoroughly
3. Switch service selector
4. Remove old deployment

## Backup and Disaster Recovery

The MCP server is stateless - all data is in Neo4j. Ensure you have:

1. **Neo4j Backups:** Regular backups of the graph database
2. **Configuration Backups:** Store ConfigMaps and Secrets in version control
3. **Deployment Automation:** Use GitOps (ArgoCD, Flux) for reproducible deployments

## Next Steps

- [MCP Server User Guide](../docs/services/mcp-server/README.md)
- [Service Documentation](../docs/services/)
- [Cypher Query Examples](../docs/reference/cypher-queries.md)

