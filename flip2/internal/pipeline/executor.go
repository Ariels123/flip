// Package pipeline provides pipeline execution with retry logic and timeout handling.
package pipeline

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"math"
	"os/exec"
	"time"
)

// =============================================================================
// RETRY CONFIGURATION
// =============================================================================

// RetryConfig holds retry behavior configuration for stage execution.
type RetryConfig struct {
	// MaxAttempts is the maximum number of attempts (including the first try).
	// Must be >= 1. If 1, no retries occur. If 3, allows 2 retries (3 total attempts).
	MaxAttempts int `json:"max_attempts"`

	// BackoffMultiplier is the factor by which to multiply delay after each retry.
	// For exponential backoff: delay = initialDelay * (backoffMultiplier ^ attempt)
	// Should be > 1.0 for meaningful backoff. Typical values: 2.0, 1.5.
	BackoffMultiplier float64 `json:"backoff_multiplier"`

	// InitialDelay is the delay before the first retry.
	// Subsequent retries use: initialDelay * (backoffMultiplier ^ retryAttempt)
	InitialDelay time.Duration `json:"initial_delay"`

	// MaxDelay is the maximum delay between retries. Prevents exponential growth from
	// becoming excessive. If 0, no maximum is enforced.
	MaxDelay time.Duration `json:"max_delay"`

	// OnlyTransient controls whether to retry only on transient errors.
	// If true, permanent errors (validation, auth, not found) are not retried.
	// If false, all non-terminal errors trigger retries.
	OnlyTransient bool `json:"only_transient"`
}

// IsTransientError determines whether an error is transient and should be retried.
// Transient errors include timeouts, network issues, and temporary failures.
// Permanent errors include validation, authentication, and authorization failures.
func IsTransientError(err error) bool {
	if err == nil {
		return false
	}

	errMsg := err.Error()

	// Timeout errors
	if errMsg == "timeout" || errMsg == "context deadline exceeded" {
		return true
	}

	// Network errors
	if errMsg == "connection refused" || errMsg == "network unreachable" ||
		errMsg == "host unreachable" || errMsg == "connection reset" {
		return true
	}

	// Temporary service errors
	if errMsg == "service unavailable" || errMsg == "temporarily unavailable" ||
		errMsg == "rate limited" || errMsg == "too many requests" {
		return true
	}

	// Specific HTTP-like status codes (if available in error message)
	if errMsg == "503" || errMsg == "429" || errMsg == "502" || errMsg == "504" {
		return true
	}

	// LLM-specific transient errors
	if errMsg == "model overloaded" || errMsg == "quota exceeded" ||
		errMsg == "temporary failure" {
		return true
	}

	return false
}

// DefaultRetryConfig returns sensible default retry configuration.
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxAttempts:       3,
		BackoffMultiplier: 2.0,
		InitialDelay:      1 * time.Second,
		MaxDelay:          30 * time.Second,
		OnlyTransient:     true,
	}
}

// =============================================================================
// STAGE EXECUTOR (RETRY LOGIC)
// =============================================================================

// StageExecutor handles execution of individual stages with retry logic.
type StageExecutor struct {
	// Logger is used for logging retry attempts and other diagnostic information.
	Logger *log.Logger

	// ExecuteFunc is the actual stage execution function.
	// It should return the output and any error.
	// This allows for dependency injection during testing.
	ExecuteFunc func(*Stage) (interface{}, error)
}

// NewStageExecutor creates a new stage executor with optional logger.
func NewStageExecutor(logger *log.Logger) *StageExecutor {
	if logger == nil {
		logger = log.New(nil, "", 0) // no-op logger
	}
	return &StageExecutor{
		Logger: logger,
	}
}

// Execute runs a stage with default retry configuration.
func (e *StageExecutor) Execute(stage *Stage) (interface{}, error) {
	return e.ExecuteWithRetry(stage, DefaultRetryConfig())
}

// ExecuteWithRetry executes a stage with configurable retry logic.
// Returns the stage output and any error encountered.
// If MaxAttempts is 1, no retries occur. If 3, up to 3 attempts are made.
func (e *StageExecutor) ExecuteWithRetry(stage *Stage, config *RetryConfig) (interface{}, error) {
	if config == nil {
		config = DefaultRetryConfig()
	}

	// Validate configuration
	if config.MaxAttempts < 1 {
		config.MaxAttempts = 1
	}
	if config.BackoffMultiplier < 1.0 {
		config.BackoffMultiplier = 1.0
	}
	if config.InitialDelay < 0 {
		config.InitialDelay = 0
	}

	var lastErr error
	var output interface{}

	// Attempt execution up to MaxAttempts times
	for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
		// Execute the stage
		result, err := e.ExecuteFunc(stage)

		if err == nil {
			// Success
			return result, nil
		}

		lastErr = err

		// Determine if we should retry
		shouldRetry := !config.OnlyTransient || IsTransientError(err)

		if !shouldRetry {
			// Permanent error - don't retry
			e.Logger.Printf("[%s] Stage execution failed with permanent error (attempt %d/%d): %v",
				stage.ID, attempt, config.MaxAttempts, err)
			return nil, err
		}

		// Check if we can retry
		if attempt >= config.MaxAttempts {
			// No more attempts
			e.Logger.Printf("[%s] Stage execution failed after %d attempts: %v",
				stage.ID, attempt, err)
			return nil, err
		}

		// Calculate backoff delay
		delay := e.calculateBackoff(attempt-1, config)

		e.Logger.Printf("[%s] Transient error on attempt %d/%d: %v. Retrying in %v...",
			stage.ID, attempt, config.MaxAttempts, err, delay)

		// Wait before retrying
		time.Sleep(delay)
	}

	return output, lastErr
}

// calculateBackoff computes the delay before the nth retry using exponential backoff.
// retryNumber is 0-indexed (0 for first retry, 1 for second, etc.)
func (e *StageExecutor) calculateBackoff(retryNumber int, config *RetryConfig) time.Duration {
	// Exponential backoff: delay = initialDelay * (backoffMultiplier ^ retryNumber)
	exponent := math.Pow(config.BackoffMultiplier, float64(retryNumber))
	delay := time.Duration(float64(config.InitialDelay) * exponent)

	// Apply maximum delay cap if configured
	if config.MaxDelay > 0 && delay > config.MaxDelay {
		delay = config.MaxDelay
	}

	return delay
}

// =============================================================================
// STAGE RETRY HELPERS
// =============================================================================

// StageWithRetryConfig is a convenience struct that associates a Stage with retry configuration.
type StageWithRetryConfig struct {
	Stage  *Stage
	Config *RetryConfig
}

// ExecutionResult contains the result of a stage execution including retry metadata.
type ExecutionResult struct {
	// Output is the result of the stage execution.
	Output interface{}

	// Error is any error that occurred.
	Error error

	// Attempts is the number of execution attempts made.
	Attempts int

	// LastRetryDelay is the delay used before the final (successful or failed) attempt.
	LastRetryDelay time.Duration

	// TotalDuration is the total time including all retry delays.
	TotalDuration time.Duration

	// StartedAt is when execution started.
	StartedAt time.Time

	// CompletedAt is when execution completed.
	CompletedAt time.Time
}

// ExecuteWithMetrics executes a stage with retry logic and collects detailed metrics.
func (e *StageExecutor) ExecuteWithMetrics(stage *Stage, config *RetryConfig) *ExecutionResult {
	if config == nil {
		config = DefaultRetryConfig()
	}

	result := &ExecutionResult{
		StartedAt: time.Now(),
	}

	// Validate configuration
	if config.MaxAttempts < 1 {
		config.MaxAttempts = 1
	}
	if config.BackoffMultiplier < 1.0 {
		config.BackoffMultiplier = 1.0
	}
	if config.InitialDelay < 0 {
		config.InitialDelay = 0
	}

	// Attempt execution up to MaxAttempts times
	for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
		result.Attempts = attempt

		// Execute the stage
		output, err := e.ExecuteFunc(stage)

		if err == nil {
			// Success
			result.Output = output
			result.CompletedAt = time.Now()
			result.TotalDuration = result.CompletedAt.Sub(result.StartedAt)
			return result
		}

		result.Error = err

		// Determine if we should retry
		shouldRetry := !config.OnlyTransient || IsTransientError(err)

		if !shouldRetry {
			// Permanent error - don't retry
			e.Logger.Printf("[%s] Stage execution failed with permanent error (attempt %d/%d): %v",
				stage.ID, attempt, config.MaxAttempts, err)
			result.CompletedAt = time.Now()
			result.TotalDuration = result.CompletedAt.Sub(result.StartedAt)
			return result
		}

		// Check if we can retry
		if attempt >= config.MaxAttempts {
			// No more attempts
			e.Logger.Printf("[%s] Stage execution failed after %d attempts: %v",
				stage.ID, attempt, err)
			result.CompletedAt = time.Now()
			result.TotalDuration = result.CompletedAt.Sub(result.StartedAt)
			return result
		}

		// Calculate backoff delay
		delay := e.calculateBackoff(attempt-1, config)
		result.LastRetryDelay = delay

		e.Logger.Printf("[%s] Transient error on attempt %d/%d: %v. Retrying in %v...",
			stage.ID, attempt, config.MaxAttempts, err, delay)

		// Wait before retrying
		time.Sleep(delay)
	}

	result.CompletedAt = time.Now()
	result.TotalDuration = result.CompletedAt.Sub(result.StartedAt)
	return result
}

// Validate checks if the retry configuration is valid.
func (r *RetryConfig) Validate() error {
	if r.MaxAttempts < 1 {
		return fmt.Errorf("MaxAttempts must be >= 1, got %d", r.MaxAttempts)
	}
	if r.BackoffMultiplier <= 0 {
		return fmt.Errorf("BackoffMultiplier must be > 0, got %f", r.BackoffMultiplier)
	}
	if r.InitialDelay < 0 {
		return fmt.Errorf("InitialDelay cannot be negative")
	}
	if r.MaxDelay < 0 {
		return fmt.Errorf("MaxDelay cannot be negative")
	}
	if r.MaxDelay > 0 && r.MaxDelay < r.InitialDelay {
		return fmt.Errorf("MaxDelay (%v) must be >= InitialDelay (%v)", r.MaxDelay, r.InitialDelay)
	}
	return nil
}

// String returns a human-readable description of the retry configuration.
func (r *RetryConfig) String() string {
	onlyTransient := "all errors"
	if r.OnlyTransient {
		onlyTransient = "transient errors only"
	}

	if r.MaxDelay > 0 {
		return fmt.Sprintf("max_attempts=%d backoff=%fx initial_delay=%v max_delay=%v retry=%s",
			r.MaxAttempts, r.BackoffMultiplier, r.InitialDelay, r.MaxDelay, onlyTransient)
	}
	return fmt.Sprintf("max_attempts=%d backoff=%fx initial_delay=%v retry=%s",
		r.MaxAttempts, r.BackoffMultiplier, r.InitialDelay, onlyTransient)
}

// =============================================================================
// TIMEOUT EXECUTOR
// =============================================================================

// TimeoutExecutor handles the execution of individual pipeline stages with timeout support.
type TimeoutExecutor struct {
	// defaultTimeout is the default timeout if not specified in the stage config.
	defaultTimeout time.Duration
}

// NewTimeoutExecutor creates a new timeout executor with an optional default timeout.
func NewTimeoutExecutor(defaultTimeout time.Duration) *TimeoutExecutor {
	if defaultTimeout <= 0 {
		defaultTimeout = 30 * time.Minute // Default to 30 minutes
	}
	return &TimeoutExecutor{
		defaultTimeout: defaultTimeout,
	}
}

// StageResult contains the result of executing a stage.
type StageResult struct {
	// Status is the outcome of the stage execution.
	Status StageExecutionStatus `json:"status"`

	// Output is the stdout produced by the stage command.
	Output string `json:"output"`

	// StdErr is the stderr produced by the stage command.
	StdErr string `json:"stderr"`

	// Duration is the total execution time.
	Duration time.Duration `json:"duration"`

	// Error is any error that occurred during execution.
	Error error `json:"error,omitempty"`

	// ExitCode is the exit code from the command (0 = success).
	ExitCode int `json:"exit_code"`

	// TimedOut indicates whether the execution exceeded the timeout.
	TimedOut bool `json:"timed_out"`

	// StartTime is when the stage started executing.
	StartTime time.Time `json:"start_time"`

	// EndTime is when the stage finished executing.
	EndTime time.Time `json:"end_time"`
}

// StageExecutionStatus represents the outcome of a stage execution.
type StageExecutionStatus string

const (
	// StageExecutionSuccess indicates the stage completed successfully.
	StageExecutionSuccess StageExecutionStatus = "success"

	// StageExecutionFailure indicates the stage failed.
	StageExecutionFailure StageExecutionStatus = "failure"

	// StageExecutionTimeout indicates the stage exceeded its timeout.
	StageExecutionTimeout StageExecutionStatus = "timeout"

	// StageExecutionCancelled indicates the stage was cancelled.
	StageExecutionCancelled StageExecutionStatus = "cancelled"
)

// ExecuteStageWithTimeout executes a stage command with the configured timeout.
// It manages the context lifecycle, captures output, and handles timeout gracefully.
func (te *TimeoutExecutor) ExecuteStageWithTimeout(stage *Stage) (*StageResult, error) {
	if stage == nil {
		return nil, fmt.Errorf("stage cannot be nil")
	}

	// Determine the timeout duration
	timeout := te.defaultTimeout
	if stage.Timeout != nil && stage.Timeout.Duration > 0 {
		timeout = stage.Timeout.Duration
	}

	// Create a context with the timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Record start time
	startTime := time.Now()

	// Create the command
	cmd := exec.CommandContext(ctx, "sh", "-c", stage.Command)

	// Prepare output buffers
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Start the command
	err := cmd.Start()
	if err != nil {
		return &StageResult{
			Status:    StageExecutionFailure,
			Error:     fmt.Errorf("failed to start command: %w", err),
			Output:    stdout.String(),
			StdErr:    stderr.String(),
			Duration:  time.Since(startTime),
			ExitCode:  -1,
			StartTime: startTime,
			EndTime:   time.Now(),
		}, nil
	}

	// Wait for the command to finish (or timeout)
	processErr := cmd.Wait()
	endTime := time.Now()
	duration := endTime.Sub(startTime)

	result := &StageResult{
		Output:    stdout.String(),
		StdErr:    stderr.String(),
		Duration:  duration,
		ExitCode:  cmd.ProcessState.ExitCode(),
		StartTime: startTime,
		EndTime:   endTime,
	}

	// Check if context was cancelled due to timeout
	if ctx.Err() == context.DeadlineExceeded {
		result.Status = StageExecutionTimeout
		result.TimedOut = true
		result.Error = fmt.Errorf("stage execution timed out after %v", timeout)
		// The command is already killed by cmd.Wait() when context deadline is exceeded
		return result, nil
	}

	// Check if context was cancelled for other reasons
	if ctx.Err() == context.Canceled {
		result.Status = StageExecutionCancelled
		result.Error = fmt.Errorf("stage execution was cancelled")
		return result, nil
	}

	// Handle command execution errors
	if processErr != nil {
		// Extract exit code if available
		if exitErr, ok := processErr.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		}
		result.Status = StageExecutionFailure
		result.Error = processErr
		return result, nil
	}

	// Success
	result.Status = StageExecutionSuccess
	return result, nil
}

// ExecuteStageWithContextTimeout executes a stage with a provided context instead of creating one internally.
// This allows for more fine-grained control over cancellation and timeout behavior.
func (te *TimeoutExecutor) ExecuteStageWithContextTimeout(ctx context.Context, stage *Stage) (*StageResult, error) {
	if stage == nil {
		return nil, fmt.Errorf("stage cannot be nil")
	}

	if ctx == nil {
		return nil, fmt.Errorf("context cannot be nil")
	}

	// Determine the timeout duration and create a new context with timeout if needed
	timeout := te.defaultTimeout
	if stage.Timeout != nil && stage.Timeout.Duration > 0 {
		timeout = stage.Timeout.Duration
	}

	// Create a child context with timeout, inheriting the cancellation from the parent context
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Record start time
	startTime := time.Now()

	// Create the command
	cmd := exec.CommandContext(ctx, "sh", "-c", stage.Command)

	// Prepare output buffers
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Start the command
	err := cmd.Start()
	if err != nil {
		return &StageResult{
			Status:    StageExecutionFailure,
			Error:     fmt.Errorf("failed to start command: %w", err),
			Output:    stdout.String(),
			StdErr:    stderr.String(),
			Duration:  time.Since(startTime),
			ExitCode:  -1,
			StartTime: startTime,
			EndTime:   time.Now(),
		}, nil
	}

	// Wait for the command to finish (or timeout)
	processErr := cmd.Wait()
	endTime := time.Now()
	duration := endTime.Sub(startTime)

	result := &StageResult{
		Output:    stdout.String(),
		StdErr:    stderr.String(),
		Duration:  duration,
		ExitCode:  cmd.ProcessState.ExitCode(),
		StartTime: startTime,
		EndTime:   endTime,
	}

	// Check if context was cancelled due to timeout
	if ctx.Err() == context.DeadlineExceeded {
		result.Status = StageExecutionTimeout
		result.TimedOut = true
		result.Error = fmt.Errorf("stage execution timed out after %v", timeout)
		return result, nil
	}

	// Check if context was cancelled for other reasons
	if ctx.Err() == context.Canceled {
		result.Status = StageExecutionCancelled
		result.Error = fmt.Errorf("stage execution was cancelled")
		return result, nil
	}

	// Handle command execution errors
	if processErr != nil {
		// Extract exit code if available
		if exitErr, ok := processErr.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		}
		result.Status = StageExecutionFailure
		result.Error = processErr
		return result, nil
	}

	// Success
	result.Status = StageExecutionSuccess
	return result, nil
}

// String returns the execution status as a string.
func (s StageExecutionStatus) String() string {
	return string(s)
}

// IsTerminal returns true if the execution status is terminal (no further action possible).
func (s StageExecutionStatus) IsTerminal() bool {
	return s == StageExecutionSuccess || s == StageExecutionFailure || s == StageExecutionTimeout || s == StageExecutionCancelled
}
