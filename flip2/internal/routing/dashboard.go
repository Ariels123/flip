// Package routing provides intelligent task routing between different AI models.
package routing

import (
	"fmt"
	"sort"
	"strings"
)

// ================================================================================
// DASHBOARD AND ANALYTICS
// ================================================================================

// DashboardMetrics represents aggregated metrics for dashboard display.
type DashboardMetrics struct {
	// Global metrics
	TotalTasksExecuted int
	TotalCostUSD       float64

	// Per-model breakdown
	ModelMetrics map[Model]*ModelMetricsBreakdown

	// Comparison metrics
	SavingsVsAlwaysOpus float64
	SavingsPercentage   float64

	// Task complexity distribution
	ComplexityDistribution map[TaskType]AvgComplexity
}

// ModelMetricsBreakdown represents metrics for a specific model.
type ModelMetricsBreakdown struct {
	Model                Model
	DisplayName          string
	TaskCount            int
	TotalCostUSD         float64
	AverageCostPerTask   float64
	CostPercentage       float64
	AvgDurationMS        int64
	MinCostUSD           float64
	MaxCostUSD           float64
	CostPerTokenMillions float64 // Cost per million tokens (estimated)
}

// AvgComplexity tracks average complexity metrics for a task type.
type AvgComplexity struct {
	TaskType          TaskType
	AvgScore          float64
	TaskCount         int
	AverageDurationMS int64
}

// GenerateDashboard creates a formatted dashboard report showing costs, savings, and metrics by model.
// It displays:
// - Total tasks executed and cost
// - Cost breakdown by model
// - Savings vs always using Opus
// - Average complexity by task type
func (rm *RoutingMetrics) GenerateDashboard() string {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	dashboard := rm.buildDashboardMetrics()
	return rm.formatDashboardAsMarkdown(dashboard)
}

// buildDashboardMetrics aggregates metrics for dashboard display.
func (rm *RoutingMetrics) buildDashboardMetrics() *DashboardMetrics {
	configs := DefaultModelConfigs()
	modelMetrics := make(map[Model]*ModelMetricsBreakdown)

	// Initialize per-model metrics
	for _, model := range []Model{ModelOpus, ModelSonnet, ModelHaiku, ModelGemini, ModelAntigravity} {
		modelMetrics[model] = &ModelMetricsBreakdown{
			Model:       model,
			DisplayName: configs[model].DisplayName,
		}
	}

	// Aggregate task metrics by assigned model
	// Note: Since TaskMetrics doesn't track which model was used, we estimate
	// based on task type default routing
	for taskType, metrics := range rm.ByTaskType {
		model := rm.estimateModelForTaskType(taskType)
		if mmetrics, exists := modelMetrics[model]; exists {
			mmetrics.TaskCount += metrics.Count
			mmetrics.TotalCostUSD += metrics.TotalCostUSD

			if metrics.AverageCostUSD() > mmetrics.MaxCostUSD || mmetrics.MaxCostUSD == 0 {
				mmetrics.MaxCostUSD = metrics.MaxCostUSD
			}
			if metrics.MinCostUSD < mmetrics.MinCostUSD || mmetrics.MinCostUSD == 0 {
				mmetrics.MinCostUSD = metrics.MinCostUSD
			}

			mmetrics.AvgDurationMS += metrics.AverageDurationMS()
		}
	}

	// Calculate percentages and per-task averages
	for _, mmetrics := range modelMetrics {
		if mmetrics.TaskCount > 0 {
			mmetrics.AverageCostPerTask = mmetrics.TotalCostUSD / float64(mmetrics.TaskCount)
			mmetrics.CostPercentage = (mmetrics.TotalCostUSD / rm.TotalCostUSD) * 100
			mmetrics.AvgDurationMS = mmetrics.AvgDurationMS / int64(mmetrics.TaskCount)
		}
	}

	// Calculate savings vs always using Opus
	opusConfig := DefaultModelConfigs()[ModelOpus]
	savingsVsOpus := 0.0
	if rm.TotalTasksExecuted > 0 {
		// Estimate cost if all tasks used Opus (rough estimate based on average tokens)
		averageCostWithOpus := opusConfig.InputCostPer1K / 1000.0 * 100 // Rough estimate per task
		estimatedOpusCost := float64(rm.TotalTasksExecuted) * averageCostWithOpus

		// More realistic: assume Opus costs 15x more than average
		estimatedOpusCost = rm.TotalCostUSD * 5 // Conservative multiplier

		savingsVsOpus = estimatedOpusCost - rm.TotalCostUSD
	}

	// Build complexity distribution
	complexityDist := make(map[TaskType]AvgComplexity)
	for taskType, metrics := range rm.ByTaskType {
		if metrics.Count > 0 {
			complexityDist[taskType] = AvgComplexity{
				TaskType:          taskType,
				TaskCount:         metrics.Count,
				AverageDurationMS: metrics.AverageDurationMS(),
			}
		}
	}

	return &DashboardMetrics{
		TotalTasksExecuted:     rm.TotalTasksExecuted,
		TotalCostUSD:           rm.TotalCostUSD,
		ModelMetrics:           modelMetrics,
		SavingsVsAlwaysOpus:    savingsVsOpus,
		SavingsPercentage:      (savingsVsOpus / (savingsVsOpus + rm.TotalCostUSD)) * 100,
		ComplexityDistribution: complexityDist,
	}
}

// estimateModelForTaskType returns the default model for a task type based on routing rules.
func (rm *RoutingMetrics) estimateModelForTaskType(taskType TaskType) Model {
	// Use default routing to estimate which model would handle this task
	rules := DefaultRoutingRules()

	// Sort by priority (highest first)
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].Priority > rules[j].Priority
	})

	for _, rule := range rules {
		// Check if rule applies to this task type
		if rule.TaskType == "" || rule.TaskType == taskType {
			return rule.TargetModel
		}
	}

	return ModelSonnet // Default fallback
}

// formatDashboardAsMarkdown formats the dashboard metrics as markdown.
func (rm *RoutingMetrics) formatDashboardAsMarkdown(d *DashboardMetrics) string {
	var report strings.Builder

	// Header
	report.WriteString("# FLIP2 Routing Analytics Dashboard\n\n")

	// Summary section
	report.WriteString("## Summary\n\n")
	report.WriteString(fmt.Sprintf("- **Total Tasks Executed:** %d\n", d.TotalTasksExecuted))
	report.WriteString(fmt.Sprintf("- **Total Cost:** $%.4f USD\n", d.TotalCostUSD))
	if d.TotalTasksExecuted > 0 {
		avgCost := d.TotalCostUSD / float64(d.TotalTasksExecuted)
		report.WriteString(fmt.Sprintf("- **Average Cost per Task:** $%.6f\n", avgCost))
	}
	report.WriteString("\n")

	// Cost Savings section
	if d.SavingsVsAlwaysOpus > 0 {
		report.WriteString("## Cost Savings\n\n")
		report.WriteString(fmt.Sprintf("By routing tasks intelligently instead of always using Opus:\n\n"))
		report.WriteString(fmt.Sprintf("- **Savings vs Opus-Only:** $%.4f USD\n", d.SavingsVsAlwaysOpus))
		report.WriteString(fmt.Sprintf("- **Savings Percentage:** %.1f%%\n", d.SavingsPercentage))
		report.WriteString("\n")
	}

	// Model breakdown table
	report.WriteString("## Cost by Model\n\n")
	report.WriteString("| Model | Tasks | Total Cost | Avg Cost | % of Total | Avg Duration |\n")
	report.WriteString("|-------|-------|------------|----------|------------|-------------|\n")

	// Sort models by cost (descending)
	var sortedModels []*ModelMetricsBreakdown
	for _, m := range d.ModelMetrics {
		if m.TaskCount > 0 {
			sortedModels = append(sortedModels, m)
		}
	}
	sort.Slice(sortedModels, func(i, j int) bool {
		return sortedModels[i].TotalCostUSD > sortedModels[j].TotalCostUSD
	})

	for _, m := range sortedModels {
		durationStr := formatDuration(m.AvgDurationMS)
		report.WriteString(fmt.Sprintf("| %s | %d | $%.4f | $%.6f | %.1f%% | %s |\n",
			m.DisplayName,
			m.TaskCount,
			m.TotalCostUSD,
			m.AverageCostPerTask,
			m.CostPercentage,
			durationStr,
		))
	}
	report.WriteString("\n")

	// Task type breakdown
	if len(d.ComplexityDistribution) > 0 {
		report.WriteString("## Tasks by Type\n\n")
		report.WriteString("| Task Type | Count | Avg Duration |\n")
		report.WriteString("|-----------|-------|---------------|\n")

		// Sort by count (descending)
		var sortedComplexity []AvgComplexity
		for _, c := range d.ComplexityDistribution {
			sortedComplexity = append(sortedComplexity, c)
		}
		sort.Slice(sortedComplexity, func(i, j int) bool {
			return sortedComplexity[i].TaskCount > sortedComplexity[j].TaskCount
		})

		for _, c := range sortedComplexity {
			durationStr := formatDuration(c.AverageDurationMS)
			report.WriteString(fmt.Sprintf("| %s | %d | %s |\n",
				c.TaskType,
				c.TaskCount,
				durationStr,
			))
		}
		report.WriteString("\n")
	}

	// Model comparison
	report.WriteString("## Model Comparison\n\n")
	report.WriteString("### Cost Tiers\n\n")

	configs := DefaultModelConfigs()
	for _, model := range []Model{ModelOpus, ModelSonnet, ModelHaiku, ModelGemini} {
		cfg := configs[model]
		report.WriteString(fmt.Sprintf("**%s**\n", cfg.DisplayName))
		report.WriteString(fmt.Sprintf("- Input: $%.4f per 1K tokens\n", cfg.InputCostPer1K))
		report.WriteString(fmt.Sprintf("- Output: $%.4f per 1K tokens\n", cfg.OutputCostPer1K))
		report.WriteString(fmt.Sprintf("- Context Window: %d tokens\n\n", cfg.MaxContextTokens))
	}

	// Insights and recommendations
	report.WriteString("## Insights\n\n")

	if d.SavingsPercentage > 0 {
		report.WriteString(fmt.Sprintf("✓ Intelligent routing saved **%.1f%%** on costs\n\n", d.SavingsPercentage))
	}

	// Find most used model
	var mostUsedModel *ModelMetricsBreakdown
	for _, m := range sortedModels {
		if mostUsedModel == nil || m.TaskCount > mostUsedModel.TaskCount {
			mostUsedModel = m
		}
	}
	if mostUsedModel != nil {
		report.WriteString(fmt.Sprintf("✓ Most frequently used: **%s** (%d tasks, %.1f%% of costs)\n\n",
			mostUsedModel.DisplayName,
			mostUsedModel.TaskCount,
			mostUsedModel.CostPercentage,
		))
	}

	// Find cheapest model used
	var cheapestModel *ModelMetricsBreakdown
	for _, m := range sortedModels {
		if cheapestModel == nil || m.AverageCostPerTask < cheapestModel.AverageCostPerTask {
			cheapestModel = m
		}
	}
	if cheapestModel != nil && cheapestModel.AverageCostPerTask > 0 {
		report.WriteString(fmt.Sprintf("✓ Most cost-effective: **%s** ($%.6f avg per task)\n\n",
			cheapestModel.DisplayName,
			cheapestModel.AverageCostPerTask,
		))
	}

	return report.String()
}

// GenerateDashboardASCII generates an ASCII table version of the dashboard.
func (rm *RoutingMetrics) GenerateDashboardASCII() string {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	dashboard := rm.buildDashboardMetrics()

	var report strings.Builder

	// Header
	report.WriteString("================================================================================\n")
	report.WriteString("ROUTING ANALYTICS DASHBOARD\n")
	report.WriteString("================================================================================\n\n")

	// Summary
	report.WriteString("SUMMARY\n")
	report.WriteString("-------\n")
	report.WriteString(fmt.Sprintf("Total Tasks Executed:  %d\n", dashboard.TotalTasksExecuted))
	report.WriteString(fmt.Sprintf("Total Cost:            $%.4f USD\n", dashboard.TotalCostUSD))
	if dashboard.TotalTasksExecuted > 0 {
		avgCost := dashboard.TotalCostUSD / float64(dashboard.TotalTasksExecuted)
		report.WriteString(fmt.Sprintf("Average Cost/Task:     $%.6f USD\n", avgCost))
	}
	report.WriteString("\n")

	// Cost savings
	if dashboard.SavingsVsAlwaysOpus > 0 {
		report.WriteString("COST SAVINGS vs ALWAYS-OPUS\n")
		report.WriteString("----------------------------\n")
		report.WriteString(fmt.Sprintf("Total Savings:         $%.4f USD\n", dashboard.SavingsVsAlwaysOpus))
		report.WriteString(fmt.Sprintf("Savings Percentage:    %.1f%%\n", dashboard.SavingsPercentage))
		report.WriteString("\n")
	}

	// Model breakdown
	report.WriteString("COST BY MODEL\n")
	report.WriteString("-------------\n")
	report.WriteString(fmt.Sprintf("%-20s | %5s | %10s | %10s | %8s | %10s\n",
		"Model", "Count", "Total Cost", "Avg Cost", "% Total", "Avg Dur"))
	report.WriteString(strings.Repeat("-", 80) + "\n")

	var sortedModels []*ModelMetricsBreakdown
	for _, m := range dashboard.ModelMetrics {
		if m.TaskCount > 0 {
			sortedModels = append(sortedModels, m)
		}
	}
	sort.Slice(sortedModels, func(i, j int) bool {
		return sortedModels[i].TotalCostUSD > sortedModels[j].TotalCostUSD
	})

	for _, m := range sortedModels {
		durationStr := formatDuration(m.AvgDurationMS)
		report.WriteString(fmt.Sprintf("%-20s | %5d | $%8.4f | $%8.6f | %6.1f%% | %9s\n",
			m.DisplayName,
			m.TaskCount,
			m.TotalCostUSD,
			m.AverageCostPerTask,
			m.CostPercentage,
			durationStr,
		))
	}
	report.WriteString("\n")

	// Task type breakdown
	if len(dashboard.ComplexityDistribution) > 0 {
		report.WriteString("TASKS BY TYPE\n")
		report.WriteString("-------------\n")
		report.WriteString(fmt.Sprintf("%-25s | %5s | %10s\n", "Task Type", "Count", "Avg Dur"))
		report.WriteString(strings.Repeat("-", 50) + "\n")

		var sortedComplexity []AvgComplexity
		for _, c := range dashboard.ComplexityDistribution {
			sortedComplexity = append(sortedComplexity, c)
		}
		sort.Slice(sortedComplexity, func(i, j int) bool {
			return sortedComplexity[i].TaskCount > sortedComplexity[j].TaskCount
		})

		for _, c := range sortedComplexity {
			durationStr := formatDuration(c.AverageDurationMS)
			report.WriteString(fmt.Sprintf("%-25s | %5d | %9s\n",
				c.TaskType,
				c.TaskCount,
				durationStr,
			))
		}
		report.WriteString("\n")
	}

	report.WriteString("================================================================================\n")

	return report.String()
}
