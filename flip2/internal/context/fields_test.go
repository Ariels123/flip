package context

import (
	"context"
	"testing"
)

func TestWithAndGetTaskID(t *testing.T) {
	ctx := context.Background()
	taskID := "task-123"

	// Set task ID
	ctx = WithTaskID(ctx, taskID)

	// Get task ID
	retrieved := GetTaskID(ctx)
	if retrieved != taskID {
		t.Errorf("Expected task ID %q, got %q", taskID, retrieved)
	}
}

func TestGetTaskIDMissing(t *testing.T) {
	ctx := context.Background()

	// Get task ID from context without setting it
	retrieved := GetTaskID(ctx)
	if retrieved != "" {
		t.Errorf("Expected empty string for missing task ID, got %q", retrieved)
	}
}

func TestWithAndGetAgentID(t *testing.T) {
	ctx := context.Background()
	agentID := "agent-456"

	// Set agent ID
	ctx = WithAgentID(ctx, agentID)

	// Get agent ID
	retrieved := GetAgentID(ctx)
	if retrieved != agentID {
		t.Errorf("Expected agent ID %q, got %q", agentID, retrieved)
	}
}

func TestGetAgentIDMissing(t *testing.T) {
	ctx := context.Background()

	// Get agent ID from context without setting it
	retrieved := GetAgentID(ctx)
	if retrieved != "" {
		t.Errorf("Expected empty string for missing agent ID, got %q", retrieved)
	}
}

func TestWithAndGetRequestID(t *testing.T) {
	ctx := context.Background()
	requestID := "req-789"

	// Set request ID
	ctx = WithRequestID(ctx, requestID)

	// Get request ID
	retrieved := GetRequestID(ctx)
	if retrieved != requestID {
		t.Errorf("Expected request ID %q, got %q", requestID, retrieved)
	}
}

func TestGetRequestIDMissing(t *testing.T) {
	ctx := context.Background()

	// Get request ID from context without setting it
	retrieved := GetRequestID(ctx)
	if retrieved != "" {
		t.Errorf("Expected empty string for missing request ID, got %q", retrieved)
	}
}

func TestWithAndGetPipelineID(t *testing.T) {
	ctx := context.Background()
	pipelineID := "pipeline-101"

	// Set pipeline ID
	ctx = WithPipelineID(ctx, pipelineID)

	// Get pipeline ID
	retrieved := GetPipelineID(ctx)
	if retrieved != pipelineID {
		t.Errorf("Expected pipeline ID %q, got %q", pipelineID, retrieved)
	}
}

func TestGetPipelineIDMissing(t *testing.T) {
	ctx := context.Background()

	// Get pipeline ID from context without setting it
	retrieved := GetPipelineID(ctx)
	if retrieved != "" {
		t.Errorf("Expected empty string for missing pipeline ID, got %q", retrieved)
	}
}

func TestWithAndGetSessionID(t *testing.T) {
	ctx := context.Background()
	sessionID := "session-202"

	// Set session ID
	ctx = WithSessionID(ctx, sessionID)

	// Get session ID
	retrieved := GetSessionID(ctx)
	if retrieved != sessionID {
		t.Errorf("Expected session ID %q, got %q", sessionID, retrieved)
	}
}

func TestGetSessionIDMissing(t *testing.T) {
	ctx := context.Background()

	// Get session ID from context without setting it
	retrieved := GetSessionID(ctx)
	if retrieved != "" {
		t.Errorf("Expected empty string for missing session ID, got %q", retrieved)
	}
}

func TestWithAndGetCorrelationID(t *testing.T) {
	ctx := context.Background()
	correlationID := "corr-303"

	// Set correlation ID
	ctx = WithCorrelationID(ctx, correlationID)

	// Get correlation ID
	retrieved := GetCorrelationID(ctx)
	if retrieved != correlationID {
		t.Errorf("Expected correlation ID %q, got %q", correlationID, retrieved)
	}
}

func TestGetCorrelationIDMissing(t *testing.T) {
	ctx := context.Background()

	// Get correlation ID from context without setting it
	retrieved := GetCorrelationID(ctx)
	if retrieved != "" {
		t.Errorf("Expected empty string for missing correlation ID, got %q", retrieved)
	}
}

func TestWithAndGetUserID(t *testing.T) {
	ctx := context.Background()
	userID := "user-404"

	// Set user ID
	ctx = WithUserID(ctx, userID)

	// Get user ID
	retrieved := GetUserID(ctx)
	if retrieved != userID {
		t.Errorf("Expected user ID %q, got %q", userID, retrieved)
	}
}

func TestGetUserIDMissing(t *testing.T) {
	ctx := context.Background()

	// Get user ID from context without setting it
	retrieved := GetUserID(ctx)
	if retrieved != "" {
		t.Errorf("Expected empty string for missing user ID, got %q", retrieved)
	}
}

// TestMultipleContextFields verifies multiple fields can be set and retrieved independently
func TestMultipleContextFields(t *testing.T) {
	ctx := context.Background()

	// Set multiple fields
	ctx = WithTaskID(ctx, "task-1")
	ctx = WithAgentID(ctx, "agent-1")
	ctx = WithRequestID(ctx, "req-1")
	ctx = WithPipelineID(ctx, "pipeline-1")
	ctx = WithSessionID(ctx, "session-1")
	ctx = WithCorrelationID(ctx, "corr-1")
	ctx = WithUserID(ctx, "user-1")

	// Verify all fields are set correctly
	tests := []struct {
		name     string
		expected string
		actual   string
	}{
		{"TaskID", "task-1", GetTaskID(ctx)},
		{"AgentID", "agent-1", GetAgentID(ctx)},
		{"RequestID", "req-1", GetRequestID(ctx)},
		{"PipelineID", "pipeline-1", GetPipelineID(ctx)},
		{"SessionID", "session-1", GetSessionID(ctx)},
		{"CorrelationID", "corr-1", GetCorrelationID(ctx)},
		{"UserID", "user-1", GetUserID(ctx)},
	}

	for _, tt := range tests {
		if tt.expected != tt.actual {
			t.Errorf("%s: expected %q, got %q", tt.name, tt.expected, tt.actual)
		}
	}
}

// TestContextChaining verifies that context fields can be chained in method calls
func TestContextChaining(t *testing.T) {
	ctx := context.WithValue(context.Background(), "other", "value")

	// Chain multiple WithX calls
	ctx = WithTaskID(WithAgentID(WithRequestID(ctx, "req-chain"), "agent-chain"), "task-chain")

	// Verify all fields are set
	if GetTaskID(ctx) != "task-chain" {
		t.Errorf("Expected task-chain, got %q", GetTaskID(ctx))
	}
	if GetAgentID(ctx) != "agent-chain" {
		t.Errorf("Expected agent-chain, got %q", GetAgentID(ctx))
	}
	if GetRequestID(ctx) != "req-chain" {
		t.Errorf("Expected req-chain, got %q", GetRequestID(ctx))
	}
	// Verify existing value is preserved
	if ctx.Value("other") != "value" {
		t.Errorf("Expected existing value to be preserved")
	}
}

// TestEmptyStringValues verifies that empty strings are handled correctly
func TestEmptyStringValues(t *testing.T) {
	ctx := context.Background()

	// Set fields to empty strings
	ctx = WithTaskID(ctx, "")
	ctx = WithAgentID(ctx, "")
	ctx = WithRequestID(ctx, "")

	// Get should return empty strings (not missing values)
	if GetTaskID(ctx) != "" {
		t.Errorf("Expected empty string for task ID, got %q", GetTaskID(ctx))
	}
	if GetAgentID(ctx) != "" {
		t.Errorf("Expected empty string for agent ID, got %q", GetAgentID(ctx))
	}
	if GetRequestID(ctx) != "" {
		t.Errorf("Expected empty string for request ID, got %q", GetRequestID(ctx))
	}
}

// TestOverwriteValues verifies that values can be overwritten
func TestOverwriteValues(t *testing.T) {
	ctx := context.Background()

	// Set initial value
	ctx = WithTaskID(ctx, "task-old")
	if GetTaskID(ctx) != "task-old" {
		t.Errorf("Expected task-old, got %q", GetTaskID(ctx))
	}

	// Overwrite value
	ctx = WithTaskID(ctx, "task-new")
	if GetTaskID(ctx) != "task-new" {
		t.Errorf("Expected task-new after overwrite, got %q", GetTaskID(ctx))
	}
}
