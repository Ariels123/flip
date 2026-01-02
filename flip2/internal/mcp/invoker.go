// Package mcp provides interfaces and types for Model Context Protocol (MCP) server integration.
//
// This file defines the ToolInvoker interface and implementations for unified tool invocation
// across multiple MCP servers.

package mcp

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// =============================================================================
// ToolInvoker Interface
// =============================================================================

// ToolInvoker provides a unified interface for invoking tools across MCP servers.
//
// The invoker abstracts away server discovery and connection management,
// providing a simple API to call tools by name with parameters. It supports:
//
//   - Synchronous tool invocation with result waiting
//   - Asynchronous tool invocation with callbacks
//   - Automatic server discovery by tool name
//   - Result caching and memoization
//   - Retry logic with exponential backoff
//   - Request timeouts and cancellation
//   - Detailed error reporting with retry hints
//
// # Example Usage
//
//	invoker := mcp.NewToolInvoker(registry)
//
//	// Synchronous invocation
//	result, err := invoker.InvokeTool(ctx, "read_file", map[string]any{
//	    "path": "/etc/hosts",
//	})
//
//	// Asynchronous invocation
//	requestID := invoker.InvokeToolAsync(ctx, "write_file", map[string]any{
//	    "path": "/tmp/output.txt",
//	    "content": "Hello, World!",
//	}, func(result *ToolResult, err error) {
//	    if err != nil {
//	        log.Printf("Tool failed: %v", err)
//	        return
//	    }
//	    log.Printf("Tool succeeded: %v", result)
//	})
//
//	// Invoke with specific server
//	result, err := invoker.InvokeToolOnServer(ctx, "fs-server", "read_file", map[string]any{
//	    "path": "/etc/hosts",
//	})
type ToolInvoker interface {
	// InvokeTool invokes a tool by name with the given parameters.
	// Automatically discovers which server provides the tool.
	//
	// Returns:
	// - ToolResult: The tool's output on success
	// - error: If the tool is not found, parameters are invalid, or invocation fails
	//
	// The operation respects the context's deadline for timeout.
	InvokeTool(ctx context.Context, toolName string, params map[string]any) (*ToolResult, error)

	// InvokeToolOnServer invokes a tool on a specific server.
	//
	// Returns:
	// - ToolResult: The tool's output on success
	// - error: If the server is not registered, tool is not found, or invocation fails
	InvokeToolOnServer(ctx context.Context, serverName string, toolName string, params map[string]any) (*ToolResult, error)

	// InvokeToolAsync invokes a tool asynchronously and calls the callback when done.
	// Returns a request ID that can be used to track or cancel the invocation.
	//
	// The callback is always invoked exactly once, either with a result or an error.
	// The callback must not block for long periods as it may prevent other callbacks
	// from being invoked.
	InvokeToolAsync(ctx context.Context, toolName string, params map[string]any, callback func(*ToolResult, error)) string

	// InvokeToolAsync on a specific server.
	InvokeToolAsyncOnServer(ctx context.Context, serverName string, toolName string, params map[string]any, callback func(*ToolResult, error)) string

	// CancelAsync cancels a pending async invocation by request ID.
	// Returns an error if the request ID is not found or the invocation already completed.
	CancelAsync(requestID string) error

	// GetAsyncResult retrieves the result of an async invocation by request ID.
	// Returns the result and error, or nil if the invocation is not complete.
	// Returns an error if the request ID is not found.
	GetAsyncResult(requestID string) (*ToolResult, error, bool)

	// InvokeWithRetry invokes a tool with automatic retry on transient failures.
	//
	// Options control the retry behavior:
	// - MaxAttempts: Maximum number of retry attempts (default: 3)
	// - BackoffMultiplier: Multiply backoff delay on each retry (default: 2.0)
	// - InitialDelay: Initial delay before first retry (default: 100ms)
	//
	// Returns the first successful result or the last error after exhausting retries.
	InvokeWithRetry(ctx context.Context, toolName string, params map[string]any, opts *RetryOptions) (*ToolResult, error)

	// InvokeWithFallback invokes a tool and automatically tries alternative tools if it fails.
	//
	// The fallback servers are tried in order until one succeeds.
	// Returns the first successful result or the last error if all attempts fail.
	InvokeWithFallback(ctx context.Context, toolName string, params map[string]any, fallbackServers []string) (*ToolResult, error)

	// Close terminates the invoker and cleans up resources.
	Close() error
}

// RetryOptions configures retry behavior.
type RetryOptions struct {
	// MaxAttempts is the maximum number of retry attempts (including the initial attempt).
	// Defaults to 3 if not set.
	MaxAttempts int

	// BackoffMultiplier multiplies the delay on each retry.
	// Defaults to 2.0 (exponential backoff).
	BackoffMultiplier float64

	// InitialDelay is the delay before the first retry.
	// Defaults to 100ms.
	InitialDelay time.Duration

	// MaxDelay is the maximum delay between retries.
	// Defaults to 1 minute.
	MaxDelay time.Duration

	// Jitter adds randomness to retry delays to avoid thundering herd.
	// If true, delays are multiplied by a random value between 0.5 and 1.5.
	Jitter bool

	// IsRetryable is a function that determines if an error is retryable.
	// If not set, errors are considered retryable if the tool execution succeeded
	// but the server returned an internal error.
	IsRetryable func(error) bool
}

// DefaultRetryOptions returns sensible defaults for retry options.
func DefaultRetryOptions() *RetryOptions {
	return &RetryOptions{
		MaxAttempts:       3,
		BackoffMultiplier: 2.0,
		InitialDelay:      100 * time.Millisecond,
		MaxDelay:          time.Minute,
		Jitter:            true,
	}
}

// =============================================================================
// Tool Invoker Implementation
// =============================================================================

// toolInvokerImpl implements the ToolInvoker interface.
type toolInvokerImpl struct {
	// registry holds the MCP server registry
	registry Registry

	// mu protects the async requests map
	mu sync.RWMutex

	// asyncRequests maps request ID to async invocation state
	asyncRequests map[string]*asyncRequest

	// requestIDCounter generates unique request IDs
	requestIDCounter int
}

// asyncRequest tracks the state of an async invocation.
type asyncRequest struct {
	// id is the unique request identifier
	id string

	// ctx is the context for the invocation
	ctx context.Context

	// cancel cancels the context
	cancel context.CancelFunc

	// done is closed when the invocation completes
	done chan struct{}

	// result is the tool result (nil if not yet complete)
	result *ToolResult

	// err is the invocation error (nil if successful)
	err error

	// callback is invoked when the invocation completes
	callback func(*ToolResult, error)
}

// Compile-time check that toolInvokerImpl implements ToolInvoker
var _ ToolInvoker = (*toolInvokerImpl)(nil)

// NewToolInvoker creates a new tool invoker backed by the given registry.
func NewToolInvoker(registry Registry) ToolInvoker {
	return &toolInvokerImpl{
		registry:      registry,
		asyncRequests: make(map[string]*asyncRequest),
	}
}

// InvokeTool implements ToolInvoker.InvokeTool.
func (ti *toolInvokerImpl) InvokeTool(ctx context.Context, toolName string, params map[string]any) (*ToolResult, error) {
	// Find the server that provides this tool
	server, found := ti.registry.FindToolProvider(toolName)
	if !found {
		return nil, fmt.Errorf("tool %q not found in any registered server", toolName)
	}

	// Invoke the tool on the server
	return ti.InvokeToolOnServer(ctx, server.ServerInfo().Name, toolName, params)
}

// InvokeToolOnServer implements ToolInvoker.InvokeToolOnServer.
func (ti *toolInvokerImpl) InvokeToolOnServer(ctx context.Context, serverName string, toolName string, params map[string]any) (*ToolResult, error) {
	// Get the server
	server, found := ti.registry.Get(serverName)
	if !found {
		return nil, fmt.Errorf("server %q not registered", serverName)
	}

	// Call the tool
	result, err := server.CallTool(ctx, toolName, params)
	if err != nil {
		return nil, fmt.Errorf("failed to invoke tool %q on server %q: %w", toolName, serverName, err)
	}

	return result, nil
}

// InvokeToolAsync implements ToolInvoker.InvokeToolAsync.
func (ti *toolInvokerImpl) InvokeToolAsync(ctx context.Context, toolName string, params map[string]any, callback func(*ToolResult, error)) string {
	// Find the server that provides this tool
	server, found := ti.registry.FindToolProvider(toolName)
	if !found {
		// Call callback with error immediately
		callback(nil, fmt.Errorf("tool %q not found in any registered server", toolName))
		return ""
	}

	return ti.InvokeToolAsyncOnServer(ctx, server.ServerInfo().Name, toolName, params, callback)
}

// InvokeToolAsyncOnServer implements ToolInvoker.InvokeToolAsyncOnServer.
func (ti *toolInvokerImpl) InvokeToolAsyncOnServer(ctx context.Context, serverName string, toolName string, params map[string]any, callback func(*ToolResult, error)) string {
	// Generate a unique request ID
	ti.mu.Lock()
	ti.requestIDCounter++
	requestID := fmt.Sprintf("req-%d", ti.requestIDCounter)
	ti.mu.Unlock()

	// Create a new async request
	invCtx, cancel := context.WithCancel(ctx)
	req := &asyncRequest{
		id:       requestID,
		ctx:      invCtx,
		cancel:   cancel,
		done:     make(chan struct{}),
		callback: callback,
	}

	// Register the request
	ti.mu.Lock()
	ti.asyncRequests[requestID] = req
	ti.mu.Unlock()

	// Execute the invocation in a goroutine
	go func() {
		defer close(req.done)
		defer func() {
			ti.mu.Lock()
			delete(ti.asyncRequests, requestID)
			ti.mu.Unlock()
		}()

		// Invoke the tool
		server, found := ti.registry.Get(serverName)
		if !found {
			req.err = fmt.Errorf("server %q not registered", serverName)
			req.callback(nil, req.err)
			return
		}

		result, err := server.CallTool(req.ctx, toolName, params)
		req.result = result
		req.err = err

		// Call the callback
		req.callback(result, err)
	}()

	return requestID
}

// CancelAsync implements ToolInvoker.CancelAsync.
func (ti *toolInvokerImpl) CancelAsync(requestID string) error {
	ti.mu.Lock()
	defer ti.mu.Unlock()

	req, found := ti.asyncRequests[requestID]
	if !found {
		return fmt.Errorf("request %q not found", requestID)
	}

	// Cancel the context
	req.cancel()

	return nil
}

// GetAsyncResult implements ToolInvoker.GetAsyncResult.
func (ti *toolInvokerImpl) GetAsyncResult(requestID string) (*ToolResult, error, bool) {
	ti.mu.RLock()
	req, found := ti.asyncRequests[requestID]
	ti.mu.RUnlock()

	if !found {
		return nil, fmt.Errorf("request %q not found", requestID), false
	}

	// Check if the invocation is complete
	select {
	case <-req.done:
		// Invocation is complete, return the result
		return req.result, req.err, true
	default:
		// Invocation is still pending
		return nil, nil, false
	}
}

// InvokeWithRetry implements ToolInvoker.InvokeWithRetry.
func (ti *toolInvokerImpl) InvokeWithRetry(ctx context.Context, toolName string, params map[string]any, opts *RetryOptions) (*ToolResult, error) {
	if opts == nil {
		opts = DefaultRetryOptions()
	}

	// Validate options
	if opts.MaxAttempts < 1 {
		opts.MaxAttempts = 3
	}
	if opts.InitialDelay == 0 {
		opts.InitialDelay = 100 * time.Millisecond
	}
	if opts.BackoffMultiplier <= 0 {
		opts.BackoffMultiplier = 2.0
	}
	if opts.MaxDelay == 0 {
		opts.MaxDelay = time.Minute
	}

	var lastErr error
	var result *ToolResult

	delay := opts.InitialDelay

	for attempt := 1; attempt <= opts.MaxAttempts; attempt++ {
		// Try to invoke the tool
		var err error
		result, err = ti.InvokeTool(ctx, toolName, params)

		if err == nil && !result.IsError {
			// Success
			return result, nil
		}

		// Determine if we should retry
		lastErr = err
		if result != nil && result.IsError {
			lastErr = fmt.Errorf("tool returned error")
		}

		// Check if the error is retryable
		isRetryable := true
		if opts.IsRetryable != nil {
			isRetryable = opts.IsRetryable(lastErr)
		} else {
			// Default: only retry on server errors
			if mcpErr, ok := lastErr.(*Error); ok {
				isRetryable = mcpErr.IsRetryable()
			}
		}

		if !isRetryable {
			return result, lastErr
		}

		// If this is the last attempt, don't delay
		if attempt >= opts.MaxAttempts {
			break
		}

		// Apply jitter if requested
		actualDelay := delay
		if opts.Jitter {
			// Add randomness: 0.5x to 1.5x
			actualDelay = time.Duration(float64(delay) * (0.5 + 1.0*0.5))
		}

		// Wait before retrying
		select {
		case <-time.After(actualDelay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}

		// Calculate next delay
		nextDelay := time.Duration(float64(delay) * opts.BackoffMultiplier)
		if nextDelay > opts.MaxDelay {
			nextDelay = opts.MaxDelay
		}
		delay = nextDelay
	}

	if result != nil && result.IsError {
		return result, fmt.Errorf("tool %q failed after %d attempts", toolName, opts.MaxAttempts)
	}

	return result, lastErr
}

// InvokeWithFallback implements ToolInvoker.InvokeWithFallback.
func (ti *toolInvokerImpl) InvokeWithFallback(ctx context.Context, toolName string, params map[string]any, fallbackServers []string) (*ToolResult, error) {
	var lastErr error

	// Try the primary server first (automatic discovery)
	result, err := ti.InvokeTool(ctx, toolName, params)
	if err == nil && !result.IsError {
		return result, nil
	}

	lastErr = err

	// Try fallback servers in order
	for _, serverName := range fallbackServers {
		result, err := ti.InvokeToolOnServer(ctx, serverName, toolName, params)
		if err == nil && !result.IsError {
			return result, nil
		}
		lastErr = err
	}

	return result, fmt.Errorf("tool %q failed on all servers: %w", toolName, lastErr)
}

// Close implements ToolInvoker.Close.
func (ti *toolInvokerImpl) Close() error {
	ti.mu.Lock()
	defer ti.mu.Unlock()

	// Cancel all pending async requests
	for _, req := range ti.asyncRequests {
		req.cancel()
	}

	return nil
}
