// Package queue provides a distributed task queue with priority scheduling.
//
// Ported from FLIP v1 pkg/cluster/dispatch.go with adaptations for FLIP2:
// - PocketBase integration for persistence
// - Integration with internal/llm backends
// - Simplified node selection (single-node focus initially)
package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// TaskStatus represents the execution status.
type TaskStatus string

const (
	TaskPending   TaskStatus = "pending"
	TaskRunning   TaskStatus = "running"
	TaskCompleted TaskStatus = "completed"
	TaskFailed    TaskStatus = "failed"
	TaskCancelled TaskStatus = "cancelled"
)

// Task represents a task in the queue.
type Task struct {
	ID          string            `json:"id"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Backend     string            `json:"backend"`
	Model       string            `json:"model,omitempty"`
	Priority    int               `json:"priority"` // Higher = more urgent
	Status      TaskStatus        `json:"status"`
	Assignee    string            `json:"assignee,omitempty"`
	Result      string            `json:"result,omitempty"`
	Error       string            `json:"error,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	StartedAt   *time.Time        `json:"started_at,omitempty"`
	CompletedAt *time.Time        `json:"completed_at,omitempty"`
	RetryCount  int               `json:"retry_count"`
	MaxRetries  int               `json:"max_retries"`
}

// TaskResult contains the result of a task execution.
type TaskResult struct {
	TaskID    string        `json:"task_id"`
	Success   bool          `json:"success"`
	Output    string        `json:"output"`
	Error     string        `json:"error,omitempty"`
	Duration  time.Duration `json:"duration"`
	Timestamp time.Time     `json:"timestamp"`
}

// Queue manages task scheduling and execution.
type Queue struct {
	mu     sync.RWMutex
	pb     *pocketbase.PocketBase
	logger *slog.Logger

	// In-memory priority queue (synced with PocketBase)
	pending   []*Task
	running   map[string]*Task
	completed map[string]*TaskResult

	// Configuration
	maxQueueSize    int
	maxConcurrent   int
	defaultTimeout  time.Duration
	defaultRetries  int
	cleanupInterval time.Duration

	// Callbacks
	onTaskStart    func(task *Task)
	onTaskComplete func(task *Task, result *TaskResult)
	onTaskFailed   func(task *Task, err error)

	// Control
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// QueueConfig holds configuration for the Queue.
type QueueConfig struct {
	PocketBase      *pocketbase.PocketBase
	Logger          *slog.Logger
	MaxQueueSize    int
	MaxConcurrent   int
	DefaultTimeout  time.Duration
	DefaultRetries  int
	CleanupInterval time.Duration
}

// NewQueue creates a new task queue.
func NewQueue(cfg QueueConfig) *Queue {
	ctx, cancel := context.WithCancel(context.Background())

	maxQueueSize := cfg.MaxQueueSize
	if maxQueueSize == 0 {
		maxQueueSize = 1000
	}

	maxConcurrent := cfg.MaxConcurrent
	if maxConcurrent == 0 {
		maxConcurrent = 3
	}

	defaultTimeout := cfg.DefaultTimeout
	if defaultTimeout == 0 {
		defaultTimeout = 5 * time.Minute
	}

	cleanupInterval := cfg.CleanupInterval
	if cleanupInterval == 0 {
		cleanupInterval = 1 * time.Hour
	}

	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &Queue{
		pb:              cfg.PocketBase,
		logger:          logger.WithGroup("queue"),
		pending:         make([]*Task, 0),
		running:         make(map[string]*Task),
		completed:       make(map[string]*TaskResult),
		maxQueueSize:    maxQueueSize,
		maxConcurrent:   maxConcurrent,
		defaultTimeout:  defaultTimeout,
		defaultRetries:  cfg.DefaultRetries,
		cleanupInterval: cleanupInterval,
		ctx:             ctx,
		cancel:          cancel,
	}
}

// Start begins queue processing.
func (q *Queue) Start() {
	q.logger.Info("Queue started",
		"max_queue_size", q.maxQueueSize,
		"max_concurrent", q.maxConcurrent)

	// Load pending tasks from PocketBase
	q.loadPendingTasks()

	// Start cleanup routine
	q.wg.Add(1)
	go q.cleanupLoop()
}

// Stop gracefully shuts down the queue.
func (q *Queue) Stop() {
	q.logger.Info("Queue stopping...")
	q.cancel()
	q.wg.Wait()
	q.logger.Info("Queue stopped")
}

// Submit adds a task to the queue.
func (q *Queue) Submit(task *Task) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.pending) >= q.maxQueueSize {
		return fmt.Errorf("queue full (max %d)", q.maxQueueSize)
	}

	// Set defaults
	task.Status = TaskPending
	task.CreatedAt = time.Now()
	if task.MaxRetries == 0 {
		task.MaxRetries = q.defaultRetries
	}

	// Persist to PocketBase
	if err := q.persistTask(task); err != nil {
		return fmt.Errorf("failed to persist task: %w", err)
	}

	// Insert by priority (higher priority first)
	q.insertByPriority(task)

	q.logger.Info("Task submitted",
		"task_id", task.ID,
		"priority", task.Priority,
		"queue_size", len(q.pending))

	return nil
}

// insertByPriority inserts a task maintaining priority order.
func (q *Queue) insertByPriority(task *Task) {
	inserted := false
	for i, t := range q.pending {
		if task.Priority > t.Priority {
			// Insert at position i
			q.pending = append(q.pending[:i], append([]*Task{task}, q.pending[i:]...)...)
			inserted = true
			break
		}
	}
	if !inserted {
		q.pending = append(q.pending, task)
	}
}

// Dequeue removes and returns the next task to process.
func (q *Queue) Dequeue() *Task {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Check concurrent limit
	if len(q.running) >= q.maxConcurrent {
		return nil
	}

	// No pending tasks
	if len(q.pending) == 0 {
		return nil
	}

	// Get highest priority task
	task := q.pending[0]
	q.pending = q.pending[1:]

	// Mark as running
	task.Status = TaskRunning
	now := time.Now()
	task.StartedAt = &now
	q.running[task.ID] = task

	// Update in PocketBase
	q.updateTaskStatus(task)

	if q.onTaskStart != nil {
		q.onTaskStart(task)
	}

	q.logger.Info("Task dequeued",
		"task_id", task.ID,
		"running", len(q.running),
		"pending", len(q.pending))

	return task
}

// Complete marks a task as completed.
func (q *Queue) Complete(taskID string, output string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	task, ok := q.running[taskID]
	if !ok {
		return fmt.Errorf("task %s not running", taskID)
	}

	now := time.Now()
	task.Status = TaskCompleted
	task.Result = output
	task.CompletedAt = &now

	// Create result
	result := &TaskResult{
		TaskID:    taskID,
		Success:   true,
		Output:    output,
		Duration:  now.Sub(*task.StartedAt),
		Timestamp: now,
	}

	// Move from running to completed
	delete(q.running, taskID)
	q.completed[taskID] = result

	// Update in PocketBase
	q.updateTaskStatus(task)

	if q.onTaskComplete != nil {
		q.onTaskComplete(task, result)
	}

	q.logger.Info("Task completed",
		"task_id", taskID,
		"duration", result.Duration)

	return nil
}

// Fail marks a task as failed, optionally retrying.
func (q *Queue) Fail(taskID string, errMsg string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	task, ok := q.running[taskID]
	if !ok {
		return fmt.Errorf("task %s not running", taskID)
	}

	delete(q.running, taskID)

	// Check if we should retry
	if task.RetryCount < task.MaxRetries {
		task.RetryCount++
		task.Status = TaskPending
		task.Error = errMsg

		// Re-queue with same priority
		q.insertByPriority(task)
		q.updateTaskStatus(task)

		q.logger.Warn("Task failed, retrying",
			"task_id", taskID,
			"retry", task.RetryCount,
			"max_retries", task.MaxRetries,
			"error", errMsg)

		return nil
	}

	// Max retries exceeded
	now := time.Now()
	task.Status = TaskFailed
	task.Error = errMsg
	task.CompletedAt = &now

	result := &TaskResult{
		TaskID:    taskID,
		Success:   false,
		Error:     errMsg,
		Duration:  now.Sub(*task.StartedAt),
		Timestamp: now,
	}

	q.completed[taskID] = result
	q.updateTaskStatus(task)

	if q.onTaskFailed != nil {
		q.onTaskFailed(task, fmt.Errorf("%s", errMsg))
	}

	q.logger.Error("Task failed permanently",
		"task_id", taskID,
		"retries", task.RetryCount,
		"error", errMsg)

	return nil
}

// Cancel cancels a pending or running task.
func (q *Queue) Cancel(taskID string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Check running
	if task, ok := q.running[taskID]; ok {
		delete(q.running, taskID)
		task.Status = TaskCancelled
		now := time.Now()
		task.CompletedAt = &now
		q.updateTaskStatus(task)

		q.logger.Info("Running task cancelled", "task_id", taskID)
		return nil
	}

	// Check pending
	for i, task := range q.pending {
		if task.ID == taskID {
			q.pending = append(q.pending[:i], q.pending[i+1:]...)
			task.Status = TaskCancelled
			now := time.Now()
			task.CompletedAt = &now
			q.updateTaskStatus(task)

			q.logger.Info("Pending task cancelled", "task_id", taskID)
			return nil
		}
	}

	return fmt.Errorf("task %s not found", taskID)
}

// GetTask returns a task by ID.
func (q *Queue) GetTask(taskID string) (*Task, *TaskResult, bool) {
	q.mu.RLock()
	defer q.mu.RUnlock()

	// Check running
	if task, ok := q.running[taskID]; ok {
		return task, nil, true
	}

	// Check completed
	if result, ok := q.completed[taskID]; ok {
		return nil, result, true
	}

	// Check pending
	for _, task := range q.pending {
		if task.ID == taskID {
			return task, nil, true
		}
	}

	return nil, nil, false
}

// GetStats returns queue statistics.
func (q *Queue) GetStats() map[string]interface{} {
	q.mu.RLock()
	defer q.mu.RUnlock()

	return map[string]interface{}{
		"pending":        len(q.pending),
		"running":        len(q.running),
		"completed":      len(q.completed),
		"max_queue_size": q.maxQueueSize,
		"max_concurrent": q.maxConcurrent,
	}
}

// ListPending returns all pending tasks.
func (q *Queue) ListPending() []*Task {
	q.mu.RLock()
	defer q.mu.RUnlock()

	result := make([]*Task, len(q.pending))
	copy(result, q.pending)
	return result
}

// ListRunning returns all running tasks.
func (q *Queue) ListRunning() []*Task {
	q.mu.RLock()
	defer q.mu.RUnlock()

	result := make([]*Task, 0, len(q.running))
	for _, task := range q.running {
		result = append(result, task)
	}
	return result
}

// OnTaskStart sets callback for when a task starts.
func (q *Queue) OnTaskStart(fn func(task *Task)) {
	q.onTaskStart = fn
}

// OnTaskComplete sets callback for completed tasks.
func (q *Queue) OnTaskComplete(fn func(task *Task, result *TaskResult)) {
	q.onTaskComplete = fn
}

// OnTaskFailed sets callback for failed tasks.
func (q *Queue) OnTaskFailed(fn func(task *Task, err error)) {
	q.onTaskFailed = fn
}

// PocketBase integration methods

// persistTask saves a new task to PocketBase.
func (q *Queue) persistTask(task *Task) error {
	if q.pb == nil {
		return nil // No persistence configured
	}

	collection, err := q.pb.FindCollectionByNameOrId("tasks")
	if err != nil {
		return err
	}

	record := core.NewRecord(collection)
	record.Set("task_id", task.ID)
	record.Set("title", task.Title)
	record.Set("description", task.Description)
	record.Set("status", string(task.Status))
	record.Set("priority", task.Priority)
	record.Set("assignee", task.Assignee)

	if task.Metadata != nil {
		metaJSON, _ := json.Marshal(task.Metadata)
		record.Set("metadata", string(metaJSON))
	}

	return q.pb.Save(record)
}

// updateTaskStatus updates a task in PocketBase.
func (q *Queue) updateTaskStatus(task *Task) error {
	if q.pb == nil {
		return nil
	}

	// Find existing record by task_id
	records, err := q.pb.FindRecordsByFilter("tasks", fmt.Sprintf("task_id = '%s'", task.ID), "", 1, 0)
	if err != nil || len(records) == 0 {
		return err
	}

	record := records[0]
	record.Set("status", string(task.Status))
	record.Set("result", task.Result)

	if task.StartedAt != nil {
		record.Set("started_at", task.StartedAt)
	}
	if task.CompletedAt != nil {
		record.Set("completed_at", task.CompletedAt)
	}

	return q.pb.Save(record)
}

// loadPendingTasks loads pending tasks from PocketBase on startup.
func (q *Queue) loadPendingTasks() {
	if q.pb == nil {
		return
	}

	records, err := q.pb.FindRecordsByFilter("tasks", "status = 'pending'", "-priority", 100, 0)
	if err != nil {
		q.logger.Error("Failed to load pending tasks", "error", err)
		return
	}

	q.mu.Lock()
	defer q.mu.Unlock()

	for _, record := range records {
		task := &Task{
			ID:          record.GetString("task_id"),
			Title:       record.GetString("title"),
			Description: record.GetString("description"),
			Status:      TaskPending,
			Priority:    record.GetInt("priority"),
			Assignee:    record.GetString("assignee"),
		}
		q.pending = append(q.pending, task)
	}

	q.logger.Info("Loaded pending tasks from PocketBase", "count", len(records))
}

// cleanupLoop periodically cleans up old completed tasks.
func (q *Queue) cleanupLoop() {
	defer q.wg.Done()

	ticker := time.NewTicker(q.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-q.ctx.Done():
			return
		case <-ticker.C:
			q.cleanup()
		}
	}
}

// cleanup removes old completed task results from memory.
func (q *Queue) cleanup() {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Keep only last 1000 completed results in memory
	if len(q.completed) > 1000 {
		// Simple cleanup - just clear oldest entries
		// In production, would sort by timestamp
		count := len(q.completed) - 1000
		cleared := 0
		for id := range q.completed {
			if cleared >= count {
				break
			}
			delete(q.completed, id)
			cleared++
		}
		q.logger.Info("Cleaned up completed tasks", "cleared", cleared)
	}
}
