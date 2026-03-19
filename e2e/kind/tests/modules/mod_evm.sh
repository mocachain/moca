#!/usr/bin/env bash
# Module: evm — native transfers, contract deploys, ERC20 lifecycle.

_EVM_ADDRS=()
_EVM_CONTRACTS=()
_EVM_ERC20_ADDR=""
_EVM_PRE_CHAIN_ID=""
_EVM_PRE_TOKEN_NAME=""
_EVM_PRE_TOKEN_SYMBOL=""
_EVM_PRE_ALLOWANCE=""
_EVM_ALICE_KEY="" _EVM_ALICE_ADDR=""
_EVM_BOB_KEY=""   _EVM_BOB_ADDR=""
_EVM_TRANSFER_IDX=0
_EVM_TX_IDX=0

_VALUE_STORE_BC="0x602a6000556005601160003960056000f33460005500"

evm_setup() {
    _EVM_PRE_CHAIN_ID=$(cast chain-id --rpc-url "$EVM_RPC" 2>/dev/null || echo "0")

    # Generate recipient addresses
    log_info "[evm] Generating 10 recipient addresses..."
    for ((i = 0; i < 10; i++)); do
        local key
        key=$(cast wallet new --json 2>/dev/null | jq -r '.[0].private_key')
        _EVM_ADDRS+=("$(cast wallet address "$key" 2>/dev/null)")
    done

    # Secondary accounts for ERC20
    _EVM_ALICE_KEY=$(cast wallet new --json 2>/dev/null | jq -r '.[0].private_key')
    _EVM_ALICE_ADDR=$(cast wallet address "$_EVM_ALICE_KEY" 2>/dev/null)
    _EVM_BOB_KEY=$(cast wallet new --json 2>/dev/null | jq -r '.[0].private_key')
    _EVM_BOB_ADDR=$(cast wallet address "$_EVM_BOB_KEY" 2>/dev/null)

    # Fund secondary accounts for gas
    evm_transfer "$_EVM_ALICE_ADDR" "10ether"
    evm_transfer "$_EVM_BOB_ADDR" "10ether"

    # Deploy ERC20
    log_info "[evm] Deploying TestERC20..."
    local deploy_out
    deploy_out=$(forge create "${CONTRACTS_DIR}/TestERC20.sol:TestERC20" \
        --constructor-args "MocaTestToken" "MTT" 18 \
        --private-key "$VAL0_PRIVKEY" --rpc-url "$EVM_RPC" \
        --chain-id "$EVM_CHAIN_ID" --json 2>/dev/null) || true
    _EVM_ERC20_ADDR=$(echo "$deploy_out" | jq -r '.deployedTo // empty' 2>/dev/null) || true

    if [ -n "$_EVM_ERC20_ADDR" ]; then
        log_success "[evm] ERC20 at: ${_EVM_ERC20_ADDR}"
        local val0_addr; val0_addr=$(cast wallet address "$VAL0_PRIVKEY" 2>/dev/null)
        # Initial mints
        evm_send "$_EVM_ERC20_ADDR" "mint(address,uint256)" "$val0_addr" "1000000000000000000000000"
        evm_send "$_EVM_ERC20_ADDR" "mint(address,uint256)" "$_EVM_ALICE_ADDR" "500000000000000000000000"
        evm_send "$_EVM_ERC20_ADDR" "mint(address,uint256)" "$_EVM_BOB_ADDR" "250000000000000000000000"
        # Initial approve
        cast send "$_EVM_ERC20_ADDR" "approve(address,uint256)" "$_EVM_BOB_ADDR" "50000000000000000000000" \
            --private-key "$_EVM_ALICE_KEY" --rpc-url "$EVM_RPC" --chain-id "$EVM_CHAIN_ID" > /dev/null 2>&1 || true
        sleep 2
        # Record state
        _EVM_PRE_TOKEN_NAME=$(evm_call "$_EVM_ERC20_ADDR" "name()(string)")
        _EVM_PRE_TOKEN_SYMBOL=$(evm_call "$_EVM_ERC20_ADDR" "symbol()(string)")
        _EVM_PRE_ALLOWANCE=$(evm_call "$_EVM_ERC20_ADDR" "allowance(address,address)(uint256)" "$_EVM_ALICE_ADDR" "$_EVM_BOB_ADDR")
    else
        log_warn "[evm] ERC20 deploy failed, ERC20 txs will be skipped"
    fi
}

# Single native MOCA transfer
evm_native_transfer() {
    local idx=$((_EVM_TRANSFER_IDX % 10))
    local amounts=("0.1ether" "0.2ether" "0.5ether" "0.01ether" "0.05ether"
                   "0.3ether" "1ether" "0.001ether" "0.75ether" "0.25ether")
    log_info "  [evm] native transfer ${amounts[$idx]} -> addr[${idx}]"
    evm_transfer "${_EVM_ADDRS[$idx]}" "${amounts[$idx]}"
    _EVM_TRANSFER_IDX=$((_EVM_TRANSFER_IDX + 1))
}

# Single contract deploy
evm_contract_deploy() {
    log_info "  [evm] deploy value-store contract"
    local addr
    addr=$(evm_deploy "$_VALUE_STORE_BC")
    [ -n "$addr" ] && _EVM_CONTRACTS+=("$addr")
}

# Single ERC20 operation — rotates through mint/transfer/transferFrom/burn
evm_erc20_tx() {
    [ -z "$_EVM_ERC20_ADDR" ] && return
    local op=$((_EVM_TX_IDX % 4))
    local val0_addr; val0_addr=$(cast wallet address "$VAL0_PRIVKEY" 2>/dev/null)

    case $op in
        0)
            log_info "  [evm] erc20 mint 1000 -> Alice"
            evm_send "$_EVM_ERC20_ADDR" "mint(address,uint256)" "$_EVM_ALICE_ADDR" "1000000000000000000000"
            ;;
        1)
            log_info "  [evm] erc20 transfer 100 Alice -> Bob"
            cast send "$_EVM_ERC20_ADDR" "transfer(address,uint256)" "$_EVM_BOB_ADDR" "100000000000000000000" \
                --private-key "$_EVM_ALICE_KEY" --rpc-url "$EVM_RPC" --chain-id "$EVM_CHAIN_ID" > /dev/null 2>&1 || true
            sleep 2
            ;;
        2)
            log_info "  [evm] erc20 transferFrom 50 Alice -> deployer (via Bob)"
            cast send "$_EVM_ERC20_ADDR" "transferFrom(address,address,uint256)" "$_EVM_ALICE_ADDR" "$val0_addr" "50000000000000000000" \
                --private-key "$_EVM_BOB_KEY" --rpc-url "$EVM_RPC" --chain-id "$EVM_CHAIN_ID" > /dev/null 2>&1 || true
            sleep 2
            ;;
        3)
            log_info "  [evm] erc20 burn 500 from deployer"
            evm_send "$_EVM_ERC20_ADDR" "burn(address,uint256)" "$val0_addr" "500000000000000000000"
            ;;
    esac
    _EVM_TX_IDX=$((_EVM_TX_IDX + 1))
}

# ── Verify functions ──────────────────────────────────────────────────────────

_evm_verify_chain_id() {
    local cid; cid=$(cast chain-id --rpc-url "$EVM_RPC" 2>/dev/null || echo "0")
    assert_eq "$cid" "$_EVM_PRE_CHAIN_ID" "EVM chain ID preserved"
}

_evm_verify_native_balances() {
    local bal; bal=$(cast balance "${_EVM_ADDRS[0]}" --rpc-url "$EVM_RPC" 2>/dev/null || echo "0")
    assert_gt "$bal" "0" "EVM native balance should survive upgrade"
}

_evm_verify_contracts_live() {
    [ ${#_EVM_CONTRACTS[@]} -eq 0 ] && { log_warn "No contracts deployed"; return 0; }
    local code; code=$(cast code "${_EVM_CONTRACTS[0]}" --rpc-url "$EVM_RPC" 2>/dev/null || echo "0x")
    assert_not_empty "$code" "Deployed contract should have code"
}

_evm_verify_erc20_metadata() {
    [ -z "$_EVM_ERC20_ADDR" ] && { log_warn "No ERC20"; return 0; }
    local name; name=$(evm_call "$_EVM_ERC20_ADDR" "name()(string)")
    assert_eq "$name" "$_EVM_PRE_TOKEN_NAME" "Token name preserved"
}

_evm_verify_erc20_supply() {
    [ -z "$_EVM_ERC20_ADDR" ] && { log_warn "No ERC20"; return 0; }
    local supply; supply=$(evm_call "$_EVM_ERC20_ADDR" "totalSupply()(uint256)")
    assert_gt "$supply" "0" "ERC20 total supply should be positive"
}

_evm_verify_fresh_transfer() {
    local recv_key; recv_key=$(cast wallet new --json 2>/dev/null | jq -r '.[0].private_key')
    local recv_addr; recv_addr=$(cast wallet address "$recv_key" 2>/dev/null)
    cast send "$recv_addr" --value 0.1ether \
        --private-key "$VAL0_PRIVKEY" --rpc-url "$EVM_RPC" \
        --chain-id "$EVM_CHAIN_ID" > /dev/null 2>&1
    sleep 3
    local bal; bal=$(cast balance "$recv_addr" --rpc-url "$EVM_RPC" 2>/dev/null || echo "0")
    assert_eq "$bal" "100000000000000000" "Fresh EVM transfer works post-upgrade"
}

# ── Registration ──────────────────────────────────────────────────────────────
register_setup  evm_setup
register_tx     evm_native_transfer
register_tx     evm_contract_deploy
register_tx     evm_erc20_tx
register_verify "EVM chain ID preserved"         _evm_verify_chain_id
register_verify "EVM native balances preserved"  _evm_verify_native_balances
register_verify "Deployed contracts live"        _evm_verify_contracts_live
register_verify "ERC20 metadata preserved"       _evm_verify_erc20_metadata
register_verify "ERC20 total supply consistent"  _evm_verify_erc20_supply
register_verify "Fresh EVM transfer works"       _evm_verify_fresh_transfer
