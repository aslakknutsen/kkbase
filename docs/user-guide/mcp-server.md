# MCP Server

The kkbase MCP (Model Context Protocol) server provides AI agents with direct access to the Kubernetes knowledge graph stored in Neo4j. It exposes a streaming HTTP API that follows the MCP specification, enabling intelligent systems to query and explore cluster topology, dependencies, and relationships.

## Overview

The MCP server is a standalone binary that connects to the same Neo4j database as the kkbase watcher. It provides two main tools:

- **query**: Execute read-only Cypher queries against the knowledge graph
- **structure**: Get a complete overview of the graph schema

## Installation

The MCP server can be deployed in two ways:

1. **Standalone Mode** - Separate binary (recommended for production)
2. **Integrated Mode** - Combined with watcher in one binary

See [MCP Deployment Options](mcp-deployment-options.md) for detailed comparison and deployment guides.

### Build from Source (Standalone Mode)

```bash
# Build the MCP server binary
make build-mcp-server

# Or build all binaries
make all
```

### Build Integrated Mode

```bash
# Build watcher with MCP support
make build-watcher
```

### Run Locally (Standalone Mode)

```bash
# Set required environment variables
export NEO4J_URI="bolt://localhost:7687"
export NEO4J_USERNAME="neo4j"
export NEO4J_PASSWORD="your-password"
export NEO4J_DATABASE="neo4j"
export MCP_PORT="8080"
export LOG_LEVEL="info"

# Run the server
./mcp-server

# Or use make
make run-mcp-server
```

### Run Locally (Integrated Mode)

```bash
# Set required environment variables
export NEO4J_URI="bolt://localhost:7687"
export NEO4J_USERNAME="neo4j"
export NEO4J_PASSWORD="your-password"
export NEO4J_DATABASE="neo4j"
export MCP_ENABLED="true"    # Enable MCP server
export MCP_PORT="8081"        # Use different port than health server (8080)
export LOG_LEVEL="info"

# Run the combined watcher+MCP binary
./watcher

# MCP endpoint: http://localhost:8081/mcp
# Health endpoint: http://localhost:8080/healthz
```

## Configuration

The MCP server is configured via environment variables:

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `NEO4J_URI` | Neo4j connection URI | `bolt://localhost:7687` | Yes |
| `NEO4J_USERNAME` | Neo4j username | `neo4j` | Yes |
| `NEO4J_PASSWORD` | Neo4j password | - | Yes |
| `NEO4J_DATABASE` | Neo4j database name | `neo4j` | No |
| `MCP_PORT` | HTTP server port | `8080` | No |
| `LOG_LEVEL` | Logging level (debug, info, warn, error) | `info` | No |

## API Endpoints

### MCP Endpoint

**URL**: `POST /mcp`

The main MCP endpoint that handles JSON-RPC 2.0 requests following the Model Context Protocol specification.

### Health Check

**URL**: `GET /health`

Returns server health status.

**Response**:
```json
{
  "status": "healthy",
  "service": "kkbase-mcp",
  "version": "1.0.0"
}
```

## Available Tools

### query

Execute a read-only Cypher query against the knowledge graph.

**Input Parameters**:
- `query` (string, required): The Cypher query to execute
- `params` (object, optional): Query parameters for parameterized queries

**Example Request**:
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "query",
    "arguments": {
      "query": "MATCH (p:Pod)-[:SCHEDULED_ON]->(n:Node) WHERE n.name = $nodeName RETURN p.name as pod, p.namespace as namespace LIMIT 10",
      "params": {
        "nodeName": "worker-node-1"
      }
    }
  }
}
```

**Security**: Only read-only queries are allowed. The server rejects any query containing write operations:
- CREATE
- DELETE
- SET
- MERGE
- REMOVE
- DROP
- DETACH DELETE

### structure

Get the complete graph schema including node types, relationship types, properties, and schema triplets.

**Input Parameters**: None

**Example Request**:
```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "tools/call",
  "params": {
    "name": "structure",
    "arguments": {}
  }
}
```

**Output**:
- `node_types`: Array of all node labels in the graph
- `node_properties`: Object mapping node types to their properties
- `relationship_types`: Array of all relationship types
- `relationship_properties`: Object mapping relationship types to their properties
- `schema_triplets`: Array of from-relationship-to patterns

## Usage Examples

### Basic Query: List All Pods

```cypher
MATCH (p:Pod)
RETURN p.name, p.namespace, p.status
LIMIT 10
```

### Find Services and Their Endpoints

```cypher
MATCH (s:Service)-[:EXPOSES]->(p:Pod)
WHERE s.namespace = 'default'
RETURN s.name as service, collect(p.name) as pods
```

### Trace Pod Dependencies

```cypher
MATCH (d:Deployment)-[:OWNS]->(rs:ReplicaSet)-[:OWNS]->(p:Pod)-[:SCHEDULED_ON]->(n:Node)
WHERE d.name = 'nginx-deployment'
RETURN d.name as deployment, rs.name as replicaset, p.name as pod, n.name as node
```

### Find All Resources in a Namespace

```cypher
MATCH (r)
WHERE r.namespace = 'kube-system'
RETURN labels(r) as type, r.name as name, r.status as status
LIMIT 50
```

### Explore Service Mesh Configuration

```cypher
MATCH (vs:VirtualService)-[:ROUTES_TO]->(s:Service)
RETURN vs.name, vs.namespace, collect(s.name) as services
```

## Integration with AI Agents

### Claude Desktop

Add to your Claude Desktop configuration (`claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "kkbase": {
      "url": "http://localhost:8080/mcp",
      "transport": "streamable-http"
    }
  }
}
```

### MCP Inspector

Use the MCP Inspector to test and debug:

```bash
npx @modelcontextprotocol/inspector http://localhost:8080/mcp
```

## Security Considerations

### Current Implementation

- **Read-Only Access**: All queries are validated to ensure they don't modify data
- **No Authentication**: The current version doesn't implement authentication
- **HTTP Only**: Communication is not encrypted

### Production Recommendations

For production deployments, consider:

1. **Add TLS/HTTPS**: Encrypt all communication
2. **Implement Authentication**: Use OAuth 2.1 or API keys
3. **Network Policies**: Restrict access to the MCP server
4. **Rate Limiting**: Prevent abuse and resource exhaustion
5. **Query Timeouts**: Set limits on query execution time
6. **Audit Logging**: Log all queries for security monitoring

## Troubleshooting

### Connection Refused

**Problem**: Cannot connect to Neo4j

**Solution**: 
- Verify Neo4j is running: `curl http://localhost:7474`
- Check `NEO4J_URI` is correct
- Verify network connectivity

### Write Operation Rejected

**Problem**: Query returns "write operation detected" error

**Solution**: The query contains a write operation. Only read-only queries (MATCH, RETURN) are allowed.

### Empty Results

**Problem**: Query returns no results

**Solution**:
- Verify the watcher is running and syncing data
- Check the query syntax
- Use the `structure` tool to explore available data

### Server Won't Start

**Problem**: MCP server fails to start

**Solution**:
- Check all required environment variables are set
- Verify `NEO4J_PASSWORD` is provided
- Check port 8080 (or `MCP_PORT`) is available
- Review logs for detailed error messages

## Performance Considerations

- **Query Optimization**: Use LIMIT clauses to restrict result set size
- **Indexes**: The watcher creates indexes on `id` and `placeholder` fields
- **Connection Pooling**: The Neo4j driver maintains a connection pool
- **Concurrent Requests**: The HTTP server handles multiple requests concurrently

## Development

### Running Tests

```bash
# Run all tests
make test

# Run MCP package tests only
go test ./pkg/mcp/... -v
```

### Adding New Tools

To add a new tool to the MCP server:

1. Define input/output types in `pkg/mcp/types.go`
2. Implement the tool handler in `pkg/mcp/tools.go`
3. Register the tool in `pkg/mcp/server.go` using `mcp.AddTool()`
4. Add tests in `pkg/mcp/tools_test.go`

## Related Documentation

- [Graph Schema Reference](../reference/graph-schema.md)
- [Cypher Query Examples](../reference/cypher-queries.md)
- [Architecture Overview](../development/architecture.md)
- [MCP Specification](https://modelcontextprotocol.io/specification)

