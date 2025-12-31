package archiver

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// mockSignalStore is a mock implementation of SignalStore for testing.
type mockSignalStore struct {
	mu             sync.Mutex
	signals        map[string]Signal
	archiveSignals map[string]ArchivedSignal
}

func newMockStore() *mockSignalStore {
	return &mockSignalStore{
		signals:        make(map[string]Signal),
		archiveSignals: make(map[string]ArchivedSignal),
	}
}

func (m *mockSignalStore) GetSignals(ctx context.Context, filter string, limit int) ([]Signal, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var result []Signal
	for _, s := range m.signals {
		result = append(result, s)
		if len(result) >= limit {
			break
		}
	}
	return result, nil
}

func (m *mockSignalStore) DeleteSignal(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.signals, id)
	return nil
}

func (m *mockSignalStore) CreateArchiveRecord(ctx context.Context, signal ArchivedSignal) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.archiveSignals[signal.ID] = signal
	return nil
}

func (m *mockSignalStore) GetArchiveSignals(ctx context.Context, filter string, limit int) ([]ArchivedSignal, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var result []ArchivedSignal
	for _, s := range m.archiveSignals {
		result = append(result, s)
		if len(result) >= limit {
			break
		}
	}
	return result, nil
}

func (m *mockSignalStore) DeleteArchiveRecord(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.archiveSignals, id)
	return nil
}

func (m *mockSignalStore) addSignal(s Signal) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.signals[s.ID] = s
}

func (m *mockSignalStore) signalCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.signals)
}

func (m *mockSignalStore) archiveCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.archiveSignals)
}

func TestNew(t *testing.T) {
	store := newMockStore()
	config := DefaultConfig()

	a := New(config, store, nil)

	if a == nil {
		t.Fatal("New() returned nil")
	}

	if a.config.ActiveRetentionDays != 3 {
		t.Errorf("Expected ActiveRetentionDays=3, got %d", a.config.ActiveRetentionDays)
	}

	if a.config.BatchSize != 200 {
		t.Errorf("Expected BatchSize=200, got %d", a.config.BatchSize)
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.ActiveRetentionDays != 3 {
		t.Errorf("Expected ActiveRetentionDays=3, got %d", config.ActiveRetentionDays)
	}

	if config.RecentRetentionDays != 90 {
		t.Errorf("Expected RecentRetentionDays=90, got %d", config.RecentRetentionDays)
	}

	if config.BatchSize != 200 {
		t.Errorf("Expected BatchSize=200, got %d", config.BatchSize)
	}

	if len(config.ActiveAgents) != 2 {
		t.Errorf("Expected 2 active agents, got %d", len(config.ActiveAgents))
	}

	if len(config.DeprecatedAgents) != 4 {
		t.Errorf("Expected 4 deprecated agents, got %d", len(config.DeprecatedAgents))
	}
}

func TestIsActiveAgent(t *testing.T) {
	store := newMockStore()
	config := DefaultConfig()
	a := New(config, store, nil)

	tests := []struct {
		agent    string
		expected bool
	}{
		{"claude-mac", true},
		{"claude-win", true},
		{"gemini", false},
		{"claude", false},
		{"unknown", false},
	}

	for _, tt := range tests {
		t.Run(tt.agent, func(t *testing.T) {
			result := a.isActiveAgent(tt.agent)
			if result != tt.expected {
				t.Errorf("isActiveAgent(%s) = %v, expected %v", tt.agent, result, tt.expected)
			}
		})
	}
}

func TestStartStop(t *testing.T) {
	store := newMockStore()
	config := DefaultConfig()
	config.CheckInterval = 100 * time.Millisecond
	a := New(config, store, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start
	err := a.Start(ctx)
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	if !a.IsRunning() {
		t.Error("Expected IsRunning() = true after Start()")
	}

	// Starting again should fail
	err = a.Start(ctx)
	if err == nil {
		t.Error("Expected error when starting already running archiver")
	}

	// Stop
	a.Stop()

	if a.IsRunning() {
		t.Error("Expected IsRunning() = false after Stop()")
	}
}

func TestArchiveReadMessages(t *testing.T) {
	store := newMockStore()
	config := DefaultConfig()
	config.ActiveRetentionDays = 0 // Archive immediately
	a := New(config, store, nil)

	// Add some signals
	now := time.Now()
	store.addSignal(Signal{
		ID:        "1",
		SignalID:  "SIG-1",
		FromAgent: "gemini",
		ToAgent:   "claude-mac",
		Read:      true,
		Created:   now.Add(-48 * time.Hour),
	})
	store.addSignal(Signal{
		ID:        "2",
		SignalID:  "SIG-2",
		FromAgent: "claude-mac",
		ToAgent:   "claude-win",
		Read:      true,
		Created:   now.Add(-24 * time.Hour),
	})

	ctx := context.Background()
	a.RunArchiveCycle(ctx)

	// Check that signals were processed
	stats := a.Stats()
	if stats.TotalCyclesRun != 1 {
		t.Errorf("Expected TotalCyclesRun=1, got %d", stats.TotalCyclesRun)
	}
}

func TestPurgeDeprecated(t *testing.T) {
	store := newMockStore()
	config := DefaultConfig()

	// Create temp directory for archives
	tmpDir, err := os.MkdirTemp("", "archiver-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config.ArchivePath = tmpDir
	a := New(config, store, nil)

	// Add deprecated agent signals
	store.addSignal(Signal{
		ID:        "1",
		SignalID:  "OLD-GEMINI-1",
		FromAgent: "gemini",
		ToAgent:   "claude",
		Read:      true,
		Created:   time.Now().Add(-72 * time.Hour),
	})
	store.addSignal(Signal{
		ID:        "2",
		SignalID:  "OLD-CLI-1",
		FromAgent: "cli",
		ToAgent:   "gemini",
		Read:      true,
		Created:   time.Now().Add(-72 * time.Hour),
	})

	ctx := context.Background()
	purged, err := a.purgeDeprecated(ctx)
	if err != nil {
		t.Fatalf("purgeDeprecated() failed: %v", err)
	}

	if purged != 2 {
		t.Errorf("Expected 2 purged, got %d", purged)
	}

	if store.signalCount() != 0 {
		t.Errorf("Expected 0 signals remaining, got %d", store.signalCount())
	}

	// Check deprecated archive files exist
	geminiFile := filepath.Join(tmpDir, "deprecated_agents", "gemini.json")
	if _, err := os.Stat(geminiFile); os.IsNotExist(err) {
		t.Error("Expected gemini.json archive file to exist")
	}
}

func TestWriteToFileArchive(t *testing.T) {
	store := newMockStore()
	config := DefaultConfig()

	tmpDir, err := os.MkdirTemp("", "archiver-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config.ArchivePath = tmpDir
	a := New(config, store, nil)

	signals := []ArchivedSignal{
		{
			Signal: Signal{
				ID:        "1",
				SignalID:  "SIG-1",
				FromAgent: "claude-mac",
				ToAgent:   "claude-win",
				Created:   time.Date(2025, 12, 15, 10, 0, 0, 0, time.UTC),
			},
			ArchivedAt: time.Now(),
			ArchivedBy: "test",
			Reason:     "test",
		},
	}

	err = a.writeToFileArchive("2025-12", signals)
	if err != nil {
		t.Fatalf("writeToFileArchive() failed: %v", err)
	}

	// Check file exists
	expectedPath := filepath.Join(tmpDir, "2025", "12", "signals_2025-12.json")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("Expected archive file at %s", expectedPath)
	}

	// Read and verify content
	data, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("Failed to read archive file: %v", err)
	}

	var archived []ArchivedSignal
	if err := json.Unmarshal(data, &archived); err != nil {
		t.Fatalf("Failed to parse archive file: %v", err)
	}

	if len(archived) != 1 {
		t.Errorf("Expected 1 archived signal, got %d", len(archived))
	}

	if archived[0].SignalID != "SIG-1" {
		t.Errorf("Expected SignalID=SIG-1, got %s", archived[0].SignalID)
	}
}

func TestUpdateIndex(t *testing.T) {
	store := newMockStore()
	config := DefaultConfig()

	tmpDir, err := os.MkdirTemp("", "archiver-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config.ArchivePath = tmpDir
	a := New(config, store, nil)

	// Create an archive file first
	signals := []ArchivedSignal{
		{
			Signal: Signal{
				ID:        "1",
				SignalID:  "SIG-1",
				FromAgent: "claude-mac",
				ToAgent:   "claude-win",
				Created:   time.Date(2025, 12, 15, 10, 0, 0, 0, time.UTC),
			},
			ArchivedAt: time.Now(),
		},
	}
	err = a.writeToFileArchive("2025-12", signals)
	if err != nil {
		t.Fatalf("writeToFileArchive() failed: %v", err)
	}

	// Update index
	err = a.updateIndex()
	if err != nil {
		t.Fatalf("updateIndex() failed: %v", err)
	}

	// Check index file exists
	indexPath := filepath.Join(tmpDir, "index.json")
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		t.Error("Expected index.json to exist")
	}

	// Read and verify index
	data, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("Failed to read index file: %v", err)
	}

	var index ArchiveIndex
	if err := json.Unmarshal(data, &index); err != nil {
		t.Fatalf("Failed to parse index file: %v", err)
	}

	if len(index.Archives) != 1 {
		t.Errorf("Expected 1 archive entry, got %d", len(index.Archives))
	}

	if index.Archives[0].MessageCount != 1 {
		t.Errorf("Expected MessageCount=1, got %d", index.Archives[0].MessageCount)
	}
}

func TestStats(t *testing.T) {
	store := newMockStore()
	config := DefaultConfig()
	a := New(config, store, nil)

	stats := a.Stats()

	if stats.TotalCyclesRun != 0 {
		t.Errorf("Expected TotalCyclesRun=0, got %d", stats.TotalCyclesRun)
	}

	if stats.MessagesArchived != 0 {
		t.Errorf("Expected MessagesArchived=0, got %d", stats.MessagesArchived)
	}
}

func TestRunOnce(t *testing.T) {
	store := newMockStore()
	config := DefaultConfig()

	tmpDir, err := os.MkdirTemp("", "archiver-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config.ArchivePath = tmpDir
	a := New(config, store, nil)

	ctx := context.Background()
	err = a.RunOnce(ctx)
	if err != nil {
		t.Fatalf("RunOnce() failed: %v", err)
	}

	stats := a.Stats()
	if stats.TotalCyclesRun != 1 {
		t.Errorf("Expected TotalCyclesRun=1 after RunOnce(), got %d", stats.TotalCyclesRun)
	}
}

func TestConcurrentAccess(t *testing.T) {
	store := newMockStore()
	config := DefaultConfig()
	config.CheckInterval = 50 * time.Millisecond

	tmpDir, err := os.MkdirTemp("", "archiver-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config.ArchivePath = tmpDir
	a := New(config, store, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start archiver
	err = a.Start(ctx)
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	// Concurrent access to stats
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = a.Stats()
				_ = a.IsRunning()
			}
		}()
	}

	wg.Wait()
	a.Stop()
}
