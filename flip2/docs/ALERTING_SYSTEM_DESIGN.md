# FLIP2 Alerting System Design

**Date:** 2025-12-31
**Status:** Design Phase
**Estimated Effort:** 2-3 days (with agent delegation)

---

## Overview

An alerting system to proactively monitor FLIP2 health and notify operators of issues before they become critical.

---

## Architecture

### Components

```
┌─────────────────────────────────────────────────────────────┐
│                      Alert Manager                          │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐     │
│  │ Rule Engine  │→ │ Evaluator    │→ │ Dispatcher   │     │
│  └──────────────┘  └──────────────┘  └──────────────┘     │
│         ↓                  ↓                  ↓             │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐     │
│  │ Alert Rules  │  │ Metric Store │  │ Channels     │     │
│  │ (Config)     │  │ (PocketBase) │  │ (Email/Slack)│     │
│  └──────────────┘  └──────────────┘  └──────────────┘     │
└─────────────────────────────────────────────────────────────┘
```

### Data Model

**PocketBase Collection: `alerts`**
```json
{
  "id": "rec_123",
  "alert_name": "high_error_rate",
  "severity": "warning",
  "message": "Error rate is 12% (threshold: 5%)",
  "metric_value": 0.12,
  "threshold": 0.05,
  "state": "firing",
  "fired_at": "2025-12-31T12:00:00Z",
  "resolved_at": null,
  "notification_sent": true,
  "metadata": {
    "error_count": 120,
    "total_signals": 1000
  }
}
```

**Config: `config/alerts.yaml`**
```yaml
alerts:
  # Database Health
  - name: disk_space_critical
    description: Auxiliary DB exceeds size threshold
    metric: db_size_mb
    threshold: 50
    operator: ">"
    severity: critical
    enabled: true
    channels:
      - email
      - slack

  - name: disk_space_warning
    description: Auxiliary DB approaching size limit
    metric: db_size_mb
    threshold: 30
    operator: ">"
    severity: warning
    enabled: true
    channels:
      - slack

  # Error Rate
  - name: high_error_rate
    description: Error rate exceeds acceptable threshold
    metric: error_rate_percent
    threshold: 5.0
    operator: ">"
    severity: warning
    enabled: true
    channels:
      - slack

  - name: critical_error_rate
    description: Error rate critically high
    metric: error_rate_percent
    threshold: 20.0
    operator: ">"
    severity: critical
    enabled: true
    channels:
      - email
      - slack

  # Sync Health (Mac-Windows)
  - name: sync_failure
    description: Multiple sync failures detected
    metric: sync_failed_count_1h
    threshold: 3
    operator: ">"
    severity: warning
    enabled: false  # Disabled until Windows deployed
    channels:
      - slack

  # System Health
  - name: daemon_down
    description: Health check failed
    metric: health_check_failed
    threshold: 1
    operator: ">="
    severity: critical
    enabled: true
    channels:
      - email
      - slack
    actions:
      - restart_daemon

  - name: high_memory_usage
    description: Memory usage exceeds threshold
    metric: memory_mb
    threshold: 500
    operator: ">"
    severity: warning
    enabled: true
    channels:
      - slack

  # Cost Monitoring
  - name: daily_cost_exceeded
    description: Daily LLM costs exceed budget
    metric: cost_today_usd
    threshold: 10.0
    operator: ">"
    severity: warning
    enabled: true
    channels:
      - slack

notification_channels:
  slack:
    enabled: true
    webhook_url: "${SLACK_WEBHOOK_URL}"
    default_channel: "#flip2-alerts"

  email:
    enabled: true
    smtp_host: "${SMTP_HOST}"
    smtp_port: 587
    from: "flip2@localhost"
    to:
      - "admin@localhost"

evaluation:
  interval: 60s  # How often to evaluate rules
  retention: 7d  # Keep alerts for 7 days
```

---

## Implementation Phases

### Phase 1: Core Infrastructure (Agent: Codex)
**Estimated:** 2 hours

**Deliverables:**
- `internal/alerts/types.go` - Alert types and structures
- `internal/alerts/manager.go` - Alert manager core
- `internal/alerts/store.go` - PocketBase integration
- `pb_migrations/9_add_alerts_collection.go` - DB migration
- `config/alerts.yaml` - Alert configuration

**Features:**
- Alert state management (pending, firing, resolved)
- Alert deduplication (don't re-fire same alert)
- PocketBase collection for alert history
- Basic configuration loading

---

### Phase 2: Rule Engine & Evaluation (Agent: Antigravity)
**Estimated:** 2 hours

**Deliverables:**
- `internal/alerts/evaluator.go` - Metric evaluation engine
- `internal/alerts/rules.go` - Rule loading and validation
- Integration with existing metrics (health, costs, errors)

**Features:**
- Load rules from config
- Evaluate thresholds (>, <, >=, <=, ==)
- State transitions (pending → firing → resolved)
- Severity levels (info, warning, critical)
- Cooldown period (don't spam alerts)

**Metrics Integration:**
```go
// Use existing infrastructure
type MetricProvider interface {
    GetDBSize() float64
    GetErrorRate() float64
    GetSyncFailures() int
    GetMemoryUsage() float64
    GetCostToday() float64
}
```

---

### Phase 3: Notification Channels (Agent: Dashboard)
**Estimated:** 2 hours

**Deliverables:**
- `internal/alerts/channels/slack.go` - Slack notifications
- `internal/alerts/channels/email.go` - Email notifications
- `internal/alerts/dispatcher.go` - Channel dispatcher

**Features:**
- Slack webhook integration
- SMTP email sending
- Template-based messages
- Retry logic for failed sends
- Notification history tracking

**Slack Message Format:**
```json
{
  "text": "🚨 CRITICAL: disk_space_critical",
  "attachments": [{
    "color": "danger",
    "fields": [
      {"title": "Message", "value": "Auxiliary DB exceeds size threshold"},
      {"title": "Current Value", "value": "52.3 MB"},
      {"title": "Threshold", "value": "50 MB"},
      {"title": "Fired At", "value": "2025-12-31 12:00:00"}
    ]
  }]
}
```

---

### Phase 4: Dashboard Integration (Agent: Dashboard)
**Estimated:** 1 hour

**Deliverables:**
- Add alerts panel to dashboard
- Real-time alert updates via PocketBase subscriptions
- Alert history view
- Alert acknowledgement UI

**Features:**
- Live alert feed on dashboard
- Color-coded by severity
- "Acknowledge" button to dismiss
- Alert history timeline

---

### Phase 5: Daemon Integration (Me: Claude)
**Estimated:** 1 hour

**Deliverables:**
- Integrate alert manager into daemon
- Wire up metric providers
- Start evaluation loop
- Testing and verification

---

## Alert States

```
PENDING ──(threshold breached)──> FIRING ──(threshold resolved)──> RESOLVED
    ↑                                 |
    └────────(cooldown expired)───────┘
```

**State Transitions:**
1. **PENDING**: Rule exists but not triggered
2. **FIRING**: Threshold breached, notifications sent
3. **RESOLVED**: Threshold back to normal, resolution notification sent
4. **COOLDOWN**: After resolution, prevent immediate re-firing

---

## Deduplication Logic

```go
// Don't fire same alert multiple times
func (m *Manager) ShouldFire(rule *Rule, value float64) bool {
    // Check if already firing
    if alert := m.GetActiveAlert(rule.Name); alert != nil {
        return false
    }

    // Check cooldown period
    if lastFired := m.GetLastFired(rule.Name); lastFired != nil {
        if time.Since(lastFired.ResolvedAt) < 5*time.Minute {
            return false  // In cooldown
        }
    }

    return true
}
```

---

## Metric Collection

Use existing infrastructure:
- **DB Size**: Query SQLite pragma or use file size
- **Error Rate**: Count signals with level="error" (already in dashboard)
- **Sync Failures**: Track in sync manager
- **Memory**: `runtime.ReadMemStats()`
- **Cost Today**: Query costs collection (already implemented)

---

## Testing Plan

**Unit Tests:**
- Rule evaluation logic
- State transitions
- Deduplication

**Integration Tests:**
- Fire test alert
- Verify notification sent
- Verify alert stored in DB
- Test resolution flow

**Manual Tests:**
- Trigger each alert type
- Verify Slack message received
- Verify email received
- Check dashboard shows alert

---

## Rollout Plan

1. **Phase 1-2**: Core + Rules (no notifications)
   - Deploy, verify alerts firing in DB
   - Test evaluation logic

2. **Phase 3**: Add Slack (start with test channel)
   - Verify notifications working
   - Tune thresholds

3. **Phase 4**: Add Dashboard UI
   - View alerts in real-time
   - Acknowledgement workflow

4. **Phase 5**: Enable Email + Production
   - Add production Slack/email
   - Monitor for false positives
   - Tune cooldown periods

---

## Success Criteria

- ✅ Alerts fire within 60 seconds of threshold breach
- ✅ No duplicate alerts for same condition
- ✅ Notifications delivered reliably (99%+ success)
- ✅ Dashboard shows real-time alerts
- ✅ Alert history queryable
- ✅ Zero false positives for 24 hours

---

## Future Enhancements

**V2 Features:**
- Alert dependencies (only fire if parent fired)
- Scheduled maintenance windows (silence alerts)
- Alert aggregation (group related alerts)
- Anomaly detection (ML-based thresholds)
- Incident management integration
- Alert correlation (find patterns)
- Auto-remediation actions
- Alert routing (different teams for different alerts)

---

**Next Step:** Delegate Phase 1 to Codex Agent
