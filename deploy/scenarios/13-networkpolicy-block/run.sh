#!/bin/bash
set -e

SCENARIO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCENARIO_DIR/../common/functions.sh"

echo "╔════════════════════════════════════════════════════════╗"
echo "║  Scenario 13: Running Cross-Namespace Traffic Test    ║"
echo "╚════════════════════════════════════════════════════════╝"
echo ""

# Check that pods are running
echo "⏳ Checking pods..."
if ! kubectl get pods -n sf-gateway -l app=api-gateway &>/dev/null; then
  echo "ERROR: api-gateway not found. Run ./setup.sh first"
  exit 1
fi

wait_for_pods "sf-gateway" "app=api-gateway"
wait_for_pods "sf-products" "app=product-catalog"
echo "  ✅ All pods ready"
echo ""

# Verify NetworkPolicy is in place
echo "🔒 Verifying NetworkPolicy..."
if ! kubectl get networkpolicy deny-cross-namespace -n sf-products &>/dev/null; then
  echo "ERROR: NetworkPolicy not found. Run ./setup.sh first"
  exit 1
fi
echo "  ✅ NetworkPolicy in place"
echo ""

# Generate cross-namespace traffic that will timeout
echo "🌐 Generating cross-namespace traffic (will timeout due to NetworkPolicy)..."
echo "  Source: api-gateway (sf-gateway namespace)"
echo "  Target: product-catalog.sf-products.svc.cluster.local"
echo ""
echo "  Each request will timeout after 5 seconds (connection blocked)"
echo ""

for i in {1..20}; do
  echo "  Request $i/20..."
  # Use --max-time 5 to timeout after 5 seconds
  # The connection will be dropped by NetworkPolicy, resulting in status 000
  kubectl exec -n sf-gateway deployment/api-gateway -- \
    curl -s --max-time 5 -o /dev/null -w "Status: %{http_code}\n" \
    "http://product-catalog.sf-products.svc.cluster.local:8080/api/v1/products" 2>/dev/null || echo "  Connection timed out (expected)"
  sleep 2
done
echo ""

echo "╔════════════════════════════════════════════════════════╗"
echo "║  ✅ Run Complete!                                      ║"
echo "╚════════════════════════════════════════════════════════╝"
echo ""
echo "📊 Monitor investigation:"
echo "   kubectl logs -n default -l app=kkbase-integrated --tail=100 -f"
echo ""
echo "🔍 Check alerts:"
echo "   kubectl port-forward -n monitoring svc/kube-prometheus-kube-prometheus 9090:9090"
echo "   Open: http://localhost:9090/alerts"
echo ""
echo "🔧 To fix the NetworkPolicy (allow sf-gateway namespace):"
cat << 'FIXEOF'
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
FIXEOF
echo ""
echo "🔁 Re-run: ./run.sh"
echo "🧹 Cleanup: ./cleanup.sh"
echo ""



