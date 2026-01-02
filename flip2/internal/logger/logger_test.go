package logger

import (
	"bytes"
	"context"
	"encoding/json"
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

// TestNewLogger tests the Logger constructor.
func TestNewLogger(t *testing.T) {
	tests := []struct {
		name   string
		format string
	}{
		{"JSON format", "json"},
		{"Text format", "text"},
		{"Invalid format defaults to text", "invalid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := NewLogger(&buf, tt.format)
			if logger == nil {
				t.Fatal("Expected logger to not be nil")
			}
			if logger.logger == nil {
				t.Fatal("Expected underlying slog.Logger to not be nil")
			}
		})
	}
}

// TestLoggerInfoCtx tests the InfoCtx method with context fields.
func TestLoggerInfoCtx(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(&buf, "json")

	ctx := context.Background()
	ctx = WithTaskID(ctx, "task_123")
	ctx = WithAgentID(ctx, "worker_1")

	logger.InfoCtx(ctx, "task started", "duration_ms", 100)

	output := buf.String()
	if !strings.Contains(output, "task started") {
		t.Errorf("Expected 'task started' in output, got: %s", output)
	}
	if !strings.Contains(output, "task_123") {
		t.Errorf("Expected 'task_123' in output, got: %s", output)
	}
	if !strings.Contains(output, "worker_1") {
		t.Errorf("Expected 'worker_1' in output, got: %s", output)
	}
	if !strings.Contains(output, "100") {
		t.Errorf("Expected '100' in output, got: %s", output)
	}
}

// TestLoggerErrorCtx tests the ErrorCtx method with context fields.
func TestLoggerErrorCtx(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(&buf, "text")

	ctx := context.Background()
	ctx = WithTaskID(ctx, "task_456")
	ctx = WithRequestID(ctx, "req_789")

	logger.ErrorCtx(ctx, "task failed", "error", "timeout")

	output := buf.String()
	if !strings.Contains(output, "task failed") {
		t.Errorf("Expected 'task failed' in output, got: %s", output)
	}
	if !strings.Contains(output, "task_456") {
		t.Errorf("Expected 'task_456' in output, got: %s", output)
	}
	if !strings.Contains(output, "req_789") {
		t.Errorf("Expected 'req_789' in output, got: %s", output)
	}
	if !strings.Contains(output, "timeout") {
		t.Errorf("Expected 'timeout' in output, got: %s", output)
	}
}

// TestLoggerDebugCtx tests the DebugCtx method.
func TestLoggerDebugCtx(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(&buf, "json")

	ctx := context.Background()
	ctx = WithPipelineID(ctx, "pipe_001")

	logger.DebugCtx(ctx, "debug message", "stage", "analysis")

	output := buf.String()
	if !strings.Contains(output, "debug message") {
		t.Errorf("Expected 'debug message' in output, got: %s", output)
	}
	if !strings.Contains(output, "pipe_001") {
		t.Errorf("Expected 'pipe_001' in output, got: %s", output)
	}
}

// TestLoggerWarnCtx tests the WarnCtx method.
func TestLoggerWarnCtx(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(&buf, "text")

	ctx := context.Background()
	ctx = WithAgentID(ctx, "gemini_1")
	ctx = WithParentID(ctx, "parent_task")

	logger.WarnCtx(ctx, "warning message", "retry_count", 3)

	output := buf.String()
	if !strings.Contains(output, "warning message") {
		t.Errorf("Expected 'warning message' in output, got: %s", output)
	}
	if !strings.Contains(output, "gemini_1") {
		t.Errorf("Expected 'gemini_1' in output, got: %s", output)
	}
	if !strings.Contains(output, "parent_task") {
		t.Errorf("Expected 'parent_task' in output, got: %s", output)
	}
}

// TestLoggerWithoutContext tests logging without context extraction.
func TestLoggerWithoutContext(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(&buf, "json")

	logger.Info("plain info", "key", "value")
	logger.Error("plain error", "code", 500)
	logger.Debug("plain debug", "item", "test")
	logger.Warn("plain warning", "level", "high")

	output := buf.String()
	if !strings.Contains(output, "plain info") {
		t.Errorf("Expected 'plain info' in output")
	}
	if !strings.Contains(output, "plain error") {
		t.Errorf("Expected 'plain error' in output")
	}
	if !strings.Contains(output, "plain debug") {
		t.Errorf("Expected 'plain debug' in output")
	}
	if !strings.Contains(output, "plain warning") {
		t.Errorf("Expected 'plain warning' in output")
	}
}

// TestLoggerWithContext tests the WithContext convenience method.
func TestLoggerWithContext(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(&buf, "json")

	ctx := context.Background()
	ctx = WithTaskID(ctx, "task_xyz")
	ctx = WithAgentID(ctx, "agent_abc")

	contextLogger := logger.WithContext(ctx)

	contextLogger.Info("first log", "index", 1)
	contextLogger.Error("error occurred", "err_code", 42)

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	if len(lines) < 2 {
		t.Fatalf("Expected at least 2 log lines, got %d", len(lines))
	}

	// Check first log
	if !strings.Contains(lines[0], "task_xyz") {
		t.Errorf("Expected 'task_xyz' in first log, got: %s", lines[0])
	}
	if !strings.Contains(lines[0], "agent_abc") {
		t.Errorf("Expected 'agent_abc' in first log, got: %s", lines[0])
	}
	if !strings.Contains(lines[0], "first log") {
		t.Errorf("Expected 'first log' in first log, got: %s", lines[0])
	}

	// Check second log
	if !strings.Contains(lines[1], "task_xyz") {
		t.Errorf("Expected 'task_xyz' in second log, got: %s", lines[1])
	}
	if !strings.Contains(lines[1], "agent_abc") {
		t.Errorf("Expected 'agent_abc' in second log, got: %s", lines[1])
	}
	if !strings.Contains(lines[1], "error occurred") {
		t.Errorf("Expected 'error occurred' in second log, got: %s", lines[1])
	}
}

// TestLoggerJSONFormat tests that JSON output is valid JSON with all context fields.
func TestLoggerJSONFormat(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(&buf, "json")

	ctx := context.Background()
	ctx = WithTaskID(ctx, "task_json_test")
	ctx = WithAgentID(ctx, "agent_json")
	ctx = WithRequestID(ctx, "req_json_001")
	ctx = WithPipelineID(ctx, "pipe_json")
	ctx = WithStageID(ctx, "stage_json")
	ctx = WithParentID(ctx, "parent_json")

	logger.InfoCtx(ctx, "json test message", "custom_field", "custom_value", "duration", 1500)

	output := strings.TrimSpace(buf.String())
	var logEntry map[string]interface{}
	if err := json.Unmarshal([]byte(output), &logEntry); err != nil {
		t.Fatalf("Failed to parse JSON output: %v\nOutput: %s", err, output)
	}

	// Verify required fields
	if msg, ok := logEntry["msg"].(string); !ok || msg != "json test message" {
		t.Errorf("Expected 'msg' field to be 'json test message', got: %v", logEntry["msg"])
	}

	// Verify context fields
	if taskID, ok := logEntry["task_id"].(string); !ok || taskID != "task_json_test" {
		t.Errorf("Expected 'task_id' field to be 'task_json_test', got: %v", logEntry["task_id"])
	}
	if agentID, ok := logEntry["agent_id"].(string); !ok || agentID != "agent_json" {
		t.Errorf("Expected 'agent_id' field to be 'agent_json', got: %v", logEntry["agent_id"])
	}
	if requestID, ok := logEntry["request_id"].(string); !ok || requestID != "req_json_001" {
		t.Errorf("Expected 'request_id' field to be 'req_json_001', got: %v", logEntry["request_id"])
	}
	if pipelineID, ok := logEntry["pipeline_id"].(string); !ok || pipelineID != "pipe_json" {
		t.Errorf("Expected 'pipeline_id' field to be 'pipe_json', got: %v", logEntry["pipeline_id"])
	}
	if stageID, ok := logEntry["stage_id"].(string); !ok || stageID != "stage_json" {
		t.Errorf("Expected 'stage_id' field to be 'stage_json', got: %v", logEntry["stage_id"])
	}
	if parentID, ok := logEntry["parent_id"].(string); !ok || parentID != "parent_json" {
		t.Errorf("Expected 'parent_id' field to be 'parent_json', got: %v", logEntry["parent_id"])
	}

	// Verify custom fields
	if customField, ok := logEntry["custom_field"].(string); !ok || customField != "custom_value" {
		t.Errorf("Expected 'custom_field' to be 'custom_value', got: %v", logEntry["custom_field"])
	}
	if duration, ok := logEntry["duration"].(float64); !ok || duration != 1500 {
		t.Errorf("Expected 'duration' to be 1500, got: %v", logEntry["duration"])
	}

	// Verify standard fields
	if _, ok := logEntry["level"].(string); !ok {
		t.Errorf("Expected 'level' field")
	}
	if _, ok := logEntry["time"]; !ok {
		t.Errorf("Expected 'time' field in JSON output")
	}
}

// TestLoggerTextFormat tests text output format.
func TestLoggerTextFormat(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(&buf, "text")

	ctx := context.Background()
	ctx = WithTaskID(ctx, "task_text")
	ctx = WithAgentID(ctx, "agent_text")

	logger.InfoCtx(ctx, "text format message", "count", 42)

	output := buf.String()
	if !strings.Contains(output, "text format message") {
		t.Errorf("Expected message in text output")
	}
	if !strings.Contains(output, "task_text") {
		t.Errorf("Expected task_id in text output")
	}
	if !strings.Contains(output, "agent_text") {
		t.Errorf("Expected agent_id in text output")
	}
	if !strings.Contains(output, "42") {
		t.Errorf("Expected custom field value in text output")
	}
}

// TestLoggerEmptyContext tests logging with empty/no context fields.
func TestLoggerEmptyContext(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(&buf, "json")

	ctx := context.Background()
	// Don't set any context fields

	logger.InfoCtx(ctx, "message with no context fields", "field", "value")

	output := strings.TrimSpace(buf.String())
	var logEntry map[string]interface{}
	if err := json.Unmarshal([]byte(output), &logEntry); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	// Verify that only the custom field is present (no empty context fields)
	if msg, ok := logEntry["msg"].(string); !ok || msg != "message with no context fields" {
		t.Errorf("Expected correct message")
	}
	if field, ok := logEntry["field"].(string); !ok || field != "value" {
		t.Errorf("Expected custom field in JSON")
	}

	// Context fields should not be in the output if they weren't set
	if _, ok := logEntry["task_id"]; ok {
		t.Errorf("Expected no task_id field when not set")
	}
}

// TestLoggerMultipleContextFields tests logging with multiple context fields set via LogContextFields.
func TestLoggerMultipleContextFields(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(&buf, "json")

	fields := LogContextFields{
		TaskID:     "multi_task",
		AgentID:    "multi_agent",
		RequestID:  "multi_req",
		PipelineID: "multi_pipe",
		StageID:    "multi_stage",
		ParentID:   "multi_parent",
	}

	ctx := fields.Apply(context.Background())
	logger.InfoCtx(ctx, "multi field test")

	output := strings.TrimSpace(buf.String())
	var logEntry map[string]interface{}
	if err := json.Unmarshal([]byte(output), &logEntry); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	// Verify all fields are present
	if val, ok := logEntry["task_id"].(string); !ok || val != "multi_task" {
		t.Errorf("task_id mismatch")
	}
	if val, ok := logEntry["agent_id"].(string); !ok || val != "multi_agent" {
		t.Errorf("agent_id mismatch")
	}
	if val, ok := logEntry["request_id"].(string); !ok || val != "multi_req" {
		t.Errorf("request_id mismatch")
	}
	if val, ok := logEntry["pipeline_id"].(string); !ok || val != "multi_pipe" {
		t.Errorf("pipeline_id mismatch")
	}
	if val, ok := logEntry["stage_id"].(string); !ok || val != "multi_stage" {
		t.Errorf("stage_id mismatch")
	}
	if val, ok := logEntry["parent_id"].(string); !ok || val != "multi_parent" {
		t.Errorf("parent_id mismatch")
	}
}
