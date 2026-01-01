#!/bin/bash
# Sync Monitor for FLIP2
# Tests Mac ↔ Windows connectivity and sync status
#
# Cron entry (run every 1 minute after Windows is deployed):
# * * * * * /Users/arielspivakovsky/src/flip/flip2/scripts/sync_monitor.sh

LOG_DIR="/tmp/flip2_monitoring"
LOG_FILE="$LOG_DIR/sync_$(date +%Y%m%d).log"

MAC_ENDPOINT="https://localhost:8090/api/health"
WIN_ENDPOINT="http://192.168.1.220:8090/api/health"

mkdir -p "$LOG_DIR"

TIMESTAMP=$(date '+%Y-%m-%d %H:%M:%S')

echo "[$TIMESTAMP] === Sync Connectivity Check ===" >> "$LOG_FILE"

# Test Mac (local)
MAC_HEALTH=$(curl -sk "$MAC_ENDPOINT" 2>&1)
MAC_CODE=$?

if [ $MAC_CODE -eq 0 ]; then
    echo "[$TIMESTAMP] Mac: ✓ OK" >> "$LOG_FILE"
    MAC_OK=1
else
    echo "[$TIMESTAMP] Mac: ✗ FAILED (code: $MAC_CODE)" >> "$LOG_FILE"
    MAC_OK=0
fi

# Test Windows (remote)
WIN_HEALTH=$(curl -s -m 5 "$WIN_ENDPOINT" 2>&1)
WIN_CODE=$?

if [ $WIN_CODE -eq 0 ]; then
    echo "[$TIMESTAMP] Windows: ✓ OK" >> "$LOG_FILE"
    WIN_OK=1
else
    echo "[$TIMESTAMP] Windows: ✗ FAILED (code: $WIN_CODE)" >> "$LOG_FILE"
    WIN_OK=0
fi

# Calculate success rate
STATS_FILE="$LOG_DIR/sync_stats.txt"
if [ ! -f "$STATS_FILE" ]; then
    echo "0 0 0" > "$STATS_FILE"
fi

read TOTAL SUCCESS FAIL < "$STATS_FILE"
TOTAL=$((TOTAL + 1))

if [ $MAC_OK -eq 1 ] && [ $WIN_OK -eq 1 ]; then
    SUCCESS=$((SUCCESS + 1))
    echo "[$TIMESTAMP] Sync Status: ✓ BOTH ONLINE" >> "$LOG_FILE"
else
    FAIL=$((FAIL + 1))
    echo "[$TIMESTAMP] Sync Status: ✗ DEGRADED" >> "$LOG_FILE"
fi

# Update stats
echo "$TOTAL $SUCCESS $FAIL" > "$STATS_FILE"

# Calculate success rate
if [ $TOTAL -gt 0 ]; then
    SUCCESS_PCT=$((SUCCESS * 100 / TOTAL))
    echo "[$TIMESTAMP] Success Rate: ${SUCCESS_PCT}% ($SUCCESS/$TOTAL)" >> "$LOG_FILE"
fi

echo "" >> "$LOG_FILE"
