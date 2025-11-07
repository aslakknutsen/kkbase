# Investigation Guides

Learn how AI agents investigate cluster issues using kkbase.

## What's in This Section?

| Guide | Purpose | Level |
|-------|---------|-------|
| [Workflow](workflow.md) | Complete investigation flow with examples | Beginner |
| [Agent Sessions](agent-sessions.md) | Tool execution order and patterns | Intermediate |
| [Metrics RCA](metrics-rca.md) | Metrics-based root cause analysis | Advanced |
| [Best Practices](best-practices.md) | Patterns and anti-patterns | Intermediate |

## Quick Start

### Your First Investigation

In Cursor or Claude, ask:

```
Start a kkbase investigation session for:
"Service 'my-app' is returning 503 errors"

Query the graph to find the service and check its backend pods.
```

Watch the [dashboard](../../services/mcp-server/dashboard.md) in real-time!

## Investigation Flow Overview

```
1. start_agent_session
      ↓
2. update_hypothesis (evolve as you learn)
      ↓
3. query_with_session (explore the graph)
      ↓
4. spawn_investigation (get metrics - optional)
      ↓
5. record_finding (synthesize insights)
      ↓
6. record_recommendation (actionable next steps)
      ↓
7. complete_agent_session
```

See [Agent Sessions Guide](agent-sessions.md) for details.

## Learning Path

**New to investigations?**
1. Read [Workflow](workflow.md) - Complete examples
2. Try a simple investigation in Cursor
3. Watch dashboard to see what happens

**Familiar with basic flow?**
1. Study [Agent Sessions](agent-sessions.md) - Tool patterns
2. Learn [Metrics RCA](metrics-rca.md) - Advanced diagnostics
3. Review [Best Practices](best-practices.md) - Avoid pitfalls

**Building autonomous agents?**
1. Master [Best Practices](best-practices.md)
2. Study successful investigation patterns
3. Customize prompts for your environment

## Key Concepts

### Agent Session

The top-level investigation that tracks your entire diagnostic process:
- Tracks hypotheses
- Logs queries
- Extracts findings
- Calculates blast zone
- Records recommendations

### Hypothesis Evolution

As you investigate, your understanding evolves:
- Stage 1: "Service may be down"
- Stage 2: "Pods are crashing"
- Stage 3: "Memory leak in v2.3.5"

Each update triggers blast zone recalculation.

### Blast Zone

Affected resources discovered during investigation:
- Red: Failed/critical
- Yellow: Degraded
- Green: Healthy but affected

Automatically expands as findings are discovered.

### Findings

Issues discovered automatically or by agent:
- **Automatic**: Extracted from query results (unhealthy pods, errors)
- **Agent-recorded**: Synthesized insights (root cause identified)

### Recommendations

Actionable next steps with evidence:
- Root cause fixes
- Preventive actions
- Monitoring improvements
- Cleanup tasks

## Common Patterns

### Pattern 1: Service Outage

```
symptom: "service returning errors"
  ↓
check service → find pods → check events → identify cause
  ↓
recommendation: rollback deployment
```

### Pattern 2: Performance Degradation

```
symptom: "high latency"
  ↓
check service → trace dependencies → spawn metrics investigation
  ↓
find slow database → recommendation: scale database
```

### Pattern 3: Resource Exhaustion

```
symptom: "pods being evicted"
  ↓
check node → find resource pressure → check all pods on node
  ↓
recommendation: add node capacity
```

See [Best Practices](best-practices.md) for more patterns.

## Tools Reference

### Core Investigation Tools

- `start_agent_session` - Begin investigation
- `update_hypothesis` - Record current theory
- `query_with_session` - Explore graph
- `spawn_investigation` - Get metrics (optional)
- `record_finding` - Log insights
- `record_recommendation` - Document actions
- `complete_agent_session` - Finalize

See [MCP Tools Reference](../../services/mcp-server/tools-reference.md)

## See Also

- [MCP Server](../../services/mcp-server/) - Investigation platform
- [Agent Service](../../services/agent/) - Autonomous diagnostics
- [Query Guide](../querying/) - Cypher patterns
- [Dashboard](../../services/mcp-server/dashboard.md) - Monitor investigations

