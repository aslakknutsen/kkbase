# Gateway API Knowledge Graph Integration

## Overview

The kkbase watcher now supports the Kubernetes Gateway API, creating a comprehensive knowledge graph that models the role-oriented architecture of modern Kubernetes ingress and traffic routing.

## Architecture

The Gateway API is designed with separation of concerns across different personas:

- **Infrastructure Providers** manage `GatewayClass` resources
- **Cluster Operators** manage `Gateway` resources  
- **Application Developers** manage Route resources (`HTTPRoute`, `GRPCRoute`, etc.)
- **Application Owners** manage cross-namespace permissions with `ReferenceGrant`

The knowledge graph captures these relationships, enabling autonomous agents to perform root cause analysis and diagnostics.

## Supported Resources

### Stable API (v1)

- **GatewayClass** - Cluster-scoped templates defining gateway implementations
- **Gateway** - Load balancer configuration with listeners, ports, and TLS
- **HTTPRoute** - HTTP/HTTPS routing rules with path matching and filters
- **GRPCRoute** - gRPC-specific routing with method matching

### Experimental API (v1alpha2)

- **TCPRoute** - Layer 4 TCP routing
- **UDPRoute** - Layer 4 UDP routing  
- **TLSRoute** - SNI-based TLS routing

### Security (v1beta1)

- **ReferenceGrant** - Cross-namespace reference permissions

## Node Types

The following nodes are created in the graph:

| Node Type | Description | Cluster-Scoped |
|-----------|-------------|----------------|
| `GatewayClass` | Gateway implementation template | Yes |
| `Gateway` | Load balancer instance | No |
| `HTTPRoute` | HTTP routing rules | No |
| `GRPCRoute` | gRPC routing rules | No |
| `TCPRoute` | TCP routing rules | No |
| `UDPRoute` | UDP routing rules | No |
| `TLSRoute` | TLS routing rules | No |
| `ReferenceGrant` | Cross-namespace permission | No |

## Edge Types

The following relationships connect Gateway API resources:

| Edge Type | From | To | Description |
|-----------|------|----|-----------| 
| `IMPLEMENTED_BY` | Gateway | GatewayClass | Links gateway to its controller |
| `ATTACHES_TO` | Route | Gateway | Route binds to gateway listener |
| `FORWARDS_TO` | Route | Service | Route sends traffic to service |
| `USES_TLS_FROM` | Gateway | Secret | Gateway uses TLS certificate |
| `PERMITTED_BY` | Route | ReferenceGrant | Route is allowed by grant |
| `ALLOWS_ROUTE_TO` | ReferenceGrant | Service | Grant permits service access |
| `IN_NAMESPACE` | Resource | Namespace | Resource belongs to namespace |

## Node Properties

### GatewayClass

```json
{
  "name": "istio",
  "controller_name": "istio.io/gateway-controller",
  "description": "Istio Gateway implementation",
  "accepted": "True",
  "status_message": "Handled by Istio controller"
}
```

### Gateway

```json
{
  "name": "external",
  "namespace": "istio-system",
  "gateway_class_name": "istio",
  "listeners": "[{\"name\":\"http\",\"port\":80,\"protocol\":\"HTTP\",\"hostname\":\"*.example.com\"}]",
  "addresses": ["10.0.0.1"],
  "accepted": "True",
  "programmed": "True"
}
```

### HTTPRoute

```json
{
  "name": "orders-route",
  "namespace": "store",
  "hostnames": ["api.example.com"],
  "parent_refs": "[{\"name\":\"external\",\"namespace\":\"istio-system\"}]",
  "rule_count": 2,
  "rules": "[{\"matches\":[{\"path_type\":\"PathPrefix\",\"path_value\":\"/orders\"}],\"backends\":[\"store/orders-service\"]}]",
  "accepted": "True"
}
```

### ReferenceGrant

```json
{
  "name": "allow-store-frontend",
  "namespace": "payments",
  "from": "[{\"group\":\"gateway.networking.k8s.io\",\"kind\":\"HTTPRoute\",\"namespace\":\"store-frontend\"}]",
  "to": "[{\"group\":\"\",\"kind\":\"Service\"}]"
}
```

## Example Queries

### Find All Gateways Using a Specific GatewayClass

```cypher
MATCH (gc:GatewayClass {name: 'istio'})<-[:IMPLEMENTED_BY]-(gw:Gateway)
RETURN gw.name, gw.namespace, gw.addresses
```

### Trace Traffic Path for a Specific Hostname

```cypher
MATCH (route:HTTPRoute)-[:ATTACHES_TO]->(gw:Gateway)
      -[:IMPLEMENTED_BY]->(gc:GatewayClass)
WHERE 'api.example.com' IN route.hostnames
MATCH (route)-[:FORWARDS_TO]->(svc:Service)-[:SELECTS_PODS]->(pod:Pod)
RETURN gc.name as controller, 
       gw.name as gateway, 
       route.name as route,
       svc.name as service,
       collect(pod.name) as pods
```

### Find Routes Without Backend Pods

```cypher
MATCH (route:HTTPRoute)-[:FORWARDS_TO]->(svc:Service)
WHERE NOT (svc)-[:SELECTS_PODS]->(:Pod)
RETURN route.namespace as namespace,
       route.name as route,
       svc.name as service
```

### Identify Cross-Namespace Routing Without Grants

```cypher
MATCH (route:HTTPRoute)-[:FORWARDS_TO]->(svc:Service)
WHERE route.namespace <> svc.namespace
AND NOT (route)-[:PERMITTED_BY]->(:ReferenceGrant)
RETURN route.namespace as route_namespace,
       route.name as route,
       svc.namespace as service_namespace,
       svc.name as service
```

### Find Gateways Using Specific TLS Certificates

```cypher
MATCH (gw:Gateway)-[r:USES_TLS_FROM]->(secret:Secret)
WHERE secret.type = 'kubernetes.io/tls'
RETURN gw.name as gateway,
       gw.namespace as namespace,
       r.listener_name as listener,
       secret.name as certificate
```

### Analyze Gateway Listener Configuration

```cypher
MATCH (gw:Gateway)
WHERE gw.accepted = 'True'
RETURN gw.name as gateway,
       gw.namespace,
       gw.listeners as listener_config,
       size((gw)<-[:ATTACHES_TO]-(:HTTPRoute)) as attached_routes
```

## Diagnostic Scenarios

### Scenario 1: 503 Service Unavailable

**Problem**: Users report 503 errors for `api.example.com/orders`

**Agent Query Path**:

1. Find Gateway with listener matching hostname:
```cypher
MATCH (gw:Gateway)
WHERE gw.listeners CONTAINS 'api.example.com'
RETURN gw
```

2. Find attached HTTPRoute:
```cypher
MATCH (route:HTTPRoute)-[:ATTACHES_TO]->(gw:Gateway)
WHERE 'api.example.com' IN route.hostnames
RETURN route
```

3. Check if path `/orders` matches any rule:
```cypher
MATCH (route:HTTPRoute)
WHERE route.rules CONTAINS '/orders'
RETURN route
```

4. Inspect backend Service and Pods:
```cypher
MATCH (route:HTTPRoute)-[:FORWARDS_TO]->(svc:Service)
      -[:SELECTS_PODS]->(pod:Pod)
WHERE pod.status = 'Running' AND pod.ready = true
RETURN count(pod) as healthy_pods
```

**Diagnosis**: If no healthy pods exist, the agent concludes: "HTTPRoute is correctly configured but backend Service has no healthy pods."

### Scenario 2: TLS Certificate Issues

**Problem**: HTTPS connections fail with certificate errors

**Agent Query Path**:

1. Find Gateway using TLS:
```cypher
MATCH (gw:Gateway)-[:USES_TLS_FROM]->(secret:Secret)
WHERE gw.name = 'external'
RETURN secret
```

2. Check Secret type and data:
```cypher
MATCH (secret:Secret)
WHERE secret.type = 'kubernetes.io/tls'
RETURN secret.data_keys
```

**Diagnosis**: Agent verifies that `tls.crt` and `tls.key` exist in Secret data.

### Scenario 3: Cross-Namespace Permission Denied

**Problem**: HTTPRoute cannot forward to Service in different namespace

**Agent Query Path**:

1. Identify cross-namespace reference:
```cypher
MATCH (route:HTTPRoute)-[:FORWARDS_TO]->(svc:Service)
WHERE route.namespace <> svc.namespace
RETURN route, svc
```

2. Check for ReferenceGrant:
```cypher
MATCH (route:HTTPRoute)-[:PERMITTED_BY]->(grant:ReferenceGrant)
      -[:ALLOWS_ROUTE_TO]->(svc:Service)
WHERE route.namespace <> svc.namespace
RETURN grant
```

**Diagnosis**: If no ReferenceGrant found, agent concludes: "Cross-namespace routing requires ReferenceGrant in target namespace."

## Installation

Gateway API support is automatically enabled when CRDs are detected in the cluster.

### Install Gateway API CRDs

```bash
kubectl apply -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.2.1/standard-install.yaml
```

### Verify Detection

Check kkbase logs for Gateway API availability:

```bash
kubectl logs -n kkbase deployment/kkbase-watcher | grep "Gateway API availability"
```

Expected output:
```
Gateway API availability gatewayclass=true gateway=true httproute=true grpcroute=true ...
```

## Configuration

Gateway API handlers are optional and require no configuration. They automatically:

- Detect available CRDs using Kubernetes discovery API
- Register handlers only for installed resources
- Use dynamic informers for CRD watching
- Handle API version differences (v1, v1alpha2, v1beta1)

## Integration with Autonomous Agents

The Gateway API knowledge graph enables agents to:

1. **Understand Traffic Flow** - Trace requests from Gateway → Route → Service → Pod
2. **Diagnose Failures** - Identify missing configurations, certificates, or permissions
3. **Validate Security** - Check ReferenceGrants for cross-namespace access
4. **Assess Health** - Verify backend pods are running and ready
5. **Analyze Changes** - Track configuration drift and updates

## Performance Considerations

- Gateway API resources are typically low-volume (<100 per cluster)
- Dynamic informers use watch (not poll) for efficiency
- Cross-namespace relationship resolution is lazy (on-demand)
- Graph queries use indexed properties (name, namespace)

## Limitations

1. **API Versions**: Different route types use different API versions (v1 vs v1alpha2)
2. **Complex Matching**: Some advanced matching rules are simplified in graph properties
3. **Dynamic Backends**: Non-Service backends (e.g., external URLs) are not fully modeled
4. **Policy Attachments**: Policy CRDs (e.g., RateLimitPolicy) require separate handlers

## Future Enhancements

- [ ] Support for Gateway API Policy CRDs
- [ ] Topology visualization of Gateway→Route→Service chains
- [ ] Automatic ReferenceGrant recommendation
- [ ] Certificate expiration tracking
- [ ] Multi-cluster gateway federation
- [ ] Real-time traffic metrics integration

## References

- [Gateway API Documentation](https://gateway-api.sigs.k8s.io/)
- [Gateway API Specification](https://gateway-api.sigs.k8s.io/reference/spec/)
- [kkbase Architecture](../ARCHITECTURE.md)
- [Adding Custom Handlers](ADDING_HANDLERS.md)

