# kkbase Documentation

**Kubernetes Knowledge Graph** for autonomous diagnostics and AI-powered troubleshooting.

## What is kkbase?

kkbase syncs your Kubernetes cluster state into a Neo4j knowledge graph, enabling AI agents to investigate issues autonomously using graph queries and metrics analysis.

## Quick Navigation

### 🚀 New Users

**Start here** to get kkbase running in 5 minutes:

1. **[Overview](getting-started/overview.md)** - What kkbase does and why
2. **[Quick Start](getting-started/quickstart.md)** - 5-minute Kubernetes deployment
3. **[Core Concepts](getting-started/concepts.md)** - Understanding the knowledge graph

### 👥 By User Type

**Cluster Operators** - Deploy and manage kkbase:
- [Watcher Service](services/watcher/) - Sync cluster to graph
- [MCP Server](services/mcp-server/) - Query API and dashboard
- [Operations Guide](guides/operations/) - Monitoring and troubleshooting

**AI Agent Developers** - Build autonomous diagnostics:
- [Investigation Workflow](guides/investigations/workflow.md) - How agents investigate
- [MCP Tools API](reference/mcp-tools-api.md) - Complete tool reference
- [Best Practices](guides/investigations/best-practices.md) - Patterns and tips

**Graph Query Users** - Analyze cluster dependencies:
- [Query Basics](guides/querying/basics.md) - Introduction to Cypher
- [Relationships](guides/querying/relationships.md) - Traversing dependencies
- [RCA Patterns](guides/querying/rca-patterns.md) - Root cause analysis queries

**Platform Developers** - Extend kkbase:
- [Development Guide](development/) - Architecture and internals
- [Building](development/building.md) - Build and test
- [Extending](development/extending.md) - Add handlers and tools

## Documentation Structure

### 📖 [Getting Started](getting-started/)

Essential guides for new users:
- [Overview](getting-started/overview.md) - Introduction and use cases
- [Quick Start](getting-started/quickstart.md) - Deploy in 5 minutes
- [Core Concepts](getting-started/concepts.md) - Knowledge graph fundamentals
- [Local Development](getting-started/local-development.md) - Run locally

### ⚙️ [Services](services/)

Service-specific documentation:

**[Watcher](services/watcher/)** - Kubernetes → Neo4j sync
- Deployment, configuration, extensions, custom handlers

**[MCP Server](services/mcp-server/)** - Query API and dashboard
- Deployment, configuration, tools reference, dashboard

**[Agent](services/agent/)** - Autonomous diagnostics
- Deployment, configuration, webhook integration

### 📚 [Guides](guides/)

Task-oriented how-to guides:

**[Querying](guides/querying/)** - Query the knowledge graph
- Basics, relationships, advanced patterns, RCA queries

**[Investigations](guides/investigations/)** - AI agent investigations
- Workflow, agent sessions, metrics RCA, best practices

**[Operations](guides/operations/)** - Day-to-day operations
- Monitoring, troubleshooting, scaling

### 📘 [Reference](reference/)

Technical reference material:
- [Configuration](reference/configuration.md) - All environment variables
- [Graph Schema](reference/graph-schema.md) - Node and edge types
- [MCP Tools API](reference/mcp-tools-api.md) - Complete tool specifications
- [Cypher Queries](reference/cypher-queries.md) - Query examples

### 🔧 [Development](development/)

For contributors and developers:
- [Development Guide](development/README.md) - Getting started
- [Deep Dive](development/deep-dive.md) - Architecture internals
- [Building](development/building.md) - Build and test
- [Extending](development/extending.md) - Add functionality

### 🏛️ [System Architecture](ARCHITECTURE.md)

High-level system architecture and service overview.

### 📦 [Archive](archive/)

Historical documentation from previous versions.

## Common Tasks

### Deploy kkbase

```bash
# Deploy Neo4j
kubectl apply -f https://raw.githubusercontent.com/kagenti/kkbase/main/deploy/neo4j.yaml

# Deploy watcher
kubectl apply -f https://raw.githubusercontent.com/kagenti/kkbase/main/deploy/watcher.yaml

# Deploy MCP server
kubectl apply -f https://raw.githubusercontent.com/kagenti/kkbase/main/deploy/mcp-server.yaml
```

See: [Quick Start Guide](getting-started/quickstart.md)

### Query the Graph

```cypher
// Find all pods in namespace
MATCH (p:Pod {namespace: 'production'})
RETURN p.name, p.status

// Trace service dependencies
MATCH (s:Service {name: 'orders-api'})-[:SELECTS_PODS]->(p:Pod)
RETURN s, p
```

See: [Query Basics](guides/querying/basics.md)

### Start an Investigation

In Cursor or Claude:

```
Start a kkbase investigation for:
"Service orders-api returning 503 errors"

Find the service and check its backend pods.
```

See: [Investigation Workflow](guides/investigations/workflow.md)

### Configure Extensions

```yaml
# Enable Istio support
HANDLERS_ENABLED: "pod,service,deployment,virtualservice,destinationrule"
```

See: [Watcher Extensions](services/watcher/extensions.md)

### Monitor Health

```bash
# Check all services
kubectl get pods -l app.kubernetes.io/part-of=kkbase

# View logs
kubectl logs -f deployment/kkbase-watcher

# Access dashboard
kubectl port-forward svc/kkbase-mcp-server 8080:8080
open http://localhost:8080/
```

See: [Operations Guide](guides/operations/)

## Architecture Overview

```
┌─────────────────────────────────────────────────┐
│         Kubernetes Cluster                      │
│  ┌──────────┐  ┌─────────┐  ┌──────────────┐    │
│  │  Pods    │  │Services │  │ Deployments  │    │
│  └──────────┘  └─────────┘  └──────────────┘    │
└─────────────┬───────────────────────────────────┘
              │
              │ Watch Events
              ↓
      ┌──────────────┐
      │   Watcher    │ Syncs resources to graph
      └──────┬───────┘
             │
             ↓
      ┌──────────────┐
      │    Neo4j     │ Stores knowledge graph
      └──────┬───────┘
             │
             │ Cypher Queries
             ↓
      ┌──────────────┐
      │  MCP Server  │ Exposes tools + dashboard
      └──────┬───────┘
             │
             ├─→ Web Dashboard (humans)
             │
             └─→ AI Agents (autonomous diagnostics)
```

See: [Architecture Document](ARCHITECTURE.md)

## Key Features

### For Cluster Operators

- **Automatic sync** - Watch all Kubernetes resources
- **Extensions** - Istio and Gateway API support
- **Real-time updates** - Graph stays in sync
- **Query API** - Powerful Cypher queries
- **Web dashboard** - Monitor investigations
- **Kubernetes-native** - Deploy with kubectl

### For AI Agents

- **MCP protocol** - Standard AI tool interface
- **Investigation sessions** - Track diagnostic flow
- **Automatic findings** - Extract issues from queries
- **Blast zone tracking** - Visualize impact radius
- **Metrics integration** - Prometheus RCA
- **Recommendations** - Actionable next steps

### For Developers

- **Extensible** - Add custom handlers
- **Well-documented** - Comprehensive guides
- **Open source** - Contribute improvements
- **Go codebase** - Standard Kubernetes tools
- **Test coverage** - Unit and integration tests

## Support and Community

- **Issues**: [GitHub Issues](https://github.com/kagenti/kkbase/issues)
- **Discussions**: [GitHub Discussions](https://github.com/kagenti/kkbase/discussions)
- **Contributing**: [Contributing Guide](../CONTRIBUTING.md)

## License

[Apache 2.0](../LICENSE)

## Quick Links

- [System Architecture](ARCHITECTURE.md)
- [Getting Started](getting-started/)
- [Watcher Service](services/watcher/)
- [MCP Server](services/mcp-server/)
- [Agent Service](services/agent/)
- [Query Guides](guides/querying/)
- [Investigation Guides](guides/investigations/)
- [Operations](guides/operations/)
- [Reference](reference/)
- [Development](development/)
