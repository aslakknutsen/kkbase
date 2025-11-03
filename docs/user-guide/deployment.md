# Production Deployment Guide

## Overview

This guide covers deploying KKBase with the agent investigation dashboard to production environments.

## Architecture Options

### Option 1: Integrated Mode (Recommended)

Single pod running watcher + MCP server with embedded frontend.

**Pros:**
- Simple deployment
- Single service to manage
- Shared Neo4j connection pool

**Cons:**
- Single point of failure
- Watcher restart affects MCP server

### Option 2: Standalone MCP Server

Separate deployments for watcher and MCP server.

**Pros:**
- Independent scaling
- Watcher restarts don't affect dashboard
- Can run multiple MCP servers for HA

**Cons:**
- More complex deployment
- Multiple services to manage

## Prerequisites

- Kubernetes cluster 1.24+
- Neo4j 5.x (with APOC plugin)
- Prometheus (optional, for metrics investigations)
- Ingress controller (for external access)

## Step 1: Deploy Neo4j

### Using Helm

```bash
# Add Neo4j Helm repo
helm repo add neo4j https://neo4j.com/helm-charts
helm repo update

# Install Neo4j with APOC
helm install neo4j neo4j/neo4j \
  --set neo4j.name=kkbase-neo4j \
  --set neo4j.password=CHANGE_THIS_PASSWORD \
  --set plugins='["apoc"]' \
  --set resources.requests.memory=1Gi \
  --set resources.limits.memory=2Gi \
  --set persistence.size=10Gi \
  --namespace kkbase \
  --create-namespace
```

### Using Manifests

```yaml
# neo4j-deployment.yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: neo4j
  namespace: kkbase
spec:
  serviceName: neo4j
  replicas: 1
  selector:
    matchLabels:
      app: neo4j
  template:
    metadata:
      labels:
        app: neo4j
    spec:
      containers:
      - name: neo4j
        image: neo4j:5.15
        ports:
        - containerPort: 7474
          name: http
        - containerPort: 7687
          name: bolt
        env:
        - name: NEO4J_AUTH
          value: "neo4j/CHANGE_THIS_PASSWORD"
        - name: NEO4J_PLUGINS
          value: '["apoc"]'
        - name: NEO4J_apoc_export_file_enabled
          value: "true"
        - name: NEO4J_apoc_import_file_enabled
          value: "true"
        resources:
          requests:
            memory: "1Gi"
            cpu: "500m"
          limits:
            memory: "2Gi"
            cpu: "1000m"
        volumeMounts:
        - name: neo4j-data
          mountPath: /data
  volumeClaimTemplates:
  - metadata:
      name: neo4j-data
    spec:
      accessModes: ["ReadWriteOnce"]
      resources:
        requests:
          storage: 10Gi
---
apiVersion: v1
kind: Service
metadata:
  name: neo4j
  namespace: kkbase
spec:
  selector:
    app: neo4j
  ports:
  - name: http
    port: 7474
    targetPort: 7474
  - name: bolt
    port: 7687
    targetPort: 7687
```

Apply:

```bash
kubectl apply -f neo4j-deployment.yaml
```

## Step 2: Configure Secrets

Create secret for sensitive configuration:

```yaml
# secrets.yaml
apiVersion: v1
kind: Secret
metadata:
  name: kkbase-secrets
  namespace: kkbase
type: Opaque
stringData:
  NEO4J_PASSWORD: "CHANGE_THIS_PASSWORD"
  PROMETHEUS_URL: "http://prometheus.monitoring.svc:9090"
```

Apply:

```bash
kubectl apply -f secrets.yaml
```

## Step 3: Deploy KKBase (Integrated Mode)

Update the existing deployment manifest:

```bash
# Use the integrated deployment
kubectl apply -f deploy/rbac.yaml
kubectl apply -f deploy/configmap.yaml
kubectl apply -f deploy/deployment-integrated.yaml
kubectl apply -f deploy/service-integrated.yaml
```

The `deployment-integrated.yaml` should include the MCP server:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: kkbase-integrated
  namespace: kkbase
spec:
  replicas: 1
  selector:
    matchLabels:
      app: kkbase-integrated
  template:
    metadata:
      labels:
        app: kkbase-integrated
    spec:
      serviceAccountName: kkbase
      containers:
      # Watcher container
      - name: watcher
        image: quay.io/aslakknutsen/kkbase-watcher:latest
        imagePullPolicy: Always
        env:
        - name: NEO4J_URI
          value: "bolt://neo4j:7687"
        - name: NEO4J_USERNAME
          value: "neo4j"
        - name: NEO4J_PASSWORD
          valueFrom:
            secretKeyRef:
              name: kkbase-secrets
              key: NEO4J_PASSWORD
        - name: NEO4J_DATABASE
          value: "neo4j"
        - name: LOG_LEVEL
          value: "info"
        resources:
          requests:
            memory: "64Mi"
            cpu: "100m"
          limits:
            memory: "128Mi"
            cpu: "200m"
      
      # MCP Server container
      - name: mcp-server
        image: quay.io/aslakknutsen/kkbase-mcp-server:latest
        imagePullPolicy: Always
        ports:
        - containerPort: 8080
          name: http
        env:
        - name: NEO4J_URI
          value: "bolt://neo4j:7687"
        - name: NEO4J_USERNAME
          value: "neo4j"
        - name: NEO4J_PASSWORD
          valueFrom:
            secretKeyRef:
              name: kkbase-secrets
              key: NEO4J_PASSWORD
        - name: NEO4J_DATABASE
          value: "neo4j"
        - name: MCP_PORT
          value: "8080"
        - name: PROMETHEUS_URL
          valueFrom:
            secretKeyRef:
              name: kkbase-secrets
              key: PROMETHEUS_URL
        - name: LOG_LEVEL
          value: "info"
        resources:
          requests:
            memory: "128Mi"
            cpu: "100m"
          limits:
            memory: "256Mi"
            cpu: "500m"
        readinessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 5
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
```

## Step 4: Expose Dashboard

### Option A: ClusterIP + Port Forward (Development)

```bash
# Port forward for local access
kubectl port-forward -n kkbase svc/kkbase-integrated 8080:8080

# Access at: http://localhost:8080/
```

### Option B: Ingress (Production)

```yaml
# ingress.yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: kkbase-dashboard
  namespace: kkbase
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
            name: kkbase-integrated
            port:
              number: 8080
```

Apply:

```bash
kubectl apply -f ingress.yaml

# Wait for TLS certificate
kubectl wait --for=condition=Ready certificate/kkbase-tls -n kkbase --timeout=300s
```

Access: https://kkbase.example.com/

### Option C: LoadBalancer

```yaml
# loadbalancer-service.yaml
apiVersion: v1
kind: Service
metadata:
  name: kkbase-lb
  namespace: kkbase
spec:
  type: LoadBalancer
  selector:
    app: kkbase-integrated
  ports:
  - port: 80
    targetPort: 8080
    name: http
```

Apply:

```bash
kubectl apply -f loadbalancer-service.yaml

# Get external IP
kubectl get svc kkbase-lb -n kkbase
```

## Step 5: Configure Prometheus Integration

If using metrics investigations:

```yaml
# prometheus-rbac.yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: kkbase-prometheus-reader
rules:
- apiGroups: [""]
  resources: ["services", "endpoints", "pods"]
  verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: kkbase-prometheus-reader
subjects:
- kind: ServiceAccount
  name: kkbase
  namespace: kkbase
roleRef:
  kind: ClusterRole
  name: kkbase-prometheus-reader
  apiGroup: rbac.authorization.k8s.io
```

Update secret with Prometheus URL:

```bash
kubectl create secret generic kkbase-secrets \
  --from-literal=NEO4J_PASSWORD="CHANGE_THIS_PASSWORD" \
  --from-literal=PROMETHEUS_URL="http://prometheus.monitoring.svc:9090" \
  --namespace kkbase \
  --dry-run=client -o yaml | kubectl apply -f -
```

## Step 6: Verify Deployment

### Check Pods

```bash
kubectl get pods -n kkbase

# Should show:
# NAME                                 READY   STATUS    RESTARTS   AGE
# neo4j-0                             1/1     Running   0          5m
# kkbase-integrated-xxxxx-yyyyy       2/2     Running   0          2m
```

### Check Services

```bash
kubectl get svc -n kkbase

# Should show:
# NAME                TYPE        CLUSTER-IP      EXTERNAL-IP   PORT(S)
# neo4j              ClusterIP   10.96.1.100     <none>        7474/TCP,7687/TCP
# kkbase-integrated  ClusterIP   10.96.1.101     <none>        8080/TCP
```

### Check Logs

```bash
# Watcher logs
kubectl logs -n kkbase -l app=kkbase-integrated -c watcher --tail=50

# MCP server logs
kubectl logs -n kkbase -l app=kkbase-integrated -c mcp-server --tail=50

# Should see:
# INFO  Connected to Neo4j successfully
# INFO  Metrics integration enabled
# INFO  Agent session manager initialized
# INFO  Embedded frontend enabled
# INFO  MCP server listening
```

### Test Dashboard

```bash
# If using ingress
curl -I https://kkbase.example.com/

# Should return: HTTP/1.1 200 OK

# If using port-forward
curl -I http://localhost:8080/

# Should return: HTTP/1.1 200 OK
```

### Test MCP Endpoint

```bash
# List tools
curl -X POST https://kkbase.example.com/mcp \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "tools/list"
  }'

# Should return list of available tools
```

## Step 7: Configure Monitoring

### Prometheus ServiceMonitor

```yaml
# servicemonitor.yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: kkbase
  namespace: kkbase
spec:
  selector:
    matchLabels:
      app: kkbase-integrated
  endpoints:
  - port: http
    path: /metrics
    interval: 30s
```

### Grafana Dashboard

Import the KKbase dashboard (create `grafana-dashboard.json`):

```json
{
  "dashboard": {
    "title": "KKBase Agent Investigations",
    "panels": [
      {
        "title": "Active Sessions",
        "targets": [
          {
            "expr": "kkbase_active_sessions_total"
          }
        ]
      },
      {
        "title": "Query Execution Time",
        "targets": [
          {
            "expr": "histogram_quantile(0.95, rate(kkbase_query_duration_seconds_bucket[5m]))"
          }
        ]
      }
    ]
  }
}
```

## Step 8: Configure Cursor MCP (for Users)

Users need to configure Cursor to connect to the production MCP server.

**Internal access (same cluster):**

```json
{
  "mcpServers": {
    "kkbase-prod": {
      "url": "http://kkbase-integrated.kkbase.svc.cluster.local:8080/mcp",
      "transport": "sse"
    }
  }
}
```

**External access (via ingress):**

```json
{
  "mcpServers": {
    "kkbase-prod": {
      "url": "https://kkbase.example.com/mcp",
      "transport": "sse"
    }
  }
}
```

## High Availability Setup

### Neo4j Cluster (Enterprise)

```bash
# Deploy Neo4j cluster with 3 core members
helm install neo4j neo4j/neo4j-cluster-core \
  --set core.numberOfServers=3 \
  --set core.resources.requests.memory=2Gi \
  --set readReplicas.numberOfServers=2 \
  --namespace kkbase
```

### Multiple MCP Server Replicas

```yaml
# deployment-mcp-ha.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: kkbase-mcp-server
  namespace: kkbase
spec:
  replicas: 3  # Run 3 replicas for HA
  selector:
    matchLabels:
      app: kkbase-mcp-server
  template:
    metadata:
      labels:
        app: kkbase-mcp-server
    spec:
      serviceAccountName: kkbase
      containers:
      - name: mcp-server
        image: quay.io/aslakknutsen/kkbase-mcp-server:latest
        # ... same config as before
        resources:
          requests:
            memory: "256Mi"
            cpu: "200m"
          limits:
            memory: "512Mi"
            cpu: "500m"
---
apiVersion: v1
kind: Service
metadata:
  name: kkbase-mcp-server
  namespace: kkbase
spec:
  selector:
    app: kkbase-mcp-server
  ports:
  - port: 8080
    targetPort: 8080
  sessionAffinity: ClientIP  # Important for MCP notifications
```

## Resource Requirements

### Minimum (Development)

- **Neo4j**: 512 MB RAM, 5 GB disk
- **Watcher**: 64 MB RAM, 0.1 CPU
- **MCP Server**: 128 MB RAM, 0.1 CPU
- **Total**: ~700 MB RAM, 5 GB disk

### Recommended (Production)

- **Neo4j**: 2 GB RAM, 50 GB disk, 1 CPU
- **Watcher**: 128 MB RAM, 0.2 CPU
- **MCP Server**: 256 MB RAM, 0.5 CPU (per replica)
- **Total** (3 MCP replicas): ~3 GB RAM, 50 GB disk, 2.5 CPU

### Large Cluster (>1000 nodes)

- **Neo4j**: 8 GB RAM, 200 GB disk, 4 CPU
- **Watcher**: 256 MB RAM, 0.5 CPU
- **MCP Server**: 512 MB RAM, 1 CPU (per replica)
- **Total** (5 MCP replicas): ~10 GB RAM, 200 GB disk, 9 CPU

## Backup and Disaster Recovery

### Neo4j Backup

```bash
# Create backup CronJob
kubectl apply -f - <<EOF
apiVersion: batch/v1
kind: CronJob
metadata:
  name: neo4j-backup
  namespace: kkbase
spec:
  schedule: "0 2 * * *"  # Daily at 2 AM
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: backup
            image: neo4j:5.15
            command:
            - /bin/bash
            - -c
            - |
              neo4j-admin database dump neo4j --to-path=/backups/neo4j-\$(date +%Y%m%d).dump
              # Upload to S3/GCS/etc
          volumeMounts:
          - name: backups
            mountPath: /backups
          volumes:
          - name: backups
            persistentVolumeClaim:
              claimName: neo4j-backups
EOF
```

### Restore from Backup

```bash
# Copy backup to Neo4j pod
kubectl cp neo4j-20241103.dump kkbase/neo4j-0:/backups/

# Restore
kubectl exec -it neo4j-0 -n kkbase -- \
  neo4j-admin database load neo4j --from-path=/backups/neo4j-20241103.dump --overwrite-destination=true

# Restart Neo4j
kubectl rollout restart statefulset/neo4j -n kkbase
```

## Security Hardening

### Network Policies

```yaml
# network-policy.yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: kkbase-network-policy
  namespace: kkbase
spec:
  podSelector:
    matchLabels:
      app: kkbase-integrated
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
  - to:
    - namespaceSelector:
        matchLabels:
          name: monitoring
    ports:
    - protocol: TCP
      port: 9090
```

### Pod Security Standards

```yaml
# pod-security.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: kkbase
  labels:
    pod-security.kubernetes.io/enforce: restricted
    pod-security.kubernetes.io/audit: restricted
    pod-security.kubernetes.io/warn: restricted
```

### RBAC Least Privilege

The watcher needs minimal permissions:

```yaml
# Ensure RBAC is minimal
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: kkbase-watcher
rules:
- apiGroups: [""]
  resources: ["pods", "services", "nodes", "namespaces"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["apps"]
  resources: ["deployments", "replicasets", "statefulsets", "daemonsets"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["networking.k8s.io"]
  resources: ["ingresses"]
  verbs: ["get", "list", "watch"]
```

## Troubleshooting

### Pod Crash Loop

```bash
# Check logs
kubectl logs -n kkbase -l app=kkbase-integrated -c mcp-server --previous

# Common issues:
# - Neo4j not ready: Wait for Neo4j to start
# - Invalid credentials: Check secrets
# - Prometheus unavailable: Check PROMETHEUS_URL
```

### Dashboard Not Loading

```bash
# Check ingress
kubectl describe ingress kkbase-dashboard -n kkbase

# Check service
kubectl get svc kkbase-integrated -n kkbase

# Test directly
kubectl port-forward -n kkbase svc/kkbase-integrated 8080:8080
curl http://localhost:8080/

# If 404: Frontend not embedded correctly, rebuild
```

### High Memory Usage

```bash
# Check Neo4j memory
kubectl top pod neo4j-0 -n kkbase

# If high:
# 1. Increase memory limits
# 2. Tune Neo4j config
# 3. Archive old sessions
```

## Maintenance

### Archive Old Sessions

```cypher
// Archive sessions older than 30 days
MATCH (s:AgentSession)
WHERE s.created_at < datetime() - duration({days: 30})
  AND s.status = "completed"
DETACH DELETE s
```

### Cleanup Orphaned Data

```cypher
// Remove findings without sessions
MATCH (f:Finding)
WHERE NOT (f)<-[:HAS_FINDING]-(:AgentSession)
DELETE f
```

## Next Steps

- Configure alerts for high query times
- Set up log aggregation (ELK/Loki)
- Enable distributed tracing
- Add authentication layer
- Configure rate limiting
- Set up automated backups

## See Also

- [Quick Start Guide](./quickstart-full-stack.md)
- [Agent Workflow](./agent-investigation-workflow.md)
- [MCP Tools Reference](../reference/agent-mcp-tools.md)

