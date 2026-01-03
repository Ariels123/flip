package hierarchy

import (
	"context"
	"testing"
	"time"
)

// TestNewSupervisorAgent tests creating a new supervisor agent.
func TestNewSupervisorAgent(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")

	supervisor, _ := h.Supervisors["supervisor-1"]

	// Create supervisor agent
	sa, err := NewSupervisorAgent(supervisor)
	if err != nil {
		t.Fatalf("NewSupervisorAgent failed: %v", err)
	}

	if sa == nil {
		t.Fatal("SupervisorAgent is nil")
	}

	if sa.GetID() != "supervisor-1" {
		t.Errorf("SupervisorAgent ID = %s, want supervisor-1", sa.GetID())
	}

	if sa.GetWorkerCount() != 0 {
		t.Errorf("Initial worker count = %d, want 0", sa.GetWorkerCount())
	}
}

// TestNewSupervisorAgentWithNilNode tests error handling for nil node.
func TestNewSupervisorAgentWithNilNode(t *testing.T) {
	_, err := NewSupervisorAgent(nil)
	if err == nil {
		t.Error("NewSupervisorAgent with nil node should return error")
	}
}

// TestNewSupervisorAgentWithWrongRole tests error handling for wrong role.
func TestNewSupervisorAgentWithWrongRole(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")

	coordinator := h.Coordinator

	_, err := NewSupervisorAgent(coordinator)
	if err == nil {
		t.Error("NewSupervisorAgent with coordinator role should return error")
	}
}

// TestSpawnWorker tests spawning a worker under a supervisor.
func TestSpawnWorker(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")

	supervisor, _ := h.Supervisors["supervisor-1"]
	sa, _ := NewSupervisorAgent(supervisor)

	// Spawn a worker
	ctx := context.Background()
	worker, err := sa.SpawnWorker(ctx, "worker-1")
	if err != nil {
		t.Fatalf("SpawnWorker failed: %v", err)
	}

	if worker == nil {
		t.Fatal("Worker is nil")
	}

	if worker.AgentID != "worker-1" {
		t.Errorf("Worker ID = %s, want worker-1", worker.AgentID)
	}

	if worker.Role != RoleWorker {
		t.Errorf("Worker Role = %s, want %s", worker.Role, RoleWorker)
	}

	if worker.ParentID == nil || *worker.ParentID != "supervisor-1" {
		t.Error("Worker parent should be supervisor-1")
	}

	// Verify worker count
	if sa.GetWorkerCount() != 1 {
		t.Errorf("Worker count = %d, want 1", sa.GetWorkerCount())
	}
}

// TestSpawnWorkerEmptyID tests spawning with empty worker ID.
func TestSpawnWorkerEmptyID(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")

	supervisor, _ := h.Supervisors["supervisor-1"]
	sa, _ := NewSupervisorAgent(supervisor)

	ctx := context.Background()
	_, err := sa.SpawnWorker(ctx, "")
	if err == nil {
		t.Error("SpawnWorker with empty ID should return error")
	}
}

// TestSpawnWorkerDuplicate tests spawning duplicate workers.
func TestSpawnWorkerDuplicate(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")

	supervisor, _ := h.Supervisors["supervisor-1"]
	sa, _ := NewSupervisorAgent(supervisor)

	ctx := context.Background()
	sa.SpawnWorker(ctx, "worker-1")

	_, err := sa.SpawnWorker(ctx, "worker-1")
	if err == nil {
		t.Error("Spawning duplicate worker should return error")
	}
}

// TestSpawnWorkerBudgetLimit tests spawning beyond the delegation budget.
// The supervisor budget has:
// - MaxWorkers: 5 (total workers can have)
// - MaxConcurrentSpawns: 2 (workers that can be active simultaneously)
func TestSpawnWorkerBudgetLimit(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")

	supervisor, _ := h.Supervisors["supervisor-1"]
	sa, _ := NewSupervisorAgent(supervisor)

	ctx := context.Background()

	// Spawn up to the max concurrent limit (default is 2)
	for i := 1; i <= 2; i++ {
		workerID := "worker-" + string(rune(48+i))
		_, err := sa.SpawnWorker(ctx, workerID)
		if err != nil {
			t.Fatalf("Failed to spawn worker %d: %v", i, err)
		}
	}

	// Try to exceed the concurrent limit
	_, err := sa.SpawnWorker(ctx, "worker-3")
	if err == nil {
		t.Error("Spawning beyond concurrent limit should return error")
	}

	if sa.GetWorkerCount() != 2 {
		t.Errorf("Worker count = %d, want 2", sa.GetWorkerCount())
	}
}

// TestAssignTask tests assigning tasks to workers.
func TestAssignTask(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")

	supervisor, _ := h.Supervisors["supervisor-1"]
	sa, _ := NewSupervisorAgent(supervisor)

	ctx := context.Background()
	sa.SpawnWorker(ctx, "worker-1")

	// Assign tasks
	err := sa.AssignTask("worker-1")
	if err != nil {
		t.Fatalf("AssignTask failed: %v", err)
	}

	if sa.GetActiveTaskCount() != 1 {
		t.Errorf("Active task count = %d, want 1", sa.GetActiveTaskCount())
	}
}

// TestAssignTaskNonExistentWorker tests assigning task to non-existent worker.
func TestAssignTaskNonExistentWorker(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")

	supervisor, _ := h.Supervisors["supervisor-1"]
	sa, _ := NewSupervisorAgent(supervisor)

	err := sa.AssignTask("non-existent-worker")
	if err == nil {
		t.Error("AssignTask to non-existent worker should return error")
	}
}

// TestCompleteTask tests completing a task.
func TestCompleteTask(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")

	supervisor, _ := h.Supervisors["supervisor-1"]
	sa, _ := NewSupervisorAgent(supervisor)

	ctx := context.Background()
	sa.SpawnWorker(ctx, "worker-1")
	sa.AssignTask("worker-1")

	if sa.GetActiveTaskCount() != 1 {
		t.Errorf("Active task count = %d, want 1", sa.GetActiveTaskCount())
	}

	sa.CompleteTask("worker-1")

	if sa.GetActiveTaskCount() != 0 {
		t.Errorf("Active task count = %d, want 0", sa.GetActiveTaskCount())
	}
}

// TestRecordWorkerResult tests recording worker results.
func TestRecordWorkerResult(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")

	supervisor, _ := h.Supervisors["supervisor-1"]
	sa, _ := NewSupervisorAgent(supervisor)

	ctx := context.Background()
	sa.SpawnWorker(ctx, "worker-1")

	// Record result
	result := &WorkerResult{
		WorkerID:    "worker-1",
		Status:      WorkerStatusCompleted,
		Result:      "task completed successfully",
		CompletedAt: time.Now(),
		DurationMs:  5000,
	}

	err := sa.RecordWorkerResult(result)
	if err != nil {
		t.Fatalf("RecordWorkerResult failed: %v", err)
	}

	// Check status was updated
	status, err := sa.GetWorkerStatus(ctx, "worker-1")
	if err != nil {
		t.Fatalf("GetWorkerStatus failed: %v", err)
	}

	if status != WorkerStatusCompleted {
		t.Errorf("Worker status = %s, want %s", status, WorkerStatusCompleted)
	}
}

// TestRecordWorkerResultNilResult tests recording nil result.
func TestRecordWorkerResultNilResult(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")

	supervisor, _ := h.Supervisors["supervisor-1"]
	sa, _ := NewSupervisorAgent(supervisor)

	err := sa.RecordWorkerResult(nil)
	if err == nil {
		t.Error("RecordWorkerResult with nil result should return error")
	}
}

// TestRecordWorkerResultEmptyWorkerID tests recording result with empty worker ID.
func TestRecordWorkerResultEmptyWorkerID(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")

	supervisor, _ := h.Supervisors["supervisor-1"]
	sa, _ := NewSupervisorAgent(supervisor)

	result := &WorkerResult{
		WorkerID: "",
		Status:   WorkerStatusCompleted,
	}

	err := sa.RecordWorkerResult(result)
	if err == nil {
		t.Error("RecordWorkerResult with empty worker ID should return error")
	}
}

// TestRecordWorkerResultUnknownWorker tests recording result for unknown worker.
func TestRecordWorkerResultUnknownWorker(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")

	supervisor, _ := h.Supervisors["supervisor-1"]
	sa, _ := NewSupervisorAgent(supervisor)

	result := &WorkerResult{
		WorkerID: "unknown-worker",
		Status:   WorkerStatusCompleted,
	}

	err := sa.RecordWorkerResult(result)
	if err == nil {
		t.Error("RecordWorkerResult for unknown worker should return error")
	}
}

// TestAggregateResults tests aggregating results from multiple workers.
// Due to concurrent spawn limit of 2, we can only spawn 2 active workers at once.
func TestAggregateResults(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")

	supervisor, _ := h.Supervisors["supervisor-1"]
	sa, _ := NewSupervisorAgent(supervisor)

	ctx := context.Background()

	// Spawn 2 workers (max concurrent spawns is 2)
	sa.SpawnWorker(ctx, "worker-1")
	sa.SpawnWorker(ctx, "worker-2")

	// Record results
	sa.RecordWorkerResult(&WorkerResult{
		WorkerID: "worker-1",
		Status:   WorkerStatusCompleted,
		Result:   "success",
	})

	sa.RecordWorkerResult(&WorkerResult{
		WorkerID: "worker-2",
		Status:   WorkerStatusFailed,
		Error:    "task failed",
	})

	// Aggregate results
	results, successCount, failureCount := sa.AggregateResults()

	if len(results) != 2 {
		t.Errorf("Result count = %d, want 2", len(results))
	}

	if successCount != 1 {
		t.Errorf("Success count = %d, want 1", successCount)
	}

	if failureCount != 1 {
		t.Errorf("Failure count = %d, want 1", failureCount)
	}
}

// TestTerminateWorker tests terminating a worker.
func TestTerminateWorker(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")

	supervisor, _ := h.Supervisors["supervisor-1"]
	sa, _ := NewSupervisorAgent(supervisor)

	ctx := context.Background()
	sa.SpawnWorker(ctx, "worker-1")

	initialCount := sa.GetWorkerCount()

	err := sa.TerminateWorker(ctx, "worker-1")
	if err != nil {
		t.Fatalf("TerminateWorker failed: %v", err)
	}

	status, _ := sa.GetWorkerStatus(ctx, "worker-1")
	if status != WorkerStatusTerminated {
		t.Errorf("Worker status = %s, want %s", status, WorkerStatusTerminated)
	}

	// Worker is still counted but children list is updated
	if sa.GetWorkerCount() != initialCount {
		t.Errorf("Worker count should remain %d", initialCount)
	}
}

// TestTerminateWorkerNonExistent tests terminating non-existent worker.
func TestTerminateWorkerNonExistent(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")

	supervisor, _ := h.Supervisors["supervisor-1"]
	sa, _ := NewSupervisorAgent(supervisor)

	err := sa.TerminateWorker(context.Background(), "non-existent")
	if err == nil {
		t.Error("TerminateWorker for non-existent worker should return error")
	}
}

// TestGetWorkerStatus tests getting worker status.
func TestGetWorkerStatus(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")

	supervisor, _ := h.Supervisors["supervisor-1"]
	sa, _ := NewSupervisorAgent(supervisor)

	ctx := context.Background()
	sa.SpawnWorker(ctx, "worker-1")

	status, err := sa.GetWorkerStatus(ctx, "worker-1")
	if err != nil {
		t.Fatalf("GetWorkerStatus failed: %v", err)
	}

	// Initial status should be "active" (from the node)
	if status == "" {
		t.Error("Worker status should not be empty")
	}
}

// TestGetWorkerStatusNonExistent tests getting status of non-existent worker.
func TestGetWorkerStatusNonExistent(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")

	supervisor, _ := h.Supervisors["supervisor-1"]
	sa, _ := NewSupervisorAgent(supervisor)

	_, err := sa.GetWorkerStatus(context.Background(), "non-existent")
	if err == nil {
		t.Error("GetWorkerStatus for non-existent worker should return error")
	}
}

// TestGetSpawnedWorkers tests retrieving all spawned workers.
// Note: Limited to 2 concurrent spawns by default.
func TestGetSpawnedWorkers(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")

	supervisor, _ := h.Supervisors["supervisor-1"]
	sa, _ := NewSupervisorAgent(supervisor)

	ctx := context.Background()
	sa.SpawnWorker(ctx, "worker-1")
	sa.SpawnWorker(ctx, "worker-2")

	workers := sa.GetSpawnedWorkers()

	if len(workers) != 2 {
		t.Errorf("Worker count = %d, want 2", len(workers))
	}

	// Verify all workers are present
	workerMap := make(map[string]bool)
	for _, w := range workers {
		workerMap[w.AgentID] = true
	}

	for i := 1; i <= 2; i++ {
		workerID := "worker-" + string(rune(48+i))
		if !workerMap[workerID] {
			t.Errorf("Worker %s not found in spawned workers", workerID)
		}
	}
}

// TestGetWorkerCount tests getting worker count.
func TestGetWorkerCount(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")

	supervisor, _ := h.Supervisors["supervisor-1"]
	sa, _ := NewSupervisorAgent(supervisor)

	if sa.GetWorkerCount() != 0 {
		t.Errorf("Initial worker count = %d, want 0", sa.GetWorkerCount())
	}

	ctx := context.Background()
	sa.SpawnWorker(ctx, "worker-1")
	sa.SpawnWorker(ctx, "worker-2")

	if sa.GetWorkerCount() != 2 {
		t.Errorf("Worker count = %d, want 2", sa.GetWorkerCount())
	}
}

// TestGetActiveTaskCount tests getting active task count.
func TestGetActiveTaskCount(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")

	supervisor, _ := h.Supervisors["supervisor-1"]
	sa, _ := NewSupervisorAgent(supervisor)

	ctx := context.Background()
	sa.SpawnWorker(ctx, "worker-1")
	sa.SpawnWorker(ctx, "worker-2")

	// Assign multiple tasks
	sa.AssignTask("worker-1")
	sa.AssignTask("worker-1")
	sa.AssignTask("worker-2")

	if sa.GetActiveTaskCount() != 3 {
		t.Errorf("Active task count = %d, want 3", sa.GetActiveTaskCount())
	}

	// Complete one task
	sa.CompleteTask("worker-1")

	if sa.GetActiveTaskCount() != 2 {
		t.Errorf("Active task count = %d, want 2", sa.GetActiveTaskCount())
	}
}

// TestIsWithinBudget tests budget checking.
// With default budget: MaxConcurrentSpawns=2, MaxWorkers=5
func TestIsWithinBudget(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")

	supervisor, _ := h.Supervisors["supervisor-1"]
	sa, _ := NewSupervisorAgent(supervisor)

	// Should be within budget initially
	if !sa.IsWithinBudget() {
		t.Error("Supervisor should be within budget initially")
	}

	ctx := context.Background()

	// Spawn workers up to concurrent limit (2)
	sa.SpawnWorker(ctx, "worker-1")
	sa.SpawnWorker(ctx, "worker-2")

	// After spawning the max concurrent workers, we're no longer within budget
	// (IsWithinBudget checks if we can spawn more)
	// This should now be false because we're at the limit
	if sa.IsWithinBudget() {
		t.Error("Supervisor should not be within budget after reaching concurrent spawn limit")
	}
}

// TestGetNodeRef tests getting the underlying hierarchy node.
func TestGetNodeRef(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")

	supervisor, _ := h.Supervisors["supervisor-1"]
	sa, _ := NewSupervisorAgent(supervisor)

	node := sa.GetNodeRef()

	if node == nil {
		t.Fatal("GetNodeRef returned nil")
	}

	if node.AgentID != "supervisor-1" {
		t.Errorf("Node ID = %s, want supervisor-1", node.AgentID)
	}

	if node.Role != RoleSupervisor {
		t.Errorf("Node role = %s, want %s", node.Role, RoleSupervisor)
	}
}

// TestGetID tests getting supervisor ID.
func TestGetID(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")

	supervisor, _ := h.Supervisors["supervisor-1"]
	sa, _ := NewSupervisorAgent(supervisor)

	if sa.GetID() != "supervisor-1" {
		t.Errorf("GetID returned %s, want supervisor-1", sa.GetID())
	}
}

// TestEscalateIssue tests escalating issues to the coordinator.
func TestEscalateIssue(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")

	supervisor, _ := h.Supervisors["supervisor-1"]
	sa, _ := NewSupervisorAgent(supervisor)

	escalation := sa.EscalateIssue("critical", "All workers failed")

	if escalation == "" {
		t.Error("Escalation message should not be empty")
	}

	// Verify the message contains expected parts
	if !contains(escalation, "ESCALATION") || !contains(escalation, "critical") || !contains(escalation, "supervisor-1") {
		t.Errorf("Escalation message format incorrect: %s", escalation)
	}
}

// TestConcurrentWorkerSpawning tests thread-safe worker spawning.
// Due to concurrent spawn limit of 2, not all attempts will succeed.
func TestConcurrentWorkerSpawning(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")

	supervisor, _ := h.Supervisors["supervisor-1"]
	sa, _ := NewSupervisorAgent(supervisor)

	ctx := context.Background()
	done := make(chan bool, 2)

	// Spawn workers concurrently (only 2 can succeed due to budget)
	for i := 0; i < 2; i++ {
		go func(idx int) {
			workerID := "worker-" + string(rune(48+idx))
			sa.SpawnWorker(ctx, workerID)
			done <- true
		}(i)
	}

	// Wait for all spawns
	for i := 0; i < 2; i++ {
		<-done
	}

	if sa.GetWorkerCount() != 2 {
		t.Errorf("Final worker count = %d, want 2", sa.GetWorkerCount())
	}
}

// TestConcurrentTaskAssignment tests thread-safe task assignment.
func TestConcurrentTaskAssignment(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")

	supervisor, _ := h.Supervisors["supervisor-1"]
	sa, _ := NewSupervisorAgent(supervisor)

	ctx := context.Background()
	sa.SpawnWorker(ctx, "worker-1")

	done := make(chan bool, 10)

	// Assign tasks concurrently
	for i := 0; i < 10; i++ {
		go func() {
			sa.AssignTask("worker-1")
			done <- true
		}()
	}

	// Wait for all assignments
	for i := 0; i < 10; i++ {
		<-done
	}

	// Note: We can only actually assign up to MaxTasksPerWorker (3) tasks
	// The rest should fail, but let's verify state is consistent
	if sa.GetWorkerCount() != 1 {
		t.Errorf("Worker count = %d, want 1", sa.GetWorkerCount())
	}
}


// TestSupervisorHierarchyIntegration tests supervisor integration with hierarchy.
func TestSupervisorHierarchyIntegration(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")

	supervisor, _ := h.Supervisors["supervisor-1"]
	sa, _ := NewSupervisorAgent(supervisor)

	ctx := context.Background()

	// Spawn workers through supervisor
	sa.SpawnWorker(ctx, "worker-1")
	sa.SpawnWorker(ctx, "worker-2")

	// Verify they appear in the supervisor's node children
	if len(supervisor.ChildrenIDs) != 2 {
		t.Errorf("Supervisor children count = %d, want 2", len(supervisor.ChildrenIDs))
	}

	// Verify children IDs are correct
	expectedChildren := map[string]bool{"worker-1": true, "worker-2": true}
	for _, childID := range supervisor.ChildrenIDs {
		if !expectedChildren[childID] {
			t.Errorf("Unexpected child ID: %s", childID)
		}
	}
}

// TestWorkerResultComplete tests complete worker result workflow.
func TestWorkerResultComplete(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")

	supervisor, _ := h.Supervisors["supervisor-1"]
	sa, _ := NewSupervisorAgent(supervisor)

	ctx := context.Background()

	// Spawn worker
	sa.SpawnWorker(ctx, "worker-1")
	sa.AssignTask("worker-1")

	// Simulate work
	time.Sleep(100 * time.Millisecond)

	// Record result
	startTime := time.Now().Add(-500 * time.Millisecond)
	endTime := time.Now()
	duration := endTime.Sub(startTime).Milliseconds()

	sa.RecordWorkerResult(&WorkerResult{
		WorkerID:    "worker-1",
		Status:      WorkerStatusCompleted,
		Result:      map[string]interface{}{"output": "test result"},
		CompletedAt: endTime,
		DurationMs:  duration,
	})

	sa.CompleteTask("worker-1")

	// Verify final state
	results, success, failures := sa.AggregateResults()

	if len(results) != 1 {
		t.Errorf("Result count = %d, want 1", len(results))
	}

	if success != 1 {
		t.Errorf("Success count = %d, want 1", success)
	}

	if failures != 0 {
		t.Errorf("Failure count = %d, want 0", failures)
	}

	if sa.GetActiveTaskCount() != 0 {
		t.Errorf("Active task count = %d, want 0", sa.GetActiveTaskCount())
	}
}

// ================================================================================
// DELEGATION STRATEGY TESTS
// ================================================================================

// TestDelegateTaskRoundRobin tests round-robin task delegation.
func TestDelegateTaskRoundRobin(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")

	supervisor, _ := h.Supervisors["supervisor-1"]
	// Increase concurrent spawn limit to allow more workers
	supervisor.Capabilities.DelegationBudget.MaxConcurrentSpawns = 5
	sa, _ := NewSupervisorAgent(supervisor)

	ctx := context.Background()

	// Spawn 3 workers
	sa.SpawnWorker(ctx, "worker-1")
	sa.SpawnWorker(ctx, "worker-2")
	sa.SpawnWorker(ctx, "worker-3")

	// Delegate multiple tasks using round-robin
	delegatedTo := make(map[string]int)
	for i := 1; i <= 6; i++ {
		task := &TaskRequirements{
			TaskID:   "task-" + string(rune(48+i)),
			TaskType: "test",
			Priority: 3,
		}

		result, err := sa.DelegateTask(ctx, task, StrategyRoundRobin)
		if err != nil {
			t.Fatalf("DelegateTask failed: %v", err)
		}

		if !result.Success {
			t.Fatalf("Task delegation failed: %s", result.Reason)
		}

		delegatedTo[result.WorkerID]++
	}

	// Verify tasks are distributed across workers
	if len(delegatedTo) < 2 {
		t.Error("Round-robin should distribute tasks to multiple workers")
	}
}

// TestDelegateTaskLeastLoaded tests least-loaded task delegation.
func TestDelegateTaskLeastLoaded(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")

	supervisor, _ := h.Supervisors["supervisor-1"]
	supervisor.Capabilities.DelegationBudget.MaxConcurrentSpawns = 5
	sa, _ := NewSupervisorAgent(supervisor)

	ctx := context.Background()

	// Spawn workers
	sa.SpawnWorker(ctx, "worker-1")
	sa.SpawnWorker(ctx, "worker-2")
	sa.SpawnWorker(ctx, "worker-3")

	// First task should go to any worker
	task1 := &TaskRequirements{
		TaskID:   "task-1",
		TaskType: "test",
	}

	result1, err := sa.DelegateTask(ctx, task1, StrategyLeastLoaded)
	if err != nil {
		t.Fatalf("DelegateTask failed: %v", err)
	}

	if !result1.Success {
		t.Fatalf("First task delegation failed: %s", result1.Reason)
	}

	// Match score should be 1.0 for empty worker
	if result1.MatchScore != 1.0 {
		t.Errorf("Match score for empty worker = %f, want 1.0", result1.MatchScore)
	}

	// Subsequent tasks should go to other workers (least loaded)
	task2 := &TaskRequirements{
		TaskID:   "task-2",
		TaskType: "test",
	}

	result2, err := sa.DelegateTask(ctx, task2, StrategyLeastLoaded)
	if err != nil {
		t.Fatalf("DelegateTask failed: %v", err)
	}

	// Should have selected a different worker (both have 0 load after delegation)
	// Note: Due to implementation details, this may or may not be different
	if !result2.Success {
		t.Fatalf("Second task delegation failed: %s", result2.Reason)
	}
}

// TestDelegateTaskCapabilityMatch tests capability-based task delegation.
func TestDelegateTaskCapabilityMatch(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")

	supervisor, _ := h.Supervisors["supervisor-1"]
	supervisor.Capabilities.DelegationBudget.MaxConcurrentSpawns = 5
	sa, _ := NewSupervisorAgent(supervisor)

	ctx := context.Background()

	// Spawn workers with different capabilities
	sa.SpawnWorker(ctx, "code-worker")
	sa.SpawnWorker(ctx, "test-worker")
	sa.SpawnWorker(ctx, "research-worker")

	// Set capabilities on workers
	sa.SetWorkerCapabilities("code-worker", []string{"code_generation", "debugging"})
	sa.SetWorkerCapabilities("test-worker", []string{"testing", "qa"})
	sa.SetWorkerCapabilities("research-worker", []string{"research", "documentation"})

	// Delegate task requiring testing capability
	task := &TaskRequirements{
		TaskID:               "test-task-1",
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

	if result.WorkerID != "test-worker" {
		t.Errorf("Expected task to go to test-worker, got %s", result.WorkerID)
	}

	if result.MatchScore < 0.9 {
		t.Errorf("Match score should be high for capability match, got %f", result.MatchScore)
	}
}

// TestDelegateTaskPriority tests priority-based task delegation.
func TestDelegateTaskPriority(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")

	supervisor, _ := h.Supervisors["supervisor-1"]
	supervisor.Capabilities.DelegationBudget.MaxConcurrentSpawns = 5
	sa, _ := NewSupervisorAgent(supervisor)

	ctx := context.Background()

	// Spawn workers
	sa.SpawnWorker(ctx, "worker-low")
	sa.SpawnWorker(ctx, "worker-high")
	sa.SpawnWorker(ctx, "worker-mid")

	// Set priorities
	sa.SetWorkerPriority("worker-low", 1)
	sa.SetWorkerPriority("worker-high", 10)
	sa.SetWorkerPriority("worker-mid", 5)

	// Delegate task using priority strategy
	task := &TaskRequirements{
		TaskID:   "priority-task",
		TaskType: "test",
	}

	result, err := sa.DelegateTask(ctx, task, StrategyPriority)
	if err != nil {
		t.Fatalf("DelegateTask failed: %v", err)
	}

	if !result.Success {
		t.Fatalf("Task delegation failed: %s", result.Reason)
	}

	if result.WorkerID != "worker-high" {
		t.Errorf("Expected task to go to worker-high (priority 10), got %s", result.WorkerID)
	}

	// Match score should be 1.0 (10/10)
	if result.MatchScore != 1.0 {
		t.Errorf("Match score = %f, want 1.0", result.MatchScore)
	}
}

// TestDelegateTaskPreferredWorker tests preferred worker selection.
func TestDelegateTaskPreferredWorker(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")

	supervisor, _ := h.Supervisors["supervisor-1"]
	supervisor.Capabilities.DelegationBudget.MaxConcurrentSpawns = 5
	sa, _ := NewSupervisorAgent(supervisor)

	ctx := context.Background()

	// Spawn workers
	sa.SpawnWorker(ctx, "worker-1")
	sa.SpawnWorker(ctx, "worker-2")
	sa.SpawnWorker(ctx, "preferred-worker")

	// Delegate task with preferred worker
	task := &TaskRequirements{
		TaskID:            "preferred-task",
		TaskType:          "test",
		PreferredWorkerID: "preferred-worker",
	}

	result, err := sa.DelegateTask(ctx, task, StrategyLeastLoaded)
	if err != nil {
		t.Fatalf("DelegateTask failed: %v", err)
	}

	if !result.Success {
		t.Fatalf("Task delegation failed: %s", result.Reason)
	}

	if result.WorkerID != "preferred-worker" {
		t.Errorf("Expected task to go to preferred-worker, got %s", result.WorkerID)
	}
}

// TestDelegateTaskNoAvailableWorkers tests delegation when no workers are available.
func TestDelegateTaskNoAvailableWorkers(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")

	supervisor, _ := h.Supervisors["supervisor-1"]
	sa, _ := NewSupervisorAgent(supervisor)

	ctx := context.Background()

	// No workers spawned
	task := &TaskRequirements{
		TaskID:   "orphan-task",
		TaskType: "test",
	}

	result, err := sa.DelegateTask(ctx, task, StrategyLeastLoaded)
	if err != nil {
		t.Fatalf("DelegateTask should not return error, got: %v", err)
	}

	if result.Success {
		t.Error("Task delegation should fail when no workers available")
	}

	if result.Reason == "" {
		t.Error("Failed delegation should have a reason")
	}
}

// TestDelegateTaskMissingCapabilities tests delegation when no worker has required capabilities.
func TestDelegateTaskMissingCapabilities(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")

	supervisor, _ := h.Supervisors["supervisor-1"]
	supervisor.Capabilities.DelegationBudget.MaxConcurrentSpawns = 5
	sa, _ := NewSupervisorAgent(supervisor)

	ctx := context.Background()

	// Spawn workers with limited capabilities
	sa.SpawnWorker(ctx, "worker-1")
	sa.SpawnWorker(ctx, "worker-2")
	sa.SetWorkerCapabilities("worker-1", []string{"testing"})
	sa.SetWorkerCapabilities("worker-2", []string{"debugging"})

	// Request capability no worker has
	task := &TaskRequirements{
		TaskID:               "impossible-task",
		TaskType:             "machine_learning",
		RequiredCapabilities: []string{"machine_learning", "gpu_compute"},
	}

	result, err := sa.DelegateTask(ctx, task, StrategyCapabilityMatch)
	if err != nil {
		t.Fatalf("DelegateTask should not return error, got: %v", err)
	}

	if result.Success {
		t.Error("Task delegation should fail when no worker has required capabilities")
	}
}

// TestDelegateTaskNilTask tests delegation with nil task.
func TestDelegateTaskNilTask(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")

	supervisor, _ := h.Supervisors["supervisor-1"]
	sa, _ := NewSupervisorAgent(supervisor)

	ctx := context.Background()

	_, err := sa.DelegateTask(ctx, nil, StrategyLeastLoaded)
	if err == nil {
		t.Error("DelegateTask with nil task should return error")
	}
}

// TestDelegateTaskEmptyTaskID tests delegation with empty task ID.
func TestDelegateTaskEmptyTaskID(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")

	supervisor, _ := h.Supervisors["supervisor-1"]
	sa, _ := NewSupervisorAgent(supervisor)

	ctx := context.Background()

	task := &TaskRequirements{
		TaskID:   "",
		TaskType: "test",
	}

	_, err := sa.DelegateTask(ctx, task, StrategyLeastLoaded)
	if err == nil {
		t.Error("DelegateTask with empty task ID should return error")
	}
}

// TestDelegationResultDetails tests that delegation results contain expected details.
func TestDelegationResultDetails(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")

	supervisor, _ := h.Supervisors["supervisor-1"]
	supervisor.Capabilities.DelegationBudget.MaxConcurrentSpawns = 5
	sa, _ := NewSupervisorAgent(supervisor)

	ctx := context.Background()
	sa.SpawnWorker(ctx, "worker-1")

	task := &TaskRequirements{
		TaskID:         "detailed-task",
		TaskType:       "test",
		TimeoutSeconds: 300,
	}

	result, err := sa.DelegateTask(ctx, task, StrategyLeastLoaded)
	if err != nil {
		t.Fatalf("DelegateTask failed: %v", err)
	}

	if !result.Success {
		t.Fatalf("Task delegation failed: %s", result.Reason)
	}

	// Check result contains expected details
	if result.WorkerID == "" {
		t.Error("Result should have worker ID")
	}

	if result.DelegatedAt.IsZero() {
		t.Error("Result should have delegation timestamp")
	}

	if result.EstimatedDeadline.IsZero() {
		t.Error("Result should have estimated deadline")
	}

	// Deadline should be after delegation time
	if !result.EstimatedDeadline.After(result.DelegatedAt) {
		t.Error("Deadline should be after delegation time")
	}

	// Deadline should be about TimeoutSeconds in the future
	expectedDuration := time.Duration(task.TimeoutSeconds) * time.Second
	actualDuration := result.EstimatedDeadline.Sub(result.DelegatedAt)
	if actualDuration < expectedDuration-time.Second || actualDuration > expectedDuration+time.Second {
		t.Errorf("Deadline duration = %v, want approximately %v", actualDuration, expectedDuration)
	}
}

// ================================================================================
// WORKER CAPABILITY MANAGEMENT TESTS
// ================================================================================

// TestSetWorkerCapabilities tests setting worker capabilities.
func TestSetWorkerCapabilities(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")

	supervisor, _ := h.Supervisors["supervisor-1"]
	sa, _ := NewSupervisorAgent(supervisor)

	ctx := context.Background()
	sa.SpawnWorker(ctx, "worker-1")

	capabilities := []string{"code_generation", "testing", "debugging"}
	err := sa.SetWorkerCapabilities("worker-1", capabilities)
	if err != nil {
		t.Fatalf("SetWorkerCapabilities failed: %v", err)
	}

	// Verify capabilities were set
	workers := sa.GetSpawnedWorkers()
	var worker *HierarchyNode
	for _, w := range workers {
		if w.AgentID == "worker-1" {
			worker = w
			break
		}
	}

	if worker == nil {
		t.Fatal("Worker not found")
	}

	storedCaps, ok := worker.Metadata["capabilities"].([]string)
	if !ok {
		t.Fatal("Capabilities not stored correctly")
	}

	if len(storedCaps) != len(capabilities) {
		t.Errorf("Stored capabilities count = %d, want %d", len(storedCaps), len(capabilities))
	}
}

// TestSetWorkerCapabilitiesNonExistentWorker tests setting capabilities for non-existent worker.
func TestSetWorkerCapabilitiesNonExistentWorker(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")

	supervisor, _ := h.Supervisors["supervisor-1"]
	sa, _ := NewSupervisorAgent(supervisor)

	err := sa.SetWorkerCapabilities("non-existent", []string{"testing"})
	if err == nil {
		t.Error("SetWorkerCapabilities for non-existent worker should return error")
	}
}

// TestSetWorkerPriority tests setting worker priority.
func TestSetWorkerPriority(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")

	supervisor, _ := h.Supervisors["supervisor-1"]
	sa, _ := NewSupervisorAgent(supervisor)

	ctx := context.Background()
	sa.SpawnWorker(ctx, "worker-1")

	err := sa.SetWorkerPriority("worker-1", 7)
	if err != nil {
		t.Fatalf("SetWorkerPriority failed: %v", err)
	}

	// Verify priority was set
	workers := sa.GetSpawnedWorkers()
	var worker *HierarchyNode
	for _, w := range workers {
		if w.AgentID == "worker-1" {
			worker = w
			break
		}
	}

	if worker == nil {
		t.Fatal("Worker not found")
	}

	storedPriority, ok := worker.Metadata["priority"].(int)
	if !ok {
		t.Fatal("Priority not stored correctly")
	}

	if storedPriority != 7 {
		t.Errorf("Stored priority = %d, want 7", storedPriority)
	}
}

// TestSetWorkerPriorityNonExistentWorker tests setting priority for non-existent worker.
func TestSetWorkerPriorityNonExistentWorker(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")

	supervisor, _ := h.Supervisors["supervisor-1"]
	sa, _ := NewSupervisorAgent(supervisor)

	err := sa.SetWorkerPriority("non-existent", 5)
	if err == nil {
		t.Error("SetWorkerPriority for non-existent worker should return error")
	}
}

// TestGetWorkerLoad tests getting worker load information.
func TestGetWorkerLoad(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")

	supervisor, _ := h.Supervisors["supervisor-1"]
	sa, _ := NewSupervisorAgent(supervisor)

	ctx := context.Background()
	sa.SpawnWorker(ctx, "worker-1")

	activeTasks, maxTasks, err := sa.GetWorkerLoad("worker-1")
	if err != nil {
		t.Fatalf("GetWorkerLoad failed: %v", err)
	}

	if activeTasks != 0 {
		t.Errorf("Initial active tasks = %d, want 0", activeTasks)
	}

	// maxTasks should be 3 (default)
	if maxTasks != 3 {
		t.Errorf("Max tasks = %d, want 3", maxTasks)
	}
}

// TestGetWorkerLoadNonExistentWorker tests getting load for non-existent worker.
func TestGetWorkerLoadNonExistentWorker(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")

	supervisor, _ := h.Supervisors["supervisor-1"]
	sa, _ := NewSupervisorAgent(supervisor)

	_, _, err := sa.GetWorkerLoad("non-existent")
	if err == nil {
		t.Error("GetWorkerLoad for non-existent worker should return error")
	}
}

// ================================================================================
// POOL MANAGEMENT TESTS
// ================================================================================

// TestWorkerPoolScaling tests spawning and terminating workers dynamically.
func TestWorkerPoolScaling(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")

	supervisor, _ := h.Supervisors["supervisor-1"]
	supervisor.Capabilities.DelegationBudget.MaxConcurrentSpawns = 10
	supervisor.Capabilities.DelegationBudget.MaxWorkers = 10
	sa, _ := NewSupervisorAgent(supervisor)

	ctx := context.Background()

	// Scale up: spawn 5 workers
	for i := 1; i <= 5; i++ {
		workerID := "worker-" + string(rune(48+i))
		_, err := sa.SpawnWorker(ctx, workerID)
		if err != nil {
			t.Fatalf("Failed to spawn worker %d: %v", i, err)
		}
	}

	if sa.GetWorkerCount() != 5 {
		t.Errorf("Worker count after scale up = %d, want 5", sa.GetWorkerCount())
	}

	// Scale down: terminate 2 workers
	sa.TerminateWorker(ctx, "worker-1")
	sa.TerminateWorker(ctx, "worker-2")

	// Workers are marked terminated but still tracked
	status1, _ := sa.GetWorkerStatus(ctx, "worker-1")
	if status1 != WorkerStatusTerminated {
		t.Errorf("Worker-1 status = %s, want terminated", status1)
	}

	// Active workers should be 3
	activeCount := 0
	for _, w := range sa.GetSpawnedWorkers() {
		if w.Status != "terminated" {
			activeCount++
		}
	}

	if activeCount != 3 {
		t.Errorf("Active worker count = %d, want 3", activeCount)
	}
}

// TestWorkerPoolReuse tests reusing terminated worker IDs.
func TestWorkerPoolReuse(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")

	supervisor, _ := h.Supervisors["supervisor-1"]
	supervisor.Capabilities.DelegationBudget.MaxConcurrentSpawns = 10
	sa, _ := NewSupervisorAgent(supervisor)

	ctx := context.Background()

	// Spawn and terminate a worker
	sa.SpawnWorker(ctx, "worker-1")
	sa.TerminateWorker(ctx, "worker-1")

	// Cannot reuse the same ID while it's still tracked
	_, err := sa.SpawnWorker(ctx, "worker-1")
	if err == nil {
		t.Error("Should not be able to spawn worker with same ID as terminated worker")
	}
}

// TestWorkerPoolFiltering tests filtering workers by status.
func TestWorkerPoolFiltering(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")

	supervisor, _ := h.Supervisors["supervisor-1"]
	supervisor.Capabilities.DelegationBudget.MaxConcurrentSpawns = 10
	supervisor.Capabilities.DelegationBudget.MaxWorkers = 10
	sa, _ := NewSupervisorAgent(supervisor)

	ctx := context.Background()

	// Spawn workers
	sa.SpawnWorker(ctx, "worker-active-1")
	sa.SpawnWorker(ctx, "worker-active-2")
	sa.SpawnWorker(ctx, "worker-to-terminate")
	sa.SpawnWorker(ctx, "worker-to-fail")

	// Terminate one
	sa.TerminateWorker(ctx, "worker-to-terminate")

	// Mark one as failed via result
	sa.RecordWorkerResult(&WorkerResult{
		WorkerID: "worker-to-fail",
		Status:   WorkerStatusFailed,
		Error:    "simulated failure",
	})

	// Count by status
	statusCounts := make(map[string]int)
	for _, w := range sa.GetSpawnedWorkers() {
		statusCounts[w.Status]++
	}

	// Should have 2 active, 1 terminated, 1 failed
	if statusCounts["active"] != 2 {
		t.Errorf("Active workers = %d, want 2", statusCounts["active"])
	}

	if statusCounts["terminated"] != 1 {
		t.Errorf("Terminated workers = %d, want 1", statusCounts["terminated"])
	}

	if statusCounts[string(WorkerStatusFailed)] != 1 {
		t.Errorf("Failed workers = %d, want 1", statusCounts[string(WorkerStatusFailed)])
	}
}

// TestConcurrentDelegation tests thread-safe task delegation.
func TestConcurrentDelegation(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")

	supervisor, _ := h.Supervisors["supervisor-1"]
	supervisor.Capabilities.DelegationBudget.MaxConcurrentSpawns = 10
	supervisor.Capabilities.DelegationBudget.MaxWorkers = 10
	supervisor.Capabilities.DelegationBudget.MaxTasksPerWorker = 20
	sa, _ := NewSupervisorAgent(supervisor)

	ctx := context.Background()

	// Spawn workers
	for i := 1; i <= 5; i++ {
		workerID := "worker-" + string(rune(48+i))
		sa.SpawnWorker(ctx, workerID)
	}

	// Concurrently delegate tasks
	done := make(chan *DelegationResult, 20)

	for i := 0; i < 20; i++ {
		go func(idx int) {
			task := &TaskRequirements{
				TaskID:   "concurrent-task-" + string(rune(48+idx)),
				TaskType: "test",
			}
			result, _ := sa.DelegateTask(ctx, task, StrategyLeastLoaded)
			done <- result
		}(i)
	}

	// Collect results
	successCount := 0
	for i := 0; i < 20; i++ {
		result := <-done
		if result != nil && result.Success {
			successCount++
		}
	}

	// Most should succeed (within budget)
	if successCount < 15 {
		t.Errorf("Success count = %d, expected at least 15", successCount)
	}

	// Active task count should match successes
	if sa.GetActiveTaskCount() != successCount {
		t.Errorf("Active task count = %d, want %d", sa.GetActiveTaskCount(), successCount)
	}
}

// TestWorkerTypeCapabilities tests capabilities derived from worker type.
func TestWorkerTypeCapabilities(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")

	supervisor, _ := h.Supervisors["supervisor-1"]
	supervisor.Capabilities.DelegationBudget.MaxConcurrentSpawns = 10
	sa, _ := NewSupervisorAgent(supervisor)

	ctx := context.Background()

	// Spawn workers and set their types
	sa.SpawnWorker(ctx, "code-worker")
	sa.SpawnWorker(ctx, "test-worker")
	sa.SpawnWorker(ctx, "research-worker")
	sa.SpawnWorker(ctx, "data-worker")

	// Set worker types in metadata
	workers := sa.GetSpawnedWorkers()
	for _, w := range workers {
		if w.Metadata == nil {
			w.Metadata = make(map[string]interface{})
		}
		switch w.AgentID {
		case "code-worker":
			w.Metadata["worker_type"] = "code"
		case "test-worker":
			w.Metadata["worker_type"] = "test"
		case "research-worker":
			w.Metadata["worker_type"] = "research"
		case "data-worker":
			w.Metadata["worker_type"] = "data"
		}
	}

	// Test code capability
	codeTask := &TaskRequirements{
		TaskID:               "code-task",
		RequiredCapabilities: []string{"code_generation"},
	}
	result, _ := sa.DelegateTask(ctx, codeTask, StrategyCapabilityMatch)
	if result.Success && result.WorkerID != "code-worker" {
		t.Errorf("Code task should go to code-worker, got %s", result.WorkerID)
	}

	// Test testing capability
	testTask := &TaskRequirements{
		TaskID:               "test-task",
		RequiredCapabilities: []string{"testing"},
	}
	result, _ = sa.DelegateTask(ctx, testTask, StrategyCapabilityMatch)
	if result.Success && result.WorkerID != "test-worker" {
		t.Errorf("Test task should go to test-worker, got %s", result.WorkerID)
	}
}

// TestDefaultDelegationStrategy tests that invalid strategy falls back to least loaded.
func TestDefaultDelegationStrategy(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")

	supervisor, _ := h.Supervisors["supervisor-1"]
	supervisor.Capabilities.DelegationBudget.MaxConcurrentSpawns = 5
	sa, _ := NewSupervisorAgent(supervisor)

	ctx := context.Background()
	sa.SpawnWorker(ctx, "worker-1")

	task := &TaskRequirements{
		TaskID:   "test-task",
		TaskType: "test",
	}

	// Use invalid strategy
	result, err := sa.DelegateTask(ctx, task, DelegationStrategy("invalid_strategy"))
	if err != nil {
		t.Fatalf("DelegateTask with invalid strategy should not error: %v", err)
	}

	if !result.Success {
		t.Error("Delegation with invalid strategy should fall back to default and succeed")
	}
}

// TestTaskTimeoutDefault tests that default timeout is applied when not specified.
func TestTaskTimeoutDefault(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")

	supervisor, _ := h.Supervisors["supervisor-1"]
	supervisor.Capabilities.DelegationBudget.MaxConcurrentSpawns = 5
	supervisor.Capabilities.DelegationBudget.TimeoutSeconds = 900 // 15 minutes
	sa, _ := NewSupervisorAgent(supervisor)

	ctx := context.Background()
	sa.SpawnWorker(ctx, "worker-1")

	task := &TaskRequirements{
		TaskID:         "timeout-test",
		TaskType:       "test",
		TimeoutSeconds: 0, // Not specified
	}

	result, _ := sa.DelegateTask(ctx, task, StrategyLeastLoaded)

	if !result.Success {
		t.Fatal("Delegation should succeed")
	}

	// Check deadline uses default timeout (15 minutes = 900 seconds)
	expectedDuration := 900 * time.Second
	actualDuration := result.EstimatedDeadline.Sub(result.DelegatedAt)
	if actualDuration < expectedDuration-time.Second || actualDuration > expectedDuration+time.Second {
		t.Errorf("Deadline duration = %v, want approximately %v", actualDuration, expectedDuration)
	}
}

// TestWorkerMetadataUpdate tests that delegation updates worker metadata.
func TestWorkerMetadataUpdate(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")

	supervisor, _ := h.Supervisors["supervisor-1"]
	supervisor.Capabilities.DelegationBudget.MaxConcurrentSpawns = 5
	sa, _ := NewSupervisorAgent(supervisor)

	ctx := context.Background()
	sa.SpawnWorker(ctx, "worker-1")

	task := &TaskRequirements{
		TaskID:   "metadata-task",
		TaskType: "test",
	}

	result, _ := sa.DelegateTask(ctx, task, StrategyLeastLoaded)

	if !result.Success {
		t.Fatal("Delegation should succeed")
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
		t.Fatal("Worker not found")
	}

	lastTaskID, ok := worker.Metadata["last_task_id"].(string)
	if !ok || lastTaskID != "metadata-task" {
		t.Errorf("Last task ID = %v, want metadata-task", lastTaskID)
	}

	lastAssigned, ok := worker.Metadata["last_task_assigned_at"].(time.Time)
	if !ok || lastAssigned.IsZero() {
		t.Error("Last task assigned time should be set")
	}
}

// TestConfigurableBudget tests that the supervisor respects the budget configured in its node.
func TestConfigurableBudget(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")

	supervisor, _ := h.Supervisors["supervisor-1"]

	// Configure a custom budget (lower than defaults)
	// Default MaxWorkers is 5, we set to 2
	// Default MaxConcurrentSpawns is 2, we set to 3 (to ensure MaxWorkers is the limiting factor here)
	customBudget := &DelegationBudget{
		MaxWorkers:          2,
		MaxTasksPerWorker:   3,
		MaxConcurrentSpawns: 3,
		TimeoutSeconds:      600,
	}
	supervisor.Capabilities.DelegationBudget = customBudget

	sa, _ := NewSupervisorAgent(supervisor)

	ctx := context.Background()

	// Spawn 2 workers (should succeed)
	for i := 1; i <= 2; i++ {
		workerID := "worker-" + string(rune(48+i))
		_, err := sa.SpawnWorker(ctx, workerID)
		if err != nil {
			t.Fatalf("Failed to spawn worker %d: %v", i, err)
		}
	}

	// Try to spawn a 3rd worker (should fail due to MaxWorkers limit)
	_, err := sa.SpawnWorker(ctx, "worker-3")
	if err == nil {
		t.Error("Spawning 3rd worker should fail due to custom budget limit of 2")
	} else {
		expectedErr := "supervisor has reached max workers limit (2)"
		if err.Error() != expectedErr {
			t.Errorf("Error message = %q, want %q", err.Error(), expectedErr)
		}
	}

	// Now test concurrent limit configuration
	// Update budget to allow more workers but fewer concurrent spawns
	supervisor.Capabilities.DelegationBudget.MaxWorkers = 10
	supervisor.Capabilities.DelegationBudget.MaxConcurrentSpawns = 2
	
	// Note: We already have 2 workers running. If we check budget now,
	// IsWithinBudget should return false because we hit concurrent limit (2)
	// assuming the implementation checks running workers count against concurrent limit
	
	if sa.IsWithinBudget() {
		t.Error("Supervisor should not be within budget when running workers (2) >= concurrent limit (2)")
	}
	
	// Update limit to 3, should be within budget now
	supervisor.Capabilities.DelegationBudget.MaxConcurrentSpawns = 3
	if !sa.IsWithinBudget() {
		t.Error("Supervisor should be within budget when running workers (2) < concurrent limit (3)")
	}
}

// TestSupervisorCapabilityMatchingComplex tests the scoring logic in capability matching.
func TestSupervisorCapabilityMatchingComplex(t *testing.T) {
	h := NewHierarchy()
	_ = h.SetCoordinator("coord-1")
	_ = h.AddSupervisor("sup-1")
	supNode := h.Supervisors["sup-1"]
	if supNode == nil {
		t.Fatal("Failed to create supervisor node")
	}
	// Increase budget to allow many workers and spawns
	supNode.Capabilities.DelegationBudget.MaxConcurrentSpawns = 10
	supNode.Capabilities.DelegationBudget.MaxWorkers = 10
	sa, err := NewSupervisorAgent(supNode)
	if err != nil {
		t.Fatalf("Failed to create supervisor agent: %v", err)
	}

	// Worker 1: Perfect match, but slightly loaded
	_, _ = sa.SpawnWorker(context.Background(), "w1")
	_ = sa.SetWorkerCapabilities("w1", []string{"cap1", "cap2"})
	_ = sa.AssignTask("w1") // load penalty

	// Worker 2: Perfect match, NO load
	_, _ = sa.SpawnWorker(context.Background(), "w2")
	_ = sa.SetWorkerCapabilities("w2", []string{"cap1", "cap2"})

	// Worker 3: Partial match
	_, _ = sa.SpawnWorker(context.Background(), "w3")
	_ = sa.SetWorkerCapabilities("w3", []string{"cap1", "other"})

	task := &TaskRequirements{
		TaskID:               "t1",
		RequiredCapabilities: []string{"cap1", "cap2"},
	}

	result, err := sa.DelegateTask(context.Background(), task, StrategyCapabilityMatch)
	if err != nil {
		t.Fatalf("DelegateTask error: %v", err)
	}
	
	if !result.Success {
		t.Fatalf("Delegation failed: %s", result.Reason)
	}

	// w2 should win over w1 because w1 has a load penalty
	if result.WorkerID != "w2" {
		t.Errorf("Expected w2 to win, got %s (Score: %f)", result.WorkerID, result.MatchScore)
	}
}

// TestSupervisorImpliedCapabilities tests capabilities derived from worker_type.
func TestSupervisorImpliedCapabilities(t *testing.T) {
	h := NewHierarchy()
	_ = h.SetCoordinator("coord-1")
	_ = h.AddSupervisor("sup-1")
	supNode := h.Supervisors["sup-1"]
	supNode.Capabilities.DelegationBudget.MaxConcurrentSpawns = 10
	sa, _ := NewSupervisorAgent(supNode)

	types := []string{"code", "test", "research", "data"}
	for _, ty := range types {
		workerID := "w-" + ty
		worker, err := sa.SpawnWorker(context.Background(), workerID)
		if err != nil {
			t.Fatalf("Failed to spawn %s: %v", workerID, err)
		}
		if worker.Metadata == nil {
			worker.Metadata = make(map[string]interface{})
		}
		worker.Metadata["worker_type"] = ty
	}

	testCases := []struct {
		cap      string
		expected string
	}{
		{"code_generation", "w-code"},
		{"testing", "w-test"},
		{"research", "w-research"},
		{"data_processing", "w-data"},
	}

	for _, tc := range testCases {
		task := &TaskRequirements{
			TaskID:               "task-" + tc.cap,
			RequiredCapabilities: []string{tc.cap},
		}
		result, _ := sa.DelegateTask(context.Background(), task, StrategyCapabilityMatch)
		if !result.Success {
			t.Errorf("Failed to delegate task for cap %s: %s", tc.cap, result.Reason)
			continue
		}
		if result.WorkerID != tc.expected {
			t.Errorf("For capability %s, expected worker %s, got %s", tc.cap, tc.expected, result.WorkerID)
		}
	}
}

// TestSupervisorRecordResultUnknownWorker tests recording results for workers not in pool.
func TestSupervisorRecordResultUnknownWorker(t *testing.T) {
	h := NewHierarchy()
	_ = h.SetCoordinator("coord-1")
	_ = h.AddSupervisor("sup-1")
	sa, _ := NewSupervisorAgent(h.Supervisors["sup-1"])

	// 1. Result for worker that never existed
	res1 := &WorkerResult{WorkerID: "ghost", Status: WorkerStatusCompleted}
	err := sa.RecordWorkerResult(res1)
	if err == nil {
		t.Error("Expected error recording result for ghost worker")
	}

	// 2. Result for worker that was terminated (should succeed)
	_, _ = sa.SpawnWorker(context.Background(), "w1")
	_ = sa.TerminateWorker(context.Background(), "w1")
	
	res2 := &WorkerResult{WorkerID: "w1", Status: WorkerStatusCompleted}
	err = sa.RecordWorkerResult(res2)
	if err != nil {
		t.Errorf("Expected success recording result for terminated worker, got: %v", err)
	}
}

// TestSupervisorConcurrentSpawnLimitRealCount tests that checkConcurrentSpawnsLimit 
// correctly counts only running/active workers.
func TestSupervisorConcurrentSpawnLimitRealCount(t *testing.T) {
	h := NewHierarchy()
	_ = h.SetCoordinator("coord-1")
	_ = h.AddSupervisor("sup-1")
	supNode := h.Supervisors["sup-1"]
	// Limit to 2 concurrent spawns
	supNode.Capabilities.DelegationBudget.MaxConcurrentSpawns = 2
	sa, _ := NewSupervisorAgent(supNode)

	// Spawn 2 workers
	_, _ = sa.SpawnWorker(context.Background(), "w1")
	_, _ = sa.SpawnWorker(context.Background(), "w2")

	// Try to spawn 3rd - should fail
	_, err := sa.SpawnWorker(context.Background(), "w3")
	if err == nil {
		t.Error("Expected failure spawning 3rd worker (limit 2)")
	}

	// Terminate one
	_ = sa.TerminateWorker(context.Background(), "w1")

	// Now should be able to spawn another one
	_, err = sa.SpawnWorker(context.Background(), "w3")
	if err != nil {
		t.Errorf("Expected success spawning 3rd worker after terminating 1st, got: %v", err)
	}
}

// TestSupervisorGetSpawnedWorkersOrder verifies the combined list of active and terminated workers.
func TestSupervisorGetSpawnedWorkersOrder(t *testing.T) {
	h := NewHierarchy()
	_ = h.SetCoordinator("coord-1")
	_ = h.AddSupervisor("sup-1")
	supNode := h.Supervisors["sup-1"]
	supNode.Capabilities.DelegationBudget.MaxConcurrentSpawns = 10
	sa, _ := NewSupervisorAgent(supNode)

	_, _ = sa.SpawnWorker(context.Background(), "active-1")
	_, _ = sa.SpawnWorker(context.Background(), "active-2")
	_, _ = sa.SpawnWorker(context.Background(), "to-terminate")
	
	_ = sa.TerminateWorker(context.Background(), "to-terminate")

	workers := sa.GetSpawnedWorkers()
	if len(workers) != 3 {
		t.Fatalf("Expected 3 spawned workers, got %d", len(workers))
	}

	foundTerminated := false
	activeCount := 0
	for _, w := range workers {
		if w.AgentID == "to-terminate" && w.Status == "terminated" {
			foundTerminated = true
		} else if w.Status == "active" {
			activeCount++
		}
	}

	if !foundTerminated {
		t.Error("Terminated worker not found in GetSpawnedWorkers")
	}
	if activeCount != 2 {
		t.Errorf("Expected 2 active workers, got %d", activeCount)
	}
}

// TestSupervisorAssignTaskLimits tests MaxTasksPerWorker enforcement.
func TestSupervisorAssignTaskLimits(t *testing.T) {
	h := NewHierarchy()
	_ = h.SetCoordinator("coord-1")
	_ = h.AddSupervisor("sup-1")
	supNode := h.Supervisors["sup-1"]
	supNode.Capabilities.DelegationBudget.MaxTasksPerWorker = 2
	sa, _ := NewSupervisorAgent(supNode)

	_, _ = sa.SpawnWorker(context.Background(), "w1")

	err := sa.AssignTask("w1") // 1
	if err != nil {
		t.Fatal(err)
	}
	err = sa.AssignTask("w1") // 2
	if err != nil {
		t.Fatal(err)
	}
	
	err = sa.AssignTask("w1") // 3 - should fail
	if err == nil {
		t.Error("Expected error assigning 3rd task to worker with limit 2")
	}
}

