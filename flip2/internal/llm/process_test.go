package llm

import (
	"context"
	"os/exec"
	"testing"
	"time"
)

func TestNewProcessBackend(t *testing.T) {
	cfg := ProcessBackendConfig{
		Name:         "test-backend",
		Command:      "echo",
		DefaultArgs:  []string{"-n"},
		Models:       []string{"model-a", "model-b"},
		DefaultModel: "model-a",
		Timeout:      30 * time.Second,
	}

	backend := NewProcessBackend(cfg)

	if backend.Name() != "test-backend" {
		t.Errorf("expected name 'test-backend', got '%s'", backend.Name())
	}

	if backend.DefaultModel() != "model-a" {
		t.Errorf("expected default model 'model-a', got '%s'", backend.DefaultModel())
	}

	models := backend.Models()
	if len(models) != 2 {
		t.Errorf("expected 2 models, got %d", len(models))
	}
}

func TestNewClaudeBackend(t *testing.T) {
	backend := NewClaudeBackend()

	if backend.Name() != "claude" {
		t.Errorf("expected name 'claude', got '%s'", backend.Name())
	}

	if backend.command != "claude" {
		t.Errorf("expected command 'claude', got '%s'", backend.command)
	}

	if backend.DefaultModel() != "claude-sonnet-4" {
		t.Errorf("expected default model 'claude-sonnet-4', got '%s'", backend.DefaultModel())
	}
}

func TestNewGeminiBackend(t *testing.T) {
	backend := NewGeminiBackend()

	if backend.Name() != "gemini" {
		t.Errorf("expected name 'gemini', got '%s'", backend.Name())
	}

	if backend.command != "gemini" {
		t.Errorf("expected command 'gemini', got '%s'", backend.command)
	}
}

func TestNewAntigravityBackend(t *testing.T) {
	backend := NewAntigravityBackend()

	if backend.Name() != "antigravity" {
		t.Errorf("expected name 'antigravity', got '%s'", backend.Name())
	}

	// Antigravity uses gemini CLI
	if backend.command != "gemini" {
		t.Errorf("expected command 'gemini', got '%s'", backend.command)
	}
}

func TestProcessBackend_Execute_WithEcho(t *testing.T) {
	// Test with echo command which should always be available
	backend := NewProcessBackend(ProcessBackendConfig{
		Name:         "echo-test",
		Command:      "echo",
		DefaultArgs:  []string{},
		Models:       []string{"echo-v1"},
		DefaultModel: "echo-v1",
		Timeout:      5 * time.Second,
	})

	ctx := context.Background()
	resp, err := backend.Execute(ctx, "hello world", nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Content != "hello world" {
		t.Errorf("expected content 'hello world', got '%s'", resp.Content)
	}

	if resp.Model != "echo-v1" {
		t.Errorf("expected model 'echo-v1', got '%s'", resp.Model)
	}

	if resp.Latency <= 0 {
		t.Error("expected positive latency")
	}
}

func TestProcessBackend_Execute_Timeout(t *testing.T) {
	// Test timeout with sleep command
	backend := NewProcessBackend(ProcessBackendConfig{
		Name:         "timeout-test",
		Command:      "sleep",
		DefaultArgs:  []string{},
		Models:       []string{"sleep-v1"},
		DefaultModel: "sleep-v1",
		Timeout:      100 * time.Millisecond,
	})

	ctx := context.Background()
	_, err := backend.Execute(ctx, "10", nil)

	if err == nil {
		t.Error("expected timeout error, got nil")
	}
}

func TestProcessBackend_IsAvailable(t *testing.T) {
	// Test with a command that exists (echo)
	backend := NewProcessBackend(ProcessBackendConfig{
		Name:    "echo-test",
		Command: "echo",
	})

	ctx := context.Background()
	if !backend.IsAvailable(ctx) {
		t.Error("expected echo to be available")
	}

	// Test with a command that doesn't exist
	backend2 := NewProcessBackend(ProcessBackendConfig{
		Name:    "nonexistent-test",
		Command: "this-command-does-not-exist-12345",
	})

	if backend2.IsAvailable(ctx) {
		t.Error("expected nonexistent command to be unavailable")
	}
}

func TestProcessBackend_BuildArgs(t *testing.T) {
	backend := NewClaudeBackend()

	// Test with no options
	args := backend.buildArgs(nil)
	if len(args) != 1 || args[0] != "-p" {
		t.Errorf("expected [-p], got %v", args)
	}

	// Test with model option
	opts := &Options{Model: "claude-opus-4"}
	args = backend.buildArgs(opts)

	hasModel := false
	for i, arg := range args {
		if arg == "--model" && i+1 < len(args) && args[i+1] == "claude-opus-4" {
			hasModel = true
			break
		}
	}
	if !hasModel {
		t.Errorf("expected --model claude-opus-4 in args, got %v", args)
	}

	// Test with system prompt
	opts2 := &Options{SystemPrompt: "You are helpful"}
	args = backend.buildArgs(opts2)

	hasSystem := false
	for i, arg := range args {
		if arg == "--system" && i+1 < len(args) && args[i+1] == "You are helpful" {
			hasSystem = true
			break
		}
	}
	if !hasSystem {
		t.Errorf("expected --system in args, got %v", args)
	}
}

func TestProcessBackend_EstimateTokens(t *testing.T) {
	backend := NewClaudeBackend()

	input := "Hello, world!"    // 13 chars -> ~3 tokens
	output := "Hi there, human!" // 16 chars -> ~4 tokens

	inputTokens, outputTokens := backend.estimateTokens(input, output)

	if inputTokens < 1 {
		t.Errorf("expected at least 1 input token, got %d", inputTokens)
	}

	if outputTokens < 1 {
		t.Errorf("expected at least 1 output token, got %d", outputTokens)
	}
}

func TestProcessBackend_CalculateCost(t *testing.T) {
	backend := NewClaudeBackend()

	// claude-sonnet-4: $3/1M input, $15/1M output
	cost := backend.calculateCost("claude-sonnet-4", 1000, 500)

	// Expected: (1000 * 3 / 1M) + (500 * 15 / 1M) = 0.003 + 0.0075 = 0.0105
	expectedCost := 0.0105
	if cost < expectedCost-0.001 || cost > expectedCost+0.001 {
		t.Errorf("expected cost ~%.4f, got %.4f", expectedCost, cost)
	}
}

func TestProcessBackend_CheckQuota(t *testing.T) {
	backend := NewClaudeBackend()

	ctx := context.Background()
	quota, err := backend.CheckQuota(ctx)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if quota != 1.0 {
		t.Errorf("expected quota 1.0, got %f", quota)
	}

	// Test quota exhausted state
	backend.markQuotaExhausted()

	quota, err = backend.CheckQuota(ctx)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if quota != 0.0 {
		t.Errorf("expected quota 0.0 after exhaustion, got %f", quota)
	}
}

func TestProcessBackend_IsQuotaError(t *testing.T) {
	backend := NewClaudeBackend()

	testCases := []struct {
		msg      string
		expected bool
	}{
		{"quota exceeded", true},
		{"Rate limit reached", true},
		{"429 Too Many Requests", true},
		{"limit exceeded for this minute", true},
		{"normal error", false},
		{"connection failed", false},
	}

	for _, tc := range testCases {
		result := backend.isQuotaError(tc.msg)
		if result != tc.expected {
			t.Errorf("isQuotaError(%q) = %v, expected %v", tc.msg, result, tc.expected)
		}
	}
}

func TestRegistry(t *testing.T) {
	registry := NewRegistry()

	// Register test backends
	backend1 := NewProcessBackend(ProcessBackendConfig{
		Name:    "test-1",
		Command: "echo",
	})
	backend2 := NewProcessBackend(ProcessBackendConfig{
		Name:    "test-2",
		Command: "echo",
	})

	registry.Register(backend1)
	registry.Register(backend2)

	// Test Get
	b, ok := registry.Get("test-1")
	if !ok {
		t.Error("expected to find test-1")
	}
	if b.Name() != "test-1" {
		t.Errorf("expected name 'test-1', got '%s'", b.Name())
	}

	// Test List
	list := registry.List()
	if len(list) != 2 {
		t.Errorf("expected 2 backends, got %d", len(list))
	}

	// Test GetAvailable
	ctx := context.Background()
	available := registry.GetAvailable(ctx)
	if len(available) != 2 {
		t.Errorf("expected 2 available backends, got %d", len(available))
	}
}

func TestRegistry_GetBestAvailable(t *testing.T) {
	registry := NewRegistry()

	// Only register echo-based backends for testing
	claudeBackend := NewProcessBackend(ProcessBackendConfig{
		Name:    "claude",
		Command: "echo",
	})
	geminiBackend := NewProcessBackend(ProcessBackendConfig{
		Name:    "gemini",
		Command: "echo",
	})

	registry.Register(claudeBackend)
	registry.Register(geminiBackend)

	ctx := context.Background()

	// Test code task routing
	best := registry.GetBestAvailable(ctx, "code")
	if best == nil {
		t.Error("expected a backend for code task")
	}

	// Test data task routing (should prefer gemini)
	best = registry.GetBestAvailable(ctx, "data")
	if best == nil {
		t.Error("expected a backend for data task")
	}
	if best.Name() != "gemini" {
		t.Errorf("expected gemini for data task, got %s", best.Name())
	}
}

func TestParseTokenCounts(t *testing.T) {
	// Test JSON format
	jsonOutput := `{"input_tokens": 100, "output_tokens": 50}`
	input, output, found := ParseTokenCounts(jsonOutput)
	if !found {
		t.Error("expected to find tokens in JSON format")
	}
	if input != 100 || output != 50 {
		t.Errorf("expected 100/50, got %d/%d", input, output)
	}

	// Test pattern format
	patternOutput := "Processed 200 input and 100 output tokens"
	input, output, found = ParseTokenCounts(patternOutput)
	if !found {
		t.Error("expected to find tokens in pattern format")
	}
	if input != 200 || output != 100 {
		t.Errorf("expected 200/100, got %d/%d", input, output)
	}

	// Test no match
	noMatch := "Just some regular text"
	_, _, found = ParseTokenCounts(noMatch)
	if found {
		t.Error("expected not to find tokens in regular text")
	}
}

func TestCommandExists(t *testing.T) {
	// echo should exist on all Unix-like systems
	if !CommandExists("echo") {
		t.Error("expected echo to exist")
	}

	// Nonexistent command
	if CommandExists("this-command-does-not-exist-99999") {
		t.Error("expected nonexistent command to not exist")
	}
}

func TestGetAvailableBackends(t *testing.T) {
	available := GetAvailableBackends()

	// We can't know exactly which backends are available,
	// but we can check the function runs without error
	t.Logf("Available backends: %v", available)

	// If claude exists, it should appear twice (claude and claude-code)
	if CommandExists("claude") {
		foundClaude := false
		foundClaudeCode := false
		for _, name := range available {
			if name == "claude" {
				foundClaude = true
			}
			if name == "claude-code" {
				foundClaudeCode = true
			}
		}
		if !foundClaude || !foundClaudeCode {
			t.Error("expected both claude and claude-code when claude CLI exists")
		}
	}
}

// Integration test - only runs if claude CLI is available
func TestProcessBackend_Integration_Claude(t *testing.T) {
	if _, err := exec.LookPath("claude"); err != nil {
		t.Skip("claude CLI not available, skipping integration test")
	}

	backend := NewClaudeCodeBackend()
	ctx := context.Background()

	if !backend.IsAvailable(ctx) {
		t.Skip("claude backend not available")
	}

	// Simple test prompt
	resp, err := backend.Execute(ctx, "Reply with just the word 'hello' and nothing else", &Options{
		Timeout: 60 * time.Second,
	})

	if err != nil {
		t.Fatalf("integration test failed: %v", err)
	}

	t.Logf("Response: %s", resp.Content)
	t.Logf("Model: %s", resp.Model)
	t.Logf("Latency: %v", resp.Latency)
	t.Logf("Estimated tokens: %d input, %d output", resp.InputTokens, resp.OutputTokens)
	t.Logf("Estimated cost: $%.6f", resp.CostUSD)
}

// Integration test - only runs if gemini CLI is available
func TestProcessBackend_Integration_Gemini(t *testing.T) {
	if _, err := exec.LookPath("gemini"); err != nil {
		t.Skip("gemini CLI not available, skipping integration test")
	}

	backend := NewGeminiBackend()
	ctx := context.Background()

	if !backend.IsAvailable(ctx) {
		t.Skip("gemini backend not available")
	}

	// Simple test prompt
	resp, err := backend.Execute(ctx, "Reply with just the word 'hello' and nothing else", &Options{
		Timeout: 60 * time.Second,
	})

	if err != nil {
		t.Fatalf("integration test failed: %v", err)
	}

	t.Logf("Response: %s", resp.Content)
	t.Logf("Model: %s", resp.Model)
	t.Logf("Latency: %v", resp.Latency)
}
