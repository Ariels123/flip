// Package routing provides intelligent task routing between different AI models.
//
// This file implements the task classification system that categorizes tasks
// by their type using keyword-based analysis. Tasks can have multiple labels
// (multi-label classification) and each label includes a confidence score.
//
// Task ID: RTR-001
// Created: 2026-01-02
package routing

import (
	"fmt"
	"strings"
)

// ================================================================================
// TASK CLASSIFICATION
// ================================================================================

// ClassificationResult represents the output of task classification.
// A single task can have multiple labels with varying confidence levels.
type ClassificationResult struct {
	// PrimaryType is the most confident classification (highest confidence score).
	PrimaryType TaskType `json:"primary_type"`

	// PrimaryConfidence is the confidence score for the primary type (0.0-1.0).
	PrimaryConfidence float64 `json:"primary_confidence"`

	// Labels contains all matched task types with their confidence scores.
	// Sorted by confidence in descending order.
	Labels []ClassificationLabel `json:"labels"`

	// Keywords contains the matched keywords that led to this classification.
	// Useful for auditing and debugging classification decisions.
	Keywords map[string]int `json:"keywords,omitempty"`
}

// ClassificationLabel represents a single task type classification with confidence.
type ClassificationLabel struct {
	// TaskType is the classified task type.
	TaskType TaskType `json:"task_type"`

	// Confidence is the confidence score for this classification (0.0-1.0).
	// 1.0 = very confident, 0.5 = moderate confidence, 0.0 = no confidence.
	Confidence float64 `json:"confidence"`

	// MatchedKeywords is the list of keywords that matched for this type.
	MatchedKeywords []string `json:"matched_keywords"`
}

// ClassifyTask analyzes a task description and produces a multi-label classification.
// Returns a ClassificationResult with primary type and all matched types with confidence.
func ClassifyTask(description string) (*ClassificationResult, error) {
	if description == "" {
		return nil, fmt.Errorf("task description cannot be empty")
	}

	desc := strings.ToLower(description)
	signals := analyzeTaskDescription(desc, description)

	// Score each task type
	scores := scoreTaskTypes(signals)

	// Sort by confidence and find primary type
	sortedLabels := sortLabelsByConfidence(scores)

	if len(sortedLabels) == 0 {
		return nil, fmt.Errorf("no task type matches found in description")
	}

	result := &ClassificationResult{
		PrimaryType:       sortedLabels[0].TaskType,
		PrimaryConfidence: sortedLabels[0].Confidence,
		Labels:            sortedLabels,
		Keywords:          signals.matchedKeywordCounts,
	}

	return result, nil
}

// taskSignals contains extracted signals from task description analysis.
type taskSignals struct {
	// Matched keywords for each task type
	researchKeywords        int
	codeGenKeywords         int
	reviewKeywords          int
	testingKeywords         int
	documentationKeywords   int
	dataProcessingKeywords  int
	debuggingKeywords       int
	refactoringKeywords     int
	architectureKeywords    int
	configurationKeywords   int
	deploymentKeywords      int
	securityKeywords        int
	visualKeywords          int
	communicationKeywords   int
	pipelineKeywords        int

	// Feature analysis
	hasMultipleFiles   bool
	hasCrossSystems    bool
	hasNewCodeCreation bool
	hasModifications   bool
	isReadOnly         bool
	hasUserImpact      bool
	hasDataOperations  bool

	// Keyword tracking for audit
	matchedKeywordCounts map[string]int
}

// analyzeTaskDescription extracts classification signals from task description.
func analyzeTaskDescription(desc, originalDesc string) taskSignals {
	signals := taskSignals{
		matchedKeywordCounts: make(map[string]int),
	}

	// Research: Data gathering, investigation, analysis, exploration
	researchKW := []string{
		"research", "investigate", "explore", "analyze", "examine",
		"study", "survey", "review", "audit", "inspect",
		"understand", "find", "search", "locate", "discover",
		"api documentation", "codebase exploration", "error investigation",
	}
	for _, kw := range researchKW {
		if strings.Contains(desc, kw) {
			signals.researchKeywords++
			signals.matchedKeywordCounts[kw]++
		}
	}

	// Code Generation: Writing new code, implementing features
	codeGenKW := []string{
		"implement", "write", "create", "code", "generate",
		"build", "construct", "add feature", "new endpoint",
		"algorithm", "function", "method", "class", "struct",
		"new code", "green field", "from scratch",
	}
	for _, kw := range codeGenKW {
		if strings.Contains(desc, kw) {
			signals.codeGenKeywords++
			signals.matchedKeywordCounts[kw]++
			signals.hasNewCodeCreation = true
		}
	}

	// Code Review: Reviewing existing code
	reviewKW := []string{
		"review", "pr review", "code review", "pull request",
		"audit", "inspect", "analyze code", "check", "verify",
		"security audit", "performance review", "quality review",
	}
	for _, kw := range reviewKW {
		if strings.Contains(desc, kw) {
			signals.reviewKeywords++
			signals.matchedKeywordCounts[kw]++
			signals.isReadOnly = true
		}
	}

	// Testing: Writing and running tests
	testingKW := []string{
		"test", "testing", "unit test", "integration test",
		"test case", "test suite", "assert", "mock",
		"test debugging", "qa", "validation", "verify",
	}
	for _, kw := range testingKW {
		if strings.Contains(desc, kw) {
			signals.testingKeywords++
			signals.matchedKeywordCounts[kw]++
		}
	}

	// Documentation: Writing and updating docs
	documentationKW := []string{
		"documentation", "document", "readme", "api doc",
		"write doc", "update doc", "comment", "docstring",
		"architecture diagram", "guide", "tutorial",
	}
	for _, kw := range documentationKW {
		if strings.Contains(desc, kw) {
			signals.documentationKeywords++
			signals.matchedKeywordCounts[kw]++
		}
	}

	// Data Processing: Parsing, transforming, analyzing data
	dataProcessingKW := []string{
		"data processing", "parse", "transform", "extract",
		"log analysis", "data migration", "json", "csv",
		"migration", "convert", "serialize", "process data",
	}
	for _, kw := range dataProcessingKW {
		if strings.Contains(desc, kw) {
			signals.dataProcessingKeywords++
			signals.matchedKeywordCounts[kw]++
			signals.hasDataOperations = true
		}
	}

	// Debugging: Finding and understanding bugs
	debuggingKW := []string{
		"debug", "debugging", "bug", "fix bug", "issue",
		"troubleshoot", "root cause", "stack trace", "error",
		"reproduce", "error investigation", "crash",
	}
	for _, kw := range debuggingKW {
		if strings.Contains(desc, kw) {
			signals.debuggingKeywords++
			signals.matchedKeywordCounts[kw]++
		}
	}

	// Refactoring: Restructuring existing code
	refactoringKW := []string{
		"refactor", "refactoring", "extract", "rename",
		"reorganize", "restructure", "cleanup", "simplify",
		"improve code", "extract method", "split file",
	}
	for _, kw := range refactoringKW {
		if strings.Contains(desc, kw) {
			signals.refactoringKeywords++
			signals.matchedKeywordCounts[kw]++
		}
	}

	// Architecture: System design decisions
	architectureKW := []string{
		"architecture", "design", "system design", "schema",
		"pattern", "structure", "framework", "interface",
		"api design", "database design", "distributed",
	}
	for _, kw := range architectureKW {
		if strings.Contains(desc, kw) {
			signals.architectureKeywords++
			signals.matchedKeywordCounts[kw]++
		}
	}

	// Configuration: Config file modifications
	configurationKW := []string{
		"configuration", "config", "configure", "yaml",
		"environment", "env var", "setting", "properties",
		"build config", "docker", "compose", ".env",
	}
	for _, kw := range configurationKW {
		if strings.Contains(desc, kw) {
			signals.configurationKeywords++
			signals.matchedKeywordCounts[kw]++
		}
	}

	// Deployment: Deployment and DevOps
	deploymentKW := []string{
		"deploy", "deployment", "devops", "ci/cd", "pipeline",
		"docker", "kubernetes", "infrastructure", "server",
		"production", "staging", "release", "publish",
	}
	for _, kw := range deploymentKW {
		if strings.Contains(desc, kw) {
			signals.deploymentKeywords++
			signals.matchedKeywordCounts[kw]++
		}
	}

	// Security: Security-focused tasks
	securityKW := []string{
		"security", "auth", "authentication", "encrypt",
		"vulnerability", "breach", "credential", "token",
		"password", "security fix", "security review",
	}
	for _, kw := range securityKW {
		if strings.Contains(desc, kw) {
			signals.securityKeywords++
			signals.matchedKeywordCounts[kw]++
		}
	}

	// Visual: UI and visual tasks
	visualKW := []string{
		"visual", "ui", "frontend", "screenshot",
		"test", "visual test", "browser", "selenium",
		"css", "html", "design", "ui testing",
	}
	for _, kw := range visualKW {
		if strings.Contains(desc, kw) {
			signals.visualKeywords++
			signals.matchedKeywordCounts[kw]++
		}
	}

	// Communication: Drafting messages and reports
	communicationKW := []string{
		"communication", "write", "message", "report",
		"pr description", "commit message", "status update",
		"summary", "description", "draft", "email",
	}
	for _, kw := range communicationKW {
		if strings.Contains(desc, kw) {
			signals.communicationKeywords++
			signals.matchedKeywordCounts[kw]++
		}
	}

	// Pipeline: Orchestrating multiple tasks
	pipelineKW := []string{
		"pipeline", "orchestration", "coordinate", "multi-stage",
		"workflow", "sequence", "chain", "compose",
	}
	for _, kw := range pipelineKW {
		if strings.Contains(desc, kw) {
			signals.pipelineKeywords++
			signals.matchedKeywordCounts[kw]++
		}
	}

	// Feature detection
	multiFileIndicators := []string{
		"multiple files", "across files", "modules",
		"packages", "components", "services",
	}
	for _, kw := range multiFileIndicators {
		if strings.Contains(desc, kw) {
			signals.hasMultipleFiles = true
			break
		}
	}

	crossSysIndicators := []string{
		"cross", "integration", "sync", "api", "service",
		"system integration", "cross-system",
	}
	for _, kw := range crossSysIndicators {
		if strings.Contains(desc, kw) {
			signals.hasCrossSystems = true
			break
		}
	}

	if !signals.hasNewCodeCreation {
		modificationIndicators := []string{
			"modify", "change", "update", "alter", "edit",
		}
		for _, kw := range modificationIndicators {
			if strings.Contains(desc, kw) {
				signals.hasModifications = true
				break
			}
		}
	}

	// Check for user-facing impact
	if strings.Contains(desc, "user") || strings.Contains(desc, "frontend") ||
		strings.Contains(desc, "ui") || strings.Contains(desc, "api") {
		signals.hasUserImpact = true
	}

	return signals
}

// scoreTaskTypes scores each task type based on matched keywords.
func scoreTaskTypes(signals taskSignals) map[TaskType]*ClassificationLabel {
	scores := make(map[TaskType]*ClassificationLabel)

	// Initialize all task types
	allTypes := AllTaskTypes()
	for _, tt := range allTypes {
		scores[tt] = &ClassificationLabel{
			TaskType:        tt,
			Confidence:      0.0,
			MatchedKeywords: []string{},
		}
	}

	// Score each task type based on keyword matches
	scoreTypeByKeywords(signals, scores)

	// Apply confidence adjustments based on features
	adjustConfidenceByFeatures(signals, scores)

	// Filter out zero-confidence labels
	filtered := make(map[TaskType]*ClassificationLabel)
	for tt, label := range scores {
		if label.Confidence > 0.0 {
			filtered[tt] = label
		}
	}

	return filtered
}

// scoreTypeByKeywords calculates base confidence from keyword matches.
func scoreTypeByKeywords(signals taskSignals, scores map[TaskType]*ClassificationLabel) {
	// Use a weighted scoring system where more keywords = higher confidence
	// Base confidence starts higher and increases with matches

	typeScores := map[TaskType]int{
		TaskTypeResearch:       signals.researchKeywords,
		TaskTypeCodeGeneration: signals.codeGenKeywords,
		TaskTypeCodeReview:     signals.reviewKeywords,
		TaskTypeTesting:        signals.testingKeywords,
		TaskTypeDocumentation:  signals.documentationKeywords,
		TaskTypeDataProcessing: signals.dataProcessingKeywords,
		TaskTypeDebugging:      signals.debuggingKeywords,
		TaskTypeRefactoring:    signals.refactoringKeywords,
		TaskTypeArchitecture:   signals.architectureKeywords,
		TaskTypeConfiguration:  signals.configurationKeywords,
		TaskTypeDeployment:     signals.deploymentKeywords,
		TaskTypeSecurity:       signals.securityKeywords,
		TaskTypeVisual:         signals.visualKeywords,
		TaskTypeCommunication:  signals.communicationKeywords,
		TaskTypePipeline:       signals.pipelineKeywords,
	}

	// Calculate confidence for each type
	for tt, keywordCount := range typeScores {
		if keywordCount > 0 {
			// Base confidence starts at 0.5, then increases with keyword matches
			// 1 keyword: 0.5
			// 2 keywords: 0.75
			// 3+ keywords: 0.9+
			conf := 0.4 + (float64(keywordCount) * 0.2)
			if conf > 1.0 {
				conf = 1.0
			}
			scores[tt].Confidence = conf
			collectMatchedKeywords(signals, tt, scores)
		}
	}
}

// collectMatchedKeywords finds which keywords matched for a given task type.
func collectMatchedKeywords(signals taskSignals, tt TaskType, scores map[TaskType]*ClassificationLabel) {
	// Map of keywords for each task type (simplified version)
	keywordMap := map[TaskType][]string{
		TaskTypeResearch: {
			"research", "investigate", "explore", "analyze", "examine",
			"study", "survey", "review", "audit", "inspect",
		},
		TaskTypeCodeGeneration: {
			"implement", "write", "create", "code", "generate",
			"build", "construct", "add feature", "new endpoint",
		},
		TaskTypeCodeReview: {
			"review", "pr review", "code review", "pull request",
			"audit", "inspect", "analyze code", "check", "verify",
		},
		TaskTypeTesting: {
			"test", "testing", "unit test", "integration test",
			"test case", "test suite", "assert", "mock",
		},
		TaskTypeDocumentation: {
			"documentation", "document", "readme", "api doc",
			"write doc", "update doc", "comment", "docstring",
		},
		TaskTypeDataProcessing: {
			"data processing", "parse", "transform", "extract",
			"log analysis", "data migration", "json", "csv",
		},
		TaskTypeDebugging: {
			"debug", "debugging", "bug", "fix bug", "issue",
			"troubleshoot", "root cause", "stack trace",
		},
		TaskTypeRefactoring: {
			"refactor", "refactoring", "extract", "rename",
			"reorganize", "restructure", "cleanup",
		},
		TaskTypeArchitecture: {
			"architecture", "design", "system design", "schema",
			"pattern", "structure", "framework",
		},
		TaskTypeConfiguration: {
			"configuration", "config", "configure", "yaml",
			"environment", "env var", "setting",
		},
		TaskTypeDeployment: {
			"deploy", "deployment", "devops", "ci/cd", "pipeline",
			"docker", "kubernetes", "infrastructure",
		},
		TaskTypeSecurity: {
			"security", "auth", "authentication", "encrypt",
			"vulnerability", "breach", "credential",
		},
		TaskTypeVisual: {
			"visual", "ui", "frontend", "screenshot",
			"browser", "css", "html", "design",
		},
		TaskTypeCommunication: {
			"communication", "write", "message", "report",
			"pr description", "commit message", "status update",
		},
		TaskTypePipeline: {
			"pipeline", "orchestration", "coordinate", "multi-stage",
			"workflow", "sequence", "chain",
		},
	}

	if keywords, exists := keywordMap[tt]; exists {
		for kw := range signals.matchedKeywordCounts {
			for _, typeKW := range keywords {
				if strings.Contains(kw, strings.Split(typeKW, " ")[0]) ||
					strings.Contains(typeKW, strings.Split(kw, " ")[0]) {
					scores[tt].MatchedKeywords = append(scores[tt].MatchedKeywords, kw)
					break
				}
			}
		}
	}
}

// adjustConfidenceByFeatures applies feature-based adjustments to confidence scores.
func adjustConfidenceByFeatures(signals taskSignals, scores map[TaskType]*ClassificationLabel) {
	// Code generation confidence boost for multi-file or new code
	if signals.hasNewCodeCreation && signals.hasMultipleFiles {
		if score, ok := scores[TaskTypeCodeGeneration]; ok && score.Confidence > 0 {
			score.Confidence = score.Confidence * 1.2
		}
	}

	// Cross-system increases architecture and integration complexity
	if signals.hasCrossSystems {
		if score, ok := scores[TaskTypeArchitecture]; ok && score.Confidence > 0 {
			score.Confidence = score.Confidence * 1.15
		}
		if score, ok := scores[TaskTypePipeline]; ok && score.Confidence > 0 {
			score.Confidence = score.Confidence * 1.15
		}
	}

	// Data operations increase data processing confidence
	if signals.hasDataOperations {
		if score, ok := scores[TaskTypeDataProcessing]; ok && score.Confidence > 0 {
			score.Confidence = score.Confidence * 1.2
		}
	}

	// User impact can increase multiple types
	if signals.hasUserImpact {
		if score, ok := scores[TaskTypeCodeGeneration]; ok && score.Confidence > 0 {
			score.Confidence = score.Confidence * 1.1
		}
		if score, ok := scores[TaskTypeVisual]; ok && score.Confidence > 0 {
			score.Confidence = score.Confidence * 1.15
		}
	}

	// Normalize all scores back to 1.0 max
	for _, label := range scores {
		if label.Confidence > 1.0 {
			label.Confidence = 1.0
		}
	}
}

// sortLabelsByConfidence sorts classification labels by confidence (descending).
func sortLabelsByConfidence(scores map[TaskType]*ClassificationLabel) []ClassificationLabel {
	// Convert map to slice
	labels := make([]ClassificationLabel, 0, len(scores))
	for _, label := range scores {
		labels = append(labels, *label)
	}

	// Simple bubble sort (sufficient for small number of types)
	for i := 0; i < len(labels); i++ {
		for j := i + 1; j < len(labels); j++ {
			if labels[j].Confidence > labels[i].Confidence {
				// Swap
				labels[i], labels[j] = labels[j], labels[i]
			}
		}
	}

	return labels
}

// GetTaskTypeConfidence returns the confidence score for a specific task type.
func (cr *ClassificationResult) GetTaskTypeConfidence(tt TaskType) float64 {
	for _, label := range cr.Labels {
		if label.TaskType == tt {
			return label.Confidence
		}
	}
	return 0.0
}

// HasTaskType checks if a specific task type was classified.
func (cr *ClassificationResult) HasTaskType(tt TaskType) bool {
	return cr.GetTaskTypeConfidence(tt) > 0.0
}

// FilterByConfidence returns labels with confidence above the threshold.
func (cr *ClassificationResult) FilterByConfidence(threshold float64) []ClassificationLabel {
	filtered := []ClassificationLabel{}
	for _, label := range cr.Labels {
		if label.Confidence >= threshold {
			filtered = append(filtered, label)
		}
	}
	return filtered
}
