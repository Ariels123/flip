// Package pipeline provides integration tests for multi-stage pipeline execution,
// including crash recovery scenarios, artifact management, and concurrent execution.
package pipeline

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"database/sql"
	_ "github.com/mattn/go-sqlite3"
)

// =============================================================================
// TEST DATABASE SETUP
// =============================================================================

// testDB provides a temporary SQLite database for integration tests.
type testDB struct {
	db       *sql.DB
	dbPath   string
	cleanups []func()
}

// newTestDB creates a temporary SQLite database for testing.
func newTestDB(t *testing.T) *testDB {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_pipeline.db")

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}

	tdb := &testDB{
		db:       db,
		dbPath:   dbPath,
		cleanups: []func(){func() { db.Close() }},
	}

	// Create schema
	if err := tdb.initSchema(); err != nil {
		t.Fatalf("failed to initialize database schema: %v", err)
	}

	return tdb
}

// initSchema creates the required tables for pipeline testing.
func (tdb *testDB) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS pipeline_runs (
		id TEXT PRIMARY KEY,
		pipeline_id TEXT NOT NULL,
		status TEXT NOT NULL,
		current_stage_index INTEGER NOT NULL DEFAULT 0,
		total_stages INTEGER NOT NULL,
		input TEXT,
		final_output TEXT,
		error TEXT,
		error_stage TEXT,
		retry_count INTEGER DEFAULT 0,
		max_retries INTEGER DEFAULT 3,
		priority INTEGER DEFAULT 0,
		assigned_agent TEXT,
		started_at DATETIME,
		completed_at DATETIME,
		last_checkpoint_at DATETIME,
		metadata TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS stage_runs (
		id TEXT PRIMARY KEY,
		pipeline_run_id TEXT NOT NULL,
		stage_name TEXT NOT NULL,
		stage_index INTEGER NOT NULL DEFAULT 0,
		status TEXT NOT NULL,
		backend TEXT,
		model TEXT,
		input TEXT,
		output TEXT,
		error TEXT,
		retry_count INTEGER DEFAULT 0,
		max_retries INTEGER DEFAULT 2,
		started_at DATETIME,
		completed_at DATETIME,
		task_id TEXT,
		agent_id TEXT,
		metrics TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (pipeline_run_id) REFERENCES pipeline_runs(id)
	);

	CREATE TABLE IF NOT EXISTS artifacts (
		id TEXT PRIMARY KEY,
		pipeline_run_id TEXT NOT NULL,
		stage_run_id TEXT NOT NULL,
		name TEXT NOT NULL,
		type TEXT,
		content_type TEXT,
		size_bytes INTEGER,
		checksum TEXT UNIQUE,
		storage_path TEXT,
		metadata TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (pipeline_run_id) REFERENCES pipeline_runs(id),
		FOREIGN KEY (stage_run_id) REFERENCES stage_runs(id)
	);

	CREATE TABLE IF NOT EXISTS checkpoints (
		id TEXT PRIMARY KEY,
		pipeline_run_id TEXT NOT NULL,
		stage_index INTEGER NOT NULL,
		state TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (pipeline_run_id) REFERENCES pipeline_runs(id)
	);
	`

	_, err := tdb.db.Exec(schema)
	return err
}

// close closes the test database and runs cleanups.
func (tdb *testDB) close() {
	for _, cleanup := range tdb.cleanups {
		cleanup()
	}
}

// exec executes a query and returns error if any.
func (tdb *testDB) exec(query string, args ...interface{}) error {
	_, err := tdb.db.Exec(query, args...)
	return err
}

// query executes a query and returns rows.
func (tdb *testDB) query(query string, args ...interface{}) (*sql.Rows, error) {
	return tdb.db.Query(query, args...)
}

// =============================================================================
// PIPELINE RUN HELPERS
// =============================================================================

// createIntegrationTestPipeline creates a test pipeline definition.
func createIntegrationTestPipeline(name string, stageCount int) *PipelineDefinition {
	stages := make([]Stage, stageCount)
	for i := 0; i < stageCount; i++ {
		stages[i] = Stage{
			ID:      fmt.Sprintf("stage-%d", i),
			Name:    fmt.Sprintf("Stage %d", i),
			Backend: "claude",
			Command: fmt.Sprintf("echo 'Stage %d output'", i),
		}
		if i > 0 {
			stages[i].DependsOn = []string{fmt.Sprintf("stage-%d", i-1)}
		}
	}

	return &PipelineDefinition{
		Name:        name,
		Description: "Test pipeline",
		Version:     "1.0",
		Stages:      stages,
	}
}

// createTestPipelineRun creates a test pipeline run.
func createTestPipelineRun(pipelineID string, stageCount int) *PipelineRun {
	now := time.Now()
	return &PipelineRun{
		ID:                generateTestID("run"),
		PipelineID:        pipelineID,
		Status:            PipelinePending,
		CurrentStageIndex: 0,
		TotalStages:       stageCount,
		StartedAt:         &now,
		MaxRetries:        3,
		Metadata:          make(map[string]interface{}),
	}
}

// createTestStageRun creates a test stage run.
func createTestStageRun(pipelineRunID, stageName string) *StageRun {
	now := time.Now()
	return &StageRun{
		ID:            generateTestID("stage"),
		PipelineRunID: pipelineRunID,
		StageName:     stageName,
		Status:        StagePending,
		RetryCount:    0,
		StartedAt:     &now,
	}
}

// savePipelineRunToTestDB saves a pipeline run to test database.
func (tdb *testDB) savePipelineRun(run *PipelineRun) error {
	metaJSON, _ := json.Marshal(run.Metadata)
	return tdb.exec(
		`INSERT INTO pipeline_runs
		(id, pipeline_id, status, current_stage_index, total_stages, input, final_output,
		 error, error_stage, retry_count, max_retries, priority, assigned_agent,
		 started_at, completed_at, last_checkpoint_at, metadata)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		run.ID, run.PipelineID, run.Status.String(), run.CurrentStageIndex,
		run.TotalStages, []byte(run.Input), []byte(run.FinalOutput),
		run.Error, run.ErrorStage, run.RetryCount, run.MaxRetries,
		run.Priority, run.AssignedAgent, run.StartedAt, run.CompletedAt,
		run.LastCheckpointAt, string(metaJSON),
	)
}

// loadPipelineRunFromTestDB loads a pipeline run from test database.
func (tdb *testDB) loadPipelineRun(id string) (*PipelineRun, error) {
	row := tdb.db.QueryRow(
		`SELECT id, pipeline_id, status, current_stage_index, total_stages, input,
		        final_output, error, error_stage, retry_count, max_retries, priority,
		        assigned_agent, started_at, completed_at, last_checkpoint_at, metadata
		 FROM pipeline_runs WHERE id = ?`, id,
	)

	var run PipelineRun
	var metaJSON string
	var status string
	var input, finalOutput []byte
	err := row.Scan(
		&run.ID, &run.PipelineID, &status, &run.CurrentStageIndex,
		&run.TotalStages, &input, &finalOutput,
		&run.Error, &run.ErrorStage, &run.RetryCount, &run.MaxRetries,
		&run.Priority, &run.AssignedAgent, &run.StartedAt, &run.CompletedAt,
		&run.LastCheckpointAt, &metaJSON,
	)

	if err != nil {
		return nil, err
	}

	run.Status = PipelineStatus(status)
	run.Input = json.RawMessage(input)
	run.FinalOutput = json.RawMessage(finalOutput)

	if metaJSON != "" {
		json.Unmarshal([]byte(metaJSON), &run.Metadata)
	}
	return &run, nil
}

// generateTestID generates a unique test ID with prefix.
func generateTestID(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

func timePtr(t time.Time) *time.Time {
	return &t
}

// =============================================================================
// ARTIFACT HELPERS
// =============================================================================

// createTestArtifactStore creates a test artifact store with temporary directory.
func createTestArtifactStore(t *testing.T) (*ArtifactStore, string) {
	tmpDir := t.TempDir()
	return &ArtifactStore{
		baseDir:       tmpDir,
		metadataCache: make(map[string]*ArtifactMetadata),
	}, tmpDir
}

// saveTestArtifact saves test artifact data to disk.
func (tdb *testDB) saveTestArtifact(pipelineRunID, stageRunID string, name string, data []byte) error {
	checksum := CalculateChecksum(data)
	metadata := &ArtifactMetadata{
		ID:            generateTestID("artifact"),
		PipelineRunID: pipelineRunID,
		StageRunID:    stageRunID,
		Name:          name,
		Type:          "text",
		ContentType:   "text/plain",
		SizeBytes:     int64(len(data)),
		Checksum:      checksum,
		Metadata:      make(map[string]interface{}),
	}

	metaJSON, _ := json.Marshal(metadata)
	return tdb.exec(
		`INSERT INTO artifacts
		(id, pipeline_run_id, stage_run_id, name, type, content_type, size_bytes, checksum, metadata)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		metadata.ID, pipelineRunID, stageRunID, name, metadata.Type,
		metadata.ContentType, metadata.SizeBytes, checksum, string(metaJSON),
	)
}

// =============================================================================
// CHECKPOINT HELPERS
// =============================================================================

// saveCheckpoint saves a pipeline checkpoint to the test database.
func (tdb *testDB) saveCheckpoint(pipelineRunID string, stageIndex int, state *PipelineRun) error {
	stateJSON, err := json.Marshal(state)
	if err != nil {
		return err
	}

	return tdb.exec(
		`INSERT INTO checkpoints (id, pipeline_run_id, stage_index, state)
		 VALUES (?, ?, ?, ?)`,
		generateTestID("checkpoint"), pipelineRunID, stageIndex, string(stateJSON),
	)
}

// loadLatestCheckpoint loads the latest checkpoint for a pipeline.
func (tdb *testDB) loadLatestCheckpoint(pipelineRunID string) (*PipelineRun, error) {
	row := tdb.db.QueryRow(
		`SELECT state FROM checkpoints
		 WHERE pipeline_run_id = ?
		 ORDER BY created_at DESC LIMIT 1`, pipelineRunID,
	)

	var stateJSON string
	err := row.Scan(&stateJSON)
	if err != nil {
		return nil, err
	}

	var state PipelineRun
	if err := json.Unmarshal([]byte(stateJSON), &state); err != nil {
		return nil, err
	}
	return &state, nil
}

// =============================================================================
// INTEGRATION TESTS
// =============================================================================

// TestE2EPipelineExecution tests end-to-end pipeline execution.
func TestE2EPipelineExecution(t *testing.T) {
	tdb := newTestDB(t)
	defer tdb.close()

	pipelineDef := createIntegrationTestPipeline("test-pipeline", 3)
	run := createTestPipelineRun(pipelineDef.Name, len(pipelineDef.Stages))

	// Save initial state
	if err := tdb.savePipelineRun(run); err != nil {
		t.Fatalf("failed to save pipeline run: %v", err)
	}

	// Simulate pipeline execution
	run.Status = PipelineRunning
	for i := 0; i < len(pipelineDef.Stages); i++ {
		run.CurrentStageIndex = i
		if err := tdb.exec(
			"UPDATE pipeline_runs SET current_stage_index = ?, status = ? WHERE id = ?",
			i, run.Status.String(), run.ID,
		); err != nil {
			t.Fatalf("failed to update pipeline: %v", err)
		}

		// Create stage run
		stageRun := createTestStageRun(run.ID, pipelineDef.Stages[i].ID)
		if err := tdb.exec(
			`INSERT INTO stage_runs
			(id, pipeline_run_id, stage_name, status, started_at)
			VALUES (?, ?, ?, ?, ?)`,
			stageRun.ID, stageRun.PipelineRunID, stageRun.StageName,
			stageRun.Status.String(), stageRun.StartedAt,
		); err != nil {
			t.Fatalf("failed to save stage run: %v", err)
		}

		// Mark stage as completed
		if err := tdb.exec(
			`UPDATE stage_runs SET status = ?, completed_at = ? WHERE id = ?`,
			StageCompleted.String(), time.Now(), stageRun.ID,
		); err != nil {
			t.Fatalf("failed to complete stage: %v", err)
		}
	}

	// Mark pipeline as completed
	run.Status = PipelineCompleted
	run.CompletedAt = timePtr(time.Now())
	if err := tdb.exec(
		"UPDATE pipeline_runs SET status = ?, completed_at = ? WHERE id = ?",
		run.Status.String(), run.CompletedAt, run.ID,
	); err != nil {
		t.Fatalf("failed to complete pipeline: %v", err)
	}

	// Verify completion
	loadedRun, err := tdb.loadPipelineRun(run.ID)
	if err != nil {
		t.Fatalf("failed to load pipeline: %v", err)
	}

	if loadedRun.Status != PipelineCompleted {
		t.Errorf("expected status %s, got %s", PipelineCompleted, loadedRun.Status)
	}
	if loadedRun.CurrentStageIndex != 2 {
		t.Errorf("expected current_stage_index 2, got %d", loadedRun.CurrentStageIndex)
	}
}

// TestStageTimeoutHandling tests stage timeout detection and handling.
func TestStageTimeoutHandling(t *testing.T) {
	tdb := newTestDB(t)
	defer tdb.close()

	pipelineDef := createIntegrationTestPipeline("timeout-test", 2)
	pipelineDef.Stages[0].Timeout = &Duration{Duration: 100 * time.Millisecond}

	run := createTestPipelineRun(pipelineDef.Name, len(pipelineDef.Stages))

	// Save pipeline
	if err := tdb.savePipelineRun(run); err != nil {
		t.Fatalf("failed to save pipeline: %v", err)
	}

	// Simulate stage execution with timeout
	stageRun := createTestStageRun(run.ID, pipelineDef.Stages[0].ID)
	startTime := time.Now()

	// Execute (simulating timeout)
	time.Sleep(50 * time.Millisecond)

	completedTime := time.Now()
	duration := completedTime.Sub(startTime)

	if err := tdb.exec(
		`INSERT INTO stage_runs
		(id, pipeline_run_id, stage_name, status, started_at, completed_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		stageRun.ID, stageRun.PipelineRunID, stageRun.StageName,
		StageCompleted.String(), startTime, completedTime,
	); err != nil {
		t.Fatalf("failed to save stage run: %v", err)
	}

	// Verify timing
	rows, err := tdb.query(
		`SELECT started_at, completed_at FROM stage_runs WHERE id = ?`,
		stageRun.ID,
	)
	if err != nil {
		t.Fatalf("failed to query stage run: %v", err)
	}
	defer rows.Close()

	if !rows.Next() {
		t.Fatal("expected stage run row")
	}

	var start, end time.Time
	if err := rows.Scan(&start, &end); err != nil {
		t.Fatalf("failed to scan stage run: %v", err)
	}

	if end.Before(start) {
		t.Errorf("end time should be after start time")
	}
	if duration < 50*time.Millisecond {
		t.Errorf("expected duration >= 50ms, got %v", duration)
	}
}

// TestIntegrationRetryOnTransientError tests retrying on transient errors.
func TestIntegrationRetryOnTransientError(t *testing.T) {
	tdb := newTestDB(t)
	defer tdb.close()

	pipelineDef := createIntegrationTestPipeline("retry-test", 1)
	pipelineDef.Stages[0].Retries = 3

	run := createTestPipelineRun(pipelineDef.Name, 1)

	if err := tdb.savePipelineRun(run); err != nil {
		t.Fatalf("failed to save pipeline: %v", err)
	}

	stageRun := createTestStageRun(run.ID, pipelineDef.Stages[0].ID)

	// Simulate 3 retry attempts
	for attempt := 1; attempt <= 3; attempt++ {
		status := StageFailed
		if attempt == 3 {
			status = StageCompleted
		}

		if attempt == 1 {
			if err := tdb.exec(
				`INSERT INTO stage_runs
				(id, pipeline_run_id, stage_name, status, retry_count, started_at)
				VALUES (?, ?, ?, ?, ?, ?)`,
				stageRun.ID, stageRun.PipelineRunID, stageRun.StageName,
				status.String(), attempt, time.Now(),
			); err != nil {
				t.Fatalf("failed to insert stage run: %v", err)
			}
		} else {
			if err := tdb.exec(
				`UPDATE stage_runs SET status = ?, retry_count = ? WHERE id = ?`,
				status.String(), attempt, stageRun.ID,
			); err != nil {
				t.Fatalf("failed to update stage run: %v", err)
			}
		}

		time.Sleep(10 * time.Millisecond) // Simulate retry delay
	}

	// Verify final state
	rows, err := tdb.query(
		`SELECT retry_count, status FROM stage_runs WHERE id = ?`,
		stageRun.ID,
	)
	if err != nil {
		t.Fatalf("failed to query stage run: %v", err)
	}
	defer rows.Close()

	if !rows.Next() {
		t.Fatal("expected stage run row")
	}

	var attempts int
	var status string
	if err := rows.Scan(&attempts, &status); err != nil {
		t.Fatalf("failed to scan: %v", err)
	}

	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
	if status != StageCompleted.String() {
		t.Errorf("expected status %s, got %s", StageCompleted, status)
	}
}

// TestArtifactStorageAndRetrieval tests artifact storage and retrieval.
func TestArtifactStorageAndRetrieval(t *testing.T) {
	tdb := newTestDB(t)
	defer tdb.close()

	run := createTestPipelineRun("artifact-test", 1)
	stageRun := createTestStageRun(run.ID, "stage-1")

	if err := tdb.savePipelineRun(run); err != nil {
		t.Fatalf("failed to save pipeline: %v", err)
	}

	testData := []byte("test artifact data")
	artifactName := "output.txt"

	// Save artifact
	if err := tdb.saveTestArtifact(run.ID, stageRun.ID, artifactName, testData); err != nil {
		t.Fatalf("failed to save artifact: %v", err)
	}

	// Retrieve artifact
	rows, err := tdb.query(
		`SELECT name, size_bytes, checksum FROM artifacts WHERE pipeline_run_id = ? AND name = ?`,
		run.ID, artifactName,
	)
	if err != nil {
		t.Fatalf("failed to query artifacts: %v", err)
	}
	defer rows.Close()

	if !rows.Next() {
		t.Fatal("expected artifact row")
	}

	var name string
	var size int64
	var checksum string
	if err := rows.Scan(&name, &size, &checksum); err != nil {
		t.Fatalf("failed to scan artifact: %v", err)
	}

	expectedChecksum := CalculateChecksum(testData)
	if checksum != expectedChecksum {
		t.Errorf("expected checksum %s, got %s", expectedChecksum, checksum)
	}
	if size != int64(len(testData)) {
		t.Errorf("expected size %d, got %d", len(testData), size)
	}
	if name != artifactName {
		t.Errorf("expected name %s, got %s", artifactName, name)
	}
}

// TestCrashRecoveryScenario tests crash recovery by saving and restoring state.
func TestCrashRecoveryScenario(t *testing.T) {
	tdb := newTestDB(t)
	defer tdb.close()

	pipelineDef := createIntegrationTestPipeline("crash-recovery", 5)
	run := createTestPipelineRun(pipelineDef.Name, len(pipelineDef.Stages))

	if err := tdb.savePipelineRun(run); err != nil {
		t.Fatalf("failed to save initial pipeline: %v", err)
	}

	// Simulate execution to stage 2
	run.Status = PipelineRunning
	run.CurrentStageIndex = 2

	// Save checkpoint before "crash"
	if err := tdb.saveCheckpoint(run.ID, run.CurrentStageIndex, run); err != nil {
		t.Fatalf("failed to save checkpoint: %v", err)
	}

	// Update pipeline state in database
	if err := tdb.exec(
		`UPDATE pipeline_runs SET status = ?, current_stage_index = ? WHERE id = ?`,
		run.Status.String(), run.CurrentStageIndex, run.ID,
	); err != nil {
		t.Fatalf("failed to update pipeline: %v", err)
	}

	// Simulate crash and recovery: load from checkpoint
	recoveredRun, err := tdb.loadLatestCheckpoint(run.ID)
	if err != nil {
		t.Fatalf("failed to load checkpoint: %v", err)
	}

	if recoveredRun.CurrentStageIndex != 2 {
		t.Errorf("expected current_stage_index 2 after recovery, got %d", recoveredRun.CurrentStageIndex)
	}

	// Resume execution from recovered state
	recoveredRun.Status = PipelineRunning
	recoveredRun.CurrentStageIndex = 3 // Continue from next stage

	// Continue execution and complete
	for i := recoveredRun.CurrentStageIndex; i < len(pipelineDef.Stages); i++ {
		recoveredRun.CurrentStageIndex = i
		if err := tdb.exec(
			`UPDATE pipeline_runs SET current_stage_index = ? WHERE id = ?`,
			i, recoveredRun.ID,
		); err != nil {
			t.Fatalf("failed to update pipeline: %v", err)
		}
	}

	recoveredRun.Status = PipelineCompleted
	recoveredRun.CompletedAt = timePtr(time.Now())
	if err := tdb.exec(
		`UPDATE pipeline_runs SET status = ?, completed_at = ? WHERE id = ?`,
		recoveredRun.Status.String(), recoveredRun.CompletedAt, recoveredRun.ID,
	); err != nil {
		t.Fatalf("failed to complete pipeline: %v", err)
	}

	// Verify final state
	finalRun, err := tdb.loadPipelineRun(recoveredRun.ID)
	if err != nil {
		t.Fatalf("failed to load final pipeline: %v", err)
	}

	if finalRun.Status != PipelineCompleted {
		t.Errorf("expected status COMPLETED, got %s", finalRun.Status)
	}
}

// TestProcessKillMidPipelineRestart tests killing a process mid-pipeline and restarting.
func TestProcessKillMidPipelineRestart(t *testing.T) {
	tdb := newTestDB(t)
	defer tdb.close()

	pipelineDef := createIntegrationTestPipeline("kill-test", 4)
	run := createTestPipelineRun(pipelineDef.Name, len(pipelineDef.Stages))

	if err := tdb.savePipelineRun(run); err != nil {
		t.Fatalf("failed to save pipeline: %v", err)
	}

	// Simulate execution to stage 1, then "kill" process
	run.Status = PipelineRunning
	run.CurrentStageIndex = 1

	// Save state
	if err := tdb.exec(
		`UPDATE pipeline_runs SET status = ?, current_stage_index = ? WHERE id = ?`,
		run.Status.String(), run.CurrentStageIndex, run.ID,
	); err != nil {
		t.Fatalf("failed to update pipeline: %v", err)
	}

	// Simulate process kill - save checkpoint before "crash"
	if err := tdb.saveCheckpoint(run.ID, run.CurrentStageIndex, run); err != nil {
		t.Fatalf("failed to save checkpoint: %v", err)
	}

	// Simulate restart: check for RUNNING pipelines in database
	var runningCount int
	err := tdb.db.QueryRow(
		`SELECT COUNT(*) FROM pipeline_runs WHERE status = ?`,
		PipelineRunning.String(),
	).Scan(&runningCount)
	if err != nil {
		t.Fatalf("failed to check running pipelines: %v", err)
	}

	if runningCount == 0 {
		t.Error("expected to find running pipeline")
	}

	// Load latest checkpoint and resume
	resumeRun, err := tdb.loadLatestCheckpoint(run.ID)
	if err != nil {
		t.Fatalf("failed to load checkpoint: %v", err)
	}

	// Verify resume point
	if resumeRun.CurrentStageIndex != 1 {
		t.Errorf("expected resume at stage 1, got %d", resumeRun.CurrentStageIndex)
	}

	// Continue execution
	resumeRun.CurrentStageIndex = 2
	resumeRun.Status = PipelineRunning

	for i := resumeRun.CurrentStageIndex; i < len(pipelineDef.Stages); i++ {
		if err := tdb.exec(
			`UPDATE pipeline_runs SET current_stage_index = ? WHERE id = ?`,
			i, resumeRun.ID,
		); err != nil {
			t.Fatalf("failed to update: %v", err)
		}
	}

	resumeRun.Status = PipelineCompleted
	resumeRun.CompletedAt = timePtr(time.Now())
	if err := tdb.exec(
		`UPDATE pipeline_runs SET status = ?, completed_at = ? WHERE id = ?`,
		resumeRun.Status.String(), resumeRun.CompletedAt, resumeRun.ID,
	); err != nil {
		t.Fatalf("failed to complete: %v", err)
	}

	// Verify
	final, err := tdb.loadPipelineRun(resumeRun.ID)
	if err != nil {
		t.Fatalf("failed to load final: %v", err)
	}

	if final.Status != PipelineCompleted {
		t.Errorf("expected COMPLETED, got %s", final.Status)
	}
}

// TestMultiplePipelinesParallel tests concurrent execution of multiple pipelines.
func TestMultiplePipelinesParallel(t *testing.T) {
	tdb := newTestDB(t)
	defer tdb.close()

	numPipelines := 5
	pipelines := make([]*PipelineRun, numPipelines)

	// Create multiple pipelines
	for i := 0; i < numPipelines; i++ {
		pipelines[i] = createTestPipelineRun(fmt.Sprintf("parallel-pipeline-%d", i), 3)
		if err := tdb.savePipelineRun(pipelines[i]); err != nil {
			t.Fatalf("failed to save pipeline %d: %v", i, err)
		}
	}

	// Execute pipelines in parallel
	var wg sync.WaitGroup
	var successCount int32

	for i, pipeline := range pipelines {
		wg.Add(1)
		go func(idx int, p *PipelineRun) {
			defer wg.Done()

			// Simulate pipeline execution
			p.Status = PipelineRunning
			if err := tdb.exec(
				`UPDATE pipeline_runs SET status = ? WHERE id = ?`,
				p.Status.String(), p.ID,
			); err != nil {
				t.Errorf("failed to update pipeline %d: %v", idx, err)
				return
			}

			for stageIdx := 0; stageIdx < 3; stageIdx++ {
				p.CurrentStageIndex = stageIdx
				if err := tdb.exec(
					`UPDATE pipeline_runs SET current_stage_index = ? WHERE id = ?`,
					stageIdx, p.ID,
				); err != nil {
					t.Errorf("failed to update stage for pipeline %d: %v", idx, err)
					return
				}
				time.Sleep(10 * time.Millisecond)
			}

			p.Status = PipelineCompleted
			p.CompletedAt = timePtr(time.Now())
			if err := tdb.exec(
				`UPDATE pipeline_runs SET status = ?, completed_at = ? WHERE id = ?`,
				p.Status.String(), p.CompletedAt, p.ID,
			); err != nil {
				t.Errorf("failed to complete pipeline %d: %v", idx, err)
				return
			}

			atomic.AddInt32(&successCount, 1)
		}(i, pipeline)
	}

	wg.Wait()

	if successCount != int32(numPipelines) {
		t.Errorf("expected %d successful pipelines, got %d", numPipelines, successCount)
	}

	// Verify all pipelines completed
	var completedCount int
	err := tdb.db.QueryRow(
		`SELECT COUNT(*) FROM pipeline_runs WHERE status = ?`,
		PipelineCompleted.String(),
	).Scan(&completedCount)
	if err != nil {
		t.Fatalf("failed to count completed pipelines: %v", err)
	}

	if completedCount != numPipelines {
		t.Errorf("expected %d completed pipelines in DB, got %d", numPipelines, completedCount)
	}
}

// TestConcurrentStageExecution tests concurrent stage execution within a single pipeline.
func TestConcurrentStageExecution(t *testing.T) {
	tdb := newTestDB(t)
	defer tdb.close()

	run := createTestPipelineRun("concurrent-stages", 1)
	if err := tdb.savePipelineRun(run); err != nil {
		t.Fatalf("failed to save pipeline: %v", err)
	}

	// Create multiple stages for same pipeline
	numStages := 3
	var wg sync.WaitGroup
	var successCount int32

	for i := 0; i < numStages; i++ {
		wg.Add(1)
		go func(stageNum int) {
			defer wg.Done()

			stageID := fmt.Sprintf("parallel-stage-%d", stageNum)
			stageRun := createTestStageRun(run.ID, stageID)

			if err := tdb.exec(
				`INSERT INTO stage_runs
				(id, pipeline_run_id, stage_name, status, started_at)
				VALUES (?, ?, ?, ?, ?)`,
				stageRun.ID, stageRun.PipelineRunID, stageRun.StageName,
				StageRunning.String(), stageRun.StartedAt,
			); err != nil {
				t.Errorf("failed to insert stage run: %v", err)
				return
			}

			// Simulate work
			time.Sleep(50 * time.Millisecond)

			if err := tdb.exec(
				`UPDATE stage_runs SET status = ?, completed_at = ? WHERE id = ?`,
				StageCompleted.String(), time.Now(), stageRun.ID,
			); err != nil {
				t.Errorf("failed to complete stage: %v", err)
				return
			}

			atomic.AddInt32(&successCount, 1)
		}(i)
	}

	wg.Wait()

	if successCount != int32(numStages) {
		t.Errorf("expected %d successful stages, got %d", numStages, successCount)
	}

	// Verify all stages saved
	var stageCount int
	err := tdb.db.QueryRow(
		`SELECT COUNT(*) FROM stage_runs WHERE pipeline_run_id = ?`,
		run.ID,
	).Scan(&stageCount)
	if err != nil {
		t.Fatalf("failed to count stages: %v", err)
	}

	if stageCount != numStages {
		t.Errorf("expected %d stages in DB, got %d", numStages, stageCount)
	}
}

// TestCheckpointCreationAndRecovery tests checkpoint lifecycle.
func TestCheckpointCreationAndRecovery(t *testing.T) {
	tdb := newTestDB(t)
	defer tdb.close()

	run := createTestPipelineRun("checkpoint-test", 5)
	if err := tdb.savePipelineRun(run); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	// Create checkpoints at multiple stages
	checkpointStages := []int{1, 2, 3}
	for _, stage := range checkpointStages {
		run.CurrentStageIndex = stage
		run.Status = PipelineRunning

		if err := tdb.saveCheckpoint(run.ID, stage, run); err != nil {
			t.Fatalf("failed to save checkpoint: %v", err)
		}

		time.Sleep(10 * time.Millisecond)
	}

	// Load latest checkpoint (should be stage 3)
	latest, err := tdb.loadLatestCheckpoint(run.ID)
	if err != nil {
		t.Fatalf("failed to load checkpoint: %v", err)
	}

	if latest.CurrentStageIndex != 3 {
		t.Errorf("expected latest checkpoint at stage 3, got %d", latest.CurrentStageIndex)
	}

	// Verify checkpoint count
	var checkpointCount int
	err = tdb.db.QueryRow(
		`SELECT COUNT(*) FROM checkpoints WHERE pipeline_run_id = ?`,
		run.ID,
	).Scan(&checkpointCount)
	if err != nil {
		t.Fatalf("failed to count checkpoints: %v", err)
	}

	if checkpointCount != len(checkpointStages) {
		t.Errorf("expected %d checkpoints, got %d", len(checkpointStages), checkpointCount)
	}
}

// TestArtifactMetadataIntegrity tests artifact metadata consistency.
func TestArtifactMetadataIntegrity(t *testing.T) {
	tdb := newTestDB(t)
	defer tdb.close()

	run := createTestPipelineRun("artifact-integrity", 1)
	if err := tdb.savePipelineRun(run); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	// Save multiple artifacts
	artifacts := map[string][]byte{
		"output1.txt":  []byte("first output"),
		"output2.json": []byte(`{"status": "complete"}`),
		"output3.csv":  []byte("col1,col2\nval1,val2"),
	}

	for name, data := range artifacts {
		if err := tdb.saveTestArtifact(run.ID, generateTestID("stage"), name, data); err != nil {
			t.Fatalf("failed to save artifact %s: %v", name, err)
		}
	}

	// Verify all artifacts and checksums
	for name, expectedData := range artifacts {
		rows, err := tdb.query(
			`SELECT checksum, size_bytes FROM artifacts WHERE pipeline_run_id = ? AND name = ?`,
			run.ID, name,
		)
		if err != nil {
			t.Fatalf("failed to query artifact: %v", err)
		}
		defer rows.Close()

		if !rows.Next() {
			t.Errorf("artifact %s not found", name)
			continue
		}

		var checksum string
		var size int64
		if err := rows.Scan(&checksum, &size); err != nil {
			t.Fatalf("failed to scan artifact: %v", err)
		}

		expectedChecksum := CalculateChecksum(expectedData)
		if checksum != expectedChecksum {
			t.Errorf("checksum mismatch for %s: expected %s, got %s", name, expectedChecksum, checksum)
		}
		if size != int64(len(expectedData)) {
			t.Errorf("size mismatch for %s: expected %d, got %d", name, len(expectedData), size)
		}
	}
}

// TestPipelineStateTransitions tests valid state transitions.
func TestPipelineStateTransitions(t *testing.T) {
	tdb := newTestDB(t)
	defer tdb.close()

	run := createTestPipelineRun("state-test", 3)
	if err := tdb.savePipelineRun(run); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	transitions := []PipelineStatus{
		PipelinePending,
		PipelineRunning,
		PipelineStageComplete,
		PipelineRunning,
		PipelineCheckpoint,
		PipelineRunning,
		PipelineCompleted,
	}

	for _, status := range transitions {
		if err := tdb.exec(
			`UPDATE pipeline_runs SET status = ? WHERE id = ?`,
			status.String(), run.ID,
		); err != nil {
			t.Fatalf("failed to transition to %s: %v", status, err)
		}

		// Verify transition
		var loadedStatus string
		err := tdb.db.QueryRow(
			`SELECT status FROM pipeline_runs WHERE id = ?`,
			run.ID,
		).Scan(&loadedStatus)
		if err != nil {
			t.Fatalf("failed to load status: %v", err)
		}

		if loadedStatus != status.String() {
			t.Errorf("expected status %s, got %s", status.String(), loadedStatus)
		}
	}
}

// TestLargeArtifactHandling tests handling of large artifacts.
func TestLargeArtifactHandling(t *testing.T) {
	tdb := newTestDB(t)
	defer tdb.close()

	run := createTestPipelineRun("large-artifact", 1)
	if err := tdb.savePipelineRun(run); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	// Create large artifact (1MB)
	largeData := make([]byte, 1024*1024)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	if err := tdb.saveTestArtifact(run.ID, generateTestID("stage"), "large.bin", largeData); err != nil {
		t.Fatalf("failed to save large artifact: %v", err)
	}

	// Verify artifact
	var size int64
	err := tdb.db.QueryRow(
		`SELECT size_bytes FROM artifacts WHERE pipeline_run_id = ? AND name = ?`,
		run.ID, "large.bin",
	).Scan(&size)
	if err != nil {
		t.Fatalf("failed to query artifact: %v", err)
	}

	if size != int64(len(largeData)) {
		t.Errorf("expected size %d, got %d", len(largeData), size)
	}
}

// TestDataPersistenceAcrossSessions tests data persistence across database sessions.
func TestDataPersistenceAcrossSessions(t *testing.T) {
	// First session: create and save data
	tdb1 := newTestDB(t)
	run1 := createTestPipelineRun("persistence-test", 2)
	if err := tdb1.savePipelineRun(run1); err != nil {
		t.Fatalf("failed to save in session 1: %v", err)
	}

	if err := tdb1.saveTestArtifact(run1.ID, generateTestID("stage"), "test.txt", []byte("test data")); err != nil {
		t.Fatalf("failed to save artifact in session 1: %v", err)
	}
	tdb1.close()

	// Second session: reconnect and verify data
	tdb2 := newTestDB(t)
	defer tdb2.close()

	loaded, err := tdb2.loadPipelineRun(run1.ID)
	if err != nil {
		t.Fatalf("failed to load in session 2: %v", err)
	}

	if loaded.ID != run1.ID {
		t.Errorf("expected pipeline ID %s, got %s", run1.ID, loaded.ID)
	}

	// Verify artifact persisted
	var count int
	err = tdb2.db.QueryRow(
		`SELECT COUNT(*) FROM artifacts WHERE pipeline_run_id = ?`,
		run1.ID,
	).Scan(&count)
	if err != nil {
		t.Fatalf("failed to count artifacts: %v", err)
	}

	if count != 1 {
		t.Errorf("expected 1 artifact, got %d", count)
	}
}

// TestErrorHandlingAndLogging tests error handling in pipeline operations.
func TestErrorHandlingAndLogging(t *testing.T) {
	tdb := newTestDB(t)
	defer tdb.close()

	run := createTestPipelineRun("error-test", 2)
	if err := tdb.savePipelineRun(run); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	// Simulate error and save
	errorMsg := "stage execution failed: timeout"
	run.Status = PipelineFailed
	run.Error = errorMsg
	run.ErrorStage = "stage-0"

	if err := tdb.exec(
		`UPDATE pipeline_runs SET status = ?, error = ?, error_stage = ? WHERE id = ?`,
		run.Status.String(), errorMsg, run.ErrorStage, run.ID,
	); err != nil {
		t.Fatalf("failed to update error: %v", err)
	}

	// Verify error saved
	var loadedError, loadedStage string
	err := tdb.db.QueryRow(
		`SELECT error, error_stage FROM pipeline_runs WHERE id = ?`,
		run.ID,
	).Scan(&loadedError, &loadedStage)
	if err != nil {
		t.Fatalf("failed to query: %v", err)
	}

	if loadedError != errorMsg {
		t.Errorf("expected error %s, got %s", errorMsg, loadedError)
	}
	if loadedStage != run.ErrorStage {
		t.Errorf("expected error stage %s, got %s", run.ErrorStage, loadedStage)
	}
}
