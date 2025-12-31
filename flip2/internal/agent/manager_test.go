package agent

import (
	"fmt"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestNewManager(t *testing.T) {
	logger := testLogger()
	cfg := DefaultManagerConfig()

	m := NewManager(logger, cfg)
	defer m.Stop()

	if m == nil {
		t.Fatal("NewManager returned nil")
	}

	if m.Count() != 0 {
		t.Errorf("Expected 0 agents, got %d", m.Count())
	}
}

func TestRegisterAgent(t *testing.T) {
	logger := testLogger()
	m := NewManager(logger, DefaultManagerConfig())
	defer m.Stop()

	agent := &Agent{
		ID:      "test-agent-1",
		Backend: BackendClaude,
		Capabilities: Capabilities{
			CanCode:     true,
			CanResearch: true,
			MaxTokens:   100000,
		},
	}

	err := m.Register(agent)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if m.Count() != 1 {
		t.Errorf("Expected 1 agent, got %d", m.Count())
	}

	// Verify agent was registered
	retrieved, exists := m.Get("test-agent-1")
	if !exists {
		t.Fatal("Agent not found after registration")
	}

	if retrieved.Status != StatusOnline {
		t.Errorf("Expected status %s, got %s", StatusOnline, retrieved.Status)
	}

	if retrieved.Backend != BackendClaude {
		t.Errorf("Expected backend %s, got %s", BackendClaude, retrieved.Backend)
	}
}

func TestRegisterEmptyID(t *testing.T) {
	logger := testLogger()
	m := NewManager(logger, DefaultManagerConfig())
	defer m.Stop()

	agent := &Agent{
		ID:      "",
		Backend: BackendClaude,
	}

	err := m.Register(agent)
	if err == nil {
		t.Error("Expected error for empty agent ID")
	}
}

func TestUnregisterAgent(t *testing.T) {
	logger := testLogger()
	m := NewManager(logger, DefaultManagerConfig())
	defer m.Stop()

	agent := &Agent{
		ID:      "test-agent-1",
		Backend: BackendGemini,
	}

	m.Register(agent)

	err := m.Unregister("test-agent-1")
	if err != nil {
		t.Fatalf("Unregister failed: %v", err)
	}

	if m.Count() != 0 {
		t.Errorf("Expected 0 agents, got %d", m.Count())
	}

	_, exists := m.Get("test-agent-1")
	if exists {
		t.Error("Agent still exists after unregister")
	}
}

func TestUnregisterNonexistent(t *testing.T) {
	logger := testLogger()
	m := NewManager(logger, DefaultManagerConfig())
	defer m.Stop()

	err := m.Unregister("nonexistent")
	if err == nil {
		t.Error("Expected error for unregistering nonexistent agent")
	}
}

func TestHeartbeat(t *testing.T) {
	logger := testLogger()
	m := NewManager(logger, DefaultManagerConfig())
	defer m.Stop()

	agent := &Agent{
		ID:      "test-agent-1",
		Backend: BackendClaude,
	}

	m.Register(agent)

	// Record initial last_seen
	initial, _ := m.Get("test-agent-1")
	initialTime := initial.LastSeen

	time.Sleep(10 * time.Millisecond)

	// Send heartbeat
	err := m.Heartbeat("test-agent-1")
	if err != nil {
		t.Fatalf("Heartbeat failed: %v", err)
	}

	// Verify last_seen was updated
	updated, _ := m.Get("test-agent-1")
	if !updated.LastSeen.After(initialTime) {
		t.Error("Heartbeat did not update last_seen")
	}
}

func TestSetStatus(t *testing.T) {
	logger := testLogger()
	m := NewManager(logger, DefaultManagerConfig())
	defer m.Stop()

	agent := &Agent{
		ID:      "test-agent-1",
		Backend: BackendClaude,
	}

	m.Register(agent)

	// Set to busy
	err := m.SetStatus("test-agent-1", StatusBusy)
	if err != nil {
		t.Fatalf("SetStatus failed: %v", err)
	}

	retrieved, _ := m.Get("test-agent-1")
	if retrieved.Status != StatusBusy {
		t.Errorf("Expected status %s, got %s", StatusBusy, retrieved.Status)
	}

	// Set to idle
	m.SetStatus("test-agent-1", StatusIdle)
	retrieved, _ = m.Get("test-agent-1")
	if retrieved.Status != StatusIdle {
		t.Errorf("Expected status %s, got %s", StatusIdle, retrieved.Status)
	}
}

func TestListByStatus(t *testing.T) {
	logger := testLogger()
	m := NewManager(logger, DefaultManagerConfig())
	defer m.Stop()

	// Register multiple agents with different statuses
	agents := []*Agent{
		{ID: "online-1", Backend: BackendClaude},
		{ID: "online-2", Backend: BackendGemini},
		{ID: "idle-1", Backend: BackendClaude},
	}

	for _, a := range agents {
		m.Register(a)
	}

	m.SetStatus("idle-1", StatusIdle)

	// Test ListByStatus
	onlineAgents := m.ListByStatus(StatusOnline)
	if len(onlineAgents) != 2 {
		t.Errorf("Expected 2 online agents, got %d", len(onlineAgents))
	}

	idleAgents := m.ListByStatus(StatusIdle)
	if len(idleAgents) != 1 {
		t.Errorf("Expected 1 idle agent, got %d", len(idleAgents))
	}
}

func TestListByBackend(t *testing.T) {
	logger := testLogger()
	m := NewManager(logger, DefaultManagerConfig())
	defer m.Stop()

	agents := []*Agent{
		{ID: "claude-1", Backend: BackendClaude},
		{ID: "claude-2", Backend: BackendClaude},
		{ID: "gemini-1", Backend: BackendGemini},
	}

	for _, a := range agents {
		m.Register(a)
	}

	claudeAgents := m.ListByBackend(BackendClaude)
	if len(claudeAgents) != 2 {
		t.Errorf("Expected 2 claude agents, got %d", len(claudeAgents))
	}

	geminiAgents := m.ListByBackend(BackendGemini)
	if len(geminiAgents) != 1 {
		t.Errorf("Expected 1 gemini agent, got %d", len(geminiAgents))
	}
}

func TestListAvailable(t *testing.T) {
	logger := testLogger()
	m := NewManager(logger, DefaultManagerConfig())
	defer m.Stop()

	agents := []*Agent{
		{ID: "online-1", Backend: BackendClaude},
		{ID: "idle-1", Backend: BackendGemini},
		{ID: "busy-1", Backend: BackendClaude},
	}

	for _, a := range agents {
		m.Register(a)
	}

	m.SetStatus("idle-1", StatusIdle)
	m.SetStatus("busy-1", StatusBusy)

	available := m.ListAvailable()
	if len(available) != 2 {
		t.Errorf("Expected 2 available agents, got %d", len(available))
	}
}

func TestFindByCapability(t *testing.T) {
	logger := testLogger()
	m := NewManager(logger, DefaultManagerConfig())
	defer m.Stop()

	agents := []*Agent{
		{
			ID:      "coder-1",
			Backend: BackendClaude,
			Capabilities: Capabilities{
				CanCode: true,
			},
		},
		{
			ID:      "researcher-1",
			Backend: BackendGemini,
			Capabilities: Capabilities{
				CanResearch: true,
			},
		},
		{
			ID:      "tester-1",
			Backend: BackendAntigravity,
			Capabilities: Capabilities{
				CanTest:   true,
				CanBrowse: true,
			},
		},
	}

	for _, a := range agents {
		m.Register(a)
	}

	coders := m.FindByCapability("code")
	if len(coders) != 1 {
		t.Errorf("Expected 1 coder, got %d", len(coders))
	}

	testers := m.FindByCapability("test")
	if len(testers) != 1 {
		t.Errorf("Expected 1 tester, got %d", len(testers))
	}

	browsers := m.FindByCapability("browse")
	if len(browsers) != 1 {
		t.Errorf("Expected 1 browser, got %d", len(browsers))
	}
}

func TestSelectBestAgent(t *testing.T) {
	logger := testLogger()
	m := NewManager(logger, DefaultManagerConfig())
	defer m.Stop()

	agents := []*Agent{
		{
			ID:      "claude-1",
			Backend: BackendClaude,
			Capabilities: Capabilities{
				CanCode:   true,
				MaxTokens: 100000,
			},
		},
		{
			ID:      "gemini-1",
			Backend: BackendGemini,
			Capabilities: Capabilities{
				CanResearch: true,
				MaxTokens:   200000,
			},
		},
	}

	for _, a := range agents {
		m.Register(a)
	}

	// Prefer Claude for coding
	req := &Capabilities{CanCode: true}
	best, err := m.SelectBestAgent(req, BackendClaude)
	if err != nil {
		t.Fatalf("SelectBestAgent failed: %v", err)
	}

	if best.ID != "claude-1" {
		t.Errorf("Expected claude-1, got %s", best.ID)
	}

	// Prefer Gemini for research
	req = &Capabilities{CanResearch: true}
	best, err = m.SelectBestAgent(req, BackendGemini)
	if err != nil {
		t.Fatalf("SelectBestAgent failed: %v", err)
	}

	if best.ID != "gemini-1" {
		t.Errorf("Expected gemini-1, got %s", best.ID)
	}
}

func TestSelectBestAgentNoMatch(t *testing.T) {
	logger := testLogger()
	m := NewManager(logger, DefaultManagerConfig())
	defer m.Stop()

	// No agents registered
	_, err := m.SelectBestAgent(nil, BackendClaude)
	if err == nil {
		t.Error("Expected error when no agents available")
	}

	// Register but make unavailable
	agent := &Agent{
		ID:      "busy-agent",
		Backend: BackendClaude,
	}
	m.Register(agent)
	m.SetStatus("busy-agent", StatusBusy)

	_, err = m.SelectBestAgent(nil, BackendClaude)
	if err == nil {
		t.Error("Expected error when no agents available")
	}
}

func TestIncrementCounters(t *testing.T) {
	logger := testLogger()
	m := NewManager(logger, DefaultManagerConfig())
	defer m.Stop()

	agent := &Agent{
		ID:      "test-agent",
		Backend: BackendClaude,
	}
	m.Register(agent)

	// Increment task count
	m.IncrementTaskCount("test-agent")
	m.IncrementTaskCount("test-agent")
	m.IncrementTaskCount("test-agent")

	retrieved, _ := m.Get("test-agent")
	if retrieved.TaskCount != 3 {
		t.Errorf("Expected task count 3, got %d", retrieved.TaskCount)
	}

	// Increment error count
	m.IncrementErrorCount("test-agent")

	retrieved, _ = m.Get("test-agent")
	if retrieved.ErrorCount != 1 {
		t.Errorf("Expected error count 1, got %d", retrieved.ErrorCount)
	}
}

func TestStats(t *testing.T) {
	logger := testLogger()
	m := NewManager(logger, DefaultManagerConfig())
	defer m.Stop()

	agents := []*Agent{
		{ID: "online-1", Backend: BackendClaude},
		{ID: "online-2", Backend: BackendGemini},
		{ID: "idle-1", Backend: BackendClaude},
		{ID: "busy-1", Backend: BackendClaude},
	}

	for _, a := range agents {
		m.Register(a)
	}

	m.SetStatus("idle-1", StatusIdle)
	m.SetStatus("busy-1", StatusBusy)

	stats := m.Stats()

	if stats["total"] != 4 {
		t.Errorf("Expected total 4, got %d", stats["total"])
	}
	if stats["online"] != 2 {
		t.Errorf("Expected online 2, got %d", stats["online"])
	}
	if stats["idle"] != 1 {
		t.Errorf("Expected idle 1, got %d", stats["idle"])
	}
	if stats["busy"] != 1 {
		t.Errorf("Expected busy 1, got %d", stats["busy"])
	}
}

func TestHeartbeatChecker(t *testing.T) {
	logger := testLogger()
	cfg := ManagerConfig{
		HeartbeatInterval: 50 * time.Millisecond,
		OfflineThreshold:  100 * time.Millisecond,
	}

	var offlineCalled bool
	var offlineMu sync.Mutex

	cfg.OnAgentOffline = func(agent *Agent) {
		offlineMu.Lock()
		offlineCalled = true
		offlineMu.Unlock()
	}

	m := NewManager(logger, cfg)
	defer m.Stop()

	agent := &Agent{
		ID:      "test-agent",
		Backend: BackendClaude,
	}
	m.Register(agent)

	// Wait for heartbeat checker to mark offline
	time.Sleep(200 * time.Millisecond)

	retrieved, _ := m.Get("test-agent")
	if retrieved.Status != StatusOffline {
		t.Errorf("Expected status %s, got %s", StatusOffline, retrieved.Status)
	}

	offlineMu.Lock()
	if !offlineCalled {
		t.Error("OnAgentOffline callback was not called")
	}
	offlineMu.Unlock()
}

func TestConcurrentAccess(t *testing.T) {
	logger := testLogger()
	m := NewManager(logger, DefaultManagerConfig())
	defer m.Stop()

	// Concurrent registration
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			agent := &Agent{
				ID:      fmt.Sprintf("agent-%d", n),
				Backend: BackendClaude,
			}
			m.Register(agent)
		}(i)
	}
	wg.Wait()

	if m.Count() != 100 {
		t.Errorf("Expected 100 agents, got %d", m.Count())
	}

	// Concurrent reads
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			m.Get(fmt.Sprintf("agent-%d", n))
			m.List()
			m.Stats()
		}(i)
	}
	wg.Wait()
}

func TestToJSON(t *testing.T) {
	logger := testLogger()
	m := NewManager(logger, DefaultManagerConfig())
	defer m.Stop()

	agent := &Agent{
		ID:      "test-agent",
		Backend: BackendClaude,
		Capabilities: Capabilities{
			CanCode: true,
		},
	}
	m.Register(agent)

	jsonBytes, err := m.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON failed: %v", err)
	}

	if len(jsonBytes) == 0 {
		t.Error("ToJSON returned empty bytes")
	}
}

