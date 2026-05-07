#!/usr/bin/env bash
# Module: distribution — reward withdrawals.

_DIST_VAL_IDX=0

# Single distribution tx — withdraw rewards from rotating validator.
# We use each validator's own account address as the operator (in Moca,
# validator operator address == account address). VAL_OPERS[] is built
# from the staking query response, whose order is not guaranteed to
# match the validator-N pod naming, so indexing it by `idx` would mix
# up (delegator, validator) and produce a "no delegation" error (code=19).
distribution_tx() {
    local idx=$((_DIST_VAL_IDX % NUM_VALIDATORS))
    log_info "  [distribution] withdraw rewards: validator${idx}"
    local val_addr
    val_addr=$(exec_on_validator "$idx" keys show "validator${idx}" -a --keyring-backend test)
    cosmos_tx_on "$idx" distribution withdraw-rewards "$val_addr" --from "validator${idx}"
    _DIST_VAL_IDX=$((_DIST_VAL_IDX + 1))
}

_dist_verify_rewards_available() {
    fw_wait_blocks 3
    local raw; raw=$(exec_mocad query distribution rewards \
        "$(exec_mocad keys show validator0 -a --keyring-backend test)" \
        --node tcp://localhost:26657 --chain-id "${CHAIN_ID}" \
        --output json 2>/dev/null) || true
    # Handle both object format ({denom, amount}) and string format across SDK versions
    local rewards; rewards=$(echo "$raw" | jq -r '
        [.total[] | if type == "object" then select(.denom=="amoca") | .amount else . end] | first // empty
    ' 2>/dev/null) || true
    assert_not_empty "$rewards" "Validator0 should have pending rewards post-upgrade"
}

register_tx     distribution_tx
register_verify "Distribution rewards available" _dist_verify_rewards_available
