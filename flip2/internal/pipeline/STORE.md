# Pipeline State Persistence

## Overview

The `PipelineStore` provides durable state management for pipeline execution using PocketBase/SQLite. All pipeline runs, stages, artifacts, and checkpoints survive system restarts and failures.

## Architecture

### Collections

Four main PocketBase collections store pipeline state:

1. **pipeline_runs** - Overall pipeline execution state
2. **stage_runs** - Individual stage execution details
3. **stage_artifacts** - Intermediate outputs from stages
4. **pipeline_checkpoints** - Saved pipeline states for recovery

### Data Model

```
pipeline_runs (1) --< (n) stage_runs
    |                     |
    |                     +--< (n) stage_artifacts
    |
    +--< (n) pipeline_checkpoints
```

## API

### PipelineStore

```go
type PipelineStore struct {
    app core.App
}

func NewPipelineStore(app core.App) *PipelineStore
```

### Pipeline Operations

#### SavePipeline(pipeline *PipelineRun) error
Persists a pipeline run to the database. Creates or updates the record.

```go
store := NewPipelineStore(app)
pipeline := NewPipelineRun("research", input, 2)
// ... modify pipeline ...
if err := store.SavePipeline(pipeline); err != nil {
    log.Fatal(err)
}
```

#### LoadPipeline(id string) (*PipelineRun, error)
Retrieves a pipeline run by ID. Returns nil if not found.

```go
pipeline, err := store.LoadPipeline("pipeline-uuid")
if pipeline != nil {
    fmt.Printf("Status: %s, Progress: %.1f%%\n",
        pipeline.Status, pipeline.Progress())
}
```

#### ListPipelines() ([]*PipelineRun, error)
Retrieves all pipeline runs ordered by creation time (newest first).

```go
pipelines, err := store.ListPipelines()
for _, p := range pipelines {
    fmt.Printf("%s: %s\n", p.ID, p.Status)
}
```

#### ListPipelinesByStatus(status PipelineStatus) ([]*PipelineRun, error)
Retrieves all pipelines with a specific status.

```go
running, err := store.ListPipelinesByStatus(PipelineRunning)
fmt.Printf("Active pipelines: %d\n", len(running))
```

### Stage Operations

#### SaveStage(stage *StageRun) error
Persists a stage run to the database.

```go
stage := NewStageRun(pipelineID, "analyze", "claude", 1)
stage.Status = StageRunning
if err := store.SaveStage(stage); err != nil {
    log.Fatal(err)
}
```

#### LoadStages(pipelineRunID string) ([]StageRun, error)
Retrieves all stages for a pipeline, ordered by stage index.

```go
stages, err := store.LoadStages(pipelineID)
for i, stage := range stages {
    fmt.Printf("Stage %d: %s (%s)\n", i, stage.StageName, stage.Status)
}
```

#### UpdateStageStatus(pipelineID, stageID, newStatus string) error
Updates a stage's status atomically.

```go
if err := store.UpdateStageStatus(pipelineID, stageID, "completed"); err != nil {
    log.Fatal(err)
}
```

### Artifact Operations

#### SaveArtifact(artifact *StageArtifact) error
Persists a stage artifact to the database.

```go
artifact := &StageArtifact{
    ID:            "artifact-uuid",
    PipelineRunID: pipelineID,
    StageRunID:    stageID,
    Name:          "search_results",
    Type:          ArtifactJSON,
    Data:          json.RawMessage(`{"results":[...]}`),
    ContentType:   "application/json",
    SizeBytes:     1024,
}
if err := store.SaveArtifact(artifact); err != nil {
    log.Fatal(err)
}
```

#### LoadArtifact(id string) (*StageArtifact, error)
Retrieves an artifact by ID.

```go
artifact, err := store.LoadArtifact("artifact-uuid")
if artifact != nil {
    fmt.Printf("Artifact: %s (%s)\n", artifact.Name, artifact.Type)
}
```

#### LoadArtifactsByStage(stageRunID string) ([]*StageArtifact, error)
Retrieves all artifacts produced by a stage.

```go
artifacts, err := store.LoadArtifactsByStage(stageID)
for _, a := range artifacts {
    fmt.Printf("- %s: %d bytes\n", a.Name, a.SizeBytes)
}
```

### Checkpoint Operations

#### SaveCheckpoint(checkpoint *Checkpoint) error
Creates a checkpoint for pipeline recovery.

```go
checkpoint := &Checkpoint{
    ID:            "checkpoint-uuid",
    PipelineRunID: pipelineID,
    Version:       1,
    PipelineState: pipelineJSON,
    StageStates:   stagesJSON,
    Reason:        CheckpointStageComplete,
}
if err := store.SaveCheckpoint(checkpoint); err != nil {
    log.Fatal(err)
}
```

#### LoadLatestCheckpoint(pipelineRunID string) (*Checkpoint, error)
Retrieves the most recent checkpoint for a pipeline.

```go
checkpoint, err := store.LoadLatestCheckpoint(pipelineID)
if checkpoint != nil {
    fmt.Printf("Latest checkpoint: version %d\n", checkpoint.Version)
}
```

#### LoadCheckpoint(id string) (*Checkpoint, error)
Retrieves a checkpoint by ID.

```go
checkpoint, err := store.LoadCheckpoint("checkpoint-uuid")
```

#### ListCheckpoints(pipelineRunID string) ([]*Checkpoint, error)
Lists all checkpoints for a pipeline, ordered by version.

```go
checkpoints, err := store.ListCheckpoints(pipelineID)
fmt.Printf("Total checkpoints: %d\n", len(checkpoints))
```

#### DeleteCheckpoint(id string) error
Deletes a checkpoint (useful for cleanup after recovery).

```go
if err := store.DeleteCheckpoint(checkpointID); err != nil {
    log.Fatal(err)
}
```

### Recovery Operations

#### FindRecoverablePipelines() ([]*PipelineRun, error)
Finds all pipelines in recoverable states (running, checkpoint, stage_complete, paused).

```go
recoverablePipelines, err := store.FindRecoverablePipelines()
for _, p := range recoverablePipelines {
    log.Printf("Recovering pipeline %s (status: %s)", p.ID, p.Status)
}
```

#### DeletePipeline(id string) error
Deletes a pipeline and all related data (cascades to stages, artifacts, checkpoints).

```go
if err := store.DeletePipeline(pipelineID); err != nil {
    log.Fatal(err)
}
```

## State Persistence

### Automatic Persistence

The store persists state automatically for:
- Pipeline lifecycle changes (pending → running → completed)
- Stage transitions (pending → running → completed)
- Checkpoint creation during recovery scenarios
- Artifact creation and updates

### Manual Checkpointing

Applications can create explicit checkpoints at any time:

```go
checkpoint := &Checkpoint{
    ID:            generateID(),
    PipelineRunID: pipelineID,
    Version:       nextVersion,
    PipelineState: serializePipeline(pipeline),
    StageStates:   serializeStages(pipeline.Stages),
    Reason:        CheckpointManual,
}
store.SaveCheckpoint(checkpoint)
```

## Recovery Workflow

1. **Detect recoverable pipelines**:
   ```go
   pipelines, _ := store.FindRecoverablePipelines()
   ```

2. **Load pipeline state**:
   ```go
   pipeline, _ := store.LoadPipeline(pipelineID)
   stages, _ := store.LoadStages(pipelineID)
   ```

3. **Get latest checkpoint (if exists)**:
   ```go
   checkpoint, _ := store.LoadLatestCheckpoint(pipelineID)
   ```

4. **Determine recovery strategy**:
   ```go
   ctx := &RecoveryContext{
       PipelineRun:    pipeline,
       LastCheckpoint: checkpoint,
   }
   strategy := DetermineRecoveryStrategy(ctx)
   ```

5. **Resume execution** based on strategy

## Database Schema

### pipeline_runs

| Column | Type | Description |
|--------|------|-------------|
| id | TEXT PRIMARY | Unique identifier |
| pipeline_id | TEXT | Pipeline definition name |
| status | TEXT | Current status |
| current_stage_index | INTEGER | Index of current stage |
| total_stages | INTEGER | Total stages in pipeline |
| input | TEXT | JSON input |
| final_output | TEXT | JSON output |
| error | TEXT | Error message if failed |
| error_stage | TEXT | Stage where error occurred |
| retry_count | INTEGER | Total retries used |
| max_retries | INTEGER | Max retries allowed |
| priority | INTEGER | Execution priority |
| assigned_agent | TEXT | Coordinator agent |
| created_at | DATETIME | Creation timestamp |
| started_at | DATETIME | Start timestamp |
| completed_at | DATETIME | Completion timestamp |
| last_checkpoint_at | DATETIME | Latest checkpoint time |
| metadata | TEXT | JSON metadata |

Indexes:
- `idx_pipeline_runs_status` - Fast status queries
- `idx_pipeline_runs_pipeline_id` - Fast pipeline type queries
- `idx_pipeline_runs_priority` - For priority-ordered execution
- `idx_pipeline_runs_assigned_agent` - For coordinator tracking

### stage_runs

| Column | Type | Description |
|--------|------|-------------|
| id | TEXT PRIMARY | Unique identifier |
| pipeline_run_id | TEXT FOREIGN | Parent pipeline |
| stage_name | TEXT | Stage name |
| stage_index | INTEGER | Execution order |
| status | TEXT | Current status |
| backend | TEXT | LLM backend (claude, gemini) |
| model | TEXT | Specific model |
| input | TEXT | JSON input |
| output | TEXT | JSON output |
| error | TEXT | Error message |
| retry_count | INTEGER | Retry count |
| max_retries | INTEGER | Max retries |
| started_at | DATETIME | Start timestamp |
| completed_at | DATETIME | Completion timestamp |
| task_id | TEXT | Associated task |
| agent_id | TEXT | Assigned agent |
| metrics | TEXT | JSON metrics |

Indexes:
- `idx_stage_runs_pipeline_run_id` - Parent lookup
- `idx_stage_runs_status` - Status queries
- `idx_stage_runs_task_id` - Task tracking

### stage_artifacts

| Column | Type | Description |
|--------|------|-------------|
| id | TEXT PRIMARY | Unique identifier |
| pipeline_run_id | TEXT FOREIGN | Parent pipeline |
| stage_run_id | TEXT FOREIGN | Producing stage |
| name | TEXT | Artifact name |
| type | TEXT | Type (json, text, file, url, binary) |
| data | TEXT | Artifact data or reference |
| content_type | TEXT | MIME type |
| size_bytes | INTEGER | Data size |
| checksum | TEXT | SHA256 hash |
| created_at | DATETIME | Creation timestamp |
| expires_at | DATETIME | Garbage collection time |
| metadata | TEXT | JSON metadata |

Indexes:
- `idx_stage_artifacts_pipeline_run_id` - Pipeline lookup
- `idx_stage_artifacts_stage_run_id` - Stage lookup
- `idx_stage_artifacts_name` - Artifact name lookup

### pipeline_checkpoints

| Column | Type | Description |
|--------|------|-------------|
| id | TEXT PRIMARY | Unique identifier |
| pipeline_run_id | TEXT FOREIGN | Parent pipeline |
| version | INTEGER | Version number |
| pipeline_state | TEXT | Serialized pipeline |
| stage_states | TEXT | Serialized stages |
| artifact_refs | TEXT | Artifact references |
| created_at | DATETIME | Creation timestamp |
| reason | TEXT | Why checkpoint was created |
| size_bytes | INTEGER | Checkpoint size |

Indexes:
- `idx_pipeline_checkpoints_pipeline_run_id` - Pipeline lookup
- `idx_pipeline_checkpoints_version` - Latest checkpoint query

## Performance Considerations

### Indexes
All collections use strategic indexes to optimize:
- Status-based queries (finding running pipelines)
- Parent-child lookups (loading stages for a pipeline)
- Sorting operations (latest checkpoint, priority ordering)

### Connection Pooling
PocketBase manages database connections automatically. The `core.App` interface handles connection pooling internally.

### Query Optimization
- Load stages only when needed
- Use `ListPipelinesByStatus()` for filtered queries
- Checkpoint version numbers for efficient latest lookup

## Migration

The pipeline collections are created via PocketBase migration:

File: `/Users/arielspivakovsky/src/flip/flip2/pb_migrations/13_add_pipeline_collections.go`

The migration:
1. Creates all four collections
2. Defines field types and constraints
3. Creates indexes for performance
4. Sets public access rules (configurable)
5. Includes rollback logic

## Testing

See `store_test.go` for test coverage of:
- Pipeline creation and validation
- Stage lifecycle management
- Progress and duration calculations
- Status transition validation
- Recovery strategy determination

## Integration

To integrate the store into your pipeline executor:

```go
// Initialize store
store := NewPipelineStore(app)

// Save pipeline
pipeline := NewPipelineRun("research", input, numStages)
store.SavePipeline(pipeline)

// Load and resume
recovered, _ := store.LoadPipeline(pipelineID)
if recovered.Status.IsRecoverable() {
    // Resume execution
}

// Save stages as they complete
stage.Status = StageCompleted
store.SaveStage(stage)

// Create checkpoints for recovery
checkpoint := &Checkpoint{
    ID:            generateID(),
    PipelineRunID: pipeline.ID,
    Version:       getNextVersion(),
    PipelineState: serialize(pipeline),
    StageStates:   serialize(pipeline.Stages),
    Reason:        CheckpointStageComplete,
}
store.SaveCheckpoint(checkpoint)
```

## Persistence Guarantees

- **ACID Compliance**: PocketBase/SQLite provides ACID guarantees
- **Crash Recovery**: All data survives process crashes
- **Cascading Deletes**: Deleting a pipeline removes all related data
- **Foreign Key Constraints**: Database enforces referential integrity
- **Atomic Updates**: Individual record updates are atomic

## Future Enhancements

- Archival system for old pipelines
- Snapshot compression for large checkpoints
- Distributed cache for frequently accessed pipelines
- Real-time change notifications via WebSocket
- Pipeline analytics and metrics aggregation
