# Agent Investigation MCP Tools Reference

## Overview

This document provides complete specifications for all MCP tools used in agent investigation sessions.

## Tool Categories

- **Agent Tools** (7): Write operations for AI agents conducting investigations
- **Dashboard Tools** (4): Read-only operations for web dashboard observation

---

## Tool Execution Flow

### Recommended Flow: Full RCA Investigation

The standard workflow for complete root cause analysis:

1. **`start_agent_session`** - Initialize investigation session with symptom description
2. **`update_hypothesis`** - Record your current diagnostic theory (triggers blast zone calculation)
3. **`query_with_session`** - Execute Cypher queries to explore the graph (repeat as needed)
   - Findings are automatically extracted from results
   - Use provided `reasoning` to explain what you're investigating
4. **`spawn_investigation`** - (Optional) Launch metrics investigation for specific resources
   - Can be spawned multiple times within a session for different resources
   - Links metrics data to the session for deeper analysis
5. **`record_finding`** - (Optional) Explicitly record synthesized insights
6. **`record_recommendation`** - Record actionable next steps based on findings
   - Can record multiple recommendations (root cause fixes, preventive actions, etc.)
   - Link to related findings for evidence traceability
7. **`complete_agent_session`** - Finalize investigation and generate summary

### Key Points

- **Investigations are a sub-flow**: `spawn_investigation` can be called multiple times within an agent session to gather metrics for different resources. It's not a separate investigation flow - it's a tool within the agent session workflow.
- **Update hypothesis frequently**: Each time your understanding evolves, call `update_hypothesis` to trigger blast zone recalculation
- **Record recommendations**: Near the end of investigation, use `record_recommendation` to provide actionable next steps
- **Complete the session**: Always call `complete_agent_session` when done to finalize findings

---

## Agent Tools (Write Operations)

### 1. start_agent_session

**Purpose**: Initialize a new agent investigation session.

**Input Schema**:
```json
{
  "symptom": "string (required)",
  "initial_resource": "string (optional)"
}
```

**Input Example**:
```json
{
  "symptom": "Order service returning 503 errors",
  "initial_resource": "Service/production/order-service"
}
```

**Output Schema**:
```json
{
  "session_id": "string",
  "status": "active",
  "created_at": "ISO8601 timestamp"
}
```

**Output Example**:
```json
{
  "session_id": "session-abc123-def456",
  "status": "active",
  "created_at": "2024-11-03T14:30:00Z"
}
```

**Side Effects**:
- Creates `AgentSession` node in Neo4j
- Emits notification: `agent_session/created`

**Errors**:
- `INVALID_INPUT`: Missing required `symptom` field
- `DATABASE_ERROR`: Neo4j connection failure

---

### 2. query_with_session

**Purpose**: Execute Cypher query within an investigation session, with automatic finding extraction.

**Input Schema**:
```json
{
  "session_id": "string (required)",
  "query": "string (required)",
  "reasoning": "string (required)",
  "params": "object (optional)"
}
```

**Input Example**:
```json
{
  "session_id": "session-abc123",
  "query": "MATCH (s:Service {name: $service_name})-[:SELECTS_PODS]->(p:Pod) RETURN s, p, p.status",
  "reasoning": "Check the health status of pods backing the order-service to identify any failed instances",
  "params": {
    "service_name": "order-service"
  }
}
```

**Output Schema**:
```json
{
  "query_id": "string",
  "result_count": "number",
  "duration_ms": "number",
  "results": "array of records",
  "findings": "array of Finding objects"
}
```

**Output Example**:
```json
{
  "query_id": "query-xyz789",
  "result_count": 5,
  "duration_ms": 45,
  "results": [
    {
      "s": {"name": "order-service", "type": "ClusterIP"},
      "p": {"name": "order-pod-1", "status": "Running"},
      "p.status": "Running"
    },
    {
      "s": {"name": "order-service", "type": "ClusterIP"},
      "p": {"name": "order-pod-2", "status": "CrashLoopBackOff"},
      "p.status": "CrashLoopBackOff"
    }
  ],
  "findings": [
    {
      "id": "finding-unhealthy-pod-1",
      "type": "unhealthy_pod",
      "severity": "critical",
      "resource_id": "Pod/production/order-pod-2",
      "description": "Pod not running: CrashLoopBackOff",
      "detection_method": "automatic"
    }
  ]
}
```

**Side Effects**:
- Creates `QueryExecution` node in Neo4j
- Links to AgentSession
- **Automatic finding extraction**:
  - Scans results for patterns
  - Creates `Finding` nodes for detected issues
  - Links findings to affected resources
- Emits notifications:
  - `agent_session/query_executed`
  - `agent_session/finding_discovered` (if findings detected)

**Finding Extraction Patterns**:
1. Unhealthy pods: `status != "Running"`
2. Failed calls: `FAILED_CALL_TO` relationships
3. Error messages: `error_message` field present
4. High restart counts: `restartCount > threshold`
5. OOMKilled events: `status.reason == "OOMKilled"`

**Errors**:
- `SESSION_NOT_FOUND`: Invalid session_id
- `INVALID_QUERY`: Cypher syntax error
- `QUERY_TIMEOUT`: Query exceeded time limit

---

### 3. update_hypothesis

**Purpose**: Update the agent's current hypothesis, triggering blast zone recalculation.

**Input Schema**:
```json
{
  "session_id": "string (required)",
  "stage": "number (required)",
  "text": "string (required)"
}
```

**Input Example**:
```json
{
  "session_id": "session-abc123",
  "stage": 2,
  "text": "Pods are crashing due to OOMKilled events. Recent deployment v2.3.5 likely introduced memory leak."
}
```

**Output Schema**:
```json
{
  "hypothesis_id": "string",
  "stage": "number",
  "status": "active",
  "blast_zone_updated": "boolean",
  "investigation_spawned": "boolean",
  "investigation_id": "string (optional)"
}
```

**Output Example**:
```json
{
  "hypothesis_id": "hypothesis-xyz",
  "stage": 2,
  "status": "active",
  "blast_zone_updated": true,
  "investigation_spawned": true,
  "investigation_id": "inv-memory-leak-123"
}
```

**Side Effects**:
- Creates new `Hypothesis` node
- Marks previous hypothesis as "superseded"
- **Triggers blast zone recalculation** (always)
- **May auto-spawn Investigation**:
  - If hypothesis text contains resource issue keywords
  - Keywords: "OOM", "memory", "CPU", "error rate", "latency"
  - Creates Investigation for metrics collection
  - Links with `TRIGGERED_INVESTIGATION` relationship
- Emits notifications:
  - `agent_session/hypothesis_updated`
  - `agent_session/blast_zone_updated`
  - `agent_session/investigation_spawned` (if applicable)

**Auto-Investigation Triggers**:
- "OOMKilled" → Memory investigation
- "CPU throttle" → CPU investigation
- "error rate" → Error metrics investigation
- "high latency" → Latency investigation

**Errors**:
- `SESSION_NOT_FOUND`: Invalid session_id
- `INVALID_STAGE`: Stage must be positive integer

---

### 4. record_finding

**Purpose**: Explicitly record a finding that the agent has synthesized (not auto-extracted).

**Input Schema**:
```json
{
  "session_id": "string (required)",
  "type": "string (required)",
  "resource_id": "string (required)",
  "description": "string (required)",
  "severity": "critical | warning | info (required)",
  "evidence": "object (optional)"
}
```

**Input Example**:
```json
{
  "session_id": "session-abc123",
  "type": "root_cause",
  "resource_id": "Deployment/production/order-service",
  "description": "Root cause identified: Memory leak in v2.3.5. Rollback to v2.3.4 recommended.",
  "severity": "critical",
  "evidence": {
    "deployment_version": "v2.3.5",
    "deployment_time": "2024-11-03T14:15:00Z",
    "oom_count": 3,
    "memory_trend": "increasing",
    "rollback_target": "v2.3.4",
    "confidence": 0.95
  }
}
```

**Output Schema**:
```json
{
  "finding_id": "string",
  "detection_method": "agent_recorded"
}
```

**Output Example**:
```json
{
  "finding_id": "finding-root-cause-xyz",
  "detection_method": "agent_recorded"
}
```

**Side Effects**:
- Creates `Finding` node with `detection_method: "agent_recorded"`
- Links to specified resource
- Links to AgentSession
- Emits notification: `agent_session/finding_discovered`

**Finding Types**:
- `root_cause`: Identified root cause
- `contributing_factor`: Contributing issue
- `recommendation`: Suggested action
- `observation`: Notable observation
- Custom types allowed

**Errors**:
- `SESSION_NOT_FOUND`: Invalid session_id
- `RESOURCE_NOT_FOUND`: Specified resource doesn't exist
- `INVALID_SEVERITY`: Must be critical/warning/info

---

### 5. record_recommendation

**Purpose**: Record an actionable recommendation for resolving issues or improving the system based on investigation findings.

**Input Schema**:
```json
{
  "session_id": "string (required)",
  "type": "root_cause_fix | preventive_action | optimization | monitoring_improvement | cleanup (required)",
  "priority": "critical | high | medium | low (required)",
  "title": "string (required)",
  "description": "string (required)",
  "rationale": "string (required)",
  "related_findings": "array of finding IDs (optional)",
  "action_items": "array of strings (required)",
  "estimated_effort": "string (optional)",
  "automation_hint": "string (optional)",
  "tags": "array of strings (optional)",
  "metadata": "object (optional)"
}
```

**Input Example**:
```json
{
  "session_id": "session-abc123",
  "type": "root_cause_fix",
  "priority": "critical",
  "title": "Increase memory limit for order-service deployment",
  "description": "The order-service pods are being OOMKilled due to insufficient memory allocation. Current limit of 512Mi is too low based on observed usage patterns.",
  "rationale": "Memory usage analysis shows consistent growth pattern reaching 500-520Mi before OOMKill. The v2.3.5 deployment increased baseline memory usage by 30% compared to v2.3.4.",
  "related_findings": ["finding-oomkilled-1", "finding-oomkilled-2", "finding-memory-trend-3"],
  "action_items": [
    "Update deployment manifest to increase memory limit from 512Mi to 1Gi",
    "Update memory request to 768Mi for better pod scheduling",
    "Monitor memory usage for 24 hours after deployment",
    "Consider implementing memory profiling if issue persists"
  ],
  "estimated_effort": "15 minutes",
  "automation_hint": "kubectl set resources deployment order-service -n production --limits=memory=1Gi --requests=memory=768Mi",
  "tags": ["memory", "oomkilled", "production", "critical-path"],
  "metadata": {
    "deployment": "order-service",
    "namespace": "production",
    "current_limit": "512Mi",
    "recommended_limit": "1Gi",
    "confidence": 0.95
  }
}
```

**Output Schema**:
```json
{
  "recommendation_id": "string",
  "priority": "string",
  "type": "string",
  "created_at": "ISO8601 timestamp"
}
```

**Output Example**:
```json
{
  "recommendation_id": "recommendation-memory-fix-xyz",
  "priority": "critical",
  "type": "root_cause_fix",
  "created_at": "2024-11-03T14:45:00Z"
}
```

**Side Effects**:
- Creates `Recommendation` node in Neo4j
- Links to AgentSession via `HAS_RECOMMENDATION` relationship
- Links to related Finding nodes via `BASED_ON` relationships
- Emits notification: `agent_session/recommendation_recorded`

**Recommendation Types**:
- `root_cause_fix`: Direct fixes for identified root cause (highest priority typically)
- `preventive_action`: Steps to prevent similar issues in the future
- `optimization`: Performance or efficiency improvements discovered
- `monitoring_improvement`: Enhanced observability to detect similar issues earlier
- `cleanup`: Technical debt or cleanup tasks discovered

**Priority Guidelines**:
- `critical`: Immediate action required, system degraded or at risk
- `high`: Important, should be addressed soon (within hours/days)
- `medium`: Worthwhile improvement, schedule appropriately (within weeks)
- `low`: Nice to have, can be deferred (backlog)

**Best Practices**:
1. Always link recommendations to related findings using `related_findings` array
2. Provide specific, actionable steps in `action_items`
3. Include automation hints (kubectl commands, scripts) when possible
4. Estimate effort realistically to help prioritization
5. Prioritize root cause fixes as "critical" or "high"
6. Use tags for categorization and filtering
7. Record recommendations near the end of investigation, before calling `complete_agent_session`

**Errors**:
- `SESSION_NOT_FOUND`: Invalid session_id
- `INVALID_TYPE`: Type must be one of the enum values
- `INVALID_PRIORITY`: Priority must be critical/high/medium/low
- `MISSING_ACTION_ITEMS`: At least one action item required

---

### 6. spawn_investigation

**Purpose**: Launch a metrics investigation for a specific resource (links to existing Investigation system).

**Input Schema**:
```json
{
  "session_id": "string (required)",
  "hypothesis_id": "string (optional)",
  "resource_type": "string (required)",
  "resource_id": "string (required)",
  "symptom": "string (required)",
  "lookback_minutes": "number (required)"
}
```

**Input Example**:
```json
{
  "session_id": "session-abc123",
  "hypothesis_id": "hypothesis-xyz",
  "resource_type": "Deployment",
  "resource_id": "Deployment/production/order-service",
  "symptom": "OOMKilled",
  "lookback_minutes": 30
}
```

**Output Schema**:
```json
{
  "investigation_id": "string",
  "status": "active",
  "metrics_being_collected": "array of metric names"
}
```

**Output Example**:
```json
{
  "investigation_id": "inv-memory-123",
  "status": "active",
  "metrics_being_collected": [
    "container_memory_usage_bytes",
    "container_memory_working_set_bytes",
    "container_oom_events_total"
  ]
}
```

**Side Effects**:
- Calls `start_investigation` tool internally
- Pulls metrics from Prometheus (if configured)
- Creates `Investigation` node
- Links AgentSession → Investigation (`SPAWNED_INVESTIGATION`)
- If hypothesis_id provided: Links Hypothesis → Investigation (`TRIGGERED_INVESTIGATION`)
- Emits notification: `agent_session/investigation_spawned`

**Supported Resource Types**:
- `Pod`
- `Deployment`
- `StatefulSet`
- `DaemonSet`
- `Node`
- `Service`

**Errors**:
- `SESSION_NOT_FOUND`: Invalid session_id
- `PROMETHEUS_NOT_CONFIGURED`: Prometheus URL not set
- `INVALID_RESOURCE_TYPE`: Unsupported resource type
- `METRICS_UNAVAILABLE`: No metrics for specified resource

---

### 7. complete_agent_session

**Purpose**: Mark investigation session as complete and finalize blast zone snapshot.

**Input Schema**:
```json
{
  "session_id": "string (required)",
  "summary": "string (optional)"
}
```

**Input Example**:
```json
{
  "session_id": "session-abc123",
  "summary": "Identified memory leak in v2.3.5 deployment causing OOMKilled pods. Rollback to v2.3.4 recommended. Investigation completed in 15 minutes with 4 findings discovered."
}
```

**Output Schema**:
```json
{
  "session_id": "string",
  "status": "completed",
  "duration_minutes": "number",
  "query_count": "number",
  "finding_count": "number",
  "blast_zone_final": "BlastZoneSnapshot object"
}
```

**Output Example**:
```json
{
  "session_id": "session-abc123",
  "status": "completed",
  "duration_minutes": 15,
  "query_count": 3,
  "finding_count": 4,
  "blast_zone_final": {
    "affected_count": 5,
    "impact_radius": 3,
    "nodes": [...],
    "edges": [...]
  }
}
```

**Side Effects**:
- Updates AgentSession status to "completed"
- Records completion timestamp
- Finalizes blast zone snapshot
- Completes any linked Investigation sessions
- Stores summary
- Emits notification: `agent_session/completed`

**Errors**:
- `SESSION_NOT_FOUND`: Invalid session_id
- `ALREADY_COMPLETED`: Session already marked complete

---

## Dashboard Tools (Read-Only Operations)

### 1. get_active_sessions

**Purpose**: List all currently active investigation sessions.

**Input**: None (empty object `{}`)

**Output Schema**:
```json
[
  {
    "id": "string",
    "initial_symptom": "string",
    "created_at": "ISO8601 timestamp",
    "query_count": "number",
    "finding_count": "number",
    "current_stage": "number"
  }
]
```

**Output Example**:
```json
[
  {
    "id": "session-abc123",
    "initial_symptom": "Order service returning 503 errors",
    "created_at": "2024-11-03T14:30:00Z",
    "query_count": 5,
    "finding_count": 3,
    "current_stage": 2
  },
  {
    "id": "session-def456",
    "initial_symptom": "Payment service high latency",
    "created_at": "2024-11-03T15:00:00Z",
    "query_count": 2,
    "finding_count": 1,
    "current_stage": 1
  }
]
```

**Usage**: Dashboard polls this every 5 seconds to discover new sessions.

---

### 2. get_session_details

**Purpose**: Get complete state snapshot for a specific session.

**Input Schema**:
```json
{
  "session_id": "string (required)"
}
```

**Output Schema**:
```json
{
  "session": "AgentSession object",
  "hypotheses": "array of Hypothesis objects",
  "queries": "array of QueryExecution objects",
  "findings": "array of Finding objects",
  "investigations": "array of investigation IDs",
  "current_hypothesis": "Hypothesis object (optional)"
}
```

**Output Example**:
```json
{
  "session": {
    "id": "session-abc123",
    "initial_symptom": "Order service returning 503 errors",
    "status": "active",
    "created_at": "2024-11-03T14:30:00Z",
    "current_stage": 2,
    "query_count": 5,
    "finding_count": 3
  },
  "hypotheses": [
    {
      "id": "hypothesis-1",
      "stage": 1,
      "text": "Service may be failing due to backend pod issues",
      "status": "superseded",
      "created_at": "2024-11-03T14:30:05Z"
    },
    {
      "id": "hypothesis-2",
      "stage": 2,
      "text": "Pods crashing due to OOMKilled events",
      "status": "active",
      "created_at": "2024-11-03T14:32:30Z"
    }
  ],
  "queries": [...],
  "findings": [...],
  "investigations": ["inv-memory-123"],
  "current_hypothesis": {
    "id": "hypothesis-2",
    "stage": 2,
    "text": "Pods crashing due to OOMKilled events",
    "status": "active",
    "created_at": "2024-11-03T14:32:30Z"
  }
}
```

---

### 3. get_blast_zone

**Purpose**: Get current blast zone graph for a session (dynamically calculated).

**Input Schema**:
```json
{
  "session_id": "string (required)"
}
```

**Output Schema**:
```json
{
  "session_id": "string",
  "timestamp": "ISO8601 timestamp",
  "trigger_event": "string",
  "nodes": "array of BlastZoneNode objects",
  "edges": "array of BlastZoneEdge objects",
  "impact_radius": "number",
  "affected_count": "number"
}
```

**Output Example**:
```json
{
  "session_id": "session-abc123",
  "timestamp": "2024-11-03T14:35:00Z",
  "trigger_event": "hypothesis_update_stage_2",
  "nodes": [
    {
      "id": "Pod/production/order-pod-1",
      "label": "order-pod-1",
      "type": "Pod",
      "status": "failed",
      "properties": {"restartCount": 5}
    },
    {
      "id": "Service/production/order-service",
      "label": "order-service",
      "type": "Service",
      "status": "degraded",
      "properties": {"type": "ClusterIP"}
    }
  ],
  "edges": [
    {
      "source": "Service/production/order-service",
      "target": "Pod/production/order-pod-1",
      "type": "SELECTS_PODS",
      "status": "ok"
    }
  ],
  "impact_radius": 3,
  "affected_count": 5
}
```

**Node Status Values**:
- `failed`: Direct finding affects this resource
- `degraded`: Indirect failure (failed dependency)
- `healthy`: In blast zone but functioning

**Edge Status Values**:
- `failing`: Failed relationship (e.g., FAILED_CALL_TO)
- `ok`: Normal relationship

---

### 4. get_session_timeline

**Purpose**: Get chronological timeline of all events in a session.

**Input Schema**:
```json
{
  "session_id": "string (required)"
}
```

**Output Schema**:
```json
[
  {
    "timestamp": "ISO8601 timestamp",
    "type": "hypothesis | query | finding | investigation",
    "data": "object"
  }
]
```

**Output Example**:
```json
[
  {
    "timestamp": "2024-11-03T14:30:00Z",
    "type": "session",
    "data": {
      "event": "created",
      "symptom": "Order service returning 503 errors"
    }
  },
  {
    "timestamp": "2024-11-03T14:30:05Z",
    "type": "hypothesis",
    "data": {
      "stage": 1,
      "text": "Service may be failing due to backend pod issues"
    }
  },
  {
    "timestamp": "2024-11-03T14:30:15Z",
    "type": "query",
    "data": {
      "query_id": "query-1",
      "reasoning": "Check pod health",
      "result_count": 5
    }
  },
  {
    "timestamp": "2024-11-03T14:30:17Z",
    "type": "finding",
    "data": {
      "finding_id": "finding-1",
      "type": "unhealthy_pod",
      "severity": "critical",
      "description": "Pod not running: CrashLoopBackOff"
    }
  }
]
```

---

## Error Handling

All tools follow consistent error format:

```json
{
  "error": {
    "code": "ERROR_CODE",
    "message": "Human-readable error message",
    "details": "object (optional)"
  }
}
```

**Common Error Codes**:
- `SESSION_NOT_FOUND`: Invalid or non-existent session ID
- `INVALID_INPUT`: Missing or malformed input parameters
- `DATABASE_ERROR`: Neo4j connection or query error
- `PROMETHEUS_NOT_CONFIGURED`: Prometheus URL not set (for spawn_investigation)
- `QUERY_TIMEOUT`: Query exceeded execution time limit
- `ALREADY_COMPLETED`: Operation on completed session
- `RESOURCE_NOT_FOUND`: Referenced K8s resource doesn't exist

## Rate Limiting

- Agent tools: 100 requests/minute per session
- Dashboard tools: 1000 requests/minute (for polling)

## Versioning

MCP tools version: `1.0.0`

Breaking changes will increment major version and require Cursor MCP config update.

## See Also

- [Agent Investigation Workflow](../../guides/investigations/workflow.md)
- [Graph Schema Reference](../../reference/graph-schema.md)
- [Dashboard User Guide](dashboard.md)

