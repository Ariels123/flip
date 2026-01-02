// Package pipeline provides artifact storage and retrieval functionality for pipeline execution.
package pipeline

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/pocketbase/pocketbase/core"
)

// ArtifactStore manages storage and retrieval of pipeline stage outputs.
// It stores artifacts on the filesystem with metadata tracked in the database.
type ArtifactStore struct {
	app           core.App // PocketBase app instance
	baseDir       string   // Base directory for artifact storage (e.g., "artifacts")
	metadataCache map[string]*ArtifactMetadata
}

// ArtifactMetadata contains metadata about a stored artifact.
type ArtifactMetadata struct {
	ID            string                 `json:"id"`
	PipelineRunID string                 `json:"pipeline_run_id"`
	StageRunID    string                 `json:"stage_run_id"`
	Name          string                 `json:"name"`
	Type          string                 `json:"type"` // json, text, file, url, binary
	ContentType   string                 `json:"content_type"`
	SizeBytes     int64                  `json:"size_bytes"`
	Checksum      string                 `json:"checksum"` // SHA256 hex
	StoragePath   string                 `json:"storage_path"`
	CreatedAt     time.Time              `json:"created_at"`
	ExpiresAt     *time.Time             `json:"expires_at,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// NewArtifactStore creates a new artifact store instance.
// baseDir is the filesystem path where artifacts will be stored.
func NewArtifactStore(app core.App, baseDir string) (*ArtifactStore, error) {
	// Create base directory if it doesn't exist
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create artifact base directory: %w", err)
	}

	return &ArtifactStore{
		app:           app,
		baseDir:       baseDir,
		metadataCache: make(map[string]*ArtifactMetadata),
	}, nil
}

// SaveArtifact stores artifact data and saves metadata to the database.
// Returns the SHA256 checksum of the stored artifact.
func (as *ArtifactStore) SaveArtifact(
	pipelineRunID string,
	stageRunID string,
	artifactName string,
	data []byte,
	contentType string,
	metadata map[string]interface{},
) (string, error) {
	// Calculate checksum
	checksum := CalculateChecksum(data)

	// Create directory structure: artifacts/<pipelineRunID>/<stageRunID>/<checksum>
	artifactDir := filepath.Join(as.baseDir, pipelineRunID, stageRunID, checksum)
	if err := os.MkdirAll(artifactDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create artifact directory: %w", err)
	}

	// Write artifact data to file
	artifactPath := filepath.Join(artifactDir, "data")
	if err := os.WriteFile(artifactPath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write artifact file: %w", err)
	}

	// Save metadata to database
	artifactID := uuid.New().String()
	artifactType := "binary"
	if contentType == "application/json" {
		artifactType = "json"
	} else if contentType == "text/plain" {
		artifactType = "text"
	}

	collection, err := as.app.FindCollectionByNameOrId("stage_artifacts")
	if err != nil {
		_ = os.RemoveAll(artifactDir)
		return "", fmt.Errorf("stage_artifacts collection not found: %w", err)
	}

	artifactRecord := core.NewRecord(collection)
	artifactRecord.Set("id", artifactID)
	artifactRecord.Set("pipeline_run_id", pipelineRunID)
	artifactRecord.Set("stage_run_id", stageRunID)
	artifactRecord.Set("name", artifactName)
	artifactRecord.Set("type", artifactType)
	artifactRecord.Set("content_type", contentType)
	artifactRecord.Set("size_bytes", int64(len(data)))
	artifactRecord.Set("checksum", checksum)

	// Add metadata if provided
	if metadata != nil && len(metadata) > 0 {
		metadataJSON, err := json.Marshal(metadata)
		if err != nil {
			return checksum, fmt.Errorf("failed to marshal metadata: %w", err)
		}
		artifactRecord.Set("metadata", string(metadataJSON))
	}

	// Save to database
	if err := as.app.Save(artifactRecord); err != nil {
		// Clean up the artifact file if database save fails
		_ = os.RemoveAll(artifactDir)
		return "", fmt.Errorf("failed to save artifact metadata to database: %w", err)
	}

	// Cache metadata
	artifactMetadata := &ArtifactMetadata{
		ID:            artifactID,
		PipelineRunID: pipelineRunID,
		StageRunID:    stageRunID,
		Name:          artifactName,
		Type:          artifactType,
		ContentType:   contentType,
		SizeBytes:     int64(len(data)),
		Checksum:      checksum,
		StoragePath:   artifactPath,
		CreatedAt:     time.Now(),
		Metadata:      metadata,
	}
	as.metadataCache[artifactID] = artifactMetadata

	return checksum, nil
}

// LoadArtifact retrieves artifact data from storage.
// Returns the artifact data and metadata.
func (as *ArtifactStore) LoadArtifact(pipelineRunID string, stageRunID string, checksum string) ([]byte, *ArtifactMetadata, error) {
	collection, err := as.app.FindCollectionByNameOrId("stage_artifacts")
	if err != nil {
		return nil, nil, fmt.Errorf("stage_artifacts collection not found: %w", err)
	}

	// Query for artifact by pipeline, stage, and checksum
	records, err := as.app.FindRecordsByFilter(
		collection,
		fmt.Sprintf("pipeline_run_id = '%s' && stage_run_id = '%s' && checksum = '%s'", pipelineRunID, stageRunID, checksum),
		"",
		1,
		0,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to query artifact metadata: %w", err)
	}

	if len(records) == 0 {
		return nil, nil, fmt.Errorf("artifact not found: pipeline=%s, stage=%s, checksum=%s", pipelineRunID, stageRunID, checksum)
	}

	record := records[0]
	artifactDir := filepath.Join(as.baseDir, pipelineRunID, stageRunID, checksum)
	storagePath := filepath.Join(artifactDir, "data")

	// Read artifact data from filesystem
	data, err := os.ReadFile(storagePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read artifact file: %w", err)
	}

	// Build metadata
	artifactMetadata := &ArtifactMetadata{
		ID:            record.Id,
		PipelineRunID: record.Get("pipeline_run_id").(string),
		StageRunID:    record.Get("stage_run_id").(string),
		Name:          record.Get("name").(string),
		Type:          record.Get("type").(string),
		ContentType:   record.Get("content_type").(string),
		SizeBytes:     int64(record.Get("size_bytes").(float64)),
		Checksum:      record.Get("checksum").(string),
		StoragePath:   storagePath,
		CreatedAt:     record.GetDateTime("created_at").Time(),
	}

	// Parse metadata if present
	if metadataStr, ok := record.Get("metadata").(string); ok && metadataStr != "" {
		var metadata map[string]interface{}
		if err := json.Unmarshal([]byte(metadataStr), &metadata); err == nil {
			artifactMetadata.Metadata = metadata
		}
	}

	// Cache metadata
	as.metadataCache[record.Id] = artifactMetadata

	return data, artifactMetadata, nil
}

// LoadArtifactByID retrieves artifact data by artifact ID.
func (as *ArtifactStore) LoadArtifactByID(artifactID string) ([]byte, *ArtifactMetadata, error) {
	// Check cache first
	if cached, exists := as.metadataCache[artifactID]; exists {
		data, err := os.ReadFile(cached.StoragePath)
		if err == nil {
			return data, cached, nil
		}
	}

	collection, err := as.app.FindCollectionByNameOrId("stage_artifacts")
	if err != nil {
		return nil, nil, fmt.Errorf("stage_artifacts collection not found: %w", err)
	}

	// Load from database
	record, err := as.app.FindRecordById(collection, artifactID)
	if err != nil {
		return nil, nil, fmt.Errorf("artifact not found: %s", artifactID)
	}

	storagePath := filepath.Join(
		as.baseDir,
		record.Get("pipeline_run_id").(string),
		record.Get("stage_run_id").(string),
		record.Get("checksum").(string),
		"data",
	)

	data, err := os.ReadFile(storagePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read artifact file: %w", err)
	}

	// Build metadata
	artifactMetadata := &ArtifactMetadata{
		ID:            record.Id,
		PipelineRunID: record.Get("pipeline_run_id").(string),
		StageRunID:    record.Get("stage_run_id").(string),
		Name:          record.Get("name").(string),
		Type:          record.Get("type").(string),
		ContentType:   record.Get("content_type").(string),
		SizeBytes:     int64(record.Get("size_bytes").(float64)),
		Checksum:      record.Get("checksum").(string),
		StoragePath:   storagePath,
		CreatedAt:     record.GetDateTime("created_at").Time(),
	}

	// Parse metadata if present
	if metadataStr, ok := record.Get("metadata").(string); ok && metadataStr != "" {
		var metadata map[string]interface{}
		if err := json.Unmarshal([]byte(metadataStr), &metadata); err == nil {
			artifactMetadata.Metadata = metadata
		}
	}

	// Cache metadata
	as.metadataCache[record.Id] = artifactMetadata

	return data, artifactMetadata, nil
}

// VerifyChecksum validates that data matches the expected checksum.
// Returns true if the checksum matches, false otherwise.
func (as *ArtifactStore) VerifyChecksum(data []byte, expectedChecksum string) bool {
	return VerifyChecksum(data, expectedChecksum)
}

// ListArtifacts retrieves all artifacts for a given pipeline run and stage run.
func (as *ArtifactStore) ListArtifacts(pipelineRunID string, stageRunID string) ([]*ArtifactMetadata, error) {
	collection, err := as.app.FindCollectionByNameOrId("stage_artifacts")
	if err != nil {
		return nil, fmt.Errorf("stage_artifacts collection not found: %w", err)
	}

	records, err := as.app.FindRecordsByFilter(
		collection,
		fmt.Sprintf("pipeline_run_id = '%s' && stage_run_id = '%s'", pipelineRunID, stageRunID),
		"-created",
		100,
		0,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query artifacts: %w", err)
	}

	var artifacts []*ArtifactMetadata
	for _, record := range records {
		storagePath := filepath.Join(
			as.baseDir,
			record.Get("pipeline_run_id").(string),
			record.Get("stage_run_id").(string),
			record.Get("checksum").(string),
			"data",
		)

		metadata := &ArtifactMetadata{
			ID:            record.Id,
			PipelineRunID: record.Get("pipeline_run_id").(string),
			StageRunID:    record.Get("stage_run_id").(string),
			Name:          record.Get("name").(string),
			Type:          record.Get("type").(string),
			ContentType:   record.Get("content_type").(string),
			SizeBytes:     int64(record.Get("size_bytes").(float64)),
			Checksum:      record.Get("checksum").(string),
			StoragePath:   storagePath,
			CreatedAt:     record.GetDateTime("created_at").Time(),
		}

		// Parse metadata if present
		if metadataStr, ok := record.Get("metadata").(string); ok && metadataStr != "" {
			var parsedMetadata map[string]interface{}
			if err := json.Unmarshal([]byte(metadataStr), &parsedMetadata); err == nil {
				metadata.Metadata = parsedMetadata
			}
		}

		artifacts = append(artifacts, metadata)
	}

	return artifacts, nil
}

// ListArtifactsByPipelineRun retrieves all artifacts for a given pipeline run.
func (as *ArtifactStore) ListArtifactsByPipelineRun(pipelineRunID string) ([]*ArtifactMetadata, error) {
	collection, err := as.app.FindCollectionByNameOrId("stage_artifacts")
	if err != nil {
		return nil, fmt.Errorf("stage_artifacts collection not found: %w", err)
	}

	records, err := as.app.FindRecordsByFilter(
		collection,
		fmt.Sprintf("pipeline_run_id = '%s'", pipelineRunID),
		"-created",
		1000,
		0,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query artifacts: %w", err)
	}

	var artifacts []*ArtifactMetadata
	for _, record := range records {
		storagePath := filepath.Join(
			as.baseDir,
			record.Get("pipeline_run_id").(string),
			record.Get("stage_run_id").(string),
			record.Get("checksum").(string),
			"data",
		)

		metadata := &ArtifactMetadata{
			ID:            record.Id,
			PipelineRunID: record.Get("pipeline_run_id").(string),
			StageRunID:    record.Get("stage_run_id").(string),
			Name:          record.Get("name").(string),
			Type:          record.Get("type").(string),
			ContentType:   record.Get("content_type").(string),
			SizeBytes:     int64(record.Get("size_bytes").(float64)),
			Checksum:      record.Get("checksum").(string),
			StoragePath:   storagePath,
			CreatedAt:     record.GetDateTime("created_at").Time(),
		}

		// Parse metadata if present
		if metadataStr, ok := record.Get("metadata").(string); ok && metadataStr != "" {
			var parsedMetadata map[string]interface{}
			if err := json.Unmarshal([]byte(metadataStr), &parsedMetadata); err == nil {
				metadata.Metadata = parsedMetadata
			}
		}

		artifacts = append(artifacts, metadata)
	}

	return artifacts, nil
}

// DeleteArtifact removes an artifact from storage and database.
func (as *ArtifactStore) DeleteArtifact(artifactID string) error {
	collection, err := as.app.FindCollectionByNameOrId("stage_artifacts")
	if err != nil {
		return fmt.Errorf("stage_artifacts collection not found: %w", err)
	}

	// Get artifact metadata from database
	record, err := as.app.FindRecordById(collection, artifactID)
	if err != nil {
		return fmt.Errorf("artifact not found: %s", artifactID)
	}

	// Construct storage path
	artifactDir := filepath.Join(
		as.baseDir,
		record.Get("pipeline_run_id").(string),
		record.Get("stage_run_id").(string),
		record.Get("checksum").(string),
	)

	// Remove from filesystem
	if err := os.RemoveAll(artifactDir); err != nil {
		// Log but don't fail - try to clean up database record anyway
		fmt.Printf("Warning: failed to remove artifact directory: %v\n", err)
	}

	// Remove from database
	if err := as.app.Delete(record); err != nil {
		return fmt.Errorf("failed to delete artifact from database: %w", err)
	}

	// Remove from cache
	delete(as.metadataCache, artifactID)

	return nil
}

// CalculateChecksum computes the SHA256 checksum of data.
// Returns the hex-encoded checksum string.
func CalculateChecksum(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// VerifyChecksum validates that data matches the expected checksum.
func VerifyChecksum(data []byte, expectedChecksum string) bool {
	actualChecksum := CalculateChecksum(data)
	return actualChecksum == expectedChecksum
}

// GetArtifactStorageStats returns storage statistics for a pipeline run.
type ArtifactStorageStats struct {
	TotalArtifacts int64
	TotalSizeBytes int64
	AverageSize    int64
}

// GetStats returns storage statistics for a pipeline run.
func (as *ArtifactStore) GetStats(pipelineRunID string) (*ArtifactStorageStats, error) {
	collection, err := as.app.FindCollectionByNameOrId("stage_artifacts")
	if err != nil {
		return nil, fmt.Errorf("stage_artifacts collection not found: %w", err)
	}

	records, err := as.app.FindRecordsByFilter(
		collection,
		fmt.Sprintf("pipeline_run_id = '%s'", pipelineRunID),
		"",
		10000,
		0,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query artifacts: %w", err)
	}

	stats := &ArtifactStorageStats{
		TotalArtifacts: int64(len(records)),
	}

	for _, record := range records {
		stats.TotalSizeBytes += int64(record.Get("size_bytes").(float64))
	}

	if stats.TotalArtifacts > 0 {
		stats.AverageSize = stats.TotalSizeBytes / stats.TotalArtifacts
	}

	return stats, nil
}
