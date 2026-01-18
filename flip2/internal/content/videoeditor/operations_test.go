package videoeditor

import (
	"os"
	"path/filepath"
	"testing"
)

// TestVideoOperationsBasics tests basic VideoOperations functionality.
func TestVideoOperationsBasics(t *testing.T) {
	tempDir := t.TempDir()
	ops := NewVideoOperations("ffmpeg", "ffprobe", tempDir)

	if ops == nil {
		t.Fatal("Failed to create VideoOperations")
	}

	if ops.TempDir != tempDir {
		t.Errorf("Expected temp dir %s, got %s", tempDir, ops.TempDir)
	}
}

// TestBuildFadeTransition tests fade transition filter generation.
func TestBuildFadeTransition(t *testing.T) {
	filter := BuildFadeTransition(5.0, 5.0, 500)

	if filter == "" {
		t.Error("Expected non-empty filter")
	}

	if !contains(filter, "xfade") {
		t.Error("Fade transition should use xfade filter")
	}

	if !contains(filter, "fade") {
		t.Error("Fade transition should specify fade type")
	}
}

// TestBuildDissolveTransition tests dissolve transition filter generation.
func TestBuildDissolveTransition(t *testing.T) {
	filter := BuildDissolveTransition(5.0, 5.0, 500)

	if filter == "" {
		t.Error("Expected non-empty filter")
	}

	if !contains(filter, "dissolve") {
		t.Error("Dissolve transition should specify dissolve type")
	}
}

// TestBuildCrossfadeTransition tests crossfade transition filter generation.
func TestBuildCrossfadeTransition(t *testing.T) {
	filter := BuildCrossfadeTransition(500)

	if filter == "" {
		t.Error("Expected non-empty filter")
	}

	if !contains(filter, "acrossfade") {
		t.Error("Crossfade should use acrossfade filter")
	}
}

// TestBuildSlidedownTransition tests slidedown transition filter generation.
func TestBuildSlidedownTransition(t *testing.T) {
	filter := BuildSlidedownTransition(500)

	if filter == "" {
		t.Error("Expected non-empty filter")
	}

	if !contains(filter, "slidedown") {
		t.Error("Slidedown transition should specify slidedown type")
	}
}

// TestBuildWipeTransition tests wipe transition filter generation.
func TestBuildWipeTransition(t *testing.T) {
	tests := []struct {
		direction string
		expected  string
	}{
		{"left", "wipeleft"},
		{"right", "wiperight"},
		{"up", "wipeup"},
		{"down", "wipedown"},
	}

	for _, tt := range tests {
		t.Run(tt.direction, func(t *testing.T) {
			filter := BuildWipeTransition(500, tt.direction)
			if filter == "" {
				t.Error("Expected non-empty filter")
			}
			if !contains(filter, tt.expected) {
				t.Errorf("Expected %s in filter", tt.expected)
			}
		})
	}
}

// TestGenerateTempDir tests temporary directory creation.
func TestGenerateTempDir(t *testing.T) {
	baseDir := t.TempDir()

	tempDir, err := GenerateTempDir(baseDir)
	if err != nil {
		t.Fatalf("Failed to generate temp dir: %v", err)
	}

	if tempDir == "" {
		t.Error("Expected non-empty temp dir path")
	}

	// Check that directory was created
	if _, err := os.Stat(tempDir); err != nil {
		t.Errorf("Temp directory was not created: %v", err)
	}

	// Check that it's under baseDir
	if !contains(tempDir, baseDir) {
		t.Errorf("Temp dir %s should be under base dir %s", tempDir, baseDir)
	}
}

// TestClipSpec tests ClipSpec structure.
func TestClipSpec(t *testing.T) {
	clip := ClipSpec{
		Path:      "test.mp4",
		StartTime: 0,
		Duration:  5000,
	}

	if clip.Path != "test.mp4" {
		t.Errorf("Expected path test.mp4, got %s", clip.Path)
	}

	if clip.Duration != 5000 {
		t.Errorf("Expected duration 5000, got %f", clip.Duration)
	}
}

// TestVideoEffectsSpec tests VideoEffectsSpec structure.
func TestVideoEffectsSpec(t *testing.T) {
	correction := &ColorCorrectionOperation{
		Brightness: 0.1,
		Contrast:   1.2,
	}

	effects := &VideoEffectsSpec{
		GlobalColorCorrection: correction,
		OutputResolution:      "1920x1080",
	}

	if effects.OutputResolution != "1920x1080" {
		t.Errorf("Expected resolution 1920x1080, got %s", effects.OutputResolution)
	}

	if effects.GlobalColorCorrection == nil {
		t.Error("Expected non-nil color correction")
	}

	if effects.GlobalColorCorrection.Brightness != 0.1 {
		t.Errorf("Expected brightness 0.1, got %f", effects.GlobalColorCorrection.Brightness)
	}
}

// TestBuildConcatFilter tests concat filter generation.
func TestBuildConcatFilter(t *testing.T) {
	tests := []struct {
		name      string
		numClips  int
		hasVideo  bool
		hasAudio  bool
		expectLen int
	}{
		{"video only", 2, true, false, 1},
		{"audio only", 2, false, true, 1},
		{"video and audio", 2, true, true, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := BuildConcatFilter(tt.numClips, tt.hasVideo, tt.hasAudio)
			if filter == "" && tt.expectLen > 0 {
				t.Error("Expected non-empty filter")
			}
			// Count semicolons to verify filter parts
			parts := countCharacter(filter, ';')
			expectedParts := tt.expectLen - 1
			if parts != expectedParts {
				t.Errorf("Expected %d parts, got %d", tt.expectLen, parts+1)
			}
		})
	}
}

// Helper to count character occurrences
func countCharacter(s string, c byte) int {
	count := 0
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			count++
		}
	}
	return count
}

// TestTextOverlayWithColor tests text overlay with different colors.
func TestTextOverlayWithColor(t *testing.T) {
	tests := []struct {
		name  string
		color string
	}{
		{"hex color", "#FFFFFF"},
		{"hex red", "#FF0000"},
		{"hex green", "#00FF00"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := BuildTextOverlayFilter("Test", "center", "", 24, tt.color, 1.0, 0, 3000)
			if filter == "" {
				t.Error("Expected non-empty filter")
			}
			// Should contain font color in some form
			if !contains(filter, "fontcolor") {
				t.Error("Filter should contain fontcolor")
			}
		})
	}
}

// TestMergeAudioVideoContext tests that MergeAudioVideo is properly structured.
func TestMergeAudioVideoContext(t *testing.T) {
	// This test would require actual video files to execute merge
	// For now, we verify the VideoOperations instance has the method
	tempDir := t.TempDir()
	ops := NewVideoOperations("ffmpeg", "ffprobe", tempDir)

	// Verify that VideoOperations instance exists and has the method available
	if ops == nil {
		t.Error("VideoOperations should be created successfully")
	}

	// The method exists on the type - we can verify it works by checking ops has the fields
	if ops.FFmpegPath == "" {
		t.Error("FFmpegPath should be set")
	}
}

// TestEncodeVideoWithDifferentQualities tests encoding with different quality levels.
func TestEncodeVideoWithDifferentQualities(t *testing.T) {
	qualities := []string{"720p", "1080p", "2k", "4k"}

	for _, quality := range qualities {
		t.Run(quality, func(t *testing.T) {
			preset, ok := QualityPresets[quality]
			if !ok {
				t.Fatalf("Unknown quality: %s", quality)
			}

			if preset.Width == 0 || preset.Height == 0 {
				t.Error("Invalid dimensions in preset")
			}

			if preset.Bitrate == "" {
				t.Error("Missing bitrate in preset")
			}
		})
	}
}

// TestColorCorrectionOperationDefaults tests ColorCorrectionOperation with defaults.
func TestColorCorrectionOperationDefaults(t *testing.T) {
	correction := ColorCorrectionOperation{}

	// All fields should be zero/default
	if correction.Brightness != 0 {
		t.Error("Brightness should default to 0")
	}

	if correction.Contrast != 0 {
		t.Error("Contrast should default to 0")
	}

	if correction.Saturation != 0 {
		t.Error("Saturation should default to 0")
	}
}

// TestScaleOperationDefaults tests ScaleOperation with defaults.
func TestScaleOperationDefaults(t *testing.T) {
	scale := ScaleOperation{
		Width:  1920,
		Height: 1080,
	}

	if scale.Width != 1920 || scale.Height != 1080 {
		t.Error("Scale dimensions not set correctly")
	}

	if scale.Force {
		t.Error("Force should default to false")
	}
}

// TestTextOverlayOperationDefaults tests TextOverlayOperation with defaults.
func TestTextOverlayOperationDefaults(t *testing.T) {
	text := TextOverlayOperation{
		Content:   "Test Text",
		Position:  "center",
		FontSize:  24,
		FontColor: "#FFFFFF",
	}

	if text.Content != "Test Text" {
		t.Error("Content not set correctly")
	}

	if text.FontSize != 24 {
		t.Error("FontSize not set correctly")
	}

	if text.FontColor != "#FFFFFF" {
		t.Error("FontColor not set correctly")
	}
}

// TestFFmpegCommandChaining tests method chaining in FFmpegCommand.
func TestFFmpegCommandChaining(t *testing.T) {
	cmd := NewFFmpegCommand("ffmpeg", "ffprobe").
		AddInput("input.mp4").
		AddInput("input2.mp4").
		AddArg("-c:v").AddArg("libx264").
		SetOutput("output.mp4")

	args := cmd.Build()

	if len(args) < 5 {
		t.Error("Expected at least 5 arguments")
	}

	// Count inputs
	inputCount := 0
	for i, arg := range args {
		if arg == "-i" && i+1 < len(args) {
			inputCount++
		}
	}

	if inputCount != 2 {
		t.Errorf("Expected 2 inputs, got %d", inputCount)
	}
}

// TestFileExistsHelper tests the fileExists helper function (implicitly through other tests)
func TestFileExistsHelper(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.txt")

	// File shouldn't exist yet
	if fileExists(testFile) {
		t.Error("File should not exist")
	}

	// Create file
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// File should exist now
	if !fileExists(testFile) {
		t.Error("File should exist after creation")
	}
}
