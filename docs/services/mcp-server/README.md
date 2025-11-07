# MCP Server

The MCP Server exposes the kkbase knowledge graph to AI agents via the Model Context Protocol (MCP), enabling autonomous diagnostics and intelligent troubleshooting.

## What It Does

The MCP Server:
- **Implements** Model Context Protocol over HTTP/SSE
- **Provides** tools for querying and investigating the knowledge graph
- **Tracks** agent diagnostic sessions with hypothesis evolution
- **Manages** blast zone analysis and impact assessment
- **Serves** embedded web dashboard for real-time monitoring
- **Integrates** with Prometheus for metrics-based RCA (optional)

## When to Use

Deploy the MCP Server when you want:

- **AI agent integration** - Connect Cursor, Claude, or custom agents
- **Autonomous diagnostics** - Enable AI-driven troubleshooting
- **Session tracking** - Monitor investigation progress in real-time
- **Structured access** - Provide programmatic access to the knowledge graph
- **Investigation history** - Track and learn from past diagnostics

## Architecture

```
┌──────────────────────────────────────────────────────┐
│              AI Agent Tools                           │
│  ┌──────────┐  ┌────────────┐  ┌────────────────┐  │
│  │ Cursor   │  │   Claude   │  │ Custom Agents  │  │
│  └────┬─────┘  └──────┬─────┘  └────────┬───────┘  │
└───────┼────────────────┼─────────────────┼──────────┘
        │                │                 │
        │  HTTP/SSE (JSON-RPC 2.0)        │
        └────────────────┼─────────────────┘
                         ↓
        ┌────────────────────────────────┐
        │       MCP Server               │
        │                                │
        │  ┌──────────────────────────┐ │
        │  │  MCP Tools               │ │
        │  │  - query                 │ │
        │  │  - structure             │ │
        │  │  - start_agent_session   │ │
        │  │  - query_with_session    │ │
        │  │  - update_hypothesis     │ │
        │  │  - spawn_investigation   │ │
        │  │  - record_finding        │ │
        │  │  - record_recommendation │ │
        │  └──────────────────────────┘ │
        │                                │
        │  ┌──────────────────────────┐ │
        │  │  Session Manager         │ │
        │  │  - Track investigations  │ │
        │  │  - Blast zone calculation│ │
        │  │  - Finding extraction    │ │
        │  └──────────────────────────┘ │
        │                                │
        │  ┌──────────────────────────┐ │
        │  │  Web Dashboard (embedded)│ │
        │  │  - Real-time SSE updates │ │
        │  │  - Session visualization │ │
        │  └──────────────────────────┘ │
        └────────┬───────────────────────┘
                 │ Cypher Queries
                 ↓
        ┌────────────────────┐
        │       Neo4j        │
        │  Knowledge Graph   │
        └────────────────────┘
                 ↑
                 │ (Optional)
        ┌────────────────────┐
        │    Prometheus      │
        │  (Metrics RCA)     │
        └────────────────────┘
```

## Key Features

### Model Context Protocol (MCP)

Industry-standard protocol for AI agent communication:
- **JSON-RPC 2.0** over HTTP/SSE
- **Tool-based interface** for structured interactions
- **Streaming responses** for real-time updates
- **Session management** for stateful investigations

### Agent Session Tracking

Complete investigation lifecycle management:
- **Session initialization** with symptom description
- **Hypothesis evolution** tracking diagnostic theories
- **Query logging** with reasoning and results
- **Automatic finding extraction** from query results
- **Blast zone calculation** showing affected resources
- **Recommendation recording** for actionable next steps

### Investigation Tools

Metrics-based root cause analysis:
- **Spawn investigations** for specific resources
- **Pull metrics** from Prometheus
- **Correlate** with knowledge graph
- **Track** investigation progress
- **Cleanup** after completion

### Web Dashboard

Real-time monitoring interface:
- **Active sessions** list with status
- **Hypothesis timeline** showing investigation evolution
- **Blast zone visualization** with affected resources
- **Findings display** with severity and evidence
- **Query history** with reasoning
- **Recommendations** with action items
- **SSE updates** for live refresh

## Deployment Patterns

### Pattern 1: Standalone (Recommended for Production)

MCP Server deployed separately from watcher:

```
Watcher (sync) → Neo4j ← MCP Server (query) ← AI Agents
```

**Advantages**:
- Independent scaling
- Separate failure domains
- Clear separation of concerns
- Easier to secure MCP endpoint

### Pattern 2: Integrated (Simpler for Dev)

MCP Server integrated with watcher in single binary:

```
Watcher + MCP Server → Neo4j
           ↑
       AI Agents
```

**Advantages**:
- Single deployment
- Shared configuration
- Simpler setup
- Lower resource usage

See [Deployment Guide](deployment.md) for detailed instructions.

## Available Tools

### Core Tools

**query** - Execute read-only Cypher queries:
- Direct graph access
- No session tracking
- Security: write operations rejected

**structure** - Get complete graph schema:
- Node types
- Relationship types
- Properties
- Schema triplets

### Agent Session Tools

**start_agent_session** - Initialize investigation:
- Input: symptom description, optional initial resource
- Output: session ID
- Creates AgentSession node in graph

**update_hypothesis** - Record diagnostic theory:
- Input: session ID, stage number, hypothesis text
- Triggers blast zone recalculation
- Marks previous hypotheses as superseded

**query_with_session** - Execute query with tracking:
- Input: session ID, query, reasoning
- Auto-extracts findings from results
- Links findings to affected resources
- Updates session state

**record_finding** - Log discovered issues:
- Input: type, resource, description, severity, evidence
- Creates Finding node
- Links to AgentSession and affected resources

**record_recommendation** - Document action items:
- Input: type, priority, title, description, rationale, action items
- Links to related findings
- Provides automation hints

**complete_agent_session** - Finalize investigation:
- Marks session as completed
- Finalizes blast zone snapshot
- Generates summary

### Investigation Tools (Prometheus Required)

**spawn_investigation** - Launch metrics analysis:
- Input: resource type/ID, symptom, lookback period
- Pulls metrics from Prometheus
- Creates Investigation node
- Links to agent session

**get_investigation_status** - Check progress:
- Input: investigation ID
- Returns status, metrics collected, findings

**complete_investigation** - Cleanup metrics:
- Input: investigation ID
- Purges metric data points
- Returns count purged

### Dashboard Tools

**get_active_sessions** - List active investigations
**get_session_details** - Get complete session state
**get_blast_zone** - Get affected resources graph
**get_session_timeline** - Get chronological event list

See [Tools Reference](tools-reference.md) for complete API documentation.

## Configuration

Key configuration options:

| Variable | Purpose | Default |
|----------|---------|---------|
| `NEO4J_URI` | Neo4j connection | `bolt://localhost:7687` |
| `MCP_PORT` | HTTP server port | `8080` |
| `PROMETHEUS_URL` | Metrics source (enables investigation tools) | - |
| `LOG_LEVEL` | Logging verbosity | `info` |

See [Configuration Guide](configuration.md) for all options.

## Quick Deploy

### Standalone Mode

```bash
kubectl apply -f deploy/rbac.yaml
kubectl apply -f deploy/mcp-server-deployment.yaml
kubectl apply -f deploy/mcp-server-service.yaml
```

### Integrated Mode

```bash
kubectl apply -f deploy/rbac.yaml
kubectl apply -f deploy/deployment-integrated.yaml
kubectl apply -f deploy/service-integrated.yaml
```

See [Deployment Guide](deployment.md) for detailed instructions.

## Access Methods

### Via Dashboard

```bash
# Port forward
kubectl port-forward svc/kkbase-mcp 8080:8080

# Open browser
open http://localhost:8080/
```

### Via AI Tools (Cursor/Claude)

Configure MCP endpoint in your AI tool:

```json
{
  "mcpServers": {
    "kkbase": {
      "url": "http://localhost:8080/mcp",
      "transport": "sse"
    }
  }
}
```

### Via HTTP API

```bash
curl -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "tools/list"
  }'
```

## Performance

### Resource Usage

Typical consumption:
```
Memory: 50MB baseline + session state (~1MB per active session)
CPU: <0.1 core average
Latency: <100ms for typical queries
Concurrent: Handles 10s of simultaneous agent sessions
```

### Scalability

- **Sessions**: Supports dozens of concurrent investigations
- **Queries**: Sub-second for typical graph traversals
- **Dashboard**: Real-time updates via SSE
- **Horizontal scaling**: Multiple replicas supported

## Security

### Current Implementation

- **Read-only queries**: Write operations rejected
- **No authentication**: Open access (not production-ready)
- **HTTP only**: No encryption

### Production Recommendations

1. **Add TLS/HTTPS** - Encrypt communication
2. **Implement authentication** - OAuth 2.1 or API keys
3. **Network policies** - Restrict access
4. **Rate limiting** - Prevent abuse
5. **Audit logging** - Track all queries

See [Deployment Guide](deployment.md) for security setup.

## Monitoring

### Health Checks

```bash
# Health endpoint
curl http://localhost:8080/health

# Expected: {"status": "healthy"}
```

### Logs

```bash
# View logs
kubectl logs -f deployment/kkbase-mcp-server

# Check for errors
kubectl logs deployment/kkbase-mcp-server | grep ERROR

# Monitor agent sessions
kubectl logs -f deployment/kkbase-mcp-server | grep "agent session"
```

### Metrics (Planned)

Future Prometheus metrics:
- Tool calls per type
- Query latency distribution
- Active sessions count
- Session completion rate
- Error rates

## Troubleshooting

### MCP Server Won't Start

```bash
# Check deployment
kubectl get deployment kkbase-mcp-server

# Check logs
kubectl logs deployment/kkbase-mcp-server

# Common issues:
# - Neo4j not accessible
# - Invalid configuration
# - Port already in use
```

### AI Tool Can't Connect

```bash
# Test endpoint
curl -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'

# Check port forward
kubectl port-forward svc/kkbase-mcp 8080:8080

# Verify service
kubectl get svc kkbase-mcp
```

### Dashboard Shows No Sessions

Sessions only appear when created by agents. Test manually:

```bash
curl -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "tools/call",
    "params": {
      "name": "start_agent_session",
      "arguments": {
        "symptom": "Test session"
      }
    }
  }'
```

See [Troubleshooting Guide](../../guides/operations/troubleshooting.md) for more solutions.

## Extension

### Adding Custom Tools

1. Define types in `pkg/mcp/types.go`
2. Implement handler in `pkg/mcp/tools.go`
3. Register in `pkg/mcp/server.go`
4. Add tests
5. Update documentation

See [Extending MCP Guide](../../development/extending-mcp.md)

## Best Practices

1. **Use sessions** - Track investigations with agent sessions
2. **Update hypothesis** - Evolve theories as investigation progresses
3. **Explain queries** - Provide reasoning for better tracking
4. **Record findings** - Capture both automatic and synthesized insights
5. **Add recommendations** - Document actionable next steps
6. **Complete sessions** - Always finalize investigations
7. **Monitor dashboard** - Watch investigations in real-time

## Documentation

- **[Deployment Guide](deployment.md)** - Step-by-step deployment
- **[Configuration](configuration.md)** - All configuration options
- **[Tools Reference](tools-reference.md)** - Complete API documentation
- **[Dashboard Guide](dashboard.md)** - Web UI features
- **[Investigation Workflow](../../guides/investigations/workflow.md)** - How agents investigate

## Quick Links

- [Getting Started](../../getting-started/)
- [System Architecture](../../ARCHITECTURE.md)
- [Investigation Guide](../../guides/investigations/)
- [Operations Guide](../../guides/operations/)

