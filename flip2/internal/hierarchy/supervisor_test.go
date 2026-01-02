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

