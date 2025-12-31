# FLIP - File-based LLM Inter-Process Communication

## Quick Start for Claude

**You are the COORDINATOR.** When spawning other Claude/Gemini instances, they are WORKERS.

### Your Role as Coordinator
- You manage the FLIP system
- You assign tasks to worker agents
- You monitor progress and handle escalations
- Workers report back to you - you don't report to them

### Essential Commands

```bash
# Start WebSocket server (run first for real-time comms)
./flip ws

# Check system status
./flip status
./flip ws status

# Spawn worker agents
./flip spawn run <agent-id> claude "<prompt>"
./flip spawn run <agent-id> gemini "<prompt>"

# Send tasks to Antigravity (human-in-loop Gemini 2.5 Pro)
./flip antigravity ask "<question>"
./flip antigravity test "<url>" "<instructions>"
./flip antigravity inbox

# Run multi-agent pipelines
./flip pipeline list
./flip pipeline run research "<topic>"
./flip pipeline run data-analyze "<data>"

# Task management
./flip task list
./flip task create "<title>" --priority high
./flip signal send <agent> "<message>"
```

### Spawning Workers - IMPORTANT

When spawning Claude workers, ALWAYS include context that they are workers:

```bash
./flip spawn run worker1 claude "You are a WORKER agent in the FLIP system. Your coordinator is the main Claude instance. Complete this task and report back: <task here>"
```

**Worker context to include:**
- "You are a WORKER agent"
- "Report your results, do not make autonomous decisions"
- "If stuck, signal the coordinator for help"
- "Do not spawn additional agents without coordinator approval"

### Cost Optimization

| Task Type | Assign To | Reason |
|-----------|-----------|--------|
| Data processing, logs, parsing | Gemini | Cheaper, good at bulk |
| Complex reasoning, debugging | Claude | Better accuracy |
| Visual, browser testing | Antigravity | Has GUI access |
| Research + synthesis | Pipeline | Gemini gathers, Claude analyzes |

### File Locations

- Binary: `/Users/arielspivakovsky/src/flip/flip`
- Database: `/Users/arielspivakovsky/src/flip/flip.db`
- Dev Log: `/Users/arielspivakovsky/src/flip/FlipDevLog/dev_log.md`
- This file: `/Users/arielspivakovsky/src/flip/CLAUDE.md`

### Current Version: 5.2

Features:
- WebSocket real-time communication
- Multi-agent pipelines (Gemini→Claude chaining)
- Antigravity integration (human-in-loop)
- Intelligent task routing
- File locking for concurrent access
- Heartbeat watchdog

### Session Persistence & Recovery

**If the terminal session closes:**
1. All messages to you are BUFFERED in the SQLite database
2. Workers continue running independently
3. Antigravity stays connected via WebSocket

**To reconnect as coordinator:**
```bash
cd /Users/arielspivakovsky/src/flip

# Check if WebSocket server is running
./flip ws status

# If not running, start it:
./flip ws &

# Read buffered messages
./flip signal read -a coordinator
./flip signal read -a claude

# Check worker status
./flip status

# Check what tasks are pending
./flip task list
```

**Ensure server persists:**
```bash
# Use the startup script
./scripts/start_flip_server.sh

# Or run with nohup
nohup ./flip ws > logs/ws_server.log 2>&1 &
```

### Message Buffering

All signals/messages are stored in SQLite (`flip.db`), NOT just in memory:
- `signals` table stores all messages
- Messages marked as read but not deleted
- Can query historical signals with `./flip signal list`

Workers automatically buffer their results even if coordinator disconnects.

### DO NOT
- Put FLIP files in client project directories
- Let workers make autonomous decisions
- Spawn agents without clear worker context
- Use Antigravity for tasks that can be automated

### Active Deployments

| Project | Server | Status |
|---------|--------|--------|
| HometownBuzz | 178.156.185.31 | Staging |

Credentials: `/Users/arielspivakovsky/src/flip/credentials/`
