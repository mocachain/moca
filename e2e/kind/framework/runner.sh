#!/usr/bin/env bash
# Test runner: discovers and executes all tests/test_*.sh files.

set -euo pipefail

RUNNER_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
TESTS_DIR="${RUNNER_DIR}/../tests"

TOTAL=0
PASSED=0
FAILED=0
FAILED_TESTS=()

echo "=== Moca E2E Test Runner ==="
echo ""

for test_file in "${TESTS_DIR}"/test_*.sh; do
    [ -f "$test_file" ] || continue
    test_name=$(basename "$test_file" .sh)
    TOTAL=$((TOTAL + 1))

    echo ">>> Running: ${test_name}"
    if bash "$test_file"; then
        echo ">>> ${test_name}: PASSED"
        PASSED=$((PASSED + 1))
    else
        echo ">>> ${test_name}: FAILED"
        FAILED=$((FAILED + 1))
        FAILED_TESTS+=("$test_name")
    fi
    echo ""
done

echo "=== Runner Summary ==="
echo "  Total:  ${TOTAL}"
echo "  Passed: ${PASSED}"
echo "  Failed: ${FAILED}"

if [ ${FAILED} -gt 0 ]; then
    echo "  Failed tests: ${FAILED_TESTS[*]}"
    exit 1
fi
echo "All tests passed!"
