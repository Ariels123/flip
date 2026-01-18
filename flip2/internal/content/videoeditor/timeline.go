// Package videoeditor - Timeline execution engine for template rendering.
package videoeditor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"flip2/internal/logger"
)

// TimelineExecutor processes template timelines and renders videos.
type TimelineExecutor struct {
	operations  *VideoOperations
	substitutor *VariableSubstitutor
	tempDir     string
	logger      *logger.Logger
}

// NewTimelineExecutor creates a new timeline executor.
func NewTimelineExecutor(operations *VideoOperations, tempDir string, logger *logger.Logger) *TimelineExecutor {
	return &TimelineExecutor{
		operations: operations,
		tempDir:    tempDir,
		logger:     logger,
	}
}

// Execute processes a template timeline and produces the final video.
// Variables are substituted before execution.
func (te *TimelineExecutor) Execute(ctx context.Context, template *Template, variables map[string]interface{}, outputPath string) (string, error) {
	if te.logger != nil {
		te.logger.InfoCtx(ctx, "Timeline execution starting",
			"template", template.Name,
			"timeline_items", len(template.Timeline),
			"output", outputPath)
	}

	// Apply variable substitution to template
	te.substitutor = NewVariableSubstitutor(variables)
	processedTemplate, err := te.substitutor.SubstituteTemplate(template)
	if err != nil {
		return "", fmt.Errorf("variable substitution failed: %w", err)
	}

	// Validate timeline
	if err := te.validateTimeline(processedTemplate); err != nil {
		return "", fmt.Errorf("timeline validation failed: %w", err)
	}

	// Build execution plan
	plan, err := te.buildExecutionPlan(ctx, processedTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to build execution plan: %w", err)
	}

	// Execute the plan
	result, err := te.executePlan(ctx, plan, processedTemplate, outputPath)
	if err != nil {
		return "", fmt.Errorf("execution failed: %w", err)
	}

	if te.logger != nil {
		te.logger.InfoCtx(ctx, "Timeline execution completed", "output", result)
	}

	return result, nil
}

// ExecutionPlan represents the planned operations for timeline rendering.
type ExecutionPlan struct {
	// Stages are sequential processing stages
	Stages []ExecutionStage

	// TotalDuration is the expected output duration in milliseconds
	TotalDuration float64
}

// ExecutionStage represents a single stage in the execution plan.
type ExecutionStage struct {
	// Type: "video", "audio", "text", "image", "transition"
	Type string

	// Item from the timeline
	Item *TimelineItem

	// TempPath for intermediate output
	TempPath string

	// Dependencies on previous stages
	Dependencies []int
}

// validateTimeline checks the timeline for errors.
func (te *TimelineExecutor) validateTimeline(template *Template) error {
	if len(template.Timeline) == 0 {
		return fmt.Errorf("timeline is empty")
	}

	// Check for overlapping items (simple check)
	for i := 0; i < len(template.Timeline); i++ {
		item := template.Timeline[i]

		// Validate timing
		if item.StartTime < 0 {
			return fmt.Errorf("timeline item %d: negative start time", i)
		}
		if item.Duration <= 0 {
			return fmt.Errorf("timeline item %d: non-positive duration", i)
		}

		// Validate paths exist for media items
		if item.Type != "text" && item.Path != "" {
			if _, err := os.Stat(item.Path); err != nil {
				return fmt.Errorf("timeline item %d: file not found: %s", i, item.Path)
			}
		}
	}

	return nil
}

// buildExecutionPlan creates a plan for executing the timeline.
func (te *TimelineExecutor) buildExecutionPlan(ctx context.Context, template *Template) (*ExecutionPlan, error) {
	plan := &ExecutionPlan{
		Stages: make([]ExecutionStage, 0),
	}

	// Calculate total duration
	maxEndTime := 0.0
	for _, item := range template.Timeline {
		endTime := item.StartTime + item.Duration
		if endTime > maxEndTime {
			maxEndTime = endTime
		}
	}
	plan.TotalDuration = maxEndTime

	// Build stages for each timeline item
	for i, item := range template.Timeline {
		stage := ExecutionStage{
			Type:         item.Type,
			Item:         &template.Timeline[i],
			TempPath:     filepath.Join(te.tempDir, fmt.Sprintf("stage_%d_%d.mp4", i, time.Now().UnixNano())),
			Dependencies: make([]int, 0),
		}

		plan.Stages = append(plan.Stages, stage)
	}

	return plan, nil
}

// executePlan executes the planned operations.
func (te *TimelineExecutor) executePlan(ctx context.Context, plan *ExecutionPlan, template *Template, outputPath string) (string, error) {
	// Group timeline items by type for processing
	videoItems := make([]*TimelineItem, 0)
	textOverlays := make([]*TimelineItem, 0)
	audioItems := make([]*TimelineItem, 0)

	for i := range template.Timeline {
		item := &template.Timeline[i]
		switch item.Type {
		case "video", "image":
			videoItems = append(videoItems, item)
		case "text":
			textOverlays = append(textOverlays, item)
		case "audio":
			audioItems = append(audioItems, item)
		}
	}

	// Process video/image layers first
	baseVideo, err := te.processVideoLayers(ctx, videoItems, template, plan)
	if err != nil {
		return "", fmt.Errorf("failed to process video layers: %w", err)
	}

	workingFile := baseVideo

	// Apply text overlays
	if len(textOverlays) > 0 {
		workingFile, err = te.applyTextOverlays(ctx, workingFile, textOverlays, template)
		if err != nil {
			// Clean up
			if baseVideo != "" {
				os.Remove(baseVideo)
			}
			return "", fmt.Errorf("failed to apply text overlays: %w", err)
		}

		// Clean up previous working file
		if workingFile != baseVideo {
			os.Remove(baseVideo)
		}
	}

	// Mix audio if needed
	if len(audioItems) > 0 {
		workingFile, err = te.mixAudio(ctx, workingFile, audioItems)
		if err != nil {
			os.Remove(workingFile)
			return "", fmt.Errorf("failed to mix audio: %w", err)
		}
	}

	// Apply global effects
	if len(template.Effects) > 0 {
		workingFile, err = te.applyGlobalEffects(ctx, workingFile, template.Effects)
		if err != nil {
			os.Remove(workingFile)
			return "", fmt.Errorf("failed to apply global effects: %w", err)
		}
	}

	// Final encode to output path with target quality
	finalPath, err := te.operations.EncodeVideo(ctx, workingFile, outputPath, "mp4", "1080p", template.Canvas.FPS)
	if err != nil {
		os.Remove(workingFile)
		return "", fmt.Errorf("final encode failed: %w", err)
	}

	// Clean up working file
	if workingFile != finalPath {
		os.Remove(workingFile)
	}

	return finalPath, nil
}

// processVideoLayers processes video and image timeline items.
func (te *TimelineExecutor) processVideoLayers(ctx context.Context, items []*TimelineItem, template *Template, plan *ExecutionPlan) (string, error) {
	if len(items) == 0 {
		// Create blank canvas
		return te.createBlankCanvas(ctx, template.Canvas, plan.TotalDuration/1000.0)
	}

	// Process each video item
	processedClips := make([]string, 0)
	defer func() {
		// Clean up temporary clips
		for _, clip := range processedClips {
			os.Remove(clip)
		}
	}()

	for i, item := range items {
		var processedClip string
		var err error

		// Trim/adjust duration if needed
		if item.Type == "video" {
			// Check if we need to trim
			info, err := GetMediaInfo(te.operations.FFprobePath, item.Path)
			if err != nil {
				return "", fmt.Errorf("failed to get media info for %s: %w", item.Path, err)
			}

			expectedDuration := item.Duration / 1000.0 // Convert to seconds
			if info.Duration > expectedDuration+0.1 { // Allow small tolerance
				// Need to trim
				tempTrim := filepath.Join(te.tempDir, fmt.Sprintf("trim_%d_%d.mp4", i, time.Now().UnixNano()))
				processedClip, err = te.operations.Trim(ctx, item.Path, item.StartTime/1000.0, item.Duration/1000.0, tempTrim)
				if err != nil {
					return "", fmt.Errorf("failed to trim video %s: %w", item.Path, err)
				}
			} else {
				processedClip = item.Path
			}
		} else if item.Type == "image" {
			// Convert image to video clip
			tempImg := filepath.Join(te.tempDir, fmt.Sprintf("img_%d_%d.mp4", i, time.Now().UnixNano()))
			processedClip, err = te.imageToVideo(ctx, item.Path, item.Duration/1000.0, template.Canvas, tempImg)
			if err != nil {
				return "", fmt.Errorf("failed to convert image %s: %w", item.Path, err)
			}
		}

		// Scale to canvas size if needed
		info, err := GetMediaInfo(te.operations.FFprobePath, processedClip)
		if err != nil {
			if processedClip != item.Path {
				os.Remove(processedClip)
			}
			return "", fmt.Errorf("failed to get media info: %w", err)
		}

		if info.Width != template.Canvas.Width || info.Height != template.Canvas.Height {
			tempScale := filepath.Join(te.tempDir, fmt.Sprintf("scale_%d_%d.mp4", i, time.Now().UnixNano()))
			scaledClip, err := te.operations.Scale(ctx, processedClip, template.Canvas.Width, template.Canvas.Height, true, tempScale)
			if err != nil {
				if processedClip != item.Path {
					os.Remove(processedClip)
				}
				return "", fmt.Errorf("failed to scale video: %w", err)
			}

			// Clean up pre-scaled clip
			if processedClip != item.Path {
				os.Remove(processedClip)
			}
			processedClip = scaledClip
		}

		// Apply item-specific effects
		if len(item.Effects) > 0 {
			tempEffects := filepath.Join(te.tempDir, fmt.Sprintf("effects_%d_%d.mp4", i, time.Now().UnixNano()))
			effectsClip, err := te.applyItemEffects(ctx, processedClip, item.Effects, tempEffects)
			if err != nil {
				if processedClip != item.Path {
					os.Remove(processedClip)
				}
				return "", fmt.Errorf("failed to apply item effects: %w", err)
			}

			if processedClip != item.Path {
				os.Remove(processedClip)
			}
			processedClip = effectsClip
		}

		// Apply transitions if specified
		if item.Transitions.In != nil || item.Transitions.Out != nil {
			tempTrans := filepath.Join(te.tempDir, fmt.Sprintf("trans_%d_%d.mp4", i, time.Now().UnixNano()))
			transClip, err := te.applyItemTransitions(ctx, processedClip, item.Transitions, tempTrans)
			if err != nil {
				if processedClip != item.Path {
					os.Remove(processedClip)
				}
				return "", fmt.Errorf("failed to apply transitions: %w", err)
			}

			if processedClip != item.Path {
				os.Remove(processedClip)
			}
			processedClip = transClip
		}

		processedClips = append(processedClips, processedClip)
	}

	// Concatenate all processed clips
	if len(processedClips) == 1 {
		// Single clip - just return it (caller will clean up)
		result := processedClips[0]
		processedClips = nil // Prevent cleanup
		return result, nil
	}

	// Multiple clips - concatenate
	concatOutput := filepath.Join(te.tempDir, fmt.Sprintf("concat_%d.mp4", time.Now().UnixNano()))
	result, err := te.operations.Concat(ctx, processedClips, concatOutput)
	if err != nil {
		return "", fmt.Errorf("failed to concatenate clips: %w", err)
	}

	return result, nil
}

// applyTextOverlays adds text overlays to the video.
func (te *TimelineExecutor) applyTextOverlays(ctx context.Context, inputPath string, overlays []*TimelineItem, template *Template) (string, error) {
	workingFile := inputPath

	for i, overlay := range overlays {
		// Get text style parameters
		fontSize := 48
		fontColor := "#FFFFFF"
		fontFile := ""
		opacity := 1.0
		position := "center"

		if overlay.Style != nil {
			if overlay.Style.FontSize > 0 {
				fontSize = overlay.Style.FontSize
			}
			if overlay.Style.FontColor != "" {
				fontColor = overlay.Style.FontColor
			}
			if overlay.Style.FontFile != "" {
				fontFile = overlay.Style.FontFile
			}
			if overlay.Style.Opacity > 0 {
				opacity = overlay.Style.Opacity
			}
			if overlay.Style.Position != "" {
				position = overlay.Style.Position
			}
		}

		// Apply text overlay
		tempPath := filepath.Join(te.tempDir, fmt.Sprintf("text_%d_%d.mp4", i, time.Now().UnixNano()))
		result, err := te.operations.AddTextOverlay(ctx, workingFile, overlay.Content, position, fontSize, fontColor, fontFile, opacity, overlay.StartTime/1000.0, overlay.Duration/1000.0, tempPath)
		if err != nil {
			if workingFile != inputPath {
				os.Remove(workingFile)
			}
			return "", fmt.Errorf("failed to add text overlay: %w", err)
		}

		// Clean up previous working file
		if workingFile != inputPath {
			os.Remove(workingFile)
		}

		workingFile = result
	}

	return workingFile, nil
}

// mixAudio mixes audio tracks with the video.
func (te *TimelineExecutor) mixAudio(ctx context.Context, videoPath string, audioItems []*TimelineItem) (string, error) {
	// For now, just mix the first audio track
	// TODO: Implement multi-track audio mixing with proper timing
	if len(audioItems) == 0 {
		return videoPath, nil
	}

	// Use first audio item
	audio := audioItems[0]

	// Build FFmpeg command to replace audio
	tempPath := filepath.Join(te.tempDir, fmt.Sprintf("audio_%d.mp4", time.Now().UnixNano()))

	cmd := NewFFmpegCommand(te.operations.FFmpegPath, te.operations.FFprobePath)
	cmd.AddInput(videoPath)
	cmd.AddInput(audio.Path)
	cmd.AddArg("-c:v").AddArg("copy")
	cmd.AddArg("-c:a").AddArg("aac")
	cmd.AddArg("-map").AddArg("0:v:0")
	cmd.AddArg("-map").AddArg("1:a:0")
	cmd.SetOutput(tempPath)

	execCmd := exec.CommandContext(ctx, te.operations.FFmpegPath, cmd.Build()...)
	if output, err := execCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("failed to mix audio: %w\nOutput: %s", err, string(output))
	}

	return tempPath, nil
}

// applyGlobalEffects applies global effects to the entire video.
func (te *TimelineExecutor) applyGlobalEffects(ctx context.Context, inputPath string, effects []EffectConfig) (string, error) {
	workingFile := inputPath

	for i, effect := range effects {
		var result string
		var err error

		tempPath := filepath.Join(te.tempDir, fmt.Sprintf("effect_%d_%d.mp4", i, time.Now().UnixNano()))

		switch effect.Type {
		case "color_correct":
			// Parse color correction parameters from map
			colorOp := ColorCorrectionOperation{
				Brightness: getFloat64FromMap(effect.Parameters, "brightness", 0.0),
				Contrast:   getFloat64FromMap(effect.Parameters, "contrast", 1.0),
				Saturation: getFloat64FromMap(effect.Parameters, "saturation", 1.0),
				Hue:        getFloat64FromMap(effect.Parameters, "hue", 0.0),
				Gamma:      getFloat64FromMap(effect.Parameters, "gamma", 1.0),
			}
			result, err = te.operations.ApplyColorCorrection(ctx, workingFile, colorOp, tempPath)

		default:
			// Unknown effect - skip
			continue
		}

		if err != nil {
			if workingFile != inputPath {
				os.Remove(workingFile)
			}
			return "", fmt.Errorf("failed to apply effect %s: %w", effect.Type, err)
		}

		// Clean up previous working file
		if workingFile != inputPath {
			os.Remove(workingFile)
		}

		workingFile = result
	}

	return workingFile, nil
}

// applyItemEffects applies effects to a single timeline item.
func (te *TimelineExecutor) applyItemEffects(ctx context.Context, inputPath string, effects []EffectConfig, outputPath string) (string, error) {
	// Similar to applyGlobalEffects but for a single item
	return te.applyGlobalEffects(ctx, inputPath, effects)
}

// applyItemTransitions applies in/out transitions to a clip.
func (te *TimelineExecutor) applyItemTransitions(ctx context.Context, inputPath string, transitions TransitionConfig, outputPath string) (string, error) {
	workingFile := inputPath

	// Apply fade in if specified
	if transitions.In != nil && transitions.In.Type == "fade" {
		tempPath := filepath.Join(te.tempDir, fmt.Sprintf("fade_in_%d.mp4", time.Now().UnixNano()))

		duration := transitions.In.Duration / 1000.0 // Convert to seconds
		cmd := NewFFmpegCommand(te.operations.FFmpegPath, te.operations.FFprobePath)
		cmd.AddInput(inputPath)
		cmd.AddArg("-vf").AddArg(fmt.Sprintf("fade=t=in:st=0:d=%.2f", duration))
		cmd.AddArg("-c:a").AddArg("copy")
		cmd.SetOutput(tempPath)

		execCmd := exec.CommandContext(ctx, te.operations.FFmpegPath, cmd.Build()...)
		if output, err := execCmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("failed to apply fade in: %w\nOutput: %s", err, string(output))
		}

		workingFile = tempPath
	}

	// Apply fade out if specified
	if transitions.Out != nil && transitions.Out.Type == "fade" {
		tempPath := filepath.Join(te.tempDir, fmt.Sprintf("fade_out_%d.mp4", time.Now().UnixNano()))

		// Get video duration
		info, err := GetMediaInfo(te.operations.FFprobePath, workingFile)
		if err != nil {
			if workingFile != inputPath {
				os.Remove(workingFile)
			}
			return "", fmt.Errorf("failed to get media info: %w", err)
		}

		duration := transitions.Out.Duration / 1000.0 // Convert to seconds
		startTime := info.Duration - duration

		cmd := NewFFmpegCommand(te.operations.FFmpegPath, te.operations.FFprobePath)
		cmd.AddInput(workingFile)
		cmd.AddArg("-vf").AddArg(fmt.Sprintf("fade=t=out:st=%.2f:d=%.2f", startTime, duration))
		cmd.AddArg("-c:a").AddArg("copy")
		cmd.SetOutput(tempPath)

		execCmd := exec.CommandContext(ctx, te.operations.FFmpegPath, cmd.Build()...)
		if output, err := execCmd.CombinedOutput(); err != nil {
			if workingFile != inputPath {
				os.Remove(workingFile)
			}
			return "", fmt.Errorf("failed to apply fade out: %w\nOutput: %s", err, string(output))
		}

		// Clean up previous working file
		if workingFile != inputPath {
			os.Remove(workingFile)
		}

		workingFile = tempPath
	}

	// If we created new files, move to output path
	if workingFile != inputPath {
		if err := os.Rename(workingFile, outputPath); err != nil {
			os.Remove(workingFile)
			return "", fmt.Errorf("failed to move file: %w", err)
		}
		return outputPath, nil
	}

	return inputPath, nil
}

// createBlankCanvas creates a blank video canvas.
func (te *TimelineExecutor) createBlankCanvas(ctx context.Context, canvas CanvasConfig, duration float64) (string, error) {
	outputPath := filepath.Join(te.tempDir, fmt.Sprintf("blank_%d.mp4", time.Now().UnixNano()))

	cmd := NewFFmpegCommand(te.operations.FFmpegPath, te.operations.FFprobePath)
	cmd.AddArg("-f").AddArg("lavfi")
	cmd.AddArg("-i").AddArg(fmt.Sprintf("color=c=black:s=%dx%d:d=%.2f:r=%d", canvas.Width, canvas.Height, duration, canvas.FPS))
	cmd.SetOutput(outputPath)

	execCmd := exec.CommandContext(ctx, te.operations.FFmpegPath, cmd.Build()...)
	if output, err := execCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("failed to create blank canvas: %w\nOutput: %s", err, string(output))
	}

	return outputPath, nil
}

// imageToVideo converts a static image to a video clip.
func (te *TimelineExecutor) imageToVideo(ctx context.Context, imagePath string, duration float64, canvas CanvasConfig, outputPath string) (string, error) {
	cmd := NewFFmpegCommand(te.operations.FFmpegPath, te.operations.FFprobePath)
	cmd.AddArg("-loop").AddArg("1")
	cmd.AddInput(imagePath)
	cmd.AddArg("-t").AddArg(fmt.Sprintf("%.2f", duration))
	cmd.AddArg("-vf").AddArg(fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:-1:-1:color=black", canvas.Width, canvas.Height, canvas.Width, canvas.Height))
	cmd.AddArg("-r").AddArg(fmt.Sprintf("%d", canvas.FPS))
	cmd.AddArg("-pix_fmt").AddArg("yuv420p")
	cmd.SetOutput(outputPath)

	execCmd := exec.CommandContext(ctx, te.operations.FFmpegPath, cmd.Build()...)
	if output, err := execCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("failed to convert image to video: %w\nOutput: %s", err, string(output))
	}

	return outputPath, nil
}

// getFloat64FromMap extracts a float64 value from a map with a default value
func getFloat64FromMap(params map[string]interface{}, key string, defaultValue float64) float64 {
	if val, ok := params[key]; ok {
		switch v := val.(type) {
		case float64:
			return v
		case float32:
			return float64(v)
		case int:
			return float64(v)
		case int64:
			return float64(v)
		}
	}
	return defaultValue
}
