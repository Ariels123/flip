package spawn

import (
	"fmt"
	"regexp"
	"strings"
)

// RoleSchemaVersion defines the current version of the role template schema.
const RoleSchemaVersion = "1.0.0"

// ResourcePattern defines a glob pattern for resource access control.
// Examples: "**/*.go", "config/public/*", "logs/worker/*"
type ResourcePattern string

// PermissionLevel defines the granularity of permission checks.
type PermissionLevel string

const (
	// PermissionLevelStrict enforces exact path matching for permissions
	PermissionLevelStrict PermissionLevel = "strict"
	// PermissionLevelGlob allows glob pattern matching for permissions
	PermissionLevelGlob PermissionLevel = "glob"
)

// RoleSchemaValidator provides validation rules and constraints for role templates.
// It ensures that custom roles conform to the defined schema and safety requirements.
type RoleSchemaValidator struct {
	// AllowedModels defines which LLM models can be used in roles.
	// If empty, all models are allowed.
	AllowedModels []string

	// RequiredPermissions defines permissions that every role must have.
	RequiredPermissions Permissions

	// MaxTokensLimit defines the maximum tokens a role can request.
	MaxTokensLimit int

	// ReservedRoleNames defines role names that cannot be used.
	ReservedRoleNames []string

	// PermissionLevel determines how resource patterns are validated.
	PermissionLevel PermissionLevel

	// ContextFileLimit defines the maximum number of context files a role can include.
	ContextFileLimit int

	// SystemPromptMinLength defines minimum characters required in system prompt.
	SystemPromptMinLength int

	// SystemPromptMaxLength defines maximum characters allowed in system prompt.
	SystemPromptMaxLength int
}

// RoleContext defines files and data that should be injected into a worker's context.
// This allows roles to have pre-loaded information without needing to read files at runtime.
type RoleContext struct {
	// Files defines file patterns to include in the worker's context.
	// Examples: []string{"CLAUDE.md", ".eslintrc", "internal/**/*.go"}
	Files []string `json:"files,omitempty"`

	// DataSources defines external data sources to provide to the worker.
	// Examples: []string{"git:history", "db:schema", "api:docs"}
	DataSources []string `json:"data_sources,omitempty"`

	// EnvironmentVariables defines which environment variables the role can access.
	// Examples: []string{"API_KEY", "DEBUG_MODE"}
	EnvironmentVariables []string `json:"environment_variables,omitempty"`
}

// ResourceLimit defines resource consumption constraints for a role.
type ResourceLimit struct {
	// TimeoutSeconds defines the maximum execution time in seconds.
	TimeoutSeconds int `json:"timeout_seconds,omitempty"`

	// TokenBudget defines the maximum total tokens this role can consume per execution.
	TokenBudget int `json:"token_budget,omitempty"`

	// ConcurrentExecutions defines how many workers with this role can run simultaneously.
	ConcurrentExecutions int `json:"concurrent_executions,omitempty"`

	// MaxRetries defines how many times the worker can retry failed operations.
	MaxRetries int `json:"max_retries,omitempty"`
}

// RoleSchemaDefinition provides the complete schema structure for role definitions.
// This is used to validate and document what fields are expected in custom role definitions.
type RoleSchemaDefinition struct {
	// Version specifies the schema version this role definition conforms to.
	Version string `json:"version"`

	// RequiredFields lists the fields that must be present in a role definition.
	RequiredFields []string `json:"required_fields"`

	// Fields describes each field in the role definition.
	Fields map[string]FieldDefinition `json:"fields"`

	// Examples provides example role definitions.
	Examples map[string]RoleTemplate `json:"examples,omitempty"`
}

// FieldDefinition describes a single field in a role definition.
type FieldDefinition struct {
	// Type describes the field type (string, object, array, etc.)
	Type string `json:"type"`

	// Description explains what this field is for.
	Description string `json:"description"`

	// Required indicates if this field must be present.
	Required bool `json:"required"`

	// Pattern provides a regex pattern for string validation.
	Pattern string `json:"pattern,omitempty"`

	// MinLength specifies minimum string length (for strings).
	MinLength int `json:"min_length,omitempty"`

	// MaxLength specifies maximum string length (for strings).
	MaxLength int `json:"max_length,omitempty"`

	// Enum lists allowed values for this field.
	Enum []string `json:"enum,omitempty"`

	// Default provides a default value if not specified.
	Default interface{} `json:"default,omitempty"`
}

// ValidateRoleNameFormat checks if a role name follows naming conventions.
// Valid names: alphanumeric with hyphens (e.g., "code-reviewer", "security-auditor")
func ValidateRoleNameFormat(name string) error {
	if name == "" {
		return fmt.Errorf("role name cannot be empty")
	}

	if len(name) > 128 {
		return fmt.Errorf("role name must be 128 characters or less, got %d", len(name))
	}

	// Check valid character set: alphanumeric and hyphens
	validPattern := regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)
	if !validPattern.MatchString(name) {
		return fmt.Errorf("role name must start with lowercase letter or digit, contain only lowercase letters, digits, and hyphens, and end with letter or digit: %q", name)
	}

	return nil
}

// ValidateSystemPrompt checks that a system prompt meets quality standards.
func ValidateSystemPrompt(prompt string, minLength, maxLength int) error {
	if prompt == "" {
		return fmt.Errorf("system prompt cannot be empty")
	}

	if len(prompt) < minLength {
		return fmt.Errorf("system prompt must be at least %d characters, got %d", minLength, len(prompt))
	}

	if len(prompt) > maxLength {
		return fmt.Errorf("system prompt must be at most %d characters, got %d", maxLength, len(prompt))
	}

	return nil
}

// ValidateModel checks if a model name is valid.
func ValidateModel(model string, allowedModels []string) error {
	if model == "" {
		// Model is optional - system will use default
		return nil
	}

	if len(allowedModels) > 0 {
		// If allowed models list is defined, check against it
		for _, allowed := range allowedModels {
			if model == allowed {
				return nil
			}
		}
		return fmt.Errorf("model %q is not in the list of allowed models: %v", model, allowedModels)
	}

	// Basic validation: model name should be non-empty and reasonable
	if len(model) > 256 {
		return fmt.Errorf("model name is too long: %q", model)
	}

	return nil
}

// ValidateResourcePattern checks if a resource pattern is well-formed.
func ValidateResourcePattern(pattern string) error {
	if pattern == "" {
		return fmt.Errorf("resource pattern cannot be empty")
	}

	// Allow common glob patterns
	validChars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789._-/*:"
	for _, ch := range pattern {
		if !strings.ContainsRune(validChars, ch) {
			return fmt.Errorf("invalid character in resource pattern: %q", ch)
		}
	}

	return nil
}

// ValidatePermissions checks that permissions are well-formed and sensible.
func ValidatePermissions(perms Permissions) error {
	// Validate read patterns
	for _, pattern := range perms.CanRead {
		if err := ValidateResourcePattern(pattern); err != nil {
			return fmt.Errorf("invalid read permission pattern: %w", err)
		}
	}

	// Validate write patterns
	for _, pattern := range perms.CanWrite {
		if err := ValidateResourcePattern(pattern); err != nil {
			return fmt.Errorf("invalid write permission pattern: %w", err)
		}
	}

	// Validate execute permissions - should be in format "namespace:action"
	for _, perm := range perms.CanExecute {
		if !strings.Contains(perm, ":") {
			return fmt.Errorf("execute permission must be in format 'namespace:action', got: %q", perm)
		}
	}

	return nil
}

// ValidateResourceLimits checks that resource limits are reasonable.
func ValidateResourceLimits(limits ResourceLimit, maxTokensLimit int) error {
	if limits.TimeoutSeconds < 0 {
		return fmt.Errorf("timeout must be non-negative, got %d", limits.TimeoutSeconds)
	}

	if limits.TimeoutSeconds > 3600 {
		return fmt.Errorf("timeout must be 3600 seconds or less, got %d", limits.TimeoutSeconds)
	}

	if limits.TokenBudget < 0 {
		return fmt.Errorf("token budget must be non-negative, got %d", limits.TokenBudget)
	}

	if limits.TokenBudget > maxTokensLimit && maxTokensLimit > 0 {
		return fmt.Errorf("token budget %d exceeds limit %d", limits.TokenBudget, maxTokensLimit)
	}

	if limits.ConcurrentExecutions < 1 {
		return fmt.Errorf("concurrent executions must be at least 1, got %d", limits.ConcurrentExecutions)
	}

	if limits.ConcurrentExecutions > 100 {
		return fmt.Errorf("concurrent executions must be 100 or less, got %d", limits.ConcurrentExecutions)
	}

	if limits.MaxRetries < 0 {
		return fmt.Errorf("max retries must be non-negative, got %d", limits.MaxRetries)
	}

	if limits.MaxRetries > 5 {
		return fmt.Errorf("max retries should be 5 or less, got %d", limits.MaxRetries)
	}

	return nil
}

// ValidateRoleContext checks that context specifications are valid.
func ValidateRoleContext(ctx RoleContext, contextFileLimit int) error {
	if len(ctx.Files) > contextFileLimit && contextFileLimit > 0 {
		return fmt.Errorf("role defines %d context files, but limit is %d", len(ctx.Files), contextFileLimit)
	}

	// Validate each file pattern
	for _, file := range ctx.Files {
		if err := ValidateResourcePattern(file); err != nil {
			return fmt.Errorf("invalid context file pattern: %w", err)
		}
	}

	return nil
}

// GetSchemaDefinition returns the complete schema definition for role templates.
// This describes all valid fields and their constraints.
func GetSchemaDefinition() RoleSchemaDefinition {
	return RoleSchemaDefinition{
		Version: RoleSchemaVersion,
		RequiredFields: []string{
			"name",
			"description",
			"system_prompt",
			"max_tokens",
		},
		Fields: map[string]FieldDefinition{
			"name": {
				Type:        "string",
				Description: "Unique identifier for the role (lowercase alphanumeric with hyphens)",
				Required:    true,
				Pattern:     `^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`,
				MaxLength:   128,
			},
			"description": {
				Type:        "string",
				Description: "Human-readable description of the role's purpose",
				Required:    true,
				MinLength:   10,
				MaxLength:   512,
			},
			"system_prompt": {
				Type:        "string",
				Description: "System message that defines the agent's behavior and constraints",
				Required:    true,
				MinLength:   50,
				MaxLength:   8192,
			},
			"model": {
				Type:        "string",
				Description: "LLM model to use (e.g., 'claude-sonnet-4', 'gemini-2.5-pro')",
				Required:    false,
				MaxLength:   256,
			},
			"max_tokens": {
				Type:        "integer",
				Description: "Maximum tokens per completion (1-65536)",
				Required:    true,
			},
			"permissions": {
				Type:        "object",
				Description: "Access control for the role",
				Required:    false,
			},
			"context": {
				Type:        "object",
				Description: "Files and data to inject into the worker's context",
				Required:    false,
			},
			"resource_limits": {
				Type:        "object",
				Description: "Resource consumption constraints",
				Required:    false,
			},
		},
	}
}

// DefaultRoleSchemaValidator returns a validator with sensible defaults.
func DefaultRoleSchemaValidator() *RoleSchemaValidator {
	return &RoleSchemaValidator{
		AllowedModels: []string{
			"claude-opus-4-5",
			"claude-sonnet-4",
			"claude-haiku-4",
			"gemini-2.5-pro",
			"gemini-2.5-flash",
			"gpt-4",
		},
		RequiredPermissions: Permissions{
			CanRead:    []string{"**/*"},
			CanExecute: []string{"signal:send", "task:report"},
		},
		MaxTokensLimit:        65536,
		ReservedRoleNames:     []string{"coordinator", "system", "default"},
		PermissionLevel:       PermissionLevelGlob,
		ContextFileLimit:      50,
		SystemPromptMinLength: 50,
		SystemPromptMaxLength: 8192,
	}
}

// ValidateRoleWithSchema validates a role template against the full schema.
func ValidateRoleWithSchema(role *RoleTemplate, validator *RoleSchemaValidator) error {
	if role == nil {
		return fmt.Errorf("role template cannot be nil")
	}

	// Check reserved names
	for _, reserved := range validator.ReservedRoleNames {
		if role.Name == reserved {
			return fmt.Errorf("role name %q is reserved and cannot be used", role.Name)
		}
	}

	// Validate name format
	if err := ValidateRoleNameFormat(role.Name); err != nil {
		return fmt.Errorf("invalid role name: %w", err)
	}

	// Validate description
	if role.Description == "" {
		return fmt.Errorf("role description cannot be empty")
	}

	if len(role.Description) < 10 || len(role.Description) > 512 {
		return fmt.Errorf("role description must be 10-512 characters, got %d", len(role.Description))
	}

	// Validate system prompt
	if err := ValidateSystemPrompt(role.SystemPrompt, validator.SystemPromptMinLength, validator.SystemPromptMaxLength); err != nil {
		return fmt.Errorf("invalid system prompt: %w", err)
	}

	// Validate model
	if err := ValidateModel(role.Model, validator.AllowedModels); err != nil {
		return fmt.Errorf("invalid model: %w", err)
	}

	// Validate max tokens
	if role.MaxTokens <= 0 {
		return fmt.Errorf("max_tokens must be positive, got %d", role.MaxTokens)
	}

	if role.MaxTokens > validator.MaxTokensLimit {
		return fmt.Errorf("max_tokens %d exceeds limit %d", role.MaxTokens, validator.MaxTokensLimit)
	}

	// Validate permissions
	if err := ValidatePermissions(role.Permissions); err != nil {
		return fmt.Errorf("invalid permissions: %w", err)
	}

	return nil
}

// ExampleSecurityAuditorRole provides an example role definition for a security auditor.
func ExampleSecurityAuditorRole() *RoleTemplate {
	return &RoleTemplate{
		Name:        "security-auditor",
		Description: "Security auditor focused on identifying vulnerabilities and security issues in code",
		SystemPrompt: `You are a security auditor in the FLIP2 system. Your coordinator manages task assignments and approvals.

**Role:** security-auditor
**Description:** Security auditor focused on identifying vulnerabilities and security issues in code

**Responsibilities:**
- Analyze code for security vulnerabilities and weaknesses
- Identify potential attack vectors and exploitation methods
- Review authentication, authorization, and data protection mechanisms
- Report findings to the coordinator with severity levels
- Escalate critical issues immediately
- Do not make autonomous decisions beyond your scope

**Capabilities:**
- Code review with security focus
- Vulnerability identification
- Suggest security improvements

**Actions Requiring Coordinator Approval:**
- Modify code
- Execute fixes
- Change security configurations

**Important Constraints:**
- Do not spawn additional agents without explicit coordinator approval
- Do not make decisions about resource allocation or budgeting
- Do not execute operations marked as requiring escalation
- Signal the coordinator if you encounter blockers
- Report all results in structured format with severity levels
- Do not make autonomous commits or deployments`,
		Model:     "claude-sonnet-4",
		MaxTokens: 8192,
		Permissions: Permissions{
			CanRead: []string{
				"**/*.go",
				"**/*.py",
				"**/*.js",
				"**/*.ts",
				"**/package.json",
				"**/requirements.txt",
				".eslintrc*",
				"*.config.js",
			},
			CanWrite: []string{
				"security-reviews/*.md",
			},
			CanExecute: []string{
				"signal:send",
				"task:report",
			},
		},
	}
}

// ExampleDataAnalystRole provides an example role definition for a data analyst.
func ExampleDataAnalystRole() *RoleTemplate {
	return &RoleTemplate{
		Name:        "data-analyst",
		Description: "Data analyst specialized in analyzing datasets and generating insights",
		SystemPrompt: `You are a data analyst in the FLIP2 system. Your coordinator manages task assignments and approvals.

**Role:** data-analyst
**Description:** Data analyst specialized in analyzing datasets and generating insights

**Responsibilities:**
- Analyze structured and unstructured data
- Identify patterns and trends
- Generate statistical insights and summaries
- Create data visualizations and reports
- Document findings and methodology
- Do not make autonomous decisions about data interpretation

**Capabilities:**
- Data analysis and processing
- Statistical modeling
- Pattern recognition
- Report generation

**Actions Requiring Coordinator Approval:**
- Data modification
- Database operations
- External data exports

**Important Constraints:**
- Do not modify source data without explicit approval
- Do not export sensitive data without authorization
- Signal the coordinator if you encounter data quality issues
- Report all analysis with confidence levels and limitations
- Do not make autonomous policy or business decisions based on analysis`,
		Model:     "gemini-2.5-pro",
		MaxTokens: 16384,
		Permissions: Permissions{
			CanRead: []string{
				"data/**/*.csv",
				"data/**/*.json",
				"data/**/*.parquet",
				"reports/*",
				"*.md",
			},
			CanWrite: []string{
				"analysis/**/*.md",
				"reports/**/*.html",
			},
			CanExecute: []string{
				"signal:send",
				"task:report",
			},
		},
	}
}

// ExamplePerformanceOptimizerRole provides an example role for performance optimization.
func ExamplePerformanceOptimizerRole() *RoleTemplate {
	return &RoleTemplate{
		Name:        "performance-optimizer",
		Description: "Performance optimizer focused on identifying and resolving performance bottlenecks",
		SystemPrompt: `You are a performance optimizer in the FLIP2 system. Your coordinator manages task assignments and approvals.

**Role:** performance-optimizer
**Description:** Performance optimizer focused on identifying and resolving performance bottlenecks

**Responsibilities:**
- Profile and analyze application performance
- Identify performance bottlenecks and optimization opportunities
- Suggest optimizations with estimated impact
- Review code for performance anti-patterns
- Generate performance reports and recommendations
- Do not make autonomous optimization changes

**Capabilities:**
- Performance profiling and analysis
- Bottleneck identification
- Optimization recommendation
- Benchmark creation

**Actions Requiring Coordinator Approval:**
- Code modification
- Infrastructure changes
- Production deployments

**Important Constraints:**
- Do not modify code without explicit coordinator approval
- Do not make changes to production systems
- Report all findings with performance metrics
- Provide before/after comparison estimates
- Do not make autonomous deployment decisions`,
		Model:     "claude-opus-4-5",
		MaxTokens: 12288,
		Permissions: Permissions{
			CanRead: []string{
				"**/*.go",
				"**/*.py",
				"**/*.js",
				"profiles/**/*",
				"benchmarks/**/*",
			},
			CanWrite: []string{
				"performance-reports/*.md",
				"optimization-suggestions/*.md",
			},
			CanExecute: []string{
				"signal:send",
				"task:report",
			},
		},
	}
}
