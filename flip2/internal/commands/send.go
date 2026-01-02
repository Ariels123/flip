package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"flip2/internal/repl"
)

// SignalRequest represents the payload for sending a signal.
type SignalRequest struct {
	ToAgent    string `json:"to_agent"`
	Content    string `json:"content"`
	Priority   string `json:"priority,omitempty"`
	SignalType string `json:"signal_type,omitempty"`
}

// SignalResponse represents the API response when a signal is sent.
type SignalResponse struct {
	SignalID string `json:"signal_id"`
	ToAgent  string `json:"to_agent"`
	Status   string `json:"status"`
	Error    string `json:"error,omitempty"`
}

// SendCommand implements the /send slash command for sending signals to agents.
// It supports optional flags for priority and signal type.
type SendCommand struct {
	apiClient *repl.APIClient
}

// NewSendCommand creates a new send command handler.
func NewSendCommand(apiClient *repl.APIClient) *SendCommand {
	return &SendCommand{
		apiClient: apiClient,
	}
}

// Name returns the command name.
func (c *SendCommand) Name() string {
	return "send"
}

// Aliases returns alternative command names.
func (c *SendCommand) Aliases() []string {
	return []string{"msg", "signal"}
}

// Description returns a short description of the command.
func (c *SendCommand) Description() string {
	return "Send a signal to an agent"
}

// Usage returns the usage string.
func (c *SendCommand) Usage() string {
	return "/send <agent> <message> [--priority high|normal|low] [--type message|task|alert]"
}

// Help returns detailed help text.
func (c *SendCommand) Help() string {
	return `Send a signal to an agent.

Usage: /send <agent> <message> [options]

Arguments:
  agent     Target agent ID (tab-complete available)
  message   The message content (can be quoted for spaces)

Options:
  --priority  Signal priority: high, normal (default), low
  --type      Signal type: message (default), task, alert

Examples:
  /send claude "Analyze this file"
  /send gemini "Research topic X" --priority high
  /send worker-1 "Process data.csv" --type task

If you have a default agent set (via /use), you can omit the agent:
  /use claude
  /send "Analyze this file"    # Sends to claude
`
}

// Execute implements the Command interface.
// Parses arguments and sends a signal via the API.
func (c *SendCommand) Execute(ctx *repl.Context, args []string) error {
	if ctx == nil || c.apiClient == nil {
		return fmt.Errorf("context or API client not available")
	}

	// Parse arguments
	if len(args) == 0 {
		return fmt.Errorf("missing required arguments: /send <agent> <message> [options]")
	}

	// First arg could be agent or message (if default agent is set)
	var toAgent, message string
	var flagIndex int

	// Check if first arg starts with -- (it's a flag, not agent)
	if strings.HasPrefix(args[0], "--") {
		// No agent specified, use current default
		toAgent = ctx.Session.GetAgent()
		if toAgent == "" {
			return fmt.Errorf("no agent specified and no default agent set (use /use <agent> to set default)")
		}
		message = ""
		flagIndex = 0
	} else {
		// First arg is the agent
		toAgent = args[0]
		if len(args) < 2 {
			return fmt.Errorf("missing message: /send <agent> <message> [options]")
		}

		// Second arg is the message
		message = args[1]
		flagIndex = 2
	}

	// Parse flags
	priority := "normal"
	signalType := "message"

	for i := flagIndex; i < len(args); i++ {
		if strings.HasPrefix(args[i], "--") {
			flag := args[i]
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
				value := args[i+1]
				switch flag {
				case "--priority":
					priority = value
				case "--type":
					signalType = value
				case "--from":
					// Note: The API uses X-Agent-ID header for sender, we can't override it from CLI
					// but we accept the flag for forward compatibility
				}
				i++ // Skip the value in next iteration
			}
		}
	}

	// Validate inputs
	if toAgent == "" {
		return fmt.Errorf("agent ID cannot be empty")
	}
	if message == "" {
		return fmt.Errorf("message cannot be empty")
	}

	// Validate priority
	validPriorities := map[string]bool{"high": true, "normal": true, "low": true}
	if !validPriorities[priority] {
		return fmt.Errorf("invalid priority: %s (must be high, normal, or low)", priority)
	}

	// Validate signal type
	validTypes := map[string]bool{"message": true, "task": true, "alert": true}
	if !validTypes[signalType] {
		return fmt.Errorf("invalid signal type: %s (must be message, task, or alert)", signalType)
	}

	// Create request payload
	req := SignalRequest{
		ToAgent:    toAgent,
		Content:    message,
		Priority:   priority,
		SignalType: signalType,
	}

	// Marshal to JSON
	payloadBytes, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Make API request
	httpReq, err := http.NewRequest(
		http.MethodPost,
		c.apiClient.BaseURL()+"/api/signals",
		bytes.NewReader(payloadBytes),
	)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	// Execute the request using the default HTTP client
	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Parse response
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var respData SignalResponse
	if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if respData.Error != "" {
		return fmt.Errorf("API error: %s", respData.Error)
	}

	// Display confirmation
	fmt.Fprintf(ctx.Output, "Signal sent to %s (ID: %s)\n", respData.ToAgent, respData.SignalID)

	// Store in session for quick reference
	ctx.Session.LastSignal = respData.SignalID

	return nil
}

// Complete implements the Command interface for tab completion.
func (c *SendCommand) Complete(ctx *repl.Context, args []string, pos int, partial string) []repl.Completion {
	// Position 0: Complete agent names
	if pos == 0 {
		// Convert from local Completion to repl.Completion
		localCompletions := completeAgents(partial)
		result := make([]repl.Completion, len(localCompletions))
		for i, lc := range localCompletions {
			result[i] = repl.Completion{
				Value:       lc.Value,
				Description: lc.Description,
				Type:        repl.CompletionType(lc.Type),
			}
		}
		return result
	}

	// Position 1+: Complete flags
	if strings.HasPrefix(partial, "--") {
		flags := []repl.Completion{
			{Value: "--priority", Description: "Signal priority", Type: repl.CompletionFlag},
			{Value: "--type", Description: "Signal type", Type: repl.CompletionFlag},
			{Value: "--from", Description: "Sender ID", Type: repl.CompletionFlag},
		}
		var matches []repl.Completion
		for _, f := range flags {
			if strings.HasPrefix(f.Value, partial) {
				matches = append(matches, f)
			}
		}
		return matches
	}

	// After --priority: complete priority values
	if pos > 0 && len(args) > 0 && args[len(args)-1] == "--priority" {
		priorities := []repl.Completion{
			{Value: "high", Description: "High priority", Type: repl.CompletionValue},
			{Value: "normal", Description: "Normal priority (default)", Type: repl.CompletionValue},
			{Value: "low", Description: "Low priority", Type: repl.CompletionValue},
		}
		var matches []repl.Completion
		for _, p := range priorities {
			if strings.HasPrefix(p.Value, partial) {
				matches = append(matches, p)
			}
		}
		return matches
	}

	// After --type: complete type values
	if pos > 0 && len(args) > 0 && args[len(args)-1] == "--type" {
		types := []repl.Completion{
			{Value: "message", Description: "Regular message (default)", Type: repl.CompletionValue},
			{Value: "task", Description: "Task assignment", Type: repl.CompletionValue},
			{Value: "alert", Description: "Alert notification", Type: repl.CompletionValue},
		}
		var matches []repl.Completion
		for _, t := range types {
			if strings.HasPrefix(t.Value, partial) {
				matches = append(matches, t)
			}
		}
		return matches
	}

	return nil
}
