#!/usr/bin/env bash
# Module: distribution — reward withdrawals.

_DIST_VAL_IDX=0

# Single distribution tx — withdraw rewards from rotating validator
distribution_tx() {
    local idx=$((_DIST_VAL_IDX % NUM_VALIDATORS))
    log_info "  [distribution] withdraw rewards: validator${idx}"
    cosmos_tx_on "$idx" distribution withdraw-rewards "${VAL_OPERS[$idx]}" --from "validator${idx}"
    _DIST_VAL_IDX=$((_DIST_VAL_IDX + 1))
}

_dist_verify_rewards_available() {
    fw_wait_blocks 3
    local rewards; rewards=$(exec_mocad query distribution rewards \
        "$(exec_mocad keys show validator0 -a --keyring-backend test)" \
        --node tcp://localhost:26657 --chain-id "${CHAIN_ID}" \
        --output json 2>/dev/null | jq -r '.total[] | select(.denom=="amoca") | .amount // "0"') || true
    assert_not_empty "$rewards" "Validator0 should have pending rewards post-upgrade"
}

register_tx     distribution_tx
register_verify "Distribution rewards available" _dist_verify_rewards_available
