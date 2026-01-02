# Pipeline Package - State Persistence and Execution

## Package Overview

The pipeline package provides complete pipeline definition parsing and execution state management with durability via SQLite.

### Key Components

1. **Pipeline Definition** (`parser.go`)
   - YAML-based pipeline definitions
   - Stage validation and DAG checking
   - Execution order calculation

2. **Execution State** (`state_schema.go`)
   - Pipeline run lifecycle management
   - Stage execution tracking
   - Artifact and checkpoint support
   - Recovery state machines

3. **State Persistence** (`store.go`) - **NEW**
   - PocketBase/SQLite persistence layer
   - Full CRUD operations for all state
   - Recovery and crash resilience
   - Checkpoint creation and restoration

## Files

### Source Code
- `parser.go` (468 lines) - Pipeline definition parsing
- `state_schema.go` (839 lines) - State data structures
- `store.go` (857 lines) - Database persistence layer
- `store_test.go` (281 lines) - Persistence tests

### Database
- `migrations.sql` (96 lines) - SQL schema reference
- `pb_migrations/13_add_pipeline_collections.go` (155 lines) - PocketBase migration

### Documentation
- `STORE.md` (469 lines) - Complete persistence API documentation
- `QUICKSTART.md` (326 lines) - Quick start guide with examples
- `CLAUDE.md` - Project-specific instructions

## Quick Links

### For Using the Store
- **Quick Start**: See `QUICKSTART.md` for common operations
- **Full API**: See `STORE.md` for complete documentation
- **Examples**: See `QUICKSTART.md` for real-world examples

### For Integration
- **State Persistence**: All pipelines automatically persist to SQLite
- **Recovery**: Automatic recovery on daemon restart
- **Checkpoints**: Manual or automatic checkpoint creation

## Key Features

### Persistence
- All pipeline state stored in SQLite via PocketBase
- Atomic operations ensure consistency
- ACID guarantees from SQLite

### Recovery
- Automatic detection of crashed pipelines
- Checkpoint-based recovery
- Status-based recovery strategies

### Performance
- 12 strategic indexes for efficient queries
- Lazy loading of stages and artifacts
- Optimized checkpoint lookup

## Data Model

```
Pipeline (one execution)
  ├── Stages (sequential execution)
  │   ├── Input (from previous stage or pipeline input)
  │   ├── Output (artifact for next stage)
  │   └── Status (pending/running/completed/failed)
  ├── Artifacts (intermediate outputs)
  │   ├── Name (unique within pipeline)
  │   ├── Type (json/text/file/url/binary)
  │   └── Data (actual content)
  └── Checkpoints (saved states)
      ├── Version (incremental)
      ├── PipelineState (serialized)
      ├── StageStates (all stages)
      └── Reason (why checkpoint was created)
```

## Status Constants

### Pipeline Status
- `PipelinePending` - Created, not started
- `PipelineRunning` - Executing stages
- `PipelineStageComplete` - Stage done, waiting for next
- `PipelineCompleted` - All stages done
- `PipelineFailed` - Error occurred
- `PipelineCheckpoint` - Saved for recovery
- `PipelinePaused` - Manually paused
- `PipelineCancelled` - Stopped by user

### Stage Status
- `StagePending` - Not started
- `StageRunning` - Executing
- `StageCompleted` - Done
- `StageFailed` - Error
- `StageSkipped` - Conditional skip
- `StageWaiting` - Waiting for dependencies

### Checkpoint Reason
- `CheckpointStageComplete` - After each stage
- `CheckpointPeriodic` - Timer-based
- `CheckpointManual` - User requested
- `CheckpointPreShutdown` - Before shutdown
- `CheckpointError` - During error recovery

## Core API

### PipelineStore Methods

**Pipeline Operations**
- `SavePipeline(pipeline *PipelineRun) error`
- `LoadPipeline(id string) (*PipelineRun, error)`
- `ListPipelines() ([]*PipelineRun, error)`
- `ListPipelinesByStatus(status PipelineStatus) ([]*PipelineRun, error)`

**Stage Operations**
- `SaveStage(stage *StageRun) error`
- `LoadStages(pipelineRunID string) ([]StageRun, error)`
- `UpdateStageStatus(pipelineID, stageID, status string) error`

**Artifact Operations**
- `SaveArtifact(artifact *StageArtifact) error`
- `LoadArtifact(id string) (*StageArtifact, error)`
- `LoadArtifactsByStage(stageRunID string) ([]*StageArtifact, error)`

**Checkpoint Operations**
- `SaveCheckpoint(checkpoint *Checkpoint) error`
- `LoadLatestCheckpoint(pipelineRunID string) (*Checkpoint, error)`
- `LoadCheckpoint(id string) (*Checkpoint, error)`
- `ListCheckpoints(pipelineRunID string) ([]*Checkpoint, error)`
- `DeleteCheckpoint(id string) error`

**Recovery Operations**
- `FindRecoverablePipelines() ([]*PipelineRun, error)`
- `DeletePipeline(id string) error`

## Usage Example

```go
import "flip2/internal/pipeline"

// Initialize store
store := pipeline.NewPipelineStore(pbApp)

// Create pipeline
p := pipeline.NewPipelineRun("research", input, 2)
store.SavePipeline(p)

// Create and save stages
s := pipeline.NewStageRun(p.ID, "gather", "gemini", 0)
s.Status = pipeline.StageRunning
store.SaveStage(s)

// Create checkpoint for recovery
cp := &pipeline.Checkpoint{
    ID:            generateID(),
    PipelineRunID: p.ID,
    Version:       1,
    PipelineState: serializeState(p),
    StageStates:   serializeStages(p.Stages),
    Reason:        pipeline.CheckpointStageComplete,
}
store.SaveCheckpoint(cp)

// Load on restart
recovered, _ := store.LoadPipeline(p.ID)
checkpoint, _ := store.LoadLatestCheckpoint(p.ID)
// Resume execution...
```

## Testing

Run tests:
```bash
go test ./internal/pipeline
```

Test coverage:
- Pipeline creation and validation
- Stage lifecycle
- Status transitions
- Recovery strategies
- Progress and duration calculations

## Dependencies

- PocketBase (`github.com/pocketbase/pocketbase/core`)
- Standard library (encoding/json, time, fmt)

## Integration

The store integrates with:
- **Daemon**: Loads recoverable pipelines on startup
- **Executor**: Saves state after each stage
- **Scheduler**: Queries pipelines by status
- **Recovery**: Finds crashed pipelines

## Performance Characteristics

- **Write**: O(1) - Single record insert/update
- **Read**: O(log n) - Index-based lookups
- **List**: O(n) - Full scan with ordering
- **Recovery**: O(m) - m = number of crashed pipelines

## Future Enhancements

- Archival system for old pipelines
- Metrics aggregation and analytics
- Real-time change notifications
- Distributed cache layer
- Pipeline graph visualization
- Advanced recovery strategies

## Files Summary

| File | Lines | Purpose |
|------|-------|---------|
| parser.go | 468 | YAML parsing and validation |
| state_schema.go | 839 | Data structures and state machine |
| store.go | 857 | SQLite persistence layer |
| store_test.go | 281 | Persistence unit tests |
| migrations.sql | 96 | SQL schema reference |
| 13_add_pipeline_collections.go | 155 | PocketBase migration |
| STORE.md | 469 | Complete API documentation |
| QUICKSTART.md | 326 | Examples and quick start |
| PSM-003-SUMMARY.md | 292 | Task completion summary |
| **Total** | **2476** | **Complete package** |

## Notes

- All timestamps are UTC via PocketBase DateTime
- JSON fields are stored as TEXT with proper marshaling
- Foreign key cascades enabled for data integrity
- All operations include comprehensive error handling
- The store is thread-safe (PocketBase handles locking)

## See Also

- `/Users/arielspivakovsky/src/flip/CLAUDE.md` - Project instructions
- `/Users/arielspivakovsky/src/flip/flip2/PSM-003-SUMMARY.md` - Task details
