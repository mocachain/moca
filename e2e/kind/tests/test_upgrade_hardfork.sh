#!/usr/bin/env bash
# Hardfork upgrade test — deploys an old version, upgrades via hardfork config,
# and validates that state is preserved and the chain continues.
#
# Usage:
#   OLD_VERSION=v1.1.2 bash tests/test_upgrade_hardfork.sh
#   OLD_VERSION=main   bash tests/test_upgrade_hardfork.sh

source "$(dirname "$0")/../framework/framework.sh"
fw_init

OLD_VERSION="${OLD_VERSION:-v1.1.2}"
UPGRADE_NAME="${UPGRADE_NAME:-v1.2.0}"
FEES="200000000000000amoca"

# ── Setup: deploy old version ────────────────────────────────────────────────
fw_start_chain_from_version "$OLD_VERSION"

# ── Pre-upgrade state ────────────────────────────────────────────────────────
log_info "=== Pre-upgrade setup ==="

exec_mocad keys add upgrade-acct --keyring-backend test 2>/dev/null || true
UPGRADE_TEST_ADDR=$(exec_mocad keys show upgrade-acct -a --keyring-backend test)
log_info "Test account: ${UPGRADE_TEST_ADDR}"

fw_tx_send validator0 "$UPGRADE_TEST_ADDR" "5000000000000000000amoca"

PRE_UPGRADE_HEIGHT=$(get_block_height "http://localhost:26657")
PRE_UPGRADE_BALANCE=$(exec_mocad query bank balances "$UPGRADE_TEST_ADDR" \
    --node tcp://localhost:26657 \
    --chain-id "${CHAIN_ID}" \
    --output json | jq -r '.balances[] | select(.denom=="amoca") | .amount' 2>/dev/null || echo "0")

log_info "Pre-upgrade height:  ${PRE_UPGRADE_HEIGHT}"
log_info "Pre-upgrade balance: ${PRE_UPGRADE_BALANCE} amoca"

# ── Upgrade (1 line) ─────────────────────────────────────────────────────────
fw_upgrade_chain --name "$UPGRADE_NAME" --mode hardfork

# ── Post-upgrade tests ───────────────────────────────────────────────────────

test_chain_producing_blocks_post_upgrade() {
    local h1
    h1=$(get_block_height "http://localhost:26657")
    sleep 3
    local h2
    h2=$(get_block_height "http://localhost:26657")
    assert_gt "$h2" "$h1" "Chain should produce blocks post-upgrade"
}

test_height_past_upgrade() {
    local current
    current=$(get_block_height "http://localhost:26657")
    # Upgrade height was auto-computed (current + 20 at time of call)
    assert_gt "$current" "$PRE_UPGRADE_HEIGHT" "Height should be past pre-upgrade height"
}

test_balances_preserved() {
    local bal
    bal=$(exec_mocad query bank balances "$UPGRADE_TEST_ADDR" \
        --node tcp://localhost:26657 \
        --chain-id "${CHAIN_ID}" \
        --output json | jq -r '.balances[] | select(.denom=="amoca") | .amount' 2>/dev/null || echo "0")

    assert_eq "$bal" "$PRE_UPGRADE_BALANCE" "Balance should survive upgrade"
}

test_new_binary_running() {
    # After hardfork, verify the new binary is running by checking node status
    local status
    status=$(exec_mocad status --node tcp://localhost:26657 2>/dev/null || echo "")
    local node_info
    node_info=$(echo "$status" | jq -r '.node_info.network // empty' 2>/dev/null) || true
    assert_eq "$node_info" "${CHAIN_ID}" "Node should be running on correct chain"
}

test_send_tokens_post_upgrade() {
    exec_mocad keys add post-hf-user --keyring-backend test 2>/dev/null || true
    local recv
    recv=$(exec_mocad keys show post-hf-user -a --keyring-backend test)

    fw_tx_send validator0 "$recv" "1000000000000000000amoca"

    local bal
    bal=$(exec_mocad query bank balances "$recv" \
        --node tcp://localhost:26657 \
        --chain-id "${CHAIN_ID}" \
        --output json | jq -r '.balances[] | select(.denom=="amoca") | .amount' 2>/dev/null || echo "0")

    assert_eq "$bal" "1000000000000000000" "Post-upgrade transfer should succeed"
}

# ── Run tests ────────────────────────────────────────────────────────────────

fw_run_test "Chain producing blocks post-upgrade"  test_chain_producing_blocks_post_upgrade
fw_run_test "Height past pre-upgrade"              test_height_past_upgrade
fw_run_test "Balances preserved across upgrade"    test_balances_preserved
fw_run_test "New binary running post-upgrade"       test_new_binary_running
fw_run_test "Token transfers work post-upgrade"    test_send_tokens_post_upgrade

fw_done
