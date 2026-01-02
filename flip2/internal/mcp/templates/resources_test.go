package templates

import (
	"strings"
	"testing"
)

// TestRegisterTemplate tests basic template registration.
func TestRegisterTemplate(t *testing.T) {
	tests := []struct {
		name        string
		template    *ResourceTemplate
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid file template",
			template: &ResourceTemplate{
				URITemplate: "file://{path}",
				Name:        "filesystem",
				Description: "Access files",
				MimeType:    "text/plain",
			},
			expectError: false,
		},
		{
			name: "valid database template",
			template: &ResourceTemplate{
				URITemplate: "db://{schema}/{table}",
				Name:        "database",
				Description: "Access database tables",
				MimeType:    "application/json",
			},
			expectError: false,
		},
		{
			name: "valid API template",
			template: &ResourceTemplate{
				URITemplate: "api://{endpoint}/{id}",
				Name:        "api_resource",
				Description: "Access API resources",
				MimeType:    "application/json",
			},
			expectError: false,
		},
		{
			name:        "nil template",
			template:    nil,
			expectError: true,
			errorMsg:    "invalid template",
		},
		{
			name: "empty URI template",
			template: &ResourceTemplate{
				URITemplate: "",
				Name:        "bad",
			},
			expectError: true,
			errorMsg:    "invalid template",
		},
		{
			name: "empty name",
			template: &ResourceTemplate{
				URITemplate: "file://{path}",
				Name:        "",
			},
			expectError: true,
			errorMsg:    "invalid template",
		},
		{
			name: "invalid URI template - unbalanced braces",
			template: &ResourceTemplate{
				URITemplate: "file://{path",
				Name:        "bad",
			},
			expectError: true,
			errorMsg:    "invalid template",
		},
		{
			name: "whitespace-only URI template",
			template: &ResourceTemplate{
				URITemplate: "   ",
				Name:        "bad",
			},
			expectError: true,
			errorMsg:    "invalid template",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewResourceTemplateRegistry()
			err := registry.Register(tt.template)

			if tt.expectError && err == nil {
				t.Errorf("expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tt.expectError && err != nil && !strings.Contains(err.Error(), tt.errorMsg) {
				t.Errorf("expected error containing %q, got: %v", tt.errorMsg, err)
			}
		})
	}
}

// TestDuplicateTemplateRegistration tests that duplicate templates are rejected.
func TestDuplicateTemplateRegistration(t *testing.T) {
	registry := NewResourceTemplateRegistry()

	template := &ResourceTemplate{
		URITemplate: "file://{path}",
		Name:        "filesystem",
		Description: "Access files",
		MimeType:    "text/plain",
	}

	// First registration should succeed
	if err := registry.Register(template); err != nil {
		t.Fatalf("first registration failed: %v", err)
	}

	// Second registration with same URI should fail
	err := registry.Register(&ResourceTemplate{
		URITemplate: "file://{path}",
		Name:        "filesystem2",
		Description: "Different name, same URI",
		MimeType:    "text/plain",
	})

	if err == nil {
		t.Error("expected error for duplicate URI template")
	}
	if !strings.Contains(err.Error(), "already registered") {
		t.Errorf("expected 'already registered' error, got: %v", err)
	}
}

// TestLookup tests template lookup by URI template string.
func TestLookup(t *testing.T) {
	registry := NewResourceTemplateRegistry()

	template := &ResourceTemplate{
		URITemplate: "file://{path}",
		Name:        "filesystem",
		Description: "Access files",
		MimeType:    "text/plain",
	}

	registry.Register(template)

	// Lookup existing template
	found := registry.Lookup("file://{path}")
	if found == nil {
		t.Fatal("template not found")
	}
	if found.Name != "filesystem" {
		t.Errorf("expected name 'filesystem', got %q", found.Name)
	}

	// Lookup non-existent template
	notFound := registry.Lookup("db://{schema}")
	if notFound != nil {
		t.Error("expected nil for non-existent template")
	}
}

// TestLookupByName tests template lookup by name.
func TestLookupByName(t *testing.T) {
	registry := NewResourceTemplateRegistry()

	template := &ResourceTemplate{
		URITemplate: "file://{path}",
		Name:        "filesystem",
		Description: "Access files",
		MimeType:    "text/plain",
	}

	registry.Register(template)

	// Lookup existing template
	found := registry.LookupByName("filesystem")
	if found == nil {
		t.Fatal("template not found by name")
	}
	if found.URITemplate != "file://{path}" {
		t.Errorf("expected URI template 'file://{path}', got %q", found.URITemplate)
	}

	// Lookup non-existent template
	notFound := registry.LookupByName("database")
	if notFound != nil {
		t.Error("expected nil for non-existent template name")
	}
}

// TestFindMatches tests URI matching against templates.
func TestFindMatches(t *testing.T) {
	registry := NewResourceTemplateRegistry()

	templates := []*ResourceTemplate{
		{
			URITemplate: "file://{path}",
			Name:        "filesystem",
			Description: "Access files",
			MimeType:    "text/plain",
		},
		{
			URITemplate: "db://{schema}/{table}",
			Name:        "database",
			Description: "Access database tables",
			MimeType:    "application/json",
		},
		{
			URITemplate: "api://{endpoint}/{id}",
			Name:        "api",
			Description: "Access API resources",
			MimeType:    "application/json",
		},
	}

	for _, template := range templates {
		if err := registry.Register(template); err != nil {
			t.Fatalf("failed to register template: %v", err)
		}
	}

	tests := []struct {
		uri            string
		expectCount    int
		expectNames    []string
	}{
		{
			uri:         "file:///etc/hosts",
			expectCount: 1,
			expectNames: []string{"filesystem"},
		},
		{
			uri:         "db://public/users",
			expectCount: 1,
			expectNames: []string{"database"},
		},
		{
			uri:         "api://users/123",
			expectCount: 1,
			expectNames: []string{"api"},
		},
		{
			uri:         "unknown://something",
			expectCount: 0,
			expectNames: []string{},
		},
	}

	for _, tt := range tests {
		matches := registry.FindMatches(tt.uri)
		if len(matches) != tt.expectCount {
			t.Errorf("URI %q: expected %d matches, got %d", tt.uri, tt.expectCount, len(matches))
		}

		if len(matches) > 0 {
			got := make([]string, len(matches))
			for i, m := range matches {
				got[i] = m.Name
			}
			if !stringsEqual(got, tt.expectNames) {
				t.Errorf("URI %q: expected names %v, got %v", tt.uri, tt.expectNames, got)
			}
		}
	}
}

// TestValidateURI tests URI validation.
func TestValidateURI(t *testing.T) {
	registry := NewResourceTemplateRegistry()

	registry.Register(&ResourceTemplate{
		URITemplate: "file://{path}",
		Name:        "filesystem",
		Description: "Access files",
		MimeType:    "text/plain",
	})

	tests := []struct {
		uri      string
		valid    bool
	}{
		{"file:///etc/hosts", true},
		{"file:///tmp/test.txt", true},
		{"db://public/users", false},
		{"unknown://something", false},
	}

	for _, tt := range tests {
		valid := registry.ValidateURI(tt.uri)
		if valid != tt.valid {
			t.Errorf("URI %q: expected valid=%v, got %v", tt.uri, tt.valid, valid)
		}
	}
}

// TestList tests listing all templates.
func TestList(t *testing.T) {
	registry := NewResourceTemplateRegistry()

	templates := []*ResourceTemplate{
		{
			URITemplate: "file://{path}",
			Name:        "filesystem",
			MimeType:    "text/plain",
		},
		{
			URITemplate: "db://{schema}/{table}",
			Name:        "database",
			MimeType:    "application/json",
		},
	}

	for _, template := range templates {
		registry.Register(template)
	}

	list := registry.List()
	if len(list) != 2 {
		t.Errorf("expected 2 templates, got %d", len(list))
	}

	names := make([]string, len(list))
	for i, t := range list {
		names[i] = t.Name
	}
	if !stringsEqual(names, []string{"filesystem", "database"}) && !stringsEqual(names, []string{"database", "filesystem"}) {
		t.Errorf("expected names [filesystem, database], got %v", names)
	}
}

// TestClear tests clearing the registry.
func TestClear(t *testing.T) {
	registry := NewResourceTemplateRegistry()

	registry.Register(&ResourceTemplate{
		URITemplate: "file://{path}",
		Name:        "filesystem",
		MimeType:    "text/plain",
	})

	if registry.Count() != 1 {
		t.Errorf("expected 1 template, got %d", registry.Count())
	}

	registry.Clear()

	if registry.Count() != 0 {
		t.Errorf("expected 0 templates after clear, got %d", registry.Count())
	}
}

// TestCount tests counting templates.
func TestCount(t *testing.T) {
	registry := NewResourceTemplateRegistry()

	if registry.Count() != 0 {
		t.Errorf("expected 0 templates initially, got %d", registry.Count())
	}

	registry.Register(&ResourceTemplate{
		URITemplate: "file://{path}",
		Name:        "filesystem",
		MimeType:    "text/plain",
	})

	if registry.Count() != 1 {
		t.Errorf("expected 1 template, got %d", registry.Count())
	}
}

// TestParseURIVariables tests extracting variables from a URI.
func TestParseURIVariables(t *testing.T) {
	tests := []struct {
		name        string
		template    *ResourceTemplate
		uri         string
		expectError bool
		expectVars  map[string]string
	}{
		{
			name: "single variable",
			template: &ResourceTemplate{
				URITemplate: "file://{path}",
				Name:        "filesystem",
				MimeType:    "text/plain",
			},
			uri:         "file:///etc/hosts",
			expectError: false,
			expectVars: map[string]string{
				"path": "/etc/hosts",
			},
		},
		{
			name: "multiple variables",
			template: &ResourceTemplate{
				URITemplate: "db://{schema}/{table}",
				Name:        "database",
				MimeType:    "application/json",
			},
			uri:         "db://public/users",
			expectError: false,
			expectVars: map[string]string{
				"schema": "public",
				"table":  "users",
			},
		},
		{
			name: "three variables",
			template: &ResourceTemplate{
				URITemplate: "api://{endpoint}/{version}/{id}",
				Name:        "api",
				MimeType:    "application/json",
			},
			uri:         "api://users/v1/123",
			expectError: false,
			expectVars: map[string]string{
				"endpoint": "users",
				"version":  "v1",
				"id":       "123",
			},
		},
		{
			name: "URI does not match template",
			template: &ResourceTemplate{
				URITemplate: "file://{path}",
				Name:        "filesystem",
				MimeType:    "text/plain",
			},
			uri:         "db://public/users",
			expectError: true,
		},
		{
			name: "nil template",
			template: nil,
			uri: "file:///etc/hosts",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Register the template if not nil
			if tt.template != nil {
				registry := NewResourceTemplateRegistry()
				if err := registry.Register(tt.template); err != nil {
					t.Fatalf("failed to register template: %v", err)
				}
				tt.template = registry.Lookup(tt.template.URITemplate)
			}

			vars, err := ParseURIVariables(tt.template, tt.uri)

			if tt.expectError && err == nil {
				t.Errorf("expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if !tt.expectError && vars != nil {
				for key, expectedVal := range tt.expectVars {
					if val, ok := vars[key]; !ok {
						t.Errorf("missing variable %q", key)
					} else if val != expectedVal {
						t.Errorf("variable %q: expected %q, got %q", key, expectedVal, val)
					}
				}
			}
		})
	}
}

// TestIsValidURITemplate tests URI template validation.
func TestIsValidURITemplate(t *testing.T) {
	tests := []struct {
		template string
		valid    bool
	}{
		// Valid templates
		{"file://{path}", true},
		{"db://{schema}/{table}", true},
		{"api://{id}", true},
		{"http://{host}/api/{version}", true},
		{"{var}", true},
		{"/path/{var}/end", true},
		// Invalid templates
		{"", false},
		{"{unmatched", false},
		{"unmatched}", false},
		{"{nested{var}}", false},
		{"file://{{double}}", false},
	}

	for _, tt := range tests {
		valid := isValidURITemplate(tt.template)
		if valid != tt.valid {
			t.Errorf("template %q: expected valid=%v, got %v", tt.template, tt.valid, valid)
		}
	}
}

// TestCompileURITemplate tests URI template compilation.
func TestCompileURITemplate(t *testing.T) {
	tests := []struct {
		name       string
		template   string
		expectErr  bool
		shouldMatch []string
		shouldNotMatch []string
	}{
		{
			name:     "simple path variable",
			template: "file://{path}",
			shouldMatch: []string{
				"file:///etc/hosts",
				"file:///tmp/file.txt",
				"file:///home/user/documents",
			},
			shouldNotMatch: []string{
				"db:///etc/hosts",
				"file://",
			},
		},
		{
			name:     "multiple variables",
			template: "db://{schema}/{table}",
			shouldMatch: []string{
				"db://public/users",
				"db://private/accounts",
			},
			shouldNotMatch: []string{
				"db://",
				"db://public",
				"file://public/users",
			},
		},
		{
			name:     "three variables",
			template: "api://{endpoint}/{version}/{id}",
			shouldMatch: []string{
				"api://users/v1/123",
				"api://posts/v2/abc",
			},
			shouldNotMatch: []string{
				"api://users/v1",
				"api://users",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pattern, err := compileURITemplate(tt.template)
			if tt.expectErr && err != nil {
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			for _, uri := range tt.shouldMatch {
				if !pattern.MatchString(uri) {
					t.Errorf("template %q should match %q", tt.template, uri)
				}
			}

			for _, uri := range tt.shouldNotMatch {
				if pattern.MatchString(uri) {
					t.Errorf("template %q should not match %q", tt.template, uri)
				}
			}
		})
	}
}

// TestGetCommonSchemes tests the common schemes list.
func TestGetCommonSchemes(t *testing.T) {
	schemes := GetCommonSchemes()
	if len(schemes) == 0 {
		t.Error("expected non-empty schemes list")
	}

	// Check for expected schemes
	expectedSchemes := []string{"file", "db", "api", "http", "https"}
	for _, expected := range expectedSchemes {
		found := false
		for _, scheme := range schemes {
			if scheme == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected scheme %q in list", expected)
		}
	}
}

// TestGetCommonMimeTypes tests the common MIME types map.
func TestGetCommonMimeTypes(t *testing.T) {
	mimes := GetCommonMimeTypes()
	if len(mimes) == 0 {
		t.Error("expected non-empty MIME types map")
	}

	// Check for expected mappings
	expectedMimes := map[string]string{
		".txt":  "text/plain",
		".json": "application/json",
		".xml":  "application/xml",
	}

	for ext, expected := range expectedMimes {
		if actual, ok := mimes[ext]; !ok {
			t.Errorf("expected extension %q in map", ext)
		} else if actual != expected {
			t.Errorf("extension %q: expected %q, got %q", ext, expected, actual)
		}
	}
}

// TestTemplateMetadata tests template metadata handling.
func TestTemplateMetadata(t *testing.T) {
	registry := NewResourceTemplateRegistry()

	metadata := map[string]any{
		"encoding": "utf-8",
		"size":     1024,
		"readonly": true,
	}

	template := &ResourceTemplate{
		URITemplate: "file://{path}",
		Name:        "filesystem",
		Description: "Access files",
		MimeType:    "text/plain",
		Metadata:    metadata,
	}

	if err := registry.Register(template); err != nil {
		t.Fatalf("failed to register template: %v", err)
	}

	found := registry.Lookup("file://{path}")
	if found == nil {
		t.Fatal("template not found")
	}

	if found.Metadata["encoding"] != "utf-8" {
		t.Error("metadata not preserved")
	}
}

// TestConcurrentAccess tests concurrent access to the registry.
func TestConcurrentAccess(t *testing.T) {
	registry := NewResourceTemplateRegistry()

	// Register templates
	for i := 0; i < 10; i++ {
		template := &ResourceTemplate{
			URITemplate: "scheme" + string(rune('a'+i)) + "://{var}",
			Name:        "template" + string(rune('a'+i)),
			Description: "Test template",
			MimeType:    "text/plain",
		}
		if err := registry.Register(template); err != nil {
			t.Fatalf("failed to register template: %v", err)
		}
	}

	// Concurrent reads
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(i int) {
			_ = registry.List()
			_ = registry.Count()
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	if registry.Count() != 10 {
		t.Errorf("expected 10 templates, got %d", registry.Count())
	}
}

// TestPatternCompilation tests that patterns are properly compiled.
func TestPatternCompilation(t *testing.T) {
	registry := NewResourceTemplateRegistry()

	template := &ResourceTemplate{
		URITemplate: "file://{path}",
		Name:        "filesystem",
		Description: "Access files",
		MimeType:    "text/plain",
	}

	if err := registry.Register(template); err != nil {
		t.Fatalf("failed to register template: %v", err)
	}

	found := registry.Lookup("file://{path}")
	if found == nil {
		t.Fatal("template not found")
	}

	if found.Pattern == nil {
		t.Error("pattern not compiled during registration")
	}

	if !found.Pattern.MatchString("file:///etc/hosts") {
		t.Error("compiled pattern does not match expected URI")
	}
}

// Helper function to compare two string slices
func stringsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}
