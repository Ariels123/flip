// Package mcp provides Model Context Protocol (MCP) server integration.
//
// This file contains tests for the sampling handler implementation.

package mcp

import (
	"context"
	"testing"
	"time"

	"flip2/internal/llm"
)

// =============================================================================
// Mock LLM Backend for Testing
// =============================================================================

// mockLLMBackend is a mock implementation of llm.Backend for testing.
type mockLLMBackend struct {
	name           string
	models         []string
	defaultModel   string
	available      bool
	executeFunc    func(ctx context.Context, prompt string, opts *llm.Options) (*llm.Response, error)
	streamFunc     func(ctx context.Context, prompt string, opts *llm.Options) (<-chan llm.StreamChunk, error)
	checkQuotaFunc func(ctx context.Context) (float64, error)
}

func newMockBackend(name string) *mockLLMBackend {
	return &mockLLMBackend{
		name:         name,
		models:       []string{"model-1", "model-2"},
		defaultModel: "model-1",
		available:    true,
		executeFunc: func(ctx context.Context, prompt string, opts *llm.Options) (*llm.Response, error) {
			return &llm.Response{
				Content:       "Test response",
				Model:         "model-1",
				InputTokens:   100,
				OutputTokens:  50,
				CostUSD:       0.001,
				FinishReason:  "stop",
				Latency:       100 * time.Millisecond,
			}, nil
		},
		checkQuotaFunc: func(ctx context.Context) (float64, error) {
			return 0.8, nil
		},
	}
}

func (m *mockLLMBackend) Name() string {
	return m.name
}

func (m *mockLLMBackend) Execute(ctx context.Context, prompt string, opts *llm.Options) (*llm.Response, error) {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, prompt, opts)
	}
	return &llm.Response{
		Content:      "Default response",
		Model:        m.defaultModel,
		InputTokens:  10,
		OutputTokens: 5,
		CostUSD:      0.0001,
		FinishReason: "stop",
		Latency:      10 * time.Millisecond,
	}, nil
}

func (m *mockLLMBackend) Stream(ctx context.Context, prompt string, opts *llm.Options) (<-chan llm.StreamChunk, error) {
	if m.streamFunc != nil {
		return m.streamFunc(ctx, prompt, opts)
	}

	ch := make(chan llm.StreamChunk, 3)
	go func() {
		ch <- llm.StreamChunk{Text: "Stream", Done: false}
		ch <- llm.StreamChunk{Text: " response", Done: true, InputTokens: 50, OutputTokens: 25}
		close(ch)
	}()
	return ch, nil
}

func (m *mockLLMBackend) CheckQuota(ctx context.Context) (float64, error) {
	if m.checkQuotaFunc != nil {
		return m.checkQuotaFunc(ctx)
	}
	return 0.8, nil
}

func (m *mockLLMBackend) Models() []string {
	return m.models
}

func (m *mockLLMBackend) DefaultModel() string {
	return m.defaultModel
}

func (m *mockLLMBackend) IsAvailable(ctx context.Context) bool {
	return m.available
}

// =============================================================================
// Tests
// =============================================================================

// TestCreateMessage tests basic message completion.
func TestCreateMessage(t *testing.T) {
	// Create backend registry
	registry := llm.NewRegistry()
	backend := newMockBackend("claude")
	registry.Register(backend)

	// Create sampling handler
	handler := NewSamplingHandler(registry, "claude")

	// Create a sampling request
	request := &SamplingRequest{
		Messages: []SamplingMessage{
			{
				Role: "user",
				Content: MessageContent{
					Type: "text",
					Text: "Hello, what is 2+2?",
				},
			},
		},
		MaxTokens: 100,
	}

	// Execute
	ctx := context.Background()
	response, err := handler.CreateMessage(ctx, request)

	// Verify
	if err != nil {
		t.Fatalf("CreateMessage failed: %v", err)
	}

	if response == nil {
		t.Fatal("Response is nil")
	}

	if response.Role != "assistant" {
		t.Errorf("Expected role 'assistant', got %q", response.Role)
	}

	if response.Content.Type != "text" {
		t.Errorf("Expected content type 'text', got %q", response.Content.Type)
	}

	if response.Model == "" {
		t.Error("Response model is empty")
	}
}

// TestCreateMessageNoBackends tests handling when no backends are available.
func TestCreateMessageNoBackends(t *testing.T) {
	// Create empty backend registry
	registry := llm.NewRegistry()

	// Create sampling handler
	handler := NewSamplingHandler(registry, "")

	// Create a sampling request
	request := &SamplingRequest{
		Messages: []SamplingMessage{
			{
				Role: "user",
				Content: MessageContent{
					Type: "text",
					Text: "Hello",
				},
			},
		},
	}

	// Execute
	ctx := context.Background()
	_, err := handler.CreateMessage(ctx, request)

	// Verify error
	if err == nil {
		t.Fatal("Expected error when no backends available")
	}
}

// TestCreateMessageNilRequest tests handling of nil request.
func TestCreateMessageNilRequest(t *testing.T) {
	registry := llm.NewRegistry()
	registry.Register(newMockBackend("claude"))

	handler := NewSamplingHandler(registry, "claude")
	ctx := context.Background()

	_, err := handler.CreateMessage(ctx, nil)
	if err == nil {
		t.Fatal("Expected error for nil request")
	}
}

// TestCreateMessageNoMessages tests handling of empty message list.
func TestCreateMessageNoMessages(t *testing.T) {
	registry := llm.NewRegistry()
	registry.Register(newMockBackend("claude"))

	handler := NewSamplingHandler(registry, "claude")

	request := &SamplingRequest{
		Messages: []SamplingMessage{},
	}

	ctx := context.Background()
	_, err := handler.CreateMessage(ctx, request)

	if err == nil {
		t.Fatal("Expected error for empty messages")
	}
}

// TestSelectBackendWithModelHints tests backend selection with model preferences.
func TestSelectBackendWithModelHints(t *testing.T) {
	registry := llm.NewRegistry()
	claudeBackend := newMockBackend("claude")
	geminiBackend := newMockBackend("gemini")
	registry.Register(claudeBackend)
	registry.Register(geminiBackend)

	handler := NewSamplingHandler(registry, "claude").(*samplingHandlerImpl)

	// Create request with claude model hint
	request := &SamplingRequest{
		Messages: []SamplingMessage{
			{Role: "user", Content: MessageContent{Type: "text", Text: "test"}},
		},
		ModelPreferences: &ModelPreferences{
			Hints: []ModelHint{
				{Name: "claude-3-sonnet"},
			},
		},
	}

	ctx := context.Background()
	backend := handler.selectBackend(ctx, request)

	if backend == nil {
		t.Fatal("Expected backend to be selected")
	}

	if backend.Name() != "claude" {
		t.Errorf("Expected claude backend, got %s", backend.Name())
	}
}

// TestSelectBackendWithCostPriority tests cost-based backend selection.
func TestSelectBackendWithCostPriority(t *testing.T) {
	registry := llm.NewRegistry()
	claudeBackend := newMockBackend("claude")
	geminiBackend := newMockBackend("gemini")
	registry.Register(claudeBackend)
	registry.Register(geminiBackend)

	handler := NewSamplingHandler(registry, "claude").(*samplingHandlerImpl)

	request := &SamplingRequest{
		Messages: []SamplingMessage{
			{Role: "user", Content: MessageContent{Type: "text", Text: "test"}},
		},
		ModelPreferences: &ModelPreferences{
			CostPriority: 0.9, // High cost priority should prefer cheaper backends
		},
	}

	ctx := context.Background()
	backend := handler.selectBackend(ctx, request)

	if backend == nil {
		t.Fatal("Expected backend to be selected")
	}

	// Should select gemini for cost priority
	if backend.Name() != "gemini" {
		t.Errorf("Expected gemini backend for cost priority, got %s", backend.Name())
	}
}

// TestSelectBackendWithIntelligencePriority tests intelligence-based selection.
func TestSelectBackendWithIntelligencePriority(t *testing.T) {
	registry := llm.NewRegistry()
	claudeBackend := newMockBackend("claude")
	geminiBackend := newMockBackend("gemini")
	registry.Register(claudeBackend)
	registry.Register(geminiBackend)

	handler := NewSamplingHandler(registry, "claude").(*samplingHandlerImpl)

	request := &SamplingRequest{
		Messages: []SamplingMessage{
			{Role: "user", Content: MessageContent{Type: "text", Text: "test"}},
		},
		ModelPreferences: &ModelPreferences{
			IntelligencePriority: 0.9, // High intelligence priority should prefer claude
		},
	}

	ctx := context.Background()
	backend := handler.selectBackend(ctx, request)

	if backend == nil {
		t.Fatal("Expected backend to be selected")
	}

	if backend.Name() != "claude" {
		t.Errorf("Expected claude backend for intelligence priority, got %s", backend.Name())
	}
}

// TestMessageConversion tests conversion of MCP messages to prompt.
func TestMessageConversion(t *testing.T) {
	registry := llm.NewRegistry()
	registry.Register(newMockBackend("claude"))

	handler := NewSamplingHandler(registry, "claude").(*samplingHandlerImpl)

	messages := []SamplingMessage{
		{
			Role: "user",
			Content: MessageContent{
				Type: "text",
				Text: "First message",
			},
		},
		{
			Role: "assistant",
			Content: MessageContent{
				Type: "text",
				Text: "Second message",
			},
		},
	}

	prompt := handler.messagesToPrompt(messages)

	if prompt == "" {
		t.Fatal("Prompt is empty")
	}

	if len(prompt) == 0 {
		t.Fatal("Prompt has no content")
	}
}

// TestMetricsTracking tests that metrics are properly recorded.
func TestMetricsTracking(t *testing.T) {
	registry := llm.NewRegistry()
	backend := newMockBackend("claude")
	registry.Register(backend)

	handler := NewSamplingHandler(registry, "claude")

	request := &SamplingRequest{
		Messages: []SamplingMessage{
			{Role: "user", Content: MessageContent{Type: "text", Text: "test"}},
		},
	}

	ctx := context.Background()

	// Execute multiple times to accumulate metrics
	for i := 0; i < 3; i++ {
		_, err := handler.CreateMessage(ctx, request)
		if err != nil {
			t.Fatalf("CreateMessage failed: %v", err)
		}
	}

	// Check metrics
	implHandler := handler.(*samplingHandlerImpl)
	metrics := implHandler.GetMetrics("claude")

	if metrics == nil {
		t.Fatal("Metrics are nil")
	}

	if metrics.TotalRequests != 3 {
		t.Errorf("Expected 3 total requests, got %d", metrics.TotalRequests)
	}

	if metrics.SuccessfulRequests != 3 {
		t.Errorf("Expected 3 successful requests, got %d", metrics.SuccessfulRequests)
	}

	if metrics.TotalInputTokens == 0 {
		t.Error("Expected non-zero input tokens")
	}

	if metrics.TotalOutputTokens == 0 {
		t.Error("Expected non-zero output tokens")
	}

	if metrics.TotalCostUSD == 0 {
		t.Error("Expected non-zero cost")
	}
}

// TestMetricsFailure tests that failure metrics are recorded.
func TestMetricsFailure(t *testing.T) {
	registry := llm.NewRegistry()
	backend := newMockBackend("claude")

	// Set up backend to fail
	backend.executeFunc = func(ctx context.Context, prompt string, opts *llm.Options) (*llm.Response, error) {
		return nil, errTest
	}

	registry.Register(backend)

	handler := NewSamplingHandler(registry, "claude")

	request := &SamplingRequest{
		Messages: []SamplingMessage{
			{Role: "user", Content: MessageContent{Type: "text", Text: "test"}},
		},
	}

	ctx := context.Background()
	_, err := handler.CreateMessage(ctx, request)

	if err == nil {
		t.Fatal("Expected error from backend")
	}

	// Check failure metrics
	implHandler := handler.(*samplingHandlerImpl)
	metrics := implHandler.GetMetrics("claude")

	if metrics == nil {
		t.Fatal("Metrics are nil")
	}

	if metrics.FailedRequests != 1 {
		t.Errorf("Expected 1 failed request, got %d", metrics.FailedRequests)
	}
}

// TestStreamingResponse tests streaming completions.
func TestStreamingResponse(t *testing.T) {
	registry := llm.NewRegistry()
	backend := newMockBackend("claude")
	registry.Register(backend)

	handler := NewSamplingHandler(registry, "claude")

	request := &SamplingRequest{
		Messages: []SamplingMessage{
			{Role: "user", Content: MessageContent{Type: "text", Text: "test"}},
		},
	}

	ctx := context.Background()
	streamChan, err := handler.(*samplingHandlerImpl).CreateMessageStream(ctx, request)

	if err != nil {
		t.Fatalf("CreateMessageStream failed: %v", err)
	}

	if streamChan == nil {
		t.Fatal("Stream channel is nil")
	}

	// Collect chunks
	var chunks []*SamplingStreamChunk
	for chunk := range streamChan {
		chunks = append(chunks, chunk)
	}

	if len(chunks) == 0 {
		t.Fatal("No chunks received")
	}

	// Verify final chunk has Done=true
	if !chunks[len(chunks)-1].Done {
		t.Error("Final chunk should have Done=true")
	}
}

// TestBackendSelection tests backend selection fallback logic.
func TestBackendSelection(t *testing.T) {
	registry := llm.NewRegistry()

	// Register only one backend
	backend := newMockBackend("gemini")
	backend.available = true
	registry.Register(backend)

	handler := NewSamplingHandler(registry, "gemini").(*samplingHandlerImpl)

	request := &SamplingRequest{
		Messages: []SamplingMessage{
			{Role: "user", Content: MessageContent{Type: "text", Text: "test"}},
		},
		ModelPreferences: &ModelPreferences{
			Hints: []ModelHint{
				{Name: "claude-3-sonnet"}, // Requested model not available
			},
		},
	}

	ctx := context.Background()
	selected := handler.selectBackend(ctx, request)

	if selected == nil {
		t.Fatal("Expected fallback to available backend")
	}

	if selected.Name() != "gemini" {
		t.Errorf("Expected gemini fallback, got %s", selected.Name())
	}
}

// TestMultipleBackends tests selection with multiple backends.
func TestMultipleBackends(t *testing.T) {
	registry := llm.NewRegistry()

	backend1 := newMockBackend("claude")
	backend2 := newMockBackend("gemini")
	backend3 := newMockBackend("openai")

	registry.Register(backend1)
	registry.Register(backend2)
	registry.Register(backend3)

	handler := NewSamplingHandler(registry, "claude")

	request := &SamplingRequest{
		Messages: []SamplingMessage{
			{Role: "user", Content: MessageContent{Type: "text", Text: "test"}},
		},
	}

	ctx := context.Background()
	response, err := handler.CreateMessage(ctx, request)

	if err != nil {
		t.Fatalf("CreateMessage failed: %v", err)
	}

	if response == nil {
		t.Fatal("Response is nil")
	}
}

// TestRequestToOptions tests conversion of SamplingRequest to llm.Options.
func TestRequestToOptions(t *testing.T) {
	registry := llm.NewRegistry()
	registry.Register(newMockBackend("claude"))

	handler := NewSamplingHandler(registry, "claude").(*samplingHandlerImpl)

	request := &SamplingRequest{
		Messages: []SamplingMessage{
			{Role: "user", Content: MessageContent{Type: "text", Text: "test"}},
		},
		MaxTokens:    500,
		SystemPrompt: "You are a helpful assistant",
		StopSequences: []string{"STOP", "END"},
		Metadata: map[string]any{
			"temperature": float32(0.7),
		},
	}

	opts := handler.requestToOptions(request)

	if opts.MaxTokens != 500 {
		t.Errorf("Expected MaxTokens=500, got %d", opts.MaxTokens)
	}

	if opts.SystemPrompt != "You are a helpful assistant" {
		t.Errorf("SystemPrompt mismatch: %s", opts.SystemPrompt)
	}

	if len(opts.StopSequences) != 2 {
		t.Errorf("Expected 2 stop sequences, got %d", len(opts.StopSequences))
	}
}

// TestResetMetrics tests metrics reset functionality.
func TestResetMetrics(t *testing.T) {
	registry := llm.NewRegistry()
	backend := newMockBackend("claude")
	registry.Register(backend)

	handler := NewSamplingHandler(registry, "claude")

	request := &SamplingRequest{
		Messages: []SamplingMessage{
			{Role: "user", Content: MessageContent{Type: "text", Text: "test"}},
		},
	}

	ctx := context.Background()
	handler.CreateMessage(ctx, request)

	implHandler := handler.(*samplingHandlerImpl)
	implHandler.ResetMetrics()

	allMetrics := implHandler.GetAllMetrics()
	if len(allMetrics) != 0 {
		t.Errorf("Expected no metrics after reset, got %d", len(allMetrics))
	}
}

// =============================================================================
// Test Helpers
// =============================================================================

var errTest = &Error{
	Code:    ErrorCodeInternalError,
	Message: "test error",
}
