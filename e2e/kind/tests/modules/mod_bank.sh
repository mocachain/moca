#!/usr/bin/env bash
# Module: bank — bank send transactions.

_BANK_ADDRS=()
_BANK_SEND_IDX=0

bank_setup() {
    log_info "[bank] Creating 10 test accounts..."
    for ((i = 0; i < 10; i++)); do
        exec_mocad keys add "bank-test-${i}" --keyring-backend test 2>/dev/null || true
        _BANK_ADDRS+=("$(exec_mocad keys show "bank-test-${i}" -a --keyring-backend test)")
    done
}

# Single bank send with rotating recipient and amount
bank_send() {
    local amounts=("1000000000000000000" "2000000000000000000" "500000000000000000"
                   "3000000000000000000" "100000000000000000"  "750000000000000000"
                   "4000000000000000000" "250000000000000000"  "1500000000000000000"
                   "5000000000000000000")
    local idx=$((_BANK_SEND_IDX % 10))
    local amt="${amounts[$idx]}"
    log_info "  [bank] send ${amt}amoca -> bank-test-${idx}"
    cosmos_tx bank send validator0 "${_BANK_ADDRS[$idx]}" "${amt}amoca" --from validator0
    _BANK_SEND_IDX=$((_BANK_SEND_IDX + 1))
}

_bank_verify_balances() {
    local bal; bal=$(exec_mocad query bank balances "${_BANK_ADDRS[0]}" \
        --node tcp://localhost:26657 --chain-id "${CHAIN_ID}" \
        --output json 2>/dev/null | jq -r '.balances[] | select(.denom=="amoca") | .amount // "0"') || true
    assert_gt "$bal" "0" "Bank test account should have balance"
}

_bank_verify_send_works() {
    exec_mocad keys add bank-final-recv --keyring-backend test 2>/dev/null || true
    local recv; recv=$(exec_mocad keys show bank-final-recv -a --keyring-backend test)
    fw_tx_send validator0 "$recv" "1000000000000000000amoca"
    local bal; bal=$(exec_mocad query bank balances "$recv" \
        --node tcp://localhost:26657 --chain-id "${CHAIN_ID}" \
        --output json 2>/dev/null | jq -r '.balances[] | select(.denom=="amoca") | .amount // "0"') || true
    assert_eq "$bal" "1000000000000000000" "Fresh bank send should work post-upgrade"
}

register_setup  bank_setup
register_tx     bank_send
register_verify "Bank balances preserved"      _bank_verify_balances
register_verify "Post-upgrade bank send works" _bank_verify_send_works
