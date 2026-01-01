#!/bin/bash
# Database Size Monitor for FLIP2
# Tracks database growth and alerts if auxiliary.db exceeds 10MB
#
# Cron entry (run every 30 minutes):
# */30 * * * * /Users/arielspivakovsky/src/flip/flip2/scripts/db_size_monitor.sh

LOG_DIR="/tmp/flip2_monitoring"
LOG_FILE="$LOG_DIR/db_size_$(date +%Y%m%d).log"
DATA_DIR="/Users/arielspivakovsky/src/flip/flip2/pb_data"

mkdir -p "$LOG_DIR"

TIMESTAMP=$(date '+%Y-%m-%d %H:%M:%S')

echo "[$TIMESTAMP] === Database Size Check ===" >> "$LOG_FILE"

# Check data.db
if [ -f "$DATA_DIR/data.db" ]; then
    SIZE_DATA=$(ls -lh "$DATA_DIR/data.db" | awk '{print $5}')
    SIZE_DATA_BYTES=$(stat -f%z "$DATA_DIR/data.db" 2>/dev/null || stat -c%s "$DATA_DIR/data.db")
    echo "[$TIMESTAMP] data.db: $SIZE_DATA ($SIZE_DATA_BYTES bytes)" >> "$LOG_FILE"
else
    echo "[$TIMESTAMP] data.db: NOT FOUND" >> "$LOG_FILE"
fi

# Check auxiliary.db (should stay <10MB)
if [ -f "$DATA_DIR/auxiliary.db" ]; then
    SIZE_AUX=$(ls -lh "$DATA_DIR/auxiliary.db" | awk '{print $5}')
    SIZE_AUX_BYTES=$(stat -f%z "$DATA_DIR/auxiliary.db" 2>/dev/null || stat -c%s "$DATA_DIR/auxiliary.db")
    SIZE_AUX_MB=$((SIZE_AUX_BYTES / 1024 / 1024))

    echo "[$TIMESTAMP] auxiliary.db: $SIZE_AUX ($SIZE_AUX_BYTES bytes = ${SIZE_AUX_MB}MB)" >> "$LOG_FILE"

    # Alert if >10MB
    if [ $SIZE_AUX_MB -gt 10 ]; then
        echo "[$TIMESTAMP] ⚠️  WARNING: auxiliary.db exceeds 10MB!" >> "$LOG_FILE"
    fi
else
    echo "[$TIMESTAMP] auxiliary.db: NOT FOUND" >> "$LOG_FILE"
fi

# Calculate growth rate (compare with last entry)
PREV_SIZE=$(tail -20 "$LOG_FILE" | grep "data.db:" | tail -2 | head -1 | grep -o '[0-9]* bytes' | awk '{print $1}')
if [ -n "$PREV_SIZE" ] && [ -n "$SIZE_DATA_BYTES" ]; then
    GROWTH=$((SIZE_DATA_BYTES - PREV_SIZE))
    if [ $GROWTH -gt 0 ]; then
        GROWTH_MB=$((GROWTH / 1024 / 1024))
        echo "[$TIMESTAMP] Growth since last check: +${GROWTH_MB}MB" >> "$LOG_FILE"
    fi
fi

echo "" >> "$LOG_FILE"
