package session

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/pocketbase/dbx"
	_ "github.com/mattn/go-sqlite3"
)

func setupTestDB(t *testing.T) *dbx.DB {
	t.Helper()

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	dbxDB := dbx.NewFromDB(db, "sqlite3")

	// Initialize schema
	if _, err := dbxDB.NewQuery(PocketBaseCollections).Execute(); err != nil {
		t.Fatalf("Failed to initialize schema: %v", err)
	}

	return dbxDB
}

func insertTestSession(t *testing.T, db *dbx.DB, id, name string, status SessionStatus, createdAt, updatedAt, lastHeartbeat, completedAt *time.Time) {
	t.Helper()

	query := `
		INSERT INTO sessions (id, name, status, coordinator_id, created_at, updated_at, last_heartbeat_at, completed_at)
		VALUES ({:id}, {:name}, {:status}, 'test-coordinator', {:createdAt}, {:updatedAt}, {:lastHeartbeat}, {:completedAt})
	`

	_, err := db.NewQuery(query).Bind(dbx.Params{
		"id":            id,
		"name":          name,
		"status":        status,
		"createdAt":     createdAt,
		"updatedAt":     updatedAt,
		"lastHeartbeat": lastHeartbeat,
		"completedAt":   completedAt,
	}).Execute()

	if err != nil {
		t.Fatalf("Failed to insert test session: %v", err)
	}
}

func insertTestAgent(t *testing.T, db *dbx.DB, id, sessionID, agentID string, status AgentStatus, lastActivity *time.Time) {
	t.Helper()

	query := `
		INSERT INTO session_agents (id, session_id, agent_id, name, model, role, status, joined_at, last_activity_at)
		VALUES ({:id}, {:sessionID}, {:agentID}, 'test-agent', 'claude', 'worker', {:status}, datetime('now'), {:lastActivity})
	`

	_, err := db.NewQuery(query).Bind(dbx.Params{
		"id":           id,
		"sessionID":    sessionID,
		"agentID":      agentID,
		"status":       status,
		"lastActivity": lastActivity,
	}).Execute()

	if err != nil {
		t.Fatalf("Failed to insert test agent: %v", err)
	}
}

func insertTestMessage(t *testing.T, db *dbx.DB, id, sessionID string) {
	t.Helper()

	query := `
		INSERT INTO session_messages (id, session_id, role, sender_id, content, message_type, status)
		VALUES ({:id}, {:sessionID}, 'agent', 'sender-1', 'test message', 'status', 'pending')
	`

	_, err := db.NewQuery(query).Bind(dbx.Params{
		"id":        id,
		"sessionID": sessionID,
	}).Execute()

	if err != nil {
		t.Fatalf("Failed to insert test message: %v", err)
	}
}

func insertTestTask(t *testing.T, db *dbx.DB, id, sessionID string) {
	t.Helper()

	query := `
		INSERT INTO session_tasks (id, session_id, assigned_agent_id, title, status)
		VALUES ({:id}, {:sessionID}, 'agent-1', 'test task', 'pending')
	`

	_, err := db.NewQuery(query).Bind(dbx.Params{
		"id":        id,
		"sessionID": sessionID,
	}).Execute()

	if err != nil {
		t.Fatalf("Failed to insert test task: %v", err)
	}
}

func insertTestVariable(t *testing.T, db *dbx.DB, id, sessionID, key string) {
	t.Helper()

	query := `
		INSERT INTO session_variables (id, session_id, key, value)
		VALUES ({:id}, {:sessionID}, {:key}, '"test"')
	`

	_, err := db.NewQuery(query).Bind(dbx.Params{
		"id":        id,
		"sessionID": sessionID,
		"key":       key,
	}).Execute()

	if err != nil {
		t.Fatalf("Failed to insert test variable: %v", err)
	}
}

func countRows(t *testing.T, db *dbx.DB, table string) int {
	t.Helper()

	var count int
	err := db.NewQuery("SELECT COUNT(*) FROM " + table).Row(&count)
	if err != nil {
		t.Fatalf("Failed to count rows in %s: %v", table, err)
	}
	return count
}

func TestDefaultCleanupConfig(t *testing.T) {
	config := DefaultCleanupConfig()

	if config.StaleThreshold != 30*time.Minute {
		t.Errorf("Expected StaleThreshold=30m, got %v", config.StaleThreshold)
	}

	if config.ExpirationThreshold != 7*24*time.Hour {
		t.Errorf("Expected ExpirationThreshold=7d, got %v", config.ExpirationThreshold)
	}

	if config.OrphanThreshold != 15*time.Minute {
		t.Errorf("Expected OrphanThreshold=15m, got %v", config.OrphanThreshold)
	}

	if config.MaxSessionAge != 30*24*time.Hour {
		t.Errorf("Expected MaxSessionAge=30d, got %v", config.MaxSessionAge)
	}

	if config.BatchSize != 100 {
		t.Errorf("Expected BatchSize=100, got %d", config.BatchSize)
	}

	if config.DryRun {
		t.Errorf("Expected DryRun=false, got true")
	}
}

func TestCleanupResult(t *testing.T) {
	result := &CleanupResult{
		StaleSessions:     5,
		ExpiredSessions:   3,
		OrphanedAgents:    2,
		OrphanedTasks:     1,
		OrphanedMessages:  10,
		OrphanedVariables: 4,
	}

	if result.TotalCleaned() != 25 {
		t.Errorf("Expected TotalCleaned=25, got %d", result.TotalCleaned())
	}

	if result.HasErrors() {
		t.Errorf("Expected HasErrors=false when no errors")
	}

	result.Errors = append(result.Errors, "test error")
	if !result.HasErrors() {
		t.Errorf("Expected HasErrors=true when errors exist")
	}
}

func TestMarkStaleSessions(t *testing.T) {
	db := setupTestDB(t)
	// dbx.DB doesn't need explicit Close() in tests as it uses sql.DB which is closed when GC'd or we can close the underlying DB
	defer db.DB().Close()

	now := time.Now()
	old := now.Add(-1 * time.Hour)

	// Insert sessions with different states
	insertTestSession(t, db, "active-recent", "Active Recent", SessionActive, &now, &now, &now, nil)
	insertTestSession(t, db, "active-old", "Active Old", SessionActive, &old, &old, &old, nil)
	insertTestSession(t, db, "paused-old", "Paused Old", SessionPaused, &old, &old, &old, nil)
	insertTestSession(t, db, "completed-old", "Completed Old", SessionCompleted, &old, &old, &old, &old)

	config := &CleanupConfig{
		StaleThreshold: 30 * time.Minute,
		BatchSize:      100,
		DryRun:         false,
	}

	cleaner := NewSessionCleaner(db, config, nil)
	count, err := cleaner.markStaleSessions(context.Background())

	if err != nil {
		t.Fatalf("markStaleSessions failed: %v", err)
	}

	// Should mark 2 sessions as stale (active-old, paused-old)
	if count != 2 {
		t.Errorf("Expected 2 stale sessions, got %d", count)
	}

	// Verify sessions are marked as stale
	var status string
	err = db.NewQuery("SELECT status FROM sessions WHERE id = {:id}").Bind(dbx.Params{"id": "active-old"}).Row(&status)
	if err != nil {
		t.Fatalf("Failed to query session status: %v", err)
	}
	if status != string(SessionStale) {
		t.Errorf("Expected session status=stale, got %s", status)
	}

	// Active recent should still be active
	err = db.NewQuery("SELECT status FROM sessions WHERE id = {:id}").Bind(dbx.Params{"id": "active-recent"}).Row(&status)
	if err != nil {
		t.Fatalf("Failed to query session status: %v", err)
	}
	if status != string(SessionActive) {
		t.Errorf("Expected session status=active, got %s", status)
	}
}

func TestDeleteExpiredSessions(t *testing.T) {
	db := setupTestDB(t)
	defer db.DB().Close()

	now := time.Now()
	oldCompleted := now.Add(-10 * 24 * time.Hour) // 10 days ago
	recentCompleted := now.Add(-2 * 24 * time.Hour) // 2 days ago

	// Insert sessions
	insertTestSession(t, db, "completed-old", "Completed Old", SessionCompleted, &oldCompleted, &oldCompleted, nil, &oldCompleted)
	insertTestSession(t, db, "completed-recent", "Completed Recent", SessionCompleted, &recentCompleted, &recentCompleted, nil, &recentCompleted)
	insertTestSession(t, db, "active", "Active", SessionActive, &now, &now, &now, nil)

	config := &CleanupConfig{
		ExpirationThreshold: 7 * 24 * time.Hour, // 7 days
		MaxSessionAge:       30 * 24 * time.Hour,
		BatchSize:           100,
		DryRun:              false,
	}

	cleaner := NewSessionCleaner(db, config, nil)
	count, err := cleaner.deleteExpiredSessions(context.Background())

	if err != nil {
		t.Fatalf("deleteExpiredSessions failed: %v", err)
	}

	// Should delete 1 session (completed-old)
	if count != 1 {
		t.Errorf("Expected 1 deleted session, got %d", count)
	}

	// Verify session count
	remaining := countRows(t, db, "sessions")
	if remaining != 2 {
		t.Errorf("Expected 2 remaining sessions, got %d", remaining)
	}
}

func TestCleanupOrphanedAgents(t *testing.T) {
	db := setupTestDB(t)
	defer db.DB().Close()

	now := time.Now()
	old := now.Add(-1 * time.Hour)

	// Insert a session
	insertTestSession(t, db, "existing-session", "Existing Session", SessionActive, &now, &now, &now, nil)

	// Insert agents - one valid, one orphaned (no session), one disconnected old
	insertTestAgent(t, db, "agent-1", "existing-session", "agent-001", AgentStatusActive, &now)
	insertTestAgent(t, db, "agent-2", "nonexistent-session", "agent-002", AgentStatusActive, &now)
	insertTestAgent(t, db, "agent-3", "existing-session", "agent-003", AgentStatusDisconnected, &old)

	config := &CleanupConfig{
		OrphanThreshold: 30 * time.Minute,
		BatchSize:       100,
		DryRun:          false,
	}

	cleaner := NewSessionCleaner(db, config, nil)
	count, err := cleaner.cleanupOrphanedAgents(context.Background())

	if err != nil {
		t.Fatalf("cleanupOrphanedAgents failed: %v", err)
	}

	// Should delete 2 agents (orphaned + disconnected old)
	if count != 2 {
		t.Errorf("Expected 2 deleted agents, got %d", count)
	}

	// Verify agent count
	remaining := countRows(t, db, "session_agents")
	if remaining != 1 {
		t.Errorf("Expected 1 remaining agent, got %d", remaining)
	}
}

func TestCleanupOrphanedRecords(t *testing.T) {
	db := setupTestDB(t)
	defer db.DB().Close()

	now := time.Now()

	// Insert a session
	insertTestSession(t, db, "existing-session", "Existing Session", SessionActive, &now, &now, &now, nil)

	// Insert records - some valid, some orphaned
	insertTestMessage(t, db, "msg-1", "existing-session")
	insertTestMessage(t, db, "msg-2", "nonexistent-session")
	insertTestTask(t, db, "task-1", "existing-session")
	insertTestTask(t, db, "task-2", "nonexistent-session")
	insertTestVariable(t, db, "var-1", "existing-session", "key1")
	insertTestVariable(t, db, "var-2", "nonexistent-session", "key2")

	config := DefaultCleanupConfig()
	config.DryRun = false

	cleaner := NewSessionCleaner(db, config, nil)

	// Test messages
	msgCount, err := cleaner.cleanupOrphanedMessages(context.Background())
	if err != nil {
		t.Fatalf("cleanupOrphanedMessages failed: %v", err)
	}
	if msgCount != 1 {
		t.Errorf("Expected 1 deleted message, got %d", msgCount)
	}

	// Test tasks
	taskCount, err := cleaner.cleanupOrphanedTasks(context.Background())
	if err != nil {
		t.Fatalf("cleanupOrphanedTasks failed: %v", err)
	}
	if taskCount != 1 {
		t.Errorf("Expected 1 deleted task, got %d", taskCount)
	}

	// Test variables
	varCount, err := cleaner.cleanupOrphanedVariables(context.Background())
	if err != nil {
		t.Fatalf("cleanupOrphanedVariables failed: %v", err)
	}
	if varCount != 1 {
		t.Errorf("Expected 1 deleted variable, got %d", varCount)
	}
}

func TestDryRun(t *testing.T) {
	db := setupTestDB(t)
	defer db.DB().Close()

	now := time.Now()
	old := now.Add(-1 * time.Hour)

	// Insert sessions that would be marked as stale
	insertTestSession(t, db, "active-old", "Active Old", SessionActive, &old, &old, &old, nil)

	config := &CleanupConfig{
		StaleThreshold: 30 * time.Minute,
		BatchSize:      100,
		DryRun:         true, // Dry run enabled
	}

	cleaner := NewSessionCleaner(db, config, nil)
	count, err := cleaner.markStaleSessions(context.Background())

	if err != nil {
		t.Fatalf("markStaleSessions failed: %v", err)
	}

	// Should report 1 stale session
	if count != 1 {
		t.Errorf("Expected 1 stale session (dry run), got %d", count)
	}

	// Session should still be active (dry run)
	var status string
	err = db.NewQuery("SELECT status FROM sessions WHERE id = {:id}").Bind(dbx.Params{"id": "active-old"}).Row(&status)
	if err != nil {
		t.Fatalf("Failed to query session status: %v", err)
	}
	if status != string(SessionActive) {
		t.Errorf("Expected session status=active (dry run), got %s", status)
	}
}

func TestRunCleanup(t *testing.T) {
	db := setupTestDB(t)
	defer db.DB().Close()

	now := time.Now()
	old := now.Add(-1 * time.Hour)
	veryOld := now.Add(-10 * 24 * time.Hour)

	// Insert various sessions and related records
	insertTestSession(t, db, "active-old", "Active Old", SessionActive, &old, &old, &old, nil)
	insertTestSession(t, db, "completed-old", "Completed Old", SessionCompleted, &veryOld, &veryOld, nil, &veryOld)
	insertTestSession(t, db, "active-recent", "Active Recent", SessionActive, &now, &now, &now, nil)

	// Insert orphaned records
	insertTestMessage(t, db, "orphan-msg", "nonexistent")
	insertTestTask(t, db, "orphan-task", "nonexistent")
	insertTestVariable(t, db, "orphan-var", "nonexistent", "key")

	config := &CleanupConfig{
		StaleThreshold:      30 * time.Minute,
		ExpirationThreshold: 7 * 24 * time.Hour,
		OrphanThreshold:     15 * time.Minute,
		MaxSessionAge:       30 * 24 * time.Hour,
		BatchSize:           100,
		DryRun:              false,
	}

	cleaner := NewSessionCleaner(db, config, nil)
	result, err := cleaner.RunCleanup(context.Background())

	if err != nil {
		t.Fatalf("RunCleanup failed: %v", err)
	}

	// Verify results
	if result.StaleSessions != 1 {
		t.Errorf("Expected 1 stale session, got %d", result.StaleSessions)
	}

	if result.ExpiredSessions != 1 {
		t.Errorf("Expected 1 expired session, got %d", result.ExpiredSessions)
	}

	if result.OrphanedMessages != 1 {
		t.Errorf("Expected 1 orphaned message, got %d", result.OrphanedMessages)
	}

	if result.OrphanedTasks != 1 {
		t.Errorf("Expected 1 orphaned task, got %d", result.OrphanedTasks)
	}

	if result.OrphanedVariables != 1 {
		t.Errorf("Expected 1 orphaned variable, got %d", result.OrphanedVariables)
	}

	if result.Duration <= 0 {
		t.Errorf("Expected positive duration, got %v", result.Duration)
	}

	if result.HasErrors() {
		t.Errorf("Expected no errors, got %v", result.Errors)
	}
}

func TestFindStaleSessions(t *testing.T) {
	db := setupTestDB(t)
	defer db.DB().Close()

	now := time.Now()
	old := now.Add(-1 * time.Hour)

	insertTestSession(t, db, "active-recent", "Active Recent", SessionActive, &now, &now, &now, nil)
	insertTestSession(t, db, "active-old", "Active Old", SessionActive, &old, &old, &old, nil)

	config := &CleanupConfig{
		StaleThreshold: 30 * time.Minute,
		BatchSize:      100,
	}

	cleaner := NewSessionCleaner(db, config, nil)
	sessions, err := cleaner.FindStaleSessions(context.Background())

	if err != nil {
		t.Fatalf("FindStaleSessions failed: %v", err)
	}

	if len(sessions) != 1 {
		t.Errorf("Expected 1 stale session, got %d", len(sessions))
	}

	if len(sessions) > 0 && sessions[0].ID != "active-old" {
		t.Errorf("Expected session id=active-old, got %s", sessions[0].ID)
	}
}

func TestFindExpiredSessions(t *testing.T) {
	db := setupTestDB(t)
	defer db.DB().Close()

	now := time.Now()
	old := now.Add(-10 * 24 * time.Hour)
	recent := now.Add(-2 * 24 * time.Hour)

	insertTestSession(t, db, "completed-old", "Completed Old", SessionCompleted, &old, &old, nil, &old)
	insertTestSession(t, db, "completed-recent", "Completed Recent", SessionCompleted, &recent, &recent, nil, &recent)

	config := &CleanupConfig{
		ExpirationThreshold: 7 * 24 * time.Hour,
		MaxSessionAge:       30 * 24 * time.Hour,
		BatchSize:           100,
	}

	cleaner := NewSessionCleaner(db, config, nil)
	sessions, err := cleaner.FindExpiredSessions(context.Background())

	if err != nil {
		t.Fatalf("FindExpiredSessions failed: %v", err)
	}

	if len(sessions) != 1 {
		t.Errorf("Expected 1 expired session, got %d", len(sessions))
	}

	if len(sessions) > 0 && sessions[0].ID != "completed-old" {
		t.Errorf("Expected session id=completed-old, got %s", sessions[0].ID)
	}
}

func TestCleanupScheduler(t *testing.T) {
	db := setupTestDB(t)
	defer db.DB().Close()

	config := DefaultCleanupConfig()
	cleaner := NewSessionCleaner(db, config, nil)

	scheduler := NewCleanupScheduler(cleaner, 100*time.Millisecond, nil)

	if scheduler.IsRunning() {
		t.Errorf("Scheduler should not be running initially")
	}

	ctx := context.Background()
	err := scheduler.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}

	if !scheduler.IsRunning() {
		t.Errorf("Scheduler should be running after Start()")
	}

	// Try to start again - should fail
	err = scheduler.Start(ctx)
	if err == nil {
		t.Errorf("Starting already running scheduler should return error")
	}

	// Let it run a few cycles
	time.Sleep(350 * time.Millisecond)

	scheduler.Stop()

	if scheduler.IsRunning() {
		t.Errorf("Scheduler should not be running after Stop()")
	}
}

func TestSetGetConfig(t *testing.T) {
	db := setupTestDB(t)
	defer db.DB().Close()

	config := DefaultCleanupConfig()
	cleaner := NewSessionCleaner(db, config, nil)

	// Get initial config
	initialConfig := cleaner.GetConfig()
	if initialConfig.StaleThreshold != 30*time.Minute {
		t.Errorf("Expected initial StaleThreshold=30m, got %v", initialConfig.StaleThreshold)
	}

	// Set new config
	newConfig := &CleanupConfig{
		StaleThreshold:      1 * time.Hour,
		ExpirationThreshold: 14 * 24 * time.Hour,
		OrphanThreshold:     30 * time.Minute,
		MaxSessionAge:       60 * 24 * time.Hour,
		BatchSize:           200,
		DryRun:              true,
	}
	cleaner.SetConfig(newConfig)

	// Verify new config
	updatedConfig := cleaner.GetConfig()
	if updatedConfig.StaleThreshold != 1*time.Hour {
		t.Errorf("Expected updated StaleThreshold=1h, got %v", updatedConfig.StaleThreshold)
	}
	if updatedConfig.BatchSize != 200 {
		t.Errorf("Expected updated BatchSize=200, got %d", updatedConfig.BatchSize)
	}
	if !updatedConfig.DryRun {
		t.Errorf("Expected updated DryRun=true, got false")
	}
}
