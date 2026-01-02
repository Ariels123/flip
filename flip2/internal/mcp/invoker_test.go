package mcp

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// mockFailingServer is a server that fails on demand
type mockFailingServer struct {
	*mockServer
	failCount   int32
	failUntil   int32
	failOnCall  string
}

func newMockFailingServer(name string) *mockFailingServer {
	return &mockFailingServer{
		mockServer: newMockServer(name, "1.0.0"),
	}
}

func (m *mockFailingServer) CallTool(ctx context.Context, name string, arguments map[string]any) (*ToolResult, error) {
	count := atomic.AddInt32(&m.failCount, 1)
	if name == m.failOnCall && count <= m.failUntil {
		return &ToolResult{
			Content: []ContentItem{{Type: "text", Text: "error"}},
			IsError: true,
		}, &Error{Code: ErrorCodeInternalError, Message: "temporary error"}
	}
	return &ToolResult{Content: []ContentItem{{Type: "text", Text: "success"}}}, nil
}

// mockTimeoutServer is a server that always times out
type mockTimeoutServer struct {
	*mockServer
	delay time.Duration
}

func newMockTimeoutServer(name string, delay time.Duration) *mockTimeoutServer {
	return &mockTimeoutServer{
		mockServer: newMockServer(name, "1.0.0"),
		delay:      delay,
	}
}

func (m *mockTimeoutServer) CallTool(ctx context.Context, name string, arguments map[string]any) (*ToolResult, error) {
	select {
	case <-time.After(m.delay):
		return &ToolResult{Content: []ContentItem{{Type: "text", Text: "result"}}}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// TestInvokeToolSuccess tests successful tool invocation
func TestInvokeToolSuccess(t *testing.T) {
	ctx := context.Background()
	registry := NewRegistry()
	server := newMockServer("fs", "1.0.0"); server.capabilities.Tools = &ToolsCapability{}; server.capabilities.Resources = &ResourcesCapability{}
	server.tools = []Tool{{Name: "read_file", Description: "Read file"}}
	if err := registry.Register(ctx, server); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	invoker := NewToolInvoker(registry)
	defer invoker.Close()

	result, err := invoker.InvokeTool(context.Background(), "read_file", map[string]any{"path": "/tmp/test"})
	if err != nil {
		t.Fatalf("InvokeTool failed: %v", err)
	}
	if result == nil {
		t.Fatal("result is nil")
	}
	if result.IsError {
		t.Error("result should not be marked as error")
	}
}

// TestInvokeToolNotFound tests tool not found error
func TestInvokeToolNotFound(t *testing.T) {
	registry := NewRegistry()
	invoker := NewToolInvoker(registry)
	defer invoker.Close()

	_, err := invoker.InvokeTool(context.Background(), "nonexistent", map[string]any{})
	if err == nil {
		t.Fatal("should return error for nonexistent tool")
	}
}

// TestInvokeToolOnServer tests tool invocation on specific server
func TestInvokeToolOnServer(t *testing.T) {
	ctx := context.Background()
	registry := NewRegistry()
	server := newMockServer("db", "1.0.0"); server.capabilities.Tools = &ToolsCapability{}; server.capabilities.Resources = &ResourcesCapability{}
	server.tools = []Tool{{Name: "query", Description: "Query"}}
	if err := registry.Register(ctx, server); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	invoker := NewToolInvoker(registry)
	defer invoker.Close()

	result, err := invoker.InvokeToolOnServer(context.Background(), "db", "query", map[string]any{})
	if err != nil {
		t.Fatalf("InvokeToolOnServer failed: %v", err)
	}
	if result == nil {
		t.Fatal("result is nil")
	}
}

// TestInvokeToolAsyncSuccess tests async tool invocation
func TestInvokeToolAsyncSuccess(t *testing.T) {
	ctx := context.Background()
	registry := NewRegistry()
	server := newMockServer("fs", "1.0.0"); server.capabilities.Tools = &ToolsCapability{}; server.capabilities.Resources = &ResourcesCapability{}
	server.tools = []Tool{{Name: "write_file", Description: "Write"}}
	if err := registry.Register(ctx, server); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	invoker := NewToolInvoker(registry)
	defer invoker.Close()

	done := make(chan bool, 1)
	callback := func(result *ToolResult, err error) {
		done <- (result != nil && err == nil)
	}

	requestID := invoker.InvokeToolAsync(context.Background(), "write_file", map[string]any{}, callback)
	if requestID == "" {
		t.Fatal("requestID should not be empty")
	}

	select {
	case success := <-done:
		if !success {
			t.Fatal("callback should have been called with success")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("callback was not called")
	}
}

// TestInvokeWithRetrySuccess tests successful retry invocation
func TestInvokeWithRetrySuccess(t *testing.T) {
	ctx := context.Background()
	registry := NewRegistry()
	server := newMockServer("fs", "1.0.0"); server.capabilities.Tools = &ToolsCapability{}; server.capabilities.Resources = &ResourcesCapability{}
	server.tools = []Tool{{Name: "read", Description: "Read"}}
	if err := registry.Register(ctx, server); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	invoker := NewToolInvoker(registry)
	defer invoker.Close()

	opts := &RetryOptions{
		MaxAttempts:       3,
		BackoffMultiplier: 2.0,
		InitialDelay:      10 * time.Millisecond,
	}

	result, err := invoker.InvokeWithRetry(context.Background(), "read", map[string]any{}, opts)
	if err != nil {
		t.Fatalf("InvokeWithRetry failed: %v", err)
	}
	if result == nil {
		t.Fatal("result is nil")
	}
	// Result used for assertion above
	_ = result
}

// TestInvokeWithRetryTransientFailure tests retry on transient failure
func TestInvokeWithRetryTransientFailure(t *testing.T) {
	ctx := context.Background()
	registry := NewRegistry()
	server := newMockFailingServer("fs"); server.capabilities.Tools = &ToolsCapability{}; server.capabilities.Resources = &ResourcesCapability{}
	server.failOnCall = "read"
	server.failUntil = 1 // Fail first, succeed second
	server.tools = []Tool{{Name: "read", Description: "Read"}}
	if err := registry.Register(ctx, server); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	invoker := NewToolInvoker(registry)
	defer invoker.Close()

	opts := &RetryOptions{
		MaxAttempts:       3,
		BackoffMultiplier: 2.0,
		InitialDelay:      10 * time.Millisecond,
	}

	result, err := invoker.InvokeWithRetry(context.Background(), "read", map[string]any{}, opts)
	if err != nil {
		t.Fatalf("InvokeWithRetry should succeed after retry: %v", err)
	}
	if result == nil {
		t.Fatal("result is nil")
	}
	if result.IsError {
		t.Error("result should not be marked as error after retry")
	}
}

// TestInvokeWithRetryMaxAttemptsExhausted tests exhausted retries
func TestInvokeWithRetryMaxAttemptsExhausted(t *testing.T) {
	ctx := context.Background()
	registry := NewRegistry()
	server := newMockFailingServer("fs"); server.capabilities.Tools = &ToolsCapability{}; server.capabilities.Resources = &ResourcesCapability{}
	server.failOnCall = "read"
	server.failUntil = 100 // Always fail
	server.tools = []Tool{{Name: "read", Description: "Read"}}
	if err := registry.Register(ctx, server); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	invoker := NewToolInvoker(registry)
	defer invoker.Close()

	opts := &RetryOptions{
		MaxAttempts:       2,
		BackoffMultiplier: 2.0,
		InitialDelay:      5 * time.Millisecond,
	}

	result, err := invoker.InvokeWithRetry(context.Background(), "read", map[string]any{}, opts)
	_ = result // Unused in error case
	if err == nil {
		t.Fatal("should return error after exhausting retries")
	}
}

// TestInvokeWithFallbackSuccess tests successful fallback
func TestInvokeWithFallbackSuccess(t *testing.T) {
	ctx := context.Background()
	registry := NewRegistry()
	server := newMockServer("primary", "1.0.0"); server.capabilities.Tools = &ToolsCapability{}; server.capabilities.Resources = &ResourcesCapability{}
	server.tools = []Tool{{Name: "read", Description: "Read"}}
	if err := registry.Register(ctx, server); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	invoker := NewToolInvoker(registry)
	defer invoker.Close()

	result, err := invoker.InvokeWithFallback(context.Background(), "read", map[string]any{}, []string{})
	if err != nil {
		t.Fatalf("InvokeWithFallback failed: %v", err)
	}
	if result == nil {
		t.Fatal("result is nil")
	}
}

// TestInvokeWithFallbackPrimaryFails tests fallback when primary fails
func TestInvokeWithFallbackPrimaryFails(t *testing.T) {
	ctx := context.Background()
	registry := NewRegistry()

	primary := newMockFailingServer("primary")
	primary.failOnCall = "read"
	primary.failUntil = 100
	primary.tools = []Tool{{Name: "read", Description: "Read"}}
	if err := registry.Register(ctx, primary); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	fallback := newMockServer("fallback", "1.0.0")
	fallback.tools = []Tool{{Name: "read", Description: "Read"}}
	if err := registry.Register(ctx, fallback); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	invoker := NewToolInvoker(registry)
	defer invoker.Close()

	result, err := invoker.InvokeWithFallback(context.Background(), "read", map[string]any{}, []string{"fallback"})
	if err != nil {
		t.Fatalf("InvokeWithFallback should succeed via fallback: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatal("should succeed via fallback server")
	}
}

// TestConcurrentInvocations tests concurrent tool invocations
func TestConcurrentInvocations(t *testing.T) {
	ctx := context.Background()
	registry := NewRegistry()
	server := newMockServer("fs", "1.0.0"); server.capabilities.Tools = &ToolsCapability{}; server.capabilities.Resources = &ResourcesCapability{}
	server.tools = []Tool{
		{Name: "read", Description: "Read"},
		{Name: "write", Description: "Write"},
	}
	if err := registry.Register(ctx, server); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	invoker := NewToolInvoker(registry)
	defer invoker.Close()

	numGoroutines := 50
	var wg sync.WaitGroup
	errorChan := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			tool := "read"
			if idx%2 == 0 {
				tool = "write"
			}
			result, err := invoker.InvokeTool(context.Background(), tool, map[string]any{})
			if err != nil || result == nil {
				errorChan <- fmt.Errorf("goroutine %d failed", idx)
			}
		}(i)
	}

	wg.Wait()
	close(errorChan)

	if len(errorChan) > 0 {
		for err := range errorChan {
			t.Error(err)
		}
	}
}

// TestCloseInvoker tests invoker close
func TestCloseInvoker(t *testing.T) {
	invoker := NewToolInvoker(NewRegistry())
	err := invoker.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}

// TestGetAsyncResult tests getting async result
func TestGetAsyncResult(t *testing.T) {
	ctx := context.Background()
	registry := NewRegistry()
	server := newMockServer("fs", "1.0.0"); server.capabilities.Tools = &ToolsCapability{}; server.capabilities.Resources = &ResourcesCapability{}
	server.tools = []Tool{{Name: "read", Description: "Read"}}
	if err := registry.Register(ctx, server); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	invoker := NewToolInvoker(registry)
	defer invoker.Close()

	done := make(chan struct{})
	callback := func(result *ToolResult, err error) {
		close(done)
	}

	requestID := invoker.InvokeToolAsync(context.Background(), "read", map[string]any{}, callback)

	// Check pending
	_, _, complete := invoker.GetAsyncResult(requestID)
	if complete {
		t.Fatal("result should not be complete yet")
	}

	// Wait for completion
	<-done

	// Check completed
	result, err, complete := invoker.GetAsyncResult(requestID)
	if !complete {
		t.Fatal("result should be complete")
	}
	if err != nil || result == nil {
		t.Fatal("result should be valid")
	}
}

// TestCancelAsync tests canceling async invocation
func TestCancelAsync(t *testing.T) {
	ctx := context.Background()
	registry := NewRegistry()
	server := newMockTimeoutServer("slow", 500*time.Millisecond); server.capabilities.Tools = &ToolsCapability{}; server.capabilities.Resources = &ResourcesCapability{}
	server.tools = []Tool{{Name: "slow_tool", Description: "Slow"}}
	if err := registry.Register(ctx, server); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	invoker := NewToolInvoker(registry)
	defer invoker.Close()

	callback := func(result *ToolResult, err error) {}
	requestID := invoker.InvokeToolAsync(context.Background(), "slow_tool", map[string]any{}, callback)

	// Cancel before completion
	time.Sleep(50 * time.Millisecond)
	err := invoker.CancelAsync(requestID)
	if err != nil {
		t.Fatalf("CancelAsync failed: %v", err)
	}
}

// TestRetryOptionsDefaults tests default retry options
func TestRetryOptionsDefaults(t *testing.T) {
	opts := DefaultRetryOptions()
	if opts.MaxAttempts != 3 {
		t.Errorf("MaxAttempts default should be 3, got %d", opts.MaxAttempts)
	}
	if opts.BackoffMultiplier != 2.0 {
		t.Errorf("BackoffMultiplier should be 2.0, got %f", opts.BackoffMultiplier)
	}
	if !opts.Jitter {
		t.Error("Jitter should be enabled")
	}
}

// TestErrorPropagation tests error message propagation
func TestErrorPropagation(t *testing.T) {
	registry := NewRegistry()
	invoker := NewToolInvoker(registry)
	defer invoker.Close()

	_, err := invoker.InvokeTool(context.Background(), "missing", map[string]any{})
	if err == nil {
		t.Fatal("should return error for missing tool")
	}
}

// TestInvokerContextCancellation tests context cancellation handling
func TestInvokerContextCancellation(t *testing.T) {
	ctx := context.Background()
	registry := NewRegistry()
	server := newMockTimeoutServer("fs", 500*time.Millisecond); server.capabilities.Tools = &ToolsCapability{}; server.capabilities.Resources = &ResourcesCapability{}
	server.tools = []Tool{{Name: "slow", Description: "Slow"}}
	if err := registry.Register(ctx, server); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	invoker := NewToolInvoker(registry)
	defer invoker.Close()

	cancelCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := invoker.InvokeTool(cancelCtx, "slow", map[string]any{})
	if err == nil {
		t.Fatal("should return error on context cancellation")
	}
}

// TestInvokeWithNilParameters tests nil parameters handling
func TestInvokeWithNilParameters(t *testing.T) {
	ctx := context.Background()
	registry := NewRegistry()
	server := newMockServer("fs", "1.0.0"); server.capabilities.Tools = &ToolsCapability{}; server.capabilities.Resources = &ResourcesCapability{}
	server.tools = []Tool{{Name: "tool", Description: "Tool"}}
	if err := registry.Register(ctx, server); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	invoker := NewToolInvoker(registry)
	defer invoker.Close()

	result, err := invoker.InvokeTool(context.Background(), "tool", nil)
	if err != nil {
		t.Fatalf("should allow nil parameters: %v", err)
	}
	if result == nil {
		t.Fatal("result should not be nil")
	}
}

// TestInvokeWithEmptyParameters tests empty parameters handling
func TestInvokeWithEmptyParameters(t *testing.T) {
	ctx := context.Background()
	registry := NewRegistry()
	server := newMockServer("fs", "1.0.0"); server.capabilities.Tools = &ToolsCapability{}; server.capabilities.Resources = &ResourcesCapability{}
	server.tools = []Tool{{Name: "tool", Description: "Tool"}}
	if err := registry.Register(ctx, server); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	invoker := NewToolInvoker(registry)
	defer invoker.Close()

	result, err := invoker.InvokeTool(context.Background(), "tool", map[string]any{})
	if err != nil {
		t.Fatalf("should allow empty parameters: %v", err)
	}
	if result == nil {
		t.Fatal("result should not be nil")
	}
}

// TestCancelAsyncNotFound tests cancel with invalid request ID
func TestCancelAsyncNotFound(t *testing.T) {
	invoker := NewToolInvoker(NewRegistry())
	defer invoker.Close()

	err := invoker.CancelAsync("nonexistent-id")
	if err == nil {
		t.Fatal("should return error for nonexistent request")
	}
}
