# Task Delegation: FLIP2 Metrics Monitoring

**Assigned to:** Antigravity (Human-in-loop monitor)
**Priority:** Medium
**Purpose:** Preserve coordinator Claude's context by offloading routine monitoring

## Your Role

You are the **TASK MONITOR** for the FLIP2 implementation project. The coordinator (main Claude instance) keeps running out of context, so your job is to handle routine monitoring and metrics updates.

## Tasks

### 1. Check Agent Status (every 10 minutes)
```bash
cd /Users/arielspivakovsky/src/flip
./flip status
./flip task list
```

### 2. Collect Completed Outputs

The system shows 24+ completed agents but the metrics file only shows 3. Retrieve outputs from these completed agents:

```bash
# Check each completed agent
./flip task output <agent-id>
```

Known completed agents (from system):
- a224af2, a1c03cd, a66e377, ad6cc6a, a6cecf4, aa4f366
- af76244, afa4613, a252c1e, a1b66a5, a7af71d, a6943a8
- ae85307, a1f879e, a4e5008, ad2e031, aa4dec2, a854ba4
- a920ebf, aca6968, a598c7e, aee560f, aa73fae, a98356c

### 3. Update Metrics File

File: `/Users/arielspivakovsky/src/flip/flip2/IMPLEMENTATION_METRICS_2026.md`

Update these sections:
- **Summary Dashboard** - increment completed task count
- **Completed Tasks** - add new completions with effort/cost/variance
- **Active Tasks** - move completed ones to Completed section
- **Last Updated** timestamp

### 4. Report to Coordinator

Signal updates via:
```bash
./flip signal send coordinator "METRICS UPDATE: X new tasks completed, $Y spent, Z% under estimate"
```

## Current State

- **Metrics file last updated:** 2026-01-01 16:20
- **File shows:** 3 completed tasks
- **Actually completed:** 24+ agents
- **Gap:** 21+ tasks need recording
- **Cost performance:** 87-93% under estimates

## Success Criteria

- Metrics file updated within 1 hour
- All 24+ completions recorded
- Coordinator receives summary signal
- Monitoring continues every 10 mins

## Notes

- This preserves coordinator's context by offloading routine work
- Focus on data collection and file updates, not decision-making
- Escalate issues to coordinator via signal
- Keep monitoring until coordinator signals "STOP MONITORING"

---

**Created:** 2026-01-01 17:07
**Status:** READY FOR PICKUP
