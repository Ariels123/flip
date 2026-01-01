# FLIP2 Architecture & Improvements

## Overview
FLIP2 is a task execution engine built on top of PocketBase. It manages agents and executes tasks via a central daemon.

## Current Architecture
- **Daemon**: Manages the lifecycle, embeds PocketBase, runs the Scheduler and Executor.
- **Executor**: Spawns local processes or (planned) HTTP requests to agents.
- **Scheduler**: Cron-based job scheduling.
- **Database**: SQLite (via PocketBase) for state and coordination.

## Implemented Improvements (v2)

### 1. Real-time Communication (WebSocket)
- **Endpoint**: `/api/flip2/ws`
- **Protocol**: JSON-based messages.
- **Auth**: Uses PocketBase auth tokens (Bear token in header or query param).
- **Mechanism**: 
    - Agents connect and identify themselves (by Agent ID).
    - Server pushes `task_assigned` events to the specific agent connection.
    - Agents push `task_update` and `task_completed` events back.

### 2. Distributed Scalability & Concurrency
- **Atomic Claiming**: 
    - Workers/Daemon attempt to update a task from `pending` to `in_progress` with their `worker_id` (or agent ID) in a single atomic DB operation.
    - Prevents race conditions where multiple daemons pick up the same task.

### 3. Error Handling & Reliability
- **Retry Logic**: 
    - Tasks have `retry_count` and `max_retries`.
    - If execution fails, `retry_count` is incremented.
    - If `retry_count < max_retries`, task status is reset to `pending` (scheduled for future).
    - If `retry_count >= max_retries`, task is marked `failed`.

### 4. Security
- **Input Sanitization**: Basic checking of inputs before shell execution.
- **WebSocket Auth**: Strict token validation on connection.