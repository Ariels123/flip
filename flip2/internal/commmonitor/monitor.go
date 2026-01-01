// Package commmonitor provides communication monitoring with typo correction.
//
// Converted from Python monitor_communications.py to Go routine per
// COMM_MONITOR_GOROUTINE.md design doc from Claud-win.
package commmonitor

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// Config holds monitor configuration
type Config struct {
	Threshold       float64           // Fuzzy match threshold (0.0-1.0)
	Enabled         bool              // Enable/disable monitor
	ValidAgents     []string          // List of valid agent IDs (from config)
	TypoCorrections map[string]string // Typo correction map (from config)
}

// DefaultConfig returns sensible defaults
func DefaultConfig() Config {
	return Config{
		Threshold: 0.75,
		Enabled:   true,
	}
}

// Monitor runs the communication monitoring service
type Monitor struct {
	pb     *pocketbase.PocketBase
	config Config
	logger *slog.Logger

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Agent validation (from config, converted to maps for fast lookup)
	validAgents     map[string]bool
	typoCorrections map[string]string

	// Stats
	mu              sync.RWMutex
	signalCount     int
	correctionCount int
	errorCount      int
	lastError       error
	lastErrorTime   time.Time
}

// New creates a new communication monitor
func New(pb *pocketbase.PocketBase, config Config, logger *slog.Logger) *Monitor {
	ctx, cancel := context.WithCancel(context.Background())

	if logger == nil {
		logger = slog.Default()
	}

	// Convert ValidAgents array to map for fast lookup
	validAgents := make(map[string]bool, len(config.ValidAgents))
	for _, agent := range config.ValidAgents {
		validAgents[strings.ToLower(agent)] = true
	}

	// Use typo corrections from config (already a map)
	typoCorrections := config.TypoCorrections
	if typoCorrections == nil {
		typoCorrections = make(map[string]string)
	}

	return &Monitor{
		pb:              pb,
		config:          config,
		logger:          logger.WithGroup("commmonitor"),
		ctx:             ctx,
		cancel:          cancel,
		validAgents:     validAgents,
		typoCorrections: typoCorrections,
	}
}

// RegisterHooks registers event hooks for real-time signal monitoring
func (m *Monitor) RegisterHooks() {
	if !m.config.Enabled {
		m.logger.Info("Communication monitor disabled")
		return
	}

	m.logger.Info("Registering communication monitor hooks",
		"threshold", m.config.Threshold,
	)

	// Monitor signals on create
	m.pb.OnRecordAfterCreateSuccess("signals").BindFunc(func(e *core.RecordEvent) error {
		m.checkAndCorrectSignal(e.Record)
		return nil
	})

	// Monitor signals on update
	m.pb.OnRecordAfterUpdateSuccess("signals").BindFunc(func(e *core.RecordEvent) error {
		m.checkAndCorrectSignal(e.Record)
		return nil
	})

	m.logger.Info("Communication monitor hooks registered")
}

// Start is a no-op since monitoring is entirely event-driven via RegisterHooks
func (m *Monitor) Start() {
	if !m.config.Enabled {
		m.logger.Info("Communication monitor disabled")
		return
	}

	m.logger.Info("Communication monitor started (event-driven)")
}

// Stop gracefully shuts down the monitor
func (m *Monitor) Stop() {
	m.logger.Info("Stopping communication monitor...")
	m.cancel()
	m.wg.Wait()
	m.logger.Info("Communication monitor stopped",
		"signals_checked", m.signalCount,
		"corrections_made", m.correctionCount,
	)
}

// checkAndCorrectSignal processes a single signal record for typo correction
// Used by event hooks for real-time corrections
func (m *Monitor) checkAndCorrectSignal(signal *core.Record) {
	m.mu.Lock()
	m.signalCount++
	m.mu.Unlock()

	needsSave := false

	// Check and correct from_agent
	fromAgent := signal.GetString("from_agent")
	if fromAgent != "" && !m.validAgents[strings.ToLower(fromAgent)] {
		if corrected := m.fuzzyMatchAgent(fromAgent); corrected != "" && corrected != fromAgent {
			m.logger.Info("Correcting from_agent",
				"signal_id", signal.GetString("signal_id"),
				"original", fromAgent,
				"corrected", corrected,
			)
			signal.Set("from_agent", corrected)
			needsSave = true
			m.mu.Lock()
			m.correctionCount++
			m.mu.Unlock()
		}
	}

	// Check and correct to_agent
	toAgent := signal.GetString("to_agent")
	toAgentLower := strings.ToLower(toAgent)
	isValidTo := m.validAgents[toAgentLower]
	if toAgent != "" && !isValidTo {
		corrected := m.fuzzyMatchAgent(toAgent)
		if corrected != "" && corrected != toAgent {
			m.logger.Info("Correcting to_agent",
				"signal_id", signal.GetString("signal_id"),
				"original", toAgent,
				"corrected", corrected,
			)
			signal.Set("to_agent", corrected)
			needsSave = true
			m.mu.Lock()
			m.correctionCount++
			m.mu.Unlock()
		}
	}

	// Save if corrections were made
	if needsSave {
		if err := m.pb.Save(signal); err != nil {
			m.logger.Error("Failed to save correction", "error", err, "signal_id", signal.Id)
			m.mu.Lock()
			m.errorCount++
			m.lastError = err
			m.lastErrorTime = time.Now()
			m.mu.Unlock()
		}
	}
}

// fuzzyMatchAgent finds the closest matching valid agent ID
func (m *Monitor) fuzzyMatchAgent(agentID string) string {
	if agentID == "" {
		return ""
	}

	agentLower := strings.ToLower(agentID)

	// Check exact match (case-insensitive)
	for _, valid := range m.config.ValidAgents {
		if agentLower == strings.ToLower(valid) {
			return valid
		}
	}

	// Check typo corrections
	if corrected, ok := m.typoCorrections[agentLower]; ok {
		return corrected
	}

	// Fuzzy match using simple similarity
	bestMatch := ""
	bestSimilarity := 0.0

	for _, valid := range m.config.ValidAgents {
		similarity := m.calculateSimilarity(agentLower, strings.ToLower(valid))

		if similarity > bestSimilarity && similarity >= m.config.Threshold {
			bestSimilarity = similarity
			bestMatch = valid
		}
	}

	return bestMatch
}

// calculateSimilarity calculates string similarity (0.0 to 1.0)
// Uses a simple character-based comparison to avoid external dependencies
func (m *Monitor) calculateSimilarity(s1, s2 string) float64 {
	if s1 == s2 {
		return 1.0
	}

	if len(s1) == 0 || len(s2) == 0 {
		return 0.0
	}

	// Calculate Levenshtein distance
	distance := levenshteinDistance(s1, s2)

	// Convert to similarity ratio
	maxLen := max(len(s1), len(s2))
	return 1.0 - float64(distance)/float64(maxLen)
}

// levenshteinDistance calculates the edit distance between two strings
// Optimized to use O(min(N,M)) memory instead of O(N*M) by using only two rows
func levenshteinDistance(s1, s2 string) int {
	if len(s1) == 0 {
		return len(s2)
	}
	if len(s2) == 0 {
		return len(s1)
	}

	// Use two rows instead of full matrix to reduce memory allocation
	v0 := make([]int, len(s2)+1)
	v1 := make([]int, len(s2)+1)

	// Initialize first row
	for i := 0; i <= len(s2); i++ {
		v0[i] = i
	}

	// Calculate each subsequent row
	for i := 0; i < len(s1); i++ {
		v1[0] = i + 1

		for j := 0; j < len(s2); j++ {
			cost := 1
			if s1[i] == s2[j] {
				cost = 0
			}

			v1[j+1] = min(
				v1[j]+1,      // insertion
				v0[j+1]+1,    // deletion
				v0[j]+cost,   // substitution
			)
		}

		// Swap rows for next iteration
		copy(v0, v1)
	}

	return v0[len(s2)]
}

// Stats returns current monitoring statistics
func (m *Monitor) Stats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var lastErrorStr string
	if m.lastError != nil {
		lastErrorStr = m.lastError.Error()
	}

	return map[string]interface{}{
		"signals_checked":   m.signalCount,
		"corrections_made":  m.correctionCount,
		"error_count":       m.errorCount,
		"last_error":        lastErrorStr,
		"last_error_time":   m.lastErrorTime,
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}
