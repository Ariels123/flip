package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort" // Added missing import
	"strings" // Added missing import
	"sync"
	"time"

	"flip2/internal/config"
)

// RotatingLogWriter implements io.Writer and writes to a rotating file.
// It also duplicates writes to os.Stdout.
type RotatingLogWriter struct {
	mu           sync.Mutex
	config       config.DaemonConfig
	currentFile  *os.File
	currentBytes int64
	currentPath  string
}

// NewRotatingLogWriter creates a new RotatingLogWriter.
func NewRotatingLogWriter(cfg config.DaemonConfig) (*RotatingLogWriter, error) {
	w := &RotatingLogWriter{
		config: cfg,
	}

	if cfg.LogCaptureDir == "" {
		return nil, fmt.Errorf("LogCaptureDir must be specified in daemon config for rotating logs")
	}
	if err := os.MkdirAll(cfg.LogCaptureDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log capture directory: %w", err)
	}

	if err := w.rotate(); err != nil {
		return nil, fmt.Errorf("failed to create initial log file: %w", err)
	}

	return w, nil
}

// Write writes data to the current log file and also to os.Stdout.
// It performs log rotation if the current file exceeds the configured size.
func (w *RotatingLogWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Write to os.Stdout
	_, _ = os.Stdout.Write(p)

	// Check for rotation before writing to file
	if w.currentFile != nil && w.currentBytes+int64(len(p)) > int64(w.config.MaxLogFileSizeMB)*1024*1024 {
		if err := w.rotate(); err != nil {
			return 0, fmt.Errorf("failed to rotate log file: %w", err)
		}
	}

	if w.currentFile == nil {
		// This should not happen if rotate() was successful in New or above
		return 0, fmt.Errorf("no log file open for writing")
	}

	n, err = w.currentFile.Write(p)
	w.currentBytes += int64(n)
	return n, err
}

// rotate closes the current log file (if any) and opens a new one.
// It also handles old log file cleanup.
func (w *RotatingLogWriter) rotate() error {
	if w.currentFile != nil {
		w.currentFile.Close()
	}

	timestamp := time.Now().Format("20060102_150405")
	logFileName := fmt.Sprintf("daemon_%s.log", timestamp)
	newPath := filepath.Join(w.config.LogCaptureDir, logFileName)

	f, err := os.OpenFile(newPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	w.currentFile = f
	w.currentPath = newPath
	w.currentBytes = 0 // Reset byte count for the new file

	// Clean up old files synchronously
	w.cleanupOldLogs()

	return nil
}

// CurrentLogPath returns the path to the currently active log file.
func (w *RotatingLogWriter) CurrentLogPath() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.currentPath
}

// cleanupOldLogs removes log files exceeding MaxLogFiles.
func (w *RotatingLogWriter) cleanupOldLogs() {
	files, err := os.ReadDir(w.config.LogCaptureDir)
	if err != nil {
		slog.Error("Failed to read log directory for cleanup", "error", err, "dir", w.config.LogCaptureDir)
		return
	}

	var logFiles []string
	for _, file := range files {
		if !file.IsDir() && strings.HasPrefix(file.Name(), "daemon_") && strings.HasSuffix(file.Name(), ".log") {
			logFiles = append(logFiles, filepath.Join(w.config.LogCaptureDir, file.Name()))
		}
	}

	// Sort files by name (which includes timestamp), oldest first
	sort.Strings(logFiles)

	for len(logFiles) > w.config.MaxLogFiles {
		toRemove := logFiles[0]
		if err := os.Remove(toRemove); err != nil {
			slog.Error("Failed to remove old log file", "error", err, "file", toRemove)
		} else {
			slog.Info("Removed old log file", "file", toRemove)
		}
		logFiles = logFiles[1:]
	}
}

// SetupLogger initializes a new slog.Logger based on the daemon config.
// It returns the logger and the path to the currently active log file.
func SetupLogger(cfg config.DaemonConfig) (*slog.Logger, *RotatingLogWriter, string, error) {
	var level slog.Level
	switch cfg.LogLevel {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	// Default values for log rotation
	if cfg.MaxLogFileSizeMB == 0 {
		cfg.MaxLogFileSizeMB = 10 // 10 MB
	}
	if cfg.MaxLogFiles == 0 {
		cfg.MaxLogFiles = 5 // Keep 5 log files
	}
	if cfg.LogCaptureDir == "" {
		cfg.LogCaptureDir = filepath.Join(os.TempDir(), "flip2d_logs")
	}

	writer, err := NewRotatingLogWriter(cfg)
	if err != nil {
		return nil, nil, "", fmt.Errorf("failed to create rotating log writer: %w", err)
	}

	handler := slog.NewTextHandler(writer, &slog.HandlerOptions{
		Level: level,
	})

	return slog.New(handler), writer, writer.CurrentLogPath(), nil
}

// Logger wraps slog.Logger and provides context-aware logging methods.
// It automatically extracts context fields (task_id, agent_id, etc.) and includes them in log records.
type Logger struct {
	logger *slog.Logger
	format string // "json" or "text"
}

// NewLogger creates a new context-aware Logger with the specified output format.
// format should be either "json" or "text" (defaults to "text" if unknown).
func NewLogger(output io.Writer, format string) *Logger {
	if format != "json" && format != "text" {
		format = "text"
	}

	var handler slog.Handler
	if format == "json" {
		handler = slog.NewJSONHandler(output, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})
	} else {
		handler = slog.NewTextHandler(output, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})
	}

	return &Logger{
		logger: slog.New(handler),
		format: format,
	}
}

// WrapSlogLogger wraps an existing slog.Logger with context-aware logging functionality.
// This is useful when you already have a configured slog.Logger (e.g., from SetupLogger).
func WrapSlogLogger(slogger *slog.Logger) *Logger {
	return &Logger{
		logger: slogger,
		format: "text",
	}
}

// InfoCtx logs an info message with context fields extracted from the context.
// Additional key-value pairs can be provided after the message.
func (l *Logger) InfoCtx(ctx context.Context, msg string, args ...any) {
	l.logWithContext(ctx, slog.LevelInfo, msg, args...)
}

// ErrorCtx logs an error message with context fields extracted from the context.
// Additional key-value pairs can be provided after the message.
func (l *Logger) ErrorCtx(ctx context.Context, msg string, args ...any) {
	l.logWithContext(ctx, slog.LevelError, msg, args...)
}

// DebugCtx logs a debug message with context fields extracted from the context.
// Additional key-value pairs can be provided after the message.
func (l *Logger) DebugCtx(ctx context.Context, msg string, args ...any) {
	l.logWithContext(ctx, slog.LevelDebug, msg, args...)
}

// WarnCtx logs a warning message with context fields extracted from the context.
// Additional key-value pairs can be provided after the message.
func (l *Logger) WarnCtx(ctx context.Context, msg string, args ...any) {
	l.logWithContext(ctx, slog.LevelWarn, msg, args...)
}

// logWithContext is the internal helper that extracts context fields and logs.
func (l *Logger) logWithContext(ctx context.Context, level slog.Level, msg string, args ...any) {
	// Extract context fields
	fields := ExtractLogFields(ctx)

	// Build combined args with context fields first, then additional args
	combinedArgs := make([]any, 0, len(fields)*2+len(args))
	for key, value := range fields {
		combinedArgs = append(combinedArgs, key, value)
	}
	combinedArgs = append(combinedArgs, args...)

	// Log using the underlying slog logger
	l.logger.Log(ctx, level, msg, combinedArgs...)
}

// Info logs an info message (without context extraction).
// This is a convenience method that bypasses context extraction.
func (l *Logger) Info(msg string, args ...any) {
	l.logger.Info(msg, args...)
}

// Error logs an error message (without context extraction).
// This is a convenience method that bypasses context extraction.
func (l *Logger) Error(msg string, args ...any) {
	l.logger.Error(msg, args...)
}

// Debug logs a debug message (without context extraction).
// This is a convenience method that bypasses context extraction.
func (l *Logger) Debug(msg string, args ...any) {
	l.logger.Debug(msg, args...)
}

// Warn logs a warning message (without context extraction).
// This is a convenience method that bypasses context extraction.
func (l *Logger) Warn(msg string, args ...any) {
	l.logger.Warn(msg, args...)
}

// WithContext returns a new Logger that always uses the given context for field extraction.
// Useful when you want to apply the same context fields to multiple log calls.
func (l *Logger) WithContext(ctx context.Context) *ContextLogger {
	return &ContextLogger{
		logger: l,
		ctx:    ctx,
	}
}

// ContextLogger wraps Logger and a context for convenient logging with implicit context.
type ContextLogger struct {
	logger *Logger
	ctx    context.Context
}

// Info logs an info message using the embedded context.
func (cl *ContextLogger) Info(msg string, args ...any) {
	cl.logger.InfoCtx(cl.ctx, msg, args...)
}

// Error logs an error message using the embedded context.
func (cl *ContextLogger) Error(msg string, args ...any) {
	cl.logger.ErrorCtx(cl.ctx, msg, args...)
}

// Debug logs a debug message using the embedded context.
func (cl *ContextLogger) Debug(msg string, args ...any) {
	cl.logger.DebugCtx(cl.ctx, msg, args...)
}

// Warn logs a warning message using the embedded context.
func (cl *ContextLogger) Warn(msg string, args ...any) {
	cl.logger.WarnCtx(cl.ctx, msg, args...)
}
