# Quick Start: Knowledge Graph Only

Get a queryable knowledge graph of your Kubernetes cluster in 10 minutes.

## What You'll Get

- Watcher service syncing your cluster to Neo4j
- Real-time graph of all resources and relationships
- Cypher query access via Neo4j Browser

## Prerequisites

- Kubernetes cluster (v1.19+) - minikube, kind, k3s, or cloud
- kubectl configured
- Helm 3.x

## Step 1: Deploy Neo4j (3 minutes)

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
```

**Verify**:
```bash
kubectl get pods -l app=neo4j
# Should show: neo4j-0   1/1   Running
```

## Step 2: Configure kkbase (2 minutes)

Create secret with Neo4j password:

```bash
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Secret
metadata:
  name: kkbase-watcher-secret
  namespace: default
type: Opaque
stringData:
  NEO4J_PASSWORD: "changeme"
EOF
```

Create configuration:

```bash
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: kkbase-watcher-config
  namespace: default
data:
  NEO4J_URI: "bolt://neo4j:7687"
  NEO4J_USERNAME: "neo4j"
  NEO4J_DATABASE: "neo4j"
  LOG_LEVEL: "info"
  NAMESPACE: ""  # Watch all namespaces
  RESYNC_PERIOD: "30s"
EOF
```

## Step 3: Deploy kkbase Watcher (3 minutes)

```bash
# Deploy RBAC, ConfigMap, Secret, and Watcher
kubectl apply -f https://raw.githubusercontent.com/kagenti/kkbase/main/deploy/rbac.yaml
kubectl apply -f https://raw.githubusercontent.com/kagenti/kkbase/main/deploy/deployment.yaml

# Wait for deployment
kubectl wait --for=condition=available deployment/kkbase-watcher --timeout=120s
```

**Verify**:
```bash
kubectl logs deployment/kkbase-watcher | tail -20
```

Expected output:
```
INFO  successfully connected to Neo4j  uri=bolt://neo4j:7687
INFO  registered all watchers  count=15
INFO  watcher started successfully
INFO  all caches synced successfully
```

## Step 4: Access Neo4j Browser (2 minutes)

Port forward to Neo4j:

```bash
kubectl port-forward svc/neo4j 7474:7474 7687:7687
```

Open browser: http://localhost:7474

- **Username**: neo4j
- **Password**: changeme
- **Connect URL**: neo4j://localhost:7687

## Example Queries

### See What's in the Graph

```cypher
// Count resources by type
MATCH (n)
RETURN labels(n)[0] as type, count(*) as count
ORDER BY count DESC
```

### View All Pods

```cypher
MATCH (p:Pod)
RETURN p.namespace, p.name, p.status, p.ip
ORDER BY p.namespace, p.name
LIMIT 20
```

### Deployment Hierarchy

```cypher
MATCH path = (d:Deployment)-[:MANAGES]->(rs:ReplicaSet)
             -[:MANAGES]->(p:Pod)
WHERE d.namespace = 'default'
RETURN path
LIMIT 50
```

### Service to Pods

```cypher
MATCH (s:Service)-[:SELECTS_PODS]->(p:Pod)
RETURN s.namespace, 
       s.name as service,
       collect(p.name) as pods
LIMIT 10
```

### Node Resources

```cypher
MATCH (n:Node)<-[:SCHEDULED_ON]-(p:Pod)
RETURN n.name,
       n.status,
       count(p) as pod_count,
       n.cpu_capacity,
       n.memory_capacity
```

### Recent Events

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

### Impact Analysis

```cypher
// What breaks if this node fails?
MATCH (n:Node {name: 'your-node-name'})<-[:SCHEDULED_ON]-(p:Pod)
OPTIONAL MATCH (p)<-[:SELECTS_PODS]-(s:Service)
OPTIONAL MATCH (p)<-[:MANAGES]-()<-[:MANAGES]-(d:Deployment)
RETURN n.name as node,
       count(DISTINCT p) as pods,
       count(DISTINCT s) as services,
       count(DISTINCT d) as deployments
```

## Configuration Options

### Watch Specific Namespace Only

Edit the ConfigMap:

```yaml
data:
  NAMESPACE: "production"  # Only watch this namespace
```

Apply and restart:
```bash
kubectl apply -f <configmap-file>
kubectl rollout restart deployment/kkbase-watcher
```

### Adjust Resync Period

```yaml
data:
  RESYNC_PERIOD: "60s"  # Sync every 60 seconds
```

### Enable Debug Logging

```yaml
data:
  LOG_LEVEL: "debug"  # More verbose output
```

## Troubleshooting

### Pods Not Showing Up?

Check watcher is running:
```bash
kubectl get pods -l app=kkbase-watcher
kubectl logs deployment/kkbase-watcher | grep -i error
```

### Connection Issues?

Test Neo4j connectivity:
```bash
kubectl exec -it deployment/kkbase-watcher -- nc -zv neo4j 7687
```

### Graph is Empty?

Restart watcher to trigger full resync:
```bash
kubectl rollout restart deployment/kkbase-watcher
kubectl logs -f deployment/kkbase-watcher | grep "caches synced"
```

Wait for "all caches synced successfully" message.

### Neo4j Won't Start?

Check Neo4j logs:
```bash
kubectl logs neo4j-0
```

Common issues:
- Insufficient memory (needs 512MB+)
- Storage class not available
- License not accepted

## Next Steps

### Add Optional Features

**Enable Prometheus Integration** (metrics-based RCA):
- See [Full Stack Quick Start](quickstart-with-agent.md)

**Add Gateway API Support**:
- See [Extensions Guide](../services/watcher/extensions.md)

**Add Istio Support**:
- See [Extensions Guide](../services/watcher/extensions.md)

### Learn to Query

**Basic Queries**:
- [Query Guide: Basics](../guides/querying/basics.md)

**Relationship Traversal**:
- [Query Guide: Relationships](../guides/querying/relationships.md)

**Advanced Patterns**:
- [Query Guide: Advanced](../guides/querying/advanced.md)

### Add AI Agent Integration

**Use AI Tools** (Cursor, Claude):
- Deploy MCP Server: [Full Stack Quick Start](quickstart-with-agent.md)

**Build Custom Tools**:
- Neo4j drivers for your language
- Custom Cypher queries
- GraphQL wrapper (build your own)

### Extend the Watcher

**Track Custom Resources**:
- [Custom Handlers Guide](../services/watcher/custom-handlers.md)

**Contribute**:
- [Development Guide](../development/)

## Clean Up

```bash
# Remove kkbase
kubectl delete deployment kkbase-watcher
kubectl delete configmap kkbase-watcher-config
kubectl delete secret kkbase-watcher-secret
kubectl delete -f https://raw.githubusercontent.com/kagenti/kkbase/main/deploy/rbac.yaml

# Remove Neo4j
helm uninstall neo4j
```

## Resources

- [System Architecture](../ARCHITECTURE.md)
- [Core Concepts](concepts.md)
- [Graph Schema Reference](../reference/graph-schema.md)
- [Cypher Query Library](../reference/cypher-queries.md)
- [Operations Guide](../guides/operations/)

## Summary

You now have:
- ✅ Neo4j graph database with cluster data
- ✅ Watcher continuously syncing resources
- ✅ Query access via Neo4j Browser
- ✅ Real-time topology updates

**Ready to query!** Try the example queries above or explore the [Query Guide](../guides/querying/basics.md).

