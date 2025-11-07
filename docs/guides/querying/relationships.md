# Querying Relationships

Learn how to traverse edges and follow dependencies in the knowledge graph.

## Understanding Relationships

In kkbase, relationships (edges) connect resources (nodes) to show dependencies, hierarchies, and connections.

## Basic Relationship Syntax

```cypher
// Pattern: (from)-[relationship]->(to)
MATCH (pod:Pod)-[:SCHEDULED_ON]->(node:Node)
RETURN pod, node
```

## Common Relationship Types

### Hierarchical Relationships

```cypher
// Deployment → ReplicaSet → Pod
MATCH (d:Deployment)-[:MANAGES]->(rs:ReplicaSet)-[:MANAGES]->(p:Pod)
WHERE d.name = 'nginx-deployment'
RETURN d, rs, p
```

### Networking Relationships

```cypher
// Service → Pods
MATCH (s:Service)-[:SELECTS_PODS]->(p:Pod)
WHERE s.name = 'web-service'
RETURN s, p
```

### Storage Relationships

```cypher
// Pod → PVC → PV
MATCH (pod:Pod)-[:MOUNTS]->(pvc:PersistentVolumeClaim)
      -[:BOUND_TO]->(pv:PersistentVolume)
RETURN pod, pvc, pv
```

### Configuration Relationships

```cypher
// Pod → ConfigMap
MATCH (pod:Pod)-[:USES_CONFIG]->(cm:ConfigMap)
RETURN pod.name, collect(cm.name) as configs
```

## Multi-Hop Traversal

### Variable Length Paths

```cypher
// Follow MANAGES relationship 1-3 hops
MATCH path = (d:Deployment)-[:MANAGES*1..3]->(p:Pod)
RETURN path
```

### Shortest Path

```cypher
// Find shortest path between nodes
MATCH path = shortestPath(
  (s:Service)-[*]-(n:Node)
)
RETURN path
```

## Dependency Chains

### Trace Upstream Dependencies

```cypher
// What depends on this pod?
MATCH (pod:Pod {name: 'api-pod-123'})
MATCH (upstream)-[*1..3]->(pod)
RETURN upstream, labels(upstream)[0] as type
```

### Trace Downstream Dependencies

```cypher
// What does this service depend on?
MATCH (s:Service {name: 'orders-api'})
MATCH (s)-[*1..5]->(downstream)
RETURN s, downstream, labels(downstream)[0] as type
```

## Impact Analysis

### Blast Radius

```cypher
// What breaks if this node fails?
MATCH (n:Node {name: 'worker-3'})<-[:SCHEDULED_ON]-(p:Pod)
OPTIONAL MATCH (p)<-[:SELECTS_PODS]-(s:Service)
OPTIONAL MATCH (p)<-[:MANAGES]-()-[:MANAGES]-(d:Deployment)
RETURN n,
       collect(DISTINCT p.name) as pods,
       collect(DISTINCT s.name) as services,
       collect(DISTINCT d.name) as deployments
```

## Filtering Relationships

### By Relationship Properties

```cypher
// Find canary traffic routes (Istio)
MATCH (vs:VirtualService)-[r:ROUTES_TO_SUBSET]->(dr:DestinationRule)
WHERE r.subset_name = 'canary'
RETURN vs, r.weight, dr
```

### By Multiple Conditions

```cypher
// Pods using specific config in specific namespace
MATCH (pod:Pod)-[:USES_CONFIG]->(cm:ConfigMap)
WHERE pod.namespace = 'production'
  AND cm.name CONTAINS 'database'
RETURN pod.name, cm.name
```

## Optional Relationships

Use OPTIONAL MATCH for relationships that may not exist:

```cypher
MATCH (p:Pod)
OPTIONAL MATCH (p)-[:USES_CONFIG]->(cm:ConfigMap)
OPTIONAL MATCH (p)-[:USES_SECRET]->(s:Secret)
RETURN p.name,
       COALESCE(cm.name, 'none') as config,
       COALESCE(s.name, 'none') as secret
```

## Aggregating Across Relationships

```cypher
// Count pods per service
MATCH (s:Service)-[:SELECTS_PODS]->(p:Pod)
RETURN s.name,
       count(p) as total_pods,
       collect(p.status) as statuses
```

## Pattern Matching

### Complex Patterns

```cypher
// Services with unhealthy pods
MATCH (s:Service)-[:SELECTS_PODS]->(p:Pod)
WHERE p.status <> 'Running'
RETURN s.name,
       count(p) as unhealthy_pods,
       collect(p.name) as pod_names
```

### Negative Patterns

```cypher
// Services with NO backend pods
MATCH (s:Service)
WHERE NOT (s)-[:SELECTS_PODS]->(:Pod)
RETURN s.namespace, s.name
```

## Performance Tips

1. **Specify direction** - `->` is faster than `-`
2. **Limit hop depth** - Use `*1..3` not `*`
3. **Filter early** - WHERE before traversal
4. **Use OPTIONAL** - Only when needed
5. **Index lookups** - Start with indexed properties

## See Also

- [Basics](basics.md) - Basic query patterns
- [Advanced](advanced.md) - Complex patterns
- [Graph Schema](../../reference/graph-schema.md) - All relationship types

