// Package archiver provides message archiving functionality for FLIP2.
// It implements a multi-tier archiving system:
// - Tier 1: Active messages (signals collection) - 3 day retention
// - Tier 2: Recent archive (signals_archive collection) - 90 day retention
// - Tier 3: Long-term archive (file-based) - indefinite
// - Tier 4: Deprecated agent purge
package archiver

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Config holds archiver configuration.
type Config struct {
	Enabled            bool          `yaml:"enabled"`
	ActiveRetentionDays int          `yaml:"active_retention_days"` // Default: 3
	RecentRetentionDays int          `yaml:"recent_retention_days"` // Default: 90
	CheckInterval       time.Duration `yaml:"check_interval"`       // Default: 6h
	BatchSize           int           `yaml:"batch_size"`           // Default: 200
	ActiveAgents        []string      `yaml:"active_agents"`
	DeprecatedAgents    []string      `yaml:"deprecated_agents"`
	ArchivePath         string        `yaml:"archive_path"` // Default: archives/signals
}

// DefaultConfig returns the default archiver configuration.
func DefaultConfig() Config {
	return Config{
		Enabled:             true,
		ActiveRetentionDays: 3,
		RecentRetentionDays: 90,
		CheckInterval:       6 * time.Hour,
		BatchSize:           200,
		ActiveAgents:        []string{"claude-mac", "claude-win"},
		DeprecatedAgents:    []string{"gemini", "claude", "cli", "antigravity"},
		ArchivePath:         "archives/signals",
	}
}

// Signal represents a message in the signals collection.
type Signal struct {
	ID         string    `json:"id"`
	SignalID   string    `json:"signal_id"`
	FromAgent  string    `json:"from_agent"`
	ToAgent    string    `json:"to_agent"`
	SignalType string    `json:"signal_type"`
	Priority   string    `json:"priority"`
	Content    string    `json:"content"`
	Read       bool      `json:"read"`
	ReadAt     string    `json:"read_at,omitempty"`
	Created    time.Time `json:"created"`
	Updated    time.Time `json:"updated"`
}

// ArchivedSignal extends Signal with archive metadata.
type ArchivedSignal struct {
	Signal
	ArchivedAt         time.Time `json:"archived_at"`
	ArchivedBy         string    `json:"archived_by"`
	OriginalCollection string    `json:"original_collection"`
	Reason             string    `json:"reason"`
}

// ArchiveIndex tracks archived files for searching.
type ArchiveIndex struct {
	Archives []ArchiveEntry `json:"archives"`
	Updated  time.Time      `json:"updated"`
}

// ArchiveEntry represents a single archive file in the index.
type ArchiveEntry struct {
	File         string   `json:"file"`
	Period       string   `json:"period"`
	MessageCount int      `json:"message_count"`
	DateRange    []string `json:"date_range"`
	Agents       []string `json:"agents"`
}

// SignalStore defines the interface for signal storage operations.
type SignalStore interface {
	// GetSignals retrieves signals matching the filter.
	GetSignals(ctx context.Context, filter string, limit int) ([]Signal, error)
	// DeleteSignal removes a signal by ID.
	DeleteSignal(ctx context.Context, id string) error
	// CreateArchiveRecord creates a record in the archive collection.
	CreateArchiveRecord(ctx context.Context, signal ArchivedSignal) error
	// GetArchiveSignals retrieves signals from the archive collection.
	GetArchiveSignals(ctx context.Context, filter string, limit int) ([]ArchivedSignal, error)
	// DeleteArchiveRecord removes a record from the archive collection.
	DeleteArchiveRecord(ctx context.Context, id string) error
}

// Archiver manages message archiving.
type Archiver struct {
	config Config
	store  SignalStore
	logger *slog.Logger

	mu       sync.RWMutex
	running  bool
	stopChan chan struct{}

	// Stats
	stats ArchiveStats
}

// ArchiveStats tracks archiving statistics.
type ArchiveStats struct {
	mu                  sync.RWMutex
	LastRun             time.Time `json:"last_run"`
	MessagesArchived    int64     `json:"messages_archived"`
	MessagesDeleted     int64     `json:"messages_deleted"`
	DeprecatedPurged    int64     `json:"deprecated_purged"`
	LongTermArchived    int64     `json:"long_term_archived"`
	Errors              int64     `json:"errors"`
	LastError           string    `json:"last_error,omitempty"`
	LastErrorTime       time.Time `json:"last_error_time,omitempty"`
	TotalCyclesRun      int64     `json:"total_cycles_run"`
	AverageArchiveTime  float64   `json:"average_archive_time_ms"`
}

// New creates a new Archiver instance.
func New(config Config, store SignalStore, logger *slog.Logger) *Archiver {
	if logger == nil {
		logger = slog.Default()
	}

	return &Archiver{
		config:   config,
		store:    store,
		logger:   logger.With("component", "archiver"),
		stopChan: make(chan struct{}),
	}
}

// Start begins the archiving background process.
func (a *Archiver) Start(ctx context.Context) error {
	a.mu.Lock()
	if a.running {
		a.mu.Unlock()
		return fmt.Errorf("archiver already running")
	}
	a.running = true
	a.mu.Unlock()

	a.logger.Info("Starting message archiver",
		"check_interval", a.config.CheckInterval,
		"active_retention_days", a.config.ActiveRetentionDays,
		"batch_size", a.config.BatchSize,
	)

	go a.run(ctx)
	return nil
}

// Stop halts the archiving background process.
func (a *Archiver) Stop() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.running {
		return
	}

	close(a.stopChan)
	a.running = false
	a.logger.Info("Archiver stopped")
}

// run is the main archiving loop.
func (a *Archiver) run(ctx context.Context) {
	ticker := time.NewTicker(a.config.CheckInterval)
	defer ticker.Stop()

	// Run immediately on start
	a.RunArchiveCycle(ctx)

	for {
		select {
		case <-ticker.C:
			a.RunArchiveCycle(ctx)
		case <-a.stopChan:
			return
		case <-ctx.Done():
			return
		}
	}
}

// RunArchiveCycle performs a complete archive cycle.
func (a *Archiver) RunArchiveCycle(ctx context.Context) {
	start := time.Now()
	a.logger.Info("Starting archive cycle")

	// Phase 1: Archive read messages older than retention period
	archived, err := a.archiveReadMessages(ctx)
	if err != nil {
		a.recordError(err)
		a.logger.Error("Failed to archive read messages", "error", err)
	} else {
		a.stats.mu.Lock()
		a.stats.MessagesArchived += int64(archived)
		a.stats.mu.Unlock()
	}

	// Phase 2: Move old archive messages to long-term storage
	longTerm, err := a.archiveLongTerm(ctx)
	if err != nil {
		a.recordError(err)
		a.logger.Error("Failed to archive long-term messages", "error", err)
	} else {
		a.stats.mu.Lock()
		a.stats.LongTermArchived += int64(longTerm)
		a.stats.mu.Unlock()
	}

	// Phase 3: Purge deprecated agent messages
	purged, err := a.purgeDeprecated(ctx)
	if err != nil {
		a.recordError(err)
		a.logger.Error("Failed to purge deprecated messages", "error", err)
	} else {
		a.stats.mu.Lock()
		a.stats.DeprecatedPurged += int64(purged)
		a.stats.mu.Unlock()
	}

	// Update stats
	elapsed := time.Since(start)
	a.stats.mu.Lock()
	a.stats.LastRun = time.Now()
	a.stats.TotalCyclesRun++
	// Update running average
	n := float64(a.stats.TotalCyclesRun)
	a.stats.AverageArchiveTime = ((n-1)*a.stats.AverageArchiveTime + float64(elapsed.Milliseconds())) / n
	a.stats.mu.Unlock()

	a.logger.Info("Archive cycle complete",
		"archived", archived,
		"long_term", longTerm,
		"purged", purged,
		"elapsed", elapsed,
	)
}

// archiveReadMessages moves read messages older than retention to archive.
func (a *Archiver) archiveReadMessages(ctx context.Context) (int, error) {
	cutoff := time.Now().Add(-time.Duration(a.config.ActiveRetentionDays) * 24 * time.Hour)
	filter := fmt.Sprintf("read = true && created < '%s'", cutoff.Format(time.RFC3339))

	signals, err := a.store.GetSignals(ctx, filter, a.config.BatchSize)
	if err != nil {
		return 0, fmt.Errorf("failed to get signals: %w", err)
	}

	archived := 0
	for _, sig := range signals {
		// Skip active agents' recent messages
		if a.isActiveAgent(sig.FromAgent) || a.isActiveAgent(sig.ToAgent) {
			// Still archive if older than retention
			if time.Since(sig.Created) < time.Duration(a.config.ActiveRetentionDays)*24*time.Hour {
				continue
			}
		}

		archiveSig := ArchivedSignal{
			Signal:             sig,
			ArchivedAt:         time.Now(),
			ArchivedBy:         "archiver",
			OriginalCollection: "signals",
			Reason:             "read_and_expired",
		}

		if err := a.store.CreateArchiveRecord(ctx, archiveSig); err != nil {
			a.logger.Error("Failed to create archive record",
				"signal_id", sig.SignalID,
				"error", err,
			)
			continue
		}

		if err := a.store.DeleteSignal(ctx, sig.ID); err != nil {
			a.logger.Error("Failed to delete archived signal",
				"signal_id", sig.SignalID,
				"error", err,
			)
			continue
		}

		archived++
	}

	return archived, nil
}

// archiveLongTerm moves old archive records to file storage.
func (a *Archiver) archiveLongTerm(ctx context.Context) (int, error) {
	cutoff := time.Now().Add(-time.Duration(a.config.RecentRetentionDays) * 24 * time.Hour)
	filter := fmt.Sprintf("created < '%s'", cutoff.Format(time.RFC3339))

	signals, err := a.store.GetArchiveSignals(ctx, filter, a.config.BatchSize)
	if err != nil {
		return 0, fmt.Errorf("failed to get archive signals: %w", err)
	}

	if len(signals) == 0 {
		return 0, nil
	}

	// Group by month
	byMonth := make(map[string][]ArchivedSignal)
	for _, sig := range signals {
		month := sig.Created.Format("2006-01")
		byMonth[month] = append(byMonth[month], sig)
	}

	archived := 0
	for month, msgs := range byMonth {
		if err := a.writeToFileArchive(month, msgs); err != nil {
			a.logger.Error("Failed to write file archive",
				"month", month,
				"error", err,
			)
			continue
		}

		// Delete from archive collection
		for _, msg := range msgs {
			if err := a.store.DeleteArchiveRecord(ctx, msg.ID); err != nil {
				a.logger.Error("Failed to delete long-term archived signal",
					"signal_id", msg.SignalID,
					"error", err,
				)
				continue
			}
			archived++
		}
	}

	// Update index
	if err := a.updateIndex(); err != nil {
		a.logger.Error("Failed to update archive index", "error", err)
	}

	return archived, nil
}

// purgeDeprecated moves deprecated agent messages to purge storage.
func (a *Archiver) purgeDeprecated(ctx context.Context) (int, error) {
	purged := 0

	for _, agent := range a.config.DeprecatedAgents {
		// Skip if agent is also in active list (shouldn't happen but safety check)
		if a.isActiveAgent(agent) {
			continue
		}

		filter := fmt.Sprintf("from_agent = '%s' || to_agent = '%s'", agent, agent)
		signals, err := a.store.GetSignals(ctx, filter, a.config.BatchSize)
		if err != nil {
			a.logger.Error("Failed to get deprecated agent signals",
				"agent", agent,
				"error", err,
			)
			continue
		}

		if len(signals) == 0 {
			continue
		}

		// Write to deprecated archive
		if err := a.writeToDeprecatedArchive(agent, signals); err != nil {
			a.logger.Error("Failed to write deprecated archive",
				"agent", agent,
				"error", err,
			)
			continue
		}

		// Delete from active collection
		for _, sig := range signals {
			if err := a.store.DeleteSignal(ctx, sig.ID); err != nil {
				a.logger.Error("Failed to delete deprecated signal",
					"signal_id", sig.SignalID,
					"error", err,
				)
				continue
			}
			purged++
		}

		a.logger.Info("Purged deprecated agent messages",
			"agent", agent,
			"count", len(signals),
		)
	}

	return purged, nil
}

// writeToFileArchive writes signals to a monthly archive file.
func (a *Archiver) writeToFileArchive(month string, signals []ArchivedSignal) error {
	year := month[:4]
	dir := filepath.Join(a.config.ArchivePath, year, month[5:])

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create archive directory: %w", err)
	}

	filename := filepath.Join(dir, fmt.Sprintf("signals_%s.json", month))

	// Read existing file if it exists
	var existing []ArchivedSignal
	if data, err := os.ReadFile(filename); err == nil {
		if err := json.Unmarshal(data, &existing); err != nil {
			a.logger.Warn("Failed to parse existing archive, creating new",
				"file", filename,
				"error", err,
			)
		}
	}

	// Append new signals
	existing = append(existing, signals...)

	// Write back
	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal signals: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write archive file: %w", err)
	}

	a.logger.Info("Wrote to file archive",
		"file", filename,
		"count", len(signals),
	)

	return nil
}

// writeToDeprecatedArchive writes deprecated agent signals to a separate archive.
func (a *Archiver) writeToDeprecatedArchive(agent string, signals []Signal) error {
	dir := filepath.Join(a.config.ArchivePath, "deprecated_agents")

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create deprecated archive directory: %w", err)
	}

	filename := filepath.Join(dir, fmt.Sprintf("%s.json", agent))

	// Read existing file if it exists
	var existing []Signal
	if data, err := os.ReadFile(filename); err == nil {
		if err := json.Unmarshal(data, &existing); err != nil {
			a.logger.Warn("Failed to parse existing deprecated archive, creating new",
				"file", filename,
				"error", err,
			)
		}
	}

	// Append new signals
	existing = append(existing, signals...)

	// Write back
	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal signals: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write deprecated archive file: %w", err)
	}

	return nil
}

// updateIndex updates the archive search index.
func (a *Archiver) updateIndex() error {
	indexPath := filepath.Join(a.config.ArchivePath, "index.json")

	index := ArchiveIndex{
		Archives: []ArchiveEntry{},
		Updated:  time.Now(),
	}

	// Walk the archive directory
	err := filepath.Walk(a.config.ArchivePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		if info.IsDir() || filepath.Ext(path) != ".json" || filepath.Base(path) == "index.json" {
			return nil
		}

		// Skip deprecated_agents directory for main index
		if filepath.Base(filepath.Dir(path)) == "deprecated_agents" {
			return nil
		}

		// Read and parse file to get stats
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		var signals []ArchivedSignal
		if err := json.Unmarshal(data, &signals); err != nil {
			return nil
		}

		if len(signals) == 0 {
			return nil
		}

		// Extract metadata
		agents := make(map[string]bool)
		var minDate, maxDate time.Time
		for _, s := range signals {
			agents[s.FromAgent] = true
			agents[s.ToAgent] = true
			if minDate.IsZero() || s.Created.Before(minDate) {
				minDate = s.Created
			}
			if maxDate.IsZero() || s.Created.After(maxDate) {
				maxDate = s.Created
			}
		}

		agentList := make([]string, 0, len(agents))
		for agent := range agents {
			agentList = append(agentList, agent)
		}

		relPath, _ := filepath.Rel(a.config.ArchivePath, path)
		index.Archives = append(index.Archives, ArchiveEntry{
			File:         relPath,
			Period:       filepath.Base(path)[:7], // signals_YYYY-MM.json -> YYYY-MM
			MessageCount: len(signals),
			DateRange:    []string{minDate.Format("2006-01-02"), maxDate.Format("2006-01-02")},
			Agents:       agentList,
		})

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to walk archive directory: %w", err)
	}

	// Write index
	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal index: %w", err)
	}

	if err := os.WriteFile(indexPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write index file: %w", err)
	}

	return nil
}

// isActiveAgent checks if an agent is in the active agents list.
func (a *Archiver) isActiveAgent(agent string) bool {
	for _, active := range a.config.ActiveAgents {
		if active == agent {
			return true
		}
	}
	return false
}

// recordError records an error in stats.
func (a *Archiver) recordError(err error) {
	a.stats.mu.Lock()
	defer a.stats.mu.Unlock()
	a.stats.Errors++
	a.stats.LastError = err.Error()
	a.stats.LastErrorTime = time.Now()
}

// Stats returns a copy of the current archive statistics.
func (a *Archiver) Stats() ArchiveStats {
	a.stats.mu.RLock()
	defer a.stats.mu.RUnlock()
	return a.stats
}

// IsRunning returns whether the archiver is currently running.
func (a *Archiver) IsRunning() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.running
}

// RunOnce performs a single archive cycle without starting the background process.
func (a *Archiver) RunOnce(ctx context.Context) error {
	a.RunArchiveCycle(ctx)
	return nil
}
