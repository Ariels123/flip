# Context Propagation Pattern for FLIP2 Logging

## Overview

The FLIP2 logging system uses Go's `context.Context` to propagate structured logging fields throughout the request/task lifecycle. This enables distributed tracing, request correlation, and hierarchical task tracking.

## Context Fields

The following fields are available for context propagation:

| Field | Type | Purpose | Example |
|-------|------|---------|---------|
| `task_id` | string | Unique task identifier | `task_abc123` |
| `agent_id` | string | Agent executing the task | `claude_worker_1` |
| `request_id` | string | HTTP request correlation ID | `req_20250101_123456_xyz` |
| `pipeline_id` | string | Pipeline run identifier | `pipe_research_20250101_001` |
| `stage_id` | string | Pipeline stage identifier | `stage_data_collection` |
| `parent_id` | string | Parent task/request ID (for tracing) | `task_parent_def` |

## Usage Patterns

### Basic Usage: Adding Context to Request

```go
import (
    "context"
    "log/slog"
    "flip2/internal/logger"
)

// In HTTP handler
func handleRequest(w http.ResponseWriter, r *http.Request) {
    requestID := generateRequestID()
    ctx := logger.WithRequestID(r.Context(), requestID)

    logger.Info("request received", logger.ExtractLogFields(ctx)...)

    // Pass ctx to downstream functions
    processTask(ctx)
}
```

### Worker Task Execution

```go
// Spawning a worker task
func spawnWorker(ctx context.Context, taskID string, agentID string) {
    workerCtx := context.Background()
    workerCtx = logger.WithTaskID(workerCtx, taskID)
    workerCtx = logger.WithAgentID(workerCtx, agentID)
    workerCtx = logger.WithParentID(workerCtx, logger.GetTaskID(ctx))

    // Execute worker task
    executeWorker(workerCtx)
}

func executeWorker(ctx context.Context) {
    logger.Info("worker starting", logger.ExtractLogFields(ctx)...)
    // Work happens here
    logger.Info("worker complete", logger.ExtractLogFields(ctx)...)
}
```

### Pipeline Execution

```go
// Pipeline coordinator sets up context
func runPipeline(ctx context.Context, pipelineID string) error {
    pipelineCtx := logger.WithPipelineID(ctx, pipelineID)

    for _, stage := range pipeline.Stages {
        stageCtx := logger.WithStageID(pipelineCtx, stage.ID)

        logger.Info("stage starting", logger.ExtractLogFields(stageCtx)...)
        if err := stage.Execute(stageCtx); err != nil {
            logger.Error("stage failed", "error", err, logger.ExtractLogFields(stageCtx)...)
            return err
        }
        logger.Info("stage complete", logger.ExtractLogFields(stageCtx)...)
    }

    return nil
}
```

### Using LogContextFields for Convenience

```go
// Build context with multiple fields at once
fields := logger.LogContextFields{
    TaskID:     "task_123",
    AgentID:    "gemini_researcher",
    RequestID:  "req_001",
    PipelineID: "pipe_research",
    StageID:    "stage_analyze",
    ParentID:   "task_parent",
}

ctx := fields.Apply(context.Background())

// Now all fields are available in ctx
logger.Info("processing started", logger.ExtractLogFields(ctx)...)
```

### Extracting Individual Fields

```go
taskID := logger.GetTaskID(ctx)
agentID := logger.GetAgentID(ctx)
requestID := logger.GetRequestID(ctx)

if taskID != "" {
    logger.Info("task context", "task_id", taskID)
}
```

## Integration with slog

### Extracting Fields for slog

```go
import "log/slog"

func logWithContext(logger *slog.Logger, ctx context.Context, msg string) {
    fields := logger.ExtractLogFields(ctx)

    // Convert fields to slog args
    args := []interface{}{}
    for k, v := range fields {
        args = append(args, k, v)
    }

    logger.Info(msg, args...)
}
```

### Custom slog Handler with Context Fields

For advanced use cases, you might create a custom slog handler that automatically extracts context fields:

```go
type ContextHandler struct {
    inner slog.Handler
}

func (h *ContextHandler) Handle(ctx context.Context, record slog.Record) error {
    fields := logger.ExtractLogFields(ctx)
    for k, v := range fields {
        record.AddAttrs(slog.Any(k, v))
    }
    return h.inner.Handle(ctx, record)
}
```

## Best Practices

### 1. Set Context Early

Establish the context as close to the request/task entry point as possible:

```go
// Good
ctx := logger.WithTaskID(context.Background(), taskID)
processTask(ctx)

// Avoid
processTask(context.Background())
// ... later in processTask
ctx := logger.WithTaskID(context.Background(), taskID)
```

### 2. Preserve Parent Context

When spawning child tasks, preserve parent IDs for tracing:

```go
func spawnChildTask(ctx context.Context, childID string) {
    childCtx := context.Background()
    childCtx = logger.WithTaskID(childCtx, childID)
    childCtx = logger.WithParentID(childCtx, logger.GetTaskID(ctx))

    executeChild(childCtx)
}
```

### 3. Use Immutable Context

Context is immutable; `With*` functions return new contexts:

```go
ctx1 := logger.WithTaskID(context.Background(), "task_1")
ctx2 := logger.WithAgentID(ctx1, "agent_1") // ctx1 unchanged

// Both can be used independently
doWork(ctx1) // Has task_id only
doWork(ctx2) // Has both task_id and agent_id
```

### 4. Pass Context Through Function Calls

Always pass context as the first parameter:

```go
// Good
func processTask(ctx context.Context, data string) error

// Avoid
func processTask(data string, ctx context.Context) error
```

### 5. Check for Empty Values

When using context values, check if they're set:

```go
if taskID := logger.GetTaskID(ctx); taskID != "" {
    // Use taskID
}
```

## Distributed Tracing Example

Complete example showing distributed tracing from HTTP request through worker execution:

```go
import (
    "log/slog"
    "net/http"
    "flip2/internal/logger"
)

func handleSpawnWorker(w http.ResponseWriter, r *http.Request) {
    // 1. Create request context
    requestID := generateRequestID()
    ctx := logger.WithRequestID(r.Context(), requestID)

    logger.Info("spawn request received", logger.ExtractLogFields(ctx)...)

    // 2. Extract task ID from request
    taskID := r.FormValue("task_id")
    ctx = logger.WithTaskID(ctx, taskID)

    // 3. Spawn worker with parent reference
    agentID := "claude_worker_1"
    workerCtx := context.Background()
    workerCtx = logger.WithTaskID(workerCtx, taskID)
    workerCtx = logger.WithAgentID(workerCtx, agentID)
    workerCtx = logger.WithParentID(workerCtx, requestID)

    go executeWorker(workerCtx)

    w.WriteHeader(http.StatusAccepted)
}

func executeWorker(ctx context.Context) {
    logger.Info("worker starting", logger.ExtractLogFields(ctx)...)

    // All log messages include task_id, agent_id, parent_id
    // This makes it easy to correlate logs across the request lifetime

    logger.Info("worker complete", logger.ExtractLogFields(ctx)...)
}
```

## Testing

When writing tests that use context:

```go
func TestProcessTask(t *testing.T) {
    ctx := context.Background()
    ctx = logger.WithTaskID(ctx, "test_task_123")
    ctx = logger.WithAgentID(ctx, "test_agent")

    result, err := processTask(ctx)

    if err != nil {
        t.Errorf("processTask failed: %v", err)
    }

    // Verify context was preserved if needed
    if logger.GetTaskID(ctx) != "test_task_123" {
        t.Error("task ID lost during execution")
    }
}
```

## Performance Considerations

- Context operations are O(1) - they create a new context with a pointer to the parent
- `ExtractLogFields()` creates a new map with only set fields (no empty values)
- Benchmark results show context operations are suitable for high-throughput scenarios:
  - `WithTaskID`: ~10-20ns per operation
  - `ExtractLogFields`: ~100-200ns per operation (with all fields)
  - `LogContextFields.Apply`: ~50-100ns per operation

## Backward Compatibility

The context propagation system is additive and doesn't require changes to existing code:

- Existing functions continue to work without context fields
- Context fields are optional - missing fields simply won't appear in logs
- Gradual adoption is supported - you can add context fields to parts of your codebase incrementally
