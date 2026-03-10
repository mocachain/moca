#!/bin/bash
# Context cancellation test

RPC_URL="http://127.0.0.1:8545"
METRICS_URL="http://127.0.0.1:26660/metrics"

echo "==== Context Cancellation Test ===="
echo "Goal: Verify queries are cancelled when client disconnects"
echo "================================"

# Record goroutine count before test
GOROUTINES_BEFORE=$(curl -s $METRICS_URL | grep '^go_goroutines' | awk '{print $2}')
echo "Goroutines before test: $GOROUTINES_BEFORE"

# Start long query and force interrupt after 5 seconds
(
    timeout -s KILL 5 curl -X POST $RPC_URL \
        -H "Content-Type: application/json" \
        -d '{
            "jsonrpc": "2.0",
            "method": "eth_getLogs",
            "params": [{
                "fromBlock": "0x1",
                "toBlock": "0x7D0"
            }],
            "id": 1
        }' 2>/dev/null
) &

CURL_PID=$!
echo "Query process PID: $CURL_PID"
echo "Will force terminate after 5 seconds..."

sleep 5
wait $CURL_PID 2>/dev/null
echo "Query interrupted"

# Wait for server to cleanup goroutines
echo "Waiting 15 seconds for server to cleanup goroutines..."
sleep 15

# Check goroutine count
GOROUTINES_AFTER=$(curl -s $METRICS_URL | grep '^go_goroutines' | awk '{print $2}')
echo "Goroutines after test: $GOROUTINES_AFTER"

DIFF=$((GOROUTINES_AFTER - GOROUTINES_BEFORE))
echo "Goroutine difference: $DIFF"

echo "================================"
if [ $DIFF -le 50 ]; then
    echo "Context cancellation working correctly"
    exit 0
else
    echo "Goroutines not properly cleaned up (possible leak)"
    exit 1
fi
