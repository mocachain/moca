#!/usr/bin/env bash
# Creates a Kind cluster for Moca E2E tests.

set -euo pipefail

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
source "${SCRIPT_DIR}/lib.sh"

MANIFESTS_DIR="${E2E_DIR}/manifests/base"

log_info "=== Setting up Kind cluster for Moca E2E tests ==="

# Check if cluster already exists
if kind get clusters 2>/dev/null | grep -q "^${KIND_CLUSTER_NAME}$"; then
    log_info "Kind cluster '${KIND_CLUSTER_NAME}' already exists"
else
    log_info "Creating Kind cluster '${KIND_CLUSTER_NAME}'..."
    kind create cluster \
        --name "${KIND_CLUSTER_NAME}" \
        --config "${MANIFESTS_DIR}/kind-config.yaml" \
        --wait 60s
fi

log_info "Verifying cluster..."
kubectl cluster-info --context "kind-${KIND_CLUSTER_NAME}"
kubectl get nodes

log_success "Kind cluster '${KIND_CLUSTER_NAME}' is ready"
