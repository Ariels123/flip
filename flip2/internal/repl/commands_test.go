package repl

import (
	"bytes"
	"testing"
)

// TestTaskCommandStructure verifies that TaskCommand implements the Command interface
func TestTaskCommandStructure(t *testing.T) {
	cmd := &TaskCommand{}

	// Test interface implementation
	if cmd.Name() != "task" {
		t.Errorf("Expected name 'task', got '%s'", cmd.Name())
	}

	aliases := cmd.Aliases()
	if len(aliases) != 2 || aliases[0] != "t" || aliases[1] != "tasks" {
		t.Errorf("Expected aliases ['t', 'tasks'], got %v", aliases)
	}

	if desc := cmd.Description(); desc == "" {
		t.Error("Description should not be empty")
	}

	if usage := cmd.Usage(); usage == "" {
		t.Error("Usage should not be empty")
	}

	if help := cmd.Help(); help == "" {
		t.Error("Help should not be empty")
	}
}

// TestTaskCommandList verifies that /task list executes without error
func TestTaskCommandList(t *testing.T) {
	cmd := &TaskCommand{}
	output := &bytes.Buffer{}
	errOutput := &bytes.Buffer{}

	ctx := &Context{
		Output:      output,
		ErrorOutput: errOutput,
		Session:     NewSession("http://localhost:8091"),
	}

	// Test list subcommand
	err := cmd.Execute(ctx, []string{})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
}

// TestTaskCommandCreate verifies that /task create works
func TestTaskCommandCreate(t *testing.T) {
	cmd := &TaskCommand{}
	output := &bytes.Buffer{}
	errOutput := &bytes.Buffer{}

	ctx := &Context{
		Output:      output,
		ErrorOutput: errOutput,
		Session:     NewSession("http://localhost:8091"),
	}

	// Test create subcommand
	err := cmd.Execute(ctx, []string{"create", "Test Task"})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	result := output.String()
	if result == "" {
		t.Error("Expected output from task creation")
	}
}

// TestRegistryTasks verifies that TaskCommand can be registered
func TestRegistryTasks(t *testing.T) {
	registry := NewRegistry()
	cmd := &TaskCommand{}

	if err := registry.Register(cmd); err != nil {
		t.Fatalf("Failed to register TaskCommand: %v", err)
	}

	// Test lookup
	retrieved := registry.Get("task")
	if retrieved == nil {
		t.Error("Failed to retrieve TaskCommand by name")
	}

	retrieved = registry.Get("t")
	if retrieved == nil {
		t.Error("Failed to retrieve TaskCommand by alias 't'")
	}

	retrieved = registry.Get("tasks")
	if retrieved == nil {
		t.Error("Failed to retrieve TaskCommand by alias 'tasks'")
	}
}

// TestDispatcherTask verifies command dispatcher works with TaskCommand
func TestDispatcherTask(t *testing.T) {
	registry := NewRegistry()
	registry.MustRegister(&TaskCommand{})

	dispatcher := NewDispatcher(registry)

	// Parse /task list
	result := dispatcher.Parse("/task list")
	if result.Command != "task" {
		t.Errorf("Expected command 'task', got '%s'", result.Command)
	}

	if len(result.Args) != 1 || result.Args[0] != "list" {
		t.Errorf("Expected args ['list'], got %v", result.Args)
	}

	// Parse /t create "foo"
	result = dispatcher.Parse("/t create \"foo\"")
	if result.Command != "t" {
		t.Errorf("Expected command 't', got '%s'", result.Command)
	}
}
