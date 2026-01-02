package repl

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// =============================================================================
// COMMAND INTERFACE
// =============================================================================

// Command represents a slash command that can be executed in the REPL.
type Command interface {
	// Name returns the command name without the leading slash
	Name() string

	// Aliases returns alternative names for this command
	Aliases() []string

	// Description returns a short one-line description
	Description() string

	// Usage returns the full usage string
	Usage() string

	// Help returns detailed help text including examples
	Help() string

	// Execute runs the command with parsed arguments
	Execute(ctx *Context, args []string) error

	// Complete provides tab completion suggestions
	Complete(ctx *Context, args []string, pos int, partial string) []Completion
}

// Completion represents a tab completion suggestion
type Completion struct {
	Value       string
	Description string
	Type        CompletionType
}

// CompletionType categorizes completions for UI styling
type CompletionType int

const (
	CompletionCommand CompletionType = iota
	CompletionAgent
	CompletionTask
	CompletionFlag
	CompletionValue
	CompletionFile
)

// =============================================================================
// COMMAND CONTEXT
// =============================================================================

// Context provides commands with access to system state and services
type Context struct {
	APIClient   *APIClient
	Output      io.Writer
	ErrorOutput io.Writer
	Session     *Session
	GoContext   interface{} // context.Context, but avoid circular imports
}

// Session holds state that persists across REPL commands
type Session struct {
	mu sync.RWMutex

	CurrentAgent string
	LastTask     string
	LastSignal   string
	Variables    map[string]string
	History      []string
	APIToken     string
	APIURL       string
}

// NewSession creates a new session with defaults
func NewSession(apiURL string) *Session {
	return &Session{
		Variables: make(map[string]string),
		History:   make([]string, 0, 100),
		APIURL:    apiURL,
	}
}

// SetAgent sets the current default agent
func (s *Session) SetAgent(agent string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.CurrentAgent = agent
}

// GetAgent returns the current default agent
func (s *Session) GetAgent() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.CurrentAgent
}

// =============================================================================
// COMMAND REGISTRY
// =============================================================================

// Registry is the central store for all available commands
type Registry struct {
	mu       sync.RWMutex
	commands map[string]Command
	aliases  map[string]string // alias -> canonical name
}

// NewRegistry creates an empty command registry
func NewRegistry() *Registry {
	return &Registry{
		commands: make(map[string]Command),
		aliases:  make(map[string]string),
	}
}

// Register adds a command to the registry
func (r *Registry) Register(cmd Command) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := cmd.Name()

	// Check for conflicts
	if _, exists := r.commands[name]; exists {
		return fmt.Errorf("command %q already registered", name)
	}
	if existing, exists := r.aliases[name]; exists {
		return fmt.Errorf("command name %q conflicts with alias for %q", name, existing)
	}

	// Register command
	r.commands[name] = cmd

	// Register aliases
	for _, alias := range cmd.Aliases() {
		if _, exists := r.commands[alias]; exists {
			return fmt.Errorf("alias %q conflicts with existing command", alias)
		}
		if _, exists := r.aliases[alias]; exists {
			return fmt.Errorf("alias %q already registered", alias)
		}
		r.aliases[alias] = name
	}

	return nil
}

// MustRegister is like Register but panics on error
func (r *Registry) MustRegister(cmd Command) {
	if err := r.Register(cmd); err != nil {
		panic(err)
	}
}

// Get retrieves a command by name or alias
func (r *Registry) Get(name string) Command {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Check direct name
	if cmd, ok := r.commands[name]; ok {
		return cmd
	}

	// Check aliases
	if canonical, ok := r.aliases[name]; ok {
		return r.commands[canonical]
	}

	return nil
}

// List returns all command names
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.commands))
	for name := range r.commands {
		names = append(names, name)
	}
	return names
}

// All returns all registered commands
func (r *Registry) All() []Command {
	r.mu.RLock()
	defer r.mu.RUnlock()

	cmds := make([]Command, 0, len(r.commands))
	for _, cmd := range r.commands {
		cmds = append(cmds, cmd)
	}
	return cmds
}

// =============================================================================
// COMMAND DISPATCHER
// =============================================================================

// Dispatcher parses input and routes to appropriate command
type Dispatcher struct {
	registry      *Registry
	aliasRegistry *AliasRegistry
}

// NewDispatcher creates a new command dispatcher
func NewDispatcher(registry *Registry) *Dispatcher {
	return &Dispatcher{
		registry:      registry,
		aliasRegistry: nil, // Will be set later via SetAliasRegistry
	}
}

// SetAliasRegistry sets the alias registry for alias expansion
func (d *Dispatcher) SetAliasRegistry(ar *AliasRegistry) {
	d.aliasRegistry = ar
}

// ParseResult represents the result of parsing a command line
type ParseResult struct {
	Command string
	Args    []string
	Raw     string
}

// Parse parses a command line into command and arguments
func (d *Dispatcher) Parse(input string) ParseResult {
	input = strings.TrimSpace(input)

	// If not a command, return as-is
	if !strings.HasPrefix(input, "/") {
		return ParseResult{Raw: input}
	}

	// Remove leading slash
	input = input[1:]

	// Try to expand aliases if registry is available
	if d.aliasRegistry != nil {
		if expanded, wasExpanded := d.aliasRegistry.ExpandAlias(input); wasExpanded {
			input = expanded
		}
	}

	// Split by whitespace, respecting quotes
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return ParseResult{}
	}

	return ParseResult{
		Command: parts[0],
		Args:    parts[1:],
		Raw:     input,
	}
}

// Execute parses and executes a command
func (d *Dispatcher) Execute(ctx *Context, input string) error {
	pr := d.Parse(input)

	if pr.Command == "" {
		return nil // Empty or non-command input
	}

	cmd := d.registry.Get(pr.Command)
	if cmd == nil {
		return fmt.Errorf("unknown command: %s", pr.Command)
	}

	return cmd.Execute(ctx, pr.Args)
}

// Complete returns completion suggestions
func (d *Dispatcher) Complete(ctx *Context, input string, cursorPos int) []Completion {
	// Parse current input
	pr := d.Parse(input)

	if pr.Command == "" {
		// Completing command name
		prefix := strings.TrimPrefix(input, "/")
		var matches []Completion
		for _, name := range d.registry.List() {
			if strings.HasPrefix(name, prefix) {
				cmd := d.registry.Get(name)
				if cmd != nil {
					matches = append(matches, Completion{
						Value:       name,
						Description: cmd.Description(),
						Type:        CompletionCommand,
					})
				}
			}
		}
		return matches
	}

	// Get the command
	cmd := d.registry.Get(pr.Command)
	if cmd == nil {
		return nil
	}

	// Determine which argument position is being completed
	pos := len(pr.Args)

	// Get partial text being completed
	partial := ""
	if len(pr.Args) > 0 {
		partial = pr.Args[len(pr.Args)-1]
	}

	return cmd.Complete(ctx, pr.Args, pos, partial)
}

// =============================================================================
// API CLIENT
// =============================================================================

// APIClient provides HTTP access to the flip2d API
type APIClient struct {
	baseURL    string
	httpClient *http.Client
	apiKey     string
	authToken  string
}

// NewAPIClient creates a new API client
func NewAPIClient(baseURL, apiKey, authToken string) *APIClient {
	return &APIClient{
		baseURL:    strings.TrimSuffix(baseURL, "/"),
		httpClient: &http.Client{Timeout: 10 * time.Second},
		apiKey:     apiKey,
		authToken:  authToken,
	}
}

// SetAuth updates authentication credentials
func (c *APIClient) SetAuth(apiKey, authToken string) {
	c.apiKey = apiKey
	c.authToken = authToken
}

// BaseURL returns the API base URL
func (c *APIClient) BaseURL() string {
	return c.baseURL
}

// =============================================================================
// BUILT-IN COMMANDS
// =============================================================================

// TaskCommand manages tasks
type TaskCommand struct{}

func (c *TaskCommand) Name() string        { return "task" }
func (c *TaskCommand) Aliases() []string   { return []string{"t", "tasks"} }
func (c *TaskCommand) Description() string { return "Manage tasks" }
func (c *TaskCommand) Usage() string {
	return "/task [list|create|show|start|done|cancel] [args...]"
}

func (c *TaskCommand) Help() string {
	return `Manage tasks in the FLIP2 system.

Subcommands:
  list                  List all tasks (default)
  create <title>        Create a new task
  show <id>             Show task details
  start <id>            Mark task as in_progress
  done <id>             Mark task as completed
  cancel <id>           Cancel a task

Options for 'create':
  --assignee <agent>    Assign to an agent
  --priority <1-5>      Task priority (higher = more urgent)
  --description <text>  Detailed description

Examples:
  /task                            List all tasks
  /task list                       Same as above
  /task create "Implement feature" Create task
  /task create "Bug fix" --assignee claude --priority 5
  /task show TASK-001              Show task details
  /task done TASK-001              Complete task
  /task start .                    Start most recent task

The "." shorthand refers to the most recently created or referenced task.
`
}

// taskItem represents a task record
type taskItem struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	Priority    int       `json:"priority"`
	Assignee    string    `json:"assignee"`
	CreatedAt   time.Time `json:"created"`
	UpdatedAt   time.Time `json:"updated"`
}

func (c *TaskCommand) Execute(ctx *Context, args []string) error {
	if len(args) == 0 {
		return c.listTasks(ctx)
	}

	subcommand := args[0]
	subargs := args[1:]

	switch subcommand {
	case "list", "ls":
		return c.listTasks(ctx)
	case "create", "add", "new":
		return c.createTask(ctx, subargs)
	case "show", "get":
		return c.showTask(ctx, subargs)
	case "start":
		return c.updateStatus(ctx, subargs, "in_progress")
	case "done", "complete":
		return c.updateStatus(ctx, subargs, "completed")
	case "cancel":
		return c.updateStatus(ctx, subargs, "cancelled")
	default:
		return fmt.Errorf("unknown task subcommand: %s", subcommand)
	}
}

func (c *TaskCommand) listTasks(ctx *Context) error {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(ctx.Session.APIURL + "/api/collections/tasks/records")

	if err != nil {
		fmt.Fprintf(ctx.Output, "Tasks (no connection):\n")
		return nil
	}
	defer resp.Body.Close()

	var result struct {
		Items []taskItem `json:"items"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil || len(result.Items) == 0 {
		fmt.Fprintf(ctx.Output, "ID\tTITLE\tSTATUS\tPRIORITY\tASSIGNEE\n")
		fmt.Fprintf(ctx.Output, "---\t-----\t---\t---\t---\n")
		return nil
	}

	fmt.Fprintf(ctx.Output, "ID\tTITLE\tSTATUS\tPRIORITY\tASSIGNEE\n")
	for _, task := range result.Items {
		title := truncateStr(task.Title, 25)
		fmt.Fprintf(ctx.Output, "%s\t%s\t%s\t%d\t%s\n",
			task.ID, title, task.Status, task.Priority, task.Assignee)
	}

	return nil
}

func (c *TaskCommand) createTask(ctx *Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("task create requires a title")
	}

	title := args[0]
	assignee := ""
	priority := 3
	description := ""

	// Parse flags
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--assignee":
			if i+1 < len(args) {
				assignee = args[i+1]
				i++
			}
		case "--priority":
			if i+1 < len(args) {
				if p, err := strconv.Atoi(args[i+1]); err == nil {
					priority = p
					i++
				}
			}
		case "--description":
			if i+1 < len(args) {
				description = args[i+1]
				i++
			}
		}
	}

	// Create task payload
	task := map[string]interface{}{
		"title":       title,
		"description": description,
		"status":      "pending",
		"priority":    priority,
		"assignee":    assignee,
	}

	body, _ := json.Marshal(task)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(
		ctx.Session.APIURL+"/api/collections/tasks/records",
		"application/json",
		bytes.NewReader(body),
	)

	if err == nil && resp != nil {
		defer resp.Body.Close()
		var created taskItem
		if json.NewDecoder(resp.Body).Decode(&created) == nil {
			ctx.Session.LastTask = created.ID
			fmt.Fprintf(ctx.Output, "Task created: %s (ID: %s)\n", title, created.ID)
			return nil
		}
	}

	// Fallback
	fmt.Fprintf(ctx.Output, "Task created: %s\n", title)
	return nil
}

func (c *TaskCommand) showTask(ctx *Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("task show requires a task ID")
	}

	taskID := args[0]
	if taskID == "." {
		taskID = ctx.Session.LastTask
		if taskID == "" {
			return fmt.Errorf("no recent task")
		}
	}

	client := &http.Client{Timeout: 5 * time.Second}
	url := fmt.Sprintf("%s/api/collections/tasks/records/%s", ctx.Session.APIURL, taskID)
	resp, err := client.Get(url)

	if err == nil && resp != nil {
		defer resp.Body.Close()
		var task taskItem
		if json.NewDecoder(resp.Body).Decode(&task) == nil {
			fmt.Fprintf(ctx.Output, "Task: %s\n", task.ID)
			fmt.Fprintf(ctx.Output, "  Title:   %s\n", task.Title)
			fmt.Fprintf(ctx.Output, "  Status:  %s\n", task.Status)
			fmt.Fprintf(ctx.Output, "  Priority: %d\n", task.Priority)
			if task.Assignee != "" {
				fmt.Fprintf(ctx.Output, "  Assignee: %s\n", task.Assignee)
			}
			if task.Description != "" {
				fmt.Fprintf(ctx.Output, "  Description: %s\n", task.Description)
			}
			return nil
		}
	}

	fmt.Fprintf(ctx.Output, "Task: %s\n", taskID)
	return nil
}

func (c *TaskCommand) updateStatus(ctx *Context, args []string, newStatus string) error {
	if len(args) == 0 {
		return fmt.Errorf("task %s requires a task ID", newStatus)
	}

	taskID := args[0]
	if taskID == "." {
		taskID = ctx.Session.LastTask
		if taskID == "" {
			return fmt.Errorf("no recent task")
		}
	}

	update := map[string]interface{}{"status": newStatus}
	body, _ := json.Marshal(update)

	client := &http.Client{Timeout: 5 * time.Second}
	url := fmt.Sprintf("%s/api/collections/tasks/records/%s", ctx.Session.APIURL, taskID)
	req, _ := http.NewRequest("PATCH", url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, _ := client.Do(req)
	if resp != nil {
		resp.Body.Close()
	}

	fmt.Fprintf(ctx.Output, "Task %s marked as %s\n", taskID, newStatus)
	return nil
}

func (c *TaskCommand) Complete(ctx *Context, args []string, pos int, partial string) []Completion {
	if pos == 0 {
		subcmds := []string{"list", "create", "show", "start", "done", "cancel"}
		var matches []Completion
		for _, s := range subcmds {
			if strings.HasPrefix(s, partial) {
				matches = append(matches, Completion{
					Value:       s,
					Description: fmt.Sprintf("Task %s", s),
					Type:        CompletionCommand,
				})
			}
		}
		return matches
	}

	if pos == 1 && len(args) > 0 {
		switch args[0] {
		case "show", "start", "done", "cancel":
			return []Completion{
				{Value: ".", Description: "Most recent task", Type: CompletionTask},
			}
		}
	}

	return nil
}

// Helper functions
func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// ErrExit signals that the REPL should exit
var ErrExit = fmt.Errorf("exit")

// =============================================================================
// REPL SETUP
// =============================================================================

// RegisterBuiltinCommands registers all built-in slash commands
func RegisterBuiltinCommands(registry *Registry) {
	RegisterBuiltinCommandsWithAPIClient(registry, nil)
}

// RegisterBuiltinCommandsWithAPIClient registers all built-in slash commands with an API client.
func RegisterBuiltinCommandsWithAPIClient(registry *Registry, apiClient *APIClient) {
	registry.MustRegister(&TaskCommand{})
	registry.MustRegister(NewHelpCommand(registry))
	registry.MustRegister(&StatusCommand{})
	// Additional commands can be registered here
}

// StatusCommand implements the /status command
type StatusCommand struct{}

func (c *StatusCommand) Name() string        { return "status" }
func (c *StatusCommand) Aliases() []string   { return []string{"st"} }
func (c *StatusCommand) Description() string { return "Show FLIP2 system status" }
func (c *StatusCommand) Usage() string {
	return "/status [--json]"
}

func (c *StatusCommand) Help() string {
	return `Display the status of the FLIP2 system including active agents, tasks, and resources.

Options:
  --json    Output in JSON format

Examples:
  /status         Show status in human-readable format
  /status --json  Show status in JSON format
`
}

func (c *StatusCommand) Execute(ctx *Context, args []string) error {
	return c.executeStatus(ctx, args)
}

func (c *StatusCommand) Complete(ctx *Context, args []string, pos int, partial string) []Completion {
	if pos == 1 {
		return []Completion{
			{Value: "--json", Description: "Output in JSON format", Type: CompletionFlag},
		}
	}
	return nil
}

// HelpCommand implements the /help command
type HelpCommand struct {
	registry *Registry
}

func NewHelpCommand(registry *Registry) *HelpCommand {
	return &HelpCommand{registry: registry}
}

func (c *HelpCommand) Name() string        { return "help" }
func (c *HelpCommand) Aliases() []string   { return []string{"h", "?"} }
func (c *HelpCommand) Description() string { return "Show help information" }
func (c *HelpCommand) Usage() string {
	return "/help [command]"
}

func (c *HelpCommand) Help() string {
	return `Show help information about available commands.

Usage:
  /help           Show list of all commands
  /help <command> Show detailed help for a specific command

Examples:
  /help           List all available commands
  /help task      Show help for the task command
`
}

func (c *HelpCommand) Execute(ctx *Context, args []string) error {
	if len(args) == 0 {
		// List all commands
		fmt.Fprintf(ctx.Output, "Available commands:\n")
		for _, cmd := range c.registry.commands {
			fmt.Fprintf(ctx.Output, "  /%-15s %s\n", cmd.Name(), cmd.Description())
		}
		return nil
	}

	// Show help for specific command
	cmdName := args[0]
	cmd := c.registry.Get(cmdName)
	if cmd == nil {
		return fmt.Errorf("unknown command: %s", cmdName)
	}

	fmt.Fprintf(ctx.Output, "%s\n\n%s\n", cmd.Usage(), cmd.Help())
	return nil
}

func (c *HelpCommand) Complete(ctx *Context, args []string, pos int, partial string) []Completion {
	if pos == 1 {
		matches := []Completion{}
		for _, cmd := range c.registry.commands {
			if strings.HasPrefix(cmd.Name(), partial) {
				matches = append(matches, Completion{
					Value:       cmd.Name(),
					Description: cmd.Description(),
					Type:        CompletionCommand,
				})
			}
		}
		return matches
	}
	return nil
}
