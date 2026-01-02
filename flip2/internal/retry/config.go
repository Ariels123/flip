package retry

import (
	"errors"
	"math"
	"math/rand"
	"time"

	errpkg "flip2/internal/errors"
)

// RetryConfig defines retry behavior with exponential backoff and jitter.
//
// Retry Strategy:
// - Uses exponential backoff: delay = initial_delay * (multiplier ^ attempt)
// - Caps delay at max_delay to prevent unreasonable waits
// - Adds random jitter to prevent thundering herd
//
// Example backoff sequence with defaults (1s, 30s, 2x, 10% jitter):
//   Attempt 1: ~1.0s (1s +/- 10%)
//   Attempt 2: ~2.0s (2s +/- 10%)
//   Attempt 3: ~4.0s (4s +/- 10%)
//   Attempt 4: ~8.0s (8s +/- 10%)
//   Attempt 5: ~16.0s (16s +/- 10%)
//   Attempt 6: ~30.0s (capped at max, 30s +/- 10%)
type RetryConfig struct {
	// MaxAttempts is the maximum number of retry attempts.
	// Includes the initial attempt, so MaxAttempts=3 means 1 initial + 2 retries.
	// Must be >= 1. Default: 3
	MaxAttempts int

	// InitialDelay is the delay before the first retry.
	// Default: 1 second
	InitialDelay time.Duration

	// MaxDelay caps the delay between retries.
	// Prevents unreasonably long waits as backoff grows exponentially.
	// Default: 30 seconds
	MaxDelay time.Duration

	// BackoffMultiplier scales the delay for each retry attempt.
	// A multiplier of 2.0 means each delay is 2x the previous.
	// Must be > 1.0. Default: 2.0
	BackoffMultiplier float64

	// Jitter adds randomness to prevent synchronized retries (thundering herd).
	// Value is a percentage (0.0 to 1.0) applied to the calculated delay.
	// Jitter=0.1 means +/- 10% random variation.
	// Default: 0.1 (10%)
	Jitter float64
}

// DefaultConfig returns a sensible default retry configuration.
// - MaxAttempts: 3 (1 initial + 2 retries)
// - InitialDelay: 1 second
// - MaxDelay: 30 seconds
// - BackoffMultiplier: 2.0
// - Jitter: 0.1 (10%)
func DefaultConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:       3,
		InitialDelay:      1 * time.Second,
		MaxDelay:          30 * time.Second,
		BackoffMultiplier: 2.0,
		Jitter:            0.1,
	}
}

// AggressiveConfig returns a retry configuration optimized for fast recovery.
// Used for transient errors expected to resolve quickly.
// - MaxAttempts: 5 (1 initial + 4 retries)
// - InitialDelay: 100ms
// - MaxDelay: 10 seconds
// - BackoffMultiplier: 1.5
// - Jitter: 0.2 (20%)
func AggressiveConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:       5,
		InitialDelay:      100 * time.Millisecond,
		MaxDelay:          10 * time.Second,
		BackoffMultiplier: 1.5,
		Jitter:            0.2,
	}
}

// ConservativeConfig returns a retry configuration optimized for slow recovery.
// Used for errors that may take longer to resolve (e.g., quota resets).
// - MaxAttempts: 2 (1 initial + 1 retry)
// - InitialDelay: 5 seconds
// - MaxDelay: 5 minutes
// - BackoffMultiplier: 3.0
// - Jitter: 0.1 (10%)
func ConservativeConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:       2,
		InitialDelay:      5 * time.Second,
		MaxDelay:          5 * time.Minute,
		BackoffMultiplier: 3.0,
		Jitter:            0.1,
	}
}

// StrategyConfig maps error types to their retry strategies.
type StrategyConfig struct {
	// Timeout: Network/execution timeouts - retry with moderate backoff
	// Often caused by temporary load, expect quick recovery
	Timeout RetryConfig

	// QuotaExhausted: API rate limits or quota - conservative retry
	// Should wait for quota reset, typically longer recovery time
	QuotaExhausted RetryConfig

	// Transient: Other transient failures - aggressive retry
	// Temporary issues that may self-resolve
	Transient RetryConfig

	// Permanent: Non-retryable errors
	// Not included here as they don't retry
}

// DefaultStrategy returns the default retry strategy for all error types.
// Balances between fast recovery and resource usage.
func DefaultStrategy() StrategyConfig {
	return StrategyConfig{
		Timeout:        DefaultConfig(),
		QuotaExhausted: ConservativeConfig(),
		Transient:      AggressiveConfig(),
	}
}

// GetStrategyForError returns the appropriate retry strategy for a given error.
// Returns a sensible default for unknown error types.
func (s StrategyConfig) GetStrategyForError(err error) RetryConfig {
	if err == nil {
		return DefaultConfig()
	}

	// Check if it's an ExecutionError
	execErr := errpkg.AsExecutionError(err)
	if execErr == nil {
		// Unknown error type, use default
		return DefaultConfig()
	}

	// Route to appropriate strategy based on error code
	switch execErr.Code {
	case errpkg.ErrTimeout:
		return s.Timeout
	case errpkg.ErrQuotaExhausted:
		return s.QuotaExhausted
	case errpkg.ErrNotFound, errpkg.ErrPermissionDenied, errpkg.ErrInvalidConfig, errpkg.ErrCanceled:
		// Permanent errors don't get a strategy (shouldn't retry)
		return RetryConfig{MaxAttempts: 1} // No retries
	default:
		// ErrExecutionFailed and others - check retryable flag
		if execErr.Retryable {
			return s.Transient
		}
		return RetryConfig{MaxAttempts: 1} // No retries
	}
}

// ShouldRetry determines whether to retry based on error and attempt count.
// Returns (shouldRetry, nextDelay).
// nextDelay is the duration to wait before the next retry attempt.
// If shouldRetry is false, nextDelay is zero and should not be used.
//
// Retry logic:
// 1. If error is nil (success), never retry
// 2. Attempt 0 is the initial attempt - never retry for it
// 3. If max attempts exceeded, don't retry
// 4. If error is not retryable, don't retry
// 5. For quota errors with RetryAfter set, wait until that time
// 6. Otherwise use exponential backoff with jitter
func (c RetryConfig) ShouldRetry(err error, attempt int) (bool, time.Duration) {
	// If no error, operation succeeded - don't retry
	if err == nil {
		return false, 0
	}

	// Attempt numbering: 0 = first (initial) attempt, 1 = first retry, etc.
	// The initial attempt never retries - you only check ShouldRetry if it failed
	if attempt < 1 {
		return false, 0
	}

	// If we've made MaxAttempts attempts, don't retry
	if attempt >= c.MaxAttempts {
		return false, 0
	}

	// Check if error is retryable
	execErr := errpkg.AsExecutionError(err)
	if execErr != nil {
		// Handle quota errors with explicit RetryAfter time
		if execErr.Code == errpkg.ErrQuotaExhausted && !execErr.RetryAfter.IsZero() {
			waitTime := time.Until(execErr.RetryAfter)
			if waitTime > 0 {
				return true, waitTime
			}
		}

		// Check if error is retryable
		if !execErr.Retryable {
			return false, 0
		}
	} else if !errpkg.IsExecutionError(err) {
		// Unknown error type - don't retry to be safe
		return false, 0
	}

	// We should retry - calculate the delay
	// Subtract 1 from attempt because calculateBackoffDelay expects 0-indexed retry attempts
	delay := c.calculateBackoffDelay(attempt - 1)
	return true, delay
}

// calculateBackoffDelay computes exponential backoff with jitter.
// Attempt is 0-indexed (0 = first retry, 1 = second retry, etc.)
//
// Algorithm:
//   baseDelay = InitialDelay * (BackoffMultiplier ^ attempt)
//   cappedDelay = min(baseDelay, MaxDelay)
//   jitteredDelay = cappedDelay * (1 + jitterFactor)
//   jitterFactor is random in range [-Jitter, +Jitter]
func (c RetryConfig) calculateBackoffDelay(attempt int) time.Duration {
	// Validate config
	if c.InitialDelay <= 0 {
		c.InitialDelay = 1 * time.Second
	}
	if c.BackoffMultiplier <= 1.0 {
		c.BackoffMultiplier = 2.0
	}
	if c.MaxDelay <= 0 {
		c.MaxDelay = 30 * time.Second
	}
	if c.Jitter < 0 {
		c.Jitter = 0
	}
	if c.Jitter > 1 {
		c.Jitter = 1
	}

	// Calculate exponential backoff: initial * multiplier^attempt
	baseDelayFloat := float64(c.InitialDelay) * math.Pow(c.BackoffMultiplier, float64(attempt))

	// Cap at MaxDelay
	maxDelayFloat := float64(c.MaxDelay)
	if baseDelayFloat > maxDelayFloat {
		baseDelayFloat = maxDelayFloat
	}

	// Apply jitter: add random variation of +/- Jitter%
	jitterRange := baseDelayFloat * c.Jitter
	jitterFactor := (rand.Float64() * 2 * jitterRange) - jitterRange
	finalDelayFloat := baseDelayFloat + jitterFactor

	// Ensure delay is always positive
	if finalDelayFloat < 0 {
		finalDelayFloat = 0
	}

	return time.Duration(finalDelayFloat)
}

// Validate checks if the retry config is valid.
// Returns an error if any configuration value is invalid.
func (c RetryConfig) Validate() error {
	if c.MaxAttempts < 1 {
		return errors.New("MaxAttempts must be >= 1")
	}
	if c.InitialDelay <= 0 {
		return errors.New("InitialDelay must be > 0")
	}
	if c.MaxDelay <= 0 {
		return errors.New("MaxDelay must be > 0")
	}
	if c.BackoffMultiplier <= 1.0 {
		return errors.New("BackoffMultiplier must be > 1.0")
	}
	if c.Jitter < 0 || c.Jitter > 1 {
		return errors.New("Jitter must be between 0 and 1 (0% to 100%)")
	}
	return nil
}

// Note: Transient errors are identified through:
// 1. Specific error codes (ErrTimeout, ErrQuotaExhausted)
// 2. General execution failures with Retryable flag = true
// Future versions may add an explicit ErrTransient code for better type safety.
