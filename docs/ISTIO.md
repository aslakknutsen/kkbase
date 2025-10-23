# Istio Support

kkbase includes comprehensive support for Istio service mesh resources, allowing you to model traffic routing, security policies, and service mesh configuration in the knowledge graph.

## Supported Istio CRDs

### Traffic Management (networking.istio.io/v1)

1. **Gateway** - Manages ingress and egress traffic at the edge of the mesh
2. **VirtualService** - Defines traffic routing rules for services
3. **DestinationRule** - Defines policies for traffic after routing (subsets, load balancing, etc.)
4. **ServiceEntry** - Adds external services to the mesh's service registry
5. **Sidecar** - Configures Envoy sidecar proxies for specific workloads

### Security (security.istio.io/v1)

6. **AuthorizationPolicy** - Defines access control policies for workloads
7. **PeerAuthentication** - Configures mutual TLS (mTLS) for service-to-service communication
8. **RequestAuthentication** - Configures JWT authentication for incoming requests

## Graph Schema

### Node Types

- `IstioGateway` - Istio Gateway resources
- `VirtualService` - VirtualService resources
- `DestinationRule` - DestinationRule resources
- `ServiceEntry` - ServiceEntry resources
- `Sidecar` - Sidecar resources
- `AuthorizationPolicy` - AuthorizationPolicy resources
- `PeerAuthentication` - PeerAuthentication resources
- `RequestAuthentication` - RequestAuthentication resources

### Edge Types

- `SELECTS_PROXY` - Gateway → Pod (via selector labels)
- `ATTACHES_TO` - VirtualService → IstioGateway
- `ROUTES_TRAFFIC_FOR` - VirtualService → Service
- `ROUTES_TO_SUBSET` - VirtualService → DestinationRule (when routing to specific subsets)
- `DEFINES_POLICY_FOR` - DestinationRule → Service
- `SELECTS_SUBSET_PODS` - DestinationRule → Pod (per subset definition)
- `APPLIES_TO` - AuthorizationPolicy/PeerAuthentication/RequestAuthentication → Pod

## Installation

Istio handlers are automatically registered when kkbase detects that Istio CRDs are installed in the cluster. No additional configuration is required.

### Prerequisites

- Istio installed in your cluster (tested with Istio 1.20+)
- RBAC permissions configured (automatically included in `deploy/rbac.yaml`)

### Verifying Installation

Check the kkbase logs for Istio detection:

```bash
kubectl logs -f deployment/kkbase-watcher | grep -i istio
```

You should see:
```
INFO  Istio availability  {"gateway": true, "virtualservice": true, ...}
INFO  registering Istio handlers
```

## Example Queries

### 1. Find All Pods in a Canary Deployment

Find all pods that are part of a canary deployment for a specific service:

```cypher
MATCH (vs:VirtualService)-[:ROUTES_TRAFFIC_FOR]->(svc:Service {name: 'checkout-service'})
MATCH (vs)-[:ROUTES_TO_SUBSET]->(dr:DestinationRule)
MATCH (dr)-[:SELECTS_SUBSET_PODS {subset_name: 'canary'}]->(pod:Pod)
RETURN pod.name, pod.namespace, pod.status, pod.labels
```

### 2. Trace Traffic from Gateway to Canary Pods

Trace the complete traffic path from an Istio Gateway to specific canary pods:

```cypher
MATCH path = (gw:IstioGateway {name: 'main-gateway'})
  -[:SELECTS_PROXY]->(proxy:Pod)
MATCH (vs:VirtualService)-[:ATTACHES_TO]->(gw)
MATCH (vs)-[:ROUTES_TRAFFIC_FOR]->(svc:Service {name: 'my-service'})
MATCH (vs)-[:ROUTES_TO_SUBSET {subset_name: 'canary'}]->(dr:DestinationRule)
MATCH (dr)-[:SELECTS_SUBSET_PODS]->(pod:Pod)
RETURN path, svc, dr, pod
```

### 3. Find Authorization Policies Affecting a Pod

Find all authorization policies that apply to a specific pod:

```cypher
MATCH (pod:Pod {name: 'backend-xyz', namespace: 'production'})
MATCH (policy:AuthorizationPolicy)-[:APPLIES_TO]->(pod)
RETURN policy.name, policy.action, policy.rules
```

### 4. Identify All Services with Traffic Splits

Find services that have weighted traffic routing (e.g., canary deployments):

```cypher
MATCH (vs:VirtualService)-[:ROUTES_TO_SUBSET]->(dr:DestinationRule)
MATCH (vs)-[:ROUTES_TRAFFIC_FOR]->(svc:Service)
MATCH (dr)-[:SELECTS_SUBSET_PODS]->(pod:Pod)
WITH svc, dr, COUNT(DISTINCT pod) as pod_count
RETURN svc.name, svc.namespace, dr.name, 
       dr.subsets as subsets, pod_count
ORDER BY svc.name
```

### 5. Find Pods Without mTLS Enforcement

Find pods that don't have strict mTLS enforced:

```cypher
MATCH (pod:Pod)
WHERE NOT EXISTS {
  MATCH (pa:PeerAuthentication {mtls_mode: 'STRICT'})-[:APPLIES_TO]->(pod)
}
RETURN pod.name, pod.namespace, pod.labels
```

### 6. Check Security Posture for a Service

Get all security policies for a service's pods:

```cypher
MATCH (svc:Service {name: 'payment-service'})-[:SELECTS_PODS]->(pod:Pod)
OPTIONAL MATCH (authz:AuthorizationPolicy)-[:APPLIES_TO]->(pod)
OPTIONAL MATCH (peer:PeerAuthentication)-[:APPLIES_TO]->(pod)
OPTIONAL MATCH (req:RequestAuthentication)-[:APPLIES_TO]->(pod)
RETURN pod.name,
       COLLECT(DISTINCT authz.name) as authz_policies,
       COLLECT(DISTINCT peer.name) as peer_auth_policies,
       COLLECT(DISTINCT req.name) as req_auth_policies
```

### 7. Analyze Canary Rollout Progress

Find the distribution of traffic across service versions:

```cypher
MATCH (vs:VirtualService)-[:ROUTES_TRAFFIC_FOR]->(svc:Service {name: 'api-service'})
MATCH (vs)-[r:ROUTES_TO_SUBSET]->(dr:DestinationRule)
MATCH (dr)-[:SELECTS_SUBSET_PODS]->(pod:Pod)
RETURN r.subset_name as version,
       r.weight as traffic_weight,
       COUNT(pod) as pod_count,
       COLLECT(DISTINCT pod.status) as pod_statuses
ORDER BY r.weight DESC
```

### 8. Find Misconfigured VirtualServices

Find VirtualServices that reference non-existent services:

```cypher
MATCH (vs:VirtualService)
WHERE NOT EXISTS {
  MATCH (vs)-[:ROUTES_TRAFFIC_FOR]->(:Service)
}
RETURN vs.name, vs.namespace, vs.hosts
```

## Troubleshooting

### Istio Handlers Not Loading

**Problem**: Istio resources aren't being indexed in the graph.

**Solution**:

1. Check if Istio CRDs are installed:
   ```bash
   kubectl get crd | grep istio
   ```

2. Verify RBAC permissions:
   ```bash
   kubectl describe clusterrole kkbase-watcher
   ```

3. Check kkbase logs:
   ```bash
   kubectl logs -f deployment/kkbase-watcher
   ```

### CRD Version Mismatch

**Problem**: Istio CRDs are installed but handlers fail to register.

**Solution**: kkbase supports Istio v1 APIs (`networking.istio.io/v1` and `security.istio.io/v1`). Ensure your Istio version is 1.20+. For older versions, you may need to update Istio.

### Missing Relationships

**Problem**: Nodes are created but relationships are missing.

**Solution**:

1. **Check selector labels**: Ensure pods have the correct labels for DestinationRule subsets and policy selectors.

2. **Verify service names**: VirtualService hosts must match actual Kubernetes Service names.

3. **Check namespace references**: Cross-namespace references require proper RBAC and may need ReferenceGrant resources (for Gateway API integration).

## Integration with Gateway API

When both Istio and Gateway API resources are present, kkbase tracks both independently. Some organizations use Istio Gateways for east-west traffic and Gateway API for north-south traffic.

To query both:

```cypher
// Find all ingress points (both Gateway API and Istio)
MATCH (gw:Gateway) RETURN gw.name, 'Gateway API' as type
UNION
MATCH (gw:IstioGateway) RETURN gw.name, 'Istio' as type
```

## Best Practices

1. **Use Meaningful Subset Names**: Name DestinationRule subsets clearly (e.g., `canary`, `stable`, `v2`) to make queries easier.

2. **Label Pods Consistently**: Ensure pods have version labels that match DestinationRule subset selectors.

3. **Monitor Policy Coverage**: Regularly query for pods without security policies to identify gaps.

4. **Document Traffic Splits**: Use annotations on VirtualServices to document the purpose of traffic splits.

5. **Namespace Isolation**: Use namespace-scoped policies where possible to reduce the blast radius of misconfigurations.

## Reference

- [Istio Documentation](https://istio.io/latest/docs/)
- [Istio API Reference](https://istio.io/latest/docs/reference/config/)
- [Gateway API Integration](./GATEWAY_API.md)
- [Adding Custom Handlers](./ADDING_HANDLERS.md)

