# Gemini - Continue Working on These Tasks

Claude is pausing. Continue independently on these tasks.

## Priority Tasks

### 1. Complete Architecture Review (REVIEW-001)
Review and improve:
- `/Users/arielspivakovsky/src/flip/flip2/internal/daemon/daemon.go`
- `/Users/arielspivakovsky/src/flip/flip2/internal/executor/executor.go`

Focus on:
- **WebSocket Support**: Add real-time communication instead of polling
- **Error Handling**: Add retry logic with exponential backoff
- **Security**: API key authentication
- **Scalability**: Support for distributed workers

### 2. Add WebSocket Handler
Create `internal/websocket/handler.go`:
- Subscribe to PocketBase realtime events
- Push task updates to connected agents
- Handle agent heartbeats

### 3. Fix Cron Format
In `config/config.yaml`, cron needs 6 fields:
```yaml
cron: "0 */1 * * * *"  # Not "*/1 * * * *"
```

### 4. Improve Daemon Stability
The daemon keeps crashing. Investigate and fix:
- Check `/tmp/flip2d.log` for errors
- Add recovery/restart logic
- Handle database locks properly

## How to Work

1. Poll for tasks:
```bash
curl -s "http://localhost:8091/api/collections/tasks/records?filter=(status='in_progress')"
```

2. Update task when done:
```bash
curl -X PATCH "http://localhost:8091/api/collections/tasks/records/TASK_ID" \
  -H "Content-Type: application/json" \
  -d '{"status": "done", "result": "YOUR_RESULT"}'
```

3. Update your log at `logs/GEMINI_LOG.md` after each task

4. If blocked, create a signal:
```bash
curl -X POST http://localhost:8091/api/collections/signals/records \
  -H "Content-Type: application/json" \
  -d '{"signal_id": "BLOCK-001", "from_agent": "gemini", "to_agent": "claude", "signal_type": "alert", "content": "Blocked on X, need help"}'
```

## DO NOT
- Use old ./flip shell commands
- Spawn additional agents without approval
- Make breaking changes without documenting

Work independently. Claude will review when back online.
