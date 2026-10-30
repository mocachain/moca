#!/usr/bin/env bash
# Unit tests for helpers in lib.sh that can run without a cluster.
# Run: bash e2e/kind/scripts/lib_test.sh (or make e2e-scripts-test)
set -euo pipefail

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
# shellcheck source=/dev/null
source "${SCRIPT_DIR}/lib.sh"

failures=0
assert_eq() { # expected actual label
    if [ "$1" != "$2" ]; then
        log_error "FAIL $3: expected '$1', got '$2'"
        failures=$((failures + 1))
    else
        log_success "ok   $3"
    fi
}
assert_contains() { # needle haystack label
    if printf '%s' "$2" | grep -q -- "$1"; then
        log_success "ok   $3"
    else
        log_error "FAIL $3: '$2' does not contain '$1'"
        failures=$((failures + 1))
    fi
}

# Stub `cast`: answers the null-response error for the first STUB_NULL_FAILS
# calls, then a non-null error when STUB_HARD_FAIL=1, otherwise a success JSON.
stub_dir=$(mktemp -d)
trap 'rm -rf "$stub_dir"' EXIT
cat > "${stub_dir}/cast" <<'STUB'
#!/usr/bin/env bash
n=$(cat "$STUB_COUNT_FILE" 2>/dev/null || echo 0)
n=$((n + 1))
echo "$n" > "$STUB_COUNT_FILE"
if [ "$n" -le "${STUB_NULL_FAILS:-0}" ]; then
    echo '{"errors":[{"code":"cast.error","message":"server returned a null response when a non-null response was expected"}]}'
    exit 1
fi
if [ "${STUB_HARD_FAIL:-0}" = "1" ]; then
    echo '{"errors":[{"code":"cast.error","message":"insufficient funds for gas * price + value"}]}'
    exit 1
fi
echo '{"transactionHash":"0xabc"}'
STUB
chmod +x "${stub_dir}/cast"
export PATH="${stub_dir}:${PATH}"
export CAST_SEND_RETRY_SLEEP=0

# run_case <name> <null-fails> <hard-fail>: sets out, rc and calls.
run_case() {
    export STUB_COUNT_FILE="${stub_dir}/count-$1" STUB_NULL_FAILS="$2" STUB_HARD_FAIL="$3"
    rm -f "$STUB_COUNT_FILE"
    set +e
    out=$(cast_send_retry 0xrecipient --value 1 --json 2>/dev/null)
    rc=$?
    set -e
    calls=$(cat "$STUB_COUNT_FILE")
}

test_cast_send_retry() {
    run_case first-try 0 0
    assert_eq 0 "$rc" "success on the first attempt returns cast's exit code"
    assert_contains '"transactionHash":"0xabc"' "$out" "success on the first attempt passes cast's output through"
    assert_eq 1 "$calls" "success on the first attempt calls cast once"

    run_case null-then-ok 2 0
    assert_eq 0 "$rc" "null responses are retried until cast succeeds"
    assert_contains '"transactionHash":"0xabc"' "$out" "the successful output is returned after retries"
    assert_eq 3 "$calls" "two null responses cost exactly two retries"

    run_case hard-fail 0 1
    assert_eq 1 "$rc" "a non-null failure is returned as a failure"
    assert_contains 'insufficient funds' "$out" "a non-null failure passes cast's output through"
    assert_eq 1 "$calls" "a non-null failure is not retried"

    run_case exhausted 9 0
    assert_eq 1 "$rc" "persistent null responses still fail"
    assert_contains 'null response' "$out" "the last null response is returned when retries are exhausted"
    assert_eq 5 "$calls" "retries are bounded at five attempts"
}

# assert_gt <actual> <threshold> -> rc (no output)
gt_rc() {
    if assert_gt "$@" >/dev/null 2>&1; then echo 0; else echo 1; fi
}

test_assert_gt() {
    assert_eq 0 "$(gt_rc 5 3)" "assert_gt passes when above the threshold"
    assert_eq 1 "$(gt_rc 3 5)" "assert_gt fails when below the threshold"
    assert_eq 1 "$(gt_rc 0 0)" "assert_gt fails on equality"
    assert_eq 0 "$(gt_rc 0.5 0)" "assert_gt compares decimals"
    assert_eq 0 "$(gt_rc '1000000000000000000000000 [1e24]' 0)" "assert_gt reads cast's formatted uint (number and [exponent])"
    assert_eq 1 "$(gt_rc '100 [1e2]' 100)" "assert_gt compares the number of a formatted uint, not its suffix"
    assert_eq 0 "$(gt_rc '  7  ' 3)" "assert_gt ignores surrounding whitespace"
    assert_eq 1 "$(gt_rc '' 0)" "assert_gt fails on an empty value"
    assert_eq 1 "$(gt_rc null 0)" "assert_gt fails on a non-number"
}

# Stub `kubectl`: logs its arguments, prints the file in STUB_KUBECTL_OUT, exits STUB_KUBECTL_RC.
cat > "${stub_dir}/kubectl" <<'STUB'
#!/usr/bin/env bash
printf '%s\n' "$*" >> "$STUB_KUBECTL_LOG"
if [ -n "${STUB_KUBECTL_OUT:-}" ]; then cat "$STUB_KUBECTL_OUT"; fi
exit "${STUB_KUBECTL_RC:-0}"
STUB
chmod +x "${stub_dir}/kubectl"
export STUB_KUBECTL_LOG="${stub_dir}/kubectl.log"

test_cosmos_broadcast_fees() {
    printf 'gas estimate: 123456\n{"txhash":"ABC","code":0}\n' > "${stub_dir}/broadcast.out"
    export STUB_KUBECTL_OUT="${stub_dir}/broadcast.out" STUB_KUBECTL_RC=0
    local out args
    : > "$STUB_KUBECTL_LOG"
    out=$(cosmos_broadcast validator-0-0 tx bank send a b 1amoca --from a 2>"${stub_dir}/broadcast.err")
    args=$(cat "$STUB_KUBECTL_LOG")
    assert_eq '{"txhash":"ABC","code":0}' "$out" "cosmos_broadcast returns just the broadcast JSON"
    assert_eq 'gas estimate: 123456' "$(cat "${stub_dir}/broadcast.err")" "cosmos_broadcast logs the gas estimate on stderr"
    assert_contains '--gas auto --gas-adjustment' "$args" "cosmos_broadcast simulates gas"
    assert_contains "--fees ${COSMOS_TX_FEES} " "$args" "cosmos_broadcast passes the default --fees"
    assert_contains '--fees [0-9]\{1,\}amoca ' "$args" "the default fee is an amoca amount"
    if printf '%s' "$args" | grep -q -- '--gas-prices'; then
        log_error "FAIL cosmos_broadcast must not pass --gas-prices with --gas auto"
        failures=$((failures + 1))
    else
        log_success "ok   cosmos_broadcast does not pass --gas-prices with --gas auto"
    fi
    COSMOS_TX_FEES=42amoca
    : > "$STUB_KUBECTL_LOG"
    cosmos_broadcast validator-0-0 tx bank send a b 1amoca --from a >/dev/null 2>&1
    assert_contains '--fees 42amoca ' "$(cat "$STUB_KUBECTL_LOG")" "COSMOS_TX_FEES overrides the fee"
}

test_bank_native_balance() {
    local out rc
    bal_case() { # <json> <kubectl rc>: sets out and rc
        printf '%s' "$1" > "${stub_dir}/balance.out"
        export STUB_KUBECTL_OUT="${stub_dir}/balance.out" STUB_KUBECTL_RC="$2"
        set +e
        out=$(bank_native_balance 0xabc 2>/dev/null)
        rc=$?
        set -e
    }
    bal_case '{"balances":[{"denom":"other","amount":"1"},{"denom":"amoca","amount":"42"}]}' 0
    assert_eq 0 "$rc" "bank_native_balance succeeds when the denom is held"
    assert_eq 42 "$out" "bank_native_balance prints the native amount"
    bal_case '{"balances":[],"pagination":{"total":"0"}}' 0
    assert_eq 1 "$rc" "bank_native_balance fails for an account with no balances"
    bal_case '{"balances":[{"denom":"other","amount":"1"}]}' 0
    assert_eq 1 "$rc" "bank_native_balance fails when only other denoms are held"
    bal_case '' 1
    assert_eq 1 "$rc" "bank_native_balance fails when the query fails"
    bal_case 'rpc error' 0
    assert_eq 1 "$rc" "bank_native_balance fails on a non-JSON answer"
}

# Run a throwaway suite (stdin) against framework.sh in a child shell; sets suite_out and suite_rc.
run_suite() { # <suite body>
    set +e
    suite_out=$(bash -c 'source "$1/../framework/framework.sh"; _fw_collect_debug_logs() { :; }; fw_init
        t_ok() { return 0; }
        t_bad() { assert_gt 1 2 "one is not above two"; echo MID_TEST_CONTINUED; }
        '"$1" _ "$SCRIPT_DIR" 2>&1)
    suite_rc=$?
    set -e
}

test_framework_failure_accounting() {
    run_suite 'fw_run_test "first" t_ok; fw_run_test "second" t_bad; fw_run_test "third" t_ok; echo THIRD_RAN; fw_done'
    assert_eq 1 "$suite_rc" "a failing test fails the job"
    assert_contains 'Passed: 1' "$suite_out" "the test before the failure is counted as passed"
    assert_contains 'Failed: 1' "$suite_out" "the failing test is counted as failed"
    assert_contains 'Test Summary' "$suite_out" "the summary is printed on failure"
    assert_contains 'one is not above two' "$suite_out" "the assertion message is shown"
    if printf '%s' "$suite_out" | grep -q 'MID_TEST_CONTINUED'; then
        log_error "FAIL errexit must stop a test at its first failed assertion"
        failures=$((failures + 1))
    else
        log_success "ok   errexit stops a test at its first failed assertion"
    fi

    run_suite 'fw_run_test "first" t_ok; fw_run_test "second" t_ok; fw_done'
    assert_eq 0 "$suite_rc" "passing tests pass the job"
    assert_contains 'Passed: 2' "$suite_out" "all passing tests are counted"
    assert_contains 'Failed: 0' "$suite_out" "no failures are recorded"
    assert_contains 'All tests passed' "$suite_out" "success is reported"
}

test_cast_send_retry
test_assert_gt
test_cosmos_broadcast_fees
test_bank_native_balance
test_framework_failure_accounting
if [ "$failures" -ne 0 ]; then
    log_error "${failures} assertion(s) failed"
    exit 1
fi
log_success "all lib.sh tests passed"
