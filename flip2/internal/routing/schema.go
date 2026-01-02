// Package routing provides intelligent task routing between different AI models.
//
// This package implements a classification schema for tasks that enables
// cost-effective routing between Opus, Sonnet, Haiku, and Gemini models.
// The routing decisions are based on task type, complexity scoring, and
// risk assessment.
//
// Task ID: RTR-001
// Created: 2026-01-01
package routing

import (
	"fmt"
	"strings"
)

// ================================================================================
// TASK TYPE CLASSIFICATION
// ================================================================================

// TaskType represents the category of work being performed.
// This is the primary dimension for routing decisions.
type TaskType string

const (
	// Research and Information Gathering
	// Tasks that involve finding, reading, and synthesizing information.
	// Examples: API documentation research, codebase exploration, error investigation.
	// Preferred models: Gemini (cost-effective for bulk reading), Sonnet (synthesis)
	TaskTypeResearch TaskType = "research"

	// Code Generation and Implementation
	// Tasks that involve writing new code or features.
	// Examples: Implement new endpoint, create database schema, write algorithm.
	// Preferred models: Sonnet (standard), Opus (complex/novel architecture)
	TaskTypeCodeGeneration TaskType = "code_generation"

	// Code Review and Analysis
	// Tasks that involve reviewing existing code for issues.
	// Examples: PR review, security audit, performance analysis.
	// Preferred models: Sonnet (standard reviews), Opus (security-critical)
	TaskTypeCodeReview TaskType = "code_review"

	// Testing and Validation
	// Tasks that involve writing or running tests.
	// Examples: Unit tests, integration tests, test debugging.
	// Preferred models: Haiku (simple tests), Sonnet (complex test scenarios)
	TaskTypeTesting TaskType = "testing"

	// Documentation
	// Tasks that involve writing or updating documentation.
	// Examples: API docs, README updates, architecture diagrams.
	// Preferred models: Haiku (simple docs), Gemini (bulk documentation)
	TaskTypeDocumentation TaskType = "documentation"

	// Data Processing and Extraction
	// Tasks that involve parsing, transforming, or analyzing data.
	// Examples: Log analysis, data migration, JSON/CSV processing.
	// Preferred models: Gemini (bulk processing), Haiku (simple transforms)
	TaskTypeDataProcessing TaskType = "data_processing"

	// Debugging and Investigation
	// Tasks that involve finding and understanding bugs.
	// Examples: Stack trace analysis, reproduction steps, root cause analysis.
	// Preferred models: Sonnet (standard), Opus (complex multi-system bugs)
	TaskTypeDebugging TaskType = "debugging"

	// Refactoring
	// Tasks that involve restructuring existing code without changing behavior.
	// Examples: Extract method, rename variables, split files.
	// Preferred models: Haiku (simple), Sonnet (complex refactors)
	TaskTypeRefactoring TaskType = "refactoring"

	// Architecture and Design
	// Tasks that involve high-level system design decisions.
	// Examples: API design, schema design, system architecture.
	// Preferred models: Opus (always - high-stakes decisions)
	TaskTypeArchitecture TaskType = "architecture"

	// Configuration
	// Tasks that involve modifying configuration files.
	// Examples: YAML edits, environment variables, build configs.
	// Preferred models: Haiku (simple), Sonnet (complex multi-file)
	TaskTypeConfiguration TaskType = "configuration"

	// Deployment and DevOps
	// Tasks that involve deployment, CI/CD, or infrastructure.
	// Examples: Docker setup, CI pipeline, deployment scripts.
	// Preferred models: Sonnet (standard), Opus (production-critical)
	TaskTypeDeployment TaskType = "deployment"

	// Security
	// Tasks specifically focused on security concerns.
	// Examples: Vulnerability assessment, auth implementation, secrets management.
	// Preferred models: Opus (always - security is critical)
	TaskTypeSecurity TaskType = "security"

	// Visual and Browser
	// Tasks that require visual understanding or browser automation.
	// Examples: UI testing, screenshot analysis, web scraping.
	// Preferred models: Antigravity (human-in-loop), Gemini (simple visual)
	TaskTypeVisual TaskType = "visual"

	// Communication
	// Tasks that involve drafting messages or reports.
	// Examples: PR descriptions, commit messages, status updates.
	// Preferred models: Haiku (short), Sonnet (detailed)
	TaskTypeCommunication TaskType = "communication"

	// Pipeline and Orchestration
	// Tasks that involve coordinating multiple sub-tasks.
	// Examples: Multi-stage builds, research+implement flows.
	// Preferred models: Sonnet (standard), Opus (complex orchestration)
	TaskTypePipeline TaskType = "pipeline"
)

// AllTaskTypes returns all defined task types for iteration.
func AllTaskTypes() []TaskType {
	return []TaskType{
		TaskTypeResearch,
		TaskTypeCodeGeneration,
		TaskTypeCodeReview,
		TaskTypeTesting,
		TaskTypeDocumentation,
		TaskTypeDataProcessing,
		TaskTypeDebugging,
		TaskTypeRefactoring,
		TaskTypeArchitecture,
		TaskTypeConfiguration,
		TaskTypeDeployment,
		TaskTypeSecurity,
		TaskTypeVisual,
		TaskTypeCommunication,
		TaskTypePipeline,
	}
}

// IsValid checks if a TaskType is a recognized value.
func (t TaskType) IsValid() bool {
	for _, valid := range AllTaskTypes() {
		if t == valid {
			return true
		}
	}
	return false
}

// String returns the string representation of TaskType.
func (t TaskType) String() string {
	return string(t)
}

// Description returns a human-readable description of the task type.
func (t TaskType) Description() string {
	descriptions := map[TaskType]string{
		TaskTypeResearch:       "Research and information gathering",
		TaskTypeCodeGeneration: "Code generation and implementation",
		TaskTypeCodeReview:     "Code review and analysis",
		TaskTypeTesting:        "Testing and validation",
		TaskTypeDocumentation:  "Documentation writing",
		TaskTypeDataProcessing: "Data processing and extraction",
		TaskTypeDebugging:      "Debugging and investigation",
		TaskTypeRefactoring:    "Code refactoring",
		TaskTypeArchitecture:   "Architecture and design",
		TaskTypeConfiguration:  "Configuration management",
		TaskTypeDeployment:     "Deployment and DevOps",
		TaskTypeSecurity:       "Security-focused tasks",
		TaskTypeVisual:         "Visual and browser tasks",
		TaskTypeCommunication:  "Communication and reporting",
		TaskTypePipeline:       "Pipeline orchestration",
	}
	if desc, ok := descriptions[t]; ok {
		return desc
	}
	return "Unknown task type"
}

// ================================================================================
// COMPLEXITY SCORING
// ================================================================================

// ComplexityScore represents the multi-dimensional complexity assessment of a task.
type ComplexityScore struct {
	// TechnicalComplexity rates the inherent difficulty of the work (1-5).
	// 1: Trivial (variable rename, simple fix)
	// 2: Simple (standard implementation, clear requirements)
	// 3: Moderate (multiple files, some edge cases)
	// 4: Complex (cross-system, many interdependencies)
	// 5: Highly Complex (novel algorithms, architectural decisions)
	TechnicalComplexity int `json:"technical_complexity"`

	// ContextRequirements rates how much codebase knowledge is needed (1-5).
	// 1: None (standalone, self-contained)
	// 2: Minimal (one file, one module)
	// 3: Moderate (multiple modules, some history)
	// 4: Significant (cross-repo, deep understanding needed)
	// 5: Extensive (full system knowledge, historical context)
	ContextRequirements int `json:"context_requirements"`

	// RiskLevel rates the potential negative impact of mistakes (1-5).
	// 1: Negligible (easily caught, no user impact)
	// 2: Low (minor bugs, easy rollback)
	// 3: Moderate (user-facing, medium effort to fix)
	// 4: High (data integrity, security implications)
	// 5: Critical (production outage, data loss, security breach)
	RiskLevel int `json:"risk_level"`

	// Reversibility indicates how easily the change can be undone.
	// Higher values mean easier to reverse.
	// 1: Irreversible (data deletion, external API calls)
	// 2: Difficult (complex multi-step rollback)
	// 3: Moderate (standard revert, some cleanup)
	// 4: Easy (simple revert, no side effects)
	// 5: Trivial (read-only, no changes made)
	Reversibility int `json:"reversibility"`

	// EstimatedTokens is the rough estimate of tokens needed for this task.
	// Used for context window planning and cost estimation.
	EstimatedTokens int `json:"estimated_tokens,omitempty"`

	// RequiresHumanReview indicates if human approval is needed before execution.
	RequiresHumanReview bool `json:"requires_human_review"`
}

// OverallScore calculates a weighted overall complexity score.
// Returns a value between 1.0 and 5.0.
// Higher scores indicate more complex tasks that require better models.
func (c *ComplexityScore) OverallScore() float64 {
	// Weights: Technical complexity and risk are most important
	// Reversibility is inverted (lower reversibility = higher complexity)
	weights := struct {
		Technical     float64
		Context       float64
		Risk          float64
		Reversibility float64
	}{
		Technical:     0.35,
		Context:       0.20,
		Risk:          0.30,
		Reversibility: 0.15,
	}

	// Invert reversibility: 5 becomes 1, 1 becomes 5
	invertedReversibility := 6 - c.Reversibility

	score := weights.Technical*float64(c.TechnicalComplexity) +
		weights.Context*float64(c.ContextRequirements) +
		weights.Risk*float64(c.RiskLevel) +
		weights.Reversibility*float64(invertedReversibility)

	return score
}

// Validate ensures all scores are within valid ranges.
func (c *ComplexityScore) Validate() error {
	if c.TechnicalComplexity < 1 || c.TechnicalComplexity > 5 {
		return fmt.Errorf("technical_complexity must be 1-5, got %d", c.TechnicalComplexity)
	}
	if c.ContextRequirements < 1 || c.ContextRequirements > 5 {
		return fmt.Errorf("context_requirements must be 1-5, got %d", c.ContextRequirements)
	}
	if c.RiskLevel < 1 || c.RiskLevel > 5 {
		return fmt.Errorf("risk_level must be 1-5, got %d", c.RiskLevel)
	}
	if c.Reversibility < 1 || c.Reversibility > 5 {
		return fmt.Errorf("reversibility must be 1-5, got %d", c.Reversibility)
	}
	return nil
}

// NewComplexityScore creates a new ComplexityScore with validation.
func NewComplexityScore(technical, context, risk, reversibility int) (*ComplexityScore, error) {
	cs := &ComplexityScore{
		TechnicalComplexity: technical,
		ContextRequirements: context,
		RiskLevel:           risk,
		Reversibility:       reversibility,
	}
	if err := cs.Validate(); err != nil {
		return nil, err
	}
	return cs, nil
}

// ================================================================================
// MODEL DEFINITIONS
// ================================================================================

// Model represents an AI model that can execute tasks.
type Model string

const (
	// ModelOpus is the most capable model (Claude Opus).
	// Use for: architecture, security, complex debugging, novel algorithms.
	// Cost: Highest (~$0.15/1K tokens combined)
	ModelOpus Model = "opus"

	// ModelSonnet is the balanced model (Claude Sonnet).
	// Use for: standard implementation, code review, moderate complexity.
	// Cost: Medium (~$0.03/1K tokens combined)
	ModelSonnet Model = "sonnet"

	// ModelHaiku is the fast, efficient model (Claude Haiku).
	// Use for: simple tasks, tests, documentation, refactoring.
	// Cost: Low (~$0.01/1K tokens combined)
	ModelHaiku Model = "haiku"

	// ModelGemini is Google's model, good for bulk processing.
	// Use for: research, data processing, bulk documentation.
	// Cost: Low (~$0.02/1K tokens combined)
	ModelGemini Model = "gemini"

	// ModelAntigravity is the human-in-loop model for visual/browser tasks.
	// Use for: visual inspection, browser automation, high-stakes operations.
	// Cost: Variable (includes human time)
	ModelAntigravity Model = "antigravity"
)

// AllModels returns all available models.
func AllModels() []Model {
	return []Model{
		ModelOpus,
		ModelSonnet,
		ModelHaiku,
		ModelGemini,
		ModelAntigravity,
	}
}

// IsValid checks if a Model is a valid, recognized model.
func (m Model) IsValid() bool {
	for _, valid := range AllModels() {
		if m == valid {
			return true
		}
	}
	return false
}

// String returns the string representation of Model.
func (m Model) String() string {
	return string(m)
}

// ModelConfig contains configuration for a model.
type ModelConfig struct {
	Model             Model   `json:"model"`
	DisplayName       string  `json:"display_name"`
	InputCostPer1K    float64 `json:"input_cost_per_1k"`
	OutputCostPer1K   float64 `json:"output_cost_per_1k"`
	MaxContextTokens  int     `json:"max_context_tokens"`
	MaxOutputTokens   int     `json:"max_output_tokens"`
	SupportsStreaming bool    `json:"supports_streaming"`
	SupportsVision    bool    `json:"supports_vision"`
}

// DefaultModelConfigs returns the default configurations for all models.
func DefaultModelConfigs() map[Model]ModelConfig {
	return map[Model]ModelConfig{
		ModelOpus: {
			Model:             ModelOpus,
			DisplayName:       "Claude Opus 4.5",
			InputCostPer1K:    0.015,
			OutputCostPer1K:   0.075,
			MaxContextTokens:  200000,
			MaxOutputTokens:   32000,
			SupportsStreaming: true,
			SupportsVision:    true,
		},
		ModelSonnet: {
			Model:             ModelSonnet,
			DisplayName:       "Claude Sonnet 4",
			InputCostPer1K:    0.003,
			OutputCostPer1K:   0.015,
			MaxContextTokens:  200000,
			MaxOutputTokens:   16000,
			SupportsStreaming: true,
			SupportsVision:    true,
		},
		ModelHaiku: {
			Model:             ModelHaiku,
			DisplayName:       "Claude Haiku 3.5",
			InputCostPer1K:    0.001,
			OutputCostPer1K:   0.005,
			MaxContextTokens:  200000,
			MaxOutputTokens:   8000,
			SupportsStreaming: true,
			SupportsVision:    true,
		},
		ModelGemini: {
			Model:             ModelGemini,
			DisplayName:       "Gemini 2.0 Flash",
			InputCostPer1K:    0.0001,
			OutputCostPer1K:   0.0004,
			MaxContextTokens:  1000000,
			MaxOutputTokens:   8192,
			SupportsStreaming: true,
			SupportsVision:    true,
		},
		ModelAntigravity: {
			Model:             ModelAntigravity,
			DisplayName:       "Antigravity (Human-in-Loop)",
			InputCostPer1K:    0.0, // Variable
			OutputCostPer1K:   0.0, // Variable
			MaxContextTokens:  128000,
			MaxOutputTokens:   16000,
			SupportsStreaming: false,
			SupportsVision:    true,
		},
	}
}

// ================================================================================
// ROUTING DECISION MATRIX
// ================================================================================

// RoutingDecision represents the result of a routing analysis.
type RoutingDecision struct {
	// PrimaryModel is the recommended model for this task.
	PrimaryModel Model `json:"primary_model"`

	// FallbackModels are alternatives if primary is unavailable.
	FallbackModels []Model `json:"fallback_models"`

	// Confidence is how confident we are in this routing (0.0-1.0).
	Confidence float64 `json:"confidence"`

	// Reason explains why this model was chosen.
	Reason string `json:"reason"`

	// RequiresHumanApproval indicates if human should approve before execution.
	RequiresHumanApproval bool `json:"requires_human_approval"`

	// EstimatedCostUSD is the estimated cost for this task.
	EstimatedCostUSD float64 `json:"estimated_cost_usd,omitempty"`
}

// RoutingRule defines a rule for routing tasks to models.
type RoutingRule struct {
	// TaskType this rule applies to (or empty for all).
	TaskType TaskType `json:"task_type,omitempty"`

	// MinComplexity is the minimum overall complexity score to match.
	MinComplexity float64 `json:"min_complexity,omitempty"`

	// MaxComplexity is the maximum overall complexity score to match.
	MaxComplexity float64 `json:"max_complexity,omitempty"`

	// MinRiskLevel is the minimum risk level to match.
	MinRiskLevel int `json:"min_risk_level,omitempty"`

	// TargetModel is the model to use when this rule matches.
	TargetModel Model `json:"target_model"`

	// Priority determines which rule wins on conflicts (higher = wins).
	Priority int `json:"priority"`

	// Description explains the rule for debugging.
	Description string `json:"description"`
}

// DefaultRoutingRules returns the default routing rules.
// Rules are evaluated in priority order (highest first).
func DefaultRoutingRules() []RoutingRule {
	return []RoutingRule{
		// Critical/High-Risk Tasks -> Opus
		{
			MinRiskLevel: 5,
			TargetModel:  ModelOpus,
			Priority:     1000,
			Description:  "Critical risk tasks always use Opus",
		},
		{
			TaskType:    TaskTypeSecurity,
			TargetModel: ModelOpus,
			Priority:    900,
			Description: "Security tasks always use Opus",
		},
		{
			TaskType:    TaskTypeArchitecture,
			TargetModel: ModelOpus,
			Priority:    900,
			Description: "Architecture tasks always use Opus",
		},

		// Complex Tasks -> Opus
		{
			MinComplexity: 4.0,
			TargetModel:   ModelOpus,
			Priority:      800,
			Description:   "Highly complex tasks (score >= 4.0) use Opus",
		},

		// Visual Tasks -> Antigravity
		{
			TaskType:    TaskTypeVisual,
			TargetModel: ModelAntigravity,
			Priority:    700,
			Description: "Visual/browser tasks use Antigravity",
		},

		// Moderate Complexity -> Sonnet
		{
			MinComplexity: 2.5,
			MaxComplexity: 4.0,
			TargetModel:   ModelSonnet,
			Priority:      600,
			Description:   "Moderate complexity (2.5-4.0) uses Sonnet",
		},
		{
			TaskType:    TaskTypeCodeGeneration,
			TargetModel: ModelSonnet,
			Priority:    500,
			Description: "Code generation defaults to Sonnet",
		},
		{
			TaskType:    TaskTypeCodeReview,
			TargetModel: ModelSonnet,
			Priority:    500,
			Description: "Code review defaults to Sonnet",
		},
		{
			TaskType:    TaskTypeDebugging,
			TargetModel: ModelSonnet,
			Priority:    500,
			Description: "Debugging defaults to Sonnet",
		},
		{
			TaskType:    TaskTypeDeployment,
			TargetModel: ModelSonnet,
			Priority:    500,
			Description: "Deployment defaults to Sonnet",
		},
		{
			TaskType:    TaskTypePipeline,
			TargetModel: ModelSonnet,
			Priority:    500,
			Description: "Pipeline orchestration defaults to Sonnet",
		},

		// Bulk/Research Tasks -> Gemini
		{
			TaskType:    TaskTypeResearch,
			TargetModel: ModelGemini,
			Priority:    400,
			Description: "Research tasks use Gemini (cost-effective)",
		},
		{
			TaskType:    TaskTypeDataProcessing,
			TargetModel: ModelGemini,
			Priority:    400,
			Description: "Data processing uses Gemini (bulk efficiency)",
		},

		// Simple Tasks -> Haiku
		{
			MaxComplexity: 2.5,
			TargetModel:   ModelHaiku,
			Priority:      300,
			Description:   "Simple tasks (score < 2.5) use Haiku",
		},
		{
			TaskType:    TaskTypeTesting,
			TargetModel: ModelHaiku,
			Priority:    300,
			Description: "Testing defaults to Haiku",
		},
		{
			TaskType:    TaskTypeDocumentation,
			TargetModel: ModelHaiku,
			Priority:    300,
			Description: "Documentation defaults to Haiku",
		},
		{
			TaskType:    TaskTypeRefactoring,
			TargetModel: ModelHaiku,
			Priority:    300,
			Description: "Refactoring defaults to Haiku",
		},
		{
			TaskType:    TaskTypeConfiguration,
			TargetModel: ModelHaiku,
			Priority:    300,
			Description: "Configuration defaults to Haiku",
		},
		{
			TaskType:    TaskTypeCommunication,
			TargetModel: ModelHaiku,
			Priority:    300,
			Description: "Communication defaults to Haiku",
		},

		// Catch-all default -> Sonnet
		{
			TargetModel: ModelSonnet,
			Priority:    0,
			Description: "Default fallback to Sonnet",
		},
	}
}

// ================================================================================
// TASK CLASSIFICATION
// ================================================================================

// TaskClassification represents a complete classification of a task.
type TaskClassification struct {
	// TaskType is the primary category of the task.
	TaskType TaskType `json:"task_type"`

	// Complexity is the multi-dimensional complexity assessment.
	Complexity ComplexityScore `json:"complexity"`

	// Keywords are terms that helped classify this task.
	Keywords []string `json:"keywords,omitempty"`

	// Confidence is how confident we are in this classification (0.0-1.0).
	Confidence float64 `json:"confidence"`

	// RoutingDecision is the recommended model routing.
	RoutingDecision *RoutingDecision `json:"routing_decision,omitempty"`
}

// Validate checks if the classification is valid.
func (tc *TaskClassification) Validate() error {
	if !tc.TaskType.IsValid() {
		return fmt.Errorf("invalid task type: %s", tc.TaskType)
	}
	if err := tc.Complexity.Validate(); err != nil {
		return fmt.Errorf("invalid complexity: %w", err)
	}
	if tc.Confidence < 0.0 || tc.Confidence > 1.0 {
		return fmt.Errorf("confidence must be 0.0-1.0, got %f", tc.Confidence)
	}
	return nil
}

// ================================================================================
// KEYWORD MAPPINGS
// ================================================================================

// TaskTypeKeywords maps keywords to task types for classification.
var TaskTypeKeywords = map[TaskType][]string{
	TaskTypeResearch: {
		"research", "find", "search", "explore", "investigate",
		"learn", "understand", "documentation", "api docs", "readme",
	},
	TaskTypeCodeGeneration: {
		"implement", "create", "build", "write code", "add feature",
		"new endpoint", "new function", "generate", "develop",
	},
	TaskTypeCodeReview: {
		"review", "analyze", "check", "audit", "inspect",
		"pr review", "code quality", "lint", "critique",
	},
	TaskTypeTesting: {
		"test", "unit test", "integration test", "e2e", "coverage",
		"validate", "verify", "qa", "assert",
	},
	TaskTypeDocumentation: {
		"document", "docs", "readme", "comment", "explain",
		"api documentation", "guide", "tutorial",
	},
	TaskTypeDataProcessing: {
		"parse", "extract", "transform", "etl", "data",
		"json", "csv", "xml", "process", "migrate",
	},
	TaskTypeDebugging: {
		"debug", "fix", "bug", "error", "issue",
		"troubleshoot", "diagnose", "stack trace", "root cause",
	},
	TaskTypeRefactoring: {
		"refactor", "restructure", "reorganize", "clean up",
		"rename", "extract", "simplify", "optimize code",
	},
	TaskTypeArchitecture: {
		"architecture", "design", "schema", "system design",
		"api design", "structure", "pattern", "interface",
	},
	TaskTypeConfiguration: {
		"config", "configuration", "yaml", "toml", "json config",
		"environment", "settings", "env vars",
	},
	TaskTypeDeployment: {
		"deploy", "deployment", "ci", "cd", "docker",
		"kubernetes", "infrastructure", "devops", "pipeline",
	},
	TaskTypeSecurity: {
		"security", "vulnerability", "auth", "authentication",
		"authorization", "encryption", "secrets", "permissions",
	},
	TaskTypeVisual: {
		"visual", "screenshot", "browser", "ui", "frontend",
		"css", "layout", "design review", "playwright",
	},
	TaskTypeCommunication: {
		"message", "email", "report", "summary", "update",
		"commit message", "pr description", "changelog",
	},
	TaskTypePipeline: {
		"pipeline", "workflow", "orchestrate", "chain",
		"multi-step", "sequence", "coordinate",
	},
}

// ClassifyByKeywords attempts to classify a task based on keywords in description.
func ClassifyByKeywords(description string) (TaskType, float64) {
	description = strings.ToLower(description)

	bestType := TaskTypeCodeGeneration // default
	bestScore := 0.0

	for taskType, keywords := range TaskTypeKeywords {
		score := 0.0
		for _, keyword := range keywords {
			if strings.Contains(description, keyword) {
				score += 1.0
			}
		}
		// Normalize by keyword count
		if len(keywords) > 0 {
			score = score / float64(len(keywords))
		}
		if score > bestScore {
			bestScore = score
			bestType = taskType
		}
	}

	// Convert to confidence (cap at 1.0)
	confidence := bestScore
	if confidence > 1.0 {
		confidence = 1.0
	}
	if confidence < 0.3 {
		confidence = 0.3 // minimum confidence for keyword-based
	}

	return bestType, confidence
}

// ================================================================================
// EXAMPLE CLASSIFICATIONS
// ================================================================================

// ExampleClassification demonstrates task classification with routing.
type ExampleClassification struct {
	Description    string             `json:"description"`
	Classification TaskClassification `json:"classification"`
}

// GetExampleClassifications returns documented examples of task classifications.
func GetExampleClassifications() []ExampleClassification {
	return []ExampleClassification{
		// Example 1: Simple documentation (Haiku)
		{
			Description: "Update README with installation instructions",
			Classification: TaskClassification{
				TaskType: TaskTypeDocumentation,
				Complexity: ComplexityScore{
					TechnicalComplexity: 1,
					ContextRequirements: 1,
					RiskLevel:           1,
					Reversibility:       5,
				},
				Keywords:   []string{"update", "readme"},
				Confidence: 0.9,
				RoutingDecision: &RoutingDecision{
					PrimaryModel:          ModelHaiku,
					FallbackModels:        []Model{ModelGemini, ModelSonnet},
					Confidence:            0.95,
					Reason:                "Simple documentation task with low complexity",
					RequiresHumanApproval: false,
					EstimatedCostUSD:      0.002,
				},
			},
		},

		// Example 2: API implementation (Sonnet)
		{
			Description: "Implement REST endpoint for user authentication with JWT",
			Classification: TaskClassification{
				TaskType: TaskTypeCodeGeneration,
				Complexity: ComplexityScore{
					TechnicalComplexity: 3,
					ContextRequirements: 3,
					RiskLevel:           4, // Auth is sensitive
					Reversibility:       3,
				},
				Keywords:   []string{"implement", "endpoint", "authentication"},
				Confidence: 0.85,
				RoutingDecision: &RoutingDecision{
					PrimaryModel:          ModelSonnet,
					FallbackModels:        []Model{ModelOpus},
					Confidence:            0.80,
					Reason:                "Standard implementation with moderate risk",
					RequiresHumanApproval: true, // Auth code should be reviewed
					EstimatedCostUSD:      0.05,
				},
			},
		},

		// Example 3: Security audit (Opus)
		{
			Description: "Audit authentication system for security vulnerabilities",
			Classification: TaskClassification{
				TaskType: TaskTypeSecurity,
				Complexity: ComplexityScore{
					TechnicalComplexity: 4,
					ContextRequirements: 4,
					RiskLevel:           5, // Security is critical
					Reversibility:       5, // Audit is read-only
				},
				Keywords:   []string{"audit", "security", "vulnerabilities"},
				Confidence: 0.95,
				RoutingDecision: &RoutingDecision{
					PrimaryModel:          ModelOpus,
					FallbackModels:        []Model{ModelSonnet},
					Confidence:            0.99,
					Reason:                "Security tasks always use Opus for thoroughness",
					RequiresHumanApproval: true,
					EstimatedCostUSD:      0.20,
				},
			},
		},

		// Example 4: Bulk data processing (Gemini)
		{
			Description: "Parse and transform 10,000 log entries from JSON to CSV",
			Classification: TaskClassification{
				TaskType: TaskTypeDataProcessing,
				Complexity: ComplexityScore{
					TechnicalComplexity: 2,
					ContextRequirements: 1,
					RiskLevel:           2,
					Reversibility:       4,
				},
				Keywords:   []string{"parse", "transform", "json", "csv"},
				Confidence: 0.90,
				RoutingDecision: &RoutingDecision{
					PrimaryModel:          ModelGemini,
					FallbackModels:        []Model{ModelHaiku, ModelSonnet},
					Confidence:            0.90,
					Reason:                "Bulk data processing is cost-effective with Gemini",
					RequiresHumanApproval: false,
					EstimatedCostUSD:      0.01,
				},
			},
		},

		// Example 5: System architecture (Opus)
		{
			Description: "Design microservices architecture for payment processing",
			Classification: TaskClassification{
				TaskType: TaskTypeArchitecture,
				Complexity: ComplexityScore{
					TechnicalComplexity: 5,
					ContextRequirements: 4,
					RiskLevel:           5, // Payments are critical
					Reversibility:       2, // Architecture is hard to change
				},
				Keywords:   []string{"design", "architecture", "payment"},
				Confidence: 0.92,
				RoutingDecision: &RoutingDecision{
					PrimaryModel:          ModelOpus,
					FallbackModels:        []Model{}, // No fallback for critical work
					Confidence:            0.99,
					Reason:                "Architecture decisions require highest capability",
					RequiresHumanApproval: true,
					EstimatedCostUSD:      0.30,
				},
			},
		},

		// Example 6: Unit tests (Haiku)
		{
			Description: "Write unit tests for the string utility functions",
			Classification: TaskClassification{
				TaskType: TaskTypeTesting,
				Complexity: ComplexityScore{
					TechnicalComplexity: 2,
					ContextRequirements: 2,
					RiskLevel:           1,
					Reversibility:       5,
				},
				Keywords:   []string{"unit tests", "test"},
				Confidence: 0.88,
				RoutingDecision: &RoutingDecision{
					PrimaryModel:          ModelHaiku,
					FallbackModels:        []Model{ModelSonnet},
					Confidence:            0.90,
					Reason:                "Simple tests are cost-effective with Haiku",
					RequiresHumanApproval: false,
					EstimatedCostUSD:      0.005,
				},
			},
		},

		// Example 7: Browser testing (Antigravity)
		{
			Description: "Verify login flow works correctly on mobile viewport",
			Classification: TaskClassification{
				TaskType: TaskTypeVisual,
				Complexity: ComplexityScore{
					TechnicalComplexity: 2,
					ContextRequirements: 2,
					RiskLevel:           2,
					Reversibility:       5, // Testing is read-only
				},
				Keywords:   []string{"verify", "login", "mobile", "viewport"},
				Confidence: 0.85,
				RoutingDecision: &RoutingDecision{
					PrimaryModel:          ModelAntigravity,
					FallbackModels:        []Model{ModelGemini},
					Confidence:            0.88,
					Reason:                "Visual/browser testing uses human-in-loop Antigravity",
					RequiresHumanApproval: false,
					EstimatedCostUSD:      0.0, // Variable
				},
			},
		},
	}
}

// ================================================================================
// ROUTING MATRIX SUMMARY
// ================================================================================

// RoutingMatrixEntry summarizes when to use each model.
type RoutingMatrixEntry struct {
	Model    Model    `json:"model"`
	UseCases []string `json:"use_cases"`
	AvoidFor []string `json:"avoid_for"`
	CostTier string   `json:"cost_tier"`
	BestFor  string   `json:"best_for"`
}

// GetRoutingMatrix returns the complete routing matrix documentation.
func GetRoutingMatrix() []RoutingMatrixEntry {
	return []RoutingMatrixEntry{
		{
			Model: ModelOpus,
			UseCases: []string{
				"Architecture and system design",
				"Security audits and implementations",
				"Complex debugging across multiple systems",
				"Novel algorithm development",
				"Critical path decisions",
				"High-risk changes (risk level 4-5)",
			},
			AvoidFor: []string{
				"Simple documentation",
				"Basic refactoring",
				"Unit tests",
				"Configuration changes",
			},
			CostTier: "Highest ($0.015/$0.075 per 1K tokens)",
			BestFor:  "Tasks where mistakes are costly or where highest quality is required",
		},
		{
			Model: ModelSonnet,
			UseCases: []string{
				"Standard code implementation",
				"Code review and analysis",
				"Debugging and investigation",
				"Deployment and DevOps",
				"Pipeline orchestration",
				"Moderate complexity (score 2.5-4.0)",
			},
			AvoidFor: []string{
				"Simple tests",
				"Bulk data processing",
				"Critical security work",
			},
			CostTier: "Medium ($0.003/$0.015 per 1K tokens)",
			BestFor:  "Balanced quality and cost for most implementation work",
		},
		{
			Model: ModelHaiku,
			UseCases: []string{
				"Writing unit tests",
				"Documentation updates",
				"Simple refactoring",
				"Configuration changes",
				"Commit messages and PR descriptions",
				"Simple tasks (score < 2.5)",
			},
			AvoidFor: []string{
				"Complex debugging",
				"Architecture decisions",
				"Security-sensitive code",
			},
			CostTier: "Low ($0.001/$0.005 per 1K tokens)",
			BestFor:  "High-volume, low-complexity tasks",
		},
		{
			Model: ModelGemini,
			UseCases: []string{
				"Research and information gathering",
				"Bulk data processing",
				"Log analysis",
				"Documentation research",
				"Codebase exploration",
			},
			AvoidFor: []string{
				"Complex implementation",
				"Security work",
				"Architecture decisions",
			},
			CostTier: "Lowest ($0.0001/$0.0004 per 1K tokens)",
			BestFor:  "Bulk processing and research where cost matters most",
		},
		{
			Model: ModelAntigravity,
			UseCases: []string{
				"Visual/UI testing",
				"Browser automation",
				"Screenshot analysis",
				"High-stakes operations needing human oversight",
			},
			AvoidFor: []string{
				"Pure code tasks",
				"Data processing",
				"Bulk operations",
			},
			CostTier: "Variable (includes human time)",
			BestFor:  "Tasks requiring visual understanding or human judgment",
		},
	}
}
