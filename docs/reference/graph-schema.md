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

### Determining Resource Scope

Resource scope (cluster-scoped vs. namespaced) is automatically determined via the NodeType metadata registry. Each resource type registers its scope when its handler is registered:

```go
// Example: Pod is namespaced
watchers.ResourceTypeInfo{
    NodeType:      models.NodeTypePod,
    Kind:          "Pod",
    ClusterScoped: false,  // Uses namespace in ID
}

// Example: Node is cluster-scoped
watchers.ResourceTypeInfo{
    NodeType:      models.NodeTypeNode,
    Kind:          "Node",
    ClusterScoped: true,   // No namespace in ID
}
```

This metadata is used throughout the system to:
- Generate correct node IDs
- Create proper ownership relationships
- Validate cross-namespace references

See [Architecture > Resource Type Registry](../development/architecture.md#resource-type-registry) for details.

## Node Types

### Standardized Properties

All resources now include these standardized properties:

- **`created_at`**: Unix timestamp from Kubernetes resource `metadata.creationTimestamp`
- **`updated_at`**: Unix timestamp of last graph update (managed by graph store)
- **`observed_generation`**: For resources with status, tracks if controller has processed latest spec
- **`status_stale`**: Boolean flag set when `observed_generation` != `metadata.generation`
- **Status conditions as booleans**: e.g., `status_ready`, `status_accepted`, `status_progressing`
- **Per-condition messages**: e.g., `status_ready_message`, `status_accepted_message` (human-readable details)
- **Per-condition reasons**: e.g., `status_ready_reason`, `status_accepted_reason` (machine-readable codes)

### Status Properties on Relationships

For resources with per-parent or per-ancestor status (routes, policies):

- **Node properties**: Aggregate status across all parents/ancestors (e.g., `status_accepted=true` if ANY parent accepts)
- **Edge properties**: Per-parent/ancestor status with messages and reasons stored on ATTACHES_TO/APPLIES_TO relationships

Example: An HTTPRoute attached to two Gateways will have:
- Node: `status_accepted=true` (aggregate)
- Edge to Gateway A: `status_accepted=true`, `status_accepted_message="", status_accepted_reason="Accepted"`
- Edge to Gateway B: `status_accepted=false`, `status_accepted_message="Invalid route rule", status_accepted_reason="UnsupportedValue"`

### Core Kubernetes Resources

#### Compute & Infrastructure

**Cluster**
- Represents the entire Kubernetes cluster
- Properties: `name`, `version`, `cloud_provider`

**Node**
- Worker or master machines in the cluster
- Properties: `name`, `status`, `internal_ip`, `external_ip`, `cpu_capacity`, `memory_capacity`, `created_at`, `status_ready`, `status_memory_pressure`, `status_disk_pressure`, `status_pid_pressure`, `status_network_unavailable`, `labels`

**Namespace**
- Logical isolation boundary
- Properties: `name`, `status`, `created_at`, `status_deletion_discovery_failure`, `status_deletion_content_failure`, `status_deletion_gv_parsing_failure`, `status_content_remaining`, `status_finalizers_remaining`, `labels`

#### Workload Resources

**Pod**
- Smallest deployable unit
- Properties: `name`, `namespace`, `status`, `ip`, `node_name`, `host_ip`, `cpu_request`, `memory_request`, `cpu_limit`, `memory_limit`, `created_at`, `status_pod_scheduled`, `status_containers_ready`, `status_initialized`, `status_ready`, `labels`, `owners`

**Container**
- Individual container within a Pod
- Properties: `name`, `image`, `image_id`, `ports`, `restart_count`, `ready`, `started`, `exit_code`, `reason`, `message`

**Deployment**
- Manages ReplicaSets for declarative updates
- Properties: `name`, `namespace`, `desired_replicas`, `ready_replicas`, `available_replicas`, `updated_replicas`, `strategy`, `created_at`, `observed_generation`, `status_stale`, `status_available`, `status_progressing`, `status_replica_failure`, `status_message`, `labels`, `selector`

**ReplicaSet**
- Ensures specified number of pod replicas
- Properties: `name`, `namespace`, `desired_replicas`, `current_replicas`, `ready_replicas`, `created_at`, `observed_generation`, `status_stale`, `status_replica_failure`, `status_message`, `labels`, `selector`, `owners`

**StatefulSet**
- Manages stateful applications
- Properties: `name`, `namespace`, `desired_replicas`, `current_replicas`, `ready_replicas`, `created_at`, `observed_generation`, `status_stale`, `status_replicas_ready`, `labels`, `selector`

**DaemonSet**
- Ensures pods run on all/selected nodes
- Properties: `name`, `namespace`, `desired_scheduled`, `current_scheduled`, `number_ready`, `number_available`, `created_at`, `observed_generation`, `status_stale`, `status_available`, `labels`, `selector`

**Job**
- Run-to-completion workloads
- Properties: `name`, `namespace`, `uid`, `completions`, `parallelism`, `active`, `succeeded`, `failed`, `labels`, `annotations`, `created`

#### Networking Resources

**Service**
- Exposes applications running on Pods
- Properties: `name`, `namespace`, `type`, `cluster_ip`, `external_ips`, `ports`, `selector`, `created_at`, `labels`

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
- Properties: `name`, `capacity`, `access_modes`, `reclaim_policy`, `status`, `storage_class`, `volume_mode`, `created_at`, `status_ready`, `labels`

**PersistentVolumeClaim (PVC)**
- Request for storage by a user
- Properties: `name`, `namespace`, `status`, `requested_storage`, `capacity`, `access_modes`, `storage_class`, `volume_name`, `created_at`, `status_resizing`, `status_filesystem_resize_pending`, `labels`

**StorageClass**
- Storage provisioning template
- Properties: `name`, `provisioner`, `parameters`, `reclaim_policy`, `volume_binding_mode`, `allow_volume_expansion`, `created_at`, `labels`

#### Configuration Resources

**ConfigMap**
- Non-confidential configuration data
- Properties: `name`, `namespace`, `data_keys`, `created_at`, `labels`

**Secret**
- Sensitive information storage
- Properties: `name`, `namespace`, `type`, `data_keys`, `created_at`, `labels`

#### Observability Resources

**K8sEvent**
- Kubernetes API events
- Properties: `name`, `namespace`, `uid`, `type`, `reason`, `message`, `involved_object_kind`, `involved_object_name`, `first_timestamp`, `last_timestamp`, `count`, `source`, `created`

**Recommendation**
- Actionable next steps identified during investigation
- Properties: `id`, `type`, `priority`, `title`, `description`, `rationale`, `action_items` (JSON array), `estimated_effort`, `automation_hint`, `tags` (JSON array), `metadata` (JSON object), `created_at`
- Type values: `root_cause_fix`, `preventive_action`, `optimization`, `monitoring_improvement`, `cleanup`
- Priority values: `critical`, `high`, `medium`, `low`

**Pattern**
- Reusable diagnostic pattern learned from investigations
- Properties: `id`, `name`, `root_cause_resource_type`, `root_cause_issue_type`, `symptom_keywords` (JSON array), `investigation_steps` (JSON array), `diagnosis_guidance`, `recommendations` (JSON array), `bundle_id`, `source`, `usage_count`, `created_at`, `metadata` (JSON object)
- Source values: `discovered` (learned from completed sessions), `bundled` (imported from pattern bundle)
- Match key: `root_cause_resource_type` + `root_cause_issue_type` (strict matching)
- Symptom matching: Fuzzy keyword matching on `symptom_keywords` array for initial pattern suggestion

**PatternBundle**
- Versioned collection of patterns for distribution
- Properties: `id`, `name`, `version`, `source_url`, `description`, `imported_at`, `updated_at`, `active`
- Used to import/export diagnostic knowledge between kkbase installations

**Trace**
- Distributed trace aggregation (one per trace)
- Properties: `trace_id`, `start_time`, `duration_ms`, `root_operation`, `root_service`, `span_count`, `error_count`, `has_errors`, `services_involved`

**Span**
- Individual trace span (retained for recent time window, e.g., 1 hour)
- Properties: `span_id`, `trace_id`, `parent_span_id`, `operation_name`, `service_name`, `service_namespace`, `start_time`, `duration_ms`, `duration_us`, `span_kind`, `status`, `error`, `error_message`, `protocol`, `http_method`, `http_path`, `http_status_code`, `rpc_service`, `rpc_method`, `upstream_name`, `upstream_url`

**ServiceCall** (optional aggregation node)
- Aggregated service-to-service call metrics
- Properties: `from_service`, `from_namespace`, `to_service`, `to_namespace`, `protocol`, `call_count`, `error_count`, `error_rate`, `avg_latency_ms`, `p95_latency_ms`, `window`, `last_seen`, `first_seen`

#### Agent Session Resources

These nodes track AI agent diagnostic sessions and their findings.

**AgentSession**
- Complete AI agent diagnostic session
- Properties: `id`, `initial_symptom`, `initial_resource`, `status`, `created_at`, `completed_at`, `current_stage`, `query_count`, `finding_count`, `summary`
- Status values: `active`, `completed`, `timeout`, `incomplete`, `abandoned`

**Hypothesis**
- Versioned hypothesis at a specific investigation stage
- Properties: `id`, `stage`, `text`, `status`, `created_at`
- Status values: `active`, `superseded`, `confirmed`

**QueryExecution**
- Single query executed by the agent with reasoning
- Properties: `id`, `query`, `reasoning`, `params`, `result_count`, `duration`, `executed_at`, `findings` (JSON array of finding IDs)

**Finding**
- Discovered issue during investigation
- Properties: `id`, `type`, `severity`, `resource_id`, `resource_type`, `description`, `evidence`, `detection_method`, `discovered_at`
- Type values: `failed_dependency`, `unhealthy_pod`, `error_spike`, `deployment_change`, `resource_exhaustion`, `misconfiguration`
- Severity values: `critical`, `warning`, `info`
- Detection method: `automatic` (extracted from queries), `agent_recorded` (explicitly recorded)

**Investigation**
- Metrics-focused investigation session for RCA
- Properties: `id`, `resource_type`, `resource_id`, `symptom`, `start_time`, `lookback_duration`, `status`, `created_at`
- Status values: `active`, `completed`, `abandoned`

### Gateway API Resources

**GatewayClass**
- Gateway implementation template (cluster-scoped)
- Properties: `name`, `controller_name`, `description`, `created_at`, `observed_generation`, `status_stale`, `status_accepted`, `status_supported_version`, `status_message`, `labels`

**Gateway**
- Load balancer configuration
- Properties: `name`, `namespace`, `gateway_class_name`, `listeners`, `addresses`, `created_at`, `observed_generation`, `status_stale`, `status_accepted`, `status_programmed`, `status_scheduled`, `status_ready`, `status_attached_listener_sets`, `labels`

**HTTPRoute**
- HTTP/HTTPS routing rules
- Properties: `name`, `namespace`, `hostnames`, `parent_refs`, `rules`, `rule_count`, `created_at`, `observed_generation`, `status_stale`, `status_accepted`, `status_resolved_refs`, `status_partially_invalid`, `status_message`, `labels`

**GRPCRoute**
- gRPC routing rules
- Properties: `name`, `namespace`, `hostnames`, `parent_refs`, `rules`, `rule_count`, `created_at`, `observed_generation`, `status_stale`, `status_accepted`, `status_resolved_refs`, `status_partially_invalid`, `status_message`, `labels`

**TCPRoute**
- TCP routing rules
- Properties: `name`, `namespace`, `parent_refs`, `rule_count`, `created_at`, `observed_generation`, `status_stale`, `status_accepted`, `status_resolved_refs`, `status_partially_invalid`, `status_message`, `labels`

**UDPRoute**
- UDP routing rules
- Properties: `name`, `namespace`, `parent_refs`, `rule_count`, `created_at`, `observed_generation`, `status_stale`, `status_accepted`, `status_resolved_refs`, `status_partially_invalid`, `status_message`, `labels`

**TLSRoute**
- TLS routing rules
- Properties: `name`, `namespace`, `hostnames`, `parent_refs`, `rule_count`, `created_at`, `observed_generation`, `status_stale`, `status_accepted`, `status_resolved_refs`, `status_partially_invalid`, `status_message`, `labels`

**BackendTLSPolicy**
- Backend TLS validation policy
- Properties: `name`, `namespace`, `target_refs`, `hostname`, `ca_certificate_refs`, `well_known_ca_certificates`, `created_at`, `observed_generation`, `status_stale`, `status_accepted`, `status_resolved_refs`, `status_message`, `labels`

**ReferenceGrant**
- Cross-namespace reference permissions
- Properties: `name`, `namespace`, `from`, `to`, `created_at`, `labels`

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

### Agent Session Relationships

**HAS_HYPOTHESIS**
- From: `AgentSession`
- To: `Hypothesis`
- Description: Session has a hypothesis at a specific stage
- Properties: None

**EXECUTED_QUERY**
- From: `AgentSession`
- To: `QueryExecution`
- Description: Agent executed this query during the session
- Properties: `sequence` (execution order)

**HAS_FINDING**
- From: `AgentSession`
- To: `Finding`
- Description: Session discovered this finding
- Properties: None

**AFFECTS**
- From: `Finding`
- To: Any Kubernetes resource
- Description: Finding affects this specific resource
- Properties: None

**HAS_RECOMMENDATION**
- From: `AgentSession`
- To: `Recommendation`
- Description: Session produced this recommendation
- Properties: None

**BASED_ON**
- From: `Recommendation`
- To: `Finding`
- Description: Recommendation is based on these findings
- Properties: None

**DISCOVERED_PATTERN**
- From: `AgentSession`
- To: `Pattern`
- Description: Session discovered and recorded this pattern
- Properties: None

**PRESENTED_PATTERN**
- From: `AgentSession`
- To: `Pattern`
- Description: Pattern was suggested/presented to the agent during investigation
- Properties: `presented_at` (timestamp when pattern was presented)
- Created when: Pattern matches symptom at session start or when agent queries for patterns

**USED_PATTERN**
- From: `AgentSession`
- To: `Pattern`
- Description: Agent confirmed this pattern successfully guided the investigation
- Properties: `used_at` (timestamp), `notes` (optional notes about how pattern helped)
- Created when: Agent explicitly calls mark_pattern_used tool
- Effect: Increments pattern's `usage_count` by 1

**CONTAINS**
- From: `PatternBundle`
- To: `Pattern`
- Description: Bundle contains this pattern
- Properties: None

**INVESTIGATES**
- From: `Investigation`
- To: Any Kubernetes resource
- Description: Investigation session focuses on this resource
- Properties: None

### Trace Relationships

**CONTAINS_SPAN**
- From: `Trace`
- To: `Span`
- Description: Trace contains this span
- Properties: None

**PARENT_OF**
- From: `Span`
- To: `Span`
- Description: Parent-child span relationship within a trace
- Properties: None

**ORIGINATED_FROM**
- From: `Span`
- To: `Service`
- Description: Span originated from this Kubernetes Service
- Properties: None

**EXECUTED_IN**
- From: `Span`
- To: `Pod`
- Description: Span was executed in this specific Pod instance
- Properties: None
- Note: Created when span has `k8s_pod_name` attribute

**OBSERVED_CALL_TO**
- From: `Span`
- To: `Service`
- Description: Span observed calling this service
- Properties: `protocol`, `url`, `status_code`, `duration_ms`, `error`

**CALLS** (runtime observed)
- From: `Service`
- To: `Service`
- Description: Observed runtime service-to-service call from trace data
- Properties: `source` (always "trace_observed"), `protocol`, `last_observed`, `duration_ms`, `status_code`, `error`

**FAILED_CALL_TO**
- From: `Service`
- To: `Service`
- Description: Failed service call observed in traces
- Properties: `error_count`, `error_message`, `status_code`, `last_failure`

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
- **[Query Guide](../guides/querying/basics.md)** - Common query patterns
- **[Extensions Guide](../services/watcher/extensions.md)** - Gateway API and Istio specifics

