#!/usr/bin/env bash
# Module: gov — text proposal submission and voting.

_GOV_PROP_IDX=0

# Single gov tx — submit a text proposal and vote YES from all validators
gov_tx() {
    _GOV_PROP_IDX=$((_GOV_PROP_IDX + 1))
    local title="E2E Proposal ${_GOV_PROP_IDX}"
    local summary="E2E text proposal ${_GOV_PROP_IDX}"
    local tmpfile="/tmp/gov-prop-${_GOV_PROP_IDX}.json"

    log_info "  [gov] submit proposal: ${title}"
    local proposal_json
    proposal_json=$(cat <<PEOF
{"messages":[],"deposit":"${GOV_MIN_DEPOSIT_AMOUNT}${BASIC_DENOM}","title":"${title}","summary":"${summary}"}
PEOF
    )
    write_to_pod "$proposal_json" "$tmpfile"
    cosmos_tx gov submit-proposal "$tmpfile" --from validator0
    sleep 3

    local prop_id
    prop_id=$(exec_mocad query gov proposals \
        --node tcp://localhost:26657 --chain-id "${CHAIN_ID}" \
        --output json 2>/dev/null | jq -r '.proposals[-1].id // .proposals[-1].proposal_id // empty') || true
    if [ -n "$prop_id" ]; then
        for ((v = 0; v < NUM_VALIDATORS; v++)); do
            cosmos_tx_on "$v" gov vote "$prop_id" yes --from "validator${v}"
        done
    fi
}

_gov_verify_proposals_exist() {
    local count; count=$(exec_mocad query gov proposals \
        --node tcp://localhost:26657 --chain-id "${CHAIN_ID}" \
        --output json 2>/dev/null | jq '.proposals | length') || true
    assert_gt "$count" "0" "Proposals should exist post-upgrade"
}

_gov_verify_submit_works() {
    local pre_count; pre_count=$(exec_mocad query gov proposals \
        --node tcp://localhost:26657 --chain-id "${CHAIN_ID}" \
        --output json 2>/dev/null | jq '.proposals | length') || true
    gov_tx
    local post_count; post_count=$(exec_mocad query gov proposals \
        --node tcp://localhost:26657 --chain-id "${CHAIN_ID}" \
        --output json 2>/dev/null | jq '.proposals | length') || true
    assert_gt "$post_count" "$pre_count" "Post-upgrade proposal submission should work"
}

register_tx     gov_tx
register_verify "Proposals preserved"                _gov_verify_proposals_exist
register_verify "Post-upgrade proposal submission"   _gov_verify_submit_works
