package repl

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"github.com/chzyer/readline"
)

// REPL represents an interactive read-eval-print loop session.
// It provides the main entry point for interactive shell usage.
type REPL struct {
	// dispatcher routes parsed commands to handlers
	dispatcher *Dispatcher

	// registry holds registered commands
	registry *Registry

	// session maintains state across commands
	session *Session

	// context for cancellation and timeouts
	ctx context.Context
	cancel context.CancelFunc

	// I/O streams
	input  io.Reader
	output io.Writer
	errOut io.Writer

	// API client for command execution
	apiClient *APIClient

	// prompt to display
	prompt string

	// whether to show banner
	showBanner bool

	// readline instance for line editing and history
	rl *readline.Instance

	// history file path
	historyPath string

	// aliasRegistry manages user-defined command aliases
	aliasRegistry *AliasRegistry
}

// Config holds configuration for creating a new REPL.
type Config struct {
	// APIUrl is the base URL for the flip2d API (e.g., "http://localhost:8091")
	APIUrl string

	// APIKey for API authentication
	APIKey string

	// AuthToken for API authentication
	AuthToken string

	// Input stream (defaults to os.Stdin)
	Input io.Reader

	// Output stream (defaults to os.Stdout)
	Output io.Writer

	// ErrorOutput stream (defaults to os.Stderr)
	ErrorOutput io.Writer

	// ShowBanner whether to show welcome message
	ShowBanner bool

	// Prompt to display (defaults to "flip2> ")
	Prompt string
}

// New creates a new REPL instance with the given configuration.
func New(cfg Config) (*REPL, error) {
	// Apply defaults
	if cfg.APIUrl == "" {
		cfg.APIUrl = "http://localhost:8091"
	}
	if cfg.Input == nil {
		cfg.Input = os.Stdin
	}
	if cfg.Output == nil {
		cfg.Output = os.Stdout
	}
	if cfg.ErrorOutput == nil {
		cfg.ErrorOutput = os.Stderr
	}
	if cfg.Prompt == "" {
		cfg.Prompt = "flip2> "
	}

	// Create components
	registry := NewRegistry()
	dispatcher := NewDispatcher(registry)
	session := NewSession(cfg.APIUrl)
	apiClient := NewAPIClient(cfg.APIUrl, cfg.APIKey, cfg.AuthToken)
	aliasRegistry := NewAliasRegistry()

	// Set alias registry in dispatcher for alias expansion
	dispatcher.SetAliasRegistry(aliasRegistry)

	// Register built-in commands (with API client for commands that need it)
	RegisterBuiltinCommandsWithAPIClient(registry, apiClient)

	// Register alias command
	registry.MustRegister(NewAliasCommand(aliasRegistry))

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())

	// Setup history file path
	home, err := os.UserHomeDir()
	historyPath := filepath.Join(home, ".flip2_history")
	if err != nil {
		// Fallback if we can't get home directory
		historyPath = ".flip2_history"
	}

	repl := &REPL{
		dispatcher:    dispatcher,
		registry:      registry,
		session:       session,
		ctx:           ctx,
		cancel:        cancel,
		input:         cfg.Input,
		output:        cfg.Output,
		errOut:        cfg.ErrorOutput,
		apiClient:     apiClient,
		prompt:        cfg.Prompt,
		showBanner:    cfg.ShowBanner,
		historyPath:   historyPath,
		aliasRegistry: aliasRegistry,
	}

	return repl, nil
}

// Start begins the interactive REPL loop.
// It continues until the user exits or an error occurs.
func (r *REPL) Start() error {
	// Display banner
	if r.showBanner {
		r.printBanner()
	}

	// Initialize readline with history
	rl, err := r.initReadline()
	if err != nil {
		// Fall back to basic scanner if readline fails
		return r.startWithScanner()
	}
	defer rl.Close()
	r.rl = rl

	// Handle Ctrl+C gracefully
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	// Start signal handler goroutine
	go func() {
		sig := <-sigChan
		if sig == os.Interrupt {
			// Ctrl+C - exit cleanly
			fmt.Fprintf(r.output, "\nExiting...\n")
		}
		r.cancel()
	}()

	// Create context for commands
	cmdCtx := &Context{
		APIClient:   r.apiClient,
		Output:      r.output,
		ErrorOutput: r.errOut,
		Session:     r.session,
		GoContext:   r.ctx,
	}

	// Main REPL loop
	for {
		// Check for cancellation
		select {
		case <-r.ctx.Done():
			return nil
		default:
		}

		// Read input using readline (supports arrow keys, history, Ctrl+R search)
		input, err := rl.Readline()
		if err != nil {
			if err == readline.ErrInterrupt {
				// Ctrl+C pressed
				continue
			}
			// Ctrl+D (EOF)
			fmt.Fprintf(r.output, "\nExiting...\n")
			return nil
		}

		// Add to history if non-empty
		if input != "" {
			rl.SaveHistory(input)
		}

		// Execute command
		err = r.dispatcher.Execute(cmdCtx, input)
		if err != nil {
			if err == ErrExit {
				// User typed /exit or /quit
				return nil
			}
			// Print error but continue
			fmt.Fprintf(r.errOut, "Error: %v\n", err)
		}
	}
}

// initReadline initializes the readline instance with history support and tab completion
func (r *REPL) initReadline() (*readline.Instance, error) {
	// Create context for completions
	cmdCtx := &Context{
		APIClient:   r.apiClient,
		Output:      r.output,
		ErrorOutput: r.errOut,
		Session:     r.session,
		GoContext:   r.ctx,
	}

	// Create the tab completer
	completer := NewTabCompleter(r.dispatcher, cmdCtx)

	rl, err := readline.NewEx(&readline.Config{
		Prompt:            r.prompt,
		HistoryFile:       r.historyPath,
		HistorySearchFold: true,
		InterruptPrompt:   "^C",
		EOFPrompt:         "exit",
		AutoComplete:      completer,
		// Enable Vi or Emacs key bindings; readline defaults to Emacs
	})
	return rl, err
}

// startWithScanner falls back to basic scanner mode if readline initialization fails
func (r *REPL) startWithScanner() error {
	// Display banner
	if r.showBanner {
		r.printBanner()
	}

	// Handle Ctrl+C gracefully
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	// Start signal handler goroutine
	go func() {
		sig := <-sigChan
		if sig == os.Interrupt {
			// Ctrl+C - exit cleanly
			fmt.Fprintf(r.output, "\nExiting...\n")
		}
		r.cancel()
	}()

	// Create context for commands
	cmdCtx := &Context{
		APIClient:   r.apiClient,
		Output:      r.output,
		ErrorOutput: r.errOut,
		Session:     r.session,
		GoContext:   r.ctx,
	}

	// Create buffered reader
	scanner := bufio.NewScanner(r.input)
	scanner.Buffer(make([]byte, 1024), 1024*1024) // Allow up to 1MB lines

	// Main REPL loop
	for {
		// Check for cancellation
		select {
		case <-r.ctx.Done():
			return nil
		default:
		}

		// Display prompt
		fmt.Fprint(r.output, r.prompt)

		// Read input
		if !scanner.Scan() {
			// EOF or error
			if scanner.Err() != nil {
				return scanner.Err()
			}
			// Ctrl+D (EOF)
			fmt.Fprintf(r.output, "\nExiting...\n")
			return nil
		}

		input := scanner.Text()

		// Execute command
		err := r.dispatcher.Execute(cmdCtx, input)
		if err != nil {
			if err == ErrExit {
				// User typed /exit or /quit
				return nil
			}
			// Print error but continue
			fmt.Fprintf(r.errOut, "Error: %v\n", err)
		}
	}
}

// Stop terminates the REPL.
func (r *REPL) Stop() {
	r.cancel()
}

// printBanner displays a welcome message.
func (r *REPL) printBanner() {
	banner := `
╔════════════════════════════════════════════════════════════╗
║           FLIP2 Multi-Agent Coordination Shell             ║
║                  Interactive REPL Mode                     ║
╚════════════════════════════════════════════════════════════╝

Type /help for a list of available commands
Type /help <command> for detailed help about a command
Type /exit or Ctrl+D to exit

`
	fmt.Fprint(r.output, banner)
}

// StartREPL is the main entry point for starting an interactive REPL session.
// It's called from the main CLI entry point when no args or --interactive flag is given.
func StartREPL(apiUrl, apiKey, authToken string) error {
	cfg := Config{
		APIUrl:     apiUrl,
		APIKey:     apiKey,
		AuthToken:  authToken,
		ShowBanner: true,
		Prompt:     "flip2> ",
	}

	repl, err := New(cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize REPL: %w", err)
	}

	return repl.Start()
}

// Interactive runs a simple interactive shell that reads commands from stdin
// and dispatches them to registered handlers.
// This is a basic version for testing/simple use cases.
// For full-featured interactive mode with history/completion, use StartREPL.
func Interactive(apiURL, apiKey, authToken string) error {
	return StartREPL(apiURL, apiKey, authToken)
}

// =============================================================================
// TAB COMPLETION
// =============================================================================

// TabCompleter implements readline.AutoCompleter interface for FLIP2 REPL.
// It provides tab completion for commands, arguments, agent IDs, task IDs, etc.
type TabCompleter struct {
	dispatcher *Dispatcher
	ctx        *Context
}

// NewTabCompleter creates a new tab completer.
func NewTabCompleter(dispatcher *Dispatcher, ctx *Context) *TabCompleter {
	return &TabCompleter{
		dispatcher: dispatcher,
		ctx:        ctx,
	}
}

// Do implements the readline.AutoCompleter interface.
// It returns completion candidates and the length of the shared prefix.
//
// The readline library passes the full line and the current cursor position,
// then expects back an array of suffixes that would complete each candidate,
// plus the length of characters that are already matched.
//
// For example, if the user types "/se" and candidates are "send" and "set":
//   Do([]rune("/se"), 3) => [[],[]], 3
//   The user sees suggestions "send" and "set", selecting one appends "nd" or "t"
func (tc *TabCompleter) Do(line []rune, pos int) ([][]rune, int) {
	// Convert rune array to string and truncate to cursor position
	input := string(line[:pos])

	// Get completions from dispatcher
	completions := tc.dispatcher.Complete(tc.ctx, input, pos)
	if len(completions) == 0 {
		return [][]rune{}, 0
	}

	// Find the longest common prefix among all completions
	commonPrefix := findCommonPrefix(completions)

	// Convert completions to readline format (suffixes to append)
	candidates := make([][]rune, len(completions))
	for i, comp := range completions {
		// Remove the common prefix to get the suffix to append
		suffix := strings.TrimPrefix(comp.Value, commonPrefix)
		candidates[i] = []rune(suffix)
	}

	// Sort candidates for consistent ordering
	sort.Slice(candidates, func(i, j int) bool {
		return string(candidates[i]) < string(candidates[j])
	})

	return candidates, len(commonPrefix)
}

// findCommonPrefix finds the longest common prefix among completion values.
func findCommonPrefix(completions []Completion) string {
	if len(completions) == 0 {
		return ""
	}
	if len(completions) == 1 {
		return completions[0].Value
	}

	// Get the length of the shortest completion
	minLen := len(completions[0].Value)
	for _, c := range completions[1:] {
		if len(c.Value) < minLen {
			minLen = len(c.Value)
		}
	}

	// Find common prefix character by character
	for i := 0; i < minLen; i++ {
		char := completions[0].Value[i]
		for _, c := range completions[1:] {
			if c.Value[i] != char {
				return completions[0].Value[:i]
			}
		}
	}

	return completions[0].Value[:minLen]
}
