#!/bin/bash
# Run this BEFORE any FLIP2 work

echo "╔═══════════════════════════════════════════════════════╗"
echo "║  FLIP2 PRE-WORK SAFETY CHECKLIST                     ║"
echo "╚═══════════════════════════════════════════════════════╝"
echo ""

# 1. Environment
if [ -z "$FLIP2_ENV" ]; then
    echo "⚠️  WARNING: FLIP2_ENV not set (defaulting to production)"
    export FLIP2_ENV=production
else
    echo "✓ FLIP2_ENV: $FLIP2_ENV"
fi

# 2. Running processes
echo ""
echo "Running processes:"
if pgrep -f flip2d > /dev/null; then
    ps aux | grep flip2d | grep -v grep
else
    echo "  None"
fi

# 3. Port usage
echo ""
echo "Port usage:"
PROD_PORT=$(lsof -ti :8090 2>/dev/null | wc -l | xargs)
TEST_PORT=$(lsof -ti :9190 2>/dev/null | wc -l | xargs)
echo "  Port 8090 (Production): $PROD_PORT process(es)"
echo "  Port 9190 (Test):       $TEST_PORT process(es)"

if [ "$FLIP2_ENV" = "production" ] && [ "$TEST_PORT" -gt 0 ]; then
    echo "  ⚠️  WARNING: Test port in use while FLIP2_ENV=production"
fi

if [ "$FLIP2_ENV" = "test" ] && [ "$PROD_PORT" -gt 0 ]; then
    echo "  ⚠️  WARNING: Production port in use while FLIP2_ENV=test"
fi

# 4. Database sizes
echo ""
echo "Database sizes:"
[ -f pb_data/data.db ] && echo "  pb_data/data.db:      $(du -h pb_data/data.db | cut -f1)"
[ -f pb_data/auxiliary.db ] && echo "  pb_data/auxiliary.db: $(du -h pb_data/auxiliary.db | cut -f1)"
[ -f pb_data_test/data.db ] && echo "  pb_data_test/data.db: $(du -h pb_data_test/data.db | cut -f1)"

# Warn if auxiliary.db is huge
if [ -f pb_data/auxiliary.db ]; then
    SIZE=$(du -m pb_data/auxiliary.db | cut -f1)
    if [ "$SIZE" -gt 50 ]; then
        echo "  ⚠️  WARNING: auxiliary.db is ${SIZE}MB (should be <10MB)"
        echo "     Run: sqlite3 pb_data/auxiliary.db 'DELETE FROM _logs WHERE created < datetime(\"now\", \"-48 hours\"); VACUUM;'"
    fi
fi

# 5. Config validation
echo ""
if [ -f config/config.yaml ]; then
    PORT=$(grep "port:" config/config.yaml | head -1 | awk '{print $2}')
    DATA_DIR=$(grep "data_dir:" config/config.yaml | head -1 | awk '{print $2}')
    echo "Config: config/config.yaml"
    echo "  Port:     $PORT"
    echo "  Data Dir: $DATA_DIR"

    # Validate
    if [ "$FLIP2_ENV" = "production" ] && [ "$PORT" -ge 9000 ]; then
        echo "  ❌ ERROR: Production env with test port $PORT"
        echo ""
        echo "ABORT: Fix environment or config before proceeding."
        exit 1
    fi

    if [ "$FLIP2_ENV" = "test" ] && [ "$PORT" -lt 9000 ]; then
        echo "  ❌ ERROR: Test env with production port $PORT"
        echo ""
        echo "ABORT: Fix environment or config before proceeding."
        exit 1
    fi

    echo "  ✓ Port matches environment"
fi

echo ""
echo "╔═══════════════════════════════════════════════════════╗"
echo "║  CHECKLIST COMPLETE - SAFE TO PROCEED                ║"
echo "╚═══════════════════════════════════════════════════════╝"
