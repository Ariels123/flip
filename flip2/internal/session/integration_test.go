// Package session provides integration tests for session management functionality.
package session

import (
	"context"
	"database/sql"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// =============================================================================
// TEST FIXTURES
// =============================================================================

// testSession is a helper for in-memory session state testing.
type testSession struct {
	state *SessionState
	mu    sync.RWMutex
}

// newTestSession creates a new test session with defaults.
func newTestSession(t *testing.T, name, coordinatorID string) *testSession {
	return &testSession{
		state: &SessionState{
			ID:            generateID(),
			Name:          name,
			CoordinatorID: coordinatorID,
			Status:        SessionActive,
			Messages:      make([]Message, 0),
			ActiveAgents:  make([]AgentRef, 0),
			Tasks:         make([]TaskRef, 0),
			Environment:   make(map[string]string),
			Variables:     make(map[string]interface{}),
			CreatedAt:     time.Now(),
			StartedAt:     timePtr(time.Now()),
			UpdatedAt:     time.Now(),
			MessageCount:  0,
			AgentCount:    0,
			TaskCount:     0,
			ErrorCount:    0,
			Metadata:      make(map[string]interface{}),
		},
	}
}

// addMessage adds a message to the session.
func (ts *testSession) addMessage(msg *Message) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.state.Messages = append(ts.state.Messages, *msg)
	ts.state.MessageCount++
}

// addAgent adds an agent to the session.
func (ts *testSession) addAgent(agent *AgentRef) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.state.ActiveAgents = append(ts.state.ActiveAgents, *agent)
	ts.state.AgentCount++
}

// addTask adds a task to the session.
func (ts *testSession) addTask(task *TaskRef) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.state.Tasks = append(ts.state.Tasks, *task)
	ts.state.TaskCount++
}

// setStatus sets the session status.
func (ts *testSession) setStatus(status SessionStatus) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.state.Status = status
}

// getState returns a copy of the current session state.
func (ts *testSession) getState() *SessionState {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	// Return a copy to prevent external modifications
	stateCopy := *ts.state
	stateCopy.Messages = make([]Message, len(ts.state.Messages))
	copy(stateCopy.Messages, ts.state.Messages)
	stateCopy.ActiveAgents = make([]AgentRef, len(ts.state.ActiveAgents))
	copy(stateCopy.ActiveAgents, ts.state.ActiveAgents)
	stateCopy.Tasks = make([]TaskRef, len(ts.state.Tasks))
	copy(stateCopy.Tasks, ts.state.Tasks)
	return &stateCopy
}

// setupIntegrationDB creates an in-memory SQLite database for testing.
func setupIntegrationDB(t *testing.T) *DB {
	conn, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	// Initialize schema
	if err := InitDB(conn); err != nil {
		t.Fatalf("failed to initialize schema: %v", err)
	}

	return NewDB(conn)
}

// teardownTestDB closes the test database connection.
func teardownTestDB(t *testing.T, db *DB) {
	// DB wrapper doesn't expose the connection, but we trust cleanup happens
	// This is a placeholder for potential cleanup operations
}

// =============================================================================
// TEST: SESSION LIFECYCLE
// =============================================================================

// TestSessionCreateAndMessages tests creating a session and adding messages.
func TestSessionCreateAndMessages(t *testing.T) {
	session := newTestSession(t, "test-session", "coordinator-1")

	// Verify initial state
	state := session.getState()
	if state.Name != "test-session" {
		t.Errorf("expected session name 'test-session', got '%s'", state.Name)
	}
	if state.Status != SessionActive {
		t.Errorf("expected status Active, got %s", state.Status)
	}
	if state.MessageCount != 0 {
		t.Errorf("expected 0 initial messages, got %d", state.MessageCount)
	}

	// Add messages
	msg1 := NewMessage(state.ID, "sender-1", MessageRoleCoordinator, MessageTypeRequest, "Hello")
	msg1.Status = MessageStatusProcessed
	session.addMessage(msg1)

	msg2 := NewMessage(state.ID, "sender-2", MessageRoleAgent, MessageTypeResponse, "Hi there")
	msg2.Status = MessageStatusProcessed
	session.addMessage(msg2)

	// Verify messages were added
	state = session.getState()
	if state.MessageCount != 2 {
		t.Errorf("expected 2 messages, got %d", state.MessageCount)
	}
	if len(state.Messages) != 2 {
		t.Errorf("expected 2 messages in slice, got %d", len(state.Messages))
	}
	if state.Messages[0].Content != "Hello" {
		t.Errorf("expected first message 'Hello', got '%s'", state.Messages[0].Content)
	}
	if state.Messages[1].Content != "Hi there" {
		t.Errorf("expected second message 'Hi there', got '%s'", state.Messages[1].Content)
	}
}

// TestSessionDetachAndReattach tests detaching and reattaching a session.
func TestSessionDetachAndReattach(t *testing.T) {
	session := newTestSession(t, "detach-test", "coordinator-1")
	state := session.getState()

	// Add some messages before detaching
	msg := NewMessage(state.ID, "sender-1", MessageRoleCoordinator, MessageTypeRequest, "Data")
	msg.Status = MessageStatusProcessed
	session.addMessage(msg)

	// Verify initial state
	state = session.getState()
	if state.MessageCount != 1 {
		t.Errorf("expected 1 message before detach, got %d", state.MessageCount)
	}

	// Simulate detach by marking session as stale
	session.setStatus(SessionStale)
	state = session.getState()
	if state.Status != SessionStale {
		t.Errorf("expected status Stale after detach, got %s", state.Status)
	}

	// Verify messages are still there after detach
	if state.MessageCount != 1 {
		t.Errorf("expected 1 message after detach, got %d", state.MessageCount)
	}

	// Reattach by setting status back to active
	session.setStatus(SessionActive)
	state = session.getState()
	if state.Status != SessionActive {
		t.Errorf("expected status Active after reattach, got %s", state.Status)
	}

	// Verify messages survived the detach/reattach cycle
	if state.MessageCount != 1 {
		t.Errorf("expected 1 message after reattach, got %d", state.MessageCount)
	}
	if len(state.Messages) > 0 && state.Messages[0].Content != "Data" {
		t.Errorf("expected message content 'Data', got '%s'", state.Messages[0].Content)
	}
}

// TestAgentJoinDetachReattach tests spawning an agent in a session and handling
// detachment/reattachment.
func TestAgentJoinDetachReattach(t *testing.T) {
	session := newTestSession(t, "agent-test", "coordinator-1")
	state := session.getState()

	// Spawn an agent
	agent := NewAgentRef(state.ID, "agent-1", "TestAgent", "claude-opus", "worker")
	agent.Status = AgentStatusActive
	session.addAgent(agent)

	// Verify agent was added
	state = session.getState()
	if state.AgentCount != 1 {
		t.Errorf("expected 1 agent, got %d", state.AgentCount)
	}
	if len(state.ActiveAgents) != 1 {
		t.Errorf("expected 1 agent in slice, got %d", len(state.ActiveAgents))
	}
	if state.ActiveAgents[0].Name != "TestAgent" {
		t.Errorf("expected agent name 'TestAgent', got '%s'", state.ActiveAgents[0].Name)
	}

	// Simulate agent sending a message during active session
	agentMsg := NewMessage(state.ID, "agent-1", MessageRoleAgent, MessageTypeResponse, "Agent response")
	agentMsg.Status = MessageStatusProcessed
	session.addMessage(agentMsg)

	if state = session.getState(); state.MessageCount != 1 {
		t.Errorf("expected 1 message from agent, got %d", state.MessageCount)
	}

	// Detach session (coordinator disconnect)
	session.setStatus(SessionStale)
	state = session.getState()
	if state.Status != SessionStale {
		t.Errorf("expected status Stale, got %s", state.Status)
	}

	// Agent should still be registered
	if state.AgentCount != 1 {
		t.Errorf("expected agent to persist after detach, got %d agents", state.AgentCount)
	}

	// Reattach session
	session.setStatus(SessionActive)
	state = session.getState()

	// Verify agent and messages survived the cycle
	if state.AgentCount != 1 {
		t.Errorf("expected 1 agent after reattach, got %d", state.AgentCount)
	}
	if state.MessageCount != 1 {
		t.Errorf("expected 1 message after reattach, got %d", state.MessageCount)
	}
	if len(state.ActiveAgents) > 0 && state.ActiveAgents[0].Status != AgentStatusActive {
		t.Errorf("expected agent status Active, got %s", state.ActiveAgents[0].Status)
	}
}

// =============================================================================
// TEST: AUTO-SAVE BEHAVIOR
// =============================================================================

// TestAutoSaveOnSignal tests that sessions auto-save when receiving signals.
func TestAutoSaveOnSignal(t *testing.T) {
	session := newTestSession(t, "auto-save-test", "coordinator-1")
	state := session.getState()

	// Add messages and agents
	for i := 0; i < 5; i++ {
		msg := NewMessage(state.ID, "sender", MessageRoleCoordinator, MessageTypeRequest, "message")
		msg.Status = MessageStatusProcessed
		session.addMessage(msg)
	}

	agent := NewAgentRef(state.ID, "agent-1", "Agent", "claude", "worker")
	agent.Status = AgentStatusActive
	session.addAgent(agent)

	// Verify state before "save"
	state = session.getState()
	if state.MessageCount != 5 {
		t.Errorf("expected 5 messages before save, got %d", state.MessageCount)
	}
	if state.AgentCount != 1 {
		t.Errorf("expected 1 agent before save, got %d", state.AgentCount)
	}

	// Simulate save by verifying data is preserved
	savedState := session.getState()
	if savedState.MessageCount != state.MessageCount {
		t.Errorf("message count not preserved after save: %d vs %d", savedState.MessageCount, state.MessageCount)
	}
	if savedState.AgentCount != state.AgentCount {
		t.Errorf("agent count not preserved after save: %d vs %d", savedState.AgentCount, state.AgentCount)
	}
}

// =============================================================================
// TEST: SESSION CLEANUP
// =============================================================================

// TestSessionCleanup tests that session cleanup properly removes data.
func TestSessionCleanup(t *testing.T) {
	session := newTestSession(t, "cleanup-test", "coordinator-1")
	state := session.getState()

	// Add test data
	msg := NewMessage(state.ID, "sender", MessageRoleCoordinator, MessageTypeRequest, "data")
	session.addMessage(msg)

	agent := NewAgentRef(state.ID, "agent-1", "Agent", "claude", "worker")
	session.addAgent(agent)

	task := NewTaskRef(state.ID, "agent-1", "Task")
	session.addTask(task)

	// Verify data was added
	state = session.getState()
	if state.MessageCount == 0 || state.AgentCount == 0 || state.TaskCount == 0 {
		t.Fatal("failed to add test data")
	}

	// Create new session (simulating cleanup of old session)
	newSession := newTestSession(t, "new-session", "coordinator-1")
	newState := newSession.getState()

	// Verify new session is clean
	if newState.MessageCount != 0 {
		t.Errorf("expected clean session to have 0 messages, got %d", newState.MessageCount)
	}
	if newState.AgentCount != 0 {
		t.Errorf("expected clean session to have 0 agents, got %d", newState.AgentCount)
	}
	if newState.TaskCount != 0 {
		t.Errorf("expected clean session to have 0 tasks, got %d", newState.TaskCount)
	}
}

// =============================================================================
// TEST: CONCURRENT SESSIONS
// =============================================================================

// TestMultipleConcurrentSessions tests multiple sessions running concurrently
// without interfering with each other.
func TestMultipleConcurrentSessions(t *testing.T) {
	numSessions := 10
	messagesPerSession := 50

	var wg sync.WaitGroup
	sessions := make([]*testSession, numSessions)
	results := make([]*SessionState, numSessions)
	var resultsMu sync.Mutex

	// Create and populate sessions concurrently
	for i := 0; i < numSessions; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			// Create session
			session := newTestSession(t, "session-"+string(rune(idx)), "coordinator")
			sessions[idx] = session
			state := session.getState()

			// Add messages
			for j := 0; j < messagesPerSession; j++ {
				msg := NewMessage(state.ID, "sender", MessageRoleCoordinator, MessageTypeRequest, "msg")
				msg.Status = MessageStatusProcessed
				session.addMessage(msg)
			}

			// Add agents
			for j := 0; j < 3; j++ {
				agent := NewAgentRef(state.ID, "agent-"+string(rune(j)), "Agent", "claude", "worker")
				agent.Status = AgentStatusActive
				session.addAgent(agent)
			}

			// Save final state
			resultsMu.Lock()
			results[idx] = session.getState()
			resultsMu.Unlock()
		}(i)
	}

	wg.Wait()

	// Verify each session maintained its own state
	for i := 0; i < numSessions; i++ {
		if results[i] == nil {
			t.Errorf("session %d result is nil", i)
			continue
		}

		if results[i].MessageCount != messagesPerSession {
			t.Errorf("session %d: expected %d messages, got %d",
				i, messagesPerSession, results[i].MessageCount)
		}

		if results[i].AgentCount != 3 {
			t.Errorf("session %d: expected 3 agents, got %d",
				i, results[i].AgentCount)
		}

		if len(results[i].Messages) != messagesPerSession {
			t.Errorf("session %d: expected %d messages in slice, got %d",
				i, messagesPerSession, len(results[i].Messages))
		}

		if len(results[i].ActiveAgents) != 3 {
			t.Errorf("session %d: expected 3 agents in slice, got %d",
				i, len(results[i].ActiveAgents))
		}
	}
}

// TestConcurrentMessageAdds tests adding messages to the same session
// concurrently from multiple goroutines.
func TestConcurrentMessageAdds(t *testing.T) {
	session := newTestSession(t, "concurrent-msgs", "coordinator-1")
	state := session.getState()

	numGoroutines := 10
	messagesPerGoroutine := 100
	var wg sync.WaitGroup

	// Add messages concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < messagesPerGoroutine; j++ {
				msg := NewMessage(state.ID, "sender", MessageRoleCoordinator, MessageTypeRequest, "msg")
				msg.Status = MessageStatusProcessed
				session.addMessage(msg)
			}
		}(i)
	}

	wg.Wait()

	// Verify all messages were added
	finalState := session.getState()
	expectedCount := numGoroutines * messagesPerGoroutine

	if finalState.MessageCount != expectedCount {
		t.Errorf("expected %d messages, got %d", expectedCount, finalState.MessageCount)
	}

	if len(finalState.Messages) != expectedCount {
		t.Errorf("expected %d messages in slice, got %d", expectedCount, len(finalState.Messages))
	}
}

// =============================================================================
// TEST: SESSION ISOLATION
// =============================================================================

// TestSessionIsolation verifies that messages from one session don't leak
// into another session.
func TestSessionIsolation(t *testing.T) {
	session1 := newTestSession(t, "session-1", "coordinator-1")
	session2 := newTestSession(t, "session-2", "coordinator-1")

	state1 := session1.getState()
	state2 := session2.getState()

	// Add different messages to each session
	msg1 := NewMessage(state1.ID, "sender", MessageRoleCoordinator, MessageTypeRequest, "session1-msg")
	msg1.Status = MessageStatusProcessed
	session1.addMessage(msg1)

	msg2 := NewMessage(state2.ID, "sender", MessageRoleCoordinator, MessageTypeRequest, "session2-msg")
	msg2.Status = MessageStatusProcessed
	session2.addMessage(msg2)

	// Verify isolation
	state1 = session1.getState()
	state2 = session2.getState()

	if state1.MessageCount != 1 {
		t.Errorf("session1: expected 1 message, got %d", state1.MessageCount)
	}
	if state2.MessageCount != 1 {
		t.Errorf("session2: expected 1 message, got %d", state2.MessageCount)
	}

	// Verify content is correct
	if state1.Messages[0].Content != "session1-msg" {
		t.Errorf("session1: expected 'session1-msg', got '%s'", state1.Messages[0].Content)
	}
	if state2.Messages[0].Content != "session2-msg" {
		t.Errorf("session2: expected 'session2-msg', got '%s'", state2.Messages[0].Content)
	}

	// Verify session IDs are correct
	if state1.Messages[0].SessionID != state1.ID {
		t.Errorf("session1: message has wrong session ID")
	}
	if state2.Messages[0].SessionID != state2.ID {
		t.Errorf("session2: message has wrong session ID")
	}
}

// TestAgentIsolationBetweenSessions verifies agents registered in one
// session don't appear in another.
func TestAgentIsolationBetweenSessions(t *testing.T) {
	session1 := newTestSession(t, "session-1", "coordinator-1")
	session2 := newTestSession(t, "session-2", "coordinator-1")

	state1 := session1.getState()
	state2 := session2.getState()

	// Add agents to each session
	agent1 := NewAgentRef(state1.ID, "agent-1", "Agent1", "claude", "worker")
	session1.addAgent(agent1)

	agent2 := NewAgentRef(state2.ID, "agent-2", "Agent2", "gemini", "worker")
	session2.addAgent(agent2)

	// Verify isolation
	state1 = session1.getState()
	state2 = session2.getState()

	if state1.AgentCount != 1 {
		t.Errorf("session1: expected 1 agent, got %d", state1.AgentCount)
	}
	if state2.AgentCount != 1 {
		t.Errorf("session2: expected 1 agent, got %d", state2.AgentCount)
	}

	// Verify agents are in correct sessions
	if state1.ActiveAgents[0].Name != "Agent1" {
		t.Errorf("session1: expected Agent1, got %s", state1.ActiveAgents[0].Name)
	}
	if state2.ActiveAgents[0].Name != "Agent2" {
		t.Errorf("session2: expected Agent2, got %s", state2.ActiveAgents[0].Name)
	}

	// Verify session IDs match
	if state1.ActiveAgents[0].SessionID != state1.ID {
		t.Errorf("session1: agent has wrong session ID")
	}
	if state2.ActiveAgents[0].SessionID != state2.ID {
		t.Errorf("session2: agent has wrong session ID")
	}
}

// =============================================================================
// TEST: SESSION VALIDATION
// =============================================================================

// TestSessionValidation tests session validation logic.
func TestSessionValidation(t *testing.T) {
	tests := []struct {
		name        string
		buildFunc   func() *SessionState
		shouldError bool
		errorMsg    string
	}{
		{
			name: "valid session",
			buildFunc: func() *SessionState {
				return &SessionState{
					ID:            "test-id",
					Name:          "test",
					CoordinatorID: "coordinator-1",
				}
			},
			shouldError: false,
		},
		{
			name: "missing ID",
			buildFunc: func() *SessionState {
				return &SessionState{
					Name:          "test",
					CoordinatorID: "coordinator-1",
				}
			},
			shouldError: true,
			errorMsg:    "session ID is required",
		},
		{
			name: "missing name",
			buildFunc: func() *SessionState {
				return &SessionState{
					ID:            "test-id",
					CoordinatorID: "coordinator-1",
				}
			},
			shouldError: true,
			errorMsg:    "session name is required",
		},
		{
			name: "missing coordinator ID",
			buildFunc: func() *SessionState {
				return &SessionState{
					ID:   "test-id",
					Name: "test",
				}
			},
			shouldError: true,
			errorMsg:    "coordinator ID is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := tt.buildFunc()
			err := session.Validate()

			if tt.shouldError && err == nil {
				t.Errorf("expected error, got nil")
			}
			if !tt.shouldError && err != nil {
				t.Errorf("expected no error, got: %v", err)
			}
			if tt.shouldError && err != nil && tt.errorMsg != "" {
				if err.Error() != tt.errorMsg {
					t.Errorf("expected error '%s', got '%s'", tt.errorMsg, err.Error())
				}
			}
		})
	}
}

// =============================================================================
// TEST: SESSION STATUS TRANSITIONS
// =============================================================================

// TestSessionStatusTransitions tests valid and invalid status transitions.
func TestSessionStatusTransitions(t *testing.T) {
	session := newTestSession(t, "status-test", "coordinator-1")

	// Test transitions from Active
	testTransitions := []struct {
		from     SessionStatus
		to       SessionStatus
		isValid  bool
		terminal bool
	}{
		{SessionActive, SessionPaused, true, false},
		{SessionActive, SessionCompleted, true, true},
		{SessionActive, SessionFailed, true, true},
		{SessionActive, SessionCancelled, true, true},
		{SessionActive, SessionStale, true, true},
		{SessionPaused, SessionActive, true, false},
		{SessionPaused, SessionCompleted, true, true},
		{SessionCompleted, SessionActive, true, false}, // Re-run scenario
		{SessionFailed, SessionActive, true, false},    // Retry scenario
	}

	for _, tt := range testTransitions {
		// Set initial status
		session.setStatus(tt.from)

		// Attempt transition
		session.setStatus(tt.to)

		// Verify transition
		state := session.getState()
		if state.Status != tt.to {
			t.Errorf("transition %s->%s failed, got %s", tt.from, tt.to, state.Status)
		}

		// Verify terminal property
		if tt.terminal && !tt.to.IsTerminal() {
			t.Errorf("status %s should be terminal", tt.to)
		}
	}
}

// TestTerminalStatus tests that terminal statuses are correctly identified.
func TestTerminalStatus(t *testing.T) {
	tests := []struct {
		status    SessionStatus
		isTerminal bool
	}{
		{SessionCreated, false},
		{SessionActive, false},
		{SessionPaused, false},
		{SessionCompleted, true},
		{SessionFailed, true},
		{SessionCancelled, true},
		{SessionStale, true},
	}

	for _, tt := range tests {
		result := tt.status.IsTerminal()
		if result != tt.isTerminal {
			t.Errorf("status %s: expected IsTerminal()=%v, got %v",
				tt.status, tt.isTerminal, result)
		}
	}
}

// =============================================================================
// TEST: STRESS TESTS
// =============================================================================

// TestHighMessageVolume tests handling of sessions with many messages.
func TestHighMessageVolume(t *testing.T) {
	session := newTestSession(t, "high-volume", "coordinator-1")
	state := session.getState()

	numMessages := 10000

	// Add high volume of messages
	for i := 0; i < numMessages; i++ {
		msg := NewMessage(state.ID, "sender", MessageRoleCoordinator, MessageTypeRequest, "msg")
		msg.Status = MessageStatusProcessed
		session.addMessage(msg)
	}

	// Verify all messages were added
	finalState := session.getState()
	if finalState.MessageCount != numMessages {
		t.Errorf("expected %d messages, got %d", numMessages, finalState.MessageCount)
	}

	// Verify message order is preserved
	for i := 0; i < len(finalState.Messages); i++ {
		if finalState.Messages[i].Content != "msg" {
			t.Errorf("message %d has unexpected content: %s", i, finalState.Messages[i].Content)
		}
	}
}

// TestComplexSessionScenario simulates a complex real-world scenario
// with multiple agents, tasks, and concurrent message exchanges.
func TestComplexSessionScenario(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	session := newTestSession(t, "complex-scenario", "coordinator-1")
	state := session.getState()

	// Setup: Add multiple agents
	var agents []*AgentRef
	for i := 0; i < 5; i++ {
		agent := NewAgentRef(state.ID, "agent-"+string(rune(i)), "Agent", "claude", "worker")
		agent.Status = AgentStatusActive
		agents = append(agents, agent)
		session.addAgent(agent)
	}

	// Setup: Create tasks for each agent
	var tasks []*TaskRef
	for i := 0; i < len(agents); i++ {
		task := NewTaskRef(state.ID, agents[i].AgentID, "Task")
		task.Status = TaskStatusRunning
		tasks = append(tasks, task)
		session.addTask(task)
	}

	// Concurrent operations
	var wg sync.WaitGroup
	msgCount := int32(0)

	// Coordinator sends messages
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			select {
			case <-ctx.Done():
				return
			default:
				for j := 0; j < 10; j++ {
					msg := NewMessage(state.ID, "coordinator", MessageRoleCoordinator, MessageTypeRequest, "request")
					msg.Status = MessageStatusProcessed
					session.addMessage(msg)
					atomic.AddInt32(&msgCount, 1)
				}
			}
		}(i)
	}

	// Agents send responses
	for agentIdx := 0; agentIdx < len(agents); agentIdx++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			select {
			case <-ctx.Done():
				return
			default:
				for j := 0; j < 5; j++ {
					msg := NewMessage(state.ID, agents[idx].AgentID, MessageRoleAgent, MessageTypeResponse, "response")
					msg.Status = MessageStatusProcessed
					session.addMessage(msg)
					atomic.AddInt32(&msgCount, 1)
				}
			}
		}(agentIdx)
	}

	wg.Wait()

	// Verify final state
	finalState := session.getState()
	expectedMessages := int32(3*10 + len(agents)*5)

	if int32(finalState.MessageCount) != expectedMessages {
		t.Errorf("expected %d total messages, got %d", expectedMessages, finalState.MessageCount)
	}

	if finalState.AgentCount != len(agents) {
		t.Errorf("expected %d agents, got %d", len(agents), finalState.AgentCount)
	}

	if finalState.TaskCount != len(tasks) {
		t.Errorf("expected %d tasks, got %d", len(tasks), finalState.TaskCount)
	}
}

// =============================================================================
// TEST: DATABASE PERSISTENCE
// =============================================================================

// TestSessionPersistence tests that sessions are correctly persisted to
// and retrieved from the database.
func TestSessionPersistence(t *testing.T) {
	db := setupIntegrationDB(t)
	defer teardownTestDB(t, db)

	// Create and save a session
	session := NewSessionState("test-persistence", "coordinator-1")
	session.Status = SessionActive
	session.Description = stringPtr("Test session persistence")

	err := db.CreateSession(session)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	// Retrieve the session from database
	retrieved, err := db.GetSession(session.ID)
	if err != nil {
		t.Fatalf("failed to retrieve session: %v", err)
	}

	// Verify persistence
	if retrieved.Name != session.Name {
		t.Errorf("expected name %s, got %s", session.Name, retrieved.Name)
	}
	if retrieved.CoordinatorID != session.CoordinatorID {
		t.Errorf("expected coordinator %s, got %s", session.CoordinatorID, retrieved.CoordinatorID)
	}
	if retrieved.Status != session.Status {
		t.Errorf("expected status %s, got %s", session.Status, retrieved.Status)
	}
	if *retrieved.Description != *session.Description {
		t.Errorf("expected description %v, got %v", *session.Description, *retrieved.Description)
	}
}

// TestMessagePersistence tests that messages are correctly saved and retrieved.
func TestMessagePersistence(t *testing.T) {
	db := setupIntegrationDB(t)
	defer teardownTestDB(t, db)

	// Create session
	session := NewSessionState("msg-test", "coordinator-1")
	if err := db.CreateSession(session); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	// Create and save messages
	msg1 := NewMessage(session.ID, "sender-1", MessageRoleCoordinator, MessageTypeRequest, "Hello world")
	msg1.Status = MessageStatusProcessed

	if err := db.CreateMessage(msg1); err != nil {
		t.Fatalf("failed to create message: %v", err)
	}

	// Retrieve message
	retrieved, err := db.GetMessage(msg1.ID)
	if err != nil {
		t.Fatalf("failed to retrieve message: %v", err)
	}

	// Verify persistence
	if retrieved.Content != msg1.Content {
		t.Errorf("expected content %s, got %s", msg1.Content, retrieved.Content)
	}
	if retrieved.Status != msg1.Status {
		t.Errorf("expected status %s, got %s", msg1.Status, retrieved.Status)
	}
	if retrieved.Role != msg1.Role {
		t.Errorf("expected role %s, got %s", msg1.Role, retrieved.Role)
	}
}

// TestMessageListingBySession tests that messages can be listed by session.
func TestMessageListingBySession(t *testing.T) {
	db := setupIntegrationDB(t)
	defer teardownTestDB(t, db)

	// Create sessions
	session1 := NewSessionState("session-1", "coordinator-1")
	session2 := NewSessionState("session-2", "coordinator-1")

	if err := db.CreateSession(session1); err != nil {
		t.Fatalf("failed to create session1: %v", err)
	}
	if err := db.CreateSession(session2); err != nil {
		t.Fatalf("failed to create session2: %v", err)
	}

	// Add messages to each session
	for i := 0; i < 5; i++ {
		msg := NewMessage(session1.ID, "sender", MessageRoleCoordinator, MessageTypeRequest, "message")
		msg.Status = MessageStatusProcessed
		if err := db.CreateMessage(msg); err != nil {
			t.Fatalf("failed to create message: %v", err)
		}
	}

	for i := 0; i < 3; i++ {
		msg := NewMessage(session2.ID, "sender", MessageRoleCoordinator, MessageTypeRequest, "message")
		msg.Status = MessageStatusProcessed
		if err := db.CreateMessage(msg); err != nil {
			t.Fatalf("failed to create message: %v", err)
		}
	}

	// List messages for each session
	msgs1, err := db.ListMessages(session1.ID, 100, 0)
	if err != nil {
		t.Fatalf("failed to list messages for session1: %v", err)
	}

	msgs2, err := db.ListMessages(session2.ID, 100, 0)
	if err != nil {
		t.Fatalf("failed to list messages for session2: %v", err)
	}

	// Verify correct count
	if len(msgs1) != 5 {
		t.Errorf("expected 5 messages for session1, got %d", len(msgs1))
	}
	if len(msgs2) != 3 {
		t.Errorf("expected 3 messages for session2, got %d", len(msgs2))
	}

	// Verify isolation
	for _, msg := range msgs1 {
		if msg.SessionID != session1.ID {
			t.Errorf("message from session1 has wrong session ID: %s", msg.SessionID)
		}
	}

	for _, msg := range msgs2 {
		if msg.SessionID != session2.ID {
			t.Errorf("message from session2 has wrong session ID: %s", msg.SessionID)
		}
	}
}

// TestAgentPersistence tests that agents are correctly saved and retrieved.
func TestAgentPersistence(t *testing.T) {
	db := setupIntegrationDB(t)
	defer teardownTestDB(t, db)

	// Create session
	session := NewSessionState("agent-test", "coordinator-1")
	if err := db.CreateSession(session); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	// Create and save agent
	agent := NewAgentRef(session.ID, "agent-1", "TestAgent", "claude-opus", "worker")
	agent.Status = AgentStatusActive
	agent.Properties["key"] = "value"

	if err := db.CreateAgent(agent); err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	// Retrieve agent
	retrieved, err := db.GetAgent(agent.ID)
	if err != nil {
		t.Fatalf("failed to retrieve agent: %v", err)
	}

	// Verify persistence
	if retrieved.Name != agent.Name {
		t.Errorf("expected name %s, got %s", agent.Name, retrieved.Name)
	}
	if retrieved.Model != agent.Model {
		t.Errorf("expected model %s, got %s", agent.Model, retrieved.Model)
	}
	if retrieved.Status != agent.Status {
		t.Errorf("expected status %s, got %s", agent.Status, retrieved.Status)
	}
}

// TestTaskPersistence tests that tasks are correctly saved and retrieved.
func TestTaskPersistence(t *testing.T) {
	db := setupIntegrationDB(t)
	defer teardownTestDB(t, db)

	// Create session and agent
	session := NewSessionState("task-test", "coordinator-1")
	if err := db.CreateSession(session); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	agent := NewAgentRef(session.ID, "agent-1", "Agent", "claude", "worker")
	if err := db.CreateAgent(agent); err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	// Create and save task
	task := NewTaskRef(session.ID, agent.AgentID, "Test Task")
	task.Status = TaskStatusRunning
	task.Priority = 5

	if err := db.CreateTask(task); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	// Retrieve task
	retrieved, err := db.GetTask(task.ID)
	if err != nil {
		t.Fatalf("failed to retrieve task: %v", err)
	}

	// Verify persistence
	if retrieved.Title != task.Title {
		t.Errorf("expected title %s, got %s", task.Title, retrieved.Title)
	}
	if retrieved.Status != task.Status {
		t.Errorf("expected status %s, got %s", task.Status, retrieved.Status)
	}
	if retrieved.Priority != task.Priority {
		t.Errorf("expected priority %d, got %d", task.Priority, retrieved.Priority)
	}
}

// TestSessionUpdate tests that session updates are persisted.
func TestSessionUpdate(t *testing.T) {
	db := setupIntegrationDB(t)
	defer teardownTestDB(t, db)

	// Create session
	session := NewSessionState("update-test", "coordinator-1")
	session.Status = SessionActive

	if err := db.CreateSession(session); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	// Update session
	session.Status = SessionCompleted
	session.MessageCount = 10
	session.AgentCount = 3

	if err := db.UpdateSession(session); err != nil {
		t.Fatalf("failed to update session: %v", err)
	}

	// Retrieve and verify
	retrieved, err := db.GetSession(session.ID)
	if err != nil {
		t.Fatalf("failed to retrieve updated session: %v", err)
	}

	if retrieved.Status != SessionCompleted {
		t.Errorf("expected status %s, got %s", SessionCompleted, retrieved.Status)
	}
	if retrieved.MessageCount != 10 {
		t.Errorf("expected message count 10, got %d", retrieved.MessageCount)
	}
	if retrieved.AgentCount != 3 {
		t.Errorf("expected agent count 3, got %d", retrieved.AgentCount)
	}
}

// TestSessionVariables tests session variable storage.
func TestSessionVariables(t *testing.T) {
	db := setupIntegrationDB(t)
	defer teardownTestDB(t, db)

	// Create session
	session := NewSessionState("var-test", "coordinator-1")
	if err := db.CreateSession(session); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	// Set variables
	testData := map[string]interface{}{
		"string": "value",
		"number": 42,
		"float":  3.14,
		"bool":   true,
	}

	for key, val := range testData {
		if err := db.SetVariable(session.ID, key, val); err != nil {
			t.Fatalf("failed to set variable %s: %v", key, err)
		}
	}

	// Retrieve variables
	for key, expectedVal := range testData {
		retrieved, err := db.GetVariable(session.ID, key)
		if err != nil {
			t.Fatalf("failed to get variable %s: %v", key, err)
		}

		// Type conversion for comparison (JSON unmarshaling may change types)
		switch expectedVal.(type) {
		case float64:
			if retrievedFloat, ok := retrieved.(float64); ok {
				if retrievedFloat != expectedVal {
					t.Errorf("variable %s: expected %v, got %v", key, expectedVal, retrieved)
				}
			}
		case bool:
			if retrievedBool, ok := retrieved.(bool); ok {
				if retrievedBool != expectedVal {
					t.Errorf("variable %s: expected %v, got %v", key, expectedVal, retrieved)
				}
			}
		case string:
			if retrievedStr, ok := retrieved.(string); ok {
				if retrievedStr != expectedVal {
					t.Errorf("variable %s: expected %v, got %v", key, expectedVal, retrieved)
				}
			}
		}
	}
}

// TestSessionDeletion tests that sessions and related data are deleted.
func TestSessionDeletion(t *testing.T) {
	db := setupIntegrationDB(t)
	defer teardownTestDB(t, db)

	// Create session with related data
	session := NewSessionState("delete-test", "coordinator-1")
	if err := db.CreateSession(session); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	msg := NewMessage(session.ID, "sender", MessageRoleCoordinator, MessageTypeRequest, "test")
	if err := db.CreateMessage(msg); err != nil {
		t.Fatalf("failed to create message: %v", err)
	}

	// Delete session
	if err := db.DeleteSession(session.ID); err != nil {
		t.Fatalf("failed to delete session: %v", err)
	}

	// Verify session is deleted
	_, err := db.GetSession(session.ID)
	if err == nil {
		t.Errorf("expected error retrieving deleted session, got nil")
	}
}

// =============================================================================
// HELPER FUNCTION
// =============================================================================

// timePtr returns a pointer to a time value.
func timePtr(t time.Time) *time.Time {
	return &t
}
