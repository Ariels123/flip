package context

import (
	"context"
)

// Context field keys for passing values through context.Context
type contextKey string

const (
	taskIDKey      contextKey = "task_id"
	agentIDKey     contextKey = "agent_id"
	requestIDKey   contextKey = "request_id"
	pipelineIDKey  contextKey = "pipeline_id"
	sessionIDKey   contextKey = "session_id"
	correlationIDKey contextKey = "correlation_id"
	userIDKey      contextKey = "user_id"
)

// WithTaskID returns a new context with the task ID set
func WithTaskID(ctx context.Context, taskID string) context.Context {
	return context.WithValue(ctx, taskIDKey, taskID)
}

// GetTaskID extracts the task ID from context, returns empty string if not found
func GetTaskID(ctx context.Context) string {
	value := ctx.Value(taskIDKey)
	if value == nil {
		return ""
	}
	if taskID, ok := value.(string); ok {
		return taskID
	}
	return ""
}

// WithAgentID returns a new context with the agent ID set
func WithAgentID(ctx context.Context, agentID string) context.Context {
	return context.WithValue(ctx, agentIDKey, agentID)
}

// GetAgentID extracts the agent ID from context, returns empty string if not found
func GetAgentID(ctx context.Context) string {
	value := ctx.Value(agentIDKey)
	if value == nil {
		return ""
	}
	if agentID, ok := value.(string); ok {
		return agentID
	}
	return ""
}

// WithRequestID returns a new context with the request ID set
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

// GetRequestID extracts the request ID from context, returns empty string if not found
func GetRequestID(ctx context.Context) string {
	value := ctx.Value(requestIDKey)
	if value == nil {
		return ""
	}
	if requestID, ok := value.(string); ok {
		return requestID
	}
	return ""
}

// WithPipelineID returns a new context with the pipeline ID set
func WithPipelineID(ctx context.Context, pipelineID string) context.Context {
	return context.WithValue(ctx, pipelineIDKey, pipelineID)
}

// GetPipelineID extracts the pipeline ID from context, returns empty string if not found
func GetPipelineID(ctx context.Context) string {
	value := ctx.Value(pipelineIDKey)
	if value == nil {
		return ""
	}
	if pipelineID, ok := value.(string); ok {
		return pipelineID
	}
	return ""
}

// WithSessionID returns a new context with the session ID set
func WithSessionID(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, sessionIDKey, sessionID)
}

// GetSessionID extracts the session ID from context, returns empty string if not found
func GetSessionID(ctx context.Context) string {
	value := ctx.Value(sessionIDKey)
	if value == nil {
		return ""
	}
	if sessionID, ok := value.(string); ok {
		return sessionID
	}
	return ""
}

// WithCorrelationID returns a new context with the correlation ID set
func WithCorrelationID(ctx context.Context, correlationID string) context.Context {
	return context.WithValue(ctx, correlationIDKey, correlationID)
}

// GetCorrelationID extracts the correlation ID from context, returns empty string if not found
func GetCorrelationID(ctx context.Context) string {
	value := ctx.Value(correlationIDKey)
	if value == nil {
		return ""
	}
	if correlationID, ok := value.(string); ok {
		return correlationID
	}
	return ""
}

// WithUserID returns a new context with the user ID set
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

// GetUserID extracts the user ID from context, returns empty string if not found
func GetUserID(ctx context.Context) string {
	value := ctx.Value(userIDKey)
	if value == nil {
		return ""
	}
	if userID, ok := value.(string); ok {
		return userID
	}
	return ""
}
