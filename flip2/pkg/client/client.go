package client

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// Client handles communication with the FLIP2/PocketBase daemon
type Client struct {
	BaseURL   string
	AgentID   string
	APIKey    string
	AuthToken string

	clientID   string
	events     chan *SignalEvent
	tasks      chan *TaskEvent
	stop       chan struct{}
	httpClient *http.Client
	logger     *slog.Logger
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
	go func() {
		for {
			select {
			case <-c.stop:
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

// Close stops the SSE connection and cleans up resources
func (c *Client) Close() {
	close(c.stop)
}
