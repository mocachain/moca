#!/bin/bash
# Goroutine monitoring script

LOG_FILE="goroutine_monitor_$(date +%Y%m%d_%H%M%S).log"
METRICS_URL="http://127.0.0.1:26660/metrics"
ALERT_THRESHOLD=10000

echo "==== Goroutine Monitoring Started ====" | tee -a $LOG_FILE
echo "Time: $(date)" | tee -a $LOG_FILE
echo "Alert Threshold: $ALERT_THRESHOLD" | tee -a $LOG_FILE
echo "================================" | tee -a $LOG_FILE

while true; do
    TIMESTAMP=$(date '+%Y-%m-%d %H:%M:%S')
    GOROUTINES=$(curl -s $METRICS_URL | grep '^go_goroutines' | awk '{print $2}')
    
    if [ -z "$GOROUTINES" ]; then
        echo "[$TIMESTAMP] ERROR: Failed to fetch data" | tee -a $LOG_FILE
    else
        echo "[$TIMESTAMP] Goroutines: $GOROUTINES" | tee -a $LOG_FILE
        
        if [ "$GOROUTINES" -gt "$ALERT_THRESHOLD" ]; then
            echo "[$TIMESTAMP] WARNING: Exceeds threshold!" | tee -a $LOG_FILE
        fi
    fi
    
    sleep 30  # Check every 30 seconds
done
