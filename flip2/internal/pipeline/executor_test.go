// Package pipeline provides tests for stage executor with retry logic.
package pipeline

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"testing"
	"time"
)

// =============================================================================
// TEST HELPERS
// =============================================================================

// mockExecutor tracks execution attempts for testing.
type mockExecutor struct {
	attempts      int
	failUntil     int
	errorType     error
	lastAttempted time.Time
	attemptTimes  []time.Time
}

// newMockExecutor creates a test executor that fails the first N times.
func newMockExecutor(failUntil int, errorType error) *mockExecutor {
	return &mockExecutor{
		failUntil:    failUntil,
		errorType:    errorType,
		attemptTimes: []time.Time{},
	}
}

// execute simulates stage execution with configurable failures.
func (m *mockExecutor) execute(stage *Stage) (interface{}, error) {
	m.attempts++
	m.lastAttempted = time.Now()
	m.attemptTimes = append(m.attemptTimes, m.lastAttempted)

	if m.attempts <= m.failUntil {
		return nil, m.errorType
	}
	return fmt.Sprintf("Success after %d attempts", m.attempts), nil
}

// createTestStage creates a stage for testing.
func createTestStage(id string) *Stage {
	return &Stage{
		ID:      id,
		Name:    "Test Stage",
		Backend: "claude",
		Command: "test command",
	}
}

// createTestLogger creates a logger that discards output during tests.
func createTestLogger() *log.Logger {
	return log.New(os.Stderr, "[test] ", log.LstdFlags)
}

// =============================================================================
// RETRY CONFIG TESTS
// =============================================================================

func TestDefaultRetryConfig(t *testing.T) {
	config := DefaultRetryConfig()

	if config.MaxAttempts != 3 {
		t.Errorf("expected MaxAttempts=3, got %d", config.MaxAttempts)
	}
	if config.BackoffMultiplier != 2.0 {
		t.Errorf("expected BackoffMultiplier=2.0, got %f", config.BackoffMultiplier)
	}
	if config.InitialDelay != 1*time.Second {
		t.Errorf("expected InitialDelay=1s, got %v", config.InitialDelay)
	}
	if config.MaxDelay != 30*time.Second {
		t.Errorf("expected MaxDelay=30s, got %v", config.MaxDelay)
	}
	if !config.OnlyTransient {
		t.Errorf("expected OnlyTransient=true")
	}
}

func TestRetryConfigValidation(t *testing.T) {
	tests := []struct {
		name       string
		config     *RetryConfig
		expectErr  bool
		errMessage string
	}{
		{
			name: "valid config",
			config: &RetryConfig{
				MaxAttempts:       3,
				BackoffMultiplier: 2.0,
				InitialDelay:      100 * time.Millisecond,
				MaxDelay:          10 * time.Second,
			},
			expectErr: false,
		},
		{
			name: "invalid max attempts",
			config: &RetryConfig{
				MaxAttempts:       0,
				BackoffMultiplier: 2.0,
				InitialDelay:      100 * time.Millisecond,
			},
			expectErr:  true,
			errMessage: "MaxAttempts must be >= 1",
		},
		{
			name: "invalid backoff multiplier",
			config: &RetryConfig{
				MaxAttempts:       3,
				BackoffMultiplier: 0,
				InitialDelay:      100 * time.Millisecond,
			},
			expectErr:  true,
			errMessage: "BackoffMultiplier must be > 0",
		},
		{
			name: "negative initial delay",
			config: &RetryConfig{
				MaxAttempts:       3,
				BackoffMultiplier: 2.0,
				InitialDelay:      -1 * time.Second,
			},
			expectErr:  true,
			errMessage: "InitialDelay cannot be negative",
		},
		{
			name: "max delay less than initial delay",
			config: &RetryConfig{
				MaxAttempts:       3,
				BackoffMultiplier: 2.0,
				InitialDelay:      10 * time.Second,
				MaxDelay:          1 * time.Second,
			},
			expectErr:  true,
			errMessage: "must be >=",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.expectErr {
				t.Fatalf("expected error=%v, got=%v", tt.expectErr, err)
			}
			if tt.expectErr && !strings.Contains(err.Error(), tt.errMessage) {
				t.Errorf("expected error containing '%s', got '%s'", tt.errMessage, err.Error())
			}
		})
	}
}

// =============================================================================
// TRANSIENT ERROR DETECTION TESTS
// =============================================================================

func TestIsTransientError(t *testing.T) {
	tests := []struct {
		name              string
		errorMsg          string
		expectTransient   bool
	}{
		{"nil error", "", false},
		{"timeout", "timeout", true},
		{"context deadline", "context deadline exceeded", true},
		{"connection refused", "connection refused", true},
		{"network unreachable", "network unreachable", true},
		{"connection reset", "connection reset", true},
		{"service unavailable", "service unavailable", true},
		{"rate limited", "rate limited", true},
		{"HTTP 503", "503", true},
		{"HTTP 429", "429", true},
		{"model overloaded", "model overloaded", true},
		{"quota exceeded", "quota exceeded", true},
		{"validation error", "validation error", false},
		{"not found", "not found", false},
		{"permission denied", "permission denied", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			if tt.errorMsg != "" {
				err = errors.New(tt.errorMsg)
			}

			result := IsTransientError(err)
			if result != tt.expectTransient {
				t.Errorf("expected transient=%v, got %v", tt.expectTransient, result)
			}
		})
	}
}

// =============================================================================
// STAGE EXECUTOR BASIC TESTS
// =============================================================================

func TestExecutorBasicSuccess(t *testing.T) {
	logger := createTestLogger()
	executor := NewStageExecutor(logger)
	mock := newMockExecutor(0, nil)

	executor.ExecuteFunc = mock.execute

	stage := createTestStage("test-stage")
	output, err := executor.Execute(stage)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if mock.attempts != 1 {
		t.Errorf("expected 1 attempt, got %d", mock.attempts)
	}
	if output == nil {
		t.Errorf("expected output, got nil")
	}
}

func TestExecutorBasicFailure(t *testing.T) {
	logger := createTestLogger()
	executor := NewStageExecutor(logger)

	expectedErr := errors.New("timeout")
	mock := newMockExecutor(10, expectedErr)

	executor.ExecuteFunc = mock.execute

	stage := createTestStage("test-stage")
	output, err := executor.Execute(stage)

	if err == nil {
		t.Errorf("expected error, got nil")
	}
	if output != nil {
		t.Errorf("expected no output on failure, got %v", output)
	}
	if mock.attempts != 3 {
		t.Errorf("expected 3 attempts (default max), got %d", mock.attempts)
	}
}

// =============================================================================
// RETRY LOGIC TESTS
// =============================================================================

func TestRetryOnTransientError(t *testing.T) {
	logger := createTestLogger()
	executor := NewStageExecutor(logger)

	mock := newMockExecutor(2, errors.New("timeout"))
	executor.ExecuteFunc = mock.execute

	config := &RetryConfig{
		MaxAttempts:       3,
		BackoffMultiplier: 2.0,
		InitialDelay:      10 * time.Millisecond,
		OnlyTransient:     true,
	}

	stage := createTestStage("test-stage")
	output, err := executor.ExecuteWithRetry(stage, config)

	if err != nil {
		t.Errorf("expected success after retries, got error: %v", err)
	}
	if mock.attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", mock.attempts)
	}
	if output == nil {
		t.Errorf("expected output, got nil")
	}
}

func TestNoRetryOnPermanentError(t *testing.T) {
	logger := createTestLogger()
	executor := NewStageExecutor(logger)

	permanentErr := errors.New("validation error")
	mock := newMockExecutor(10, permanentErr)
	executor.ExecuteFunc = mock.execute

	config := &RetryConfig{
		MaxAttempts:       3,
		BackoffMultiplier: 2.0,
		InitialDelay:      10 * time.Millisecond,
		OnlyTransient:     true,
	}

	stage := createTestStage("test-stage")
	output, err := executor.ExecuteWithRetry(stage, config)

	if err == nil {
		t.Errorf("expected permanent error, got success")
	}
	if mock.attempts != 1 {
		t.Errorf("expected 1 attempt (no retries on permanent error), got %d", mock.attempts)
	}
	if output != nil {
		t.Errorf("expected no output on permanent error")
	}
}

func TestRetryAllErrors(t *testing.T) {
	logger := createTestLogger()
	executor := NewStageExecutor(logger)

	permanentErr := errors.New("validation error")
	mock := newMockExecutor(2, permanentErr)
	executor.ExecuteFunc = mock.execute

	config := &RetryConfig{
		MaxAttempts:       3,
		BackoffMultiplier: 2.0,
		InitialDelay:      10 * time.Millisecond,
		OnlyTransient:     false,
	}

	stage := createTestStage("test-stage")
	_, err := executor.ExecuteWithRetry(stage, config)

	if err != nil {
		t.Errorf("expected success after retries, got error: %v", err)
	}
	if mock.attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", mock.attempts)
	}
}

// =============================================================================
// BACKOFF TIMING TESTS
// =============================================================================

func TestExponentialBackoffTiming(t *testing.T) {
	logger := createTestLogger()
	executor := NewStageExecutor(logger)

	mock := newMockExecutor(2, errors.New("timeout"))
	executor.ExecuteFunc = mock.execute

	initialDelay := 20 * time.Millisecond
	config := &RetryConfig{
		MaxAttempts:       3,
		BackoffMultiplier: 2.0,
		InitialDelay:      initialDelay,
		MaxDelay:          0,
		OnlyTransient:     true,
	}

	stage := createTestStage("test-stage")
	startTime := time.Now()
	_, err := executor.ExecuteWithRetry(stage, config)
	totalTime := time.Since(startTime)

	if err != nil {
		t.Errorf("expected success, got error: %v", err)
	}

	expectedMin := initialDelay + (initialDelay * 2)
	if totalTime < expectedMin {
		t.Errorf("expected total time >= %v, got %v", expectedMin, totalTime)
	}

	if len(mock.attemptTimes) != 3 {
		t.Fatalf("expected 3 attempts, got %d", len(mock.attemptTimes))
	}

	delay1 := mock.attemptTimes[1].Sub(mock.attemptTimes[0])
	delay2 := mock.attemptTimes[2].Sub(mock.attemptTimes[1])

	if delay1 < initialDelay {
		t.Errorf("first retry delay %v should be at least %v", delay1, initialDelay)
	}

	expectedSecondDelay := initialDelay * 2
	if delay2 < expectedSecondDelay {
		t.Errorf("second retry delay %v should be at least %v", delay2, expectedSecondDelay)
	}

	if delay2 <= delay1 {
		t.Errorf("expected exponential backoff: delay2(%v) > delay1(%v)", delay2, delay1)
	}
}

func TestBackoffMaxDelayCap(t *testing.T) {
	logger := createTestLogger()
	executor := NewStageExecutor(logger)

	mock := newMockExecutor(5, errors.New("timeout"))
	executor.ExecuteFunc = mock.execute

	maxDelay := 50 * time.Millisecond
	config := &RetryConfig{
		MaxAttempts:       6,
		BackoffMultiplier: 2.0,
		InitialDelay:      10 * time.Millisecond,
		MaxDelay:          maxDelay,
		OnlyTransient:     true,
	}

	stage := createTestStage("test-stage")
	executor.ExecuteWithRetry(stage, config)

	if len(mock.attemptTimes) < 2 {
		t.Fatalf("expected at least 2 attempts")
	}

	for i := 1; i < len(mock.attemptTimes); i++ {
		delay := mock.attemptTimes[i].Sub(mock.attemptTimes[i-1])
		if delay > maxDelay+10*time.Millisecond {
			t.Errorf("delay %v exceeded maxDelay %v", delay, maxDelay)
		}
	}
}

// =============================================================================
// EXECUTION RESULT METRICS TESTS
// =============================================================================

func TestExecutionResultMetrics(t *testing.T) {
	logger := createTestLogger()
	executor := NewStageExecutor(logger)

	mock := newMockExecutor(1, errors.New("timeout"))
	executor.ExecuteFunc = mock.execute

	config := &RetryConfig{
		MaxAttempts:       3,
		BackoffMultiplier: 2.0,
		InitialDelay:      10 * time.Millisecond,
		OnlyTransient:     true,
	}

	stage := createTestStage("test-stage")
	result := executor.ExecuteWithMetrics(stage, config)

	if result.Error != nil {
		t.Errorf("expected success, got error: %v", result.Error)
	}
	if result.Output == nil {
		t.Errorf("expected output, got nil")
	}
	if result.Attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", result.Attempts)
	}
	if result.TotalDuration <= 0 {
		t.Errorf("expected positive duration, got %v", result.TotalDuration)
	}
	if result.LastRetryDelay <= 0 {
		t.Errorf("expected positive retry delay, got %v", result.LastRetryDelay)
	}
	if result.CompletedAt.Before(result.StartedAt) {
		t.Errorf("completed time should be after started time")
	}
}

func TestExecutionResultMetricsWithFailure(t *testing.T) {
	logger := createTestLogger()
	executor := NewStageExecutor(logger)

	permanentErr := errors.New("validation error")
	mock := newMockExecutor(10, permanentErr)
	executor.ExecuteFunc = mock.execute

	config := &RetryConfig{
		MaxAttempts:       3,
		BackoffMultiplier: 2.0,
		InitialDelay:      10 * time.Millisecond,
		OnlyTransient:     true,
	}

	stage := createTestStage("test-stage")
	result := executor.ExecuteWithMetrics(stage, config)

	if result.Error == nil {
		t.Errorf("expected error, got success")
	}
	if result.Output != nil {
		t.Errorf("expected no output on failure")
	}
	if result.Attempts != 1 {
		t.Errorf("expected 1 attempt (no retry on permanent error), got %d", result.Attempts)
	}
	if result.TotalDuration <= 0 {
		t.Errorf("expected positive duration")
	}
}

// =============================================================================
// EDGE CASES AND CONFIG HANDLING TESTS
// =============================================================================

func TestNilConfigUsesDefaults(t *testing.T) {
	logger := createTestLogger()
	executor := NewStageExecutor(logger)

	mock := newMockExecutor(2, errors.New("timeout"))
	executor.ExecuteFunc = mock.execute

	stage := createTestStage("test-stage")
	_, err := executor.ExecuteWithRetry(stage, nil)

	if err != nil {
		t.Errorf("expected success with default config, got error: %v", err)
	}
	if mock.attempts != 3 {
		t.Errorf("expected 3 attempts (default), got %d", mock.attempts)
	}
}

func TestInvalidConfigSanitization(t *testing.T) {
	logger := createTestLogger()
	executor := NewStageExecutor(logger)

	mock := newMockExecutor(0, errors.New("timeout"))
	executor.ExecuteFunc = mock.execute

	config := &RetryConfig{
		MaxAttempts:       0,
		BackoffMultiplier: 0.5,
		InitialDelay:      -1 * time.Second,
	}

	stage := createTestStage("test-stage")
	_, err := executor.ExecuteWithRetry(stage, config)

	if err != nil {
		t.Errorf("expected success with sanitized config, got error: %v", err)
	}
	if mock.attempts != 1 {
		t.Errorf("expected 1 attempt with sanitized config, got %d", mock.attempts)
	}
}

func TestMaxAttemptsOne(t *testing.T) {
	logger := createTestLogger()
	executor := NewStageExecutor(logger)

	mock := newMockExecutor(10, errors.New("timeout"))
	executor.ExecuteFunc = mock.execute

	config := &RetryConfig{
		MaxAttempts:       1,
		BackoffMultiplier: 2.0,
		InitialDelay:      10 * time.Millisecond,
		OnlyTransient:     true,
	}

	stage := createTestStage("test-stage")
	output, err := executor.ExecuteWithRetry(stage, config)

	if err == nil {
		t.Errorf("expected error with MaxAttempts=1")
	}
	if mock.attempts != 1 {
		t.Errorf("expected 1 attempt, got %d", mock.attempts)
	}
	if output != nil {
		t.Errorf("expected no output on failure")
	}
}

func TestRetryConfigString(t *testing.T) {
	config := &RetryConfig{
		MaxAttempts:       3,
		BackoffMultiplier: 2.0,
		InitialDelay:      1 * time.Second,
		MaxDelay:          30 * time.Second,
		OnlyTransient:     true,
	}

	str := config.String()
	if str == "" {
		t.Errorf("expected non-empty string representation")
	}
	if !strings.Contains(str, "max_attempts=3") {
		t.Errorf("string should contain max_attempts=3, got: %s", str)
	}
	if !strings.Contains(str, "backoff=2") {
		t.Errorf("string should contain backoff=2, got: %s", str)
	}
}

// =============================================================================
// BENCHMARK TESTS
// =============================================================================

func BenchmarkExecuteSuccess(b *testing.B) {
	logger := createTestLogger()
	executor := NewStageExecutor(logger)

	mock := newMockExecutor(0, nil)
	executor.ExecuteFunc = mock.execute

	stage := createTestStage("bench-stage")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		executor.Execute(stage)
	}
}

func BenchmarkBackoffCalculation(b *testing.B) {
	logger := createTestLogger()
	executor := NewStageExecutor(logger)

	config := &RetryConfig{
		MaxAttempts:       10,
		BackoffMultiplier: 2.0,
		InitialDelay:      1 * time.Second,
		MaxDelay:          30 * time.Second,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		executor.calculateBackoff(i, config)
	}
}
