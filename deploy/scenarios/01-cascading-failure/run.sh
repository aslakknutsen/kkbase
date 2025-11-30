#!/bin/bash
set -e

SCENARIO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCENARIO_DIR/../common/functions.sh"

echo "╔════════════════════════════════════════════════════════╗"
echo "║  Scenario 1: Running Failure Injection                ║"
echo "╚════════════════════════════════════════════════════════╝"
echo ""

# Check that pods are running
echo "⏳ Checking pods..."
if ! kubectl get pods -n sf-gateway -l app=api-gateway &>/dev/null; then
  echo "ERROR: api-gateway not found. Run ./setup.sh first"
  exit 1
fi

wait_for_pods "sf-gateway" "app=api-gateway"
wait_for_pods "sf-shopping" "app=checkout"
wait_for_pods "sf-orders" "app=order-management"
wait_for_pods "sf-payments" "app=payment"
echo "  ✅ All pods ready"
echo ""

# Generate traffic with injected failures
echo "🌐 Generating traffic with 503 errors (50 requests over ~50 seconds)..."
echo "  (behavior: error=503:1.0:duration=60s)"
for i in {1..50}; do
  kubectl exec -n sf-gateway deployment/api-gateway -- \
    curl -s "http://localhost:8080/api/v1/checkout?behavior=error=503:1.0:duration=60s" > /dev/null 2>&1 &
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
echo "⏰ Behavior will auto-clear after 60s"
echo "🔁 Re-run: ./run.sh"
echo "🧹 Cleanup: ./cleanup.sh"
echo ""

