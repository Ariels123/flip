# FLIP2 Code Quality Review - Quick Reference Guide

## 7 Code Quality Improvements Identified

### 1. Structured Error Types ⭐⭐⭐
**File**: `/Users/arielspivakovsky/src/flip/flip2/internal/llm/process.go`
**Lines**: 180-204
**Priority**: HIGH
**Effort**: Medium

**Issue**: All errors are generic `error`. Callers can't distinguish timeout vs quota vs missing executable.

**Fix**: Create `ExecutionError` type with `Code` field ("timeout", "quota_exhausted", "not_found", "execution_error").

**Benefit**:
- Error handling becomes type-safe
- Can route errors differently: retry timeout, backoff quota, fail immediately for not_found
- Metrics can aggregate by error type

---

### 2. Retry with Exponential Backoff ⭐⭐⭐
**File**: `/Users/arielspivakovsky/src/flip/flip2/internal/llm/process.go`
**Missing**: No retry logic visible; fire-and-forget on failure
**Priority**: HIGH
**Effort**: Medium

**Issue**: No automatic retry for transient failures (network glitch, temporary rate limit).

**Fix**: Add `ExecuteWithRetry()` method with exponential backoff + jitter.
- MaxAttempts: 2-3
- InitialDelay: 500ms
- BackoffFactor: 1.5
- Max retry only for timeout/rate-limit (not for missing executable)

**Benefit**:
- Automatic recovery from transient failures
- Exponential backoff prevents wasted calls during outages
- Jitter prevents synchronized retry storms

---

### 3. Context Cleanup with Defer ⭐⭐⭐
**File**: `/Users/arielspivakovsky/src/flip/flip2/internal/llm/process.go`
**Lines**: 244-269 (Stream method)
**Priority**: HIGH
**Effort**: Low

**Issue**: `cancel()` only called in error paths. Success path relies on timeout to cleanup. Goroutine cleanup is deferred at deepest level (risky if panic).

**Fix**: Add `defer cancel()` immediately after `context.WithTimeout()`. This is Go idiom.

```go
execCtx, cancel := context.WithTimeout(ctx, timeout)
defer cancel()  // <-- Add this
```

**Benefit**:
- Eliminates resource leaks on all code paths
- Prevents goroutine leaks
- Go best practice (every context.With* should have defer)

---

### 4. Streaming with Type-Safe Event Handlers ⭐⭐
**File**: `/Users/arielspivakovsky/src/flip/flip2/internal/llm/process.go`
**Lines**: 266-323
**Priority**: MEDIUM
**Effort**: Medium

**Issue**: Callers get raw `<-chan StreamChunk` and must manually check `Done` flag, accumulate text, extract tokens, handle errors.

**Fix**: Create `AccumulatingStream` wrapper with `.Consume(ctx, handlers)` method.

```go
stream.Consume(ctx, &StreamHandler{
    OnText: func(text string) error { fmt.Print(text); return nil },
    OnComplete: func(content string) error { ... },
    OnError: func(err error) { ... },
})
```

**Benefit**:
- Handlers eliminate error-prone manual loops
- Type-safe (compiler checks handler signature)
- Easy to test handlers independently

---

### 5. Circuit Breaker Pattern ⭐⭐⭐
**File**: `/Users/arielspivakovsky/src/flip/flip2/internal/llm/process.go`
**Lines**: 334-373 (CheckQuota/IsAvailable methods)
**Priority**: HIGH
**Effort**: Medium

**Issue**: Only quota tracked. Other failures don't cause circuit to open. No "fast fail" for repeatedly failing backends.

**Fix**: Add `CircuitBreaker` that tracks:
- State: Closed (ok) → Open (reject calls) → HalfOpen (testing recovery)
- FailureThreshold: Open after N failures (e.g., 3)
- ResetTimeout: Try recovery after 30 seconds

```go
// IsOpen() returns true immediately if circuit is open
if backend.breaker.IsOpen() {
    return nil, "circuit open"
}
```

**Benefit**:
- Prevents cascading failures by failing fast
- Reduces wasted API calls to failing backends
- User gets error immediately instead of timeout

---

### 6. Middleware/Interceptor Pattern ⭐⭐
**File**: `/Users/arielspivakovsky/src/flip/flip2/internal/llm/process.go`
**Lines**: 152-228 (Execute method)
**Priority**: MEDIUM
**Effort**: High

**Issue**: Metrics logic (execCount, lastExec) mixed in Execute(). Adding new concerns (logging, tracing, auth) requires modifying Execute().

**Fix**: Create `Interceptor` interface and `InterceptingBackend` wrapper:

```go
type Interceptor interface {
    BeforeExecute(ctx context.Context, prompt string, opts *Options) error
    AfterExecute(ctx context.Context, resp *Response, err error) error
}

// Usage:
backend := &InterceptingBackend{
    backend: NewClaudeBackend(),
    interceptors: []Interceptor{
        NewMetricsInterceptor(),
        NewLoggingInterceptor(),
        NewTracingInterceptor(),
    },
}
```

**Benefit**:
- Separates concerns (metrics, logging, tracing)
- Each interceptor handles one aspect
- Extensible without modifying Execute()

---

### 7. Structured Logging with Context ⭐⭐
**File**: `/Users/arielspivakovsky/src/flip/flip2/internal/executor/executor.go` (good), `/Users/arielspivakovsky/src/flip/flip2/scripts/signal_monitor.py` (poor)
**Priority**: MEDIUM
**Effort**: Low

**Issue**: Python scripts use `print()` instead of logging. Go partially uses slog but no context propagation.

**Fix**:
- Go: Add context to every log call
  ```go
  logger.ErrorCtx(ctx, "task processing failed",
      "error", err,
      "duration_ms", time.Since(start).Milliseconds(),
  )
  ```
- Python: Use `logging` module with context variables
  ```python
  logger.info("signal monitor started", extra={"agent_id": self.agent_id})
  ```

**Benefit**:
- Logs become queryable by task_id, request_id, agent_id
- Enables log aggregation and filtering
- Reduces string formatting errors

---

## Implementation Roadmap

### Phase 1: High-Impact, Low-Effort (2 weeks)
1. Structured Error Types (#1)
2. Context Cleanup (#3)
3. Structured Logging (#7)

### Phase 2: Reliability Improvements (4 weeks)
4. Retry with Backoff (#2)
5. Circuit Breaker (#5)

### Phase 3: Usability & Extensibility (6+ weeks)
6. Streaming Events (#4)
7. Interceptors (#6)

---

## Quick Checklist for Reviewers

- [ ] Error types defined for each failure mode
- [ ] Every `context.With*()` has `defer cancel()`
- [ ] Retry logic includes jitter to prevent thundering herd
- [ ] Circuit breaker tracks non-quota failures too
- [ ] All logs use structured fields (slog in Go, logging in Python)
- [ ] Handler tests don't require running actual backends
- [ ] Metrics can be aggregated by error type and circuit state
- [ ] Context propagates through all async operations

---

## File References for Implementation

| Pattern | Go Files | Python Files |
|---------|----------|--------------|
| Error Types | `internal/llm/process.go`, `internal/llm/backend.go` | `scripts/signal_monitor.py` |
| Retry | `internal/llm/process.go` (Execute) | `scripts/signal_monitor.py` (api_request) |
| Cleanup | `internal/llm/process.go` (Stream) | N/A |
| Circuit Breaker | `internal/llm/process.go` (CheckQuota) | `scripts/signal_monitor.py` |
| Interceptors | `internal/executor/executor.go` | `internal/api/handlers.go` |
| Logging | `internal/executor/executor.go` (good), `internal/logger/logger.go` | `scripts/signal_monitor.py` (needs work) |

---

**See**: `/Users/arielspivakovsky/src/flip/flip2/CODE_QUALITY_REVIEW_2026.md` for full implementation examples and rationale.
