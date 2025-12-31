package queue

import (
	"testing"
	"time"
)

func TestNewQueue(t *testing.T) {
	q := NewQueue(QueueConfig{
		MaxQueueSize:   100,
		MaxConcurrent:  5,
		DefaultTimeout: 1 * time.Minute,
	})

	if q == nil {
		t.Fatal("expected non-nil queue")
	}

	stats := q.GetStats()
	if stats["max_queue_size"] != 100 {
		t.Errorf("expected max_queue_size 100, got %v", stats["max_queue_size"])
	}
	if stats["max_concurrent"] != 5 {
		t.Errorf("expected max_concurrent 5, got %v", stats["max_concurrent"])
	}
}

func TestQueue_Submit(t *testing.T) {
	q := NewQueue(QueueConfig{MaxQueueSize: 10})

	task := &Task{
		ID:       "test-task-1",
		Title:    "Test Task",
		Priority: 5,
	}

	err := q.Submit(task)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stats := q.GetStats()
	if stats["pending"].(int) != 1 {
		t.Errorf("expected 1 pending task, got %v", stats["pending"])
	}
}

func TestQueue_Submit_QueueFull(t *testing.T) {
	q := NewQueue(QueueConfig{MaxQueueSize: 2})

	// Fill the queue
	q.Submit(&Task{ID: "t1", Priority: 1})
	q.Submit(&Task{ID: "t2", Priority: 1})

	// This should fail
	err := q.Submit(&Task{ID: "t3", Priority: 1})
	if err == nil {
		t.Error("expected error when queue is full")
	}
}

func TestQueue_PriorityOrdering(t *testing.T) {
	q := NewQueue(QueueConfig{MaxQueueSize: 10, MaxConcurrent: 10})

	// Submit in non-priority order
	q.Submit(&Task{ID: "low", Title: "Low Priority", Priority: 1})
	q.Submit(&Task{ID: "high", Title: "High Priority", Priority: 10})
	q.Submit(&Task{ID: "medium", Title: "Medium Priority", Priority: 5})

	// Dequeue should return highest priority first
	task := q.Dequeue()
	if task.ID != "high" {
		t.Errorf("expected high priority task first, got %s", task.ID)
	}

	task = q.Dequeue()
	if task.ID != "medium" {
		t.Errorf("expected medium priority task second, got %s", task.ID)
	}

	task = q.Dequeue()
	if task.ID != "low" {
		t.Errorf("expected low priority task third, got %s", task.ID)
	}
}

func TestQueue_Dequeue_ConcurrentLimit(t *testing.T) {
	q := NewQueue(QueueConfig{MaxQueueSize: 10, MaxConcurrent: 2})

	// Submit 3 tasks
	q.Submit(&Task{ID: "t1", Priority: 1})
	q.Submit(&Task{ID: "t2", Priority: 1})
	q.Submit(&Task{ID: "t3", Priority: 1})

	// Dequeue 2 (up to concurrent limit)
	task1 := q.Dequeue()
	task2 := q.Dequeue()

	if task1 == nil || task2 == nil {
		t.Error("expected to dequeue 2 tasks")
	}

	// Third dequeue should return nil (concurrent limit reached)
	task3 := q.Dequeue()
	if task3 != nil {
		t.Error("expected nil when concurrent limit reached")
	}

	// Complete one task
	q.Complete(task1.ID, "done")

	// Now we can dequeue again
	task3 = q.Dequeue()
	if task3 == nil {
		t.Error("expected to dequeue after completing a task")
	}
}

func TestQueue_Complete(t *testing.T) {
	q := NewQueue(QueueConfig{MaxQueueSize: 10, MaxConcurrent: 5})

	task := &Task{ID: "complete-test", Priority: 1}
	q.Submit(task)
	q.Dequeue()

	err := q.Complete("complete-test", "success output")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stats := q.GetStats()
	if stats["running"].(int) != 0 {
		t.Errorf("expected 0 running, got %v", stats["running"])
	}
	if stats["completed"].(int) != 1 {
		t.Errorf("expected 1 completed, got %v", stats["completed"])
	}

	// Check result
	_, result, found := q.GetTask("complete-test")
	if !found {
		t.Error("expected to find completed task")
	}
	if result == nil || result.Output != "success output" {
		t.Error("expected result with correct output")
	}
}

func TestQueue_Fail_WithRetry(t *testing.T) {
	q := NewQueue(QueueConfig{MaxQueueSize: 10, MaxConcurrent: 5, DefaultRetries: 2})

	task := &Task{ID: "retry-test", Priority: 1}
	q.Submit(task)
	q.Dequeue()

	// First failure - should retry
	err := q.Fail("retry-test", "temporary error")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stats := q.GetStats()
	if stats["pending"].(int) != 1 {
		t.Errorf("expected task to be re-queued, got pending=%v", stats["pending"])
	}

	// Dequeue again
	retriedTask := q.Dequeue()
	if retriedTask == nil || retriedTask.RetryCount != 1 {
		t.Error("expected retried task with retry count 1")
	}

	// Second failure - should retry again
	q.Fail("retry-test", "temporary error 2")
	q.Dequeue()

	// Third failure - max retries exceeded, should fail permanently
	q.Fail("retry-test", "permanent error")

	stats = q.GetStats()
	if stats["pending"].(int) != 0 {
		t.Errorf("expected 0 pending after max retries, got %v", stats["pending"])
	}

	_, result, found := q.GetTask("retry-test")
	if !found || result == nil || result.Success {
		t.Error("expected failed result")
	}
}

func TestQueue_Cancel_Pending(t *testing.T) {
	q := NewQueue(QueueConfig{MaxQueueSize: 10, MaxConcurrent: 5})

	q.Submit(&Task{ID: "cancel-pending", Priority: 1})

	err := q.Cancel("cancel-pending")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stats := q.GetStats()
	if stats["pending"].(int) != 0 {
		t.Errorf("expected 0 pending after cancel, got %v", stats["pending"])
	}
}

func TestQueue_Cancel_Running(t *testing.T) {
	q := NewQueue(QueueConfig{MaxQueueSize: 10, MaxConcurrent: 5})

	q.Submit(&Task{ID: "cancel-running", Priority: 1})
	q.Dequeue()

	err := q.Cancel("cancel-running")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stats := q.GetStats()
	if stats["running"].(int) != 0 {
		t.Errorf("expected 0 running after cancel, got %v", stats["running"])
	}
}

func TestQueue_Cancel_NotFound(t *testing.T) {
	q := NewQueue(QueueConfig{MaxQueueSize: 10})

	err := q.Cancel("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent task")
	}
}

func TestQueue_GetTask(t *testing.T) {
	q := NewQueue(QueueConfig{MaxQueueSize: 10, MaxConcurrent: 5})

	// Pending task
	q.Submit(&Task{ID: "get-pending", Title: "Pending Task", Priority: 1})
	task, _, found := q.GetTask("get-pending")
	if !found || task == nil || task.Title != "Pending Task" {
		t.Error("expected to find pending task")
	}

	// Dequeue the pending task to make it running
	dequeuedTask := q.Dequeue()
	if dequeuedTask == nil || dequeuedTask.ID != "get-pending" {
		t.Error("expected to dequeue get-pending task")
	}

	// Check running task
	task, _, found = q.GetTask("get-pending")
	if !found || task == nil || task.Status != TaskRunning {
		t.Error("expected to find running task")
	}

	// Complete the task
	q.Complete("get-pending", "done")
	_, result, found := q.GetTask("get-pending")
	if !found || result == nil {
		t.Error("expected to find completed task result")
	}
}

func TestQueue_ListPending(t *testing.T) {
	q := NewQueue(QueueConfig{MaxQueueSize: 10})

	q.Submit(&Task{ID: "p1", Priority: 1})
	q.Submit(&Task{ID: "p2", Priority: 2})

	pending := q.ListPending()
	if len(pending) != 2 {
		t.Errorf("expected 2 pending tasks, got %d", len(pending))
	}
}

func TestQueue_ListRunning(t *testing.T) {
	q := NewQueue(QueueConfig{MaxQueueSize: 10, MaxConcurrent: 5})

	q.Submit(&Task{ID: "r1", Priority: 1})
	q.Submit(&Task{ID: "r2", Priority: 1})
	q.Dequeue()
	q.Dequeue()

	running := q.ListRunning()
	if len(running) != 2 {
		t.Errorf("expected 2 running tasks, got %d", len(running))
	}
}

func TestQueue_Callbacks(t *testing.T) {
	q := NewQueue(QueueConfig{MaxQueueSize: 10, MaxConcurrent: 5})

	startCalled := false
	completeCalled := false

	q.OnTaskStart(func(task *Task) {
		startCalled = true
	})

	q.OnTaskComplete(func(task *Task, result *TaskResult) {
		completeCalled = true
	})

	q.Submit(&Task{ID: "callback-test", Priority: 1})
	q.Dequeue()

	if !startCalled {
		t.Error("expected OnTaskStart to be called")
	}

	q.Complete("callback-test", "done")

	if !completeCalled {
		t.Error("expected OnTaskComplete to be called")
	}
}

func TestQueue_StartStop(t *testing.T) {
	q := NewQueue(QueueConfig{
		MaxQueueSize:    10,
		CleanupInterval: 100 * time.Millisecond,
	})

	q.Start()
	time.Sleep(50 * time.Millisecond)
	q.Stop()

	// Should not panic or hang
}

func TestTaskStatus_Values(t *testing.T) {
	statuses := []TaskStatus{
		TaskPending,
		TaskRunning,
		TaskCompleted,
		TaskFailed,
		TaskCancelled,
	}

	expected := []string{
		"pending",
		"running",
		"completed",
		"failed",
		"cancelled",
	}

	for i, status := range statuses {
		if string(status) != expected[i] {
			t.Errorf("expected %s, got %s", expected[i], status)
		}
	}
}

func BenchmarkQueue_Submit(b *testing.B) {
	q := NewQueue(QueueConfig{MaxQueueSize: b.N + 100})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q.Submit(&Task{
			ID:       string(rune(i)),
			Priority: i % 10,
		})
	}
}

func BenchmarkQueue_PriorityInsertion(b *testing.B) {
	q := NewQueue(QueueConfig{MaxQueueSize: 10000})

	// Pre-fill with some tasks
	for i := 0; i < 1000; i++ {
		q.Submit(&Task{ID: string(rune(i)), Priority: 5})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q.Submit(&Task{
			ID:       string(rune(i + 1000)),
			Priority: i % 10,
		})
	}
}
