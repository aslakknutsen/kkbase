# Graph Schema Reference

This document describes all node types, edge types, and their properties in the kkbase knowledge graph.

## Resource Identification

All Kubernetes resources use a consistent ID format:

### Namespaced Resources
Format: `Type/namespace/name`

Examples:
- `Pod/default/nginx-abc123`
- `Service/kube-system/metrics-server`
- `HTTPRoute/store-frontend/api-route`
- `ConfigMap/production/app-config`

### Cluster-Scoped Resources
Format: `Type/name`

Examples:
- `Node/worker-node-1`
- `GatewayClass/istio`
- `PersistentVolume/pv-nfs-001`
- `Namespace/default`

### Container Resources
Format: `Container/namespace/pod-name/container-name`

Example:
- `Container/default/nginx-abc123/app`

## Node Types

### Core Kubernetes Resources

#### Compute & Infrastructure

**Cluster**
- Represents the entire Kubernetes cluster
- Properties: `name`, `version`, `cloud_provider`

**Node**
- Worker or master machines in the cluster
- Properties: `name`, `status`, `internal_ip`, `external_ip`, `cpu_capacity`, `memory_capacity`, `conditions`, `labels`

**Namespace**
- Logical isolation boundary
- Properties: `name`, `status`, `labels`, `annotations`

#### Workload Resources

**Pod**
- Smallest deployable unit
- Properties: `name`, `namespace`, `uid`, `status`, `ip`, `node_name`, `host_ip`, `phase`, `qos_class`, `restart_policy`, `labels`, `annotations`, `created`, `started`

**Container**
- Individual container within a Pod
- Properties: `name`, `image`, `image_id`, `ports`, `restart_count`, `ready`, `started`, `exit_code`, `reason`, `message`

**Deployment**
- Manages ReplicaSets for declarative updates
- Properties: `name`, `namespace`, `uid`, `replicas`, `ready_replicas`, `available_replicas`, `updated_replicas`, `labels`, `annotations`, `created`, `generation`

**ReplicaSet**
- Ensures specified number of pod replicas
- Properties: `name`, `namespace`, `uid`, `replicas`, `ready_replicas`, `available_replicas`, `labels`, `annotations`, `created`

**StatefulSet**
- Manages stateful applications
- Properties: `name`, `namespace`, `uid`, `replicas`, `ready_replicas`, `labels`, `annotations`, `created`

**DaemonSet**
- Ensures pods run on all/selected nodes
- Properties: `name`, `namespace`, `uid`, `desired_number_scheduled`, `current_number_scheduled`, `number_ready`, `labels`, `annotations`, `created`

**Job**
- Run-to-completion workloads
- Properties: `name`, `namespace`, `uid`, `completions`, `parallelism`, `active`, `succeeded`, `failed`, `labels`, `annotations`, `created`

#### Networking Resources

**Service**
- Exposes applications running on Pods
- Properties: `name`, `namespace`, `uid`, `type`, `cluster_ip`, `external_ips`, `ports`, `selector`, `session_affinity`, `labels`, `annotations`, `created`

**Ingress**
- HTTP/HTTPS routing to services
- Properties: `name`, `namespace`, `uid`, `class_name`, `rules`, `tls`, `labels`, `annotations`, `created`

**Endpoint**
- IP addresses and ports for a Service
- Properties: `name`, `namespace`, `uid`, `subsets`, `labels`, `annotations`, `created`

**NetworkPolicy**
- Network traffic rules for Pods
- Properties: `name`, `namespace`, `uid`, `pod_selector`, `policy_types`, `ingress`, `egress`, `labels`, `annotations`, `created`

#### Storage Resources

**PersistentVolume (PV)**
- Cluster-scoped storage resource
- Properties: `name`, `capacity`, `access_modes`, `reclaim_policy`, `status`, `storage_class`, `volume_mode`, `labels`, `annotations`, `created`

**PersistentVolumeClaim (PVC)**
- Request for storage by a user
- Properties: `name`, `namespace`, `uid`, `status`, `capacity`, `access_modes`, `storage_class`, `volume_name`, `labels`, `annotations`, `created`

**StorageClass**
- Storage provisioning template
- Properties: `name`, `provisioner`, `parameters`, `reclaim_policy`, `volume_binding_mode`, `allow_volume_expansion`, `labels`, `annotations`, `created`

#### Configuration Resources

**ConfigMap**
- Non-confidential configuration data
- Properties: `name`, `namespace`, `uid`, `data_keys`, `labels`, `annotations`, `created`

**Secret**
- Sensitive information storage
- Properties: `name`, `namespace`, `uid`, `type`, `data_keys`, `labels`, `annotations`, `created`

#### Observability Resources

**K8sEvent**
- Kubernetes API events
- Properties: `name`, `namespace`, `uid`, `type`, `reason`, `message`, `involved_object_kind`, `involved_object_name`, `first_timestamp`, `last_timestamp`, `count`, `source`, `created`

### Gateway API Resources

**GatewayClass**
- Gateway implementation template (cluster-scoped)
- Properties: `name`, `controller_name`, `description`, `accepted`, `status_message`, `labels`, `annotations`, `created`

**Gateway**
- Load balancer configuration
- Properties: `name`, `namespace`, `uid`, `gateway_class_name`, `listeners`, `addresses`, `accepted`, `programmed`, `labels`, `annotations`, `created`

**HTTPRoute**
- HTTP/HTTPS routing rules
- Properties: `name`, `namespace`, `uid`, `hostnames`, `parent_refs`, `rules`, `rule_count`, `accepted`, `labels`, `annotations`, `created`

**GRPCRoute**
- gRPC routing rules
- Properties: `name`, `namespace`, `uid`, `hostnames`, `parent_refs`, `rules`, `rule_count`, `accepted`, `labels`, `annotations`, `created`

**TCPRoute**
- TCP routing rules
- Properties: `name`, `namespace`, `uid`, `parent_refs`, `rules`, `rule_count`, `accepted`, `labels`, `annotations`, `created`

**UDPRoute**
- UDP routing rules
- Properties: `name`, `namespace`, `uid`, `parent_refs`, `rules`, `rule_count`, `accepted`, `labels`, `annotations`, `created`

**TLSRoute**
- TLS routing rules
- Properties: `name`, `namespace`, `uid`, `hostnames`, `parent_refs`, `rules`, `rule_count`, `accepted`, `labels`, `annotations`, `created`

**ReferenceGrant**
- Cross-namespace reference permissions
- Properties: `name`, `namespace`, `uid`, `from`, `to`, `labels`, `annotations`, `created`

### Istio Resources

**IstioGateway**
- Istio gateway configuration
- Properties: `name`, `namespace`, `uid`, `selector`, `servers`, `labels`, `annotations`, `created`

**VirtualService**
- Traffic routing rules
- Properties: `name`, `namespace`, `uid`, `hosts`, `gateways`, `http_routes`, `tcp_routes`, `tls_routes`, `labels`, `annotations`, `created`

**DestinationRule**
- Post-routing policies and subsets
- Properties: `name`, `namespace`, `uid`, `host`, `subsets`, `traffic_policy`, `labels`, `annotations`, `created`

**ServiceEntry**
- External service registration
- Properties: `name`, `namespace`, `uid`, `hosts`, `location`, `resolution`, `endpoints`, `labels`, `annotations`, `created`

**Sidecar**
- Sidecar proxy configuration
- Properties: `name`, `namespace`, `uid`, `workload_selector`, `egress_hosts`, `ingress_listeners`, `egress_listeners`, `labels`, `annotations`, `created`

**AuthorizationPolicy**
- Access control policies
- Properties: `name`, `namespace`, `uid`, `selector`, `action`, `rules`, `labels`, `annotations`, `created`

**PeerAuthentication**
- Mutual TLS configuration
- Properties: `name`, `namespace`, `uid`, `selector`, `mtls_mode`, `port_level_mtls`, `labels`, `annotations`, `created`

**RequestAuthentication**
- JWT authentication configuration
- Properties: `name`, `namespace`, `uid`, `selector`, `jwt_rules`, `labels`, `annotations`, `created`

## Edge Types

### Core Kubernetes Relationships

**SCHEDULED_ON**
- From: `Pod`
- To: `Node`
- Description: Pod is scheduled on a specific node
- Properties: None

**MANAGES**
- From: `Deployment`, `ReplicaSet`, `StatefulSet`, `DaemonSet`
- To: `ReplicaSet`, `Pod`
- Description: Controller manages child resources
- Properties: None

**CONTAINS**
- From: `Pod`
- To: `Container`
- Description: Pod contains one or more containers
- Properties: None

**SELECTS_PODS**
- From: `Service`
- To: `Pod`
- Description: Service selects pods based on labels
- Properties: None

**ROUTES_TO**
- From: `Ingress`
- To: `Service`
- Description: Ingress routes traffic to service
- Properties: `path`, `path_type`

**MOUNTS**
- From: `Pod`
- To: `PersistentVolumeClaim`
- Description: Pod mounts a PVC
- Properties: `mount_path`, `read_only`, `volume_name`

**BOUND_TO**
- From: `PersistentVolumeClaim`
- To: `PersistentVolume`
- Description: PVC is bound to a PV
- Properties: None

**PROVISIONED_BY**
- From: `PersistentVolume`
- To: `StorageClass`
- Description: PV was provisioned by a storage class
- Properties: None

**USES_CONFIG**
- From: `Pod`
- To: `ConfigMap`
- Description: Pod uses configuration from ConfigMap
- Properties: `usage_type` (env, volume, envFrom)

**USES_SECRET**
- From: `Pod`
- To: `Secret`
- Description: Pod uses secrets
- Properties: `usage_type` (env, volume, envFrom)

**IN_NAMESPACE**
- From: Any namespaced resource
- To: `Namespace`
- Description: Resource belongs to a namespace
- Properties: None

**INVOLVES**
- From: `K8sEvent`
- To: Any resource
- Description: Event involves a specific resource
- Properties: None

### Gateway API Relationships

**IMPLEMENTED_BY**
- From: `Gateway`
- To: `GatewayClass`
- Description: Gateway is implemented by a controller
- Properties: None

**ATTACHES_TO**
- From: `HTTPRoute`, `GRPCRoute`, `TCPRoute`, `UDPRoute`, `TLSRoute`
- To: `Gateway`
- Description: Route attaches to gateway listener
- Properties: `listener_name`, `port`, `section_name`

**FORWARDS_TO**
- From: `HTTPRoute`, `GRPCRoute`, `TCPRoute`, `UDPRoute`, `TLSRoute`
- To: `Service`
- Description: Route forwards traffic to service
- Properties: `weight`, `port`, `backend_namespace`

**USES_TLS_FROM**
- From: `Gateway`
- To: `Secret`
- Description: Gateway uses TLS certificate from secret
- Properties: `listener_name`, `hostname`

**PERMITTED_BY**
- From: `HTTPRoute`, `GRPCRoute`, etc.
- To: `ReferenceGrant`
- Description: Cross-namespace reference is permitted
- Properties: None

**ALLOWS_ROUTE_TO**
- From: `ReferenceGrant`
- To: `Service`
- Description: Grant allows routing to service
- Properties: None

### Istio Relationships

**SELECTS_PROXY**
- From: `IstioGateway`
- To: `Pod`
- Description: Gateway selects proxy pods by labels
- Properties: `selector_labels`

**ATTACHES_TO** (Istio)
- From: `VirtualService`
- To: `IstioGateway`
- Description: VirtualService attaches to gateway
- Properties: `gateway_ref`

**ROUTES_TRAFFIC_FOR**
- From: `VirtualService`
- To: `Service`
- Description: VirtualService routes traffic for service
- Properties: `host`

**ROUTES_TO_SUBSET**
- From: `VirtualService`
- To: `DestinationRule`
- Description: VirtualService routes to specific subset
- Properties: `subset_name`, `weight`

**DEFINES_POLICY_FOR**
- From: `DestinationRule`
- To: `Service`
- Description: DestinationRule defines policies for service
- Properties: `host`

**SELECTS_SUBSET_PODS**
- From: `DestinationRule`
- To: `Pod`
- Description: DestinationRule subset selects pods by labels
- Properties: `subset_name`, `subset_labels`

**APPLIES_TO**
- From: `AuthorizationPolicy`, `PeerAuthentication`, `RequestAuthentication`
- To: `Pod`
- Description: Security policy applies to workload
- Properties: `action` (for AuthorizationPolicy), `mtls_mode` (for PeerAuthentication), `selector_labels`

## Property Conventions

### Common Properties

All nodes typically include:
- `name`: Resource name (string)
- `namespace`: Namespace name (string, omitted for cluster-scoped)
- `uid`: Kubernetes UID (string)
- `labels`: JSON-serialized labels (string)
- `annotations`: JSON-serialized annotations (string)
- `created`: Creation timestamp (string)

### Status Properties

Resources include their current status:
- Pods: `status` (Running, Pending, Failed, etc.)
- Nodes: `status` (Ready, NotReady, Unknown)
- PVCs: `status` (Pending, Bound, Lost)
- Gateway API: `accepted`, `programmed` (True/False/Unknown)

### Serialized Properties

Complex nested structures are JSON-serialized:
- `labels`: `{"app": "nginx", "version": "v1"}`
- `selector`: `{"app": "nginx"}`
- `listeners`: Array of listener configurations
- `rules`: Array of routing rules

### Timestamp Format

All timestamps are ISO 8601 strings:
- `created`: `2025-01-15T10:30:00Z`
- `last_timestamp`: `2025-01-15T11:45:23Z`

## Query Tips

### Finding Nodes by ID

```cypher
MATCH (n {id: 'Pod/default/my-pod'})
RETURN n
```

### Finding Nodes by Property

```cypher
MATCH (n:Pod)
WHERE n.status = 'Running' AND n.namespace = 'production'
RETURN n
```

### Traversing Relationships

```cypher
MATCH (d:Deployment)-[:MANAGES*]->(p:Pod)
WHERE d.name = 'my-app'
RETURN d, p
```

## Further Reading

- **[Cypher Query Reference](cypher-queries.md)** - Complete query library
- **[Query Guide](../user-guide/querying.md)** - Common query patterns
- **[Extensions Guide](../user-guide/extensions.md)** - Gateway API and Istio specifics

