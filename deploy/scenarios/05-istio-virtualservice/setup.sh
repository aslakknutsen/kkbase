#!/bin/bash
set -e

SCENARIO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCENARIO_DIR/../common/functions.sh"

echo "╔════════════════════════════════════════════════════════╗"
echo "║  Scenario 5: Istio VirtualService Misconfiguration    ║"
echo "╚════════════════════════════════════════════════════════╝"
echo ""

# Check prerequisites
check_prerequisites
check_kkbase

# Check Istio is installed
echo "🔍 Checking Istio installation..."
if ! kubectl get namespace istio-system &>/dev/null; then
  echo "ERROR: Istio not installed (istio-system namespace not found)"
  echo ""
  echo "Install Istio first:"
  echo "  istioctl install --set profile=demo"
  exit 1
fi

if ! kubectl get pods -n istio-system -l app=istiod --no-headers 2>/dev/null | grep -q Running; then
  echo "ERROR: istiod not running"
  echo "Check Istio installation: kubectl get pods -n istio-system"
  exit 1
fi
echo "  ✅ Istio is installed and running"
echo ""

# Find testapp
TESTAPP_DIR=$(find_testapp)
echo "Using testapp: $TESTAPP_DIR"
echo ""

# Generate manifests
echo "📦 Generating manifests from ecommerce-full..."
cd "$TESTAPP_DIR"
./testgen generate examples/ecommerce-full/app.yaml -o /tmp/scenario-05-output -i quay.io/aslakknutsen/kkbase-testservice:latest
echo ""

# Deploy namespace with Istio injection
echo "🚀 Deploying namespace with Istio sidecar injection..."
kubectl apply -f /tmp/scenario-05-output/shopflow-ecommerce/00-namespaces.yaml
kubectl label namespace sf-payments istio-injection=enabled --overwrite
echo ""

# Deploy payment service
echo "🚀 Deploying payment service..."
kubectl apply -f /tmp/scenario-05-output/shopflow-ecommerce/10-services/payment-deployment.yaml
kubectl apply -f /tmp/scenario-05-output/shopflow-ecommerce/10-services/payment-service.yaml
kubectl apply -f /tmp/scenario-05-output/shopflow-ecommerce/10-services/payment-servicemonitor.yaml
echo ""

# Wait for pods with sidecar
echo "⏳ Waiting for payment pod to be ready (with Istio sidecar)..."
wait_for_pods "sf-payments" "app=payment"
echo "  ✅ Payment pod ready"
echo ""

# Verify sidecar injection
if kubectl get pods -n sf-payments -l app=payment -o jsonpath='{.items[0].spec.containers[*].name}' | grep -q istio-proxy; then
  echo "  ✅ Istio sidecar injected"
else
  echo "  ⚠️  WARNING: Istio sidecar not detected. Scenario may not work correctly."
  echo "      Try: kubectl rollout restart deployment/payment -n sf-payments"
fi
echo ""

# Apply Istio mesh resources (VirtualService and DestinationRule)
echo "🌐 Applying Istio mesh configuration..."

# Check if mesh resources exist in testapp output
if [ -d "/tmp/scenario-05-output/shopflow-ecommerce/40-mesh" ]; then
  # Apply existing mesh resources if available
  kubectl apply -f /tmp/scenario-05-output/shopflow-ecommerce/40-mesh/payment-*.yaml 2>/dev/null || true
fi

# Create/update DestinationRule with subsets
cat <<EOF | kubectl apply -f -
apiVersion: networking.istio.io/v1
kind: DestinationRule
metadata:
  name: payment
  namespace: sf-payments
spec:
  host: payment.sf-payments.svc.cluster.local
  subsets:
  - name: v1
    labels:
      version: v1
  - name: v2
    labels:
      version: v2
EOF

# Create VirtualService routing to v1 (healthy state)
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
        subset: v1
      weight: 100
EOF

echo "  ✅ Istio VirtualService and DestinationRule applied"
echo ""

# Apply alert rules
echo "🔔 Applying alert rules..."
kubectl apply -f "$SCENARIO_DIR/alert-rules.yaml"
echo ""

echo "╔════════════════════════════════════════════════════════╗"
echo "║  ✅ Setup Complete!                                    ║"
echo "╚════════════════════════════════════════════════════════╝"
echo ""
echo "Current state:"
echo "  - Payment pods have label: version=v1"
echo "  - VirtualService routes to: subset v1 (working)"
echo "  - DestinationRule defines: v1 and v2 subsets"
echo ""
echo "🚀 Next steps:"
echo "   Run the misconfiguration injection: ./run.sh"
echo "   Or use: make scenario-05-run"
echo ""
echo "🧹 Cleanup: ./cleanup.sh"
echo ""

