package executor

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"flip2/internal/config"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// Executor manages task execution via agent spawning
type Executor struct {
	pb        *pocketbase.PocketBase
	config    *config.Config
	executing map[string]bool // Set of task IDs currently executing
	runningProcs map[string]*exec.Cmd // Map of task ID to running command
	mu        sync.Mutex
	logger    *slog.Logger
	semaphore chan struct{} // Limit concurrent tasks
}

// New creates a new Executor
func New(pb *pocketbase.PocketBase, cfg *config.Config, logger *slog.Logger) *Executor {
	// Use the provided logger, and add an "executor" group
	executorLogger := logger.WithGroup("executor")

	maxTasks := cfg.Flip2.Executor.MaxConcurrentTasks
	if maxTasks <= 0 {
		maxTasks = 3
	}

	return &Executor{
		pb:        pb,
		config:    cfg,
		executing: make(map[string]bool),
		runningProcs: make(map[string]*exec.Cmd),
		logger:    executorLogger,
		semaphore: make(chan struct{}, maxTasks),
	}
}

// Start listens for tasks (usually via hooks, but here we might just set up initial state)
// In this architecture, actual execution is triggered by calls to QueueTask from hooks in daemon.
func (e *Executor) Start() {
	e.logger.Info("Executor started", "max_concurrent", cap(e.semaphore))
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
	// Acquire semaphore
	e.semaphore <- struct{}{}
	defer func() {
		<-e.semaphore
		e.mu.Lock()
		delete(e.executing, taskID)
		e.mu.Unlock()
	}()

	e.logger.Info("Processing task", "task_id", taskID)

	// Atomic Claim
	var task *core.Record
	
	// Attempt to claim the task
	err := e.pb.RunInTransaction(func(txApp core.App) error {
		t, err := txApp.FindRecordById("tasks", taskID)
		if err != nil {
			return err
		}

		status := t.GetString("status")
		if status != "pending" && status != "retry_scheduled" {
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
		e.logger.Info("Failed to claim task", "task_id", taskID, "reason", err)
		return
	}

	assigneeID := task.GetString("assignee")
	if assigneeID == "" {
		e.logger.Error("Task has no assignee", "task_id", taskID)
		e.failTask(task, "No assignee", "", "")
		return
	}

	agent, err := e.pb.FindRecordById("agents", assigneeID)
	if err != nil {
		e.logger.Error("Failed to find agent", "agent_id", assigneeID, "error", err)
		e.failTask(task, fmt.Sprintf("Agent not found: %v", err), "", "")
		return
	}

	backendName := agent.GetString("backend")
	backendConfig, ok := e.config.Flip2.Backends[backendName]
	if !ok {
		// Try generic default or error?
		// For now error
		e.logger.Error("Unknown backend", "backend", backendName)
		e.failTask(task, fmt.Sprintf("Unknown backend: %s", backendName), "", "")
		return
	}

	// Prepare execution
	ctx, cancel := context.WithTimeout(context.Background(), e.config.Flip2.Executor.DefaultTimeout)
	defer cancel()
	
	var stdout, stderr string
	var execErr error

	if backendConfig.Type == "http" {
		// For now, HTTP execution doesn't return separate stdout/stderr,
		// so we'll treat the single output as combined.
		var output string
		output, execErr = e.executeHTTP(ctx, backendConfig, task)
		stdout = output // Assuming HTTP output is primarily "stdout" like
		stderr = ""     // No separate stderr for HTTP currently
	} else {
		stdout, stderr, execErr = e.executeProcess(ctx, backendConfig, task)
	}

	if execErr != nil {
		e.logger.Error("Task execution failed", "task_id", taskID, "error", execErr)
		e.failTask(task, execErr.Error(), stdout, stderr)
	} else {
		e.logger.Info("Task completed", "task_id", taskID)
		e.completeTask(task, stdout, stderr)
	}
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
		return "", "", err
	}

	err := cmd.Wait()

	if err != nil {
		return stdout.String(), stderr.String(), err
	}
	return stdout.String(), stderr.String(), nil
}

func (e *Executor) executeHTTP(ctx context.Context, cfg config.BackendConfig, task *core.Record) (string, error) {
	// TODO: Implement HTTP client for antigravity/remote agents
	return "HTTP backend not implemented yet", nil
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

func (e *Executor) failTask(task *core.Record, errorMessage, stdoutLog, stderrLog string) {
	// Retry logic
	retries := task.GetInt("retry_count")
	maxRetries := task.GetInt("max_retries") // Ensure this field exists in schema or default to 0

	if retries < maxRetries {
		e.logger.Info("Retrying task", "task_id", task.Id, "attempt", retries+1)
		task.Set("retry_count", retries+1)
		task.Set("status", "pending") // Send back to queue
		task.Set("last_error", errorMessage)
	} else {
		task.Set("status", "failed")
		task.Set("result", errorMessage) // Store just the error message in 'result' for failed tasks
		task.Set("completed_at", time.Now())
	}
	
	task.Set("stdout_log", stdoutLog)
	task.Set("stderr_log", stderrLog)
	
	if err := e.pb.Save(task); err != nil {
		e.logger.Error("Failed to update task status", "error", err)
	}
}

func (e *Executor) completeTask(task *core.Record, stdoutLog, stderrLog string) {
	task.Set("status", "done")
	task.Set("result", "Task completed successfully.") // A generic success message
	task.Set("stdout_log", stdoutLog)
	task.Set("stderr_log", stderrLog)
	task.Set("completed_at", time.Now())
	
	if err := e.pb.Save(task); err != nil {
		e.logger.Error("Failed to update task to done", "error", err)
	}
}

// SignalTask sends a signal to a running task
func (e *Executor) SignalTask(taskID string, signal os.Signal) error {
	e.mu.Lock()
	cmd, ok := e.runningProcs[taskID]
	e.mu.Unlock()

	if !ok {
		return fmt.Errorf("task %s is not running (locally)", taskID)
	}

	if cmd.Process == nil {
		return fmt.Errorf("process not started yet")
	}

	e.logger.Info("Signaling task", "task_id", taskID, "signal", signal)
	return cmd.Process.Signal(signal)
}
