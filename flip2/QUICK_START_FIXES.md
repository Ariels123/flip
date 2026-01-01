# FLIP2 Quick Start: Critical Fixes

**Priority 0 fixes to deploy in next 24 hours**

---

## Fix #1: Add Environment Validation (15 minutes)

**File:** `internal/daemon/daemon.go`

**Location:** In `Start()` function, replace lines 103-113

**Replace this:**
```go
// Environment safeguard - log prominently which environment we're in
env := os.Getenv("FLIP2_ENV")
if env == "" {
    env = "production" // Default to production for safety
}
d.logger.Info("========================================")
d.logger.Info("FLIP2 DAEMON STARTING",
    "environment", env,
    "pid", os.Getpid(),
    "port", d.config.Flip2.PocketBase.Port)
d.logger.Info("========================================")
```

**With this:**
```go
// Environment safeguard - VALIDATE environment vs port
env := os.Getenv("FLIP2_ENV")
if env == "" {
    env = "production" // Default to production
}
port := d.config.Flip2.PocketBase.Port

// SAFETY: Reject invalid environment/port combinations
if env == "production" && port >= 9000 && port < 10000 {
    return fmt.Errorf("SAFETY ABORT: Production environment cannot run on test port %d (9xxx range is for testing only)", port)
}
if env == "test" && port >= 8000 && port < 9000 {
    return fmt.Errorf("SAFETY ABORT: Test environment cannot run on production port %d (8xxx range is for production only)", port)
}
if env == "production" && d.config.Flip2.PocketBase.DataDir == "./pb_data_test" {
    return fmt.Errorf("SAFETY ABORT: Production cannot use test data directory")
}
if env == "test" && d.config.Flip2.PocketBase.DataDir == "./pb_data" {
    return fmt.Errorf("SAFETY ABORT: Test cannot use production data directory")
}

// Log prominently
d.logger.Info("==========================================")
d.logger.Info("🚀 FLIP2 DAEMON STARTING - VALIDATED",
    "environment", strings.ToUpper(env),
    "port", port,
    "data_dir", d.config.Flip2.PocketBase.DataDir,
    "pid", os.Getpid())
d.logger.Info("==========================================")
```

**Test:**
```bash
# Should start normally
FLIP2_ENV=production ./flip2d --config config/config.yaml

# Should ABORT with error
FLIP2_ENV=test ./flip2d --config config/config.yaml  # Wrong: test env on prod port
FLIP2_ENV=production ./flip2d --config config/config-test.yaml  # Wrong: prod env on test port
```

---

## Fix #2: Add Log Cleanup Jobs (20 minutes)

**File:** `internal/daemon/daemon.go`

**Location:** In `registerJobs()` function, add these new jobs AFTER the zombie-reaper job

**Add this code:**
```go
// Log Cleanup - Run every 8 hours, delete logs older than 48 hours
d.scheduler.RegisterJob("log-cleanup", "0 0 */8 * * *", func(ctx context.Context) error {
    cutoffTime := time.Now().Add(-48 * time.Hour).Format("2006-01-02 15:04:05")

    result, err := d.pb.DB().NewQuery(
        "DELETE FROM _logs WHERE created < {:cutoff}",
    ).Bind(dbx.Params{
        "cutoff": cutoffTime,
    }).Execute()

    if err != nil {
        d.logger.Error("Log cleanup failed", "error", err)
        return err
    }

    rowsAffected, _ := result.RowsAffected()
    d.logger.Info("Log cleanup completed", "deleted", rowsAffected, "cutoff", cutoffTime)
    return nil
})

// Pattern-based cleanup - Delete repetitive polling logs (keep last 100)
d.scheduler.RegisterJob("log-pattern-cleanup", "0 0 4,12,20 * * *", func(ctx context.Context) error {
    patterns := []string{
        "GET /api/collections/signals/records?filter=(to_agent='WinPc-AG'&&read=false)",
        "GET /api/collections/signals/records?filter=to_agent='claude-mac' && read=false&perPage=50",
        "GET /api/collections/signals/records?filter=(to_agent='Claud-win'+&&+read=false)",
        "GET /api/realtime",
    }

    totalDeleted := int64(0)
    for _, pattern := range patterns {
        result, err := d.pb.DB().NewQuery(`
            DELETE FROM _logs
            WHERE message = {:pattern}
            AND id NOT IN (
                SELECT id FROM _logs
                WHERE message = {:pattern}
                ORDER BY created DESC
                LIMIT 100
            )
        `).Bind(dbx.Params{
            "pattern": pattern,
        }).Execute()

        if err != nil {
            d.logger.Warn("Pattern cleanup failed", "pattern", pattern, "error", err)
            continue
        }

        rows, _ := result.RowsAffected()
        totalDeleted += rows
    }

    d.logger.Info("Log pattern cleanup completed", "deleted", totalDeleted)
    return nil
})

// VACUUM database - Run weekly on Sunday at 3 AM
d.scheduler.RegisterJob("db-vacuum", "0 0 3 * * 0", func(ctx context.Context) error {
    d.logger.Info("Starting database VACUUM (may take several minutes)")
    startTime := time.Now()

    _, err := d.pb.DB().NewQuery("VACUUM").Execute()
    duration := time.Since(startTime)

    if err != nil {
        d.logger.Error("VACUUM failed", "error", err, "duration", duration)
        return err
    }

    d.logger.Info("Database VACUUM completed", "duration", duration)
    return nil
})
```

**Add import at top of file:**
```go
import (
    // ... existing imports ...
    "github.com/pocketbase/dbx"  // ADD THIS
)
```

**Manual cleanup (run once now):**
```bash
# Stop daemon first
pkill flip2d

# Clean up logs manually
sqlite3 pb_data/auxiliary.db "DELETE FROM _logs WHERE created < datetime('now', '-48 hours')"
sqlite3 pb_data/auxiliary.db "VACUUM"

# Check size reduction
ls -lh pb_data/auxiliary.db

# Restart daemon with new jobs
./flip2d --config config/config.yaml &
```

---

## Fix #3: Create Test Config (10 minutes)

**File:** `config/config-test.yaml` (create new file)

```yaml
flip2:
  daemon:
    pid_file: /tmp/flip2d-test.pid
    log_file: /tmp/flip2d-test.log
    log_level: debug

  pocketbase:
    host: 0.0.0.0
    port: 9190  # TEST PORT
    data_dir: ./pb_data_test
    tls:
      enabled: false

  security:
    api_keys_enabled: true
    api_key: flip2_test_key_123
    jwt_secret: flip2_test_jwt_456

  sync:
    enabled: false  # Don't sync test to production!

  archiver:
    enabled: false  # No archiving in test

  executor:
    max_concurrent_tasks: 1
    default_timeout: 60s

  metrics:
    enabled: true
```

**Test startup script:** `scripts/start_test_server.sh`

```bash
#!/bin/bash
export FLIP2_ENV=test
./flip2d --config config/config-test.yaml --foreground
```

**Make executable:**
```bash
chmod +x scripts/start_test_server.sh
```

**Test:**
```bash
# Create test data directory
mkdir -p pb_data_test

# Start test server
FLIP2_ENV=test ./flip2d --config config/config-test.yaml

# Should start on port 9190
# Should create collections via bootstrap
# Should ABORT if you try: FLIP2_ENV=production with this config
```

---

## Fix #4: Immediate Deployment to Windows

**Run on Mac:**

```bash
#!/bin/bash
# scripts/deploy_windows_emergency.sh

set -e
cd /Users/arielspivakovsky/src/flip/flip2

echo "=== Emergency Windows Deployment ==="

# 1. Build latest with all fixes
echo "Building Windows binary..."
GOOS=windows GOARCH=amd64 go build -o flip2d-win.exe ./cmd/flip2d

# 2. Create Windows config
echo "Creating Windows config..."
cat > config-win-prod.yaml << 'EOF'
flip2:
  daemon:
    pid_file: C:\flip2\flip2d.pid
    log_file: C:\flip2\flip2d.log
    log_level: info

  pocketbase:
    host: 0.0.0.0
    port: 8090
    data_dir: C:\flip2\pb_data

  security:
    api_keys_enabled: true
    api_key: flip2_secret_key_123
    jwt_secret: flip2_jwt_secret_key_456
    bootstrap_api_key: flip2_bootstrap_key_789

  sync:
    enabled: true
    node_id: windows
    sync_interval: 15s
    peers:
      - id: mac
        url: http://192.168.1.53:8090
        api_key: flip2_secret_key_123
        enabled: true

  archiver:
    enabled: true
    active_retention_days: 3
    recent_retention_days: 90
    check_interval: 6h
    batch_size: 200
    archive_path: C:\flip2\archives\signals
    active_agents:
      - claude-mac
      - claude-win
    deprecated_agents:
      - gemini
      - claude
EOF

# 3. Stop Windows daemon (if running)
echo "Stopping Windows daemon..."
ssh Agnizar@192.168.1.220 'taskkill /F /IM flip2d.exe 2>nul || echo "Not running"'

# 4. Deploy files
echo "Deploying to Windows..."
scp flip2d-win.exe Agnizar@192.168.1.220:C:/flip2/flip2d-new.exe
scp config-win-prod.yaml Agnizar@192.168.1.220:C:/flip2/config.yaml

# 5. Create pb_data directory
echo "Creating pb_data directory..."
ssh Agnizar@192.168.1.220 'mkdir C:\flip2\pb_data 2>nul || echo "Already exists"'

# 6. Backup old binary, install new
echo "Installing new binary..."
ssh Agnizar@192.168.1.220 'copy C:\flip2\flip2d.exe C:\flip2\flip2d-backup.exe 2>nul || echo "No old binary"'
ssh Agnizar@192.168.1.220 'copy C:\flip2\flip2d-new.exe C:\flip2\flip2d.exe'

# 7. Start daemon
echo "Starting Windows daemon..."
ssh Agnizar@192.168.1.220 'cd C:\flip2 && set FLIP2_ENV=production && start /B flip2d.exe --config config.yaml'

sleep 5

# 8. Verify
echo "Verifying Windows daemon..."
curl -s http://192.168.1.220:8090/api/health && echo "✓ Windows daemon healthy" || echo "✗ Windows daemon not responding"

echo ""
echo "=== Deployment Complete ==="
echo "Monitor logs: ssh Agnizar@192.168.1.220 'type C:\flip2\flip2d.log'"
```

**Make executable and run:**
```bash
chmod +x scripts/deploy_windows_emergency.sh
./scripts/deploy_windows_emergency.sh
```

---

## Fix #5: Pre-Work Safety Checklist

**File:** `scripts/pre_work_checklist.sh`

```bash
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
PROD_PORT=$(lsof -ti :8090 | wc -l | xargs)
TEST_PORT=$(lsof -ti :9190 | wc -l | xargs)
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
```

**Make executable:**
```bash
chmod +x scripts/pre_work_checklist.sh
```

**Add to your workflow:**
```bash
# ALWAYS run this before working on FLIP2
./scripts/pre_work_checklist.sh
```

---

## Testing the Fixes

### 1. Test Environment Validation

```bash
# These should work:
FLIP2_ENV=production ./flip2d --config config/config.yaml
FLIP2_ENV=test ./flip2d --config config/config-test.yaml

# These should ABORT with error:
FLIP2_ENV=test ./flip2d --config config/config.yaml  # Test on prod port
FLIP2_ENV=production ./flip2d --config config/config-test.yaml  # Prod on test port
```

### 2. Test Log Cleanup (Manual)

```bash
# Before
sqlite3 pb_data/auxiliary.db "SELECT COUNT(*) FROM _logs"
# Should show ~490K

# Run cleanup
sqlite3 pb_data/auxiliary.db "DELETE FROM _logs WHERE created < datetime('now', '-48 hours')"
sqlite3 pb_data/auxiliary.db "VACUUM"

# After
sqlite3 pb_data/auxiliary.db "SELECT COUNT(*) FROM _logs"
# Should show much fewer

# Check size
ls -lh pb_data/auxiliary.db
# Should be <10MB
```

### 3. Test Scheduled Jobs

```bash
# Check logs after daemon runs for a few hours
tail -f /tmp/flip2d.log | grep -E "(log-cleanup|pattern-cleanup|vacuum)"

# Should see:
# "Log cleanup completed" - every 8 hours
# "Log pattern cleanup completed" - 3 times/day
# "Database VACUUM completed" - weekly
```

### 4. Verify Windows Deployment

```bash
# Check Windows is running
curl http://192.168.1.220:8090/api/health

# Check Windows has signals collection
curl -H "X-API-Key: flip2_secret_key_123" \
  http://192.168.1.220:8090/api/collections/signals/records?perPage=1

# Send test signal from Mac
curl -X POST http://localhost:8090/api/collections/signals/records \
  -H "X-API-Key: flip2_secret_key_123" \
  -d '{"signal_id":"test-sync-001","from_agent":"mac","to_agent":"windows","content":"Test sync"}'

# Wait 30 seconds for sync
sleep 30

# Verify on Windows
curl -H "X-API-Key: flip2_secret_key_123" \
  "http://192.168.1.220:8090/api/collections/signals/records?filter=signal_id='test-sync-001'"
```

---

## Build & Deploy Commands

```bash
# 1. Build Mac binary with fixes
go build -o flip2d ./cmd/flip2d

# 2. Build Windows binary
GOOS=windows GOARCH=amd64 go build -o flip2d-win.exe ./cmd/flip2d

# 3. Stop old daemon
pkill flip2d

# 4. Backup old binary
cp flip2d flip2d.backup

# 5. Start new daemon
FLIP2_ENV=production ./flip2d --config config/config.yaml &

# 6. Verify
curl http://localhost:8090/api/health
tail -f /tmp/flip2d.log
```

---

## Rollback Procedure

If something goes wrong:

```bash
# Mac rollback
pkill flip2d
cp flip2d.backup flip2d
FLIP2_ENV=production ./flip2d --config config/config.yaml &

# Windows rollback
ssh Agnizar@192.168.1.220 'taskkill /F /IM flip2d.exe'
ssh Agnizar@192.168.1.220 'copy C:\flip2\flip2d-backup.exe C:\flip2\flip2d.exe'
ssh Agnizar@192.168.1.220 'cd C:\flip2 && set FLIP2_ENV=production && start /B flip2d.exe --config config.yaml'
```

---

## Expected Results

After deploying all fixes:

✅ Daemon refuses to start with wrong environment/port combination
✅ Logs cleaned up every 8 hours (48h retention)
✅ Polling logs cleaned aggressively (keep last 100)
✅ Database VACUUMed weekly
✅ auxiliary.db stays <10MB
✅ Windows has pb_data and bootstrap works
✅ Sync works Mac ↔ Windows
✅ Test server can run on port 9190 without conflicts

---

**Time to implement all fixes: ~90 minutes**

**Priority order:**
1. Fix #2 (Log cleanup) - 20 min - PREVENTS DISK EXHAUSTION
2. Fix #1 (Environment validation) - 15 min - PREVENTS INCIDENT RECURRENCE
3. Fix #4 (Windows deployment) - 30 min - FIXES BROKEN SYNC
4. Fix #3 (Test config) - 10 min - ENABLES SAFE TESTING
5. Fix #5 (Safety checklist) - 5 min - WORKFLOW IMPROVEMENT

Start with Fix #2 to prevent immediate disk space issues.
