package session

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// DB provides session database operations.
type DB struct {
	conn *sql.DB
}

// NewDB creates a new session database handler.
func NewDB(conn *sql.DB) *DB {
	return &DB{conn: conn}
}

// InitDB creates the session database tables if they don't exist.
// This function should be called during application startup to ensure
// the schema is initialized.
func InitDB(db *sql.DB) error {
	// Execute the schema creation statements
	if _, err := db.Exec(PocketBaseCollections); err != nil {
		return fmt.Errorf("failed to initialize session schema: %w", err)
	}
	return nil
}

// =============================================================================
// SESSION OPERATIONS
// =============================================================================

// CreateSession saves a new session to the database.
func (db *DB) CreateSession(session *SessionState) error {
	if err := session.Validate(); err != nil {
		return fmt.Errorf("invalid session: %w", err)
	}

	metadataJSON, err := json.Marshal(session.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `
		INSERT INTO sessions (
			id, name, status, coordinator_id, parent_session_id, description,
			created_at, updated_at, message_count, agent_count, task_count, error_count, metadata
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err = db.conn.Exec(
		query,
		session.ID, session.Name, string(session.Status), session.CoordinatorID,
		session.ParentSessionID, session.Description,
		session.CreatedAt, session.UpdatedAt,
		session.MessageCount, session.AgentCount, session.TaskCount, session.ErrorCount,
		string(metadataJSON),
	)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	return nil
}

// GetSession retrieves a session by ID.
func (db *DB) GetSession(sessionID string) (*SessionState, error) {
	session := &SessionState{
		Messages:     make([]Message, 0),
		ActiveAgents: make([]AgentRef, 0),
		Tasks:        make([]TaskRef, 0),
		Environment:  make(map[string]string),
		Variables:    make(map[string]interface{}),
		Metadata:     make(map[string]interface{}),
	}

	query := `
		SELECT id, name, status, coordinator_id, parent_session_id, description,
		       created_at, started_at, completed_at, updated_at, last_heartbeat_at,
		       message_count, agent_count, task_count, error_count, metadata
		FROM sessions WHERE id = ?
	`

	var metadataJSON []byte
	var status string

	err := db.conn.QueryRow(query, sessionID).Scan(
		&session.ID, &session.Name, &status, &session.CoordinatorID, &session.ParentSessionID,
		&session.Description, &session.CreatedAt, &session.StartedAt, &session.CompletedAt,
		&session.UpdatedAt, &session.LastHeartbeatAt,
		&session.MessageCount, &session.AgentCount, &session.TaskCount, &session.ErrorCount,
		&metadataJSON,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("session not found: %s", sessionID)
		}
		return nil, fmt.Errorf("failed to retrieve session: %w", err)
	}

	session.Status = SessionStatus(status)

	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &session.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	return session, nil
}

// UpdateSession updates an existing session.
func (db *DB) UpdateSession(session *SessionState) error {
	if err := session.Validate(); err != nil {
		return fmt.Errorf("invalid session: %w", err)
	}

	metadataJSON, err := json.Marshal(session.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `
		UPDATE sessions SET
			name = ?, status = ?, coordinator_id = ?, parent_session_id = ?, description = ?,
			started_at = ?, completed_at = ?, updated_at = ?, last_heartbeat_at = ?,
			message_count = ?, agent_count = ?, task_count = ?, error_count = ?, metadata = ?
		WHERE id = ?
	`

	result, err := db.conn.Exec(
		query,
		session.Name, string(session.Status), session.CoordinatorID, session.ParentSessionID,
		session.Description, session.StartedAt, session.CompletedAt, time.Now(), session.LastHeartbeatAt,
		session.MessageCount, session.AgentCount, session.TaskCount, session.ErrorCount,
		string(metadataJSON), session.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update session: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("session not found: %s", session.ID)
	}

	return nil
}

// ListSessions returns sessions matching the given status (or all if empty).
func (db *DB) ListSessions(status string, limit int, offset int) ([]*SessionState, error) {
	query := `SELECT id, name, status, coordinator_id, parent_session_id, description,
	                 created_at, started_at, completed_at, updated_at, last_heartbeat_at,
	                 message_count, agent_count, task_count, error_count, metadata
	          FROM sessions`

	args := []interface{}{}
	if status != "" {
		query += " WHERE status = ?"
		args = append(args, status)
	}
	query += " ORDER BY created_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*SessionState
	for rows.Next() {
		session := &SessionState{
			Messages:     make([]Message, 0),
			ActiveAgents: make([]AgentRef, 0),
			Tasks:        make([]TaskRef, 0),
			Environment:  make(map[string]string),
			Variables:    make(map[string]interface{}),
			Metadata:     make(map[string]interface{}),
		}

		var metadataJSON []byte
		var statusStr string

		if err := rows.Scan(
			&session.ID, &session.Name, &statusStr, &session.CoordinatorID, &session.ParentSessionID,
			&session.Description, &session.CreatedAt, &session.StartedAt, &session.CompletedAt,
			&session.UpdatedAt, &session.LastHeartbeatAt,
			&session.MessageCount, &session.AgentCount, &session.TaskCount, &session.ErrorCount,
			&metadataJSON,
		); err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}

		session.Status = SessionStatus(statusStr)
		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &session.Metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		}

		sessions = append(sessions, session)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating sessions: %w", err)
	}

	return sessions, nil
}

// DeleteSession removes a session and cascades to related records.
func (db *DB) DeleteSession(sessionID string) error {
	query := `DELETE FROM sessions WHERE id = ?`
	result, err := db.conn.Exec(query, sessionID)
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	return nil
}

// =============================================================================
// MESSAGE OPERATIONS
// =============================================================================

// CreateMessage saves a new message to the database.
func (db *DB) CreateMessage(msg *Message) error {
	if err := msg.Validate(); err != nil {
		return fmt.Errorf("invalid message: %w", err)
	}

	tokensJSON, err := json.Marshal(msg.TokensUsed)
	if err != nil {
		return fmt.Errorf("failed to marshal tokens: %w", err)
	}

	metadataJSON, err := json.Marshal(msg.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `
		INSERT INTO session_messages (
			id, session_id, role, sender_id, recipient_id, content, content_type,
			message_type, status, tokens_used, metadata, created_at, processed_at, error
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err = db.conn.Exec(
		query,
		msg.ID, msg.SessionID, string(msg.Role), msg.SenderID, msg.RecipientID,
		msg.Content, msg.ContentType, string(msg.MessageType), string(msg.Status),
		string(tokensJSON), string(metadataJSON), msg.CreatedAt, msg.ProcessedAt, msg.Error,
	)
	if err != nil {
		return fmt.Errorf("failed to create message: %w", err)
	}
	return nil
}

// GetMessage retrieves a message by ID.
func (db *DB) GetMessage(messageID string) (*Message, error) {
	msg := &Message{
		Metadata: make(map[string]interface{}),
	}

	query := `
		SELECT id, session_id, role, sender_id, recipient_id, content, content_type,
		       message_type, status, tokens_used, metadata, created_at, processed_at, error
		FROM session_messages WHERE id = ?
	`

	var tokensJSON, metadataJSON []byte
	var role, messageType, status string

	err := db.conn.QueryRow(query, messageID).Scan(
		&msg.ID, &msg.SessionID, &role, &msg.SenderID, &msg.RecipientID,
		&msg.Content, &msg.ContentType, &messageType, &status,
		&tokensJSON, &metadataJSON, &msg.CreatedAt, &msg.ProcessedAt, &msg.Error,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("message not found: %s", messageID)
		}
		return nil, fmt.Errorf("failed to retrieve message: %w", err)
	}

	msg.Role = MessageRole(role)
	msg.MessageType = MessageType(messageType)
	msg.Status = MessageStatus(status)

	if len(tokensJSON) > 0 && string(tokensJSON) != "null" {
		if err := json.Unmarshal(tokensJSON, &msg.TokensUsed); err != nil {
			return nil, fmt.Errorf("failed to unmarshal tokens: %w", err)
		}
	}

	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &msg.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	return msg, nil
}

// ListMessages returns messages for a session.
func (db *DB) ListMessages(sessionID string, limit int, offset int) ([]*Message, error) {
	query := `
		SELECT id, session_id, role, sender_id, recipient_id, content, content_type,
		       message_type, status, tokens_used, metadata, created_at, processed_at, error
		FROM session_messages
		WHERE session_id = ?
		ORDER BY created_at DESC LIMIT ? OFFSET ?
	`

	rows, err := db.conn.Query(query, sessionID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query messages: %w", err)
	}
	defer rows.Close()

	var messages []*Message
	for rows.Next() {
		msg := &Message{
			Metadata: make(map[string]interface{}),
		}

		var tokensJSON, metadataJSON []byte
		var role, messageType, status string

		if err := rows.Scan(
			&msg.ID, &msg.SessionID, &role, &msg.SenderID, &msg.RecipientID,
			&msg.Content, &msg.ContentType, &messageType, &status,
			&tokensJSON, &metadataJSON, &msg.CreatedAt, &msg.ProcessedAt, &msg.Error,
		); err != nil {
			return nil, fmt.Errorf("failed to scan message: %w", err)
		}

		msg.Role = MessageRole(role)
		msg.MessageType = MessageType(messageType)
		msg.Status = MessageStatus(status)

		if len(tokensJSON) > 0 && string(tokensJSON) != "null" {
			if err := json.Unmarshal(tokensJSON, &msg.TokensUsed); err != nil {
				return nil, fmt.Errorf("failed to unmarshal tokens: %w", err)
			}
		}

		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &msg.Metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		}

		messages = append(messages, msg)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating messages: %w", err)
	}

	return messages, nil
}

// UpdateMessage updates a message status.
func (db *DB) UpdateMessage(msg *Message) error {
	metadataJSON, err := json.Marshal(msg.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `
		UPDATE session_messages SET
			status = ?, processed_at = ?, error = ?, metadata = ?
		WHERE id = ?
	`

	result, err := db.conn.Exec(query, string(msg.Status), msg.ProcessedAt, msg.Error, string(metadataJSON), msg.ID)
	if err != nil {
		return fmt.Errorf("failed to update message: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("message not found: %s", msg.ID)
	}

	return nil
}

// =============================================================================
// AGENT REFERENCE OPERATIONS
// =============================================================================

// CreateAgent saves a new agent reference to the database.
func (db *DB) CreateAgent(agent *AgentRef) error {
	if err := agent.Validate(); err != nil {
		return fmt.Errorf("invalid agent: %w", err)
	}

	propertiesJSON, err := json.Marshal(agent.Properties)
	if err != nil {
		return fmt.Errorf("failed to marshal properties: %w", err)
	}

	metadataJSON, err := json.Marshal(agent.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `
		INSERT INTO session_agents (
			id, session_id, agent_id, name, model, role, status,
			joined_at, last_activity_at, left_at, message_count, task_count, properties, metadata
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err = db.conn.Exec(
		query,
		agent.ID, agent.SessionID, agent.AgentID, agent.Name, agent.Model, agent.Role,
		string(agent.Status), agent.JoinedAt, agent.LastActivityAt, agent.LeftAt,
		agent.MessageCount, agent.TaskCount, string(propertiesJSON), string(metadataJSON),
	)
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}
	return nil
}

// GetAgent retrieves an agent reference by ID.
func (db *DB) GetAgent(agentID string) (*AgentRef, error) {
	agent := &AgentRef{
		Properties: make(map[string]interface{}),
		Metadata:   make(map[string]interface{}),
	}

	query := `
		SELECT id, session_id, agent_id, name, model, role, status,
		       joined_at, last_activity_at, left_at, message_count, task_count, properties, metadata
		FROM session_agents WHERE id = ?
	`

	var propertiesJSON, metadataJSON []byte
	var status string

	err := db.conn.QueryRow(query, agentID).Scan(
		&agent.ID, &agent.SessionID, &agent.AgentID, &agent.Name, &agent.Model, &agent.Role,
		&status, &agent.JoinedAt, &agent.LastActivityAt, &agent.LeftAt,
		&agent.MessageCount, &agent.TaskCount, &propertiesJSON, &metadataJSON,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("agent not found: %s", agentID)
		}
		return nil, fmt.Errorf("failed to retrieve agent: %w", err)
	}

	agent.Status = AgentStatus(status)

	if len(propertiesJSON) > 0 {
		if err := json.Unmarshal(propertiesJSON, &agent.Properties); err != nil {
			return nil, fmt.Errorf("failed to unmarshal properties: %w", err)
		}
	}

	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &agent.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	return agent, nil
}

// ListAgents returns agents for a session.
func (db *DB) ListAgents(sessionID string) ([]*AgentRef, error) {
	query := `
		SELECT id, session_id, agent_id, name, model, role, status,
		       joined_at, last_activity_at, left_at, message_count, task_count, properties, metadata
		FROM session_agents
		WHERE session_id = ?
		ORDER BY joined_at ASC
	`

	rows, err := db.conn.Query(query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to query agents: %w", err)
	}
	defer rows.Close()

	var agents []*AgentRef
	for rows.Next() {
		agent := &AgentRef{
			Properties: make(map[string]interface{}),
			Metadata:   make(map[string]interface{}),
		}

		var propertiesJSON, metadataJSON []byte
		var status string

		if err := rows.Scan(
			&agent.ID, &agent.SessionID, &agent.AgentID, &agent.Name, &agent.Model, &agent.Role,
			&status, &agent.JoinedAt, &agent.LastActivityAt, &agent.LeftAt,
			&agent.MessageCount, &agent.TaskCount, &propertiesJSON, &metadataJSON,
		); err != nil {
			return nil, fmt.Errorf("failed to scan agent: %w", err)
		}

		agent.Status = AgentStatus(status)

		if len(propertiesJSON) > 0 {
			if err := json.Unmarshal(propertiesJSON, &agent.Properties); err != nil {
				return nil, fmt.Errorf("failed to unmarshal properties: %w", err)
			}
		}

		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &agent.Metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		}

		agents = append(agents, agent)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating agents: %w", err)
	}

	return agents, nil
}

// UpdateAgent updates an agent reference.
func (db *DB) UpdateAgent(agent *AgentRef) error {
	propertiesJSON, err := json.Marshal(agent.Properties)
	if err != nil {
		return fmt.Errorf("failed to marshal properties: %w", err)
	}

	metadataJSON, err := json.Marshal(agent.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `
		UPDATE session_agents SET
			status = ?, last_activity_at = ?, left_at = ?,
			message_count = ?, task_count = ?, properties = ?, metadata = ?
		WHERE id = ?
	`

	result, err := db.conn.Exec(
		query,
		string(agent.Status), agent.LastActivityAt, agent.LeftAt,
		agent.MessageCount, agent.TaskCount, string(propertiesJSON), string(metadataJSON),
		agent.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update agent: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("agent not found: %s", agent.ID)
	}

	return nil
}

// =============================================================================
// TASK REFERENCE OPERATIONS
// =============================================================================

// CreateTask saves a new task reference to the database.
func (db *DB) CreateTask(task *TaskRef) error {
	if err := task.Validate(); err != nil {
		return fmt.Errorf("invalid task: %w", err)
	}

	metricsJSON, err := json.Marshal(task.Metrics)
	if err != nil {
		return fmt.Errorf("failed to marshal metrics: %w", err)
	}

	dependenciesJSON, err := json.Marshal(task.Dependencies)
	if err != nil {
		return fmt.Errorf("failed to marshal dependencies: %w", err)
	}

	tagsJSON, err := json.Marshal(task.Tags)
	if err != nil {
		return fmt.Errorf("failed to marshal tags: %w", err)
	}

	metadataJSON, err := json.Marshal(task.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `
		INSERT INTO session_tasks (
			id, session_id, assigned_agent_id, title, description, status,
			input, result, error, priority, retry_count, max_retries,
			created_at, started_at, completed_at, due_at, metrics, dependencies, tags, metadata
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err = db.conn.Exec(
		query,
		task.ID, task.SessionID, task.AssignedAgentID, task.Title, task.Description, string(task.Status),
		task.Input, task.Result, task.Error, task.Priority, task.RetryCount, task.MaxRetries,
		task.CreatedAt, task.StartedAt, task.CompletedAt, task.DueAt,
		string(metricsJSON), string(dependenciesJSON), string(tagsJSON), string(metadataJSON),
	)
	if err != nil {
		return fmt.Errorf("failed to create task: %w", err)
	}
	return nil
}

// GetTask retrieves a task reference by ID.
func (db *DB) GetTask(taskID string) (*TaskRef, error) {
	task := &TaskRef{
		Dependencies: make([]string, 0),
		Tags:         make([]string, 0),
		Metadata:     make(map[string]interface{}),
	}

	query := `
		SELECT id, session_id, assigned_agent_id, title, description, status,
		       input, result, error, priority, retry_count, max_retries,
		       created_at, started_at, completed_at, due_at, metrics, dependencies, tags, metadata
		FROM session_tasks WHERE id = ?
	`

	var metricsJSON, dependenciesJSON, tagsJSON, metadataJSON []byte
	var status string

	err := db.conn.QueryRow(query, taskID).Scan(
		&task.ID, &task.SessionID, &task.AssignedAgentID, &task.Title, &task.Description, &status,
		&task.Input, &task.Result, &task.Error, &task.Priority, &task.RetryCount, &task.MaxRetries,
		&task.CreatedAt, &task.StartedAt, &task.CompletedAt, &task.DueAt,
		&metricsJSON, &dependenciesJSON, &tagsJSON, &metadataJSON,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("task not found: %s", taskID)
		}
		return nil, fmt.Errorf("failed to retrieve task: %w", err)
	}

	task.Status = TaskStatus(status)

	if len(metricsJSON) > 0 && string(metricsJSON) != "null" {
		if err := json.Unmarshal(metricsJSON, &task.Metrics); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metrics: %w", err)
		}
	}

	if len(dependenciesJSON) > 0 {
		if err := json.Unmarshal(dependenciesJSON, &task.Dependencies); err != nil {
			return nil, fmt.Errorf("failed to unmarshal dependencies: %w", err)
		}
	}

	if len(tagsJSON) > 0 {
		if err := json.Unmarshal(tagsJSON, &task.Tags); err != nil {
			return nil, fmt.Errorf("failed to unmarshal tags: %w", err)
		}
	}

	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &task.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	return task, nil
}

// ListTasks returns tasks for a session.
func (db *DB) ListTasks(sessionID string, taskStatus string) ([]*TaskRef, error) {
	query := `
		SELECT id, session_id, assigned_agent_id, title, description, status,
		       input, result, error, priority, retry_count, max_retries,
		       created_at, started_at, completed_at, due_at, metrics, dependencies, tags, metadata
		FROM session_tasks
		WHERE session_id = ?`

	args := []interface{}{sessionID}
	if taskStatus != "" {
		query += " AND status = ?"
		args = append(args, taskStatus)
	}
	query += " ORDER BY priority DESC, created_at DESC"

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query tasks: %w", err)
	}
	defer rows.Close()

	var tasks []*TaskRef
	for rows.Next() {
		task := &TaskRef{
			Dependencies: make([]string, 0),
			Tags:         make([]string, 0),
			Metadata:     make(map[string]interface{}),
		}

		var metricsJSON, dependenciesJSON, tagsJSON, metadataJSON []byte
		var status string

		if err := rows.Scan(
			&task.ID, &task.SessionID, &task.AssignedAgentID, &task.Title, &task.Description, &status,
			&task.Input, &task.Result, &task.Error, &task.Priority, &task.RetryCount, &task.MaxRetries,
			&task.CreatedAt, &task.StartedAt, &task.CompletedAt, &task.DueAt,
			&metricsJSON, &dependenciesJSON, &tagsJSON, &metadataJSON,
		); err != nil {
			return nil, fmt.Errorf("failed to scan task: %w", err)
		}

		task.Status = TaskStatus(status)

		if len(metricsJSON) > 0 && string(metricsJSON) != "null" {
			if err := json.Unmarshal(metricsJSON, &task.Metrics); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metrics: %w", err)
			}
		}

		if len(dependenciesJSON) > 0 {
			if err := json.Unmarshal(dependenciesJSON, &task.Dependencies); err != nil {
				return nil, fmt.Errorf("failed to unmarshal dependencies: %w", err)
			}
		}

		if len(tagsJSON) > 0 {
			if err := json.Unmarshal(tagsJSON, &task.Tags); err != nil {
				return nil, fmt.Errorf("failed to unmarshal tags: %w", err)
			}
		}

		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &task.Metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		}

		tasks = append(tasks, task)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating tasks: %w", err)
	}

	return tasks, nil
}

// UpdateTask updates a task reference.
func (db *DB) UpdateTask(task *TaskRef) error {
	metricsJSON, err := json.Marshal(task.Metrics)
	if err != nil {
		return fmt.Errorf("failed to marshal metrics: %w", err)
	}

	dependenciesJSON, err := json.Marshal(task.Dependencies)
	if err != nil {
		return fmt.Errorf("failed to marshal dependencies: %w", err)
	}

	tagsJSON, err := json.Marshal(task.Tags)
	if err != nil {
		return fmt.Errorf("failed to marshal tags: %w", err)
	}

	metadataJSON, err := json.Marshal(task.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `
		UPDATE session_tasks SET
			title = ?, description = ?, status = ?, result = ?, error = ?,
			priority = ?, retry_count = ?, started_at = ?, completed_at = ?,
			metrics = ?, dependencies = ?, tags = ?, metadata = ?
		WHERE id = ?
	`

	result, err := db.conn.Exec(
		query,
		task.Title, task.Description, string(task.Status), task.Result, task.Error,
		task.Priority, task.RetryCount, task.StartedAt, task.CompletedAt,
		string(metricsJSON), string(dependenciesJSON), string(tagsJSON), string(metadataJSON),
		task.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update task: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("task not found: %s", task.ID)
	}

	return nil
}

// =============================================================================
// VARIABLE OPERATIONS
// =============================================================================

// SetVariable sets a session variable.
func (db *DB) SetVariable(sessionID, key string, value interface{}) error {
	valueJSON, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %w", err)
	}

	query := `
		INSERT INTO session_variables (id, session_id, key, value, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(session_id, key) DO UPDATE SET
			value = excluded.value,
			updated_at = CURRENT_TIMESTAMP
	`

	_, err = db.conn.Exec(query, generateID(), sessionID, key, string(valueJSON), time.Now(), time.Now())
	if err != nil {
		return fmt.Errorf("failed to set variable: %w", err)
	}
	return nil
}

// GetVariable gets a session variable.
func (db *DB) GetVariable(sessionID, key string) (interface{}, error) {
	query := `SELECT value FROM session_variables WHERE session_id = ? AND key = ?`

	var valueJSON []byte
	err := db.conn.QueryRow(query, sessionID, key).Scan(&valueJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("variable not found: %s", key)
		}
		return nil, fmt.Errorf("failed to retrieve variable: %w", err)
	}

	var value interface{}
	if err := json.Unmarshal(valueJSON, &value); err != nil {
		return nil, fmt.Errorf("failed to unmarshal value: %w", err)
	}

	return value, nil
}

// ListVariables returns all variables for a session.
func (db *DB) ListVariables(sessionID string) (map[string]interface{}, error) {
	query := `SELECT key, value FROM session_variables WHERE session_id = ? ORDER BY key`

	rows, err := db.conn.Query(query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to query variables: %w", err)
	}
	defer rows.Close()

	variables := make(map[string]interface{})
	for rows.Next() {
		var key string
		var valueJSON []byte

		if err := rows.Scan(&key, &valueJSON); err != nil {
			return nil, fmt.Errorf("failed to scan variable: %w", err)
		}

		var value interface{}
		if err := json.Unmarshal(valueJSON, &value); err != nil {
			return nil, fmt.Errorf("failed to unmarshal value: %w", err)
		}

		variables[key] = value
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating variables: %w", err)
	}

	return variables, nil
}
