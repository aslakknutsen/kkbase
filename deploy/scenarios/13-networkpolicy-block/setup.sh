#!/bin/bash
set -e

SCENARIO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCENARIO_DIR/../common/functions.sh"

echo "╔════════════════════════════════════════════════════════╗"
echo "║  Scenario 13: Cross-Namespace Network Policy Block    ║"
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
./testgen generate examples/ecommerce-full/app.yaml -o /tmp/scenario-13-output -i quay.io/aslakknutsen/kkbase-testservice:latest
echo ""

# Deploy namespaces
echo "🚀 Deploying namespaces..."
kubectl apply -f /tmp/scenario-13-output/shopflow-ecommerce/00-namespaces.yaml
echo ""

# Deploy services (api-gateway in sf-gateway, product-catalog in sf-products)
echo "🚀 Deploying api-gateway (sf-gateway namespace)..."
kubectl apply -f /tmp/scenario-13-output/shopflow-ecommerce/10-services/api-gateway-deployment.yaml
kubectl apply -f /tmp/scenario-13-output/shopflow-ecommerce/10-services/api-gateway-service.yaml
kubectl apply -f /tmp/scenario-13-output/shopflow-ecommerce/10-services/api-gateway-servicemonitor.yaml

echo "🚀 Deploying product-catalog (sf-products namespace)..."
kubectl apply -f /tmp/scenario-13-output/shopflow-ecommerce/10-services/product-catalog-deployment.yaml
kubectl apply -f /tmp/scenario-13-output/shopflow-ecommerce/10-services/product-catalog-service.yaml
kubectl apply -f /tmp/scenario-13-output/shopflow-ecommerce/10-services/product-catalog-servicemonitor.yaml
echo ""

# Wait for pods
echo "⏳ Waiting for pods to be ready..."
wait_for_pods "sf-gateway" "app=api-gateway"
wait_for_pods "sf-products" "app=product-catalog"
echo "  ✅ All pods ready"
echo ""

# Apply NetworkPolicy that blocks cross-namespace traffic
echo "🔒 Applying restrictive NetworkPolicy to sf-products namespace..."
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
    - podSelector: {}  # Only allows traffic from same namespace
EOF
echo "  ✅ NetworkPolicy applied"
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
echo "   Or use: make scenario-13-run"
echo ""
echo "⚠️  Note: This scenario requires NetworkPolicy support"
echo "   (CNI plugin like Calico, Cilium, etc.)"
echo ""
echo "🧹 Cleanup: ./cleanup.sh"
echo ""



