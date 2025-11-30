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
./testgen generate examples/ecommerce-full/app.yaml -o /tmp/scenario-01-output -i quay.io/aslakknutsen/kkbase-testservice:latest
echo ""

# Deploy services
echo "🚀 Deploying services..."
kubectl apply -f /tmp/scenario-01-output/shopflow-ecommerce/00-namespaces.yaml

for service in api-gateway checkout order-management payment; do
  kubectl apply -f /tmp/scenario-01-output/shopflow-ecommerce/10-services/${service}-deployment.yaml
  kubectl apply -f /tmp/scenario-01-output/shopflow-ecommerce/10-services/${service}-service.yaml
  kubectl apply -f /tmp/scenario-01-output/shopflow-ecommerce/10-services/${service}-servicemonitor.yaml
done
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

echo "╔════════════════════════════════════════════════════════╗"
echo "║  ✅ Setup Complete!                                    ║"
echo "╚════════════════════════════════════════════════════════╝"
echo ""
echo "🚀 Next steps:"
echo "   Run the failure injection: ./run.sh"
echo "   Or use: make scenario-01-run"
echo ""
echo "🧹 Cleanup: ./cleanup.sh"
echo ""

