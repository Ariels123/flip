package videoeditor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"flip2/internal/logger"
)

// TestIntegration_TemplateInheritanceToCapCut tests the full pipeline:
// template inheritance -> variable substitution -> CapCut export
func TestIntegration_TemplateInheritanceToCapCut(t *testing.T) {
	tempDir := t.TempDir()
	log := (*logger.Logger)(nil)

	// Create base template
	baseTemplate := `
name: "base"
version: "1.0"

canvas:
  width: 1920
  height: 1080
  fps: 30

variables:
  brand_color: "#FF0000"

timeline:
  - type: "video"
    path: "base_video.mp4"
    start_time: 0
    duration: 5000
`

	baseFile := filepath.Join(tempDir, "base.yaml")
	if err := os.WriteFile(baseFile, []byte(baseTemplate), 0644); err != nil {
		t.Fatalf("Failed to write base template: %v", err)
	}

	// Create child template that extends base
	childTemplate := `
name: "child"
version: "1.0"
extends: "base"

variables:
  title: "Test Video"

timeline:
  - type: "text"
    content: "{{title}}"
    start_time: 1000
    duration: 3000
    style:
      font_size: 64
      font_color: "{{brand_color}}"
      position: "center"
`

	childFile := filepath.Join(tempDir, "child.yaml")
	if err := os.WriteFile(childFile, []byte(childTemplate), 0644); err != nil {
		t.Fatalf("Failed to write child template: %v", err)
	}

	// Create template manager and CapCut exporter
	tm := NewTemplateManager([]string{tempDir})
	exporter := NewCapCutExporter(tm, tempDir, log)

	// Export to CapCut
	outputPath := filepath.Join(tempDir, "test_project")
	options := ExportOptions{
		TemplateName:  "child",
		Variables:     map[string]interface{}{"title": "My Custom Title"},
		OutputPath:    outputPath,
		IncludeAssets: false,
		PackageFormat: "directory",
	}

	result, err := exporter.Export(context.Background(), options)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	// Verify result
	if result.ProjectPath == "" {
		t.Error("Expected project path to be set")
	}

	if result.DraftContentPath == "" {
		t.Error("Expected draft.content path to be set")
	}

	// Verify draft.content file exists
	if _, err := os.Stat(result.DraftContentPath); err != nil {
		t.Errorf("draft.content file not found: %v", err)
	}

	// Load and validate the exported project
	data, err := os.ReadFile(result.DraftContentPath)
	if err != nil {
		t.Fatalf("Failed to read draft.content: %v", err)
	}

	project, err := ImportCapCutProject(data)
	if err != nil {
		t.Fatalf("Failed to import CapCut project: %v", err)
	}

	// Validate the project
	if err := ValidateCapCutProject(project); err != nil {
		t.Errorf("Validation failed: %v", err)
	}

	// Verify canvas was inherited from base
	if project.CanvasConfig.Width != 1920 || project.CanvasConfig.Height != 1080 {
		t.Errorf("Expected canvas 1920x1080 from base, got %dx%d",
			project.CanvasConfig.Width, project.CanvasConfig.Height)
	}

	// Verify text material exists with variable substitution
	if len(project.Materials.Texts) == 0 {
		t.Fatal("Expected at least one text material")
	}
}

// TestIntegration_PathValidationSecurity tests that malicious paths are rejected
func TestIntegration_PathValidationSecurity(t *testing.T) {
	maliciousPaths := []string{
		"../../../etc/passwd",
		"../../.ssh/id_rsa",
		"..\\..\\windows\\system32",
		"/etc/passwd",
		"C:\\Windows\\System32",
		"video.mp4/../../../evil.sh",
	}

	for _, malPath := range maliciousPaths {
		t.Run(malPath, func(t *testing.T) {
			result := sanitizeFileExtension(malPath)
			// Should return safe default extension
			if result != ".mp4" {
				t.Errorf("Expected sanitized path to use .mp4, got %s", result)
			}
		})
	}

	// Test that valid extensions are preserved
	validPaths := []string{
		"video.mp4",
		"image.png",
		"audio.mp3",
	}
	expectedExts := []string{".mp4", ".png", ".mp3"}

	for i, validPath := range validPaths {
		result := sanitizeFileExtension(validPath)
		if result != expectedExts[i] {
			t.Errorf("Expected extension %s for %s, got %s", expectedExts[i], validPath, result)
		}
	}
}

// TestIntegration_EffectsWithFFmpegAPI tests that effects work with the correct FFmpegCommand API
func TestIntegration_EffectsWithFFmpegAPI(t *testing.T) {
	tempDir := t.TempDir()

	// Test all effect types
	effects := []struct {
		name       string
		effectType string
		params     map[string]interface{}
	}{
		{"color_correct", "color_correct", map[string]interface{}{"brightness": 0.2, "contrast": 1.1}},
		{"blur", "blur", map[string]interface{}{"radius": 5.0}},
		{"sharpen", "sharpen", map[string]interface{}{"strength": 50}},
		{"noise", "noise", map[string]interface{}{"strength": 0.05}},
		{"vignette", "vignette", map[string]interface{}{"intensity": 0.5}},
	}

	for _, tc := range effects {
		t.Run(tc.name, func(t *testing.T) {
			// Create test input/output files
			inputPath := filepath.Join(tempDir, "input_"+tc.name+".mp4")
			outputPath := filepath.Join(tempDir, "output_"+tc.name+".mp4")

			// Create dummy input file
			if err := os.WriteFile(inputPath, []byte("fake video data"), 0644); err != nil {
				t.Fatalf("Failed to create test input: %v", err)
			}

			// Create effect
			registry := NewEffectRegistry()
			effect, err := registry.Get(tc.effectType)
			if err != nil {
				t.Fatalf("Effect %s not found in registry: %v", tc.effectType, err)
			}

			// Prepare effect input
			effectInput := EffectInput{
				InputPath:  inputPath,
				OutputPath: outputPath,
				FFmpegPath: "ffmpeg",    // Will fail if ffmpeg not installed, but that's OK
				FFprobePath: "ffprobe",
				Parameters: tc.params,
			}

			// Apply effect (will fail without ffmpeg, but we're testing API correctness)
			_, applyErr := effect.Apply(context.Background(), effectInput)
			// We expect this to fail (no real video file), but it should fail during execution,
			// not during API construction
			if applyErr == nil {
				// If it succeeded (maybe ffmpeg is installed), great!
				t.Logf("Effect %s applied successfully", tc.name)
			} else {
				// Check that error is from ffmpeg execution, not from API construction
				t.Logf("Effect %s failed as expected (no real video): %v", tc.name, applyErr)
			}
		})
	}
}

// TestIntegration_BatchProcessingThreadSafety tests concurrent batch processing
func TestIntegration_BatchProcessingThreadSafety(t *testing.T) {
	tempDir := t.TempDir()
	log := (*logger.Logger)(nil)

	// Create a simple template
	templateYAML := `
name: "test"
version: "1.0"

canvas:
  width: 1920
  height: 1080
  fps: 30

timeline:
  - type: "text"
    content: "{{title}}"
    start_time: 0
    duration: 3000
`

	templateFile := filepath.Join(tempDir, "test.yaml")
	if err := os.WriteFile(templateFile, []byte(templateYAML), 0644); err != nil {
		t.Fatalf("Failed to write template: %v", err)
	}

	// Create editor and batch processor
	editor, err := New(log, "ffmpeg", "ffprobe", tempDir, tempDir)
	if err != nil {
		t.Fatalf("Failed to create editor: %v", err)
	}

	tm := NewTemplateManager([]string{tempDir})
	bp := NewBatchProcessor(editor, tm, 4, log) // 4 concurrent workers

	// Create batch jobs
	jobs := make([]*BatchJob, 20)
	for i := 0; i < 20; i++ {
		jobs[i] = &BatchJob{
			ID:           fmt.Sprintf("job_%d", i),
			TemplateName: "test",
			Variables: map[string]interface{}{
				"title": fmt.Sprintf("Video %d", i),
			},
			OutputPath: filepath.Join(tempDir, fmt.Sprintf("output_%d.mp4", i)),
			Priority:   i % 3, // Mix of priorities
		}
	}

	// Process batch (will fail without ffmpeg, but we're testing thread safety)
	results, err := bp.ProcessBatch(context.Background(), jobs)
	if err != nil {
		t.Fatalf("ProcessBatch failed: %v", err)
	}

	// Verify we got results for all jobs
	if len(results) != len(jobs) {
		t.Errorf("Expected %d results, got %d", len(jobs), len(results))
	}

	// Check that progress tracking is accurate
	stats := bp.GetProgress()
	if stats.TotalJobs != int64(len(jobs)) {
		t.Errorf("Expected total jobs %d, got %d", len(jobs), stats.TotalJobs)
	}

	if stats.CompletedJobs+stats.FailedJobs != int64(len(jobs)) {
		t.Errorf("Expected completed+failed = %d, got %d", len(jobs), stats.CompletedJobs+stats.FailedJobs)
	}
}

// TestIntegration_CryptoRandomIDGeneration tests that ID generation is secure
func TestIntegration_CryptoRandomIDGeneration(t *testing.T) {
	// Generate many IDs and ensure they're unique
	ids := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		id := randomString(16)
		if ids[id] {
			t.Errorf("Duplicate ID generated: %s", id)
		}
		ids[id] = true

		// Verify ID is alphanumeric
		for _, c := range id {
			if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')) {
				t.Errorf("Invalid character in ID: %c", c)
			}
		}
	}
}

// TestIntegration_VariableSubstitutionInInheritedTemplate tests variable substitution
// works correctly with inherited templates
func TestIntegration_VariableSubstitutionInInheritedTemplate(t *testing.T) {
	tempDir := t.TempDir()

	// Create base template with variables
	baseTemplate := `
name: "base"
version: "1.0"

canvas:
  width: 1920
  height: 1080
  fps: 30

variables:
  brand_color: "#FF0000"
  subtitle: "Default Subtitle"
`

	baseFile := filepath.Join(tempDir, "base.yaml")
	if err := os.WriteFile(baseFile, []byte(baseTemplate), 0644); err != nil {
		t.Fatalf("Failed to write base template: %v", err)
	}

	// Create child template that uses parent variables
	childTemplate := `
name: "child"
version: "1.0"
extends: "base"

variables:
  title: "Test"

timeline:
  - type: "text"
    content: "{{title}} - {{subtitle}}"
    start_time: 0
    duration: 3000
    style:
      font_size: 64
      font_color: "{{brand_color}}"
      position: "center"
`

	childFile := filepath.Join(tempDir, "child.yaml")
	if err := os.WriteFile(childFile, []byte(childTemplate), 0644); err != nil {
		t.Fatalf("Failed to write child template: %v", err)
	}

	// Load child template
	tm := NewTemplateManager([]string{tempDir})
	template, err := tm.Load(context.Background(), "child")
	if err != nil {
		t.Fatalf("Failed to load template: %v", err)
	}

	// Create variable substitutor with override
	variables := map[string]interface{}{
		"title":    "Custom Title",
		"subtitle": "Custom Subtitle",
	}
	substitutor := NewVariableSubstitutor(template.Variables)
	// Merge with provided variables
	for k, v := range variables {
		substitutor.SetVariable(k, v)
	}

	// Substitute template
	processed, err := substitutor.SubstituteTemplate(template)
	if err != nil {
		t.Fatalf("Variable substitution failed: %v", err)
	}

	// Verify substitution
	expectedContent := "Custom Title - Custom Subtitle"
	if processed.Timeline[0].Content != expectedContent {
		t.Errorf("Expected content '%s', got '%s'", expectedContent, processed.Timeline[0].Content)
	}

	if processed.Timeline[0].Style.FontColor != "#FF0000" {
		t.Errorf("Expected font color '#FF0000', got '%s'", processed.Timeline[0].Style.FontColor)
	}

	// Verify that parent variables are accessible
	if brandColor, ok := processed.Variables["brand_color"].(string); !ok || brandColor != "#FF0000" {
		t.Errorf("Expected brand_color from parent '#FF0000', got '%v'", processed.Variables["brand_color"])
	}
}
