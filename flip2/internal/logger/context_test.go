package logger

import (
	"context"
	"testing"
)

func TestWithTaskID(t *testing.T) {
	tests := []struct {
		name      string
		taskID    string
		expectSet bool
	}{
		{
			name:      "Valid task ID",
			taskID:    "task_123",
			expectSet: true,
		},
		{
			name:      "Empty task ID",
			taskID:    "",
			expectSet: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := WithTaskID(context.Background(), tt.taskID)
			retrieved := GetTaskID(ctx)

			if tt.expectSet {
				if retrieved != tt.taskID {
					t.Errorf("expected %s, got %s", tt.taskID, retrieved)
				}
			} else {
				if retrieved != "" {
					t.Errorf("expected empty string, got %s", retrieved)
				}
			}
		})
	}
}

func TestWithAgentID(t *testing.T) {
	agentID := "claude_worker_1"
	ctx := WithAgentID(context.Background(), agentID)
	retrieved := GetAgentID(ctx)

	if retrieved != agentID {
		t.Errorf("expected %s, got %s", agentID, retrieved)
	}
}

func TestWithRequestID(t *testing.T) {
	requestID := "req_20250101_123456"
	ctx := WithRequestID(context.Background(), requestID)
	retrieved := GetRequestID(ctx)

	if retrieved != requestID {
		t.Errorf("expected %s, got %s", requestID, retrieved)
	}
}

func TestWithPipelineID(t *testing.T) {
	pipelineID := "pipe_research_001"
	ctx := WithPipelineID(context.Background(), pipelineID)
	retrieved := GetPipelineID(ctx)

	if retrieved != pipelineID {
		t.Errorf("expected %s, got %s", pipelineID, retrieved)
	}
}

func TestWithStageID(t *testing.T) {
	stageID := "stage_data_collection"
	ctx := WithStageID(context.Background(), stageID)
	retrieved := GetStageID(ctx)

	if retrieved != stageID {
		t.Errorf("expected %s, got %s", stageID, retrieved)
	}
}

func TestWithParentID(t *testing.T) {
	parentID := "parent_task_456"
	ctx := WithParentID(context.Background(), parentID)
	retrieved := GetParentID(ctx)

	if retrieved != parentID {
		t.Errorf("expected %s, got %s", parentID, retrieved)
	}
}

func TestMultipleContextFields(t *testing.T) {
	ctx := context.Background()
	ctx = WithTaskID(ctx, "task_123")
	ctx = WithAgentID(ctx, "claude_worker_1")
	ctx = WithRequestID(ctx, "req_001")
	ctx = WithPipelineID(ctx, "pipe_001")
	ctx = WithStageID(ctx, "stage_001")
	ctx = WithParentID(ctx, "parent_123")

	if GetTaskID(ctx) != "task_123" {
		t.Error("TaskID not set correctly")
	}
	if GetAgentID(ctx) != "claude_worker_1" {
		t.Error("AgentID not set correctly")
	}
	if GetRequestID(ctx) != "req_001" {
		t.Error("RequestID not set correctly")
	}
	if GetPipelineID(ctx) != "pipe_001" {
		t.Error("PipelineID not set correctly")
	}
	if GetStageID(ctx) != "stage_001" {
		t.Error("StageID not set correctly")
	}
	if GetParentID(ctx) != "parent_123" {
		t.Error("ParentID not set correctly")
	}
}

func TestExtractLogFields(t *testing.T) {
	tests := []struct {
		name           string
		setupCtx       func(context.Context) context.Context
		expectedFields map[string]interface{}
	}{
		{
			name: "Empty context",
			setupCtx: func(ctx context.Context) context.Context {
				return ctx
			},
			expectedFields: map[string]interface{}{},
		},
		{
			name: "Single field",
			setupCtx: func(ctx context.Context) context.Context {
				return WithTaskID(ctx, "task_123")
			},
			expectedFields: map[string]interface{}{
				"task_id": "task_123",
			},
		},
		{
			name: "Multiple fields",
			setupCtx: func(ctx context.Context) context.Context {
				ctx = WithTaskID(ctx, "task_123")
				ctx = WithAgentID(ctx, "worker_1")
				ctx = WithRequestID(ctx, "req_001")
				return ctx
			},
			expectedFields: map[string]interface{}{
				"task_id":    "task_123",
				"agent_id":   "worker_1",
				"request_id": "req_001",
			},
		},
		{
			name: "All fields",
			setupCtx: func(ctx context.Context) context.Context {
				ctx = WithTaskID(ctx, "task_abc")
				ctx = WithAgentID(ctx, "gemini_bot")
				ctx = WithRequestID(ctx, "req_xyz")
				ctx = WithPipelineID(ctx, "pipe_research")
				ctx = WithStageID(ctx, "stage_analyze")
				ctx = WithParentID(ctx, "parent_def")
				return ctx
			},
			expectedFields: map[string]interface{}{
				"task_id":     "task_abc",
				"agent_id":    "gemini_bot",
				"request_id":  "req_xyz",
				"pipeline_id": "pipe_research",
				"stage_id":    "stage_analyze",
				"parent_id":   "parent_def",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.setupCtx(context.Background())
			fields := ExtractLogFields(ctx)

			// Check expected fields
			for key, expectedValue := range tt.expectedFields {
				if actualValue, ok := fields[key]; !ok {
					t.Errorf("expected field %s to be present", key)
				} else if actualValue != expectedValue {
					t.Errorf("field %s: expected %v, got %v", key, expectedValue, actualValue)
				}
			}

			// Check no unexpected fields
			if len(fields) != len(tt.expectedFields) {
				t.Errorf("expected %d fields, got %d", len(tt.expectedFields), len(fields))
			}
		})
	}
}

func TestLogContextFieldsApply(t *testing.T) {
	fields := LogContextFields{
		TaskID:     "task_123",
		AgentID:    "worker_1",
		RequestID:  "req_001",
		PipelineID: "pipe_001",
		StageID:    "stage_001",
		ParentID:   "parent_123",
	}

	ctx := fields.Apply(context.Background())

	if GetTaskID(ctx) != "task_123" {
		t.Error("TaskID not applied")
	}
	if GetAgentID(ctx) != "worker_1" {
		t.Error("AgentID not applied")
	}
	if GetRequestID(ctx) != "req_001" {
		t.Error("RequestID not applied")
	}
	if GetPipelineID(ctx) != "pipe_001" {
		t.Error("PipelineID not applied")
	}
	if GetStageID(ctx) != "stage_001" {
		t.Error("StageID not applied")
	}
	if GetParentID(ctx) != "parent_123" {
		t.Error("ParentID not applied")
	}
}

func TestLogContextFieldsApplyPartial(t *testing.T) {
	fields := LogContextFields{
		TaskID:  "task_123",
		AgentID: "worker_1",
		// Empty RequestID, PipelineID, StageID, ParentID
	}

	ctx := fields.Apply(context.Background())

	if GetTaskID(ctx) != "task_123" {
		t.Error("TaskID not applied")
	}
	if GetAgentID(ctx) != "worker_1" {
		t.Error("AgentID not applied")
	}
	if GetRequestID(ctx) != "" {
		t.Errorf("expected empty RequestID, got %s", GetRequestID(ctx))
	}
	if GetPipelineID(ctx) != "" {
		t.Errorf("expected empty PipelineID, got %s", GetPipelineID(ctx))
	}
	if GetStageID(ctx) != "" {
		t.Errorf("expected empty StageID, got %s", GetStageID(ctx))
	}
	if GetParentID(ctx) != "" {
		t.Errorf("expected empty ParentID, got %s", GetParentID(ctx))
	}
}

func TestEmptyStringFieldsNotSet(t *testing.T) {
	ctx := context.Background()
	ctx = WithTaskID(ctx, "")
	ctx = WithAgentID(ctx, "")

	fields := ExtractLogFields(ctx)
	if len(fields) != 0 {
		t.Errorf("expected no fields for empty strings, got %d", len(fields))
	}
}

func TestContextImmutability(t *testing.T) {
	ctx1 := context.Background()
	ctx2 := WithTaskID(ctx1, "task_123")

	// Original context should not be modified
	if GetTaskID(ctx1) != "" {
		t.Error("original context was modified")
	}

	// New context should have the task ID
	if GetTaskID(ctx2) != "task_123" {
		t.Error("new context does not have task ID")
	}
}

func TestContextChaining(t *testing.T) {
	// Test that we can chain context operations
	ctx := context.Background()
	ctx = WithTaskID(ctx, "task_1")
	ctx = WithAgentID(ctx, "agent_1")
	ctx = WithParentID(ctx, "task_0")

	// Add more context to a derived context
	ctx2 := WithRequestID(ctx, "req_1")
	ctx2 = WithPipelineID(ctx2, "pipe_1")

	// ctx should not have RequestID or PipelineID
	if GetRequestID(ctx) != "" {
		t.Error("ctx should not have RequestID")
	}
	if GetPipelineID(ctx) != "" {
		t.Error("ctx should not have PipelineID")
	}

	// ctx2 should have all fields
	if GetTaskID(ctx2) != "task_1" {
		t.Error("ctx2 lost TaskID")
	}
	if GetAgentID(ctx2) != "agent_1" {
		t.Error("ctx2 lost AgentID")
	}
	if GetParentID(ctx2) != "task_0" {
		t.Error("ctx2 lost ParentID")
	}
	if GetRequestID(ctx2) != "req_1" {
		t.Error("ctx2 missing RequestID")
	}
	if GetPipelineID(ctx2) != "pipe_1" {
		t.Error("ctx2 missing PipelineID")
	}
}

func BenchmarkWithTaskID(b *testing.B) {
	ctx := context.Background()
	for i := 0; i < b.N; i++ {
		WithTaskID(ctx, "task_123")
	}
}

func BenchmarkExtractLogFields(b *testing.B) {
	ctx := context.Background()
	ctx = WithTaskID(ctx, "task_123")
	ctx = WithAgentID(ctx, "worker_1")
	ctx = WithRequestID(ctx, "req_001")
	ctx = WithPipelineID(ctx, "pipe_001")
	ctx = WithStageID(ctx, "stage_001")
	ctx = WithParentID(ctx, "parent_123")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ExtractLogFields(ctx)
	}
}

func BenchmarkLogContextFieldsApply(b *testing.B) {
	fields := LogContextFields{
		TaskID:     "task_123",
		AgentID:    "worker_1",
		RequestID:  "req_001",
		PipelineID: "pipe_001",
		StageID:    "stage_001",
		ParentID:   "parent_123",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fields.Apply(context.Background())
	}
}
