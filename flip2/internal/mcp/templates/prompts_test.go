package templates

import (
	"log/slog"
	"strings"
	"testing"
)

// TestPromptTemplateValidate tests template validation.
func TestPromptTemplateValidate(t *testing.T) {
	tests := []struct {
		name      string
		template  *PromptTemplate
		wantError bool
		errorMsg  string
	}{
		{
			name:      "nil template",
			template:  nil,
			wantError: true,
			errorMsg:  "template is nil",
		},
		{
			name: "empty name",
			template: &PromptTemplate{
				Name: "",
				Messages: []PromptMessage{
					{Role: "user", Content: MessageContent{Type: "text", Text: "hello"}},
				},
			},
			wantError: true,
			errorMsg:  "template name is required",
		},
		{
			name: "no messages",
			template: &PromptTemplate{
				Name:     "test",
				Messages: []PromptMessage{},
			},
			wantError: true,
			errorMsg:  "template must have at least one message",
		},
		{
			name: "invalid message role",
			template: &PromptTemplate{
				Name: "test",
				Messages: []PromptMessage{
					{Role: "invalid", Content: MessageContent{Type: "text", Text: "hello"}},
				},
			},
			wantError: true,
			errorMsg:  "invalid role",
		},
		{
			name: "missing content type",
			template: &PromptTemplate{
				Name: "test",
				Messages: []PromptMessage{
					{Role: "user", Content: MessageContent{Type: "", Text: "hello"}},
				},
			},
			wantError: true,
			errorMsg:  "content type is required",
		},
		{
			name: "text content empty",
			template: &PromptTemplate{
				Name: "test",
				Messages: []PromptMessage{
					{Role: "user", Content: MessageContent{Type: "text", Text: ""}},
				},
			},
			wantError: true,
			errorMsg:  "text content is required",
		},
		{
			name: "valid template",
			template: &PromptTemplate{
				Name:        "test",
				Description: "Test template",
				Messages: []PromptMessage{
					{Role: "user", Content: MessageContent{Type: "text", Text: "hello"}},
				},
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.template.Validate()
			if (err != nil) != tt.wantError {
				t.Errorf("Validate() error = %v, wantError %v", err, tt.wantError)
				return
			}
			if tt.wantError && !strings.Contains(err.Error(), tt.errorMsg) {
				t.Errorf("Validate() error = %v, want error containing %q", err, tt.errorMsg)
			}
		})
	}
}

// TestPromptTemplateGetArgumentByName tests argument lookup.
func TestPromptTemplateGetArgumentByName(t *testing.T) {
	template := &PromptTemplate{
		Name: "test",
		Arguments: []TemplateArgument{
			{Name: "language", Required: true},
			{Name: "code", Required: true},
			{Name: "focus", Required: false},
		},
	}

	tests := []struct {
		name   string
		argName string
		want   *TemplateArgument
	}{
		{
			name:    "find language",
			argName: "language",
			want:    &TemplateArgument{Name: "language", Required: true},
		},
		{
			name:    "find code",
			argName: "code",
			want:    &TemplateArgument{Name: "code", Required: true},
		},
		{
			name:    "find focus",
			argName: "focus",
			want:    &TemplateArgument{Name: "focus", Required: false},
		},
		{
			name:    "not found",
			argName: "nonexistent",
			want:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := template.GetArgumentByName(tt.argName)
			if tt.want == nil {
				if got != nil {
					t.Errorf("GetArgumentByName() = %v, want nil", got)
				}
				return
			}
			if got == nil || got.Name != tt.want.Name || got.Required != tt.want.Required {
				t.Errorf("GetArgumentByName() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestPromptTemplateRender tests argument substitution.
func TestPromptTemplateRender(t *testing.T) {
	defaultFocus := "best practices"
	template := &PromptTemplate{
		Name:        "code-review",
		Description: "Code review template",
		Arguments: []TemplateArgument{
			{Name: "language", Required: true},
			{Name: "code", Required: true},
			{Name: "focus", Required: false, Default: &defaultFocus},
		},
		Messages: []PromptMessage{
			{
				Role: "user",
				Content: MessageContent{
					Type: "text",
					Text: "Review {language} code:\n{code}\nFocus: {focus}",
				},
			},
		},
		logger: slog.Default(),
	}

	tests := []struct {
		name      string
		args      map[string]string
		wantError bool
		errorMsg  string
		wantText  string
	}{
		{
			name: "all required args provided",
			args: map[string]string{
				"language": "Go",
				"code":     "func main() {}",
				"focus":    "performance",
			},
			wantError: false,
			wantText:  "Review Go code:\nfunc main() {}\nFocus: performance",
		},
		{
			name: "missing required argument",
			args: map[string]string{
				"language": "Go",
			},
			wantError: true,
			errorMsg:  "required argument missing",
		},
		{
			name: "use default value",
			args: map[string]string{
				"language": "Python",
				"code":     "def main(): pass",
			},
			wantError: false,
			wantText:  "Review Python code:\ndef main(): pass\nFocus: best practices",
		},
		{
			name: "override default value",
			args: map[string]string{
				"language": "JavaScript",
				"code":     "function main() {}",
				"focus":    "security",
			},
			wantError: false,
			wantText:  "Review JavaScript code:\nfunction main() {}\nFocus: security",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := template.Render(tt.args)
			if (err != nil) != tt.wantError {
				t.Errorf("Render() error = %v, wantError %v", err, tt.wantError)
				return
			}
			if tt.wantError {
				if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Render() error = %v, want error containing %q", err, tt.errorMsg)
				}
				return
			}

			if len(got.Messages) == 0 {
				t.Errorf("Render() got no messages")
				return
			}

			if got.Messages[0].Content.Text != tt.wantText {
				t.Errorf("Render() text = %q, want %q", got.Messages[0].Content.Text, tt.wantText)
			}
		})
	}
}

// TestPromptTemplateRenderUnusedPlaceholders tests that unused placeholders are preserved.
func TestPromptTemplateRenderUnusedPlaceholders(t *testing.T) {
	template := &PromptTemplate{
		Name: "test",
		Arguments: []TemplateArgument{
			{Name: "var1", Required: true},
		},
		Messages: []PromptMessage{
			{
				Role: "user",
				Content: MessageContent{
					Type: "text",
					Text: "Use {var1} and {var2}",
				},
			},
		},
		logger: slog.Default(),
	}

	args := map[string]string{"var1": "value1"}
	rendered, err := template.Render(args)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	expected := "Use value1 and {var2}"
	if rendered.Messages[0].Content.Text != expected {
		t.Errorf("Render() text = %q, want %q", rendered.Messages[0].Content.Text, expected)
	}
}

// TestPromptTemplateRenderUsedArguments tests tracking of used arguments.
func TestPromptTemplateRenderUsedArguments(t *testing.T) {
	template := &PromptTemplate{
		Name: "test",
		Arguments: []TemplateArgument{
			{Name: "a", Required: true},
			{Name: "b", Required: true},
		},
		Messages: []PromptMessage{
			{
				Role: "user",
				Content: MessageContent{
					Type: "text",
					Text: "Value A: {a}",
				},
			},
		},
		logger: slog.Default(),
	}

	args := map[string]string{"a": "val_a", "b": "val_b"}
	rendered, err := template.Render(args)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if !rendered.UsedArguments["a"] {
		t.Errorf("UsedArguments['a'] = false, want true")
	}
	if rendered.UsedArguments["b"] {
		t.Errorf("UsedArguments['b'] = true, want false")
	}
}

// TestPromptRegistryRegister tests template registration.
func TestPromptRegistryRegister(t *testing.T) {
	registry := NewPromptRegistry()

	template1 := &PromptTemplate{
		Name:        "test1",
		Description: "Test template 1",
		Messages: []PromptMessage{
			{Role: "user", Content: MessageContent{Type: "text", Text: "hello"}},
		},
	}

	// Register first template
	if err := registry.Register(template1); err != nil {
		t.Errorf("Register() error = %v", err)
	}

	// Try to register duplicate
	if err := registry.Register(template1); err == nil {
		t.Errorf("Register() duplicate should error")
	} else if !strings.Contains(err.Error(), "already registered") {
		t.Errorf("Register() error = %v, want 'already registered'", err)
	}

	// Register nil template
	if err := registry.Register(nil); err == nil {
		t.Errorf("Register(nil) should error")
	}

	// Register template with empty name
	if err := registry.Register(&PromptTemplate{
		Name:     "",
		Messages: []PromptMessage{{Role: "user", Content: MessageContent{Type: "text", Text: "test"}}},
	}); err == nil {
		t.Errorf("Register() empty name should error")
	}
}

// TestPromptRegistryLookup tests template lookup.
func TestPromptRegistryLookup(t *testing.T) {
	registry := NewPromptRegistry()

	template := &PromptTemplate{
		Name:        "mytemplate",
		Description: "My template",
		Messages: []PromptMessage{
			{Role: "user", Content: MessageContent{Type: "text", Text: "hello"}},
		},
	}

	registry.Register(template)

	// Lookup existing template
	got, err := registry.Lookup("mytemplate")
	if err != nil {
		t.Errorf("Lookup() error = %v", err)
	}
	if got == nil || got.Name != "mytemplate" {
		t.Errorf("Lookup() = %v, want template with name 'mytemplate'", got)
	}

	// Lookup non-existent template
	_, err = registry.Lookup("nonexistent")
	if err == nil {
		t.Errorf("Lookup() non-existent should error")
	} else if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Lookup() error = %v, want 'not found'", err)
	}
}

// TestPromptRegistryList tests listing all templates.
func TestPromptRegistryList(t *testing.T) {
	registry := NewPromptRegistry()

	templates := []*PromptTemplate{
		{
			Name:     "zebra",
			Messages: []PromptMessage{{Role: "user", Content: MessageContent{Type: "text", Text: "z"}}},
		},
		{
			Name:     "apple",
			Messages: []PromptMessage{{Role: "user", Content: MessageContent{Type: "text", Text: "a"}}},
		},
		{
			Name:     "mango",
			Messages: []PromptMessage{{Role: "user", Content: MessageContent{Type: "text", Text: "m"}}},
		},
	}

	for _, t := range templates {
		registry.Register(t)
	}

	// List should return all templates sorted by name
	got := registry.List()
	if len(got) != 3 {
		t.Errorf("List() length = %d, want 3", len(got))
	}

	expectedNames := []string{"apple", "mango", "zebra"}
	for i, expected := range expectedNames {
		if got[i].Name != expected {
			t.Errorf("List()[%d] name = %q, want %q", i, got[i].Name, expected)
		}
	}
}

// TestPromptRegistryUnregister tests template removal.
func TestPromptRegistryUnregister(t *testing.T) {
	registry := NewPromptRegistry()

	template := &PromptTemplate{
		Name:     "test",
		Messages: []PromptMessage{{Role: "user", Content: MessageContent{Type: "text", Text: "test"}}},
	}

	registry.Register(template)

	// Unregister existing
	if err := registry.Unregister("test"); err != nil {
		t.Errorf("Unregister() error = %v", err)
	}

	// Should not be found after unregistration
	if _, err := registry.Lookup("test"); err == nil {
		t.Errorf("Lookup() after unregister should error")
	}

	// Unregister non-existent
	if err := registry.Unregister("nonexistent"); err == nil {
		t.Errorf("Unregister() non-existent should error")
	}
}

// TestPromptRegistryExists tests existence check.
func TestPromptRegistryExists(t *testing.T) {
	registry := NewPromptRegistry()

	template := &PromptTemplate{
		Name:     "test",
		Messages: []PromptMessage{{Role: "user", Content: MessageContent{Type: "text", Text: "test"}}},
	}

	if registry.Exists("test") {
		t.Errorf("Exists() = true before register, want false")
	}

	registry.Register(template)

	if !registry.Exists("test") {
		t.Errorf("Exists() = false after register, want true")
	}

	registry.Unregister("test")

	if registry.Exists("test") {
		t.Errorf("Exists() = true after unregister, want false")
	}
}

// TestGetBuiltinTemplate tests retrieving built-in templates.
func TestGetBuiltinTemplate(t *testing.T) {
	tests := []struct {
		name      string
		wantError bool
		wantName  string
	}{
		{"code-review", false, "code-review"},
		{"debugging", false, "debugging"},
		{"testing", false, "testing"},
		{"nonexistent", true, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetBuiltinTemplate(tt.name)
			if (err != nil) != tt.wantError {
				t.Errorf("GetBuiltinTemplate() error = %v, wantError %v", err, tt.wantError)
				return
			}
			if !tt.wantError && got.Name != tt.wantName {
				t.Errorf("GetBuiltinTemplate() name = %q, want %q", got.Name, tt.wantName)
			}
		})
	}
}

// TestCodeReviewTemplate tests the code-review built-in template.
func TestCodeReviewTemplate(t *testing.T) {
	template, err := GetBuiltinTemplate("code-review")
	if err != nil {
		t.Fatalf("GetBuiltinTemplate() error = %v", err)
	}

	if err := template.Validate(); err != nil {
		t.Errorf("Validate() error = %v", err)
	}

	template.logger = slog.Default()

	// Test rendering with required arguments
	args := map[string]string{
		"language": "Go",
		"code":     "func main() {}",
	}

	rendered, err := template.Render(args)
	if err != nil {
		t.Errorf("Render() error = %v", err)
	}

	if !strings.Contains(rendered.Messages[0].Content.Text, "Go") {
		t.Errorf("Render() missing language substitution")
	}
	if !strings.Contains(rendered.Messages[0].Content.Text, "func main() {}") {
		t.Errorf("Render() missing code substitution")
	}
	if !strings.Contains(rendered.Messages[0].Content.Text, "general quality and best practices") {
		t.Errorf("Render() missing default focus_areas")
	}
}

// TestDebuggingTemplate tests the debugging built-in template.
func TestDebuggingTemplate(t *testing.T) {
	template, err := GetBuiltinTemplate("debugging")
	if err != nil {
		t.Fatalf("GetBuiltinTemplate() error = %v", err)
	}

	if err := template.Validate(); err != nil {
		t.Errorf("Validate() error = %v", err)
	}

	template.logger = slog.Default()

	args := map[string]string{
		"error_message": "nil pointer exception",
		"code":          "var x *int\nprint(x.value)",
		"language":      "Go",
	}

	rendered, err := template.Render(args)
	if err != nil {
		t.Errorf("Render() error = %v", err)
	}

	if !strings.Contains(rendered.Messages[0].Content.Text, "nil pointer exception") {
		t.Errorf("Render() missing error_message")
	}
}

// TestTestingTemplate tests the testing built-in template.
func TestTestingTemplate(t *testing.T) {
	template, err := GetBuiltinTemplate("testing")
	if err != nil {
		t.Fatalf("GetBuiltinTemplate() error = %v", err)
	}

	if err := template.Validate(); err != nil {
		t.Errorf("Validate() error = %v", err)
	}

	template.logger = slog.Default()

	args := map[string]string{
		"language": "Python",
		"code":     "def add(a, b): return a + b",
	}

	rendered, err := template.Render(args)
	if err != nil {
		t.Errorf("Render() error = %v", err)
	}

	if !strings.Contains(rendered.Messages[0].Content.Text, "Python") {
		t.Errorf("Render() missing language")
	}
}

// TestRegisterBuiltins tests registering all built-in templates.
func TestRegisterBuiltins(t *testing.T) {
	registry := NewPromptRegistry()

	if err := RegisterBuiltins(registry); err != nil {
		t.Errorf("RegisterBuiltins() error = %v", err)
	}

	// Verify all built-ins are registered
	expectedNames := []string{"code-review", "debugging", "testing"}
	for _, name := range expectedNames {
		if !registry.Exists(name) {
			t.Errorf("RegisterBuiltins() missing %q", name)
		}
	}

	// Verify list contains all
	templates := registry.List()
	if len(templates) != 3 {
		t.Errorf("RegisterBuiltins() list length = %d, want 3", len(templates))
	}
}

// TestPromptRegistryWithLogger tests registry with custom logger.
func TestPromptRegistryWithLogger(t *testing.T) {
	logger := slog.Default()
	registry := NewPromptRegistryWithLogger(logger)

	template := &PromptTemplate{
		Name:     "test",
		Messages: []PromptMessage{{Role: "user", Content: MessageContent{Type: "text", Text: "test"}}},
	}

	if err := registry.Register(template); err != nil {
		t.Errorf("Register() error = %v", err)
	}

	if !registry.Exists("test") {
		t.Errorf("Exists() = false, want true")
	}
}

// TestComplexTemplateRendering tests rendering with multiple messages.
func TestComplexTemplateRendering(t *testing.T) {
	template := &PromptTemplate{
		Name: "complex",
		Arguments: []TemplateArgument{
			{Name: "topic", Required: true},
			{Name: "details", Required: false},
		},
		Messages: []PromptMessage{
			{
				Role: "user",
				Content: MessageContent{
					Type: "text",
					Text: "Tell me about {topic}",
				},
			},
			{
				Role: "assistant",
				Content: MessageContent{
					Type: "text",
					Text: "I'll help with {topic}",
				},
			},
			{
				Role: "user",
				Content: MessageContent{
					Type: "text",
					Text: "Also consider: {details}",
				},
			},
		},
		logger: slog.Default(),
	}

	args := map[string]string{
		"topic":   "machine learning",
		"details": "neural networks",
	}

	rendered, err := template.Render(args)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if len(rendered.Messages) != 3 {
		t.Errorf("Render() message count = %d, want 3", len(rendered.Messages))
	}

	if !strings.Contains(rendered.Messages[0].Content.Text, "machine learning") {
		t.Errorf("Message 0: missing topic substitution")
	}

	if !strings.Contains(rendered.Messages[2].Content.Text, "neural networks") {
		t.Errorf("Message 2: missing details substitution")
	}
}

// TestArgumentValidation tests argument definition validation.
func TestArgumentValidation(t *testing.T) {
	template := &PromptTemplate{
		Name: "test",
		Arguments: []TemplateArgument{
			{Name: "required_arg", Required: true},
			{Name: "optional_arg", Required: false},
		},
		Messages: []PromptMessage{
			{Role: "user", Content: MessageContent{Type: "text", Text: "Hello {required_arg}"}},
		},
		logger: slog.Default(),
	}

	// Should fail without required argument
	_, err := template.Render(map[string]string{})
	if err == nil {
		t.Errorf("Render() should fail without required argument")
	}

	// Should succeed with required argument
	_, err = template.Render(map[string]string{"required_arg": "value"})
	if err != nil {
		t.Errorf("Render() error = %v", err)
	}
}
