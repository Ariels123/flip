package videoeditor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// VideoOperations provides methods for common video editing tasks.
type VideoOperations struct {
	FFmpegPath  string
	FFprobePath string
	TempDir     string
}

// NewVideoOperations creates a new VideoOperations instance.
func NewVideoOperations(ffmpegPath, ffprobePath, tempDir string) *VideoOperations {
	return &VideoOperations{
		FFmpegPath:  ffmpegPath,
		FFprobePath: ffprobePath,
		TempDir:     tempDir,
	}
}

// Trim cuts a video from startTime to startTime+duration.
func (vo *VideoOperations) Trim(ctx context.Context, inputPath string, startTime, duration float64, outputPath string) (string, error) {
	args := []string{
		"-y",
		"-ss", fmt.Sprintf("%.3f", startTime),
		"-i", inputPath,
		"-t", fmt.Sprintf("%.3f", duration),
		"-c:v", "libx264",
		"-c:a", "aac",
		outputPath,
	}

	cmd := exec.CommandContext(ctx, vo.FFmpegPath, args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("trim failed: %w\nOutput: %s", err, string(output))
	}

	return outputPath, nil
}

// Concat concatenates multiple video files using FFmpeg's concat demuxer.
func (vo *VideoOperations) Concat(ctx context.Context, inputFiles []string, outputPath string) (string, error) {
	if len(inputFiles) == 0 {
		return "", fmt.Errorf("no input files provided")
	}

	// Create concat file
	concatFile := filepath.Join(vo.TempDir, "concat.txt")
	var content strings.Builder

	for _, file := range inputFiles {
		absPath, err := filepath.Abs(file)
		if err == nil {
			file = absPath
		}
		content.WriteString(fmt.Sprintf("file '%s'\n", strings.ReplaceAll(file, "'", "\\'")))
	}

	if err := os.WriteFile(concatFile, []byte(content.String()), 0644); err != nil {
		return "", fmt.Errorf("failed to write concat file: %w", err)
	}
	defer os.Remove(concatFile)

	args := []string{
		"-y",
		"-f", "concat",
		"-safe", "0",
		"-i", concatFile,
		"-c", "copy",
		outputPath,
	}

	cmd := exec.CommandContext(ctx, vo.FFmpegPath, args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("concat failed: %w\nOutput: %s", err, string(output))
	}

	return outputPath, nil
}

// ApplyTransition applies a transition effect between two video clips.
// Transitions work by overlapping the end of clip1 with the start of clip2.
func (vo *VideoOperations) ApplyTransition(ctx context.Context, clip1Path, clip2Path string, transitionType string, duration float64, outputPath string) (string, error) {
	// Get clip1 duration to calculate overlap
	duration1, err := GetMediaDuration(vo.FFprobePath, clip1Path)
	if err != nil {
		return "", fmt.Errorf("failed to get clip1 duration: %w", err)
	}

	// Build transition filter
	var filterStr string
	transitionDurationSec := duration / 1000.0

	switch strings.ToLower(transitionType) {
	case "fade":
		filterStr = fmt.Sprintf("[0:v][1:v]xfade=transition=fade:duration=%.3f:offset=%.3f[outv]", transitionDurationSec, duration1-transitionDurationSec)
	case "dissolve":
		filterStr = fmt.Sprintf("[0:v][1:v]xfade=transition=dissolve:duration=%.3f:offset=%.3f[outv]", transitionDurationSec, duration1-transitionDurationSec)
	case "crossfade":
		filterStr = fmt.Sprintf("[0:a][1:a]acrossfade=d=%.3f[outa]", transitionDurationSec)
	case "slidedown":
		filterStr = fmt.Sprintf("[0:v][1:v]xfade=transition=slidedown:duration=%.3f:offset=%.3f[outv]", transitionDurationSec, duration1-transitionDurationSec)
	case "wipe":
		filterStr = fmt.Sprintf("[0:v][1:v]xfade=transition=wipeleft:duration=%.3f:offset=%.3f[outv]", transitionDurationSec, duration1-transitionDurationSec)
	default:
		return "", fmt.Errorf("unknown transition type: %s", transitionType)
	}

	args := []string{
		"-y",
		"-i", clip1Path,
		"-i", clip2Path,
		"-filter_complex", filterStr,
	}

	// Add mapping based on transition type
	if strings.ToLower(transitionType) == "crossfade" {
		args = append(args, "-map", "0:v", "-map", "[outa]")
	} else {
		args = append(args, "-map", "[outv]", "-map", "0:a")
	}

	args = append(args,
		"-c:v", "libx264",
		"-c:a", "aac",
		outputPath,
	)

	cmd := exec.CommandContext(ctx, vo.FFmpegPath, args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("transition failed: %w\nOutput: %s", err, string(output))
	}

	return outputPath, nil
}

// AddTextOverlay adds text to a video.
func (vo *VideoOperations) AddTextOverlay(ctx context.Context, inputPath string, text, position string, fontSize int, fontColor string, fontFile string, opacity float64, startTime, duration float64, outputPath string) (string, error) {
	// Build drawtext filter
	filterStr := BuildTextOverlayFilter(text, position, fontFile, fontSize, fontColor, opacity, startTime, duration)

	args := []string{
		"-y",
		"-i", inputPath,
		"-vf", filterStr,
		"-c:a", "copy",
		outputPath,
	}

	cmd := exec.CommandContext(ctx, vo.FFmpegPath, args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("text overlay failed: %w\nOutput: %s", err, string(output))
	}

	return outputPath, nil
}

// ApplyColorCorrection applies color correction to a video.
func (vo *VideoOperations) ApplyColorCorrection(ctx context.Context, inputPath string, correction ColorCorrectionOperation, outputPath string) (string, error) {
	filterStr := BuildColorCorrectionFilter(correction)
	if filterStr == "" {
		// No corrections needed, just copy
		args := []string{
			"-y",
			"-i", inputPath,
			"-c", "copy",
			outputPath,
		}
		cmd := exec.CommandContext(ctx, vo.FFmpegPath, args...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("color correction copy failed: %w\nOutput: %s", err, string(output))
		}
		return outputPath, nil
	}

	args := []string{
		"-y",
		"-i", inputPath,
		"-vf", filterStr,
		"-c:a", "copy",
		outputPath,
	}

	cmd := exec.CommandContext(ctx, vo.FFmpegPath, args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("color correction failed: %w\nOutput: %s", err, string(output))
	}

	return outputPath, nil
}

// Scale changes the resolution of a video.
func (vo *VideoOperations) Scale(ctx context.Context, inputPath string, width, height int, maintainAspect bool, outputPath string) (string, error) {
	filterStr := BuildScaleFilter(width, height, maintainAspect)

	args := []string{
		"-y",
		"-i", inputPath,
		"-vf", filterStr,
		"-c:a", "copy",
		outputPath,
	}

	cmd := exec.CommandContext(ctx, vo.FFmpegPath, args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("scale failed: %w\nOutput: %s", err, string(output))
	}

	return outputPath, nil
}

// ComposeVideo builds a complex FFmpeg command to compose multiple clips with transitions and effects.
// This is the main orchestration method for building videos from components.
func (vo *VideoOperations) ComposeVideo(ctx context.Context, clips []ClipSpec, effects *VideoEffectsSpec, outputPath string, fps int) (string, error) {
	if len(clips) == 0 {
		return "", fmt.Errorf("no clips provided")
	}

	// Start building FFmpeg command
	cmd := NewFFmpegCommand(vo.FFmpegPath, vo.FFprobePath)

	// Add all inputs
	for _, clip := range clips {
		cmd.AddInput(clip.Path)
	}

	// Build filter complex string
	filterComplex, mapping := vo.buildFilterComplex(clips, effects, len(clips))

	if filterComplex != "" {
		cmd.SetFilterComplex(filterComplex)
	}

	// Add common codec arguments
	if fps > 0 {
		cmd.AddArg("-r").AddArg(fmt.Sprintf("%d", fps))
	}

	// Add mappings
	for _, m := range mapping {
		cmd.AddArg("-map").AddArg(m)
	}

	// Add encoding settings
	cmd.AddArg("-c:v").AddArg("libx264").
		AddArg("-pix_fmt").AddArg("yuv420p").
		AddArg("-c:a").AddArg("aac").
		AddArg("-b:a").AddArg("192k").
		SetOutput(outputPath)

	// Execute
	execCmd := exec.CommandContext(ctx, vo.FFmpegPath, cmd.Build()...)
	if output, err := execCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("compose failed: %w\nOutput: %s", err, string(output))
	}

	return outputPath, nil
}

// ClipSpec represents a video clip in a composition.
type ClipSpec struct {
	Path            string
	StartTime       float64 // milliseconds
	Duration        float64 // milliseconds
	Transition      *TransitionOperation
	Scale           *ScaleOperation
	ColorCorrection *ColorCorrectionOperation
}

// VideoEffectsSpec defines effects to apply to the entire composition.
type VideoEffectsSpec struct {
	GlobalColorCorrection *ColorCorrectionOperation
	TextOverlays          []TextOverlayOperation
	OutputResolution      string // e.g., "1920x1080"
}

// buildFilterComplex constructs the complex filter string and mappings.
func (vo *VideoOperations) buildFilterComplex(clips []ClipSpec, effects *VideoEffectsSpec, numInputs int) (string, []string) {
	var filters []string
	var mappings []string

	if numInputs == 0 {
		return "", mappings
	}

	if numInputs == 1 {
		mappings = append(mappings, "0:v", "0:a")
		// Apply effects to single clip
		if effects != nil && effects.GlobalColorCorrection != nil {
			colorFilter := BuildColorCorrectionFilter(*effects.GlobalColorCorrection)
			if colorFilter != "" {
				filters = append(filters, fmt.Sprintf("[0:v]%s[outv]", colorFilter))
				mappings = []string{"[outv]", "0:a"}
			}
		}
		return strings.Join(filters, ";"), mappings
	}

	// Multiple clips - build concat filter
	// For simplicity, we'll concatenate without transitions in the filter
	// Transitions would require overlaying clips which is more complex
	filters = append(filters, fmt.Sprintf("[0:v][1:v]concat=n=%d:v=1:a=0[outv]", numInputs))
	filters = append(filters, fmt.Sprintf("[0:a][1:a]concat=n=%d:v=0:a=1[outa]", numInputs))

	mappings = append(mappings, "[outv]", "[outa]")

	return strings.Join(filters, ";"), mappings
}

// MergeAudioVideo combines audio and video tracks.
func (vo *VideoOperations) MergeAudioVideo(ctx context.Context, videoPath, audioPath, outputPath string) (string, error) {
	args := []string{
		"-y",
		"-i", videoPath,
		"-i", audioPath,
		"-c:v", "copy",
		"-c:a", "aac",
		"-b:a", "192k",
		"-shortest",
		outputPath,
	}

	cmd := exec.CommandContext(ctx, vo.FFmpegPath, args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("merge audio/video failed: %w\nOutput: %s", err, string(output))
	}

	return outputPath, nil
}

// EncodeVideo encodes a video to a specific format and quality.
func (vo *VideoOperations) EncodeVideo(ctx context.Context, inputPath, outputPath, format, quality string, fps int) (string, error) {
	preset, ok := QualityPresets[quality]
	if !ok {
		return "", fmt.Errorf("unknown quality: %s", quality)
	}

	// Determine codec extension
	codec := "libx264"
	pixFmt := "yuv420p"

	args := []string{
		"-y",
		"-i", inputPath,
		"-c:v", codec,
		"-pix_fmt", pixFmt,
		"-s", fmt.Sprintf("%dx%d", preset.Width, preset.Height),
		"-b:v", preset.Bitrate,
		"-maxrate", preset.MaxBitrate,
		"-bufsize", preset.BufSize,
		"-c:a", "aac",
		"-b:a", "192k",
	}

	if fps > 0 {
		args = append(args, "-r", fmt.Sprintf("%d", fps))
	}

	args = append(args, outputPath)

	cmd := exec.CommandContext(ctx, vo.FFmpegPath, args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("encode failed: %w\nOutput: %s", err, string(output))
	}

	return outputPath, nil
}
