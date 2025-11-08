# Extensions: Gateway API, Istio, and Kuadrant

kkbase extends beyond core Kubernetes resources to support modern ingress, service mesh, and API management technologies. Extension handlers automatically activate when their CRDs are detected in your cluster.

## Gateway API Support

The [Kubernetes Gateway API](https://gateway-api.sigs.k8s.io/) provides role-oriented, portable APIs for ingress and routing. kkbase models these resources and their relationships for powerful traffic flow analysis.

### Supported Resources

| Resource | Description | API Version |
|----------|-------------|-------------|
| `GatewayClass` | Gateway implementation template | v1 |
| `Gateway` | Load balancer configuration | v1 |
| `HTTPRoute` | HTTP/HTTPS routing rules | v1 |
| `GRPCRoute` | gRPC routing rules | v1 |
| `TCPRoute` | TCP routing rules | v1alpha2 |
| `UDPRoute` | UDP routing rules | v1alpha2 |
| `TLSRoute` | TLS routing rules | v1alpha2 |
| `ReferenceGrant` | Cross-namespace permissions | v1beta1 |

### Key Relationships

- `Gateway` → `IMPLEMENTED_BY` → `GatewayClass` (which controller manages it)
- `HTTPRoute` → `ATTACHES_TO` → `Gateway` (routing rules bind to gateway)
- `HTTPRoute` → `FORWARDS_TO` → `Service` (traffic destination)
- `Gateway` → `USES_TLS_FROM` → `Secret` (TLS certificates)
- `HTTPRoute` → `PERMITTED_BY` → `ReferenceGrant` (cross-namespace authorization)

### Example Queries

#### Trace Traffic from Gateway to Pods

```cypher
MATCH (gc:GatewayClass)<-[:IMPLEMENTED_BY]-(gw:Gateway)
      <-[:ATTACHES_TO]-(route:HTTPRoute)
      -[:FORWARDS_TO]->(svc:Service)
      -[:SELECTS_PODS]->(pod:Pod)
WHERE gw.name = 'external'
RETURN gc.controller_name as controller,
       gw.name as gateway,
       route.name as route,
       svc.name as service,
       collect(pod.name) as pods
```

#### Find Routes Without Backend Pods

```cypher
MATCH (route:HTTPRoute)-[:FORWARDS_TO]->(svc:Service)
WHERE NOT (svc)-[:SELECTS_PODS]->(:Pod)
RETURN route.namespace, route.name, svc.name
```

#### Identify Cross-Namespace Routing Without Grants

```cypher
MATCH (route:HTTPRoute)-[:FORWARDS_TO]->(svc:Service)
WHERE route.namespace <> svc.namespace
AND NOT (route)-[:PERMITTED_BY]->(:ReferenceGrant)
RETURN route.namespace as route_ns,
       route.name as route,
       svc.namespace as service_ns,
       svc.name as service
```

This query finds security violations where routes access services in other namespaces without proper ReferenceGrants.

#### Find Gateways Using Specific TLS Certificates

```cypher
MATCH (gw:Gateway)-[r:USES_TLS_FROM]->(secret:Secret)
WHERE secret.type = 'kubernetes.io/tls'
RETURN gw.name, gw.namespace, r.listener_name, secret.name
```

### Installation

Gateway API support is automatic when CRDs are installed:

```bash
kubectl apply -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.2.1/standard-install.yaml
```

Verify in kkbase logs:
```
INFO  Gateway API availability  gatewayclass=true gateway=true httproute=true
```

## Istio Support

[Istio](https://istio.io/) is a service mesh providing traffic management, security, and observability. kkbase models Istio CRDs to enable traffic flow analysis, canary deployment tracking, and security policy auditing.

### Supported Resources

**Traffic Management** (networking.istio.io/v1):
- `Gateway` - Ingress/egress gateway configuration
- `VirtualService` - Traffic routing rules
- `DestinationRule` - Post-routing policies and subsets
- `ServiceEntry` - External service registration
- `Sidecar` - Sidecar proxy configuration

**Security** (security.istio.io/v1):
- `AuthorizationPolicy` - Access control policies
- `PeerAuthentication` - Mutual TLS configuration
- `RequestAuthentication` - JWT authentication

### Key Relationships

- `IstioGateway` → `SELECTS_PROXY` → `Pod` (gateway proxy pods)
- `VirtualService` → `ATTACHES_TO` → `IstioGateway` (routing rules on gateway)
- `VirtualService` → `ROUTES_TRAFFIC_FOR` → `Service` (which service is routed)
- `VirtualService` → `ROUTES_TO_SUBSET` → `DestinationRule` (weighted routing to versions)
- `DestinationRule` → `SELECTS_SUBSET_PODS` → `Pod` (which pods are in each subset)
- `AuthorizationPolicy` → `APPLIES_TO` → `Pod` (access control on workloads)

### Example Queries

#### Find Canary Deployment Pods

```cypher
MATCH (vs:VirtualService)-[:ROUTES_TRAFFIC_FOR]->(svc:Service {name: 'checkout'})
MATCH (vs)-[:ROUTES_TO_SUBSET]->(dr:DestinationRule)
MATCH (dr)-[:SELECTS_SUBSET_PODS {subset_name: 'canary'}]->(pod:Pod)
RETURN pod.name, pod.status, pod.labels
```

#### Trace Traffic from Istio Gateway to Canary Pods

```cypher
MATCH (ig:IstioGateway {name: 'main-gateway'})-[:SELECTS_PROXY]->(proxy:Pod)
MATCH (vs:VirtualService)-[:ATTACHES_TO]->(ig)
MATCH (vs)-[:ROUTES_TRAFFIC_FOR]->(svc:Service)
MATCH (vs)-[r:ROUTES_TO_SUBSET]->(dr:DestinationRule)
MATCH (dr)-[:SELECTS_SUBSET_PODS]->(pod:Pod)
RETURN vs.name, r.subset_name as version, r.weight as traffic_percent, count(pod) as pods
ORDER BY r.weight DESC
```

#### Check Authorization Policies for a Service

```cypher
MATCH (svc:Service {name: 'payment-service'})-[:SELECTS_PODS]->(pod:Pod)
MATCH (policy:AuthorizationPolicy)-[:APPLIES_TO]->(pod)
RETURN policy.name, policy.action, policy.namespace
```

#### Find Services with Traffic Splits

```cypher
MATCH (vs:VirtualService)-[r:ROUTES_TO_SUBSET]->(dr:DestinationRule)
MATCH (vs)-[:ROUTES_TRAFFIC_FOR]->(svc:Service)
RETURN svc.name, svc.namespace,
       collect({subset: r.subset_name, weight: r.weight}) as traffic_split
```

This identifies all services using weighted routing (canary, blue-green, etc.).

### Installation

Istio support is automatic when Istio is installed in your cluster (tested with Istio 1.20+):

```bash
istioctl install --set profile=demo
```

Verify in kkbase logs:
```
INFO  Istio availability  gateway=true virtualservice=true destinationrule=true
```

## Kuadrant Support

[Kuadrant](https://kuadrant.io/) extends the Gateway API with declarative policies for authentication, rate limiting, DNS management, and TLS configuration. kkbase models these policies and their relationships for comprehensive API management diagnostics.

### Supported Resources

| Resource | Description | API Version |
|----------|-------------|-------------|
| `Kuadrant` | Kuadrant operator instance | v1beta1 |
| `AuthPolicy` | Authentication and authorization rules | v1 |
| `RateLimitPolicy` | Rate limiting policies | v1 |
| `DNSPolicy` | DNS configuration and load balancing | v1 |
| `TLSPolicy` | TLS certificate management | v1 |

### Key Relationships

- `AuthPolicy` → `APPLIES_TO` → `Gateway` / `HTTPRoute` (policy enforcement points)
- `RateLimitPolicy` → `APPLIES_TO` → `Gateway` / `HTTPRoute` (rate limit application)
- `DNSPolicy` → `APPLIES_TO` → `Gateway` (DNS configuration)
- `TLSPolicy` → `APPLIES_TO` → `Gateway` (TLS certificate management)
- `DNSPolicy` → `USES_SECRET` → `Secret` (DNS provider credentials)
- `Kuadrant` → `MANAGES` → Policies (operator lifecycle management)

### Indexed Properties

For fast diagnostics, kkbase indexes key policy properties:

**All Policies:**
- `status_accepted`: Policy accepted by controller
- `status_enforced` / `status_ready`: Policy actively enforced
- `status_failed`: Policy has failures
- `status_message`: Failure details
- `status_stale`: Status out of sync with spec

**AuthPolicy & RateLimitPolicy:**
- `policy_type`: "defaults" | "overrides" | "implicit_defaults" (precedence control)
- `authentication_configured`: Auth rules present
- `limits_count`: Number of rate limit definitions

**DNSPolicy:**
- `has_load_balancing`: Load balancing configured

**TLSPolicy:**
- `has_issuer_ref`: Certificate issuer configured

### Example Queries

#### Check Policy Enforcement Status

```cypher
MATCH (policy:AuthPolicy)-[:APPLIES_TO]->(gw:Gateway)
WHERE policy.status_enforced = false
RETURN policy.name, policy.namespace, policy.status_message
```

This finds policies that exist but aren't being enforced, helping diagnose configuration issues.

#### Find Policy Conflicts (Multiple Policies on Same Target)

```cypher
MATCH (p1:AuthPolicy)-[:APPLIES_TO]->(target)
MATCH (p2:AuthPolicy)-[:APPLIES_TO]->(target)
WHERE id(p1) < id(p2)
RETURN target.name, target.kind,
       p1.name as policy1, p1.policy_type as type1,
       p2.name as policy2, p2.policy_type as type2
```

Kuadrant policies follow precedence rules (overrides > defaults). This query identifies potential conflicts.

#### Trace Gateway with All Applied Policies

```cypher
MATCH (gw:Gateway {name: 'api-gateway'})<-[:ATTACHES_TO]-(route:HTTPRoute)
OPTIONAL MATCH (auth:AuthPolicy)-[:APPLIES_TO]->(route)
OPTIONAL MATCH (rate:RateLimitPolicy)-[:APPLIES_TO]->(route)
OPTIONAL MATCH (dns:DNSPolicy)-[:APPLIES_TO]->(gw)
OPTIONAL MATCH (tls:TLSPolicy)-[:APPLIES_TO]->(gw)
RETURN gw.name,
       route.name as route,
       auth.name as auth_policy,
       rate.name as rate_limit,
       dns.name as dns_policy,
       tls.name as tls_policy
```

Complete picture of all policies affecting a gateway and its routes.

#### Find DNS Policies with Missing Provider Secrets

```cypher
MATCH (dns:DNSPolicy)-[:USES_SECRET]->(secret:Secret)
WHERE NOT EXISTS(secret.creation_timestamp)
RETURN dns.name, dns.namespace, secret.name
```

Identifies DNS policies that reference non-existent secrets (placeholder nodes).

#### Find Stale Policy Status

```cypher
MATCH (policy)
WHERE policy:AuthPolicy OR policy:RateLimitPolicy OR policy:DNSPolicy OR policy:TLSPolicy
AND policy.status_stale = true
RETURN labels(policy)[0] as type, policy.name, policy.namespace,
       policy.observed_generation as last_seen,
       policy.status_message
```

Finds policies where the status hasn't been updated to reflect spec changes.

### Installation

Kuadrant support is automatic when the operator is installed:

```bash
# Install Kuadrant operator (example using Helm)
helm repo add kuadrant https://kuadrant.io/helm-charts/
helm install kuadrant-operator kuadrant/kuadrant-operator
```

Verify in kkbase logs:
```
INFO  creating AuthPolicy handler  version=v1
INFO  creating RateLimitPolicy handler  version=v1
INFO  creating DNSPolicy handler  version=v1
INFO  creating TLSPolicy handler  version=v1
```

### Version-Agnostic Design

Kuadrant handlers use a version-agnostic architecture that adapts to different API versions automatically. See the [Architecture documentation](../../pkg/watchers/handlers/extensions/kuadrant/ARCHITECTURE.md) for implementation details.

## Enabling and Disabling Extensions

Extensions automatically activate when their CRDs are detected. No explicit configuration is required.

To verify which extensions are active:

```bash
kubectl logs -f deployment/kkbase-watcher | grep "availability"
```

To disable watching specific resources, modify the RBAC permissions in `deploy/rbac.yaml` (remove the resource from the ClusterRole).

## Use Cases

### Gateway API Use Cases

1. **Troubleshoot 503 Errors** - Trace from hostname → gateway → route → service → pods
2. **Validate TLS Certificates** - Find which gateways use which certificates
3. **Audit Cross-Namespace Routing** - Ensure ReferenceGrants are properly configured
4. **Capacity Planning** - Count routes per gateway for load balancing decisions

### Istio Use Cases

1. **Monitor Canary Rollouts** - Track traffic splits and pod health per version
2. **Security Auditing** - Find services without AuthorizationPolicies
3. **mTLS Compliance** - Verify PeerAuthentication policies are applied
4. **Debug Traffic Routing** - Trace VirtualService rules to actual pods
5. **Analyze Service Dependencies** - Map traffic flow through the mesh

### Kuadrant Use Cases

1. **Policy Enforcement Diagnostics** - Check if policies are accepted and enforced
2. **Authentication Troubleshooting** - Trace auth failures to policy configuration
3. **Rate Limit Analysis** - Verify rate limits are applied correctly
4. **DNS Configuration Validation** - Ensure DNS policies and provider credentials are valid
5. **Policy Precedence Auditing** - Identify which policy wins when multiple apply
6. **Stale Status Detection** - Find policies with out-of-sync status

## Troubleshooting Extensions

### Extension Handlers Not Loading

Check if CRDs are installed:

```bash
kubectl get crd | grep gateway
kubectl get crd | grep istio
kubectl get crd | grep kuadrant
```

### Missing Relationships

Verify that resources reference each other correctly:
- HTTPRoutes must reference Gateway names in `parentRefs`
- VirtualServices must list Service hosts in `spec.hosts`
- DestinationRule subsets must match Pod labels
- Kuadrant policies must reference targets in `spec.targetRef`

### RBAC Errors

Ensure ClusterRole includes permissions for extension resources. Check logs:

```bash
kubectl logs deployment/kkbase-watcher | grep -i forbidden
```

## Further Reading

- **[Complete Query Reference](../../reference/cypher-queries.md)** - All Gateway API, Istio, and Kuadrant queries
- **[Graph Schema](../../reference/graph-schema.md)** - Extension node and edge types
- **[Adding Handlers](../../development/adding-handlers.md)** - Create your own extension handlers
- **[Kuadrant Architecture](../../../pkg/watchers/handlers/extensions/kuadrant/ARCHITECTURE.md)** - Version-agnostic handler design
- **[Gateway API Documentation](https://gateway-api.sigs.k8s.io/)**
- **[Istio Documentation](https://istio.io/latest/docs/)**
- **[Kuadrant Documentation](https://docs.kuadrant.io/)**

