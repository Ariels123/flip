package videoeditor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// FFmpegCommand represents a composable FFmpeg command.
type FFmpegCommand struct {
	FFmpegPath    string   // Path to ffmpeg binary
	FFprobePath   string   // Path to ffprobe binary
	Inputs        []string // Input files
	FilterComplex string   // Complex filter string
	Args          []string // Additional FFmpeg arguments
	OutputPath    string   // Output file path
}

// NewFFmpegCommand creates a new FFmpeg command builder.
func NewFFmpegCommand(ffmpegPath, ffprobePath string) *FFmpegCommand {
	return &FFmpegCommand{
		FFmpegPath:  ffmpegPath,
		FFprobePath: ffprobePath,
		Inputs:      []string{},
		Args:        []string{},
	}
}

// AddInput adds an input file to the command.
func (fc *FFmpegCommand) AddInput(filePath string) *FFmpegCommand {
	fc.Inputs = append(fc.Inputs, filePath)
	return fc
}

// SetOutput sets the output file path.
func (fc *FFmpegCommand) SetOutput(outputPath string) *FFmpegCommand {
	fc.OutputPath = outputPath
	return fc
}

// AddArg adds a raw FFmpeg argument.
func (fc *FFmpegCommand) AddArg(arg string) *FFmpegCommand {
	fc.Args = append(fc.Args, arg)
	return fc
}

// SetFilterComplex sets the complex filter string.
func (fc *FFmpegCommand) SetFilterComplex(filter string) *FFmpegCommand {
	fc.FilterComplex = filter
	return fc
}

// Build constructs the final FFmpeg command line arguments.
func (fc *FFmpegCommand) Build() []string {
	args := []string{"-y"} // Overwrite output files without asking

	// Add inputs
	for _, input := range fc.Inputs {
		args = append(args, "-i", input)
	}

	// Add filter complex if present
	if fc.FilterComplex != "" {
		args = append(args, "-filter_complex", fc.FilterComplex)
	}

	// Add additional arguments
	args = append(args, fc.Args...)

	// Add output
	if fc.OutputPath != "" {
		args = append(args, fc.OutputPath)
	}

	return args
}

// BuildFadeTransition creates an xfade filter for fade transitions.
func BuildFadeTransition(clip1Duration, clip2StartTime, transitionDuration float64) string {
	// Convert to ms for xfade filter
	offset := clip1Duration - (transitionDuration / 1000.0)
	return fmt.Sprintf("xfade=transition=fade:duration=%.3f:offset=%.3f", transitionDuration/1000.0, offset)
}

// BuildDissolveTransition creates an xfade filter for dissolve transitions.
func BuildDissolveTransition(clip1Duration, clip2StartTime, transitionDuration float64) string {
	offset := clip1Duration - (transitionDuration / 1000.0)
	return fmt.Sprintf("xfade=transition=dissolve:duration=%.3f:offset=%.3f", transitionDuration/1000.0, offset)
}

// BuildCrossfadeTransition creates a crossfade between audio clips.
func BuildCrossfadeTransition(duration float64) string {
	return fmt.Sprintf("acrossfade=d=%.3f", duration/1000.0)
}

// BuildSlidedownTransition creates a slidedown transition.
func BuildSlidedownTransition(duration float64) string {
	return fmt.Sprintf("xfade=transition=slidedown:duration=%.3f", duration/1000.0)
}

// BuildWipeTransition creates a wipe transition.
func BuildWipeTransition(duration float64, direction string) string {
	transition := "wipeleft"
	if direction == "right" {
		transition = "wiperight"
	} else if direction == "up" {
		transition = "wipeup"
	} else if direction == "down" {
		transition = "wipedown"
	}
	return fmt.Sprintf("xfade=transition=%s:duration=%.3f", transition, duration/1000.0)
}

// BuildTextOverlayFilter creates a drawtext filter for text overlays.
func BuildTextOverlayFilter(text, position string, fontFile string, fontSize int, fontColor string, opacity float64, startTime, duration float64) string {
	// Escape special characters in text
	escapedText := strings.ReplaceAll(text, "'", "\\'")
	escapedText = strings.ReplaceAll(escapedText, ":", "\\:")

	// Convert colors from hex to FFmpeg format if needed
	if strings.HasPrefix(fontColor, "#") {
		// Convert #RRGGBB to 0xRRGGBB
		fontColor = "0x" + fontColor[1:]
	}

	// Build drawtext filter
	filter := fmt.Sprintf("drawtext=text='%s':fontsize=%d:fontcolor=%s", escapedText, fontSize, fontColor)

	// Add position if specified
	if pos, ok := TextPositions[position]; ok {
		filter += ":" + pos
	}

	// Add opacity via fade effect if less than 1.0
	if opacity < 1.0 {
		filter += fmt.Sprintf(":alpha=%.2f", opacity)
	}

	// Add font if specified
	if fontFile != "" && fileExists(fontFile) {
		escapedFont := strings.ReplaceAll(fontFile, ":", "\\:")
		escapedFont = strings.ReplaceAll(escapedFont, "'", "\\'")
		filter += ":fontfile='" + escapedFont + "'"
	}

	// Add timing
	startSec := startTime / 1000.0
	endSec := (startTime + duration) / 1000.0
	filter += fmt.Sprintf(":enable='between(t,%.3f,%.3f)'", startSec, endSec)

	return filter
}

// BuildColorCorrectionFilter creates color correction filters.
func BuildColorCorrectionFilter(correction ColorCorrectionOperation) string {
	var filters []string

	// Brightness and contrast
	if correction.Brightness != 0 || correction.Contrast != 0 {
		br := 0.5 + correction.Brightness
		if br < 0.1 {
			br = 0.1
		}
		if br > 1.9 {
			br = 1.9
		}
		ct := correction.Contrast
		if ct < 0.5 {
			ct = 0.5
		}
		if ct > 2.0 {
			ct = 2.0
		}
		filters = append(filters, fmt.Sprintf("brightness=%.2f:contrast=%.2f", br, ct))
	}

	// Saturation
	if correction.Saturation != 0 && correction.Saturation != 1.0 {
		filters = append(filters, fmt.Sprintf("saturation=%.2f", correction.Saturation))
	}

	// Hue
	if correction.Hue != 0 {
		filters = append(filters, fmt.Sprintf("hue=h=%.1f", correction.Hue))
	}

	// Gamma
	if correction.Gamma != 0 && correction.Gamma != 1.0 {
		filters = append(filters, fmt.Sprintf("scale=in_color_matrix=bt709:out_color_matrix=bt709,eq=gamma=%.2f", correction.Gamma))
	}

	return strings.Join(filters, ",")
}

// BuildScaleFilter creates a scale filter for resolution changes.
func BuildScaleFilter(width, height int, maintainAspect bool) string {
	w := width
	h := height

	if maintainAspect {
		if w == -1 {
			return fmt.Sprintf("scale=-1:%d", h)
		}
		if h == -1 {
			return fmt.Sprintf("scale:%d:-1", w)
		}
		return fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease", w, h)
	}

	return fmt.Sprintf("scale=%d:%d", w, h)
}

// BuildConcatFilter creates a concat demuxer filter string for multiple clips.
func BuildConcatFilter(numClips int, hasVideo, hasAudio bool) string {
	var parts []string

	if hasVideo {
		parts = append(parts, fmt.Sprintf("[0:v][1:v]concat=n=%d:v=1:a=0[outv]", numClips))
	}

	if hasAudio {
		if hasVideo {
			parts = append(parts, fmt.Sprintf("[0:a][1:a]concat=n=%d:v=0:a=1[outa]", numClips))
		} else {
			parts = append(parts, fmt.Sprintf("[0:a][1:a]concat=n=%d:v=0:a=1[outa]", numClips))
		}
	}

	return strings.Join(parts, ";")
}

// GetMediaDuration retrieves the duration of a media file in seconds using ffprobe.
func GetMediaDuration(ffprobePath, filePath string) (float64, error) {
	args := []string{
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		filePath,
	}

	cmd := exec.Command(ffprobePath, args...)
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("ffprobe failed: %w", err)
	}

	var duration float64
	_, err = fmt.Sscanf(strings.TrimSpace(string(output)), "%f", &duration)
	if err != nil {
		return 0, fmt.Errorf("failed to parse duration: %w", err)
	}

	return duration, nil
}

// GetMediaInfo retrieves detailed information about a media file.
type MediaInfo struct {
	Duration  float64
	Width     int
	Height    int
	FPS       float64
	BitRate   string
	Codec     string
	FrameRate string
}

// GetMediaInfo retrieves information about a media file.
func GetMediaInfo(ffprobePath, filePath string) (*MediaInfo, error) {
	// Get duration
	args := []string{
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height,r_frame_rate,codec_name",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1",
		filePath,
	}

	cmd := exec.Command(ffprobePath, args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe failed: %w", err)
	}

	info := &MediaInfo{}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")

	for _, line := range lines {
		parts := strings.Split(line, "=")
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "duration":
			fmt.Sscanf(value, "%f", &info.Duration)
		case "width":
			fmt.Sscanf(value, "%d", &info.Width)
		case "height":
			fmt.Sscanf(value, "%d", &info.Height)
		case "codec_name":
			info.Codec = value
		case "r_frame_rate":
			info.FrameRate = value
		}
	}

	return info, nil
}

// GenerateTempDir creates a temporary directory with a timestamp.
func GenerateTempDir(baseDir string) (string, error) {
	tempDir := filepath.Join(baseDir, fmt.Sprintf("videoeditor_%d", time.Now().UnixNano()))
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}
	return tempDir, nil
}

// fileExists checks if a file exists at the given path.
func fileExists(filePath string) bool {
	_, err := os.Stat(filePath)
	return err == nil
}

// Note: exec.Command is used directly from os/exec package in the implementation.
// This is sufficient for the current use case.
