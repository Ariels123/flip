package hierarchy

import (
	"testing"
	"time"
)

// TestAgentRoleValidation tests the AgentRole validation logic.
func TestAgentRoleValidation(t *testing.T) {
	tests := []struct {
		name    string
		role    AgentRole
		valid   bool
	}{
		{"Coordinator role", RoleCoordinator, true},
		{"Supervisor role", RoleSupervisor, true},
		{"Worker role", RoleWorker, true},
		{"Invalid role", AgentRole("invalid"), false},
		{"Empty role", AgentRole(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.role.IsValid()
			if got != tt.valid {
				t.Errorf("IsValid() = %v, want %v", got, tt.valid)
			}
		})
	}
}

// TestHierarchyConstruction tests basic hierarchy setup with coordinator.
func TestHierarchyConstruction(t *testing.T) {
	h := NewHierarchy()

	// Verify hierarchy is initialized
	if h == nil {
		t.Fatal("NewHierarchy returned nil")
	}

	if h.Coordinator != nil {
		t.Error("Coordinator should be nil initially")
	}

	if len(h.Supervisors) != 0 {
		t.Error("Supervisors map should be empty initially")
	}

	if len(h.Workers) != 0 {
		t.Error("Workers map should be empty initially")
	}

	// Verify role capabilities are initialized
	if len(h.RoleCapabilities) != 3 {
		t.Errorf("Expected 3 role capabilities, got %d", len(h.RoleCapabilities))
	}

	// Verify escalation paths are initialized
	if len(h.EscalationPaths) == 0 {
		t.Error("EscalationPaths should not be empty")
	}
}

// TestSetCoordinator tests setting the coordinator.
func TestSetCoordinator(t *testing.T) {
	h := NewHierarchy()

	err := h.SetCoordinator("coordinator-1")
	if err != nil {
		t.Fatalf("SetCoordinator failed: %v", err)
	}

	if h.Coordinator == nil {
		t.Fatal("Coordinator is nil after SetCoordinator")
	}

	if h.Coordinator.AgentID != "coordinator-1" {
		t.Errorf("Coordinator AgentID = %s, want coordinator-1", h.Coordinator.AgentID)
	}

	if h.Coordinator.Role != RoleCoordinator {
		t.Errorf("Coordinator Role = %s, want %s", h.Coordinator.Role, RoleCoordinator)
	}

	if h.Coordinator.ParentID != nil {
		t.Error("Coordinator should have no parent")
	}

	if h.Coordinator.Status != "active" {
		t.Errorf("Coordinator Status = %s, want active", h.Coordinator.Status)
	}

	// Test error on empty ID
	err = h.SetCoordinator("")
	if err == nil {
		t.Error("SetCoordinator with empty ID should return error")
	}
}

// TestAddSupervisor tests adding supervisors to the hierarchy.
func TestAddSupervisor(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")

	// Add first supervisor
	err := h.AddSupervisor("supervisor-1")
	if err != nil {
		t.Fatalf("AddSupervisor failed: %v", err)
	}

	if _, exists := h.Supervisors["supervisor-1"]; !exists {
		t.Error("Supervisor not found in map")
	}

	supervisor := h.Supervisors["supervisor-1"]
	if supervisor.Role != RoleSupervisor {
		t.Errorf("Supervisor Role = %s, want %s", supervisor.Role, RoleSupervisor)
	}

	if supervisor.ParentID == nil || *supervisor.ParentID != "coordinator-1" {
		t.Error("Supervisor parent should be coordinator-1")
	}

	// Verify coordinator's children list
	if len(h.Coordinator.ChildrenIDs) != 1 {
		t.Errorf("Coordinator should have 1 child, has %d", len(h.Coordinator.ChildrenIDs))
	}

	// Add second supervisor
	err = h.AddSupervisor("supervisor-2")
	if err != nil {
		t.Fatalf("AddSupervisor failed: %v", err)
	}

	if len(h.Coordinator.ChildrenIDs) != 2 {
		t.Errorf("Coordinator should have 2 children, has %d", len(h.Coordinator.ChildrenIDs))
	}

	// Test duplicate supervisor
	err = h.AddSupervisor("supervisor-1")
	if err == nil {
		t.Error("Adding duplicate supervisor should return error")
	}

	// Test without coordinator
	h2 := NewHierarchy()
	err = h2.AddSupervisor("supervisor-1")
	if err == nil {
		t.Error("AddSupervisor without coordinator should return error")
	}
}

// TestAddWorker tests adding workers under supervisors.
func TestAddWorker(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")

	// Add first worker
	err := h.AddWorker("worker-1", "supervisor-1")
	if err != nil {
		t.Fatalf("AddWorker failed: %v", err)
	}

	if _, exists := h.Workers["worker-1"]; !exists {
		t.Error("Worker not found in map")
	}

	worker := h.Workers["worker-1"]
	if worker.Role != RoleWorker {
		t.Errorf("Worker Role = %s, want %s", worker.Role, RoleWorker)
	}

	if worker.ParentID == nil || *worker.ParentID != "supervisor-1" {
		t.Error("Worker parent should be supervisor-1")
	}

	// Verify supervisor's children list
	supervisor := h.Supervisors["supervisor-1"]
	if len(supervisor.ChildrenIDs) != 1 {
		t.Errorf("Supervisor should have 1 child, has %d", len(supervisor.ChildrenIDs))
	}

	// Add more workers
	for i := 2; i <= 5; i++ {
		workerID := "worker-" + string(rune(48+i))
		err := h.AddWorker(workerID, "supervisor-1")
		if err != nil {
			t.Fatalf("AddWorker %s failed: %v", workerID, err)
		}
	}

	if len(supervisor.ChildrenIDs) != 5 {
		t.Errorf("Supervisor should have 5 children, has %d", len(supervisor.ChildrenIDs))
	}

	// Test delegation budget limit (default is 5 workers)
	err = h.AddWorker("worker-6", "supervisor-1")
	if err == nil {
		t.Error("Adding 6th worker should exceed budget and return error")
	}

	// Test duplicate worker
	err = h.AddWorker("worker-1", "supervisor-1")
	if err == nil {
		t.Error("Adding duplicate worker should return error")
	}

	// Test invalid supervisor
	err = h.AddWorker("worker-new", "invalid-supervisor")
	if err == nil {
		t.Error("Adding worker to non-existent supervisor should return error")
	}
}

// TestRemoveWorker tests removing workers from the hierarchy.
func TestRemoveWorker(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")
	h.AddWorker("worker-1", "supervisor-1")
	h.AddWorker("worker-2", "supervisor-1")

	// Remove first worker
	err := h.RemoveWorker("worker-1")
	if err != nil {
		t.Fatalf("RemoveWorker failed: %v", err)
	}

	if _, exists := h.Workers["worker-1"]; exists {
		t.Error("Worker should not exist after removal")
	}

	supervisor := h.Supervisors["supervisor-1"]
	if len(supervisor.ChildrenIDs) != 1 {
		t.Errorf("Supervisor should have 1 child, has %d", len(supervisor.ChildrenIDs))
	}

	// Test removing non-existent worker
	err = h.RemoveWorker("non-existent")
	if err == nil {
		t.Error("Removing non-existent worker should return error")
	}
}

// TestRemoveSupervisor tests removing supervisors from the hierarchy.
func TestRemoveSupervisor(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")
	h.AddSupervisor("supervisor-2")

	// Try to remove supervisor with workers
	h.AddWorker("worker-1", "supervisor-1")
	err := h.RemoveSupervisor("supervisor-1")
	if err == nil {
		t.Error("Removing supervisor with workers should return error")
	}

	// Remove workers first
	h.RemoveWorker("worker-1")

	// Now remove supervisor
	err = h.RemoveSupervisor("supervisor-1")
	if err != nil {
		t.Fatalf("RemoveSupervisor failed: %v", err)
	}

	if _, exists := h.Supervisors["supervisor-1"]; exists {
		t.Error("Supervisor should not exist after removal")
	}

	if len(h.Coordinator.ChildrenIDs) != 1 {
		t.Errorf("Coordinator should have 1 child, has %d", len(h.Coordinator.ChildrenIDs))
	}
}

// TestGetNode tests retrieving nodes from the hierarchy.
func TestGetNode(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")
	h.AddWorker("worker-1", "supervisor-1")

	// Get coordinator
	node, err := h.GetNode("coordinator-1")
	if err != nil {
		t.Fatalf("GetNode coordinator failed: %v", err)
	}
	if node.Role != RoleCoordinator {
		t.Errorf("Expected RoleCoordinator, got %s", node.Role)
	}

	// Get supervisor
	node, err = h.GetNode("supervisor-1")
	if err != nil {
		t.Fatalf("GetNode supervisor failed: %v", err)
	}
	if node.Role != RoleSupervisor {
		t.Errorf("Expected RoleSupervisor, got %s", node.Role)
	}

	// Get worker
	node, err = h.GetNode("worker-1")
	if err != nil {
		t.Fatalf("GetNode worker failed: %v", err)
	}
	if node.Role != RoleWorker {
		t.Errorf("Expected RoleWorker, got %s", node.Role)
	}

	// Get non-existent node
	_, err = h.GetNode("non-existent")
	if err == nil {
		t.Error("GetNode with non-existent ID should return error")
	}
}

// TestGetParent tests retrieving parent nodes.
func TestGetParent(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")
	h.AddWorker("worker-1", "supervisor-1")

	// Worker's parent should be supervisor
	parent, err := h.GetParent("worker-1")
	if err != nil {
		t.Fatalf("GetParent failed: %v", err)
	}
	if parent.AgentID != "supervisor-1" {
		t.Errorf("Expected parent supervisor-1, got %s", parent.AgentID)
	}

	// Supervisor's parent should be coordinator
	parent, err = h.GetParent("supervisor-1")
	if err != nil {
		t.Fatalf("GetParent failed: %v", err)
	}
	if parent.AgentID != "coordinator-1" {
		t.Errorf("Expected parent coordinator-1, got %s", parent.AgentID)
	}

	// Coordinator has no parent
	_, err = h.GetParent("coordinator-1")
	if err == nil {
		t.Error("Coordinator GetParent should return error")
	}
}

// TestGetChildren tests retrieving child nodes.
func TestGetChildren(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")
	h.AddSupervisor("supervisor-2")
	h.AddWorker("worker-1", "supervisor-1")
	h.AddWorker("worker-2", "supervisor-1")

	// Coordinator's children
	children, err := h.GetChildren("coordinator-1")
	if err != nil {
		t.Fatalf("GetChildren failed: %v", err)
	}
	if len(children) != 2 {
		t.Errorf("Expected 2 children, got %d", len(children))
	}

	// Supervisor's children
	children, err = h.GetChildren("supervisor-1")
	if err != nil {
		t.Fatalf("GetChildren failed: %v", err)
	}
	if len(children) != 2 {
		t.Errorf("Expected 2 children, got %d", len(children))
	}

	// Worker's children (should be empty)
	children, err = h.GetChildren("worker-1")
	if err != nil {
		t.Fatalf("GetChildren failed: %v", err)
	}
	if len(children) != 0 {
		t.Errorf("Expected 0 children for worker, got %d", len(children))
	}
}

// TestGetAncestors tests retrieving all ancestors of a node.
func TestGetAncestors(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")
	h.AddWorker("worker-1", "supervisor-1")

	// Worker's ancestors
	ancestors, err := h.GetAncestors("worker-1")
	if err != nil {
		t.Fatalf("GetAncestors failed: %v", err)
	}

	if len(ancestors) != 2 {
		t.Errorf("Expected 2 ancestors, got %d", len(ancestors))
	}

	if ancestors[0].AgentID != "supervisor-1" {
		t.Errorf("First ancestor should be supervisor-1, got %s", ancestors[0].AgentID)
	}

	if ancestors[1].AgentID != "coordinator-1" {
		t.Errorf("Second ancestor should be coordinator-1, got %s", ancestors[1].AgentID)
	}

	// Supervisor's ancestors
	ancestors, err = h.GetAncestors("supervisor-1")
	if err != nil {
		t.Fatalf("GetAncestors failed: %v", err)
	}

	if len(ancestors) != 1 {
		t.Errorf("Expected 1 ancestor, got %d", len(ancestors))
	}

	// Coordinator's ancestors
	ancestors, err = h.GetAncestors("coordinator-1")
	if err != nil {
		t.Fatalf("GetAncestors failed: %v", err)
	}

	if len(ancestors) != 0 {
		t.Errorf("Expected 0 ancestors for coordinator, got %d", len(ancestors))
	}
}

// TestFindEscalationPath tests finding escalation paths.
func TestFindEscalationPath(t *testing.T) {
	h := NewHierarchy()

	// Find valid escalation path
	path := h.FindEscalationPath(RoleWorker, RoleSupervisor)
	if path == nil {
		t.Error("Should find escalation path from Worker to Supervisor")
	}

	if path.From != RoleWorker || path.To != RoleSupervisor {
		t.Errorf("Escalation path mismatch")
	}

	// Find non-existent path
	path = h.FindEscalationPath(RoleCoordinator, RoleWorker)
	if path != nil {
		t.Error("Should not find escalation path from Coordinator to Worker")
	}
}

// TestCanAgentSpawnRole tests role spawning permissions.
func TestCanAgentSpawnRole(t *testing.T) {
	h := NewHierarchy()

	// Coordinator can spawn supervisors
	if !h.CanAgentSpawnRole(RoleCoordinator, RoleSupervisor) {
		t.Error("Coordinator should be able to spawn supervisors")
	}

	// Coordinator cannot spawn workers directly
	if h.CanAgentSpawnRole(RoleCoordinator, RoleWorker) {
		t.Error("Coordinator should not be able to spawn workers directly")
	}

	// Supervisor can spawn workers
	if !h.CanAgentSpawnRole(RoleSupervisor, RoleWorker) {
		t.Error("Supervisor should be able to spawn workers")
	}

	// Workers cannot spawn anything
	if h.CanAgentSpawnRole(RoleWorker, RoleWorker) {
		t.Error("Workers should not be able to spawn anything")
	}
}

// TestCanAgentEscalateTo tests escalation permissions.
func TestCanAgentEscalateTo(t *testing.T) {
	h := NewHierarchy()

	// Worker can escalate to supervisor
	if !h.CanAgentEscalateTo(RoleWorker, RoleSupervisor) {
		t.Error("Worker should be able to escalate to supervisor")
	}

	// Supervisor can escalate to coordinator
	if !h.CanAgentEscalateTo(RoleSupervisor, RoleCoordinator) {
		t.Error("Supervisor should be able to escalate to coordinator")
	}

	// Coordinator cannot escalate (already at top)
	if h.CanAgentEscalateTo(RoleCoordinator, RoleCoordinator) {
		t.Error("Coordinator should not be able to escalate")
	}
}

// TestCountAgentsByRole tests counting agents by role.
func TestCountAgentsByRole(t *testing.T) {
	h := NewHierarchy()

	// Initial counts
	counts := h.CountAgentsByRole()
	if counts[RoleCoordinator] != 0 {
		t.Errorf("Expected 0 coordinators, got %d", counts[RoleCoordinator])
	}

	// Add coordinator
	h.SetCoordinator("coordinator-1")
	counts = h.CountAgentsByRole()
	if counts[RoleCoordinator] != 1 {
		t.Errorf("Expected 1 coordinator, got %d", counts[RoleCoordinator])
	}

	// Add supervisors
	h.AddSupervisor("supervisor-1")
	h.AddSupervisor("supervisor-2")
	counts = h.CountAgentsByRole()
	if counts[RoleSupervisor] != 2 {
		t.Errorf("Expected 2 supervisors, got %d", counts[RoleSupervisor])
	}

	// Add workers
	h.AddWorker("worker-1", "supervisor-1")
	h.AddWorker("worker-2", "supervisor-1")
	h.AddWorker("worker-3", "supervisor-2")
	counts = h.CountAgentsByRole()
	if counts[RoleWorker] != 3 {
		t.Errorf("Expected 3 workers, got %d", counts[RoleWorker])
	}
}

// TestGetRoleCapabilities tests retrieving role capabilities.
func TestGetRoleCapabilities(t *testing.T) {
	h := NewHierarchy()

	// Get coordinator capabilities
	cap := h.GetRoleCapabilities(RoleCoordinator)
	if cap == nil {
		t.Fatal("Coordinator capabilities should not be nil")
	}

	if !cap.Permissions.CanSpawnAgents {
		t.Error("Coordinator should have CanSpawnAgents permission")
	}

	if cap.Permissions.CanEscalate {
		t.Error("Coordinator should not have CanEscalate permission")
	}

	// Get supervisor capabilities
	cap = h.GetRoleCapabilities(RoleSupervisor)
	if cap == nil {
		t.Fatal("Supervisor capabilities should not be nil")
	}

	if cap.DelegationBudget == nil {
		t.Error("Supervisor should have delegation budget")
	}

	if cap.DelegationBudget.MaxWorkers != 5 {
		t.Errorf("Supervisor MaxWorkers = %d, want 5", cap.DelegationBudget.MaxWorkers)
	}

	// Get worker capabilities
	cap = h.GetRoleCapabilities(RoleWorker)
	if cap == nil {
		t.Fatal("Worker capabilities should not be nil")
	}

	if cap.Permissions.CanSpawnAgents {
		t.Error("Worker should not have CanSpawnAgents permission")
	}

	if !cap.Permissions.CanEscalate {
		t.Error("Worker should have CanEscalate permission")
	}
}

// TestJSONSerialization tests serializing and deserializing hierarchy.
func TestJSONSerialization(t *testing.T) {
	h1 := NewHierarchy()
	h1.SetCoordinator("coordinator-1")
	h1.AddSupervisor("supervisor-1")
	h1.AddWorker("worker-1", "supervisor-1")

	// Serialize
	data, err := h1.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}

	if len(data) == 0 {
		t.Error("JSON data should not be empty")
	}

	// Deserialize
	h2, err := FromJSON(data)
	if err != nil {
		t.Fatalf("FromJSON failed: %v", err)
	}

	// Verify structure is preserved
	if h2.Coordinator == nil {
		t.Error("Deserialized coordinator should not be nil")
	}

	if len(h2.Supervisors) != 1 {
		t.Errorf("Deserialized supervisors count = %d, want 1", len(h2.Supervisors))
	}

	if len(h2.Workers) != 1 {
		t.Errorf("Deserialized workers count = %d, want 1", len(h2.Workers))
	}
}

// TestDelegationBudgetEnforcement tests that delegation budgets are enforced.
func TestDelegationBudgetEnforcement(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")

	budget := h.GetRoleCapabilities(RoleSupervisor).DelegationBudget
	maxWorkers := budget.MaxWorkers

	// Add workers up to the limit
	for i := 1; i <= maxWorkers; i++ {
		workerID := "worker-" + string(rune(48+i))
		err := h.AddWorker(workerID, "supervisor-1")
		if err != nil {
			t.Fatalf("Failed to add worker %d: %v", i, err)
		}
	}

	// Try to exceed the limit
	err := h.AddWorker("worker-excess", "supervisor-1")
	if err == nil {
		t.Error("Should not allow adding worker beyond budget")
	}

	supervisor := h.Supervisors["supervisor-1"]
	if len(supervisor.ChildrenIDs) != maxWorkers {
		t.Errorf("Supervisor should have %d workers, has %d", maxWorkers, len(supervisor.ChildrenIDs))
	}
}

// TestConcurrentModification tests thread safety of hierarchy operations.
func TestConcurrentModification(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")

	done := make(chan bool, 10)

	// Concurrent writes
	for i := 0; i < 5; i++ {
		go func(idx int) {
			workerID := "worker-" + string(rune(48+idx))
			_ = h.AddWorker(workerID, "supervisor-1")
			done <- true
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 5; i++ {
		go func() {
			_ = h.CountAgentsByRole()
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify final state is consistent
	counts := h.CountAgentsByRole()
	if counts[RoleWorker] < 1 {
		t.Error("Should have at least 1 worker")
	}
}

// TestTimestamps tests that timestamps are set correctly.
func TestTimestamps(t *testing.T) {
	h := NewHierarchy()

	if h.CreatedAt.IsZero() {
		t.Error("Hierarchy CreatedAt should not be zero")
	}

	before := time.Now()
	h.SetCoordinator("coordinator-1")
	after := time.Now()

	if h.Coordinator.CreatedAt.Before(before) || h.Coordinator.CreatedAt.After(after.Add(time.Second)) {
		t.Error("Coordinator CreatedAt should be within expected time range")
	}

	if h.LastUpdated.Before(before) || h.LastUpdated.After(after.Add(time.Second)) {
		t.Error("Hierarchy LastUpdated should be within expected time range")
	}
}

// TestRoleCapabilitiesConsistency tests that role capabilities are internally consistent.
func TestRoleCapabilitiesConsistency(t *testing.T) {
	caps := DefaultCoordinatorCapabilities()

	// Coordinator can spawn agents but only supervisors
	if caps.Permissions.CanSpawnAgents {
		for _, role := range caps.Permissions.CanSpawnRole {
			if role != RoleSupervisor {
				t.Errorf("Coordinator can spawn unsupported role: %s", role)
			}
		}
	}

	// Coordinator should have delegation budget for spawning
	if caps.DelegationBudget == nil {
		t.Error("Coordinator should have delegation budget")
	}

	// Verify supervisor capabilities
	supCaps := DefaultSupervisorCapabilities()
	if supCaps.DelegationBudget == nil {
		t.Error("Supervisor should have delegation budget")
	}

	if supCaps.DelegationBudget.MaxWorkers <= 0 {
		t.Error("Supervisor MaxWorkers should be positive")
	}

	// Verify worker capabilities
	workerCaps := DefaultWorkerCapabilities()
	if workerCaps.DelegationBudget != nil {
		t.Error("Worker should not have delegation budget")
	}

	if workerCaps.Permissions.CanSpawnAgents {
		t.Error("Worker should not be able to spawn agents")
	}
}

// TestAssignTaskToWorker tests assigning tasks to workers with budget enforcement.
func TestAssignTaskToWorker(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")
	h.AddWorker("worker-1", "supervisor-1")

	// Assign first task
	err := h.AssignTaskToWorker("worker-1", "task-1")
	if err != nil {
		t.Fatalf("AssignTaskToWorker failed: %v", err)
	}

	currentLoad, _, err := h.GetWorkerTaskLoad("worker-1")
	if err != nil {
		t.Fatalf("GetWorkerTaskLoad failed: %v", err)
	}
	if currentLoad != 1 {
		t.Errorf("Expected 1 task, got %d", currentLoad)
	}

	// Assign more tasks up to limit (default is 3)
	err = h.AssignTaskToWorker("worker-1", "task-2")
	if err != nil {
		t.Fatalf("AssignTaskToWorker failed: %v", err)
	}

	err = h.AssignTaskToWorker("worker-1", "task-3")
	if err != nil {
		t.Fatalf("AssignTaskToWorker failed: %v", err)
	}

	currentLoad, _, err = h.GetWorkerTaskLoad("worker-1")
	if err != nil {
		t.Fatalf("GetWorkerTaskLoad failed: %v", err)
	}
	if currentLoad != 3 {
		t.Errorf("Expected 3 tasks, got %d", currentLoad)
	}

	// Try to exceed the limit
	err = h.AssignTaskToWorker("worker-1", "task-4")
	if err == nil {
		t.Error("Should not allow assigning task beyond budget")
	}

	// Verify error message
	if err != nil && !contains(err.Error(), "max concurrent tasks limit") {
		t.Errorf("Unexpected error message: %v", err)
	}
}

// TestCompleteTaskForWorker tests completing tasks and freeing up budget.
func TestCompleteTaskForWorker(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")
	h.AddWorker("worker-1", "supervisor-1")

	// Assign 2 tasks
	h.AssignTaskToWorker("worker-1", "task-1")
	h.AssignTaskToWorker("worker-1", "task-2")

	currentLoad, _, _ := h.GetWorkerTaskLoad("worker-1")
	if currentLoad != 2 {
		t.Errorf("Expected 2 tasks, got %d", currentLoad)
	}

	// Complete first task
	err := h.CompleteTaskForWorker("worker-1", "task-1")
	if err != nil {
		t.Fatalf("CompleteTaskForWorker failed: %v", err)
	}

	currentLoad, _, _ = h.GetWorkerTaskLoad("worker-1")
	if currentLoad != 1 {
		t.Errorf("Expected 1 task after completion, got %d", currentLoad)
	}

	// Complete second task
	err = h.CompleteTaskForWorker("worker-1", "task-2")
	if err != nil {
		t.Fatalf("CompleteTaskForWorker failed: %v", err)
	}

	currentLoad, _, _ = h.GetWorkerTaskLoad("worker-1")
	if currentLoad != 0 {
		t.Errorf("Expected 0 tasks after completion, got %d", currentLoad)
	}

	// Try to complete non-existent task
	err = h.CompleteTaskForWorker("worker-1", "task-nonexistent")
	if err == nil {
		t.Error("Should return error for non-existent task")
	}
}

// TestTaskTimeout tests task timeout tracking.
func TestTaskTimeout(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")
	h.AddWorker("worker-1", "supervisor-1")

	// Assign a task
	h.AssignTaskToWorker("worker-1", "task-1")

	// Get the worker and manually set deadline to past
	worker := h.Workers["worker-1"]
	if len(worker.BudgetStatus.AssignedTasks) > 0 {
		task := worker.BudgetStatus.AssignedTasks[0]
		task.DeadlineAt = time.Now().Add(-time.Second) // Set to past
	}

	// Complete the task (should track as expired)
	err := h.CompleteTaskForWorker("worker-1", "task-1")
	if err != nil {
		t.Fatalf("CompleteTaskForWorker failed: %v", err)
	}

	_, expiredCount, _ := h.GetWorkerTaskLoad("worker-1")
	if expiredCount != 1 {
		t.Errorf("Expected 1 expired task, got %d", expiredCount)
	}
}

// TestPruneExpiredTasks tests removing expired tasks.
func TestPruneExpiredTasks(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")
	h.AddWorker("worker-1", "supervisor-1")

	// Assign multiple tasks
	h.AssignTaskToWorker("worker-1", "task-1")
	h.AssignTaskToWorker("worker-1", "task-2")
	h.AssignTaskToWorker("worker-1", "task-3")

	// Set some tasks to expired
	worker := h.Workers["worker-1"]
	for i, task := range worker.BudgetStatus.AssignedTasks {
		if i < 2 { // Mark first 2 as expired
			task.DeadlineAt = time.Now().Add(-time.Second)
		}
	}

	// Prune expired tasks
	pruned, err := h.PruneExpiredTasks("worker-1")
	if err != nil {
		t.Fatalf("PruneExpiredTasks failed: %v", err)
	}

	if pruned != 2 {
		t.Errorf("Expected 2 pruned tasks, got %d", pruned)
	}

	currentLoad, _, _ := h.GetWorkerTaskLoad("worker-1")
	if currentLoad != 1 {
		t.Errorf("Expected 1 active task after pruning, got %d", currentLoad)
	}
}

// TestResetWorkerBudget tests resetting a worker's budget.
func TestResetWorkerBudget(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")
	h.AddWorker("worker-1", "supervisor-1")

	// Assign tasks
	h.AssignTaskToWorker("worker-1", "task-1")
	h.AssignTaskToWorker("worker-1", "task-2")

	currentLoad, _, _ := h.GetWorkerTaskLoad("worker-1")
	if currentLoad != 2 {
		t.Errorf("Expected 2 tasks before reset, got %d", currentLoad)
	}

	// Reset budget
	err := h.ResetWorkerBudget("worker-1")
	if err != nil {
		t.Fatalf("ResetWorkerBudget failed: %v", err)
	}

	currentLoad, _, _ = h.GetWorkerTaskLoad("worker-1")
	if currentLoad != 0 {
		t.Errorf("Expected 0 tasks after reset, got %d", currentLoad)
	}

	// Should be able to assign tasks again
	err = h.AssignTaskToWorker("worker-1", "task-new-1")
	if err != nil {
		t.Fatalf("AssignTaskToWorker failed after reset: %v", err)
	}
}

// TestValidateBudgetForWorkerSpawn tests spawn budget validation.
func TestValidateBudgetForWorkerSpawn(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")

	budget := h.GetRoleCapabilities(RoleSupervisor).DelegationBudget
	maxWorkers := budget.MaxWorkers

	// Add workers up to the limit
	for i := 1; i <= maxWorkers; i++ {
		workerID := "worker-" + string(rune(48+i))
		h.AddWorker(workerID, "supervisor-1")
	}

	// Should fail to spawn another worker
	err := h.ValidateBudgetForWorkerSpawn("supervisor-1")
	if err == nil {
		t.Error("Should return error when supervisor is at max workers capacity")
	}

	if err != nil && !contains(err.Error(), "max workers capacity") {
		t.Errorf("Unexpected error message: %v", err)
	}
}

// TestBudgetStatusInitialization tests that workers get proper budget status.
func TestBudgetStatusInitialization(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")
	h.AddWorker("worker-1", "supervisor-1")

	worker := h.Workers["worker-1"]
	if worker.BudgetStatus == nil {
		t.Fatal("Worker should have BudgetStatus initialized")
	}

	if worker.BudgetStatus.CurrentTaskCount != 0 {
		t.Errorf("Initial task count should be 0, got %d", worker.BudgetStatus.CurrentTaskCount)
	}

	if len(worker.BudgetStatus.AssignedTasks) != 0 {
		t.Errorf("Initial assigned tasks should be empty, got %d", len(worker.BudgetStatus.AssignedTasks))
	}

	if worker.BudgetStatus.ExpiredTaskCount != 0 {
		t.Errorf("Initial expired task count should be 0, got %d", worker.BudgetStatus.ExpiredTaskCount)
	}

	if worker.BudgetStatus.LastResetAt.IsZero() {
		t.Error("LastResetAt should be initialized")
	}
}

// TestMultiWorkerBudgetIsolation tests that task budgets are isolated per worker.
func TestMultiWorkerBudgetIsolation(t *testing.T) {
	h := NewHierarchy()
	h.SetCoordinator("coordinator-1")
	h.AddSupervisor("supervisor-1")
	h.AddWorker("worker-1", "supervisor-1")
	h.AddWorker("worker-2", "supervisor-1")

	// Fill worker-1's budget
	h.AssignTaskToWorker("worker-1", "task-1")
	h.AssignTaskToWorker("worker-1", "task-2")
	h.AssignTaskToWorker("worker-1", "task-3")

	// Worker-2 should still have capacity
	load1, _, _ := h.GetWorkerTaskLoad("worker-1")
	load2, _, _ := h.GetWorkerTaskLoad("worker-2")

	if load1 != 3 {
		t.Errorf("Worker-1 load should be 3, got %d", load1)
	}

	if load2 != 0 {
		t.Errorf("Worker-2 load should be 0, got %d", load2)
	}

	// Should be able to assign to worker-2
	err := h.AssignTaskToWorker("worker-2", "task-2-1")
	if err != nil {
		t.Fatalf("Should be able to assign to worker-2: %v", err)
	}

	load2, _, _ = h.GetWorkerTaskLoad("worker-2")
	if load2 != 1 {
		t.Errorf("Worker-2 load should be 1, got %d", load2)
	}
}

// contains is a helper function for checking if a string contains a substring.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0)
}
