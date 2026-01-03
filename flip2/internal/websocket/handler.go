package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 512 * 1024
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for now, security via API keys/Auth
	},
}

// Client represents a connected agent/user via WebSocket
type Client struct {
	hub *Hub

	// The websocket connection.
	conn *websocket.Conn

	// Buffered channel of outbound messages.
	send chan []byte

	// Agent ID (if authenticated)
	agentID string

	// Session ID (if associated)
	sessionID string

	// ConnectedAt is when the client connected.
	connectedAt time.Time

	// LastMessageAt is the last time a message was received.
	lastMessageAt time.Time

	// ReconnectionToken is used for session recovery.
	reconnectionToken string
}

// Hub maintains the set of active clients and broadcasts messages to them.
type Hub struct {
	// Registered clients.
	clients map[*Client]bool

	// Inbound messages from the clients.
	broadcast chan []byte

	// Register requests from the clients.
	register chan *Client

	// Unregister requests from clients.
	unregister chan *Client

	// Map of agentID to client for targeted messaging
	agents map[string]*Client

	// Map of sessionID to clients for session-scoped messaging
	sessions map[string]map[*Client]bool

	// Reconnection tokens for session recovery
	reconnectionTokens map[string]*ReconnectionInfo

	mu sync.RWMutex
}

// ReconnectionInfo stores information for reconnection tokens.
type ReconnectionInfo struct {
	AgentID     string
	SessionID   string
	CreatedAt   time.Time
	ExpiresAt   time.Time
	LastStateAt time.Time
}

func NewHub() *Hub {
	return &Hub{
		broadcast:          make(chan []byte),
		register:           make(chan *Client),
		unregister:         make(chan *Client),
		clients:            make(map[*Client]bool),
		agents:             make(map[string]*Client),
		sessions:           make(map[string]map[*Client]bool),
		reconnectionTokens: make(map[string]*ReconnectionInfo),
	}
}

func (h *Hub) Run(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("WebSocket Hub panic recovered", "error", r)
			// Restart hub if possible or log critical
		}
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			if client.agentID != "" {
				h.agents[client.agentID] = client
			}
			// Track session membership
			if client.sessionID != "" {
				if h.sessions[client.sessionID] == nil {
					h.sessions[client.sessionID] = make(map[*Client]bool)
				}
				h.sessions[client.sessionID][client] = true
			}
			h.mu.Unlock()
		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				if client.agentID != "" {
					delete(h.agents, client.agentID)
				}
				// Remove from session tracking
				if client.sessionID != "" {
					if sessionClients, exists := h.sessions[client.sessionID]; exists {
						delete(sessionClients, client)
						if len(sessionClients) == 0 {
							delete(h.sessions, client.sessionID)
						}
					}
				}
				close(client.send)
			}
			h.mu.Unlock()
		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					// If client buffer is full, disconnect them
					go func(c *Client) { h.unregister <- c }(client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// SendToAgent sends a message to a specific agent
func (h *Hub) SendToAgent(agentID string, message []byte) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	client, ok := h.agents[agentID]
	if !ok {
		return false
	}
	select {
	case client.send <- message:
		return true
	default:
		return false
	}
}

// IsAgentConnected checks if an agent has an active WebSocket connection.
func (h *Hub) IsAgentConnected(agentID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, ok := h.agents[agentID]
	return ok
}

// BroadcastToSession sends a message to all agents in a session.
// Returns the number of agents that received the message.
func (h *Hub) BroadcastToSession(sessionID string, message []byte) int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	sessionClients, exists := h.sessions[sessionID]
	if !exists {
		return 0
	}

	sent := 0
	for client := range sessionClients {
		select {
		case client.send <- message:
			sent++
		default:
			// Client buffer full, skip
		}
	}
	return sent
}

// GetConnectedAgents returns a list of all connected agent IDs.
func (h *Hub) GetConnectedAgents() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	agents := make([]string, 0, len(h.agents))
	for agentID := range h.agents {
		agents = append(agents, agentID)
	}
	return agents
}

// GetSessionAgents returns a list of agent IDs connected to a session.
func (h *Hub) GetSessionAgents(sessionID string) []string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	sessionClients, exists := h.sessions[sessionID]
	if !exists {
		return []string{}
	}

	agents := make([]string, 0, len(sessionClients))
	for client := range sessionClients {
		if client.agentID != "" {
			agents = append(agents, client.agentID)
		}
	}
	return agents
}

// AssociateClientWithSession associates a client with a session.
func (h *Hub) AssociateClientWithSession(agentID, sessionID string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	client, ok := h.agents[agentID]
	if !ok {
		return false
	}

	// Remove from old session if any
	if client.sessionID != "" && client.sessionID != sessionID {
		if sessionClients, exists := h.sessions[client.sessionID]; exists {
			delete(sessionClients, client)
			if len(sessionClients) == 0 {
				delete(h.sessions, client.sessionID)
			}
		}
	}

	// Add to new session
	client.sessionID = sessionID
	if h.sessions[sessionID] == nil {
		h.sessions[sessionID] = make(map[*Client]bool)
	}
	h.sessions[sessionID][client] = true
	return true
}

// CreateReconnectionToken creates a token for session recovery.
func (h *Hub) CreateReconnectionToken(agentID, sessionID string, ttl time.Duration) string {
	h.mu.Lock()
	defer h.mu.Unlock()

	token := fmt.Sprintf("recon_%s_%d", agentID, time.Now().UnixNano())
	h.reconnectionTokens[token] = &ReconnectionInfo{
		AgentID:     agentID,
		SessionID:   sessionID,
		CreatedAt:   time.Now(),
		ExpiresAt:   time.Now().Add(ttl),
		LastStateAt: time.Now(),
	}
	return token
}

// ValidateReconnectionToken validates a reconnection token and returns the info.
func (h *Hub) ValidateReconnectionToken(token string) (*ReconnectionInfo, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	info, exists := h.reconnectionTokens[token]
	if !exists {
		return nil, false
	}

	if time.Now().After(info.ExpiresAt) {
		return nil, false
	}

	return info, true
}

// ConsumeReconnectionToken validates and removes a reconnection token.
func (h *Hub) ConsumeReconnectionToken(token string) (*ReconnectionInfo, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	info, exists := h.reconnectionTokens[token]
	if !exists {
		return nil, false
	}

	if time.Now().After(info.ExpiresAt) {
		delete(h.reconnectionTokens, token)
		return nil, false
	}

	delete(h.reconnectionTokens, token)
	return info, true
}

// CleanupExpiredTokens removes expired reconnection tokens.
func (h *Hub) CleanupExpiredTokens() int {
	h.mu.Lock()
	defer h.mu.Unlock()

	now := time.Now()
	cleaned := 0
	for token, info := range h.reconnectionTokens {
		if now.After(info.ExpiresAt) {
			delete(h.reconnectionTokens, token)
			cleaned++
		}
	}
	return cleaned
}

// GetConnectionStats returns connection statistics.
func (h *Hub) GetConnectionStats() map[string]interface{} {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return map[string]interface{}{
		"total_clients":        len(h.clients),
		"connected_agents":     len(h.agents),
		"active_sessions":      len(h.sessions),
		"reconnection_tokens":  len(h.reconnectionTokens),
	}
}

// Handler handles websocket requests
type Handler struct {
	hub    *Hub
	pb     *pocketbase.PocketBase
	logger *slog.Logger
	apiKey string
}

func NewHandler(pb *pocketbase.PocketBase, hub *Hub, logger *slog.Logger, apiKey string) *Handler {
	return &Handler{
		hub:    hub,
		pb:     pb,
		logger: logger.WithGroup("websocket"),
		apiKey: apiKey,
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Authentication
	apiKey := r.Header.Get("X-API-Key")
	if apiKey == "" {
		apiKey = r.URL.Query().Get("api_key")
	}

	if h.apiKey != "" && apiKey != h.apiKey {
		http.Error(w, "Unauthorized: Invalid API Key", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("Upgrade error", "error", err)
		return
	}

	agentID := r.URL.Query().Get("agent_id")
	sessionID := r.URL.Query().Get("session_id")
	reconnToken := r.URL.Query().Get("reconnection_token")

	now := time.Now()
	client := &Client{
		hub:           h.hub,
		conn:          conn,
		send:          make(chan []byte, 256),
		agentID:       agentID,
		sessionID:     sessionID,
		connectedAt:   now,
		lastMessageAt: now,
	}

	// Handle reconnection token if provided
	if reconnToken != "" {
		if info, valid := h.hub.ConsumeReconnectionToken(reconnToken); valid {
			// Restore session association from token
			if client.sessionID == "" {
				client.sessionID = info.SessionID
			}
			client.reconnectionToken = reconnToken

			h.logger.Info("Client reconnected with token",
				"agent_id", agentID,
				"session_id", info.SessionID,
				"token", reconnToken,
			)

			// Send reconnection acknowledgment
			ack := map[string]interface{}{
				"type": "reconnection_ack",
				"payload": map[string]interface{}{
					"session_id":     info.SessionID,
					"last_state_at":  info.LastStateAt,
					"reconnected_at": now,
				},
			}
			ackBytes, _ := json.Marshal(ack)
			client.send <- ackBytes
		} else {
			h.logger.Warn("Invalid or expired reconnection token",
				"agent_id", agentID,
				"token", reconnToken,
			)
		}
	}

	client.hub.register <- client

	// Allow collection of memory referenced by the caller by doing all work in
	// new goroutines.
	go client.writePump()
	go client.readPump(h)
}

func (c *Client) readPump(h *Handler) {
	defer func() {
		if r := recover(); r != nil {
			h.logger.Error("readPump panic recovered", "error", r)
		}
		c.hub.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				h.logger.Error("Read error", "error", err)
			}
			break
		}
		
		// Handle incoming message (e.g., heartbeat, task status update)
		var msg struct {
			Type    string          `json:"type"`
			Payload json.RawMessage `json:"payload"`
		}
		if err := json.Unmarshal(message, &msg); err != nil {
			continue
		}

		switch msg.Type {
		case "heartbeat":
			// Update agent last_seen in PocketBase
			if c.agentID != "" {
				go h.updateAgentLastSeen(c.agentID)
			}
		case "task_update":
			// Handle task update from agent
			// ...
		}
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		if r := recover(); r != nil {
			slog.Error("writePump panic recovered", "error", r)
		}
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel.
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued chat messages to the current websocket message.
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (h *Handler) updateAgentLastSeen(agentID string) {
	record, err := h.pb.FindRecordById("agents", agentID)
	if err != nil {
		// Try by agent_id
		records, err := h.pb.FindRecordsByFilter("agents", fmt.Sprintf("agent_id='%s'", agentID), "", 1, 0)
		if err != nil || len(records) == 0 {
			return
		}
		record = records[0]
	}

	record.Set("last_seen", time.Now())
	record.Set("status", "online")
	if err := h.pb.Save(record); err != nil {
		h.logger.Error("Failed to update agent last_seen via heartbeat", "agent_id", agentID, "error", err)
	}
}

// BroadcastTaskUpdate sends a task update to all connected clients
func (h *Hub) BroadcastTaskUpdate(taskRecord *core.Record) {
	data := map[string]interface{}{
		"type": "task_update",
		"payload": map[string]interface{}{
			"id":     taskRecord.Id,
			"status": taskRecord.GetString("status"),
			"title":  taskRecord.GetString("title"),
		},
	}
	msg, _ := json.Marshal(data)
	h.broadcast <- msg
}
