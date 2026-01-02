# Bug Analysis: Executor Status Mismatch

## Symptoms
1. `flip2 agent spawn` created Agents but failed to create Tasks.
2. Manual Task Injection API calls failed with `status: invalid value` when trying to set `pending` or `created`.
3. Schema (`pb_migrations/1_initial_schema.go`) enforces `status` ENUM: `["todo", "in_progress", "done", "failed"]`.
4. Executor Code (`internal/executor/executor.go`) queries for `status = 'pending'`.
5. Executor Claim Logic (`internal/executor/executor.go`) checks `if status != "pending" ... return error`.

## Root Cause
**Schema/Code Drift**: The database schema was updated to use standard Kanban statuses (`todo`), but the Go executor code was not updated to match, retaining the legacy `pending` status.

## Proposed Fix (Robust)
**Align Code to Schema**: The Schema supports `todo`, which is a standard convention. The Code involves multiple references to `pending` in `executor.go`.
1. Modify `internal/executor/executor.go`:
   - Change `queuePendingTasks` query filter from `'pending'` to `'todo'`.
   - Change `processTask` claim check from `'pending'` to `'todo'`.
   - Change `failTask` retry logic to reset status to `'todo'`.

## Backup Communication Channel
- Primary: `AG_STATUS_UPDATES.md`
- Backup: FLIP2 Signals (`POST /api/signals`)
  - Target: `Claud-win` (Coordinator)
  - Type: `BUG_REPORT`
