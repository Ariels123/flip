// Package routing provides intelligent task routing between different AI models.
package routing

import (
	"fmt"
	"strings"
)

// ================================================================================
// COMPLEXITY SCORING ALGORITHM
// ================================================================================

// ComplexityLevel represents a simple 1-5 score for a single dimension.
type ComplexityLevel int

const (
	// Level 1: Trivial
	LevelTrivial ComplexityLevel = 1
	// Level 2: Simple
	LevelSimple ComplexityLevel = 2
	// Level 3: Moderate
	LevelModerate ComplexityLevel = 3
	// Level 4: Complex
	LevelComplex ComplexityLevel = 4
	// Level 5: Highly Complex
	LevelHighlyComplex ComplexityLevel = 5
)

// ScoreTask analyzes a task description and produces a detailed ComplexityScore.
//
// SCORING RUBRIC:
// ==============
//
// Technical Complexity (1-5):
//
//	1 - Trivial: Single operation, no edge cases (rename variable, simple fix)
//	2 - Simple: Standard implementation, clear requirements (add basic feature)
//	3 - Moderate: Multiple files, some edge cases (implement multi-part feature)
//	4 - Complex: Cross-system impact, many interdependencies (major refactor)
//	5 - Highly Complex: Novel algorithms, architectural decisions (new system)
//
// Context Requirements (1-5):
//
//	1 - None: Standalone, self-contained, no system knowledge needed
//	2 - Minimal: Single file or module, limited history needed
//	3 - Moderate: Multiple modules, need to understand relationships
//	4 - Significant: Cross-repository understanding, deep knowledge needed
//	5 - Extensive: Full system knowledge, historical context, multiple systems
//
// Risk Level (1-5):
//
//	1 - Negligible: Read-only, easily caught mistakes (list files, read data)
//	2 - Low: Minor bugs, easy rollback (simple formatting, style changes)
//	3 - Moderate: User-facing changes, medium effort to fix (feature change)
//	4 - High: Data integrity, security implications (auth, database schema)
//	5 - Critical: Production outage risk, data loss, security breach (payments)
//
// Reversibility (1-5):
//
//	5 - Trivial: Read-only, no changes made (inspection, analysis)
//	4 - Easy: Simple revert, no side effects (code addition, deletion)
//	3 - Moderate: Standard revert with some cleanup (config change)
//	2 - Difficult: Complex multi-step rollback (database migration)
//	1 - Irreversible: Data deletion, external calls, cannot undo (deletes, API calls)
func ScoreTask(description string) (*ComplexityScore, error) {
	desc := strings.ToLower(description)

	// Analyze the description for scoring signals
	signals := analyzeDescription(desc, description)

	// Calculate each dimension
	technical := calculateTechnicalComplexity(signals)
	context := calculateContextRequirements(signals)
	risk := calculateRiskLevel(signals)
	reversibility := calculateReversibility(signals)

	// Estimate tokens based on various factors
	estimatedTokens := estimateTokens(description, technical)

	// Determine if human review is required
	requiresReview := risk >= 4 || technical >= 4

	score := &ComplexityScore{
		TechnicalComplexity: int(technical),
		ContextRequirements: int(context),
		RiskLevel:           int(risk),
		Reversibility:       int(reversibility),
		EstimatedTokens:     estimatedTokens,
		RequiresHumanReview: requiresReview,
	}

	// Validate before returning
	if err := score.Validate(); err != nil {
		return nil, fmt.Errorf("invalid score generated: %w", err)
	}

	return score, nil
}

// descriptionSignals contains extracted signals from task description analysis.
type descriptionSignals struct {
	// Keyword matches
	highComplexityKeywords   int
	mediumComplexityKeywords int
	lowComplexityKeywords    int
	highRiskKeywords         int
	mediumRiskKeywords       int
	lowRiskKeywords          int
	readOnlyKeywords         int
	irreversibleKeywords     int

	// Length analysis
	tokenCount int
	wordCount  int

	// Feature analysis
	hasMultipleFileRefs bool
	hasCrossSysRefs     bool
	hasAlgorithmicReqs  bool
	hasSecurityKeywords bool
	hasDataKeywords     bool
	hasUIKeywords       bool
	isRefactoring       bool
	isReadOnly          bool
}

// analyzeDescription extracts scoring signals from task description.
func analyzeDescription(desc, originalDesc string) descriptionSignals {
	signals := descriptionSignals{}

	// Token estimation: rough heuristic (1 word ≈ 1.3 tokens)
	signals.wordCount = len(strings.Fields(originalDesc))
	signals.tokenCount = int(float64(signals.wordCount) * 1.3)

	// Check for high complexity indicators (Architecture/Design tasks)
	highComplexKeywords := []string{
		"architecture", "design", "schema", "system", "pattern",
		"algorithm", "framework", "interface", "restructure",
		"refactor", "migrate", "upgrade", "rewrite", "optimize",
		"performance", "novel", "machine learning", "encryption",
		"distributed", "synchronization", "orchestrate", "microservices",
	}
	for _, kw := range highComplexKeywords {
		if strings.Contains(desc, kw) {
			signals.highComplexityKeywords++
		}
	}

	// Check for medium complexity indicators (Implementation/Modification)
	mediumComplexKeywords := []string{
		"implement", "add", "feature", "endpoint", "method",
		"class", "function", "modify", "change", "update",
		"integration", "config", "setup", "connect", "hook",
	}
	for _, kw := range mediumComplexKeywords {
		if strings.Contains(desc, kw) {
			signals.mediumComplexityKeywords++
		}
	}

	// Check for low complexity indicators (Simple operations)
	lowComplexKeywords := []string{
		"fix", "list", "check", "read", "verify",
		"rename", "delete", "remove", "format", "lint",
		"test", "document", "comment", "style", "typo",
	}
	for _, kw := range lowComplexKeywords {
		if strings.Contains(desc, kw) {
			signals.lowComplexityKeywords++
		}
	}

	// Check for high risk indicators
	highRiskKeywords := []string{
		"security", "auth", "password", "token", "credential",
		"payment", "transaction", "financial", "data loss",
		"encryption", "vulnerability", "breach", "critical",
	}
	for _, kw := range highRiskKeywords {
		if strings.Contains(desc, kw) {
			signals.highRiskKeywords++
		}
	}

	// Check for medium risk indicators
	mediumRiskKeywords := []string{
		"database", "schema", "migration", "api", "deploy",
		"production", "rollback", "version", "backward",
		"compatibility", "breaking", "cache", "state",
	}
	for _, kw := range mediumRiskKeywords {
		if strings.Contains(desc, kw) {
			signals.mediumRiskKeywords++
		}
	}

	// Check for low risk indicators
	lowRiskKeywords := []string{
		"documentation", "readme", "comment", "log", "debug",
		"test", "helper", "utility", "mock", "example",
	}
	for _, kw := range lowRiskKeywords {
		if strings.Contains(desc, kw) {
			signals.lowRiskKeywords++
		}
	}

	// Check for reversibility indicators
	readOnlyKeywords := []string{
		"read", "check", "verify", "analyze", "review",
		"audit", "inspect", "investigate", "search", "find",
	}
	for _, kw := range readOnlyKeywords {
		if strings.Contains(desc, kw) {
			signals.readOnlyKeywords++
		}
	}

	irreversibleKeywords := []string{
		"delete", "remove", "drop", "destroy", "erase",
		"migrate", "replace", "overwrite", "truncate",
	}
	for _, kw := range irreversibleKeywords {
		if strings.Contains(desc, kw) {
			signals.irreversibleKeywords++
		}
	}

	// Check for multi-dimensional features
	multiFileIndicators := []string{
		"multiple", "across", "files", "modules", "packages",
		"components", "systems", "services", "repositories",
	}
	for _, kw := range multiFileIndicators {
		if strings.Contains(desc, kw) {
			signals.hasMultipleFileRefs = true
			break
		}
	}

	crossSysIndicators := []string{
		"cross", "integration", "sync", "coordinate", "orchestrate",
		"pipeline", "workflow", "endpoint", "api", "service",
	}
	for _, kw := range crossSysIndicators {
		if strings.Contains(desc, kw) {
			signals.hasCrossSysRefs = true
			break
		}
	}

	algorithmicIndicators := []string{
		"algorithm", "optimize", "performance", "efficiency",
		"calculation", "compute", "analyze", "process",
	}
	for _, kw := range algorithmicIndicators {
		if strings.Contains(desc, kw) {
			signals.hasAlgorithmicReqs = true
			break
		}
	}

	if strings.Contains(desc, "security") || strings.Contains(desc, "auth") ||
		strings.Contains(desc, "encrypt") || strings.Contains(desc, "vulnerability") {
		signals.hasSecurityKeywords = true
	}

	if strings.Contains(desc, "data") || strings.Contains(desc, "database") ||
		strings.Contains(desc, "storage") || strings.Contains(desc, "migration") {
		signals.hasDataKeywords = true
	}

	if strings.Contains(desc, "ui") || strings.Contains(desc, "frontend") ||
		strings.Contains(desc, "visual") || strings.Contains(desc, "css") {
		signals.hasUIKeywords = true
	}

	signals.isRefactoring = strings.Contains(desc, "refactor") ||
		strings.Contains(desc, "reorganize") ||
		strings.Contains(desc, "restructure")

	signals.isReadOnly = strings.Contains(desc, "read") ||
		strings.Contains(desc, "review") ||
		strings.Contains(desc, "analyze") ||
		strings.Contains(desc, "audit")

	return signals
}

// calculateTechnicalComplexity scores the inherent difficulty of the work.
func calculateTechnicalComplexity(signals descriptionSignals) ComplexityLevel {
	score := 0.0

	// High complexity indicators - strongest signal
	if signals.highComplexityKeywords > 0 {
		score += float64(signals.highComplexityKeywords) * 2.5
	}

	// Medium complexity indicators
	if signals.mediumComplexityKeywords > 0 {
		score += float64(signals.mediumComplexityKeywords) * 1.2
	}

	// Low complexity indicators reduce score
	if signals.lowComplexityKeywords > 0 {
		score -= float64(signals.lowComplexityKeywords) * 0.5
	}

	// Multi-file references add complexity
	if signals.hasMultipleFileRefs {
		score += 1.5
	}

	// Cross-system integration adds significant complexity
	if signals.hasCrossSysRefs {
		score += 2.0
	}

	// Algorithmic requirements add complexity
	if signals.hasAlgorithmicReqs {
		score += 2.5
	}

	// Security-focused work adds complexity
	if signals.hasSecurityKeywords {
		score += 2.0
	}

	// Data-focused work adds complexity
	if signals.hasDataKeywords {
		score += 1.5
	}

	// Length-based scoring: longer descriptions often indicate more complex tasks
	if signals.wordCount > 100 {
		score += 1.0
	} else if signals.wordCount > 50 {
		score += 0.5
	}

	// Refactoring is generally moderate complexity
	if signals.isRefactoring {
		score += 0.8
	}

	// Convert to 1-5 scale
	level := levelFromScore(score, 1, 5)
	return level
}

// calculateContextRequirements scores knowledge requirements for the task.
func calculateContextRequirements(signals descriptionSignals) ComplexityLevel {
	score := 0.0

	// Multi-file references require moderate context
	if signals.hasMultipleFileRefs {
		score += 2.0
	}

	// Cross-system references require significant context
	if signals.hasCrossSysRefs {
		score += 2.5
	}

	// Architecture/design requires extensive context
	if signals.highComplexityKeywords > 0 {
		score += 1.5
	}

	// Database/schema changes require understanding
	if signals.hasDataKeywords {
		score += 1.0
	}

	// Security requires deep context
	if signals.hasSecurityKeywords {
		score += 1.0
	}

	// Simple/trivial tasks need minimal context
	if signals.lowComplexityKeywords > 2 {
		score -= 1.5
	}

	// Read-only operations need minimal context
	if signals.isReadOnly {
		score -= 1.0
	}

	// Token count can indicate context needs
	if signals.tokenCount > 500 {
		score += 1.0
	}

	// Convert to 1-5 scale
	level := levelFromScore(score, 1, 5)
	return level
}

// calculateRiskLevel scores potential negative impact of mistakes.
func calculateRiskLevel(signals descriptionSignals) ComplexityLevel {
	score := 0.0

	// Critical security/payment operations - highest risk
	if signals.hasSecurityKeywords {
		score += 4.5 // Very high
	}
	if signals.highRiskKeywords > 0 {
		score += float64(signals.highRiskKeywords) * 1.5
	}

	// Data-related changes have higher risk
	if signals.hasDataKeywords {
		score += 2.0
	}

	// Medium risk operations
	if signals.mediumRiskKeywords > 0 {
		score += float64(signals.mediumRiskKeywords) * 0.8
	}

	// Irreversible operations increase risk significantly
	if signals.irreversibleKeywords > 0 {
		score += 2.0
	}

	// High complexity work is risky
	if signals.highComplexityKeywords > 0 {
		score += 1.5
	}

	// Read-only operations have low risk
	if signals.isReadOnly || signals.readOnlyKeywords > 2 {
		score -= 2.0
	}

	// Simple operations have low risk
	if signals.lowComplexityKeywords > 2 && signals.highRiskKeywords == 0 && !signals.hasSecurityKeywords {
		score -= 1.0
	}

	// Documentation/testing has minimal risk
	if signals.lowRiskKeywords > 1 {
		score -= 1.5
	}

	// Convert to 1-5 scale
	level := levelFromScore(score, 1, 5)
	return level
}

// calculateReversibility scores how easily changes can be undone.
func calculateReversibility(signals descriptionSignals) ComplexityLevel {
	score := 5.0 // Start at "Trivial" (easiest to reverse)

	// Read-only operations are trivially reversible
	if signals.readOnlyKeywords > 1 {
		return LevelTrivial // 5
	}

	// Irreversible operations are hard to undo
	if signals.irreversibleKeywords > 0 {
		score -= 4.0 // Becomes 1 (Irreversible)
	}

	// Data migrations are difficult to reverse
	if signals.hasDataKeywords && (strings.Contains(strings.ToLower("migration"), "migrate") ||
		strings.Contains(strings.ToLower("database"), "database")) {
		score -= 3.0
	}

	// Architecture changes are difficult to reverse
	if signals.highComplexityKeywords > 1 {
		score -= 2.5
	}

	// Cross-system changes are moderately difficult to reverse
	if signals.hasCrossSysRefs {
		score -= 1.5
	}

	// Multi-file changes are somewhat harder to reverse
	if signals.hasMultipleFileRefs {
		score -= 0.5
	}

	// Simple additions are easy to reverse
	if signals.mediumComplexityKeywords > 0 && signals.highComplexityKeywords == 0 {
		score += 0.0 // No penalty
	}

	// Ensure score stays in bounds before conversion
	if score > 5.0 {
		score = 5.0
	}
	if score < 1.0 {
		score = 1.0
	}

	// Convert to 1-5 scale directly
	level := ComplexityLevel(int(score))
	if level < 1 {
		level = 1
	}
	if level > 5 {
		level = 5
	}
	return level
}

// estimateTokens calculates rough token count for the task.
// Used for context window planning and cost estimation.
func estimateTokens(description string, technicalComplexity ComplexityLevel) int {
	// Base token count: estimate 1.3 tokens per word (common heuristic)
	wordCount := len(strings.Fields(description))
	baseTokens := int(float64(wordCount) * 1.3)

	// Add base for prompt overhead
	baseTokens += 500

	// Adjust based on complexity
	complexityMultiplier := map[ComplexityLevel]float64{
		LevelTrivial:       1.0,
		LevelSimple:        1.3,
		LevelModerate:      1.8,
		LevelComplex:       2.5,
		LevelHighlyComplex: 3.5,
	}

	multiplier := complexityMultiplier[technicalComplexity]
	estimatedTokens := int(float64(baseTokens) * multiplier)

	// Ensure reasonable bounds
	if estimatedTokens < 100 {
		estimatedTokens = 100 // Minimum reasonable estimate
	}
	if estimatedTokens > 50000 {
		estimatedTokens = 50000 // Reasonable maximum
	}

	return estimatedTokens
}

// levelFromScore converts a floating point score to a 1-5 complexity level.
func levelFromScore(score float64, min, max int) ComplexityLevel {
	// Clamp to reasonable range
	if score < 0 {
		score = 0
	}
	if score > 10 {
		score = 10
	}

	// Map to 1-5 scale
	// score 0-2 -> level 1
	// score 2-4 -> level 2
	// score 4-6 -> level 3
	// score 6-8 -> level 4
	// score 8-10 -> level 5

	level := 1 + int(score/2.0)
	if level > 5 {
		level = 5
	}
	if level < 1 {
		level = 1
	}

	return ComplexityLevel(level)
}

// ScoreTaskByKeywords scores a task based primarily on keyword matching.
// This is a simplified version that relies on task type keywords.
func ScoreTaskByKeywords(taskType TaskType, description string) (*ComplexityScore, error) {
	// Start with baseline scores
	score := &ComplexityScore{
		TechnicalComplexity: 2,
		ContextRequirements: 2,
		RiskLevel:           2,
		Reversibility:       3,
	}

	// Adjust based on task type
	switch taskType {
	case TaskTypeArchitecture, TaskTypeSecurity:
		score.TechnicalComplexity = 4
		score.ContextRequirements = 4
		score.RiskLevel = 5
		score.Reversibility = 2
		score.RequiresHumanReview = true

	case TaskTypeCodeGeneration:
		score.TechnicalComplexity = 3
		score.ContextRequirements = 2
		score.RiskLevel = 3
		score.Reversibility = 3

	case TaskTypeCodeReview, TaskTypeDebugging:
		score.TechnicalComplexity = 3
		score.ContextRequirements = 3
		score.RiskLevel = 2
		score.Reversibility = 5

	case TaskTypeRefactoring:
		score.TechnicalComplexity = 3
		score.ContextRequirements = 2
		score.RiskLevel = 2
		score.Reversibility = 4

	case TaskTypeTesting, TaskTypeDocumentation:
		score.TechnicalComplexity = 1
		score.ContextRequirements = 1
		score.RiskLevel = 1
		score.Reversibility = 5

	case TaskTypeResearch, TaskTypeDataProcessing:
		score.TechnicalComplexity = 2
		score.ContextRequirements = 1
		score.RiskLevel = 1
		score.Reversibility = 5

	case TaskTypeConfiguration:
		score.TechnicalComplexity = 2
		score.ContextRequirements = 2
		score.RiskLevel = 2
		score.Reversibility = 4

	case TaskTypeDeployment:
		score.TechnicalComplexity = 3
		score.ContextRequirements = 3
		score.RiskLevel = 4
		score.Reversibility = 2
		score.RequiresHumanReview = true

	case TaskTypeVisual:
		score.TechnicalComplexity = 2
		score.ContextRequirements = 2
		score.RiskLevel = 1
		score.Reversibility = 5

	case TaskTypeCommunication:
		score.TechnicalComplexity = 1
		score.ContextRequirements = 1
		score.RiskLevel = 1
		score.Reversibility = 5

	case TaskTypePipeline:
		score.TechnicalComplexity = 3
		score.ContextRequirements = 3
		score.RiskLevel = 3
		score.Reversibility = 3
	}

	// Estimate tokens
	words := len(strings.Fields(description))
	score.EstimatedTokens = int(float64(words) * 1.3 * float64(score.TechnicalComplexity))

	if err := score.Validate(); err != nil {
		return nil, fmt.Errorf("invalid keyword-based score: %w", err)
	}

	return score, nil
}
