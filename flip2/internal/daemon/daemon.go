package daemon

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	gosync "sync"
	"syscall"
	"time"

	"flip2/internal/api"
	"flip2/internal/archiver"
	"flip2/internal/auth"
	"flip2/internal/commmonitor"
	"flip2/internal/config"
	"flip2/internal/executor"
	"flip2/internal/llm"
	"flip2/internal/logger"
	"flip2/internal/queue"
	"flip2/internal/scheduler"
	"flip2/internal/supervisor"
	"flip2/internal/sync"

	pocketbase "github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"
	"github.com/pocketbase/pocketbase/tools/types"
)

// Daemon manages the FLIP2 service lifecycle
type Daemon struct {
	config         *config.Config
	pb             *pocketbase.PocketBase
	scheduler      *scheduler.Scheduler
	executor       *executor.Executor
	logger         *slog.Logger
	pidFile        string
	currentLogPath string
	// Sync components
	jwtManager     *auth.JWTManager
	replicator     *sync.Replicator
	restServer     *api.RESTServer
	// Communication monitor
	commMonitor    *commmonitor.Monitor
	// FLIP2 API components (Step 3)
	llmRegistry    *llm.Registry
	taskQueue      *queue.Queue
	apiHandlers    *api.APIHandlers
	// Message archiver
	msgArchiver    *archiver.Archiver
	// Supervisor for fault tolerance
	supervisor     *supervisor.Supervisor
}

// New creates a new Daemon instance
func New(configPath string) (*Daemon, error) {
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return nil, err
	}

	// Setup logger with rotating file and console capture
	mainLogger, _, currentLogPath, err := logger.SetupLogger(cfg.Flip2.Daemon)
	if err != nil {
		return nil, fmt.Errorf("failed to setup logger: %w", err)
	}

	// Redirect os.Stdout and os.Stderr to the rotating writer
	// This captures all output, not just slog, into the rotating files
	// os.Stdout = rotatingWriter
	// os.Stderr = rotatingWriter

	return &Daemon{
		config:  cfg,
		logger:  mainLogger,
		pidFile: cfg.Flip2.Daemon.PIDFile,
		currentLogPath: currentLogPath,
	}, nil
}

// Start starts the daemon
func (d *Daemon) Start() error {
	// Check if already running
	if d.isRunning() {
		return fmt.Errorf("daemon already running (PID: %d)", d.getPID())
	}

	// Daemonize if requested via environment variable (set by wrapper)
	if os.Getenv("FLIP2_FOREGROUND") != "1" {
		return d.daemonize()
	}

	// Write PID file
	if err := d.writePID(); err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}
	defer d.removePID()

	// Environment safeguard - log prominently which environment we're in
	env := os.Getenv("FLIP2_ENV")
	if env == "" {
		env = "production" // Default to production for safety
	}

	// CRITICAL: Validate environment matches port convention
	// Production ports: 8xxx (specifically 8090)
	// Test ports: 9xxx (specifically 9190)
	port := d.config.Flip2.PocketBase.Port
	if err := validateEnvironmentPort(env, port); err != nil {
		d.logger.Error("ENVIRONMENT VALIDATION FAILED", "error", err)
		return fmt.Errorf("environment validation failed: %w", err)
	}

	d.logger.Info("========================================")
	d.logger.Info("FLIP2 DAEMON STARTING",
		"environment", env,
		"pid", os.Getpid(),
		"port", port)
	d.logger.Info("========================================")
    
    // Initialize PocketBase
	d.pb = pocketbase.NewWithConfig(pocketbase.Config{
		DefaultDataDir: d.config.Flip2.PocketBase.DataDir,
	})

	// Initialize Scheduler and Executor
	d.scheduler = scheduler.New(d.pb, d.logger)
	d.executor = executor.New(d.pb, d.config, d.logger)

	// Initialize Sync components if enabled
	if d.config.Flip2.Sync.Enabled {
		d.initializeSync()
	}

	// Bootstrap required collections (create if missing)
	d.bootstrapCollections()

	// Register hooks
	d.registerHooks()

	// Register built-in jobs
	d.registerJobs()

	// Initialize Supervisor for fault tolerance (Erlang-style supervision tree)
	d.supervisor = supervisor.New("flip2-root", d.logger, 10, 1*time.Minute)

	// Add Executor worker (Permanent = always restart)
	execWorker := supervisor.NewExecutorWorker(d.executor, d.logger)
	d.supervisor.AddWorker(supervisor.WorkerSpec{
		Worker:        execWorker,
		Strategy:      supervisor.Permanent,
		MaxRestarts:   5,
		RestartWindow: 1 * time.Minute,
	})

	// Add Scheduler worker (Permanent = always restart)
	schedWorker := supervisor.NewSchedulerWorker(d.scheduler, d.logger)
	d.supervisor.AddWorker(supervisor.WorkerSpec{
		Worker:        schedWorker,
		Strategy:      supervisor.Permanent,
		MaxRestarts:   5,
		RestartWindow: 1 * time.Minute,
	})

	// Add Replicator worker if sync is enabled
	if d.config.Flip2.Sync.Enabled && d.replicator != nil {
		replWorker := supervisor.NewReplicatorWorker(d.replicator, d.logger)
		d.supervisor.AddWorker(supervisor.WorkerSpec{
			Worker:        replWorker,
			Strategy:      supervisor.Permanent,
			MaxRestarts:   3,
			RestartWindow: 2 * time.Minute,
		})
	}

	// Start supervisor (this starts all workers in goroutines)
	if err := d.supervisor.Start(context.Background()); err != nil {
		return fmt.Errorf("failed to start supervisor: %w", err)
	}
	d.logger.Info("Supervisor started", "workers", d.supervisor.WorkerCount())

	// Communication Monitor will be started in OnServe hook (after PocketBase is ready)
	d.commMonitor = commmonitor.New(d.pb, commmonitor.DefaultConfig(), d.logger)
	d.registerCommMonitor()

	// Initialize Message Archiver (if enabled)
	d.initializeArchiver()

	// Initialize FLIP2 API components (Step 3)
	d.initializeFLIP2API()

	d.logger.Info("PocketBase initialized", "port", d.config.Flip2.PocketBase.Port)

	// Start PocketBase (this blocks until it's ready to serve)
	listenAddr := fmt.Sprintf("%s:%d", d.config.Flip2.PocketBase.Host, d.config.Flip2.PocketBase.Port)

	// Override os.Args to force PocketBase to run 'serve' command
	if d.config.Flip2.PocketBase.TLS.Enabled {
		// HTTPS mode with TLS certificates
		os.Args = []string{
			"pocketbase", "serve",
			"--https", listenAddr,
			"--dir", d.config.Flip2.PocketBase.DataDir,
		}
		d.logger.Info("Starting PocketBase HTTPS server",
			"addr", listenAddr,
			"cert", d.config.Flip2.PocketBase.TLS.CertFile,
			"key", d.config.Flip2.PocketBase.TLS.KeyFile)
	} else {
		// HTTP mode
		os.Args = []string{
			"pocketbase", "serve",
			"--http", listenAddr,
			"--dir", d.config.Flip2.PocketBase.DataDir,
		}
		d.logger.Info("Starting PocketBase HTTP server", "addr", listenAddr)
	}

	d.logger.Info("PocketBase args", "args", os.Args)
	if err := d.pb.Start(); err != nil { // Use Start() which blocks until server stops
		d.logger.Error("PocketBase server failed", "error", err)
		return fmt.Errorf("failed to start PocketBase: %w", err)
	}
	d.logger.Info("PocketBase server stopped")

	return nil
}

// Shutdown gracefully stops the daemon
func (d *Daemon) Shutdown() error {
	d.logger.Info("Shutting down...")

	// Stop supervisor first (stops all supervised workers: scheduler, executor, replicator)
	if d.supervisor != nil {
		if err := d.supervisor.Stop(); err != nil {
			d.logger.Error("Error stopping supervisor", "error", err)
		}
	}

	if d.msgArchiver != nil {
		d.msgArchiver.Stop()
	}

	// PocketBase doesn't expose a Clean Stop method easily from outside,
	// but mostly we just need to ensure our components stop.
	return nil
}


// In registerHooks, ensure ServeEvent.Next() is called where appropriate
func (d *Daemon) registerHooks() {
	// Trigger task executor when new task assigned
	d.pb.OnRecordAfterCreateSuccess("tasks").BindFunc(func(e *core.RecordEvent) error {
		assignee := e.Record.GetString("assignee")
		if assignee != "" {
			d.logger.Info("New task assigned", "task_id", e.Record.Id, "assignee", assignee)
			d.executor.QueueTask(e.Record.Id)
		}
		return nil
	})

	d.pb.OnRecordAfterUpdateSuccess("tasks").BindFunc(func(e *core.RecordEvent) error {
		status := e.Record.GetString("status")
		assignee := e.Record.GetString("assignee")

		// If status changed to in_progress or assigned changed
		if status == "in_progress" && assignee != "" {
			d.executor.QueueTask(e.Record.Id)
		}
		return nil
	})

	// Case-Insensitive Agent IDs - Normalize on Create
	d.pb.OnRecordCreateRequest("signals").BindFunc(func(e *core.RecordRequestEvent) error {
		fromAgent := e.Record.GetString("from_agent")
		toAgent := e.Record.GetString("to_agent")

		if fromAgent != "" {
			e.Record.Set("from_agent", strings.ToLower(fromAgent))
		}
		if toAgent != "" {
			e.Record.Set("to_agent", strings.ToLower(toAgent))
		}

		d.logger.Debug("Normalized signal agent IDs",
			"from", fromAgent, "from_normalized", strings.ToLower(fromAgent),
			"to", toAgent, "to_normalized", strings.ToLower(toAgent))

		return e.Next()
	})

	// Case-Insensitive Agent IDs - Normalize on Update
	d.pb.OnRecordUpdateRequest("signals").BindFunc(func(e *core.RecordRequestEvent) error {
		fromAgent := e.Record.GetString("from_agent")
		toAgent := e.Record.GetString("to_agent")

		if fromAgent != "" {
			e.Record.Set("from_agent", strings.ToLower(fromAgent))
		}
		if toAgent != "" {
			e.Record.Set("to_agent", strings.ToLower(toAgent))
		}

		return e.Next()
	})

	// TLS Configuration (if enabled)
	if d.config.Flip2.PocketBase.TLS.Enabled {
		d.pb.OnServe().BindFunc(func(e *core.ServeEvent) error {
			cert, err := tls.LoadX509KeyPair(
				d.config.Flip2.PocketBase.TLS.CertFile,
				d.config.Flip2.PocketBase.TLS.KeyFile,
			)
			if err != nil {
				d.logger.Error("Failed to load TLS certificates", "error", err)
				return err
			}

			e.Server.TLSConfig = &tls.Config{
				Certificates: []tls.Certificate{cert},
				MinVersion:   tls.VersionTLS12,
			}
			d.logger.Info("TLS configured",
				"cert", d.config.Flip2.PocketBase.TLS.CertFile,
				"min_version", "TLS1.2")

			return e.Next() // CRITICAL: Always call Next()
		})
	}

	// API Key Middleware
	d.pb.OnServe().BindFunc(func(e *core.ServeEvent) error {
		e.Router.BindFunc(func(re *core.RequestEvent) error { // re is of type *RequestEvent
			if !d.config.Flip2.Security.APIKeysEnabled {
				return re.Next() // Continue to next handler if API keys are not enabled
			}

			// Check for API Key
			apiKey := re.Request.Header.Get("X-API-Key")
			if apiKey != "" {
				if apiKey == d.config.Flip2.Security.APIKey {
					return re.Next() // Valid API key, continue
				}
				return &router.ApiError{Status: http.StatusUnauthorized, Message: "Invalid API Key"}
			} else if re.Request.Header.Get("Authorization") != "" {
				return re.Next() // Authorization header present, let PB handle it
			}

			// Allow Admin UI and static assets
			path := re.Request.URL.Path
			if path == "/" || path == "/_" || (len(path) > 3 && path[:3] == "/_/") {
				return re.Next()
			}

			// Allow Admin API (Login/Request Reset)
			if len(path) >= 11 && strings.HasPrefix(path, "/api/admins") { // Use strings.HasPrefix
				return re.Next()
			}

			// Allow Health check
			if path == "/api/health" {
				return re.Next()
			}

			// Block public API access
			if len(path) >= 5 && strings.HasPrefix(path, "/api/") { // Use strings.HasPrefix
				return &router.ApiError{Status: http.StatusUnauthorized, Message: "API Key or Auth Required"}
			}

			return re.Next() // Continue to next handler if none of the above conditions met
		})
		return e.Next()
	})

	// Metrics Endpoint
	d.pb.OnServe().BindFunc(func(e *core.ServeEvent) error {
		e.Router.GET("/api/metrics", func(e *core.RequestEvent) error { // Changed to *core.RequestEvent
			if !d.config.Flip2.Metrics.Enabled {
				return e.NoContent(http.StatusForbidden) // Use e.NoContent
			}

			stats := make(map[string]interface{})
			stats["goroutines"] = runtime.NumGoroutine()

			if count, err := d.pb.CountRecords("tasks"); err == nil {
				stats["tasks_total"] = count
			}

			if count, err := d.pb.CountRecords("agents"); err == nil {
				stats["agents_total"] = count
			}

			return e.JSON(http.StatusOK, stats) // Use e.JSON
		})
		return e.Next()
	})


	// Task Signal Endpoint
	d.pb.OnServe().BindFunc(func(e *core.ServeEvent) error {
		e.Router.POST("/api/tasks/{id}/signal", func(evt *core.RequestEvent) error {
			taskID := evt.Request.PathValue("id")

			var data struct {
				Signal string `json:"signal"`
			}
			// Attempt to bind, ignore error if empty body (defaults will handle)
			// evt.Bind(&data)
			json.NewDecoder(evt.Request.Body).Decode(&data)

			sigStr := data.Signal
			if sigStr == "" {
				sigStr = "SIGTERM"
			}

			var sig os.Signal
			switch sigStr {
			case "SIGTERM":
				sig = syscall.SIGTERM
			case "SIGKILL":
				sig = syscall.SIGKILL
			case "SIGINT":
				sig = syscall.SIGINT
			default:
				return evt.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid signal"})
			}

			if err := d.executor.SignalTask(taskID, sig); err != nil {
				return evt.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}

			return evt.JSON(http.StatusOK, map[string]string{"status": "signaled"})
		})
		return e.Next()
	})

	// Register Daemon Agent on Startup
	d.pb.OnServe().BindFunc(func(e *core.ServeEvent) error {
		go func() {
			// Give server a moment to start
			time.Sleep(1 * time.Second)
			
			// Use fixed shorter ID
			daemonID := "daemon_svc_01"
			hostname, _ := os.Hostname()
			agentID := fmt.Sprintf("daemon-%s", hostname)
			// Truncate agentID if needed, but ID field is the critical one for length.
			
			collection, err := d.pb.FindCollectionByNameOrId("agents")
			if err != nil {
				d.logger.Error("Agents collection not found", "error", err)
				return
			}
			
			agent, err := d.pb.FindRecordById("agents", daemonID)
			if err != nil {
				// Create
				agent = core.NewRecord(collection)
				agent.Set("id", daemonID)
				agent.Set("agent_id", agentID)
				agent.Set("status", "online")
				// Use 'local' for backend/mode to avoid validation errors
				agent.Set("backend", "local") 
				agent.Set("mode", "local")
				agent.Set("last_seen", time.Now())
				if d.currentLogPath != "" {
					agent.Set("daemon_log_path", d.currentLogPath)
				}
				
				if err := d.pb.Save(agent); err != nil {
					d.logger.Error("Failed to register daemon agent", "error", err)
				} else {
					d.logger.Info("Registered daemon agent", "id", daemonID)
				}
			} else {
				// Update
				agent.Set("status", "online")
				agent.Set("last_seen", time.Now())
				if d.currentLogPath != "" {
					agent.Set("daemon_log_path", d.currentLogPath)
				}
				if err := d.pb.Save(agent); err != nil {
					d.logger.Error("Failed to update daemon agent", "error", err)
				}
			}
		}()
		return e.Next()
	})
}


func (d *Daemon) registerJobs() {
	// Health check job
	d.scheduler.RegisterJob("health-check", "0 */1 * * * *", func(ctx context.Context) error {
		d.logger.Info("Health check: OK")
		// TODO: write to events?
		return nil
	})
	
	// Zombie Task Reaper
	// Runs every 5 minutes (Seconds Minutes Hours DOM Month DOW)
	d.scheduler.RegisterJob("zombie-reaper", "0 */5 * * * *", func(ctx context.Context) error {
		// Find tasks that are in_progress
		// Using FindRecordsByFilter(collection, filter, sort, limit, offset)
		records, err := d.pb.FindRecordsByFilter("tasks", "status = 'in_progress'", "", 100, 0)
		if err != nil {
			return err
		}
		
		for _, rec := range records {
			assigneeID := rec.GetString("assignee")
			if assigneeID == "" {
				continue
			}
			
			// Check agent health
			agent, err := d.pb.FindRecordById("agents", assigneeID)
			if err != nil {
				d.logger.Error("Reaper: Failed to find agent", "agent_id", assigneeID, "error", err)
				continue
			}
			
			lastSeen := agent.GetDateTime("last_seen")
			if lastSeen.IsZero() {
				// No last_seen? Skip or assume active if recently started.
				continue
			}
			
			// If last_seen > 5 minutes ago
			if time.Since(lastSeen.Time()) > 5*time.Minute {
				d.logger.Warn("Reaper: Detected zombie task", "task_id", rec.Id, "agent_id", assigneeID)
				
				// Reset task
				rec.Set("status", "failed")
				rec.Set("result", "Agent lost connection (Zombie Task)")
				rec.Set("completed_at", time.Now())
				if err := d.pb.Save(rec); err != nil {
					d.logger.Error("Reaper: Failed to recover task", "task_id", rec.Id, "error", err)
				}
			}
		}
		return nil
	})

	// Log Pattern Cleanup - Every 8 hours
	// Removes obviously junk/transient logs that have no value after a few hours
	d.scheduler.RegisterJob("log-pattern-cleanup", "0 0 */8 * * *", func(ctx context.Context) error {
		d.logger.Info("Running log pattern cleanup")

		// Patterns that are junk after a few hours:
		// - Health check logs (we have metrics for this)
		// - SSE connection logs (transient)
		// - Heartbeat logs (too frequent)
		// - Debug-level request logs older than 4 hours
		patterns := []string{
			// Health checks older than 4 hours
			`DELETE FROM _logs WHERE created < datetime('now', '-4 hours') AND (
				message LIKE '%health%' OR
				message LIKE '%heartbeat%' OR
				message LIKE '%ping%' OR
				url LIKE '%/api/health%'
			)`,
			// SSE/realtime connection logs older than 2 hours
			`DELETE FROM _logs WHERE created < datetime('now', '-2 hours') AND (
				url LIKE '%/api/realtime%' OR
				url LIKE '%/_/%' OR
				message LIKE '%SSE%' OR
				message LIKE '%connection%established%'
			)`,
			// Repetitive sync logs older than 6 hours
			`DELETE FROM _logs WHERE created < datetime('now', '-6 hours') AND (
				message LIKE '%peer-sync%' OR
				message LIKE '%Running job%' OR
				message LIKE '%Job completed%'
			)`,
		}

		totalDeleted := 0
		for _, query := range patterns {
			result, err := d.pb.DB().NewQuery(query).Execute()
			if err != nil {
				d.logger.Warn("Log pattern cleanup query failed", "error", err)
				continue
			}
			if affected, _ := result.RowsAffected(); affected > 0 {
				totalDeleted += int(affected)
			}
		}

		d.logger.Info("Log pattern cleanup completed", "deleted", totalDeleted)
		return nil
	})

	// Full Log Pruning - Every 48 hours
	// Removes all logs older than 48 hours and VACUUMs the database
	d.scheduler.RegisterJob("log-full-prune", "0 0 0 */2 * *", func(ctx context.Context) error {
		d.logger.Info("Running full log prune (48h retention)")

		// Delete all logs older than 48 hours
		result, err := d.pb.DB().NewQuery(
			"DELETE FROM _logs WHERE created < datetime('now', '-48 hours')",
		).Execute()
		if err != nil {
			d.logger.Error("Failed to prune logs", "error", err)
			return err
		}

		deleted, _ := result.RowsAffected()
		d.logger.Info("Logs pruned", "deleted", deleted)

		// VACUUM to reclaim disk space (runs on auxiliary.db)
		// This is expensive but necessary to actually free disk space
		_, err = d.pb.DB().NewQuery("VACUUM").Execute()
		if err != nil {
			d.logger.Warn("VACUUM failed (non-critical)", "error", err)
			// Don't return error - VACUUM failure is not critical
		} else {
			d.logger.Info("VACUUM completed - disk space reclaimed")
		}

		return nil
	})

	// Statistics Gathering & Anomaly Detection - Every 6 hours
	// Collects signal statistics, detects anomalies, and optionally spawns Gemini for analysis
	// Uses non-blocking goroutines with channels for parallel data collection
	d.scheduler.RegisterJob("stats-anomaly-detection", "0 0 */6 * * *", func(ctx context.Context) error {
		d.logger.Info("Running statistics gathering and anomaly detection")

		// Determine paths based on OS
		reportsDir := "reports/statistics"
		anomalyDir := "reports/anomalies"
		if d.config.Flip2.PocketBase.DataDir != "" && strings.HasPrefix(d.config.Flip2.PocketBase.DataDir, "C:") {
			reportsDir = "C:\\flip2\\reports\\statistics"
			anomalyDir = "C:\\flip2\\reports\\anomalies"
		}

		// Result channels for concurrent query execution
		type queryResult struct {
			name  string
			value interface{}
			err   error
		}
		resultsChan := make(chan queryResult, 6)

		// Spawn goroutines for parallel query execution
		// Query 1: Total signals count
		go func() {
			var count int
			err := d.pb.DB().NewQuery("SELECT COUNT(*) FROM signals").Row(&count)
			resultsChan <- queryResult{"total_signals", count, err}
		}()

		// Query 2: Signals by type
		go func() {
			typeStats := make(map[string]int)
			rows, err := d.pb.DB().NewQuery("SELECT signal_type, COUNT(*) as count FROM signals GROUP BY signal_type").Rows()
			if err == nil && rows != nil {
				defer rows.Close()
				for rows.Next() {
					var signalType string
					var count int
					if rows.Scan(&signalType, &count) == nil && signalType != "" {
						typeStats[signalType] = count
					}
				}
			}
			resultsChan <- queryResult{"by_signal_type", typeStats, err}
		}()

		// Query 3: Signals by from_agent
		go func() {
			agentStats := make(map[string]int)
			rows, err := d.pb.DB().NewQuery("SELECT from_agent, COUNT(*) as count FROM signals GROUP BY from_agent").Rows()
			if err == nil && rows != nil {
				defer rows.Close()
				for rows.Next() {
					var agent string
					var count int
					if rows.Scan(&agent, &count) == nil && agent != "" {
						agentStats[agent] = count
					}
				}
			}
			resultsChan <- queryResult{"by_from_agent", agentStats, err}
		}()

		// Query 4: Signals by to_agent
		go func() {
			destStats := make(map[string]int)
			rows, err := d.pb.DB().NewQuery("SELECT to_agent, COUNT(*) as count FROM signals GROUP BY to_agent").Rows()
			if err == nil && rows != nil {
				defer rows.Close()
				for rows.Next() {
					var agent string
					var count int
					if rows.Scan(&agent, &count) == nil && agent != "" {
						destStats[agent] = count
					}
				}
			}
			resultsChan <- queryResult{"by_to_agent", destStats, err}
		}()

		// Query 5: Recent 24h signals
		go func() {
			var count int
			err := d.pb.DB().NewQuery("SELECT COUNT(*) FROM signals WHERE created > datetime('now', '-24 hours')").Row(&count)
			resultsChan <- queryResult{"signals_last_24h", count, err}
		}()

		// Query 6: Unread signals
		go func() {
			var count int
			err := d.pb.DB().NewQuery("SELECT COUNT(*) FROM signals WHERE read = false OR read IS NULL").Row(&count)
			resultsChan <- queryResult{"unread_signals", count, err}
		}()

		// Collect results with timeout using select
		stats := make(map[string]interface{})
		timestamp := time.Now().Format("2006-01-02T15:04:05")
		stats["timestamp"] = timestamp
		stats["node_id"] = d.config.Flip2.Sync.NodeID

		timeoutChan := time.After(30 * time.Second)
		collected := 0
		for collected < 6 {
			select {
			case result := <-resultsChan:
				if result.err == nil {
					stats[result.name] = result.value
				} else {
					d.logger.Warn("Query failed", "query", result.name, "error", result.err)
				}
				collected++
			case <-timeoutChan:
				d.logger.Warn("Statistics query timeout, proceeding with partial data", "collected", collected)
				goto ProcessStats
			case <-ctx.Done():
				d.logger.Warn("Context cancelled during statistics gathering")
				return ctx.Err()
			}
		}

	ProcessStats:
		// Extract values for anomaly detection (with safe type assertions)
		totalSignals, _ := stats["total_signals"].(int)
		recent24h, _ := stats["signals_last_24h"].(int)
		unreadCount, _ := stats["unread_signals"].(int)
		agentStats, _ := stats["by_from_agent"].(map[string]int)

		// Anomaly Detection - run in parallel with result aggregation
		anomalyChan := make(chan map[string]interface{}, 10)
		var wg gosync.WaitGroup

		// Anomaly checker 1: High unread count
		wg.Add(1)
		go func() {
			defer wg.Done()
			if unreadCount > 100 {
				anomalyChan <- map[string]interface{}{
					"type":     "high_unread_count",
					"severity": "warning",
					"message":  fmt.Sprintf("High number of unread signals: %d (threshold: 100)", unreadCount),
					"value":    unreadCount,
				}
			}
		}()

		// Anomaly checker 2: Very high activity
		wg.Add(1)
		go func() {
			defer wg.Done()
			if recent24h > 500 {
				anomalyChan <- map[string]interface{}{
					"type":     "high_activity_rate",
					"severity": "info",
					"message":  fmt.Sprintf("High signal activity in last 24h: %d", recent24h),
					"value":    recent24h,
				}
			}
		}()

		// Anomaly checker 3: No recent activity
		wg.Add(1)
		go func() {
			defer wg.Done()
			if recent24h == 0 && totalSignals > 0 {
				anomalyChan <- map[string]interface{}{
					"type":     "no_recent_activity",
					"severity": "warning",
					"message":  "No signal activity in the last 24 hours",
					"value":    0,
				}
			}
		}()

		// Anomaly checker 4: Agent dominance
		wg.Add(1)
		go func() {
			defer wg.Done()
			for agent, count := range agentStats {
				if totalSignals > 0 && float64(count)/float64(totalSignals) > 0.8 {
					anomalyChan <- map[string]interface{}{
						"type":     "agent_dominance",
						"severity": "info",
						"message":  fmt.Sprintf("Agent '%s' accounts for >80%% of signals (%d/%d)", agent, count, totalSignals),
						"agent":    agent,
						"value":    count,
					}
				}
			}
		}()

		// Close channel when all checkers complete
		go func() {
			wg.Wait()
			close(anomalyChan)
		}()

		// Collect anomalies
		anomalies := []map[string]interface{}{}
		for anomaly := range anomalyChan {
			anomalies = append(anomalies, anomaly)
		}

		stats["anomalies"] = anomalies
		stats["anomaly_count"] = len(anomalies)

		// File I/O in separate goroutine (non-blocking)
		go func() {
			statsJSON, err := json.MarshalIndent(stats, "", "  ")
			if err != nil {
				d.logger.Error("Failed to marshal statistics", "error", err)
				return
			}

			if err := os.MkdirAll(reportsDir, 0755); err != nil {
				d.logger.Error("Failed to create reports directory", "error", err)
				return
			}

			statsFile := fmt.Sprintf("%s/stats_%s.json", reportsDir, time.Now().Format("2006-01-02_15-04"))
			if err := os.WriteFile(statsFile, statsJSON, 0644); err != nil {
				d.logger.Error("Failed to write statistics file", "error", err)
			} else {
				d.logger.Info("Statistics saved", "file", statsFile)
			}

			// Save anomaly report if detected
			if len(anomalies) > 0 {
				anomalyReport := map[string]interface{}{
					"timestamp":  timestamp,
					"node_id":    d.config.Flip2.Sync.NodeID,
					"anomalies":  anomalies,
					"statistics": stats,
				}

				anomalyJSON, _ := json.MarshalIndent(anomalyReport, "", "  ")
				if err := os.MkdirAll(anomalyDir, 0755); err != nil {
					d.logger.Error("Failed to create anomaly directory", "error", err)
					return
				}

				anomalyFile := fmt.Sprintf("%s/anomaly_%s.json", anomalyDir, time.Now().Format("2006-01-02_15-04"))
				if err := os.WriteFile(anomalyFile, anomalyJSON, 0644); err != nil {
					d.logger.Error("Failed to write anomaly file", "error", err)
				} else {
					d.logger.Warn("Anomalies detected and saved", "file", anomalyFile, "count", len(anomalies))
				}

				// Spawn Gemini analysis for severe anomalies (non-blocking)
				hasSevereAnomaly := false
				for _, a := range anomalies {
					if a["severity"] == "warning" || a["severity"] == "critical" {
						hasSevereAnomaly = true
						break
					}
				}

				if hasSevereAnomaly && d.llmRegistry != nil {
					d.spawnGeminiAnalysis(anomalyDir, statsJSON, anomalyJSON, stats)
				}
			}
		}()

		d.logger.Info("Statistics gathering completed",
			"total_signals", totalSignals,
			"signals_24h", recent24h,
			"unread", unreadCount,
			"anomalies", len(anomalies))

		return nil
	})

	// Aggregate Statistics - Daily at midnight
	// Maintains long-term aggregate statistics for trend analysis
	d.scheduler.RegisterJob("stats-daily-aggregate", "0 0 0 * * *", func(ctx context.Context) error {
		d.logger.Info("Running daily aggregate statistics")

		aggregateDir := "reports/statistics/aggregates"
		if d.config.Flip2.PocketBase.DataDir != "" && strings.HasPrefix(d.config.Flip2.PocketBase.DataDir, "C:") {
			aggregateDir = "C:\\flip2\\reports\\statistics\\aggregates"
		}
		if err := os.MkdirAll(aggregateDir, 0755); err != nil {
			d.logger.Error("Failed to create aggregate directory", "error", err)
		}

		today := time.Now().Format("2006-01-02")
		aggregateFile := fmt.Sprintf("%s/daily_%s.json", aggregateDir, today)

		// Gather daily statistics
		dailyStats := map[string]interface{}{
			"date":    today,
			"node_id": d.config.Flip2.Sync.NodeID,
		}

		// Total signals at end of day
		var totalSignals int
		d.pb.DB().NewQuery("SELECT COUNT(*) FROM signals").Row(&totalSignals)
		dailyStats["total_signals"] = totalSignals

		// Signals created today
		var todaySignals int
		d.pb.DB().NewQuery("SELECT COUNT(*) FROM signals WHERE DATE(created) = DATE('now')").Row(&todaySignals)
		dailyStats["signals_today"] = todaySignals

		// Signals by type today
		typeStats := make(map[string]int)
		rows, _ := d.pb.DB().NewQuery("SELECT signal_type, COUNT(*) FROM signals WHERE DATE(created) = DATE('now') GROUP BY signal_type").Rows()
		if rows != nil {
			defer rows.Close()
			for rows.Next() {
				var signalType string
				var count int
				if rows.Scan(&signalType, &count) == nil && signalType != "" {
					typeStats[signalType] = count
				}
			}
		}
		dailyStats["by_type_today"] = typeStats

		// Active agents today
		var activeAgents int
		d.pb.DB().NewQuery("SELECT COUNT(DISTINCT from_agent) FROM signals WHERE DATE(created) = DATE('now')").Row(&activeAgents)
		dailyStats["active_agents"] = activeAgents

		// Save daily aggregate
		aggregateJSON, _ := json.MarshalIndent(dailyStats, "", "  ")
		if err := os.WriteFile(aggregateFile, aggregateJSON, 0644); err != nil {
			d.logger.Error("Failed to write daily aggregate", "error", err)
		} else {
			d.logger.Info("Daily aggregate saved", "file", aggregateFile)
		}

		// Also append to monthly summary (for long-term trends)
		monthlyFile := fmt.Sprintf("%s/monthly_%s.jsonl", aggregateDir, time.Now().Format("2006-01"))
		f, err := os.OpenFile(monthlyFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err == nil {
			defer f.Close()
			lineJSON, _ := json.Marshal(dailyStats)
			f.Write(append(lineJSON, '\n'))
			d.logger.Info("Appended to monthly aggregate", "file", monthlyFile)
		}

		return nil
	})
}

func (d *Daemon) daemonize() error {
	// Fork and exec with FLIP2_FOREGROUND=1
	cmd := exec.Command(os.Args[0], os.Args[1:]...)
	cmd.Env = append(os.Environ(), "FLIP2_FOREGROUND=1")
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	// Platform-specific daemonization (Linux/Mac only, skipped on Windows)
	return cmd.Start()
}

func (d *Daemon) writePID() error {
	if d.pidFile == "" {
		// Use default if not configured
		d.pidFile = "/tmp/flip2d.pid"
	}
	return os.WriteFile(d.pidFile, []byte(strconv.Itoa(os.Getpid())), 0644)
}

func (d *Daemon) removePID() {
	if d.pidFile != "" {
		os.Remove(d.pidFile)
	}
}

func (d *Daemon) getPID() int {
	if d.pidFile == "" {
		return 0
	}
	data, err := os.ReadFile(d.pidFile)
	if err != nil {
		return 0
	}
	pid, err := strconv.Atoi(string(data))
	if err != nil {
		return 0
	}
	return pid
}

func (d *Daemon) isRunning() bool {
	pid := d.getPID()
	if pid == 0 {
		return false
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// Send signal 0 to check if process exists
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// validateEnvironmentPort ensures the environment matches the port convention
// Production: port 8xxx (specifically 8090)
// Test: port 9xxx (specifically 9190)
// This prevents accidentally running production configs on test ports (or vice versa)
func validateEnvironmentPort(env string, port int) error {
	isProductionPort := port >= 8000 && port < 9000 // 8xxx range
	isTestPort := port >= 9000 && port < 10000      // 9xxx range

	switch env {
	case "production", "prod", "":
		if isTestPort {
			return fmt.Errorf("CRITICAL: Production environment cannot use test port %d. "+
				"Production must use port 8xxx (typically 8090). "+
				"Set FLIP2_ENV=test if you intend to run in test mode, or change the port to 8090", port)
		}
		if !isProductionPort {
			return fmt.Errorf("CRITICAL: Production environment must use port 8xxx (typically 8090). "+
				"Current port %d is outside the valid range", port)
		}
	case "test", "testing", "dev", "development":
		if isProductionPort {
			return fmt.Errorf("CRITICAL: Test environment cannot use production port %d. "+
				"Test must use port 9xxx (typically 9190). "+
				"Set FLIP2_ENV=production if you intend to run in production mode, or change the port to 9190", port)
		}
		if !isTestPort {
			return fmt.Errorf("CRITICAL: Test environment must use port 9xxx (typically 9190). "+
				"Current port %d is outside the valid range", port)
		}
	default:
		return fmt.Errorf("CRITICAL: Unknown environment '%s'. Use 'production' or 'test'", env)
	}

	return nil
}

// spawnGeminiAnalysis spawns a non-blocking Gemini analysis goroutine
// for anomaly detection. It uses context with timeout and proper error handling.
func (d *Daemon) spawnGeminiAnalysis(anomalyDir string, statsJSON, anomalyJSON []byte, stats map[string]interface{}) {
	gemini, ok := d.llmRegistry.Get("gemini")
	if !ok {
		d.logger.Debug("Gemini backend not available for analysis")
		return
	}

	// Check availability in a non-blocking way
	go func() {
		// Create analysis context with timeout
		analysisCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		// Check if Gemini is available
		if !gemini.IsAvailable(analysisCtx) {
			d.logger.Debug("Gemini not available, skipping analysis")
			return
		}

		// Build the analysis prompt
		prompt := fmt.Sprintf(`You are analyzing FLIP2 multi-agent system statistics.

Current statistics:
%s

Anomalies detected:
%s

Please analyze these statistics and anomalies:
1. Are the detected anomalies concerning?
2. What patterns do you see in the signal distribution?
3. Any recommendations for system health?
4. Are there any early warning signs we should monitor?

Keep your response concise (under 500 words) and actionable.`, string(statsJSON), string(anomalyJSON))

		// Execute the analysis
		response, err := gemini.Execute(analysisCtx, prompt, nil)
		if err != nil {
			d.logger.Warn("Gemini analysis failed", "error", err)
			return
		}

		// Prepare the analysis report
		analysisReport := map[string]interface{}{
			"timestamp":  time.Now().Format("2006-01-02T15:04:05"),
			"analyzer":   "gemini",
			"model":      response.Model,
			"prompt":     prompt,
			"analysis":   response.Content,
			"statistics": stats,
			"latency_ms": response.Latency.Milliseconds(),
		}

		// Save the analysis report
		analysisJSON, err := json.MarshalIndent(analysisReport, "", "  ")
		if err != nil {
			d.logger.Error("Failed to marshal Gemini analysis", "error", err)
			return
		}

		analysisFile := fmt.Sprintf("%s/gemini_analysis_%s.json", anomalyDir, time.Now().Format("2006-01-02_15-04"))
		if err := os.WriteFile(analysisFile, analysisJSON, 0644); err != nil {
			d.logger.Error("Failed to save Gemini analysis", "error", err)
		} else {
			d.logger.Info("Gemini analysis completed and saved", "file", analysisFile)
		}

		// Also create a human-readable summary
		summaryFile := fmt.Sprintf("%s/ANALYSIS_SUMMARY_%s.md", anomalyDir, time.Now().Format("2006-01-02"))
		summary := fmt.Sprintf(`# FLIP2 Anomaly Analysis Summary

**Date:** %s
**Analyzer:** Gemini
**Node:** %v

## Analysis

%s

---
*Auto-generated by FLIP2 anomaly detection system*
`, time.Now().Format("2006-01-02 15:04:05"), stats["node_id"], response.Content)

		if err := os.WriteFile(summaryFile, []byte(summary), 0644); err != nil {
			d.logger.Warn("Failed to save analysis summary", "error", err)
		}
	}()
}

// initializeSync sets up the sync components
func (d *Daemon) initializeSync() {
	d.logger.Info("Initializing sync components",
		"node_id", d.config.Flip2.Sync.NodeID,
		"peer_count", len(d.config.Flip2.Sync.Peers))

	// Initialize JWT Manager
	jwtSecret := d.config.Flip2.Security.JWTSecret
	if jwtSecret == "" {
		jwtSecret = d.config.Flip2.Security.APIKey // Fallback to API key
	}
	d.jwtManager = auth.NewJWTManager(jwtSecret, 24*time.Hour)

	// Initialize PocketBase-backed store for real signal synchronization
	// This replaces the old MemoryStore and enables signals to sync via PocketBase
	store := sync.NewPBStore(d.pb, d.config.Flip2.Sync.NodeID, d.logger)
	d.logger.Info("Using PocketBase store for sync")

	// Initialize Replicator
	d.replicator = sync.NewReplicator(
		d.config.Flip2.Sync.NodeID,
		store,
		d.logger,
	)

	// Add configured peers
	for _, peerCfg := range d.config.Flip2.Sync.Peers {
		if !peerCfg.Enabled {
			d.logger.Debug("Skipping disabled peer", "peer_id", peerCfg.ID)
			continue
		}

		peer := sync.NewHTTPPeer(peerCfg.ID, peerCfg.URL, peerCfg.APIKey, d.logger)
		peer.SetNodeID(d.config.Flip2.Sync.NodeID) // Set our node ID for JWT auth
		d.replicator.AddPeer(peer)
		d.logger.Info("Added sync peer", "peer_id", peerCfg.ID, "url", peerCfg.URL)
	}

	// Initialize REST server (with bootstrap API key for JWT security)
	d.restServer = api.NewRESTServer(d.logger, d.jwtManager, store, d.config.Flip2.Security.BootstrapAPIKey)
	d.restServer.SetReplicator(d.replicator)

	// Register sync routes and job
	d.registerSyncRoutes()
	d.registerSyncJob()
	d.registerSyncHooks() // Track local changes for vector clock

	d.logger.Info("Sync initialized", "peers", d.replicator.GetPeerCount())
}

// registerSyncRoutes adds sync-related routes to PocketBase
func (d *Daemon) registerSyncRoutes() {
	d.pb.OnServe().BindFunc(func(e *core.ServeEvent) error {
		// Register REST API routes
		d.restServer.RegisterRoutes(e.Router)
		d.logger.Info("Sync REST routes registered")
		return e.Next()
	})
}

// registerSyncHooks adds hooks to track local changes for vector clock sync
func (d *Daemon) registerSyncHooks() {
	// Set sync metadata BEFORE create (so it's saved with the record)
	d.pb.OnRecordCreateRequest("signals").BindFunc(func(e *core.RecordRequestEvent) error {
		// Only set sync metadata if not already set (i.e., not from sync)
		if e.Record.GetString("sync_origin") == "" && d.replicator != nil {
			// Increment vector clock for this local change
			d.replicator.OnLocalChange()
			vc := d.replicator.GetVectorClock()
			vcJSON, _ := json.Marshal(vc)
			e.Record.Set("sync_vector_clock", string(vcJSON))
			e.Record.Set("sync_origin", d.config.Flip2.Sync.NodeID)
			e.Record.Set("sync_timestamp", time.Now())
			d.logger.Debug("Set sync metadata on signal create",
				"signal_id", e.Record.GetString("signal_id"),
				"vector_clock", string(vcJSON))
		}
		return e.Next()
	})

	// Set sync metadata BEFORE update
	d.pb.OnRecordUpdateRequest("signals").BindFunc(func(e *core.RecordRequestEvent) error {
		// Only update sync metadata if this is a local change (not from sync)
		// Check if sync_origin matches our node or is empty (local change)
		origin := e.Record.GetString("sync_origin")
		if (origin == "" || origin == d.config.Flip2.Sync.NodeID) && d.replicator != nil {
			d.replicator.OnLocalChange()
			vc := d.replicator.GetVectorClock()
			vcJSON, _ := json.Marshal(vc)
			e.Record.Set("sync_vector_clock", string(vcJSON))
			e.Record.Set("sync_origin", d.config.Flip2.Sync.NodeID)
			e.Record.Set("sync_timestamp", time.Now())
			d.logger.Debug("Updated sync metadata on signal update",
				"signal_id", e.Record.GetString("signal_id"),
				"vector_clock", string(vcJSON))
		}
		return e.Next()
	})

	// Track deletes for vector clock
	d.pb.OnRecordAfterDeleteSuccess("signals").BindFunc(func(e *core.RecordEvent) error {
		if d.replicator != nil {
			d.replicator.OnLocalChange()
			d.logger.Debug("Vector clock incremented on signal delete", "signal_id", e.Record.GetString("signal_id"))
		}
		return e.Next()
	})

	d.logger.Info("Sync hooks registered for signals collection")
}

// registerCommMonitor registers communication monitor hooks after PocketBase is ready
// UPDATED: Now uses event hooks (real-time) instead of polling for performance
func (d *Daemon) registerCommMonitor() {
	d.pb.OnServe().BindFunc(func(e *core.ServeEvent) error {
		// Register communication monitor hooks (typo correction, agent validation)
		if d.commMonitor != nil {
			d.commMonitor.RegisterHooks()  // Event-driven, no polling
			d.logger.Info("Communication monitor hooks registered (real-time corrections enabled)")
		}
		return e.Next()
	})
}

// bootstrapCollections ensures required collections exist on startup
func (d *Daemon) bootstrapCollections() {
	d.pb.OnServe().BindFunc(func(e *core.ServeEvent) error {
		// Check if signals collection exists
		_, err := d.pb.FindCollectionByNameOrId("signals")
		if err != nil {
			d.logger.Info("Creating signals collection (bootstrap)")

			// Create signals collection with required fields
			collection := core.NewBaseCollection("signals")

			// Add fields
			collection.Fields.Add(&core.TextField{
				Name:     "signal_id",
				Required: true,
			})
			collection.Fields.Add(&core.TextField{
				Name:     "from_agent",
				Required: true,
			})
			collection.Fields.Add(&core.TextField{
				Name:     "to_agent",
				Required: true,
			})
			collection.Fields.Add(&core.SelectField{
				Name:   "signal_type",
				Values: []string{"message", "ping", "task", "alert", "handoff"},
			})
			collection.Fields.Add(&core.SelectField{
				Name:   "priority",
				Values: []string{"low", "normal", "high", "critical"},
			})
			collection.Fields.Add(&core.TextField{
				Name: "content",
			})
			collection.Fields.Add(&core.BoolField{
				Name: "read",
			})
			collection.Fields.Add(&core.DateField{
				Name: "read_at",
			})
			// Sync metadata fields
			collection.Fields.Add(&core.TextField{
				Name:   "sync_vector_clock",
				Hidden: true,
			})
			collection.Fields.Add(&core.TextField{
				Name:   "sync_origin",
				Hidden: true,
			})
			collection.Fields.Add(&core.DateField{
				Name:   "sync_timestamp",
				Hidden: true,
			})

			// Set rules to allow API key access
			collection.ListRule = types.Pointer("")
			collection.ViewRule = types.Pointer("")
			collection.CreateRule = types.Pointer("")
			collection.UpdateRule = types.Pointer("")
			collection.DeleteRule = types.Pointer("")

			// Add unique index on signal_id
			collection.AddIndex("idx_signals_signal_id", true, "signal_id", "")

			if err := d.pb.Save(collection); err != nil {
				d.logger.Error("Failed to create signals collection", "error", err)
				return err
			}
			d.logger.Info("Signals collection created successfully")
		} else {
			d.logger.Debug("Signals collection already exists")
		}

		return e.Next()
	})
}

// registerSyncJob adds the periodic sync job to the scheduler
func (d *Daemon) registerSyncJob() {
	interval := d.config.Flip2.Sync.SyncInterval
	if interval < 10*time.Second {
		interval = 30 * time.Second
	}

	// Convert interval to cron expression (every N seconds)
	seconds := int(interval.Seconds())
	cronExpr := fmt.Sprintf("*/%d * * * * *", seconds)

	d.scheduler.RegisterJob("peer-sync", cronExpr, func(ctx context.Context) error {
		if d.replicator == nil || d.replicator.GetPeerCount() == 0 {
			return nil
		}

		d.logger.Debug("Running peer sync")
		return d.replicator.SyncAll(ctx)
	})

	d.logger.Info("Sync job registered", "interval", interval)
}

// initializeFLIP2API initializes LLM registry, task queue, and API handlers (Step 3)
func (d *Daemon) initializeFLIP2API() {
	d.logger.Info("Initializing FLIP2 API components")

	// Initialize LLM Registry using the default registry (auto-detects available CLIs)
	d.llmRegistry = llm.DefaultRegistry()
	d.logger.Info("LLM Registry initialized", "backends", d.llmRegistry.List())

	// Initialize Task Queue
	queueConfig := queue.QueueConfig{
		PocketBase:      d.pb,
		Logger:          d.logger,
		MaxQueueSize:    1000,
		MaxConcurrent:   3,
		DefaultTimeout:  5 * time.Minute,
		DefaultRetries:  2,
		CleanupInterval: 1 * time.Hour,
	}
	d.taskQueue = queue.NewQueue(queueConfig)
	d.logger.Info("Task Queue initialized (will start after PocketBase is ready)")

	// Initialize API Handlers
	d.apiHandlers = api.NewAPIHandlers(d.pb, d.llmRegistry, d.taskQueue)

	// Register API routes via OnServe hook
	d.pb.OnServe().BindFunc(func(e *core.ServeEvent) error {
		d.apiHandlers.RegisterAPIRoutes(e.Router)
		d.logger.Info("FLIP2 API routes registered",
			"endpoints", []string{
				"/api/agents", "/api/tasks", "/api/llm/invoke",
				"/api/llm/backends", "/api/signals", "/api/flip/health",
			})
		return e.Next()
	})

	// Start Task Queue after PocketBase is ready (deferred via OnServe)
	d.pb.OnServe().BindFunc(func(e *core.ServeEvent) error {
		d.taskQueue.Start()
		d.logger.Info("Task Queue started via OnServe hook")
		return e.Next()
	})
}

// initializeArchiver sets up the message archiver component
func (d *Daemon) initializeArchiver() {
	if !d.config.Flip2.Archiver.Enabled {
		d.logger.Info("Message archiver disabled")
		return
	}

	// Create PBStore adapter for archiver
	store := archiver.NewPBStore(d.pb)

	// Create archiver config from daemon config
	archiverConfig := archiver.Config{
		Enabled:             d.config.Flip2.Archiver.Enabled,
		ActiveRetentionDays: d.config.Flip2.Archiver.ActiveRetentionDays,
		RecentRetentionDays: d.config.Flip2.Archiver.RecentRetentionDays,
		CheckInterval:       d.config.Flip2.Archiver.CheckInterval,
		BatchSize:           d.config.Flip2.Archiver.BatchSize,
		ActiveAgents:        d.config.Flip2.Archiver.ActiveAgents,
		DeprecatedAgents:    d.config.Flip2.Archiver.DeprecatedAgents,
		ArchivePath:         d.config.Flip2.Archiver.ArchivePath,
	}

	// Create archiver
	d.msgArchiver = archiver.New(archiverConfig, store, d.logger)

	// Start archiver via OnServe hook (after PocketBase is ready)
	d.pb.OnServe().BindFunc(func(e *core.ServeEvent) error {
		ctx := context.Background()
		d.msgArchiver.Start(ctx)
		d.logger.Info("Message archiver started via OnServe hook",
			"check_interval", archiverConfig.CheckInterval,
			"active_retention_days", archiverConfig.ActiveRetentionDays)
		return e.Next()
	})

	d.logger.Info("Message archiver initialized",
		"active_agents", archiverConfig.ActiveAgents,
		"deprecated_agents", archiverConfig.DeprecatedAgents)
}
