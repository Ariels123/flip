// Package llm provides a unified interface for AI backend providers.
//
// This package implements a strategy pattern where different backend types
// (ProcessBackend for CLI tools via stdin/stdout, HTTPBackend for remote APIs)
// implement a common interface. This allows the executor to work with
// any backend without knowing the underlying transport mechanism.
//
// Ported from FLIP v1 pkg/backends with adaptations for FLIP2.
package llm

import (
	"context"
	"time"
)

// Response contains the result from any backend execution.
type Response struct {
	// Content is the generated text response
	Content string `json:"content"`

	// Model is the model identifier used for this request
	Model string `json:"model"`

	// InputTokens is the actual token count from the API response
	InputTokens int `json:"input_tokens"`

	// OutputTokens is the actual output token count from the API
	OutputTokens int `json:"output_tokens"`

	// CostUSD is the calculated cost based on model pricing
	CostUSD float64 `json:"cost_usd"`

	// FinishReason indicates why generation stopped
	// Common values: "stop", "max_tokens", "error"
	FinishReason string `json:"finish_reason"`

	// Metadata contains provider-specific data
	Metadata map[string]any `json:"metadata,omitempty"`

	// Latency is the total request duration
	Latency time.Duration `json:"latency"`

	// Stderr captures any stderr output from CLI execution
	Stderr string `json:"stderr,omitempty"`
}

// StreamChunk represents a piece of a streaming response.
type StreamChunk struct {
	// Text is the incremental text content
	Text string `json:"text,omitempty"`

	// Done indicates this is the final chunk
	Done bool `json:"done"`

	// InputTokens is only populated on the final chunk
	InputTokens int `json:"input_tokens,omitempty"`

	// OutputTokens is only populated on the final chunk
	OutputTokens int `json:"output_tokens,omitempty"`

	// Error is set if an error occurred during streaming
	Error error `json:"error,omitempty"`
}

// Options configures a backend execution request.
type Options struct {
	// Model overrides the default model for this backend
	Model string `json:"model,omitempty"`

	// MaxTokens limits the output token count
	MaxTokens int `json:"max_tokens,omitempty"`

	// Temperature controls randomness (0.0 = deterministic, 1.0 = creative)
	Temperature float32 `json:"temperature,omitempty"`

	// SystemPrompt provides context or instructions
	SystemPrompt string `json:"system_prompt,omitempty"`

	// StopSequences causes generation to stop when encountered
	StopSequences []string `json:"stop_sequences,omitempty"`

	// Timeout overrides the default request timeout
	Timeout time.Duration `json:"timeout,omitempty"`

	// Metadata is passed through to the backend
	Metadata map[string]string `json:"metadata,omitempty"`

	// DangerouslySkipPermissions bypasses permission checks (for claude-code)
	DangerouslySkipPermissions bool `json:"dangerously_skip_permissions,omitempty"`
}

// Backend is the interface that all backend implementations must satisfy.
type Backend interface {
	// Name returns the backend identifier (e.g., "claude", "gemini", "antigravity")
	Name() string

	// Execute sends a prompt and waits for the complete response.
	Execute(ctx context.Context, prompt string, opts *Options) (*Response, error)

	// Stream sends a prompt and returns a channel of response chunks.
	Stream(ctx context.Context, prompt string, opts *Options) (<-chan StreamChunk, error)

	// CheckQuota returns available capacity as a 0-1 scale.
	// Returns -1 if quota status is unknown, 0 if exhausted.
	CheckQuota(ctx context.Context) (float64, error)

	// Models returns the list of available models for this backend
	Models() []string

	// DefaultModel returns the default model identifier
	DefaultModel() string

	// IsAvailable checks if the backend is currently operational
	IsAvailable(ctx context.Context) bool
}

// ModelConfig contains pricing and limits for a specific model.
type ModelConfig struct {
	// ID is the model identifier used in API calls
	ID string `json:"id"`

	// DisplayName is the human-readable name
	DisplayName string `json:"display_name"`

	// InputCostPer1M is the cost per 1 million input tokens
	InputCostPer1M float64 `json:"input_cost_per_1m"`

	// OutputCostPer1M is the cost per 1 million output tokens
	OutputCostPer1M float64 `json:"output_cost_per_1m"`

	// MaxTokens is the maximum output token limit
	MaxTokens int `json:"max_tokens"`

	// ContextWindow is the maximum context size in tokens
	ContextWindow int `json:"context_window"`
}

// Registry manages available backends and provides routing.
type Registry struct {
	backends map[string]Backend
}

// NewRegistry creates a new backend registry.
func NewRegistry() *Registry {
	return &Registry{
		backends: make(map[string]Backend),
	}
}

// Register adds a backend to the registry.
func (r *Registry) Register(backend Backend) {
	r.backends[backend.Name()] = backend
}

// Get retrieves a backend by name.
func (r *Registry) Get(name string) (Backend, bool) {
	b, ok := r.backends[name]
	return b, ok
}

// List returns all registered backend names.
func (r *Registry) List() []string {
	names := make([]string, 0, len(r.backends))
	for name := range r.backends {
		names = append(names, name)
	}
	return names
}

// GetAvailable returns backends that are currently operational.
func (r *Registry) GetAvailable(ctx context.Context) []Backend {
	available := make([]Backend, 0)
	for _, b := range r.backends {
		if b.IsAvailable(ctx) {
			available = append(available, b)
		}
	}
	return available
}

// GetBestAvailable returns the best available backend based on task type.
// Routing logic:
//   - Code tasks: prefer claude-code
//   - Analysis/data tasks: prefer gemini (cheaper)
//   - Complex reasoning: prefer claude
//   - Visual/browser: prefer antigravity
func (r *Registry) GetBestAvailable(ctx context.Context, taskType string) Backend {
	available := r.GetAvailable(ctx)
	if len(available) == 0 {
		return nil
	}

	// Priority order based on task type
	var priorities []string
	switch taskType {
	case "code", "debug", "refactor":
		priorities = []string{"claude-code", "claude", "gemini"}
	case "data", "parse", "transform":
		priorities = []string{"gemini", "claude", "antigravity"}
	case "visual", "browser", "test":
		priorities = []string{"antigravity", "gemini", "claude"}
	case "research", "analyze":
		priorities = []string{"gemini", "claude", "antigravity"}
	default:
		priorities = []string{"claude", "gemini", "antigravity"}
	}

	// Find first available in priority order
	for _, name := range priorities {
		for _, b := range available {
			if b.Name() == name {
				return b
			}
		}
	}

	// Return any available backend
	return available[0]
}
