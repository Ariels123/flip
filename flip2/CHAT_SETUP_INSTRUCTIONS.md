# FLIP2 Claude-Gemini Chat Setup Instructions

## For Gemini (Antigravity): Execute These Steps

### Step 1: Build the FLIP2 Binaries

```bash
cd /Users/arielspivakovsky/src/flip/flip2

# Build both binaries
go build -o flip2 ./cmd/flip2
go build -o flip2d ./cmd/flip2d
```

### Step 2: Start the Daemon

```bash
# Start daemon in foreground first to see any errors
FLIP2_FOREGROUND=1 ./flip2d

# If it starts successfully, stop it (Ctrl+C) and run detached:
./flip2 start
```

### Step 3: Verify Daemon is Running

```bash
./flip2 status
```

Expected output:
```
FLIP2 Daemon Status
==================
Status:     running
PID:        <some_pid>
API:        http://localhost:8090
API:        healthy
```

### Step 4: Register Agents

The CLI doesn't have an `agent add` command yet. Use curl to register agents directly:

```bash
# Register Claude agent
curl -X POST http://localhost:8090/api/collections/agents/records \
  -H "Content-Type: application/json" \
  -d '{
    "agent_id": "claude",
    "status": "online",
    "backend": "claude",
    "capabilities": {"reasoning": true, "code": true},
    "metadata": {"role": "coordinator"}
  }'

# Register Gemini agent
curl -X POST http://localhost:8090/api/collections/agents/records \
  -H "Content-Type: application/json" \
  -d '{
    "agent_id": "gemini",
    "status": "online",
    "backend": "gemini",
    "capabilities": {"reasoning": true, "bulk_processing": true},
    "metadata": {"role": "worker"}
  }'
```

### Step 5: Verify Agents Registered

```bash
./flip2 agent list
```

Expected output:
```
AGENT_ID             STATUS               BACKEND
------------------------------------------------------------
claude               online               claude
gemini               online               gemini
```

### Step 6: Create Initial Communication Task

Create a task from Gemini to Claude to initiate the chat:

```bash
./flip2 task add "Chat initiation from Gemini" --assignee claude --priority 1
```

Then create a task from Claude to Gemini:

```bash
./flip2 task add "Chat initiation from Claude" --assignee gemini --priority 1
```

### Step 7: Send a Signal to Start the Conversation

```bash
# Send a message signal from Gemini to Claude
./flip2 signal send claude message "Hello Claude! This is Gemini. The FLIP2 system is now operational. We can communicate via tasks and signals. What would you like me to work on?"
```

### Step 8: Verify Setup

```bash
# List all tasks
./flip2 task list

# Check daemon logs
tail -f /tmp/flip2d.log
```

---

## Communication Protocol

Once set up, Claude and Gemini can communicate via:

### 1. Tasks (for work assignments)
```bash
# Claude assigns work to Gemini
./flip2 task add "Research topic X" --assignee gemini

# Gemini assigns work to Claude
./flip2 task add "Analyze data Y" --assignee claude
```

### 2. Signals (for real-time messages)
```bash
# Signal types: message, ping, task, alert, handoff
./flip2 signal send <agent_id> <type> "<content>"

# Examples:
./flip2 signal send claude message "Status update: completed task"
./flip2 signal send gemini alert "Priority change needed"
./flip2 signal send claude handoff "Taking over task T-123"
```

### 3. Direct API (for custom integrations)
```bash
# Read signals for an agent
curl "http://localhost:8090/api/collections/signals/records?filter=(to_agent='claude')&filter=(read=false)"

# Update task result
curl -X PATCH http://localhost:8090/api/collections/tasks/records/<id> \
  -H "Content-Type: application/json" \
  -d '{"result": "Task completed successfully", "status": "done"}'
```

---

## Troubleshooting

### If daemon fails to start:
```bash
# Check if port 8090 is in use
lsof -i :8090

# Check PID file
cat /tmp/flip2d.pid

# Run in foreground to see errors
FLIP2_FOREGROUND=1 ./flip2d
```

### If agents aren't executing tasks:
- Ensure the daemon has access to `claude` and `gemini` CLI tools
- Check that backends are configured correctly in config/config.yaml
- Verify agent backend matches registered backend name

### Reset everything:
```bash
./flip2 stop
rm -rf pb_data
rm /tmp/flip2d.pid
./flip2 start
# Re-register agents
```

---

## Report Back to Claude

After completing setup, send this signal to confirm:

```bash
./flip2 signal send claude message "FLIP2 setup complete. Daemon running, both agents registered. Ready for chat. Send me a task to verify execution pipeline."
```
