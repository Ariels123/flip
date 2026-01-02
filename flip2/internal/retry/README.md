# Retry Package - Exponential Backoff with Jitter

The `retry` package implements intelligent retry logic for transient failures in the FLIP system. It combines exponential backoff, jitter, and error-type-aware strategies to maximize reliability while minimizing resource waste.

## Overview

### Core Components

1. **RetryConfig** - Configuration for retry behavior
2. **StrategyConfig** - Per-error-type retry strategies
3. **ExecuteWithRetry** - High-level function for executing with retries
4. **ShouldRetry** - Low-level decision function for retry logic

## RetryConfig Structure

```go
type RetryConfig struct {
    MaxAttempts       int           // Total attempts including initial (default: 3)
    InitialDelay      time.Duration // Delay before first retry (default: 1s)
    MaxDelay          time.Duration // Cap on backoff delay (default: 30s)
    BackoffMultiplier float64       // Exponential growth factor (default: 2.0)
    Jitter            float64       // Random variation 0-1 (default: 0.1 = 10%)
}
```

## Backoff Algorithm

The retry delay is calculated using exponential backoff with jitter:

```
baseDelay = InitialDelay × (BackoffMultiplier ^ retryAttempt)
cappedDelay = min(baseDelay, MaxDelay)
jitterRange = cappedDelay × Jitter
finalDelay = cappedDelay ± random(jitterRange)
```

### Example: Default Configuration

With defaults (MaxAttempts=3, InitialDelay=1s, BackoffMultiplier=2.0, Jitter=0.1):

| Attempt | Type | Delay | Description |
|---------|------|-------|-------------|
| 0 | Initial | - | First attempt, no delay |
| 1 | Retry | ~1.0s | 1s ± 10% jitter |
| 2 | Retry | ~2.0s | 2s ± 10% jitter |
| 3+ | - | - | Max attempts exceeded |

With more retries visible:

| Retry # | Formula | Base Delay | Jitter | Final Range |
|---------|---------|-----------|--------|-------------|
| 1 | 1s × 2^0 | 1s | ±100ms | 0.9-1.1s |
| 2 | 1s × 2^1 | 2s | ±200ms | 1.8-2.2s |
| 3 | 1s × 2^2 | 4s | ±400ms | 3.6-4.4s |
| 4 | 1s × 2^3 | 8s | ±800ms | 7.2-8.8s |
| 5 | 1s × 2^4 | 16s | ±1.6s | 14.4-17.6s |
| 6 | 1s × 2^5 | 32s→30s* | ±3s | 27-33s |

*Capped at MaxDelay (30s)

## Pre-configured Strategies

### DefaultConfig()
Balanced for general use. Reasonable recovery times with limited overload.

```go
MaxAttempts: 3
InitialDelay: 1s
MaxDelay: 30s
BackoffMultiplier: 2.0
Jitter: 0.1 (10%)
```

### AggressiveConfig()
Fast recovery for transient errors expected to resolve quickly.

```go
MaxAttempts: 5          // More attempts
InitialDelay: 100ms     // Start quickly
MaxDelay: 10s           // Shorter max wait
BackoffMultiplier: 1.5  // Slower growth
Jitter: 0.2 (20%)       // More variation
```

### ConservativeConfig()
Slow recovery for errors that may take time to resolve (quota, rate limits).

```go
MaxAttempts: 2          // Fewer retries
InitialDelay: 5s        // Wait longer initially
MaxDelay: 5m            // Long recovery window
BackoffMultiplier: 3.0  // Aggressive growth
Jitter: 0.1 (10%)       // Standard variation
```

## Error-Type Routing

The `StrategyConfig` routes errors to appropriate retry strategies:

```go
strategy := DefaultStrategy()

// Timeout errors: use default strategy (moderate backoff)
config := strategy.GetStrategyForError(errpkg.NewTimeout(...))

// Quota exhausted: use conservative strategy (long waits)
config := strategy.GetStrategyForError(errpkg.NewQuotaExhausted(...))

// General transient: use aggressive strategy (quick recovery)
config := strategy.GetStrategyForError(errpkg.NewExecutionFailed("temp", true, ...))

// Permanent errors: no retries
config := strategy.GetStrategyForError(errpkg.NewNotFound(...))
// config.MaxAttempts == 1 (no retries)
```

## Retry Decision Matrix

| Error Type | Retryable | Strategy | Notes |
|------------|-----------|----------|-------|
| Timeout | Yes | Default | 2x backoff |
| QuotaExhausted | Yes | Conservative | Respects RetryAfter time |
| ExecutionFailed (retryable=true) | Yes | Aggressive | Quick recovery |
| ExecutionFailed (retryable=false) | No | None | No retries |
| NotFound | No | None | Permanent error |
| PermissionDenied | No | None | Requires intervention |
| InvalidConfig | No | None | Requires intervention |
| Canceled | No | None | Intentional stop |
| Unknown Error | No | None | Safe default |

## High-Level API: ExecuteWithRetry

Simplest way to add retry logic to any operation:

```go
ctx := context.Background()
config := retry.DefaultConfig()

err := retry.ExecuteWithRetry(ctx, func(ctx context.Context) error {
    // Your operation here
    return someOperation(ctx)
}, &config)

if err != nil {
    // All retries exhausted or non-retryable error
    log.Fatal("Operation failed:", err)
}
```

### Features

- **Context-aware**: Respects cancellation and deadlines
- **Structured logging**: All retries logged with attempt number and error details
- **Smart backoff**: Automatically calculates delays based on config
- **Error detection**: Handles ExecutionError codes intelligently
- **Quota handling**: Special case for RetryAfter times

### Behavior

1. Executes function once
2. If success (nil error), returns immediately
3. If non-retryable error, returns immediately
4. If retryable error and attempts remain:
   - Logs the retry
   - Waits for calculated delay
   - Checks context cancellation
   - Retries the function
5. If max attempts exceeded, returns last error

## Low-Level API: ShouldRetry

For fine-grained control:

```go
config := retry.DefaultConfig()

for attempt := 0; attempt < config.MaxAttempts; attempt++ {
    err := myOperation()

    if err == nil {
        return nil // Success
    }

    should, delay := config.ShouldRetry(err, attempt)
    if !should {
        return err // Don't retry
    }

    time.Sleep(delay)
}

return err // Max attempts exhausted
```

## Quota Exhaustion Handling

For quota errors with known reset times:

```go
resetTime := time.Now().Add(1 * time.Hour)
quotaErr := errpkg.NewQuotaExhausted("rate limited", resetTime)

config := retry.DefaultConfig()
should, delay := config.ShouldRetry(quotaErr, 1)

if should {
    fmt.Printf("Wait %v for quota reset\n", delay)
    // delay will be ~1 hour
    time.Sleep(delay)
}
```

## Configuration Validation

Always validate custom configs:

```go
config := retry.RetryConfig{
    MaxAttempts: 5,
    InitialDelay: 100 * time.Millisecond,
    MaxDelay: 30 * time.Second,
    BackoffMultiplier: 2.0,
    Jitter: 0.1,
}

if err := config.Validate(); err != nil {
    log.Fatal("Invalid config:", err)
}
```

## Logging

ExecuteWithRetry integrates with the structured logger:

```
DEBUG executing function attempt=1 max_attempts=3 [context fields...]
INFO retrying execution after failure attempt=1 next_delay=1.234s error="Timeout: ..."
DEBUG executing function attempt=2 max_attempts=3 [context fields...]
[success]
```

## Performance Considerations

### Thundering Herd Prevention

Jitter prevents synchronized retries from multiple clients:

```go
// With 10% jitter on 10s delay:
// Client 1: 9.5-10.5s
// Client 2: 9.2-10.8s
// Client 3: 9.7-10.3s
// Spread out naturally, no coordinated spike
```

### Resource Usage

- **Time**: Max total wait time depends on config
  - Default: 1s + 2s + 4s = 7s max (before failure)
  - Conservative: 5s + 15s = 20s max
- **Memory**: Negligible (no state accumulation)
- **CPU**: Minimal (simple math, one sleep per retry)

## Testing

Comprehensive test suite includes:

- Configuration validation
- Backoff delay progression
- Jitter variation
- Error-type routing
- Quota handling with RetryAfter
- Context cancellation
- Max attempt enforcement
- Strategy selection

Run tests:

```bash
go test ./internal/retry/... -v
```

## Error Classification

### Transient (Retryable)

These errors may resolve on retry:
- **Timeout**: Network/execution timeouts
- **ErrQuotaExhausted**: Rate limits or API quotas

### Permanent (Non-Retryable)

These errors require intervention:
- **ErrNotFound**: Resource doesn't exist
- **ErrPermissionDenied**: Insufficient permissions
- **ErrInvalidConfig**: Invalid configuration
- **ErrCanceled**: Operation was explicitly canceled

### Unknown Errors

Errors not from the ExecutionError type are not retried (safe default).

## Best Practices

1. **Use appropriate strategies**: Match your error characteristics
2. **Validate configs**: Always call Validate() for custom configs
3. **Respect context**: Always pass cancellation context
4. **Log carefully**: Let ExecuteWithRetry handle logging
5. **Handle permanent errors first**: Don't waste time retrying permanent failures
6. **Consider quotas**: Use appropriate delays for rate-limited APIs
7. **Test with jitter**: Ensure systems handle variable delays

## Example: LLM API Integration

```go
config := retry.DefaultStrategy().Timeout

err := retry.ExecuteWithRetry(ctx, func(ctx context.Context) error {
    resp, err := llm.Complete(ctx, prompt)
    if err != nil {
        return errors.NewTimeout(5*time.Second, err)
    }
    return nil
}, &config)

// Will retry timeouts with 1s, 2s, 4s delays
// Won't retry permanent errors (missing API key, etc.)
```

## Future Enhancements

- Metrics integration (retry counts, success rates)
- Adaptive backoff based on success rates
- Per-endpoint retry strategies
- Circuit breaker pattern integration
- Request deduplication for idempotent operations
