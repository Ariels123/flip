package retry

import (
	"fmt"
	"testing"
	"time"

	errpkg "flip2/internal/errors"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.MaxAttempts != 3 {
		t.Errorf("expected MaxAttempts=3, got %d", config.MaxAttempts)
	}
	if config.InitialDelay != 1*time.Second {
		t.Errorf("expected InitialDelay=1s, got %v", config.InitialDelay)
	}
	if config.MaxDelay != 30*time.Second {
		t.Errorf("expected MaxDelay=30s, got %v", config.MaxDelay)
	}
	if config.BackoffMultiplier != 2.0 {
		t.Errorf("expected BackoffMultiplier=2.0, got %v", config.BackoffMultiplier)
	}
	if config.Jitter != 0.1 {
		t.Errorf("expected Jitter=0.1, got %v", config.Jitter)
	}

	if err := config.Validate(); err != nil {
		t.Errorf("default config should be valid: %v", err)
	}
}

func TestAggressiveConfig(t *testing.T) {
	config := AggressiveConfig()

	if config.MaxAttempts != 5 {
		t.Errorf("expected MaxAttempts=5, got %d", config.MaxAttempts)
	}
	if config.InitialDelay != 100*time.Millisecond {
		t.Errorf("expected InitialDelay=100ms, got %v", config.InitialDelay)
	}
	if config.MaxDelay != 10*time.Second {
		t.Errorf("expected MaxDelay=10s, got %v", config.MaxDelay)
	}

	if err := config.Validate(); err != nil {
		t.Errorf("aggressive config should be valid: %v", err)
	}
}

func TestConservativeConfig(t *testing.T) {
	config := ConservativeConfig()

	if config.MaxAttempts != 2 {
		t.Errorf("expected MaxAttempts=2, got %d", config.MaxAttempts)
	}
	if config.InitialDelay != 5*time.Second {
		t.Errorf("expected InitialDelay=5s, got %v", config.InitialDelay)
	}
	if config.MaxDelay != 5*time.Minute {
		t.Errorf("expected MaxDelay=5m, got %v", config.MaxDelay)
	}

	if err := config.Validate(); err != nil {
		t.Errorf("conservative config should be valid: %v", err)
	}
}

func TestDefaultStrategy(t *testing.T) {
	strategy := DefaultStrategy()

	if err := strategy.Timeout.Validate(); err != nil {
		t.Errorf("timeout strategy should be valid: %v", err)
	}
	if err := strategy.QuotaExhausted.Validate(); err != nil {
		t.Errorf("quota exhausted strategy should be valid: %v", err)
	}
	if err := strategy.Transient.Validate(); err != nil {
		t.Errorf("transient strategy should be valid: %v", err)
	}
}

func TestGetStrategyForError(t *testing.T) {
	strategy := DefaultStrategy()

	tests := []struct {
		name            string
		err             error
		expectedConfig  RetryConfig
		checkMaxAttempt bool
	}{
		{
			name:           "nil error uses default",
			err:            nil,
			expectedConfig: DefaultConfig(),
		},
		{
			name:           "timeout error uses timeout strategy",
			err:            errpkg.NewTimeout(5*time.Second, nil),
			expectedConfig: strategy.Timeout,
		},
		{
			name:           "quota exhausted uses conservative strategy",
			err:            errpkg.NewQuotaExhausted("rate limited", time.Now().Add(1*time.Hour)),
			expectedConfig: strategy.QuotaExhausted,
		},
		{
			name:           "retryable execution failed uses transient strategy",
			err:            errpkg.NewExecutionFailed("temporary issue", true, nil),
			expectedConfig: strategy.Transient,
		},
		{
			name:            "non-retryable error has no retries",
			err:             errpkg.NewNotFound("binary", nil),
			checkMaxAttempt: true,
		},
		{
			name:            "permission denied has no retries",
			err:             errpkg.NewPermissionDenied("/bin/claude", nil),
			checkMaxAttempt: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := strategy.GetStrategyForError(tt.err)

			if tt.checkMaxAttempt {
				if config.MaxAttempts != 1 {
					t.Errorf("expected no retries (MaxAttempts=1), got %d", config.MaxAttempts)
				}
			} else if config.MaxAttempts != tt.expectedConfig.MaxAttempts {
				t.Errorf("expected MaxAttempts=%d, got %d", tt.expectedConfig.MaxAttempts, config.MaxAttempts)
			}
		})
	}
}

func TestShouldRetry(t *testing.T) {
	config := DefaultConfig() // 3 attempts max, 1s initial, 2x multiplier

	tests := []struct {
		name        string
		err         error
		attempt     int
		shouldRetry bool
		description string
	}{
		{
			name:        "initial attempt (0) never retries",
			err:         nil,
			attempt:     0,
			shouldRetry: false,
			description: "attempt 0 is the initial attempt, no retry logic applied",
		},
		{
			name:        "retryable error on first retry (attempt 1)",
			err:         errpkg.NewTimeout(5*time.Second, nil),
			attempt:     1,
			shouldRetry: true,
			description: "timeout is retryable on first retry",
		},
		{
			name:        "retryable error on second retry (attempt 2)",
			err:         errpkg.NewTimeout(5*time.Second, nil),
			attempt:     2,
			shouldRetry: true,
			description: "still within max attempts",
		},
		{
			name:        "max attempts exceeded (attempt 3)",
			err:         errpkg.NewTimeout(5*time.Second, nil),
			attempt:     3,
			shouldRetry: false,
			description: "reached max attempts (attempts 0,1,2 = 3 total, so 3 is beyond)",
		},
		{
			name:        "non-retryable error",
			err:         errpkg.NewNotFound("binary", nil),
			attempt:     1,
			shouldRetry: false,
			description: "permanent errors don't retry",
		},
		{
			name:        "execution failed non-retryable",
			err:         errpkg.NewExecutionFailed("crashed", false, nil),
			attempt:     1,
			shouldRetry: false,
			description: "marked as non-retryable",
		},
		{
			name:        "execution failed retryable",
			err:         errpkg.NewExecutionFailed("temporary", true, nil),
			attempt:     1,
			shouldRetry: true,
			description: "marked as retryable",
		},
		{
			name:        "nil error (no error) on retry",
			err:         nil,
			attempt:     1,
			shouldRetry: false,
			description: "nil error means operation succeeded, don't retry",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shouldRetry, delay := config.ShouldRetry(tt.err, tt.attempt)
			if shouldRetry != tt.shouldRetry {
				t.Errorf("%s: expected shouldRetry=%v, got %v", tt.description, tt.shouldRetry, shouldRetry)
			}
			if !shouldRetry && delay != 0 {
				t.Errorf("%s: expected zero delay when not retrying, got %v", tt.description, delay)
			}
		})
	}
}

func TestQuotaExhaustedRetryAfter(t *testing.T) {
	config := DefaultConfig()

	// Quota error with RetryAfter set in the future
	retryTime := time.Now().Add(2 * time.Hour)
	quotaErr := errpkg.NewQuotaExhausted("rate limited", retryTime)

	shouldRetry, delay := config.ShouldRetry(quotaErr, 1)

	if !shouldRetry {
		t.Error("quota error should be retryable")
	}

	// The delay should be approximately 2 hours
	expectedMin := 2*time.Hour - 1*time.Minute // Allow some tolerance
	expectedMax := 2*time.Hour + 1*time.Minute

	if delay < expectedMin || delay > expectedMax {
		t.Errorf("expected delay ~2h, got %v", delay)
	}
}

func TestBackoffDelayProgression(t *testing.T) {
	config := RetryConfig{
		MaxAttempts:       10,
		InitialDelay:      1 * time.Second,
		MaxDelay:          30 * time.Second,
		BackoffMultiplier: 2.0,
		Jitter:            0.0, // No jitter for predictable testing
	}

	tests := []struct {
		retryAttempt int // 0-indexed retry number
		minDelay     time.Duration
		maxDelay     time.Duration
		description  string
	}{
		{0, 1 * time.Second, 1 * time.Second, "first retry: 1s"},
		{1, 2 * time.Second, 2 * time.Second, "second retry: 2s"},
		{2, 4 * time.Second, 4 * time.Second, "third retry: 4s"},
		{3, 8 * time.Second, 8 * time.Second, "fourth retry: 8s"},
		{4, 16 * time.Second, 16 * time.Second, "fifth retry: 16s"},
		{5, 30 * time.Second, 30 * time.Second, "sixth retry: capped at 30s"},
		{6, 30 * time.Second, 30 * time.Second, "seventh retry: capped at 30s"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("retry_attempt_%d", tt.retryAttempt), func(t *testing.T) {
			// Test calculateBackoffDelay directly (0-indexed retry attempts)
			delay := config.calculateBackoffDelay(tt.retryAttempt)

			if delay < tt.minDelay || delay > tt.maxDelay {
				t.Errorf("%s: expected delay %v-%v, got %v", tt.description, tt.minDelay, tt.maxDelay, delay)
			}
		})
	}
}

func TestBackoffWithJitter(t *testing.T) {
	config := RetryConfig{
		MaxAttempts:       3,
		InitialDelay:      10 * time.Second,
		MaxDelay:          30 * time.Second,
		BackoffMultiplier: 2.0,
		Jitter:            0.1, // 10% jitter
	}

	// Collect delays from multiple attempts to verify jitter variation
	// Use calculateBackoffDelay directly with retry attempt 0 (first retry)
	delays := make([]time.Duration, 0)
	for i := 0; i < 20; i++ {
		delay := config.calculateBackoffDelay(0) // First retry delay
		delays = append(delays, delay)
	}

	// All delays should be around 10s with +/- 1s jitter (10% of 10s)
	minExpected := 9 * time.Second
	maxExpected := 11 * time.Second

	for _, delay := range delays {
		if delay < minExpected || delay > maxExpected {
			t.Errorf("jitter failed: expected %v-%v, got %v", minExpected, maxExpected, delay)
		}
	}

	// Verify we actually got variation (not all the same)
	minDelay := delays[0]
	maxDelay := delays[0]
	for _, d := range delays {
		if d < minDelay {
			minDelay = d
		}
		if d > maxDelay {
			maxDelay = d
		}
	}

	if minDelay == maxDelay {
		t.Error("jitter should introduce variation in delays")
	}
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name      string
		config    RetryConfig
		shouldErr bool
		errMsg    string
	}{
		{
			name:      "valid default config",
			config:    DefaultConfig(),
			shouldErr: false,
		},
		{
			name: "invalid max attempts (0)",
			config: RetryConfig{
				MaxAttempts:       0,
				InitialDelay:      1 * time.Second,
				MaxDelay:          30 * time.Second,
				BackoffMultiplier: 2.0,
				Jitter:            0.1,
			},
			shouldErr: true,
			errMsg:    "MaxAttempts must be >= 1",
		},
		{
			name: "invalid initial delay (negative)",
			config: RetryConfig{
				MaxAttempts:       3,
				InitialDelay:      -1 * time.Second,
				MaxDelay:          30 * time.Second,
				BackoffMultiplier: 2.0,
				Jitter:            0.1,
			},
			shouldErr: true,
			errMsg:    "InitialDelay must be > 0",
		},
		{
			name: "invalid max delay (zero)",
			config: RetryConfig{
				MaxAttempts:       3,
				InitialDelay:      1 * time.Second,
				MaxDelay:          0,
				BackoffMultiplier: 2.0,
				Jitter:            0.1,
			},
			shouldErr: true,
			errMsg:    "MaxDelay must be > 0",
		},
		{
			name: "invalid backoff multiplier (1.0)",
			config: RetryConfig{
				MaxAttempts:       3,
				InitialDelay:      1 * time.Second,
				MaxDelay:          30 * time.Second,
				BackoffMultiplier: 1.0,
				Jitter:            0.1,
			},
			shouldErr: true,
			errMsg:    "BackoffMultiplier must be > 1.0",
		},
		{
			name: "invalid jitter (negative)",
			config: RetryConfig{
				MaxAttempts:       3,
				InitialDelay:      1 * time.Second,
				MaxDelay:          30 * time.Second,
				BackoffMultiplier: 2.0,
				Jitter:            -0.1,
			},
			shouldErr: true,
			errMsg:    "Jitter must be between 0 and 1 (0% to 100%)",
		},
		{
			name: "invalid jitter (> 1.0)",
			config: RetryConfig{
				MaxAttempts:       3,
				InitialDelay:      1 * time.Second,
				MaxDelay:          30 * time.Second,
				BackoffMultiplier: 2.0,
				Jitter:            1.5,
			},
			shouldErr: true,
			errMsg:    "Jitter must be between 0 and 1 (0% to 100%)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.shouldErr {
				t.Errorf("expected error=%v, got error=%v", tt.shouldErr, err != nil)
			}
			if err != nil && tt.errMsg != "" && err.Error() != tt.errMsg {
				t.Errorf("expected error msg %q, got %q", tt.errMsg, err.Error())
			}
		})
	}
}

// TestRetryMatrix demonstrates retry behavior across different error types.
// This serves as documentation for the retry system.
func TestRetryMatrix(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping detailed retry matrix in short mode")
	}

	strategy := DefaultStrategy()

	t.Run("timeout error retry sequence", func(t *testing.T) {
		timeoutErr := errpkg.NewTimeout(30*time.Second, nil)
		config := strategy.GetStrategyForError(timeoutErr)

		t.Logf("Timeout Strategy: MaxAttempts=%d, InitialDelay=%v, BackoffMultiplier=%v",
			config.MaxAttempts, config.InitialDelay, config.BackoffMultiplier)

		// Attempt 0 is initial, should never retry
		should, delay := config.ShouldRetry(timeoutErr, 0)
		t.Logf("  Attempt 0 (initial): shouldRetry=%v, delay=%v", should, delay)

		// Attempts 1-2 should retry with increasing delays
		for attempt := 1; attempt < 3; attempt++ {
			should, delay := config.ShouldRetry(timeoutErr, attempt)
			t.Logf("  Attempt %d: shouldRetry=%v, delay=%v", attempt, should, delay)
		}
	})

	t.Run("quota exhausted retry sequence", func(t *testing.T) {
		resetTime := time.Now().Add(1 * time.Hour)
		quotaErr := errpkg.NewQuotaExhausted("rate limit", resetTime)
		config := strategy.GetStrategyForError(quotaErr)

		t.Logf("Quota Exhausted Strategy: MaxAttempts=%d, InitialDelay=%v, MaxDelay=%v",
			config.MaxAttempts, config.InitialDelay, config.MaxDelay)

		should, delay := config.ShouldRetry(quotaErr, 1)
		t.Logf("  Attempt 1: shouldRetry=%v, delay=%v (should be ~1h)", should, delay)
	})

	t.Run("transient error retry sequence", func(t *testing.T) {
		transientErr := errpkg.NewExecutionFailed("temporary", true, nil)
		config := strategy.GetStrategyForError(transientErr)

		t.Logf("Transient Strategy: MaxAttempts=%d, InitialDelay=%v, BackoffMultiplier=%v",
			config.MaxAttempts, config.InitialDelay, config.BackoffMultiplier)

		for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
			should, delay := config.ShouldRetry(transientErr, attempt)
			t.Logf("  Attempt %d: shouldRetry=%v, delay=%v", attempt, should, delay)
		}
	})

	t.Run("permanent error (no retries)", func(t *testing.T) {
		permanentErr := errpkg.NewNotFound("claude", nil)
		config := strategy.GetStrategyForError(permanentErr)

		t.Logf("Permanent Error Strategy: MaxAttempts=%d (no retries)", config.MaxAttempts)

		should, delay := config.ShouldRetry(permanentErr, 1)
		t.Logf("  Attempt 1: shouldRetry=%v, delay=%v", should, delay)
	})
}
