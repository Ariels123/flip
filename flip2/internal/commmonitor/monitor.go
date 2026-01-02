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
	PollInterval    time.Duration     // Poll interval for non-event monitoring
}

// DefaultConfig returns sensible defaults
func DefaultConfig() Config {
	return Config{
		Threshold:    0.75,
		Enabled:      true,
		PollInterval: 30 * time.Second,
	}
}

// Global agent data used for initialization and testing
var (
	// ValidAgents defines the set of all known valid agent IDs
	ValidAgents = map[string]bool{
		"Claud-win":    true,
		"claude-mac":   true,
		"ag-win":       true,
		"antigravity":  true,
		"gemini":       true,
		"comm-monitor": true,
		"claude":       true,
		"cli":          true,
	}

	// TypoCorrections maps common typos to valid agent IDs
	TypoCorrections = map[string]string{
		"claude-win":   "Claud-win",
		"claudwin":     "Claud-win",
		"claud win":    "Claud-win",
		"calude-win":   "Claud-win",
		"claud-win":    "Claud-win",
		"claude mac":   "claude-mac",
		"claudemac":    "claude-mac",
		"agwin":        "ag-win",
		"ag win":       "ag-win",
		"anti-gravity": "antigravity",
		"anti gravity": "antigravity",
	}
)

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
	mu                 sync.RWMutex
	signalCount        int
	correctionCount    int
	errorCount         int
	detectionFailures  int // Signals with invalid agents that couldn't be corrected
	validationErrors   int // Signals with malformed data
	fuzzyMatchAttempts int // Total fuzzy match attempts
	fuzzyMatchSuccess  int // Successful fuzzy matches
	saveErrors         int // Failed save operations
	lastError          error
	lastErrorTime      time.Time
	errorsByType       map[string]int // Count errors by type
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
		errorsByType:    make(map[string]int),
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

	// Validate signal data
	if !m.validateSignal(signal) {
		m.recordError("validation", nil)
		return
	}

	needsSave := false
	detectionFailed := false

	// Check and correct from_agent
	fromAgent := signal.GetString("from_agent")
	if fromAgent != "" && !m.validAgents[strings.ToLower(fromAgent)] {
		corrected, matched := m.fuzzyMatchAgentWithStats(fromAgent)
		if matched && corrected != fromAgent {
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
		} else if !matched {
			// Detection failure: invalid agent couldn't be corrected
			m.logger.Warn("Could not correct invalid from_agent",
				"signal_id", signal.GetString("signal_id"),
				"agent", fromAgent,
			)
			detectionFailed = true
		}
	}

	// Check and correct to_agent
	toAgent := signal.GetString("to_agent")
	toAgentLower := strings.ToLower(toAgent)
	isValidTo := m.validAgents[toAgentLower]
	if toAgent != "" && !isValidTo {
		corrected, matched := m.fuzzyMatchAgentWithStats(toAgent)
		if matched && corrected != toAgent {
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
		} else if !matched {
			// Detection failure: invalid agent couldn't be corrected
			m.logger.Warn("Could not correct invalid to_agent",
				"signal_id", signal.GetString("signal_id"),
				"agent", toAgent,
			)
			detectionFailed = true
		}
	}

	// Track detection failures (silent failures where invalid agents exist but can't be fixed)
	if detectionFailed {
		m.mu.Lock()
		m.detectionFailures++
		m.mu.Unlock()
	}

	// Save if corrections were made
	if needsSave {
		if err := m.pb.Save(signal); err != nil {
			m.logger.Error("Failed to save correction", "error", err, "signal_id", signal.Id)
			m.recordError("save", err)
		}
	}
}

// validateSignal checks if signal has required fields
func (m *Monitor) validateSignal(signal *core.Record) bool {
	signalID := signal.GetString("signal_id")
	fromAgent := signal.GetString("from_agent")
	toAgent := signal.GetString("to_agent")

	// Check for missing required fields
	if signalID == "" || fromAgent == "" || toAgent == "" {
		m.logger.Warn("Signal missing required fields",
			"id", signal.Id,
			"signal_id", signalID,
			"from_agent", fromAgent,
			"to_agent", toAgent,
		)
		m.mu.Lock()
		m.validationErrors++
		m.mu.Unlock()
		return false
	}

	return true
}

// fuzzyMatchAgentWithStats wraps fuzzyMatchAgent with statistics tracking
func (m *Monitor) fuzzyMatchAgentWithStats(agentID string) (string, bool) {
	if agentID == "" {
		return "", false
	}

	// Track fuzzy match attempt
	m.mu.Lock()
	m.fuzzyMatchAttempts++
	m.mu.Unlock()

	corrected := m.fuzzyMatchAgent(agentID)
	matched := corrected != "" && corrected != agentID

	// Track success
	if matched {
		m.mu.Lock()
		m.fuzzyMatchSuccess++
		m.mu.Unlock()
	}

	return corrected, matched
}

// recordError tracks errors by type and updates error stats
func (m *Monitor) recordError(errorType string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.errorCount++
	m.errorsByType[errorType]++

	if errorType == "save" {
		m.saveErrors++
	}

	if err != nil {
		m.lastError = err
		m.lastErrorTime = time.Now()
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

	// Calculate success rate for fuzzy matching
	fuzzyMatchRate := 0.0
	if m.fuzzyMatchAttempts > 0 {
		fuzzyMatchRate = float64(m.fuzzyMatchSuccess) / float64(m.fuzzyMatchAttempts)
	}

	// Copy error counts by type
	errorsByType := make(map[string]int)
	for k, v := range m.errorsByType {
		errorsByType[k] = v
	}

	return map[string]interface{}{
		"signals_checked":      m.signalCount,
		"corrections_made":     m.correctionCount,
		"error_count":          m.errorCount,
		"detection_failures":   m.detectionFailures,
		"validation_errors":    m.validationErrors,
		"fuzzy_match_attempts": m.fuzzyMatchAttempts,
		"fuzzy_match_success":  m.fuzzyMatchSuccess,
		"fuzzy_match_rate":     fuzzyMatchRate,
		"save_errors":          m.saveErrors,
		"errors_by_type":       errorsByType,
		"last_error":           lastErrorStr,
		"last_error_time":      m.lastErrorTime,
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
