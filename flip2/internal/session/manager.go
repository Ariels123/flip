// Package session provides session management functionality for FLIP2.
// Sessions represent bounded execution contexts where agents collaborate to complete tasks.
package session

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/pocketbase/pocketbase/core"
)

// SessionManager handles session lifecycle including creation, activation, and termination.
type SessionManager struct {
	mu     sync.RWMutex
	app    core.App
	logger *slog.Logger

	// In-memory cache of active sessions
	activeSessions map[string]*SessionState
}

// NewSessionManager creates a new session manager.
func NewSessionManager(app core.App, logger *slog.Logger) *SessionManager {
	if logger == nil {
		logger = slog.Default()
	}

	return &SessionManager{
		app:            app,
		logger:         logger.WithGroup("session"),
		activeSessions: make(map[string]*SessionState),
	}
}

// StartSession creates a new session and marks it as active.
// Returns the created session object.
func (m *SessionManager) StartSession(ctx context.Context, name string, coordinatorID string) (*SessionState, error) {
	if name == "" {
		return nil, fmt.Errorf("session name is required")
	}
	if coordinatorID == "" {
		return nil, fmt.Errorf("coordinator ID is required")
	}

	// Create session object
	now := time.Now()
	session := &SessionState{
		ID:            uuid.New().String(),
		Name:          name,
		CoordinatorID: coordinatorID,
		Status:        SessionActive,
		Messages:      make([]Message, 0),
		ActiveAgents:  make([]AgentRef, 0),
		Tasks:         make([]TaskRef, 0),
		Environment:   make(map[string]string),
		Variables:     make(map[string]interface{}),
		CreatedAt:     now,
		StartedAt:     &now,
		UpdatedAt:     now,
		MessageCount:  0,
		AgentCount:    0,
		TaskCount:     0,
		ErrorCount:    0,
		Metadata:      make(map[string]interface{}),
	}

	// Validate session
	if err := session.Validate(); err != nil {
		return nil, fmt.Errorf("session validation failed: %w", err)
	}

	// Store in database via PocketBase
	collection, err := m.app.FindCollectionByNameOrId("sessions")
	if err != nil {
		return nil, fmt.Errorf("sessions collection not found: %w", err)
	}

	// Prepare record data
	recordData := map[string]interface{}{
		"id":             session.ID,
		"name":           session.Name,
		"status":         session.Status,
		"coordinator_id": session.CoordinatorID,
		"created_at":     session.CreatedAt,
		"started_at":     session.StartedAt,
		"updated_at":     session.UpdatedAt,
		"message_count":  session.MessageCount,
		"agent_count":    session.AgentCount,
		"task_count":     session.TaskCount,
		"error_count":    session.ErrorCount,
	}

	// Handle metadata as JSON string
	if session.Metadata != nil {
		metadataBytes, _ := json.Marshal(session.Metadata)
		recordData["metadata"] = string(metadataBytes)
	}

	record := core.NewRecord(collection)
	record.Load(recordData)

	// Save record
	if err := m.app.Save(record); err != nil {
		return nil, fmt.Errorf("failed to save session to database: %w", err)
	}

	// Store in memory cache
	m.mu.Lock()
	m.activeSessions[session.ID] = session
	m.mu.Unlock()

	m.logger.Info("Session started",
		"session_id", session.ID,
		"name", session.Name,
		"coordinator_id", coordinatorID,
	)

	return session, nil
}

// StopSession terminates a session and updates its status.
// Updates status to a terminal state and saves final state.
func (m *SessionManager) StopSession(ctx context.Context, sessionID string, finalStatus SessionStatus) error {
	if sessionID == "" {
		return fmt.Errorf("session ID is required")
	}

	if !finalStatus.IsTerminal() {
		return fmt.Errorf("final status must be terminal: %s", finalStatus)
	}

	// Get session from memory cache first
	m.mu.RLock()
	session, exists := m.activeSessions[sessionID]
	m.mu.RUnlock()

	if !exists {
		// Try to load from database
		var err error
		session, err = m.GetSession(ctx, sessionID)
		if err != nil {
			return fmt.Errorf("session not found: %w", err)
		}
	}

	// Update session status
	session.Status = finalStatus
	now := time.Now()
	session.CompletedAt = &now
	session.UpdatedAt = now

	// Find and update the record in PocketBase
	collection, err := m.app.FindCollectionByNameOrId("sessions")
	if err != nil {
		return fmt.Errorf("sessions collection not found: %w", err)
	}

	record, err := m.app.FindRecordById(collection, sessionID)
	if err != nil {
		return fmt.Errorf("failed to find session record: %w", err)
	}

	// Update record fields
	record.Set("status", session.Status)
	record.Set("completed_at", session.CompletedAt)
	record.Set("updated_at", session.UpdatedAt)

	// Save updated record
	if err := m.app.Save(record); err != nil {
		return fmt.Errorf("failed to save session update: %w", err)
	}

	// Remove from memory cache
	m.mu.Lock()
	delete(m.activeSessions, sessionID)
	m.mu.Unlock()

	m.logger.Info("Session stopped",
		"session_id", sessionID,
		"final_status", finalStatus,
	)

	return nil
}

// GetActiveSession returns the currently active session for the coordinator.
// If multiple sessions are active, returns the most recently created one.
func (m *SessionManager) GetActiveSession(ctx context.Context, coordinatorID string) (*SessionState, error) {
	if coordinatorID == "" {
		return nil, fmt.Errorf("coordinator ID is required")
	}

	// Check memory cache first
	m.mu.RLock()
	for _, session := range m.activeSessions {
		if session.CoordinatorID == coordinatorID && session.Status == SessionActive {
			m.mu.RUnlock()
			return session, nil
		}
	}
	m.mu.RUnlock()

	// Query database for active sessions
	collection, err := m.app.FindCollectionByNameOrId("sessions")
	if err != nil {
		return nil, fmt.Errorf("sessions collection not found: %w", err)
	}

	// Find active sessions for this coordinator
	filter := fmt.Sprintf("coordinator_id = '%s' && status = '%s'", coordinatorID, SessionActive)
	records, err := m.app.FindRecordsByFilter(collection.Id, filter, "", 100, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to query sessions: %w", err)
	}

	if len(records) == 0 {
		return nil, fmt.Errorf("no active session found for coordinator %s", coordinatorID)
	}

	// Return the first (most recent) active session
	// In a real implementation, you might want to sort by created_at descending
	record := records[0]
	session, err := m.recordToSession(ctx, record)
	if err != nil {
		return nil, fmt.Errorf("failed to convert record to session: %w", err)
	}

	return session, nil
}

// GetSession retrieves a session by ID.
func (m *SessionManager) GetSession(ctx context.Context, sessionID string) (*SessionState, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("session ID is required")
	}

	// Check memory cache first
	m.mu.RLock()
	if session, exists := m.activeSessions[sessionID]; exists {
		m.mu.RUnlock()
		return session, nil
	}
	m.mu.RUnlock()

	// Query database
	collection, err := m.app.FindCollectionByNameOrId("sessions")
	if err != nil {
		return nil, fmt.Errorf("sessions collection not found: %w", err)
	}

	record, err := m.app.FindRecordById(collection, sessionID)
	if err != nil {
		return nil, fmt.Errorf("session not found: %w", err)
	}

	session, err := m.recordToSession(ctx, record)
	if err != nil {
		return nil, fmt.Errorf("failed to convert record to session: %w", err)
	}

	return session, nil
}

// ListSessions returns all sessions, optionally filtered by status or coordinator.
func (m *SessionManager) ListSessions(ctx context.Context, coordinatorID string, status SessionStatus) ([]*SessionState, error) {
	collection, err := m.app.FindCollectionByNameOrId("sessions")
	if err != nil {
		return nil, fmt.Errorf("sessions collection not found: %w", err)
	}

	// Build filter
	filter := ""
	if coordinatorID != "" {
		filter = fmt.Sprintf("coordinator_id = '%s'", coordinatorID)
	}
	if status != "" {
		if filter != "" {
			filter += " && "
		}
		filter += fmt.Sprintf("status = '%s'", status)
	}

	var records []*core.Record
	records, err = m.app.FindRecordsByFilter(collection.Id, filter, "", 100, 0)

	if err != nil {
		return nil, fmt.Errorf("failed to query sessions: %w", err)
	}

	sessions := make([]*SessionState, 0, len(records))
	for _, record := range records {
		session, err := m.recordToSession(ctx, record)
		if err != nil {
			m.logger.Error("failed to convert record to session", "error", err)
			continue
		}
		sessions = append(sessions, session)
	}

	return sessions, nil
}

// UpdateSession updates the session in the database.
func (m *SessionManager) UpdateSession(ctx context.Context, session *SessionState) error {
	if err := session.Validate(); err != nil {
		return fmt.Errorf("session validation failed: %w", err)
	}

	collection, err := m.app.FindCollectionByNameOrId("sessions")
	if err != nil {
		return fmt.Errorf("sessions collection not found: %w", err)
	}

	record, err := m.app.FindRecordById(collection, session.ID)
	if err != nil {
		return fmt.Errorf("failed to find session record: %w", err)
	}

	// Update record fields
	record.Set("name", session.Name)
	record.Set("status", session.Status)
	record.Set("description", session.Description)
	record.Set("coordinator_id", session.CoordinatorID)
	record.Set("message_count", session.MessageCount)
	record.Set("agent_count", session.AgentCount)
	record.Set("task_count", session.TaskCount)
	record.Set("error_count", session.ErrorCount)
	record.Set("updated_at", session.UpdatedAt)

	if session.StartedAt != nil {
		record.Set("started_at", session.StartedAt)
	}
	if session.CompletedAt != nil {
		record.Set("completed_at", session.CompletedAt)
	}
	if session.LastHeartbeatAt != nil {
		record.Set("last_heartbeat_at", session.LastHeartbeatAt)
	}

	if session.Metadata != nil {
		metadataBytes, _ := json.Marshal(session.Metadata)
		record.Set("metadata", string(metadataBytes))
	}

	if err := m.app.Save(record); err != nil {
		return fmt.Errorf("failed to save session: %w", err)
	}

	// Update memory cache
	m.mu.Lock()
	m.activeSessions[session.ID] = session
	m.mu.Unlock()

	return nil
}

// recordToSession converts a PocketBase record to a SessionState.
func (m *SessionManager) recordToSession(ctx context.Context, record *core.Record) (*SessionState, error) {
	session := &SessionState{
		ID:            record.Id,
		Name:          record.GetString("name"),
		Status:        SessionStatus(record.GetString("status")),
		CoordinatorID: record.GetString("coordinator_id"),
		Description:   record.GetString("description"),
		CreatedAt:     record.GetDateTime("created").Time(),
		UpdatedAt:     record.GetDateTime("updated").Time(),
		MessageCount:  record.GetInt("message_count"),
		AgentCount:    record.GetInt("agent_count"),
		TaskCount:     record.GetInt("task_count"),
		ErrorCount:    record.GetInt("error_count"),
		Messages:      make([]Message, 0),
		ActiveAgents:  make([]AgentRef, 0),
		Tasks:         make([]TaskRef, 0),
		Environment:   make(map[string]string),
		Variables:     make(map[string]interface{}),
		Metadata:      make(map[string]interface{}),
	}

	// Parse timestamps
	if startedAtStr := record.GetString("started_at"); startedAtStr != "" {
		if t, err := time.Parse(time.RFC3339, startedAtStr); err == nil {
			session.StartedAt = &t
		}
	}

	if completedAtStr := record.GetString("completed_at"); completedAtStr != "" {
		if t, err := time.Parse(time.RFC3339, completedAtStr); err == nil {
			session.CompletedAt = &t
		}
	}

	if lastHeartbeatStr := record.GetString("last_heartbeat_at"); lastHeartbeatStr != "" {
		if t, err := time.Parse(time.RFC3339, lastHeartbeatStr); err == nil {
			session.LastHeartbeatAt = &t
		}
	}

	// Parse parent session ID if present
	if parentID := record.GetString("parent_session_id"); parentID != "" {
		session.ParentSessionID = &parentID
	}

	// Parse metadata
	if metadataStr := record.GetString("metadata"); metadataStr != "" {
		if err := json.Unmarshal([]byte(metadataStr), &session.Metadata); err != nil {
			m.logger.Error("failed to parse metadata", "error", err, "session_id", session.ID)
		}
	}

	return session, nil
}

// ReconnectAgents reestablishes connections with agents in a session.
// For each active agent in the session, it:
// - Checks if the agent is still running
// - Reestablishes the connection
// - Resumes monitoring
// - Marks agents as "disconnected" if they're unavailable
func (m *SessionManager) ReconnectAgents(ctx context.Context, session *SessionState) error {
	if session == nil {
		return fmt.Errorf("session cannot be nil")
	}

	if len(session.ActiveAgents) == 0 {
		m.logger.Debug("No active agents to reconnect",
			"session_id", session.ID,
		)
		return nil
	}

	m.logger.Info("Attempting to reconnect agents",
		"session_id", session.ID,
		"agent_count", len(session.ActiveAgents),
	)

	// Track reconnection results
	reconnectedCount := 0
	disconnectedCount := 0

	// Attempt to reconnect each agent
	for i, agentRef := range session.ActiveAgents {
		// Skip agents that have already left
		if agentRef.Status == AgentStatusLeft {
			m.logger.Debug("Skipping agent that already left",
				"agent_id", agentRef.AgentID,
				"session_id", session.ID,
			)
			continue
		}

		// Check if agent is still responsive
		// In a real implementation, this would involve pinging the agent
		// via a heartbeat mechanism or gRPC health check
		if err := m.checkAgentHealth(ctx, &agentRef); err != nil {
			m.logger.Warn("Agent health check failed, marking as disconnected",
				"agent_id", agentRef.AgentID,
				"session_id", session.ID,
				"error", err,
			)
			session.ActiveAgents[i].Status = AgentStatusDisconnected
			disconnectedCount++
			continue
		}

		// Agent is still responsive, reestablish monitoring
		if err := m.resumeAgentMonitoring(ctx, &agentRef); err != nil {
			m.logger.Warn("Failed to resume agent monitoring",
				"agent_id", agentRef.AgentID,
				"session_id", session.ID,
				"error", err,
			)
			session.ActiveAgents[i].Status = AgentStatusError
			disconnectedCount++
			continue
		}

		// Update agent status to active and update last activity timestamp
		now := time.Now()
		session.ActiveAgents[i].Status = AgentStatusActive
		session.ActiveAgents[i].LastActivityAt = &now

		m.logger.Info("Successfully reconnected agent",
			"agent_id", agentRef.AgentID,
			"session_id", session.ID,
		)
		reconnectedCount++
	}

	// Log summary
	m.logger.Info("Agent reconnection complete",
		"session_id", session.ID,
		"reconnected", reconnectedCount,
		"disconnected", disconnectedCount,
		"total", len(session.ActiveAgents),
	)

	return nil
}

// checkAgentHealth verifies if an agent is still running and responsive.
// This is a placeholder implementation that can be enhanced with actual health checks.
func (m *SessionManager) checkAgentHealth(ctx context.Context, agent *AgentRef) error {
	// In a production implementation, this would:
	// 1. Send a heartbeat/ping request to the agent
	// 2. Check the agent registry for status
	// 3. Verify the agent process is still running
	// For now, we assume all agents that existed before reconnection are potentially available

	// Example: Check with agent manager if available
	// This would require injecting an agent manager into SessionManager
	if agent.AgentID == "" {
		return fmt.Errorf("agent ID is empty")
	}

	// Simulate a health check by verifying agent ID is valid
	if len(agent.AgentID) < 5 {
		return fmt.Errorf("invalid agent ID format")
	}

	return nil
}

// resumeAgentMonitoring reestablishes monitoring for a reconnected agent.
// This involves:
// - Setting up message delivery channels
// - Resuming task assignment
// - Restarting heartbeat tracking
func (m *SessionManager) resumeAgentMonitoring(ctx context.Context, agent *AgentRef) error {
	if agent == nil {
		return fmt.Errorf("agent cannot be nil")
	}

	// In a production implementation, this would:
	// 1. Reconnect to agent's communication channel
	// 2. Resume queued message delivery
	// 3. Restart heartbeat monitoring
	// 4. Notify agent of reconnection

	m.logger.Debug("Resuming monitoring for agent",
		"agent_id", agent.AgentID,
		"session_id", agent.SessionID,
	)

	return nil
}

// AttachSession reattaches a coordinator to a previously running session.
// This is called when a coordinator reconnects after a disconnection.
// It:
// - Loads the session from persistent storage
// - Validates the session state
// - Reconnects agents
// - Resumes message processing
// Returns the loaded session state
func (m *SessionManager) AttachSession(ctx context.Context, sessionID string, coordinatorID string) (*SessionState, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("session ID is required")
	}
	if coordinatorID == "" {
		return nil, fmt.Errorf("coordinator ID is required")
	}

	// Load the session from database
	m.mu.RLock()
	session, exists := m.activeSessions[sessionID]
	m.mu.RUnlock()

	var err error
	if !exists {
		// Try to load from database if not in memory
		session, err = m.GetSession(ctx, sessionID)
		if err != nil {
			return nil, fmt.Errorf("failed to load session: %w", err)
		}
	}

	// Verify coordinator ownership
	if session.CoordinatorID != coordinatorID {
		return nil, fmt.Errorf("coordinator mismatch: session belongs to %s, not %s",
			session.CoordinatorID, coordinatorID)
	}

	// Validate session is in a recoverable state
	if !session.Status.IsRecoverable() {
		return nil, fmt.Errorf("session is in terminal state %s and cannot be attached",
			session.Status)
	}

	m.logger.Info("Attaching to session",
		"session_id", sessionID,
		"coordinator_id", coordinatorID,
		"current_status", session.Status,
		"agent_count", len(session.ActiveAgents),
	)

	// Reconnect agents after loading the session state
	if err := m.ReconnectAgents(ctx, session); err != nil {
		m.logger.Warn("Failed to fully reconnect agents during session attach",
			"session_id", sessionID,
			"error", err,
		)
		// Note: We don't return error here as partial reconnection is acceptable
		// The session attach succeeds even if some agents fail to reconnect
	}

	// Update session status and timestamps
	session.Status = SessionActive
	session.UpdatedAt = time.Now()
	session.LastHeartbeatAt = &session.UpdatedAt

	// Persist the updated session
	if err := m.UpdateSession(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to update session after attach: %w", err)
	}

	// Add to memory cache for fast access
	m.mu.Lock()
	m.activeSessions[sessionID] = session
	m.mu.Unlock()

	m.logger.Info("Session attached successfully",
		"session_id", sessionID,
		"coordinator_id", coordinatorID,
	)

	return session, nil
}

// DeleteSession deletes a session from the database.
func (m *SessionManager) DeleteSession(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		return fmt.Errorf("session ID is required")
	}

	collection, err := m.app.FindCollectionByNameOrId("sessions")
	if err != nil {
		return fmt.Errorf("sessions collection not found: %w", err)
	}

	record, err := m.app.FindRecordById(collection, sessionID)
	if err != nil {
		return fmt.Errorf("failed to find session record: %w", err)
	}

	if err := m.app.Delete(record); err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	// Remove from memory cache
	m.mu.Lock()
	delete(m.activeSessions, sessionID)
	m.mu.Unlock()

	m.logger.Info("Session deleted",
		"session_id", sessionID,
	)

	return nil
}
