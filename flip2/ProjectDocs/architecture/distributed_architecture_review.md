# Distributed Architecture Review & Gap Analysis

**Date**: 2025-12-13
**Reviewer**: Gemini
**Context**: Review of `flip2 agent listen` implementation and Distributed Task Execution.

## 1. Executive Summary
The distributed task execution framework is **functional** but lacks robustness mechanisms for production scaling. Agents can successfully receive, execute, and report tasks via Realtime API (SSE). However, significant gaps exist in **concurrency control**, **failure recovery**, and **agent monitoring**.

## 2. Capability Analysis

### ✅ Implemented & Working
1.  **Realtime Event Streaming**: `pkg/client` successfully implements SSE client with expo-backoff.
2.  **Task Distribution**: Tasks created via API/CLI are pushed to specific agents via the `assignee` filter.
3.  **Local Execution**: `flip2 agent listen` correctly spawns local processes (e.g., `claude`, `bash`) to execute work.
4.  **Status Reporting**: Tasks transition `todo` -> `in_progress` -> `done/failed` correctly.

### ⚠️ Implementation Gaps (Critical)

#### A. No Concurrency Limiting
*   **Observation**: The `agent listen` loop spawns a `go func()` for *every* incoming task event immediately.
*   **Risk**: If 100 tasks are assigned simultaneously to `claude`, the agent will spawn 100 `claude` processes, likely crashing the host via OOM (Out Of Memory) or process limits.
*   **Recommendation**: Implement a **Semaphore / Worker Pool** pattern in `main.go`.
    ```go
    // Example Plan
    sem := make(chan struct{}, maxConcurrent) // e.g., 1 for serial LLM agents
    for event := range tasks {
        sem <- struct{}{} // Acquire
        go func() {
            defer func() { <-sem }() // Release
            execute(...)
        }()
    }
    ```

#### B. Missing Heartbeats
*   **Observation**: The Architecture Plan specified `last_seen` updates, but `main.go` does not implement a heartbeat loop.
*   **Risk**: The Daemon/System has no way to know if an agent is actually online or just quiet. Dead agents look the same as idle agents.
*   **Recommendation**: Add a ticker (e.g., 30s) in `main.go` to PATCH `agents/{id}` with `last_seen=NOW`.

#### C. Non-Atomic Task Claiming
*   **Observation**: Agents check `rec.Status == "todo"` in the *event payload* and then blindly send `PATCH status='in_progress'`.
*   **Risk**: Efficiency issue. Use Optimistic Concurrency Control if multiple workers share an identity (queue model).
*   **Recommendation**: Update `updateTaskStatus` to verify `status='todo'` in the update filter or check result.

#### D. No "Zombie" Recovery
*   **Observation**: If an agent crashes *during* execution (`in_progress`), the task remains `in_progress` indefinitely.
*   **Recommendation**: Implement a **Reaper** process in the Daemon (or a scheduled job) that resets `in_progress` tasks if the assignee's `last_seen` > 5 minutes ago.

## 3. Security Considerations
*   **Current**: `tasks` collection has Public API rules (`ListRule=""`).
*   **Risk**: Any user with network access can view all tasks.
*   **Future**: Implement `RequireAuth` for agents. Agents should authenticate as "users" or a new "agents" auth collection.

## 4. Next Steps for Claude
1.  **Refactor `agent listen`**: Add Semaphore/Worker Pool.
2.  **Implement Heartbeat**: Add 30s `last_seen` loop.
3.  **Harden Client**: Add proper JSON logging instead of `fmt.Printf`.
4.  **Daemon Logic**: Add `ZombieTaskReaper` job.
