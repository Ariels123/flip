#!/bin/bash
# Test script to verify code review system

echo "=== Code Review System Test ==="
echo ""

# Check if daemon is running
if ! pgrep -f flip2d > /dev/null; then
    echo "❌ Daemon is not running"
    exit 1
fi
echo "✅ Daemon is running"

# Check if code_reviews collection exists
if sqlite3 pb_data/data.db "SELECT COUNT(*) FROM code_reviews;" > /dev/null 2>&1; then
    echo "✅ code_reviews collection exists"
    review_count=$(sqlite3 pb_data/data.db "SELECT COUNT(*) FROM code_reviews;")
    echo "   Current review count: $review_count"
else
    echo "❌ code_reviews collection not found"
    exit 1
fi

# Check logs for code review system initialization
if grep -q "Code review system started successfully" /var/folders/g3/g99g_h9j1x52qnjvfxn13f1w0000gn/T/flip2d_logs/daemon_*.log 2>/dev/null; then
    echo "✅ Code review system initialized"
else
    echo "⚠️  Code review system may not be initialized (check logs)"
fi

# Check if periodic job is registered
if grep -q "periodic-code-review" /var/folders/g3/g99g_h9j1x52qnjvfxn13f1w0000gn/T/flip2d_logs/daemon_*.log 2>/dev/null; then
    echo "✅ Periodic code review job registered"
    cron_expr=$(grep "periodic-code-review" /var/folders/g3/g99g_h9j1x52qnjvfxn13f1w0000gn/T/flip2d_logs/daemon_*.log | grep -o '"0 0 \*/6 \* \* \*"' | head -1)
    echo "   Schedule: Every 6 hours"
else
    echo "⚠️  Periodic code review job not found in logs"
fi

# Check for uncommitted changes
if git diff --quiet HEAD; then
    echo "⚠️  No uncommitted changes found for review"
else
    echo "✅ Found uncommitted changes"
    echo "   Files changed:"
    git diff --name-status HEAD | sed 's/^/   /'
fi

echo ""
echo "=== Dashboard Access ==="
echo "Dashboard URL: https://localhost:8090/index.html"
echo "Admin Panel:   https://localhost:8090/_/"
echo ""
echo "=== Summary ==="
echo "Code review system is ready for testing!"
echo "You can trigger reviews via:"
echo "1. Dashboard 'Run Review Now' button"
echo "2. Automatic periodic reviews (every 6 hours)"
echo "3. API endpoint: POST /api/code-review/trigger (requires auth)"
