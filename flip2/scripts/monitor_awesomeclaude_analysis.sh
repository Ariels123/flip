#!/bin/bash
# Monitor Awesome Claude Analysis Tasks
# Usage: ./scripts/monitor_awesomeclaude_analysis.sh

DB_PATH="pb_data/data.db"

echo "======================================"
echo "Awesome Claude Analysis - Task Monitor"
echo "======================================"
echo ""

# Check if database exists
if [ ! -f "$DB_PATH" ]; then
    echo "Error: Database not found at $DB_PATH"
    exit 1
fi

# Display task status
echo "📊 TASK STATUS"
echo "----------------------------------------"
sqlite3 "$DB_PATH" << 'SQL'
.mode column
.headers on
SELECT
  substr(task_id, 1, 35) as task,
  assignee,
  status,
  priority,
  progress || '%' as progress
FROM tasks
WHERE task_id LIKE '%awesomeclaude%'
ORDER BY task_id;
SQL

echo ""
echo "📈 PROGRESS SUMMARY"
echo "----------------------------------------"

# Count by status
TOTAL=$(sqlite3 "$DB_PATH" "SELECT COUNT(*) FROM tasks WHERE task_id LIKE '%awesomeclaude%';")
PENDING=$(sqlite3 "$DB_PATH" "SELECT COUNT(*) FROM tasks WHERE task_id LIKE '%awesomeclaude%' AND status = 'pending';")
IN_PROGRESS=$(sqlite3 "$DB_PATH" "SELECT COUNT(*) FROM tasks WHERE task_id LIKE '%awesomeclaude%' AND status = 'in_progress';")
DONE=$(sqlite3 "$DB_PATH" "SELECT COUNT(*) FROM tasks WHERE task_id LIKE '%awesomeclaude%' AND status = 'done';")
FAILED=$(sqlite3 "$DB_PATH" "SELECT COUNT(*) FROM tasks WHERE task_id LIKE '%awesomeclaude%' AND status = 'failed';")

echo "Total Tasks: $TOTAL"
echo "  Pending: $PENDING"
echo "  In Progress: $IN_PROGRESS"
echo "  Done: $DONE"
echo "  Failed: $FAILED"
echo ""

# Show completed tasks
if [ "$DONE" -gt 0 ]; then
    echo "✅ COMPLETED TASKS"
    echo "----------------------------------------"
    sqlite3 "$DB_PATH" << 'SQL'
.mode column
.headers on
SELECT
  substr(task_id, 1, 35) as task,
  assignee,
  substr(completed_at, 1, 19) as completed
FROM tasks
WHERE task_id LIKE '%awesomeclaude%' AND status = 'done'
ORDER BY completed_at;
SQL
    echo ""
fi

# Show next ready task
echo "🎯 NEXT ACTION"
echo "----------------------------------------"
READY=$(sqlite3 "$DB_PATH" "SELECT task_id FROM tasks WHERE task_id LIKE '%awesomeclaude%' AND status = 'pending' AND (depends_on IS NULL OR depends_on = '[]') LIMIT 1;")
if [ -n "$READY" ]; then
    echo "Ready to execute: $READY"
    ASSIGNEE=$(sqlite3 "$DB_PATH" "SELECT assignee FROM tasks WHERE task_id = '$READY';")
    echo "Agent: $ASSIGNEE"
else
    echo "Waiting on dependencies..."
fi

echo ""
echo "======================================"
echo "Last updated: $(date)"
echo "======================================"
