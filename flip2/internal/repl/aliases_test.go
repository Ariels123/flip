package repl

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewAliasRegistry(t *testing.T) {
	ar := NewAliasRegistry()
	if ar == nil {
		t.Fatal("NewAliasRegistry returned nil")
	}
	if ar.aliases == nil {
		t.Fatal("aliases map is nil")
	}
}

func TestSetAlias(t *testing.T) {
	ar := NewAliasRegistry()

	// Valid alias
	err := ar.SetAlias("s", "/send")
	if err != nil {
		t.Fatalf("SetAlias failed: %v", err)
	}

	// Get the alias
	cmd, exists := ar.GetAlias("s")
	if !exists {
		t.Fatal("alias not found after setting")
	}
	if cmd != "/send" {
		t.Fatalf("expected /send, got %s", cmd)
	}
}

func TestSetAliasValidation(t *testing.T) {
	ar := NewAliasRegistry()

	tests := []struct {
		name    string
		alias   string
		command string
		wantErr bool
	}{
		{"valid", "s", "/send", false},
		{"valid_underscore", "_alias", "/cmd", false},
		{"valid_digits", "s1", "/send", false},
		{"empty_name", "", "/send", true},
		{"spaces_in_name", "my alias", "/send", true},
		{"empty_command", "s", "", true},
		{"digit_first", "1alias", "/cmd", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ar.SetAlias(tt.alias, tt.command)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetAlias() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExpandAlias(t *testing.T) {
	ar := NewAliasRegistry()
	ar.SetAlias("s", "/send claude")
	ar.SetAlias("t", "/task")

	tests := []struct {
		input    string
		expected string
		expanded bool
	}{
		{"/s hello", "/send claude hello", true},
		{"/s", "/send claude", true},
		{"/t item", "/task item", true},
		{"/unknown", "/unknown", false},
		{"/status", "/status", false},
	}

	for _, tt := range tests {
		expanded, wasExpanded := ar.ExpandAlias(tt.input)
		if wasExpanded != tt.expanded {
			t.Errorf("ExpandAlias(%q) expanded = %v, want %v", tt.input, wasExpanded, tt.expanded)
		}
		if expanded != tt.expected {
			t.Errorf("ExpandAlias(%q) = %q, want %q", tt.input, expanded, tt.expected)
		}
	}
}

func TestDeleteAlias(t *testing.T) {
	ar := NewAliasRegistry()
	ar.SetAlias("s", "/send")

	// Delete existing alias
	err := ar.DeleteAlias("s")
	if err != nil {
		t.Fatalf("DeleteAlias failed: %v", err)
	}

	// Verify it's gone
	_, exists := ar.GetAlias("s")
	if exists {
		t.Fatal("alias still exists after deletion")
	}

	// Try to delete non-existent alias
	err = ar.DeleteAlias("s")
	if err == nil {
		t.Fatal("expected error when deleting non-existent alias")
	}
}

func TestListAliases(t *testing.T) {
	ar := NewAliasRegistry()
	ar.SetAlias("s", "/send")
	ar.SetAlias("t", "/task")

	list := ar.ListAliases()
	if len(list) != 2 {
		t.Fatalf("expected 2 aliases, got %d", len(list))
	}

	if list["s"] != "/send" {
		t.Errorf("expected s -> /send, got %s", list["s"])
	}
	if list["t"] != "/task" {
		t.Errorf("expected t -> /task, got %s", list["t"])
	}

	// Ensure returned map is a copy
	list["x"] = "/extra"
	if len(ar.ListAliases()) != 2 {
		t.Fatal("external modification affected alias registry")
	}
}

func TestClear(t *testing.T) {
	ar := NewAliasRegistry()
	ar.SetAlias("s", "/send")
	ar.SetAlias("t", "/task")

	ar.Clear()
	list := ar.ListAliases()
	if len(list) != 0 {
		t.Fatalf("expected 0 aliases after clear, got %d", len(list))
	}
}

func TestSaveAndLoad(t *testing.T) {
	// Use a temporary directory for testing
	tmpDir := t.TempDir()
	aliasPath := filepath.Join(tmpDir, ".flip2_aliases.json")

	// Create registry and set path
	ar := &AliasRegistry{
		aliases: make(map[string]string),
		path:    aliasPath,
	}

	ar.SetAlias("s", "/send claude")
	ar.SetAlias("t", "/task")

	// Save aliases
	err := ar.Save()
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(aliasPath); err != nil {
		t.Fatalf("aliases file not created: %v", err)
	}

	// Create new registry and load
	ar2 := &AliasRegistry{
		aliases: make(map[string]string),
		path:    aliasPath,
	}

	err = ar2.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Verify aliases were loaded
	list := ar2.ListAliases()
	if len(list) != 2 {
		t.Fatalf("expected 2 aliases after load, got %d", len(list))
	}

	if list["s"] != "/send claude" {
		t.Errorf("expected s -> /send claude, got %s", list["s"])
	}
	if list["t"] != "/task" {
		t.Errorf("expected t -> /task, got %s", list["t"])
	}
}

func TestValidateAliasName(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{"s", false},
		{"send", false},
		{"_private", false},
		{"s1", false},
		{"s_1", false},
		{"", true},
		{"my alias", true},
		{"1first", true},
		{"-dash", true},
		{"with.dot", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAliasName(tt.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateAliasName() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAliasCommand(t *testing.T) {
	ar := NewAliasRegistry()
	cmd := NewAliasCommand(ar)

	if cmd.Name() != "alias" {
		t.Errorf("expected name 'alias', got %s", cmd.Name())
	}

	if len(cmd.Aliases()) == 0 {
		t.Error("expected aliases to be non-empty")
	}

	desc := cmd.Description()
	if desc == "" {
		t.Error("expected description to be non-empty")
	}
}
