# Investigation Tools for AI Agents

This guide explains how to use the MCP investigation tools for root cause analysis (RCA) with metrics.

## Overview

The investigation tools provide a workflow for AI agents to:
1. **Start** an investigation for a specific Kubernetes resource exhibiting a symptom
2. **Query** metrics and correlations in the knowledge graph during investigation
3. **Complete** the investigation to cleanup temporary metric data

These tools implement a hybrid metrics approach where metrics are pulled on-demand from Prometheus during active investigations and stored temporarily in the graph for correlation with Kubernetes resources.

## Prerequisites

- MCP server must be configured with `PROMETHEUS_URL` environment variable
- Target Kubernetes resources must exist in the knowledge graph
- Prometheus must be accessible and contain metrics for the resources

## Tool: `start_investigation`

Starts a new RCA investigation and pulls relevant metrics from Prometheus.

### Input

```json
{
  "resource_type": "Pod",
  "resource_id": "Pod/prod/api-gateway-xyz",
  "symptom": "OOMKilled",
  "lookback_minutes": 15
}
```

**Parameters:**
- `resource_type` (required): Type of resource - one of: `Pod`, `Service`, `Node`, `Deployment`, `StatefulSet`, `DaemonSet`
- `resource_id` (required): Full resource identifier (format: `{Type}/{Namespace}/{Name}` or `{Type}/{Name}` for cluster-scoped)
- `symptom` (required): Symptom being investigated, such as:
  - `OOMKilled` - Out of memory kills
  - `CrashLoopBackOff` - Repeated crashes
  - `HighLatency` - High response times
  - `HighErrorRate` - Elevated error rates
  - `NodeNotReady` - Node unavailability
  - `HighCPU` - CPU saturation
- `lookback_minutes` (optional): How far back to pull metrics (default: 15, range: 5-120)

### Output

```json
{
  "investigation_id": "inv-20231031-143022-abc123",
  "status": "active",
  "resource_type": "Pod",
  "resource_id": "Pod/prod/api-gateway-xyz",
  "symptom": "OOMKilled",
  "metrics_collected": 450,
  "message": "Investigation started successfully. Use investigation ID 'inv-20231031-143022-abc123' to query metrics and complete investigation."
}
```

### What Happens

1. Creates an `Investigation` node in the graph
2. Selects relevant metrics based on the symptom:
   - **OOMKilled/CrashLoopBackOff**: Memory and CPU metrics
   - **HighLatency**: Latency and CPU metrics
   - **HighErrorRate**: Error rate and network metrics
   - **NodeNotReady**: Node-level resource metrics
   - **Default**: Core resource metrics
3. Queries Prometheus for the selected metrics over the lookback period
4. Correlates each metric to the appropriate Kubernetes resource using labels
5. Stores metrics temporarily in the graph with the `investigation_id`
6. Returns an investigation ID for subsequent queries

### Example Usage

**Scenario: Pod experiencing OOM kills**

```
Agent: I notice Pod "api-gateway-xyz" in namespace "prod" was OOMKilled. 
       Let me start an investigation.

Call: start_investigation({
  "resource_type": "Pod",
  "resource_id": "Pod/prod/api-gateway-xyz",
  "symptom": "OOMKilled",
  "lookback_minutes": 30
})

Response: Investigation inv-20231031-143022-abc123 started. 
          Collected 450 metric data points.
```

## Querying During Investigation

Once an investigation is started, use the `query` tool to explore the metrics and correlations:

### Finding Metrics for the Investigation

```cypher
// Get all metrics collected for this investigation
MATCH (i:Investigation {id: 'inv-20231031-143022-abc123'})-[:HAS_METRIC_EVIDENCE]->(m:Metric)
RETURN m.name, m.value, m.timestamp, m.labels
ORDER BY m.timestamp
```

### Memory Trend Analysis

```cypher
// Analyze memory usage trend for the investigated Pod
MATCH (i:Investigation {id: 'inv-20231031-143022-abc123'})-[:INVESTIGATING]->(p:Pod)
MATCH (i)-[:HAS_METRIC_EVIDENCE]->(m:Metric {name: 'container_memory_usage_bytes'})
WHERE m.labels.pod = p.name AND m.labels.namespace = p.namespace
RETURN m.timestamp, m.value
ORDER BY m.timestamp
```

### Comparing Against Limits

```cypher
// Compare memory usage against configured limits
MATCH (i:Investigation {id: 'inv-20231031-143022-abc123'})-[:INVESTIGATING]->(p:Pod)
MATCH (i)-[:HAS_METRIC_EVIDENCE]->(m:Metric)
WHERE m.name IN ['container_memory_usage_bytes', 'container_memory_limit_bytes']
RETURN m.name, m.value, m.timestamp
ORDER BY m.timestamp
```

### Cross-Resource Correlation

```cypher
// Find "noisy neighbor" issues on the same node
MATCH (i:Investigation {id: 'inv-20231031-143022-abc123'})-[:INVESTIGATING]->(p:Pod)-[:RUNS_ON]->(n:Node)
MATCH (n)<-[:RUNS_ON]-(other:Pod)
MATCH (i)-[:HAS_METRIC_EVIDENCE]->(m:Metric)
WHERE m.labels.pod = other.name 
  AND m.labels.namespace = other.namespace
  AND m.name = 'container_cpu_usage_seconds_total'
RETURN other.name, avg(m.value) as avg_cpu
ORDER BY avg_cpu DESC
```

See [Metrics RCA Queries](../reference/metrics-rca-queries.md) for comprehensive query patterns.

## Tool: `complete_investigation`

Completes an investigation and purges all associated metrics from the graph.

### Input

```json
{
  "investigation_id": "inv-20231031-143022-abc123"
}
```

### Output

```json
{
  "investigation_id": "inv-20231031-143022-abc123",
  "status": "completed",
  "metrics_purged": 450,
  "message": "Investigation 'inv-20231031-143022-abc123' completed successfully. Purged 450 metric data points."
}
```

### What Happens

1. Updates the `Investigation` node status to "completed"
2. Deletes all `Metric` nodes with the associated `investigation_id`
3. Removes all `HAS_METRIC_EVIDENCE` relationships
4. Returns the count of purged metrics

### When to Call

- After completing your RCA analysis
- When you have sufficient evidence for the root cause
- Always call this to prevent metric accumulation in the graph

### Example Usage

```
Agent: Based on the metrics, I've identified the root cause as insufficient memory limits.
       The Pod's memory usage climbed to 2GB while the limit was set to 1.5GB.
       Let me complete this investigation.

Call: complete_investigation({
  "investigation_id": "inv-20231031-143022-abc123"
})

Response: Investigation completed. Purged 450 metric data points.
```

## Tool: `get_investigation_status`

Retrieves the current status and details of an investigation.

### Input

```json
{
  "investigation_id": "inv-20231031-143022-abc123"
}
```

### Output

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

### When to Use

- To verify an investigation is still active
- To retrieve investigation details after resuming work
- To check if metrics have been cleaned up

## Complete Investigation Workflow

Here's a complete example workflow for an AI agent:

### Step 1: Detect Issue

```cypher
// Agent detects a Pod with recent restarts
MATCH (p:Pod {name: 'api-gateway-xyz', namespace: 'prod'})
WHERE p.restart_count > 5
RETURN p.name, p.restart_count, p.status
```

### Step 2: Start Investigation

```json
start_investigation({
  "resource_type": "Pod",
  "resource_id": "Pod/prod/api-gateway-xyz",
  "symptom": "CrashLoopBackOff",
  "lookback_minutes": 30
})
```

### Step 3: Analyze Metrics

```cypher
// Check memory trend
MATCH (i:Investigation {id: 'inv-xyz'})-[:HAS_METRIC_EVIDENCE]->(m:Metric)
WHERE m.name = 'container_memory_usage_bytes'
RETURN m.timestamp, m.value
ORDER BY m.timestamp

// Check CPU usage
MATCH (i:Investigation {id: 'inv-xyz'})-[:HAS_METRIC_EVIDENCE]->(m:Metric)
WHERE m.name = 'container_cpu_usage_seconds_total'
RETURN avg(m.value) as avg_cpu

// Check for errors
MATCH (i:Investigation {id: 'inv-xyz'})-[:HAS_METRIC_EVIDENCE]->(m:Metric)
WHERE m.name = 'http_requests_total' AND m.labels.status >= '500'
RETURN count(m) as error_count
```

### Step 4: Correlate with K8s Resources

```cypher
// Check resource limits
MATCH (i:Investigation {id: 'inv-xyz'})-[:INVESTIGATING]->(p:Pod)
RETURN p.memory_request, p.memory_limit, p.cpu_request, p.cpu_limit

// Check node resources
MATCH (i:Investigation {id: 'inv-xyz'})-[:INVESTIGATING]->(p:Pod)-[:RUNS_ON]->(n:Node)
RETURN n.name, n.status, n.memory_capacity
```

### Step 5: Complete Investigation

```json
complete_investigation({
  "investigation_id": "inv-xyz"
})
```

## Best Practices

### 1. Always Complete Investigations

Metrics are stored temporarily and must be cleaned up:

```
✅ Good:
start_investigation() → query metrics → analyze → complete_investigation()

❌ Bad:
start_investigation() → query metrics → [forget to cleanup]
```

### 2. Use Appropriate Lookback Windows

- **Recent issues**: 5-15 minutes
- **Intermittent issues**: 30-60 minutes  
- **Historical analysis**: 60-120 minutes (max)

Longer windows = more metrics = slower queries.

### 3. Symptom-Specific Investigations

Let the system select appropriate metrics by providing accurate symptoms:

```
✅ Good: symptom: "HighLatency"
❌ Bad: symptom: "Something wrong"
```

### 4. Combine with Graph Traversal

Don't just look at metrics - correlate with the graph:

```cypher
// Find related services
MATCH (i:Investigation)-[:INVESTIGATING]->(p:Pod)<-[:TARGETS]-(s:Service)
MATCH (s)-[:ROUTES_TO]->(upstream:Service)
RETURN upstream.name
```

### 5. Handle Missing Prometheus

If `PROMETHEUS_URL` is not configured, these tools won't be available:

```
Error: investigation tools are not available (Prometheus not configured)

Solution: Set PROMETHEUS_URL environment variable when starting the MCP server
```

## Configuration

### Environment Variables

```bash
# Required for investigation tools
export PROMETHEUS_URL="http://prometheus.monitoring.svc:9090"

# MCP Server configuration
export NEO4J_URI="bolt://localhost:7687"
export NEO4J_USERNAME="neo4j"
export NEO4J_PASSWORD="password"
export MCP_PORT="8080"
```

### Starting the MCP Server

```bash
# With metrics support
PROMETHEUS_URL=http://prometheus:9090 ./mcp-server

# Without metrics support (investigation tools disabled)
./mcp-server
```

The server logs will indicate if investigation tools are available:

```
INFO  Metrics integration enabled - investigation tools available
INFO  Registered MCP tools  ["query", "structure", "start_investigation", "complete_investigation", "get_investigation_status"]
```

## Troubleshooting

### "Investigation not found"

You're querying a non-existent or already completed investigation.

**Solution**: Call `start_investigation` again or verify the investigation ID.

### "Failed to query Prometheus"

Prometheus is unreachable or the query is invalid.

**Solution**: 
- Verify `PROMETHEUS_URL` is correct
- Check network connectivity
- Verify Prometheus has metrics for the resource

### "Resource not found in graph"

The Kubernetes resource you're investigating doesn't exist in the knowledge graph.

**Solution**:
- Verify the resource exists: `MATCH (n {id: 'Pod/prod/api-gateway-xyz'}) RETURN n`
- Ensure the watcher has discovered the resource
- Check for typos in the resource_id

### No metrics collected

`metrics_collected: 0` in the response.

**Possible causes**:
- Prometheus doesn't have metrics for this resource
- Label mismatch (metric labels don't match resource names)
- Lookback window too short or too far in the past

**Solution**:
- Verify metrics exist in Prometheus directly
- Try a different lookback window
- Check metric labels match K8s resource names

## See Also

- [Metrics Investigation Workflow](./metrics-investigation.md) - Detailed investigation patterns
- [Metrics RCA Queries](../reference/metrics-rca-queries.md) - Comprehensive Cypher query examples
- [Querying the Knowledge Graph](./querying.md) - General query guide
- [MCP Server Deployment](./mcp-deployment-options.md) - Deployment configurations

