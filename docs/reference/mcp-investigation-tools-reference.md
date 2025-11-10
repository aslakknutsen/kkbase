# MCP Investigation Tools Quick Reference

Quick reference for the three investigation tools available in the kkbase MCP server.

## Prerequisites

```bash
export PROMETHEUS_URL="http://prometheus:9090"
```

## Tool: start_investigation

Start a new RCA investigation and pull metrics from Prometheus.

### Request

```json
{
  "tool": "start_investigation",
  "arguments": {
    "resource_type": "Pod",
    "resource_id": "Pod/prod/api-gateway-xyz",
    "symptom": "OOMKilled",
    "lookback_minutes": 30
  }
}
```

### Response

```json
{
  "investigation_id": "inv-20231031-143022-abc123",
  "status": "active",
  "resource_type": "Pod",
  "resource_id": "Pod/prod/api-gateway-xyz",
  "symptom": "OOMKilled",
  "metrics_collected": 450,
  "message": "Investigation started successfully..."
}
```

### Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `resource_type` | string | Yes | Pod, Service, Node, Deployment, StatefulSet, DaemonSet |
| `resource_id` | string | Yes | Full ID like "Pod/namespace/name" or "Node/name" |
| `symptom` | string | Yes | OOMKilled, CrashLoopBackOff, HighLatency, HighErrorRate, HighCPU, NodeNotReady |
| `lookback_minutes` | int | No | 5-120, default 15 |

### Common Symptoms

- **OOMKilled**: Container killed due to memory limit
- **CrashLoopBackOff**: Repeated container crashes
- **HighLatency**: Slow response times
- **HighErrorRate**: Elevated error rates
- **HighCPU**: CPU saturation/throttling
- **NodeNotReady**: Node unavailable

---

## Tool: complete_investigation

Complete an investigation and purge all metrics.

### Request

```json
{
  "tool": "complete_investigation",
  "arguments": {
    "investigation_id": "inv-20231031-143022-abc123"
  }
}
```

### Response

```json
{
  "investigation_id": "inv-20231031-143022-abc123",
  "status": "completed",
  "metrics_purged": 450,
  "message": "Investigation completed successfully. Purged 450 metric data points."
}
```

### When to Call

- After completing RCA analysis
- When you have identified root cause
- Always call to prevent metric accumulation

---

## Tool: get_investigation_status

Query current investigation status and details.

### Request

```json
{
  "tool": "get_investigation_status",
  "arguments": {
    "investigation_id": "inv-20231031-143022-abc123"
  }
}
```

### Response

```json
{
  "investigation_id": "inv-20231031-143022-abc123",
  "status": "active",
  "resource_type": "Pod",
  "resource_id": "Pod/prod/api-gateway-xyz",
  "symptom": "OOMKilled",
  "start_time": "2023-10-31T14:30:22Z",
  "lookback_duration": "30m0s"
}
```

### Status Values

- **active**: Investigation in progress
- **completed**: Investigation finished, metrics purged
- **abandoned**: Investigation abandoned (error/timeout)

---

## Query Patterns

### Find All Metrics for Investigation

```cypher
MATCH (i:Investigation {id: $investigation_id})-[:HAS_METRIC_EVIDENCE]->(m:Metric)
RETURN m.name, m.value, m.timestamp, m.labels
ORDER BY m.timestamp
```

### Memory Usage Timeline

```cypher
MATCH (i:Investigation {id: $inv_id})-[:HAS_METRIC_EVIDENCE]->(m:Metric)
WHERE m.name = 'container_memory_usage_bytes'
RETURN m.timestamp, m.value
ORDER BY m.timestamp
```

### CPU Usage by Container

```cypher
MATCH (i:Investigation {id: $inv_id})-[:HAS_METRIC_EVIDENCE]->(m:Metric)
WHERE m.name = 'container_cpu_usage_seconds_total'
RETURN m.labels.container, avg(m.value) as avg_cpu
ORDER BY avg_cpu DESC
```

### Compare Usage vs Limits

```cypher
MATCH (i:Investigation {id: $inv_id})-[:HAS_METRIC_EVIDENCE]->(m:Metric)
WHERE m.name IN ['container_memory_usage_bytes', 'container_memory_limit_bytes']
WITH m.name as metric, m.timestamp as time, m.value as value
ORDER BY time
RETURN time, 
       max(CASE WHEN metric = 'container_memory_usage_bytes' THEN value END) as usage,
       max(CASE WHEN metric = 'container_memory_limit_bytes' THEN value END) as limit
```

### Find "Noisy Neighbor" on Same Node

```cypher
MATCH (i:Investigation {id: $inv_id})-[:INVESTIGATING]->(p:Pod)-[:RUNS_ON]->(n:Node)
MATCH (n)<-[:RUNS_ON]-(other:Pod)
WHERE other <> p
MATCH (i)-[:HAS_METRIC_EVIDENCE]->(m:Metric)
WHERE m.labels.pod = other.name AND m.name = 'container_cpu_usage_seconds_total'
RETURN other.name, avg(m.value) as avg_cpu
ORDER BY avg_cpu DESC
LIMIT 5
```

---

## Complete Workflow Example

```javascript
// 1. Start investigation
const inv = await mcp.call_tool("start_investigation", {
  resource_type: "Pod",
  resource_id: "Pod/prod/api-gateway-xyz",
  symptom: "HighLatency",
  lookback_minutes: 30
});

console.log(`Investigation ${inv.investigation_id} started`);
console.log(`Collected ${inv.metrics_collected} metrics`);

// 2. Query metrics
const latency = await mcp.call_tool("query", {
  query: `
    MATCH (i:Investigation {id: $inv_id})-[:HAS_METRIC_EVIDENCE]->(m:Metric)
    WHERE m.name = 'http_request_duration_seconds'
    RETURN m.timestamp, m.value
    ORDER BY m.timestamp
  `,
  params: { inv_id: inv.investigation_id }
});

// 3. Analyze results
const avgLatency = latency.results.reduce((sum, r) => sum + r.value, 0) / latency.count;
console.log(`Average latency: ${avgLatency}s`);

// 4. Check correlations (CPU contention?)
const cpu = await mcp.call_tool("query", {
  query: `
    MATCH (i:Investigation {id: $inv_id})-[:INVESTIGATING]->(p:Pod)
    MATCH (i)-[:HAS_METRIC_EVIDENCE]->(m:Metric)
    WHERE m.name = 'container_cpu_usage_seconds_total'
    RETURN avg(m.value) as avg_cpu
  `,
  params: { inv_id: inv.investigation_id }
});

if (cpu.results[0].avg_cpu > 0.8) {
  console.log("Root cause: CPU throttling causing high latency");
}

// 5. Complete investigation
const complete = await mcp.call_tool("complete_investigation", {
  investigation_id: inv.investigation_id
});

console.log(`Cleaned up ${complete.metrics_purged} metrics`);
```

---

## Error Handling

### Tool Not Available

```
Error: tool 'start_investigation' not found
```

**Solution**: Set `PROMETHEUS_URL` environment variable and restart MCP server.

### Investigation Not Found

```
Error: investigation not found: inv-xyz
```

**Solution**: Check investigation ID or start a new investigation.

### No Metrics Collected

```
{
  "investigation_id": "inv-xyz",
  "metrics_collected": 0
}
```

**Causes**:
- Prometheus doesn't have data for this resource
- Time range too old or too recent
- Resource labels don't match Prometheus labels

**Solutions**:
- Verify resource exists in Prometheus
- Try different `lookback_minutes`
- Check Prometheus label format

### Prometheus Unreachable

```
Error: failed to query Prometheus: connection refused
```

**Solutions**:
- Verify `PROMETHEUS_URL` is correct
- Check network connectivity
- Verify Prometheus is running

---

## Best Practices

### 1. Always Complete Investigations

```
✅ Good: start → analyze → complete
❌ Bad: start → analyze → [forget to complete]
```

Uncompleted investigations leave metrics in the graph.

### 2. Use Appropriate Lookback Windows

- Recent issues: 5-15 minutes
- Intermittent issues: 30-60 minutes
- Historical analysis: 60-120 minutes (max)

### 3. Match Symptom to Issue

Use specific symptoms for better metric selection:

```
✅ Good: symptom: "HighLatency"
❌ Bad: symptom: "something wrong"
```

### 4. Query Incrementally

Start with overview queries, then drill down:

```
1. All metrics → What metrics do we have?
2. Memory timeline → Is memory the issue?
3. Memory vs limit → Did we exceed limits?
4. Other pods on node → Is it "noisy neighbor"?
```

### 5. Check Investigation Status

Before querying, verify investigation is active:

```javascript
const status = await mcp.call_tool("get_investigation_status", {
  investigation_id: inv_id
});

if (status.status !== "active") {
  console.log("Investigation no longer active");
}
```

---

## See Also

- [Investigation Workflow Guide](../guides/investigations/workflow.md) - Detailed guide
- [Metrics RCA Queries](./metrics-rca-queries.md) - Comprehensive query examples
- [MCP Server Documentation](../services/mcp-server/README.md) - General MCP usage

