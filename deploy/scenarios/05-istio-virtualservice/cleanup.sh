#!/bin/bash
set -e

SCENARIO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "🧹 Cleaning up Scenario 5..."
echo ""

# Remove alert rules
echo "Removing alert rules..."
kubectl delete -f "$SCENARIO_DIR/alert-rules.yaml" --ignore-not-found=true

# Remove Istio resources
echo "Removing Istio mesh configuration..."
kubectl delete virtualservice payment -n sf-payments --ignore-not-found=true
kubectl delete destinationrule payment -n sf-payments --ignore-not-found=true

# Remove deployments
if [ -d /tmp/scenario-05-output ]; then
  echo "Removing payment service..."
  kubectl delete -f /tmp/scenario-05-output/shopflow-ecommerce/10-services/payment-*.yaml --ignore-not-found=true
  
  echo "Removing namespace..."
  kubectl delete namespace sf-payments --ignore-not-found=true 2>/dev/null || true
  
  echo "Cleaning temp files..."
  rm -rf /tmp/scenario-05-output
else
  echo "Removing payment service (no temp files found)..."
  kubectl delete deployment payment -n sf-payments --ignore-not-found=true
  kubectl delete service payment -n sf-payments --ignore-not-found=true
  kubectl delete servicemonitor payment -n sf-payments --ignore-not-found=true
  
  echo "Removing namespace..."
  kubectl delete namespace sf-payments --ignore-not-found=true 2>/dev/null || true
fi

echo ""
echo "✅ Cleanup complete"

