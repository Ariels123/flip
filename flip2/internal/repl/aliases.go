package repl

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// AliasRegistry manages user-defined command aliases.
// Aliases allow users to create shortcuts for frequently used commands.
// For example: alias ll="/send claude" allows typing "ll message" instead of "/send claude message"
type AliasRegistry struct {
	mu      sync.RWMutex
	aliases map[string]string // alias -> full command
	dirty   bool              // whether aliases need to be saved
	path    string            // path to aliases file (~/.flip2_aliases.json)
}

// NewAliasRegistry creates a new alias registry.
// It automatically loads aliases from ~/.flip2_aliases.json if it exists.
func NewAliasRegistry() *AliasRegistry {
	ar := &AliasRegistry{
		aliases: make(map[string]string),
		path:    expandHome("~/.flip2_aliases.json"),
	}
	// Try to load existing aliases, but don't fail if file doesn't exist
	_ = ar.Load()
	return ar
}

// SetAlias sets an alias to expand to the given command.
// Returns an error if the alias name is invalid.
func (ar *AliasRegistry) SetAlias(name, command string) error {
	if err := validateAliasName(name); err != nil {
		return err
	}

	if strings.TrimSpace(command) == "" {
		return fmt.Errorf("alias command cannot be empty")
	}

	ar.mu.Lock()
	defer ar.mu.Unlock()

	ar.aliases[name] = strings.TrimSpace(command)
	ar.dirty = true
	return nil
}

// GetAlias retrieves an alias and whether it exists.
func (ar *AliasRegistry) GetAlias(name string) (command string, exists bool) {
	ar.mu.RLock()
	defer ar.mu.RUnlock()

	command, exists = ar.aliases[name]
	return
}

// DeleteAlias removes an alias.
// Returns an error if the alias doesn't exist.
func (ar *AliasRegistry) DeleteAlias(name string) error {
	ar.mu.Lock()
	defer ar.mu.Unlock()

	if _, exists := ar.aliases[name]; !exists {
		return fmt.Errorf("alias %q not found", name)
	}

	delete(ar.aliases, name)
	ar.dirty = true
	return nil
}

// ListAliases returns all defined aliases as a map.
func (ar *AliasRegistry) ListAliases() map[string]string {
	ar.mu.RLock()
	defer ar.mu.RUnlock()

	// Return a copy to prevent external modification
	result := make(map[string]string, len(ar.aliases))
	for k, v := range ar.aliases {
		result[k] = v
	}
	return result
}

// ExpandAlias expands an alias if it exists.
// Returns the expanded command and true if the alias was found.
// If not found, returns the original input and false.
func (ar *AliasRegistry) ExpandAlias(input string) (string, bool) {
	ar.mu.RLock()
	defer ar.mu.RUnlock()

	// Extract the first token (the potential alias name)
	tokens := strings.Fields(input)
	if len(tokens) == 0 {
		return input, false
	}

	aliasName := tokens[0]
	if command, exists := ar.aliases[aliasName]; exists {
		// Replace the alias with the expanded command
		// Preserve any arguments after the alias
		if len(tokens) > 1 {
			return command + " " + strings.Join(tokens[1:], " "), true
		}
		return command, true
	}

	return input, false
}

// Load loads aliases from ~/.flip2_aliases.json.
func (ar *AliasRegistry) Load() error {
	ar.mu.Lock()
	defer ar.mu.Unlock()

	data, err := ioutil.ReadFile(ar.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // File doesn't exist yet, that's OK
		}
		return fmt.Errorf("failed to read aliases file: %w", err)
	}

	var aliases map[string]string
	if err := json.Unmarshal(data, &aliases); err != nil {
		return fmt.Errorf("failed to parse aliases file: %w", err)
	}

	ar.aliases = aliases
	ar.dirty = false
	return nil
}

// Save persists aliases to ~/.flip2_aliases.json.
func (ar *AliasRegistry) Save() error {
	ar.mu.Lock()
	defer ar.mu.Unlock()

	if !ar.dirty {
		return nil // No changes to save
	}

	// Ensure directory exists
	dir := filepath.Dir(ar.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(ar.aliases, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal aliases: %w", err)
	}

	// Write to file
	if err := ioutil.WriteFile(ar.path, data, 0600); err != nil {
		return fmt.Errorf("failed to write aliases file: %w", err)
	}

	ar.dirty = false
	return nil
}

// Clear removes all aliases.
func (ar *AliasRegistry) Clear() {
	ar.mu.Lock()
	defer ar.mu.Unlock()

	ar.aliases = make(map[string]string)
	ar.dirty = true
}

// validateAliasName checks if an alias name is valid.
// Valid names must:
// - Not be empty
// - Not contain spaces
// - Start with a letter or underscore
// - Only contain alphanumeric characters and underscores
func validateAliasName(name string) error {
	if name == "" {
		return fmt.Errorf("alias name cannot be empty")
	}

	if strings.Contains(name, " ") {
		return fmt.Errorf("alias name cannot contain spaces")
	}

	// Check first character
	if first := rune(name[0]); !(isLetter(first) || first == '_') {
		return fmt.Errorf("alias name must start with a letter or underscore")
	}

	// Check remaining characters
	for _, r := range name[1:] {
		if !(isLetter(r) || isDigit(r) || r == '_') {
			return fmt.Errorf("alias name can only contain letters, digits, and underscores")
		}
	}

	return nil
}

// isLetter checks if a rune is a letter.
func isLetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

// isDigit checks if a rune is a digit.
func isDigit(r rune) bool {
	return r >= '0' && r <= '9'
}

// expandHome expands ~ to the user's home directory.
func expandHome(path string) string {
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path // Return as-is if we can't get home dir
		}
		return filepath.Join(home, path[1:])
	}
	return path
}

// AliasCommand is the /alias slash command for managing aliases.
type AliasCommand struct {
	aliasRegistry *AliasRegistry
}

// NewAliasCommand creates an AliasCommand.
func NewAliasCommand(ar *AliasRegistry) *AliasCommand {
	return &AliasCommand{
		aliasRegistry: ar,
	}
}

func (c *AliasCommand) Name() string        { return "alias" }
func (c *AliasCommand) Aliases() []string   { return []string{"a"} }
func (c *AliasCommand) Description() string { return "Manage command aliases" }
func (c *AliasCommand) Usage() string {
	return "/alias [<name> <command>] [--list] [--delete <name>] [--clear] [--save]"
}

func (c *AliasCommand) Help() string {
	return `Manage user-defined command aliases.

Aliases allow you to create shortcuts for frequently used commands.

Usage: /alias [options] [<name> <command>]

Subcommands:
  (no args)           Show all aliases
  <name> <command>    Create or update an alias
  --list              List all aliases
  --delete <name>     Delete an alias
  --clear             Delete all aliases
  --save              Save aliases to ~/.flip2_aliases.json

Examples:
  /alias s "/send"
  /alias s "/send claude"          Create alias: s → /send claude
  /alias --list                    Show all aliases
  /alias --delete s                Delete alias "s"
  /alias --clear                   Delete all aliases
  /alias --save                    Save to file

Aliases are automatically loaded from ~/.flip2_aliases.json on startup.
Use --save to persist changes to disk.

Alias names must:
  - Start with a letter or underscore
  - Contain only letters, digits, and underscores
  - Be unique
`
}

func (c *AliasCommand) Execute(ctx *Context, args []string) error {
	if len(args) == 0 {
		// Show all aliases
		return c.listAliases(ctx)
	}

	// Handle flags
	switch args[0] {
	case "--list":
		return c.listAliases(ctx)

	case "--delete":
		if len(args) < 2 {
			return fmt.Errorf("--delete requires an alias name")
		}
		return c.deleteAlias(ctx, args[1])

	case "--clear":
		c.aliasRegistry.Clear()
		fmt.Fprintf(ctx.Output, "All aliases cleared.\n")
		return nil

	case "--save":
		if err := c.aliasRegistry.Save(); err != nil {
			return fmt.Errorf("failed to save aliases: %w", err)
		}
		fmt.Fprintf(ctx.Output, "Aliases saved to ~/.flip2_aliases.json\n")
		return nil

	default:
		// Create/update alias: /alias <name> <command>
		if len(args) < 2 {
			return fmt.Errorf("expected: /alias <name> <command>")
		}

		name := args[0]
		command := strings.Join(args[1:], " ")

		if err := c.aliasRegistry.SetAlias(name, command); err != nil {
			return err
		}

		fmt.Fprintf(ctx.Output, "Alias created: %s → %s\n", name, command)
		return nil
	}
}

func (c *AliasCommand) Complete(ctx *Context, args []string, pos int, partial string) []Completion {
	if pos == 0 {
		// First argument: flag or alias name
		flags := []Completion{
			{Value: "--list", Description: "List all aliases", Type: CompletionFlag},
			{Value: "--delete", Description: "Delete an alias", Type: CompletionFlag},
			{Value: "--clear", Description: "Delete all aliases", Type: CompletionFlag},
			{Value: "--save", Description: "Save aliases to file", Type: CompletionFlag},
		}

		// Filter flags by partial match
		var matches []Completion
		for _, f := range flags {
			if strings.HasPrefix(f.Value, partial) {
				matches = append(matches, f)
			}
		}

		// Also suggest existing alias names
		for name := range c.aliasRegistry.ListAliases() {
			if strings.HasPrefix(name, partial) {
				matches = append(matches, Completion{
					Value:       name,
					Description: "Existing alias",
					Type:        CompletionValue,
				})
			}
		}

		return matches
	}

	if pos == 1 && len(args) > 0 {
		// Second argument depends on first
		switch args[0] {
		case "--delete":
			// Complete alias names to delete
			var completions []Completion
			for name := range c.aliasRegistry.ListAliases() {
				if strings.HasPrefix(name, partial) {
					completions = append(completions, Completion{
						Value:       name,
						Description: "Alias to delete",
						Type:        CompletionValue,
					})
				}
			}
			return completions
		}
	}

	return nil
}

// listAliases displays all defined aliases.
func (c *AliasCommand) listAliases(ctx *Context) error {
	aliases := c.aliasRegistry.ListAliases()

	if len(aliases) == 0 {
		fmt.Fprintf(ctx.Output, "No aliases defined. Use '/alias <name> <command>' to create one.\n")
		return nil
	}

	fmt.Fprintf(ctx.Output, "Aliases:\n")
	for name, command := range aliases {
		fmt.Fprintf(ctx.Output, "  %-20s → %s\n", name, command)
	}

	return nil
}

// deleteAlias removes an alias.
func (c *AliasCommand) deleteAlias(ctx *Context, name string) error {
	if err := c.aliasRegistry.DeleteAlias(name); err != nil {
		return err
	}

	fmt.Fprintf(ctx.Output, "Alias %q deleted.\n", name)
	return nil
}
