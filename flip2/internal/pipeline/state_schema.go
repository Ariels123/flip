// Package pipeline provides state management for multi-stage pipeline execution
// with automatic recovery capabilities.
//
// State Machine Overview:
//
//	                                  +-------------+
//	                                  |   PENDING   |
//	                                  +------+------+
//	                                         |
//	                                         | start()
//	                                         v
//	                         +---------------+---------------+
//	                         |            RUNNING            |
//	                         +------+--------+--------+------+
//	                                |        |        |
//	                   stageComplete|  fail()|  checkpoint()
//	                                v        v        v
//	+-------------+        +--------+--+  +--+------+ +--+---------+
//	|  COMPLETED  | <------+ STAGE_    |  | FAILED  | | CHECKPOINT |
//	+-------------+        | COMPLETE  |  +---------+ +------------+
//	       ^               +-----+-----+       ^            |
//	       |                     |             |            |
//	       | allStagesDone()     | nextStage() | recover()  | resume()
//	       +---------------------+             +-----------+
//
// Recovery Flow:
//
//	1. System crash occurs
//	2. On restart, find all RUNNING or CHECKPOINT pipelines
//	3. For each pipeline:
//	   a. If CHECKPOINT exists, restore from checkpoint
//	   b. If RUNNING, check last completed stage
//	   c. Resume from last known good state
//	4. Continue execution
//
// Stage Artifact Flow:
//
//	Stage 1 (Gemini: gather) --> artifacts --> Stage 2 (Claude: analyze) --> artifacts --> Stage 3 (output)
//
package pipeline

import (
	"encoding/json"
	"fmt"
	"time"
)

// =============================================================================
// PIPELINE-LEVEL STATE
// =============================================================================

// PipelineStatus represents the overall status of a pipeline run.
type PipelineStatus string

const (
	// PipelinePending indicates the pipeline has been created but not started.
	PipelinePending PipelineStatus = "pending"

	// PipelineRunning indicates the pipeline is actively executing stages.
	PipelineRunning PipelineStatus = "running"

	// PipelineStageComplete indicates a stage has completed and the pipeline
	// is ready to move to the next stage.
	PipelineStageComplete PipelineStatus = "stage_complete"

	// PipelineCompleted indicates all stages have finished successfully.
	PipelineCompleted PipelineStatus = "completed"

	// PipelineFailed indicates the pipeline failed with an unrecoverable error.
	PipelineFailed PipelineStatus = "failed"

	// PipelineCheckpoint indicates the pipeline state has been saved for recovery.
	// This is set when creating a checkpoint, and cleared when resuming.
	PipelineCheckpoint PipelineStatus = "checkpoint"

	// PipelinePaused indicates the pipeline was manually paused by a user/coordinator.
	PipelinePaused PipelineStatus = "paused"

	// PipelineCancelled indicates the pipeline was cancelled before completion.
	PipelineCancelled PipelineStatus = "cancelled"
)

// String returns the string representation of the pipeline status.
func (s PipelineStatus) String() string {
	return string(s)
}

// IsTerminal returns true if the status is a terminal state (no further transitions).
func (s PipelineStatus) IsTerminal() bool {
	return s == PipelineCompleted || s == PipelineFailed || s == PipelineCancelled
}

// IsRecoverable returns true if the pipeline can be recovered from this state.
func (s PipelineStatus) IsRecoverable() bool {
	return s == PipelineRunning || s == PipelineCheckpoint || s == PipelineStageComplete || s == PipelinePaused
}

// =============================================================================
// STAGE-LEVEL STATE
// =============================================================================

// StageStatus represents the status of an individual pipeline stage.
type StageStatus string

const (
	// StagePending indicates the stage has not started yet.
	StagePending StageStatus = "pending"

	// StageRunning indicates the stage is currently executing.
	StageRunning StageStatus = "running"

	// StageCompleted indicates the stage finished successfully.
	StageCompleted StageStatus = "completed"

	// StageFailed indicates the stage failed (may be retried).
	StageFailed StageStatus = "failed"

	// StageSkipped indicates the stage was skipped (conditional execution).
	StageSkipped StageStatus = "skipped"

	// StageWaiting indicates the stage is waiting for dependencies.
	StageWaiting StageStatus = "waiting"
)

// String returns the string representation of the stage status.
func (s StageStatus) String() string {
	return string(s)
}

// =============================================================================
// PIPELINE RUN
// =============================================================================

// PipelineRun represents a single execution of a pipeline.
// This maps to the pipeline_runs SQLite table.
type PipelineRun struct {
	// ID is the unique identifier for this pipeline run (UUID).
	ID string `json:"id" db:"id"`

	// PipelineID references the pipeline definition (e.g., "research", "data-analyze").
	PipelineID string `json:"pipeline_id" db:"pipeline_id"`

	// Status is the current status of the pipeline run.
	Status PipelineStatus `json:"status" db:"status"`

	// CurrentStageIndex is the index of the currently executing stage (0-based).
	CurrentStageIndex int `json:"current_stage_index" db:"current_stage_index"`

	// TotalStages is the total number of stages in this pipeline.
	TotalStages int `json:"total_stages" db:"total_stages"`

	// Input is the initial input to the pipeline (JSON).
	Input json.RawMessage `json:"input" db:"input"`

	// FinalOutput is the output from the last stage (JSON), set when completed.
	FinalOutput json.RawMessage `json:"final_output,omitempty" db:"final_output"`

	// Error contains the error message if the pipeline failed.
	Error string `json:"error,omitempty" db:"error"`

	// ErrorStage is the stage name where the error occurred.
	ErrorStage string `json:"error_stage,omitempty" db:"error_stage"`

	// RetryCount is the total number of retries across all stages.
	RetryCount int `json:"retry_count" db:"retry_count"`

	// MaxRetries is the maximum total retries allowed for the pipeline.
	MaxRetries int `json:"max_retries" db:"max_retries"`

	// Priority determines execution order (higher = more urgent).
	Priority int `json:"priority" db:"priority"`

	// AssignedAgent is the coordinator agent managing this pipeline.
	AssignedAgent string `json:"assigned_agent,omitempty" db:"assigned_agent"`

	// CreatedAt is when the pipeline run was created.
	CreatedAt time.Time `json:"created_at" db:"created_at"`

	// StartedAt is when the pipeline started executing.
	StartedAt *time.Time `json:"started_at,omitempty" db:"started_at"`

	// CompletedAt is when the pipeline finished (success or failure).
	CompletedAt *time.Time `json:"completed_at,omitempty" db:"completed_at"`

	// UpdatedAt is the last time any field was updated.
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`

	// LastCheckpointAt is when the last checkpoint was created.
	LastCheckpointAt *time.Time `json:"last_checkpoint_at,omitempty" db:"last_checkpoint_at"`

	// Metadata stores arbitrary key-value pairs for extensibility.
	Metadata map[string]interface{} `json:"metadata,omitempty" db:"metadata"`

	// Stages contains the state of each stage (populated from stage_runs table).
	Stages []StageRun `json:"stages,omitempty" db:"-"`
}

// Duration returns the elapsed time for the pipeline run.
func (p *PipelineRun) Duration() time.Duration {
	if p.StartedAt == nil {
		return 0
	}
	if p.CompletedAt != nil {
		return p.CompletedAt.Sub(*p.StartedAt)
	}
	return time.Since(*p.StartedAt)
}

// Progress returns the completion percentage (0-100).
func (p *PipelineRun) Progress() float64 {
	if p.TotalStages == 0 {
		return 0
	}
	completedStages := 0
	for _, stage := range p.Stages {
		if stage.Status == StageCompleted || stage.Status == StageSkipped {
			completedStages++
		}
	}
	return float64(completedStages) / float64(p.TotalStages) * 100
}

// =============================================================================
// STAGE RUN
// =============================================================================

// StageRun represents the execution state of a single pipeline stage.
// This maps to the stage_runs SQLite table.
type StageRun struct {
	// ID is the unique identifier for this stage run (UUID).
	ID string `json:"id" db:"id"`

	// PipelineRunID references the parent pipeline run.
	PipelineRunID string `json:"pipeline_run_id" db:"pipeline_run_id"`

	// StageName is the name of this stage (e.g., "gather", "analyze", "format").
	StageName string `json:"stage_name" db:"stage_name"`

	// StageIndex is the position of this stage in the pipeline (0-based).
	StageIndex int `json:"stage_index" db:"stage_index"`

	// Status is the current status of this stage.
	Status StageStatus `json:"status" db:"status"`

	// Backend is the LLM backend to use (e.g., "claude", "gemini").
	Backend string `json:"backend" db:"backend"`

	// Model is the specific model to use (optional, uses backend default if empty).
	Model string `json:"model,omitempty" db:"model"`

	// Input is the input to this stage (JSON).
	// For stage 0, this comes from PipelineRun.Input.
	// For subsequent stages, this comes from the previous stage's Output.
	Input json.RawMessage `json:"input,omitempty" db:"input"`

	// Output is the output from this stage (JSON).
	// This becomes the input for the next stage.
	Output json.RawMessage `json:"output,omitempty" db:"output"`

	// Error contains the error message if the stage failed.
	Error string `json:"error,omitempty" db:"error"`

	// RetryCount is the number of times this stage has been retried.
	RetryCount int `json:"retry_count" db:"retry_count"`

	// MaxRetries is the maximum retries allowed for this stage.
	MaxRetries int `json:"max_retries" db:"max_retries"`

	// StartedAt is when this stage started executing.
	StartedAt *time.Time `json:"started_at,omitempty" db:"started_at"`

	// CompletedAt is when this stage finished (success or failure).
	CompletedAt *time.Time `json:"completed_at,omitempty" db:"completed_at"`

	// TaskID references the task record if this stage spawns an agent task.
	TaskID string `json:"task_id,omitempty" db:"task_id"`

	// AgentID is the agent assigned to execute this stage.
	AgentID string `json:"agent_id,omitempty" db:"agent_id"`

	// Metrics contains performance metrics for this stage.
	Metrics StageMetrics `json:"metrics,omitempty" db:"metrics"`
}

// Duration returns the elapsed time for this stage.
func (s *StageRun) Duration() time.Duration {
	if s.StartedAt == nil {
		return 0
	}
	if s.CompletedAt != nil {
		return s.CompletedAt.Sub(*s.StartedAt)
	}
	return time.Since(*s.StartedAt)
}

// StageMetrics contains performance metrics for a stage.
type StageMetrics struct {
	// TokensIn is the number of input tokens.
	TokensIn int `json:"tokens_in,omitempty"`

	// TokensOut is the number of output tokens.
	TokensOut int `json:"tokens_out,omitempty"`

	// Cost is the estimated cost in USD.
	Cost float64 `json:"cost,omitempty"`

	// DurationMs is the execution time in milliseconds.
	DurationMs int64 `json:"duration_ms,omitempty"`

	// MemoryUsedBytes is the peak memory usage.
	MemoryUsedBytes int64 `json:"memory_used_bytes,omitempty"`
}

// =============================================================================
// STAGE ARTIFACTS
// =============================================================================

// StageArtifact represents an artifact produced by a stage.
// Artifacts are intermediate outputs that can be referenced by later stages.
// This maps to the stage_artifacts SQLite table.
type StageArtifact struct {
	// ID is the unique identifier for this artifact (UUID).
	ID string `json:"id" db:"id"`

	// PipelineRunID references the parent pipeline run.
	PipelineRunID string `json:"pipeline_run_id" db:"pipeline_run_id"`

	// StageRunID references the stage that produced this artifact.
	StageRunID string `json:"stage_run_id" db:"stage_run_id"`

	// Name is the artifact name (e.g., "search_results", "analysis", "summary").
	Name string `json:"name" db:"name"`

	// Type indicates the artifact type (json, text, file, url).
	Type ArtifactType `json:"type" db:"type"`

	// Data contains the artifact data (inline for json/text, path for file).
	Data json.RawMessage `json:"data" db:"data"`

	// ContentType is the MIME type (e.g., "application/json", "text/plain").
	ContentType string `json:"content_type" db:"content_type"`

	// SizeBytes is the size of the artifact in bytes.
	SizeBytes int64 `json:"size_bytes" db:"size_bytes"`

	// Checksum is the SHA256 hash of the artifact data.
	Checksum string `json:"checksum,omitempty" db:"checksum"`

	// CreatedAt is when the artifact was created.
	CreatedAt time.Time `json:"created_at" db:"created_at"`

	// ExpiresAt is when the artifact should be garbage collected (optional).
	ExpiresAt *time.Time `json:"expires_at,omitempty" db:"expires_at"`

	// Metadata stores arbitrary key-value pairs.
	Metadata map[string]interface{} `json:"metadata,omitempty" db:"metadata"`
}

// ArtifactType represents the type of stage artifact.
type ArtifactType string

const (
	// ArtifactJSON is structured JSON data.
	ArtifactJSON ArtifactType = "json"

	// ArtifactText is plain text data.
	ArtifactText ArtifactType = "text"

	// ArtifactFile is a reference to a file on disk.
	ArtifactFile ArtifactType = "file"

	// ArtifactURL is a reference to a remote resource.
	ArtifactURL ArtifactType = "url"

	// ArtifactBinary is raw binary data.
	ArtifactBinary ArtifactType = "binary"
)

// =============================================================================
// CHECKPOINT
// =============================================================================

// Checkpoint represents a saved pipeline state for recovery.
// Checkpoints are created periodically and after each stage completes.
type Checkpoint struct {
	// ID is the unique identifier for this checkpoint (UUID).
	ID string `json:"id" db:"id"`

	// PipelineRunID references the pipeline run.
	PipelineRunID string `json:"pipeline_run_id" db:"pipeline_run_id"`

	// Version is an incrementing counter for checkpoint ordering.
	Version int `json:"version" db:"version"`

	// PipelineState is the full serialized state of the pipeline.
	PipelineState json.RawMessage `json:"pipeline_state" db:"pipeline_state"`

	// StageStates contains the serialized states of all stages.
	StageStates json.RawMessage `json:"stage_states" db:"stage_states"`

	// ArtifactRefs contains references to artifacts (not the actual data).
	ArtifactRefs json.RawMessage `json:"artifact_refs" db:"artifact_refs"`

	// CreatedAt is when the checkpoint was created.
	CreatedAt time.Time `json:"created_at" db:"created_at"`

	// Reason describes why the checkpoint was created.
	Reason CheckpointReason `json:"reason" db:"reason"`

	// SizeBytes is the total size of the checkpoint in bytes.
	SizeBytes int64 `json:"size_bytes" db:"size_bytes"`
}

// CheckpointReason describes why a checkpoint was created.
type CheckpointReason string

const (
	// CheckpointStageComplete is created after each stage completes.
	CheckpointStageComplete CheckpointReason = "stage_complete"

	// CheckpointPeriodic is created by the periodic checkpoint timer.
	CheckpointPeriodic CheckpointReason = "periodic"

	// CheckpointManual is created by explicit user/coordinator request.
	CheckpointManual CheckpointReason = "manual"

	// CheckpointPreShutdown is created before planned shutdown.
	CheckpointPreShutdown CheckpointReason = "pre_shutdown"

	// CheckpointError is created when an error occurs (for debugging).
	CheckpointError CheckpointReason = "error"
)

// =============================================================================
// STATE TRANSITIONS
// =============================================================================

// StateTransition represents a valid state transition for pipelines.
type StateTransition struct {
	From      PipelineStatus
	To        PipelineStatus
	Condition string
}

// ValidPipelineTransitions defines all valid state transitions for pipelines.
var ValidPipelineTransitions = []StateTransition{
	// Starting
	{PipelinePending, PipelineRunning, "start()"},
	{PipelinePending, PipelineCancelled, "cancel()"},

	// Running transitions
	{PipelineRunning, PipelineStageComplete, "stageComplete()"},
	{PipelineRunning, PipelineFailed, "fail() - unrecoverable error"},
	{PipelineRunning, PipelineCheckpoint, "checkpoint() - save state"},
	{PipelineRunning, PipelinePaused, "pause() - manual pause"},
	{PipelineRunning, PipelineCancelled, "cancel()"},

	// Stage complete transitions
	{PipelineStageComplete, PipelineRunning, "nextStage() - more stages"},
	{PipelineStageComplete, PipelineCompleted, "allStagesDone()"},
	{PipelineStageComplete, PipelineCheckpoint, "checkpoint()"},
	{PipelineStageComplete, PipelinePaused, "pause()"},
	{PipelineStageComplete, PipelineCancelled, "cancel()"},

	// Checkpoint transitions
	{PipelineCheckpoint, PipelineRunning, "resume() - continue execution"},
	{PipelineCheckpoint, PipelinePaused, "pause()"},
	{PipelineCheckpoint, PipelineCancelled, "cancel()"},

	// Paused transitions
	{PipelinePaused, PipelineRunning, "resume()"},
	{PipelinePaused, PipelineCancelled, "cancel()"},

	// Recovery (special case - from any recoverable state)
	// Handled by IsRecoverable() method
}

// CanTransition checks if a state transition is valid.
func CanTransition(from, to PipelineStatus) bool {
	for _, t := range ValidPipelineTransitions {
		if t.From == from && t.To == to {
			return true
		}
	}
	return false
}

// ValidStageTransitions defines all valid state transitions for stages.
var ValidStageTransitions = []struct {
	From      StageStatus
	To        StageStatus
	Condition string
}{
	{StagePending, StageWaiting, "dependencies not ready"},
	{StagePending, StageRunning, "start()"},
	{StagePending, StageSkipped, "skip() - conditional"},

	{StageWaiting, StageRunning, "dependencies ready"},
	{StageWaiting, StageSkipped, "skip()"},

	{StageRunning, StageCompleted, "success()"},
	{StageRunning, StageFailed, "fail()"},

	{StageFailed, StagePending, "retry() - if retries remaining"},
	{StageFailed, StageRunning, "retry() - immediate"},
}

// =============================================================================
// RECOVERY LOGIC
// =============================================================================

// RecoveryStrategy defines how to recover a pipeline from a given state.
type RecoveryStrategy string

const (
	// RecoveryRestartStage restarts the current stage from the beginning.
	RecoveryRestartStage RecoveryStrategy = "restart_stage"

	// RecoveryResumeFromCheckpoint resumes from the last checkpoint.
	RecoveryResumeFromCheckpoint RecoveryStrategy = "resume_checkpoint"

	// RecoverySkipStage skips the failed stage and continues.
	RecoverySkipStage RecoveryStrategy = "skip_stage"

	// RecoveryAbort aborts the pipeline.
	RecoveryAbort RecoveryStrategy = "abort"

	// RecoveryManual requires manual intervention.
	RecoveryManual RecoveryStrategy = "manual"
)

// RecoveryContext contains information needed for pipeline recovery.
type RecoveryContext struct {
	// PipelineRun is the pipeline being recovered.
	PipelineRun *PipelineRun

	// LastCheckpoint is the most recent checkpoint (may be nil).
	LastCheckpoint *Checkpoint

	// FailedStage is the stage that caused the failure (may be nil).
	FailedStage *StageRun

	// Reason is why recovery is needed.
	Reason string

	// Timestamp is when recovery was initiated.
	Timestamp time.Time
}

// DetermineRecoveryStrategy determines the best recovery strategy.
func DetermineRecoveryStrategy(ctx *RecoveryContext) RecoveryStrategy {
	if ctx.PipelineRun == nil {
		return RecoveryAbort
	}

	status := ctx.PipelineRun.Status

	// Terminal states cannot be recovered
	if status.IsTerminal() {
		return RecoveryAbort
	}

	// If we have a checkpoint, prefer resuming from it
	if ctx.LastCheckpoint != nil && status == PipelineCheckpoint {
		return RecoveryResumeFromCheckpoint
	}

	// If a stage failed, check if we can retry
	if ctx.FailedStage != nil {
		if ctx.FailedStage.RetryCount < ctx.FailedStage.MaxRetries {
			return RecoveryRestartStage
		}
		// Max retries exceeded - check if stage can be skipped
		// (This would need to be configured per-stage in the pipeline definition)
		return RecoveryAbort
	}

	// Running or stage_complete - try to resume from last known state
	if status == PipelineRunning || status == PipelineStageComplete {
		if ctx.LastCheckpoint != nil {
			return RecoveryResumeFromCheckpoint
		}
		return RecoveryRestartStage
	}

	// Paused - just resume
	if status == PipelinePaused {
		return RecoveryRestartStage
	}

	return RecoveryManual
}

// =============================================================================
// POCKETBASE/SQLITE SCHEMA
// =============================================================================

// PocketBaseCollections returns the PocketBase collection definitions.
// These should be added via a migration file.
const PocketBaseCollections = `
-- Pipeline Runs Table
-- Tracks the overall execution of a pipeline instance

CREATE TABLE IF NOT EXISTS pipeline_runs (
    id TEXT PRIMARY KEY,
    pipeline_id TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    current_stage_index INTEGER NOT NULL DEFAULT 0,
    total_stages INTEGER NOT NULL DEFAULT 0,
    input TEXT,
    final_output TEXT,
    error TEXT,
    error_stage TEXT,
    retry_count INTEGER NOT NULL DEFAULT 0,
    max_retries INTEGER NOT NULL DEFAULT 3,
    priority INTEGER NOT NULL DEFAULT 0,
    assigned_agent TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    started_at DATETIME,
    completed_at DATETIME,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_checkpoint_at DATETIME,
    metadata TEXT
);

CREATE INDEX idx_pipeline_runs_status ON pipeline_runs(status);
CREATE INDEX idx_pipeline_runs_pipeline_id ON pipeline_runs(pipeline_id);
CREATE INDEX idx_pipeline_runs_priority ON pipeline_runs(priority DESC);
CREATE INDEX idx_pipeline_runs_assigned_agent ON pipeline_runs(assigned_agent);

-- Stage Runs Table
-- Tracks the execution of each stage within a pipeline

CREATE TABLE IF NOT EXISTS stage_runs (
    id TEXT PRIMARY KEY,
    pipeline_run_id TEXT NOT NULL,
    stage_name TEXT NOT NULL,
    stage_index INTEGER NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    backend TEXT NOT NULL,
    model TEXT,
    input TEXT,
    output TEXT,
    error TEXT,
    retry_count INTEGER NOT NULL DEFAULT 0,
    max_retries INTEGER NOT NULL DEFAULT 2,
    started_at DATETIME,
    completed_at DATETIME,
    task_id TEXT,
    agent_id TEXT,
    metrics TEXT,
    FOREIGN KEY (pipeline_run_id) REFERENCES pipeline_runs(id) ON DELETE CASCADE
);

CREATE INDEX idx_stage_runs_pipeline_run_id ON stage_runs(pipeline_run_id);
CREATE INDEX idx_stage_runs_status ON stage_runs(status);
CREATE INDEX idx_stage_runs_task_id ON stage_runs(task_id);

-- Stage Artifacts Table
-- Stores intermediate outputs that can be referenced by later stages

CREATE TABLE IF NOT EXISTS stage_artifacts (
    id TEXT PRIMARY KEY,
    pipeline_run_id TEXT NOT NULL,
    stage_run_id TEXT NOT NULL,
    name TEXT NOT NULL,
    type TEXT NOT NULL DEFAULT 'json',
    data TEXT,
    content_type TEXT NOT NULL DEFAULT 'application/json',
    size_bytes INTEGER NOT NULL DEFAULT 0,
    checksum TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at DATETIME,
    metadata TEXT,
    FOREIGN KEY (pipeline_run_id) REFERENCES pipeline_runs(id) ON DELETE CASCADE,
    FOREIGN KEY (stage_run_id) REFERENCES stage_runs(id) ON DELETE CASCADE
);

CREATE INDEX idx_stage_artifacts_pipeline_run_id ON stage_artifacts(pipeline_run_id);
CREATE INDEX idx_stage_artifacts_stage_run_id ON stage_artifacts(stage_run_id);
CREATE INDEX idx_stage_artifacts_name ON stage_artifacts(name);

-- Checkpoints Table
-- Stores saved pipeline state for recovery

CREATE TABLE IF NOT EXISTS pipeline_checkpoints (
    id TEXT PRIMARY KEY,
    pipeline_run_id TEXT NOT NULL,
    version INTEGER NOT NULL,
    pipeline_state TEXT NOT NULL,
    stage_states TEXT NOT NULL,
    artifact_refs TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    reason TEXT NOT NULL DEFAULT 'stage_complete',
    size_bytes INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY (pipeline_run_id) REFERENCES pipeline_runs(id) ON DELETE CASCADE
);

CREATE INDEX idx_pipeline_checkpoints_pipeline_run_id ON pipeline_checkpoints(pipeline_run_id);
CREATE INDEX idx_pipeline_checkpoints_version ON pipeline_checkpoints(version DESC);
`

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// NewPipelineRun creates a new pipeline run with default values.
func NewPipelineRun(pipelineID string, input json.RawMessage, totalStages int) *PipelineRun {
	now := time.Now()
	return &PipelineRun{
		ID:                generateID(),
		PipelineID:        pipelineID,
		Status:            PipelinePending,
		CurrentStageIndex: 0,
		TotalStages:       totalStages,
		Input:             input,
		RetryCount:        0,
		MaxRetries:        3,
		Priority:          0,
		CreatedAt:         now,
		UpdatedAt:         now,
		Stages:            make([]StageRun, 0, totalStages),
		Metadata:          make(map[string]interface{}),
	}
}

// NewStageRun creates a new stage run with default values.
func NewStageRun(pipelineRunID, stageName, backend string, stageIndex int) *StageRun {
	return &StageRun{
		ID:            generateID(),
		PipelineRunID: pipelineRunID,
		StageName:     stageName,
		StageIndex:    stageIndex,
		Status:        StagePending,
		Backend:       backend,
		RetryCount:    0,
		MaxRetries:    2,
		Metrics:       StageMetrics{},
	}
}

// generateID generates a unique ID for pipeline entities.
// Uses a simple timestamp-based approach; in production, use UUID.
func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// =============================================================================
// VALIDATION
// =============================================================================

// Validate checks if the pipeline run is in a valid state.
func (p *PipelineRun) Validate() error {
	if p.ID == "" {
		return fmt.Errorf("pipeline run ID is required")
	}
	if p.PipelineID == "" {
		return fmt.Errorf("pipeline ID is required")
	}
	if p.TotalStages < 1 {
		return fmt.Errorf("total stages must be at least 1")
	}
	if p.CurrentStageIndex < 0 || p.CurrentStageIndex >= p.TotalStages {
		if p.Status != PipelineCompleted {
			return fmt.Errorf("current stage index %d out of bounds [0, %d)",
				p.CurrentStageIndex, p.TotalStages)
		}
	}
	return nil
}

// Validate checks if the stage run is in a valid state.
func (s *StageRun) Validate() error {
	if s.ID == "" {
		return fmt.Errorf("stage run ID is required")
	}
	if s.PipelineRunID == "" {
		return fmt.Errorf("pipeline run ID is required")
	}
	if s.StageName == "" {
		return fmt.Errorf("stage name is required")
	}
	if s.Backend == "" {
		return fmt.Errorf("backend is required")
	}
	if s.StageIndex < 0 {
		return fmt.Errorf("stage index must be non-negative")
	}
	return nil
}

// =============================================================================
// EVENT TYPES
// =============================================================================

// PipelineEvent represents an event that occurred during pipeline execution.
// These can be used for logging, metrics, and debugging.
type PipelineEvent struct {
	// Type is the type of event.
	Type PipelineEventType `json:"type"`

	// PipelineRunID is the pipeline run this event belongs to.
	PipelineRunID string `json:"pipeline_run_id"`

	// StageRunID is the stage run this event belongs to (optional).
	StageRunID string `json:"stage_run_id,omitempty"`

	// Timestamp is when the event occurred.
	Timestamp time.Time `json:"timestamp"`

	// Data contains event-specific data.
	Data map[string]interface{} `json:"data,omitempty"`
}

// PipelineEventType represents the type of pipeline event.
type PipelineEventType string

const (
	EventPipelineCreated   PipelineEventType = "pipeline.created"
	EventPipelineStarted   PipelineEventType = "pipeline.started"
	EventPipelineCompleted PipelineEventType = "pipeline.completed"
	EventPipelineFailed    PipelineEventType = "pipeline.failed"
	EventPipelinePaused    PipelineEventType = "pipeline.paused"
	EventPipelineResumed   PipelineEventType = "pipeline.resumed"
	EventPipelineCancelled PipelineEventType = "pipeline.cancelled"

	EventStageStarted   PipelineEventType = "stage.started"
	EventStageCompleted PipelineEventType = "stage.completed"
	EventStageFailed    PipelineEventType = "stage.failed"
	EventStageRetried   PipelineEventType = "stage.retried"
	EventStageSkipped   PipelineEventType = "stage.skipped"

	EventCheckpointCreated PipelineEventType = "checkpoint.created"
	EventCheckpointLoaded  PipelineEventType = "checkpoint.loaded"

	EventArtifactCreated PipelineEventType = "artifact.created"
	EventArtifactDeleted PipelineEventType = "artifact.deleted"
)
