# MCP Server Deployment Guide

This guide covers deploying the kkbase MCP Server in standalone and integrated modes.

## Deployment Modes

### Standalone Mode (Recommended for Production)

MCP Server deployed separately from watcher:

**Advantages**:
- Independent scaling
- Separate failure domains
- Clear separation of concerns
- Easier to secure MCP endpoint
- Can restart without affecting watcher

**When to use**: Production environments, multiple clusters, security-sensitive deployments

### Integrated Mode (Simpler for Development)

MCP Server combined with watcher in single binary:

**Advantages**:
- Single deployment
- Shared configuration
- Simpler setup
- Lower resource usage
- Fewer manifests to manage

**When to use**: Development, testing, small clusters, quick setup

## Prerequisites

- Kubernetes v1.19+
- kubectl configured
- Neo4j deployed (see [Watcher Deployment](../watcher/deployment.md))
- Watcher service running (for knowledge graph data)
- Prometheus (optional, for metrics investigation tools)

## Standalone Mode Deployment

### Step 1: Create Configuration

Create `mcp-server-config.yaml`:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: kkbase-mcp-server-config
  namespace: default
data:
  # Neo4j Connection
  NEO4J_URI: "bolt://neo4j:7687"
  NEO4J_USERNAME: "neo4j"
  NEO4J_DATABASE: "neo4j"
  
  # MCP Server
  MCP_PORT: "8080"
  LOG_LEVEL: "info"
  
  # Prometheus (optional - enables investigation tools)
  PROMETHEUS_URL: "http://prometheus-kube-prometheus-prometheus.monitoring.svc:9090"
```

Create secret:

```bash
kubectl create secret generic kkbase-mcp-server-secret \
  --from-literal=NEO4J_PASSWORD=your-secure-password
```

Apply:

```bash
kubectl apply -f mcp-server-config.yaml
```

### Step 2: Deploy MCP Server

Create `mcp-server-deployment.yaml`:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: kkbase-mcp-server
  namespace: default
  labels:
    app: kkbase-mcp-server
spec:
  replicas: 1
  selector:
    matchLabels:
      app: kkbase-mcp-server
  template:
    metadata:
      labels:
        app: kkbase-mcp-server
    spec:
      containers:
      - name: mcp-server
        image: kkbase-mcp-server:latest
        imagePullPolicy: IfNotPresent
        
        # Environment from ConfigMap
        envFrom:
        - configMapRef:
            name: kkbase-mcp-server-config
        
        # Password from Secret
        env:
        - name: NEO4J_PASSWORD
          valueFrom:
            secretKeyRef:
              name: kkbase-mcp-server-secret
              key: NEO4J_PASSWORD
        
        ports:
        - name: http
          containerPort: 8080
          protocol: TCP
        
        # Resource limits
        resources:
          limits:
            memory: "512Mi"
            cpu: "1000m"
          requests:
            memory: "256Mi"
            cpu: "500m"
        
        # Health check
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 30
        
        readinessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 10
      
      # Security
      securityContext:
        runAsNonRoot: true
        runAsUser: 65534
        fsGroup: 65534
```

Apply:

```bash
kubectl apply -f mcp-server-deployment.yaml
```

### Step 3: Expose Service

Create `mcp-server-service.yaml`:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: kkbase-mcp-server
  namespace: default
  labels:
    app: kkbase-mcp-server
spec:
  type: ClusterIP
  ports:
  - port: 8080
    targetPort: 8080
    protocol: TCP
    name: http
  selector:
    app: kkbase-mcp-server
```

Apply:

```bash
kubectl apply -f mcp-server-service.yaml
```

### Step 4: Verify

```bash
# Check deployment
kubectl get deployment kkbase-mcp-server

# Check pods
kubectl get pods -l app=kkbase-mcp-server

# Check logs
kubectl logs -f deployment/kkbase-mcp-server

# Expected output:
# INFO  Connected to Neo4j successfully
# INFO  MCP server listening  port=8080
# INFO  Embedded frontend enabled
```

## Integrated Mode Deployment

### Step 1: Create Configuration

Create `integrated-config.yaml`:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: kkbase-config
  namespace: default
data:
  # Neo4j Connection
  NEO4J_URI: "bolt://neo4j:7687"
  NEO4J_USERNAME: "neo4j"
  NEO4J_DATABASE: "neo4j"
  
  # Watcher Configuration
  NAMESPACE: ""
  RESYNC_PERIOD: "30s"
  LOG_LEVEL: "info"
  
  # MCP Server (enable integrated mode)
  MCP_ENABLED: "true"
  MCP_PORT: "8080"
  
  # Prometheus (optional)
  PROMETHEUS_URL: "http://prometheus-kube-prometheus-prometheus.monitoring.svc:9090"
```

Create secret:

```bash
kubectl create secret generic kkbase-secret \
  --from-literal=NEO4J_PASSWORD=your-secure-password
```

Apply:

```bash
kubectl apply -f integrated-config.yaml
```

### Step 2: Deploy Integrated Service

Create `integrated-deployment.yaml`:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: kkbase
  namespace: default
  labels:
    app: kkbase
spec:
  replicas: 1
  selector:
    matchLabels:
      app: kkbase
  template:
    metadata:
      labels:
        app: kkbase
    spec:
      serviceAccountName: kkbase-watcher  # Uses watcher RBAC
      containers:
      - name: kkbase
        image: kkbase-watcher:latest  # Same image, MCP enabled via env
        imagePullPolicy: IfNotPresent
        
        envFrom:
        - configMapRef:
            name: kkbase-config
        
        env:
        - name: NEO4J_PASSWORD
          valueFrom:
            secretKeyRef:
              name: kkbase-secret
              key: NEO4J_PASSWORD
        
        ports:
        - name: http
          containerPort: 8080
          protocol: TCP
        
        resources:
          limits:
            memory: "768Mi"   # Combined watcher + MCP
            cpu: "1500m"
          requests:
            memory: "384Mi"
            cpu: "750m"
        
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 30
        
        readinessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 10
      
      securityContext:
        runAsNonRoot: true
        runAsUser: 65534
        fsGroup: 65534
```

Apply:

```bash
kubectl apply -f integrated-deployment.yaml
```

### Step 3: Expose Service

Create `integrated-service.yaml`:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: kkbase
  namespace: default
  labels:
    app: kkbase
spec:
  type: ClusterIP
  ports:
  - port: 8080
    targetPort: 8080
    protocol: TCP
    name: http
  selector:
    app: kkbase
```

Apply:

```bash
kubectl apply -f integrated-service.yaml
```

### Step 4: Verify

```bash
# Check deployment
kubectl get deployment kkbase

# Check logs for both watcher and MCP
kubectl logs -f deployment/kkbase

# Expected output:
# INFO  successfully connected to Neo4j
# INFO  watcher started successfully
# INFO  MCP server listening  port=8080
# INFO  embedded frontend enabled
```

## Access the Dashboard

### Port Forward

```bash
# Standalone mode
kubectl port-forward svc/kkbase-mcp-server 8080:8080

# Integrated mode
kubectl port-forward svc/kkbase 8080:8080

# Open browser
open http://localhost:8080/
```

### Ingress (Production)

Create `mcp-ingress.yaml`:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: kkbase-mcp-ingress
  namespace: default
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
    nginx.ingress.kubernetes.io/ssl-redirect: "true"
spec:
  ingressClassName: nginx
  tls:
  - hosts:
    - kkbase.example.com
    secretName: kkbase-tls
  rules:
  - host: kkbase.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: kkbase-mcp-server  # or 'kkbase' for integrated
            port:
              number: 8080
```

Apply:

```bash
kubectl apply -f mcp-ingress.yaml
```

## AI Tool Integration

### Configure Cursor

Edit `~/.cursor/mcp.json`:

```json
{
  "mcpServers": {
    "kkbase": {
      "url": "http://localhost:8080/mcp",
      "transport": "sse"
    }
  }
}
```

With Ingress:

```json
{
  "mcpServers": {
    "kkbase": {
      "url": "https://kkbase.example.com/mcp",
      "transport": "sse"
    }
  }
}
```

Restart Cursor.

### Configure Claude Desktop

Edit `~/Library/Application Support/Claude/claude_desktop_config.json` (macOS):

```json
{
  "mcpServers": {
    "kkbase": {
      "url": "http://localhost:8080/mcp",
      "transport": "streamable-http"
    }
  }
}
```

Restart Claude Desktop.

## Production Setup

### 1. TLS/HTTPS

Use cert-manager for automatic certificates:

```bash
# Install cert-manager
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml

# Create ClusterIssuer
cat <<EOF | kubectl apply -f -
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-prod
spec:
  acme:
    server: https://acme-v02.api.letsencrypt.org/directory
    email: your-email@example.com
    privateKeySecretRef:
      name: letsencrypt-prod
    solvers:
    - http01:
        ingress:
          class: nginx
EOF
```

### 2. Authentication

Add OAuth2 proxy:

```bash
helm repo add oauth2-proxy https://oauth2-proxy.github.io/manifests
helm repo update

helm install oauth2-proxy oauth2-proxy/oauth2-proxy \
  --set config.clientID=<your-client-id> \
  --set config.clientSecret=<your-client-secret> \
  --set config.cookieSecret=<random-secret> \
  --set config.upstreams[0]=http://kkbase-mcp-server:8080
```

Update Ingress:

```yaml
annotations:
  nginx.ingress.kubernetes.io/auth-url: "https://$host/oauth2/auth"
  nginx.ingress.kubernetes.io/auth-signin: "https://$host/oauth2/start?rd=$escaped_request_uri"
```

### 3. Network Policies

Restrict MCP server access:

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: kkbase-mcp-server-policy
spec:
  podSelector:
    matchLabels:
      app: kkbase-mcp-server
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          name: ingress-nginx
    ports:
    - protocol: TCP
      port: 8080
  egress:
  - to:
    - podSelector:
        matchLabels:
          app: neo4j
    ports:
    - protocol: TCP
      port: 7687
  - to:  # Prometheus (if enabled)
    - namespaceSelector:
        matchLabels:
          name: monitoring
    ports:
    - protocol: TCP
      port: 9090
```

### 4. High Availability

Run multiple replicas:

```yaml
spec:
  replicas: 3
```

Add Pod Disruption Budget:

```yaml
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: kkbase-mcp-server-pdb
spec:
  minAvailable: 2
  selector:
    matchLabels:
      app: kkbase-mcp-server
```

### 5. Resource Limits

Adjust based on usage:

```yaml
resources:
  limits:
    memory: "1Gi"
    cpu: "2000m"
  requests:
    memory: "512Mi"
    cpu: "1000m"
```

## Monitoring

### Prometheus ServiceMonitor

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: kkbase-mcp-server
spec:
  selector:
    matchLabels:
      app: kkbase-mcp-server
  endpoints:
  - port: http
    path: /metrics
    interval: 30s
```

### Grafana Dashboard

Import dashboard for MCP server metrics (when available).

## Troubleshooting

### MCP Server Won't Start

```bash
# Check logs
kubectl logs deployment/kkbase-mcp-server | grep ERROR

# Common issues:
# 1. Neo4j not accessible
kubectl exec deployment/kkbase-mcp-server -- nc -zv neo4j 7687

# 2. Port conflict (integrated mode)
# MCP_PORT conflicts with watcher health port (8080)
# Solution: Use different port for MCP (8081) or deploy standalone
```

### AI Tool Connection Failed

```bash
# Test MCP endpoint
curl -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'

# Should return list of tools

# Check port forward
kubectl get svc
kubectl port-forward svc/kkbase-mcp-server 8080:8080
```

### Dashboard Not Loading

```bash
# Check if frontend is embedded
kubectl logs deployment/kkbase-mcp-server | grep frontend

# Expected: "embedded frontend enabled"

# If missing, rebuild with frontend:
# cd frontend && npm run build
# Copy dist/ to cmd/mcp-server/frontend/dist/
```

### High Memory Usage

```bash
# Check current usage
kubectl top pod -l app=kkbase-mcp-server

# Increase limits
kubectl set resources deployment/kkbase-mcp-server \
  --limits=memory=1Gi,cpu=2000m
```

## Upgrading

### Standalone Mode

```bash
# Update image
kubectl set image deployment/kkbase-mcp-server \
  mcp-server=kkbase-mcp-server:v2.0.0

# Monitor rollout
kubectl rollout status deployment/kkbase-mcp-server

# Verify
kubectl logs -f deployment/kkbase-mcp-server
```

### Integrated Mode

```bash
# Update image (updates both watcher and MCP)
kubectl set image deployment/kkbase \
  kkbase=kkbase-watcher:v2.0.0

# Monitor
kubectl rollout status deployment/kkbase
```

## Comparison: Standalone vs Integrated

| Aspect | Standalone | Integrated |
|--------|-----------|-----------|
| **Deployments** | 2 (watcher + MCP) | 1 (combined) |
| **Resource Usage** | Higher (2 pods) | Lower (1 pod) |
| **Scaling** | Independent | Together only |
| **Failure Domain** | Separate | Shared |
| **Setup Complexity** | More manifests | Fewer manifests |
| **Production Ready** | Yes | Dev/test |
| **Security** | Easier to isolate | Shared permissions |
| **Best For** | Production | Development |

## Clean Up

### Standalone Mode

```bash
kubectl delete deployment kkbase-mcp-server
kubectl delete service kkbase-mcp-server
kubectl delete configmap kkbase-mcp-server-config
kubectl delete secret kkbase-mcp-server-secret
kubectl delete ingress kkbase-mcp-ingress
```

### Integrated Mode

```bash
kubectl delete deployment kkbase
kubectl delete service kkbase
kubectl delete configmap kkbase-config
kubectl delete secret kkbase-secret
kubectl delete ingress kkbase-ingress
```

## Next Steps

- **[Configuration Guide](configuration.md)** - Detailed configuration options
- **[Tools Reference](tools-reference.md)** - Complete MCP tools API
- **[Dashboard Guide](dashboard.md)** - Web UI features
- **[Investigation Workflow](../../guides/investigations/workflow.md)** - How to use with AI agents

## Quick Reference

```bash
# Deploy standalone
kubectl apply -f mcp-server-config.yaml
kubectl create secret generic kkbase-mcp-server-secret --from-literal=NEO4J_PASSWORD=pass
kubectl apply -f mcp-server-deployment.yaml
kubectl apply -f mcp-server-service.yaml

# Deploy integrated
kubectl apply -f integrated-config.yaml
kubectl create secret generic kkbase-secret --from-literal=NEO4J_PASSWORD=pass
kubectl apply -f integrated-deployment.yaml
kubectl apply -f integrated-service.yaml

# Access dashboard
kubectl port-forward svc/kkbase-mcp-server 8080:8080
open http://localhost:8080/

# Test MCP endpoint
curl -X POST http://localhost:8080/mcp -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'
```

