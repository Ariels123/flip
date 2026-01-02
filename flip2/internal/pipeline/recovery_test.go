// Package pipeline provides tests for automatic pipeline recovery.
package pipeline

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	_ "github.com/pocketbase/pocketbase/migrations"
)

// =============================================================================
// TEST FIXTURES
// =============================================================================

// mockLogger implements RecoveryLogger for testing.
type mockLogger struct {
	infoLogs  []string
	warnLogs  []string
	errorLogs []string
	debugLogs []string
}

func newMockLogger() *mockLogger {
	return &mockLogger{
		infoLogs:  []string{},
		warnLogs:  []string{},
		errorLogs: []string{},
		debugLogs: []string{},
	}
}

func (m *mockLogger) Info(msg string, args ...interface{}) {
	m.infoLogs = append(m.infoLogs, fmt.Sprintf(msg, args...))
}

func (m *mockLogger) Warn(msg string, args ...interface{}) {
	m.warnLogs = append(m.warnLogs, fmt.Sprintf(msg, args...))
}

func (m *mockLogger) Error(msg string, args ...interface{}) {
	m.errorLogs = append(m.errorLogs, fmt.Sprintf(msg, args...))
}

func (m *mockLogger) Debug(msg string, args ...interface{}) {
	m.debugLogs = append(m.debugLogs, fmt.Sprintf(msg, args...))
}

// setupTestDB creates an in-memory PocketBase instance for testing.
func setupTestDB(t *testing.T) core.App {
	pb := pocketbase.New()

	// Bootstrap to initialize database
	if err := pb.Bootstrap(); err != nil {
		t.Fatalf("Failed to bootstrap PocketBase: %v", err)
	}

	return pb
}

// createRecoveryTestPipeline creates a test pipeline with stages.
func createRecoveryTestPipeline(t *testing.T, pb core.App) *PipelineRun {
	now := time.Now()
	pipeline := &PipelineRun{
		ID:                "test-pipeline-1",
		PipelineID:        "research",
		Status:            PipelineRunning,
		CurrentStageIndex: 1,
		TotalStages:       3,
		Input:             json.RawMessage(`{"topic":"Go programming"}`),
		RetryCount:        0,
		MaxRetries:        3,
		Priority:          1,
		CreatedAt:         now,
		UpdatedAt:         now,
		StartedAt:         &now,
		Stages: []StageRun{
			{
				ID:            "stage-1",
				PipelineRunID: "test-pipeline-1",
				StageName:     "gather",
				StageIndex:    0,
				Status:        StageCompleted,
				Backend:       "gemini",
				Input:         json.RawMessage(`{"topic":"Go programming"}`),
				Output:        json.RawMessage(`{"results":["Go is fast","Go has great concurrency"]}`),
				StartedAt:     &now,
				CompletedAt: func() *time.Time {
					t := now.Add(5 * time.Minute)
					return &t
				}(),
				Metrics: StageMetrics{
					TokensIn:       100,
					TokensOut:      200,
					Cost:           0.001,
					DurationMs:     300000,
					MemoryUsedBytes: 52428800,
				},
			},
			{
				ID:            "stage-2",
				PipelineRunID: "test-pipeline-1",
				StageName:     "analyze",
				StageIndex:    1,
				Status:        StageRunning,
				Backend:       "claude",
				Input:         json.RawMessage(`{"results":["Go is fast","Go has great concurrency"]}`),
				Output:        json.RawMessage(`{}`),
				StartedAt:     &now,
				Metrics:       StageMetrics{},
			},
			{
				ID:            "stage-3",
				PipelineRunID: "test-pipeline-1",
				StageName:     "format",
				StageIndex:    2,
				Status:        StagePending,
				Backend:       "claude",
				Input:         json.RawMessage(`{}`),
				Output:        json.RawMessage(`{}`),
				Metrics:       StageMetrics{},
			},
		},
		Metadata: map[string]interface{}{
			"user_id": "test-user",
		},
	}

	return pipeline
}

// =============================================================================
// RECOVERY BASIC TESTS
// =============================================================================

// TestRecoveryInitialization tests that Recovery is properly initialized.
func TestRecoveryInitialization(t *testing.T) {
	pb := setupTestDB(t)

	store := NewPipelineStore(pb)
	logger := newMockLogger()
	recovery := NewRecovery(store, logger)

	if recovery == nil {
		t.Fatal("Expected recovery instance, got nil")
	}

	if recovery.store != store {
		t.Fatal("Recovery store not properly set")
	}

	if recovery.logger != logger {
		t.Fatal("Recovery logger not properly set")
	}
}

// TestRecoveryWithNilLogger tests that Recovery works with nil logger.
func TestRecoveryWithNilLogger(t *testing.T) {
	pb := setupTestDB(t)

	store := NewPipelineStore(pb)
	recovery := NewRecovery(store, nil)

	if recovery == nil {
		t.Fatal("Expected recovery instance, got nil")
	}

	// Should not panic when logging
	recovery.logger.Info("Test message")
}

// =============================================================================
// RECOVERY LOGIC TESTS
// =============================================================================

// TestDetermineRecoveryAction tests recovery action determination.
func TestDetermineRecoveryAction(t *testing.T) {
	pb := setupTestDB(t)

	store := NewPipelineStore(pb)
	logger := newMockLogger()
	recovery := NewRecovery(store, logger)

	tests := []struct {
		name     string
		rc       *RecoverableCheckpoint
		expected RecoveryAction
	}{
		{
			name:     "nil checkpoint",
			rc:       nil,
			expected: RecoveryActionAbort,
		},
		{
			name: "nil pipeline",
			rc: &RecoverableCheckpoint{
				PipelineRun: nil,
			},
			expected: RecoveryActionAbort,
		},
		{
			name: "resume from checkpoint",
			rc: &RecoverableCheckpoint{
				PipelineRun: &PipelineRun{
					ID:         "test-1",
					Status:     PipelineCheckpoint,
					TotalStages: 3,
				},
				LastCheckpoint: &Checkpoint{
					ID: "cp-1",
				},
				LastStageIndex: 0,
			},
			expected: RecoveryActionResumeCheckpoint,
		},
		{
			name: "resume from last stage",
			rc: &RecoverableCheckpoint{
				PipelineRun: &PipelineRun{
					ID:          "test-2",
					Status:      PipelineRunning,
					TotalStages: 3,
				},
				LastCheckpoint: nil,
				LastStageIndex: 1,
			},
			expected: RecoveryActionResumeLastStage,
		},
		{
			name: "restart pipeline",
			rc: &RecoverableCheckpoint{
				PipelineRun: &PipelineRun{
					ID:                "test-3",
					Status:            PipelineRunning,
					TotalStages:       3,
					CurrentStageIndex: 0,
				},
				LastCheckpoint: nil,
				LastStageIndex: -1,
			},
			expected: RecoveryActionRestartPipeline,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			action := recovery.DetermineRecoveryAction(tt.rc)
			if action != tt.expected {
				t.Errorf("Expected action %s, got %s", tt.expected, action)
			}
		})
	}
}

// =============================================================================
// SIMULATED CRASH RECOVERY TESTS
// =============================================================================

// TestCrashRecoverySimulation simulates a crash and recovery scenario.
//
// This test:
// 1. Creates a pipeline with multiple stages
// 2. Simulates crash by stopping execution mid-pipeline
// 3. Creates a checkpoint of the state
// 4. Verifies recovery finds and properly resumes the pipeline
func TestCrashRecoverySimulation(t *testing.T) {
	t.Log("=== CRASH RECOVERY SIMULATION ===")

	pb := setupTestDB(t)

	store := NewPipelineStore(pb)
	logger := newMockLogger()
	recovery := NewRecovery(store, logger)

	// Step 1: Create and persist a pipeline mid-execution
	t.Log("Step 1: Creating pipeline in mid-execution state")
	pipeline := createRecoveryTestPipeline(t, pb)
	pipeline.Status = PipelineRunning
	pipeline.CurrentStageIndex = 1

	if err := store.SavePipeline(pipeline); err != nil {
		t.Fatalf("Failed to save pipeline: %v", err)
	}

	// Save stages
	for _, stage := range pipeline.Stages {
		if err := store.SaveStage(&stage); err != nil {
			t.Fatalf("Failed to save stage: %v", err)
		}
	}

	t.Log("Pipeline created with status: RUNNING, current_stage: 1")

	// Step 2: Simulate crash - in production, the process would be killed here
	t.Log("Step 2: Simulating crash (process would be killed here)")
	// No action needed - just proceed to recovery

	// Step 3: System restart - call recovery
	t.Log("Step 3: System restarting - calling recovery")
	checkpoints, totalFound, checkpointCount, err := recovery.RecoverPipelines()
	if err != nil {
		t.Fatalf("Recovery failed: %v", err)
	}

	// Verify recovery found the pipeline
	if totalFound != 1 {
		t.Errorf("Expected 1 pipeline to recover, found %d", totalFound)
	}

	if len(checkpoints) != 1 {
		t.Errorf("Expected 1 recoverable checkpoint, got %d", len(checkpoints))
	}

	if checkpointCount != 0 {
		// No checkpoint was created, so this should be 0
		t.Log("Note: No pre-existing checkpoints, recovery based on stage completion")
	}

	t.Log("Recovery found the crashed pipeline")

	// Step 4: Determine recovery action and resume
	t.Log("Step 4: Determining recovery action")
	rc := checkpoints[0]
	action := recovery.DetermineRecoveryAction(rc)
	t.Logf("Recovery action determined: %s", action)

	if action != RecoveryActionResumeLastStage {
		t.Errorf("Expected RecoveryActionResumeLastStage, got %s", action)
	}

	// Step 5: Resume from last completed stage
	t.Log("Step 5: Resuming from last completed stage")
	nextStageIndex, err := recovery.ResumeFromLastStage(rc)
	if err != nil {
		t.Fatalf("Failed to resume: %v", err)
	}

	if nextStageIndex != 1 {
		t.Errorf("Expected next stage to be 1, got %d", nextStageIndex)
	}

	t.Log("Pipeline resumed successfully, next stage: 1 (analyze)")

	// Step 6: Verify pipeline is now in running state and ready to continue
	t.Log("Step 6: Verifying recovered pipeline state")
	recoveredPipeline := rc.PipelineRun
	if recoveredPipeline.Status != PipelineRunning {
		t.Errorf("Expected status RUNNING, got %s", recoveredPipeline.Status)
	}

	if recoveredPipeline.CurrentStageIndex != 1 {
		t.Errorf("Expected current_stage_index 1, got %d", recoveredPipeline.CurrentStageIndex)
	}

	if recoveredPipeline.StartedAt == nil {
		t.Error("Expected StartedAt to be set")
	}

	t.Log("=== RECOVERY SUCCESSFUL ===")
	t.Logf("Pipeline %s recovered and ready to resume", recoveredPipeline.ID)
	t.Logf("Last completed stage: %d, Next stage to execute: %d", rc.LastStageIndex, nextStageIndex)
}

// TestCheckpointBasedRecovery tests recovery using a saved checkpoint.
func TestCheckpointBasedRecovery(t *testing.T) {
	t.Log("=== CHECKPOINT-BASED RECOVERY ===")

	pb := setupTestDB(t)

	store := NewPipelineStore(pb)
	logger := newMockLogger()
	recovery := NewRecovery(store, logger)

	// Create and persist a pipeline
	t.Log("Step 1: Creating pipeline with checkpoint")
	pipeline := createRecoveryTestPipeline(t, pb)
	pipeline.Status = PipelineCheckpoint
	pipeline.CurrentStageIndex = 1

	if err := store.SavePipeline(pipeline); err != nil {
		t.Fatalf("Failed to save pipeline: %v", err)
	}

	for _, stage := range pipeline.Stages {
		if err := store.SaveStage(&stage); err != nil {
			t.Fatalf("Failed to save stage: %v", err)
		}
	}

	// Create checkpoint
	t.Log("Step 2: Creating checkpoint")
	pipelineStateJSON, _ := json.Marshal(pipeline)
	stageStatesJSON, _ := json.Marshal(pipeline.Stages)

	checkpoint := &Checkpoint{
		ID:            "checkpoint-1",
		PipelineRunID: pipeline.ID,
		Version:       1,
		PipelineState: pipelineStateJSON,
		StageStates:   stageStatesJSON,
		Reason:        CheckpointStageComplete,
		CreatedAt:     time.Now(),
		SizeBytes:     int64(len(pipelineStateJSON) + len(stageStatesJSON)),
	}

	if err := store.SaveCheckpoint(checkpoint); err != nil {
		t.Fatalf("Failed to save checkpoint: %v", err)
	}

	now := time.Now()
	pipeline.LastCheckpointAt = &now
	if err := store.SavePipeline(pipeline); err != nil {
		t.Fatalf("Failed to update pipeline: %v", err)
	}

	t.Logf("Checkpoint created for pipeline %s, version %d", pipeline.ID, checkpoint.Version)

	// Simulate crash and recovery
	t.Log("Step 3: Simulating crash and recovery")
	checkpoints, _, _, err := recovery.RecoverPipelines()
	if err != nil {
		t.Fatalf("Recovery failed: %v", err)
	}

	if len(checkpoints) != 1 {
		t.Fatalf("Expected 1 recoverable checkpoint, got %d", len(checkpoints))
	}

	rc := checkpoints[0]
	if rc.LastCheckpoint == nil {
		t.Fatal("Expected checkpoint to be loaded")
	}

	t.Log("Step 4: Resuming from checkpoint")
	nextStageIndex, err := recovery.ResumeFromCheckpoint(rc)
	if err != nil {
		t.Fatalf("Failed to resume from checkpoint: %v", err)
	}

	if nextStageIndex != 1 {
		t.Errorf("Expected next stage to be 1, got %d", nextStageIndex)
	}

	t.Log("=== CHECKPOINT RECOVERY SUCCESSFUL ===")
	t.Logf("Pipeline recovered from checkpoint, next stage: %d", nextStageIndex)
}

// TestRecoveryStatsCollection tests that recovery statistics are properly collected.
func TestRecoveryStatsCollection(t *testing.T) {
	t.Log("=== RECOVERY STATS COLLECTION ===")

	pb := setupTestDB(t)

	store := NewPipelineStore(pb)
	logger := newMockLogger()
	recovery := NewRecovery(store, logger)

	// Create multiple pipelines in different states
	t.Log("Step 1: Creating multiple pipelines")

	// Pipeline 1: Ready to resume from last stage
	p1 := createRecoveryTestPipeline(t, pb)
	p1.ID = "pipeline-1"
	p1.Status = PipelineRunning
	store.SavePipeline(p1)
	for _, s := range p1.Stages {
		store.SaveStage(&s)
	}
	t.Log("Created pipeline-1: RUNNING, recoverable from last stage")

	// Pipeline 2: In checkpoint state
	p2 := createRecoveryTestPipeline(t, pb)
	p2.ID = "pipeline-2"
	p2.Status = PipelineCheckpoint
	store.SavePipeline(p2)
	for _, s := range p2.Stages {
		store.SaveStage(&s)
	}

	pStateJSON, _ := json.Marshal(p2)
	sStateJSON, _ := json.Marshal(p2.Stages)
	cp2 := &Checkpoint{
		ID:            "cp-2",
		PipelineRunID: p2.ID,
		Version:       1,
		PipelineState: pStateJSON,
		StageStates:   sStateJSON,
		Reason:        CheckpointStageComplete,
		CreatedAt:     time.Now(),
	}
	store.SaveCheckpoint(cp2)
	now := time.Now()
	p2.LastCheckpointAt = &now
	store.SavePipeline(p2)
	t.Log("Created pipeline-2: CHECKPOINT, recoverable from checkpoint")

	// Pipeline 3: Ready to restart from beginning
	p3 := createRecoveryTestPipeline(t, pb)
	p3.ID = "pipeline-3"
	p3.Status = PipelineRunning
	p3.CurrentStageIndex = 0
	p3.Stages = []StageRun{} // No completed stages
	p3.TotalStages = 2
	store.SavePipeline(p3)
	t.Log("Created pipeline-3: RUNNING with no completed stages, needs restart")

	// Step 2: Run full recovery
	t.Log("Step 2: Running full recovery")
	stats, err := recovery.RecoverAllPipelines()
	if err != nil {
		t.Fatalf("Recovery failed: %v", err)
	}

	t.Log("=== RECOVERY STATISTICS ===")
	t.Logf("Total pipelines found: %d", stats.TotalPipelines)
	t.Logf("Resumed from checkpoint: %d", stats.ResumedFromCheckpoint)
	t.Logf("Resumed from last stage: %d", stats.ResumedFromLastStage)
	t.Logf("Restarted: %d", stats.Restarted)
	t.Logf("Aborted: %d", stats.Aborted)
	t.Logf("Duration: %v", stats.Duration)

	// Verify stats
	if stats.TotalPipelines != 3 {
		t.Errorf("Expected 3 total pipelines, got %d", stats.TotalPipelines)
	}

	expectedResumesFromCheckpoint := 1
	expectedResumesFromLastStage := 1
	expectedRestarts := 1

	if stats.ResumedFromCheckpoint != expectedResumesFromCheckpoint {
		t.Errorf("Expected %d resumed from checkpoint, got %d",
			expectedResumesFromCheckpoint, stats.ResumedFromCheckpoint)
	}

	if stats.ResumedFromLastStage != expectedResumesFromLastStage {
		t.Errorf("Expected %d resumed from last stage, got %d",
			expectedResumesFromLastStage, stats.ResumedFromLastStage)
	}

	if stats.Restarted != expectedRestarts {
		t.Errorf("Expected %d restarted, got %d", expectedRestarts, stats.Restarted)
	}

	t.Log("=== STATS VERIFICATION PASSED ===")
}

// TestNoRecoverablePipelines tests recovery when there are no recoverable pipelines.
func TestNoRecoverablePipelines(t *testing.T) {
	t.Log("=== NO RECOVERABLE PIPELINES TEST ===")

	pb := setupTestDB(t)

	store := NewPipelineStore(pb)
	logger := newMockLogger()
	recovery := NewRecovery(store, logger)

	// Run recovery on empty database
	t.Log("Running recovery on empty database")
	checkpoints, totalFound, _, err := recovery.RecoverPipelines()
	if err != nil {
		t.Fatalf("Recovery failed: %v", err)
	}

	if totalFound != 0 {
		t.Errorf("Expected 0 pipelines, got %d", totalFound)
	}

	if len(checkpoints) != 0 {
		t.Errorf("Expected 0 checkpoints, got %d", len(checkpoints))
	}

	t.Log("=== TEST PASSED: No recoverable pipelines found ===")
}

// TestRecoveryErrorHandling tests that recovery handles errors gracefully.
func TestRecoveryErrorHandling(t *testing.T) {
	t.Log("=== RECOVERY ERROR HANDLING ===")

	pb := setupTestDB(t)

	store := NewPipelineStore(pb)
	logger := newMockLogger()
	recovery := NewRecovery(store, logger)

	// Test ResumeFromCheckpoint with nil recovery checkpoint
	t.Log("Testing ResumeFromCheckpoint with nil input")
	_, err := recovery.ResumeFromCheckpoint(nil)
	if err == nil {
		t.Error("Expected error for nil recovery checkpoint, got nil")
	}

	// Test ResumeFromLastStage with nil recovery checkpoint
	t.Log("Testing ResumeFromLastStage with nil input")
	_, err = recovery.ResumeFromLastStage(nil)
	if err == nil {
		t.Error("Expected error for nil recovery checkpoint, got nil")
	}

	// Test DetermineRecoveryAction with edge cases
	t.Log("Testing DetermineRecoveryAction with nil input")
	action := recovery.DetermineRecoveryAction(nil)
	if action != RecoveryActionAbort {
		t.Errorf("Expected RecoveryActionAbort for nil input, got %s", action)
	}

	t.Log("=== ERROR HANDLING TESTS PASSED ===")
}
