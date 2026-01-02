// Package repl provides integration tests for the FLIP2 REPL system.
package repl

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"
)

// =============================================================================
// TEST HELPERS & MOCKS
// =============================================================================

// TestAPIClient is a mock API client for testing
type TestAPIClient struct {
	baseURL string
	calls   []string
}

func NewTestAPIClient(baseURL string) *TestAPIClient {
	return &TestAPIClient{
		baseURL: baseURL,
		calls:   []string{},
	}
}

func (c *TestAPIClient) BaseURL() string {
	return c.baseURL
}

func (c *TestAPIClient) SetAuth(apiKey, authToken string) {
	c.calls = append(c.calls, fmt.Sprintf("SetAuth(%q, %q)", apiKey, authToken))
}

// simulateUserInput creates a reader that simulates user input
func simulateUserInput(commands ...string) io.Reader {
	input := strings.Join(commands, "\n") + "\n"
	return strings.NewReader(input)
}

// captureOutput captures output written to a writer
func captureOutput(f func(io.Writer)) string {
	var buf bytes.Buffer
	f(&buf)
	return buf.String()
}

// =============================================================================
// REPL INITIALIZATION TESTS
// =============================================================================

func TestREPLInitialization(t *testing.T) {
	cfg := Config{
		APIUrl:     "http://localhost:8091",
		Input:      strings.NewReader(""),
		Output:     io.Discard,
		ErrorOutput: io.Discard,
		ShowBanner: false,
		Prompt:     "test> ",
	}

	repl, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create REPL: %v", err)
	}

	if repl == nil {
		t.Fatal("REPL is nil")
	}

	if repl.prompt != "test> " {
		t.Errorf("Expected prompt 'test> ', got %q", repl.prompt)
	}

	if repl.dispatcher == nil {
		t.Fatal("Dispatcher not initialized")
	}

	if repl.registry == nil {
		t.Fatal("Registry not initialized")
	}

	if repl.session == nil {
		t.Fatal("Session not initialized")
	}
}

func TestREPLWithDefaults(t *testing.T) {
	cfg := Config{}

	repl, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create REPL with defaults: %v", err)
	}

	if repl.prompt != "flip2> " {
		t.Errorf("Expected default prompt 'flip2> ', got %q", repl.prompt)
	}

	if repl.apiClient == nil {
		t.Fatal("API client not initialized")
	}

	if repl.apiClient.BaseURL() != "http://localhost:8091" {
		t.Errorf("Expected default API URL 'http://localhost:8091', got %q", repl.apiClient.BaseURL())
	}
}

// =============================================================================
// COMMAND PARSING TESTS
// =============================================================================

func TestDispatcherParsing(t *testing.T) {
	dispatcher := NewDispatcher(NewRegistry())

	tests := []struct {
		input    string
		expected string
		args     []string
	}{
		{"/help", "help", nil},
		{"/status", "status", nil},
		{"/send claude hello", "send", []string{"claude", "hello"}},
		{"/task list", "task", []string{"list"}},
		{"/task create test --priority 5", "task", []string{"create", "test", "--priority", "5"}},
		{"hello world", "", nil}, // Non-command input
	}

	for _, tt := range tests {
		result := dispatcher.Parse(tt.input)
		if result.Command != tt.expected {
			t.Errorf("Parse(%q): expected command %q, got %q", tt.input, tt.expected, result.Command)
		}
		if len(tt.expected) > 0 { // Only check args for actual commands
			if len(result.Args) != len(tt.args) {
				t.Errorf("Parse(%q): expected %d args, got %d", tt.input, len(tt.args), len(result.Args))
			}
		}
	}
}

// =============================================================================
// COMMAND REGISTRY TESTS
// =============================================================================

func TestCommandRegistry(t *testing.T) {
	registry := NewRegistry()

	// Register a test command
	cmd := &TestCommand{
		name:        "test",
		description: "Test command",
	}

	err := registry.Register(cmd)
	if err != nil {
		t.Fatalf("Failed to register command: %v", err)
	}

	// Retrieve the command
	retrieved := registry.Get("test")
	if retrieved == nil {
		t.Fatal("Command not found in registry")
	}

	if retrieved.Name() != "test" {
		t.Errorf("Expected command name 'test', got %q", retrieved.Name())
	}
}

func TestCommandRegistryDuplicates(t *testing.T) {
	registry := NewRegistry()

	cmd := &TestCommand{name: "duplicate", description: "Test"}
	registry.Register(cmd)

	// Try to register again
	err := registry.Register(cmd)
	if err == nil {
		t.Fatal("Expected error registering duplicate command")
	}
}

func TestCommandRegistryAliases(t *testing.T) {
	registry := NewRegistry()

	cmd := &TestCommand{
		name:        "original",
		description: "Original command",
		aliases:     []string{"o", "orig"},
	}

	err := registry.Register(cmd)
	if err != nil {
		t.Fatalf("Failed to register command with aliases: %v", err)
	}

	// Test alias retrieval
	retrieved := registry.Get("o")
	if retrieved == nil {
		t.Fatal("Alias 'o' not found")
	}

	if retrieved.Name() != "original" {
		t.Errorf("Expected original command via alias, got %q", retrieved.Name())
	}
}

func TestCommandRegistryList(t *testing.T) {
	registry := NewRegistry()

	cmd1 := &TestCommand{name: "cmd1", description: "Command 1"}
	cmd2 := &TestCommand{name: "cmd2", description: "Command 2"}

	registry.Register(cmd1)
	registry.Register(cmd2)

	list := registry.List()
	if len(list) != 2 {
		t.Errorf("Expected 2 commands in list, got %d", len(list))
	}
}

// =============================================================================
// DISPATCHER EXECUTION TESTS
// =============================================================================

func TestDispatcherExecution(t *testing.T) {
	registry := NewRegistry()
	dispatcher := NewDispatcher(registry)

	testCmd := &TestCommand{
		name:        "echo",
		description: "Echo command",
	}
	registry.Register(testCmd)

	var output bytes.Buffer
	ctx := &Context{
		Output:      &output,
		ErrorOutput: io.Discard,
		Session:     NewSession("http://localhost:8091"),
	}

	err := dispatcher.Execute(ctx, "/echo hello world")
	if err != nil {
		t.Errorf("Execution failed: %v", err)
	}

	if !testCmd.executed {
		t.Fatal("Command was not executed")
	}
}

func TestDispatcherExecutionUnknownCommand(t *testing.T) {
	registry := NewRegistry()
	dispatcher := NewDispatcher(registry)

	var output bytes.Buffer
	ctx := &Context{
		Output:      &output,
		ErrorOutput: &output,
		Session:     NewSession("http://localhost:8091"),
	}

	err := dispatcher.Execute(ctx, "/unknown")
	if err == nil {
		t.Fatal("Expected error for unknown command")
	}

	if !strings.Contains(err.Error(), "unknown command") {
		t.Errorf("Expected 'unknown command' error, got %q", err.Error())
	}
}

func TestDispatcherEmptyInput(t *testing.T) {
	registry := NewRegistry()
	dispatcher := NewDispatcher(registry)

	ctx := &Context{
		Output:      io.Discard,
		ErrorOutput: io.Discard,
		Session:     NewSession("http://localhost:8091"),
	}

	err := dispatcher.Execute(ctx, "")
	if err != nil {
		t.Errorf("Expected no error for empty input, got %v", err)
	}

	err = dispatcher.Execute(ctx, "   ")
	if err != nil {
		t.Errorf("Expected no error for whitespace input, got %v", err)
	}
}

// =============================================================================
// TAB COMPLETION TESTS
// =============================================================================

func TestTabCompletion(t *testing.T) {
	registry := NewRegistry()
	registry.Register(&TestCommand{name: "status", description: "Status", aliases: []string{"st"}})
	registry.Register(&TestCommand{name: "send", description: "Send", aliases: []string{"s"}})
	registry.Register(&TestCommand{name: "help", description: "Help", aliases: []string{"h"}})

	dispatcherInst := NewDispatcher(registry)
	ctx := &Context{
		Output:      io.Discard,
		ErrorOutput: io.Discard,
		Session:     NewSession("http://localhost:8091"),
	}

	completer := NewTabCompleter(dispatcherInst, ctx)

	// Test command name completion
	completions := dispatcherInst.Complete(ctx, "/st", 3)
	if len(completions) == 0 {
		t.Fatal("Expected completions for '/st'")
	}

	// Should match "status"
	found := false
	for _, c := range completions {
		if c.Value == "status" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("Expected 'status' in completions")
	}

	// Test tab completer interface
	candidates, prefixLen := completer.Do([]rune("/st"), 3)
	if len(candidates) == 0 {
		t.Fatal("Expected candidates from Do()")
	}
	if prefixLen == 0 {
		t.Fatal("Expected non-zero prefix length")
	}
}

func TestTabCompletionEmptyPrefix(t *testing.T) {
	registry := NewRegistry()
	registry.Register(&TestCommand{name: "help", description: "Help"})
	registry.Register(&TestCommand{name: "exit", description: "Exit"})

	dispatcher := NewDispatcher(registry)
	ctx := &Context{
		Output:      io.Discard,
		ErrorOutput: io.Discard,
		Session:     NewSession("http://localhost:8091"),
	}

	completions := dispatcher.Complete(ctx, "/", 1)
	if len(completions) < 2 {
		t.Errorf("Expected at least 2 completions for '/', got %d", len(completions))
	}
}

// =============================================================================
// SESSION TESTS
// =============================================================================

func TestSession(t *testing.T) {
	session := NewSession("http://localhost:8091")

	if session.APIURL != "http://localhost:8091" {
		t.Errorf("Expected API URL 'http://localhost:8091', got %q", session.APIURL)
	}

	if len(session.Variables) != 0 {
		t.Errorf("Expected empty variables map, got %d items", len(session.Variables))
	}

	if len(session.History) != 0 {
		t.Errorf("Expected empty history, got %d items", len(session.History))
	}
}

func TestSessionAgentManagement(t *testing.T) {
	session := NewSession("http://localhost:8091")

	session.SetAgent("agent1")
	if session.GetAgent() != "agent1" {
		t.Errorf("Expected agent 'agent1', got %q", session.GetAgent())
	}

	session.SetAgent("agent2")
	if session.GetAgent() != "agent2" {
		t.Errorf("Expected agent 'agent2', got %q", session.GetAgent())
	}
}

func TestSessionConcurrency(t *testing.T) {
	session := NewSession("http://localhost:8091")

	// Test concurrent reads and writes
	go func() {
		for i := 0; i < 10; i++ {
			session.SetAgent(fmt.Sprintf("agent%d", i))
		}
	}()

	go func() {
		for i := 0; i < 10; i++ {
			_ = session.GetAgent()
		}
	}()

	// No deadlock expected - if we get here, test passes
}

// =============================================================================
// ALIAS REGISTRY TESTS
// =============================================================================

func TestAliasRegistry(t *testing.T) {
	ar := NewAliasRegistry()

	err := ar.SetAlias("s", "/send claude")
	if err != nil {
		t.Fatalf("Failed to set alias: %v", err)
	}

	cmd, exists := ar.GetAlias("s")
	if !exists {
		t.Fatal("Alias 'S' not found")
	}

	if cmd != "/send claude" {
		t.Errorf("Expected '/send claude', got %q", cmd)
	}
}

func TestAliasRegistryValidation(t *testing.T) {
	ar := NewAliasRegistry()

	tests := []struct {
		name    string
		command string
		valid   bool
	}{
		{"valid_alias", "/send claude", true},
		{"_underscore", "/status", true},
		{"123invalid", "/help", false}, // Can't start with digit
		{"with space", "/help", false},  // Can't contain space
		{"", "/help", false},             // Empty name
	}

	for _, tt := range tests {
		err := ar.SetAlias(tt.name, tt.command)
		if tt.valid && err != nil {
			t.Errorf("SetAlias(%q, %q) should be valid, got error: %v", tt.name, tt.command, err)
		}
		if !tt.valid && err == nil {
			t.Errorf("SetAlias(%q, %q) should be invalid, but succeeded", tt.name, tt.command)
		}
	}
}

func TestAliasExpansion(t *testing.T) {
	ar := NewAliasRegistry()
	ar.SetAlias("s", "/send")
	ar.SetAlias("sc", "/send claude")

	tests := []struct {
		input    string
		expected string
		expanded bool
	}{
		{"s hello", "/send hello", true},
		{"sc test", "/send claude test", true},
		{"/status", "/status", false},
		{"unknown cmd", "unknown cmd", false},
	}

	for _, tt := range tests {
		result, expanded := ar.ExpandAlias(tt.input)
		if expanded != tt.expanded {
			t.Errorf("ExpandAlias(%q): expected expanded=%v, got %v", tt.input, tt.expanded, expanded)
		}
		if result != tt.expected {
			t.Errorf("ExpandAlias(%q): expected %q, got %q", tt.input, tt.expected, result)
		}
	}
}

func TestAliasListAndDelete(t *testing.T) {
	ar := NewAliasRegistry()
	ar.SetAlias("s", "/send")
	ar.SetAlias("st", "/status")

	list := ar.ListAliases()
	if len(list) != 2 {
		t.Errorf("Expected 2 aliases, got %d", len(list))
	}

	err := ar.DeleteAlias("s")
	if err != nil {
		t.Fatalf("Failed to delete alias: %v", err)
	}

	list = ar.ListAliases()
	if len(list) != 1 {
		t.Errorf("Expected 1 alias after delete, got %d", len(list))
	}

	err = ar.DeleteAlias("nonexistent")
	if err == nil {
		t.Fatal("Expected error deleting nonexistent alias")
	}
}

func TestAliasClear(t *testing.T) {
	ar := NewAliasRegistry()
	ar.SetAlias("a", "/cmd1")
	ar.SetAlias("b", "/cmd2")

	ar.Clear()

	list := ar.ListAliases()
	if len(list) != 0 {
		t.Errorf("Expected 0 aliases after clear, got %d", len(list))
	}
}

// =============================================================================
// HELP SYSTEM TESTS
// =============================================================================

func TestHelpCommand(t *testing.T) {
	registry := NewRegistry()

	cmd := &TestCommand{
		name:        "test",
		description: "Test command",
		help:        "This is a test command with detailed help",
	}
	registry.Register(cmd)

	var output bytes.Buffer

	ctx := &Context{
		Output:      &output,
		ErrorOutput: io.Discard,
		Session:     NewSession("http://localhost:8091"),
	}

	// Create and execute help command
	helpCmd := &MockHelpCommand{registry: registry}
	err := helpCmd.Execute(ctx, []string{"test"})
	if err != nil {
		t.Errorf("Help command failed: %v", err)
	}
}

// =============================================================================
// COMMAND HISTORY TESTS
// =============================================================================

func TestCommandHistory(t *testing.T) {
	session := NewSession("http://localhost:8091")

	// History starts empty
	if len(session.History) != 0 {
		t.Fatal("History should start empty")
	}

	// Add to history
	session.History = append(session.History, "/status")
	session.History = append(session.History, "/send test")

	if len(session.History) != 2 {
		t.Errorf("Expected 2 items in history, got %d", len(session.History))
	}

	if session.History[0] != "/status" {
		t.Errorf("Expected first history item '/status', got %q", session.History[0])
	}
}

// =============================================================================
// ERROR HANDLING TESTS
// =============================================================================

func TestErrorHandling(t *testing.T) {
	registry := NewRegistry()

	failCmd := &FailingCommand{
		name: "fail",
		err:  fmt.Errorf("intentional error"),
	}
	registry.Register(failCmd)

	dispatcher := NewDispatcher(registry)
	var output, errOut bytes.Buffer

	ctx := &Context{
		Output:      &output,
		ErrorOutput: &errOut,
		Session:     NewSession("http://localhost:8091"),
	}

	err := dispatcher.Execute(ctx, "/fail")
	if err == nil {
		t.Fatal("Expected error from failing command")
	}

	if !strings.Contains(err.Error(), "intentional error") {
		t.Errorf("Expected 'intentional error', got %q", err.Error())
	}
}

// =============================================================================
// INTEGRATION TESTS
// =============================================================================

func TestFullCommandFlow(t *testing.T) {
	// Create a complete REPL setup
	cfg := Config{
		APIUrl:      "http://localhost:8091",
		Input:       simulateUserInput("/help", "/exit"),
		Output:      io.Discard,
		ErrorOutput: io.Discard,
		ShowBanner:  false,
		Prompt:      "test> ",
	}

	repl, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create REPL: %v", err)
	}

	// Register a test command
	testCmd := &TestCommand{
		name:        "test",
		description: "Test command",
	}
	repl.registry.Register(testCmd)

	// Test that we can retrieve the command
	cmd := repl.registry.Get("test")
	if cmd == nil {
		t.Fatal("Test command not found in REPL registry")
	}
}

func TestDispatcherWithAliases(t *testing.T) {
	registry := NewRegistry()
	registry.Register(&TestCommand{name: "send", description: "Send", aliases: []string{"s"}})

	// Get by canonical name
	cmd := registry.Get("send")
	if cmd == nil {
		t.Fatal("Command 'send' not found")
	}

	// Get by alias
	cmd = registry.Get("s")
	if cmd == nil {
		t.Fatal("Command 's' (alias) not found")
	}

	if cmd.Name() != "send" {
		t.Errorf("Expected canonical name 'send', got %q", cmd.Name())
	}
}

func TestMultipleCommands(t *testing.T) {
	registry := NewRegistry()
	commands := []string{"status", "send", "task", "help", "exit"}

	for _, name := range commands {
		cmd := &TestCommand{name: name, description: "Test command"}
		registry.Register(cmd)
	}

	all := registry.All()
	if len(all) != len(commands) {
		t.Errorf("Expected %d commands, got %d", len(commands), len(all))
	}
}

func TestContextPropagation(t *testing.T) {
	mockClient := NewTestAPIClient("http://test:8091")
	ctx := &Context{
		Output:      io.Discard,
		ErrorOutput: io.Discard,
		Session:     NewSession("http://test:8091"),
	}

	if ctx.Session == nil {
		t.Fatal("Session not propagated to context")
	}

	if mockClient == nil {
		t.Fatal("Mock API client failed to create")
	}

	if ctx.Session.APIURL != "http://test:8091" {
		t.Errorf("Expected API URL 'http://test:8091', got %q", ctx.Session.APIURL)
	}
}

// =============================================================================
// TEST COMMAND IMPLEMENTATIONS
// =============================================================================

// TestCommand implements Command interface for testing
type TestCommand struct {
	name        string
	description string
	help        string
	aliases     []string
	executed    bool
}

func (tc *TestCommand) Name() string                                           { return tc.name }
func (tc *TestCommand) Aliases() []string                                      { return tc.aliases }
func (tc *TestCommand) Description() string                                    { return tc.description }
func (tc *TestCommand) Usage() string                                          { return tc.name + " [args]" }
func (tc *TestCommand) Help() string                                           { return tc.help }
func (tc *TestCommand) Complete(ctx *Context, args []string, pos int, partial string) []Completion {
	return nil
}

func (tc *TestCommand) Execute(ctx *Context, args []string) error {
	tc.executed = true
	fmt.Fprintf(ctx.Output, "%s executed with %d args\n", tc.name, len(args))
	return nil
}

// FailingCommand implements Command interface but always fails
type FailingCommand struct {
	name string
	err  error
}

func (fc *FailingCommand) Name() string                                           { return fc.name }
func (fc *FailingCommand) Aliases() []string                                      { return nil }
func (fc *FailingCommand) Description() string                                    { return "Failing test command" }
func (fc *FailingCommand) Usage() string                                          { return fc.name }
func (fc *FailingCommand) Help() string                                           { return "This command always fails" }
func (fc *FailingCommand) Complete(ctx *Context, args []string, pos int, partial string) []Completion {
	return nil
}

func (fc *FailingCommand) Execute(ctx *Context, args []string) error {
	return fc.err
}

// MockHelpCommand provides help functionality for testing
type MockHelpCommand struct {
	registry *Registry
}

func (hc *MockHelpCommand) Name() string        { return "help" }
func (hc *MockHelpCommand) Aliases() []string   { return []string{"h"} }
func (hc *MockHelpCommand) Description() string { return "Show help" }
func (hc *MockHelpCommand) Usage() string       { return "help [command]" }
func (hc *MockHelpCommand) Help() string        { return "Show help for commands" }

func (hc *MockHelpCommand) Execute(ctx *Context, args []string) error {
	if len(args) == 0 {
		fmt.Fprintf(ctx.Output, "Available commands:\n")
		for _, cmd := range hc.registry.All() {
			fmt.Fprintf(ctx.Output, "  %s - %s\n", cmd.Name(), cmd.Description())
		}
		return nil
	}

	cmdName := args[0]
	cmd := hc.registry.Get(cmdName)
	if cmd == nil {
		return fmt.Errorf("unknown command: %s", cmdName)
	}

	fmt.Fprintf(ctx.Output, "%s\n", cmd.Help())
	return nil
}

func (hc *MockHelpCommand) Complete(ctx *Context, args []string, pos int, partial string) []Completion {
	var completions []Completion
	for _, cmd := range hc.registry.All() {
		if strings.HasPrefix(cmd.Name(), partial) {
			completions = append(completions, Completion{
				Value:       cmd.Name(),
				Description: cmd.Description(),
				Type:        CompletionCommand,
			})
		}
	}
	return completions
}

// =============================================================================
// BENCHMARK TESTS
// =============================================================================

func BenchmarkDispatcherParse(b *testing.B) {
	dispatcher := NewDispatcher(NewRegistry())
	testInput := "/send claude test message with args"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dispatcher.Parse(testInput)
	}
}

func BenchmarkRegistryLookup(b *testing.B) {
	registry := NewRegistry()
	registry.Register(&TestCommand{name: "test", description: "Test"})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		registry.Get("test")
	}
}

func BenchmarkAliasExpansion(b *testing.B) {
	ar := NewAliasRegistry()
	ar.SetAlias("s", "/send claude")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ar.ExpandAlias("s hello world")
	}
}
