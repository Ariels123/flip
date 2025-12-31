# FLIP2 Code Review - Go Best Practices
**Date:** 2025-12-31
**Reviewers:** Claude (primary), Gemini (commmonitor)
**Scope:** Architecture, concurrency, error handling, resource management

---

## Executive Summary

**Overall Grade:** B+ (Good, with critical issues to address)

### Critical Issues (Fix Immediately):
1. **Memory Leak** - commmonitor can load entire signals table into RAM
2. **Polling Anti-Pattern** - commmonitor polls every 10s when SSE is available

### High Priority:
3. **Missing API Method** - client.UpdateSignal() needed for SSE migration
4. **Hardcoded Config** - ValidAgents/TypoCorrections should be in config.yaml

### Medium Priority:
5. **Silent Failures** - commmonitor swallows errors without metrics
6. **Tight Coupling** - commmonitor depends on PocketBase server SDK

---

## Detailed Findings

### 1. Architecture & Design Patterns

#### ✅ GOOD: Supervisor Pattern (daemon.go:152-188)
```go
// Excellent use of Erlang-style supervision tree
d.supervisor = supervisor.New("flip2-root", d.logger, 10, 1*time.Minute)
d.supervisor.AddWorker(supervisor.WorkerSpec{
    Worker:        execWorker,
    Strategy:      supervisor.Permanent,
    MaxRestarts:   5,
    RestartWindow: 1 * time.Minute,
})
```
**Verdict:** Best practice for fault tolerance. Workers auto-restart on crash.

#### ✅ GOOD: Environment Validation (daemon.go:113-120)
```go
// CRITICAL: Validate environment matches port convention
port := d.config.Flip2.PocketBase.Port
if err := validateEnvironmentPort(env, port); err != nil {
    d.logger.Error("ENVIRONMENT VALIDATION FAILED", "error", err)
    return fmt.Errorf("environment validation failed: %w", err)
}
```
**Verdict:** Prevents production/test confusion. Excellent safety mechanism.

#### ✅ GOOD: Dependency Injection (daemon.go:64-87)
```go
func New(configPath string) (*Daemon, error) {
    cfg, err := config.LoadConfig(configPath)
    mainLogger, _, currentLogPath, err := logger.SetupLogger(cfg.Flip2.Daemon)
    return &Daemon{
        config:  cfg,
        logger:  mainLogger,
        ...
    }, nil
}
```
**Verdict:** Clean DI pattern, testable design.

#### ❌ CRITICAL: Unbounded Memory Load (commmonitor/monitor.go:282-310)
```go
func (m *Monitor) getRecentSignals(limit int) ([]*core.Record, error) {
    // ...
    if err != nil {
        // Fallback loads ENTIRE TABLE into RAM!
        records, err = m.pb.FindAllRecords(collection)
        // Risk: OOM crash as database grows
    }
    // ...
}
```
**Verdict:** DANGEROUS. Will crash when signals table exceeds available RAM.

**Fix:**
```go
// Remove dangerous fallback, enforce limit
func (m *Monitor) getRecentSignals(limit int) ([]*core.Record, error) {
    collection, err := m.pb.FindCollectionByNameOrId("signals")
    if err != nil {
        return nil, err
    }

    records, err := m.pb.FindRecordsByFilter(
        "signals",
        "",
        "-created", // Sort newest first
        limit,
        0,
    )
    // No fallback - fail fast with error
    return records, err
}
```

---

### 2. Concurrency & Thread Safety

#### ✅ GOOD: Mutex Protection (commmonitor/monitor.go:190-192)
```go
m.mu.Lock()
m.signalCount++
m.mu.Unlock()
```
**Verdict:** Correct mutex usage for shared stats.

#### ✅ GOOD: Channel-Based SSE (pkg/client/client.go:71-72)
```go
events:    make(chan *SignalEvent, 10),
tasks:     make(chan *TaskEvent, 10),
```
**Verdict:** Buffered channels prevent blocking. Good size (10).

#### ⚠️ WARNING: Context Handling (client.go:98-99)
```go
select {
case <-c.stop:
    return
default:
    if err := c.connectOnce(); err != nil {
```
**Recommendation:** Use `context.Context` instead of custom `stop chan`:
```go
func (c *Client) Connect(ctx context.Context) error {
    go func() {
        for {
            select {
            case <-ctx.Done():
                return
            default:
                if err := c.connectOnce(); err != nil { /*...*/ }
            }
        }
    }()
}
```

---

### 3. Error Handling Strategy

#### ✅ GOOD: Wrapped Errors (daemon.go:117-119)
```go
if err := validateEnvironmentPort(env, port); err != nil {
    return fmt.Errorf("environment validation failed: %w", err)
}
```
**Verdict:** Proper error wrapping with `%w` for error chains.

#### ❌ BAD: Silent Failures (commmonitor/monitor.go:167-172)
```go
func (m *Monitor) checkAndCorrectSignals(correctedIDs map[string]bool) {
    signals, err := m.getSignalsNeedingCorrection()
    if err != nil {
        m.logger.Error("Failed to fetch signals", "error", err)
        return  // Error logged but no alert/metric
    }
```
**Problem:** Persistent DB failures might go unnoticed for hours.

**Fix:** Add metrics/alerting:
```go
if err != nil {
    m.logger.Error("Failed to fetch signals", "error", err)
    m.mu.Lock()
    m.errorCount++
    m.lastError = err
    m.mu.Unlock()
    return
}

// In Stats():
"error_count": m.errorCount,
"last_error": m.lastError.Error(),
```

---

### 4. Resource Management

#### ✅ GOOD: Deferred Cleanup (daemon.go:105)
```go
if err := d.writePID(); err != nil { /*...*/ }
defer d.removePID()
```
**Verdict:** Proper use of defer for cleanup.

#### ✅ GOOD: HTTP Client Config (client.go:74-80)
```go
httpClient: &http.Client{
    Transport: &http.Transport{
        MaxIdleConns:      10,
        IdleConnTimeout:   30 * time.Second,
        DisableKeepAlives: true,
    },
    Timeout: 0, // No timeout for SSE
},
```
**Verdict:** SSE requires no timeout, DisableKeepAlives prevents reuse issues.

#### ⚠️ WARNING: Goroutine Leaks (client.go:95-108)
```go
func (c *Client) Connect() error {
    go func() {
        for {
            select {
            case <-c.stop:
                return
            default:
                if err := c.connectOnce(); err != nil {
                    c.logger.Error("Connection error", "error", err)
                    time.Sleep(3 * time.Second)
                }
            }
        }
    }()
    return nil
}
```
**Recommendation:** Add WaitGroup for tracking:
```go
type Client struct {
    // ...
    wg sync.WaitGroup
}

func (c *Client) Connect() error {
    c.wg.Add(1)
    go func() {
        defer c.wg.Done()
        // ...
    }()
}

func (c *Client) Close() {
    close(c.stop)
    c.wg.Wait() // Ensure goroutine exits
}
```

---

### 5. Configuration Management

#### ✅ GOOD: YAML Config (config/config.yaml)
```yaml
flip2:
  pocketbase:
    port: 8090
  sync:
    enabled: true
```
**Verdict:** Clean, hierarchical configuration.

#### ❌ BAD: Hardcoded Agent List (commmonitor/monitor.go:20-29)
```go
var ValidAgents = map[string]bool{
    "Claud-win":    true,
    "claude-mac":   true,
    // ... hardcoded list
}
```
**Problem:** Adding a new agent requires recompiling.

**Fix:** Move to config.yaml:
```yaml
flip2:
  agents:
    valid_agents:
      - claud-win
      - claude-mac
      - ag-win
      - antigravity
    typo_corrections:
      claude-win: claud-win
      claudwin: claud-win
```

---

### 6. Performance & Efficiency

#### ❌ CRITICAL: Polling Anti-Pattern (commmonitor/monitor.go:150-164)
```go
func (m *Monitor) monitorLoop() {
    ticker := time.NewTicker(m.config.PollInterval) // 10 seconds
    defer ticker.Stop()

    for {
        select {
        case <-m.ctx.Done():
            return
        case <-ticker.C:
            m.checkAndCorrectSignals(correctedIDs)
        }
    }
}
```
**Problem:**
- 10-second latency for typo corrections
- Unnecessary DB queries every 10s (creates log spam)
- SSE client exists but not used (client.go)

**Fix:** Migrate to SSE (see section 8)

---

## 7. Comparison: Gemini vs Claude Reviews

| Issue | Gemini Found | Claude Found | Severity |
|-------|--------------|--------------|----------|
| Unbounded memory load | ✅ Critical | ✅ Critical | 🔴 **CRITICAL** |
| Polling anti-pattern | ✅ High | ✅ High | 🟠 **HIGH** |
| Missing UpdateSignal() | ✅ High | ✅ High | 🟠 **HIGH** |
| Silent error handling | ✅ Medium | ✅ Medium | 🟡 **MEDIUM** |
| Hardcoded config | ✅ Medium | ✅ Medium | 🟡 **MEDIUM** |
| Goroutine leak risk | ❌ Not found | ✅ Warning | 🟡 **MEDIUM** |
| Context vs stop chan | ❌ Not found | ✅ Warning | 🟢 **LOW** |
| Supervisor pattern | ❌ Not reviewed | ✅ Excellent | ✅ **GOOD** |
| Environment validation | ❌ Not reviewed | ✅ Excellent | ✅ **GOOD** |

**Verdict:** Gemini excelled at focused commmonitor review. Claude provided broader architectural analysis.

---

## 8. Migration Plan: Polling → SSE

### Step 1: Add UpdateSignal to Client (HIGH)
```go
// In pkg/client/client.go
func (c *Client) UpdateSignal(id string, data map[string]interface{}) error {
    jsonData, _ := json.Marshal(data)
    req, _ := http.NewRequest("PATCH",
        fmt.Sprintf("%s/api/collections/signals/records/%s", c.BaseURL, id),
        bytes.NewBuffer(jsonData))
    req.Header.Set("Content-Type", "application/json")
    c.setAuthHeaders(req)

    resp, err := c.httpClient.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode >= 400 {
        return fmt.Errorf("api error: %s", resp.Status)
    }
    return nil
}
```

### Step 2: Rewrite Monitor to Use SSE (HIGH)
```go
// In internal/commmonitor/monitor.go
type Monitor struct {
    client      *client.Client  // Replace: pb *pocketbase.PocketBase
    config      Config
    logger      *slog.Logger
    // ... rest
}

func New(baseURL, apiKey string, config Config, logger *slog.Logger) *Monitor {
    return &Monitor{
        client: client.New(baseURL, "comm-monitor", apiKey, "", logger),
        config: config,
        logger: logger,
    }
}

func (m *Monitor) Start() {
    m.client.Connect()
    m.wg.Add(1)
    go m.consumeSignals()
}

func (m *Monitor) consumeSignals() {
    defer m.wg.Done()

    for {
        select {
        case <-m.ctx.Done():
            return
        case evt := <-m.client.Signals():
            if evt.Action == "create" || evt.Action == "update" {
                m.processSignal(evt.Record)
            }
        }
    }
}

func (m *Monitor) processSignal(sig SignalRecord) {
    needsUpdate := false
    updates := make(map[string]interface{})

    // Check from_agent
    if sig.FromAgent != "" && !ValidAgents[strings.ToLower(sig.FromAgent)] {
        if corrected := m.fuzzyMatchAgent(sig.FromAgent); corrected != "" {
            updates["from_agent"] = corrected
            needsUpdate = true
        }
    }

    // Check to_agent
    if sig.ToAgent != "" && !ValidAgents[strings.ToLower(sig.ToAgent)] {
        if corrected := m.fuzzyMatchAgent(sig.ToAgent); corrected != "" {
            updates["to_agent"] = corrected
            needsUpdate = true
        }
    }

    if needsUpdate {
        if err := m.client.UpdateSignal(sig.ID, updates); err != nil {
            m.logger.Error("Failed to update signal", "error", err)
        }
    }
}
```

**Benefits:**
- ✅ Real-time corrections (no 10s delay)
- ✅ 90% reduction in log spam (no polling)
- ✅ Lower DB load
- ✅ Can run as standalone agent (not tied to PocketBase server)

---

## 9. Recommended Fixes (Priority Order)

### Immediate (Do Now):
1. ✅ **Fix unbounded memory load** - Remove `FindAllRecords` fallback
2. ✅ **Add client.UpdateSignal()** - Needed for SSE migration
3. ✅ **Migrate commmonitor to SSE** - Eliminate polling

### This Week:
4. ⚠️ **Add error metrics** - Track silent failures
5. ⚠️ **Move ValidAgents to config** - Allow runtime agent registration
6. ⚠️ **Add WaitGroup to client** - Prevent goroutine leaks

### This Month:
7. 🔵 **Context.Context adoption** - Replace custom stop channels
8. 🔵 **Monitoring dashboard** - Expose commmonitor.Stats() endpoint
9. 🔵 **Integration tests** - Test SSE failure scenarios

---

## 10. Build & Test Plan

### Build Steps:
```bash
cd /Users/arielspivakovsky/src/flip/flip2

# 1. Add UpdateSignal to client
# (Edit pkg/client/client.go)

# 2. Fix memory leak in commmonitor
# (Edit internal/commmonitor/monitor.go)

# 3. Migrate to SSE
# (Rewrite internal/commmonitor/monitor.go)

# 4. Rebuild binaries
go build -o flip2d ./cmd/flip2d
GOOS=windows GOARCH=amd64 go build -o flip2d-win.exe ./cmd/flip2d

# 5. Test on Mac
./flip2d --config config/config-test.yaml

# 6. Verify no polling in logs
tail -f /tmp/flip2d-test.log | grep -E "(Cycle complete|polling)"

# 7. Test typo correction
curl -X POST http://localhost:9190/api/collections/signals/records \
  -H "X-API-Key: flip2_secret_key_123" \
  -H "Content-Type: application/json" \
  -d '{"from_agent": "cluade-mac", "to_agent": "gemini", "content": "test"}'

# 8. Verify immediate correction (no 10s delay)
curl http://localhost:9190/api/collections/signals/records \
  -H "X-API-Key: flip2_secret_key_123"
```

---

## 11. Final Recommendations

### Architecture Grade: A-
- Excellent supervision pattern
- Good separation of concerns
- Clean dependency injection

### Code Quality Grade: B+
- Well-structured, readable code
- Good error wrapping
- Some resource leak risks

### Performance Grade: C
- Polling anti-pattern hurts performance
- Memory leak risk with current fallback

### Overall: B+ → A (after fixes)
With the recommended fixes, FLIP2 will be production-ready at scale.

---

**Next Actions:**
1. Implement fixes (estimated 2-3 hours)
2. Build and test
3. Deploy to Mac test server (port 9190)
4. Verify fixes work
5. Deploy to production (port 8090)
6. Update documentation

---

**Review Completed:** 2025-12-31
**Reviewers:** Claude Sonnet 4.5, Gemini Flash
**Status:** Ready for implementation
