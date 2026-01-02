// Package hierarchy provides 3-tier agent hierarchy schema and role definitions.
// This file implements the SupervisorAgent type which manages workers within the hierarchy.
package hierarchy

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// WorkerStatus tracks the operational state of a worker agent.
type WorkerStatus string

const (
	// WorkerStatusPending indicates the worker has been spawned but not yet started
	WorkerStatusPending WorkerStatus = "pending"

	// WorkerStatusRunning indicates the worker is currently executing
	WorkerStatusRunning WorkerStatus = "running"

	// WorkerStatusCompleted indicates the worker finished successfully
	WorkerStatusCompleted WorkerStatus = "completed"

	// WorkerStatusFailed indicates the worker encountered an error
	WorkerStatusFailed WorkerStatus = "failed"

	// WorkerStatusTerminated indicates the worker was terminated
	WorkerStatusTerminated WorkerStatus = "terminated"
)

// WorkerResult contains the result of a worker's task execution.
type WorkerResult struct {
	// WorkerID is the unique identifier of the worker
	WorkerID string `json:"worker_id"`

	// Status is the final status of the worker
	Status WorkerStatus `json:"status"`

	// Result is the output data from the worker's task
	Result interface{} `json:"result,omitempty"`

	// Error contains any error message if the task failed
	Error string `json:"error,omitempty"`

	// CompletedAt is when the worker finished
	CompletedAt time.Time `json:"completed_at,omitempty"`

	// DurationMs is the total time the worker spent executing (in milliseconds)
	DurationMs int64 `json:"duration_ms,omitempty"`
}

// SupervisorAgent represents a supervisor agent within the hierarchy.
// A supervisor can spawn workers (up to its delegation budget), assign tasks,
// aggregate results, and escalate issues to the coordinator.
type SupervisorAgent struct {
	// mu protects concurrent access to supervisor state
	mu sync.RWMutex

	// node is the hierarchy node for this supervisor
	node *HierarchyNode

	// spawnedWorkers tracks all workers spawned by this supervisor (ID -> node)
	spawnedWorkers map[string]*HierarchyNode

	// workerResults tracks execution results from workers (ID -> result)
	workerResults map[string]*WorkerResult

	// activeTaskCount tracks the total number of active tasks across all workers
	activeTaskCount int

	// createdAt is when this supervisor was instantiated
	createdAt time.Time
}

// NewSupervisorAgent creates a new supervisor agent from a hierarchy node.
//
// The supervisor will use the delegation budget from the hierarchy's role capabilities.
// It initializes empty worker and result maps.
//
// Parameters:
//   - node: The HierarchyNode representing this supervisor in the hierarchy
//
// Returns:
//   - A new SupervisorAgent instance
//   - An error if the node is not a supervisor role
func NewSupervisorAgent(node *HierarchyNode) (*SupervisorAgent, error) {
	if node == nil {
		return nil, fmt.Errorf("hierarchy node cannot be nil")
	}

	if node.Role != RoleSupervisor {
		return nil, fmt.Errorf("supervisor agent requires RoleSupervisor role, got %s", node.Role)
	}

	return &SupervisorAgent{
		node:           node,
		spawnedWorkers: make(map[string]*HierarchyNode),
		workerResults:  make(map[string]*WorkerResult),
		createdAt:      time.Now(),
	}, nil
}

// SpawnWorker spawns a new worker under this supervisor.
//
// The worker is added to the hierarchy under this supervisor, respecting
// the delegation budget limits:
// - MaxWorkers: Maximum number of workers this supervisor can have
// - MaxConcurrentSpawns: Maximum workers that can be spawned simultaneously
// - MaxTasksPerWorker: Maximum tasks per worker
//
// Parameters:
//   - ctx: Context for cancellation
//   - workerID: Unique identifier for the worker
//
// Returns:
//   - The spawned worker's HierarchyNode
//   - An error if spawning fails (e.g., budget exceeded, invalid ID)
func (s *SupervisorAgent) SpawnWorker(ctx context.Context, workerID string) (*HierarchyNode, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if workerID == "" {
		return nil, fmt.Errorf("worker ID cannot be empty")
	}

	if _, exists := s.spawnedWorkers[workerID]; exists {
		return nil, fmt.Errorf("worker %q already spawned by this supervisor", workerID)
	}

	// Check if we're within spawn limits
	if err := s.checkSpawnBudget(); err != nil {
		return nil, err
	}

	// Create the worker node
	now := time.Now()
	worker := &HierarchyNode{
		AgentID:      workerID,
		Role:         RoleWorker,
		ParentID:     &s.node.AgentID,
		ChildrenIDs:  make([]string, 0), // Workers don't have children
		CreatedAt:    now,
		LastUpdated:  now,
		Status:       "active",
		Capabilities: s.getWorkerCapabilities(),
		Metadata: map[string]interface{}{
			"supervisor_id": s.node.AgentID,
			"spawned_at":    now,
		},
	}

	// Add to our internal tracking
	s.spawnedWorkers[workerID] = worker

	// Add to supervisor's children list
	s.node.ChildrenIDs = append(s.node.ChildrenIDs, workerID)

	return worker, nil
}

// AssignTask marks a task as assigned to a worker (for tracking purposes).
//
// This increments the active task count for the supervisor, which is used
// to enforce MaxTasksPerWorker limits.
//
// Parameters:
//   - workerID: The ID of the worker to assign the task to
//
// Returns:
//   - An error if the worker is not found or task assignment fails
func (s *SupervisorAgent) AssignTask(workerID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.spawnedWorkers[workerID]; !exists {
		return fmt.Errorf("worker %q not found", workerID)
	}

	// Check task limits
	budget := s.getBudget()
	if budget != nil && s.activeTaskCount >= budget.MaxTasksPerWorker {
		return fmt.Errorf("worker %q has reached max tasks limit (%d)", workerID, budget.MaxTasksPerWorker)
	}

	s.activeTaskCount++
	return nil
}

// CompleteTask marks a task as complete for a worker.
//
// This decrements the active task count.
//
// Parameters:
//   - workerID: The ID of the worker that completed the task
func (s *SupervisorAgent) CompleteTask(workerID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.activeTaskCount > 0 {
		s.activeTaskCount--
	}
}

// RecordWorkerResult records the result of a worker's execution.
//
// Parameters:
//   - result: The WorkerResult to record
func (s *SupervisorAgent) RecordWorkerResult(result *WorkerResult) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if result == nil {
		return fmt.Errorf("result cannot be nil")
	}

	if result.WorkerID == "" {
		return fmt.Errorf("result worker ID cannot be empty")
	}

	if _, exists := s.spawnedWorkers[result.WorkerID]; !exists {
		return fmt.Errorf("result references unknown worker %q", result.WorkerID)
	}

	s.workerResults[result.WorkerID] = result

	// Update worker status in hierarchy node
	if worker, exists := s.spawnedWorkers[result.WorkerID]; exists {
		worker.Status = string(result.Status)
		worker.LastUpdated = time.Now()
	}

	return nil
}

// AggregateResults collects and returns all worker results.
//
// Returns:
//   - A map of worker ID to WorkerResult
//   - The number of successful completions
//   - The number of failed workers
func (s *SupervisorAgent) AggregateResults() (map[string]*WorkerResult, int, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	successCount := 0
	failureCount := 0

	for _, result := range s.workerResults {
		if result.Status == WorkerStatusCompleted {
			successCount++
		} else if result.Status == WorkerStatusFailed {
			failureCount++
		}
	}

	// Create a copy to return
	resultsCopy := make(map[string]*WorkerResult)
	for id, result := range s.workerResults {
		resultsCopy[id] = result
	}

	return resultsCopy, successCount, failureCount
}

// TerminateWorker removes a worker from supervision and marks it as terminated.
//
// Parameters:
//   - ctx: Context for cancellation
//   - workerID: The ID of the worker to terminate
//
// Returns:
//   - An error if the worker is not found
func (s *SupervisorAgent) TerminateWorker(ctx context.Context, workerID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	worker, exists := s.spawnedWorkers[workerID]
	if !exists {
		return fmt.Errorf("worker %q not found", workerID)
	}

	// Mark as terminated
	worker.Status = "terminated"
	worker.LastUpdated = time.Now()

	// Record termination if not already done
	if _, resultExists := s.workerResults[workerID]; !resultExists {
		s.workerResults[workerID] = &WorkerResult{
			WorkerID:    workerID,
			Status:      WorkerStatusTerminated,
			CompletedAt: time.Now(),
		}
	}

	// Remove from children list
	newChildren := make([]string, 0)
	for _, childID := range s.node.ChildrenIDs {
		if childID != workerID {
			newChildren = append(newChildren, childID)
		}
	}
	s.node.ChildrenIDs = newChildren

	// Keep in spawnedWorkers for historical tracking

	return nil
}

// GetWorkerStatus returns the current status of a worker.
//
// Parameters:
//   - ctx: Context for cancellation
//   - workerID: The ID of the worker to check
//
// Returns:
//   - The current status
//   - An error if the worker is not found
func (s *SupervisorAgent) GetWorkerStatus(ctx context.Context, workerID string) (WorkerStatus, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	worker, exists := s.spawnedWorkers[workerID]
	if !exists {
		return "", fmt.Errorf("worker %q not found", workerID)
	}

	return WorkerStatus(worker.Status), nil
}

// GetSpawnedWorkers returns all workers spawned by this supervisor.
//
// Returns:
//   - A slice of HierarchyNode pointers representing the workers
func (s *SupervisorAgent) GetSpawnedWorkers() []*HierarchyNode {
	s.mu.RLock()
	defer s.mu.RUnlock()

	workers := make([]*HierarchyNode, 0, len(s.spawnedWorkers))
	for _, worker := range s.spawnedWorkers {
		workers = append(workers, worker)
	}

	return workers
}

// GetWorkerCount returns the total number of workers spawned.
//
// Returns:
//   - The number of workers currently managed by this supervisor
func (s *SupervisorAgent) GetWorkerCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.spawnedWorkers)
}

// GetActiveTaskCount returns the number of currently active tasks.
//
// Returns:
//   - The count of active tasks across all workers
func (s *SupervisorAgent) GetActiveTaskCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.activeTaskCount
}

// IsWithinBudget checks if the supervisor is within its delegation budget.
//
// Returns:
//   - True if within budget, false if at or exceeding limits
func (s *SupervisorAgent) IsWithinBudget() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.checkSpawnBudget() == nil
}

// GetNodeRef returns the underlying HierarchyNode for this supervisor.
//
// Returns:
//   - The HierarchyNode representing this supervisor
func (s *SupervisorAgent) GetNodeRef() *HierarchyNode {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.node
}

// GetID returns the unique identifier of this supervisor.
//
// Returns:
//   - The agent ID
func (s *SupervisorAgent) GetID() string {
	return s.node.AgentID
}

// checkSpawnBudget verifies if we can spawn another worker.
// Must be called with mu held.
func (s *SupervisorAgent) checkSpawnBudget() error {
	budget := s.getBudget()
	if budget == nil {
		return nil // No budget configured (shouldn't happen for supervisors)
	}

	// Check max workers limit
	if len(s.spawnedWorkers) >= budget.MaxWorkers {
		return fmt.Errorf("supervisor has reached max workers limit (%d)", budget.MaxWorkers)
	}

	// Check concurrent spawns (simplified: count running workers)
	// In a real implementation, you'd track spawn timestamps
	runningCount := 0
	for _, worker := range s.spawnedWorkers {
		if worker.Status == "active" || worker.Status == "running" {
			runningCount++
		}
	}

	if runningCount >= budget.MaxConcurrentSpawns {
		return fmt.Errorf("supervisor cannot spawn more workers simultaneously (limit: %d)", budget.MaxConcurrentSpawns)
	}

	return nil
}

// getBudget returns the delegation budget for supervisors.
// Must be called with mu held.
func (s *SupervisorAgent) getBudget() *DelegationBudget {
	// Return the budget from the node capabilities if available
	if s.node != nil && s.node.Capabilities != nil && s.node.Capabilities.DelegationBudget != nil {
		return s.node.Capabilities.DelegationBudget
	}

	// Fallback to default supervisor budget if not configured
	return &DelegationBudget{
		MaxWorkers:          5,
		MaxTasksPerWorker:   3,
		MaxConcurrentSpawns: 2,
		TimeoutSeconds:      600,
	}
}

// getWorkerCapabilities returns the role capabilities for workers.
// Must be called with mu held.
func (s *SupervisorAgent) getWorkerCapabilities() *RoleCapabilities {
	return DefaultWorkerCapabilities()
}

// EscalateIssue creates an escalation message to the coordinator.
//
// This would normally send a signal/message to the coordinator,
// but for now we return the escalation details.
//
// Parameters:
//   - severity: The severity level of the issue (e.g., "critical", "warning")
//   - message: Description of the issue
//
// Returns:
//   - An escalation message that should be sent to the coordinator
func (s *SupervisorAgent) EscalateIssue(severity string, message string) string {
	return fmt.Sprintf("ESCALATION [%s]: Supervisor %s reports: %s", severity, s.node.AgentID, message)
}
