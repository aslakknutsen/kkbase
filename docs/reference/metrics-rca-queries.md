# Metrics-Based Root Cause Analysis Queries

This document provides comprehensive Cypher query examples for performing root cause analysis (RCA) using metrics in the knowledge graph. These queries demonstrate how to leverage the investigation-scoped metrics integration to diagnose common Kubernetes issues.

## Table of Contents

1. [OOMKilled Diagnosis](#oomkilled-diagnosis)
2. [High Latency Investigation](#high-latency-investigation)
3. [Service Error Rate Analysis](#service-error-rate-analysis)
4. [Node Pressure Detection](#node-pressure-detection)
5. [Network Issues](#network-issues)
6. [Container Resource Exhaustion](#container-resource-exhaustion)

---

## OOMKilled Diagnosis

### Symptom
Pod is in `CrashLoopBackOff` status with events indicating `OOMKilled` (Out of Memory).

### Relevant Metrics
- `container_memory_usage_bytes` - Current memory usage
- `container_memory_working_set_bytes` - Memory working set (used for OOM decisions)
- `container_cpu_usage_seconds_total` - CPU usage (to rule out CPU issues)

### Query 1: Find Memory Usage Leading to Crash

```cypher
// Find investigation for the pod
MATCH (inv:Investigation)-[:INVESTIGATING]->(p:Pod {name: "my-crashing-pod", namespace: "prod"})
WHERE inv.symptom = "OOMKilled" OR inv.symptom = "CrashLoopBackOff"

// Get memory metrics from the investigation
MATCH (inv)-[:HAS_METRIC_EVIDENCE]->(m:Metric)
WHERE m.name = "container_memory_usage_bytes"
  AND m.label_pod = p.name
  AND m.label_namespace = p.namespace

// Get pod's memory limit
WITH p, m, p.memory_limit as limit
ORDER BY datetime(m.timestamp) ASC

RETURN 
  datetime(m.timestamp) as time,
  m.value as memory_bytes,
  limit,
  (m.value / toFloat(limit)) * 100 as memory_percent
ORDER BY time ASC
```

**Interpretation:**
- If `memory_percent` climbs steadily to 100% → Memory leak in application
- If `memory_percent` spikes suddenly → Memory-intensive operation or memory leak
- If `memory_bytes` is consistently near limit → Under-provisioned; increase memory limit

### Query 2: Compare Memory Usage vs Memory Limit

```cypher
// Find pod with OOM issue
MATCH (p:Pod {name: "my-crashing-pod", namespace: "prod"})

// Get investigation metrics
MATCH (inv:Investigation)-[:INVESTIGATING]->(p)
MATCH (inv)-[:HAS_METRIC_EVIDENCE]->(m:Metric)
WHERE m.name = "container_memory_working_set_bytes"

// Calculate statistics
WITH p, 
     max(m.value) as max_memory,
     avg(m.value) as avg_memory,
     p.memory_limit as limit

RETURN 
  p.name as pod_name,
  limit as memory_limit,
  max_memory,
  avg_memory,
  (max_memory >= toFloat(limit)) as hit_limit,
  CASE
    WHEN max_memory >= toFloat(limit) THEN "OOM: Memory limit reached"
    WHEN (max_memory / toFloat(limit)) > 0.9 THEN "Warning: Near memory limit"
    ELSE "Normal: Memory within limits"
  END as diagnosis
```

**Next Steps:**
- If hit_limit = true: Check for memory leaks or increase memory limit
- If near limit: Review memory usage patterns and optimize application
- If normal: Look for other issues (check events, logs)

---

## High Latency Investigation

### Symptom
Service experiencing high request latency (p95/p99 above threshold).

### Relevant Metrics
- `http_request_duration_seconds` - Request latency distribution
- `container_cpu_usage_seconds_total` - CPU usage (potential cause)
- `node_cpu_seconds_total` - Node-level CPU (noisy neighbor detection)

### Query 3: Analyze Latency Spike with CPU Correlation

```cypher
// Find service investigation
MATCH (inv:Investigation)-[:INVESTIGATING]->(s:Service {name: "api-service", namespace: "prod"})
WHERE inv.symptom = "HighLatency"

// Get latency metrics
MATCH (inv)-[:HAS_METRIC_EVIDENCE]->(latency:Metric)
WHERE latency.name = "http_request_duration_seconds"

// Get pods for this service and their CPU metrics
MATCH (s)-[:SELECTS]->(p:Pod)
MATCH (inv)-[:HAS_METRIC_EVIDENCE]->(cpu:Metric)
WHERE cpu.name = "container_cpu_usage_seconds_total"
  AND cpu.label_pod = p.name

// Correlate latency with CPU
WITH datetime(latency.timestamp) as time,
     latency.value as latency_seconds,
     cpu.value as cpu_rate,
     p.name as pod_name
ORDER BY time ASC

RETURN 
  time,
  latency_seconds * 1000 as latency_ms,
  cpu_rate * 100 as cpu_percent,
  pod_name,
  CASE
    WHEN cpu_rate > 0.9 THEN "High CPU may cause latency"
    WHEN latency_seconds > 1.0 THEN "High latency without CPU pressure"
    ELSE "Normal"
  END as analysis
ORDER BY time ASC
```

**Interpretation:**
- High latency + High CPU → CPU bottleneck, consider scaling or optimization
- High latency + Normal CPU → Application issue, downstream dependency, or network
- Sudden spike → Check for deployment or configuration change

### Query 4: Identify "Noisy Neighbor" on Node

```cypher
// Find service with high latency
MATCH (inv:Investigation)-[:INVESTIGATING]->(s:Service {name: "api-service"})

// Find pods and the node they're on
MATCH (s)-[:SELECTS]->(p:Pod)-[:SCHEDULED_ON]->(n:Node)

// Get node CPU metrics
MATCH (inv)-[:HAS_METRIC_EVIDENCE]->(nodeCpu:Metric)
WHERE nodeCpu.name = "node_cpu_seconds_total"
  AND nodeCpu.label_node = n.name

// Find all pods on the same node
MATCH (n)<-[:SCHEDULED_ON]-(otherPod:Pod)

// Get CPU usage for all pods on node
MATCH (inv)-[:HAS_METRIC_EVIDENCE]->(podCpu:Metric)
WHERE podCpu.name = "container_cpu_usage_seconds_total"
  AND podCpu.label_pod = otherPod.name

WITH n.name as node,
     nodeCpu.value as node_cpu,
     collect({pod: otherPod.name, cpu: podCpu.value}) as pod_cpus

RETURN 
  node,
  node_cpu * 100 as node_cpu_percent,
  [pod IN pod_cpus WHERE pod.cpu > 0.7 | pod.pod] as high_cpu_pods,
  CASE
    WHEN node_cpu > 0.9 THEN "Node CPU exhausted - noisy neighbor detected"
    ELSE "Node CPU normal"
  END as diagnosis
```

**Next Steps:**
- If noisy neighbor detected: Investigate high-CPU pods, consider pod resource limits
- If node CPU normal: Issue is likely in application or downstream service

---

## Service Error Rate Analysis

### Symptom
Service experiencing elevated error rates (HTTP 5xx, failures).

### Relevant Metrics
- `http_requests_total` - Total requests (filter by status code)
- `container_network_receive_errors_total` - Network receive errors
- `container_network_transmit_errors_total` - Network transmit errors

### Query 5: Trace Errors Through Service Chain

```cypher
// Find service with high error rate
MATCH (inv:Investigation)-[:INVESTIGATING]->(s:Service {name: "frontend"})

// Get error rate metrics
MATCH (inv)-[:HAS_METRIC_EVIDENCE]->(m:Metric)
WHERE m.name = "http_requests_total"

// Follow service dependencies (via observed CALLS relationships)
MATCH (s)-[:CALLS]->(downstream:Service)

// Get error metrics for downstream services too
OPTIONAL MATCH (inv2:Investigation)-[:INVESTIGATING]->(downstream)
OPTIONAL MATCH (inv2)-[:HAS_METRIC_EVIDENCE]->(downstreamMetric:Metric)
WHERE downstreamMetric.name = "http_requests_total"

WITH s.name as service,
     avg(m.value) as error_rate,
     downstream.name as downstream_service,
     avg(downstreamMetric.value) as downstream_error_rate

RETURN 
  service,
  error_rate,
  collect({
    downstream: downstream_service,
    error_rate: downstream_error_rate
  }) as dependencies,
  CASE
    WHEN downstream_error_rate > error_rate THEN downstream_service
    ELSE service
  END as root_cause_candidate
ORDER BY error_rate DESC
```

**Interpretation:**
- If downstream has higher error rate → Issue in downstream service
- If current service has highest error rate → Issue in current service
- Trace backwards to find the root cause

### Query 6: Correlate Errors with Network Issues

```cypher
// Find pod with errors
MATCH (inv:Investigation)-[:INVESTIGATING]->(p:Pod)

// Get network error metrics
MATCH (inv)-[:HAS_METRIC_EVIDENCE]->(netErr:Metric)
WHERE netErr.name IN ["container_network_receive_errors_total", "container_network_transmit_errors_total"]
  AND netErr.label_pod = p.name

// Get application error metrics
MATCH (inv)-[:HAS_METRIC_EVIDENCE]->(appErr:Metric)
WHERE appErr.name = "http_requests_total"
  AND appErr.label_pod = p.name

WITH p.name as pod,
     datetime(netErr.timestamp) as time,
     netErr.value as network_errors,
     appErr.value as app_errors

RETURN 
  pod,
  time,
  network_errors,
  app_errors,
  CASE
    WHEN network_errors > 0 THEN "Network issues detected"
    ELSE "No network issues"
  END as network_status
ORDER BY time ASC
```

**Next Steps:**
- If network errors present: Check network policies, CNI configuration, node networking
- If no network errors: Focus on application logic and downstream dependencies

---

## Node Pressure Detection

### Symptom
Node marked as `NotReady` or experiencing resource pressure.

### Relevant Metrics
- `node_cpu_seconds_total` - Node CPU usage
- `node_memory_MemAvailable_bytes` - Available memory on node
- `node_load1` - 1-minute load average

### Query 7: Analyze Node Resource Utilization

```cypher
// Find investigation for node
MATCH (inv:Investigation)-[:INVESTIGATING]->(n:Node {name: "worker-1"})

// Get node metrics
MATCH (inv)-[:HAS_METRIC_EVIDENCE]->(m:Metric)
WHERE m.label_node = n.name
  AND m.name IN ["node_cpu_seconds_total", "node_memory_MemAvailable_bytes", "node_load1"]

WITH n,
     [metric IN collect(m) WHERE metric.name = "node_cpu_seconds_total" | metric.value][0] as cpu,
     [metric IN collect(m) WHERE metric.name = "node_memory_MemAvailable_bytes" | metric.value][0] as mem_avail,
     [metric IN collect(m) WHERE metric.name = "node_load1" | metric.value][0] as load

RETURN 
  n.name as node,
  cpu * 100 as cpu_percent,
  mem_avail / 1073741824.0 as mem_available_gb,
  load,
  CASE
    WHEN cpu > 0.95 THEN "CPU exhausted"
    WHEN mem_avail < 1073741824 THEN "Memory exhausted (< 1GB available)"
    WHEN load > (n.cpu_count * 2) THEN "High load average"
    ELSE "Normal"
  END as pressure_type
```

**Interpretation:**
- CPU exhausted → Too many CPU-intensive pods, consider node scaling
- Memory exhausted → Memory pressure, eviction likely
- High load → I/O bottleneck or too many processes

### Query 8: Find Pods Affected by Node Pressure

```cypher
// Find node with pressure
MATCH (n:Node {name: "worker-1"})

// Find all pods on this node
MATCH (n)<-[:SCHEDULED_ON]-(p:Pod)

// Find any investigations related to these pods
OPTIONAL MATCH (inv:Investigation)-[:INVESTIGATING]->(p)

WITH n, p, inv
ORDER BY inv.created_at DESC

RETURN 
  n.name as node,
  collect(DISTINCT {
    pod: p.name,
    namespace: p.namespace,
    status: p.status,
    investigation: inv.symptom,
    investigation_time: inv.created_at
  }) as affected_pods,
  count(DISTINCT p) as total_pods,
  count(DISTINCT inv) as pods_under_investigation
```

**Next Steps:**
- Evaluate pod resource requests vs actual usage
- Consider adding more nodes or evicting low-priority pods
- Check for pods without resource limits

---

## Network Issues

### Symptom
Connectivity problems, packet loss, or network errors.

### Relevant Metrics
- `container_network_receive_errors_total`
- `container_network_transmit_errors_total`

### Query 9: Identify Pods with Network Errors

```cypher
// Find investigation
MATCH (inv:Investigation)
WHERE inv.symptom CONTAINS "Network" OR inv.symptom = "HighErrorRate"

// Get network error metrics
MATCH (inv)-[:HAS_METRIC_EVIDENCE]->(m:Metric)
WHERE m.name IN ["container_network_receive_errors_total", "container_network_transmit_errors_total"]

// Get the pod
MATCH (m)-[:EMITTED_BY]->(p:Pod)

WITH p, 
     sum(CASE WHEN m.name = "container_network_receive_errors_total" THEN m.value ELSE 0 END) as rx_errors,
     sum(CASE WHEN m.name = "container_network_transmit_errors_total" THEN m.value ELSE 0 END) as tx_errors

WHERE rx_errors > 0 OR tx_errors > 0

RETURN 
  p.name as pod,
  p.namespace as namespace,
  rx_errors,
  tx_errors,
  CASE
    WHEN rx_errors > tx_errors THEN "Incoming network issues"
    WHEN tx_errors > rx_errors THEN "Outgoing network issues"
    ELSE "Bidirectional network issues"
  END as issue_type
ORDER BY (rx_errors + tx_errors) DESC
```

**Next Steps:**
- Check NetworkPolicy resources that might be blocking traffic
- Verify CNI plugin health
- Check node-level networking (ip routes, iptables)

---

## Container Resource Exhaustion

### Symptom
Container restart due to resource limits, throttling, or slow performance.

### Query 10: Detect Container Throttling Patterns

```cypher
// Find container investigation
MATCH (inv:Investigation)-[:INVESTIGATING]->(c:Container)

// Get CPU metrics over time
MATCH (inv)-[:HAS_METRIC_EVIDENCE]->(cpu:Metric)
WHERE cpu.name = "container_cpu_usage_seconds_total"
  AND cpu.label_container = c.name

// Calculate if hitting CPU limit
WITH c,
     collect(cpu.value) as cpu_values,
     c.cpu_limit as limit

WITH c,
     cpu_values,
     limit,
     reduce(sum = 0.0, val IN cpu_values | sum + val) / size(cpu_values) as avg_cpu

RETURN 
  c.name as container,
  c.pod_name as pod,
  avg_cpu * 100 as avg_cpu_percent,
  limit,
  CASE
    WHEN avg_cpu >= (limit * 0.95) THEN "CPU throttling likely"
    WHEN avg_cpu >= (limit * 0.80) THEN "Approaching CPU limit"
    ELSE "CPU usage normal"
  END as status,
  "Consider increasing CPU limit to " + toString(limit * 1.5) as recommendation
```

**Interpretation:**
- CPU throttling likely → Container is being throttled, causing slowness
- Approaching CPU limit → May experience intermittent throttling
- Recommendation: Increase CPU limit based on actual usage patterns

---

## Tips for Effective RCA Queries

1. **Always start with the Investigation node** - It provides context and links to relevant metrics
2. **Use time-based filtering** - Focus on metrics around the incident time
3. **Correlate multiple metric types** - CPU + Memory + Network for complete picture
4. **Follow relationships** - Use `CALLS`, `SCHEDULED_ON`, `SELECTS` to understand dependencies
5. **Compare before/after** - Look at metric trends, not just single points
6. **Check for temporal patterns** - Spikes, gradual increases, or sudden drops
7. **Clean up after investigation** - Call `CompleteInvestigation()` to purge temporary metrics

## Query Optimization Tips

- Use `WHERE` clauses to filter early in the query
- Limit result sets with `LIMIT` when exploring
- Use `PROFILE` or `EXPLAIN` to understand query performance
- Index commonly queried properties (name, namespace, timestamp)
- For time-series analysis, process metrics in application code when needed

