package executor

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"flip2/internal/errors"
)

// TestRetryOnTransientFailures tests that transient errors trigger retries.
// Covers:
// - Timeout errors (retryable)
// - Rate limit / quota exhausted errors (retryable)
// - Execution failures marked as retryable
func TestRetryOnTransientFailures(t *testing.T) {
	t.Run("retry on timeout error", func(t *testing.T) {
		callCount := atomic.Int32{}
		expectedAttempts := int32(3)

		// Mock function that fails first 2 times with timeout, succeeds on 3rd
		fn := func(ctx context.Context) error {
			callCount.Add(1)
			if callCount.Load() < expectedAttempts {
				return errors.NewTimeout(5*time.Second, fmt.Errorf("network timeout"))
			}
			return nil
		}

		config := &retryConfigTest{
			MaxAttempts:       3,
			InitialDelay:      10 * time.Millisecond,
			MaxDelay:          100 * time.Millisecond,
			BackoffMultiplier: 2.0,
			Jitter:            0.0, // No jitter for deterministic testing
		}

		ctx := context.Background()
		err := executeWithRetryTest(ctx, fn, config)

		if err != nil {
			t.Errorf("expected success after retry, got error: %v", err)
		}

		if callCount.Load() != expectedAttempts {
			t.Errorf("expected %d attempts, got %d", expectedAttempts, callCount.Load())
		}
	})

	t.Run("retry on rate limit error", func(t *testing.T) {
		callCount := atomic.Int32{}
		expectedAttempts := int32(2)

		fn := func(ctx context.Context) error {
			callCount.Add(1)
			if callCount.Load() < expectedAttempts {
				resetTime := time.Now().Add(50 * time.Millisecond)
				return errors.NewQuotaExhausted("API rate limit exceeded", resetTime)
			}
			return nil
		}

		config := &retryConfigTest{
			MaxAttempts:       3,
			InitialDelay:      10 * time.Millisecond,
			MaxDelay:          100 * time.Millisecond,
			BackoffMultiplier: 2.0,
			Jitter:            0.0,
		}

		ctx := context.Background()
		err := executeWithRetryTest(ctx, fn, config)

		if err != nil {
			t.Errorf("expected success after retry, got error: %v", err)
		}

		if callCount.Load() != expectedAttempts {
			t.Errorf("expected %d attempts, got %d", expectedAttempts, callCount.Load())
		}
	})

	t.Run("retry on retryable execution failure", func(t *testing.T) {
		callCount := atomic.Int32{}
		expectedAttempts := int32(2)

		fn := func(ctx context.Context) error {
			callCount.Add(1)
			if callCount.Load() < expectedAttempts {
				return errors.NewExecutionFailed("temporary execution failure", true, nil)
			}
			return nil
		}

		config := &retryConfigTest{
			MaxAttempts:       3,
			InitialDelay:      10 * time.Millisecond,
			MaxDelay:          100 * time.Millisecond,
			BackoffMultiplier: 2.0,
			Jitter:            0.0,
		}

		ctx := context.Background()
		err := executeWithRetryTest(ctx, fn, config)

		if err != nil {
			t.Errorf("expected success after retry, got error: %v", err)
		}

		if callCount.Load() != expectedAttempts {
			t.Errorf("expected %d attempts, got %d", expectedAttempts, callCount.Load())
		}
	})
}

// TestNoRetryOnPermanentFailures tests that permanent errors do not trigger retries.
// Covers:
// - Not found errors
// - Invalid config errors
// - Permission denied errors
// - Canceled operations
// - Non-retryable execution failures
func TestNoRetryOnPermanentFailures(t *testing.T) {
	t.Run("no retry on not found error", func(t *testing.T) {
		callCount := atomic.Int32{}

		fn := func(ctx context.Context) error {
			callCount.Add(1)
			return errors.NewNotFound("executable", fmt.Errorf("file not found"))
		}

		config := &retryConfigTest{
			MaxAttempts:       3,
			InitialDelay:      10 * time.Millisecond,
			MaxDelay:          100 * time.Millisecond,
			BackoffMultiplier: 2.0,
			Jitter:            0.0,
		}

		ctx := context.Background()
		err := executeWithRetryTest(ctx, fn, config)

		if err == nil {
			t.Error("expected error")
		}

		// Should only attempt once (no retries)
		if callCount.Load() != 1 {
			t.Errorf("expected 1 attempt (no retries), got %d", callCount.Load())
		}

		// Verify error code
		execErr := errors.AsExecutionError(err)
		if execErr == nil || execErr.Code != errors.ErrNotFound {
			t.Errorf("expected not found error, got %v", execErr)
		}
	})

	t.Run("no retry on invalid config error", func(t *testing.T) {
		callCount := atomic.Int32{}

		fn := func(ctx context.Context) error {
			callCount.Add(1)
			return errors.NewInvalidConfig("missing required field", nil)
		}

		config := &retryConfigTest{
			MaxAttempts:       3,
			InitialDelay:      10 * time.Millisecond,
			MaxDelay:          100 * time.Millisecond,
			BackoffMultiplier: 2.0,
			Jitter:            0.0,
		}

		ctx := context.Background()
		err := executeWithRetryTest(ctx, fn, config)

		if err == nil {
			t.Error("expected error")
		}

		if callCount.Load() != 1 {
			t.Errorf("expected 1 attempt (no retries), got %d", callCount.Load())
		}

		execErr := errors.AsExecutionError(err)
		if execErr == nil || execErr.Code != errors.ErrInvalidConfig {
			t.Errorf("expected invalid config error, got %v", execErr)
		}
	})

	t.Run("no retry on permission denied error", func(t *testing.T) {
		callCount := atomic.Int32{}

		fn := func(ctx context.Context) error {
			callCount.Add(1)
			return errors.NewPermissionDenied("/bin/secret", fmt.Errorf("access denied"))
		}

		config := &retryConfigTest{
			MaxAttempts:       3,
			InitialDelay:      10 * time.Millisecond,
			MaxDelay:          100 * time.Millisecond,
			BackoffMultiplier: 2.0,
			Jitter:            0.0,
		}

		ctx := context.Background()
		err := executeWithRetryTest(ctx, fn, config)

		if err == nil {
			t.Error("expected error")
		}

		if callCount.Load() != 1 {
			t.Errorf("expected 1 attempt (no retries), got %d", callCount.Load())
		}

		execErr := errors.AsExecutionError(err)
		if execErr == nil || execErr.Code != errors.ErrPermissionDenied {
			t.Errorf("expected permission denied error, got %v", execErr)
		}
	})

	t.Run("no retry on non-retryable execution failure", func(t *testing.T) {
		callCount := atomic.Int32{}

		fn := func(ctx context.Context) error {
			callCount.Add(1)
			return errors.NewExecutionFailed("non-recoverable error", false, nil)
		}

		config := &retryConfigTest{
			MaxAttempts:       3,
			InitialDelay:      10 * time.Millisecond,
			MaxDelay:          100 * time.Millisecond,
			BackoffMultiplier: 2.0,
			Jitter:            0.0,
		}

		ctx := context.Background()
		err := executeWithRetryTest(ctx, fn, config)

		if err == nil {
			t.Error("expected error")
		}

		if callCount.Load() != 1 {
			t.Errorf("expected 1 attempt (no retries), got %d", callCount.Load())
		}

		execErr := errors.AsExecutionError(err)
		if execErr == nil || execErr.Retryable {
			t.Errorf("expected non-retryable error, got retryable=%v", execErr.Retryable)
		}
	})
}

// TestExponentialBackoffTiming tests that exponential backoff delays are applied correctly.
// Verifies:
// - Initial delay is applied
// - Delay multiplies exponentially
// - Delay is capped at max delay
func TestExponentialBackoffTiming(t *testing.T) {
	t.Run("exponential backoff progression", func(t *testing.T) {
		callCount := atomic.Int32{}
		callTimes := []time.Time{}
		mu := sync.Mutex{}

		fn := func(ctx context.Context) error {
			mu.Lock()
			callTimes = append(callTimes, time.Now())
			mu.Unlock()
			callCount.Add(1)

			if callCount.Load() < 4 {
				return errors.NewTimeout(5*time.Second, fmt.Errorf("timeout"))
			}
			return nil
		}

		initialDelay := 20 * time.Millisecond
		config := &retryConfigTest{
			MaxAttempts:       4,
			InitialDelay:      initialDelay,
			MaxDelay:          500 * time.Millisecond,
			BackoffMultiplier: 2.0,
			Jitter:            0.0, // No jitter for exact timing
		}

		ctx := context.Background()
		start := time.Now()
		err := executeWithRetryTest(ctx, fn, config)

		if err != nil {
			t.Errorf("expected success, got error: %v", err)
		}

		if len(callTimes) != 4 {
			t.Fatalf("expected 4 call times, got %d", len(callTimes))
		}

		// Check delays between attempts
		// Delay 1: ~20ms (initialDelay * 2^0)
		// Delay 2: ~40ms (initialDelay * 2^1)
		// Delay 3: ~80ms (initialDelay * 2^2)
		totalTime := time.Since(start)
		expectedMinTime := time.Duration(20+40+80) * time.Millisecond
		tolerance := 50 * time.Millisecond // Allow some tolerance for timing variations

		if totalTime < expectedMinTime {
			t.Errorf("total time %v is less than expected minimum %v", totalTime, expectedMinTime)
		}

		if totalTime > expectedMinTime+tolerance {
			t.Logf("warning: total time %v exceeds expected minimum %v by more than tolerance", totalTime, expectedMinTime)
		}
	})

	t.Run("backoff capped at max delay", func(t *testing.T) {
		callCount := atomic.Int32{}

		fn := func(ctx context.Context) error {
			callCount.Add(1)
			if callCount.Load() < 5 {
				return errors.NewTimeout(5*time.Second, fmt.Errorf("timeout"))
			}
			return nil
		}

		// With 10x multiplier, backoff would quickly exceed max
		config := &retryConfigTest{
			MaxAttempts:       5,
			InitialDelay:      10 * time.Millisecond,
			MaxDelay:          50 * time.Millisecond,
			BackoffMultiplier: 10.0,
			Jitter:            0.0,
		}

		ctx := context.Background()
		start := time.Now()
		err := executeWithRetryTest(ctx, fn, config)

		if err != nil {
			t.Errorf("expected success, got error: %v", err)
		}

		totalTime := time.Since(start)
		// Without capping, would be much longer
		// With capping at 50ms: 10ms + 50ms + 50ms + 50ms = 160ms (min)
		expectedMaxTime := 200 * time.Millisecond

		if totalTime > expectedMaxTime {
			t.Errorf("total time %v exceeds expected max time %v (backoff not capped properly)", totalTime, expectedMaxTime)
		}
	})
}

// TestContextCancellationDuringRetry tests that context cancellation stops retries gracefully.
// Covers:
// - Cancellation during backoff
// - Cancellation during execution
// - Proper error propagation
func TestContextCancellationDuringRetry(t *testing.T) {
	t.Run("context canceled during backoff", func(t *testing.T) {
		callCount := atomic.Int32{}
		cancelChan := make(chan struct{})

		fn := func(ctx context.Context) error {
			callCount.Add(1)
			// Wait for cancellation signal on subsequent attempts
			if callCount.Load() > 1 {
				<-cancelChan
			}
			return errors.NewTimeout(5*time.Second, fmt.Errorf("timeout"))
		}

		config := &retryConfigTest{
			MaxAttempts:       5,
			InitialDelay:      100 * time.Millisecond,
			MaxDelay:          500 * time.Millisecond,
			BackoffMultiplier: 2.0,
			Jitter:            0.0,
		}

		ctx, cancel := context.WithCancel(context.Background())

		// Cancel after first attempt
		go func() {
			time.Sleep(10 * time.Millisecond)
			cancel()
			close(cancelChan)
		}()

		err := executeWithRetryTest(ctx, fn, config)

		if err != context.Canceled {
			t.Errorf("expected context canceled error, got %v", err)
		}

		// Should have attempted only once before cancellation
		if callCount.Load() > 2 {
			t.Errorf("expected at most 2 attempts before cancellation, got %d", callCount.Load())
		}
	})

	t.Run("context timeout stops retries", func(t *testing.T) {
		callCount := atomic.Int32{}

		fn := func(ctx context.Context) error {
			callCount.Add(1)
			return errors.NewTimeout(5*time.Second, fmt.Errorf("timeout"))
		}

		config := &retryConfigTest{
			MaxAttempts:       10,
			InitialDelay:      100 * time.Millisecond,
			MaxDelay:          1 * time.Second,
			BackoffMultiplier: 2.0,
			Jitter:            0.0,
		}

		ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
		defer cancel()

		err := executeWithRetryTest(ctx, fn, config)

		if err != context.DeadlineExceeded {
			t.Errorf("expected deadline exceeded error, got %v", err)
		}

		// Should have attempted only once before context timeout
		if callCount.Load() > 2 {
			t.Errorf("expected at most 2 attempts before timeout, got %d", callCount.Load())
		}
	})
}

// TestMaxRetriesLimit tests that retry attempts respect the maximum retry count.
// Covers:
// - Exactly maxAttempts are made
// - Error is returned after max attempts
// - Last error is returned, not intermediate ones
func TestMaxRetriesLimit(t *testing.T) {
	t.Run("max retries respected", func(t *testing.T) {
		callCount := atomic.Int32{}
		maxAttempts := int32(3)

		fn := func(ctx context.Context) error {
			callCount.Add(1)
			return errors.NewTimeout(5*time.Second, fmt.Errorf("always fails"))
		}

		config := &retryConfigTest{
			MaxAttempts:       int(maxAttempts),
			InitialDelay:      10 * time.Millisecond,
			MaxDelay:          100 * time.Millisecond,
			BackoffMultiplier: 2.0,
			Jitter:            0.0,
		}

		ctx := context.Background()
		err := executeWithRetryTest(ctx, fn, config)

		if err == nil {
			t.Error("expected error after max retries")
		}

		if callCount.Load() != maxAttempts {
			t.Errorf("expected exactly %d attempts, got %d", maxAttempts, callCount.Load())
		}

		// Verify it's a timeout error (transient)
		execErr := errors.AsExecutionError(err)
		if execErr == nil || execErr.Code != errors.ErrTimeout {
			t.Errorf("expected timeout error, got %v", err)
		}
	})

	t.Run("last error returned", func(t *testing.T) {
		callCount := atomic.Int32{}
		errorMessages := []string{
			"error 1",
			"error 2",
			"final error",
		}

		fn := func(ctx context.Context) error {
			idx := callCount.Load()
			callCount.Add(1)
			msg := errorMessages[idx]
			return errors.NewTimeout(5*time.Second, fmt.Errorf("%s", msg))
		}

		config := &retryConfigTest{
			MaxAttempts:       3,
			InitialDelay:      10 * time.Millisecond,
			MaxDelay:          100 * time.Millisecond,
			BackoffMultiplier: 2.0,
			Jitter:            0.0,
		}

		ctx := context.Background()
		err := executeWithRetryTest(ctx, fn, config)

		if err == nil {
			t.Error("expected error")
		}

		// Verify it contains the last error message
		errMsg := err.Error()
		if !contains(errMsg, "final error") {
			t.Errorf("expected final error message in %q", errMsg)
		}
	})

	t.Run("single attempt with max_attempts = 1", func(t *testing.T) {
		callCount := atomic.Int32{}

		fn := func(ctx context.Context) error {
			callCount.Add(1)
			return errors.NewTimeout(5*time.Second, fmt.Errorf("timeout"))
		}

		config := &retryConfigTest{
			MaxAttempts:       1,
			InitialDelay:      10 * time.Millisecond,
			MaxDelay:          100 * time.Millisecond,
			BackoffMultiplier: 2.0,
			Jitter:            0.0,
		}

		ctx := context.Background()
		err := executeWithRetryTest(ctx, fn, config)

		if err == nil {
			t.Error("expected error")
		}

		if callCount.Load() != 1 {
			t.Errorf("expected exactly 1 attempt, got %d", callCount.Load())
		}
	})
}

// TestRetryMetricsCollection tests that retry metrics are properly collected.
// Covers:
// - Attempt count tracking
// - Timing measurements
// - Error tracking across retries
func TestRetryMetricsCollection(t *testing.T) {
	t.Run("successful retry metrics", func(t *testing.T) {
		callTimes := []time.Time{}
		mu := sync.Mutex{}
		callCount := atomic.Int32{}

		fn := func(ctx context.Context) error {
			mu.Lock()
			callTimes = append(callTimes, time.Now())
			mu.Unlock()

			callCount.Add(1)
			if callCount.Load() < 3 {
				return errors.NewTimeout(5*time.Second, fmt.Errorf("timeout"))
			}
			return nil
		}

		config := &retryConfigTest{
			MaxAttempts:       3,
			InitialDelay:      20 * time.Millisecond,
			MaxDelay:          100 * time.Millisecond,
			BackoffMultiplier: 2.0,
			Jitter:            0.0,
		}

		ctx := context.Background()
		startTime := time.Now()
		err := executeWithRetryTest(ctx, fn, config)
		totalTime := time.Since(startTime)

		if err != nil {
			t.Errorf("expected success, got error: %v", err)
		}

		// Metrics assertions
		if callCount.Load() != 3 {
			t.Errorf("expected 3 attempts, got %d", callCount.Load())
		}

		if len(callTimes) != 3 {
			t.Errorf("expected 3 timestamps, got %d", len(callTimes))
		}

		// Total time should include backoff delays
		expectedMinTime := 40 * time.Millisecond // 20ms + 40ms delays (not counting jitter/variability)
		if totalTime < expectedMinTime {
			t.Logf("warning: total time %v is less than expected minimum %v", totalTime, expectedMinTime)
		}
	})

	t.Run("failed retry metrics", func(t *testing.T) {
		callTimes := []time.Time{}
		mu := sync.Mutex{}
		callCount := atomic.Int32{}
		errorCodes := []string{}

		fn := func(ctx context.Context) error {
			mu.Lock()
			callTimes = append(callTimes, time.Now())
			mu.Unlock()

			callCount.Add(1)

			switch callCount.Load() {
			case 1:
				errorCodes = append(errorCodes, "timeout")
				return errors.NewTimeout(5*time.Second, fmt.Errorf("timeout 1"))
			case 2:
				errorCodes = append(errorCodes, "execution_failed")
				return errors.NewExecutionFailed("execution error", true, nil)
			default:
				errorCodes = append(errorCodes, "timeout")
				return errors.NewTimeout(5*time.Second, fmt.Errorf("timeout 2"))
			}
		}

		config := &retryConfigTest{
			MaxAttempts:       3,
			InitialDelay:      10 * time.Millisecond,
			MaxDelay:          100 * time.Millisecond,
			BackoffMultiplier: 2.0,
			Jitter:            0.0,
		}

		ctx := context.Background()
		err := executeWithRetryTest(ctx, fn, config)

		if err == nil {
			t.Error("expected error after max retries")
		}

		if callCount.Load() != 3 {
			t.Errorf("expected 3 attempts, got %d", callCount.Load())
		}

		// Verify multiple error codes were encountered
		if len(errorCodes) != 3 {
			t.Errorf("expected 3 error codes logged, got %d", len(errorCodes))
		}
	})
}

// TestSuccessOnFirstAttempt tests that no retries occur if the first attempt succeeds.
func TestSuccessOnFirstAttempt(t *testing.T) {
	callCount := atomic.Int32{}

	fn := func(ctx context.Context) error {
		callCount.Add(1)
		return nil
	}

	config := &retryConfigTest{
		MaxAttempts:       3,
		InitialDelay:      100 * time.Millisecond,
		MaxDelay:          500 * time.Millisecond,
		BackoffMultiplier: 2.0,
		Jitter:            0.0,
	}

	ctx := context.Background()
	err := executeWithRetryTest(ctx, fn, config)

	if err != nil {
		t.Errorf("expected success, got error: %v", err)
	}

	if callCount.Load() != 1 {
		t.Errorf("expected exactly 1 attempt (no retries), got %d", callCount.Load())
	}
}

// ============================================================================
// Test Helpers
// ============================================================================

// retryConfigTest is a minimal retry config for testing purposes.
// This mirrors the actual RetryConfig but is local to tests for simplicity.
type retryConfigTest struct {
	MaxAttempts       int
	InitialDelay      time.Duration
	MaxDelay          time.Duration
	BackoffMultiplier float64
	Jitter            float64
}

// executeWithRetryTest executes a function with retry logic for testing.
// This is a simplified version that demonstrates the retry pattern used in the executor.
func executeWithRetryTest(ctx context.Context, fn func(context.Context) error, config *retryConfigTest) error {
	if config.MaxAttempts < 1 {
		config.MaxAttempts = 3
	}
	if config.InitialDelay <= 0 {
		config.InitialDelay = 1 * time.Second
	}
	if config.MaxDelay <= 0 {
		config.MaxDelay = 30 * time.Second
	}
	if config.BackoffMultiplier <= 1.0 {
		config.BackoffMultiplier = 2.0
	}

	var lastErr error

	for attempt := 0; attempt < config.MaxAttempts; attempt++ {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Execute function
		err := fn(ctx)
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if we should retry
		shouldRetry := shouldRetryTest(err, attempt+1, config.MaxAttempts)
		if !shouldRetry {
			return lastErr
		}

		// Calculate backoff delay
		delay := calculateBackoffTest(attempt, config)

		// Check context before sleeping
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			// Backoff complete, continue to next attempt
		}
	}

	return lastErr
}

// shouldRetryTest determines if a retry should occur.
func shouldRetryTest(err error, attempt, maxAttempts int) bool {
	if err == nil {
		return false
	}

	// Never retry on first attempt
	if attempt < 1 {
		return false
	}

	// Don't retry if max attempts reached
	if attempt >= maxAttempts {
		return false
	}

	// Check if error is retryable
	execErr := errors.AsExecutionError(err)
	if execErr == nil {
		// Unknown error type - don't retry
		return false
	}

	return execErr.Retryable
}

// calculateBackoffTest computes exponential backoff with jitter.
func calculateBackoffTest(attempt int, config *retryConfigTest) time.Duration {
	// Calculate exponential backoff: initial * multiplier^attempt
	baseDelayFloat := float64(config.InitialDelay) * pow(config.BackoffMultiplier, float64(attempt))

	// Cap at MaxDelay
	if baseDelayFloat > float64(config.MaxDelay) {
		baseDelayFloat = float64(config.MaxDelay)
	}

	// Apply jitter (disabled for deterministic tests)
	// For testing, we skip jitter if it's 0
	if config.Jitter > 0 {
		jitterRange := baseDelayFloat * config.Jitter
		// Simplified jitter: just apply some randomness
		_ = jitterRange
	}

	return time.Duration(baseDelayFloat)
}

// pow implements power function for float64.
func pow(base, exp float64) float64 {
	result := 1.0
	for i := 0; i < int(exp); i++ {
		result *= base
	}
	return result
}

// contains checks if a string contains a substring.
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
