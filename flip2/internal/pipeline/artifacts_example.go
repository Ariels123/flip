package pipeline

// This file provides example usage patterns for the ArtifactStore.
// These are reference implementations - not executable code.

/*
Example 1: Basic Artifact Storage and Retrieval

	import (
		"log"
		"github.com/pocketbase/pocketbase"
		"flip2/internal/pipeline"
	)

	func exampleBasicArtifacts(pb *pocketbase.PocketBase) {
		// Create a new artifact store
		store, err := pipeline.NewArtifactStore(pb, "./data/artifacts")
		if err != nil {
			log.Fatal(err)
		}

		// Stage execution produces output
		pipelineRunID := "run-20250101-abc123"
		stageRunID := "stage-gather-001"
		outputData := []byte(`{"key": "value", "results": [1, 2, 3]}`)

		// Save the artifact with metadata
		checksum, err := store.SaveArtifact(
			pipelineRunID,
			stageRunID,
			"analysis_output.json",
			outputData,
			"application/json",
			map[string]interface{}{
				"description": "Analysis results from gather stage",
				"records": 1000,
				"version": "1.0",
			},
		)
		if err != nil {
			log.Fatal(err)
		}

		// Later, retrieve the artifact
		data, metadata, err := store.LoadArtifact(pipelineRunID, stageRunID, checksum)
		if err != nil {
			log.Fatal(err)
		}

		log.Printf("Loaded artifact: %s (size: %d bytes)", metadata.Name, metadata.SizeBytes)
	}

Example 2: Checksum Verification

	// Verify data integrity
	data := []byte("Important data")
	checksum := pipeline.CalculateChecksum(data)

	// Later, verify the data hasn't been corrupted
	if pipeline.VerifyChecksum(data, checksum) {
		log.Println("Data integrity verified!")
	}

Example 3: Listing Artifacts

	// Get all artifacts from a stage
	artifacts, err := store.ListArtifacts(pipelineRunID, stageRunID)
	if err != nil {
		log.Fatal(err)
	}

	for _, artifact := range artifacts {
		log.Printf("Artifact: %s, Type: %s, Size: %d bytes",
			artifact.Name, artifact.Type, artifact.SizeBytes)
	}

	// Get all artifacts from a pipeline run
	allArtifacts, err := store.ListArtifactsByPipelineRun(pipelineRunID)
	if err != nil {
		log.Fatal(err)
	}

Example 4: Storage Statistics

	stats, err := store.GetStats(pipelineRunID)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Pipeline storage stats:")
	log.Printf("  Total artifacts: %d", stats.TotalArtifacts)
	log.Printf("  Total size: %d bytes (%.2f MB)", stats.TotalSizeBytes, float64(stats.TotalSizeBytes)/1024/1024)
	log.Printf("  Average artifact size: %d bytes", stats.AverageSize)

Example 5: Loading by Artifact ID

	// If you have the artifact ID, you can load directly
	data, metadata, err := store.LoadArtifactByID(artifactID)
	if err != nil {
		log.Fatal(err)
	}

	// Check metadata cache hit
	log.Printf("Loaded from cache: %v", metadata != nil)

Example 6: Artifact Deletion

	// Clean up artifacts after pipeline completes
	artifacts, err := store.ListArtifactsByPipelineRun(pipelineRunID)
	if err != nil {
		log.Fatal(err)
	}

	for _, artifact := range artifacts {
		if artifact.Type == "temporary" {
			err := store.DeleteArtifact(artifact.ID)
			if err != nil {
				log.Printf("Warning: failed to delete artifact %s: %v", artifact.ID, err)
			}
		}
	}

Example 7: Multi-Stage Pipeline with Artifact Chain

	// Stage 1: Gather data
	gatherOutput := []byte(`{...stage1 results...}`)
	checksum1, err := store.SaveArtifact(
		pipelineRunID, "stage-gather", "gathered_data.json",
		gatherOutput, "application/json", nil,
	)

	// Stage 2: Process with input from Stage 1
	stage1Data, _, err := store.LoadArtifact(pipelineRunID, "stage-gather", checksum1)

	processedOutput := processData(stage1Data) // Application logic
	checksum2, err := store.SaveArtifact(
		pipelineRunID, "stage-process", "processed_data.json",
		processedOutput, "application/json", nil,
	)

	// Stage 3: Analyze with input from Stage 2
	stage2Data, _, err := store.LoadArtifact(pipelineRunID, "stage-process", checksum2)

	analysisOutput := analyzeData(stage2Data) // Application logic
	checksum3, err := store.SaveArtifact(
		pipelineRunID, "stage-analyze", "analysis_results.json",
		analysisOutput, "application/json", nil,
	)

Example 8: Error Handling and Reliability

	// Handle missing artifacts gracefully
	data, metadata, err := store.LoadArtifact(pipelineRunID, stageRunID, checksum)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			log.Printf("Artifact not found, retrying stage execution...")
			// Re-execute the stage
		} else {
			log.Fatal(err)
		}
	}

	// Verify integrity before processing
	if !store.VerifyChecksum(data, metadata.Checksum) {
		log.Printf("WARNING: Artifact checksum verification failed!")
		log.Printf("Expected: %s, got: %s", metadata.Checksum, pipeline.CalculateChecksum(data))
	}

Storage Directory Structure (created automatically):

	artifacts/
	├── run-20250101-abc123/
	│   ├── stage-gather-001/
	│   │   ├── a1b2c3d4.../
	│   │   │   └── data           (256KB)
	│   │   └── f6e5d4c3.../
	│   │       └── data           (512KB)
	│   ├── stage-process-002/
	│   │   └── b2c3d4e5.../
	│   │       └── data           (1.2MB)
	│   └── stage-analyze-003/
	│       └── c3d4e5f6.../
	│           └── data           (2.5MB)
	└── run-20250102-def456/
	    └── ...

Database Schema (PocketBase):

	stage_artifacts Table:
	├── id (TEXT, PRIMARY KEY)
	├── pipeline_run_id (TEXT, INDEX)
	├── stage_run_id (TEXT, INDEX)
	├── name (TEXT)
	├── type (TEXT: json|text|file|url|binary)
	├── content_type (TEXT: MIME type)
	├── size_bytes (NUMBER)
	├── checksum (TEXT: SHA256 hex)
	├── metadata (TEXT: JSON)
	├── created (DATETIME: auto)
	└── updated (DATETIME: auto)

Key Features:

1. Hierarchical Storage:
   - Automatically organizes files by pipeline → stage → checksum
   - Enables content-addressable storage for deduplication

2. Checksum Verification:
   - SHA256 checksums for data integrity
   - Automatic verification on load

3. Metadata Tracking:
   - Database records all artifact metadata
   - In-memory caching for performance
   - Supports custom JSON metadata

4. Query Capabilities:
   - Find by pipeline run and stage run
   - Find by pipeline run (all stages)
   - Find by artifact ID
   - List with filtering and sorting

5. Storage Management:
   - Statistics on total size and artifacts
   - Delete artifacts and clean up filesystem
   - Automatic directory creation

Testing:

	go test ./internal/pipeline -v -run TestChecksum*
	go test ./internal/pipeline -v -run TestArtifactStore*
	go test ./internal/pipeline -v

Dependencies:

	- github.com/google/uuid (for artifact IDs)
	- github.com/pocketbase/pocketbase/core (for App interface)
	- Standard library: crypto/sha256, encoding/hex, encoding/json, os, path/filepath, time

Migration Status:

	The stage_artifacts collection is created by migration 13_add_pipeline_collections.go
	No additional migrations required - just use NewArtifactStore()
*/
