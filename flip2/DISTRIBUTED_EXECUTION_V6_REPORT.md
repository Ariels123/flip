# Distributed Execution V6 - Analysis & Implementation Report

**Worker Agent**: Opus Worker
**Date**: 2026-01-02
**Task**: Distributed Execution V6 Analysis

---

## Executive Summary

This report analyzes the current state of distributed execution in FLIP/FLIP2 and provides a roadmap for **Distributed Execution V6** - the next evolution of multi-node task coordination.

### Key Findings

| Component | Current State | V6 Target |
|-----------|--------------|-----------|
| **Task Distribution** | SSE-based (FLIP2) | gRPC bi-directional streaming |
| **Transport Security** | API Key + JWT | mTLS with short-lived certificates |
| **Concurrency Control** | Semaphore (implemented) | Semaphore + capability-based routing |
| **Heartbeats** | 30s HTTP PATCH | 10s gRPC streaming |
| **Multi-Node** | Single coordinator | Coordinator + N workers (gRPC) |
| **Failure Recovery** | Manual | Automatic zombie recovery + task reassignment |

---

## 1. Current State Analysis

### 1.1 FLIP2 Agent System (Working)

**Location**: `flip2/cmd/flip2/main.go:539-737`

**Implemented Features**:
- Real-time task streaming via SSE (`/api/flip2/realtime`)
- Semaphore-based concurrency control (configurable)
- 30-second heartbeat loop updating `last_seen`
- 3-second polling fallback
- Task state machine: `todo` → `in_progress` → `done/failed`
- Role-based agent spawning with built-in templates

**Code Example** (existing semaphore):
```go
concurrency, _ := cmd.Flags().GetInt("concurrency")
sem := make(chan struct{}, concurrency)

for event := range c.Tasks() {
    sem <- struct{}{}  // Acquire
    go func() {
        defer func() { <-sem }()  // Release
        executeAgentTask(...)
    }()
}
```

### 1.2 Parent FLIP Distributed Infrastructure (Partial)

**Location**: `/Users/arielspivakovsky/src/flip/pkg/distributed/`

**Components**:
| Package | Status | Purpose |
|---------|--------|---------|
| `coordinator/` | Partial | gRPC server, worker registry, task dispatcher |
| `node/` | Partial | Worker with mTLS connection, heartbeat |
| `pki/` | Working | Certificate generation, CA management |

**What Works**:
- PKI initialization (`flip distributed init`)
- Certificate generation for workers
- gRPC + mTLS connection setup
- Basic worker registration/heartbeat framework

**What's Missing**:
- Protocol Buffer definitions (no .proto files)
- Actual gRPC service implementation
- Task streaming over gRPC
- Result collection

### 1.3 Architecture Gap Analysis

| Gap | Severity | Current Workaround | V6 Solution |
|-----|----------|-------------------|-------------|
| No gRPC proto definitions | Critical | N/A - uses HTTP/SSE | Define FlipService.proto |
| Non-atomic task claiming | Medium | Trust single assignee | Optimistic concurrency control |
| No zombie recovery | High | Manual intervention | Reaper process with heartbeat tracking |
| Single coordinator SPOF | Medium | N/A | Future: Multi-coordinator HA |
| No capability matching | Medium | Manual task routing | Capability-based task dispatcher |

---

## 2. Distributed Execution V6 Design

### 2.1 Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        DISTRIBUTED EXECUTION V6                              │
└─────────────────────────────────────────────────────────────────────────────┘

                              COORDINATOR (flip2)
                                     │
                    ┌────────────────┼────────────────┐
                    │                │                │
                    ▼                ▼                ▼
             ┌──────────┐     ┌──────────┐     ┌──────────┐
             │ Worker 1 │     │ Worker 2 │     │ Worker N │
             │ (Local)  │     │ (Remote) │     │ (Remote) │
             └──────────┘     └──────────┘     └──────────┘
                  │                │                │
                  └────────────────┴────────────────┘
                              gRPC/mTLS
                         (bi-directional streaming)
```

### 2.2 Protocol Buffer Service Definition

**File**: `proto/flip_distributed.proto`

```protobuf
syntax = "proto3";
package flip.distributed.v6;
option go_package = "flip/pkg/distributed/proto";

// FlipDistributedService - Core distributed execution service
service FlipDistributedService {
    // Worker registration and heartbeat
    rpc Register(RegisterRequest) returns (RegisterResponse);
    rpc Heartbeat(stream HeartbeatMessage) returns (stream HeartbeatAck);

    // Task streaming (bi-directional)
    rpc TaskStream(stream TaskUpdate) returns (stream TaskAssignment);

    // Result collection
    rpc SubmitResult(TaskResult) returns (ResultAck);

    // Coordinator queries
    rpc GetWorkerStatus(WorkerStatusRequest) returns (WorkerStatusResponse);
    rpc ListTasks(ListTasksRequest) returns (ListTasksResponse);
}

message RegisterRequest {
    string worker_id = 1;
    string host = 2;
    repeated string capabilities = 3;
    int32 max_concurrency = 4;
}

message RegisterResponse {
    bool success = 1;
    string session_id = 2;
    int64 heartbeat_interval_ms = 3;
}

message HeartbeatMessage {
    string worker_id = 1;
    string status = 2;  // online, busy, draining, offline
    int32 active_tasks = 3;
    map<string, string> metrics = 4;
}

message HeartbeatAck {
    bool acknowledged = 1;
    int64 server_time = 2;
}

message TaskAssignment {
    string task_id = 1;
    string title = 2;
    string description = 3;
    string agent_type = 4;  // claude, gemini, bash, etc.
    int32 priority = 5;
    int64 timeout_ms = 6;
    map<string, string> context = 7;
    repeated string required_capabilities = 8;
}

message TaskUpdate {
    string task_id = 1;
    string status = 2;  // claimed, in_progress, done, failed
    string progress_message = 3;
    float progress_percent = 4;
}

message TaskResult {
    string task_id = 1;
    string status = 2;  // done, failed
    string result = 3;
    string error_message = 4;
    int64 execution_time_ms = 5;
    int32 tokens_used = 6;
}

message ResultAck {
    bool success = 1;
    string message = 2;
}

message WorkerStatusRequest {
    string worker_id = 1;  // empty = all workers
}

message WorkerStatusResponse {
    repeated WorkerInfo workers = 1;
}

message WorkerInfo {
    string worker_id = 1;
    string host = 2;
    string status = 3;
    int32 active_tasks = 4;
    int64 last_seen = 5;
    repeated string capabilities = 6;
}

message ListTasksRequest {
    string status_filter = 1;  // empty = all
    string assignee_filter = 2;
    int32 limit = 3;
}

message ListTasksResponse {
    repeated TaskInfo tasks = 1;
}

message TaskInfo {
    string task_id = 1;
    string title = 2;
    string status = 3;
    string assignee = 4;
    int64 created_at = 5;
    int64 updated_at = 6;
}
```

### 2.3 V6 Feature Set

#### A. Capability-Based Task Routing

```go
// Worker capabilities
type WorkerCapabilities struct {
    Languages    []string  // go, python, js, rust
    Models       []string  // claude, gemini, haiku, opus
    MaxTokens    int       // Max context window
    Concurrency  int       // Max parallel tasks
    HasGPU       bool      // For local model inference
    HasBrowser   bool      // For browser automation
}

// Task routing rules
func (c *Coordinator) RouteTask(task *Task) *WorkerInfo {
    // 1. Filter workers by required capabilities
    candidates := c.FilterByCapabilities(task.RequiredCapabilities)

    // 2. Score by load (prefer least loaded)
    scored := c.ScoreByLoad(candidates)

    // 3. Apply priority boost (high-priority to fastest workers)
    if task.Priority >= PriorityHigh {
        scored = c.BoostFastWorkers(scored)
    }

    // 4. Select best match
    return scored[0]
}
```

#### B. Zombie Task Recovery

```go
// ReaperConfig defines zombie detection settings
type ReaperConfig struct {
    CheckInterval    time.Duration  // How often to check (default: 60s)
    HeartbeatTimeout time.Duration  // Max time since last_seen (default: 5min)
    MaxRetries       int            // Max reassignment attempts (default: 3)
}

// ZombieReaper recovers stuck tasks
func (c *Coordinator) ZombieReaper(ctx context.Context, cfg ReaperConfig) {
    ticker := time.NewTicker(cfg.CheckInterval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            c.recoverZombieTasks(cfg)
        }
    }
}

func (c *Coordinator) recoverZombieTasks(cfg ReaperConfig) {
    now := time.Now()

    // Find tasks stuck in 'in_progress' with dead assignees
    stuckTasks := c.db.Query(`
        SELECT t.* FROM tasks t
        JOIN agents a ON t.assignee = a.id
        WHERE t.status = 'in_progress'
        AND a.last_seen < ?
        AND t.retry_count < ?
    `, now.Add(-cfg.HeartbeatTimeout), cfg.MaxRetries)

    for _, task := range stuckTasks {
        logger.Warn("Recovering zombie task",
            "task_id", task.ID,
            "assignee", task.Assignee,
            "stuck_since", task.UpdatedAt)

        // Reset to 'todo' for reassignment
        c.db.Update("tasks", task.ID, map[string]any{
            "status":      "todo",
            "assignee":    nil,
            "retry_count": task.RetryCount + 1,
            "updated_at":  now,
        })
    }
}
```

#### C. Optimistic Concurrency Control

```go
// ClaimTask atomically claims a task for a worker
func (w *Worker) ClaimTask(taskID string) error {
    // Use compare-and-swap pattern
    result := w.coordinator.UpdateTask(taskID, map[string]any{
        "status":   "in_progress",
        "assignee": w.ID,
    }, map[string]any{
        // Only update if current status is 'todo'
        "status": "todo",
    })

    if result.ModifiedCount == 0 {
        return ErrTaskAlreadyClaimed
    }
    return nil
}
```

#### D. Connection Pool & Circuit Breaker

```go
// WorkerPool manages connections to workers
type WorkerPool struct {
    workers  map[string]*grpc.ClientConn
    breakers map[string]*CircuitBreaker
    mu       sync.RWMutex
}

// CircuitBreaker prevents cascading failures
type CircuitBreaker struct {
    State        CircuitState  // Closed, Open, HalfOpen
    FailureCount int
    LastFailure  time.Time
    Threshold    int           // Open after N failures
    Timeout      time.Duration // Reset after timeout
}

func (p *WorkerPool) GetConnection(workerID string) (*grpc.ClientConn, error) {
    p.mu.RLock()
    breaker := p.breakers[workerID]
    p.mu.RUnlock()

    if breaker.IsOpen() {
        return nil, ErrCircuitOpen
    }

    conn := p.workers[workerID]
    if conn == nil {
        return nil, ErrWorkerNotConnected
    }
    return conn, nil
}
```

---

## 3. Implementation Plan

### 3.1 Phase Breakdown

| Phase | Tasks | Duration | Dependencies |
|-------|-------|----------|--------------|
| **V6-P1** | Proto definitions, code generation | 2 days | None |
| **V6-P2** | Coordinator gRPC server | 3 days | V6-P1 |
| **V6-P3** | Worker gRPC client | 2 days | V6-P2 |
| **V6-P4** | Zombie recovery + circuit breaker | 2 days | V6-P3 |
| **V6-P5** | CLI integration (`flip2 distributed`) | 2 days | V6-P3 |
| **V6-P6** | Integration tests | 2 days | V6-P5 |

**Total Estimated Duration**: 13 days

### 3.2 Task List

#### V6-P1: Protocol Definitions
```
V6-001: Create proto/flip_distributed.proto (4h) - Sonnet
V6-002: Generate Go code with protoc (2h) - Haiku
V6-003: Add proto deps to go.mod (1h) - Haiku
```

#### V6-P2: Coordinator Server
```
V6-004: Implement FlipDistributedService interface (6h) - Opus
V6-005: Add TaskStream bi-directional streaming (4h) - Sonnet
V6-006: Implement RegisterWorker + heartbeat handling (4h) - Sonnet
V6-007: Implement capability-based routing (4h) - Sonnet
V6-008: Add PocketBase integration for task persistence (4h) - Sonnet
```

#### V6-P3: Worker Client
```
V6-009: Create gRPC worker client (4h) - Sonnet
V6-010: Implement task stream consumer (4h) - Sonnet
V6-011: Add heartbeat producer goroutine (2h) - Sonnet
V6-012: Integrate with existing LLM executor (4h) - Sonnet
```

#### V6-P4: Resilience
```
V6-013: Implement ZombieReaper (3h) - Sonnet
V6-014: Add CircuitBreaker for worker connections (3h) - Sonnet
V6-015: Implement retry with exponential backoff (2h) - Sonnet
V6-016: Add graceful shutdown + drain mode (2h) - Sonnet
```

#### V6-P5: CLI Integration
```
V6-017: Add `flip2 distributed init` command (2h) - Haiku
V6-018: Add `flip2 distributed coordinator` command (3h) - Sonnet
V6-019: Add `flip2 distributed worker` command (3h) - Sonnet
V6-020: Add `flip2 distributed status` command (2h) - Haiku
V6-021: Add `flip2 distributed cert` subcommands (2h) - Haiku
```

#### V6-P6: Testing
```
V6-022: Unit tests for coordinator (4h) - Haiku
V6-023: Unit tests for worker (4h) - Haiku
V6-024: Integration test with 2 workers (4h) - Sonnet
V6-025: Chaos test (worker disconnect) (3h) - Sonnet
V6-026: Documentation update (2h) - Haiku
```

### 3.3 Model Assignment

| Model | Tasks | Est. Hours | Est. Cost |
|-------|-------|------------|-----------|
| Opus | 1 (V6-004) | 6h | $0.90 |
| Sonnet | 15 | 51h | $1.53 |
| Haiku | 10 | 26h | $0.26 |
| **Total** | **26** | **83h** | **$2.69** |

---

## 4. Migration Path

### 4.1 Backward Compatibility

V6 will support **hybrid mode** during migration:

```
┌─────────────────────────────────────────────────────────────────┐
│                        FLIP2 DAEMON                              │
├─────────────────────────────────────────────────────────────────┤
│  SSE API (Legacy)      │      gRPC API (V6)                     │
│  /api/flip2/realtime   │      :9090 FlipDistributedService      │
│  ────────────────────  │      ──────────────────────────────    │
│  • HTTP/SSE            │      • gRPC/mTLS                       │
│  • API Key auth        │      • Certificate auth                │
│  • Single coordinator  │      • Multi-node capable              │
└─────────────────────────────────────────────────────────────────┘
```

**Migration Steps**:
1. Deploy V6 with SSE bridge (both APIs active)
2. Migrate workers one-by-one to gRPC
3. Monitor for 1 week
4. Deprecate SSE API (configurable flag)
5. Remove SSE code in V7

### 4.2 Configuration

**FLIP2.md project configuration**:
```yaml
# FLIP2.md
distributed:
  enabled: true
  transport: grpc  # or 'sse' for legacy
  coordinator:
    host: 0.0.0.0
    port: 9090
  workers:
    - id: staging-server
      host: 178.156.185.31
      capabilities: [code, test, deploy]
    - id: local-gpu
      host: localhost
      capabilities: [ml, inference]
  pki:
    certs_dir: ~/.flip2/certs
    auto_rotate: true
    validity_days: 7
```

---

## 5. Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| gRPC dependency bloat | Medium | Low | Use grpc-lite, tree-shake unused features |
| Certificate management complexity | Medium | Medium | Automate rotation, provide clear CLI |
| SSE→gRPC migration breaks existing workers | Low | High | Hybrid mode, gradual rollout |
| Network partitions cause task duplication | Medium | High | Idempotency keys, deduplication |
| Proto changes break compatibility | Low | High | Use proto versioning, avoid removing fields |

---

## 6. Success Metrics

### 6.1 Performance Targets

| Metric | Current (SSE) | V6 Target |
|--------|---------------|-----------|
| Task assignment latency | ~100ms | <50ms |
| Heartbeat interval | 30s | 10s |
| Max concurrent workers | 1 (local) | 10+ (distributed) |
| Task throughput | ~20/min | ~100/min |
| Failure detection time | 5min | 30s |

### 6.2 Acceptance Criteria

- [ ] 3+ workers can connect and execute tasks in parallel
- [ ] Zombie tasks are recovered within 60 seconds of worker death
- [ ] Certificate rotation happens automatically without downtime
- [ ] All existing `flip2 agent` commands continue to work
- [ ] Integration tests pass with simulated network failures

---

## 7. Recommendations

### Immediate (This Sprint)

1. **Define Proto First** - Agree on the service contract before implementation
2. **Port PKI Code** - Copy `/pkg/distributed/pki` to flip2 (already working)
3. **Start with Coordinator** - It's the critical path

### Short-Term (Next 2 Sprints)

1. **Add Metrics** - Prometheus metrics for task latency, worker health
2. **Dashboard Integration** - Show distributed workers in web dashboard
3. **Alert on Worker Death** - Integrate with existing alerting system

### Long-Term (Future Releases)

1. **Multi-Coordinator HA** - Raft-based consensus for coordinator failover
2. **Geographic Routing** - Route tasks to nearest worker
3. **Cost-Aware Routing** - Factor in cloud costs when selecting workers

---

## Appendix A: File Locations

### Existing Code to Reuse

| File | Purpose | Action |
|------|---------|--------|
| `/pkg/distributed/pki/pki.go` | Certificate management | Copy to flip2 |
| `/pkg/distributed/coordinator/coordinator.go` | Base coordinator | Refactor for flip2 |
| `/pkg/distributed/node/worker.go` | Base worker | Refactor for flip2 |
| `/distributed_cmd.go` | CLI commands | Port to flip2 cobra |

### New Files to Create

| File | Purpose |
|------|---------|
| `flip2/proto/flip_distributed.proto` | Protocol definitions |
| `flip2/internal/distributed/coordinator.go` | Coordinator service |
| `flip2/internal/distributed/worker.go` | Worker client |
| `flip2/internal/distributed/reaper.go` | Zombie recovery |
| `flip2/internal/distributed/circuit.go` | Circuit breaker |
| `flip2/cmd/flip2/distributed_cmd.go` | CLI commands |

---

## Appendix B: gRPC Dependencies

Add to `go.mod`:
```go
require (
    google.golang.org/grpc v1.60.0
    google.golang.org/protobuf v1.32.0
)
```

Install protoc plugins:
```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

Generate code:
```bash
protoc --go_out=. --go-grpc_out=. proto/flip_distributed.proto
```

---

**Report Complete**

Worker Agent signing off. Ready for coordinator review.
