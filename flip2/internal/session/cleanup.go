package session

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/pocketbase/dbx"
)

// CleanupConfig holds configuration for session cleanup operations.
type CleanupConfig struct {
	// StaleThreshold is the duration after which a session without heartbeat is considered stale.
	// Default: 30 minutes
	StaleThreshold time.Duration

	// ExpirationThreshold is the duration after which completed sessions should be deleted.
	// Default: 7 days
	ExpirationThreshold time.Duration

	// OrphanThreshold is the duration after which disconnected agents are considered orphaned.
	// Default: 15 minutes
	OrphanThreshold time.Duration

	// MaxSessionAge is the maximum age for any session regardless of status.
	// Sessions older than this are always cleaned up.
	// Default: 30 days
	MaxSessionAge time.Duration

	// BatchSize is the maximum number of records to process in a single cleanup run.
	// Default: 100
	BatchSize int

	// DryRun when true, reports what would be cleaned without actually deleting.
	DryRun bool
}

// DefaultCleanupConfig returns a CleanupConfig with sensible defaults.
func DefaultCleanupConfig() *CleanupConfig {
	return &CleanupConfig{
		StaleThreshold:      30 * time.Minute,
		ExpirationThreshold: 7 * 24 * time.Hour,  // 7 days
		OrphanThreshold:     15 * time.Minute,
		MaxSessionAge:       30 * 24 * time.Hour, // 30 days
		BatchSize:           100,
		DryRun:              false,
	}
}

// CleanupResult contains the results of a cleanup operation.
type CleanupResult struct {
	// StaleSessions is the number of sessions marked as stale.
	StaleSessions int `json:"stale_sessions"`

	// ExpiredSessions is the number of expired sessions deleted.
	ExpiredSessions int `json:"expired_sessions"`

	// OrphanedAgents is the number of orphaned agent records cleaned up.
	OrphanedAgents int `json:"orphaned_agents"`

	// OrphanedTasks is the number of orphaned task records cleaned up.
	OrphanedTasks int `json:"orphaned_tasks"`

	// OrphanedMessages is the number of orphaned message records cleaned up.
	OrphanedMessages int `json:"orphaned_messages"`

	// OrphanedVariables is the number of orphaned variable records cleaned up.
	OrphanedVariables int `json:"orphaned_variables"`

	// Errors contains any errors encountered during cleanup.
	Errors []string `json:"errors,omitempty"`

	// Duration is how long the cleanup took.
	Duration time.Duration `json:"duration"`

	// StartedAt is when the cleanup started.
	StartedAt time.Time `json:"started_at"`

	// CompletedAt is when the cleanup completed.
	CompletedAt time.Time `json:"completed_at"`

	// DryRun indicates if this was a dry run.
	DryRun bool `json:"dry_run"`
}

// HasErrors returns true if any errors occurred during cleanup.
func (r *CleanupResult) HasErrors() bool {
	return len(r.Errors) > 0
}

// TotalCleaned returns the total number of records affected.
func (r *CleanupResult) TotalCleaned() int {
	return r.StaleSessions + r.ExpiredSessions + r.OrphanedAgents +
		r.OrphanedTasks + r.OrphanedMessages + r.OrphanedVariables
}

// SessionCleaner handles session cleanup operations.
type SessionCleaner struct {
	db     dbx.Builder
	config *CleanupConfig
	logger *slog.Logger
	mu     sync.Mutex
}

// NewSessionCleaner creates a new session cleaner.
func NewSessionCleaner(db dbx.Builder, config *CleanupConfig, logger *slog.Logger) *SessionCleaner {
	if config == nil {
		config = DefaultCleanupConfig()
	}
	if logger == nil {
		logger = slog.Default()
	}

	return &SessionCleaner{
		db:     db,
		config: config,
		logger: logger.With("component", "session_cleaner"),
	}
}

// RunCleanup executes all cleanup operations.
func (c *SessionCleaner) RunCleanup(ctx context.Context) (*CleanupResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	result := &CleanupResult{
		StartedAt: time.Now(),
		DryRun:    c.config.DryRun,
	}

	c.logger.Info("Starting session cleanup",
		"dry_run", c.config.DryRun,
		"stale_threshold", c.config.StaleThreshold,
		"expiration_threshold", c.config.ExpirationThreshold,
		"orphan_threshold", c.config.OrphanThreshold,
	)

	// Step 1: Mark stale sessions
	staleCount, err := c.markStaleSessions(ctx)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("mark stale sessions: %v", err))
		c.logger.Error("Failed to mark stale sessions", "error", err)
	} else {
		result.StaleSessions = staleCount
	}

	// Step 2: Delete expired sessions
	expiredCount, err := c.deleteExpiredSessions(ctx)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("delete expired sessions: %v", err))
		c.logger.Error("Failed to delete expired sessions", "error", err)
	} else {
		result.ExpiredSessions = expiredCount
	}

	// Step 3: Clean up orphaned agents
	orphanedAgents, err := c.cleanupOrphanedAgents(ctx)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("cleanup orphaned agents: %v", err))
		c.logger.Error("Failed to cleanup orphaned agents", "error", err)
	} else {
		result.OrphanedAgents = orphanedAgents
	}

	// Step 4: Clean up orphaned tasks
	orphanedTasks, err := c.cleanupOrphanedTasks(ctx)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("cleanup orphaned tasks: %v", err))
		c.logger.Error("Failed to cleanup orphaned tasks", "error", err)
	} else {
		result.OrphanedTasks = orphanedTasks
	}

	// Step 5: Clean up orphaned messages
	orphanedMessages, err := c.cleanupOrphanedMessages(ctx)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("cleanup orphaned messages: %v", err))
		c.logger.Error("Failed to cleanup orphaned messages", "error", err)
	} else {
		result.OrphanedMessages = orphanedMessages
	}

	// Step 6: Clean up orphaned variables
	orphanedVars, err := c.cleanupOrphanedVariables(ctx)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("cleanup orphaned variables: %v", err))
		c.logger.Error("Failed to cleanup orphaned variables", "error", err)
	} else {
		result.OrphanedVariables = orphanedVars
	}

	result.CompletedAt = time.Now()
	result.Duration = result.CompletedAt.Sub(result.StartedAt)

	c.logger.Info("Session cleanup completed",
		"duration", result.Duration,
		"stale_sessions", result.StaleSessions,
		"expired_sessions", result.ExpiredSessions,
		"orphaned_agents", result.OrphanedAgents,
		"orphaned_tasks", result.OrphanedTasks,
		"orphaned_messages", result.OrphanedMessages,
		"orphaned_variables", result.OrphanedVariables,
		"errors", len(result.Errors),
		"dry_run", result.DryRun,
	)

	return result, nil
}

// markStaleSessions finds active sessions without recent heartbeat and marks them as stale.
func (c *SessionCleaner) markStaleSessions(ctx context.Context) (int, error) {
	// Safety check: ensure database is initialized
	if c.db == nil {
		return 0, fmt.Errorf("database not initialized")
	}

	threshold := time.Now().Add(-c.config.StaleThreshold)

	// Find sessions that are active/paused but haven't had a heartbeat within threshold
	// Also consider sessions where last_heartbeat_at is NULL and updated_at is old
	var sessionIDs []string
	err := c.db.Select("id").
		From("sessions").
		Where(dbx.In("status", "active", "paused")).
		AndWhere(dbx.Or(
			dbx.And(dbx.Not(dbx.NewExp("last_heartbeat_at IS NULL")), dbx.NewExp("last_heartbeat_at < {:threshold}", dbx.Params{"threshold": threshold})),
			dbx.And(dbx.NewExp("last_heartbeat_at IS NULL"), dbx.NewExp("updated_at < {:threshold}", dbx.Params{"threshold": threshold})),
		)).
		Limit(int64(c.config.BatchSize)).
		Column(&sessionIDs)

	if err != nil {
		return 0, fmt.Errorf("query stale sessions: %w", err)
	}

	if len(sessionIDs) == 0 {
		return 0, nil
	}

	c.logger.Debug("Found stale sessions", "count", len(sessionIDs))

	if c.config.DryRun {
		return len(sessionIDs), nil
	}

	// Mark sessions as stale
	now := time.Now()
	
	result, err := c.db.Update("sessions", dbx.Params{
		"status":     SessionStale,
		"updated_at": now,
	}, dbx.In("id", stringSliceToInterfaceSlice(sessionIDs)...)).Execute()

	if err != nil {
		c.logger.Warn("Failed to mark sessions as stale", "error", err)
		return 0, err
	}
	
	affected, _ := result.RowsAffected()
	count := int(affected)
	
	c.logger.Info("Marked sessions as stale", "count", count)

	return count, nil
}

// deleteExpiredSessions removes sessions that have been in terminal state beyond expiration threshold.
func (c *SessionCleaner) deleteExpiredSessions(ctx context.Context) (int, error) {
	// Safety check: ensure database is initialized
	if c.db == nil {
		return 0, fmt.Errorf("database not initialized")
	}

	threshold := time.Now().Add(-c.config.ExpirationThreshold)
	maxAgeThreshold := time.Now().Add(-c.config.MaxSessionAge)

	var sessionIDs []string
	err := c.db.Select("id").
		From("sessions").
		Where(dbx.Or(
			dbx.And(
				dbx.In("status", "completed", "failed", "cancelled", "stale"),
				dbx.Not(dbx.NewExp("completed_at IS NULL")),
				dbx.NewExp("completed_at < {:threshold}", dbx.Params{"threshold": threshold}),
			),
			dbx.NewExp("created_at < {:maxAge}", dbx.Params{"maxAge": maxAgeThreshold}),
		)).
		Limit(int64(c.config.BatchSize)).
		Column(&sessionIDs)

	if err != nil {
		return 0, fmt.Errorf("query expired sessions: %w", err)
	}

	if len(sessionIDs) == 0 {
		return 0, nil
	}

	c.logger.Debug("Found expired sessions", "count", len(sessionIDs))

	if c.config.DryRun {
		return len(sessionIDs), nil
	}

	// Delete sessions
	result, err := c.db.Delete("sessions", dbx.In("id", stringSliceToInterfaceSlice(sessionIDs)...)).Execute()
	
	if err != nil {
		c.logger.Warn("Failed to delete expired sessions", "error", err)
		return 0, err
	}
	
	affected, _ := result.RowsAffected()
	count := int(affected)

	c.logger.Info("Deleted expired sessions", "count", count)

	return count, nil
}

// cleanupOrphanedAgents removes agent records whose parent session no longer exists
// or that have been disconnected beyond the orphan threshold.
func (c *SessionCleaner) cleanupOrphanedAgents(ctx context.Context) (int, error) {
	// Safety check: ensure database is initialized
	if c.db == nil {
		return 0, fmt.Errorf("database not initialized")
	}

	threshold := time.Now().Add(-c.config.OrphanThreshold)

	var agentIDs []string
	err := c.db.Select("a.id").
		From("session_agents a").
		LeftJoin("sessions s", dbx.NewExp("a.session_id = s.id")).
		Where(dbx.Or(
			dbx.NewExp("s.id IS NULL"),
			dbx.And(
				dbx.NewExp("a.status = 'disconnected'"),
				dbx.NewExp("a.last_activity_at < {:threshold}", dbx.Params{"threshold": threshold}),
			),
		)).
		Limit(int64(c.config.BatchSize)).
		Column(&agentIDs)

	if err != nil {
		return 0, fmt.Errorf("query orphaned agents: %w", err)
	}

	if len(agentIDs) == 0 {
		return 0, nil
	}

	c.logger.Debug("Found orphaned agents", "count", len(agentIDs))

	if c.config.DryRun {
		return len(agentIDs), nil
	}

	result, err := c.db.Delete("session_agents", dbx.In("id", stringSliceToInterfaceSlice(agentIDs)...)).Execute()
	
	if err != nil {
		c.logger.Warn("Failed to delete orphaned agents", "error", err)
		return 0, err
	}
	
	affected, _ := result.RowsAffected()
	count := int(affected)

	c.logger.Debug("Deleted orphaned agents", "count", count)

	return count, nil
}

// cleanupOrphanedTasks removes task records whose parent session no longer exists.
func (c *SessionCleaner) cleanupOrphanedTasks(ctx context.Context) (int, error) {
	// Safety check: ensure database is initialized
	if c.db == nil {
		return 0, fmt.Errorf("database not initialized")
	}

	var taskIDs []string
	err := c.db.Select("t.id").
		From("session_tasks t").
		LeftJoin("sessions s", dbx.NewExp("t.session_id = s.id")).
		Where(dbx.NewExp("s.id IS NULL")).
		Limit(int64(c.config.BatchSize)).
		Column(&taskIDs)

	if err != nil {
		return 0, fmt.Errorf("query orphaned tasks: %w", err)
	}

	if len(taskIDs) == 0 {
		return 0, nil
	}

	c.logger.Debug("Found orphaned tasks", "count", len(taskIDs))

	if c.config.DryRun {
		return len(taskIDs), nil
	}

	result, err := c.db.Delete("session_tasks", dbx.In("id", stringSliceToInterfaceSlice(taskIDs)...)).Execute()
	
	if err != nil {
		c.logger.Warn("Failed to delete orphaned tasks", "error", err)
		return 0, err
	}
	
	affected, _ := result.RowsAffected()
	count := int(affected)

	c.logger.Debug("Deleted orphaned tasks", "count", count)

	return count, nil
}

// cleanupOrphanedMessages removes message records whose parent session no longer exists.
func (c *SessionCleaner) cleanupOrphanedMessages(ctx context.Context) (int, error) {
	// Safety check: ensure database is initialized
	if c.db == nil {
		return 0, fmt.Errorf("database not initialized")
	}

	var messageIDs []string
	err := c.db.Select("m.id").
		From("session_messages m").
		LeftJoin("sessions s", dbx.NewExp("m.session_id = s.id")).
		Where(dbx.NewExp("s.id IS NULL")).
		Limit(int64(c.config.BatchSize)).
		Column(&messageIDs)

	if err != nil {
		return 0, fmt.Errorf("query orphaned messages: %w", err)
	}

	if len(messageIDs) == 0 {
		return 0, nil
	}

	c.logger.Debug("Found orphaned messages", "count", len(messageIDs))

	if c.config.DryRun {
		return len(messageIDs), nil
	}

	result, err := c.db.Delete("session_messages", dbx.In("id", stringSliceToInterfaceSlice(messageIDs)...)).Execute()
	
	if err != nil {
		c.logger.Warn("Failed to delete orphaned messages", "error", err)
		return 0, err
	}
	
	affected, _ := result.RowsAffected()
	count := int(affected)

	c.logger.Debug("Deleted orphaned messages", "count", count)

	return count, nil
}

// cleanupOrphanedVariables removes variable records whose parent session no longer exists.
func (c *SessionCleaner) cleanupOrphanedVariables(ctx context.Context) (int, error) {
	// Safety check: ensure database is initialized
	if c.db == nil {
		return 0, fmt.Errorf("database not initialized")
	}

	var varIDs []string
	err := c.db.Select("v.id").
		From("session_variables v").
		LeftJoin("sessions s", dbx.NewExp("v.session_id = s.id")).
		Where(dbx.NewExp("s.id IS NULL")).
		Limit(int64(c.config.BatchSize)).
		Column(&varIDs)

	if err != nil {
		return 0, fmt.Errorf("query orphaned variables: %w", err)
	}

	if len(varIDs) == 0 {
		return 0, nil
	}

	c.logger.Debug("Found orphaned variables", "count", len(varIDs))

	if c.config.DryRun {
		return len(varIDs), nil
	}

	result, err := c.db.Delete("session_variables", dbx.In("id", stringSliceToInterfaceSlice(varIDs)...)).Execute()
	
	if err != nil {
		c.logger.Warn("Failed to delete orphaned variables", "error", err)
		return 0, err
	}
	
	affected, _ := result.RowsAffected()
	count := int(affected)

	c.logger.Debug("Deleted orphaned variables", "count", count)

	return count, nil
}

// sessionRow is a helper struct for scanning session rows without the metadata JSON field.
type sessionRow struct {
	ID              string     `db:"id"`
	Name            string     `db:"name"`
	Status          string     `db:"status"`
	CoordinatorID   string     `db:"coordinator_id"`
	ParentSessionID *string    `db:"parent_session_id"`
	Description     *string    `db:"description"`
	CreatedAt       time.Time  `db:"created_at"`
	StartedAt       *time.Time `db:"started_at"`
	CompletedAt     *time.Time `db:"completed_at"`
	UpdatedAt       time.Time  `db:"updated_at"`
	LastHeartbeatAt *time.Time `db:"last_heartbeat_at"`
	MessageCount    int        `db:"message_count"`
	AgentCount      int        `db:"agent_count"`
	TaskCount       int        `db:"task_count"`
	ErrorCount      int        `db:"error_count"`
}

// rowToSessionState converts a sessionRow to a SessionState.
func rowToSessionState(row sessionRow) *SessionState {
	return &SessionState{
		ID:              row.ID,
		Name:            row.Name,
		Status:          SessionStatus(row.Status),
		CoordinatorID:   row.CoordinatorID,
		ParentSessionID: row.ParentSessionID,
		Description:     row.Description,
		CreatedAt:       row.CreatedAt,
		StartedAt:       row.StartedAt,
		CompletedAt:     row.CompletedAt,
		UpdatedAt:       row.UpdatedAt,
		LastHeartbeatAt: row.LastHeartbeatAt,
		MessageCount:    row.MessageCount,
		AgentCount:      row.AgentCount,
		TaskCount:       row.TaskCount,
		ErrorCount:      row.ErrorCount,
		Metadata:        make(map[string]interface{}),
	}
}

// FindStaleSessions returns sessions that haven't had activity within the stale threshold.
func (c *SessionCleaner) FindStaleSessions(ctx context.Context) ([]*SessionState, error) {
	threshold := time.Now().Add(-c.config.StaleThreshold)

	var rows []sessionRow
	err := c.db.Select(
		"id", "name", "status", "coordinator_id", "parent_session_id", "description",
		"created_at", "started_at", "completed_at", "updated_at", "last_heartbeat_at",
		"message_count", "agent_count", "task_count", "error_count",
	).
		From("sessions").
		Where(dbx.In("status", "active", "paused")).
		AndWhere(dbx.Or(
			dbx.And(dbx.Not(dbx.NewExp("last_heartbeat_at IS NULL")), dbx.NewExp("last_heartbeat_at < {:threshold}", dbx.Params{"threshold": threshold})),
			dbx.And(dbx.NewExp("last_heartbeat_at IS NULL"), dbx.NewExp("updated_at < {:threshold}", dbx.Params{"threshold": threshold})),
		)).
		OrderBy("updated_at ASC").
		Limit(int64(c.config.BatchSize)).
		All(&rows)

	if err != nil {
		return nil, fmt.Errorf("query sessions: %w", err)
	}

	sessions := make([]*SessionState, len(rows))
	for i, row := range rows {
		sessions[i] = rowToSessionState(row)
	}

	return sessions, nil
}

// FindExpiredSessions returns sessions that are eligible for deletion.
func (c *SessionCleaner) FindExpiredSessions(ctx context.Context) ([]*SessionState, error) {
	threshold := time.Now().Add(-c.config.ExpirationThreshold)
	maxAgeThreshold := time.Now().Add(-c.config.MaxSessionAge)

	var rows []sessionRow
	err := c.db.Select(
		"id", "name", "status", "coordinator_id", "parent_session_id", "description",
		"created_at", "started_at", "completed_at", "updated_at", "last_heartbeat_at",
		"message_count", "agent_count", "task_count", "error_count",
	).
		From("sessions").
		Where(dbx.Or(
			dbx.And(
				dbx.In("status", "completed", "failed", "cancelled", "stale"),
				dbx.Not(dbx.NewExp("completed_at IS NULL")),
				dbx.NewExp("completed_at < {:threshold}", dbx.Params{"threshold": threshold}),
			),
			dbx.NewExp("created_at < {:maxAge}", dbx.Params{"maxAge": maxAgeThreshold}),
		)).
		OrderBy("created_at ASC").
		Limit(int64(c.config.BatchSize)).
		All(&rows)

	if err != nil {
		return nil, fmt.Errorf("query sessions: %w", err)
	}

	sessions := make([]*SessionState, len(rows))
	for i, row := range rows {
		sessions[i] = rowToSessionState(row)
	}

	return sessions, nil
}

// stringSliceToInterfaceSlice converts []string to []interface{}
func stringSliceToInterfaceSlice(s []string) []interface{} {
	r := make([]interface{}, len(s))
	for i, v := range s {
		r[i] = v
	}
	return r
}

// SetConfig updates the cleanup configuration.
func (c *SessionCleaner) SetConfig(config *CleanupConfig) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.config = config
}

// GetConfig returns a copy of the current cleanup configuration.
func (c *SessionCleaner) GetConfig() CleanupConfig {
	c.mu.Lock()
	defer c.mu.Unlock()
	return *c.config
}

// CleanupScheduler runs periodic cleanup operations.
type CleanupScheduler struct {
	cleaner  *SessionCleaner
	interval time.Duration
	stopCh   chan struct{}
	doneCh   chan struct{}
	logger   *slog.Logger
	mu       sync.Mutex
	running  bool
}

// NewCleanupScheduler creates a new cleanup scheduler.
func NewCleanupScheduler(cleaner *SessionCleaner, interval time.Duration, logger *slog.Logger) *CleanupScheduler {
	if interval < time.Minute {
		interval = time.Minute // Minimum interval of 1 minute
	}
	if logger == nil {
		logger = slog.Default()
	}

	return &CleanupScheduler{
		cleaner:  cleaner,
		interval: interval,
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
		logger:   logger.With("component", "cleanup_scheduler"),
	}
}

// Start begins the cleanup scheduler.
func (s *CleanupScheduler) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("scheduler already running")
	}
	s.running = true
	s.stopCh = make(chan struct{})
	s.doneCh = make(chan struct{})
	s.mu.Unlock()

	s.logger.Info("Starting cleanup scheduler", "interval", s.interval)

	go s.run(ctx)
	return nil
}

// Stop stops the cleanup scheduler.
func (s *CleanupScheduler) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()

	s.logger.Info("Stopping cleanup scheduler")
	close(s.stopCh)
	<-s.doneCh

	s.mu.Lock()
	s.running = false
	s.mu.Unlock()

	s.logger.Info("Cleanup scheduler stopped")
}

// IsRunning returns true if the scheduler is running.
func (s *CleanupScheduler) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

func (s *CleanupScheduler) run(ctx context.Context) {
	defer close(s.doneCh)

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	// Run cleanup immediately on start
	s.runCleanup(ctx)

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("Cleanup scheduler context cancelled")
			return
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.runCleanup(ctx)
		}
	}
}

func (s *CleanupScheduler) runCleanup(ctx context.Context) {
	result, err := s.cleaner.RunCleanup(ctx)
	if err != nil {
		s.logger.Error("Cleanup failed", "error", err)
		return
	}

	if result.TotalCleaned() > 0 || result.HasErrors() {
		s.logger.Info("Cleanup cycle completed",
			"total_cleaned", result.TotalCleaned(),
			"duration", result.Duration,
			"errors", len(result.Errors),
		)
	}
}