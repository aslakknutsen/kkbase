# Metrics Investigation User Guide

This guide explains how to use the investigation-scoped metrics system for root cause analysis in kkbase.

## Table of Contents

1. [Overview](#overview)
2. [Architecture](#architecture)
3. [Starting an Investigation](#starting-an-investigation)
4. [Understanding Investigation Sessions](#understanding-investigation-sessions)
5. [Querying Metrics](#querying-metrics)
6. [Metric Retention and Cleanup](#metric-retention-and-cleanup)
7. [Adding Custom Metrics](#adding-custom-metrics)
8. [Troubleshooting](#troubleshooting)

---

## Overview

The kkbase metrics investigation system uses a **hybrid on-demand approach**:

- **On-demand**: Metrics are pulled from Prometheus only when needed for investigation
- **Temporary storage**: Metrics are stored in Neo4j only during active investigations
- **Automatic cleanup**: Metrics are purged after investigation completes
- **Label-based correlation**: Metrics are automatically linked to K8s resources

This approach avoids the cardinality explosion of storing all metrics in Neo4j while still enabling powerful RCA queries.

### Key Benefits

- **No continuous ingestion**: Only pull metrics when investigating issues
- **Efficient storage**: Temporary metrics don't bloat the graph
- **Rich correlation**: Metrics automatically linked to Pods, Nodes, Services
- **Flexible queries**: Use Cypher to correlate metrics with K8s topology

---

## Architecture

```
┌─────────────────┐
│  Agent detects  │
│     issue       │
└────────┬────────┘
         │
         ▼
┌─────────────────────────┐
│ InvestigationMetrics    │
│      Processor          │
└──────┬──────────────────┘
       │
       ├──> Query Prometheus (15min lookback)
       │
       ├──> Store metrics in Neo4j with Investigation context
       │
       ├──> Create EMITTED_BY edges (Metric → Resource)
       │
       └──> Enable RCA queries
            │
            ▼
    [Investigation Complete]
            │
            ▼
    Purge all metrics from graph
```

### Node Types

- **Investigation**: Represents an active RCA session
- **Metric**: Individual metric data points (temporary)

### Relationship Types

- `INVESTIGATING`: Investigation → Resource (Pod, Node, Service)
- `HAS_METRIC_EVIDENCE`: Investigation → Metric
- `EMITTED_BY`: Metric → Resource (Pod, Container, Node, Service)

---

## Starting an Investigation

### Basic Usage

```go
package main

import (
    "context"
    "time"

    "github.com/kagenti/kkbase/pkg/graph/neo4j"
    "github.com/kagenti/kkbase/pkg/observability"
    "github.com/kagenti/kkbase/pkg/observability/prometheus"
    "go.uber.org/zap"
)

func main() {
    // Setup
    logger, _ := zap.NewProduction()
    graphStore, _ := neo4j.NewNeo4jStore("bolt://localhost:7687", "", "")
    promProvider := prometheus.NewProvider("http://localhost:9090", logger)
    
    // Create processor
    processor := observability.NewInvestigationMetricsProcessor(
        graphStore,
        promProvider,
        logger,
    )
    
    // Start investigation
    session, err := processor.StartInvestigation(
        context.Background(),
        "Pod",                       // Resource type
        "Pod/prod/api-gateway-xyz",  // Resource ID
        "CrashLoopBackOff",          // Symptom
        15*time.Minute,              // Lookback duration
    )
    if err != nil {
        logger.Fatal("Failed to start investigation", zap.Error(err))
    }
    
    logger.Info("Investigation started", 
        zap.String("id", session.ID),
        zap.String("status", session.Status))
    
    // ... Perform RCA queries ...
    
    // Complete and cleanup
    if err := processor.CompleteInvestigation(context.Background(), session.ID); err != nil {
        logger.Error("Failed to complete investigation", zap.Error(err))
    }
}
```

### Symptom-Based Metric Selection

The processor automatically selects relevant metrics based on the symptom:

| Symptom | Selected Metrics |
|---------|------------------|
| `OOMKilled` | `container_memory_usage_bytes`, `container_memory_working_set_bytes`, `container_cpu_usage_seconds_total` |
| `CrashLoopBackOff` | Same as OOMKilled |
| `HighLatency` | `http_request_duration_seconds`, `container_cpu_usage_seconds_total`, `node_cpu_seconds_total` |
| `HighErrorRate` | `http_requests_total`, `container_network_receive_errors_total`, `container_network_transmit_errors_total` |
| `NodeNotReady` | `node_cpu_seconds_total`, `node_memory_MemAvailable_bytes`, `node_load1` |
| Default | Core resource metrics for the resource type |

---

## Understanding Investigation Sessions

### Investigation Lifecycle

```
┌──────────┐     StartInvestigation()     ┌────────┐
│          │──────────────────────────────>│        │
│  Idle    │                               │ Active │
│          │<──────────────────────────────│        │
└──────────┘   CompleteInvestigation()    └────────┘
                                                 │
                                                 │ AbandonInvestigation()
                                                 ▼
                                          ┌──────────┐
                                          │          │
                                          │Completed │
                                          │          │
                                          └──────────┘
```

### Session Properties

```go
type InvestigationSession struct {
    ID               string        // Unique investigation ID
    ResourceType     string        // Pod, Service, Node, etc.
    ResourceID       string        // Full resource ID
    Symptom          string        // Symptom being investigated
    StartTime        time.Time     // When investigation started
    LookbackDuration time.Duration // How far back metrics were pulled
    Status           string        // "active", "completed", "abandoned"
    CreatedAt        time.Time     // Creation timestamp
}
```

---

## Querying Metrics

### Example 1: Find Memory Usage During Investigation

```cypher
// Find investigation for pod
MATCH (inv:Investigation)-[:INVESTIGATING]->(p:Pod {name: "api-gateway", namespace: "prod"})

// Get memory metrics
MATCH (inv)-[:HAS_METRIC_EVIDENCE]->(m:Metric)
WHERE m.name = "container_memory_usage_bytes"

RETURN 
  datetime(m.timestamp) as time,
  m.value / 1048576 as memory_mb,
  m.label_container as container
ORDER BY time ASC
```

### Example 2: Correlate Metrics with Pod Events

```cypher
// Find pod investigation
MATCH (inv:Investigation)-[:INVESTIGATING]->(p:Pod {name: "api-gateway"})

// Get metrics and events
MATCH (inv)-[:HAS_METRIC_EVIDENCE]->(m:Metric)
WHERE m.name = "container_memory_usage_bytes"

MATCH (p)<-[:AFFECTS]-(e:K8sEvent)
WHERE datetime(e.timestamp) >= datetime(inv.start_time)

RETURN 
  datetime(m.timestamp) as metric_time,
  m.value as memory,
  datetime(e.timestamp) as event_time,
  e.reason as event_reason,
  e.message as event_message
ORDER BY metric_time ASC
```

See [metrics-rca-queries.md](../reference/metrics-rca-queries.md) for comprehensive query examples.

---

## Metric Retention and Cleanup

### Automatic Cleanup

Metrics are automatically purged when:
- `CompleteInvestigation()` is called
- `AbandonInvestigation()` is called

```go
// Manual cleanup
err := processor.CompleteInvestigation(ctx, session.ID)
```

### Cleanup Verification

```cypher
// Check for orphaned metrics (shouldn't exist)
MATCH (m:Metric)
WHERE NOT EXISTS((m)<-[:HAS_METRIC_EVIDENCE]-(:Investigation))
RETURN count(m) as orphaned_metrics
```

If orphaned metrics exist, they can be cleaned up manually:

```cypher
// Clean up orphaned metrics
MATCH (m:Metric)
WHERE NOT EXISTS((m)<-[:HAS_METRIC_EVIDENCE]-(:Investigation))
DETACH DELETE m
```

### Best Practices

1. **Always complete investigations** - Use `defer` to ensure cleanup
   ```go
   session, err := processor.StartInvestigation(...)
   defer processor.CompleteInvestigation(ctx, session.ID)
   ```

2. **Set appropriate lookback duration** - Longer = more metrics, more storage
   ```go
   // For recent issues
   lookback := 15 * time.Minute
   
   // For historical analysis
   lookback := 2 * time.Hour
   ```

3. **Monitor investigation count** - Too many active investigations may indicate issues
   ```cypher
   MATCH (inv:Investigation {status: "active"})
   RETURN count(inv) as active_investigations
   ```

---

## Adding Custom Metrics

### 1. Register Metric in Catalog

```go
catalog := observability.NewMetricCatalog()

// Define custom metric
customMetric := observability.MetricDefinition{
    Name:              "custom_queue_depth",
    Type:              observability.MetricTypeGauge,
    Description:       "Number of items in processing queue",
    Unit:              "items",
    PromQLTemplate:    `custom_queue_depth{namespace="{{.Namespace}}", pod="{{.PodName}}"}`,
    ApplicableToTypes: []string{"Pod", "Service"},
    Category:          "performance",
}

catalog.Register(customMetric)
```

### 2. Add to Symptom Mapping

Extend `selectMetricsForSymptom()` in your processor:

```go
func (imp *InvestigationMetricsProcessor) selectMetricsForSymptom(symptom, resourceType string) []string {
    switch symptom {
    case "HighQueueDepth":
        return []string{
            "custom_queue_depth",
            "container_cpu_usage_seconds_total",
        }
    // ... existing cases ...
    }
}
```

### 3. Query Custom Metrics

```cypher
MATCH (inv:Investigation)-[:HAS_METRIC_EVIDENCE]->(m:Metric)
WHERE m.name = "custom_queue_depth"
RETURN 
  datetime(m.timestamp) as time,
  m.value as queue_depth
ORDER BY time ASC
```

---

## Troubleshooting

### Issue: No Metrics Found

**Symptoms:**
- Investigation starts but no metrics are stored
- Queries return empty results

**Possible Causes:**

1. **Prometheus Unavailable**
   ```go
   // Check provider configuration
   promProvider := prometheus.NewProvider("http://localhost:9090", logger)
   ```

2. **Metric Names Don't Exist**
   ```bash
   # Verify metrics exist in Prometheus
   curl http://localhost:9090/api/v1/label/__name__/values | grep container_memory
   ```

3. **Label Mismatch**
   - Metrics have labels like `pod_name` instead of `pod`
   - Solution: Update correlator to support alternative labels

4. **Time Range Issue**
   - Lookback duration is too short
   - Metrics were scraped after investigation started
   - Solution: Increase lookback duration

### Issue: Metrics Not Correlating to Resources

**Symptoms:**
- Metrics stored but not linked to Pods/Nodes/Services
- No `EMITTED_BY` edges

**Solutions:**

1. **Check Label Format**
   ```cypher
   MATCH (m:Metric {name: "container_memory_usage_bytes"})
   RETURN m.label_pod, m.label_namespace, m.label_container
   LIMIT 5
   ```

2. **Verify Resource Exists**
   ```cypher
   MATCH (p:Pod {name: "api-gateway", namespace: "prod"})
   RETURN p
   ```

3. **Check Correlator Logic**
   - Add debug logging to `MetricCorrelator`
   - Verify label names match Prometheus conventions

### Issue: Graph Growing Too Large

**Symptoms:**
- Neo4j running out of memory
- Query performance degrading

**Solutions:**

1. **Check for Uncompleted Investigations**
   ```cypher
   MATCH (inv:Investigation {status: "active"})
   WHERE datetime(inv.created_at) < datetime() - duration('PT1H')
   RETURN count(inv)
   ```

2. **Force Cleanup**
   ```cypher
   MATCH (inv:Investigation {status: "active"})
   WHERE datetime(inv.created_at) < datetime() - duration('PT1H')
   WITH inv
   MATCH (inv)-[:HAS_METRIC_EVIDENCE]->(m:Metric)
   DETACH DELETE m, inv
   ```

3. **Reduce Lookback Duration**
   ```go
   // Instead of 2 hours
   lookback := 2 * time.Hour
   
   // Use 15 minutes
   lookback := 15 * time.Minute
   ```

### Issue: Prometheus Rate Limiting

**Symptoms:**
- Intermittent failures when querying metrics
- Error: "query timeout" or "too many requests"

**Solutions:**

1. **Increase Prometheus Query Timeout**
   ```go
   provider := prometheus.NewProvider("http://localhost:9090", logger)
   provider.httpClient.Timeout = 60 * time.Second
   ```

2. **Reduce Metric Count**
   - Query fewer metrics per investigation
   - Increase step duration (reduces data points)

3. **Implement Retry Logic**
   - Add exponential backoff for Prometheus queries
   - Handle rate limit errors gracefully

---

## Performance Considerations

### Query Performance

- **Index key properties**: Ensure `id`, `name`, `namespace`, `timestamp` are indexed
- **Limit result sets**: Use `LIMIT` in development queries
- **Avoid full graph scans**: Always start queries with specific nodes
- **Use temporal filtering**: Filter by timestamp early in queries

### Storage Impact

| Lookback Duration | Metric Count | Step Duration | Approximate Metrics Stored |
|-------------------|--------------|---------------|---------------------------|
| 15 minutes        | 3 metrics    | 1 minute      | 45 metric nodes           |
| 1 hour            | 3 metrics    | 1 minute      | 180 metric nodes          |
| 6 hours           | 5 metrics    | 1 minute      | 1,800 metric nodes        |

**Recommendation**: Use 15-minute lookback for most investigations.

---

## Next Steps

- Review [Metrics RCA Queries](../reference/metrics-rca-queries.md) for query examples
- Check [Graph Schema](../reference/graph-schema.md) for complete data model
- See [Architecture](../development/architecture.md) for system design details

