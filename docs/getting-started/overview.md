# kkbase Overview

## What is kkbase?

kkbase (Kubernetes Knowledge Base) is a multi-service platform that transforms your Kubernetes cluster into a queryable knowledge graph, enabling autonomous diagnostics and intelligent troubleshooting.

It continuously watches your cluster, models all resources and their relationships in Neo4j, and exposes this knowledge to AI agents through a standardized protocol (MCP - Model Context Protocol).

## The Problem

Modern Kubernetes environments generate massive amounts of telemetry data: metrics, logs, traces, and events. However, this data exists in silos, making it difficult to:

- **Understand relationships** between failing components
- **Trace dependencies** through complex service meshes
- **Perform root cause analysis** across system boundaries
- **Automate diagnostics** without manual investigation

Traditional monitoring systems show you metrics and logs, but they don't understand how your pods, services, deployments, and nodes relate to each other. When something breaks, you're left correlating data manually.

## The Solution

kkbase solves this by creating a **living knowledge graph** of your entire cluster. Every resource becomes a node, every relationship becomes an edge, and the whole topology is queryable in real-time.

This enables:

1. **Multi-hop reasoning** - Trace issues through dependency chains
2. **Impact analysis** - Instantly understand blast radius
3. **Autonomous investigation** - AI agents can explore and diagnose
4. **Context-aware troubleshooting** - Full cluster state in every query

## The Three Services

kkbase consists of three complementary services that can be deployed together or separately:

### 1. Watcher Service

**What it does**: Continuously syncs your Kubernetes cluster to Neo4j as a graph.

**How it works**:
- Watches Kubernetes API using SharedInformers
- Converts resources to graph nodes (Pod, Service, Deployment, etc.)
- Creates edges for relationships (manages, selects, routes-to, etc.)
- Updates Neo4j in real-time as resources change

**When to use**:
- You want a queryable graph of your cluster
- Need dependency mapping and impact analysis
- Want to run custom Cypher queries
- Building custom tools that need topology data

**Deploy alone for**: Manual analysis, custom integrations, learning the graph model

### 2. MCP Server

**What it does**: Exposes the knowledge graph to AI agents via Model Context Protocol.

**How it works**:
- Implements MCP over HTTP/SSE
- Provides tools: query, structure, agent sessions, investigations
- Tracks diagnostic sessions with hypothesis evolution
- Serves embedded web dashboard for real-time monitoring

**When to use**:
- You want AI agents to investigate issues
- Need structured access for autonomous systems
- Want session tracking and blast zone analysis
- Need to monitor investigations in real-time

**Deploy with watcher for**: AI-assisted troubleshooting with Cursor or Claude

### 3. Agent Service

**What it does**: Autonomous diagnostic system that responds to alerts and investigates issues.

**How it works**:
- Responds to webhooks (from Prometheus, PagerDuty, etc.)
- Pulls issues from external systems
- Executes investigation workflows using MCP tools
- Generates findings, recommendations, and reports

**When to use**:
- You want fully autonomous diagnostics
- Need to respond to alerts automatically
- Want integration with incident management
- Building self-healing systems

**Deploy all three for**: Production-grade autonomous troubleshooting

## Why a Knowledge Graph?

### Traditional Approach: Siloed Data

```
Metrics Dashboard  ─┐
                    ├→  Manual Correlation  → Root Cause?
Log Aggregator    ─┤
                    │
Tracing System    ─┘
```

You have the data, but you must manually connect the dots.

### Knowledge Graph Approach: Connected Understanding

```
                    ┌─→ Pod → Node
                    │
Service → Deployment ─→ ReplicaSet → Pod → ConfigMap
                    │
                    └─→ PVC → PV
```

The relationships are explicit and queryable.

### Why This Matters for AI Agents

AI agents need structured world models to reason effectively. A knowledge graph provides:

1. **Explicit relationships** - No need to infer connections
2. **Multi-hop traversal** - Follow dependency chains automatically
3. **Contextual queries** - Get related resources in one query
4. **Temporal awareness** - Track changes over time
5. **Grounded reasoning** - Factual data prevents hallucinations

## The Knowledge Graph as World Model

kkbase implements the knowledge graph as the **world model** for autonomous diagnostic agents. This architectural pattern is essential for effective AI-driven troubleshooting.

### Core Purpose

The knowledge graph overcomes the fundamental limitation of analyzing high-volume, siloed telemetry data in isolation. It transforms disparate data streams into a unified, structured representation that reveals:

- Hidden patterns
- Explicit relationships between components
- System topology, state, and history

This enables agents to perform sophisticated **multi-hop reasoning** essential for accurate root cause analysis.

### As Agent Memory

The knowledge graph serves as the agent's **long-term structured memory**:

- **Query-based perception**: Agents perceive the environment by executing graph queries
- **State representation**: Query results update the agent's internal beliefs
- **Reasoning cycle**: Graph state drives hypothesis generation and diagnostic planning
- **Iterative refinement**: New observations update the graph, refining hypotheses

### Data Fusion and Lifecycle

The knowledge graph is a **living model** that continuously updates to reflect the ephemeral nature of Kubernetes:

1. **Data Collection**: Ingest from Kubernetes API, Prometheus, Jaeger
2. **Knowledge Extraction**: Parse and correlate data into entities and relationships
3. **Graph Construction**: Incremental ETL pipeline updates Neo4j in real-time
4. **Temporal Awareness**: Track resource changes, creations, and deletions over time

### Retrieval-Augmented Generation (GraphRAG)

For LLM-based agents, the knowledge graph is the ideal retrieval source:

- Convert diagnostic questions into graph queries
- Retrieve highly relevant, factual context
- Ground reasoning in actual cluster state
- Reduce hallucinations and improve accuracy

## Key Capabilities

### 1. Contextualization

When an agent receives an alert (e.g., high latency), it can immediately query the graph to retrieve full context:

- Which container emitted the metric
- Which pod contains the container
- Which node hosts the pod
- Which service routes to the pod
- All upstream and downstream dependencies

### 2. Impact Analysis

By traversing the graph from a failed component, the agent instantly identifies the **blast radius**:

```cypher
// What breaks if this node fails?
MATCH (n:Node {name: 'worker-3'})<-[:SCHEDULED_ON]-(p:Pod)
OPTIONAL MATCH (p)<-[:SELECTS_PODS]-(s:Service)
OPTIONAL MATCH (p)<-[:MANAGES]-()<-[:MANAGES]-(d:Deployment)
RETURN n, p, s, d
```

Result: All affected pods, services, and deployments.

### 3. Root Cause Analysis

Execute multi-hop traversals to trace symptoms back to causes:

```cypher
// Why is this pod crashing?
MATCH (p:Pod {name: 'app-xyz', status: 'CrashLoopBackOff'})
OPTIONAL MATCH (p)-[:HAS_EVENT]->(e:K8sEvent)
OPTIONAL MATCH (p)-[:USES_CONFIG]->(cm:ConfigMap)
OPTIONAL MATCH (p)-[:MOUNTS]->(pvc:PersistentVolumeClaim)-[:BOUND_TO]->(pv:PersistentVolume)
RETURN p, e, cm, pvc, pv
```

Differentiate between application errors, OOM kills, or configuration issues.

### 4. Dependency Mapping

Visualize and query the complete service dependency graph:

```cypher
MATCH path = (svc:Service)-[:SELECTS_PODS]->(:Pod)<-[:MANAGES*1..2]-(d:Deployment)
RETURN path
```

Understand how services connect through the pod hierarchy.

## Real-World Use Cases

### For SRE Teams

**Autonomous On-Call Response**:
- Alert fires in Prometheus
- Webhook triggers kkbase agent
- Agent investigates using knowledge graph
- Generates findings and recommended actions
- Posts summary to incident channel

### For Platform Engineers

**Impact Analysis Before Changes**:
- Query graph to see what depends on a service
- Understand blast radius of planned changes
- Identify affected teams and systems
- Plan migration safely

### For Developers

**AI-Assisted Debugging in Cursor**:
- Pod is crashing in staging
- Ask Cursor: "Why is my pod crashing?"
- Cursor uses kkbase MCP tools
- Traverses graph to find root cause
- Explains issue with context

### For Security Teams

**Policy Compliance Auditing**:
- Query all services without NetworkPolicies
- Find pods with privileged access
- Identify exposed services
- Audit Istio authorization policies

## Deployment Flexibility

kkbase adapts to your needs with three deployment patterns:

### Pattern 1: Graph Only
Deploy just the **Watcher** for manual graph exploration.

**Use case**: Learning, custom integrations, manual analysis

### Pattern 2: AI-Assisted
Deploy **Watcher + MCP Server** for AI tool integration.

**Use case**: Development teams, on-demand troubleshooting

### Pattern 3: Fully Autonomous
Deploy all three services with webhook integration.

**Use case**: Production SRE teams, autonomous diagnostics

## Technology Stack

- **Go 1.21+**: All backend services
- **Neo4j 4.0+**: Graph database
- **Model Context Protocol**: AI agent communication
- **Kubernetes client-go**: Cluster watching
- **React + TypeScript**: Web dashboard

## What Makes kkbase Different?

### Compared to Traditional Monitoring

| Traditional | kkbase |
|------------|--------|
| Metrics dashboards | Relationship-aware queries |
| Manual correlation | Automatic dependency tracking |
| Siloed tools | Unified knowledge graph |
| Reactive alerts | Proactive investigation |
| Human-driven | AI-enabled |

### Compared to Other K8s Tools

- **kubectl**: Imperative commands vs. declarative queries
- **K9s/Lens**: UI-focused vs. API-driven for agents
- **Prometheus**: Time-series metrics vs. topology + relationships
- **Grafana**: Visualization vs. autonomous reasoning

kkbase complements these tools by providing the **relationship layer** they lack.

## Next Steps

Ready to try kkbase? Choose your path:

**Just want to explore the graph?**
→ [Minimal Quick Start](quickstart-minimal.md) - Deploy watcher + Neo4j in 10 minutes

**Want AI-assisted investigation?**
→ [Full Stack Quick Start](quickstart-with-agent.md) - Complete setup in 20 minutes

**Want to understand deeply first?**
→ [Core Concepts](concepts.md) - Learn the knowledge graph model

**Planning production deployment?**
→ [System Architecture](../ARCHITECTURE.md) - Understand the components

## Learn More

- [System Architecture](../ARCHITECTURE.md) - Deep dive into components
- [Core Concepts](concepts.md) - Knowledge graph fundamentals
- [Watcher Service](../services/watcher/) - Cluster synchronization
- [MCP Server](../services/mcp-server/) - AI agent interface
- [Agent Service](../services/agent/) - Autonomous diagnostics

