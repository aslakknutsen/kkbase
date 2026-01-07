#!/bin/bash
set -e

echo "╔════════════════════════════════════════════════════════╗"
echo "║  Scenario 13: Cleanup                                  ║"
echo "╚════════════════════════════════════════════════════════╝"
echo ""

# Remove NetworkPolicy
echo "🗑️  Removing NetworkPolicy..."
kubectl delete networkpolicy deny-cross-namespace -n sf-products --ignore-not-found
echo ""

# Remove alert rules
echo "🗑️  Removing alert rules..."
kubectl delete prometheusrule scenario-13-networkpolicy-block -n monitoring --ignore-not-found
echo ""

# Remove services
echo "🗑️  Removing api-gateway..."
kubectl delete deployment api-gateway -n sf-gateway --ignore-not-found
kubectl delete service api-gateway -n sf-gateway --ignore-not-found
kubectl delete servicemonitor api-gateway -n sf-gateway --ignore-not-found

echo "🗑️  Removing product-catalog..."
kubectl delete deployment product-catalog -n sf-products --ignore-not-found
kubectl delete service product-catalog -n sf-products --ignore-not-found
kubectl delete servicemonitor product-catalog -n sf-products --ignore-not-found
echo ""

# Optionally remove namespaces (commented out to avoid removing other resources)
# echo "🗑️  Removing namespaces..."
# kubectl delete namespace sf-gateway --ignore-not-found
# kubectl delete namespace sf-products --ignore-not-found

echo "╔════════════════════════════════════════════════════════╗"
echo "║  ✅ Cleanup Complete!                                  ║"
echo "╚════════════════════════════════════════════════════════╝"
echo ""



