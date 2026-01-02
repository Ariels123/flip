package commands

import (
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"

	"flip2/internal/repl"
)

// HelpCommand provides comprehensive help for all available commands.
// Implements subcommands: no args (list all), <cmd> (detailed help)
type HelpCommand struct {
	apiClient *repl.APIClient
}

// NewHelpCommand creates a new help command handler.
func NewHelpCommand(apiClient *repl.APIClient) *HelpCommand {
	return &HelpCommand{
		apiClient: apiClient,
	}
}

// Name returns the command name.
func (c *HelpCommand) Name() string {
	return "help"
}

// Aliases returns alternative command names.
func (c *HelpCommand) Aliases() []string {
	return []string{"h", "?"}
}

// Description returns a short description.
func (c *HelpCommand) Description() string {
	return "Show help information about commands"
}

// Usage returns the usage string.
func (c *HelpCommand) Usage() string {
	return "/help [command]"
}

// Help returns detailed help text.
func (c *HelpCommand) Help() string {
	return `Show help information for FLIP2 REPL commands.

Usage: /help [command]

Without arguments, displays a list of all available commands with brief descriptions.
With a command name, displays detailed help including usage, examples, and options.

Examples:
  /help              List all available commands
  /help task         Show detailed help for /task command
  /help send         Show detailed help for /send command
  /help agents       Show detailed help for /agents command

Tips:
  - Use /help <cmd> to see full usage and examples
  - Commands are context-aware and support tab completion
  - Some commands accept flags like --priority, --assignee, etc.
  - Use "." to reference the most recent task or signal
  - Use /use <agent> to set a default target agent for commands
`
}

// Execute implements the Command interface.
func (c *HelpCommand) Execute(ctx *repl.Context, args []string) error {
	if len(args) == 0 {
		// List all available commands with descriptions
		return c.listAllCommands(ctx)
	}

	// Show detailed help for a specific command
	cmdName := args[0]
	return c.showCommandHelp(ctx, cmdName)
}

// Complete provides tab completion suggestions.
func (c *HelpCommand) Complete(ctx *repl.Context, args []string, pos int, partial string) []repl.Completion {
	if pos == 0 {
		// Complete command names
		commands := getAllCommandNames()
		var matches []repl.Completion
		for _, cmd := range commands {
			if strings.HasPrefix(cmd, partial) {
				matches = append(matches, repl.Completion{
					Value:       cmd,
					Description: getCommandDescription(cmd),
					Type:        repl.CompletionCommand,
				})
			}
		}
		return matches
	}
	return nil
}

// listAllCommands displays all available commands in a formatted table.
func (c *HelpCommand) listAllCommands(ctx *repl.Context) error {
	fmt.Fprintln(ctx.Output, "")
	fmt.Fprintln(ctx.Output, "FLIP2 Available Commands:")
	fmt.Fprintln(ctx.Output, strings.Repeat("=", 70))
	fmt.Fprintln(ctx.Output, "")

	w := tabwriter.NewWriter(ctx.Output, 0, 0, 2, ' ', 0)

	// Collect all commands and sort them
	commands := []struct {
		name        string
		aliases     string
		description string
	}{
		{
			name:        "help",
			aliases:     "h, ?",
			description: "Show help information about commands",
		},
		{
			name:        "status",
			aliases:     "st, s",
			description: "Show system and daemon status",
		},
		{
			name:        "task",
			aliases:     "t",
			description: "Manage tasks (list, create, show, start, done, cancel)",
		},
		{
			name:        "agents",
			aliases:     "a",
			description: "List and manage agents",
		},
		{
			name:        "send",
			aliases:     "s, msg",
			description: "Send a signal/message to an agent",
		},
		{
			name:        "spawn",
			aliases:     "run, exec",
			description: "Spawn a new worker agent instance",
		},
		{
			name:        "use",
			aliases:     "",
			description: "Set the default target agent",
		},
		{
			name:        "clear",
			aliases:     "cls",
			description: "Clear the terminal screen",
		},
		{
			name:        "watch",
			aliases:     "w",
			description: "Watch for real-time signals and task updates",
		},
		{
			name:        "history",
			aliases:     "hist",
			description: "Show command history",
		},
		{
			name:        "exit",
			aliases:     "quit, q",
			description: "Exit the REPL",
		},
	}

	// Print header
	fmt.Fprintln(w, "COMMAND\tALIASES\tDESCRIPTION")
	fmt.Fprintln(w, "-------\t-------\t-----------")

	// Print each command
	for _, cmd := range commands {
		fmt.Fprintf(w, "/%s\t%s\t%s\n", cmd.name, cmd.aliases, cmd.description)
	}

	w.Flush()
	fmt.Fprintln(ctx.Output, "")
	fmt.Fprintln(ctx.Output, "Type /help <command> for detailed help on a specific command.")
	fmt.Fprintln(ctx.Output, "")

	return nil
}

// showCommandHelp displays detailed help for a specific command.
func (c *HelpCommand) showCommandHelp(ctx *repl.Context, cmdName string) error {
	help := getDetailedHelp(cmdName)
	if help == "" {
		return fmt.Errorf("unknown command: %s", cmdName)
	}

	fmt.Fprintln(ctx.Output, "")
	fmt.Fprint(ctx.Output, help)
	fmt.Fprintln(ctx.Output, "")

	return nil
}

// getDetailedHelp returns detailed help text for a command.
func getDetailedHelp(cmdName string) string {
	helpMap := map[string]string{
		"help": `HELP - Show help information

Usage: /help [command]

Description:
  Display help information for FLIP2 commands. Without arguments, lists all
  available commands. With a command name argument, shows detailed help for
  that specific command.

Subcommands/Arguments:
  (none)        List all available commands with brief descriptions
  <command>     Show detailed help for the specified command

Examples:
  /help                 List all commands
  /help task            Show help for /task
  /help send            Show help for /send
  /help agents          Show help for /agents
  /help spawn           Show help for /spawn

Aliases: h, ?

See Also: /status, /agents, /task`,

		"status": `STATUS - Show system and daemon status

Usage: /status [--json]

Description:
  Display the current status of the FLIP2 daemon, connected agents,
  and system information including uptime and resource usage.

Options:
  --json    Output in JSON format

Examples:
  /status           Show human-readable status
  /status --json    Show machine-readable status

Output includes:
  - System resources (goroutines, memory usage)
  - API health status
  - Agent summary (online, offline, busy counts)
  - Task queue statistics (pending, running, completed, failed)

Aliases: st, s

See Also: /agents, /watch`,

		"task": `TASK - Manage tasks

Usage: /task [subcommand] [args]

Description:
  Manage tasks in the FLIP2 system. Create, list, show details, and update
  task status (start, mark done, cancel).

Subcommands:
  list, ls                    List all tasks with status and priority
  create, add, new <title>    Create a new task with optional metadata
  show, get <id>              Show detailed information about a task
  start <id>                  Mark a task as in_progress
  done, complete <id>         Mark a task as completed
  cancel <id>                 Mark a task as cancelled

Flags (for create subcommand):
  --assignee <agent>          Assign task to an agent
  --priority <1-5>            Set task priority (default: 3)
  --description <text>        Add task description

Special References:
  .                           Refers to the most recently created/viewed task

Examples:
  /task list
  /task create "Implement feature X"
  /task create "Fix bug" --priority 5 --assignee claude
  /task show TASK-001
  /task start .
  /task done TASK-001
  /task cancel TASK-002

Aliases: t

See Also: /send, /agents, /spawn`,

		"agents": `AGENTS - List and manage agents

Usage: /agents [subcommand]

Description:
  View and manage agents in the FLIP2 system. List active agents, view details,
  and manage agent registration and status.

Subcommands:
  list, ls            List all registered agents with status
  add <agent-id>      Register a new agent (requires --backend flag)
  remove <agent-id>   Unregister an agent
  info <agent-id>     Show detailed information about an agent

Flags:
  --backend <type>    Agent backend type (claude, gemini, etc.)
  --status <status>   Filter agents by status

Examples:
  /agents list
  /agents add claude-worker-1 --backend claude
  /agents info claude-mac
  /agents remove old-agent

Aliases: a

See Also: /use, /send, /task`,

		"send": `SEND - Send a signal/message to an agent

Usage: /send [agent] <message> [flags]

Description:
  Send a signal or message to an agent for communication and task assignment.
  If no agent is specified, uses the default agent set via /use.

Arguments:
  agent       Target agent ID (optional if default set with /use)
  message     Message content to send

Flags:
  --priority <level>    Message priority: high, normal, low (default: normal)
  --type <type>         Signal type: message, task, query (default: message)
  --timeout <seconds>   Response timeout in seconds

Examples:
  /use claude
  /send "Analyze this codebase"           # Uses default agent
  /send claude-mac "Hello"
  /send gemini "What's 2+2?" --priority high
  /send claude-worker "Execute task X" --type task

Aliases: s, msg

See Also: /use, /agents, /watch`,

		"spawn": `SPAWN - Spawn a new worker agent instance

Usage: /spawn <agent-id> <backend> [prompt]

Description:
  Create and spawn a new worker agent instance. Worker agents are typically
  short-lived instances spawned to perform specific tasks.

Arguments:
  agent-id    Unique identifier for the new agent
  backend     Backend type: claude, gemini, etc.
  prompt      Initial task/prompt for the agent (optional)

Examples:
  /spawn worker-1 claude "Analyze error logs"
  /spawn research-bot gemini
  /spawn task-processor claude --priority high

Aliases: run, exec

See Also: /agents, /task, /send`,

		"use": `USE - Set the default target agent

Usage: /use [agent-id]

Description:
  Set the default target agent for commands that require an agent parameter.
  Once set, many commands can omit the agent argument and will use the default.

Arguments:
  agent-id    Agent ID to set as default (or - to clear)

Examples:
  /use claude-mac
  /use gemini
  /use -                    # Clear default agent
  /use                      # Show current default agent

See Also: /agents, /send`,

		"clear": `CLEAR - Clear the terminal screen

Usage: /clear

Description:
  Clear all text from the terminal screen.

Aliases: cls

See Also: /history`,

		"watch": `WATCH - Watch for real-time updates

Usage: /watch [type] [flags]

Description:
  Monitor and display real-time signals and task updates in the FLIP2 system.

Arguments:
  type        Type of updates: signals, tasks, all (default: signals)

Flags:
  --agent <id>        Filter updates for specific agent

Examples:
  /watch
  /watch signals
  /watch tasks
  /watch all --agent claude-mac
  /watch signals --agent gemini

Press Ctrl+C to stop watching.

Aliases: w

See Also: /task, /send, /agents`,

		"history": `HISTORY - Show command history

Usage: /history [count]

Description:
  Display recent command history. Use arrow keys in the REPL to navigate
  command history without using this command.

Arguments:
  count       Number of recent commands to show (default: 20)

Examples:
  /history
  /history 50

Aliases: hist

See Also: /clear`,

		"exit": `EXIT - Exit the REPL

Usage: /exit

Description:
  Terminate the FLIP2 interactive shell and return to the system shell.

Examples:
  /exit
  /quit
  /q

Aliases: quit, q

Note: You can also use Ctrl+D to exit.`,
	}

	if help, exists := helpMap[cmdName]; exists {
		return help
	}

	return ""
}

// getCommandDescription returns a brief description of a command.
func getCommandDescription(cmdName string) string {
	descriptions := map[string]string{
		"help":    "Show help information about commands",
		"status":  "Show system and daemon status",
		"task":    "Manage tasks",
		"agents":  "List and manage agents",
		"send":    "Send a signal/message to an agent",
		"spawn":   "Spawn a new worker agent instance",
		"use":     "Set the default target agent",
		"clear":   "Clear the terminal screen",
		"watch":   "Watch for real-time updates",
		"history": "Show command history",
		"exit":    "Exit the REPL",
	}

	if desc, exists := descriptions[cmdName]; exists {
		return desc
	}
	return ""
}

// getAllCommandNames returns a sorted list of all command names.
func getAllCommandNames() []string {
	names := []string{
		"agents",
		"clear",
		"exit",
		"help",
		"history",
		"send",
		"spawn",
		"status",
		"task",
		"use",
		"watch",
	}
	sort.Strings(names)
	return names
}
