### The Istio Knowledge Graph: Modeling Traffic, Policy, and Intent

Integrating Istio into the Kubernetes knowledge graph adds a rich, behavioral layer on top of the existing structural model. While the core Kubernetes graph tells an agent *what* resources exist and *where* they are, the Istio layer explains *how* they communicate, *what* policies govern their interactions, and *why* traffic flows the way it does. This moves the agent's understanding from a static inventory to a dynamic map of traffic intent, security posture, and resilience strategy.

The graph is composed of nodes (Istio's Custom Resource Definitions), edges (the relationships between them), and properties (their specific configurations).

**Implementation Details:**
- **Package:** `pkg/watchers/handlers/extensions/istio/`
- **API Versions:** `networking.istio.io/v1` (traffic management), `security.istio.io/v1` (security policies)
- **Pattern:** Follows the Gateway API extension pattern with CRD availability checking
- **Dependencies:** `istio.io/client-go` for typed Istio resources

---

### 1. Core Entities (Nodes) in the Graph

These nodes represent the key Istio CRDs that control the service mesh, categorized by their primary function.

| Category | Node Type (Entity) | Description & Key Properties | Graph Node Type |
|---|---|---|---|
| **Traffic Routing** | `Gateway` | Manages ingress and egress traffic at the edge of the mesh. It specifies ports, protocols, and hosts to expose. **Properties:** `name`, `namespace`, `uid`, `created`, `labels`, `annotations`, `selector` (e.g., `istio: ingressgateway`), `servers` (serialized JSON: port_number, protocol, hosts, tls_settings). | `IstioGateway` |
| | `VirtualService` | Defines a set of traffic routing rules to apply when a host is addressed. It allows for fine-grained control over how requests are routed to services within the mesh. **Properties:** `name`, `namespace`, `uid`, `created`, `labels`, `annotations`, `hosts` (array), `gateways` (array), `http_routes` (serialized JSON: match conditions, destinations, weights, retries, timeouts), `tcp_routes`, `tls_routes`. | `VirtualService` |
| | `DestinationRule` | Defines policies that are applied to traffic *after* routing has occurred. It is where you configure service versions (subsets), load balancing policies, and connection settings. **Properties:** `name`, `namespace`, `uid`, `created`, `labels`, `annotations`, `host` (service FQDN), `subsets` (serialized JSON: name, labels), `traffic_policy` (serialized JSON: load_balancer, connection_pool, outlier_detection). | `DestinationRule` |
| | `ServiceEntry` | Adds an entry to Istio's internal service registry, allowing services outside the mesh (e.g., external APIs, legacy VMs) to be treated as if they were part of it. **Properties:** `name`, `namespace`, `uid`, `created`, `labels`, `annotations`, `hosts` (array), `location` (MESH_EXTERNAL/MESH_INTERNAL), `resolution` (DNS/STATIC/NONE), `endpoints` (serialized JSON). | `ServiceEntry` |
| | `Sidecar` | Provides fine-grained configuration for the Envoy sidecar proxies, controlling the set of services they can reach. **Properties:** `name`, `namespace`, `uid`, `created`, `labels`, `annotations`, `workload_selector` (serialized JSON), `egress_hosts` (array), `ingress_listeners`, `egress_listeners` (serialized JSON). | `Sidecar` |
| **Security** | `AuthorizationPolicy` | Enables access control on workloads in the mesh. It specifies which requests are allowed or denied based on identity, namespace, IP, and other attributes. **Properties:** `name`, `namespace`, `uid`, `created`, `labels`, `annotations`, `selector` (serialized JSON), `action` (ALLOW/DENY/AUDIT/CUSTOM), `rules` (serialized JSON: from sources, to operations, when conditions). | `AuthorizationPolicy` |
| | `PeerAuthentication` | Defines the mutual TLS (mTLS) mode for workloads receiving traffic. **Properties:** `name`, `namespace`, `uid`, `created`, `labels`, `annotations`, `selector` (serialized JSON), `mtls_mode` (PERMISSIVE/STRICT/DISABLE), `port_level_mtls` (serialized JSON). | `PeerAuthentication` |
| | `RequestAuthentication` | Specifies how to authenticate requests using JSON Web Tokens (JWT). **Properties:** `name`, `namespace`, `uid`, `created`, `labels`, `annotations`, `selector` (serialized JSON), `jwt_rules` (serialized JSON: issuer, jwks_uri, audiences). | `RequestAuthentication` |

---

### 2. Key Relationships (Edges) in the Graph

The edges in the Istio graph are crucial for diagnostics, as they trace the complete path of a request from the edge of the cluster down to a specific version of a container.

*   **Traffic Flow & Configuration:**
    *   `IstioGateway` —(`SELECTS_PROXY`)→ `Pod` (Connects the Gateway config to the actual Envoy proxy pod via the `selector` label, e.g., `istio: ingressgateway`). **Edge Properties:** `selector_labels` (serialized JSON).
    *   `VirtualService` —(`ATTACHES_TO`)→ `IstioGateway` (Links a set of routing rules to a specific ingress or egress point). **Edge Properties:** `gateway_ref` (name).
    *   `VirtualService` —(`ROUTES_TRAFFIC_FOR`)→ `Service` (The primary edge showing which Kubernetes Service's traffic is being managed by the `VirtualService`). **Edge Properties:** `host` (service host/FQDN).
    *   `VirtualService` —(`ROUTES_TO_SUBSET`)→ `DestinationRule` (A crucial link created when a `VirtualService` route destination specifies a `subset`, directing traffic to a specific version). **Edge Properties:** `subset_name`, `weight` (traffic percentage).

*   **Policy and Subset Definition:**
    *   `DestinationRule` —(`DEFINES_POLICY_FOR`)→ `Service` (Connects post-routing policies to the target Kubernetes Service). **Edge Properties:** `host` (service FQDN).
    *   `DestinationRule` —(`SELECTS_SUBSET_PODS`)→ `Pod` (Connects a named `subset` (e.g., "v2") to the specific Pods that match its labels (e.g., `version: v2`)). This is how Istio differentiates between service versions. **Edge Properties:** `subset_name`, `subset_labels` (serialized JSON).

*   **Security Policy Application:**
    *   `AuthorizationPolicy` —(`APPLIES_TO`)→ `Pod` (Links an access policy to the target workload(s) via its `selector`). **Edge Properties:** `action` (ALLOW/DENY), `selector_labels` (serialized JSON).
    *   `PeerAuthentication` —(`APPLIES_TO`)→ `Pod` (Links an mTLS policy to the target workload(s) via its `selector`). **Edge Properties:** `mtls_mode`, `selector_labels` (serialized JSON).
    *   `RequestAuthentication` —(`APPLIES_TO`)→ `Pod` (Links a JWT authentication policy to the target workload(s) via its `selector`). **Edge Properties:** `selector_labels` (serialized JSON).

---

### 3. How an Autonomous Agent Uses the Istio Knowledge Graph

With this enriched graph, an agent can diagnose complex traffic and security issues that are invisible at the standard Kubernetes level.

**Scenario 1: A Canary Deployment is Failing**

An alert fires for a high error rate on the `checkout-service`.
1.  **Initial Query:** The agent queries the graph for the `checkout-service`. It finds a `VirtualService` connected via a `ROUTES_TRAFFIC_FOR` edge.
2.  **Identify Traffic Split:** The agent inspects the `VirtualService` node's properties and discovers a weighted route: 90% of traffic goes to the `stable` subset, and 10% goes to the `canary` subset.
3.  **Isolate Canary Pods:** The agent traverses the `ROUTES_TO_SUBSET` edge to the `DestinationRule` for `checkout-service`. It inspects the `canary` subset and finds it selects for pods with the label `version: v2`.
4.  **Targeted Investigation:** The agent follows the `SELECTS_SUBSET_PODS` edge to the specific `Pod` nodes with the `version: v2` label. It can now focus its investigation (checking logs, events, resource usage) exclusively on these canary pods, ignoring the healthy `v1` pods.
5.  **Conclusion:** The agent can conclude with high confidence: "The high error rate for `checkout-service` is isolated to the canary deployment. The `VirtualService` 'checkout-routing' is splitting 10% of traffic to the 'canary' subset, which targets pods with label `version: v2`. These pods are exhibiting `CrashLoopBackOff`."

**Scenario 2: A Request is Returning a 403 Forbidden Error**

A user reports that a request from `frontend-service` to `backend-service` is being denied.
1.  **Identify Target Pod:** The agent identifies a `Pod` belonging to the `backend-service`.
2.  **Check for Security Policies:** The agent queries the graph: "Find all `AuthorizationPolicy` nodes that have an `APPLIES_TO` relationship with the target `Pod`."
3.  **Analyze Policy Rules:** It finds an `AuthorizationPolicy` named `deny-all-by-default`. It inspects the policy's properties and sees its `action` is `DENY`. It then looks for another policy with an `ALLOW` action.
4.  **Find Missing Rule:** The agent finds an `ALLOW` policy, but upon inspecting its `rules.from.source.principals`, it sees that the service account for `frontend-service` is not listed.
5.  **Conclusion:** The agent can report: "The 403 Forbidden error is caused by an Istio `AuthorizationPolicy`. The `deny-all-by-default` policy is active, and the `allow-specific-services` policy does not include the principal for `frontend-service` in its allow list."

By modeling Istio's CRDs, the knowledge graph gives the agent a deep understanding of L7 traffic rules, security policies, and resilience features, enabling it to perform highly contextual and efficient root cause analysis that would be impossible with Kubernetes resources alone.

---

### Example Cypher Queries

**Query 1: Find all pods in a canary deployment for a service**
```cypher
MATCH (vs:VirtualService)-[:ROUTES_TRAFFIC_FOR]->(svc:Service {name: 'checkout-service'})
MATCH (vs)-[:ROUTES_TO_SUBSET]->(dr:DestinationRule)
MATCH (dr)-[:SELECTS_SUBSET_PODS {subset_name: 'canary'}]->(pod:Pod)
RETURN pod.name, pod.namespace, pod.status, pod.labels
```

**Query 2: Trace traffic from Gateway to specific Pod subset**
```cypher
MATCH path = (gw:IstioGateway {name: 'main-gateway'})
  -[:SELECTS_PROXY]->(proxy:Pod)
MATCH (vs:VirtualService)-[:ATTACHES_TO]->(gw)
MATCH (vs)-[:ROUTES_TRAFFIC_FOR]->(svc:Service)
MATCH (vs)-[:ROUTES_TO_SUBSET]->(dr:DestinationRule)
MATCH (dr)-[:SELECTS_SUBSET_PODS]->(pod:Pod)
RETURN path, svc, dr, pod
```

**Query 3: Find all authorization policies affecting a pod**
```cypher
MATCH (pod:Pod {name: 'backend-xyz', namespace: 'production'})
MATCH (policy:AuthorizationPolicy)-[:APPLIES_TO]->(pod)
RETURN policy.name, policy.action, policy.rules
```

**Query 4: Find all services with canary deployments**
```cypher
MATCH (vs:VirtualService)-[:ROUTES_TO_SUBSET {weight: 10}]->(dr:DestinationRule)
MATCH (vs)-[:ROUTES_TRAFFIC_FOR]->(svc:Service)
MATCH (dr)-[:SELECTS_SUBSET_PODS {subset_name: 'canary'}]->(pod:Pod)
RETURN DISTINCT svc.name, svc.namespace, COUNT(pod) as canary_pods
```