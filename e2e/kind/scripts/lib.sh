#!/usr/bin/env bash
# Shared helper functions for e2e Kind tests.

set -euo pipefail

# ── Source config ─────────────────────────────────────────────────────────────
SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
E2E_DIR=$(cd -- "${SCRIPT_DIR}/.." && pwd)

if [ -f "${E2E_DIR}/e2e.env" ]; then
    # shellcheck disable=SC1091
    source "${E2E_DIR}/e2e.env"
fi

# ── Logging ───────────────────────────────────────────────────────────────────
log_info()    { echo -e "\033[0;34m[INFO]\033[0m $*"; }
log_success() { echo -e "\033[0;32m[PASS]\033[0m $*"; }
log_error()   { echo -e "\033[0;31m[FAIL]\033[0m $*"; }
log_warn()    { echo -e "\033[0;33m[WARN]\033[0m $*"; }

# ── Chain queries ─────────────────────────────────────────────────────────────

# Get the current block height from an RPC endpoint.
get_block_height() {
    local rpc_url="${1:-http://localhost:26657}"
    curl -sf "${rpc_url}/status" 2>/dev/null | jq -r '.result.sync_info.latest_block_height' 2>/dev/null || echo "0"
}

# Wait until the chain is producing blocks at the given RPC endpoint.
wait_for_chain_ready() {
    local rpc_url="${1:-http://localhost:26657}"
    local max_wait="${2:-120}"
    local elapsed=0

    log_info "Waiting for chain to be ready at ${rpc_url}..."
    while [ $elapsed -lt $max_wait ]; do
        local height
        height=$(get_block_height "$rpc_url")
        if [ "$height" -gt 2 ] 2>/dev/null; then
            log_success "Chain is ready at block height ${height}"
            return 0
        fi
        elapsed=$((elapsed + 3))
        sleep 3
    done
    log_error "Chain not ready after ${max_wait}s"
    return 1
}

# Wait until the chain reaches a specific block height.
wait_for_height() {
    local target="$1"
    local rpc_url="${2:-http://localhost:26657}"
    local max_attempts="${3:-120}"
    local attempt=0

    while [ $attempt -lt $max_attempts ]; do
        local current
        current=$(get_block_height "$rpc_url")
        if [ "$current" -ge "$target" ] 2>/dev/null; then
            return 0
        fi
        attempt=$((attempt + 1))
        sleep 2
    done
    log_error "Chain did not reach height ${target} within $((max_attempts * 2))s"
    return 1
}

# Execute mocad command inside validator-0 pod.
exec_mocad() {
    kubectl exec -n "${K8S_NAMESPACE}" validator-0-0 -c mocad -- \
        mocad "$@" --home /root/.mocad 2>/dev/null
}

# Base RPC URL for validator index i (parity with moca-devcontainer check-validators.sh).
# Index 0: NodePort on the host. Index > 0: in-cluster DNS (use with kubectl exec curl from validator-0).
kind_validator_rpc_base() {
    local idx="$1"
    if [ "$idx" -eq 0 ]; then
        echo "http://localhost:26657"
    else
        echo "http://validator-${idx}-0.validator-headless.${K8S_NAMESPACE}.svc.cluster.local:26657"
    fi
}

# Fetch /status JSON for validator idx (host curl for 0; kubectl exec for others).
kind_fetch_rpc_status() {
    local idx="$1"
    local base
    base=$(kind_validator_rpc_base "$idx")
    if [ "$idx" -eq 0 ]; then
        curl -sf "${base}/status" 2>/dev/null || return 1
    else
        kubectl exec -n "${K8S_NAMESPACE}" validator-0-0 -c mocad -- \
            curl -sf "${base}/status" 2>/dev/null || return 1
    fi
}

get_block_height_for_validator_index() {
    local idx="$1"
    kind_fetch_rpc_status "$idx" | jq -r '.result.sync_info.latest_block_height // "0"'
}

kind_validator_pod_is_running() {
    local idx="$1"
    local phase
    phase=$(kubectl get pod -n "${K8S_NAMESPACE}" "validator-${idx}-0" -o jsonpath='{.status.phase}' 2>/dev/null || echo "")
    [ "$phase" = "Running" ]
}

# Same semantics as moca-devcontainer test/validator/check-validators.sh test_validator_production:
# pod running, RPC up, not catching_up, voting_power != 0, then MIN_BLOCKS new blocks within MAX_WAIT.
kind_test_validator_block_production() {
    local index="$1"
    local check_interval="${CHECK_INTERVAL:-5}"
    local max_wait="${MAX_WAIT:-60}"
    local min_blocks="${MIN_BLOCKS:-3}"
    local name="validator-${index}"

    log_info "===== Testing ${name} block production (devcontainer parity) ====="

    if ! kind_validator_pod_is_running "$index"; then
        log_error "Pod ${name}-0 is not Running"
        return 1
    fi
    log_info "Pod ${name}-0 is running"

    local status
    if ! status=$(kind_fetch_rpc_status "$index"); then
        log_error "Cannot access RPC for ${name}"
        return 1
    fi
    log_info "RPC endpoint reachable for ${name}"

    local catching_up initial_height voting_power chain_id
    catching_up=$(echo "$status" | jq -r '.result.sync_info.catching_up // false')
    initial_height=$(echo "$status" | jq -r '.result.sync_info.latest_block_height // "0"')
    voting_power=$(echo "$status" | jq -r '.result.validator_info.voting_power // "0"')
    chain_id=$(echo "$status" | jq -r '.result.node_info.network // "unknown"')

    log_info "Chain ID: ${chain_id}"
    log_info "Initial Height: ${initial_height}"
    log_info "Voting Power: ${voting_power}"
    log_info "Catching Up: ${catching_up}"

    if [ "$catching_up" = "true" ]; then
        log_warn "${name} is still syncing, skipping block production check"
        return 2
    fi

    if [ "$voting_power" = "0" ]; then
        log_warn "${name} has no voting power"
        return 3
    fi

    log_info "Monitoring block production (every ${check_interval}s, max ${max_wait}s, need ${min_blocks} blocks)..."

    local elapsed=0
    local blocks_produced=0
    local last_height=$initial_height

    while [ "$elapsed" -lt "$max_wait" ]; do
        sleep "$check_interval"
        elapsed=$((elapsed + check_interval))

        local current_height
        current_height=$(get_block_height_for_validator_index "$index")
        if [ -z "$current_height" ] || [ "$current_height" = "0" ]; then
            log_warn "Failed to get current height, retrying..."
            continue
        fi

        if [ "$current_height" -gt "$last_height" ]; then
            blocks_produced=$((blocks_produced + current_height - last_height))
            log_info "Height: ${last_height} -> ${current_height} (blocks +${blocks_produced} total)"
            last_height=$current_height
            if [ "$blocks_produced" -ge "$min_blocks" ]; then
                log_success "${name} producing blocks (${blocks_produced} new in ${elapsed}s)"
                return 0
            fi
        else
            log_warn "Height unchanged: ${current_height} (elapsed: ${elapsed}s)"
        fi
    done

    if [ "$blocks_produced" -lt "$min_blocks" ]; then
        log_error "${name} only produced ${blocks_produced} new blocks in ${max_wait}s (need ${min_blocks})"
        return 1
    fi
    return 0
}

# ── EVM / HTTP helpers (parity with moca-devcontainer test/validator/RPC/rpc.sh) ─

# Default CometBFT RPC for curl helpers (override with COMETBFT_RPC_URL).
COMETBFT_RPC_URL="${COMETBFT_RPC_URL:-http://localhost:26657}"

# Return HTTP status code for a GET request (or 000 on failure).
check_http_status() {
    local url="$1"
    curl -s -o /dev/null -w '%{http_code}' --connect-timeout 5 --max-time 15 "$url" 2>/dev/null || echo "000"
}

# Convert 0x-prefixed hex string to decimal (uses python3 for large integers).
hex_to_decimal() {
    local h="$1"
    h="${h#0x}"
    [ -z "$h" ] && echo "0" && return
    python3 -c "print(int('${h}', 16))" 2>/dev/null || echo "0"
}

# POST JSON-RPC to EVM HTTP endpoint; prints full response body.
evm_rpc_call() {
    local method="$1"
    local params="${2:-[]}"
    local base="${EVM_RPC_URL:-${EVM_RPC:-http://localhost:8545}}"
    curl -sf -X POST "${base}" \
        -H "Content-Type: application/json" \
        -d "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"${method}\",\"params\":${params}}" \
        --connect-timeout 5 --max-time 30 2>/dev/null
}

get_evm_block_number() {
    local resp hex
    resp=$(evm_rpc_call "eth_blockNumber" "[]") || return 1
    hex=$(echo "$resp" | jq -r '.result // empty' 2>/dev/null)
    [ -z "$hex" ] && return 1
    hex_to_decimal "$hex"
}

# Latest block timestamp (seconds) from eth_getBlockByNumber("latest", false).
get_evm_block_timestamp() {
    local resp ts_hex
    resp=$(evm_rpc_call "eth_getBlockByNumber" '["latest", false]') || return 1
    ts_hex=$(echo "$resp" | jq -r '.result.timestamp // empty' 2>/dev/null)
    [ -z "$ts_hex" ] && return 1
    hex_to_decimal "$ts_hex"
}

# ── Assertions ────────────────────────────────────────────────────────────────

assert_eq() {
    local actual="$1" expected="$2" msg="${3:-}"
    if [ "$actual" = "$expected" ]; then
        return 0
    fi
    log_error "Assertion failed: ${msg}"
    log_error "  expected: ${expected}"
    log_error "  actual:   ${actual}"
    return 1
}

assert_gt() {
    local actual="$1" threshold="$2" msg="${3:-}"
    if echo "${actual} > ${threshold}" | bc -l | grep -q '^1$'; then
        return 0
    fi
    log_error "Assertion failed: ${msg}"
    log_error "  expected > ${threshold}, got ${actual}"
    return 1
}

assert_not_empty() {
    local val="$1" msg="${2:-}"
    if [ -n "$val" ]; then
        return 0
    fi
    log_error "Assertion failed: ${msg}"
    log_error "  value is empty"
    return 1
}
