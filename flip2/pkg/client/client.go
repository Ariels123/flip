package client

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Client handles communication with the FLIP2/PocketBase daemon
type Client struct {
	BaseURL   string
	AgentID   string
	APIKey    string
	AuthToken string

	clientID     string
	events       chan *SignalEvent
	tasks        chan *TaskEvent
	stop         chan struct{}
	httpClient   *http.Client
	logger       *slog.Logger
	wg           sync.WaitGroup // Tracks connection goroutine lifecycle
	lastEventID  string         // Distributed Execution V2: Track last SSE event ID for resiliency
	eventIDMutex sync.RWMutex   // Protects lastEventID
}

type SignalEvent struct {
	Action string       `json:"action"`
	Record SignalRecord `json:"record"`
}

type SignalRecord struct {
	ID        string `json:"id"`
	FromAgent string `json:"from_agent"`
	ToAgent   string `json:"to_agent"`
	Content   string `json:"content"`
	Type      string `json:"signal_type"`
	Read      bool   `json:"read"`
}

type TaskEvent struct {
	Action string     `json:"action"`
	Record TaskRecord `json:"record"`
}

type TaskRecord struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Assignee    string `json:"assignee"`
	Status      string `json:"status"`
}

type sseEvent struct {
	ID    string
	Event string
	Data  []byte
}

func New(baseURL, agentID, apiKey, authToken string, logger *slog.Logger) *Client {
	if logger == nil {
		logger = slog.Default()
	}
	return &Client{
		BaseURL:   baseURL,
		AgentID:   agentID,
		APIKey:    apiKey,
		AuthToken: authToken,
		events:    make(chan *SignalEvent, 10),
		tasks:     make(chan *TaskEvent, 10),
		stop:      make(chan struct{}),
		httpClient: &http.Client{
			Transport: &http.Transport{
				MaxIdleConns:      10,
				IdleConnTimeout:   30 * time.Second,
				DisableKeepAlives: true, // Force new connection to avoid reuse issues
			},
			Timeout: 0, // No timeout for SSE
		},
		logger: logger,
	}
}

func (c *Client) Signals() <-chan *SignalEvent {
	return c.events
}

func (c *Client) Tasks() <-chan *TaskEvent {
	return c.tasks
}

func (c *Client) Connect() error {
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		for {
			select {
			case <-c.stop:
				c.logger.Info("Connection goroutine stopping")
				return
			default:
				if err := c.connectOnce(); err != nil {
					c.logger.Error("Connection error", "error", err)
					time.Sleep(3 * time.Second)
				}
			}
		}
	}()
	return nil
}

func (c *Client) setAuthHeaders(req *http.Request) {
	if c.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.AuthToken)
	} else if c.APIKey != "" {
		req.Header.Set("X-API-Key", c.APIKey)
	}
}

func (c *Client) connectOnce() error {
	req, _ := http.NewRequest("GET", c.BaseURL+"/api/realtime", nil)
	req.Header.Set("Accept", "text/event-stream")
	c.setAuthHeaders(req)

	// Distributed Execution V2: Include Last-Event-ID header for resiliency
	// This allows the server to replay missed events after a reconnection
	c.eventIDMutex.RLock()
	lastID := c.lastEventID
	c.eventIDMutex.RUnlock()
	if lastID != "" {
		req.Header.Set("Last-Event-ID", lastID)
		c.logger.Debug("Reconnecting with Last-Event-ID", "id", lastID)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("sse connection failed: %s", resp.Status)
	}

	reader := bufio.NewReader(resp.Body)
	var currentEvent sseEvent

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		line = strings.TrimSpace(line)

		if line == "" {
			// End of event
			if len(currentEvent.Data) > 0 {
				// Distributed Execution V2: Store the event ID for resiliency
				if currentEvent.ID != "" {
					c.eventIDMutex.Lock()
					c.lastEventID = currentEvent.ID
					c.eventIDMutex.Unlock()
				}
				c.handleEvent(currentEvent)
			}
			currentEvent = sseEvent{}
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		switch key {
		case "id":
			currentEvent.ID = val
		case "event":
			currentEvent.Event = val
		case "data":
			currentEvent.Data = []byte(val)
		}
	}
}

// GetLastEventID returns the last processed SSE event ID (for diagnostics)
func (c *Client) GetLastEventID() string {
	c.eventIDMutex.RLock()
	defer c.eventIDMutex.RUnlock()
	return c.lastEventID
}

func (c *Client) handleEvent(evt sseEvent) {
	if evt.Event == "PB_CONNECT" {
		var data map[string]interface{}
		json.Unmarshal(evt.Data, &data)
		if id, ok := data["clientId"].(string); ok {
			c.clientID = id
			c.logger.Info("Realtime connected", "clientId", id)
			// Subscribe
			c.subscribeMulti([]string{"signals", "tasks"})
		}
		return
	}
	
	if evt.Event == "create" || evt.Event == "update" {
		var wrapper struct {
			Action     string          `json:"action"`
			Collection string          `json:"collection"`
			Record     json.RawMessage `json:"record"`
		}
		if err := json.Unmarshal(evt.Data, &wrapper); err != nil {
			return
		}
		
		if wrapper.Collection == "signals" {
			var rec SignalRecord
			json.Unmarshal(wrapper.Record, &rec)
			if rec.ToAgent == c.AgentID && !rec.Read {
				c.events <- &SignalEvent{Action: wrapper.Action, Record: rec}
			}
		} else if wrapper.Collection == "tasks" {
			var rec TaskRecord
			json.Unmarshal(wrapper.Record, &rec)
			if rec.Assignee == c.AgentID || (rec.Assignee == "" && rec.Status == "todo") {
				c.tasks <- &TaskEvent{Action: wrapper.Action, Record: rec}
			}
		}
	}
}

func (c *Client) subscribeMulti(collections []string) error {
	if c.clientID == "" {
		return fmt.Errorf("no client id")
	}
	jsonBody, err := json.Marshal(map[string]interface{}{
		"clientId":      c.clientID,
		"subscriptions": collections,
	})
	if err != nil {
		return err
	}

	req, _ := http.NewRequest("POST", c.BaseURL+"/api/realtime", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	c.setAuthHeaders(req)
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (c *Client) SendSignal(to, sigType, content string) error {
	data := map[string]interface{}{
		"signal_id": fmt.Sprintf("SIG-%d", time.Now().UnixNano()),
		"from_agent": c.AgentID,
		"to_agent":   to,
		"signal_type": sigType,
		"content":    content,
		"read":       false,
	}
	
	jsonData, _ := json.Marshal(data)
	req, _ := http.NewRequest("POST", c.BaseURL+"/api/collections/signals/records", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	c.setAuthHeaders(req)
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode >= 400 {
		return fmt.Errorf("api error: %s", resp.Status)
	}
	return nil
}

func (c *Client) SignalTask(taskID, signal string) error {
	data := map[string]string{
		"signal": signal,
	}
	jsonData, _ := json.Marshal(data)

	req, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/tasks/%s/signal", c.BaseURL, taskID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	c.setAuthHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("api error: %s", resp.Status)
	}
	return nil
}

func (c *Client) MarkRead(signalID string) error {
	data := map[string]interface{}{"read": true}
	jsonData, _ := json.Marshal(data)

	req, _ := http.NewRequest("PATCH", fmt.Sprintf("%s/api/collections/signals/records/%s", c.BaseURL, signalID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	c.setAuthHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// UpdateSignal updates specific fields of a signal record
// Used by commmonitor for typo correction and other signal modifications
func (c *Client) UpdateSignal(signalID string, data map[string]interface{}) error {
	jsonData, _ := json.Marshal(data)

	req, _ := http.NewRequest("PATCH", fmt.Sprintf("%s/api/collections/signals/records/%s", c.BaseURL, signalID), bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	c.setAuthHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("api error: %s", resp.Status)
	}
	return nil
}

// ListTasks lists tasks with optional filter
func (c *Client) ListTasks(filter string) ([]TaskRecord, error) {
	url := fmt.Sprintf("%s/api/collections/tasks/records?filter=%s", c.BaseURL, url.QueryEscape(filter))
	req, _ := http.NewRequest("GET", url, nil)
	c.setAuthHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("api error: %s", resp.Status)
	}

	var result struct {
		Items []TaskRecord `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Items, nil
}

// CountTasks returns the count of tasks matching the given filter.
func (c *Client) CountTasks(filter string) (int64, error) {
	url := fmt.Sprintf("%s/api/collections/tasks/records?filter=%s&perPage=1", c.BaseURL, url.QueryEscape(filter))
	req, _ := http.NewRequest("GET", url, nil)
	c.setAuthHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("API error: %s %s", resp.Status, string(body))
	}

	var result struct {
		TotalItems int64 `json:"totalItems"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.TotalItems, nil
}

// GetTask fetches a single task by ID
func (c *Client) GetTask(taskID string) (map[string]interface{}, error) {
	req, _ := http.NewRequest("GET",
		fmt.Sprintf("%s/api/collections/tasks/records/%s", c.BaseURL, taskID),
		nil)
	c.setAuthHeaders(req)

	c.logger.Debug("Fetching task", "task_id", taskID, "url", req.URL.String())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch task: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	c.logger.Debug("GetTask response", "status", resp.Status, "body", string(bodyBytes))

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch task failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var task map[string]interface{}
	if err := json.NewDecoder(bytes.NewReader(bodyBytes)).Decode(&task); err != nil {
		return nil, fmt.Errorf("failed to decode task: %w", err)
	}

	return task, nil
}

// Helper to log map keys
func getKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// GetAgent fetches a single agent by agent_id (name)
func (c *Client) GetAgent(agentID string) (map[string]interface{}, error) {
	// Query by agent_id field, not record ID
	filter := fmt.Sprintf("agent_id='%s'", agentID)
	req, _ := http.NewRequest("GET",
		fmt.Sprintf("%s/api/collections/agents/records?filter=%s", c.BaseURL, url.QueryEscape(filter)),
		nil)
	c.setAuthHeaders(req)

	c.logger.Debug("Fetching agent", "agent_id", agentID, "url", req.URL.String())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch agent: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	c.logger.Debug("GetAgent response", "status", resp.Status, "body", string(bodyBytes))

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch agent failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result struct {
		Items []map[string]interface{} `json:"items"`
	}
	if err := json.NewDecoder(bytes.NewReader(bodyBytes)).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode agent: %w", err)
	}

	if len(result.Items) == 0 {
		return nil, fmt.Errorf("agent not found: %s", agentID)
	}

	return result.Items[0], nil
}


// ClaimTaskAtomic attempts to atomically claim a task using database-level atomic operations.
// This prevents race conditions where multiple agents try to claim the same task.
// Returns true if the claim was successful, false if another agent claimed it first.
//
// P0 Fix #2: Uses the /api/tasks/:id/claim endpoint which performs an atomic UPDATE
// with a WHERE clause at the database level, eliminating the race condition.
//
// Tier 1 Enhancement: Checks role compatibility before claiming. If the task has a required_role,
// the agent must have a matching role to claim it.
func (c *Client) ClaimTaskAtomic(taskID, agentID, agentName string) (bool, error) {
	// Role checking re-enabled after fixing "updated_at" bug
	
	// First, fetch the task to check required_role
	task, err := c.GetTask(taskID)
	if err != nil {
		return false, fmt.Errorf("failed to fetch task for role check: %w", err)
	}

	// If task has a required role, verify agent compatibility
	if requiredRole, ok := task["required_role"].(string); ok && requiredRole != "" {
		// Fetch agent's role
		agent, err := c.GetAgent(agentName)
		if err != nil {
			return false, fmt.Errorf("failed to fetch agent role: %w", err)
		}

		// Check metadata.role first (preferred), then root level role
		var agentRole string
		if metadata, ok := agent["metadata"].(map[string]interface{}); ok {
			if r, ok := metadata["role"].(string); ok {
				agentRole = r
			}
		}
		if agentRole == "" {
			agentRole, _ = agent["role"].(string)
		}

		if agentRole != requiredRole {
			c.logger.Info("Role mismatch, skipping claim",
				"task_id", taskID,
				"required_role", requiredRole,
				"agent_role", agentRole,
				"agent_name", agentName)
			return false, nil // Not an error, just incompatible
		}
	}
	
	// Proceed with atomic claim
	data := map[string]interface{}{
		"agent_id": agentID,
	}
	jsonData, _ := json.Marshal(data)

	req, _ := http.NewRequest("POST",
		fmt.Sprintf("%s/api/tasks/%s/claim", c.BaseURL, taskID),
		bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	c.setAuthHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to claim task: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode == http.StatusOK {
		c.logger.Info("Task claimed successfully", "task_id", taskID, "agent", agentID)
		return true, nil
	}

	if resp.StatusCode == http.StatusConflict {
		// Task was already claimed by another agent
		c.logger.Debug("Task already claimed by another agent", "task_id", taskID)
		return false, nil
	}

	// Other error occurred
	bodyBytes, _ := io.ReadAll(resp.Body)
	return false, fmt.Errorf("claim failed with status %d: %s", resp.StatusCode, string(bodyBytes))
}

// Close stops the SSE connection and cleans up resources
// Blocks until the connection goroutine has exited
func (c *Client) Close() {
	close(c.stop)
	c.wg.Wait() // Wait for connection goroutine to exit
	c.logger.Info("Client closed gracefully")
}
