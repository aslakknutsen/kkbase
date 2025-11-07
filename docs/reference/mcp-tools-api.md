# MCP Tools API Reference

Complete specification for all MCP tools available via the kkbase MCP Server.

## Complete Documentation

**See**: [MCP Server Tools Reference](../services/mcp-server/tools-reference.md)

The comprehensive tools reference includes:
- Input/output schemas for all tools
- Detailed examples
- Side effects and errors
- Tool execution flow
- Best practices

## Quick Reference

### Agent Session Tools

| Tool | Purpose |
|------|---------|
| `start_agent_session` | Initialize investigation session |
| `update_hypothesis` | Record current diagnostic theory |
| `query_with_session` | Execute Cypher queries with finding extraction |
| `spawn_investigation` | Launch metrics investigation for resource |
| `record_finding` | Explicitly log synthesized insights |
| `record_recommendation` | Document actionable next steps |
| `complete_agent_session` | Finalize investigation |

### Dashboard Tools (Read-Only)

| Tool | Purpose |
|------|---------|
| `get_active_sessions` | List all active investigation sessions |
| `get_session_details` | Get complete session state |
| `get_blast_zone` | Get affected resources graph |
| `get_session_timeline` | Get chronological event log |

### Core Tools

| Tool | Purpose |
|------|---------|
| `query` | Execute standalone Cypher query |
| `structure` | Get graph schema overview |

### Investigation Tools (Requires Prometheus)

| Tool | Purpose |
|------|---------|
| `get_investigation_status` | Check metrics investigation status |
| `complete_investigation` | Finalize metrics investigation |

## Tool Availability

| Category | Always Available | Requires Configuration |
|----------|------------------|----------------------|
| Core | ✅ | - |
| Agent Session | ✅ | - |
| Dashboard | ✅ | - |
| Investigation | ❌ | PROMETHEUS_URL |

## Standard Execution Flow

```
1. start_agent_session
2. update_hypothesis (repeat as understanding evolves)
3. query_with_session (repeat as needed)
4. spawn_investigation (optional, can spawn multiple)
5. record_finding (optional)
6. record_recommendation (repeat for each recommendation)
7. complete_agent_session
```

## Examples

### Start Session

```json
{
  "name": "start_agent_session",
  "arguments": {
    "symptom": "Service orders-api returning 503 errors",
    "initial_resource": "Service/production/orders-api"
  }
}
```

### Query with Session

```json
{
  "name": "query_with_session",
  "arguments": {
    "session_id": "session-abc123",
    "query": "MATCH (s:Service {name: $service_name})-[:SELECTS_PODS]->(p:Pod) RETURN s, p",
    "reasoning": "Check health of pods backing the service",
    "params": {
      "service_name": "orders-api"
    }
  }
}
```

### Record Recommendation

```json
{
  "name": "record_recommendation",
  "arguments": {
    "session_id": "session-abc123",
    "type": "root_cause_fix",
    "priority": "critical",
    "title": "Rollback to v2.3.4",
    "description": "Memory leak detected in v2.3.5",
    "rationale": "Memory usage shows 30% increase, OOM kills after 10 minutes",
    "related_findings": ["finding-oom-1", "finding-oom-2"],
    "action_items": [
      "kubectl rollout undo deployment/orders-api",
      "Monitor for 30 minutes",
      "Create hotfix for v2.3.6"
    ],
    "automation_hint": "kubectl rollout undo deployment/orders-api"
  }
}
```

## See Also

- **[Complete Tools Reference](../services/mcp-server/tools-reference.md)** - Full specifications
- **[Investigation Workflow](../guides/investigations/workflow.md)** - How to use tools
- **[Best Practices](../guides/investigations/best-practices.md)** - Patterns and tips
- **[MCP Server Configuration](../services/mcp-server/configuration.md)** - Enable tools

