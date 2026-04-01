#!/usr/bin/env bash
# Module: validator health checks (all validators — parity with moca-devcontainer check-validators.sh).

_validator_count() {
    echo "${NUM_VALIDATORS:-4}"
}

_validator_test_rpc_accessible() {
    local n i
    n=$(_validator_count)
    for ((i = 0; i < n; i++)); do
        local status node_id
        status=$(kind_fetch_rpc_status "$i") || return 1
        node_id=$(echo "$status" | jq -r '.result.node_info.id // empty' 2>/dev/null || true)
        assert_not_empty "$node_id" "validator-${i} RPC should return node id"
    done
}

_validator_test_sync_status() {
    local n i catching_up deadline stable_false_samples sync_poll_interval
    n=$(_validator_count)
    stable_false_samples="${VALIDATOR_SYNC_STABLE_SAMPLES:-3}"
    sync_poll_interval="${VALIDATOR_SYNC_POLL_INTERVAL:-5}"
    for ((i = 0; i < n; i++)); do
        local consecutive_false=0
        local initial_height=0
        local latest_height=0
        deadline=$(($(date +%s) + ${VALIDATOR_SYNC_MAX_WAIT:-120}))
        initial_height=$(get_block_height_for_validator_index "$i")
        latest_height="$initial_height"
        while true; do
            local status
            status=$(kind_fetch_rpc_status "$i" 2>/dev/null || echo "{}")
            catching_up=$(echo "$status" | jq -r '.result.sync_info.catching_up // "true"' 2>/dev/null || echo "true")
            latest_height=$(echo "$status" | jq -r '.result.sync_info.latest_block_height // "0"' 2>/dev/null || echo "0")
            if [ "$catching_up" = "false" ]; then
                consecutive_false=$((consecutive_false + 1))
                if [ "$consecutive_false" -ge "$stable_false_samples" ]; then
                    break
                fi
            else
                consecutive_false=0
            fi
            if [ "$(date +%s)" -ge "$deadline" ]; then
                if [ "$latest_height" -gt "$initial_height" ] 2>/dev/null; then
                    log_warn "validator-${i} still reports catching_up=true, but height advanced (${initial_height} -> ${latest_height}); continuing"
                    break
                fi
                assert_eq "$catching_up" "false" "validator-${i} should not be catching up"
                return 1
            fi
            sleep "$sync_poll_interval"
        done
    done
}

_validator_test_voting_power() {
    local n i vp
    n=$(_validator_count)
    for ((i = 0; i < n; i++)); do
        vp=$(kind_fetch_rpc_status "$i" | jq -r '.result.validator_info.voting_power // "0"' 2>/dev/null || echo "0")
        assert_gt "$vp" "0" "validator-${i} validator_info.voting_power should be > 0"
    done

    local validators_json total
    validators_json=$(exec_mocad query staking validators --node tcp://localhost:26657 --output json 2>/dev/null || echo "{}")
    total=$(echo "$validators_json" | jq -r '.validators | length // 0' 2>/dev/null || echo "0")
    assert_gt "$total" "0" "staking should list at least one validator"
}

_validator_test_block_production() {
    local n i
    n=$(_validator_count)
    for ((i = 0; i < n; i++)); do
        local h1 h2
        h1=$(get_block_height_for_validator_index "$i")
        sleep 4
        h2=$(get_block_height_for_validator_index "$i")
        assert_gt "$h2" "$h1" "validator-${i} block height should increase"
    done
}

# Parity with moca-devcontainer upgrade verify-validators: height spread across validators.
_validator_test_heights_consistent() {
    local n i h min_h max_h diff
    n=$(_validator_count)
    min_h=$(get_block_height_for_validator_index 0)
    max_h=$min_h
    for ((i = 1; i < n; i++)); do
        h=$(get_block_height_for_validator_index "$i")
        if [ "$h" -lt "$min_h" ] 2>/dev/null; then
            min_h=$h
        fi
        if [ "$h" -gt "$max_h" ] 2>/dev/null; then
            max_h=$h
        fi
    done
    diff=$((max_h - min_h))
    if [ "$diff" -gt 2 ]; then
        log_error "validator height spread ${diff} exceeds max 2 (min ${min_h} max ${max_h})"
        return 1
    fi
    return 0
}

# Parity with verify_validator_on_chain: bonded validator count matches cluster size.
_validator_test_on_chain_validator_count() {
    local validators_json total expected
    expected=$(_validator_count)
    validators_json=$(exec_mocad query staking validators \
        --node tcp://localhost:26657 --output json 2>/dev/null || echo "{}")
    total=$(echo "$validators_json" | jq -r '.pagination.total // empty' 2>/dev/null || echo "")
    if [ -z "$total" ] || [ "$total" = "null" ]; then
        total=$(echo "$validators_json" | jq -r '.validators | length // 0' 2>/dev/null || echo "0")
    fi
    assert_eq "$total" "$expected" "staking validators count should equal NUM_VALIDATORS"
}

register_verify "Validator RPC accessible (all pods)"      _validator_test_rpc_accessible
register_verify "Validator sync status healthy (all pods)" _validator_test_sync_status
register_verify "Validator voting power healthy (all pods)"  _validator_test_voting_power
register_verify "Validator block production (all pods)"       _validator_test_block_production
register_verify "Validator heights consistent (spread <= 2)"  _validator_test_heights_consistent
register_verify "On-chain staking validator count matches cluster" _validator_test_on_chain_validator_count
