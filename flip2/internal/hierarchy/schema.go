// Package hierarchy provides 3-tier agent hierarchy schema and role definitions.
// This package implements the Coordinator → Supervisor → Worker hierarchy pattern
// for managing multi-agent orchestration in the FLIP system.
package hierarchy

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// AgentRole defines the position of an agent within the hierarchy.
type AgentRole string

const (
	// RoleCoordinator is the top-level agent that makes strategic decisions,
	// spawns supervisors, and manages the entire system.
	RoleCoordinator AgentRole = "coordinator"

	// RoleSupervisor is a middle-tier agent that supervises workers,
	// aggregates their results, and reports to the coordinator.
	// A supervisor can spawn workers up to its delegation budget.
	RoleSupervisor AgentRole = "supervisor"

	// RoleWorker is a bottom-tier agent that executes tasks,
	// reports results to its supervisor, and requests help when blocked.
	RoleWorker AgentRole = "worker"
)

// String returns the string representation of the role.
func (r AgentRole) String() string {
	return string(r)
}

// IsValid returns true if the role is a recognized hierarchy role.
func (r AgentRole) IsValid() bool {
	switch r {
	case RoleCoordinator, RoleSupervisor, RoleWorker:
		return true
	default:
		return false
	}
}

// DelegationBudget defines resource limits for agents that can spawn subordinates.
type DelegationBudget struct {
	// MaxWorkers is the maximum number of workers a supervisor can spawn.
	MaxWorkers int `json:"max_workers"`

	// MaxTasksPerWorker is the maximum concurrent tasks assigned to each worker.
	MaxTasksPerWorker int `json:"max_tasks_per_worker"`

	// MaxConcurrentSpawns limits how many workers can be spawned simultaneously.
	MaxConcurrentSpawns int `json:"max_concurrent_spawns"`

	// TimeoutSeconds is the maximum time a worker can spend on a single task.
	TimeoutSeconds int `json:"timeout_seconds"`
}

// EscalationPath defines how communication flows up the hierarchy.
type EscalationPath struct {
	// From is the role that initiates escalation.
	From AgentRole `json:"from"`

	// To is the role that receives the escalation.
	To AgentRole `json:"to"`

	// Reason describes why this escalation path exists.
	Reason string `json:"reason"`

	// RequiresApproval indicates if the escalation requires approval before action.
	RequiresApproval bool `json:"requires_approval"`

	// TimeoutSeconds is the max time to wait for escalation response.
	TimeoutSeconds int `json:"timeout_seconds"`
}

// Permissions defines what actions an agent with a specific role can perform.
type Permissions struct {
	// CanSpawnAgents allows creating new agents in the system.
	CanSpawnAgents bool `json:"can_spawn_agents"`

	// CanSpawnRole defines which roles this agent can spawn (e.g., supervisor can spawn workers).
	CanSpawnRole []AgentRole `json:"can_spawn_role,omitempty"`

	// CanViewHierarchy allows viewing the complete system hierarchy structure.
	CanViewHierarchy bool `json:"can_view_hierarchy"`

	// CanViewAllAgents allows viewing information about all agents in the system.
	CanViewAllAgents bool `json:"can_view_all_agents"`

	// CanViewOwnSubordinates allows viewing direct children in the hierarchy.
	CanViewOwnSubordinates bool `json:"can_view_own_subordinates"`

	// CanAssignTasks allows assigning tasks to agents.
	CanAssignTasks bool `json:"can_assign_tasks"`

	// CanMakeBroadcast allows sending messages to all agents in the system.
	CanMakeBroadcast bool `json:"can_make_broadcast"`

	// CanEscalate allows sending escalation requests up the hierarchy.
	CanEscalate bool `json:"can_escalate"`

	// CanAggregateResults allows collecting and combining results from subordinates.
	CanAggregateResults bool `json:"can_aggregate_results"`

	// CanTerminateWorkers allows terminating subordinate agents.
	CanTerminateWorkers bool `json:"can_terminate_workers"`

	// MaxBroadcastSize is the maximum number of agents that can receive a broadcast.
	// 0 means unlimited.
	MaxBroadcastSize int `json:"max_broadcast_size"`
}

// RoleCapabilities defines what each role can do in the hierarchy.
type RoleCapabilities struct {
	// Role is the hierarchy role.
	Role AgentRole `json:"role"`

	// Permissions for this role.
	Permissions Permissions `json:"permissions"`

	// DelegationBudget for roles that can spawn subordinates.
	DelegationBudget *DelegationBudget `json:"delegation_budget,omitempty"`

	// Description explains the role's purpose.
	Description string `json:"description"`

	// ResponsibilitiesRequired lists actions the role must perform.
	ResponsibilitiesRequired []string `json:"responsibilities_required,omitempty"`

	// ProhibitedActions lists actions the role cannot perform.
	ProhibitedActions []string `json:"prohibited_actions,omitempty"`
}

// TaskAssignment tracks an assigned task and its deadline.
type TaskAssignment struct {
	// TaskID is the unique identifier for the task.
	TaskID string `json:"task_id"`

	// AssignedAt is when the task was assigned.
	AssignedAt time.Time `json:"assigned_at"`

	// DeadlineAt is the maximum time the task should complete.
	DeadlineAt time.Time `json:"deadline_at"`
}

// BudgetStatus tracks current consumption of a worker's budget.
type BudgetStatus struct {
	// CurrentTaskCount is the number of currently assigned tasks.
	CurrentTaskCount int `json:"current_task_count"`

	// LastResetAt is when the budget was last reset (for monitoring).
	LastResetAt time.Time `json:"last_reset_at"`

	// AssignedTasks tracks all active task assignments.
	AssignedTasks []*TaskAssignment `json:"assigned_tasks,omitempty"`

	// ExpiredTaskCount tracks tasks that exceeded their timeout.
	ExpiredTaskCount int `json:"expired_task_count"`
}

// HierarchyNode represents an agent within the hierarchy tree.
type HierarchyNode struct {
	// AgentID is the unique identifier for this agent.
	AgentID string `json:"agent_id"`

	// Role is the agent's position in the hierarchy.
	Role AgentRole `json:"role"`

	// ParentID is the ID of the parent agent (nil for coordinator).
	ParentID *string `json:"parent_id,omitempty"`

	// ChildrenIDs are the IDs of direct subordinate agents.
	ChildrenIDs []string `json:"children_ids,omitempty"`

	// Capabilities describes what this agent can do.
	Capabilities *RoleCapabilities `json:"capabilities,omitempty"`

	// CreatedAt is when this node was added to the hierarchy.
	CreatedAt time.Time `json:"created_at"`

	// LastUpdated is when this node was last modified.
	LastUpdated time.Time `json:"last_updated"`

	// Status tracks the agent's operational status.
	Status string `json:"status"` // "active", "inactive", "failed"

	// Metadata stores arbitrary additional information.
	Metadata map[string]interface{} `json:"metadata,omitempty"`

	// BudgetStatus tracks the current budget consumption (for workers and supervisors with task limits).
	BudgetStatus *BudgetStatus `json:"budget_status,omitempty"`
}

// Hierarchy represents the complete 3-tier agent hierarchy.
type Hierarchy struct {
	mu sync.RWMutex

	// Coordinator is the root node (should be unique).
	Coordinator *HierarchyNode `json:"coordinator,omitempty"`

	// Supervisors maps supervisor agent IDs to their hierarchy nodes.
	Supervisors map[string]*HierarchyNode `json:"supervisors,omitempty"`

	// Workers maps worker agent IDs to their hierarchy nodes.
	Workers map[string]*HierarchyNode `json:"workers,omitempty"`

	// RoleCapabilities defines permissions for each role.
	RoleCapabilities map[AgentRole]*RoleCapabilities `json:"role_capabilities,omitempty"`

	// EscalationPaths defines how communication flows up the hierarchy.
	EscalationPaths []*EscalationPath `json:"escalation_paths,omitempty"`

	// CreatedAt is when the hierarchy was initialized.
	CreatedAt time.Time `json:"created_at"`

	// LastUpdated is the last time the hierarchy was modified.
	LastUpdated time.Time `json:"last_updated"`
}

// NewHierarchy creates a new empty hierarchy with default role capabilities.
func NewHierarchy() *Hierarchy {
	h := &Hierarchy{
		Supervisors:      make(map[string]*HierarchyNode),
		Workers:          make(map[string]*HierarchyNode),
		RoleCapabilities: make(map[AgentRole]*RoleCapabilities),
		EscalationPaths:  make([]*EscalationPath, 0),
		CreatedAt:        time.Now(),
		LastUpdated:      time.Now(),
	}

	// Initialize default role capabilities
	h.RoleCapabilities[RoleCoordinator] = DefaultCoordinatorCapabilities()
	h.RoleCapabilities[RoleSupervisor] = DefaultSupervisorCapabilities()
	h.RoleCapabilities[RoleWorker] = DefaultWorkerCapabilities()

	// Initialize default escalation paths
	h.EscalationPaths = DefaultEscalationPaths()

	return h
}

// SetCoordinator sets the root coordinator agent.
func (h *Hierarchy) SetCoordinator(agentID string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if agentID == "" {
		return fmt.Errorf("coordinator agent ID cannot be empty")
	}

	now := time.Now()
	h.Coordinator = &HierarchyNode{
		AgentID:    agentID,
		Role:       RoleCoordinator,
		ParentID:   nil,
		CreatedAt:  now,
		LastUpdated: now,
		Status:     "active",
		ChildrenIDs: make([]string, 0),
		Capabilities: h.RoleCapabilities[RoleCoordinator],
	}

	h.LastUpdated = now
	return nil
}

// AddSupervisor adds a supervisor to the hierarchy.
// The supervisor is a direct child of the coordinator.
func (h *Hierarchy) AddSupervisor(agentID string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.Coordinator == nil {
		return fmt.Errorf("coordinator must be set before adding supervisors")
	}

	if agentID == "" {
		return fmt.Errorf("supervisor agent ID cannot be empty")
	}

	if _, exists := h.Supervisors[agentID]; exists {
		return fmt.Errorf("supervisor %q already exists", agentID)
	}

	coordinatorID := h.Coordinator.AgentID
	now := time.Now()

	supervisor := &HierarchyNode{
		AgentID:      agentID,
		Role:         RoleSupervisor,
		ParentID:     &coordinatorID,
		ChildrenIDs:  make([]string, 0),
		CreatedAt:    now,
		LastUpdated:  now,
		Status:       "active",
		Capabilities: h.RoleCapabilities[RoleSupervisor],
	}

	h.Supervisors[agentID] = supervisor
	h.Coordinator.ChildrenIDs = append(h.Coordinator.ChildrenIDs, agentID)
	h.LastUpdated = now

	return nil
}

// AddWorker adds a worker to the hierarchy under a specific supervisor.
func (h *Hierarchy) AddWorker(workerID, supervisorID string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if workerID == "" {
		return fmt.Errorf("worker agent ID cannot be empty")
	}

	if supervisorID == "" {
		return fmt.Errorf("supervisor ID cannot be empty")
	}

	supervisor, exists := h.Supervisors[supervisorID]
	if !exists {
		return fmt.Errorf("supervisor %q not found", supervisorID)
	}

	if _, exists := h.Workers[workerID]; exists {
		return fmt.Errorf("worker %q already exists", workerID)
	}

	// Check delegation budget
	budget := h.RoleCapabilities[RoleSupervisor].DelegationBudget
	if budget != nil && len(supervisor.ChildrenIDs) >= budget.MaxWorkers {
		return fmt.Errorf("supervisor %q has reached max workers limit (%d)", supervisorID, budget.MaxWorkers)
	}

	now := time.Now()
	worker := &HierarchyNode{
		AgentID:      workerID,
		Role:         RoleWorker,
		ParentID:     &supervisorID,
		ChildrenIDs:  make([]string, 0), // Workers have no children
		CreatedAt:    now,
		LastUpdated:  now,
		Status:       "active",
		Capabilities: h.RoleCapabilities[RoleWorker],
		BudgetStatus: &BudgetStatus{
			CurrentTaskCount: 0,
			LastResetAt:      now,
			AssignedTasks:    make([]*TaskAssignment, 0),
			ExpiredTaskCount: 0,
		},
	}

	h.Workers[workerID] = worker
	supervisor.ChildrenIDs = append(supervisor.ChildrenIDs, workerID)
	h.LastUpdated = now

	return nil
}

// RemoveWorker removes a worker from the hierarchy.
func (h *Hierarchy) RemoveWorker(workerID string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	worker, exists := h.Workers[workerID]
	if !exists {
		return fmt.Errorf("worker %q not found", workerID)
	}

	if worker.ParentID == nil {
		return fmt.Errorf("worker %q has no parent supervisor", workerID)
	}

	supervisor, exists := h.Supervisors[*worker.ParentID]
	if !exists {
		return fmt.Errorf("parent supervisor not found for worker %q", workerID)
	}

	// Remove from supervisor's children
	newChildren := make([]string, 0)
	for _, childID := range supervisor.ChildrenIDs {
		if childID != workerID {
			newChildren = append(newChildren, childID)
		}
	}
	supervisor.ChildrenIDs = newChildren

	delete(h.Workers, workerID)
	h.LastUpdated = time.Now()

	return nil
}

// RemoveSupervisor removes a supervisor from the hierarchy.
// Returns error if supervisor has active workers.
func (h *Hierarchy) RemoveSupervisor(supervisorID string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	supervisor, exists := h.Supervisors[supervisorID]
	if !exists {
		return fmt.Errorf("supervisor %q not found", supervisorID)
	}

	if len(supervisor.ChildrenIDs) > 0 {
		return fmt.Errorf("supervisor %q has active workers, cannot remove", supervisorID)
	}

	if h.Coordinator == nil {
		return fmt.Errorf("coordinator not found")
	}

	// Remove from coordinator's children
	newChildren := make([]string, 0)
	for _, childID := range h.Coordinator.ChildrenIDs {
		if childID != supervisorID {
			newChildren = append(newChildren, childID)
		}
	}
	h.Coordinator.ChildrenIDs = newChildren

	delete(h.Supervisors, supervisorID)
	h.LastUpdated = time.Now()

	return nil
}

// GetNode retrieves a hierarchy node by agent ID.
func (h *Hierarchy) GetNode(agentID string) (*HierarchyNode, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.Coordinator != nil && h.Coordinator.AgentID == agentID {
		return h.Coordinator, nil
	}

	if supervisor, exists := h.Supervisors[agentID]; exists {
		return supervisor, nil
	}

	if worker, exists := h.Workers[agentID]; exists {
		return worker, nil
	}

	return nil, fmt.Errorf("agent %q not found in hierarchy", agentID)
}

// GetParent returns the parent node of the given agent.
func (h *Hierarchy) GetParent(agentID string) (*HierarchyNode, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	node, err := h.getNodeUnsafe(agentID)
	if err != nil {
		return nil, err
	}

	if node.ParentID == nil {
		return nil, fmt.Errorf("agent %q has no parent", agentID)
	}

	return h.getNodeUnsafe(*node.ParentID)
}

// GetChildren returns all direct children of the given agent.
func (h *Hierarchy) GetChildren(agentID string) ([]*HierarchyNode, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	node, err := h.getNodeUnsafe(agentID)
	if err != nil {
		return nil, err
	}

	children := make([]*HierarchyNode, 0, len(node.ChildrenIDs))
	for _, childID := range node.ChildrenIDs {
		child, _ := h.getNodeUnsafe(childID)
		if child != nil {
			children = append(children, child)
		}
	}

	return children, nil
}

// GetAncestors returns all ancestors up to the root coordinator.
func (h *Hierarchy) GetAncestors(agentID string) ([]*HierarchyNode, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	ancestors := make([]*HierarchyNode, 0)

	current, err := h.getNodeUnsafe(agentID)
	if err != nil {
		return nil, err
	}

	for current.ParentID != nil {
		parent, err := h.getNodeUnsafe(*current.ParentID)
		if err != nil {
			break
		}
		ancestors = append(ancestors, parent)
		current = parent
	}

	return ancestors, nil
}

// FindEscalationPath returns the escalation path between two roles.
func (h *Hierarchy) FindEscalationPath(from, to AgentRole) *EscalationPath {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, path := range h.EscalationPaths {
		if path.From == from && path.To == to {
			return path
		}
	}
	return nil
}

// GetRoleCapabilities returns the capabilities for a specific role.
func (h *Hierarchy) GetRoleCapabilities(role AgentRole) *RoleCapabilities {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if cap, exists := h.RoleCapabilities[role]; exists {
		return cap
	}
	return nil
}

// CanAgentSpawnRole checks if an agent with a given role can spawn agents of a target role.
func (h *Hierarchy) CanAgentSpawnRole(agentRole, targetRole AgentRole) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()

	cap, exists := h.RoleCapabilities[agentRole]
	if !exists || !cap.Permissions.CanSpawnAgents {
		return false
	}

	for _, allowed := range cap.Permissions.CanSpawnRole {
		if allowed == targetRole {
			return true
		}
	}

	return false
}

// CanAgentEscalateTo checks if an agent can escalate to a specific role.
func (h *Hierarchy) CanAgentEscalateTo(fromRole, toRole AgentRole) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()

	cap, exists := h.RoleCapabilities[fromRole]
	if !exists {
		return false
	}

	if !cap.Permissions.CanEscalate {
		return false
	}

	// Check escalation paths
	for _, path := range h.EscalationPaths {
		if path.From == fromRole && path.To == toRole {
			return true
		}
	}

	return false
}

// AssignTaskToWorker assigns a task to a worker, enforcing delegation budgets.
// Returns an error if the worker has reached task limits or if the task would exceed timeout.
func (h *Hierarchy) AssignTaskToWorker(workerID, taskID string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	worker, exists := h.Workers[workerID]
	if !exists {
		return fmt.Errorf("worker %q not found", workerID)
	}

	if worker.BudgetStatus == nil {
		return fmt.Errorf("worker %q has no budget status initialized", workerID)
	}

	// Get supervisor and their delegation budget
	if worker.ParentID == nil {
		return fmt.Errorf("worker %q has no parent supervisor", workerID)
	}

	supervisor, exists := h.Supervisors[*worker.ParentID]
	if !exists {
		return fmt.Errorf("supervisor %q not found for worker %q", *worker.ParentID, workerID)
	}

	budget := supervisor.Capabilities.DelegationBudget
	if budget == nil {
		return fmt.Errorf("supervisor %q has no delegation budget", *worker.ParentID)
	}

	// Check if worker has reached task limit
	if worker.BudgetStatus.CurrentTaskCount >= budget.MaxTasksPerWorker {
		return fmt.Errorf("worker %q has reached max concurrent tasks limit (%d/%d)",
			workerID, worker.BudgetStatus.CurrentTaskCount, budget.MaxTasksPerWorker)
	}

	// Create task assignment with timeout deadline
	now := time.Now()
	deadline := now.Add(time.Duration(budget.TimeoutSeconds) * time.Second)

	assignment := &TaskAssignment{
		TaskID:     taskID,
		AssignedAt: now,
		DeadlineAt: deadline,
	}

	// Add task to worker's budget status
	worker.BudgetStatus.AssignedTasks = append(worker.BudgetStatus.AssignedTasks, assignment)
	worker.BudgetStatus.CurrentTaskCount++
	worker.LastUpdated = now
	h.LastUpdated = now

	return nil
}

// CompleteTaskForWorker marks a task as complete, freeing up the worker's task budget.
func (h *Hierarchy) CompleteTaskForWorker(workerID, taskID string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	worker, exists := h.Workers[workerID]
	if !exists {
		return fmt.Errorf("worker %q not found", workerID)
	}

	if worker.BudgetStatus == nil {
		return fmt.Errorf("worker %q has no budget status initialized", workerID)
	}

	// Find and remove the task
	found := false
	newTasks := make([]*TaskAssignment, 0, len(worker.BudgetStatus.AssignedTasks))

	for _, task := range worker.BudgetStatus.AssignedTasks {
		if task.TaskID == taskID {
			found = true
			// Check if task exceeded timeout
			if time.Now().After(task.DeadlineAt) {
				worker.BudgetStatus.ExpiredTaskCount++
			}
		} else {
			newTasks = append(newTasks, task)
		}
	}

	if !found {
		return fmt.Errorf("task %q not found for worker %q", taskID, workerID)
	}

	worker.BudgetStatus.AssignedTasks = newTasks
	worker.BudgetStatus.CurrentTaskCount--
	if worker.BudgetStatus.CurrentTaskCount < 0 {
		worker.BudgetStatus.CurrentTaskCount = 0
	}
	worker.LastUpdated = time.Now()
	h.LastUpdated = time.Now()

	return nil
}

// ValidateBudgetForWorkerSpawn checks if supervisor can spawn a new worker given current spawn load.
// Returns an error if concurrent spawn limit is exceeded.
func (h *Hierarchy) ValidateBudgetForWorkerSpawn(supervisorID string) error {
	h.mu.RLock()
	defer h.mu.RUnlock()

	supervisor, exists := h.Supervisors[supervisorID]
	if !exists {
		return fmt.Errorf("supervisor %q not found", supervisorID)
	}

	if supervisor.Capabilities == nil || supervisor.Capabilities.DelegationBudget == nil {
		return fmt.Errorf("supervisor %q has no delegation budget", supervisorID)
	}

	budget := supervisor.Capabilities.DelegationBudget
	currentWorkerCount := len(supervisor.ChildrenIDs)
	maxWorkers := budget.MaxWorkers

	if currentWorkerCount >= maxWorkers {
		return fmt.Errorf("supervisor %q at max workers capacity (%d/%d), cannot spawn new workers",
			supervisorID, currentWorkerCount, maxWorkers)
	}

	return nil
}

// GetWorkerTaskLoad returns the current task load for a worker.
func (h *Hierarchy) GetWorkerTaskLoad(workerID string) (int, int, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	worker, exists := h.Workers[workerID]
	if !exists {
		return 0, 0, fmt.Errorf("worker %q not found", workerID)
	}

	if worker.BudgetStatus == nil {
		return 0, 0, nil
	}

	return worker.BudgetStatus.CurrentTaskCount, worker.BudgetStatus.ExpiredTaskCount, nil
}

// ResetWorkerBudget resets a worker's task budget for a new period.
// This is typically called periodically to allow fresh task assignments.
func (h *Hierarchy) ResetWorkerBudget(workerID string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	worker, exists := h.Workers[workerID]
	if !exists {
		return fmt.Errorf("worker %q not found", workerID)
	}

	if worker.BudgetStatus == nil {
		return fmt.Errorf("worker %q has no budget status initialized", workerID)
	}

	now := time.Now()
	worker.BudgetStatus.CurrentTaskCount = 0
	worker.BudgetStatus.LastResetAt = now
	worker.BudgetStatus.AssignedTasks = make([]*TaskAssignment, 0)
	worker.LastUpdated = now
	h.LastUpdated = now

	return nil
}

// PruneExpiredTasks removes task assignments that exceeded their deadline.
// Returns the number of tasks pruned.
func (h *Hierarchy) PruneExpiredTasks(workerID string) (int, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	worker, exists := h.Workers[workerID]
	if !exists {
		return 0, fmt.Errorf("worker %q not found", workerID)
	}

	if worker.BudgetStatus == nil {
		return 0, nil
	}

	now := time.Now()
	pruned := 0
	activeTasks := make([]*TaskAssignment, 0, len(worker.BudgetStatus.AssignedTasks))

	for _, task := range worker.BudgetStatus.AssignedTasks {
		if now.After(task.DeadlineAt) {
			pruned++
			worker.BudgetStatus.ExpiredTaskCount++
		} else {
			activeTasks = append(activeTasks, task)
		}
	}

	worker.BudgetStatus.AssignedTasks = activeTasks
	worker.BudgetStatus.CurrentTaskCount = len(activeTasks)
	worker.LastUpdated = now
	h.LastUpdated = now

	return pruned, nil
}

// CountAgentsByRole returns the number of agents of each role type.
func (h *Hierarchy) CountAgentsByRole() map[AgentRole]int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	counts := map[AgentRole]int{
		RoleCoordinator: 0,
		RoleSupervisor:  0,
		RoleWorker:      0,
	}

	if h.Coordinator != nil {
		counts[RoleCoordinator] = 1
	}

	counts[RoleSupervisor] = len(h.Supervisors)
	counts[RoleWorker] = len(h.Workers)

	return counts
}

// ToJSON serializes the hierarchy to JSON.
func (h *Hierarchy) ToJSON() ([]byte, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return json.Marshal(h)
}

// FromJSON deserializes a hierarchy from JSON.
func FromJSON(data []byte) (*Hierarchy, error) {
	var h Hierarchy
	if err := json.Unmarshal(data, &h); err != nil {
		return nil, err
	}
	return &h, nil
}

// getNodeUnsafe retrieves a node without locking (must be called with mutex held).
func (h *Hierarchy) getNodeUnsafe(agentID string) (*HierarchyNode, error) {
	if h.Coordinator != nil && h.Coordinator.AgentID == agentID {
		return h.Coordinator, nil
	}

	if supervisor, exists := h.Supervisors[agentID]; exists {
		return supervisor, nil
	}

	if worker, exists := h.Workers[agentID]; exists {
		return worker, nil
	}

	return nil, fmt.Errorf("agent %q not found", agentID)
}

// DefaultCoordinatorCapabilities returns the default permissions for coordinators.
func DefaultCoordinatorCapabilities() *RoleCapabilities {
	return &RoleCapabilities{
		Role: RoleCoordinator,
		Description: "Top-level agent that makes strategic decisions, spawns supervisors, and manages the entire system.",
		Permissions: Permissions{
			CanSpawnAgents:        true,
			CanSpawnRole:          []AgentRole{RoleSupervisor},
			CanViewHierarchy:      true,
			CanViewAllAgents:      true,
			CanViewOwnSubordinates: true,
			CanAssignTasks:        true,
			CanMakeBroadcast:      true,
			CanEscalate:           false, // Coordinator is at the top
			CanAggregateResults:   true,
			CanTerminateWorkers:   true,
			MaxBroadcastSize:      0, // Unlimited
		},
		DelegationBudget: &DelegationBudget{
			MaxWorkers:          0, // Coordinators don't spawn workers directly
			MaxTasksPerWorker:   0, // N/A
			MaxConcurrentSpawns: 10,
			TimeoutSeconds:      3600,
		},
		ResponsibilitiesRequired: []string{
			"Monitor overall system health",
			"Decide which supervisors to spawn",
			"Handle critical escalations",
			"Provide strategic direction",
			"Track system-wide metrics",
		},
		ProhibitedActions: []string{
			"Execute work tasks directly",
			"Modify worker assignments without supervisor knowledge",
			"Make architectural changes without testing",
		},
	}
}

// DefaultSupervisorCapabilities returns the default permissions for supervisors.
func DefaultSupervisorCapabilities() *RoleCapabilities {
	return &RoleCapabilities{
		Role: RoleSupervisor,
		Description: "Middle-tier agent that supervises workers, aggregates results, and escalates issues.",
		Permissions: Permissions{
			CanSpawnAgents:        true,
			CanSpawnRole:          []AgentRole{RoleWorker},
			CanViewHierarchy:      true,
			CanViewAllAgents:      false,
			CanViewOwnSubordinates: true,
			CanAssignTasks:        true,
			CanMakeBroadcast:      false,
			CanEscalate:           true,
			CanAggregateResults:   true,
			CanTerminateWorkers:   true,
			MaxBroadcastSize:      0, // Can only broadcast to own workers
		},
		DelegationBudget: &DelegationBudget{
			MaxWorkers:          5, // Can spawn up to 5 workers
			MaxTasksPerWorker:   3, // Each worker can have up to 3 concurrent tasks
			MaxConcurrentSpawns: 2, // Spawn at most 2 workers simultaneously
			TimeoutSeconds:      600, // 10 minutes per task
		},
		ResponsibilitiesRequired: []string{
			"Spawn and manage workers",
			"Assign tasks to workers",
			"Aggregate worker results",
			"Monitor worker health",
			"Escalate issues to coordinator",
			"Report progress regularly",
		},
		ProhibitedActions: []string{
			"Spawn agents other than workers",
			"Execute tasks directly (delegate to workers)",
			"Modify worker permissions",
			"Terminate other supervisors",
		},
	}
}

// DefaultWorkerCapabilities returns the default permissions for workers.
func DefaultWorkerCapabilities() *RoleCapabilities {
	return &RoleCapabilities{
		Role: RoleWorker,
		Description: "Bottom-tier agent that executes tasks, reports results, and requests help when blocked.",
		Permissions: Permissions{
			CanSpawnAgents:        false,
			CanSpawnRole:          []AgentRole{}, // Workers cannot spawn agents
			CanViewHierarchy:      false,
			CanViewAllAgents:      false,
			CanViewOwnSubordinates: false,
			CanAssignTasks:        false,
			CanMakeBroadcast:      false,
			CanEscalate:           true, // Can escalate issues
			CanAggregateResults:   false,
			CanTerminateWorkers:   false,
			MaxBroadcastSize:      1, // Can only signal to supervisor
		},
		DelegationBudget: nil, // Workers have no subordinates
		ResponsibilitiesRequired: []string{
			"Execute assigned tasks",
			"Report progress regularly",
			"Report results when complete",
			"Signal supervisor if blocked",
			"Ask for clarification when needed",
			"Do not make autonomous decisions",
		},
		ProhibitedActions: []string{
			"Spawn new agents",
			"Assign tasks to other workers",
			"Execute unapproved operations",
			"Modify system configuration",
			"Broadcast to multiple agents",
			"Make autonomous deployments",
		},
	}
}

// DefaultEscalationPaths returns the standard escalation paths in the hierarchy.
func DefaultEscalationPaths() []*EscalationPath {
	return []*EscalationPath{
		{
			From:             RoleWorker,
			To:               RoleSupervisor,
			Reason:           "Worker escalates blocked tasks or requests help",
			RequiresApproval: false,
			TimeoutSeconds:   60,
		},
		{
			From:             RoleSupervisor,
			To:               RoleCoordinator,
			Reason:           "Supervisor escalates critical issues or requests additional resources",
			RequiresApproval: false,
			TimeoutSeconds:   300,
		},
		{
			From:             RoleWorker,
			To:               RoleCoordinator,
			Reason:           "Worker escalates critical system failures (bypass supervisor)",
			RequiresApproval: true,
			TimeoutSeconds:   30,
		},
	}
}
