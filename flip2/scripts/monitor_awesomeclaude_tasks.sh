#!/bin/bash
# Monitor Awesome Claude Analysis Tasks
# Runs every 180 seconds (3 minutes) to check progress
# Alerts every 5 minutes if tasks stuck due to token limits

DB_PATH="pb_data/data.db"
LOG_FILE="logs/task_monitor.log"
LAST_TOKEN_CHECK="/tmp/last_token_check"

mkdir -p logs

echo "======================================" | tee -a "$LOG_FILE"
echo "Awesome Claude Task Monitor - $(date)" | tee -a "$LOG_FILE"
echo "======================================" | tee -a "$LOG_FILE"
echo "" | tee -a "$LOG_FILE"

# Check task status
echo "📊 TASK STATUS" | tee -a "$LOG_FILE"
sqlite3 "$DB_PATH" << 'SQL' | tee -a "$LOG_FILE"
.mode column
.headers on
SELECT
  task_id,
  status,
  progress || '%' as progress,
  CASE
    WHEN status = 'in_progress' THEN datetime(started_at)
    WHEN status = 'done' THEN datetime(completed_at)
    ELSE '-'
  END as timestamp
FROM tasks
WHERE task_id LIKE '%awesomeclaude%'
ORDER BY task_id;
SQL

echo "" | tee -a "$LOG_FILE"

# Count by status
TOTAL=$(sqlite3 "$DB_PATH" "SELECT COUNT(*) FROM tasks WHERE task_id LIKE '%awesomeclaude%';")
PENDING=$(sqlite3 "$DB_PATH" "SELECT COUNT(*) FROM tasks WHERE task_id LIKE '%awesomeclaude%' AND status = 'pending';")
IN_PROGRESS=$(sqlite3 "$DB_PATH" "SELECT COUNT(*) FROM tasks WHERE task_id LIKE '%awesomeclaude%' AND status = 'in_progress';")
DONE=$(sqlite3 "$DB_PATH" "SELECT COUNT(*) FROM tasks WHERE task_id LIKE '%awesomeclaude%' AND status = 'done';")
FAILED=$(sqlite3 "$DB_PATH" "SELECT COUNT(*) FROM tasks WHERE task_id LIKE '%awesomeclaude%' AND status = 'failed';")

echo "📈 SUMMARY" | tee -a "$LOG_FILE"
echo "Total: $TOTAL | Pending: $PENDING | Running: $IN_PROGRESS | Done: $DONE | Failed: $FAILED" | tee -a "$LOG_FILE"
echo "" | tee -a "$LOG_FILE"

# Check for stuck tasks every 5 minutes
CURRENT_TIME=$(date +%s)
if [ -f "$LAST_TOKEN_CHECK" ]; then
    LAST_CHECK=$(cat "$LAST_TOKEN_CHECK")
    TIME_DIFF=$((CURRENT_TIME - LAST_CHECK))
    if [ $TIME_DIFF -ge 300 ]; then
        # It's been 5+ minutes, check for stuck tasks
        echo "🔍 TOKEN LIMIT CHECK (5-minute interval)" | tee -a "$LOG_FILE"

        # Check for tasks in_progress for more than 15 minutes
        STUCK_TASKS=$(sqlite3 "$DB_PATH" "SELECT COUNT(*) FROM tasks WHERE task_id LIKE '%awesomeclaude%' AND status = 'in_progress' AND julianday('now') - julianday(started_at) > 0.010417;")  # 15 minutes

        if [ "$STUCK_TASKS" -gt 0 ]; then
            echo "⚠️  WARNING: $STUCK_TASKS task(s) have been in_progress for >15 minutes" | tee -a "$LOG_FILE"
            echo "   Possible causes: token limits, errors, or long-running operations" | tee -a "$LOG_FILE"

            # Show stuck tasks
            sqlite3 "$DB_PATH" << 'SQL' | tee -a "$LOG_FILE"
SELECT task_id,
       CAST((julianday('now') - julianday(started_at)) * 24 * 60 AS INTEGER) || ' mins' as duration
FROM tasks
WHERE task_id LIKE '%awesomeclaude%'
  AND status = 'in_progress'
  AND julianday('now') - julianday(started_at) > 0.010417
ORDER BY started_at;
SQL
        else
            echo "✅ No stuck tasks detected" | tee -a "$LOG_FILE"
        fi

        echo "$CURRENT_TIME" > "$LAST_TOKEN_CHECK"
        echo "" | tee -a "$LOG_FILE"
    fi
else
    echo "$CURRENT_TIME" > "$LAST_TOKEN_CHECK"
fi

# Show recently completed tasks
if [ "$DONE" -gt 0 ]; then
    echo "✅ COMPLETED TASKS" | tee -a "$LOG_FILE"
    sqlite3 "$DB_PATH" << 'SQL' | tee -a "$LOG_FILE"
.mode column
SELECT task_id, datetime(completed_at) as completed
FROM tasks
WHERE task_id LIKE '%awesomeclaude%' AND status = 'done'
ORDER BY completed_at DESC;
SQL
    echo "" | tee -a "$LOG_FILE"
fi

# Show failed tasks with errors
if [ "$FAILED" -gt 0 ]; then
    echo "❌ FAILED TASKS" | tee -a "$LOG_FILE"
    sqlite3 "$DB_PATH" << 'SQL' | tee -a "$LOG_FILE"
.mode column
SELECT task_id, substr(result, 1, 100) as error_preview
FROM tasks
WHERE task_id LIKE '%awesomeclaude%' AND status = 'failed';
SQL
    echo "" | tee -a "$LOG_FILE"
fi

echo "======================================" | tee -a "$LOG_FILE"
echo "Next check in 180 seconds (3 minutes)" | tee -a "$LOG_FILE"
echo "======================================" | tee -a "$LOG_FILE"
echo "" | tee -a "$LOG_FILE"
