// Package api provides REST API handlers for FLIP2.
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"flip2/internal/costtracker"
	"flip2/internal/budget"
	"flip2/internal/llm"
	"flip2/internal/queue"
	"flip2/internal/session"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// APIHandlers provides HTTP handlers for the FLIP2 API
type APIHandlers struct {
	pb                   *pocketbase.PocketBase
	registry             *llm.Registry
	queue                *queue.Queue
	costTracker          *costtracker.Tracker
	budgetTracker        *budget.Tracker
	sessionMgr           *session.SessionManager
	reconnectionMgr      *session.ReconnectionManager
}

// NewAPIHandlers creates a new API handlers instance
func NewAPIHandlers(pb *pocketbase.PocketBase, registry *llm.Registry, q *queue.Queue, ct *costtracker.Tracker, bt *budget.Tracker) *APIHandlers {
	return &APIHandlers{
		pb:          pb,
		registry:    registry,
		queue:       q,
		costTracker: ct,
		budgetTracker: bt,
	}
}

// SetCostTracker sets the cost tracker for the API handlers
func (h *APIHandlers) SetCostTracker(ct *costtracker.Tracker) {
	h.costTracker = ct
}

// SetSessionManager sets the session manager for the API handlers
func (h *APIHandlers) SetSessionManager(sm *session.SessionManager) {
	h.sessionMgr = sm
}

// SetReconnectionManager sets the reconnection manager for the API handlers
func (h *APIHandlers) SetReconnectionManager(rm *session.ReconnectionManager) {
	h.reconnectionMgr = rm
}

// === Agent Endpoints ===

// AgentInfo represents agent information
type AgentInfo struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Type        string    `json:"type"`
	Status      string    `json:"status"`
	LastSeen    time.Time `json:"last_seen"`
	Capabilities []string `json:"capabilities,omitempty"`
}

// HandleListAgents returns all registered agents
func (h *APIHandlers) HandleListAgents(e *core.RequestEvent) error {
	records, err := h.pb.FindRecordsByFilter("agents", "", "", 100, 0)
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to fetch agents",
		})
	}

	agents := make([]AgentInfo, 0, len(records))
	for _, r := range records {
		agents = append(agents, AgentInfo{
			ID:       r.Id,
			Name:     r.GetString("name"),
			Type:     r.GetString("type"),
			Status:   r.GetString("status"),
			LastSeen: r.GetDateTime("last_seen").Time(),
		})
	}

	return e.JSON(http.StatusOK, map[string]interface{}{
		"agents": agents,
		"count":  len(agents),
	})
}

// HandleGetAgent returns a specific agent
func (h *APIHandlers) HandleGetAgent(e *core.RequestEvent) error {
	agentID := e.Request.PathValue("id")
	if agentID == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{
			"error": "Agent ID required",
		})
	}

	record, err := h.pb.FindRecordById("agents", agentID)
	if err != nil {
		return e.JSON(http.StatusNotFound, map[string]string{
			"error": "Agent not found",
		})
	}

	agent := AgentInfo{
		ID:       record.Id,
		Name:     record.GetString("name"),
		Type:     record.GetString("type"),
		Status:   record.GetString("status"),
		LastSeen: record.GetDateTime("last_seen").Time(),
	}

	return e.JSON(http.StatusOK, agent)
}

// RegisterAgentRequest is the request body for agent registration
type RegisterAgentRequest struct {
	Name         string   `json:"name"`
	Type         string   `json:"type"`
	Capabilities []string `json:"capabilities,omitempty"`
}

// HandleRegisterAgent registers a new agent
func (h *APIHandlers) HandleRegisterAgent(e *core.RequestEvent) error {
	var req RegisterAgentRequest
	if err := json.NewDecoder(e.Request.Body).Decode(&req); err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{
			"error": "Invalid request body",
		})
	}

	if req.Name == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{
			"error": "Agent name required",
		})
	}

	collection, err := h.pb.FindCollectionByNameOrId("agents")
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Agents collection not found",
		})
	}

	record := core.NewRecord(collection)
	record.Set("name", req.Name)
	record.Set("type", req.Type)
	record.Set("status", "online")
	record.Set("last_seen", time.Now())

	if err := h.pb.Save(record); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to register agent",
		})
	}

	return e.JSON(http.StatusCreated, map[string]interface{}{
		"id":      record.Id,
		"name":    req.Name,
		"status":  "online",
		"message": "Agent registered successfully",
	})
}

// HandleAgentHeartbeat updates agent's last seen time
func (h *APIHandlers) HandleAgentHeartbeat(e *core.RequestEvent) error {
	agentID := e.Request.PathValue("id")
	if agentID == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{
			"error": "Agent ID required",
		})
	}

	record, err := h.pb.FindRecordById("agents", agentID)
	if err != nil {
		return e.JSON(http.StatusNotFound, map[string]string{
			"error": "Agent not found",
		})
	}

	record.Set("last_seen", time.Now())
	record.Set("status", "online")

	if err := h.pb.Save(record); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to update heartbeat",
		})
	}

	return e.JSON(http.StatusOK, map[string]string{
		"status":  "ok",
		"message": "Heartbeat recorded",
	})
}

// === Task Endpoints ===

// TaskRequest is the request body for task submission
type TaskRequest struct {
	Title       string            `json:"title"`
	Description string            `json:"description,omitempty"`
	Backend     string            `json:"backend,omitempty"`
	Priority    int               `json:"priority,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// HandleSubmitTask submits a new task to the queue
func (h *APIHandlers) HandleSubmitTask(e *core.RequestEvent) error {
	var req TaskRequest
	if err := json.NewDecoder(e.Request.Body).Decode(&req); err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{
			"error": "Invalid request body",
		})
	}

	if req.Title == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{
			"error": "Task title required",
		})
	}

	if h.queue == nil {
		return e.JSON(http.StatusServiceUnavailable, map[string]string{
			"error": "Task queue not available",
		})
	}

	task := &queue.Task{
		ID:          generateTaskID(),
		Title:       req.Title,
		Description: req.Description,
		Backend:     req.Backend,
		Priority:    req.Priority,
		Metadata:    req.Metadata,
	}

	if err := h.queue.Submit(task); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
	}

	return e.JSON(http.StatusAccepted, map[string]interface{}{
		"task_id":  task.ID,
		"status":   "pending",
		"message":  "Task submitted successfully",
		"priority": task.Priority,
	})
}

// HandleGetTask returns task status and result
func (h *APIHandlers) HandleGetTask(e *core.RequestEvent) error {
	taskID := e.Request.PathValue("id")
	if taskID == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{
			"error": "Task ID required",
		})
	}

	if h.queue == nil {
		return e.JSON(http.StatusServiceUnavailable, map[string]string{
			"error": "Task queue not available",
		})
	}

	task, result, found := h.queue.GetTask(taskID)
	if !found {
		return e.JSON(http.StatusNotFound, map[string]string{
			"error": "Task not found",
		})
	}

	response := map[string]interface{}{
		"task_id": taskID,
	}

	if task != nil {
		response["status"] = string(task.Status)
		response["title"] = task.Title
		response["priority"] = task.Priority
		response["created_at"] = task.CreatedAt
		if task.StartedAt != nil {
			response["started_at"] = task.StartedAt
		}
	}

	if result != nil {
		response["status"] = "completed"
		if result.Success {
			response["output"] = result.Output
		} else {
			response["error"] = result.Error
		}
		response["duration"] = result.Duration.String()
	}

	return e.JSON(http.StatusOK, response)
}

// HandleCancelTask cancels a pending or running task
func (h *APIHandlers) HandleCancelTask(e *core.RequestEvent) error {
	taskID := e.Request.PathValue("id")
	if taskID == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{
			"error": "Task ID required",
		})
	}

	if h.queue == nil {
		return e.JSON(http.StatusServiceUnavailable, map[string]string{
			"error": "Task queue not available",
		})
	}

	if err := h.queue.Cancel(taskID); err != nil {
		return e.JSON(http.StatusNotFound, map[string]string{
			"error": err.Error(),
		})
	}

	return e.JSON(http.StatusOK, map[string]string{
		"task_id": taskID,
		"status":  "cancelled",
		"message": "Task cancelled successfully",
	})
}

// HandleListTasks returns pending and running tasks
func (h *APIHandlers) HandleListTasks(e *core.RequestEvent) error {
	if h.queue == nil {
		return e.JSON(http.StatusServiceUnavailable, map[string]string{
			"error": "Task queue not available",
		})
	}

	pending := h.queue.ListPending()
	running := h.queue.ListRunning()
	stats := h.queue.GetStats()

	return e.JSON(http.StatusOK, map[string]interface{}{
		"pending": pending,
		"running": running,
		"stats":   stats,
	})
}

// === LLM Endpoints ===

// LLMRequest is the request body for LLM invocation
type LLMRequest struct {
	Prompt   string            `json:"prompt"`
	Backend  string            `json:"backend,omitempty"`
	Model    string            `json:"model,omitempty"`
	Options  map[string]string `json:"options,omitempty"`
	Timeout  int               `json:"timeout,omitempty"` // seconds
}

// HandleInvokeLLM invokes an LLM backend
func (h *APIHandlers) HandleInvokeLLM(e *core.RequestEvent) error {
	var req LLMRequest
	if err := json.NewDecoder(e.Request.Body).Decode(&req); err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{
			"error": "Invalid request body",
		})
	}

	if req.Prompt == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{
			"error": "Prompt required",
		})
	}

	if h.registry == nil {
		return e.JSON(http.StatusServiceUnavailable, map[string]string{
			"error": "LLM registry not available",
		})
	}

	// Get backend
	var backend llm.Backend
	var found bool
	if req.Backend != "" {
		backend, found = h.registry.Get(req.Backend)
		if !found {
			return e.JSON(http.StatusBadRequest, map[string]string{
				"error": "Backend not found: " + req.Backend,
			})
		}
	} else {
		backend = h.registry.GetBestAvailable(e.Request.Context(), "default")
		if backend == nil {
			return e.JSON(http.StatusServiceUnavailable, map[string]string{
				"error": "No LLM backends available",
			})
		}
	}

	// Check budget if tracker is available
	if h.budgetTracker != nil {
		// Get agent ID from request header or use "api" as default
		agentID := e.Request.Header.Get("X-Agent-ID")
		if agentID == "" {
			agentID = "api"
		}
		
		// Check if budget allows execution (passing 0.0 as we don't know cost yet)
		allowed, reason, err := h.budgetTracker.CheckBudget(e.Request.Context(), agentID, req.Model, 0.0)
		if err != nil {
			// Log error but proceed? Or fail? Let's log and proceed for resilience, unless it's critical.
			// But for budget enforcement, we should probably fail if check fails?
			// Let's assume fail on error for strictness.
			return e.JSON(http.StatusInternalServerError, map[string]string{
				"error": "Budget check failed: " + err.Error(),
			})
		}
		if !allowed {
			return e.JSON(http.StatusForbidden, map[string]string{
				"error": "Budget exceeded: " + reason,
			})
		}
	}

	// Execute
	opts := &llm.Options{
		Model: req.Model,
	}

	response, err := backend.Execute(e.Request.Context(), req.Prompt, opts)
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{
			"error":   "LLM execution failed",
			"details": err.Error(),
		})
	}

	// Record cost if tracker is available
	if h.costTracker != nil {
		// Get agent ID from request header or use "api" as default
		agentID := e.Request.Header.Get("X-Agent-ID")
		if agentID == "" {
			agentID = "api"
		}

		// Record the cost (use empty string for task_id since this is a direct API call)
		if err := h.costTracker.RecordCost(
			context.Background(),
			agentID,
			response.Model,
			"", // No task_id for direct API calls
			response.InputTokens,
			response.OutputTokens,
			response.CostUSD,
		); err != nil {
			// Log error but don't fail the request
			// The cost tracking is non-critical for API functionality
		}
		
		// Also record against budget
		if h.budgetTracker != nil {
			if err := h.budgetTracker.RecordSpend(
				context.Background(),
				agentID,
				response.Model,
				"", // No task_id
				response.CostUSD,
				response.InputTokens,
				response.OutputTokens,
			); err != nil {
				// Log error
			}
		}
	}

	return e.JSON(http.StatusOK, map[string]interface{}{
		"backend":       backend.Name(),
		"model":         response.Model,
		"content":       response.Content,
		"input_tokens":  response.InputTokens,
		"output_tokens": response.OutputTokens,
		"cost_usd":      response.CostUSD,
		"latency":       response.Latency.String(),
	})
}

// HandleListBackends returns available LLM backends
func (h *APIHandlers) HandleListBackends(e *core.RequestEvent) error {
	if h.registry == nil {
		return e.JSON(http.StatusServiceUnavailable, map[string]string{
			"error": "LLM registry not available",
		})
	}

	backends := h.registry.List()
	result := make([]map[string]interface{}, 0, len(backends))

	for _, name := range backends {
		backend, _ := h.registry.Get(name)
		if backend != nil {
			result = append(result, map[string]interface{}{
				"name":          backend.Name(),
				"default_model": backend.DefaultModel(),
				"models":        backend.Models(),
				"available":     backend.IsAvailable(e.Request.Context()),
			})
		}
	}

	return e.JSON(http.StatusOK, map[string]interface{}{
		"backends": result,
		"count":    len(result),
	})
}

// === Health Endpoints ===

// HandleHealth returns service health status
func (h *APIHandlers) HandleHealth(e *core.RequestEvent) error {
	status := "healthy"
	checks := make(map[string]string)

	// Check PocketBase
	if h.pb != nil {
		checks["pocketbase"] = "ok"
	} else {
		checks["pocketbase"] = "unavailable"
		status = "degraded"
	}

	// Check Queue
	if h.queue != nil {
		stats := h.queue.GetStats()
		checks["queue"] = "ok"
		checks["queue_pending"] = string(rune(stats["pending"].(int)))
	} else {
		checks["queue"] = "unavailable"
	}

	// Check LLM Registry
	if h.registry != nil {
		backends := h.registry.List()
		checks["llm_backends"] = string(rune(len(backends)))
	} else {
		checks["llm_registry"] = "unavailable"
	}

	return e.JSON(http.StatusOK, map[string]interface{}{
		"status":    status,
		"checks":    checks,
		"timestamp": time.Now(),
	})
}

// === Signal Endpoints ===

// SignalRequest is the request body for sending signals
type SignalRequest struct {
	ToAgent   string `json:"to_agent"`
	Content   string `json:"content"`
	Priority  string `json:"priority,omitempty"`
	SignalType string `json:"signal_type,omitempty"`
}

// HandleSendSignal sends a signal to another agent
func (h *APIHandlers) HandleSendSignal(e *core.RequestEvent) error {
	var req SignalRequest
	if err := json.NewDecoder(e.Request.Body).Decode(&req); err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{
			"error": "Invalid request body",
		})
	}

	if req.ToAgent == "" || req.Content == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{
			"error": "to_agent and content required",
		})
	}

	// Get from_agent from request header or default
	fromAgent := e.Request.Header.Get("X-Agent-ID")
	if fromAgent == "" {
		fromAgent = "api"
	}

	collection, err := h.pb.FindCollectionByNameOrId("signals")
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Signals collection not found",
		})
	}

	record := core.NewRecord(collection)
	record.Set("signal_id", "API-"+generateTaskID())
	record.Set("from_agent", fromAgent)
	record.Set("to_agent", req.ToAgent)
	record.Set("content", req.Content)
	record.Set("signal_type", cond(req.SignalType != "", req.SignalType, "message"))
	record.Set("priority", cond(req.Priority != "", req.Priority, "normal"))
	record.Set("read", false)

	if err := h.pb.Save(record); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to send signal",
		})
	}

	return e.JSON(http.StatusCreated, map[string]interface{}{
		"signal_id": record.GetString("signal_id"),
		"to_agent":  req.ToAgent,
		"status":    "sent",
	})
}

// HandleGetSignals returns signals for an agent
func (h *APIHandlers) HandleGetSignals(e *core.RequestEvent) error {
	agentID := e.Request.URL.Query().Get("agent")
	unreadOnly := e.Request.URL.Query().Get("unread") == "true"

	filter := ""
	if agentID != "" {
		filter = "to_agent = '" + agentID + "'"
		if unreadOnly {
			filter += " && read = false"
		}
	}

	records, err := h.pb.FindRecordsByFilter("signals", filter, "-@created", 50, 0)
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to fetch signals",
		})
	}

	signals := make([]map[string]interface{}, 0, len(records))
	for _, r := range records {
		signals = append(signals, map[string]interface{}{
			"id":          r.Id,
			"signal_id":   r.GetString("signal_id"),
			"from_agent":  r.GetString("from_agent"),
			"to_agent":    r.GetString("to_agent"),
			"content":     r.GetString("content"),
			"signal_type": r.GetString("signal_type"),
			"priority":    r.GetString("priority"),
			"read":        r.GetBool("read"),
		})
	}

	return e.JSON(http.StatusOK, map[string]interface{}{
		"signals": signals,
		"count":   len(signals),
	})
}

// === Helpers ===

func generateTaskID() string {
	return time.Now().Format("20060102-150405") + "-" + randomString(6)
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
		time.Sleep(time.Nanosecond)
	}
	return string(b)
}

func cond(condition bool, ifTrue, ifFalse string) string {
	if condition {
		return ifTrue
	}
	return ifFalse
}

// === Cost Stats Endpoints ===

// HandleGetCostSummary returns overall cost summary
func (h *APIHandlers) HandleGetCostSummary(e *core.RequestEvent) error {
	if h.costTracker == nil {
		return e.JSON(http.StatusServiceUnavailable, map[string]string{
			"error": "Cost tracker not available",
		})
	}

	// Parse days parameter (default: 1 day)
	daysStr := e.Request.URL.Query().Get("days")
	days := 1
	if daysStr != "" {
		parsedDays, err := strconv.Atoi(daysStr)
		if err != nil || parsedDays < 1 {
			return e.JSON(http.StatusBadRequest, map[string]string{
				"error": "Invalid days parameter (must be positive integer)",
			})
		}
		days = parsedDays
	}

	// Calculate date range
	end := time.Now()
	start := end.Add(-time.Duration(days) * 24 * time.Hour)

	// Get summary from cost tracker
	summary, err := h.costTracker.GetSummary(e.Request.Context(), start, end)
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{
			"error":   "Failed to retrieve cost summary",
			"details": err.Error(),
		})
	}

	return e.JSON(http.StatusOK, summary)
}

// HandleGetCostsByAgent returns costs for a specific agent
func (h *APIHandlers) HandleGetCostsByAgent(e *core.RequestEvent) error {
	if h.costTracker == nil {
		return e.JSON(http.StatusServiceUnavailable, map[string]string{
			"error": "Cost tracker not available",
		})
	}

	// Get agent_id from path
	agentID := e.Request.PathValue("agent_id")
	if agentID == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{
			"error": "Agent ID required",
		})
	}

	// Parse days parameter (default: 1 day)
	daysStr := e.Request.URL.Query().Get("days")
	days := 1
	if daysStr != "" {
		parsedDays, err := strconv.Atoi(daysStr)
		if err != nil || parsedDays < 1 {
			return e.JSON(http.StatusBadRequest, map[string]string{
				"error": "Invalid days parameter (must be positive integer)",
			})
		}
		days = parsedDays
	}

	// Calculate date range
	end := time.Now()
	start := end.Add(-time.Duration(days) * 24 * time.Hour)

	// Get costs by agent
	costs, err := h.costTracker.GetAgentCosts(e.Request.Context(), agentID, start, end)
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{
			"error":   "Failed to retrieve agent costs",
			"details": err.Error(),
		})
	}

	return e.JSON(http.StatusOK, map[string]interface{}{
		"agent_id": agentID,
		"days":     days,
		"costs":    costs,
		"count":    len(costs),
	})
}

// HandleGetCostsByModel returns costs for a specific model
func (h *APIHandlers) HandleGetCostsByModel(e *core.RequestEvent) error {
	if h.costTracker == nil {
		return e.JSON(http.StatusServiceUnavailable, map[string]string{
			"error": "Cost tracker not available",
		})
	}

	// Get model from path
	model := e.Request.PathValue("model")
	if model == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{
			"error": "Model name required",
		})
	}

	// Parse days parameter (default: 1 day)
	daysStr := e.Request.URL.Query().Get("days")
	days := 1
	if daysStr != "" {
		parsedDays, err := strconv.Atoi(daysStr)
		if err != nil || parsedDays < 1 {
			return e.JSON(http.StatusBadRequest, map[string]string{
				"error": "Invalid days parameter (must be positive integer)",
			})
		}
		days = parsedDays
	}

	// Calculate date range
	end := time.Now()
	start := end.Add(-time.Duration(days) * 24 * time.Hour)

	// Get costs by model
	costs, err := h.costTracker.GetModelCosts(e.Request.Context(), model, start, end)
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{
			"error":   "Failed to retrieve model costs",
			"details": err.Error(),
		})
	}

	return e.JSON(http.StatusOK, map[string]interface{}{
		"model": model,
		"days":  days,
		"costs": costs,
		"count": len(costs),
	})
}

// === Agent Reconnection Endpoints ===

// ReconnectAgentRequest is the request body for agent reconnection
type ReconnectAgentRequest struct {
	AgentID   string `json:"agent_id"`
	SessionID string `json:"session_id"`
}

// HandleReconnectAgent triggers reconnection for a specific agent
func (h *APIHandlers) HandleReconnectAgent(e *core.RequestEvent) error {
	if h.reconnectionMgr == nil {
		return e.JSON(http.StatusServiceUnavailable, map[string]string{
			"error": "Reconnection manager not available",
		})
	}

	var req ReconnectAgentRequest
	if err := json.NewDecoder(e.Request.Body).Decode(&req); err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{
			"error": "Invalid request body",
		})
	}

	if req.AgentID == "" || req.SessionID == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{
			"error": "agent_id and session_id are required",
		})
	}

	// Start reconnection (async by default, but we'll wait for result)
	result, err := h.reconnectionMgr.ReconnectAgent(e.Request.Context(), req.SessionID, req.AgentID)
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{
			"error":   "Failed to start reconnection",
			"details": err.Error(),
		})
	}

	statusCode := http.StatusOK
	if !result.Success {
		statusCode = http.StatusConflict
	}

	return e.JSON(statusCode, map[string]interface{}{
		"agent_id": result.AgentID,
		"success":  result.Success,
		"state":    result.State,
		"duration": result.Duration.String(),
		"error":    result.Error,
	})
}

// HandleReconnectSession triggers reconnection for all agents in a session
func (h *APIHandlers) HandleReconnectSession(e *core.RequestEvent) error {
	if h.reconnectionMgr == nil || h.sessionMgr == nil {
		return e.JSON(http.StatusServiceUnavailable, map[string]string{
			"error": "Reconnection or session manager not available",
		})
	}

	sessionID := e.Request.PathValue("id")
	if sessionID == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{
			"error": "Session ID required",
		})
	}

	// Get the session
	sess, err := h.sessionMgr.GetSession(e.Request.Context(), sessionID)
	if err != nil {
		return e.JSON(http.StatusNotFound, map[string]string{
			"error":   "Session not found",
			"details": err.Error(),
		})
	}

	// Reconnect all agents
	results, err := h.reconnectionMgr.ReconnectAllAgents(e.Request.Context(), sess)
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{
			"error":   "Failed to reconnect agents",
			"details": err.Error(),
		})
	}

	// Count results
	succeeded := 0
	failed := 0
	for _, r := range results {
		if r.Success {
			succeeded++
		} else {
			failed++
		}
	}

	return e.JSON(http.StatusOK, map[string]interface{}{
		"session_id": sessionID,
		"succeeded":  succeeded,
		"failed":     failed,
		"total":      len(results),
		"results":    results,
	})
}

// HandleGetReconnectionState returns the state of an active reconnection
func (h *APIHandlers) HandleGetReconnectionState(e *core.RequestEvent) error {
	if h.reconnectionMgr == nil {
		return e.JSON(http.StatusServiceUnavailable, map[string]string{
			"error": "Reconnection manager not available",
		})
	}

	agentID := e.Request.PathValue("agent_id")
	if agentID == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{
			"error": "Agent ID required",
		})
	}

	state, exists := h.reconnectionMgr.GetReconnectionState(agentID)
	if !exists {
		return e.JSON(http.StatusNotFound, map[string]string{
			"error": "No active reconnection for agent",
		})
	}

	return e.JSON(http.StatusOK, state)
}

// HandleListActiveReconnections returns all active reconnection states
func (h *APIHandlers) HandleListActiveReconnections(e *core.RequestEvent) error {
	if h.reconnectionMgr == nil {
		return e.JSON(http.StatusServiceUnavailable, map[string]string{
			"error": "Reconnection manager not available",
		})
	}

	states := h.reconnectionMgr.ListActiveReconnections()

	return e.JSON(http.StatusOK, map[string]interface{}{
		"reconnections": states,
		"count":         len(states),
	})
}

// HandleCancelReconnection cancels an active reconnection attempt
func (h *APIHandlers) HandleCancelReconnection(e *core.RequestEvent) error {
	if h.reconnectionMgr == nil {
		return e.JSON(http.StatusServiceUnavailable, map[string]string{
			"error": "Reconnection manager not available",
		})
	}

	agentID := e.Request.PathValue("agent_id")
	if agentID == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{
			"error": "Agent ID required",
		})
	}

	if err := h.reconnectionMgr.CancelReconnection(agentID); err != nil {
		return e.JSON(http.StatusNotFound, map[string]string{
			"error":   "Failed to cancel reconnection",
			"details": err.Error(),
		})
	}

	return e.JSON(http.StatusOK, map[string]string{
		"agent_id": agentID,
		"status":   "cancelled",
		"message":  "Reconnection cancelled successfully",
	})
}

// === Session Endpoints ===

// HandleListSessions returns all sessions
func (h *APIHandlers) HandleListSessions(e *core.RequestEvent) error {
	if h.sessionMgr == nil {
		return e.JSON(http.StatusServiceUnavailable, map[string]string{
			"error": "Session manager not available",
		})
	}

	coordinatorID := e.Request.URL.Query().Get("coordinator_id")
	status := e.Request.URL.Query().Get("status")

	sessions, err := h.sessionMgr.ListSessions(e.Request.Context(), coordinatorID, session.SessionStatus(status))
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{
			"error":   "Failed to list sessions",
			"details": err.Error(),
		})
	}

	return e.JSON(http.StatusOK, map[string]interface{}{
		"sessions": sessions,
		"count":    len(sessions),
	})
}

// HandleStartSession starts a new session
func (h *APIHandlers) HandleStartSession(e *core.RequestEvent) error {
	if h.sessionMgr == nil {
		return e.JSON(http.StatusServiceUnavailable, map[string]string{
			"error": "Session manager not available",
		})
	}

	var req struct {
		Name        string `json:"name"`
		Coordinator string `json:"coordinator"`
		Description string `json:"description"`
	}

	if err := json.NewDecoder(e.Request.Body).Decode(&req); err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{
			"error": "Invalid request body",
		})
	}

	if req.Name == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{
			"error": "Session name is required",
		})
	}

	if req.Coordinator == "" {
		req.Coordinator = "api-coordinator"
	}

	sess, err := h.sessionMgr.StartSession(e.Request.Context(), req.Name, req.Coordinator)
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{
			"error":   "Failed to start session",
			"details": err.Error(),
		})
	}

	return e.JSON(http.StatusCreated, map[string]interface{}{
		"session": sess,
		"message": "Session started successfully",
	})
}

// HandleAttachSession attaches to an existing session
func (h *APIHandlers) HandleAttachSession(e *core.RequestEvent) error {
	if h.sessionMgr == nil {
		return e.JSON(http.StatusServiceUnavailable, map[string]string{
			"error": "Session manager not available",
		})
	}

	var req struct {
		NameOrID string `json:"name_or_id"`
	}

	if err := json.NewDecoder(e.Request.Body).Decode(&req); err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{
			"error": "Invalid request body",
		})
	}

	if req.NameOrID == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{
			"error": "Session name or ID is required",
		})
	}

	// Try to get by ID first
	sess, err := h.sessionMgr.GetSession(e.Request.Context(), req.NameOrID)
	if err != nil {
		// Try to find by name
		sessions, listErr := h.sessionMgr.ListSessions(e.Request.Context(), "", "")
		if listErr != nil {
			return e.JSON(http.StatusNotFound, map[string]string{
				"error": "Session not found",
			})
		}
		for _, s := range sessions {
			if s.Name == req.NameOrID {
				sess = s
				break
			}
		}
		if sess == nil {
			return e.JSON(http.StatusNotFound, map[string]string{
				"error": "Session not found",
			})
		}
	}

	// Get coordinator from header or use default
	coordinatorID := e.Request.Header.Get("X-Coordinator-ID")
	if coordinatorID == "" {
		coordinatorID = sess.CoordinatorID
	}

	// Attach to session
	attachedSess, err := h.sessionMgr.AttachSession(e.Request.Context(), sess.ID, coordinatorID)
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{
			"error":   "Failed to attach to session",
			"details": err.Error(),
		})
	}

	return e.JSON(http.StatusOK, map[string]interface{}{
		"session": attachedSess,
		"message": "Session attached successfully",
	})
}

// HandleStopSession stops a running session
func (h *APIHandlers) HandleStopSession(e *core.RequestEvent) error {
	if h.sessionMgr == nil {
		return e.JSON(http.StatusServiceUnavailable, map[string]string{
			"error": "Session manager not available",
		})
	}

	var req struct {
		SessionID   string `json:"session_id"`
		FinalStatus string `json:"final_status"`
	}

	if err := json.NewDecoder(e.Request.Body).Decode(&req); err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{
			"error": "Invalid request body",
		})
	}

	if req.SessionID == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{
			"error": "Session ID is required",
		})
	}

	if req.FinalStatus == "" {
		req.FinalStatus = "completed"
	}

	err := h.sessionMgr.StopSession(e.Request.Context(), req.SessionID, session.SessionStatus(req.FinalStatus))
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{
			"error":   "Failed to stop session",
			"details": err.Error(),
		})
	}

	return e.JSON(http.StatusOK, map[string]string{
		"session_id": req.SessionID,
		"status":     req.FinalStatus,
		"message":    "Session stopped successfully",
	})
}

// === Budget Endpoints ===

// BudgetRequest represents a request to create/update a budget
type BudgetRequest struct {
	Name        string             `json:"name"`
	Scope       string             `json:"scope"`
	ScopeID     string             `json:"scope_id"`
	LimitUSD    float64            `json:"limit_usd"`
	ResetPeriod budget.ResetPeriod `json:"reset_period"`
}

// HandleCreateBudget creates a new budget
func (h *APIHandlers) HandleCreateBudget(e *core.RequestEvent) error {
	if h.budgetTracker == nil {
		return e.JSON(http.StatusServiceUnavailable, map[string]string{
			"error": "Budget tracker not available",
		})
	}

	var req BudgetRequest
	if err := json.NewDecoder(e.Request.Body).Decode(&req); err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{
			"error": "Invalid request body",
		})
	}

	scope := budget.BudgetScope(req.Scope)
	if !scope.IsValid() {
		return e.JSON(http.StatusBadRequest, map[string]string{
			"error": "Invalid scope (must be agent, model, or global)",
		})
	}

	if !req.ResetPeriod.IsValid() {
		return e.JSON(http.StatusBadRequest, map[string]string{
			"error": "Invalid reset period (must be hourly, daily, weekly, or monthly)",
		})
	}

	b, err := h.budgetTracker.CreateBudget(
		e.Request.Context(),
		req.Name,
		scope,
		req.ScopeID,
		req.LimitUSD,
		req.ResetPeriod,
	)
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to create budget: " + err.Error(),
		})
	}

	return e.JSON(http.StatusCreated, b)
}

// HandleListBudgets returns all budgets
func (h *APIHandlers) HandleListBudgets(e *core.RequestEvent) error {
	if h.budgetTracker == nil {
		return e.JSON(http.StatusServiceUnavailable, map[string]string{
			"error": "Budget tracker not available",
		})
	}

	budgets, err := h.budgetTracker.ListBudgets(e.Request.Context())
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to list budgets: " + err.Error(),
		})
	}

	return e.JSON(http.StatusOK, map[string]interface{}{
		"budgets": budgets,
		"count":   len(budgets),
	})
}

// HandleGetBudget returns a specific budget
func (h *APIHandlers) HandleGetBudget(e *core.RequestEvent) error {
	if h.budgetTracker == nil {
		return e.JSON(http.StatusServiceUnavailable, map[string]string{
			"error": "Budget tracker not available",
		})
	}

	id := e.Request.PathValue("id")
	if id == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{
			"error": "Budget ID required",
		})
	}

	b, err := h.budgetTracker.GetBudget(e.Request.Context(), id)
	if err != nil {
		return e.JSON(http.StatusNotFound, map[string]string{
			"error": "Budget not found",
		})
	}

	return e.JSON(http.StatusOK, b)
}

// HandleUpdateBudget updates a budget limit
func (h *APIHandlers) HandleUpdateBudget(e *core.RequestEvent) error {
	if h.budgetTracker == nil {
		return e.JSON(http.StatusServiceUnavailable, map[string]string{
			"error": "Budget tracker not available",
		})
	}

	id := e.Request.PathValue("id")
	if id == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{
			"error": "Budget ID required",
		})
	}

	var req struct {
		LimitUSD float64 `json:"limit_usd"`
		Enabled  *bool   `json:"enabled,omitempty"`
	}
	if err := json.NewDecoder(e.Request.Body).Decode(&req); err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{
			"error": "Invalid request body",
		})
	}

	// Update limit
	if err := h.budgetTracker.UpdateLimit(e.Request.Context(), id, req.LimitUSD); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to update budget: " + err.Error(),
		})
	}

	// Enable/Disable if specified
	if req.Enabled != nil {
		if *req.Enabled {
			h.budgetTracker.EnableBudget(e.Request.Context(), id)
		} else {
			h.budgetTracker.DisableBudget(e.Request.Context(), id)
		}
	}

	b, _ := h.budgetTracker.GetBudget(e.Request.Context(), id)
	return e.JSON(http.StatusOK, b)
}

// HandleResetBudget manually resets a budget
func (h *APIHandlers) HandleResetBudget(e *core.RequestEvent) error {
	if h.budgetTracker == nil {
		return e.JSON(http.StatusServiceUnavailable, map[string]string{
			"error": "Budget tracker not available",
		})
	}

	id := e.Request.PathValue("id")
	if id == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{
			"error": "Budget ID required",
		})
	}

	if err := h.budgetTracker.ResetBudget(e.Request.Context(), id); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to reset budget: " + err.Error(),
		})
	}

	b, _ := h.budgetTracker.GetBudget(e.Request.Context(), id)
	return e.JSON(http.StatusOK, b)
}

// HandleGetBudgetReport returns a detailed report
func (h *APIHandlers) HandleGetBudgetReport(e *core.RequestEvent) error {
	if h.budgetTracker == nil {
		return e.JSON(http.StatusServiceUnavailable, map[string]string{
			"error": "Budget tracker not available",
		})
	}

	id := e.Request.PathValue("id")
	if id == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{
			"error": "Budget ID required",
		})
	}

	report, err := h.budgetTracker.GetReport(e.Request.Context(), id)
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Failed to generate report: " + err.Error(),
		})
	}

	return e.JSON(http.StatusOK, report)
}
