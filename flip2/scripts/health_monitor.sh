#!/bin/bash
# Health Monitor for FLIP2
# Checks daemon health, process status, and memory usage every 5 minutes
#
# Cron entry (run every 5 minutes):
# */5 * * * * /Users/arielspivakovsky/src/flip/flip2/scripts/health_monitor.sh

LOG_DIR="/tmp/flip2_monitoring"
LOG_FILE="$LOG_DIR/health_$(date +%Y%m%d).log"
ENDPOINT="https://localhost:8090/api/health"

mkdir -p "$LOG_DIR"

TIMESTAMP=$(date '+%Y-%m-%d %H:%M:%S')

echo "[$TIMESTAMP] === Health Check ===" >> "$LOG_FILE"

# Check health endpoint
HEALTH=$(curl -sk "$ENDPOINT" 2>&1)
HEALTH_CODE=$?

if [ $HEALTH_CODE -eq 0 ]; then
    echo "[$TIMESTAMP] Health: OK - $HEALTH" >> "$LOG_FILE"
else
    echo "[$TIMESTAMP] Health: FAILED (curl code: $HEALTH_CODE)" >> "$LOG_FILE"
fi

# Check process status
PROCESS=$(ps aux | grep "[f]lip2d" | head -1)
if [ -n "$PROCESS" ]; then
    PID=$(echo "$PROCESS" | awk '{print $2}')
    MEM=$(echo "$PROCESS" | awk '{print $6}')
    CPU=$(echo "$PROCESS" | awk '{print $3}')
    echo "[$TIMESTAMP] Process: RUNNING (PID: $PID, MEM: ${MEM}KB, CPU: ${CPU}%)" >> "$LOG_FILE"
else
    echo "[$TIMESTAMP] Process: NOT FOUND - DAEMON DOWN!" >> "$LOG_FILE"
fi

# Check PID file
if [ -f /tmp/flip2d.pid ]; then
    PID_FILE=$(cat /tmp/flip2d.pid)
    echo "[$TIMESTAMP] PID File: $PID_FILE" >> "$LOG_FILE"
fi

echo "" >> "$LOG_FILE"
