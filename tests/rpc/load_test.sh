#!/bin/bash
# Load stress test

RPC_URL="http://127.0.0.1:8545"
METRICS_URL="http://127.0.0.1:26660/metrics"
DURATION=${DURATION:-3600}  # Default 1 hour, can be overridden via environment variable
LOG_FILE="load_test_$(date +%Y%m%d_%H%M%S).log"

echo "==== Load Stress Test ====" | tee -a $LOG_FILE
echo "Start time: $(date)" | tee -a $LOG_FILE
echo "Duration: $DURATION seconds ($(($DURATION/60)) minutes)" | tee -a $LOG_FILE
echo "================================" | tee -a $LOG_FILE

START_TIME=$(date +%s)
END_TIME=$((START_TIME + DURATION))
REQUEST_COUNT=0
SUCCESS_COUNT=0
ERROR_COUNT=0
RATE_LIMITED=0

GOROUTINES_START=$(curl -s $METRICS_URL | grep '^go_goroutines' | awk '{print $2}')
echo "Initial Goroutines: $GOROUTINES_START" | tee -a $LOG_FILE

while [ $(date +%s) -lt $END_TIME ]; do
    ((REQUEST_COUNT++))
    
    # Randomly select query range (simulate real scenario)
    RANGE=$((RANDOM % 50 + 1))
    FROM_BLOCK=$((RANDOM % 1000))
    TO_BLOCK=$((FROM_BLOCK + RANGE))
    
    RESPONSE=$(curl -s -X POST $RPC_URL \
        -H "Content-Type: application/json" \
        -d "{
            \"jsonrpc\": \"2.0\",
            \"method\": \"eth_getLogs\",
            \"params\": [{
                \"fromBlock\": \"0x$(printf '%x' $FROM_BLOCK)\",
                \"toBlock\": \"0x$(printf '%x' $TO_BLOCK)\"
            }],
            \"id\": 1
        }")
    
    if echo "$RESPONSE" | grep -q '"result"'; then
        ((SUCCESS_COUNT++))
    elif echo "$RESPONSE" | grep -qi "rate limit"; then
        ((RATE_LIMITED++))
    else
        ((ERROR_COUNT++))
    fi
    
    # Log status every 100 requests
    if [ $((REQUEST_COUNT % 100)) -eq 0 ]; then
        CURRENT_GOROUTINES=$(curl -s $METRICS_URL | grep '^go_goroutines' | awk '{print $2}')
        ELAPSED=$(($(date +%s) - START_TIME))
        echo "[${ELAPSED}s] Requests:$REQUEST_COUNT Success:$SUCCESS_COUNT RateLimited:$RATE_LIMITED Errors:$ERROR_COUNT Goroutines:$CURRENT_GOROUTINES" | tee -a $LOG_FILE
    fi
    
    # Simulate real load (average 2-3 requests per second)
    sleep $(awk 'BEGIN{srand(); print 0.3+rand()*0.2}')
done

GOROUTINES_END=$(curl -s $METRICS_URL | grep '^go_goroutines' | awk '{print $2}')
SUCCESS_RATE=$(awk "BEGIN {printf \"%.2f\", $SUCCESS_COUNT*100/$REQUEST_COUNT}")

echo "================================" | tee -a $LOG_FILE
echo "Load test completed - $(date)" | tee -a $LOG_FILE
echo "Total requests: $REQUEST_COUNT" | tee -a $LOG_FILE
echo "Success: $SUCCESS_COUNT ($SUCCESS_RATE%)" | tee -a $LOG_FILE
echo "Rate limited: $RATE_LIMITED" | tee -a $LOG_FILE
echo "Errors: $ERROR_COUNT" | tee -a $LOG_FILE
echo "Start Goroutines: $GOROUTINES_START" | tee -a $LOG_FILE
echo "End Goroutines: $GOROUTINES_END" | tee -a $LOG_FILE
echo "Difference: $((GOROUTINES_END - GOROUTINES_START))" | tee -a $LOG_FILE
echo "================================" | tee -a $LOG_FILE

if [ $((GOROUTINES_END - GOROUTINES_START)) -gt 1000 ]; then
    echo "Goroutine leak detected!" | tee -a $LOG_FILE
    exit 1
else
    echo "No goroutine leak detected" | tee -a $LOG_FILE
    exit 0
fi
