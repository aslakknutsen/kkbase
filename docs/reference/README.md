# Reference Documentation

Technical reference material for kkbase.

## Contents

### Configuration

- **[Configuration Reference](configuration.md)** - All environment variables and settings
  - Neo4j connection
  - Service configuration
  - Logging and observability
  - Kubernetes resources

### Knowledge Graph

- **[Graph Schema](graph-schema.md)** - Node types, relationships, and properties
  - Core Kubernetes resources
  - Extension resources (Istio, Gateway API)
  - Agent session nodes
  - Investigation nodes

### Querying

- **[Cypher Queries](cypher-queries.md)** - Query examples and patterns
  - Basic queries
  - Relationship traversal
  - Advanced patterns

- **[Metrics RCA Queries](metrics-rca-queries.md)** - Metrics-based root cause analysis
  - Prometheus integration
  - Metrics correlation

### MCP Tools

- **[MCP Tools API](mcp-tools-api.md)** - Complete MCP tools reference
  - Agent session tools
  - Investigation tools
  - Dashboard tools
  - Detailed schemas and examples

### Health & Monitoring

- **[Health Endpoint](health-endpoint.md)** - Service health check endpoints

## Quick Reference

### Common Configuration

```yaml
# Neo4j
NEO4J_URI: "bolt://neo4j:7687"
NEO4J_USERNAME: "neo4j"
NEO4J_PASSWORD: "<secret>"

# Watcher
NAMESPACE: ""
RESYNC_PERIOD: "30s"

# MCP Server
MCP_PORT: "8080"
```

### Node Types

- Pod, Service, Deployment, ReplicaSet
- Node, PersistentVolume, PersistentVolumeClaim
- ConfigMap, Secret
- Istio: VirtualService, DestinationRule, Gateway
- Gateway API: HTTPRoute, Gateway

### Relationship Types

- `MANAGES` - Deployment → ReplicaSet → Pod
- `SELECTS_PODS` - Service → Pod
- `SCHEDULED_ON` - Pod → Node
- `MOUNTS` - Pod → PVC
- `BOUND_TO` - PVC → PV
- `ROUTES_TO` - VirtualService → Service

See [Graph Schema](graph-schema.md) for complete list.

### Core MCP Tools

- `start_agent_session` - Begin investigation
- `query_with_session` - Execute Cypher queries
- `update_hypothesis` - Record diagnostic theory
- `spawn_investigation` - Get metrics data
- `record_finding` - Log insights
- `record_recommendation` - Document actions
- `complete_agent_session` - Finalize session

See [MCP Tools API](mcp-tools-api.md) for details.

## See Also

- [Service Documentation](../services/)
- [Guides](../guides/)
- [Getting Started](../getting-started/)

