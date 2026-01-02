# Persistent AG Orchestrator Solution

**Generated**: 2026-01-02 16:05 EST
**Problem**: AG orchestrators go OFFLINE immediately after spawning
**Solution**: Use `flip2 agent listen` to create persistent event loops

---

## The Problem

### What We Did (WRONG)
```bash
./flip2 agent spawn --role researcher --task "You are AG orchestrator..."
```

**What this does**:
1. ✅ Registers agent in database
2. ✅ Executes task ONCE using LLM
3. ❌ Agent goes OFFLINE after single execution
4. ❌ No persistent loop
5. ❌ No signal monitoring
6. ❌ No autonomous behavior

**Result**: AG writes files once, then disappears

---

## The Solution

### Two-Step Process for Persistent AG

#### Step 1: Spawn the AG (Database Registration)
```bash
./flip2 agent spawn --role researcher --task "Initial AG setup task"
# This creates: researcher-858f00385f0f
```

#### Step 2: Start Persistent Listen Loop
```bash
./flip2 agent listen researcher-858f00385f0f --api http://localhost:8090 &
```

**What `agent listen` does**:
1. ✅ **Connects via SSE** - Real-time signal reception
2. ✅ **Heartbeat loop** - Updates last_seen every 30 seconds
3. ✅ **Signal event loop** - Processes incoming signals
4. ✅ **Task event loop** - Handles assigned tasks
5. ✅ **Stays online** - Runs indefinitely until killed
6. ✅ **Concurrent tasks** - Can handle multiple tasks (configurable)

---

## Complete AG Orchestrator Workflow

### Option A: Spawn + Listen Pattern (Recommended)

```bash
# 1. Spawn AG with initial instructions
AG_ID=$(./flip2 agent spawn --api http://localhost:8090 --role researcher \
  --task "You are AG Orchestrator. Read COORDINATOR_TO_AG_COMMANDS.md and begin orchestration." \
  2>&1 | grep agent_id= | sed 's/.*agent_id=//' | awk '{print $1}')

# 2. Wait for initial execution (1-2 minutes)
sleep 120

# 3. Start persistent listen mode (background daemon)
nohup ./flip2 agent listen $AG_ID --api http://localhost:8090 --concurrency 3 \
  > logs/ag_orchestrator.log 2>&1 &

echo "AG Orchestrator running in background: $AG_ID"
```

### Option B: Pre-registered AG (Manual)

```bash
# 1. Manually create AG entry with specific ID
./flip2 agent add researcher-ag-orchestrator --role researcher --api http://localhost:8090

# 2. Start listen mode immediately
nohup ./flip2 agent listen researcher-ag-orchestrator \
  --api http://localhost:8090 \
  --concurrency 5 \
  > logs/ag_orchestrator.log 2>&1 &

# 3. Send initial task via signal
./flip2 signal send researcher-ag-orchestrator task \
  "Read COORDINATOR_TO_AG_COMMANDS.md and begin orchestration" \
  --api http://localhost:8090
```

---

## Listen Mode Features

### Command Line Options

```bash
flip2 agent listen <agent_id> [flags]

Flags:
  --concurrency int   Max concurrent tasks (default 1)
  --api-key string    API Key for authentication
  --api string        FLIP2 API URL (default "https://localhost:8090")
```

### What Happens in Listen Mode

```
┌─────────────────────────────────────────┐
│   flip2 agent listen researcher-ag      │
│                                         │
│  ┌────────────────────────────────────┐│
│  │ 1. Heartbeat Loop (every 30s)     ││
│  │    - Update last_seen             ││
│  │    - Keep agent status = "online" ││
│  └────────────────────────────────────┘│
│                                         │
│  ┌────────────────────────────────────┐│
│  │ 2. Signal Event Loop (SSE)        ││
│  │    - Listen for incoming signals  ││
│  │    - Generate LLM reply           ││
│  │    - Send response signal         ││
│  │    - Mark signal as read          ││
│  └────────────────────────────────────┘│
│                                         │
│  ┌────────────────────────────────────┐│
│  │ 3. Task Event Loop (SSE)          ││
│  │    - Listen for task assignments  ││
│  │    - Mark task in_progress        ││
│  │    - Execute with LLM             ││
│  │    - Update task to completed     ││
│  └────────────────────────────────────┘│
│                                         │
│  ┌────────────────────────────────────┐│
│  │ 4. Polling Fallback (every 3s)    ││
│  │    - Poll for unread signals      ││
│  │    - Process if SSE fails         ││
│  └────────────────────────────────────┘│
└─────────────────────────────────────────┘
```

---

## AG Orchestrator Specific Setup

### For Our FLIP2 Project

```bash
#!/bin/bash
# File: spawn_persistent_ag.sh

set -e

API_URL="http://localhost:8090"
LOG_DIR="/Users/arielspivakovsky/src/flip/flip2/logs"
mkdir -p "$LOG_DIR"

echo "=== Spawning AG Orchestrator ==="

# Spawn AG with initial task
SPAWN_OUTPUT=$(./flip2 agent spawn --api $API_URL --role researcher --task "
You are the AG Orchestrator for FLIP2 implementation.

INITIAL SETUP:
1. Read /Users/arielspivakovsky/src/flip/flip2/COORDINATOR_TO_AG_COMMANDS.md
2. Write acknowledgment to AG_STATUS_UPDATES.md
3. Assess current project status (check for Batch 6 completion)
4. Prepare to spawn workers as instructed

You are now entering PERSISTENT MODE. You will receive ongoing instructions
via signals and task assignments. Monitor COORDINATOR_TO_AG_COMMANDS.md for
updates and spawn workers autonomously.
" 2>&1)

# Extract agent ID
AG_ID=$(echo "$SPAWN_OUTPUT" | grep -o 'researcher-[a-f0-9]*' | head -1)

if [ -z "$AG_ID" ]; then
    echo "ERROR: Failed to extract agent ID"
    echo "$SPAWN_OUTPUT"
    exit 1
fi

echo "AG Spawned: $AG_ID"

# Wait for initial execution
echo "Waiting 90 seconds for initial setup..."
sleep 90

# Start persistent listen mode
echo "Starting persistent listen mode..."
nohup ./flip2 agent listen $AG_ID \
    --api $API_URL \
    --concurrency 3 \
    > "$LOG_DIR/ag_${AG_ID}.log" 2>&1 &

LISTEN_PID=$!

echo "=== AG Orchestrator Running ==="
echo "Agent ID: $AG_ID"
echo "Listen PID: $LISTEN_PID"
echo "Log file: $LOG_DIR/ag_${AG_ID}.log"
echo ""
echo "To monitor: tail -f $LOG_DIR/ag_${AG_ID}.log"
echo "To stop: kill $LISTEN_PID"

# Save PID for later management
echo "$LISTEN_PID" > "$LOG_DIR/ag_orchestrator.pid"
```

---

## Why Previous AGs Failed

### researcher-361db2f46164 (Original AG)
**Timeline**:
- 07:31 - Spawned with `agent spawn`
- 07:31-07:50 - Executed task once, wrote to files
- 07:50 - Task completed, went OFFLINE
- 08:00+ - No listen mode started = **DEAD**

**Problem**: We never ran `agent listen` on it!

### researcher-858f00385f0f (Backup AG)
**Timeline**:
- 15:50 - Spawned with `agent spawn`
- 15:50 - Executed task once
- 15:50 - Task completed, went OFFLINE immediately
- 16:00+ - No listen mode started = **DEAD**

**Problem**: Same issue - no `agent listen`!

---

## Correct Architecture for AG Orchestrators

### Production Setup

```yaml
# docker-compose.yml or systemd service
services:
  flip2-daemon:
    command: flip2 start
    restart: always

  ag-orchestrator:
    command: flip2 agent listen researcher-ag-primary --concurrency 5
    restart: always
    depends_on:
      - flip2-daemon
    environment:
      FLIP2_API_URL: http://flip2-daemon:8090

  ag-orchestrator-backup:
    command: flip2 agent listen researcher-ag-backup --concurrency 5
    restart: always
    depends_on:
      - flip2-daemon
```

---

## Communication Flow with Persistent AG

### How Claude Coordinator Interacts

```
┌──────────────────┐                    ┌──────────────────┐
│ Claude           │                    │ AG Orchestrator  │
│ Coordinator      │                    │ (Listen Mode)    │
└────────┬─────────┘                    └────────┬─────────┘
         │                                       │
         │ 1. Update command file                │
         ├──────────────────────────────────────>│
         │   COORDINATOR_TO_AG_COMMANDS.md       │
         │                                       │
         │ 2. Send signal                        │
         ├──────────────────────────────────────>│
         │   flip2 signal send ag-id ping "..."  │
         │                                       │
         │                                  3. AG receives
         │                                     signal (SSE)
         │                                       │
         │                                  4. AG reads
         │                                     command file
         │                                       │
         │                                  5. AG spawns
         │                                     workers
         │                                       │
         │ 6. AG writes status <────────────────┤
         │   AG_STATUS_UPDATES.md                │
         │                                       │
         │ 7. Read status file                   │
         │<──────────────────────────────────────│
         │                                       │
```

---

## Testing Persistent AG

### Verify It's Working

```bash
# 1. Check agent status
./flip2 agent list --api http://localhost:8090
# Should show status=online (not offline!)

# 2. Check heartbeats (last_seen should update every 30s)
watch -n 5 './flip2 agent list --api http://localhost:8090'

# 3. Send test signal
./flip2 signal send $AG_ID ping "Status check" --api http://localhost:8090

# 4. Monitor AG log for response
tail -f logs/ag_*.log

# 5. Check for autonomous activity
ls -lt *.md | head -5
# Should see AG_STATUS_UPDATES.md being updated
```

---

## Recommendation for FLIP2

### Immediate Action

1. **Don't waste time on old AGs** - They're dead, leave them
2. **Spawn new AG with 2-step process**:
   - Spawn to register
   - Listen to persist
3. **Monitor for 5 minutes** to confirm it's working
4. **If successful**, use this pattern for all future orchestrators

### Long-term Solution

Create a dedicated orchestrator service:
```bash
# /etc/systemd/system/flip2-orchestrator.service
[Unit]
Description=FLIP2 AG Orchestrator
After=flip2-daemon.service

[Service]
ExecStart=/path/to/flip2 agent listen ag-orchestrator --concurrency 5
Restart=always

[Install]
WantedBy=multi-user.target
```

---

**Status**: Solution identified. Ready to implement persistent AG orchestrator.
