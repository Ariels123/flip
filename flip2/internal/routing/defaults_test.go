// Package routing provides intelligent task routing between different AI models.
package routing

import (
	"strings"
	"testing"
)

// ================================================================================
// DEFAULT ROUTING MATRIX TESTS
// ================================================================================

// TestDefaultRoutingMatrix tests that the default matrix is properly structured.
func TestDefaultRoutingMatrix(t *testing.T) {
	matrix := GetDefaultRoutingMatrix()

	if matrix == nil {
		t.Fatal("DefaultRoutingMatrix should not be nil")
	}

	// Check that all task types are represented
	allTaskTypes := AllTaskTypes()
	if len(matrix) != len(allTaskTypes) {
		t.Errorf("Matrix has %d task types, expected %d", len(matrix), len(allTaskTypes))
	}

	// Check each task type
	for _, taskType := range allTaskTypes {
		if _, exists := matrix[taskType]; !exists {
			t.Errorf("Task type %s missing from matrix", taskType)
		}
	}
}

// TestMatrixCompleteness ensures all complexity levels 1-5 are defined for all tasks.
func TestMatrixCompleteness(t *testing.T) {
	matrix := GetDefaultRoutingMatrix()

	for taskType, taskMap := range matrix {
		// Check that complexity levels 1-5 are all present
		for level := 1; level <= 5; level++ {
			if _, exists := taskMap[level]; !exists {
				t.Errorf("Task type %s missing complexity level %d", taskType, level)
			}
		}

		// Check that no unexpected levels are present
		for level := range taskMap {
			if level < 1 || level > 5 {
				t.Errorf("Task type %s has unexpected complexity level %d", taskType, level)
			}
		}
	}
}

// TestMatrixValidModels ensures all models in the matrix are valid.
func TestMatrixValidModels(t *testing.T) {
	matrix := GetDefaultRoutingMatrix()
	validModels := AllModels()

	for taskType, taskMap := range matrix {
		for level, model := range taskMap {
			// Check if model is in valid models list
			found := false
			for _, validModel := range validModels {
				if model == validModel {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Task type %s level %d has invalid model %s", taskType, level, model)
			}
		}
	}
}

// TestCriticalTasksUseOpus ensures Architecture and Security tasks always use Opus.
func TestCriticalTasksUseOpus(t *testing.T) {
	matrix := GetDefaultRoutingMatrix()

	criticalTypes := []TaskType{TaskTypeArchitecture, TaskTypeSecurity}

	for _, criticalType := range criticalTypes {
		if taskMap, exists := matrix[criticalType]; exists {
			for level := 1; level <= 5; level++ {
				if model, exists := taskMap[level]; exists {
					if model != ModelOpus {
						t.Errorf("%s complexity %d should use Opus, got %s", criticalType, level, model)
					}
				}
			}
		}
	}
}

// TestVisualTasksUseAntigravity ensures Visual tasks always use Antigravity.
func TestVisualTasksUseAntigravity(t *testing.T) {
	matrix := GetDefaultRoutingMatrix()

	if taskMap, exists := matrix[TaskTypeVisual]; exists {
		for level := 1; level <= 5; level++ {
			if model, exists := taskMap[level]; exists {
				if model != ModelAntigravity {
					t.Errorf("Visual task complexity %d should use Antigravity, got %s", level, model)
				}
			}
		}
	}
}

// TestLookupRoute tests the basic lookup function.
func TestLookupRoute(t *testing.T) {
	testCases := []struct {
		taskType    TaskType
		complexity  int
		expectedMin Model // At least should be one of these
	}{
		{TaskTypeCodeGeneration, 1, ModelHaiku},
		{TaskTypeCodeGeneration, 2, ModelHaiku},
		{TaskTypeCodeGeneration, 3, ModelSonnet},
		{TaskTypeCodeGeneration, 4, ModelSonnet},
		{TaskTypeCodeGeneration, 5, ModelOpus},
		{TaskTypeResearch, 1, ModelGemini},
		{TaskTypeResearch, 2, ModelGemini},
		{TaskTypeResearch, 3, ModelOpus},
		{TaskTypeTesting, 1, ModelHaiku},
		{TaskTypeTesting, 3, ModelHaiku},
		{TaskTypeSecurity, 1, ModelOpus},
		{TaskTypeSecurity, 5, ModelOpus},
		{TaskTypeArchitecture, 3, ModelOpus},
	}

	for _, tc := range testCases {
		model := LookupRoute(tc.taskType, tc.complexity)
		if model != tc.expectedMin {
			t.Errorf("LookupRoute(%s, %d) = %s, expected %s",
				tc.taskType, tc.complexity, model, tc.expectedMin)
		}
	}
}

// TestLookupRouteOutOfBounds tests that out-of-bounds complexity is clamped.
func TestLookupRouteOutOfBounds(t *testing.T) {
	testCases := []struct {
		taskType   TaskType
		complexity int
		expected   Model
	}{
		{TaskTypeCodeGeneration, 0, ModelHaiku},  // Should clamp to 1
		{TaskTypeCodeGeneration, -5, ModelHaiku}, // Should clamp to 1
		{TaskTypeCodeGeneration, 10, ModelOpus},  // Should clamp to 5
		{TaskTypeCodeGeneration, 100, ModelOpus}, // Should clamp to 5
	}

	for _, tc := range testCases {
		model := LookupRoute(tc.taskType, tc.complexity)
		if model != tc.expected {
			t.Errorf("LookupRoute(%s, %d) = %s, expected %s (out of bounds test)",
				tc.taskType, tc.complexity, model, tc.expected)
		}
	}
}

// TestLookupRouteWithScore tests floating-point score rounding.
func TestLookupRouteWithScore(t *testing.T) {
	testCases := []struct {
		taskType           TaskType
		complexityScore    float64
		expectedComplexity int // For verification
	}{
		{TaskTypeCodeGeneration, 1.0, 1},
		{TaskTypeCodeGeneration, 1.4, 1},
		{TaskTypeCodeGeneration, 1.5, 2},
		{TaskTypeCodeGeneration, 2.9, 3},
		{TaskTypeCodeGeneration, 3.0, 3},
		{TaskTypeCodeGeneration, 4.6, 5},
		{TaskTypeCodeGeneration, 5.0, 5},
	}

	for _, tc := range testCases {
		model := LookupRouteWithScore(tc.taskType, tc.complexityScore)
		expectedModel := LookupRoute(tc.taskType, tc.expectedComplexity)

		if model != expectedModel {
			t.Errorf("LookupRouteWithScore(%s, %.1f) = %s, expected %s",
				tc.taskType, tc.complexityScore, model, expectedModel)
		}
	}
}

// TestLookupRouteFallback tests the fallback behavior for invalid task types.
func TestLookupRouteFallback(t *testing.T) {
	// Even though we don't have invalid task types, we can test the behavior
	// by checking that an unsupported hypothetical type returns the default
	model := LookupRoute(TaskType("nonexistent"), 3)
	if model != ModelSonnet {
		t.Errorf("LookupRoute with invalid task type should fallback to Sonnet, got %s", model)
	}
}

// TestValidateMatrix tests the validation function.
func TestValidateMatrix(t *testing.T) {
	errs := ValidateMatrix()
	if len(errs) > 0 {
		t.Errorf("Matrix validation failed with %d errors:", len(errs))
		for _, err := range errs {
			t.Log("  -", err)
		}
	}
}

// TestDescribeRoute tests the human-readable description.
func TestDescribeRoute(t *testing.T) {
	desc := DescribeRoute(TaskTypeCodeGeneration, 3)
	if desc == "" {
		t.Fatal("DescribeRoute returned empty string")
	}

	// Check that description contains key information
	expectedStrings := []string{
		"Code generation",
		"complexity 3",
		"Moderate",
		"Sonnet",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(desc, expected) {
			t.Errorf("DescribeRoute missing expected string '%s': %s", expected, desc)
		}
	}
}

// TestDescribeRouteAllCombinations tests description for all combinations.
func TestDescribeRouteAllCombinations(t *testing.T) {
	for _, taskType := range AllTaskTypes() {
		for level := 1; level <= 5; level++ {
			desc := DescribeRoute(taskType, level)
			if desc == "" {
				t.Errorf("DescribeRoute(%s, %d) returned empty string", taskType, level)
			}
			// Basic sanity check - should contain complexity level info
			complexityStr := []string{"Trivial", "Simple", "Moderate", "Complex", "Highly Complex"}[level-1]
			if !strings.Contains(desc, complexityStr) && !strings.Contains(desc, "complexity") {
				t.Errorf("DescribeRoute missing complexity info for level %d: %s", level, desc)
			}
		}
	}
}

// TestGetMatrixAsTable tests the table export function.
func TestGetMatrixAsTable(t *testing.T) {
	table := GetMatrixAsTable()

	// Should have 15 task types * 5 complexity levels = 75 entries
	expected := len(AllTaskTypes()) * 5
	if len(table) != expected {
		t.Errorf("GetMatrixAsTable returned %d entries, expected %d", len(table), expected)
	}

	// Check that each entry is valid
	for _, entry := range table {
		if !entry.TaskType.IsValid() {
			t.Errorf("Invalid task type in table: %s", entry.TaskType)
		}
		if entry.ComplexityLevel < 1 || entry.ComplexityLevel > 5 {
			t.Errorf("Invalid complexity level in table: %d", entry.ComplexityLevel)
		}
		if !entry.RecommendedModel.IsValid() {
			t.Errorf("Invalid model in table: %s", entry.RecommendedModel)
		}
		if entry.Justification == "" {
			t.Errorf("Empty justification for %s complexity %d", entry.TaskType, entry.ComplexityLevel)
		}
	}
}

// TestRoutingConsistency verifies that routing decisions are consistent.
// The same task should always route to the same model.
func TestRoutingConsistency(t *testing.T) {
	testCases := []struct {
		taskType   TaskType
		complexity int
	}{
		{TaskTypeCodeGeneration, 3},
		{TaskTypeResearch, 2},
		{TaskTypeArchitecture, 1},
		{TaskTypeSecurity, 5},
		{TaskTypeTesting, 2},
	}

	for _, tc := range testCases {
		// Get the same route multiple times
		models := []Model{
			LookupRoute(tc.taskType, tc.complexity),
			LookupRoute(tc.taskType, tc.complexity),
			LookupRoute(tc.taskType, tc.complexity),
		}

		// All should be identical
		for i := 1; i < len(models); i++ {
			if models[i] != models[0] {
				t.Errorf("Routing inconsistency for %s complexity %d: %s vs %s",
					tc.taskType, tc.complexity, models[0], models[i])
			}
		}
	}
}

// TestResearchComplexityEscalation tests that research tasks escalate correctly.
// Simple research -> Gemini, Complex research -> Opus
func TestResearchComplexityEscalation(t *testing.T) {
	// Low complexity should use Gemini (cheaper)
	lowModel := LookupRoute(TaskTypeResearch, 1)
	if lowModel != ModelGemini {
		t.Errorf("Research complexity 1 should use Gemini, got %s", lowModel)
	}
	lowModel2 := LookupRoute(TaskTypeResearch, 2)
	if lowModel2 != ModelGemini {
		t.Errorf("Research complexity 2 should use Gemini, got %s", lowModel2)
	}

	// High complexity should use Opus (better quality)
	highModel := LookupRoute(TaskTypeResearch, 3)
	if highModel != ModelOpus {
		t.Errorf("Research complexity 3 should use Opus, got %s", highModel)
	}
	highModel2 := LookupRoute(TaskTypeResearch, 5)
	if highModel2 != ModelOpus {
		t.Errorf("Research complexity 5 should use Opus, got %s", highModel2)
	}
}

// TestCodeGenerationEscalation tests complexity escalation for code generation.
// 1-2 -> Haiku, 3-4 -> Sonnet, 5 -> Opus
func TestCodeGenerationEscalation(t *testing.T) {
	testCases := []struct {
		complexity int
		expected   Model
	}{
		{1, ModelHaiku},
		{2, ModelHaiku},
		{3, ModelSonnet},
		{4, ModelSonnet},
		{5, ModelOpus},
	}

	for _, tc := range testCases {
		model := LookupRoute(TaskTypeCodeGeneration, tc.complexity)
		if model != tc.expected {
			t.Errorf("CodeGeneration complexity %d: got %s, expected %s",
				tc.complexity, model, tc.expected)
		}
	}
}

// TestDeploymentRiskAversion tests that deployment tasks avoid using Haiku.
// Deployment is risky, so should use at least Sonnet
func TestDeploymentRiskAversion(t *testing.T) {
	for level := 1; level <= 5; level++ {
		model := LookupRoute(TaskTypeDeployment, level)
		if model == ModelHaiku || model == ModelGemini {
			t.Errorf("Deployment complexity %d should not use %s (too risky)", level, model)
		}
	}
}

// TestTestingEfficiency ensures testing tasks prioritize Haiku for efficiency.
// Testing is high-volume, so should default to Haiku
func TestTestingEfficiency(t *testing.T) {
	// Levels 1-3 should use Haiku
	for level := 1; level <= 3; level++ {
		model := LookupRoute(TaskTypeTesting, level)
		if model != ModelHaiku {
			t.Errorf("Testing complexity %d should use Haiku for efficiency, got %s", level, model)
		}
	}

	// Even level 4 should use Haiku typically
	model4 := LookupRoute(TaskTypeTesting, 4)
	if model4 != ModelHaiku && model4 != ModelSonnet {
		t.Errorf("Testing complexity 4 should prefer Haiku/Sonnet, got %s", model4)
	}
}

// TestMatrixErrorHandling tests error handling in ValidateMatrix.
func TestMatrixErrorHandling(t *testing.T) {
	errs := ValidateMatrix()

	// Should be no errors in the default matrix
	if len(errs) > 0 {
		t.Logf("Matrix validation found errors (may be expected):")
		for _, err := range errs {
			t.Log("  -", err)
		}
	}
}

// BenchmarkLookupRoute benchmarks the routing lookup performance.
func BenchmarkLookupRoute(b *testing.B) {
	taskType := TaskTypeCodeGeneration
	complexity := 3

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		LookupRoute(taskType, complexity)
	}
}

// BenchmarkLookupRouteWithScore benchmarks floating-point score lookup.
func BenchmarkLookupRouteWithScore(b *testing.B) {
	taskType := TaskTypeCodeGeneration
	score := 3.5

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		LookupRouteWithScore(taskType, score)
	}
}

// BenchmarkGetMatrixAsTable benchmarks table generation.
func BenchmarkGetMatrixAsTable(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		GetMatrixAsTable()
	}
}

// TestMatrixDocumentation is a test that also serves as documentation.
// It demonstrates how to use the routing matrix.
func TestMatrixDocumentation(t *testing.T) {
	t.Log("=== Default Routing Matrix Usage Examples ===")

	examples := []struct {
		description string
		taskType    TaskType
		complexity  int
	}{
		{"Simple variable rename", TaskTypeCodeGeneration, 1},
		{"Add a new function", TaskTypeCodeGeneration, 2},
		{"Implement a feature with multiple files", TaskTypeCodeGeneration, 3},
		{"Cross-system integration", TaskTypeCodeGeneration, 4},
		{"Design a new system", TaskTypeArchitecture, 1}, // Always Opus
		{"Research API documentation", TaskTypeResearch, 1},
		{"Analyze complex system interactions", TaskTypeResearch, 3},
		{"Write a simple test", TaskTypeTesting, 1},
		{"Complex integration test", TaskTypeTesting, 4},
		{"Security vulnerability review", TaskTypeSecurity, 1}, // Always Opus
		{"Update README", TaskTypeDocumentation, 1},
	}

	for _, ex := range examples {
		model := LookupRoute(ex.taskType, ex.complexity)
		desc := DescribeRoute(ex.taskType, ex.complexity)
		t.Logf("  %s -> %s", ex.description, model)
		t.Logf("    %s", desc)
	}
}
