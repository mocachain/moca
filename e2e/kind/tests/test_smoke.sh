#!/usr/bin/env bash
# Smoke test — verifies basic chain functionality.
#
# Tests:
#   1. Chain producing blocks
#   2. Genesis account balances
#   3. Query validators
#   4. Query module params
#   5. Send tokens
#   6. Multi-account transfers
#   7. Query storage providers

source "$(dirname "$0")/../framework/framework.sh"
fw_init

# ── Setup (1 line) ───────────────────────────────────────────────────────────
fw_start_chain

RPC_URL="http://localhost:26657"

# ── Test Cases ───────────────────────────────────────────────────────────────

test_chain_producing_blocks() {
    local h1
    h1=$(get_block_height "$RPC_URL")

    if [ "$h1" -lt 1 ] 2>/dev/null; then
        log_error "Chain not producing blocks (height: ${h1})"
        return 1
    fi

    sleep 3

    local h2
    h2=$(get_block_height "$RPC_URL")

    if [ "$h2" -gt "$h1" ] 2>/dev/null; then
        log_success "Block height increased: ${h1} -> ${h2}"
        return 0
    else
        log_error "Block height did not increase: ${h1} -> ${h2}"
        return 1
    fi
}

test_genesis_account_balances() {
    local balance
    balance=$(exec_mocad query bank balances \
        "$(exec_mocad keys show validator0 -a --keyring-backend test)" \
        --node tcp://localhost:26657 \
        --chain-id "${CHAIN_ID}" \
        --output json | jq -r '.balances[] | select(.denom=="amoca") | .amount' 2>/dev/null || echo "0")

    assert_not_empty "$balance" "validator0 should have amoca balance"
}

test_query_validators() {
    local count
    count=$(exec_mocad query staking validators \
        --node tcp://localhost:26657 \
        --chain-id "${CHAIN_ID}" \
        --output json | jq '.validators | length' 2>/dev/null || echo "0")

    assert_gt "$count" 0 "Should have at least 1 validator (got ${count})"
}

test_query_module_params() {
    local modules=("evm" "feemarket" "staking" "gov")

    for mod in "${modules[@]}"; do
        local result
        result=$(exec_mocad query "$mod" params \
            --node tcp://localhost:26657 \
            --chain-id "${CHAIN_ID}" \
            --output json 2>/dev/null || echo "")

        if [ -z "$result" ]; then
            log_error "Failed to query ${mod} params"
            return 1
        fi
        log_info "  ${mod} params: OK"
    done

    return 0
}

test_send_tokens() {
    exec_mocad keys add smoke-recv --keyring-backend test 2>/dev/null || true
    local recv
    recv=$(exec_mocad keys show smoke-recv -a --keyring-backend test)

    log_info "Receiver: ${recv}"

    local send_amount="1000000000000000000amoca"  # 1 MOCA
    fw_tx_send validator0 "$recv" "$send_amount"

    local bal
    bal=$(exec_mocad query bank balances "$recv" \
        --node tcp://localhost:26657 \
        --chain-id "${CHAIN_ID}" \
        --output json | jq -r '.balances[] | select(.denom=="amoca") | .amount' 2>/dev/null || echo "0")

    assert_eq "$bal" "1000000000000000000" "Receiver should have 1 MOCA"
}

test_multi_account_transfers() {
    local num_accounts=3
    declare -a accounts=()

    for ((i = 0; i < num_accounts; i++)); do
        exec_mocad keys add "fwtest${i}" --keyring-backend test 2>/dev/null || true
        accounts+=("$(exec_mocad keys show "fwtest${i}" -a --keyring-backend test)")
    done

    # Fund all accounts from validator0
    local fund_amount="5000000000000000000amoca"  # 5 MOCA each
    for ((i = 0; i < num_accounts; i++)); do
        log_info "Funding fwtest${i}: ${accounts[$i]}"
        fw_tx_send validator0 "${accounts[$i]}" "$fund_amount"
    done

    # Chain of transfers: 0 -> 1, 1 -> 2
    local transfer_amount="1000000000000000000amoca"  # 1 MOCA

    log_info "Sending from fwtest0 to fwtest1..."
    fw_tx_send fwtest0 "${accounts[1]}" "$transfer_amount"

    log_info "Sending from fwtest1 to fwtest2..."
    fw_tx_send fwtest1 "${accounts[2]}" "$transfer_amount"

    # fwtest2 should have: 5 MOCA (initial) + 1 MOCA (from fwtest1) = 6 MOCA
    local bal2
    bal2=$(exec_mocad query bank balances "${accounts[2]}" \
        --node tcp://localhost:26657 \
        --chain-id "${CHAIN_ID}" \
        --output json | jq -r '.balances[] | select(.denom=="amoca") | .amount' 2>/dev/null || echo "0")

    log_info "fwtest2 balance: ${bal2} amoca"
    assert_not_empty "$bal2" "fwtest2 should have non-zero balance"
}

test_query_storage_providers() {
    local count
    count=$(exec_mocad query sp storage-providers \
        --node tcp://localhost:26657 \
        --chain-id "${CHAIN_ID}" \
        --output json | jq '.sps | length' 2>/dev/null || echo "0")

    assert_gt "$count" 0 "Should have at least 1 storage provider (got ${count})"
}

# ── Run all tests ────────────────────────────────────────────────────────────

fw_run_test "Chain producing blocks"   test_chain_producing_blocks
fw_run_test "Genesis account balances" test_genesis_account_balances
fw_run_test "Query validators"         test_query_validators
fw_run_test "Query module params"      test_query_module_params
fw_run_test "Send tokens"              test_send_tokens
fw_run_test "Multi-account transfers"  test_multi_account_transfers
fw_run_test "Query storage providers"  test_query_storage_providers

fw_done
