#!/usr/bin/env bash
# Creates a Kind cluster for Moca E2E tests.

set -euo pipefail

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
# shellcheck source=/dev/null
source "${SCRIPT_DIR}/lib.sh"

MANIFESTS_DIR="${E2E_DIR}/manifests/base"

log_info "=== Setting up Kind cluster for Moca E2E tests ==="

# When building fresh images, force a clean Kind cluster. Stale clusters
# retain old image SHAs under the same name in containerd's snapshotter,
# leading to silent ErrImageNeverPull when pods try to use the new build.
# CI runners always start clean so this is a no-op there; locally it's
# the difference between "passes first try" and "fails mysteriously".
if [ "${FW_SKIP_BUILD:-false}" != "true" ] \
    && kind get clusters 2>/dev/null | grep -q "^${KIND_CLUSTER_NAME}$"; then
    log_info "FW_SKIP_BUILD=false; deleting stale cluster '${KIND_CLUSTER_NAME}' for clean image state"
    kind delete cluster --name "${KIND_CLUSTER_NAME}" 2>&1 | tail -3 || true
fi

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
