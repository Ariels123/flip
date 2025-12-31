# FLIP Development Log

## Session: 2025-12-10

### v5.0 Implementation Complete

**Completed Features:**

1. **Extended Status System (P0-B)** ✅
   - 8 agent statuses: idle, busy, blocked, testing, review, error, offline, paused
   - New fields: --reason, --detail, --blocked-by, --phase

2. **Signal Priority System (P1-E)** ✅
   - 4 priority levels: critical, high, normal, low
   - Priority-sorted message reading
   - New flags: --priority, --task, --type, --subject, --expire

3. **Heartbeat Watchdog (P0-A)** ✅
   - Detects stale/crashed agents
   - Marks offline, sends CRITICAL signal, reassigns tasks
   - Command: `./flip watchdog --threshold 30`

4. **Handoff Protocol (P1-G)** ✅
   - Formal task ownership transfers
   - Subcommands: create, ack, reject, list, status

---

### v5.1 Autonomous Orchestration System

**Key Insight:** With spawn capability, FLIP can now:
1. **Self-heal** - Detect crashed agents and respawn them
2. **Auto-scale** - Spin up more agents when queue grows
3. **Load balance** - Distribute tasks based on agent capabilities
4. **Coordinate** - Run a "coordinator" agent that manages others

**Agent Hierarchy:**
```
                    ┌─────────────────┐
                    │   COORDINATOR   │  (Claude - strategic decisions)
                    │  (flip orchestrate) │
                    └────────┬────────┘
                             │
         ┌───────────────────┼───────────────────┐
         │                   │                   │
    ┌────▼────┐        ┌────▼────┐        ┌────▼────┐
    │ claude1 │        │ claude2 │        │ gemini1 │
    │ (coding)│        │ (review)│        │(research)│
    └─────────┘        └─────────┘        └─────────┘
```

**Capabilities Matrix:**
| Agent     | Backend | Strengths                    | Weaknesses        |
|-----------|---------|------------------------------|-------------------|
| claude*   | claude  | Coding, debugging, analysis  | Slower            |
| gemini*   | gemini  | Fast research, summarization | Less precise code |
| antigravity| GUI    | Gemini 2.5 Pro, visual tasks | Can't automate    |

**Workflow Improvements:**

1. **Automatic Task Assignment**
   - Coordinator reads queue, analyzes task requirements
   - Matches tasks to best-fit agents based on capabilities
   - Spawns agents on-demand if none available

2. **Self-Healing**
   - Watchdog detects stale agents (no heartbeat)
   - Coordinator respawns crashed agents
   - Reassigns incomplete tasks to new agents

3. **Persistent State**
   - All orchestrator state saved to disk every 30s
   - On crash recovery, resume from last checkpoint
   - Task progress, agent assignments, conversation history

4. **Escalation Path**
   - Agent stuck > 30min → escalate to coordinator
   - Coordinator stuck → alert human (antigravity/manual)
   - Critical failures → pause all, await human intervention

5. **Intelligent Task Routing (Cost Optimization)**
   - Gemini preferred for: simple, moderate, repetitive tasks (cheaper)
   - Claude reserved for: complex, debugging, architecture tasks
   - Auto-escalation: If Gemini fails 2x, escalate to Claude
   - Keyword-based complexity analysis

---

### v5.1 Implementation Complete ✅

**New Commands:**

1. **`flip orchestrate start`** - Run the autonomous orchestration loop
   - Auto-spawns agents when queue grows
   - Recovers crashed/stale agents
   - Assigns tasks to best-fit agents
   - Persists state to disk every 30s

2. **`flip orchestrate status`** - View orchestrator metrics and state

3. **`flip orchestrate queue <task-id>`** - Add task to auto-assignment queue

4. **`flip orchestrate config`** - View configuration

**Intelligent Routing:**

```
Task Complexity → Preferred Backend
─────────────────────────────────────
simple          → Gemini (cheaper, good at bulk data)
moderate        → Gemini (cost savings)
complex         → Claude (thought-intensive)

Gemini Excels At (Simple):
  - Data processing: log, parse, extract, convert, transform, filter
  - Bulk operations: batch, bulk, aggregate, count, sort
  - Format handling: csv, json, template, boilerplate
  - Repetitive: rename, update, format, lint, cleanup
  - Research: summarize, list, find, search, documentation

Claude Reserved For (Complex):
  - Deep reasoning: refactor, architecture, design, algorithm
  - Debugging: debug, investigate, analyze, edge case
  - Concurrency: concurrent, async, race, deadlock
  - System: security, performance, optimize, migrate, memory
```

**Gemini Failure Escalation:**
- If Gemini fails a task 2+ times → auto-escalate to Claude
- Tracks `gemini_failures` per task
- Prevents cost savings from blocking progress

---

### Antigravity Integration (Human-in-the-Loop)

Antigravity is a GUI-based Gemini 2.5 Pro interface that cannot be automated.
However, it can participate in FLIP workflows as a **human-assisted agent**.

**Protocol for Antigravity:**
1. Register as agent: `./flip register antigravity --capabilities "gemini-2.5-pro,visual,manual"`
2. Receive tasks via signals: `./flip signal read -a antigravity`
3. Update status manually: `./flip update antigravity --status busy --task TASK-XXX`
4. Mark completion: `./flip task complete TASK-XXX`
5. Send signals back: `./flip signal send coordinator "TASK-XXX complete" -f antigravity`

**When to Use Antigravity:**
- Tasks requiring Gemini 2.5 Pro capabilities (more powerful than Gemini CLI)
- Visual/multimodal tasks
- Tasks requiring human judgment
- Final verification before deployment
- As escalation target when automated agents fail

**Workflow:**
```
Automated Agents (Claude/Gemini CLI)
        │
        │ (complex/failed tasks)
        ▼
   COORDINATOR
        │
        │ (human-needed)
        ▼
   ANTIGRAVITY (manual)
```

---

### Conflict Prevention & File Locking

**Problem:** Multiple agents editing same files = race conditions, overwrites, corruption

**Solution:** File locking system in orchestrator

**Rules:**
1. **FILE OWNERSHIP**: Each file edited by ONE agent at a time
   - `AcquireFileLock(path, agentID, taskID)` before editing
   - Locks expire after 5 minutes (stale protection)
   - Same agent can refresh their lock

2. **TASK BOUNDARIES**: Tasks specify which files they may modify
   - Orchestrator prevents overlapping file assignments
   - Conflict = CRITICAL signal to coordinator

3. **ATOMIC OPERATIONS**:
   - Write to .tmp file first
   - Rename atomically to target

4. **CONFLICT RESOLUTION**:
   - Conflict detected → HALT conflicting agent
   - Send CRITICAL signal to coordinator
   - Coordinator decides resolution (human-in-loop if needed)

**API:**
```go
AcquireFileLock(filePath, agentID, taskID) error
ReleaseFileLock(filePath, agentID)
ReleaseAllLocks(agentID)
CheckFileLock(filePath) (*FileLock, bool)
GetAllFileLocks() []*FileLock
```

---

### Shell Execution Integration - COMPLETE ✅

**Objective:**
- Examine flipit implementation for shell command execution via goroutines
- Run FLIP server with this functionality
- Spawn shells and communicate with Claude instances
- Test with Claude2.1 designation

**Status:** ✅ COMPLETE

### Implementation Details

Added `spawn` command to FLIP CLI with the following subcommands:

1. **`spawn run <agent-id> <backend> <prompt>`** (Recommended)
   - Spawns a session, sends prompt, returns response in one command
   - Automatically cleans up after completion
   - Example: `./flip spawn run claude2.1 claude "What is 2+2?"`

2. **`spawn repl <agent-id> <backend>`**
   - Interactive REPL mode for continuous conversation
   - Type 'exit' or 'quit' to end session
   - Example: `./flip spawn repl claude2.1 claude`

3. **`spawn start/send/stop/list`**
   - Lower-level session management (for server mode)

### Shell Session Architecture

```
ShellSession struct:
  - ID: unique session identifier
  - AgentID: FLIP agent name (e.g., "claude2.1")
  - Backend: "claude" | "gemini" | "codex"
  - Cmd: *exec.Cmd (bash process)
  - Stdin: io.WriteCloser (input pipe)
  - StdoutChan/StderrChan: buffered output channels
  - Goroutines: 2 readers for non-blocking stdout/stderr
```

### Backend Commands

- **Claude**: `claude -p 'prompt' --dangerously-skip-permissions --output-format text`
- **Gemini**: `gemini -p 'prompt'`

### Test Results

1. **Claude2.1 Test:**
   ```
   $ ./flip spawn run claude2.1 claude "What is 2+2?"
   🚀 Spawning claude session 'claude2.1'...
   📤 Sending prompt to claude2.1...
   📥 Response from claude2.1:
   --------------------------------------------------
   Four
   --------------------------------------------------
   ```

2. **Gemini Test:**
   ```
   $ ./flip spawn run gemini1 gemini "What is 3+3?"
   🚀 Spawning gemini session 'gemini1'...
   📤 Sending prompt to gemini1...
   📥 Response from gemini1:
   --------------------------------------------------
   Six
   --------------------------------------------------
   ```

### Integration with FLIP

- Spawned agents are registered in the FLIP database
- Agent status updates (busy/idle) during prompts
- Events logged to the events table
- Can be monitored via `./flip status` and web dashboard

---

### v5.2 Multi-Agent Pipelines

**Objective:** Chain agents together for complex tasks requiring both data processing AND deep reasoning.

**Use Case:** Tasks that need Gemini's speed/cost-efficiency for preprocessing AND Claude's reasoning for analysis.

**Pipeline Command:**
```bash
# List available pipelines
./flip pipeline list

# Run a pipeline
./flip pipeline run <pipeline-id> "<input>"

# Get pipeline details
./flip pipeline info <pipeline-id>
```

**Built-in Pipeline Templates:**

1. **`data-analyze`** - Data Analysis Pipeline
   - Stage 1: Gemini preprocesses/filters data
   - Stage 2: Claude does deep analysis
   - Use for: Log analysis, data extraction + insights

2. **`plan-execute`** - Plan & Execute Pipeline
   - Stage 1: Claude creates strategic plan
   - Stage 2: Gemini executes bulk operations
   - Use for: Refactoring, migrations, bulk changes

3. **`research`** - Research Pipeline
   - Stage 1: Gemini gathers information
   - Stage 2: Claude synthesizes insights
   - Use for: Technical research, best practices discovery

4. **`code-review`** - Code Review Pipeline
   - Stage 1: Gemini scans for obvious issues
   - Stage 2: Claude does deep architectural review
   - Use for: PR reviews, security audits

**Pipeline Architecture:**
```
Input → [Stage 1: Gemini] → Intermediate Result → [Stage 2: Claude] → Final Output
         (fast, cheap)                            (deep reasoning)
```

**Example Run:**
```
$ ./flip pipeline run research "Go error handling best practices"

Running Pipeline: Research Pipeline
Stage 1/2: gather (gemini) - 58s
Stage 2/2: synthesize (claude) - 22s

Total time: 1m21s
[Synthesized insights output]
```

---

### v5.2 Antigravity Integration (Enhanced)

**New Command:** `./flip antigravity`

Dedicated command for human-in-the-loop integration with Antigravity (GUI Gemini 2.5 Pro).

**Setup:**
```bash
./flip antigravity setup
```

**Send Tasks:**
```bash
# Ask for second opinion
./flip antigravity ask "Does this architecture look solid?"

# Request browser testing
./flip antigravity test "http://localhost:3000" "Test the login flow"

# Request code/content review
./flip antigravity review "func processData() { ... }"
```

**Receive & Process:**
```bash
# Check inbox
./flip antigravity inbox

# Respond to task
./flip antigravity respond AG-123456 "The architecture looks good, but consider..."

# Check status
./flip antigravity status
```

**Workflow:**
```
Automated Agents           Antigravity (Manual)
      │                          │
      │ ./flip antigravity ask   │
      ├─────────────────────────→│ (question)
      │                          │
      │                          │ Process in Gemini 2.5 Pro
      │                          │
      │ ./flip antigravity respond
      │←─────────────────────────┤ (answer)
      │                          │
```

**When to Use Antigravity:**
- Second opinions on architecture/design decisions
- Browser testing (visual verification)
- Complex visual/multimodal tasks
- Final verification before deployment
- Tasks requiring Gemini 2.5 Pro's full capabilities

---

### v5.2 WebSocket Server for Real-Time Communication

**Objective:** Enable real-time bidirectional communication between FLIP and connected agents (like Antigravity) without polling.

**Benefits:**
- Instant message delivery (no 2-second polling delay)
- Automatic reconnection detection
- Persistent connection state
- Lower latency for critical signals
- More robust than file-based polling

**Start WebSocket Server:**
```bash
./flip ws                    # Start on default port 8091
./flip ws -p 8092            # Start on custom port
```

**Endpoints:**
| Endpoint | Description |
|----------|-------------|
| `ws://localhost:8091/ws?agent=antigravity` | WebSocket connection |
| `http://localhost:8091/status` | Server status (REST) |
| `http://localhost:8091/send` | Send message (REST POST) |

**CLI Commands:**
```bash
./flip ws status                              # Check server status
./flip ws send antigravity "Hello!"           # Send message to agent
./flip ws send antigravity "Urgent" -t signal --priority high
```

**Connect from Browser/Antigravity:**
```javascript
// In browser console
const ws = new WebSocket('ws://localhost:8091/ws?agent=antigravity');

ws.onmessage = (event) => {
    const msg = JSON.parse(event.data);
    console.log('Received:', msg);

    if (msg.type === 'signals') {
        // New tasks/signals arrived
        msg.payload.forEach(signal => {
            console.log(`Task: ${signal.subject} - ${signal.message}`);
        });
    }
};

// Send heartbeat
ws.send(JSON.stringify({type: 'heartbeat'}));

// Update status
ws.send(JSON.stringify({
    type: 'status_update',
    payload: {status: 'busy', task: 'TASK-123'}
}));

// Send response
ws.send(JSON.stringify({
    type: 'response',
    to: 'coordinator',
    payload: {message: 'Task completed successfully'}
}));
```

**Message Types:**

| Type | Direction | Description |
|------|-----------|-------------|
| `welcome` | Server→Client | Connection confirmed |
| `signals` | Server→Client | New signals/tasks pushed |
| `agent_status` | Server→All | Agent connected/disconnected |
| `heartbeat` | Client→Server | Keep-alive ping |
| `heartbeat_ack` | Server→Client | Ping response |
| `status_update` | Client→Server | Update agent status |
| `response` | Client→Server | Reply to a task/question |
| `task_complete` | Client→Server | Mark task done |

**Architecture:**
```
┌─────────────────────────────────────────────────────────────┐
│                    FLIP WebSocket Hub                       │
│  ┌─────────────────────────────────────────────────────┐   │
│  │                   WSHub.Run()                        │   │
│  │  - Manages all connections                          │   │
│  │  - Routes messages                                  │   │
│  │  - Checks DB for new signals (2s interval)         │   │
│  │  - Broadcasts status changes                       │   │
│  └─────────────────────────────────────────────────────┘   │
│                          │                                  │
│            ┌─────────────┼─────────────┐                   │
│            │             │             │                    │
│       ┌────▼────┐  ┌────▼────┐  ┌────▼────┐               │
│       │antigravity│ │coordinator│ │claude1   │              │
│       │(WSClient) │ │(WSClient)│  │(WSClient)│              │
│       └─────────┘  └─────────┘  └─────────┘               │
└─────────────────────────────────────────────────────────────┘
              │              │              │
              ▼              ▼              ▼
        [Browser]    [CLI Agent]    [Spawned Agent]
```

**Offline Message Handling:**
- If agent disconnects, messages are stored in DB
- On reconnect, queued signals are pushed immediately
- No messages lost during temporary disconnection

---

