#!/bin/bash
# Query timeout test

RPC_URL="http://127.0.0.1:8545"
METRICS_URL="http://127.0.0.1:26660/metrics"

echo "==== Query Timeout Test ===="
echo "Goal: Verify large range queries timeout after 30 seconds"
echo "================================"

# Record goroutine count before test
GOROUTINES_BEFORE=$(curl -s $METRICS_URL | grep '^go_goroutines' | awk '{print $2}')
echo "Goroutines before test: $GOROUTINES_BEFORE"

START_TIME=$(date +%s)
echo "Start time: $(date)"

# Send large range query (fromBlock=1 toBlock=2000)
RESPONSE=$(timeout 35 curl -s -X POST $RPC_URL \
    -H "Content-Type: application/json" \
    -d '{
        "jsonrpc": "2.0",
        "method": "eth_getLogs",
        "params": [{
            "fromBlock": "0x1",
            "toBlock": "0x7D0"
        }],
        "id": 1
    }')

END_TIME=$(date +%s)
DURATION=$((END_TIME - START_TIME))

echo "End time: $(date)"
echo "Query duration: ${DURATION} seconds"

# Check response
if echo "$RESPONSE" | grep -qi "context deadline exceeded\|query cancelled\|timeout"; then
    echo "Query timed out correctly"
    TIMEOUT_OK=1
elif [ $DURATION -ge 30 ] && [ $DURATION -le 32 ]; then
    echo "Query completed within timeout period"
    TIMEOUT_OK=1
else
    echo "Query behavior abnormal (duration: ${DURATION}s)"
    TIMEOUT_OK=0
fi

# Wait for goroutine cleanup
echo "Waiting 10 seconds for goroutine cleanup..."
sleep 10

# Check goroutine count after test
GOROUTINES_AFTER=$(curl -s $METRICS_URL | grep '^go_goroutines' | awk '{print $2}')
echo "Goroutines after test: $GOROUTINES_AFTER"

DIFF=$((GOROUTINES_AFTER - GOROUTINES_BEFORE))
echo "Goroutine difference: $DIFF"

echo "================================"
if [ $DIFF -le 100 ] && [ $TIMEOUT_OK -eq 1 ]; then
    echo "Timeout and cleanup mechanism working correctly"
    exit 0
else
    echo "Test failed"
    exit 1
fi
