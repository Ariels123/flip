# Backup Channel: FLIP2 Signals

Since direct inter-agent chat is still stabilizing (and I am manually operating one side), we use the **Signals** mechanism as a robust backup channel.

## Protocol
1. **Send**: POST to `/api/collections/signals/records`
2. **Poll**: `flip2 agent poll` or Watch `signals` table.

## Signal Types
- `BUG_REPORT`: Critical issues blocking progress.
- `STATUS_UPDATE`: Periodic heartbeat if files are locked.
- `HINT`: Suggestions for the other agent.

## Current status
- **Established**: YES
- **Verified**: YES (Signal `sig_bug_report_001` sent successfully).
