package logger

import (
	"context"
	"log/slog"
)

// Example functions demonstrating context propagation patterns.
// These are not meant to be executed, but rather serve as reference implementations.

// ExampleBasicContextUsage shows how to create and use basic context fields.
func ExampleBasicContextUsage() {
	// Create a base context with task information
	ctx := context.Background()
	ctx = WithTaskID(ctx, "task_20250101_001")
	ctx = WithAgentID(ctx, "claude_worker_1")

	// Extract fields for logging
	fields := ExtractLogFields(ctx)
	_ = fields // Use in your logger

	// Or get individual fields
	taskID := GetTaskID(ctx)
	agentID := GetAgentID(ctx)
	_, _ = taskID, agentID
}

// ExampleHTTPRequestContext shows context usage in HTTP request handlers.
func ExampleHTTPRequestContext(parentCtx context.Context) {
	// In an HTTP handler, create a request-scoped context
	requestID := "req_20250101_123456_abc"

	// Start with parent context (might be background or have other values)
	ctx := WithRequestID(parentCtx, requestID)

	// Process the request
	_ = ctx
}

// ExampleWorkerSpawning shows how to spawn a worker with proper context hierarchy.
func ExampleWorkerSpawning(ctx context.Context) {
	parentTaskID := GetTaskID(ctx)

	// Create a new context for the worker task
	workerCtx := context.Background()
	workerCtx = WithTaskID(workerCtx, "worker_task_123")
	workerCtx = WithAgentID(workerCtx, "gemini_researcher")

	// Link back to parent for tracing
	if parentTaskID != "" {
		workerCtx = WithParentID(workerCtx, parentTaskID)
	}

	// Pass to worker function
	_ = workerCtx
}

// ExamplePipelineExecution shows context usage in a multi-stage pipeline.
func ExamplePipelineExecution(ctx context.Context) {
	// Enhance context with pipeline information
	ctx = WithPipelineID(ctx, "pipe_research_20250101_001")

	stages := []string{"data_collection", "analysis", "synthesis"}

	for _, stageName := range stages {
		// Create stage-specific context
		stageCtx := WithStageID(ctx, stageName)

		// All logs in this stage will include pipeline_id and stage_id
		_ = stageCtx
	}
}

// ExampleLogContextFields shows using the convenience type for setting multiple fields.
func ExampleLogContextFields() {
	// Set multiple fields at once
	fields := LogContextFields{
		TaskID:     "task_abc123",
		AgentID:    "claude_worker_1",
		RequestID:  "req_20250101_001",
		PipelineID: "pipe_research_001",
		StageID:    "stage_analyze",
		ParentID:   "task_parent_def",
	}

	ctx := fields.Apply(context.Background())

	// All fields are now available in ctx
	_ = ctx
}

// ExampleContextImmutability shows that context operations don't modify the original.
func ExampleContextImmutability(ctx context.Context) {
	// Original context is not modified
	ctx1 := WithTaskID(ctx, "task_1")
	ctx2 := WithTaskID(ctx, "task_2")

	// ctx, ctx1, ctx2 are all independent
	_ = ctx1
	_ = ctx2
}

// ExampleContextChaining shows building up context through multiple operations.
func ExampleContextChaining() {
	// Start with request context
	ctx := context.Background()
	ctx = WithRequestID(ctx, "req_001")

	// Add task information
	ctx = WithTaskID(ctx, "task_001")
	ctx = WithAgentID(ctx, "worker_1")

	// Add pipeline information
	ctx = WithPipelineID(ctx, "pipe_001")
	ctx = WithStageID(ctx, "stage_001")

	// All fields are available for logging
	_ = ctx
}

// ExampleConditionalContextUsage shows checking for field presence.
func ExampleConditionalContextUsage(ctx context.Context) {
	// Only log specific fields if they're set
	if taskID := GetTaskID(ctx); taskID != "" {
		// Log with task information
		_ = taskID
	}

	if pipelineID := GetPipelineID(ctx); pipelineID != "" {
		// Log with pipeline information
		_ = pipelineID
	}
}

// ExampleLoggingWithContext shows integrating context fields with slog.
func ExampleLoggingWithContext(slgLogger *slog.Logger, ctx context.Context) {
	// Extract all context fields
	fields := ExtractLogFields(ctx)

	// Convert to slog args
	args := []interface{}{}
	for k, v := range fields {
		args = append(args, k, v)
	}

	// Log with context fields
	slgLogger.Info("processing task", args...)
}

// ExampleErrorLoggingWithContext shows logging errors with context.
func ExampleErrorLoggingWithContext(slgLogger *slog.Logger, ctx context.Context, err error) {
	fields := ExtractLogFields(ctx)
	args := []interface{}{"error", err}
	for k, v := range fields {
		args = append(args, k, v)
	}

	slgLogger.Error("operation failed", args...)
}

// ExampleDistributedTracing shows complete request tracing through multiple functions.
func ExampleDistributedTracing(slgLogger *slog.Logger, requestID string) {
	// Create initial context
	ctx := context.Background()
	ctx = WithRequestID(ctx, requestID)

	fields := ExtractLogFields(ctx)
	args := []interface{}{}
	for k, v := range fields {
		args = append(args, k, v)
	}
	slgLogger.Info("request started", args...)

	// Call downstream service
	_ = downstreamService(ctx)

	fields = ExtractLogFields(ctx)
	args = []interface{}{}
	for k, v := range fields {
		args = append(args, k, v)
	}
	slgLogger.Info("request completed", args...)
}

func downstreamService(ctx context.Context) error {
	// Add task context
	ctx = WithTaskID(ctx, "service_task_1")
	ctx = WithAgentID(ctx, "service_worker")

	// RequestID is preserved from parent context
	requestID := GetRequestID(ctx)
	_ = requestID

	// All logs here will include both request_id and task_id
	fields := ExtractLogFields(ctx)
	_ = fields

	return nil
}

// ExampleTestingWithContext shows context usage in tests.
func ExampleTestingWithContext(slgLogger *slog.Logger) {
	// Create test context
	ctx := context.Background()
	ctx = WithTaskID(ctx, "test_task_1")
	ctx = WithAgentID(ctx, "test_agent")

	fields := ExtractLogFields(ctx)
	args := []interface{}{}
	for k, v := range fields {
		args = append(args, k, v)
	}
	slgLogger.Info("test starting", args...)

	// Simulate test execution
	_ = ctx

	fields = ExtractLogFields(ctx)
	args = []interface{}{}
	for k, v := range fields {
		args = append(args, k, v)
	}
	slgLogger.Info("test completed", args...)
}

// ExampleChildTaskCreation shows proper context hierarchy for child tasks.
func ExampleChildTaskCreation(ctx context.Context, parentTaskID string) context.Context {
	// Create child context
	childCtx := context.Background()
	childCtx = WithTaskID(childCtx, "child_task_1")

	// Link to parent
	if parentTaskID != "" {
		childCtx = WithParentID(childCtx, parentTaskID)
	}

	// Copy other context values if needed
	if agentID := GetAgentID(ctx); agentID != "" {
		childCtx = WithAgentID(childCtx, agentID)
	}

	return childCtx
}
