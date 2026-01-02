// Package routing provides intelligent task routing between different AI models.
package routing

import "fmt"

// ================================================================================
// DEFAULT ROUTING MATRIX
// ================================================================================

// DefaultRoutingMatrix is a hardcoded lookup table for common task routing decisions.
// It maps TaskType and ComplexityLevel to recommended models for fast, offline routing.
// This serves as a fallback and baseline for the dynamic routing engine.
//
// Matrix Structure:
//   - Primary dimension: TaskType (research, code_generation, testing, etc.)
//   - Secondary dimension: Complexity score (1-5 scale, can be float)
//   - Output: Recommended Model (opus, sonnet, haiku, gemini, antigravity)
//
// Task ID: RTR-004
// Created: 2026-01-01
type DefaultRoutingMatrix map[TaskType]map[int]Model

// GetDefaultRoutingMatrix returns the hardcoded default routing matrix.
// This matrix provides immediate routing decisions without dynamic calculation.
//
// The matrix is organized by:
// 1. Task Type: The category of work being performed
// 2. Complexity Level: A discrete 1-5 score representing overall task complexity
//
// Example usage:
//
//	matrix := GetDefaultRoutingMatrix()
//	model := matrix[TaskTypeResearch][3]  // Research with complexity 3 -> opus
func GetDefaultRoutingMatrix() DefaultRoutingMatrix {
	return DefaultRoutingMatrix{
		// ====================================================================
		// RESEARCH TASKS
		// ====================================================================
		// Research is bulk-oriented, benefits from Gemini's cost-effectiveness
		// Complexity 1-2: Simple searches, straightforward research -> Gemini (cheap)
		// Complexity 3-5: Synthesis, analysis, complex research -> Opus (quality)
		TaskTypeResearch: {
			1: ModelGemini, // Trivial: Simple searches, basic lookups
			2: ModelGemini, // Simple: Straightforward research, single topics
			3: ModelOpus,   // Moderate: Multi-source synthesis, analysis needed
			4: ModelOpus,   // Complex: Deep research, expert-level synthesis
			5: ModelOpus,   // Highly Complex: Novel research, original analysis
		},

		// ====================================================================
		// CODE GENERATION TASKS
		// ====================================================================
		// Implementation is the core workload, scales from Haiku to Opus
		// Complexity 1-2: Simple implementations -> Haiku (efficient)
		// Complexity 3-4: Standard implementations -> Sonnet (balanced)
		// Complexity 5: Complex/novel implementations -> Opus (best quality)
		TaskTypeCodeGeneration: {
			1: ModelHaiku,  // Trivial: Add simple function, fix typo
			2: ModelHaiku,  // Simple: Basic feature, standard implementation
			3: ModelSonnet, // Moderate: Multi-feature implementation, some complexity
			4: ModelSonnet, // Complex: Cross-system implementation, many dependencies
			5: ModelOpus,   // Highly Complex: Novel algorithms, new patterns
		},

		// ====================================================================
		// CODE REVIEW TASKS
		// ====================================================================
		// Reviews benefit from thorough analysis
		// Complexity 1-2: Simple reviews -> Haiku
		// Complexity 3-4: Standard reviews -> Sonnet (best for PR review)
		// Complexity 5: Security/architecture review -> Opus
		TaskTypeCodeReview: {
			1: ModelHaiku,  // Trivial: Style/formatting review
			2: ModelHaiku,  // Simple: Basic code quality review
			3: ModelSonnet, // Moderate: Standard PR review, logic check
			4: ModelSonnet, // Complex: Cross-system review, interdependencies
			5: ModelOpus,   // Highly Complex: Security audit, architecture review
		},

		// ====================================================================
		// TESTING TASKS
		// ====================================================================
		// Testing is high-volume, low-complexity work
		// Default to Haiku (efficient for all complexity levels)
		// Only escalate to Sonnet for complex test scenarios
		TaskTypeTesting: {
			1: ModelHaiku,  // Trivial: Simple unit test
			2: ModelHaiku,  // Simple: Standard unit/integration test
			3: ModelHaiku,  // Moderate: Complex test with fixtures
			4: ModelSonnet, // Complex: Multi-layer integration test
			5: ModelSonnet, // Highly Complex: End-to-end test scenarios
		},

		// ====================================================================
		// DOCUMENTATION TASKS
		// ====================================================================
		// Documentation is low-risk, straightforward work
		// Default to Haiku for all but the most complex cases
		// Gemini can handle bulk documentation generation
		TaskTypeDocumentation: {
			1: ModelHaiku,  // Trivial: Update README, add comment
			2: ModelHaiku,  // Simple: Write basic docs, simple API docs
			3: ModelHaiku,  // Moderate: Comprehensive docs for feature
			4: ModelSonnet, // Complex: Architecture documentation, guides
			5: ModelSonnet, // Highly Complex: System architecture documentation
		},

		// ====================================================================
		// DATA PROCESSING TASKS
		// ====================================================================
		// Data processing is bulk-oriented, cost matters most
		// Default to Gemini (cheap) for all but the most complex transformations
		// Haiku can handle simple transforms
		TaskTypeDataProcessing: {
			1: ModelGemini, // Trivial: Simple parsing, basic transforms
			2: ModelGemini, // Simple: JSON/CSV parsing, data migration
			3: ModelHaiku,  // Moderate: Complex transformation logic
			4: ModelSonnet, // Complex: Multi-format ETL, optimization needed
			5: ModelSonnet, // Highly Complex: Advanced analytics, custom algorithms
		},

		// ====================================================================
		// DEBUGGING TASKS
		// ====================================================================
		// Debugging requires reasoning and problem-solving
		// Complexity 1-2: Simple issues -> Haiku
		// Complexity 3-4: Standard debugging -> Sonnet
		// Complexity 5: Complex multi-system bugs -> Opus
		TaskTypeDebugging: {
			1: ModelHaiku,  // Trivial: Simple error message, obvious fix
			2: ModelHaiku,  // Simple: Single file bug, clear root cause
			3: ModelSonnet, // Moderate: Multi-file bug, some investigation needed
			4: ModelSonnet, // Complex: Multi-system bug, deep investigation
			5: ModelOpus,   // Highly Complex: Complex distributed system bug
		},

		// ====================================================================
		// REFACTORING TASKS
		// ====================================================================
		// Refactoring is generally lower complexity, default to Haiku
		// Complexity 1-2: Simple refactors -> Haiku
		// Complexity 3-4: Significant refactors -> Sonnet
		// Complexity 5: Major architectural refactors -> Opus
		TaskTypeRefactoring: {
			1: ModelHaiku,  // Trivial: Rename variable, extract function
			2: ModelHaiku,  // Simple: Extract method, basic restructuring
			3: ModelSonnet, // Moderate: Significant refactoring, multiple files
			4: ModelSonnet, // Complex: Major module refactor, impact analysis
			5: ModelOpus,   // Highly Complex: System-wide refactoring
		},

		// ====================================================================
		// ARCHITECTURE TASKS
		// ====================================================================
		// Architecture decisions are high-stakes, always use Opus
		// Complexity level doesn't matter - architecture always needs Opus
		TaskTypeArchitecture: {
			1: ModelOpus, // Trivial (shouldn't exist): Even simple architecture -> Opus
			2: ModelOpus, // Simple design decisions still need Opus
			3: ModelOpus, // Moderate architecture
			4: ModelOpus, // Complex architecture
			5: ModelOpus, // Highly Complex architecture (system-wide design)
		},

		// ====================================================================
		// CONFIGURATION TASKS
		// ====================================================================
		// Configuration is straightforward, mostly use Haiku
		// Complexity 1-2: Simple config -> Haiku
		// Complexity 3-4: Complex config with dependencies -> Sonnet
		// Complexity 5: System-wide configuration redesign -> Opus
		TaskTypeConfiguration: {
			1: ModelHaiku,  // Trivial: Update single config value
			2: ModelHaiku,  // Simple: Add simple config, basic YAML edits
			3: ModelSonnet, // Moderate: Multi-file config, interdependencies
			4: ModelSonnet, // Complex: System-wide configuration changes
			5: ModelOpus,   // Highly Complex: Config architecture redesign
		},

		// ====================================================================
		// DEPLOYMENT TASKS
		// ====================================================================
		// Deployment is risky (production impact), escalate to Sonnet/Opus
		// Complexity 1-2: Simple deployment -> Sonnet (safer than Haiku)
		// Complexity 3-4: Standard deployment -> Sonnet
		// Complexity 5: Critical deployment -> Opus (human oversight recommended)
		TaskTypeDeployment: {
			1: ModelSonnet, // Trivial: Deploy simple service
			2: ModelSonnet, // Simple: Standard deployment procedure
			3: ModelSonnet, // Moderate: Multi-service deployment, some coordination
			4: ModelOpus,   // Complex: Complex deployment, many services, rollback planning
			5: ModelOpus,   // Highly Complex: Critical infrastructure deployment
		},

		// ====================================================================
		// SECURITY TASKS
		// ====================================================================
		// Security is always critical, always use Opus (no exceptions)
		// Risk level is high regardless of complexity
		TaskTypeSecurity: {
			1: ModelOpus, // Trivial (shouldn't happen): Even simple security -> Opus
			2: ModelOpus, // Simple security task -> Opus
			3: ModelOpus, // Moderate security work -> Opus
			4: ModelOpus, // Complex security work -> Opus
			5: ModelOpus, // Highly Complex security work -> Opus
		},

		// ====================================================================
		// VISUAL AND BROWSER TASKS
		// ====================================================================
		// Visual tasks need human in the loop (Antigravity)
		// Falls back to Gemini if Antigravity unavailable
		TaskTypeVisual: {
			1: ModelAntigravity, // Trivial: Simple screenshot review
			2: ModelAntigravity, // Simple: Basic browser testing
			3: ModelAntigravity, // Moderate: UI testing, visual validation
			4: ModelAntigravity, // Complex: Complex visual testing
			5: ModelAntigravity, // Highly Complex: High-stakes visual operations
		},

		// ====================================================================
		// COMMUNICATION TASKS
		// ====================================================================
		// Communication is low-complexity, use Haiku for efficiency
		// Complexity 1-2: Simple messaging -> Haiku
		// Complexity 3-4: Detailed reports -> Sonnet
		// Complexity 5: Complex/strategic communication -> Opus
		TaskTypeCommunication: {
			1: ModelHaiku,  // Trivial: Short commit message, simple summary
			2: ModelHaiku,  // Simple: PR description, basic documentation
			3: ModelSonnet, // Moderate: Detailed technical report
			4: ModelSonnet, // Complex: Multi-section report, strategic messaging
			5: ModelOpus,   // Highly Complex: Executive communication, strategy docs
		},

		// ====================================================================
		// PIPELINE AND ORCHESTRATION TASKS
		// ====================================================================
		// Pipeline coordination is moderate complexity, use Sonnet
		// Complexity 1-2: Simple pipelines -> Sonnet (safest)
		// Complexity 3-4: Standard pipelines -> Sonnet
		// Complexity 5: Complex orchestration -> Opus
		TaskTypePipeline: {
			1: ModelSonnet, // Trivial: Simple single-stage pipeline
			2: ModelSonnet, // Simple: Two-stage pipeline, basic orchestration
			3: ModelSonnet, // Moderate: Multi-stage pipeline, simple coordination
			4: ModelSonnet, // Complex: Complex multi-stage pipeline, error handling
			5: ModelOpus,   // Highly Complex: Distributed orchestration, failure recovery
		},
	}
}

// LookupRoute looks up the recommended model for a task using the default matrix.
// Returns the recommended Model or DefaultFallback if not found.
// This is a simple O(1) lookup in the hardcoded matrix.
//
// Args:
//
//	taskType: The type of task
//	complexityLevel: The complexity score (1-5)
//
// Returns:
//
//	Recommended Model for the task, or ModelSonnet as fallback
//
// Example:
//
//	model := LookupRoute(TaskTypeCodeGeneration, 3)
//	// Returns: ModelSonnet (standard complexity implementation)
func LookupRoute(taskType TaskType, complexityLevel int) Model {
	matrix := GetDefaultRoutingMatrix()

	// Clamp complexity to valid range
	if complexityLevel < 1 {
		complexityLevel = 1
	} else if complexityLevel > 5 {
		complexityLevel = 5
	}

	// Look up in matrix
	if taskMap, exists := matrix[taskType]; exists {
		if model, exists := taskMap[complexityLevel]; exists {
			return model
		}
	}

	// Fallback: return Sonnet as the balanced default
	return ModelSonnet
}

// LookupRouteWithScore looks up the recommended model using a floating-point complexity score.
// Rounds the complexity score to the nearest integer (1-5) for matrix lookup.
//
// Args:
//
//	taskType: The type of task
//	complexityScore: The complexity score (typically 1.0-5.0, float)
//
// Returns:
//
//	Recommended Model for the task, rounded complexity applied
//
// Example:
//
//	model := LookupRouteWithScore(TaskTypeResearch, 2.7)
//	// Rounds to 3, returns: ModelOpus (research complexity 3 -> opus)
func LookupRouteWithScore(taskType TaskType, complexityScore float64) Model {
	// Round to nearest integer
	complexityLevel := int(complexityScore + 0.5)
	return LookupRoute(taskType, complexityLevel)
}

// MatrixEntry represents a single entry in the default routing matrix.
// Used for debugging, documentation, and validation.
type MatrixEntry struct {
	TaskType         TaskType
	ComplexityLevel  int
	RecommendedModel Model
	Justification    string
}

// ValidateMatrix checks that the default routing matrix is well-formed.
// Returns a list of any validation errors found.
// This is useful for testing and ensuring consistency.
//
// Validation rules:
//   - All task types have entries for complexity levels 1-5
//   - All recommended models are valid
//   - Critical tasks (Architecture, Security) use Opus or higher
func ValidateMatrix() []error {
	var errs []error
	matrix := GetDefaultRoutingMatrix()

	// Check that all task types are represented
	for _, taskType := range AllTaskTypes() {
		taskMap, exists := matrix[taskType]
		if !exists {
			errs = append(errs, fmt.Errorf("task type %s missing from matrix", taskType))
			continue
		}

		// Check that all complexity levels 1-5 are defined
		for level := 1; level <= 5; level++ {
			model, exists := taskMap[level]
			if !exists {
				errs = append(errs, fmt.Errorf("task type %s missing complexity level %d", taskType, level))
				continue
			}

			// Check that the model is valid
			if !model.IsValid() {
				errs = append(errs, fmt.Errorf("task type %s complexity %d has invalid model %s", taskType, level, model))
			}

			// Validate critical task constraints
			if taskType == TaskTypeArchitecture || taskType == TaskTypeSecurity {
				if model != ModelOpus {
					errs = append(errs, fmt.Errorf("task type %s (critical) should use Opus, found %s for complexity %d", taskType, model, level))
				}
			}
		}
	}

	return errs
}

// DescribeRoute provides a human-readable explanation for a routing decision.
// Useful for debugging and understanding why a particular model was chosen.
//
// Args:
//
//	taskType: The type of task
//	complexityLevel: The complexity score (1-5)
//
// Returns:
//
//	A formatted string describing the routing decision and rationale
//
// Example:
//
//	desc := DescribeRoute(TaskTypeCodeGeneration, 3)
//	// Returns: "Code generation with complexity 3 (Moderate) -> Sonnet (standard)"
func DescribeRoute(taskType TaskType, complexityLevel int) string {
	if complexityLevel < 1 {
		complexityLevel = 1
	} else if complexityLevel > 5 {
		complexityLevel = 5
	}

	model := LookupRoute(taskType, complexityLevel)

	// Get complexity level name
	complexityName := map[int]string{
		1: "Trivial",
		2: "Simple",
		3: "Moderate",
		4: "Complex",
		5: "Highly Complex",
	}[complexityLevel]

	// Get routing rationale
	rationale := getRoutingRationale(taskType, complexityLevel, model)

	return fmt.Sprintf(
		"%s with complexity %d (%s) -> %s (%s)",
		taskType.Description(),
		complexityLevel,
		complexityName,
		model.DisplayName(),
		rationale,
	)
}

// getRoutingRationale returns a brief explanation for a routing decision.
func getRoutingRationale(taskType TaskType, level int, model Model) string {
	// Map common patterns
	switch {
	case model == ModelOpus:
		return "highest quality required"
	case model == ModelSonnet:
		return "balanced quality and cost"
	case model == ModelHaiku:
		return "efficient for simple tasks"
	case model == ModelGemini:
		return "cost-effective bulk processing"
	case model == ModelAntigravity:
		return "requires human oversight"
	default:
		return "default routing"
	}
}

// GetMatrixAsTable returns the entire default matrix as a slice of MatrixEntry.
// Useful for logging, visualization, or exporting routing decisions.
//
// Returns:
//
//	A slice of all matrix entries in a consistent order
func GetMatrixAsTable() []MatrixEntry {
	var entries []MatrixEntry
	matrix := GetDefaultRoutingMatrix()

	for _, taskType := range AllTaskTypes() {
		if taskMap, exists := matrix[taskType]; exists {
			for level := 1; level <= 5; level++ {
				if model, exists := taskMap[level]; exists {
					entry := MatrixEntry{
						TaskType:         taskType,
						ComplexityLevel:  level,
						RecommendedModel: model,
						Justification:    getRoutingRationale(taskType, level, model),
					}
					entries = append(entries, entry)
				}
			}
		}
	}

	return entries
}
