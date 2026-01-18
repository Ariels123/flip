// Package videoeditor - Template management for video composition.
package videoeditor

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Template represents a video composition template with timeline and effects.
type Template struct {
	// Name of the template
	Name string `json:"name" yaml:"name"`

	// Version of the template format
	Version string `json:"version" yaml:"version"`

	// Extends references a parent template for inheritance
	Extends string `json:"extends,omitempty" yaml:"extends,omitempty"`

	// Canvas defines output video dimensions and framerate
	Canvas CanvasConfig `json:"canvas" yaml:"canvas"`

	// Timeline contains ordered video composition elements
	Timeline []TimelineItem `json:"timeline" yaml:"timeline"`

	// Effects are global effects applied to the composition
	Effects []EffectConfig `json:"effects,omitempty" yaml:"effects,omitempty"`

	// Variables defines template variables with default values
	Variables map[string]interface{} `json:"variables,omitempty" yaml:"variables,omitempty"`

	// Metadata contains additional template information
	Metadata map[string]interface{} `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

// CanvasConfig defines output video specifications.
type CanvasConfig struct {
	Width  int `json:"width" yaml:"width"`
	Height int `json:"height" yaml:"height"`
	FPS    int `json:"fps" yaml:"fps"`
}

// TimelineItem represents a single element in the video timeline.
type TimelineItem struct {
	// Type: "video", "audio", "text", "image"
	Type string `json:"type" yaml:"type"`

	// Path to the media file (supports {{variables}})
	Path string `json:"path,omitempty" yaml:"path,omitempty"`

	// Content for text items (supports {{variables}})
	Content string `json:"content,omitempty" yaml:"content,omitempty"`

	// StartTime in milliseconds
	StartTime float64 `json:"start_time" yaml:"start_time"`

	// Duration in milliseconds
	Duration float64 `json:"duration" yaml:"duration"`

	// Transitions applied to this item
	Transitions TransitionConfig `json:"transitions,omitempty" yaml:"transitions,omitempty"`

	// Style for text items
	Style *TextStyle `json:"style,omitempty" yaml:"style,omitempty"`

	// Effects specific to this timeline item
	Effects []EffectConfig `json:"effects,omitempty" yaml:"effects,omitempty"`

	// Volume for audio/video items (0.0 to 1.0)
	Volume float64 `json:"volume,omitempty" yaml:"volume,omitempty"`
}

// TransitionConfig defines transitions for a timeline item.
type TransitionConfig struct {
	// In transition when item appears
	In *Transition `json:"in,omitempty" yaml:"in,omitempty"`

	// Out transition when item disappears
	Out *Transition `json:"out,omitempty" yaml:"out,omitempty"`
}

// Transition defines a single transition effect.
type Transition struct {
	// Type: "fade", "dissolve", "crossfade", "wipe", "slidedown"
	Type string `json:"type" yaml:"type"`

	// Duration in milliseconds
	Duration float64 `json:"duration" yaml:"duration"`

	// Parameters for transition-specific settings
	Parameters map[string]interface{} `json:"parameters,omitempty" yaml:"parameters,omitempty"`
}

// TextStyle defines styling for text overlays.
type TextStyle struct {
	FontSize  int     `json:"font_size" yaml:"font_size"`
	FontColor string  `json:"font_color" yaml:"font_color"`
	FontFile  string  `json:"font_file,omitempty" yaml:"font_file,omitempty"`
	Position  string  `json:"position" yaml:"position"` // "top", "center", "bottom", or "x,y"
	Opacity   float64 `json:"opacity,omitempty" yaml:"opacity,omitempty"`
}

// EffectConfig defines a video effect.
type EffectConfig struct {
	// Type of effect: "color_correct", "blur", "sharpen", etc.
	Type string `json:"type" yaml:"type"`

	// Parameters for effect-specific settings
	Parameters map[string]interface{} `json:"parameters,omitempty" yaml:"parameters,omitempty"`
}

// TemplateManager handles loading, parsing, and managing templates.
type TemplateManager struct {
	templateDirs []string
	cache        map[string]*Template
}

// NewTemplateManager creates a new template manager.
func NewTemplateManager(templateDirs []string) *TemplateManager {
	return &TemplateManager{
		templateDirs: templateDirs,
		cache:        make(map[string]*Template),
	}
}

// Load loads a template by name, supporting template inheritance.
func (tm *TemplateManager) Load(ctx context.Context, name string) (*Template, error) {
	// Check cache first
	if cached, ok := tm.cache[name]; ok {
		return cached, nil
	}

	// Find template file
	templatePath, err := tm.findTemplate(name)
	if err != nil {
		return nil, fmt.Errorf("template not found: %s", name)
	}

	// Load and parse template
	template, err := tm.parseTemplateFile(templatePath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}

	// Handle template inheritance
	if template.Extends != "" {
		baseTemplate, err := tm.Load(ctx, template.Extends)
		if err != nil {
			return nil, fmt.Errorf("failed to load parent template %s: %w", template.Extends, err)
		}

		// Merge with parent template
		template = tm.mergeTemplates(baseTemplate, template)

		// Validate the merged template
		template.Extends = "" // Clear extends so validation checks canvas
		if err := tm.validateTemplate(template); err != nil {
			return nil, fmt.Errorf("merged template validation failed: %w", err)
		}
	}

	// Cache the resolved template
	tm.cache[name] = template

	return template, nil
}

// findTemplate searches for a template file in configured directories.
func (tm *TemplateManager) findTemplate(name string) (string, error) {
	// Try with various extensions
	extensions := []string{".yaml", ".yml", ".json"}

	for _, dir := range tm.templateDirs {
		for _, ext := range extensions {
			// Try direct path
			path := filepath.Join(dir, name+ext)
			if _, err := os.Stat(path); err == nil {
				return path, nil
			}

			// Try with verticals/ prefix
			path = filepath.Join(dir, "verticals", name+ext)
			if _, err := os.Stat(path); err == nil {
				return path, nil
			}

			// Try splitting name into path components (e.g., "youtube/educational")
			parts := strings.Split(name, "/")
			if len(parts) > 1 {
				path = filepath.Join(dir, "verticals", filepath.Join(parts...)+ext)
				if _, err := os.Stat(path); err == nil {
					return path, nil
				}
			}
		}
	}

	return "", fmt.Errorf("template not found in any directory")
}

// parseTemplateFile parses a template from a file.
func (tm *TemplateManager) parseTemplateFile(path string) (*Template, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read template file: %w", err)
	}

	ext := filepath.Ext(path)
	var template Template

	switch ext {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &template); err != nil {
			return nil, fmt.Errorf("failed to parse YAML: %w", err)
		}
	case ".json":
		if err := json.Unmarshal(data, &template); err != nil {
			return nil, fmt.Errorf("failed to parse JSON: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported template format: %s", ext)
	}

	// Validate template
	if err := tm.validateTemplate(&template); err != nil {
		return nil, fmt.Errorf("template validation failed: %w", err)
	}

	return &template, nil
}

// validateTemplate performs basic validation on a template.
func (tm *TemplateManager) validateTemplate(t *Template) error {
	if t.Name == "" {
		return fmt.Errorf("template name is required")
	}

	// Skip canvas validation if this template extends another (will be validated after merge)
	if t.Extends == "" {
		if t.Canvas.Width <= 0 || t.Canvas.Height <= 0 {
			return fmt.Errorf("invalid canvas dimensions: %dx%d", t.Canvas.Width, t.Canvas.Height)
		}

		if t.Canvas.FPS <= 0 {
			t.Canvas.FPS = 30 // Default FPS
		}
	}

	// Validate timeline items
	for i, item := range t.Timeline {
		if item.Type == "" {
			return fmt.Errorf("timeline item %d: type is required", i)
		}

		if item.Type == "text" && item.Content == "" {
			return fmt.Errorf("timeline item %d: text content is required", i)
		}

		if item.Type != "text" && item.Path == "" {
			return fmt.Errorf("timeline item %d: path is required for %s items", i, item.Type)
		}

		if item.Duration <= 0 {
			return fmt.Errorf("timeline item %d: duration must be positive", i)
		}
	}

	return nil
}

// mergeTemplates merges a child template with its parent.
// Child values override parent values.
func (tm *TemplateManager) mergeTemplates(parent, child *Template) *Template {
	result := &Template{
		Name:      child.Name,
		Version:   child.Version,
		Canvas:    child.Canvas,
		Timeline:  make([]TimelineItem, 0),
		Effects:   make([]EffectConfig, 0),
		Variables: make(map[string]interface{}),
		Metadata:  make(map[string]interface{}),
	}

	// If child doesn't override canvas, use parent's
	if child.Canvas.Width == 0 && child.Canvas.Height == 0 {
		result.Canvas = parent.Canvas
	}

	// Merge variables (parent first, then child overrides)
	for k, v := range parent.Variables {
		result.Variables[k] = v
	}
	for k, v := range child.Variables {
		result.Variables[k] = v
	}

	// Merge metadata
	for k, v := range parent.Metadata {
		result.Metadata[k] = v
	}
	for k, v := range child.Metadata {
		result.Metadata[k] = v
	}

	// Merge effects (parent effects first, then child effects)
	result.Effects = append(result.Effects, parent.Effects...)
	result.Effects = append(result.Effects, child.Effects...)

	// Merge timeline (child timeline replaces parent if provided, otherwise use parent)
	if len(child.Timeline) > 0 {
		result.Timeline = child.Timeline
	} else {
		result.Timeline = parent.Timeline
	}

	return result
}

// ListTemplates returns all available templates in configured directories.
func (tm *TemplateManager) ListTemplates() ([]string, error) {
	templates := make([]string, 0)
	seen := make(map[string]bool)

	for _, dir := range tm.templateDirs {
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // Skip inaccessible paths
			}

			if info.IsDir() {
				return nil
			}

			ext := filepath.Ext(path)
			if ext != ".yaml" && ext != ".yml" && ext != ".json" {
				return nil
			}

			// Get relative path from template directory
			relPath, _ := filepath.Rel(dir, path)
			// Remove extension
			name := strings.TrimSuffix(relPath, ext)
			// Remove "verticals/" prefix if present
			name = strings.TrimPrefix(name, "verticals/")
			name = strings.TrimPrefix(name, "verticals\\") // Windows

			if !seen[name] {
				templates = append(templates, name)
				seen[name] = true
			}

			return nil
		})

		if err != nil {
			continue // Skip directories with errors
		}
	}

	return templates, nil
}

// ClearCache clears the template cache.
func (tm *TemplateManager) ClearCache() {
	tm.cache = make(map[string]*Template)
}
