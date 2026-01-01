package alerts

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestGetDBSizeMB(t *testing.T) {
	// Create a temporary directory
	tmpDir := t.TempDir()

	// Create a test file with known size
	testFile := filepath.Join(tmpDir, "auxiliary.db")
	testData := make([]byte, 1024*1024*2) // 2 MB
	if err := os.WriteFile(testFile, testData, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Create provider (nil app is ok for this test since we only check file size)
	provider := NewPBMetricProvider(nil, tmpDir)

	ctx := context.Background()
	size, err := provider.GetDBSizeMB(ctx)
	if err != nil {
		t.Fatalf("GetDBSizeMB failed: %v", err)
	}

	// Should be approximately 2 MB
	if size < 1.9 || size > 2.1 {
		t.Errorf("Expected size ~2.0 MB, got %.2f MB", size)
	}
}

func TestGetDBSizeMB_FileNotExists(t *testing.T) {
	tmpDir := t.TempDir()

	provider := NewPBMetricProvider(nil, tmpDir)

	ctx := context.Background()
	size, err := provider.GetDBSizeMB(ctx)

	// Should return 0 (not an error condition)
	if err != nil {
		t.Errorf("Expected no error for missing file, got: %v", err)
	}
	if size != 0 {
		t.Errorf("Expected size 0 for missing file, got %.2f", size)
	}
}

func TestGetMemoryMB(t *testing.T) {
	provider := NewPBMetricProvider(nil, "")

	ctx := context.Background()
	memory, err := provider.GetMemoryMB(ctx)
	if err != nil {
		t.Fatalf("GetMemoryMB failed: %v", err)
	}

	// Memory should be positive
	if memory <= 0 {
		t.Errorf("Expected positive memory value, got %.2f", memory)
	}

	// Sanity check - should be reasonable (less than 10GB for tests)
	if memory > 10240 {
		t.Errorf("Memory value seems unreasonably high: %.2f MB", memory)
	}

	// Verify it matches runtime.ReadMemStats
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	expectedMemory := float64(m.Alloc) / (1024 * 1024)

	// Should be very close (within 1 MB)
	if memory < expectedMemory-1 || memory > expectedMemory+1 {
		t.Errorf("Memory mismatch: got %.2f, expected %.2f", memory, expectedMemory)
	}
}

func TestGetHealthCheckFailed(t *testing.T) {
	provider := NewPBMetricProvider(nil, "")

	ctx := context.Background()
	failed, err := provider.GetHealthCheckFailed(ctx)
	if err != nil {
		t.Fatalf("GetHealthCheckFailed failed: %v", err)
	}

	// Should return 0 (not implemented yet in Phase 2)
	if failed != 0 {
		t.Errorf("Expected 0 (healthy), got %d", failed)
	}
}

func TestGetSyncFailedCount1H_NoCollection(t *testing.T) {
	// Test with nil app - should handle gracefully
	provider := NewPBMetricProvider(nil, "")

	ctx := context.Background()
	count, err := provider.GetSyncFailedCount1H(ctx)

	// Should return 0 when collection doesn't exist (not an error)
	if err != nil {
		t.Errorf("Expected no error when collection missing, got: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected count 0 when collection missing, got %d", count)
	}
}
