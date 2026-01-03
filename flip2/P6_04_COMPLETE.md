# P6-04: Zombie Reaper Enhancement (v2) Complete

**Status**: ✅ Complete
**Date**: 2026-01-03 17:35 EST
**Assignee**: Gemini

## 1. Summary
Enhanced the Zombie Task Reaper to auto-reassign tasks from dead agents instead of immediately failing them. This improves system resilience against transient agent failures.

## 2. Changes Implemented

### Configuration (`internal/config/config.go`)
Added `ZombieReaperConfig` with customizable defaults:
- `Interval`: `0 */1 * * * *` (1 minute check interval)
- `StaleThreshold`: 5 minutes (time since `last_seen` to consider agent dead)
- `MaxReassignments`: 3 (max times a task can be reassigned before hard failure)

### Daemon Logic (`internal/daemon/daemon.go`)
Updated `zombie-reaper` job:
- **Detection**: Identifies agents with `last_seen > StaleThreshold`.
- **Metadata Tracking**: Uses task `metadata.retry_count` to track attempts.
- **Auto-Reassignment**:
  - If `retry_count < MaxReassignments`:
    - Set `status` = `todo`
    - Set `assignee` = `""`
    - Increment `retry_count`
    - Log warning
  - Else:
    - Set `status` = `failed`
    - Log error

## 3. Verification
- **Build**: `go build ./cmd/flip2` passed.
- **Logic Check**: Code implements specific logic for metadata handling and state transitions.

## 4. Next Steps
- Monitor logs for "Reaper: Reassigning zombie task" to verify behavior in production.
- Consider exposing reaper stats in metrics endpoint.
