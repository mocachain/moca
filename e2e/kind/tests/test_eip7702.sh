#!/usr/bin/env bash
# EIP-7702 (set-EOA-code / account abstraction) suite.
#
# Confirms the property investigated in the Go-level app package test: a
# real signed SetCodeTx, delivered through a live multi-validator cluster's
# actual RPC/ante-handler/consensus pipeline (not a simulated stateDB.SetCode
# shortcut), correctly delegates an EOA to a contract, lets a third party
# invoke that delegated EOA and execute the contract's logic against the
# EOA's OWN storage, and leaves native-token supply/balances correct
# throughout -- moca's dual cosmos/EVM account model does not break under
# EIP-7702, which is active from genesis on this binary (Prague, via
# cosmos/evm). foundry's `cast` has native EIP-7702 support (wallet
# sign-auth, send --auth), so this needs no custom tx-building tooling.

# shellcheck source=/dev/null
source "$(dirname "$0")/../framework/framework.sh"
fw_init

fw_start_chain

EVM_RPC="${EVM_RPC:-http://localhost:8545}"
export EVM_RPC
# moca's cosmos chain-id is conjoint (moca_<evmid>-<epoch>); the EIP-155 EVM
# chain id cast must sign with is its numeric part (see test_rpc_suite.sh).
EVM_CHAIN_ID=$(printf '%s' "${CHAIN_ID}" | sed -E 's/.*_([0-9]+)-.*/\1/')
VAL0_PRIVKEY="0x${VALIDATOR0_PRIKEY}"
CONTRACTS_DIR="$(cd "$(dirname "$0")/../contracts" && pwd)"
RPC_NODE="tcp://localhost:26657"

# ── Shared fixture: one funded EOA, one deployed Counter, delegated once ────
# Every test case below reads this same state rather than re-delegating, so
# a later case can assert on an EARLIER case's on-chain effects (e.g. the
# supply-invariant check needs the EOA to already be delegated).
EOA_KEY=""
EOA_ADDR=""
COUNTER_ADDR=""

_deploy_counter() {
    local artifact bytecode deploy_out addr
    (cd "$CONTRACTS_DIR" && forge build --quiet) || {
        log_error "forge build Counter failed"
        return 1
    }
    artifact="${CONTRACTS_DIR}/out/Counter.sol/Counter.json"
    if [ ! -f "$artifact" ]; then
        log_error "missing forge artifact: ${artifact}"
        return 1
    fi
    bytecode=$(jq -r '.bytecode.object' "$artifact" 2>/dev/null) || true
    if [ -z "$bytecode" ] || [ "$bytecode" = "null" ]; then
        log_error "could not read bytecode from ${artifact}"
        return 1
    fi
    deploy_out=$(cast send --json --private-key "$VAL0_PRIVKEY" --rpc-url "$EVM_RPC" \
        --chain-id "$EVM_CHAIN_ID" --create "0x${bytecode#0x}" 2>&1) || {
        log_error "cast send --create broadcast failed: $(echo "$deploy_out" | head -c 1200)"
        return 1
    }
    addr=$(echo "$deploy_out" | jq -r '.contractAddress // empty' 2>/dev/null) || true
    if [ -z "$addr" ]; then
        log_error "cast send --create returned no contract address; output: $(echo "$deploy_out" | head -c 1200)"
        return 1
    fi
    echo "$addr"
}

test_eip7702_eoa_starts_with_no_code() {
    EOA_KEY=$(cast wallet new --json 2>/dev/null | jq -r '.[0].private_key') || true
    EOA_ADDR=$(cast wallet address "$EOA_KEY" 2>/dev/null) || true
    assert_not_empty "$EOA_KEY" "fresh EOA key"
    assert_not_empty "$EOA_ADDR" "fresh EOA address"

    local fund_out fund_hash
    fund_out=$(cast send "$EOA_ADDR" --value 1ether --private-key "$VAL0_PRIVKEY" \
        --rpc-url "$EVM_RPC" --chain-id "$EVM_CHAIN_ID" --json 2>&1) || {
        log_error "funding fresh EOA failed: $(echo "$fund_out" | head -c 800)"
        return 1
    }
    fund_hash=$(echo "$fund_out" | jq -r '.transactionHash // empty' 2>/dev/null)
    [ -n "$fund_hash" ] && { fw_wait_evm_tx "$fund_hash" 10 "$EVM_RPC" || return 1; }

    local code
    code=$(cast code "$EOA_ADDR" --rpc-url "$EVM_RPC" 2>/dev/null)
    assert_eq "$code" "0x" "fresh EOA must have no code before any delegation"
    return 0
}

test_eip7702_delegate_to_contract() {
    COUNTER_ADDR=$(_deploy_counter) || return 1
    assert_not_empty "$COUNTER_ADDR" "deployed Counter address"

    local auth send_out send_hash
    # --self-broadcast: this EOA both signs the authorization and sends the
    # outer tx, so the authorized nonce must be current+1 (the outer tx's
    # own nonce consumption happens before the authorization list is
    # processed). Without this flag cast signs for the current nonce, which
    # go-ethereum's authorization-list validation then silently skips as
    # stale -- the tx still succeeds, but no delegation gets applied.
    auth=$(cast wallet sign-auth "$COUNTER_ADDR" --self-broadcast --private-key "$EOA_KEY" --rpc-url "$EVM_RPC" 2>/dev/null) || true
    assert_not_empty "$auth" "signed EIP-7702 authorization"

    # Self-authorizing SetCodeTx: EOA both authorizes the delegation and pays
    # for/sends the outer tx. The outer call targets the ZERO address, not
    # the EOA's own address: authorization-list processing (which sets the
    # delegation designator) runs before the main call executes, so a call
    # to the EOA's own address here would immediately dispatch into
    # Counter's (now-delegated) logic with empty calldata -- and Counter has
    # no fallback/receive, so that inner call reverts the whole tx. The zero
    # address is a harmless no-op target; only the auth-list side effect
    # (the delegation write, which is NOT rolled back even if the main call
    # were to fail) matters here.
    send_out=$(cast send "0x0000000000000000000000000000000000000000" --auth "$auth" --private-key "$EOA_KEY" \
        --rpc-url "$EVM_RPC" --chain-id "$EVM_CHAIN_ID" --json 2>&1) || {
        log_error "SetCodeTx broadcast failed: $(echo "$send_out" | head -c 1200)"
        return 1
    }
    send_hash=$(echo "$send_out" | jq -r '.transactionHash // empty' 2>/dev/null) || true
    assert_not_empty "$send_hash" "SetCodeTx transaction hash"
    fw_wait_evm_tx "$send_hash" 15 "$EVM_RPC" || return 1

    local code expect_prefix
    code=$(cast code "$EOA_ADDR" --rpc-url "$EVM_RPC" 2>/dev/null | tr 'A-F' 'a-f')
    # EIP-7702 delegation designator: 0xef0100 ++ 20-byte target address.
    expect_prefix="0xef0100$(echo "${COUNTER_ADDR#0x}" | tr 'A-F' 'a-f')"
    assert_eq "$code" "$expect_prefix" "EOA code must be the EIP-7702 delegation designator pointing at Counter"
    return 0
}

test_eip7702_third_party_invokes_delegated_eoa() {
    local third_key third_addr fund_out fund_hash call_out call_hash
    third_key=$(cast wallet new --json 2>/dev/null | jq -r '.[0].private_key') || true
    third_addr=$(cast wallet address "$third_key" 2>/dev/null) || true
    assert_not_empty "$third_key" "third-party key"
    assert_not_empty "$third_addr" "third-party address"

    fund_out=$(cast send "$third_addr" --value 1ether --private-key "$VAL0_PRIVKEY" \
        --rpc-url "$EVM_RPC" --chain-id "$EVM_CHAIN_ID" --json 2>&1) || {
        log_error "funding third party failed: $(echo "$fund_out" | head -c 800)"
        return 1
    }
    fund_hash=$(echo "$fund_out" | jq -r '.transactionHash // empty' 2>/dev/null)
    [ -n "$fund_hash" ] && { fw_wait_evm_tx "$fund_hash" 10 "$EVM_RPC" || return 1; }

    # The third party -- NOT the delegated EOA itself -- calls the EOA's own
    # address. No --auth here: this is an ordinary call against an address
    # that already carries a delegation designator from the prior test case.
    call_out=$(cast send "$EOA_ADDR" "inc()" --private-key "$third_key" \
        --rpc-url "$EVM_RPC" --chain-id "$EVM_CHAIN_ID" --json 2>&1) || {
        log_error "third-party inc() call failed: $(echo "$call_out" | head -c 1200)"
        return 1
    }
    call_hash=$(echo "$call_out" | jq -r '.transactionHash // empty' 2>/dev/null) || true
    assert_not_empty "$call_hash" "third-party inc() tx hash"
    fw_wait_evm_tx "$call_hash" 15 "$EVM_RPC" || return 1

    local eoa_count eoa_last_caller underlying_count
    eoa_count=$(cast call "$EOA_ADDR" "count()(uint256)" --rpc-url "$EVM_RPC" 2>/dev/null)
    assert_eq "$eoa_count" "1" "inc() must have incremented count in the EOA's OWN storage"

    eoa_last_caller=$(cast call "$EOA_ADDR" "lastCaller()(address)" --rpc-url "$EVM_RPC" 2>/dev/null | tr 'A-F' 'a-f')
    assert_eq "$eoa_last_caller" "$(echo "$third_addr" | tr 'A-F' 'a-f')" "msg.sender inside the delegated call must be the third party who invoked it"

    # The underlying Counter contract's OWN storage must be untouched --
    # delegation executes in the EOA's storage, not the contract's.
    underlying_count=$(cast call "$COUNTER_ADDR" "count()(uint256)" --rpc-url "$EVM_RPC" 2>/dev/null)
    assert_eq "$underlying_count" "0" "underlying Counter contract's own storage must remain untouched"
    return 0
}

test_eip7702_delegated_eoa_native_transfer_supply_invariant() {
    local receiver_key receiver_addr supply_before supply_after
    local send_out send_hash receiver_balance send_amount_wei
    receiver_key=$(cast wallet new --json 2>/dev/null | jq -r '.[0].private_key') || true
    receiver_addr=$(cast wallet address "$receiver_key" 2>/dev/null) || true
    assert_not_empty "$receiver_addr" "receiver address"

    supply_before=$(exec_mocad query bank total-supply-of amoca \
        --node "$RPC_NODE" --chain-id "${CHAIN_ID}" --output json 2>/dev/null | jq -r '.amount // empty') || true
    assert_not_empty "$supply_before" "amoca total supply before transfer"

    # The now-DELEGATED EOA (its own codehash is the 0xef0100 designator)
    # sends a plain native value transfer -- the exact class of scenario
    # #332 fixed (native-token inflation for a "7702-dirtied caller").
    send_amount_wei="100000000000000000" # 0.1 native token
    send_out=$(cast send "$receiver_addr" --value "$send_amount_wei" --private-key "$EOA_KEY" \
        --rpc-url "$EVM_RPC" --chain-id "$EVM_CHAIN_ID" --json 2>&1) || {
        log_error "delegated EOA value transfer failed: $(echo "$send_out" | head -c 1200)"
        return 1
    }
    send_hash=$(echo "$send_out" | jq -r '.transactionHash // empty' 2>/dev/null) || true
    assert_not_empty "$send_hash" "delegated EOA transfer tx hash"
    fw_wait_evm_tx "$send_hash" 15 "$EVM_RPC" || return 1

    receiver_balance=$(cast balance "$receiver_addr" --rpc-url "$EVM_RPC" 2>/dev/null)
    assert_eq "$receiver_balance" "$send_amount_wei" "receiver must be credited exactly the sent amount"

    supply_after=$(exec_mocad query bank total-supply-of amoca \
        --node "$RPC_NODE" --chain-id "${CHAIN_ID}" --output json 2>/dev/null | jq -r '.amount // empty') || true
    assert_not_empty "$supply_after" "amoca total supply after transfer"
    assert_eq "$supply_after" "$supply_before" "total amoca supply must be unchanged after a 7702-delegated EOA sends value"
    return 0
}

fw_run_test "Fresh EOA has no code before delegation" test_eip7702_eoa_starts_with_no_code
fw_run_test "SetCodeTx delegates EOA to deployed Counter" test_eip7702_delegate_to_contract
fw_run_test "Third party invokes delegated EOA (account abstraction + storage isolation)" test_eip7702_third_party_invokes_delegated_eoa
fw_run_test "Delegated EOA native transfer + total supply invariant" test_eip7702_delegated_eoa_native_transfer_supply_invariant

fw_done
