# Extensions: Gateway API and Istio

kkbase extends beyond core Kubernetes resources to support modern ingress and service mesh technologies. Extension handlers automatically activate when their CRDs are detected in your cluster.

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

## Troubleshooting Extensions

### Extension Handlers Not Loading

Check if CRDs are installed:

```bash
kubectl get crd | grep gateway
kubectl get crd | grep istio
```

### Missing Relationships

Verify that resources reference each other correctly:
- HTTPRoutes must reference Gateway names in `parentRefs`
- VirtualServices must list Service hosts in `spec.hosts`
- DestinationRule subsets must match Pod labels

### RBAC Errors

Ensure ClusterRole includes permissions for extension resources. Check logs:

```bash
kubectl logs deployment/kkbase-watcher | grep -i forbidden
```

## Further Reading

- **[Complete Query Reference](../reference/cypher-queries.md)** - All Gateway API and Istio queries
- **[Graph Schema](../reference/graph-schema.md)** - Extension node and edge types
- **[Adding Handlers](../development/adding-handlers.md)** - Create your own extension handlers
- **[Gateway API Documentation](https://gateway-api.sigs.k8s.io/)**
- **[Istio Documentation](https://istio.io/latest/docs/)**

