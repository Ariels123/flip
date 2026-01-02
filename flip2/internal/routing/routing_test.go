// Package routing provides intelligent task routing between different AI models.
package routing

import (
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ================================================================================
// TEST: DEFAULT ROUTING MATRIX LOOKUPS
// ================================================================================

// TestRoutingMatrixCoverage verifies all routing paths through the default matrix.
// This ensures every task type and complexity level has a defined route.
func TestRoutingMatrixCoverage(t *testing.T) {
	matrix := GetDefaultRoutingMatrix()

	tests := []struct {
		name          string
		taskType      TaskType
		complexity    int
		expectedModel Model
		description   string
	}{
		// Research routing
		{"Research complexity 1", TaskTypeResearch, 1, ModelGemini, "Research 1 -> Gemini"},
		{"Research complexity 2", TaskTypeResearch, 2, ModelGemini, "Research 2 -> Gemini"},
		{"Research complexity 3", TaskTypeResearch, 3, ModelOpus, "Research 3 -> Opus"},
		{"Research complexity 4", TaskTypeResearch, 4, ModelOpus, "Research 4 -> Opus"},
		{"Research complexity 5", TaskTypeResearch, 5, ModelOpus, "Research 5 -> Opus"},

		// Code generation routing
		{"CodeGen complexity 1", TaskTypeCodeGeneration, 1, ModelHaiku, "CodeGen 1 -> Haiku"},
		{"CodeGen complexity 2", TaskTypeCodeGeneration, 2, ModelHaiku, "CodeGen 2 -> Haiku"},
		{"CodeGen complexity 3", TaskTypeCodeGeneration, 3, ModelSonnet, "CodeGen 3 -> Sonnet"},
		{"CodeGen complexity 4", TaskTypeCodeGeneration, 4, ModelSonnet, "CodeGen 4 -> Sonnet"},
		{"CodeGen complexity 5", TaskTypeCodeGeneration, 5, ModelOpus, "CodeGen 5 -> Opus"},

		// Code review routing
		{"CodeReview complexity 1", TaskTypeCodeReview, 1, ModelHaiku, "CodeReview 1 -> Haiku"},
		{"CodeReview complexity 2", TaskTypeCodeReview, 2, ModelHaiku, "CodeReview 2 -> Haiku"},
		{"CodeReview complexity 3", TaskTypeCodeReview, 3, ModelSonnet, "CodeReview 3 -> Sonnet"},
		{"CodeReview complexity 4", TaskTypeCodeReview, 4, ModelSonnet, "CodeReview 4 -> Sonnet"},
		{"CodeReview complexity 5", TaskTypeCodeReview, 5, ModelOpus, "CodeReview 5 -> Opus"},

		// Testing routing
		{"Testing complexity 1", TaskTypeTesting, 1, ModelHaiku, "Testing 1 -> Haiku"},
		{"Testing complexity 2", TaskTypeTesting, 2, ModelHaiku, "Testing 2 -> Haiku"},
		{"Testing complexity 3", TaskTypeTesting, 3, ModelHaiku, "Testing 3 -> Haiku"},
		{"Testing complexity 4", TaskTypeTesting, 4, ModelSonnet, "Testing 4 -> Sonnet"},
		{"Testing complexity 5", TaskTypeTesting, 5, ModelSonnet, "Testing 5 -> Sonnet"},

		// Documentation routing
		{"Documentation complexity 1", TaskTypeDocumentation, 1, ModelHaiku, "Documentation 1 -> Haiku"},
		{"Documentation complexity 2", TaskTypeDocumentation, 2, ModelHaiku, "Documentation 2 -> Haiku"},
		{"Documentation complexity 3", TaskTypeDocumentation, 3, ModelHaiku, "Documentation 3 -> Haiku"},
		{"Documentation complexity 4", TaskTypeDocumentation, 4, ModelSonnet, "Documentation 4 -> Sonnet"},
		{"Documentation complexity 5", TaskTypeDocumentation, 5, ModelSonnet, "Documentation 5 -> Sonnet"},

		// Data processing routing
		{"DataProcessing complexity 1", TaskTypeDataProcessing, 1, ModelGemini, "DataProcessing 1 -> Gemini"},
		{"DataProcessing complexity 2", TaskTypeDataProcessing, 2, ModelGemini, "DataProcessing 2 -> Gemini"},
		{"DataProcessing complexity 3", TaskTypeDataProcessing, 3, ModelHaiku, "DataProcessing 3 -> Haiku"},
		{"DataProcessing complexity 4", TaskTypeDataProcessing, 4, ModelSonnet, "DataProcessing 4 -> Sonnet"},
		{"DataProcessing complexity 5", TaskTypeDataProcessing, 5, ModelSonnet, "DataProcessing 5 -> Sonnet"},

		// Debugging routing
		{"Debugging complexity 1", TaskTypeDebugging, 1, ModelHaiku, "Debugging 1 -> Haiku"},
		{"Debugging complexity 2", TaskTypeDebugging, 2, ModelHaiku, "Debugging 2 -> Haiku"},
		{"Debugging complexity 3", TaskTypeDebugging, 3, ModelSonnet, "Debugging 3 -> Sonnet"},
		{"Debugging complexity 4", TaskTypeDebugging, 4, ModelSonnet, "Debugging 4 -> Sonnet"},
		{"Debugging complexity 5", TaskTypeDebugging, 5, ModelOpus, "Debugging 5 -> Opus"},

		// Refactoring routing
		{"Refactoring complexity 1", TaskTypeRefactoring, 1, ModelHaiku, "Refactoring 1 -> Haiku"},
		{"Refactoring complexity 2", TaskTypeRefactoring, 2, ModelHaiku, "Refactoring 2 -> Haiku"},
		{"Refactoring complexity 3", TaskTypeRefactoring, 3, ModelSonnet, "Refactoring 3 -> Sonnet"},
		{"Refactoring complexity 4", TaskTypeRefactoring, 4, ModelSonnet, "Refactoring 4 -> Sonnet"},
		{"Refactoring complexity 5", TaskTypeRefactoring, 5, ModelOpus, "Refactoring 5 -> Opus"},

		// Architecture routing (all -> Opus)
		{"Architecture complexity 1", TaskTypeArchitecture, 1, ModelOpus, "Architecture always Opus"},
		{"Architecture complexity 5", TaskTypeArchitecture, 5, ModelOpus, "Architecture always Opus"},

		// Configuration routing
		{"Configuration complexity 1", TaskTypeConfiguration, 1, ModelHaiku, "Configuration 1 -> Haiku"},
		{"Configuration complexity 2", TaskTypeConfiguration, 2, ModelHaiku, "Configuration 2 -> Haiku"},
		{"Configuration complexity 3", TaskTypeConfiguration, 3, ModelSonnet, "Configuration 3 -> Sonnet"},
		{"Configuration complexity 4", TaskTypeConfiguration, 4, ModelSonnet, "Configuration 4 -> Sonnet"},
		{"Configuration complexity 5", TaskTypeConfiguration, 5, ModelOpus, "Configuration 5 -> Opus"},

		// Deployment routing
		{"Deployment complexity 1", TaskTypeDeployment, 1, ModelSonnet, "Deployment 1 -> Sonnet"},
		{"Deployment complexity 2", TaskTypeDeployment, 2, ModelSonnet, "Deployment 2 -> Sonnet"},
		{"Deployment complexity 3", TaskTypeDeployment, 3, ModelSonnet, "Deployment 3 -> Sonnet"},
		{"Deployment complexity 4", TaskTypeDeployment, 4, ModelOpus, "Deployment 4 -> Opus"},
		{"Deployment complexity 5", TaskTypeDeployment, 5, ModelOpus, "Deployment 5 -> Opus"},

		// Security routing (all -> Opus)
		{"Security complexity 1", TaskTypeSecurity, 1, ModelOpus, "Security always Opus"},
		{"Security complexity 5", TaskTypeSecurity, 5, ModelOpus, "Security always Opus"},

		// Visual routing (all -> Antigravity)
		{"Visual complexity 1", TaskTypeVisual, 1, ModelAntigravity, "Visual always Antigravity"},
		{"Visual complexity 5", TaskTypeVisual, 5, ModelAntigravity, "Visual always Antigravity"},

		// Communication routing
		{"Communication complexity 1", TaskTypeCommunication, 1, ModelHaiku, "Communication 1 -> Haiku"},
		{"Communication complexity 2", TaskTypeCommunication, 2, ModelHaiku, "Communication 2 -> Haiku"},
		{"Communication complexity 3", TaskTypeCommunication, 3, ModelSonnet, "Communication 3 -> Sonnet"},
		{"Communication complexity 4", TaskTypeCommunication, 4, ModelSonnet, "Communication 4 -> Sonnet"},
		{"Communication complexity 5", TaskTypeCommunication, 5, ModelOpus, "Communication 5 -> Opus"},

		// Pipeline routing
		{"Pipeline complexity 1", TaskTypePipeline, 1, ModelSonnet, "Pipeline 1 -> Sonnet"},
		{"Pipeline complexity 2", TaskTypePipeline, 2, ModelSonnet, "Pipeline 2 -> Sonnet"},
		{"Pipeline complexity 3", TaskTypePipeline, 3, ModelSonnet, "Pipeline 3 -> Sonnet"},
		{"Pipeline complexity 4", TaskTypePipeline, 4, ModelSonnet, "Pipeline 4 -> Sonnet"},
		{"Pipeline complexity 5", TaskTypePipeline, 5, ModelOpus, "Pipeline 5 -> Opus"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := LookupRoute(tt.taskType, tt.complexity)
			if result != tt.expectedModel {
				t.Errorf("LookupRoute(%s, %d) = %s, want %s. %s",
					tt.taskType, tt.complexity, result, tt.expectedModel, tt.description)
			}

			// Verify matrix directly
			if taskMap, exists := matrix[tt.taskType]; exists {
				if model, exists := taskMap[tt.complexity]; exists {
					if model != tt.expectedModel {
						t.Errorf("Matrix[%s][%d] = %s, want %s",
							tt.taskType, tt.complexity, model, tt.expectedModel)
					}
				}
			}
		})
	}
}

// ================================================================================
// TEST: CUSTOM YAML RULES
// ================================================================================

// TestCustomRulesLoading tests loading and applying custom routing rules from YAML.
func TestCustomRulesLoading(t *testing.T) {
	engine := NewRulesEngine()

	tests := []struct {
		name          string
		rule          RoutingRule
		taskType      TaskType
		complexity    float64
		expectedMatch bool
		description   string
	}{
		{
			name: "Task type specific rule matches",
			rule: RoutingRule{
				TaskType:    TaskTypeSecurity,
				TargetModel: ModelOpus,
				Priority:    1000,
			},
			taskType:      TaskTypeSecurity,
			complexity:    2.0,
			expectedMatch: true,
			description:   "Security rule should match security tasks",
		},
		{
			name: "Task type mismatch",
			rule: RoutingRule{
				TaskType:    TaskTypeSecurity,
				TargetModel: ModelOpus,
			},
			taskType:      TaskTypeCodeGeneration,
			complexity:    2.0,
			expectedMatch: false,
			description:   "Security rule should not match code generation",
		},
		{
			name: "Complexity range rule matches",
			rule: RoutingRule{
				MinComplexity: 2.5,
				MaxComplexity: 4.0,
				TargetModel:   ModelSonnet,
				Priority:      500,
			},
			taskType:      TaskTypeCodeGeneration,
			complexity:    3.0,
			expectedMatch: true,
			description:   "Range rule should match within bounds",
		},
		{
			name: "Complexity range below minimum",
			rule: RoutingRule{
				MinComplexity: 2.5,
				MaxComplexity: 4.0,
				TargetModel:   ModelSonnet,
			},
			taskType:      TaskTypeCodeGeneration,
			complexity:    2.0,
			expectedMatch: false,
			description:   "Range rule should not match below minimum",
		},
		{
			name: "Complexity range above maximum",
			rule: RoutingRule{
				MinComplexity: 2.5,
				MaxComplexity: 4.0,
				TargetModel:   ModelSonnet,
			},
			taskType:      TaskTypeCodeGeneration,
			complexity:    4.5,
			expectedMatch: false,
			description:   "Range rule should not match above maximum",
		},
		{
			name: "Min complexity only",
			rule: RoutingRule{
				MinComplexity: 4.0,
				TargetModel:   ModelOpus,
				Priority:      800,
			},
			taskType:      TaskTypeCodeGeneration,
			complexity:    4.5,
			expectedMatch: true,
			description:   "Min-only rule should match above threshold",
		},
		{
			name: "Max complexity only",
			rule: RoutingRule{
				MaxComplexity: 2.5,
				TargetModel:   ModelHaiku,
				Priority:      300,
			},
			taskType:      TaskTypeCodeGeneration,
			complexity:    2.0,
			expectedMatch: true,
			description:   "Max-only rule should match below threshold",
		},
		{
			name: "Catch-all rule",
			rule: RoutingRule{
				TargetModel: ModelSonnet,
				Priority:    0,
			},
			taskType:      TaskTypeResearch,
			complexity:    3.0,
			expectedMatch: true,
			description:   "Catch-all rule should match everything",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := engine.ruleMatches(tt.rule, tt.taskType, tt.complexity)
			if result != tt.expectedMatch {
				t.Errorf("ruleMatches() = %v, want %v. %s",
					result, tt.expectedMatch, tt.description)
			}
		})
	}
}

// ================================================================================
// TEST: OVERRIDE MECHANISM
// ================================================================================

// TestOverrideMechanism tests the override precedence and functionality.
func TestOverrideMechanism(t *testing.T) {
	engine := NewRulesEngine()

	tests := []struct {
		name          string
		taskID        string
		override      Model
		taskType      TaskType
		complexity    float64
		expectedModel Model
		description   string
	}{
		{
			name:          "Override takes precedence",
			taskID:        "override-test-1",
			override:      ModelOpus,
			taskType:      TaskTypeTesting,
			complexity:    1.0,
			expectedModel: ModelOpus,
			description:   "Override should route to Opus despite testing->Haiku rule",
		},
		{
			name:          "No override follows rules",
			taskID:        "no-override-1",
			override:      "",
			taskType:      TaskTypeTesting,
			complexity:    1.0,
			expectedModel: ModelHaiku,
			description:   "Without override, testing should route to Haiku",
		},
		{
			name:          "Override persists across calls",
			taskID:        "persist-test-1",
			override:      ModelSonnet,
			taskType:      TaskTypeDocumentation,
			complexity:    1.0,
			expectedModel: ModelSonnet,
			description:   "Override should persist and take effect",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.override != "" {
				err := engine.SetOverride(tt.taskID, tt.override)
				if err != nil {
					t.Fatalf("SetOverride failed: %v", err)
				}
			}

			result := engine.RouteTask(tt.taskID, tt.taskType, tt.complexity)
			if result != tt.expectedModel {
				t.Errorf("RouteTask() = %s, want %s. %s",
					result, tt.expectedModel, tt.description)
			}
		})
	}
}

// TestOverrideManagement tests override CRUD operations.
func TestOverrideManagement(t *testing.T) {
	engine := NewRulesEngine()

	// Test SetOverride
	taskID := "manage-override-1"
	err := engine.SetOverride(taskID, ModelOpus)
	if err != nil {
		t.Fatalf("SetOverride failed: %v", err)
	}

	// Test GetOverride
	model, exists := engine.GetOverride(taskID)
	if !exists {
		t.Error("GetOverride: expected override to exist")
	}
	if model != ModelOpus {
		t.Errorf("GetOverride: got %s, want ModelOpus", model)
	}

	// Test invalid model
	err = engine.SetOverride(taskID, Model("invalid"))
	if err == nil {
		t.Error("SetOverride: expected error for invalid model")
	}

	// Test empty task ID
	err = engine.SetOverride("", ModelOpus)
	if err == nil {
		t.Error("SetOverride: expected error for empty task ID")
	}

	// Test ClearOverride
	engine.ClearOverride(taskID)
	_, exists = engine.GetOverride(taskID)
	if exists {
		t.Error("ClearOverride: expected override to be cleared")
	}

	// Test GetOverride on non-existent key
	_, exists = engine.GetOverride("nonexistent")
	if exists {
		t.Error("GetOverride: expected false for non-existent override")
	}
}

// ================================================================================
// TEST: A/B TEST ROUTING
// ================================================================================

// TestABTestConfiguration tests A/B test setup and configuration.
func TestABTestConfiguration(t *testing.T) {
	engine := NewRulesEngine()

	tests := []struct {
		name        string
		config      *ABTestConfig
		wantErr     bool
		checkFn     func(*testing.T, *RulesEngine)
		description string
	}{
		{
			name: "Valid AB test config",
			config: &ABTestConfig{
				Percentage:   50,
				VariantModel: ModelHaiku,
				ControlModel: ModelSonnet,
				Enabled:      false, // Will be set by EnableABTest
			},
			wantErr: false,
			checkFn: func(t *testing.T, e *RulesEngine) {
				if e.ABTest == nil {
					t.Error("ABTest should be set")
				}
				if !e.ABTest.Enabled {
					t.Error("ABTest should be enabled")
				}
				if e.ABTest.Percentage != 50 {
					t.Errorf("Percentage = %d, want 50", e.ABTest.Percentage)
				}
			},
			description: "AB test should configure correctly",
		},
		{
			name: "Invalid percentage too low",
			config: &ABTestConfig{
				Percentage:   -1,
				VariantModel: ModelHaiku,
				ControlModel: ModelSonnet,
			},
			wantErr:     true,
			description: "Negative percentage should error",
		},
		{
			name: "Invalid percentage too high",
			config: &ABTestConfig{
				Percentage:   101,
				VariantModel: ModelHaiku,
				ControlModel: ModelSonnet,
			},
			wantErr:     true,
			description: "Percentage > 100 should error",
		},
		{
			name: "Invalid variant model",
			config: &ABTestConfig{
				Percentage:   50,
				VariantModel: Model("invalid"),
				ControlModel: ModelSonnet,
			},
			wantErr:     true,
			description: "Invalid variant model should error",
		},
		{
			name: "Invalid control model",
			config: &ABTestConfig{
				Percentage:   50,
				VariantModel: ModelHaiku,
				ControlModel: Model("invalid"),
			},
			wantErr:     true,
			description: "Invalid control model should error",
		},
		{
			name:        "Nil config",
			config:      nil,
			wantErr:     true,
			description: "Nil config should error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := engine.EnableABTest(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("EnableABTest() error = %v, wantErr %v. %s",
					err, tt.wantErr, tt.description)
				return
			}

			if !tt.wantErr && tt.checkFn != nil {
				tt.checkFn(t, engine)
			}
		})
	}
}

// TestABTestDisable tests disabling A/B tests.
func TestABTestDisable(t *testing.T) {
	engine := NewRulesEngine()

	// Enable A/B test
	config := &ABTestConfig{
		Percentage:   50,
		VariantModel: ModelHaiku,
		ControlModel: ModelSonnet,
	}
	err := engine.EnableABTest(config)
	if err != nil {
		t.Fatalf("EnableABTest failed: %v", err)
	}

	if !engine.ABTest.Enabled {
		t.Error("ABTest should be enabled")
	}

	// Disable A/B test
	engine.DisableABTest()
	if engine.ABTest.Enabled {
		t.Error("ABTest should be disabled after DisableABTest")
	}

	// Disable when nil should not panic
	engine.ABTest = nil
	engine.DisableABTest() // Should not panic
}

// TestABTestBoundaryPercentages tests edge case percentages.
func TestABTestBoundaryPercentages(t *testing.T) {
	tests := []struct {
		name       string
		percentage int
		wantErr    bool
	}{
		{"Percentage 0", 0, false},
		{"Percentage 1", 1, false},
		{"Percentage 50", 50, false},
		{"Percentage 99", 99, false},
		{"Percentage 100", 100, false},
		{"Percentage -1", -1, true},
		{"Percentage 101", 101, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := NewRulesEngine()
			config := &ABTestConfig{
				Percentage:   tt.percentage,
				VariantModel: ModelHaiku,
				ControlModel: ModelSonnet,
			}
			err := engine.EnableABTest(config)
			if (err != nil) != tt.wantErr {
				t.Errorf("EnableABTest(%d) error = %v, wantErr %v",
					tt.percentage, err, tt.wantErr)
			}
		})
	}
}

// ================================================================================
// TEST: FALLBACK ROUTING
// ================================================================================

// TestFallbackWhenNoMatch tests fallback routing when no rules match.
func TestFallbackWhenNoMatch(t *testing.T) {
	tests := []struct {
		name             string
		defaultModel     Model
		rules            []RoutingRule
		taskType         TaskType
		complexity       float64
		expectedFallback Model
		description      string
	}{
		{
			name:             "Empty rules uses default",
			defaultModel:     ModelSonnet,
			rules:            []RoutingRule{},
			taskType:         TaskTypeCodeGeneration,
			complexity:       2.0,
			expectedFallback: ModelSonnet,
			description:      "With no rules, should use default model",
		},
		{
			name:         "No matching rules uses default",
			defaultModel: ModelHaiku,
			rules: []RoutingRule{
				{
					TaskType:    TaskTypeSecurity,
					TargetModel: ModelOpus,
					Priority:    1000,
				},
			},
			taskType:         TaskTypeResearch,
			complexity:       2.0,
			expectedFallback: ModelHaiku,
			description:      "Non-matching task should use default",
		},
		{
			name:         "Complexity out of range uses default",
			defaultModel: ModelOpus,
			rules: []RoutingRule{
				{
					MinComplexity: 3.0,
					MaxComplexity: 4.0,
					TargetModel:   ModelSonnet,
					Priority:      500,
				},
			},
			taskType:         TaskTypeCodeGeneration,
			complexity:       2.0,
			expectedFallback: ModelOpus,
			description:      "Complexity out of range should use default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := &RulesEngine{
				Rules:        tt.rules,
				DefaultModel: tt.defaultModel,
				Overrides:    make(map[string]Model),
			}

			result := engine.RouteTask("fallback-test", tt.taskType, tt.complexity)
			if result != tt.expectedFallback {
				t.Errorf("RouteTask() = %s, want %s. %s",
					result, tt.expectedFallback, tt.description)
			}
		})
	}
}

// TestDefaultModelConfiguration tests setting and getting default model.
func TestDefaultModelConfiguration(t *testing.T) {
	engine := NewRulesEngine()

	defaultModel := engine.DefaultModel
	if defaultModel != ModelSonnet {
		t.Errorf("Default model = %s, want ModelSonnet", defaultModel)
	}

	// Modify default model
	engine.DefaultModel = ModelHaiku
	result := engine.RouteTask("test-id", TaskTypeResearch, 1.0)
	// Research should use Gemini per rules, not default
	if result != ModelGemini {
		t.Errorf("Expected Gemini from rule, got %s", result)
	}
}

// ================================================================================
// TEST: COST TRACKING ACCURACY
// ================================================================================

// TestCostTrackingBasic tests basic cost recording and retrieval.
func TestCostTrackingBasic(t *testing.T) {
	metrics := NewRoutingMetrics()

	// Record some task executions
	metrics.RecordTaskExecution(TaskTypeCodeGeneration, 0.05, 1000)
	metrics.RecordTaskExecution(TaskTypeCodeGeneration, 0.06, 1200)
	metrics.RecordTaskExecution(TaskTypeTesting, 0.01, 500)

	// Check total cost
	if metrics.TotalCostUSD != 0.12 {
		t.Errorf("TotalCostUSD = %.4f, want 0.12", metrics.TotalCostUSD)
	}

	// Check total tasks
	if metrics.TotalTasksExecuted != 3 {
		t.Errorf("TotalTasksExecuted = %d, want 3", metrics.TotalTasksExecuted)
	}

	// Check task-specific metrics
	cgMetrics := metrics.GetMetrics(TaskTypeCodeGeneration)
	if cgMetrics == nil {
		t.Fatal("Expected code generation metrics")
	}
	if cgMetrics.Count != 2 {
		t.Errorf("CodeGeneration count = %d, want 2", cgMetrics.Count)
	}
	if cgMetrics.TotalCostUSD != 0.11 {
		t.Errorf("CodeGeneration cost = %.4f, want 0.11", cgMetrics.TotalCostUSD)
	}
}

// TestCostTrackingAverages tests average cost and duration calculations.
func TestCostTrackingAverages(t *testing.T) {
	metrics := NewRoutingMetrics()

	// Record tasks
	metrics.RecordTaskExecution(TaskTypeCodeGeneration, 0.10, 1000)
	metrics.RecordTaskExecution(TaskTypeCodeGeneration, 0.20, 2000)

	cgMetrics := metrics.GetMetrics(TaskTypeCodeGeneration)
	expectedAvgCost := 0.15
	expectedAvgDuration := int64(1500)

	if avgCost := cgMetrics.AverageCostUSD(); avgCost != expectedAvgCost {
		t.Errorf("AverageCostUSD() = %.4f, want %.4f", avgCost, expectedAvgCost)
	}

	if avgDuration := cgMetrics.AverageDurationMS(); avgDuration != expectedAvgDuration {
		t.Errorf("AverageDurationMS() = %d, want %d", avgDuration, expectedAvgDuration)
	}
}

// TestCostTrackingMinMax tests min/max cost and duration tracking.
func TestCostTrackingMinMax(t *testing.T) {
	metrics := NewRoutingMetrics()

	costs := []float64{0.05, 0.15, 0.10}
	durations := []int64{500, 2000, 1500}

	for i := range costs {
		metrics.RecordTaskExecution(TaskTypeCodeGeneration, costs[i], durations[i])
	}

	cgMetrics := metrics.GetMetrics(TaskTypeCodeGeneration)

	if cgMetrics.MinCostUSD != 0.05 {
		t.Errorf("MinCostUSD = %.4f, want 0.05", cgMetrics.MinCostUSD)
	}
	if cgMetrics.MaxCostUSD != 0.15 {
		t.Errorf("MaxCostUSD = %.4f, want 0.15", cgMetrics.MaxCostUSD)
	}
	if cgMetrics.MinDurationMS != 500 {
		t.Errorf("MinDurationMS = %d, want 500", cgMetrics.MinDurationMS)
	}
	if cgMetrics.MaxDurationMS != 2000 {
		t.Errorf("MaxDurationMS = %d, want 2000", cgMetrics.MaxDurationMS)
	}
}

// TestCostTrackingTimestamps tests timestamp recording.
func TestCostTrackingTimestamps(t *testing.T) {
	metrics := NewRoutingMetrics()
	startTime := time.Now()

	metrics.RecordTaskExecution(TaskTypeCodeGeneration, 0.05, 1000)
	time.Sleep(10 * time.Millisecond)
	metrics.RecordTaskExecution(TaskTypeCodeGeneration, 0.06, 1100)

	cgMetrics := metrics.GetMetrics(TaskTypeCodeGeneration)

	// Check FirstExecutedAt is around start time
	if cgMetrics.FirstExecutedAt.Before(startTime) || cgMetrics.FirstExecutedAt.After(time.Now()) {
		t.Error("FirstExecutedAt timestamp seems incorrect")
	}

	// Check LastExecutedAt is after FirstExecutedAt
	if !cgMetrics.LastExecutedAt.After(cgMetrics.FirstExecutedAt) {
		t.Error("LastExecutedAt should be after FirstExecutedAt")
	}
}

// ================================================================================
// TEST: METRICS AGGREGATION
// ================================================================================

// TestMetricsAggregation tests aggregating metrics across multiple task types.
func TestMetricsAggregation(t *testing.T) {
	metrics := NewRoutingMetrics()

	// Record tasks of different types
	metrics.RecordTaskExecution(TaskTypeCodeGeneration, 0.10, 1000)
	metrics.RecordTaskExecution(TaskTypeCodeGeneration, 0.15, 1500)
	metrics.RecordTaskExecution(TaskTypeTesting, 0.02, 500)
	metrics.RecordTaskExecution(TaskTypeTesting, 0.03, 600)
	metrics.RecordTaskExecution(TaskTypeDocumentation, 0.01, 300)

	// Verify total aggregation
	if metrics.TotalTasksExecuted != 5 {
		t.Errorf("TotalTasksExecuted = %d, want 5", metrics.TotalTasksExecuted)
	}

	expectedTotal := 0.10 + 0.15 + 0.02 + 0.03 + 0.01
	if metrics.TotalCostUSD != expectedTotal {
		t.Errorf("TotalCostUSD = %.4f, want %.4f", metrics.TotalCostUSD, expectedTotal)
	}

	// Verify per-type aggregation
	cgMetrics := metrics.GetMetrics(TaskTypeCodeGeneration)
	if cgMetrics.Count != 2 {
		t.Errorf("CodeGeneration count = %d, want 2", cgMetrics.Count)
	}

	testMetrics := metrics.GetMetrics(TaskTypeTesting)
	if testMetrics.Count != 2 {
		t.Errorf("Testing count = %d, want 2", testMetrics.Count)
	}

	docMetrics := metrics.GetMetrics(TaskTypeDocumentation)
	if docMetrics.Count != 1 {
		t.Errorf("Documentation count = %d, want 1", docMetrics.Count)
	}
}

// TestMetricsReportGeneration tests generating a metrics report.
func TestMetricsReportGeneration(t *testing.T) {
	metrics := NewRoutingMetrics()

	metrics.RecordTaskExecution(TaskTypeCodeGeneration, 0.10, 1000)
	metrics.RecordTaskExecution(TaskTypeTesting, 0.02, 500)

	report := metrics.GenerateReport()

	// Check that report contains expected content
	expectedStrings := []string{
		"ROUTING METRICS REPORT",
		"SUMMARY",
		"Total Tasks Executed",
		"Total Cost",
		"TASK TYPE BREAKDOWN",
		"code_generation",
		"testing",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(report, expected) {
			t.Errorf("Report missing expected string: %s", expected)
		}
	}
}

// TestMetricsConcurrentAccess tests thread-safe metric recording.
func TestMetricsConcurrentAccess(t *testing.T) {
	metrics := NewRoutingMetrics()
	numGoroutines := 100
	tasksPerGoroutine := 10

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < tasksPerGoroutine; j++ {
				taskType := AllTaskTypes()[id%len(AllTaskTypes())]
				metrics.RecordTaskExecution(taskType, 0.01, 100)
			}
		}(i)
	}

	wg.Wait()

	// Verify all tasks were recorded
	expectedTotal := numGoroutines * tasksPerGoroutine
	if metrics.TotalTasksExecuted != expectedTotal {
		t.Errorf("TotalTasksExecuted = %d, want %d", metrics.TotalTasksExecuted, expectedTotal)
	}
}

// TestMetricsClearAndReset tests clearing metrics.
func TestMetricsClearAndReset(t *testing.T) {
	metrics := NewRoutingMetrics()

	// Record some data
	metrics.RecordTaskExecution(TaskTypeCodeGeneration, 0.10, 1000)
	metrics.RecordTaskExecution(TaskTypeTesting, 0.02, 500)

	if metrics.TotalTasksExecuted != 2 {
		t.Errorf("Expected 2 tasks before clear, got %d", metrics.TotalTasksExecuted)
	}

	// Clear metrics
	metrics.ClearMetrics()

	if metrics.TotalTasksExecuted != 0 {
		t.Errorf("Expected 0 tasks after clear, got %d", metrics.TotalTasksExecuted)
	}
	if metrics.TotalCostUSD != 0.0 {
		t.Errorf("Expected 0 cost after clear, got %.4f", metrics.TotalCostUSD)
	}
	if len(metrics.ByTaskType) != 0 {
		t.Errorf("Expected empty metrics map after clear, got %d entries", len(metrics.ByTaskType))
	}
}

// TestMetricsInvalidTaskType tests handling of invalid task types.
func TestMetricsInvalidTaskType(t *testing.T) {
	metrics := NewRoutingMetrics()

	// Record with invalid task type should be silently ignored
	metrics.RecordTaskExecution(TaskType("invalid"), 0.10, 1000)

	// Verify it wasn't recorded
	if metrics.TotalTasksExecuted != 0 {
		t.Errorf("Invalid task type should be ignored, got count %d", metrics.TotalTasksExecuted)
	}
}

// TestMetricsZeroValues tests handling of zero cost/duration.
func TestMetricsZeroValues(t *testing.T) {
	metrics := NewRoutingMetrics()

	// Record with zero cost and duration
	metrics.RecordTaskExecution(TaskTypeCodeGeneration, 0.0, 0)

	if metrics.TotalTasksExecuted != 1 {
		t.Errorf("Should record task with zero values, got count %d", metrics.TotalTasksExecuted)
	}
	if metrics.TotalCostUSD != 0.0 {
		t.Errorf("Cost should be 0.0, got %.4f", metrics.TotalCostUSD)
	}
}

// ================================================================================
// TEST: ROUTING DECISION INTEGRATION
// ================================================================================

// TestFullClassificationRouting tests routing with complete classification.
func TestFullClassificationRouting(t *testing.T) {
	engine := NewRulesEngine()

	tests := []struct {
		name           string
		classification *TaskClassification
		expectedModel  Model
		description    string
	}{
		{
			name: "Simple documentation",
			classification: &TaskClassification{
				TaskType: TaskTypeDocumentation,
				Complexity: ComplexityScore{
					TechnicalComplexity: 1,
					ContextRequirements: 1,
					RiskLevel:           1,
					Reversibility:       5,
				},
				Confidence: 0.95,
			},
			expectedModel: ModelHaiku,
			description:   "Simple documentation should route to Haiku",
		},
		{
			name: "Complex security audit",
			classification: &TaskClassification{
				TaskType: TaskTypeSecurity,
				Complexity: ComplexityScore{
					TechnicalComplexity: 5,
					ContextRequirements: 5,
					RiskLevel:           5,
					Reversibility:       5,
				},
				Confidence: 0.95,
			},
			expectedModel: ModelOpus,
			description:   "Security should always route to Opus",
		},
		{
			name: "Moderate code implementation",
			classification: &TaskClassification{
				TaskType: TaskTypeCodeGeneration,
				Complexity: ComplexityScore{
					TechnicalComplexity: 3,
					ContextRequirements: 3,
					RiskLevel:           2,
					Reversibility:       3,
				},
				Confidence: 0.85,
			},
			expectedModel: ModelSonnet,
			description:   "Moderate implementation should route to Sonnet",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := engine.RouteTaskWithFullClassification("test-id", tt.classification)
			if result != tt.expectedModel {
				t.Errorf("RouteTaskWithFullClassification() = %s, want %s. %s",
					result, tt.expectedModel, tt.description)
			}
		})
	}
}

// TestPrioritySorting tests that rules are properly prioritized.
func TestPrioritySorting(t *testing.T) {
	engine := NewRulesEngine()

	// Verify rules are sorted by priority (descending)
	for i := 1; i < len(engine.Rules); i++ {
		if engine.Rules[i].Priority > engine.Rules[i-1].Priority {
			t.Errorf("Rules not sorted by priority: rule %d has priority %d > rule %d priority %d",
				i, engine.Rules[i].Priority, i-1, engine.Rules[i-1].Priority)
		}
	}
}

// ================================================================================
// TABLE-DRIVEN TESTS: COMPREHENSIVE ROUTING SCENARIOS
// ================================================================================

// TestAllRoutingPaths tests all possible routing paths comprehensively.
func TestAllRoutingPaths(t *testing.T) {
	engine := NewRulesEngine()

	// Verify every task type has routes for all complexity levels
	for _, taskType := range AllTaskTypes() {
		for complexity := 1; complexity <= 5; complexity++ {
			model := engine.RouteTask("test-id", taskType, float64(complexity))
			if model == "" || !model.IsValid() {
				t.Errorf("RouteTask(%s, %d) returned invalid model: %s",
					taskType, complexity, model)
			}
		}
	}
}

// TestComplexityEdgeCases tests edge cases in complexity scoring.
func TestComplexityEdgeCases(t *testing.T) {
	engine := NewRulesEngine()

	tests := []struct {
		name        string
		complexity  float64
		shouldRoute bool
		description string
	}{
		{"Zero complexity", 0.0, true, "Should handle zero"},
		{"Negative complexity", -5.0, true, "Should handle negative"},
		{"Very high complexity", 1000.0, true, "Should handle very high"},
		{"Exact decimal", 3.5, true, "Should handle decimals"},
		{"Near integer", 3.0001, true, "Should handle near-integer"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := engine.RouteTask("test-id", TaskTypeCodeGeneration, tt.complexity)
			if tt.shouldRoute && result == "" {
				t.Errorf("RouteTask should return a model for complexity %.4f", tt.complexity)
			}
		})
	}
}

// ================================================================================
// BENCHMARKS
// ================================================================================

// BenchmarkDefaultMatrixLookup benchmarks matrix lookup performance.
func BenchmarkDefaultMatrixLookup(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		LookupRoute(TaskTypeCodeGeneration, 3)
	}
}

// BenchmarkRulesEngineRouting benchmarks rules engine routing.
func BenchmarkRulesEngineRouting(b *testing.B) {
	engine := NewRulesEngine()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.RouteTask("bench-task", TaskTypeCodeGeneration, 3.0)
	}
}

// BenchmarkMetricsRecording benchmarks metrics recording.
func BenchmarkMetricsRecording(b *testing.B) {
	metrics := NewRoutingMetrics()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		metrics.RecordTaskExecution(TaskTypeCodeGeneration, 0.05, 1000)
	}
}

// BenchmarkConcurrentMetricsRecording benchmarks concurrent metrics recording.
func BenchmarkConcurrentMetricsRecording(b *testing.B) {
	metrics := NewRoutingMetrics()
	var counter atomic.Int64

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			idx := counter.Add(1) % int64(len(AllTaskTypes()))
			taskType := AllTaskTypes()[idx]
			metrics.RecordTaskExecution(taskType, 0.05, 1000)
		}
	})
}

// ================================================================================
// PROPERTY-BASED TESTS
// ================================================================================

// TestRoutingConsistencyProperty tests that routing is deterministic.
func TestRoutingConsistencyProperty(t *testing.T) {
	engine := NewRulesEngine()

	// For any task type and complexity, routing should be consistent
	for _, taskType := range AllTaskTypes() {
		for complexity := 1; complexity <= 5; complexity++ {
			first := engine.RouteTask("prop-test", taskType, float64(complexity))
			second := engine.RouteTask("prop-test", taskType, float64(complexity))
			third := engine.RouteTask("prop-test", taskType, float64(complexity))

			if first != second || second != third {
				t.Errorf("Routing not consistent for %s:%d: %s vs %s vs %s",
					taskType, complexity, first, second, third)
			}
		}
	}
}

// TestCostTrackingMonotonicity tests that cost tracking increases monotonically.
func TestCostTrackingMonotonicity(t *testing.T) {
	metrics := NewRoutingMetrics()

	previousTotal := 0.0
	for i := 0; i < 10; i++ {
		metrics.RecordTaskExecution(TaskTypeCodeGeneration, 0.01, 100)
		if metrics.TotalCostUSD < previousTotal {
			t.Errorf("Cost decreased: %.4f < %.4f", metrics.TotalCostUSD, previousTotal)
		}
		previousTotal = metrics.TotalCostUSD
	}
}
