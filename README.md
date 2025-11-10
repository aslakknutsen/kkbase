# Kubernetes Knowledge Base (kkbase)

A Kubernetes operator that builds a real-time knowledge graph of your cluster in Neo4j, enabling powerful diagnostics and autonomous troubleshooting.

## What It Does

kkbase watches your Kubernetes cluster and syncs all resources and their relationships to a Neo4j graph database. This creates a queryable, interconnected model where you can trace dependencies, analyze impact, and perform root cause analysis using graph queries.

## Quick Start

```bash
# Install Neo4j
helm install neo4j neo4j/neo4j --set neo4j.password=changeme

# Deploy kkbase
kubectl apply -f deploy/rbac.yaml
kubectl apply -f deploy/configmap.yaml
kubectl apply -f deploy/secret.yaml
kubectl apply -f deploy/deployment.yaml

# Query your cluster
kubectl port-forward svc/neo4j 7474:7474
# Open http://localhost:7474 and run:
# MATCH (pod:Pod)-[:SCHEDULED_ON]->(node:Node) RETURN pod, node LIMIT 10
```

See [Quick Start Guide](docs/getting-started/quickstart-minimal.md) for detailed instructions.

## Key Features

- **Real-time Synchronization** - Continuously watches Kubernetes API and updates the graph
- **Complete Topology** - Models all core Kubernetes resources and their relationships
- **Gateway API Support** - Tracks ingress resources, routes, and traffic flow
- **Istio Support** - Models service mesh configuration, security policies, and canary deployments
- **Kuadrant Support** - Tracks AuthPolicy, RateLimitPolicy, DNSPolicy, and TLSPolicy for enhanced API management
- **Extensible** - Plugin architecture for custom resources and CRDs
- **Agent-Ready** - Designed as the knowledge base for autonomous diagnostic systems
- **MCP Server** - Streaming HTTP server implementing Model Context Protocol for AI agent integration

## Use Cases

- **Autonomous Diagnostic Agents** - Provide AI agents with structured cluster knowledge
- **Root Cause Analysis** - Trace failures through dependency chains
- **Impact Analysis** - Understand blast radius of changes or outages ("what breaks if this node fails?")
- **Dependency Mapping** - Visualize service dependencies and data flow
- **Security Auditing** - Query policies, permissions, and configuration
- **Capacity Planning** - Analyze resource utilization patterns

## MCP Server for AI Agents

kkbase includes a streaming MCP (Model Context Protocol) server that exposes the knowledge graph to AI agents via HTTP. This enables LLMs and autonomous agents to directly query and explore your cluster topology.

```bash
# Start the MCP server
export NEO4J_URI="bolt://localhost:7687"
export NEO4J_PASSWORD="changeme"
./mcp-server

# Access at http://localhost:8080/mcp
```

**Available Tools**:
- `query` - Execute read-only Cypher queries
- `structure` - Get complete graph schema

See [MCP Server Guide](docs/services/mcp-server/README.md) for integration with Claude Desktop, MCP Inspector, and other AI tools.

## Documentation

- **[Getting Started](docs/getting-started/)** - Installation, configuration, and querying
- **[Concepts](docs/getting-started/concepts.md)** - Understanding the knowledge graph model
- **[Reference](docs/reference/)** - Complete query library, schema, and configuration
- **[Development](docs/development/)** - Architecture and extending kkbase

## Example Query

Find all services with no healthy backend pods:

```cypher
MATCH (s:Service)-[:SELECTS_PODS]->(p:Pod)
WHERE p.status <> 'Running'
RETURN s.namespace, s.name, count(p) as unhealthy_pods
```

See [Query Guide](docs/guides/querying/basics.md) for more examples.

## Requirements

- Kubernetes v1.19+
- Neo4j v4.0+

## License

MIT License
