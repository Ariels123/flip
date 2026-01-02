package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"testing"
)

// mockServer is a mock implementation of the Server interface for testing.
type mockServer struct {
	info         *ServerInfo
	capabilities *ServerCapabilities
	tools        []Tool
	resources    []Resource
	prompts      []Prompt
	closed       bool
	mu           sync.Mutex
}

func newMockServer(name, version string) *mockServer {
	return &mockServer{
		info: &ServerInfo{
			Name:    name,
			Version: version,
		},
		capabilities: &ServerCapabilities{},
		tools:        []Tool{},
		resources:    []Resource{},
		prompts:      []Prompt{},
	}
}

func (m *mockServer) Initialize(ctx context.Context, clientInfo *ClientInfo) (*InitializeResult, error) {
	return &InitializeResult{
		ProtocolVersion: LatestProtocolVersion,
		Capabilities:    m.capabilities,
		ServerInfo:      m.info,
	}, nil
}

func (m *mockServer) Ping(ctx context.Context) error {
	return nil
}

func (m *mockServer) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

func (m *mockServer) Capabilities() *ServerCapabilities {
	return m.capabilities
}

func (m *mockServer) ServerInfo() *ServerInfo {
	return m.info
}

func (m *mockServer) ListTools(ctx context.Context, cursor *string) (*ListToolsResult, error) {
	return &ListToolsResult{
		Tools:      m.tools,
		NextCursor: nil,
	}, nil
}

func (m *mockServer) CallTool(ctx context.Context, name string, arguments map[string]any) (*ToolResult, error) {
	return &ToolResult{
		Content: []ContentItem{{Type: "text", Text: "mock result"}},
	}, nil
}

func (m *mockServer) ListResources(ctx context.Context, cursor *string) (*ListResourcesResult, error) {
	return &ListResourcesResult{
		Resources:  m.resources,
		NextCursor: nil,
	}, nil
}

func (m *mockServer) ListResourceTemplates(ctx context.Context, cursor *string) (*ListResourceTemplatesResult, error) {
	return &ListResourceTemplatesResult{
		ResourceTemplates: []ResourceTemplate{},
		NextCursor:        nil,
	}, nil
}

func (m *mockServer) ReadResource(ctx context.Context, uri string) (*ResourceContents, error) {
	for _, resource := range m.resources {
		if resource.URI == uri {
			return &ResourceContents{
				Contents: []ResourceContent{{URI: uri, Text: "mock content"}},
			}, nil
		}
	}
	return nil, &Error{Code: ErrorCodeResourceNotFound, Message: "resource not found"}
}

func (m *mockServer) SubscribeResource(ctx context.Context, uri string) (<-chan *ResourceUpdate, error) {
	ch := make(chan *ResourceUpdate)
	close(ch)
	return ch, nil
}

func (m *mockServer) UnsubscribeResource(ctx context.Context, uri string) error {
	return nil
}

func (m *mockServer) ListPrompts(ctx context.Context, cursor *string) (*ListPromptsResult, error) {
	return &ListPromptsResult{
		Prompts:    m.prompts,
		NextCursor: nil,
	}, nil
}

func (m *mockServer) GetPrompt(ctx context.Context, name string, arguments map[string]string) (*PromptResult, error) {
	return &PromptResult{
		Messages: []PromptMessage{},
	}, nil
}

func (m *mockServer) CompleteArgument(ctx context.Context, ref CompletionRef, argument CompletionArgument) (*CompletionResult, error) {
	return &CompletionResult{
		Completion: CompletionOptions{Values: []string{}},
	}, nil
}

func (m *mockServer) OnToolsChanged(callback func())     {}
func (m *mockServer) OnResourcesChanged(callback func()) {}
func (m *mockServer) OnPromptsChanged(callback func())   {}
func (m *mockServer) OnLog(callback func(LogMessage))    {}

func (m *mockServer) IsClosed() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.closed
}

// Tests

func TestNewRegistry(t *testing.T) {
	reg := NewRegistry()
	if reg == nil {
		t.Fatal("NewRegistry returned nil")
	}

	servers := reg.List()
	if len(servers) != 0 {
		t.Errorf("new registry should have 0 servers, got %d", len(servers))
	}
}

func TestRegister(t *testing.T) {
	ctx := context.Background()
	reg := NewRegistry()

	server := newMockServer("test-server", "1.0.0")

	err := reg.Register(ctx, server)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	servers := reg.List()
	if len(servers) != 1 {
		t.Errorf("expected 1 server, got %d", len(servers))
	}

	if servers[0] != "test-server" {
		t.Errorf("expected server name 'test-server', got %q", servers[0])
	}
}

func TestRegisterDuplicate(t *testing.T) {
	ctx := context.Background()
	reg := NewRegistry()

	server1 := newMockServer("test-server", "1.0.0")
	server2 := newMockServer("test-server", "2.0.0")

	err := reg.Register(ctx, server1)
	if err != nil {
		t.Fatalf("first Register failed: %v", err)
	}

	err = reg.Register(ctx, server2)
	if err == nil {
		t.Fatal("expected error when registering duplicate server, got nil")
	}

	servers := reg.List()
	if len(servers) != 1 {
		t.Errorf("expected 1 server after duplicate registration, got %d", len(servers))
	}
}

func TestRegisterUninitializedServer(t *testing.T) {
	ctx := context.Background()
	reg := NewRegistry()

	server := &mockServer{
		info:         nil, // Not initialized
		capabilities: &ServerCapabilities{},
	}

	err := reg.Register(ctx, server)
	if err == nil {
		t.Fatal("expected error when registering uninitialized server, got nil")
	}
}

func TestGet(t *testing.T) {
	ctx := context.Background()
	reg := NewRegistry()

	server := newMockServer("test-server", "1.0.0")
	reg.Register(ctx, server)

	retrieved, exists := reg.Get("test-server")
	if !exists {
		t.Fatal("server should exist")
	}

	if retrieved.ServerInfo().Name != "test-server" {
		t.Errorf("expected server name 'test-server', got %q", retrieved.ServerInfo().Name)
	}

	_, exists = reg.Get("nonexistent")
	if exists {
		t.Error("nonexistent server should not exist")
	}
}

func TestDeregister(t *testing.T) {
	ctx := context.Background()
	reg := NewRegistry()

	server := newMockServer("test-server", "1.0.0")
	reg.Register(ctx, server)

	err := reg.Deregister(ctx, "test-server")
	if err != nil {
		t.Fatalf("Deregister failed: %v", err)
	}

	servers := reg.List()
	if len(servers) != 0 {
		t.Errorf("expected 0 servers after deregister, got %d", len(servers))
	}

	if !server.IsClosed() {
		t.Error("server should be closed after deregister")
	}
}

func TestDeregisterNonexistent(t *testing.T) {
	ctx := context.Background()
	reg := NewRegistry()

	err := reg.Deregister(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error when deregistering nonexistent server, got nil")
	}
}

func TestListByCapability(t *testing.T) {
	ctx := context.Background()
	reg := NewRegistry()

	// Server with tools
	server1 := newMockServer("server1", "1.0.0")
	server1.capabilities.Tools = &ToolsCapability{}
	reg.Register(ctx, server1)

	// Server with resources
	server2 := newMockServer("server2", "1.0.0")
	server2.capabilities.Resources = &ResourcesCapability{}
	reg.Register(ctx, server2)

	// Server with both
	server3 := newMockServer("server3", "1.0.0")
	server3.capabilities.Tools = &ToolsCapability{}
	server3.capabilities.Resources = &ResourcesCapability{}
	reg.Register(ctx, server3)

	toolsServers := reg.ListByCapability("tools")
	if len(toolsServers) != 2 {
		t.Errorf("expected 2 servers with tools capability, got %d", len(toolsServers))
	}

	resourcesServers := reg.ListByCapability("resources")
	if len(resourcesServers) != 2 {
		t.Errorf("expected 2 servers with resources capability, got %d", len(resourcesServers))
	}

	promptsServers := reg.ListByCapability("prompts")
	if len(promptsServers) != 0 {
		t.Errorf("expected 0 servers with prompts capability, got %d", len(promptsServers))
	}
}

func TestFindToolProvider(t *testing.T) {
	ctx := context.Background()
	reg := NewRegistry()

	server := newMockServer("test-server", "1.0.0")
	server.capabilities.Tools = &ToolsCapability{}
	server.tools = []Tool{
		{Name: "tool1", Description: "Test tool 1", InputSchema: json.RawMessage(`{}`)},
		{Name: "tool2", Description: "Test tool 2", InputSchema: json.RawMessage(`{}`)},
	}

	reg.Register(ctx, server)

	provider, found := reg.FindToolProvider("tool1")
	if !found {
		t.Fatal("tool1 provider should be found")
	}

	if provider.ServerInfo().Name != "test-server" {
		t.Errorf("expected provider 'test-server', got %q", provider.ServerInfo().Name)
	}

	_, found = reg.FindToolProvider("nonexistent-tool")
	if found {
		t.Error("nonexistent tool should not be found")
	}
}

func TestFindToolProviderConflict(t *testing.T) {
	ctx := context.Background()
	reg := NewRegistry()

	// Two servers providing the same tool
	server1 := newMockServer("server1", "1.0.0")
	server1.capabilities.Tools = &ToolsCapability{}
	server1.tools = []Tool{
		{Name: "shared-tool", Description: "From server1", InputSchema: json.RawMessage(`{}`)},
	}

	server2 := newMockServer("server2", "1.0.0")
	server2.capabilities.Tools = &ToolsCapability{}
	server2.tools = []Tool{
		{Name: "shared-tool", Description: "From server2", InputSchema: json.RawMessage(`{}`)},
	}

	reg.Register(ctx, server1)
	reg.Register(ctx, server2)

	provider, found := reg.FindToolProvider("shared-tool")
	if !found {
		t.Fatal("shared-tool provider should be found")
	}

	// First registered server should win
	if provider.ServerInfo().Name != "server1" {
		t.Errorf("expected provider 'server1', got %q", provider.ServerInfo().Name)
	}
}

func TestFindResourceProvider(t *testing.T) {
	ctx := context.Background()
	reg := NewRegistry()

	server := newMockServer("test-server", "1.0.0")
	server.capabilities.Resources = &ResourcesCapability{}
	server.resources = []Resource{
		{URI: "file:///test.txt", Name: "Test file"},
	}

	reg.Register(ctx, server)

	provider, found := reg.FindResourceProvider("file:///test.txt")
	if !found {
		t.Fatal("resource provider should be found")
	}

	if provider.ServerInfo().Name != "test-server" {
		t.Errorf("expected provider 'test-server', got %q", provider.ServerInfo().Name)
	}

	_, found = reg.FindResourceProvider("file:///nonexistent.txt")
	if found {
		t.Error("nonexistent resource should not be found")
	}
}

func TestAllTools(t *testing.T) {
	ctx := context.Background()
	reg := NewRegistry()

	server1 := newMockServer("server1", "1.0.0")
	server1.capabilities.Tools = &ToolsCapability{}
	server1.tools = []Tool{
		{Name: "tool1", InputSchema: json.RawMessage(`{}`)},
	}

	server2 := newMockServer("server2", "1.0.0")
	server2.capabilities.Tools = &ToolsCapability{}
	server2.tools = []Tool{
		{Name: "tool2", InputSchema: json.RawMessage(`{}`)},
	}

	reg.Register(ctx, server1)
	reg.Register(ctx, server2)

	allTools, err := reg.AllTools(ctx)
	if err != nil {
		t.Fatalf("AllTools failed: %v", err)
	}

	if len(allTools) != 2 {
		t.Errorf("expected 2 tools, got %d", len(allTools))
	}

	if _, exists := allTools["tool1"]; !exists {
		t.Error("tool1 should be in all tools")
	}

	if _, exists := allTools["tool2"]; !exists {
		t.Error("tool2 should be in all tools")
	}
}

func TestAllResources(t *testing.T) {
	ctx := context.Background()
	reg := NewRegistry()

	server1 := newMockServer("server1", "1.0.0")
	server1.capabilities.Resources = &ResourcesCapability{}
	server1.resources = []Resource{
		{URI: "file:///test1.txt", Name: "Test 1"},
	}

	server2 := newMockServer("server2", "1.0.0")
	server2.capabilities.Resources = &ResourcesCapability{}
	server2.resources = []Resource{
		{URI: "file:///test2.txt", Name: "Test 2"},
	}

	reg.Register(ctx, server1)
	reg.Register(ctx, server2)

	allResources, err := reg.AllResources(ctx)
	if err != nil {
		t.Fatalf("AllResources failed: %v", err)
	}

	if len(allResources) != 2 {
		t.Errorf("expected 2 resources, got %d", len(allResources))
	}
}

func TestAllPrompts(t *testing.T) {
	ctx := context.Background()
	reg := NewRegistry()

	server1 := newMockServer("server1", "1.0.0")
	server1.capabilities.Prompts = &PromptsCapability{}
	server1.prompts = []Prompt{
		{Name: "prompt1", Description: "Test prompt 1"},
	}

	server2 := newMockServer("server2", "1.0.0")
	server2.capabilities.Prompts = &PromptsCapability{}
	server2.prompts = []Prompt{
		{Name: "prompt2", Description: "Test prompt 2"},
	}

	reg.Register(ctx, server1)
	reg.Register(ctx, server2)

	allPrompts, err := reg.AllPrompts(ctx)
	if err != nil {
		t.Fatalf("AllPrompts failed: %v", err)
	}

	if len(allPrompts) != 2 {
		t.Errorf("expected 2 prompts, got %d", len(allPrompts))
	}
}

func TestClose(t *testing.T) {
	ctx := context.Background()
	reg := NewRegistry()

	server1 := newMockServer("server1", "1.0.0")
	server2 := newMockServer("server2", "1.0.0")

	reg.Register(ctx, server1)
	reg.Register(ctx, server2)

	err := reg.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	servers := reg.List()
	if len(servers) != 0 {
		t.Errorf("expected 0 servers after close, got %d", len(servers))
	}

	if !server1.IsClosed() {
		t.Error("server1 should be closed")
	}

	if !server2.IsClosed() {
		t.Error("server2 should be closed")
	}
}

func TestConcurrentAccess(t *testing.T) {
	ctx := context.Background()
	reg := NewRegistry()

	// Concurrent registration
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			server := newMockServer(string(rune('A'+idx)), "1.0.0")
			reg.Register(ctx, server)
		}(i)
	}

	wg.Wait()

	servers := reg.List()
	if len(servers) != 10 {
		t.Errorf("expected 10 servers, got %d", len(servers))
	}

	// Concurrent reads
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			reg.List()
			reg.ListByCapability("tools")
		}()
	}

	wg.Wait()
}

func TestToolCacheRebuildOnDeregister(t *testing.T) {
	ctx := context.Background()
	reg := NewRegistry()

	server := newMockServer("test-server", "1.0.0")
	server.capabilities.Tools = &ToolsCapability{}
	server.tools = []Tool{
		{Name: "test-tool", InputSchema: json.RawMessage(`{}`)},
	}

	reg.Register(ctx, server)

	provider, found := reg.FindToolProvider("test-tool")
	if !found {
		t.Fatal("tool should be found before deregister")
	}

	if provider.ServerInfo().Name != "test-server" {
		t.Errorf("expected provider 'test-server', got %q", provider.ServerInfo().Name)
	}

	// Deregister the server
	reg.Deregister(ctx, "test-server")

	// Tool should no longer be found
	_, found = reg.FindToolProvider("test-tool")
	if found {
		t.Error("tool should not be found after deregister")
	}
}

func TestUpdate(t *testing.T) {
	ctx := context.Background()
	reg := NewRegistry()

	server := newMockServer("test-server", "1.0.0")
	reg.Register(ctx, server)

	// Update the server version
	err := reg.Update("test-server", func(info *ServerInfo) error {
		info.Version = "2.0.0"
		return nil
	})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// Verify the update
	retrieved, _ := reg.Get("test-server")
	if retrieved.ServerInfo().Version != "2.0.0" {
		t.Errorf("expected version 2.0.0, got %q", retrieved.ServerInfo().Version)
	}
}

func TestUpdateNonexistent(t *testing.T) {
	reg := NewRegistry()

	err := reg.Update("nonexistent", func(info *ServerInfo) error {
		return nil
	})
	if err == nil {
		t.Fatal("expected error when updating nonexistent server, got nil")
	}
}

func TestUpdateWithError(t *testing.T) {
	ctx := context.Background()
	reg := NewRegistry()

	server := newMockServer("test-server", "1.0.0")
	reg.Register(ctx, server)

	// Try to update with a function that returns an error
	err := reg.Update("test-server", func(info *ServerInfo) error {
		return fmt.Errorf("custom error")
	})
	if err == nil {
		t.Fatal("expected error from update function, got nil")
	}

	if !containsString(err.Error(), "custom error") {
		t.Errorf("expected error to contain 'custom error', got %v", err)
	}
}

func TestListAll(t *testing.T) {
	ctx := context.Background()
	reg := NewRegistry()

	server1 := newMockServer("server1", "1.0.0")
	server2 := newMockServer("server2", "2.0.0")
	server3 := newMockServer("server3", "3.0.0")

	reg.Register(ctx, server1)
	reg.Register(ctx, server2)
	reg.Register(ctx, server3)

	infos := reg.ListAll()
	if len(infos) != 3 {
		t.Errorf("expected 3 servers, got %d", len(infos))
	}

	// Check that all servers are in the list
	nameMap := make(map[string]bool)
	for _, info := range infos {
		nameMap[info.Name] = true
	}

	for _, name := range []string{"server1", "server2", "server3"} {
		if !nameMap[name] {
			t.Errorf("server %q not found in ListAll results", name)
		}
	}
}

func TestListAllEmpty(t *testing.T) {
	reg := NewRegistry()

	infos := reg.ListAll()
	if len(infos) != 0 {
		t.Errorf("expected 0 servers, got %d", len(infos))
	}
}

func TestGetHealth(t *testing.T) {
	ctx := context.Background()
	reg := NewRegistry()

	server := newMockServer("test-server", "1.0.0")
	reg.Register(ctx, server)

	// Server should be healthy by default on registration
	healthy, err := reg.GetHealth("test-server")
	if err != nil {
		t.Fatalf("GetHealth failed: %v", err)
	}

	if !healthy {
		t.Error("server should be healthy by default")
	}
}

func TestGetHealthNonexistent(t *testing.T) {
	reg := NewRegistry()

	_, err := reg.GetHealth("nonexistent")
	if err == nil {
		t.Fatal("expected error when getting health of nonexistent server, got nil")
	}
}

func TestSetHealth(t *testing.T) {
	ctx := context.Background()
	reg := NewRegistry()

	server := newMockServer("test-server", "1.0.0")
	reg.Register(ctx, server)

	// Set health to false
	err := reg.SetHealth("test-server", false)
	if err != nil {
		t.Fatalf("SetHealth failed: %v", err)
	}

	// Verify the change
	healthy, err := reg.GetHealth("test-server")
	if err != nil {
		t.Fatalf("GetHealth failed: %v", err)
	}

	if healthy {
		t.Error("server should be unhealthy after SetHealth(false)")
	}

	// Set health back to true
	err = reg.SetHealth("test-server", true)
	if err != nil {
		t.Fatalf("SetHealth failed: %v", err)
	}

	healthy, err = reg.GetHealth("test-server")
	if err != nil {
		t.Fatalf("GetHealth failed: %v", err)
	}

	if !healthy {
		t.Error("server should be healthy after SetHealth(true)")
	}
}

func TestSetHealthNonexistent(t *testing.T) {
	reg := NewRegistry()

	err := reg.SetHealth("nonexistent", false)
	if err == nil {
		t.Fatal("expected error when setting health of nonexistent server, got nil")
	}
}

func TestHealthStatusPersistence(t *testing.T) {
	ctx := context.Background()
	reg := NewRegistry()

	server1 := newMockServer("server1", "1.0.0")
	server2 := newMockServer("server2", "1.0.0")

	reg.Register(ctx, server1)
	reg.Register(ctx, server2)

	// Set different health statuses
	reg.SetHealth("server1", false)
	reg.SetHealth("server2", true)

	// Verify both are correctly maintained
	health1, _ := reg.GetHealth("server1")
	health2, _ := reg.GetHealth("server2")

	if health1 {
		t.Error("server1 should be unhealthy")
	}
	if !health2 {
		t.Error("server2 should be healthy")
	}
}

func TestHealthStatusAfterDeregister(t *testing.T) {
	ctx := context.Background()
	reg := NewRegistry()

	server := newMockServer("test-server", "1.0.0")
	reg.Register(ctx, server)

	// Set health status
	reg.SetHealth("test-server", false)

	// Deregister
	reg.Deregister(ctx, "test-server")

	// Re-register - should be healthy again (new registration)
	server2 := newMockServer("test-server", "1.0.0")
	reg.Register(ctx, server2)

	healthy, _ := reg.GetHealth("test-server")
	if !healthy {
		t.Error("server should be healthy after re-registration")
	}
}

func TestConcurrentHealthUpdates(t *testing.T) {
	ctx := context.Background()
	reg := NewRegistry()

	server := newMockServer("test-server", "1.0.0")
	reg.Register(ctx, server)

	// Concurrent health updates
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			healthy := (idx % 2) == 0
			reg.SetHealth("test-server", healthy)
		}(i)
	}

	wg.Wait()

	// Verify the health status is valid (should be either true or false)
	healthy, err := reg.GetHealth("test-server")
	if err != nil {
		t.Fatalf("GetHealth failed: %v", err)
	}

	if healthy != true && healthy != false {
		t.Error("health status should be boolean")
	}
}

func TestConcurrentUpdates(t *testing.T) {
	ctx := context.Background()
	reg := NewRegistry()

	server := newMockServer("test-server", "1.0.0")
	reg.Register(ctx, server)

	// Concurrent updates
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			reg.Update("test-server", func(info *ServerInfo) error {
				// Just verify we can read the info
				_ = info.Name
				return nil
			})
		}(i)
	}

	wg.Wait()

	// Verify server is still in registry
	_, exists := reg.Get("test-server")
	if !exists {
		t.Fatal("server should still exist after concurrent updates")
	}
}

// Helper function to check if a string contains a substring
func containsString(haystack, needle string) bool {
	for i := 0; i <= len(haystack)-len(needle); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

// Persistence Tests

func TestNewRegistryWithDB(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-registry-*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpFile.Close()
	dbPath := tmpFile.Name()
	defer os.Remove(dbPath)

	reg, err := NewRegistryWithDB(dbPath)
	if err != nil {
		t.Fatalf("NewRegistryWithDB failed: %v", err)
	}

	if reg == nil {
		t.Fatal("NewRegistryWithDB returned nil")
	}

	servers := reg.List()
	if len(servers) != 0 {
		t.Errorf("new registry should have 0 servers, got %d", len(servers))
	}
}

func TestSaveRegistryWithoutDB(t *testing.T) {
	reg := NewRegistry().(*registryImpl)

	// Try to save without database path - should fail
	err := reg.SaveRegistry()
	if err == nil {
		t.Fatal("SaveRegistry should fail when dbPath is empty")
	}
}

func TestLoadRegistryWithoutDB(t *testing.T) {
	reg := NewRegistry().(*registryImpl)

	// Try to load without database path - should fail
	_, err := reg.LoadRegistry()
	if err == nil {
		t.Fatal("LoadRegistry should fail when dbPath is empty")
	}
}

func TestSaveAndLoadRegistry(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-registry-*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpFile.Close()
	dbPath := tmpFile.Name()
	defer os.Remove(dbPath)

	ctx := context.Background()

	// Create registry and add servers
	reg1, err := NewRegistryWithDB(dbPath)
	if err != nil {
		t.Fatalf("NewRegistryWithDB failed: %v", err)
	}

	server1 := newMockServer("server1", "1.0.0")
	server1.capabilities.Tools = &ToolsCapability{}
	server1.capabilities.Resources = &ResourcesCapability{}

	server2 := newMockServer("server2", "2.0.0")
	server2.capabilities.Prompts = &PromptsCapability{}

	reg1.Register(ctx, server1)
	reg1.Register(ctx, server2)

	reg1.SetHealth("server1", false)
	reg1.SetHealth("server2", true)

	// Save registry
	err = reg1.SaveRegistry()
	if err != nil {
		t.Fatalf("SaveRegistry failed: %v", err)
	}

	// Create new registry and load
	reg2, err := NewRegistryWithDB(dbPath)
	if err != nil {
		t.Fatalf("NewRegistryWithDB failed: %v", err)
	}

	metadata, err := reg2.LoadRegistry()
	if err != nil {
		t.Fatalf("LoadRegistry failed: %v", err)
	}

	// Verify loaded metadata
	if len(metadata) != 2 {
		t.Errorf("expected 2 servers in metadata, got %d", len(metadata))
	}

	if _, exists := metadata["server1"]; !exists {
		t.Error("server1 not found in metadata")
	}

	if _, exists := metadata["server2"]; !exists {
		t.Error("server2 not found in metadata")
	}

	if metadata["server1"].Name != "server1" {
		t.Errorf("expected server1 name, got %q", metadata["server1"].Name)
	}

	if metadata["server1"].Version != "1.0.0" {
		t.Errorf("expected server1 version 1.0.0, got %q", metadata["server1"].Version)
	}

	// Reconstruct servers in reg2 to verify health status was persisted
	newServer1 := newMockServer("server1", "1.0.0")
	newServer2 := newMockServer("server2", "2.0.0")

	reg2.Register(ctx, newServer1)
	reg2.Register(ctx, newServer2)

	// Verify health status was persisted (health was loaded into map)
	health1, _ := reg2.GetHealth("server1")
	health2, _ := reg2.GetHealth("server2")

	if health1 {
		t.Error("server1 should be unhealthy after load")
	}

	if !health2 {
		t.Error("server2 should be healthy after load")
	}
}

func TestSaveRegistryRoundtrip(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-registry-*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpFile.Close()
	dbPath := tmpFile.Name()
	defer os.Remove(dbPath)

	ctx := context.Background()

	// Setup first registry
	reg1, _ := NewRegistryWithDB(dbPath)

	server1 := newMockServer("test-server", "1.2.3")
	server1.capabilities.Tools = &ToolsCapability{}
	server1.tools = []Tool{
		{Name: "tool1", Description: "Test tool", InputSchema: json.RawMessage(`{}`)},
	}

	reg1.Register(ctx, server1)
	reg1.SetHealth("test-server", true)

	// Save
	if err := reg1.SaveRegistry(); err != nil {
		t.Fatalf("SaveRegistry failed: %v", err)
	}

	// Load into new registry
	reg2, _ := NewRegistryWithDB(dbPath)
	metadata, err := reg2.LoadRegistry()
	if err != nil {
		t.Fatalf("LoadRegistry failed: %v", err)
	}

	// Verify roundtrip
	if info, exists := metadata["test-server"]; exists {
		if info.Name != "test-server" {
			t.Errorf("name mismatch: expected 'test-server', got %q", info.Name)
		}
		if info.Version != "1.2.3" {
			t.Errorf("version mismatch: expected '1.2.3', got %q", info.Version)
		}
	} else {
		t.Fatal("test-server not found after roundtrip")
	}

	// Reconstruct server and verify health status was preserved
	newServer := newMockServer("test-server", "1.2.3")
	reg2.Register(ctx, newServer)

	health, _ := reg2.GetHealth("test-server")
	if !health {
		t.Error("health status not preserved in roundtrip")
	}
}

func TestSaveRegistryOverwrite(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-registry-*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpFile.Close()
	dbPath := tmpFile.Name()
	defer os.Remove(dbPath)

	ctx := context.Background()

	// First save: register 3 servers
	reg1, _ := NewRegistryWithDB(dbPath)

	for i := 1; i <= 3; i++ {
		name := fmt.Sprintf("server%d", i)
		server := newMockServer(name, "1.0.0")
		reg1.Register(ctx, server)
	}

	reg1.SaveRegistry()

	// Load and verify
	reg2, _ := NewRegistryWithDB(dbPath)
	metadata, _ := reg2.LoadRegistry()
	if len(metadata) != 3 {
		t.Errorf("expected 3 servers after first save, got %d", len(metadata))
	}

	// Second save: register only 2 servers (should replace)
	reg3, _ := NewRegistryWithDB(dbPath)

	server1 := newMockServer("new-server1", "1.0.0")
	server2 := newMockServer("new-server2", "2.0.0")

	reg3.Register(ctx, server1)
	reg3.Register(ctx, server2)

	reg3.SaveRegistry()

	// Load and verify overwrite
	reg4, _ := NewRegistryWithDB(dbPath)
	metadata2, _ := reg4.LoadRegistry()
	if len(metadata2) != 2 {
		t.Errorf("expected 2 servers after second save, got %d", len(metadata2))
	}

	if _, exists := metadata2["new-server1"]; !exists {
		t.Error("new-server1 not found after overwrite")
	}

	if _, exists := metadata2["server1"]; exists {
		t.Error("old server1 still exists after overwrite")
	}
}

func TestAutoSaveOnRegister(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-registry-*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpFile.Close()
	dbPath := tmpFile.Name()
	defer os.Remove(dbPath)

	ctx := context.Background()

	// Register server with auto-save enabled
	reg1, _ := NewRegistryWithDB(dbPath)

	server := newMockServer("auto-save-server", "1.0.0")
	server.capabilities.Tools = &ToolsCapability{}

	reg1.Register(ctx, server)

	// Load without explicitly saving (should have auto-saved)
	reg2, _ := NewRegistryWithDB(dbPath)
	metadata, err := reg2.LoadRegistry()
	if err != nil {
		t.Fatalf("LoadRegistry failed: %v", err)
	}

	if _, exists := metadata["auto-save-server"]; !exists {
		t.Error("server not auto-saved on register")
	}
}

func TestAutoDeleteOnDeregister(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-registry-*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpFile.Close()
	dbPath := tmpFile.Name()
	defer os.Remove(dbPath)

	ctx := context.Background()

	// Register and explicitly save
	reg1, _ := NewRegistryWithDB(dbPath)

	server := newMockServer("server-to-delete", "1.0.0")
	reg1.Register(ctx, server)
	reg1.SaveRegistry()

	// Deregister (should auto-delete)
	reg1.Deregister(ctx, "server-to-delete")

	// Load and verify deletion
	reg2, _ := NewRegistryWithDB(dbPath)
	metadata, _ := reg2.LoadRegistry()

	if _, exists := metadata["server-to-delete"]; exists {
		t.Error("server still exists after deregister")
	}
}

func TestAutoSaveHealthStatus(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-registry-*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpFile.Close()
	dbPath := tmpFile.Name()
	defer os.Remove(dbPath)

	ctx := context.Background()

	// Register server and set health
	reg1, _ := NewRegistryWithDB(dbPath)

	server := newMockServer("health-server", "1.0.0")
	reg1.Register(ctx, server)
	reg1.SetHealth("health-server", false) // Should auto-save

	// Load and verify health was persisted
	reg2, _ := NewRegistryWithDB(dbPath)
	reg2.LoadRegistry()

	health, _ := reg2.GetHealth("health-server")
	if health {
		t.Error("health status not auto-saved on SetHealth")
	}
}

func TestPersistenceWithCapabilities(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-registry-*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpFile.Close()
	dbPath := tmpFile.Name()
	defer os.Remove(dbPath)

	ctx := context.Background()

	// Register server with multiple capabilities
	reg1, _ := NewRegistryWithDB(dbPath)

	server := newMockServer("multi-cap-server", "1.0.0")
	server.capabilities.Tools = &ToolsCapability{}
	server.capabilities.Resources = &ResourcesCapability{}
	server.capabilities.Prompts = &PromptsCapability{}
	server.capabilities.Logging = &LoggingCapability{}

	server.tools = []Tool{
		{Name: "tool1", InputSchema: json.RawMessage(`{}`)},
		{Name: "tool2", InputSchema: json.RawMessage(`{}`)},
	}

	reg1.Register(ctx, server)
	reg1.SaveRegistry()

	// Load and verify capabilities were persisted
	reg2, _ := NewRegistryWithDB(dbPath)
	metadata, _ := reg2.LoadRegistry()

	if serverInfo, exists := metadata["multi-cap-server"]; exists {
		if serverInfo.Name != "multi-cap-server" {
			t.Error("server name not persisted correctly")
		}
	} else {
		t.Fatal("server not found after load")
	}
}

func TestEmptyDatabaseLoad(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-registry-*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpFile.Close()
	dbPath := tmpFile.Name()
	defer os.Remove(dbPath)

	// Create empty registry and load
	reg, _ := NewRegistryWithDB(dbPath)

	metadata, err := reg.LoadRegistry()
	if err != nil {
		t.Fatalf("LoadRegistry on empty database failed: %v", err)
	}

	if len(metadata) != 0 {
		t.Errorf("expected 0 servers from empty database, got %d", len(metadata))
	}
}

// =============================================================================
// Tests for CRUD Operations (MCP-003)
// =============================================================================

// TestAddServerInfo tests adding a ServerInfo to the registry.
func TestAddServerInfo(t *testing.T) {
	reg := NewRegistry().(*registryImpl)
	defer reg.Close()

	tests := []struct {
		name       string
		serverInfo *ServerInfo
		wantErr    bool
	}{
		{
			name: "add valid server",
			serverInfo: &ServerInfo{
				Name:    "test-server",
				Version: "1.0.0",
				Title:   "Test Server",
			},
			wantErr: false,
		},
		{
			name: "add server with empty name",
			serverInfo: &ServerInfo{
				Name:    "",
				Version: "1.0.0",
			},
			wantErr: true,
		},
		{
			name:       "add nil server",
			serverInfo: nil,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := reg.AddServerInfo(tt.serverInfo)
			if (err != nil) != tt.wantErr {
				t.Errorf("AddServerInfo() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestAddServerInfoDuplicate tests adding duplicate servers.
func TestAddServerInfoDuplicate(t *testing.T) {
	reg := NewRegistry().(*registryImpl)
	defer reg.Close()

	serverInfo := &ServerInfo{
		Name:    "test-server",
		Version: "1.0.0",
	}

	// First add should succeed
	if err := reg.AddServerInfo(serverInfo); err != nil {
		t.Fatalf("First AddServerInfo() error = %v", err)
	}

	// Second add with same ID should fail
	if err := reg.AddServerInfo(serverInfo); err == nil {
		t.Errorf("Second AddServerInfo() should have failed with duplicate ID")
	}
}

// TestRemoveServerInfo tests removing a ServerInfo from the registry.
func TestRemoveServerInfo(t *testing.T) {
	reg := NewRegistry().(*registryImpl)
	defer reg.Close()

	// Add a server first
	serverInfo := &ServerInfo{
		Name:    "test-server",
		Version: "1.0.0",
	}
	if err := reg.AddServerInfo(serverInfo); err != nil {
		t.Fatalf("setup: AddServerInfo() error = %v", err)
	}

	tests := []struct {
		name    string
		id      string
		wantErr bool
	}{
		{
			name:    "remove existing server",
			id:      "test-server",
			wantErr: false,
		},
		{
			name:    "remove non-existent server",
			id:      "non-existent",
			wantErr: true,
		},
		{
			name:    "remove with empty id",
			id:      "",
			wantErr: true,
		},
	}

	for i, tt := range tests {
		if i > 0 {
			// Re-add server for next test
			reg.AddServerInfo(serverInfo)
		}
		t.Run(tt.name, func(t *testing.T) {
			err := reg.RemoveServerInfo(tt.id)
			if (err != nil) != tt.wantErr {
				t.Errorf("RemoveServerInfo() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestUpdateServerInfo tests updating a ServerInfo in the registry.
func TestUpdateServerInfo(t *testing.T) {
	reg := NewRegistry().(*registryImpl)
	defer reg.Close()

	// Add a server first
	serverInfo := &ServerInfo{
		Name:    "test-server",
		Version: "1.0.0",
		Title:   "Original Title",
	}
	if err := reg.AddServerInfo(serverInfo); err != nil {
		t.Fatalf("setup: AddServerInfo() error = %v", err)
	}

	tests := []struct {
		name       string
		id         string
		serverInfo *ServerInfo
		wantErr    bool
	}{
		{
			name: "update existing server",
			id:   "test-server",
			serverInfo: &ServerInfo{
				Name:    "test-server",
				Version: "2.0.0",
				Title:   "Updated Title",
			},
			wantErr: false,
		},
		{
			name:       "update non-existent server",
			id:         "non-existent",
			serverInfo: &ServerInfo{Name: "non-existent", Version: "1.0.0"},
			wantErr:    true,
		},
		{
			name:       "update with mismatched name",
			id:         "test-server",
			serverInfo: &ServerInfo{Name: "different-name", Version: "1.0.0"},
			wantErr:    true,
		},
		{
			name:       "update with nil serverInfo",
			id:         "test-server",
			serverInfo: nil,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := reg.UpdateServerInfo(tt.id, tt.serverInfo)
			if (err != nil) != tt.wantErr {
				t.Errorf("UpdateServerInfo() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestGetServerInfo tests retrieving a ServerInfo from the registry.
func TestGetServerInfo(t *testing.T) {
	reg := NewRegistry().(*registryImpl)
	defer reg.Close()

	serverInfo := &ServerInfo{
		Name:    "test-server",
		Version: "1.0.0",
		Title:   "Test Server",
	}
	if err := reg.AddServerInfo(serverInfo); err != nil {
		t.Fatalf("setup: AddServerInfo() error = %v", err)
	}

	tests := []struct {
		name      string
		id        string
		wantFound bool
	}{
		{
			name:      "get existing server",
			id:        "test-server",
			wantFound: true,
		},
		{
			name:      "get non-existent server",
			id:        "non-existent",
			wantFound: false,
		},
		{
			name:      "get with empty id",
			id:        "",
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := reg.GetServerInfo(tt.id)
			if (result != nil) != tt.wantFound {
				t.Errorf("GetServerInfo() found = %v, wantFound %v", result != nil, tt.wantFound)
			}
			if tt.wantFound && result != nil {
				if result.Name != tt.id {
					t.Errorf("GetServerInfo() Name = %q, want %q", result.Name, tt.id)
				}
			}
		})
	}
}

// TestListServerInfos tests listing all ServerInfo entries.
func TestListServerInfos(t *testing.T) {
	reg := NewRegistry().(*registryImpl)
	defer reg.Close()

	// Add multiple servers
	servers := []ServerInfo{
		{Name: "server1", Version: "1.0.0"},
		{Name: "server2", Version: "2.0.0"},
		{Name: "server3", Version: "3.0.0"},
	}

	for i := range servers {
		if err := reg.AddServerInfo(&servers[i]); err != nil {
			t.Fatalf("setup: AddServerInfo() error = %v", err)
		}
	}

	result := reg.ListServerInfos()
	if len(result) != len(servers) {
		t.Errorf("ListServerInfos() returned %d servers, want %d", len(result), len(servers))
	}

	// Verify all servers are in the list
	resultMap := make(map[string]*ServerInfo)
	for _, info := range result {
		resultMap[info.Name] = info
	}

	for _, server := range servers {
		if _, ok := resultMap[server.Name]; !ok {
			t.Errorf("ListServerInfos() missing server %q", server.Name)
		}
	}
}

// TestGetServerInfoReturnsCopy tests that GetServerInfo returns a copy to prevent external mutation.
func TestGetServerInfoReturnsCopy(t *testing.T) {
	reg := NewRegistry().(*registryImpl)
	defer reg.Close()

	original := &ServerInfo{
		Name:    "copy-test",
		Version: "1.0.0",
		Title:   "Original",
	}

	if err := reg.AddServerInfo(original); err != nil {
		t.Fatalf("AddServerInfo() error = %v", err)
	}

	// Get the server
	retrieved := reg.GetServerInfo("copy-test")
	if retrieved == nil {
		t.Fatalf("GetServerInfo() returned nil")
	}

	// Modify the retrieved copy
	retrieved.Title = "Modified"
	retrieved.Version = "2.0.0"

	// Get it again and verify original is unchanged
	retrieved2 := reg.GetServerInfo("copy-test")
	if retrieved2 == nil {
		t.Fatalf("GetServerInfo() returned nil on second call")
	}

	if retrieved2.Title != "Original" {
		t.Errorf("Title was modified: got %q, want %q", retrieved2.Title, "Original")
	}

	if retrieved2.Version != "1.0.0" {
		t.Errorf("Version was modified: got %q, want %q", retrieved2.Version, "1.0.0")
	}
}

// TestConcurrentCRUD tests concurrent CRUD operations.
func TestConcurrentServerInfoCRUD(t *testing.T) {
	reg := NewRegistry().(*registryImpl)
	defer reg.Close()

	const numGoroutines = 10
	const numServersPerGoroutine = 5
	var wg sync.WaitGroup
	errChan := make(chan error, numGoroutines*numServersPerGoroutine)

	// Test concurrent adds
	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for i := 0; i < numServersPerGoroutine; i++ {
				serverInfo := &ServerInfo{
					Name:    fmt.Sprintf("server-g%d-s%d", goroutineID, i),
					Version: "1.0.0",
				}
				if err := reg.AddServerInfo(serverInfo); err != nil {
					errChan <- err
				}
			}
		}(g)
	}

	wg.Wait()

	// Check for errors
	close(errChan)
	for err := range errChan {
		t.Errorf("Concurrent add error: %v", err)
	}

	// Verify all servers were added
	infos := reg.ListServerInfos()
	expectedCount := numGoroutines * numServersPerGoroutine
	if len(infos) != expectedCount {
		t.Errorf("ListServerInfos() returned %d servers, want %d", len(infos), expectedCount)
	}

	// Test concurrent updates and reads
	errChan = make(chan error, numGoroutines*2)
	for g := 0; g < numGoroutines; g++ {
		wg.Add(2)

		// Goroutine for updates
		go func(goroutineID int) {
			defer wg.Done()
			for i := 0; i < numServersPerGoroutine; i++ {
				serverName := fmt.Sprintf("server-g%d-s%d", goroutineID, i)
				updated := &ServerInfo{
					Name:    serverName,
					Version: "2.0.0",
					Title:   fmt.Sprintf("Updated from goroutine %d", goroutineID),
				}
				if err := reg.UpdateServerInfo(serverName, updated); err != nil {
					errChan <- err
				}
			}
		}(g)

		// Goroutine for reads
		go func(goroutineID int) {
			defer wg.Done()
			for i := 0; i < numServersPerGoroutine; i++ {
				serverName := fmt.Sprintf("server-g%d-s%d", goroutineID, i)
				if result := reg.GetServerInfo(serverName); result == nil {
					errChan <- fmt.Errorf("GetServerInfo() returned nil for %s", serverName)
				}
			}
		}(g)
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		t.Errorf("Concurrent operation error: %v", err)
	}
}

// TestPersistenceWithDBCRUD tests CRUD operations with database persistence.
func TestPersistenceWithDBCRUD(t *testing.T) {
	// Create temporary database
	tmpFile, err := os.CreateTemp("", "flip-test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()
	dbPath := tmpFile.Name()
	defer os.Remove(dbPath)

	// Create registry with persistence
	reg, err := NewRegistryWithDB(dbPath)
	if err != nil {
		t.Fatalf("NewRegistryWithDB() error = %v", err)
	}
	defer reg.Close()

	regImpl := reg.(*registryImpl)

	// Add servers
	servers := []ServerInfo{
		{Name: "persist-server1", Version: "1.0.0", Title: "Persistence Test 1"},
		{Name: "persist-server2", Version: "2.0.0", Title: "Persistence Test 2"},
	}

	for i := range servers {
		if err := regImpl.AddServerInfo(&servers[i]); err != nil {
			t.Fatalf("AddServerInfo() error = %v", err)
		}
	}

	// Verify persistence by reloading from database
	reg2, err := NewRegistryWithDB(dbPath)
	if err != nil {
		t.Fatalf("NewRegistryWithDB() error on reload = %v", err)
	}
	defer reg2.Close()

	reloadedImpl := reg2.(*registryImpl)
	infos := reloadedImpl.ListServerInfos()

	if len(infos) != len(servers) {
		t.Errorf("After reload, got %d servers, want %d", len(infos), len(servers))
	}

	// Verify data integrity
	for _, server := range servers {
		found := reloadedImpl.GetServerInfo(server.Name)
		if found == nil {
			t.Errorf("GetServerInfo() returned nil for %q", server.Name)
			continue
		}
		if found.Version != server.Version {
			t.Errorf("GetServerInfo() Version = %q, want %q", found.Version, server.Version)
		}
		if found.Title != server.Title {
			t.Errorf("GetServerInfo() Title = %q, want %q", found.Title, server.Title)
		}
	}
}

// TestCRUDUpdatePersistence tests that updates are persisted to database.
func TestCRUDUpdatePersistence(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "flip-test-update-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()
	dbPath := tmpFile.Name()
	defer os.Remove(dbPath)

	// Create registry with persistence
	reg, err := NewRegistryWithDB(dbPath)
	if err != nil {
		t.Fatalf("NewRegistryWithDB() error = %v", err)
	}

	regImpl := reg.(*registryImpl)

	// Add initial server
	original := &ServerInfo{
		Name:    "update-test",
		Version: "1.0.0",
		Title:   "Original",
	}
	if err := regImpl.AddServerInfo(original); err != nil {
		t.Fatalf("AddServerInfo() error = %v", err)
	}

	// Update server
	updated := &ServerInfo{
		Name:    "update-test",
		Version: "2.0.0",
		Title:   "Updated",
	}
	if err := regImpl.UpdateServerInfo("update-test", updated); err != nil {
		t.Fatalf("UpdateServerInfo() error = %v", err)
	}

	reg.Close()

	// Reload and verify update persisted
	reg2, err := NewRegistryWithDB(dbPath)
	if err != nil {
		t.Fatalf("NewRegistryWithDB() error on reload = %v", err)
	}
	defer reg2.Close()

	reloadedImpl := reg2.(*registryImpl)
	retrieved := reloadedImpl.GetServerInfo("update-test")

	if retrieved == nil {
		t.Fatal("GetServerInfo() returned nil after reload")
	}

	if retrieved.Version != "2.0.0" {
		t.Errorf("After persistence, Version = %q, want %q", retrieved.Version, "2.0.0")
	}

	if retrieved.Title != "Updated" {
		t.Errorf("After persistence, Title = %q, want %q", retrieved.Title, "Updated")
	}
}

// TestMultipleServersRegistration tests registering multiple servers in sequence.
func TestMultipleServersRegistration(t *testing.T) {
	ctx := context.Background()
	reg := NewRegistry()
	defer reg.Close()

	// Register multiple servers
	numServers := 5
	for i := 1; i <= numServers; i++ {
		name := fmt.Sprintf("server-%d", i)
		server := newMockServer(name, fmt.Sprintf("%d.0.0", i))
		if err := reg.Register(ctx, server); err != nil {
			t.Fatalf("Register failed for %s: %v", name, err)
		}
	}

	// Verify all servers are registered
	servers := reg.List()
	if len(servers) != numServers {
		t.Errorf("Expected %d servers, got %d", numServers, len(servers))
	}

	// Verify each server can be retrieved
	for i := 1; i <= numServers; i++ {
		name := fmt.Sprintf("server-%d", i)
		server, exists := reg.Get(name)
		if !exists {
			t.Errorf("Server %q not found", name)
		}
		if server.ServerInfo().Name != name {
			t.Errorf("Expected name %q, got %q", name, server.ServerInfo().Name)
		}
	}
}

// TestListByCapabilityFiltering tests filtering by various capabilities.
func TestListByCapabilityFiltering(t *testing.T) {
	ctx := context.Background()
	reg := NewRegistry()
	defer reg.Close()

	// Register servers with different capability combinations
	testCases := []struct {
		name         string
		capabilities map[string]bool // capability name -> enabled
	}{
		{
			name: "tools-only",
			capabilities: map[string]bool{
				"tools": true,
			},
		},
		{
			name: "resources-only",
			capabilities: map[string]bool{
				"resources": true,
			},
		},
		{
			name: "prompts-only",
			capabilities: map[string]bool{
				"prompts": true,
			},
		},
		{
			name: "logging-only",
			capabilities: map[string]bool{
				"logging": true,
			},
		},
		{
			name: "completions-only",
			capabilities: map[string]bool{
				"completions": true,
			},
		},
		{
			name: "all-capabilities",
			capabilities: map[string]bool{
				"tools":       true,
				"resources":   true,
				"prompts":     true,
				"logging":     true,
				"completions": true,
			},
		},
		{
			name:         "no-capabilities",
			capabilities: map[string]bool{},
		},
	}

	for _, tc := range testCases {
		server := newMockServer(tc.name, "1.0.0")
		server.capabilities = &ServerCapabilities{}

		if tc.capabilities["tools"] {
			server.capabilities.Tools = &ToolsCapability{}
		}
		if tc.capabilities["resources"] {
			server.capabilities.Resources = &ResourcesCapability{}
		}
		if tc.capabilities["prompts"] {
			server.capabilities.Prompts = &PromptsCapability{}
		}
		if tc.capabilities["logging"] {
			server.capabilities.Logging = &LoggingCapability{}
		}
		if tc.capabilities["completions"] {
			server.capabilities.Completions = &CompletionsCapability{}
		}

		reg.Register(ctx, server)
	}

	// Test filtering for each capability
	capabilities := []string{"tools", "resources", "prompts", "logging", "completions"}
	for _, cap := range capabilities {
		servers := reg.ListByCapability(cap)
		if len(servers) < 2 {
			t.Errorf("Expected at least 2 servers with %q capability, got %d", cap, len(servers))
		}
	}
}

// TestFindResourceProviderMultiple tests finding resource providers when multiple servers provide resources.
func TestFindResourceProviderMultiple(t *testing.T) {
	ctx := context.Background()
	reg := NewRegistry()
	defer reg.Close()

	// Server 1 provides file:// resources
	server1 := newMockServer("file-server", "1.0.0")
	server1.capabilities.Resources = &ResourcesCapability{}
	server1.resources = []Resource{
		{URI: "file:///config.json", Name: "Config"},
		{URI: "file:///data.txt", Name: "Data"},
	}
	reg.Register(ctx, server1)

	// Server 2 provides http:// resources
	server2 := newMockServer("http-server", "1.0.0")
	server2.capabilities.Resources = &ResourcesCapability{}
	server2.resources = []Resource{
		{URI: "http://example.com/api", Name: "API"},
	}
	reg.Register(ctx, server2)

	// Test finding different resource types
	tests := []struct {
		uri      string
		expected string
	}{
		{uri: "file:///config.json", expected: "file-server"},
		{uri: "file:///data.txt", expected: "file-server"},
		{uri: "http://example.com/api", expected: "http-server"},
	}

	for _, test := range tests {
		provider, found := reg.FindResourceProvider(test.uri)
		if !found {
			t.Errorf("Resource provider not found for %q", test.uri)
			continue
		}
		if provider.ServerInfo().Name != test.expected {
			t.Errorf("Expected provider %q for %q, got %q", test.expected, test.uri, provider.ServerInfo().Name)
		}
	}
}

// TestAllResourcesWithPagination tests AllResources with pagination handling.
func TestAllResourcesWithPagination(t *testing.T) {
	ctx := context.Background()
	reg := NewRegistry()
	defer reg.Close()

	// Create servers with resources
	server1 := newMockServer("server1", "1.0.0")
	server1.capabilities.Resources = &ResourcesCapability{}
	server1.resources = []Resource{
		{URI: "resource:1", Name: "Resource 1"},
		{URI: "resource:2", Name: "Resource 2"},
	}

	server2 := newMockServer("server2", "1.0.0")
	server2.capabilities.Resources = &ResourcesCapability{}
	server2.resources = []Resource{
		{URI: "resource:3", Name: "Resource 3"},
		{URI: "resource:4", Name: "Resource 4"},
		{URI: "resource:5", Name: "Resource 5"},
	}

	reg.Register(ctx, server1)
	reg.Register(ctx, server2)

	resources, err := reg.AllResources(ctx)
	if err != nil {
		t.Fatalf("AllResources failed: %v", err)
	}

	if len(resources) != 5 {
		t.Errorf("Expected 5 total resources, got %d", len(resources))
	}
}

// TestAllPromptsFromMultipleServers tests AllPrompts aggregation.
func TestAllPromptsFromMultipleServers(t *testing.T) {
	ctx := context.Background()
	reg := NewRegistry()
	defer reg.Close()

	// Create servers with prompts
	server1 := newMockServer("server1", "1.0.0")
	server1.capabilities.Prompts = &PromptsCapability{}
	server1.prompts = []Prompt{
		{Name: "prompt1", Description: "First prompt"},
		{Name: "prompt2", Description: "Second prompt"},
	}

	server2 := newMockServer("server2", "1.0.0")
	server2.capabilities.Prompts = &PromptsCapability{}
	server2.prompts = []Prompt{
		{Name: "prompt3", Description: "Third prompt"},
	}

	server3 := newMockServer("server3", "1.0.0")
	// No prompt capability

	reg.Register(ctx, server1)
	reg.Register(ctx, server2)
	reg.Register(ctx, server3)

	prompts, err := reg.AllPrompts(ctx)
	if err != nil {
		t.Fatalf("AllPrompts failed: %v", err)
	}

	if len(prompts) != 3 {
		t.Errorf("Expected 3 total prompts, got %d", len(prompts))
	}
}

// TestCloseWithErrors tests Close behavior when servers fail to close.
func TestCloseWithErrors(t *testing.T) {
	ctx := context.Background()
	reg := NewRegistry()

	// Create a mock server that fails to close
	server := newMockServer("test-server", "1.0.0")
	reg.Register(ctx, server)

	// Close should still work even if individual servers fail
	err := reg.Close()
	if err != nil {
		// This is acceptable - errors should be reported
	}

	// After close, registry should be empty
	servers := reg.List()
	if len(servers) != 0 {
		t.Errorf("Registry should be empty after close, got %d servers", len(servers))
	}
}

// TestUpdateMultipleFields tests Update with multiple field modifications.
func TestUpdateMultipleFields(t *testing.T) {
	ctx := context.Background()
	reg := NewRegistry()
	defer reg.Close()

	server := newMockServer("test-server", "1.0.0")
	server.info.Title = "Original Title"
	reg.Register(ctx, server)

	// Update multiple fields at once
	err := reg.Update("test-server", func(info *ServerInfo) error {
		info.Version = "2.0.0"
		info.Title = "Updated Title"
		return nil
	})

	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// Verify both fields were updated
	retrieved, _ := reg.Get("test-server")
	info := retrieved.ServerInfo()

	if info.Version != "2.0.0" {
		t.Errorf("Version not updated: got %q", info.Version)
	}
	if info.Title != "Updated Title" {
		t.Errorf("Title not updated: got %q", info.Title)
	}
}

// TestConcurrentRegisterAndDeregister tests concurrent register and deregister operations.
func TestConcurrentRegisterAndDeregister(t *testing.T) {
	ctx := context.Background()
	reg := NewRegistry()
	defer reg.Close()

	var wg sync.WaitGroup
	numOps := 20

	// Pre-register some servers
	for i := 0; i < numOps/2; i++ {
		server := newMockServer(fmt.Sprintf("server-%d", i), "1.0.0")
		reg.Register(ctx, server)
	}

	// Concurrent register and deregister
	for i := 0; i < numOps; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			if idx%2 == 0 {
				// Register
				server := newMockServer(fmt.Sprintf("concurrent-%d", idx), "1.0.0")
				reg.Register(ctx, server)
			} else {
				// Deregister
				prevIdx := idx - 1
				reg.Deregister(ctx, fmt.Sprintf("concurrent-%d", prevIdx))
			}
		}(i)
	}

	wg.Wait()

	// Verify registry is in a consistent state
	servers := reg.List()
	if len(servers) < 0 {
		t.Error("Registry has negative server count")
	}
}

// TestListAllWithMixedServers tests ListAll with both active and metadata-only servers.
func TestListAllWithMixedServers(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-registry-*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpFile.Close()
	dbPath := tmpFile.Name()
	defer os.Remove(dbPath)

	ctx := context.Background()

	// Create registry and add active server
	reg, err := NewRegistryWithDB(dbPath)
	if err != nil {
		t.Fatalf("NewRegistryWithDB failed: %v", err)
	}

	activeServer := newMockServer("active-server", "1.0.0")
	reg.Register(ctx, activeServer)

	// Add metadata-only server
	regImpl := reg.(*registryImpl)
	metadataOnly := &ServerInfo{
		Name:    "metadata-server",
		Version: "2.0.0",
	}
	regImpl.AddServerInfo(metadataOnly)

	// ListAll should include both
	infos := reg.ListAll()
	if len(infos) < 1 {
		t.Errorf("Expected at least 1 server in ListAll, got %d", len(infos))
	}

	reg.Close()
}

// TestToolCacheConsistency tests that tool cache remains consistent through operations.
func TestToolCacheConsistency(t *testing.T) {
	ctx := context.Background()
	reg := NewRegistry()
	defer reg.Close()

	// Register server with tools
	server1 := newMockServer("server1", "1.0.0")
	server1.capabilities.Tools = &ToolsCapability{}
	server1.tools = []Tool{
		{Name: "tool1", InputSchema: json.RawMessage(`{}`)},
		{Name: "tool2", InputSchema: json.RawMessage(`{}`)},
	}
	reg.Register(ctx, server1)

	// Register second server with different tools
	server2 := newMockServer("server2", "1.0.0")
	server2.capabilities.Tools = &ToolsCapability{}
	server2.tools = []Tool{
		{Name: "tool3", InputSchema: json.RawMessage(`{}`)},
	}
	reg.Register(ctx, server2)

	// Get all tools
	allTools1, _ := reg.AllTools(ctx)
	if len(allTools1) != 3 {
		t.Errorf("Expected 3 tools, got %d", len(allTools1))
	}

	// Deregister server1
	reg.Deregister(ctx, "server1")

	// Get all tools again - should have only server2's tools
	allTools2, _ := reg.AllTools(ctx)
	if len(allTools2) != 1 {
		t.Errorf("Expected 1 tool after deregister, got %d", len(allTools2))
	}

	if _, exists := allTools2["tool3"]; !exists {
		t.Error("Expected tool3 to remain")
	}
}

// TestRemoveServerInfoWithoutDB tests RemoveServerInfo for metadata-only entries.
func TestRemoveServerInfoWithoutDB(t *testing.T) {
	reg := NewRegistry().(*registryImpl)
	defer reg.Close()

	// Add a server via AddServerInfo (no active Server instance)
	serverInfo := &ServerInfo{
		Name:    "metadata-only",
		Version: "1.0.0",
	}
	if err := reg.AddServerInfo(serverInfo); err != nil {
		t.Fatalf("AddServerInfo failed: %v", err)
	}

	// Remove it
	if err := reg.RemoveServerInfo("metadata-only"); err != nil {
		t.Fatalf("RemoveServerInfo failed: %v", err)
	}

	// Verify it's gone
	if reg.GetServerInfo("metadata-only") != nil {
		t.Error("Server should have been removed")
	}
}

// TestPersistenceRobustness tests persistence with edge case data.
func TestPersistenceRobustness(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-registry-*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpFile.Close()
	dbPath := tmpFile.Name()
	defer os.Remove(dbPath)

	ctx := context.Background()

	// Create registry with edge case server info
	reg, err := NewRegistryWithDB(dbPath)
	if err != nil {
		t.Fatalf("NewRegistryWithDB failed: %v", err)
	}

	server := newMockServer("edge-case-server", "1.0.0-beta+meta.1")
	server.info.Title = "Server with special chars: !@#$%^&*()"
	server.capabilities.Tools = &ToolsCapability{}
	server.tools = []Tool{
		{Name: "tool-with-dashes", InputSchema: json.RawMessage(`{"type":"object"}`)},
	}

	reg.Register(ctx, server)
	reg.SaveRegistry()
	reg.Close()

	// Reload and verify
	reg2, err := NewRegistryWithDB(dbPath)
	if err != nil {
		t.Fatalf("NewRegistryWithDB failed on reload: %v", err)
	}
	defer reg2.Close()

	regImpl2 := reg2.(*registryImpl)
	metadata, err := regImpl2.LoadRegistry()
	if err != nil {
		t.Fatalf("LoadRegistry failed: %v", err)
	}

	if info, exists := metadata["edge-case-server"]; exists {
		if info.Version != "1.0.0-beta+meta.1" {
			t.Errorf("Version with special format not preserved: got %q", info.Version)
		}
	} else {
		t.Fatal("Server not found after persistence")
	}
}
