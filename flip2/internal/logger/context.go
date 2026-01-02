package logger

import (
	"context"
)

// ContextKey is a type for context keys to prevent collisions
type ContextKey string

const (
	// TaskIDKey is the context key for task ID
	TaskIDKey ContextKey = "task_id"
	// AgentIDKey is the context key for agent ID
	AgentIDKey ContextKey = "agent_id"
	// RequestIDKey is the context key for HTTP request ID
	RequestIDKey ContextKey = "request_id"
	// PipelineIDKey is the context key for pipeline run ID
	PipelineIDKey ContextKey = "pipeline_id"
	// StageIDKey is the context key for pipeline stage ID
	StageIDKey ContextKey = "stage_id"
	// ParentIDKey is the context key for parent task/request ID (for tracing)
	ParentIDKey ContextKey = "parent_id"
)

// WithTaskID returns a new context with the task ID set.
// This ID uniquely identifies a task within the system.
// Example: "task_abc123" or "spawn_20250101_001"
func WithTaskID(ctx context.Context, taskID string) context.Context {
	if taskID == "" {
		return ctx
	}
	return context.WithValue(ctx, TaskIDKey, taskID)
}

// WithAgentID returns a new context with the agent ID set.
// This identifies which agent is executing the task.
// Example: "claude_worker_1" or "gemini_researcher"
func WithAgentID(ctx context.Context, agentID string) context.Context {
	if agentID == "" {
		return ctx
	}
	return context.WithValue(ctx, AgentIDKey, agentID)
}

// WithRequestID returns a new context with the HTTP request ID set.
// Used for correlating log entries from a single HTTP request.
// Example: "req_20250101_123456_abc" (UUID or similar)
func WithRequestID(ctx context.Context, requestID string) context.Context {
	if requestID == "" {
		return ctx
	}
	return context.WithValue(ctx, RequestIDKey, requestID)
}

// WithPipelineID returns a new context with the pipeline ID set.
// Identifies which pipeline execution this task belongs to.
// Example: "pipe_20250101_research_001"
func WithPipelineID(ctx context.Context, pipelineID string) context.Context {
	if pipelineID == "" {
		return ctx
	}
	return context.WithValue(ctx, PipelineIDKey, pipelineID)
}

// WithStageID returns a new context with the pipeline stage ID set.
// Identifies the specific stage within a pipeline execution.
// Example: "stage_data_collection" or "stage_analysis"
func WithStageID(ctx context.Context, stageID string) context.Context {
	if stageID == "" {
		return ctx
	}
	return context.WithValue(ctx, StageIDKey, stageID)
}

// WithParentID returns a new context with the parent ID set.
// Used for distributed tracing - links child tasks to their parent.
// Example: parent is a task, children are spawned worker tasks
func WithParentID(ctx context.Context, parentID string) context.Context {
	if parentID == "" {
		return ctx
	}
	return context.WithValue(ctx, ParentIDKey, parentID)
}

// ExtractLogFields extracts all logging context fields from the context.
// Returns a map suitable for passing to slog handlers.
// Returns an empty map if no context fields are set.
//
// Usage with slog:
//   ctx := WithTaskID(context.Background(), "task_123")
//   fields := ExtractLogFields(ctx)
//   for key, value := range fields {
//       logger.Info("message", key, value)
//   }
func ExtractLogFields(ctx context.Context) map[string]interface{} {
	fields := make(map[string]interface{})

	if taskID, ok := ctx.Value(TaskIDKey).(string); ok && taskID != "" {
		fields["task_id"] = taskID
	}
	if agentID, ok := ctx.Value(AgentIDKey).(string); ok && agentID != "" {
		fields["agent_id"] = agentID
	}
	if requestID, ok := ctx.Value(RequestIDKey).(string); ok && requestID != "" {
		fields["request_id"] = requestID
	}
	if pipelineID, ok := ctx.Value(PipelineIDKey).(string); ok && pipelineID != "" {
		fields["pipeline_id"] = pipelineID
	}
	if stageID, ok := ctx.Value(StageIDKey).(string); ok && stageID != "" {
		fields["stage_id"] = stageID
	}
	if parentID, ok := ctx.Value(ParentIDKey).(string); ok && parentID != "" {
		fields["parent_id"] = parentID
	}

	return fields
}

// LogContextFields is a convenience type for building context with multiple fields.
// Usage:
//   fields := LogContextFields{
//       TaskID: "task_123",
//       AgentID: "worker_1",
//   }
//   ctx := fields.Apply(context.Background())
type LogContextFields struct {
	TaskID     string
	AgentID    string
	RequestID  string
	PipelineID string
	StageID    string
	ParentID   string
}

// Apply applies all set fields to the context and returns the new context.
// Fields with empty string values are skipped.
func (f LogContextFields) Apply(ctx context.Context) context.Context {
	ctx = WithTaskID(ctx, f.TaskID)
	ctx = WithAgentID(ctx, f.AgentID)
	ctx = WithRequestID(ctx, f.RequestID)
	ctx = WithPipelineID(ctx, f.PipelineID)
	ctx = WithStageID(ctx, f.StageID)
	ctx = WithParentID(ctx, f.ParentID)
	return ctx
}

// GetTaskID extracts the task ID from context, returns empty string if not set.
func GetTaskID(ctx context.Context) string {
	if id, ok := ctx.Value(TaskIDKey).(string); ok {
		return id
	}
	return ""
}

// GetAgentID extracts the agent ID from context, returns empty string if not set.
func GetAgentID(ctx context.Context) string {
	if id, ok := ctx.Value(AgentIDKey).(string); ok {
		return id
	}
	return ""
}

// GetRequestID extracts the request ID from context, returns empty string if not set.
func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(RequestIDKey).(string); ok {
		return id
	}
	return ""
}

// GetPipelineID extracts the pipeline ID from context, returns empty string if not set.
func GetPipelineID(ctx context.Context) string {
	if id, ok := ctx.Value(PipelineIDKey).(string); ok {
		return id
	}
	return ""
}

// GetStageID extracts the stage ID from context, returns empty string if not set.
func GetStageID(ctx context.Context) string {
	if id, ok := ctx.Value(StageIDKey).(string); ok {
		return id
	}
	return ""
}

// GetParentID extracts the parent ID from context, returns empty string if not set.
func GetParentID(ctx context.Context) string {
	if id, ok := ctx.Value(ParentIDKey).(string); ok {
		return id
	}
	return ""
}
