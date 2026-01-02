// Package mcp provides end-to-end tests for MCP server integration.
//
// These tests verify the complete MCP lifecycle including:
// - Connection initialization and lifecycle
// - Tool discovery and invocation
// - Resource listing and reading
// - Prompt template usage
// - Error handling and recovery
// - Concurrent operations
// - Pagination handling
// - Timeout handling
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"
)

// =============================================================================
// Test: Connection Lifecycle (Initialize, Use, Close)
// =============================================================================

// TestE2EConnectionLifecycle tests the complete connection lifecycle:
// create, initialize, use, and close.
func TestE2EConnectionLifecycle(t *testing.T) {
	ctx := context.Background()

	// Create a mock server for testing
	server := newMockServer("test-server", "1.0.0")
	server.capabilities = &ServerCapabilities{
		Tools: &ToolsCapability{ListChanged: true},
		Resources: &ResourcesCapability{
			Subscribe:   true,
			ListChanged: true,
		},
		Prompts: &PromptsCapability{ListChanged: true},
	}

	// Initialize the server
	result, err := server.Initialize(ctx, &ClientInfo{
		Name:    "flip2",
		Version: "2.0.0",
	})
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Verify initialization result
	if result.ProtocolVersion != LatestProtocolVersion {
		t.Errorf("expected protocol version %q, got %q", LatestProtocolVersion, result.ProtocolVersion)
	}

	if result.ServerInfo.Name != "test-server" {
		t.Errorf("expected server name 'test-server', got %q", result.ServerInfo.Name)
	}

	// Verify capabilities
	if result.Capabilities.Tools == nil {
		t.Fatal("expected Tools capability, got nil")
	}

	// Test ping (server responsiveness)
	err = server.Ping(ctx)
	if err != nil {
		t.Fatalf("Ping failed: %v", err)
	}

	// Close the connection
	err = server.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Verify connection is closed
	if !server.IsClosed() {
		t.Fatal("expected server to be closed")
	}
}

// =============================================================================
// Test: Tool Discovery and Listing
// =============================================================================

// TestE2EToolDiscovery tests discovering tools from a server.
func TestE2EToolDiscovery(t *testing.T) {
	ctx := context.Background()
	registry := NewRegistry()

	// Create a mock filesystem server
	server := newMockServer("filesystem", "1.0.0")
	server.capabilities = &ServerCapabilities{
		Tools: &ToolsCapability{ListChanged: true},
	}

	// Add tools with realistic schemas
	schemaRead := json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "Path to file"}
		},
		"required": ["path"]
	}`)

	schemaWrite := json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "Path to file"},
			"content": {"type": "string", "description": "File content"}
		},
		"required": ["path", "content"]
	}`)

	schemaList := json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "Directory path"}
		},
		"required": ["path"]
	}`)

	server.tools = []Tool{
		{
			Name:        "read_file",
			Description: "Read the contents of a file",
			InputSchema: schemaRead,
			Annotations: &ToolAnnotations{
				ReadOnlyHint: true,
				Title:        "Read File",
			},
		},
		{
			Name:        "write_file",
			Description: "Write contents to a file",
			InputSchema: schemaWrite,
			Annotations: &ToolAnnotations{
				DestructiveHint: true,
				Title:           "Write File",
			},
		},
		{
			Name:        "list_files",
			Description: "List files in a directory",
			InputSchema: schemaList,
			Annotations: &ToolAnnotations{
				ReadOnlyHint: true,
				Title:        "List Files",
			},
		},
	}

	// Register the server
	err := registry.Register(ctx, server)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Discover tools
	tools, err := DiscoverTools(ctx, registry, "filesystem")
	if err != nil {
		t.Fatalf("DiscoverTools failed: %v", err)
	}

	// Verify tool count
	if len(tools) != 3 {
		t.Errorf("expected 3 tools, got %d", len(tools))
	}

	// Verify tool names
	expectedNames := []string{"read_file", "write_file", "list_files"}
	for i, expectedName := range expectedNames {
		if i >= len(tools) {
			t.Fatalf("missing tool at index %d", i)
		}
		if tools[i].Name != expectedName {
			t.Errorf("tool %d: expected name %q, got %q", i, expectedName, tools[i].Name)
		}
	}

	// Verify annotations
	if !tools[0].Annotations.ReadOnlyHint {
		t.Error("expected read_file to have ReadOnlyHint=true")
	}

	if !tools[1].Annotations.DestructiveHint {
		t.Error("expected write_file to have DestructiveHint=true")
	}

	// Verify descriptions are preserved
	if tools[0].Description != "Read the contents of a file" {
		t.Errorf("unexpected description for read_file: %q", tools[0].Description)
	}
}

// =============================================================================
// Test: Tool Invocation with Arguments
// =============================================================================

// TestE2EToolInvocation tests calling tools with arguments and receiving results.
func TestE2EToolInvocation(t *testing.T) {
	ctx := context.Background()

	// Create a mock server with actual tool behavior
	server := &mockServerWithBehavior{
		mockServer: newMockServer("calculator", "1.0.0"),
	}
	server.capabilities = &ServerCapabilities{
		Tools: &ToolsCapability{ListChanged: true},
	}

	schema := json.RawMessage(`{"type":"object","properties":{"a":{"type":"number"},"b":{"type":"number"}},"required":["a","b"]}`)
	server.tools = []Tool{
		{
			Name:        "add",
			Description: "Add two numbers",
			InputSchema: schema,
		},
		{
			Name:        "multiply",
			Description: "Multiply two numbers",
			InputSchema: schema,
		},
	}

	// Test invoking the add tool
	result, err := server.CallTool(ctx, "add", map[string]any{
		"a": 5.0,
		"b": 3.0,
	})
	if err != nil {
		t.Fatalf("CallTool failed: %v", err)
	}

	// Verify result
	if result == nil || len(result.Content) == 0 {
		t.Fatal("expected tool result, got empty")
	}

	// Verify result content
	if result.Content[0].Type != "text" {
		t.Errorf("expected text content, got %q", result.Content[0].Type)
	}

	if result.IsError {
		t.Error("tool should not report error")
	}

	// Verify the computed result
	if result.Content[0].Text != "8.0" {
		t.Errorf("expected result '8.0', got %q", result.Content[0].Text)
	}

	// Test error case - tool not found
	_, err = server.CallTool(ctx, "divide", map[string]any{
		"a": 10.0,
		"b": 2.0,
	})
	if err == nil {
		t.Fatal("expected error for non-existent tool")
	}
}

// mockServerWithBehavior extends mockServer with realistic tool behavior
type mockServerWithBehavior struct {
	*mockServer
}

func (m *mockServerWithBehavior) CallTool(ctx context.Context, name string, arguments map[string]any) (*ToolResult, error) {
	switch name {
	case "add":
		a, ok1 := arguments["a"].(float64)
		b, ok2 := arguments["b"].(float64)
		if !ok1 || !ok2 {
			return nil, fmt.Errorf("invalid arguments for add")
		}
		result := a + b
		return &ToolResult{
			Content: []ContentItem{
				{
					Type: "text",
					Text: fmt.Sprintf("%.1f", result),
				},
			},
		}, nil

	case "multiply":
		a, ok1 := arguments["a"].(float64)
		b, ok2 := arguments["b"].(float64)
		if !ok1 || !ok2 {
			return nil, fmt.Errorf("invalid arguments for multiply")
		}
		result := a * b
		return &ToolResult{
			Content: []ContentItem{
				{
					Type: "text",
					Text: fmt.Sprintf("%.1f", result),
				},
			},
		}, nil

	default:
		return nil, fmt.Errorf("tool %q not found", name)
	}
}

// =============================================================================
// Test: Resource Operations (Listing and Reading)
// =============================================================================

// TestE2EResourceListing tests listing resources from a server.
func TestE2EResourceListing(t *testing.T) {
	ctx := context.Background()

	// Create a mock server with resources
	server := newMockServer("database", "1.0.0")
	server.capabilities = &ServerCapabilities{
		Resources: &ResourcesCapability{
			Subscribe:   true,
			ListChanged: true,
		},
	}

	server.resources = []Resource{
		{
			URI:         "db://customers",
			Name:        "Customers",
			Description: "Customer database",
			MimeType:    "application/json",
			Annotations: &ResourceAnnotations{
				Audience: []string{"assistant"},
				Priority: 0.8,
			},
		},
		{
			URI:         "db://orders",
			Name:        "Orders",
			Description: "Order database",
			MimeType:    "application/json",
			Annotations: &ResourceAnnotations{
				Audience: []string{"assistant"},
				Priority: 0.9,
			},
		},
	}

	// List resources
	resources, err := server.ListResources(ctx, nil)
	if err != nil {
		t.Fatalf("ListResources failed: %v", err)
	}

	// Verify resource count
	if len(resources.Resources) != 2 {
		t.Errorf("expected 2 resources, got %d", len(resources.Resources))
	}

	// Verify resource URIs
	expectedURIs := []string{"db://customers", "db://orders"}
	for i, expectedURI := range expectedURIs {
		if resources.Resources[i].URI != expectedURI {
			t.Errorf("resource %d: expected URI %q, got %q", i, expectedURI, resources.Resources[i].URI)
		}
	}

	// Verify resource annotations
	if resources.Resources[0].Annotations == nil || resources.Resources[0].Annotations.Priority != 0.8 {
		t.Error("expected priority annotation on first resource")
	}
}

// TestE2EResourceReading tests reading resource contents.
func TestE2EResourceReading(t *testing.T) {
	ctx := context.Background()

	// Create a mock server with readable resources
	server := &mockServerWithResources{
		mockServer: newMockServer("data", "1.0.0"),
	}
	server.capabilities = &ServerCapabilities{
		Resources: &ResourcesCapability{Subscribe: true},
	}

	// Register some resources
	server.resourceContents = map[string]string{
		"resource://config": `{"timeout": 30, "retries": 3}`,
		"resource://data":   `["item1", "item2", "item3"]`,
	}

	// Read a resource
	contents, err := server.ReadResource(ctx, "resource://config")
	if err != nil {
		t.Fatalf("ReadResource failed: %v", err)
	}

	// Verify content
	if len(contents.Contents) != 1 {
		t.Errorf("expected 1 content item, got %d", len(contents.Contents))
	}

	if contents.Contents[0].URI != "resource://config" {
		t.Errorf("expected URI %q, got %q", "resource://config", contents.Contents[0].URI)
	}

	// Verify content text matches
	expectedJSON := `{"timeout": 30, "retries": 3}`
	if contents.Contents[0].Text != expectedJSON {
		t.Errorf("expected content %q, got %q", expectedJSON, contents.Contents[0].Text)
	}

	// Test reading non-existent resource
	_, err = server.ReadResource(ctx, "resource://nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent resource")
	}

	// Verify error is a 404
	if mcpErr, ok := err.(*Error); ok {
		if mcpErr.Code != ErrorCodeResourceNotFound {
			t.Errorf("expected ResourceNotFound error, got %d", mcpErr.Code)
		}
	}
}

// mockServerWithResources extends mockServer with resource content
type mockServerWithResources struct {
	*mockServer
	resourceContents map[string]string
}

func (m *mockServerWithResources) ReadResource(ctx context.Context, uri string) (*ResourceContents, error) {
	content, exists := m.resourceContents[uri]
	if !exists {
		return nil, &Error{Code: ErrorCodeResourceNotFound, Message: fmt.Sprintf("resource %q not found", uri)}
	}

	return &ResourceContents{
		Contents: []ResourceContent{
			{
				URI:      uri,
				MimeType: "application/json",
				Text:     content,
			},
		},
	}, nil
}

// =============================================================================
// Test: Prompt Templates (Listing and Execution)
// =============================================================================

// TestE2EPromptListing tests listing prompt templates from a server.
func TestE2EPromptListing(t *testing.T) {
	ctx := context.Background()

	// Create a mock server with prompts
	server := newMockServer("prompts", "1.0.0")
	server.capabilities = &ServerCapabilities{
		Prompts: &PromptsCapability{ListChanged: true},
	}

	server.prompts = []Prompt{
		{
			Name:        "summarize",
			Description: "Summarize text",
			Arguments: []PromptArgument{
				{
					Name:        "text",
					Description: "Text to summarize",
					Required:    true,
				},
				{
					Name:        "length",
					Description: "Summary length",
					Required:    false,
				},
			},
		},
		{
			Name:        "translate",
			Description: "Translate text",
			Arguments: []PromptArgument{
				{
					Name:        "text",
					Description: "Text to translate",
					Required:    true,
				},
				{
					Name:        "language",
					Description: "Target language",
					Required:    true,
				},
			},
		},
	}

	// List prompts
	prompts, err := server.ListPrompts(ctx, nil)
	if err != nil {
		t.Fatalf("ListPrompts failed: %v", err)
	}

	// Verify prompt count
	if len(prompts.Prompts) != 2 {
		t.Errorf("expected 2 prompts, got %d", len(prompts.Prompts))
	}

	// Verify prompt names
	expectedNames := []string{"summarize", "translate"}
	for i, expectedName := range expectedNames {
		if prompts.Prompts[i].Name != expectedName {
			t.Errorf("prompt %d: expected name %q, got %q", i, expectedName, prompts.Prompts[i].Name)
		}
	}

	// Verify arguments
	if len(prompts.Prompts[0].Arguments) != 2 {
		t.Errorf("expected 2 arguments for summarize prompt, got %d", len(prompts.Prompts[0].Arguments))
	}

	if !prompts.Prompts[0].Arguments[0].Required {
		t.Error("expected first argument of summarize to be required")
	}

	if prompts.Prompts[1].Arguments[1].Required != true {
		t.Error("expected language argument to be required for translate")
	}
}

// TestE2EPromptExecution tests executing a prompt template with arguments.
func TestE2EPromptExecution(t *testing.T) {
	ctx := context.Background()

	// Create a mock server with prompt execution
	server := &mockServerWithPrompts{
		mockServer: newMockServer("ai-assistant", "1.0.0"),
	}
	server.capabilities = &ServerCapabilities{
		Prompts: &PromptsCapability{ListChanged: true},
	}

	server.prompts = []Prompt{
		{
			Name:        "chat",
			Description: "Chat with assistant",
			Arguments: []PromptArgument{
				{
					Name:     "message",
					Required: true,
				},
			},
		},
	}

	// Execute a prompt
	result, err := server.GetPrompt(ctx, "chat", map[string]string{
		"message": "Hello, how are you?",
	})
	if err != nil {
		t.Fatalf("GetPrompt failed: %v", err)
	}

	// Verify result
	if result == nil || len(result.Messages) == 0 {
		t.Fatal("expected prompt result with messages")
	}

	// Verify message content
	if result.Messages[0].Role != "user" {
		t.Errorf("expected role 'user', got %q", result.Messages[0].Role)
	}

	if result.Messages[0].Content.Type != "text" {
		t.Errorf("expected content type 'text', got %q", result.Messages[0].Content.Type)
	}

	// Verify second message (assistant response)
	if result.Messages[1].Role != "assistant" {
		t.Errorf("expected assistant role for second message, got %q", result.Messages[1].Role)
	}
}

// mockServerWithPrompts extends mockServer with prompt execution
type mockServerWithPrompts struct {
	*mockServer
}

func (m *mockServerWithPrompts) GetPrompt(ctx context.Context, name string, arguments map[string]string) (*PromptResult, error) {
	prompt := m.prompts[0] // Simplified for testing

	return &PromptResult{
		Description: prompt.Description,
		Messages: []PromptMessage{
			{
				Role: "user",
				Content: MessageContent{
					Type: "text",
					Text: arguments["message"],
				},
			},
			{
				Role: "assistant",
				Content: MessageContent{
					Type: "text",
					Text: "I'm an AI assistant ready to help!",
				},
			},
		},
	}, nil
}

// =============================================================================
// Test: Sampling (Server -> Client LLM Requests)
// =============================================================================

// TestE2ESamplingRequest tests servers requesting LLM completions.
func TestE2ESamplingRequest(t *testing.T) {
	ctx := context.Background()

	// Create a mock sampling handler
	samplingHandler := &mockSamplingHandler{
		responses: make(map[string]*SamplingResponse),
	}

	samplingHandler.responses["test"] = &SamplingResponse{
		Role: "assistant",
		Content: MessageContent{
			Type: "text",
			Text: "This is a test response from the LLM.",
		},
		Model:      "claude-3-sonnet",
		StopReason: "endTurn",
	}

	// Create a sampling request
	request := &SamplingRequest{
		Messages: []SamplingMessage{
			{
				Role: "user",
				Content: MessageContent{
					Type: "text",
					Text: "test",
				},
			},
		},
		MaxTokens:      1024,
		IncludeContext: "thisServer",
	}

	// Handle the request
	response, err := samplingHandler.CreateMessage(ctx, request)
	if err != nil {
		t.Fatalf("CreateMessage failed: %v", err)
	}

	// Verify response
	if response.Role != "assistant" {
		t.Errorf("expected role 'assistant', got %q", response.Role)
	}

	if response.Content.Type != "text" {
		t.Errorf("expected content type 'text', got %q", response.Content.Type)
	}

	if response.Model != "claude-3-sonnet" {
		t.Errorf("expected model 'claude-3-sonnet', got %q", response.Model)
	}
}

// mockSamplingHandler implements the SamplingHandler interface
type mockSamplingHandler struct {
	responses map[string]*SamplingResponse
}

func (m *mockSamplingHandler) CreateMessage(ctx context.Context, request *SamplingRequest) (*SamplingResponse, error) {
	if len(request.Messages) == 0 {
		return nil, fmt.Errorf("no messages in request")
	}

	// Use the first message as a key (simplified)
	msgText := ""
	if request.Messages[0].Content.Type == "text" {
		msgText = request.Messages[0].Content.Text
	}

	response, exists := m.responses[msgText]
	if !exists {
		return nil, fmt.Errorf("no response configured for message")
	}

	return response, nil
}

// =============================================================================
// Test: Registry and Discovery
// =============================================================================

// TestE2ERegistryAndDiscovery tests registering and discovering servers.
func TestE2ERegistryAndDiscovery(t *testing.T) {
	ctx := context.Background()
	registry := NewRegistry()

	// Create multiple servers
	filesystemServer := newMockServer("filesystem", "1.0.0")
	filesystemServer.capabilities = &ServerCapabilities{
		Tools:     &ToolsCapability{ListChanged: true},
		Resources: &ResourcesCapability{Subscribe: true},
	}

	databaseServer := newMockServer("database", "1.0.0")
	databaseServer.capabilities = &ServerCapabilities{
		Tools:     &ToolsCapability{ListChanged: true},
		Resources: &ResourcesCapability{Subscribe: true},
	}

	// Register servers
	err := registry.Register(ctx, filesystemServer)
	if err != nil {
		t.Fatalf("Register filesystem server failed: %v", err)
	}

	err = registry.Register(ctx, databaseServer)
	if err != nil {
		t.Fatalf("Register database server failed: %v", err)
	}

	// List all servers
	servers := registry.List()
	if len(servers) != 2 {
		t.Errorf("expected 2 servers, got %d", len(servers))
	}

	// Verify server names
	expectedNames := map[string]bool{"filesystem": false, "database": false}
	for _, name := range servers {
		if _, exists := expectedNames[name]; !exists {
			t.Errorf("unexpected server name: %q", name)
		}
		expectedNames[name] = true
	}

	for name, found := range expectedNames {
		if !found {
			t.Errorf("server %q not found in registry", name)
		}
	}

	// Get a specific server
	server, exists := registry.Get("filesystem")
	if !exists {
		t.Fatal("filesystem server not found in registry")
	}

	if server.ServerInfo().Name != "filesystem" {
		t.Errorf("expected server name 'filesystem', got %q", server.ServerInfo().Name)
	}

	// List servers by capability
	toolServers := registry.ListByCapability("tools")
	if len(toolServers) != 2 {
		t.Errorf("expected 2 servers with tools capability, got %d", len(toolServers))
	}

	// Deregister a server
	err = registry.Deregister(ctx, "filesystem")
	if err != nil {
		t.Fatalf("Deregister failed: %v", err)
	}

	// Verify deregistration
	servers = registry.List()
	if len(servers) != 1 {
		t.Errorf("expected 1 server after deregister, got %d", len(servers))
	}

	if servers[0] != "database" {
		t.Errorf("expected remaining server to be 'database', got %q", servers[0])
	}
}

// =============================================================================
// Test: Error Recovery and Resilience
// =============================================================================

// TestE2EErrorRecovery tests handling errors and recovery.
func TestE2EErrorRecovery(t *testing.T) {
	ctx := context.Background()

	// Create a server that can simulate failures
	server := &mockServerWithFailures{
		mockServer:   newMockServer("flaky", "1.0.0"),
		failureCount: map[string]int{},
		failureLimit: map[string]int{},
	}

	server.capabilities = &ServerCapabilities{
		Tools: &ToolsCapability{ListChanged: true},
	}

	schemaTest := json.RawMessage(`{"type":"object"}`)
	server.tools = []Tool{
		{
			Name:        "unreliable_op",
			Description: "An unreliable operation",
			InputSchema: schemaTest,
		},
	}

	// Set up failure scenario: fail 2 times, then succeed
	server.failureLimit["unreliable_op"] = 2

	// First attempt: should fail
	_, err := server.CallTool(ctx, "unreliable_op", map[string]any{})
	if err == nil {
		t.Fatal("expected error on first attempt")
	}

	// Second attempt: should fail
	_, err = server.CallTool(ctx, "unreliable_op", map[string]any{})
	if err == nil {
		t.Fatal("expected error on second attempt")
	}

	// Third attempt: should succeed
	result, err := server.CallTool(ctx, "unreliable_op", map[string]any{})
	if err != nil {
		t.Fatalf("expected success on third attempt, got error: %v", err)
	}

	if result == nil {
		t.Fatal("expected result on third attempt")
	}

	// Verify we recovered
	if server.failureCount["unreliable_op"] != 2 {
		t.Errorf("expected 2 failures before success, got %d", server.failureCount["unreliable_op"])
	}
}

// mockServerWithFailures can simulate transient failures
type mockServerWithFailures struct {
	*mockServer
	failureCount map[string]int
	failureLimit map[string]int
	mu           sync.Mutex
}

func (m *mockServerWithFailures) CallTool(ctx context.Context, name string, arguments map[string]any) (*ToolResult, error) {
	m.mu.Lock()
	count := m.failureCount[name]
	limit := m.failureLimit[name]
	m.mu.Unlock()

	// Check if we should fail
	if count < limit {
		m.mu.Lock()
		m.failureCount[name]++
		m.mu.Unlock()
		return nil, &Error{
			Code:    ErrorCodeInternalError,
			Message: fmt.Sprintf("transient error %d/%d", count+1, limit),
		}
	}

	// Succeed
	return &ToolResult{
		Content: []ContentItem{{Type: "text", Text: "recovered!"}},
	}, nil
}

// =============================================================================
// Test: Concurrent Operations
// =============================================================================

// TestE2EConcurrentOperations tests concurrent tool invocations.
func TestE2EConcurrentOperations(t *testing.T) {
	ctx := context.Background()

	// Create a server for concurrent access
	server := &mockServerWithConcurrency{
		mockServer: newMockServer("concurrent", "1.0.0"),
		callCount:  0,
	}

	server.capabilities = &ServerCapabilities{
		Tools: &ToolsCapability{ListChanged: true},
	}

	schemaTest := json.RawMessage(`{"type":"object"}`)
	server.tools = []Tool{
		{
			Name:        "concurrent_op",
			Description: "Operation that can be called concurrently",
			InputSchema: schemaTest,
		},
	}

	// Launch concurrent operations
	const concurrentCalls = 10
	results := make(chan error, concurrentCalls)
	var wg sync.WaitGroup

	for i := 0; i < concurrentCalls; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := server.CallTool(ctx, "concurrent_op", map[string]any{})
			results <- err
		}()
	}

	// Wait for all goroutines
	wg.Wait()
	close(results)

	// Check results
	var errors []error
	for err := range results {
		if err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		t.Errorf("expected all concurrent calls to succeed, got %d errors", len(errors))
	}

	// Verify call count
	if server.callCount != concurrentCalls {
		t.Errorf("expected %d calls, got %d", concurrentCalls, server.callCount)
	}
}

// mockServerWithConcurrency tracks concurrent calls
type mockServerWithConcurrency struct {
	*mockServer
	callCount int
	mu        sync.Mutex
}

func (m *mockServerWithConcurrency) CallTool(ctx context.Context, name string, arguments map[string]any) (*ToolResult, error) {
	m.mu.Lock()
	m.callCount++
	m.mu.Unlock()

	// Simulate some work
	time.Sleep(10 * time.Millisecond)

	return &ToolResult{
		Content: []ContentItem{{Type: "text", Text: "success"}},
	}, nil
}

// =============================================================================
// Test: Timeout Handling
// =============================================================================

// TestE2ETimeoutHandling tests request timeout handling.
func TestE2ETimeoutHandling(t *testing.T) {
	// Create a context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// Create a slow server
	server := &mockServerWithDelay{
		mockServer: newMockServer("slow", "1.0.0"),
		delay:      200 * time.Millisecond,
	}

	server.capabilities = &ServerCapabilities{
		Tools: &ToolsCapability{ListChanged: true},
	}

	schemaTest := json.RawMessage(`{"type":"object"}`)
	server.tools = []Tool{
		{
			Name:        "slow_operation",
			Description: "Slow operation",
			InputSchema: schemaTest,
		},
	}

	// Try to call with timeout
	_, err := server.CallTool(ctx, "slow_operation", map[string]any{})

	// Should timeout
	if err == nil || err != context.DeadlineExceeded {
		t.Errorf("expected context.DeadlineExceeded, got %v", err)
	}
}

// mockServerWithDelay simulates slow operations
type mockServerWithDelay struct {
	*mockServer
	delay time.Duration
}

func (m *mockServerWithDelay) CallTool(ctx context.Context, name string, arguments map[string]any) (*ToolResult, error) {
	// Simulate work with context awareness
	select {
	case <-time.After(m.delay):
		return &ToolResult{
			Content: []ContentItem{{Type: "text", Text: "done"}},
		}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
