# Core Concepts: The Knowledge Graph

This document explains the fundamental concepts behind kkbase's knowledge graph model and why it's essential for autonomous Kubernetes diagnostics.

## Why a Knowledge Graph?

Traditional observability tools provide siloed telemetry data—metrics, logs, traces, and events exist in isolation. When a problem occurs, engineers must manually correlate data across these systems, a time-consuming and error-prone process.

kkbase solves this by transforming disparate data into a unified **Knowledge Graph**—a living, structured representation of your entire Kubernetes cluster. This graph reveals the hidden relationships between resources, enabling autonomous diagnostic agents to perform sophisticated multi-hop reasoning for accurate root cause analysis.

### The Problem with Siloed Data

```
Metrics Dashboard  ─┐
                    ├→  Manual Correlation  → Root Cause?
Log Aggregator    ─┤    (slow, error-prone)
                    │
Tracing System    ─┘
```

You have the data, but relationships are implicit. You must manually:
- Correlate metrics with logs
- Match trace IDs across services
- Map events to specific pods
- Understand service dependencies

### The Graph Database Advantage

```
                    ┌─→ Pod → Node (SCHEDULED_ON)
                    │
Service → Deployment ─→ ReplicaSet (MANAGES) → Pod → ConfigMap (USES_CONFIG)
                    │
                    └─→ PVC (MOUNTS) → PV (BOUND_TO)
```

Relationships are **explicit and queryable**. One query traverses the entire dependency chain.

## The Knowledge Graph as World Model

The knowledge graph serves as the definitive, machine-readable **"world model"** for autonomous diagnostic agents operating within the complex, dynamic Kubernetes environment.

### Strategic Value

The knowledge graph overcomes fundamental limitations of analyzing high-volume, siloed telemetry data in isolation by:

1. **Transforming disparate data streams** into unified, structured representation
2. **Revealing hidden patterns** across system components
3. **Making relationships explicit** for multi-hop reasoning
4. **Providing single source of truth** for topology, state, and history

This enables agents to perform sophisticated reasoning essential for accurate root cause analysis and intelligent decision-making.

### Why Graph Database?

Graph databases are optimized for this domain because they:

- **Store relationships as first-class citizens** (not foreign keys)
- **Enable constant-time traversals** (no expensive joins)
- **Support pattern matching** with declarative queries
- **Scale to millions of relationships** efficiently

For Kubernetes, where "everything connects to everything," this is critical.

## Core Components

A knowledge graph consists of three fundamental elements:

### 1. Nodes (Entities)

Nodes represent discrete resources and concepts within the cluster:

**Structural Entities**:
- `Cluster`, `Node`, `Pod`, `Container`
- `Deployment`, `ReplicaSet`, `StatefulSet`, `DaemonSet`
- `Service`, `PersistentVolume`, `Namespace`

**Observability Entities**:
- `Metric`, `LogEntry`, `Trace`, `K8sEvent`

**Configuration & Policy Entities**:
- `ConfigMap`, `Secret`, `NetworkPolicy`
- Service mesh: `VirtualService`, `DestinationRule`, `AuthorizationPolicy`
- Gateway API: `Gateway`, `HTTPRoute`, `GRPCRoute`

**Agent System Entities**:
- `AgentSession`, `Hypothesis`, `Finding`, `Recommendation`, `Investigation`

### 2. Edges (Relationships)

Edges define meaningful connections between nodes:

**Hierarchical Relationships**:
- `Deployment` → `MANAGES` → `ReplicaSet` → `MANAGES` → `Pod` → `CONTAINS` → `Container`

**Placement Relationships**:
- `Pod` → `SCHEDULED_ON` → `Node`

**Networking Relationships**:
- `Service` → `SELECTS_PODS` → `Pod`
- `HTTPRoute` → `FORWARDS_TO` → `Service`
- `VirtualService` → `ROUTES_TO` → `Service`

**Dependency Relationships**:
- `Pod` → `USES_CONFIG` → `ConfigMap`
- `Pod` → `USES_SECRET` → `Secret`
- `Pod` → `MOUNTS` → `PVC` → `BOUND_TO` → `PV`

**Causal/Observability Relationships**:
- `K8sEvent` → `INVOLVES` → `Pod`
- `Span` → `ORIGINATED_FROM` → `Service`
- `Service` → `CALLS` → `Service` (derived from traces)

**Agent Investigation Relationships**:
- `AgentSession` → `HAS_HYPOTHESIS` → `Hypothesis`
- `AgentSession` → `HAS_FINDING` → `Finding`
- `Finding` → `AFFECTS` → `Resource`
- `Recommendation` → `BASED_ON` → `Finding`

### 3. Properties

Properties are key-value attributes that store state and metadata:

**Node Properties**:
- `status: "Running"`, `phase: "Failed"`
- `ip: "10.1.2.3"`, `restarts: 2`
- `cpu_capacity: "4"`, `memory_capacity: "8Gi"`
- `labels: {"app": "nginx", "env": "prod"}`

**Edge Properties**:
- `weight: 90` (traffic percentage in canary routing)
- `subset_name: "canary"` (traffic routing subset)
- `mount_path: "/etc/config"` (volume mount location)

## Resource Identification

All resources use consistent, globally unique identifiers:

**Namespaced Resources**:
```
Type/namespace/name
```
Examples:
- `Pod/default/nginx-abc123`
- `Service/production/order-service`
- `ConfigMap/kube-system/coredns`

**Cluster-Scoped Resources**:
```
Type/name
```
Examples:
- `Node/worker-1`
- `GatewayClass/istio`
- `PersistentVolume/pv-storage-001`

This format ensures clarity and prevents collisions across the entire cluster.

## Data Fusion and Lifecycle

The knowledge graph is a **living model** that continuously updates to reflect Kubernetes' ephemeral nature.

### Continuous Data Fusion Process

1. **Data Collection**: Ingest from multiple sources
   - Kubernetes API server (structured resource data)
   - Prometheus (metrics time-series)
   - Jaeger (distributed traces)
   - Logs and events (semi-structured data)

2. **Knowledge Extraction**: Process raw data
   - Parse and map to graph schema
   - Correlate metrics with resources
   - Extract dependencies from API objects
   - Link traces to services

3. **Graph Construction**: ETL pipeline updates
   - Incremental upsert operations
   - Temporal awareness (track changes over time)
   - Handle resource creations and deletions
   - Maintain referential integrity

### Real-Time Synchronization

```
Kubernetes API Event
       ↓
Watcher receives notification
       ↓
Handler converts to graph operations
       ↓
Neo4j updates nodes and edges
       ↓
Graph reflects current state (< 1 second)
```

The watcher ensures:
- **No polling**: Efficient watch API
- **Local caching**: Reduced API server load
- **Batch updates**: Optimized graph operations
- **Automatic resync**: Prevents drift

## How Agents Use the Graph

The knowledge graph enables four foundational capabilities for autonomous agents:

### 1. Contextualization

Upon receiving an alert, retrieve full context in one query:

```cypher
// Alert: High latency detected
MATCH (metric:Metric {name: 'http_request_duration_high'})
      -[:EMITTED_BY]->(container:Container)
      <-[:CONTAINS]-(pod:Pod)
      -[:SCHEDULED_ON]->(node:Node)
MATCH (pod)<-[:SELECTS_PODS]-(service:Service)
RETURN container, pod, node, service
```

**Result**: The agent instantly knows:
- Which container emitted the metric
- Which pod contains it
- Which node hosts it
- Which services expose it
- All dependencies (ConfigMaps, Secrets, PVCs)

### 2. Impact Analysis (Blast Radius)

Traverse the graph from a failed component to identify all affected resources:

```cypher
// What breaks if this node fails?
MATCH (node:Node {name: 'worker-3', status: 'NotReady'})
      <-[:SCHEDULED_ON]-(pod:Pod)
OPTIONAL MATCH (pod)<-[:SELECTS_PODS]-(service:Service)
OPTIONAL MATCH (pod)<-[:MANAGES]-()-[:MANAGES]-(deployment:Deployment)
RETURN node, 
       collect(DISTINCT pod.name) as affected_pods,
       collect(DISTINCT service.name) as affected_services,
       collect(DISTINCT deployment.name) as affected_deployments
```

**Result**: Full blast radius with counts and names.

### 3. Root Cause Analysis

Execute multi-hop traversals to trace symptoms to causes:

```cypher
// Why is this pod crashing?
MATCH (pod:Pod {name: 'app-xyz', status: 'CrashLoopBackOff'})
OPTIONAL MATCH (event:K8sEvent)-[:INVOLVES]->(pod)
WHERE event.last_timestamp > timestamp() - 300000  // Last 5 minutes
OPTIONAL MATCH (pod)-[:USES_CONFIG]->(cm:ConfigMap)
OPTIONAL MATCH (pod)-[:USES_SECRET]->(secret:Secret)
OPTIONAL MATCH (pod)-[:MOUNTS]->(pvc:PersistentVolumeClaim)
      -[:BOUND_TO]->(pv:PersistentVolume)
RETURN pod,
       collect(event.reason) as event_reasons,
       collect(cm.name) as config_maps,
       collect(pvc.status) as pvc_statuses
```

**Result**: Differentiate between:
- Application errors (events show `Error`, `Failed`)
- Resource limits (events show `OOMKilled`)
- Configuration issues (missing ConfigMap, unbound PVC)
- Storage problems (PV not available)

### 4. Dependency Mapping

Visualize complete service topology:

```cypher
MATCH path = (svc:Service)-[:SELECTS_PODS]->(:Pod)
             <-[:MANAGES*1..2]-(d:Deployment)
RETURN path
```

**Result**: Full service-to-deployment graph for capacity planning or migration analysis.

## Agent Integration Protocol

For autonomous diagnostic agents, the knowledge graph is not just a data store—it's an active component of the agent's cognitive loop:

### As Agent Memory

The graph serves as the agent's **long-term structured memory**:
- Stores beliefs about the world
- Maintains investigation history
- Tracks hypothesis evolution
- Records findings and recommendations

### Query-Based Perception

Agents perceive the environment by executing graph queries:
- Execute Cypher queries to explore topology
- Results update agent's internal "beliefs"
- State representation drives reasoning

### Reasoning Cycle

The graph state feeds the agent's reasoning module:
1. **Observe**: Query graph for current state
2. **Hypothesize**: Generate theories based on state
3. **Investigate**: Execute diagnostic queries
4. **Update**: Refine hypothesis with new data
5. **Recommend**: Generate action items

### Feedback Loop

Agent actions update the graph:
- Tool executions produce observations
- Findings are stored as graph nodes
- Recommendations link to evidence
- Session state tracked for monitoring

### Retrieval-Augmented Generation (GraphRAG)

For LLM-based agents, the graph provides **grounded reasoning**:
- Convert diagnostic questions to graph queries
- Retrieve highly relevant, factual context
- Use context to ground LLM responses
- Reduce hallucinations with real cluster data
- Generate accurate analysis and reports

## Extension Support

The graph model extends beyond core Kubernetes:

### Gateway API

Models role-oriented ingress and routing:
- `GatewayClass` → `Gateway` → `HTTPRoute` → `Service` → `Pod`
- Cross-namespace routing with `ReferenceGrant`
- TLS certificate tracking

### Istio Service Mesh

Captures traffic management and security:
- `VirtualService` → `DestinationRule` (traffic splitting)
- `AuthorizationPolicy` → `Pod` (access control)
- `PeerAuthentication` (mTLS configuration)

### Custom Resources

Extensible handler framework for any CRD:
- Define node type
- Create handler
- Register with watcher
- Automatically tracked in graph

See [Custom Handlers Guide](../services/watcher/custom-handlers.md)

## Performance Characteristics

### Query Performance

- **Typical traversals**: < 50ms
- **Complex multi-hop**: < 200ms
- **Indexed lookups**: < 10ms

### Scalability

- **Nodes**: 100K+ resources efficiently
- **Edges**: Millions of relationships
- **Memory**: ~1KB per resource
- **Storage**: Linear with cluster size

### Real-Time Updates

- **Sync latency**: < 1 second from K8s event
- **Batch operations**: 1000s of resources/second
- **API load**: Minimal (watch, not poll)

## Best Practices

### For Query Design

1. **Use indexes**: Query by `id` field (automatically indexed)
2. **Limit depth**: Specify `MATCH` depth for traversals (`-[:MANAGES*1..3]->`)
3. **Use OPTIONAL**: Handle missing relationships gracefully
4. **Add LIMIT**: Prevent overwhelming result sets
5. **Profile queries**: Use `EXPLAIN` and `PROFILE` in Neo4j

### For Schema Design

1. **Meaningful relationships**: Edge types should be semantic
2. **Directional consistency**: Follow established patterns
3. **Property normalization**: Store computed values when beneficial
4. **Temporal markers**: Include timestamps for history

## Next Steps

Now that you understand the knowledge graph model:

**Deploy and explore**:
- [Minimal Quick Start](quickstart-minimal.md) - Get the graph running
- [Query Basics](../guides/querying/basics.md) - Learn Cypher

**Understand the system**:
- [System Architecture](../ARCHITECTURE.md) - How services work together
- [Graph Schema](../reference/graph-schema.md) - Complete node/edge reference

**Build with kkbase**:
- [Investigation Workflow](../guides/investigations/workflow.md) - AI agent patterns
- [Custom Handlers](../services/watcher/custom-handlers.md) - Extend the graph

