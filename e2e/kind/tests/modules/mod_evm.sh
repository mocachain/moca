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
        key=$(cast_new_privkey)
        _EVM_ADDRS+=("$(cast wallet address "$key" 2>/dev/null)")
    done

    # Secondary accounts for ERC20
    _EVM_ALICE_KEY=$(cast_new_privkey)
    _EVM_ALICE_ADDR=$(cast wallet address "$_EVM_ALICE_KEY" 2>/dev/null)
    _EVM_BOB_KEY=$(cast_new_privkey)
    _EVM_BOB_ADDR=$(cast wallet address "$_EVM_BOB_KEY" 2>/dev/null)

    # Fund secondary accounts for gas
    evm_transfer "$_EVM_ALICE_ADDR" "10ether"
    evm_transfer "$_EVM_BOB_ADDR" "10ether"

    # Deploy ERC20 (same forge build + cast send --create path as the RPC suite). A failed deploy fails the suite.
    log_info "[evm] Deploying TestERC20..."
    local artifact="${CONTRACTS_DIR}/out/TestERC20.sol/TestERC20.json" bytecode ctor_args
    (cd "$CONTRACTS_DIR" && forge build --quiet) || { log_error "[evm] forge build TestERC20 failed"; return 1; }
    bytecode=$(jq -r '.bytecode.object // empty' "$artifact")
    ctor_args=$(cast abi-encode "constructor(string,string,uint8)" "MocaTestToken" "MTT" 18)
    assert_not_empty "$bytecode" "TestERC20 bytecode in ${artifact}"
    assert_not_empty "$ctor_args" "TestERC20 constructor args"
    _EVM_ERC20_ADDR=$(evm_deploy "0x${bytecode#0x}${ctor_args#0x}")
    assert_not_empty "$_EVM_ERC20_ADDR" "TestERC20 deploy returned no contract address"
    log_success "[evm] ERC20 at: ${_EVM_ERC20_ADDR}"

    local val0_addr; val0_addr=$(cast wallet address "$VAL0_PRIVKEY" 2>/dev/null)
    # Initial mints
    evm_send "$_EVM_ERC20_ADDR" "mint(address,uint256)" "$val0_addr" "1000000000000000000000000"
    evm_send "$_EVM_ERC20_ADDR" "mint(address,uint256)" "$_EVM_ALICE_ADDR" "500000000000000000000000"
    evm_send "$_EVM_ERC20_ADDR" "mint(address,uint256)" "$_EVM_BOB_ADDR" "250000000000000000000000"
    # Initial approve
    evm_send_from "$_EVM_ALICE_KEY" "$_EVM_ERC20_ADDR" "approve(address,uint256)" "$_EVM_BOB_ADDR" "50000000000000000000000"
    # Record state
    _EVM_PRE_TOKEN_NAME=$(evm_call "$_EVM_ERC20_ADDR" "name()(string)")
    _EVM_PRE_TOKEN_SYMBOL=$(evm_call "$_EVM_ERC20_ADDR" "symbol()(string)")
    _EVM_PRE_ALLOWANCE=$(evm_call "$_EVM_ERC20_ADDR" "allowance(address,address)(uint256)" "$_EVM_ALICE_ADDR" "$_EVM_BOB_ADDR")
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
    assert_not_empty "$addr" "value-store deploy returned no contract address"
    _EVM_CONTRACTS+=("$addr")
}

# Single ERC20 operation — rotates through mint/transfer/transferFrom/burn
evm_erc20_tx() {
    assert_not_empty "$_EVM_ERC20_ADDR" "ERC20 must have been deployed in setup"
    local op=$((_EVM_TX_IDX % 4))
    local val0_addr; val0_addr=$(cast wallet address "$VAL0_PRIVKEY" 2>/dev/null)

    case $op in
        0)
            log_info "  [evm] erc20 mint 1000 -> Alice"
            evm_send "$_EVM_ERC20_ADDR" "mint(address,uint256)" "$_EVM_ALICE_ADDR" "1000000000000000000000"
            ;;
        1)
            log_info "  [evm] erc20 transfer 100 Alice -> Bob"
            evm_send_from "$_EVM_ALICE_KEY" "$_EVM_ERC20_ADDR" "transfer(address,uint256)" "$_EVM_BOB_ADDR" "100000000000000000000"
            ;;
        2)
            log_info "  [evm] erc20 transferFrom 50 Alice -> deployer (via Bob)"
            evm_send_from "$_EVM_BOB_KEY" "$_EVM_ERC20_ADDR" "transferFrom(address,address,uint256)" "$_EVM_ALICE_ADDR" "$val0_addr" "50000000000000000000"
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
    assert_gt "${#_EVM_CONTRACTS[@]}" "0" "At least one contract should have been deployed"
    local c code
    for c in "${_EVM_CONTRACTS[@]}"; do
        code=$(cast code "$c" --rpc-url "$EVM_RPC") || { log_error "cast code ${c} failed"; return 1; }
        if [ -z "$code" ] || [ "$code" = "0x" ]; then
            log_error "Deployed contract ${c} has no code (got '${code}')"
            return 1
        fi
    done
}

_evm_verify_erc20_metadata() {
    assert_not_empty "$_EVM_ERC20_ADDR" "ERC20 must have been deployed in setup"
    local name; name=$(evm_call "$_EVM_ERC20_ADDR" "name()(string)")
    assert_not_empty "$name" "Token name should be readable"
    assert_eq "$name" "$_EVM_PRE_TOKEN_NAME" "Token name preserved"
}

_evm_verify_erc20_supply() {
    assert_not_empty "$_EVM_ERC20_ADDR" "ERC20 must have been deployed in setup"
    local supply; supply=$(evm_call "$_EVM_ERC20_ADDR" "totalSupply()(uint256)")
    assert_gt "$supply" "0" "ERC20 total supply should be positive"
}

_evm_verify_fresh_transfer() {
    local recv_key; recv_key=$(cast_new_privkey)
    local recv_addr; recv_addr=$(cast wallet address "$recv_key" 2>/dev/null)
    cast send "$recv_addr" --value 0.1ether \
        --private-key "$VAL0_PRIVKEY" --rpc-url "$EVM_RPC" \
        --chain-id "$EVM_CHAIN_ID" --gas-price 30000000000 > /dev/null 2>&1
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
