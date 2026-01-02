# FLIP2 Code Quality Review - Technical Implementation Assessment
## Independent Review Against awesome-claude.ai Best Practices

**Review Date**: January 1, 2026
**Focus**: Code patterns, error handling, async operations, testing, and reliability
**Scope**: FLIP2 framework vs. Claude SDKs, Cookbook, and Agent implementations

---

## Executive Summary

This review examines FLIP2's current code quality against established patterns from:
- **anthropic-sdk-python** (async, error handling, type safety)
- **anthropic-sdk-typescript** (streaming, error recovery)
- **Claude Cookbook** (tool use patterns, RAG, state management)
- **Claude Agent SDK** (multi-agent patterns, orchestration)

**Key Finding**: FLIP2 has solid foundational patterns but can adopt 5-7 specific improvements from the official SDKs for production robustness.

---

## Code Quality Improvements

### 1. STRUCTURED ERROR TYPES WITH SPECIFIC EXCEPTION HIERARCHY

**Pattern Name**: Type-Safe Error Handling with Exception Wrapping

**Source**: anthropic-sdk-python (github.com/anthropics/anthropic-sdk-python)
- Implements hierarchy: `APIError` → (`APIStatusError`, `APIConnectionError`, `RateLimitError`, etc.)
- Status codes mapped to specific exception types (400→BadRequestError, 429→RateLimitError)
- Allows fine-grained error handling without string matching

**Current FLIP2 Code** (`/Users/arielspivakovsky/src/flip/flip2/internal/llm/process.go:180-204`):
```go
if err != nil {
    return nil, fmt.Errorf("failed to start %s: %w", p.command, err)
}
// ... later
if err != nil {
    return nil, fmt.Errorf("%s error: %w, stderr: %s", p.command, err, stderr.String())
}
```

All errors are generic `error` type. Callers cannot distinguish between quota exhaustion, timeout, missing executable, or stderr parsing failures without string inspection.

**Improved Implementation**:
```go
// Define error types matching SDK pattern
type ExecutionError struct {
    Code    string // "timeout", "quota_exhausted", "not_found", "execution_error"
    Message string
    Stderr  string
    Wrapped error
}

func (e *ExecutionError) Error() string {
    return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// In Execute() method
if execCtx.Err() == context.DeadlineExceeded {
    return nil, &ExecutionError{
        Code:    "timeout",
        Message: fmt.Sprintf("execution timeout after %v", timeout),
        Wrapped: execCtx.Err(),
    }
}

if p.isQuotaError(stderr.String()) {
    return nil, &ExecutionError{
        Code:    "quota_exhausted",
        Message: stderr.String(),
        Stderr:  stderr.String(),
        Wrapped: err,
    }
}

// Caller-side usage
resp, err := backend.Execute(ctx, prompt, opts)
if err != nil {
    if execErr, ok := err.(*ExecutionError); ok {
        switch execErr.Code {
        case "quota_exhausted":
            // Handle quota with backoff
        case "timeout":
            // Handle timeout with retry
        default:
            // Generic error
        }
    }
}
```

**Quality Impact**:
- **Testability**: Can mock specific error types in unit tests
- **Reliability**: Error handling code becomes declarative instead of string-based
- **Maintainability**: Error codes serve as contract between layers
- **Observability**: Metrics can aggregate by error type, not just "error occurred"

**Adoption Recommendation**: **ADOPT**
- Add 3-5 specific error types (TimeoutError, QuotaExhaustedError, ProcessNotFoundError)
- Update all process.go error returns to use new types
- Minimal breaking changes (error interface still satisfied)

---

### 2. AUTOMATIC RETRY WITH EXPONENTIAL BACKOFF AND JITTER

**Pattern Name**: Resilient Retry Strategy with Temporal Decay

**Source**: anthropic-sdk-python, anthropic-sdk-typescript
- Both SDKs implement automatic retries for transient failures
- Exponential backoff with jitter prevents thundering herd
- Configurable retry budgets per error type
- Max retry attempts: 2-3 for rate limits, 0 for permanent failures

**Current FLIP2 Code** (`/Users/arielspivakovsky/src/flip/flip2/internal/executor/executor.go`):
```go
// No retry logic visible; execution is fire-and-forget on failure
if err != nil {
    return nil, fmt.Errorf("failed to start %s: %w", p.command, err)
}
```

**Improved Implementation**:
```go
type RetryConfig struct {
    MaxAttempts     int
    InitialDelay    time.Duration
    MaxDelay        time.Duration
    BackoffFactor   float64
    JitterFraction  float64 // 0.1 = 10% jitter
    RetryableErrors map[string]bool // "timeout", "rate_limit", etc.
}

func (p *ProcessBackend) ExecuteWithRetry(ctx context.Context, prompt string, opts *Options) (*Response, error) {
    var lastErr error

    for attempt := 0; attempt < p.retryConfig.MaxAttempts; attempt++ {
        // Execute attempt
        resp, err := p.Execute(ctx, prompt, opts)
        if err == nil {
            return resp, nil
        }

        // Check if error is retryable
        execErr, ok := err.(*ExecutionError)
        if !ok || !p.retryConfig.RetryableErrors[execErr.Code] {
            return nil, err // Non-retryable error
        }

        lastErr = err

        if attempt < p.retryConfig.MaxAttempts-1 {
            // Calculate backoff: exponential + jitter
            baseDelay := time.Duration(math.Pow(
                float64(p.retryConfig.BackoffFactor),
                float64(attempt),
            ) * float64(p.retryConfig.InitialDelay))

            if baseDelay > p.retryConfig.MaxDelay {
                baseDelay = p.retryConfig.MaxDelay
            }

            // Add jitter
            jitter := time.Duration(
                rand.Float64() * float64(baseDelay) * p.retryConfig.JitterFraction,
            )

            delay := baseDelay + jitter

            select {
            case <-time.After(delay):
                // Continue to next attempt
            case <-ctx.Done():
                return nil, ctx.Err()
            }
        }
    }

    return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}
```

**Quality Impact**:
- **Reliability**: Automatic recovery from transient failures (network hiccups, temporary rate limits)
- **Latency**: Exponential backoff prevents wasted API calls during outages
- **Fairness**: Jitter prevents synchronized retry storms across multiple clients
- **Observability**: Retry count metrics identify problematic backends

**Adoption Recommendation**: **ADAPT**
- Implement for timeout and rate-limit errors only (not for missing executables)
- Make retry config per-backend (Claude different from Gemini)
- Start with MaxAttempts=2 and InitialDelay=500ms
- Add metrics: `process_backend_retry_total` and `process_backend_retry_delay_seconds`

---

### 3. CONTEXT-AWARE RESOURCE CLEANUP WITH DEFER PATTERNS

**Pattern Name**: Guaranteed Resource Release with Hierarchical Context

**Source**: anthropic-sdk-typescript (context managers), anthropic-sdk-python (async context managers)
- Uses `async with` for guaranteed cleanup even on exceptions
- Cancellation propagates down context hierarchy
- No orphaned resources or file handles

**Current FLIP2 Code** (`/Users/arielspivakovsky/src/flip/flip2/internal/llm/process.go:244-269`):
```go
execCtx, cancel := context.WithTimeout(ctx, timeout)

// ... but cancel is NOT deferred; called only in error paths
if err != nil {
    cancel()
    return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
}

// Missing: cancel() called in success path - ctx will timeout by itself
// Risk: If no timeout set, cancel is never called
```

The Stream method creates goroutines that depend on `cancel()` cleanup:
```go
go func() {
    defer close(ch)
    defer cancel()  // Only here - if goroutine panics before this, leak!
    // ...
}()
```

**Improved Implementation**:
```go
func (p *ProcessBackend) Stream(ctx context.Context, prompt string, opts *Options) (<-chan StreamChunk, error) {
    ch := make(chan StreamChunk, 100)

    // ... setup code ...

    execCtx, cancel := context.WithTimeout(ctx, timeout)
    defer cancel()  // ALWAYS call, regardless of success/failure

    cmd := exec.CommandContext(execCtx, p.command, args...)

    stdout, err := cmd.StdoutPipe()
    if err != nil {
        // cancel() WILL be called by defer above
        return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
    }

    go func() {
        defer close(ch)
        // No need to defer cancel() - outer function will handle it
        // Instead, use execCtx for cancellation signals

        // Stream stdout
        var fullContent strings.Builder
        reader := bufio.NewReader(stdout)

        for {
            select {
            case <-execCtx.Done():
                ch <- StreamChunk{Error: execCtx.Err(), Done: true}
                return
            default:
                // Read with timeout
                char, _, err := reader.ReadRune()
                if err != nil {
                    if err != io.EOF {
                        ch <- StreamChunk{Error: err, Done: true}
                    }
                    break
                }

                text := string(char)
                fullContent.WriteString(text)

                select {
                case ch <- StreamChunk{Text: text}:
                case <-execCtx.Done():
                    ch <- StreamChunk{Error: execCtx.Err(), Done: true}
                    return
                }
            }
        }
    }()

    return ch, nil
}
```

**Quality Impact**:
- **Safety**: Eliminates resource leaks on error paths
- **Debuggability**: Stack traces clearly show resource allocation points
- **Concurrency**: Prevents goroutine leaks from forgotten cancellations
- **Clarity**: Defer statements make intent explicit

**Adoption Recommendation**: **ADOPT**
- Audit all `context.WithTimeout` and `context.WithCancel` calls
- Add `defer cancel()` immediately after creation (Go idiom)
- Test with `-race` flag to catch synchronization issues
- Update linter rules to require defer immediately after context creation

---

### 4. STRUCTURED STREAMING WITH TYPE-SAFE EVENT HANDLERS

**Pattern Name**: Accumulating Streaming Responses with Event Callbacks

**Source**: anthropic-sdk-typescript (stream.on()), anthropic-sdk-python (helpers.md)
- TypeScript: `stream.on('text', handler)` for specific event types
- Python: `text_stream` lens for text-only iteration
- Both: Type-safe final message accumulation

**Current FLIP2 Code** (`/Users/arielspivakovsky/src/flip/flip2/internal/llm/process.go:266-323`):
```go
// Returns raw channel of chunks; caller must handle accumulation
for {
    select {
    case chunk := <-ch:
        if chunk.Done {
            return chunk.InputTokens, chunk.OutputTokens, nil
        }
        fullContent.WriteString(chunk.Text)
    }
}
```

Callers must manually:
- Check `chunk.Done` flag
- Accumulate `chunk.Text`
- Extract tokens from final chunk
- Handle errors mixed into channel

**Improved Implementation**:
```go
// StreamHandler is called for each chunk type
type StreamHandler struct {
    OnText     func(text string) error
    OnTokens   func(input, output int) error
    OnComplete func(content string) error
    OnError    func(err error)
}

// AccumulatingStream manages the accumulation state
type AccumulatingStream struct {
    chunks       <-chan StreamChunk
    content      strings.Builder
    inputTokens  int
    outputTokens int
}

// Text returns accumulated text so far
func (s *AccumulatingStream) Text() string {
    return s.content.String()
}

// Consume processes all chunks with optional handlers
func (s *AccumulatingStream) Consume(ctx context.Context, h *StreamHandler) error {
    if h == nil {
        h = &StreamHandler{}
    }

    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case chunk := <-s.chunks:
            if chunk.Error != nil {
                if h.OnError != nil {
                    h.OnError(chunk.Error)
                }
                return chunk.Error
            }

            if chunk.Done {
                s.inputTokens = chunk.InputTokens
                s.outputTokens = chunk.OutputTokens

                if h.OnTokens != nil {
                    if err := h.OnTokens(chunk.InputTokens, chunk.OutputTokens); err != nil {
                        return err
                    }
                }

                if h.OnComplete != nil {
                    if err := h.OnComplete(s.content.String()); err != nil {
                        return err
                    }
                }

                return nil
            }

            s.content.WriteString(chunk.Text)
            if h.OnText != nil {
                if err := h.OnText(chunk.Text); err != nil {
                    return err
                }
            }
        }
    }
}

// Caller usage: Type-safe, readable
func streamExample(backend llm.Backend) error {
    ch, err := backend.Stream(ctx, "Hello", opts)
    if err != nil {
        return err
    }

    stream := &AccumulatingStream{chunks: ch}
    return stream.Consume(ctx, &StreamHandler{
        OnText: func(text string) error {
            fmt.Print(text) // Real-time output
            return nil
        },
        OnComplete: func(content string) error {
            fmt.Printf("\nFinal content: %s\n", content)
            return nil
        },
        OnError: func(err error) {
            log.Printf("Stream error: %v", err)
        },
    })
}
```

**Quality Impact**:
- **Usability**: Handlers replace error-prone manual loop logic
- **Testing**: Easy to mock handlers in tests
- **Composability**: Handlers can be chained (logging + metrics + processing)
- **Type Safety**: Compiler checks handler signature compatibility

**Adoption Recommendation**: **ADAPT**
- Create AccumulatingStream wrapper type
- Keep raw channel API for low-level access
- Document handler error semantics (continue or stop?)
- Add example in godoc showing handler usage

---

### 5. CIRCUIT BREAKER PATTERN FOR QUOTA MANAGEMENT

**Pattern Name**: Fail-Fast on Known Exhaustion (Circuit Breaker)

**Source**: Claude Cookbook (memory management), SDK error handling patterns
- Prevents repeated calls to exhausted backends
- Tracks reset time and auto-recovery
- Allows graceful degradation to fallback backends

**Current FLIP2 Code** (`/Users/arielspivakovsky/src/flip/flip2/internal/llm/process.go:334-373`):
```go
func (p *ProcessBackend) CheckQuota(ctx context.Context) (float64, error) {
    p.mu.RLock()
    exhausted := p.quotaExhausted
    resetsAt := p.quotaResetsAt
    p.mu.RUnlock()

    if exhausted {
        if time.Now().After(resetsAt) {
            // Reset quota by flipping flag
            p.mu.Lock()
            p.quotaExhausted = false
            p.mu.Unlock()
            return 1.0, nil
        }
        return 0.0, nil
    }

    return 1.0, nil
}

// But: IsAvailable() only checks quota, doesn't track recent failures
func (p *ProcessBackend) IsAvailable(ctx context.Context) bool {
    // Returns true even if last 5 calls failed
    return !p.quotaExhausted
}
```

Issues:
- Only quota is tracked; other failures not considered
- No distinction between "recently failing" and "healthy"
- Circuit never opens for non-quota errors

**Improved Implementation**:
```go
type CircuitState string

const (
    CircuitClosed   CircuitState = "closed"   // Normal operation
    CircuitOpen     CircuitState = "open"     // Failing, reject calls
    CircuitHalfOpen CircuitState = "half_open" // Testing recovery
)

type CircuitBreaker struct {
    state             CircuitState
    failureCount      int
    lastFailureTime   time.Time
    halfOpenTestAt    time.Time
    failureThreshold  int           // Open after N failures
    resetTimeout      time.Duration // Try recovery after timeout
    successCount      int           // For half-open recovery
}

// OnSuccess marks a successful execution
func (cb *CircuitBreaker) OnSuccess() {
    if cb.state == CircuitHalfOpen {
        cb.successCount++
        if cb.successCount >= 2 { // 2 successes = fully recovered
            cb.state = CircuitClosed
            cb.failureCount = 0
            cb.successCount = 0
        }
    } else if cb.state == CircuitClosed {
        cb.failureCount = 0
    }
}

// OnFailure marks a failed execution
func (cb *CircuitBreaker) OnFailure() {
    cb.failureCount++
    cb.lastFailureTime = time.Now()

    if cb.state == CircuitClosed && cb.failureCount >= cb.failureThreshold {
        cb.state = CircuitOpen
    } else if cb.state == CircuitHalfOpen {
        cb.state = CircuitOpen
        cb.successCount = 0
    }
}

// IsOpen checks if circuit is open (rejecting calls)
func (cb *CircuitBreaker) IsOpen() bool {
    if cb.state == CircuitOpen {
        // Try recovery if timeout elapsed
        if time.Since(cb.lastFailureTime) > cb.resetTimeout {
            cb.state = CircuitHalfOpen
            cb.successCount = 0
            cb.halfOpenTestAt = time.Now()
            return false // Allow test call
        }
        return true
    }
    return false
}

// In ProcessBackend Execute()
func (p *ProcessBackend) Execute(ctx context.Context, prompt string, opts *Options) (*Response, error) {
    if p.breaker.IsOpen() {
        return nil, &ExecutionError{
            Code:    "circuit_open",
            Message: "backend is unavailable (circuit breaker open)",
        }
    }

    resp, err := p.executeInternal(ctx, prompt, opts)

    if err != nil {
        p.breaker.OnFailure()
        return nil, err
    }

    p.breaker.OnSuccess()
    return resp, nil
}
```

**Quality Impact**:
- **Reliability**: Prevents cascading failures by failing fast
- **User Experience**: Returns error immediately instead of timeout
- **Cost**: Reduces wasted API calls to failing backends
- **Observability**: Circuit state is a key metric

**Adoption Recommendation**: **ADOPT**
- Implement for all process backends
- Add metrics: `circuit_breaker_state{backend,state}` (gauge)
- Add tracing: log circuit state transitions
- Configure threshold=3 failures, resetTimeout=30s initially
- Document circuit breaker states in API docs

---

### 6. MIDDLEWARE/INTERCEPTOR PATTERN FOR CROSS-CUTTING CONCERNS

**Pattern Name**: Request/Response Interception for Logging, Metrics, Tracing

**Source**: SDK patterns, middleware in web frameworks
- Separates concerns: logging, metrics, auth from core logic
- Enables chaining multiple concerns (e.g., trace → metrics → auth → execute)
- Easier to test each concern independently

**Current FLIP2 Code** (`/Users/arielspivakovsky/src/flip/flip2/internal/llm/process.go:152-228`):
```go
// All concerns mixed in Execute()
func (p *ProcessBackend) Execute(ctx context.Context, prompt string, opts *Options) (*Response, error) {
    start := time.Now()  // Timing

    // ... execution code ...

    // Update metrics (embedded)
    p.mu.Lock()
    p.execCount++
    p.lastExec = time.Now()
    p.mu.Unlock()

    return &Response{
        // ...
        Latency: time.Since(start),  // Metric
    }, nil
}
```

Issues:
- Metrics logic commingled with execution
- Adding new concerns (tracing, auth) requires modifying Execute()
- Difficult to test metric collection without executing backend

**Improved Implementation**:
```go
// Interceptor interface for middleware
type Interceptor interface {
    BeforeExecute(ctx context.Context, prompt string, opts *Options) error
    AfterExecute(ctx context.Context, resp *Response, err error) error
}

// MetricsInterceptor collects latency, token counts, cost
type MetricsInterceptor struct {
    startTime time.Time
}

func (i *MetricsInterceptor) BeforeExecute(ctx context.Context, prompt string, opts *Options) error {
    i.startTime = time.Now()
    return nil
}

func (i *MetricsInterceptor) AfterExecute(ctx context.Context, resp *Response, err error) error {
    latency := time.Since(i.startTime)
    // Record metrics
    recordLatency(latency)
    if resp != nil {
        recordTokens(resp.InputTokens, resp.OutputTokens)
        recordCost(resp.CostUSD)
    }
    return nil
}

// LoggingInterceptor adds structured logging
type LoggingInterceptor struct {
    logger *slog.Logger
}

func (i *LoggingInterceptor) BeforeExecute(ctx context.Context, prompt string, opts *Options) error {
    i.logger.InfoCtx(ctx, "executing backend",
        "prompt_length", len(prompt),
        "model", opts.Model,
    )
    return nil
}

func (i *LoggingInterceptor) AfterExecute(ctx context.Context, resp *Response, err error) error {
    if err != nil {
        i.logger.ErrorCtx(ctx, "execution failed", "error", err)
    } else {
        i.logger.InfoCtx(ctx, "execution succeeded",
            "output_length", len(resp.Content),
            "tokens", resp.OutputTokens,
        )
    }
    return nil
}

// InterceptingBackend wraps any backend with interceptors
type InterceptingBackend struct {
    backend       Backend
    interceptors  []Interceptor
}

func (ib *InterceptingBackend) Execute(ctx context.Context, prompt string, opts *Options) (*Response, error) {
    // Before
    for _, i := range ib.interceptors {
        if err := i.BeforeExecute(ctx, prompt, opts); err != nil {
            return nil, err
        }
    }

    // Execute
    resp, err := ib.backend.Execute(ctx, prompt, opts)

    // After (always runs, even on error)
    for _, i := range ib.interceptors {
        i.AfterExecute(ctx, resp, err)
    }

    return resp, err
}

// Usage
backend := &InterceptingBackend{
    backend: NewClaudeBackend(),
    interceptors: []Interceptor{
        NewLogggingInterceptor(logger),
        NewMetricsInterceptor(),
        NewTracingInterceptor(),
    },
}
```

**Quality Impact**:
- **Separation of Concerns**: Each interceptor handles one aspect
- **Testability**: Easy to test metrics collection independently
- **Extensibility**: Add new interceptors without modifying Execute()
- **Observability**: Tracing interceptor can set span attributes

**Adoption Recommendation**: **ADAPT**
- Create MetricsInterceptor and LoggingInterceptor initially
- Keep InterceptingBackend wrapper optional (backward compatible)
- Add tracing interceptor later
- Document interceptor contract (execution order matters)

---

### 7. TYPE-SAFE STRUCTURED LOGGING WITH CONTEXT ATTRIBUTES

**Pattern Name**: Contextual Logging with Field Aggregation

**Source**: Go `log/slog` best practices, anthropic-sdk-python structured logging
- Uses `slog.Logger` with context and structured fields
- Enables log aggregation, filtering, sampling
- Reduces string formatting errors

**Current FLIP2 Code** (`/Users/arielspivakovsky/src/flip/flip2/scripts/signal_monitor.py:40-44`):
```python
def log(self, msg):
    timestamp = datetime.now().strftime("%H:%M:%S")
    print(f"[{timestamp}] {msg}")  # String formatting, no structure
```

And (`/Users/arielspivakovsky/src/flip/flip2/internal/executor/executor.go:34-35`):
```go
executorLogger := logger.WithGroup("executor")

e.logger.Info("Executor started", "max_concurrent", cap(e.semaphore))
```

Good usage of slog here, but inconsistency in scripts. No context propagation.

**Improved Implementation**:
```go
// Add context to every log call
func (e *Executor) processTask(ctx context.Context, taskID string) {
    // Bind context to logger for automatic attribute propagation
    logger := e.logger.WithGroup("task").With(
        "task_id", taskID,
        "request_id", ctx.Value("request_id"), // From parent context
    )

    logger.DebugCtx(ctx, "task processing started")

    // ... actual work ...

    if err != nil {
        logger.ErrorCtx(ctx, "task processing failed",
            "error", err,
            "duration_ms", time.Since(start).Milliseconds(),
        )
        return
    }

    logger.InfoCtx(ctx, "task processing completed",
        "duration_ms", time.Since(start).Milliseconds(),
        "result_size", len(result),
    )
}

// Python equivalent using logging.LoggerAdapter
import logging
from contextvars import ContextVar

_request_id = ContextVar("request_id", default="unknown")

class ContextualLoggerAdapter(logging.LoggerAdapter):
    def process(self, msg, kwargs):
        # Inject context variables into every log
        request_id = _request_id.get()
        kwargs["extra"] = {
            "request_id": request_id,
            **kwargs.get("extra", {}),
        }
        return msg, kwargs

logger = ContextualLoggerAdapter(
    logging.getLogger("flip2"),
    extra={}
)

def signal_monitor_run(self):
    # Set context at entry point
    _request_id.set(str(uuid.uuid4()))

    logger = ContextualLoggerAdapter(
        logging.getLogger("signal_monitor"),
        extra={"agent_id": self.agent_id}
    )

    logger.info("signal monitor started")
```

**Quality Impact**:
- **Observability**: Logs can be queried by task_id, request_id, etc.
- **Debugging**: Context variables make distributed tracing easier
- **Testing**: Assert on structured fields instead of string matching
- **Performance**: Lazy evaluation of log fields (only evaluated if logged)

**Adoption Recommendation**: **ADOPT**
- Migrate all `fmt.Printf` logging to `slog`
- Add context propagation to all critical paths
- Define standard context keys (request_id, task_id, agent_id)
- Update Python scripts to use logging module instead of print()

---

## Summary Table

| # | Pattern | Source | Current State | Recommendation | Effort | Impact |
|---|---------|--------|----------------|-----------------|--------|--------|
| 1 | Structured Error Types | SDK Python | ❌ Generic errors | ADOPT | Medium | High |
| 2 | Retry with Backoff | SDK Python/TS | ❌ No retry logic | ADAPT | Medium | High |
| 3 | Context Cleanup | SDK TS | ⚠️ Partial (no defer) | ADOPT | Low | High |
| 4 | Streaming Events | SDK TS | ⚠️ Raw channel | ADAPT | Medium | Medium |
| 5 | Circuit Breaker | Cookbook | ⚠️ Quota only | ADOPT | Medium | High |
| 6 | Interceptors | Web frameworks | ❌ Mixed concerns | ADAPT | High | High |
| 7 | Structured Logging | slog/SDK | ⚠️ Partial (Go ok) | ADOPT | Low | Medium |

**Legend**: ❌ Missing | ⚠️ Incomplete | ✅ Present

---

## Implementation Roadmap

### Phase 1 (High Priority - 2 weeks)
1. **Structured Error Types** (#1) - Unblock all error handling improvements
2. **Context Cleanup** (#3) - Eliminate resource leaks
3. **Structured Logging** (#7) - Improve observability immediately

### Phase 2 (Medium Priority - 4 weeks)
4. **Retry with Backoff** (#2) - Improve reliability for transient failures
5. **Circuit Breaker** (#5) - Prevent cascade failures

### Phase 3 (Enhancement - 6+ weeks)
6. **Streaming Events** (#4) - Improve usability (API change, coordinate with callers)
7. **Interceptors** (#6) - Large refactoring, test thoroughly before deploying

---

## Testing Strategy

For each improvement:

1. **Unit Tests**: Mock next layer (e.g., mock Backend for CircuitBreaker)
2. **Integration Tests**: Real backends with failure injection
3. **Chaos Testing**: Kill CLI processes, simulate timeouts
4. **Load Testing**: Verify backoff doesn't cause thundering herd

Example test for Retry:
```go
func TestProcessBackend_RetryOnTimeout(t *testing.T) {
    attempts := 0
    backend := &MockBackend{
        ExecuteFn: func(ctx context.Context, prompt string, opts *Options) (*Response, error) {
            attempts++
            if attempts < 2 {
                return nil, &ExecutionError{Code: "timeout"}
            }
            return &Response{Content: "success"}, nil
        },
    }

    wrappedBackend := &RetryingBackend{
        backend: backend,
        config: RetryConfig{
            MaxAttempts: 3,
            InitialDelay: 10 * time.Millisecond,
        },
    }

    resp, err := wrappedBackend.Execute(context.Background(), "test", nil)
    require.NoError(t, err)
    require.Equal(t, "success", resp.Content)
    require.Equal(t, 2, attempts) // Verify retried once
}
```

---

## References

- [anthropic-sdk-python](https://github.com/anthropics/anthropic-sdk-python)
- [anthropic-sdk-typescript](https://github.com/anthropics/anthropic-sdk-typescript)
- [Claude Cookbook](https://github.com/anthropics/anthropic-cookbook)
- [Claude Agent SDK](https://github.com/anthropics/anthropic-sdk-python)
- Go Standard Library: context, log/slog
- Release Notes: Claude Opus 4.5, Claude Sonnet 4 (December 2025)

---

**Review Author**: Independent Code Quality Assessment
**Review Period**: January 1, 2026
**Status**: Ready for Architecture Review Committee
