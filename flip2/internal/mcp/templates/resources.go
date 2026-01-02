// Package templates provides template management for MCP resources.
//
// The templates package handles resource templates from MCP servers, including
// URI pattern matching, validation, and registry operations. Resource templates
// allow servers to define dynamic resources that can be accessed via URI patterns.
//
// # Resource Templates
//
// Resource templates use RFC 6570 URI templates to define patterns for accessing
// resources dynamically. For example:
//
//	file://{filename}       - Access files by name
//	db://{schema}/{table}   - Access database tables
//	api://{endpoint}/{id}   - Access API resources by ID
//
// # Common Patterns
//
// The package supports common resource patterns:
//
//	- file://     File system resources
//	- db://       Database resources
//	- api://      HTTP API resources
//	- http://     Web resources
//	- https://    Secure web resources
//	- custom://   Custom application resources
//
// # Template Registry
//
// The TemplateRegistry provides centralized management of resource templates,
// supporting registration, lookup, and validation operations.
//
// # Example Usage
//
//	registry := templates.NewResourceTemplateRegistry()
//
//	// Register a resource template
//	template := &mcp.ResourceTemplate{
//	    URITemplate: "file://{path}",
//	    Name:        "filesystem",
//	    Description: "Access files by path",
//	    MimeType:    "text/plain",
//	}
//	if err := registry.Register(template); err != nil {
//	    log.Fatal(err)
//	}
//
//	// Find matching template for a URI
//	matches := registry.FindMatches("file:///etc/hosts")
//	for _, match := range matches {
//	    fmt.Printf("Template: %s\n", match.Name)
//	}
//
//	// Validate a URI against templates
//	if !registry.ValidateURI("file:///invalid") {
//	    fmt.Println("URI does not match any template")
//	}
package templates

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
)

// ResourceTemplate represents a URI template for dynamic resources.
//
// Resource templates define patterns that can be used to access resources
// dynamically. The URITemplate field uses RFC 6570 URI template syntax.
type ResourceTemplate struct {
	// URITemplate is an RFC 6570 URI template string.
	// Examples: "file://{path}", "db://{schema}/{table}", "api://{id}"
	URITemplate string

	// Name is a human-readable identifier for the template.
	Name string

	// Description explains the purpose and usage of this template.
	Description string

	// MimeType is the content type of resources accessed via this template.
	// Examples: "text/plain", "application/json", "application/octet-stream"
	MimeType string

	// Metadata contains additional template-specific information.
	Metadata map[string]any

	// Pattern is the compiled regex pattern derived from the URI template.
	// This field is populated by the registry during registration.
	Pattern *regexp.Regexp
}

// ResourceTemplateRegistry manages a collection of resource templates.
//
// The registry maintains resource templates from all connected MCP servers
// and provides lookup, validation, and matching operations.
//
// The registry is safe for concurrent use by multiple goroutines.
type ResourceTemplateRegistry struct {
	mu        sync.RWMutex
	templates map[string]*ResourceTemplate
	nameIndex map[string]*ResourceTemplate
}

// NewResourceTemplateRegistry creates a new, empty resource template registry.
func NewResourceTemplateRegistry() *ResourceTemplateRegistry {
	return &ResourceTemplateRegistry{
		templates: make(map[string]*ResourceTemplate),
		nameIndex: make(map[string]*ResourceTemplate),
	}
}

// Register adds a resource template to the registry.
//
// Register compiles the URI template into a regex pattern and stores
// the template for later lookup and matching operations.
//
// Returns an error if:
// - The URI template is invalid
// - A template with the same URI already exists
// - Pattern compilation fails
func (tr *ResourceTemplateRegistry) Register(template *ResourceTemplate) error {
	if err := validateTemplate(template); err != nil {
		return fmt.Errorf("invalid template: %w", err)
	}

	pattern, err := compileURITemplate(template.URITemplate)
	if err != nil {
		return fmt.Errorf("failed to compile URI template %q: %w", template.URITemplate, err)
	}

	template.Pattern = pattern

	tr.mu.Lock()
	defer tr.mu.Unlock()

	// Check for duplicate URI template
	if existing, exists := tr.templates[template.URITemplate]; exists {
		return fmt.Errorf("template for URI %q already registered (existing: %s)", template.URITemplate, existing.Name)
	}

	tr.templates[template.URITemplate] = template
	tr.nameIndex[template.Name] = template

	return nil
}

// Lookup retrieves a template by its URI template string.
//
// Returns nil if no template matches the URI template.
func (tr *ResourceTemplateRegistry) Lookup(uriTemplate string) *ResourceTemplate {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	return tr.templates[uriTemplate]
}

// LookupByName retrieves a template by its name.
//
// Returns nil if no template with that name exists.
func (tr *ResourceTemplateRegistry) LookupByName(name string) *ResourceTemplate {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	return tr.nameIndex[name]
}

// FindMatches returns all templates whose patterns match the given URI.
//
// A URI matches a template if it follows the pattern derived from the
// template's RFC 6570 URI template string.
//
// Returns an empty slice if no templates match.
func (tr *ResourceTemplateRegistry) FindMatches(uri string) []*ResourceTemplate {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	var matches []*ResourceTemplate
	for _, template := range tr.templates {
		if template.Pattern != nil && template.Pattern.MatchString(uri) {
			matches = append(matches, template)
		}
	}

	return matches
}

// ValidateURI checks if a URI matches any registered template.
//
// Returns true if at least one template's pattern matches the URI.
func (tr *ResourceTemplateRegistry) ValidateURI(uri string) bool {
	return len(tr.FindMatches(uri)) > 0
}

// List returns all registered templates.
func (tr *ResourceTemplateRegistry) List() []*ResourceTemplate {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	result := make([]*ResourceTemplate, 0, len(tr.templates))
	for _, template := range tr.templates {
		result = append(result, template)
	}

	return result
}

// Clear removes all templates from the registry.
func (tr *ResourceTemplateRegistry) Clear() {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	tr.templates = make(map[string]*ResourceTemplate)
	tr.nameIndex = make(map[string]*ResourceTemplate)
}

// Count returns the number of registered templates.
func (tr *ResourceTemplateRegistry) Count() int {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	return len(tr.templates)
}

// validateTemplate checks that a resource template has required fields.
func validateTemplate(template *ResourceTemplate) error {
	if template == nil {
		return fmt.Errorf("template cannot be nil")
	}

	if strings.TrimSpace(template.URITemplate) == "" {
		return fmt.Errorf("URITemplate cannot be empty")
	}

	if strings.TrimSpace(template.Name) == "" {
		return fmt.Errorf("Name cannot be empty")
	}

	// Validate URI template format (basic RFC 6570 compliance)
	if !isValidURITemplate(template.URITemplate) {
		return fmt.Errorf("URITemplate %q is not a valid RFC 6570 template", template.URITemplate)
	}

	return nil
}

// isValidURITemplate checks if a string is a valid RFC 6570 URI template.
func isValidURITemplate(template string) bool {
	// RFC 6570 templates can be empty or contain valid template expressions
	if len(template) == 0 {
		return false
	}

	// Check for balanced braces
	openBraces := strings.Count(template, "{")
	closeBraces := strings.Count(template, "}")

	if openBraces != closeBraces {
		return false
	}

	// Check for valid expression syntax (basic check)
	// Valid examples: {var}, {+var}, {#var}, etc.
	var inExpression bool
	for _, ch := range template {
		if ch == '{' {
			if inExpression {
				return false // nested braces
			}
			inExpression = true
		} else if ch == '}' {
			if !inExpression {
				return false // closing brace without opening
			}
			inExpression = false
		}
	}

	return !inExpression // should end with closed braces
}

// compileURITemplate converts an RFC 6570 URI template to a regex pattern.
//
// This function converts URI templates like "file://{path}" into regex patterns
// that can match concrete URIs. Variable parts are replaced with capture groups.
func compileURITemplate(template string) (*regexp.Regexp, error) {
	// First, extract variable names and their positions
	// RFC 6570 variables: {var}, {+var}, {#var}, etc.
	varRe := regexp.MustCompile(`\{[+#.;?&]?([^}]+)\}`)
	matches := varRe.FindAllStringSubmatchIndex(template, -1)

	if matches == nil {
		// No variables found, just escape the template as a literal regex
		escaped := regexp.QuoteMeta(template)
		pattern := "^" + escaped + "$"
		compiled, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("failed to compile pattern %q: %w", pattern, err)
		}
		return compiled, nil
	}

	// Build the pattern by processing the template with variables
	var result strings.Builder
	result.WriteString("^")

	lastEnd := 0
	for i, match := range matches {
		// match[0], match[1] = full match span
		// match[2], match[3] = first capture group span (variable name)
		start := match[0]
		end := match[1]
		varNameStart := match[2]
		varNameEnd := match[3]

		// Escape the literal part before this variable
		if start > lastEnd {
			literal := template[lastEnd:start]
			result.WriteString(regexp.QuoteMeta(literal))
		}

		// Get the variable name (without the braces and modifiers)
		varName := template[varNameStart:varNameEnd]

		// Determine the pattern for this variable based on what comes after
		isLast := i == len(matches)-1
		var varPattern string
		if isLast {
			// Last variable - match everything to the end
			varPattern = ".+"
		} else {
			// Not the last variable - match non-greedily until we find the next delimiter
			varPattern = ".+?"
		}

		// Add a named capture group
		result.WriteString("(?P<")
		result.WriteString(varName)
		result.WriteString(">")
		result.WriteString(varPattern)
		result.WriteString(")")

		lastEnd = end
	}

	// Escape any remaining literal part
	if lastEnd < len(template) {
		literal := template[lastEnd:]
		result.WriteString(regexp.QuoteMeta(literal))
	}

	result.WriteString("$")

	pattern := result.String()
	compiled, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to compile pattern %q: %w", pattern, err)
	}

	return compiled, nil
}

// ParseURIVariables extracts variable values from a concrete URI using a template.
//
// This function matches a concrete URI against a template and returns the
// extracted variable values.
//
// Returns a map of variable names to their values, or an error if the URI
// does not match the template.
func ParseURIVariables(template *ResourceTemplate, uri string) (map[string]string, error) {
	if template == nil {
		return nil, fmt.Errorf("template cannot be nil")
	}

	if template.Pattern == nil {
		return nil, fmt.Errorf("template pattern not compiled")
	}

	matches := template.Pattern.FindStringSubmatch(uri)
	if matches == nil {
		return nil, fmt.Errorf("URI %q does not match template %q", uri, template.URITemplate)
	}

	// Extract named groups
	variables := make(map[string]string)
	for i, name := range template.Pattern.SubexpNames() {
		if i > 0 && name != "" && i <= len(matches) {
			variables[name] = matches[i]
		}
	}

	return variables, nil
}

// GetCommonSchemes returns a list of common resource URI schemes.
//
// These are typical schemes used in resource URIs across MCP servers.
func GetCommonSchemes() []string {
	return []string{
		"file",      // Local filesystem
		"db",        // Database resources
		"api",       // HTTP API resources
		"http",      // Web resources (unencrypted)
		"https",     // Web resources (encrypted)
		"ws",        // WebSocket
		"wss",       // WebSocket Secure
		"custom",    // Custom application schemes
	}
}

// GetCommonMimeTypes returns a map of common file extensions to MIME types.
func GetCommonMimeTypes() map[string]string {
	return map[string]string{
		".txt":   "text/plain",
		".json":  "application/json",
		".xml":   "application/xml",
		".html":  "text/html",
		".csv":   "text/csv",
		".pdf":   "application/pdf",
		".zip":   "application/zip",
		".tar":   "application/x-tar",
		".gz":    "application/gzip",
		".md":    "text/markdown",
		".yaml":  "application/x-yaml",
		".yml":   "application/x-yaml",
		".jpg":   "image/jpeg",
		".jpeg":  "image/jpeg",
		".png":   "image/png",
		".gif":   "image/gif",
		".svg":   "image/svg+xml",
		".sql":   "application/sql",
		".proto": "application/protobuf",
	}
}
