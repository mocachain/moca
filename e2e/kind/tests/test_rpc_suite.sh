#!/usr/bin/env bash
# RPC + staking parity suite (moca-devcontainer test/validator/RPC/rpc.sh + check-validators balances/validators).

# shellcheck source=/dev/null
source "$(dirname "$0")/../framework/framework.sh"
fw_init

fw_start_chain

EVM_RPC="${EVM_RPC:-http://localhost:8545}"
export EVM_RPC
# moca's cosmos chain-id is conjoint (moca_<evmid>-<epoch>), so the EIP-155 EVM
# chain id is its numeric part (moca_5151-1 -> 5151). Derive the EXPECTED value
# and sign cast txs with it; test_evm_chain_id asserts the node actually reports
# it. (This previously adopted whatever `cast chain-id` returned, so a node that
# fell back to cosmos/evm's 262144 default — evm.evm-chain-id unset in app.toml —
# would sign for 262144 and pass the suite silently.)
EVM_CHAIN_ID=$(printf '%s' "${CHAIN_ID}" | sed -E 's/.*_([0-9]+)-.*/\1/')
VAL0_PRIVKEY="0x${VALIDATOR0_PRIKEY}"
CONTRACTS_DIR="$(cd "$(dirname "$0")/../contracts" && pwd)"
RPC_NODE="tcp://localhost:26657"
CT_RPC="${COMETBFT_RPC_URL:-http://localhost:26657}"
NUM_EXPECT="${NUM_VALIDATORS:-4}"

_rpc_evm_call() {
    cast call "$@" --rpc-url "$EVM_RPC" 2>/dev/null
}

# eth_chainId must equal the EVM id derived from the cosmos chain-id on EVERY
# validator. This is node-local app.toml config: a single validator that fell
# back to cosmos/evm's 262144 default (evm.evm-chain-id unset) would split
# consensus, yet stay invisible if only the NodePort endpoint (validator-0) is
# queried — the other validators keep >2/3 power and the suite passes green. So
# query each validator pod's own EVM RPC and require them all to match.
test_evm_chain_id() {
    local num="${NUM_VALIDATORS:-4}"
    local i hex reported fail=0
    for ((i = 0; i < num; i++)); do
        hex=$(kubectl exec -n "${K8S_NAMESPACE}" "validator-${i}-0" -c mocad -- \
            curl -sS -m 5 -X POST -H "Content-Type: application/json" \
            -d '{"jsonrpc":"2.0","method":"eth_chainId","params":[],"id":1}' \
            http://localhost:8545 2>/dev/null | jq -r '.result // empty' 2>/dev/null) || true
        if ! printf '%s' "$hex" | grep -qiE '^0x[0-9a-f]+$'; then
            log_error "  validator-${i}: eth_chainId query failed (got '${hex}')"
            fail=1
            continue
        fi
        reported=$(( hex )) # 0x141f -> 5151
        if [ "$reported" != "$EVM_CHAIN_ID" ]; then
            log_error "  validator-${i}: eth_chainId=${reported}, expected ${EVM_CHAIN_ID} (from ${CHAIN_ID}); 262144 means evm.evm-chain-id is unset in this pod's app.toml"
            fail=1
        else
            log_info "  validator-${i}: eth_chainId = ${reported}"
        fi
    done
    if [ "$fail" -ne 0 ]; then
        log_error "eth_chainId must be ${EVM_CHAIN_ID} on all ${num} validators (from ${CHAIN_ID})"
        return 1
    fi
    log_info "eth_chainId = ${EVM_CHAIN_ID} on all ${num} validators (matches ${CHAIN_ID})"
    return 0
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
    local deploy_hash
    deploy_out=$(cast send --json \
        --private-key "$VAL0_PRIVKEY" \
        --rpc-url "$EVM_RPC" \
        --chain-id "$EVM_CHAIN_ID" \
        --create "$full" 2>&1) || {
        log_error "cast send --create broadcast failed: $(echo "$deploy_out" | head -c 1200)"
        return 1
    }
    deploy_hash=$(echo "$deploy_out" | jq -r '.transactionHash // empty' 2>/dev/null) || true
    if [ -n "$deploy_hash" ]; then fw_wait_evm_tx "$deploy_hash" 10 "$EVM_RPC" || return 1; fi
    addr=$(echo "$deploy_out" | jq -r '.contractAddress // empty' 2>/dev/null) || true
    if [ -z "$addr" ]; then
        log_error "cast send --create returned no contract address; output: $(echo "$deploy_out" | head -c 1200)"
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

    local fund_out fund_hash
    fund_out=$(cast send "$alice_addr" --value 1ether \
        --private-key "$VAL0_PRIVKEY" --rpc-url "$EVM_RPC" \
        --chain-id "$EVM_CHAIN_ID" --json 2>&1) || { log_error "fund alice failed: $fund_out"; return 1; }
    fund_hash=$(echo "$fund_out" | jq -r '.transactionHash // empty' 2>/dev/null)
    [ -n "$fund_hash" ] && fw_wait_evm_tx "$fund_hash" 10 "$EVM_RPC"

    local mint_out mint_hash
    mint_out=$(cast send "$addr" "mint(address,uint256)" "$alice_addr" "1000000000000000000000" \
        --private-key "$VAL0_PRIVKEY" --rpc-url "$EVM_RPC" --chain-id "$EVM_CHAIN_ID" --json 2>&1) \
        || { log_error "mint failed: $mint_out"; return 1; }
    mint_hash=$(echo "$mint_out" | jq -r '.transactionHash // empty' 2>/dev/null)
    [ -n "$mint_hash" ] && fw_wait_evm_tx "$mint_hash" 10 "$EVM_RPC"

    local xfer_out xfer_hash
    xfer_out=$(cast send "$addr" "transfer(address,uint256)" "$bob_addr" "100000000000000000000" \
        --private-key "$alice_key" --rpc-url "$EVM_RPC" --chain-id "$EVM_CHAIN_ID" --json 2>&1) \
        || { log_error "transfer failed: $xfer_out"; return 1; }
    xfer_hash=$(echo "$xfer_out" | jq -r '.transactionHash // empty' 2>/dev/null)
    [ -n "$xfer_hash" ] && fw_wait_evm_tx "$xfer_hash" 10 "$EVM_RPC"

    b_alice=$(cast call "$addr" "balanceOf(address)(uint256)" "$alice_addr" --rpc-url "$EVM_RPC" 2>/dev/null | awk '{print $1}' || echo "")
    assert_not_empty "$b_alice" "Alice balanceOf after transfer"
    assert_gt "$b_alice" "0" "Alice ERC20 balance after transfer"
    return 0
}

# Exercises the log-rehydration path introduced with cosmos/evm v0.6.0: EVM logs are
# now decoded from finalized block results (backend GetLogsByHeight /
# GetLogsFromBlockResults) rather than from per-tx cosmos events. Deploys TestERC20,
# opens a logs filter, emits a Transfer, and asserts the log surfaces through the
# receipt, eth_getFilterChanges (the live NewFilter poller) and eth_getLogs.
test_evm_log_subscription() {
    local artifact bytecode enc full deploy_out addr deploy_hash
    local val0_addr transfer_topic filter_id mint_out mint_hash mint_block recv_logs
    local changes changes_len log_addr log_topic get_len i
    local log_txhash log_blockhash log_blocktime mint_blockhash
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
    enc=$(cast abi-encode "constructor(string,string,uint8)" "MocaLogToken" "MLT" 18 2>/dev/null) || true
    if [ -z "$enc" ]; then
        log_error "cast abi-encode constructor args failed"
        return 1
    fi
    full="0x${bytecode#0x}${enc#0x}"
    deploy_out=$(cast send --json \
        --private-key "$VAL0_PRIVKEY" \
        --rpc-url "$EVM_RPC" \
        --chain-id "$EVM_CHAIN_ID" \
        --create "$full" 2>&1) || {
        log_error "cast send --create broadcast failed: $(echo "$deploy_out" | head -c 1200)"
        return 1
    }
    deploy_hash=$(echo "$deploy_out" | jq -r '.transactionHash // empty' 2>/dev/null) || true
    if [ -n "$deploy_hash" ]; then fw_wait_evm_tx "$deploy_hash" 10 "$EVM_RPC" || return 1; fi
    addr=$(echo "$deploy_out" | jq -r '.contractAddress // empty' 2>/dev/null) || true
    if [ -z "$addr" ]; then
        log_error "cast send --create returned no contract address; output: $(echo "$deploy_out" | head -c 1200)"
        return 1
    fi

    val0_addr=$(cast wallet address "$VAL0_PRIVKEY" 2>/dev/null) || true
    assert_not_empty "$val0_addr" "val0 address"

    transfer_topic=$(cast keccak "Transfer(address,address,uint256)" 2>/dev/null) || true
    assert_not_empty "$transfer_topic" "Transfer event topic"

    # Open a logs filter BEFORE emitting so the NewFilter poller captures the new log.
    # eth_getFilterChanges is fed by the same finalized-block-result rehydration
    # (backend GetLogsByHeight / GetLogsFromBlockResults) as eth_subscribe("logs"):
    # filters/api.go NewFilter()+Logs() and websockets.go subscribeLogs all source logs
    # from the block result, not the live tx events. Asserting getFilterChanges here
    # exercises the reviewer's concern without needing a WS client (websocat/wscat).
    filter_id=$(cast rpc eth_newFilter "{\"address\":\"${addr}\"}" --rpc-url "$EVM_RPC" 2>/dev/null | tr -d '"' | tr -d '\n') || true
    assert_not_empty "$filter_id" "eth_newFilter filter id"

    # Emit a Transfer(0x0 -> val0) by minting; val0 is the contract owner/deployer.
    mint_out=$(cast send "$addr" "mint(address,uint256)" "$val0_addr" "1000000000000000000000" \
        --private-key "$VAL0_PRIVKEY" --rpc-url "$EVM_RPC" --chain-id "$EVM_CHAIN_ID" --json 2>&1) \
        || { log_error "mint failed: $mint_out"; return 1; }
    mint_hash=$(echo "$mint_out" | jq -r '.transactionHash // empty' 2>/dev/null) || true
    if [ -n "$mint_hash" ]; then fw_wait_evm_tx "$mint_hash" 10 "$EVM_RPC" || return 1; fi

    # Receipt logs must be populated (tx_info.go GetTransactionReceipt DecodeMsgLogs path).
    recv_logs=$(cast receipt "$mint_hash" --rpc-url "$EVM_RPC" --json 2>/dev/null | jq -r '.logs | length' 2>/dev/null) || true
    assert_not_empty "$recv_logs" "mint receipt logs length"
    assert_gt "$recv_logs" "0" "mint receipt should contain logs"

    mint_block=$(cast receipt "$mint_hash" --rpc-url "$EVM_RPC" --json 2>/dev/null | jq -r '.blockNumber // empty' 2>/dev/null) || true
    assert_not_empty "$mint_block" "mint block number"

    # Poll eth_getFilterChanges: the poller ingests block results asynchronously, so give
    # it a few seconds to surface the rehydrated log.
    changes="[]"
    changes_len="0"
    for ((i = 0; i < 12; i++)); do
        changes=$(cast rpc eth_getFilterChanges "$filter_id" --rpc-url "$EVM_RPC" 2>/dev/null) || changes="[]"
        changes_len=$(echo "$changes" | jq -r 'length' 2>/dev/null) || changes_len="0"
        if [ -n "$changes_len" ] && [ "$changes_len" != "0" ]; then break; fi
        sleep 1
    done
    assert_gt "$changes_len" "0" "eth_getFilterChanges should surface the rehydrated log"

    log_addr=$(echo "$changes" | jq -r '.[0].address // empty' 2>/dev/null | tr 'A-F' 'a-f') || true
    log_topic=$(echo "$changes" | jq -r '.[0].topics[0] // empty' 2>/dev/null | tr 'A-F' 'a-f') || true
    assert_eq "$log_addr" "$(echo "$addr" | tr 'A-F' 'a-f')" "filter log address matches contract"
    assert_eq "$log_topic" "$(echo "$transfer_topic" | tr 'A-F' 'a-f')" "filter log topic0 is Transfer"

    # Regression guard for the actual fix: the rehydrated log must carry finalized
    # block context (transactionHash / blockHash / blockTimestamp) sourced from the
    # block result. Pre-fix these came from the live tx event and were empty/zero,
    # yet address + topics still matched — so those assertions alone did not guard it.
    mint_blockhash=$(cast receipt "$mint_hash" --rpc-url "$EVM_RPC" --json 2>/dev/null | jq -r '.blockHash // empty') || true
    log_txhash=$(echo "$changes" | jq -r '.[0].transactionHash // empty' 2>/dev/null | tr 'A-F' 'a-f') || true
    log_blockhash=$(echo "$changes" | jq -r '.[0].blockHash // empty' 2>/dev/null | tr 'A-F' 'a-f') || true
    log_blocktime=$(echo "$changes" | jq -r '.[0].blockTimestamp // empty' 2>/dev/null) || true
    assert_eq "$log_txhash" "$(echo "$mint_hash" | tr 'A-F' 'a-f')" "filter log transactionHash matches mint tx"
    assert_eq "$log_blockhash" "$(echo "$mint_blockhash" | tr 'A-F' 'a-f')" "filter log blockHash matches finalized block"
    assert_not_empty "$log_blocktime" "filter log blockTimestamp is populated"
    if [ "$log_blocktime" = "0x0" ] || [ "$log_blocktime" = "0x" ] || [ "$log_blocktime" = "0" ]; then
        log_error "filter log blockTimestamp is zero (finalized context missing)"
        return 1
    fi

    # eth_getLogs must also return the historical log from the finalized block result
    # (backend GetLogsByHeight). Pin the range to the mint block so the query is
    # deterministic (default fromBlock/toBlock = latest would race block production).
    get_len=$(cast rpc eth_getLogs "{\"fromBlock\":\"${mint_block}\",\"toBlock\":\"${mint_block}\",\"address\":\"${addr}\",\"topics\":[\"${transfer_topic}\"]}" --rpc-url "$EVM_RPC" 2>/dev/null | jq -r 'length' 2>/dev/null) || true
    assert_not_empty "$get_len" "eth_getLogs length"
    assert_gt "$get_len" "0" "eth_getLogs should return the Transfer log"

    # Clean up the filter.
    cast rpc eth_uninstallFilter "$filter_id" --rpc-url "$EVM_RPC" >/dev/null 2>&1 || true
    return 0
}

# Exercises the eth_subscribe("logs") WEBSOCKET transport end-to-end. The HTTP
# test above covers eth_getFilterChanges/eth_getLogs; this one drives the real WS
# push path in server/websockets.go subscribeLogs (which rehydrates logs from the
# finalized block result). It uses a small first-party go-ethereum client
# (e2e/kind/tools/wslogs, SubscribeFilterLogs) rather than an external ws CLI:
# deploy TestERC20, open a WS logs subscription, then — only once the sub is live
# (subscriptions are future-only) — mint to emit a Transfer and assert the pushed
# log carries the contract address and the Transfer topic.
test_evm_ws_log_subscription() {
    local ws_url host repo_root work_dir tool_bin out_file err_file
    local artifact bytecode enc full deploy_out addr deploy_hash
    local val0_addr transfer_topic mint_out mint_hash
    local tool_pid ws_timeout subscribed exited rc i log_addr log_topic
    local log_txhash log_blockhash log_blocktime mint_blockhash

    ws_timeout=40

    # ── Resolve the WS URL: explicit EVM_WS, else derive ws://HOST:8546 from the
    #    HTTP EVM_RPC. Port 8546 is the EVM WS nodePort (see manifests/base
    #    kind-config.yaml + validator-services.yaml), reachable on the host the
    #    same way :8545 is.
    if [ -n "${EVM_WS:-}" ]; then
        ws_url="$EVM_WS"
    else
        host=$(printf '%s' "$EVM_RPC" | sed -E 's#^[a-zA-Z]+://##; s#/.*$##; s#:[0-9]+$##')
        [ -z "$host" ] && host="localhost"
        ws_url="ws://${host}:8546"
    fi
    log_info "  WS endpoint: ${ws_url}"

    # ── Build the wslogs helper to a temp binary. Building (vs. `go run`) gives a
    #    single PID we can reliably kill and surfaces compile errors up front.
    if ! command -v go >/dev/null 2>&1; then
        log_error "go toolchain not found on host; required to build the WS logs helper"
        return 1
    fi
    repo_root=$(cd -- "${E2E_DIR}/../.." && pwd)
    work_dir=$(mktemp -d "${TMPDIR:-/tmp}/moca-wslogs.XXXXXX") || {
        log_error "mktemp -d failed"
        return 1
    }
    tool_bin="${work_dir}/wslogs"
    out_file="${work_dir}/out.json"
    err_file="${work_dir}/err.log"
    # Kill the bg tool (if launched) and drop the temp dir on any explicit return.
    # `trap - RETURN` first so this fires exactly once (for this function) and does
    # not re-fire — with its locals out of scope — when fw_run_test later returns.
    trap 'trap - RETURN; if [ -n "${tool_pid:-}" ]; then kill "${tool_pid}" 2>/dev/null || true; fi; rm -rf "${work_dir:-}" 2>/dev/null || true' RETURN
    if ! (cd "$repo_root" &&
        CGO_CFLAGS="-O -D__BLST_PORTABLE__" CGO_CFLAGS_ALLOW="-O -D__BLST_PORTABLE__" \
            go build -o "$tool_bin" ./e2e/kind/tools/wslogs) 2>"${work_dir}/build.log"; then
        log_error "failed to build wslogs helper: $(head -c 800 "${work_dir}/build.log" 2>/dev/null)"
        return 1
    fi

    # ── Deploy a fresh TestERC20 (same pattern as the HTTP test above).
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
    enc=$(cast abi-encode "constructor(string,string,uint8)" "MocaWsToken" "MWS" 18 2>/dev/null) || true
    if [ -z "$enc" ]; then
        log_error "cast abi-encode constructor args failed"
        return 1
    fi
    full="0x${bytecode#0x}${enc#0x}"
    deploy_out=$(cast send --json \
        --private-key "$VAL0_PRIVKEY" \
        --rpc-url "$EVM_RPC" \
        --chain-id "$EVM_CHAIN_ID" \
        --create "$full" 2>&1) || {
        log_error "cast send --create broadcast failed: $(echo "$deploy_out" | head -c 1200)"
        return 1
    }
    deploy_hash=$(echo "$deploy_out" | jq -r '.transactionHash // empty' 2>/dev/null) || true
    if [ -n "$deploy_hash" ]; then fw_wait_evm_tx "$deploy_hash" 10 "$EVM_RPC" || return 1; fi
    addr=$(echo "$deploy_out" | jq -r '.contractAddress // empty' 2>/dev/null) || true
    if [ -z "$addr" ]; then
        log_error "cast send --create returned no contract address; output: $(echo "$deploy_out" | head -c 1200)"
        return 1
    fi

    val0_addr=$(cast wallet address "$VAL0_PRIVKEY" 2>/dev/null) || true
    assert_not_empty "$val0_addr" "val0 address"
    transfer_topic=$(cast keccak "Transfer(address,address,uint256)" 2>/dev/null) || true
    assert_not_empty "$transfer_topic" "Transfer event topic"

    # ── Open the WS subscription in the background BEFORE emitting the tx.
    "$tool_bin" "$ws_url" "$addr" "$ws_timeout" >"$out_file" 2>"$err_file" &
    tool_pid=$!

    # ── Wait for the helper to report SUBSCRIBED (up to ~15s), so the tx below is
    #    emitted only after the subscription is live (eth_subscribe is future-only).
    subscribed=false
    for ((i = 0; i < 30; i++)); do
        if grep -q "SUBSCRIBED" "$err_file" 2>/dev/null; then subscribed=true; break; fi
        if ! kill -0 "$tool_pid" 2>/dev/null; then break; fi # helper exited early
        sleep 0.5
    done
    if [ "$subscribed" != "true" ]; then
        log_error "wslogs never reported SUBSCRIBED (ws=${ws_url}); stderr: $(head -c 800 "$err_file" 2>/dev/null)"
        return 1
    fi

    # ── Emit a Transfer(0x0 -> val0) by minting; val0 is the contract deployer.
    mint_out=$(cast send "$addr" "mint(address,uint256)" "$val0_addr" "1000000000000000000000" \
        --private-key "$VAL0_PRIVKEY" --rpc-url "$EVM_RPC" --chain-id "$EVM_CHAIN_ID" --json 2>&1) \
        || { log_error "mint failed: $mint_out"; return 1; }
    mint_hash=$(echo "$mint_out" | jq -r '.transactionHash // empty' 2>/dev/null) || true
    if [ -n "$mint_hash" ]; then fw_wait_evm_tx "$mint_hash" 15 "$EVM_RPC" || return 1; fi

    # ── Wait for the helper to exit (it self-times-out at ${ws_timeout}s; give slack).
    exited=false
    for ((i = 0; i < ws_timeout + 15; i++)); do
        if ! kill -0 "$tool_pid" 2>/dev/null; then exited=true; break; fi
        sleep 1
    done
    if [ "$exited" != "true" ]; then
        log_error "wslogs did not exit within $((ws_timeout + 15))s; stderr: $(head -c 800 "$err_file" 2>/dev/null)"
        return 1
    fi
    rc=0
    wait "$tool_pid" || rc=$?
    if [ "$rc" != "0" ]; then
        log_error "wslogs exited non-zero (rc=${rc}); stderr: $(head -c 800 "$err_file" 2>/dev/null)"
        return 1
    fi

    # ── Assert the pushed log matches the contract + Transfer topic (case-insensitive).
    log_addr=$(jq -r '.address // empty' "$out_file" 2>/dev/null | tr 'A-F' 'a-f') || true
    log_topic=$(jq -r '.topics[0] // empty' "$out_file" 2>/dev/null | tr 'A-F' 'a-f') || true
    assert_eq "$log_addr" "$(echo "$addr" | tr 'A-F' 'a-f')" "WS pushed log address matches contract"
    assert_eq "$log_topic" "$(echo "$transfer_topic" | tr 'A-F' 'a-f')" "WS pushed log topic0 is Transfer"

    # ── Regression guard: the pushed log must carry finalized block context
    #    (transactionHash / blockHash / blockTimestamp) from the block result, not
    #    the live tx event — pre-fix these were empty/zero while address+topics matched.
    mint_blockhash=$(cast receipt "$mint_hash" --rpc-url "$EVM_RPC" --json 2>/dev/null | jq -r '.blockHash // empty') || true
    log_txhash=$(jq -r '.transactionHash // empty' "$out_file" 2>/dev/null | tr 'A-F' 'a-f') || true
    log_blockhash=$(jq -r '.blockHash // empty' "$out_file" 2>/dev/null | tr 'A-F' 'a-f') || true
    log_blocktime=$(jq -r '.blockTimestamp // empty' "$out_file" 2>/dev/null) || true
    assert_eq "$log_txhash" "$(echo "$mint_hash" | tr 'A-F' 'a-f')" "WS pushed log transactionHash matches mint tx"
    assert_eq "$log_blockhash" "$(echo "$mint_blockhash" | tr 'A-F' 'a-f')" "WS pushed log blockHash matches finalized block"
    assert_not_empty "$log_blocktime" "WS pushed log blockTimestamp is populated"
    if [ "$log_blocktime" = "0x0" ] || [ "$log_blocktime" = "0x" ] || [ "$log_blocktime" = "0" ]; then
        log_error "WS pushed log blockTimestamp is zero (finalized context missing)"
        return 1
    fi
    return 0
}

# ── EVM tx indexer (cosmos/evm KV indexer) ───────────────────────────────────
# Validators run with enable-indexer=true (init-chain.sh). The assertion must
# be indexer-specific: with the indexer off/broken, receipt/tx-hash lookups
# silently fall back to CometBFT tx_search, so a green RPC suite alone proves
# nothing about the indexer. Assert the indexed artifact itself: the tx-hash
# key is written to validator-0's evmindexer.db only by KVIndexer.IndexBlock.
test_evm_tx_indexer() {
    local val0_addr send_out tx_hash hash_hex found i err_count

    if ! command -v kubectl >/dev/null 2>&1; then
        log_error "kubectl not available; cannot assert the tx indexer"
        return 1
    fi

    val0_addr=$(cast wallet address "$VAL0_PRIVKEY" 2>/dev/null) || true
    assert_not_empty "$val0_addr" "val0 address"

    # Emit a fresh EVM tx to index (self-transfer).
    send_out=$(cast send "$val0_addr" --value 1 \
        --private-key "$VAL0_PRIVKEY" --rpc-url "$EVM_RPC" --chain-id "$EVM_CHAIN_ID" --json 2>&1) \
        || { log_error "indexer probe tx failed: $(echo "$send_out" | head -c 800)"; return 1; }
    tx_hash=$(echo "$send_out" | jq -r '.transactionHash // empty' 2>/dev/null) || true
    assert_not_empty "$tx_hash" "indexer probe tx hash"
    fw_wait_evm_tx "$tx_hash" 15 "$EVM_RPC" || return 1
    hash_hex=$(echo "$tx_hash" | tr 'A-F' 'a-f' | sed 's/^0x//')

    # The EVMIndexerService indexes asynchronously on new blocks; poll briefly.
    # Entries live in the LevelDB WAL (*.log) until compaction (*.ldb).
    found=0
    for ((i = 0; i < 15; i++)); do
        if kubectl exec -n "${K8S_NAMESPACE:-moca-e2e}" validator-0-0 -c mocad -- \
            sh -c 'cat /root/.mocad/data/evmindexer.db/*.log /root/.mocad/data/evmindexer.db/*.ldb 2>/dev/null | od -An -v -t x1 | tr -d " \n"' 2>/dev/null \
            | grep -q "$hash_hex"; then
            found=1
            break
        fi
        sleep 1
    done
    assert_eq "$found" "1" "tx hash indexed into validator-0's evmindexer.db"

    # And the indexer produced no errors on any indexed block.
    err_count=$(kubectl logs -n "${K8S_NAMESPACE:-moca-e2e}" validator-0-0 -c mocad --tail=-1 2>/dev/null \
        | grep -cE 'failed to index block|Fail to decode tx|Fail to parse event' || true)
    assert_eq "$err_count" "0" "no EVM indexer errors in validator-0 logs"
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
fw_run_test "EVM eth_chainId matches cosmos chain-id" test_evm_chain_id
fw_run_test "CometBFT /status" test_cometbft_status
fw_run_test "CometBFT /health" test_cometbft_health
fw_run_test "EVM eth_blockNumber JSON-RPC 2.0" test_evm_jsonrpc
fw_run_test "EVM block timestamp freshness + monotonic height" test_evm_block_production
fw_run_test "EVM TestERC20 deploy + transfer" test_evm_erc20
fw_run_test "EVM log subscription + getFilterChanges rehydration" test_evm_log_subscription
fw_run_test "EVM eth_subscribe(logs) WebSocket transport" test_evm_ws_log_subscription
fw_run_test "EVM tx indexer (cosmos/evm KV indexer)" test_evm_tx_indexer
fw_run_test "Validator operator bank balances" test_validator_balances
fw_run_test "Staking validators list + monikers" test_validator_info

fw_done
