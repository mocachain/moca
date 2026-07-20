#!/usr/bin/env bash
# Comprehensive upgrade test orchestrator.
# Sources all modules from tests/modules/*.sh, which register:
#   - setup functions (one-time account creation, etc.)
#   - tx functions (individual atomic transactions)
#   - verify functions (post-upgrade assertions)
#
# The orchestrator shuffles and calls tx functions pre/post upgrade,
# then runs all verify functions.
#
# Usage:
#   OLD_VERSION=v1.2.0 bash tests/test_upgrade_comprehensive.sh
#   UPGRADE_MODE=hardfork OLD_VERSION=main bash tests/test_upgrade_comprehensive.sh
#   TX_ROUNDS=5 bash tests/test_upgrade_comprehensive.sh  # 5 rounds of all txs
#
# Adding a new module:
#   1. Create tests/modules/mod_<name>.sh
#   2. Register: register_setup, register_tx, register_verify
#   3. The orchestrator auto-discovers and calls them
# shellcheck source=/dev/null
source "$(dirname "$0")/../framework/framework.sh"
fw_init

OLD_VERSION="${OLD_VERSION:-v1.3.0}"
UPGRADE_NAME="${UPGRADE_NAME:-v2.0.0}"
UPGRADE_MODE="${UPGRADE_MODE:-governance}"
TX_ROUNDS="${TX_ROUNDS:-3}"
RELEASE_TAG="${RELEASE_TAG:-}"
RELEASE_IMAGE="${RELEASE_IMAGE:-}"
EVM_RPC="http://localhost:8545"
EVM_CHAIN_ID="${SRC_CHAIN_ID}"
VAL0_PRIVKEY="0x${VALIDATOR0_PRIKEY}"
# shellcheck disable=SC2034  # CONTRACTS_DIR is consumed by sourced modules (e.g. mod_evm.sh)
CONTRACTS_DIR="$(cd "$(dirname "$0")/../contracts" && pwd)"

# ── Module registry ───────────────────────────────────────────────────────────
_SETUP_FNS=()
_TX_FNS=()          # array of tx function names
_VERIFY_FNS=()      # array of "description|function_name" pairs

register_setup()  { _SETUP_FNS+=("$1"); }
register_tx()     { _TX_FNS+=("$1"); }
register_verify() { _VERIFY_FNS+=("$1|$2"); }

# ── Shared helpers (available to all modules) ─────────────────────────────────

exec_on_validator() {
    local idx="$1"; shift
    kubectl exec -n "${K8S_NAMESPACE}" "validator-${idx}-0" -c mocad -- \
        mocad "$@" --home /root/.mocad 2>/dev/null
}

write_to_pod() {
    echo "$1" | kubectl exec -i -n "${K8S_NAMESPACE}" validator-0-0 -c mocad -- \
        bash -c "cat > $2" 2>/dev/null
}

# Broadcast a Cosmos tx via sync mode and wait for inclusion. Returns 0 on
# successful on-chain execution, 1 on broadcast or execution failure.
#
# We invoke kubectl directly (not through exec_mocad) so that mocad's stderr
# (which carries error messages on broadcast failure) flows through to our
# 2>&1 capture instead of being silenced inside the helper.
cosmos_tx() {
    local out hash
    out=$(cosmos_broadcast validator-0-0 tx "$@")
    if ! printf '%s' "$out" | jq -e . >/dev/null 2>&1; then
        log_error "  cosmos_tx broadcast failed: $out"
        return 1
    fi
    if [ "$(echo "$out" | jq -r '.code // 0')" != "0" ]; then
        log_error "  cosmos_tx CheckTx rejected: $out"
        return 1
    fi
    hash=$(echo "$out" | jq -r '.txhash // empty' 2>/dev/null)
    if [ -z "$hash" ]; then
        log_error "  cosmos_tx returned no txhash: $out"
        return 1
    fi
    fw_wait_cosmos_tx "$hash"
}

cosmos_tx_on() {
    local idx="$1"; shift
    local out hash
    out=$(cosmos_broadcast "validator-${idx}-0" tx "$@")
    if ! printf '%s' "$out" | jq -e . >/dev/null 2>&1; then
        log_error "  cosmos_tx_on broadcast failed: $out"
        return 1
    fi
    if [ "$(echo "$out" | jq -r '.code // 0')" != "0" ]; then
        log_error "  cosmos_tx_on CheckTx rejected: $out"
        return 1
    fi
    hash=$(echo "$out" | jq -r '.txhash // empty' 2>/dev/null)
    if [ -z "$hash" ]; then
        log_error "  cosmos_tx_on returned no txhash: $out"
        return 1
    fi
    fw_wait_cosmos_tx "$hash"
}

# Broadcast an EVM tx and wait for receipt. cast send is synchronous by
# default; we capture the txhash so failures surface (instead of being
# swallowed) and so callers can verify status explicitly if needed.
evm_transfer() {
    local out hash
    out=$(cast send "$1" --value "$2" \
        --private-key "$VAL0_PRIVKEY" --rpc-url "$EVM_RPC" \
        --chain-id "$EVM_CHAIN_ID" --gas-price 30000000000 --json 2>&1) || {
        log_error "  evm_transfer broadcast failed: $out"
        return 1
    }
    hash=$(echo "$out" | jq -r '.transactionHash // empty' 2>/dev/null)
    [ -z "$hash" ] && { log_error "  evm_transfer returned no hash: $out"; return 1; }
    fw_wait_evm_tx "$hash" 30 "$EVM_RPC" || { log_error "  evm_transfer wait timeout: $hash ($out)"; return 1; }
}

evm_send() {
    local out hash
    out=$(cast send "$@" --private-key "$VAL0_PRIVKEY" --rpc-url "$EVM_RPC" \
        --chain-id "$EVM_CHAIN_ID" --gas-price 30000000000 --json 2>&1) || {
        log_error "  evm_send broadcast failed: $out"
        return 1
    }
    hash=$(echo "$out" | jq -r '.transactionHash // empty' 2>/dev/null)
    [ -z "$hash" ] && { log_error "  evm_send returned no hash: $out"; return 1; }
    fw_wait_evm_tx "$hash" 30 "$EVM_RPC" || { log_error "  evm_send wait timeout: $hash ($out)"; return 1; }
}

evm_call() {
    cast call "$@" --rpc-url "$EVM_RPC" 2>/dev/null
}

evm_deploy() {
    local bytecode="$1"
    local output hash
    output=$(cast send --private-key "$VAL0_PRIVKEY" --rpc-url "$EVM_RPC" \
        --chain-id "$EVM_CHAIN_ID" --gas-price 30000000000 --json --create "$bytecode" 2>&1) || {
        log_error "  evm_deploy broadcast failed: $output"
        return 1
    }
    hash=$(echo "$output" | jq -r '.transactionHash // empty' 2>/dev/null)
    [ -z "$hash" ] && { log_error "  evm_deploy returned no hash: $output"; return 1; }
    fw_wait_evm_tx "$hash" 30 "$EVM_RPC" || { log_error "  evm_deploy wait timeout: $hash ($output)"; return 1; }
    local rcpt; rcpt=$(cast receipt "$hash" --rpc-url "$EVM_RPC" --json 2>&1) || { log_error "  evm_deploy receipt failed: $rcpt"; return 1; }
    echo "$rcpt" | jq -r '.contractAddress // empty' 2>/dev/null
}

# Shuffle an array (Fisher-Yates). Usage: shuffle_array array_name
shuffle_array() {
    local -n _arr=$1
    local i j tmp n=${#_arr[@]}
    for ((i = n - 1; i > 0; i--)); do
        j=$((RANDOM % (i + 1)))
        tmp="${_arr[$i]}"
        _arr[$i]="${_arr[$j]}"
        _arr[$j]="$tmp"
    done
}

# ── Load modules ──────────────────────────────────────────────────────────────
MODULES_DIR="$(dirname "$0")/modules"
if [ -d "$MODULES_DIR" ]; then
    for mod in "${MODULES_DIR}"/mod_*.sh; do
        [ -f "$mod" ] || continue
        log_info "Loading module: $(basename "$mod")"
        # shellcheck source=/dev/null
        source "$mod"
    done
fi

if [ ${#_TX_FNS[@]} -eq 0 ] && [ ${#_VERIFY_FNS[@]} -eq 0 ]; then
    log_error "No modules loaded — add module files to tests/modules/"
    exit 1
fi

log_info "Loaded: ${#_SETUP_FNS[@]} setup, ${#_TX_FNS[@]} tx, ${#_VERIFY_FNS[@]} verify functions"

# ══════════════════════════════════════════════════════════════════════════════
# SETUP
# ══════════════════════════════════════════════════════════════════════════════
fw_start_chain_from_version "$OLD_VERSION"

# If a release image is specified, pull and load it
if [ -n "$RELEASE_TAG" ] && [ -z "$RELEASE_IMAGE" ]; then
    ARCH=$(uname -m)
    case "$ARCH" in aarch64|arm64) _ARCH_TAG="arm64" ;; *) _ARCH_TAG="amd64" ;; esac
    _GHCR_IMAGE="ghcr.io/mocachain/mocad:${RELEASE_TAG}-${_ARCH_TAG}"
    RELEASE_IMAGE="mocachain/moca:${RELEASE_TAG}"
    log_info "Pulling release image: ${_GHCR_IMAGE}..."
    docker pull "$_GHCR_IMAGE" 2>&1
    echo "FROM ${_GHCR_IMAGE}" | docker build -t "$RELEASE_IMAGE" - 2>&1
    kind_load_image "$RELEASE_IMAGE"
fi

# Get validator operator addresses indexed by validator-N pod number.
# We pull each from its own keyring instead of indexing the staking-validators
# API response, because the API order is not guaranteed to match the pod
# naming, and an off-by-one between VAL_OPERS[i] and validator-i pod produces
# code=19 ("no delegation for (address, validator) tuple") on tx broadcast.
# In Moca, the validator operator address equals the account address.
VAL_OPERS=()
for ((i = 0; i < NUM_VALIDATORS; i++)); do
    VAL_OPERS+=("$(exec_on_validator "$i" keys show "validator${i}" -a --keyring-backend test)")
done

# Run module setup functions
for fn in "${_SETUP_FNS[@]}"; do
    log_info "Setup: ${fn}"
    "$fn"
done

# ══════════════════════════════════════════════════════════════════════════════
# PRE-UPGRADE: randomized tx rounds
# ══════════════════════════════════════════════════════════════════════════════
PRE_HEIGHT=$(get_block_height "http://localhost:26657")
log_info "=== Pre-upgrade txs: ${TX_ROUNDS} rounds x ${#_TX_FNS[@]} tx types (height=${PRE_HEIGHT}) ==="

for ((round = 1; round <= TX_ROUNDS; round++)); do
    local_fns=("${_TX_FNS[@]}")
    shuffle_array local_fns
    log_info "  Round ${round}/${TX_ROUNDS} (shuffled)"
    for fn in "${local_fns[@]}"; do
        "$fn"
    done
done

# ══════════════════════════════════════════════════════════════════════════════
# UPGRADE
# ══════════════════════════════════════════════════════════════════════════════
UPGRADE_ARGS=(--name "$UPGRADE_NAME" --mode "$UPGRADE_MODE")
[ -n "$RELEASE_IMAGE" ] && UPGRADE_ARGS+=(--new-image "$RELEASE_IMAGE")
fw_upgrade_chain "${UPGRADE_ARGS[@]}"

# ══════════════════════════════════════════════════════════════════════════════
# POST-UPGRADE: randomized tx rounds
# ══════════════════════════════════════════════════════════════════════════════
log_info "=== Post-upgrade txs: ${TX_ROUNDS} rounds x ${#_TX_FNS[@]} tx types ==="

for ((round = 1; round <= TX_ROUNDS; round++)); do
    local_fns=("${_TX_FNS[@]}")
    shuffle_array local_fns
    log_info "  Round ${round}/${TX_ROUNDS} (shuffled)"
    for fn in "${local_fns[@]}"; do
        "$fn"
    done
done

# ══════════════════════════════════════════════════════════════════════════════
# VERIFICATION
# ══════════════════════════════════════════════════════════════════════════════

# Built-in orchestrator tests
test_upgrade_applied() {
    local result; result=$(exec_mocad query upgrade applied "$UPGRADE_NAME" \
        --node tcp://localhost:26657 --chain-id "${CHAIN_ID}" \
        --output json 2>/dev/null || echo "{}")
    local height; height=$(echo "$result" | jq -r '.height // empty' 2>/dev/null) || true
    assert_not_empty "$height" "Upgrade '${UPGRADE_NAME}' should be applied"
}

test_height_advanced() {
    local h; h=$(get_block_height "http://localhost:26657")
    assert_gt "$h" "$PRE_HEIGHT" "Height should advance past pre-upgrade"
}

test_chain_stable() {
    local h1; h1=$(get_block_height "http://localhost:26657")
    fw_wait_blocks 10
    local h2; h2=$(get_block_height "http://localhost:26657")
    assert_gt "$((h2 - h1))" "9" "Chain should produce 10+ blocks post-upgrade"
}

fw_run_test "Upgrade handler applied"   test_upgrade_applied
fw_run_test "Height advanced"           test_height_advanced
fw_run_test "Chain stable (10 blocks)"  test_chain_stable

# Module-registered verify functions
for entry in "${_VERIFY_FNS[@]}"; do
    desc="${entry%%|*}"
    func="${entry##*|}"
    fw_run_test "$desc" "$func"
done

fw_done
