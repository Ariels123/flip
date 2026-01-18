// Package videoeditor provides C-008: Enhanced Video Editor skill.
// Extends basic video assembly with professional editing capabilities including
// transitions, text overlays, color correction, and template-based rendering.
package videoeditor

import (
	"context"
	"encoding/json"
	"flip2/internal/content"
	"flip2/internal/logger"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Editor implements the Enhanced Video Editor skill (C-008).
// It provides professional video editing capabilities using FFmpeg.
type Editor struct {
	ffmpegPath  string
	ffprobePath string
	tempDir     string
	outputDir   string
	logger      *logger.Logger
	ops         *VideoOperations
}

// New creates a new Video Editor instance.
// ffmpegPath: path to ffmpeg binary (default: "ffmpeg")
// ffprobePath: path to ffprobe binary (default: "ffprobe")
// tempDir: directory for temporary files
// outputDir: directory for output videos
// logger: optional logger instance
func New(l *logger.Logger, ffmpegPath, ffprobePath, tempDir, outputDir string) (*Editor, error) {
	if ffmpegPath == "" {
		ffmpegPath = "ffmpeg"
	}
	if ffprobePath == "" {
		ffprobePath = "ffprobe"
	}
	if tempDir == "" {
		tempDir = os.TempDir()
	}
	if outputDir == "" {
		outputDir = "output/videos"
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	editor := &Editor{
		ffmpegPath:  ffmpegPath,
		ffprobePath: ffprobePath,
		tempDir:     tempDir,
		outputDir:   outputDir,
		logger:      l,
	}

	// Initialize operations handler
	editor.ops = NewVideoOperations(ffmpegPath, ffprobePath, tempDir)

	return editor, nil
}

// Name returns the skill identifier.
func (e *Editor) Name() string {
	return "video-editor"
}

// RequiredInputs lists the required input fields.
func (e *Editor) RequiredInputs() []string {
	return []string{"video_files", "output_format", "quality"}
}

// ProducedOutputs lists the output fields this skill produces.
func (e *Editor) ProducedOutputs() []string {
	return []string{"video_file", "duration", "resolution", "file_size"}
}

// Execute processes video editing operations.
// Input should contain VideoFiles (required), along with optional Operations, TemplateName, etc.
func (e *Editor) Execute(ctx context.Context, input content.Input) (content.Output, error) {
	startTime := time.Now()

	// Validate input
	if len(input.Files) == 0 {
		return content.Output{Error: "no input files provided"}, fmt.Errorf("no input files")
	}

	// Parse input specification
	var editorInput Input
	if input.Metadata != nil {
		// Convert metadata to editor input format
		data, err := json.Marshal(input.Metadata)
		if err == nil {
			json.Unmarshal(data, &editorInput)
		}
	}

	// Use Files from content.Input if VideoFiles is empty
	if len(editorInput.VideoFiles) == 0 {
		editorInput.VideoFiles = input.Files
	}

	// Set defaults
	if editorInput.OutputFormat == "" {
		editorInput.OutputFormat = "mp4"
	}
	if editorInput.Quality == "" {
		editorInput.Quality = "1080p"
	}
	if editorInput.FPS == 0 {
		editorInput.FPS = 30
	}

	// Determine output path
	outputPath := editorInput.OutputPath
	if outputPath == "" {
		outputPath = filepath.Join(e.outputDir, fmt.Sprintf("video_%d.%s", time.Now().UnixNano(), editorInput.OutputFormat))
	}

	if e.logger != nil {
		e.logger.InfoCtx(ctx, "Video Editor starting composition",
			"input_files", len(editorInput.VideoFiles),
			"quality", editorInput.Quality,
			"output", outputPath)
	}

	// Process the video based on input specification
	var resultPath string
	var err error

	if len(editorInput.Operations) > 0 {
		// Execute explicit operations
		resultPath, err = e.executeOperations(ctx, editorInput)
	} else if editorInput.TemplateName != "" {
		// Apply a template (Phase 2 feature)
		return content.Output{Error: "templates not yet implemented"}, fmt.Errorf("template mode not implemented in Phase 1")
	} else {
		// Basic composition: simple concat or single file handling
		resultPath, err = e.basicCompose(ctx, editorInput)
	}

	if err != nil {
		if e.logger != nil {
			e.logger.ErrorCtx(ctx, "Video editor failed", "error", err)
		}
		return content.Output{Error: err.Error()}, err
	}

	// Verify output file exists
	if _, err := os.Stat(resultPath); err != nil {
		return content.Output{Error: "output file not created"}, fmt.Errorf("output file not found: %s", resultPath)
	}

	// Get output file info
	fileInfo, err := os.Stat(resultPath)
	if err != nil {
		return content.Output{Error: err.Error()}, err
	}

	// Get video duration and resolution
	duration, _ := GetMediaDuration(e.ffprobePath, resultPath)
	info, _ := GetMediaInfo(e.ffprobePath, resultPath)

	resolution := "unknown"
	if info != nil && info.Width > 0 && info.Height > 0 {
		resolution = fmt.Sprintf("%dx%d", info.Width, info.Height)
	}

	processingTime := time.Since(startTime).Seconds()

	if e.logger != nil {
		e.logger.InfoCtx(ctx, "Video editor completed",
			"output", resultPath,
			"duration", duration,
			"size_mb", float64(fileInfo.Size())/1024/1024,
			"processing_time", processingTime)
	}

	return content.Output{
		Files: []string{resultPath},
		Data: map[string]interface{}{
			"duration":       duration,
			"resolution":     resolution,
			"file_size":      fileInfo.Size(),
			"processing_time": processingTime,
			"quality":        editorInput.Quality,
			"fps":            editorInput.FPS,
		},
	}, nil
}

// executeOperations applies a series of video operations to the input.
func (e *Editor) executeOperations(ctx context.Context, input Input) (string, error) {
	if len(input.VideoFiles) == 0 {
		return "", fmt.Errorf("no input files for operations")
	}

	workingFile := input.VideoFiles[0]
	outputPath := input.OutputPath
	if outputPath == "" {
		outputPath = filepath.Join(e.outputDir, fmt.Sprintf("edited_%d.mp4", time.Now().UnixNano()))
	}

	// Process operations sequentially
	for i, op := range input.Operations {
		var err error
		var nextWorkingFile string

		tempPath := filepath.Join(e.tempDir, fmt.Sprintf("step_%d_%d.mp4", i, time.Now().UnixNano()))

		switch op.Type {
		case "trim":
			var cutOp CutOperation
			json.Unmarshal(op.Parameters, &cutOp)
			nextWorkingFile, err = e.ops.Trim(ctx, workingFile, cutOp.StartTime, cutOp.Duration, tempPath)

		case "concat":
			var concatOp ConcatOperation
			json.Unmarshal(op.Parameters, &concatOp)
			if len(concatOp.InputFiles) > 0 {
				nextWorkingFile, err = e.ops.Concat(ctx, concatOp.InputFiles, tempPath)
			}

		case "transition":
			if i+1 < len(input.VideoFiles) {
				var transOp TransitionOperation
				json.Unmarshal(op.Parameters, &transOp)
				nextWorkingFile, err = e.ops.ApplyTransition(ctx, workingFile, input.VideoFiles[i+1], transOp.Type, transOp.Duration, tempPath)
			}

		case "text_overlay":
			var textOp TextOverlayOperation
			json.Unmarshal(op.Parameters, &textOp)
			nextWorkingFile, err = e.ops.AddTextOverlay(ctx, workingFile, textOp.Content, textOp.Position, textOp.FontSize, textOp.FontColor, textOp.FontFile, textOp.Opacity, textOp.StartTime, textOp.Duration, tempPath)

		case "color_correct":
			var colorOp ColorCorrectionOperation
			json.Unmarshal(op.Parameters, &colorOp)
			nextWorkingFile, err = e.ops.ApplyColorCorrection(ctx, workingFile, colorOp, tempPath)

		case "scale":
			var scaleOp ScaleOperation
			json.Unmarshal(op.Parameters, &scaleOp)
			nextWorkingFile, err = e.ops.Scale(ctx, workingFile, scaleOp.Width, scaleOp.Height, true, tempPath)

		default:
			return "", fmt.Errorf("unknown operation: %s", op.Type)
		}

		if err != nil {
			return "", fmt.Errorf("operation %s failed: %w", op.Type, err)
		}

		// Clean up previous temp file
		if i > 0 && workingFile != input.VideoFiles[0] {
			os.Remove(workingFile)
		}

		workingFile = nextWorkingFile
	}

	// Final encode to desired quality
	finalPath, err := e.ops.EncodeVideo(ctx, workingFile, outputPath, "mp4", "1080p", 30)
	if err != nil {
		return "", fmt.Errorf("final encode failed: %w", err)
	}

	// Clean up working file
	if workingFile != input.VideoFiles[0] {
		os.Remove(workingFile)
	}

	return finalPath, nil
}

// basicCompose handles simple video composition without explicit operations.
func (e *Editor) basicCompose(ctx context.Context, input Input) (string, error) {
	outputPath := input.OutputPath
	if outputPath == "" {
		outputPath = filepath.Join(e.outputDir, fmt.Sprintf("video_%d.mp4", time.Now().UnixNano()))
	}

	// If single file, just re-encode to target quality
	if len(input.VideoFiles) == 1 {
		quality := input.Quality
		if quality == "" {
			quality = "1080p"
		}

		resultPath, err := e.ops.EncodeVideo(ctx, input.VideoFiles[0], outputPath, input.OutputFormat, quality, input.FPS)
		if err != nil {
			return "", fmt.Errorf("encode failed: %w", err)
		}
		return resultPath, nil
	}

	// Multiple files: concatenate them
	resultPath, err := e.ops.Concat(ctx, input.VideoFiles, outputPath)
	if err != nil {
		return "", fmt.Errorf("concat failed: %w", err)
	}

	return resultPath, nil
}
