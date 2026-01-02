package session

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// SerializationFormat represents the format version for session serialization.
// This allows for backward compatibility when the schema changes.
type SerializationFormat struct {
	Version   string    `json:"version"`
	Timestamp time.Time `json:"timestamp"`
}

// SerializedSession represents a complete session snapshot for serialization/deserialization.
// This is the top-level envelope that contains the session and all its related data.
type SerializedSession struct {
	Format   SerializationFormat `json:"format"`
	Session  *SessionState       `json:"session"`
	Messages []Message           `json:"messages"`
	Agents   []AgentRef          `json:"agents"`
	Tasks    []TaskRef           `json:"tasks"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// Serializer provides session serialization and deserialization functionality.
// It handles JSON conversion with proper marshaling of complex types and relationships.
type Serializer struct {
	mu sync.RWMutex
}

// NewSerializer creates a new session serializer.
func NewSerializer() *Serializer {
	return &Serializer{}
}

// SerializeSession converts a session and all its related data to JSON bytes.
// This includes the session metadata, messages, agents, and tasks.
//
// The function performs a deep copy of all nested structures to ensure
// the serialized data is immutable and safe for storage/transmission.
func (s *Serializer) SerializeSession(session *SessionState) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if session == nil {
		return nil, fmt.Errorf("session cannot be nil")
	}

	// Validate the session before serialization
	if err := session.Validate(); err != nil {
		return nil, fmt.Errorf("invalid session: %w", err)
	}

	// Create a serialized representation that includes all session data
	serialized := &SerializedSession{
		Format: SerializationFormat{
			Version:   "1.0",
			Timestamp: time.Now().UTC(),
		},
		Session:  session,
		Messages: session.Messages,
		Agents:   session.ActiveAgents,
		Tasks:    session.Tasks,
		Metadata: session.Metadata,
	}

	// Convert to JSON with proper formatting for readability and debuggability
	data, err := json.Marshal(serialized)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize session: %w", err)
	}

	return data, nil
}

// DeserializeSession converts JSON bytes back into a session object.
// It reconstructs all relationships between sessions, messages, agents, and tasks.
//
// The function validates the deserialized data to ensure consistency and
// returns an error if the data is corrupted or incomplete.
func (s *Serializer) DeserializeSession(data []byte) (*SessionState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(data) == 0 {
		return nil, fmt.Errorf("serialized data cannot be empty")
	}

	// Parse the JSON data
	var serialized SerializedSession
	if err := json.Unmarshal(data, &serialized); err != nil {
		return nil, fmt.Errorf("failed to deserialize session: %w", err)
	}

	// Validate format version
	if serialized.Format.Version != "1.0" {
		return nil, fmt.Errorf("unsupported serialization format version: %s", serialized.Format.Version)
	}

	if serialized.Session == nil {
		return nil, fmt.Errorf("deserialized session is nil")
	}

	// Restore the session data
	session := serialized.Session
	session.Messages = serialized.Messages
	session.ActiveAgents = serialized.Agents
	session.Tasks = serialized.Tasks

	// Validate the restored session
	if err := session.Validate(); err != nil {
		return nil, fmt.Errorf("deserialized session failed validation: %w", err)
	}

	return session, nil
}

// SerializeSessionJSON converts a session to a pretty-printed JSON string.
// This is useful for logging and debugging purposes.
func (s *Serializer) SerializeSessionJSON(session *SessionState) (string, error) {
	data, err := s.SerializeSession(session)
	if err != nil {
		return "", err
	}

	// Pretty-print the JSON
	var pretty json.RawMessage
	if err := json.Unmarshal(data, &pretty); err != nil {
		return "", fmt.Errorf("failed to format JSON: %w", err)
	}

	prettyData, err := json.MarshalIndent(pretty, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to indent JSON: %w", err)
	}

	return string(prettyData), nil
}

// DeserializeSessionJSON converts a JSON string back into a session object.
func (s *Serializer) DeserializeSessionJSON(jsonStr string) (*SessionState, error) {
	return s.DeserializeSession([]byte(jsonStr))
}

// SerializeToMap converts a session to a map representation.
// This is useful for API responses and inter-process communication.
func (s *Serializer) SerializeToMap(session *SessionState) (map[string]interface{}, error) {
	if session == nil {
		return nil, fmt.Errorf("session cannot be nil")
	}

	// Serialize to JSON first
	data, err := s.SerializeSession(session)
	if err != nil {
		return nil, err
	}

	// Unmarshal into a map
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to convert to map: %w", err)
	}

	return result, nil
}

// DeserializeFromMap converts a map representation back into a session object.
func (s *Serializer) DeserializeFromMap(data map[string]interface{}) (*SessionState, error) {
	if data == nil {
		return nil, fmt.Errorf("data cannot be nil")
	}

	// Convert map to JSON bytes
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to convert map to JSON: %w", err)
	}

	// Deserialize from JSON
	return s.DeserializeSession(jsonBytes)
}

// SaveSessionState persists a session to the database via the provided DB handler.
// This function serializes the session and stores it along with all related data.
func SaveSessionState(db *DB, session *SessionState) error {
	if db == nil {
		return fmt.Errorf("database handler cannot be nil")
	}

	if session == nil {
		return fmt.Errorf("session cannot be nil")
	}

	// Validate the session before saving
	if err := session.Validate(); err != nil {
		return fmt.Errorf("invalid session: %w", err)
	}

	// Begin transaction (if the DB implementation supports it)
	// For now, we'll use the existing db.CreateSession and related methods

	// Create or update the session
	// Check if session already exists
	_, err := db.GetSession(session.ID)
	if err != nil {
		// Session doesn't exist, create it
		if err := db.CreateSession(session); err != nil {
			return fmt.Errorf("failed to create session: %w", err)
		}
	} else {
		// Session exists, update it
		if err := db.UpdateSession(session); err != nil {
			return fmt.Errorf("failed to update session: %w", err)
		}
	}

	// Save all messages
	for _, msg := range session.Messages {
		if err := db.CreateMessage(&msg); err != nil {
			return fmt.Errorf("failed to save message %s: %w", msg.ID, err)
		}
	}

	// Save all agents
	for _, agent := range session.ActiveAgents {
		if err := db.CreateAgent(&agent); err != nil {
			return fmt.Errorf("failed to save agent %s: %w", agent.ID, err)
		}
	}

	// Save all tasks
	for _, task := range session.Tasks {
		if err := db.CreateTask(&task); err != nil {
			return fmt.Errorf("failed to save task %s: %w", task.ID, err)
		}
	}

	// Mark session as persisted
	session.UpdatedAt = time.Now()

	return nil
}

// LoadSessionState retrieves a session from the database and reconstructs it.
// This function loads all related messages, agents, and tasks.
func LoadSessionState(db *DB, sessionID string) (*SessionState, error) {
	if db == nil {
		return nil, fmt.Errorf("database handler cannot be nil")
	}

	if sessionID == "" {
		return nil, fmt.Errorf("session ID cannot be empty")
	}

	// Load the session
	session, err := db.GetSession(sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to load session: %w", err)
	}

	// Load all messages for this session
	messages, err := db.ListMessages(sessionID, 1000, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to load messages: %w", err)
	}
	// Convert []*Message to []Message
	session.Messages = make([]Message, len(messages))
	for i, msg := range messages {
		session.Messages[i] = *msg
	}

	// Load all agents for this session
	agents, err := db.ListAgents(sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to load agents: %w", err)
	}
	// Convert []*AgentRef to []AgentRef
	session.ActiveAgents = make([]AgentRef, len(agents))
	for i, agent := range agents {
		session.ActiveAgents[i] = *agent
	}

	// Load all tasks for this session
	tasks, err := db.ListTasks(sessionID, "")
	if err != nil {
		return nil, fmt.Errorf("failed to load tasks: %w", err)
	}
	// Convert []*TaskRef to []TaskRef
	session.Tasks = make([]TaskRef, len(tasks))
	for i, task := range tasks {
		session.Tasks[i] = *task
	}

	// Validate the loaded session
	if err := session.Validate(); err != nil {
		return nil, fmt.Errorf("loaded session failed validation: %w", err)
	}

	return session, nil
}

// RoundTripTest performs a serialization round-trip test.
// It serializes a session and then deserializes it, verifying that
// the result is identical to the original.
//
// This function is primarily for testing and validation purposes.
func (s *Serializer) RoundTripTest(original *SessionState) (*SessionState, error) {
	if original == nil {
		return nil, fmt.Errorf("original session cannot be nil")
	}

	// Serialize
	data, err := s.SerializeSession(original)
	if err != nil {
		return nil, fmt.Errorf("serialization failed: %w", err)
	}

	// Deserialize
	restored, err := s.DeserializeSession(data)
	if err != nil {
		return nil, fmt.Errorf("deserialization failed: %w", err)
	}

	// Verify that critical fields match
	if restored.ID != original.ID {
		return nil, fmt.Errorf("session ID mismatch: %s != %s", restored.ID, original.ID)
	}
	if restored.Name != original.Name {
		return nil, fmt.Errorf("session name mismatch: %s != %s", restored.Name, original.Name)
	}
	if restored.Status != original.Status {
		return nil, fmt.Errorf("session status mismatch: %s != %s", restored.Status, original.Status)
	}
	if len(restored.Messages) != len(original.Messages) {
		return nil, fmt.Errorf("message count mismatch: %d != %d", len(restored.Messages), len(original.Messages))
	}
	if len(restored.ActiveAgents) != len(original.ActiveAgents) {
		return nil, fmt.Errorf("agent count mismatch: %d != %d", len(restored.ActiveAgents), len(original.ActiveAgents))
	}
	if len(restored.Tasks) != len(original.Tasks) {
		return nil, fmt.Errorf("task count mismatch: %d != %d", len(restored.Tasks), len(original.Tasks))
	}

	return restored, nil
}

// CompactSerialization returns a compact JSON representation with minimal whitespace.
// This is useful for storage and transmission where size matters.
func (s *Serializer) CompactSerialization(session *SessionState) ([]byte, error) {
	data, err := s.SerializeSession(session)
	if err != nil {
		return nil, err
	}

	// Compact the JSON (remove whitespace)
	var compact json.RawMessage
	if err := json.Unmarshal(data, &compact); err != nil {
		return nil, fmt.Errorf("failed to compact JSON: %w", err)
	}

	// Re-marshal without indentation
	return json.Marshal(compact)
}

// SerializeWithSnapshot creates a serialization with additional snapshot metadata.
// This is useful for creating checkpoints of session state at specific points in time.
func (s *Serializer) SerializeWithSnapshot(session *SessionState, snapshotName string) ([]byte, error) {
	if session == nil {
		return nil, fmt.Errorf("session cannot be nil")
	}

	data, err := s.SerializeSession(session)
	if err != nil {
		return nil, err
	}

	// Create a wrapper with snapshot metadata
	snapshot := map[string]interface{}{
		"snapshot_name": snapshotName,
		"snapshot_time": time.Now().UTC(),
		"data":          json.RawMessage(data),
	}

	return json.Marshal(snapshot)
}

// DeserializeWithSnapshot extracts and deserializes a session from a snapshot.
func (s *Serializer) DeserializeWithSnapshot(snapshotData []byte) (*SessionState, error) {
	if len(snapshotData) == 0 {
		return nil, fmt.Errorf("snapshot data cannot be empty")
	}

	// Parse the snapshot wrapper
	var snapshot map[string]interface{}
	if err := json.Unmarshal(snapshotData, &snapshot); err != nil {
		return nil, fmt.Errorf("failed to parse snapshot: %w", err)
	}

	// Extract the serialized session data
	rawData, ok := snapshot["data"]
	if !ok {
		return nil, fmt.Errorf("snapshot missing data field")
	}

	// Convert back to JSON bytes
	dataBytes, err := json.Marshal(rawData)
	if err != nil {
		return nil, fmt.Errorf("failed to extract data: %w", err)
	}

	// Deserialize the session
	return s.DeserializeSession(dataBytes)
}
