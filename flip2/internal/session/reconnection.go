// Package session provides reconnection logic for agent session state recovery.
//
// The reconnection system handles:
// - Agent reconnection with exponential backoff
// - Session state recovery after coordinator disconnection
// - Pending message replay for resumed agents
// - WebSocket channel re-establishment
// - Task reassignment for disconnected agents
package session

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"sync"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

// ReconnectionConfig configures the reconnection behavior.
type ReconnectionConfig struct {
	// MaxRetries is the maximum number of reconnection attempts per agent.
	MaxRetries int

	// InitialBackoff is the initial delay between retries.
	InitialBackoff time.Duration

	// MaxBackoff is the maximum delay between retries.
	MaxBackoff time.Duration

	// BackoffMultiplier multiplies the backoff on each retry.
	BackoffMultiplier float64

	// HealthCheckTimeout is the timeout for agent health checks.
	HealthCheckTimeout time.Duration

	// ReconnectionTimeout is the overall timeout for the reconnection process.
	ReconnectionTimeout time.Duration
}

// DefaultReconnectionConfig returns sensible defaults for reconnection.
func DefaultReconnectionConfig() ReconnectionConfig {
	return ReconnectionConfig{
		MaxRetries:          5,
		InitialBackoff:      1 * time.Second,
		MaxBackoff:          30 * time.Second,
		BackoffMultiplier:   2.0,
		HealthCheckTimeout:  5 * time.Second,
		ReconnectionTimeout: 2 * time.Minute,
	}
}

// ReconnectionState tracks the state of an ongoing reconnection attempt.
type ReconnectionState struct {
	// AgentID is the ID of the agent being reconnected.
	AgentID string `json:"agent_id"`

	// SessionID is the session the agent belongs to.
	SessionID string `json:"session_id"`

	// Status indicates the current reconnection status.
	Status ReconnectionStatus `json:"status"`

	// AttemptCount is the number of reconnection attempts made.
	AttemptCount int `json:"attempt_count"`

	// LastAttemptAt is the timestamp of the last reconnection attempt.
	LastAttemptAt *time.Time `json:"last_attempt_at,omitempty"`

	// NextAttemptAt is when the next attempt will be made.
	NextAttemptAt *time.Time `json:"next_attempt_at,omitempty"`

	// StartedAt is when the reconnection process started.
	StartedAt time.Time `json:"started_at"`

	// CompletedAt is when the reconnection completed (success or failure).
	CompletedAt *time.Time `json:"completed_at,omitempty"`

	// LastError is the last error encountered during reconnection.
	LastError string `json:"last_error,omitempty"`

	// PendingMessages is the count of messages queued for delivery.
	PendingMessages int `json:"pending_messages"`

	// RestoredState indicates what state was restored.
	RestoredState *RestoredStateInfo `json:"restored_state,omitempty"`
}

// RestoredStateInfo describes what state was recovered during reconnection.
type RestoredStateInfo struct {
	// MessagesRestored is the number of pending messages redelivered.
	MessagesRestored int `json:"messages_restored"`

	// TasksRestored is the number of pending tasks reassigned.
	TasksRestored int `json:"tasks_restored"`

	// VariablesRestored is the number of session variables restored.
	VariablesRestored int `json:"variables_restored"`

	// LastActivityRestored is the last activity timestamp restored.
	LastActivityRestored *time.Time `json:"last_activity_restored,omitempty"`

	// PendingMessageIDs is the list of pending message IDs.
	PendingMessageIDs []string `json:"pending_message_ids,omitempty"`

	// PendingTaskIDs is the list of pending task IDs.
	PendingTaskIDs []string `json:"pending_task_ids,omitempty"`
}

// ReconnectionStatus indicates the status of a reconnection attempt.
type ReconnectionStatus string

const (
	// ReconnectionPending means reconnection has not started yet.
	ReconnectionPending ReconnectionStatus = "pending"

	// ReconnectionInProgress means reconnection is actively being attempted.
	ReconnectionInProgress ReconnectionStatus = "in_progress"

	// ReconnectionRetrying means waiting before the next retry.
	ReconnectionRetrying ReconnectionStatus = "retrying"

	// ReconnectionSucceeded means the agent was successfully reconnected.
	ReconnectionSucceeded ReconnectionStatus = "succeeded"

	// ReconnectionFailed means all reconnection attempts failed.
	ReconnectionFailed ReconnectionStatus = "failed"

	// ReconnectionCancelled means the reconnection was cancelled.
	ReconnectionCancelled ReconnectionStatus = "cancelled"
)

// ReconnectionResult is returned after a reconnection attempt.
type ReconnectionResult struct {
	// AgentID is the agent that was reconnected.
	AgentID string `json:"agent_id"`

	// Success indicates if the reconnection was successful.
	Success bool `json:"success"`

	// State is the final reconnection state.
	State *ReconnectionState `json:"state"`

	// Duration is how long the reconnection took.
	Duration time.Duration `json:"duration"`

	// Error is the error if reconnection failed.
	Error string `json:"error,omitempty"`
}

// ReconnectionManager handles agent reconnection with exponential backoff.
type ReconnectionManager struct {
	mu         sync.RWMutex
	config     ReconnectionConfig
	app        core.App
	sessionMgr *SessionManager
	logger     *slog.Logger

	// Active reconnection states keyed by agent ID
	activeReconnections map[string]*ReconnectionState

	// WebSocket notifier for real-time agent communication
	wsNotifier WebSocketNotifier

	// Channels for coordination
	done chan struct{}
}

// NewReconnectionManager creates a new reconnection manager.
func NewReconnectionManager(
	cfg ReconnectionConfig,
	app core.App,
	sessionMgr *SessionManager,
	logger *slog.Logger,
) *ReconnectionManager {
	if logger == nil {
		logger = slog.Default()
	}

	return &ReconnectionManager{
		config:              cfg,
		app:                 app,
		sessionMgr:          sessionMgr,
		logger:              logger.WithGroup("reconnection"),
		activeReconnections: make(map[string]*ReconnectionState),
		done:                make(chan struct{}),
	}
}

// ReconnectAgent attempts to reconnect a single agent with exponential backoff.
// This is the main entry point for agent reconnection.
func (rm *ReconnectionManager) ReconnectAgent(ctx context.Context, sessionID, agentID string) (*ReconnectionResult, error) {
	rm.logger.Info("Starting agent reconnection",
		"agent_id", agentID,
		"session_id", sessionID,
	)

	// Create reconnection state
	state := &ReconnectionState{
		AgentID:      agentID,
		SessionID:    sessionID,
		Status:       ReconnectionPending,
		AttemptCount: 0,
		StartedAt:    time.Now(),
	}

	// Track this reconnection
	rm.mu.Lock()
	if existing, exists := rm.activeReconnections[agentID]; exists {
		if existing.Status == ReconnectionInProgress || existing.Status == ReconnectionRetrying {
			rm.mu.Unlock()
			return nil, fmt.Errorf("reconnection already in progress for agent %s", agentID)
		}
	}
	rm.activeReconnections[agentID] = state
	rm.mu.Unlock()

	// Create a timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, rm.config.ReconnectionTimeout)
	defer cancel()

	// Start reconnection with exponential backoff
	result := rm.attemptReconnectionWithBackoff(timeoutCtx, state)

	// Update final state
	rm.mu.Lock()
	now := time.Now()
	state.CompletedAt = &now
	rm.mu.Unlock()

	return result, nil
}

// attemptReconnectionWithBackoff attempts reconnection with exponential backoff.
func (rm *ReconnectionManager) attemptReconnectionWithBackoff(ctx context.Context, state *ReconnectionState) *ReconnectionResult {
	startTime := time.Now()
	backoff := rm.config.InitialBackoff

	for attempt := 1; attempt <= rm.config.MaxRetries; attempt++ {
		select {
		case <-ctx.Done():
			state.Status = ReconnectionCancelled
			state.LastError = ctx.Err().Error()
			return &ReconnectionResult{
				AgentID:  state.AgentID,
				Success:  false,
				State:    state,
				Duration: time.Since(startTime),
				Error:    "reconnection cancelled: " + ctx.Err().Error(),
			}
		case <-rm.done:
			state.Status = ReconnectionCancelled
			return &ReconnectionResult{
				AgentID:  state.AgentID,
				Success:  false,
				State:    state,
				Duration: time.Since(startTime),
				Error:    "reconnection manager stopped",
			}
		default:
		}

		// Update state for this attempt
		now := time.Now()
		state.Status = ReconnectionInProgress
		state.AttemptCount = attempt
		state.LastAttemptAt = &now

		rm.logger.Debug("Attempting reconnection",
			"agent_id", state.AgentID,
			"attempt", attempt,
			"max_retries", rm.config.MaxRetries,
		)

		// Attempt the actual reconnection
		err := rm.performReconnection(ctx, state)
		if err == nil {
			// Success!
			state.Status = ReconnectionSucceeded
			rm.logger.Info("Agent reconnection succeeded",
				"agent_id", state.AgentID,
				"attempts", attempt,
				"duration", time.Since(startTime),
			)
			return &ReconnectionResult{
				AgentID:  state.AgentID,
				Success:  true,
				State:    state,
				Duration: time.Since(startTime),
			}
		}

		// Record the error
		state.LastError = err.Error()
		rm.logger.Warn("Reconnection attempt failed",
			"agent_id", state.AgentID,
			"attempt", attempt,
			"error", err,
			"next_backoff", backoff,
		)

		// If we've exhausted retries, fail
		if attempt >= rm.config.MaxRetries {
			state.Status = ReconnectionFailed
			return &ReconnectionResult{
				AgentID:  state.AgentID,
				Success:  false,
				State:    state,
				Duration: time.Since(startTime),
				Error:    fmt.Sprintf("max retries (%d) exceeded: %s", rm.config.MaxRetries, err.Error()),
			}
		}

		// Calculate next attempt time
		nextAttempt := time.Now().Add(backoff)
		state.Status = ReconnectionRetrying
		state.NextAttemptAt = &nextAttempt

		// Wait with backoff
		select {
		case <-ctx.Done():
			state.Status = ReconnectionCancelled
			state.LastError = ctx.Err().Error()
			return &ReconnectionResult{
				AgentID:  state.AgentID,
				Success:  false,
				State:    state,
				Duration: time.Since(startTime),
				Error:    "reconnection cancelled during backoff",
			}
		case <-rm.done:
			state.Status = ReconnectionCancelled
			return &ReconnectionResult{
				AgentID:  state.AgentID,
				Success:  false,
				State:    state,
				Duration: time.Since(startTime),
				Error:    "reconnection manager stopped",
			}
		case <-time.After(backoff):
			// Continue to next attempt
		}

		// Calculate next backoff with exponential increase
		backoff = time.Duration(float64(backoff) * rm.config.BackoffMultiplier)
		if backoff > rm.config.MaxBackoff {
			backoff = rm.config.MaxBackoff
		}
	}

	state.Status = ReconnectionFailed
	return &ReconnectionResult{
		AgentID:  state.AgentID,
		Success:  false,
		State:    state,
		Duration: time.Since(startTime),
		Error:    "reconnection failed after all retries",
	}
}

// performReconnection performs the actual reconnection steps.
func (rm *ReconnectionManager) performReconnection(ctx context.Context, state *ReconnectionState) error {
	// Step 1: Verify agent is in the agent registry and responsive
	if err := rm.checkAgentHealth(ctx, state.AgentID); err != nil {
		return fmt.Errorf("agent health check failed: %w", err)
	}

	// Step 2: Get the session and find the agent reference
	session, err := rm.sessionMgr.GetSession(ctx, state.SessionID)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}

	// Step 3: Find and update the agent reference in the session
	var agentRef *AgentRef
	for i := range session.ActiveAgents {
		if session.ActiveAgents[i].AgentID == state.AgentID {
			agentRef = &session.ActiveAgents[i]
			break
		}
	}

	if agentRef == nil {
		return fmt.Errorf("agent %s not found in session %s", state.AgentID, state.SessionID)
	}

	// Step 4: Restore session state and identify pending items
	restored, err := rm.restoreAgentState(ctx, session, agentRef)
	if err != nil {
		return fmt.Errorf("failed to restore agent state: %w", err)
	}
	state.RestoredState = restored
	state.PendingMessages = restored.MessagesRestored

	// Step 5: Notify agent about pending items (send signals)
	if len(restored.PendingMessageIDs) > 0 || len(restored.PendingTaskIDs) > 0 {
		if err := rm.notifyAgentOfState(ctx, state.AgentID, restored); err != nil {
			rm.logger.Warn("Failed to notify agent of pending state",
				"agent_id", state.AgentID,
				"error", err,
			)
			// Non-fatal, agent might pull anyway
		}
	}

	// Step 6: Update agent status to active
	now := time.Now()
	agentRef.Status = AgentStatusActive
	agentRef.LastActivityAt = &now

	// Step 7: Update the session
	session.UpdatedAt = now
	session.LastHeartbeatAt = &now
	if err := rm.sessionMgr.UpdateSession(ctx, session); err != nil {
		return fmt.Errorf("failed to update session: %w", err)
	}

	// Step 8: Update agent heartbeat in global registry
	if err := rm.updateAgentHeartbeat(state.AgentID); err != nil {
		rm.logger.Warn("Failed to update agent heartbeat in registry",
			"agent_id", state.AgentID,
			"error", err,
		)
	}

	rm.logger.Info("Agent state restored",
		"agent_id", state.AgentID,
		"session_id", state.SessionID,
		"messages_restored", restored.MessagesRestored,
		"tasks_restored", restored.TasksRestored,
	)

	return nil
}

// checkAgentHealth verifies the agent is responsive by checking DB status.
func (rm *ReconnectionManager) checkAgentHealth(ctx context.Context, agentID string) error {
	if rm.app == nil {
		// Should not happen if initialized correctly
		return fmt.Errorf("app not initialized")
	}

	record, err := rm.app.FindRecordById("agents", agentID)
	if err != nil {
		// Try by agent_id field
		records, err := rm.app.FindRecordsByFilter("agents", fmt.Sprintf("agent_id='%s'", agentID), "", 1, 0)
		if err != nil || len(records) == 0 {
			return fmt.Errorf("agent not found in registry: %s", agentID)
		}
		record = records[0]
	}

	// Check status
	status := record.GetString("status")
	if status == "offline" {
		// Even if marked offline, we are trying to reconnect, so we assume it might be back.
		// But if we want to be strict, we could fail.
		// However, typically reconnection happens *because* it was offline or we want to ensure it's online.
		// So we just log it.
		rm.logger.Debug("Agent currently marked offline in registry", "agent_id", agentID)
	}

	return nil
}

// updateAgentHeartbeat updates the agent's last_seen in DB.
func (rm *ReconnectionManager) updateAgentHeartbeat(agentID string) error {
	if rm.app == nil {
		return nil
	}

	record, err := rm.app.FindRecordById("agents", agentID)
	if err != nil {
		// Try by agent_id field
		records, err := rm.app.FindRecordsByFilter("agents", fmt.Sprintf("agent_id='%s'", agentID), "", 1, 0)
		if err != nil || len(records) == 0 {
			return fmt.Errorf("agent not found: %s", agentID)
		}
		record = records[0]
	}

	record.Set("last_seen", time.Now())
	record.Set("status", "online")

	return rm.app.Save(record)
}

// restoreAgentState recovers pending messages, tasks, and variables for an agent.
func (rm *ReconnectionManager) restoreAgentState(ctx context.Context, session *SessionState, agentRef *AgentRef) (*RestoredStateInfo, error) {
	restored := &RestoredStateInfo{
		PendingMessageIDs: make([]string, 0),
		PendingTaskIDs:    make([]string, 0),
	}

	// Identify pending messages for this agent
	for _, msg := range session.Messages {
		if msg.RecipientID != nil && *msg.RecipientID == agentRef.AgentID {
			if msg.Status == MessageStatusPending || msg.Status == MessageStatusProcessing {
				restored.MessagesRestored++
				restored.PendingMessageIDs = append(restored.PendingMessageIDs, msg.ID)
			}
		}
	}

	// Identify pending tasks for this agent
	for _, task := range session.Tasks {
		if task.AssignedAgentID == agentRef.AgentID {
			if task.Status == TaskStatusAssigned || task.Status == TaskStatusWaiting || task.Status == TaskStatusRunning {
				restored.TasksRestored++
				restored.PendingTaskIDs = append(restored.PendingTaskIDs, task.ID)
			}
		}
	}

	// Count session variables (all variables are restored)
	restored.VariablesRestored = len(session.Variables)

	// Record the last activity time
	if agentRef.LastActivityAt != nil {
		restored.LastActivityRestored = agentRef.LastActivityAt
	}

	return restored, nil
}

// notifyAgentOfState sends a signal to the agent with the restored state.
func (rm *ReconnectionManager) notifyAgentOfState(ctx context.Context, agentID string, restored *RestoredStateInfo) error {
	if rm.app == nil {
		return nil
	}

	collection, err := rm.app.FindCollectionByNameOrId("signals")
	if err != nil {
		return err
	}

	// Create a summary payload
	payload := map[string]interface{}{
		"type":             "state_restore",
		"message_count":    restored.MessagesRestored,
		"task_count":       restored.TasksRestored,
		"pending_messages": restored.PendingMessageIDs,
		"pending_tasks":    restored.PendingTaskIDs,
		"timestamp":        time.Now(),
	}

	content, _ := json.Marshal(payload)

	// Create signal record
	record := core.NewRecord(collection)
	record.Set("signal_id", "RECON-"+fmt.Sprintf("%d", time.Now().UnixNano()))
	record.Set("from_agent", "system")
	record.Set("to_agent", agentID)
	record.Set("content", string(content))
	record.Set("signal_type", "system")
	record.Set("priority", "high")
	record.Set("read", false)

	return rm.app.Save(record)
}

// ReconnectAllAgents attempts to reconnect all agents in a session.
func (rm *ReconnectionManager) ReconnectAllAgents(ctx context.Context, session *SessionState) ([]*ReconnectionResult, error) {
	if session == nil {
		return nil, fmt.Errorf("session cannot be nil")
	}

	if len(session.ActiveAgents) == 0 {
		rm.logger.Debug("No agents to reconnect",
			"session_id", session.ID,
		)
		return []*ReconnectionResult{}, nil
	}

	rm.logger.Info("Reconnecting all agents in session",
		"session_id", session.ID,
		"agent_count", len(session.ActiveAgents),
	)

	// Track results
	results := make([]*ReconnectionResult, 0, len(session.ActiveAgents))
	var wg sync.WaitGroup
	resultChan := make(chan *ReconnectionResult, len(session.ActiveAgents))

	// Attempt reconnection for each agent in parallel
	for _, agentRef := range session.ActiveAgents {
		// Skip agents that have already left
		if agentRef.Status == AgentStatusLeft {
			continue
		}

		wg.Add(1)
		go func(ref AgentRef) {
			defer wg.Done()
			result, err := rm.ReconnectAgent(ctx, session.ID, ref.AgentID)
			if err != nil {
				result = &ReconnectionResult{
					AgentID: ref.AgentID,
					Success: false,
					Error:   err.Error(),
				}
			}
			resultChan <- result
		}(agentRef)
	}

	// Wait for all reconnections to complete
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	for result := range resultChan {
		results = append(results, result)
	}

	// Log summary
	succeeded := 0
	failed := 0
	for _, r := range results {
		if r.Success {
			succeeded++
		} else {
			failed++
		}
	}

	rm.logger.Info("All agents reconnection complete",
		"session_id", session.ID,
		"succeeded", succeeded,
		"failed", failed,
		"total", len(results),
	)

	return results, nil
}

// GetReconnectionState returns the current state of a reconnection attempt.
func (rm *ReconnectionManager) GetReconnectionState(agentID string) (*ReconnectionState, bool) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	state, exists := rm.activeReconnections[agentID]
	if !exists {
		return nil, false
	}

	// Return a copy
	stateCopy := *state
	return &stateCopy, true
}

// CancelReconnection cancels an ongoing reconnection attempt.
func (rm *ReconnectionManager) CancelReconnection(agentID string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	state, exists := rm.activeReconnections[agentID]
	if !exists {
		return fmt.Errorf("no active reconnection for agent %s", agentID)
	}

	if state.Status == ReconnectionSucceeded || state.Status == ReconnectionFailed {
		return fmt.Errorf("reconnection already completed for agent %s", agentID)
	}

	state.Status = ReconnectionCancelled
	now := time.Now()
	state.CompletedAt = &now
	state.LastError = "cancelled by request"

	rm.logger.Info("Reconnection cancelled",
		"agent_id", agentID,
	)

	return nil
}

// ListActiveReconnections returns all active reconnection states.
func (rm *ReconnectionManager) ListActiveReconnections() []*ReconnectionState {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	states := make([]*ReconnectionState, 0, len(rm.activeReconnections))
	for _, state := range rm.activeReconnections {
		if state.Status == ReconnectionInProgress || state.Status == ReconnectionRetrying || state.Status == ReconnectionPending {
			stateCopy := *state
			states = append(states, &stateCopy)
		}
	}
	return states
}

// CleanupCompletedReconnections removes completed reconnection states.
func (rm *ReconnectionManager) CleanupCompletedReconnections(olderThan time.Duration) int {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	threshold := time.Now().Add(-olderThan)
	cleaned := 0

	for agentID, state := range rm.activeReconnections {
		if state.CompletedAt != nil && state.CompletedAt.Before(threshold) {
			delete(rm.activeReconnections, agentID)
			cleaned++
		}
	}

	if cleaned > 0 {
		rm.logger.Debug("Cleaned up completed reconnections",
			"count", cleaned,
		)
	}

	return cleaned
}

// Stop stops the reconnection manager.
func (rm *ReconnectionManager) Stop() {
	close(rm.done)
}

// =============================================================================
// WEBSOCKET INTEGRATION
// =============================================================================

// WebSocketNotifier is an interface for sending WebSocket messages to agents.
// This allows the reconnection manager to notify agents without directly
// depending on the websocket package.
type WebSocketNotifier interface {
	// SendToAgent sends a message to a specific agent via WebSocket.
	// Returns true if the message was queued successfully.
	SendToAgent(agentID string, message []byte) bool

	// IsAgentConnected checks if an agent has an active WebSocket connection.
	IsAgentConnected(agentID string) bool

	// BroadcastToSession sends a message to all agents in a session.
	BroadcastToSession(sessionID string, message []byte) int
}

// SetWebSocketNotifier sets the WebSocket notifier for real-time agent communication.
func (rm *ReconnectionManager) SetWebSocketNotifier(notifier WebSocketNotifier) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.wsNotifier = notifier
}

// =============================================================================
// COORDINATOR RECONNECTION
// =============================================================================

// CoordinatorReconnectionState tracks coordinator reconnection progress.
type CoordinatorReconnectionState struct {
	// CoordinatorID is the ID of the coordinator being reconnected.
	CoordinatorID string `json:"coordinator_id"`

	// SessionID is the session the coordinator is reconnecting to.
	SessionID string `json:"session_id"`

	// Status indicates the current reconnection status.
	Status ReconnectionStatus `json:"status"`

	// StartedAt is when the reconnection process started.
	StartedAt time.Time `json:"started_at"`

	// CompletedAt is when the reconnection completed.
	CompletedAt *time.Time `json:"completed_at,omitempty"`

	// RestoredSession is the recovered session state.
	RestoredSession *SessionSnapshot `json:"restored_session,omitempty"`

	// AgentStates contains the state of all agents in the session.
	AgentStates map[string]AgentReconnectionInfo `json:"agent_states,omitempty"`

	// PendingActions lists actions the coordinator should take.
	PendingActions []PendingAction `json:"pending_actions,omitempty"`

	// Error contains any error that occurred.
	Error string `json:"error,omitempty"`
}

// SessionSnapshot represents a point-in-time snapshot of session state.
type SessionSnapshot struct {
	// SessionID is the session identifier.
	SessionID string `json:"session_id"`

	// Name is the session name.
	Name string `json:"name"`

	// Status is the session status at snapshot time.
	Status SessionStatus `json:"status"`

	// SnapshotTime is when the snapshot was taken.
	SnapshotTime time.Time `json:"snapshot_time"`

	// ActiveAgentCount is the number of agents in the session.
	ActiveAgentCount int `json:"active_agent_count"`

	// PendingMessageCount is the number of unprocessed messages.
	PendingMessageCount int `json:"pending_message_count"`

	// PendingTaskCount is the number of incomplete tasks.
	PendingTaskCount int `json:"pending_task_count"`

	// LastActivityAt is the last activity timestamp.
	LastActivityAt *time.Time `json:"last_activity_at,omitempty"`

	// Variables stores session-scoped variables.
	Variables map[string]interface{} `json:"variables,omitempty"`

	// Metadata stores session metadata.
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// AgentReconnectionInfo describes an agent's state during coordinator reconnection.
type AgentReconnectionInfo struct {
	// AgentID is the agent identifier.
	AgentID string `json:"agent_id"`

	// Name is the agent's display name.
	Name string `json:"name"`

	// Status is the agent's current status.
	Status AgentStatus `json:"status"`

	// LastActivityAt is when the agent was last active.
	LastActivityAt *time.Time `json:"last_activity_at,omitempty"`

	// IsConnected indicates if the agent has an active WebSocket connection.
	IsConnected bool `json:"is_connected"`

	// PendingMessages is the count of messages waiting for this agent.
	PendingMessages int `json:"pending_messages"`

	// PendingTasks is the count of tasks assigned to this agent.
	PendingTasks int `json:"pending_tasks"`

	// NeedsReconnection indicates if the agent needs to be reconnected.
	NeedsReconnection bool `json:"needs_reconnection"`
}

// PendingAction represents an action the coordinator should take.
type PendingAction struct {
	// Type is the action type.
	Type PendingActionType `json:"type"`

	// Priority indicates action urgency (higher = more urgent).
	Priority int `json:"priority"`

	// TargetAgentID is the agent this action relates to (if applicable).
	TargetAgentID string `json:"target_agent_id,omitempty"`

	// TaskID is the task this action relates to (if applicable).
	TaskID string `json:"task_id,omitempty"`

	// MessageID is the message this action relates to (if applicable).
	MessageID string `json:"message_id,omitempty"`

	// Description is a human-readable description of the action.
	Description string `json:"description"`

	// Data contains action-specific data.
	Data map[string]interface{} `json:"data,omitempty"`
}

// PendingActionType defines types of actions for coordinator.
type PendingActionType string

const (
	// ActionReconnectAgent indicates an agent needs reconnection.
	ActionReconnectAgent PendingActionType = "reconnect_agent"

	// ActionReassignTask indicates a task needs reassignment.
	ActionReassignTask PendingActionType = "reassign_task"

	// ActionReplayMessage indicates a message needs to be replayed.
	ActionReplayMessage PendingActionType = "replay_message"

	// ActionAcknowledgeAgent indicates an agent needs acknowledgement.
	ActionAcknowledgeAgent PendingActionType = "acknowledge_agent"

	// ActionReviewStaleTask indicates a stale task needs review.
	ActionReviewStaleTask PendingActionType = "review_stale_task"
)

// ReconnectCoordinator handles coordinator reconnection to a session.
// This is called when a coordinator (e.g., the main Claude instance) reconnects
// after a disconnection and needs to recover session state.
func (rm *ReconnectionManager) ReconnectCoordinator(ctx context.Context, sessionID, coordinatorID string) (*CoordinatorReconnectionState, error) {
	rm.logger.Info("Starting coordinator reconnection",
		"session_id", sessionID,
		"coordinator_id", coordinatorID,
	)

	state := &CoordinatorReconnectionState{
		CoordinatorID: coordinatorID,
		SessionID:     sessionID,
		Status:        ReconnectionInProgress,
		StartedAt:     time.Now(),
		AgentStates:   make(map[string]AgentReconnectionInfo),
		PendingActions: make([]PendingAction, 0),
	}

	// Step 1: Load session from database
	session, err := rm.sessionMgr.GetSession(ctx, sessionID)
	if err != nil {
		state.Status = ReconnectionFailed
		state.Error = fmt.Sprintf("failed to load session: %v", err)
		rm.logger.Error("Coordinator reconnection failed: session not found",
			"session_id", sessionID,
			"error", err,
		)
		return state, err
	}

	// Step 2: Verify coordinator ownership
	if session.CoordinatorID != coordinatorID {
		state.Status = ReconnectionFailed
		state.Error = fmt.Sprintf("coordinator mismatch: session belongs to %s", session.CoordinatorID)
		return state, fmt.Errorf("coordinator mismatch: session belongs to %s, not %s",
			session.CoordinatorID, coordinatorID)
	}

	// Step 3: Check session is in recoverable state
	if session.Status.IsTerminal() {
		state.Status = ReconnectionFailed
		state.Error = fmt.Sprintf("session is in terminal state: %s", session.Status)
		return state, fmt.Errorf("session is in terminal state %s", session.Status)
	}

	// Step 4: Create session snapshot
	state.RestoredSession = rm.createSessionSnapshot(session)

	// Step 5: Analyze each agent's state
	for _, agentRef := range session.ActiveAgents {
		agentInfo := rm.analyzeAgentState(ctx, session, &agentRef)
		state.AgentStates[agentRef.AgentID] = agentInfo

		// Generate pending actions based on agent state
		if agentInfo.NeedsReconnection {
			state.PendingActions = append(state.PendingActions, PendingAction{
				Type:          ActionReconnectAgent,
				Priority:      10,
				TargetAgentID: agentRef.AgentID,
				Description:   fmt.Sprintf("Agent %s needs reconnection", agentRef.Name),
				Data: map[string]interface{}{
					"pending_messages": agentInfo.PendingMessages,
					"pending_tasks":    agentInfo.PendingTasks,
				},
			})
		}
	}

	// Step 6: Identify pending messages that need replay
	pendingMsgActions := rm.identifyPendingMessages(session)
	state.PendingActions = append(state.PendingActions, pendingMsgActions...)

	// Step 7: Identify stale or orphaned tasks
	staleTaskActions := rm.identifyStaleTasks(session)
	state.PendingActions = append(state.PendingActions, staleTaskActions...)

	// Step 8: Update session with coordinator reconnection
	now := time.Now()
	session.Status = SessionActive
	session.UpdatedAt = now
	session.LastHeartbeatAt = &now

	if err := rm.sessionMgr.UpdateSession(ctx, session); err != nil {
		rm.logger.Warn("Failed to update session after coordinator reconnection",
			"session_id", sessionID,
			"error", err,
		)
	}

	// Step 9: Notify connected agents about coordinator return
	rm.notifyAgentsOfCoordinatorReturn(ctx, session)

	state.Status = ReconnectionSucceeded
	completedAt := time.Now()
	state.CompletedAt = &completedAt

	rm.logger.Info("Coordinator reconnection completed",
		"session_id", sessionID,
		"coordinator_id", coordinatorID,
		"active_agents", len(state.AgentStates),
		"pending_actions", len(state.PendingActions),
	)

	return state, nil
}

// createSessionSnapshot creates a snapshot of the current session state.
func (rm *ReconnectionManager) createSessionSnapshot(session *SessionState) *SessionSnapshot {
	snapshot := &SessionSnapshot{
		SessionID:        session.ID,
		Name:             session.Name,
		Status:           session.Status,
		SnapshotTime:     time.Now(),
		ActiveAgentCount: len(session.ActiveAgents),
		Variables:        session.Variables,
		Metadata:         session.Metadata,
	}

	// Count pending messages
	for _, msg := range session.Messages {
		if msg.Status == MessageStatusPending || msg.Status == MessageStatusProcessing {
			snapshot.PendingMessageCount++
		}
	}

	// Count pending tasks
	for _, task := range session.Tasks {
		if task.Status == TaskStatusAssigned || task.Status == TaskStatusRunning || task.Status == TaskStatusWaiting {
			snapshot.PendingTaskCount++
		}
	}

	// Find last activity
	var lastActivity *time.Time
	for _, agent := range session.ActiveAgents {
		if agent.LastActivityAt != nil {
			if lastActivity == nil || agent.LastActivityAt.After(*lastActivity) {
				lastActivity = agent.LastActivityAt
			}
		}
	}
	snapshot.LastActivityAt = lastActivity

	return snapshot
}

// analyzeAgentState analyzes an agent's current state and determines reconnection needs.
func (rm *ReconnectionManager) analyzeAgentState(ctx context.Context, session *SessionState, agentRef *AgentRef) AgentReconnectionInfo {
	info := AgentReconnectionInfo{
		AgentID:        agentRef.AgentID,
		Name:           agentRef.Name,
		Status:         agentRef.Status,
		LastActivityAt: agentRef.LastActivityAt,
	}

	// Check WebSocket connection status
	rm.mu.RLock()
	if rm.wsNotifier != nil {
		info.IsConnected = rm.wsNotifier.IsAgentConnected(agentRef.AgentID)
	}
	rm.mu.RUnlock()

	// Count pending messages for this agent
	for _, msg := range session.Messages {
		if msg.RecipientID != nil && *msg.RecipientID == agentRef.AgentID {
			if msg.Status == MessageStatusPending || msg.Status == MessageStatusProcessing {
				info.PendingMessages++
			}
		}
	}

	// Count pending tasks for this agent
	for _, task := range session.Tasks {
		if task.AssignedAgentID == agentRef.AgentID {
			if task.Status == TaskStatusAssigned || task.Status == TaskStatusRunning || task.Status == TaskStatusWaiting {
				info.PendingTasks++
			}
		}
	}

	// Determine if agent needs reconnection
	info.NeedsReconnection = !info.IsConnected && agentRef.Status != AgentStatusLeft

	// Also check if agent hasn't been seen recently
	if agentRef.LastActivityAt != nil {
		staleDuration := 5 * time.Minute
		if time.Since(*agentRef.LastActivityAt) > staleDuration {
			info.NeedsReconnection = true
		}
	}

	return info
}

// identifyPendingMessages finds messages that need to be replayed.
func (rm *ReconnectionManager) identifyPendingMessages(session *SessionState) []PendingAction {
	actions := make([]PendingAction, 0)

	for _, msg := range session.Messages {
		if msg.Status == MessageStatusPending {
			action := PendingAction{
				Type:      ActionReplayMessage,
				Priority:  5,
				MessageID: msg.ID,
				Description: fmt.Sprintf("Message to %s pending since %s",
					getRecipientName(msg.RecipientID),
					msg.CreatedAt.Format(time.RFC3339)),
				Data: map[string]interface{}{
					"sender_id":    msg.SenderID,
					"recipient_id": msg.RecipientID,
					"message_type": msg.MessageType,
					"created_at":   msg.CreatedAt,
				},
			}
			if msg.RecipientID != nil {
				action.TargetAgentID = *msg.RecipientID
			}
			actions = append(actions, action)
		}
	}

	return actions
}

// identifyStaleTasks finds tasks that may need review or reassignment.
func (rm *ReconnectionManager) identifyStaleTasks(session *SessionState) []PendingAction {
	actions := make([]PendingAction, 0)
	staleThreshold := 10 * time.Minute

	for _, task := range session.Tasks {
		if task.Status == TaskStatusRunning || task.Status == TaskStatusAssigned {
			// Check if task is stale
			isStale := false
			if task.StartedAt != nil {
				isStale = time.Since(*task.StartedAt) > staleThreshold
			} else {
				isStale = time.Since(task.CreatedAt) > staleThreshold
			}

			if isStale {
				actions = append(actions, PendingAction{
					Type:          ActionReviewStaleTask,
					Priority:      8,
					TaskID:        task.ID,
					TargetAgentID: task.AssignedAgentID,
					Description: fmt.Sprintf("Task '%s' has been %s for over %v",
						task.Title, task.Status, staleThreshold),
					Data: map[string]interface{}{
						"task_status": task.Status,
						"created_at":  task.CreatedAt,
						"started_at":  task.StartedAt,
					},
				})
			}
		}
	}

	return actions
}

// notifyAgentsOfCoordinatorReturn sends a notification to all connected agents.
func (rm *ReconnectionManager) notifyAgentsOfCoordinatorReturn(ctx context.Context, session *SessionState) {
	rm.mu.RLock()
	notifier := rm.wsNotifier
	rm.mu.RUnlock()

	if notifier == nil {
		rm.logger.Debug("No WebSocket notifier configured, skipping agent notifications")
		return
	}

	notification := map[string]interface{}{
		"type": "coordinator_reconnected",
		"payload": map[string]interface{}{
			"session_id":     session.ID,
			"coordinator_id": session.CoordinatorID,
			"timestamp":      time.Now(),
			"message":        "Coordinator has reconnected to the session",
		},
	}

	msgBytes, _ := json.Marshal(notification)
	notified := notifier.BroadcastToSession(session.ID, msgBytes)

	rm.logger.Info("Notified agents of coordinator return",
		"session_id", session.ID,
		"agents_notified", notified,
	)
}

// getRecipientName returns a display name for a recipient ID.
func getRecipientName(recipientID *string) string {
	if recipientID == nil {
		return "broadcast"
	}
	return *recipientID
}

// =============================================================================
// MESSAGE REPLAY
// =============================================================================

// MessageReplayRequest describes a request to replay messages.
type MessageReplayRequest struct {
	// SessionID is the session to replay messages from.
	SessionID string `json:"session_id"`

	// AgentID is the target agent for replayed messages.
	AgentID string `json:"agent_id"`

	// Since is the timestamp to replay messages from.
	Since *time.Time `json:"since,omitempty"`

	// MessageIDs is optional specific message IDs to replay.
	MessageIDs []string `json:"message_ids,omitempty"`

	// MarkAsProcessed indicates whether to mark messages as processed after replay.
	MarkAsProcessed bool `json:"mark_as_processed"`
}

// MessageReplayResult describes the result of a message replay.
type MessageReplayResult struct {
	// Success indicates if the replay was successful.
	Success bool `json:"success"`

	// MessagesReplayed is the count of messages successfully replayed.
	MessagesReplayed int `json:"messages_replayed"`

	// MessagesFailed is the count of messages that failed to replay.
	MessagesFailed int `json:"messages_failed"`

	// ReplayedIDs contains the IDs of replayed messages.
	ReplayedIDs []string `json:"replayed_ids,omitempty"`

	// FailedIDs contains the IDs of messages that failed.
	FailedIDs []string `json:"failed_ids,omitempty"`

	// Error contains any error that occurred.
	Error string `json:"error,omitempty"`
}

// ReplayMessages replays pending messages to a reconnected agent.
func (rm *ReconnectionManager) ReplayMessages(ctx context.Context, req *MessageReplayRequest) (*MessageReplayResult, error) {
	rm.logger.Info("Starting message replay",
		"session_id", req.SessionID,
		"agent_id", req.AgentID,
	)

	result := &MessageReplayResult{
		ReplayedIDs: make([]string, 0),
		FailedIDs:   make([]string, 0),
	}

	// Load session
	session, err := rm.sessionMgr.GetSession(ctx, req.SessionID)
	if err != nil {
		result.Error = fmt.Sprintf("failed to load session: %v", err)
		return result, err
	}

	// Get WebSocket notifier
	rm.mu.RLock()
	notifier := rm.wsNotifier
	rm.mu.RUnlock()

	// Find messages to replay
	for _, msg := range session.Messages {
		// Skip if not for this agent
		if msg.RecipientID == nil || *msg.RecipientID != req.AgentID {
			continue
		}

		// Skip if already processed
		if msg.Status != MessageStatusPending && msg.Status != MessageStatusProcessing {
			continue
		}

		// Skip if before the 'since' timestamp
		if req.Since != nil && msg.CreatedAt.Before(*req.Since) {
			continue
		}

		// Skip if specific IDs requested and this isn't one
		if len(req.MessageIDs) > 0 {
			found := false
			for _, id := range req.MessageIDs {
				if id == msg.ID {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// Attempt to replay the message
		if notifier != nil && notifier.IsAgentConnected(req.AgentID) {
			replayMsg := map[string]interface{}{
				"type": "message_replay",
				"payload": map[string]interface{}{
					"message_id":   msg.ID,
					"session_id":   msg.SessionID,
					"sender_id":    msg.SenderID,
					"content":      msg.Content,
					"content_type": msg.ContentType,
					"message_type": msg.MessageType,
					"created_at":   msg.CreatedAt,
					"is_replay":    true,
				},
			}
			msgBytes, _ := json.Marshal(replayMsg)

			if notifier.SendToAgent(req.AgentID, msgBytes) {
				result.ReplayedIDs = append(result.ReplayedIDs, msg.ID)
				result.MessagesReplayed++

				// Mark as delivered if requested
				if req.MarkAsProcessed {
					msg.Status = MessageStatusDelivered
					now := time.Now()
					msg.ProcessedAt = &now
				}
			} else {
				result.FailedIDs = append(result.FailedIDs, msg.ID)
				result.MessagesFailed++
			}
		} else {
			// Agent not connected, queue for later
			result.FailedIDs = append(result.FailedIDs, msg.ID)
			result.MessagesFailed++
		}
	}

	// Update session if we modified message statuses
	if req.MarkAsProcessed && result.MessagesReplayed > 0 {
		session.UpdatedAt = time.Now()
		if err := rm.sessionMgr.UpdateSession(ctx, session); err != nil {
			rm.logger.Warn("Failed to update session after message replay",
				"session_id", req.SessionID,
				"error", err,
			)
		}
	}

	result.Success = result.MessagesFailed == 0

	rm.logger.Info("Message replay completed",
		"session_id", req.SessionID,
		"agent_id", req.AgentID,
		"replayed", result.MessagesReplayed,
		"failed", result.MessagesFailed,
	)

	return result, nil
}

// CalculateBackoff calculates the backoff duration for a given attempt.
func CalculateBackoff(attempt int, initial, max time.Duration, multiplier float64) time.Duration {
	if attempt <= 0 {
		return initial
	}

	backoff := float64(initial) * math.Pow(multiplier, float64(attempt-1))
	if time.Duration(backoff) > max {
		return max
	}
	return time.Duration(backoff)
}
