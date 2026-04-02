#!/usr/bin/env bash
# RPC + staking parity suite (moca-devcontainer test/validator/RPC/rpc.sh + check-validators balances/validators).

# shellcheck source=/dev/null
source "$(dirname "$0")/../framework/framework.sh"
fw_init

fw_start_chain

EVM_RPC="${EVM_RPC:-http://localhost:8545}"
export EVM_RPC
EVM_CHAIN_ID="${SRC_CHAIN_ID}"
cid=$(cast chain-id --rpc-url "$EVM_RPC" 2>/dev/null || echo "")
if [ -n "$cid" ] && [ "$cid" != "0" ]; then
    EVM_CHAIN_ID="$cid"
fi
VAL0_PRIVKEY="0x${VALIDATOR0_PRIKEY}"
CONTRACTS_DIR="$(cd "$(dirname "$0")/../contracts" && pwd)"
RPC_NODE="tcp://localhost:26657"
CT_RPC="${COMETBFT_RPC_URL:-http://localhost:26657}"
NUM_EXPECT="${NUM_VALIDATORS:-4}"

_rpc_evm_call() {
    cast call "$@" --rpc-url "$EVM_RPC" 2>/dev/null
}

test_evm_connectivity() {
    local code
    code=$(check_http_status "$EVM_RPC")
    if [ "$code" = "000" ] || [ -z "$code" ]; then
        log_error "EVM RPC unreachable (HTTP ${code})"
        return 1
    fi
    if [ "$code" -ge 500 ] 2>/dev/null; then
        log_error "EVM RPC server error (HTTP ${code})"
        return 1
    fi
    return 0
}

test_cometbft_status() {
    local body
    body=$(curl -sf "${CT_RPC}/status" --connect-timeout 5 --max-time 15 2>/dev/null) || {
        log_error "CometBFT /status unreachable"
        return 1
    }
    echo "$body" | jq -e '.result.node_info.id != null' >/dev/null 2>&1 || {
        log_error "/status missing result.node_info"
        return 1
    }
    return 0
}

test_cometbft_health() {
    local body
    body=$(curl -sf "${CT_RPC}/health" --connect-timeout 5 --max-time 15 2>/dev/null) || {
        log_error "CometBFT /health unreachable"
        return 1
    }
    [ -n "$body" ] || {
        log_error "/health empty response"
        return 1
    }
    return 0
}

test_evm_jsonrpc() {
    local resp
    resp=$(evm_rpc_call "eth_blockNumber" "[]") || {
        log_error "eth_blockNumber request failed"
        return 1
    }
    echo "$resp" | jq -e '.jsonrpc == "2.0" and (.result != null)' >/dev/null 2>&1 || {
        log_error "Invalid JSON-RPC 2.0 response for eth_blockNumber"
        return 1
    }
    return 0
}

test_evm_block_production() {
    local now ts diff h1 h2
    now=$(date +%s)
    ts=$(get_evm_block_timestamp) || {
        log_error "Cannot read latest block timestamp"
        return 1
    }
    diff=$((now - ts))
    if [ "$diff" -lt 0 ]; then
        diff=$((ts - now))
    fi
    if [ "$diff" -gt 300 ]; then
        log_error "Latest EVM block timestamp too stale (delta ${diff}s, max 300s)"
        return 1
    fi
    h1=$(get_evm_block_number) || {
        log_error "Cannot read eth block number"
        return 1
    }
    sleep 5
    h2=$(get_evm_block_number) || {
        log_error "Cannot read eth block number (second sample)"
        return 1
    }
    if [ "$h2" -lt "$h1" ] 2>/dev/null; then
        log_error "Block number decreased: ${h1} -> ${h2}"
        return 1
    fi
    return 0
}

test_evm_erc20() {
    local artifact bytecode enc full deploy_out addr sym supply alice_key alice_addr bob_key bob_addr b_alice
    (cd "$CONTRACTS_DIR" && forge build --quiet) || {
        log_error "forge build TestERC20 failed"
        return 1
    }
    artifact="${CONTRACTS_DIR}/out/TestERC20.sol/TestERC20.json"
    if [ ! -f "$artifact" ]; then
        log_error "missing forge artifact: ${artifact}"
        return 1
    fi
    bytecode=$(jq -r '.bytecode.object' "$artifact" 2>/dev/null) || true
    if [ -z "$bytecode" ] || [ "$bytecode" = "null" ]; then
        log_error "could not read bytecode from ${artifact}"
        return 1
    fi
    enc=$(cast abi-encode "constructor(string,string,uint8)" "MocaTestToken" "MTT" 18 2>/dev/null) || true
    if [ -z "$enc" ]; then
        log_error "cast abi-encode constructor args failed"
        return 1
    fi
    full="0x${bytecode#0x}${enc#0x}"
    deploy_out=$(cast send --json \
        --private-key "$VAL0_PRIVKEY" \
        --rpc-url "$EVM_RPC" \
        --chain-id "$EVM_CHAIN_ID" \
        --create "$full" 2>&1) || true
    addr=$(echo "$deploy_out" | jq -r '.contractAddress // empty' 2>/dev/null) || true
    if [ -z "$addr" ]; then
        log_error "cast send --create failed; output: $(echo "$deploy_out" | head -c 1200)"
        return 1
    fi

    sym=$(_rpc_evm_call "$addr" "symbol()(string)" | tr -d '"' | tr -d '\n')
    assert_eq "$sym" "MTT" "ERC20 symbol"

    supply=$(_rpc_evm_call "$addr" "totalSupply()(uint256)")
    assert_eq "$supply" "0" "ERC20 initial totalSupply"

    alice_key=$(cast wallet new --json 2>/dev/null | jq -r '.[0].private_key' 2>/dev/null) || true
    alice_addr=$(cast wallet address "$alice_key" 2>/dev/null) || true
    bob_key=$(cast wallet new --json 2>/dev/null | jq -r '.[0].private_key' 2>/dev/null) || true
    bob_addr=$(cast wallet address "$bob_key" 2>/dev/null) || true
    assert_not_empty "$alice_key" "alice key"
    assert_not_empty "$alice_addr" "alice address"
    assert_not_empty "$bob_key" "bob key"
    assert_not_empty "$bob_addr" "bob address"

    cast send "$alice_addr" --value 1ether \
        --private-key "$VAL0_PRIVKEY" --rpc-url "$EVM_RPC" \
        --chain-id "$EVM_CHAIN_ID" > /dev/null 2>&1 || true
    sleep 4

    cast send "$addr" "mint(address,uint256)" "$alice_addr" "1000000000000000000000" \
        --private-key "$VAL0_PRIVKEY" --rpc-url "$EVM_RPC" --chain-id "$EVM_CHAIN_ID" > /dev/null 2>&1 || true
    sleep 4

    cast send "$addr" "transfer(address,uint256)" "$bob_addr" "100000000000000000000" \
        --private-key "$alice_key" --rpc-url "$EVM_RPC" --chain-id "$EVM_CHAIN_ID" > /dev/null 2>&1 || true
    sleep 4

    b_alice=$(cast call "$addr" "balanceOf(address)(uint256)" "$alice_addr" --rpc-url "$EVM_RPC" 2>/dev/null | awk '{print $1}' || echo "")
    assert_not_empty "$b_alice" "Alice balanceOf after transfer"
    assert_gt "$b_alice" "0" "Alice ERC20 balance after transfer"
    return 0
}

test_validator_balances() {
    local validators_json op amoca
    validators_json=$(exec_mocad query staking validators \
        --node "$RPC_NODE" --chain-id "${CHAIN_ID}" --output json 2>/dev/null) || {
        log_error "staking validators query failed"
        return 1
    }
    while IFS= read -r op; do
        [ -z "$op" ] && continue
        local balance_json
        balance_json=$(exec_mocad query bank balances "$op" \
            --node "$RPC_NODE" --chain-id "${CHAIN_ID}" --output json 2>/dev/null) || {
            log_error "bank balances failed for ${op}"
            return 1
        }
        amoca=$(echo "$balance_json" | jq -r '[.balances[]? | select(.denom=="amoca") | .amount][0] // "0"')
        assert_gt "$amoca" "0" "operator ${op} should have amoca balance"
    done < <(echo "$validators_json" | jq -r '.validators[]?.operator_address // empty')
    return 0
}

test_validator_info() {
    local validators_json count i op mon
    validators_json=$(exec_mocad query staking validators \
        --node "$RPC_NODE" --chain-id "${CHAIN_ID}" --output json 2>/dev/null) || {
        log_error "staking validators query failed"
        return 1
    }
    count=$(echo "$validators_json" | jq -r '.validators | length')
    assert_eq "$count" "$NUM_EXPECT" "validator count should equal NUM_VALIDATORS"

    i=0
    while IFS=$'\t' read -r op mon; do
        assert_not_empty "$op" "validator ${i} operator_address"
        assert_not_empty "$mon" "validator ${i} moniker"
        i=$((i + 1))
    done < <(echo "$validators_json" | jq -r '.validators[] | [.operator_address, (.description.moniker // "unknown")] | @tsv')
    return 0
}

fw_run_test "EVM HTTP connectivity" test_evm_connectivity
fw_run_test "CometBFT /status" test_cometbft_status
fw_run_test "CometBFT /health" test_cometbft_health
fw_run_test "EVM eth_blockNumber JSON-RPC 2.0" test_evm_jsonrpc
fw_run_test "EVM block timestamp freshness + monotonic height" test_evm_block_production
fw_run_test "EVM TestERC20 deploy + transfer" test_evm_erc20
fw_run_test "Validator operator bank balances" test_validator_balances
fw_run_test "Staking validators list + monikers" test_validator_info

fw_done
