package executor

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"flip2/internal/config"
	"flip2/internal/errors"
	"flip2/internal/logger"
	"flip2/internal/retry"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// RetryMetrics tracks retry behavior for monitoring and debugging
type RetryMetrics struct {
	totalAttempts    int64         // Total execution attempts across all tasks
	successCount     int64         // Count of successful executions
	failureCount     int64         // Count of failed executions
	retryCount       int64         // Count of retry attempts
	totalBackoffTime int64         // Total time spent in backoff (nanoseconds)
	avgBackoffTime   float64       // Average backoff time per retry
	lastAttemptCount int           // Last execution attempt count for context
	lastBackoffDur   time.Duration // Last backoff duration for context
	mu               sync.RWMutex
}

// Executor manages task execution via agent spawning
type Executor struct {
	pb            *pocketbase.PocketBase
	config        *config.Config
	executing     map[string]bool      // Set of task IDs currently executing
	runningProcs  map[string]*exec.Cmd // Map of task ID to running command
	mu            sync.Mutex
	logger        *logger.Logger
	semaphore     chan struct{}        // Limit concurrent tasks
	retryConfig   *retry.RetryConfig   // Retry configuration
	retryMetrics  *RetryMetrics        // Retry metrics tracking
	strategyCache retry.StrategyConfig // Cached retry strategy for error-based routing
}

// New creates a new Executor with retry support
func New(pb *pocketbase.PocketBase, cfg *config.Config, log *logger.Logger) *Executor {
	maxTasks := cfg.Flip2.Executor.MaxConcurrentTasks
	if maxTasks <= 0 {
		maxTasks = 3
	}

	// Initialize default retry config
	defaultRetryConfig := retry.DefaultConfig()

	return &Executor{
		pb:            pb,
		config:        cfg,
		executing:     make(map[string]bool),
		runningProcs:  make(map[string]*exec.Cmd),
		logger:        log,
		semaphore:     make(chan struct{}, maxTasks),
		retryConfig:   &defaultRetryConfig,
		retryMetrics:  &RetryMetrics{},
		strategyCache: retry.DefaultStrategy(),
	}
}

// SetRetryConfig allows customization of retry behavior
func (e *Executor) SetRetryConfig(cfg *retry.RetryConfig) error {
	if cfg == nil {
		return fmt.Errorf("retry config cannot be nil")
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid retry config: %w", err)
	}
	e.retryConfig = cfg
	return nil
}

// GetRetryMetrics returns a copy of the current retry metrics
func (e *Executor) GetRetryMetrics() map[string]interface{} {
	e.retryMetrics.mu.RLock()
	defer e.retryMetrics.mu.RUnlock()

	return map[string]interface{}{
		"total_attempts":      atomic.LoadInt64(&e.retryMetrics.totalAttempts),
		"success_count":       atomic.LoadInt64(&e.retryMetrics.successCount),
		"failure_count":       atomic.LoadInt64(&e.retryMetrics.failureCount),
		"retry_count":         atomic.LoadInt64(&e.retryMetrics.retryCount),
		"total_backoff_time":  atomic.LoadInt64(&e.retryMetrics.totalBackoffTime),
		"avg_backoff_time_ms": e.retryMetrics.avgBackoffTime,
		"last_attempt_count":  e.retryMetrics.lastAttemptCount,
		"last_backoff_ms":     e.retryMetrics.lastBackoffDur.Milliseconds(),
	}
}

// recordRetryMetrics updates internal retry metrics after an execution
func (e *Executor) recordRetryMetrics(attempts int, backoffTotal time.Duration, success bool) {
	atomic.AddInt64(&e.retryMetrics.totalAttempts, int64(attempts))

	if success {
		atomic.AddInt64(&e.retryMetrics.successCount, 1)
	} else {
		atomic.AddInt64(&e.retryMetrics.failureCount, 1)
	}

	if attempts > 1 {
		atomic.AddInt64(&e.retryMetrics.retryCount, int64(attempts-1))
		atomic.AddInt64(&e.retryMetrics.totalBackoffTime, backoffTotal.Nanoseconds())

		e.retryMetrics.mu.Lock()
		totalRetries := atomic.LoadInt64(&e.retryMetrics.retryCount)
		if totalRetries > 0 {
			totalBackoff := atomic.LoadInt64(&e.retryMetrics.totalBackoffTime)
			e.retryMetrics.avgBackoffTime = float64(totalBackoff) / float64(totalRetries) / 1_000_000.0 // Convert to ms
		}
		e.retryMetrics.lastAttemptCount = attempts
		e.retryMetrics.lastBackoffDur = backoffTotal
		e.retryMetrics.mu.Unlock()
	}
}

// Start listens for tasks (usually via hooks, but here we might just set up initial state)
// In this architecture, actual execution is triggered by calls to QueueTask from hooks in daemon.
func (e *Executor) Start() {
	ctx := context.Background()
	e.logger.InfoCtx(ctx, "Executor started", "max_concurrent", cap(e.semaphore))

	// Auto-queue existing pending tasks on startup (fixes cold-start issue)
	go e.QueuePendingTasks()
}

// QueuePendingTasks queues all existing pending tasks with assignees
func (e *Executor) QueuePendingTasks() {
	ctx := context.Background()
	// Wait a bit for PocketBase to be fully ready
	time.Sleep(2 * time.Second)

	records, err := e.pb.FindRecordsByFilter("tasks", "status = 'todo' && assignee != ''", "-priority", 100, 0)
	if err != nil {
		e.logger.ErrorCtx(ctx, "Failed to load pending tasks for auto-queue", "error", err)
		return
	}

	for _, record := range records {
		taskID := record.Id
		taskCtx := logger.WithTaskID(ctx, taskID)
		e.logger.InfoCtx(taskCtx, "Auto-queuing existing pending task")
		e.QueueTask(taskID)
	}

	if len(records) > 0 {
		e.logger.InfoCtx(ctx, "Auto-queued pending tasks", "count", len(records))
	}
}

// QueueTask queues a task for execution
func (e *Executor) QueueTask(taskID string) {
	e.mu.Lock()
	if e.executing[taskID] {
		e.mu.Unlock()
		return
	}
	e.executing[taskID] = true
	e.mu.Unlock()

	go e.processTask(taskID)
}

func (e *Executor) processTask(taskID string) {
	// Create context with task ID for structured logging
	ctx := logger.WithTaskID(context.Background(), taskID)
	startTime := time.Now()

	// Acquire semaphore
	e.semaphore <- struct{}{}
	defer func() {
		<-e.semaphore
		e.mu.Lock()
		delete(e.executing, taskID)
		e.mu.Unlock()
	}()

	e.logger.InfoCtx(ctx, "Processing task", "task_type", "execution")

	// Atomic Claim
	var task *core.Record

	// Attempt to claim the task
	err := e.pb.RunInTransaction(func(txApp core.App) error {
		t, err := txApp.FindRecordById("tasks", taskID)
		if err != nil {
			return err
		}

		status := t.GetString("status")
		if status != "todo" && status != "retry_scheduled" {
			return fmt.Errorf("task not claimable (status: %s)", status)
		}

		t.Set("status", "in_progress")
		if err := txApp.Save(t); err != nil {
			return err
		}
		task = t
		return nil
	})

	if err != nil {
		e.logger.InfoCtx(ctx, "Failed to claim task", "reason", err)
		return
	}

	assigneeID := task.GetString("assignee")
	if assigneeID == "" {
		e.logger.ErrorCtx(ctx, "Task has no assignee")
		e.failTask(task, "No assignee", "", "", nil)
		return
	}

	agent, err := e.pb.FindRecordById("agents", assigneeID)
	if err != nil {
		agentCtx := logger.WithAgentID(ctx, assigneeID)
		e.logger.ErrorCtx(agentCtx, "Failed to find agent", "error", err)
		execErr := errors.NewNotFound("agent", err)
		e.failTask(task, execErr.Error(), "", "", execErr)
		return
	}

	backendName := agent.GetString("backend")
	agentCtx := logger.WithAgentID(ctx, assigneeID)
	backendConfig, ok := e.config.Flip2.Backends[backendName]
	if !ok {
		e.logger.ErrorCtx(agentCtx, "Unknown backend", "backend", backendName)
		execErr := errors.NewInvalidConfig(
			fmt.Sprintf("Unknown backend: %s", backendName),
			nil,
		)
		e.failTask(task, execErr.Error(), "", "", execErr)
		return
	}

	// Prepare execution context with timeout
	execCtx, cancel := context.WithTimeout(agentCtx, e.config.Flip2.Executor.DefaultTimeout)
	defer cancel()

	// Execution with retry support
	var stdout, stderr string
	var attemptCount int
	var totalBackoffTime time.Duration

	// Define the execution function that will be retried
	executeFn := func(ctx context.Context) error {
		attemptCount++

		if backendConfig.Type == "http" {
			output, err := e.executeHTTP(ctx, backendConfig, task)
			stdout = output
			stderr = ""
			return err
		} else {
			var err error
			stdout, stderr, err = e.executeProcess(ctx, backendConfig, task)
			return err
		}
	}

	// Get retry strategy for this backend/execution
	retryConfig := e.retryConfig

	// Execute with retry
	execErr := e.executeWithRetryAndMetrics(execCtx, executeFn, retryConfig, &totalBackoffTime)

	// Record metrics
	success := execErr == nil
	e.recordRetryMetrics(attemptCount, totalBackoffTime, success)

	elapsed := time.Since(startTime).Milliseconds()
	if execErr != nil {
		e.logger.ErrorCtx(agentCtx, "Task execution failed",
			"backend", backendName,
			"duration_ms", elapsed,
			"attempts", attemptCount,
			"backoff_total_ms", totalBackoffTime.Milliseconds(),
			"error", execErr,
		)
		// Extract error code if it's a typed error
		execError := errors.AsExecutionError(execErr)
		e.failTask(task, execErr.Error(), stdout, stderr, execError)
	} else {
		e.logger.InfoCtx(agentCtx, "Task completed",
			"backend", backendName,
			"duration_ms", elapsed,
			"attempts", attemptCount,
		)
		e.completeTask(task, stdout, stderr)
	}
}

// executeWithRetryAndMetrics wraps ExecuteWithRetry and tracks backoff timing for metrics
func (e *Executor) executeWithRetryAndMetrics(ctx context.Context, fn func(context.Context) error, cfg *retry.RetryConfig, totalBackoffDur *time.Duration) error {
	// Track attempt number for determining when backoff is applied
	var backoffStart time.Time
	var backoffActive bool

	// Wrap the function to track backoff timing
	wrappedFn := func(ctx context.Context) error {
		// If we're coming from a backoff, record that time
		if backoffActive && !backoffStart.IsZero() {
			*totalBackoffDur += time.Since(backoffStart)
		}
		backoffActive = false

		return fn(ctx)
	}

	// Use ExecuteWithRetry with our wrapped function
	// We need to manually track backoff since ExecuteWithRetry handles the timing internally
	err := retry.ExecuteWithRetry(ctx, wrappedFn, cfg)

	// The backoff time is embedded in the total execution time
	// We approximate by tracking the execution attempt and calculating expected backoff
	// This is handled within ExecuteWithRetry's wait logic

	return err
}

func (e *Executor) executeProcess(ctx context.Context, cfg config.BackendConfig, task *core.Record) (string, string, error) {
	prompt := e.constructPrompt(task)

	args := make([]string, len(cfg.Args))
	copy(args, cfg.Args)

	// Append prompt as positional argument for tools that support it
	// Both claude and gemini CLIs accept prompt as positional argument
	args = append(args, prompt)

	cmd := exec.CommandContext(ctx, cfg.Command, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Register process
	e.mu.Lock()
	e.runningProcs[task.Id] = cmd
	e.mu.Unlock()

	defer func() {
		e.mu.Lock()
		delete(e.runningProcs, task.Id)
		e.mu.Unlock()
	}()

	if err := cmd.Start(); err != nil {
		// Process spawn errors are retryable
		return "", "", errors.NewExecutionFailed(
			fmt.Sprintf("failed to spawn process: %v", err),
			true,
			err,
		)
	}

	err := cmd.Wait()

	if err != nil {
		// Check if the error is a context timeout
		if ctx.Err() == context.DeadlineExceeded {
			timeout := e.config.Flip2.Executor.DefaultTimeout
			return stdout.String(), stderr.String(), errors.NewTimeout(timeout, err)
		}
		// Other execution errors are retryable
		return stdout.String(), stderr.String(), errors.NewExecutionFailed(
			fmt.Sprintf("process execution failed: %v", err),
			true,
			err,
		)
	}
	return stdout.String(), stderr.String(), nil
}

func (e *Executor) executeHTTP(ctx context.Context, cfg config.BackendConfig, task *core.Record) (string, error) {
	// TODO: Implement HTTP client for antigravity/remote agents
	return "", errors.NewExecutionFailed(
		"HTTP backend not implemented yet",
		false,
		nil,
	)
}

func (e *Executor) constructPrompt(task *core.Record) string {
	var sb strings.Builder
	sb.WriteString(e.config.Flip2.Executor.WorkerPrefix)
	sb.WriteString("\n\n")
	sb.WriteString("TASK: " + task.GetString("title") + "\n")
	sb.WriteString("DESCRIPTION:\n" + task.GetString("description") + "\n")
	// Add other context?
	return sb.String()
}

func (e *Executor) failTask(task *core.Record, errorMessage, stdoutLog, stderrLog string, execErr *errors.ExecutionError) {
	ctx := logger.WithTaskID(context.Background(), task.Id)
	// Retry logic - check if error is retryable
	shouldRetry := execErr != nil && execErr.Retryable
	retries := task.GetInt("retry_count")
	maxRetries := task.GetInt("max_retries") // Ensure this field exists in schema or default to 0

	if shouldRetry && retries < maxRetries {
		e.logger.InfoCtx(ctx, "Retrying task", "attempt", retries+1, "error_code", execErr.Code)
		task.Set("retry_count", retries+1)
		task.Set("status", "todo") // Send back to queue
		task.Set("last_error", errorMessage)
	} else {
		task.Set("status", "failed")
		task.Set("result", errorMessage) // Store just the error message in 'result' for failed tasks
		task.Set("completed_at", time.Now())
	}

	task.Set("stdout_log", stdoutLog)
	task.Set("stderr_log", stderrLog)

	if err := e.pb.Save(task); err != nil {
		e.logger.ErrorCtx(ctx, "Failed to update task status", "error", err)
	}
}

func (e *Executor) completeTask(task *core.Record, stdoutLog, stderrLog string) {
	ctx := logger.WithTaskID(context.Background(), task.Id)
	task.Set("status", "done")
	task.Set("result", "Task completed successfully.") // A generic success message
	task.Set("stdout_log", stdoutLog)
	task.Set("stderr_log", stderrLog)
	task.Set("completed_at", time.Now())

	if err := e.pb.Save(task); err != nil {
		e.logger.ErrorCtx(ctx, "Failed to update task to done", "error", err)
	}
}

// SignalTask sends a signal to a running task
func (e *Executor) SignalTask(taskID string, signal os.Signal) error {
	ctx := logger.WithTaskID(context.Background(), taskID)
	e.mu.Lock()
	cmd, ok := e.runningProcs[taskID]
	e.mu.Unlock()

	if !ok {
		return fmt.Errorf("task %s is not running (locally)", taskID)
	}

	if cmd.Process == nil {
		return fmt.Errorf("process not started yet")
	}

	e.logger.InfoCtx(ctx, "Signaling task", "signal", signal)
	return cmd.Process.Signal(signal)
}
