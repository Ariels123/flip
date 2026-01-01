# FLIP2 Implementation Instructions

**For: Gemini Worker Agent**
**Task: Implement FLIP2 daemon and CLI**
**Date: December 13, 2025**

---

## Context

You are implementing FLIP2, a rewrite of the FLIP multi-agent coordination system. The architecture has been designed and scaffolding created. Your job is to complete the implementation.

**Key Files to Read First:**
1. `/Users/arielspivakovsky/src/flip/ProjectDocs/architecture/flip2_architecture.md` - Full architecture design
2. `/Users/arielspivakovsky/src/flip/flip2/README.md` - Project overview
3. `/Users/arielspivakovsky/src/flip/flip2/config/config.yaml.example` - Configuration structure

**Working Directory:** `/Users/arielspivakovsky/src/flip/flip2`

---

## Current State

### Files Already Created:
```
flip2/
├── cmd/
│   ├── flip2/main.go        # CLI - basic structure, needs completion
│   └── flip2d/main.go       # Daemon - basic structure, needs completion
├── internal/                 # EMPTY - needs implementation
│   ├── daemon/
│   ├── scheduler/
│   ├── executor/
│   ├── api/
│   └── migrate/
├── pb_migrations/
│   └── 1_initial_schema.go  # PocketBase schema - complete
├── config/
│   └── config.yaml.example  # Config template - complete
├── scripts/                  # EMPTY - needs service files
├── go.mod                    # Basic - needs go mod tidy
└── README.md                 # Complete
```

---

## Implementation Tasks

### Task 1: Fix and Complete go.mod

```bash
cd /Users/arielspivakovsky/src/flip/flip2
go mod tidy
```

If there are import errors, add missing dependencies.

### Task 2: Implement internal/daemon/daemon.go

Create a proper daemon implementation with:

```go
// internal/daemon/daemon.go
package daemon

// Daemon manages the FLIP2 service lifecycle
type Daemon struct {
    config    *Config
    pb        *pocketbase.PocketBase
    scheduler *scheduler.Scheduler
    executor  *executor.Executor
    pidFile   string
    logger    *slog.Logger
}

// Required methods:
// - New(configPath string) (*Daemon, error)
// - Start() error
// - Stop() error
// - Reload() error (for SIGHUP)
// - Status() *Status
```

Features needed:
- PID file management (`/tmp/flip2d.pid` for dev, configurable for prod)
- Signal handling (SIGTERM, SIGINT, SIGHUP)
- Graceful shutdown
- Config file loading (YAML)
- Embedded PocketBase server

### Task 3: Implement internal/scheduler/scheduler.go

Cron-based job scheduler:

```go
// internal/scheduler/scheduler.go
package scheduler

// Scheduler runs periodic jobs
type Scheduler struct {
    jobs    map[string]*Job
    cron    *cron.Cron
    pb      *pocketbase.PocketBase
    logger  *slog.Logger
}

// Job represents a scheduled task
type Job struct {
    ID          string
    Name        string
    Cron        string
    Handler     func(context.Context) error
    Enabled     bool
    LastRun     time.Time
    NextRun     time.Time
    LastResult  string
}
```

Built-in jobs to implement:
1. `health-check` - Check system health (1m)
2. `task-executor` - Execute assigned tasks (30s)
3. `agent-heartbeat` - Mark stale agents offline (2m)
4. `task-cleanup` - Remove old completed tasks (1h)

Use `github.com/robfig/cron/v3` for cron scheduling.

### Task 4: Implement internal/executor/executor.go

Task executor that spawns agents:

```go
// internal/executor/executor.go
package executor

// Executor manages task execution via agent spawning
type Executor struct {
    pb          *pocketbase.PocketBase
    config      *Config
    executing   map[string]bool
    mu          sync.Mutex
    logger      *slog.Logger
}

// Backend represents an AI backend configuration
type Backend struct {
    Name    string
    Command string
    Args    []string
    Timeout time.Duration
    Type    string // "process" or "http"
    URL     string // for http type
}
```

Features needed:
- Query PocketBase for tasks with status="in_progress" and assignee set
- Map assignee to backend (claude/gemini/antigravity)
- Spawn process with structured prompt
- Capture output and update task result
- Mark task done/failed
- Concurrent execution limit (default 3)

### Task 5: Implement internal/migrate/flip1.go

Migration from old flip.db:

```go
// internal/migrate/flip1.go
package migrate

// MigrateFromFlip1 imports data from FLIP v1 SQLite database
func MigrateFromFlip1(oldDBPath string, pb *pocketbase.PocketBase) error {
    // 1. Open old SQLite database
    // 2. Read agents table -> create PocketBase agent records
    // 3. Read tasks table -> create PocketBase task records
    // 4. Read signals table -> create PocketBase signal records
    // 5. Read events table -> create PocketBase event records
    // Return count of migrated records
}
```

Old schema (from flip.db):
```sql
-- agents: id, status, capabilities, last_seen, backend, ...
-- tasks: id, title, description, status, assignee, priority, progress, depends_on, ...
-- signals: id, from_agent, to_agent, type, priority, content, read, ...
-- events: type, agent_id, task_id, details, cost, tokens, ...
```

### Task 6: Complete cmd/flip2d/main.go

The daemon entry point needs:
- Config file loading
- Proper daemonization (fork to background)
- Integration with internal/daemon package
- Command-line flags: `--config`, `--foreground`, `--pid-file`

### Task 7: Complete cmd/flip2/main.go

The CLI needs additional commands:
- `flip2 task add <title> --assignee <agent> --priority <1-5>`
- `flip2 task start <id>` - Set status to in_progress
- `flip2 task done <id>` - Set status to done
- `flip2 signal send <to> <type> <message>`
- `flip2 signal read <agent>`
- `flip2 agent spawn <id> <backend> <prompt>`

### Task 8: Create Service Files

**systemd (Linux):** `config/flip2.service`
```ini
[Unit]
Description=FLIP2 Multi-Agent Coordination Daemon
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/flip2d --config /etc/flip2/config.yaml
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

**launchd (macOS):** `config/com.flip.flip2d.plist`
```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "...">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.flip.flip2d</string>
    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/bin/flip2d</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
</dict>
</plist>
```

---

## Dependencies to Add

```go
// go.mod additions needed:
require (
    github.com/pocketbase/pocketbase v0.24.4
    github.com/spf13/cobra v1.8.1
    github.com/robfig/cron/v3 v3.0.1
    gopkg.in/yaml.v3 v3.0.1
    modernc.org/sqlite v1.40.1
)
```

---

## Testing Instructions

After implementation:

```bash
# Build
cd /Users/arielspivakovsky/src/flip/flip2
go build -o flip2 ./cmd/flip2
go build -o flip2d ./cmd/flip2d

# Test daemon in foreground
FLIP2_FOREGROUND=1 ./flip2d

# In another terminal, test CLI
./flip2 status
./flip2 task list
./flip2 admin  # Opens browser to PocketBase admin

# Test migration
./flip2 migrate --from /Users/arielspivakovsky/src/flip/ProjectDocs/LLMcomms/flip.db

# Verify migrated data
./flip2 task list
./flip2 agent list
```

---

## File Writing Instructions

When writing files, use the full absolute path:
- `/Users/arielspivakovsky/src/flip/flip2/internal/daemon/daemon.go`
- `/Users/arielspivakovsky/src/flip/flip2/internal/scheduler/scheduler.go`
- etc.

Create directories if they don't exist.

---

## Priority Order

1. **First**: Get daemon starting with PocketBase (Task 2, 6)
2. **Second**: Implement scheduler (Task 3)
3. **Third**: Implement executor (Task 4)
4. **Fourth**: Implement migration (Task 5)
5. **Fifth**: Complete CLI commands (Task 7)
6. **Last**: Service files (Task 8)

---

## Success Criteria

1. `./flip2d` starts PocketBase on port 8090
2. `./flip2 status` shows daemon running
3. `./flip2 admin` opens admin UI at http://localhost:8090/_/
4. `./flip2 task list` shows tasks from PocketBase
5. `./flip2 migrate --from <path>` imports old data
6. Scheduler runs jobs on schedule
7. Executor spawns agents for in_progress tasks

---

## Important Notes

- Use `log/slog` for structured logging (Go 1.21+)
- PocketBase uses SQLite by default, but supports MySQL via connection string
- The daemon should work both as foreground process and background daemon
- All PocketBase collections are defined in `pb_migrations/1_initial_schema.go`
- Refer to existing FLIP code at `/Users/arielspivakovsky/src/flip/` for patterns

---

## Report Format

After completing each task, report:
1. Files created/modified
2. Any issues encountered
3. Testing results
4. Next steps

When fully done, provide:
1. Summary of all changes
2. How to build and run
3. Any remaining TODOs
