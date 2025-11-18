# Scenario 1: Cascading Service Failure

**Complexity:** ⭐ Easy  
**Graph Edges:** Service -[CALLS/FAILED_CALL_TO]-> Service  
**Duration:** ~2 minutes  
**Issue:** https://github.com/aslakknutsen/kkbase/issues/2

## Overview

Tests the agent's ability to traverse multi-hop service dependencies. Alert fires on `api-gateway` when it sees 503 errors from `checkout`, but the root cause is 3 hops downstream in the `payment` service.

## Service Chain

```
api-gateway (sf-gateway)
    ↓ CALLS
checkout (sf-shopping)
    ↓ CALLS
order-management (sf-orders)
    ↓ CALLS
payment (sf-payments) ← ROOT CAUSE (injected 503 errors)
```

## Quick Start

```bash
# From this directory
./setup.sh

# In another terminal, watch investigation
kubectl logs -n default -l app=kkbase-integrated --tail=100 -f

# When done
./cleanup.sh
```

## What Happens

1. Script deploys 4 services from ecommerce-full example
2. Injects 503 errors (100% rate, 60s duration) in payment service
3. Generates traffic through api-gateway
4. Alert fires: "api-gateway seeing upstream errors to checkout"
5. Agent receives alert and begins investigation
6. Agent should traverse: api-gateway → checkout → order-management → payment
7. Agent identifies payment as leaf service and root cause

## Expected Agent Investigation

The agent should execute these queries (or equivalent):

```cypher
// 1. What does api-gateway call?
MATCH (gw:Service {name: "api-gateway"})-[c:CALLS|FAILED_CALL_TO]->(downstream:Service)
WHERE c.status_code =~ "5.."
RETURN downstream.name, c.status_code

// 2. Follow the chain - what does checkout call?
MATCH (checkout:Service {name: "checkout"})-[c:CALLS|FAILED_CALL_TO]->(downstream:Service)
WHERE c.status_code =~ "5.."
RETURN downstream.name, c.status_code

// 3. What does order-management call?
MATCH (order:Service {name: "order-management"})-[c:CALLS|FAILED_CALL_TO]->(downstream:Service)
WHERE c.status_code =~ "5.."
RETURN downstream.name, c.status_code

// 4. Is payment a leaf service?
MATCH (payment:Service {name: "payment"})-[c:CALLS]->(downstream:Service)
RETURN count(downstream)  // Should be 0
```

## Success Criteria

- ✅ Alert fires on api-gateway within 30 seconds
- ✅ Agent receives webhook
- ✅ Agent identifies payment as root cause
- ✅ Agent explains error cascaded upstream
- ✅ Agent recommends investigating payment service

## Troubleshooting

**Alert not firing:**
```bash
# Check if traffic is flowing
kubectl logs -n sf-gateway deployment/api-gateway --tail=20

# Check Prometheus metrics
kubectl port-forward -n monitoring svc/kube-prometheus-kube-prometheus 9090:9090
# Query: rate(http_client_requests_total{app="api-gateway",status_code=~"5.."}[2m])
```

**Behavior not injected:**
```bash
# Verify behavior endpoint responds
kubectl exec -n sf-payments deployment/payment -- curl -s http://localhost:8080/health
```

**Investigation not triggered:**
```bash
# Check webhook config
kubectl get alertmanagerconfig -n monitoring

# Check kkbase logs for webhook errors
kubectl logs -n default -l app=kkbase-integrated --tail=50 | grep webhook
```

