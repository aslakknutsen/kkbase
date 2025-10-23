# Kubernetes Knowledge Base (kkbase)

A Kubernetes operator that watches cluster resources and syncs them to a Neo4j graph database, creating a real-time knowledge graph of your entire cluster topology.

## Overview

kkbase implements the knowledge graph model described in `basic-idea.md`, creating a living representation of your Kubernetes cluster with:

- **Nodes**: All Kubernetes resources (Pods, Services, Deployments, etc.)
- **Edges**: Relationships between resources (MANAGES, SELECTS_PODS, SCHEDULED_ON, etc.)
- **Properties**: Detailed metadata about each resource

This graph serves as a powerful foundation for:
- Autonomous diagnostic agents
- Root cause analysis
- Impact analysis
- Dependency mapping
- Cluster visualization

## Architecture

### Components

1. **Graph Database Layer** (`pkg/graph/`)
   - Abstract `GraphStore` interface
   - Neo4j implementation with connection pooling and retry logic

2. **Resource Models** (`pkg/models/`)
   - Node and Edge type definitions
   - Converters from Kubernetes objects to graph nodes

3. **Watcher Framework** (`pkg/watchers/`)
   - Generic watcher using client-go informers
   - Relationship builder for creating edges
   - Handler registration system

4. **Resource Handlers** (`pkg/watchers/handlers/`)
   - Per-resource handlers for all Kubernetes types
   - Automatic relationship extraction

5. **Observability Interfaces** (`pkg/observability/`)
   - Plugin interfaces for metrics, logs, and traces
   - Extensible for future integration

## Deployment

### Prerequisites

- Kubernetes cluster (v1.19+)
- Neo4j database (v4.0+)

### Quick Start

1. **Deploy Neo4j** (if not already available):

```bash
helm repo add neo4j https://helm.neo4j.com/neo4j
helm install neo4j neo4j/neo4j \
  --set neo4j.name=neo4j \
  --set neo4j.password=changeme \
  --set neo4j.edition=community \
  --set neo4j.acceptLicenseAgreement=yes \
  --set volumes.data.mode=defaultStorageClass
```

2. **Configure the watcher**:

Edit `deploy/secret.yaml` and set your Neo4j password:

```yaml
stringData:
  NEO4J_PASSWORD: "your-neo4j-password"
```

Edit `deploy/configmap.yaml` to configure settings:

```yaml
data:
  NEO4J_URI: "bolt://neo4j:7687"  # Update if needed
  NAMESPACE: ""                    # Empty = all namespaces
  RESYNC_PERIOD: "30s"
  LOG_LEVEL: "info"
```

3. **Deploy the watcher**:

```bash
kubectl apply -f deploy/rbac.yaml
kubectl apply -f deploy/configmap.yaml
kubectl apply -f deploy/secret.yaml
kubectl apply -f deploy/deployment.yaml
```

4. **Verify deployment**:

```bash
kubectl logs -f deployment/kkbase-watcher
```

### Building from Source

```bash
# Build the binary
go build -o watcher ./cmd/watcher

# Build the Docker image
docker build -t kkbase-watcher:latest .

# Push to your registry
docker tag kkbase-watcher:latest your-registry/kkbase-watcher:latest
docker push your-registry/kkbase-watcher:latest
```

## Configuration

Configuration is done via environment variables:

| Variable | Description | Default |
|----------|-------------|---------|
| `NEO4J_URI` | Neo4j connection URI | `bolt://localhost:7687` |
| `NEO4J_USERNAME` | Neo4j username | `neo4j` |
| `NEO4J_PASSWORD` | Neo4j password | *Required* |
| `NEO4J_DATABASE` | Neo4j database name | `neo4j` |
| `KUBECONFIG` | Path to kubeconfig (local dev only) | Empty (uses in-cluster config) |
| `NAMESPACE` | Namespace to watch (empty = all) | Empty |
| `RESYNC_PERIOD` | How often to resync | `30s` |
| `LOG_LEVEL` | Logging level (debug, info, warn, error) | `info` |
| `ENABLE_METRICS` | Enable metrics integration | `false` |
| `ENABLE_LOGS` | Enable logs integration | `false` |
| `ENABLE_TRACES` | Enable traces integration | `false` |

## Graph Schema

### Node Types

The following Kubernetes resources are represented as nodes:

**Compute & Hardware**
- `Cluster`, `Node`

**Workloads**
- `Pod`, `Container`, `Deployment`, `ReplicaSet`, `StatefulSet`, `DaemonSet`

**Networking**
- `Service`, `Ingress`, `Endpoint`, `NetworkPolicy`

**Storage**
- `PersistentVolume`, `PersistentVolumeClaim`, `StorageClass`

**Configuration**
- `ConfigMap`, `Secret`, `Namespace`

**Observability**
- `K8sEvent`

### Edge Types

Relationships between resources:

**Structural**
- `MANAGES`: Deployment → ReplicaSet → Pod
- `CONTAINS`: Pod → Container
- `SCHEDULED_ON`: Pod → Node
- `IN_NAMESPACE`: Resource → Namespace

**Networking**
- `SELECTS_PODS`: Service → Pod
- `ROUTES_TO`: Ingress → Service

**Storage**
- `MOUNTS`: Pod → PVC
- `BOUND_TO`: PVC → PV
- `PROVISIONED_BY`: PV → StorageClass

**Configuration**
- `USES_CONFIG`: Pod → ConfigMap
- `USES_SECRET`: Pod → Secret

**Observability**
- `INVOLVES`: Event → Resource

## Querying the Graph

Once running, you can query the graph using Cypher via Neo4j Browser or API.

### Example Queries

**Find all pods on a specific node:**
```cypher
MATCH (n:Node {name: 'node-1'})<-[:SCHEDULED_ON]-(p:Pod)
RETURN p.name, p.namespace, p.status
```

**Find all pods selected by a service:**
```cypher
MATCH (s:Service {name: 'my-service', namespace: 'default'})-[:SELECTS_PODS]->(p:Pod)
RETURN p.name, p.status, p.ip
```

**Find deployment hierarchy:**
```cypher
MATCH path = (d:Deployment)-[:MANAGES]->(rs:ReplicaSet)-[:MANAGES]->(p:Pod)
WHERE d.name = 'my-app'
RETURN path
```

**Find all resources in a namespace:**
```cypher
MATCH (r)-[:IN_NAMESPACE]->(ns:Namespace {name: 'production'})
RETURN labels(r)[0] as type, r.name as name
```

**Find pods with errors (via events):**
```cypher
MATCH (e:K8sEvent {type: 'Warning'})-[:INVOLVES]->(p:Pod)
RETURN p.name, p.namespace, e.reason, e.message
ORDER BY e.last_timestamp DESC
LIMIT 10
```

**Impact analysis - what's affected by a node failure:**
```cypher
MATCH (n:Node {name: 'failed-node'})<-[:SCHEDULED_ON]-(p:Pod)
OPTIONAL MATCH (s:Service)-[:SELECTS_PODS]->(p)
OPTIONAL MATCH (d:Deployment)-[:MANAGES]->()-[:MANAGES]->(p)
RETURN n, p, s, d
```

**Find all node types and relationships currently found:**
```cypher
MATCH (a)-[r]->(b)
WITH labels(a)[0] as FromNode, type(r) as Relationship, labels(b)[0] as ToNode, count(*) as Count
RETURN FromNode + ' -[' + Relationship + ']-> ' + ToNode as Pattern, Count
ORDER BY Count DESC
LIMIT 50
```

## Development

### Project Structure

```
kkbase/
├── cmd/
│   └── watcher/          # Main application entry point
├── pkg/
│   ├── config/           # Configuration management
│   ├── graph/            # Graph database abstraction
│   │   ├── interface.go
│   │   └── neo4j/        # Neo4j implementation
│   ├── models/           # Node and edge models
│   ├── observability/    # Observability interfaces
│   └── watchers/         # Watcher framework
│       └── handlers/     # Resource handlers
├── deploy/               # Kubernetes manifests
├── Dockerfile
└── README.md
```

### Adding a New Resource Type

1. Add converter in `pkg/models/converters.go`
2. Create handler in `pkg/watchers/handlers/`
3. Register handler in `cmd/watcher/main.go`

### Running Locally

```bash
# Set up environment
export NEO4J_URI="bolt://localhost:7687"
export NEO4J_USERNAME="neo4j"
export NEO4J_PASSWORD="changeme"
export KUBECONFIG="$HOME/.kube/config"
export LOG_LEVEL="debug"

# Run
go run ./cmd/watcher
```

## Future Enhancements

- [ ] Health check endpoints
- [ ] Prometheus metrics for watcher operations
- [ ] Metrics integration (Prometheus)
- [ ] Logs integration (Elasticsearch/Loki)
- [ ] Traces integration (Jaeger/Tempo)
- [ ] GraphQL API for querying
- [ ] Web UI for visualization
- [ ] Anomaly detection
- [ ] Automated remediation actions

## Contributing

Contributions are welcome! Please feel free to submit issues or pull requests.

## License

MIT License - see LICENSE file for details.

