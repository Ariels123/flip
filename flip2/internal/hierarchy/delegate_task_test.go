package hierarchy

import (
	"context"
	"testing"
	"time"
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

// TestDelegateTaskWorkerCapacityLimit verifies that delegation respects MaxTasksPerWorker limit.
func TestDelegateTaskWorkerCapacityLimit(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")

	supervisor, _ := h.Supervisors["supervisor-1"]
	// Set low task limit for testing
	supervisor.Capabilities.DelegationBudget.MaxTasksPerWorker = 2
	supervisor.Capabilities.DelegationBudget.MaxConcurrentSpawns = 1
	sa, _ := NewSupervisorAgent(supervisor)

	ctx := context.Background()
	sa.SpawnWorker(ctx, "worker-1")

	// Delegate tasks up to capacity
	for i := 1; i <= 2; i++ {
		task := &TaskRequirements{
			TaskID:   "task-" + string(rune(48+i)),
			TaskType: "test",
		}
		result, err := sa.DelegateTask(ctx, task, StrategyLeastLoaded)
		if err != nil {
			t.Fatalf("DelegateTask %d failed: %v", i, err)
		}
		if !result.Success {
			t.Fatalf("Task %d delegation should succeed: %s", i, result.Reason)
		}
	}

	// Verify worker is at capacity
	load, maxLoad, _ := sa.GetWorkerLoad("worker-1")
	if load != 2 || maxLoad != 2 {
		t.Errorf("Expected load 2/2, got %d/%d", load, maxLoad)
	}

	// Third task should fail (no available workers)
	task3 := &TaskRequirements{
		TaskID:   "task-3",
		TaskType: "test",
	}
	result3, err := sa.DelegateTask(ctx, task3, StrategyLeastLoaded)
	if err != nil {
		t.Fatalf("DelegateTask 3 should not error: %v", err)
	}
	if result3.Success {
		t.Error("Task 3 should fail when single worker is at capacity")
	}
	if result3.Reason == "" {
		t.Error("Failed delegation should have a reason")
	}
}

// TestDelegateTaskMultipleWorkersCapacity verifies delegation distributes across workers when one is full.
func TestDelegateTaskMultipleWorkersCapacity(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")

	supervisor, _ := h.Supervisors["supervisor-1"]
	supervisor.Capabilities.DelegationBudget.MaxTasksPerWorker = 2
	supervisor.Capabilities.DelegationBudget.MaxConcurrentSpawns = 3
	sa, _ := NewSupervisorAgent(supervisor)

	ctx := context.Background()
	sa.SpawnWorker(ctx, "worker-1")
	sa.SpawnWorker(ctx, "worker-2")

	// Fill worker-1 first (2 tasks)
	for i := 1; i <= 2; i++ {
		task := &TaskRequirements{
			TaskID:            "task-" + string(rune(48+i)),
			TaskType:          "test",
			PreferredWorkerID: "worker-1",
		}
		sa.DelegateTask(ctx, task, StrategyLeastLoaded)
	}

	// Next task should go to worker-2
	task3 := &TaskRequirements{
		TaskID:   "task-3",
		TaskType: "test",
	}
	result3, err := sa.DelegateTask(ctx, task3, StrategyLeastLoaded)
	if err != nil {
		t.Fatalf("DelegateTask 3 failed: %v", err)
	}
	if !result3.Success {
		t.Fatalf("Task 3 should succeed: %s", result3.Reason)
	}
	if result3.WorkerID != "worker-2" {
		t.Errorf("Task 3 should go to worker-2, got %s", result3.WorkerID)
	}

	// Verify loads
	load1, _, _ := sa.GetWorkerLoad("worker-1")
	load2, _, _ := sa.GetWorkerLoad("worker-2")
	if load1 != 2 {
		t.Errorf("worker-1 load = %d, want 2", load1)
	}
	if load2 != 1 {
		t.Errorf("worker-2 load = %d, want 1", load2)
	}
}

// TestDelegateTaskPriorityWithLoad verifies priority delegation considers load.
func TestDelegateTaskPriorityWithLoad(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")

	supervisor, _ := h.Supervisors["supervisor-1"]
	supervisor.Capabilities.DelegationBudget.MaxTasksPerWorker = 2
	supervisor.Capabilities.DelegationBudget.MaxConcurrentSpawns = 3
	sa, _ := NewSupervisorAgent(supervisor)

	ctx := context.Background()
	sa.SpawnWorker(ctx, "worker-low")
	sa.SpawnWorker(ctx, "worker-high")

	sa.SetWorkerPriority("worker-low", 1)
	sa.SetWorkerPriority("worker-high", 10)

	// Fill high-priority worker to capacity
	for i := 1; i <= 2; i++ {
		task := &TaskRequirements{
			TaskID:            "task-" + string(rune(48+i)),
			TaskType:          "test",
			PreferredWorkerID: "worker-high",
		}
		sa.DelegateTask(ctx, task, StrategyPriority)
	}

	// Next task should go to worker-low (worker-high is full)
	task3 := &TaskRequirements{
		TaskID:   "task-3",
		TaskType: "test",
	}
	result3, err := sa.DelegateTask(ctx, task3, StrategyPriority)
	if err != nil {
		t.Fatalf("DelegateTask 3 failed: %v", err)
	}
	if !result3.Success {
		t.Fatalf("Task 3 should succeed: %s", result3.Reason)
	}
	if result3.WorkerID != "worker-low" {
		t.Errorf("Task 3 should go to worker-low (only available), got %s", result3.WorkerID)
	}
}

// TestDelegateTaskDefaultStrategy verifies unknown strategy falls back to least loaded.
func TestDelegateTaskDefaultStrategy(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")

	supervisor, _ := h.Supervisors["supervisor-1"]
	supervisor.Capabilities.DelegationBudget.MaxConcurrentSpawns = 2
	sa, _ := NewSupervisorAgent(supervisor)

	ctx := context.Background()
	sa.SpawnWorker(ctx, "worker-1")
	sa.SpawnWorker(ctx, "worker-2")

	// Use unknown strategy
	task := &TaskRequirements{
		TaskID:   "task-1",
		TaskType: "test",
	}
	result, err := sa.DelegateTask(ctx, task, DelegationStrategy("unknown_strategy"))
	if err != nil {
		t.Fatalf("DelegateTask with unknown strategy failed: %v", err)
	}
	if !result.Success {
		t.Fatalf("Task delegation should succeed with fallback: %s", result.Reason)
	}
}

// TestDelegateTaskEstimatedDeadline verifies deadline is calculated correctly.
func TestDelegateTaskEstimatedDeadline(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")

	supervisor, _ := h.Supervisors["supervisor-1"]
	supervisor.Capabilities.DelegationBudget.TimeoutSeconds = 300
	sa, _ := NewSupervisorAgent(supervisor)

	ctx := context.Background()
	sa.SpawnWorker(ctx, "worker-1")

	beforeDelegation := time.Now()

	task := &TaskRequirements{
		TaskID:   "task-1",
		TaskType: "test",
	}
	result, err := sa.DelegateTask(ctx, task, StrategyLeastLoaded)
	if err != nil {
		t.Fatalf("DelegateTask failed: %v", err)
	}

	afterDelegation := time.Now()

	// Check deadline is approximately 300 seconds from delegation time
	expectedDeadline := beforeDelegation.Add(300 * time.Second)
	if result.EstimatedDeadline.Before(expectedDeadline.Add(-time.Second)) {
		t.Errorf("Deadline too early: got %v, expected around %v", result.EstimatedDeadline, expectedDeadline)
	}
	if result.EstimatedDeadline.After(afterDelegation.Add(301 * time.Second)) {
		t.Errorf("Deadline too late: got %v", result.EstimatedDeadline)
	}
}

// TestDelegateTaskCustomTimeout verifies task-specific timeout overrides budget timeout.
func TestDelegateTaskCustomTimeout(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")

	supervisor, _ := h.Supervisors["supervisor-1"]
	supervisor.Capabilities.DelegationBudget.TimeoutSeconds = 600
	sa, _ := NewSupervisorAgent(supervisor)

	ctx := context.Background()
	sa.SpawnWorker(ctx, "worker-1")

	beforeDelegation := time.Now()

	// Task with custom timeout
	task := &TaskRequirements{
		TaskID:         "task-1",
		TaskType:       "test",
		TimeoutSeconds: 120,
	}
	result, err := sa.DelegateTask(ctx, task, StrategyLeastLoaded)
	if err != nil {
		t.Fatalf("DelegateTask failed: %v", err)
	}

	// Deadline should be ~120 seconds, not 600
	expectedDeadline := beforeDelegation.Add(120 * time.Second)
	if result.EstimatedDeadline.After(expectedDeadline.Add(5 * time.Second)) {
		t.Errorf("Deadline should use task timeout (120s), got %v", result.EstimatedDeadline)
	}
}

// TestDelegateTaskDelegatedAtTimestamp verifies DelegatedAt is set correctly.
func TestDelegateTaskDelegatedAtTimestamp(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")

	supervisor, _ := h.Supervisors["supervisor-1"]
	sa, _ := NewSupervisorAgent(supervisor)

	ctx := context.Background()
	sa.SpawnWorker(ctx, "worker-1")

	beforeDelegation := time.Now()

	task := &TaskRequirements{
		TaskID:   "task-1",
		TaskType: "test",
	}
	result, err := sa.DelegateTask(ctx, task, StrategyLeastLoaded)
	if err != nil {
		t.Fatalf("DelegateTask failed: %v", err)
	}

	afterDelegation := time.Now()

	if result.DelegatedAt.Before(beforeDelegation) {
		t.Errorf("DelegatedAt %v is before test start %v", result.DelegatedAt, beforeDelegation)
	}
	if result.DelegatedAt.After(afterDelegation) {
		t.Errorf("DelegatedAt %v is after test end %v", result.DelegatedAt, afterDelegation)
	}
}

// TestDelegateTaskTerminatedWorkerExcluded verifies terminated workers are not selected.
func TestDelegateTaskTerminatedWorkerExcluded(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")

	supervisor, _ := h.Supervisors["supervisor-1"]
	supervisor.Capabilities.DelegationBudget.MaxConcurrentSpawns = 2
	sa, _ := NewSupervisorAgent(supervisor)

	ctx := context.Background()
	sa.SpawnWorker(ctx, "worker-1")
	sa.SpawnWorker(ctx, "worker-2")

	// Terminate worker-1
	sa.TerminateWorker(ctx, "worker-1")

	// All tasks should go to worker-2
	for i := 1; i <= 3; i++ {
		task := &TaskRequirements{
			TaskID:   "task-" + string(rune(48+i)),
			TaskType: "test",
		}
		result, err := sa.DelegateTask(ctx, task, StrategyRoundRobin)
		if err != nil {
			t.Fatalf("DelegateTask %d failed: %v", i, err)
		}
		if !result.Success {
			t.Fatalf("Task %d should succeed: %s", i, result.Reason)
		}
		if result.WorkerID != "worker-2" {
			t.Errorf("Task %d should go to worker-2 (only active), got %s", i, result.WorkerID)
		}
	}
}

// TestDelegateTaskMultipleCapabilities verifies capability matching with multiple requirements.
func TestDelegateTaskMultipleCapabilities(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")

	supervisor, _ := h.Supervisors["supervisor-1"]
	supervisor.Capabilities.DelegationBudget.MaxConcurrentSpawns = 3
	sa, _ := NewSupervisorAgent(supervisor)

	ctx := context.Background()
	sa.SpawnWorker(ctx, "partial-worker")
	sa.SpawnWorker(ctx, "full-worker")

	// One worker has partial capabilities
	sa.SetWorkerCapabilities("partial-worker", []string{"testing"})
	// One worker has all capabilities
	sa.SetWorkerCapabilities("full-worker", []string{"testing", "debugging", "code_review"})

	// Task requires multiple capabilities
	task := &TaskRequirements{
		TaskID:               "complex-task",
		TaskType:             "complex",
		RequiredCapabilities: []string{"testing", "debugging"},
	}
	result, err := sa.DelegateTask(ctx, task, StrategyCapabilityMatch)
	if err != nil {
		t.Fatalf("DelegateTask failed: %v", err)
	}
	if !result.Success {
		t.Fatalf("Task should succeed: %s", result.Reason)
	}
	if result.WorkerID != "full-worker" {
		t.Errorf("Task should go to full-worker (has all capabilities), got %s", result.WorkerID)
	}
}

// TestDelegateTaskPreferredWorkerUnavailable verifies fallback when preferred worker is unavailable.
func TestDelegateTaskPreferredWorkerUnavailable(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")

	supervisor, _ := h.Supervisors["supervisor-1"]
	supervisor.Capabilities.DelegationBudget.MaxTasksPerWorker = 1
	supervisor.Capabilities.DelegationBudget.MaxConcurrentSpawns = 2
	sa, _ := NewSupervisorAgent(supervisor)

	ctx := context.Background()
	sa.SpawnWorker(ctx, "worker-1")
	sa.SpawnWorker(ctx, "worker-2")

	// Fill worker-1 to capacity
	task1 := &TaskRequirements{
		TaskID:            "task-1",
		TaskType:          "test",
		PreferredWorkerID: "worker-1",
	}
	sa.DelegateTask(ctx, task1, StrategyLeastLoaded)

	// Request worker-1 again (should fallback to worker-2)
	task2 := &TaskRequirements{
		TaskID:            "task-2",
		TaskType:          "test",
		PreferredWorkerID: "worker-1", // preferred but full
	}
	result2, err := sa.DelegateTask(ctx, task2, StrategyLeastLoaded)
	if err != nil {
		t.Fatalf("DelegateTask 2 failed: %v", err)
	}
	if !result2.Success {
		t.Fatalf("Task 2 should succeed with fallback: %s", result2.Reason)
	}
	if result2.WorkerID != "worker-2" {
		t.Errorf("Task 2 should fallback to worker-2, got %s", result2.WorkerID)
	}
}

// TestDelegateTaskActiveTaskCountSync verifies active task count stays synchronized.
func TestDelegateTaskActiveTaskCountSync(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")

	supervisor, _ := h.Supervisors["supervisor-1"]
	supervisor.Capabilities.DelegationBudget.MaxConcurrentSpawns = 2
	sa, _ := NewSupervisorAgent(supervisor)

	ctx := context.Background()
	sa.SpawnWorker(ctx, "worker-1")
	sa.SpawnWorker(ctx, "worker-2")

	if sa.GetActiveTaskCount() != 0 {
		t.Errorf("Initial active task count = %d, want 0", sa.GetActiveTaskCount())
	}

	// Delegate 4 tasks
	for i := 1; i <= 4; i++ {
		task := &TaskRequirements{
			TaskID:   "task-" + string(rune(48+i)),
			TaskType: "test",
		}
		sa.DelegateTask(ctx, task, StrategyRoundRobin)
	}

	if sa.GetActiveTaskCount() != 4 {
		t.Errorf("After 4 delegations, active task count = %d, want 4", sa.GetActiveTaskCount())
	}

	// Complete 2 tasks
	sa.CompleteTask("worker-1")
	sa.CompleteTask("worker-2")

	if sa.GetActiveTaskCount() != 2 {
		t.Errorf("After 2 completions, active task count = %d, want 2", sa.GetActiveTaskCount())
	}
}

// TestDelegateTaskWorkerMetadataUpdate verifies worker metadata is updated on delegation.
func TestDelegateTaskWorkerMetadataUpdate(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")

	supervisor, _ := h.Supervisors["supervisor-1"]
	sa, _ := NewSupervisorAgent(supervisor)

	ctx := context.Background()
	sa.SpawnWorker(ctx, "worker-1")

	task := &TaskRequirements{
		TaskID:   "test-task-123",
		TaskType: "test",
	}
	result, err := sa.DelegateTask(ctx, task, StrategyLeastLoaded)
	if err != nil {
		t.Fatalf("DelegateTask failed: %v", err)
	}
	if !result.Success {
		t.Fatalf("Task delegation failed: %s", result.Reason)
	}

	// Check worker metadata was updated
	workers := sa.GetSpawnedWorkers()
	var worker *HierarchyNode
	for _, w := range workers {
		if w.AgentID == "worker-1" {
			worker = w
			break
		}
	}

	if worker == nil {
		t.Fatal("worker-1 not found")
	}

	if worker.Metadata["last_task_id"] != "test-task-123" {
		t.Errorf("last_task_id = %v, want test-task-123", worker.Metadata["last_task_id"])
	}

	if _, ok := worker.Metadata["last_task_assigned_at"].(time.Time); !ok {
		t.Error("last_task_assigned_at should be set to a time.Time")
	}
}

// TestDelegateTaskCapabilityMatchScore verifies match scores are calculated correctly.
func TestDelegateTaskCapabilityMatchScore(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")

	supervisor, _ := h.Supervisors["supervisor-1"]
	supervisor.Capabilities.DelegationBudget.MaxConcurrentSpawns = 2
	sa, _ := NewSupervisorAgent(supervisor)

	ctx := context.Background()
	sa.SpawnWorker(ctx, "worker-1")

	// Worker with exact capability match
	sa.SetWorkerCapabilities("worker-1", []string{"testing"})

	task := &TaskRequirements{
		TaskID:               "test-task",
		TaskType:             "testing",
		RequiredCapabilities: []string{"testing"},
	}
	result, err := sa.DelegateTask(ctx, task, StrategyCapabilityMatch)
	if err != nil {
		t.Fatalf("DelegateTask failed: %v", err)
	}
	if !result.Success {
		t.Fatalf("Task delegation failed: %s", result.Reason)
	}

	// Match score should be high (1.0 or close)
	if result.MatchScore < 0.9 {
		t.Errorf("Match score = %f, want >= 0.9 for exact capability match", result.MatchScore)
	}
}

// TestDelegateTaskLeastLoadedMatchScore verifies match scores reflect capacity.
func TestDelegateTaskLeastLoadedMatchScore(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")

	supervisor, _ := h.Supervisors["supervisor-1"]
	supervisor.Capabilities.DelegationBudget.MaxTasksPerWorker = 4
	sa, _ := NewSupervisorAgent(supervisor)

	ctx := context.Background()
	sa.SpawnWorker(ctx, "worker-1")

	// First task should have match score 1.0 (0/4 capacity used)
	task1 := &TaskRequirements{
		TaskID:   "task-1",
		TaskType: "test",
	}
	result1, err := sa.DelegateTask(ctx, task1, StrategyLeastLoaded)
	if err != nil {
		t.Fatalf("DelegateTask 1 failed: %v", err)
	}
	if result1.MatchScore != 1.0 {
		t.Errorf("First task match score = %f, want 1.0 (empty worker)", result1.MatchScore)
	}

	// Second task should have match score 0.75 (1/4 capacity used)
	task2 := &TaskRequirements{
		TaskID:   "task-2",
		TaskType: "test",
	}
	result2, err := sa.DelegateTask(ctx, task2, StrategyLeastLoaded)
	if err != nil {
		t.Fatalf("DelegateTask 2 failed: %v", err)
	}
	if result2.MatchScore != 0.75 {
		t.Errorf("Second task match score = %f, want 0.75", result2.MatchScore)
	}
}
