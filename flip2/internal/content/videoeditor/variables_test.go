package videoeditor

import (
	"testing"
)

func TestVariableSubstitutor_Substitute(t *testing.T) {
	tests := []struct {
		name      string
		variables map[string]interface{}
		input     string
		expected  string
	}{
		{
			name:      "simple variable",
			variables: map[string]interface{}{"title": "My Video"},
			input:     "Welcome to {{title}}",
			expected:  "Welcome to My Video",
		},
		{
			name:      "multiple variables",
			variables: map[string]interface{}{"first": "John", "last": "Doe"},
			input:     "{{first}} {{last}}",
			expected:  "John Doe",
		},
		{
			name:      "variable with default (variable exists)",
			variables: map[string]interface{}{"title": "My Video"},
			input:     "{{title|Default Title}}",
			expected:  "My Video",
		},
		{
			name:      "variable with default (variable missing)",
			variables: map[string]interface{}{},
			input:     "{{title|Default Title}}",
			expected:  "Default Title",
		},
		{
			name:      "nested variable",
			variables: map[string]interface{}{"user": map[string]interface{}{"name": "Alice"}},
			input:     "Hello {{user.name}}",
			expected:  "Hello Alice",
		},
		{
			name: "deeply nested variable",
			variables: map[string]interface{}{
				"config": map[string]interface{}{
					"video": map[string]interface{}{
						"quality": "1080p",
					},
				},
			},
			input:    "Quality: {{config.video.quality}}",
			expected: "Quality: 1080p",
		},
		{
			name:      "missing variable without default",
			variables: map[string]interface{}{},
			input:     "Title: {{title}}",
			expected:  "Title: {{title}}", // Should remain unchanged
		},
		{
			name:      "integer variable",
			variables: map[string]interface{}{"count": 42},
			input:     "Count: {{count}}",
			expected:  "Count: 42",
		},
		{
			name:      "float variable",
			variables: map[string]interface{}{"price": 19.99},
			input:     "Price: ${{price}}",
			expected:  "Price: $19.99",
		},
		{
			name:      "boolean variable",
			variables: map[string]interface{}{"enabled": true},
			input:     "Enabled: {{enabled}}",
			expected:  "Enabled: true",
		},
		{
			name:      "no variables in input",
			variables: map[string]interface{}{"title": "My Video"},
			input:     "No variables here",
			expected:  "No variables here",
		},
		{
			name:      "empty variable map",
			variables: map[string]interface{}{},
			input:     "Static text",
			expected:  "Static text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vs := NewVariableSubstitutor(tt.variables)
			result, err := vs.Substitute(tt.input)
			if err != nil {
				t.Fatalf("Substitute failed: %v", err)
			}

			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestVariableSubstitutor_SetVariable(t *testing.T) {
	vs := NewVariableSubstitutor(make(map[string]interface{}))

	// Set simple variable
	err := vs.SetVariable("title", "My Video")
	if err != nil {
		t.Fatalf("SetVariable failed: %v", err)
	}

	// Verify it was set
	val, err := vs.GetVariable("title")
	if err != nil {
		t.Fatalf("GetVariable failed: %v", err)
	}

	if val != "My Video" {
		t.Errorf("Expected 'My Video', got '%v'", val)
	}
}

func TestVariableSubstitutor_SetNestedVariable(t *testing.T) {
	vs := NewVariableSubstitutor(make(map[string]interface{}))

	// Set nested variable
	err := vs.SetVariable("user.name", "Alice")
	if err != nil {
		t.Fatalf("SetVariable failed: %v", err)
	}

	// Verify it was set
	val, err := vs.GetVariable("user.name")
	if err != nil {
		t.Fatalf("GetVariable failed: %v", err)
	}

	if val != "Alice" {
		t.Errorf("Expected 'Alice', got '%v'", val)
	}

	// Set another nested variable in the same path
	err = vs.SetVariable("user.age", 30)
	if err != nil {
		t.Fatalf("SetVariable failed: %v", err)
	}

	// Verify both exist
	name, err := vs.GetVariable("user.name")
	if err != nil || name != "Alice" {
		t.Errorf("Expected user.name to still be 'Alice', got '%v'", name)
	}

	age, err := vs.GetVariable("user.age")
	if err != nil || age != 30 {
		t.Errorf("Expected user.age to be 30, got '%v'", age)
	}
}

func TestVariableSubstitutor_HasVariable(t *testing.T) {
	vs := NewVariableSubstitutor(map[string]interface{}{
		"title": "My Video",
		"user": map[string]interface{}{
			"name": "Alice",
		},
	})

	tests := []struct {
		path   string
		exists bool
	}{
		{"title", true},
		{"user.name", true},
		{"missing", false},
		{"user.age", false},
		{"user.profile.bio", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			exists := vs.HasVariable(tt.path)
			if exists != tt.exists {
				t.Errorf("HasVariable('%s'): expected %v, got %v", tt.path, tt.exists, exists)
			}
		})
	}
}

func TestVariableSubstitutor_MergeVariables(t *testing.T) {
	vs := NewVariableSubstitutor(map[string]interface{}{
		"title":       "Original Title",
		"description": "Original Description",
	})

	// Merge new variables
	newVars := map[string]interface{}{
		"title":  "New Title",      // Should override
		"author": "John Doe",       // Should add
	}

	vs.MergeVariables(newVars)

	// Check title was overridden
	title, err := vs.GetVariable("title")
	if err != nil || title != "New Title" {
		t.Errorf("Expected title to be overridden to 'New Title', got '%v'", title)
	}

	// Check description is still there
	desc, err := vs.GetVariable("description")
	if err != nil || desc != "Original Description" {
		t.Errorf("Expected description to remain 'Original Description', got '%v'", desc)
	}

	// Check author was added
	author, err := vs.GetVariable("author")
	if err != nil || author != "John Doe" {
		t.Errorf("Expected author to be added as 'John Doe', got '%v'", author)
	}
}

func TestVariableSubstitutor_SubstituteTemplate(t *testing.T) {
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
				Path:      "{{video_path}}",
				StartTime: 0,
				Duration:  5000,
			},
			{
				Type:      "text",
				Content:   "Welcome to {{title}}",
				StartTime: 1000,
				Duration:  3000,
				Style: &TextStyle{
					FontSize:  64,
					FontColor: "{{brand_color}}",
					Position:  "center",
				},
			},
		},
		Variables: map[string]interface{}{
			"title":       "My Video",
			"brand_color": "#FF0000",
		},
	}

	variables := map[string]interface{}{
		"video_path":  "/path/to/video.mp4",
		"title":       "Custom Title",
		"brand_color": "#0000FF",
	}

	vs := NewVariableSubstitutor(variables)
	result, err := vs.SubstituteTemplate(template)
	if err != nil {
		t.Fatalf("SubstituteTemplate failed: %v", err)
	}

	// Check path substitution
	if result.Timeline[0].Path != "/path/to/video.mp4" {
		t.Errorf("Expected path '/path/to/video.mp4', got '%s'", result.Timeline[0].Path)
	}

	// Check content substitution
	if result.Timeline[1].Content != "Welcome to Custom Title" {
		t.Errorf("Expected content 'Welcome to Custom Title', got '%s'", result.Timeline[1].Content)
	}

	// Check style substitution
	if result.Timeline[1].Style.FontColor != "#0000FF" {
		t.Errorf("Expected font color '#0000FF', got '%s'", result.Timeline[1].Style.FontColor)
	}

	// Original template should be unchanged
	if template.Timeline[0].Path != "{{video_path}}" {
		t.Errorf("Original template was modified")
	}
}

func TestVariableSubstitutor_EdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		variables map[string]interface{}
		input     string
		expected  string
	}{
		{
			name:      "adjacent variables",
			variables: map[string]interface{}{"first": "Hello", "second": "World"},
			input:     "{{first}}{{second}}",
			expected:  "HelloWorld",
		},
		{
			name:      "variable with spaces",
			variables: map[string]interface{}{"my title": "Spaced Variable"},
			input:     "{{ my title }}",
			expected:  "Spaced Variable",
		},
		{
			name:      "malformed variable (no closing braces)",
			variables: map[string]interface{}{"title": "My Video"},
			input:     "{{title",
			expected:  "{{title", // Should remain unchanged
		},
		{
			name:      "malformed variable (no opening braces)",
			variables: map[string]interface{}{"title": "My Video"},
			input:     "title}}",
			expected:  "title}}", // Should remain unchanged
		},
		{
			name:      "nested braces",
			variables: map[string]interface{}{"title": "My Video"},
			input:     "{{{{title}}}}",
			expected:  "{{{{title}}}}", // Inner {{title}} should be substituted but outer preserved
		},
		{
			name:      "empty variable name",
			variables: map[string]interface{}{"": "Empty Key"},
			input:     "{{}}",
			expected:  "{{}}", // Should remain unchanged
		},
		{
			name:      "special characters in variable value",
			variables: map[string]interface{}{"special": "Value with {{braces}} and |pipes|"},
			input:     "{{special}}",
			expected:  "Value with {{braces}} and |pipes|",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vs := NewVariableSubstitutor(tt.variables)
			result, err := vs.Substitute(tt.input)
			if err != nil {
				t.Fatalf("Substitute failed: %v", err)
			}

			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}
