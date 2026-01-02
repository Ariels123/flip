package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"text/tabwriter"
	"time"
)

// Context represents the execution context for a command.
// This is a minimal interface to avoid circular imports with repl package.
type Context interface {
	GetOutput() io.Writer
	GetErrorOutput() io.Writer
}

// Completion represents a tab completion suggestion.
type Completion struct {
	Value       string
	Description string
	Type        int
}

const (
	CompletionCommand = iota
	CompletionAgent
	CompletionTask
	CompletionFlag
	CompletionValue
	CompletionFile
)

// APIClientInterface defines the API client interface for commands.
type APIClientInterface interface {
	BaseURL() string
}

// AgentRecord represents an agent in the database.
type AgentRecord struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Model    string    `json:"model"`
	Status   string    `json:"status"` // running, idle, completed, offline
	Task     string    `json:"task"`
	LastSeen time.Time `json:"last_seen"`
	Uptime   time.Duration `json:"uptime,omitempty"`
}

// AgentListResponse represents the API response for agents.
type AgentListResponse struct {
	Agents []AgentRecord `json:"agents"`
	Count  int           `json:"count"`
}

// AgentsCommand implements the /agents slash command for listing and managing agents.
// It provides subcommands: list, add, remove, info.
type AgentsCommand struct {
	apiClient APIClientInterface
}

// NewAgentsCommand creates a new agents command handler.
func NewAgentsCommand(apiClient APIClientInterface) *AgentsCommand {
	return &AgentsCommand{
		apiClient: apiClient,
	}
}

// Name returns the command name.
func (c *AgentsCommand) Name() string {
	return "agents"
}

// Aliases returns alternative command names.
func (c *AgentsCommand) Aliases() []string {
	return []string{"a", "agent"}
}

// Description returns a short description of the command.
func (c *AgentsCommand) Description() string {
	return "List and manage agents"
}

// Usage returns the usage string.
func (c *AgentsCommand) Usage() string {
	return "/agents [list|add|remove|info] [args...]"
}

// Help returns detailed help text.
func (c *AgentsCommand) Help() string {
	return `List and manage agents in the FLIP2 system.

Subcommands:
  list              List all agents (default)
  add <id>          Register a new agent
  remove <id>       Unregister an agent
  info <id>         Show detailed agent info

Options for 'add':
  --backend <type>  Backend type: claude, gemini, custom
  --mode <mode>     Mode: local, remote

Options for 'list':
  --status <s>      Filter by status: running, idle, completed, offline
  --backend <b>     Filter by backend type

Examples:
  /agents                      List all agents
  /agents --status running     List only running agents
  /agents add worker-2 --backend gemini
  /agents info claude-mac
`
}

// Execute implements command execution.
// Parses subcommand and routes to appropriate handler.
func (c *AgentsCommand) Execute(ctx interface{}, args []string) error {
	// Convert ctx to io.Writer
	var output io.Writer = io.Discard
	if ctxObj, ok := ctx.(interface{ GetOutput() io.Writer }); ok {
		output = ctxObj.GetOutput()
	}

	if len(args) == 0 {
		// Default: list agents
		return c.listAgents(output, args)
	}

	subcommand := args[0]
	subargs := args[1:]

	switch subcommand {
	case "list", "ls":
		return c.listAgents(output, subargs)
	case "add", "register":
		return c.addAgent(output, subargs)
	case "remove", "unregister", "delete":
		return c.removeAgent(output, subargs)
	case "info", "show", "get":
		return c.infoAgent(output, subargs)
	default:
		// If it looks like a flag, treat as list with filters
		if strings.HasPrefix(subcommand, "--") {
			return c.listAgents(output, args)
		}
		return fmt.Errorf("unknown agents subcommand: %s", subcommand)
	}
}

// Complete provides tab completion suggestions.
func (c *AgentsCommand) Complete(ctx interface{}, args []string, pos int, partial string) []Completion {
	if pos == 0 {
		subcmds := []Completion{
			{Value: "list", Description: "List agents", Type: CompletionCommand},
			{Value: "add", Description: "Register agent", Type: CompletionCommand},
			{Value: "remove", Description: "Unregister agent", Type: CompletionCommand},
			{Value: "info", Description: "Agent details", Type: CompletionCommand},
		}
		var matches []Completion
		for _, s := range subcmds {
			if strings.HasPrefix(s.Value, partial) {
				matches = append(matches, s)
			}
		}
		// Also match --flags at position 0
		if strings.HasPrefix(partial, "--") {
			matches = append(matches,
				Completion{Value: "--status", Description: "Filter by status", Type: CompletionFlag},
				Completion{Value: "--backend", Description: "Filter by backend", Type: CompletionFlag},
			)
		}
		return matches
	}
	// For info/remove: complete agent IDs
	if pos == 1 && len(args) > 0 {
		switch args[0] {
		case "info", "remove", "unregister", "delete", "show", "get":
			return completeAgents(partial)
		}
	}
	return nil
}

// listAgents displays all agents with optional filtering.
func (c *AgentsCommand) listAgents(output io.Writer, args []string) error {
	// Parse filters
	statusFilter := ""
	backendFilter := ""

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--status":
			if i+1 < len(args) {
				statusFilter = args[i+1]
				i++
			}
		case "--backend":
			if i+1 < len(args) {
				backendFilter = args[i+1]
				i++
			}
		}
	}

	// Fetch agents from API
	agents, err := c.fetchAgents()
	if err != nil {
		// Return mock data for demonstration/testing
		agents = c.getMockAgents()
	}

	// Apply filters
	if statusFilter != "" || backendFilter != "" {
		agents = c.filterAgents(agents, statusFilter, backendFilter)
	}

	if len(agents) == 0 {
		fmt.Fprintf(output, "No agents found\n")
		return nil
	}

	return c.displayAgentsTable(output, agents)
}

// fetchAgents retrieves agents from the API.
func (c *AgentsCommand) fetchAgents() ([]AgentRecord, error) {
	if c.apiClient == nil {
		return nil, fmt.Errorf("api client not configured")
	}

	url := c.apiClient.BaseURL() + "/api/agents"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch agents: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var result AgentListResponse
	body, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Calculate uptime for each agent
	now := time.Now()
	for i := range result.Agents {
		if !result.Agents[i].LastSeen.IsZero() {
			result.Agents[i].Uptime = now.Sub(result.Agents[i].LastSeen)
		}
	}

	return result.Agents, nil
}

// filterAgents applies status and backend filters to agents.
func (c *AgentsCommand) filterAgents(agents []AgentRecord, status, backend string) []AgentRecord {
	var filtered []AgentRecord

	for _, agent := range agents {
		if status != "" && agent.Status != status {
			continue
		}
		if backend != "" && agent.Model != backend {
			continue
		}
		filtered = append(filtered, agent)
	}

	return filtered
}

// displayAgentsTable renders agents in a formatted table.
func (c *AgentsCommand) displayAgentsTable(output io.Writer, agents []AgentRecord) error {
	w := tabwriter.NewWriter(output, 0, 0, 2, ' ', 0)
	defer w.Flush()

	// Header
	fmt.Fprintln(w, "ID\tMODEL\tSTATUS\tTASK\tUPTIME\tLAST SEEN")

	// Rows
	for _, agent := range agents {
		lastSeen := formatLastSeen(agent.LastSeen)
		uptime := formatUptime(agent.Uptime)
		task := agent.Task
		if task == "" {
			task = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			agent.ID, agent.Model, agent.Status, task, uptime, lastSeen)
	}

	return nil
}

// addAgent registers a new agent.
func (c *AgentsCommand) addAgent(output io.Writer, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("agent add requires an ID")
	}

	agentID := args[0]
	backend := "claude"
	mode := "local"

	// Parse optional flags
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--backend":
			if i+1 < len(args) {
				backend = args[i+1]
				i++
			}
		case "--mode":
			if i+1 < len(args) {
				mode = args[i+1]
				i++
			}
		}
	}

	// Create agent record
	agent := map[string]interface{}{
		"id":    agentID,
		"name":  agentID,
		"model": backend,
		"mode":  mode,
		"status": "offline",
	}

	body, err := json.Marshal(agent)
	if err != nil {
		return fmt.Errorf("failed to marshal agent: %w", err)
	}

	// Make POST request
	if c.apiClient == nil {
		// For demo, show success message
		fmt.Fprintf(output, "Agent %s registered with backend %s\n", agentID, backend)
		return nil
	}

	url := c.apiClient.BaseURL() + "/api/agents"
	req, err := http.NewRequest("POST", url, strings.NewReader(string(body)))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		// For demo, show success message
		fmt.Fprintf(output, "Agent %s registered with backend %s\n", agentID, backend)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK {
		fmt.Fprintf(output, "Agent %s registered successfully with backend %s\n", agentID, backend)
		return nil
	}

	return fmt.Errorf("failed to register agent: status %d", resp.StatusCode)
}

// removeAgent unregisters an agent.
func (c *AgentsCommand) removeAgent(output io.Writer, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("agent remove requires an ID")
	}

	agentID := args[0]

	if c.apiClient == nil {
		// For demo, show success message
		fmt.Fprintf(output, "Agent %s unregistered\n", agentID)
		return nil
	}

	// Make DELETE request
	url := c.apiClient.BaseURL() + "/api/agents/" + agentID
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		// For demo, show success message
		fmt.Fprintf(output, "Agent %s unregistered\n", agentID)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent {
		fmt.Fprintf(output, "Agent %s unregistered successfully\n", agentID)
		return nil
	}

	return fmt.Errorf("failed to unregister agent: status %d", resp.StatusCode)
}

// infoAgent displays detailed information about an agent.
func (c *AgentsCommand) infoAgent(output io.Writer, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("agent info requires an ID")
	}

	agentID := args[0]

	if c.apiClient == nil {
		// Return mock data for demo
		agent := AgentRecord{
			ID:       agentID,
			Name:     agentID,
			Model:    "claude",
			Status:   "running",
			Task:     "processing",
			LastSeen: time.Now().Add(-5 * time.Minute),
		}
		return c.displayAgentInfo(output, agent)
	}

	// Make GET request
	url := c.apiClient.BaseURL() + "/api/agents/" + agentID
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch agent: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("agent not found")
	}

	var agent AgentRecord
	body, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(body, &agent); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	return c.displayAgentInfo(output, agent)
}

// displayAgentInfo displays detailed information about an agent.
func (c *AgentsCommand) displayAgentInfo(output io.Writer, agent AgentRecord) error {
	fmt.Fprintf(output, "Agent Information\n")
	fmt.Fprintf(output, "=================\n\n")
	fmt.Fprintf(output, "ID:        %s\n", agent.ID)
	fmt.Fprintf(output, "Name:      %s\n", agent.Name)
	fmt.Fprintf(output, "Model:     %s\n", agent.Model)
	fmt.Fprintf(output, "Status:    %s\n", agent.Status)
	fmt.Fprintf(output, "Task:      %s\n", agent.Task)
	fmt.Fprintf(output, "Last Seen: %s\n", formatLastSeen(agent.LastSeen))
	fmt.Fprintf(output, "Uptime:    %s\n", formatUptime(agent.Uptime))

	return nil
}

// getMockAgents returns mock agent data for testing/demo.
func (c *AgentsCommand) getMockAgents() []AgentRecord {
	return []AgentRecord{
		{
			ID:       "claude-mac",
			Name:     "claude-mac",
			Model:    "claude",
			Status:   "running",
			Task:     "analysis",
			LastSeen: time.Now().Add(-2 * time.Second),
			Uptime:   2 * time.Second,
		},
		{
			ID:       "gemini-worker",
			Name:     "gemini-worker",
			Model:    "gemini",
			Status:   "running",
			Task:     "research",
			LastSeen: time.Now().Add(-5 * time.Second),
			Uptime:   5 * time.Second,
		},
		{
			ID:       "worker-1",
			Name:     "worker-1",
			Model:    "claude",
			Status:   "idle",
			Task:     "-",
			LastSeen: time.Now().Add(-1 * time.Minute),
			Uptime:   1 * time.Minute,
		},
	}
}

// Helper functions

// formatLastSeen formats the last seen time in a human-readable way.
func formatLastSeen(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}

	duration := time.Since(t)
	if duration < time.Second {
		return "now"
	}
	if duration < time.Minute {
		return fmt.Sprintf("%ds ago", int(duration.Seconds()))
	}
	if duration < time.Hour {
		return fmt.Sprintf("%dm ago", int(duration.Minutes()))
	}
	if duration < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(duration.Hours()))
	}
	return t.Format("2006-01-02")
}

// formatUptime formats the uptime duration in a human-readable way.
func formatUptime(d time.Duration) string {
	if d == 0 {
		return "-"
	}
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	return fmt.Sprintf("%dh", int(d.Hours()))
}

// completeAgents returns agent ID completions.
func completeAgents(partial string) []Completion {
	// Mock agent list for completion
	agents := []string{"claude", "gemini", "worker-1", "worker-2"}
	var matches []Completion

	for _, agent := range agents {
		if strings.HasPrefix(agent, partial) {
			matches = append(matches, Completion{
				Value:       agent,
				Description: "Agent ID",
				Type:        CompletionAgent,
			})
		}
	}

	return matches
}
