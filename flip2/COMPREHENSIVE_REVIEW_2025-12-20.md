# FLIP2 Comprehensive Security & Reliability Review

**Date:** 2025-12-20
**Reviewer:** Claude (Sonnet 4.5)
**Scope:** Post-incident architectural review and recommendations

---

## Executive Summary

The FLIP2 multi-agent coordination system experienced a critical incident where production and test environments were confused, leading to unintended modifications of production servers. This review identifies **7 critical risks**, **12 architectural vulnerabilities**, and provides **28 actionable recommendations** organized by priority.

### Key Findings

**CRITICAL (Fix Immediately):**
1. **No runtime environment validation** - System allows production configs on test ports
2. **Disk exhaustion risk** - 337MB auxiliary.db (489K log entries), growing ~50K/day
3. **Port confusion** - Mac production on 8090 but config says 8091, inconsistent with Windows
4. **Missing bootstrap safeguards** - Windows deployment directory has no pb_data
5. **Polling storm** - 129K+ identical polling queries logged (wasted resources)

**HIGH Priority:**
1. Mac-Windows code drift (31 vs 23 Go files)
2. No test infrastructure on Windows
3. Sync mechanism untested end-to-end
4. Log patterns indicate inefficient polling

**Architecture Strengths:**
- Vector clock-based conflict resolution is solid
- PocketBase integration provides good foundation
- Separation of concerns (daemon, sync, archiver)
- Bootstrap collection creation (recently added)

---

## 1. INCIDENT ANALYSIS

### What Happened (2025-12-20)

**Timeline:**
1. Intended to work on **test server** (port 9190)
2. Actually modified **production server** (ports 8090/8091)
3. Violated Development Rule #1: "Test before production"
4. Root cause: No verification of environment before work
5. Discovered when checking which server was running

### Root Causes

1. **No Runtime Environment Checks**
   - Code does not validate FLIP2_ENV vs port
   - No startup warnings about port/env mismatches
   - Easy to accidentally run wrong config

2. **Port Convention Confusion**
   ```
   Mac config.yaml:     port: 8090  (but should be 8091 per docs?)
   Windows expected:    port: 8090
   Test ports:          9190/9191 (not actively used)
   ```

3. **Manual Process**
   - Relied on human to check `ps aux` before starting
   - No automated safeguards
   - Easy to skip verification steps

4. **Inadequate Visual Indicators**
   - Log files don't prominently show environment
   - PID files same name pattern
   - No banner on startup showing PROD vs TEST

---

## 2. CRITICAL RISKS IDENTIFIED

### Risk #1: Disk Exhaustion from Logs

**Severity:** CRITICAL
**Impact:** System failure when disk fills

**Current State:**
```
pb_data/auxiliary.db:  337MB  (489,110 log entries)
pb_data/data.db:       912KB  (actual application data)

Growth rate:
- 2025-12-20:  27,119 logs (partial day)
- 2025-12-19:  71,970 logs
- 2025-12-18:  63,305 logs
- 2025-12-17:  49,615 logs
- 2025-12-16: 169,869 logs (development spike)
- 2025-12-15: 108,232 logs

Total: 490K logs = 337MB (689 bytes/log average)
```

**Log Breakdown:**
- 377,739 logs at level 0 (INFO)
- 112,350 logs at level 8 (DEBUG)
- 21 logs at level 4 (WARN)

**Top "Junk" Patterns:**
1. `GET /api/collections/signals/records?filter=(to_agent='WinPc-AG'&&read=false)` - 129,101 times
2. `GET /api/collections/signals/records?filter=to_agent='claude-mac' && read=false&perPage=50` - 100,628 times
3. `GET /api/collections/signals/records?filter=(to_agent='Claud-win'+&&+read=false)` - 48,720 times
4. `GET /api/realtime` - 47,806 times

**Analysis:**
- Polling agents hitting API every few seconds
- Same queries logged repeatedly (no deduplication)
- 280K+ logs from just 4 polling patterns
- No log rotation or pruning configured

**Recommendation:** See Section 5, Recommendation #1

---

### Risk #2: Production/Test Confusion (No Runtime Safeguards)

**Severity:** CRITICAL
**Impact:** Accidental production modifications, data loss, downtime

**Missing Safeguards:**
1. No validation that `FLIP2_ENV=production` doesn't run on port 9xxx
2. No validation that `FLIP2_ENV=test` doesn't run on port 8xxx
3. No prominent startup banner showing environment
4. No config file naming enforcement (both use `config.yaml`)
5. No data directory separation enforcement

**Current Code (daemon.go:103-113):**
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

**Problem:** Logs environment but doesn't VALIDATE or REJECT mismatches.

**Recommendation:** See Section 5, Recommendation #2

---

### Risk #3: Port Convention Inconsistency

**Severity:** HIGH
**Impact:** Confusion, accidental connection to wrong server

**Current Reality:**
```
Mac config.yaml:           port: 8090  ← RUNNING ON THIS
Mac DEVELOPMENT_RULES.md:  "Mac Port: 8091" (production)
Windows expected:          port: 8090
Test ports documented:     9190/9191

Actual running process:
./flip2d --config ./config/config.yaml --foreground
  Listening on 8090 (verified with lsof)
```

**Issues:**
1. Documentation says Mac should use 8091, but config uses 8090
2. Mac and Windows both trying to use 8090 (sync peer config confirms)
3. Test ports defined but never actually used
4. Creates confusion about which is "production"

**Recommendation:** See Section 5, Recommendation #3

---

### Risk #4: Mac-Windows Code Drift

**Severity:** HIGH
**Impact:** Sync failures, incompatible behavior, difficult debugging

**File Count Discrepancy:**
```
Mac:      31 Go files in internal/
Windows:  23 Go files in internal/ (8 files behind)

Missing on Windows (likely):
- Recent sync fixes (pbstore.go updates)
- Bootstrap collection code
- Communication monitor improvements
- Archiver enhancements
```

**Windows Directory Chaos:**
```
C:\Users\Agnizar\src\flip\
├── flip2d.exe (11 different versions!)
├── flip2d-backup.exe
├── flip2d-https.exe
├── flip2d-https-fixed.exe
├── flip2d-new.exe
├── flip2d-old.exe
├── flip2d-pb.exe
├── flip2d-test.exe
├── flip2d-test-backup.exe
├── flip2d-v2-fixed.exe
├── flip2d-v3-latest.exe
└── 100+ .md files (documentation sprawl)

C:\flip2\claude\
├── flip2d.exe
├── flip2.exe
├── config.yaml
└── (NO pb_data directory!) ← CRITICAL
```

**Recommendation:** See Section 5, Recommendation #4

---

### Risk #5: No Test Infrastructure

**Severity:** HIGH
**Impact:** Cannot safely test changes before production deployment

**Current Situation:**
- Test ports (9190/9191) defined but NOT USED
- No pb_data_test on Windows
- No automated test->prod promotion process
- Rule #1 says "test on separate port first" but no tooling to support this

**Testing Process Should Be:**
```
1. Start test daemon on port 9190
2. Verify collections created
3. Test sync with production
4. Test API endpoints
5. ONLY THEN deploy to production
```

**Current Reality:**
```
No test server running
No test config (config-test.yaml doesn't exist)
No test startup script
```

**Recommendation:** See Section 5, Recommendation #5

---

### Risk #6: Sync Mechanism Untested End-to-End

**Severity:** HIGH
**Impact:** Data loss, message duplication, sync loops

**Sync Status (per WORK_LOG.md):**
- Bootstrap code added to auto-create signals collection
- Mac can reach Windows API (verified)
- Windows cannot properly receive synced records
- Vector clock comparison may need adjustment
- Windows has no pb_data directory in deployment location

**Untested Scenarios:**
1. What happens when Windows is offline?
2. Do messages queue properly for delayed sync?
3. Are conflicts resolved correctly?
4. Does bootstrap work on fresh Windows install?
5. Can sync recover from network partition?

**Code Analysis (internal/sync/replicator.go):**
```go
// Sync performs bidirectional synchronization
func (r *Replicator) Sync(ctx context.Context, peerID string) error {
    // 1. Get peer's vector clock
    // 2. Compare clocks
    // 3. Push local records that peer doesn't have
    // 4. Fetch remote records we don't have
    // 5. Update our vector clock
}
```

**Concerns:**
- No timeout handling visible
- No retry logic for failed syncs
- LastWriteWinsResolver may not be appropriate for all signal types
- No detection of sync loops
- Concurrent writes not clearly handled

**Recommendation:** See Section 5, Recommendation #6

---

### Risk #7: Inefficient Polling Causing Resource Waste

**Severity:** MEDIUM
**Impact:** CPU waste, network bandwidth, log spam

**Evidence:**
```
129,101 logs: GET /api/collections/signals/records?filter=(to_agent='WinPc-AG'&&read=false)
100,628 logs: GET /api/collections/signals/records?filter=to_agent='claude-mac' && read=false
 48,720 logs: GET /api/collections/signals/records?filter=(to_agent='Claud-win'+&&+read=false)
```

**Analysis:**
- Agents polling every few seconds
- Each poll creates a log entry
- Same query repeated hundreds of thousands of times
- No long-polling or SSE used (despite pkg/client supporting SSE)

**Better Approaches:**
1. Use Server-Sent Events (SSE) - already implemented in pkg/client/client.go
2. Implement WebSocket push notifications
3. Increase polling interval from seconds to 30-60 seconds
4. Use conditional requests (If-Modified-Since headers)
5. Batch signal retrieval

**Recommendation:** See Section 5, Recommendation #7

---

## 3. ARCHITECTURE ANALYSIS

### 3.1 System Overview

```
FLIP2 Architecture:
┌─────────────────────────────────────────────────────────┐
│                    flip2d Daemon                        │
│                                                         │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐             │
│  │Scheduler │  │ Executor │  │Archiver  │             │
│  │(Cron)    │  │(Tasks)   │  │(Cleanup) │             │
│  └──────────┘  └──────────┘  └──────────┘             │
│                                                         │
│  ┌──────────────────────────────────────┐              │
│  │      PocketBase (Embedded)           │              │
│  │  ┌────────┐ ┌────────┐ ┌──────────┐ │              │
│  │  │signals │ │tasks   │ │agents    │ │              │
│  │  │(1166)  │ │        │ │          │ │              │
│  │  └────────┘ └────────┘ └──────────┘ │              │
│  │                                      │              │
│  │  ┌─────────────────────────────┐   │              │
│  │  │  auxiliary.db (337MB logs!) │   │              │
│  │  └─────────────────────────────┘   │              │
│  └──────────────────────────────────────┘              │
│                                                         │
│  ┌──────────────────────────────────────┐              │
│  │      Sync Replicator                 │              │
│  │  - Vector Clock                      │              │
│  │  - HTTP Peers (Mac ↔ Windows)        │              │
│  │  - PBStore (PocketBase backend)      │              │
│  └──────────────────────────────────────┘              │
└─────────────────────────────────────────────────────────┘
           │                           │
           │ Sync (every 15s)          │ API (port 8090)
           ▼                           ▼
    ┌─────────────┐            ┌──────────────┐
    │Windows Node │            │ CLI / Agents │
    │(port 8090)  │            │              │
    └─────────────┘            └──────────────┘
```

### 3.2 Strengths

1. **Vector Clock Sync** - Solid CRDT-based design for distributed sync
2. **PocketBase Integration** - SQLite-backed, lightweight, good API
3. **Separation of Concerns** - Clean package structure (daemon, sync, archiver, etc.)
4. **Bootstrap Auto-Creation** - Handles missing collections gracefully
5. **Multi-Tier Archiver** - Good design (active → recent → long-term → purge)
6. **SSE Support** - pkg/client implements Server-Sent Events (though underutilized)

### 3.3 Weaknesses

1. **No Environment Validation** - Production can run on test ports and vice versa
2. **Inefficient Polling** - Agents poll instead of using SSE
3. **Log Explosion** - No pruning, retention, or VACUUM
4. **Sync Not Battle-Tested** - Theoretical correctness, but untested failure modes
5. **No Monitoring/Alerting** - No way to know when things break
6. **Manual Deployment** - Error-prone, no CI/CD
7. **Config Inconsistencies** - Documentation vs reality don't match
8. **Missing Test Harness** - Can't safely test changes

---

## 4. DETAILED CODE REVIEW

### 4.1 daemon.go - Environment Validation (CRITICAL)

**File:** `/Users/arielspivakovsky/src/flip/flip2/internal/daemon/daemon.go`

**Current Code (Lines 103-113):**
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

**Issues:**
1. Logs environment but doesn't validate
2. Allows production env to run on test port (9xxx)
3. Allows test env to run on production port (8xxx)
4. No rejection of invalid combinations

**Recommended Fix:**
```go
// Validate environment vs port (SAFETY CHECK)
env := os.Getenv("FLIP2_ENV")
if env == "" {
    env = "production" // Default to production
}
port := d.config.Flip2.PocketBase.Port

// Production safety checks
if env == "production" {
    if port >= 9000 && port < 10000 {
        return fmt.Errorf("SAFETY ABORT: Production environment (FLIP2_ENV=production) cannot run on test port %d (9xxx range reserved for testing)", port)
    }
    if d.config.Flip2.PocketBase.DataDir == "./pb_data_test" {
        return fmt.Errorf("SAFETY ABORT: Production environment cannot use test data directory")
    }
}

// Test safety checks
if env == "test" {
    if port >= 8000 && port < 9000 {
        return fmt.Errorf("SAFETY ABORT: Test environment (FLIP2_ENV=test) cannot run on production port %d (8xxx range reserved for production)", port)
    }
    if d.config.Flip2.PocketBase.DataDir == "./pb_data" {
        return fmt.Errorf("SAFETY ABORT: Test environment cannot use production data directory")
    }
}

// Log prominently
d.logger.Info("========================================")
d.logger.Info("FLIP2 DAEMON STARTING - ENVIRONMENT VALIDATED",
    "environment", env,
    "port", port,
    "data_dir", d.config.Flip2.PocketBase.DataDir,
    "pid", os.Getpid())
d.logger.Info("========================================")
```

**Priority:** CRITICAL - Prevents incident recurrence

---

### 4.2 daemon.go - Log Cleanup Job (CRITICAL)

**File:** `/Users/arielspivakovsky/src/flip/flip2/internal/daemon/daemon.go`

**Current Code (Lines 451-503):**
Zombie reaper job exists, but NO log cleanup job.

**Recommended Addition:**
```go
func (d *Daemon) registerJobs() {
    // Health check job (existing)
    d.scheduler.RegisterJob("health-check", "0 */1 * * * *", func(ctx context.Context) error {
        d.logger.Info("Health check: OK")
        return nil
    })

    // Zombie Task Reaper (existing)
    d.scheduler.RegisterJob("zombie-reaper", "0 */5 * * * *", func(ctx context.Context) error {
        // ... existing code ...
    })

    // NEW: Log Cleanup Job - Run every 8 hours
    d.scheduler.RegisterJob("log-cleanup", "0 0 */8 * * *", func(ctx context.Context) error {
        cutoff := time.Now().Add(-48 * time.Hour)

        // Delete logs older than 48 hours
        result, err := d.pb.DB().NewQuery(
            "DELETE FROM _logs WHERE created < {:cutoff}",
        ).Bind(dbx.Params{
            "cutoff": cutoff.Format("2006-01-02 15:04:05"),
        }).Execute()

        if err != nil {
            d.logger.Error("Log cleanup failed", "error", err)
            return err
        }

        rowsAffected, _ := result.RowsAffected()
        d.logger.Info("Log cleanup completed",
            "deleted", rowsAffected,
            "cutoff", cutoff.Format("2006-01-02 15:04:05"))

        return nil
    })

    // NEW: Log Pattern Cleanup - Run every 8 hours (offset by 4h from main cleanup)
    d.scheduler.RegisterJob("log-pattern-cleanup", "0 0 4,12,20 * * *", func(ctx context.Context) error {
        // Delete repetitive polling logs immediately (keep last 100 of each pattern)
        patterns := []string{
            "GET /api/collections/signals/records?filter=(to_agent='WinPc-AG'&&read=false)",
            "GET /api/collections/signals/records?filter=to_agent='claude-mac' && read=false",
            "GET /api/collections/signals/records?filter=(to_agent='Claud-win'+&&+read=false)",
            "GET /api/realtime",
        }

        totalDeleted := int64(0)
        for _, pattern := range patterns {
            // Keep only the 100 most recent logs of this pattern
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

    // NEW: Database VACUUM - Run weekly on Sunday at 3 AM
    d.scheduler.RegisterJob("db-vacuum", "0 0 3 * * 0", func(ctx context.Context) error {
        d.logger.Info("Starting database VACUUM (this may take several minutes)")

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
}
```

**Priority:** CRITICAL - Prevents disk exhaustion

---

### 4.3 pbstore.go - Sync Reliability (HIGH)

**File:** `/Users/arielspivakovsky/src/flip/flip2/internal/sync/pbstore.go`

**Current Code (Lines 84-196):**
ApplyRecord handles create/update/delete but has potential issues.

**Issues Identified:**
1. **Line 145-149:** ID assignment logic complex, may cause primary key conflicts
2. **No conflict detection:** If record exists with different data, always overwrites
3. **No retry logic:** Network/DB errors fail immediately
4. **Silent failures:** Errors logged but not bubbled up to caller

**Recommended Improvements:**
```go
// ApplyRecord applies an incoming record to the signals collection
func (s *PBStore) ApplyRecord(record *Record) error {
    if record == nil {
        return fmt.Errorf("cannot apply nil record")
    }

    s.mu.Lock()
    defer s.mu.Unlock()

    // Only handle signals collection for now
    if record.Collection != "signals" {
        s.logger.Debug("Skipping non-signals collection", "collection", record.Collection)
        return nil
    }

    // Parse the data
    var signalData map[string]interface{}
    if err := json.Unmarshal(record.Data, &signalData); err != nil {
        return fmt.Errorf("failed to unmarshal record data: %w", err)
    }

    collection, err := s.pb.FindCollectionByNameOrId("signals")
    if err != nil {
        return fmt.Errorf("signals collection not found: %w", err)
    }

    signalID, _ := signalData["signal_id"].(string)
    if signalID == "" {
        signalID = record.RecordID
    }
    if signalID == "" {
        return fmt.Errorf("record missing signal_id")
    }

    existingRecords, err := s.pb.FindRecordsByFilter(
        collection.Id,
        fmt.Sprintf("signal_id = '%s'", signalID),
        "",
        1,
        0,
    )

    switch record.Operation {
    case "delete":
        if len(existingRecords) > 0 {
            if err := s.pb.Delete(existingRecords[0]); err != nil {
                return fmt.Errorf("failed to delete record: %w", err)
            }
            s.logger.Info("Deleted synced record", "signal_id", signalID)
        }
        return nil

    case "create", "update", "":
        var pbRecord *core.Record

        if len(existingRecords) > 0 {
            // Update existing - check vector clock first
            pbRecord = existingRecords[0]

            // Get existing vector clock
            existingVCStr := pbRecord.GetString("sync_vector_clock")
            if existingVCStr != "" {
                existingVC := &VectorClock{}
                if err := json.Unmarshal([]byte(existingVCStr), existingVC); err == nil {
                    // Compare vector clocks
                    comparison := record.VectorClock.Compare(existingVC)
                    if comparison == Before {
                        s.logger.Debug("Rejected older record",
                            "signal_id", signalID,
                            "existing_vc", existingVC.Clocks,
                            "incoming_vc", record.VectorClock.Clocks)
                        return nil // Don't apply older version
                    }
                    if comparison == Concurrent {
                        // Conflict - use timestamp as tiebreaker
                        existingTS := pbRecord.GetDateTime("sync_timestamp").Time()
                        if record.Timestamp.Before(existingTS) {
                            s.logger.Debug("Rejected concurrent record (older timestamp)",
                                "signal_id", signalID)
                            return nil
                        }
                    }
                }
            }
        } else {
            // Create new record
            pbRecord = core.NewRecord(collection)

            // IMPROVED: Use signal_id as PocketBase ID to avoid collisions
            // This ensures same signal always gets same PocketBase record ID
            pbRecord.Id = signalID
        }

        // Set fields from signal data
        if v, ok := signalData["signal_id"]; ok {
            pbRecord.Set("signal_id", v)
        }
        if v, ok := signalData["from_agent"]; ok {
            pbRecord.Set("from_agent", v)
        }
        if v, ok := signalData["to_agent"]; ok {
            pbRecord.Set("to_agent", v)
        }
        if v, ok := signalData["signal_type"]; ok {
            pbRecord.Set("signal_type", v)
        }
        if v, ok := signalData["priority"]; ok {
            pbRecord.Set("priority", v)
        }
        if v, ok := signalData["content"]; ok {
            pbRecord.Set("content", v)
        }
        if v, ok := signalData["read"]; ok {
            pbRecord.Set("read", v)
        }
        if v, ok := signalData["read_at"]; ok {
            pbRecord.Set("read_at", v)
        }

        // Store sync metadata
        vcJSON, _ := json.Marshal(record.VectorClock)
        pbRecord.Set("sync_vector_clock", string(vcJSON))
        pbRecord.Set("sync_origin", record.OriginNodeID)
        pbRecord.Set("sync_timestamp", record.Timestamp)

        // Retry logic for transient failures
        maxRetries := 3
        var lastErr error
        for attempt := 0; attempt < maxRetries; attempt++ {
            if err := s.pb.Save(pbRecord); err != nil {
                lastErr = err
                if attempt < maxRetries-1 {
                    time.Sleep(time.Duration(attempt+1) * 100 * time.Millisecond)
                    s.logger.Debug("Retrying save", "attempt", attempt+1, "error", err)
                    continue
                }
                return fmt.Errorf("failed to save record after %d attempts: %w", maxRetries, err)
            }
            lastErr = nil
            break
        }

        if lastErr != nil {
            return lastErr
        }

        s.logger.Info("Applied synced record",
            "signal_id", signalID,
            "operation", record.Operation,
            "origin", record.OriginNodeID)
        return nil

    default:
        return fmt.Errorf("unknown operation: %s", record.Operation)
    }
}
```

**Priority:** HIGH - Improves sync reliability

---

### 4.4 config.yaml - Port Standardization (HIGH)

**File:** `/Users/arielspivakovsky/src/flip/flip2/config/config.yaml`

**Current (Mac):**
```yaml
pocketbase:
  port: 8090  # ← INCORRECT per documentation
```

**Documentation says:**
```
Mac Production: 8091
Windows Production: 8090
```

**Recommended Fix:**

**Option A: Follow Documentation (Change Mac to 8091)**
```yaml
pocketbase:
  port: 8091  # Mac production port

sync:
  node_id: mac
  peers:
    - id: windows
      url: http://192.168.1.220:8090  # Windows production
      api_key: flip2_secret_key_123
```

**Option B: Standardize on 8090 for ALL Production (Simpler)**
```yaml
pocketbase:
  port: 8090  # ALL production nodes use 8090

sync:
  node_id: mac
  peers:
    - id: windows
      url: http://192.168.1.220:8090
      api_key: flip2_secret_key_123
```

**Recommendation:** Option B (8090 for all production) is simpler and less confusing.

**Update Documentation:**
```markdown
### Port Convention (FINAL)
| Environment | Port | Data Dir |
|-------------|------|----------|
| Production  | 8090 | ./pb_data |
| Test        | 9190 | ./pb_data_test |

**All production nodes (Mac, Windows) use port 8090.**
**All test instances use port 9190.**
```

**Priority:** HIGH - Eliminates confusion

---

## 5. RECOMMENDATIONS (Prioritized)

### Priority 0: CRITICAL (Fix Immediately)

#### Recommendation #1: Implement Aggressive Log Pruning

**Action:**
1. Add log cleanup job to daemon.go (see Section 4.2)
2. Run every 8 hours, delete logs > 48 hours old
3. Add pattern-based cleanup for polling logs (keep last 100 of each pattern)
4. Add weekly VACUUM job to reclaim space
5. Monitor log growth with metrics

**Implementation:**
```bash
# Immediate cleanup (manual)
sqlite3 pb_data/auxiliary.db "DELETE FROM _logs WHERE created < datetime('now', '-48 hours')"
sqlite3 pb_data/auxiliary.db "VACUUM"

# Verify space reclaimed
ls -lh pb_data/auxiliary.db
```

**Expected Impact:** Reduce auxiliary.db from 337MB to <10MB, prevent future disk exhaustion

**Timeline:** Deploy within 24 hours

---

#### Recommendation #2: Add Runtime Environment Validation

**Action:**
1. Add environment validation function to daemon.go (see Section 4.1)
2. Reject production env on test ports (9xxx)
3. Reject test env on production ports (8xxx)
4. Reject data directory mismatches
5. Log prominently on startup

**Implementation:** See Section 4.1 code

**Expected Impact:** Prevents production/test confusion incidents

**Timeline:** Deploy within 24 hours

---

#### Recommendation #3: Standardize Port Convention

**Action:**
1. Decision: ALL production nodes use port 8090
2. Update Mac config.yaml: `port: 8090` (already correct)
3. Update Windows config: `port: 8090` (already correct)
4. Update DEVELOPMENT_RULES.md to remove Mac/Windows port differences
5. Document: Production=8090, Test=9190 everywhere

**Implementation:**
```yaml
# config/config.yaml (Mac & Windows production)
pocketbase:
  port: 8090
  data_dir: ./pb_data

# config/config-test.yaml (Mac & Windows test)
pocketbase:
  port: 9190
  data_dir: ./pb_data_test
```

**Expected Impact:** Eliminates port confusion

**Timeline:** Documentation update within 1 day, deploy config changes next sync

---

#### Recommendation #4: Windows Deployment Bootstrap

**Action:**
1. Create clean Windows deployment directory: `C:\flip2\`
2. Cross-compile latest binaries with all Mac fixes
3. Create proper config.yaml for Windows
4. SCP binaries and config to Windows
5. Initialize pb_data directory (bootstrap will create collections)
6. Verify sync works end-to-end
7. Clean up old Windows locations (archive)

**Script:**
```bash
#!/bin/bash
# scripts/deploy_to_windows.sh

set -e

cd /Users/arielspivakovsky/src/flip/flip2

# 1. Build Windows binaries
echo "Building Windows binaries..."
GOOS=windows GOARCH=amd64 go build -o flip2d-win.exe ./cmd/flip2d
GOOS=windows GOARCH=amd64 go build -o flip2-win.exe ./cmd/flip2

# 2. Create Windows config
cat > config-win.yaml << 'EOF'
flip2:
  daemon:
    pid_file: C:\flip2\flip2d.pid
    log_file: C:\flip2\flip2d.log
    log_level: info
  pocketbase:
    host: 0.0.0.0
    port: 8090
    data_dir: C:\flip2\pb_data
  sync:
    enabled: true
    node_id: windows
    sync_interval: 15s
    peers:
      - id: mac
        url: http://192.168.1.53:8090
        api_key: flip2_secret_key_123
        enabled: true
  security:
    api_keys_enabled: true
    api_key: flip2_secret_key_123
    jwt_secret: flip2_jwt_secret_key_456
    bootstrap_api_key: flip2_bootstrap_key_789
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

# 3. Deploy to Windows
echo "Deploying to Windows..."
scp flip2d-win.exe Agnizar@192.168.1.220:C:/flip2/flip2d.exe
scp flip2-win.exe Agnizar@192.168.1.220:C:/flip2/flip2.exe
scp config-win.yaml Agnizar@192.168.1.220:C:/flip2/config.yaml

echo "Deployment complete. Now SSH to Windows and start daemon."
```

**Timeline:** Deploy within 48 hours

---

#### Recommendation #5: Reduce Polling, Use SSE

**Action:**
1. Audit all agents that poll /api/collections/signals/records
2. Switch to Server-Sent Events (pkg/client already supports this!)
3. For agents that must poll, increase interval from seconds to 30-60 seconds
4. Implement conditional requests (ETag or Last-Modified)
5. Reduce PocketBase log verbosity for API requests

**Implementation:**
```go
// Example: Using SSE instead of polling (pkg/client/client.go already has this)

import "flip2/pkg/client"

// Old way (polling)
for {
    signals, _ := GetSignals("claude-mac", false)
    // process signals
    time.Sleep(5 * time.Second) // ← Creates 17K logs/day!
}

// New way (SSE)
c := client.New("http://localhost:8090", "flip2_secret_key_123")
events, err := c.StreamSignals("claude-mac")
if err != nil {
    log.Fatal(err)
}

for event := range events {
    // Process signal in real-time, no polling!
    log.Printf("New signal: %+v", event)
}
```

**Expected Impact:**
- Reduce 280K+ repetitive logs to near-zero
- Improve real-time responsiveness
- Reduce CPU/network waste

**Timeline:** Refactor agents within 1 week

---

### Priority 1: HIGH (This Week)

#### Recommendation #6: Sync End-to-End Testing

**Action:**
1. Deploy latest code to both Mac and Windows
2. Start both daemons with sync enabled
3. Test scenarios:
   - Send signal from Mac, verify on Windows
   - Send signal from Windows, verify on Mac
   - Take Windows offline, send signals from Mac, bring Windows back online (delayed sync)
   - Concurrent writes to same signal (conflict resolution)
   - Network partition simulation
4. Monitor vector clocks during tests
5. Document actual sync behavior vs expected

**Test Plan:**
```bash
# Test 1: Mac → Windows
curl -X POST http://localhost:8090/api/collections/signals/records \
  -H "X-API-Key: flip2_secret_key_123" \
  -d '{"signal_id":"test-001","from_agent":"test-mac","to_agent":"test-win","content":"Hello from Mac"}'

# Wait 30 seconds for sync cycle
sleep 30

# Verify on Windows
curl http://192.168.1.220:8090/api/collections/signals/records?filter=signal_id='test-001' \
  -H "X-API-Key: flip2_secret_key_123"

# Test 2: Windows → Mac (repeat in reverse)
# Test 3: Conflict resolution (both write same signal_id with different content)
# Test 4: Network partition (stop Windows daemon, write on Mac, restart Windows, verify sync catches up)
```

**Timeline:** Complete within 3 days

---

#### Recommendation #7: Create Test Infrastructure

**Action:**
1. Create `config/config-test.yaml` for Mac test server
2. Create `config/config-test-win.yaml` for Windows test server
3. Create startup scripts: `scripts/start_test_server.sh`
4. Add `make test-server` target to Makefile
5. Document test server usage in DEVELOPMENT_RULES.md

**Implementation:**
```yaml
# config/config-test.yaml (Mac test)
flip2:
  daemon:
    pid_file: /tmp/flip2d-test.pid
    log_file: /tmp/flip2d-test.log
    log_level: debug
  pocketbase:
    host: 0.0.0.0
    port: 9190
    data_dir: ./pb_data_test
    tls:
      enabled: false
  sync:
    enabled: false  # Don't sync test to production!
  security:
    api_keys_enabled: true
    api_key: flip2_test_key_123
```

```bash
# scripts/start_test_server.sh
#!/bin/bash
export FLIP2_ENV=test
./flip2d --config config/config-test.yaml --foreground
```

**Timeline:** Complete within 3 days

---

#### Recommendation #8: Consolidate Windows Codebase

**Action:**
1. Archive `C:\Users\Agnizar\src\flip\` to `C:\Users\Agnizar\src\flip.archive\`
2. Remove `C:\flip2\v1\` (appears unused)
3. Remove `C:\flip2\claude\` (will be replaced by `C:\flip2\`)
4. Establish `C:\flip2\` as ONLY Windows deployment location
5. Delete all 11 flip2d-*.exe variants except current production binary
6. Move 100+ .md files to archive (keep only essential docs)

**Timeline:** Complete within 3 days

---

#### Recommendation #9: Improve Sync Conflict Resolution

**Action:**
1. Review LastWriteWinsResolver logic in pbstore.go
2. Add vector clock comparison BEFORE overwriting existing records
3. Implement retry logic for transient failures
4. Add sync metrics (conflicts detected, resolutions, failures)
5. Log conflicts at WARN level for investigation

**Implementation:** See Section 4.3 code improvements

**Timeline:** Complete within 5 days

---

### Priority 2: MEDIUM (Next Sprint)

#### Recommendation #10: Monitoring & Alerting

**Action:**
1. Add `/api/metrics/health` endpoint with disk space check
2. Implement alerting when auxiliary.db > 50MB
3. Track sync lag (time since last successful peer sync)
4. Monitor signal queue depth per agent
5. Add Prometheus metrics export (optional)

**Timeline:** Complete within 2 weeks

---

#### Recommendation #11: Automated Deployment Pipeline

**Action:**
1. Create Git repository for FLIP2 (if not already)
2. Add GitHub Actions or similar CI/CD
3. Automated cross-compile on commit
4. Automated deployment to Windows test server
5. Smoke tests before promoting to production

**Timeline:** Complete within 2 weeks

---

#### Recommendation #12: Config File Naming Enforcement

**Action:**
1. Update config loader to check filename matches environment
2. Production must use `config.yaml` or `config-prod.yaml`
3. Test must use `config-test.yaml`
4. Reject mismatches

**Implementation:**
```go
func LoadConfig(path string) (*Config, error) {
    env := os.Getenv("FLIP2_ENV")
    if env == "" {
        env = "production"
    }

    filename := filepath.Base(path)

    // Validate filename matches environment
    if env == "production" && !strings.Contains(filename, "prod") && filename != "config.yaml" {
        return nil, fmt.Errorf("production environment requires config.yaml or config-prod.yaml, got %s", filename)
    }
    if env == "test" && !strings.Contains(filename, "test") {
        return nil, fmt.Errorf("test environment requires config-test.yaml, got %s", filename)
    }

    // ... rest of loading ...
}
```

**Timeline:** Complete within 2 weeks

---

#### Recommendation #13: Log Level Configuration

**Action:**
1. Reduce PocketBase log verbosity in production
2. Only log API requests at DEBUG level, not INFO
3. Add ability to change log level at runtime via API
4. Consider separate log files for different components

**Timeline:** Complete within 2 weeks

---

#### Recommendation #14: Database Backup Strategy

**Action:**
1. Automated daily backup of data.db (NOT auxiliary.db)
2. Keep 7 daily backups
3. Weekly backup retention (4 weeks)
4. Store backups in archives/backups/
5. Test restore procedure

**Timeline:** Complete within 2 weeks

---

### Priority 3: LOW (Backlog)

#### Recommendation #15-28: Additional Improvements

15. **File Transfer Optimization** - Implement chunked upload for large files
16. **OAuth Integration** - Add OAuth for human users
17. **Multi-Peer Sync** - Support more than 2 nodes
18. **Sync Topology** - Star vs mesh sync patterns
19. **Rate Limiting** - Prevent API abuse
20. **Query Optimization** - Add indexes for common filters
21. **Connection Pooling** - Reuse HTTP connections in sync
22. **Compression** - Gzip sync payloads
23. **Encryption** - Encrypt signals at rest
24. **Audit Log** - Track all admin actions
25. **API Versioning** - Support /api/v1/, /api/v2/
26. **WebSocket Upgrade** - Replace HTTP polling in sync
27. **Health Checks** - More comprehensive system health
28. **Documentation** - API documentation, architecture diagrams

---

## 6. INCIDENT PREVENTION CHECKLIST

Before ANY work on FLIP2, run this checklist:

```bash
#!/bin/bash
# scripts/pre_work_checklist.sh

echo "=== FLIP2 PRE-WORK SAFETY CHECKLIST ==="
echo ""

# 1. Check environment variable
if [ -z "$FLIP2_ENV" ]; then
    echo "⚠️  WARNING: FLIP2_ENV not set. Defaulting to PRODUCTION."
    export FLIP2_ENV=production
else
    echo "✓ FLIP2_ENV=$FLIP2_ENV"
fi

# 2. Check running processes
echo ""
echo "Running FLIP2 processes:"
ps aux | grep flip2d | grep -v grep || echo "  None"

# 3. Check port usage
echo ""
echo "Port usage:"
echo "  8090 (Production): $(lsof -ti :8090 | wc -l | xargs) processes"
echo "  9190 (Test):       $(lsof -ti :9190 | wc -l | xargs) processes"

# 4. Check database sizes
echo ""
echo "Database sizes:"
if [ -f pb_data/data.db ]; then
    echo "  pb_data/data.db:      $(du -h pb_data/data.db | cut -f1)"
fi
if [ -f pb_data/auxiliary.db ]; then
    echo "  pb_data/auxiliary.db: $(du -h pb_data/auxiliary.db | cut -f1)"
fi
if [ -f pb_data_test/data.db ]; then
    echo "  pb_data_test/data.db: $(du -h pb_data_test/data.db | cut -f1)"
fi

# 5. Check config file
echo ""
if [ -f config/config.yaml ]; then
    echo "Config file: config/config.yaml"
    PORT=$(grep "port:" config/config.yaml | head -1 | awk '{print $2}')
    DATA_DIR=$(grep "data_dir:" config/config.yaml | head -1 | awk '{print $2}')
    echo "  Port: $PORT"
    echo "  Data Dir: $DATA_DIR"

    # Validate port vs environment
    if [ "$FLIP2_ENV" = "production" ] && [ "$PORT" -ge 9000 ]; then
        echo "  ⚠️  ERROR: Production environment on test port!"
    elif [ "$FLIP2_ENV" = "test" ] && [ "$PORT" -lt 9000 ]; then
        echo "  ⚠️  ERROR: Test environment on production port!"
    else
        echo "  ✓ Port matches environment"
    fi
fi

echo ""
echo "=== CHECKLIST COMPLETE ==="
echo ""
echo "Proceed with work? (y/n)"
read -r response
if [ "$response" != "y" ]; then
    echo "Aborted."
    exit 1
fi
```

**Usage:**
```bash
# Before ANY FLIP2 work:
./scripts/pre_work_checklist.sh
```

---

## 7. TESTING STRATEGY

### 7.1 Unit Tests Needed

Current test coverage is minimal. Need tests for:

1. **Vector Clock** - `internal/sync/vectorclock_test.go`
   - Increment, Update, Compare
   - Concurrent operations
   - Edge cases (empty clocks, same node)

2. **Replicator** - `internal/sync/replicator_test.go`
   - Sync scenarios (after, before, concurrent)
   - Conflict resolution
   - Network failures

3. **PBStore** - `internal/sync/pbstore_test.go`
   - ApplyRecord with conflicts
   - GetRecordsSince filtering
   - Error handling

4. **Archiver** - `internal/archiver/archiver_test.go` (exists but expand)
   - Multi-tier archiving
   - Agent filtering
   - Batch processing

5. **Daemon** - `internal/daemon/daemon_test.go`
   - Environment validation
   - Job registration
   - Shutdown cleanup

### 7.2 Integration Tests Needed

1. **End-to-End Sync Test**
   - Start two daemons
   - Send signal on one
   - Verify appears on other
   - Test conflict resolution

2. **Bootstrap Test**
   - Delete pb_data
   - Start daemon
   - Verify collections created
   - Verify sync metadata fields exist

3. **Log Cleanup Test**
   - Insert 10K junk logs
   - Run cleanup job
   - Verify deletion
   - Verify VACUUM reclaimed space

4. **Archiver Test**
   - Insert old signals
   - Run archiver
   - Verify moved to archive
   - Verify file export

---

## 8. CONFIGURATION TEMPLATES

### 8.1 Mac Production (config.yaml)

```yaml
flip2:
  daemon:
    pid_file: /tmp/flip2d.pid
    log_file: /tmp/flip2d.log
    log_level: info
    max_log_file_size_mb: 100
    max_log_files: 5

  pocketbase:
    host: 0.0.0.0
    port: 8090  # Production port
    data_dir: ./pb_data
    tls:
      enabled: true
      cert_file: ./certs/flip2.crt
      key_file: ./certs/flip2.key

  security:
    api_keys_enabled: true
    api_key: flip2_secret_key_123
    jwt_secret: flip2_jwt_secret_key_456
    bootstrap_api_key: flip2_bootstrap_key_789

  sync:
    enabled: true
    node_id: mac
    sync_interval: 15s
    peers:
      - id: windows
        url: http://192.168.1.220:8090
        api_key: flip2_secret_key_123
        enabled: true

  archiver:
    enabled: true
    active_retention_days: 3
    recent_retention_days: 90
    check_interval: 6h
    batch_size: 200
    archive_path: archives/signals
    active_agents:
      - claude-mac
      - claude-win
    deprecated_agents:
      - gemini
      - claude
      - cli
      - antigravity

  executor:
    max_concurrent_tasks: 3
    default_timeout: 300s

  metrics:
    enabled: true
```

### 8.2 Mac Test (config-test.yaml)

```yaml
flip2:
  daemon:
    pid_file: /tmp/flip2d-test.pid
    log_file: /tmp/flip2d-test.log
    log_level: debug

  pocketbase:
    host: 0.0.0.0
    port: 9190  # Test port
    data_dir: ./pb_data_test
    tls:
      enabled: false  # No TLS for local test

  security:
    api_keys_enabled: true
    api_key: flip2_test_key_123

  sync:
    enabled: false  # Don't sync test to prod!

  archiver:
    enabled: false  # No archiving in test
```

### 8.3 Windows Production (config-win.yaml)

```yaml
flip2:
  daemon:
    pid_file: C:\flip2\flip2d.pid
    log_file: C:\flip2\flip2d.log
    log_level: info

  pocketbase:
    host: 0.0.0.0
    port: 8090  # Production port
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
```

---

## 9. DEPLOYMENT PLAN

### Phase 1: Immediate Fixes (24 hours)

1. ✅ Add environment validation to daemon.go
2. ✅ Add log cleanup jobs (48h retention + pattern cleanup + weekly VACUUM)
3. ✅ Standardize port convention documentation
4. ✅ Run manual log cleanup on Mac production
5. ✅ Test environment validation locally

### Phase 2: Windows Bootstrap (48 hours)

1. ✅ Build latest Mac code with all fixes
2. ✅ Cross-compile for Windows
3. ✅ Create clean Windows config
4. ✅ Deploy to C:\flip2\
5. ✅ Initialize pb_data (bootstrap)
6. ✅ Test sync end-to-end
7. ✅ Archive old Windows locations

### Phase 3: Test Infrastructure (1 week)

1. ✅ Create config-test.yaml templates
2. ✅ Create test startup scripts
3. ✅ Document test procedure
4. ✅ Test on both Mac and Windows
5. ✅ Add to DEVELOPMENT_RULES.md

### Phase 4: Agent Refactoring (1 week)

1. ✅ Audit polling agents
2. ✅ Convert to SSE where possible
3. ✅ Increase polling intervals for remaining pollers
4. ✅ Monitor log reduction
5. ✅ Verify real-time responsiveness

### Phase 5: Monitoring & Hardening (2 weeks)

1. ✅ Add health metrics endpoint
2. ✅ Implement sync monitoring
3. ✅ Add conflict resolution metrics
4. ✅ Create automated deployment pipeline
5. ✅ Comprehensive sync testing

---

## 10. SUCCESS METRICS

Track these metrics to measure improvement:

### Disk Space
- **Before:** auxiliary.db = 337MB, growing 50K logs/day
- **Target:** auxiliary.db < 10MB, stable
- **Measurement:** `du -h pb_data/auxiliary.db`

### Log Volume
- **Before:** 490K total logs, 280K from polling
- **Target:** <10K logs/day after SSE migration
- **Measurement:** `sqlite3 pb_data/auxiliary.db "SELECT COUNT(*) FROM _logs WHERE DATE(created) = DATE('now')"`

### Sync Reliability
- **Before:** Untested, Windows has no data
- **Target:** 99.9% sync success rate, <15s lag
- **Measurement:** Monitor sync job logs, track failures

### Incident Rate
- **Before:** 1 critical incident (prod/test confusion)
- **Target:** 0 incidents for 90 days
- **Measurement:** Manual tracking

### Code Consistency
- **Before:** Mac 31 files, Windows 23 files (26% drift)
- **Target:** 100% parity
- **Measurement:** `diff -qr mac/internal windows/internal`

---

## 11. RISK MATRIX

| Risk | Likelihood | Impact | Severity | Mitigation |
|------|------------|--------|----------|------------|
| Disk exhaustion from logs | HIGH | HIGH | CRITICAL | Rec #1: Log cleanup |
| Prod/test confusion | MEDIUM | HIGH | CRITICAL | Rec #2: Environment validation |
| Sync data loss | MEDIUM | MEDIUM | HIGH | Rec #6: End-to-end testing |
| Code drift Mac/Win | HIGH | MEDIUM | HIGH | Rec #4: Windows bootstrap |
| Windows deployment broken | HIGH | MEDIUM | HIGH | Rec #4: Fix pb_data |
| Polling resource waste | HIGH | LOW | MEDIUM | Rec #5: SSE migration |
| Sync conflict errors | MEDIUM | MEDIUM | MEDIUM | Rec #9: Improve resolution |
| No test environment | HIGH | LOW | MEDIUM | Rec #7: Test infrastructure |
| Port confusion | MEDIUM | MEDIUM | MEDIUM | Rec #3: Standardize ports |
| No monitoring | MEDIUM | LOW | MEDIUM | Rec #10: Add metrics |

---

## 12. CONCLUSION

The FLIP2 system has a solid architectural foundation with vector clock-based sync and PocketBase integration. However, the recent incident exposed critical gaps in operational safeguards and testing.

**Most Critical Issues:**
1. Log explosion (337MB, growing unchecked)
2. No runtime environment validation
3. Windows deployment broken (no pb_data)
4. Sync untested end-to-end
5. Inefficient polling wasting resources

**Recommended Action Plan:**
1. **Day 1:** Deploy log cleanup + environment validation (Rec #1, #2)
2. **Day 2:** Fix Windows deployment (Rec #4)
3. **Week 1:** Test sync, reduce polling (Rec #5, #6)
4. **Week 2:** Create test infrastructure, monitoring (Rec #7, #10)

By implementing these recommendations in priority order, FLIP2 will achieve:
- ✅ Prevention of production/test confusion
- ✅ Disk space sustainability
- ✅ Reliable Mac-Windows synchronization
- ✅ Resource efficiency (SSE vs polling)
- ✅ Safe testing before production deployment

The architecture is sound. The focus now should be on **operational hardening** and **test infrastructure** to prevent incidents and enable confident deployments.

---

**End of Review**

*Generated by Claude Sonnet 4.5 on 2025-12-20*
