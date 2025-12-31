// Package api provides REST API handlers for FLIP2.
package api

import (
	"encoding/json"
	"net/http"
	"time"

	"flip2/internal/llm"
	"flip2/internal/queue"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// APIHandlers provides HTTP handlers for the FLIP2 API
type APIHandlers struct {
	pb       *pocketbase.PocketBase
	registry *llm.Registry
	queue    *queue.Queue
}

// NewAPIHandlers creates a new API handlers instance
func NewAPIHandlers(pb *pocketbase.PocketBase, registry *llm.Registry, q *queue.Queue) *APIHandlers {
	return &APIHandlers{
		pb:       pb,
		registry: registry,
		queue:    q,
	}
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

	return e.JSON(http.StatusOK, map[string]interface{}{
		"backend":       backend.Name(),
		"model":         response.Model,
		"content":       response.Content,
		"input_tokens":  response.InputTokens,
		"output_tokens": response.OutputTokens,
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

	records, err := h.pb.FindRecordsByFilter("signals", filter, "-created", 50, 0)
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
