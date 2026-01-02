# FLIP2 System Architecture Review

**Reviewer**: Claude Opus 4.5
**Date**: 2026-01-01
**Purpose**: Investigate why dashboard shows no data despite 98 completed tasks in IMPLEMENTATION_METRICS_2026.md

---

## Executive Summary

The dashboard is empty because **there is a fundamental mismatch between where task completions are tracked and what the dashboard displays**:

1. **The "98 completed tasks"** in IMPLEMENTATION_METRICS_2026.md are **documentation tracking** entries - they represent implementation work logged in a markdown file, NOT actual PocketBase database records
2. **The dashboard displays data from PocketBase** (signals, agents, costs, alerts) but the 98 tasks were tracked manually in markdown, not created as task records in the database
3. **The PocketBase database has only 34 tasks** (10 done, 9 failed, 13 todo, 2 pending), and **0 signals**

---

## Architecture Diagram

```
                                 FLIP2 SYSTEM ARCHITECTURE

+------------------+     +------------------+     +------------------+
|   flip2 CLI      |     |   flip2d Daemon  |     |  External CLIs   |
|  (68k lines)     |     |  (PocketBase)    |     | claude, gemini   |
+--------+---------+     +--------+---------+     +--------+---------+
         |                        |                        |
         |   Create Tasks/        |   OnServe Hooks        |   Subprocess
         |   Register Agents      |   Real-time Events     |   Execution
         v                        v                        v
+------------------------------------------------------------------------+
|                         PocketBase (SQLite)                            |
|                         Port 8090 (HTTPS)                              |
|------------------------------------------------------------------------|
|  Collections:                                                          |
|  - agents (6 records)      - signals (0 records!)                      |
|  - tasks (34 records)      - costs (3 records)                         |
|  - alerts (1 record)       - code_reviews (3 records)                  |
|  - vibescore (2 records)   - signals_archive (archived)                |
+------------------------------------------------------------------------+
         |                        |
         |   REST API             |   Realtime SSE
         v                        v
+------------------+     +------------------+
|  Dashboard       |     |  Signal Monitor  |
|  pb_public/      |     |  Python Script   |
|  index.html      |     +------------------+
|  js/dashboard.js |
+------------------+
         |
         | Fetches from /dashboard/data
         v
+-----------------------------------------------------+
| Dashboard Data Endpoint Returns:                    |
| - agents: 6 records                                 |
| - signals: 0 records  <-- WHY DASHBOARD IS EMPTY   |
| - costs: 3 records                                  |
| - alerts: 0 (filtering for state='firing')         |
| - code_reviews: 3 records                           |
+-----------------------------------------------------+
```

---

## Data Flow Analysis

### How Tasks Should Flow (Design Intent)

```
1. Task Created (CLI or API)
      |
      v
2. Task saved to PocketBase `tasks` collection
      |
      v
3. OnRecordAfterCreateSuccess hook triggers
      |
      v
4. Executor.QueueTask() called
      |
      v
5. Executor spawns subprocess (claude/gemini CLI)
      |
      v
6. Task marked done/failed, result saved
      |
      v
7. Dashboard polls /dashboard/data, displays tasks
```

### What Is Actually Happening

The 98 "completed tasks" in IMPLEMENTATION_METRICS_2026.md are **not created through the FLIP2 task system**. They are:
- Manually logged implementation progress
- External Claude/Opus sessions doing work
- Tracked in markdown, not the database

**Evidence from database:**
```sql
SELECT id, status, COUNT(*) as cnt FROM tasks GROUP BY status;
-- Results:
-- todo: 13
-- done: 10
-- failed: 9
-- pending: 2
-- TOTAL: 34 tasks (NOT 98!)
```

---

## Root Causes Identified

### Issue 1: IMPLEMENTATION_METRICS_2026.md is Manual Tracking (NOT Database)

The file shows "98 completed tasks" but this represents:
- Work done by developers/agents outside FLIP2
- Progress tracked in a markdown file
- NOT tasks created via `./flip2 task create`

**Why this happened**: The IMPLEMENTATION_PLAN work was executed by Claude Code sessions directly, not routed through the FLIP2 task queue system.

### Issue 2: Dashboard Shows Signals, But There Are 0 Signals

The dashboard prominently displays:
- **Recent Signals (last 10)** - but signals table is empty
- **Signal Throughput Chart** - no data
- **Signals Per Minute stat** - shows 0

```sql
SELECT COUNT(*) FROM signals;
-- Result: 0
```

**Why signals are empty**:
1. The `archiver` module moves old signals to `signals_archive`
2. Signals may have been archived (check: `SELECT COUNT(*) FROM signals_archive`)
3. Or no signals were ever created during current testing

### Issue 3: Dashboard Does Not Display Tasks

Looking at `pb_public/js/dashboard.js`:
```javascript
async loadAllData() {
  const response = await fetch('https://localhost:8090/dashboard/data');
  const data = await response.json();

  this.agents = data.agents || [];
  this.signals = data.signals || [];   // DISPLAYED
  this.costs = data.costs || [];       // DISPLAYED
  this.alerts = data.alerts || [];     // DISPLAYED
  this.codeReviews = data.code_reviews || []; // DISPLAYED
  // NOTE: data.tasks is NOT loaded!
}
```

**The dashboard intentionally does not fetch tasks!** It focuses on:
- Agent status
- Signal throughput
- Costs
- Alerts
- Code reviews

The `stats.activeTasks` is fetched separately and only shows count of in_progress tasks.

### Issue 4: The `/dashboard/data` Endpoint Returns Limited Data

From `internal/daemon/daemon.go`:
```go
e.Router.GET("/dashboard/data", func(evt *core.RequestEvent) error {
  // Returns: agents, signals, costs, alerts, code_reviews
  // Does NOT return: tasks, vibescore, signals_archive
})
```

---

## PocketBase's Purpose in This System

PocketBase serves as:

1. **Central Data Store**: SQLite database for all collections
2. **REST API Server**: Provides CRUD operations via `/api/collections/<name>/records`
3. **Real-time Events**: SSE subscriptions for live updates
4. **Admin UI**: Available at `https://localhost:8090/_/`
5. **Authentication**: API key and JWT-based auth
6. **Static File Server**: Serves dashboard from `pb_public/`

### Collections and Their Purposes

| Collection | Purpose | Records |
|------------|---------|---------|
| `agents` | Registered LLM backends and workers | 6 |
| `tasks` | Task queue for worker execution | 34 |
| `signals` | Inter-agent communication messages | 0 |
| `signals_archive` | Archived old signals | ? |
| `costs` | LLM API cost tracking | 3 |
| `alerts` | System health alerts | 1 |
| `code_reviews` | Automated code review results | 3 |
| `vibescore` | Quality evaluation scores | 2 |

---

## Why Dashboard Shows Empty Data

### Primary Reason: Signal Table is Empty

The dashboard's main content sections rely on signals:
- "Recent Signals (last 10)" - empty
- "Signal Throughput (1h)" chart - no data
- "Signals Per Minute" stat - 0

With 0 signals, the dashboard appears mostly empty.

### Secondary Reason: Dashboard Does Not Show Tasks

Even though 34 tasks exist in the database, the dashboard UI:
1. Only shows `stats.activeTasks` count (tasks in "in_progress" status)
2. Does not have a "Recent Tasks" panel
3. Does not display task list or task history

### Tertiary Reason: Limited Cost/Alert Data

- Only 3 cost records exist
- Only 1 alert (not in 'firing' state)
- 3 code reviews (all pending status)

---

## Disconnected Components

### 1. IMPLEMENTATION_METRICS vs Database
- **Disconnected**: Manual markdown tracking vs. PocketBase records
- **Fix**: If tracking matters, create tasks via `./flip2 task create`

### 2. Dashboard vs Task Queue
- **Disconnected**: Dashboard doesn't display task list
- **Fix**: Add task list panel to dashboard OR accept it's a "signals/monitoring" dashboard

### 3. Signal Creation
- **Disconnected**: No signals being generated currently
- **Fix**: Use `./flip2 signal send` or ensure inter-agent communication creates signals

### 4. Alerting System
- **Broken**: `config/alerts.yaml` not found
- **Fix**: Create the alerts configuration file or copy from `config/alerts.yaml.example`

---

## Prioritized Fix Recommendations

### Priority 1: CRITICAL - Create Missing Config File

```bash
# The daemon logs show:
# level=ERROR msg="Failed to load alert rules" error="open config/alerts.yaml: no such file or directory"

cp /Users/arielspivakovsky/src/flip/flip2/config/alerts.yaml.example \
   /Users/arielspivakovsky/src/flip/flip2/config/alerts.yaml
```

### Priority 2: HIGH - Populate Signals for Testing

The dashboard needs signals to show meaningful data:

```bash
# Create test signals
./flip2 signal send claude gemini "Test message for dashboard" --type message
./flip2 signal send gemini claude "Response message" --type message
```

Or programmatically via API.

### Priority 3: MEDIUM - Add Tasks Panel to Dashboard

If you want to see tasks on the dashboard, modify:

1. **daemon.go**: Add tasks to `/dashboard/data` endpoint
```go
if tasks, err := d.pb.FindRecordsByFilter("tasks", "", "-created", 20, 0); err == nil {
    data["tasks"] = tasks
}
```

2. **dashboard.js**: Add tasks to loadAllData
```javascript
this.tasks = data.tasks || [];
```

3. **index.html**: Add tasks panel UI

### Priority 4: LOW - Clarify IMPLEMENTATION_METRICS Purpose

The 98 tasks in IMPLEMENTATION_METRICS_2026.md represent implementation progress tracking, not FLIP2 task queue items. Either:
- A) Rename to `IMPLEMENTATION_PROGRESS_LOG.md` for clarity
- B) Create corresponding FLIP2 tasks if you want them in the dashboard

### Priority 5: LOW - Restart Daemon from flip2/ Directory

The daemon is looking for config in relative paths:
```
level=ERROR msg="Failed to load alert rules" path=config/alerts.yaml
```

Ensure you run from the correct directory:
```bash
cd /Users/arielspivakovsky/src/flip/flip2
./flip2d
```

---

## Configuration Files Summary

| File | Purpose | Status |
|------|---------|--------|
| `config/config.yaml` | Main daemon config | OK |
| `config/alerts.yaml` | Alert rules | MISSING! |
| `config/com.flip.flip2d.plist` | macOS launchd service | OK |
| `config/flip2.service` | Linux systemd service | OK |

---

## Startup Procedure

### Correct Startup Sequence

```bash
# 1. Navigate to flip2 directory
cd /Users/arielspivakovsky/src/flip/flip2

# 2. Ensure config files exist
ls config/alerts.yaml  # Must exist!

# 3. Start daemon (blocks terminal)
./flip2d

# Or start in background
FLIP2_FOREGROUND=1 ./flip2d &

# 4. Verify it's running
curl -k https://localhost:8090/api/health

# 5. Open dashboard
open https://localhost:8090/
```

### Current Running Processes

```
./flip2 agent listen claude --concurrency 1  (since Dec 13)
python3 signal_monitor.py --interval 2        (since Dec 17)
./flip2d                                       (running)
```

---

## Summary

The dashboard appears empty because:

1. **0 signals exist** in the database - the main data the dashboard displays
2. **Tasks are not shown** in the dashboard UI (by design - it's a monitoring dashboard)
3. **The "98 completed tasks"** in IMPLEMENTATION_METRICS are markdown documentation, not database records
4. **Alerting is broken** due to missing config file

To see data on the dashboard:
1. Fix the missing alerts.yaml config
2. Generate signals via CLI or inter-agent communication
3. Optionally modify dashboard to show tasks

---

**Last Updated**: 2026-01-01 21:30
**Reviewed By**: Claude Opus 4.5
