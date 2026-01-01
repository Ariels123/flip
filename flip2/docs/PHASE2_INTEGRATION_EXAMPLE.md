# Phase 2 Integration Example

This document shows how Phase 2 components work together and how they'll integrate with Phase 5 (daemon).

## Complete Example

```go
package main

import (
    "context"
    "log/slog"
    "os"
    "time"

    "flip2/internal/alerts"
    "github.com/pocketbase/pocketbase"
)

func main() {
    // Setup logger
    logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

    // Initialize PocketBase app (provided by daemon)
    app := pocketbase.New()
    // ... app initialization ...

    // Step 1: Load alert rules from config
    rules, err := alerts.LoadRules("config/alerts.yaml")
    if err != nil {
        logger.Error("Failed to load rules", "error", err)
        return
    }
    logger.Info("Loaded alert rules", "count", len(rules.Alerts))

    // Step 2: Create alert store (Phase 1)
    store := alerts.NewPBAlertStore(app)

    // Step 3: Create alert manager (Phase 1)
    manager := alerts.NewManager(store, logger)

    // Step 4: Create metric provider (Phase 2)
    metrics := alerts.NewPBMetricProvider(app, "./pb_data")

    // Step 5: Create evaluator (Phase 2)
    evaluator, err := alerts.NewEvaluator(manager, rules, metrics, logger)
    if err != nil {
        logger.Error("Failed to create evaluator", "error", err)
        return
    }

    // Step 6: Start evaluation loop
    evaluator.Start()
    logger.Info("Alert evaluator started",
        "interval", rules.Evaluation.Interval)

    // Keep running (in daemon, this would be the main loop)
    // The evaluator runs in background every 60 seconds
    select {}

    // On shutdown:
    // evaluator.Stop()
}
```

## Manual Evaluation Example

```go
// Instead of automatic loop, you can trigger evaluation manually
ctx := context.Background()
if err := evaluator.EvaluateOnce(ctx); err != nil {
    logger.Error("Evaluation failed", "error", err)
}
```

## Testing Metrics Example

```go
// Create metric provider
metrics := alerts.NewPBMetricProvider(app, "./pb_data")

ctx := context.Background()

// Check DB size
dbSize, err := metrics.GetDBSizeMB(ctx)
if err != nil {
    logger.Error("Failed to get DB size", "error", err)
} else {
    logger.Info("DB size", "mb", dbSize)
}

// Check error rate
errorRate, err := metrics.GetErrorRatePercent(ctx)
if err != nil {
    logger.Error("Failed to get error rate", "error", err)
} else {
    logger.Info("Error rate", "percent", errorRate)
}

// Check memory usage
memory, err := metrics.GetMemoryMB(ctx)
if err != nil {
    logger.Error("Failed to get memory", "error", err)
} else {
    logger.Info("Memory usage", "mb", memory)
}

// Check today's cost
cost, err := metrics.GetCostTodayUSD(ctx)
if err != nil {
    logger.Error("Failed to get cost", "error", err)
} else {
    logger.Info("Cost today", "usd", cost)
}
```

## What Happens During Evaluation

### Evaluation Cycle (Every 60 seconds)

1. **For each enabled rule:**
   ```
   disk_space_critical: db_size_mb > 50 MB (critical)
   disk_space_warning: db_size_mb > 30 MB (warning)
   high_error_rate: error_rate_percent > 5.0% (warning)
   critical_error_rate: error_rate_percent > 20.0% (critical)
   daemon_down: health_check_failed >= 1 (critical)
   high_memory_usage: memory_mb > 500 MB (warning)
   daily_cost_exceeded: cost_today_usd > $10 (warning)
   ```

2. **Fetch metric value:**
   ```
   DB size: 45.2 MB
   Error rate: 3.5%
   Memory: 480 MB
   Cost: $8.50
   Health: 0 (healthy)
   ```

3. **Compare against threshold:**
   ```
   disk_space_critical: 45.2 > 50? NO
   disk_space_warning: 45.2 > 30? YES ← BREACH
   high_error_rate: 3.5 > 5.0? NO
   high_memory_usage: 480 > 500? NO
   daily_cost_exceeded: 8.50 > 10? NO
   ```

4. **Fire alerts for breaches (if should fire):**
   ```
   Check: manager.ShouldFire("disk_space_warning")
   - Not already firing? YES
   - Out of cooldown? YES
   → FIRE ALERT

   manager.Fire(ctx, rule, 45.2, "DB size is 45.2 MB (threshold: 30 MB)", {...})
   ```

5. **Resolve alerts if threshold back to normal:**
   ```
   (Next cycle, if DB size drops to 25 MB)

   disk_space_warning: 25 > 30? NO
   Currently firing? YES
   → RESOLVE ALERT

   manager.Resolve(ctx, "disk_space_warning")
   ```

### Logging Output Example

```json
{"time":"2025-12-31T22:00:00Z","level":"INFO","msg":"Starting evaluation cycle","group":"evaluator"}
{"time":"2025-12-31T22:00:00Z","level":"DEBUG","msg":"Evaluated rule","group":"evaluator","rule_name":"disk_space_warning","metric":"db_size_mb","value":45.2,"threshold":30,"operator":">","breached":true}
{"time":"2025-12-31T22:00:00Z","level":"WARN","msg":"Alert FIRED","group":"evaluator","alert_name":"disk_space_warning","severity":"warning","value":45.2,"threshold":30}
{"time":"2025-12-31T22:00:00Z","level":"INFO","msg":"Alert fired","group":"alerts","alert_name":"disk_space_warning","severity":"warning","metric_value":45.2,"threshold":30,"message":"Auxiliary DB approaching size limit: 45.2 MB (threshold: 30 MB)"}
{"time":"2025-12-31T22:00:00Z","level":"INFO","msg":"Evaluation cycle completed","group":"evaluator","errors":0,"alerts_checked":8}
```

## Alert Lifecycle Example

### Scenario: Memory usage spikes

**T=0s:** Memory at 400 MB
- Evaluation: 400 > 500? NO
- State: No alert

**T=60s:** Memory at 550 MB
- Evaluation: 550 > 500? YES
- Action: Fire alert
- State: high_memory_usage FIRING

**T=120s:** Memory still at 550 MB
- Evaluation: 550 > 500? YES
- ShouldFire: Already firing? YES
- Action: None (deduplication)
- State: high_memory_usage FIRING

**T=180s:** Memory drops to 450 MB
- Evaluation: 450 > 500? NO
- Currently firing? YES
- Action: Resolve alert
- State: high_memory_usage RESOLVED

**T=240s:** Memory at 450 MB
- Evaluation: 450 > 500? NO
- Currently firing? NO
- Action: None
- State: No alert

**T=300s:** Memory spikes to 600 MB
- Evaluation: 600 > 500? YES
- ShouldFire: In cooldown? NO (5 min passed)
- Action: Fire alert
- State: high_memory_usage FIRING (new alert)

## Integration with Phase 3 (Notifications)

After Phase 3 is implemented:

```go
// In evaluator.go, after firing alert:
alert, err := e.manager.Fire(ctx, rule, value, message, metadata)
if err != nil {
    return err
}

if alert != nil {
    // NEW: Send notifications (Phase 3)
    for _, channel := range rule.Channels {
        if err := dispatcher.Send(ctx, alert, channel); err != nil {
            e.logger.Error("Failed to send notification",
                "channel", channel,
                "error", err)
        }
    }

    // Mark as sent
    e.manager.MarkNotificationSent(ctx, alert.ID)
}
```

## Integration with Phase 5 (Daemon)

In daemon startup:

```go
// cmd/flip2/main.go

func startDaemon() error {
    // ... existing daemon setup ...

    // Add alerting system
    alertStore := alerts.NewPBAlertStore(app)
    alertManager := alerts.NewManager(alertStore, logger)

    alertRules, err := alerts.LoadRules("config/alerts.yaml")
    if err != nil {
        return fmt.Errorf("failed to load alert rules: %w", err)
    }

    metrics := alerts.NewPBMetricProvider(app, dataPath)
    evaluator, err := alerts.NewEvaluator(alertManager, alertRules, metrics, logger)
    if err != nil {
        return fmt.Errorf("failed to create evaluator: %w", err)
    }

    // Start evaluation loop
    evaluator.Start()
    logger.Info("Alert evaluator started")

    // ... daemon main loop ...

    // On shutdown:
    evaluator.Stop()
    logger.Info("Alert evaluator stopped")

    return nil
}
```

## Success Criteria Met

- ✅ Rules load from config/alerts.yaml
- ✅ Environment variable expansion works
- ✅ All metrics work (except health check placeholder)
- ✅ Threshold comparison logic correct
- ✅ Evaluator runs without crashing
- ✅ Clear logging at each step
- ✅ Integration with Phase 1 manager
- ✅ All tests pass
- ✅ All code compiles

Phase 2 is complete and ready for Phase 3 (notifications)!
