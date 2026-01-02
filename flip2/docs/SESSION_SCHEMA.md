# Session Schema Documentation

## Overview

The Session Schema defines the complete persistence model for agent sessions in FLIP. It enables multi-agent coordination, message tracking, task management, and state recovery across coordinator disconnections and system failures.

**Location**: `/Users/arielspivakovsky/src/flip/flip2/internal/session/schema.go`

## Core Concepts

### Session
A **session** is a bounded execution context where agents communicate, execute tasks, and maintain shared state. Each session has:
- A unique ID and name
- A coordinator agent managing the session
- Optional parent session (for sub-sessions)
- Status tracking (created, active, completed, failed, etc.)
- Shared environment variables and configuration

### Message Flow
All inter-agent communication is logged as **messages**:
- Coordinator-to-Agent directives
- Agent-to-Coordinator status updates
- Agent-to-Agent coordination
- System events and errors

### Tasks & Agents
- **Tasks** are units of work spawned by the coordinator or agents
- **Agents** are LLM-based workers that join sessions and execute tasks
- Each agent's activity is tracked: messages sent, tasks completed, status changes

## Persistence Strategy

### What Gets Saved

```
Sessions Database Schema:
├── sessions (metadata)
├── session_messages (message log)
├── session_agents (agent participation)
├── session_tasks (task lifecycle)
└── session_variables (configuration)
```

#### 1. **sessions** Table
Stores session metadata and summary metrics.

**What's Persisted:**
- Session ID, name, description
- Coordinator ID
- Status and timestamps
- Aggregate counters (message_count, agent_count, task_count, error_count)
- Custom metadata (extensible JSON)

**Why:** Enables fast lookup, status queries, and recovery point identification.

**Example Query:**
```sql
SELECT * FROM sessions WHERE status = 'active' AND coordinator_id = 'coordinator-1'
```

#### 2. **session_messages** Table
Complete audit log of all messages exchanged.

**What's Persisted:**
- Message ID and session reference
- Role (coordinator, agent, system, user)
- Sender and recipient IDs
- Content and content type (text, json, markdown)
- Message type (request, response, status, error, task, signal, heartbeat)
- Status (pending, processing, processed, delivered, failed)
- Token metrics (for LLM-based messages)
- Timestamps (created, processed)
- Errors that occurred during processing

**Why:** Provides:
- Complete audit trail for debugging
- Conversation history for context
- Token usage tracking for cost analysis
- Ability to replay sessions
- Evidence of task assignments and results

**Example Queries:**
```sql
-- Find all errors in a session
SELECT * FROM session_messages
WHERE session_id = 'sess-123' AND message_type = 'error'
ORDER BY created_at DESC;

-- Find task assignments for an agent
SELECT * FROM session_messages
WHERE session_id = 'sess-123'
  AND message_type = 'task'
  AND sender_id = 'coordinator-1'
  AND recipient_id = 'worker-1';
```

#### 3. **session_agents** Table
Tracks agent participation in sessions.

**What's Persisted:**
- Agent reference ID and session reference
- Agent ID, name, model
- Role in the session (worker, analyzer, etc.)
- Status (joining, active, busy, waiting, paused, error, disconnected, left)
- Timestamps (joined, last_activity, left)
- Message and task counters
- Agent-specific properties (configuration)
- Metadata

**Why:** Enables:
- Finding active agents in a session
- Tracking agent availability
- Detecting stale/disconnected agents
- Analyzing agent performance

**Example Queries:**
```sql
-- Find all active agents in a session
SELECT * FROM session_agents
WHERE session_id = 'sess-123' AND status = 'active';

-- Find agents that haven't been active for 30 minutes
SELECT * FROM session_agents
WHERE session_id = 'sess-123'
  AND last_activity_at < datetime('now', '-30 minutes')
  AND status != 'left';
```

#### 4. **session_tasks** Table
Tracks task lifecycle from creation to completion.

**What's Persisted:**
- Task ID and session reference
- Assigned agent ID
- Title and description
- Status (created, waiting, assigned, running, completed, failed, skipped, cancelled)
- Input (JSON) and result (JSON)
- Error message if failed
- Priority and retry information
- Timestamps (created, started, completed, due)
- Performance metrics (tokens, duration, memory, cost)
- Dependencies (task IDs that must complete first)
- Tags for categorization
- Metadata

**Why:** Enables:
- Task scheduling and prioritization
- Dependency management
- Retry logic and error recovery
- Performance analysis
- Task traceability

**Example Queries:**
```sql
-- Find all failed tasks in a session
SELECT * FROM session_tasks
WHERE session_id = 'sess-123' AND status = 'failed'
ORDER BY completed_at DESC;

-- Find tasks assigned to an agent that are still running
SELECT * FROM session_tasks
WHERE session_id = 'sess-123'
  AND assigned_agent_id = 'worker-1'
  AND status IN ('assigned', 'running');

-- Calculate total cost for a session
SELECT SUM(CAST(json_extract(metrics, '$.cost') AS FLOAT))
FROM session_tasks
WHERE session_id = 'sess-123';
```

#### 5. **session_variables** Table
Stores session-scoped configuration and variables.

**What's Persisted:**
- Variable key and value
- Creation and update timestamps

**Why:** Enables:
- Runtime configuration changes
- Shared state between agents
- Feature flags and toggles

## What Gets Restored

### On Coordinator Reconnection

When the coordinator reconnects to an existing session:

1. **Session metadata** is loaded
   - Status is verified
   - Last heartbeat time is checked
   - Agent and task counts are validated

2. **Active agents** are identified
   - Agents still marked as "active" or "busy" are contacted
   - Stale agents (no activity for N minutes) are marked as "disconnected"
   - Join messages are re-sent for reconnection

3. **Pending messages** are replayed
   - Any unprocessed messages are restored from the queue
   - This ensures no work is lost

4. **Running tasks** are resumed
   - Tasks with status "assigned" or "running" are checked
   - If the assigned agent is still active, execution resumes
   - If the agent is disconnected, the task is reassigned or failed

5. **Variables and environment** are loaded
   - Session configuration is restored
   - Custom metadata is accessible

### Recovery Scenarios

#### Scenario 1: Coordinator Crashes
```
1. Session marked as "active" in database
2. Coordinator restarts
3. Queries database for "active" sessions it was managing
4. Checks heartbeat timestamp
5. If recent (< threshold), resumes the session
6. Contacts all previously active agents
7. Replays unprocessed messages
```

#### Scenario 2: Agent Disconnects
```
1. Agent stops sending heartbeats
2. Coordinator detects timeout (no message for N seconds)
3. Marks agent status as "disconnected"
4. Tasks assigned to agent are reassigned or failed
5. Agent can rejoin later by reconnecting to coordinator
```

#### Scenario 3: Message Loss
```
1. All messages logged before sending
2. Messages marked "pending" until acknowledged
3. On reconnection, pending messages are replayed
4. Agent can confirm receipt to mark as "delivered"
```

## SQLite Schema

```sql
-- Core session metadata
CREATE TABLE sessions (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    status TEXT NOT NULL,
    coordinator_id TEXT NOT NULL,
    parent_session_id TEXT,
    description TEXT,
    created_at DATETIME,
    started_at DATETIME,
    completed_at DATETIME,
    updated_at DATETIME,
    last_heartbeat_at DATETIME,
    message_count INTEGER,
    agent_count INTEGER,
    task_count INTEGER,
    error_count INTEGER,
    metadata TEXT
);

-- Complete message audit trail
CREATE TABLE session_messages (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    role TEXT NOT NULL,
    sender_id TEXT NOT NULL,
    recipient_id TEXT,
    content TEXT NOT NULL,
    content_type TEXT,
    message_type TEXT NOT NULL,
    status TEXT,
    tokens_used TEXT,
    metadata TEXT,
    created_at DATETIME,
    processed_at DATETIME,
    error TEXT,
    FOREIGN KEY (session_id) REFERENCES sessions(id)
);

-- Agent participation tracking
CREATE TABLE session_agents (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    agent_id TEXT NOT NULL,
    name TEXT NOT NULL,
    model TEXT NOT NULL,
    role TEXT NOT NULL,
    status TEXT,
    joined_at DATETIME,
    last_activity_at DATETIME,
    left_at DATETIME,
    message_count INTEGER,
    task_count INTEGER,
    properties TEXT,
    metadata TEXT,
    FOREIGN KEY (session_id) REFERENCES sessions(id)
);

-- Task lifecycle tracking
CREATE TABLE session_tasks (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    assigned_agent_id TEXT NOT NULL,
    title TEXT NOT NULL,
    description TEXT,
    status TEXT,
    input TEXT,
    result TEXT,
    error TEXT,
    priority INTEGER,
    retry_count INTEGER,
    max_retries INTEGER,
    created_at DATETIME,
    started_at DATETIME,
    completed_at DATETIME,
    due_at DATETIME,
    metrics TEXT,
    dependencies TEXT,
    tags TEXT,
    metadata TEXT,
    FOREIGN KEY (session_id) REFERENCES sessions(id)
);

-- Session configuration and state
CREATE TABLE session_variables (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    key TEXT NOT NULL,
    value TEXT,
    created_at DATETIME,
    updated_at DATETIME,
    FOREIGN KEY (session_id) REFERENCES sessions(id)
);
```

## State Transitions

### Session Status Flow
```
created → active → [paused] → completed
              ↓
           failed
              ↓
          cancelled
              ↓
            stale
```

### Agent Status Flow
```
joining → active ← disconnected
            ↓
           busy
            ↓
         waiting → active

paused → active → left
```

### Task Status Flow
```
created → waiting → assigned → running → completed
              ↓         ↓         ↓          ↓
           skipped   failed    failed     skipped
                                 ↓
                            cancelled
```

## Performance Considerations

### Indexing Strategy
```
Sessions:
  - PRIMARY KEY (id)
  - INDEX (status) - for finding active sessions
  - INDEX (coordinator_id) - for coordinator queries
  - INDEX (created_at DESC) - for time-based queries

Messages:
  - PRIMARY KEY (id)
  - INDEX (session_id, created_at DESC) - for retrieval order
  - INDEX (message_type) - for filtering by type
  - INDEX (status) - for finding pending messages

Agents:
  - PRIMARY KEY (id)
  - INDEX (session_id) - for session lookup
  - INDEX (status) - for finding active agents
  - UNIQUE (session_id, agent_id) - prevent duplicates

Tasks:
  - PRIMARY KEY (id)
  - INDEX (session_id) - for session lookup
  - INDEX (status) - for filtering by state
  - INDEX (priority DESC) - for task prioritization
```

### Data Cleanup
- Message logs can grow large over time
- Implement archival strategy: archive completed sessions after N days
- Task results can be compressed or moved to separate store
- Variables can be expired with TTL

## Example Workflows

### Workflow 1: Spawning a Worker Agent
```
1. Coordinator creates SessionState
2. Creates AgentRef for new worker
3. Inserts message: "You are a WORKER agent, complete this task..."
4. Message status = "pending"
5. Worker starts and reads pending messages
6. Worker marks message as "delivered"
7. Worker begins task execution
```

### Workflow 2: Task Completion & Recovery
```
1. Worker completes task
2. Sends message with result (message_type = "response")
3. Message persisted with tokens_used metrics
4. Task marked as "completed" with result
5. Coordinator reads result from messages table
6. If coordinator crashes here:
   - Session remains in "active" state
   - Message is already persisted
   - On recovery, coordinator sees completed task
   - No duplicate execution
```

### Workflow 3: Agent Reconnection
```
1. Agent joins session: insert session_agents row
2. Agent receives tasks and completes them
3. Agent disconnects (network failure)
4. last_activity_at timestamp shows staleness
5. Coordinator detects timeout
6. Agent reconnects
7. Query session_messages for unprocessed messages
8. Replay pending tasks
9. Agent resumes work
```

## Implementation Notes

### Type Safety
- All structs use proper Go types with json/db tags
- Status fields use type-safe enums (SessionStatus, AgentStatus, etc.)
- Validation methods catch invalid states

### Extensibility
- Metadata fields support arbitrary JSON
- Custom properties per agent
- Task tags for categorization
- Event system for extensions

### Concurrency
- SQLite file locking prevents corruption
- Each session has its own isolated state
- Message ordering preserved by created_at timestamp
- Atomic updates via transactions

## Future Enhancements

1. **Message compression**: Store only deltas for large message histories
2. **Task result caching**: Cache frequently requested task results
3. **Agent profiles**: Learn agent performance patterns
4. **Automatic cleanup**: Archive old sessions, compress logs
5. **Analytics**: Built-in queries for session metrics, cost analysis
6. **Replication**: Multi-node session state for high availability
