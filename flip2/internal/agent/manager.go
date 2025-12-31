package agent

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Status represents an agent's current status
type Status string

const (
	StatusOnline  Status = "online"
	StatusOffline Status = "offline"
	StatusBusy    Status = "busy"
	StatusIdle    Status = "idle"
)

// Backend represents the LLM backend type
type Backend string

const (
	BackendClaude      Backend = "claude"
	BackendGemini      Backend = "gemini"
	BackendAntigravity Backend = "antigravity"
	BackendCustom      Backend = "custom"
)

// Capabilities defines what an agent can do
type Capabilities struct {
	CanCode       bool     `json:"can_code"`
	CanResearch   bool     `json:"can_research"`
	CanTest       bool     `json:"can_test"`
	CanBrowse     bool     `json:"can_browse"`
	MaxTokens     int      `json:"max_tokens"`
	Languages     []string `json:"languages,omitempty"`
	Specialties   []string `json:"specialties,omitempty"`
}

// Agent represents a registered agent in the system
type Agent struct {
	ID           string       `json:"agent_id"`
	Status       Status       `json:"status"`
	Backend      Backend      `json:"backend"`
	Capabilities Capabilities `json:"capabilities"`
	LastSeen     time.Time    `json:"last_seen"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`

	// Internal tracking
	RegisteredAt time.Time `json:"registered_at"`
	TaskCount    int       `json:"task_count"`
	ErrorCount   int       `json:"error_count"`
}

// Manager handles agent registration, status tracking, and routing
type Manager struct {
	mu      sync.RWMutex
	agents  map[string]*Agent
	logger  *slog.Logger

	// Heartbeat configuration
	heartbeatInterval time.Duration
	offlineThreshold  time.Duration

	// Callbacks
	onAgentOnline  func(agent *Agent)
	onAgentOffline func(agent *Agent)

	// Shutdown
	done chan struct{}
}

// ManagerConfig configures the agent manager
type ManagerConfig struct {
	HeartbeatInterval time.Duration
	OfflineThreshold  time.Duration
	OnAgentOnline     func(agent *Agent)
	OnAgentOffline    func(agent *Agent)
}

// DefaultManagerConfig returns sensible defaults
func DefaultManagerConfig() ManagerConfig {
	return ManagerConfig{
		HeartbeatInterval: 30 * time.Second,
		OfflineThreshold:  90 * time.Second,
	}
}

// NewManager creates a new agent manager
func NewManager(logger *slog.Logger, cfg ManagerConfig) *Manager {
	if cfg.HeartbeatInterval == 0 {
		cfg.HeartbeatInterval = 30 * time.Second
	}
	if cfg.OfflineThreshold == 0 {
		cfg.OfflineThreshold = 90 * time.Second
	}

	m := &Manager{
		agents:            make(map[string]*Agent),
		logger:            logger,
		heartbeatInterval: cfg.HeartbeatInterval,
		offlineThreshold:  cfg.OfflineThreshold,
		onAgentOnline:     cfg.OnAgentOnline,
		onAgentOffline:    cfg.OnAgentOffline,
		done:              make(chan struct{}),
	}

	// Start background heartbeat checker
	go m.heartbeatChecker()

	return m
}

// Register adds or updates an agent in the registry
func (m *Manager) Register(agent *Agent) error {
	if agent.ID == "" {
		return fmt.Errorf("agent ID cannot be empty")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	existing, exists := m.agents[agent.ID]

	agent.LastSeen = time.Now()
	if !exists {
		agent.RegisteredAt = time.Now()
		agent.Status = StatusOnline
		m.agents[agent.ID] = agent
		m.logger.Info("Agent registered",
			"agent_id", agent.ID,
			"backend", agent.Backend,
			"capabilities", agent.Capabilities)

		if m.onAgentOnline != nil {
			go m.onAgentOnline(agent)
		}
	} else {
		// Update existing agent
		agent.RegisteredAt = existing.RegisteredAt
		agent.TaskCount = existing.TaskCount
		agent.ErrorCount = existing.ErrorCount

		wasOffline := existing.Status == StatusOffline
		if wasOffline && agent.Status != StatusOffline {
			m.logger.Info("Agent came back online", "agent_id", agent.ID)
			if m.onAgentOnline != nil {
				go m.onAgentOnline(agent)
			}
		}
		m.agents[agent.ID] = agent
	}

	return nil
}

// Unregister removes an agent from the registry
func (m *Manager) Unregister(agentID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	agent, exists := m.agents[agentID]
	if !exists {
		return fmt.Errorf("agent not found: %s", agentID)
	}

	delete(m.agents, agentID)
	m.logger.Info("Agent unregistered", "agent_id", agentID)

	if m.onAgentOffline != nil {
		go m.onAgentOffline(agent)
	}

	return nil
}

// Heartbeat updates an agent's last_seen timestamp
func (m *Manager) Heartbeat(agentID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	agent, exists := m.agents[agentID]
	if !exists {
		return fmt.Errorf("agent not found: %s", agentID)
	}

	wasOffline := agent.Status == StatusOffline
	agent.LastSeen = time.Now()

	// Bring back online if it was offline
	if wasOffline {
		agent.Status = StatusOnline
		m.logger.Info("Agent heartbeat restored", "agent_id", agentID)
		if m.onAgentOnline != nil {
			go m.onAgentOnline(agent)
		}
	}

	return nil
}

// SetStatus updates an agent's status
func (m *Manager) SetStatus(agentID string, status Status) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	agent, exists := m.agents[agentID]
	if !exists {
		return fmt.Errorf("agent not found: %s", agentID)
	}

	oldStatus := agent.Status
	agent.Status = status
	agent.LastSeen = time.Now()

	m.logger.Debug("Agent status changed",
		"agent_id", agentID,
		"old_status", oldStatus,
		"new_status", status)

	return nil
}

// Get returns an agent by ID
func (m *Manager) Get(agentID string) (*Agent, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	agent, exists := m.agents[agentID]
	if !exists {
		return nil, false
	}

	// Return a copy to prevent external modification
	copy := *agent
	return &copy, true
}

// List returns all registered agents
func (m *Manager) List() []*Agent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	agents := make([]*Agent, 0, len(m.agents))
	for _, agent := range m.agents {
		copy := *agent
		agents = append(agents, &copy)
	}
	return agents
}

// ListByStatus returns agents with a specific status
func (m *Manager) ListByStatus(status Status) []*Agent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	agents := make([]*Agent, 0)
	for _, agent := range m.agents {
		if agent.Status == status {
			copy := *agent
			agents = append(agents, &copy)
		}
	}
	return agents
}

// ListByBackend returns agents with a specific backend
func (m *Manager) ListByBackend(backend Backend) []*Agent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	agents := make([]*Agent, 0)
	for _, agent := range m.agents {
		if agent.Backend == backend {
			copy := *agent
			agents = append(agents, &copy)
		}
	}
	return agents
}

// ListAvailable returns agents that are online and idle
func (m *Manager) ListAvailable() []*Agent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	agents := make([]*Agent, 0)
	for _, agent := range m.agents {
		if agent.Status == StatusOnline || agent.Status == StatusIdle {
			copy := *agent
			agents = append(agents, &copy)
		}
	}
	return agents
}

// FindByCapability returns agents that have a specific capability
func (m *Manager) FindByCapability(capability string) []*Agent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	agents := make([]*Agent, 0)
	for _, agent := range m.agents {
		if hasCapability(agent, capability) {
			copy := *agent
			agents = append(agents, &copy)
		}
	}
	return agents
}

// SelectBestAgent finds the best available agent for a task
func (m *Manager) SelectBestAgent(requirements *Capabilities, preferredBackend Backend) (*Agent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var best *Agent
	var bestScore int

	for _, agent := range m.agents {
		// Skip unavailable agents
		if agent.Status != StatusOnline && agent.Status != StatusIdle {
			continue
		}

		score := m.scoreAgent(agent, requirements, preferredBackend)
		if score > bestScore {
			bestScore = score
			best = agent
		}
	}

	if best == nil {
		return nil, fmt.Errorf("no suitable agent found")
	}

	copy := *best
	return &copy, nil
}

// scoreAgent calculates a match score for an agent
func (m *Manager) scoreAgent(agent *Agent, req *Capabilities, preferred Backend) int {
	score := 0

	// Prefer the requested backend
	if agent.Backend == preferred {
		score += 100
	}

	// Status preference (idle > online)
	if agent.Status == StatusIdle {
		score += 50
	} else if agent.Status == StatusOnline {
		score += 25
	}

	if req == nil {
		return score
	}

	// Capability matching
	if req.CanCode && agent.Capabilities.CanCode {
		score += 20
	}
	if req.CanResearch && agent.Capabilities.CanResearch {
		score += 20
	}
	if req.CanTest && agent.Capabilities.CanTest {
		score += 20
	}
	if req.CanBrowse && agent.Capabilities.CanBrowse {
		score += 20
	}

	// Token capacity
	if req.MaxTokens > 0 && agent.Capabilities.MaxTokens >= req.MaxTokens {
		score += 10
	}

	// Prefer agents with fewer errors
	score -= agent.ErrorCount * 5

	return score
}

// IncrementTaskCount increments the task counter for an agent
func (m *Manager) IncrementTaskCount(agentID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if agent, exists := m.agents[agentID]; exists {
		agent.TaskCount++
	}
}

// IncrementErrorCount increments the error counter for an agent
func (m *Manager) IncrementErrorCount(agentID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if agent, exists := m.agents[agentID]; exists {
		agent.ErrorCount++
	}
}

// heartbeatChecker periodically marks agents as offline if no heartbeat
func (m *Manager) heartbeatChecker() {
	ticker := time.NewTicker(m.heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.done:
			return
		case <-ticker.C:
			m.checkHeartbeats()
		}
	}
}

// checkHeartbeats marks agents as offline if they haven't sent a heartbeat
func (m *Manager) checkHeartbeats() {
	m.mu.Lock()
	defer m.mu.Unlock()

	threshold := time.Now().Add(-m.offlineThreshold)

	for _, agent := range m.agents {
		if agent.Status != StatusOffline && agent.LastSeen.Before(threshold) {
			agent.Status = StatusOffline
			m.logger.Warn("Agent marked offline (no heartbeat)",
				"agent_id", agent.ID,
				"last_seen", agent.LastSeen)

			if m.onAgentOffline != nil {
				go m.onAgentOffline(agent)
			}
		}
	}
}

// Stop shuts down the manager
func (m *Manager) Stop() {
	close(m.done)
}

// Count returns the total number of registered agents
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.agents)
}

// Stats returns agent statistics
func (m *Manager) Stats() map[string]int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := map[string]int{
		"total":   len(m.agents),
		"online":  0,
		"offline": 0,
		"busy":    0,
		"idle":    0,
	}

	for _, agent := range m.agents {
		switch agent.Status {
		case StatusOnline:
			stats["online"]++
		case StatusOffline:
			stats["offline"]++
		case StatusBusy:
			stats["busy"]++
		case StatusIdle:
			stats["idle"]++
		}
	}

	return stats
}

// ToJSON serializes the agent registry
func (m *Manager) ToJSON() ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return json.Marshal(m.agents)
}

// hasCapability checks if an agent has a specific capability
func hasCapability(agent *Agent, capability string) bool {
	switch capability {
	case "code":
		return agent.Capabilities.CanCode
	case "research":
		return agent.Capabilities.CanResearch
	case "test":
		return agent.Capabilities.CanTest
	case "browse":
		return agent.Capabilities.CanBrowse
	default:
		// Check specialties
		for _, s := range agent.Capabilities.Specialties {
			if s == capability {
				return true
			}
		}
		return false
	}
}
