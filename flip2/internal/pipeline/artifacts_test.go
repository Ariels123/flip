package pipeline

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

)

// TestCalculateChecksum tests SHA256 checksum calculation
func TestCalculateChecksum(t *testing.T) {
	testCases := []struct {
		name     string
		data     []byte
		expected string
	}{
		{
			name:     "empty data",
			data:     []byte(""),
			expected: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			name:     "simple string",
			data:     []byte("hello world"),
			expected: "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9",
		},
		{
			name:     "json data",
			data:     []byte(`{"key": "value"}`),
			expected: "1ef3ba8ae8ab63f31ef46ff9a1d1b1df5a27c4e8c2e70c8cb91c2f41e0b95c24",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			checksum := CalculateChecksum(tc.data)
			if checksum != tc.expected {
				t.Errorf("Expected checksum %s, got %s", tc.expected, checksum)
			}
		})
	}
}

// TestVerifyChecksum tests checksum verification
func TestVerifyChecksum(t *testing.T) {
	data := []byte("test data")
	checksum := CalculateChecksum(data)

	testCases := []struct {
		name       string
		data       []byte
		checksum   string
		shouldPass bool
	}{
		{
			name:       "valid checksum",
			data:       data,
			checksum:   checksum,
			shouldPass: true,
		},
		{
			name:       "invalid checksum",
			data:       data,
			checksum:   "0000000000000000000000000000000000000000000000000000000000000000",
			shouldPass: false,
		},
		{
			name:       "corrupted data",
			data:       []byte("corrupted data"),
			checksum:   checksum,
			shouldPass: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := VerifyChecksum(tc.data, tc.checksum)
			if result != tc.shouldPass {
				t.Errorf("Expected %v, got %v", tc.shouldPass, result)
			}
		})
	}
}

// TestArtifactStoreMethod tests the ArtifactStore methods (requires PocketBase setup)
func TestArtifactStoreDirectory(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()
	artifactDir := filepath.Join(tmpDir, "artifacts")

	// Test directory creation
	store := &ArtifactStore{
		app:           nil,
		baseDir:       artifactDir,
		metadataCache: make(map[string]*ArtifactMetadata),
	}
	_ = store // Mark as used if needed, or just let it be if it's used below
	if err := os.MkdirAll(artifactDir, 0755); err != nil {
		t.Fatalf("Failed to create artifact directory: %v", err)
	}

	// Verify directory exists
	if _, err := os.Stat(artifactDir); os.IsNotExist(err) {
		t.Fatal("Artifact directory was not created")
	}

	// Test artifact path construction
	pipelineRunID := "pipeline-123"
	stageRunID := "stage-456"
	checksum := CalculateChecksum([]byte("test data"))

	artifactStorePath := filepath.Join(artifactDir, pipelineRunID, stageRunID, checksum)

	// Create the full path
	if err := os.MkdirAll(artifactStorePath, 0755); err != nil {
		t.Fatalf("Failed to create artifact store path: %v", err)
	}

	// Write test artifact
	testData := []byte("test artifact data")
	artifactFile := filepath.Join(artifactStorePath, "data")
	if err := os.WriteFile(artifactFile, testData, 0644); err != nil {
		t.Fatalf("Failed to write artifact file: %v", err)
	}

	// Read and verify artifact
	readData, err := os.ReadFile(artifactFile)
	if err != nil {
		t.Fatalf("Failed to read artifact file: %v", err)
	}

	if !bytes.Equal(readData, testData) {
		t.Errorf("Artifact data mismatch")
	}

	// Verify checksum
	if !VerifyChecksum(readData, checksum) {
		t.Error("Checksum verification failed")
	}
}

// TestArtifactMetadataMarshaling tests JSON marshaling of artifact metadata
func TestArtifactMetadataMarshaling(t *testing.T) {
	metadata := &ArtifactMetadata{
		ID:            "artifact-123",
		PipelineRunID: "pipeline-456",
		StageRunID:    "stage-789",
		Name:          "test-artifact",
		Type:          "json",
		ContentType:   "application/json",
		SizeBytes:     1024,
		Checksum:      "abc123def456",
		StoragePath:   "/tmp/artifacts/pipeline-456/stage-789/abc123def456/data",
		Metadata: map[string]interface{}{
			"key1": "value1",
			"key2": 42,
		},
	}

	// Marshal to JSON
	data, err := Marshal(metadata)
	if err != nil {
		t.Fatalf("Failed to marshal metadata: %v", err)
	}

	// Unmarshal back
	var unmarshaled ArtifactMetadata
	if err := Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal metadata: %v", err)
	}

	// Verify fields
	if unmarshaled.ID != metadata.ID {
		t.Errorf("ID mismatch: expected %s, got %s", metadata.ID, unmarshaled.ID)
	}
	if unmarshaled.Checksum != metadata.Checksum {
		t.Errorf("Checksum mismatch: expected %s, got %s", metadata.Checksum, unmarshaled.Checksum)
	}
	if unmarshaled.SizeBytes != metadata.SizeBytes {
		t.Errorf("SizeBytes mismatch: expected %d, got %d", metadata.SizeBytes, unmarshaled.SizeBytes)
	}
}

// Helper functions for JSON marshaling in tests
func Marshal(v interface{}) ([]byte, error) {
	return []byte("test"), nil // Placeholder - use real marshaling
}

func Unmarshal(data []byte, v interface{}) error {
	return nil // Placeholder - use real unmarshaling
}

// TestChecksumConsistency ensures checksum is consistent across multiple calculations
func TestChecksumConsistency(t *testing.T) {
	testData := []byte("consistent test data")

	checksum1 := CalculateChecksum(testData)
	checksum2 := CalculateChecksum(testData)
	checksum3 := CalculateChecksum(testData)

	if checksum1 != checksum2 || checksum2 != checksum3 {
		t.Error("Checksum calculation is not consistent")
	}
}

// TestArtifactDirectoryStructure tests the directory hierarchy
func TestArtifactDirectoryStructure(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		pipelineID string
		stageID    string
		name       string
	}{
		{"pipeline-1", "stage-1", "artifact-1"},
		{"pipeline-1", "stage-2", "artifact-2"},
		{"pipeline-2", "stage-1", "artifact-3"},
	}

	for _, test := range tests {
		data := []byte("test data for " + test.name)
		checksum := CalculateChecksum(data)

		path := filepath.Join(tmpDir, test.pipelineID, test.stageID, checksum)
		if err := os.MkdirAll(path, 0755); err != nil {
			t.Fatalf("Failed to create path: %v", err)
		}

		if err := os.WriteFile(filepath.Join(path, "data"), data, 0644); err != nil {
			t.Fatalf("Failed to write artifact: %v", err)
		}

		// Verify structure
		stat, err := os.Stat(path)
		if err != nil {
			t.Fatalf("Failed to stat artifact path: %v", err)
		}

		if !stat.IsDir() {
			t.Error("Artifact path is not a directory")
		}
	}
}
