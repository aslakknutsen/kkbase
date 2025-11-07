# Installation Guide

This guide provides detailed instructions for deploying kkbase in your Kubernetes cluster.

## Prerequisites

### Kubernetes Cluster

- Kubernetes v1.19 or later
- `kubectl` configured to access your cluster
- Cluster admin permissions (for RBAC setup)

Supported environments:
- Cloud providers: GKE, EKS, AKS
- Local development: minikube, kind, k3s
- On-premises clusters

### Neo4j Database

- Neo4j v4.0 or later (Community or Enterprise edition)
- Persistent storage for database
- Network connectivity from kkbase pods

## Step 1: Deploy Neo4j

### Option A: Helm Installation (Recommended)

```bash
# Add Neo4j Helm repository
helm repo add neo4j https://helm.neo4j.com/neo4j
helm repo update

# Install Neo4j with persistence
helm install neo4j neo4j/neo4j \
  --set neo4j.name=neo4j \
  --set neo4j.password=changeme \
  --set neo4j.edition=community \
  --set neo4j.acceptLicenseAgreement=yes \
  --set volumes.data.mode=defaultStorageClass

# Wait for Neo4j to be ready
kubectl wait --for=condition=ready pod -l app=neo4j --timeout=300s
```

### Option B: Neo4j Aura (Cloud)

Use Neo4j's managed cloud service:

1. Create a free instance at https://neo4j.com/cloud/aura/
2. Note the connection URI and credentials
3. Update `NEO4J_URI` in the configuration below

## Step 2: Configure kkbase

### Create Configuration Secret

Edit `deploy/secret.yaml` with your Neo4j password:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: kkbase-watcher-secret
  namespace: default
type: Opaque
stringData:
  NEO4J_PASSWORD: "changeme"  # Replace with your password
```

### Configure Settings

Edit `deploy/configmap.yaml`:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: kkbase-watcher-config
  namespace: default
data:
  NEO4J_URI: "bolt://neo4j:7687"  # Update if using Neo4j Aura
  NEO4J_USERNAME: "neo4j"
  NEO4J_DATABASE: "neo4j"
  NAMESPACE: ""                    # Empty = watch all namespaces
  RESYNC_PERIOD: "30s"
  LOG_LEVEL: "info"
  ENABLE_METRICS: "false"
  ENABLE_LOGS: "false"
  ENABLE_TRACES: "false"
```

See [Configuration Reference](../reference/configuration.md) for all options.

## Step 3: Deploy kkbase

### Apply Manifests

```bash
# Create RBAC resources
kubectl apply -f deploy/rbac.yaml

# Create configuration
kubectl apply -f deploy/configmap.yaml
kubectl apply -f deploy/secret.yaml

# Deploy the watcher
kubectl apply -f deploy/deployment.yaml
```

### Verify Deployment

```bash
# Check pod status
kubectl get pods -l app=kkbase-watcher

# View logs
kubectl logs -f deployment/kkbase-watcher

# Expected log output:
# - "successfully connected to Neo4j"
# - "registered all watchers"
# - "watcher started successfully"
# - "all caches synced successfully"
```

## Step 4: Access Neo4j Browser

### Port Forward to Neo4j

```bash
kubectl port-forward svc/neo4j 7474:7474 7687:7687
```

Open your browser to: http://localhost:7474

- **Username**: `neo4j`
- **Password**: Your configured password

### Verify Data

Run this query in Neo4j Browser:

```cypher
MATCH (n)
RETURN labels(n)[0] as type, count(*) as count
ORDER BY count DESC
```

You should see your cluster resources populating the graph.

## Building from Source

### Build Binary

```bash
# Clone repository
git clone https://github.com/kagenti/kkbase.git
cd kkbase

# Build
go build -o watcher ./cmd/watcher
```

### Build Docker Image

```bash
# Build image
docker build -t kkbase-watcher:latest .

# Push to registry
docker tag kkbase-watcher:latest your-registry/kkbase-watcher:latest
docker push your-registry/kkbase-watcher:latest

# Update deployment to use your image
kubectl set image deployment/kkbase-watcher \
  kkbase-watcher=your-registry/kkbase-watcher:latest
```

## Configuration Options

### Watch Specific Namespace

To watch only a specific namespace:

```yaml
# In deploy/configmap.yaml
data:
  NAMESPACE: "production"
```

### Adjust Resync Period

Control how often the system resyncs with Kubernetes:

```yaml
data:
  RESYNC_PERIOD: "60s"  # Sync every 60 seconds
```

### Enable Debug Logging

For troubleshooting:

```yaml
data:
  LOG_LEVEL: "debug"
```

## Troubleshooting

### Pods Not Showing Up

```bash
# Check watcher is running
kubectl get pods -l app=kkbase-watcher

# Check for errors
kubectl logs deployment/kkbase-watcher | grep -i error
```

### Connection Issues

```bash
# Verify Neo4j is accessible
kubectl exec -it deployment/kkbase-watcher -- nc -zv neo4j 7687

# Check service endpoints
kubectl get endpoints neo4j
```

### Graph is Empty

```bash
# Restart watcher to trigger full resync
kubectl rollout restart deployment/kkbase-watcher

# Check if informers synced
kubectl logs deployment/kkbase-watcher | grep "caches synced"
```

### RBAC Permission Errors

If you see "forbidden" errors in logs:

```bash
# Verify ClusterRole permissions
kubectl describe clusterrole kkbase-watcher

# Check ServiceAccount binding
kubectl describe clusterrolebinding kkbase-watcher
```

## Upgrading

### Upgrade kkbase

```bash
# Pull latest image
kubectl set image deployment/kkbase-watcher \
  kkbase-watcher=your-registry/kkbase-watcher:v1.x.x

# Or reapply manifests
kubectl apply -f deploy/deployment.yaml
```

### Upgrade Neo4j

Refer to Neo4j Helm chart documentation for upgrade procedures. Ensure database backup before upgrading.

## Uninstalling

### Remove kkbase

```bash
kubectl delete -f deploy/deployment.yaml
kubectl delete -f deploy/configmap.yaml
kubectl delete -f deploy/secret.yaml
kubectl delete -f deploy/rbac.yaml
```

### Remove Neo4j

```bash
helm uninstall neo4j
```

## Next Steps

- **[Quick Start Guide](quickstart.md)** - Run your first queries
- **[Querying Guide](querying.md)** - Learn common query patterns
- **[Extensions](extensions.md)** - Enable Gateway API and Istio support
- **[Configuration Reference](../reference/configuration.md)** - All configuration options

