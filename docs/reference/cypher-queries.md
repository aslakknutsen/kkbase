# Neo4j Cypher Queries Reference

This document contains all major Neo4j Cypher queries of interest for the Kubernetes Knowledge Base system.

## Table of Contents
- [Graph Overview & Statistics](#graph-overview--statistics)
- [Node Discovery](#node-discovery)
- [Relationship Discovery](#relationship-discovery)
- [Core Kubernetes Resources](#core-kubernetes-resources)
- [Workload Queries](#workload-queries)
- [Networking Queries](#networking-queries)
- [Storage Queries](#storage-queries)
- [Configuration Queries](#configuration-queries)
- [Gateway API Queries](#gateway-api-queries)
- [Istio Queries](#istio-queries)
- [Distributed Tracing Queries](#distributed-tracing-queries)
- [Troubleshooting & Debugging](#troubleshooting--debugging)
- [Performance & Analytics](#performance--analytics)
- [Data Cleanup](#data-cleanup)
- [Placeholder Node Queries](#placeholder-node-queries)

---

## Graph Overview & Statistics

### Count all nodes by type
```cypher
MATCH (n)
RETURN labels(n)[0] AS NodeType, count(n) AS Count
ORDER BY Count DESC
```

### Count all relationships by type
```cypher
MATCH ()-[r]->()
RETURN type(r) AS RelationshipType, count(r) AS Count
ORDER BY Count DESC
```

### Get graph statistics
```cypher
MATCH (n)
WITH labels(n)[0] AS NodeType, count(n) AS NodeCount
OPTIONAL MATCH (n)-[r]->()
WHERE labels(n)[0] = NodeType
RETURN NodeType, NodeCount, count(r) AS RelationshipCount
ORDER BY NodeCount DESC
```

### Get all node types in the graph
```cypher
MATCH (n)
RETURN DISTINCT labels(n) AS Labels
```

### Get all relationship types in the graph
```cypher
MATCH ()-[r]->()
RETURN DISTINCT type(r) AS RelationshipType
```

### List all Node types and their Relationship types
```cypher
MATCH (n)-[r]->(m)
RETURN DISTINCT labels(n)[0] AS NodeType, 
       type(r) AS RelationshipType, 
       labels(m)[0] AS TargetNodeType,
       count(r) AS Count
ORDER BY NodeType, RelationshipType, TargetNodeType
```

### List Node types with their outgoing relationships
```cypher
MATCH (n)-[r]->()
RETURN DISTINCT labels(n)[0] AS NodeType, 
       collect(DISTINCT type(r)) AS OutgoingRelationships
ORDER BY NodeType
```

### Find nodes with most relationships
```cypher
MATCH (n)-[r]-()
RETURN labels(n)[0] AS NodeType, n.name AS Name, n.namespace AS Namespace, count(r) AS RelationshipCount
ORDER BY RelationshipCount DESC
LIMIT 20
```

### Find orphaned nodes (no relationships)
```cypher
MATCH (n)
WHERE NOT (n)-[]-()
RETURN labels(n)[0] AS NodeType, n.id AS ID, n.name AS Name
```

---

## Node Discovery

### List all nodes of a specific type
```cypher
MATCH (n:Pod)
RETURN n.name AS Name, n.namespace AS Namespace, n.status AS Status, n.node_name AS NodeName
ORDER BY n.namespace, n.name
```

### Find nodes by name pattern
```cypher
MATCH (n)
WHERE n.name CONTAINS 'nginx'
RETURN labels(n)[0] AS Type, n.name AS Name, n.namespace AS Namespace
```

### Find nodes by namespace
```cypher
MATCH (n)
WHERE n.namespace = 'production'
RETURN labels(n)[0] AS Type, n.name AS Name, n.status AS Status
ORDER BY Type, Name
```

### Find nodes with specific labels
```cypher
MATCH (n:Pod)
WHERE n.labels CONTAINS 'app=frontend'
RETURN n.name AS Name, n.namespace AS Namespace, n.labels AS Labels
```

### Get detailed node properties
```cypher
MATCH (n:Pod {name: 'my-pod', namespace: 'default'})
RETURN n
```

---

## Relationship Discovery

### Find all relationships of a specific node
```cypher
MATCH (n {name: 'my-service', namespace: 'default'})-[r]-(m)
RETURN labels(n)[0] AS FromType, n.name AS FromName, 
       type(r) AS Relationship, 
       labels(m)[0] AS ToType, m.name AS ToName
```

### Find paths between two nodes
```cypher
MATCH path = shortestPath(
  (a:Pod {name: 'frontend-pod', namespace: 'default'})-[*]-(b:Service {name: 'backend-service', namespace: 'default'})
)
RETURN path
```

### Find all paths up to depth N
```cypher
MATCH path = (a:Pod {name: 'my-pod'})-[*1..3]-(b)
RETURN path
LIMIT 100
```

### Find nodes connected by specific relationship
```cypher
MATCH (a)-[r:SELECTS_PODS]->(b)
RETURN labels(a)[0] AS From, a.name AS FromName, 
       labels(b)[0] AS To, b.name AS ToName, 
       a.namespace AS Namespace
```

---

## Core Kubernetes Resources

### List all namespaces
```cypher
MATCH (n:Namespace)
RETURN n.name AS Namespace, n.status AS Status
ORDER BY n.name
```

### List all nodes (servers) with capacity
```cypher
MATCH (n:Node)
RETURN n.name AS NodeName, n.status AS Status, 
       n.cpu_capacity AS CPU, n.memory_capacity AS Memory,
       n.internal_ip AS IP
ORDER BY n.name
```

### List all events
```cypher
MATCH (e:K8sEvent)
RETURN e.name AS Event, e.namespace AS Namespace, 
       e.type AS Type, e.reason AS Reason, e.message AS Message,
       e.first_timestamp AS FirstSeen, e.last_timestamp AS LastSeen
ORDER BY e.last_timestamp DESC
LIMIT 50
```

### Find recent warning/error events
```cypher
MATCH (e:K8sEvent)
WHERE e.type IN ['Warning', 'Error']
RETURN e.namespace AS Namespace, e.involved_object_name AS Resource,
       e.reason AS Reason, e.message AS Message, e.last_timestamp AS Time
ORDER BY e.last_timestamp DESC
LIMIT 20
```

---

## Workload Queries

### List all deployments with replica counts
```cypher
MATCH (d:Deployment)
RETURN d.name AS Deployment, d.namespace AS Namespace,
       d.replicas AS DesiredReplicas, d.ready_replicas AS ReadyReplicas,
       d.available_replicas AS AvailableReplicas
ORDER BY d.namespace, d.name
```

### Find pods by deployment
```cypher
MATCH (d:Deployment {name: 'nginx-deployment', namespace: 'default'})-[:MANAGES]->(rs:ReplicaSet)-[:MANAGES]->(p:Pod)
RETURN p.name AS Pod, p.status AS Status, p.node_name AS Node, p.ip AS IP
```

### Find all deployments and their pods
```cypher
MATCH (d:Deployment)-[:MANAGES]->(rs:ReplicaSet)-[:MANAGES]->(p:Pod)
RETURN d.name AS Deployment, d.namespace AS Namespace,
       count(p) AS PodCount,
       collect(DISTINCT p.status) AS PodStatuses
ORDER BY d.namespace, d.name
```

### Find pods not in Running state
```cypher
MATCH (p:Pod)
WHERE p.status <> 'Running'
RETURN p.namespace AS Namespace, p.name AS Pod, p.status AS Status,
       p.node_name AS Node
ORDER BY p.namespace, p.name
```

### Find pods scheduled on a specific node
```cypher
MATCH (p:Pod)-[:SCHEDULED_ON]->(n:Node {name: 'worker-node-1'})
RETURN p.namespace AS Namespace, p.name AS Pod, p.status AS Status
ORDER BY p.namespace, p.name
```

### Find statefulsets with their pods
```cypher
MATCH (ss:StatefulSet)-[:MANAGES]->(p:Pod)
RETURN ss.name AS StatefulSet, ss.namespace AS Namespace,
       count(p) AS PodCount, collect(p.name) AS Pods
ORDER BY ss.namespace, ss.name
```

### Find daemonsets and node coverage
```cypher
MATCH (ds:DaemonSet)
OPTIONAL MATCH (ds)-[:MANAGES]->(p:Pod)-[:SCHEDULED_ON]->(n:Node)
WITH ds, count(DISTINCT n) AS NodeCount
MATCH (allNodes:Node)
WITH ds, NodeCount, count(allNodes) AS TotalNodes
RETURN ds.name AS DaemonSet, ds.namespace AS Namespace,
       NodeCount AS NodesWithPod, TotalNodes AS TotalNodes
```

### Find containers and their images
```cypher
MATCH (p:Pod)-[:CONTAINS]->(c:Container)
RETURN p.namespace AS Namespace, p.name AS Pod, 
       c.name AS Container, c.image AS Image,
       c.restart_count AS Restarts
ORDER BY c.restart_count DESC
```

---

## Networking Queries

### List all services with their types
```cypher
MATCH (s:Service)
RETURN s.name AS Service, s.namespace AS Namespace,
       s.type AS Type, s.cluster_ip AS ClusterIP,
       s.selector AS Selector
ORDER BY s.namespace, s.name
```

### Find services and their backend pods
```cypher
MATCH (s:Service)-[:SELECTS_PODS]->(p:Pod)
RETURN s.name AS Service, s.namespace AS Namespace,
       count(p) AS PodCount, collect(p.name) AS Pods
ORDER BY s.namespace, s.name
```

### Find services with no backend pods
```cypher
MATCH (s:Service)
WHERE NOT (s)-[:SELECTS_PODS]->()
RETURN s.namespace AS Namespace, s.name AS Service, s.selector AS Selector
ORDER BY s.namespace, s.name
```

### Find ingress resources and their routing
```cypher
MATCH (i:Ingress)-[:ROUTES_TO]->(s:Service)
RETURN i.name AS Ingress, i.namespace AS Namespace,
       i.host AS Host, s.name AS Service
ORDER BY i.namespace, i.name
```

### Find service dependencies (services calling other services)
```cypher
MATCH (s1:Service)-[:SELECTS_PODS]->(p:Pod)-[:COMMUNICATES_WITH]->(p2:Pod)<-[:SELECTS_PODS]-(s2:Service)
RETURN DISTINCT s1.name AS FromService, s1.namespace AS FromNamespace,
                s2.name AS ToService, s2.namespace AS ToNamespace
```

### List all network policies
```cypher
MATCH (np:NetworkPolicy)
RETURN np.name AS NetworkPolicy, np.namespace AS Namespace,
       np.pod_selector AS PodSelector
ORDER BY np.namespace, np.name
```

### Find pods affected by network policies
```cypher
MATCH (np:NetworkPolicy)-[:AFFECTED_BY]->(p:Pod)
RETURN np.name AS NetworkPolicy, np.namespace AS Namespace,
       count(p) AS AffectedPods
ORDER BY np.namespace, np.name
```

---

## Storage Queries

### List all persistent volumes
```cypher
MATCH (pv:PersistentVolume)
RETURN pv.name AS PersistentVolume, pv.status AS Status,
       pv.capacity AS Capacity, pv.storage_class AS StorageClass,
       pv.reclaim_policy AS ReclaimPolicy
ORDER BY pv.name
```

### List all persistent volume claims
```cypher
MATCH (pvc:PersistentVolumeClaim)
RETURN pvc.name AS PVC, pvc.namespace AS Namespace,
       pvc.status AS Status, pvc.storage_class AS StorageClass,
       pvc.capacity AS Capacity
ORDER BY pvc.namespace, pvc.name
```

### Find PVC to PV bindings
```cypher
MATCH (pvc:PersistentVolumeClaim)-[:BOUND_TO]->(pv:PersistentVolume)
RETURN pvc.namespace AS Namespace, pvc.name AS PVC,
       pv.name AS PV, pvc.capacity AS Capacity
ORDER BY pvc.namespace, pvc.name
```

### Find pods mounting PVCs
```cypher
MATCH (p:Pod)-[:MOUNTS]->(pvc:PersistentVolumeClaim)
RETURN p.namespace AS Namespace, p.name AS Pod,
       pvc.name AS PVC, pvc.capacity AS Capacity
ORDER BY p.namespace, p.name
```

### Find storage classes and their volumes
```cypher
MATCH (pv:PersistentVolume)-[:PROVISIONED_BY]->(sc:StorageClass)
RETURN sc.name AS StorageClass, count(pv) AS VolumeCount,
       collect(pv.name) AS Volumes
ORDER BY VolumeCount DESC
```

### Find unbound PVCs
```cypher
MATCH (pvc:PersistentVolumeClaim)
WHERE NOT (pvc)-[:BOUND_TO]->()
RETURN pvc.namespace AS Namespace, pvc.name AS PVC,
       pvc.status AS Status, pvc.storage_class AS StorageClass
ORDER BY pvc.namespace, pvc.name
```

---

## Configuration Queries

### List all ConfigMaps
```cypher
MATCH (cm:ConfigMap)
RETURN cm.name AS ConfigMap, cm.namespace AS Namespace
ORDER BY cm.namespace, cm.name
```

### List all Secrets
```cypher
MATCH (s:Secret)
RETURN s.name AS Secret, s.namespace AS Namespace, s.type AS Type
ORDER BY s.namespace, s.name
```

### Find pods using specific ConfigMap
```cypher
MATCH (p:Pod)-[:USES_CONFIG]->(cm:ConfigMap {name: 'app-config', namespace: 'default'})
RETURN p.name AS Pod, p.namespace AS Namespace
ORDER BY p.name
```

### Find pods using specific Secret
```cypher
MATCH (p:Pod)-[:USES_SECRET]->(s:Secret {name: 'db-credentials', namespace: 'default'})
RETURN p.name AS Pod, p.namespace AS Namespace
ORDER BY p.name
```

### Find all ConfigMaps and their consumers
```cypher
MATCH (cm:ConfigMap)
OPTIONAL MATCH (p:Pod)-[:USES_CONFIG]->(cm)
RETURN cm.namespace AS Namespace, cm.name AS ConfigMap,
       count(p) AS ConsumerCount, collect(p.name) AS Consumers
ORDER BY cm.namespace, cm.name
```

### Find all Secrets and their consumers
```cypher
MATCH (s:Secret)
OPTIONAL MATCH (p:Pod)-[:USES_SECRET]->(s)
RETURN s.namespace AS Namespace, s.name AS Secret,
       count(p) AS ConsumerCount, collect(p.name) AS Consumers
ORDER BY s.namespace, s.name
```

### Find unused ConfigMaps
```cypher
MATCH (cm:ConfigMap)
WHERE NOT ()-[:USES_CONFIG]->(cm)
RETURN cm.namespace AS Namespace, cm.name AS ConfigMap
ORDER BY cm.namespace, cm.name
```

### Find unused Secrets
```cypher
MATCH (s:Secret)
WHERE NOT ()-[:USES_SECRET]->(s) AND NOT ()-[:USES_TLS_FROM]->(s)
RETURN s.namespace AS Namespace, s.name AS Secret
ORDER BY s.namespace, s.name
```

---

## Gateway API Queries

### List all Gateway Classes
```cypher
MATCH (gc:GatewayClass)
RETURN gc.name AS GatewayClass, gc.controller AS Controller
ORDER BY gc.name
```

### List all Gateways
```cypher
MATCH (g:Gateway)
RETURN g.name AS Gateway, g.namespace AS Namespace,
       g.gateway_class_name AS GatewayClass
ORDER BY g.namespace, g.name
```

### Find Gateways and their classes
```cypher
MATCH (g:Gateway)-[:IMPLEMENTED_BY]->(gc:GatewayClass)
RETURN g.name AS Gateway, g.namespace AS Namespace,
       gc.name AS GatewayClass, gc.controller AS Controller
ORDER BY g.namespace, g.name
```

### Find Gateway TLS configuration
```cypher
MATCH (g:Gateway)-[:USES_TLS_FROM]->(s:Secret)
RETURN g.name AS Gateway, g.namespace AS GatewayNamespace,
       s.name AS Secret, s.namespace AS SecretNamespace
ORDER BY g.namespace, g.name
```

### List all HTTPRoutes
```cypher
MATCH (hr:HTTPRoute)
RETURN hr.name AS HTTPRoute, hr.namespace AS Namespace
ORDER BY hr.namespace, hr.name
```

### Find HTTPRoutes attached to Gateway
```cypher
MATCH (hr:HTTPRoute)-[:ATTACHES_TO]->(g:Gateway)
RETURN hr.name AS HTTPRoute, hr.namespace AS RouteNamespace,
       g.name AS Gateway, g.namespace AS GatewayNamespace
ORDER BY g.namespace, g.name, hr.namespace, hr.name
```

### Find HTTPRoute backend services
```cypher
MATCH (hr:HTTPRoute)-[:FORWARDS_TO]->(s:Service)
RETURN hr.name AS HTTPRoute, hr.namespace AS RouteNamespace,
       s.name AS Service, s.namespace AS ServiceNamespace
ORDER BY hr.namespace, hr.name
```

### Find complete Gateway routing path
```cypher
MATCH (g:Gateway)<-[:ATTACHES_TO]-(hr:HTTPRoute)-[:FORWARDS_TO]->(s:Service)-[:SELECTS_PODS]->(p:Pod)
RETURN g.name AS Gateway, hr.name AS Route,
       s.name AS Service, count(p) AS PodCount
ORDER BY g.name, hr.name, s.name
```

### List all ReferenceGrants
```cypher
MATCH (rg:ReferenceGrant)
RETURN rg.name AS ReferenceGrant, rg.namespace AS Namespace
ORDER BY rg.namespace, rg.name
```

### Find cross-namespace routing with ReferenceGrants
```cypher
MATCH (hr:HTTPRoute)-[:PERMITTED_BY]->(rg:ReferenceGrant)-[:ALLOWS_ROUTE_TO]->(s:Service)
WHERE hr.namespace <> s.namespace
RETURN hr.name AS Route, hr.namespace AS RouteNamespace,
       rg.name AS Grant, s.name AS Service, s.namespace AS ServiceNamespace
ORDER BY hr.namespace, s.namespace
```

### Find all TLS/GRPC/TCP routes
```cypher
MATCH (r)
WHERE r:TLSRoute OR r:GRPCRoute OR r:TCPRoute OR r:UDPRoute
RETURN labels(r)[0] AS RouteType, r.name AS Route, r.namespace AS Namespace
ORDER BY RouteType, r.namespace, r.name
```

---

## Istio Queries

### List all Istio Gateways
```cypher
MATCH (ig:IstioGateway)
RETURN ig.name AS Gateway, ig.namespace AS Namespace
ORDER BY ig.namespace, ig.name
```

### Find Istio Gateway proxy pods
```cypher
MATCH (ig:IstioGateway)-[:SELECTS_PROXY]->(p:Pod)
RETURN ig.name AS Gateway, ig.namespace AS GatewayNamespace,
       p.name AS ProxyPod, p.namespace AS PodNamespace, p.status AS Status
ORDER BY ig.namespace, ig.name
```

### List all VirtualServices
```cypher
MATCH (vs:VirtualService)
RETURN vs.name AS VirtualService, vs.namespace AS Namespace
ORDER BY vs.namespace, vs.name
```

### Find VirtualService to Gateway attachments
```cypher
MATCH (vs:VirtualService)-[:ATTACHES_TO]->(ig:IstioGateway)
RETURN vs.name AS VirtualService, vs.namespace AS VSNamespace,
       ig.name AS Gateway, ig.namespace AS GatewayNamespace
ORDER BY ig.namespace, ig.name
```

### Find VirtualService routing to services
```cypher
MATCH (vs:VirtualService)-[:ROUTES_TRAFFIC_FOR]->(s:Service)
RETURN vs.name AS VirtualService, vs.namespace AS VSNamespace,
       s.name AS Service, s.namespace AS ServiceNamespace
ORDER BY vs.namespace, vs.name
```

### Find VirtualService with subset routing
```cypher
MATCH (vs:VirtualService)-[r:ROUTES_TO_SUBSET]->(dr:DestinationRule)
RETURN vs.name AS VirtualService, vs.namespace AS Namespace,
       dr.name AS DestinationRule, r.subset_name AS Subset, r.weight AS Weight
ORDER BY vs.namespace, vs.name
```

### List all DestinationRules
```cypher
MATCH (dr:DestinationRule)
RETURN dr.name AS DestinationRule, dr.namespace AS Namespace
ORDER BY dr.namespace, dr.name
```

### Find DestinationRule policies for services
```cypher
MATCH (dr:DestinationRule)-[:DEFINES_POLICY_FOR]->(s:Service)
RETURN dr.name AS DestinationRule, dr.namespace AS DRNamespace,
       s.name AS Service, s.namespace AS ServiceNamespace
ORDER BY dr.namespace, dr.name
```

### Find DestinationRule subset pod selections
```cypher
MATCH (dr:DestinationRule)-[r:SELECTS_SUBSET_PODS]->(p:Pod)
RETURN dr.name AS DestinationRule, dr.namespace AS Namespace,
       r.subset_name AS Subset, count(p) AS PodCount,
       collect(p.name) AS Pods
ORDER BY dr.namespace, dr.name, Subset
```

### Complete Istio traffic routing path
```cypher
MATCH path = (ig:IstioGateway)<-[:ATTACHES_TO]-(vs:VirtualService)-[:ROUTES_TRAFFIC_FOR]->(s:Service)-[:SELECTS_PODS]->(p:Pod)
RETURN ig.name AS Gateway, vs.name AS VirtualService,
       s.name AS Service, count(p) AS PodCount
ORDER BY ig.name, vs.name
```

### List all ServiceEntries
```cypher
MATCH (se:ServiceEntry)
RETURN se.name AS ServiceEntry, se.namespace AS Namespace
ORDER BY se.namespace, se.name
```

### List all Sidecars
```cypher
MATCH (sc:Sidecar)
RETURN sc.name AS Sidecar, sc.namespace AS Namespace
ORDER BY sc.namespace, sc.name
```

### List all AuthorizationPolicies
```cypher
MATCH (ap:AuthorizationPolicy)
RETURN ap.name AS AuthorizationPolicy, ap.namespace AS Namespace,
       ap.action AS Action
ORDER BY ap.namespace, ap.name
```

### Find AuthorizationPolicy applied to pods
```cypher
MATCH (ap:AuthorizationPolicy)-[:APPLIES_TO]->(p:Pod)
RETURN ap.name AS AuthorizationPolicy, ap.namespace AS Namespace,
       ap.action AS Action, count(p) AS PodCount
ORDER BY ap.namespace, ap.name
```

### List all PeerAuthentications
```cypher
MATCH (pa:PeerAuthentication)
RETURN pa.name AS PeerAuthentication, pa.namespace AS Namespace,
       pa.mtls_mode AS MTLSMode
ORDER BY pa.namespace, pa.name
```

### Find PeerAuthentication applied to pods
```cypher
MATCH (pa:PeerAuthentication)-[:APPLIES_TO]->(p:Pod)
RETURN pa.name AS PeerAuthentication, pa.namespace AS Namespace,
       pa.mtls_mode AS MTLSMode, count(p) AS PodCount
ORDER BY pa.namespace, pa.name
```

### List all RequestAuthentications
```cypher
MATCH (ra:RequestAuthentication)
RETURN ra.name AS RequestAuthentication, ra.namespace AS Namespace
ORDER BY ra.namespace, ra.name
```

### Find RequestAuthentication applied to pods
```cypher
MATCH (ra:RequestAuthentication)-[:APPLIES_TO]->(p:Pod)
RETURN ra.name AS RequestAuthentication, ra.namespace AS Namespace,
       count(p) AS PodCount
ORDER BY ra.namespace, ra.name
```

---

## Troubleshooting & Debugging

### Find pods with high restart counts
```cypher
MATCH (p:Pod)-[:CONTAINS]->(c:Container)
WHERE c.restart_count > 5
RETURN p.namespace AS Namespace, p.name AS Pod,
       c.name AS Container, c.restart_count AS Restarts
ORDER BY c.restart_count DESC
```

### Find pods not ready
```cypher
MATCH (p:Pod)-[:CONTAINS]->(c:Container)
WHERE c.ready = false
RETURN p.namespace AS Namespace, p.name AS Pod,
       c.name AS Container, p.status AS PodStatus
ORDER BY p.namespace, p.name
```

### Find failed pods
```cypher
MATCH (p:Pod)
WHERE p.status IN ['Failed', 'CrashLoopBackOff', 'Error']
RETURN p.namespace AS Namespace, p.name AS Pod,
       p.status AS Status, p.node_name AS Node
ORDER BY p.namespace, p.name
```

### Find events related to a specific pod
```cypher
MATCH (e:K8sEvent)-[:INVOLVES]->(p:Pod {name: 'my-pod', namespace: 'default'})
RETURN e.type AS Type, e.reason AS Reason, e.message AS Message,
       e.last_timestamp AS Time
ORDER BY e.last_timestamp DESC
```

### Find all warning events by namespace
```cypher
MATCH (e:K8sEvent)-[:INVOLVES]->(resource)
WHERE e.type = 'Warning'
RETURN e.namespace AS Namespace, labels(resource)[0] AS ResourceType,
       resource.name AS Resource, e.reason AS Reason, e.message AS Message,
       e.last_timestamp AS Time
ORDER BY e.last_timestamp DESC
LIMIT 50
```

### Find disconnected services (no pods)
```cypher
MATCH (s:Service)
WHERE NOT (s)-[:SELECTS_PODS]->(:Pod)
RETURN s.namespace AS Namespace, s.name AS Service
ORDER BY s.namespace, s.name
```

### Find pods without owner (not managed by controller)
```cypher
MATCH (p:Pod)
WHERE NOT ()-[:MANAGES]->(p)
RETURN p.namespace AS Namespace, p.name AS Pod, p.status AS Status
ORDER BY p.namespace, p.name
```

### Find resource bottlenecks on nodes
```cypher
MATCH (n:Node)<-[:SCHEDULED_ON]-(p:Pod)
WITH n, count(p) AS PodCount, sum(p.cpu_request) AS TotalCPU, sum(p.memory_request) AS TotalMemory
RETURN n.name AS Node, PodCount, TotalCPU, TotalMemory,
       n.cpu_capacity AS CPUCapacity, n.memory_capacity AS MemoryCapacity
ORDER BY PodCount DESC
```

### Find image pull errors
```cypher
MATCH (e:K8sEvent)-[:INVOLVES]->(p:Pod)
WHERE e.reason CONTAINS 'ImagePull' OR e.reason CONTAINS 'ErrImage'
RETURN p.namespace AS Namespace, p.name AS Pod,
       e.reason AS Reason, e.message AS Message, e.last_timestamp AS Time
ORDER BY e.last_timestamp DESC
```

### Find deployment rollout issues
```cypher
MATCH (d:Deployment)
WHERE d.replicas <> d.ready_replicas
RETURN d.namespace AS Namespace, d.name AS Deployment,
       d.replicas AS Desired, d.ready_replicas AS Ready,
       d.available_replicas AS Available
ORDER BY d.namespace, d.name
```

---

## Performance & Analytics

### Count pods by namespace
```cypher
MATCH (p:Pod)-[:IN_NAMESPACE]->(ns:Namespace)
RETURN ns.name AS Namespace, count(p) AS PodCount
ORDER BY PodCount DESC
```

### Count resources by namespace
```cypher
MATCH (r)-[:IN_NAMESPACE]->(ns:Namespace)
RETURN ns.name AS Namespace, labels(r)[0] AS ResourceType, count(r) AS Count
ORDER BY ns.name, Count DESC
```

### Find most used images
```cypher
MATCH (c:Container)
RETURN c.image AS Image, count(c) AS Count
ORDER BY Count DESC
LIMIT 20
```

### Find nodes by pod density
```cypher
MATCH (n:Node)
OPTIONAL MATCH (p:Pod)-[:SCHEDULED_ON]->(n)
RETURN n.name AS Node, count(p) AS PodCount
ORDER BY PodCount DESC
```

### Calculate resource utilization by namespace
```cypher
MATCH (p:Pod)-[:IN_NAMESPACE]->(ns:Namespace)
WITH ns.name AS Namespace,
     sum(p.cpu_request) AS TotalCPURequest,
     sum(p.memory_request) AS TotalMemoryRequest,
     sum(p.cpu_limit) AS TotalCPULimit,
     sum(p.memory_limit) AS TotalMemoryLimit
RETURN Namespace, TotalCPURequest, TotalMemoryRequest,
       TotalCPULimit, TotalMemoryLimit
ORDER BY TotalCPURequest DESC
```

### Find service mesh adoption (pods with sidecars)
```cypher
MATCH (p:Pod)-[:CONTAINS]->(c:Container)
WHERE c.name = 'istio-proxy' OR c.image CONTAINS 'istio/proxyv2'
WITH count(DISTINCT p) AS PodsWithSidecar
MATCH (allPods:Pod)
WITH PodsWithSidecar, count(allPods) AS TotalPods
RETURN PodsWithSidecar, TotalPods,
       round(100.0 * PodsWithSidecar / TotalPods) AS PercentageWithSidecar
```

### Count routes by gateway
```cypher
MATCH (g:Gateway)<-[:ATTACHES_TO]-(route)
WHERE route:HTTPRoute OR route:GRPCRoute OR route:TCPRoute
RETURN g.name AS Gateway, g.namespace AS Namespace,
       labels(route)[0] AS RouteType, count(route) AS RouteCount
ORDER BY g.namespace, g.name, RouteType
```

### Find namespace resource topology
```cypher
MATCH (ns:Namespace {name: 'production'})
OPTIONAL MATCH (d:Deployment)-[:IN_NAMESPACE]->(ns)
OPTIONAL MATCH (s:Service)-[:IN_NAMESPACE]->(ns)
OPTIONAL MATCH (p:Pod)-[:IN_NAMESPACE]->(ns)
OPTIONAL MATCH (cm:ConfigMap)-[:IN_NAMESPACE]->(ns)
OPTIONAL MATCH (sec:Secret)-[:IN_NAMESPACE]->(ns)
RETURN ns.name AS Namespace,
       count(DISTINCT d) AS Deployments,
       count(DISTINCT s) AS Services,
       count(DISTINCT p) AS Pods,
       count(DISTINCT cm) AS ConfigMaps,
       count(DISTINCT sec) AS Secrets
```

---

## Data Cleanup

### Delete nodes updated before a specific time
```cypher
// Find nodes not updated in the last 7 days (timestamp is in seconds)
MATCH (n)
WHERE n.updated_at < (timestamp() / 1000) - (7 * 24 * 60 * 60)
RETURN labels(n)[0] AS Type, count(n) AS Count
```

### Delete specific node and its relationships
```cypher
MATCH (n:Pod {id: 'Pod/default/old-pod'})
DETACH DELETE n
```

### Delete all nodes of a specific type
```cypher
// CAUTION: This will delete all nodes of the specified type
MATCH (n:K8sEvent)
DETACH DELETE n
```

### Delete orphaned relationships
```cypher
// Find and delete relationships where either endpoint doesn't exist
MATCH (a)-[r]->(b)
WHERE a IS NULL OR b IS NULL
DELETE r
```

### Clear all data (DANGEROUS!)
```cypher
// WARNING: This will delete EVERYTHING in the database
MATCH (n)
DETACH DELETE n
```

---

## Advanced Queries

### Find circular dependencies
```cypher
MATCH path = (a)-[:DEPENDS_ON*2..5]->(a)
RETURN nodes(path)
LIMIT 10
```

### Find all resources in a namespace with their relationships
```cypher
MATCH (ns:Namespace {name: 'production'})<-[:IN_NAMESPACE]-(r)
OPTIONAL MATCH (r)-[rel]-(connected)
RETURN r, rel, connected
LIMIT 100
```

### Export namespace topology
```cypher
MATCH (ns:Namespace {name: 'production'})<-[:IN_NAMESPACE]-(r)
RETURN labels(r)[0] AS Type, r.name AS Name,
       collect(DISTINCT [(r)-[rel]->(other) | 
         {relationship: type(rel), to: labels(other)[0], name: other.name}
       ]) AS Relationships
```

### Find critical path (most connected resources)
```cypher
MATCH (n)
WITH n, size((n)--()) AS degree
WHERE degree > 10
MATCH (n)-[r]-(m)
RETURN labels(n)[0] AS Type, n.name AS Name, n.namespace AS Namespace,
       degree AS ConnectionCount
ORDER BY degree DESC
LIMIT 20
```

### Find blast radius (what would be affected if a service fails)
```cypher
MATCH path = (s:Service {name: 'critical-service', namespace: 'production'})-[*1..3]-(affected)
RETURN DISTINCT labels(affected)[0] AS AffectedType,
       affected.name AS Name,
       affected.namespace AS Namespace,
       length(path) AS Distance
ORDER BY Distance, AffectedType
```

### Gateway API and Istio coexistence check
```cypher
MATCH (g:Gateway)
OPTIONAL MATCH (ig:IstioGateway)
RETURN 'Gateway API Gateways' AS Type, count(g) AS Count
UNION
MATCH (ig:IstioGateway)
RETURN 'Istio Gateways' AS Type, count(ig) AS Count
```

### Find pods with most configuration dependencies
```cypher
MATCH (p:Pod)
OPTIONAL MATCH (p)-[:USES_CONFIG]->(cm:ConfigMap)
OPTIONAL MATCH (p)-[:USES_SECRET]->(s:Secret)
OPTIONAL MATCH (p)-[:MOUNTS]->(pvc:PersistentVolumeClaim)
WITH p, count(DISTINCT cm) AS ConfigMaps, count(DISTINCT s) AS Secrets, count(DISTINCT pvc) AS PVCs
WHERE ConfigMaps + Secrets + PVCs > 0
RETURN p.namespace AS Namespace, p.name AS Pod,
       ConfigMaps, Secrets, PVCs,
       ConfigMaps + Secrets + PVCs AS TotalDependencies
ORDER BY TotalDependencies DESC
```

---

## Distributed Tracing Queries

### Find all traces for a service
```cypher
MATCH (t:Trace)
WHERE t.root_service = 'checkout' OR t.services_involved CONTAINS 'checkout'
RETURN t.trace_id AS TraceID, t.start_time AS StartTime,
       t.duration_ms AS DurationMs, t.has_errors AS HasErrors,
       t.span_count AS SpanCount
ORDER BY t.start_time DESC
LIMIT 10
```

### Analyze Span Kinds distribution
```cypher
MATCH (s:Span)
RETURN s.span_kind AS SpanKind, count(s) AS Count
ORDER BY Count DESC
```

### Find CLIENT spans (outgoing calls)
```cypher
MATCH (s:Span)
WHERE s.span_kind = 'client'
RETURN s.service_name AS Service, s.operation_name AS Operation,
       s.server_address AS TargetAddress, s.duration_ms AS DurationMs
ORDER BY s.start_time DESC
LIMIT 20
```

### Find SERVER spans (incoming requests)
```cypher
MATCH (s:Span)
WHERE s.span_kind = 'server'
RETURN s.service_name AS Service, s.operation_name AS Operation,
       s.http_request_method AS Method, s.url_path AS Path,
       s.duration_ms AS DurationMs
ORDER BY s.start_time DESC
LIMIT 20
```

### Find slowest spans in recent traces
```cypher
MATCH (s:Span)
WHERE datetime(s.start_time) > datetime() - duration('PT1H')
RETURN s.service_name AS Service, s.operation_name AS Operation,
       s.duration_ms AS DurationMs, s.trace_id AS TraceID,
       s.error AS HasError, s.start_time AS StartTime
ORDER BY s.duration_ms DESC
LIMIT 20
```

### Find all error spans
```cypher
MATCH (s:Span)
WHERE s.error = true
  AND datetime(s.start_time) > datetime() - duration('PT1H')
RETURN s.service_name AS Service, s.service_namespace AS Namespace,
       s.operation_name AS Operation, s.error_message AS ErrorMessage,
       s.http_status_code AS StatusCode, s.trace_id AS TraceID
ORDER BY s.start_time DESC
LIMIT 50
```

### Trace a specific request by trace ID
```cypher
MATCH (t:Trace {trace_id: 'e7a3198ca98778aa2f9460e8a9e58c1b'})-[:CONTAINS_SPAN]->(s:Span)
OPTIONAL MATCH (s)-[:PARENT_OF]->(child:Span)
RETURN s.operation_name AS Operation, s.service_name AS Service,
       s.duration_ms AS DurationMs, s.error AS Error,
       collect(child.operation_name) AS ChildOperations
ORDER BY s.start_time
```

### Visualize span hierarchy for a trace
```cypher
MATCH path = (t:Trace {trace_id: 'e7a3198ca98778aa2f9460e8a9e58c1b'})-[:CONTAINS_SPAN]->(root:Span)
WHERE root.parent_span_id = ''
MATCH hierarchy = (root)-[:PARENT_OF*0..]->(descendant:Span)
RETURN hierarchy
```

### Compare static vs runtime service dependencies
```cypher
// Configured dependencies (static)
MATCH (s1:Service {name: 'checkout'})-[:FORWARDS_TO]->(s2:Service)
WITH collect(s2.name) AS ConfiguredBackends

// Runtime dependencies (observed)
MATCH (s1:Service {name: 'checkout'})-[:CALLS]->(s2:Service)
WITH ConfiguredBackends, collect(s2.name) AS ActualBackends

RETURN ConfiguredBackends, ActualBackends,
       [backend IN ActualBackends WHERE NOT backend IN ConfiguredBackends] AS UndocumentedBackends,
       [backend IN ConfiguredBackends WHERE NOT backend IN ActualBackends] AS UnusedBackends
```

### Find services with high error rates
```cypher
MATCH (s:Service)-[c:CALLS|FAILED_CALL_TO]->(target:Service)
WITH s, target, 
     count(CASE WHEN type(c) = 'FAILED_CALL_TO' THEN 1 END) AS Failures,
     count(c) AS Total
WHERE Failures > 0
RETURN s.name AS SourceService, s.namespace AS SourceNamespace,
       target.name AS TargetService, target.namespace AS TargetNamespace,
       Failures, Total,
       round(toFloat(Failures) / Total * 100, 2) AS ErrorRate
ORDER BY ErrorRate DESC, Failures DESC
LIMIT 20
```

### Find error propagation paths
```cypher
MATCH path = (failing:Service)-[:CALLS|FAILED_CALL_TO*1..3]->(downstream:Service)
WHERE EXISTS((failing)-[:FAILED_CALL_TO]->())
RETURN failing.name AS FailingService, failing.namespace AS Namespace,
       [node IN nodes(path) | node.name] AS PropagationPath,
       length(path) AS HopsAway
ORDER BY length(path)
LIMIT 20
```

### Verify Span to Service relationships exist
```cypher
MATCH (s:Span)-[r:ORIGINATED_FROM]->(svc:Service)
RETURN s.service_name AS SpanService, s.service_namespace AS SpanNamespace,
       type(r) AS Relationship, 
       svc.name AS ServiceName, svc.namespace AS ServiceNamespace
LIMIT 20
```

### Count ORIGINATED_FROM relationships
```cypher
MATCH (s:Span)-[r:ORIGINATED_FROM]->(svc:Service)
RETURN count(r) AS TotalOriginatedFromLinks
```

### List all Spans and check which have ORIGINATED_FROM links
```cypher
MATCH (s:Span)
OPTIONAL MATCH (s)-[r:ORIGINATED_FROM]->(svc:Service)
RETURN s.span_id AS SpanID, s.service_name AS SpanService, 
       s.service_namespace AS SpanNamespace,
       CASE WHEN r IS NOT NULL THEN 'YES' ELSE 'NO' END AS HasLink,
       svc.name AS LinkedServiceName
LIMIT 20
```

### Link trace spans to Kubernetes resources
```cypher
MATCH (s:Span {service_name: 'checkout'})-[:ORIGINATED_FROM]->(svc:Service)
MATCH (svc)-[:SELECTS_PODS]->(pod:Pod)
RETURN s.trace_id AS TraceID, s.operation_name AS Operation,
       svc.name AS Service, svc.namespace AS Namespace,
       pod.name AS Pod, pod.node_name AS Node, pod.status AS PodStatus
LIMIT 20
```

### Find services calling across namespaces (from CLIENT spans)
```cypher
MATCH (s1:Service)-[c:CALLS]->(s2:Service)
WHERE s1.namespace <> s2.namespace
  AND c.source = 'trace_observed'
RETURN s1.namespace AS SourceNamespace, s1.name AS SourceService,
       s2.namespace AS TargetNamespace, s2.name AS TargetService,
       c.protocol AS Protocol, c.last_observed AS LastObserved,
       c.error AS HasErrors
ORDER BY c.last_observed DESC
```

### Verify CALLS edges are only from CLIENT spans
```cypher
MATCH (s1:Service)-[c:CALLS]->(s2:Service)
WHERE c.source = 'trace_observed'
RETURN s1.name AS FromService, s2.name AS ToService,
       count(c) AS CallCount
ORDER BY CallCount DESC
LIMIT 10
```

### Identify slow service calls
```cypher
MATCH (s1:Service)-[c:CALLS]->(s2:Service)
WHERE c.duration_ms > 100
  AND datetime(c.last_observed) > datetime() - duration('PT1H')
RETURN s1.name AS Caller, s2.name AS Callee,
       c.duration_ms AS LatencyMs, c.protocol AS Protocol,
       c.last_observed AS LastObserved
ORDER BY c.duration_ms DESC
LIMIT 20
```

### Find services with failed downstream calls
```cypher
MATCH (s:Service)-[:FAILED_CALL_TO]->(failed:Service)
RETURN s.name AS Service, s.namespace AS Namespace,
       collect(failed.name) AS FailedDownstreams,
       count(failed) AS FailureCount
ORDER BY FailureCount DESC
```

### Trace impact: what's affected when a service fails
```cypher
MATCH (failing:Service {name: 'shipping', namespace: 'sf-orders'})
OPTIONAL MATCH (upstream:Service)-[:CALLS*1..3]->(failing)
OPTIONAL MATCH (failing)<-[:SELECTS_PODS]-(pod:Pod)
OPTIONAL MATCH (route:HTTPRoute)-[:FORWARDS_TO]->(failing)
RETURN upstream.name AS UpstreamService,
       upstream.namespace AS UpstreamNamespace,
       pod.name AS AffectedPod,
       route.name AS AffectedRoute
```

### Cleanup old spans (maintenance query)
```cypher
MATCH (s:Span)
WHERE datetime(s.start_time) < datetime() - duration('PT1H')
DETACH DELETE s
RETURN count(s) AS DeletedSpans
```

---

## Placeholder Node Queries

### Find all placeholder nodes
```cypher
MATCH (n)
WHERE n.placeholder = true
RETURN labels(n)[0] AS Type, n.id AS ID, n.updated_at AS UpdatedAt
ORDER BY n.updated_at DESC
LIMIT 100
```

### Count placeholder nodes by type
```cypher
MATCH (n)
WHERE n.placeholder = true
RETURN labels(n)[0] AS Type, count(n) AS Count
ORDER BY Count DESC
```

### Find placeholder nodes older than 1 hour
```cypher
MATCH (n)
WHERE n.placeholder = true 
  AND n.updated_at < (timestamp() / 1000) - 3600
RETURN labels(n)[0] AS Type, n.id AS ID, 
       (timestamp() / 1000 - n.updated_at) / 60 AS AgeMinutes
ORDER BY AgeMinutes DESC
```

### Find placeholder nodes with relationships
```cypher
MATCH (n)-[r]-(m)
WHERE n.placeholder = true
RETURN labels(n)[0] AS PlaceholderType, n.id AS PlaceholderID,
       type(r) AS Relationship, labels(m)[0] AS ConnectedType, m.id AS ConnectedID
LIMIT 50
```

### Find orphaned placeholders (no relationships)
```cypher
MATCH (n)
WHERE n.placeholder = true AND NOT (n)-[]-()
RETURN labels(n)[0] AS Type, n.id AS ID, n.updated_at AS UpdatedAt
ORDER BY n.updated_at
```

### Exclude placeholder nodes from query results
```cypher
// Example: Get all services, excluding placeholders
MATCH (s:Service)
WHERE s.placeholder <> true OR s.placeholder IS NULL
RETURN s.name AS Service, s.namespace AS Namespace
ORDER BY s.namespace, s.name
```

### Include placeholder status in results
```cypher
// Example: Get all ConfigMaps with placeholder flag
MATCH (cm:ConfigMap)
RETURN cm.name AS ConfigMap, cm.namespace AS Namespace,
       COALESCE(cm.placeholder, false) AS IsPlaceholder
ORDER BY IsPlaceholder DESC, cm.namespace, cm.name
```

### Find nodes referencing placeholder nodes
```cypher
MATCH (n)-[r]->(p)
WHERE p.placeholder = true
RETURN labels(n)[0] AS Type, n.name AS Name, n.namespace AS Namespace,
       type(r) AS Relationship,
       labels(p)[0] AS PlaceholderType, p.id AS PlaceholderID
ORDER BY PlaceholderType, p.id
LIMIT 50
```

### Find pods referencing placeholder ConfigMaps or Secrets
```cypher
MATCH (p:Pod)-[r:USES_CONFIG|USES_SECRET]->(resource)
WHERE resource.placeholder = true
RETURN p.namespace AS Namespace, p.name AS Pod,
       type(r) AS Relationship, labels(resource)[0] AS ResourceType,
       resource.id AS MissingResource
ORDER BY p.namespace, p.name
```

### Manually cleanup specific placeholder nodes
```cypher
// Remove placeholder nodes older than 2 hours with no relationships
MATCH (n)
WHERE n.placeholder = true 
  AND n.updated_at < (timestamp() / 1000) - 7200
  AND NOT (n)-[]-()
DETACH DELETE n
RETURN count(n) AS DeletedCount
```

### Track placeholder resolution rate
```cypher
// Count total nodes vs placeholder nodes
MATCH (n)
WITH count(n) AS TotalNodes
MATCH (p)
WHERE p.placeholder = true
WITH TotalNodes, count(p) AS PlaceholderNodes
RETURN TotalNodes, PlaceholderNodes,
       round(toFloat(PlaceholderNodes) / TotalNodes * 100, 2) AS PlaceholderPercentage
```

---

## Index Management

### List all indexes
```cypher
SHOW INDEXES
```

### Create index for better performance (already done by the system)
```cypher
CREATE INDEX IF NOT EXISTS FOR (n:Pod) ON (n.id)
```

### Drop index
```cypher
DROP INDEX index_name IF EXISTS
```

---

## Notes

1. **Performance**: For large graphs, always use `LIMIT` clauses to prevent overwhelming queries
2. **IDs**: All nodes use the format `Kind/Namespace/Name` or `Kind/Name` for cluster-scoped resources
3. **Labels**: Labels are stored as JSON strings in the `labels` property
4. **Timestamps**: The `updated_at` property contains Unix timestamps in seconds
5. **Relationships**: All relationships are directional; use `-[r]-` for bidirectional queries
6. **Namespaces**: Cross-namespace queries are common with Gateway API and Istio resources

## Common Patterns

### Pattern: Find resource by name across all namespaces
```cypher
MATCH (n)
WHERE n.name = 'my-resource'
RETURN labels(n)[0] AS Type, n.namespace AS Namespace, n
```

### Pattern: Find all resources in namespace
```cypher
MATCH (r)-[:IN_NAMESPACE]->(ns:Namespace {name: 'production'})
RETURN labels(r)[0] AS Type, r.name AS Name
ORDER BY Type, Name
```

### Pattern: Trace request path
```cypher
MATCH path = (gateway)-[:ATTACHES_TO|FORWARDS_TO|SELECTS_PODS*]->(pod:Pod)
WHERE (gateway:Gateway OR gateway:IstioGateway) AND gateway.name = 'my-gateway'
RETURN path
LIMIT 10
```

### Pattern: Find resource owners
```cypher
MATCH path = (owner)-[:MANAGES*]->(resource {name: 'my-pod', namespace: 'default'})
RETURN path
```

---

**Generated for**: Kubernetes Knowledge Base (kkbase)
**Neo4j Version**: 5.x
**Last Updated**: 2025-10-24

