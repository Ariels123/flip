// Package session provides state management for agent sessions
// with persistence, recovery, and multi-agent coordination capabilities.
//
// Session State Overview:
//
// A session represents a bounded execution context where:
// - Messages are exchanged between agents
// - Tasks are spawned and tracked
// - Agents maintain state and report status
// - Environment variables configure execution
//
// State Persistence:
//
// The session state is persisted to enable:
// - Recovery from crashes or disconnections
// - Audit trails of all activity
// - State restoration when coordinator reconnects
// - Cross-session replay and debugging
package session

import (
	"encoding/json"
	"fmt"
	"time"
)

// =============================================================================
// SESSION-LEVEL STATE
// =============================================================================

// SessionStatus represents the overall status of a session.
type SessionStatus string

const (
	// SessionCreated indicates the session has been created but not started.
	SessionCreated SessionStatus = "created"

	// SessionActive indicates the session is actively executing.
	SessionActive SessionStatus = "active"

	// SessionPaused indicates the session has been paused.
	SessionPaused SessionStatus = "paused"

	// SessionCompleted indicates the session finished successfully.
	SessionCompleted SessionStatus = "completed"

	// SessionFailed indicates the session failed with an unrecoverable error.
	SessionFailed SessionStatus = "failed"

	// SessionCancelled indicates the session was cancelled before completion.
	SessionCancelled SessionStatus = "cancelled"

	// SessionStale indicates the session has been inactive for too long.
	SessionStale SessionStatus = "stale"
)

// String returns the string representation of the session status.
func (s SessionStatus) String() string {
	return string(s)
}

// IsTerminal returns true if the status is a terminal state.
func (s SessionStatus) IsTerminal() bool {
	return s == SessionCompleted || s == SessionFailed || s == SessionCancelled || s == SessionStale
}

// IsRecoverable returns true if the session can be recovered from this state.
func (s SessionStatus) IsRecoverable() bool {
	return s == SessionActive || s == SessionPaused
}

// =============================================================================
// SESSION RUN
// =============================================================================

// SessionState represents the complete state of a session execution.
// This maps to the sessions SQLite table.
type SessionState struct {
	// ID is the unique identifier for this session (UUID).
	ID string `json:"id" db:"id"`

	// Name is a human-readable name for the session.
	Name string `json:"name" db:"name"`

	// Status is the current status of the session.
	Status SessionStatus `json:"status" db:"status"`

	// CoordinatorID is the agent ID of the coordinator managing this session.
	CoordinatorID string `json:"coordinator_id" db:"coordinator_id"`

	// ParentSessionID references the parent session if this is a sub-session (optional).
	ParentSessionID *string `json:"parent_session_id,omitempty" db:"parent_session_id"`

	// Description provides context about the session's purpose.
	Description string `json:"description,omitempty" db:"description"`

	// Messages is the ordered list of all messages exchanged in this session.
	Messages []Message `json:"messages" db:"-"`

	// ActiveAgents contains references to all agents currently in the session.
	ActiveAgents []AgentRef `json:"active_agents" db:"-"`

	// Tasks contains references to all tasks spawned in this session.
	Tasks []TaskRef `json:"tasks" db:"-"`

	// Environment contains session-level environment variables and configuration.
	Environment map[string]string `json:"environment" db:"-"`

	// Variables stores session-scoped variables accessible to agents.
	Variables map[string]interface{} `json:"variables" db:"-"`

	// CreatedAt is when the session was created.
	CreatedAt time.Time `json:"created_at" db:"created_at"`

	// StartedAt is when the session started executing.
	StartedAt *time.Time `json:"started_at,omitempty" db:"started_at"`

	// CompletedAt is when the session finished (success or failure).
	CompletedAt *time.Time `json:"completed_at,omitempty" db:"completed_at"`

	// UpdatedAt is the last time any field was updated.
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`

	// LastHeartbeatAt is the last time we heard from the coordinator.
	LastHeartbeatAt *time.Time `json:"last_heartbeat_at,omitempty" db:"last_heartbeat_at"`

	// MessageCount tracks the total number of messages in this session.
	MessageCount int `json:"message_count" db:"message_count"`

	// AgentCount tracks the number of active agents.
	AgentCount int `json:"agent_count" db:"agent_count"`

	// TaskCount tracks the total number of tasks spawned.
	TaskCount int `json:"task_count" db:"task_count"`

	// ErrorCount tracks the number of errors that occurred.
	ErrorCount int `json:"error_count" db:"error_count"`

	// Metadata stores arbitrary key-value pairs for extensibility.
	Metadata map[string]interface{} `json:"metadata,omitempty" db:"metadata"`
}

// Duration returns the elapsed time for the session.
func (s *SessionState) Duration() time.Duration {
	if s.StartedAt == nil {
		return 0
	}
	if s.CompletedAt != nil {
		return s.CompletedAt.Sub(*s.StartedAt)
	}
	return time.Since(*s.StartedAt)
}

// Progress returns the completion percentage (0-100) based on task completion.
func (s *SessionState) Progress() float64 {
	if s.TaskCount == 0 {
		return 0
	}
	completedTasks := 0
	for _, task := range s.Tasks {
		if task.Status == TaskStatusCompleted || task.Status == TaskStatusSkipped {
			completedTasks++
		}
	}
	return float64(completedTasks) / float64(s.TaskCount) * 100
}

// IsHealthy checks if the session is in a healthy state (has recent heartbeat).
func (s *SessionState) IsHealthy(maxStaleDuration time.Duration) bool {
	if s.LastHeartbeatAt == nil {
		return false
	}
	return time.Since(*s.LastHeartbeatAt) < maxStaleDuration
}

// =============================================================================
// MESSAGE
// =============================================================================

// Message represents a single message exchanged in the session.
// This maps to the session_messages SQLite table.
type Message struct {
	// ID is the unique identifier for this message (UUID).
	ID string `json:"id" db:"id"`

	// SessionID references the parent session.
	SessionID string `json:"session_id" db:"session_id"`

	// Role indicates who sent the message (coordinator, agent, system).
	Role MessageRole `json:"role" db:"role"`

	// SenderID is the ID of the agent or system that sent this message.
	SenderID string `json:"sender_id" db:"sender_id"`

	// RecipientID is the ID of the agent that should receive this message (optional).
	// If empty, it's a broadcast to all agents.
	RecipientID *string `json:"recipient_id,omitempty" db:"recipient_id"`

	// Content is the message content (can be JSON for structured data).
	Content string `json:"content" db:"content"`

	// ContentType indicates the format of the content (text, json, markdown).
	ContentType string `json:"content_type" db:"content_type"`

	// MessageType indicates what kind of message this is.
	MessageType MessageType `json:"message_type" db:"message_type"`

	// Status indicates whether the message was processed.
	Status MessageStatus `json:"status" db:"status"`

	// TokensUsed tracks tokens consumed by this message (for LLM messages).
	TokensUsed *TokenMetrics `json:"tokens_used,omitempty" db:"tokens_used"`

	// Metadata stores arbitrary key-value pairs (tags, labels, etc).
	Metadata map[string]interface{} `json:"metadata,omitempty" db:"metadata"`

	// CreatedAt is when the message was created.
	CreatedAt time.Time `json:"created_at" db:"created_at"`

	// ProcessedAt is when the message was processed.
	ProcessedAt *time.Time `json:"processed_at,omitempty" db:"processed_at"`

	// Error contains any error that occurred while processing this message.
	Error *string `json:"error,omitempty" db:"error"`
}

// MessageRole indicates who sent the message.
type MessageRole string

const (
	// MessageRoleCoordinator indicates the coordinator sent this message.
	MessageRoleCoordinator MessageRole = "coordinator"

	// MessageRoleAgent indicates an agent sent this message.
	MessageRoleAgent MessageRole = "agent"

	// MessageRoleSystem indicates the system sent this message.
	MessageRoleSystem MessageRole = "system"

	// MessageRoleUser indicates a human user sent this message.
	MessageRoleUser MessageRole = "user"
)

// MessageType indicates what kind of message this is.
type MessageType string

const (
	// MessageTypeRequest is a request to perform an action.
	MessageTypeRequest MessageType = "request"

	// MessageTypeResponse is a response to a request.
	MessageTypeResponse MessageType = "response"

	// MessageTypeStatus is a status update.
	MessageTypeStatus MessageType = "status"

	// MessageTypeError indicates an error occurred.
	MessageTypeError MessageType = "error"

	// MessageTypeTask is a task assignment.
	MessageTypeTask MessageType = "task"

	// MessageTypeSignal is a control signal (pause, resume, etc).
	MessageTypeSignal MessageType = "signal"

	// MessageTypeHeartbeat is a keepalive heartbeat.
	MessageTypeHeartbeat MessageType = "heartbeat"
)

// MessageStatus indicates whether the message was processed.
type MessageStatus string

const (
	// MessageStatusPending indicates the message is waiting to be processed.
	MessageStatusPending MessageStatus = "pending"

	// MessageStatusProcessing indicates the message is being processed.
	MessageStatusProcessing MessageStatus = "processing"

	// MessageStatusProcessed indicates the message was successfully processed.
	MessageStatusProcessed MessageStatus = "processed"

	// MessageStatusFailed indicates the message failed to process.
	MessageStatusFailed MessageStatus = "failed"

	// MessageStatusDelivered indicates the message was delivered to the recipient.
	MessageStatusDelivered MessageStatus = "delivered"
)

// TokenMetrics tracks token usage for a message.
type TokenMetrics struct {
	// InputTokens is the number of input tokens.
	InputTokens int `json:"input_tokens"`

	// OutputTokens is the number of output tokens.
	OutputTokens int `json:"output_tokens"`

	// TotalTokens is the total tokens used.
	TotalTokens int `json:"total_tokens"`

	// Cost is the estimated cost in USD.
	Cost float64 `json:"cost"`
}

// =============================================================================
// AGENT REFERENCE
// =============================================================================

// AgentRef represents a reference to an active agent in the session.
// This maps to the session_agents SQLite table.
type AgentRef struct {
	// ID is the unique identifier for this agent in the session context (UUID).
	ID string `json:"id" db:"id"`

	// SessionID references the parent session.
	SessionID string `json:"session_id" db:"session_id"`

	// AgentID is the global agent ID.
	AgentID string `json:"agent_id" db:"agent_id"`

	// Name is the human-readable name of the agent.
	Name string `json:"name" db:"name"`

	// Model is the LLM model this agent uses (e.g., "claude-opus", "gemini-2.5-pro").
	Model string `json:"model" db:"model"`

	// Role describes what this agent does (coordinator, worker, analyzer, etc).
	Role string `json:"role" db:"role"`

	// Status indicates the current status of the agent.
	Status AgentStatus `json:"status" db:"status"`

	// JoinedAt is when the agent joined the session.
	JoinedAt time.Time `json:"joined_at" db:"joined_at"`

	// LastActivityAt is when the agent last sent a message.
	LastActivityAt *time.Time `json:"last_activity_at,omitempty" db:"last_activity_at"`

	// LeftAt is when the agent left the session (optional).
	LeftAt *time.Time `json:"left_at,omitempty" db:"left_at"`

	// MessageCount is the number of messages this agent has sent.
	MessageCount int `json:"message_count" db:"message_count"`

	// TaskCount is the number of tasks assigned to this agent.
	TaskCount int `json:"task_count" db:"task_count"`

	// Properties stores agent-specific configuration and state.
	Properties map[string]interface{} `json:"properties" db:"properties"`

	// Metadata stores arbitrary key-value pairs.
	Metadata map[string]interface{} `json:"metadata,omitempty" db:"metadata"`
}

// AgentStatus indicates the status of an agent in the session.
type AgentStatus string

const (
	// AgentStatusJoining indicates the agent is joining the session.
	AgentStatusJoining AgentStatus = "joining"

	// AgentStatusActive indicates the agent is active and ready to work.
	AgentStatusActive AgentStatus = "active"

	// AgentStatusBusy indicates the agent is busy processing a task.
	AgentStatusBusy AgentStatus = "busy"

	// AgentStatusWaiting indicates the agent is waiting for a response.
	AgentStatusWaiting AgentStatus = "waiting"

	// AgentStatusPaused indicates the agent has been paused.
	AgentStatusPaused AgentStatus = "paused"

	// AgentStatusError indicates the agent encountered an error.
	AgentStatusError AgentStatus = "error"

	// AgentStatusDisconnected indicates the agent is disconnected.
	AgentStatusDisconnected AgentStatus = "disconnected"

	// AgentStatusLeft indicates the agent has left the session.
	AgentStatusLeft AgentStatus = "left"
)

// =============================================================================
// TASK REFERENCE
// =============================================================================

// TaskRef represents a reference to a task spawned within the session.
// This maps to the session_tasks SQLite table.
type TaskRef struct {
	// ID is the unique identifier for this task (UUID).
	ID string `json:"id" db:"id"`

	// SessionID references the parent session.
	SessionID string `json:"session_id" db:"session_id"`

	// AssignedAgentID is the ID of the agent assigned to this task.
	AssignedAgentID string `json:"assigned_agent_id" db:"assigned_agent_id"`

	// Title is a human-readable description of the task.
	Title string `json:"title" db:"title"`

	// Description provides detailed context about the task.
	Description string `json:"description,omitempty" db:"description"`

	// Status indicates the current status of the task.
	Status TaskStatus `json:"status" db:"status"`

	// Input is the input provided to the task (JSON).
	Input json.RawMessage `json:"input,omitempty" db:"input"`

	// Result is the output from the task (JSON), set when completed.
	Result json.RawMessage `json:"result,omitempty" db:"result"`

	// Error contains the error message if the task failed.
	Error *string `json:"error,omitempty" db:"error"`

	// Priority indicates task urgency (higher = more urgent).
	Priority int `json:"priority" db:"priority"`

	// RetryCount is the number of times this task has been retried.
	RetryCount int `json:"retry_count" db:"retry_count"`

	// MaxRetries is the maximum number of retries allowed.
	MaxRetries int `json:"max_retries" db:"max_retries"`

	// CreatedAt is when the task was created.
	CreatedAt time.Time `json:"created_at" db:"created_at"`

	// StartedAt is when the task started executing.
	StartedAt *time.Time `json:"started_at,omitempty" db:"started_at"`

	// CompletedAt is when the task finished.
	CompletedAt *time.Time `json:"completed_at,omitempty" db:"completed_at"`

	// DueAt is when the task is due (optional).
	DueAt *time.Time `json:"due_at,omitempty" db:"due_at"`

	// Metrics contains performance metrics for the task execution.
	Metrics *TaskMetrics `json:"metrics,omitempty" db:"metrics"`

	// Dependencies lists IDs of tasks that must complete before this one.
	Dependencies []string `json:"dependencies" db:"dependencies"`

	// Tags are arbitrary labels for categorization and filtering.
	Tags []string `json:"tags" db:"tags"`

	// Metadata stores arbitrary key-value pairs.
	Metadata map[string]interface{} `json:"metadata,omitempty" db:"metadata"`
}

// TaskStatus indicates the status of a task.
type TaskStatus string

const (
	// TaskStatusCreated indicates the task has been created but not started.
	TaskStatusCreated TaskStatus = "created"

	// TaskStatusWaiting indicates the task is waiting for dependencies.
	TaskStatusWaiting TaskStatus = "waiting"

	// TaskStatusAssigned indicates the task has been assigned to an agent.
	TaskStatusAssigned TaskStatus = "assigned"

	// TaskStatusRunning indicates the task is currently executing.
	TaskStatusRunning TaskStatus = "running"

	// TaskStatusCompleted indicates the task finished successfully.
	TaskStatusCompleted TaskStatus = "completed"

	// TaskStatusFailed indicates the task failed.
	TaskStatusFailed TaskStatus = "failed"

	// TaskStatusSkipped indicates the task was skipped.
	TaskStatusSkipped TaskStatus = "skipped"

	// TaskStatusCancelled indicates the task was cancelled.
	TaskStatusCancelled TaskStatus = "cancelled"
)

// TaskMetrics contains performance metrics for task execution.
type TaskMetrics struct {
	// TokensUsed tracks tokens consumed by the task.
	TokensUsed *TokenMetrics `json:"tokens_used,omitempty"`

	// DurationMs is the execution time in milliseconds.
	DurationMs int64 `json:"duration_ms,omitempty"`

	// MemoryUsedBytes is the peak memory usage.
	MemoryUsedBytes int64 `json:"memory_used_bytes,omitempty"`

	// Cost is the estimated cost in USD.
	Cost float64 `json:"cost,omitempty"`
}

// =============================================================================
// SESSION SUMMARY
// =============================================================================

// SessionSummary provides a concise representation of a session for listing and display.
type SessionSummary struct {
	// ID is the unique identifier for this session.
	ID string `json:"id"`

	// Name is the human-readable name for the session.
	Name string `json:"name"`

	// Status is the current status of the session.
	Status SessionStatus `json:"status"`

	// CoordinatorID is the agent ID of the coordinator managing this session.
	CoordinatorID string `json:"coordinator_id"`

	// CreatedAt is when the session was created.
	CreatedAt time.Time `json:"created_at"`

	// MessageCount tracks the total number of messages in this session.
	MessageCount int `json:"message_count"`

	// AgentCount tracks the number of active agents.
	AgentCount int `json:"agent_count"`

	// TaskCount tracks the total number of tasks spawned.
	TaskCount int `json:"task_count"`

	// ErrorCount tracks the number of errors that occurred.
	ErrorCount int `json:"error_count"`
}

// ToSummary converts a SessionState to a SessionSummary.
func (s *SessionState) ToSummary() *SessionSummary {
	return &SessionSummary{
		ID:            s.ID,
		Name:          s.Name,
		Status:        s.Status,
		CoordinatorID: s.CoordinatorID,
		CreatedAt:     s.CreatedAt,
		MessageCount:  s.MessageCount,
		AgentCount:    s.AgentCount,
		TaskCount:     s.TaskCount,
		ErrorCount:    s.ErrorCount,
	}
}

// =============================================================================
// PERSISTENCE SCHEMA
// =============================================================================

// PocketBaseCollections returns the PocketBase collection definitions.
// These should be added via a migration file.
const PocketBaseCollections = `
-- Sessions Table
-- Stores metadata about each session execution

CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'created',
    coordinator_id TEXT NOT NULL,
    parent_session_id TEXT,
    description TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    started_at DATETIME,
    completed_at DATETIME,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_heartbeat_at DATETIME,
    message_count INTEGER NOT NULL DEFAULT 0,
    agent_count INTEGER NOT NULL DEFAULT 0,
    task_count INTEGER NOT NULL DEFAULT 0,
    error_count INTEGER NOT NULL DEFAULT 0,
    metadata TEXT
);

CREATE INDEX idx_sessions_status ON sessions(status);
CREATE INDEX idx_sessions_coordinator_id ON sessions(coordinator_id);
CREATE INDEX idx_sessions_parent_session_id ON sessions(parent_session_id);
CREATE INDEX idx_sessions_created_at ON sessions(created_at DESC);

-- Session Messages Table
-- Logs all messages exchanged within a session

CREATE TABLE IF NOT EXISTS session_messages (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    role TEXT NOT NULL,
    sender_id TEXT NOT NULL,
    recipient_id TEXT,
    content TEXT NOT NULL,
    content_type TEXT NOT NULL DEFAULT 'text',
    message_type TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    tokens_used TEXT,
    metadata TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    processed_at DATETIME,
    error TEXT,
    FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE
);

CREATE INDEX idx_session_messages_session_id ON session_messages(session_id);
CREATE INDEX idx_session_messages_sender_id ON session_messages(sender_id);
CREATE INDEX idx_session_messages_recipient_id ON session_messages(recipient_id);
CREATE INDEX idx_session_messages_message_type ON session_messages(message_type);
CREATE INDEX idx_session_messages_status ON session_messages(status);
CREATE INDEX idx_session_messages_created_at ON session_messages(created_at DESC);

-- Session Agents Table
-- Tracks agents participating in a session

CREATE TABLE IF NOT EXISTS session_agents (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    agent_id TEXT NOT NULL,
    name TEXT NOT NULL,
    model TEXT NOT NULL,
    role TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'joining',
    joined_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_activity_at DATETIME,
    left_at DATETIME,
    message_count INTEGER NOT NULL DEFAULT 0,
    task_count INTEGER NOT NULL DEFAULT 0,
    properties TEXT,
    metadata TEXT,
    FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE,
    UNIQUE (session_id, agent_id)
);

CREATE INDEX idx_session_agents_session_id ON session_agents(session_id);
CREATE INDEX idx_session_agents_agent_id ON session_agents(agent_id);
CREATE INDEX idx_session_agents_status ON session_agents(status);
CREATE INDEX idx_session_agents_role ON session_agents(role);

-- Session Tasks Table
-- Tracks tasks spawned within a session

CREATE TABLE IF NOT EXISTS session_tasks (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    assigned_agent_id TEXT NOT NULL,
    title TEXT NOT NULL,
    description TEXT,
    status TEXT NOT NULL DEFAULT 'created',
    input TEXT,
    result TEXT,
    error TEXT,
    priority INTEGER NOT NULL DEFAULT 0,
    retry_count INTEGER NOT NULL DEFAULT 0,
    max_retries INTEGER NOT NULL DEFAULT 3,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    started_at DATETIME,
    completed_at DATETIME,
    due_at DATETIME,
    metrics TEXT,
    dependencies TEXT,
    tags TEXT,
    metadata TEXT,
    FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE,
    FOREIGN KEY (assigned_agent_id) REFERENCES session_agents(agent_id) ON DELETE RESTRICT
);

CREATE INDEX idx_session_tasks_session_id ON session_tasks(session_id);
CREATE INDEX idx_session_tasks_assigned_agent_id ON session_tasks(assigned_agent_id);
CREATE INDEX idx_session_tasks_status ON session_tasks(status);
CREATE INDEX idx_session_tasks_priority ON session_tasks(priority DESC);
CREATE INDEX idx_session_tasks_created_at ON session_tasks(created_at DESC);
CREATE INDEX idx_session_tasks_due_at ON session_tasks(due_at);

-- Session Variables Table
-- Stores session-scoped variables and configuration

CREATE TABLE IF NOT EXISTS session_variables (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    key TEXT NOT NULL,
    value TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE,
    UNIQUE (session_id, key)
);

CREATE INDEX idx_session_variables_session_id ON session_variables(session_id);
`

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// NewSessionState creates a new session with default values.
func NewSessionState(name, coordinatorID string) *SessionState {
	now := time.Now()
	return &SessionState{
		ID:            generateID(),
		Name:          name,
		CoordinatorID: coordinatorID,
		Status:        SessionCreated,
		Messages:      make([]Message, 0),
		ActiveAgents:  make([]AgentRef, 0),
		Tasks:         make([]TaskRef, 0),
		Environment:   make(map[string]string),
		Variables:     make(map[string]interface{}),
		CreatedAt:     now,
		UpdatedAt:     now,
		MessageCount:  0,
		AgentCount:    0,
		TaskCount:     0,
		ErrorCount:    0,
		Metadata:      make(map[string]interface{}),
	}
}

// NewMessage creates a new message.
func NewMessage(sessionID, senderID string, role MessageRole, messageType MessageType, content string) *Message {
	return &Message{
		ID:          generateID(),
		SessionID:   sessionID,
		SenderID:    senderID,
		Role:        role,
		MessageType: messageType,
		Content:     content,
		ContentType: "text",
		Status:      MessageStatusPending,
		CreatedAt:   time.Now(),
	}
}

// NewAgentRef creates a new agent reference.
func NewAgentRef(sessionID, agentID, name, model, role string) *AgentRef {
	return &AgentRef{
		ID:           generateID(),
		SessionID:    sessionID,
		AgentID:      agentID,
		Name:         name,
		Model:        model,
		Role:         role,
		Status:       AgentStatusJoining,
		JoinedAt:     time.Now(),
		MessageCount: 0,
		TaskCount:    0,
		Properties:   make(map[string]interface{}),
		Metadata:     make(map[string]interface{}),
	}
}

// NewTaskRef creates a new task reference.
func NewTaskRef(sessionID, agentID, title string) *TaskRef {
	return &TaskRef{
		ID:              generateID(),
		SessionID:       sessionID,
		AssignedAgentID: agentID,
		Title:           title,
		Status:          TaskStatusCreated,
		Priority:        0,
		RetryCount:      0,
		MaxRetries:      3,
		CreatedAt:       time.Now(),
		Dependencies:    make([]string, 0),
		Tags:            make([]string, 0),
		Metadata:        make(map[string]interface{}),
	}
}

// generateID generates a unique ID for session entities.
// Uses a simple timestamp-based approach; in production, use UUID.
func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// =============================================================================
// VALIDATION
// =============================================================================

// Validate checks if the session state is in a valid state.
func (s *SessionState) Validate() error {
	if s.ID == "" {
		return fmt.Errorf("session ID is required")
	}
	if s.Name == "" {
		return fmt.Errorf("session name is required")
	}
	if s.CoordinatorID == "" {
		return fmt.Errorf("coordinator ID is required")
	}
	return nil
}

// Validate checks if the message is in a valid state.
func (m *Message) Validate() error {
	if m.ID == "" {
		return fmt.Errorf("message ID is required")
	}
	if m.SessionID == "" {
		return fmt.Errorf("session ID is required")
	}
	if m.SenderID == "" {
		return fmt.Errorf("sender ID is required")
	}
	if m.Content == "" {
		return fmt.Errorf("message content is required")
	}
	return nil
}

// Validate checks if the agent reference is in a valid state.
func (a *AgentRef) Validate() error {
	if a.ID == "" {
		return fmt.Errorf("agent ref ID is required")
	}
	if a.SessionID == "" {
		return fmt.Errorf("session ID is required")
	}
	if a.AgentID == "" {
		return fmt.Errorf("agent ID is required")
	}
	if a.Name == "" {
		return fmt.Errorf("agent name is required")
	}
	if a.Model == "" {
		return fmt.Errorf("agent model is required")
	}
	return nil
}

// Validate checks if the task reference is in a valid state.
func (t *TaskRef) Validate() error {
	if t.ID == "" {
		return fmt.Errorf("task ID is required")
	}
	if t.SessionID == "" {
		return fmt.Errorf("session ID is required")
	}
	if t.AssignedAgentID == "" {
		return fmt.Errorf("assigned agent ID is required")
	}
	if t.Title == "" {
		return fmt.Errorf("task title is required")
	}
	return nil
}

// =============================================================================
// EVENT TYPES
// =============================================================================

// SessionEvent represents an event that occurred during session execution.
type SessionEvent struct {
	// Type is the type of event.
	Type SessionEventType `json:"type"`

	// SessionID is the session this event belongs to.
	SessionID string `json:"session_id"`

	// AgentID is the agent involved in this event (optional).
	AgentID *string `json:"agent_id,omitempty"`

	// TaskID is the task involved in this event (optional).
	TaskID *string `json:"task_id,omitempty"`

	// Timestamp is when the event occurred.
	Timestamp time.Time `json:"timestamp"`

	// Data contains event-specific data.
	Data map[string]interface{} `json:"data,omitempty"`
}

// SessionEventType represents the type of session event.
type SessionEventType string

const (
	EventSessionCreated   SessionEventType = "session.created"
	EventSessionStarted   SessionEventType = "session.started"
	EventSessionCompleted SessionEventType = "session.completed"
	EventSessionFailed    SessionEventType = "session.failed"
	EventSessionPaused    SessionEventType = "session.paused"
	EventSessionResumed   SessionEventType = "session.resumed"
	EventSessionCancelled SessionEventType = "session.cancelled"

	EventAgentJoined        SessionEventType = "agent.joined"
	EventAgentLeft          SessionEventType = "agent.left"
	EventAgentStatusChanged SessionEventType = "agent.status_changed"

	EventTaskSpawned   SessionEventType = "task.spawned"
	EventTaskStarted   SessionEventType = "task.started"
	EventTaskCompleted SessionEventType = "task.completed"
	EventTaskFailed    SessionEventType = "task.failed"
	EventTaskRetried   SessionEventType = "task.retried"

	EventMessageReceived  SessionEventType = "message.received"
	EventMessageProcessed SessionEventType = "message.processed"
	EventMessageFailed    SessionEventType = "message.failed"

	EventHeartbeat SessionEventType = "heartbeat"
	EventError     SessionEventType = "error"
)
