#!/bin/bash
set -e

SCENARIO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCENARIO_DIR/../common/functions.sh"

echo "╔════════════════════════════════════════════════════════╗"
echo "║  Scenario 5: Injecting VirtualService Misconfiguration║"
echo "╚════════════════════════════════════════════════════════╝"
echo ""

# Check that pods are running
echo "⏳ Checking pods..."
if ! kubectl get pods -n sf-payments -l app=payment &>/dev/null; then
  echo "ERROR: payment not found. Run ./setup.sh first"
  exit 1
fi

wait_for_pods "sf-payments" "app=payment"
echo "  ✅ Payment pod ready"
echo ""

# Show current pod labels
echo "📋 Current payment pod labels:"
kubectl get pods -n sf-payments -l app=payment -o jsonpath='{range .items[*]}{.metadata.name}: {.metadata.labels}{"\n"}{end}'
echo ""

# Patch VirtualService to route to non-existent subset v2
echo "🔧 Patching VirtualService to route to subset v2 (which doesn't exist)..."
cat <<EOF | kubectl apply -f -
apiVersion: networking.istio.io/v1
kind: VirtualService
metadata:
  name: payment
  namespace: sf-payments
spec:
  hosts:
  - payment.sf-payments.svc.cluster.local
  http:
  - route:
    - destination:
        host: payment.sf-payments.svc.cluster.local
        subset: v2
      weight: 100
EOF
echo "  ✅ VirtualService now routes 100% to subset v2"
echo ""

# Verify the mismatch
echo "🔍 Verifying configuration mismatch:"
echo "  VirtualService routes to: subset v2 (labels: version=v2)"
echo "  Actual pod labels:        version=v1"
echo "  Expected result:          503 'no healthy upstream'"
echo ""

# Wait a moment for Envoy to sync config
echo "⏳ Waiting for Envoy config sync (5 seconds)..."
sleep 5
echo ""

# Generate traffic (will fail with 503)
echo "🌐 Generating traffic (20 requests)..."
echo "  (All requests should fail with 503 'no healthy upstream')"
echo ""

# Get a pod that can call the payment service
# We'll use the payment pod itself to call its own service
PAYMENT_POD=$(kubectl get pods -n sf-payments -l app=payment -o jsonpath='{.items[0].metadata.name}')

for i in {1..20}; do
  kubectl exec -n sf-payments "$PAYMENT_POD" -c testservice -- \
    curl -s -o /dev/null -w "Request $i: HTTP %{http_code}\n" \
    http://payment.sf-payments.svc.cluster.local:8080/health 2>/dev/null || echo "Request $i: Failed (connection error)"
  sleep 1
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
echo "🔧 Verify Envoy sees no endpoints for v2:"
echo "   kubectl exec -n sf-payments $PAYMENT_POD -c istio-proxy -- \\"
echo "     curl -s localhost:15000/clusters | grep payment"
echo ""
echo "🔁 Re-run: ./run.sh"
echo "🧹 Cleanup: ./cleanup.sh"
echo ""

