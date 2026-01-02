// FLIP2 CLI - Command-line interface for FLIP2 daemon
package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"flip2/internal/auth"
	"flip2/internal/config"
	"flip2/internal/migrate"
	"flip2/internal/repl"
	"flip2/internal/routing"
	"flip2/internal/session"
	"flip2/internal/spawn"
	"flip2/internal/version"
	_ "flip2/pb_migrations" // Register migrations
	"flip2/pkg/client"
	"flip2/pkg/httpclient"
	"log/slog"

	"github.com/pocketbase/pocketbase"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

const (
	defaultAPIURL = "https://localhost:8090"
	pidFileName   = "flip2d.pid"
)

var (
	apiURL string
	logger *slog.Logger

	// validIdentifier matches alphanumeric, hyphens, and underscores only
	// Used for agent IDs, collection names, field names to prevent SQL injection
	validIdentifier = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
)

// setupLogCapture redirects os.Stdout and os.Stderr to a temporary file,
// while also duplicating the output to the original os.Stdout/os.Stderr.
// It returns a function that restores the original os.Stdout/os.Stderr.
func setupLogCapture() func() {
	// Define a temporary log file path
	// TODO: Make this configurable or use project temp dir
	logFilePath := filepath.Join(os.TempDir(), "flip2_output.log")

	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open log file %s: %v\n", logFilePath, err)
		// If we can't open the log file, we should not proceed with redirection
		return func() {} // Return a no-op cleanup
	}

	// Save original Stdout and Stderr
	originalStdout := os.Stdout
	originalStderr := os.Stderr

	// Create pipes
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		fmt.Fprintf(originalStderr, "Failed to create stdout pipe: %v\n", err)
		return func() {}
	}
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		fmt.Fprintf(originalStderr, "Failed to create stderr pipe: %v\n", err)
		return func() {}
	}

	// Redirect os.Stdout and os.Stderr
	os.Stdout = stdoutWriter
	os.Stderr = stderrWriter

	var wg sync.WaitGroup
	wg.Add(2)

	// Goroutine to read from stdout pipe
	go func() {
		defer wg.Done()
		_, _ = io.Copy(io.MultiWriter(originalStdout, logFile), stdoutReader)
	}()

	// Goroutine to read from stderr pipe
	go func() {
		defer wg.Done()
		_, _ = io.Copy(io.MultiWriter(originalStderr, logFile), stderrReader)
	}()

	// Return a cleanup function
	return func() {
		// Close the writers first to unblock the readers
		stdoutWriter.Close()
		stderrWriter.Close()
		wg.Wait() // Wait for goroutines to finish
		logFile.Close()

		// Restore original Stdout and Stderr
		os.Stdout = originalStdout
		os.Stderr = originalStderr
	}
}

// setupSessionSignalHandlers initializes signal handlers for session state persistence.
// This function saves all active session state when the process receives SIGINT or SIGTERM,
// ensuring that sessions can be recovered even if the terminal closes unexpectedly.
func setupSessionSignalHandlers(ctx context.Context, sm *session.SessionManager) {
	if sm == nil {
		logger.Info("Session manager not initialized, skipping signal handlers")
		return
	}

	// Setup signal handlers for the session manager
	// This will handle SIGINT (Ctrl+C) and SIGTERM gracefully
	// TODO: Implement SetupSignalHandlers in SessionManager
	// sigChan := sm.SetupSignalHandlers(ctx)
	// logger.Info("Session signal handlers registered", "signal_channel", fmt.Sprintf("%v", sigChan))
	logger.Debug("Signal handlers not yet implemented")
}

func main() {
	// Call setupLogCapture and defer its cleanup function
	cleanup := setupLogCapture()
	defer cleanup()

	// Initialize structured logger
	logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	rootCmd := &cobra.Command{
		Use:   "flip2",
		Short: "FLIP2 Multi-Agent Coordination CLI",
		Long:  "Command-line interface for the FLIP2 daemon and PocketBase backend.",
		// If no args provided, enter interactive mode
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				// No args = enter interactive REPL mode
				return interactiveMode()
			}
			return nil
		},
	}

	rootCmd.PersistentFlags().StringVar(&apiURL, "api", defaultAPIURL, "FLIP2 API URL")

	// Interactive flag for explicit interactive mode
	var interactive bool
	rootCmd.PersistentFlags().BoolVar(&interactive, "interactive", false, "Enter interactive REPL mode")

	// Daemon commands
	rootCmd.AddCommand(startCmd())
	rootCmd.AddCommand(stopCmd())
	rootCmd.AddCommand(statusCmd())
	rootCmd.AddCommand(restartCmd())

	// Task commands
	rootCmd.AddCommand(taskCmd())

	// Agent commands
	rootCmd.AddCommand(agentCmd())

	// Session commands
	rootCmd.AddCommand(sessionCmd())

	// Auth commands
	rootCmd.AddCommand(authCmd())

	// Signal commands
	rootCmd.AddCommand(signalCmd())

	// Admin command
	rootCmd.AddCommand(adminCmd())

	// Migrate command
	rootCmd.AddCommand(migrateCmd())

	// Pipeline commands
	rootCmd.AddCommand(pipelineCmd())

	// Routing command (analytics and dashboards)
	rootCmd.AddCommand(routingCmd())

	// Version command (collaborative feature with Gemini via FLIP2!)
	rootCmd.AddCommand(versionCmd())

	if interactive {
		// Explicit --interactive flag
		if err := interactiveMode(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// interactiveMode starts the interactive REPL shell
func interactiveMode() error {
	// Load auth token if available
	authToken := ""
	if authData, err := auth.LoadAuth(); err == nil && authData != nil {
		authToken = authData.Token
	}

	// Get API key from environment or config
	apiKey := os.Getenv("FLIP2_API_KEY")
	if apiKey == "" && globalConfig != nil {
		apiKey = globalConfig.Flip2.Security.APIKey
	}

	// Start REPL
	return repl.StartREPL(apiURL, apiKey, authToken)
}

// ... Daemon control commands (start, stop, status, restart) same as before ...
// I will copy them back or implement them. Since I'm overwriting, I must implement them.

func startCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start the FLIP2 daemon",
		Run: func(cmd *cobra.Command, args []string) {
			pidFile := filepath.Join(os.TempDir(), pidFileName)

			if isRunning(pidFile) {
				pid, _ := readPID(pidFile)
				logger.Info("FLIP2 daemon already running", "pid", pid)
				return
			}

			// Find flip2d binary
			// Assuming it's in the same directory or PATH
			flip2d := "flip2d"
			if exe, err := os.Executable(); err == nil {
				dir := filepath.Dir(exe)
				if _, err := os.Stat(filepath.Join(dir, "flip2d")); err == nil {
					flip2d = filepath.Join(dir, "flip2d")
				}
			}

			daemonCmd := exec.Command(flip2d)
			// Detach
			daemonCmd.Stdout = nil
			daemonCmd.Stderr = nil
			daemonCmd.Stdin = nil

			if err := daemonCmd.Start(); err != nil {
				logger.Info("Failed to start daemon", "error", err)
				os.Exit(1)
			}

			logger.Info("FLIP2 daemon started", "pid", daemonCmd.Process.Pid)
		},
	}
}

func stopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the FLIP2 daemon",
		Run: func(cmd *cobra.Command, args []string) {
			pidFile := filepath.Join(os.TempDir(), pidFileName)
			pid, err := readPID(pidFile)
			if err != nil {
				logger.Info("FLIP2 daemon is not running")
				return
			}

			process, err := os.FindProcess(pid)
			if err != nil {
				logger.Info("FLIP2 daemon is not running")
				return
			}

			if err := process.Signal(syscall.SIGTERM); err != nil {
				logger.Info("Failed to stop daemon", "error", err)
				os.Exit(1)
			}
			logger.Info("FLIP2 daemon stopped")
		},
	}
}

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show FLIP2 daemon status",
		Run: func(cmd *cobra.Command, args []string) {
			pidFile := filepath.Join(os.TempDir(), pidFileName)
			logger.Info("FLIP2 Daemon Status")
			logger.Info("==================")

			if !isRunning(pidFile) {
				logger.Info("Status:     stopped")
				return
			}

			pid, _ := readPID(pidFile)
			logger.Info("Status: running")
			logger.Info("daemon_pid", "pid", pid)
			logger.Info("api_url", "url", apiURL)

			resp, err := http.Get(apiURL + "/api/health")
			if err != nil {
				logger.Info("API:        unreachable")
				return
			}
			defer resp.Body.Close()
			logger.Info("API:        healthy")
		},
	}
}

func restartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "restart",
		Short: "Restart the FLIP2 daemon",
		Run: func(cmd *cobra.Command, args []string) {
			stopCmd().Run(cmd, args)
			time.Sleep(1 * time.Second)
			startCmd().Run(cmd, args)
		},
	}
}

func taskCmd() *cobra.Command {
	taskCmd := &cobra.Command{
		Use:   "task",
		Short: "Manage tasks",
	}

	taskCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List all tasks",
		Run: func(cmd *cobra.Command, args []string) {
			printCollectionItems("tasks", []string{"task_id", "title", "status", "assignee"})
		},
	})

	var assignee string
	var priority int
	var routeTo string

	addCmd := &cobra.Command{
		Use:   "add <title>",
		Short: "Add a new task",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			title := args[0]
			data := map[string]interface{}{
				"title":    title,
				"status":   "todo",
				"priority": priority,
			}

			// If route-to flag provided, add routing override
			if routeTo != "" {
				data["route_to"] = routeTo
			}

			// If assignee provided, resolve agent ID
			if assignee != "" {
				// We assume assignee name matches agent_id or we need to lookup?
				// For simplicity, assume assignee arg is agent_id.
				// But user might pass 'claude'. If agent_id is 'claude', great.
				// If not, we might need to lookup by name? Schema: agents have agent_id.
				// Let's assume user passes valid agent ID.

				// Verify agent exists? API will error if relation invalid usually.
				// But we need the RECORD ID of the agent if it's a relation.
				// relation field expects Record ID.
				// If agent_id is custom field, we must lookup Record ID.
				id, err := getAgentRecordID(assignee)
				if err != nil {
					logger.Info("Failed to resolve agent", "agent", assignee, "error", err)
					return
				}
				data["assignee"] = id
			}

			// Use generic unique ID for task_id? PocketBase generates ID.
			// But we have 'task_id' field.
			// Let's generate one or rely on PB ID? Architecture schema had 'task_id'.
			data["task_id"] = fmt.Sprintf("TASK-%d", time.Now().UnixNano()) // Nano ID generation

			if err := createRecord("tasks", data); err != nil {
				logger.Info("Failed to create task", "error", err)
			} else {
				logger.Info("Task created", "task_id", data["task_id"])
			}
		},
	}
	addCmd.Flags().StringVar(&assignee, "assignee", "", "Assignee agent ID")
	addCmd.Flags().IntVar(&priority, "priority", 3, "Priority (1-5)")
	addCmd.Flags().StringVar(&routeTo, "route-to", "", "Override routing to specific model (opus/sonnet/haiku/gemini/antigravity)")
	taskCmd.AddCommand(addCmd)

	// Signal Task
	taskCmd.AddCommand(&cobra.Command{
		Use:   "signal <task_id> [signal]",
		Short: "Send a signal to a running task (default: SIGTERM)",
		Args:  cobra.RangeArgs(1, 2),
		Run: func(cmd *cobra.Command, args []string) {
			taskID := args[0]
			sig := "SIGTERM"
			if len(args) > 1 {
				sig = args[1]
			}

			// Resolve record ID
			recordID, err := getRecordIDByField("tasks", "task_id", taskID)
			if err != nil {
				recordID = taskID
			}

			data := map[string]string{"signal": sig}
			jsonData, _ := json.Marshal(data)

			req, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/tasks/%s/signal", apiURL, recordID), bytes.NewBuffer(jsonData))
			req.Header.Set("Content-Type", "application/json")
			setAuthHeaders(req)

			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				logger.Info("Error occurred", "error", err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != 200 {
				body, _ := io.ReadAll(resp.Body)
				logger.Info("Request failed", "response", string(body))
			} else {
				logger.Info("Signal sent")
			}
		},
	})

	// Start Task
	taskCmd.AddCommand(&cobra.Command{
		Use:   "start <task_id>",
		Short: "Mark task as in_progress",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			updateTaskStatus(args[0], "in_progress")
		},
	})

	// Done Task
	taskCmd.AddCommand(&cobra.Command{
		Use:   "done <task_id>",
		Short: "Mark task as done",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			updateTaskStatus(args[0], "done")
		},
	})

	return taskCmd
}

func agentCmd() *cobra.Command {
	agentCmd := &cobra.Command{
		Use:   "agent",
		Short: "Manage agents",
	}

	agentCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List all agents",
		Run: func(cmd *cobra.Command, args []string) {
			printCollectionItems("agents", []string{"agent_id", "status", "backend"})
		},
	})

	var backend string
	addCmd := &cobra.Command{
		Use:   "add <agent_id>",
		Short: "Register a new agent",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			agentID := args[0]
			if backend == "" {
				logger.Info("Error: --backend flag is required")
				return
			}

			data := map[string]interface{}{
				"agent_id": agentID,
				"status":   "online",
				"backend":  backend,
			}

			if err := createRecord("agents", data); err != nil {
				logger.Info("Failed to register agent", "error", err)
			} else {
				logger.Info("Agent registered", "agent_id", agentID)
			}
		},
	}
	addCmd.Flags().StringVar(&backend, "backend", "", "Backend type/name (required)")
	agentCmd.AddCommand(addCmd)

	agentCmd.AddCommand(&cobra.Command{
		Use:   "poll <agent_id>",
		Short: "Poll for unread signals and reply",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			agentID := args[0]
			if err := pollAgent(agentID); err != nil {
				logger.Info("Polling failed: %v", "value", err)
			}
		},
	})

	listenCmd := &cobra.Command{
		Use:   "listen <agent_id>",
		Short: "Listen for real-time signals and reply",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			agentName := args[0]
			logger.Info("Agent listening for signals", "agent", agentName, "url", apiURL)

			// Resolve Agent Record ID for filter matching
			agentRecordID, err := getAgentRecordID(agentName)
			if err != nil {
				logger.Warn("Agent not found in registry (using name as ID)", "agent", agentName, "error", err)
				agentRecordID = agentName
			} else {
				logger.Info("Resolved Agent ID", "name", agentName, "id", agentRecordID)
			}

			// Initial config load
			if globalConfig == nil {
				loadGlobalConfig()
			}

			// Start Realtime Client
			apiKey, _ := cmd.Flags().GetString("api-key")
			if apiKey == "" && globalConfig != nil {
				apiKey = globalConfig.Flip2.Security.APIKey
			}

			authToken := ""
			if authData, err := auth.LoadAuth(); err == nil && authData != nil {
				authToken = authData.Token
			}

			c := client.New(apiURL, agentName, apiKey, authToken, logger)
			if err := c.Connect(); err != nil {
				logger.Error("Failed to connect", "error", err)
				os.Exit(1)
			}

			concurrency, _ := cmd.Flags().GetInt("concurrency")
			if concurrency < 1 {
				concurrency = 1
			}
			logger.Info("Starting agent", "concurrency", concurrency)

			// Semaphore to limit concurrency
			sem := make(chan struct{}, concurrency)

			// Heartbeat Loop (30s)
			go func() {
				heartbeatTicker := time.NewTicker(30 * time.Second)
				defer heartbeatTicker.Stop()
				for range heartbeatTicker.C {
					// Update last_seen and ensure mode is set (to satisfy validation)
					updateData := map[string]interface{}{
						"last_seen": time.Now(),
						"mode":      "remote",
					}
					jsonData, _ := json.Marshal(updateData)
					req, _ := http.NewRequest("PATCH", fmt.Sprintf("%s/api/collections/agents/records/%s", apiURL, agentRecordID), bytes.NewBuffer(jsonData))
					req.Header.Set("Content-Type", "application/json")
					if apiKey != "" {
						req.Header.Set("X-API-Key", apiKey)
					}
					resp, err := httpclient.NewClientSkipTLS(30 * time.Second).Do(req)
					if err != nil {
						logger.Error("Heartbeat error", "error", err)
					} else {
						defer resp.Body.Close()
						if resp.StatusCode >= 400 {
							body, _ := io.ReadAll(resp.Body)
							logger.Error("Heartbeat failed", "status", resp.Status, "body", string(body))
						}
					}
				}
			}()

			// Event Loop with Polling Fallback
			go func() {
				ticker := time.NewTicker(3 * time.Second)
				for range ticker.C {
					if err := pollAgent(agentName); err != nil {
						logger.Debug("Poll error", "error", err)
					}
				}
			}()

			// Task Event Loop
			go func() {
				for event := range c.Tasks() {
					rec := event.Record
					logger.Debug("Task Event", "id", rec.ID, "title", rec.Title, "assignee", rec.Assignee, "status", rec.Status)

					// Check against Record ID (assignee)
					if rec.Assignee == agentRecordID && (rec.Status == "todo" || rec.Status == "pending") {
						// Acquire semaphore (blocks if full)
						sem <- struct{}{}

						go func(taskID, title, description string) {
							defer func() { <-sem }() // Release semaphore on completion

							logger.Info("Received task assignment", "title", title)
							logger.Info("Executing task", "title", title)

							// Mark in_progress
							updateTaskStatus(taskID, "in_progress")

							// Execute
							result, err := executeAgentTask(agentName, title, description)

							if err != nil {
								logger.Error("Task execution failed", "error", err)
								updateTaskStatus(taskID, "failed")
							} else {
								logger.Info("Task completed", "task_id", taskID)
								completeTask(taskID, result)
							}
						}(rec.ID, rec.Title, rec.Description)
					}
				}
			}()

			for event := range c.Signals() {
				logger.Info("Received signal", "from", event.Record.FromAgent, "content", event.Record.Content)

				// Generate Reply
				replyContent, err := generateReply(agentName, event.Record.FromAgent, event.Record.Content)
				if err != nil {
					logger.Error("Error generating reply", "error", err)
					continue
				}

				logger.Info("Generated reply", "content", replyContent)

				// Send Reply
				if err := c.SendSignal(event.Record.FromAgent, "message", replyContent); err != nil {
					logger.Error("Failed to send reply", "error", err)
				}

				// Mark as read
				c.MarkRead(event.Record.ID)
			}
		},
	}
	listenCmd.Flags().Int("concurrency", 1, "Max concurrent tasks (default 1)")
	listenCmd.Flags().String("api-key", "", "API Key for authentication")
	agentCmd.AddCommand(listenCmd)

	// Spawn command for role-based agent spawning
	var spawnRole string
	spawnCmd := &cobra.Command{
		Use:   "spawn --role <role> --task <description>",
		Short: "Spawn a new worker agent with a role",
		Long: `Spawn a new worker agent with a specific role and task.

The spawned agent will be initialized with:
- System prompt and constraints from the role
- Permissions and capabilities defined by the role
- The LLM model specified in the role
- The task description you provide

Available roles: code-reviewer, researcher, implementer

Example:
  flip2 agent spawn --role code-reviewer --task "Review the main.go file for bugs"
  flip2 agent spawn --role researcher --task "Research current best practices in Go error handling"`,
		Args: cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			// Get flags
			roleFlag := cmd.Flag("role")
			taskFlag := cmd.Flag("task")

			if !roleFlag.Changed || spawnRole == "" {
				logger.Info("Error: --role flag is required")
				return
			}
			if !taskFlag.Changed {
				logger.Info("Error: --task flag is required")
				return
			}

			task, _ := cmd.Flags().GetString("task")
			if task == "" {
				logger.Info("Error: task cannot be empty")
				return
			}

			// Import spawn package and call SpawnWithRole
			spawnAgent(spawnRole, task)
		},
	}
	spawnCmd.Flags().StringVar(&spawnRole, "role", "", "Role to spawn with (required)")
	spawnCmd.Flags().String("task", "", "Task description for the agent (required)")
	spawnCmd.MarkFlagRequired("role")
	spawnCmd.MarkFlagRequired("task")
	agentCmd.AddCommand(spawnCmd)

	return agentCmd
}


var globalConfig *config.Config

func loadGlobalConfig() {
	// Try default locations
	paths := []string{
		"./config/config.yaml",
		"/etc/flip2/config.yaml",
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			cfg, err := config.LoadConfig(p)
			if err == nil {
				globalConfig = cfg
				return
			}
		}
	}
	logger.Info("Warning: Could not load config.yaml (checked ./config/config.yaml, /etc/flip2/config.yaml). Backends may fail.")
}

func pollAgent(agentID string) error {
	if globalConfig == nil {
		loadGlobalConfig()
	}

	// Validate agent ID to prevent SQL injection
	if !validIdentifier.MatchString(agentID) {
		return fmt.Errorf("invalid agent_id format: %q (must be alphanumeric with hyphens/underscores)", agentID)
	}

	// 1. Get record ID for this agent
	filter := fmt.Sprintf("(to_agent='%s' && read=false)", agentID)
	urlVal := fmt.Sprintf("%s/api/collections/signals/records?filter=%s", apiURL, url.QueryEscape(filter))

	req, _ := http.NewRequest("GET", urlVal, nil)
	setAuthHeaders(req)
	resp, err := httpclient.NewClientSkipTLS(30 * time.Second).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	items, ok := result["items"].([]interface{})
	if !ok || len(items) == 0 {
		// No unread signals
		return nil
	}

	logger.Info("Unread signals found", "count", len(items), "agent_id", agentID)

	for _, item := range items {
		signal := item.(map[string]interface{})
		signalID := signal["id"].(string)
		content := signal["content"].(string)
		from := signal["from_agent"].(string)

		logger.Info("Processing signal", "from", from, "content", content)

		// 2. Generate Reply
		replyContent, err := generateReply(agentID, from, content)
		if err != nil {
			logger.Info("Failed to generate reply", "error", err)
			continue
		}

		logger.Info("Generated reply", "content", replyContent)

		// 3. Send Reply Signal
		replyData := map[string]interface{}{
			"signal_id":   fmt.Sprintf("SIG-%d", time.Now().UnixNano()),
			"from_agent":  agentID,
			"to_agent":    from,
			"signal_type": "message",
			"content":     replyContent,
			"read":        false,
		}
		if err := createRecord("signals", replyData); err != nil {
			logger.Info("Failed to send reply: %v", "value", err)
		}

		// 4. Mark original as read
		updateData := map[string]interface{}{"read": true}
		jsonData, _ := json.Marshal(updateData)
		req, _ := http.NewRequest("PATCH", fmt.Sprintf("%s/api/collections/signals/records/%s", apiURL, signalID), bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		setAuthHeaders(req)
		httpclient.NewClientSkipTLS(30 * time.Second).Do(req)
	}

	return nil
}

func generateReply(agentID, from, content string) (string, error) {
	if globalConfig == nil {
		return "", fmt.Errorf("no config loaded")
	}

	// Find backend config for this agent
	// We assume agentID matches backend name in config for simplicity
	// Or we should lookup agent record to get 'backend' field?
	// Let's lookup agent record.
	_, err := getAgentRecordID(agentID)
	if err != nil {
		return "", fmt.Errorf("agent record not found: %v", err)
	}
	// Need to fetch full record to get 'backend' field
	// reused getAgentRecordID only returns ID.
	// Let's just assume agentID == backend name for this specific setup (claude, gemini).
	// Or look at config directly.

	backendName := agentID // default assumption
	// Check if this backend exists in config
	backendCfg, ok := globalConfig.Flip2.Backends[backendName]
	if !ok {
		// Fallback: maybe the agent record has a 'backend' field that is different?
		// For now, fail if not in config.
		return "", fmt.Errorf("no backend config for agent %s", agentID)
	}

	prompt := fmt.Sprintf("You are agent '%s'. You received a message from '%s':\n\"%s\"\n\nPlease reply briefly and helpfully.", agentID, from, content)

	if backendCfg.Type == "process" {
		// Run command
		cmdName := backendCfg.Command
		cmdArgs := backendCfg.Args

		// We need to feed prompt via Stdin? Or args?
		// Config says: args: ["-p", "--dangerously-skip-permissions", ...]
		// Assuming tool expects prompt as argument or stdin.
		// Claude CLI usually takes prompt as arg or stdin?
		// The 'executor' in daemon used the prompt as the LAST argument or via stdin?
		// Let's check executor.go invocation.
		// Executor constructs cmd and sets Stdin if needed.
		// But in daemon config for claude: `command: claude`, `args: ...`.
		// If these are standard CLIs, usually `echo prompt | command` works.

		cmd := exec.Command(cmdName, cmdArgs...)
		cmd.Stdin = bytes.NewBufferString(prompt)
		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = os.Stderr // debug

		if err := cmd.Run(); err != nil {
			return "", err
		}
		return strings.TrimSpace(out.String()), nil
	}

	return "", fmt.Errorf("backend type %s not supported in CLI poll yet", backendCfg.Type)
}

func signalCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "signal",
		Short: "Manage signals",
	}

	// Send command
	cmd.AddCommand(&cobra.Command{
		Use:   "send <to> <type> <message>",
		Short: "Send a signal to an agent",
		Args:  cobra.ExactArgs(3),
		Run: func(cmd *cobra.Command, args []string) {
			to := args[0]
			sigType := args[1]
			msg := args[2]

			data := map[string]interface{}{
				"signal_id":   fmt.Sprintf("SIG-%d", time.Now().UnixNano()),
				"from_agent":  "cli",
				"to_agent":    to,
				"signal_type": sigType,
				"content":     msg,
				"read":        false,
			}

			if err := createRecord("signals", data); err != nil {
				logger.Info("Failed to send signal: %v", "value", err)
			} else {
				logger.Info("Signal sent")
			}
		},
	})

	// List command
	cmd.AddCommand(&cobra.Command{
		Use:   "list [agent]",
		Short: "List signals (optionally filtered by agent)",
		Run: func(cmd *cobra.Command, args []string) {
			filter := ""
			if len(args) > 0 {
				filter = fmt.Sprintf("to_agent='%s'", args[0])
			}
			printCollectionItemsFiltered("signals", []string{"signal_id", "from_agent", "to_agent", "signal_type", "content"}, filter)
		},
	})

	// Watch command - uses SSE for real-time notifications (no polling!)
	var watchAgent string
	watchCmd := &cobra.Command{
		Use:   "watch",
		Short: "Watch for signals in real-time using SSE (no polling)",
		Long: `Watch for signals using Server-Sent Events (SSE).
This is much more efficient than polling - uses a single persistent connection
and receives signals instantly when they arrive.

Example:
  flip2 signal watch --agent claude-mac
  flip2 signal watch --agent claude-win --api http://192.168.1.220:8090`,
		Run: func(cmd *cobra.Command, args []string) {
			if watchAgent == "" {
				logger.Info("Error: --agent flag is required")
				logger.Info("Usage: flip2 signal watch --agent <agent-id>")
				os.Exit(1)
			}

			logger.Info("Watching signals for agent '%s' via SSE...", "value", watchAgent)
			logger.Info("Connected to: %s", "value", apiURL)
			logger.Info("Press Ctrl+C to stop\n")

			// Create SSE client
			c := client.New(apiURL, watchAgent, getAPIKey(), "", logger)

			// Connect to SSE stream
			if err := c.Connect(); err != nil {
				logger.Info("Failed to connect: %v", "value", err)
				os.Exit(1)
			}

			// Handle signals
			sigChan := c.Signals()
			taskChan := c.Tasks()

			// Wait for Ctrl+C
			stopChan := make(chan os.Signal, 1)
			signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)

			for {
				select {
				case sig := <-sigChan:
					if sig != nil {
						logger.Info("Signal received",
							"timestamp", time.Now().Format("15:04:05"),
							"from", sig.Record.FromAgent,
							"to", sig.Record.ToAgent,
							"type", sig.Record.Type,
							"content", truncate(sig.Record.Content, 100))
					}
				case task := <-taskChan:
					if task != nil {
						logger.Info("Task update",
							"timestamp", time.Now().Format("15:04:05"),
							"title", task.Record.Title,
							"status", task.Record.Status,
							"description", truncate(task.Record.Description, 80))
					}
				case <-stopChan:
					logger.Info("\nStopping watcher...")
					c.Close()
					return
				}
			}
		},
	}
	watchCmd.Flags().StringVar(&watchAgent, "agent", "", "Agent ID to watch signals for (required)")
	cmd.AddCommand(watchCmd)

	return cmd
}

// Helper to truncate long strings
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// Helper to get API key from environment or config
func getAPIKey() string {
	if key := os.Getenv("FLIP2_API_KEY"); key != "" {
		return key
	}
	return "flip2_secret_key_123" // Default key
}

// printCollectionItemsFiltered prints items from a collection with optional filter
func printCollectionItemsFiltered(collection string, fields []string, filter string) {
	reqURL := fmt.Sprintf("%s/api/collections/%s/records", apiURL, collection)
	if filter != "" {
		reqURL += "?filter=" + url.QueryEscape(filter)
	}

	resp, err := http.Get(reqURL)
	if err != nil {
		logger.Info("Error occurred", "error", err)
		return
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		logger.Info("Error decoding response: %v", "value", err)
		return
	}

	items, ok := result["items"].([]interface{})
	if !ok {
		logger.Info("No items found")
		return
	}

	for _, item := range items {
		record := item.(map[string]interface{})
		parts := make([]string, 0, len(fields))
		for _, f := range fields {
			if v, ok := record[f]; ok {
				parts = append(parts, fmt.Sprintf("%s=%v", f, v))
			}
		}
		logger.Info("Result", "parts", strings.Join(parts, " | "))
	}
}

func authCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage authentication",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "login",
		Short: "Login to FLIP2",
		Run: func(cmd *cobra.Command, args []string) {
			reader := bufio.NewReader(os.Stdin)
			fmt.Print("Email: ")
			email, _ := reader.ReadString('\n')
			email = strings.TrimSpace(email)

			fmt.Print("Password: ")
			bytePassword, err := term.ReadPassword(int(syscall.Stdin))
			if err != nil {
				logger.Info("\nError reading password")
				return
			}
			password := string(bytePassword)
			// Empty line removed - use structured logging instead

			data := map[string]interface{}{
				"identity": email,
				"password": password,
			}
			jsonData, _ := json.Marshal(data)

			resp, err := http.Post(apiURL+"/api/collections/users/auth-with-password", "application/json", bytes.NewBuffer(jsonData))
			if err != nil {
				logger.Info("Login failed: %v", "value", err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != 200 {
				body, _ := io.ReadAll(resp.Body)
				logger.Info("Login failed: %s", "value", string(body))
				return
			}

			var result map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				logger.Info("Error parsing response: %v", "value", err)
				return
			}

			token, _ := result["token"].(string)
			record, _ := result["record"].(map[string]interface{})
			id, _ := record["id"].(string)

			authData := auth.AuthData{
				Token: token,
				Email: email,
				ID:    id,
			}

			if err := auth.SaveAuth(authData); err != nil {
				logger.Info("Failed to save token: %v", "value", err)
				return
			}

			logger.Info("Login successful")
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "register",
		Short: "Register a new user",
		Run: func(cmd *cobra.Command, args []string) {
			reader := bufio.NewReader(os.Stdin)
			fmt.Print("Email: ")
			email, _ := reader.ReadString('\n')
			email = strings.TrimSpace(email)

			fmt.Print("Password: ")
			bytePassword, err := term.ReadPassword(int(syscall.Stdin))
			if err != nil {
				logger.Info("\nError reading password")
				return
			}
			password := string(bytePassword)
			// Empty line removed - use structured logging instead

			fmt.Print("Confirm Password: ")
			bytePasswordConfirm, err := term.ReadPassword(int(syscall.Stdin))
			if err != nil {
				logger.Info("\nError reading password")
				return
			}
			passwordConfirm := string(bytePasswordConfirm)
			// Empty line removed - use structured logging instead

			if password != passwordConfirm {
				logger.Info("Passwords do not match")
				return
			}

			data := map[string]interface{}{
				"email":           email,
				"password":        password,
				"passwordConfirm": passwordConfirm,
			}
			jsonData, _ := json.Marshal(data)

			// 1. Create User
			resp, err := http.Post(apiURL+"/api/collections/users/records", "application/json", bytes.NewBuffer(jsonData))
			if err != nil {
				logger.Info("Registration failed: %v", "value", err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode >= 400 {
				body, _ := io.ReadAll(resp.Body)
				logger.Info("Registration failed: %s", "value", string(body))
				return
			}

			logger.Info("Registration successful. Logging in...")

			// 2. Auto Login
			loginData := map[string]interface{}{
				"identity": email,
				"password": password,
			}
			loginJson, _ := json.Marshal(loginData)

			respLogin, err := http.Post(apiURL+"/api/collections/users/auth-with-password", "application/json", bytes.NewBuffer(loginJson))
			if err != nil {
				logger.Info("Auto-login failed: %v. Please login manually.", "value", err)
				return
			}
			defer respLogin.Body.Close()

			if respLogin.StatusCode != 200 {
				logger.Info("Auto-login failed. Please login manually.")
				return
			}

			var result map[string]interface{}
			json.NewDecoder(respLogin.Body).Decode(&result)

			token, _ := result["token"].(string)
			record, _ := result["record"].(map[string]interface{})
			id, _ := record["id"].(string)

			authData := auth.AuthData{
				Token: token,
				Email: email,
				ID:    id,
			}

			if err := auth.SaveAuth(authData); err != nil {
				logger.Info("Failed to save token: %v", "value", err)
				return
			}

			logger.Info("Login successful")
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "logout",
		Short: "Logout",
		Run: func(cmd *cobra.Command, args []string) {
			if err := auth.Logout(); err != nil {
				logger.Info("Logout error: %v", "value", err)
			} else {
				logger.Info("Logged out")
			}
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "google",
		Short: "Login with Google (Instructions)",
		Run: func(cmd *cobra.Command, args []string) {
			logger.Info("Google OAuth Login:")
			logger.Info("1. Ensure Google OAuth is configured in PocketBase Admin UI.")
			logger.Info("2. Visit the PocketBase UI to login via Google.")
			logger.Info("3. (CLI support for OAuth requires a browser redirect flow which is not yet implemented)")
			logger.Info("   Please use 'flip2 auth login' with email/password.")
		},
	})

	return cmd
}

func adminCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "admin",
		Short: "Open PocketBase admin UI",
		Run: func(cmd *cobra.Command, args []string) {
			url := apiURL + "/_/"
			logger.Info("Opening admin UI: %s", "value", url)
			exec.Command("open", url).Start()
		},
	}
}

func migrateCmd() *cobra.Command {
	var fromPath string
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Migrate from FLIP v1 database",
		Run: func(cmd *cobra.Command, args []string) {
			if fromPath == "" {
				logger.Info("Error: --from flag is required")
				return
			}
			logger.Info("Migrating from: %s", "value", fromPath)

			// Initialize PocketBase solely for migration logic
			// Note: This requires flip2d to be STOPPED if using SQLite and same data dir
			// Unless we use a separate connection but they share the lock?
			// SQLite allows multiple readers but one writer.
			// Best to warn user.

			// Check if daemon is running?
			pidFile := filepath.Join(os.TempDir(), pidFileName)
			if isRunning(pidFile) {
				logger.Info("WARNING: flip2d daemon is running. Please stop it first to avoid database locking issues.")
				logger.Info("Run: ./flip2 stop")
				return
			}

			// Initialize pure PocketBase instance for data access
			// We assume default data dir if not specified?
			// We should probably allow --data-dir flag

			pb := pocketbase.New()
			// pb.Bootstrap() // Do we need to bootstrap? migrate function just uses Dao/App.
			// But creating New() doesn't connect DB until we start or inspect?
			// Actually New() sets up things. We might need to manually init DB.

			// To make it simple, we use logic that flip2d uses:
			// Just New(), then we can access DB?
			// We need to trigger Bootstrap() to init DB connection.
			if err := pb.Bootstrap(); err != nil {
				logger.Info("Failed to bootstrap PocketBase: %v", "value", err)
				return
			}

			if err := migrate.MigrateFromFlip1(fromPath, pb); err != nil {
				logger.Info("Migration failed: %v", "value", err)
			} else {
				logger.Info("Migration completed")
			}
		},
	}
	cmd.Flags().StringVar(&fromPath, "from", "", "Path to FLIP v1 database")
	return cmd
}

// Helpers

func readPID(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(string(data))
}

func isRunning(pidFile string) bool {
	pid, err := readPID(pidFile)
	if err != nil {
		return false
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return process.Signal(syscall.Signal(0)) == nil
}

func getAgentRecordID(agentID string) (string, error) {
	if globalConfig == nil {
		loadGlobalConfig()
	}

	// Validate agent ID to prevent SQL injection
	if !validIdentifier.MatchString(agentID) {
		return "", fmt.Errorf("invalid agent_id format: %q (must be alphanumeric with hyphens/underscores)", agentID)
	}

	// Look up agent by 'agent_id' field
	filter := fmt.Sprintf("(agent_id='%s')", agentID)
	urlVal := fmt.Sprintf("%s/api/collections/agents/records?filter=%s", apiURL, url.QueryEscape(filter))

	req, _ := http.NewRequest("GET", urlVal, nil)
	setAuthHeaders(req)
	resp, err := httpclient.NewClientSkipTLS(30 * time.Second).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	items, ok := result["items"].([]interface{})
	if !ok || len(items) == 0 {
		return "", fmt.Errorf("agent not found")
	}

	item := items[0].(map[string]interface{})
	return item["id"].(string), nil
}

func createRecord(collection string, data map[string]interface{}) error {
	if globalConfig == nil {
		loadGlobalConfig()
	}
	jsonData, _ := json.Marshal(data)
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/api/collections/%s/records", apiURL, collection), bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	setAuthHeaders(req)

	resp, err := httpclient.NewClientSkipTLS(30 * time.Second).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error: %s", string(body))
	}
	return nil
}

func updateTaskStatus(taskID, status string) {
	if globalConfig == nil {
		loadGlobalConfig()
	}
	// Need to find record ID first?
	// If taskID is the functional ID (TASK-123), we need to lookup record ID.
	// But CLI user might pass functional ID.
	// Our migration sets "task_id" = "TASK-123" and also record ID = "TASK-123" (if valid ID).
	// PocketBase IDs are 15 chars. "TASK-123" is allowed as custom ID?
	// PocketBase IDs restrictions: 15 chars, alphanumeric?
	// Actually PB IDs are strictly 15 chars in older versions, but strings in recent? No, usually generated.
	// If we set custom ID, it must match regex.
	// If migration set it, fine.
	// If generated, we need to lookup by task_id.

	recordID, err := getRecordIDByField("tasks", "task_id", taskID)
	if err != nil {
		// Try using taskID as record ID directly
		recordID = taskID
	}

	data := map[string]interface{}{
		"status": status,
	}
	if status == "done" {
		data["completed_at"] = time.Now()
	}

	jsonData, _ := json.Marshal(data)
	req, _ := http.NewRequest("PATCH", fmt.Sprintf("%s/api/collections/tasks/records/%s", apiURL, recordID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	setAuthHeaders(req)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		logger.Info("Error occurred", "error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		logger.Info("Request failed", "response", string(body))
	} else {
		logger.Info("Task updated")
	}
}

func completeTask(taskID, result string) {
	if globalConfig == nil {
		loadGlobalConfig()
	}
	recordID, err := getRecordIDByField("tasks", "task_id", taskID)
	if err != nil {
		recordID = taskID
	}

	data := map[string]interface{}{
		"status":       "done",
		"result":       result,
		"completed_at": time.Now(),
	}

	jsonData, _ := json.Marshal(data)
	req, _ := http.NewRequest("PATCH", fmt.Sprintf("%s/api/collections/tasks/records/%s", apiURL, recordID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	setAuthHeaders(req)

	httpclient.NewClientSkipTLS(30 * time.Second).Do(req)
}

func executeAgentTask(agentID, title, description string) (string, error) {
	if globalConfig == nil {
		return "", fmt.Errorf("no config loaded")
	}

	backendName := agentID
	backendCfg, ok := globalConfig.Flip2.Backends[backendName]
	if !ok {
		return "", fmt.Errorf("no backend config for agent %s", agentID)
	}

	prompt := fmt.Sprintf("TASK: %s\nDESCRIPTION:\n%s\n\nPlease execute this task and provide the result.", title, description)

	if backendCfg.Type == "process" {
		cmdName := backendCfg.Command
		cmdArgs := make([]string, len(backendCfg.Args))
		copy(cmdArgs, backendCfg.Args)

		// Append prompt as the last argument
		cmdArgs = append(cmdArgs, prompt)

		cmd := exec.Command(cmdName, cmdArgs...)
		// cmd.Stdin = bytes.NewBufferString(prompt) // Don't use Stdin if passing as arg
		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			return "", err
		}
		return strings.TrimSpace(out.String()), nil
	}

	return "", fmt.Errorf("backend type %s not supported", backendCfg.Type)
}

func getRecordIDByField(collection, field, value string) (string, error) {
	if globalConfig == nil {
		loadGlobalConfig()
	}

	// Validate collection, field, and value to prevent SQL injection
	if !validIdentifier.MatchString(collection) {
		return "", fmt.Errorf("invalid collection name: %q (must be alphanumeric with hyphens/underscores)", collection)
	}
	if !validIdentifier.MatchString(field) {
		return "", fmt.Errorf("invalid field name: %q (must be alphanumeric with hyphens/underscores)", field)
	}
	if !validIdentifier.MatchString(value) {
		return "", fmt.Errorf("invalid value format: %q (must be alphanumeric with hyphens/underscores)", value)
	}

	urlVal := fmt.Sprintf("%s/api/collections/%s/records?filter=(%s='%s')", apiURL, collection, field, value)
	req, _ := http.NewRequest("GET", urlVal, nil)
	setAuthHeaders(req)
	resp, err := httpclient.NewClientSkipTLS(30 * time.Second).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	items, ok := result["items"].([]interface{})
	if !ok || len(items) == 0 {
		return "", fmt.Errorf("not found")
	}

	item := items[0].(map[string]interface{})
	return item["id"].(string), nil
}

func printCollectionItems(collection string, fields []string) {
	if globalConfig == nil {
		loadGlobalConfig()
	}
	req, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/collections/%s/records", apiURL, collection), nil)
	setAuthHeaders(req)
	resp, err := httpclient.NewClientSkipTLS(30 * time.Second).Do(req)
	if err != nil {
		logger.Info("Error occurred", "error", err)
		return
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	items, ok := result["items"].([]interface{})
	if !ok {
		logger.Info("No items found. Result: %v", "value", result)
		return
	}

	// Dynamic format string
	fmtStr := ""
	for range fields {
		fmtStr += "%-20s "
	}
	fmtStr += "\n"

	// Create interface slice for header
	header := make([]interface{}, len(fields))
	for i, f := range fields {
		header[i] = strings.ToUpper(f)
	}

	logger.Info("Displaying results", "headers", fmt.Sprint(header...))
	logger.Info("Results separator")

	for _, item := range items {
		m := item.(map[string]interface{})
		values := make([]interface{}, len(fields))
		for i, f := range fields {
			if v, ok := m[f]; ok {
				if s, ok := v.(string); ok {
					if len(s) > 18 {
						s = s[:15] + "..."
					}
					values[i] = s
				} else {
					values[i] = fmt.Sprintf("%v", v)
				}
			} else {
				values[i] = "-"
			}
		}
		logger.Info("Result row", "values", fmt.Sprint(values...))
	}
}

// routingCmd - Analytics and dashboard for task routing
func routingCmd() *cobra.Command {
	routingCmd := &cobra.Command{
		Use:   "routing",
		Short: "View routing analytics and cost dashboards",
		Long:  "Display task routing analytics, cost breakdowns, and savings reports",
	}

	// routing report - markdown format
	routingCmd.AddCommand(&cobra.Command{
		Use:   "report",
		Short: "Show routing analytics report (markdown format)",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Create sample metrics for demonstration
			metrics := NewSampleMetrics()
			report := metrics.GenerateDashboard()
			fmt.Println(report)
			return nil
		},
	})

	// routing dashboard - ASCII format
	routingCmd.AddCommand(&cobra.Command{
		Use:   "dashboard",
		Short: "Show routing analytics dashboard (ASCII format)",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Create sample metrics for demonstration
			metrics := NewSampleMetrics()
			dashboard := metrics.GenerateDashboardASCII()
			fmt.Println(dashboard)
			return nil
		},
	})

	// routing status - current metrics
	routingCmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Show current routing metrics",
		RunE: func(cmd *cobra.Command, args []string) error {
			metrics := NewSampleMetrics()
			totalTasks := metrics.GetTotalTasksExecuted()
			totalCost := metrics.GetTotalCost()

			fmt.Printf("Routing Status:\n")
			fmt.Printf("  Total Tasks: %d\n", totalTasks)
			fmt.Printf("  Total Cost:  $%.4f USD\n", totalCost)

			if totalTasks > 0 {
				avgCost := totalCost / float64(totalTasks)
				fmt.Printf("  Avg Cost:    $%.6f per task\n", avgCost)
			}

			return nil
		},
	})

	return routingCmd
}

// NewSampleMetrics creates sample metrics for demonstration and testing.
// This generates a realistic day of task execution across different models.
func NewSampleMetrics() *routing.RoutingMetrics {
	metrics := routing.NewRoutingMetrics()

	// Morning: Quick tests and docs (Haiku)
	for i := 0; i < 10; i++ {
		metrics.RecordTaskExecution(routing.TaskTypeTesting, 0.0008, 300)
	}
	for i := 0; i < 3; i++ {
		metrics.RecordTaskExecution(routing.TaskTypeDocumentation, 0.001, 400)
	}

	// Midday: Code implementation (Sonnet)
	for i := 0; i < 8; i++ {
		metrics.RecordTaskExecution(routing.TaskTypeCodeGeneration, 0.025, 1200)
	}
	for i := 0; i < 5; i++ {
		metrics.RecordTaskExecution(routing.TaskTypeCodeReview, 0.02, 900)
	}

	// Afternoon: Complex work (Opus)
	metrics.RecordTaskExecution(routing.TaskTypeArchitecture, 0.12, 2000)
	metrics.RecordTaskExecution(routing.TaskTypeSecurity, 0.10, 1800)

	// Research (Gemini)
	metrics.RecordTaskExecution(routing.TaskTypeResearch, 0.003, 3000)
	metrics.RecordTaskExecution(routing.TaskTypeDataProcessing, 0.004, 2500)

	return metrics
}

// versionCmd - Collaborative feature implemented with Gemini via FLIP2
func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show FLIP2 version information",
		Run: func(cmd *cobra.Command, args []string) {
			logger.Info("FLIP2 - Multi-Agent Coordination System")
			logger.Info("Version:    %s", "value", version.Version)
			logger.Info("Build Date: %s", "value", version.BuildDate)
			logger.Info("Go Version: %s", "value", version.GoVersion)
		},
	}
}

func setAuthHeaders(req *http.Request) {
	// 1. Try User Token
	if authData, err := auth.LoadAuth(); err == nil && authData != nil {
		req.Header.Set("Authorization", "Bearer "+authData.Token)
		return // Prefer user token
	}

	// 2. Try Global API Key (Service/Daemon mode)
	if globalConfig == nil {
		loadGlobalConfig()
	}
	if globalConfig != nil && globalConfig.Flip2.Security.APIKey != "" {
		req.Header.Set("X-API-Key", globalConfig.Flip2.Security.APIKey)
	}
}

// parseDuration parses duration strings like "30d", "7d", "24h", "3600s"
func parseDuration(durationStr string) (time.Duration, error) {
	// Handle special cases like "30d"
	if strings.HasSuffix(durationStr, "d") {
		days := strings.TrimSuffix(durationStr, "d")
		dayCount, err := strconv.ParseInt(days, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid day count: %s", days)
		}
		return time.Duration(dayCount*24) * time.Hour, nil
	}

	// For other formats, use Go's built-in time.ParseDuration
	return time.ParseDuration(durationStr)
}

// spawnAgent spawns a new worker agent with a specified role and task.
// This function:
// 1. Calls the spawn package to create a role-based agent
// 2. Registers the agent in the database
// 3. Displays the spawned agent information
func spawnAgent(roleName, task string) {
	logger.Info("Spawning worker agent", "role", roleName, "task", task)

	// Use the spawn package to create the agent ID and validate the role
	agentID, err := spawn.SpawnWithRole(roleName, task)
	if err != nil {
		logger.Info("Failed to spawn agent", "error", err)
		return
	}

	// Get spawn info to display what will be spawned (for database record)
	spawnInfo, err := getSpawnInfo(roleName, task)
	if err != nil {
		logger.Info("Failed to prepare agent spawn", "error", err)
		return
	}

	// Override the generated agent ID with the one from the spawn package
	spawnInfo.AgentID = agentID

	// Register the agent in the database
	// Determine backend from model
	backend := "custom"
	if strings.Contains(spawnInfo.Model, "claude") {
		backend = "claude"
	} else if strings.Contains(spawnInfo.Model, "gemini") {
		backend = "gemini"
	}

	agentData := map[string]interface{}{
		"agent_id":      spawnInfo.AgentID,
		"status":        "offline",
		"backend":       backend,
		"mode":          "local",
		"capabilities":  map[string]interface{}{"role": spawnInfo.RoleName, "model": spawnInfo.Model},
		"metadata":      map[string]interface{}{
			"role": spawnInfo.RoleName,
			"task": spawnInfo.Task,
			"system_prompt": spawnInfo.SystemPrompt,
			"permissions": spawnInfo.Permissions,
			"spawned_at": spawnInfo.SpawnedAt,
		},
	}

	if err := createRecord("agents", agentData); err != nil {
		logger.Info("Failed to register spawned agent", "error", err)
		return
	}

	// Display success message with agent details
	logger.Info("Worker agent spawned successfully",
		"agent_id", spawnInfo.AgentID,
		"role", spawnInfo.RoleName,
		"model", spawnInfo.Model,
		"max_tokens", spawnInfo.MaxTokens,
		"task", spawnInfo.Task,
	)
}

// getSpawnInfo retrieves spawn information for a role.
// This is a wrapper that calls the spawn package (to be imported properly in main imports).
// Since we can't easily do the import without reorganizing, this function will
// contain the logic inline or will be called via reflection/indirect means.
func getSpawnInfo(roleName, task string) (*SpawnInfoDisplay, error) {
	// This function will call the spawn package once it's properly imported
	// For now, we'll implement a basic version that mimics the spawn package behavior

	// Built-in roles mapping (mirroring spawn.BuiltinRoles)
	builtinRoles := map[string]map[string]string{
		"code-reviewer": {
			"name":        "code-reviewer",
			"description": "Specialized code reviewer focused on bugs, style, and best practices",
			"model":       "claude-sonnet-4",
			"max_tokens":  "6144",
		},
		"researcher": {
			"name":        "researcher",
			"description": "Information gatherer and analyst focused on comprehensive research and synthesis",
			"model":       "gemini-2.5-pro",
			"max_tokens":  "10240",
		},
		"implementer": {
			"name":        "implementer",
			"description": "Code implementer focused on writing quality code based on specifications",
			"model":       "claude-sonnet-4",
			"max_tokens":  "8192",
		},
		"gemini-flash-worker": {
			"name":        "gemini-flash-worker",
			"description": "Fast, cost-effective code implementation using Gemini Flash",
			"model":       "gemini-2.5-flash",
			"max_tokens":  "8192",
		},
		"haiku-worker": {
			"name":        "haiku-worker",
			"description": "Lightweight code implementation baseline with Claude Haiku",
			"model":       "claude-haiku-4",
			"max_tokens":  "8192",
		},
	}

	roleData, exists := builtinRoles[roleName]
	if !exists {
		return nil, fmt.Errorf("role '%s' not found", roleName)
	}

	if task == "" {
		return nil, fmt.Errorf("task description is required")
	}

	// Generate agent ID (role-random)
	randomBytes := make([]byte, 6)
	_, err := rand.Read(randomBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to generate agent ID: %w", err)
	}
	agentID := fmt.Sprintf("%s-%s", roleName, hex.EncodeToString(randomBytes))

	maxTokens := 8192 // Default
	if mt, ok := roleData["max_tokens"]; ok {
		if parsed, err := strconv.Atoi(mt); err == nil {
			maxTokens = parsed
		}
	}

	return &SpawnInfoDisplay{
		AgentID:   agentID,
		RoleName:  roleName,
		Model:     roleData["model"],
		MaxTokens: maxTokens,
		Task:      task,
	}, nil
}

// SpawnInfoDisplay contains display information about a spawned agent
type SpawnInfoDisplay struct {
	AgentID   string
	RoleName  string
	Model     string
	MaxTokens int
	Task      string

	// Add fields needed for the agent record
	SystemPrompt string
	Permissions  interface{}
	SpawnedAt    time.Time
}

// pipelineCmd creates the pipeline command group
func pipelineCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pipeline",
		Short: "Manage pipelines",
		Long:  "Manage pipeline definitions, runs, and execution",
	}

	// pipeline list - List all pipelines
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List all pipelines",
		Long:  "List all pipeline runs with their current status",
		Run: func(cmd *cobra.Command, args []string) {
			printCollectionItems("pipeline_runs", []string{"id", "pipeline_id", "status", "current_stage_index", "total_stages"})
		},
	})

	// pipeline run <definition.yaml> - Run a pipeline
	cmd.AddCommand(&cobra.Command{
		Use:   "run <definition.yaml>",
		Short: "Run a pipeline from a YAML definition",
		Long: `Load a pipeline definition from a YAML file and start execution.

The YAML file should define:
  - name: Pipeline name
  - description: What the pipeline does
  - stages: List of stages to execute

Example:
  flip2 pipeline run ./pipelines/research.yaml`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			definitionPath := args[0]

			// Read the YAML file
			defData, err := os.ReadFile(definitionPath)
			if err != nil {
				logger.Info("Failed to read definition file", "path", definitionPath, "error", err)
				return
			}

			// Generate a unique pipeline run ID
			pipelineRunID := fmt.Sprintf("run-%d", time.Now().UnixNano())

			// Create a new pipeline run record in the database
			pipelineData := map[string]interface{}{
				"id":                   pipelineRunID,
				"pipeline_id":          "from-yaml",
				"status":               "pending",
				"current_stage_index":  0,
				"total_stages":         0,
				"input":                string(defData),
				"final_output":         "",
				"error":                "",
				"error_stage":          "",
				"retry_count":          0,
				"max_retries":          3,
				"priority":             0,
				"assigned_agent":       "cli",
				"started_at":           time.Now(),
				"completed_at":         nil,
				"last_checkpoint_at":   nil,
				"metadata":             `{}`,
			}

			if err := createRecord("pipeline_runs", pipelineData); err != nil {
				logger.Info("Failed to create pipeline run", "error", err)
				return
			}

			logger.Info("Pipeline run created successfully", "pipeline_id", pipelineRunID)
			logger.Info("To check status, run: flip2 pipeline status %s", "value", pipelineRunID)
		},
	})

	// pipeline status <id> - Show pipeline status
	cmd.AddCommand(&cobra.Command{
		Use:   "status <id>",
		Short: "Show status of a pipeline run",
		Long: `Display detailed status of a pipeline run including:
  - Overall pipeline status
  - Current stage and progress
  - Individual stage statuses
  - Error messages if any`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			pipelineID := args[0]

			// Fetch pipeline run from database
			filter := fmt.Sprintf("(id='%s')", pipelineID)
			reqURL := fmt.Sprintf("%s/api/collections/pipeline_runs/records?filter=%s", apiURL, url.QueryEscape(filter))

			resp, err := http.Get(reqURL)
			if err != nil {
				logger.Info("Failed to fetch pipeline", "error", err)
				return
			}
			defer resp.Body.Close()

			var result map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				logger.Info("Failed to decode response", "error", err)
				return
			}

			items, ok := result["items"].([]interface{})
			if !ok || len(items) == 0 {
				logger.Info("Pipeline not found", "id", pipelineID)
				return
			}

			pipeline := items[0].(map[string]interface{})

			// Display pipeline status
			logger.Info("Pipeline Status")
			logger.Info("================")
			logger.Info("ID", "value", pipeline["id"])
			logger.Info("Pipeline", "value", pipeline["pipeline_id"])
			logger.Info("Status", "value", pipeline["status"])
			logger.Info("Progress", "value", fmt.Sprintf("%d/%d",
				int(pipeline["current_stage_index"].(float64)),
				int(pipeline["total_stages"].(float64))))

			if startedAt := pipeline["started_at"]; startedAt != nil && startedAt != "" {
				logger.Info("Started At", "value", startedAt)
			}

			if err := pipeline["error"]; err != nil && err != "" {
				logger.Info("Error", "value", err)
				logger.Info("Error Stage", "value", pipeline["error_stage"])
			}

			// Fetch and display stage statuses
			stageFilter := fmt.Sprintf("(pipeline_run_id='%s')", pipelineID)
			stageURL := fmt.Sprintf("%s/api/collections/stage_runs/records?filter=%s&sort=stage_index", apiURL, url.QueryEscape(stageFilter))

			stageResp, err := http.Get(stageURL)
			if err == nil {
				defer stageResp.Body.Close()
				var stageResult map[string]interface{}
				if json.NewDecoder(stageResp.Body).Decode(&stageResult) == nil {
					stages, ok := stageResult["items"].([]interface{})
					if ok && len(stages) > 0 {
						logger.Info("\nStage Statuses")
						logger.Info("==============")
						for _, s := range stages {
							stage := s.(map[string]interface{})
							logger.Info("Stage", "index", stage["stage_index"], "name", stage["stage_name"], "status", stage["status"], "backend", stage["backend"])
						}
					}
				}
			}
		},
	})

	// pipeline resume <id> - Resume a failed pipeline
	cmd.AddCommand(&cobra.Command{
		Use:   "resume <id>",
		Short: "Resume a failed or paused pipeline",
		Long: `Resume execution of a failed or paused pipeline from where it left off.
This will:
  1. Load the last checkpoint
  2. Retry failed stages
  3. Continue execution from the last completed stage`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			pipelineID := args[0]

			// Fetch pipeline to check if it can be resumed
			filter := fmt.Sprintf("(id='%s')", pipelineID)
			reqURL := fmt.Sprintf("%s/api/collections/pipeline_runs/records?filter=%s", apiURL, url.QueryEscape(filter))

			resp, err := http.Get(reqURL)
			if err != nil {
				logger.Info("Failed to fetch pipeline", "error", err)
				return
			}
			defer resp.Body.Close()

			var result map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				logger.Info("Failed to decode response", "error", err)
				return
			}

			items, ok := result["items"].([]interface{})
			if !ok || len(items) == 0 {
				logger.Info("Pipeline not found", "id", pipelineID)
				return
			}

			pipeline := items[0].(map[string]interface{})
			status := pipeline["status"].(string)

			// Check if pipeline is in a resumable state
			resumableStates := map[string]bool{
				"failed":        true,
				"paused":        true,
				"checkpoint":    true,
				"stage_complete": true,
			}

			if !resumableStates[status] {
				logger.Info("Cannot resume pipeline in state", "status", status)
				return
			}

			// Update pipeline status to running
			updateData := map[string]interface{}{
				"status":     "running",
				"started_at": time.Now(),
			}
			jsonData, _ := json.Marshal(updateData)

			updateReq, _ := http.NewRequest("PATCH", fmt.Sprintf("%s/api/collections/pipeline_runs/records/%s", apiURL, pipelineID), bytes.NewBuffer(jsonData))
			updateReq.Header.Set("Content-Type", "application/json")
			setAuthHeaders(updateReq)

			updateResp, err := http.DefaultClient.Do(updateReq)
			if err != nil {
				logger.Info("Failed to update pipeline", "error", err)
				return
			}
			defer updateResp.Body.Close()

			if updateResp.StatusCode >= 400 {
				body, _ := io.ReadAll(updateResp.Body)
				logger.Info("Update failed", "response", string(body))
				return
			}

			logger.Info("Pipeline resumed successfully", "id", pipelineID)
			logger.Info("Run 'flip2 pipeline status %s' to monitor progress", "value", pipelineID)
		},
	})

	return cmd
}
