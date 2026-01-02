package hierarchy

import (
	"context"
	"testing"
)

// TestDelegationStrategies covers the various task delegation strategies.
func TestDelegationStrategies(t *testing.T) {
	// Setup common hierarchy
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")
	supervisorNode, _ := h.Supervisors["supervisor-1"]

	// Ensure budget allows enough workers/tasks
	if supervisorNode.Capabilities == nil {
		supervisorNode.Capabilities = DefaultSupervisorCapabilities()
	}
	supervisorNode.Capabilities.DelegationBudget.MaxWorkers = 5
	supervisorNode.Capabilities.DelegationBudget.MaxConcurrentSpawns = 5
	supervisorNode.Capabilities.DelegationBudget.MaxTasksPerWorker = 5

	ctx := context.Background()

	t.Run("RoundRobin", func(t *testing.T) {
		sa, _ := NewSupervisorAgent(supervisorNode)
		
		// Spawn 3 workers
		workers := []string{"w-rr-1", "w-rr-2", "w-rr-3"}
		for _, id := range workers {
			if _, err := sa.SpawnWorker(ctx, id); err != nil {
				t.Fatalf("Failed to spawn worker %s: %v", id, err)
			}
		}

		// Delegate 5 tasks
		assignments := make(map[string]int)
		for i := 0; i < 5; i++ {
			task := &TaskRequirements{TaskID: "task-" + string(rune('0'+i)), TaskType: "test"}
			result, err := sa.DelegateTask(ctx, task, StrategyRoundRobin)
			if err != nil {
				t.Fatalf("DelegateTask failed: %v", err)
			}
			assignments[result.WorkerID]++
		}

		// Verify distribution (3 workers, 5 tasks -> 2, 2, 1 or similar)
		if len(assignments) != 3 {
			t.Errorf("Expected tasks distributed to 3 workers, got %d", len(assignments))
		}
	})

	t.Run("LeastLoaded", func(t *testing.T) {
		sa, _ := NewSupervisorAgent(supervisorNode)
		
		// Spawn 2 workers
		sa.SpawnWorker(ctx, "w-ll-1")
		sa.SpawnWorker(ctx, "w-ll-2")

		// Artificially load w-ll-1
		sa.AssignTask("w-ll-1")
		sa.AssignTask("w-ll-1")

		// Delegate next task
		task := &TaskRequirements{TaskID: "task-ll", TaskType: "test"}
		result, err := sa.DelegateTask(ctx, task, StrategyLeastLoaded)
		if err != nil {
			t.Fatalf("DelegateTask failed: %v", err)
		}

		// Should go to w-ll-2 (load 0 vs 2)
		if result.WorkerID != "w-ll-2" {
			t.Errorf("Expected assignment to w-ll-2 (least loaded), got %s", result.WorkerID)
		}
	})

	t.Run("CapabilityMatch", func(t *testing.T) {
		sa, _ := NewSupervisorAgent(supervisorNode)
		
		// Spawn workers with different capabilities
		sa.SpawnWorker(ctx, "w-cap-gen")
		sa.SetWorkerCapabilities("w-cap-gen", []string{"code_generation"})

		sa.SpawnWorker(ctx, "w-cap-test")
		sa.SetWorkerCapabilities("w-cap-test", []string{"testing"})

		// Delegate task requiring "testing"
		task := &TaskRequirements{
			TaskID:               "task-cap",
			TaskType:             "test",
			RequiredCapabilities: []string{"testing"},
		}

		result, err := sa.DelegateTask(ctx, task, StrategyCapabilityMatch)
		if err != nil {
			t.Fatalf("DelegateTask failed: %v", err)
		}

		if result.WorkerID != "w-cap-test" {
			t.Errorf("Expected assignment to w-cap-test, got %s", result.WorkerID)
		}
	})

	t.Run("Priority", func(t *testing.T) {
		sa, _ := NewSupervisorAgent(supervisorNode)
		
		sa.SpawnWorker(ctx, "w-prio-low")
		sa.SetWorkerPriority("w-prio-low", 1)

		sa.SpawnWorker(ctx, "w-prio-high")
		sa.SetWorkerPriority("w-prio-high", 10)

		task := &TaskRequirements{TaskID: "task-prio", TaskType: "test"}
		result, err := sa.DelegateTask(ctx, task, StrategyPriority)
		if err != nil {
			t.Fatalf("DelegateTask failed: %v", err)
		}

		if result.WorkerID != "w-prio-high" {
			t.Errorf("Expected assignment to w-prio-high, got %s", result.WorkerID)
		}
	})
}

// TestDelegationFailures covers scenarios where delegation should fail.
func TestDelegationFailures(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")
	supervisorNode, _ := h.Supervisors["supervisor-1"]
	
	ctx := context.Background()

	t.Run("NoAvailableWorkers", func(t *testing.T) {
		sa, _ := NewSupervisorAgent(supervisorNode)
		// Don't spawn any workers

		task := &TaskRequirements{TaskID: "task-fail-1", TaskType: "test"}
		result, err := sa.DelegateTask(ctx, task, StrategyLeastLoaded)
		
		// Expect no error, but success=false
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if result.Success {
			t.Error("Delegation should have failed with no workers")
		}
	})

	t.Run("AllWorkersBusy", func(t *testing.T) {
		sa, _ := NewSupervisorAgent(supervisorNode)
		
		// Max tasks per worker = 1
		supervisorNode.Capabilities.DelegationBudget.MaxTasksPerWorker = 1
		sa.syncPoolConfigLocked() // ensure config is picked up if needed (though max tasks is checked in code)

		sa.SpawnWorker(ctx, "w-busy")
		
		// Fill capacity
		sa.AssignTask("w-busy")

		// Try to delegate another
		task := &TaskRequirements{TaskID: "task-fail-2", TaskType: "test"}
		result, err := sa.DelegateTask(ctx, task, StrategyLeastLoaded)
		
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if result.Success {
			t.Error("Delegation should have failed when worker is busy")
		}
	})

	t.Run("MissingCapabilities", func(t *testing.T) {
		sa, _ := NewSupervisorAgent(supervisorNode)
		sa.SpawnWorker(ctx, "w-generic")
		
		task := &TaskRequirements{
			TaskID: "task-fail-3", 
			TaskType: "test",
			RequiredCapabilities: []string{"advanced_physics"},
		}

		result, err := sa.DelegateTask(ctx, task, StrategyCapabilityMatch)
		
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if result.Success {
			t.Error("Delegation should have failed for missing capability")
		}
	})
}

// TestWorkerPoolIntegration verifies interactions between Supervisor and WorkerPool.
func TestWorkerPoolIntegration(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")
	supervisorNode, _ := h.Supervisors["supervisor-1"]
	
	ctx := context.Background()

	t.Run("UnhealthyWorkerExclusion", func(t *testing.T) {
		sa, _ := NewSupervisorAgent(supervisorNode)
		
		sa.SpawnWorker(ctx, "w-healthy")
		sa.SpawnWorker(ctx, "w-unhealthy")

		// Mark one unhealthy via pool (accessed via reflection or helper if possible, 
		// but here we can rely on MarkWorkerUnhealthy if exposed, or simulated)
		// SupervisorAgent doesn't expose MarkWorkerUnhealthy directly, but it has a pool.
		// NOTE: SupervisorAgent struct fields are unexported in the package, but we are IN the package (test).
		sa.pool.MarkWorkerUnhealthy("w-unhealthy", "simulation")

		task := &TaskRequirements{TaskID: "task-pool-1", TaskType: "test"}
		result, err := sa.DelegateTask(ctx, task, StrategyLeastLoaded)
		
		if err != nil {
			t.Fatalf("DelegateTask failed: %v", err)
		}

		if result.WorkerID == "w-unhealthy" {
			t.Error("Task assigned to unhealthy worker")
		}
		if result.WorkerID != "w-healthy" {
			t.Errorf("Expected assignment to w-healthy, got %s", result.WorkerID)
		}
	})

	t.Run("TerminatedWorkerExclusion", func(t *testing.T) {
		sa, _ := NewSupervisorAgent(supervisorNode)
		
		sa.SpawnWorker(ctx, "w-active")
		sa.SpawnWorker(ctx, "w-term")

		// Terminate one
		sa.TerminateWorker(ctx, "w-term")

		task := &TaskRequirements{TaskID: "task-pool-2", TaskType: "test"}
		result, err := sa.DelegateTask(ctx, task, StrategyLeastLoaded)
		
		if err != nil {
			t.Fatalf("DelegateTask failed: %v", err)
		}

		if result.WorkerID == "w-term" {
			t.Error("Task assigned to terminated worker")
		}
	})
}

// TestBudgetEnforcement verifies strict budget limits.
func TestBudgetEnforcement(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")
	supervisorNode, _ := h.Supervisors["supervisor-1"]
	
	// Set strict limits
	supervisorNode.Capabilities.DelegationBudget.MaxWorkers = 2
	supervisorNode.Capabilities.DelegationBudget.MaxConcurrentSpawns = 2
	
	ctx := context.Background()
	sa, _ := NewSupervisorAgent(supervisorNode)

	// Spawn up to limit
	if _, err := sa.SpawnWorker(ctx, "w-1"); err != nil {
		t.Fatalf("Failed to spawn w-1: %v", err)
	}
	if _, err := sa.SpawnWorker(ctx, "w-2"); err != nil {
		t.Fatalf("Failed to spawn w-2: %v", err)
	}

	// Try to exceed MaxWorkers
	if _, err := sa.SpawnWorker(ctx, "w-3"); err == nil {
		t.Error("Should have failed to spawn w-3 (MaxWorkers exceeded)")
	}

	// Verify we can still delegate to existing workers
	task := &TaskRequirements{TaskID: "task-budget", TaskType: "test"}
	result, err := sa.DelegateTask(ctx, task, StrategyLeastLoaded)
	if err != nil || !result.Success {
		t.Errorf("Should be able to delegate despite max workers reached")
	}
}

// TestPreferredWorker tests targeting a specific worker.
func TestPreferredWorker(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")
	supervisorNode, _ := h.Supervisors["supervisor-1"]
	
	ctx := context.Background()
	sa, _ := NewSupervisorAgent(supervisorNode)

	sa.SpawnWorker(ctx, "w-generic")
	sa.SpawnWorker(ctx, "w-special")

	task := &TaskRequirements{
		TaskID: "task-pref", 
		TaskType: "test",
		PreferredWorkerID: "w-special",
	}

	result, err := sa.DelegateTask(ctx, task, StrategyLeastLoaded) // Strategy shouldn't matter
	if err != nil {
		t.Fatalf("DelegateTask failed: %v", err)
	}

	if result.WorkerID != "w-special" {
		t.Errorf("Expected assignment to w-special, got %s", result.WorkerID)
	}
}
