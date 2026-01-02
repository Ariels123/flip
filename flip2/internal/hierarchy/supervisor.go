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

	// pool manages the lifecycle, health, and limits of workers
	pool *WorkerPool

	// workerResults tracks execution results from workers (ID -> result)
	workerResults map[string]*WorkerResult

	// workerTaskCounts tracks the number of active tasks per worker (ID -> count)
	workerTaskCounts map[string]int

	// terminatedWorkers tracks terminated workers for history (ID -> node)
	terminatedWorkers map[string]*HierarchyNode

	// activeTaskCount tracks the total number of active tasks across all workers
	activeTaskCount int

	// createdAt is when this supervisor was instantiated
	createdAt time.Time
}

// NewSupervisorAgent creates a new supervisor agent from a hierarchy node.
//
// The supervisor will use the delegation budget from the hierarchy's role capabilities.
// It initializes a WorkerPool to manage workers.
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

	// Configure pool
	poolConfig := DefaultPoolConfig()
	if node.Capabilities != nil && node.Capabilities.DelegationBudget != nil {
		poolConfig.MaxSize = node.Capabilities.DelegationBudget.MaxWorkers
	}

	pool := NewWorkerPool("pool-"+node.AgentID, node.AgentID, poolConfig)

	// Set default health checker
	pool.SetHealthChecker(DefaultHealthChecker())

	return &SupervisorAgent{
		node:              node,
		pool:              pool,
		workerResults:     make(map[string]*WorkerResult),
		workerTaskCounts:  make(map[string]int),
		terminatedWorkers: make(map[string]*HierarchyNode),
		createdAt:         time.Now(),
	}, nil
}

// Start starts the supervisor's background processes, such as health monitoring.
func (s *SupervisorAgent) Start(ctx context.Context) error {
	return s.pool.StartHealthMonitoring(ctx)
}

// Stop stops the supervisor's background processes.
func (s *SupervisorAgent) Stop() {
	s.pool.StopHealthMonitoring()
}

// SpawnWorker spawns a new worker under this supervisor.
//
// The worker is added to the hierarchy under this supervisor, respecting
// the delegation budget limits:
// - MaxWorkers: Maximum number of workers this supervisor can have (handled by WorkerPool)
// - MaxConcurrentSpawns: Maximum workers that can be spawned simultaneously
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

	// Sync pool config with current budget (handling dynamic updates)
	s.syncPoolConfigLocked()

	if workerID == "" {
		return nil, fmt.Errorf("worker ID cannot be empty")
	}

	if _, exists := s.pool.GetWorker(workerID); exists {
		return nil, fmt.Errorf("worker %q already spawned by this supervisor", workerID)
	}

	if _, exists := s.terminatedWorkers[workerID]; exists {
		return nil, fmt.Errorf("worker %q already spawned by this supervisor", workerID)
	}

	// Check MaxWorkers limit with specific error message expected by tests
	budget := s.getBudget()
	if budget != nil && s.pool.Size() >= budget.MaxWorkers {
		return nil, fmt.Errorf("supervisor has reached max workers limit (%d)", budget.MaxWorkers)
	}

	// Check concurrent spawn limits
	if err := s.checkConcurrentSpawnsLimit(); err != nil {
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

	// Add to pool (this checks MaxSize too, but we checked above for custom error)
	if err := s.pool.AddWorker(worker); err != nil {
		return nil, err
	}

	s.workerTaskCounts[workerID] = 0

	// Add to supervisor's children list
	s.node.ChildrenIDs = append(s.node.ChildrenIDs, workerID)

	return worker, nil
}

// AssignTask marks a task as assigned to a worker (for tracking purposes).
//
// This increments the active task count for the supervisor and the specific worker,
// which is used to enforce MaxTasksPerWorker limits.
//
// Parameters:
//   - workerID: The ID of the worker to assign the task to
//
// Returns:
//   - An error if the worker is not found or task assignment fails
func (s *SupervisorAgent) AssignTask(workerID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.pool.GetWorker(workerID); !exists {
		return fmt.Errorf("worker %q not found", workerID)
	}

	// Check task limits
	budget := s.getBudget()
	currentLoad := s.workerTaskCounts[workerID]
	
	if budget != nil && currentLoad >= budget.MaxTasksPerWorker {
		return fmt.Errorf("worker %q has reached max tasks limit (%d)", workerID, budget.MaxTasksPerWorker)
	}

	s.workerTaskCounts[workerID]++
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

	if count, exists := s.workerTaskCounts[workerID]; exists && count > 0 {
		s.workerTaskCounts[workerID]--
	}

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

	if _, exists := s.pool.GetWorker(result.WorkerID); !exists {
		// Check if it's a known terminated worker or has historical result
		_, knownTerminated := s.terminatedWorkers[result.WorkerID]
		_, knownResult := s.workerResults[result.WorkerID]

		if !knownTerminated && !knownResult {
			return fmt.Errorf("result references unknown worker %q", result.WorkerID)
		}
	}

	s.workerResults[result.WorkerID] = result

	// Update worker status in hierarchy node if still in pool
	if worker, exists := s.pool.GetWorker(result.WorkerID); exists {
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

	// Check existence
	worker, exists := s.pool.GetWorker(workerID)
	if !exists {
		return fmt.Errorf("worker %q not found", workerID)
	}

	// Record termination result
	s.workerResults[workerID] = &WorkerResult{
		WorkerID:    workerID,
		Status:      WorkerStatusTerminated,
		CompletedAt: time.Now(),
	}

	// Update worker status and move to terminated tracking
	worker.Status = "terminated"
	worker.LastUpdated = time.Now()
	s.terminatedWorkers[workerID] = worker

	// Remove from pool
	if err := s.pool.RemoveWorker(workerID); err != nil {
		return err
	}

	// Remove from children list
	newChildren := make([]string, 0)
	for _, childID := range s.node.ChildrenIDs {
		if childID != workerID {
			newChildren = append(newChildren, childID)
		}
	}
	s.node.ChildrenIDs = newChildren

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

	worker, exists := s.pool.GetWorker(workerID)
	if !exists {
		// Check if terminated
		if _, ok := s.terminatedWorkers[workerID]; ok {
			return WorkerStatusTerminated, nil
		}
		// Check if likely terminated (in results)
		if res, ok := s.workerResults[workerID]; ok {
			return res.Status, nil
		}
		return "", fmt.Errorf("worker %q not found", workerID)
	}

	return WorkerStatus(worker.Status), nil
}

// GetSpawnedWorkers returns all workers spawned by this supervisor (active and terminated).
//
// Returns:
//   - A slice of HierarchyNode pointers representing the workers
func (s *SupervisorAgent) GetSpawnedWorkers() []*HierarchyNode {
	s.mu.RLock()
	defer s.mu.RUnlock()

	active := s.pool.ListWorkers()
	workers := make([]*HierarchyNode, 0, len(active)+len(s.terminatedWorkers))

	workers = append(workers, active...)
	for _, w := range s.terminatedWorkers {
		workers = append(workers, w)
	}

	return workers
}

// GetWorkerCount returns the total number of workers spawned (active + terminated).
//
// Returns:
//   - The number of workers currently managed by this supervisor
func (s *SupervisorAgent) GetWorkerCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.pool.Size() + len(s.terminatedWorkers)
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

	s.syncPoolConfigLocked()

	// Check pool size limit
	if s.pool.IsFull() {
		return false
	}

	// Check concurrent spawns
	if err := s.checkConcurrentSpawnsLimit(); err != nil {
		return false
	}

	return true
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

// syncPoolConfigLocked updates the pool configuration if the supervisor's budget has changed.
// Must be called with mu held.
func (s *SupervisorAgent) syncPoolConfigLocked() {
	budget := s.getBudget()
	if budget == nil {
		return
	}

	currentConfig := s.pool.Config()
	if currentConfig.MaxSize != budget.MaxWorkers {
		newConfig := currentConfig
		newConfig.MaxSize = budget.MaxWorkers
		s.pool.UpdateConfig(newConfig)
	}
}

// checkConcurrentSpawnsLimit verifies if we can spawn another worker based on concurrency.
// Must be called with mu held.
func (s *SupervisorAgent) checkConcurrentSpawnsLimit() error {
	budget := s.getBudget()
	if budget == nil {
		return nil
	}

	// Count running workers
	runningCount := 0
	for _, worker := range s.pool.ListWorkers() {
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

// ================================================================================
// TASK DELEGATION
// ================================================================================

// DelegationStrategy defines how tasks are distributed among workers.
type DelegationStrategy string

const (
	// StrategyRoundRobin distributes tasks evenly across workers in order.
	StrategyRoundRobin DelegationStrategy = "round_robin"

	// StrategyLeastLoaded assigns tasks to the worker with the fewest active tasks.
	StrategyLeastLoaded DelegationStrategy = "least_loaded"

	// StrategyCapabilityMatch assigns to workers whose capabilities best match the task.
	StrategyCapabilityMatch DelegationStrategy = "capability_match"

	// StrategyPriority assigns to the first available worker based on priority ordering.
	StrategyPriority DelegationStrategy = "priority"
)

// TaskRequirements specifies what a task needs from a worker.
type TaskRequirements struct {
	// TaskID is the unique identifier for this task.
	TaskID string `json:"task_id"`

	// TaskType classifies the kind of work (e.g., "code_generation", "testing").
	TaskType string `json:"task_type"`

	// Priority indicates task urgency (1-5, where 5 is highest).
	Priority int `json:"priority"`

	// RequiredCapabilities lists capabilities the worker must have.
	RequiredCapabilities []string `json:"required_capabilities,omitempty"`

	// PreferredWorkerID optionally specifies a preferred worker.
	PreferredWorkerID string `json:"preferred_worker_id,omitempty"`

	// TimeoutSeconds is the maximum time allowed for task execution.
	TimeoutSeconds int `json:"timeout_seconds,omitempty"`

	// Metadata contains additional task-specific data.
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// DelegationResult contains the outcome of a task delegation attempt.
type DelegationResult struct {
	// Success indicates if delegation was successful.
	Success bool `json:"success"`

	// WorkerID is the ID of the worker the task was delegated to.
	WorkerID string `json:"worker_id,omitempty"`

	// Reason explains why delegation succeeded or failed.
	Reason string `json:"reason"`

	// DelegatedAt is when the delegation occurred.
	DelegatedAt time.Time `json:"delegated_at,omitempty"`

	// EstimatedDeadline is the expected completion time based on timeout.
	EstimatedDeadline time.Time `json:"estimated_deadline,omitempty"`

	// MatchScore indicates how well the worker matched (0.0-1.0).
	MatchScore float64 `json:"match_score,omitempty"`
}

// WorkerCapabilityInfo tracks a worker's capabilities and current load.
type WorkerCapabilityInfo struct {
	// WorkerID is the unique identifier.
	WorkerID string

	// Capabilities lists what this worker can do.
	Capabilities []string

	// ActiveTasks is the current number of assigned tasks.
	ActiveTasks int

	// MaxTasks is the maximum tasks this worker can handle.
	MaxTasks int

	// IsAvailable indicates if the worker can accept new tasks.
	IsAvailable bool

	// Priority is the worker's priority order (higher = preferred).
	Priority int
}

// DelegateTask delegates a task to an appropriate worker based on the specified strategy.
//
// The delegation process:
// 1. Validates the task requirements
// 2. Filters workers based on availability and capabilities
// 3. Selects the best worker using the specified strategy
// 4. Assigns the task to the selected worker
//
// Parameters:
//   - ctx: Context for cancellation
//   - task: The task requirements to delegate
//   - strategy: The delegation strategy to use
//
// Returns:
//   - DelegationResult containing the outcome of the delegation
//   - An error if delegation fails due to validation or system errors
func (s *SupervisorAgent) DelegateTask(ctx context.Context, task *TaskRequirements, strategy DelegationStrategy) (*DelegationResult, error) {
	if task == nil {
		return nil, fmt.Errorf("task requirements cannot be nil")
	}

	if task.TaskID == "" {
		return nil, fmt.Errorf("task ID cannot be empty")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.syncPoolConfigLocked()

	// Get available workers with their info
	candidates := s.getAvailableWorkersLocked()

	if len(candidates) == 0 {
		return &DelegationResult{
			Success: false,
			Reason:  "no available workers to delegate to",
		}, nil
	}

	// Filter by capabilities if required
	if len(task.RequiredCapabilities) > 0 {
		candidates = s.filterByCapabilities(candidates, task.RequiredCapabilities)
		if len(candidates) == 0 {
			return &DelegationResult{
				Success: false,
				Reason:  fmt.Sprintf("no workers with required capabilities: %v", task.RequiredCapabilities),
			}, nil
		}
	}

	// Check for preferred worker
	if task.PreferredWorkerID != "" {
		for _, c := range candidates {
			if c.WorkerID == task.PreferredWorkerID {
				return s.delegateToWorkerLocked(task, c)
			}
		}
		// Preferred worker not available, continue with strategy
	}

	// Select worker based on strategy
	var selected *WorkerCapabilityInfo
	var matchScore float64

	switch strategy {
	case StrategyRoundRobin:
		selected, matchScore = s.selectRoundRobin(candidates)
	case StrategyLeastLoaded:
		selected, matchScore = s.selectLeastLoaded(candidates)
	case StrategyCapabilityMatch:
		selected, matchScore = s.selectByCapabilityMatch(candidates, task.RequiredCapabilities)
	case StrategyPriority:
		selected, matchScore = s.selectByPriority(candidates)
	default:
		// Default to least loaded
		selected, matchScore = s.selectLeastLoaded(candidates)
	}

	if selected == nil {
		return &DelegationResult{
			Success: false,
			Reason:  "failed to select a worker using strategy " + string(strategy),
		}, nil
	}

	result, err := s.delegateToWorkerLocked(task, selected)
	if err != nil {
		return nil, err
	}

	result.MatchScore = matchScore
	return result, nil
}

// getAvailableWorkersLocked returns info about all workers that can accept tasks.
// Must be called with mu held.
func (s *SupervisorAgent) getAvailableWorkersLocked() []*WorkerCapabilityInfo {
	budget := s.getBudget()
	maxTasksPerWorker := 3 // default
	if budget != nil {
		maxTasksPerWorker = budget.MaxTasksPerWorker
	}

	candidates := make([]*WorkerCapabilityInfo, 0)

	for _, worker := range s.pool.ListWorkers() {
		workerID := worker.AgentID
		// Skip terminated or failed workers
		if worker.Status == "terminated" || worker.Status == "failed" {
			continue
		}

		// Count active tasks for this worker
		activeTasks := s.workerTaskCounts[workerID]

		// Check if worker can accept more tasks
		isAvailable := activeTasks < maxTasksPerWorker

		// Extract capabilities from worker metadata
		capabilities := extractWorkerCapabilities(worker)

		// Get priority from metadata (default to 0)
		priority := 0
		if p, ok := worker.Metadata["priority"].(int); ok {
			priority = p
		}

		candidates = append(candidates, &WorkerCapabilityInfo{
			WorkerID:     workerID,
			Capabilities: capabilities,
			ActiveTasks:  activeTasks,
			MaxTasks:     maxTasksPerWorker,
			IsAvailable:  isAvailable,
			Priority:     priority,
		})
	}

	return candidates
}

// extractWorkerCapabilities extracts capability strings from a worker node.
func extractWorkerCapabilities(worker *HierarchyNode) []string {
	capabilities := make([]string, 0)

	if worker.Metadata == nil {
		return capabilities
	}

	// Check for explicit capabilities list
	if caps, ok := worker.Metadata["capabilities"].([]interface{}); ok {
		for _, cap := range caps {
			if capStr, ok := cap.(string); ok {
				capabilities = append(capabilities, capStr)
			}
		}
	}

	// Check for capability strings
	if caps, ok := worker.Metadata["capabilities"].([]string); ok {
		capabilities = append(capabilities, caps...)
	}

	// Add implied capabilities based on worker type
	if workerType, ok := worker.Metadata["worker_type"].(string); ok {
		switch workerType {
		case "code":
			capabilities = append(capabilities, "code_generation", "code_review", "debugging")
		case "test":
			capabilities = append(capabilities, "testing", "test_generation", "qa")
		case "research":
			capabilities = append(capabilities, "research", "documentation", "analysis")
		case "data":
			capabilities = append(capabilities, "data_processing", "parsing", "transformation")
		}
	}

	return capabilities
}

// filterByCapabilities returns workers that have all required capabilities.
func (s *SupervisorAgent) filterByCapabilities(candidates []*WorkerCapabilityInfo, required []string) []*WorkerCapabilityInfo {
	filtered := make([]*WorkerCapabilityInfo, 0)

	for _, c := range candidates {
		if !c.IsAvailable {
			continue
		}

		hasAll := true
		for _, req := range required {
			found := false
			for _, cap := range c.Capabilities {
				if cap == req {
					found = true
					break
				}
			}
			if !found {
				hasAll = false
				break
			}
		}

		if hasAll {
			filtered = append(filtered, c)
		}
	}

	return filtered
}

// selectRoundRobin selects the next worker in round-robin order.
// Must be called with mu held.
func (s *SupervisorAgent) selectRoundRobin(candidates []*WorkerCapabilityInfo) (*WorkerCapabilityInfo, float64) {
	if len(candidates) == 0 {
		return nil, 0
	}

	// Filter to only available workers
	available := make([]*WorkerCapabilityInfo, 0)
	for _, c := range candidates {
		if c.IsAvailable {
			available = append(available, c)
		}
	}

	if len(available) == 0 {
		return nil, 0
	}

	// Use activeTaskCount as a simple round-robin index
	idx := s.activeTaskCount % len(available)
	return available[idx], 0.8 // Round-robin doesn't have a "match" concept
}

// selectLeastLoaded selects the worker with the fewest active tasks.
func (s *SupervisorAgent) selectLeastLoaded(candidates []*WorkerCapabilityInfo) (*WorkerCapabilityInfo, float64) {
	if len(candidates) == 0 {
		return nil, 0
	}

	var best *WorkerCapabilityInfo
	lowestLoad := -1

	for _, c := range candidates {
		if !c.IsAvailable {
			continue
		}

		if lowestLoad == -1 || c.ActiveTasks < lowestLoad {
			lowestLoad = c.ActiveTasks
			best = c
		}
	}

	if best == nil {
		return nil, 0
	}

	// Calculate score based on available capacity
	capacityUsed := float64(best.ActiveTasks) / float64(best.MaxTasks)
	matchScore := 1.0 - capacityUsed

	return best, matchScore
}

// selectByCapabilityMatch selects the worker that best matches required capabilities.
func (s *SupervisorAgent) selectByCapabilityMatch(candidates []*WorkerCapabilityInfo, required []string) (*WorkerCapabilityInfo, float64) {
	if len(candidates) == 0 {
		return nil, 0
	}

	var best *WorkerCapabilityInfo
	bestScore := -1.0

	for _, c := range candidates {
		if !c.IsAvailable {
			continue
		}

		// Calculate match score
		matchCount := 0
		for _, req := range required {
			for _, cap := range c.Capabilities {
				if cap == req {
					matchCount++
					break
				}
			}
		}

		var score float64
		if len(required) > 0 {
			score = float64(matchCount) / float64(len(required))
		} else {
			// No requirements, all workers match equally
			score = 1.0
		}

		// Bonus for having extra capabilities (versatility)
		if len(c.Capabilities) > len(required) {
			score += 0.1
			if score > 1.0 {
				score = 1.0
			}
		}

		// Slight penalty for higher load
		loadPenalty := float64(c.ActiveTasks) * 0.05
		score -= loadPenalty
		if score < 0 {
			score = 0
		}

		if score > bestScore {
			bestScore = score
			best = c
		}
	}

	return best, bestScore
}

// selectByPriority selects the highest priority worker that is available.
func (s *SupervisorAgent) selectByPriority(candidates []*WorkerCapabilityInfo) (*WorkerCapabilityInfo, float64) {
	if len(candidates) == 0 {
		return nil, 0
	}

	var best *WorkerCapabilityInfo
	highestPriority := -1

	for _, c := range candidates {
		if !c.IsAvailable {
			continue
		}

		if c.Priority > highestPriority {
			highestPriority = c.Priority
			best = c
		}
	}

	if best == nil {
		return nil, 0
	}

	// Normalize priority to 0-1 score (assuming max priority is 10)
	matchScore := float64(best.Priority) / 10.0
	if matchScore > 1.0 {
		matchScore = 1.0
	}

	return best, matchScore
}

// delegateToWorkerLocked performs the actual delegation to a selected worker.
// Must be called with mu held.
func (s *SupervisorAgent) delegateToWorkerLocked(task *TaskRequirements, worker *WorkerCapabilityInfo) (*DelegationResult, error) {
	// Increment active task count
	s.activeTaskCount++
	s.workerTaskCounts[worker.WorkerID]++

	// Calculate deadline
	timeoutSecs := task.TimeoutSeconds
	if timeoutSecs <= 0 {
		budget := s.getBudget()
		if budget != nil {
			timeoutSecs = budget.TimeoutSeconds
		} else {
			timeoutSecs = 600 // 10 minute default
		}
	}

	now := time.Now()
	deadline := now.Add(time.Duration(timeoutSecs) * time.Second)

	// Update worker metadata to track assigned task
	if workerNode, exists := s.pool.GetWorker(worker.WorkerID); exists {
		if workerNode.Metadata == nil {
			workerNode.Metadata = make(map[string]interface{})
		}
		workerNode.Metadata["last_task_id"] = task.TaskID
		workerNode.Metadata["last_task_assigned_at"] = now
		workerNode.LastUpdated = now
	}

	return &DelegationResult{
		Success:           true,
		WorkerID:          worker.WorkerID,
		Reason:            fmt.Sprintf("task delegated to worker %s", worker.WorkerID),
		DelegatedAt:       now,
		EstimatedDeadline: deadline,
	}, nil
}

// GetWorkerLoad returns the current load information for a specific worker.
//
// Parameters:
//   - workerID: The ID of the worker to check
//
// Returns:
//   - The number of active tasks
//   - The maximum tasks allowed
//   - An error if the worker is not found
func (s *SupervisorAgent) GetWorkerLoad(workerID string) (int, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, exists := s.pool.GetWorker(workerID); !exists {
		return 0, 0, fmt.Errorf("worker %q not found", workerID)
	}

	budget := s.getBudget()
	maxTasks := 3 // default
	if budget != nil {
		maxTasks = budget.MaxTasksPerWorker
	}

	// Count tasks using the tracking map
	activeTasks := s.workerTaskCounts[workerID]

	return activeTasks, maxTasks, nil
}

// SetWorkerCapabilities sets the capabilities for a worker.
//
// Parameters:
//   - workerID: The ID of the worker
//   - capabilities: The list of capabilities to set
//
// Returns:
//   - An error if the worker is not found
func (s *SupervisorAgent) SetWorkerCapabilities(workerID string, capabilities []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	worker, exists := s.pool.GetWorker(workerID)
	if !exists {
		return fmt.Errorf("worker %q not found", workerID)
	}

	if worker.Metadata == nil {
		worker.Metadata = make(map[string]interface{})
	}

	worker.Metadata["capabilities"] = capabilities
	worker.LastUpdated = time.Now()

	return nil
}

// SetWorkerPriority sets the priority for a worker (used in priority-based delegation).
//
// Parameters:
//   - workerID: The ID of the worker
//   - priority: The priority level (higher = preferred for delegation)
//
// Returns:
//   - An error if the worker is not found
func (s *SupervisorAgent) SetWorkerPriority(workerID string, priority int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	worker, exists := s.pool.GetWorker(workerID)
	if !exists {
		return fmt.Errorf("worker %q not found", workerID)
	}

	if worker.Metadata == nil {
		worker.Metadata = make(map[string]interface{})
	}

	worker.Metadata["priority"] = priority
	worker.LastUpdated = time.Now()

	return nil
}
