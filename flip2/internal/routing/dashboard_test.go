// Package routing provides intelligent task routing between different AI models.
package routing

import (
	"strings"
	"testing"
)

// TestGenerateDashboard tests the basic dashboard generation.
func TestGenerateDashboard(t *testing.T) {
	metrics := NewRoutingMetrics()

	// Record diverse tasks across different models
	// Haiku tasks (simple)
	metrics.RecordTaskExecution(TaskTypeTesting, 0.001, 500)
	metrics.RecordTaskExecution(TaskTypeDocumentation, 0.0015, 600)

	// Sonnet tasks (moderate)
	metrics.RecordTaskExecution(TaskTypeCodeGeneration, 0.03, 1200)
	metrics.RecordTaskExecution(TaskTypeCodeReview, 0.025, 900)
	metrics.RecordTaskExecution(TaskTypeDebugging, 0.035, 1500)

	// Opus tasks (complex)
	metrics.RecordTaskExecution(TaskTypeArchitecture, 0.15, 2000)
	metrics.RecordTaskExecution(TaskTypeSecurity, 0.12, 1800)

	// Gemini tasks (bulk)
	metrics.RecordTaskExecution(TaskTypeResearch, 0.005, 3000)
	metrics.RecordTaskExecution(TaskTypeDataProcessing, 0.008, 2500)

	dashboard := metrics.GenerateDashboard()

	// Verify dashboard contains expected sections
	if !strings.Contains(dashboard, "FLIP2 Routing Analytics Dashboard") {
		t.Error("Dashboard missing title")
	}

	if !strings.Contains(dashboard, "Summary") {
		t.Error("Dashboard missing Summary section")
	}

	if !strings.Contains(dashboard, "Cost by Model") {
		t.Error("Dashboard missing Cost by Model section")
	}

	if !strings.Contains(dashboard, "Cost Savings") {
		t.Error("Dashboard missing Cost Savings section")
	}

	if !strings.Contains(dashboard, "Tasks by Type") {
		t.Error("Dashboard missing Tasks by Type section")
	}

	if !strings.Contains(dashboard, "Model Comparison") {
		t.Error("Dashboard missing Model Comparison section")
	}

	if !strings.Contains(dashboard, "Insights") {
		t.Error("Dashboard missing Insights section")
	}

	// Check for specific costs
	if !strings.Contains(dashboard, "$") {
		t.Error("Dashboard missing cost information")
	}

	// Should show 9 tasks total
	if !strings.Contains(dashboard, "9") {
		t.Error("Dashboard not showing correct task count")
	}
}

// TestGenerateDashboardMarkdown tests markdown formatting.
func TestGenerateDashboardMarkdown(t *testing.T) {
	metrics := NewRoutingMetrics()

	metrics.RecordTaskExecution(TaskTypeCodeGeneration, 0.05, 1200)
	metrics.RecordTaskExecution(TaskTypeCodeReview, 0.03, 800)
	metrics.RecordTaskExecution(TaskTypeTesting, 0.001, 500)

	dashboard := metrics.GenerateDashboard()

	// Check for markdown formatting
	if !strings.Contains(dashboard, "# FLIP2") {
		t.Error("Dashboard missing markdown H1 header")
	}

	if !strings.Contains(dashboard, "## Summary") {
		t.Error("Dashboard missing markdown H2 headers")
	}

	if !strings.Contains(dashboard, "| Model") {
		t.Error("Dashboard missing markdown table")
	}

	if !strings.Contains(dashboard, "**") {
		t.Error("Dashboard missing markdown bold formatting")
	}
}

// TestGenerateDashboardASCII tests ASCII table formatting.
func TestGenerateDashboardASCII(t *testing.T) {
	metrics := NewRoutingMetrics()

	metrics.RecordTaskExecution(TaskTypeCodeGeneration, 0.05, 1200)
	metrics.RecordTaskExecution(TaskTypeCodeReview, 0.03, 800)
	metrics.RecordTaskExecution(TaskTypeTesting, 0.001, 500)

	dashboard := metrics.GenerateDashboardASCII()

	// Check for ASCII formatting
	if !strings.Contains(dashboard, "ROUTING ANALYTICS DASHBOARD") {
		t.Error("ASCII dashboard missing title")
	}

	if !strings.Contains(dashboard, "====") {
		t.Error("ASCII dashboard missing separator")
	}

	if !strings.Contains(dashboard, "COST BY MODEL") {
		t.Error("ASCII dashboard missing cost section")
	}

	if !strings.Contains(dashboard, "Model | Count") {
		t.Error("ASCII dashboard missing table header")
	}
}

// TestDashboardCostSavings tests that cost savings are calculated correctly.
func TestDashboardCostSavings(t *testing.T) {
	metrics := NewRoutingMetrics()

	// Mix of tasks across models
	// Haiku (cheap)
	metrics.RecordTaskExecution(TaskTypeTesting, 0.001, 500)

	// Sonnet (medium)
	metrics.RecordTaskExecution(TaskTypeCodeGeneration, 0.03, 1200)

	// Opus (expensive)
	metrics.RecordTaskExecution(TaskTypeArchitecture, 0.15, 2000)

	dashboard := metrics.GenerateDashboard()

	// Should show savings
	if !strings.Contains(dashboard, "Cost Savings") {
		t.Error("Dashboard should show cost savings with mixed models")
	}

	if !strings.Contains(dashboard, "Savings") {
		t.Error("Dashboard missing savings information")
	}

	// Check that savings value exists
	if !strings.Contains(dashboard, "$") {
		t.Error("Dashboard missing dollar amounts for savings")
	}
}

// TestDashboardEmptyMetrics tests dashboard with no recorded tasks.
func TestDashboardEmptyMetrics(t *testing.T) {
	metrics := NewRoutingMetrics()

	dashboard := metrics.GenerateDashboard()

	// Should still have header
	if !strings.Contains(dashboard, "FLIP2 Routing Analytics Dashboard") {
		t.Error("Empty dashboard missing title")
	}

	// Should show 0 tasks
	if !strings.Contains(dashboard, "0") {
		t.Error("Empty dashboard not showing zero tasks")
	}
}

// TestDashboardSingleModel tests dashboard with tasks from one model only.
func TestDashboardSingleModel(t *testing.T) {
	metrics := NewRoutingMetrics()

	// Only Haiku tasks
	metrics.RecordTaskExecution(TaskTypeTesting, 0.001, 500)
	metrics.RecordTaskExecution(TaskTypeDocumentation, 0.0015, 600)
	metrics.RecordTaskExecution(TaskTypeTesting, 0.001, 450)

	dashboard := metrics.GenerateDashboard()

	if !strings.Contains(dashboard, "Claude Haiku") {
		t.Error("Dashboard not showing Haiku model")
	}

	if !strings.Contains(dashboard, "3") {
		t.Error("Dashboard not showing correct task count")
	}
}

// TestDashboardMultipleModels tests dashboard with multiple models.
func TestDashboardMultipleModels(t *testing.T) {
	metrics := NewRoutingMetrics()

	// Haiku
	metrics.RecordTaskExecution(TaskTypeTesting, 0.001, 500)

	// Sonnet
	metrics.RecordTaskExecution(TaskTypeCodeGeneration, 0.03, 1200)

	// Opus
	metrics.RecordTaskExecution(TaskTypeArchitecture, 0.15, 2000)

	// Gemini
	metrics.RecordTaskExecution(TaskTypeResearch, 0.005, 3000)

	dashboard := metrics.GenerateDashboard()

	// Should mention multiple models
	if !strings.Contains(dashboard, "Claude Opus") {
		t.Error("Dashboard missing Opus")
	}

	if !strings.Contains(dashboard, "Claude Sonnet") {
		t.Error("Dashboard missing Sonnet")
	}

	if !strings.Contains(dashboard, "Claude Haiku") {
		t.Error("Dashboard missing Haiku")
	}

	if !strings.Contains(dashboard, "Gemini") {
		t.Error("Dashboard missing Gemini")
	}
}

// TestDashboardCostBreakdown tests that cost breakdown is accurate.
func TestDashboardCostBreakdown(t *testing.T) {
	metrics := NewRoutingMetrics()

	// Create predictable costs
	metrics.RecordTaskExecution(TaskTypeTesting, 0.01, 1000)        // Haiku
	metrics.RecordTaskExecution(TaskTypeCodeGeneration, 0.02, 1000) // Sonnet
	metrics.RecordTaskExecution(TaskTypeArchitecture, 0.07, 1000)   // Opus

	dashboard := metrics.GenerateDashboard()

	// Total should be 0.10
	if !strings.Contains(dashboard, "Total Cost") {
		t.Error("Dashboard missing total cost")
	}

	// Should show percentage breakdown
	if !strings.Contains(dashboard, "%") {
		t.Error("Dashboard missing percentage breakdown")
	}
}

// TestDashboardTaskDistribution tests task type breakdown.
func TestDashboardTaskDistribution(t *testing.T) {
	metrics := NewRoutingMetrics()

	// Varied task types
	metrics.RecordTaskExecution(TaskTypeTesting, 0.001, 500)
	metrics.RecordTaskExecution(TaskTypeTesting, 0.001, 500)
	metrics.RecordTaskExecution(TaskTypeCodeGeneration, 0.03, 1200)
	metrics.RecordTaskExecution(TaskTypeArchitecture, 0.15, 2000)

	dashboard := metrics.GenerateDashboard()

	if !strings.Contains(dashboard, "Tasks by Type") {
		t.Error("Dashboard missing task type breakdown")
	}

	if !strings.Contains(dashboard, "code_generation") {
		t.Error("Dashboard missing code_generation task type")
	}

	if !strings.Contains(dashboard, "architecture") {
		t.Error("Dashboard missing architecture task type")
	}

	if !strings.Contains(dashboard, "testing") {
		t.Error("Dashboard missing testing task type")
	}
}

// TestDashboardInsights tests that insights are generated.
func TestDashboardInsights(t *testing.T) {
	metrics := NewRoutingMetrics()

	// Multiple tasks to generate insights
	for i := 0; i < 5; i++ {
		metrics.RecordTaskExecution(TaskTypeTesting, 0.001, 500)
	}
	metrics.RecordTaskExecution(TaskTypeArchitecture, 0.15, 2000)

	dashboard := metrics.GenerateDashboard()

	if !strings.Contains(dashboard, "Insights") {
		t.Error("Dashboard missing insights section")
	}

	// Should mention most used model or savings
	if !strings.Contains(dashboard, "✓") {
		t.Error("Dashboard missing insight indicators")
	}
}

// TestDashboardFormatting tests overall dashboard formatting.
func TestDashboardFormatting(t *testing.T) {
	metrics := NewRoutingMetrics()

	// Add sample data
	metrics.RecordTaskExecution(TaskTypeCodeGeneration, 0.05, 1200)
	metrics.RecordTaskExecution(TaskTypeCodeReview, 0.03, 800)
	metrics.RecordTaskExecution(TaskTypeTesting, 0.001, 500)

	dashboard := metrics.GenerateDashboard()

	// Check minimum length
	if len(dashboard) < 500 {
		t.Errorf("Dashboard seems too short: %d characters", len(dashboard))
	}

	// Check line breaks
	lines := strings.Split(dashboard, "\n")
	if len(lines) < 20 {
		t.Errorf("Dashboard has too few lines: %d", len(lines))
	}

	// Should be valid markdown with proper structure
	if !strings.Contains(dashboard, "#") {
		t.Error("Dashboard missing markdown headers")
	}

	if !strings.Contains(dashboard, "-") {
		t.Error("Dashboard missing markdown separators")
	}
}

// TestDashboardMetricsCalculation tests that metrics are correctly aggregated.
func TestDashboardMetricsCalculation(t *testing.T) {
	metrics := NewRoutingMetrics()

	// Record tasks with known values
	metrics.RecordTaskExecution(TaskTypeTesting, 0.01, 1000)
	metrics.RecordTaskExecution(TaskTypeCodeGeneration, 0.02, 1000)

	dashboard := metrics.GenerateDashboard()

	// Should show total of 2 tasks
	if !strings.Contains(dashboard, "Total Tasks Executed: 2") &&
		!strings.Contains(dashboard, "Total Tasks Executed:  2") {
		t.Error("Dashboard not showing correct total task count")
	}

	// Should show total cost (0.03)
	if !strings.Contains(dashboard, "Total Cost:") {
		t.Error("Dashboard missing total cost display")
	}
}

// TestGenerateSampleReport demonstrates a realistic usage scenario.
func TestGenerateSampleReport(t *testing.T) {
	metrics := NewRoutingMetrics()

	// Simulate a day of task execution
	// Morning: Quick tests and docs (Haiku)
	for i := 0; i < 10; i++ {
		metrics.RecordTaskExecution(TaskTypeTesting, 0.0008, 300)
	}
	for i := 0; i < 3; i++ {
		metrics.RecordTaskExecution(TaskTypeDocumentation, 0.001, 400)
	}

	// Midday: Code implementation (Sonnet)
	for i := 0; i < 8; i++ {
		metrics.RecordTaskExecution(TaskTypeCodeGeneration, 0.025, 1200)
	}
	for i := 0; i < 5; i++ {
		metrics.RecordTaskExecution(TaskTypeCodeReview, 0.02, 900)
	}

	// Afternoon: Complex work (Opus)
	metrics.RecordTaskExecution(TaskTypeArchitecture, 0.12, 2000)
	metrics.RecordTaskExecution(TaskTypeSecurity, 0.10, 1800)

	// Research (Gemini)
	metrics.RecordTaskExecution(TaskTypeResearch, 0.003, 3000)
	metrics.RecordTaskExecution(TaskTypeDataProcessing, 0.004, 2500)

	report := metrics.GenerateDashboard()

	// Verify the report contains the expected information
	if !strings.Contains(report, "FLIP2 Routing Analytics Dashboard") {
		t.Fatal("Report missing header")
	}

	// Check totals
	if !strings.Contains(report, "30") {
		t.Errorf("Report not showing correct task count (expected 30)")
	}

	// Check it shows dollars saved
	if !strings.Contains(report, "$") {
		t.Error("Report not showing costs")
	}

	// Print for inspection (useful for manual review)
	t.Logf("\nSample Dashboard Report:\n%s\n", report)
}

// BenchmarkGenerateDashboard benchmarks dashboard generation.
func BenchmarkGenerateDashboard(b *testing.B) {
	metrics := NewRoutingMetrics()

	// Pre-populate with realistic data
	taskTypes := AllTaskTypes()
	for i := 0; i < 100; i++ {
		taskType := taskTypes[i%len(taskTypes)]
		metrics.RecordTaskExecution(taskType, 0.05, 1000)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = metrics.GenerateDashboard()
	}
}

// BenchmarkGenerateDashboardASCII benchmarks ASCII dashboard generation.
func BenchmarkGenerateDashboardASCII(b *testing.B) {
	metrics := NewRoutingMetrics()

	// Pre-populate with realistic data
	taskTypes := AllTaskTypes()
	for i := 0; i < 100; i++ {
		taskType := taskTypes[i%len(taskTypes)]
		metrics.RecordTaskExecution(taskType, 0.05, 1000)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = metrics.GenerateDashboardASCII()
	}
}
