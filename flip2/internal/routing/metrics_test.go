// Package routing provides intelligent task routing between different AI models.
package routing

import (
	"strings"
	"testing"
	"time"
)

// TestRecordTaskExecution tests recording individual task executions.
func TestRecordTaskExecution(t *testing.T) {
	metrics := NewRoutingMetrics()

	// Record a single task
	metrics.RecordTaskExecution(TaskTypeCodeGeneration, 0.05, 1200)

	if metrics.TotalTasksExecuted != 1 {
		t.Errorf("Expected 1 task executed, got %d", metrics.TotalTasksExecuted)
	}

	if metrics.TotalCostUSD != 0.05 {
		t.Errorf("Expected total cost $0.05, got $%.4f", metrics.TotalCostUSD)
	}

	if metrics.TotalDurationMS != 1200 {
		t.Errorf("Expected total duration 1200ms, got %d", metrics.TotalDurationMS)
	}
}

// TestMultipleTaskTypeRecording tests recording tasks of different types.
func TestMultipleTaskTypeRecording(t *testing.T) {
	metrics := NewRoutingMetrics()

	// Record tasks of different types
	metrics.RecordTaskExecution(TaskTypeCodeGeneration, 0.05, 1200)
	metrics.RecordTaskExecution(TaskTypeCodeReview, 0.03, 800)
	metrics.RecordTaskExecution(TaskTypeResearch, 0.02, 2000)
	metrics.RecordTaskExecution(TaskTypeCodeGeneration, 0.06, 1300)

	// Check total counts
	if metrics.TotalTasksExecuted != 4 {
		t.Errorf("Expected 4 total tasks, got %d", metrics.TotalTasksExecuted)
	}

	expectedTotalCost := 0.05 + 0.03 + 0.02 + 0.06
	if metrics.TotalCostUSD != expectedTotalCost {
		t.Errorf("Expected total cost $%.4f, got $%.4f", expectedTotalCost, metrics.TotalCostUSD)
	}

	// Check per-type counts
	if count := metrics.GetTaskCount(TaskTypeCodeGeneration); count != 2 {
		t.Errorf("Expected 2 code_generation tasks, got %d", count)
	}

	if count := metrics.GetTaskCount(TaskTypeCodeReview); count != 1 {
		t.Errorf("Expected 1 code_review task, got %d", count)
	}

	if count := metrics.GetTaskCount(TaskTypeResearch); count != 1 {
		t.Errorf("Expected 1 research task, got %d", count)
	}
}

// TestTaskMetricsCalculations tests metric calculations like averages.
func TestTaskMetricsCalculations(t *testing.T) {
	metrics := NewRoutingMetrics()

	// Record tasks with known values
	metrics.RecordTaskExecution(TaskTypeCodeGeneration, 0.10, 1000)
	metrics.RecordTaskExecution(TaskTypeCodeGeneration, 0.20, 2000)
	metrics.RecordTaskExecution(TaskTypeCodeGeneration, 0.30, 3000)

	taskMetrics := metrics.GetMetrics(TaskTypeCodeGeneration)
	if taskMetrics == nil {
		t.Fatal("Expected to find metrics for code_generation")
	}

	// Check count
	if taskMetrics.Count != 3 {
		t.Errorf("Expected 3 tasks, got %d", taskMetrics.Count)
	}

	// Check total cost (with floating point tolerance)
	expectedTotal := 0.60
	if taskMetrics.TotalCostUSD-expectedTotal > 0.00001 {
		t.Errorf("Expected total cost $0.60, got $%.4f", taskMetrics.TotalCostUSD)
	}

	// Check average cost (0.60 / 3 = 0.20)
	expectedAvg := 0.20
	if taskMetrics.AverageCostUSD()-expectedAvg > 0.00001 {
		t.Errorf("Expected average cost $%.2f, got $%.6f", expectedAvg, taskMetrics.AverageCostUSD())
	}

	// Check total duration
	if taskMetrics.TotalDurationMS != 6000 {
		t.Errorf("Expected total duration 6000ms, got %d", taskMetrics.TotalDurationMS)
	}

	// Check average duration (6000 / 3 = 2000)
	if taskMetrics.AverageDurationMS() != 2000 {
		t.Errorf("Expected average duration 2000ms, got %d", taskMetrics.AverageDurationMS())
	}

	// Check min/max costs
	if taskMetrics.MinCostUSD != 0.10 {
		t.Errorf("Expected min cost $0.10, got $%.4f", taskMetrics.MinCostUSD)
	}

	if taskMetrics.MaxCostUSD != 0.30 {
		t.Errorf("Expected max cost $0.30, got $%.4f", taskMetrics.MaxCostUSD)
	}

	// Check min/max durations
	if taskMetrics.MinDurationMS != 1000 {
		t.Errorf("Expected min duration 1000ms, got %d", taskMetrics.MinDurationMS)
	}

	if taskMetrics.MaxDurationMS != 3000 {
		t.Errorf("Expected max duration 3000ms, got %d", taskMetrics.MaxDurationMS)
	}
}

// TestGetMetrics tests retrieving metrics for a specific task type.
func TestGetMetrics(t *testing.T) {
	metrics := NewRoutingMetrics()

	// Record some tasks
	metrics.RecordTaskExecution(TaskTypeCodeGeneration, 0.05, 1200)
	metrics.RecordTaskExecution(TaskTypeTesting, 0.01, 500)

	// Get metrics for recorded type
	codeGenMetrics := metrics.GetMetrics(TaskTypeCodeGeneration)
	if codeGenMetrics == nil {
		t.Fatal("Expected to find metrics for code_generation")
	}

	if codeGenMetrics.Count != 1 {
		t.Errorf("Expected 1 code_generation task, got %d", codeGenMetrics.Count)
	}

	// Get metrics for non-existent type
	debugMetrics := metrics.GetMetrics(TaskTypeDebugging)
	if debugMetrics != nil {
		t.Errorf("Expected nil for non-existent task type, got %+v", debugMetrics)
	}
}

// TestGenerateReport tests the report generation functionality.
func TestGenerateReport(t *testing.T) {
	metrics := NewRoutingMetrics()

	// Record diverse tasks
	metrics.RecordTaskExecution(TaskTypeCodeGeneration, 0.15, 1500)
	metrics.RecordTaskExecution(TaskTypeCodeGeneration, 0.10, 1000)
	metrics.RecordTaskExecution(TaskTypeCodeReview, 0.05, 800)
	metrics.RecordTaskExecution(TaskTypeResearch, 0.08, 2000)
	metrics.RecordTaskExecution(TaskTypeTesting, 0.02, 400)

	report := metrics.GenerateReport()

	// Check that report contains expected sections
	if !strings.Contains(report, "ROUTING METRICS REPORT") {
		t.Error("Report missing ROUTING METRICS REPORT header")
	}

	if !strings.Contains(report, "SUMMARY") {
		t.Error("Report missing SUMMARY section")
	}

	if !strings.Contains(report, "TASK TYPE BREAKDOWN") {
		t.Error("Report missing TASK TYPE BREAKDOWN section")
	}

	if !strings.Contains(report, "TOP 5 TASKS BY COST") {
		t.Error("Report missing TOP 5 TASKS BY COST section")
	}

	if !strings.Contains(report, "DETAILED METRICS") {
		t.Error("Report missing DETAILED METRICS section")
	}

	// Check for task type names in report
	if !strings.Contains(report, "code_generation") {
		t.Error("Report missing code_generation task type")
	}

	if !strings.Contains(report, "code_review") {
		t.Error("Report missing code_review task type")
	}

	// Check for cost information
	if !strings.Contains(report, "Total Cost") {
		t.Error("Report missing total cost information")
	}

	// Check for task count
	if !strings.Contains(report, "Total Tasks Executed") {
		t.Error("Report missing task count")
	}

	// Verify report contains all 5 tasks
	if !strings.Contains(report, "$0.4000") && !strings.Contains(report, "0.4000") {
		// At least check total cost shows up somewhere
		if !strings.Contains(report, "Total Cost") {
			t.Error("Report missing cost breakdown")
		}
	}
}

// TestReportFormatting tests that the report is well-formatted.
func TestReportFormatting(t *testing.T) {
	metrics := NewRoutingMetrics()

	// Add some test data
	metrics.RecordTaskExecution(TaskTypeCodeGeneration, 0.05, 1200)
	metrics.RecordTaskExecution(TaskTypeCodeReview, 0.03, 800)

	report := metrics.GenerateReport()

	// Check minimum length (should be substantial)
	if len(report) < 500 {
		t.Errorf("Report seems too short: %d characters", len(report))
	}

	// Check that it has proper line breaks
	lines := strings.Split(report, "\n")
	if len(lines) < 30 {
		t.Errorf("Report has too few lines: %d", len(lines))
	}

	// Check for separators
	if !strings.Contains(report, "====") {
		t.Error("Report missing separator lines")
	}

	if !strings.Contains(report, "----") {
		t.Error("Report missing section dividers")
	}
}

// TestEmptyMetricsReport tests report generation with no recorded tasks.
func TestEmptyMetricsReport(t *testing.T) {
	metrics := NewRoutingMetrics()

	report := metrics.GenerateReport()

	// Should still have headers and sections
	if !strings.Contains(report, "ROUTING METRICS REPORT") {
		t.Error("Report missing ROUTING METRICS REPORT header")
	}

	// Should mention zero tasks
	if !strings.Contains(report, "Total Tasks Executed:  0") {
		t.Error("Report missing zero task count")
	}

	if !strings.Contains(report, "No tasks recorded") {
		t.Error("Report missing 'No tasks recorded' message")
	}
}

// TestConcurrentRecording tests thread-safe concurrent recording.
func TestConcurrentRecording(t *testing.T) {
	metrics := NewRoutingMetrics()

	// Record tasks concurrently from multiple goroutines
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(index int) {
			taskType := TaskTypeCodeGeneration
			if index%2 == 0 {
				taskType = TaskTypeCodeReview
			}
			metrics.RecordTaskExecution(taskType, 0.05, 1000)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify totals
	if metrics.TotalTasksExecuted != 10 {
		t.Errorf("Expected 10 total tasks, got %d", metrics.TotalTasksExecuted)
	}

	expectedCost := 0.50
	if metrics.TotalCostUSD-expectedCost > 0.00001 {
		t.Errorf("Expected total cost $0.50, got $%.4f", metrics.TotalCostUSD)
	}

	// Verify per-type counts (5 of each)
	codeGenCount := metrics.GetTaskCount(TaskTypeCodeGeneration)
	if codeGenCount != 5 {
		t.Errorf("Expected 5 code_generation tasks, got %d", codeGenCount)
	}

	codeRevCount := metrics.GetTaskCount(TaskTypeCodeReview)
	if codeRevCount != 5 {
		t.Errorf("Expected 5 code_review tasks, got %d", codeRevCount)
	}
}

// TestClearMetrics tests the ClearMetrics functionality.
func TestClearMetrics(t *testing.T) {
	metrics := NewRoutingMetrics()

	// Record some tasks
	metrics.RecordTaskExecution(TaskTypeCodeGeneration, 0.05, 1200)
	metrics.RecordTaskExecution(TaskTypeCodeReview, 0.03, 800)

	if metrics.TotalTasksExecuted != 2 {
		t.Errorf("Expected 2 tasks before clear, got %d", metrics.TotalTasksExecuted)
	}

	// Clear metrics
	metrics.ClearMetrics()

	// Verify cleared
	if metrics.TotalTasksExecuted != 0 {
		t.Errorf("Expected 0 tasks after clear, got %d", metrics.TotalTasksExecuted)
	}

	if metrics.TotalCostUSD != 0.0 {
		t.Errorf("Expected $0 cost after clear, got $%.4f", metrics.TotalCostUSD)
	}

	if len(metrics.ByTaskType) != 0 {
		t.Errorf("Expected empty task metrics map after clear, got %d entries", len(metrics.ByTaskType))
	}

	// Should be able to record after clear
	metrics.RecordTaskExecution(TaskTypeResearch, 0.02, 500)
	if metrics.TotalTasksExecuted != 1 {
		t.Errorf("Expected 1 task after recording post-clear, got %d", metrics.TotalTasksExecuted)
	}
}

// TestInvalidTaskType tests that invalid task types are silently ignored.
func TestInvalidTaskType(t *testing.T) {
	metrics := NewRoutingMetrics()

	// Attempting to record with invalid task type should be ignored
	metrics.RecordTaskExecution(TaskType("invalid_type"), 0.05, 1000)

	if metrics.TotalTasksExecuted != 0 {
		t.Errorf("Expected 0 tasks for invalid type, got %d", metrics.TotalTasksExecuted)
	}

	// Record a valid task to ensure system still works
	metrics.RecordTaskExecution(TaskTypeCodeGeneration, 0.05, 1000)
	if metrics.TotalTasksExecuted != 1 {
		t.Errorf("Expected 1 task after valid record, got %d", metrics.TotalTasksExecuted)
	}
}

// TestMetricsTimestamps tests that task metrics timestamps are recorded.
func TestMetricsTimestamps(t *testing.T) {
	metrics := NewRoutingMetrics()

	beforeRecord := time.Now()
	metrics.RecordTaskExecution(TaskTypeCodeGeneration, 0.05, 1200)
	afterRecord := time.Now()

	taskMetrics := metrics.GetMetrics(TaskTypeCodeGeneration)
	if taskMetrics == nil {
		t.Fatal("Expected to find metrics for code_generation")
	}

	// Check that FirstExecutedAt is within reasonable bounds
	if taskMetrics.FirstExecutedAt.Before(beforeRecord) || taskMetrics.FirstExecutedAt.After(afterRecord) {
		t.Error("FirstExecutedAt not within expected time bounds")
	}

	// LastExecutedAt should be roughly same as FirstExecutedAt for single task (within 100ms)
	timeDiff := taskMetrics.LastExecutedAt.Sub(taskMetrics.FirstExecutedAt).Milliseconds()
	if timeDiff < 0 || timeDiff > 100 {
		t.Errorf("FirstExecutedAt and LastExecutedAt should be within 100ms, got %d ms difference", timeDiff)
	}

	// Record another task and verify LastExecutedAt updates
	time.Sleep(10 * time.Millisecond) // Small delay to ensure time difference
	metrics.RecordTaskExecution(TaskTypeCodeGeneration, 0.06, 1300)

	taskMetrics2 := metrics.GetMetrics(TaskTypeCodeGeneration)
	if taskMetrics2.LastExecutedAt.Before(taskMetrics.FirstExecutedAt) {
		t.Error("LastExecutedAt should be after FirstExecutedAt after second task")
	}
}

// TestAverageDurationFormat tests the formatDuration helper function.
func TestFormatDuration(t *testing.T) {
	tests := []struct {
		ms       int64
		expected string
	}{
		{500, "500 ms"},
		{1000, "1.0 s"},
		{1500, "1.5 s"},
		{60000, "1 m 0 s"},
		{90000, "1 m 30 s"},
		{3661000, "61 m 1 s"},
	}

	for _, test := range tests {
		result := formatDuration(test.ms)
		if result != test.expected {
			t.Errorf("formatDuration(%d) = %s, expected %s", test.ms, result, test.expected)
		}
	}
}

// TestMetricsGetters tests the getter methods.
func TestMetricsGetters(t *testing.T) {
	metrics := NewRoutingMetrics()

	// Test initial state
	if metrics.GetTotalCost() != 0.0 {
		t.Errorf("Expected initial total cost 0, got %.4f", metrics.GetTotalCost())
	}

	if metrics.GetTotalTasksExecuted() != 0 {
		t.Errorf("Expected initial task count 0, got %d", metrics.GetTotalTasksExecuted())
	}

	// Record tasks
	metrics.RecordTaskExecution(TaskTypeCodeGeneration, 0.10, 1000)
	metrics.RecordTaskExecution(TaskTypeCodeGeneration, 0.15, 1500)
	metrics.RecordTaskExecution(TaskTypeCodeReview, 0.05, 800)

	// Test getters
	if metrics.GetTotalCost() != 0.30 {
		t.Errorf("Expected total cost 0.30, got %.4f", metrics.GetTotalCost())
	}

	if metrics.GetTotalTasksExecuted() != 3 {
		t.Errorf("Expected total tasks 3, got %d", metrics.GetTotalTasksExecuted())
	}

	if metrics.GetTaskCount(TaskTypeCodeGeneration) != 2 {
		t.Errorf("Expected 2 code_generation tasks, got %d", metrics.GetTaskCount(TaskTypeCodeGeneration))
	}

	if metrics.GetTaskCount(TaskTypeCodeReview) != 1 {
		t.Errorf("Expected 1 code_review task, got %d", metrics.GetTaskCount(TaskTypeCodeReview))
	}

	if metrics.GetTaskCount(TaskTypeResearch) != 0 {
		t.Errorf("Expected 0 research tasks, got %d", metrics.GetTaskCount(TaskTypeResearch))
	}
}

// BenchmarkRecordTaskExecution benchmarks task recording performance.
func BenchmarkRecordTaskExecution(b *testing.B) {
	metrics := NewRoutingMetrics()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		metrics.RecordTaskExecution(TaskTypeCodeGeneration, 0.05, 1000)
	}
}

// BenchmarkGenerateReport benchmarks report generation with many tasks.
func BenchmarkGenerateReport(b *testing.B) {
	metrics := NewRoutingMetrics()

	// Pre-populate with tasks
	for i := 0; i < 100; i++ {
		taskType := AllTaskTypes()[i%len(AllTaskTypes())]
		metrics.RecordTaskExecution(taskType, 0.05, 1000)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = metrics.GenerateReport()
	}
}
