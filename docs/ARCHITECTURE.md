# kkbase System Architecture

This document provides a high-level overview of the kkbase system architecture, its three core services, and how they work together to enable autonomous Kubernetes diagnostics.

## System Overview

kkbase is a multi-service platform that transforms your Kubernetes cluster into a queryable knowledge graph, enabling AI agents to autonomously diagnose cluster issues.

```
┌─────────────────────────────────────────────────────────────────────┐
│                        Kubernetes Cluster                           │
│  ┌──────┐  ┌─────────┐  ┌─────────┐  ┌───────────┐  ┌──────────┐    │
│  │ Pods │  │ Services│  │  Nodes  │  │Deployments│  │  Events  │    │
│  └──────┘  └─────────┘  └─────────┘  └───────────┘  └──────────┘    │
└─────────────────────┬───────────────────────────────────────────────┘
                      │
                      │ Kubernetes API (Watch)
                      ↓
           ┌──────────────────────┐
           │   Watcher Service    │──────────┐
           │                      │          │
           │  • Watches K8s API   │          │
           │  • Converts to graph │          │
           │  • Syncs resources   │          │
           │  • Handles CRDs      │          │
           └──────────┬───────────┘          │
                      │                      │
                      │ Bolt Protocol        │ Optional:
                      ↓                      │ Prometheus/Jaeger
           ┌──────────────────────┐          │
           │      Neo4j           │          │
           │  Knowledge Graph     │←─────────┘
           │                      │
           │  • Nodes (resources) │
           │  • Edges (relations) │
           │  • Properties        │
           └──────────┬───────────┘
                      │
                      │ Cypher Queries
                      ↓
           ┌──────────────────────┐
           │   MCP Server         │
           │                      │
           │  • Model Context     │
           │    Protocol API      │
           │  • Query tools       │
           │  • Agent sessions    │
           │  • Web dashboard     │
           └──────────┬───────────┘
                      │
                      │ HTTP/SSE
                      ↓
           ┌──────────────────────┐           ┌─────────────────┐
           │   AI Agents          │           │  Web Dashboard  │
           │                      │           │                 │
           │  • Cursor/Claude     │           │  • Real-time    │
           │  • Custom agents     │           │  • Visualize    │
           │  • Investigation     │           │  • Monitor      │
           │  • Autonomous RCA    │           │                 │
           └──────────────────────┘           └─────────────────┘
```

## The Three Services

### 1. Watcher Service

**Purpose**: Continuously syncs Kubernetes cluster state to Neo4j as a knowledge graph.

**Key Responsibilities**:
- Watch Kubernetes API for resource changes
- Convert K8s resources to graph nodes and edges
- Maintain real-time synchronization
- Support core resources and extensions (Gateway API, Istio)

**When to Use**:
- You want a queryable graph of your cluster
- Need dependency mapping and impact analysis
- Want to run custom Cypher queries
- Building custom tools that need cluster topology

**Deployment**: Runs as a Kubernetes Deployment with RBAC permissions.

**Documentation**: [Watcher Service Docs](services/watcher/)

### 2. MCP Server

**Purpose**: Exposes the knowledge graph to AI agents via Model Context Protocol.

**Key Responsibilities**:
- Implement MCP (Model Context Protocol) over HTTP/SSE
- Provide query, structure, and investigation tools
- Track agent diagnostic sessions
- Serve embedded web dashboard

**When to Use**:
- You want AI agents to investigate cluster issues
- Need structured access to the knowledge graph
- Want session tracking and blast zone analysis
- Need real-time investigation monitoring

**Deployment**: Can run standalone or integrated with watcher.

**Documentation**: [MCP Server Docs](services/mcp-server/)

### 3. Agent Service

**Purpose**: Autonomous diagnostic system that investigates cluster issues.

**Key Responsibilities**:
- Respond to webhooks (alerts, incidents)
- Pull issues from external systems
- Execute investigation workflows
- Generate findings and recommendations

**When to Use**:
- You want fully autonomous diagnostics
- Need to respond to alerts automatically
- Want to integrate with incident management
- Building self-healing systems

**Deployment**: Runs as a Kubernetes Deployment, connects to MCP server.

**Documentation**: [Agent Service Docs](services/agent/)

## Data Flow

### Knowledge Graph Construction

```
1. K8s API Event → Watcher receives change notification
2. Watcher → Converts resource to graph node + edges
3. Watcher → Upserts to Neo4j via Bolt protocol
4. Neo4j → Updates graph in real-time
```

### AI Agent Investigation

```
1. Agent → Calls MCP server tools via HTTP
2. MCP Server → Executes Cypher queries against Neo4j
3. Neo4j → Returns graph data
4. MCP Server → Processes results, extracts findings
5. MCP Server → Updates agent session state
6. Agent → Receives results, continues investigation
7. Dashboard → Polls for updates, displays in real-time
```

## Deployment Patterns

### Pattern 1: Graph-Only (Watcher + Neo4j)

**Use Case**: Just want a queryable cluster graph.

```
Kubernetes → Watcher → Neo4j
```

**Components**:
- Watcher deployment
- Neo4j database

**Access**: Direct Cypher queries via Neo4j browser or drivers.

**Best For**: Manual analysis, custom integrations, learning the graph model.

### Pattern 2: AI-Assisted (Watcher + MCP Server + Neo4j)

**Use Case**: AI agents (Cursor, Claude) investigate on-demand.

```
Kubernetes → Watcher → Neo4j ← MCP Server ← AI Tools
```

**Components**:
- Watcher deployment
- Neo4j database
- MCP Server (standalone or integrated)
- AI tool integration (Cursor, Claude Desktop)

**Access**: AI agents use MCP tools, humans use dashboard.

**Best For**: Development teams using AI-assisted troubleshooting.

### Pattern 3: Fully Autonomous (All Services)

**Use Case**: Autonomous diagnostics with alert integration.

```
Kubernetes → Watcher → Neo4j ← MCP Server ← Agent Service
                                      ↑
                                   Webhooks/Alerts
```

**Components**:
- Watcher deployment
- Neo4j database
- MCP Server
- Agent service with webhook/puller configuration

**Access**: Agents work autonomously, humans monitor via dashboard.

**Best For**: Production environments, on-call automation, SRE teams.

## Technology Stack

### Core Technologies

- **Go 1.21+**: All services written in Go
- **Neo4j 4.0+**: Graph database for knowledge storage
- **Kubernetes client-go**: K8s API interaction
- **TypeScript/React**: Web dashboard frontend
- **Vite**: Frontend build system

### Communication Protocols

- **Bolt**: Watcher ↔ Neo4j
- **HTTP/SSE**: MCP Server ↔ AI Agents
- **JSON-RPC 2.0**: MCP tool calls
- **Kubernetes API**: Watcher ↔ K8s

### Optional Integrations

- **Prometheus**: Metrics for RCA investigations
- **Jaeger**: Distributed tracing correlation
- **Istio**: Service mesh visibility
- **Gateway API**: Modern ingress tracking

## Knowledge Graph Model

### Node Types

**Core Kubernetes**:
- Compute: `Node`, `Cluster`
- Workloads: `Pod`, `Container`, `Deployment`, `ReplicaSet`, `StatefulSet`, `DaemonSet`, `Job`
- Networking: `Service`, `Ingress`, `Endpoint`, `NetworkPolicy`
- Storage: `PersistentVolume`, `PersistentVolumeClaim`, `StorageClass`
- Configuration: `ConfigMap`, `Secret`, `Namespace`
- Observability: `K8sEvent`

**Extensions**:
- Gateway API: `GatewayClass`, `Gateway`, `HTTPRoute`, `GRPCRoute`, `TCPRoute`, `UDPRoute`, `TLSRoute`
- Istio: `IstioGateway`, `VirtualService`, `DestinationRule`, `ServiceEntry`, `AuthorizationPolicy`

**Agent System**:
- `AgentSession`: Investigation tracking
- `Hypothesis`: Current diagnostic theory
- `Finding`: Discovered issues
- `Recommendation`: Action items
- `Investigation`: Metrics analysis

### Edge Types

**Resource Relationships**:
- `SCHEDULED_ON`: Pod → Node
- `MANAGES`: Deployment → ReplicaSet → Pod
- `CONTAINS`: Pod → Container
- `SELECTS_PODS`: Service → Pod
- `ROUTES_TO`: Ingress/HTTPRoute → Service
- `BOUND_TO`: PVC → PV
- `MOUNTS`: Pod → PVC
- `USES_CONFIG`: Pod → ConfigMap
- `USES_SECRET`: Pod → Secret
- `IN_NAMESPACE`: Resource → Namespace

**Agent Relationships**:
- `HAS_HYPOTHESIS`: AgentSession → Hypothesis
- `HAS_FINDING`: AgentSession → Finding
- `HAS_RECOMMENDATION`: AgentSession → Recommendation
- `AFFECTS`: Finding → Resource
- `BASED_ON`: Recommendation → Finding

### Resource Identification

All resources use consistent ID format:
- Namespaced: `Type/namespace/name` (e.g., `Pod/default/nginx-abc123`)
- Cluster-scoped: `Type/name` (e.g., `Node/worker-1`)

## Scalability and Performance

### Watcher Service

- **Efficiency**: Shared informers, local caching, batch operations
- **Scalability**: Single instance handles 1000s of resources
- **Memory**: ~50-100MB baseline
- **API Load**: Minimal (watch, not poll)

### MCP Server

- **Concurrency**: Handles multiple agent sessions simultaneously
- **Memory**: ~50MB + session state
- **Latency**: <100ms for typical queries
- **Sessions**: Supports dozens of concurrent investigations

### Neo4j

- **Performance**: Indexed on resource IDs
- **Scalability**: Handles 100K+ nodes efficiently
- **Queries**: Sub-second for typical traversals
- **Storage**: ~1KB per resource

### Agent Service

- **Concurrency**: Parallel webhook handling
- **Throughput**: 10s of investigations per minute
- **Memory**: ~30MB + LLM client overhead

## Security Considerations

### Watcher Service

- **RBAC**: Read-only cluster access
- **Secrets**: Neo4j credentials in K8s Secrets
- **TLS**: Supports encrypted Neo4j connections
- **Privilege**: Runs as non-root user

### MCP Server

- **Authentication**: Currently none (add OAuth/API keys for production)
- **Authorization**: Read-only graph queries enforced
- **TLS**: Recommended for production
- **Rate Limiting**: Should be added for production

### Agent Service

- **Webhook Authentication**: Signature verification
- **Secret Management**: LLM API keys in Secrets
- **Network Policies**: Restrict egress/ingress
- **Audit Logging**: All actions logged

## Observability

### Health Checks

- Watcher: `/healthz` (liveness), `/ready` (readiness)
- MCP Server: `/health`
- All services: Structured logging with zap

### Metrics (Planned)

- Events processed per resource type
- Graph operations (nodes/edges created)
- Query latency
- Agent session duration
- Investigation success rate

### Logging

- Structured JSON logs
- Correlation IDs for tracing
- Error stack traces
- Performance metrics

## Development and Extension

### Adding Custom Handlers

Extend watcher to track custom CRDs:
- Core handlers: Standard K8s resources
- Extension handlers: Optional resources (Istio, Gateway API, custom CRDs)
- Plugin architecture with factory pattern

See: [Custom Handlers Guide](services/watcher/custom-handlers.md)

### Adding MCP Tools

Add new diagnostic capabilities:
- Define tool schema
- Implement handler function
- Register with MCP server
- Add tests

See: [Extending MCP Server](development/extending.md)

### Contributing

See: [Development Guide](development/) for architecture deep-dive, building, and testing.

## Next Steps

### For Cluster Operators
- [Quick Start: Watcher Only](getting-started/quickstart-minimal.md)
- [Watcher Deployment Guide](services/watcher/deployment.md)

### For AI Agent Developers
- [Quick Start: Full Stack](getting-started/quickstart-with-agent.md)
- [Investigation Workflow](guides/investigations/workflow.md)
- [MCP Tools Reference](services/mcp-server/tools-reference.md)

### For Platform Developers
- [Architecture Deep Dive](development/deep-dive.md)
- [Local Development](getting-started/local-development.md)
- [Custom Handlers](services/watcher/custom-handlers.md)

## Additional Resources

- [Concepts: Knowledge Graph](getting-started/concepts.md)
- [Query Guide](guides/querying/)
- [Operations Guide](guides/operations/)
- [Reference Documentation](reference/)

