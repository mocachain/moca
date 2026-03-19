#!/usr/bin/env bash
# Module: staking — validator edits, delegations, unbonds.

_STAKING_TX_IDX=0

# Single staking tx — rotates through edit/delegate/unbond
staking_tx() {
    local op=$((_STAKING_TX_IDX % 3))
    local val_idx=$((_STAKING_TX_IDX % NUM_VALIDATORS))

    case $op in
        0)
            log_info "  [staking] edit-validator ${val_idx}"
            cosmos_tx_on "$val_idx" staking edit-validator \
                --moniker "Val${val_idx}-R${_STAKING_TX_IDX}" --from "validator${val_idx}"
            ;;
        1)
            log_info "  [staking] delegate 0.1 MOCA -> validator${val_idx}"
            cosmos_tx staking delegate "${VAL_OPERS[$val_idx]}" \
                "100000000000000000amoca" --from validator0
            ;;
        2)
            log_info "  [staking] unbond 0.01 MOCA <- validator${val_idx}"
            cosmos_tx staking unbond "${VAL_OPERS[$val_idx]}" \
                "10000000000000000amoca" --from validator0
            ;;
    esac
    _STAKING_TX_IDX=$((_STAKING_TX_IDX + 1))
}

_staking_verify_validators_active() {
    local count; count=$(exec_mocad query staking validators \
        --node tcp://localhost:26657 --output json 2>/dev/null \
        | jq '[.validators[] | select(.status=="BOND_STATUS_BONDED")] | length') || true
    assert_eq "$count" "$NUM_VALIDATORS" "All validators should be bonded"
}

_staking_verify_delegate_works() {
    cosmos_tx staking delegate "${VAL_OPERS[1]}" "100000000000000000amoca" --from validator0
    local del; del=$(exec_mocad query staking delegations-to "${VAL_OPERS[1]}" \
        --node tcp://localhost:26657 --output json 2>/dev/null \
        | jq -r '.delegation_responses | length') || true
    assert_gt "$del" "0" "Post-upgrade delegation should work"
}

register_tx     staking_tx
register_verify "All validators active"         _staking_verify_validators_active
register_verify "Post-upgrade delegation works" _staking_verify_delegate_works
