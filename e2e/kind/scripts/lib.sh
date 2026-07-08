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
# Errors and warnings must go to stderr — otherwise they get captured by
# command substitution (e.g. `addr=$(some_func)` swallows the error message
# into `addr`, hiding it from the test log AND polluting downstream values).
log_info()    { echo -e "\033[0;34m[INFO]\033[0m $*"; }
log_success() { echo -e "\033[0;32m[PASS]\033[0m $*"; }
log_error()   { echo -e "\033[0;31m[FAIL]\033[0m $*" >&2; }
log_warn()    { echo -e "\033[0;33m[WARN]\033[0m $*" >&2; }

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

# Wait until every validator pod is actually serving its own RPC. Goes beyond
# wait_for_chain_ready (which only checks the host's NodePort RPC, so a single
# healthy pod can satisfy it) by polling each validator's internal RPC. Critical
# after a rolling restart where pods come back at staggered times.
#
# Usage: wait_for_all_validator_rpcs <num> [timeout_seconds=120]
wait_for_all_validator_rpcs() {
    local num="$1"
    local timeout="${2:-120}"
    local deadline=$(( $(date +%s) + timeout ))
    local i height now
    log_info "Waiting for all ${num} validator RPCs to serve /status..."
    for ((i = 0; i < num; i++)); do
        while :; do
            height=$(kubectl exec -n "${K8S_NAMESPACE}" "validator-${i}-0" -c mocad -- \
                curl -sS -m 5 http://localhost:26657/status 2>/dev/null \
                | jq -r '.result.sync_info.latest_block_height // empty' 2>/dev/null) || true
            if [ -n "$height" ] && [ "$height" -gt 0 ] 2>/dev/null; then
                log_success "  validator-${i} RPC serving (height=${height})"
                break
            fi
            now=$(date +%s)
            if [ "$now" -ge "$deadline" ]; then
                log_error "  validator-${i} RPC not serving after timeout"
                return 1
            fi
            sleep 2
        done
    done
}

# wait_for_evm_rpc_ready: poll the EVM JSON-RPC at the host's port-forward
# until eth_blockNumber returns a non-zero block. The cosmos /status endpoint
# can be live before the EVM RPC is fully attached, leading to "server returned
# a null response" errors on the first `cast send` after start.
#
# Usage: wait_for_evm_rpc_ready [rpc=http://localhost:8545] [timeout=60]
wait_for_evm_rpc_ready() {
    local rpc="${1:-http://localhost:8545}"
    local timeout="${2:-60}"
    local deadline=$(( $(date +%s) + timeout ))
    local block now
    log_info "Waiting for EVM RPC ${rpc} to serve eth_blockNumber..."
    while :; do
        block=$(curl -sS -m 5 -X POST -H "Content-Type: application/json" \
            -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
            "$rpc" 2>/dev/null | jq -r '.result // empty' 2>/dev/null) || true
        if [ -n "$block" ] && [ "$block" != "0x0" ] && [ "$block" != "null" ]; then
            log_success "EVM RPC ready (block=${block})"
            return 0
        fi
        now=$(date +%s)
        if [ "$now" -ge "$deadline" ]; then
            log_error "EVM RPC ${rpc} not ready after ${timeout}s"
            return 1
        fi
        sleep 2
    done
}

# kind_load_image: load a local docker image into the Kind cluster's containerd
# snapshotter. Uses `docker save | docker exec ctr import` instead of the
# default `kind load docker-image` because the latter can silently fail with
# the desktop-linux buildx driver — the image manifest gets exported but the
# image doesn't actually land in the cluster's snapshotter under the expected
# SHA, leading to ErrImageNeverPull at pod start.
#
# Usage: kind_load_image <image:tag>
kind_load_image() {
    local image="$1"
    local node="${KIND_CLUSTER_NAME}-control-plane"
    if ! docker exec "$node" true 2>/dev/null; then
        log_error "Kind control-plane container '$node' not running"
        return 1
    fi
    docker save "$image" | docker exec -i "$node" \
        ctr --namespace=k8s.io images import - >/dev/null 2>&1 || {
        log_error "Failed to load image $image into Kind"
        return 1
    }
    log_success "Image $image loaded into Kind cluster"
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

# ── Cosmos tx broadcast (--gas auto + retry) ──────────────────────────────────
# Gas/fee policy for cosmos txs: simulate-derived gas (`--gas auto`) with the fee
# auto-derived from the node's minimum-gas-prices — never fixed --gas/--fees, and NOT
# --gas-prices: moca's `--gas auto` already derives the fee internally, so pairing it
# with --gas-prices errors "cannot provide both fees and gas prices". `--gas auto` adds
# a simulate round-trip that widens the window in which validator0's account sequence —
# shared across the suite's cosmos AND EVM txs, because cosmos/evm maps the EVM nonce
# onto the cosmos account sequence — can be read stale, so we retry on the mismatch.
# (The node's minimum-gas-prices must clear the post-v2 feemarket floor — init-chain.sh.)
# 1.5, not 1.3: post-v2 store-write (WriteFlat) gas is ~30% above the simulate estimate,
# so 1.3 leaves some txs (e.g. distribution withdraw) a few hundred gas short -> out of gas.
: "${COSMOS_GAS_ADJUSTMENT:=1.5}"
: "${COSMOS_TX_RETRIES:=6}"
: "${COSMOS_TX_RETRY_SLEEP:=1.5}"

# Broadcast a cosmos tx (sync mode) with the gas-auto policy, retrying on
# `account sequence mismatch`. Common flags (home/keyring/chain-id/node/gas/
# broadcast/output) are added here; callers pass the pod then the mocad args,
# e.g.  cosmos_broadcast validator-0-0 tx bank send "$a" "$b" "$amt" --from "$a"
# Echoes the final broadcast JSON (or mocad's error text on hard failure).
# shellcheck disable=SC2153  # CHAIN_ID is set by callers (workflow env / sourced constants)
cosmos_broadcast() {
    local pod="$1"; shift
    local attempt out
    for ((attempt = 1; attempt <= COSMOS_TX_RETRIES; attempt++)); do
        out=$(kubectl exec -n "${K8S_NAMESPACE}" "$pod" -c mocad -- \
            mocad "$@" \
            --home /root/.mocad \
            --keyring-backend test \
            --chain-id "${CHAIN_ID}" \
            --node tcp://localhost:26657 \
            --gas auto --gas-adjustment "${COSMOS_GAS_ADJUSTMENT}" \
            --broadcast-mode sync -y --output json 2>&1) || true
        if printf '%s' "$out" | grep -qiE "account sequence mismatch" \
           && [ "$attempt" -lt "$COSMOS_TX_RETRIES" ]; then
            sleep "${COSMOS_TX_RETRY_SLEEP}"
            continue
        fi
        break
    done
    # `--gas auto` prints "gas estimate: N" to stderr (captured via 2>&1). Return just the
    # broadcast JSON line when present so callers get clean JSON; otherwise the raw text
    # (hard-error path, e.g. a build/simulate failure with no JSON — callers surface it).
    local json
    json=$(printf '%s\n' "$out" | grep -E '^\{.*\}$' | tail -1)
    if [ -n "$json" ]; then printf '%s' "$json"; else printf '%s' "$out"; fi
}

# Poll a Cosmos tx hash until it's included in a block (or timeout).
# Returns 0 if the tx was included with code=0, 1 if it failed on chain
# or wasn't included within the retry budget.
#
# Usage: fw_wait_cosmos_tx <txhash> [retries=10]
# shellcheck disable=SC2153  # CHAIN_ID is set by callers (workflow env / sourced constants)
fw_wait_cosmos_tx() {
    local hash="$1"
    local retries="${2:-10}"
    local i out code raw
    for ((i = 0; i < retries; i++)); do
        out=$(exec_mocad query tx "$hash" \
            --node tcp://localhost:26657 --chain-id "${CHAIN_ID}" \
            --output json 2>/dev/null) || true
        code=$(echo "$out" | jq -r '.code // empty' 2>/dev/null)
        if [ -n "$code" ]; then
            if [ "$code" = "0" ]; then return 0; fi
            raw=$(echo "$out" | jq -r '.raw_log // empty' 2>/dev/null)
            log_error "  cosmos tx $hash failed on-chain: code=$code log=$raw"
            return 1
        fi
        sleep 1
    done
    log_error "  cosmos tx $hash not included after ${retries}s"
    return 1
}

# Poll an EVM tx hash until it's mined (or timeout).
# Returns 0 if the receipt status == 0x1, 1 if reverted or not mined.
#
# Usage: fw_wait_evm_tx <txhash> [retries=10] [rpc=http://localhost:8545]
fw_wait_evm_tx() {
    local hash="$1"
    local retries="${2:-10}"
    local rpc="${3:-http://localhost:8545}"
    local i receipt status
    for ((i = 0; i < retries; i++)); do
        receipt=$(cast receipt "$hash" --rpc-url "$rpc" --json 2>/dev/null) || true
        status=$(echo "$receipt" | jq -r '.status // empty' 2>/dev/null)
        if [ -n "$status" ]; then
            if [ "$status" = "0x1" ]; then return 0; fi
            log_error "  evm tx $hash reverted: status=$status"
            return 1
        fi
        sleep 1
    done
    log_error "  evm tx $hash not mined after ${retries}s"
    return 1
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
    catching_up=$(echo "$status" | jq -r '.result.sync_info.catching_up')
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
