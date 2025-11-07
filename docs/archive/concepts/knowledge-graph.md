# Knowledge Graph Model

## Why a Knowledge Graph?

Traditional observability tools provide siloed telemetry data—metrics, logs, traces, and events exist in isolation. When a problem occurs, engineers must manually correlate data across these systems, a time-consuming and error-prone process.

kkbase solves this by transforming disparate data into a unified **Knowledge Graph**—a living, structured representation of your entire Kubernetes cluster. This graph reveals the hidden relationships between resources, enabling autonomous diagnostic agents to perform sophisticated multi-hop reasoning for accurate root cause analysis.

The Knowledge Graph serves as the agent's "world model," providing the essential context needed to understand dependencies, assess impact, and trace problems to their source.

## Core Components

A knowledge graph consists of three fundamental elements:

### Nodes (Entities)

Nodes represent discrete resources and concepts within the cluster:

| Category | Examples |
|----------|----------|
| **Compute** | Cluster, Node |
| **Workloads** | Pod, Container, Deployment, ReplicaSet, StatefulSet, DaemonSet |
| **Networking** | Service, Ingress, Endpoint, NetworkPolicy |
| **Storage** | PersistentVolume, PersistentVolumeClaim, StorageClass |
| **Configuration** | ConfigMap, Secret, Namespace |
| **Observability** | K8sEvent, Metric, LogEntry, Trace |
| **Extensions** | Gateway API resources, Istio resources |

### Edges (Relationships)

Edges define the meaningful connections between nodes:

| Type | Examples | Purpose |
|------|----------|---------|
| **Structural** | `MANAGES`, `CONTAINS`, `SCHEDULED_ON` | Resource hierarchy |
| **Networking** | `SELECTS_PODS`, `ROUTES_TO`, `FORWARDS_TO` | Traffic flow |
| **Storage** | `MOUNTS`, `BOUND_TO`, `PROVISIONED_BY` | Storage dependencies |
| **Configuration** | `USES_CONFIG`, `USES_SECRET` | Configuration links |
| **Security** | `APPLIES_TO`, `PERMITTED_BY` | Policy application |
| **Observability** | `INVOLVES`, `EMITS` | Event correlation |

### Properties

Properties are key-value attributes attached to nodes and edges that store state and metadata:

- **Node properties**: `status: "Running"`, `ip: "10.1.2.3"`, `restarts: 2`
- **Edge properties**: `weight: 90` (traffic percentage), `subset_name: "canary"`

## Data Model Overview

The graph is continuously updated in real-time through:

1. **Data Collection**: Ingest from Kubernetes API server, metrics systems, logs, and events
2. **Knowledge Extraction**: Parse and map data to the graph schema (nodes, edges, properties)
3. **Graph Construction**: Upsert operations maintain current state while preserving history

Resources are identified uniquely:
- **Namespaced**: `Pod/default/nginx-abc123`
- **Cluster-scoped**: `Node/worker-1`

This ensures globally unique identifiers across the entire cluster topology.

## How Agents Use the Graph

The knowledge graph enables sophisticated diagnostic capabilities:

### 1. Contextualization

An alert is no longer an isolated event. When a metric shows high latency:

```cypher
// Start with a metric alert
MATCH (metric:Metric {name: 'http_request_duration_high'})-[:EMITS]-(container:Container)
MATCH (container)<-[:CONTAINS]-(pod:Pod)-[:SCHEDULED_ON]->(node:Node)
MATCH (pod)<-[:SELECTS_PODS]-(service:Service)
RETURN container, pod, node, service
```

The agent instantly sees: the container, its pod, the node it runs on, and all services exposing it.

### 2. Impact Analysis ("Blast Radius")

When a node fails, identify all affected resources:

```cypher
MATCH (node:Node {name: 'failed-node', status: 'NotReady'})
      <-[:SCHEDULED_ON]-(pod:Pod)
OPTIONAL MATCH (pod)<-[:SELECTS_PODS]-(service:Service)
OPTIONAL MATCH (pod)<-[:MANAGES]-()<-[:MANAGES]-(deployment:Deployment)
RETURN node, pod, service, deployment
```

The agent traces the full blast radius: every pod, deployment, and service affected by the outage.

### 3. Root Cause Analysis

For a pod in `CrashLoopBackOff`, the agent queries in one operation:

```cypher
MATCH (pod:Pod {name: 'failing-pod', status: 'CrashLoopBackOff'})
OPTIONAL MATCH (event:K8sEvent)-[:INVOLVES]->(pod)
WHERE event.last_timestamp > timestamp() - 300000
OPTIONAL MATCH (pod)-[:USES_CONFIG]->(cm:ConfigMap)
OPTIONAL MATCH (pod)-[:USES_SECRET]->(secret:Secret)
RETURN pod, collect(event) as recent_events, collect(cm) as configs
```

This reveals whether the issue is an application error (events show errors), resource limits (OOMKilled events), or configuration problems (missing ConfigMap).

## Real-Time Synchronization

The graph is not static—it continuously reflects cluster state:

- **Watchers** monitor Kubernetes API for resource changes
- **Handlers** process Add/Update/Delete events
- **Graph Store** maintains consistency through upsert/delete operations
- **Resync** periodically ensures no drift from cluster state

## Integration with Agents

For autonomous diagnostic agents:

1. **As Memory**: The graph serves as the agent's long-term, structured memory
2. **Query-Based Perception**: Agents perceive the environment by querying the graph
3. **Reasoning Input**: Graph state feeds the agent's reasoning engine (BDI, LLM planner)
4. **Feedback Loop**: Agent actions produce observations that update the graph
5. **GraphRAG**: For LLM agents, the graph provides grounded, factual context to reduce hallucinations

## Extension Support

The graph model extends beyond core Kubernetes:

- **Gateway API**: Models role-oriented ingress (GatewayClass → Gateway → Routes → Services)
- **Istio**: Captures service mesh traffic routing, security policies, and mTLS configuration
- **Custom Resources**: Extensible handler framework for any CRD

See [Extensions Guide](../user-guide/extensions.md) for details.

## Further Reading

- **[Graph Schema](../reference/graph-schema.md)** - Complete node and edge type reference
- **[Architecture](../development/architecture.md)** - Implementation details
- **[Query Examples](../user-guide/querying.md)** - Common diagnostic queries
- **[Cypher Reference](../reference/cypher-queries.md)** - Complete query library

