#!/bin/bash
# Complete test suite

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RESULTS_DIR="test_results_$(date +%Y%m%d_%H%M%S)"
mkdir -p $RESULTS_DIR

echo "===================================="
echo "  RPC Goroutine Leak Fix Test Suite"
echo "===================================="
echo "Start time: $(date)"
echo ""

# Test 1: Rate limit
echo ">>> Test 1/4: Rate Limit Verification"
if bash $SCRIPT_DIR/test_rate_limit.sh > $RESULTS_DIR/rate_limit.log 2>&1; then
    echo "Rate limit test passed"
else
    echo "Rate limit test failed"
fi
echo ""

# Test 2: Query timeout
echo ">>> Test 2/4: Query Timeout Verification"
if bash $SCRIPT_DIR/test_query_timeout.sh > $RESULTS_DIR/query_timeout.log 2>&1; then
    echo "Query timeout test passed"
else
    echo "Query timeout test failed"
fi
echo ""

# Test 3: Context cancellation
echo ">>> Test 3/4: Context Cancellation Verification"
if bash $SCRIPT_DIR/test_context_cancel.sh > $RESULTS_DIR/context_cancel.log 2>&1; then
    echo "Context cancellation test passed"
else
    echo "Context cancellation test failed"
fi
echo ""

# Test 4: Short load test (10 minutes)
echo ">>> Test 4/4: Short Load Test (10 minutes)"
DURATION=600 bash $SCRIPT_DIR/load_test.sh > $RESULTS_DIR/load_test.log 2>&1 &
LOAD_PID=$!
echo "Load test PID: $LOAD_PID"
wait $LOAD_PID
if [ $? -eq 0 ]; then
    echo "Load test passed"
else
    echo "Load test failed"
fi
echo ""

echo "===================================="
echo "Tests completed: $(date)"
echo "Results saved in: $RESULTS_DIR/"
echo ""
echo "View detailed results:"
echo "  cat $RESULTS_DIR/*.log"
echo "===================================="
