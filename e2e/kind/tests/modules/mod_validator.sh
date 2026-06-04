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
    local n i
    local poll_interval="${VALIDATOR_SYNC_POLL_INTERVAL:-2}"
    local max_wait="${VALIDATOR_SYNC_MAX_WAIT:-30}"
    n=$(_validator_count)

    for ((i = 0; i < n; i++)); do
        local deadline catching_up status
        deadline=$(($(date +%s) + max_wait))

        while true; do
            status=$(kind_fetch_rpc_status "$i" 2>/dev/null || echo "{}")
            catching_up=$(echo "$status" | jq -r '.result.sync_info.catching_up' 2>/dev/null || echo "true")

            if [ "$catching_up" = "false" ]; then
                break
            fi

            if [ "$(date +%s)" -ge "$deadline" ]; then
                local height
                height=$(echo "$status" | jq -r '.result.sync_info.latest_block_height // "0"' 2>/dev/null || echo "0")
                log_error "validator-${i} still catching up after ${max_wait}s (height: ${height})"
                return 1
            fi

            sleep "$poll_interval"
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
    local -a initial_heights
    n=$(_validator_count)
    for ((i = 0; i < n; i++)); do
        initial_heights[$i]=$(get_block_height_for_validator_index "$i")
    done

    sleep 4

    for ((i = 0; i < n; i++)); do
        local h2
        h2=$(get_block_height_for_validator_index "$i")
        assert_gt "$h2" "${initial_heights[$i]}" "validator-${i} block height should increase"
    done
}

_validator_test_latest_commit_signed_by_all() {
    local rpc_url block_json commit_height validators_json
    local expected_count signed_count missing=0
    rpc_url="${COMETBFT_RPC_URL:-http://localhost:26657}"

    block_json=$(curl -sf "${rpc_url}/block" 2>/dev/null || echo "")
    assert_not_empty "$block_json" "latest block should be queryable from ${rpc_url}"

    commit_height=$(echo "$block_json" | jq -r '.result.block.last_commit.height // "0"' 2>/dev/null || echo "0")
    assert_gt "$commit_height" "0" "latest block should include a non-zero last_commit height"

    validators_json=$(curl -sf "${rpc_url}/validators?height=${commit_height}&per_page=100" 2>/dev/null || echo "")
    assert_not_empty "$validators_json" "validator set should be queryable at height ${commit_height}"

    expected_count=$(echo "$validators_json" | jq -r '.result.validators | length // 0' 2>/dev/null || echo "0")
    assert_eq "$expected_count" "$(_validator_count)" "validator set size at height ${commit_height} should match NUM_VALIDATORS"

    signed_count=$(echo "$block_json" | jq -r '
        [.result.block.last_commit.signatures[]?
         | select(.block_id_flag == 2 and (.validator_address // "") != "")
         | .validator_address] | unique | length
    ' 2>/dev/null || echo "0")
    assert_eq "$signed_count" "$expected_count" "latest last_commit should contain signatures from all validators"

    while IFS= read -r addr; do
        [ -z "$addr" ] && continue
        if ! echo "$block_json" | jq -e --arg addr "$addr" '
            any(.result.block.last_commit.signatures[]?;
                .block_id_flag == 2 and .validator_address == $addr)
        ' >/dev/null 2>&1; then
            log_error "validator address ${addr} is missing from last_commit signatures at height ${commit_height}"
            missing=1
        fi
    done < <(echo "$validators_json" | jq -r '.result.validators[]?.address' 2>/dev/null || true)

    [ "$missing" -eq 0 ]
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
register_verify "Latest commit signed by all validators"      _validator_test_latest_commit_signed_by_all
register_verify "Validator voting power healthy (all pods)"  _validator_test_voting_power
register_verify "Validator block production (all pods)"       _validator_test_block_production
register_verify "Validator heights consistent (spread <= 2)"  _validator_test_heights_consistent
register_verify "On-chain staking validator count matches cluster" _validator_test_on_chain_validator_count
