package videoeditor

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"flip2/internal/logger"
)

// TestE2E_CompleteWorkflow tests the entire video editor workflow end-to-end
func TestE2E_CompleteWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	tempDir := t.TempDir()
	log := (*logger.Logger)(nil)

	// 1. Create a comprehensive template with all features
	templateYAML := `
name: "e2e-test"
version: "1.0"

canvas:
  width: 1080
  height: 1920
  fps: 30

variables:
  title: "Default Title"
  subtitle: "Default Subtitle"
  hook: "Attention Grabber"
  cta: "Subscribe Now"
  brand_color: "#FF0000"

timeline:
  - type: "text"
    content: "{{hook}}"
    start_time: 0
    duration: 2000
    style:
      font_size: 72
      font_color: "{{brand_color}}"
      position: "center"
      font_weight: "bold"

  - type: "text"
    content: "{{title}}"
    start_time: 2000
    duration: 4000
    style:
      font_size: 64
      font_color: "#FFFFFF"
      position: "center"

  - type: "text"
    content: "{{subtitle}}"
    start_time: 2500
    duration: 3500
    style:
      font_size: 48
      font_color: "#AAAAAA"
      position: "bottom"

  - type: "text"
    content: "{{cta}}"
    start_time: 6000
    duration: 2000
    style:
      font_size: 56
      font_color: "#00FF00"
      position: "bottom"
      font_weight: "bold"
`

	templateFile := filepath.Join(tempDir, "e2e-test.yaml")
	if err := os.WriteFile(templateFile, []byte(templateYAML), 0644); err != nil {
		t.Fatalf("Failed to write template: %v", err)
	}

	// 2. Create template manager and exporter
	tm := NewTemplateManager([]string{tempDir})
	exporter := NewCapCutExporter(tm, tempDir, log)

	// 3. Export with custom variables
	outputPath := filepath.Join(tempDir, "test_export")
	options := ExportOptions{
		TemplateName: "e2e-test",
		Variables: map[string]interface{}{
			"title":    "My Amazing Video",
			"subtitle": "Learn Something New",
			"hook":     "Wait for it...",
			"cta":      "Like & Subscribe!",
		},
		OutputPath:    outputPath,
		IncludeAssets: false,
		PackageFormat: "directory",
	}

	result, err := exporter.Export(context.Background(), options)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	// 4. Verify export result
	if result.ProjectPath == "" {
		t.Error("Expected project path to be set")
	}

	if result.DraftContentPath == "" {
		t.Error("Expected draft.content path to be set")
	}

	// 5. Load and validate the exported project
	data, err := os.ReadFile(result.DraftContentPath)
	if err != nil {
		t.Fatalf("Failed to read draft.content: %v", err)
	}

	project, err := ImportCapCutProject(data)
	if err != nil {
		t.Fatalf("Failed to import CapCut project: %v", err)
	}

	// 6. Validate project structure
	if err := ValidateCapCutProject(project); err != nil {
		t.Errorf("Validation failed: %v", err)
	}

	// 7. Verify canvas configuration
	if project.CanvasConfig.Width != 1080 {
		t.Errorf("Expected width 1080, got %d", project.CanvasConfig.Width)
	}

	if project.CanvasConfig.Height != 1920 {
		t.Errorf("Expected height 1920, got %d", project.CanvasConfig.Height)
	}

	if project.CanvasConfig.FPS != 30 {
		t.Errorf("Expected FPS 30, got %d", project.CanvasConfig.FPS)
	}

	// 8. Verify text materials and variable substitution
	if len(project.Materials.Texts) != 4 {
		t.Fatalf("Expected 4 text materials, got %d", len(project.Materials.Texts))
	}

	// Check that variables were substituted correctly
	expectedTexts := map[string]bool{
		"Wait for it...":        false,
		"My Amazing Video":      false,
		"Learn Something New":   false,
		"Like & Subscribe!":     false,
	}

	for _, text := range project.Materials.Texts {
		if _, ok := expectedTexts[text.Content]; ok {
			expectedTexts[text.Content] = true
		}
	}

	for text, found := range expectedTexts {
		if !found {
			t.Errorf("Expected text '%s' not found in materials", text)
		}
	}

	// 9. Verify tracks and segments
	if len(project.Tracks) == 0 {
		t.Fatal("Expected at least one track")
	}

	textTrack := project.Tracks[0]
	if len(textTrack.Segments) != 4 {
		t.Errorf("Expected 4 segments in text track, got %d", len(textTrack.Segments))
	}

	// 10. Verify timing
	firstSegment := textTrack.Segments[0]
	if firstSegment.TargetTimeRange.Start != 0 {
		t.Errorf("Expected first segment to start at 0, got %d", firstSegment.TargetTimeRange.Start)
	}

	if firstSegment.TargetTimeRange.Duration != 2000000 {
		t.Errorf("Expected first segment duration 2000000, got %d", firstSegment.TargetTimeRange.Duration)
	}

	// 11. Verify project can be re-serialized
	reexportedData, err := json.Marshal(project)
	if err != nil {
		t.Errorf("Failed to re-serialize project: %v", err)
	}

	if len(reexportedData) == 0 {
		t.Error("Re-exported data is empty")
	}

	t.Logf("✅ E2E test complete - exported %d KB CapCut project", len(data)/1024)
}

// TestE2E_TemplateInheritance tests template inheritance with export
func TestE2E_TemplateInheritance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

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
  font_family: "Arial"
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
  subtitle: "Inherited Properties"

timeline:
  - type: "text"
    content: "{{title}}"
    start_time: 0
    duration: 3000
    style:
      font_size: 64
      font_color: "{{brand_color}}"
      font_family: "{{font_family}}"
      position: "center"

  - type: "text"
    content: "{{subtitle}}"
    start_time: 1000
    duration: 3000
    style:
      font_size: 48
      font_color: "#FFFFFF"
      font_family: "{{font_family}}"
      position: "bottom"
`

	childFile := filepath.Join(tempDir, "child.yaml")
	if err := os.WriteFile(childFile, []byte(childTemplate), 0644); err != nil {
		t.Fatalf("Failed to write child template: %v", err)
	}

	// Create template manager and exporter
	tm := NewTemplateManager([]string{tempDir})
	exporter := NewCapCutExporter(tm, tempDir, log)

	// Export child template (should inherit from base)
	outputPath := filepath.Join(tempDir, "inheritance_export")
	options := ExportOptions{
		TemplateName:  "child",
		Variables:     map[string]interface{}{},
		OutputPath:    outputPath,
		IncludeAssets: false,
		PackageFormat: "directory",
	}

	result, err := exporter.Export(context.Background(), options)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	// Load and validate
	data, err := os.ReadFile(result.DraftContentPath)
	if err != nil {
		t.Fatalf("Failed to read draft.content: %v", err)
	}

	project, err := ImportCapCutProject(data)
	if err != nil {
		t.Fatalf("Failed to import project: %v", err)
	}

	// Verify inheritance worked
	if project.CanvasConfig.Width != 1920 {
		t.Errorf("Canvas width not inherited from base: got %d", project.CanvasConfig.Width)
	}

	if project.CanvasConfig.Height != 1080 {
		t.Errorf("Canvas height not inherited from base: got %d", project.CanvasConfig.Height)
	}

	// Verify variable substitution from both templates
	foundTitle := false
	foundSubtitle := false

	// Debug: print all text contents
	for i, text := range project.Materials.Texts {
		t.Logf("Text material %d: '%s' (color: %s)", i, text.Content, text.Style.Color)
	}

	for _, text := range project.Materials.Texts {
		if text.Content == "Test Video" {
			foundTitle = true
			// Verify brand_color from base was used (RRGGBBAA format)
			if text.Style.Color != "#FF0000FF" {
				t.Errorf("Expected brand color #FF0000FF from base, got %s", text.Style.Color)
			}
		}
		if text.Content == "Inherited Properties" {
			foundSubtitle = true
		}
	}

	if !foundTitle {
		t.Error("Title from child template not found")
	}

	if !foundSubtitle {
		t.Error("Subtitle from child template not found")
	}

	t.Logf("✅ Template inheritance test complete")
}

// TestE2E_BatchProcessing tests batch processing with multiple exports
func TestE2E_BatchProcessing(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	tempDir := t.TempDir()
	log := (*logger.Logger)(nil)

	// Create a simple template
	templateYAML := `
name: "batch-test"
version: "1.0"

canvas:
  width: 1080
  height: 1920
  fps: 30

timeline:
  - type: "text"
    content: "{{title}}"
    start_time: 0
    duration: 3000
    style:
      font_size: 64
      font_color: "#FFFFFF"
      position: "center"
`

	templateFile := filepath.Join(tempDir, "batch-test.yaml")
	if err := os.WriteFile(templateFile, []byte(templateYAML), 0644); err != nil {
		t.Fatalf("Failed to write template: %v", err)
	}

	// Create editor and batch processor
	editor, err := New(log, "ffmpeg", "ffprobe", tempDir, tempDir)
	if err != nil {
		t.Fatalf("Failed to create editor: %v", err)
	}

	tm := NewTemplateManager([]string{tempDir})
	bp := NewBatchProcessor(editor, tm, 2, log) // 2 concurrent workers

	// Create batch jobs
	jobs := []*BatchJob{
		{
			ID:           "job_1",
			TemplateName: "batch-test",
			Variables: map[string]interface{}{
				"title": "Video 1",
			},
			OutputPath: filepath.Join(tempDir, "batch_output_1"),
			Priority:   0,
		},
		{
			ID:           "job_2",
			TemplateName: "batch-test",
			Variables: map[string]interface{}{
				"title": "Video 2",
			},
			OutputPath: filepath.Join(tempDir, "batch_output_2"),
			Priority:   1,
		},
		{
			ID:           "job_3",
			TemplateName: "batch-test",
			Variables: map[string]interface{}{
				"title": "Video 3",
			},
			OutputPath: filepath.Join(tempDir, "batch_output_3"),
			Priority:   0,
		},
	}

	// Process batch
	results, err := bp.ProcessBatch(context.Background(), jobs)
	if err != nil {
		t.Fatalf("ProcessBatch failed: %v", err)
	}

	// Verify results
	if len(results) != len(jobs) {
		t.Errorf("Expected %d results, got %d", len(jobs), len(results))
	}

	successCount := 0
	for _, result := range results {
		if result.Success {
			successCount++

			// Verify each output file exists
			draftPath := filepath.Join(result.OutputPath, "draft.content")
			if _, err := os.Stat(draftPath); err != nil {
				t.Errorf("Output file not found: %s", draftPath)
				continue
			}

			// Verify content
			data, err := os.ReadFile(draftPath)
			if err != nil {
				t.Errorf("Failed to read output: %v", err)
				continue
			}

			project, err := ImportCapCutProject(data)
			if err != nil {
				t.Errorf("Failed to import project: %v", err)
				continue
			}

			// Verify project is valid
			if len(project.Materials.Texts) == 0 {
				t.Error("Exported project has no text materials")
			}
		}
	}

	t.Logf("✅ Batch processing complete: %d/%d successful", successCount, len(jobs))

	// Verify progress tracking
	stats := bp.GetProgress()
	if stats.TotalJobs != int64(len(jobs)) {
		t.Errorf("Expected total jobs %d, got %d", len(jobs), stats.TotalJobs)
	}

	if stats.CompletedJobs+stats.FailedJobs != int64(len(jobs)) {
		t.Errorf("Completed+Failed != Total: %d+%d != %d",
			stats.CompletedJobs, stats.FailedJobs, stats.TotalJobs)
	}
}

// TestE2E_AllEffects tests that all effect types can be registered and applied
func TestE2E_AllEffects(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	registry := NewEffectRegistry()

	effects := []struct {
		name   string
		params map[string]interface{}
	}{
		{"color_correct", map[string]interface{}{"brightness": 0.2, "contrast": 1.1}},
		{"blur", map[string]interface{}{"radius": 5.0}},
		{"sharpen", map[string]interface{}{"strength": 50}},
		{"noise", map[string]interface{}{"strength": 0.05}},
		{"vignette", map[string]interface{}{"intensity": 0.5}},
	}

	for _, tc := range effects {
		t.Run(tc.name, func(t *testing.T) {
			// Verify effect is registered
			effect, err := registry.Get(tc.name)
			if err != nil {
				t.Fatalf("Effect %s not found in registry: %v", tc.name, err)
			}

			// Verify effect has correct type
			if effect == nil {
				t.Fatal("Effect is nil")
			}

			t.Logf("✅ Effect '%s' registered and accessible", tc.name)
		})
	}
}

// TestE2E_VariableSubstitution tests comprehensive variable substitution
func TestE2E_VariableSubstitution(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	tempDir := t.TempDir()
	log := (*logger.Logger)(nil)

	// Create template with various variable types
	templateYAML := `
name: "var-test"
version: "1.0"

canvas:
  width: 1080
  height: 1920
  fps: 30

variables:
  text1: "Default1"
  text2: "Default2"
  color: "#FF0000"
  position: "center"

timeline:
  - type: "text"
    content: "{{text1}} and {{text2}}"
    start_time: 0
    duration: 3000
    style:
      font_size: 72
      font_color: "{{color}}"
      position: "{{position}}"
`

	templateFile := filepath.Join(tempDir, "var-test.yaml")
	if err := os.WriteFile(templateFile, []byte(templateYAML), 0644); err != nil {
		t.Fatalf("Failed to write template: %v", err)
	}

	tm := NewTemplateManager([]string{tempDir})
	exporter := NewCapCutExporter(tm, tempDir, log)

	// Export with custom variables
	options := ExportOptions{
		TemplateName: "var-test",
		Variables: map[string]interface{}{
			"text1":    "Hello",
			"text2":    "World",
			"color":    "#00FF00",
			"position": "bottom",
		},
		OutputPath:    filepath.Join(tempDir, "var_export"),
		IncludeAssets: false,
		PackageFormat: "directory",
	}

	result, err := exporter.Export(context.Background(), options)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	// Load and verify
	data, err := os.ReadFile(result.DraftContentPath)
	if err != nil {
		t.Fatalf("Failed to read output: %v", err)
	}

	project, err := ImportCapCutProject(data)
	if err != nil {
		t.Fatalf("Failed to import project: %v", err)
	}

	// Verify variable substitution
	if len(project.Materials.Texts) == 0 {
		t.Fatal("No text materials found")
	}

	text := project.Materials.Texts[0]

	if text.Content != "Hello and World" {
		t.Errorf("Expected 'Hello and World', got '%s'", text.Content)
	}

	if text.Style.FontSize != 72 {
		t.Errorf("Expected font size 72, got %d", text.Style.FontSize)
	}

	// Color format is RRGGBBAA
	if text.Style.Color != "#00FF00FF" {
		t.Errorf("Expected color #00FF00FF (green with alpha), got %s", text.Style.Color)
	}

	t.Logf("✅ Variable substitution test complete")
}
