// Package config provides FLIP2.md configuration loading, inheritance, and custom command registration.
package config

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"flip2/internal/repl"
)

// ConfigLoader handles loading and merging configurations from multiple levels.
type ConfigLoader struct {
	globalPath    string // ~/.flip2/FLIP2.md
	projectPath   string // <project-root>/FLIP2.md
	directoryPath string // <working-dir>/FLIP2.md
}

// CustomCommand represents a command defined in FLIP2.md that should be dynamically registered.
type CustomCommand struct {
	// Name of the command (without leading slash)
	Name string

	// Description for help text
	Description string

	// Script or handler to execute
	Script string

	// Variables that can be interpolated in the script
	Variables map[string]string

	// Whether this command requires approval before execution
	RequiresApproval bool

	// Roles allowed to execute this command
	AllowedRoles []string
}

// MergedConfig represents the final merged configuration after inheritance is applied.
type MergedConfig struct {
	// Source information
	GlobalConfig    *ProjectConfig
	ProjectConfig   *ProjectConfig
	DirectoryConfig *ProjectConfig

	// Merged results (directory > project > global)
	Agents   []AgentRole
	Commands []Command
	Routes   []Route
	Context  ContextConfig

	// Parsed custom commands ready for registration
	CustomCommands []CustomCommand

	// Metadata about the merge
	MergeInfo struct {
		GlobalLoaded    bool
		ProjectLoaded   bool
		DirectoryLoaded bool
	}
}

// NewConfigLoader creates a new ConfigLoader for the given working directory.
// It automatically discovers the project root (by looking for go.mod) and sets up paths.
func NewConfigLoader(workingDir string) (*ConfigLoader, error) {
	loader := &ConfigLoader{}

	// Resolve global config path
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to determine home directory: %w", err)
	}
	globalDir := filepath.Join(homeDir, ".flip2")
	loader.globalPath = filepath.Join(globalDir, "FLIP2.md")

	// Find project root by traversing up from workingDir looking for go.mod
	projectRoot := findProjectRoot(workingDir)
	if projectRoot != "" {
		loader.projectPath = filepath.Join(projectRoot, "FLIP2.md")
	}

	// Set directory config path
	loader.directoryPath = filepath.Join(workingDir, "FLIP2.md")

	return loader, nil
}

// LoadConfigChain loads configurations from all three levels (global, project, directory)
// and merges them with the precedence: directory > project > global
func (cl *ConfigLoader) LoadConfigChain() (*MergedConfig, error) {
	merged := &MergedConfig{
		Agents:   []AgentRole{},
		Commands: []Command{},
		Routes:   []Route{},
		Context: ContextConfig{
			AutoLoadFiles: []ContextFile{},
		},
	}

	// Load global config
	globalConfig, err := cl.loadConfig(cl.globalPath)
	if err != nil {
		// Global config is optional, log but don't fail
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("error reading global config: %w", err)
		}
	} else {
		merged.GlobalConfig = globalConfig
		merged.MergeInfo.GlobalLoaded = true
	}

	// Load project config
	projectConfig, err := cl.loadConfig(cl.projectPath)
	if err != nil {
		// Project config is optional
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("error reading project config: %w", err)
		}
	} else {
		merged.ProjectConfig = projectConfig
		merged.MergeInfo.ProjectLoaded = true
	}

	// Load directory config
	directoryConfig, err := cl.loadConfig(cl.directoryPath)
	if err != nil {
		// Directory config is optional
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("error reading directory config: %w", err)
		}
	} else {
		merged.DirectoryConfig = directoryConfig
		merged.MergeInfo.DirectoryLoaded = true
	}

	// Perform the merge with proper precedence: directory > project > global
	merged.mergeConfigs()

	return merged, nil
}

// loadConfig loads and parses a single FLIP2.md file
func (cl *ConfigLoader) loadConfig(path string) (*ProjectConfig, error) {
	if path == "" {
		return nil, os.ErrNotExist
	}

	// Check if file exists
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}

	// Parse the file
	config, err := ParseFLIP2MD(path)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config at %s: %w", path, err)
	}

	return config, nil
}

// mergeConfigs merges configurations from all three levels with proper precedence.
// Precedence: directory (highest) > project > global (lowest)
//
// Merge rules:
// - Commands: later overrides earlier (same Name overwrites)
// - Agents: later overrides earlier (same Name overwrites)
// - Routes: later overrides earlier (same Name overwrites)
// - Context: accumulate all context files from all levels
func (m *MergedConfig) mergeConfigs() {
	// Build lookup maps for commands, agents, and routes
	// This allows later configs to override earlier ones
	commandMap := make(map[string]Command)
	agentMap := make(map[string]AgentRole)
	routeMap := make(map[string]Route)
	contextFiles := make(map[string]ContextFile) // key is path to prevent duplicates

	// Phase 1: Add global configs (lowest precedence)
	if m.GlobalConfig != nil {
		for _, agent := range m.GlobalConfig.Agents {
			agentMap[agent.Name] = agent
		}
		for _, cmd := range m.GlobalConfig.Commands {
			commandMap[cmd.Name] = cmd
		}
		for _, route := range m.GlobalConfig.Routes {
			routeMap[route.Name] = route
		}
		for _, file := range m.GlobalConfig.Context.AutoLoadFiles {
			contextFiles[file.Path] = file
		}
	}

	// Phase 2: Add project configs (medium precedence, overrides global)
	if m.ProjectConfig != nil {
		for _, agent := range m.ProjectConfig.Agents {
			agentMap[agent.Name] = agent
		}
		for _, cmd := range m.ProjectConfig.Commands {
			commandMap[cmd.Name] = cmd
		}
		for _, route := range m.ProjectConfig.Routes {
			routeMap[route.Name] = route
		}
		for _, file := range m.ProjectConfig.Context.AutoLoadFiles {
			contextFiles[file.Path] = file
		}
	}

	// Phase 3: Add directory configs (highest precedence, overrides everything)
	if m.DirectoryConfig != nil {
		for _, agent := range m.DirectoryConfig.Agents {
			agentMap[agent.Name] = agent
		}
		for _, cmd := range m.DirectoryConfig.Commands {
			commandMap[cmd.Name] = cmd
		}
		for _, route := range m.DirectoryConfig.Routes {
			routeMap[route.Name] = route
		}
		for _, file := range m.DirectoryConfig.Context.AutoLoadFiles {
			contextFiles[file.Path] = file
		}
	}

	// Convert maps back to slices
	for _, agent := range agentMap {
		m.Agents = append(m.Agents, agent)
	}
	for _, cmd := range commandMap {
		m.Commands = append(m.Commands, cmd)
	}
	for _, route := range routeMap {
		m.Routes = append(m.Routes, route)
	}
	for _, file := range contextFiles {
		m.Context.AutoLoadFiles = append(m.Context.AutoLoadFiles, file)
	}
}

// GetAgent returns an agent by name from the merged configuration
func (m *MergedConfig) GetAgent(name string) *AgentRole {
	for _, agent := range m.Agents {
		if agent.Name == name {
			return &agent
		}
	}
	return nil
}

// GetCommand returns a command by name from the merged configuration
func (m *MergedConfig) GetCommand(name string) *Command {
	for _, cmd := range m.Commands {
		if cmd.Name == name {
			return &cmd
		}
	}
	return nil
}

// GetRoute returns a route by name from the merged configuration
func (m *MergedConfig) GetRoute(name string) *Route {
	for _, route := range m.Routes {
		if route.Name == name {
			return &route
		}
	}
	return nil
}

// findProjectRoot searches up from the given directory looking for a go.mod file
// Returns the directory containing go.mod, or empty string if not found
func findProjectRoot(startDir string) string {
	currentDir := startDir

	// Traverse up the directory tree looking for go.mod
	for {
		if currentDir == "/" || currentDir == "" {
			break
		}

		goModPath := filepath.Join(currentDir, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			return currentDir
		}

		// Move to parent directory
		parentDir := filepath.Dir(currentDir)
		if parentDir == currentDir {
			// Reached filesystem root
			break
		}
		currentDir = parentDir
	}

	return ""
}

// GetLoadPaths returns the paths that were attempted for loading configs
// Useful for debugging and logging
func (cl *ConfigLoader) GetLoadPaths() struct {
	Global    string
	Project   string
	Directory string
} {
	return struct {
		Global    string
		Project   string
		Directory string
	}{
		Global:    cl.globalPath,
		Project:   cl.projectPath,
		Directory: cl.directoryPath,
	}
}

// PrepareCustomCommands parses commands from the merged config and prepares them for registration.
// This method should be called after mergeConfigs to set up custom command objects.
func (m *MergedConfig) PrepareCustomCommands() error {
	m.CustomCommands = make([]CustomCommand, 0, len(m.Commands))

	for _, cmd := range m.Commands {
		// Remove leading slash from command name if present
		name := strings.TrimPrefix(cmd.Name, "/")

		customCmd := CustomCommand{
			Name:             name,
			Description:      cmd.Description,
			Script:           cmd.Handler,
			RequiresApproval: cmd.RequiresApproval,
			AllowedRoles:     cmd.AllowedRoles,
			Variables:        make(map[string]string),
		}

		m.CustomCommands = append(m.CustomCommands, customCmd)
	}

	return nil
}

// RegisterCustomCommands dynamically registers all custom commands from the merged configuration
// with the REPL command registry. Each command becomes callable from the REPL.
func (m *MergedConfig) RegisterCustomCommands(registry *repl.Registry) error {
	if len(m.CustomCommands) == 0 {
		return nil // No custom commands to register
	}

	for _, cmd := range m.CustomCommands {
		// Create a dynamic command handler for this custom command
		handler, err := createCommandHandler(cmd)
		if err != nil {
			return fmt.Errorf("failed to create handler for command %q: %w", cmd.Name, err)
		}

		// Register the command with the registry
		if err := registry.Register(handler); err != nil {
			return fmt.Errorf("failed to register command %q: %w", cmd.Name, err)
		}
	}

	return nil
}

// createCommandHandler creates a repl.Command implementation for a custom command.
// The handler wraps the script execution with template variable substitution.
func createCommandHandler(customCmd CustomCommand) (repl.Command, error) {
	return &DynamicCommand{
		name:             customCmd.Name,
		description:      customCmd.Description,
		script:           customCmd.Script,
		variables:        customCmd.Variables,
		requiresApproval: customCmd.RequiresApproval,
		allowedRoles:     customCmd.AllowedRoles,
	}, nil
}

// DynamicCommand is a repl.Command that executes a custom script with template support.
// It implements the Command interface to be compatible with the REPL registry.
type DynamicCommand struct {
	name             string
	description      string
	script           string
	variables        map[string]string
	requiresApproval bool
	allowedRoles     []string
}

// Name implements repl.Command.Name()
func (d *DynamicCommand) Name() string {
	return d.name
}

// Aliases implements repl.Command.Aliases()
// Dynamic commands have no aliases by default
func (d *DynamicCommand) Aliases() []string {
	return nil
}

// Description implements repl.Command.Description()
func (d *DynamicCommand) Description() string {
	if d.description != "" {
		return d.description
	}
	return "Custom command from FLIP2.md"
}

// Usage implements repl.Command.Usage()
func (d *DynamicCommand) Usage() string {
	return fmt.Sprintf("/%s [args...]", d.name)
}

// Help implements repl.Command.Help()
func (d *DynamicCommand) Help() string {
	var buf strings.Builder

	fmt.Fprintf(&buf, "Custom command: %s\n\n", d.name)
	fmt.Fprintf(&buf, "Usage: /%s [args...]\n\n", d.name)

	if d.description != "" {
		fmt.Fprintf(&buf, "Description:\n  %s\n\n", d.description)
	}

	fmt.Fprintf(&buf, "Script:\n  %s\n\n", d.script)

	if d.requiresApproval {
		fmt.Fprint(&buf, "Note: This command requires approval before execution.\n\n")
	}

	if len(d.allowedRoles) > 0 {
		fmt.Fprintf(&buf, "Allowed Roles: %s\n", strings.Join(d.allowedRoles, ", "))
	}

	return buf.String()
}

// Execute implements repl.Command.Execute()
// This method executes the custom command's script with template variable substitution.
func (d *DynamicCommand) Execute(ctx *repl.Context, args []string) error {
	// Build context for template rendering
	tmplVars := d.buildTemplateVars(ctx, args)

	// Parse and execute script template
	scriptOutput, err := d.executeScript(tmplVars)
	if err != nil {
		return fmt.Errorf("script execution failed: %w", err)
	}

	// Write output to context
	if ctx.Output != nil {
		_, err := io.WriteString(ctx.Output, scriptOutput)
		if err != nil {
			return fmt.Errorf("failed to write output: %w", err)
		}
	}

	return nil
}

// Complete implements repl.Command.Complete() for tab completion
func (d *DynamicCommand) Complete(ctx *repl.Context, args []string, pos int, partial string) []repl.Completion {
	// Dynamic commands don't provide completions by default
	return nil
}

// buildTemplateVars creates a map of variables available for template substitution.
// This includes command arguments, session state, and other context.
func (d *DynamicCommand) buildTemplateVars(ctx *repl.Context, args []string) map[string]interface{} {
	vars := make(map[string]interface{})

	// Add static variables from custom command definition
	for k, v := range d.variables {
		vars[k] = v
	}

	// Add arguments as indexed variables (arg0, arg1, etc.)
	for i, arg := range args {
		vars[fmt.Sprintf("arg%d", i)] = arg
	}

	// Add special variables
	vars["args"] = strings.Join(args, " ")
	vars["numArgs"] = len(args)

	// Add session context if available
	if ctx.Session != nil {
		vars["currentAgent"] = ctx.Session.GetAgent()
	}

	// Add environment variables
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			vars["env_"+parts[0]] = parts[1]
		}
	}

	return vars
}

// executeScript processes the script template and executes the resulting command.
// It supports three types of scripts:
// 1. Shell commands (e.g., "bash -c 'echo hello'")
// 2. File paths (e.g., "./scripts/deploy.sh staging")
// 3. Inline templates with variable substitution
func (d *DynamicCommand) executeScript(vars map[string]interface{}) (string, error) {
	// First, render the script as a template to substitute variables
	renderedScript, err := d.renderTemplate(d.script, vars)
	if err != nil {
		return "", fmt.Errorf("template rendering failed: %w", err)
	}

	// Check if script is a file path or shell command
	if strings.Contains(renderedScript, "/") && !strings.HasPrefix(renderedScript, "-") {
		// Likely a file path, try to execute it
		parts := strings.Fields(renderedScript)
		if len(parts) == 0 {
			return "", fmt.Errorf("empty script")
		}

		cmd := exec.Command(parts[0], parts[1:]...)
		return d.runCommand(cmd)
	}

	// Otherwise, execute as shell command
	cmd := exec.Command("sh", "-c", renderedScript)
	return d.runCommand(cmd)
}

// renderTemplate applies Go template rendering to the script string,
// allowing variable substitution using {{ .varName }} syntax.
func (d *DynamicCommand) renderTemplate(scriptTemplate string, vars map[string]interface{}) (string, error) {
	// Create template with lenient parsing to allow arbitrary text
	tmpl, err := template.New(d.name).
		Option("missingkey=zero").
		Parse(scriptTemplate)
	if err != nil {
		return "", fmt.Errorf("template parse error: %w", err)
	}

	// Execute template with variables
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, vars); err != nil {
		return "", fmt.Errorf("template execution error: %w", err)
	}

	return buf.String(), nil
}

// runCommand executes a system command and captures its output.
func (d *DynamicCommand) runCommand(cmd *exec.Cmd) (string, error) {
	// Capture both stdout and stderr
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	// Run command
	err := cmd.Run()
	if err != nil {
		// Include both stdout and stderr in error message
		output := stdoutBuf.String() + stderrBuf.String()
		return output, fmt.Errorf("command execution failed: %w (output: %s)", err, output)
	}

	return stdoutBuf.String(), nil
}
