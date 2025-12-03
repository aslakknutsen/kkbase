# Scenario 5: Istio VirtualService Misconfiguration

**Complexity:** ⭐⭐⭐ High  
**Graph Edges:** VirtualService -[ROUTES_TRAFFIC_FOR]-> Service, DestinationRule -[DEFINES_POLICY_FOR]-> Service, Service -[SELECTS_PODS]-> Pod  
**Duration:** ~3 minutes  
**Infrastructure:** Requires Istio service mesh  
**Issue:** https://github.com/aslakknutsen/kkbase/issues/6

## Overview

Tests the agent's ability to correlate Istio mesh configuration across 3 layers. Envoy returns generic "503 no healthy upstream" for the payment service because the VirtualService routes to subset `v2`, but only `v1` pods exist.

## Prerequisites

- Istio service mesh installed in the cluster
- Sidecar injection enabled for target namespaces

To verify Istio is installed:
```bash
kubectl get pods -n istio-system
kubectl get namespace -L istio-injection
```

## Configuration Mismatch

```
VirtualService: payment
  route: subset v2 (100% weight)

DestinationRule: payment
  subsets: [{name: v2, labels: {version: v2}}]

Pods: payment-xxx
  labels: {version: v1}  ← Only v1 exists!
```

## Quick Start

```bash
# From this directory
./setup.sh    # Deploy services, Istio resources, and alert rules
./run.sh      # Patch VirtualService to route to non-existent subset

# Or using make from repo root
make scenario-05      # Setup
make scenario-05-run  # Run

# In another terminal, watch investigation
kubectl logs -n default -l app=kkbase-integrated --tail=100 -f

# When done
./cleanup.sh  # or: make scenario-05-cleanup
```

## What Happens

**Setup phase (`./setup.sh`):**
1. Verifies Istio is installed
2. Deploys payment service from ecommerce-full example
3. Applies Istio VirtualService and DestinationRule
4. Applies alert rules to Prometheus

**Run phase (`./run.sh`):**
1. Patches VirtualService to route 100% traffic to subset `v2`
2. Generates traffic through the payment service
3. Traffic fails with 503 "no healthy upstream" (subset v2 has no pods)
4. Alert fires: "Envoy 503 for payment cluster"
5. Agent receives alert and begins investigation
6. Agent should traverse: VirtualService → DestinationRule → Pod labels
7. Agent identifies that subset v2 requires `version:v2` but pods only have `version:v1`

## Expected Agent Investigation

The agent should execute queries like:

```cypher
// 1. What VirtualService affects this service?
MATCH (vs:VirtualService)-[:ROUTES_TRAFFIC_FOR]->(s:Service {name: "payment"})
RETURN vs.name, vs.http_routes

// 2. What subsets are defined?
MATCH (dr:DestinationRule)-[:DEFINES_POLICY_FOR]->(s:Service {name: "payment"})
RETURN dr.name, dr.subsets

// 3. What pods exist and their labels?
MATCH (s:Service {name: "payment"})-[:SELECTS_PODS]->(p:Pod)
RETURN p.name, p.labels, p.status

// 4. Count pods matching subset v2 requirements
MATCH (s:Service {name: "payment"})-[:SELECTS_PODS]->(p:Pod)
WHERE p.labels CONTAINS "version:v2"
RETURN count(p) as v2_pod_count  // Should be 0
```

## Success Criteria

- Alert fires on Envoy 503 within 30 seconds
- Agent receives webhook
- Agent queries VirtualService routing configuration
- Agent identifies subset v2 requirement from routing rules
- Agent checks DestinationRule subset definitions
- Agent counts pods matching subset labels (should be 0)
- Agent correctly diagnoses subset mismatch as root cause
- Agent recommends: deploy v2 pods, route to v1, or remove subset routing

## Troubleshooting

**Istio not installed:**
```bash
# Check Istio installation
kubectl get pods -n istio-system
istioctl version

# If missing, install Istio
istioctl install --set profile=demo
```

**Sidecar not injected:**
```bash
# Check if namespace has injection enabled
kubectl get namespace sf-payments -L istio-injection

# Enable injection
kubectl label namespace sf-payments istio-injection=enabled
kubectl rollout restart deployment/payment -n sf-payments
```

**VirtualService not taking effect:**
```bash
# Verify VirtualService exists
kubectl get virtualservice payment -n sf-payments -o yaml

# Check Envoy config
istioctl proxy-config routes deployment/payment -n sf-payments
```

**Alert not firing:**
```bash
# Check Istio metrics are being collected
kubectl port-forward -n istio-system svc/prometheus 9090:9090
# Query: istio_requests_total{destination_service="payment.sf-payments.svc.cluster.local"}

# Or check Envoy stats directly
kubectl exec -n sf-payments deployment/payment -c istio-proxy -- \
  curl -s localhost:15000/stats | grep upstream_rq_503
```

**Investigation not triggered:**
```bash
# Check webhook config
kubectl get alertmanagerconfig -n monitoring

# Check kkbase logs for webhook errors
kubectl logs -n default -l app=kkbase-integrated --tail=50 | grep webhook
```

