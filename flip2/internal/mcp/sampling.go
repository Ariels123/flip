// Package mcp provides Model Context Protocol (MCP) server integration.
//
// This file implements the SamplingHandler interface for MCP-009 Sampling Support.
// It handles LLM completion requests from MCP servers and routes them to FLIP2's
// configured LLM backends.

package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"flip2/internal/llm"
)

// =============================================================================
// Sampling Handler Implementation
// =============================================================================

// samplingHandlerImpl implements the SamplingHandler interface.
//
// The sampling handler coordinates between MCP servers requesting completions
// and FLIP2's LLM backend registry. It:
//   - Accepts SamplingRequest messages from MCP servers
//   - Routes requests to the most appropriate LLM backend
//   - Supports streaming responses when available
//   - Handles sampling parameters (temperature, max_tokens, etc.)
//   - Manages cost tracking and quotas
type samplingHandlerImpl struct {
	// backendRegistry holds available LLM backends (claude, gemini, etc.)
	backendRegistry *llm.Registry

	// mu protects the metrics map
	mu sync.RWMutex

	// metrics tracks sampling statistics per server and model
	metrics map[string]*SamplingMetrics

	// defaultBackendName is the fallback backend if preferred model is unavailable
	defaultBackendName string

	// enableCostTracking enables cost accounting for sampling requests
	enableCostTracking bool

	// logger is used for diagnostics
	logger *log.Logger
}

// SamplingMetrics tracks sampling statistics for a particular model or server.
type SamplingMetrics struct {
	// TotalRequests is the number of sampling requests processed
	TotalRequests int64

	// SuccessfulRequests is the number of successful completions
	SuccessfulRequests int64

	// FailedRequests is the number of failed requests
	FailedRequests int64

	// TotalInputTokens is the aggregate input token count
	TotalInputTokens int64

	// TotalOutputTokens is the aggregate output token count
	TotalOutputTokens int64

	// TotalCostUSD is the aggregate cost
	TotalCostUSD float64

	// AverageLatency is the average request latency
	AverageLatency time.Duration

	// LastUpdated is when these metrics were last updated
	LastUpdated time.Time
}

// Compile-time check that samplingHandlerImpl implements SamplingHandler
var _ SamplingHandler = (*samplingHandlerImpl)(nil)

// NewSamplingHandler creates a new sampling handler with the given backend registry.
//
// The handler will use the registry to route completion requests to available
// LLM backends. If defaultBackend is empty, the handler uses the registry's
// best available backend.
func NewSamplingHandler(backendRegistry *llm.Registry, defaultBackend string) SamplingHandler {
	return &samplingHandlerImpl{
		backendRegistry:    backendRegistry,
		defaultBackendName: defaultBackend,
		enableCostTracking: true,
		metrics:            make(map[string]*SamplingMetrics),
		logger:             log.New(nil, "mcp/sampling: ", log.LstdFlags|log.Lshortfile),
	}
}

// CreateMessage implements SamplingHandler.CreateMessage.
//
// It processes an LLM completion request from an MCP server by:
//   1. Converting the SamplingRequest to a prompt string
//   2. Selecting the most appropriate backend based on preferences
//   3. Executing the request with sampling parameters
//   4. Converting the backend response back to SamplingResponse format
//   5. Tracking metrics for cost and usage
func (sh *samplingHandlerImpl) CreateMessage(ctx context.Context, request *SamplingRequest) (*SamplingResponse, error) {
	if request == nil {
		return nil, fmt.Errorf("sampling request is nil")
	}

	// Validate the request has at least one message
	if len(request.Messages) == 0 {
		return nil, fmt.Errorf("sampling request has no messages")
	}

	// Select the backend to use
	backend := sh.selectBackend(ctx, request)
	if backend == nil {
		return nil, fmt.Errorf("no available LLM backends for sampling")
	}

	// Convert MCP messages to a prompt string
	prompt := sh.messagesToPrompt(request.Messages)

	// Build options for the backend
	opts := sh.requestToOptions(request)

	// Execute the sampling request
	startTime := time.Now()
	response, err := backend.Execute(ctx, prompt, opts)
	if err != nil {
		sh.recordFailure(backend.Name(), request.ModelPreferences)
		return nil, fmt.Errorf("backend execution failed: %w", err)
	}

	// Record metrics
	duration := time.Since(startTime)
	sh.recordMetrics(backend.Name(), response, duration)

	// Convert backend response to SamplingResponse
	samplingResp := &SamplingResponse{
		Role: "assistant",
		Content: MessageContent{
			Type: "text",
			Text: response.Content,
		},
		Model:      response.Model,
		StopReason: response.FinishReason,
	}

	return samplingResp, nil
}

// selectBackend chooses the best LLM backend for a sampling request.
//
// Selection strategy:
//   1. If ModelPreferences.Hints contains specific models, try to match them
//   2. Otherwise, use the best available backend from the registry
//   3. Fall back to the configured default backend if needed
func (sh *samplingHandlerImpl) selectBackend(ctx context.Context, request *SamplingRequest) llm.Backend {
	// Check if model preferences specify a hint
	if request.ModelPreferences != nil && len(request.ModelPreferences.Hints) > 0 {
		for _, hint := range request.ModelPreferences.Hints {
			// Try to match hint to a backend
			// For now, we use simple heuristics:
			// "claude-3-*" -> "claude"
			// "gemini-*" -> "gemini"
			// "gpt-*" -> other backends

			backendName := sh.inferBackendFromModel(hint.Name)
			if backend, ok := sh.backendRegistry.Get(backendName); ok && backend.IsAvailable(ctx) {
				return backend
			}
		}
	}

	// Use model preferences to rank backends
	if request.ModelPreferences != nil {
		// If cost priority is highest, prefer cheaper backends (gemini)
		if request.ModelPreferences.CostPriority > 0.6 {
			if backend, ok := sh.backendRegistry.Get("gemini"); ok && backend.IsAvailable(ctx) {
				return backend
			}
		}

		// If intelligence priority is highest, prefer powerful models (claude)
		if request.ModelPreferences.IntelligencePriority > 0.6 {
			if backend, ok := sh.backendRegistry.Get("claude"); ok && backend.IsAvailable(ctx) {
				return backend
			}
		}
	}

	// Fall back to best available backend
	if available := sh.backendRegistry.GetAvailable(ctx); len(available) > 0 {
		return available[0]
	}

	// Last resort: use configured default if available
	if sh.defaultBackendName != "" {
		if backend, ok := sh.backendRegistry.Get(sh.defaultBackendName); ok {
			return backend
		}
	}

	return nil
}

// inferBackendFromModel infers the backend name from a model identifier.
func (sh *samplingHandlerImpl) inferBackendFromModel(modelName string) string {
	// Simple heuristics to map model names to backends
	switch {
	case modelName == "claude-3-opus", modelName == "claude-3-sonnet", modelName == "claude-3-haiku":
		return "claude"
	case modelName == "gemini-2.0-flash", modelName == "gemini-1.5-pro":
		return "gemini"
	case modelName == "gpt-4", modelName == "gpt-3.5-turbo":
		return "openai"
	default:
		return "claude" // Default fallback
	}
}

// messagesToPrompt converts MCP messages to a single prompt string.
//
// This is a simple implementation that concatenates messages.
// A more sophisticated version could use message templates or
// special formatting for multi-turn conversations.
func (sh *samplingHandlerImpl) messagesToPrompt(messages []SamplingMessage) string {
	var result string

	for _, msg := range messages {
		role := msg.Role
		if role == "user" {
			role = "User"
		} else if role == "assistant" {
			role = "Assistant"
		}

		// Add the message content
		switch msg.Content.Type {
		case "text":
			result += fmt.Sprintf("%s: %s\n", role, msg.Content.Text)
		case "image":
			result += fmt.Sprintf("%s: [Image: %s]\n", role, msg.Content.MimeType)
		case "audio":
			result += fmt.Sprintf("%s: [Audio: %s]\n", role, msg.Content.MimeType)
		}
	}

	return result
}

// requestToOptions converts a SamplingRequest to llm.Options for backend execution.
func (sh *samplingHandlerImpl) requestToOptions(request *SamplingRequest) *llm.Options {
	opts := &llm.Options{
		MaxTokens:    request.MaxTokens,
		SystemPrompt: request.SystemPrompt,
		StopSequences: request.StopSequences,
	}

	// Extract temperature from metadata if present
	if request.Metadata != nil {
		if temp, ok := request.Metadata["temperature"].(float32); ok {
			opts.Temperature = temp
		} else if tempFloat64, ok := request.Metadata["temperature"].(float64); ok {
			opts.Temperature = float32(tempFloat64)
		}
	}

	// If no stop sequences in metadata, use default
	if len(opts.StopSequences) == 0 {
		opts.StopSequences = request.StopSequences
	}

	return opts
}

// recordMetrics updates sampling statistics after a successful request.
func (sh *samplingHandlerImpl) recordMetrics(backendName string, response *llm.Response, duration time.Duration) {
	sh.mu.Lock()
	defer sh.mu.Unlock()

	key := backendName
	metrics, exists := sh.metrics[key]

	if !exists {
		metrics = &SamplingMetrics{}
		sh.metrics[key] = metrics
	}

	metrics.TotalRequests++
	metrics.SuccessfulRequests++
	metrics.TotalInputTokens += int64(response.InputTokens)
	metrics.TotalOutputTokens += int64(response.OutputTokens)
	metrics.TotalCostUSD += response.CostUSD

	// Update average latency (simple running average)
	if metrics.AverageLatency == 0 {
		metrics.AverageLatency = duration
	} else {
		metrics.AverageLatency = time.Duration(
			(int64(metrics.AverageLatency)*(metrics.TotalRequests-1) + int64(duration)) / metrics.TotalRequests,
		)
	}

	metrics.LastUpdated = time.Now()
}

// recordFailure updates metrics for a failed request.
func (sh *samplingHandlerImpl) recordFailure(backendName string, prefs *ModelPreferences) {
	sh.mu.Lock()
	defer sh.mu.Unlock()

	key := backendName
	metrics, exists := sh.metrics[key]

	if !exists {
		metrics = &SamplingMetrics{}
		sh.metrics[key] = metrics
	}

	metrics.TotalRequests++
	metrics.FailedRequests++
	metrics.LastUpdated = time.Now()
}

// GetMetrics returns sampling metrics for a specific backend.
func (sh *samplingHandlerImpl) GetMetrics(backendName string) *SamplingMetrics {
	sh.mu.RLock()
	defer sh.mu.RUnlock()

	if metrics, ok := sh.metrics[backendName]; ok {
		// Return a copy to prevent external modifications
		copy := *metrics
		return &copy
	}

	return nil
}

// GetAllMetrics returns all sampling metrics.
func (sh *samplingHandlerImpl) GetAllMetrics() map[string]*SamplingMetrics {
	sh.mu.RLock()
	defer sh.mu.RUnlock()

	result := make(map[string]*SamplingMetrics)
	for key, metrics := range sh.metrics {
		copy := *metrics
		result[key] = &copy
	}

	return result
}

// ResetMetrics clears all sampling metrics.
func (sh *samplingHandlerImpl) ResetMetrics() {
	sh.mu.Lock()
	defer sh.mu.Unlock()

	sh.metrics = make(map[string]*SamplingMetrics)
}

// =============================================================================
// Streaming Support
// =============================================================================

// CreateMessageStream sends a sampling request with streaming response.
//
// This is an extension to the SamplingHandler interface that supports
// streaming responses from backends that support it.
func (sh *samplingHandlerImpl) CreateMessageStream(
	ctx context.Context,
	request *SamplingRequest,
) (<-chan *SamplingStreamChunk, error) {
	if request == nil {
		return nil, fmt.Errorf("sampling request is nil")
	}

	if len(request.Messages) == 0 {
		return nil, fmt.Errorf("sampling request has no messages")
	}

	// Select backend
	backend := sh.selectBackend(ctx, request)
	if backend == nil {
		return nil, fmt.Errorf("no available LLM backends for sampling")
	}

	// Convert to prompt
	prompt := sh.messagesToPrompt(request.Messages)
	opts := sh.requestToOptions(request)

	// Get streaming channel from backend
	streamChan, err := backend.Stream(ctx, prompt, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to start streaming: %w", err)
	}

	// Wrap backend stream channel to convert types
	outputChan := make(chan *SamplingStreamChunk, 1)

	go func() {
		defer close(outputChan)

		var totalInputTokens int
		var totalOutputTokens int
		var totalCost float64

		for chunk := range streamChan {
			// Convert chunk to SamplingStreamChunk
			samplingChunk := &SamplingStreamChunk{
				Content:        chunk.Text,
				Done:           chunk.Done,
				InputTokens:    chunk.InputTokens,
				OutputTokens:   chunk.OutputTokens,
				ErrorMessage:   "",
			}

			if chunk.Error != nil {
				samplingChunk.ErrorMessage = chunk.Error.Error()
			}

			select {
			case outputChan <- samplingChunk:
				totalInputTokens += chunk.InputTokens
				totalOutputTokens += chunk.OutputTokens
			case <-ctx.Done():
				return
			}

			// Record metrics on final chunk
			if chunk.Done {
				sh.mu.Lock()
				metrics := sh.metrics[backend.Name()]
				if metrics != nil {
					metrics.TotalInputTokens += int64(totalInputTokens)
					metrics.TotalOutputTokens += int64(totalOutputTokens)
					metrics.TotalCostUSD += totalCost
				}
				sh.mu.Unlock()
			}
		}
	}()

	return outputChan, nil
}

// SamplingStreamChunk represents a streamed chunk from sampling.
type SamplingStreamChunk struct {
	// Content is the incremental text content
	Content string

	// Done indicates this is the final chunk
	Done bool

	// InputTokens is the input token count (only on final chunk)
	InputTokens int

	// OutputTokens is the output token count (only on final chunk)
	OutputTokens int

	// ErrorMessage is set if an error occurred
	ErrorMessage string
}

// =============================================================================
// Helper Functions
// =============================================================================

// SamplingRequestToJSON converts a sampling request to JSON for logging/storage.
func SamplingRequestToJSON(request *SamplingRequest) string {
	data, err := json.MarshalIndent(request, "", "  ")
	if err != nil {
		return fmt.Sprintf("{error: %v}", err)
	}
	return string(data)
}

// SamplingResponseToJSON converts a sampling response to JSON.
func SamplingResponseToJSON(response *SamplingResponse) string {
	data, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return fmt.Sprintf("{error: %v}", err)
	}
	return string(data)
}
