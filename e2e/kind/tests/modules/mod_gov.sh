#!/usr/bin/env bash
# Module: gov — text proposal submission and voting.

_GOV_PROP_IDX=0

# Submit a gov proposal using direct kubectl exec (same pattern as upgrade-chain.sh).
# This bypasses the cosmos_tx helper which silences all output and has proven unreliable
# for submit-proposal commands that require file arguments.
# Usage: _gov_submit_proposal <proposal_json> <tmpfile>
# Prints the new proposal ID on stdout if created.
_gov_submit_proposal() {
    local proposal_json="$1"
    local tmpfile="$2"
    local fees="200000000000000amoca"

    # Count proposals before submission
    local before_count
    before_count=$(kubectl exec -n "${K8S_NAMESPACE}" validator-0-0 -c mocad -- \
        mocad query gov proposals \
        --node tcp://localhost:26657 --chain-id "${CHAIN_ID}" \
        --home /root/.mocad --output json 2>/dev/null \
        | jq '.proposals | length' 2>/dev/null) || true
    before_count="${before_count:-0}"

    # Write proposal JSON into the pod
    echo "$proposal_json" | kubectl exec -i -n "${K8S_NAMESPACE}" validator-0-0 -c mocad -- \
        bash -c "cat > ${tmpfile}" 2>/dev/null || true

    # Verify the file was written
    local written
    written=$(kubectl exec -n "${K8S_NAMESPACE}" validator-0-0 -c mocad -- \
        cat "${tmpfile}" 2>/dev/null) || true
    if [ -z "$written" ]; then
        log_warn "  [gov] Failed to write proposal JSON to pod at ${tmpfile}" >&2
        return 1
    fi

    # Submit proposal via sync broadcast so we deterministically get a txhash
    local submit_out broadcast_code txhash
    submit_out=$(kubectl exec -n "${K8S_NAMESPACE}" validator-0-0 -c mocad -- \
        mocad tx gov submit-proposal "${tmpfile}" \
        --from validator0 \
        --keyring-backend test \
        --chain-id "${CHAIN_ID}" \
        --node tcp://localhost:26657 \
        --fees "$fees" \
        --home /root/.mocad \
        --broadcast-mode sync -y --output json 2>&1) || {
        log_warn "  [gov] broadcast failed: $submit_out" >&2
        return 1
    }
    broadcast_code=$(echo "$submit_out" | jq -r '.code // empty' 2>/dev/null)
    txhash=$(echo "$submit_out" | jq -r '.txhash // empty' 2>/dev/null)
    log_info "  [gov] broadcast response: code=${broadcast_code:-?} txhash=${txhash:-none}" >&2
    if [ -z "$txhash" ]; then
        log_warn "  [gov] no txhash in broadcast response: $submit_out" >&2
        return 1
    fi

    # Wait for inclusion + on-chain success
    fw_wait_cosmos_tx "$txhash" || return 1

    # Verify proposal was actually created by checking count increased
    local after_count
    after_count=$(kubectl exec -n "${K8S_NAMESPACE}" validator-0-0 -c mocad -- \
        mocad query gov proposals \
        --node tcp://localhost:26657 --chain-id "${CHAIN_ID}" \
        --home /root/.mocad --output json 2>/dev/null \
        | jq '.proposals | length' 2>/dev/null) || true
    after_count="${after_count:-0}"

    if [ "$after_count" -le "$before_count" ] 2>/dev/null; then
        log_warn "  [gov] proposal NOT created (count: ${before_count} -> ${after_count})" >&2
        return 1
    fi

    # Return the new proposal ID
    local new_id
    new_id=$(kubectl exec -n "${K8S_NAMESPACE}" validator-0-0 -c mocad -- \
        mocad query gov proposals \
        --node tcp://localhost:26657 --chain-id "${CHAIN_ID}" \
        --home /root/.mocad --output json 2>/dev/null \
        | jq -r '.proposals[-1].id // .proposals[-1].proposal_id // empty' 2>/dev/null) || true
    log_info "  [gov] proposal created: id=${new_id} (count: ${before_count} -> ${after_count})" >&2
    echo "$new_id"
    return 0
}

# Single gov tx — submit a text proposal and vote YES from all validators
gov_tx() {
    _GOV_PROP_IDX=$((_GOV_PROP_IDX + 1))
    local title="E2E Proposal ${_GOV_PROP_IDX}"
    local summary="E2E text proposal ${_GOV_PROP_IDX}"
    local tmpfile="/tmp/gov-prop-${_GOV_PROP_IDX}.json"

    log_info "  [gov] submit proposal: ${title}"
    local proposal_json
    proposal_json=$(cat <<PEOF
{"messages":[],"metadata":"text","deposit":"${GOV_MIN_DEPOSIT_AMOUNT}${BASIC_DENOM}","title":"${title}","summary":"${summary}"}
PEOF
    )

    local prop_id
    prop_id=$(_gov_submit_proposal "$proposal_json" "$tmpfile") || true

    if [ -n "$prop_id" ]; then
        for ((v = 0; v < NUM_VALIDATORS; v++)); do
            cosmos_tx_on "$v" gov vote "$prop_id" yes --from "validator${v}"
        done
    fi
}

_gov_verify_proposals_exist() {
    local count
    count=$(exec_mocad query gov proposals \
        --node tcp://localhost:26657 --chain-id "${CHAIN_ID}" \
        --output json 2>/dev/null | jq '.proposals | length') || true
    assert_gt "$count" "0" "Proposals should exist post-upgrade"
}

_gov_verify_submit_works() {
    local pre_count
    pre_count=$(exec_mocad query gov proposals \
        --node tcp://localhost:26657 --chain-id "${CHAIN_ID}" \
        --output json 2>/dev/null | jq '.proposals | length') || true
    pre_count="${pre_count:-0}"
    log_info "  [gov] pre_count=${pre_count}"

    # Query the actual min deposit from chain params (may differ across versions)
    local min_deposit
    min_deposit=$(exec_mocad query gov params \
        --node tcp://localhost:26657 --chain-id "${CHAIN_ID}" \
        --output json 2>/dev/null | jq -r '
            .params.min_deposit[0].amount //
            .deposit_params.min_deposit[0].amount //
            empty' 2>/dev/null) || true
    min_deposit="${min_deposit:-${GOV_MIN_DEPOSIT_AMOUNT}}"
    local denom
    denom=$(exec_mocad query gov params \
        --node tcp://localhost:26657 --chain-id "${CHAIN_ID}" \
        --output json 2>/dev/null | jq -r '
            .params.min_deposit[0].denom //
            .deposit_params.min_deposit[0].denom //
            empty' 2>/dev/null) || true
    denom="${denom:-${BASIC_DENOM}}"
    log_info "  [gov] using deposit=${min_deposit}${denom}"

    local prop_json="{\"messages\":[],\"metadata\":\"text\",\"deposit\":\"${min_deposit}${denom}\",\"title\":\"E2E Post-Upgrade Test\",\"summary\":\"Post-upgrade proposal\"}"
    local tmpfile="/tmp/gov-prop-verify.json"

    local new_id
    new_id=$(_gov_submit_proposal "$prop_json" "$tmpfile") || true

    local post_count
    post_count=$(exec_mocad query gov proposals \
        --node tcp://localhost:26657 --chain-id "${CHAIN_ID}" \
        --output json 2>/dev/null | jq '.proposals | length') || true
    post_count="${post_count:-0}"
    log_info "  [gov] post_count=${post_count} new_id=${new_id:-none}"
    assert_gt "$post_count" "$pre_count" "Post-upgrade proposal submission should work"
}

register_tx     gov_tx
register_verify "Proposals preserved"                _gov_verify_proposals_exist
register_verify "Post-upgrade proposal submission"   _gov_verify_submit_works
