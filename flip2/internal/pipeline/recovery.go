// Package pipeline provides automatic recovery capabilities for interrupted pipelines.
package pipeline

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

// RecoveryLogger is an interface for logging recovery operations.
type RecoveryLogger interface {
	Info(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
	Error(msg string, args ...interface{})
	Debug(msg string, args ...interface{})
}

// Recovery handles pipeline recovery on system restart.
type Recovery struct {
	store  *PipelineStore
	logger RecoveryLogger
}

// NewRecovery creates a new recovery handler.
func NewRecovery(store *PipelineStore, logger RecoveryLogger) *Recovery {
	if logger == nil {
		// Provide a no-op logger if none is provided
		logger = &noOpLogger{}
	}
	return &Recovery{
		store:  store,
		logger: logger,
	}
}

// RecoverableCheckpoint contains information about a recoverable pipeline state.
type RecoverableCheckpoint struct {
	PipelineRun    *PipelineRun
	LastCheckpoint *Checkpoint
	LastStageIndex int
	LastStageRun   *StageRun
}

// RecoverPipelines recovers all pipelines that were interrupted.
// This function should be called during daemon startup.
//
// Recovery process:
// 1. Find all pipelines with recoverable status (running, checkpoint, stage_complete, paused)
// 2. For each pipeline:
//    a. Load the pipeline and its stages from the database
//    b. Find the last completed stage
//    c. Load the most recent checkpoint (if available)
//    d. Prepare recovery information
// 3. Return list of pipelines ready to resume
//
// Returns:
// - List of recoverable checkpoints
// - Number of pipelines found
// - Number of pipelines with checkpoints
// - Any error that occurred
func (r *Recovery) RecoverPipelines() ([]*RecoverableCheckpoint, int, int, error) {
	r.logger.Info("Starting pipeline recovery on system startup")

	// Find all pipelines in recoverable states
	pipelines, err := r.store.FindRecoverablePipelines()
	if err != nil {
		r.logger.Error("Failed to find recoverable pipelines", "error", err)
		return nil, 0, 0, fmt.Errorf("failed to find recoverable pipelines: %w", err)
	}

	r.logger.Info("Found pipelines to recover", "count", len(pipelines))

	if len(pipelines) == 0 {
		r.logger.Info("No pipelines to recover")
		return nil, 0, 0, nil
	}

	checkpoints := make([]*RecoverableCheckpoint, 0, len(pipelines))
	checkpointCount := 0

	// Process each pipeline
	for i, pipeline := range pipelines {
		r.logger.Debug("Processing pipeline for recovery",
			"pipeline_id", pipeline.ID,
			"pipeline_name", pipeline.PipelineID,
			"status", pipeline.Status.String(),
			"current_stage_index", pipeline.CurrentStageIndex,
			"index", i+1,
			"total", len(pipelines))

		// Load complete pipeline with stages
		fullPipeline, err := r.store.LoadPipeline(pipeline.ID)
		if err != nil {
			r.logger.Warn("Failed to load pipeline for recovery",
				"pipeline_id", pipeline.ID,
				"error", err)
			continue
		}

		// Find the last completed stage
		lastStageIndex := -1
		var lastStageRun *StageRun

		if fullPipeline.Stages != nil {
			for j, stage := range fullPipeline.Stages {
				if stage.Status == StageCompleted {
					lastStageIndex = j
					stageRunCopy := stage // Copy to avoid pointer issues
					lastStageRun = &stageRunCopy
				}
			}
		}

		// Load the most recent checkpoint (if available)
		var lastCheckpoint *Checkpoint
		if fullPipeline.Status == PipelineCheckpoint {
			cp, err := r.store.LoadLatestCheckpoint(pipeline.ID)
			if err != nil {
				r.logger.Warn("Failed to load checkpoint for pipeline",
					"pipeline_id", pipeline.ID,
					"error", err)
			} else if cp != nil {
				lastCheckpoint = cp
				checkpointCount++
				r.logger.Debug("Loaded checkpoint for pipeline",
					"pipeline_id", pipeline.ID,
					"checkpoint_id", cp.ID,
					"checkpoint_version", cp.Version)
			}
		}

		// Create recoverable checkpoint record
		rc := &RecoverableCheckpoint{
			PipelineRun:    fullPipeline,
			LastCheckpoint: lastCheckpoint,
			LastStageIndex: lastStageIndex,
			LastStageRun:   lastStageRun,
		}

		checkpoints = append(checkpoints, rc)

		r.logger.Info("Pipeline ready for recovery",
			"pipeline_id", pipeline.ID,
			"pipeline_name", pipeline.PipelineID,
			"status", pipeline.Status.String(),
			"last_completed_stage_index", lastStageIndex,
			"has_checkpoint", lastCheckpoint != nil,
			"total_stages", pipeline.TotalStages,
			"current_stage_index", pipeline.CurrentStageIndex)
	}

	r.logger.Info("Pipeline recovery preparation complete",
		"total_recoverable", len(checkpoints),
		"with_checkpoints", checkpointCount)

	return checkpoints, len(pipelines), checkpointCount, nil
}

// ResumeFromCheckpoint resumes a pipeline from its checkpoint.
// This involves:
// 1. Validating the checkpoint
// 2. Updating pipeline status to running
// 3. Preparing to execute the next stage
//
// Returns:
// - The next stage index to execute
// - Any error that occurred
func (r *Recovery) ResumeFromCheckpoint(rc *RecoverableCheckpoint) (int, error) {
	if rc == nil {
		return -1, fmt.Errorf("recoverable checkpoint cannot be nil")
	}

	if rc.PipelineRun == nil {
		return -1, fmt.Errorf("pipeline run cannot be nil")
	}

	pipeline := rc.PipelineRun
	r.logger.Info("Resuming pipeline from checkpoint",
		"pipeline_id", pipeline.ID,
		"pipeline_name", pipeline.PipelineID,
		"current_stage_index", pipeline.CurrentStageIndex,
		"total_stages", pipeline.TotalStages)

	// Calculate the next stage to execute
	nextStageIndex := rc.LastStageIndex + 1

	// Validate that there are more stages to run
	if nextStageIndex >= pipeline.TotalStages {
		r.logger.Warn("No more stages to execute after recovery",
			"pipeline_id", pipeline.ID,
			"next_stage_index", nextStageIndex,
			"total_stages", pipeline.TotalStages)
		return -1, fmt.Errorf("no more stages to execute (next: %d, total: %d)", nextStageIndex, pipeline.TotalStages)
	}

	// Update pipeline status to running (from checkpoint state)
	pipeline.Status = PipelineRunning
	pipeline.CurrentStageIndex = nextStageIndex

	// Update the started_at if not already set
	if pipeline.StartedAt == nil {
		now := time.Now()
		pipeline.StartedAt = &now
	}

	// Persist the updated pipeline
	if err := r.store.SavePipeline(pipeline); err != nil {
		r.logger.Error("Failed to update pipeline status during recovery",
			"pipeline_id", pipeline.ID,
			"error", err)
		return -1, fmt.Errorf("failed to update pipeline status: %w", err)
	}

	r.logger.Info("Pipeline resumed successfully",
		"pipeline_id", pipeline.ID,
		"next_stage_index", nextStageIndex,
		"checkpoint_version", func() int {
			if rc.LastCheckpoint != nil {
				return rc.LastCheckpoint.Version
			}
			return -1
		}())

	return nextStageIndex, nil
}

// ResumeFromLastStage resumes a pipeline from the last completed stage (when no checkpoint exists).
// This is used when we know the last completed stage but don't have a checkpoint.
//
// Returns:
// - The next stage index to execute
// - Any error that occurred
func (r *Recovery) ResumeFromLastStage(rc *RecoverableCheckpoint) (int, error) {
	if rc == nil {
		return -1, fmt.Errorf("recoverable checkpoint cannot be nil")
	}

	if rc.PipelineRun == nil {
		return -1, fmt.Errorf("pipeline run cannot be nil")
	}

	pipeline := rc.PipelineRun
	r.logger.Info("Resuming pipeline from last completed stage",
		"pipeline_id", pipeline.ID,
		"pipeline_name", pipeline.PipelineID,
		"last_completed_stage_index", rc.LastStageIndex,
		"total_stages", pipeline.TotalStages)

	// Calculate the next stage to execute
	nextStageIndex := rc.LastStageIndex + 1

	// Validate that there are more stages to run
	if nextStageIndex >= pipeline.TotalStages {
		r.logger.Warn("No more stages to execute",
			"pipeline_id", pipeline.ID,
			"next_stage_index", nextStageIndex,
			"total_stages", pipeline.TotalStages)
		return -1, fmt.Errorf("no more stages to execute (next: %d, total: %d)", nextStageIndex, pipeline.TotalStages)
	}

	// Update pipeline status to running
	pipeline.Status = PipelineRunning
	pipeline.CurrentStageIndex = nextStageIndex

	// Update the started_at if not already set
	if pipeline.StartedAt == nil {
		now := time.Now()
		pipeline.StartedAt = &now
	}

	// Persist the updated pipeline
	if err := r.store.SavePipeline(pipeline); err != nil {
		r.logger.Error("Failed to update pipeline status during recovery",
			"pipeline_id", pipeline.ID,
			"error", err)
		return -1, fmt.Errorf("failed to update pipeline status: %w", err)
	}

	r.logger.Info("Pipeline resumed from last stage",
		"pipeline_id", pipeline.ID,
		"next_stage_index", nextStageIndex,
		"skipped_stages", nextStageIndex)

	return nextStageIndex, nil
}

// DetermineRecoveryAction determines which recovery strategy should be used for a pipeline.
func (r *Recovery) DetermineRecoveryAction(rc *RecoverableCheckpoint) RecoveryAction {
	if rc == nil {
		return RecoveryActionAbort
	}

	if rc.PipelineRun == nil {
		return RecoveryActionAbort
	}

	pipeline := rc.PipelineRun

	// If we have a checkpoint and pipeline is in checkpoint state, resume from checkpoint
	if rc.LastCheckpoint != nil && pipeline.Status == PipelineCheckpoint {
		return RecoveryActionResumeCheckpoint
	}

	// If we have stages completed, resume from last stage
	if rc.LastStageIndex >= 0 {
		return RecoveryActionResumeLastStage
	}

	// If no stages completed and no checkpoint, restart from beginning
	if pipeline.CurrentStageIndex == 0 && rc.LastStageIndex < 0 {
		return RecoveryActionRestartPipeline
	}

	// Default: abort (unknown state)
	r.logger.Warn("Unknown recovery state for pipeline",
		"pipeline_id", pipeline.ID,
		"status", pipeline.Status.String(),
		"last_stage_index", rc.LastStageIndex,
		"has_checkpoint", rc.LastCheckpoint != nil)

	return RecoveryActionAbort
}

// RecoveryAction represents the action to take for pipeline recovery.
type RecoveryAction string

const (
	// RecoveryActionResumeCheckpoint resumes from the latest checkpoint
	RecoveryActionResumeCheckpoint RecoveryAction = "resume_checkpoint"

	// RecoveryActionResumeLastStage resumes from the last completed stage
	RecoveryActionResumeLastStage RecoveryAction = "resume_last_stage"

	// RecoveryActionRestartPipeline restarts the pipeline from the beginning
	RecoveryActionRestartPipeline RecoveryAction = "restart_pipeline"

	// RecoveryActionAbort aborts the recovery (unrecoverable state)
	RecoveryActionAbort RecoveryAction = "abort"
)

// RecoveryStats contains statistics about the recovery process.
type RecoveryStats struct {
	TotalPipelines      int
	ResumedFromCheckpoint int
	ResumedFromLastStage int
	Restarted           int
	Aborted             int
	Duration            time.Duration
}

// RecoverAllPipelines performs full recovery of all interrupted pipelines.
// This is the main entry point for recovery during daemon startup.
//
// This function:
// 1. Finds all recoverable pipelines
// 2. Determines the recovery action for each
// 3. Updates their status appropriately
// 4. Returns statistics about the recovery
func (r *Recovery) RecoverAllPipelines() (*RecoveryStats, error) {
	startTime := time.Now()
	stats := &RecoveryStats{}

	r.logger.Info("Starting full pipeline recovery")

	// Find all recoverable pipelines
	checkpoints, totalFound, checkpointCount, err := r.RecoverPipelines()
	if err != nil {
		r.logger.Error("Failed to initiate recovery", "error", err)
		return stats, err
	}

	stats.TotalPipelines = totalFound

	if totalFound == 0 {
		r.logger.Info("No pipelines to recover")
		stats.Duration = time.Since(startTime)
		return stats, nil
	}

	r.logger.Info("Processing recovery for pipelines",
		"total", totalFound,
		"with_checkpoints", checkpointCount)

	// Process each recoverable pipeline
	for i, rc := range checkpoints {
		if rc == nil || rc.PipelineRun == nil {
			continue
		}

		pipeline := rc.PipelineRun
		r.logger.Debug("Processing recovery action",
			"pipeline_id", pipeline.ID,
			"index", i+1,
			"total", len(checkpoints))

		// Determine recovery action
		action := r.DetermineRecoveryAction(rc)

		switch action {
		case RecoveryActionResumeCheckpoint:
			r.logger.Info("Resuming from checkpoint",
				"pipeline_id", pipeline.ID,
				"checkpoint_version", rc.LastCheckpoint.Version)
			if _, err := r.ResumeFromCheckpoint(rc); err != nil {
				r.logger.Error("Failed to resume from checkpoint",
					"pipeline_id", pipeline.ID,
					"error", err)
				stats.Aborted++
			} else {
				stats.ResumedFromCheckpoint++
			}

		case RecoveryActionResumeLastStage:
			r.logger.Info("Resuming from last completed stage",
				"pipeline_id", pipeline.ID,
				"last_stage_index", rc.LastStageIndex)
			if _, err := r.ResumeFromLastStage(rc); err != nil {
				r.logger.Error("Failed to resume from last stage",
					"pipeline_id", pipeline.ID,
					"error", err)
				stats.Aborted++
			} else {
				stats.ResumedFromLastStage++
			}

		case RecoveryActionRestartPipeline:
			r.logger.Info("Restarting pipeline from beginning",
				"pipeline_id", pipeline.ID)
			// Reset pipeline to initial state
			pipeline.Status = PipelineRunning
			pipeline.CurrentStageIndex = 0
			now := time.Now()
			pipeline.StartedAt = &now

			if err := r.store.SavePipeline(pipeline); err != nil {
				r.logger.Error("Failed to restart pipeline",
					"pipeline_id", pipeline.ID,
					"error", err)
				stats.Aborted++
			} else {
				stats.Restarted++
			}

		case RecoveryActionAbort:
			r.logger.Warn("Aborting recovery for pipeline in unrecoverable state",
				"pipeline_id", pipeline.ID,
				"status", pipeline.Status.String())
			stats.Aborted++
		}
	}

	stats.Duration = time.Since(startTime)

	r.logger.Info("Pipeline recovery complete",
		"total", stats.TotalPipelines,
		"resumed_from_checkpoint", stats.ResumedFromCheckpoint,
		"resumed_from_last_stage", stats.ResumedFromLastStage,
		"restarted", stats.Restarted,
		"aborted", stats.Aborted,
		"duration_ms", stats.Duration.Milliseconds())

	return stats, nil
}

// CreateCheckpointOnShutdown creates a checkpoint for all running pipelines before shutdown.
// This should be called during graceful shutdown to preserve state.
func (r *Recovery) CreateCheckpointOnShutdown(app core.App) error {
	r.logger.Info("Creating checkpoints for running pipelines before shutdown")

	// Find all running pipelines
	runningPipelines, err := r.store.ListPipelinesByStatus(PipelineRunning)
	if err != nil {
		r.logger.Error("Failed to find running pipelines for pre-shutdown checkpoint", "error", err)
		return fmt.Errorf("failed to find running pipelines: %w", err)
	}

	if len(runningPipelines) == 0 {
		r.logger.Info("No running pipelines to checkpoint")
		return nil
	}

	r.logger.Info("Creating checkpoints for running pipelines", "count", len(runningPipelines))

	for _, pipeline := range runningPipelines {
		// Create a checkpoint with PreShutdown reason
		pipelineStateData, _ := json.Marshal(pipeline)
		stageStatesData, _ := json.Marshal(pipeline.Stages)

		checkpoint := &Checkpoint{
			ID:            generateID(),
			PipelineRunID: pipeline.ID,
			Version:       0, // Will be overwritten by storage
			PipelineState: pipelineStateData,
			StageStates:   stageStatesData,
			Reason:        CheckpointPreShutdown,
			CreatedAt:     time.Now(),
			SizeBytes:     int64(len(pipelineStateData) + len(stageStatesData)),
		}

		if err := r.store.SaveCheckpoint(checkpoint); err != nil {
			r.logger.Error("Failed to create pre-shutdown checkpoint",
				"pipeline_id", pipeline.ID,
				"error", err)
		} else {
			r.logger.Info("Created pre-shutdown checkpoint",
				"pipeline_id", pipeline.ID,
				"checkpoint_id", checkpoint.ID)
		}

		// Update pipeline status to checkpoint
		pipeline.Status = PipelineCheckpoint
		now := time.Now()
		pipeline.LastCheckpointAt = &now

		if err := r.store.SavePipeline(pipeline); err != nil {
			r.logger.Error("Failed to update pipeline status to checkpoint",
				"pipeline_id", pipeline.ID,
				"error", err)
		}
	}

	r.logger.Info("Pre-shutdown checkpointing complete", "count", len(runningPipelines))
	return nil
}

// noOpLogger is a logger that discards all messages.
type noOpLogger struct{}

func (n *noOpLogger) Info(msg string, args ...interface{})   {}
func (n *noOpLogger) Warn(msg string, args ...interface{})   {}
func (n *noOpLogger) Error(msg string, args ...interface{})  {}
func (n *noOpLogger) Debug(msg string, args ...interface{})  {}

// NewDefaultLogger creates a slog-based logger for recovery.
func NewDefaultLogger(l *slog.Logger) RecoveryLogger {
	if l == nil {
		return &noOpLogger{}
	}
	return &slogAdapter{log: l}
}

// slogAdapter adapts slog.Logger to our RecoveryLogger interface.
type slogAdapter struct {
	log *slog.Logger
}

func (s *slogAdapter) Info(msg string, args ...interface{}) {
	s.log.Info(msg, convertArgs(args)...)
}

func (s *slogAdapter) Warn(msg string, args ...interface{}) {
	s.log.Warn(msg, convertArgs(args)...)
}

func (s *slogAdapter) Error(msg string, args ...interface{}) {
	s.log.Error(msg, convertArgs(args)...)
}

func (s *slogAdapter) Debug(msg string, args ...interface{}) {
	s.log.Debug(msg, convertArgs(args)...)
}

// convertArgs converts variadic interface{} args to slog.Attr slice.
func convertArgs(args []interface{}) []interface{} {
	// slog expects alternating key-value pairs or slog.Attr
	// We'll pass them through as-is since they should already be in the right format
	return args
}
