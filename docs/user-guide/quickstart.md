# Quick Start Guide

## Prerequisites

1. **Kubernetes Cluster** (v1.19+)
   - Local: minikube, kind, k3s
   - Cloud: GKE, EKS, AKS

2. **Neo4j Database** (v4.0+)
   - Install via Helm (recommended)
   - Or use Neo4j Aura (cloud service)

3. **Prometheus** (Optional - for metrics-based RCA)
   - Enables investigation tools in MCP server
   - Install via kube-prometheus-stack (see Step 3a)
   - Or use existing Prometheus instance

## 5-Minute Setup

### Step 1: Deploy Neo4j

```bash
# Add Neo4j Helm repository
helm repo add neo4j https://helm.neo4j.com/neo4j
helm repo update

# Install Neo4j
helm install neo4j neo4j/neo4j \
  --set neo4j.name=neo4j \
  --set neo4j.password=changeme \
  --set neo4j.edition=community \
  --set neo4j.acceptLicenseAgreement=yes \
  --set volumes.data.mode=defaultStorageClass

# Wait for Neo4j to be ready
kubectl wait --for=condition=ready pod -l app=neo4j --timeout=300s

# Get Neo4j service
kubectl get svc neo4j
```

### Step 2: Configure kkbase

Edit the secret with your Neo4j password:

```bash
# Edit deploy/secret.yaml
cat <<EOF > deploy/secret.yaml
---
apiVersion: v1
kind: Secret
metadata:
  name: kkbase-watcher-secret
  namespace: default
type: Opaque
stringData:
  NEO4J_PASSWORD: "changeme"  # Use your actual password
EOF
```

### Step 3: Deploy kkbase

```bash
# Deploy all resources
kubectl apply -f deploy/rbac.yaml
kubectl apply -f deploy/configmap.yaml
kubectl apply -f deploy/secret.yaml
kubectl apply -f deploy/deployment.yaml

# Wait for deployment
kubectl wait --for=condition=available deployment/kkbase-watcher --timeout=120s
```

### Step 3a: Deploy Prometheus (Optional - Enables Metrics Investigation)

Prometheus enables the metrics-based RCA investigation tools in the MCP server.

#### Using kube-prometheus-stack (Recommended)

```bash
# Add Prometheus community Helm repository
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update

# Install Prometheus with default configuration
helm install prometheus prometheus-community/kube-prometheus-stack \
  --namespace monitoring \
  --create-namespace \
  --set prometheus.prometheusSpec.serviceMonitorSelectorNilUsesHelmValues=false

# Wait for Prometheus to be ready
kubectl wait --for=condition=ready pod \
  -l app.kubernetes.io/name=prometheus \
  -n monitoring \
  --timeout=300s
```

#### Verify Prometheus is Working

```bash
# Check Prometheus pods
kubectl get pods -n monitoring

# Port forward to access Prometheus UI
kubectl port-forward -n monitoring svc/prometheus-kube-prometheus-prometheus 9090:9090

# Open browser to: http://localhost:9090
# Try a test query: up{job="kubernetes-nodes"}
```

#### Configure kkbase to Use Prometheus

Update your ConfigMap to enable metrics investigation:

```bash
cat <<EOF >> deploy/configmap.yaml
  # Enable Prometheus metrics investigation (optional)
  PROMETHEUS_URL: "http://prometheus-kube-prometheus-prometheus.monitoring.svc:9090"
EOF

# Apply the updated ConfigMap
kubectl apply -f deploy/configmap.yaml

# Restart kkbase to pick up the change
kubectl rollout restart deployment/kkbase-watcher
```

**For MCP Server deployment**, add the same configuration:

```bash
# If using standalone MCP server deployment
cat <<EOF >> deploy/mcp-server-deployment.yaml
        - name: PROMETHEUS_URL
          value: "http://prometheus-kube-prometheus-prometheus.monitoring.svc:9090"
EOF
```

#### Verify Metrics Integration

```bash
# Check logs for metrics integration message
kubectl logs deployment/kkbase-watcher | grep "metrics integration"

# You should see:
# "metrics integration enabled - investigation tools available"
# "registered MCP tools" [..., "start_investigation", "complete_investigation", "get_investigation_status"]
```

#### Alternative: Minimal Prometheus Deployment

If you don't need the full kube-prometheus-stack, use a minimal Prometheus:

```bash
# Deploy minimal Prometheus
kubectl apply -f - <<EOF
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: prometheus-config
  namespace: monitoring
data:
  prometheus.yml: |
    global:
      scrape_interval: 15s
    scrape_configs:
      - job_name: 'kubernetes-pods'
        kubernetes_sd_configs:
          - role: pod
        relabel_configs:
          - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_scrape]
            action: keep
            regex: true
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: prometheus
  namespace: monitoring
spec:
  replicas: 1
  selector:
    matchLabels:
      app: prometheus
  template:
    metadata:
      labels:
        app: prometheus
    spec:
      serviceAccountName: prometheus
      containers:
      - name: prometheus
        image: prom/prometheus:latest
        args:
          - '--config.file=/etc/prometheus/prometheus.yml'
          - '--storage.tsdb.path=/prometheus'
        ports:
        - containerPort: 9090
        volumeMounts:
        - name: config
          mountPath: /etc/prometheus
        - name: storage
          mountPath: /prometheus
      volumes:
      - name: config
        configMap:
          name: prometheus-config
      - name: storage
        emptyDir: {}
---
apiVersion: v1
kind: Service
metadata:
  name: prometheus
  namespace: monitoring
spec:
  selector:
    app: prometheus
  ports:
  - port: 9090
    targetPort: 9090
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: prometheus
  namespace: monitoring
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: prometheus
rules:
- apiGroups: [""]
  resources:
  - nodes
  - nodes/proxy
  - services
  - endpoints
  - pods
  verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: prometheus
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: prometheus
subjects:
- kind: ServiceAccount
  name: prometheus
  namespace: monitoring
EOF
```

### Step 4: Verify

```bash
# Check logs
kubectl logs -f deployment/kkbase-watcher

# You should see:
# - "successfully connected to Neo4j"
# - "registered all watchers"
# - "watcher started successfully"
# - "all caches synced successfully"
```

## Access Neo4j Browser

### Port Forward to Neo4j

```bash
kubectl port-forward svc/neo4j 7474:7474 7687:7687
```

Open browser to: http://localhost:7474

- **Username**: neo4j
- **Password**: changeme (or your password)

## Example Queries

### 1. See All Nodes

```cypher
MATCH (n)
RETURN labels(n)[0] as type, count(*) as count
ORDER BY count DESC
```

### 2. View Cluster Topology

```cypher
MATCH (n:Namespace)
OPTIONAL MATCH (n)<-[:IN_NAMESPACE]-(r)
RETURN n, r
LIMIT 100
```

### 3. Find All Pods

```cypher
MATCH (p:Pod)
RETURN p.namespace, p.name, p.status, p.ip
ORDER BY p.namespace, p.name
```

### 4. Deployment Hierarchy

```cypher
MATCH path = (d:Deployment)-[:MANAGES]->(rs:ReplicaSet)-[:MANAGES]->(p:Pod)
RETURN path
LIMIT 50
```

### 5. Service to Pods

```cypher
MATCH (s:Service)-[:SELECTS_PODS]->(p:Pod)
RETURN s.name, s.namespace, collect(p.name) as pods
```

### 6. Node Resources

```cypher
MATCH (n:Node)<-[:SCHEDULED_ON]-(p:Pod)
RETURN n.name,
       n.status,
       count(p) as pod_count,
       n.cpu_capacity,
       n.memory_capacity
```

### 7. Recent Events

```cypher
MATCH (e:K8sEvent)-[:INVOLVES]->(r)
RETURN e.reason,
       e.type,
       e.message,
       labels(r)[0] as resource_type,
       r.name as resource_name
ORDER BY e.last_timestamp DESC
LIMIT 20
```

### 8. Pods Using ConfigMaps

```cypher
MATCH (p:Pod)-[:USES_CONFIG]->(cm:ConfigMap)
RETURN p.namespace, p.name, collect(cm.name) as configmaps
```

### 9. Storage Chain

```cypher
MATCH path = (p:Pod)-[:MOUNTS]->(pvc:PersistentVolumeClaim)-[:BOUND_TO]->(pv:PersistentVolume)
RETURN path
```

### 10. Impact Analysis 

```cypher
// What happens if this node fails?
MATCH (n:Node {name: 'your-node-name'})<-[:SCHEDULED_ON]-(p:Pod)
OPTIONAL MATCH (p)<-[:SELECTS_PODS]-(s:Service)
OPTIONAL MATCH (p)<-[:MANAGES]-()<-[:MANAGES]-(d:Deployment)
RETURN n, p, s, d
```

## Troubleshooting

### Pods Not Showing Up?

```bash
# Check watcher is running
kubectl get pods -l app=kkbase-watcher

# Check for errors
kubectl logs deployment/kkbase-watcher | grep -i error
```

### Connection Issues?

```bash
# Verify Neo4j is accessible
kubectl exec -it deployment/kkbase-watcher -- nc -zv neo4j 7687

# Check service endpoints
kubectl get endpoints neo4j
```

### Graph is Empty?

```bash
# Restart watcher to trigger full resync
kubectl rollout restart deployment/kkbase-watcher

# Check if informers synced
kubectl logs deployment/kkbase-watcher | grep "caches synced"
```

### Investigation Tools Not Available?

```bash
# Check if Prometheus URL is configured
kubectl get configmap kkbase-watcher-config -o yaml | grep PROMETHEUS_URL

# Verify Prometheus is accessible from kkbase
kubectl exec -it deployment/kkbase-watcher -- \
  curl -s http://prometheus-kube-prometheus-prometheus.monitoring.svc:9090/api/v1/status/config

# Check logs for metrics integration status
kubectl logs deployment/kkbase-watcher | grep "metrics integration"
```

### Prometheus Connection Failed?

```bash
# Test Prometheus connectivity
kubectl run -it --rm debug --image=curlimages/curl --restart=Never -- \
  curl http://prometheus-kube-prometheus-prometheus.monitoring.svc:9090/-/healthy

# Check Prometheus service
kubectl get svc -n monitoring | grep prometheus

# Verify DNS resolution
kubectl exec -it deployment/kkbase-watcher -- \
  nslookup prometheus-kube-prometheus-prometheus.monitoring.svc
```

## Configuration Options

### Watch Specific Namespace Only

Edit `deploy/configmap.yaml`:
```yaml
data:
  NAMESPACE: "production"  # Only watch this namespace
```

### Adjust Resync Period

```yaml
data:
  RESYNC_PERIOD: "60s"  # Sync every 60 seconds
```

### Change Log Level

```yaml
data:
  LOG_LEVEL: "debug"  # debug, info, warn, error
```

## Next Steps

1. **Enable Metrics**: Deploy Prometheus (see Step 3a) to enable RCA investigation tools
2. **Use MCP Tools**: Try the [Investigation Tools](./investigation-tools.md) for AI-powered RCA
3. **Visualize**: Use Neo4j Bloom for visual graph exploration
4. **Query**: Write custom Cypher queries for your use cases - see [Metrics RCA Queries](../reference/metrics-rca-queries.md)
5. **Integrate**: Use Neo4j drivers to query from your apps
6. **Extend**: Add custom resource handlers for CRDs
7. **Monitor**: Add Grafana dashboards for visualization

## Clean Up

```bash
# Remove kkbase
kubectl delete -f deploy/

# Remove Neo4j
helm uninstall neo4j

# Remove Prometheus (if installed)
helm uninstall prometheus -n monitoring
# Or for minimal deployment:
kubectl delete namespace monitoring
```

## Resources

- **Neo4j Cypher Manual**: https://neo4j.com/docs/cypher-manual/
- **Kubernetes API**: https://kubernetes.io/docs/reference/kubernetes-api/
- **Prometheus Docs**: https://prometheus.io/docs/
- **Documentation**: See [Documentation Index](../README.md)
- **Query Reference**: See [Cypher Queries](../reference/cypher-queries.md)
- **Investigation Tools**: See [Investigation Tools Guide](./investigation-tools.md)
- **Metrics RCA**: See [Metrics RCA Queries](../reference/metrics-rca-queries.md)

## Support

For issues or questions:
1. Check logs: `kubectl logs deployment/kkbase-watcher`
2. Review configuration in ConfigMap and Secret
3. Verify RBAC permissions are correct
4. Test Neo4j connectivity manually

## Tips

- Start with a small namespace to test
- Use `LIMIT` in Cypher queries to avoid overwhelming results
- Create indexes in Neo4j for custom queries
- Use Neo4j Browser's query history
- Export interesting queries as bookmarks

