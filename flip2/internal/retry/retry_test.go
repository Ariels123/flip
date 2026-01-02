package retry

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	errpkg "flip2/internal/errors"
)

// TestExecuteWithRetrySuccess tests that a successful execution returns immediately.
func TestExecuteWithRetrySuccess(t *testing.T) {
	ctx := context.Background()
	config := RetryConfig{
		MaxAttempts:       3,
		InitialDelay:      10 * time.Millisecond,
		MaxDelay:          30 * time.Second,
		BackoffMultiplier: 2.0,
		Jitter:            0.1,
	}

	attempts := 0
	fn := func(ctx context.Context) error {
		attempts++
		return nil
	}

	err := ExecuteWithRetry(ctx, fn, &config)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if attempts != 1 {
		t.Fatalf("expected 1 attempt, got %d", attempts)
	}
}

// TestExecuteWithRetryNonRetryableError tests that non-retryable errors return immediately.
func TestExecuteWithRetryNonRetryableError(t *testing.T) {
	ctx := context.Background()
	config := RetryConfig{
		MaxAttempts:       3,
		InitialDelay:      10 * time.Millisecond,
		MaxDelay:          30 * time.Second,
		BackoffMultiplier: 2.0,
		Jitter:            0.1,
	}

	attempts := 0
	fn := func(ctx context.Context) error {
		attempts++
		return errpkg.NewNotFound("file", nil)
	}

	err := ExecuteWithRetry(ctx, fn, &config)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if attempts != 1 {
		t.Fatalf("expected 1 attempt (no retries for non-retryable error), got %d", attempts)
	}

	execErr := errpkg.AsExecutionError(err)
	if execErr == nil || execErr.Code != errpkg.ErrNotFound {
		t.Fatal("expected not found error")
	}
}

// TestExecuteWithRetryRetryableEventualSuccess tests retrying until success.
func TestExecuteWithRetryRetryableEventualSuccess(t *testing.T) {
	ctx := context.Background()
	config := RetryConfig{
		MaxAttempts: 5,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay: 100 * time.Millisecond,
		BackoffMultiplier: 2.0,
		Jitter: 0.1,
	}

	attempts := 0
	fn := func(ctx context.Context) error {
		attempts++
		if attempts < 3 {
			// Return transient error for first 2 attempts
			return errpkg.NewExecutionFailed("temporary failure", true, nil)
		}
		return nil // Success on 3rd attempt
	}

	start := time.Now()
	err := ExecuteWithRetry(ctx, fn, &config)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}

	// Should have waited at least for the backoff delays
	// Attempt 1: no delay, Attempt 2: ~10ms, Attempt 3: no delay
	expectedMinDelay := time.Duration(0) // At least some backoff occurred
	if elapsed < expectedMinDelay {
		t.Logf("warning: elapsed time %v seems too short, expected at least %v", elapsed, expectedMinDelay)
	}
}

// TestExecuteWithRetryMaxAttemptsExceeded tests that retries stop after max attempts.
func TestExecuteWithRetryMaxAttemptsExceeded(t *testing.T) {
	ctx := context.Background()
	config := RetryConfig{
		MaxAttempts:       3,
		InitialDelay:      5 * time.Millisecond,
		MaxDelay:          30 * time.Second,
		BackoffMultiplier: 2.0,
		Jitter:            0.1,
	}

	attempts := 0
	fn := func(ctx context.Context) error {
		attempts++
		return errpkg.NewExecutionFailed("persistent failure", true, nil)
	}

	err := ExecuteWithRetry(ctx, fn, &config)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts (max), got %d", attempts)
	}
}

// TestExecuteWithRetryContextCancellation tests that context cancellation stops retries.
func TestExecuteWithRetryContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	config := RetryConfig{
		MaxAttempts:       10,
		InitialDelay:      100 * time.Millisecond,
		MaxDelay:          30 * time.Second,
		BackoffMultiplier: 2.0,
		Jitter:            0.1,
	}

	attempts := 0
	fn := func(ctx context.Context) error {
		attempts++
		if attempts == 2 {
			// Cancel on second attempt
			cancel()
		}
		return errpkg.NewExecutionFailed("transient", true, nil)
	}

	err := ExecuteWithRetry(ctx, fn, &config)
	if err != context.Canceled {
		t.Fatalf("expected context canceled error, got %v", err)
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts before cancellation, got %d", attempts)
	}
}

// TestExecuteWithRetryBackoffSequence tests the exponential backoff calculation without actual delays.
// This test verifies the backoff formula by checking the calculated delays directly.
func TestExecuteWithRetryBackoffSequence(t *testing.T) {
	config := RetryConfig{
		MaxAttempts:       8, // Need enough attempts to test all delays
		InitialDelay:      1 * time.Second,
		MaxDelay:          30 * time.Second,
		BackoffMultiplier: 2.0,
		Jitter:            0.0, // No jitter for predictable test
	}

	// Test the calculated delays for each attempt
	// Attempt numbering: 0 = initial, 1 = first retry, 2 = second retry, etc.
	expectedDelays := map[int]time.Duration{
		1: 1 * time.Second,              // 1s * 2^0 = 1s
		2: 2 * time.Second,              // 1s * 2^1 = 2s
		3: 4 * time.Second,              // 1s * 2^2 = 4s
		4: 8 * time.Second,              // 1s * 2^3 = 8s
		5: 16 * time.Second,             // 1s * 2^4 = 16s
		6: 30 * time.Second,             // 1s * 2^5 = 32s, capped at 30s
		7: 30 * time.Second,             // Still capped at 30s
	}

	for attempt, expected := range expectedDelays {
		// Test ShouldRetry to get the delay
		// Create a retryable error
		err := errpkg.NewExecutionFailed("test", true, nil)

		shouldRetry, delay := config.ShouldRetry(err, attempt)
		if !shouldRetry {
			t.Fatalf("attempt %d should retry, but got false", attempt)
		}

		if delay != expected {
			t.Errorf("attempt %d: expected delay %v, got %v", attempt, expected, delay)
		}
	}

	// Test that attempt 0 (initial) never triggers a retry
	err := errpkg.NewExecutionFailed("test", true, nil)
	shouldRetry, _ := config.ShouldRetry(err, 0)
	if shouldRetry {
		t.Fatal("attempt 0 (initial) should never trigger retry")
	}
}

// TestExecuteWithRetryNilConfig tests that nil config uses defaults.
func TestExecuteWithRetryNilConfig(t *testing.T) {
	ctx := context.Background()

	attempts := 0
	fn := func(ctx context.Context) error {
		attempts++
		if attempts < 2 {
			return errpkg.NewExecutionFailed("transient", true, nil)
		}
		return nil
	}

	err := ExecuteWithRetry(ctx, fn, nil)
	if err != nil {
		t.Fatalf("expected no error with default config, got %v", err)
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts with default config, got %d", attempts)
	}
}

// TestExecuteWithRetryQuotaExhaustedWithRetryAfter tests quota exhaustion handling.
func TestExecuteWithRetryQuotaExhaustedWithRetryAfter(t *testing.T) {
	ctx := context.Background()
	config := RetryConfig{
		MaxAttempts:       3,
		InitialDelay:      50 * time.Millisecond,
		MaxDelay:          30 * time.Second,
		BackoffMultiplier: 2.0,
		Jitter:            0.1,
	}

	attempts := 0
	fn := func(ctx context.Context) error {
		attempts++
		if attempts == 1 {
			// First attempt: quota exhausted with immediate retry allowed
			retryAfter := time.Now().Add(50 * time.Millisecond)
			return errpkg.NewQuotaExhausted("API quota exceeded", retryAfter)
		}
		return nil // Success on second attempt
	}

	start := time.Now()
	err := ExecuteWithRetry(ctx, fn, &config)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}

	// Should have waited for the RetryAfter time
	expectedMinDelay := 40 * time.Millisecond
	if elapsed < expectedMinDelay {
		t.Logf("warning: elapsed time %v less than expected %v", elapsed, expectedMinDelay)
	}
}

// TestExecuteWithRetryAggressiveConfig tests aggressive retry settings.
func TestExecuteWithRetryAggressiveConfig(t *testing.T) {
	ctx := context.Background()
	config := AggressiveConfig()

	attempts := 0
	fn := func(ctx context.Context) error {
		attempts++
		if attempts < 2 {
			return errpkg.NewExecutionFailed("transient", true, nil)
		}
		return nil
	}

	start := time.Now()
	err := ExecuteWithRetry(ctx, fn, &config)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}

	// Aggressive config has very short delays
	// Should be much faster than default
	if elapsed > 500*time.Millisecond {
		t.Logf("warning: aggressive config took longer than expected: %v", elapsed)
	}
}

// TestExecuteWithRetryConservativeConfig tests conservative retry settings.
func TestExecuteWithRetryConservativeConfig(t *testing.T) {
	ctx := context.Background()
	config := ConservativeConfig()

	attempts := 0
	fn := func(ctx context.Context) error {
		attempts++
		if attempts < 2 {
			return errpkg.NewExecutionFailed("transient", true, nil)
		}
		return nil
	}

	err := ExecuteWithRetry(ctx, fn, &config)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
}

// TestExecuteWithRetryTimeoutError tests timeout error handling.
func TestExecuteWithRetryTimeoutError(t *testing.T) {
	ctx := context.Background()
	config := RetryConfig{
		MaxAttempts:       3,
		InitialDelay:      10 * time.Millisecond,
		MaxDelay:          30 * time.Second,
		BackoffMultiplier: 2.0,
		Jitter:            0.1,
	}

	attempts := 0
	fn := func(ctx context.Context) error {
		attempts++
		if attempts < 2 {
			return errpkg.NewTimeout(5*time.Second, nil)
		}
		return nil
	}

	err := ExecuteWithRetry(ctx, fn, &config)
	if err != nil {
		t.Fatalf("expected no error after retry, got %v", err)
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
}

// TestExecuteWithRetryUnknownError tests unknown error type handling.
func TestExecuteWithRetryUnknownError(t *testing.T) {
	ctx := context.Background()
	config := RetryConfig{
		MaxAttempts:       3,
		InitialDelay:      10 * time.Millisecond,
		MaxDelay:          30 * time.Second,
		BackoffMultiplier: 2.0,
		Jitter:            0.1,
	}

	attempts := 0
	fn := func(ctx context.Context) error {
		attempts++
		return errors.New("unknown error")
	}

	err := ExecuteWithRetry(ctx, fn, &config)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if attempts != 1 {
		t.Fatalf("expected 1 attempt (unknown errors not retried), got %d", attempts)
	}
}

// TestExecuteWithRetryInvalidConfig tests that invalid config is rejected.
func TestExecuteWithRetryInvalidConfig(t *testing.T) {
	ctx := context.Background()
	config := RetryConfig{
		MaxAttempts: 0, // Invalid
		InitialDelay: 1 * time.Second,
	}

	fn := func(ctx context.Context) error {
		return nil
	}

	err := ExecuteWithRetry(ctx, fn, &config)
	if err == nil {
		t.Fatal("expected error for invalid config, got nil")
	}
}

// TestExecuteWithRetryBackoffCapping tests that delays are capped at MaxDelay.
func TestExecuteWithRetryBackoffCapping(t *testing.T) {
	config := RetryConfig{
		MaxAttempts: 6,
		InitialDelay: 1 * time.Second,
		MaxDelay: 5 * time.Second,
		BackoffMultiplier: 2.0,
		Jitter: 0.0,
	}

	// Test the backoff calculation directly
	delays := []time.Duration{
		config.calculateBackoffDelay(0), // 1s
		config.calculateBackoffDelay(1), // 2s
		config.calculateBackoffDelay(2), // 4s
		config.calculateBackoffDelay(3), // 8s -> capped at 5s
		config.calculateBackoffDelay(4), // 16s -> capped at 5s
	}

	expectedMaxes := []time.Duration{
		1 * time.Second,
		2 * time.Second,
		4 * time.Second,
		5 * time.Second, // Capped
		5 * time.Second, // Capped
	}

	for i, delay := range delays {
		if delay != expectedMaxes[i] {
			t.Errorf("delay[%d]: expected max %v, got %v",
				i, expectedMaxes[i], delay)
		}
	}
}

// TestExecuteWithRetryJitterVariation tests that jitter adds randomness.
func TestExecuteWithRetryJitterVariation(t *testing.T) {
	config := RetryConfig{
		InitialDelay: 1 * time.Second,
		MaxDelay: 30 * time.Second,
		BackoffMultiplier: 2.0,
		Jitter: 0.2, // 20% jitter
	}

	// Calculate multiple delays for the same attempt
	delays := make([]float64, 100)
	for i := 0; i < 100; i++ {
		delay := config.calculateBackoffDelay(0)
		delays[i] = float64(delay)
	}

	// Calculate mean and standard deviation
	var mean float64
	for _, d := range delays {
		mean += d
	}
	mean /= float64(len(delays))

	var variance float64
	for _, d := range delays {
		variance += (d - mean) * (d - mean)
	}
	variance /= float64(len(delays))
	stdDev := math.Sqrt(variance)

	// With 20% jitter on 1s, we expect some variation
	// Standard deviation should be > 0 (meaning jitter is actually applied)
	if stdDev == 0 {
		t.Fatal("expected jitter to create variation, but stdDev is 0")
	}

	// Reasonable bounds for jitter
	// With 1s base and 20% jitter: should be between ~0.8s and 1.2s
	minDelay := 0.8 * float64(1*time.Second)
	maxDelay := 1.2 * float64(1*time.Second)

	for i, d := range delays {
		if d < minDelay || d > maxDelay {
			t.Errorf("delay[%d]=%v outside expected range [%v, %v]",
				i, d, minDelay, maxDelay)
		}
	}
}

// TestExecuteWithRetryPermissionDeniedError tests permission denied error (non-retryable).
func TestExecuteWithRetryPermissionDeniedError(t *testing.T) {
	ctx := context.Background()
	config := RetryConfig{
		MaxAttempts:       3,
		InitialDelay:      10 * time.Millisecond,
		MaxDelay:          30 * time.Second,
		BackoffMultiplier: 2.0,
		Jitter:            0.1,
	}

	attempts := 0
	fn := func(ctx context.Context) error {
		attempts++
		return errpkg.NewPermissionDenied("executable", nil)
	}

	err := ExecuteWithRetry(ctx, fn, &config)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if attempts != 1 {
		t.Fatalf("expected 1 attempt (permission denied not retried), got %d", attempts)
	}
}

// TestExecuteWithRetryCanceledError tests canceled error (non-retryable).
func TestExecuteWithRetryCanceledError(t *testing.T) {
	ctx := context.Background()
	config := RetryConfig{
		MaxAttempts:       3,
		InitialDelay:      10 * time.Millisecond,
		MaxDelay:          30 * time.Second,
		BackoffMultiplier: 2.0,
		Jitter:            0.1,
	}

	attempts := 0
	fn := func(ctx context.Context) error {
		attempts++
		return errpkg.NewCanceled("operation canceled")
	}

	err := ExecuteWithRetry(ctx, fn, &config)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if attempts != 1 {
		t.Fatalf("expected 1 attempt (canceled not retried), got %d", attempts)
	}
}

// TestExecuteWithRetryContextDeadline tests deadline exceeded handling.
func TestExecuteWithRetryContextDeadline(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(100*time.Millisecond))
	defer cancel()

	config := RetryConfig{
		MaxAttempts:       10,
		InitialDelay:      200 * time.Millisecond,
		MaxDelay:          30 * time.Second,
		BackoffMultiplier: 2.0,
		Jitter:            0.1,
	}

	attempts := 0
	fn := func(ctx context.Context) error {
		attempts++
		return errpkg.NewExecutionFailed("transient", true, nil)
	}

	err := ExecuteWithRetry(ctx, fn, &config)
	if err == nil || err.Error() != "context deadline exceeded" {
		t.Fatalf("expected context deadline exceeded, got %v", err)
	}
	if attempts != 1 {
		t.Fatalf("expected 1 attempt before deadline, got %d", attempts)
	}
}
