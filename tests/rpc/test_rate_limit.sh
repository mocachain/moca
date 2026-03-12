#!/bin/bash
# Rate limit test

RPC_URL="http://127.0.0.1:8545"
REQUESTS=30  # Send 30 requests (exceeds rate limit)

echo "==== Rate Limit Test ===="
echo "Goal: Verify rate limiting triggers after exceeding configured req/s"
echo "Sending: $REQUESTS requests"
echo "================================"

SUCCESS=0
RATE_LIMITED=0
OTHER_ERROR=0

for i in $(seq 1 $REQUESTS); do
    RESPONSE=$(curl -s -w "\n%{http_code}" -X POST $RPC_URL \
        -H "Content-Type: application/json" \
        -d '{
            "jsonrpc": "2.0",
            "method": "eth_getLogs",
            "params": [{
                "fromBlock": "latest",
                "toBlock": "latest"
            }],
            "id": 1
        }')
    
    HTTP_CODE=$(echo "$RESPONSE" | tail -1)
    BODY=$(echo "$RESPONSE" | head -n -1)
    
    if echo "$BODY" | grep -qi "rate limit"; then
        echo "[$i] Rate Limited"
        ((RATE_LIMITED++))
    elif echo "$BODY" | grep -q '"result"'; then
        echo "[$i] Success"
        ((SUCCESS++))
    else
        echo "[$i] Other Error"
        ((OTHER_ERROR++))
    fi
    
    # Send requests quickly (about 20 per second)
    sleep 0.05
done

echo "================================"
echo "Results:"
echo "  Success: $SUCCESS"
echo "  Rate Limited: $RATE_LIMITED"
echo "  Other Errors: $OTHER_ERROR"
echo "================================"

if [ $RATE_LIMITED -gt 0 ]; then
    echo "Rate limiting is working correctly"
    exit 0
else
    echo "Rate limiting not triggered (possible issue)"
    exit 1
fi
