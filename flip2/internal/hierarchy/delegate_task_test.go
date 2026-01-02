package hierarchy

import (
	"context"
	"testing"
)

// TestDelegateTaskIncrementsLoad verifies that delegating a task increments the worker's load count.
func TestDelegateTaskIncrementsLoad(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")

	supervisor, _ := h.Supervisors["supervisor-1"]
	sa, _ := NewSupervisorAgent(supervisor)

	ctx := context.Background()
	sa.SpawnWorker(ctx, "worker-1")

	// Check initial load
	activeTasks, _, err := sa.GetWorkerLoad("worker-1")
	if err != nil {
		t.Fatalf("GetWorkerLoad failed: %v", err)
	}
	if activeTasks != 0 {
		t.Errorf("Initial active tasks = %d, want 0", activeTasks)
	}

	// Delegate a task
	task := &TaskRequirements{
		TaskID:   "task-1",
		TaskType: "test",
	}

	result, err := sa.DelegateTask(ctx, task, StrategyLeastLoaded)
	if err != nil {
		t.Fatalf("DelegateTask failed: %v", err)
	}
	if !result.Success {
		t.Fatalf("Task delegation failed: %s", result.Reason)
	}

	// Check load after delegation
	activeTasks, _, err = sa.GetWorkerLoad("worker-1")
	if err != nil {
		t.Fatalf("GetWorkerLoad failed: %v", err)
	}
	if activeTasks != 1 {
		t.Errorf("Active tasks after delegation = %d, want 1", activeTasks)
	}

	// Complete the task
	sa.CompleteTask("worker-1")

	// Check load after completion
	activeTasks, _, err = sa.GetWorkerLoad("worker-1")
	if err != nil {
		t.Fatalf("GetWorkerLoad failed: %v", err)
	}
	if activeTasks != 0 {
		t.Errorf("Active tasks after completion = %d, want 0", activeTasks)
	}
}

// TestDelegateTaskLeastLoadedDistribution verifies that LeastLoaded strategy distributes tasks evenly.
func TestDelegateTaskLeastLoadedDistribution(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")

	supervisor, _ := h.Supervisors["supervisor-1"]
	// Allow 2 concurrent workers
	supervisor.Capabilities.DelegationBudget.MaxConcurrentSpawns = 2
	sa, _ := NewSupervisorAgent(supervisor)

	ctx := context.Background()
	sa.SpawnWorker(ctx, "worker-1")
	sa.SpawnWorker(ctx, "worker-2")

	// Delegate task 1
	task1 := &TaskRequirements{TaskID: "task-1", TaskType: "test"}
	result1, err := sa.DelegateTask(ctx, task1, StrategyLeastLoaded)
	if err != nil {
		t.Fatalf("DelegateTask 1 failed: %v", err)
	}

	// Delegate task 2
	task2 := &TaskRequirements{TaskID: "task-2", TaskType: "test"}
	result2, err := sa.DelegateTask(ctx, task2, StrategyLeastLoaded)
	if err != nil {
		t.Fatalf("DelegateTask 2 failed: %v", err)
	}

	// Workers should be different
	if result1.WorkerID == result2.WorkerID {
		t.Errorf("LeastLoaded strategy assigned both tasks to %s, expected distribution", result1.WorkerID)
	}

	// Verify loads
	load1, _, _ := sa.GetWorkerLoad("worker-1")
	load2, _, _ := sa.GetWorkerLoad("worker-2")

	if load1 != 1 || load2 != 1 {
		t.Errorf("Expected load 1 for both workers, got worker-1: %d, worker-2: %d", load1, load2)
	}
}
