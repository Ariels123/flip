// Package config provides FLIP2.md schema definitions and validation.
package config

import (
	"fmt"
	"regexp"
)

// FLIP2MDSchema defines the complete structure and validation rules for FLIP2.md configuration files.
// This schema enables per-project configuration with agent roles, custom commands, routing rules,
// and context management.
type FLIP2MDSchema struct {
	// Metadata section
	Metadata MetadataSchema `json:"metadata"`

	// Agent roles section
	Agents AgentsSchema `json:"agents"`

	// Custom commands section
	Commands CommandsSchema `json:"commands"`

	// Routing rules section
	Routing RoutingSchema `json:"routing"`

	// Context auto-load files section
	Context ContextSchema `json:"context"`

	// Resource limits section
	Limits ResourceLimitsSchema `json:"limits"`
}

// MetadataSchema defines project-level metadata.
type MetadataSchema struct {
	// Project name (required)
	Project struct {
		Type        string // string
		Description string // Human-readable project identifier
		Required    bool   // true
		MaxLength   int    // 256
	}

	// Project version (recommended)
	Version struct {
		Type        string // string, semantic versioning (X.Y.Z)
		Description string // Semantic version of config
		Required    bool   // false, defaults to "1.0"
		Pattern     string // "^\\d+\\.\\d+\\.\\d+$"
	}

	// Coordinator agent ID (recommended)
	Coordinator struct {
		Type        string // string
		Description string // Which agent coordinates tasks
		Required    bool   // false
		Pattern     string // Must match valid agent ID pattern
	}

	// Last update timestamp (optional)
	LastUpdated struct {
		Type        string // RFC3339 timestamp
		Description string // When config was last modified
		Required    bool   // false
		Format      string // "2006-01-02T15:04:05Z07:00"
	}
}

// AgentsSchema defines custom agent role configuration.
type AgentsSchema struct {
	Type        string // "array"
	Description string // "Custom agent roles for this project"

	Agent struct {
		// Role name (required)
		Name struct {
			Type        string // string
			Description string // Unique role name
			Required    bool   // true
			MaxLength   int    // 50
		}

		// ID pattern for agents (required)
		IDPattern struct {
			Type        string // string, glob pattern
			Description string // Pattern matching agent IDs (e.g., "analyst-*")
			Required    bool   // true
			Pattern     string // Must contain valid glob/regex
		}

		// Model to use (required)
		Model struct {
			Type        string // string enum
			Description string // LLM model to use (claude, gemini, etc.)
			Required    bool   // true
			Enum        []string // Allowed values: claude, gemini, gpt-4, etc.
		}

		// Capabilities (optional)
		Capabilities struct {
			Type        string // array of strings
			Description string // What this role can do
			Required    bool   // false
			Examples    []string // read-logs, external-api-calls, approve-changes, etc.
		}

		// Permissions (optional)
		Permissions struct {
			Type        string // array of strings
			Description string // What resources this role can access
			Required    bool   // false
			Examples    []string // read-inbox, send-signals, create-tasks, modify-own-tasks, etc.
		}

		// Max concurrent tasks (optional)
		MaxConcurrentTasks struct {
			Type        string // integer
			Description string // Maximum tasks this role can handle simultaneously
			Required    bool   // false
			Default     int    // 5
			Minimum     int    // 1
			Maximum     int    // 1000
		}

		// Escalation requirements (optional)
		EscalationRequiredFor struct {
			Type        string // array of strings
			Description string // Actions requiring approval/escalation
			Required    bool   // false
			Examples    []string // access-secrets, execute-destructive, etc.
		}

		// Cost budget (optional)
		CostBudgetPerHour struct {
			Type        string // float64 (USD)
			Description string // Maximum hourly cost for this role
			Required    bool   // false
			Minimum     float64 // 0.0
			Maximum     float64 // 1000.0
		}

		// Description (optional)
		Description struct {
			Type        string // string
			Description string // Human-readable role description
			Required    bool   // false
			MaxLength   int    // 500
		}
	}
}

// CommandsSchema defines custom slash commands for the project.
type CommandsSchema struct {
	Type        string // "array"
	Description string // "Project-specific slash commands"

	Command struct {
		// Command name (required)
		Name struct {
			Type        string // string
			Description string // Command name with leading /
			Required    bool   // true
			Pattern     string // "^/[a-z-]+$"
		}

		// Aliases (optional)
		Aliases struct {
			Type        string // array of strings
			Description string // Alternative command names
			Required    bool   // false
		}

		// Handler (required)
		Handler struct {
			Type        string // string
			Description string // Script path or handler identifier
			Required    bool   // true
		}

		// Arguments specification (optional)
		Args struct {
			Type        string // string
			Description string // Argument pattern (e.g., "<dataset> [--format=json|csv]")
			Required    bool   // false
		}

		// Description (optional)
		Description struct {
			Type        string // string
			Description string // What the command does
			Required    bool   // false
			MaxLength   int    // 500
		}

		// Requires approval (optional)
		RequiresApproval struct {
			Type        string // boolean
			Description string // Whether coordinator approval is required
			Required    bool   // false
			Default     bool   // false
		}

		// Allowed roles (optional)
		AllowedRoles struct {
			Type        string // array of strings
			Description string // Which roles can execute this command
			Required    bool   // false
		}
	}
}

// RoutingSchema defines task routing rules based on conditions.
type RoutingSchema struct {
	Type        string // "array"
	Description string // "Task routing rules for agent/model selection"

	Route struct {
		// Route name (required)
		Name struct {
			Type        string // string
			Description string // Descriptive name for the rule
			Required    bool   // true
			MaxLength   int    // 100
		}

		// Condition (required)
		Condition struct {
			Type        string // string (expression)
			Description string // Expression evaluated to route task (e.g., "task.type == 'analysis' && task.complexity < 5")
			Required    bool   // true
			Examples    []string // Various expression examples
		}

		// Route destination (required)
		RouteTo struct {
			Type        string // string enum
			Description string // Target model or agent role (claude, gemini, analyst, etc.)
			Required    bool   // true
		}

		// Reason (optional)
		Reason struct {
			Type        string // string
			Description string // Why this routing decision is optimal
			Required    bool   // false
			MaxLength   int    // 500
		}

		// Cost impact (optional)
		CostImpact struct {
			Type        string // float64 (cost modifier)
			Description string // Relative cost change (+0.50 = 50% more expensive)
			Required    bool   // false
			Minimum     float64 // -1.0
			Maximum     float64 // 10.0
		}
	}
}

// ContextSchema defines auto-loaded context files.
type ContextSchema struct {
	Type        string // "array"
	Description string // "Context files to auto-load for agents"

	ContextFile struct {
		// File path (required)
		Path struct {
			Type        string // string (file path)
			Description string // Relative path to file
			Required    bool   // true
		}

		// File description (optional)
		Description struct {
			Type        string // string
			Description string // What this file contains
			Required    bool   // false
			MaxLength   int    // 500
		}

		// Load weight/priority (optional)
		Weight struct {
			Type        string // string enum
			Description string // Loading priority (high, medium, low)
			Required    bool   // false
			Default     string // "medium"
			Enum        []string // high, medium, low
		}
	}
}

// ResourceLimitsSchema defines resource constraints for the project.
type ResourceLimitsSchema struct {
	// Maximum concurrent agents (optional)
	MaxAgents struct {
		Type        string // integer
		Description string // Maximum agents spawned simultaneously
		Required    bool   // false
		Default     int    // 10
		Minimum     int    // 1
		Maximum     int    // 1000
	}

	// Monthly budget (optional)
	MonthlyBudget struct {
		Type        string // float64 (USD)
		Description string // Total monthly cost limit
		Required    bool   // false
		Minimum     float64 // 0.0
	}

	// Default timeout for tasks (optional)
	DefaultTimeout struct {
		Type        string // duration (seconds)
		Description string // Default task timeout
		Required    bool   // false
		Default     int    // 3600 (1 hour)
		Minimum     int    // 1
		Maximum     int    // 86400 (24 hours)
	}
}

// ValidationResult represents validation errors and warnings.
type ValidationResult struct {
	Valid    bool
	Errors   []ValidationError
	Warnings []ValidationWarning
}

// ValidationError represents a schema violation.
type ValidationError struct {
	Field   string
	Message string
	Value   interface{}
}

// ValidationWarning represents a non-critical issue.
type ValidationWarning struct {
	Field   string
	Message string
	Value   interface{}
}

// ValidateProjectConfig validates a parsed project configuration against the schema.
func ValidateProjectConfig(config *ProjectConfig) *ValidationResult {
	result := &ValidationResult{
		Valid:    true,
		Errors:   []ValidationError{},
		Warnings: []ValidationWarning{},
	}

	// Validate metadata
	validateMetadata(config, result)

	// Validate agents
	validateAgents(config, result)

	// Validate commands
	validateCommands(config, result)

	// Validate routing rules
	validateRouting(config, result)

	// Validate context files
	validateContextFiles(config, result)

	// Set Valid flag based on errors
	result.Valid = len(result.Errors) == 0

	return result
}

func validateMetadata(config *ProjectConfig, result *ValidationResult) {
	if config.Project == "" {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "metadata.project",
			Message: "Project name is required",
			Value:   config.Project,
		})
	}

	if len(config.Project) > 256 {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "metadata.project",
			Message: "Project name exceeds maximum length of 256 characters",
			Value:   len(config.Project),
		})
	}

	// Version pattern: X.Y.Z
	if config.Version != "" {
		versionRe := regexp.MustCompile(`^\d+\.\d+\.\d+$`)
		if !versionRe.MatchString(config.Version) {
			result.Warnings = append(result.Warnings, ValidationWarning{
				Field:   "metadata.version",
				Message: "Version should follow semantic versioning (X.Y.Z)",
				Value:   config.Version,
			})
		}
	}
}

func validateAgents(config *ProjectConfig, result *ValidationResult) {
	seenIDPatterns := make(map[string]bool)

	for i, agent := range config.Agents {
		if agent.Name == "" {
			result.Errors = append(result.Errors, ValidationError{
				Field:   fmt.Sprintf("agents[%d].name", i),
				Message: "Agent name is required",
				Value:   "",
			})
		}

		if agent.IDPattern == "" {
			result.Errors = append(result.Errors, ValidationError{
				Field:   fmt.Sprintf("agents[%d].id_pattern", i),
				Message: "Agent ID pattern is required",
				Value:   agent.IDPattern,
			})
		}

		if seenIDPatterns[agent.IDPattern] {
			result.Errors = append(result.Errors, ValidationError{
				Field:   fmt.Sprintf("agents[%d].id_pattern", i),
				Message: "Duplicate agent ID pattern",
				Value:   agent.IDPattern,
			})
		}
		seenIDPatterns[agent.IDPattern] = true

		if agent.Model == "" {
			result.Errors = append(result.Errors, ValidationError{
				Field:   fmt.Sprintf("agents[%d].model", i),
				Message: "Agent model is required",
				Value:   "",
			})
		}

		if agent.MaxConcurrentTasks < 1 && agent.MaxConcurrentTasks != 0 {
			result.Errors = append(result.Errors, ValidationError{
				Field:   fmt.Sprintf("agents[%d].max_concurrent_tasks", i),
				Message: "Max concurrent tasks must be >= 1",
				Value:   agent.MaxConcurrentTasks,
			})
		}

		if agent.CostBudgetPerHour < 0 {
			result.Errors = append(result.Errors, ValidationError{
				Field:   fmt.Sprintf("agents[%d].cost_budget_per_hour", i),
				Message: "Cost budget must be >= 0",
				Value:   agent.CostBudgetPerHour,
			})
		}
	}
}

func validateCommands(config *ProjectConfig, result *ValidationResult) {
	seenCmds := make(map[string]bool)

	for i, cmd := range config.Commands {
		if cmd.Name == "" {
			result.Errors = append(result.Errors, ValidationError{
				Field:   fmt.Sprintf("commands[%d].name", i),
				Message: "Command name is required",
				Value:   "",
			})
		}

		if !regexp.MustCompile(`^/[a-z\-]+$`).MatchString(cmd.Name) {
			result.Errors = append(result.Errors, ValidationError{
				Field:   fmt.Sprintf("commands[%d].name", i),
				Message: "Command must start with / and contain only lowercase letters and hyphens",
				Value:   cmd.Name,
			})
		}

		if seenCmds[cmd.Name] {
			result.Errors = append(result.Errors, ValidationError{
				Field:   fmt.Sprintf("commands[%d].name", i),
				Message: "Duplicate command",
				Value:   cmd.Name,
			})
		}
		seenCmds[cmd.Name] = true

		if cmd.Handler == "" {
			result.Errors = append(result.Errors, ValidationError{
				Field:   fmt.Sprintf("commands[%d].handler", i),
				Message: "Command handler is required",
				Value:   "",
			})
		}
	}
}

func validateRouting(config *ProjectConfig, result *ValidationResult) {
	seenRoutes := make(map[string]bool)

	for i, route := range config.Routes {
		if route.Name == "" {
			result.Errors = append(result.Errors, ValidationError{
				Field:   fmt.Sprintf("routing[%d].name", i),
				Message: "Route name is required",
				Value:   "",
			})
		}

		if seenRoutes[route.Name] {
			result.Errors = append(result.Errors, ValidationError{
				Field:   fmt.Sprintf("routing[%d].name", i),
				Message: "Duplicate route name",
				Value:   route.Name,
			})
		}
		seenRoutes[route.Name] = true

		if route.Condition == "" {
			result.Errors = append(result.Errors, ValidationError{
				Field:   fmt.Sprintf("routing[%d].condition", i),
				Message: "Route condition is required",
				Value:   "",
			})
		}

		if route.RouteTo == "" {
			result.Errors = append(result.Errors, ValidationError{
				Field:   fmt.Sprintf("routing[%d].route_to", i),
				Message: "Route destination is required",
				Value:   "",
			})
		}

		if route.CostImpact < -1.0 || route.CostImpact > 10.0 {
			result.Warnings = append(result.Warnings, ValidationWarning{
				Field:   fmt.Sprintf("routing[%d].cost_impact", i),
				Message: "Cost impact is unusually high or low",
				Value:   route.CostImpact,
			})
		}
	}
}

func validateContextFiles(config *ProjectConfig, result *ValidationResult) {
	for i, file := range config.Context.AutoLoadFiles {
		if file.Path == "" {
			result.Errors = append(result.Errors, ValidationError{
				Field:   fmt.Sprintf("context[%d].path", i),
				Message: "Context file path is required",
				Value:   "",
			})
		}

		validWeights := map[string]bool{"low": true, "medium": true, "high": true}
		if file.Weight != "" && !validWeights[file.Weight] {
			result.Errors = append(result.Errors, ValidationError{
				Field:   fmt.Sprintf("context[%d].weight", i),
				Message: "Weight must be 'low', 'medium', or 'high'",
				Value:   file.Weight,
			})
		}
	}
}

// String returns a human-readable validation report.
func (vr *ValidationResult) String() string {
	if vr.Valid {
		return "Configuration is valid"
	}

	report := "Validation failed with errors:\n"
	for _, err := range vr.Errors {
		report += fmt.Sprintf("  - %s: %s (value: %v)\n", err.Field, err.Message, err.Value)
	}

	if len(vr.Warnings) > 0 {
		report += "\nWarnings:\n"
		for _, warn := range vr.Warnings {
			report += fmt.Sprintf("  - %s: %s (value: %v)\n", warn.Field, warn.Message, warn.Value)
		}
	}

	return report
}
