# Pipeline Recovery Integration Guide

This document describes how to integrate the automatic pipeline recovery feature into the FLIP2 daemon startup.

## Overview

The recovery system automatically resumes pipelines that were interrupted due to system crashes or graceful shutdowns. It provides three recovery strategies:

1. **Resume from Checkpoint**: Restores from the most recent checkpoint
2. **Resume from Last Stage**: Continues from the last completed stage
3. **Restart Pipeline**: Starts from the beginning (for pipelines with no progress)

## Integration Points

### 1. Daemon Startup (daemon/daemon.go)

In the `Daemon.Start()` method, after initializing PocketBase but before starting the scheduler:

```go
func (d *Daemon) Start() error {
    // ... existing initialization code ...

    // Initialize PocketBase
    d.pb = pocketbase.NewWithConfig(pocketbase.Config{
        DefaultDataDir: d.config.Flip2.PocketBase.DataDir,
    })

    // ... other initialization ...

    // RECOVERY: Initialize recovery system and recover pipelines
    d.logger.Info("Initializing pipeline recovery system")

    store := pipeline.NewPipelineStore(d.pb)
    recovery := pipeline.NewRecovery(store, pipeline.NewDefaultLogger(d.logger))

    // Recover all interrupted pipelines
    stats, err := recovery.RecoverAllPipelines()
    if err != nil {
        d.logger.Error("Pipeline recovery failed", "error", err)
        // Continue startup even if recovery fails - log but don't block
    } else {
        d.logger.Info("Pipeline recovery complete",
            "total", stats.TotalPipelines,
            "resumed_checkpoint", stats.ResumedFromCheckpoint,
            "resumed_last_stage", stats.ResumedFromLastStage,
            "restarted", stats.Restarted,
            "aborted", stats.Aborted)
    }

    // Continue with scheduler and other initialization...
    d.scheduler = scheduler.New(d.pb, d.logger)

    // ... rest of startup ...
}
```

### 2. Graceful Shutdown

Before daemon shutdown, create checkpoints for all running pipelines:

```go
func (d *Daemon) Stop() error {
    d.logger.Info("Daemon shutting down...")

    // RECOVERY: Create checkpoints before shutdown
    store := pipeline.NewPipelineStore(d.pb)
    recovery := pipeline.NewRecovery(store, pipeline.NewDefaultLogger(d.logger))

    if err := recovery.CreateCheckpointOnShutdown(d.pb); err != nil {
        d.logger.Error("Failed to create shutdown checkpoints", "error", err)
        // Log but continue with shutdown
    }

    // Proceed with normal shutdown sequence
    // ... existing shutdown code ...

    return nil
}
```

### 3. Signal Handling

Add signal handlers to trigger graceful shutdown with checkpoints:

```go
import "os/signal"

// In daemon initialization
sigChan := make(chan os.Signal, 1)
signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

go func() {
    sig := <-sigChan
    d.logger.Info("Received signal, initiating graceful shutdown", "signal", sig)
    d.Stop()
    os.Exit(0)
}()
```

## Recovery Flow Diagram

```
System Startup
    |
    v
Initialize PocketBase
    |
    v
Find Recoverable Pipelines
(status = running|checkpoint|stage_complete|paused)
    |
    +---> For Each Pipeline:
    |     |
    |     v
    |     Load Pipeline + Stages
    |     |
    |     v
    |     Find Last Completed Stage
    |     |
    |     v
    |     Load Latest Checkpoint (if exists)
    |     |
    |     v
    |     Determine Recovery Strategy
    |     |
    |     +---> Has Checkpoint?
    |     |     YES -> Resume from Checkpoint
    |     |     NO  -> Resume from Last Stage (if any)
    |     |           else Restart Pipeline
    |     |
    |     v
    |     Update Pipeline Status to RUNNING
    |     Set CurrentStageIndex to Next Stage
    |     Persist Updated Pipeline
    |
    v
Recovery Complete - Pipelines Ready to Resume
    |
    v
Start Scheduler + Executor
    |
    v
Scheduler Picks Up Pipelines at Next Stage
```

## Recovery Decision Tree

```
Pipeline Found in Recoverable State
    |
    v
Load Pipeline + Stages
    |
    v
Status = PipelineCheckpoint?
    |
    +---> YES: Has Latest Checkpoint?
    |           |
    |           +---> YES: Resume from Checkpoint ✓
    |           |
    |           +---> NO: Resume from Last Stage ✓
    |
    +---> NO: Check Last Completed Stage
            |
            +---> Found (LastStageIndex >= 0)?
            |     |
            |     +---> YES: Resume from Last Stage ✓
            |     |
            |     +---> NO: Restart Pipeline ✓
            |
            +---> ALL ELSE: Abort Recovery
```

## Database Schema Requirements

The recovery system relies on these PocketBase collections:

### pipeline_runs
- `id` (TEXT PRIMARY KEY)
- `pipeline_id` (TEXT)
- `status` (TEXT) - one of: pending, running, checkpoint, stage_complete, completed, failed, paused, cancelled
- `current_stage_index` (INTEGER)
- `total_stages` (INTEGER)
- `started_at` (DATETIME)
- `completed_at` (DATETIME)
- `last_checkpoint_at` (DATETIME)

### stage_runs
- `id` (TEXT PRIMARY KEY)
- `pipeline_run_id` (TEXT FOREIGN KEY)
- `stage_name` (TEXT)
- `stage_index` (INTEGER)
- `status` (TEXT) - one of: pending, running, completed, failed, skipped, waiting
- `completed_at` (DATETIME)

### pipeline_checkpoints
- `id` (TEXT PRIMARY KEY)
- `pipeline_run_id` (TEXT FOREIGN KEY)
- `version` (INTEGER)
- `pipeline_state` (TEXT) - JSON serialized PipelineRun
- `stage_states` (TEXT) - JSON serialized []StageRun
- `artifact_refs` (TEXT) - JSON references to artifacts
- `reason` (TEXT) - checkpoint reason
- `created_at` (DATETIME)

## API Reference

### NewRecovery(store, logger)
Creates a new recovery handler.

**Parameters:**
- `store`: PipelineStore instance
- `logger`: RecoveryLogger implementation (or nil for no-op)

**Returns:** Recovery instance

### RecoverPipelines()
Finds and prepares all recoverable pipelines.

**Returns:**
- `[]*RecoverableCheckpoint` - List of pipelines ready to recover
- `int` - Total number of pipelines found
- `int` - Number of pipelines with checkpoints
- `error` - Any error that occurred

### RecoverAllPipelines()
Performs full recovery of all interrupted pipelines.

**Returns:**
- `*RecoveryStats` - Statistics about the recovery
- `error` - Any error that occurred

**Recovery Statistics:**
```go
type RecoveryStats struct {
    TotalPipelines         int
    ResumedFromCheckpoint  int
    ResumedFromLastStage   int
    Restarted             int
    Aborted               int
    Duration              time.Duration
}
```

### ResumeFromCheckpoint(rc)
Resumes a pipeline from its saved checkpoint.

**Parameters:**
- `rc`: RecoverableCheckpoint

**Returns:**
- `int` - Next stage index to execute
- `error` - Any error that occurred

### ResumeFromLastStage(rc)
Resumes a pipeline from the last completed stage.

**Parameters:**
- `rc`: RecoverableCheckpoint

**Returns:**
- `int` - Next stage index to execute
- `error` - Any error that occurred

### CreateCheckpointOnShutdown(app)
Creates checkpoints for all running pipelines before shutdown.

**Parameters:**
- `app`: PocketBase core.App instance

**Returns:**
- `error` - Any error that occurred

## Testing

The recovery system includes comprehensive tests:

```bash
# Run all recovery tests
go test -v ./internal/pipeline -run Recovery

# Run specific test
go test -v ./internal/pipeline -run TestCrashRecoverySimulation

# Run with coverage
go test -cover ./internal/pipeline
```

### Test Scenarios

1. **TestCrashRecoverySimulation**: Simulates a complete crash and recovery cycle
2. **TestCheckpointBasedRecovery**: Tests recovery from checkpoints
3. **TestRecoveryStatsCollection**: Verifies recovery statistics
4. **TestNoRecoverablePipelines**: Handles empty database gracefully
5. **TestRecoveryErrorHandling**: Tests error handling and edge cases

## Configuration

The recovery system respects these settings:

- **PocketBase Data Directory**: Location of pipeline state database
- **Logger Level**: Controls verbosity of recovery logs
- **Checkpoint Retention**: Handled by store cleanup policies

## Performance Considerations

- Recovery happens sequentially at startup (before scheduler starts)
- Each pipeline load includes loading all its stages
- Checkpoint restoration is I/O bound
- Expected overhead: ~100-200ms per pipeline

## Monitoring and Logging

Recovery operations are logged at different levels:

- **INFO**: Pipeline found, recovery actions taken, statistics
- **WARN**: Recoverable pipelines with issues (not found, corrupted state)
- **ERROR**: Recovery failures (I/O errors, validation failures)
- **DEBUG**: Detailed recovery processing information

Example logs:
```
Starting pipeline recovery on system startup
Found pipelines to recover count=3
Pipeline ready for recovery pipeline_id=p1 status=running last_completed_stage_index=1 has_checkpoint=true
Pipeline recovery preparation complete total_recoverable=3 with_checkpoints=1
Processing recovery for pipelines total=3
Resuming from checkpoint pipeline_id=p1 checkpoint_version=1
Pipeline recovery complete total=3 resumed_from_checkpoint=1 resumed_from_last_stage=2 aborted=0 duration_ms=45
```

## Troubleshooting

### Pipeline Not Recovered
- Verify pipeline status is one of: running, checkpoint, stage_complete, paused
- Check database connectivity
- Verify pipeline_runs and stage_runs tables exist
- Review error logs for specific failure reason

### Incorrect Next Stage Calculated
- Verify stage_runs records have correct stage_index and status
- Check that StageCompleted status is set on completed stages
- Ensure TotalStages in pipeline_runs is correct

### Checkpoint Not Found
- Create checkpoint using `CreateCheckpointOnShutdown` during shutdown
- Manually create checkpoint using PipelineStore.SaveCheckpoint
- Recovery will use last_completed_stage as fallback if checkpoint missing

## Future Enhancements

- Parallel recovery for multiple pipelines
- Checkpoint garbage collection (delete old versions)
- Recovery metrics and monitoring
- Partial recovery (skip failed stages)
- Custom recovery callbacks per pipeline
