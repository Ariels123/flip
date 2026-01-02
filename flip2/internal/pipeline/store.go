// Package pipeline provides pipeline state management and persistence.
package pipeline

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

// PipelineStore handles persistence of pipeline state to SQLite via PocketBase.
// All pipeline runs, stages, artifacts, and checkpoints are stored durably.
type PipelineStore struct {
	app core.App
}

// NewPipelineStore creates a new PipelineStore instance.
func NewPipelineStore(app core.App) *PipelineStore {
	return &PipelineStore{
		app: app,
	}
}

// =============================================================================
// PIPELINE RUN OPERATIONS
// =============================================================================

// SavePipeline persists a pipeline run to the database.
// If the pipeline already exists, it will be updated.
func (s *PipelineStore) SavePipeline(pipeline *PipelineRun) error {
	if pipeline == nil {
		return fmt.Errorf("pipeline cannot be nil")
	}

	if err := pipeline.Validate(); err != nil {
		return fmt.Errorf("invalid pipeline: %w", err)
	}

	collection, err := s.app.FindCollectionByNameOrId("pipeline_runs")
	if err != nil {
		return fmt.Errorf("pipeline_runs collection not found: %w", err)
	}

	// Prepare metadata as JSON
	metadataJSON, err := json.Marshal(pipeline.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	// Check if the pipeline already exists
	record, err := s.app.FindRecordById(collection, pipeline.ID)
	isNew := err != nil

	if isNew {
		record = core.NewRecord(collection)
		record.Set("id", pipeline.ID)
	}

	// Set all fields
	record.Set("pipeline_id", pipeline.PipelineID)
	record.Set("status", pipeline.Status.String())
	record.Set("current_stage_index", pipeline.CurrentStageIndex)
	record.Set("total_stages", pipeline.TotalStages)
	record.Set("input", string(pipeline.Input))
	record.Set("final_output", string(pipeline.FinalOutput))
	record.Set("error", pipeline.Error)
	record.Set("error_stage", pipeline.ErrorStage)
	record.Set("retry_count", pipeline.RetryCount)
	record.Set("max_retries", pipeline.MaxRetries)
	record.Set("priority", pipeline.Priority)
	record.Set("assigned_agent", pipeline.AssignedAgent)
	record.Set("started_at", pipeline.StartedAt)
	record.Set("completed_at", pipeline.CompletedAt)
	record.Set("last_checkpoint_at", pipeline.LastCheckpointAt)
	record.Set("metadata", string(metadataJSON))

	// Save to database
	if err := s.app.Save(record); err != nil {
		return fmt.Errorf("failed to save pipeline: %w", err)
	}

	// Update the pipeline's UpdatedAt field
	pipeline.UpdatedAt = time.Now()

	return nil
}

// LoadPipeline retrieves a pipeline run from the database by ID.
// If the pipeline is not found, it returns nil and no error.
func (s *PipelineStore) LoadPipeline(id string) (*PipelineRun, error) {
	if id == "" {
		return nil, fmt.Errorf("pipeline ID cannot be empty")
	}

	collection, err := s.app.FindCollectionByNameOrId("pipeline_runs")
	if err != nil {
		return nil, fmt.Errorf("pipeline_runs collection not found: %w", err)
	}

	record, err := s.app.FindRecordById(collection, id)
	if err != nil {
		return nil, nil // Not found
	}

	pipeline := &PipelineRun{
		ID:                record.Get("id").(string),
		PipelineID:        record.Get("pipeline_id").(string),
		Status:            PipelineStatus(record.Get("status").(string)),
		CurrentStageIndex: int(record.Get("current_stage_index").(float64)),
		TotalStages:       int(record.Get("total_stages").(float64)),
		Input:             json.RawMessage(record.Get("input").(string)),
		FinalOutput:       json.RawMessage(record.Get("final_output").(string)),
		Error:             record.Get("error").(string),
		ErrorStage:        record.Get("error_stage").(string),
		RetryCount:        int(record.Get("retry_count").(float64)),
		MaxRetries:        int(record.Get("max_retries").(float64)),
		Priority:          int(record.Get("priority").(float64)),
		AssignedAgent:     record.Get("assigned_agent").(string),
		CreatedAt:         record.GetDateTime("created_at").Time(),
		UpdatedAt:         record.GetDateTime("updated_at").Time(),
	}

	// Parse optional date fields
	if startedAt := record.Get("started_at"); startedAt != nil && startedAt != "" {
		t := record.GetDateTime("started_at").Time()
		pipeline.StartedAt = &t
	}

	if completedAt := record.Get("completed_at"); completedAt != nil && completedAt != "" {
		t := record.GetDateTime("completed_at").Time()
		pipeline.CompletedAt = &t
	}

	if lastCheckpointAt := record.Get("last_checkpoint_at"); lastCheckpointAt != nil && lastCheckpointAt != "" {
		t := record.GetDateTime("last_checkpoint_at").Time()
		pipeline.LastCheckpointAt = &t
	}

	// Parse metadata
	metadataStr := record.Get("metadata").(string)
	if metadataStr != "" {
		metadata := make(map[string]interface{})
		if err := json.Unmarshal([]byte(metadataStr), &metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
		pipeline.Metadata = metadata
	} else {
		pipeline.Metadata = make(map[string]interface{})
	}

	// Load stages for this pipeline
	stages, err := s.LoadStages(pipeline.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to load stages: %w", err)
	}
	pipeline.Stages = stages

	return pipeline, nil
}

// ListPipelines retrieves all pipeline runs from the database.
// Returns an empty slice if no pipelines are found.
func (s *PipelineStore) ListPipelines() ([]*PipelineRun, error) {
	collection, err := s.app.FindCollectionByNameOrId("pipeline_runs")
	if err != nil {
		return nil, fmt.Errorf("pipeline_runs collection not found: %w", err)
	}

	records, err := s.app.FindRecordsByFilter(collection, "", "-created_at", 0, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to list pipelines: %w", err)
	}

	pipelines := make([]*PipelineRun, 0, len(records))
	for _, record := range records {
		pipeline := &PipelineRun{
			ID:                record.Get("id").(string),
			PipelineID:        record.Get("pipeline_id").(string),
			Status:            PipelineStatus(record.Get("status").(string)),
			CurrentStageIndex: int(record.Get("current_stage_index").(float64)),
			TotalStages:       int(record.Get("total_stages").(float64)),
			Input:             json.RawMessage(record.Get("input").(string)),
			FinalOutput:       json.RawMessage(record.Get("final_output").(string)),
			Error:             record.Get("error").(string),
			ErrorStage:        record.Get("error_stage").(string),
			RetryCount:        int(record.Get("retry_count").(float64)),
			MaxRetries:        int(record.Get("max_retries").(float64)),
			Priority:          int(record.Get("priority").(float64)),
			AssignedAgent:     record.Get("assigned_agent").(string),
			CreatedAt:         record.GetDateTime("created_at").Time(),
			UpdatedAt:         record.GetDateTime("updated_at").Time(),
		}

		// Parse optional date fields
		if startedAt := record.Get("started_at"); startedAt != nil && startedAt != "" {
			t := record.GetDateTime("started_at").Time()
			pipeline.StartedAt = &t
		}
		if completedAt := record.Get("completed_at"); completedAt != nil && completedAt != "" {
			t := record.GetDateTime("completed_at").Time()
			pipeline.CompletedAt = &t
		}
		if lastCheckpointAt := record.Get("last_checkpoint_at"); lastCheckpointAt != nil && lastCheckpointAt != "" {
			t := record.GetDateTime("last_checkpoint_at").Time()
			pipeline.LastCheckpointAt = &t
		}

		// Parse metadata
		metadataStr := record.Get("metadata").(string)
		if metadataStr != "" {
			metadata := make(map[string]interface{})
			if err := json.Unmarshal([]byte(metadataStr), &metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
			pipeline.Metadata = metadata
		} else {
			pipeline.Metadata = make(map[string]interface{})
		}

		pipelines = append(pipelines, pipeline)
	}

	return pipelines, nil
}

// ListPipelinesByStatus retrieves all pipeline runs with a specific status.
func (s *PipelineStore) ListPipelinesByStatus(status PipelineStatus) ([]*PipelineRun, error) {
	collection, err := s.app.FindCollectionByNameOrId("pipeline_runs")
	if err != nil {
		return nil, fmt.Errorf("pipeline_runs collection not found: %w", err)
	}

	filter := fmt.Sprintf("status = '%s'", status.String())
	records, err := s.app.FindRecordsByFilter(collection, filter, "-created_at", 0, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to list pipelines by status: %w", err)
	}

	pipelines := make([]*PipelineRun, 0, len(records))
	for _, record := range records {
		pipeline := &PipelineRun{
			ID:                record.Get("id").(string),
			PipelineID:        record.Get("pipeline_id").(string),
			Status:            PipelineStatus(record.Get("status").(string)),
			CurrentStageIndex: int(record.Get("current_stage_index").(float64)),
			TotalStages:       int(record.Get("total_stages").(float64)),
			Input:             json.RawMessage(record.Get("input").(string)),
			FinalOutput:       json.RawMessage(record.Get("final_output").(string)),
			Error:             record.Get("error").(string),
			ErrorStage:        record.Get("error_stage").(string),
			RetryCount:        int(record.Get("retry_count").(float64)),
			MaxRetries:        int(record.Get("max_retries").(float64)),
			Priority:          int(record.Get("priority").(float64)),
			AssignedAgent:     record.Get("assigned_agent").(string),
			CreatedAt:         record.GetDateTime("created_at").Time(),
			UpdatedAt:         record.GetDateTime("updated_at").Time(),
		}

		// Parse optional date fields
		if startedAt := record.Get("started_at"); startedAt != nil && startedAt != "" {
			t := record.GetDateTime("started_at").Time()
			pipeline.StartedAt = &t
		}
		if completedAt := record.Get("completed_at"); completedAt != nil && completedAt != "" {
			t := record.GetDateTime("completed_at").Time()
			pipeline.CompletedAt = &t
		}
		if lastCheckpointAt := record.Get("last_checkpoint_at"); lastCheckpointAt != nil && lastCheckpointAt != "" {
			t := record.GetDateTime("last_checkpoint_at").Time()
			pipeline.LastCheckpointAt = &t
		}

		// Parse metadata
		metadataStr := record.Get("metadata").(string)
		if metadataStr != "" {
			metadata := make(map[string]interface{})
			if err := json.Unmarshal([]byte(metadataStr), &metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
			pipeline.Metadata = metadata
		} else {
			pipeline.Metadata = make(map[string]interface{})
		}

		pipelines = append(pipelines, pipeline)
	}

	return pipelines, nil
}

// =============================================================================
// STAGE RUN OPERATIONS
// =============================================================================

// SaveStage persists a stage run to the database.
func (s *PipelineStore) SaveStage(stage *StageRun) error {
	if stage == nil {
		return fmt.Errorf("stage cannot be nil")
	}

	if err := stage.Validate(); err != nil {
		return fmt.Errorf("invalid stage: %w", err)
	}

	collection, err := s.app.FindCollectionByNameOrId("stage_runs")
	if err != nil {
		return fmt.Errorf("stage_runs collection not found: %w", err)
	}

	// Marshal metrics to JSON
	metricsJSON, err := json.Marshal(stage.Metrics)
	if err != nil {
		return fmt.Errorf("failed to marshal metrics: %w", err)
	}

	// Check if the stage already exists
	record, err := s.app.FindRecordById(collection, stage.ID)
	isNew := err != nil

	if isNew {
		record = core.NewRecord(collection)
		record.Set("id", stage.ID)
	}

	// Set all fields
	record.Set("pipeline_run_id", stage.PipelineRunID)
	record.Set("stage_name", stage.StageName)
	record.Set("stage_index", stage.StageIndex)
	record.Set("status", stage.Status.String())
	record.Set("backend", stage.Backend)
	record.Set("model", stage.Model)
	record.Set("input", string(stage.Input))
	record.Set("output", string(stage.Output))
	record.Set("error", stage.Error)
	record.Set("retry_count", stage.RetryCount)
	record.Set("max_retries", stage.MaxRetries)
	record.Set("started_at", stage.StartedAt)
	record.Set("completed_at", stage.CompletedAt)
	record.Set("task_id", stage.TaskID)
	record.Set("agent_id", stage.AgentID)
	record.Set("metrics", string(metricsJSON))

	// Save to database
	if err := s.app.Save(record); err != nil {
		return fmt.Errorf("failed to save stage: %w", err)
	}

	return nil
}

// LoadStages retrieves all stages for a pipeline run.
func (s *PipelineStore) LoadStages(pipelineRunID string) ([]StageRun, error) {
	if pipelineRunID == "" {
		return nil, fmt.Errorf("pipeline run ID cannot be empty")
	}

	collection, err := s.app.FindCollectionByNameOrId("stage_runs")
	if err != nil {
		return nil, fmt.Errorf("stage_runs collection not found: %w", err)
	}

	filter := fmt.Sprintf("pipeline_run_id = '%s'", pipelineRunID)
	records, err := s.app.FindRecordsByFilter(collection, filter, "stage_index", 0, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to load stages: %w", err)
	}

	stages := make([]StageRun, 0, len(records))
	for _, record := range records {
		stage := StageRun{
			ID:            record.Get("id").(string),
			PipelineRunID: record.Get("pipeline_run_id").(string),
			StageName:     record.Get("stage_name").(string),
			StageIndex:    int(record.Get("stage_index").(float64)),
			Status:        StageStatus(record.Get("status").(string)),
			Backend:       record.Get("backend").(string),
			Model:         record.Get("model").(string),
			Input:         json.RawMessage(record.Get("input").(string)),
			Output:        json.RawMessage(record.Get("output").(string)),
			Error:         record.Get("error").(string),
			RetryCount:    int(record.Get("retry_count").(float64)),
			MaxRetries:    int(record.Get("max_retries").(float64)),
			TaskID:        record.Get("task_id").(string),
			AgentID:       record.Get("agent_id").(string),
		}

		// Parse optional date fields
		if startedAt := record.Get("started_at"); startedAt != nil && startedAt != "" {
			t := record.GetDateTime("started_at").Time()
			stage.StartedAt = &t
		}
		if completedAt := record.Get("completed_at"); completedAt != nil && completedAt != "" {
			t := record.GetDateTime("completed_at").Time()
			stage.CompletedAt = &t
		}

		// Parse metrics
		metricsStr := record.Get("metrics").(string)
		if metricsStr != "" {
			metrics := StageMetrics{}
			if err := json.Unmarshal([]byte(metricsStr), &metrics); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metrics: %w", err)
			}
			stage.Metrics = metrics
		}

		stages = append(stages, stage)
	}

	return stages, nil
}

// UpdateStageStatus updates a stage's status.
func (s *PipelineStore) UpdateStageStatus(pipelineID, stageID, newStatus string) error {
	if pipelineID == "" || stageID == "" || newStatus == "" {
		return fmt.Errorf("pipeline ID, stage ID, and status are required")
	}

	collection, err := s.app.FindCollectionByNameOrId("stage_runs")
	if err != nil {
		return fmt.Errorf("stage_runs collection not found: %w", err)
	}

	record, err := s.app.FindRecordById(collection, stageID)
	if err != nil {
		return fmt.Errorf("stage not found: %w", err)
	}

	record.Set("status", newStatus)

	if err := s.app.Save(record); err != nil {
		return fmt.Errorf("failed to update stage status: %w", err)
	}

	return nil
}

// =============================================================================
// STAGE ARTIFACT OPERATIONS
// =============================================================================

// SaveArtifact persists a stage artifact to the database.
func (s *PipelineStore) SaveArtifact(artifact *StageArtifact) error {
	if artifact == nil {
		return fmt.Errorf("artifact cannot be nil")
	}

	collection, err := s.app.FindCollectionByNameOrId("stage_artifacts")
	if err != nil {
		return fmt.Errorf("stage_artifacts collection not found: %w", err)
	}

	// Marshal metadata to JSON
	metadataJSON, err := json.Marshal(artifact.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	// Check if the artifact already exists
	record, err := s.app.FindRecordById(collection, artifact.ID)
	isNew := err != nil

	if isNew {
		record = core.NewRecord(collection)
		record.Set("id", artifact.ID)
	}

	// Set all fields
	record.Set("pipeline_run_id", artifact.PipelineRunID)
	record.Set("stage_run_id", artifact.StageRunID)
	record.Set("name", artifact.Name)
	record.Set("type", string(artifact.Type))
	record.Set("data", string(artifact.Data))
	record.Set("content_type", artifact.ContentType)
	record.Set("size_bytes", artifact.SizeBytes)
	record.Set("checksum", artifact.Checksum)
	record.Set("expires_at", artifact.ExpiresAt)
	record.Set("metadata", string(metadataJSON))

	// Save to database
	if err := s.app.Save(record); err != nil {
		return fmt.Errorf("failed to save artifact: %w", err)
	}

	return nil
}

// LoadArtifact retrieves a stage artifact by ID.
func (s *PipelineStore) LoadArtifact(id string) (*StageArtifact, error) {
	if id == "" {
		return nil, fmt.Errorf("artifact ID cannot be empty")
	}

	collection, err := s.app.FindCollectionByNameOrId("stage_artifacts")
	if err != nil {
		return nil, fmt.Errorf("stage_artifacts collection not found: %w", err)
	}

	record, err := s.app.FindRecordById(collection, id)
	if err != nil {
		return nil, nil // Not found
	}

	artifact := &StageArtifact{
		ID:            record.Get("id").(string),
		PipelineRunID: record.Get("pipeline_run_id").(string),
		StageRunID:    record.Get("stage_run_id").(string),
		Name:          record.Get("name").(string),
		Type:          ArtifactType(record.Get("type").(string)),
		Data:          json.RawMessage(record.Get("data").(string)),
		ContentType:   record.Get("content_type").(string),
		SizeBytes:     int64(record.Get("size_bytes").(float64)),
		Checksum:      record.Get("checksum").(string),
		CreatedAt:     record.GetDateTime("created_at").Time(),
	}

	// Parse optional expiration
	if expiresAt := record.Get("expires_at"); expiresAt != nil && expiresAt != "" {
		t := record.GetDateTime("expires_at").Time()
		artifact.ExpiresAt = &t
	}

	// Parse metadata
	metadataStr := record.Get("metadata").(string)
	if metadataStr != "" {
		metadata := make(map[string]interface{})
		if err := json.Unmarshal([]byte(metadataStr), &metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
		artifact.Metadata = metadata
	} else {
		artifact.Metadata = make(map[string]interface{})
	}

	return artifact, nil
}

// LoadArtifactsByStage retrieves all artifacts produced by a stage.
func (s *PipelineStore) LoadArtifactsByStage(stageRunID string) ([]*StageArtifact, error) {
	if stageRunID == "" {
		return nil, fmt.Errorf("stage run ID cannot be empty")
	}

	collection, err := s.app.FindCollectionByNameOrId("stage_artifacts")
	if err != nil {
		return nil, fmt.Errorf("stage_artifacts collection not found: %w", err)
	}

	filter := fmt.Sprintf("stage_run_id = '%s'", stageRunID)
	records, err := s.app.FindRecordsByFilter(collection, filter, "created_at", 0, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to load artifacts: %w", err)
	}

	artifacts := make([]*StageArtifact, 0, len(records))
	for _, record := range records {
		artifact := &StageArtifact{
			ID:            record.Get("id").(string),
			PipelineRunID: record.Get("pipeline_run_id").(string),
			StageRunID:    record.Get("stage_run_id").(string),
			Name:          record.Get("name").(string),
			Type:          ArtifactType(record.Get("type").(string)),
			Data:          json.RawMessage(record.Get("data").(string)),
			ContentType:   record.Get("content_type").(string),
			SizeBytes:     int64(record.Get("size_bytes").(float64)),
			Checksum:      record.Get("checksum").(string),
			CreatedAt:     record.GetDateTime("created_at").Time(),
		}

		// Parse optional expiration
		if expiresAt := record.Get("expires_at"); expiresAt != nil && expiresAt != "" {
			t := record.GetDateTime("expires_at").Time()
			artifact.ExpiresAt = &t
		}

		// Parse metadata
		metadataStr := record.Get("metadata").(string)
		if metadataStr != "" {
			metadata := make(map[string]interface{})
			if err := json.Unmarshal([]byte(metadataStr), &metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
			artifact.Metadata = metadata
		} else {
			artifact.Metadata = make(map[string]interface{})
		}

		artifacts = append(artifacts, artifact)
	}

	return artifacts, nil
}

// =============================================================================
// CHECKPOINT OPERATIONS
// =============================================================================

// SaveCheckpoint persists a checkpoint to the database.
func (s *PipelineStore) SaveCheckpoint(checkpoint *Checkpoint) error {
	if checkpoint == nil {
		return fmt.Errorf("checkpoint cannot be nil")
	}

	collection, err := s.app.FindCollectionByNameOrId("pipeline_checkpoints")
	if err != nil {
		return fmt.Errorf("pipeline_checkpoints collection not found: %w", err)
	}

	// Check if the checkpoint already exists
	record, err := s.app.FindRecordById(collection, checkpoint.ID)
	isNew := err != nil

	if isNew {
		record = core.NewRecord(collection)
		record.Set("id", checkpoint.ID)
	}

	// Set all fields
	record.Set("pipeline_run_id", checkpoint.PipelineRunID)
	record.Set("version", checkpoint.Version)
	record.Set("pipeline_state", string(checkpoint.PipelineState))
	record.Set("stage_states", string(checkpoint.StageStates))
	record.Set("artifact_refs", string(checkpoint.ArtifactRefs))
	record.Set("reason", string(checkpoint.Reason))
	record.Set("size_bytes", checkpoint.SizeBytes)

	// Save to database
	if err := s.app.Save(record); err != nil {
		return fmt.Errorf("failed to save checkpoint: %w", err)
	}

	return nil
}

// LoadLatestCheckpoint retrieves the most recent checkpoint for a pipeline.
func (s *PipelineStore) LoadLatestCheckpoint(pipelineRunID string) (*Checkpoint, error) {
	if pipelineRunID == "" {
		return nil, fmt.Errorf("pipeline run ID cannot be empty")
	}

	collection, err := s.app.FindCollectionByNameOrId("pipeline_checkpoints")
	if err != nil {
		return nil, fmt.Errorf("pipeline_checkpoints collection not found: %w", err)
	}

	filter := fmt.Sprintf("pipeline_run_id = '%s'", pipelineRunID)
	records, err := s.app.FindRecordsByFilter(collection, filter, "-version", 1, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to load checkpoint: %w", err)
	}

	if len(records) == 0 {
		return nil, nil // No checkpoint exists
	}

	record := records[0]
	checkpoint := &Checkpoint{
		ID:            record.Get("id").(string),
		PipelineRunID: record.Get("pipeline_run_id").(string),
		Version:       int(record.Get("version").(float64)),
		PipelineState: json.RawMessage(record.Get("pipeline_state").(string)),
		StageStates:   json.RawMessage(record.Get("stage_states").(string)),
		ArtifactRefs:  json.RawMessage(record.Get("artifact_refs").(string)),
		CreatedAt:     record.GetDateTime("created_at").Time(),
		Reason:        CheckpointReason(record.Get("reason").(string)),
		SizeBytes:     int64(record.Get("size_bytes").(float64)),
	}

	return checkpoint, nil
}

// LoadCheckpoint retrieves a checkpoint by ID.
func (s *PipelineStore) LoadCheckpoint(id string) (*Checkpoint, error) {
	if id == "" {
		return nil, fmt.Errorf("checkpoint ID cannot be empty")
	}

	collection, err := s.app.FindCollectionByNameOrId("pipeline_checkpoints")
	if err != nil {
		return nil, fmt.Errorf("pipeline_checkpoints collection not found: %w", err)
	}

	record, err := s.app.FindRecordById(collection, id)
	if err != nil {
		return nil, nil // Not found
	}

	checkpoint := &Checkpoint{
		ID:            record.Get("id").(string),
		PipelineRunID: record.Get("pipeline_run_id").(string),
		Version:       int(record.Get("version").(float64)),
		PipelineState: json.RawMessage(record.Get("pipeline_state").(string)),
		StageStates:   json.RawMessage(record.Get("stage_states").(string)),
		ArtifactRefs:  json.RawMessage(record.Get("artifact_refs").(string)),
		CreatedAt:     record.GetDateTime("created_at").Time(),
		Reason:        CheckpointReason(record.Get("reason").(string)),
		SizeBytes:     int64(record.Get("size_bytes").(float64)),
	}

	return checkpoint, nil
}

// ListCheckpoints retrieves all checkpoints for a pipeline, ordered by version.
func (s *PipelineStore) ListCheckpoints(pipelineRunID string) ([]*Checkpoint, error) {
	if pipelineRunID == "" {
		return nil, fmt.Errorf("pipeline run ID cannot be empty")
	}

	collection, err := s.app.FindCollectionByNameOrId("pipeline_checkpoints")
	if err != nil {
		return nil, fmt.Errorf("pipeline_checkpoints collection not found: %w", err)
	}

	filter := fmt.Sprintf("pipeline_run_id = '%s'", pipelineRunID)
	records, err := s.app.FindRecordsByFilter(collection, filter, "version", 0, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to list checkpoints: %w", err)
	}

	checkpoints := make([]*Checkpoint, 0, len(records))
	for _, record := range records {
		checkpoint := &Checkpoint{
			ID:            record.Get("id").(string),
			PipelineRunID: record.Get("pipeline_run_id").(string),
			Version:       int(record.Get("version").(float64)),
			PipelineState: json.RawMessage(record.Get("pipeline_state").(string)),
			StageStates:   json.RawMessage(record.Get("stage_states").(string)),
			ArtifactRefs:  json.RawMessage(record.Get("artifact_refs").(string)),
			CreatedAt:     record.GetDateTime("created_at").Time(),
			Reason:        CheckpointReason(record.Get("reason").(string)),
			SizeBytes:     int64(record.Get("size_bytes").(float64)),
		}

		checkpoints = append(checkpoints, checkpoint)
	}

	return checkpoints, nil
}

// DeleteCheckpoint deletes a checkpoint from the database.
func (s *PipelineStore) DeleteCheckpoint(id string) error {
	if id == "" {
		return fmt.Errorf("checkpoint ID cannot be empty")
	}

	collection, err := s.app.FindCollectionByNameOrId("pipeline_checkpoints")
	if err != nil {
		return fmt.Errorf("pipeline_checkpoints collection not found: %w", err)
	}

	record, err := s.app.FindRecordById(collection, id)
	if err != nil {
		return nil // Already deleted
	}

	if err := s.app.Delete(record); err != nil {
		return fmt.Errorf("failed to delete checkpoint: %w", err)
	}

	return nil
}

// =============================================================================
// RECOVERY OPERATIONS
// =============================================================================

// FindRecoverablePipelines returns all pipelines in a recoverable state.
// These are pipelines that are running, checkpointed, stage_complete, or paused.
func (s *PipelineStore) FindRecoverablePipelines() ([]*PipelineRun, error) {
	collection, err := s.app.FindCollectionByNameOrId("pipeline_runs")
	if err != nil {
		return nil, fmt.Errorf("pipeline_runs collection not found: %w", err)
	}

	// Status values are: running, checkpoint, stage_complete, paused
	filter := "status = 'running' || status = 'checkpoint' || status = 'stage_complete' || status = 'paused'"
	records, err := s.app.FindRecordsByFilter(collection, filter, "-updated_at", 0, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to find recoverable pipelines: %w", err)
	}

	pipelines := make([]*PipelineRun, 0, len(records))
	for _, record := range records {
		pipeline := &PipelineRun{
			ID:                record.Get("id").(string),
			PipelineID:        record.Get("pipeline_id").(string),
			Status:            PipelineStatus(record.Get("status").(string)),
			CurrentStageIndex: int(record.Get("current_stage_index").(float64)),
			TotalStages:       int(record.Get("total_stages").(float64)),
			Input:             json.RawMessage(record.Get("input").(string)),
			FinalOutput:       json.RawMessage(record.Get("final_output").(string)),
			Error:             record.Get("error").(string),
			ErrorStage:        record.Get("error_stage").(string),
			RetryCount:        int(record.Get("retry_count").(float64)),
			MaxRetries:        int(record.Get("max_retries").(float64)),
			Priority:          int(record.Get("priority").(float64)),
			AssignedAgent:     record.Get("assigned_agent").(string),
			CreatedAt:         record.GetDateTime("created_at").Time(),
			UpdatedAt:         record.GetDateTime("updated_at").Time(),
		}

		// Parse optional date fields
		if startedAt := record.Get("started_at"); startedAt != nil && startedAt != "" {
			t := record.GetDateTime("started_at").Time()
			pipeline.StartedAt = &t
		}
		if completedAt := record.Get("completed_at"); completedAt != nil && completedAt != "" {
			t := record.GetDateTime("completed_at").Time()
			pipeline.CompletedAt = &t
		}
		if lastCheckpointAt := record.Get("last_checkpoint_at"); lastCheckpointAt != nil && lastCheckpointAt != "" {
			t := record.GetDateTime("last_checkpoint_at").Time()
			pipeline.LastCheckpointAt = &t
		}

		// Parse metadata
		metadataStr := record.Get("metadata").(string)
		if metadataStr != "" {
			metadata := make(map[string]interface{})
			if err := json.Unmarshal([]byte(metadataStr), &metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
			pipeline.Metadata = metadata
		} else {
			pipeline.Metadata = make(map[string]interface{})
		}

		pipelines = append(pipelines, pipeline)
	}

	return pipelines, nil
}

// DeletePipeline deletes a pipeline and all related data from the database.
func (s *PipelineStore) DeletePipeline(id string) error {
	if id == "" {
		return fmt.Errorf("pipeline ID cannot be empty")
	}

	collection, err := s.app.FindCollectionByNameOrId("pipeline_runs")
	if err != nil {
		return fmt.Errorf("pipeline_runs collection not found: %w", err)
	}

	record, err := s.app.FindRecordById(collection, id)
	if err != nil {
		return nil // Already deleted
	}

	// Delete the record (foreign key constraints should cascade)
	if err := s.app.Delete(record); err != nil {
		return fmt.Errorf("failed to delete pipeline: %w", err)
	}

	return nil
}
