# Pipeline Store Quick Start

## Setup

```go
import "flip2/internal/pipeline"

// Create store with PocketBase app instance
store := pipeline.NewPipelineStore(pbApp)
```

## Create a Pipeline

```go
// Define pipeline input
input := json.RawMessage(`{"topic":"machine learning"}`)

// Create pipeline run
pipeline := pipeline.NewPipelineRun("research", input, 2)

// Save to database
if err := store.SavePipeline(pipeline); err != nil {
    log.Fatal(err)
}
```

## Create Stages

```go
// Stage 1: Gather research (Gemini)
stage1 := pipeline.NewStageRun(pipeline.ID, "gather", "gemini", 0)
stage1.Status = pipeline.StageRunning
store.SaveStage(stage1)

// Update status when complete
stage1.Status = pipeline.StageCompleted
store.SaveStage(stage1)

// Stage 2: Analyze (Claude)
stage2 := pipeline.NewStageRun(pipeline.ID, "analyze", "claude", 1)
stage2.Status = pipeline.StageRunning
store.SaveStage(stage2)
```

## Save Artifacts

```go
artifact := &pipeline.StageArtifact{
    ID:            generateID(),
    PipelineRunID: pipeline.ID,
    StageRunID:    stage1.ID,
    Name:          "search_results",
    Type:          pipeline.ArtifactJSON,
    Data:          json.RawMessage(`{"results":[...]}`),
    ContentType:   "application/json",
    SizeBytes:     1024,
}
store.SaveArtifact(artifact)
```

## Create Checkpoints

```go
pipelineStateJSON, _ := json.Marshal(pipeline)
stagesStateJSON, _ := json.Marshal(pipeline.Stages)

checkpoint := &pipeline.Checkpoint{
    ID:            generateID(),
    PipelineRunID: pipeline.ID,
    Version:       1,
    PipelineState: pipelineStateJSON,
    StageStates:   stagesStateJSON,
    Reason:        pipeline.CheckpointStageComplete,
}
store.SaveCheckpoint(checkpoint)
```

## Load Pipelines

```go
// Load specific pipeline
p, err := store.LoadPipeline(pipelineID)
if p != nil {
    fmt.Printf("Pipeline %s: %s (progress: %.1f%%)\n",
        p.ID, p.Status, p.Progress())
}

// Load all pipelines
all, _ := store.ListPipelines()
for _, p := range all {
    fmt.Printf("- %s\n", p.ID)
}

// Find running pipelines
running, _ := store.ListPipelinesByStatus(pipeline.PipelineRunning)
fmt.Printf("Active pipelines: %d\n", len(running))

// Load stages for a pipeline
stages, _ := store.LoadStages(pipelineID)
for _, s := range stages {
    fmt.Printf("Stage: %s (%s)\n", s.StageName, s.Status)
}
```

## Recovery Workflow

```go
// Find all pipelines that crashed
crashed, _ := store.FindRecoverablePipelines()

for _, p := range crashed {
    // Load full state
    full, _ := store.LoadPipeline(p.ID)

    // Get latest checkpoint
    cp, _ := store.LoadLatestCheckpoint(p.ID)

    // Determine recovery strategy
    ctx := &pipeline.RecoveryContext{
        PipelineRun:    full,
        LastCheckpoint: cp,
    }
    strategy := pipeline.DetermineRecoveryStrategy(ctx)

    // Act based on strategy
    switch strategy {
    case pipeline.RecoveryResumeFromCheckpoint:
        // Resume from checkpoint
    case pipeline.RecoveryRestartStage:
        // Restart current stage
    case pipeline.RecoveryAbort:
        // Cannot recover
    }
}
```

## Status Types

### Pipeline Status
```go
pipeline.PipelinePending        // Created but not started
pipeline.PipelineRunning        // Actively executing
pipeline.PipelineStageComplete  // A stage finished, waiting for next
pipeline.PipelineCompleted      // All stages done
pipeline.PipelineFailed         // Error occurred
pipeline.PipelineCheckpoint     // Saved state for recovery
pipeline.PipelinePaused         // Manually paused
pipeline.PipelineCancelled      // Stopped by user
```

### Stage Status
```go
pipeline.StagePending    // Not started
pipeline.StageRunning    // Executing
pipeline.StageCompleted  // Done
pipeline.StageFailed     // Error
pipeline.StageSkipped    // Conditional skip
pipeline.StageWaiting    // Waiting for dependencies
```

## Common Operations

### Check Pipeline Status
```go
p, _ := store.LoadPipeline(id)
if p.Status.IsTerminal() {
    fmt.Println("Pipeline is finished")
}
if p.Status.IsRecoverable() {
    fmt.Println("Pipeline can be recovered")
}
```

### Get Pipeline Progress
```go
p, _ := store.LoadPipeline(id)
fmt.Printf("Progress: %.1f%%\n", p.Progress())
fmt.Printf("Duration: %v\n", p.Duration())
```

### Get Stage Details
```go
stages, _ := store.LoadStages(pipelineID)
for _, s := range stages {
    fmt.Printf("%s: %v (took %v)\n",
        s.StageName, s.Status, s.Duration())
}
```

### Get Stage Artifacts
```go
artifacts, _ := store.LoadArtifactsByStage(stageID)
for _, a := range artifacts {
    fmt.Printf("- %s: %d bytes\n", a.Name, a.SizeBytes)
}
```

### Update Stage Status
```go
err := store.UpdateStageStatus(pipelineID, stageID, "completed")
```

### List Checkpoints
```go
checkpoints, _ := store.ListCheckpoints(pipelineID)
fmt.Printf("Checkpoints: %d\n", len(checkpoints))

// Get latest
latest, _ := store.LoadLatestCheckpoint(pipelineID)
fmt.Printf("Latest: version %d\n", latest.Version)
```

### Clean Up
```go
// Delete old checkpoint
store.DeleteCheckpoint(oldCheckpointID)

// Delete entire pipeline
store.DeletePipeline(pipelineID)
```

## Error Handling

```go
// All methods return descriptive errors
pipeline := pipeline.NewPipelineRun("test", input, 1)
if err := pipeline.Validate(); err != nil {
    fmt.Printf("Invalid pipeline: %v\n", err)
}

if err := store.SavePipeline(pipeline); err != nil {
    fmt.Printf("Failed to save: %v\n", err)
}

// Check if not found (not an error)
p, err := store.LoadPipeline("nonexistent")
if p == nil && err == nil {
    fmt.Println("Pipeline not found")
}
```

## Examples

### Multi-stage Pipeline
```go
pipeline := pipeline.NewPipelineRun("research", input, 3)
store.SavePipeline(pipeline)

// Stage 1: Gather
stage1 := pipeline.NewStageRun(pipeline.ID, "gather", "gemini", 0)
stage1.Input = json.RawMessage(`{"query":"ml papers"}`)
stage1.Status = pipeline.StageRunning
store.SaveStage(stage1)
// ... execute stage ...
stage1.Status = pipeline.StageCompleted
stage1.Output = json.RawMessage(`{"papers":[...]}`)
store.SaveStage(stage1)

// Save output as artifact
artifact := &pipeline.StageArtifact{...}
store.SaveArtifact(artifact)

// Stage 2: Analyze
stage2 := pipeline.NewStageRun(pipeline.ID, "analyze", "claude", 1)
stage2.Input = stage1.Output  // Use previous stage output
stage2.Status = pipeline.StageRunning
store.SaveStage(stage2)
// ... execute stage ...
stage2.Status = pipeline.StageCompleted
stage2.Output = json.RawMessage(`{"analysis":{...}}`)
store.SaveStage(stage2)

// Stage 3: Format
stage3 := pipeline.NewStageRun(pipeline.ID, "format", "claude", 2)
stage3.Input = stage2.Output
stage3.Status = pipeline.StageRunning
store.SaveStage(stage3)
// ... execute stage ...
stage3.Status = pipeline.StageCompleted
stage3.Output = json.RawMessage(`{"report":{...}}`)
store.SaveStage(stage3)

// Mark pipeline as complete
pipeline.Status = pipeline.PipelineCompleted
pipeline.FinalOutput = stage3.Output
store.SavePipeline(pipeline)
```

### Crash Recovery
```go
// On daemon startup
func recoverPipelines(store *pipeline.PipelineStore) {
    crashed, _ := store.FindRecoverablePipelines()

    for _, p := range crashed {
        log.Printf("Recovering pipeline %s", p.ID)

        full, _ := store.LoadPipeline(p.ID)
        cp, _ := store.LoadLatestCheckpoint(p.ID)

        ctx := &pipeline.RecoveryContext{
            PipelineRun:    full,
            LastCheckpoint: cp,
            Reason:        "daemon restart",
            Timestamp:     time.Now(),
        }

        strategy := pipeline.DetermineRecoveryStrategy(ctx)

        if strategy == pipeline.RecoveryResumeFromCheckpoint {
            log.Printf("Resuming %s from checkpoint v%d",
                p.ID, cp.Version)
            // Deserialize and resume
        }
    }
}
```

## Next Steps

See `/Users/arielspivakovsky/src/flip/flip2/internal/pipeline/STORE.md` for:
- Complete API documentation
- Database schema details
- Recovery strategies
- Integration guide
- Performance considerations
