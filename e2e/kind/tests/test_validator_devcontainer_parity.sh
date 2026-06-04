#!/usr/bin/env bash
# Parity with moca-devcontainer: test/validator/check-validators.sh test
# For each validator pod: Running, RPC /status, not catching_up, voting_power > 0,
# then block production monitoring (CHECK_INTERVAL, MAX_WAIT, MIN_BLOCKS — same defaults as devcontainer).

# shellcheck source=/dev/null
source "$(dirname "$0")/../framework/framework.sh"
fw_init

fw_start_chain

test_all_validators_devcontainer_parity() {
    local num="${NUM_VALIDATORS:-4}"
    local i
    local failed=0

    for ((i = 0; i < num; i++)); do
        if ! kind_test_validator_block_production "$i"; then
            failed=1
        fi
        echo ""
    done

    return "$failed"
}

fw_run_test "Devcontainer validator parity (per-pod RPC + block production)" test_all_validators_devcontainer_parity
fw_done
