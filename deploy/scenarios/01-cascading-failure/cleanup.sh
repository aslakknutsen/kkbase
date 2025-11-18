#!/bin/bash
set -e

SCENARIO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "🧹 Cleaning up Scenario 1..."
echo ""

if [ -d /tmp/scenario-01-output ]; then
  echo "Removing deployments..."
  kubectl delete -f /tmp/scenario-01-output/shopflow-ecommerce/10-services/payment-*.yaml --ignore-not-found=true
  kubectl delete -f /tmp/scenario-01-output/shopflow-ecommerce/10-services/order-management-*.yaml --ignore-not-found=true
  kubectl delete -f /tmp/scenario-01-output/shopflow-ecommerce/10-services/checkout-*.yaml --ignore-not-found=true
  kubectl delete -f /tmp/scenario-01-output/shopflow-ecommerce/10-services/api-gateway-*.yaml --ignore-not-found=true
  
  echo "Removing namespaces (keeping if other scenarios use them)..."
  kubectl delete namespace sf-payments --ignore-not-found=true 2>/dev/null || true
  kubectl delete namespace sf-orders --ignore-not-found=true 2>/dev/null || true
  kubectl delete namespace sf-shopping --ignore-not-found=true 2>/dev/null || true
  kubectl delete namespace sf-gateway --ignore-not-found=true 2>/dev/null || true
  
  echo "Cleaning temp files..."
  rm -rf /tmp/scenario-01-output
else
  echo "No temp files found (already cleaned or setup didn't complete)"
fi

echo ""
echo "✅ Cleanup complete"

