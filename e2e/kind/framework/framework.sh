#!/usr/bin/env bash
# E2E Kind Test Framework — high-level API for self-contained test files.
#
# Usage in a test file:
#   source "$(dirname "$0")/../framework/framework.sh"
#   fw_init
#   fw_start_chain
#   fw_run_test "name" test_func
#   fw_done

set -euo pipefail

FW_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
E2E_DIR=$(cd -- "${FW_DIR}/.." && pwd)
SCRIPTS_DIR="${E2E_DIR}/scripts"

# Source shared helpers
source "${SCRIPTS_DIR}/lib.sh"

# ── State ─────────────────────────────────────────────────────────────────────
_FW_PASS=0
_FW_FAIL=0
_FW_STARTED=false
_FW_DONE=false
_FW_GENESIS_PATCHES=()
_FW_APPTOML_PATCHES=()

# ── Lifecycle ─────────────────────────────────────────────────────────────────

fw_init() {
    log_info "=== Framework initialized ==="
    trap '_fw_cleanup_on_exit' EXIT
}

fw_done() {
    _FW_DONE=true
    echo ""
    log_info "=== Test Summary ==="
    log_info "  Passed: ${_FW_PASS}"
    log_info "  Failed: ${_FW_FAIL}"

    if [ "${_FW_FAIL}" -gt 0 ]; then
        log_error "Some tests failed"
        _fw_collect_debug_logs
    fi

    _fw_cleanup

    if [ "${_FW_FAIL}" -gt 0 ]; then
        exit 1
    fi
    log_success "All tests passed"
}

_fw_cleanup_on_exit() {
    local exit_code=$?
    if [ "${_FW_DONE}" = "true" ]; then
        return
    fi
    if [ $exit_code -ne 0 ]; then
        log_error "Test exited early (code ${exit_code})"
        _fw_collect_debug_logs
    fi
    _fw_cleanup
    exit $exit_code
}

_fw_cleanup() {
    if [ "${FW_SKIP_CLEANUP:-false}" = "true" ]; then
        log_warn "FW_SKIP_CLEANUP=true, leaving cluster running"
        return
    fi
    if [ "${_FW_STARTED}" = "true" ]; then
        log_info "Cleaning up..."
        bash "${SCRIPTS_DIR}/cleanup.sh" 2>/dev/null || true
    fi
}

_fw_collect_debug_logs() {
    local log_dir="/tmp/moca-e2e-logs"
    log_info "Collecting debug logs to ${log_dir}..."
    mkdir -p "${log_dir}"
    for ((i = 0; i < ${NUM_VALIDATORS:-4}; i++)); do
        kubectl logs -n "${K8S_NAMESPACE}" "validator-${i}-0" -c mocad --tail=100 \
            > "${log_dir}/validator-${i}.log" 2>/dev/null || true
        kubectl describe pod -n "${K8S_NAMESPACE}" "validator-${i}-0" \
            > "${log_dir}/validator-${i}-describe.txt" 2>/dev/null || true
    done
    log_info "Logs collected at ${log_dir}"
}

# ── Config overrides (call before fw_start_chain) ─────────────────────────────

fw_config() {
    local key="$1" val="$2"
    export "$key"="$val"
}

fw_genesis_patch() {
    _FW_GENESIS_PATCHES+=("$1")
}

fw_apptoml_patch() {
    _FW_APPTOML_PATCHES+=("$1")
}

# ── Chain setup ───────────────────────────────────────────────────────────────

fw_start_chain() {
    local version="" validators="" image=""
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --version) version="$2"; shift 2 ;;
            --validators) validators="$2"; shift 2 ;;
            --image) image="$2"; shift 2 ;;
            *) shift ;;
        esac
    done

    [ -n "$validators" ] && fw_config NUM_VALIDATORS "$validators"

    _FW_STARTED=true

    # Setup Kind cluster
    bash "${SCRIPTS_DIR}/setup-kind.sh"

    # Build images
    if [ -n "$version" ]; then
        OLD_VERSION="$version" bash "${SCRIPTS_DIR}/build-images.sh"
    else
        bash "${SCRIPTS_DIR}/build-images.sh"
    fi

    # Deploy chain
    local deploy_image="${image:-${DOCKER_IMAGE}:${DOCKER_TAG}}"
    DEPLOY_IMAGE="${deploy_image}" bash "${SCRIPTS_DIR}/deploy.sh"

    # Wait for chain
    wait_for_chain_ready "http://localhost:26657" 120
}

# Deploy chain with an old version (for upgrade tests).
fw_start_chain_from_version() {
    local old_version="$1"
    log_info "=== Starting chain from version ${old_version} ==="

    _FW_STARTED=true

    # Setup Kind cluster
    bash "${SCRIPTS_DIR}/setup-kind.sh"

    # Build both images
    OLD_VERSION="${old_version}" bash "${SCRIPTS_DIR}/build-images.sh"

    # Deploy with old version
    DEPLOY_IMAGE="${DOCKER_IMAGE}:${old_version}" bash "${SCRIPTS_DIR}/deploy.sh"

    # Wait for chain
    wait_for_chain_ready "http://localhost:26657" 120

    # Verify version
    local running_version
    running_version=$(exec_mocad version 2>/dev/null || echo "unknown")
    log_success "Chain running on old version: ${running_version}"
}

# ── Upgrade ───────────────────────────────────────────────────────────────────

fw_upgrade_chain() {
    local name="" mode="" height="" new_image=""
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --name) name="$2"; shift 2 ;;
            --mode) mode="$2"; shift 2 ;;
            --height) height="$2"; shift 2 ;;
            --new-image) new_image="$2"; shift 2 ;;
            *) shift ;;
        esac
    done

    : "${name:?--name is required}"
    : "${mode:?--mode is required}"

    # Default new image is the current build
    new_image="${new_image:-${DOCKER_IMAGE}:${DOCKER_TAG}}"

    # Auto-compute upgrade height if not specified
    if [ -z "$height" ]; then
        local current
        current=$(get_block_height "http://localhost:26657")
        # Give enough time for governance voting period (15s) + buffer
        if [ "$mode" = "governance" ]; then
            height=$((current + 40))
        else
            height=$((current + 20))
        fi
    fi

    log_info "=== Upgrading chain ==="
    log_info "  Name:   ${name}"
    log_info "  Mode:   ${mode}"
    log_info "  Height: ${height}"
    log_info "  Image:  ${new_image}"

    UPGRADE_NAME="${name}" \
    UPGRADE_HEIGHT="${height}" \
    UPGRADE_MODE="${mode}" \
    NEW_DOCKER_IMAGE="${new_image}" \
        bash "${SCRIPTS_DIR}/upgrade-chain.sh"
}

# ── Test execution ────────────────────────────────────────────────────────────

fw_run_test() {
    local name="$1"
    local func="$2"

    echo -n "  [TEST] ${name}... "
    if $func; then
        log_success "PASS"
        _FW_PASS=$((_FW_PASS + 1))
    else
        log_error "FAIL"
        _FW_FAIL=$((_FW_FAIL + 1))
    fi
}

# ── Utility helpers ───────────────────────────────────────────────────────────

fw_tx_send() {
    local from="$1" to="$2" amount="$3"
    local fees="${4:-200000000000000amoca}"

    exec_mocad tx bank send "$from" "$to" "$amount" \
        --from "$from" \
        --keyring-backend test \
        --chain-id "${CHAIN_ID}" \
        --node tcp://localhost:26657 \
        --fees "$fees" \
        -y > /dev/null 2>&1
    sleep 5
}

fw_wait_blocks() {
    local n="$1"
    local current
    current=$(get_block_height "http://localhost:26657")
    local target=$((current + n))
    wait_for_height "$target" "http://localhost:26657"
}
