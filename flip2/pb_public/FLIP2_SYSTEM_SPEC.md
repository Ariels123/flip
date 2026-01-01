# FLIP2 System Specification & Implementation Plan

**Version:** 2.0
**Date:** 2025-12-13
**Authors:** Claude (Coordinator), Gemini (Antigravity)

---

## 1. Executive Summary

FLIP2 is a federated multi-agent coordination system that enables LLM agents (Claude, Gemini, etc.) to communicate, coordinate tasks, and work autonomously across distributed nodes.

### Current Architecture
```
MAC (192.168.1.53)                    WINDOWS (192.168.1.220)
+------------------+                  +------------------+
| flip2d daemon    |   <-- SYNC -->   | flip2d daemon    |
| Port 8091        |                  | Port 8091        |
| PocketBase DB    |                  | PocketBase DB    |
+------------------+                  +------------------+
        |                                     |
   +---------+                           +---------+
   | Claude  |                           | Claude  |
   | Agent   |                           | Agent   |
   +---------+                           +---------+
```

### Key Goals
1. **Autonomous Operation** - Each daemon operates independently
2. **Peer Sync** - Daemons sync signals/tasks when connected
3. **Agent Execution** - Execute LLM tasks via CLI backends (claude, gemini)
4. **Realtime Communication** - SSE-based event streaming

---

## 2. Current Implementation Status

### IMPLEMENTED

| Component | Status | Location |
|-----------|--------|----------|
| Daemon Core | Working | `internal/daemon/daemon.go` |
| Scheduler (Cron) | Working | `internal/scheduler/scheduler.go` |
| Executor (Task Runner) | Working | `internal/executor/executor.go` |
| CLI Tool | Working | `cmd/flip2/main.go` |
| Database Schema | Working | `pb_migrations/*.go` |
| API Key Auth | Working | Middleware in daemon.go |
| SSE Client | Working | `pkg/client/client.go` |
| Agent Listener | Working | `flip2 agent listen` |
| Signal Send/Receive | Working | Via signals collection |
| Task Assignment | Working | Via tasks collection |
| Heartbeat | Working | 30s interval in agent listen |
| Zombie Task Reaper | Partial | Sort field bug needs fix |

### NOT IMPLEMENTED (TODO)

| Component | Priority | Description |
|-----------|----------|-------------|
| Daemon Sync | HIGH | Peer-to-peer sync between daemons |
| File Transfer | HIGH | Transfer files between agents |
| HTTP Backend | MEDIUM | Execute via HTTP API (antigravity) |
| WebSocket | LOW | Upgrade SSE to WebSocket |
| OAuth Login | LOW | Google OAuth flow in CLI |

---

## 3. Directory Structure

```
flip2/
+-- cmd/
|   +-- flip2/           # CLI binary
|   |   +-- main.go      # CLI entry point (1348 lines)
|   +-- flip2d/          # Daemon binary
|       +-- main.go      # Daemon entry point
|
+-- internal/
|   +-- auth/            # Authentication helpers
|   |   +-- auth.go      # Token storage/retrieval
|   +-- config/          # Configuration loading
|   |   +-- config.go    # YAML config parser
|   +-- daemon/          # Daemon core
|   |   +-- daemon.go    # Main daemon logic (430 lines)
|   +-- executor/        # Task execution
|   |   +-- executor.go  # Process spawning (267 lines)
|   +-- logger/          # Logging setup
|   +-- migrate/         # FLIP v1 migration
|   |   +-- flip1.go
|   +-- scheduler/       # Cron job scheduler
|   |   +-- scheduler.go # (131 lines)
|   +-- version/
|       +-- version.go
|
+-- pb_migrations/       # Database schema
|   +-- 1_initial_schema.go
|   +-- 2_fix_signals_rules.go
|   +-- 3_add_agent_mode.go
|   +-- 4_update_task_rules.go
|   +-- 5_add_task_logs.go
|   +-- 6_add_daemon_log_path_to_agents.go
|   +-- 7_update_auth_model.go
|
+-- pkg/
|   +-- client/
|       +-- client.go    # SSE realtime client
|
+-- config/
|   +-- config.yaml      # Main configuration
|
+-- pb_data/             # PocketBase SQLite data
+-- pb_public/           # Static file serving
|   +-- flip2d.exe       # Windows daemon binary
|   +-- config_win.yaml  # Windows config template
+-- logs/
    +-- WORK_LOG.md      # Development log
```

---

## 4. Database Schema

### Collections

#### agents
| Field | Type | Description |
|-------|------|-------------|
| id | string | PocketBase record ID |
| agent_id | string | Human-readable ID (e.g., "claude", "gemini") |
| status | select | online, offline, busy, idle |
| backend | select | claude, gemini, antigravity, local, custom |
| mode | select | local, remote |
| capabilities | json | Agent capabilities array |
| last_seen | datetime | Last heartbeat timestamp |
| daemon_log_path | string | Path to daemon log file |
| metadata | json | Extra data |

#### tasks
| Field | Type | Description |
|-------|------|-------------|
| id | string | PocketBase record ID |
| task_id | string | Human-readable ID (e.g., "TASK-123") |
| title | string | Task title |
| description | text | Task description/prompt |
| status | select | pending, in_progress, done, failed |
| priority | number | 1-5 (1=highest) |
| assignee | string | Agent record ID |
| depends_on | json | Task dependencies |
| progress | number | 0-100 |
| result | text | Task output |
| stdout_log | text | Standard output |
| stderr_log | text | Standard error |
| retry_count | number | Current retry attempt |
| max_retries | number | Max retry attempts |
| started_at | datetime | Execution start |
| completed_at | datetime | Execution end |

#### signals
| Field | Type | Description |
|-------|------|-------------|
| id | string | PocketBase record ID |
| signal_id | string | Human-readable ID (e.g., "SIG-123") |
| from_agent | string | Sender agent_id |
| to_agent | string | Recipient agent_id |
| signal_type | select | message, ping, task, alert, handoff |
| priority | select | low, normal, high, critical |
| content | text | Signal payload |
| read | bool | Read status |
| read_at | datetime | When read |

#### jobs
| Field | Type | Description |
|-------|------|-------------|
| name | string | Job name |
| description | text | Job description |
| cron | string | 6-field cron expression |
| handler | string | Handler function name |
| enabled | bool | Active status |
| last_run | datetime | Last execution |
| next_run | datetime | Next scheduled run |
| last_result | select | success, failed, running |

#### events
| Field | Type | Description |
|-------|------|-------------|
| event_type | string | Event type |
| agent_id | string | Related agent |
| task_id | string | Related task |
| details | json | Event details |
| cost | number | Cost in dollars |
| tokens | number | Token count |

---

## 5. Configuration

### config/config.yaml
```yaml
flip2:
  daemon:
    pid_file: /tmp/flip2d.pid
    log_file: /tmp/flip2d.log
    log_level: info

  pocketbase:
    host: 0.0.0.0
    port: 8091
    data_dir: ./pb_data

  backends:
    claude:
      command: claude
      args:
        - "-p"
        - "--dangerously-skip-permissions"
        - "--output-format"
        - "text"
      timeout: 300s
      type: process

    gemini:
      command: gemini
      args:
        - "-y"
        - "--output-format"
        - "text"
      timeout: 180s
      type: process

    antigravity:
      type: http
      url: http://localhost:9222
      timeout: 300s

  scheduler:
    timezone: UTC
    max_concurrent_jobs: 4

  executor:
    max_concurrent_tasks: 3
    default_timeout: 300s
    retry_attempts: 2
    retry_delay: 30s
    worker_prefix: |
      You are a WORKER agent in the FLIP2 multi-agent system.
      Complete your assigned task and report results clearly.
      Do NOT spawn additional agents without explicit approval.

  metrics:
    enabled: true
    retention_days: 30

  security:
    admin_email: admin@localhost
    api_keys_enabled: true
    api_key: flip2_secret_key_123
```

### Windows Config (config_win.yaml)
```yaml
flip2:
  daemon:
    pid_file: C:\ProgramData\flip2\flip2d.pid
    log_file: C:\ProgramData\flip2\flip2d.log
    log_level: info

  pocketbase:
    host: 0.0.0.0
    port: 8091
    data_dir: C:\ProgramData\flip2\pb_data

  backends:
    claude:
      command: claude
      args:
        - "-p"
        - "--dangerously-skip-permissions"
        - "--output-format"
        - "text"
      timeout: 300s
      type: process

  # SYNC CONFIG (TODO: Implement)
  sync:
    enabled: true
    peers:
      - url: http://192.168.1.53:8091
        api_key: flip2_secret_key_123
    interval: 30s
    collections:
      - signals
      - tasks

  security:
    api_keys_enabled: true
    api_key: flip2_secret_key_123
```

---

## 6. API Endpoints

### PocketBase Standard
- `GET /api/collections/{collection}/records` - List records
- `POST /api/collections/{collection}/records` - Create record
- `GET /api/collections/{collection}/records/{id}` - Get record
- `PATCH /api/collections/{collection}/records/{id}` - Update record
- `DELETE /api/collections/{collection}/records/{id}` - Delete record

### Custom Endpoints
- `GET /api/health` - Health check
- `GET /api/metrics` - System metrics
- `POST /api/tasks/{id}/signal` - Send signal to running task
- `GET /api/realtime` - SSE event stream

### Authentication
All API calls require either:
- `X-API-Key: flip2_secret_key_123` header
- `Authorization: Bearer <token>` header (from user login)

---

## 7. CLI Commands

```bash
# Daemon Control
flip2 start                    # Start daemon
flip2 stop                     # Stop daemon
flip2 status                   # Show daemon status
flip2 restart                  # Restart daemon

# Agent Management
flip2 agent list               # List all agents
flip2 agent add <id> --backend claude  # Register agent
flip2 agent listen <id>        # Listen for signals/tasks
flip2 agent poll <id>          # Poll for unread signals

# Task Management
flip2 task list                # List all tasks
flip2 task add "Title" --assignee claude  # Create task
flip2 task start <id>          # Mark in_progress
flip2 task done <id>           # Mark done
flip2 task signal <id> SIGTERM # Signal running task

# Signal Management
flip2 signal send <to> <type> <message>  # Send signal

# Auth
flip2 auth login               # Login with email/password
flip2 auth register            # Create account
flip2 auth logout              # Logout

# Utilities
flip2 version                  # Show version
flip2 admin                    # Open PocketBase admin UI
flip2 migrate --from <db>      # Migrate from FLIP v1
```

---

## 8. Implementation Plan: Daemon Sync

### Overview
Each daemon maintains its own PocketBase database. When two daemons connect, they sync their signals and tasks bidirectionally.

### Sync Algorithm
```
Every 30 seconds (configurable):
  For each peer in sync.peers:
    1. Pull remote changes:
       - GET /api/collections/signals/records?filter=(updated > last_sync)
       - For each signal:
         - If not exists locally: CREATE
         - If exists but older: UPDATE

    2. Push local changes:
       - Find local records where updated > last_sync
       - POST/PATCH to peer

    3. Update last_sync timestamp
```

### Conflict Resolution
- **Signals**: Append-only, no conflicts (use signal_id as unique key)
- **Tasks**: Last-write-wins based on `updated` timestamp
- **Agents**: Each daemon owns its local agents (no sync)

### Required Changes

#### New Config Section
```yaml
sync:
  enabled: true
  peers:
    - url: http://192.168.1.53:8091
      api_key: flip2_secret_key_123
      name: mac-daemon
  interval: 30s
  collections:
    - signals
    - tasks
```

#### New Files
1. `internal/sync/sync.go` - Sync logic
2. `internal/sync/peer.go` - Peer connection management

#### New Scheduler Job
```go
d.scheduler.RegisterJob("peer-sync", "*/30 * * * * *", func(ctx context.Context) error {
    return d.syncManager.SyncAll(ctx)
})
```

### API for Sync
```
# Get records updated after timestamp
GET /api/collections/{collection}/records?filter=(updated > '{timestamp}')

# Bulk upsert (custom endpoint)
POST /api/sync/upsert
{
  "collection": "signals",
  "records": [...]
}
```

---

## 9. Implementation Plan: File Transfer

### Option 1: Inline in Signals (for small files)
```json
{
  "signal_type": "file",
  "content": "{\"filename\": \"code.go\", \"data\": \"base64encoded...\", \"encoding\": \"base64\"}"
}
```
- Limit: ~10KB after base64 encoding
- No additional infrastructure needed

### Option 2: File Upload Endpoint
```go
// POST /api/files/upload
// multipart/form-data
// Returns: {"file_id": "abc123", "url": "/api/files/abc123"}

// GET /api/files/{id}
// Returns file content
```

### Option 3: External Storage
- Use S3/GCS bucket shared between daemons
- Store URL references in signals

**Recommendation**: Start with Option 1 for code snippets, implement Option 2 for larger files.

---

## 10. Known Issues & Fixes Needed

### 1. Zombie Reaper Sort Field (daemon.go:334)
```go
// BROKEN: "created" is not valid sort field
records, err := d.pb.FindRecordsByFilter("tasks", "status = 'in_progress'", "created", 100, 0)

// FIX: Use empty string or remove sort
records, err := d.pb.FindRecordsByFilter("tasks", "status = 'in_progress'", "", 100, 0)
```

### 2. Hardcoded from_agent in CLI (main.go:774)
```go
// BROKEN: Always sends as "gemini"
"from_agent": "gemini",

// FIX: Add --from flag or detect current agent
```

### 3. setupLogCapture in CLI (main.go:47-106)
- Should only be in daemon, not CLI
- Remove from cmd/flip2/main.go

### 4. Windows Cross-Compilation
- Removed `syscall.SysProcAttr{Setsid: true}` for Windows
- Daemonization doesn't work on Windows (needs Windows service)

---

## 11. Build Instructions

### Mac/Linux
```bash
cd flip2

# Build CLI
go build -o flip2 ./cmd/flip2

# Build Daemon
go build -o flip2d ./cmd/flip2d
```

### Windows (Cross-compile from Mac)
```bash
GOOS=windows GOARCH=amd64 go build -mod=mod -o pb_public/flip2d.exe ./cmd/flip2d
GOOS=windows GOARCH=amd64 go build -mod=mod -o pb_public/flip2.exe ./cmd/flip2
```

### Running
```bash
# Start daemon
./flip2d

# Or via CLI
./flip2 start

# Agent listener
./flip2 agent listen claude --api-key flip2_secret_key_123
```

---

## 12. Source Code Reference

### Core Files

#### internal/daemon/daemon.go (430 lines)
- `type Daemon struct` - Main daemon struct
- `func New()` - Constructor
- `func (d *Daemon) Start()` - Start daemon
- `func (d *Daemon) registerHooks()` - PocketBase hooks
- `func (d *Daemon) registerJobs()` - Scheduled jobs

#### internal/executor/executor.go (267 lines)
- `type Executor struct` - Task executor
- `func (e *Executor) QueueTask()` - Queue task for execution
- `func (e *Executor) processTask()` - Execute task
- `func (e *Executor) executeProcess()` - Run CLI command
- `func (e *Executor) SignalTask()` - Send signal to process

#### internal/scheduler/scheduler.go (131 lines)
- `type Scheduler struct` - Cron scheduler
- `func (s *Scheduler) RegisterJob()` - Add job
- `func (s *Scheduler) Start()` - Start scheduler
- `func (s *Scheduler) runJob()` - Execute job

#### cmd/flip2/main.go (1348 lines)
- `func main()` - CLI entry
- `func agentCmd()` - Agent subcommands
- `func taskCmd()` - Task subcommands
- `func signalCmd()` - Signal subcommands
- `func generateReply()` - Generate LLM response

#### pkg/client/client.go
- `type Client struct` - SSE client
- `func (c *Client) Connect()` - Connect to SSE
- `func (c *Client) Signals()` - Signal event channel
- `func (c *Client) Tasks()` - Task event channel

---

## 13. Task for Claude-Win

### Primary Task: Implement Daemon Sync

1. **Create `internal/sync/sync.go`**
   - SyncManager struct
   - SyncAll() method
   - Pull/Push logic

2. **Create `internal/sync/peer.go`**
   - Peer connection struct
   - HTTP client with auth

3. **Update config.go**
   - Add sync config section

4. **Update daemon.go**
   - Initialize SyncManager
   - Register sync job

5. **Test sync between Mac and Windows daemons**

### Secondary Task: Set Up Windows Daemon

1. Download `flip2d.exe` from Mac: `http://192.168.1.53:8092/flip2d.exe`
2. Create config at `C:\ProgramData\flip2\config.yaml`
3. Initialize database: run flip2d once
4. Start daemon: `flip2d.exe`
5. Verify API: `curl http://localhost:8091/api/health`

### Deliverable

Send the sync implementation code back via signals. Use multiple signals if needed (split by file).

```
Signal 1: sync.go (part 1)
Signal 2: sync.go (part 2)
Signal 3: peer.go
Signal 4: config changes
```

---

## 14. Contact & Coordination

- **Mac Daemon**: http://192.168.1.53:8091
- **API Key**: flip2_secret_key_123
- **Signal to Claude (Mac)**: `flip2 signal send claude message "your message"`
- **Signal to Claude-Win**: `flip2 signal send Claud-win message "your message"`

---

*Document generated by FLIP2 Coordinator (Claude) on 2025-12-13*
