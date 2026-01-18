package videoeditor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTemplateManager_Load(t *testing.T) {
	// Create temp directory for test templates
	tempDir := t.TempDir()

	// Create a base template
	baseTemplate := `
name: "base"
version: "1.0"

canvas:
  width: 1920
  height: 1080
  fps: 30

variables:
  title: "Base Title"
  color: "#FF0000"

timeline:
  - type: "text"
    content: "{{title}}"
    start_time: 0
    duration: 5000
    style:
      font_size: 48
      font_color: "{{color}}"
      position: "center"
`

	baseFile := filepath.Join(tempDir, "base.yaml")
	if err := os.WriteFile(baseFile, []byte(baseTemplate), 0644); err != nil {
		t.Fatalf("Failed to write base template: %v", err)
	}

	// Test loading base template
	tm := NewTemplateManager([]string{tempDir})
	template, err := tm.Load(context.Background(), "base")
	if err != nil {
		t.Fatalf("Failed to load template: %v", err)
	}

	// Validate loaded template
	if template.Name != "base" {
		t.Errorf("Expected name 'base', got '%s'", template.Name)
	}

	if template.Canvas.Width != 1920 || template.Canvas.Height != 1080 {
		t.Errorf("Expected canvas 1920x1080, got %dx%d", template.Canvas.Width, template.Canvas.Height)
	}

	if template.Canvas.FPS != 30 {
		t.Errorf("Expected FPS 30, got %d", template.Canvas.FPS)
	}

	if len(template.Timeline) != 1 {
		t.Errorf("Expected 1 timeline item, got %d", len(template.Timeline))
	}

	if title, ok := template.Variables["title"].(string); !ok || title != "Base Title" {
		t.Errorf("Expected title 'Base Title', got '%v'", template.Variables["title"])
	}
}

func TestTemplateManager_Inheritance(t *testing.T) {
	tempDir := t.TempDir()

	// Create base template
	baseTemplate := `
name: "base"
version: "1.0"

canvas:
  width: 1920
  height: 1080
  fps: 30

variables:
  title: "Base Title"
  brand_color: "#FF0000"

timeline:
  - type: "video"
    path: "base_video.mp4"
    start_time: 0
    duration: 5000

effects:
  - type: "color_correct"
    parameters:
      brightness: 0.1
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
  title: "Child Title"
  subtitle: "Child Subtitle"

timeline:
  - type: "text"
    content: "{{title}}"
    start_time: 1000
    duration: 3000
    style:
      font_size: 64
      font_color: "{{brand_color}}"
      position: "center"

effects:
  - type: "sharpen"
    parameters:
      strength: 50
`

	childFile := filepath.Join(tempDir, "child.yaml")
	if err := os.WriteFile(childFile, []byte(childTemplate), 0644); err != nil {
		t.Fatalf("Failed to write child template: %v", err)
	}

	// Load child template (should merge with base)
	tm := NewTemplateManager([]string{tempDir})
	template, err := tm.Load(context.Background(), "child")
	if err != nil {
		t.Fatalf("Failed to load child template: %v", err)
	}

	// Validate inheritance
	if template.Name != "child" {
		t.Errorf("Expected name 'child', got '%s'", template.Name)
	}

	// Canvas should come from base
	if template.Canvas.Width != 1920 || template.Canvas.Height != 1080 {
		t.Errorf("Expected canvas 1920x1080 from base, got %dx%d", template.Canvas.Width, template.Canvas.Height)
	}

	// Variables should be merged (child overrides base)
	if title, ok := template.Variables["title"].(string); !ok || title != "Child Title" {
		t.Errorf("Expected title 'Child Title', got '%v'", template.Variables["title"])
	}

	if brandColor, ok := template.Variables["brand_color"].(string); !ok || brandColor != "#FF0000" {
		t.Errorf("Expected brand_color from base '#FF0000', got '%v'", template.Variables["brand_color"])
	}

	if subtitle, ok := template.Variables["subtitle"].(string); !ok || subtitle != "Child Subtitle" {
		t.Errorf("Expected subtitle 'Child Subtitle', got '%v'", template.Variables["subtitle"])
	}

	// Timeline should be from child (replaces base)
	if len(template.Timeline) != 1 {
		t.Errorf("Expected 1 timeline item from child, got %d", len(template.Timeline))
	}

	if template.Timeline[0].Type != "text" {
		t.Errorf("Expected text timeline item from child, got '%s'", template.Timeline[0].Type)
	}

	// Effects should be merged (base + child)
	if len(template.Effects) != 2 {
		t.Errorf("Expected 2 effects (base + child), got %d", len(template.Effects))
	}
}

func TestTemplateManager_Validation(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name          string
		templateYAML  string
		expectError   bool
		errorContains string
	}{
		{
			name: "valid template",
			templateYAML: `
name: "valid"
version: "1.0"
canvas:
  width: 1920
  height: 1080
  fps: 30
timeline:
  - type: "video"
    path: "test.mp4"
    start_time: 0
    duration: 5000
`,
			expectError: false,
		},
		{
			name: "missing name",
			templateYAML: `
version: "1.0"
canvas:
  width: 1920
  height: 1080
  fps: 30
timeline: []
`,
			expectError:   true,
			errorContains: "name is required",
		},
		{
			name: "invalid canvas dimensions",
			templateYAML: `
name: "invalid"
version: "1.0"
canvas:
  width: 0
  height: 0
  fps: 30
timeline: []
`,
			expectError:   true,
			errorContains: "invalid canvas dimensions",
		},
		{
			name: "missing timeline item type",
			templateYAML: `
name: "invalid"
version: "1.0"
canvas:
  width: 1920
  height: 1080
  fps: 30
timeline:
  - path: "test.mp4"
    start_time: 0
    duration: 5000
`,
			expectError:   true,
			errorContains: "type is required",
		},
		{
			name: "text item without content",
			templateYAML: `
name: "invalid"
version: "1.0"
canvas:
  width: 1920
  height: 1080
  fps: 30
timeline:
  - type: "text"
    start_time: 0
    duration: 5000
`,
			expectError:   true,
			errorContains: "text content is required",
		},
		{
			name: "video item without path",
			templateYAML: `
name: "invalid"
version: "1.0"
canvas:
  width: 1920
  height: 1080
  fps: 30
timeline:
  - type: "video"
    start_time: 0
    duration: 5000
`,
			expectError:   true,
			errorContains: "path is required",
		},
		{
			name: "negative duration",
			templateYAML: `
name: "invalid"
version: "1.0"
canvas:
  width: 1920
  height: 1080
  fps: 30
timeline:
  - type: "video"
    path: "test.mp4"
    start_time: 0
    duration: -5000
`,
			expectError:   true,
			errorContains: "duration must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testFile := filepath.Join(tempDir, tt.name+".yaml")
			if err := os.WriteFile(testFile, []byte(tt.templateYAML), 0644); err != nil {
				t.Fatalf("Failed to write test template: %v", err)
			}

			tm := NewTemplateManager([]string{tempDir})
			_, err := tm.Load(context.Background(), tt.name)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
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

func TestTemplateManager_Cache(t *testing.T) {
	tempDir := t.TempDir()

	templateYAML := `
name: "cached"
version: "1.0"
canvas:
  width: 1920
  height: 1080
  fps: 30
timeline: []
`

	templateFile := filepath.Join(tempDir, "cached.yaml")
	if err := os.WriteFile(templateFile, []byte(templateYAML), 0644); err != nil {
		t.Fatalf("Failed to write template: %v", err)
	}

	tm := NewTemplateManager([]string{tempDir})

	// Load template first time
	template1, err := tm.Load(context.Background(), "cached")
	if err != nil {
		t.Fatalf("Failed to load template: %v", err)
	}

	// Load template second time (should come from cache)
	template2, err := tm.Load(context.Background(), "cached")
	if err != nil {
		t.Fatalf("Failed to load cached template: %v", err)
	}

	// Should be same pointer (cached)
	if template1 != template2 {
		t.Errorf("Expected cached template to be same pointer")
	}

	// Clear cache
	tm.ClearCache()

	// Load template third time (should reload from file)
	template3, err := tm.Load(context.Background(), "cached")
	if err != nil {
		t.Fatalf("Failed to load template after cache clear: %v", err)
	}

	// Should be different pointer (not cached)
	if template1 == template3 {
		t.Errorf("Expected new template after cache clear")
	}
}

func TestTemplateManager_ListTemplates(t *testing.T) {
	tempDir := t.TempDir()

	// Create several test templates
	templates := []string{"template1", "template2", "template3"}
	for _, name := range templates {
		templateYAML := `
name: "` + name + `"
version: "1.0"
canvas:
  width: 1920
  height: 1080
  fps: 30
timeline: []
`
		templateFile := filepath.Join(tempDir, name+".yaml")
		if err := os.WriteFile(templateFile, []byte(templateYAML), 0644); err != nil {
			t.Fatalf("Failed to write template %s: %v", name, err)
		}
	}

	tm := NewTemplateManager([]string{tempDir})
	found, err := tm.ListTemplates()
	if err != nil {
		t.Fatalf("Failed to list templates: %v", err)
	}

	if len(found) != len(templates) {
		t.Errorf("Expected %d templates, got %d", len(templates), len(found))
	}

	// Check that all templates were found
	foundMap := make(map[string]bool)
	for _, name := range found {
		foundMap[name] = true
	}

	for _, name := range templates {
		if !foundMap[name] {
			t.Errorf("Expected to find template '%s' in list", name)
		}
	}
}

