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

// ValidAgents is the authoritative list of known agent IDs
// Note: Claud-win uses Capital C per Windows sync (SYNC-STATUS-1766055837)
var ValidAgents = map[string]bool{
	"Claud-win":    true,
	"claude-mac":   true,
	"ag-win":       true,
	"antigravity":  true,
	"gemini":       true,
	"comm-monitor": true,
	"claude":       true,
	"cli":          true,
}

// TypoCorrections maps common typos to correct agent IDs
// Note: Claud-win uses Capital C per Windows sync (SYNC-STATUS-1766055837)
var TypoCorrections = map[string]string{
	"claude-win": "Claud-win",
	"claudwin":   "Claud-win",
	"claud win":  "Claud-win",
	"calude-win": "Claud-win",
	"cluad-win":  "Claud-win",
	"claud_win":  "Claud-win",
	"claud-win":  "Claud-win",

	"claude mac":  "claude-mac",
	"claudemac":   "claude-mac",
	"claude_mac":  "claude-mac",
	"cluade-mac":  "claude-mac",
	"calude-mac":  "claude-mac",

	"agwin":       "ag-win",
	"ag win":      "ag-win",
	"ag_win":      "ag-win",

	"anti-gravity": "antigravity",
	"anti gravity": "antigravity",
}

// Config holds monitor configuration
type Config struct {
	Threshold    float64       // Fuzzy match threshold (0.0-1.0)
	Enabled      bool          // Enable/disable monitor
	UseHooks     bool          // Use event hooks instead of polling (recommended)
	PollInterval time.Duration // DEPRECATED: Only used if UseHooks=false
}

// DefaultConfig returns sensible defaults
func DefaultConfig() Config {
	return Config{
		Threshold:    0.75,
		Enabled:      true,
		UseHooks:     true,                // Event-driven by default (no polling)
		PollInterval: 10 * time.Second,   // DEPRECATED: Only for backward compat
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

	// Stats
	mu              sync.RWMutex
	signalCount     int
	correctionCount int
}

// New creates a new communication monitor
func New(pb *pocketbase.PocketBase, config Config, logger *slog.Logger) *Monitor {
	ctx, cancel := context.WithCancel(context.Background())

	if logger == nil {
		logger = slog.Default()
	}

	return &Monitor{
		pb:     pb,
		config: config,
		logger: logger.WithGroup("commmonitor"),
		ctx:    ctx,
		cancel: cancel,
	}
}

// RegisterHooks registers event hooks for real-time signal monitoring
// This is the recommended approach (UseHooks=true) - event-driven, no polling
func (m *Monitor) RegisterHooks() {
	if !m.config.Enabled {
		m.logger.Info("Communication monitor disabled")
		return
	}

	if !m.config.UseHooks {
		m.logger.Warn("Hook-based monitoring disabled, falling back to polling (deprecated)")
		return
	}

	m.logger.Info("Registering communication monitor hooks (event-driven)",
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

	m.logger.Info("Communication monitor hooks registered (real-time corrections enabled)")
}

// Start begins the monitoring service
// If UseHooks=true, this does nothing (hooks are registered separately via RegisterHooks)
// If UseHooks=false, this starts the deprecated polling loop
func (m *Monitor) Start() {
	if !m.config.Enabled {
		m.logger.Info("Communication monitor disabled")
		return
	}

	if m.config.UseHooks {
		m.logger.Info("Communication monitor using event hooks (no polling needed)")
		return
	}

	// Deprecated polling mode
	m.logger.Warn("Starting DEPRECATED polling mode - consider switching to UseHooks=true")
	m.wg.Add(1)
	go m.monitorLoop()
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

// Stats returns current monitoring statistics
func (m *Monitor) Stats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return map[string]interface{}{
		"signals_checked":   m.signalCount,
		"corrections_made":  m.correctionCount,
		"enabled":           m.config.Enabled,
		"poll_interval":     m.config.PollInterval.String(),
		"threshold":         m.config.Threshold,
	}
}

// monitorLoop is the main monitoring loop
func (m *Monitor) monitorLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.config.PollInterval)
	defer ticker.Stop()

	// Track corrected signal IDs to avoid re-processing
	correctedIDs := make(map[string]bool)

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.checkAndCorrectSignals(correctedIDs)
		}
	}
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
	if fromAgent != "" && !ValidAgents[strings.ToLower(fromAgent)] {
		if corrected := m.fuzzyMatchAgent(fromAgent); corrected != "" {
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
	isValidTo := ValidAgents[toAgentLower]
	if toAgent != "" && !isValidTo {
		corrected := m.fuzzyMatchAgent(toAgent)
		if corrected != "" {
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
		}
	}
}

// checkAndCorrectSignals scans signals for invalid agent IDs and corrects them
// DEPRECATED: Used only by polling mode (UseHooks=false)
func (m *Monitor) checkAndCorrectSignals(correctedIDs map[string]bool) {
	// Get signals with potentially invalid to_agent or from_agent
	signals, err := m.getSignalsNeedingCorrection()
	if err != nil {
		m.logger.Error("Failed to fetch signals", "error", err)
		return
	}

	if len(signals) == 0 {
		return
	}

	correctedThisCycle := 0
	checkedThisCycle := 0

	for _, signal := range signals {
		// Skip already corrected
		if correctedIDs[signal.Id] {
			continue
		}

		checkedThisCycle++

		m.mu.Lock()
		m.signalCount++
		m.mu.Unlock()

		needsSave := false

		// Check and correct from_agent
		fromAgent := signal.GetString("from_agent")
		if fromAgent != "" && !ValidAgents[strings.ToLower(fromAgent)] {
			if corrected := m.fuzzyMatchAgent(fromAgent); corrected != "" {
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
		isValidTo := ValidAgents[toAgentLower]
		if toAgent != "" && !isValidTo {
			corrected := m.fuzzyMatchAgent(toAgent)
			m.logger.Info("Checking to_agent",
				"signal_id", signal.GetString("signal_id"),
				"to_agent", toAgent,
				"to_agent_lower", toAgentLower,
				"is_valid", isValidTo,
				"corrected", corrected,
			)
			if corrected != "" {
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
			} else {
				correctedIDs[signal.Id] = true
				correctedThisCycle++
			}
		}
	}

	// Log stats if anything was checked or corrected
	if checkedThisCycle > 0 || correctedThisCycle > 0 {
		m.logger.Info("Cycle complete",
			"checked", checkedThisCycle,
			"corrected", correctedThisCycle,
			"total_corrected", len(correctedIDs),
		)
	}
}

// getSignalsNeedingCorrection finds signals with invalid agent IDs
func (m *Monitor) getSignalsNeedingCorrection() ([]*core.Record, error) {
	// Get all signals and filter in Go (PocketBase filter syntax is limited)
	return m.getAllSignalsForCheck()
}

// getAllSignalsForCheck gets a batch of signals to check (newest first)
func (m *Monitor) getAllSignalsForCheck() ([]*core.Record, error) {
	// Get recent signals - PocketBase accepts -id for descending by ID
	// IDs in PocketBase are alphabetically sortable and newer ones come later
	records, err := m.pb.FindRecordsByFilter(
		"signals",
		"",
		"-id", // Sort by ID descending (newer signals have alphabetically later IDs)
		100,
		0,
	)
	return records, err
}

// getRecentSignals fetches recent signals from the database
// FIXED: Removed dangerous FindAllRecords fallback that could load entire table into RAM
func (m *Monitor) getRecentSignals(limit int) ([]*core.Record, error) {
	// Sort by ID descending (newer signals have alphabetically later IDs in PocketBase)
	records, err := m.pb.FindRecordsByFilter(
		"signals",
		"",           // match all
		"-id",        // Sort by ID descending to get newest first
		limit,
		0,
	)
	// Fail fast - no fallback that loads entire table
	return records, err
}

// fuzzyMatchAgent finds the closest matching valid agent ID
func (m *Monitor) fuzzyMatchAgent(agentID string) string {
	if agentID == "" {
		return ""
	}

	agentLower := strings.ToLower(agentID)

	// Check exact match (case-insensitive)
	for valid := range ValidAgents {
		if agentLower == strings.ToLower(valid) {
			return valid
		}
	}

	// Check typo corrections
	if corrected, ok := TypoCorrections[agentLower]; ok {
		return corrected
	}

	// Fuzzy match using simple similarity
	bestMatch := ""
	bestSimilarity := 0.0

	for valid := range ValidAgents {
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
func levenshteinDistance(s1, s2 string) int {
	if len(s1) == 0 {
		return len(s2)
	}
	if len(s2) == 0 {
		return len(s1)
	}

	// Create matrix
	matrix := make([][]int, len(s1)+1)
	for i := range matrix {
		matrix[i] = make([]int, len(s2)+1)
	}

	// Initialize first row and column
	for i := 0; i <= len(s1); i++ {
		matrix[i][0] = i
	}
	for j := 0; j <= len(s2); j++ {
		matrix[0][j] = j
	}

	// Fill matrix
	for i := 1; i <= len(s1); i++ {
		for j := 1; j <= len(s2); j++ {
			cost := 1
			if s1[i-1] == s2[j-1] {
				cost = 0
			}

			matrix[i][j] = min(
				matrix[i-1][j]+1,      // deletion
				matrix[i][j-1]+1,      // insertion
				matrix[i-1][j-1]+cost, // substitution
			)
		}
	}

	return matrix[len(s1)][len(s2)]
}

// correctSignal updates a signal's agent ID field
func (m *Monitor) correctSignal(signal *core.Record, field, corrected string) bool {
	signal.Set(field, corrected)

	if err := m.pb.Save(signal); err != nil {
		m.logger.Error("Failed to correct signal",
			"error", err,
			"field", field,
			"value", corrected,
		)
		return false
	}

	return true
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
