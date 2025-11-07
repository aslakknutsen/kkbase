# Watcher Deployment Guide

This guide covers deploying the kkbase Watcher service to your Kubernetes cluster.

## Prerequisites

- **Kubernetes** v1.19+
- **kubectl** configured for your cluster
- **Cluster admin** permissions (for RBAC)
- **Neo4j** database (see below)
- **Helm** 3.x (recommended for Neo4j)

## Architecture

```
┌──────────────────────┐
│  Kubernetes Cluster  │
│                      │
│  ┌────────────────┐ │
│  │  kkbase-watcher│ │  ← Deployment
│  │  (ServiceAcct) │ │
│  └───────┬────────┘ │
│          │          │
│          │ Bolt     │
│          ↓          │
│  ┌────────────────┐ │
│  │     Neo4j      │ │  ← StatefulSet
│  │   (Storage)    │ │
│  └────────────────┘ │
└──────────────────────┘
```

## Step 1: Deploy Neo4j

### Option A: Helm (Recommended)

```bash
# Add repository
helm repo add neo4j https://helm.neo4j.com/neo4j
helm repo update

# Install with persistence
helm install neo4j neo4j/neo4j \
  --set neo4j.name=neo4j \
  --set neo4j.password=changeme \
  --set neo4j.edition=community \
  --set neo4j.acceptLicenseAgreement=yes \
  --set volumes.data.mode=defaultStorageClass \
  --set neo4j.resources.requests.memory=512Mi \
  --set neo4j.resources.requests.cpu=500m

# Wait for ready
kubectl wait --for=condition=ready pod -l app=neo4j --timeout=300s

# Verify
kubectl get pods -l app=neo4j
kubectl get svc neo4j
```

### Option B: Neo4j Aura (Managed Cloud)

1. Create account at https://neo4j.com/cloud/aura/
2. Create free instance
3. Save connection URI: `neo4j+s://xxxxx.databases.neo4j.io`
4. Save credentials
5. Use in configuration below

### Option C: Production Neo4j (Enterprise HA)

```bash
helm install neo4j neo4j/neo4j \
  --set neo4j.name=neo4j \
  --set neo4j.edition=enterprise \
  --set neo4j.password=changeme \
  --set neo4j.acceptLicenseAgreement=yes \
  --set neo4j.cluster.enabled=true \
  --set neo4j.cluster.servers=3 \
  --set neo4j.resources.requests.memory=2Gi \
  --set neo4j.resources.requests.cpu=1000m \
  --set persistence.size=20Gi
```

## Step 2: Create RBAC Resources

The watcher needs read-only cluster access.

Create `watcher-rbac.yaml`:

```yaml
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: kkbase-watcher
  namespace: default

---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: kkbase-watcher
rules:
# Core resources
- apiGroups: [""]
  resources:
    - nodes
    - namespaces
    - pods
    - services
    - endpoints
    - persistentvolumes
    - persistentvolumeclaims
    - configmaps
    - secrets
    - events
  verbs: ["get", "list", "watch"]

# Apps resources
- apiGroups: ["apps"]
  resources:
    - deployments
    - replicasets
    - statefulsets
    - daemonsets
  verbs: ["get", "list", "watch"]

# Batch resources
- apiGroups: ["batch"]
  resources:
    - jobs
    - cronjobs
  verbs: ["get", "list", "watch"]

# Networking
- apiGroups: ["networking.k8s.io"]
  resources:
    - ingresses
    - networkpolicies
  verbs: ["get", "list", "watch"]

# Storage
- apiGroups: ["storage.k8s.io"]
  resources:
    - storageclasses
  verbs: ["get", "list", "watch"]

# Gateway API (optional)
- apiGroups: ["gateway.networking.k8s.io"]
  resources:
    - gatewayclasses
    - gateways
    - httproutes
    - grpcroutes
    - tcproutes
    - udproutes
    - tlsroutes
    - referencegrants
  verbs: ["get", "list", "watch"]

# Istio (optional)
- apiGroups: ["networking.istio.io"]
  resources:
    - virtualservices
    - destinationrules
    - gateways
    - serviceentries
    - sidecars
  verbs: ["get", "list", "watch"]

- apiGroups: ["security.istio.io"]
  resources:
    - authorizationpolicies
    - peerauthentications
    - requestauthentications
  verbs: ["get", "list", "watch"]

---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: kkbase-watcher
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: kkbase-watcher
subjects:
- kind: ServiceAccount
  name: kkbase-watcher
  namespace: default
```

Apply:
```bash
kubectl apply -f watcher-rbac.yaml
```

## Step 3: Create Configuration

### Create Secret

```bash
kubectl create secret generic kkbase-watcher-secret \
  --from-literal=NEO4J_PASSWORD=your-secure-password
```

Or use YAML:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: kkbase-watcher-secret
  namespace: default
type: Opaque
stringData:
  NEO4J_PASSWORD: "changeme"
```

### Create ConfigMap

Create `watcher-config.yaml`:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: kkbase-watcher-config
  namespace: default
data:
  # Neo4j Connection
  NEO4J_URI: "bolt://neo4j:7687"
  NEO4J_USERNAME: "neo4j"
  NEO4J_DATABASE: "neo4j"
  
  # Watcher Configuration
  NAMESPACE: ""          # Watch all namespaces (empty) or specific namespace
  RESYNC_PERIOD: "30s"   # Full resync interval
  LOG_LEVEL: "info"      # debug, info, warn, error
  
  # Optional: Observability Integration
  # ENABLE_TRACES: "true"
  # JAEGER_QUERY_URL: "http://jaeger-query:16686"
  # PROMETHEUS_URL: "http://prometheus:9090"
```

Apply:
```bash
kubectl apply -f watcher-config.yaml
```

## Step 4: Deploy Watcher

Create `watcher-deployment.yaml`:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: kkbase-watcher
  namespace: default
  labels:
    app: kkbase-watcher
spec:
  replicas: 1  # Single replica sufficient
  selector:
    matchLabels:
      app: kkbase-watcher
  template:
    metadata:
      labels:
        app: kkbase-watcher
    spec:
      serviceAccountName: kkbase-watcher
      containers:
      - name: watcher
        image: kkbase-watcher:latest  # Replace with your image
        imagePullPolicy: IfNotPresent
        
        # Environment from ConfigMap
        envFrom:
        - configMapRef:
            name: kkbase-watcher-config
        
        # Password from Secret
        env:
        - name: NEO4J_PASSWORD
          valueFrom:
            secretKeyRef:
              name: kkbase-watcher-secret
              key: NEO4J_PASSWORD
        
        # Resource limits
        resources:
          limits:
            memory: "512Mi"
            cpu: "1000m"
          requests:
            memory: "256Mi"
            cpu: "500m"
        
        # Health checks
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 30
        
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 10
        
        # Graceful shutdown
        lifecycle:
          preStop:
            exec:
              command: ["/bin/sh", "-c", "sleep 5"]
      
      # Security
      securityContext:
        runAsNonRoot: true
        runAsUser: 65534
        fsGroup: 65534
```

Apply:
```bash
kubectl apply -f watcher-deployment.yaml
```

## Step 5: Verify Deployment

### Check Deployment Status

```bash
# Deployment status
kubectl get deployment kkbase-watcher

# Pod status
kubectl get pods -l app=kkbase-watcher

# Wait for ready
kubectl wait --for=condition=available deployment/kkbase-watcher --timeout=120s
```

### Check Logs

```bash
# View logs
kubectl logs -f deployment/kkbase-watcher

# Expected output:
# INFO  successfully connected to Neo4j  uri=bolt://neo4j:7687
# INFO  registered all watchers  count=15
# INFO  watcher started successfully
# INFO  all caches synced successfully
```

### Verify Neo4j Connection

```bash
# Check readiness probe
kubectl exec deployment/kkbase-watcher -- curl -s http://localhost:8080/ready

# Should return: {"status":"ready"}
```

### Verify Data in Neo4j

```bash
# Port forward to Neo4j browser
kubectl port-forward svc/neo4j 7474:7474

# Open http://localhost:7474
# Run query:
# MATCH (n) RETURN labels(n)[0] as type, count(*) as count
```

## Configuration Options

### Watch Specific Namespace

Edit ConfigMap:
```yaml
data:
  NAMESPACE: "production"  # Only watch production namespace
```

Apply and restart:
```bash
kubectl apply -f watcher-config.yaml
kubectl rollout restart deployment/kkbase-watcher
```

### Adjust Resource Limits

For large clusters (1000+ pods):
```yaml
resources:
  limits:
    memory: "1Gi"
    cpu: "2000m"
  requests:
    memory: "512Mi"
    cpu: "1000m"
```

### Enable Debug Logging

```yaml
data:
  LOG_LEVEL: "debug"
```

### Enable Tracing Integration

```yaml
data:
  ENABLE_TRACES: "true"
  JAEGER_QUERY_URL: "http://jaeger-query.observability:16686"
  JAEGER_POLL_INTERVAL: "30s"
  JAEGER_LOOKBACK_WINDOW: "5m"
  JAEGER_SPAN_RETENTION: "24h"
```

## Production Recommendations

### 1. Resource Limits

Set appropriate limits based on cluster size:

| Cluster Size | Memory | CPU |
|--------------|--------|-----|
| Small (<100 pods) | 256Mi | 500m |
| Medium (<500 pods) | 512Mi | 1000m |
| Large (<2000 pods) | 1Gi | 2000m |
| XLarge (>2000 pods) | 2Gi | 4000m |

### 2. High Availability Neo4j

Use Neo4j Enterprise with clustering:

```bash
helm install neo4j neo4j/neo4j \
  --set neo4j.edition=enterprise \
  --set neo4j.cluster.enabled=true \
  --set neo4j.cluster.servers=3 \
  --set persistence.size=50Gi
```

### 3. Network Policies

Restrict watcher egress:

```yaml
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

### 4. Pod Disruption Budget

Ensure graceful shutdown:

```yaml
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: kkbase-watcher-pdb
spec:
  minAvailable: 1
  selector:
    matchLabels:
      app: kkbase-watcher
```

### 5. Monitoring

Add Prometheus annotations:

```yaml
metadata:
  annotations:
    prometheus.io/scrape: "true"
    prometheus.io/port: "8080"
    prometheus.io/path: "/metrics"
```

## Troubleshooting

### Watcher Won't Start

```bash
# Check logs for errors
kubectl logs deployment/kkbase-watcher | grep ERROR

# Common issues:
# 1. Neo4j not accessible
kubectl exec deployment/kkbase-watcher -- nc -zv neo4j 7687

# 2. Wrong password
kubectl get secret kkbase-watcher-secret -o yaml

# 3. RBAC permissions missing
kubectl auth can-i list pods --as=system:serviceaccount:default:kkbase-watcher
```

### Resources Not Syncing

```bash
# Trigger full resync
kubectl rollout restart deployment/kkbase-watcher

# Check for specific errors
kubectl logs deployment/kkbase-watcher | grep -i "failed\|error"

# Verify caches synced
kubectl logs deployment/kkbase-watcher | grep "caches synced"
```

### High Memory Usage

```bash
# Check current usage
kubectl top pod -l app=kkbase-watcher

# Reduce scope by namespace
kubectl set env deployment/kkbase-watcher NAMESPACE=production

# Increase limits
kubectl set resources deployment/kkbase-watcher \
  --limits=memory=1Gi,cpu=2000m \
  --requests=memory=512Mi,cpu=1000m
```

### Neo4j Connection Lost

```bash
# Check Neo4j status
kubectl get pods -l app=neo4j

# Test connectivity
kubectl exec deployment/kkbase-watcher -- \
  curl -f bolt://neo4j:7687

# Check Neo4j logs
kubectl logs neo4j-0
```

## Upgrading

### Upgrade Watcher

```bash
# Update image
kubectl set image deployment/kkbase-watcher \
  watcher=kkbase-watcher:v2.0.0

# Monitor rollout
kubectl rollout status deployment/kkbase-watcher

# Verify
kubectl logs -f deployment/kkbase-watcher
```

### Upgrade Neo4j

```bash
# Backup first!
kubectl exec neo4j-0 -- neo4j-admin database dump neo4j

# Upgrade via Helm
helm upgrade neo4j neo4j/neo4j \
  --set neo4j.password=changeme \
  --reuse-values

# Verify
kubectl logs neo4j-0
```

## Clean Up

```bash
# Remove watcher
kubectl delete deployment kkbase-watcher
kubectl delete configmap kkbase-watcher-config
kubectl delete secret kkbase-watcher-secret
kubectl delete -f watcher-rbac.yaml

# Remove Neo4j (if desired)
helm uninstall neo4j
kubectl delete pvc data-neo4j-0  # Remove data
```

## Next Steps

- **[Configuration Guide](configuration.md)** - Detailed configuration options
- **[Extensions](extensions.md)** - Enable Gateway API and Istio support
- **[Custom Handlers](custom-handlers.md)** - Track custom CRDs
- **[Query Guide](../../guides/querying/)** - Learn to query the graph
- **[Operations](../../guides/operations/)** - Monitoring and troubleshooting

## Quick Reference

```bash
# Deploy
kubectl apply -f watcher-rbac.yaml
kubectl apply -f watcher-config.yaml
kubectl create secret generic kkbase-watcher-secret --from-literal=NEO4J_PASSWORD=changeme
kubectl apply -f watcher-deployment.yaml

# Verify
kubectl get deployment kkbase-watcher
kubectl logs -f deployment/kkbase-watcher

# Update config
kubectl edit configmap kkbase-watcher-config
kubectl rollout restart deployment/kkbase-watcher

# Scale (usually 1 replica sufficient)
kubectl scale deployment kkbase-watcher --replicas=1

# Delete
kubectl delete deployment kkbase-watcher
```

