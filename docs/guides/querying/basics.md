# Querying the Knowledge Graph

This guide covers the most common query patterns for exploring and troubleshooting your cluster using Cypher queries in Neo4j.

For a complete query reference, see [Cypher Queries Reference](../reference/cypher-queries.md).

## Getting Started

Access the Neo4j Browser:

```bash
kubectl port-forward svc/neo4j 7474:7474 7687:7687
```

Open http://localhost:7474 and log in with your credentials.

## Graph Overview Queries

### See What's in Your Graph

```cypher
MATCH (n)
RETURN labels(n)[0] as type, count(*) as count
ORDER BY count DESC
```

Shows all node types and their counts to understand what's being tracked.

### View All Relationships

```cypher
MATCH ()-[r]->()
RETURN type(r) as relationship, count(r) as count
ORDER BY count DESC
```

Lists all relationship types in your cluster graph.

## Workload Queries

### Find All Pods and Their Status

```cypher
MATCH (p:Pod)
RETURN p.namespace, p.name, p.status, p.node_name
ORDER BY p.namespace, p.name
```

### View Deployment Hierarchy

```cypher
MATCH path = (d:Deployment)-[:MANAGES]->(rs:ReplicaSet)-[:MANAGES]->(p:Pod)
WHERE d.name = 'my-app'
RETURN path
```

Visualize the full deployment → replicaset → pod chain.

### Find Failing Pods

```cypher
MATCH (p:Pod)
WHERE p.status IN ['Failed', 'CrashLoopBackOff', 'Error', 'Pending']
RETURN p.namespace, p.name, p.status, p.node_name
ORDER BY p.namespace, p.name
```

Quickly identify problematic pods across your cluster.

## Networking Queries

### Service to Pod Mapping

```cypher
MATCH (s:Service)-[:SELECTS_PODS]->(p:Pod)
RETURN s.name, s.namespace, collect(p.name) as pods
ORDER BY s.namespace, s.name
```

See which pods are selected by each service.

### Find Services with No Backend Pods

```cypher
MATCH (s:Service)
WHERE NOT (s)-[:SELECTS_PODS]->(:Pod)
RETURN s.namespace, s.name, s.selector
ORDER BY s.namespace, s.name
```

Identify broken services that have no pods to route to.

### Trace Traffic Path (Gateway API)

```cypher
MATCH (gw:Gateway)<-[:ATTACHES_TO]-(route:HTTPRoute)-[:FORWARDS_TO]->(svc:Service)-[:SELECTS_PODS]->(pod:Pod)
WHERE gw.name = 'my-gateway'
RETURN gw.name as gateway, route.name as route, svc.name as service, count(pod) as pod_count
```

Follow the path from gateway to backend pods.

## Troubleshooting Queries

### Find Recent Events for a Resource

```cypher
MATCH (e:K8sEvent)-[:INVOLVES]->(p:Pod {name: 'my-pod', namespace: 'default'})
RETURN e.type, e.reason, e.message, e.last_timestamp
ORDER BY e.last_timestamp DESC
LIMIT 10
```

Get recent events related to a specific pod.

### Find Warning Events Cluster-Wide

```cypher
MATCH (e:K8sEvent)-[:INVOLVES]->(resource)
WHERE e.type = 'Warning'
RETURN e.namespace, labels(resource)[0] as resource_type,
       resource.name, e.reason, e.message, e.last_timestamp
ORDER BY e.last_timestamp DESC
LIMIT 20
```

See recent warnings across all resources.

### Find Pods with High Restart Counts

```cypher
MATCH (p:Pod)-[:CONTAINS]->(c:Container)
WHERE c.restart_count > 5
RETURN p.namespace, p.name, c.name as container, c.restart_count as restarts
ORDER BY c.restart_count DESC
```

Identify containers that are repeatedly failing.

## Impact Analysis

### Blast Radius: What Would a Node Failure Affect?

```cypher
MATCH (n:Node {name: 'worker-1'})<-[:SCHEDULED_ON]-(p:Pod)
OPTIONAL MATCH (p)<-[:SELECTS_PODS]-(s:Service)
OPTIONAL MATCH (p)<-[:MANAGES]-()<-[:MANAGES]-(d:Deployment)
RETURN n.name as node,
       count(DISTINCT p) as affected_pods,
       count(DISTINCT s) as affected_services,
       count(DISTINCT d) as affected_deployments
```

Understand the impact of a node going down.

### Find All Resources in a Namespace

```cypher
MATCH (r)-[:IN_NAMESPACE]->(ns:Namespace {name: 'production'})
RETURN labels(r)[0] as type, count(r) as count
ORDER BY count DESC
```

Get resource counts for capacity planning or cleanup.

## Configuration Queries

### Find Pods Using a ConfigMap

```cypher
MATCH (p:Pod)-[:USES_CONFIG]->(cm:ConfigMap {name: 'app-config', namespace: 'default'})
RETURN p.name, p.namespace
ORDER BY p.name
```

Identify which pods would be affected by a ConfigMap change.

### Find Unused ConfigMaps

```cypher
MATCH (cm:ConfigMap)
WHERE NOT ()-[:USES_CONFIG]->(cm)
RETURN cm.namespace, cm.name
ORDER BY cm.namespace, cm.name
```

Clean up unused configuration resources.

## Storage Queries

### View Storage Chain: Pod → PVC → PV

```cypher
MATCH path = (p:Pod)-[:MOUNTS]->(pvc:PersistentVolumeClaim)-[:BOUND_TO]->(pv:PersistentVolume)
WHERE p.namespace = 'default'
RETURN path
```

Trace storage dependencies for a namespace.

### Find Unbound PVCs

```cypher
MATCH (pvc:PersistentVolumeClaim)
WHERE NOT (pvc)-[:BOUND_TO]->()
RETURN pvc.namespace, pvc.name, pvc.status, pvc.storage_class
ORDER BY pvc.namespace, pvc.name
```

Identify PVCs waiting for volumes.

## Extension Queries

### Gateway API: Find Routes Without Healthy Backends

```cypher
MATCH (route:HTTPRoute)-[:FORWARDS_TO]->(svc:Service)
WHERE NOT (svc)-[:SELECTS_PODS]->(:Pod {status: 'Running'})
RETURN route.namespace, route.name, svc.name
```

### Istio: Find Canary Deployments

```cypher
MATCH (vs:VirtualService)-[r:ROUTES_TO_SUBSET]->(dr:DestinationRule)
MATCH (dr)-[:SELECTS_SUBSET_PODS]->(pod:Pod)
WHERE r.weight < 100
RETURN vs.name, r.subset_name as version, r.weight as traffic_percent, count(pod) as pod_count
ORDER BY vs.name, r.weight DESC
```

### Istio: Check Security Posture

```cypher
MATCH (svc:Service {name: 'payment-service'})-[:SELECTS_PODS]->(pod:Pod)
OPTIONAL MATCH (authz:AuthorizationPolicy)-[:APPLIES_TO]->(pod)
OPTIONAL MATCH (peer:PeerAuthentication)-[:APPLIES_TO]->(pod)
RETURN pod.name,
       collect(DISTINCT authz.name) as authz_policies,
       collect(DISTINCT peer.mtls_mode) as mtls_modes
```

## Tips for Effective Queries

1. **Always use LIMIT** - Prevent overwhelming results on large clusters
   ```cypher
   MATCH (p:Pod) RETURN p LIMIT 100
   ```

2. **Filter early** - Use WHERE clauses to narrow results
   ```cypher
   MATCH (p:Pod)
   WHERE p.namespace = 'production' AND p.status = 'Running'
   RETURN p
   ```

3. **Use indexes** - Node IDs are already indexed for fast lookups
   ```cypher
   MATCH (p:Pod {id: 'Pod/default/my-pod'}) RETURN p
   ```

4. **Collect related data** - Use `collect()` to aggregate
   ```cypher
   MATCH (s:Service)-[:SELECTS_PODS]->(p:Pod)
   RETURN s.name, collect(p.name) as pods
   ```

5. **Visualize paths** - Use `RETURN path` for graph visualization
   ```cypher
   MATCH path = (d:Deployment)-[:MANAGES*]->(:Pod)
   RETURN path LIMIT 10
   ```

## Next Steps

- **[Complete Query Reference](../reference/cypher-queries.md)** - 100+ queries for all scenarios
- **[Graph Schema](../reference/graph-schema.md)** - All node and edge types
- **[Extensions Guide](extensions.md)** - Gateway API and Istio specific queries
- **[Neo4j Cypher Documentation](https://neo4j.com/docs/cypher-manual/)** - Learn more Cypher

