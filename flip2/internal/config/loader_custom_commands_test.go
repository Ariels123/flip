package config

import (
	"bytes"
	"strings"
	"testing"

	"flip2/internal/repl"
)

// TestDynamicCommandName tests the Name method.
func TestDynamicCommandName(t *testing.T) {
	cmd := &DynamicCommand{
		name: "deploy",
	}

	if got := cmd.Name(); got != "deploy" {
		t.Errorf("Name() = %q, want %q", got, "deploy")
	}
}

// TestDynamicCommandDescription tests the Description method.
func TestDynamicCommandDescription(t *testing.T) {
	tests := []struct {
		name        string
		description string
		want        string
	}{
		{
			name:        "with description",
			description: "Deploy to staging",
			want:        "Deploy to staging",
		},
		{
			name:        "empty description",
			description: "",
			want:        "Custom command from FLIP2.md",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &DynamicCommand{
				description: tt.description,
			}

			if got := cmd.Description(); got != tt.want {
				t.Errorf("Description() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestDynamicCommandUsage tests the Usage method.
func TestDynamicCommandUsage(t *testing.T) {
	cmd := &DynamicCommand{
		name: "deploy",
	}

	expected := "/deploy [args...]"
	if got := cmd.Usage(); got != expected {
		t.Errorf("Usage() = %q, want %q", got, expected)
	}
}

// TestBuildTemplateVars tests template variable building.
func TestBuildTemplateVars(t *testing.T) {
	cmd := &DynamicCommand{
		name:      "test",
		variables: map[string]string{"env": "staging"},
	}

	ctx := &repl.Context{
		Session: &repl.Session{
			CurrentAgent: "claude",
		},
	}

	args := []string{"arg1", "arg2", "arg3"}
	vars := cmd.buildTemplateVars(ctx, args)

	tests := []struct {
		key      string
		expected interface{}
	}{
		{"arg0", "arg1"},
		{"arg1", "arg2"},
		{"arg2", "arg3"},
		{"numArgs", 3},
		{"args", "arg1 arg2 arg3"},
		{"env", "staging"},
		{"currentAgent", "claude"},
	}

	for _, tt := range tests {
		if got, ok := vars[tt.key]; !ok {
			t.Errorf("Variable %q not found", tt.key)
		} else if got != tt.expected {
			t.Errorf("Variable %q = %v, want %v", tt.key, got, tt.expected)
		}
	}
}

// TestRenderTemplate tests template rendering with variable substitution.
func TestRenderTemplate(t *testing.T) {
	tests := []struct {
		name     string
		template string
		vars     map[string]interface{}
		want     string
		wantErr  bool
	}{
		{
			name:     "simple variable",
			template: "echo {{.msg}}",
			vars:     map[string]interface{}{"msg": "hello"},
			want:     "echo hello",
			wantErr:  false,
		},
		{
			name:     "multiple variables",
			template: "{{.cmd}} {{.env}}",
			vars:     map[string]interface{}{"cmd": "deploy", "env": "staging"},
			want:     "deploy staging",
			wantErr:  false,
		},
		{
			name:     "no variables",
			template: "echo hello world",
			vars:     map[string]interface{}{},
			want:     "echo hello world",
			wantErr:  false,
		},
		{
			name:     "missing variable",
			template: "{{.missing}}",
			vars:     map[string]interface{}{},
			want:     "",
			wantErr:  false, // missingkey=default returns empty
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &DynamicCommand{name: "test"}

			got, err := cmd.renderTemplate(tt.template, tt.vars)
			if (err != nil) != tt.wantErr {
				t.Errorf("renderTemplate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if got != tt.want {
				t.Errorf("renderTemplate() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestPrepareCustomCommands tests the PrepareCustomCommands method.
func TestPrepareCustomCommands(t *testing.T) {
	merged := &MergedConfig{
		Commands: []Command{
			{
				Name:            "/deploy",
				Description:     "Deploy to staging",
				Handler:         "./scripts/deploy.sh staging",
				RequiresApproval: true,
				AllowedRoles:    []string{"devops"},
			},
			{
				Name:            "/status",
				Description:     "Show system status",
				Handler:         "echo 'System OK'",
				RequiresApproval: false,
			},
		},
	}

	if err := merged.PrepareCustomCommands(); err != nil {
		t.Fatalf("PrepareCustomCommands() error = %v", err)
	}

	if len(merged.CustomCommands) != 2 {
		t.Errorf("PrepareCustomCommands() prepared %d commands, want 2", len(merged.CustomCommands))
	}

	// Check first command
	if merged.CustomCommands[0].Name != "deploy" {
		t.Errorf("Command 0 name = %q, want %q", merged.CustomCommands[0].Name, "deploy")
	}
	if merged.CustomCommands[0].Description != "Deploy to staging" {
		t.Errorf("Command 0 description = %q, want %q", merged.CustomCommands[0].Description, "Deploy to staging")
	}
	if merged.CustomCommands[0].RequiresApproval != true {
		t.Errorf("Command 0 requiresApproval = %v, want true", merged.CustomCommands[0].RequiresApproval)
	}

	// Check second command
	if merged.CustomCommands[1].Name != "status" {
		t.Errorf("Command 1 name = %q, want %q", merged.CustomCommands[1].Name, "status")
	}
}

// TestRegisterCustomCommands tests registering custom commands with a registry.
func TestRegisterCustomCommands(t *testing.T) {
	registry := repl.NewRegistry()

	merged := &MergedConfig{
		Commands: []Command{
			{
				Name:        "/test1",
				Description: "Test command 1",
				Handler:     "echo test1",
			},
			{
				Name:        "/test2",
				Description: "Test command 2",
				Handler:     "echo test2",
			},
		},
	}

	if err := merged.PrepareCustomCommands(); err != nil {
		t.Fatalf("PrepareCustomCommands() error = %v", err)
	}

	if err := merged.RegisterCustomCommands(registry); err != nil {
		t.Fatalf("RegisterCustomCommands() error = %v", err)
	}

	// Verify commands are registered
	cmd1 := registry.Get("test1")
	if cmd1 == nil {
		t.Errorf("Command 'test1' not found in registry")
	}

	cmd2 := registry.Get("test2")
	if cmd2 == nil {
		t.Errorf("Command 'test2' not found in registry")
	}
}

// TestDynamicCommandExecute tests command execution with output capture.
func TestDynamicCommandExecute(t *testing.T) {
	cmd := &DynamicCommand{
		name:        "test",
		description: "Test command",
		script:      "echo 'Hello from test'",
	}

	output := &bytes.Buffer{}
	ctx := &repl.Context{
		Output:      output,
		Session:     &repl.Session{},
	}

	err := cmd.Execute(ctx, []string{})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	result := output.String()
	expected := "Hello from test\n"
	if result != expected {
		t.Errorf("Execute() output = %q, want %q", result, expected)
	}
}

// TestDynamicCommandExecuteWithTemplates tests execution with template substitution.
func TestDynamicCommandExecuteWithTemplates(t *testing.T) {
	cmd := &DynamicCommand{
		name:        "test",
		description: "Test with templates",
		script:      "echo 'Arg is {{.arg0}}'",
	}

	output := &bytes.Buffer{}
	ctx := &repl.Context{
		Output:      output,
		Session:     &repl.Session{},
	}

	err := cmd.Execute(ctx, []string{"myarg"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	result := output.String()
	if !strings.Contains(result, "myarg") {
		t.Errorf("Execute() output = %q, doesn't contain %q", result, "myarg")
	}
}

// TestDynamicCommandAliases tests the Aliases method.
func TestDynamicCommandAliases(t *testing.T) {
	cmd := &DynamicCommand{
		name: "deploy",
	}

	aliases := cmd.Aliases()
	if aliases != nil && len(aliases) > 0 {
		t.Errorf("Aliases() = %v, want empty or nil", aliases)
	}
}

// TestDynamicCommandComplete tests tab completion.
func TestDynamicCommandComplete(t *testing.T) {
	cmd := &DynamicCommand{
		name: "deploy",
	}

	ctx := &repl.Context{
		Session: &repl.Session{},
	}

	completions := cmd.Complete(ctx, []string{}, 0, "")
	if completions != nil && len(completions) > 0 {
		t.Errorf("Complete() = %v, want empty or nil (not implemented)", completions)
	}
}

// TestCreateCommandHandler tests handler factory.
func TestCreateCommandHandler(t *testing.T) {
	customCmd := CustomCommand{
		Name:        "deploy",
		Description: "Deploy to staging",
		Script:      "./scripts/deploy.sh staging",
	}

	handler, err := createCommandHandler(customCmd)
	if err != nil {
		t.Fatalf("createCommandHandler() error = %v", err)
	}

	if handler.Name() != "deploy" {
		t.Errorf("Handler Name() = %q, want %q", handler.Name(), "deploy")
	}

	if handler.Description() != "Deploy to staging" {
		t.Errorf("Handler Description() = %q, want %q", handler.Description(), "Deploy to staging")
	}
}

// TestEmptyCustomCommands tests handling of empty custom commands.
func TestEmptyCustomCommands(t *testing.T) {
	registry := repl.NewRegistry()

	merged := &MergedConfig{
		Commands: []Command{},
	}

	if err := merged.PrepareCustomCommands(); err != nil {
		t.Fatalf("PrepareCustomCommands() error = %v", err)
	}

	// Should not error on empty commands
	if err := merged.RegisterCustomCommands(registry); err != nil {
		t.Fatalf("RegisterCustomCommands() error = %v", err)
	}
}

// TestCommandHelp tests the Help method formatting.
func TestCommandHelp(t *testing.T) {
	cmd := &DynamicCommand{
		name:             "deploy",
		description:      "Deploy to staging",
		script:           "./scripts/deploy.sh {{.env}}",
		requiresApproval: true,
		allowedRoles:     []string{"devops", "release"},
	}

	help := cmd.Help()

	tests := []struct {
		name     string
		substring string
	}{
		{"contains name", "deploy"},
		{"contains description", "Deploy to staging"},
		{"contains script", "./scripts/deploy.sh"},
		{"contains approval notice", "requires approval"},
		{"contains roles", "devops"},
	}

	for _, tt := range tests {
		if !strings.Contains(help, tt.substring) {
			t.Errorf("Help() output doesn't contain %q: %s", tt.substring, help)
		}
	}
}

// TestDuplicateCommandRegistration tests error on duplicate registration.
func TestDuplicateCommandRegistration(t *testing.T) {
	registry := repl.NewRegistry()

	merged := &MergedConfig{
		Commands: []Command{
			{
				Name:        "/test",
				Description: "Test command",
				Handler:     "echo test",
			},
		},
	}

	if err := merged.PrepareCustomCommands(); err != nil {
		t.Fatalf("PrepareCustomCommands() error = %v", err)
	}

	if err := merged.RegisterCustomCommands(registry); err != nil {
		t.Fatalf("First RegisterCustomCommands() error = %v", err)
	}

	// Try to register again - should fail
	if err := merged.RegisterCustomCommands(registry); err == nil {
		t.Errorf("Second RegisterCustomCommands() should fail but didn't")
	}
}
