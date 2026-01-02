// Package templates provides prompt template management for MCP servers.
//
// This package implements a registry-based system for managing reusable prompt
// templates with argument substitution, validation, and built-in templates for
// common tasks like code review, debugging, and testing.
//
// # PromptTemplate Structure
//
// A PromptTemplate defines a reusable prompt with:
//   - Name: Unique identifier for the template
//   - Description: Human-readable explanation
//   - Arguments: List of named parameters with validation rules
//   - Messages: Template content with placeholder substitution
//
// # Argument Substitution
//
// Template messages can reference arguments using {argument_name} syntax.
// The Render method performs substitution and validates required arguments.
//
// Example:
//
//	template := &PromptTemplate{
//	    Name: "code-review",
//	    Arguments: []TemplateArgument{
//	        {Name: "language", Description: "Programming language", Required: true},
//	        {Name: "focus_areas", Description: "Areas to focus on", Required: false},
//	    },
//	    Messages: []PromptMessage{{
//	        Role: "user",
//	        Content: MessageContent{
//	            Type: "text",
//	            Text: "Review this {language} code: {code}. Focus on: {focus_areas}",
//	        },
//	    }},
//	}
//
//	// Render with arguments
//	rendered, err := template.Render(map[string]string{
//	    "language": "Go",
//	    "code": "func main() { ... }",
//	    "focus_areas": "error handling",
//	})
//
// # Registry Operations
//
// The TemplateRegistry manages a collection of templates:
//
//	registry := NewPromptRegistry()
//	registry.Register(codeReviewTemplate)
//	template, err := registry.Lookup("code-review")
//	templates := registry.List()
//
// # Built-in Templates
//
// The package includes pre-configured templates for common use cases:
//   - code-review: Code quality analysis
//   - debugging: Error diagnosis and troubleshooting
//   - testing: Test case generation
//
// Access built-in templates via GetBuiltinTemplate or RegisterBuiltins.
package templates

import (
	"fmt"
	"log/slog"
	"regexp"
	"sort"
	"sync"
)

// MessageContent represents content in a message (from mcp package)
type MessageContent struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	Data     string `json:"data,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
}

// PromptMessage represents a message in a prompt result (from mcp package)
type PromptMessage struct {
	Role    string         `json:"role"`
	Content MessageContent `json:"content"`
}

// TemplateArgument defines a parameter for a prompt template.
type TemplateArgument struct {
	// Name is the argument identifier.
	Name string

	// Description explains the argument.
	Description string

	// Required indicates if the argument must be provided.
	Required bool

	// Default is an optional default value if not provided.
	Default *string
}

// PromptTemplate represents a reusable prompt template with argument substitution.
type PromptTemplate struct {
	// Name uniquely identifies the template.
	Name string

	// Description explains the template.
	Description string

	// Arguments defines the template parameters.
	Arguments []TemplateArgument

	// Messages is the template content with placeholders.
	Messages []PromptMessage

	// logger is used for structured logging (optional).
	logger *slog.Logger
}

// RenderedPrompt is the result of rendering a template with arguments.
type RenderedPrompt struct {
	// Description is the template description.
	Description string

	// Messages are the rendered prompt messages.
	Messages []PromptMessage

	// UsedArguments tracks which arguments were substituted.
	UsedArguments map[string]bool
}

// PromptRegistry manages a collection of prompt templates.
type PromptRegistry interface {
	// Register adds a template to the registry.
	// Returns error if a template with the same name already exists.
	Register(template *PromptTemplate) error

	// Lookup retrieves a template by name.
	// Returns error if template not found.
	Lookup(name string) (*PromptTemplate, error)

	// List returns all registered templates, sorted by name.
	List() []*PromptTemplate

	// Unregister removes a template from the registry.
	Unregister(name string) error

	// Exists checks if a template with the given name is registered.
	Exists(name string) bool
}

// promptRegistryImpl implements PromptRegistry with thread-safe operations.
type promptRegistryImpl struct {
	mu        sync.RWMutex
	templates map[string]*PromptTemplate
	logger    *slog.Logger
}

// Compile-time check that promptRegistryImpl implements PromptRegistry interface.
var _ PromptRegistry = (*promptRegistryImpl)(nil)

// NewPromptRegistry creates a new prompt template registry.
func NewPromptRegistry() PromptRegistry {
	return &promptRegistryImpl{
		templates: make(map[string]*PromptTemplate),
		logger:    slog.Default(),
	}
}

// NewPromptRegistryWithLogger creates a new prompt template registry with a custom logger.
func NewPromptRegistryWithLogger(logger *slog.Logger) PromptRegistry {
	if logger == nil {
		logger = slog.Default()
	}
	return &promptRegistryImpl{
		templates: make(map[string]*PromptTemplate),
		logger:    logger,
	}
}

// Register adds a template to the registry.
func (r *promptRegistryImpl) Register(template *PromptTemplate) error {
	if template == nil {
		err := fmt.Errorf("cannot register nil template")
		r.logger.Error("template registration failed", slog.String("error", err.Error()))
		return err
	}

	if template.Name == "" {
		err := fmt.Errorf("template name cannot be empty")
		r.logger.Error("template registration failed", slog.String("error", err.Error()))
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.templates[template.Name]; exists {
		err := fmt.Errorf("template already registered: %s", template.Name)
		r.logger.Warn("template already exists", slog.String("name", template.Name))
		return err
	}

	// Set logger on template
	template.logger = r.logger

	r.templates[template.Name] = template
	r.logger.Info("template registered", slog.String("name", template.Name))

	return nil
}

// Lookup retrieves a template by name.
func (r *promptRegistryImpl) Lookup(name string) (*PromptTemplate, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	template, exists := r.templates[name]
	if !exists {
		err := fmt.Errorf("template not found: %s", name)
		r.logger.Debug("template lookup failed", slog.String("name", name), slog.String("error", err.Error()))
		return nil, err
	}

	return template, nil
}

// List returns all registered templates, sorted by name.
func (r *promptRegistryImpl) List() []*PromptTemplate {
	r.mu.RLock()
	defer r.mu.RUnlock()

	templates := make([]*PromptTemplate, 0, len(r.templates))
	for _, template := range r.templates {
		templates = append(templates, template)
	}

	// Sort by name for consistent ordering
	sort.Slice(templates, func(i, j int) bool {
		return templates[i].Name < templates[j].Name
	})

	return templates
}

// Unregister removes a template from the registry.
func (r *promptRegistryImpl) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.templates[name]; !exists {
		err := fmt.Errorf("template not found: %s", name)
		r.logger.Debug("template unregister failed", slog.String("name", name), slog.String("error", err.Error()))
		return err
	}

	delete(r.templates, name)
	r.logger.Info("template unregistered", slog.String("name", name))

	return nil
}

// Exists checks if a template is registered.
func (r *promptRegistryImpl) Exists(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, exists := r.templates[name]
	return exists
}

// Validate checks if the template is properly configured.
// Returns error if required fields are missing or invalid.
func (t *PromptTemplate) Validate() error {
	if t == nil {
		return fmt.Errorf("template is nil")
	}

	if t.Name == "" {
		return fmt.Errorf("template name is required")
	}

	if len(t.Messages) == 0 {
		return fmt.Errorf("template must have at least one message")
	}

	// Validate messages
	for i, msg := range t.Messages {
		if msg.Role == "" {
			return fmt.Errorf("message %d: role is required", i)
		}
		if msg.Role != "user" && msg.Role != "assistant" {
			return fmt.Errorf("message %d: invalid role %q, must be 'user' or 'assistant'", i, msg.Role)
		}
		if msg.Content.Type == "" {
			return fmt.Errorf("message %d: content type is required", i)
		}
		if msg.Content.Type == "text" && msg.Content.Text == "" {
			return fmt.Errorf("message %d: text content is required", i)
		}
	}

	if t.logger == nil {
		t.logger = slog.Default()
	}

	return nil
}

// GetArgumentByName retrieves an argument definition by name.
// Returns nil if not found.
func (t *PromptTemplate) GetArgumentByName(name string) *TemplateArgument {
	for i := range t.Arguments {
		if t.Arguments[i].Name == name {
			return &t.Arguments[i]
		}
	}
	return nil
}

// Render substitutes arguments into the template messages.
// Returns error if required arguments are missing.
func (t *PromptTemplate) Render(args map[string]string) (*RenderedPrompt, error) {
	if err := t.Validate(); err != nil {
		t.logger.Error("template validation failed", slog.String("template", t.Name), slog.String("error", err.Error()))
		return nil, err
	}

	// Check required arguments
	providedArgs := make(map[string]bool)
	for key := range args {
		providedArgs[key] = true
	}

	for _, argDef := range t.Arguments {
		if argDef.Required && !providedArgs[argDef.Name] {
			err := fmt.Errorf("required argument missing: %s", argDef.Name)
			t.logger.Error("argument validation failed", slog.String("template", t.Name), slog.String("error", err.Error()))
			return nil, err
		}
	}

	// Apply defaults for missing arguments
	resolvedArgs := make(map[string]string)
	for key, value := range args {
		resolvedArgs[key] = value
	}

	for _, argDef := range t.Arguments {
		if _, hasValue := resolvedArgs[argDef.Name]; !hasValue && argDef.Default != nil {
			resolvedArgs[argDef.Name] = *argDef.Default
			t.logger.Debug("using default argument", slog.String("template", t.Name), slog.String("argument", argDef.Name))
		}
	}

	// Render messages
	renderedMessages := make([]PromptMessage, len(t.Messages))
	usedArgs := make(map[string]bool)

	for i, msg := range t.Messages {
		renderedMsg := PromptMessage{
			Role: msg.Role,
			Content: MessageContent{
				Type:     msg.Content.Type,
				MimeType: msg.Content.MimeType,
				Data:     msg.Content.Data,
			},
		}

		if msg.Content.Type == "text" {
			renderedText := t.substituteArguments(msg.Content.Text, resolvedArgs, usedArgs)
			renderedMsg.Content.Text = renderedText
		}

		renderedMessages[i] = renderedMsg
	}

	t.logger.Debug("template rendered", slog.String("template", t.Name), slog.Any("usedArguments", usedArgs))

	return &RenderedPrompt{
		Description:   t.Description,
		Messages:      renderedMessages,
		UsedArguments: usedArgs,
	}, nil
}

// substituteArguments replaces {argument_name} placeholders with values.
func (t *PromptTemplate) substituteArguments(text string, args map[string]string, usedArgs map[string]bool) string {
	// Find all placeholders: {argument_name}
	pattern := regexp.MustCompile(`\{([a-zA-Z_][a-zA-Z0-9_]*)\}`)

	result := pattern.ReplaceAllStringFunc(text, func(match string) string {
		// Extract argument name from {argument_name}
		argName := match[1 : len(match)-1]

		if value, exists := args[argName]; exists {
			usedArgs[argName] = true
			return value
		}

		// Leave unsubstituted if not provided
		t.logger.Debug("placeholder not substituted", slog.String("template", t.Name), slog.String("argument", argName))
		return match
	})

	return result
}

// GetBuiltinTemplate retrieves a built-in template by name.
// Built-in templates: "code-review", "debugging", "testing".
// Returns error if template not found.
func GetBuiltinTemplate(name string) (*PromptTemplate, error) {
	switch name {
	case "code-review":
		return newCodeReviewTemplate(), nil
	case "debugging":
		return newDebuggingTemplate(), nil
	case "testing":
		return newTestingTemplate(), nil
	default:
		return nil, fmt.Errorf("unknown built-in template: %s", name)
	}
}

// RegisterBuiltins registers all built-in templates in the given registry.
func RegisterBuiltins(registry PromptRegistry) error {
	builtins := []string{"code-review", "debugging", "testing"}

	for _, name := range builtins {
		template, _ := GetBuiltinTemplate(name)
		if err := registry.Register(template); err != nil {
			return fmt.Errorf("failed to register built-in template %q: %w", name, err)
		}
	}

	return nil
}

// newCodeReviewTemplate creates the code-review built-in template.
func newCodeReviewTemplate() *PromptTemplate {
	return &PromptTemplate{
		Name:        "code-review",
		Description: "Review code for quality, performance, and best practices",
		Arguments: []TemplateArgument{
			{
				Name:        "language",
				Description: "Programming language of the code",
				Required:    true,
			},
			{
				Name:        "code",
				Description: "The code to review",
				Required:    true,
			},
			{
				Name:        "focus_areas",
				Description: "Specific areas to focus the review on (e.g., performance, security, readability)",
				Required:    false,
				Default:     stringPtr("general quality and best practices"),
			},
			{
				Name:        "context",
				Description: "Additional context about the code purpose or constraints",
				Required:    false,
			},
		},
		Messages: []PromptMessage{
			{
				Role: "user",
				Content: MessageContent{
					Type: "text",
					Text: `Please review the following {language} code for {focus_areas}.

Code:
{code}

{context}

Provide specific feedback on improvements needed, potential bugs, performance issues, and adherence to best practices.`,
				},
			},
		},
	}
}

// newDebuggingTemplate creates the debugging built-in template.
func newDebuggingTemplate() *PromptTemplate {
	return &PromptTemplate{
		Name:        "debugging",
		Description: "Debug code to identify and fix errors",
		Arguments: []TemplateArgument{
			{
				Name:        "error_message",
				Description: "The error message or symptoms",
				Required:    true,
			},
			{
				Name:        "code",
				Description: "The code that is causing the error",
				Required:    true,
			},
			{
				Name:        "language",
				Description: "Programming language",
				Required:    true,
			},
			{
				Name:        "steps_taken",
				Description: "Debugging steps already taken",
				Required:    false,
			},
			{
				Name:        "environment",
				Description: "Environment details (OS, runtime version, dependencies)",
				Required:    false,
			},
		},
		Messages: []PromptMessage{
			{
				Role: "user",
				Content: MessageContent{
					Type: "text",
					Text: `I'm debugging a {language} application and need help.

Error:
{error_message}

Code:
{code}

Environment: {environment}

Steps already taken:
{steps_taken}

Please help me identify the root cause and provide a solution.`,
				},
			},
		},
	}
}

// newTestingTemplate creates the testing built-in template.
func newTestingTemplate() *PromptTemplate {
	return &PromptTemplate{
		Name:        "testing",
		Description: "Generate test cases for code",
		Arguments: []TemplateArgument{
			{
				Name:        "language",
				Description: "Programming language",
				Required:    true,
			},
			{
				Name:        "code",
				Description: "The code to write tests for",
				Required:    true,
			},
			{
				Name:        "test_framework",
				Description: "Testing framework to use (e.g., pytest, jest, golang testing)",
				Required:    false,
				Default:     stringPtr("standard"),
			},
			{
				Name:        "coverage_focus",
				Description: "Which aspects to focus on (e.g., happy path, edge cases, error handling)",
				Required:    false,
				Default:     stringPtr("comprehensive coverage including edge cases"),
			},
		},
		Messages: []PromptMessage{
			{
				Role: "user",
				Content: MessageContent{
					Type: "text",
					Text: `Generate comprehensive test cases for the following {language} code using {test_framework}.

Code:
{code}

Focus on: {coverage_focus}

Please generate tests that are:
- Clear and well-documented
- Testing both normal cases and edge cases
- Following {language} testing best practices`,
				},
			},
		},
	}
}

// stringPtr returns a pointer to a string.
func stringPtr(s string) *string {
	return &s
}
