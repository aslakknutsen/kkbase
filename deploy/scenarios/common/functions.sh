#!/bin/bash

check_prerequisites() {
  command -v kubectl >/dev/null 2>&1 || { echo "ERROR: kubectl required"; exit 1; }
  command -v testgen >/dev/null 2>&1 || { 
    echo "ERROR: testgen required"
    echo "Install from: https://github.com/aslakknutsen/kkbase-testapp"
    echo "Or add to PATH"
    exit 1
  }
  kubectl cluster-info >/dev/null 2>&1 || { echo "ERROR: No kubernetes cluster"; exit 1; }
}

wait_for_pods() {
  local namespace=$1
  local label=$2
  local timeout=${3:-120}
  echo "  Waiting for pods with label=$label in namespace=$namespace..."
  kubectl wait --for=condition=ready pod -l "$label" -n "$namespace" --timeout="${timeout}s" 2>/dev/null
}

check_kkbase() {
  if ! kubectl get deployment kkbase-integrated -n default &>/dev/null; then
    echo "WARNING: kkbase-integrated not found in default namespace"
    echo "Make sure kkbase is deployed before running scenarios"
    echo ""
  fi
  return 0
}

find_testapp() {
  local testapp_dir="${TESTAPP_DIR:-$HOME/kkbase-testapp}"
  if [ ! -d "$testapp_dir" ]; then
    echo "ERROR: kkbase-testapp not found at $testapp_dir"
    echo ""
    echo "Options:"
    echo "  1. Set TESTAPP_DIR environment variable"
    echo "  2. Clone to $HOME/kkbase-testapp"
    echo "     git clone https://github.com/aslakknutsen/kkbase-testapp.git ~/kkbase-testapp"
    exit 1
  fi
  echo "$testapp_dir"
}

