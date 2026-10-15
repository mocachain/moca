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

test_cast_send_retry
if [ "$failures" -ne 0 ]; then
    log_error "${failures} assertion(s) failed"
    exit 1
fi
log_success "all lib.sh tests passed"
