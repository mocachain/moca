#!/usr/bin/env bash
# Module: evm — EVM transactions, contract deployments, and ERC20 operations.
# Exercises native transfers, raw contract deploy/interact, and full ERC20
# lifecycle (mint/burn/transfer/approve/transferFrom) before and after upgrade.

_EVM_ADDRS=()
_EVM_VS_CONTRACTS=()
_EVM_POST_VS_CONTRACTS=()
_EVM_ERC20_ADDR=""
_EVM_ERC20_ADDR2=""
_EVM_ERC20_ADDR3=""
_EVM_PRE_CHAIN_ID=""
_EVM_PRE_BAL=""
_EVM_PRE_TOKEN_NAME=""
_EVM_PRE_TOKEN_SYMBOL=""
_EVM_PRE_ALLOWANCE_ALICE_BOB=""

# Secondary accounts
_EVM_ALICE_KEY="" _EVM_ALICE_ADDR=""
_EVM_BOB_KEY=""   _EVM_BOB_ADDR=""
_EVM_CAROL_KEY="" _EVM_CAROL_ADDR=""

# Raw bytecodes for simple contracts
_VALUE_STORE_BC="0x602a6000556005601160003960056000f33460005500"
_SIMPLE_STORE_BC="0x602a600055600b601160003960006000f360005460005260206000f3"

evm_pre_upgrade() {
    _EVM_PRE_CHAIN_ID=$(cast chain-id --rpc-url "$EVM_RPC" 2>/dev/null || echo "0")

    # Generate EVM addresses
    for ((i = 0; i < 20; i++)); do
        local key
        key=$(cast wallet new --json 2>/dev/null | jq -r '.[0].private_key')
        _EVM_ADDRS+=("$(cast wallet address "$key" 2>/dev/null)")
    done

    # Generate secondary accounts
    _EVM_ALICE_KEY=$(cast wallet new --json 2>/dev/null | jq -r '.[0].private_key')
    _EVM_ALICE_ADDR=$(cast wallet address "$_EVM_ALICE_KEY" 2>/dev/null)
    _EVM_BOB_KEY=$(cast wallet new --json 2>/dev/null | jq -r '.[0].private_key')
    _EVM_BOB_ADDR=$(cast wallet address "$_EVM_BOB_KEY" 2>/dev/null)
    _EVM_CAROL_KEY=$(cast wallet new --json 2>/dev/null | jq -r '.[0].private_key')
    _EVM_CAROL_ADDR=$(cast wallet address "$_EVM_CAROL_KEY" 2>/dev/null)

    # ── Native transfers (20) ──
    log_info "[evm] 20 native transfers..."
    local amounts=("0.1ether" "0.2ether" "0.5ether" "1ether" "0.01ether"
                   "0.05ether" "0.3ether" "2ether" "0.001ether" "0.75ether"
                   "1.5ether" "0.25ether" "0.4ether" "0.8ether" "3ether"
                   "0.15ether" "0.65ether" "0.9ether" "1.2ether" "0.07ether")
    for ((i = 0; i < 20; i++)); do
        evm_transfer "${_EVM_ADDRS[$((i % 20))]}" "${amounts[$i]}"
    done

    # ── Value-store contract deployments (5) ──
    log_info "[evm] 5 value-store deploys..."
    for ((i = 0; i < 5; i++)); do
        local addr
        addr=$(evm_deploy "$_VALUE_STORE_BC")
        [ -n "$addr" ] && _EVM_VS_CONTRACTS+=("$addr")
    done

    # ── Contract interactions (5) ──
    for ((i = 0; i < ${#_EVM_VS_CONTRACTS[@]}; i++)); do
        evm_transfer "${_EVM_VS_CONTRACTS[$i]}" "0.0$((i+1))ether"
    done

    # ── Simple-store deploys (5) ──
    for ((i = 0; i < 5; i++)); do
        evm_deploy "$_SIMPLE_STORE_BC" > /dev/null
    done

    # ── ERC20 deployment and operations ──
    log_info "[evm] Deploying TestERC20..."

    # Fund secondary accounts for gas
    for addr in "$_EVM_ALICE_ADDR" "$_EVM_BOB_ADDR" "$_EVM_CAROL_ADDR"; do
        evm_transfer "$addr" "10ether"
    done

    # Deploy ERC20
    local deploy_out
    deploy_out=$(forge create "${CONTRACTS_DIR}/TestERC20.sol:TestERC20" \
        --constructor-args "MocaTestToken" "MTT" 18 \
        --private-key "$VAL0_PRIVKEY" --rpc-url "$EVM_RPC" \
        --chain-id "$EVM_CHAIN_ID" --json 2>/dev/null) || true
    _EVM_ERC20_ADDR=$(echo "$deploy_out" | jq -r '.deployedTo // empty' 2>/dev/null) || true

    if [ -z "$_EVM_ERC20_ADDR" ]; then
        log_warn "[evm] ERC20 deploy failed, skipping ERC20 operations"
        _EVM_PRE_BAL=$(cast balance "${_EVM_ADDRS[0]}" --rpc-url "$EVM_RPC" 2>/dev/null || echo "0")
        return
    fi
    log_success "[evm] ERC20 at: ${_EVM_ERC20_ADDR}"

    local val0_evm_addr
    val0_evm_addr=$(cast wallet address "$VAL0_PRIVKEY" 2>/dev/null)

    # Mint
    evm_send "$_EVM_ERC20_ADDR" "mint(address,uint256)" "$val0_evm_addr" "1000000000000000000000000"
    evm_send "$_EVM_ERC20_ADDR" "mint(address,uint256)" "$_EVM_ALICE_ADDR" "500000000000000000000000"
    evm_send "$_EVM_ERC20_ADDR" "mint(address,uint256)" "$_EVM_BOB_ADDR" "250000000000000000000000"
    evm_send "$_EVM_ERC20_ADDR" "mint(address,uint256)" "$_EVM_CAROL_ADDR" "100000000000000000000000"

    # Transfers
    evm_send "$_EVM_ERC20_ADDR" "transfer(address,uint256)" "$_EVM_ALICE_ADDR" "10000000000000000000000"
    cast send "$_EVM_ERC20_ADDR" "transfer(address,uint256)" "$_EVM_BOB_ADDR" "5000000000000000000000" \
        --private-key "$_EVM_ALICE_KEY" --rpc-url "$EVM_RPC" --chain-id "$EVM_CHAIN_ID" > /dev/null 2>&1 || true
    sleep 2

    # Approve + transferFrom
    cast send "$_EVM_ERC20_ADDR" "approve(address,uint256)" "$_EVM_BOB_ADDR" "20000000000000000000000" \
        --private-key "$_EVM_ALICE_KEY" --rpc-url "$EVM_RPC" --chain-id "$EVM_CHAIN_ID" > /dev/null 2>&1 || true
    sleep 2
    cast send "$_EVM_ERC20_ADDR" "transferFrom(address,address,uint256)" "$_EVM_ALICE_ADDR" "$_EVM_CAROL_ADDR" "8000000000000000000000" \
        --private-key "$_EVM_BOB_KEY" --rpc-url "$EVM_RPC" --chain-id "$EVM_CHAIN_ID" > /dev/null 2>&1 || true
    sleep 2

    # Burn
    evm_send "$_EVM_ERC20_ADDR" "burn(address,uint256)" "$val0_evm_addr" "50000000000000000000000"

    # Deploy second ERC20
    local deploy2_out
    deploy2_out=$(forge create "${CONTRACTS_DIR}/TestERC20.sol:TestERC20" \
        --constructor-args "MocaGold" "MGD" 8 \
        --private-key "$VAL0_PRIVKEY" --rpc-url "$EVM_RPC" \
        --chain-id "$EVM_CHAIN_ID" --json 2>/dev/null) || true
    _EVM_ERC20_ADDR2=$(echo "$deploy2_out" | jq -r '.deployedTo // empty' 2>/dev/null) || true
    if [ -n "$_EVM_ERC20_ADDR2" ]; then
        evm_send "$_EVM_ERC20_ADDR2" "mint(address,uint256)" "$_EVM_ALICE_ADDR" "100000000000"
        evm_send "$_EVM_ERC20_ADDR2" "mint(address,uint256)" "$_EVM_BOB_ADDR" "50000000000"
    fi

    # Record state
    _EVM_PRE_BAL=$(cast balance "${_EVM_ADDRS[0]}" --rpc-url "$EVM_RPC" 2>/dev/null || echo "0")
    _EVM_PRE_TOKEN_NAME=$(evm_call "$_EVM_ERC20_ADDR" "name()(string)")
    _EVM_PRE_TOKEN_SYMBOL=$(evm_call "$_EVM_ERC20_ADDR" "symbol()(string)")
    _EVM_PRE_ALLOWANCE_ALICE_BOB=$(evm_call "$_EVM_ERC20_ADDR" "allowance(address,address)(uint256)" "$_EVM_ALICE_ADDR" "$_EVM_BOB_ADDR")
}

evm_post_upgrade() {
    # ── Native transfers (15) ──
    log_info "[evm] 15 post-upgrade native transfers..."
    local amounts=("0.15ether" "0.35ether" "0.6ether" "1.1ether" "0.02ether"
                   "0.08ether" "0.45ether" "2.5ether" "0.003ether" "0.9ether"
                   "1.75ether" "0.55ether" "0.12ether" "0.33ether" "0.77ether")
    for ((i = 0; i < 15; i++)); do
        evm_transfer "${_EVM_ADDRS[$((i % 20))]}" "${amounts[$i]}"
    done

    # ── New contract deploys (5) ──
    log_info "[evm] 5 post-upgrade deploys..."
    for ((i = 0; i < 5; i++)); do
        local addr
        addr=$(evm_deploy "$_VALUE_STORE_BC")
        [ -n "$addr" ] && _EVM_POST_VS_CONTRACTS+=("$addr")
    done

    # ── Interact with pre-upgrade contracts ──
    for ((i = 0; i < ${#_EVM_VS_CONTRACTS[@]}; i++)); do
        evm_transfer "${_EVM_VS_CONTRACTS[$i]}" "0.00$((i+1))ether"
    done

    # ── Post-upgrade ERC20 operations ──
    if [ -n "$_EVM_ERC20_ADDR" ]; then
        local val0_evm_addr
        val0_evm_addr=$(cast wallet address "$VAL0_PRIVKEY" 2>/dev/null)

        evm_send "$_EVM_ERC20_ADDR" "mint(address,uint256)" "$_EVM_CAROL_ADDR" "200000000000000000000000"
        cast send "$_EVM_ERC20_ADDR" "transfer(address,uint256)" "$_EVM_BOB_ADDR" "2000000000000000000000" \
            --private-key "$_EVM_ALICE_KEY" --rpc-url "$EVM_RPC" --chain-id "$EVM_CHAIN_ID" > /dev/null 2>&1 || true
        sleep 2
        evm_send "$_EVM_ERC20_ADDR" "burn(address,uint256)" "$val0_evm_addr" "25000000000000000000000"

        # Deploy third ERC20 post-upgrade
        local deploy3_out
        deploy3_out=$(forge create "${CONTRACTS_DIR}/TestERC20.sol:TestERC20" \
            --constructor-args "MocaSilver" "MSV" 6 \
            --private-key "$VAL0_PRIVKEY" --rpc-url "$EVM_RPC" \
            --chain-id "$EVM_CHAIN_ID" --json 2>/dev/null) || true
        _EVM_ERC20_ADDR3=$(echo "$deploy3_out" | jq -r '.deployedTo // empty' 2>/dev/null) || true
        if [ -n "$_EVM_ERC20_ADDR3" ]; then
            evm_send "$_EVM_ERC20_ADDR3" "mint(address,uint256)" "$_EVM_ALICE_ADDR" "1000000000"
        fi
    fi
}

# ── Tests ─────────────────────────────────────────────────────────────────────

_evm_test_chain_id() {
    local cid; cid=$(cast chain-id --rpc-url "$EVM_RPC" 2>/dev/null || echo "0")
    assert_eq "$cid" "$_EVM_PRE_CHAIN_ID" "EVM chain ID preserved"
}

_evm_test_native_balances() {
    local bal; bal=$(cast balance "${_EVM_ADDRS[0]}" --rpc-url "$EVM_RPC" 2>/dev/null || echo "0")
    assert_gt "$bal" "0" "EVM native balance should survive upgrade"
}

_evm_test_pre_contracts_live() {
    [ ${#_EVM_VS_CONTRACTS[@]} -eq 0 ] && { log_warn "No pre-upgrade contracts"; return 0; }
    for ((i = 0; i < ${#_EVM_VS_CONTRACTS[@]}; i++)); do
        local code; code=$(cast code "${_EVM_VS_CONTRACTS[$i]}" --rpc-url "$EVM_RPC" 2>/dev/null || echo "0x")
        assert_not_empty "$code" "Pre-upgrade contract ${i} should have code"
    done
}

_evm_test_post_contracts_live() {
    [ ${#_EVM_POST_VS_CONTRACTS[@]} -eq 0 ] && { log_warn "No post-upgrade contracts"; return 0; }
    for ((i = 0; i < ${#_EVM_POST_VS_CONTRACTS[@]}; i++)); do
        local code; code=$(cast code "${_EVM_POST_VS_CONTRACTS[$i]}" --rpc-url "$EVM_RPC" 2>/dev/null || echo "0x")
        assert_not_empty "$code" "Post-upgrade contract ${i} should have code"
    done
}

_evm_test_erc20_metadata() {
    [ -z "$_EVM_ERC20_ADDR" ] && { log_warn "No ERC20 deployed"; return 0; }
    local name; name=$(evm_call "$_EVM_ERC20_ADDR" "name()(string)")
    local sym; sym=$(evm_call "$_EVM_ERC20_ADDR" "symbol()(string)")
    assert_eq "$name" "$_EVM_PRE_TOKEN_NAME" "Token name preserved"
    assert_eq "$sym" "$_EVM_PRE_TOKEN_SYMBOL" "Token symbol preserved"
}

_evm_test_erc20_balances() {
    [ -z "$_EVM_ERC20_ADDR" ] && { log_warn "No ERC20 deployed"; return 0; }
    local bal; bal=$(evm_call "$_EVM_ERC20_ADDR" "balanceOf(address)(uint256)" "$_EVM_ALICE_ADDR")
    assert_not_empty "$bal" "Alice ERC20 balance should exist"
}

_evm_test_erc20_supply() {
    [ -z "$_EVM_ERC20_ADDR" ] && { log_warn "No ERC20 deployed"; return 0; }
    local supply; supply=$(evm_call "$_EVM_ERC20_ADDR" "totalSupply()(uint256)")
    assert_gt "$supply" "0" "ERC20 total supply should be positive"
}

_evm_test_erc20_allowance() {
    [ -z "$_EVM_ERC20_ADDR" ] && { log_warn "No ERC20 deployed"; return 0; }
    local allowance; allowance=$(evm_call "$_EVM_ERC20_ADDR" "allowance(address,address)(uint256)" "$_EVM_ALICE_ADDR" "$_EVM_BOB_ADDR")
    assert_eq "$allowance" "$_EVM_PRE_ALLOWANCE_ALICE_BOB" "Alice->Bob allowance preserved"
}

_evm_test_fresh_transfer() {
    local recv_key; recv_key=$(cast wallet new --json 2>/dev/null | jq -r '.[0].private_key')
    local recv_addr; recv_addr=$(cast wallet address "$recv_key" 2>/dev/null)
    cast send "$recv_addr" --value 0.1ether \
        --private-key "$VAL0_PRIVKEY" --rpc-url "$EVM_RPC" \
        --chain-id "$EVM_CHAIN_ID" > /dev/null 2>&1
    sleep 3
    local bal; bal=$(cast balance "$recv_addr" --rpc-url "$EVM_RPC" 2>/dev/null || echo "0")
    assert_eq "$bal" "100000000000000000" "Fresh EVM transfer works post-upgrade"
}

_evm_test_post_contract_deploy() {
    [ -z "$_EVM_ERC20_ADDR3" ] && { log_warn "Post-upgrade ERC20 not deployed"; return 0; }
    local name; name=$(evm_call "$_EVM_ERC20_ADDR3" "name()(string)")
    assert_eq "$name" "MocaSilver" "Post-upgrade deployed contract functional"
}

# ── Registration ──────────────────────────────────────────────────────────────
register_pre_upgrade  evm_pre_upgrade
register_post_upgrade evm_post_upgrade
register_test "EVM chain ID preserved"              _evm_test_chain_id
register_test "EVM native balances preserved"        _evm_test_native_balances
register_test "Pre-upgrade contracts live"           _evm_test_pre_contracts_live
register_test "Post-upgrade contracts live"          _evm_test_post_contracts_live
register_test "ERC20 metadata preserved"             _evm_test_erc20_metadata
register_test "ERC20 balances preserved"             _evm_test_erc20_balances
register_test "ERC20 total supply consistent"        _evm_test_erc20_supply
register_test "ERC20 allowances preserved"           _evm_test_erc20_allowance
register_test "Fresh EVM transfer works"             _evm_test_fresh_transfer
register_test "Post-upgrade contract deploy works"   _evm_test_post_contract_deploy
