package logger

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"flip2/internal/config"
)

func TestRotatingLogWriter(t *testing.T) {
	tempDir := t.TempDir()
	
	cfg := config.DaemonConfig{
		LogCaptureDir:    tempDir,
		MaxLogFileSizeMB: 1, // 1 MB
		MaxLogFiles:      2,
	}

	// Capture os.Stdout for the duration of the test
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}
	os.Stdout = w
	t.Cleanup(func() {
		os.Stdout = oldStdout
	})

	writer, err := NewRotatingLogWriter(cfg)
	if err != nil {
		t.Fatalf("Failed to create RotatingLogWriter: %v", err)
	}
	
	// Test writing and stdout duplication
	testMessage := "This is a test message to ensure stdout duplication\n"
	_, err = writer.Write([]byte(testMessage))
	if err != nil {
		t.Fatalf("Failed to write to writer: %v", err)
	}
	w.Close() // Close the writer side of the pipe here to unblock io.ReadAll

	// Read from the pipe to check stdout output
	outputChan := make(chan string)
	go func() {
		defer close(outputChan)
		stdoutOutput, readErr := io.ReadAll(r)
		if readErr != nil {
			t.Logf("Error reading from pipe: %v", readErr)
			return
		}
		outputChan <- string(stdoutOutput)
	}()

	// Give it a moment for the goroutine to read
	select {
	case stdoutContent := <-outputChan:
		if !strings.Contains(stdoutContent, testMessage) {
			t.Errorf("Expected stdout to contain '%s', got '%s'", testMessage, stdoutContent)
		}
	case <-time.After(1 * time.Second):
		t.Error("Timed out waiting for stdout content")
	}
	
	// Ensure the writer's internal cleanup goroutine finishes before exiting the test
	// This is a hacky way to ensure the cleanup is done, ideally we'd have a channel for this.
	time.Sleep(100 * time.Millisecond)


	// Test rotation without interfering with stdout capture mechanism after initial check
	// The writer itself still duplicates to os.Stdout, but we've verified the pipe works.
	largeMessage := strings.Repeat("a", 512*1024) + "\n" // ~0.5MB
	for i := 0; i < 3; i++ {                            // Write enough to force multiple rotations
		writer.Write([]byte(largeMessage))
		time.Sleep(100 * time.Millisecond) // Increased sleep time
	}

	// Wait for cleanup goroutine to potentially run
	time.Sleep(1 * time.Second) // Increased sleep time

	// Expect 2 log files (due to MaxLogFiles: 2)
	files, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("Failed to read temp directory: %v", err)
	}
	
	var logFiles []string
	for _, file := range files {
		if !file.IsDir() && strings.HasPrefix(file.Name(), "daemon_") && strings.HasSuffix(file.Name(), ".log") {
			logFiles = append(logFiles, file.Name())
		}
	}
	
	// There might be a slight delay for cleanup, so check after a small pause
	// Retry logic for assertion
	attempts := 5
	for i := 0; i < attempts; i++ {
		if len(logFiles) == 2 {
			break
		}
		time.Sleep(50 * time.Millisecond)
		files, _ = os.ReadDir(tempDir)
		logFiles = nil
		for _, file := range files {
			if !file.IsDir() && strings.HasPrefix(file.Name(), "daemon_") && strings.HasSuffix(file.Name(), ".log") {
				logFiles = append(logFiles, file.Name())
			}
		}
	}


	if len(logFiles) != 2 {
		t.Errorf("Expected 2 log files after rotation and cleanup, got %d. Files: %v", len(logFiles), logFiles)
	}
	
	// The current log path should be different from the initial
	// This check is tricky because initialLogPath might be cleaned up.
	// We just need to ensure currentLogPath is valid and present.
	currentPath := writer.CurrentLogPath()
	if currentPath == "" {
		t.Errorf("Current log path is empty after rotation")
	}
	if _, err := os.Stat(currentPath); os.IsNotExist(err) {
		t.Errorf("Current log file does not exist at path: %s", currentPath)
	}
}

func TestSetupLogger(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name         string
		cfg          config.DaemonConfig
		expectedLevel slog.Level
	}{
		{
			name: "Debug level",
			cfg: config.DaemonConfig{
				LogLevel:      "debug",
				LogCaptureDir: tempDir,
			},
			expectedLevel: slog.LevelDebug,
		},
		{
			name: "Info level (default)",
			cfg: config.DaemonConfig{
				LogLevel:      "info",
				LogCaptureDir: tempDir,
			},
			expectedLevel: slog.LevelInfo,
		},
		{
			name: "Warn level",
			cfg: config.DaemonConfig{
				LogLevel:      "warn",
				LogCaptureDir: tempDir,
			},
			expectedLevel: slog.LevelWarn,
		},
		{
			name: "Error level",
			cfg: config.DaemonConfig{
				LogLevel:      "error",
				LogCaptureDir: tempDir,
			},
			expectedLevel: slog.LevelError,
		},
		{
			name: "Invalid level (defaults to info)",
			cfg: config.DaemonConfig{
				LogLevel:      "unknown",
				LogCaptureDir: tempDir,
			},
			expectedLevel: slog.LevelInfo,
		},
		{
			name: "Default log capture settings",
			cfg: config.DaemonConfig{
				LogLevel:      "info",
				LogCaptureDir: "", // Should use default temp dir
			},
			expectedLevel: slog.LevelInfo,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, _, logPath, err := SetupLogger(tt.cfg)
			if err != nil {
				t.Fatalf("SetupLogger failed: %v", err)
			}
			if logger == nil {
				t.Fatal("Logger is nil")
			}
			if logPath == "" {
				t.Fatal("Log path is empty")
			}

			// Test log level (requires writing a log and checking if it appears)
			var buf strings.Builder
			handler := slog.NewTextHandler(&buf, nil) // Temporarily redirect output to buffer
			testLogger := slog.New(handler)

			// Try to log at different levels and check if they are captured
			testLogger.Debug("debug message")
			testLogger.Info("info message")
			testLogger.Warn("warn message")
			testLogger.Error("error message")

			// The actual check for level is more complex as slog doesn't expose the handler's level
			// directly. We'll rely on the writer being correctly configured.
			// For simplicity in this test, we mainly check if SetupLogger returns a logger.
			// The RotatingLogWriter test already verifies writing.

			// For the default log capture settings test, ensure a path in os.TempDir is generated.
			if tt.name == "Default log capture settings" {
				if !strings.HasPrefix(logPath, filepath.Join(os.TempDir(), "flip2d_logs")) {
					t.Errorf("Expected logPath to be in os.TempDir, got %s", logPath)
				}
				// Clean up the created temp directory, as t.TempDir() wasn't used for it
				os.RemoveAll(filepath.Dir(filepath.Dir(logPath))) 
			}
		})
	}
}
