# Scenario 13: Cross-Namespace Network Policy Block

**Complexity:** ⭐⭐⭐ High  
**Graph Edges:** Service -[CALLS]-> Service, Service -[IN_NAMESPACE]-> Namespace  
**Duration:** ~3 minutes  
**Issue:** https://github.com/aslakknutsen/kkbase/issues/14  
**Infrastructure:** Requires NetworkPolicy support (CNI plugin like Calico, Cilium)

## Overview

Tests the agent's ability to diagnose network-level blocking vs application errors. api-gateway (sf-gateway namespace) cannot reach product-catalog (sf-products namespace) due to NetworkPolicy blocking cross-namespace traffic.

**Key Diagnostic Insight:** Timeout (status 000) indicates network-level blocking, while 5xx errors indicate application issues.

## Service Topology

```
api-gateway (sf-gateway namespace)
    ↓ CALLS (cross-namespace)
product-catalog (sf-products namespace) ← BLOCKED by NetworkPolicy
```

## NetworkPolicy Configuration

```yaml
# Only allows traffic from same namespace (sf-products)
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: deny-cross-namespace
  namespace: sf-products
spec:
  podSelector: {}
  policyTypes:
  - Ingress
  ingress:
  - from:
    - podSelector: {}  # Same namespace only
```

## Quick Start

```bash
# From this directory
./setup.sh    # Deploy services and NetworkPolicy
./run.sh      # Generate cross-namespace traffic (will timeout)

# Or using make from repo root
make scenario-13      # Setup
make scenario-13-run  # Run

# In another terminal, watch investigation
kubectl logs -n default -l app=kkbase-integrated --tail=100 -f

# When done
./cleanup.sh  # or: make scenario-13-cleanup
```

## What Happens

**Setup phase (`./setup.sh`):**
1. Deploys api-gateway in sf-gateway namespace
2. Deploys product-catalog in sf-products namespace
3. Creates NetworkPolicy that blocks cross-namespace ingress
4. Applies alert rules to Prometheus

**Run phase (`./run.sh`):**
1. Generates traffic from api-gateway to product-catalog
2. Connections timeout (status 000) due to NetworkPolicy
3. Alert fires: "CrossNamespaceConnectionTimeout"
4. Agent receives alert and begins investigation
5. Agent should check namespaces, verify pods are healthy, diagnose NetworkPolicy block

## Expected Agent Investigation

The agent should execute these queries (or equivalent):

```cypher
// 1. Check the failing service call
MATCH (caller:Service {name: "api-gateway"})-[c:FAILED_CALL_TO]->(target:Service {name: "product-catalog"})
RETURN caller.name, target.name, c.status_code, c.error_message

// 2. Check if services are in different namespaces
MATCH (caller:Service {name: "api-gateway"})-[:IN_NAMESPACE]->(callerNs:Namespace)
MATCH (target:Service {name: "product-catalog"})-[:IN_NAMESPACE]->(targetNs:Namespace)
RETURN callerNs.name as caller_namespace, targetNs.name as target_namespace

// 3. Verify target pods are healthy (rules out pod issues)
MATCH (s:Service {name: "product-catalog"})-[:SELECTS_PODS]->(p:Pod)
RETURN p.name, p.status, p.ready
```

## Diagnostic Logic

| Symptom | Namespace | Target Health | Diagnosis |
|---------|-----------|---------------|-----------|
| Timeout (000) | Cross-namespace | Healthy | NetworkPolicy block |
| Timeout (000) | Same namespace | Healthy | Other network issue |
| 5xx errors | Any | Any | Application error |

## Success Criteria

- Alert fires on connection timeout within 30 seconds
- Agent receives webhook
- Agent identifies cross-namespace communication
- Agent verifies target pods are healthy
- Agent diagnoses NetworkPolicy as root cause
- Agent recommends updating NetworkPolicy to allow source namespace

## Fix

To allow traffic from sf-gateway namespace:

```bash
kubectl apply -f - <<EOF
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: deny-cross-namespace
  namespace: sf-products
spec:
  podSelector: {}
  policyTypes:
  - Ingress
  ingress:
  - from:
    - podSelector: {}
    - namespaceSelector:
        matchLabels:
          kubernetes.io/metadata.name: sf-gateway
EOF
```

## Troubleshooting

**NetworkPolicy not enforced:**
```bash
# Check CNI plugin supports NetworkPolicy
kubectl get pods -n kube-system | grep -E 'calico|cilium|weave'

# If using Kind, NetworkPolicy may not be enforced by default
# Use Calico: https://docs.tigera.io/calico/latest/getting-started/kubernetes/kind
```

**Alert not firing:**
```bash
# Check if timeout metrics are being recorded
kubectl port-forward -n monitoring svc/kube-prometheus-kube-prometheus 9090:9090
# Query: http_client_requests_total{status_code="000"}
```

**Connection succeeds (should timeout):**
```bash
# Verify NetworkPolicy exists
kubectl get networkpolicy -n sf-products

# Test connectivity manually
kubectl exec -n sf-gateway deployment/api-gateway -- \
  curl -v --max-time 5 http://product-catalog.sf-products.svc.cluster.local:8080/
```



