package retry

import (
	"context"
	"log/slog"
	"time"

	errpkg "flip2/internal/errors"
	"flip2/internal/logger"
)

// ExecuteWithRetry executes a function with exponential backoff retry logic.
//
// The function will be executed up to config.MaxAttempts times, with exponential backoff
// delays between retries. If the function returns nil (success), execution stops immediately.
// If it returns a non-retryable error, execution stops immediately. Otherwise, the function
// is retried up to the configured number of attempts.
//
// Backoff calculation:
//   - delay = InitialDelay * (BackoffMultiplier ^ attemptNumber)
//   - delay is capped at MaxDelay
//   - random jitter is applied to prevent thundering herd
//
// Context cancellation:
//   - If the context is canceled, the function waits for the current execution to complete,
//     but does not perform any retries. Returns the context error.
//
// Structured logging:
//   - All retry attempts are logged with structured fields (attempt, error, delay)
//   - Context fields (task_id, agent_id, etc.) are automatically included
//
// Parameters:
//   - ctx: context for cancellation and log field extraction
//   - fn: function to execute (called with the same context)
//   - config: retry configuration (will use defaults if nil)
//
// Returns:
//   - nil if the function succeeds
//   - context error if context is canceled
//   - the last error from fn if all retries are exhausted
//   - any non-retryable error from fn immediately
func ExecuteWithRetry(ctx context.Context, fn func(context.Context) error, config *RetryConfig) error {
	// Validate config
	if config == nil {
		defaultCfg := DefaultConfig()
		config = &defaultCfg
	}

	if err := config.Validate(); err != nil {
		slog.Error("invalid retry config", "error", err)
		return err
	}

	// Extract log fields from context
	logFields := logger.ExtractLogFields(ctx)
	logArgs := flattenLogFields(logFields)

	var lastErr error

	// Attempt loop - attempt number is 0 for initial, 1 for first retry, etc.
	for attempt := 0; attempt < config.MaxAttempts; attempt++ {
		// Check for context cancellation before each attempt
		select {
		case <-ctx.Done():
			slog.Warn("execution canceled during retry loop",
				append(logArgs, "attempt", attempt, "error", ctx.Err())...)
			return ctx.Err()
		default:
		}

		// Log the attempt (1-indexed for user-friendly output)
		slog.Debug("executing function", append(logArgs, "attempt", attempt+1, "max_attempts", config.MaxAttempts)...)

		// Execute the function
		err := fn(ctx)
		if err == nil {
			// Success
			slog.Debug("execution succeeded", append(logArgs, "attempt", attempt+1)...)
			return nil
		}

		lastErr = err

		// Check if we should retry - pass the attempt number we just completed
		// ShouldRetry expects: 0 = initial (never retry), 1 = first retry attempt, etc.
		shouldRetry, nextDelay := config.ShouldRetry(err, attempt+1)
		if !shouldRetry {
			// Non-retryable error or max attempts reached
			execErr := errpkg.AsExecutionError(err)
			if execErr != nil && !execErr.Retryable {
				slog.Error("execution failed with non-retryable error",
					append(logArgs, "attempt", attempt+1, "error", err.Error(), "error_code", execErr.Code)...)
			} else {
				slog.Error("execution failed, max attempts exceeded",
					append(logArgs, "attempt", attempt+1, "total_attempts", config.MaxAttempts, "error", err.Error())...)
			}
			return lastErr
		}

		// Log retry attempt
		slog.Info("retrying execution after failure",
			append(logArgs, "attempt", attempt+1, "next_delay", nextDelay.String(), "error", err.Error())...)

		// Check for context cancellation before sleeping
		select {
		case <-ctx.Done():
			slog.Warn("execution canceled during retry backoff",
				append(logArgs, "attempt", attempt+1, "error", ctx.Err())...)
			return ctx.Err()
		case <-time.After(nextDelay):
			// Backoff complete, continue to next attempt
		}
	}

	// All retries exhausted
	return lastErr
}

// flattenLogFields converts a map of log fields into a flat slice of alternating keys/values.
// This format is required by slog functions.
// Example: {a: "1", b: "2"} becomes ["a", "1", "b", "2"]
func flattenLogFields(fields map[string]interface{}) []interface{} {
	if len(fields) == 0 {
		return []interface{}{}
	}

	args := make([]interface{}, 0, len(fields)*2)
	for key, value := range fields {
		args = append(args, key, value)
	}
	return args
}
