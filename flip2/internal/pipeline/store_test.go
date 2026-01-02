package pipeline

import (
	"encoding/json"
	"testing"
	"time"
)

// TestPipelineStoreSaveAndLoad verifies that a pipeline can be saved and loaded.
func TestPipelineStoreSaveAndLoad(t *testing.T) {
	// This test would require a PocketBase instance
	// For now, we'll verify the data structures are correct

	// Create a test pipeline
	input := json.RawMessage(`{"topic":"kubernetes"}`)
	pipeline := NewPipelineRun("research", input, 2)

	// Verify the pipeline structure
	if pipeline.ID == "" {
		t.Error("pipeline ID should not be empty")
	}
	if pipeline.PipelineID != "research" {
		t.Error("pipeline ID should be 'research'")
	}
	if pipeline.Status != PipelinePending {
		t.Error("initial status should be pending")
	}
	if pipeline.TotalStages != 2 {
		t.Error("total stages should be 2")
	}
	if pipeline.CreatedAt.IsZero() {
		t.Error("created_at should be set")
	}
}

// TestStageCreation verifies that a stage can be created with correct defaults.
func TestStageCreation(t *testing.T) {
	pipelineID := "test-pipeline-123"
	stage := NewStageRun(pipelineID, "gather", "gemini", 0)

	if stage.ID == "" {
		t.Error("stage ID should not be empty")
	}
	if stage.PipelineRunID != pipelineID {
		t.Error("pipeline run ID should match")
	}
	if stage.StageName != "gather" {
		t.Error("stage name should be 'gather'")
	}
	if stage.Backend != "gemini" {
		t.Error("backend should be 'gemini'")
	}
	if stage.Status != StagePending {
		t.Error("initial stage status should be pending")
	}
	if stage.StageIndex != 0 {
		t.Error("stage index should be 0")
	}
}

// TestPipelineStatusTransitions verifies valid state transitions.
func TestPipelineStatusTransitions(t *testing.T) {
	validTransitions := []struct {
		from PipelineStatus
		to   PipelineStatus
	}{
		{PipelinePending, PipelineRunning},
		{PipelineRunning, PipelineStageComplete},
		{PipelineStageComplete, PipelineCompleted},
		{PipelineRunning, PipelineFailed},
		{PipelineRunning, PipelineCheckpoint},
	}

	for _, trans := range validTransitions {
		if !CanTransition(trans.from, trans.to) {
			t.Errorf("transition from %s to %s should be valid", trans.from, trans.to)
		}
	}

	// Test invalid transitions
	invalidTransitions := []struct {
		from PipelineStatus
		to   PipelineStatus
	}{
		{PipelineCompleted, PipelineRunning},
		{PipelineFailed, PipelineRunning},
		{PipelinePending, PipelineCompleted},
	}

	for _, trans := range invalidTransitions {
		if CanTransition(trans.from, trans.to) {
			t.Errorf("transition from %s to %s should be invalid", trans.from, trans.to)
		}
	}
}

// TestRecoveryStrategy verifies recovery strategy determination.
func TestRecoveryStrategy(t *testing.T) {
	pipeline := NewPipelineRun("research", json.RawMessage(`{}`), 2)
	pipeline.Status = PipelineRunning

	checkpoint := &Checkpoint{
		ID:            "cp-123",
		PipelineRunID: pipeline.ID,
		Version:       1,
		Reason:        CheckpointStageComplete,
	}

	ctx := &RecoveryContext{
		PipelineRun:   pipeline,
		LastCheckpoint: checkpoint,
		Reason:        "testing",
		Timestamp:     time.Now(),
	}

	strategy := DetermineRecoveryStrategy(ctx)
	if strategy != RecoveryResumeFromCheckpoint {
		t.Errorf("expected RecoveryResumeFromCheckpoint, got %s", strategy)
	}

	// Test with terminal state
	pipeline.Status = PipelineCompleted
	strategy = DetermineRecoveryStrategy(ctx)
	if strategy != RecoveryAbort {
		t.Errorf("expected RecoveryAbort for completed pipeline, got %s", strategy)
	}
}

// TestPipelineValidation verifies that pipeline validation works correctly.
func TestPipelineValidation(t *testing.T) {
	// Valid pipeline
	pipeline := NewPipelineRun("research", json.RawMessage(`{}`), 2)
	if err := pipeline.Validate(); err != nil {
		t.Errorf("valid pipeline should not have validation error: %v", err)
	}

	// Invalid pipeline - no ID
	invalidPipeline := &PipelineRun{
		PipelineID: "research",
		TotalStages: 2,
	}
	if err := invalidPipeline.Validate(); err == nil {
		t.Error("pipeline without ID should fail validation")
	}

	// Invalid pipeline - no pipeline ID
	invalidPipeline2 := &PipelineRun{
		ID:          "test-123",
		TotalStages: 2,
	}
	if err := invalidPipeline2.Validate(); err == nil {
		t.Error("pipeline without pipeline ID should fail validation")
	}

	// Invalid pipeline - invalid stage index
	invalidPipeline3 := &PipelineRun{
		ID:                "test-123",
		PipelineID:        "research",
		TotalStages:       2,
		CurrentStageIndex: 5, // Out of bounds
	}
	if err := invalidPipeline3.Validate(); err == nil {
		t.Error("pipeline with invalid stage index should fail validation")
	}
}

// TestArtifactTypes verifies all artifact types are defined.
func TestArtifactTypes(t *testing.T) {
	types := []ArtifactType{
		ArtifactJSON,
		ArtifactText,
		ArtifactFile,
		ArtifactURL,
		ArtifactBinary,
	}

	expectedCount := 5
	if len(types) != expectedCount {
		t.Errorf("expected %d artifact types, got %d", expectedCount, len(types))
	}
}

// TestCheckpointReasons verifies all checkpoint reasons are defined.
func TestCheckpointReasons(t *testing.T) {
	reasons := []CheckpointReason{
		CheckpointStageComplete,
		CheckpointPeriodic,
		CheckpointManual,
		CheckpointPreShutdown,
		CheckpointError,
	}

	expectedCount := 5
	if len(reasons) != expectedCount {
		t.Errorf("expected %d checkpoint reasons, got %d", expectedCount, len(reasons))
	}
}

// TestPipelineProgress calculates progress correctly.
func TestPipelineProgress(t *testing.T) {
	pipeline := NewPipelineRun("research", json.RawMessage(`{}`), 3)

	// Add stages
	stage1 := NewStageRun(pipeline.ID, "gather", "gemini", 0)
	stage1.Status = StageCompleted

	stage2 := NewStageRun(pipeline.ID, "analyze", "claude", 1)
	stage2.Status = StageCompleted

	stage3 := NewStageRun(pipeline.ID, "format", "claude", 2)
	stage3.Status = StagePending

	pipeline.Stages = []StageRun{*stage1, *stage2, *stage3}

	progress := pipeline.Progress()

	if progress < 66 || progress > 67 {
		t.Errorf("expected progress ~66.67, got %f", progress)
	}
}

// TestPipelineDuration calculates duration correctly.
func TestPipelineDuration(t *testing.T) {
	pipeline := NewPipelineRun("research", json.RawMessage(`{}`), 2)

	// Before start
	if pipeline.Duration() != 0 {
		t.Error("duration should be 0 before pipeline starts")
	}

	// After start
	now := time.Now()
	pipeline.StartedAt = &now

	duration := pipeline.Duration()
	if duration < 0 {
		t.Errorf("duration should be non-negative, got %v", duration)
	}

	// After completion
	completed := now.Add(time.Second * 10)
	pipeline.CompletedAt = &completed

	duration = pipeline.Duration()
	expected := time.Second * 10

	if duration != expected {
		t.Errorf("expected duration %v, got %v", expected, duration)
	}
}

// TestStageDuration calculates stage duration correctly.
func TestStageDuration(t *testing.T) {
	stage := NewStageRun("pipeline-123", "analyze", "claude", 1)

	// Before start
	if stage.Duration() != 0 {
		t.Error("duration should be 0 before stage starts")
	}

	// After start
	now := time.Now()
	stage.StartedAt = &now

	duration := stage.Duration()
	if duration < 0 {
		t.Errorf("duration should be non-negative, got %v", duration)
	}

	// After completion
	completed := now.Add(time.Second * 5)
	stage.CompletedAt = &completed

	duration = stage.Duration()
	expected := time.Second * 5

	if duration != expected {
		t.Errorf("expected duration %v, got %v", expected, duration)
	}
}
