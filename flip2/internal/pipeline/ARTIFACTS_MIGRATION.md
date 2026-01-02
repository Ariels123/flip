# Pipeline Artifacts Storage Migration

## Overview

This document describes the artifact storage system for FLIP pipeline execution. Artifacts are intermediate outputs produced by pipeline stages that can be referenced by subsequent stages.

## Database Schema

The artifact storage is implemented using PocketBase collections defined in migration `13_add_pipeline_collections.go`.

### stage_artifacts Collection

The `stage_artifacts` collection stores metadata about artifacts produced by pipeline stage runs.

#### Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | Text | Yes | Unique identifier (UUID) |
| `pipeline_run_id` | Text | Yes | Reference to the pipeline run |
| `stage_run_id` | Text | Yes | Reference to the stage run that produced this artifact |
| `name` | Text | Yes | Human-readable artifact name |
| `type` | Text | Yes | Artifact type: json, text, file, url, binary |
| `data` | Text | No | JSON or text data (for small artifacts) |
| `content_type` | Text | Yes | MIME type (e.g., application/json, text/plain) |
| `size_bytes` | Number | Yes | Size of artifact in bytes |
| `checksum` | Text | No | SHA256 checksum (hex-encoded) for data integrity verification |
| `expires_at` | Date | No | Optional expiration time for garbage collection |
| `metadata` | Text | No | JSON metadata object |
| `created` | Date | Auto | Creation timestamp |
| `updated` | Date | Auto | Last update timestamp |

#### Indexes

- `idx_stage_artifacts_pipeline_run_id` - Query artifacts by pipeline run
- `idx_stage_artifacts_stage_run_id` - Query artifacts by stage run
- `idx_stage_artifacts_name` - Query artifacts by name

## File Storage Structure

Artifacts are stored on the filesystem in the following hierarchy:

```
artifacts/
├── <pipeline_run_id>/
│   ├── <stage_run_id>/
│   │   ├── <checksum>/
│   │   │   └── data
│   │   └── <another_checksum>/
│   │       └── data
│   └── <another_stage_run_id>/
│       └── ...
└── ...
```

### Example

```
artifacts/
├── run-20250101-abc123/
│   ├── gather-stage-001/
│   │   ├── a1b2c3d4e5f6.../
│   │   │   └── data
│   │   └── f6e5d4c3b2a1.../
│   │       └── data
│   └── analyze-stage-002/
│       └── b2c3d4e5f6a1.../
│           └── data
└── run-20250102-def456/
    └── ...
```

## API Usage

### Creating an ArtifactStore

```go
import "flip2/internal/pipeline"

// Create a new artifact store
store, err := pipeline.NewArtifactStore(pb, "/path/to/artifacts")
if err != nil {
    // Handle error
}
```

### Saving an Artifact

```go
// Save artifact data with metadata
checksum, err := store.SaveArtifact(
    pipelineRunID,
    stageRunID,
    "output.json",
    jsonData,
    "application/json",
    map[string]interface{}{
        "description": "Analysis results",
        "version": "1.0",
    },
)
if err != nil {
    // Handle error
}
// Checksum: a1b2c3d4e5f6...
```

### Loading an Artifact

```go
// Load by checksum
data, metadata, err := store.LoadArtifact(pipelineRunID, stageRunID, checksum)
if err != nil {
    // Handle error
}

// Load by artifact ID
data, metadata, err := store.LoadArtifactByID(artifactID)
if err != nil {
    // Handle error
}
```

### Verifying Checksums

```go
// Verify checksum of artifact data
isValid := store.VerifyChecksum(data, expectedChecksum)
if !isValid {
    log.Println("Artifact data integrity check failed!")
}

// Or use the standalone function
isValid := pipeline.VerifyChecksum(data, expectedChecksum)
```

### Listing Artifacts

```go
// List all artifacts for a stage run
artifacts, err := store.ListArtifacts(pipelineRunID, stageRunID)
if err != nil {
    // Handle error
}

for _, artifact := range artifacts {
    fmt.Printf("Artifact: %s (size: %d bytes)\n", artifact.Name, artifact.SizeBytes)
}

// List all artifacts for a pipeline run
allArtifacts, err := store.ListArtifactsByPipelineRun(pipelineRunID)
if err != nil {
    // Handle error
}
```

### Getting Storage Statistics

```go
stats, err := store.GetStats(pipelineRunID)
if err != nil {
    // Handle error
}

fmt.Printf("Total artifacts: %d\n", stats.TotalArtifacts)
fmt.Printf("Total size: %d bytes\n", stats.TotalSizeBytes)
fmt.Printf("Average size: %d bytes\n", stats.AverageSize)
```

### Deleting Artifacts

```go
err := store.DeleteArtifact(artifactID)
if err != nil {
    // Handle error
}
```

## Checksum Verification

All artifacts are protected by SHA256 checksums for data integrity verification.

### Calculation

```go
checksum := pipeline.CalculateChecksum(data)
// Returns: "a1b2c3d4e5f6...7a8b9c0d1e2f3" (hex-encoded)
```

### Verification

```go
isValid := pipeline.VerifyChecksum(data, expectedChecksum)
```

## Design Decisions

1. **File-Based Storage**: Artifacts are stored on the filesystem for performance and simplicity
2. **Metadata in Database**: References and metadata are stored in PocketBase for querying
3. **SHA256 Checksums**: Used for data integrity verification and deduplication
4. **Hierarchical Organization**: Directory structure matches pipeline → stage → checksum hierarchy
5. **Metadata Caching**: In-memory cache reduces database queries for frequently accessed artifacts

## Future Enhancements

1. **Deduplication**: Store identical artifacts only once (content-addressable storage)
2. **Compression**: Gzip compression for storage efficiency
3. **Garbage Collection**: Automatic cleanup of expired artifacts
4. **Cloud Storage**: Support for S3 or other cloud storage backends
5. **Encryption**: Encrypt sensitive artifact data at rest
6. **Streaming**: Stream large artifacts instead of loading into memory

## Testing

The artifact store includes comprehensive tests:

```bash
go test ./internal/pipeline -v -run TestArtifact*
```

Tests verify:
- Checksum calculation consistency
- Checksum verification accuracy
- Directory structure creation
- Artifact storage and retrieval
- Metadata marshaling
