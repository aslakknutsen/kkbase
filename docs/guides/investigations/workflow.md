# Agent Investigation Workflow Guide

## Overview

This guide demonstrates a complete agent investigation session from start to finish, showing how the AI agent (in Cursor) diagnoses cluster issues while the web dashboard observes in real-time.

## Prerequisites

1. **Backend running**: MCP server connected to Neo4j and Prometheus
2. **Dashboard open**: Browser at http://localhost:8080/
3. **Cursor with MCP**: Cursor configured with kkbase MCP server

## Tool Execution Order

### Complete RCA Investigation Flow

Use this flow for full root cause analysis investigations:

1. **`start_agent_session`** - Initialize investigation with symptom description
2. **`update_hypothesis`** - Record your current theory (repeat as understanding evolves)
3. **`query_with_session`** - Execute Cypher queries (repeat as needed to explore graph)
4. **`spawn_investigation`** - (Optional, can repeat) Launch metrics investigation for specific resources
   - This is a **sub-flow** within the agent session, not a separate flow
   - Can spawn multiple investigations for different resources
   - Metrics are linked to the parent session
5. **`record_finding`** - (Optional, can repeat) Record synthesized insights
6. **`record_recommendation`** - (Can repeat) Record actionable next steps
   - Link to related findings for evidence
   - Provide specific action items
   - Include automation hints when possible
7. **`complete_agent_session`** - Finalize and generate summary

### Key Concepts

- **Agent Session**: The top-level investigation that tracks your entire diagnostic process
- **Investigation (Metrics)**: A sub-component within an agent session that pulls metrics data for a specific resource
  - Spawned via `spawn_investigation` when you need metrics analysis
  - Can have multiple investigations per session
  - Each investigation is linked to the parent session
- **Recommendations**: Actionable next steps recorded near the end of investigation
  - Should be linked to findings using `related_findings` array
  - Multiple recommendations can be recorded per session

## Complete Workflow Example

### Step 1: Agent Starts Investigation

**In Cursor, the AI agent calls:**

```
Tool: start_agent_session
Input: {
  "symptom": "Order service API failing with 503 errors",
  "initial_resource": "Service/production/order-service"
}
```

**Backend creates:**
- AgentSession node in Neo4j
- Returns session ID: `session-abc123`

**Dashboard updates:**
- New session appears in sidebar
- Shows "Order service API failing with 503 errors"
- Status: Active
- Stage: 1

### Step 2: Agent Forms Initial Hypothesis

**Agent calls:**

```
Tool: update_hypothesis
Input: {
  "session_id": "session-abc123",
  "stage": 1,
  "text": "Service may be failing due to backend pod issues"
}
```

**Backend:**
- Creates Hypothesis node
- Links to AgentSession
- Triggers initial blast zone calculation (empty at this point)

**Dashboard updates:**
- Hypothesis panel shows: "Service may be failing due to backend pod issues"
- Stage indicator: 1
- Blast zone graph: Empty (no findings yet)

### Step 3: Agent Investigates Service

**Agent calls:**

```
Tool: query_with_session
Input: {
  "session_id": "session-abc123",
  "query": "MATCH (s:Service {name: 'order-service'})-[:SELECTS_PODS]->(p:Pod) RETURN s, p, p.status as pod_status",
  "reasoning": "Check the health of pods backing the order-service"
}
```

**Backend:**
- Executes Cypher query
- Returns pod list with statuses
- **Automatic finding extraction** detects:
  - 3 pods in "CrashLoopBackOff" state
  - Creates Finding nodes:
    - Type: `unhealthy_pod`
    - Severity: `critical`
    - Resource: `Pod/production/order-service-xyz`
    - Detection: `automatic`

**Dashboard updates:**
- Query History: Shows query + reasoning
- Findings List: 3 new critical findings appear
  - "Pod not running: CrashLoopBackOff"
  - Evidence shows pod details
- Timeline: Query execution event added
- Blast Zone: Still calculating...

### Step 4: Agent Refines Hypothesis

**Agent reviews findings and calls:**

```
Tool: update_hypothesis
Input: {
  "session_id": "session-abc123",
  "stage": 2,
  "text": "Pods are crashing due to OOMKilled events - likely memory leak in recent deployment"
}
```

**Backend:**
- Creates new Hypothesis node (stage 2)
- Marks previous hypothesis as "superseded"
- **Triggers blast zone recalculation** ⚡
  - Finds affected resources (3 pods)
  - Expands 3 hops through relationships
  - Discovers: Service → Pods → ReplicaSet → Deployment
  - Calculates impact: 5 resources affected
- **Auto-spawns Investigation** (optional):
  - Detects "OOMKilled" keyword
  - Launches metrics investigation for memory analysis
  - Links Investigation to Hypothesis

**Dashboard updates:**
- Hypothesis Panel: "Pods are crashing due to OOMKilled..."
- Stage: 2
- Blast Zone Graph: **Major update!**
  - Red nodes: 3 failed pods
  - Yellow nodes: order-service (degraded)
  - Green nodes: ReplicaSet, Deployment (affected but healthy)
  - Animated red edges: Service → Pods (failing selectors)
  - Impact: 5 resources, 3-hop radius
- Linked Investigations: Shows new metrics investigation ID

### Step 5: Agent Checks Recent Deployments

**Agent calls:**

```
Tool: query_with_session
Input: {
  "session_id": "session-abc123",
  "query": "MATCH (d:Deployment {name: 'order-service'})-[:MANAGES]->(rs:ReplicaSet) WHERE rs.creationTimestamp > $recent RETURN rs ORDER BY rs.creationTimestamp DESC LIMIT 1",
  "reasoning": "Find recent deployments that may have introduced the memory leak",
  "params": {
    "recent": "2024-11-03T10:00:00Z"
  }
}
```

**Backend:**
- Executes query
- **Automatic finding extraction** detects:
  - Recent ReplicaSet created 15 minutes ago
  - Creates Finding:
    - Type: `deployment_change`
    - Severity: `warning`
    - Description: "Recent deployment detected: v2.3.5"

**Dashboard updates:**
- Query History: New query with deployment search
- Findings: "Recent deployment detected: v2.3.5"
- Timeline: New query + finding events

### Step 6: Agent Records Explicit Finding

**Agent synthesizes insight and calls:**

```
Tool: record_finding
Input: {
  "session_id": "session-abc123",
  "type": "root_cause",
  "resource_id": "Deployment/production/order-service",
  "description": "Root cause: Memory leak introduced in v2.3.5 deployment at 14:15. Recommendation: Rollback to v2.3.4",
  "severity": "critical",
  "evidence": {
    "deployment_version": "v2.3.5",
    "deployment_time": "2024-11-03T14:15:00Z",
    "oom_count": 3,
    "memory_trend": "increasing",
    "rollback_target": "v2.3.4"
  }
}
```

**Backend:**
- Creates Finding node (agent-recorded)
- Links to Deployment resource
- Does NOT trigger automatic extraction (explicit)

**Dashboard updates:**
- Findings: New critical finding at top
  - "Root cause: Memory leak introduced in v2.3.5..."
  - Detection method: `agent_recorded`
  - Expandable evidence shows all details
- Timeline: Finding discovery event

### Step 7: Agent Spawns Metrics Investigation

**Agent calls:**

```
Tool: spawn_investigation
Input: {
  "session_id": "session-abc123",
  "resource_type": "Deployment",
  "resource_id": "Deployment/production/order-service",
  "symptom": "Memory leak in v2.3.5",
  "lookback_minutes": 30
}
```

**Backend:**
- Calls existing `start_investigation` internally
- Pulls memory metrics from Prometheus (last 30 minutes)
- Creates Investigation node
- Links AgentSession → Investigation
- Returns investigation ID

**Dashboard updates:**
- Linked Investigations section expands
- Shows investigation ID: `inv-xyz789`
- Status: Metrics being collected

### Step 8: Agent Records Recommendations

**Agent calls:**

```
Tool: record_recommendation
Input: {
  "session_id": "session-abc123",
  "type": "root_cause_fix",
  "priority": "critical",
  "title": "Increase memory limit for order-service deployment",
  "description": "The order-service pods are being OOMKilled due to insufficient memory allocation. Current limit of 512Mi is too low based on observed usage patterns.",
  "rationale": "Memory usage analysis shows consistent growth pattern reaching 500-520Mi before OOMKill. The v2.3.5 deployment increased baseline memory usage by 30% compared to v2.3.4.",
  "related_findings": ["finding-oomkilled-1", "finding-oomkilled-2"],
  "action_items": [
    "Update deployment manifest to increase memory limit from 512Mi to 1Gi",
    "Update memory request to 768Mi for better pod scheduling",
    "Monitor memory usage for 24 hours after deployment"
  ],
  "estimated_effort": "15 minutes",
  "automation_hint": "kubectl set resources deployment order-service -n production --limits=memory=1Gi --requests=memory=768Mi",
  "tags": ["memory", "oomkilled", "production"]
}
```

**Backend:**
- Creates Recommendation node in Neo4j
- Links to AgentSession via HAS_RECOMMENDATION
- Links to related Finding nodes via BASED_ON
- Returns recommendation ID

**Dashboard updates:**
- Recommendations section appears/updates
- Shows recommendation card with priority badge (CRITICAL)
- Displays action items as numbered list
- Shows automation hint in code block
- Indicates link to 2 findings

**Agent may record multiple recommendations:**

```
Tool: record_recommendation
Input: {
  "session_id": "session-abc123",
  "type": "preventive_action",
  "priority": "high",
  "title": "Implement memory usage alerts for order-service",
  "description": "Add alerting to detect memory growth patterns before OOMKill occurs",
  "rationale": "Early detection would have prevented service disruption by alerting on memory growth trend",
  "related_findings": ["finding-oomkilled-1"],
  "action_items": [
    "Create Prometheus alert for memory usage > 80% of limit",
    "Add alert for memory growth rate > 10MB/minute",
    "Set up PagerDuty integration for critical alerts"
  ],
  "estimated_effort": "1 hour",
  "tags": ["monitoring", "preventive", "alerting"]
}
```

### Step 9: Agent Completes Session

**Agent calls:**

```
Tool: complete_agent_session
Input: {
  "session_id": "session-abc123",
  "summary": "Identified memory leak in v2.3.5 deployment causing OOMKilled pods. Rollback to v2.3.4 recommended."
}
```

**Backend:**
- Marks AgentSession as "completed"
- Finalizes blast zone snapshot
- Completes linked Investigation sessions
- Stores summary

**Dashboard updates:**
- Session status: Completed
- Summary displayed at top
- Session remains viewable but marked inactive
- New sessions list no longer includes this (moved to completed)

## Final Dashboard State

### Session Header
```
Order service API failing with 503 errors
Status: Completed ✓
Started: Nov 3, 2024 14:10
Completed: Nov 3, 2024 14:25
Duration: 15 minutes
```

### Current Hypothesis (Final)
```
Stage 2: Pods are crashing due to OOMKilled events - likely 
memory leak in recent deployment
```

### Blast Zone Graph
- **5 nodes total**
  - 3 red (failed pods)
  - 1 yellow (degraded service)
  - 1 green (deployment)
- **Impact**: 5 resources, 3-hop radius
- **Topology**: Service → Pods ← ReplicaSet ← Deployment

### Findings (4 total)
1. **Critical** - Root cause: Memory leak in v2.3.5 (agent-recorded)
2. **Critical** - Pod not running: CrashLoopBackOff × 3 (automatic)
3. **Warning** - Recent deployment: v2.3.5 (automatic)

### Recommendations (2 total)
1. **Critical** - Root Cause Fix: Increase memory limit for order-service deployment
   - 3 action items, Est. 15 minutes
   - Automation: kubectl command provided
   - Based on 2 findings
2. **High** - Preventive Action: Implement memory usage alerts
   - 3 action items, Est. 1 hour
   - Based on 1 finding

### Query History (3 queries)
1. Check pod health (2 results, 1 finding)
2. Find recent deployments (1 result, 1 finding)
3. Analyze deployment timeline (5 results)

### Timeline (14 events)
- Session created
- Hypothesis 1 added
- Query 1 executed
- 3 findings discovered
- Hypothesis 2 added
- Blast zone updated
- Query 2 executed
- Finding discovered
- Investigation spawned
- Root cause finding recorded
- Recommendation 1 recorded
- Recommendation 2 recorded
- Session completed

### Linked Investigations
- `inv-xyz789` - Memory metrics for order-service deployment

## Real-Time Updates During Investigation

**What the dashboard observer sees:**

```
14:10:00 - New session appears: "Order service API failing..."
14:10:05 - Hypothesis: "Service may be failing due to backend pod issues"
14:10:15 - Query executing... ⏳
14:10:17 - 3 findings discovered! 🔴
14:10:17 - Blast zone: Empty → 3 nodes
14:12:30 - Hypothesis updated: "Pods crashing due to OOMKilled..."
14:12:31 - Blast zone recalculating... ⏳
14:12:33 - Blast zone: 3 nodes → 5 nodes 📈
14:12:33 - Investigation spawned: Memory analysis
14:15:45 - Query executing...
14:15:47 - Finding: Recent deployment detected
14:18:20 - Root cause identified! 🎯
14:23:00 - Recommendation recorded: Increase memory limit (Critical) 🔴
14:24:00 - Recommendation recorded: Implement alerts (High) 🟠
14:25:00 - Session completed ✓
```

## MCP Tool Call Summary

| Tool | Purpose | When to Use | Dashboard Effect |
|------|---------|-------------|------------------|
| `start_agent_session` | Initialize investigation | User reports issue | Session appears in sidebar |
| `update_hypothesis` | Record current theory | Understanding evolves | Hypothesis panel + blast zone recalc |
| `query_with_session` | Execute Cypher + reason | Exploring graph (repeat) | Query history + auto findings |
| `spawn_investigation` | Launch metrics sub-flow | Need resource metrics (can repeat) | Linked investigation appears |
| `record_finding` | Explicit insight | Synthesize findings (can repeat) | Findings list update |
| `record_recommendation` | Actionable next steps | Near end of investigation (can repeat) | Recommendations list appears |
| `complete_agent_session` | Finalize session | Investigation complete | Status changes to completed |

**Flow Notes**:
- Tools marked "(can repeat)" can be called multiple times within a session
- `spawn_investigation` creates a **sub-flow** - investigations are not separate, they're linked to the parent session
- Recommendations should be recorded after findings are identified and before completing the session

## Best Practices

### For AI Agents

1. **Start with clear symptom**: Be specific in initial_symptom
2. **Update hypothesis frequently**: Each major insight should update hypothesis
3. **Explain query reasoning**: Always provide clear reasoning for queries
4. **Use investigations as sub-flows**: Call `spawn_investigation` when you need metrics data
   - Can spawn multiple investigations within a single session
   - Each investigation focuses on a specific resource
   - Results are linked to the parent agent session
5. **Record explicit findings**: Use `record_finding` for synthesized insights
6. **Record actionable recommendations**: Use `record_recommendation` to provide concrete next steps
   - Link recommendations to related findings using `related_findings` array
   - Prioritize root cause fixes as critical/high
   - Include automation hints (kubectl commands, scripts)
   - Provide realistic effort estimates
   - Record multiple recommendations if needed (root cause fix, preventive actions, monitoring improvements)
7. **Complete sessions**: Always call `complete_agent_session` when done

### For Dashboard Observers

1. **Monitor timeline**: Shows investigation flow chronologically
2. **Watch blast zone**: Expands as agent discovers more affected resources
3. **Review findings**: Auto-discovered vs agent-recorded show different insights
4. **Check linked investigations**: Shows when agent needs metrics data
5. **Export session**: Save completed sessions for incident reports

## Common Patterns

### Pattern 1: Service Outage Investigation
```
1. start_agent_session (symptom: service down)
2. update_hypothesis (theory: pod issues)
3. query_with_session (check pods) → auto-finds unhealthy pods
4. update_hypothesis (theory: deployment problem)
5. query_with_session (check recent deployments)
6. record_finding (root cause identified)
7. record_recommendation (fix: rollback deployment)
8. record_recommendation (preventive: add readiness probes)
9. complete_agent_session
```

### Pattern 2: Performance Degradation with Metrics
```
1. start_agent_session (symptom: high latency)
2. update_hypothesis (theory: slow dependencies)
3. query_with_session (trace analysis) → auto-finds slow calls
4. spawn_investigation (get latency metrics) ← sub-flow within session
5. update_hypothesis (theory: database bottleneck)
6. query_with_session (check database connections)
7. spawn_investigation (get database metrics) ← another sub-flow
8. record_finding (root cause + evidence)
9. record_recommendation (fix: increase db connection pool)
10. record_recommendation (monitoring: add latency alerts)
11. complete_agent_session
```

### Pattern 3: Error Spike Analysis
```
1. start_agent_session (symptom: error rate spike)
2. query_with_session (check error traces) → auto-finds error patterns
3. update_hypothesis (theory: failed external service)
4. query_with_session (check FAILED_CALL_TO relationships)
5. spawn_investigation (error rate metrics) ← sub-flow within session
6. record_finding (external service identified)
7. record_recommendation (fix: add circuit breaker)
8. record_recommendation (monitoring: alert on error rate)
9. complete_agent_session
```

### Pattern 4: Memory Leak Investigation (Multiple Investigations)
```
1. start_agent_session (symptom: OOMKilled pods)
2. update_hypothesis (theory: memory leak)
3. query_with_session (check affected pods)
4. spawn_investigation (pod memory metrics) ← first sub-flow
5. query_with_session (check recent deployments)
6. spawn_investigation (deployment metrics) ← second sub-flow
7. update_hypothesis (theory: leak in v2.3.5)
8. record_finding (root cause with evidence)
9. record_recommendation (fix: rollback to v2.3.4)
10. record_recommendation (fix: increase memory limits)
11. record_recommendation (monitoring: memory growth alerts)
12. complete_agent_session
```

**Note**: Investigations (spawned via `spawn_investigation`) are sub-flows within the agent session. You can spawn multiple investigations for different resources, and they're all linked to the parent session.

## Troubleshooting

### Session Not Appearing in Dashboard

**Check:**
1. MCP server logs: "Agent session created"
2. Neo4j: `MATCH (s:AgentSession) RETURN s`
3. Dashboard polling: Should refresh every 5 seconds

### Blast Zone Not Updating

**Trigger blast zone update:**
- Call `update_hypothesis` (always triggers recalc)
- Findings must have `AFFECTS` relationships to resources
- Check Neo4j: `MATCH (f:Finding)-[:AFFECTS]->(r) RETURN f, r`

### Findings Not Auto-Extracted

**Check query results:**
- Results must match patterns in `FindingExtractor`
- Look for: `error_message`, `status != "Running"`, `FAILED_CALL_TO`
- Use `record_finding` for explicit findings

### Investigation Not Spawning

**Verify:**
1. Prometheus URL configured
2. `spawn_investigation` called with correct resource type
3. Check logs: "Starting investigation for..."

## Next Steps

- See [MCP Tools Reference](../../reference/agent-mcp-tools.md) for complete API
- See [Graph Schema Reference](../../reference/graph-schema.md) for data model
- See [Dashboard Guide](../../services/mcp-server/dashboard.md) for UI features

