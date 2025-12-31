package main

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"flip/pkg/agent"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/spf13/cobra"
	_ "modernc.org/sqlite"
)

// --- Version ---
const FLIP_VERSION = "5.2"

// ============================================================================
// WebSocket Hub for Real-Time Agent Communication
// ============================================================================

// WSMessage represents a message sent over WebSocket
type WSMessage struct {
	Type      string      `json:"type"`       // message type: task, signal, heartbeat, status, etc.
	From      string      `json:"from"`       // sender agent ID
	To        string      `json:"to"`         // recipient agent ID (or "broadcast")
	Payload   interface{} `json:"payload"`    // message content
	ID        string      `json:"id"`         // message ID for acknowledgment
	Timestamp time.Time   `json:"timestamp"`  // when message was sent
	Priority  string      `json:"priority"`   // critical, high, normal, low
}

// ============================================================================
// Worker Pool for Rate-Limited API Calls
// ============================================================================

// APITask represents a task to be executed by the worker pool
type APITask struct {
	ID       string
	Model    string // "gemini", "claude", "gpt"
	Prompt   string
	Callback func(result string, err error)
	Retries  int
	Created  time.Time
}

// APIResult represents the result of an API call
type APIResult struct {
	TaskID  string
	Model   string
	Result  string
	Error   error
	Latency time.Duration
}

// WorkerPool manages concurrent API calls with rate limiting
type WorkerPool struct {
	tasks      chan *APITask
	results    chan *APIResult
	retryQueue chan *APITask
	workers    int
	maxRetries int
	metrics    *PoolMetrics
	mu         sync.RWMutex
}

// PoolMetrics tracks worker pool statistics
type PoolMetrics struct {
	TotalTasks     int64
	SuccessCount   int64
	ErrorCount     int64
	RetryCount     int64
	AvgLatencyMs   float64
	TasksPerMinute float64
	mu             sync.Mutex
}

// NewWorkerPool creates a new worker pool with specified size
func NewWorkerPool(size int) *WorkerPool {
	p := &WorkerPool{
		tasks:      make(chan *APITask, 100),
		results:    make(chan *APIResult, 100),
		retryQueue: make(chan *APITask, 50),
		workers:    size,
		maxRetries: 3,
		metrics:    &PoolMetrics{},
	}
	// Start workers
	for i := 0; i < size; i++ {
		go p.worker(i)
	}
	// Start retry processor
	go p.retryProcessor()
	// Start metrics collector
	go p.metricsCollector()
	return p
}

// Submit adds a task to the worker pool
func (p *WorkerPool) Submit(task *APITask) {
	task.Created = time.Now()
	p.tasks <- task
	p.metrics.mu.Lock()
	p.metrics.TotalTasks++
	p.metrics.mu.Unlock()
}

// worker processes tasks from the queue
func (p *WorkerPool) worker(id int) {
	for task := range p.tasks {
		start := time.Now()
		result, err := p.executeTask(task)
		latency := time.Since(start)

		if err != nil && task.Retries < p.maxRetries {
			// Retry with exponential backoff
			task.Retries++
			p.metrics.mu.Lock()
			p.metrics.RetryCount++
			p.metrics.mu.Unlock()
			go func(t *APITask, delay time.Duration) {
				time.Sleep(delay)
				p.retryQueue <- t
			}(task, time.Duration(1<<task.Retries)*time.Second)
		} else {
			// Send result
			apiResult := &APIResult{
				TaskID:  task.ID,
				Model:   task.Model,
				Result:  result,
				Error:   err,
				Latency: latency,
			}
			p.results <- apiResult

			// Update metrics
			p.metrics.mu.Lock()
			if err != nil {
				p.metrics.ErrorCount++
			} else {
				p.metrics.SuccessCount++
			}
			// Running average for latency
			total := p.metrics.SuccessCount + p.metrics.ErrorCount
			p.metrics.AvgLatencyMs = (p.metrics.AvgLatencyMs*float64(total-1) + float64(latency.Milliseconds())) / float64(total)
			p.metrics.mu.Unlock()

			// Call callback if provided
			if task.Callback != nil {
				task.Callback(result, err)
			}
		}
	}
}

// executeTask runs the actual API call (placeholder - integrate with existing spawn logic)
func (p *WorkerPool) executeTask(task *APITask) (string, error) {
	// This will be integrated with existing model execution
	// For now, returns placeholder
	return "", nil
}

// retryProcessor handles retried tasks
func (p *WorkerPool) retryProcessor() {
	for task := range p.retryQueue {
		p.tasks <- task
	}
}

// metricsCollector periodically calculates tasks per minute
func (p *WorkerPool) metricsCollector() {
	ticker := time.NewTicker(1 * time.Minute)
	var lastTotal int64
	for range ticker.C {
		p.metrics.mu.Lock()
		current := p.metrics.TotalTasks
		p.metrics.TasksPerMinute = float64(current - lastTotal)
		lastTotal = current
		p.metrics.mu.Unlock()
	}
}

// GetMetrics returns current pool metrics
func (p *WorkerPool) GetMetrics() PoolMetrics {
	p.metrics.mu.Lock()
	defer p.metrics.mu.Unlock()
	return *p.metrics
}

// ============================================================================
// Message Batcher for WebSocket Efficiency
// ============================================================================

// MessageBatcher batches multiple messages for efficient transmission
type MessageBatcher struct {
	messages   [][]byte
	mu         sync.Mutex
	flushChan  chan struct{}
	maxBatch   int
	flushDelay time.Duration
}

// NewMessageBatcher creates a new message batcher
func NewMessageBatcher(maxBatch int, flushDelay time.Duration) *MessageBatcher {
	b := &MessageBatcher{
		messages:   make([][]byte, 0, maxBatch),
		flushChan:  make(chan struct{}, 1),
		maxBatch:   maxBatch,
		flushDelay: flushDelay,
	}
	return b
}

// Add adds a message to the batch
func (b *MessageBatcher) Add(msg []byte) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.messages = append(b.messages, msg)
	if len(b.messages) >= b.maxBatch {
		select {
		case b.flushChan <- struct{}{}:
		default:
		}
		return true // batch is full
	}
	return false
}

// Flush returns all batched messages and clears the buffer
func (b *MessageBatcher) Flush() [][]byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.messages) == 0 {
		return nil
	}
	msgs := b.messages
	b.messages = make([][]byte, 0, b.maxBatch)
	return msgs
}

// ============================================================================
// Global Metrics Collector
// ============================================================================

// FlipMetrics tracks overall system metrics
type FlipMetrics struct {
	StartTime         time.Time
	MessagesTotal     int64
	MessagesSent      int64
	MessagesReceived  int64
	WebSocketConns    int64
	APICallsTotal     int64
	APICallsSuccess   int64
	APICallsError     int64
	AvgResponseTimeMs float64
	mu                sync.RWMutex
}

// Global metrics instance
var flipMetrics = &FlipMetrics{StartTime: time.Now()}

// RecordMessage records a message metric
func (m *FlipMetrics) RecordMessage(sent bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.MessagesTotal++
	if sent {
		m.MessagesSent++
	} else {
		m.MessagesReceived++
	}
}

// RecordAPICall records an API call metric
func (m *FlipMetrics) RecordAPICall(success bool, latencyMs float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.APICallsTotal++
	if success {
		m.APICallsSuccess++
	} else {
		m.APICallsError++
	}
	// Running average
	m.AvgResponseTimeMs = (m.AvgResponseTimeMs*float64(m.APICallsTotal-1) + latencyMs) / float64(m.APICallsTotal)
}

// GetStats returns current metrics as a map
func (m *FlipMetrics) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return map[string]interface{}{
		"uptime_seconds":       time.Since(m.StartTime).Seconds(),
		"messages_total":       m.MessagesTotal,
		"messages_sent":        m.MessagesSent,
		"messages_received":    m.MessagesReceived,
		"websocket_conns":      m.WebSocketConns,
		"api_calls_total":      m.APICallsTotal,
		"api_calls_success":    m.APICallsSuccess,
		"api_calls_error":      m.APICallsError,
		"avg_response_time_ms": m.AvgResponseTimeMs,
	}
}

// ============================================================================
// WebSocket Types
// ============================================================================

// WSClient represents a connected WebSocket client
type WSClient struct {
	AgentID    string
	Conn       *websocket.Conn
	Send       chan []byte
	Hub        *WSHub
	LastPing   time.Time
	Registered bool
}

// WSHub manages all WebSocket connections
type WSHub struct {
	Clients    map[string]*WSClient // agentID -> client
	Broadcast  chan []byte          // broadcast to all clients
	Register   chan *WSClient       // register new client
	Unregister chan *WSClient       // unregister client
	Direct     chan *WSMessage      // send to specific client
	mu         sync.RWMutex
	db         *sql.DB
}

// NewWSHub creates a new WebSocket hub
func NewWSHub(db *sql.DB) *WSHub {
	return &WSHub{
		Clients:    make(map[string]*WSClient),
		Broadcast:  make(chan []byte, 256),
		Register:   make(chan *WSClient),
		Unregister: make(chan *WSClient),
		Direct:     make(chan *WSMessage, 256),
		db:         db,
	}
}

// Run starts the hub's main loop
func (h *WSHub) Run() {
	// Start periodic signal checker (checks DB for new signals every 2s)
	go h.signalChecker()

	for {
		select {
		case client := <-h.Register:
			h.mu.Lock()
			h.Clients[client.AgentID] = client
			h.mu.Unlock()
			log.Printf("[WS] Client registered: %s", client.AgentID)

			// Update agent status in DB
			h.db.Exec(`UPDATE agents SET status = 'idle', hb = datetime('now') WHERE id = ?`, client.AgentID)

			// Send welcome message
			welcome := WSMessage{
				Type:      "welcome",
				To:        client.AgentID,
				Payload:   map[string]interface{}{"message": "Connected to FLIP WebSocket server", "version": FLIP_VERSION},
				Timestamp: time.Now(),
			}
			if data, err := json.Marshal(welcome); err == nil {
				client.Send <- data
			}

			// Notify others
			h.broadcastStatus(client.AgentID, "connected")

		case client := <-h.Unregister:
			h.mu.Lock()
			if _, ok := h.Clients[client.AgentID]; ok {
				delete(h.Clients, client.AgentID)
				close(client.Send)
				log.Printf("[WS] Client disconnected: %s", client.AgentID)

				// Update agent status
				h.db.Exec(`UPDATE agents SET status = 'offline', hb = datetime('now') WHERE id = ?`, client.AgentID)

				// Notify others
				h.broadcastStatus(client.AgentID, "disconnected")
			}
			h.mu.Unlock()

		case message := <-h.Broadcast:
			h.mu.RLock()
			for _, client := range h.Clients {
				select {
				case client.Send <- message:
				default:
					// Client buffer full, skip
				}
			}
			h.mu.RUnlock()

		case msg := <-h.Direct:
			h.mu.RLock()
			if client, ok := h.Clients[msg.To]; ok {
				if data, err := json.Marshal(msg); err == nil {
					select {
					case client.Send <- data:
					default:
						log.Printf("[WS] Failed to send to %s: buffer full", msg.To)
					}
				}
			} else {
				// Client not connected, store in DB for later delivery
				h.storeOfflineMessage(msg)
			}
			h.mu.RUnlock()
		}
	}
}

// broadcastStatus sends a status update to all connected clients
func (h *WSHub) broadcastStatus(agentID, status string) {
	msg := WSMessage{
		Type:      "agent_status",
		From:      "system",
		To:        "broadcast",
		Payload:   map[string]string{"agent": agentID, "status": status},
		Timestamp: time.Now(),
	}
	if data, err := json.Marshal(msg); err == nil {
		h.Broadcast <- data
	}
}

// storeOfflineMessage stores a message for offline clients in the DB
func (h *WSHub) storeOfflineMessage(msg *WSMessage) {
	// Handle payload - if it's already a string, use as-is, otherwise JSON marshal
	var payloadStr string
	switch p := msg.Payload.(type) {
	case string:
		payloadStr = p
	case nil:
		payloadStr = ""
	default:
		if data, err := json.Marshal(p); err == nil {
			payloadStr = string(data)
		}
	}

	log.Printf("[STORE] Offline message for %s: from=%s payload=%s", msg.To, msg.From, payloadStr)

	_, err := h.db.Exec(`
		INSERT INTO signals (id, from_agent, to_agent, msg, priority, signal_type, created_at)
		VALUES (?, ?, ?, ?, ?, ?, datetime('now'))
	`, msg.ID, msg.From, msg.To, payloadStr, msg.Priority, msg.Type)
	if err != nil {
		log.Printf("[STORE] Error storing offline message: %v", err)
	}
}

// signalChecker periodically checks DB for new signals and pushes to connected clients
func (h *WSHub) signalChecker() {
	ticker := time.NewTicker(500 * time.Millisecond) // Reduced from 2s for faster signal delivery
	defer ticker.Stop()

	for range ticker.C {
		h.mu.RLock()
		for agentID, client := range h.Clients {
			// Check for unread signals
			rows, err := h.db.Query(`
				SELECT id, from_agent, msg, priority, signal_type, subject, created_at
				FROM signals
				WHERE to_agent = ? AND read = 0
				ORDER BY CASE priority
					WHEN 'critical' THEN 1
					WHEN 'high' THEN 2
					WHEN 'normal' THEN 3
					WHEN 'low' THEN 4
				END, created_at ASC
				LIMIT 10
			`, agentID)
			if err != nil {
				continue
			}

			var signals []map[string]interface{}
			for rows.Next() {
				var id, from, msg, priority, sigType, subject, created string
				rows.Scan(&id, &from, &msg, &priority, &sigType, &subject, &created)
				signals = append(signals, map[string]interface{}{
					"id":       id,
					"from":     from,
					"message":  msg,
					"priority": priority,
					"type":     sigType,
					"subject":  subject,
					"created":  created,
				})
				// Mark as read
				h.db.Exec(`UPDATE signals SET read = 1 WHERE id = ?`, id)
			}
			rows.Close()

			if len(signals) > 0 {
				wsMsg := WSMessage{
					Type:      "signals",
					To:        agentID,
					Payload:   signals,
					Timestamp: time.Now(),
				}
				if data, err := json.Marshal(wsMsg); err == nil {
					select {
					case client.Send <- data:
					default:
					}
				}
			}
		}
		h.mu.RUnlock()
	}
}

// SendToAgent sends a message to a specific agent
func (h *WSHub) SendToAgent(to string, msgType string, payload interface{}, priority string) error {
	msg := &WSMessage{
		Type:      msgType,
		To:        to,
		Payload:   payload,
		ID:        fmt.Sprintf("MSG-%d", time.Now().UnixNano()%1000000),
		Timestamp: time.Now(),
		Priority:  priority,
	}
	h.Direct <- msg
	return nil
}

// GetConnectedAgents returns list of currently connected agents
func (h *WSHub) GetConnectedAgents() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	agents := make([]string, 0, len(h.Clients))
	for id := range h.Clients {
		agents = append(agents, id)
	}
	return agents
}

// WebSocket upgrader
var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for now
	},
}

// handleWSConnection handles incoming WebSocket connections
func handleWSConnection(hub *WSHub, w http.ResponseWriter, r *http.Request) {
	agentID := r.URL.Query().Get("agent")
	if agentID == "" {
		http.Error(w, "agent parameter required", http.StatusBadRequest)
		return
	}

	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[WS] Upgrade error: %v", err)
		return
	}

	client := &WSClient{
		AgentID:  agentID,
		Conn:     conn,
		Send:     make(chan []byte, 256),
		Hub:      hub,
		LastPing: time.Now(),
	}

	hub.Register <- client

	// Start read/write pumps
	go client.writePump()
	go client.readPump()
}

// readPump pumps messages from the WebSocket connection to the hub
func (c *WSClient) readPump() {
	defer func() {
		c.Hub.Unregister <- c
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(64 * 1024)
	c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		c.LastPing = time.Now()
		return nil
	})

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("[WS] Read error from %s: %v", c.AgentID, err)
			}
			break
		}

		// Parse and handle incoming message
		var msg WSMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Printf("[WS] Invalid message from %s: %v", c.AgentID, err)
			continue
		}

		msg.From = c.AgentID
		msg.Timestamp = time.Now()

		// Handle message based on type
		c.handleMessage(&msg)
	}
}

// handleMessage processes incoming WebSocket messages
func (c *WSClient) handleMessage(msg *WSMessage) {
	switch msg.Type {
	case "heartbeat":
		// Update heartbeat in DB
		c.Hub.db.Exec(`UPDATE agents SET hb = datetime('now') WHERE id = ?`, c.AgentID)
		// Echo back
		response := WSMessage{
			Type:      "heartbeat_ack",
			To:        c.AgentID,
			Timestamp: time.Now(),
		}
		if data, err := json.Marshal(response); err == nil {
			c.Send <- data
		}

	case "status_update":
		// Agent status update
		if payload, ok := msg.Payload.(map[string]interface{}); ok {
			status, _ := payload["status"].(string)
			task, _ := payload["task"].(string)
			c.Hub.db.Exec(`UPDATE agents SET status = ?, task = ?, hb = datetime('now') WHERE id = ?`,
				status, task, c.AgentID)
			c.Hub.broadcastStatus(c.AgentID, status)
		}

	case "signal":
		// Forward signal to another agent
		if msg.To != "" && msg.To != "broadcast" {
			c.Hub.Direct <- msg
		} else {
			// Store in DB
			payload, _ := json.Marshal(msg.Payload)
			c.Hub.db.Exec(`
				INSERT INTO signals (id, from_agent, to_agent, msg, priority, signal_type, created_at)
				VALUES (?, ?, ?, ?, ?, 'message', datetime('now'))
			`, msg.ID, msg.From, msg.To, string(payload), msg.Priority)
		}

	case "task_complete":
		// Mark task as complete
		if payload, ok := msg.Payload.(map[string]interface{}); ok {
			taskID, _ := payload["task_id"].(string)
			c.Hub.db.Exec(`UPDATE tasks SET status = 'done', updated_at = datetime('now') WHERE id = ?`, taskID)
			// Notify coordinator
			c.Hub.SendToAgent("coordinator", "task_completed", payload, "normal")
		}

	case "response":
		// Response to a previous request (e.g., Antigravity answering a question)
		if msg.To != "" {
			c.Hub.Direct <- msg
		}

	default:
		log.Printf("[WS] Unknown message type from %s: %s", c.AgentID, msg.Type)
	}
}

// writePump pumps messages from the hub to the WebSocket connection
func (c *WSClient) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Batch queued messages
			n := len(c.Send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.Send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// Global hub instance (set when ws server starts)
var globalWSHub *WSHub

// --- Database Schema (v5.0) ---
// v5.0: Extended status system, signal priorities, heartbeat watchdog, handoffs

// Valid agent statuses (extended in v5.0)
var validAgentStatuses = map[string]bool{
	"idle":    true, // Available for new tasks
	"busy":    true, // Actively working on task
	"blocked": true, // Waiting on external input
	"testing": true, // Code complete, awaiting verification
	"review":  true, // Awaiting code review
	"error":   true, // Task failed, needs intervention
	"offline": true, // Session ended/crashed (auto-set by watchdog)
	"paused":  true, // Temporarily suspended
}

// Signal priority levels
const (
	PriorityCritical = "critical" // Agent crash, task fail, blocking - immediate push
	PriorityHigh     = "high"     // Escalation, handoff, verification - read within 15min
	PriorityNormal   = "normal"   // Status updates, progress - standard queue
	PriorityLow      = "low"      // FYI, logging - read when convenient
)

var validPriorities = map[string]bool{
	PriorityCritical: true,
	PriorityHigh:     true,
	PriorityNormal:   true,
	PriorityLow:      true,
}

const schema = `
CREATE TABLE IF NOT EXISTS agents (
	id TEXT PRIMARY KEY,       -- e.g. 'claude', 'gemini'
	name TEXT,                 -- Display name
	status TEXT,               -- 'idle', 'busy', 'blocked', 'testing', 'review', 'error', 'offline', 'paused'
	status_detail TEXT,        -- v5.0: Detailed status (e.g., 'awaiting_browser_verification')
	status_reason TEXT,        -- v5.0: Reason for current status
	task TEXT,                 -- Current Task ID (validated)
	task_phase TEXT,           -- v5.0: Current phase (coding, testing, review, etc.)
	blocked_by TEXT,           -- v5.0: Agent ID blocking this agent
	blocked_since DATETIME,    -- v5.0: When agent became blocked
	hb DATETIME,               -- Last Heartbeat
	last_activity DATETIME,    -- v5.0: Last meaningful activity timestamp
	context TEXT,              -- Log file path
	capabilities TEXT,         -- JSON/Text of skills (structured in v5.0)
	cannot_do TEXT,            -- v5.0: Things this agent cannot do
	session_start DATETIME,    -- v5.0: When this session started
	tasks_completed INTEGER DEFAULT 0  -- v5.0: Tasks completed this session
);

CREATE TABLE IF NOT EXISTS tasks (
	id TEXT PRIMARY KEY,
	title TEXT,
	description TEXT,          -- NEW: Task description
	status TEXT,               -- 'todo', 'in_progress', 'done'
	assignee TEXT,
	priority INTEGER DEFAULT 1,
	progress INTEGER DEFAULT 0, -- Progress percentage (0-100)
	parent TEXT,               -- Parent task ID for subtasks
	depends_on TEXT,           -- Comma-separated list of task IDs this task depends on
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS signals (
	id TEXT PRIMARY KEY,
	to_agent TEXT,
	from_agent TEXT,
	msg TEXT,
	priority TEXT DEFAULT 'normal',  -- v5.0: critical, high, normal, low
	signal_type TEXT,                -- v5.0: escalation, handoff, status, etc.
	subject TEXT,                    -- v5.0: Short subject line
	related_task TEXT,               -- v5.0: Related task ID
	read BOOLEAN DEFAULT 0,
	acknowledged BOOLEAN DEFAULT 0,  -- v5.0: Agent confirmed understanding
	acted BOOLEAN DEFAULT 0,         -- v5.0: Agent took action
	reply_to TEXT,
	expects_reply BOOLEAN DEFAULT 0,
	expires_at DATETIME,             -- v5.0: Signal expiration time
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS handoffs (
	id TEXT PRIMARY KEY,
	task_id TEXT NOT NULL,
	from_agent TEXT NOT NULL,
	to_agent TEXT NOT NULL,
	context_file TEXT,               -- Path to context/handoff document
	action_required TEXT,            -- What the recipient needs to do
	status TEXT DEFAULT 'pending',   -- pending, acknowledged, rejected, completed
	reject_reason TEXT,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	acknowledged_at DATETIME,
	completed_at DATETIME
);

CREATE TABLE IF NOT EXISTS task_history (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	task_id TEXT,
	old_status TEXT,
	new_status TEXT,
	changed_by TEXT,
	changed_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS events (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	event_type TEXT,           -- 'agent_start', 'agent_stop', 'task_start', 'task_complete', 'error', 'retry'
	agent_id TEXT,
	task_id TEXT,
	details TEXT,              -- JSON or text details
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS stats (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	stat_date DATE,
	tasks_created INTEGER DEFAULT 0,
	tasks_completed INTEGER DEFAULT 0,
	signals_sent INTEGER DEFAULT 0,
	errors INTEGER DEFAULT 0,
	retries INTEGER DEFAULT 0,
	agent_sessions INTEGER DEFAULT 0
);

CREATE TABLE IF NOT EXISTS task_results (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	task_id TEXT,
	agent_id TEXT,
	model_used TEXT,
	cost_usd REAL DEFAULT 0.0,
	input_tokens INTEGER DEFAULT 0,
	output_tokens INTEGER DEFAULT 0,
	completed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY(task_id) REFERENCES tasks(id),
	FOREIGN KEY(agent_id) REFERENCES agents(id)
);

CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
CREATE INDEX IF NOT EXISTS idx_tasks_assignee ON tasks(assignee);
CREATE INDEX IF NOT EXISTS idx_events_type ON events(event_type);
CREATE INDEX IF NOT EXISTS idx_events_date ON events(created_at);
CREATE INDEX IF NOT EXISTS idx_signals_to ON signals(to_agent, read);
CREATE INDEX IF NOT EXISTS idx_signals_priority ON signals(priority, read);
CREATE INDEX IF NOT EXISTS idx_handoffs_task ON handoffs(task_id);
CREATE INDEX IF NOT EXISTS idx_handoffs_to ON handoffs(to_agent, status);
CREATE INDEX IF NOT EXISTS idx_agents_status ON agents(status);
CREATE INDEX IF NOT EXISTS idx_task_results_completed ON task_results(completed_at);
CREATE INDEX IF NOT EXISTS idx_task_results_agent ON task_results(agent_id);
CREATE INDEX IF NOT EXISTS idx_task_results_model ON task_results(model_used);
`

// ===========================================
// SHELL SESSION MANAGEMENT (v5.0)
// ===========================================

// ShellSession represents an interactive shell session with an AI agent
type ShellSession struct {
	ID         string
	AgentID    string
	Backend    string // "claude", "gemini", "codex"
	Cmd        *exec.Cmd
	Stdin      io.WriteCloser
	StdoutChan chan string
	StderrChan chan string
	State      string // "running", "stopped"
	CreatedAt  time.Time
	Ctx        context.Context
	Cancel     context.CancelFunc
	Wg         sync.WaitGroup
	Mu         sync.RWMutex
}

// Global session manager
var (
	shellSessions   = make(map[string]*ShellSession)
	shellSessionsMu sync.RWMutex
)

// NewShellSession creates and starts a new shell session
func NewShellSession(ctx context.Context, agentID, backend string) (*ShellSession, error) {
	ctx, cancel := context.WithCancel(ctx)

	s := &ShellSession{
		ID:         uuid.New().String()[:8],
		AgentID:    agentID,
		Backend:    backend,
		StdoutChan: make(chan string, 256),
		StderrChan: make(chan string, 256),
		State:      "running",
		CreatedAt:  time.Now(),
		Ctx:        ctx,
		Cancel:     cancel,
	}

	// Start bash shell
	s.Cmd = exec.CommandContext(ctx, "/bin/bash")
	s.Cmd.Dir, _ = os.Getwd()
	s.Cmd.Env = os.Environ()

	var err error
	s.Stdin, err = s.Cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}

	stdoutPipe, err := s.Cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}

	stderrPipe, err := s.Cmd.StderrPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}

	if err := s.Cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("start shell: %w", err)
	}

	// Start output readers
	s.Wg.Add(2)
	go s.readPipe(stdoutPipe, s.StdoutChan)
	go s.readPipe(stderrPipe, s.StderrChan)

	// Wait for shell ready
	s.ExecRaw("echo 'SHELL_READY_" + s.ID + "'")
	s.WaitForMarker("SHELL_READY_"+s.ID, 3*time.Second)

	return s, nil
}

func (s *ShellSession) readPipe(r io.Reader, ch chan<- string) {
	defer s.Wg.Done()
	scanner := bufio.NewScanner(r)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		select {
		case ch <- scanner.Text():
		default:
			// Drop if channel full
		}
	}
}

// ExecRaw sends a raw command to the shell
func (s *ShellSession) ExecRaw(cmd string) {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	if s.Stdin != nil {
		fmt.Fprintln(s.Stdin, cmd)
	}
}

// WaitForMarker waits for a specific marker in output
func (s *ShellSession) WaitForMarker(marker string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case line := <-s.StdoutChan:
			if strings.Contains(line, marker) {
				return true
			}
		case <-time.After(100 * time.Millisecond):
		}
	}
	return false
}

// SendPrompt sends a prompt to the AI and collects response
func (s *ShellSession) SendPrompt(prompt string, timeout time.Duration) (string, error) {
	endMarker := fmt.Sprintf("__END_%d__", time.Now().UnixNano())
	escaped := strings.ReplaceAll(prompt, "'", "'\\''")

	var cmd string
	switch s.Backend {
	case "claude":
		cmd = fmt.Sprintf("claude -p '%s' --dangerously-skip-permissions --output-format text < /dev/null 2>&1 && echo '%s'", escaped, endMarker)
	case "gemini":
		cmd = fmt.Sprintf("gemini -y --output-format text '%s' 2>&1 && echo '%s'", escaped, endMarker)
	case "claude-code":
		// Claude Code headless mode - no permissions skip, cleaner output
		cmd = fmt.Sprintf("claude -p '%s' --output-format text < /dev/null 2>&1 && echo '%s'", escaped, endMarker)
	case "antigravity":
		// Antigravity headless mode - Gemini 3 Pro Preview for complex tasks
		cmd = fmt.Sprintf("gemini -m gemini-3-pro-preview -y --output-format text '%s' 2>&1 && echo '%s'", escaped, endMarker)
	default:
		cmd = fmt.Sprintf("echo 'Unknown backend: %s' && echo '%s'", s.Backend, endMarker)
	}

	s.ExecRaw(cmd)

	// Collect output
	var output strings.Builder
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		select {
		case line := <-s.StdoutChan:
			if strings.Contains(line, endMarker) {
				return strings.TrimSpace(output.String()), nil
			}
			output.WriteString(line)
			output.WriteString("\n")
		case line := <-s.StderrChan:
			output.WriteString("[stderr] ")
			output.WriteString(line)
			output.WriteString("\n")
		case <-time.After(100 * time.Millisecond):
		}
	}

	return output.String(), fmt.Errorf("timeout waiting for response")
}

// Close terminates the shell session
func (s *ShellSession) Close() error {
	s.Mu.Lock()
	s.State = "stopped"
	s.Mu.Unlock()

	s.Cancel()

	if s.Stdin != nil {
		s.Stdin.Close()
	}

	// Wait for readers with timeout
	done := make(chan struct{})
	go func() {
		s.Wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}

	if s.Cmd != nil && s.Cmd.Process != nil {
		s.Cmd.Process.Kill()
	}

	return nil
}

// GetShellSession retrieves a session by agent ID
func GetShellSession(agentID string) (*ShellSession, bool) {
	shellSessionsMu.RLock()
	defer shellSessionsMu.RUnlock()
	s, ok := shellSessions[agentID]
	return s, ok
}

// RegisterShellSession adds a session to the global manager
func RegisterShellSession(s *ShellSession) {
	shellSessionsMu.Lock()
	defer shellSessionsMu.Unlock()
	shellSessions[s.AgentID] = s
}

// CloseShellSession closes and removes a session
func CloseShellSession(agentID string) error {
	shellSessionsMu.Lock()
	defer shellSessionsMu.Unlock()

	s, ok := shellSessions[agentID]
	if !ok {
		return fmt.Errorf("session not found: %s", agentID)
	}

	err := s.Close()
	delete(shellSessions, agentID)
	return err
}

// ListShellSessions returns all active sessions
func ListShellSessions() []*ShellSession {
	shellSessionsMu.RLock()
	defer shellSessionsMu.RUnlock()

	sessions := make([]*ShellSession, 0, len(shellSessions))
	for _, s := range shellSessions {
		sessions = append(sessions, s)
	}
	return sessions
}

// ===========================================
// ORCHESTRATOR SYSTEM (v5.1)
// ===========================================

// ===========================================
// FILE LOCKING & CONFLICT PREVENTION
// ===========================================
// Rules to prevent race conditions when multiple agents work concurrently:
//
// 1. FILE OWNERSHIP: Each file should only be edited by ONE agent at a time
//    - Before editing, agent must claim file in FileLocks
//    - Other agents must check locks before editing
//    - Locks expire after 5 minutes (stale protection)
//
// 2. TASK BOUNDARIES: Tasks should have clear file boundaries
//    - Each task specifies which files it may modify
//    - Orchestrator prevents overlapping file assignments
//
// 3. ATOMIC OPERATIONS: Use temp files + rename for writes
//    - Write to .tmp file first
//    - Rename atomically to target
//
// 4. CONFLICT RESOLUTION:
//    - If conflict detected, HALT the conflicting agent
//    - Send CRITICAL signal to coordinator
//    - Let coordinator decide resolution

// FileLock represents a lock on a file
type FileLock struct {
	FilePath  string    `json:"file_path"`
	AgentID   string    `json:"agent_id"`
	TaskID    string    `json:"task_id"`
	LockedAt  time.Time `json:"locked_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// Global file locks (persisted to disk)
var (
	fileLocks   = make(map[string]*FileLock)
	fileLocksMu sync.RWMutex
)

// AcquireFileLock tries to acquire a lock on a file
func AcquireFileLock(filePath, agentID, taskID string) error {
	fileLocksMu.Lock()
	defer fileLocksMu.Unlock()

	now := time.Now()

	// Check if already locked
	if lock, exists := fileLocks[filePath]; exists {
		// Check if lock expired
		if lock.ExpiresAt.After(now) {
			if lock.AgentID != agentID {
				return fmt.Errorf("file '%s' is locked by agent '%s' (task: %s)", filePath, lock.AgentID, lock.TaskID)
			}
			// Same agent, refresh lock
			lock.ExpiresAt = now.Add(5 * time.Minute)
			return nil
		}
		// Lock expired, remove it
		delete(fileLocks, filePath)
	}

	// Acquire new lock
	fileLocks[filePath] = &FileLock{
		FilePath:  filePath,
		AgentID:   agentID,
		TaskID:    taskID,
		LockedAt:  now,
		ExpiresAt: now.Add(5 * time.Minute),
	}

	return nil
}

// ReleaseFileLock releases a lock on a file
func ReleaseFileLock(filePath, agentID string) {
	fileLocksMu.Lock()
	defer fileLocksMu.Unlock()

	if lock, exists := fileLocks[filePath]; exists {
		if lock.AgentID == agentID {
			delete(fileLocks, filePath)
		}
	}
}

// ReleaseAllLocks releases all locks held by an agent
func ReleaseAllLocks(agentID string) {
	fileLocksMu.Lock()
	defer fileLocksMu.Unlock()

	for path, lock := range fileLocks {
		if lock.AgentID == agentID {
			delete(fileLocks, path)
		}
	}
}

// CheckFileLock checks if a file is locked (without acquiring)
func CheckFileLock(filePath string) (*FileLock, bool) {
	fileLocksMu.RLock()
	defer fileLocksMu.RUnlock()

	lock, exists := fileLocks[filePath]
	if !exists {
		return nil, false
	}

	// Check expiration
	if lock.ExpiresAt.Before(time.Now()) {
		return nil, false
	}

	return lock, true
}

// GetAllFileLocks returns all active locks
func GetAllFileLocks() []*FileLock {
	fileLocksMu.RLock()
	defer fileLocksMu.RUnlock()

	now := time.Now()
	locks := make([]*FileLock, 0)
	for _, lock := range fileLocks {
		if lock.ExpiresAt.After(now) {
			locks = append(locks, lock)
		}
	}
	return locks
}

// ===========================================

// OrchestratorState holds the complete state of the orchestration system
type OrchestratorState struct {
	Version       string                       `json:"version"`
	StartedAt     time.Time                    `json:"started_at"`
	LastSaveAt    time.Time                    `json:"last_save_at"`
	AgentPool     map[string]*ManagedAgent     `json:"agent_pool"`
	TaskQueue     []QueuedTask                 `json:"task_queue"`
	ActiveTasks   map[string]*ActiveTask       `json:"active_tasks"`
	FileLocks     map[string]*FileLock         `json:"file_locks"` // Persisted locks
	Metrics       OrchestratorMetrics          `json:"metrics"`
	Config        OrchestratorConfig           `json:"config"`
	mu            sync.RWMutex
}

// ManagedAgent represents an agent under orchestrator control
type ManagedAgent struct {
	ID            string    `json:"id"`
	Backend       string    `json:"backend"`       // claude, gemini
	Role          string    `json:"role"`          // coder, reviewer, researcher, coordinator
	Capabilities  []string  `json:"capabilities"`  // coding, debugging, research, review
	CurrentTask   string    `json:"current_task"`
	Status        string    `json:"status"`
	SpawnedAt     time.Time `json:"spawned_at"`
	LastHeartbeat time.Time `json:"last_heartbeat"`
	TasksCompleted int      `json:"tasks_completed"`
	Errors        int       `json:"errors"`
	AutoSpawned   bool      `json:"auto_spawned"`  // Was this agent spawned by orchestrator?
}

// QueuedTask represents a task waiting for assignment
type QueuedTask struct {
	TaskID           string    `json:"task_id"`
	Title            string    `json:"title"`
	Description      string    `json:"description"`
	Priority         int       `json:"priority"`
	RequiredCap      []string  `json:"required_capabilities"` // What capabilities needed
	PreferredBackend string    `json:"preferred_backend"`     // Prefer claude/gemini
	Complexity       string    `json:"complexity"`            // simple, moderate, complex
	AddedAt          time.Time `json:"added_at"`
	Attempts         int       `json:"attempts"`
	GeminiFailures   int       `json:"gemini_failures"`       // Track Gemini-specific failures
}

// ActiveTask represents a task currently being worked on
type ActiveTask struct {
	TaskID       string    `json:"task_id"`
	AgentID      string    `json:"agent_id"`
	StartedAt    time.Time `json:"started_at"`
	LastProgress time.Time `json:"last_progress"`
	Progress     int       `json:"progress"`
	Phase        string    `json:"phase"`           // planning, coding, testing, review
	Retries      int       `json:"retries"`
	MaxRetries   int       `json:"max_retries"`
}

// OrchestratorMetrics tracks performance
type OrchestratorMetrics struct {
	TasksAssigned    int           `json:"tasks_assigned"`
	TasksCompleted   int           `json:"tasks_completed"`
	TasksFailed      int           `json:"tasks_failed"`
	AgentsSpawned    int           `json:"agents_spawned"`
	AgentsRespawned  int           `json:"agents_respawned"`
	AverageTaskTime  time.Duration `json:"average_task_time"`
	Uptime           time.Duration `json:"uptime"`
}

// OrchestratorConfig configures behavior
type OrchestratorConfig struct {
	MaxAgents          int           `json:"max_agents"`
	MinIdleAgents      int           `json:"min_idle_agents"`
	StaleThreshold     time.Duration `json:"stale_threshold"`
	TaskTimeout        time.Duration `json:"task_timeout"`
	SaveInterval       time.Duration `json:"save_interval"`
	AutoScale          bool          `json:"auto_scale"`
	AutoRecover        bool          `json:"auto_recover"`
	DefaultBackend     string        `json:"default_backend"`
}

// AgentCapabilities defines what each backend/role can do
var AgentCapabilities = map[string][]string{
	"claude:coder":     {"coding", "debugging", "refactoring", "documentation", "complex_logic", "architecture"},
	"claude:reviewer":  {"code_review", "security_audit", "best_practices"},
	"claude:analyst":   {"architecture", "planning", "analysis", "complex_logic"},
	"gemini:researcher": {"research", "summarization", "web_search", "simple_tasks"},
	"gemini:coder":     {"coding", "quick_fixes", "simple_tasks", "repetitive"},
}

// TaskComplexity levels
const (
	ComplexitySimple   = "simple"   // Gemini can handle - research, summarization, simple fixes
	ComplexityModerate = "moderate" // Either can handle - standard coding tasks
	ComplexityComplex  = "complex"  // Claude preferred - architecture, complex debugging
)

// ComplexityKeywords maps keywords to complexity levels
// Strategy: Gemini for data-heavy/repetitive tasks (cheaper, good at processing)
//           Claude for thought-intensive/complex logic tasks (more capable)
var ComplexityKeywords = map[string]string{
	// Simple tasks - Gemini preferred (data processing, repetitive, bulk operations)
	"research":      ComplexitySimple,
	"summarize":     ComplexitySimple,
	"summary":       ComplexitySimple,
	"list":          ComplexitySimple,
	"find":          ComplexitySimple,
	"search":        ComplexitySimple,
	"rename":        ComplexitySimple,
	"update":        ComplexitySimple,
	"fix typo":      ComplexitySimple,
	"add comment":   ComplexitySimple,
	"documentation": ComplexitySimple,
	"readme":        ComplexitySimple,
	"format":        ComplexitySimple,
	"lint":          ComplexitySimple,
	"cleanup":       ComplexitySimple,
	// Data processing tasks - Gemini excels at these (high volume, pattern-based)
	"log":           ComplexitySimple, // Process logs
	"logs":          ComplexitySimple,
	"parse":         ComplexitySimple, // Parse data
	"extract":       ComplexitySimple, // Extract info
	"convert":       ComplexitySimple, // Convert formats
	"transform":     ComplexitySimple, // Transform data
	"filter":        ComplexitySimple, // Filter data
	"sort":          ComplexitySimple, // Sort data
	"count":         ComplexitySimple, // Count items
	"aggregate":     ComplexitySimple, // Aggregate data
	"report":        ComplexitySimple, // Generate reports
	"csv":           ComplexitySimple, // CSV processing
	"json":          ComplexitySimple, // JSON processing
	"bulk":          ComplexitySimple, // Bulk operations
	"batch":         ComplexitySimple, // Batch processing
	"repetitive":    ComplexitySimple, // Repetitive tasks
	"template":      ComplexitySimple, // Template-based generation
	"boilerplate":   ComplexitySimple, // Boilerplate code

	// Moderate tasks - either backend, prefer Gemini for cost savings
	"implement":  ComplexityModerate,
	"add":        ComplexityModerate,
	"create":     ComplexityModerate,
	"build":      ComplexityModerate,
	"test":       ComplexityModerate,
	"fix":        ComplexityModerate,
	"bug":        ComplexityModerate,

	// Complex tasks - Claude preferred (thought-intensive, complex reasoning)
	"refactor":     ComplexityComplex,
	"architecture": ComplexityComplex,
	"design":       ComplexityComplex,
	"optimize":     ComplexityComplex, // Optimization requires deep understanding
	"security":     ComplexityComplex, // Security requires careful analysis
	"performance":  ComplexityComplex, // Performance tuning is nuanced
	"debug":        ComplexityComplex, // Debugging requires reasoning
	"investigate":  ComplexityComplex, // Investigation needs reasoning
	"analyze":      ComplexityComplex, // Deep analysis
	"complex":      ComplexityComplex,
	"migrate":      ComplexityComplex, // Migration has many edge cases
	"integration":  ComplexityComplex, // Integrations are tricky
	"algorithm":    ComplexityComplex, // Algorithm design
	"concurrent":   ComplexityComplex, // Concurrency is hard
	"async":        ComplexityComplex, // Async patterns are complex
	"race":         ComplexityComplex, // Race conditions
	"deadlock":     ComplexityComplex, // Deadlock analysis
	"memory":       ComplexityComplex, // Memory management
	"edge case":    ComplexityComplex, // Edge cases need careful thought
}

// AnalyzeTaskComplexity determines task complexity from title/description
func AnalyzeTaskComplexity(title, description string) string {
	text := strings.ToLower(title + " " + description)

	// Check for complex keywords first (highest priority)
	for keyword, complexity := range ComplexityKeywords {
		if complexity == ComplexityComplex && strings.Contains(text, keyword) {
			return ComplexityComplex
		}
	}

	// Then check for simple keywords
	for keyword, complexity := range ComplexityKeywords {
		if complexity == ComplexitySimple && strings.Contains(text, keyword) {
			return ComplexitySimple
		}
	}

	// Default to moderate
	return ComplexityModerate
}

// GetPreferredBackend returns the preferred backend for a task based on complexity
func GetPreferredBackend(complexity string) string {
	switch complexity {
	case ComplexitySimple:
		return "gemini" // Cheaper, faster for simple tasks
	case ComplexityComplex:
		return "claude" // More capable for complex tasks
	default:
		return "gemini" // Default to Gemini to save costs
	}
}

// TaskRouting holds routing decision info
type TaskRouting struct {
	TaskID           string `json:"task_id"`
	Complexity       string `json:"complexity"`
	PreferredBackend string `json:"preferred_backend"`
	Reason           string `json:"reason"`
	GeminiSuitable   bool   `json:"gemini_suitable"`
}

// ===========================================
// AGENT PIPELINE SYSTEM
// ===========================================
// Chain agents together for complex workflows:
// - Gemini → Claude: Gemini preprocesses data, Claude does reasoning
// - Claude → Gemini: Claude plans, Gemini executes bulk operations
// - Multi-stage: Any combination of agents in sequence
//
// Benefits:
// - Cost optimization: Gemini handles bulk data (cheap)
// - Quality optimization: Claude handles reasoning (capable)
// - Context optimization: Each agent gets focused, smaller context

// PipelineStage represents one stage in a pipeline
type PipelineStage struct {
	Name        string        `json:"name"`
	Backend     string        `json:"backend"`     // claude, gemini
	Role        string        `json:"role"`        // preprocess, analyze, synthesize, execute
	Prompt      string        `json:"prompt"`      // Template with {{input}} placeholder
	Timeout     time.Duration `json:"timeout"`
	MaxTokens   int           `json:"max_tokens"`  // Limit output to control cost/context
	PassThrough bool          `json:"pass_through"` // If true, pass input + output to next stage
}

// Pipeline represents a multi-agent workflow
type Pipeline struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Stages      []PipelineStage `json:"stages"`
	CreatedAt   time.Time       `json:"created_at"`
}

// PipelineResult holds the result of running a pipeline
type PipelineResult struct {
	PipelineID   string                 `json:"pipeline_id"`
	Input        string                 `json:"input"`
	FinalOutput  string                 `json:"final_output"`
	StageOutputs map[string]string      `json:"stage_outputs"`
	TotalTime    time.Duration          `json:"total_time"`
	StageTimes   map[string]time.Duration `json:"stage_times"`
	Errors       []string               `json:"errors"`
	Success      bool                   `json:"success"`
}

// Pre-defined pipeline templates
var PipelineTemplates = map[string]Pipeline{
	// Gemini preprocesses large data, Claude analyzes
	"data-analyze": {
		Name:        "Data Analysis Pipeline",
		Description: "Gemini filters/summarizes data, Claude does deep analysis",
		Stages: []PipelineStage{
			{
				Name:    "preprocess",
				Backend: "gemini",
				Role:    "preprocess",
				Prompt: `You are a data preprocessor. Given the following data/logs, extract the key information and summarize it concisely. Remove noise, duplicates, and irrelevant details. Output a clean, structured summary.

INPUT DATA:
{{input}}

Provide a concise summary (max 500 words) focusing on:
1. Key patterns or anomalies
2. Important values/metrics
3. Notable events or changes

SUMMARY:`,
				Timeout:   60 * time.Second,
				MaxTokens: 1000,
			},
			{
				Name:    "analyze",
				Backend: "claude",
				Role:    "analyze",
				Prompt: `You are an expert analyst. Based on this preprocessed summary, provide deep analysis and actionable recommendations.

PREPROCESSED DATA:
{{input}}

Provide:
1. Root cause analysis (if applicable)
2. Key insights
3. Recommended actions
4. Potential risks or concerns

ANALYSIS:`,
				Timeout:   90 * time.Second,
				MaxTokens: 2000,
			},
		},
	},

	// Claude plans, Gemini executes
	"plan-execute": {
		Name:        "Plan & Execute Pipeline",
		Description: "Claude creates a plan, Gemini executes bulk operations",
		Stages: []PipelineStage{
			{
				Name:    "plan",
				Backend: "claude",
				Role:    "plan",
				Prompt: `You are a technical architect. Create a detailed execution plan for the following task.

TASK:
{{input}}

Provide a step-by-step plan with:
1. Numbered steps (1, 2, 3...)
2. Specific commands or actions for each step
3. Expected outcomes
4. Error handling notes

Keep instructions clear and unambiguous for automated execution.

PLAN:`,
				Timeout:   60 * time.Second,
				MaxTokens: 1500,
			},
			{
				Name:    "execute",
				Backend: "gemini",
				Role:    "execute",
				Prompt: `You are an execution agent. Follow this plan and report the results of each step.

PLAN:
{{input}}

For each step:
1. Execute or simulate the action
2. Report success/failure
3. Note any issues

EXECUTION REPORT:`,
				Timeout:   90 * time.Second,
				MaxTokens: 2000,
			},
		},
	},

	// Research pipeline: Gemini gathers, Claude synthesizes
	"research": {
		Name:        "Research Pipeline",
		Description: "Gemini gathers information, Claude synthesizes insights",
		Stages: []PipelineStage{
			{
				Name:    "gather",
				Backend: "gemini",
				Role:    "research",
				Prompt: `You are a research assistant. Gather comprehensive information about the following topic.

TOPIC:
{{input}}

Provide:
1. Key facts and definitions
2. Current state/status
3. Different perspectives or approaches
4. Relevant examples or case studies

Keep output factual and well-organized.

RESEARCH FINDINGS:`,
				Timeout:   60 * time.Second,
				MaxTokens: 1500,
			},
			{
				Name:    "synthesize",
				Backend: "claude",
				Role:    "synthesize",
				Prompt: `You are a senior analyst. Synthesize these research findings into actionable insights.

RESEARCH FINDINGS:
{{input}}

Provide:
1. Executive summary (2-3 sentences)
2. Key insights and implications
3. Recommended approach or decision
4. Open questions or areas needing more research

SYNTHESIS:`,
				Timeout:   90 * time.Second,
				MaxTokens: 2000,
			},
		},
	},

	// Code review pipeline: Gemini scans, Claude deep-reviews
	"code-review": {
		Name:        "Code Review Pipeline",
		Description: "Gemini scans for obvious issues, Claude does deep review",
		Stages: []PipelineStage{
			{
				Name:    "scan",
				Backend: "gemini",
				Role:    "scan",
				Prompt: `You are a code scanner. Quickly review this code for obvious issues.

CODE:
{{input}}

List any:
1. Syntax errors
2. Obvious bugs
3. Security red flags
4. Style violations
5. Missing error handling

Keep output brief - just list the issues found.

SCAN RESULTS:`,
				Timeout:   45 * time.Second,
				MaxTokens: 800,
			},
			{
				Name:    "deep-review",
				Backend: "claude",
				Role:    "review",
				Prompt: `You are a senior code reviewer. Given this code and initial scan results, provide a thorough review.

CODE AND SCAN RESULTS:
{{input}}

Provide:
1. Architecture/design feedback
2. Performance concerns
3. Security analysis
4. Maintainability suggestions
5. Specific improvement recommendations with code examples

DETAILED REVIEW:`,
				Timeout:     120 * time.Second,
				MaxTokens:   3000,
				PassThrough: true, // Include original code in context
			},
		},
	},
}

// RunPipeline executes a pipeline with the given input
func RunPipeline(ctx context.Context, pipeline Pipeline, input string, db *sql.DB) (*PipelineResult, error) {
	result := &PipelineResult{
		PipelineID:   pipeline.ID,
		Input:        input,
		StageOutputs: make(map[string]string),
		StageTimes:   make(map[string]time.Duration),
		Errors:       make([]string, 0),
		Success:      true,
	}

	startTime := time.Now()
	currentInput := input

	for i, stage := range pipeline.Stages {
		stageStart := time.Now()

		fmt.Printf("\n🔄 Stage %d/%d: %s (%s)\n", i+1, len(pipeline.Stages), stage.Name, stage.Backend)

		// Create session for this stage
		agentID := fmt.Sprintf("pipeline-%s-%s", pipeline.ID, stage.Name)
		session, err := NewShellSession(ctx, agentID, stage.Backend)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Stage %s: failed to spawn: %v", stage.Name, err))
			result.Success = false
			return result, err
		}

		// Build prompt with input substitution
		prompt := strings.ReplaceAll(stage.Prompt, "{{input}}", currentInput)

		// Send prompt
		timeout := stage.Timeout
		if timeout == 0 {
			timeout = 60 * time.Second
		}

		response, err := session.SendPrompt(prompt, timeout)
		session.Close()

		stageDuration := time.Since(stageStart)
		result.StageTimes[stage.Name] = stageDuration

		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Stage %s: %v", stage.Name, err))
			result.Success = false
			// Continue with partial output if available
			if response == "" {
				return result, err
			}
		}

		// Store stage output
		result.StageOutputs[stage.Name] = response

		// Prepare input for next stage
		if stage.PassThrough {
			// Include both original input and this stage's output
			currentInput = fmt.Sprintf("ORIGINAL INPUT:\n%s\n\nPREVIOUS STAGE OUTPUT (%s):\n%s", currentInput, stage.Name, response)
		} else {
			currentInput = response
		}

		fmt.Printf("   ✅ Completed in %v\n", stageDuration.Round(time.Millisecond))

		// Log stage completion
		if db != nil {
			logEvent(db, "pipeline_stage_complete", agentID, pipeline.ID,
				fmt.Sprintf("stage=%s backend=%s duration=%v", stage.Name, stage.Backend, stageDuration))
		}
	}

	result.FinalOutput = currentInput
	result.TotalTime = time.Since(startTime)

	return result, nil
}

// Global orchestrator instance
var orchestrator *OrchestratorState
var orchestratorMu sync.Mutex

// NewOrchestrator creates a new orchestrator with default config
func NewOrchestrator() *OrchestratorState {
	return &OrchestratorState{
		Version:     "5.1",
		StartedAt:   time.Now(),
		AgentPool:   make(map[string]*ManagedAgent),
		TaskQueue:   make([]QueuedTask, 0),
		ActiveTasks: make(map[string]*ActiveTask),
		FileLocks:   make(map[string]*FileLock),
		Config: OrchestratorConfig{
			MaxAgents:      5,
			MinIdleAgents:  1,
			StaleThreshold: 60 * time.Second,
			TaskTimeout:    30 * time.Minute,
			SaveInterval:   30 * time.Second,
			AutoScale:      true,
			AutoRecover:    true,
			DefaultBackend: "gemini", // Default to Gemini for cost savings
		},
	}
}

// GetOrchestratorStatePath returns the path to save orchestrator state
func GetOrchestratorStatePath() string {
	root := findProjectRoot()
	stateDir := filepath.Join(root, "ProjectDocs", "LLMcomms")
	os.MkdirAll(stateDir, 0755)
	return filepath.Join(stateDir, "orchestrator_state.json")
}

// SaveState persists the orchestrator state to disk
func (o *OrchestratorState) SaveState() error {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.LastSaveAt = time.Now()
	o.Metrics.Uptime = time.Since(o.StartedAt)

	data, err := json.MarshalIndent(o, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}

	path := GetOrchestratorStatePath()

	// Write to temp file first, then rename (atomic)
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("write state: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename state: %w", err)
	}

	return nil
}

// LoadState loads the orchestrator state from disk
func LoadOrchestratorState() (*OrchestratorState, error) {
	path := GetOrchestratorStatePath()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return NewOrchestrator(), nil
		}
		return nil, fmt.Errorf("read state: %w", err)
	}

	var state OrchestratorState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("unmarshal state: %w", err)
	}

	// Initialize mutex (not serialized)
	state.mu = sync.RWMutex{}

	return &state, nil
}

// RegisterAgent adds an agent to the orchestrator pool
func (o *OrchestratorState) RegisterAgent(id, backend, role string, capabilities []string, autoSpawned bool) {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.AgentPool[id] = &ManagedAgent{
		ID:            id,
		Backend:       backend,
		Role:          role,
		Capabilities:  capabilities,
		Status:        "idle",
		SpawnedAt:     time.Now(),
		LastHeartbeat: time.Now(),
		AutoSpawned:   autoSpawned,
	}

	if autoSpawned {
		o.Metrics.AgentsSpawned++
	}
}

// UnregisterAgent removes an agent from the pool
func (o *OrchestratorState) UnregisterAgent(id string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	delete(o.AgentPool, id)
}

// SyncAgentsFromDB syncs agents from database into the orchestrator's pool
func (o *OrchestratorState) SyncAgentsFromDB(db *sql.DB) error {
	rows, err := db.Query(`
		SELECT id, COALESCE(status, 'idle'), COALESCE(task, ''), COALESCE(capabilities, 'gemini')
		FROM agents
		WHERE status IN ('idle', 'busy')
		ORDER BY hb DESC`)
	if err != nil {
		return err
	}
	defer rows.Close()

	o.mu.Lock()
	defer o.mu.Unlock()

	for rows.Next() {
		var id, status, task, capabilities string
		if err := rows.Scan(&id, &status, &task, &capabilities); err != nil {
			continue
		}

		// Only add if not already in pool
		if _, exists := o.AgentPool[id]; !exists {
			// Determine backend from capabilities
			backend := "gemini"
			if strings.Contains(strings.ToLower(capabilities), "claude") {
				backend = "claude"
			} else if strings.Contains(strings.ToLower(capabilities), "antigravity") {
				backend = "antigravity"
			} else if strings.Contains(strings.ToLower(capabilities), "codex") {
				backend = "codex"
			}

			o.AgentPool[id] = &ManagedAgent{
				ID:            id,
				Backend:       backend,
				Capabilities:  strings.Split(capabilities, ","),
				Status:        status,
				CurrentTask:   task,
				SpawnedAt:     time.Now(),
				LastHeartbeat: time.Now(),
				AutoSpawned:   false,
			}
		}
	}
	return nil
}

// SyncTasksFromDB syncs queued tasks from database into the orchestrator's queue
func (o *OrchestratorState) SyncTasksFromDB(db *sql.DB) error {
	rows, err := db.Query(`
		SELECT id, COALESCE(priority, 1)
		FROM tasks
		WHERE status = 'todo' AND (assignee IS NULL OR assignee = '')
		ORDER BY priority DESC, created_at ASC`)
	if err != nil {
		return err
	}
	defer rows.Close()

	o.mu.Lock()
	defer o.mu.Unlock()

	for rows.Next() {
		var id string
		var priority int
		if err := rows.Scan(&id, &priority); err != nil {
			continue
		}

		// Only add if not already in queue
		found := false
		for _, task := range o.TaskQueue {
			if task.TaskID == id {
				found = true
				break
			}
		}

		if !found {
			o.TaskQueue = append(o.TaskQueue, QueuedTask{
				TaskID:   id,
				Priority: priority,
				AddedAt:  time.Now(),
				Attempts: 0,
			})
		}
	}
	return nil
}

// UpdateAgentHeartbeat updates the heartbeat for an agent
func (o *OrchestratorState) UpdateAgentHeartbeat(id string) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if agent, ok := o.AgentPool[id]; ok {
		agent.LastHeartbeat = time.Now()
	}
}

// GetIdleAgents returns agents that are available for work
func (o *OrchestratorState) GetIdleAgents() []*ManagedAgent {
	o.mu.RLock()
	defer o.mu.RUnlock()

	var idle []*ManagedAgent
	for _, agent := range o.AgentPool {
		if agent.Status == "idle" && agent.CurrentTask == "" {
			idle = append(idle, agent)
		}
	}
	return idle
}

// GetStaleAgents returns agents that haven't heartbeated recently
func (o *OrchestratorState) GetStaleAgents() []*ManagedAgent {
	o.mu.RLock()
	defer o.mu.RUnlock()

	var stale []*ManagedAgent
	threshold := time.Now().Add(-o.Config.StaleThreshold)

	for _, agent := range o.AgentPool {
		if agent.LastHeartbeat.Before(threshold) {
			stale = append(stale, agent)
		}
	}
	return stale
}

// QueueTask adds a task to the queue with intelligent routing
func (o *OrchestratorState) QueueTask(taskID, title string, priority int, requiredCaps []string, preferredBackend string) {
	o.QueueTaskWithDescription(taskID, title, "", priority, requiredCaps, preferredBackend)
}

// QueueTaskWithDescription adds a task with full description for complexity analysis
func (o *OrchestratorState) QueueTaskWithDescription(taskID, title, description string, priority int, requiredCaps []string, preferredBackend string) {
	o.mu.Lock()
	defer o.mu.Unlock()

	// Analyze complexity to determine best backend
	complexity := AnalyzeTaskComplexity(title, description)

	// If no preferred backend specified, use intelligent routing
	if preferredBackend == "" {
		preferredBackend = GetPreferredBackend(complexity)
	}

	o.TaskQueue = append(o.TaskQueue, QueuedTask{
		TaskID:           taskID,
		Title:            title,
		Description:      description,
		Priority:         priority,
		RequiredCap:      requiredCaps,
		PreferredBackend: preferredBackend,
		Complexity:       complexity,
		AddedAt:          time.Now(),
	})

	// Sort by priority (higher first)
	for i := len(o.TaskQueue) - 1; i > 0; i-- {
		if o.TaskQueue[i].Priority > o.TaskQueue[i-1].Priority {
			o.TaskQueue[i], o.TaskQueue[i-1] = o.TaskQueue[i-1], o.TaskQueue[i]
		}
	}
}

// DequeueTask gets the next task for an agent with given capabilities
// Uses intelligent routing: Gemini for simple tasks, Claude for complex ones
func (o *OrchestratorState) DequeueTask(agentCaps []string, backend string) *QueuedTask {
	o.mu.Lock()
	defer o.mu.Unlock()

	var fallbackIdx = -1 // Track a fallback task if no perfect match

	for i, task := range o.TaskQueue {
		// Check if agent can handle this task's capabilities
		canHandle := true
		for _, reqCap := range task.RequiredCap {
			found := false
			for _, agentCap := range agentCaps {
				if agentCap == reqCap {
					found = true
					break
				}
			}
			if !found {
				canHandle = false
				break
			}
		}

		if !canHandle {
			continue
		}

		// Intelligent routing logic
		isGemini := backend == "gemini"
		isClaude := backend == "claude"

		// If task has failed with Gemini too many times, escalate to Claude
		if task.GeminiFailures >= 2 && isGemini {
			// Gemini has failed this task multiple times - skip for Gemini
			continue
		}

		// Match based on complexity and backend
		switch task.Complexity {
		case ComplexitySimple:
			// Simple tasks: Prefer Gemini (cheaper), but Claude can do them too
			if isGemini {
				// Perfect match - Gemini for simple tasks
				result := o.TaskQueue[i]
				o.TaskQueue = append(o.TaskQueue[:i], o.TaskQueue[i+1:]...)
				return &result
			}
			// Claude can do it but save it as fallback
			if fallbackIdx == -1 {
				fallbackIdx = i
			}

		case ComplexityComplex:
			// Complex tasks: Prefer Claude
			if isClaude {
				// Perfect match - Claude for complex tasks
				result := o.TaskQueue[i]
				o.TaskQueue = append(o.TaskQueue[:i], o.TaskQueue[i+1:]...)
				return &result
			}
			// Gemini shouldn't do complex tasks unless no Claude available
			// Don't set as fallback

		case ComplexityModerate:
			// Moderate tasks: Either can handle, prefer Gemini for cost savings
			if isGemini {
				result := o.TaskQueue[i]
				o.TaskQueue = append(o.TaskQueue[:i], o.TaskQueue[i+1:]...)
				return &result
			}
			// Claude can do it as fallback
			if fallbackIdx == -1 {
				fallbackIdx = i
			}

		default:
			// Undefined complexity: treat as simple, any agent can handle
			// First available agent takes it
			result := o.TaskQueue[i]
			o.TaskQueue = append(o.TaskQueue[:i], o.TaskQueue[i+1:]...)
			return &result
		}

		// Check explicit backend preference
		if task.PreferredBackend != "" && task.PreferredBackend == backend {
			result := o.TaskQueue[i]
			o.TaskQueue = append(o.TaskQueue[:i], o.TaskQueue[i+1:]...)
			return &result
		}
	}

	// If no perfect match, return fallback (if any)
	if fallbackIdx >= 0 {
		result := o.TaskQueue[fallbackIdx]
		o.TaskQueue = append(o.TaskQueue[:fallbackIdx], o.TaskQueue[fallbackIdx+1:]...)
		return &result
	}

	return nil
}

// EscalateTaskFromGemini marks a task as having failed with Gemini and requeues for Claude
func (o *OrchestratorState) EscalateTaskFromGemini(taskID string) {
	o.mu.Lock()
	defer o.mu.Unlock()

	// Find in active tasks
	if active, ok := o.ActiveTasks[taskID]; ok {
		// Requeue with Gemini failure count incremented
		o.TaskQueue = append([]QueuedTask{{
			TaskID:           taskID,
			Priority:         10, // High priority for retries
			Attempts:         active.Retries + 1,
			GeminiFailures:   1, // Will be checked in DequeueTask
			PreferredBackend: "claude", // Escalate to Claude
			Complexity:       ComplexityComplex, // Mark as complex since Gemini failed
			AddedAt:          time.Now(),
		}}, o.TaskQueue...)

		delete(o.ActiveTasks, taskID)
	}
}

// AssignTask assigns a task to an agent
func (o *OrchestratorState) AssignTask(taskID, agentID string) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if agent, ok := o.AgentPool[agentID]; ok {
		agent.CurrentTask = taskID
		agent.Status = "busy"
	}

	o.ActiveTasks[taskID] = &ActiveTask{
		TaskID:       taskID,
		AgentID:      agentID,
		StartedAt:    time.Now(),
		LastProgress: time.Now(),
		MaxRetries:   3,
	}

	o.Metrics.TasksAssigned++
}

// CompleteTask marks a task as completed
func (o *OrchestratorState) CompleteTask(taskID string) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if active, ok := o.ActiveTasks[taskID]; ok {
		if agent, ok := o.AgentPool[active.AgentID]; ok {
			agent.CurrentTask = ""
			agent.Status = "idle"
			agent.TasksCompleted++
		}
		delete(o.ActiveTasks, taskID)
	}

	o.Metrics.TasksCompleted++
}

// FailTask marks a task as failed
func (o *OrchestratorState) FailTask(taskID string, requeue bool) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if active, ok := o.ActiveTasks[taskID]; ok {
		if agent, ok := o.AgentPool[active.AgentID]; ok {
			agent.CurrentTask = ""
			agent.Status = "idle"
			agent.Errors++
		}

		if requeue && active.Retries < active.MaxRetries {
			// Requeue with incremented retry count
			o.TaskQueue = append([]QueuedTask{{
				TaskID:   taskID,
				Priority: 10, // High priority for retries
				Attempts: active.Retries + 1,
				AddedAt:  time.Now(),
			}}, o.TaskQueue...)
		}

		delete(o.ActiveTasks, taskID)
	}

	o.Metrics.TasksFailed++
}

// GetTimedOutTasks returns tasks that have exceeded the timeout
func (o *OrchestratorState) GetTimedOutTasks() []*ActiveTask {
	o.mu.RLock()
	defer o.mu.RUnlock()

	var timedOut []*ActiveTask
	threshold := time.Now().Add(-o.Config.TaskTimeout)

	for _, task := range o.ActiveTasks {
		if task.StartedAt.Before(threshold) {
			timedOut = append(timedOut, task)
		}
	}
	return timedOut
}

// SpawnAgentIfNeeded checks if we need more agents and spawns them
func (o *OrchestratorState) SpawnAgentIfNeeded(ctx context.Context, db *sql.DB) error {
	if !o.Config.AutoScale {
		return nil
	}

	o.mu.RLock()
	idleCount := 0
	totalCount := len(o.AgentPool)
	queueLen := len(o.TaskQueue)

	for _, agent := range o.AgentPool {
		if agent.Status == "idle" {
			idleCount++
		}
	}
	o.mu.RUnlock()

	// Need more agents if:
	// 1. Queue has tasks and no idle agents
	// 2. Idle agents below minimum
	needMore := (queueLen > 0 && idleCount == 0) || idleCount < o.Config.MinIdleAgents

	if needMore && totalCount < o.Config.MaxAgents {
		// Spawn a new agent
		agentID := fmt.Sprintf("auto-%s-%d", o.Config.DefaultBackend, time.Now().Unix()%1000)

		session, err := NewShellSession(ctx, agentID, o.Config.DefaultBackend)
		if err != nil {
			return fmt.Errorf("spawn agent: %w", err)
		}

		RegisterShellSession(session)

		// Register in orchestrator
		caps := []string{"coding", "debugging"}
		if o.Config.DefaultBackend == "gemini" {
			caps = []string{"research", "summarization"}
		}
		o.RegisterAgent(agentID, o.Config.DefaultBackend, "coder", caps, true)

		// Register in database
		_, err = db.Exec(`
			INSERT OR REPLACE INTO agents (id, name, status, hb, session_start, capabilities)
			VALUES (?, ?, 'idle', datetime('now'), datetime('now'), ?)`,
			agentID, agentID, o.Config.DefaultBackend)
		if err != nil {
			return fmt.Errorf("register agent in db: %w", err)
		}

		logEvent(db, "agent_auto_spawned", agentID, "", fmt.Sprintf("backend=%s reason=auto_scale", o.Config.DefaultBackend))

		o.Metrics.AgentsSpawned++

		fmt.Printf("🤖 Auto-spawned agent: %s (%s)\n", agentID, o.Config.DefaultBackend)
	}

	return nil
}

// RecoverStaleAgents respawns agents that have gone stale
func (o *OrchestratorState) RecoverStaleAgents(ctx context.Context, db *sql.DB) error {
	if !o.Config.AutoRecover {
		return nil
	}

	staleAgents := o.GetStaleAgents()

	for _, agent := range staleAgents {
		if !agent.AutoSpawned {
			// Don't auto-recover manually spawned agents
			continue
		}

		fmt.Printf("🔄 Recovering stale agent: %s\n", agent.ID)

		// Close old session if exists
		CloseShellSession(agent.ID)

		// Respawn
		session, err := NewShellSession(ctx, agent.ID, agent.Backend)
		if err != nil {
			fmt.Printf("❌ Failed to respawn %s: %v\n", agent.ID, err)
			continue
		}

		RegisterShellSession(session)
		o.UpdateAgentHeartbeat(agent.ID)

		// If agent had a task, requeue it
		if agent.CurrentTask != "" {
			o.FailTask(agent.CurrentTask, true)
		}

		logEvent(db, "agent_respawned", agent.ID, agent.CurrentTask, "stale_recovery")
		o.Metrics.AgentsRespawned++
	}

	return nil
}

// executeTaskAsync executes a task asynchronously using spawn run logic
func (o *OrchestratorState) executeTaskAsync(ctx context.Context, db *sql.DB, taskID, agentID, backend, prompt string) {
	go func() {
		fmt.Printf("🚀 Executing task %s with agent %s (backend: %s)...\n", taskID, agentID, backend)

		// Create session
		session, err := NewShellSession(ctx, agentID, backend)
		if err != nil {
			fmt.Printf("❌ Task %s failed to create session: %v\n", taskID, err)
			o.FailTask(taskID, true)
			db.Exec("UPDATE tasks SET status = 'todo', assignee = NULL, updated_at = datetime('now') WHERE id = ?", taskID)
			db.Exec("UPDATE agents SET task = NULL, status = 'idle', hb = datetime('now') WHERE id = ?", agentID)
			logEvent(db, "task_execution_failed", agentID, taskID, fmt.Sprintf("error=%v", err))
			return
		}
		defer session.Close()

		// Prepend worker context
		workerPrefix := fmt.Sprintf(`You are WORKER AGENT "%s" in the FLIP multi-agent system.

IMPORTANT RULES:
- You are a WORKER, not the coordinator
- Complete your assigned task and report results clearly
- Do NOT spawn additional agents without explicit approval
- Do NOT make autonomous decisions beyond your task scope
- If you encounter blockers, report them - don't try to solve everything
- Keep responses focused and concise

YOUR TASK:
`, agentID)
		fullPrompt := workerPrefix + prompt

		// Execute with timeout
		timeout := 120 * time.Second
		if o.Config.TaskTimeout > 0 {
			timeout = o.Config.TaskTimeout
		}

		response, err := session.SendPrompt(fullPrompt, timeout)

		if err != nil {
			fmt.Printf("❌ Task %s execution error: %v\n", taskID, err)
			o.FailTask(taskID, true)
			db.Exec("UPDATE tasks SET status = 'todo', assignee = NULL, updated_at = datetime('now') WHERE id = ?", taskID)
			db.Exec("UPDATE agents SET task = NULL, status = 'idle', hb = datetime('now') WHERE id = ?", agentID)
			logEvent(db, "task_execution_failed", agentID, taskID, fmt.Sprintf("error=%v", err))
			return
		}

		// Task completed successfully
		fmt.Printf("✅ Task %s completed by %s\n", taskID, agentID)

		// Mark task as done
		o.CompleteTask(taskID)
		db.Exec("UPDATE tasks SET status = 'done', progress = 100, updated_at = datetime('now') WHERE id = ?", taskID)
		db.Exec("UPDATE agents SET task = NULL, status = 'idle', hb = datetime('now'), tasks_completed = tasks_completed + 1 WHERE id = ?", agentID)

		// Store the response in task_results or as a signal
		resultID := uuid.New().String()[:8]
		db.Exec(`INSERT INTO signals (id, to_agent, from_agent, msg, priority, signal_type, related_task, created_at)
			VALUES (?, 'coordinator', ?, ?, 'normal', 'task_result', ?, datetime('now'))`,
			resultID, agentID, response, taskID)

		// Calculate and store cost metrics
		inputTokens := estimateTokens(fullPrompt)
		outputTokens := estimateTokens(response)
		modelUsed := getDefaultModel(backend)
		costUSD := agent.CalculateCost(modelUsed, inputTokens, outputTokens)
		storeTaskResult(db, taskID, agentID, modelUsed, costUSD, inputTokens, outputTokens)

		logEvent(db, "task_completed", agentID, taskID, fmt.Sprintf("tokens=%d cost=$%.6f", inputTokens+outputTokens, costUSD))

		fmt.Printf("💰 Task %s cost: $%.6f | Tokens: %d in / %d out\n", taskID, costUSD, inputTokens, outputTokens)
	}()
}

// RunOrchestrationLoop runs the main orchestration loop
func (o *OrchestratorState) RunOrchestrationLoop(ctx context.Context, db *sql.DB) {
	ticker := time.NewTicker(5 * time.Second)
	saveTicker := time.NewTicker(o.Config.SaveInterval)
	defer ticker.Stop()
	defer saveTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			o.SaveState()
			return

		case <-saveTicker.C:
			if err := o.SaveState(); err != nil {
				fmt.Printf("⚠️  Failed to save state: %v\n", err)
			}

		case <-ticker.C:
			// 0. Sync agents and tasks from database
			if err := o.SyncAgentsFromDB(db); err != nil {
				fmt.Printf("⚠️  Agent sync error: %v\n", err)
			}
			if err := o.SyncTasksFromDB(db); err != nil {
				fmt.Printf("⚠️  Task sync error: %v\n", err)
			}

			// Debug: Show current pool and queue state
			fmt.Printf("📊 Pool: %d agents, Queue: %d tasks, Idle: %d\n",
				len(o.AgentPool), len(o.TaskQueue), len(o.GetIdleAgents()))

			// 1. Check for stale agents and recover
			if err := o.RecoverStaleAgents(ctx, db); err != nil {
				fmt.Printf("⚠️  Recovery error: %v\n", err)
			}

			// 2. Check for timed out tasks
			timedOut := o.GetTimedOutTasks()
			for _, task := range timedOut {
				fmt.Printf("⏰ Task %s timed out, reassigning...\n", task.TaskID)
				o.FailTask(task.TaskID, true)

				// Send critical signal
				db.Exec(`INSERT INTO signals (id, to_agent, from_agent, msg, priority, signal_type)
					VALUES (?, 'coordinator', 'orchestrator', ?, 'critical', 'timeout')`,
					uuid.New().String()[:8],
					fmt.Sprintf("Task %s timed out after %v", task.TaskID, o.Config.TaskTimeout))
			}

			// 3. Auto-scale if needed
			if err := o.SpawnAgentIfNeeded(ctx, db); err != nil {
				fmt.Printf("⚠️  Auto-scale error: %v\n", err)
			}

			// 4. Assign queued tasks to idle agents and EXECUTE them
			idleAgents := o.GetIdleAgents()
			for _, agent := range idleAgents {
				task := o.DequeueTask(agent.Capabilities, agent.Backend)
				if task == nil {
					continue
				}

				// Fetch task description from database
				var title, description sql.NullString
				err := db.QueryRow("SELECT title, COALESCE(description, '') FROM tasks WHERE id = ?", task.TaskID).Scan(&title, &description)
				if err != nil {
					fmt.Printf("⚠️  Failed to fetch task %s details: %v\n", task.TaskID, err)
					continue
				}

				// Build prompt from title + description
				prompt := title.String
				if description.String != "" {
					prompt = title.String + "\n\n" + description.String
				}

				// Determine backend from agent capabilities or use default
				backend := agent.Backend
				if backend == "" {
					backend = "gemini" // default to gemini if not specified
				}

				o.AssignTask(task.TaskID, agent.ID)

				// Update database
				db.Exec("UPDATE tasks SET status = 'in_progress', assignee = ?, updated_at = datetime('now') WHERE id = ?",
					agent.ID, task.TaskID)
				db.Exec("UPDATE agents SET task = ?, status = 'busy', hb = datetime('now') WHERE id = ?",
					task.TaskID, agent.ID)

				logEvent(db, "task_auto_assigned", agent.ID, task.TaskID, fmt.Sprintf("attempt=%d backend=%s", task.Attempts, backend))

				fmt.Printf("📋 Auto-assigned task %s to %s (backend: %s)\n", task.TaskID, agent.ID, backend)

				// ACTUALLY EXECUTE THE TASK
				o.executeTaskAsync(ctx, db, task.TaskID, agent.ID, backend, prompt)
			}
		}
	}
}

// --- Project Root Detection ---
// Finds the project root by looking for .git or package.json

func findProjectRoot() string {
	// Start from current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}

	// Walk up the directory tree looking for markers
	dir := cwd
	for {
		// Check for .git directory
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		// Check for package.json (npm project root)
		if _, err := os.Stat(filepath.Join(dir, "package.json")); err == nil {
			return dir
		}
		// Check for go.mod
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root, return cwd
			return cwd
		}
		dir = parent
	}
}

func getDBPath() string {
	// Check for environment variable override
	if envPath := os.Getenv("FLIP_DB"); envPath != "" {
		return envPath
	}
	if envRoot := os.Getenv("FLIP_ROOT"); envRoot != "" {
		return filepath.Join(envRoot, "ProjectDocs", "LLMcomms", "flip.db")
	}
	root := findProjectRoot()
	return filepath.Join(root, "ProjectDocs", "LLMcomms", "flip.db")
}

func getContextPath(id string) string {
	root := findProjectRoot()
	base := filepath.Join(root, "ProjectDocs", "LLMcomms")

	// Special cases for known agents
	switch id {
	case "codex":
		return filepath.Join(base, "codex7_agent.md")
	case "claude2":
		return filepath.Join(base, "claude_agent_secondary.md")
	default:
		return filepath.Join(base, fmt.Sprintf("%s_agent.md", id))
	}
}

// --- DB Helper ---

// execWithRetry executes a query with exponential backoff retry on lock errors
func execWithRetry(db *sql.DB, query string, args ...interface{}) error {
	maxRetries := 5
	for i := 0; i < maxRetries; i++ {
		_, err := db.Exec(query, args...)
		if err == nil {
			return nil
		}
		if !strings.Contains(err.Error(), "locked") && !strings.Contains(err.Error(), "busy") {
			return err
		}
		// Exponential backoff: 100ms, 200ms, 400ms, 800ms, 1600ms
		time.Sleep(time.Duration(100*(1<<i)) * time.Millisecond)
	}
	return fmt.Errorf("database locked after %d retries", maxRetries)
}

// queryRowWithRetry executes a query row with retry on lock errors
func queryWithRetry(db *sql.DB, query string, args ...interface{}) (*sql.Rows, error) {
	maxRetries := 5
	for i := 0; i < maxRetries; i++ {
		rows, err := db.Query(query, args...)
		if err == nil {
			return rows, nil
		}
		if !strings.Contains(err.Error(), "locked") && !strings.Contains(err.Error(), "busy") {
			return nil, err
		}
		time.Sleep(time.Duration(100*(1<<i)) * time.Millisecond)
	}
	return nil, fmt.Errorf("database locked after %d retries", maxRetries)
}

func initDB() *sql.DB {
	dbPath := getDBPath()

	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	os.MkdirAll(dir, 0755)

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatal(err)
	}

	// Set connection pool settings
	db.SetMaxOpenConns(1) // SQLite only supports one writer
	db.SetMaxIdleConns(1)

	// Performance optimization (WAL mode) with error checking
	if _, err := db.Exec("PRAGMA journal_mode = WAL;"); err != nil {
		log.Printf("Warning: Failed to set WAL mode: %v", err)
	}
	// Use _busy_timeout in connection string since PRAGMA may not work
	// Also try the pragma anyway
	if _, err := db.Exec("PRAGMA busy_timeout = 5000;"); err != nil {
		log.Printf("Warning: Failed to set busy_timeout: %v", err)
	}

	// Init Schema
	if _, err := db.Exec(schema); err != nil {
		log.Fatalf("Failed to init schema: %v", err)
	}

	// Migrate: Add columns if missing (backward compatibility)
	// Tasks table migrations
	db.Exec("ALTER TABLE tasks ADD COLUMN description TEXT")
	db.Exec("ALTER TABLE tasks ADD COLUMN created_at DATETIME DEFAULT CURRENT_TIMESTAMP")
	db.Exec("ALTER TABLE tasks ADD COLUMN updated_at DATETIME DEFAULT CURRENT_TIMESTAMP")
	db.Exec("ALTER TABLE tasks ADD COLUMN depends_on TEXT")
	db.Exec("ALTER TABLE tasks ADD COLUMN progress INTEGER DEFAULT 0")

	// Signals table migrations (v5.0)
	db.Exec("ALTER TABLE signals ADD COLUMN reply_to TEXT")
	db.Exec("ALTER TABLE signals ADD COLUMN expects_reply BOOLEAN DEFAULT 0")
	db.Exec("ALTER TABLE signals ADD COLUMN priority TEXT DEFAULT 'normal'")
	db.Exec("ALTER TABLE signals ADD COLUMN signal_type TEXT")
	db.Exec("ALTER TABLE signals ADD COLUMN subject TEXT")
	db.Exec("ALTER TABLE signals ADD COLUMN related_task TEXT")
	db.Exec("ALTER TABLE signals ADD COLUMN acknowledged BOOLEAN DEFAULT 0")
	db.Exec("ALTER TABLE signals ADD COLUMN acted BOOLEAN DEFAULT 0")
	db.Exec("ALTER TABLE signals ADD COLUMN expires_at DATETIME")

	// Agents table migrations (v5.0)
	db.Exec("ALTER TABLE agents ADD COLUMN status_detail TEXT")
	db.Exec("ALTER TABLE agents ADD COLUMN status_reason TEXT")
	db.Exec("ALTER TABLE agents ADD COLUMN task_phase TEXT")
	db.Exec("ALTER TABLE agents ADD COLUMN blocked_by TEXT")
	db.Exec("ALTER TABLE agents ADD COLUMN blocked_since DATETIME")
	db.Exec("ALTER TABLE agents ADD COLUMN last_activity DATETIME")
	db.Exec("ALTER TABLE agents ADD COLUMN cannot_do TEXT")
	db.Exec("ALTER TABLE agents ADD COLUMN session_start DATETIME")
	db.Exec("ALTER TABLE agents ADD COLUMN tasks_completed INTEGER DEFAULT 0")

	return db
}

// --- Helper: Validate Task ID Exists ---

func taskExists(db *sql.DB, taskID string) bool {
	if taskID == "" {
		return true // Empty is valid (clearing task)
	}
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM tasks WHERE id = ?", taskID).Scan(&count)
	if err != nil {
		return false
	}
	return count > 0
}

// --- Helper: Log Task History ---

func logTaskHistory(db *sql.DB, taskID, oldStatus, newStatus, changedBy string) {
	db.Exec(`INSERT INTO task_history (task_id, old_status, new_status, changed_by) VALUES (?, ?, ?, ?)`,
		taskID, oldStatus, newStatus, changedBy)
}

// --- Logging Infrastructure ---

func getLogPath() string {
	if envPath := os.Getenv("FLIP_LOG"); envPath != "" {
		return envPath
	}
	root := findProjectRoot()
	return filepath.Join(root, "ProjectDocs", "LLMcomms", "flip.log")
}

// logToFile appends a timestamped message to flip.log
func logToFile(message string) {
	logPath := getLogPath()
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	f.WriteString(fmt.Sprintf("[%s] %s\n", timestamp, message))
}

// logEvent records an event to both the events table and log file
func logEvent(db *sql.DB, eventType, agentID, taskID, details string) {
	// Insert into events table
	db.Exec(`INSERT INTO events (event_type, agent_id, task_id, details) VALUES (?, ?, ?, ?)`,
		eventType, agentID, taskID, details)

	// Also log to file
	msg := fmt.Sprintf("[%s] agent=%s task=%s %s", eventType, agentID, taskID, details)
	logToFile(msg)
}

// incrementStat increments a daily stat counter
func incrementStat(db *sql.DB, column string) {
	today := time.Now().Format("2006-01-02")
	// Try to update existing row
	result, _ := db.Exec(fmt.Sprintf(`UPDATE stats SET %s = %s + 1 WHERE stat_date = ?`, column, column), today)
	if rows, _ := result.RowsAffected(); rows == 0 {
		// Insert new row for today
		db.Exec(fmt.Sprintf(`INSERT INTO stats (stat_date, %s) VALUES (?, 1)`, column), today)
	}
}

// storeTaskResult records cost and token metrics for a completed API call
func storeTaskResult(db *sql.DB, taskID, agentID, modelUsed string, costUSD float64, inputTokens, outputTokens int) error {
	_, err := db.Exec(`
		INSERT INTO task_results (task_id, agent_id, model_used, cost_usd, input_tokens, output_tokens)
		VALUES (?, ?, ?, ?, ?, ?)`,
		taskID, agentID, modelUsed, costUSD, inputTokens, outputTokens)

	if err != nil {
		return fmt.Errorf("store task result: %w", err)
	}

	return nil
}

// estimateTokens provides a rough estimate of token count (4 chars ≈ 1 token)
func estimateTokens(text string) int {
	return len(text) / 4
}

// getDefaultModel returns the default model for a backend
func getDefaultModel(backend string) string {
	switch backend {
	case "claude":
		return "claude-3-5-sonnet-20241022"
	case "claude-code":
		return "claude-3-5-sonnet-20241022"
	case "antigravity":
		return "gemini-3-pro-preview"
	case "gemini":
		return "gemini-1.5-flash"
	case "codex":
		return "gpt-3.5-turbo"
	default:
		return "unknown"
	}
}

// --- Helper: Get Task Dependencies ---

func getTaskDependencies(db *sql.DB, taskID string) []string {
	var dependsOn sql.NullString
	err := db.QueryRow("SELECT depends_on FROM tasks WHERE id = ?", taskID).Scan(&dependsOn)
	if err != nil || !dependsOn.Valid || dependsOn.String == "" {
		return nil
	}
	deps := strings.Split(dependsOn.String, ",")
	var result []string
	for _, d := range deps {
		d = strings.TrimSpace(d)
		if d != "" {
			result = append(result, d)
		}
	}
	return result
}

// --- Helper: Check if Dependencies are Satisfied ---

func checkDependencies(db *sql.DB, taskID string) (bool, []string) {
	deps := getTaskDependencies(db, taskID)
	if len(deps) == 0 {
		return true, nil
	}

	var blocking []string
	for _, depID := range deps {
		var status string
		err := db.QueryRow("SELECT status FROM tasks WHERE id = ?", depID).Scan(&status)
		if err != nil {
			// Dependency doesn't exist - treat as blocking
			blocking = append(blocking, fmt.Sprintf("%s (not found)", depID))
		} else if status != "done" {
			blocking = append(blocking, fmt.Sprintf("%s (%s)", depID, status))
		}
	}

	return len(blocking) == 0, blocking
}

// --- Helper: Add Dependency ---

func addDependency(db *sql.DB, taskID, dependsOnID string) error {
	// Get current dependencies
	deps := getTaskDependencies(db, taskID)

	// Check if already exists
	for _, d := range deps {
		if d == dependsOnID {
			return fmt.Errorf("dependency already exists")
		}
	}

	// Add new dependency
	deps = append(deps, dependsOnID)
	newDeps := strings.Join(deps, ",")

	_, err := db.Exec("UPDATE tasks SET depends_on = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", newDeps, taskID)
	return err
}

// --- Helper: Remove Dependency ---

func removeDependency(db *sql.DB, taskID, dependsOnID string) error {
	deps := getTaskDependencies(db, taskID)

	var newDeps []string
	found := false
	for _, d := range deps {
		if d == dependsOnID {
			found = true
		} else {
			newDeps = append(newDeps, d)
		}
	}

	if !found {
		return fmt.Errorf("dependency not found")
	}

	depsStr := strings.Join(newDeps, ",")
	if len(newDeps) == 0 {
		depsStr = ""
	}

	_, err := db.Exec("UPDATE tasks SET depends_on = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", depsStr, taskID)
	return err
}

// --- Helper: Min Int ---

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// --- Helper: Format Duration Friendly ---

func formatDuration(d time.Duration) string {
	d = d.Round(time.Minute)
	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	days := int(d.Hours() / 24)
	if days == 1 {
		return "1 day ago"
	}
	return fmt.Sprintf("%d days ago", days)
}

// --- Helper: Write Connection Alert File ---

func writeConnectionAlert(agentID string, failCount int, lastError string) string {
	root := findProjectRoot()
	alertDir := filepath.Join(root, "ProjectDocs", "LLMcomms", "alerts")
	os.MkdirAll(alertDir, 0755)

	alertFile := filepath.Join(alertDir, fmt.Sprintf("%s_connection_alert.md", agentID))
	content := fmt.Sprintf(`# Connection Alert

**Agent**: %s
**Time**: %s
**Failure Count**: %d
**Last Error**: %s

## Instructions

The FLIP database server appears to be unavailable. This agent has failed to connect %d times.

### For Other Agents
If you see this file, the server may have been restarted. Check:
1. Run: ./flip status
2. If that fails, wait 3 minutes and try again
3. Check for other alert files in this directory

### For Human Operator
- Check if the FLIP database is accessible
- Restart the serve command if needed: ./flip serve
- Clear this alert after resolving: rm %s
`, agentID, time.Now().Format(time.RFC3339), failCount, lastError, failCount, alertFile)

	os.WriteFile(alertFile, []byte(content), 0644)
	return alertFile
}

// --- Helper: Check for Alert Files ---

func checkAlertFiles() []string {
	root := findProjectRoot()
	alertDir := filepath.Join(root, "ProjectDocs", "LLMcomms", "alerts")

	var alerts []string
	files, err := os.ReadDir(alertDir)
	if err != nil {
		return alerts
	}

	for _, f := range files {
		if strings.HasSuffix(f.Name(), "_connection_alert.md") {
			alerts = append(alerts, f.Name())
		}
	}
	return alerts
}

// --- Helper: Clear Alert File ---

func clearConnectionAlert(agentID string) {
	root := findProjectRoot()
	alertFile := filepath.Join(root, "ProjectDocs", "LLMcomms", "alerts", fmt.Sprintf("%s_connection_alert.md", agentID))
	os.Remove(alertFile)
}

// --- Helper: Read Signals from Agent Log File (Fallback) ---

func readSignalsFromLogFile(agentID string) []string {
	logFile := getContextPath(agentID)

	file, err := os.Open(logFile)
	if err != nil {
		return nil
	}
	defer file.Close()

	// Read last 8KB for recent signals
	stat, _ := file.Stat()
	start := int64(0)
	if stat.Size() > 8192 {
		start = stat.Size() - 8192
	}
	file.Seek(start, 0)

	var signals []string
	scanner := bufio.NewScanner(file)
	re := regexp.MustCompile(`(?i)@` + agentID + `[:\s]+(.+)`)

	for scanner.Scan() {
		line := scanner.Text()
		matches := re.FindStringSubmatch(line)
		if len(matches) > 1 {
			signals = append(signals, strings.TrimSpace(matches[1]))
		}
	}

	// Return last 5 signals
	if len(signals) > 5 {
		signals = signals[len(signals)-5:]
	}
	return signals
}

// --- Helper: Scan Status from Log ---

func scanLogStatus(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// Read last 4KB to save memory/time
	stat, _ := file.Stat()
	start := int64(0)
	if stat.Size() > 4096 {
		start = stat.Size() - 4096
	}
	file.Seek(start, 0)

	scanner := bufio.NewScanner(file)
	lastStatus := "unknown"
	re := regexp.MustCompile(`(?i)\*\*Status\*\*:\s*([a-z_]+)`)

	for scanner.Scan() {
		line := scanner.Text()
		matches := re.FindStringSubmatch(line)
		if len(matches) > 1 {
			s := strings.ToLower(strings.TrimSpace(matches[1]))
			if s == "busy" || s == "idle" || s == "blocked" || s == "in_progress" {
				lastStatus = s
			}
		}
	}
	return lastStatus, nil
}

// --- Commands ---

func main() {
	var rootCmd = &cobra.Command{
		Use:   "flip",
		Short: "FLIP CLI - File-based LLM Inter-Process Communication",
		Long: `FLIP (File-based LLM Inter-Process Communication) CLI Tool v2.6

A coordination system for multi-agent AI development workflows.
Provides status tracking, task management, and inter-agent messaging.
Features retry logic for database operations and improved error handling.`,
	}

	// ===================
	// STATUS Command
	// ===================
	var verbose bool
	var statusCmd = &cobra.Command{
		Use:   "status",
		Short: "Show swarm status",
		Long:  "Display the current status of all registered agents in the swarm.",
		Run: func(cmd *cobra.Command, args []string) {
			db := initDB()
			defer db.Close()

			rows, err := db.Query(`
				SELECT agents.id, agents.status, agents.task, agents.hb, COALESCE(tasks.title, '') as task_title
				FROM agents
				LEFT JOIN tasks ON agents.task = tasks.id
				ORDER BY agents.id`)
			if err != nil {
				log.Fatal(err)
			}
			defer rows.Close()

			if verbose {
				fmt.Println("AGENT      | STATUS   | TASK       | TITLE                          | HEARTBEAT")
				fmt.Println("-----------+----------+------------+--------------------------------+----------")
			} else {
				fmt.Println("AGENT      | STATUS   | TASK")
				fmt.Println("-----------+----------+------------")
			}

			for rows.Next() {
				var id, status, taskTitle string
				var task sql.NullString
				var hb time.Time
				if err := rows.Scan(&id, &status, &task, &hb, &taskTitle); err != nil {
					log.Fatal(err)
				}

				taskStr := "-"
				if task.Valid && task.String != "" {
					taskStr = task.String
				}

				since := formatDuration(time.Since(hb))

				if verbose {
					titleDisplay := taskTitle
					if len(titleDisplay) > 30 {
						titleDisplay = titleDisplay[:27] + "..."
					}
					fmt.Printf("%-10s | %-8s | %-10s | %-30s | %s\n", id, status, taskStr, titleDisplay, since)
				} else {
					fmt.Printf("%-10s | %-8s | %s\n", id, status, taskStr)
				}
			}
		},
	}
	statusCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show detailed table with task titles")

	// ===================
	// REGISTER Command
	// ===================
	var capabilities string
	var registerCmd = &cobra.Command{
		Use:   "register [agent_id] [display_name]",
		Short: "Register a new agent",
		Long:  "Register a new agent in the FLIP system with optional capabilities.",
		Args:  cobra.MinimumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			db := initDB()
			defer db.Close()

			contextPath := getContextPath(args[0])

			stmt, _ := db.Prepare(`
				INSERT OR REPLACE INTO agents (id, name, status, hb, context, capabilities)
				VALUES (?, ?, 'idle', ?, ?, ?)`)
			_, err := stmt.Exec(args[0], args[1], time.Now(), contextPath, capabilities)
			if err != nil {
				log.Fatal(err)
			}
			fmt.Printf("Registered %s (%s)\n", args[1], args[0])
			if capabilities != "" {
				fmt.Printf("  Capabilities: %s\n", capabilities)
			}
		},
	}
	registerCmd.Flags().StringVarP(&capabilities, "capabilities", "c", "", "Agent capabilities (comma-separated)")

	// ===================
	// UNREGISTER Command
	// ===================
	var unregisterCmd = &cobra.Command{
		Use:   "unregister [agent_id]",
		Short: "Remove an agent",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			db := initDB()
			defer db.Close()

			_, err := db.Exec("DELETE FROM agents WHERE id = ?", args[0])
			if err != nil {
				log.Fatal(err)
			}
			fmt.Printf("Unregistered %s\n", args[0])
		},
	}

	// ===================
	// UPDATE Command (v5.0 - Extended Status System)
	// ===================
	var taskID string
	var clearTask bool
	var statusReason string
	var statusDetail string
	var blockedBy string
	var taskPhase string
	var updateCmd = &cobra.Command{
		Use:   "update [agent_id] [status]",
		Short: "Update agent status and task",
		Long: `Update an agent's status and optionally their current task.

Valid statuses (v5.0):
  idle     - Available for new tasks
  busy     - Actively working on task
  blocked  - Waiting on external input
  testing  - Code complete, awaiting verification
  review   - Awaiting code review
  error    - Task failed, needs intervention
  offline  - Session ended/crashed
  paused   - Temporarily suspended

Examples:
  flip update claude busy --task TASK-018
  flip update claude idle --clear-task
  flip update claude5 blocked --reason "awaiting browser test" --blocked-by antigravity
  flip update claude5 testing --task TASK-089 --phase verification
  flip update claude5 error --reason "tests failing"`,
		Args: cobra.MinimumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			db := initDB()
			defer db.Close()

			agentID := args[0]
			status := strings.ToLower(args[1])

			// Validate status (v5.0 extended statuses)
			if !validAgentStatuses[status] {
				validList := []string{}
				for s := range validAgentStatuses {
					validList = append(validList, s)
				}
				log.Fatalf("Invalid status '%s'. Valid: %s", status, strings.Join(validList, ", "))
			}

			// Build the update query dynamically
			updates := []string{"status = ?", "hb = ?", "last_activity = ?"}
			values := []interface{}{status, time.Now(), time.Now()}

			// Handle blocked status specially
			if status == "blocked" {
				updates = append(updates, "blocked_since = ?")
				values = append(values, time.Now())
				if blockedBy != "" {
					updates = append(updates, "blocked_by = ?")
					values = append(values, blockedBy)
				}
			} else {
				// Clear blocked fields when not blocked
				updates = append(updates, "blocked_since = NULL", "blocked_by = NULL")
			}

			// Handle optional fields
			if statusReason != "" {
				updates = append(updates, "status_reason = ?")
				values = append(values, statusReason)
			}
			if statusDetail != "" {
				updates = append(updates, "status_detail = ?")
				values = append(values, statusDetail)
			}
			if taskPhase != "" {
				updates = append(updates, "task_phase = ?")
				values = append(values, taskPhase)
			}

			// Handle task assignment
			if clearTask {
				updates = append(updates, "task = NULL", "task_phase = NULL")
			} else if taskID != "" {
				// Validate task exists
				if !taskExists(db, taskID) {
					log.Fatalf("Task '%s' does not exist. Use 'flip task add %s \"Title\"' to create it first.", taskID, taskID)
				}
				updates = append(updates, "task = ?")
				values = append(values, taskID)

				// Update task to in_progress
				db.Exec("UPDATE tasks SET status = 'in_progress', updated_at = ? WHERE id = ?", time.Now(), taskID)
			}

			// Add agent ID for WHERE clause
			values = append(values, agentID)

			query := fmt.Sprintf("UPDATE agents SET %s WHERE id = ?", strings.Join(updates, ", "))
			_, err := db.Exec(query, values...)
			if err != nil {
				log.Fatal(err)
			}

			// Log event
			details := fmt.Sprintf("status=%s", status)
			if statusReason != "" {
				details += fmt.Sprintf(" reason='%s'", statusReason)
			}
			if blockedBy != "" {
				details += fmt.Sprintf(" blocked_by=%s", blockedBy)
			}
			logEvent(db, "agent_update", agentID, taskID, details)

			// Output
			fmt.Printf("Updated %s -> %s\n", agentID, status)
			if taskID != "" {
				fmt.Printf("  Task: %s\n", taskID)
			}
			if taskPhase != "" {
				fmt.Printf("  Phase: %s\n", taskPhase)
			}
			if statusReason != "" {
				fmt.Printf("  Reason: %s\n", statusReason)
			}
			if blockedBy != "" {
				fmt.Printf("  Blocked by: %s\n", blockedBy)
			}
			if clearTask {
				fmt.Println("  (task cleared)")
			}
		},
	}
	updateCmd.Flags().StringVarP(&taskID, "task", "t", "", "Current Task ID")
	updateCmd.Flags().BoolVar(&clearTask, "clear-task", false, "Clear current task assignment")
	updateCmd.Flags().StringVar(&statusReason, "reason", "", "Reason for status (v5.0)")
	updateCmd.Flags().StringVar(&statusDetail, "detail", "", "Detailed status info (v5.0)")
	updateCmd.Flags().StringVar(&blockedBy, "blocked-by", "", "Agent ID blocking this agent (v5.0)")
	updateCmd.Flags().StringVar(&taskPhase, "phase", "", "Current task phase: coding, testing, review, etc. (v5.0)")

	// ===================
	// TASK Command
	// ===================
	var taskPriority int
	var taskParent string
	var taskDescription string
	var taskCmd = &cobra.Command{
		Use:   "task [subcommand]",
		Short: "Manage tasks",
		Long: `Task management commands:
  add [id] [title]     - Add a new task
  complete [id]        - Mark task as done
  assign [id] [agent]  - Assign task to agent
  start [id]           - Mark task as in_progress
  list                 - List all tasks
  delete [id]          - Delete a task
  update [id]          - Update task details
  reopen [id]          - Reopen a completed task

Dependency commands:
  block [id] [dep-id]  - Block task until dependency is done
  unblock [id] [dep-id]- Remove a dependency
  deps [id]            - Show task dependencies`,
	}

	// TASK ADD
	var taskAddCmd = &cobra.Command{
		Use:   "add [id] [title]",
		Short: "Add a new task",
		Args:  cobra.MinimumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			db := initDB()
			defer db.Close()

			taskID := args[0]
			title := args[1]

			_, err := db.Exec(`
				INSERT OR REPLACE INTO tasks (id, title, description, status, priority, parent, created_at, updated_at)
				VALUES (?, ?, ?, 'todo', ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
				taskID, title, taskDescription, taskPriority, taskParent)
			if err != nil {
				log.Fatal(err)
			}
			fmt.Printf("Task %s added to queue.\n", taskID)
			if taskParent != "" {
				fmt.Printf("  Parent: %s\n", taskParent)
			}
		},
	}
	taskAddCmd.Flags().IntVarP(&taskPriority, "priority", "p", 1, "Task priority (higher = more important)")
	taskAddCmd.Flags().StringVar(&taskParent, "parent", "", "Parent task ID for subtasks")
	taskAddCmd.Flags().StringVarP(&taskDescription, "description", "d", "", "Task description")

	// TASK COMPLETE
	var taskCompleteCmd = &cobra.Command{
		Use:   "complete [id]",
		Short: "Mark task as done",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			db := initDB()
			defer db.Close()

			taskID := args[0]

			// Get old status for history
			var oldStatus string
			db.QueryRow("SELECT status FROM tasks WHERE id = ?", taskID).Scan(&oldStatus)

			_, err := db.Exec("UPDATE tasks SET status='done', updated_at = CURRENT_TIMESTAMP WHERE id=?", taskID)
			if err != nil {
				log.Fatal(err)
			}

			// Log history
			logTaskHistory(db, taskID, oldStatus, "done", "cli")

			// Log event and increment stats
			logEvent(db, "task_complete", "", taskID, fmt.Sprintf("completed from status=%s", oldStatus))
			incrementStat(db, "tasks_completed")

			// Clear from any agent working on it
			db.Exec("UPDATE agents SET task = NULL WHERE task = ?", taskID)

			fmt.Printf("Task %s marked done.\n", taskID)
		},
	}

	// TASK START
	var forceStart bool
	var taskStartCmd = &cobra.Command{
		Use:   "start [id]",
		Short: "Mark task as in_progress",
		Long: `Mark a task as in_progress. If the task has dependencies that are not
yet completed, it will be blocked unless --force is used.`,
		Args: cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			db := initDB()
			defer db.Close()

			taskID := args[0]

			// Check if task exists
			if !taskExists(db, taskID) {
				log.Fatalf("Task '%s' does not exist.", taskID)
			}

			// Check dependencies
			satisfied, blocking := checkDependencies(db, taskID)
			if !satisfied && !forceStart {
				fmt.Printf("❌ Cannot start task %s - blocked by incomplete dependencies:\n", taskID)
				for _, b := range blocking {
					fmt.Printf("   • %s\n", b)
				}
				fmt.Println("\nComplete the blocking tasks first, or use --force to override.")
				return
			}

			if !satisfied && forceStart {
				fmt.Printf("⚠️  Warning: Starting task %s despite incomplete dependencies\n", taskID)
			}

			var oldStatus string
			db.QueryRow("SELECT status FROM tasks WHERE id = ?", taskID).Scan(&oldStatus)

			_, err := db.Exec("UPDATE tasks SET status='in_progress', updated_at = CURRENT_TIMESTAMP WHERE id=?", taskID)
			if err != nil {
				log.Fatal(err)
			}

			logTaskHistory(db, taskID, oldStatus, "in_progress", "cli")
			logEvent(db, "task_start", "", taskID, fmt.Sprintf("started from status=%s", oldStatus))

			fmt.Printf("Task %s marked in_progress.\n", taskID)
		},
	}
	taskStartCmd.Flags().BoolVarP(&forceStart, "force", "f", false, "Start task even if dependencies are not satisfied")

	// TASK ASSIGN
	var autoAssign bool
	var taskAssignCmd = &cobra.Command{
		Use:   "assign [task_id] [agent_id]",
		Short: "Assign task to agent",
		Long: `Assign a task to a specific agent, or auto-assign to best matching idle agent.

Examples:
  flip task assign TASK-071 claude2       # Assign to specific agent
  flip task assign TASK-071 --auto        # Auto-assign based on capabilities`,
		Args: cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			db := initDB()
			defer db.Close()

			taskID := args[0]

			// Verify task exists
			if !taskExists(db, taskID) {
				log.Fatalf("Task '%s' does not exist. Use 'flip task add %s \"Title\"' to create it first.", taskID, taskID)
			}

			var agentID string

			if autoAssign {
				// Get task title and description for matching
				var title, description sql.NullString
				db.QueryRow("SELECT title, description FROM tasks WHERE id = ?", taskID).Scan(&title, &description)
				taskText := strings.ToLower(title.String + " " + description.String)

				// Find idle agents with capabilities
				rows, err := db.Query(`
					SELECT id, COALESCE(capabilities, '') FROM agents
					WHERE status = 'idle'
					ORDER BY hb DESC`)
				if err != nil {
					log.Fatal(err)
				}
				defer rows.Close()

				type agentMatch struct {
					id    string
					score int
				}
				var candidates []agentMatch

				for rows.Next() {
					var id, caps string
					rows.Scan(&id, &caps)

					score := 0
					capsLower := strings.ToLower(caps)

					// Score based on capability keyword matches
					capsList := strings.Split(capsLower, ",")
					for _, cap := range capsList {
						cap = strings.TrimSpace(cap)
						if cap != "" && strings.Contains(taskText, cap) {
							score += 10
						}
					}

					// Common keywords matching
					keywords := []string{"frontend", "backend", "api", "ui", "database", "auth", "test", "bug", "fix", "feature", "refactor", "css", "react", "supabase"}
					for _, kw := range keywords {
						if strings.Contains(taskText, kw) && strings.Contains(capsLower, kw) {
							score += 5
						}
					}

					// Idle agents always get base score
					candidates = append(candidates, agentMatch{id: id, score: score})
				}

				if len(candidates) == 0 {
					log.Fatal("No idle agents available. Use 'flip status' to check agent availability.")
				}

				// Sort by score (highest first), then pick the best
				bestAgent := candidates[0]
				for _, c := range candidates[1:] {
					if c.score > bestAgent.score {
						bestAgent = c
					}
				}

				agentID = bestAgent.id
				fmt.Printf("Auto-assigning based on capabilities (score: %d)\n", bestAgent.score)

				if bestAgent.score == 0 {
					fmt.Println("  (No capability matches - assigned to least recently active idle agent)")
				}
			} else {
				if len(args) < 2 {
					log.Fatal("Usage: flip task assign [task_id] [agent_id] or use --auto")
				}
				agentID = args[1]

				// Verify agent exists
				var agentExists int
				err := db.QueryRow("SELECT COUNT(*) FROM agents WHERE id = ?", agentID).Scan(&agentExists)
				if err != nil || agentExists == 0 {
					log.Fatalf("Agent '%s' is not registered. Use 'flip register %s' first.", agentID, agentID)
				}

				// Check capability match and warn if mismatch
				var agentCaps string
				db.QueryRow("SELECT COALESCE(capabilities, '') FROM agents WHERE id = ?", agentID).Scan(&agentCaps)

				if agentCaps != "" {
					// Get task details for capability checking
					var title, description sql.NullString
					db.QueryRow("SELECT title, description FROM tasks WHERE id = ?", taskID).Scan(&title, &description)
					taskText := strings.ToLower(title.String + " " + description.String)
					capsLower := strings.ToLower(agentCaps)

					// Check for common capability keywords in task that agent doesn't have
					commonCaps := []string{"browser", "test", "sql", "review", "frontend", "backend", "api"}
					missingCaps := []string{}
					for _, cap := range commonCaps {
						if strings.Contains(taskText, cap) && !strings.Contains(capsLower, cap) {
							missingCaps = append(missingCaps, cap)
						}
					}

					if len(missingCaps) > 0 {
						fmt.Printf("⚠️  Warning: Task mentions capabilities that %s may not have: %s\n", agentID, strings.Join(missingCaps, ", "))
						fmt.Printf("   Agent capabilities: %s\n", agentCaps)
						fmt.Printf("   Consider using: flip task assign %s --auto\n", taskID)
					}
				}
			}

			_, err := db.Exec("UPDATE tasks SET assignee=?, updated_at = CURRENT_TIMESTAMP WHERE id=?", agentID, taskID)
			if err != nil {
				log.Fatal(err)
			}
			fmt.Printf("Task %s assigned to %s.\n", taskID, agentID)

			// Auto-verify: Show the updated task
			var title, status, assignee string
			err = db.QueryRow("SELECT title, status, COALESCE(assignee, '-') FROM tasks WHERE id = ?", taskID).Scan(&title, &status, &assignee)
			if err == nil {
				fmt.Printf("\n✓ Verified: [%s] %s | Status: %s | Assignee: %s\n", taskID, title, status, assignee)
			}

			// Show agent capabilities if auto-assigned
			if autoAssign {
				var caps string
				db.QueryRow("SELECT COALESCE(capabilities, '') FROM agents WHERE id = ?", agentID).Scan(&caps)
				if caps != "" {
					fmt.Printf("  Agent capabilities: %s\n", caps)
				}
			}
		},
	}
	taskAssignCmd.Flags().BoolVarP(&autoAssign, "auto", "a", false, "Auto-assign to best matching idle agent")

	// TASK LIST (NEW)
	var jsonOutput bool
	var filterStatus string
	var taskListCmd = &cobra.Command{
		Use:   "list",
		Short: "List all tasks",
		Long: `List all tasks with optional filtering and output formats.

Examples:
  flip task list
  flip task list --json
  flip task list --status todo
  flip task list --status in_progress --json`,
		Run: func(cmd *cobra.Command, args []string) {
			db := initDB()
			defer db.Close()

			query := `SELECT id, title, COALESCE(description, ''), status, COALESCE(assignee, ''), priority, COALESCE(parent, '')
				FROM tasks`
			queryArgs := []interface{}{}

			if filterStatus != "" {
				query += " WHERE status = ?"
				queryArgs = append(queryArgs, filterStatus)
			}
			query += " ORDER BY priority DESC, status, id"

			rows, err := db.Query(query, queryArgs...)
			if err != nil {
				log.Fatal(err)
			}
			defer rows.Close()

			type TaskJSON struct {
				ID          string `json:"id"`
				Title       string `json:"title"`
				Description string `json:"description,omitempty"`
				Status      string `json:"status"`
				Assignee    string `json:"assignee,omitempty"`
				Priority    int    `json:"priority"`
				Parent      string `json:"parent,omitempty"`
			}

			var tasks []TaskJSON

			for rows.Next() {
				var id, title, description, status, assignee, parent string
				var prio int
				rows.Scan(&id, &title, &description, &status, &assignee, &prio, &parent)

				if jsonOutput {
					t := TaskJSON{ID: id, Title: title, Description: description, Status: status, Priority: prio}
					if assignee != "" && assignee != "-" {
						t.Assignee = assignee
					}
					if parent != "" && parent != "-" {
						t.Parent = parent
					}
					tasks = append(tasks, t)
				} else {
					// Truncate title if too long
					titleDisplay := title
					if len(titleDisplay) > 30 {
						titleDisplay = titleDisplay[:27] + "..."
					}

					// Color coding via symbols
					statusSymbol := ""
					switch status {
					case "todo":
						statusSymbol = "[ ] todo"
					case "in_progress":
						statusSymbol = "[~] active"
					case "done":
						statusSymbol = "[x] done"
					}

					if assignee == "" {
						assignee = "-"
					}
					if parent == "" {
						parent = "-"
					}

					// Print header on first iteration
					if len(tasks) == 0 {
						fmt.Println("ID           | STATUS      | PRIO | ASSIGNEE   | PARENT     | TITLE")
						fmt.Println("-------------+-------------+------+------------+------------+--------------------------------")
					}
					tasks = append(tasks, TaskJSON{}) // Just to track we started

					fmt.Printf("%-12s | %-11s | %-4d | %-10s | %-10s | %s\n",
						id, statusSymbol, prio, assignee, parent, titleDisplay)
				}
			}

			if jsonOutput {
				output, _ := json.MarshalIndent(tasks, "", "  ")
				fmt.Println(string(output))
			}
		},
	}
	taskListCmd.Flags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")
	taskListCmd.Flags().StringVar(&filterStatus, "status", "", "Filter by status (todo, in_progress, done)")

	// TASK DELETE (NEW)
	var taskDeleteCmd = &cobra.Command{
		Use:   "delete [id]",
		Short: "Delete a task",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			db := initDB()
			defer db.Close()

			taskID := args[0]

			// Check if task exists
			if !taskExists(db, taskID) {
				log.Fatalf("Task '%s' does not exist.", taskID)
			}

			// Clear from any agent working on it
			db.Exec("UPDATE agents SET task = NULL WHERE task = ?", taskID)

			// Delete the task
			_, err := db.Exec("DELETE FROM tasks WHERE id = ?", taskID)
			if err != nil {
				log.Fatal(err)
			}

			fmt.Printf("Task %s deleted.\n", taskID)
		},
	}

	// TASK UPDATE (NEW)
	var updateTitle string
	var updateDesc string
	var updatePriority int
	var taskUpdateCmd = &cobra.Command{
		Use:   "update [id]",
		Short: "Update task details",
		Long: `Update a task's title, description, or priority.

Examples:
  flip task update TASK-001 --title "New title"
  flip task update TASK-001 --desc "New description"
  flip task update TASK-001 --priority 5`,
		Args: cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			db := initDB()
			defer db.Close()

			taskID := args[0]

			if !taskExists(db, taskID) {
				log.Fatalf("Task '%s' does not exist.", taskID)
			}

			updates := []string{}
			values := []interface{}{}

			if updateTitle != "" {
				updates = append(updates, "title = ?")
				values = append(values, updateTitle)
			}
			if updateDesc != "" {
				updates = append(updates, "description = ?")
				values = append(values, updateDesc)
			}
			if updatePriority > 0 {
				updates = append(updates, "priority = ?")
				values = append(values, updatePriority)
			}

			if len(updates) == 0 {
				fmt.Println("No updates specified. Use --title, --desc, or --priority flags.")
				return
			}

			updates = append(updates, "updated_at = CURRENT_TIMESTAMP")
			values = append(values, taskID)

			query := fmt.Sprintf("UPDATE tasks SET %s WHERE id = ?", strings.Join(updates, ", "))
			_, err := db.Exec(query, values...)
			if err != nil {
				log.Fatal(err)
			}

			fmt.Printf("Task %s updated.\n", taskID)
		},
	}
	taskUpdateCmd.Flags().StringVar(&updateTitle, "title", "", "New task title")
	taskUpdateCmd.Flags().StringVar(&updateDesc, "desc", "", "New task description")
	taskUpdateCmd.Flags().IntVar(&updatePriority, "priority", 0, "New task priority")

	// TASK REOPEN (NEW)
	var taskReopenCmd = &cobra.Command{
		Use:   "reopen [id]",
		Short: "Reopen a completed task (done → todo)",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			db := initDB()
			defer db.Close()

			taskID := args[0]

			var oldStatus string
			err := db.QueryRow("SELECT status FROM tasks WHERE id = ?", taskID).Scan(&oldStatus)
			if err != nil {
				log.Fatalf("Task '%s' not found.", taskID)
			}

			if oldStatus != "done" {
				fmt.Printf("Task %s is not done (current status: %s). Use 'flip task start' instead.\n", taskID, oldStatus)
				return
			}

			_, err = db.Exec("UPDATE tasks SET status = 'todo', updated_at = CURRENT_TIMESTAMP WHERE id = ?", taskID)
			if err != nil {
				log.Fatal(err)
			}

			logTaskHistory(db, taskID, oldStatus, "todo", "cli")

			fmt.Printf("Task %s reopened (done → todo).\n", taskID)
		},
	}

	// TASK BLOCK (Add dependency)
	var taskBlockCmd = &cobra.Command{
		Use:   "block [task-id] [blocked-by-task-id]",
		Short: "Add a dependency (block task until another is done)",
		Long: `Add a dependency between tasks. The first task will be blocked
until the second task is completed.

Examples:
  flip task block TASK-002 TASK-001   # TASK-002 depends on TASK-001
  flip task block TASK-003 TASK-001   # TASK-003 also depends on TASK-001`,
		Args: cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			db := initDB()
			defer db.Close()

			taskID := args[0]
			blockedByID := args[1]

			// Validate both tasks exist
			if !taskExists(db, taskID) {
				log.Fatalf("Task '%s' does not exist.", taskID)
			}
			if !taskExists(db, blockedByID) {
				log.Fatalf("Task '%s' does not exist.", blockedByID)
			}

			// Prevent self-dependency
			if taskID == blockedByID {
				log.Fatalf("A task cannot depend on itself.")
			}

			// Add dependency
			err := addDependency(db, taskID, blockedByID)
			if err != nil {
				log.Fatalf("Failed to add dependency: %v", err)
			}

			// Get task titles for display
			var taskTitle, blockedByTitle string
			db.QueryRow("SELECT title FROM tasks WHERE id = ?", taskID).Scan(&taskTitle)
			db.QueryRow("SELECT title FROM tasks WHERE id = ?", blockedByID).Scan(&blockedByTitle)

			fmt.Printf("✓ Added dependency: %s → %s\n", taskID, blockedByID)
			fmt.Printf("  '%s' is now blocked until '%s' is completed.\n", taskTitle, blockedByTitle)

			// Show all dependencies
			deps := getTaskDependencies(db, taskID)
			if len(deps) > 1 {
				fmt.Printf("\n  All dependencies for %s: %s\n", taskID, strings.Join(deps, ", "))
			}
		},
	}

	// TASK UNBLOCK (Remove dependency)
	var taskUnblockCmd = &cobra.Command{
		Use:   "unblock [task-id] [blocked-by-task-id]",
		Short: "Remove a dependency",
		Long: `Remove a dependency between tasks.

Examples:
  flip task unblock TASK-002 TASK-001   # Remove TASK-001 from TASK-002's dependencies`,
		Args: cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			db := initDB()
			defer db.Close()

			taskID := args[0]
			blockedByID := args[1]

			// Validate task exists
			if !taskExists(db, taskID) {
				log.Fatalf("Task '%s' does not exist.", taskID)
			}

			// Remove dependency
			err := removeDependency(db, taskID, blockedByID)
			if err != nil {
				log.Fatalf("Failed to remove dependency: %v", err)
			}

			fmt.Printf("✓ Removed dependency: %s no longer depends on %s\n", taskID, blockedByID)

			// Show remaining dependencies
			deps := getTaskDependencies(db, taskID)
			if len(deps) > 0 {
				fmt.Printf("  Remaining dependencies: %s\n", strings.Join(deps, ", "))
			} else {
				fmt.Printf("  Task %s has no remaining dependencies.\n", taskID)
			}
		},
	}

	// TASK DEPS (Show dependencies)
	var taskDepsCmd = &cobra.Command{
		Use:   "deps [task-id]",
		Short: "Show task dependencies",
		Long:  "Display all dependencies for a task and their current status.",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			db := initDB()
			defer db.Close()

			taskID := args[0]

			if !taskExists(db, taskID) {
				log.Fatalf("Task '%s' does not exist.", taskID)
			}

			deps := getTaskDependencies(db, taskID)
			if len(deps) == 0 {
				fmt.Printf("Task %s has no dependencies.\n", taskID)
				return
			}

			var taskTitle string
			db.QueryRow("SELECT title FROM tasks WHERE id = ?", taskID).Scan(&taskTitle)

			fmt.Printf("Dependencies for %s (%s):\n", taskID, taskTitle)
			fmt.Println("─────────────────────────────────────")

			allDone := true
			for _, depID := range deps {
				var title, status string
				err := db.QueryRow("SELECT title, status FROM tasks WHERE id = ?", depID).Scan(&title, &status)
				if err != nil {
					fmt.Printf("  ❓ %s - not found\n", depID)
					allDone = false
				} else {
					icon := "⬜"
					if status == "done" {
						icon = "✅"
					} else if status == "in_progress" {
						icon = "🔄"
						allDone = false
					} else {
						allDone = false
					}
					fmt.Printf("  %s %s (%s) - %s\n", icon, depID, status, title)
				}
			}

			fmt.Println("─────────────────────────────────────")
			if allDone {
				fmt.Println("✓ All dependencies satisfied - task can be started")
			} else {
				fmt.Println("⚠ Some dependencies not complete - task is blocked")
			}
		},
	}

	// TASK PROGRESS (Set progress percentage)
	var taskProgressCmd = &cobra.Command{
		Use:   "progress [task-id] [percentage]",
		Short: "Set task progress percentage",
		Long: `Set the progress percentage (0-100) for a task.

Examples:
  flip task progress TASK-071 25    # 25% complete
  flip task progress TASK-071 100   # Marks as complete`,
		Args: cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			db := initDB()
			defer db.Close()

			taskID := args[0]

			if !taskExists(db, taskID) {
				log.Fatalf("Task '%s' does not exist.", taskID)
			}

			var progress int
			_, err := fmt.Sscanf(args[1], "%d", &progress)
			if err != nil || progress < 0 || progress > 100 {
				log.Fatal("Progress must be a number between 0 and 100")
			}

			_, err = db.Exec("UPDATE tasks SET progress = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", progress, taskID)
			if err != nil {
				log.Fatal(err)
			}

			// If 100%, offer to mark complete
			if progress == 100 {
				fmt.Printf("Task %s progress set to 100%%\n", taskID)
				fmt.Println("  → To mark complete: ./flip task complete " + taskID)
			} else {
				// Show progress bar
				barWidth := 20
				filled := progress * barWidth / 100
				bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
				fmt.Printf("Task %s: [%s] %d%%\n", taskID, bar, progress)
			}
		},
	}

	// TASK TEMPLATE (create from template)
	var templateTitle string
	var templateAssign string
	var taskTemplateCmd = &cobra.Command{
		Use:   "template [template-name]",
		Short: "Create task(s) from a template",
		Long: `Create one or more tasks from a predefined template.

Available templates:
  bug-fix      - Bug fix workflow (investigate, fix, test, review)
  feature      - New feature workflow (design, implement, test, docs)
  refactor     - Code refactoring (analyze, refactor, test)
  hotfix       - Emergency fix (fix, deploy, verify)
  review       - Code review task

Examples:
  flip task template bug-fix --title "Fix login crash"
  flip task template feature --title "Add dark mode" --assign claude2`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			db := initDB()
			defer db.Close()

			templateName := args[0]
			if templateTitle == "" {
				templateTitle = strings.Title(strings.ReplaceAll(templateName, "-", " "))
			}

			// Template definitions
			type TaskDef struct {
				suffix      string
				title       string
				description string
				priority    int
			}

			templates := map[string][]TaskDef{
				"bug-fix": {
					{"INV", "Investigate: %s", "Reproduce the bug, identify root cause, document findings", 3},
					{"FIX", "Fix: %s", "Implement the fix for the identified issue", 3},
					{"TEST", "Test: %s", "Write tests and verify the fix works correctly", 2},
					{"REVIEW", "Review: %s", "Code review and final verification", 1},
				},
				"feature": {
					{"DESIGN", "Design: %s", "Design the feature, create specs, identify components", 3},
					{"IMPL", "Implement: %s", "Build the core functionality", 3},
					{"TEST", "Test: %s", "Write tests and manual testing", 2},
					{"DOCS", "Document: %s", "Update documentation and add comments", 1},
				},
				"refactor": {
					{"ANALYZE", "Analyze: %s", "Identify areas to refactor, plan approach", 2},
					{"REFACTOR", "Refactor: %s", "Perform the refactoring", 3},
					{"TEST", "Verify: %s", "Run tests, ensure no regressions", 2},
				},
				"hotfix": {
					{"FIX", "Hotfix: %s", "Emergency fix implementation", 5},
					{"DEPLOY", "Deploy: %s", "Deploy the fix to production", 5},
					{"VERIFY", "Verify: %s", "Verify fix is working in production", 4},
				},
				"review": {
					{"REVIEW", "Review: %s", "Review code changes, provide feedback", 2},
				},
			}

			tasks, ok := templates[templateName]
			if !ok {
				fmt.Printf("Unknown template '%s'. Available: bug-fix, feature, refactor, hotfix, review\n", templateName)
				return
			}

			// Generate base task ID
			baseID := fmt.Sprintf("TASK-%03d", time.Now().Unix()%1000)

			fmt.Printf("📋 Creating tasks from '%s' template:\n", templateName)
			fmt.Println(strings.Repeat("-", 50))

			var createdIDs []string
			for i, t := range tasks {
				taskID := fmt.Sprintf("%s-%s", baseID, t.suffix)
				title := fmt.Sprintf(t.title, templateTitle)

				_, err := db.Exec(`
					INSERT INTO tasks (id, title, description, status, priority, assignee)
					VALUES (?, ?, ?, 'todo', ?, ?)`,
					taskID, title, t.description, t.priority, sql.NullString{String: templateAssign, Valid: templateAssign != ""})

				if err != nil {
					fmt.Printf("  ❌ %s: %v\n", taskID, err)
				} else {
					fmt.Printf("  ✓ [%s] %s (P%d)\n", taskID, title, t.priority)
					createdIDs = append(createdIDs, taskID)
				}

				// Add dependency: each task depends on previous
				if i > 0 && len(createdIDs) > 1 {
					prevID := createdIDs[i-1]
					addDependency(db, taskID, prevID)
					fmt.Printf("      └─ depends on %s\n", prevID)
				}
			}

			fmt.Println(strings.Repeat("-", 50))
			fmt.Printf("Created %d task(s).\n", len(createdIDs))
			if templateAssign != "" {
				fmt.Printf("Assigned to: %s\n", templateAssign)
			}
			fmt.Printf("\nStart with: ./flip task start %s\n", createdIDs[0])
		},
	}
	taskTemplateCmd.Flags().StringVarP(&templateTitle, "title", "t", "", "Title for the task group")
	taskTemplateCmd.Flags().StringVarP(&templateAssign, "assign", "a", "", "Assign all tasks to agent")

	taskCmd.AddCommand(taskAddCmd, taskCompleteCmd, taskStartCmd, taskAssignCmd, taskListCmd, taskDeleteCmd, taskUpdateCmd, taskReopenCmd, taskBlockCmd, taskUnblockCmd, taskDepsCmd, taskProgressCmd, taskTemplateCmd)

	// ===================
	// QUEUE Command
	// ===================
	var queueCmd = &cobra.Command{
		Use:   "queue",
		Short: "List unassigned tasks in queue",
		Run: func(cmd *cobra.Command, args []string) {
			db := initDB()
			defer db.Close()

			rows, err := db.Query(`
				SELECT id, title, priority, COALESCE(description, '')
				FROM tasks
				WHERE status='todo' AND (assignee IS NULL OR assignee = '')
				ORDER BY priority DESC`)
			if err != nil {
				log.Fatal(err)
			}
			defer rows.Close()

			fmt.Println("ID           | PRIO | TITLE                          | DESCRIPTION")
			fmt.Println("-------------+------+--------------------------------+-------------------")
			for rows.Next() {
				var id, title, description string
				var prio int
				rows.Scan(&id, &title, &prio, &description)

				titleDisplay := title
				if len(titleDisplay) > 30 {
					titleDisplay = titleDisplay[:27] + "..."
				}
				descDisplay := description
				if len(descDisplay) > 17 {
					descDisplay = descDisplay[:14] + "..."
				}

				fmt.Printf("%-12s | %-4d | %-30s | %s\n", id, prio, titleDisplay, descDisplay)
			}
		},
	}

	// ===================
	// SIGNAL Command (NEW)
	// ===================
	var signalCmd = &cobra.Command{
		Use:   "signal [subcommand]",
		Short: "Send and receive messages between agents",
		Long: `Inter-agent messaging system:
  send [to_agent] [message]  - Send a message
  read                       - Read unread messages
  history                    - View all messages`,
	}

	// SIGNAL SEND (v5.0 - with priority system)
	var fromAgent string
	var expectReply bool
	var signalPriority string
	var signalType string
	var signalSubject string
	var signalRelatedTask string
	var signalExpireHours int
	var signalSendCmd = &cobra.Command{
		Use:   "send [to_agent] [message]",
		Short: "Send a message to another agent",
		Long: `Send a message to another agent with optional priority.

Priority levels (v5.0):
  critical - Agent crash, task fail, blocking - immediate push to coordinator
  high     - Escalation, handoff, verification - read within 15min
  normal   - Status updates, progress (default)
  low      - FYI, logging - read when convenient

Examples:
  flip signal send claude2 "What's your status?"
  flip signal send claude "TASK-089 BLOCKED 2hrs" --priority critical
  flip signal send antigravity "Ready for verification" --priority high --task TASK-089
  flip signal send claude2 "FYI: build is broken" --priority low -f claude`,
		Args: cobra.MinimumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			db := initDB()
			defer db.Close()

			toAgent := args[0]
			message := strings.Join(args[1:], " ")
			msgID := uuid.New().String()[:8]

			if fromAgent == "" {
				fromAgent = "cli"
			}

			// Validate priority
			if signalPriority == "" {
				signalPriority = PriorityNormal
			}
			if !validPriorities[signalPriority] {
				log.Fatalf("Invalid priority '%s'. Valid: critical, high, normal, low", signalPriority)
			}

			// Calculate expiration if set
			var expiresAt *time.Time
			if signalExpireHours > 0 {
				t := time.Now().Add(time.Duration(signalExpireHours) * time.Hour)
				expiresAt = &t
			}

			_, err := db.Exec(`
				INSERT INTO signals (id, to_agent, from_agent, msg, priority, signal_type, subject, related_task, expects_reply, expires_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				msgID, toAgent, fromAgent, message, signalPriority, signalType, signalSubject, signalRelatedTask, expectReply, expiresAt)
			if err != nil {
				log.Fatal(err)
			}

			// Log event and increment stats
			preview := message
			if len(preview) > 50 {
				preview = preview[:50] + "..."
			}
			logEvent(db, "signal_sent", fromAgent, signalRelatedTask, fmt.Sprintf("to=%s priority=%s msg_preview=%s", toAgent, signalPriority, preview))
			incrementStat(db, "signals_sent")

			// Priority indicator
			priorityIcon := ""
			switch signalPriority {
			case PriorityCritical:
				priorityIcon = "🚨 CRITICAL"
			case PriorityHigh:
				priorityIcon = "⚡ HIGH"
			case PriorityNormal:
				priorityIcon = "📨"
			case PriorityLow:
				priorityIcon = "📝 low"
			}

			fmt.Printf("%s Message sent to %s (id: %s)\n", priorityIcon, toAgent, msgID)
			if signalRelatedTask != "" {
				fmt.Printf("  Related task: %s\n", signalRelatedTask)
			}
			if expectReply {
				fmt.Printf("  Expecting reply. Track with: ./flip signal pending\n")
			}
		},
	}
	signalSendCmd.Flags().StringVarP(&fromAgent, "from", "f", "", "Sender agent ID")
	signalSendCmd.Flags().BoolVarP(&expectReply, "expect-reply", "e", false, "Mark as expecting a reply")
	signalSendCmd.Flags().StringVarP(&signalPriority, "priority", "p", "normal", "Priority: critical, high, normal, low (v5.0)")
	signalSendCmd.Flags().StringVar(&signalType, "type", "", "Signal type: escalation, handoff, status, etc. (v5.0)")
	signalSendCmd.Flags().StringVar(&signalSubject, "subject", "", "Short subject line (v5.0)")
	signalSendCmd.Flags().StringVar(&signalRelatedTask, "task", "", "Related task ID (v5.0)")
	signalSendCmd.Flags().IntVar(&signalExpireHours, "expire", 0, "Expire after N hours (v5.0)")

	// SIGNAL READ (v5.0 - with priority filtering)
	var signalReadAgent string
	var signalReadPriority string
	var signalReadNoMark bool
	var signalReadCmd = &cobra.Command{
		Use:   "read",
		Short: "Read unread messages",
		Long: `Read unread messages with optional priority filtering (v5.0).

Examples:
  flip signal read                           # All unread
  flip signal read --priority critical       # Only critical
  flip signal read --priority high           # Only high priority
  flip signal read -a claude                 # For specific agent
  flip signal read --no-mark                 # Don't mark as read`,
		Run: func(cmd *cobra.Command, args []string) {
			db := initDB()
			defer db.Close()

			query := `SELECT id, from_agent, msg, COALESCE(priority, 'normal'), COALESCE(related_task, ''), created_at
				FROM signals WHERE read = 0`
			queryArgs := []interface{}{}

			if signalReadAgent != "" {
				query += " AND to_agent = ?"
				queryArgs = append(queryArgs, signalReadAgent)
			}
			if signalReadPriority != "" {
				if !validPriorities[signalReadPriority] {
					log.Fatalf("Invalid priority '%s'. Valid: critical, high, normal, low", signalReadPriority)
				}
				query += " AND priority = ?"
				queryArgs = append(queryArgs, signalReadPriority)
			}
			// Order by priority (critical first) then time
			query += ` ORDER BY
				CASE priority
					WHEN 'critical' THEN 1
					WHEN 'high' THEN 2
					WHEN 'normal' THEN 3
					WHEN 'low' THEN 4
					ELSE 3
				END, created_at`

			rows, err := db.Query(query, queryArgs...)
			if err != nil {
				log.Fatal(err)
			}

			// Collect messages first to avoid deadlock (can't UPDATE while iterating)
			type message struct {
				id          string
				from        string
				msg         string
				priority    string
				relatedTask string
				createdAt   time.Time
			}
			var messages []message

			for rows.Next() {
				var m message
				rows.Scan(&m.id, &m.from, &m.msg, &m.priority, &m.relatedTask, &m.createdAt)
				messages = append(messages, m)
			}
			rows.Close() // Close BEFORE updating to avoid SQLite deadlock

			fmt.Println("UNREAD MESSAGES:")
			fmt.Println("================")

			// Now safe to display and update
			for _, m := range messages {
				// Priority indicator
				priorityIcon := ""
				switch m.priority {
				case PriorityCritical:
					priorityIcon = "🚨"
				case PriorityHigh:
					priorityIcon = "⚡"
				case PriorityLow:
					priorityIcon = "📝"
				default:
					priorityIcon = "📨"
				}

				fmt.Printf("\n%s [%s] From: %s (%s) [%s]\n", priorityIcon, m.id, m.from, m.createdAt.Format("Jan 02 15:04"), m.priority)
				if m.relatedTask != "" {
					fmt.Printf("   Task: %s\n", m.relatedTask)
				}
				fmt.Printf("   %s\n", m.msg)

				// Mark as read (safe now - rows are closed)
				if !signalReadNoMark {
					db.Exec("UPDATE signals SET read = 1 WHERE id = ?", m.id)
				}
			}

			if len(messages) == 0 {
				if signalReadPriority != "" {
					fmt.Printf("No unread %s priority messages.\n", signalReadPriority)
				} else {
					fmt.Println("No unread messages.")
				}
			} else {
				if signalReadNoMark {
					fmt.Printf("\n%d message(s) displayed (not marked as read).\n", len(messages))
				} else {
					fmt.Printf("\n%d message(s) marked as read.\n", len(messages))
				}
			}
		},
	}
	signalReadCmd.Flags().StringVarP(&signalReadAgent, "agent", "a", "", "Filter by recipient agent")
	signalReadCmd.Flags().StringVarP(&signalReadPriority, "priority", "p", "", "Filter by priority: critical, high, normal, low (v5.0)")
	signalReadCmd.Flags().BoolVar(&signalReadNoMark, "no-mark", false, "Don't mark messages as read (v5.0)")

	// SIGNAL HISTORY
	var signalHistoryCmd = &cobra.Command{
		Use:   "history",
		Short: "View all message history",
		Run: func(cmd *cobra.Command, args []string) {
			db := initDB()
			defer db.Close()

			rows, err := db.Query(`
				SELECT id, from_agent, to_agent, msg, read, created_at
				FROM signals ORDER BY created_at DESC LIMIT 20`)
			if err != nil {
				log.Fatal(err)
			}
			defer rows.Close()

			fmt.Println("MESSAGE HISTORY (last 20):")
			fmt.Println("ID       | FROM       | TO         | READ | TIME      | MESSAGE")
			fmt.Println("---------+------------+------------+------+-----------+-------------------")

			for rows.Next() {
				var id, from, to, msg string
				var read bool
				var createdAt time.Time
				rows.Scan(&id, &from, &to, &msg, &read, &createdAt)

				readStr := "NO"
				if read {
					readStr = "YES"
				}

				msgDisplay := msg
				if len(msgDisplay) > 17 {
					msgDisplay = msgDisplay[:14] + "..."
				}

				fmt.Printf("%-8s | %-10s | %-10s | %-4s | %-9s | %s\n",
					id, from, to, readStr, createdAt.Format("15:04:05"), msgDisplay)
			}
		},
	}

	// SIGNAL REPLY
	var replyFromAgent string
	var signalReplyCmd = &cobra.Command{
		Use:   "reply [msg_id] [message]",
		Short: "Reply to a specific message",
		Long: `Reply to a message, creating a conversation thread.

Examples:
  flip signal reply abc123 "Here's my update on that task"
  flip signal reply abc123 "Done with the fix" -f claude2`,
		Args: cobra.MinimumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			db := initDB()
			defer db.Close()

			replyToID := args[0]
			message := strings.Join(args[1:], " ")
			msgID := uuid.New().String()[:8]

			// Get the original message to find who to reply to
			var originalFrom, originalTo string
			err := db.QueryRow("SELECT from_agent, to_agent FROM signals WHERE id = ?", replyToID).Scan(&originalFrom, &originalTo)
			if err != nil {
				log.Fatalf("Message '%s' not found", replyToID)
			}

			// Reply goes back to the original sender
			toAgent := originalFrom
			if replyFromAgent == "" {
				replyFromAgent = originalTo // Default: reply as the recipient
			}

			_, err = db.Exec(`
				INSERT INTO signals (id, to_agent, from_agent, msg, reply_to)
				VALUES (?, ?, ?, ?, ?)`,
				msgID, toAgent, replyFromAgent, message, replyToID)
			if err != nil {
				log.Fatal(err)
			}

			// Mark original as replied (clear expects_reply)
			db.Exec("UPDATE signals SET expects_reply = 0 WHERE id = ?", replyToID)

			fmt.Printf("Reply sent to %s (id: %s, thread: %s)\n", toAgent, msgID, replyToID)
		},
	}
	signalReplyCmd.Flags().StringVarP(&replyFromAgent, "from", "f", "", "Sender agent ID")

	// SIGNAL PENDING (unanswered messages)
	var signalPendingCmd = &cobra.Command{
		Use:   "pending",
		Short: "Show messages expecting replies",
		Run: func(cmd *cobra.Command, args []string) {
			db := initDB()
			defer db.Close()

			rows, err := db.Query(`
				SELECT s.id, s.from_agent, s.to_agent, s.msg, s.created_at,
					   (SELECT COUNT(*) FROM signals WHERE reply_to = s.id) as reply_count
				FROM signals s
				WHERE s.expects_reply = 1
				ORDER BY s.created_at DESC`)
			if err != nil {
				log.Fatal(err)
			}
			defer rows.Close()

			fmt.Println("PENDING REPLIES:")
			fmt.Println("================")

			count := 0
			for rows.Next() {
				var id, from, to, msg string
				var createdAt time.Time
				var replyCount int
				rows.Scan(&id, &from, &to, &msg, &createdAt, &replyCount)

				status := "⏳ WAITING"
				if replyCount > 0 {
					status = fmt.Sprintf("✅ %d REPLIES", replyCount)
				}

				msgDisplay := msg
				if len(msgDisplay) > 50 {
					msgDisplay = msgDisplay[:47] + "..."
				}

				fmt.Printf("\n[%s] %s -> %s (%s)\n", id, from, to, status)
				fmt.Printf("   %s\n", msgDisplay)
				fmt.Printf("   Sent: %s\n", createdAt.Format("Jan 02 15:04"))
				count++
			}

			if count == 0 {
				fmt.Println("No messages pending reply.")
			}
		},
	}

	// SIGNAL THREAD (view conversation)
	var signalThreadCmd = &cobra.Command{
		Use:   "thread [msg_id]",
		Short: "View a conversation thread",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			db := initDB()
			defer db.Close()

			rootID := args[0]

			// Find the root of this thread (follow reply_to chain up)
			currentID := rootID
			for {
				var replyTo sql.NullString
				err := db.QueryRow("SELECT reply_to FROM signals WHERE id = ?", currentID).Scan(&replyTo)
				if err != nil || !replyTo.Valid || replyTo.String == "" {
					break
				}
				currentID = replyTo.String
			}
			rootID = currentID

			// Now get all messages in this thread
			fmt.Println("CONVERSATION THREAD:")
			fmt.Println("====================")

			var printThread func(parentID string, indent int)
			printThread = func(parentID string, indent int) {
				var id, from, to, msg string
				var createdAt time.Time
				err := db.QueryRow(`
					SELECT id, from_agent, to_agent, msg, created_at
					FROM signals WHERE id = ?`, parentID).Scan(&id, &from, &to, &msg, &createdAt)
				if err != nil {
					return
				}

				prefix := strings.Repeat("  ", indent)
				if indent > 0 {
					prefix = strings.Repeat("  ", indent-1) + "↳ "
				}

				fmt.Printf("\n%s[%s] %s -> %s (%s)\n", prefix, id, from, to, createdAt.Format("Jan 02 15:04"))
				fmt.Printf("%s   %s\n", strings.Repeat("  ", indent), msg)

				// Find replies to this message
				rows, err := db.Query("SELECT id FROM signals WHERE reply_to = ? ORDER BY created_at", parentID)
				if err != nil {
					return
				}
				defer rows.Close()
				for rows.Next() {
					var replyID string
					rows.Scan(&replyID)
					printThread(replyID, indent+1)
				}
			}

			printThread(rootID, 0)
		},
	}

	// SIGNAL BROADCAST (send to multiple agents)
	var broadcastFrom string
	var broadcastFilter string
	var signalBroadcastCmd = &cobra.Command{
		Use:   "broadcast [message]",
		Short: "Send message to multiple agents",
		Long: `Broadcast a message to all agents or filtered by status.

Filters:
  @all   - All registered agents (default)
  @busy  - Only busy agents
  @idle  - Only idle agents

Examples:
  flip signal broadcast "Server restarting in 5 mins"
  flip signal broadcast "Need help with TASK-071" --filter @busy
  flip signal broadcast "New tasks available" --filter @idle -f claude`,
		Args: cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			db := initDB()
			defer db.Close()

			message := strings.Join(args, " ")

			if broadcastFrom == "" {
				broadcastFrom = "broadcast"
			}

			// Get agents based on filter
			var query string
			switch broadcastFilter {
			case "@busy":
				query = "SELECT id FROM agents WHERE status = 'busy'"
			case "@idle":
				query = "SELECT id FROM agents WHERE status = 'idle'"
			default:
				query = "SELECT id FROM agents"
			}

			rows, err := db.Query(query)
			if err != nil {
				log.Fatal(err)
			}
			defer rows.Close()

			var agents []string
			for rows.Next() {
				var id string
				rows.Scan(&id)
				agents = append(agents, id)
			}

			if len(agents) == 0 {
				fmt.Printf("No agents match filter '%s'\n", broadcastFilter)
				return
			}

			// Send to each agent
			fmt.Printf("📢 Broadcasting to %d agent(s):\n", len(agents))
			for _, agent := range agents {
				msgID := uuid.New().String()[:8]
				_, err := db.Exec(`
					INSERT INTO signals (id, to_agent, from_agent, msg)
					VALUES (?, ?, ?, ?)`,
					msgID, agent, broadcastFrom, message)
				if err != nil {
					fmt.Printf("  ❌ %s: failed\n", agent)
				} else {
					fmt.Printf("  ✓ %s (id: %s)\n", agent, msgID)
				}
			}
			fmt.Printf("\nMessage: %s\n", message)
		},
	}
	signalBroadcastCmd.Flags().StringVarP(&broadcastFrom, "from", "f", "", "Sender agent ID")
	signalBroadcastCmd.Flags().StringVar(&broadcastFilter, "filter", "@all", "Filter: @all, @busy, @idle")

	signalCmd.AddCommand(signalSendCmd, signalReadCmd, signalHistoryCmd, signalReplyCmd, signalPendingCmd, signalThreadCmd, signalBroadcastCmd)

	// ===================
	// AUDIT Command
	// ===================
	var auditCmd = &cobra.Command{
		Use:   "audit",
		Short: "Check for inconsistencies between DB and log files",
		Run: func(cmd *cobra.Command, args []string) {
			db := initDB()
			defer db.Close()

			rows, err := db.Query("SELECT id, status FROM agents")
			if err != nil {
				log.Fatal(err)
			}
			defer rows.Close()

			fmt.Println("AGENT      | DB STATUS | LOG STATUS | MATCH?")
			fmt.Println("-----------+-----------+------------+-------")

			issues := 0
			for rows.Next() {
				var id, dbStatus string
				rows.Scan(&id, &dbStatus)

				logPath := getContextPath(id)

				logStatus, err := scanLogStatus(logPath)
				if err != nil {
					logStatus = "err"
				}

				match := "YES"
				if dbStatus != logStatus && logStatus != "unknown" && logStatus != "err" {
					match = "NO ⚠"
					issues++
				}

				fmt.Printf("%-10s | %-9s | %-10s | %s\n", id, dbStatus, logStatus, match)
			}

			if issues > 0 {
				fmt.Printf("\n⚠ Found %d inconsistencies. Agents should run './flip update' to sync.\n", issues)
			} else {
				fmt.Println("\n✓ All agents are in sync.")
			}
		},
	}

	// ===================
	// PRUNE Command
	// ===================
	var pruneCmd = &cobra.Command{
		Use:   "prune [agent_id]",
		Short: "Archive old log entries for token optimization",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			id := args[0]
			logPath := getContextPath(id)

			file, err := os.Open(logPath)
			if err != nil {
				log.Fatal(err)
			}
			defer file.Close()

			var lines []string
			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				lines = append(lines, scanner.Text())
			}

			if len(lines) <= 1000 {
				fmt.Printf("Log for %s is short (%d lines). No prune needed.\n", id, len(lines))
				return
			}

			// Slice: Keep Headers (Lines 0-50) + Last 200
			keepHeader := lines[:50]
			keepTail := lines[len(lines)-200:]
			toArchive := lines[50 : len(lines)-200]

			// Write Archive
			root := findProjectRoot()
			archiveDir := filepath.Join(root, "ProjectDocs", "LLMcomms", "archive")
			os.MkdirAll(archiveDir, 0755)
			archivePath := filepath.Join(archiveDir, fmt.Sprintf("%s_%d.md", id, time.Now().Unix()))
			fArch, _ := os.Create(archivePath)
			defer fArch.Close()
			for _, line := range toArchive {
				fArch.WriteString(line + "\n")
			}
			fmt.Printf("Archived %d lines to %s\n", len(toArchive), archivePath)

			// Rewrite Active Log
			fNew, _ := os.Create(logPath)
			defer fNew.Close()
			for _, line := range keepHeader {
				fNew.WriteString(line + "\n")
			}
			fNew.WriteString("\n\n--- [PRUNED FOR TOKEN OPTIMIZATION] ---\n\n")
			for _, line := range keepTail {
				fNew.WriteString(line + "\n")
			}
			fmt.Printf("Pruned %s. New size: %d lines.\n", logPath, 50+200+3)
		},
	}

	// ===================
	// SERVE Command (Dashboard with HTMX)
	// ===================
	var port string
	var serveCmd = &cobra.Command{
		Use:   "serve",
		Short: "Start the Hive Dashboard web interface",
		Run: func(cmd *cobra.Command, args []string) {
			db := initDB()
			defer db.Close()

			http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
				// --- 1. Fetch Agents with Task Details ---
				rows, err := db.Query(`
					SELECT agents.id, agents.status, agents.task, agents.hb,
					       COALESCE(tasks.title, ''), COALESCE(tasks.description, '')
					FROM agents
					LEFT JOIN tasks ON agents.task = tasks.id
					ORDER BY agents.id`)
				if err != nil {
					http.Error(w, err.Error(), 500)
					return
				}
				defer rows.Close()

				type Task struct {
					ID, Title, Description, Status, Assignee string
					Priority, Progress                       int
				}
				type Agent struct {
					ID, Status, TaskID, TaskTitle, TaskDesc, Since, Color string
				}

				agentMap := make(map[string]*Agent)
				var agentOrder []string

				for rows.Next() {
					var id, status string
					var taskID, taskTitle, taskDesc sql.NullString
					var hb time.Time
					rows.Scan(&id, &status, &taskID, &hb, &taskTitle, &taskDesc)

					since := time.Since(hb).Round(time.Second)
					color := "gray"
					if status == "busy" {
						color = "green"
					}
					if status == "blocked" {
						color = "red"
					}

					tID := "-"
					tTitle := ""
					tDesc := ""
					if taskID.Valid && taskID.String != "" {
						tID = taskID.String
					}
					if taskTitle.Valid {
						tTitle = taskTitle.String
					}
					if taskDesc.Valid {
						tDesc = taskDesc.String
					}

					agent := &Agent{
						ID: id, Status: status, TaskID: tID, TaskTitle: tTitle, TaskDesc: tDesc, Since: since.String(), Color: color,
					}
					agentMap[id] = agent
					agentOrder = append(agentOrder, id)
				}

				// --- 2. Fetch Recent Signals (Messages) ---
				type Signal struct {
					ID, From, To, Msg, Time string
				}
				var signals []Signal
				sigRows, err := db.Query(`
					SELECT id, from_agent, to_agent, msg, created_at
					FROM signals WHERE read = 0
					ORDER BY created_at DESC LIMIT 10`)
				if err == nil {
					defer sigRows.Close()
					for sigRows.Next() {
						var id, from, to, msg string
						var created time.Time
						sigRows.Scan(&id, &from, &to, &msg, &created)
						signals = append(signals, Signal{
							ID: id, From: from, To: to, Msg: msg,
							Time: time.Since(created).Round(time.Second).String(),
						})
					}
				}

				// --- 3. Fetch ALL Tasks for Kanban ---
				tRows, err := db.Query(`
					SELECT id, title, COALESCE(description, ''), status, COALESCE(assignee, ''), priority, COALESCE(progress, 0)
					FROM tasks ORDER BY priority DESC, id ASC`)
				if err != nil {
					log.Printf("Warning: Failed to query tasks: %v", err)
				}
				if tRows != nil {
					defer tRows.Close()
				}

				var todo []Task
				var doing []Task
				var done []Task

				for tRows != nil && tRows.Next() {
					var id, title, desc, status string
					var assignee sql.NullString
					var prio, prog int
					tRows.Scan(&id, &title, &desc, &status, &assignee, &prio, &prog)

					owner := "-"
					if assignee.Valid && assignee.String != "" {
						owner = assignee.String
					}

					t := Task{ID: id, Title: title, Description: desc, Status: status, Assignee: owner, Priority: prio, Progress: prog}

					if status == "done" {
						done = append(done, t)
					} else if status == "in_progress" {
						doing = append(doing, t)
					} else {
						// Check if any agent is working on it
						isActive := false
						for _, ag := range agentMap {
							if ag.TaskID == id {
								isActive = true
								break
							}
						}
						if isActive {
							doing = append(doing, t)
						} else {
							todo = append(todo, t)
						}
					}
				}

				// Convert to slice
				var agents []Agent
				for _, id := range agentOrder {
					agents = append(agents, *agentMap[id])
				}

				// Calculate stats
				total := len(todo) + len(doing) + len(done)
				progress := 0
				if total > 0 {
					progress = (len(done) * 100) / total
				}
				busyCount := 0
				for _, a := range agents {
					if a.Status == "busy" {
						busyCount++
					}
				}

				// Get today's metrics
				var todayCompleted, weekCompleted, totalEvents int
				db.QueryRow(`SELECT COUNT(*) FROM task_history
					WHERE new_status = 'done' AND DATE(changed_at) = DATE('now')`).Scan(&todayCompleted)
				db.QueryRow(`SELECT COUNT(*) FROM task_history
					WHERE new_status = 'done' AND changed_at >= DATE('now', '-7 days')`).Scan(&weekCompleted)
				db.QueryRow("SELECT COUNT(*) FROM events").Scan(&totalEvents)

				// Query cost tracking data
				var totalCost float64
				var totalTokens int
				db.QueryRow("SELECT COALESCE(SUM(cost_usd), 0), COALESCE(SUM(input_tokens + output_tokens), 0) FROM task_results").Scan(&totalCost, &totalTokens)

				// Query registered agents count
				var registeredAgents int
				db.QueryRow("SELECT COUNT(DISTINCT id) FROM agents WHERE capabilities IS NOT NULL AND capabilities != ''").Scan(&registeredAgents)

				// Check RAG status (OpenAI key present)
				ragEnabled := os.Getenv("OPENAI_API_KEY") != ""

				data := struct {
					Agents           []Agent
					Todo             []Task
					Doing            []Task
					Done             []Task
					Signals          []Signal
					Progress         int
					TotalTasks       int
					DoneTasks        int
					BusyAgents       int
					TotalAgents      int
					Version          string
					TodayCompleted   int
					WeekCompleted    int
					TotalEvents      int
					TotalCost        float64
					TotalTokens      int
					RegisteredAgents int
					RAGEnabled       bool
				}{agents, todo, doing, done, signals, progress, total, len(done), busyCount, len(agents), FLIP_VERSION, todayCompleted, weekCompleted, totalEvents, totalCost, totalTokens, registeredAgents, ragEnabled}

				// --- Template with 5s refresh and task descriptions ---
				tmpl := `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Hive Mind Dashboard</title>
    <script src="https://unpkg.com/htmx.org@1.9.10"></script>
    <style>
        :root { --bg: #0f172a; --card: #1e293b; --text: #f1f5f9; --green: #4ade80; --red: #f87171; --yellow: #facc15; --blue: #60a5fa; --gray: #94a3b8; --border: #334155; }
        body { background: var(--bg); color: var(--text); font-family: system-ui, -apple-system, sans-serif; padding: 2rem; max-width: 1600px; margin: 0 auto; line-height: 1.5; }

        h1 { margin-bottom: 0.5rem; font-weight: 800; letter-spacing: -0.05em; display:flex; justify-content:space-between; align-items:baseline; color: #fff; }
        h2 { font-size: 0.9rem; text-transform: uppercase; letter-spacing: 0.1em; color: var(--gray); margin-bottom: 1rem; border-bottom: 1px solid var(--border); padding-bottom: 0.5rem; font-weight: 600; }

        /* Agent Grid */
        .agent-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(300px, 1fr)); gap: 1.5rem; margin-bottom: 3rem; }

        .agent-card { background: var(--card); border-radius: 0.75rem; border-left: 4px solid var(--gray); overflow:hidden; box-shadow: 0 4px 6px -1px rgb(0 0 0 / 0.3); transition: transform 0.2s; }
        .agent-card.green { border-color: var(--green); }
        .agent-card.red { border-color: var(--red); }
        .agent-card:hover { transform: translateY(-2px); }

        .ac-header { padding: 1rem; background: rgba(255,255,255,0.03); border-bottom: 1px solid var(--border); display:flex; justify-content:space-between; align-items:center; }
        .ac-id { font-size: 1.25rem; font-weight: 700; text-transform: capitalize; color: #fff; }
        .ac-status { text-transform:uppercase; font-size:0.75rem; font-weight:bold; }
        .ac-hb { font-size: 0.7rem; color: var(--gray); margin-top:2px; text-align:right;}

        .ac-body { padding: 1rem; }
        .active-task { background: rgba(74, 222, 128, 0.05); border: 1px solid rgba(74, 222, 128, 0.15); padding: 0.75rem; border-radius: 0.5rem; }
        .task-desc { font-size: 0.8rem; color: var(--gray); margin-top: 0.5rem; font-style: italic; line-height: 1.4; }

        /* Plan Board */
        .plan-board { display: grid; grid-template-columns: 1fr 1fr 1fr; gap: 1.5rem; }
        .col { background: rgba(30, 41, 59, 0.5); border-radius: 1rem; padding: 1rem; border: 1px solid var(--border); max-height: 600px; overflow-y: auto; }
        .col.todo { border-top: 4px solid var(--gray); }
        .col.doing { border-top: 4px solid var(--blue); background: rgba(59, 130, 246, 0.05); }
        .col.done { border-top: 4px solid var(--green); }

        .task-card { background: var(--card); padding: 1rem; border-radius: 0.5rem; margin-bottom: 0.75rem; border: 1px solid var(--border); box-shadow: 0 2px 4px rgba(0,0,0,0.2); }

        .tc-header { display:flex; justify-content:space-between; margin-bottom:0.5rem; font-size:0.75rem; color:var(--gray); font-family:monospace; }
        .tc-title { font-weight:600; font-size:0.95rem; margin-bottom:0.25rem; color:#fff; }
        .tc-desc { font-size:0.8rem; color:var(--gray); margin-bottom:0.5rem; line-height:1.3; }
        .tc-assignee { display:inline-block; background:rgba(255,255,255,0.1); padding:2px 8px; border-radius:12px; font-size:0.7rem; font-weight:bold; }

        /* Version badge */
        .version { font-size: 0.7rem; background: var(--card); padding: 2px 8px; border-radius: 4px; font-family: monospace; }
    </style>
</head>
<body>
    <h1>Hive Mind <span class="version">FLIP v{{.Version}}</span> <span style="font-size:0.8rem; font-weight:normal; opacity:0.8; font-family:monospace" id="clock"></span></h1>

    <div id="board" hx-get="/" hx-trigger="every 2s" hx-select="#board" hx-swap="outerHTML">
        <script>document.getElementById("clock").innerText = new Date().toLocaleTimeString();</script>

        <!-- PROGRESS BAR -->
        <div style="margin-bottom:2rem; background:var(--card); border-radius:1rem; padding:1.5rem; border:1px solid var(--border);">
            <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:1rem;">
                <div>
                    <div style="font-size:2rem; font-weight:800; color:#fff;">{{.Progress}}%</div>
                    <div style="font-size:0.8rem; color:var(--gray);">Project Complete</div>
                </div>
                <div style="display:flex; gap:2rem; text-align:center;">
                    <div>
                        <div style="font-size:1.5rem; font-weight:700; color:var(--green);">{{.DoneTasks}}</div>
                        <div style="font-size:0.7rem; color:var(--gray); text-transform:uppercase;">Done</div>
                    </div>
                    <div>
                        <div style="font-size:1.5rem; font-weight:700; color:var(--blue);">{{len .Doing}}</div>
                        <div style="font-size:0.7rem; color:var(--gray); text-transform:uppercase;">Active</div>
                    </div>
                    <div>
                        <div style="font-size:1.5rem; font-weight:700; color:var(--gray);">{{len .Todo}}</div>
                        <div style="font-size:0.7rem; color:var(--gray); text-transform:uppercase;">Queued</div>
                    </div>
                    <div>
                        <div style="font-size:1.5rem; font-weight:700; color:var(--green);">{{.BusyAgents}}/{{.TotalAgents}}</div>
                        <div style="font-size:0.7rem; color:var(--gray); text-transform:uppercase;">Agents Busy</div>
                    </div>
                </div>
            </div>
            <div style="background:var(--border); border-radius:0.5rem; height:12px; overflow:hidden;">
                <div style="background:linear-gradient(90deg, var(--green), var(--blue)); height:100%; width:{{.Progress}}%; transition:width 0.5s;"></div>
            </div>
        </div>

        <!-- METRICS PANEL -->
        <div style="display:grid; grid-template-columns: repeat(4, 1fr); gap:1rem; margin-bottom:2rem;">
            <div style="background:var(--card); border-radius:0.75rem; padding:1rem; border:1px solid var(--border); text-align:center;">
                <div style="font-size:1.5rem; font-weight:700; color:var(--green);">{{.TodayCompleted}}</div>
                <div style="font-size:0.7rem; color:var(--gray); text-transform:uppercase;">Completed Today</div>
            </div>
            <div style="background:var(--card); border-radius:0.75rem; padding:1rem; border:1px solid var(--border); text-align:center;">
                <div style="font-size:1.5rem; font-weight:700; color:var(--blue);">{{.WeekCompleted}}</div>
                <div style="font-size:0.7rem; color:var(--gray); text-transform:uppercase;">This Week</div>
            </div>
            <div style="background:var(--card); border-radius:0.75rem; padding:1rem; border:1px solid var(--border); text-align:center;">
                <div style="font-size:1.5rem; font-weight:700; color:var(--yellow);">{{len .Signals}}</div>
                <div style="font-size:0.7rem; color:var(--gray); text-transform:uppercase;">Unread Signals</div>
            </div>
            <div style="background:var(--card); border-radius:0.75rem; padding:1rem; border:1px solid var(--border); text-align:center;">
                <div style="font-size:1.5rem; font-weight:700; color:var(--gray);">{{.TotalEvents}}</div>
                <div style="font-size:0.7rem; color:var(--gray); text-transform:uppercase;">Events Logged</div>
            </div>
        </div>

        <!-- COST & FEATURES PANEL (NEW) -->
        <div style="display:grid; grid-template-columns: repeat(3, 1fr); gap:1rem; margin-bottom:2rem;">
            <div style="background:linear-gradient(135deg, rgba(74, 222, 128, 0.1), rgba(96, 165, 250, 0.1)); border-radius:0.75rem; padding:1rem; border:1px solid var(--green); text-align:center;">
                <div style="font-size:1.25rem; font-weight:700; color:var(--green);">${{printf "%.4f" .TotalCost}}</div>
                <div style="font-size:0.7rem; color:var(--gray); text-transform:uppercase;">Total Cost (USD)</div>
                <div style="font-size:0.65rem; color:var(--gray); margin-top:0.25rem;">{{.TotalTokens}} tokens</div>
            </div>
            <div style="background:linear-gradient(135deg, rgba(96, 165, 250, 0.1), rgba(139, 92, 246, 0.1)); border-radius:0.75rem; padding:1rem; border:1px solid var(--blue); text-align:center;">
                <div style="font-size:1.25rem; font-weight:700; color:var(--blue);">{{.RegisteredAgents}}</div>
                <div style="font-size:0.7rem; color:var(--gray); text-transform:uppercase;">Registered Agents</div>
                <div style="font-size:0.65rem; color:var(--gray); margin-top:0.25rem;">Capability Registry</div>
            </div>
            <div style="background:linear-gradient(135deg, rgba(139, 92, 246, 0.1), rgba(236, 72, 153, 0.1)); border-radius:0.75rem; padding:1rem; border:1px solid #8b5cf6; text-align:center;">
                <div style="font-size:1.25rem; font-weight:700; color:#8b5cf6;">{{if .RAGEnabled}}ONLINE{{else}}OFFLINE{{end}}</div>
                <div style="font-size:0.7rem; color:var(--gray); text-transform:uppercase;">RAG System</div>
                <div style="font-size:0.65rem; color:var(--gray); margin-top:0.25rem;">Vector Search Ready</div>
            </div>
        </div>

        <!-- TEAM STATUS -->
        <h2>Team Status</h2>
        <div class="agent-grid">
            {{range .Agents}}
            <div class="agent-card {{.Color}}">
                <div class="ac-header">
                    <div>
                        <div class="ac-id">{{.ID}}</div>
                        <div class="ac-status" style="color:var(--{{.Color}})">{{.Status}}</div>
                    </div>
                    <div class="ac-hb">HB: {{.Since}} ago</div>
                </div>
                <div class="ac-body">
                    {{if ne .TaskID "-"}}
                    <div class="active-task">
                        <div style="color:var(--yellow); font-family:monospace; font-size:0.8rem; margin-bottom:0.25rem;">{{.TaskID}}</div>
                        <div style="font-weight:600; font-size:0.9rem; line-height:1.3">{{if .TaskTitle}}{{.TaskTitle}}{{else}}(No title){{end}}</div>
                        {{if .TaskDesc}}<div class="task-desc">{{.TaskDesc}}</div>{{end}}
                    </div>
                    {{else}}
                    <div style="font-style:italic; color:var(--gray); font-size:0.9rem;">Idle / Available</div>
                    {{end}}
                </div>
            </div>
            {{end}}
        </div>

        <!-- KANBAN BOARD -->
        <h2>Project Plan</h2>
        <div class="plan-board">
            <!-- TODO -->
            <div class="col todo">
                <h3 style="font-size:0.8rem; color:var(--gray); text-transform:uppercase; margin-bottom:1rem; font-weight:bold;">Queue ({{len .Todo}})</h3>
                {{range .Todo}}
                <div class="task-card">
                    <div class="tc-header">
                        <span style="color:var(--yellow)">{{.ID}}</span>
                        <span>P{{.Priority}}</span>
                    </div>
                    <div class="tc-title">{{.Title}}</div>
                    {{if .Description}}<div class="tc-desc">{{.Description}}</div>{{end}}
                    {{if ne .Assignee "-"}}<span class="tc-assignee">{{.Assignee}}</span>{{end}}
                </div>
                {{end}}
            </div>

            <!-- DOING -->
            <div class="col doing">
                 <h3 style="font-size:0.8rem; color:var(--blue); text-transform:uppercase; margin-bottom:1rem; font-weight:bold;">In Progress ({{len .Doing}})</h3>
                {{range .Doing}}
                <div class="task-card" style="border-left: 3px solid var(--blue)">
                    <div class="tc-header">
                        <span style="color:var(--yellow)">{{.ID}}</span>
                        <span style="color:var(--green)">{{if gt .Progress 0}}{{.Progress}}%{{else}}ACTIVE{{end}}</span>
                    </div>
                    <div class="tc-title">{{.Title}}</div>
                    {{if .Description}}<div class="tc-desc">{{.Description}}</div>{{end}}
                    {{if gt .Progress 0}}
                    <div style="background:var(--border); border-radius:4px; height:6px; margin:8px 0; overflow:hidden;">
                        <div style="background:linear-gradient(90deg, var(--blue), var(--green)); height:100%; width:{{.Progress}}%; transition:width 0.3s;"></div>
                    </div>
                    {{end}}
                    <span class="tc-assignee" style="background:var(--blue); color:#000">{{.Assignee}}</span>
                </div>
                {{end}}
            </div>

            <!-- DONE -->
            <div class="col done">
                 <h3 style="font-size:0.8rem; color:var(--green); text-transform:uppercase; margin-bottom:1rem; font-weight:bold;">Completed ({{len .Done}})</h3>
                {{range .Done}}
                <div class="task-card" style="opacity:0.75">
                    <div class="tc-header">
                        <span style="text-decoration:line-through">{{.ID}}</span>
                        <span>DONE</span>
                    </div>
                    <div class="tc-title" style="text-decoration:line-through; color:var(--gray)">{{.Title}}</div>
                    <span class="tc-assignee">{{.Assignee}}</span>
                </div>
                {{end}}
            </div>
        </div>

        <!-- MESSAGE FEED -->
        {{if .Signals}}
        <h2 style="margin-top:2rem;">Unread Messages ({{len .Signals}})</h2>
        <div style="background:var(--card); border-radius:1rem; padding:1rem; border:1px solid var(--border); max-height:300px; overflow-y:auto;">
            {{range .Signals}}
            <div style="padding:0.75rem; border-bottom:1px solid var(--border); display:flex; gap:1rem; align-items:flex-start;">
                <div style="flex-shrink:0;">
                    <span style="background:var(--yellow); color:#000; padding:2px 8px; border-radius:4px; font-size:0.7rem; font-weight:bold; text-transform:uppercase;">{{.From}}</span>
                    <span style="color:var(--gray); font-size:0.7rem;"> → </span>
                    <span style="background:var(--blue); color:#000; padding:2px 8px; border-radius:4px; font-size:0.7rem; font-weight:bold; text-transform:uppercase;">{{.To}}</span>
                </div>
                <div style="flex:1; font-size:0.85rem; color:var(--text); line-height:1.4;">{{.Msg}}</div>
                <div style="font-size:0.7rem; color:var(--gray); white-space:nowrap;">{{.Time}} ago</div>
            </div>
            {{end}}
        </div>
        {{end}}

    </div>
</body>
</html>
`
				t, _ := template.New("dash").Parse(tmpl)
				t.Execute(w, data)
			})

			fmt.Printf("Hive Dashboard running at http://localhost:%s\n", port)
			fmt.Println("Press Ctrl+C to stop.")
			log.Fatal(http.ListenAndServe(":"+port, nil))
		},
	}
	serveCmd.Flags().StringVarP(&port, "port", "p", "8090", "Port to serve on")

	// ===================
	// SUMMARY Command (NEW)
	// ===================
	var summaryCmd = &cobra.Command{
		Use:   "summary",
		Short: "Show project summary statistics",
		Run: func(cmd *cobra.Command, args []string) {
			db := initDB()
			defer db.Close()

			fmt.Println("📊 PROJECT SUMMARY")
			fmt.Println("==================")

			// Task counts by status
			var todo, inProgress, done int
			db.QueryRow("SELECT COUNT(*) FROM tasks WHERE status = 'todo'").Scan(&todo)
			db.QueryRow("SELECT COUNT(*) FROM tasks WHERE status = 'in_progress'").Scan(&inProgress)
			db.QueryRow("SELECT COUNT(*) FROM tasks WHERE status = 'done'").Scan(&done)
			total := todo + inProgress + done

			fmt.Println("\n📋 Tasks:")
			fmt.Printf("   ⬜ Todo:        %d\n", todo)
			fmt.Printf("   🔄 In Progress: %d\n", inProgress)
			fmt.Printf("   ✅ Done:        %d\n", done)
			fmt.Printf("   ─────────────────\n")
			fmt.Printf("   Total:          %d\n", total)

			if total > 0 {
				pct := float64(done) / float64(total) * 100
				fmt.Printf("   Progress:       %.0f%%\n", pct)
			}

			// Agent counts by status
			var idle, busy, blocked int
			db.QueryRow("SELECT COUNT(*) FROM agents WHERE status = 'idle'").Scan(&idle)
			db.QueryRow("SELECT COUNT(*) FROM agents WHERE status = 'busy'").Scan(&busy)
			db.QueryRow("SELECT COUNT(*) FROM agents WHERE status = 'blocked'").Scan(&blocked)

			fmt.Println("\n🤖 Agents:")
			fmt.Printf("   💤 Idle:    %d\n", idle)
			fmt.Printf("   🔥 Busy:    %d\n", busy)
			fmt.Printf("   🚫 Blocked: %d\n", blocked)

			// Unread signals
			var unread int
			db.QueryRow("SELECT COUNT(*) FROM signals WHERE read = 0").Scan(&unread)
			if unread > 0 {
				fmt.Printf("\n📬 Unread Messages: %d\n", unread)
			}
		},
	}

	// ===================
	// AUTO-ASSIGN Command
	// ===================
	var dryRun bool
	var autoAssignCmd = &cobra.Command{
		Use:   "auto-assign",
		Short: "Auto-assign queued tasks to idle agents",
		Long:  "Finds idle agents and assigns them tasks from the queue based on priority",
		Run: func(cmd *cobra.Command, args []string) {
			db := initDB()
			defer db.Close()

			fmt.Println("🤖 AUTO-ASSIGNMENT")
			fmt.Println("==================")

			// Get idle agents
			idleRows, err := db.Query(`SELECT id FROM agents WHERE status = 'idle' ORDER BY hb DESC`)
			if err != nil {
				log.Fatal(err)
			}

			var idleAgents []string
			for idleRows.Next() {
				var id string
				idleRows.Scan(&id)
				idleAgents = append(idleAgents, id)
			}
			idleRows.Close()

			if len(idleAgents) == 0 {
				fmt.Println("No idle agents available for assignment.")
				return
			}

			fmt.Printf("Found %d idle agent(s): %v\n\n", len(idleAgents), idleAgents)

			// Get unassigned tasks ordered by priority
			taskRows, err := db.Query(`
				SELECT id, title, priority FROM tasks
				WHERE (assignee IS NULL OR assignee = '') AND status = 'todo'
				ORDER BY priority DESC, created_at ASC`)
			if err != nil {
				log.Fatal(err)
			}

			type QueuedTask struct {
				id, title string
				priority  int
			}
			var queuedTasks []QueuedTask
			for taskRows.Next() {
				var t QueuedTask
				taskRows.Scan(&t.id, &t.title, &t.priority)
				queuedTasks = append(queuedTasks, t)
			}
			taskRows.Close()

			if len(queuedTasks) == 0 {
				fmt.Println("No unassigned tasks in queue.")
				return
			}

			fmt.Printf("Found %d unassigned task(s).\n\n", len(queuedTasks))

			// Assign tasks to agents
			assigned := 0
			for i := 0; i < len(idleAgents) && i < len(queuedTasks); i++ {
				agent := idleAgents[i]
				task := queuedTasks[i]

				if dryRun {
					fmt.Printf("Would assign: %s (%s) → %s\n", task.id, task.title, agent)
				} else {
					// Update task assignee
					_, err := db.Exec("UPDATE tasks SET assignee = ? WHERE id = ?", agent, task.id)
					if err != nil {
						fmt.Printf("Error assigning %s: %v\n", task.id, err)
						continue
					}

					// Update agent status and task
					db.Exec("UPDATE agents SET status = 'busy', task = ? WHERE id = ?", task.id, agent)

					// Send signal to agent
					sigID := uuid.New().String()[:8]
					db.Exec(`INSERT INTO signals (id, to_agent, from_agent, msg) VALUES (?, ?, 'auto-assign', ?)`,
						sigID, agent, fmt.Sprintf("Assigned task %s: %s", task.id, task.title))

					fmt.Printf("✓ Assigned: %s (%s) → %s\n", task.id, task.title, agent)
					assigned++
				}
			}

			if dryRun {
				fmt.Printf("\n[DRY RUN] Would have assigned %d task(s).\n", min(len(idleAgents), len(queuedTasks)))
			} else {
				fmt.Printf("\n✓ Assigned %d task(s) to idle agents.\n", assigned)
			}
		},
	}
	autoAssignCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview assignments without making changes")

	// ===================
	// GIT Integration Commands
	// ===================
	var gitCmd = &cobra.Command{
		Use:   "git",
		Short: "Git integration commands",
	}

	// git branch - create branch for task
	var gitBranchCmd = &cobra.Command{
		Use:   "branch [task-id]",
		Short: "Create a git branch for a task",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			db := initDB()
			defer db.Close()

			taskID := args[0]

			// Get task info
			var title string
			err := db.QueryRow("SELECT title FROM tasks WHERE id = ?", taskID).Scan(&title)
			if err != nil {
				fmt.Printf("Task %s not found\n", taskID)
				return
			}

			// Create branch name: feature/TASK-001-short-title
			branchName := fmt.Sprintf("feature/%s-%s",
				taskID,
				strings.ToLower(regexp.MustCompile(`[^a-zA-Z0-9]+`).ReplaceAllString(title, "-")))

			// Limit length
			if len(branchName) > 50 {
				branchName = branchName[:50]
			}

			fmt.Printf("Creating branch: %s\n", branchName)
			fmt.Printf("Run: git checkout -b %s\n", branchName)
		},
	}

	// git link - link commit to task
	var gitLinkCmd = &cobra.Command{
		Use:   "link [task-id] [commit-hash]",
		Short: "Link a commit to a task",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			db := initDB()
			defer db.Close()

			taskID := args[0]
			commitHash := args[1]

			// Check task exists
			var title string
			err := db.QueryRow("SELECT title FROM tasks WHERE id = ?", taskID).Scan(&title)
			if err != nil {
				fmt.Printf("Task %s not found\n", taskID)
				return
			}

			// Add to task history as a comment
			db.Exec(`INSERT INTO task_history (task_id, old_status, new_status, changed_by, created_at)
				VALUES (?, 'commit', ?, 'git', CURRENT_TIMESTAMP)`, taskID, commitHash)

			fmt.Printf("✓ Linked commit %s to task %s (%s)\n", commitHash[:7], taskID, title)
		},
	}

	// git complete - mark task as done on merge
	var gitCompleteCmd = &cobra.Command{
		Use:   "complete [task-id]",
		Short: "Mark task as complete (for merge hooks)",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			db := initDB()
			defer db.Close()

			taskID := args[0]

			// Check task exists
			var title, assignee string
			err := db.QueryRow("SELECT title, COALESCE(assignee, '') FROM tasks WHERE id = ?", taskID).Scan(&title, &assignee)
			if err != nil {
				fmt.Printf("Task %s not found\n", taskID)
				return
			}

			// Mark task as done
			db.Exec("UPDATE tasks SET status = 'done', updated_at = CURRENT_TIMESTAMP WHERE id = ?", taskID)

			// Update agent if assigned
			if assignee != "" {
				db.Exec("UPDATE agents SET status = 'idle', task = NULL WHERE id = ?", assignee)
			}

			// Log to history
			db.Exec(`INSERT INTO task_history (task_id, old_status, new_status, changed_by, created_at)
				VALUES (?, 'in_progress', 'done', 'git-merge', CURRENT_TIMESTAMP)`, taskID)

			fmt.Printf("✓ Task %s (%s) completed via git merge\n", taskID, title)
		},
	}

	// git hook - show example git hook script
	var gitHookCmd = &cobra.Command{
		Use:   "hook",
		Short: "Show git hook integration script",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(`# Add to .git/hooks/commit-msg:
#!/bin/bash
COMMIT_MSG=$(cat $1)
TASK_ID=$(echo "$COMMIT_MSG" | grep -oE 'TASK-[0-9]+|FLIP-[0-9]+' | head -1)
if [ -n "$TASK_ID" ]; then
    COMMIT_HASH=$(git rev-parse HEAD 2>/dev/null || echo "pending")
    ./flip git link "$TASK_ID" "$COMMIT_HASH" 2>/dev/null
fi

# Add to .git/hooks/post-merge:
#!/bin/bash
# Find all task IDs in merged commits
for TASK_ID in $(git log -1 --oneline | grep -oE 'TASK-[0-9]+|FLIP-[0-9]+'); do
    ./flip git complete "$TASK_ID" 2>/dev/null
done`)
		},
	}

	// git pr - generate PR description
	var gitPRCmd = &cobra.Command{
		Use:   "pr [task-id]",
		Short: "Generate PR description for a task",
		Long: `Generate a pull request description based on task info.

Outputs markdown that can be used with 'gh pr create'.

Examples:
  flip git pr TASK-071                    # Print PR description
  flip git pr TASK-071 | gh pr create -F- # Create PR directly`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			db := initDB()
			defer db.Close()

			taskID := args[0]

			// Get task info
			var title, description, status, assignee string
			var progress int
			err := db.QueryRow(`
				SELECT title, COALESCE(description, ''), status, COALESCE(assignee, ''), COALESCE(progress, 0)
				FROM tasks WHERE id = ?`, taskID).Scan(&title, &description, &status, &assignee, &progress)
			if err != nil {
				fmt.Printf("Task %s not found\n", taskID)
				return
			}

			// Get linked commits from history
			var commits []string
			rows, err := db.Query(`
				SELECT new_status FROM task_history
				WHERE task_id = ? AND old_status = 'commit'
				ORDER BY created_at`, taskID)
			if err != nil {
				log.Printf("Warning: Failed to query commits: %v", err)
			}
			if rows != nil {
				defer rows.Close()
				for rows.Next() {
					var commit string
					rows.Scan(&commit)
					if len(commit) > 7 {
						commits = append(commits, commit[:7])
					}
				}
			}

			// Get dependencies
			deps := getTaskDependencies(db, taskID)

			// Generate PR description
			fmt.Printf("## %s\n\n", title)
			fmt.Printf("**Task ID:** %s\n", taskID)
			fmt.Printf("**Status:** %s\n", status)
			if assignee != "" {
				fmt.Printf("**Assignee:** %s\n", assignee)
			}
			if progress > 0 {
				fmt.Printf("**Progress:** %d%%\n", progress)
			}
			fmt.Println()

			if description != "" {
				fmt.Println("### Description")
				fmt.Println(description)
				fmt.Println()
			}

			if len(commits) > 0 {
				fmt.Println("### Commits")
				for _, c := range commits {
					fmt.Printf("- %s\n", c)
				}
				fmt.Println()
			}

			if len(deps) > 0 {
				fmt.Println("### Dependencies")
				for _, d := range deps {
					var depTitle, depStatus string
					db.QueryRow("SELECT title, status FROM tasks WHERE id = ?", d).Scan(&depTitle, &depStatus)
					icon := "⬜"
					if depStatus == "done" {
						icon = "✅"
					}
					fmt.Printf("- %s %s: %s\n", icon, d, depTitle)
				}
				fmt.Println()
			}

			fmt.Println("### Checklist")
			fmt.Println("- [ ] Code reviewed")
			fmt.Println("- [ ] Tests pass")
			fmt.Println("- [ ] Documentation updated")
			fmt.Println()
			fmt.Printf("---\n*Generated by FLIP CLI for %s*\n", taskID)
		},
	}

	gitCmd.AddCommand(gitBranchCmd, gitLinkCmd, gitCompleteCmd, gitHookCmd, gitPRCmd)

	// ===================
	// WORKLOAD Command (NEW)
	// ===================
	var workloadCmd = &cobra.Command{
		Use:   "workload",
		Short: "Show task distribution per agent",
		Run: func(cmd *cobra.Command, args []string) {
			db := initDB()
			defer db.Close()

			fmt.Println("📊 AGENT WORKLOAD")
			fmt.Println("=================")
			fmt.Println("AGENT      | ACTIVE | ASSIGNED | COMPLETED")
			fmt.Println("-----------+--------+----------+----------")

			rows, err := db.Query("SELECT id FROM agents ORDER BY id")
			if err != nil {
				log.Fatal(err)
			}
			defer rows.Close()

			for rows.Next() {
				var agentID string
				rows.Scan(&agentID)

				var active, assigned, completed int
				db.QueryRow(`SELECT COUNT(*) FROM tasks WHERE assignee = ? AND status = 'in_progress'`, agentID).Scan(&active)
				db.QueryRow(`SELECT COUNT(*) FROM tasks WHERE assignee = ? AND status != 'done'`, agentID).Scan(&assigned)
				db.QueryRow(`SELECT COUNT(*) FROM tasks WHERE assignee = ? AND status = 'done'`, agentID).Scan(&completed)

				fmt.Printf("%-10s | %-6d | %-8d | %d\n", agentID, active, assigned, completed)
			}

			// Unassigned tasks
			var unassigned int
			db.QueryRow(`SELECT COUNT(*) FROM tasks WHERE (assignee IS NULL OR assignee = '') AND status != 'done'`).Scan(&unassigned)
			fmt.Printf("\n⚠️  Unassigned tasks: %d\n", unassigned)
		},
	}

	// ===================
	// HEALTH Command (Stale Agent Detection)
	// ===================
	var healthCmd = &cobra.Command{
		Use:   "health",
		Short: "Check swarm health and detect stale agents",
		Long: `Performs health checks on the FLIP swarm:
- Detects stale agents (no heartbeat in 30+ minutes)
- Identifies blocked tasks
- Finds orphaned tasks (assigned to non-existent agents)
- Suggests reassignments`,
		Run: func(cmd *cobra.Command, args []string) {
			db := initDB()
			defer db.Close()

			fmt.Println("🏥 FLIP HEALTH CHECK")
			fmt.Println("====================")

			issues := 0

			// 1. Check for stale agents
			fmt.Println("\n📡 Agent Heartbeats:")
			rows, err := db.Query(`
				SELECT id, status, task, hb FROM agents ORDER BY hb DESC`)
			if err != nil {
				fmt.Printf("Error querying agents: %v\n", err)
				return
			}
			defer rows.Close()

			for rows.Next() {
				var id, status string
				var task sql.NullString
				var hb time.Time
				rows.Scan(&id, &status, &task, &hb)

				since := time.Since(hb)
				icon := "✅"
				warning := ""

				if since > 60*time.Minute {
					icon = "🔴"
					warning = " - STALE (may need restart)"
					issues++
				} else if since > 30*time.Minute {
					icon = "🟡"
					warning = " - getting stale"
				}

				taskStr := "-"
				if task.Valid && task.String != "" {
					taskStr = task.String
				}

				fmt.Printf("  %s %-10s | %-6s | %-10s | %s%s\n",
					icon, id, status, taskStr, formatDuration(since), warning)
			}

			// 2. Check for blocked tasks
			fmt.Println("\n🚧 Blocked Tasks:")
			blockedRows, err := db.Query(`
				SELECT id, title, assignee FROM tasks
				WHERE status = 'in_progress'
				AND id IN (SELECT task FROM agents WHERE status = 'blocked')`)
			if err != nil {
				fmt.Printf("  ❌ Failed to query blocked tasks: %v\n", err)
			}
			if blockedRows != nil {
				defer blockedRows.Close()
			}

			blockedCount := 0
			for blockedRows != nil && blockedRows.Next() {
				var id, title string
				var assignee sql.NullString
				blockedRows.Scan(&id, &title, &assignee)

				assigneeStr := "unassigned"
				if assignee.Valid {
					assigneeStr = assignee.String
				}

				fmt.Printf("  ⚠️  [%s] %s (assigned: %s)\n", id, title, assigneeStr)
				blockedCount++
				issues++
			}
			if blockedCount == 0 {
				fmt.Println("  ✅ No blocked tasks")
			}

			// 3. Check for orphaned tasks (assigned to stale agents)
			fmt.Println("\n👻 Orphaned Tasks (assigned to stale agents):")
			orphanRows, err := db.Query(`
				SELECT t.id, t.title, t.assignee
				FROM tasks t
				JOIN agents a ON t.assignee = a.id
				WHERE t.status = 'in_progress'
				AND a.hb < datetime('now', '-60 minutes')`)
			if err != nil {
				fmt.Printf("  ❌ Failed to query orphaned tasks: %v\n", err)
			}
			if orphanRows != nil {
				defer orphanRows.Close()
			}

			orphanCount := 0
			for orphanRows != nil && orphanRows.Next() {
				var id, title, assignee string
				orphanRows.Scan(&id, &title, &assignee)

				fmt.Printf("  ⚠️  [%s] %s (stuck with: %s)\n", id, title, assignee)
				fmt.Printf("      → Suggest: ./flip task assign %s --auto\n", id)
				orphanCount++
				issues++
			}
			if orphanCount == 0 {
				fmt.Println("  ✅ No orphaned tasks")
			}

			// 4. Check unread signals
			fmt.Println("\n📬 Pending Signals:")
			var unreadCount int
			db.QueryRow("SELECT COUNT(*) FROM signals WHERE read = 0").Scan(&unreadCount)
			if unreadCount > 0 {
				fmt.Printf("  ⚠️  %d unread messages\n", unreadCount)
			} else {
				fmt.Println("  ✅ All messages read")
			}

			// Summary
			fmt.Println("\n" + strings.Repeat("=", 40))
			if issues == 0 {
				fmt.Println("✅ HEALTHY - No issues detected")
			} else {
				fmt.Printf("⚠️  %d ISSUE(S) DETECTED\n", issues)
				fmt.Println("\nRecommended actions:")
				fmt.Println("  • Restart stale agents or mark them idle")
				fmt.Println("  • Reassign orphaned tasks: ./flip auto-assign")
				fmt.Println("  • Check blocked tasks for dependencies")
			}
		},
	}

	// ===================
	// WATCH Command (Concurrent Monitoring)
	// ===================
	var watchInterval int
	var watchAgent string
	var watchCmd = &cobra.Command{
		Use:   "watch",
		Short: "Monitor agents with concurrent heartbeat checking",
		Long: `Runs background monitoring for the FLIP swarm.

If --agent is specified, also polls for signals and updates heartbeat for that agent.
This is the recommended way for agents to stay connected to FLIP.

Examples:
  flip watch                    # Monitor swarm only
  flip watch --agent claude2    # Monitor + poll signals for claude2
  flip watch -a claude -i 120   # Custom 2-minute interval`,
		Run: func(cmd *cobra.Command, args []string) {
			db := initDB()
			defer db.Close()

			fmt.Println("🔍 FLIP Watch Mode - Concurrent Monitoring")
			fmt.Println("==========================================")
			fmt.Printf("Checking every %d seconds. Press Ctrl+C to stop.\n", watchInterval)
			if watchAgent != "" {
				fmt.Printf("Polling signals for agent: %s\n", watchAgent)
			}
			fmt.Println()

			// Channel to signal shutdown
			done := make(chan bool)

			// Goroutine 1: Heartbeat monitor with auto-recovery
			go func() {
				ticker := time.NewTicker(time.Duration(watchInterval) * time.Second)
				defer ticker.Stop()

				for {
					select {
					case <-ticker.C:
						// Check for stale agents (no heartbeat in 30 minutes)
						rows, err := db.Query(`
							SELECT id, status, task, hb FROM agents
							WHERE status = 'busy' AND hb < datetime('now', '-30 minutes')`)
						if err != nil {
							continue
						}

						type StaleAgent struct {
							id, task string
							since    time.Duration
						}
						var staleAgents []StaleAgent

						for rows.Next() {
							var id, status string
							var task sql.NullString
							var hb time.Time
							rows.Scan(&id, &status, &task, &hb)

							taskID := ""
							if task.Valid {
								taskID = task.String
							}
							staleAgents = append(staleAgents, StaleAgent{
								id:    id,
								task:  taskID,
								since: time.Since(hb).Round(time.Second),
							})
						}
						rows.Close()

						// Auto-recover stale agents
						for _, agent := range staleAgents {
							fmt.Printf("\n⚠️  [%s] RECOVERING STALE AGENT: %s (last seen: %s ago)\n",
								time.Now().Format("15:04:05"), agent.id, agent.since)

							// Mark agent as idle and clear task
							db.Exec("UPDATE agents SET status = 'idle', task = NULL WHERE id = ?", agent.id)
							fmt.Printf("   ✓ Marked %s as idle\n", agent.id)

							// If agent had a task, reassign it to queue
							if agent.task != "" && agent.task != "-" {
								db.Exec("UPDATE tasks SET assignee = NULL, status = 'todo' WHERE id = ? AND status = 'in_progress'", agent.task)
								fmt.Printf("   ✓ Task %s returned to queue\n", agent.task)
							}
						}

						// Show active count
						var busyCount, idleCount int
						db.QueryRow("SELECT COUNT(*) FROM agents WHERE status = 'busy'").Scan(&busyCount)
						db.QueryRow("SELECT COUNT(*) FROM agents WHERE status = 'idle'").Scan(&idleCount)
						fmt.Printf("[%s] Agents: %d busy, %d idle\n", time.Now().Format("15:04:05"), busyCount, idleCount)

					case <-done:
						return
					}
				}
			}()

			// Goroutine 2: Task progress monitor
			go func() {
				ticker := time.NewTicker(time.Duration(watchInterval*2) * time.Second)
				defer ticker.Stop()

				var lastDone int
				for {
					select {
					case <-ticker.C:
						var todo, inProgress, doneCount int
						db.QueryRow("SELECT COUNT(*) FROM tasks WHERE status = 'todo'").Scan(&todo)
						db.QueryRow("SELECT COUNT(*) FROM tasks WHERE status = 'in_progress'").Scan(&inProgress)
						db.QueryRow("SELECT COUNT(*) FROM tasks WHERE status = 'done'").Scan(&doneCount)

						if doneCount > lastDone {
							fmt.Printf("\n🎉 [%s] TASK COMPLETED! Progress: %d/%d (%d%%)\n",
								time.Now().Format("15:04:05"),
								doneCount, todo+inProgress+doneCount,
								(doneCount*100)/(todo+inProgress+doneCount))
							lastDone = doneCount
						}

					case <-done:
						return
					}
				}
			}()

			// Goroutine 3: Signal polling for specific agent (if --agent specified)
			if watchAgent != "" {
				go func() {
					ticker := time.NewTicker(time.Duration(watchInterval) * time.Second)
					defer ticker.Stop()

					failCount := 0
					const maxRetries = 3
					const retryWaitMinutes = 3

					// Poll signals with retry logic
					pollSignals := func() bool {
						// Try to update heartbeat
						_, err := db.Exec("UPDATE agents SET hb = ? WHERE id = ?", time.Now(), watchAgent)
						if err != nil {
							failCount++
							fmt.Printf("\n⚠️  [%s] DB connection failed (attempt %d/%d): %v\n",
								time.Now().Format("15:04:05"), failCount, maxRetries, err)

							if failCount >= maxRetries {
								// Write alert file
								alertFile := writeConnectionAlert(watchAgent, failCount, err.Error())
								fmt.Printf("\n🚨 [%s] CONNECTION ALERT WRITTEN\n", time.Now().Format("15:04:05"))
								fmt.Printf("   Alert file: %s\n", alertFile)
								fmt.Printf("   Falling back to file-based communication.\n")
								fmt.Printf("   Will retry DB in %d minutes.\n\n", retryWaitMinutes)

								// Try file-based fallback
								signals := readSignalsFromLogFile(watchAgent)
								if len(signals) > 0 {
									fmt.Printf("📁 Found %d signal(s) in log file:\n", len(signals))
									for _, s := range signals {
										fmt.Printf("   > %s\n", s)
									}
								}

								// Check for other agent alerts
								alerts := checkAlertFiles()
								if len(alerts) > 1 {
									fmt.Printf("\n⚠️  Other agents also reporting issues:\n")
									for _, a := range alerts {
										if !strings.HasPrefix(a, watchAgent) {
											fmt.Printf("   - %s\n", a)
										}
									}
								}

								return false
							}

							fmt.Printf("   Waiting %d minutes before retry...\n", retryWaitMinutes)
							time.Sleep(time.Duration(retryWaitMinutes) * time.Minute)
							return false
						}

						// Connection successful - clear any previous alert
						if failCount > 0 {
							clearConnectionAlert(watchAgent)
							fmt.Printf("\n✅ [%s] DB connection restored after %d failures\n",
								time.Now().Format("15:04:05"), failCount)
							failCount = 0
						}

						// Check for unread signals
						rows, err := db.Query(`
							SELECT id, from_agent, msg, created_at FROM signals
							WHERE to_agent = ? AND read = 0
							ORDER BY created_at ASC`, watchAgent)
						if err != nil {
							return false
						}

						var messages []struct {
							id, from, msg string
							created       time.Time
						}

						for rows.Next() {
							var id, from, msg string
							var created time.Time
							rows.Scan(&id, &from, &msg, &created)
							messages = append(messages, struct {
								id, from, msg string
								created       time.Time
							}{id, from, msg, created})
						}
						rows.Close() // Close BEFORE updating to avoid SQLite deadlock

						if len(messages) > 0 {
							fmt.Printf("\n📬 [%s] %d NEW MESSAGE(S) for %s:\n",
								time.Now().Format("15:04:05"), len(messages), watchAgent)
							fmt.Println(strings.Repeat("-", 50))

							for _, m := range messages {
								fmt.Printf("  From: %s (%s)\n", m.from, m.created.Format("Jan 02 15:04"))
								fmt.Printf("  > %s\n\n", m.msg)

								// Mark as read
								db.Exec("UPDATE signals SET read = 1 WHERE id = ?", m.id)
							}

							fmt.Println("  Reply with: ./flip signal send <agent> \"message\"")
							fmt.Println(strings.Repeat("-", 50))
						}

						// Check for other agent alert files (even when connected)
						alerts := checkAlertFiles()
						if len(alerts) > 0 {
							fmt.Printf("\n⚠️  [%s] Alert files detected from other agents:\n", time.Now().Format("15:04:05"))
							for _, a := range alerts {
								fmt.Printf("   - %s\n", a)
							}
						}

						return true
					}

					// Poll immediately on start
					pollSignals()

					for {
						select {
						case <-ticker.C:
							pollSignals()
						case <-done:
							return
						}
					}
				}()
			}

			// Wait for interrupt
			<-make(chan struct{})
		},
	}
	watchCmd.Flags().IntVarP(&watchInterval, "interval", "i", 300, "Check interval in seconds (default 5 minutes)")
	watchCmd.Flags().StringVarP(&watchAgent, "agent", "a", "", "Agent ID to poll signals for")

	// ===================
	// ALERTS Command
	// ===================
	var clearAlerts bool
	var alertsCmd = &cobra.Command{
		Use:   "alerts",
		Short: "Check or clear connection alert files",
		Long: `View connection alerts from agents that lost DB connectivity.

When an agent fails to connect 3 times, it writes an alert file.
Other agents can see these alerts and respond accordingly.

Examples:
  flip alerts           # List all alerts
  flip alerts --clear   # Clear all alert files`,
		Run: func(cmd *cobra.Command, args []string) {
			root := findProjectRoot()
			alertDir := filepath.Join(root, "ProjectDocs", "LLMcomms", "alerts")

			if clearAlerts {
				files, _ := os.ReadDir(alertDir)
				count := 0
				for _, f := range files {
					if strings.HasSuffix(f.Name(), "_connection_alert.md") {
						os.Remove(filepath.Join(alertDir, f.Name()))
						fmt.Printf("Cleared: %s\n", f.Name())
						count++
					}
				}
				if count == 0 {
					fmt.Println("No alert files to clear.")
				} else {
					fmt.Printf("\n✅ Cleared %d alert file(s).\n", count)
				}
				return
			}

			alerts := checkAlertFiles()
			if len(alerts) == 0 {
				fmt.Println("✅ No connection alerts.")
				return
			}

			fmt.Printf("🚨 %d CONNECTION ALERT(S):\n", len(alerts))
			fmt.Println(strings.Repeat("=", 40))

			for _, alertFile := range alerts {
				content, err := os.ReadFile(filepath.Join(alertDir, alertFile))
				if err != nil {
					continue
				}
				fmt.Println()
				fmt.Println(string(content))
				fmt.Println(strings.Repeat("-", 40))
			}

			fmt.Println("\nTo clear alerts: ./flip alerts --clear")
		},
	}
	alertsCmd.Flags().BoolVarP(&clearAlerts, "clear", "c", false, "Clear all alert files")

	// ===================
	// EXPORT Command
	// ===================
	var exportCmd = &cobra.Command{
		Use:   "export",
		Short: "Export all data to JSON",
		Long: `Export all FLIP data (agents, tasks, signals) to JSON format.

Examples:
  flip export > backup.json
  flip export | jq .tasks`,
		Run: func(cmd *cobra.Command, args []string) {
			db := initDB()
			defer db.Close()

			type Agent struct {
				ID           string `json:"id"`
				Name         string `json:"name"`
				Status       string `json:"status"`
				Task         string `json:"task"`
				Capabilities string `json:"capabilities"`
			}

			type Task struct {
				ID          string `json:"id"`
				Title       string `json:"title"`
				Description string `json:"description"`
				Status      string `json:"status"`
				Assignee    string `json:"assignee"`
				Priority    int    `json:"priority"`
				Progress    int    `json:"progress"`
				DependsOn   string `json:"depends_on"`
			}

			type Signal struct {
				ID        string `json:"id"`
				ToAgent   string `json:"to_agent"`
				FromAgent string `json:"from_agent"`
				Msg       string `json:"msg"`
				Read      bool   `json:"read"`
				ReplyTo   string `json:"reply_to"`
			}

			type Export struct {
				Version   string   `json:"version"`
				Timestamp string   `json:"timestamp"`
				Agents    []Agent  `json:"agents"`
				Tasks     []Task   `json:"tasks"`
				Signals   []Signal `json:"signals"`
			}

			export := Export{
				Version:   FLIP_VERSION,
				Timestamp: time.Now().Format(time.RFC3339),
			}

			// Export agents
			rows, err := db.Query("SELECT id, COALESCE(name, ''), status, COALESCE(task, ''), COALESCE(capabilities, '') FROM agents")
			if err != nil {
				log.Printf("Warning: Failed to query agents: %v", err)
			}
			if rows != nil {
				defer rows.Close()
				for rows.Next() {
					var a Agent
					rows.Scan(&a.ID, &a.Name, &a.Status, &a.Task, &a.Capabilities)
					export.Agents = append(export.Agents, a)
				}
			}

			// Export tasks
			tRows, err := db.Query("SELECT id, title, COALESCE(description, ''), status, COALESCE(assignee, ''), priority, COALESCE(progress, 0), COALESCE(depends_on, '') FROM tasks")
			if err != nil {
				log.Printf("Warning: Failed to query tasks: %v", err)
			}
			if tRows != nil {
				defer tRows.Close()
				for tRows.Next() {
					var t Task
					tRows.Scan(&t.ID, &t.Title, &t.Description, &t.Status, &t.Assignee, &t.Priority, &t.Progress, &t.DependsOn)
					export.Tasks = append(export.Tasks, t)
				}
			}

			// Export signals
			sRows, err := db.Query("SELECT id, to_agent, from_agent, msg, read, COALESCE(reply_to, '') FROM signals")
			if err != nil {
				log.Printf("Warning: Failed to query signals: %v", err)
			}
			if sRows != nil {
				defer sRows.Close()
				for sRows.Next() {
					var s Signal
					sRows.Scan(&s.ID, &s.ToAgent, &s.FromAgent, &s.Msg, &s.Read, &s.ReplyTo)
					export.Signals = append(export.Signals, s)
				}
			}

			// Output JSON
			output, _ := json.MarshalIndent(export, "", "  ")
			fmt.Println(string(output))
		},
	}

	// ===================
	// IMPORT Command
	// ===================
	var importMerge bool
	var importCmd = &cobra.Command{
		Use:   "import [file]",
		Short: "Import data from JSON",
		Long: `Import FLIP data from a JSON export file.

By default, clears existing data before import.
Use --merge to add to existing data instead.

Examples:
  flip import backup.json
  flip import backup.json --merge`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			db := initDB()
			defer db.Close()

			// Read file
			data, err := os.ReadFile(args[0])
			if err != nil {
				log.Fatalf("Cannot read file: %v", err)
			}

			type Agent struct {
				ID           string `json:"id"`
				Name         string `json:"name"`
				Status       string `json:"status"`
				Task         string `json:"task"`
				Capabilities string `json:"capabilities"`
			}

			type Task struct {
				ID          string `json:"id"`
				Title       string `json:"title"`
				Description string `json:"description"`
				Status      string `json:"status"`
				Assignee    string `json:"assignee"`
				Priority    int    `json:"priority"`
				Progress    int    `json:"progress"`
				DependsOn   string `json:"depends_on"`
			}

			type Signal struct {
				ID        string `json:"id"`
				ToAgent   string `json:"to_agent"`
				FromAgent string `json:"from_agent"`
				Msg       string `json:"msg"`
				Read      bool   `json:"read"`
				ReplyTo   string `json:"reply_to"`
			}

			type Import struct {
				Version   string   `json:"version"`
				Timestamp string   `json:"timestamp"`
				Agents    []Agent  `json:"agents"`
				Tasks     []Task   `json:"tasks"`
				Signals   []Signal `json:"signals"`
			}

			var imp Import
			if err := json.Unmarshal(data, &imp); err != nil {
				log.Fatalf("Invalid JSON: %v", err)
			}

			fmt.Printf("Importing from %s (v%s)\n", args[0], imp.Version)
			fmt.Printf("  Agents: %d\n", len(imp.Agents))
			fmt.Printf("  Tasks: %d\n", len(imp.Tasks))
			fmt.Printf("  Signals: %d\n", len(imp.Signals))

			if !importMerge {
				fmt.Println("\nClearing existing data...")
				db.Exec("DELETE FROM agents")
				db.Exec("DELETE FROM tasks")
				db.Exec("DELETE FROM signals")
			}

			fmt.Println("\nImporting...")

			// Import agents
			for _, a := range imp.Agents {
				db.Exec(`INSERT OR REPLACE INTO agents (id, name, status, task, capabilities, hb)
					VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`,
					a.ID, a.Name, a.Status, a.Task, a.Capabilities)
			}
			fmt.Printf("  ✓ %d agents\n", len(imp.Agents))

			// Import tasks
			for _, t := range imp.Tasks {
				db.Exec(`INSERT OR REPLACE INTO tasks (id, title, description, status, assignee, priority, progress, depends_on)
					VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
					t.ID, t.Title, t.Description, t.Status, t.Assignee, t.Priority, t.Progress, t.DependsOn)
			}
			fmt.Printf("  ✓ %d tasks\n", len(imp.Tasks))

			// Import signals
			for _, s := range imp.Signals {
				db.Exec(`INSERT OR REPLACE INTO signals (id, to_agent, from_agent, msg, read, reply_to)
					VALUES (?, ?, ?, ?, ?, ?)`,
					s.ID, s.ToAgent, s.FromAgent, s.Msg, s.Read, s.ReplyTo)
			}
			fmt.Printf("  ✓ %d signals\n", len(imp.Signals))

			fmt.Println("\n✅ Import complete!")
		},
	}
	importCmd.Flags().BoolVarP(&importMerge, "merge", "m", false, "Merge with existing data instead of replacing")

	// ===================
	// CONFIG Command (for webhooks etc)
	// ===================
	var configCmd = &cobra.Command{
		Use:   "config",
		Short: "Manage FLIP configuration",
		Long: `Manage configuration settings like webhook URLs.

Subcommands:
  set [key] [value]  - Set a config value
  get [key]          - Get a config value
  list               - List all config values`,
	}

	var configSetCmd = &cobra.Command{
		Use:   "set [key] [value]",
		Short: "Set a configuration value",
		Long: `Set a configuration value.

Available keys:
  webhook.url      - Webhook URL for notifications (Slack/Discord)
  webhook.events   - Comma-separated events to notify (task.complete,alert,signal)

Examples:
  flip config set webhook.url https://hooks.slack.com/services/xxx
  flip config set webhook.events task.complete,alert`,
		Args: cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			root := findProjectRoot()
			configFile := filepath.Join(root, "ProjectDocs", "LLMcomms", "flip.config.json")

			// Read existing config
			config := make(map[string]string)
			if data, err := os.ReadFile(configFile); err == nil {
				json.Unmarshal(data, &config)
			}

			// Set value
			config[args[0]] = args[1]

			// Write config
			data, _ := json.MarshalIndent(config, "", "  ")
			os.WriteFile(configFile, data, 0644)

			fmt.Printf("✓ Set %s = %s\n", args[0], args[1])
		},
	}

	var configGetCmd = &cobra.Command{
		Use:   "get [key]",
		Short: "Get a configuration value",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			root := findProjectRoot()
			configFile := filepath.Join(root, "ProjectDocs", "LLMcomms", "flip.config.json")

			config := make(map[string]string)
			if data, err := os.ReadFile(configFile); err == nil {
				json.Unmarshal(data, &config)
			}

			if val, ok := config[args[0]]; ok {
				fmt.Println(val)
			} else {
				fmt.Printf("Key '%s' not set\n", args[0])
			}
		},
	}

	var configListCmd = &cobra.Command{
		Use:   "list",
		Short: "List all configuration values",
		Run: func(cmd *cobra.Command, args []string) {
			root := findProjectRoot()
			configFile := filepath.Join(root, "ProjectDocs", "LLMcomms", "flip.config.json")

			config := make(map[string]string)
			if data, err := os.ReadFile(configFile); err == nil {
				json.Unmarshal(data, &config)
			}

			if len(config) == 0 {
				fmt.Println("No configuration set.")
				fmt.Println("\nAvailable keys:")
				fmt.Println("  webhook.url     - Slack/Discord webhook URL")
				fmt.Println("  webhook.events  - Events to notify (task.complete,alert,signal)")
				return
			}

			fmt.Println("CONFIGURATION:")
			fmt.Println("==============")
			for k, v := range config {
				// Mask webhook URLs for security
				if strings.Contains(k, "webhook") && strings.Contains(k, "url") && len(v) > 20 {
					v = v[:20] + "..."
				}
				fmt.Printf("%s = %s\n", k, v)
			}
		},
	}

	// Webhook test command
	var configTestCmd = &cobra.Command{
		Use:   "test",
		Short: "Test webhook configuration",
		Run: func(cmd *cobra.Command, args []string) {
			root := findProjectRoot()
			configFile := filepath.Join(root, "ProjectDocs", "LLMcomms", "flip.config.json")

			config := make(map[string]string)
			if data, err := os.ReadFile(configFile); err == nil {
				json.Unmarshal(data, &config)
			}

			webhookURL, ok := config["webhook.url"]
			if !ok || webhookURL == "" {
				fmt.Println("No webhook URL configured.")
				fmt.Println("Set one with: flip config set webhook.url <URL>")
				return
			}

			fmt.Printf("Testing webhook: %s...\n", webhookURL[:min(50, len(webhookURL))]+"...")

			// Create test payload (Slack/Discord compatible)
			payload := map[string]string{
				"text": "🧪 FLIP Webhook Test - If you see this, your webhook is working!",
			}
			payloadBytes, _ := json.Marshal(payload)

			resp, err := http.Post(webhookURL, "application/json", strings.NewReader(string(payloadBytes)))
			if err != nil {
				fmt.Printf("❌ Failed: %v\n", err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				fmt.Println("✅ Webhook test successful!")
			} else {
				fmt.Printf("❌ Webhook returned status: %d\n", resp.StatusCode)
			}
		},
	}

	configCmd.AddCommand(configSetCmd, configGetCmd, configListCmd, configTestCmd)

	// ===================
	// STATS Command
	// ===================
	var statsCmd = &cobra.Command{
		Use:   "stats",
		Short: "Show detailed statistics and metrics",
		Long:  "Display comprehensive statistics including task completion rates, agent activity, and historical metrics.",
		Run: func(cmd *cobra.Command, args []string) {
			db := initDB()
			defer db.Close()

			fmt.Println("📊 FLIP STATISTICS")
			fmt.Println("==================")

			// Overall task stats
			var totalTasks, todoTasks, inProgressTasks, doneTasks int
			db.QueryRow("SELECT COUNT(*) FROM tasks").Scan(&totalTasks)
			db.QueryRow("SELECT COUNT(*) FROM tasks WHERE status = 'todo'").Scan(&todoTasks)
			db.QueryRow("SELECT COUNT(*) FROM tasks WHERE status = 'in_progress'").Scan(&inProgressTasks)
			db.QueryRow("SELECT COUNT(*) FROM tasks WHERE status = 'done'").Scan(&doneTasks)

			progress := 0
			if totalTasks > 0 {
				progress = (doneTasks * 100) / totalTasks
			}

			fmt.Println("\n📋 Task Overview:")
			fmt.Printf("   Total Tasks:    %d\n", totalTasks)
			fmt.Printf("   Todo:           %d\n", todoTasks)
			fmt.Printf("   In Progress:    %d\n", inProgressTasks)
			fmt.Printf("   Completed:      %d\n", doneTasks)
			fmt.Printf("   Progress:       %d%%\n", progress)

			// Tasks completed today/this week
			var todayCompleted, weekCompleted int
			db.QueryRow(`SELECT COUNT(*) FROM task_history
				WHERE new_status = 'done' AND DATE(changed_at) = DATE('now')`).Scan(&todayCompleted)
			db.QueryRow(`SELECT COUNT(*) FROM task_history
				WHERE new_status = 'done' AND changed_at >= DATE('now', '-7 days')`).Scan(&weekCompleted)

			fmt.Println("\n📈 Completion Metrics:")
			fmt.Printf("   Completed Today:      %d\n", todayCompleted)
			fmt.Printf("   Completed This Week:  %d\n", weekCompleted)

			// Agent stats
			var totalAgents, busyAgents, idleAgents int
			db.QueryRow("SELECT COUNT(*) FROM agents").Scan(&totalAgents)
			db.QueryRow("SELECT COUNT(*) FROM agents WHERE status = 'busy'").Scan(&busyAgents)
			db.QueryRow("SELECT COUNT(*) FROM agents WHERE status = 'idle'").Scan(&idleAgents)

			fmt.Println("\n🤖 Agent Status:")
			fmt.Printf("   Total Agents:  %d\n", totalAgents)
			fmt.Printf("   Busy:          %d\n", busyAgents)
			fmt.Printf("   Idle:          %d\n", idleAgents)

			// Signals stats
			var totalSignals, unreadSignals int
			db.QueryRow("SELECT COUNT(*) FROM signals").Scan(&totalSignals)
			db.QueryRow("SELECT COUNT(*) FROM signals WHERE read = 0").Scan(&unreadSignals)

			fmt.Println("\n📬 Signals:")
			fmt.Printf("   Total Signals:  %d\n", totalSignals)
			fmt.Printf("   Unread:         %d\n", unreadSignals)

			// Event stats (if available)
			var totalEvents int
			db.QueryRow("SELECT COUNT(*) FROM events").Scan(&totalEvents)
			if totalEvents > 0 {
				fmt.Println("\n📝 Events Logged:")
				fmt.Printf("   Total Events:   %d\n", totalEvents)

				// Recent events by type
				rows, err := db.Query(`SELECT event_type, COUNT(*) as cnt
					FROM events GROUP BY event_type ORDER BY cnt DESC LIMIT 5`)
				if err == nil && rows != nil {
					defer rows.Close()
					fmt.Println("   By Type:")
					for rows.Next() {
						var eventType string
						var count int
						rows.Scan(&eventType, &count)
						fmt.Printf("      %-20s %d\n", eventType, count)
					}
				}
			}

			// Daily stats (if available)
			var statRows int
			db.QueryRow("SELECT COUNT(*) FROM stats").Scan(&statRows)
			if statRows > 0 {
				fmt.Println("\n📅 Daily Stats (Last 7 Days):")
				rows, err := db.Query(`SELECT stat_date, tasks_completed, signals_sent, errors
					FROM stats ORDER BY stat_date DESC LIMIT 7`)
				if err == nil && rows != nil {
					defer rows.Close()
					fmt.Println("   Date       | Completed | Signals | Errors")
					fmt.Println("   -----------+-----------+---------+-------")
					for rows.Next() {
						var date string
						var completed, signals, errors int
						rows.Scan(&date, &completed, &signals, &errors)
						fmt.Printf("   %s |     %5d |   %5d |  %5d\n", date, completed, signals, errors)
					}
				}
			}

			// Recent activity
			fmt.Println("\n🕐 Recent Activity:")
			rows, err := db.Query(`SELECT task_id, old_status, new_status, changed_by, changed_at
				FROM task_history ORDER BY changed_at DESC LIMIT 5`)
			if err == nil && rows != nil {
				defer rows.Close()
				for rows.Next() {
					var taskID, oldStatus, newStatus, changedBy string
					var changedAt time.Time
					rows.Scan(&taskID, &oldStatus, &newStatus, &changedBy, &changedAt)
					fmt.Printf("   [%s] %s → %s (by %s, %s ago)\n",
						taskID, oldStatus, newStatus, changedBy,
						time.Since(changedAt).Round(time.Minute))
				}
			}

			fmt.Println("\n==================")
			fmt.Printf("Log file: %s\n", getLogPath())
		},
	}
	// Add costs subcommand to stats
	statsCmd.AddCommand(GetStatsCostsCommand())

	// ===================
	// LOGS Command
	// ===================
	var logsLines int
	var logsCmd = &cobra.Command{
		Use:   "logs",
		Short: "View the FLIP log file",
		Long:  "Display recent entries from the FLIP log file.",
		Run: func(cmd *cobra.Command, args []string) {
			logPath := getLogPath()

			// Check if file exists
			if _, err := os.Stat(logPath); os.IsNotExist(err) {
				fmt.Printf("Log file not found: %s\n", logPath)
				fmt.Println("Logs will be created when events occur.")
				return
			}

			// Read file
			file, err := os.Open(logPath)
			if err != nil {
				fmt.Printf("Error opening log file: %v\n", err)
				return
			}
			defer file.Close()

			// Read all lines
			var lines []string
			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				lines = append(lines, scanner.Text())
			}

			// Show last N lines
			start := 0
			if len(lines) > logsLines {
				start = len(lines) - logsLines
			}

			fmt.Printf("📜 FLIP Log (last %d entries)\n", logsLines)
			fmt.Printf("   File: %s\n", logPath)
			fmt.Println("==================")
			for i := start; i < len(lines); i++ {
				fmt.Println(lines[i])
			}
		},
	}
	logsCmd.Flags().IntVarP(&logsLines, "lines", "n", 20, "Number of lines to show")

	// ===================
	// VERSION Command
	// ===================
	var versionCmd = &cobra.Command{
		Use:   "version",
		Short: "Show FLIP CLI version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("FLIP CLI v%s\n", FLIP_VERSION)
			fmt.Println("File-based LLM Inter-Process Communication Tool")
			fmt.Println("v5.0: Extended status, signal priorities, watchdog, handoffs")
			fmt.Println("https://github.com/BenGMc/htb-task")
		},
	}

	// ===================
	// WATCHDOG Command (v5.0 - Heartbeat monitoring)
	// ===================
	var watchdogStaleThreshold int
	var watchdogCmd = &cobra.Command{
		Use:   "watchdog",
		Short: "Monitor agent heartbeats and auto-mark stale agents (v5.0)",
		Long: `Heartbeat watchdog system for detecting stale/crashed agents.

The watchdog checks all agent heartbeats and:
- Marks agents as 'offline' if heartbeat exceeds threshold
- Sends CRITICAL signal to coordinator
- Reassigns stale agent's tasks to queue
- Logs event to dashboard

Examples:
  flip watchdog                      # Check with 30min threshold (default)
  flip watchdog --threshold 60       # Check with 60min threshold
  flip watchdog --threshold 15       # More aggressive 15min check`,
		Run: func(cmd *cobra.Command, args []string) {
			db := initDB()
			defer db.Close()

			threshold := time.Duration(watchdogStaleThreshold) * time.Minute
			staleTime := time.Now().Add(-threshold)

			fmt.Printf("🐕 WATCHDOG: Checking for agents stale > %d minutes\n", watchdogStaleThreshold)
			fmt.Println("=========================================")

			// Find stale agents (not already offline)
			rows, err := db.Query(`
				SELECT id, name, status, task, hb
				FROM agents
				WHERE hb < ? AND status != 'offline'`,
				staleTime)
			if err != nil {
				log.Fatal(err)
			}

			type staleAgent struct {
				id     string
				name   string
				status string
				task   sql.NullString
				hb     time.Time
			}
			var staleAgents []staleAgent

			for rows.Next() {
				var a staleAgent
				rows.Scan(&a.id, &a.name, &a.status, &a.task, &a.hb)
				staleAgents = append(staleAgents, a)
			}
			rows.Close()

			if len(staleAgents) == 0 {
				fmt.Println("✅ All agents are responsive")
				return
			}

			fmt.Printf("⚠️  Found %d stale agent(s):\n\n", len(staleAgents))

			for _, a := range staleAgents {
				staleDuration := time.Since(a.hb)
				fmt.Printf("🔴 %s (%s)\n", a.id, a.name)
				fmt.Printf("   Last heartbeat: %s (%s ago)\n", a.hb.Format("Jan 02 15:04"), formatDuration(staleDuration))
				fmt.Printf("   Previous status: %s\n", a.status)

				// Mark as offline
				_, err := db.Exec(`
					UPDATE agents SET
						status = 'offline',
						status_reason = ?,
						status_detail = 'watchdog_detected',
						last_activity = hb
					WHERE id = ?`,
					fmt.Sprintf("Stale heartbeat: %s", formatDuration(staleDuration)),
					a.id)
				if err != nil {
					fmt.Printf("   ❌ Failed to update: %v\n", err)
					continue
				}
				fmt.Printf("   → Marked as OFFLINE\n")

				// Reassign task if any
				if a.task.Valid && a.task.String != "" {
					db.Exec("UPDATE tasks SET status = 'todo', assignee = NULL WHERE id = ?", a.task.String)
					fmt.Printf("   → Task %s returned to queue\n", a.task.String)
				}

				// Send CRITICAL signal to coordinator
				msgID := uuid.New().String()[:8]
				db.Exec(`
					INSERT INTO signals (id, to_agent, from_agent, msg, priority, signal_type, related_task)
					VALUES (?, 'claude', 'watchdog', ?, 'critical', 'watchdog_alert', ?)`,
					msgID,
					fmt.Sprintf("AGENT OFFLINE: %s has been unresponsive for %s. Previous status: %s. Task reassigned to queue.", a.id, formatDuration(staleDuration), a.status),
					a.task.String)
				fmt.Printf("   → CRITICAL signal sent to coordinator\n")

				// Log event
				logEvent(db, "watchdog_offline", a.id, a.task.String,
					fmt.Sprintf("Agent marked offline by watchdog. Stale for %s", formatDuration(staleDuration)))
				fmt.Println()
			}

			fmt.Printf("\n🐕 Watchdog complete. %d agent(s) marked offline.\n", len(staleAgents))
		},
	}
	watchdogCmd.Flags().IntVar(&watchdogStaleThreshold, "threshold", 30, "Minutes before marking agent as offline")

	// ===================
	// HANDOFF Command (v5.0 - Formal task handoffs)
	// ===================
	var handoffCmd = &cobra.Command{
		Use:   "handoff [subcommand]",
		Short: "Manage task handoffs between agents (v5.0)",
		Long: `Formal handoff protocol for transferring task ownership between agents.

Subcommands:
  create [task] --to [agent]   - Initiate a handoff
  ack [handoff_id]             - Accept a handoff
  reject [handoff_id]          - Reject a handoff
  list                         - List pending handoffs
  status [handoff_id]          - Check handoff status`,
	}

	// HANDOFF CREATE
	var handoffTo string
	var handoffContext string
	var handoffAction string
	var handoffFromAgent string
	var handoffCreateCmd = &cobra.Command{
		Use:   "create [task_id]",
		Short: "Initiate a task handoff to another agent",
		Long: `Create a formal handoff request for a task.

Examples:
  flip handoff create TASK-089 --to antigravity --action "Verify fix in browser"
  flip handoff create TASK-089 --to claude2 --context "docs/report.md" --from claude5`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			db := initDB()
			defer db.Close()

			taskID := args[0]

			if handoffTo == "" {
				log.Fatal("--to flag is required")
			}
			if handoffAction == "" {
				log.Fatal("--action flag is required")
			}
			if handoffFromAgent == "" {
				handoffFromAgent = "cli"
			}

			// Validate task exists
			if !taskExists(db, taskID) {
				log.Fatalf("Task '%s' does not exist", taskID)
			}

			// Validate target agent exists
			var targetExists int
			db.QueryRow("SELECT COUNT(*) FROM agents WHERE id = ?", handoffTo).Scan(&targetExists)
			if targetExists == 0 {
				log.Fatalf("Agent '%s' is not registered", handoffTo)
			}

			handoffID := uuid.New().String()[:8]

			_, err := db.Exec(`
				INSERT INTO handoffs (id, task_id, from_agent, to_agent, context_file, action_required, status)
				VALUES (?, ?, ?, ?, ?, ?, 'pending')`,
				handoffID, taskID, handoffFromAgent, handoffTo, handoffContext, handoffAction)
			if err != nil {
				log.Fatal(err)
			}

			// Send HIGH priority signal to target agent
			msgID := uuid.New().String()[:8]
			db.Exec(`
				INSERT INTO signals (id, to_agent, from_agent, msg, priority, signal_type, related_task)
				VALUES (?, ?, ?, ?, 'high', 'handoff', ?)`,
				msgID, handoffTo, handoffFromAgent,
				fmt.Sprintf("HANDOFF REQUEST [%s]: Task %s - %s. Use 'flip handoff ack %s' to accept or 'flip handoff reject %s' to decline.", handoffID, taskID, handoffAction, handoffID, handoffID),
				taskID)

			// Get task title
			var taskTitle string
			db.QueryRow("SELECT title FROM tasks WHERE id = ?", taskID).Scan(&taskTitle)

			fmt.Printf("📤 Handoff created (id: %s)\n", handoffID)
			fmt.Printf("   Task: %s - %s\n", taskID, taskTitle)
			fmt.Printf("   From: %s → To: %s\n", handoffFromAgent, handoffTo)
			fmt.Printf("   Action: %s\n", handoffAction)
			if handoffContext != "" {
				fmt.Printf("   Context: %s\n", handoffContext)
			}
			fmt.Printf("\n   ⚡ HIGH priority signal sent to %s\n", handoffTo)

			logEvent(db, "handoff_created", handoffFromAgent, taskID,
				fmt.Sprintf("Handoff to %s: %s", handoffTo, handoffAction))
		},
	}
	handoffCreateCmd.Flags().StringVar(&handoffTo, "to", "", "Target agent ID (required)")
	handoffCreateCmd.Flags().StringVar(&handoffContext, "context", "", "Path to context/handoff document")
	handoffCreateCmd.Flags().StringVar(&handoffAction, "action", "", "What the recipient needs to do (required)")
	handoffCreateCmd.Flags().StringVar(&handoffFromAgent, "from", "", "Source agent ID")

	// HANDOFF ACK
	var handoffAckCmd = &cobra.Command{
		Use:   "ack [handoff_id]",
		Short: "Accept a handoff",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			db := initDB()
			defer db.Close()

			handoffID := args[0]

			// Get handoff details
			var taskID, fromAgent, toAgent, action string
			err := db.QueryRow(`
				SELECT task_id, from_agent, to_agent, action_required
				FROM handoffs WHERE id = ? AND status = 'pending'`,
				handoffID).Scan(&taskID, &fromAgent, &toAgent, &action)
			if err != nil {
				log.Fatalf("Handoff '%s' not found or already processed", handoffID)
			}

			// Update handoff status
			_, err = db.Exec(`
				UPDATE handoffs SET status = 'acknowledged', acknowledged_at = CURRENT_TIMESTAMP
				WHERE id = ?`, handoffID)
			if err != nil {
				log.Fatal(err)
			}

			// Update task assignee
			db.Exec("UPDATE tasks SET assignee = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", toAgent, taskID)

			// Update agent status
			db.Exec(`UPDATE agents SET status = 'busy', task = ?, hb = ?, last_activity = ? WHERE id = ?`,
				taskID, time.Now(), time.Now(), toAgent)

			// Send confirmation signal to original agent
			msgID := uuid.New().String()[:8]
			db.Exec(`
				INSERT INTO signals (id, to_agent, from_agent, msg, priority, signal_type, related_task)
				VALUES (?, ?, ?, ?, 'normal', 'handoff_ack', ?)`,
				msgID, fromAgent, toAgent,
				fmt.Sprintf("HANDOFF ACCEPTED: %s has accepted task %s", toAgent, taskID),
				taskID)

			fmt.Printf("✅ Handoff %s accepted\n", handoffID)
			fmt.Printf("   Task %s now assigned to %s\n", taskID, toAgent)
			fmt.Printf("   Action: %s\n", action)
			fmt.Printf("   📨 Confirmation sent to %s\n", fromAgent)

			logEvent(db, "handoff_accepted", toAgent, taskID, fmt.Sprintf("Accepted handoff from %s", fromAgent))
		},
	}

	// HANDOFF REJECT
	var handoffRejectReason string
	var handoffRejectCmd = &cobra.Command{
		Use:   "reject [handoff_id]",
		Short: "Reject a handoff",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			db := initDB()
			defer db.Close()

			handoffID := args[0]

			// Get handoff details
			var taskID, fromAgent, toAgent string
			err := db.QueryRow(`
				SELECT task_id, from_agent, to_agent
				FROM handoffs WHERE id = ? AND status = 'pending'`,
				handoffID).Scan(&taskID, &fromAgent, &toAgent)
			if err != nil {
				log.Fatalf("Handoff '%s' not found or already processed", handoffID)
			}

			if handoffRejectReason == "" {
				handoffRejectReason = "No reason provided"
			}

			// Update handoff status
			_, err = db.Exec(`
				UPDATE handoffs SET status = 'rejected', reject_reason = ?
				WHERE id = ?`, handoffRejectReason, handoffID)
			if err != nil {
				log.Fatal(err)
			}

			// Send HIGH priority signal back to original agent
			msgID := uuid.New().String()[:8]
			db.Exec(`
				INSERT INTO signals (id, to_agent, from_agent, msg, priority, signal_type, related_task)
				VALUES (?, ?, ?, ?, 'high', 'handoff_reject', ?)`,
				msgID, fromAgent, toAgent,
				fmt.Sprintf("HANDOFF REJECTED: %s declined task %s. Reason: %s", toAgent, taskID, handoffRejectReason),
				taskID)

			fmt.Printf("❌ Handoff %s rejected\n", handoffID)
			fmt.Printf("   Task %s remains with %s\n", taskID, fromAgent)
			fmt.Printf("   Reason: %s\n", handoffRejectReason)
			fmt.Printf("   ⚡ HIGH priority signal sent to %s\n", fromAgent)

			logEvent(db, "handoff_rejected", toAgent, taskID,
				fmt.Sprintf("Rejected handoff from %s: %s", fromAgent, handoffRejectReason))
		},
	}
	handoffRejectCmd.Flags().StringVar(&handoffRejectReason, "reason", "", "Reason for rejection")

	// HANDOFF LIST
	var handoffListCmd = &cobra.Command{
		Use:   "list",
		Short: "List pending handoffs",
		Run: func(cmd *cobra.Command, args []string) {
			db := initDB()
			defer db.Close()

			rows, err := db.Query(`
				SELECT h.id, h.task_id, t.title, h.from_agent, h.to_agent, h.action_required, h.status, h.created_at
				FROM handoffs h
				LEFT JOIN tasks t ON h.task_id = t.id
				WHERE h.status = 'pending'
				ORDER BY h.created_at DESC`)
			if err != nil {
				log.Fatal(err)
			}
			defer rows.Close()

			fmt.Println("📋 PENDING HANDOFFS:")
			fmt.Println("====================")

			count := 0
			for rows.Next() {
				var id, taskID, taskTitle, from, to, action, status string
				var createdAt time.Time
				rows.Scan(&id, &taskID, &taskTitle, &from, &to, &action, &status, &createdAt)

				fmt.Printf("\n[%s] %s: %s\n", id, taskID, taskTitle)
				fmt.Printf("   %s → %s\n", from, to)
				fmt.Printf("   Action: %s\n", action)
				fmt.Printf("   Created: %s\n", formatDuration(time.Since(createdAt)))
				count++
			}

			if count == 0 {
				fmt.Println("No pending handoffs.")
			} else {
				fmt.Printf("\n%d pending handoff(s)\n", count)
			}
		},
	}

	// HANDOFF STATUS
	var handoffStatusCmd = &cobra.Command{
		Use:   "status [handoff_id]",
		Short: "Check handoff status",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			db := initDB()
			defer db.Close()

			handoffID := args[0]

			var taskID, from, to, context, action, status, rejectReason sql.NullString
			var createdAt, ackAt, completedAt sql.NullTime
			err := db.QueryRow(`
				SELECT task_id, from_agent, to_agent, context_file, action_required, status,
				       reject_reason, created_at, acknowledged_at, completed_at
				FROM handoffs WHERE id = ?`, handoffID).Scan(
				&taskID, &from, &to, &context, &action, &status, &rejectReason, &createdAt, &ackAt, &completedAt)
			if err != nil {
				log.Fatalf("Handoff '%s' not found", handoffID)
			}

			statusIcon := "⏳"
			switch status.String {
			case "acknowledged":
				statusIcon = "✅"
			case "rejected":
				statusIcon = "❌"
			case "completed":
				statusIcon = "🎉"
			}

			fmt.Printf("%s Handoff %s\n", statusIcon, handoffID)
			fmt.Printf("   Status: %s\n", status.String)
			fmt.Printf("   Task: %s\n", taskID.String)
			fmt.Printf("   From: %s → To: %s\n", from.String, to.String)
			fmt.Printf("   Action: %s\n", action.String)
			if context.Valid && context.String != "" {
				fmt.Printf("   Context: %s\n", context.String)
			}
			if rejectReason.Valid && rejectReason.String != "" {
				fmt.Printf("   Reject reason: %s\n", rejectReason.String)
			}
			fmt.Printf("   Created: %s\n", createdAt.Time.Format("Jan 02 15:04"))
			if ackAt.Valid {
				fmt.Printf("   Acknowledged: %s\n", ackAt.Time.Format("Jan 02 15:04"))
			}
		},
	}

	// Register handoff subcommands
	handoffCmd.AddCommand(handoffCreateCmd, handoffAckCmd, handoffRejectCmd, handoffListCmd, handoffStatusCmd)

	// ===================
	// SPAWN Command (v5.0 - Shell Session Management)
	// ===================
	var spawnCmd = &cobra.Command{
		Use:   "spawn",
		Short: "Manage AI agent shell sessions",
		Long: `Spawn and manage shell sessions for AI agents (Claude, Gemini, etc).

Subcommands:
  run <agent-id> <backend> <prompt>  - Spawn and send in one command (recommended)
  repl <agent-id> <backend>          - Interactive REPL mode
  start <agent-id> <backend>         - Start a session (server mode only)
  send <agent-id> <prompt>           - Send a prompt (server mode only)
  list                               - List active sessions
  stop <agent-id>                    - Stop a session

Examples:
  flip spawn run claude2.1 claude "What is 2+2?"
  flip spawn repl claude2.1 claude
  flip spawn list`,
	}

	// SPAWN RUN - Spawn and send in one command (recommended approach)
	var spawnRunTimeout int
	var spawnRunCmd = &cobra.Command{
		Use:   "run [agent-id] [backend] [prompt]",
		Short: "Spawn agent and send prompt in one command",
		Long: `Spawn a shell session, send a prompt, and get a response all in one command.
This is the recommended way to interact with AI agents.

Backends:
  claude  - Claude CLI (--dangerously-skip-permissions)
  gemini  - Gemini CLI

Example:
  flip spawn run claude2.1 claude "What is 2+2?"
  flip spawn run gemini1 gemini "Explain Go channels"`,
		Args: cobra.MinimumNArgs(3),
		Run: func(cmd *cobra.Command, args []string) {
			agentID := args[0]
			backend := args[1]
			rawPrompt := strings.Join(args[2:], " ")

			// Validate backend
			validBackends := map[string]bool{"claude": true, "gemini": true, "codex": true, "claude-code": true, "antigravity": true}
			if !validBackends[backend] {
				fmt.Printf("❌ Unknown backend '%s'. Valid: claude, gemini, codex, claude-code, antigravity\n", backend)
				return
			}

			// Check if --raw flag is set (skip worker prefix)
			isRaw, _ := cmd.Flags().GetBool("raw")

			// Prepend worker context unless --raw is specified
			var prompt string
			if isRaw {
				prompt = rawPrompt
			} else {
				workerPrefix := fmt.Sprintf(`You are WORKER AGENT "%s" in the FLIP multi-agent system.

IMPORTANT RULES:
- You are a WORKER, not the coordinator
- Complete your assigned task and report results clearly
- Do NOT spawn additional agents without explicit approval
- Do NOT make autonomous decisions beyond your task scope
- If you encounter blockers, report them - don't try to solve everything
- Keep responses focused and concise

YOUR TASK:
`, agentID)
				prompt = workerPrefix + rawPrompt
			}

			fmt.Printf("🚀 Spawning %s WORKER '%s'...\n", backend, agentID)

			// Create session
			ctx := context.Background()
			session, err := NewShellSession(ctx, agentID, backend)
			if err != nil {
				fmt.Printf("❌ Failed to create session: %v\n", err)
				return
			}
			defer session.Close()

			// Register as FLIP agent
			db := initDB()
			defer db.Close()

			_, err = db.Exec(`
				INSERT OR REPLACE INTO agents (id, name, status, hb, session_start, capabilities)
				VALUES (?, ?, 'busy', datetime('now'), datetime('now'), ?)`,
				agentID, agentID, backend)
			if err != nil {
				fmt.Printf("⚠️  Warning: failed to register agent: %v\n", err)
			}

			logEvent(db, "agent_spawned", agentID, "", fmt.Sprintf("backend=%s", backend))

			fmt.Printf("📤 Sending prompt to %s...\n\n", agentID)

			// Send prompt
			timeout := time.Duration(spawnRunTimeout) * time.Second
			response, err := session.SendPrompt(prompt, timeout)

			// Update status
			db.Exec("UPDATE agents SET status = 'idle', hb = datetime('now') WHERE id = ?", agentID)

			if err != nil {
				fmt.Printf("❌ Error: %v\n", err)
				if response != "" {
					fmt.Printf("\nPartial response:\n%s\n", response)
				}
				return
			}

			fmt.Printf("📥 Response from %s:\n", agentID)
			fmt.Println(strings.Repeat("-", 50))
			fmt.Println(response)
			fmt.Println(strings.Repeat("-", 50))

			// Calculate and store cost metrics
			inputTokens := estimateTokens(prompt)
			outputTokens := estimateTokens(response)
			modelUsed := getDefaultModel(backend)
			costUSD := agent.CalculateCost(modelUsed, inputTokens, outputTokens)

			// Store metrics (use agentID as task_id for spawn run commands)
			if err := storeTaskResult(db, agentID, agentID, modelUsed, costUSD, inputTokens, outputTokens); err != nil {
				fmt.Printf("⚠️  Warning: failed to store cost metrics: %v\n", err)
			} else {
				fmt.Printf("\n💰 Cost: $%.6f | Tokens: %d in / %d out | Model: %s\n", costUSD, inputTokens, outputTokens, modelUsed)
			}
		},
	}
	spawnRunCmd.Flags().IntVarP(&spawnRunTimeout, "timeout", "t", 120, "Timeout in seconds")
	spawnRunCmd.Flags().Bool("raw", false, "Send prompt without worker context prefix")

	// SPAWN REPL - Interactive REPL mode
	var spawnReplTimeout int
	var spawnReplCmd = &cobra.Command{
		Use:   "repl [agent-id] [backend]",
		Short: "Interactive REPL mode with an AI agent",
		Long: `Start an interactive REPL (Read-Eval-Print-Loop) session with an AI agent.
Type prompts and get responses interactively. Type 'exit' or 'quit' to end.

Example:
  flip spawn repl claude2.1 claude
  flip spawn repl gemini1 gemini`,
		Args: cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			agentID := args[0]
			backend := args[1]

			// Validate backend
			validBackends := map[string]bool{"claude": true, "gemini": true, "codex": true, "claude-code": true, "antigravity": true}
			if !validBackends[backend] {
				fmt.Printf("❌ Unknown backend '%s'. Valid: claude, gemini, codex, claude-code, antigravity\n", backend)
				return
			}

			fmt.Printf("🚀 Starting %s REPL for '%s'...\n", backend, agentID)

			// Create session
			ctx := context.Background()
			session, err := NewShellSession(ctx, agentID, backend)
			if err != nil {
				fmt.Printf("❌ Failed to create session: %v\n", err)
				return
			}
			defer session.Close()

			// Register as FLIP agent
			db := initDB()
			defer db.Close()

			_, err = db.Exec(`
				INSERT OR REPLACE INTO agents (id, name, status, hb, session_start, capabilities)
				VALUES (?, ?, 'idle', datetime('now'), datetime('now'), ?)`,
				agentID, agentID, backend)
			if err != nil {
				fmt.Printf("⚠️  Warning: failed to register agent: %v\n", err)
			}

			logEvent(db, "agent_spawned", agentID, "", fmt.Sprintf("backend=%s repl=true", backend))

			fmt.Println("✅ Session started. Type 'exit' or 'quit' to end.")
			fmt.Println(strings.Repeat("-", 50))

			scanner := bufio.NewScanner(os.Stdin)
			for {
				fmt.Printf("\n[%s]> ", agentID)
				if !scanner.Scan() {
					break
				}
				input := strings.TrimSpace(scanner.Text())

				if input == "" {
					continue
				}
				if input == "exit" || input == "quit" {
					fmt.Println("👋 Goodbye!")
					break
				}

				db.Exec("UPDATE agents SET status = 'busy', hb = datetime('now') WHERE id = ?", agentID)

				timeout := time.Duration(spawnReplTimeout) * time.Second
				response, err := session.SendPrompt(input, timeout)

				db.Exec("UPDATE agents SET status = 'idle', hb = datetime('now') WHERE id = ?", agentID)

				if err != nil {
					fmt.Printf("❌ Error: %v\n", err)
					if response != "" {
						fmt.Printf("Partial response:\n%s\n", response)
					}
					continue
				}

				fmt.Println()
				fmt.Println(response)
			}

			db.Exec("UPDATE agents SET status = 'offline', hb = datetime('now') WHERE id = ?", agentID)
			logEvent(db, "agent_stopped", agentID, "", "repl ended")
		},
	}
	spawnReplCmd.Flags().IntVarP(&spawnReplTimeout, "timeout", "t", 120, "Timeout per prompt in seconds")

	// SPAWN START - Start a new AI session (for server mode)
	var spawnStartCmd = &cobra.Command{
		Use:   "start [agent-id] [backend]",
		Short: "Start a new AI shell session (server mode)",
		Long: `Start a new shell session for an AI backend.
Note: Sessions only persist within a single process. Use 'spawn run' or 'spawn repl' instead.

Backends:
  claude  - Claude CLI (--dangerously-skip-permissions)
  gemini  - Gemini CLI

Example:
  flip spawn start claude2.1 claude
  flip spawn start gemini1 gemini`,
		Args: cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			agentID := args[0]
			backend := args[1]

			// Validate backend
			validBackends := map[string]bool{"claude": true, "gemini": true, "codex": true, "claude-code": true, "antigravity": true}
			if !validBackends[backend] {
				fmt.Printf("❌ Unknown backend '%s'. Valid: claude, gemini, codex, claude-code, antigravity\n", backend)
				return
			}

			// Check if session already exists
			if _, exists := GetShellSession(agentID); exists {
				fmt.Printf("❌ Session '%s' already exists. Use 'flip spawn stop %s' first.\n", agentID, agentID)
				return
			}

			fmt.Printf("🚀 Starting %s session for agent '%s'...\n", backend, agentID)

			// Create session
			ctx := context.Background()
			session, err := NewShellSession(ctx, agentID, backend)
			if err != nil {
				fmt.Printf("❌ Failed to create session: %v\n", err)
				return
			}

			// Register in global manager
			RegisterShellSession(session)

			// Register as FLIP agent
			db := initDB()
			defer db.Close()

			_, err = db.Exec(`
				INSERT OR REPLACE INTO agents (id, name, status, hb, session_start, capabilities)
				VALUES (?, ?, 'idle', datetime('now'), datetime('now'), ?)`,
				agentID, agentID, backend)
			if err != nil {
				fmt.Printf("⚠️  Session started but failed to register agent: %v\n", err)
			}

			logEvent(db, "agent_spawned", agentID, "", fmt.Sprintf("backend=%s session_id=%s", backend, session.ID))

			fmt.Printf("✅ Session '%s' started (ID: %s)\n", agentID, session.ID)
			fmt.Printf("   Backend: %s\n", backend)
			fmt.Printf("   Note: Session only persists in current process.\n")
			fmt.Printf("   Use 'flip spawn repl' for interactive mode.\n")
		},
	}

	// SPAWN SEND - Send a prompt to an agent
	var spawnTimeout int
	var spawnSendCmd = &cobra.Command{
		Use:   "send [agent-id] [prompt]",
		Short: "Send a prompt to an AI agent",
		Args:  cobra.MinimumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			agentID := args[0]
			prompt := strings.Join(args[1:], " ")

			session, exists := GetShellSession(agentID)
			if !exists {
				fmt.Printf("❌ No session found for '%s'. Start with: ./flip spawn start %s <backend>\n", agentID, agentID)
				return
			}

			fmt.Printf("📤 Sending prompt to %s...\n", agentID)

			// Update agent status
			db := initDB()
			defer db.Close()
			db.Exec("UPDATE agents SET status = 'busy', hb = datetime('now') WHERE id = ?", agentID)

			// Send prompt
			timeout := time.Duration(spawnTimeout) * time.Second
			response, err := session.SendPrompt(prompt, timeout)

			// Update status back
			db.Exec("UPDATE agents SET status = 'idle', hb = datetime('now') WHERE id = ?", agentID)

			if err != nil {
				fmt.Printf("❌ Error: %v\n", err)
				if response != "" {
					fmt.Printf("\nPartial response:\n%s\n", response)
				}
				return
			}

			fmt.Printf("\n📥 Response from %s:\n", agentID)
			fmt.Println(strings.Repeat("-", 50))
			fmt.Println(response)
			fmt.Println(strings.Repeat("-", 50))
		},
	}
	spawnSendCmd.Flags().IntVarP(&spawnTimeout, "timeout", "t", 120, "Timeout in seconds")

	// SPAWN LIST - List active sessions
	var spawnListCmd = &cobra.Command{
		Use:   "list",
		Short: "List all active AI sessions",
		Run: func(cmd *cobra.Command, args []string) {
			sessions := ListShellSessions()

			if len(sessions) == 0 {
				fmt.Println("No active sessions.")
				fmt.Println("Start one with: ./flip spawn start <agent-id> <backend>")
				return
			}

			fmt.Printf("ACTIVE SESSIONS (%d)\n", len(sessions))
			fmt.Println(strings.Repeat("=", 60))
			fmt.Printf("%-15s %-10s %-10s %-20s\n", "AGENT", "BACKEND", "STATE", "STARTED")
			fmt.Println(strings.Repeat("-", 60))

			for _, s := range sessions {
				started := s.CreatedAt.Format("Jan 02 15:04:05")
				fmt.Printf("%-15s %-10s %-10s %-20s\n", s.AgentID, s.Backend, s.State, started)
			}
		},
	}

	// SPAWN STOP - Stop a session
	var spawnStopCmd = &cobra.Command{
		Use:   "stop [agent-id]",
		Short: "Stop an AI session",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			agentID := args[0]

			err := CloseShellSession(agentID)
			if err != nil {
				fmt.Printf("❌ Failed to stop session: %v\n", err)
				return
			}

			// Update agent status
			db := initDB()
			defer db.Close()
			db.Exec("UPDATE agents SET status = 'offline', hb = datetime('now') WHERE id = ?", agentID)
			logEvent(db, "agent_stopped", agentID, "", "session closed")

			fmt.Printf("✅ Session '%s' stopped.\n", agentID)
		},
	}

	// SPAWN STOP-ALL - Stop all sessions
	var spawnStopAllCmd = &cobra.Command{
		Use:   "stop-all",
		Short: "Stop all AI sessions",
		Run: func(cmd *cobra.Command, args []string) {
			sessions := ListShellSessions()
			if len(sessions) == 0 {
				fmt.Println("No active sessions to stop.")
				return
			}

			db := initDB()
			defer db.Close()

			for _, s := range sessions {
				if err := CloseShellSession(s.AgentID); err != nil {
					fmt.Printf("❌ Failed to stop %s: %v\n", s.AgentID, err)
				} else {
					db.Exec("UPDATE agents SET status = 'offline', hb = datetime('now') WHERE id = ?", s.AgentID)
					fmt.Printf("✅ Stopped: %s\n", s.AgentID)
				}
			}
		},
	}

	// Register spawn subcommands
	spawnCmd.AddCommand(spawnRunCmd, spawnReplCmd, spawnStartCmd, spawnSendCmd, spawnListCmd, spawnStopCmd, spawnStopAllCmd)

	// ===================
	// ORCHESTRATE Command (v5.1)
	// ===================
	var orchestrateCmd = &cobra.Command{
		Use:   "orchestrate",
		Short: "Autonomous agent orchestration system",
		Long: `Run the autonomous orchestration system that manages AI agents.

The orchestrator:
  - Auto-spawns agents when queue grows
  - Recovers crashed/stale agents
  - Assigns tasks to best-fit agents
  - Persists state to disk every 30s
  - Handles timeouts and retries

Subcommands:
  start   - Start the orchestration loop
  status  - Show orchestrator status
  config  - View/modify configuration
  queue   - Add task to orchestrator queue`,
	}

	// ORCHESTRATE START
	var orchMaxAgents int
	var orchBackend string
	var orchAutoScale bool
	var orchestrateStartCmd = &cobra.Command{
		Use:   "start",
		Short: "Start the orchestration loop",
		Long: `Start the autonomous orchestration loop.

This will:
  1. Load previous state (if any)
  2. Spawn minimum idle agents
  3. Begin monitoring and auto-assignment loop
  4. Save state every 30 seconds

Press Ctrl+C to stop gracefully.`,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("🎯 FLIP Orchestrator v5.1")
			fmt.Println(strings.Repeat("=", 50))

			// Load or create state
			var err error
			orchestrator, err = LoadOrchestratorState()
			if err != nil {
				fmt.Printf("⚠️  Could not load state, starting fresh: %v\n", err)
				orchestrator = NewOrchestrator()
			} else {
				fmt.Printf("📂 Loaded state from %s\n", orchestrator.LastSaveAt.Format("Jan 02 15:04:05"))
			}

			// Apply command-line config
			if orchMaxAgents > 0 {
				orchestrator.Config.MaxAgents = orchMaxAgents
			}
			if orchBackend != "" {
				orchestrator.Config.DefaultBackend = orchBackend
			}
			orchestrator.Config.AutoScale = orchAutoScale

			fmt.Printf("\n📋 Configuration:\n")
			fmt.Printf("   Max agents: %d\n", orchestrator.Config.MaxAgents)
			fmt.Printf("   Min idle: %d\n", orchestrator.Config.MinIdleAgents)
			fmt.Printf("   Default backend: %s\n", orchestrator.Config.DefaultBackend)
			fmt.Printf("   Auto-scale: %v\n", orchestrator.Config.AutoScale)
			fmt.Printf("   Task timeout: %v\n", orchestrator.Config.TaskTimeout)
			fmt.Printf("   Save interval: %v\n", orchestrator.Config.SaveInterval)

			// Initialize database
			db := initDB()
			defer db.Close()

			// Create context with cancellation
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// Handle graceful shutdown
			go func() {
				sigCh := make(chan os.Signal, 1)
				// Note: We can't import signal package in this edit, so we'll skip signal handling
				// The user can Ctrl+C which will kill the process
				<-sigCh
				fmt.Println("\n🛑 Shutting down...")
				cancel()
			}()

			fmt.Println("\n🚀 Starting orchestration loop...")
			fmt.Println("   Press Ctrl+C to stop\n")

			logEvent(db, "orchestrator_start", "", "", fmt.Sprintf("version=%s max_agents=%d", orchestrator.Version, orchestrator.Config.MaxAgents))

			// Run the main loop
			orchestrator.RunOrchestrationLoop(ctx, db)

			fmt.Println("✅ Orchestrator stopped. State saved.")
		},
	}
	orchestrateStartCmd.Flags().IntVar(&orchMaxAgents, "max-agents", 0, "Maximum number of agents (0 = use saved config)")
	orchestrateStartCmd.Flags().StringVar(&orchBackend, "backend", "", "Default backend (claude/gemini)")
	orchestrateStartCmd.Flags().BoolVar(&orchAutoScale, "auto-scale", true, "Enable auto-scaling")

	// ORCHESTRATE STATUS
	var orchestrateStatusCmd = &cobra.Command{
		Use:   "status",
		Short: "Show orchestrator status",
		Run: func(cmd *cobra.Command, args []string) {
			state, err := LoadOrchestratorState()
			if err != nil {
				fmt.Printf("❌ Failed to load state: %v\n", err)
				return
			}

			fmt.Println("🎯 FLIP Orchestrator Status")
			fmt.Println(strings.Repeat("=", 50))

			fmt.Printf("\n📊 Metrics:\n")
			fmt.Printf("   Version: %s\n", state.Version)
			fmt.Printf("   Started: %s\n", state.StartedAt.Format("Jan 02 15:04:05"))
			fmt.Printf("   Last save: %s\n", state.LastSaveAt.Format("Jan 02 15:04:05"))
			fmt.Printf("   Uptime: %v\n", state.Metrics.Uptime)
			fmt.Printf("   Tasks assigned: %d\n", state.Metrics.TasksAssigned)
			fmt.Printf("   Tasks completed: %d\n", state.Metrics.TasksCompleted)
			fmt.Printf("   Tasks failed: %d\n", state.Metrics.TasksFailed)
			fmt.Printf("   Agents spawned: %d\n", state.Metrics.AgentsSpawned)
			fmt.Printf("   Agents respawned: %d\n", state.Metrics.AgentsRespawned)

			fmt.Printf("\n👥 Agent Pool (%d):\n", len(state.AgentPool))
			for id, agent := range state.AgentPool {
				autoTag := ""
				if agent.AutoSpawned {
					autoTag = " [auto]"
				}
				fmt.Printf("   %-15s %s/%s %s%s\n", id, agent.Backend, agent.Role, agent.Status, autoTag)
				if agent.CurrentTask != "" {
					fmt.Printf("                  └─ Task: %s\n", agent.CurrentTask)
				}
			}

			fmt.Printf("\n📋 Task Queue (%d):\n", len(state.TaskQueue))
			for i, task := range state.TaskQueue {
				if i >= 5 {
					fmt.Printf("   ... and %d more\n", len(state.TaskQueue)-5)
					break
				}
				fmt.Printf("   [P%d] %s - %s\n", task.Priority, task.TaskID, task.Title)
			}

			fmt.Printf("\n⚡ Active Tasks (%d):\n", len(state.ActiveTasks))
			for _, task := range state.ActiveTasks {
				duration := time.Since(task.StartedAt).Round(time.Second)
				fmt.Printf("   %s → %s (%v, %d%% progress)\n", task.TaskID, task.AgentID, duration, task.Progress)
			}
		},
	}

	// ORCHESTRATE QUEUE - Add task to queue
	var orchQueuePriority int
	var orchQueueBackend string
	var orchQueueCaps string
	var orchestrateQueueCmd = &cobra.Command{
		Use:   "queue [task-id]",
		Short: "Add a task to the orchestrator queue",
		Long: `Add a FLIP task to the orchestrator's automatic assignment queue.

Example:
  flip orchestrate queue TASK-001 --priority 5 --backend claude
  flip orchestrate queue TASK-002 --caps coding,debugging`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			taskID := args[0]

			// Load state
			state, err := LoadOrchestratorState()
			if err != nil {
				fmt.Printf("❌ Failed to load state: %v\n", err)
				return
			}

			// Get task info from database
			db := initDB()
			defer db.Close()

			var title string
			var priority int
			err = db.QueryRow("SELECT title, priority FROM tasks WHERE id = ?", taskID).Scan(&title, &priority)
			if err != nil {
				fmt.Printf("❌ Task not found: %s\n", taskID)
				return
			}

			// Override priority if specified
			if orchQueuePriority > 0 {
				priority = orchQueuePriority
			}

			// Parse capabilities
			var caps []string
			if orchQueueCaps != "" {
				caps = strings.Split(orchQueueCaps, ",")
			}

			// Add to queue
			state.QueueTask(taskID, title, priority, caps, orchQueueBackend)

			// Save state
			if err := state.SaveState(); err != nil {
				fmt.Printf("❌ Failed to save state: %v\n", err)
				return
			}

			fmt.Printf("✅ Queued task %s (P%d)\n", taskID, priority)
			if orchQueueBackend != "" {
				fmt.Printf("   Preferred backend: %s\n", orchQueueBackend)
			}
			if len(caps) > 0 {
				fmt.Printf("   Required capabilities: %v\n", caps)
			}
			fmt.Printf("   Queue position: %d\n", len(state.TaskQueue))
		},
	}
	orchestrateQueueCmd.Flags().IntVar(&orchQueuePriority, "priority", 0, "Task priority (higher = more urgent)")
	orchestrateQueueCmd.Flags().StringVar(&orchQueueBackend, "backend", "", "Preferred backend (claude/gemini)")
	orchestrateQueueCmd.Flags().StringVar(&orchQueueCaps, "caps", "", "Required capabilities (comma-separated)")

	// ORCHESTRATE CONFIG
	var orchestrateConfigCmd = &cobra.Command{
		Use:   "config",
		Short: "View/modify orchestrator configuration",
		Run: func(cmd *cobra.Command, args []string) {
			state, err := LoadOrchestratorState()
			if err != nil {
				fmt.Printf("❌ Failed to load state: %v\n", err)
				return
			}

			fmt.Println("🔧 Orchestrator Configuration")
			fmt.Println(strings.Repeat("=", 50))
			configJSON, _ := json.MarshalIndent(state.Config, "", "  ")
			fmt.Println(string(configJSON))

			fmt.Println("\n📁 State file:", GetOrchestratorStatePath())
		},
	}

	// ORCHESTRATE ANALYZE - Use Claude to analyze system state and make recommendations
	var orchestrateAnalyzeCmd = &cobra.Command{
		Use:   "analyze",
		Short: "Use Claude to analyze system state and recommend actions",
		Long: `Spawn a Claude agent to analyze the current FLIP system state and make
intelligent recommendations for task assignment, agent scaling, and workflow optimization.

This is a "coordinator" function that uses AI to make strategic decisions.`,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("🧠 FLIP Coordinator Analysis")
			fmt.Println(strings.Repeat("=", 50))

			// Gather system state
			db := initDB()
			defer db.Close()

			// Get agents
			var agents []string
			rows, _ := db.Query("SELECT id, status, COALESCE(task, '-') FROM agents")
			for rows.Next() {
				var id, status, task string
				rows.Scan(&id, &status, &task)
				agents = append(agents, fmt.Sprintf("%s: %s (task: %s)", id, status, task))
			}
			rows.Close()

			// Get tasks
			var tasks []string
			rows, _ = db.Query("SELECT id, title, status, COALESCE(assignee, '-') FROM tasks WHERE status != 'done' ORDER BY priority DESC LIMIT 10")
			for rows.Next() {
				var id, title, status, assignee string
				rows.Scan(&id, &title, &status, &assignee)
				tasks = append(tasks, fmt.Sprintf("%s: %s [%s] (assigned: %s)", id, title, status, assignee))
			}
			rows.Close()

			// Get unread signals
			var signalCount int
			db.QueryRow("SELECT COUNT(*) FROM signals WHERE read = 0").Scan(&signalCount)

			// Build analysis prompt
			prompt := fmt.Sprintf(`You are the FLIP Coordinator. Analyze this system state and provide brief recommendations.

AGENTS (%d):
%s

PENDING TASKS (%d):
%s

UNREAD SIGNALS: %d

Based on this state, provide:
1. Assessment (2-3 sentences)
2. Top 3 recommended actions (one line each)
3. Any concerns or blockers

Keep response under 200 words. Be specific and actionable.`,
				len(agents), strings.Join(agents, "\n"),
				len(tasks), strings.Join(tasks, "\n"),
				signalCount)

			fmt.Println("\n📊 Gathering system state...")
			fmt.Printf("   Agents: %d\n", len(agents))
			fmt.Printf("   Pending tasks: %d\n", len(tasks))
			fmt.Printf("   Unread signals: %d\n", signalCount)

			fmt.Println("\n🤖 Spawning Claude for analysis...")

			// Create a session for analysis
			ctx := context.Background()
			session, err := NewShellSession(ctx, "coordinator-tmp", "claude")
			if err != nil {
				fmt.Printf("❌ Failed to spawn coordinator: %v\n", err)
				return
			}
			defer session.Close()

			response, err := session.SendPrompt(prompt, 90*time.Second)
			if err != nil {
				fmt.Printf("❌ Analysis failed: %v\n", err)
				return
			}

			fmt.Println("\n📋 Coordinator Analysis:")
			fmt.Println(strings.Repeat("-", 50))
			fmt.Println(response)
			fmt.Println(strings.Repeat("-", 50))

			logEvent(db, "coordinator_analysis", "coordinator", "", "analysis_complete")
		},
	}

	// Register orchestrate subcommands
	orchestrateCmd.AddCommand(orchestrateStartCmd, orchestrateStatusCmd, orchestrateQueueCmd, orchestrateConfigCmd, orchestrateAnalyzeCmd)

	// ===================
	// PIPELINE Command (v5.1)
	// ===================
	var pipelineCmd = &cobra.Command{
		Use:   "pipeline",
		Short: "Run multi-agent pipelines",
		Long: `Chain multiple AI agents together for complex workflows.

Available pipelines:
  data-analyze  - Gemini preprocesses data, Claude analyzes
  plan-execute  - Claude plans, Gemini executes bulk operations
  research      - Gemini gathers info, Claude synthesizes insights
  code-review   - Gemini scans code, Claude does deep review

Examples:
  flip pipeline run data-analyze "$(cat logs.txt)"
  flip pipeline run research "Go concurrency patterns"
  flip pipeline list`,
	}

	// PIPELINE LIST - List available pipelines
	var pipelineListCmd = &cobra.Command{
		Use:   "list",
		Short: "List available pipeline templates",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("🔗 Available Pipelines")
			fmt.Println(strings.Repeat("=", 60))

			for id, pipeline := range PipelineTemplates {
				fmt.Printf("\n📋 %s\n", id)
				fmt.Printf("   Name: %s\n", pipeline.Name)
				fmt.Printf("   Description: %s\n", pipeline.Description)
				fmt.Printf("   Stages:\n")
				for i, stage := range pipeline.Stages {
					fmt.Printf("     %d. %s (%s) - %s\n", i+1, stage.Name, stage.Backend, stage.Role)
				}
			}

			fmt.Println("\n" + strings.Repeat("-", 60))
			fmt.Println("Usage: flip pipeline run <pipeline-id> \"<input>\"")
		},
	}

	// PIPELINE RUN - Run a pipeline
	var pipelineRunCmd = &cobra.Command{
		Use:   "run [pipeline-id] [input]",
		Short: "Run a pipeline with the given input",
		Long: `Execute a multi-agent pipeline with the given input.

The pipeline chains multiple agents together, each processing
the output of the previous stage.

Examples:
  flip pipeline run data-analyze "$(cat server.log)"
  flip pipeline run research "Kubernetes best practices"
  flip pipeline run code-review "$(cat main.go)"
  flip pipeline run plan-execute "Refactor the authentication module"`,
		Args: cobra.MinimumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			pipelineID := args[0]
			input := strings.Join(args[1:], " ")

			// Get pipeline template
			pipeline, exists := PipelineTemplates[pipelineID]
			if !exists {
				fmt.Printf("❌ Unknown pipeline: %s\n", pipelineID)
				fmt.Println("Available pipelines:")
				for id := range PipelineTemplates {
					fmt.Printf("  - %s\n", id)
				}
				return
			}

			pipeline.ID = pipelineID

			fmt.Printf("🔗 Running Pipeline: %s\n", pipeline.Name)
			fmt.Println(strings.Repeat("=", 60))
			fmt.Printf("Description: %s\n", pipeline.Description)
			fmt.Printf("Stages: %d\n", len(pipeline.Stages))
			fmt.Printf("Input length: %d chars\n", len(input))

			db := initDB()
			defer db.Close()

			ctx := context.Background()

			logEvent(db, "pipeline_start", "", pipelineID, fmt.Sprintf("stages=%d input_len=%d", len(pipeline.Stages), len(input)))

			result, err := RunPipeline(ctx, pipeline, input, db)

			fmt.Println("\n" + strings.Repeat("=", 60))

			if err != nil {
				fmt.Printf("❌ Pipeline failed: %v\n", err)
				logEvent(db, "pipeline_failed", "", pipelineID, err.Error())
			}

			if result != nil {
				fmt.Printf("📊 Pipeline Results\n")
				fmt.Printf("   Success: %v\n", result.Success)
				fmt.Printf("   Total time: %v\n", result.TotalTime.Round(time.Millisecond))
				fmt.Printf("   Stage times:\n")
				for stage, duration := range result.StageTimes {
					fmt.Printf("     - %s: %v\n", stage, duration.Round(time.Millisecond))
				}

				if len(result.Errors) > 0 {
					fmt.Println("   Errors:")
					for _, e := range result.Errors {
						fmt.Printf("     ⚠️  %s\n", e)
					}
				}

				fmt.Println("\n" + strings.Repeat("-", 60))
				fmt.Println("📤 FINAL OUTPUT:")
				fmt.Println(strings.Repeat("-", 60))
				fmt.Println(result.FinalOutput)

				logEvent(db, "pipeline_complete", "", pipelineID,
					fmt.Sprintf("success=%v duration=%v", result.Success, result.TotalTime))
			}
		},
	}

	// PIPELINE INFO - Show details about a specific pipeline
	var pipelineInfoCmd = &cobra.Command{
		Use:   "info [pipeline-id]",
		Short: "Show detailed info about a pipeline",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			pipelineID := args[0]

			pipeline, exists := PipelineTemplates[pipelineID]
			if !exists {
				fmt.Printf("❌ Unknown pipeline: %s\n", pipelineID)
				return
			}

			fmt.Printf("🔗 Pipeline: %s\n", pipelineID)
			fmt.Println(strings.Repeat("=", 60))
			fmt.Printf("Name: %s\n", pipeline.Name)
			fmt.Printf("Description: %s\n\n", pipeline.Description)

			fmt.Println("Stages:")
			for i, stage := range pipeline.Stages {
				fmt.Printf("\n  Stage %d: %s\n", i+1, stage.Name)
				fmt.Printf("  Backend: %s\n", stage.Backend)
				fmt.Printf("  Role: %s\n", stage.Role)
				fmt.Printf("  Timeout: %v\n", stage.Timeout)
				fmt.Printf("  Max tokens: %d\n", stage.MaxTokens)
				fmt.Printf("  Pass-through: %v\n", stage.PassThrough)
				fmt.Println("  Prompt template:")
				// Show first 200 chars of prompt
				promptPreview := stage.Prompt
				if len(promptPreview) > 200 {
					promptPreview = promptPreview[:200] + "..."
				}
				fmt.Printf("    %s\n", strings.ReplaceAll(promptPreview, "\n", "\n    "))
			}
		},
	}

	// Register pipeline subcommands
	pipelineCmd.AddCommand(pipelineListCmd, pipelineRunCmd, pipelineInfoCmd)

	// Antigravity command - for human-in-the-loop integration with GUI Gemini 2.5 Pro
	var antigravityCmd = &cobra.Command{
		Use:   "antigravity",
		Short: "Manage Antigravity (GUI Gemini 2.5 Pro) integration",
		Long:  "Antigravity is a GUI-based Gemini 2.5 Pro interface. Use for second opinions, browser testing, visual tasks.",
	}

	var antigravitySetupCmd = &cobra.Command{
		Use:   "setup",
		Short: "Register antigravity as a FLIP agent",
		Run: func(cmd *cobra.Command, args []string) {
			db := initDB()
			defer db.Close()

			// Register antigravity as manual agent
			_, err := db.Exec(`
				INSERT OR REPLACE INTO agents (id, status, hb, capabilities, task, task_phase, blocked_by, status_reason, status_detail)
				VALUES ('antigravity', 'idle', datetime('now'), 'gemini-2.5-pro,visual,browser-testing,second-opinion,manual,powerful', '', '', '', 'Manual GUI agent', 'Human-operated Gemini 2.5 Pro interface')
			`)
			if err != nil {
				fmt.Printf("❌ Error: %v\n", err)
				return
			}

			fmt.Println("✅ Antigravity registered as FLIP agent")
			fmt.Println("")
			fmt.Println("📋 Capabilities: gemini-2.5-pro, visual, browser-testing, second-opinion, manual, powerful")
			fmt.Println("")
			fmt.Println("🔧 Usage:")
			fmt.Println("  Send task:     ./flip antigravity ask \"<question>\"")
			fmt.Println("  Browser test:  ./flip antigravity test \"<url>\" \"<instructions>\"")
			fmt.Println("  Second opinion: ./flip antigravity review \"<content>\"")
			fmt.Println("  Check inbox:   ./flip antigravity inbox")
			fmt.Println("  Respond:       ./flip antigravity respond \"<task-id>\" \"<response>\"")
		},
	}

	var antigravityAskCmd = &cobra.Command{
		Use:   "ask <question>",
		Short: "Send a question to Antigravity for second opinion",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			db := initDB()
			defer db.Close()

			question := strings.Join(args, " ")
			priorityStr, _ := cmd.Flags().GetString("priority")
			priority := 1 // normal
			switch priorityStr {
			case "critical":
				priority = 4
			case "high":
				priority = 3
			case "low":
				priority = 0
			}

			taskID := fmt.Sprintf("AG-%d", time.Now().UnixNano()%1000000)

			// Create task for antigravity
			_, err := db.Exec(`
				INSERT INTO tasks (id, title, description, status, assignee, priority, created_at)
				VALUES (?, ?, ?, 'todo', 'antigravity', ?, datetime('now'))
			`, taskID, "Second Opinion Request", question, priority)
			if err != nil {
				fmt.Printf("❌ Error creating task: %v\n", err)
				return
			}

			// Send signal
			_, err = db.Exec(`
				INSERT INTO signals (id, from_agent, to_agent, msg, priority, signal_type, subject, related_task, created_at)
				VALUES (?, 'coordinator', 'antigravity', ?, ?, 'second-opinion', ?, ?, datetime('now'))
			`, fmt.Sprintf("SIG-%d", time.Now().UnixNano()%1000000), question, priorityStr, taskID, taskID)
			if err != nil {
				fmt.Printf("❌ Error sending signal: %v\n", err)
				return
			}

			fmt.Printf("📤 Question sent to Antigravity\n")
			fmt.Printf("   Task ID: %s\n", taskID)
			fmt.Printf("   Priority: %s\n", priorityStr)
			fmt.Println("")
			fmt.Println("📋 In Antigravity GUI:")
			fmt.Println("   1. Check inbox: ./flip antigravity inbox")
			fmt.Println("   2. Process the question in Gemini 2.5 Pro")
			fmt.Printf("   3. Respond: ./flip antigravity respond %s \"<your response>\"\n", taskID)
		},
	}
	antigravityAskCmd.Flags().StringP("priority", "p", "normal", "Priority: critical, high, normal, low")

	var antigravityTestCmd = &cobra.Command{
		Use:   "test <url> <instructions>",
		Short: "Request browser testing from Antigravity",
		Args:  cobra.MinimumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			db := initDB()
			defer db.Close()

			url := args[0]
			instructions := strings.Join(args[1:], " ")
			taskID := fmt.Sprintf("BT-%d", time.Now().UnixNano()%1000000)

			message := fmt.Sprintf("BROWSER TEST REQUEST\n\nURL: %s\n\nInstructions:\n%s", url, instructions)

			// Create task (priority 3 = high)
			_, err := db.Exec(`
				INSERT INTO tasks (id, title, description, status, assignee, priority, created_at)
				VALUES (?, ?, ?, 'todo', 'antigravity', 3, datetime('now'))
			`, taskID, "Browser Test: "+url, message)
			if err != nil {
				fmt.Printf("❌ Error creating task: %v\n", err)
				return
			}

			// Send signal
			_, err = db.Exec(`
				INSERT INTO signals (id, from_agent, to_agent, msg, priority, signal_type, subject, related_task, created_at)
				VALUES (?, 'coordinator', 'antigravity', ?, 'high', 'browser-test', ?, ?, datetime('now'))
			`, fmt.Sprintf("SIG-%d", time.Now().UnixNano()%1000000), message, taskID, taskID)
			if err != nil {
				fmt.Printf("❌ Error sending signal: %v\n", err)
				return
			}

			fmt.Printf("🌐 Browser test request sent to Antigravity\n")
			fmt.Printf("   Task ID: %s\n", taskID)
			fmt.Printf("   URL: %s\n", url)
			fmt.Println("")
			fmt.Println("📋 In Antigravity:")
			fmt.Println("   1. Open URL in browser")
			fmt.Println("   2. Follow test instructions")
			fmt.Printf("   3. Report: ./flip antigravity respond %s \"<test results>\"\n", taskID)
		},
	}

	var antigravityReviewCmd = &cobra.Command{
		Use:   "review <content>",
		Short: "Request code/content review from Antigravity",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			db := initDB()
			defer db.Close()

			content := strings.Join(args, " ")
			taskID := fmt.Sprintf("RV-%d", time.Now().UnixNano()%1000000)

			message := fmt.Sprintf("REVIEW REQUEST\n\nContent to review:\n%s\n\nPlease provide:\n1. Issues found\n2. Suggestions\n3. Overall assessment", content)

			// Create task (priority 1 = normal)
			_, err := db.Exec(`
				INSERT INTO tasks (id, title, description, status, assignee, priority, created_at)
				VALUES (?, ?, ?, 'todo', 'antigravity', 1, datetime('now'))
			`, taskID, "Review Request", message)
			if err != nil {
				fmt.Printf("❌ Error: %v\n", err)
				return
			}

			// Send signal
			_, err = db.Exec(`
				INSERT INTO signals (id, from_agent, to_agent, msg, priority, signal_type, subject, related_task, created_at)
				VALUES (?, 'coordinator', 'antigravity', ?, 'normal', 'review', ?, ?, datetime('now'))
			`, fmt.Sprintf("SIG-%d", time.Now().UnixNano()%1000000), message, taskID, taskID)
			if err != nil {
				fmt.Printf("❌ Error: %v\n", err)
				return
			}

			fmt.Printf("📝 Review request sent to Antigravity\n")
			fmt.Printf("   Task ID: %s\n", taskID)
			fmt.Printf("   Respond: ./flip antigravity respond %s \"<review>\"\n", taskID)
		},
	}

	var antigravityInboxCmd = &cobra.Command{
		Use:   "inbox",
		Short: "Check Antigravity's pending tasks and signals",
		Run: func(cmd *cobra.Command, args []string) {
			db := initDB()
			defer db.Close()

			fmt.Println("📥 Antigravity Inbox")
			fmt.Println("============================================================")

			// Pending tasks
			rows, err := db.Query(`
				SELECT id, title, priority, created_at
				FROM tasks
				WHERE assignee = 'antigravity' AND status = 'todo'
				ORDER BY priority DESC, created_at DESC
			`)
			if err != nil {
				fmt.Printf("❌ Error: %v\n", err)
				return
			}
			defer rows.Close()

			taskCount := 0
			for rows.Next() {
				var id, title, created string
				var priority int
				rows.Scan(&id, &title, &priority, &created)
				taskCount++
				priorityStr := "normal"
				switch priority {
				case 4:
					priorityStr = "critical"
				case 3:
					priorityStr = "high"
				case 0:
					priorityStr = "low"
				}
				fmt.Printf("\n📋 Task: %s [%s]\n", id, priorityStr)
				fmt.Printf("   Title: %s\n", title)
				fmt.Printf("   Created: %s\n", created)
			}

			if taskCount == 0 {
				fmt.Println("\n✅ No pending tasks")
			}

			// Unread signals
			fmt.Println("\n------------------------------------------------------------")
			rows2, err := db.Query(`
				SELECT id, from_agent, msg, priority, signal_type, created_at
				FROM signals
				WHERE to_agent = 'antigravity' AND read = 0
				ORDER BY
					CASE priority
						WHEN 'critical' THEN 1
						WHEN 'high' THEN 2
						WHEN 'normal' THEN 3
						WHEN 'low' THEN 4
					END, created_at DESC
				LIMIT 10
			`)
			if err != nil {
				fmt.Printf("❌ Error: %v\n", err)
				return
			}
			defer rows2.Close()

			signalCount := 0
			for rows2.Next() {
				var id, from, msg, priority, sigType, created string
				rows2.Scan(&id, &from, &msg, &priority, &sigType, &created)
				signalCount++
				fmt.Printf("\n📨 Signal %s from %s [%s] (%s)\n", id, from, priority, sigType)
				// Truncate long messages
				if len(msg) > 200 {
					msg = msg[:200] + "..."
				}
				fmt.Printf("   %s\n", strings.ReplaceAll(msg, "\n", "\n   "))
			}

			if signalCount == 0 {
				fmt.Println("\n✅ No unread signals")
			}

			fmt.Println("\n============================================================")
			fmt.Printf("Summary: %d pending tasks, %d unread signals\n", taskCount, signalCount)
		},
	}

	var antigravityRespondCmd = &cobra.Command{
		Use:   "respond <task-id> <response>",
		Short: "Send response for a task",
		Args:  cobra.MinimumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			db := initDB()
			defer db.Close()

			taskID := args[0]
			response := strings.Join(args[1:], " ")

			// Update task status
			result, err := db.Exec(`
				UPDATE tasks SET status = 'done', updated_at = datetime('now')
				WHERE id = ? AND assignee = 'antigravity'
			`, taskID)
			if err != nil {
				fmt.Printf("❌ Error: %v\n", err)
				return
			}
			affected, _ := result.RowsAffected()
			if affected == 0 {
				fmt.Printf("⚠️  Task %s not found or not assigned to antigravity\n", taskID)
			}

			// Send response signal back to coordinator
			_, err = db.Exec(`
				INSERT INTO signals (id, from_agent, to_agent, msg, priority, signal_type, subject, related_task, created_at)
				VALUES (?, 'antigravity', 'coordinator', ?, 'normal', 'response', ?, ?, datetime('now'))
			`, fmt.Sprintf("SIG-%d", time.Now().UnixNano()%1000000), response, taskID, taskID)
			if err != nil {
				fmt.Printf("❌ Error sending response: %v\n", err)
				return
			}

			// Mark related signals as read
			db.Exec(`UPDATE signals SET read = 1 WHERE to_agent = 'antigravity' AND subject = ?`, taskID)

			fmt.Printf("✅ Response sent for task %s\n", taskID)
			fmt.Println("")
			fmt.Println("The response has been recorded and the task marked complete.")
		},
	}

	var antigravityStatusCmd = &cobra.Command{
		Use:   "status",
		Short: "Show Antigravity's current status",
		Run: func(cmd *cobra.Command, args []string) {
			db := initDB()
			defer db.Close()

			var status, lastHeart, caps, task, reason string
			err := db.QueryRow(`
				SELECT status, hb, capabilities, task, status_reason
				FROM agents WHERE id = 'antigravity'
			`).Scan(&status, &lastHeart, &caps, &task, &reason)

			if err != nil {
				fmt.Println("⚠️  Antigravity not registered. Run: ./flip antigravity setup")
				return
			}

			fmt.Println("🚀 Antigravity Status")
			fmt.Println("============================================================")
			fmt.Printf("Status: %s\n", status)
			fmt.Printf("Last activity: %s\n", lastHeart)
			fmt.Printf("Capabilities: %s\n", caps)
			if task != "" {
				fmt.Printf("Current task: %s\n", task)
			}
			if reason != "" {
				fmt.Printf("Note: %s\n", reason)
			}

			// Count pending work
			var pendingTasks, unreadSignals int
			db.QueryRow(`SELECT COUNT(*) FROM tasks WHERE assignee = 'antigravity' AND status = 'todo'`).Scan(&pendingTasks)
			db.QueryRow(`SELECT COUNT(*) FROM signals WHERE to_agent = 'antigravity' AND read = 0`).Scan(&unreadSignals)

			fmt.Println("------------------------------------------------------------")
			fmt.Printf("Pending tasks: %d\n", pendingTasks)
			fmt.Printf("Unread signals: %d\n", unreadSignals)

			if pendingTasks > 0 || unreadSignals > 0 {
				fmt.Println("\nRun: ./flip antigravity inbox")
			}
		},
	}

	// Register antigravity subcommands
	antigravityCmd.AddCommand(antigravitySetupCmd, antigravityAskCmd, antigravityTestCmd, antigravityReviewCmd, antigravityInboxCmd, antigravityRespondCmd, antigravityStatusCmd)

	// ============================================================================
	// AGENT Command - Capability Registry
	// ============================================================================
	var agentCmd = &cobra.Command{
		Use:   "agent",
		Short: "Agent management commands",
		Long:  "Commands for managing agent capabilities and finding agents by capability",
	}

	// AGENT REGISTER - Register agent with capabilities
	var agentRegisterCapabilities string
	var agentRegisterCmd = &cobra.Command{
		Use:   "register [agent_id] --capabilities code,test,browser",
		Short: "Register or update agent capabilities",
		Long: `Register or update an agent's capabilities for task matching.
Capabilities should be comma-separated values like: code,test,browser,sql,review

Examples:
  flip agent register claude5 --capabilities code,test,browser
  flip agent register gemini2 --capabilities code,sql,review`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			db := initDB()
			defer db.Close()

			agentID := args[0]

			// Check if agent exists
			var exists int
			err := db.QueryRow("SELECT COUNT(*) FROM agents WHERE id = ?", agentID).Scan(&exists)
			if err != nil {
				log.Fatalf("Database error: %v", err)
			}

			if exists == 0 {
				log.Fatalf("Agent '%s' not found. Register the agent first with: flip register %s \"Display Name\"", agentID, agentID)
			}

			// Update capabilities
			_, err = db.Exec("UPDATE agents SET capabilities = ? WHERE id = ?", agentRegisterCapabilities, agentID)
			if err != nil {
				log.Fatalf("Failed to update capabilities: %v", err)
			}

			fmt.Printf("✓ Updated capabilities for %s:\n", agentID)
			fmt.Printf("  %s\n", agentRegisterCapabilities)

			// Log the event
			logEvent(db, "agent_capabilities_updated", agentID, "", fmt.Sprintf("capabilities='%s'", agentRegisterCapabilities))
		},
	}
	agentRegisterCmd.Flags().StringVarP(&agentRegisterCapabilities, "capabilities", "c", "", "Comma-separated list of capabilities (required)")
	agentRegisterCmd.MarkFlagRequired("capabilities")

	// AGENT FIND - Find agents by capability
	var agentFindCmd = &cobra.Command{
		Use:   "find --capability browser",
		Short: "Find agents with specific capability",
		Long: `Search for agents that have a specific capability.
Useful for finding which agents can handle particular types of tasks.

Examples:
  flip agent find --capability browser
  flip agent find --capability sql
  flip agent find --capability test`,
		Run: func(cmd *cobra.Command, args []string) {
			db := initDB()
			defer db.Close()

			capability, _ := cmd.Flags().GetString("capability")
			if capability == "" {
				log.Fatal("--capability flag is required")
			}

			// Query agents with matching capability
			rows, err := db.Query(`
				SELECT id, name, status, COALESCE(capabilities, ''), COALESCE(task, '')
				FROM agents
				WHERE capabilities LIKE '%' || ? || '%'
				ORDER BY status DESC, id ASC`, capability)
			if err != nil {
				log.Fatalf("Database error: %v", err)
			}
			defer rows.Close()

			fmt.Printf("🔍 Agents with capability: %s\n", capability)
			fmt.Println(strings.Repeat("=", 80))
			fmt.Printf("%-15s %-20s %-10s %-20s %s\n", "AGENT ID", "NAME", "STATUS", "CURRENT TASK", "ALL CAPABILITIES")
			fmt.Println(strings.Repeat("-", 80))

			count := 0
			for rows.Next() {
				var id, name, status, capabilities, task string
				rows.Scan(&id, &name, &status, &capabilities, &task)

				taskDisplay := "-"
				if task != "" {
					taskDisplay = task
				}

				fmt.Printf("%-15s %-20s %-10s %-20s %s\n", id, name, status, taskDisplay, capabilities)
				count++
			}

			if count == 0 {
				fmt.Printf("\nNo agents found with capability: %s\n", capability)
			} else {
				fmt.Printf("\nFound %d agent(s) with capability: %s\n", count, capability)
			}
		},
	}
	agentFindCmd.Flags().String("capability", "", "Capability to search for (required)")
	agentFindCmd.MarkFlagRequired("capability")

	agentCmd.AddCommand(agentRegisterCmd, agentFindCmd)

	// ============================================================================
	// WebSocket Server Command
	// ============================================================================
	var wsPort string
	var wsCmd = &cobra.Command{
		Use:   "ws",
		Short: "Start WebSocket server for real-time agent communication",
		Long: `Start a WebSocket server that enables real-time bidirectional communication
between FLIP and connected agents (like Antigravity).

Features:
- Real-time message delivery (no polling)
- Automatic signal push to connected clients
- Heartbeat monitoring
- Connection state tracking
- Offline message queuing

Connect from Antigravity or any WebSocket client:
  ws://localhost:8091/ws?agent=antigravity`,
		Run: func(cmd *cobra.Command, args []string) {
			db := initDB()
			// Note: don't defer db.Close() - server runs indefinitely

			// Create and start the hub
			hub := NewWSHub(db)
			globalWSHub = hub
			go hub.Run()

			// HTTP endpoints
			http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
				handleWSConnection(hub, w, r)
			})

			// Status endpoint (REST)
			http.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				status := map[string]interface{}{
					"version":          FLIP_VERSION,
					"connected_agents": hub.GetConnectedAgents(),
					"uptime":           time.Now().Format(time.RFC3339),
				}
				json.NewEncoder(w).Encode(status)
			})

			// Send message endpoint (REST -> WS)
			// Accepts flexible input: {"to": "agent", "message": "text"} or {"to": "agent", "payload": {...}}
			http.HandleFunc("/send", func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "POST" {
					http.Error(w, "POST required", http.StatusMethodNotAllowed)
					return
				}

				// Parse as generic map first to handle flexible input
				var raw map[string]interface{}
				if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
					http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
					return
				}

				// Extract "to" field (required)
				to, ok := raw["to"].(string)
				if !ok || to == "" {
					http.Error(w, "to field required (string)", http.StatusBadRequest)
					return
				}

				// Build WSMessage from flexible input
				msg := WSMessage{
					To:        to,
					ID:        fmt.Sprintf("REST-%d", time.Now().UnixNano()%1000000),
					Timestamp: time.Now(),
				}

				// Handle "from" field
				if from, ok := raw["from"].(string); ok {
					msg.From = from
				}

				// Handle "type" field
				if typ, ok := raw["type"].(string); ok {
					msg.Type = typ
				} else {
					msg.Type = "message"
				}

				// Handle "priority" field
				if pri, ok := raw["priority"].(string); ok {
					msg.Priority = pri
				} else {
					msg.Priority = "normal"
				}

				// Handle payload/message - accept either field name
				// Priority: payload > message > (empty string)
				if payload, ok := raw["payload"]; ok && payload != nil {
					msg.Payload = payload
				} else if message, ok := raw["message"]; ok && message != nil {
					msg.Payload = message
				} else {
					msg.Payload = ""
				}

				// Log for debugging
				log.Printf("[SEND] From=%s To=%s Type=%s Payload=%v", msg.From, msg.To, msg.Type, msg.Payload)

				hub.Direct <- &msg

				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]interface{}{
					"status":  "sent",
					"id":      msg.ID,
					"to":      msg.To,
					"payload": msg.Payload,
				})
			})

			// Inbox endpoint (REST) - Get pending signals for an agent
			http.HandleFunc("/inbox/", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Access-Control-Allow-Origin", "*") // Allow browser access

				// Extract agent ID from path: /inbox/antigravity
				agentID := strings.TrimPrefix(r.URL.Path, "/inbox/")
				if agentID == "" {
					http.Error(w, "agent ID required: /inbox/<agent-id>", http.StatusBadRequest)
					return
				}

				// Get pending tasks
				tasks := []map[string]interface{}{}
				taskRows, err := db.Query(`
					SELECT id, title, description, priority, created_at
					FROM tasks WHERE assignee = ? AND status = 'todo'
					ORDER BY priority DESC, created_at DESC
				`, agentID)
				if err == nil {
					defer taskRows.Close()
					for taskRows.Next() {
						var id, title, desc, created string
						var priority int
						taskRows.Scan(&id, &title, &desc, &priority, &created)
						tasks = append(tasks, map[string]interface{}{
							"id":          id,
							"title":       title,
							"description": desc,
							"priority":    priority,
							"created":     created,
						})
					}
				}

				// Get unread signals
				signals := []map[string]interface{}{}
				sigRows, err := db.Query(`
					SELECT id, from_agent, msg, priority, signal_type, subject, created_at
					FROM signals WHERE to_agent = ? AND read = 0
					ORDER BY created_at DESC LIMIT 20
				`, agentID)
				if err == nil {
					defer sigRows.Close()
					for sigRows.Next() {
						var id, from, msg, priority, sigType, subject, created string
						sigRows.Scan(&id, &from, &msg, &priority, &sigType, &subject, &created)
						signals = append(signals, map[string]interface{}{
							"id":       id,
							"from":     from,
							"message":  msg,
							"priority": priority,
							"type":     sigType,
							"subject":  subject,
							"created":  created,
						})
					}
				}

				json.NewEncoder(w).Encode(map[string]interface{}{
					"agent":   agentID,
					"tasks":   tasks,
					"signals": signals,
				})
			})

			// Poll endpoint (REST) - Simple polling without long-wait (returns immediately)
			// Usage: GET /poll/<agent-id>
			http.HandleFunc("/poll/", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Access-Control-Allow-Origin", "*")

				agentID := strings.TrimPrefix(r.URL.Path, "/poll/")
				if agentID == "" {
					http.Error(w, "agent ID required: /poll/<agent-id>", http.StatusBadRequest)
					return
				}

				// Get unread signals (returns immediately)
				signals := []map[string]interface{}{}
				rows, err := db.Query(`
					SELECT id, from_agent, msg, priority, signal_type, subject, created_at
					FROM signals WHERE to_agent = ? AND read = 0
					ORDER BY created_at ASC LIMIT 10
				`, agentID)
				if err == nil {
					defer rows.Close()
					for rows.Next() {
						var id, from, msg, priority, sigType, subject, created string
						rows.Scan(&id, &from, &msg, &priority, &sigType, &subject, &created)
						signals = append(signals, map[string]interface{}{
							"id":       id,
							"from":     from,
							"message":  msg,
							"priority": priority,
							"type":     sigType,
							"subject":  subject,
							"created":  created,
						})
					}
				}

				json.NewEncoder(w).Encode(map[string]interface{}{
					"agent":    agentID,
					"messages": signals,
					"count":    len(signals),
				})
			})

			// Ack endpoint (REST) - Mark message(s) as read
			// Usage: POST /ack {"ids": ["msg1", "msg2"]} or {"id": "msg1"}
			http.HandleFunc("/ack", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Access-Control-Allow-Origin", "*")

				if r.Method != "POST" {
					http.Error(w, "POST required", http.StatusMethodNotAllowed)
					return
				}

				var req map[string]interface{}
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					http.Error(w, "Invalid JSON", http.StatusBadRequest)
					return
				}

				var ids []string
				if id, ok := req["id"].(string); ok && id != "" {
					ids = append(ids, id)
				}
				if idsArr, ok := req["ids"].([]interface{}); ok {
					for _, v := range idsArr {
						if s, ok := v.(string); ok {
							ids = append(ids, s)
						}
					}
				}

				if len(ids) == 0 {
					http.Error(w, "id or ids required", http.StatusBadRequest)
					return
				}

				acked := 0
				for _, id := range ids {
					result, err := db.Exec(`UPDATE signals SET read = 1 WHERE id = ?`, id)
					if err == nil {
						if n, _ := result.RowsAffected(); n > 0 {
							acked++
						}
					}
				}

				json.NewEncoder(w).Encode(map[string]interface{}{
					"status":    "ok",
					"acked":     acked,
					"requested": len(ids),
				})
			})

			// Respond endpoint (REST) - Submit response to a task
			http.HandleFunc("/respond", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Access-Control-Allow-Origin", "*")

				if r.Method == "OPTIONS" {
					w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
					w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
					return
				}

				if r.Method != "POST" {
					http.Error(w, "POST required", http.StatusMethodNotAllowed)
					return
				}

				var req struct {
					Agent    string `json:"agent"`
					TaskID   string `json:"task_id"`
					Response string `json:"response"`
				}
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}

				// Mark task done
				db.Exec(`UPDATE tasks SET status = 'done', updated_at = datetime('now') WHERE id = ?`, req.TaskID)

				// Mark related signals as read
				db.Exec(`UPDATE signals SET read = 1 WHERE to_agent = ? AND subject = ?`, req.Agent, req.TaskID)

				// Store response as signal to coordinator
				sigID := fmt.Sprintf("RESP-%d", time.Now().UnixNano()%1000000)
				db.Exec(`
					INSERT INTO signals (id, from_agent, to_agent, msg, priority, signal_type, subject, created_at)
					VALUES (?, ?, 'coordinator', ?, 'normal', 'response', ?, datetime('now'))
				`, sigID, req.Agent, req.Response, req.TaskID)

				json.NewEncoder(w).Encode(map[string]string{"status": "ok", "signal_id": sigID})
			})

			// RAG Search endpoint (GET)
			http.HandleFunc("/metrics/costs", GetMetricsCostsEndpoint(db))

		http.HandleFunc("/rag/search", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Access-Control-Allow-Origin", "*")

				if r.Method == "OPTIONS" {
					w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
					w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
					return
				}

				if r.Method != "GET" {
					http.Error(w, "GET required", http.StatusMethodNotAllowed)
					return
				}

				// Parse query parameters
				query := r.URL.Query().Get("q")
				if query == "" {
					http.Error(w, "query parameter 'q' is required", http.StatusBadRequest)
					return
				}

				topK := 5
				if topKStr := r.URL.Query().Get("top_k"); topKStr != "" {
					fmt.Sscanf(topKStr, "%d", &topK)
				}

				collection := r.URL.Query().Get("collection")
				if collection == "" {
					collection = "rag_documents"
				}

				hybrid := r.URL.Query().Get("hybrid") == "true"
				keywordWeight := float32(0.3)
				if kwStr := r.URL.Query().Get("keyword_weight"); kwStr != "" {
					fmt.Sscanf(kwStr, "%f", &keywordWeight)
				}

				// Get OpenAI key from environment
				apiKey := os.Getenv("OPENAI_API_KEY")
				if apiKey == "" {
					http.Error(w, "OPENAI_API_KEY environment variable not set", http.StatusInternalServerError)
					return
				}

				// Perform search
				results, err := HandleRAGSearchRequest(query, topK, collection, hybrid, keywordWeight, apiKey)
				if err != nil {
					http.Error(w, fmt.Sprintf("Search failed: %v", err), http.StatusInternalServerError)
					return
				}

				// Return results as JSON
				response := map[string]interface{}{
					"query":   query,
					"results": results,
					"count":   len(results),
				}
				json.NewEncoder(w).Encode(response)
			})

			fmt.Println("🔌 FLIP WebSocket Server v" + FLIP_VERSION)
			fmt.Println("============================================================")
			fmt.Printf("WebSocket endpoint: ws://localhost:%s/ws?agent=<agent-id>\n", wsPort)
			fmt.Printf("Status endpoint:    http://localhost:%s/status\n", wsPort)
			fmt.Printf("Send endpoint:      http://localhost:%s/send (POST)\n", wsPort)
			fmt.Println("============================================================")
			fmt.Println("")
			fmt.Println("📋 Connection Instructions for Antigravity:")
			fmt.Println("   1. Open browser console or WebSocket client")
			fmt.Printf("   2. Connect to: ws://localhost:%s/ws?agent=antigravity\n", wsPort)
			fmt.Println("   3. Messages will be pushed automatically")
			fmt.Println("")
			fmt.Println("📨 Message Types:")
			fmt.Println("   → welcome        - Connection confirmed")
			fmt.Println("   → signals        - New signals/tasks")
			fmt.Println("   → agent_status   - Agent connect/disconnect")
			fmt.Println("   → heartbeat_ack  - Heartbeat response")
			fmt.Println("")
			fmt.Println("📤 To send messages (from client):")
			fmt.Println(`   {"type":"heartbeat"}`)
			fmt.Println(`   {"type":"status_update","payload":{"status":"busy","task":"TASK-123"}}`)
			fmt.Println(`   {"type":"response","to":"coordinator","payload":{"message":"Done!"}}`)
			fmt.Println("")
			fmt.Println("Press Ctrl+C to stop server...")
			fmt.Println("")

			if err := http.ListenAndServe(":"+wsPort, nil); err != nil {
				log.Fatalf("WebSocket server error: %v", err)
			}
		},
	}
	wsCmd.Flags().StringVarP(&wsPort, "port", "p", "8091", "Port for WebSocket server")

	// WebSocket status subcommand
	var wsStatusCmd = &cobra.Command{
		Use:   "status",
		Short: "Check WebSocket server status",
		Run: func(cmd *cobra.Command, args []string) {
			resp, err := http.Get(fmt.Sprintf("http://localhost:%s/status", wsPort))
			if err != nil {
				fmt.Println("❌ WebSocket server not running")
				fmt.Printf("   Start with: ./flip ws -p %s\n", wsPort)
				return
			}
			defer resp.Body.Close()

			var status map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&status)

			fmt.Println("🔌 WebSocket Server Status")
			fmt.Println("============================================================")
			fmt.Printf("Version: %v\n", status["version"])
			fmt.Printf("Uptime since: %v\n", status["uptime"])
			fmt.Println("")
			fmt.Println("Connected Agents:")
			if agents, ok := status["connected_agents"].([]interface{}); ok {
				if len(agents) == 0 {
					fmt.Println("   (none)")
				}
				for _, a := range agents {
					fmt.Printf("   ✅ %v\n", a)
				}
			}
		},
	}
	wsStatusCmd.Flags().StringVarP(&wsPort, "port", "p", "8091", "Port to check")

	// WebSocket send subcommand (CLI -> WS)
	var wsSendCmd = &cobra.Command{
		Use:   "send <agent> <message>",
		Short: "Send a message to a connected agent",
		Args:  cobra.MinimumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			agent := args[0]
			message := strings.Join(args[1:], " ")
			msgType, _ := cmd.Flags().GetString("type")
			priority, _ := cmd.Flags().GetString("priority")

			payload := map[string]interface{}{
				"type":     msgType,
				"to":       agent,
				"payload":  map[string]string{"message": message},
				"priority": priority,
			}

			body, _ := json.Marshal(payload)
			resp, err := http.Post(
				fmt.Sprintf("http://localhost:%s/send", wsPort),
				"application/json",
				strings.NewReader(string(body)),
			)
			if err != nil {
				fmt.Println("❌ Failed to send (server not running?)")
				return
			}
			defer resp.Body.Close()

			var result map[string]string
			json.NewDecoder(resp.Body).Decode(&result)
			fmt.Printf("✅ Message sent to %s (ID: %s)\n", agent, result["id"])
		},
	}
	wsSendCmd.Flags().StringVarP(&wsPort, "port", "p", "8091", "WebSocket server port")
	wsSendCmd.Flags().StringP("type", "t", "signal", "Message type")
	wsSendCmd.Flags().String("priority", "normal", "Priority: critical, high, normal, low")

	wsCmd.AddCommand(wsStatusCmd, wsSendCmd)

	// Distributed FLIP command
	distributedCmd := &cobra.Command{
		Use:   "distributed",
		Short: "Manage distributed FLIP multi-node coordination",
		Long:  "Initialize PKI, start coordinator/worker nodes, and manage certificates for remote task delegation",
		Run: func(cmd *cobra.Command, args []string) {
			handleDistributedCommands(args)
		},
	}

	// Register all commands
	rootCmd.AddCommand(statusCmd, registerCmd, unregisterCmd, updateCmd, serveCmd, taskCmd, queueCmd, auditCmd, pruneCmd, signalCmd, summaryCmd, workloadCmd, healthCmd, alertsCmd, exportCmd, importCmd, configCmd, statsCmd, logsCmd, versionCmd, watchCmd, autoAssignCmd, gitCmd, watchdogCmd, handoffCmd, spawnCmd, orchestrateCmd, pipelineCmd, antigravityCmd, agentCmd, wsCmd, GetRAGCommand(), distributedCmd, GetSchedulerCommand(), GetKnowledgeCommand())
	rootCmd.Execute()
}
