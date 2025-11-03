# Integrated Deployment Guide

## Architecture: 2 Containers, 1 Pod

This is the **recommended production deployment** for KKBase with the Agent Investigation Dashboard.

```
┌─────────────────────────────────────────────────────────────┐
│                  Kubernetes Pod: kkbase-integrated          │
│                                                             │
│  ┌────────────────────────┐  ┌──────────────────────────┐  │
│  │  Container 1:          │  │  Container 2:            │  │
│  │  watcher               │  │  mcp-server              │  │
│  │                        │  │                          │  │
│  │  - Watches K8s         │  │  - HTTP :8080           │  │
│  │  - Writes to Neo4j     │  │  - MCP SSE transport    │  │
│  │  - No HTTP server      │  │  - Embedded frontend    │  │
│  │                        │  │  - AgentSessionManager  │  │
│  └────────┬───────────────┘  └────────┬─────────────────┘  │
│           │                           │                     │
│           └───────────┬───────────────┘                     │
│                       │ Both connect to                     │
└───────────────────────┼─────────────────────────────────────┘
                        ↓
              ┌─────────────────┐
              │  Neo4j Service  │
              │  bolt://neo4j   │
              └─────────────────┘
```

## Why This Architecture?

✅ **Separation of concerns**:
- Watcher: K8s resource watching only
- MCP Server: API + frontend + agent tools

✅ **Independent lifecycle**:
- Restart watcher without affecting dashboard users
- Update MCP server without disrupting K8s watching

✅ **Shared resources**:
- Same pod network (efficient localhost communication if needed)
- Shared volume mounts if needed
- Single point of deployment

✅ **Resource efficiency**:
- Share pod overhead
- Better resource utilization than separate pods

## Deployment

### 1. Prerequisites

```bash
# Neo4j must be running
kubectl get svc neo4j -n kkbase
# Should show: neo4j ClusterIP ... 7687/TCP

# Prometheus (optional, for metrics investigations)
kubectl get svc prometheus -n monitoring
```

### 2. Build Docker Images

```bash
# Build watcher image
cd kkbase
docker build -f Dockerfile.watcher -t quay.io/aslakknutsen/kkbase-watcher:latest .
docker push quay.io/aslakknutsen/kkbase-watcher:latest

# Build MCP server image (includes embedded frontend)
make build-mcp-server  # Builds frontend + Go binary
docker build -f Dockerfile.mcp-server -t quay.io/aslakknutsen/kkbase-mcp-server:latest .
docker push quay.io/aslakknutsen/kkbase-mcp-server:latest
```

### 3. Deploy to Kubernetes

```bash
# Deploy integrated pod
kubectl apply -f deploy/rbac.yaml
kubectl apply -f deploy/configmap.yaml
kubectl apply -f deploy/secret.yaml
kubectl apply -f deploy/deployment-integrated.yaml
kubectl apply -f deploy/service-integrated.yaml
kubectl apply -f deploy/mcp-server-ingress.yaml
```

### 4. Verify Deployment

```bash
# Check pod status
kubectl get pods -n kkbase -l app=kkbase-integrated

# Should show:
# NAME                                  READY   STATUS    RESTARTS   AGE
# kkbase-integrated-xxxxx-yyyyy        2/2     Running   0          1m

# Check logs
kubectl logs -n kkbase -l app=kkbase-integrated -c watcher --tail=20
kubectl logs -n kkbase -l app=kkbase-integrated -c mcp-server --tail=20

# Port forward to test locally
kubectl port-forward -n kkbase svc/kkbase-integrated 8080:8080

# Open browser: http://localhost:8080/
```

## What's Running

### Container 1: watcher

**Binary**: `watcher`
**Command**: `./watcher`
**No ports exposed**

**What it does**:
- Watches Kubernetes resources (Pods, Services, Deployments, etc.)
- Writes to Neo4j graph database
- Creates/updates nodes and relationships
- No HTTP server, no frontend

**Environment**:
```yaml
env:
- name: NEO4J_URI
  value: bolt://neo4j:7687
- name: NEO4J_USERNAME
  value: neo4j
- name: NEO4J_PASSWORD
  valueFrom:
    secretKeyRef:
      name: kkbase-secrets
      key: NEO4J_PASSWORD
```

**Resources**:
```yaml
resources:
  requests:
    memory: 64Mi
    cpu: 100m
  limits:
    memory: 128Mi
    cpu: 200m
```

### Container 2: mcp-server

**Binary**: `mcp-server` (17 MB with embedded frontend)
**Command**: `./mcp-server`
**Port**: 8080

**What it does**:
- HTTP server with MCP SSE transport
- Serves embedded React dashboard
- Provides MCP tools for AI agent (Cursor)
- Provides MCP resources for dashboard
- AgentSessionManager for investigation tracking
- Custom SSE notifications for real-time updates

**Environment**:
```yaml
env:
- name: NEO4J_URI
  value: bolt://neo4j:7687
- name: NEO4J_USERNAME
  value: neo4j
- name: NEO4J_PASSWORD
  valueFrom:
    secretKeyRef:
      name: kkbase-secrets
      key: NEO4J_PASSWORD
- name: PROMETHEUS_URL
  value: http://prometheus.monitoring.svc:9090
- name: MCP_PORT
  value: "8080"
```

**Resources**:
```yaml
resources:
  requests:
    memory: 128Mi
    cpu: 100m
  limits:
    memory: 256Mi
    cpu: 500m
```

## Accessing the Dashboard

### Via Port Forward (Development)

```bash
kubectl port-forward -n kkbase svc/kkbase-integrated 8080:8080
```

**URLs**:
- Dashboard: http://localhost:8080/
- MCP endpoint: http://localhost:8080/mcp
- Health check: http://localhost:8080/health

### Via Ingress (Production)

```yaml
# deploy/mcp-server-ingress.yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: kkbase-dashboard
  namespace: kkbase
spec:
  rules:
  - host: kkbase.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: kkbase-integrated
            port:
              number: 8080
```

**URLs**:
- Dashboard: https://kkbase.example.com/
- MCP endpoint: https://kkbase.example.com/mcp

## Using the Dashboard

### 1. Start Investigation from Cursor

In Cursor, ask the AI:
```
Using kkbase MCP tools, investigate why the order-service is failing
```

The AI will:
1. Call `start_agent_session` → Creates session in Neo4j
2. Call `query_with_session` → Runs Cypher queries
3. Call `update_hypothesis` → Updates theory + recalcs blast zone
4. Call `record_finding` → Records discovered issues

### 2. Observe in Dashboard

Open browser: https://kkbase.example.com/

**What you'll see**:
- New session appears in sidebar (via MCP SSE notification)
- Hypothesis panel updates in real-time
- Blast zone graph expands as findings discovered
- Query history shows agent's reasoning
- Timeline shows chronological events

**Update latency**: < 100ms from agent action to dashboard update

## Resource Requirements

### Per Pod (Integrated)

**Minimum** (development):
- Memory: 192 Mi (64 + 128)
- CPU: 200m (100 + 100)

**Recommended** (production):
- Memory: 384 Mi (128 + 256)
- CPU: 700m (200 + 500)

### Total Cluster (with Neo4j)

**Minimum**:
- 3 pods × 192 Mi = ~600 MB RAM
- Neo4j: 512 MB RAM
- **Total**: ~1.2 GB RAM

**Production**:
- 3 pods × 384 Mi = ~1.2 GB RAM
- Neo4j: 2 GB RAM
- **Total**: ~3.2 GB RAM

## Scaling

### Horizontal Scaling

```yaml
# Increase replicas
kubectl scale deployment kkbase-integrated --replicas=3 -n kkbase
```

**Result**:
- 3 watcher containers (each watching K8s)
- 3 MCP server containers (load balanced)
- Shared Neo4j (single source of truth)

**Notes**:
- Multiple watchers are safe (idempotent writes to Neo4j)
- MCP servers are stateless (can scale freely)
- Use session affinity for MCP SSE connections

### Service Configuration for HA

```yaml
# deploy/service-integrated.yaml
apiVersion: v1
kind: Service
metadata:
  name: kkbase-integrated
spec:
  type: ClusterIP
  sessionAffinity: ClientIP  # Important for SSE
  sessionAffinityConfig:
    clientIP:
      timeoutSeconds: 3600  # 1 hour
  ports:
  - port: 8080
    targetPort: 8080
  selector:
    app: kkbase-integrated
```

## Monitoring

### Health Checks

```bash
# MCP server health
curl http://kkbase-integrated:8080/health

# Should return:
# {"status":"healthy","service":"kkbase-mcp","version":"1.0.0"}
```

### Logs

```bash
# Watcher logs
kubectl logs -n kkbase -l app=kkbase-integrated -c watcher -f

# MCP server logs
kubectl logs -n kkbase -l app=kkbase-integrated -c mcp-server -f

# Both containers
kubectl logs -n kkbase -l app=kkbase-integrated --all-containers -f
```

### Metrics (if Prometheus enabled)

```bash
# Port forward Prometheus
kubectl port-forward -n monitoring svc/prometheus 9090:9090

# Query metrics:
# - kkbase_active_sessions_total
# - kkbase_query_duration_seconds
# - kkbase_findings_discovered_total
```

## Troubleshooting

### Pod Not Starting

```bash
# Check events
kubectl describe pod -n kkbase -l app=kkbase-integrated

# Common issues:
# - Neo4j not ready: Wait for neo4j service
# - Secrets missing: Apply deploy/secret.yaml
# - RBAC issues: Apply deploy/rbac.yaml
```

### Dashboard Not Loading

```bash
# Check if mcp-server container is running
kubectl get pods -n kkbase -l app=kkbase-integrated

# Should show 2/2 READY

# Check mcp-server logs
kubectl logs -n kkbase -l app=kkbase-integrated -c mcp-server

# Should see:
# INFO  Embedded frontend enabled
# INFO  MCP server listening address=:8080
```

### No Notifications Received

```bash
# Check SSE connection in browser console
# Should see:
# "SSE connection established"
# "Received SSE notification: agent_session/created"

# If not, check mcp-server logs
kubectl logs -n kkbase -l app=kkbase-integrated -c mcp-server | grep -i notification
```

### Watcher Not Populating Data

```bash
# Check watcher logs
kubectl logs -n kkbase -l app=kkbase-integrated -c watcher

# Should see:
# INFO  Starting Kubernetes watchers
# INFO  Connected to Neo4j successfully

# Verify Neo4j has data
kubectl port-forward -n kkbase svc/neo4j 7474:7474
# Open: http://localhost:7474
# Run: MATCH (n) RETURN count(n)
```

## Updating

### Update Watcher Only

```bash
# Build new image
docker build -f Dockerfile.watcher -t kkbase-watcher:v2 .
docker push kkbase-watcher:v2

# Update deployment
kubectl set image deployment/kkbase-integrated watcher=kkbase-watcher:v2 -n kkbase

# Rolling update - MCP server stays up
```

### Update MCP Server Only

```bash
# Rebuild with new frontend
make build-mcp-server

# Build and push image
docker build -f Dockerfile.mcp-server -t kkbase-mcp-server:v2 .
docker push kkbase-mcp-server:v2

# Update deployment
kubectl set image deployment/kkbase-integrated mcp-server=kkbase-mcp-server:v2 -n kkbase

# Rolling update - watcher stays up
```

### Update Both

```bash
# Edit deployment YAML
kubectl edit deployment kkbase-integrated -n kkbase

# Or apply updated file
kubectl apply -f deploy/deployment-integrated.yaml
```

## Security

### Network Policies

```yaml
# Restrict watcher (no ingress needed)
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: kkbase-watcher
spec:
  podSelector:
    matchLabels:
      app: kkbase-integrated
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
```

### Pod Security

```yaml
# Apply pod security standards
apiVersion: v1
kind: Namespace
metadata:
  name: kkbase
  labels:
    pod-security.kubernetes.io/enforce: restricted
```

## Summary

✅ **Deployed**: 2-container pod with watcher + MCP server
✅ **Frontend**: Embedded in mcp-server binary
✅ **MCP SSE**: Proper protocol transport
✅ **Notifications**: Real-time push via custom SSE
✅ **Scalable**: Can run 3+ replicas
✅ **Production ready**: Health checks, monitoring, security

**Access**:
- Dashboard: https://kkbase.example.com/
- MCP: https://kkbase.example.com/mcp

**Next**: Configure Cursor to use the MCP endpoint and start investigating! 🚀

