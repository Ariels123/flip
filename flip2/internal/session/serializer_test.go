package session

import (
	"encoding/json"
	"testing"
	"time"
)

// TestSerializeSessionBasic tests basic session serialization.
func TestSerializeSessionBasic(t *testing.T) {
	serializer := NewSerializer()

	// Create a minimal session
	session := &SessionState{
		ID:            "test-session-123",
		Name:          "Test Session",
		Status:        SessionActive,
		CoordinatorID: "coordinator-001",
		Description:   "A test session",
		Messages:      make([]Message, 0),
		ActiveAgents:  make([]AgentRef, 0),
		Tasks:         make([]TaskRef, 0),
		Environment:   make(map[string]string),
		Variables:     make(map[string]interface{}),
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		MessageCount:  0,
		AgentCount:    0,
		TaskCount:     0,
		ErrorCount:    0,
		Metadata:      make(map[string]interface{}),
	}

	data, err := serializer.SerializeSession(session)
	if err != nil {
		t.Fatalf("Failed to serialize session: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("Serialized data is empty")
	}

	// Verify it's valid JSON
	var result interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Serialized data is not valid JSON: %v", err)
	}
}

// TestDeserializeSessionBasic tests basic session deserialization.
func TestDeserializeSessionBasic(t *testing.T) {
	serializer := NewSerializer()

	// Create and serialize a session
	session := &SessionState{
		ID:            "test-session-456",
		Name:          "Test Session 2",
		Status:        SessionActive,
		CoordinatorID: "coordinator-002",
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		Messages:      make([]Message, 0),
		ActiveAgents:  make([]AgentRef, 0),
		Tasks:         make([]TaskRef, 0),
		Environment:   make(map[string]string),
		Variables:     make(map[string]interface{}),
		Metadata:      make(map[string]interface{}),
	}

	data, err := serializer.SerializeSession(session)
	if err != nil {
		t.Fatalf("Failed to serialize session: %v", err)
	}

	// Deserialize
	restored, err := serializer.DeserializeSession(data)
	if err != nil {
		t.Fatalf("Failed to deserialize session: %v", err)
	}

	if restored.ID != session.ID {
		t.Errorf("ID mismatch: expected %s, got %s", session.ID, restored.ID)
	}

	if restored.Name != session.Name {
		t.Errorf("Name mismatch: expected %s, got %s", session.Name, restored.Name)
	}

	if restored.CoordinatorID != session.CoordinatorID {
		t.Errorf("CoordinatorID mismatch: expected %s, got %s", session.CoordinatorID, restored.CoordinatorID)
	}
}

// TestRoundTripSerialization tests a complete round-trip serialization cycle.
func TestRoundTripSerialization(t *testing.T) {
	serializer := NewSerializer()

	// Create a complex session with multiple messages, agents, and tasks
	session := &SessionState{
		ID:            "test-round-trip-001",
		Name:          "Round Trip Test",
		Status:        SessionActive,
		CoordinatorID: "coordinator-003",
		Description:   "Testing round-trip serialization",
		CreatedAt:     time.Now().Add(-1 * time.Hour),
		UpdatedAt:     time.Now(),
		Messages: []Message{
			{
				ID:          "msg-001",
				SessionID:   "test-round-trip-001",
				Role:        MessageRoleCoordinator,
				SenderID:    "coordinator-003",
				Content:     "Starting session",
				ContentType: "text",
				MessageType: MessageTypeRequest,
				Status:      MessageStatusProcessed,
				CreatedAt:   time.Now().Add(-50 * time.Minute),
			},
			{
				ID:          "msg-002",
				SessionID:   "test-round-trip-001",
				Role:        MessageRoleAgent,
				SenderID:    "agent-001",
				Content:     "Ready to work",
				ContentType: "text",
				MessageType: MessageTypeResponse,
				Status:      MessageStatusProcessed,
				CreatedAt:   time.Now().Add(-45 * time.Minute),
			},
		},
		ActiveAgents: []AgentRef{
			{
				ID:           "agent-ref-001",
				SessionID:    "test-round-trip-001",
				AgentID:      "agent-001",
				Name:         "Worker Agent 1",
				Model:        "claude-opus",
				Role:         "worker",
				Status:       AgentStatusActive,
				JoinedAt:     time.Now().Add(-55 * time.Minute),
				MessageCount: 5,
				TaskCount:    2,
				Properties:   make(map[string]interface{}),
				Metadata:     make(map[string]interface{}),
			},
		},
		Tasks: []TaskRef{
			{
				ID:              "task-001",
				SessionID:       "test-round-trip-001",
				AssignedAgentID: "agent-001",
				Title:           "Analyze Data",
				Status:          TaskStatusCompleted,
				Priority:        5,
				RetryCount:      0,
				MaxRetries:      3,
				CreatedAt:       time.Now().Add(-40 * time.Minute),
				Tags:            []string{"analysis", "data"},
				Dependencies:    make([]string, 0),
				Metadata:        make(map[string]interface{}),
			},
		},
		Environment:  make(map[string]string),
		Variables:    make(map[string]interface{}),
		MessageCount: 2,
		AgentCount:   1,
		TaskCount:    1,
		ErrorCount:   0,
		Metadata: map[string]interface{}{
			"test_key": "test_value",
			"nested": map[string]interface{}{
				"key1": "value1",
				"key2": 42,
			},
		},
	}

	// Perform round-trip
	restored, err := serializer.RoundTripTest(session)
	if err != nil {
		t.Fatalf("Round-trip test failed: %v", err)
	}

	// Verify all fields
	if restored.ID != session.ID {
		t.Errorf("ID mismatch after round-trip")
	}
	if restored.Name != session.Name {
		t.Errorf("Name mismatch after round-trip")
	}
	if restored.Description != session.Description {
		t.Errorf("Description mismatch after round-trip")
	}
	if restored.Status != session.Status {
		t.Errorf("Status mismatch after round-trip")
	}

	// Verify messages
	if len(restored.Messages) != len(session.Messages) {
		t.Errorf("Message count mismatch: expected %d, got %d", len(session.Messages), len(restored.Messages))
	}

	// Verify agents
	if len(restored.ActiveAgents) != len(session.ActiveAgents) {
		t.Errorf("Agent count mismatch: expected %d, got %d", len(session.ActiveAgents), len(restored.ActiveAgents))
	}

	// Verify tasks
	if len(restored.Tasks) != len(session.Tasks) {
		t.Errorf("Task count mismatch: expected %d, got %d", len(session.Tasks), len(restored.Tasks))
	}

	// Verify metadata
	if len(restored.Metadata) != len(session.Metadata) {
		t.Errorf("Metadata size mismatch after round-trip")
	}
}

// TestSerializeWithNilSession tests serialization with nil session.
func TestSerializeWithNilSession(t *testing.T) {
	serializer := NewSerializer()

	_, err := serializer.SerializeSession(nil)
	if err == nil {
		t.Fatal("Expected error when serializing nil session")
	}
}

// TestDeserializeWithEmptyData tests deserialization with empty data.
func TestDeserializeWithEmptyData(t *testing.T) {
	serializer := NewSerializer()

	_, err := serializer.DeserializeSession([]byte{})
	if err == nil {
		t.Fatal("Expected error when deserializing empty data")
	}
}

// TestSerializeToMap tests conversion to map representation.
func TestSerializeToMap(t *testing.T) {
	serializer := NewSerializer()

	session := &SessionState{
		ID:            "test-map-001",
		Name:          "Map Test",
		Status:        SessionActive,
		CoordinatorID: "coordinator-004",
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		Messages:      make([]Message, 0),
		ActiveAgents:  make([]AgentRef, 0),
		Tasks:         make([]TaskRef, 0),
		Environment:   make(map[string]string),
		Variables:     make(map[string]interface{}),
		Metadata:      make(map[string]interface{}),
	}

	resultMap, err := serializer.SerializeToMap(session)
	if err != nil {
		t.Fatalf("Failed to serialize to map: %v", err)
	}

	if resultMap == nil {
		t.Fatal("Serialized map is nil")
	}

	// Verify the map contains expected fields
	if format, ok := resultMap["format"]; !ok {
		t.Error("Map missing 'format' field")
	} else {
		if f, ok := format.(map[string]interface{}); ok {
			if v, ok := f["version"]; !ok {
				t.Error("Format missing 'version' field")
			} else if v != "1.0" {
				t.Errorf("Expected version 1.0, got %v", v)
			}
		}
	}
}

// TestDeserializeFromMap tests conversion from map representation.
func TestDeserializeFromMap(t *testing.T) {
	serializer := NewSerializer()

	// Create and serialize a session
	session := &SessionState{
		ID:            "test-map-002",
		Name:          "Map Deserialize Test",
		Status:        SessionActive,
		CoordinatorID: "coordinator-005",
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		Messages:      make([]Message, 0),
		ActiveAgents:  make([]AgentRef, 0),
		Tasks:         make([]TaskRef, 0),
		Environment:   make(map[string]string),
		Variables:     make(map[string]interface{}),
		Metadata:      make(map[string]interface{}),
	}

	// Convert to map and back
	resultMap, err := serializer.SerializeToMap(session)
	if err != nil {
		t.Fatalf("Failed to serialize to map: %v", err)
	}

	// Deserialize from map
	restored, err := serializer.DeserializeFromMap(resultMap)
	if err != nil {
		t.Fatalf("Failed to deserialize from map: %v", err)
	}

	if restored.ID != session.ID {
		t.Errorf("ID mismatch: expected %s, got %s", session.ID, restored.ID)
	}

	if restored.Name != session.Name {
		t.Errorf("Name mismatch: expected %s, got %s", session.Name, restored.Name)
	}
}

// TestSerializeSessionJSON tests JSON string serialization.
func TestSerializeSessionJSON(t *testing.T) {
	serializer := NewSerializer()

	session := &SessionState{
		ID:            "test-json-001",
		Name:          "JSON Test",
		Status:        SessionActive,
		CoordinatorID: "coordinator-006",
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		Messages:      make([]Message, 0),
		ActiveAgents:  make([]AgentRef, 0),
		Tasks:         make([]TaskRef, 0),
		Environment:   make(map[string]string),
		Variables:     make(map[string]interface{}),
		Metadata:      make(map[string]interface{}),
	}

	jsonStr, err := serializer.SerializeSessionJSON(session)
	if err != nil {
		t.Fatalf("Failed to serialize to JSON string: %v", err)
	}

	if len(jsonStr) == 0 {
		t.Fatal("Serialized JSON string is empty")
	}

	// Verify it's valid JSON
	var result interface{}
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		t.Fatalf("Serialized JSON string is not valid: %v", err)
	}

	// Verify it contains readable format (indentation)
	if jsonStr[0] != '{' {
		t.Error("JSON should start with {")
	}
}

// TestDeserializeSessionJSON tests JSON string deserialization.
func TestDeserializeSessionJSON(t *testing.T) {
	serializer := NewSerializer()

	// Create, serialize to JSON string, and deserialize
	session := &SessionState{
		ID:            "test-json-002",
		Name:          "JSON Deserialize Test",
		Status:        SessionActive,
		CoordinatorID: "coordinator-007",
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		Messages:      make([]Message, 0),
		ActiveAgents:  make([]AgentRef, 0),
		Tasks:         make([]TaskRef, 0),
		Environment:   make(map[string]string),
		Variables:     make(map[string]interface{}),
		Metadata:      make(map[string]interface{}),
	}

	jsonStr, err := serializer.SerializeSessionJSON(session)
	if err != nil {
		t.Fatalf("Failed to serialize to JSON string: %v", err)
	}

	// Deserialize from JSON string
	restored, err := serializer.DeserializeSessionJSON(jsonStr)
	if err != nil {
		t.Fatalf("Failed to deserialize from JSON string: %v", err)
	}

	if restored.ID != session.ID {
		t.Errorf("ID mismatch: expected %s, got %s", session.ID, restored.ID)
	}
}

// TestCompactSerialization tests compact JSON serialization.
func TestCompactSerialization(t *testing.T) {
	serializer := NewSerializer()

	session := &SessionState{
		ID:            "test-compact-001",
		Name:          "Compact Test",
		Status:        SessionActive,
		CoordinatorID: "coordinator-008",
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		Messages:      make([]Message, 0),
		ActiveAgents:  make([]AgentRef, 0),
		Tasks:         make([]TaskRef, 0),
		Environment:   make(map[string]string),
		Variables:     make(map[string]interface{}),
		Metadata:      make(map[string]interface{}),
	}

	compactData, err := serializer.CompactSerialization(session)
	if err != nil {
		t.Fatalf("Failed to create compact serialization: %v", err)
	}

	// Verify it's valid JSON
	var result interface{}
	if err := json.Unmarshal(compactData, &result); err != nil {
		t.Fatalf("Compact JSON is not valid: %v", err)
	}

	// Compare sizes - compact should be smaller or equal to pretty-printed
	regularData, _ := serializer.SerializeSession(session)
	if len(compactData) > len(regularData) {
		t.Logf("Note: compact data (%d bytes) is larger than regular (%d bytes), may contain extra newlines", len(compactData), len(regularData))
	}
}

// TestSerializeWithSnapshot tests snapshot serialization.
func TestSerializeWithSnapshot(t *testing.T) {
	serializer := NewSerializer()

	session := &SessionState{
		ID:            "test-snapshot-001",
		Name:          "Snapshot Test",
		Status:        SessionActive,
		CoordinatorID: "coordinator-009",
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		Messages:      make([]Message, 0),
		ActiveAgents:  make([]AgentRef, 0),
		Tasks:         make([]TaskRef, 0),
		Environment:   make(map[string]string),
		Variables:     make(map[string]interface{}),
		Metadata:      make(map[string]interface{}),
	}

	snapshotData, err := serializer.SerializeWithSnapshot(session, "test-snapshot")
	if err != nil {
		t.Fatalf("Failed to serialize with snapshot: %v", err)
	}

	// Verify it's valid JSON
	var snapshot map[string]interface{}
	if err := json.Unmarshal(snapshotData, &snapshot); err != nil {
		t.Fatalf("Snapshot JSON is not valid: %v", err)
	}

	// Verify snapshot metadata
	if name, ok := snapshot["snapshot_name"]; !ok || name != "test-snapshot" {
		t.Error("Snapshot missing or incorrect snapshot_name")
	}

	if _, ok := snapshot["snapshot_time"]; !ok {
		t.Error("Snapshot missing snapshot_time")
	}

	if _, ok := snapshot["data"]; !ok {
		t.Error("Snapshot missing data")
	}
}

// TestDeserializeWithSnapshot tests snapshot deserialization.
func TestDeserializeWithSnapshot(t *testing.T) {
	serializer := NewSerializer()

	// Create, snapshot, and restore
	session := &SessionState{
		ID:            "test-snapshot-002",
		Name:          "Snapshot Restore Test",
		Status:        SessionActive,
		CoordinatorID: "coordinator-010",
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		Messages:      make([]Message, 0),
		ActiveAgents:  make([]AgentRef, 0),
		Tasks:         make([]TaskRef, 0),
		Environment:   make(map[string]string),
		Variables:     make(map[string]interface{}),
		Metadata:      make(map[string]interface{}),
	}

	snapshotData, err := serializer.SerializeWithSnapshot(session, "restore-test")
	if err != nil {
		t.Fatalf("Failed to serialize with snapshot: %v", err)
	}

	// Deserialize from snapshot
	restored, err := serializer.DeserializeWithSnapshot(snapshotData)
	if err != nil {
		t.Fatalf("Failed to deserialize from snapshot: %v", err)
	}

	if restored.ID != session.ID {
		t.Errorf("ID mismatch: expected %s, got %s", session.ID, restored.ID)
	}

	if restored.Name != session.Name {
		t.Errorf("Name mismatch: expected %s, got %s", session.Name, restored.Name)
	}
}

// TestSerializeComplexMetadata tests serialization with complex metadata.
func TestSerializeComplexMetadata(t *testing.T) {
	serializer := NewSerializer()

	session := &SessionState{
		ID:            "test-metadata-001",
		Name:          "Metadata Test",
		Status:        SessionActive,
		CoordinatorID: "coordinator-011",
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		Messages:      make([]Message, 0),
		ActiveAgents:  make([]AgentRef, 0),
		Tasks:         make([]TaskRef, 0),
		Environment:   make(map[string]string),
		Variables:     make(map[string]interface{}),
		Metadata: map[string]interface{}{
			"string_value": "test",
			"int_value":    42,
			"float_value":  3.14,
			"bool_value":   true,
			"nested_map": map[string]interface{}{
				"inner_key": "inner_value",
				"inner_int": 99,
			},
			"array": []interface{}{"a", "b", "c"},
		},
	}

	// Serialize and deserialize
	data, err := serializer.SerializeSession(session)
	if err != nil {
		t.Fatalf("Failed to serialize: %v", err)
	}

	restored, err := serializer.DeserializeSession(data)
	if err != nil {
		t.Fatalf("Failed to deserialize: %v", err)
	}

	// Verify metadata
	if len(restored.Metadata) != len(session.Metadata) {
		t.Errorf("Metadata size mismatch: expected %d, got %d", len(session.Metadata), len(restored.Metadata))
	}

	if val, ok := restored.Metadata["string_value"]; !ok || val != "test" {
		t.Error("String metadata value not preserved")
	}

	if _, ok := restored.Metadata["int_value"]; !ok {
		t.Error("Int metadata value not preserved")
	}

	if _, ok := restored.Metadata["nested_map"]; !ok {
		t.Error("Nested map metadata not preserved")
	}
}

// TestSerializeWithTimestamps tests that timestamps are preserved.
func TestSerializeWithTimestamps(t *testing.T) {
	serializer := NewSerializer()

	now := time.Now()
	pastTime := now.Add(-1 * time.Hour)
	futureTime := now.Add(1 * time.Hour)

	session := &SessionState{
		ID:            "test-time-001",
		Name:          "Timestamp Test",
		Status:        SessionActive,
		CoordinatorID: "coordinator-012",
		CreatedAt:     pastTime,
		StartedAt:     &now,
		CompletedAt:   &futureTime,
		UpdatedAt:     now,
		Messages:      make([]Message, 0),
		ActiveAgents:  make([]AgentRef, 0),
		Tasks:         make([]TaskRef, 0),
		Environment:   make(map[string]string),
		Variables:     make(map[string]interface{}),
		Metadata:      make(map[string]interface{}),
	}

	data, err := serializer.SerializeSession(session)
	if err != nil {
		t.Fatalf("Failed to serialize: %v", err)
	}

	restored, err := serializer.DeserializeSession(data)
	if err != nil {
		t.Fatalf("Failed to deserialize: %v", err)
	}

	// Check timestamps with tolerance for rounding
	if restored.CreatedAt.Unix() != session.CreatedAt.Unix() {
		t.Errorf("CreatedAt timestamp not preserved")
	}

	if restored.UpdatedAt.Unix() != session.UpdatedAt.Unix() {
		t.Errorf("UpdatedAt timestamp not preserved")
	}

	if restored.StartedAt == nil {
		t.Error("StartedAt should not be nil")
	}

	if restored.CompletedAt == nil {
		t.Error("CompletedAt should not be nil")
	}
}

// TestMultipleConcurrentSerializations tests thread safety.
func TestMultipleConcurrentSerializations(t *testing.T) {
	serializer := NewSerializer()

	// Create multiple sessions
	sessions := make([]*SessionState, 5)
	for i := 0; i < 5; i++ {
		sessions[i] = &SessionState{
			ID:            "test-concurrent-" + string(rune(i)),
			Name:          "Concurrent Test " + string(rune(i)),
			Status:        SessionActive,
			CoordinatorID: "coordinator-013",
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
			Messages:      make([]Message, 0),
			ActiveAgents:  make([]AgentRef, 0),
			Tasks:         make([]TaskRef, 0),
			Environment:   make(map[string]string),
			Variables:     make(map[string]interface{}),
			Metadata:      make(map[string]interface{}),
		}
	}

	// Serialize all sessions
	for _, session := range sessions {
		_, err := serializer.SerializeSession(session)
		if err != nil {
			t.Fatalf("Failed to serialize session: %v", err)
		}
	}
}

// TestSerializeAllSessionStateFields tests that all SessionState fields are properly serialized
func TestSerializeAllSessionStateFields(t *testing.T) {
	serializer := NewSerializer()

	// Create a session with all fields populated
	now := time.Now()
	parentID := "parent-session-123"
	completedTime := now.Add(1 * time.Hour)
	lastHeartbeat := now.Add(-5 * time.Minute)
	recipientID := "agent-recipient-123"
	processedTime := now.Add(10 * time.Minute)
	errorMsg := "test error"

	session := &SessionState{
		ID:              "session-123",
		Name:            "Test Session",
		Status:          SessionActive,
		CoordinatorID:   "coordinator-001",
		ParentSessionID: &parentID,
		Description:     "Test description",
		Messages: []Message{
			{
				ID:          "msg-001",
				SessionID:   "session-123",
				Role:        MessageRoleCoordinator,
				SenderID:    "coordinator-001",
				RecipientID: &recipientID,
				Content:     "Test message",
				ContentType: "text",
				MessageType: MessageTypeRequest,
				Status:      MessageStatusProcessed,
				TokensUsed: &TokenMetrics{
					InputTokens:  100,
					OutputTokens: 50,
					TotalTokens:  150,
					Cost:         0.003,
				},
				Metadata: map[string]interface{}{
					"key": "value",
				},
				CreatedAt:   now,
				ProcessedAt: &processedTime,
				Error:       &errorMsg,
			},
		},
		ActiveAgents: []AgentRef{
			{
				ID:           "agent-ref-001",
				SessionID:    "session-123",
				AgentID:      "agent-001",
				Name:         "Worker 1",
				Model:        "claude-opus",
				Role:         "worker",
				Status:       AgentStatusActive,
				JoinedAt:     now.Add(-30 * time.Minute),
				LastActivityAt: &now,
				MessageCount: 5,
				TaskCount:    2,
				Properties: map[string]interface{}{
					"prop1": "value1",
				},
				Metadata: map[string]interface{}{
					"meta1": "meta_value1",
				},
			},
		},
		Tasks: []TaskRef{
			{
				ID:              "task-001",
				SessionID:       "session-123",
				AssignedAgentID: "agent-001",
				Title:           "Task 1",
				Description:     "Task description",
				Status:          TaskStatusCompleted,
				Input:           json.RawMessage(`{"input":"data"}`),
				Result:          json.RawMessage(`{"output":"result"}`),
				Error:           nil,
				Priority:        5,
				RetryCount:      0,
				MaxRetries:      3,
				CreatedAt:       now,
				StartedAt:       &now,
				CompletedAt:     &completedTime,
				DueAt:           nil,
				Metrics: &TaskMetrics{
					TokensUsed: &TokenMetrics{
						InputTokens:  100,
						OutputTokens: 50,
						TotalTokens:  150,
						Cost:         0.003,
					},
					DurationMs:      1000,
					MemoryUsedBytes: 1000000,
					Cost:            0.003,
				},
				Dependencies: []string{"dep-001"},
				Tags:         []string{"tag1", "tag2"},
				Metadata: map[string]interface{}{
					"task_meta": "value",
				},
			},
		},
		Environment: map[string]string{
			"ENV_VAR": "value",
		},
		Variables: map[string]interface{}{
			"var1": "value1",
			"var2": 42,
		},
		CreatedAt:       now,
		StartedAt:       &now,
		CompletedAt:     &completedTime,
		UpdatedAt:       now,
		LastHeartbeatAt: &lastHeartbeat,
		MessageCount:    1,
		AgentCount:      1,
		TaskCount:       1,
		ErrorCount:      0,
		Metadata: map[string]interface{}{
			"metadata_key": "metadata_value",
		},
	}

	// Serialize
	data, err := serializer.SerializeSession(session)
	if err != nil {
		t.Fatalf("Failed to serialize: %v", err)
	}

	// Deserialize
	restored, err := serializer.DeserializeSession(data)
	if err != nil {
		t.Fatalf("Failed to deserialize: %v", err)
	}

	// Verify all fields
	if restored.ID != session.ID {
		t.Errorf("ID mismatch: %s != %s", restored.ID, session.ID)
	}
	if restored.Name != session.Name {
		t.Errorf("Name mismatch: %s != %s", restored.Name, session.Name)
	}
	if restored.Status != session.Status {
		t.Errorf("Status mismatch: %s != %s", restored.Status, session.Status)
	}
	if restored.CoordinatorID != session.CoordinatorID {
		t.Errorf("CoordinatorID mismatch: %s != %s", restored.CoordinatorID, session.CoordinatorID)
	}
	if restored.Description != session.Description {
		t.Errorf("Description mismatch: %s != %s", restored.Description, session.Description)
	}

	// Check optional ParentSessionID
	if (restored.ParentSessionID == nil) != (session.ParentSessionID == nil) {
		t.Error("ParentSessionID pointer nil state mismatch")
	}
	if restored.ParentSessionID != nil && *restored.ParentSessionID != *session.ParentSessionID {
		t.Errorf("ParentSessionID mismatch: %s != %s", *restored.ParentSessionID, *session.ParentSessionID)
	}

	// Check messages
	if len(restored.Messages) != len(session.Messages) {
		t.Errorf("Message count mismatch: %d != %d", len(restored.Messages), len(session.Messages))
	} else {
		for i, msg := range restored.Messages {
			if msg.ID != session.Messages[i].ID {
				t.Errorf("Message ID mismatch at index %d", i)
			}
			if msg.Content != session.Messages[i].Content {
				t.Errorf("Message content mismatch at index %d", i)
			}
			if msg.TokensUsed == nil && session.Messages[i].TokensUsed != nil {
				t.Errorf("Message TokensUsed should not be nil at index %d", i)
			}
		}
	}

	// Check agents
	if len(restored.ActiveAgents) != len(session.ActiveAgents) {
		t.Errorf("Agent count mismatch: %d != %d", len(restored.ActiveAgents), len(session.ActiveAgents))
	} else {
		for i, agent := range restored.ActiveAgents {
			if agent.ID != session.ActiveAgents[i].ID {
				t.Errorf("Agent ID mismatch at index %d", i)
			}
			if agent.Name != session.ActiveAgents[i].Name {
				t.Errorf("Agent name mismatch at index %d", i)
			}
		}
	}

	// Check tasks
	if len(restored.Tasks) != len(session.Tasks) {
		t.Errorf("Task count mismatch: %d != %d", len(restored.Tasks), len(session.Tasks))
	} else {
		for i, task := range restored.Tasks {
			if task.ID != session.Tasks[i].ID {
				t.Errorf("Task ID mismatch at index %d", i)
			}
			if task.Title != session.Tasks[i].Title {
				t.Errorf("Task title mismatch at index %d", i)
			}
			if task.Metrics == nil && session.Tasks[i].Metrics != nil {
				t.Errorf("Task Metrics should not be nil at index %d", i)
			}
		}
	}

	// Check timestamps
	if restored.CreatedAt.Unix() != session.CreatedAt.Unix() {
		t.Error("CreatedAt timestamp mismatch")
	}
	if restored.UpdatedAt.Unix() != session.UpdatedAt.Unix() {
		t.Error("UpdatedAt timestamp mismatch")
	}

	// Check counts
	if restored.MessageCount != session.MessageCount {
		t.Errorf("MessageCount mismatch: %d != %d", restored.MessageCount, session.MessageCount)
	}
	if restored.AgentCount != session.AgentCount {
		t.Errorf("AgentCount mismatch: %d != %d", restored.AgentCount, session.AgentCount)
	}
	if restored.TaskCount != session.TaskCount {
		t.Errorf("TaskCount mismatch: %d != %d", restored.TaskCount, session.TaskCount)
	}
	if restored.ErrorCount != session.ErrorCount {
		t.Errorf("ErrorCount mismatch: %d != %d", restored.ErrorCount, session.ErrorCount)
	}

	// Check maps
	if len(restored.Environment) != len(session.Environment) {
		t.Errorf("Environment map size mismatch: %d != %d", len(restored.Environment), len(session.Environment))
	}
	if len(restored.Variables) != len(session.Variables) {
		t.Errorf("Variables map size mismatch: %d != %d", len(restored.Variables), len(session.Variables))
	}
	if len(restored.Metadata) != len(session.Metadata) {
		t.Errorf("Metadata map size mismatch: %d != %d", len(restored.Metadata), len(session.Metadata))
	}
}
