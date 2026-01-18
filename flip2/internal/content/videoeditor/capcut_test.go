package videoeditor

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"flip2/internal/logger"
)

func TestToCapCutProject(t *testing.T) {
	template := &Template{
		Name:    "test-template",
		Version: "1.0",
		Canvas: CanvasConfig{
			Width:  1920,
			Height: 1080,
			FPS:    30,
		},
		Timeline: []TimelineItem{
			{
				Type:      "video",
				Path:      "/path/to/video.mp4",
				StartTime: 0,
				Duration:  5000,
				Volume:    1.0,
			},
			{
				Type:      "text",
				Content:   "Hello World",
				StartTime: 1000,
				Duration:  3000,
				Style: &TextStyle{
					FontSize:  64,
					FontColor: "#FFFFFF",
					Position:  "center",
					Opacity:   1.0,
				},
			},
		},
	}

	variables := map[string]interface{}{}

	project, err := ToCapCutProject(template, variables)
	if err != nil {
		t.Fatalf("ToCapCutProject failed: %v", err)
	}

	// Verify project structure
	if project.Version == "" {
		t.Error("Expected project version to be set")
	}

	if project.ID == "" {
		t.Error("Expected project ID to be set")
	}

	if project.Duration != 5000*1000 { // 5 seconds in microseconds
		t.Errorf("Expected duration 5000000, got %d", project.Duration)
	}

	if project.CanvasConfig.Width != 1920 || project.CanvasConfig.Height != 1080 {
		t.Errorf("Expected canvas 1920x1080, got %dx%d",
			project.CanvasConfig.Width, project.CanvasConfig.Height)
	}

	if project.CanvasConfig.FPS != 30 {
		t.Errorf("Expected FPS 30, got %d", project.CanvasConfig.FPS)
	}

	// Verify materials
	if len(project.Materials.Videos) != 1 {
		t.Errorf("Expected 1 video material, got %d", len(project.Materials.Videos))
	}

	if len(project.Materials.Texts) != 1 {
		t.Errorf("Expected 1 text material, got %d", len(project.Materials.Texts))
	}

	// Verify tracks
	if len(project.Tracks) == 0 {
		t.Error("Expected at least one track")
	}

	// Find video track
	var videoTrack *CapCutTrack
	for i := range project.Tracks {
		if project.Tracks[i].Type == "video" {
			videoTrack = &project.Tracks[i]
			break
		}
	}

	if videoTrack == nil {
		t.Fatal("Expected video track")
	}

	if len(videoTrack.Segments) != 1 {
		t.Errorf("Expected 1 video segment, got %d", len(videoTrack.Segments))
	}

	// Find text track
	var textTrack *CapCutTrack
	for i := range project.Tracks {
		if project.Tracks[i].Type == "text" {
			textTrack = &project.Tracks[i]
			break
		}
	}

	if textTrack == nil {
		t.Fatal("Expected text track")
	}

	if len(textTrack.Segments) != 1 {
		t.Errorf("Expected 1 text segment, got %d", len(textTrack.Segments))
	}
}

func TestToCapCutProject_WithTransitions(t *testing.T) {
	template := &Template{
		Name:    "test-template",
		Version: "1.0",
		Canvas: CanvasConfig{
			Width:  1920,
			Height: 1080,
			FPS:    30,
		},
		Timeline: []TimelineItem{
			{
				Type:      "video",
				Path:      "/path/to/video.mp4",
				StartTime: 0,
				Duration:  5000,
				Transitions: TransitionConfig{
					In: &Transition{
						Type:     "fade",
						Duration: 500,
					},
					Out: &Transition{
						Type:     "fade",
						Duration: 500,
					},
				},
			},
		},
	}

	variables := map[string]interface{}{}

	project, err := ToCapCutProject(template, variables)
	if err != nil {
		t.Fatalf("ToCapCutProject failed: %v", err)
	}

	// Verify transitions were added
	if len(project.Materials.Transitions) != 2 {
		t.Errorf("Expected 2 transition materials, got %d", len(project.Materials.Transitions))
	}

	// Verify video segment has animations
	videoTrack := &project.Tracks[0]
	if len(videoTrack.Segments) == 0 {
		t.Fatal("Expected video segment")
	}

	segment := videoTrack.Segments[0]
	if len(segment.Animations) != 2 {
		t.Errorf("Expected 2 animations (in/out), got %d", len(segment.Animations))
	}
}

func TestExportCapCutProject(t *testing.T) {
	project := &CapCutProject{
		Version:  "3.0.0",
		ID:       "test123",
		Duration: 5000000,
		Materials: CapCutMaterials{
			Videos: []CapCutVideoMaterial{
				{
					ID:       "video1",
					Path:     "/test.mp4",
					Duration: 5000000,
					Type:     "video",
				},
			},
		},
		Tracks: []CapCutTrack{
			{
				ID:   "track1",
				Type: "video",
				Segments: []CapCutSegment{
					{
						ID:         "seg1",
						MaterialID: "video1",
						TargetTimeRange: CapCutTimeRange{
							Start:    0,
							Duration: 5000000,
						},
					},
				},
			},
		},
		CanvasConfig: CapCutCanvasConfig{
			Width:  1920,
			Height: 1080,
			FPS:    30,
			Ratio:  "16:9",
		},
	}

	jsonData, err := ExportCapCutProject(project)
	if err != nil {
		t.Fatalf("ExportCapCutProject failed: %v", err)
	}

	// Verify JSON is valid
	var parsed CapCutProject
	if err := json.Unmarshal(jsonData, &parsed); err != nil {
		t.Fatalf("Failed to parse exported JSON: %v", err)
	}

	// Verify key fields
	if parsed.ID != "test123" {
		t.Errorf("Expected ID 'test123', got '%s'", parsed.ID)
	}

	if parsed.Duration != 5000000 {
		t.Errorf("Expected duration 5000000, got %d", parsed.Duration)
	}
}

func TestCapCutExporter_Export(t *testing.T) {
	tempDir := t.TempDir()
	log := (*logger.Logger)(nil)

	// Create test template
	templateDir := filepath.Join(tempDir, "templates")
	os.MkdirAll(templateDir, 0755)

	templateYAML := `
name: "test-template"
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
    style:
      font_size: 64
      font_color: "#FFFFFF"
      position: "center"
`

	templateFile := filepath.Join(templateDir, "test-template.yaml")
	os.WriteFile(templateFile, []byte(templateYAML), 0644)

	// Create template manager and exporter
	tm := NewTemplateManager([]string{templateDir})
	exporter := NewCapCutExporter(tm, tempDir, log)

	// Export options
	options := ExportOptions{
		TemplateName: "test-template",
		Variables: map[string]interface{}{
			"title": "Test Video",
		},
		OutputPath:    filepath.Join(tempDir, "test_project"),
		IncludeAssets: false,
		PackageFormat: "directory",
	}

	// Export
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

	// Verify draft.content exists
	if _, err := os.Stat(result.DraftContentPath); err != nil {
		t.Errorf("draft.content file not found: %v", err)
	}

	// Verify draft.content is valid JSON
	data, err := os.ReadFile(result.DraftContentPath)
	if err != nil {
		t.Fatalf("Failed to read draft.content: %v", err)
	}

	var project CapCutProject
	if err := json.Unmarshal(data, &project); err != nil {
		t.Fatalf("draft.content is not valid JSON: %v", err)
	}

	// Verify project structure
	if project.Version == "" {
		t.Error("Expected project version")
	}

	if len(project.Tracks) == 0 {
		t.Error("Expected at least one track")
	}
}

func TestCapCutExporter_Export_WithZip(t *testing.T) {
	tempDir := t.TempDir()
	log := (*logger.Logger)(nil)

	// Create test template
	templateDir := filepath.Join(tempDir, "templates")
	os.MkdirAll(templateDir, 0755)

	templateYAML := `
name: "test-template"
version: "1.0"

canvas:
  width: 1920
  height: 1080
  fps: 30

timeline:
  - type: "text"
    content: "Test"
    start_time: 0
    duration: 3000
`

	templateFile := filepath.Join(templateDir, "test-template.yaml")
	os.WriteFile(templateFile, []byte(templateYAML), 0644)

	// Create template manager and exporter
	tm := NewTemplateManager([]string{templateDir})
	exporter := NewCapCutExporter(tm, tempDir, log)

	// Export options
	options := ExportOptions{
		TemplateName:  "test-template",
		Variables:     map[string]interface{}{},
		OutputPath:    filepath.Join(tempDir, "test_project"),
		IncludeAssets: false,
		PackageFormat: "zip",
	}

	// Export
	result, err := exporter.Export(context.Background(), options)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	// Verify zip file exists
	if _, err := os.Stat(result.ProjectPath); err != nil {
		t.Errorf("Zip file not found: %v", err)
	}

	// Verify it's a zip file
	if filepath.Ext(result.ProjectPath) != ".zip" {
		t.Errorf("Expected .zip extension, got %s", filepath.Ext(result.ProjectPath))
	}
}

func TestValidateCapCutProject(t *testing.T) {
	tests := []struct {
		name          string
		project       *CapCutProject
		expectError   bool
		errorContains string
	}{
		{
			name: "valid project",
			project: &CapCutProject{
				Version:  "3.0.0",
				ID:       "test123",
				Duration: 5000000,
				CanvasConfig: CapCutCanvasConfig{
					Width:  1920,
					Height: 1080,
					FPS:    30,
				},
				Tracks: []CapCutTrack{
					{ID: "track1", Type: "video"},
				},
			},
			expectError: false,
		},
		{
			name: "missing version",
			project: &CapCutProject{
				ID:       "test123",
				Duration: 5000000,
				CanvasConfig: CapCutCanvasConfig{
					Width:  1920,
					Height: 1080,
					FPS:    30,
				},
				Tracks: []CapCutTrack{
					{ID: "track1", Type: "video"},
				},
			},
			expectError:   true,
			errorContains: "version is required",
		},
		{
			name: "missing ID",
			project: &CapCutProject{
				Version:  "3.0.0",
				Duration: 5000000,
				CanvasConfig: CapCutCanvasConfig{
					Width:  1920,
					Height: 1080,
					FPS:    30,
				},
				Tracks: []CapCutTrack{
					{ID: "track1", Type: "video"},
				},
			},
			expectError:   true,
			errorContains: "ID is required",
		},
		{
			name: "negative duration",
			project: &CapCutProject{
				Version:  "3.0.0",
				ID:       "test123",
				Duration: -1000,
				CanvasConfig: CapCutCanvasConfig{
					Width:  1920,
					Height: 1080,
					FPS:    30,
				},
				Tracks: []CapCutTrack{
					{ID: "track1", Type: "video"},
				},
			},
			expectError:   true,
			errorContains: "duration must be positive",
		},
		{
			name: "invalid canvas dimensions",
			project: &CapCutProject{
				Version:  "3.0.0",
				ID:       "test123",
				Duration: 5000000,
				CanvasConfig: CapCutCanvasConfig{
					Width:  0,
					Height: 0,
					FPS:    30,
				},
				Tracks: []CapCutTrack{
					{ID: "track1", Type: "video"},
				},
			},
			expectError:   true,
			errorContains: "invalid canvas dimensions",
		},
		{
			name: "no tracks",
			project: &CapCutProject{
				Version:  "3.0.0",
				ID:       "test123",
				Duration: 5000000,
				CanvasConfig: CapCutCanvasConfig{
					Width:  1920,
					Height: 1080,
					FPS:    30,
				},
				Tracks: []CapCutTrack{},
			},
			expectError:   true,
			errorContains: "must have at least one track",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCapCutProject(tt.project)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tt.errorContains != "" && !contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain '%s', got '%s'", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestCalculateRatio(t *testing.T) {
	tests := []struct {
		width    int
		height   int
		expected string
	}{
		{1920, 1080, "16:9"},
		{1080, 1920, "9:16"},
		{1280, 720, "16:9"},
		{720, 1280, "9:16"},
		{2560, 1440, "16:9"},
		{1024, 768, "4:3"},
		{800, 600, "4:3"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%dx%d", tt.width, tt.height), func(t *testing.T) {
			result := calculateRatio(tt.width, tt.height)
			if result != tt.expected {
				t.Errorf("Expected ratio '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestConvertColorToRGBA(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"#FFFFFF", "#FFFFFFFF"},
		{"#000000", "#000000FF"},
		{"#FF0000", "#FF0000FF"},
		{"#FFFFFFFF", "#FFFFFFFF"}, // Already has alpha
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := convertColorToRGBA(tt.input)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}
