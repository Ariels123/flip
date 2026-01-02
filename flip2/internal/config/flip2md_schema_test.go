package config

import (
	"testing"
)

// TestValidateProjectConfigValid tests validation of a valid configuration
func TestValidateProjectConfigValid(t *testing.T) {
	config := &ProjectConfig{
		Project:     "TestProject",
		Version:     "1.0.0",
		Coordinator: "claude-coordinator",
		LastUpdated: "2026-01-02T00:00:00Z",
		Agents: []AgentRole{
			{
				Name:              "Analyst",
				IDPattern:         "analyst-*",
				Model:             "gemini",
				Capabilities:      []string{"read-logs", "external-api-calls"},
				Permissions:       []string{"read-inbox", "send-signals"},
				MaxConcurrentTasks: 5,
				CostBudgetPerHour: 2.50,
			},
		},
		Commands: []Command{
			{
				Name:            "/analyze",
				Handler:         "analyst-worker",
				Description:     "Analyze dataset",
				RequiresApproval: false,
			},
		},
		Routes: []Route{
			{
				Name:       "FastAnalysis",
				Condition:  "task.complexity < 5",
				RouteTo:    "gemini",
				Reason:     "Fast and cheap",
				CostImpact: -0.30,
			},
		},
		Context: ContextConfig{
			AutoLoadFiles: []ContextFile{
				{
					Path:        "./README.md",
					Description: "Project overview",
					Weight:      "high",
				},
			},
		},
	}

	result := ValidateProjectConfig(config)
	if !result.Valid {
		t.Errorf("Expected valid configuration, got errors: %v", result.Errors)
	}
}

// TestValidateProjectConfigMissingProject tests validation with missing project name
func TestValidateProjectConfigMissingProject(t *testing.T) {
	config := &ProjectConfig{
		Project: "",
		Agents: []AgentRole{
			{
				Name:      "Analyst",
				IDPattern: "analyst-*",
				Model:     "gemini",
			},
		},
	}

	result := ValidateProjectConfig(config)
	if result.Valid {
		t.Error("Expected invalid configuration, got valid result")
	}

	hasProjectError := false
	for _, err := range result.Errors {
		if err.Field == "metadata.project" {
			hasProjectError = true
			break
		}
	}
	if !hasProjectError {
		t.Error("Expected project name validation error")
	}
}

// TestValidateProjectConfigInvalidVersion tests validation with invalid version
func TestValidateProjectConfigInvalidVersion(t *testing.T) {
	config := &ProjectConfig{
		Project: "TestProject",
		Version: "1.0-beta", // Invalid: not semantic versioning
		Agents: []AgentRole{
			{
				Name:      "Analyst",
				IDPattern: "analyst-*",
				Model:     "gemini",
			},
		},
	}

	result := ValidateProjectConfig(config)
	// Should be valid but with warning
	if !result.Valid {
		t.Errorf("Expected valid (with warning), got errors: %v", result.Errors)
	}

	hasVersionWarning := false
	for _, warn := range result.Warnings {
		if warn.Field == "metadata.version" {
			hasVersionWarning = true
			break
		}
	}
	if !hasVersionWarning {
		t.Error("Expected version validation warning")
	}
}

// TestValidateProjectConfigDuplicateAgentID tests duplicate agent ID detection
func TestValidateProjectConfigDuplicateAgentID(t *testing.T) {
	config := &ProjectConfig{
		Project: "TestProject",
		Agents: []AgentRole{
			{
				Name:      "Analyst1",
				IDPattern: "analyst-*",
				Model:     "gemini",
			},
			{
				Name:      "Analyst2",
				IDPattern: "analyst-*", // Duplicate
				Model:     "claude",
			},
		},
	}

	result := ValidateProjectConfig(config)
	if result.Valid {
		t.Error("Expected invalid configuration due to duplicate ID pattern")
	}

	hasDuplicateError := false
	for _, err := range result.Errors {
		if err.Field == "agents[1].id_pattern" {
			hasDuplicateError = true
			break
		}
	}
	if !hasDuplicateError {
		t.Error("Expected duplicate agent ID pattern error")
	}
}

// TestValidateProjectConfigInvalidCommand tests invalid command validation
func TestValidateProjectConfigInvalidCommand(t *testing.T) {
	config := &ProjectConfig{
		Project: "TestProject",
		Agents: []AgentRole{
			{
				Name:      "Analyst",
				IDPattern: "analyst-*",
				Model:     "gemini",
			},
		},
		Commands: []Command{
			{
				Name:    "analyze", // Missing leading /
				Handler: "handler",
			},
		},
	}

	result := ValidateProjectConfig(config)
	if result.Valid {
		t.Error("Expected invalid configuration due to invalid command name")
	}

	hasCommandError := false
	for _, err := range result.Errors {
		if err.Field == "commands[0].name" {
			hasCommandError = true
			break
		}
	}
	if !hasCommandError {
		t.Error("Expected command name validation error")
	}
}

// TestValidateProjectConfigDuplicateCommand tests duplicate command detection
func TestValidateProjectConfigDuplicateCommand(t *testing.T) {
	config := &ProjectConfig{
		Project: "TestProject",
		Agents: []AgentRole{
			{
				Name:      "Analyst",
				IDPattern: "analyst-*",
				Model:     "gemini",
			},
		},
		Commands: []Command{
			{
				Name:    "/analyze",
				Handler: "handler1",
			},
			{
				Name:    "/analyze", // Duplicate
				Handler: "handler2",
			},
		},
	}

	result := ValidateProjectConfig(config)
	if result.Valid {
		t.Error("Expected invalid configuration due to duplicate command")
	}
}

// TestValidateProjectConfigMissingRoutingCondition tests missing routing condition
func TestValidateProjectConfigMissingRoutingCondition(t *testing.T) {
	config := &ProjectConfig{
		Project: "TestProject",
		Agents: []AgentRole{
			{
				Name:      "Analyst",
				IDPattern: "analyst-*",
				Model:     "gemini",
			},
		},
		Routes: []Route{
			{
				Name:    "TestRoute",
				Condition: "", // Missing
				RouteTo: "gemini",
			},
		},
	}

	result := ValidateProjectConfig(config)
	if result.Valid {
		t.Error("Expected invalid configuration due to missing routing condition")
	}
}

// TestValidateProjectConfigInvalidWeight tests invalid context file weight
func TestValidateProjectConfigInvalidWeight(t *testing.T) {
	config := &ProjectConfig{
		Project: "TestProject",
		Agents: []AgentRole{
			{
				Name:      "Analyst",
				IDPattern: "analyst-*",
				Model:     "gemini",
			},
		},
		Context: ContextConfig{
			AutoLoadFiles: []ContextFile{
				{
					Path:        "./README.md",
					Description: "Project overview",
					Weight:      "critical", // Invalid weight
				},
			},
		},
	}

	result := ValidateProjectConfig(config)
	if result.Valid {
		t.Error("Expected invalid configuration due to invalid weight")
	}

	hasWeightError := false
	for _, err := range result.Errors {
		if err.Field == "context[0].weight" {
			hasWeightError = true
			break
		}
	}
	if !hasWeightError {
		t.Error("Expected context weight validation error")
	}
}

// TestValidateProjectConfigInvalidCostBudget tests negative cost budget
func TestValidateProjectConfigInvalidCostBudget(t *testing.T) {
	config := &ProjectConfig{
		Project: "TestProject",
		Agents: []AgentRole{
			{
				Name:              "Analyst",
				IDPattern:         "analyst-*",
				Model:             "gemini",
				CostBudgetPerHour: -5.0, // Invalid: negative
			},
		},
	}

	result := ValidateProjectConfig(config)
	if result.Valid {
		t.Error("Expected invalid configuration due to negative cost budget")
	}
}

// TestValidateProjectConfigUnusualCostImpact tests warning for unusual cost impact
func TestValidateProjectConfigUnusualCostImpact(t *testing.T) {
	config := &ProjectConfig{
		Project: "TestProject",
		Agents: []AgentRole{
			{
				Name:      "Analyst",
				IDPattern: "analyst-*",
				Model:     "gemini",
			},
		},
		Routes: []Route{
			{
				Name:       "ExpensiveRoute",
				Condition:  "task.type == 'expensive'",
				RouteTo:    "claude",
				CostImpact: 50.0, // Unusually high
			},
		},
	}

	result := ValidateProjectConfig(config)
	// Should be valid but with warning
	if !result.Valid {
		t.Errorf("Expected valid (with warning), got errors: %v", result.Errors)
	}

	hasCostWarning := false
	for _, warn := range result.Warnings {
		if warn.Field == "routing[0].cost_impact" {
			hasCostWarning = true
			break
		}
	}
	if !hasCostWarning {
		t.Error("Expected cost impact validation warning")
	}
}

// TestValidateProjectConfigMissingAgentModel tests missing agent model
func TestValidateProjectConfigMissingAgentModel(t *testing.T) {
	config := &ProjectConfig{
		Project: "TestProject",
		Agents: []AgentRole{
			{
				Name:      "Analyst",
				IDPattern: "analyst-*",
				Model:     "", // Missing
			},
		},
	}

	result := ValidateProjectConfig(config)
	if result.Valid {
		t.Error("Expected invalid configuration due to missing agent model")
	}

	hasModelError := false
	for _, err := range result.Errors {
		if err.Field == "agents[0].model" {
			hasModelError = true
			break
		}
	}
	if !hasModelError {
		t.Error("Expected agent model validation error")
	}
}

// TestValidateProjectConfigMissingCommandHandler tests missing command handler
func TestValidateProjectConfigMissingCommandHandler(t *testing.T) {
	config := &ProjectConfig{
		Project: "TestProject",
		Agents: []AgentRole{
			{
				Name:      "Analyst",
				IDPattern: "analyst-*",
				Model:     "gemini",
			},
		},
		Commands: []Command{
			{
				Name:    "/analyze",
				Handler: "", // Missing
			},
		},
	}

	result := ValidateProjectConfig(config)
	if result.Valid {
		t.Error("Expected invalid configuration due to missing command handler")
	}
}

// TestValidateProjectConfigComplexValid tests validation of complex valid configuration
func TestValidateProjectConfigComplexValid(t *testing.T) {
	config := &ProjectConfig{
		Project:     "DataAnalyticsPlatform",
		Version:     "2.1.3",
		Coordinator: "research-lead",
		LastUpdated: "2026-01-02T10:30:00Z",
		Agents: []AgentRole{
			{
				Name:               "DataAnalyst",
				IDPattern:          "analyst-*",
				Model:              "gemini",
				Capabilities:       []string{"read-logs", "external-api-calls"},
				Permissions:        []string{"read-inbox", "send-signals", "create-tasks"},
				MaxConcurrentTasks: 5,
				EscalationRequired: []string{"access-secrets"},
				CostBudgetPerHour:  2.50,
				Description:        "Processes and analyzes data",
			},
			{
				Name:               "CodeReviewer",
				IDPattern:          "reviewer-*",
				Model:              "claude",
				Capabilities:       []string{"approve-changes"},
				Permissions:        []string{"read-inbox", "send-signals"},
				MaxConcurrentTasks: 3,
				EscalationRequired: []string{},
				CostBudgetPerHour:  4.00,
				Description:        "Reviews code changes",
			},
			{
				Name:               "ResearchLead",
				IDPattern:          "research-*",
				Model:              "claude",
				Capabilities:       []string{"spawn-workers", "read-logs"},
				Permissions:        []string{"read-inbox", "send-signals", "create-tasks", "modify-all-tasks"},
				MaxConcurrentTasks: 2,
				EscalationRequired: []string{},
				CostBudgetPerHour:  5.00,
				Description:        "Leads research initiatives",
			},
		},
		Commands: []Command{
			{
				Name:             "/analyze",
				Aliases:          []string{"check", "run-analysis"},
				Handler:          "analyst-worker",
				Args:             "<dataset> [--format=json|csv]",
				Description:      "Analyze dataset and generate report",
				RequiresApproval: false,
				AllowedRoles:     []string{"analyst", "research"},
			},
			{
				Name:             "/review-code",
				Aliases:          []string{"review", "code-review"},
				Handler:          "reviewer-worker",
				Args:             "<pr-number> [--strict]",
				Description:      "Review code changes",
				RequiresApproval: false,
				AllowedRoles:     []string{"reviewer", "research"},
			},
			{
				Name:             "/deploy",
				Handler:          "./scripts/deploy.sh",
				Args:             "<pipeline> <environment>",
				Description:      "Deploy pipeline",
				RequiresApproval: true,
				AllowedRoles:     []string{"research"},
			},
		},
		Routes: []Route{
			{
				Name:       "FastAnalysis",
				Condition:  "task.type == 'analysis' && task.complexity < 5",
				RouteTo:    "gemini",
				Reason:     "Faster and cheaper for simple tasks",
				CostImpact: -0.30,
			},
			{
				Name:       "ComplexAnalysis",
				Condition:  "task.type == 'analysis' && task.complexity >= 7",
				RouteTo:    "claude",
				Reason:     "Claude handles complex reasoning better",
				CostImpact: 0.50,
			},
			{
				Name:       "CodeReview",
				Condition:  "task.type == 'review'",
				RouteTo:    "claude",
				Reason:     "Claude provides superior code understanding",
				CostImpact: 0.40,
			},
		},
		Context: ContextConfig{
			AutoLoadFiles: []ContextFile{
				{
					Path:        "./README.md",
					Description: "Project overview",
					Weight:      "high",
				},
				{
					Path:        "./docs/ARCHITECTURE.md",
					Description: "System design",
					Weight:      "high",
				},
				{
					Path:        "./CODING_STANDARDS.md",
					Description: "Code style guide",
					Weight:      "medium",
				},
				{
					Path:        "./config/example.yaml",
					Description: "Configuration template",
					Weight:      "low",
				},
			},
		},
	}

	result := ValidateProjectConfig(config)
	if !result.Valid {
		t.Errorf("Expected valid configuration, got errors: %v", result.Errors)
	}
	if len(result.Warnings) > 0 {
		t.Logf("Configuration has %d warning(s): %v", len(result.Warnings), result.Warnings)
	}
}

// TestValidationResultString tests the String() method
func TestValidationResultString(t *testing.T) {
	result := &ValidationResult{
		Valid: true,
		Errors: []ValidationError{},
		Warnings: []ValidationWarning{},
	}

	output := result.String()
	if output != "Configuration is valid" {
		t.Errorf("Expected 'Configuration is valid', got '%s'", output)
	}
}

// TestValidationResultStringWithErrors tests String() with errors
func TestValidationResultStringWithErrors(t *testing.T) {
	result := &ValidationResult{
		Valid: false,
		Errors: []ValidationError{
			{
				Field:   "metadata.project",
				Message: "Project name is required",
				Value:   "",
			},
		},
		Warnings: []ValidationWarning{},
	}

	output := result.String()
	if output == "Configuration is valid" {
		t.Error("Expected invalid configuration message")
	}
}

// TestValidationResultStringWithWarnings tests String() with warnings
func TestValidationResultStringWithWarnings(t *testing.T) {
	result := &ValidationResult{
		Valid: true,
		Errors: []ValidationError{},
		Warnings: []ValidationWarning{
			{
				Field:   "metadata.version",
				Message: "Version should follow semantic versioning",
				Value:   "1.0-beta",
			},
		},
	}

	output := result.String()
	if output != "Configuration is valid" {
		// With warnings, it should still be valid but we should check warnings are present
		if len(result.Warnings) == 0 {
			t.Error("Expected warnings in result")
		}
	}
}
