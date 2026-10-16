#!/usr/bin/env bash
# EIP-7702 suite: real signed SetCodeTxs through a live cluster, checking delegation, account-abstraction
# storage and exact native-token accounting for delegated EOAs (plain transfers and precompile calls).

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

# Tests run in order and share this state: test 2 delegates the EOA that later tests use.
EOA_KEY=""
EOA_ADDR=""
COUNTER_ADDR=""

# Deploy contracts/<name>.sol from validator-0 and print its address.
_deploy_contract() {
    local name="$1" artifact bytecode deploy_out addr
    (cd "$CONTRACTS_DIR" && forge build --quiet) || {
        log_error "forge build ${name} failed"
        return 1
    }
    artifact="${CONTRACTS_DIR}/out/${name}.sol/${name}.json"
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

# Delegate EOA_ADDR to $1 with a SetCodeTx the EOA signs and sends itself.
_delegate_eoa() {
    local target="$1" auth send_out send_hash code
    # --self-broadcast: the EOA also sends the tx, so the authorization must carry nonce+1.
    auth=$(cast wallet sign-auth "$target" --self-broadcast --private-key "$EOA_KEY" --rpc-url "$EVM_RPC" 2>/dev/null) || true
    assert_not_empty "$auth" "signed EIP-7702 authorization"

    # Zero-address target: calling the EOA would run the new code with empty calldata and revert.
    # Explicit --gas-limit: cast >= 1.8 asks for confirmation before gas-estimating a tx with an auth.
    send_out=$(cast send "0x0000000000000000000000000000000000000000" --auth "$auth" --gas-limit 100000 \
        --private-key "$EOA_KEY" --rpc-url "$EVM_RPC" --chain-id "$EVM_CHAIN_ID" --json 2>&1) || {
        log_error "SetCodeTx broadcast failed: $(echo "$send_out" | head -c 1200)"
        return 1
    }
    send_hash=$(echo "$send_out" | jq -r '.transactionHash // empty' 2>/dev/null) || true
    assert_not_empty "$send_hash" "SetCodeTx transaction hash"
    fw_wait_evm_tx "$send_hash" 15 "$EVM_RPC" || return 1

    # Delegation designator: 0xef0100 ++ target address.
    code=$(cast code "$EOA_ADDR" --rpc-url "$EVM_RPC" 2>/dev/null | tr 'A-F' 'a-f')
    assert_eq "$code" "0xef0100$(echo "${target#0x}" | tr 'A-F' 'a-f')" "EOA code must be the delegation designator for ${target}"
}

_amoca_supply() {
    exec_mocad query bank total-supply-of amoca --node "$RPC_NODE" --output json \
        | jq -r '.amount | (if type == "object" then .amount else . end) // empty'
}

# Fee a mined EVM tx paid: gasUsed * effectiveGasPrice (hex quantities in the receipt).
_tx_fee_wei() {
    local receipt gas price
    receipt=$(cast receipt "$1" --rpc-url "$EVM_RPC" --json 2>/dev/null) || return 1
    gas=$(echo "$receipt" | jq -r '.gasUsed // empty' 2>/dev/null)
    price=$(echo "$receipt" | jq -r '.effectiveGasPrice // empty' 2>/dev/null)
    if [ -z "$gas" ] || [ -z "$price" ]; then
        return 1
    fi
    echo "$((gas * price))"
}

# After tx $1 moved $3 wei from the delegated EOA to $2: both balances moved by exactly that
# (plus the fee for the EOA) and total supply did not change.
_assert_exact_settlement() {
    local hash="$1" receiver="$2" amount="$3" eoa_before="$4" supply_before="$5" fee
    fee=$(_tx_fee_wei "$hash") || true
    assert_not_empty "$fee" "fee paid by ${hash}"
    assert_eq "$(cast balance "$receiver" --rpc-url "$EVM_RPC" 2>/dev/null)" "$amount" \
        "receiver must be credited exactly the sent amount"
    assert_eq "$(cast balance "$EOA_ADDR" --rpc-url "$EVM_RPC" 2>/dev/null)" "$((eoa_before - amount - fee))" \
        "delegated EOA must be debited exactly the sent amount plus the fee"
    assert_eq "$(_amoca_supply)" "$supply_before" "total amoca supply must be unchanged"
}

test_eip7702_eoa_starts_with_no_code() {
    EOA_KEY=$(cast_new_privkey) || true
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
    COUNTER_ADDR=$(_deploy_contract Counter) || return 1
    assert_not_empty "$COUNTER_ADDR" "deployed Counter address"
    _delegate_eoa "$COUNTER_ADDR"
    return 0
}

test_eip7702_third_party_invokes_delegated_eoa() {
    local third_key third_addr fund_out fund_hash call_out call_hash
    third_key=$(cast_new_privkey) || true
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

    underlying_count=$(cast call "$COUNTER_ADDR" "count()(uint256)" --rpc-url "$EVM_RPC" 2>/dev/null)
    assert_eq "$underlying_count" "0" "underlying Counter contract's own storage must remain untouched"
    return 0
}

test_eip7702_delegated_eoa_native_transfer() {
    local receiver_key receiver_addr supply_before eoa_before send_out send_hash
    local amount="100000000000000000" # 0.1 amoca
    receiver_key=$(cast_new_privkey) || true
    receiver_addr=$(cast wallet address "$receiver_key" 2>/dev/null) || true
    assert_not_empty "$receiver_addr" "receiver address"

    supply_before=$(_amoca_supply) || true
    assert_not_empty "$supply_before" "amoca total supply before transfer"
    eoa_before=$(cast balance "$EOA_ADDR" --rpc-url "$EVM_RPC" 2>/dev/null) || true
    assert_not_empty "$eoa_before" "delegated EOA balance before transfer"

    send_out=$(cast send "$receiver_addr" --value "$amount" --private-key "$EOA_KEY" \
        --rpc-url "$EVM_RPC" --chain-id "$EVM_CHAIN_ID" --json 2>&1) || {
        log_error "delegated EOA value transfer failed: $(echo "$send_out" | head -c 1200)"
        return 1
    }
    send_hash=$(echo "$send_out" | jq -r '.transactionHash // empty' 2>/dev/null) || true
    assert_not_empty "$send_hash" "delegated EOA transfer tx hash"
    fw_wait_evm_tx "$send_hash" 15 "$EVM_RPC" || return 1

    _assert_exact_settlement "$send_hash" "$receiver_addr" "$amount" "$eoa_before" "$supply_before"
    return 0
}

test_eip7702_delegated_code_calls_bank_precompile() {
    local bank_sender_addr receiver_key receiver_addr supply_before eoa_before send_out send_hash
    local amount="50000000000000000" # 0.05 amoca
    bank_sender_addr=$(_deploy_contract BankSender) || return 1
    assert_not_empty "$bank_sender_addr" "deployed BankSender address"
    _delegate_eoa "$bank_sender_addr"

    receiver_key=$(cast_new_privkey) || true
    receiver_addr=$(cast wallet address "$receiver_key" 2>/dev/null) || true
    assert_not_empty "$receiver_addr" "receiver address"

    supply_before=$(_amoca_supply) || true
    assert_not_empty "$supply_before" "amoca total supply before bank send"
    eoa_before=$(cast balance "$EOA_ADDR" --rpc-url "$EVM_RPC" 2>/dev/null) || true
    assert_not_empty "$eoa_before" "delegated EOA balance before bank send"

    # Self-signed call into the EOA's own code, which calls the bank precompile as the EOA. Explicit
    # --gas-limit: in a block a precompile call needs ~10k more gas than eth_estimateGas returns.
    send_out=$(cast send "$EOA_ADDR" "sendAmoca(address,uint256)" "$receiver_addr" "$amount" --gas-limit 300000 \
        --private-key "$EOA_KEY" --rpc-url "$EVM_RPC" --chain-id "$EVM_CHAIN_ID" --json 2>&1) || {
        log_error "delegated bank send failed: $(echo "$send_out" | head -c 1200)"
        return 1
    }
    send_hash=$(echo "$send_out" | jq -r '.transactionHash // empty' 2>/dev/null) || true
    assert_not_empty "$send_hash" "delegated bank send tx hash"
    fw_wait_evm_tx "$send_hash" 15 "$EVM_RPC" || return 1

    _assert_exact_settlement "$send_hash" "$receiver_addr" "$amount" "$eoa_before" "$supply_before"
    return 0
}

fw_run_test "Fresh EOA has no code before delegation" test_eip7702_eoa_starts_with_no_code
fw_run_test "SetCodeTx delegates EOA to deployed Counter" test_eip7702_delegate_to_contract
fw_run_test "Third party invokes delegated EOA (account abstraction + storage isolation)" test_eip7702_third_party_invokes_delegated_eoa
fw_run_test "Delegated EOA native transfer settles exactly (balances + total supply)" test_eip7702_delegated_eoa_native_transfer
fw_run_test "Delegated EOA code calls the bank precompile, settles exactly (balances + total supply)" test_eip7702_delegated_code_calls_bank_precompile

fw_done
