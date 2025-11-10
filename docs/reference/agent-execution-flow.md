# Agent Investigation Tool Execution Flow

## Quick Reference for AI Agents

### Standard RCA Investigation Flow

Execute tools in this order for complete root cause analysis:

1. **`start_agent_session`**
   - **Input**: `symptom` (required), `initial_resource` (optional)
   - **Purpose**: Initialize investigation session
   - **When**: At the start of every investigation

2. **`update_hypothesis`**
   - **Input**: `session_id`, `stage`, `text`
   - **Purpose**: Record your current diagnostic theory
   - **When**: Each time your understanding evolves
   - **Side Effect**: Triggers blast zone recalculation
   - **Can repeat**: Yes - increment stage each time

3. **`query_with_session`**
   - **Input**: `session_id`, `query`, `reasoning`, `params` (optional)
   - **Purpose**: Execute Cypher queries to explore the graph
   - **When**: Investigating resources, relationships, or patterns
   - **Side Effect**: Automatically extracts findings from results
   - **Can repeat**: Yes - use as many queries as needed

4. **`spawn_investigation`** *(Optional, Sub-flow)*
   - **Input**: `session_id`, `resource_type`, `resource_id`, `symptom`, `lookback_minutes`
   - **Purpose**: Launch metrics investigation for a specific resource
   - **When**: You need metrics data (CPU, memory, latency, etc.)
   - **Can repeat**: Yes - spawn multiple investigations for different resources
   - **Note**: This is a **sub-flow** within the agent session, not a separate flow

5. **`record_finding`** *(Optional)*
   - **Input**: `session_id`, `type`, `resource_id`, `description`, `severity`, `evidence`
   - **Purpose**: Explicitly record synthesized insights
   - **When**: You make connections or discoveries not auto-detected
   - **Can repeat**: Yes - record multiple findings

6. **`record_recommendation`**
   - **Input**: `session_id`, `type`, `priority`, `title`, `description`, `rationale`, `related_findings`, `action_items`
   - **Purpose**: Record actionable next steps
   - **When**: Near end of investigation, after identifying root cause
   - **Can repeat**: Yes - record multiple recommendations
   - **Types**: `root_cause_fix`, `preventive_action`, `optimization`, `monitoring_improvement`, `cleanup`
   - **Priorities**: `critical`, `high`, `medium`, `low`

7. **`complete_agent_session`**
   - **Input**: `session_id`, `summary` (optional)
   - **Purpose**: Finalize investigation
   - **When**: Investigation complete
   - **Side Effect**: Marks session as completed, finalizes blast zone

---

## Key Concepts

### Agent Session (Top-Level)
- The complete investigation from start to finish
- Tracks hypotheses, queries, findings, and recommendations
- Has a blast zone that evolves as you discover issues

### Investigation (Sub-Flow)
- Metrics investigation for a specific resource
- **Not a separate flow** - it's a tool within the agent session
- Spawned via `spawn_investigation` when metrics analysis is needed
- Can spawn multiple investigations per session
- Each investigation is linked to the parent session

### Recommendations
- Actionable next steps based on findings
- Should be linked to related findings for evidence traceability
- Record multiple recommendations if needed (fix root cause, prevent recurrence, improve monitoring, etc.)
- Include automation hints (kubectl commands, scripts) when possible

---

## Example Complete Investigation

```
1. start_agent_session
   → symptom: "Orders API returning 503 errors"
   → session_id: "session-abc123"

2. update_hypothesis (stage 1)
   → "Service may be failing due to backend pod issues"

3. query_with_session
   → Check pod health
   → Auto-finds: 3 unhealthy pods (automatic findings)

4. update_hypothesis (stage 2)
   → "Pods crashing due to OOMKilled events"
   → Triggers blast zone recalculation

5. spawn_investigation (sub-flow)
   → Get memory metrics for affected pods
   → investigation_id: "inv-memory-xyz"

6. query_with_session
   → Check recent deployments
   → Auto-finds: Recent deployment v2.3.5

7. record_finding (explicit)
   → Root cause: Memory leak in v2.3.5

8. record_recommendation #1
   → Type: root_cause_fix
   → Priority: critical
   → Title: "Rollback to v2.3.4"
   → Related findings: [finding-1, finding-2]

9. record_recommendation #2
   → Type: preventive_action
   → Priority: high
   → Title: "Increase memory limits"

10. record_recommendation #3
    → Type: monitoring_improvement
    → Priority: medium
    → Title: "Add memory usage alerts"

11. complete_agent_session
    → Summary: "Memory leak in v2.3.5 causing OOMKills. Rollback recommended."
```

---

## Tool Repeatability

| Tool | Can Repeat? | Notes |
|------|-------------|-------|
| `start_agent_session` | No | Once per investigation |
| `update_hypothesis` | **Yes** | Update as understanding evolves |
| `query_with_session` | **Yes** | Execute as many queries as needed |
| `spawn_investigation` | **Yes** | Spawn for each resource needing metrics |
| `record_finding` | **Yes** | Record all synthesized insights |
| `record_recommendation` | **Yes** | Record multiple actionable recommendations |
| `complete_agent_session` | No | Once at the end |

---

## Investigation Sub-Flow Details

When you call `spawn_investigation`:
- Creates an Investigation node in Neo4j
- Pulls metrics from Prometheus (if configured)
- Links Investigation to parent AgentSession
- Optionally links to current Hypothesis
- Returns investigation_id for querying metrics

Multiple investigations can coexist within a single session:
```
session-abc123
├── investigation-pod-1 (memory metrics)
├── investigation-pod-2 (cpu metrics)
└── investigation-db (latency metrics)
```

Each investigation is a **sub-flow**, not a separate investigation session.

---

## Recommendation Best Practices

1. **Link to findings**: Always use `related_findings` array to show evidence
2. **Be specific**: Provide concrete action items, not vague suggestions
3. **Include automation**: Add kubectl commands, scripts, or runbooks when possible
4. **Estimate effort**: Help prioritize by providing realistic time estimates
5. **Prioritize correctly**:
   - `critical`: Immediate action required (root cause fixes)
   - `high`: Important, address soon (preventive measures)
   - `medium`: Worthwhile improvements (monitoring, optimization)
   - `low`: Nice to have (cleanup, technical debt)

---

## Common Mistakes to Avoid

❌ **Don't treat investigations as separate flows**
- Investigations are spawned within agent sessions, not started independently

❌ **Don't forget to record recommendations**
- Always provide actionable next steps before completing the session

❌ **Don't skip linking recommendations to findings**
- Use `related_findings` to show evidence supporting each recommendation

❌ **Don't forget to update hypothesis**
- Update hypothesis as your understanding evolves to trigger blast zone updates

❌ **Don't forget to complete the session**
- Always call `complete_agent_session` when done

---

## See Also

- [Complete MCP Tools Reference](./agent-mcp-tools.md)
- [Detailed Workflow Guide](../guides/investigations/workflow.md)
- [Graph Schema Reference](./graph-schema.md)

