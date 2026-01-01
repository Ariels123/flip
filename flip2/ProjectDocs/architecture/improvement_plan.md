# FLIP2 Architecture Improvement Plan

## 1. Distributed Task Execution (Major Upgrade)
**Current State**: The central Daemon executes all tasks via `internal/executor`. This requires all agent tools/CLIs to be installed on the Daemon's machine.
**Problem**: Limits scalability and distributed deployment. Remote agents cannot execute tasks in their own environment.
**Solution**: Enhance `flip2 agent listen` to become a full **Task Worker**.
- **Change**: `agent listen` subscribes to `tasks` (create/update) events.
- **Logic**:
  1. Filter for valid tasks (status='todo', assignee='MY_ID').
  2. **Atomic Claim**: Attempt to set status='in_progress' and worker_id=MY_ID.
  3. **Execute**: Run the task command locally (e.g. `claude prompt "..."`).
  4. **Report**: Update task with result.
- **Daemon Update**: Configure Daemon to *skip* execution for `type=remote` agents.

## 2. Agent Heartbeats (Robustness)
**Current State**: Agents are "online" if registered, but we don't know if the process is actually running.
**Solution**: 
- Add `last_seen` timestamp to `agents` collection.
- `agent listen` sends a heartbeat (HTTP PATCH) every 30s.
- Daemon/Admin UI can show "Offline" if > 1min silence.

## 3. SSE Connection Resiliency (Efficiency)
**Current State**: Retries exist, but we can improve via **Event ID tracking**.
**Solution**:
- Track `Last-Event-ID` (if supported by PocketBase, or use `created` timestamp filters) to fetch missed events during brief disconnects.
- (Already mitigated by Hybrid Polling, but tracking IDs is cleaner).

## 4. Concurrent Task Limiting (Efficiency)
**Current State**: Daemon limits total concurrency.
**Solution**: Agents should limit *their own* concurrency.
- `agent listen` maintains a local semaphore (e.g., max 1 task at a time for serial agents).

## Implementation Steps
1. **Schema Update**: Add `mode` (local/remote) to `agents`. Add `last_seen` to `agents`.
2. **Client Update**: Add `Subscribe("tasks")` and task handling logic to `pkg/client`.
3. **CLI Update**: Upgrade `flip2 agent listen` to handle queued tasks.
4. **Daemon Update**: Update `internal/executor` to respect agent mode.
