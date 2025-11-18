#!/bin/bash
set -e

SCENARIO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCENARIO_DIR/../common/functions.sh"

echo "╔════════════════════════════════════════════════════════╗"
echo "║  Scenario 1: Cascading Service Failure                ║"
echo "╚════════════════════════════════════════════════════════╝"
echo ""

# Check prerequisites
check_prerequisites
check_kkbase

# Find testapp
TESTAPP_DIR=$(find_testapp)
echo "Using testapp: $TESTAPP_DIR"
echo ""

# Generate manifests
echo "📦 Generating manifests from ecommerce-full..."
cd "$TESTAPP_DIR"
./testgen generate -f examples/ecommerce-full/app.yaml -o /tmp/scenario-01-output
echo ""

# Deploy services
echo "🚀 Deploying services..."
kubectl apply -f /tmp/scenario-01-output/shopflow-ecommerce/00-namespaces.yaml
kubectl apply -f /tmp/scenario-01-output/shopflow-ecommerce/10-services/api-gateway-*.yaml
kubectl apply -f /tmp/scenario-01-output/shopflow-ecommerce/10-services/checkout-*.yaml
kubectl apply -f /tmp/scenario-01-output/shopflow-ecommerce/10-services/order-management-*.yaml
kubectl apply -f /tmp/scenario-01-output/shopflow-ecommerce/10-services/payment-*.yaml
echo ""

# Wait for pods
echo "⏳ Waiting for pods to be ready..."
wait_for_pods "sf-gateway" "app=api-gateway"
wait_for_pods "sf-shopping" "app=checkout"
wait_for_pods "sf-orders" "app=order-management"
wait_for_pods "sf-payments" "app=payment"
echo "  ✅ All pods ready"
echo ""

# Apply alert rules
echo "🔔 Applying alert rules..."
kubectl apply -f "$SCENARIO_DIR/alert-rules.yaml"
echo ""

# Inject failure
echo "💥 Injecting 503 errors in payment service..."
echo "  (60s duration with auto-recovery)"
kubectl exec -n sf-payments deployment/payment -- \
  curl -s "http://localhost:8080/?behavior=error=503:1.0:duration=60s" > /dev/null
echo ""

# Generate traffic
echo "🌐 Generating traffic (50 requests over ~50 seconds)..."
for i in {1..50}; do
  kubectl exec -n sf-gateway deployment/api-gateway -- \
    curl -s http://localhost:8080/api/v1/checkout > /dev/null 2>&1 &
  sleep 1
done
echo ""

echo "╔════════════════════════════════════════════════════════╗"
echo "║  ✅ Setup Complete!                                    ║"
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
echo "🧹 Cleanup: ./cleanup.sh"
echo ""

