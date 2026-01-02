package pipeline

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

// TestTimeoutExecutorSuccess tests successful stage execution without timeout.
func TestTimeoutExecutorSuccess(t *testing.T) {
	executor := NewTimeoutExecutor(5 * time.Second)

	stage := &Stage{
		ID:      "test-stage",
		Name:    "Test Stage",
		Backend: "test",
		Command: "echo 'Hello, World!'",
	}

	result, err := executor.ExecuteStageWithTimeout(stage)

	if err != nil {
		t.Fatalf("ExecuteStageWithTimeout returned error: %v", err)
	}

	if result == nil {
		t.Fatalf("ExecuteStageWithTimeout returned nil result")
	}

	if result.Status != StageExecutionSuccess {
		t.Errorf("Expected status %v, got %v", StageExecutionSuccess, result.Status)
	}

	if !strings.Contains(result.Output, "Hello, World!") {
		t.Errorf("Expected 'Hello, World!' in output, got: %v", result.Output)
	}

	if result.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %v", result.ExitCode)
	}

	if result.TimedOut {
		t.Errorf("Expected TimedOut to be false, got true")
	}

	if result.Duration <= 0 {
		t.Errorf("Expected positive duration, got %v", result.Duration)
	}
}

// TestTimeoutExecutorWithStderr tests that stderr is captured.
func TestTimeoutExecutorWithStderr(t *testing.T) {
	executor := NewTimeoutExecutor(5 * time.Second)

	stage := &Stage{
		ID:      "stderr-stage",
		Name:    "Stderr Stage",
		Backend: "test",
		Command: "sh -c 'echo stdout && echo stderr >&2'",
	}

	result, err := executor.ExecuteStageWithTimeout(stage)

	if err != nil {
		t.Fatalf("ExecuteStageWithTimeout returned error: %v", err)
	}

	if !strings.Contains(result.Output, "stdout") {
		t.Errorf("Expected 'stdout' in output, got: %v", result.Output)
	}

	if !strings.Contains(result.StdErr, "stderr") {
		t.Errorf("Expected 'stderr' in stderr, got: %v", result.StdErr)
	}
}

// TestTimeoutExecutorFailure tests stage execution failure.
func TestTimeoutExecutorFailure(t *testing.T) {
	executor := NewTimeoutExecutor(5 * time.Second)

	stage := &Stage{
		ID:      "fail-stage",
		Name:    "Fail Stage",
		Backend: "test",
		Command: "exit 1",
	}

	result, err := executor.ExecuteStageWithTimeout(stage)

	if err != nil {
		t.Fatalf("ExecuteStageWithTimeout returned error: %v", err)
	}

	if result.Status != StageExecutionFailure {
		t.Errorf("Expected status %v, got %v", StageExecutionFailure, result.Status)
	}

	if result.ExitCode != 1 {
		t.Errorf("Expected exit code 1, got %v", result.ExitCode)
	}

	if result.Error == nil {
		t.Errorf("Expected error to be set for failed command")
	}
}

// TestTimeoutExecutorTimeout tests that stage execution times out properly.
func TestTimeoutExecutorTimeout(t *testing.T) {
	executor := NewTimeoutExecutor(1 * time.Second)

	stage := &Stage{
		ID:      "timeout-stage",
		Name:    "Timeout Stage",
		Backend: "test",
		Command: "sleep 10",
		Timeout: &Duration{Duration: 1 * time.Second},
	}

	result, err := executor.ExecuteStageWithTimeout(stage)

	if err != nil {
		t.Fatalf("ExecuteStageWithTimeout returned error: %v", err)
	}

	if result.Status != StageExecutionTimeout {
		t.Errorf("Expected status %v, got %v", StageExecutionTimeout, result.Status)
	}

	if !result.TimedOut {
		t.Errorf("Expected TimedOut to be true, got false")
	}

	if result.Error == nil {
		t.Errorf("Expected error to be set for timed out command")
	}

	// Verify that execution time is close to the timeout value (with some tolerance)
	// Should be roughly 1 second, not 10 seconds
	if result.Duration > 3*time.Second {
		t.Errorf("Expected duration around 1 second, got %v", result.Duration)
	}
}

// TestTimeoutExecutorWithStageDuration tests that stage-specific timeout is used.
func TestTimeoutExecutorWithStageDuration(t *testing.T) {
	executor := NewTimeoutExecutor(10 * time.Second)

	// Stage has a shorter timeout than the executor default
	stage := &Stage{
		ID:      "short-timeout-stage",
		Name:    "Short Timeout Stage",
		Backend: "test",
		Command: "sleep 5",
		Timeout: &Duration{Duration: 1 * time.Second},
	}

	result, err := executor.ExecuteStageWithTimeout(stage)

	if err != nil {
		t.Fatalf("ExecuteStageWithTimeout returned error: %v", err)
	}

	if result.Status != StageExecutionTimeout {
		t.Errorf("Expected status %v, got %v", StageExecutionTimeout, result.Status)
	}

	if !result.TimedOut {
		t.Errorf("Expected TimedOut to be true, got false")
	}

	if result.Duration > 3*time.Second {
		t.Errorf("Expected duration around 1 second, got %v", result.Duration)
	}
}

// TestTimeoutExecutorNilStage tests that nil stage is handled properly.
func TestTimeoutExecutorNilStage(t *testing.T) {
	executor := NewTimeoutExecutor(5 * time.Second)

	_, err := executor.ExecuteStageWithTimeout(nil)

	if err == nil {
		t.Errorf("Expected error for nil stage, got nil")
	}

	if !strings.Contains(err.Error(), "stage cannot be nil") {
		t.Errorf("Expected 'stage cannot be nil' in error message, got: %v", err)
	}
}

// TestTimeoutExecutorCommandNotFound tests that missing command is handled.
func TestTimeoutExecutorCommandNotFound(t *testing.T) {
	executor := NewTimeoutExecutor(5 * time.Second)

	stage := &Stage{
		ID:      "notfound-stage",
		Name:    "Not Found Stage",
		Backend: "test",
		Command: "this_command_does_not_exist_12345",
	}

	result, err := executor.ExecuteStageWithTimeout(stage)

	if err != nil {
		t.Fatalf("ExecuteStageWithTimeout returned error: %v", err)
	}

	if result.Status != StageExecutionFailure {
		t.Errorf("Expected status %v, got %v", StageExecutionFailure, result.Status)
	}

	if result.Error == nil {
		t.Errorf("Expected error to be set for missing command")
	}
}

// TestTimeoutExecutorWithContext tests execution using a provided context.
func TestTimeoutExecutorWithContext(t *testing.T) {
	executor := NewTimeoutExecutor(10 * time.Second)

	stage := &Stage{
		ID:      "context-stage",
		Name:    "Context Stage",
		Backend: "test",
		Command: "echo 'test'",
	}

	ctx := context.Background()
	result, err := executor.ExecuteStageWithContextTimeout(ctx, stage)

	if err != nil {
		t.Fatalf("ExecuteStageWithContextTimeout returned error: %v", err)
	}

	if result.Status != StageExecutionSuccess {
		t.Errorf("Expected status %v, got %v", StageExecutionSuccess, result.Status)
	}
}

// TestTimeoutExecutorWithContextTimeout tests timeout using a provided context.
func TestTimeoutExecutorWithContextTimeout(t *testing.T) {
	executor := NewTimeoutExecutor(10 * time.Second)

	stage := &Stage{
		ID:      "context-timeout-stage",
		Name:    "Context Timeout Stage",
		Backend: "test",
		Command: "sleep 5",
		Timeout: &Duration{Duration: 1 * time.Second},
	}

	ctx := context.Background()
	result, err := executor.ExecuteStageWithContextTimeout(ctx, stage)

	if err != nil {
		t.Fatalf("ExecuteStageWithContextTimeout returned error: %v", err)
	}

	if result.Status != StageExecutionTimeout {
		t.Errorf("Expected status %v, got %v", StageExecutionTimeout, result.Status)
	}

	if !result.TimedOut {
		t.Errorf("Expected TimedOut to be true, got false")
	}
}

// TestTimeoutExecutorWithCancelledContext tests execution with a cancelled context.
func TestTimeoutExecutorWithCancelledContext(t *testing.T) {
	executor := NewTimeoutExecutor(10 * time.Second)

	stage := &Stage{
		ID:      "cancelled-stage",
		Name:    "Cancelled Stage",
		Backend: "test",
		Command: "sleep 10",
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	result, err := executor.ExecuteStageWithContextTimeout(ctx, stage)

	if err != nil {
		t.Fatalf("ExecuteStageWithContextTimeout returned error: %v", err)
	}

	if result.Status != StageExecutionCancelled {
		t.Errorf("Expected status %v, got %v", StageExecutionCancelled, result.Status)
	}

	if result.Error == nil {
		t.Errorf("Expected error to be set for cancelled stage")
	}
}

// TestTimeoutExecutorWithNilContext tests that nil context is rejected.
func TestTimeoutExecutorWithNilContext(t *testing.T) {
	executor := NewTimeoutExecutor(5 * time.Second)

	stage := &Stage{
		ID:      "test-stage",
		Name:    "Test Stage",
		Backend: "test",
		Command: "echo test",
	}

	_, err := executor.ExecuteStageWithContextTimeout(nil, stage)

	if err == nil {
		t.Errorf("Expected error for nil context, got nil")
	}

	if !strings.Contains(err.Error(), "context cannot be nil") {
		t.Errorf("Expected 'context cannot be nil' in error message, got: %v", err)
	}
}

// TestStageExecutionStatusString tests string representation of execution status.
func TestStageExecutionStatusString(t *testing.T) {
	tests := []struct {
		status   StageExecutionStatus
		expected string
	}{
		{StageExecutionSuccess, "success"},
		{StageExecutionFailure, "failure"},
		{StageExecutionTimeout, "timeout"},
		{StageExecutionCancelled, "cancelled"},
	}

	for _, test := range tests {
		if test.status.String() != test.expected {
			t.Errorf("Expected %q, got %q", test.expected, test.status.String())
		}
	}
}

// TestStageExecutionStatusIsTerminal tests terminal status detection.
func TestStageExecutionStatusIsTerminal(t *testing.T) {
	tests := []struct {
		status   StageExecutionStatus
		terminal bool
	}{
		{StageExecutionSuccess, true},
		{StageExecutionFailure, true},
		{StageExecutionTimeout, true},
		{StageExecutionCancelled, true},
	}

	for _, test := range tests {
		if test.status.IsTerminal() != test.terminal {
			t.Errorf("Expected IsTerminal() to return %v for status %q", test.terminal, test.status.String())
		}
	}
}

// TestTimeoutExecutorComplexCommand tests execution of a complex multi-command shell script.
func TestTimeoutExecutorComplexCommand(t *testing.T) {
	executor := NewTimeoutExecutor(5 * time.Second)

	stage := &Stage{
		ID:      "complex-stage",
		Name:    "Complex Stage",
		Backend: "test",
		Command: "echo 'line1' && echo 'line2' && echo 'line3'",
	}

	result, err := executor.ExecuteStageWithTimeout(stage)

	if err != nil {
		t.Fatalf("ExecuteStageWithTimeout returned error: %v", err)
	}

	if result.Status != StageExecutionSuccess {
		t.Errorf("Expected status %v, got %v", StageExecutionSuccess, result.Status)
	}

	output := result.Output
	if !strings.Contains(output, "line1") || !strings.Contains(output, "line2") || !strings.Contains(output, "line3") {
		t.Errorf("Expected all lines in output, got: %v", output)
	}
}

// TestTimeoutExecutorDefaultTimeout tests that default timeout is applied.
func TestTimeoutExecutorDefaultTimeout(t *testing.T) {
	defaultTimeout := 1 * time.Second
	executor := NewTimeoutExecutor(defaultTimeout)

	stage := &Stage{
		ID:      "default-timeout-stage",
		Name:    "Default Timeout Stage",
		Backend: "test",
		Command: "sleep 10",
		// No timeout specified, should use executor's default
	}

	result, err := executor.ExecuteStageWithTimeout(stage)

	if err != nil {
		t.Fatalf("ExecuteStageWithTimeout returned error: %v", err)
	}

	if result.Status != StageExecutionTimeout {
		t.Errorf("Expected status %v, got %v", StageExecutionTimeout, result.Status)
	}

	if !result.TimedOut {
		t.Errorf("Expected TimedOut to be true, got false")
	}
}

// TestTimeoutExecutorZeroExitCode tests that exit code 0 is properly captured.
func TestTimeoutExecutorZeroExitCode(t *testing.T) {
	executor := NewTimeoutExecutor(5 * time.Second)

	stage := &Stage{
		ID:      "zero-exit-stage",
		Name:    "Zero Exit Stage",
		Backend: "test",
		Command: "true",
	}

	result, err := executor.ExecuteStageWithTimeout(stage)

	if err != nil {
		t.Fatalf("ExecuteStageWithTimeout returned error: %v", err)
	}

	if result.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %v", result.ExitCode)
	}

	if result.Status != StageExecutionSuccess {
		t.Errorf("Expected status %v, got %v", StageExecutionSuccess, result.Status)
	}
}

// TestTimeoutExecutorNonZeroExitCode tests that non-zero exit codes are properly captured.
func TestTimeoutExecutorNonZeroExitCode(t *testing.T) {
	executor := NewTimeoutExecutor(5 * time.Second)

	stage := &Stage{
		ID:      "nonzero-exit-stage",
		Name:    "Non-zero Exit Stage",
		Backend: "test",
		Command: "exit 42",
	}

	result, err := executor.ExecuteStageWithTimeout(stage)

	if err != nil {
		t.Fatalf("ExecuteStageWithTimeout returned error: %v", err)
	}

	if result.ExitCode != 42 {
		t.Errorf("Expected exit code 42, got %v", result.ExitCode)
	}

	if result.Status != StageExecutionFailure {
		t.Errorf("Expected status %v, got %v", StageExecutionFailure, result.Status)
	}
}

// TestTimeoutExecutorStartAndEndTime tests that start and end times are recorded.
func TestTimeoutExecutorStartAndEndTime(t *testing.T) {
	executor := NewTimeoutExecutor(5 * time.Second)

	stage := &Stage{
		ID:      "timing-stage",
		Name:    "Timing Stage",
		Backend: "test",
		Command: "sleep 0.1",
	}

	beforeExecution := time.Now()
	result, err := executor.ExecuteStageWithTimeout(stage)
	afterExecution := time.Now()

	if err != nil {
		t.Fatalf("ExecuteStageWithTimeout returned error: %v", err)
	}

	if result.StartTime.Before(beforeExecution) {
		t.Errorf("StartTime should be after before-execution time")
	}

	if result.EndTime.After(afterExecution) {
		t.Errorf("EndTime should be before after-execution time")
	}

	if result.EndTime.Before(result.StartTime) {
		t.Errorf("EndTime should be after StartTime")
	}

	if !result.StartTime.Before(result.EndTime) {
		t.Errorf("StartTime should be before EndTime")
	}
}

// TestNewTimeoutExecutor tests that NewTimeoutExecutor sets default timeout.
func TestNewTimeoutExecutor(t *testing.T) {
	executor := NewTimeoutExecutor(5 * time.Second)

	if executor == nil {
		t.Fatalf("NewTimeoutExecutor returned nil")
	}

	if executor.defaultTimeout != 5*time.Second {
		t.Errorf("Expected default timeout 5s, got %v", executor.defaultTimeout)
	}
}

// TestNewTimeoutExecutorZeroTimeout tests that zero timeout gets default.
func TestNewTimeoutExecutorZeroTimeout(t *testing.T) {
	executor := NewTimeoutExecutor(0)

	if executor == nil {
		t.Fatalf("NewTimeoutExecutor returned nil")
	}

	if executor.defaultTimeout != 30*time.Minute {
		t.Errorf("Expected default timeout 30m, got %v", executor.defaultTimeout)
	}
}

// TestNewTimeoutExecutorNegativeTimeout tests that negative timeout gets default.
func TestNewTimeoutExecutorNegativeTimeout(t *testing.T) {
	executor := NewTimeoutExecutor(-1 * time.Second)

	if executor == nil {
		t.Fatalf("NewTimeoutExecutor returned nil")
	}

	if executor.defaultTimeout != 30*time.Minute {
		t.Errorf("Expected default timeout 30m, got %v", executor.defaultTimeout)
	}
}

// BenchmarkTimeoutExecutorSuccess benchmarks successful stage execution.
func BenchmarkTimeoutExecutorSuccess(b *testing.B) {
	executor := NewTimeoutExecutor(5 * time.Second)

	stage := &Stage{
		ID:      "bench-stage",
		Name:    "Bench Stage",
		Backend: "test",
		Command: "echo 'benchmark'",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		executor.ExecuteStageWithTimeout(stage)
	}
}

// BenchmarkTimeoutExecutorWithTimeout benchmarks execution with timeout.
func BenchmarkTimeoutExecutorWithTimeout(b *testing.B) {
	executor := NewTimeoutExecutor(5 * time.Second)

	stage := &Stage{
		ID:      "bench-timeout-stage",
		Name:    "Bench Timeout Stage",
		Backend: "test",
		Command: "echo 'benchmark'",
		Timeout: &Duration{Duration: 1 * time.Second},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		executor.ExecuteStageWithTimeout(stage)
	}
}

// TestTimeoutExecutorContextDeadlineExceeded tests that context deadline is properly detected.
func TestTimeoutExecutorContextDeadlineExceeded(t *testing.T) {
	executor := NewTimeoutExecutor(30 * time.Second)

	stage := &Stage{
		ID:      "deadline-stage",
		Name:    "Deadline Stage",
		Backend: "test",
		Command: "sleep 10",
		Timeout: &Duration{Duration: 500 * time.Millisecond},
	}

	result, err := executor.ExecuteStageWithTimeout(stage)

	if err != nil {
		t.Fatalf("ExecuteStageWithTimeout returned error: %v", err)
	}

	if result.Status != StageExecutionTimeout {
		t.Errorf("Expected status %v, got %v", StageExecutionTimeout, result.Status)
	}

	if result.Duration >= 5*time.Second {
		t.Errorf("Expected duration < 5s, got %v", result.Duration)
	}
}

// TestTimeoutExecutorCommandWithOutput tests multiline output capture.
func TestTimeoutExecutorCommandWithOutput(t *testing.T) {
	executor := NewTimeoutExecutor(5 * time.Second)

	stage := &Stage{
		ID:      "multiline-stage",
		Name:    "Multiline Stage",
		Backend: "test",
		Command: "printf 'line1\\nline2\\nline3\\n'",
	}

	result, err := executor.ExecuteStageWithTimeout(stage)

	if err != nil {
		t.Fatalf("ExecuteStageWithTimeout returned error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(result.Output), "\n")
	if len(lines) != 3 {
		t.Errorf("Expected 3 lines, got %d", len(lines))
	}

	if lines[0] != "line1" || lines[1] != "line2" || lines[2] != "line3" {
		t.Errorf("Lines do not match expected output: %v", lines)
	}
}

// TestStageResultErrorString tests that error is properly set in result.
func TestStageResultErrorString(t *testing.T) {
	executor := NewTimeoutExecutor(1 * time.Second)

	stage := &Stage{
		ID:      "error-stage",
		Name:    "Error Stage",
		Backend: "test",
		Command: "sleep 10",
		Timeout: &Duration{Duration: 500 * time.Millisecond},
	}

	result, err := executor.ExecuteStageWithTimeout(stage)

	if err != nil {
		t.Fatalf("ExecuteStageWithTimeout returned error: %v", err)
	}

	if result.Error == nil {
		t.Errorf("Expected error to be set for timeout")
	}

	errorMsg := fmt.Sprintf("%v", result.Error)
	if !strings.Contains(errorMsg, "timed out") {
		t.Errorf("Expected 'timed out' in error message, got: %v", errorMsg)
	}
}
