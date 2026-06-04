#!/usr/bin/env bash
# Cleans up all resources created by the Moca E2E test framework.
# Deletes the Kind cluster and removes temporary files.

set -euo pipefail

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
source "${SCRIPT_DIR}/lib.sh"

log_info "=== Cleaning up Moca E2E test environment ==="

# Delete Kind cluster
if kind get clusters 2>/dev/null | grep -q "^${KIND_CLUSTER_NAME}$"; then
    log_info "Deleting Kind cluster '${KIND_CLUSTER_NAME}'..."
    kind delete cluster --name "${KIND_CLUSTER_NAME}"
    log_success "Kind cluster deleted"
else
    log_info "Kind cluster '${KIND_CLUSTER_NAME}' does not exist, skipping"
fi

# Clean up temporary files
log_info "Cleaning up temporary files..."
rm -rf /tmp/moca-e2e-init
rm -rf /tmp/moca-e2e-logs
log_success "Temporary files removed"

log_success "=== Moca E2E cleanup complete ==="
