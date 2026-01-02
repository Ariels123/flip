// Package routing provides intelligent task routing between different AI models.
package routing

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// ================================================================================
// A/B TESTING FOR ROUTING DECISIONS
// ================================================================================

// ABTestOutcome represents the result of a task executed in an A/B test.
type ABTestOutcome struct {
	// TaskID is the unique identifier for the task.
	TaskID string

	// Variant indicates if this was control ("control") or variant ("variant").
	Variant string

	// Model is the model that was used to execute the task.
	Model Model

	// Cost is the cost incurred for executing this task.
	Cost float64

	// Success indicates if the task execution was successful.
	Success bool

	// Duration is the execution time in milliseconds.
	Duration int64

	// Timestamp is when the outcome was recorded.
	Timestamp time.Time
}

// ABTestOutcomes is a type alias for []ABTestOutcome to allow methods.
type ABTestOutcomes []ABTestOutcome

// ABTest manages A/B testing experiments for routing decisions.
type ABTest struct {
	mu sync.RWMutex

	// ExperimentID is a unique identifier for this A/B testing experiment.
	ExperimentID string

	// ControlModel is the baseline model being tested against.
	ControlModel Model

	// VariantModel is the alternative model being tested.
	VariantModel Model

	// Percentage is the percentage of tasks routed to the variant.
	Percentage int

	// Outcomes stores all recorded task outcomes from this experiment.
	Outcomes []ABTestOutcome

	// CreatedAt is when the experiment was started.
	CreatedAt time.Time

	// UpdatedAt is when the last outcome was recorded.
	UpdatedAt time.Time

	// Active indicates if this experiment is currently running.
	Active bool
}

// NewABTest creates a new A/B testing experiment.
func NewABTest(experimentID string, controlModel, variantModel Model, percentage int) (*ABTest, error) {
	if !controlModel.IsValid() {
		return nil, fmt.Errorf("invalid control model: %s", controlModel)
	}
	if !variantModel.IsValid() {
		return nil, fmt.Errorf("invalid variant model: %s", variantModel)
	}
	if percentage < 0 || percentage > 100 {
		return nil, fmt.Errorf("percentage must be between 0 and 100, got %d", percentage)
	}
	if experimentID == "" {
		return nil, fmt.Errorf("experiment ID cannot be empty")
	}

	return &ABTest{
		ExperimentID: experimentID,
		ControlModel: controlModel,
		VariantModel: variantModel,
		Percentage:   percentage,
		Outcomes:     make([]ABTestOutcome, 0),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		Active:       true,
	}, nil
}

// RecordOutcome records the outcome of a task executed in the A/B test.
// The variant parameter should be "control" or "variant".
func (at *ABTest) RecordOutcome(taskID string, variant string, model Model, cost float64, success bool, durationMS int64) error {
	if taskID == "" {
		return fmt.Errorf("task ID cannot be empty")
	}
	if variant != "control" && variant != "variant" {
		return fmt.Errorf("variant must be 'control' or 'variant', got '%s'", variant)
	}
	if !model.IsValid() {
		return fmt.Errorf("invalid model: %s", model)
	}
	if cost < 0 {
		return fmt.Errorf("cost cannot be negative: %f", cost)
	}
	if durationMS < 0 {
		return fmt.Errorf("duration cannot be negative: %d", durationMS)
	}

	at.mu.Lock()
	defer at.mu.Unlock()

	outcome := ABTestOutcome{
		TaskID:    taskID,
		Variant:   variant,
		Model:     model,
		Cost:      cost,
		Success:   success,
		Duration:  durationMS,
		Timestamp: time.Now(),
	}

	at.Outcomes = append(at.Outcomes, outcome)
	at.UpdatedAt = time.Now()

	return nil
}

// Stop stops the A/B testing experiment.
func (at *ABTest) Stop() {
	at.mu.Lock()
	defer at.mu.Unlock()
	at.Active = false
}

// GetOutcomeCount returns the number of recorded outcomes.
func (at *ABTest) GetOutcomeCount() int {
	at.mu.RLock()
	defer at.mu.RUnlock()
	return len(at.Outcomes)
}

// GenerateABReport generates a comprehensive report comparing control and variant performance.
func (at *ABTest) GenerateABReport() string {
	at.mu.RLock()
	defer at.mu.RUnlock()

	var report strings.Builder

	// Header
	report.WriteString("================================================================================\n")
	report.WriteString("A/B TESTING REPORT\n")
	report.WriteString("================================================================================\n\n")

	// Experiment info
	report.WriteString("EXPERIMENT INFORMATION\n")
	report.WriteString("----------------------\n")
	report.WriteString(fmt.Sprintf("Experiment ID:      %s\n", at.ExperimentID))
	report.WriteString(fmt.Sprintf("Control Model:      %s\n", at.ControlModel.DisplayName()))
	report.WriteString(fmt.Sprintf("Variant Model:      %s\n", at.VariantModel.DisplayName()))
	report.WriteString(fmt.Sprintf("Variant Percentage: %d%%\n", at.Percentage))
	report.WriteString(fmt.Sprintf("Status:             "))
	if at.Active {
		report.WriteString("ACTIVE")
	} else {
		report.WriteString("STOPPED")
	}
	report.WriteString("\n")
	report.WriteString(fmt.Sprintf("Created:            %s\n", at.CreatedAt.Format("2006-01-02 15:04:05")))
	report.WriteString(fmt.Sprintf("Last Updated:       %s\n", at.UpdatedAt.Format("2006-01-02 15:04:05")))
	report.WriteString("\n")

	// Calculate metrics by variant
	controlMetrics := at.calculateMetrics("control")
	variantMetrics := at.calculateMetrics("variant")

	// Summary comparison
	report.WriteString("SUMMARY COMPARISON\n")
	report.WriteString("------------------\n")
	report.WriteString(fmt.Sprintf("%-25s | %-20s | %-20s | %-15s\n",
		"Metric", "Control", "Variant", "Difference"))
	report.WriteString(strings.Repeat("-", 85) + "\n")

	// Count
	report.WriteString(fmt.Sprintf("%-25s | %20d | %20d | %15d\n",
		"Total Tasks",
		controlMetrics.count,
		variantMetrics.count,
		variantMetrics.count-controlMetrics.count,
	))

	// Success rate
	report.WriteString(fmt.Sprintf("%-25s | %19.1f%% | %19.1f%% | %14.1f%%\n",
		"Success Rate",
		controlMetrics.successRate*100,
		variantMetrics.successRate*100,
		(variantMetrics.successRate-controlMetrics.successRate)*100,
	))

	// Average cost
	report.WriteString(fmt.Sprintf("%-25s | $%18.6f | $%18.6f | $%13.6f\n",
		"Avg Cost per Task",
		controlMetrics.avgCost,
		variantMetrics.avgCost,
		variantMetrics.avgCost-controlMetrics.avgCost,
	))

	// Total cost
	report.WriteString(fmt.Sprintf("%-25s | $%18.4f | $%18.4f | $%13.4f\n",
		"Total Cost",
		controlMetrics.totalCost,
		variantMetrics.totalCost,
		variantMetrics.totalCost-controlMetrics.totalCost,
	))

	// Average duration
	report.WriteString(fmt.Sprintf("%-25s | %20d ms | %20d ms | %15d ms\n",
		"Avg Duration",
		controlMetrics.avgDuration,
		variantMetrics.avgDuration,
		variantMetrics.avgDuration-controlMetrics.avgDuration,
	))

	report.WriteString("\n")

	// Detailed breakdown
	report.WriteString("CONTROL VARIANT DETAILS\n")
	report.WriteString("----------------------\n")
	report.WriteString(fmt.Sprintf("Total Tasks:       %d\n", controlMetrics.count))
	report.WriteString(fmt.Sprintf("Successful:        %d (%.1f%%)\n", controlMetrics.successCount, controlMetrics.successRate*100))
	report.WriteString(fmt.Sprintf("Failed:            %d (%.1f%%)\n", controlMetrics.failureCount, (1-controlMetrics.successRate)*100))
	report.WriteString(fmt.Sprintf("Total Cost:        $%.4f USD\n", controlMetrics.totalCost))
	report.WriteString(fmt.Sprintf("Average Cost:      $%.6f USD\n", controlMetrics.avgCost))
	report.WriteString(fmt.Sprintf("Min Cost:          $%.6f USD\n", controlMetrics.minCost))
	report.WriteString(fmt.Sprintf("Max Cost:          $%.6f USD\n", controlMetrics.maxCost))
	report.WriteString(fmt.Sprintf("Total Duration:    %d ms\n", controlMetrics.totalDuration))
	report.WriteString(fmt.Sprintf("Average Duration:  %d ms\n", controlMetrics.avgDuration))
	report.WriteString(fmt.Sprintf("Min Duration:      %d ms\n", controlMetrics.minDuration))
	report.WriteString(fmt.Sprintf("Max Duration:      %d ms\n", controlMetrics.maxDuration))

	report.WriteString("\n")

	report.WriteString("VARIANT VARIANT DETAILS\n")
	report.WriteString("----------------------\n")
	report.WriteString(fmt.Sprintf("Total Tasks:       %d\n", variantMetrics.count))
	report.WriteString(fmt.Sprintf("Successful:        %d (%.1f%%)\n", variantMetrics.successCount, variantMetrics.successRate*100))
	report.WriteString(fmt.Sprintf("Failed:            %d (%.1f%%)\n", variantMetrics.failureCount, (1-variantMetrics.successRate)*100))
	report.WriteString(fmt.Sprintf("Total Cost:        $%.4f USD\n", variantMetrics.totalCost))
	report.WriteString(fmt.Sprintf("Average Cost:      $%.6f USD\n", variantMetrics.avgCost))
	report.WriteString(fmt.Sprintf("Min Cost:          $%.6f USD\n", variantMetrics.minCost))
	report.WriteString(fmt.Sprintf("Max Cost:          $%.6f USD\n", variantMetrics.maxCost))
	report.WriteString(fmt.Sprintf("Total Duration:    %d ms\n", variantMetrics.totalDuration))
	report.WriteString(fmt.Sprintf("Average Duration:  %d ms\n", variantMetrics.avgDuration))
	report.WriteString(fmt.Sprintf("Min Duration:      %d ms\n", variantMetrics.minDuration))
	report.WriteString(fmt.Sprintf("Max Duration:      %d ms\n", variantMetrics.maxDuration))

	report.WriteString("\n")

	// Cost efficiency analysis
	report.WriteString("COST EFFICIENCY ANALYSIS\n")
	report.WriteString("------------------------\n")
	if controlMetrics.count > 0 && variantMetrics.count > 0 {
		costDiff := variantMetrics.avgCost - controlMetrics.avgCost
		costDiffPct := (costDiff / controlMetrics.avgCost) * 100
		report.WriteString(fmt.Sprintf("Cost per successful task (Control):  $%.6f USD\n", at.getCostPerSuccess(controlMetrics)))
		report.WriteString(fmt.Sprintf("Cost per successful task (Variant):  $%.6f USD\n", at.getCostPerSuccess(variantMetrics)))
		report.WriteString(fmt.Sprintf("Cost difference:                    $%.6f USD (%.1f%%)\n", costDiff, costDiffPct))

		if costDiff < 0 {
			report.WriteString(fmt.Sprintf("RECOMMENDATION: Variant is %.1f%% cheaper per task\n", -costDiffPct))
		} else {
			report.WriteString(fmt.Sprintf("RECOMMENDATION: Control is %.1f%% cheaper per task\n", costDiffPct))
		}
	}

	report.WriteString("\n")
	report.WriteString("================================================================================\n")

	return report.String()
}

// metricsData holds calculated metrics for a variant group.
type metricsData struct {
	count         int
	successCount  int
	failureCount  int
	successRate   float64
	totalCost     float64
	avgCost       float64
	minCost       float64
	maxCost       float64
	totalDuration int64
	avgDuration   int64
	minDuration   int64
	maxDuration   int64
}

// calculateMetrics calculates metrics for a specific variant group.
func (at *ABTest) calculateMetrics(variant string) metricsData {
	var metrics metricsData
	metrics.minCost = float64(1<<63 - 1) // Max float64
	metrics.minDuration = 1<<63 - 1      // Max int64

	for _, outcome := range at.Outcomes {
		if outcome.Variant != variant {
			continue
		}

		metrics.count++
		metrics.totalCost += outcome.Cost
		metrics.totalDuration += outcome.Duration

		if outcome.Success {
			metrics.successCount++
		} else {
			metrics.failureCount++
		}

		if outcome.Cost < metrics.minCost {
			metrics.minCost = outcome.Cost
		}
		if outcome.Cost > metrics.maxCost {
			metrics.maxCost = outcome.Cost
		}

		if outcome.Duration < metrics.minDuration {
			metrics.minDuration = outcome.Duration
		}
		if outcome.Duration > metrics.maxDuration {
			metrics.maxDuration = outcome.Duration
		}
	}

	if metrics.count > 0 {
		metrics.avgCost = metrics.totalCost / float64(metrics.count)
		metrics.avgDuration = metrics.totalDuration / int64(metrics.count)
		metrics.successRate = float64(metrics.successCount) / float64(metrics.count)
	}

	// Handle uninitialized min values
	if metrics.count == 0 {
		metrics.minCost = 0
		metrics.minDuration = 0
	}

	return metrics
}

// getCostPerSuccess calculates the cost per successful task.
func (at *ABTest) getCostPerSuccess(metrics metricsData) float64 {
	if metrics.successCount == 0 {
		return 0
	}
	return metrics.totalCost / float64(metrics.successCount)
}

// ExportOutcomes returns all recorded outcomes as a slice.
func (at *ABTest) ExportOutcomes() []ABTestOutcome {
	at.mu.RLock()
	defer at.mu.RUnlock()

	// Return a copy to prevent external modification
	outcomes := make([]ABTestOutcome, len(at.Outcomes))
	copy(outcomes, at.Outcomes)
	return outcomes
}

// FilterOutcomesByVariant returns outcomes for a specific variant.
func (at *ABTest) FilterOutcomesByVariant(variant string) []ABTestOutcome {
	at.mu.RLock()
	defer at.mu.RUnlock()

	var filtered []ABTestOutcome
	for _, outcome := range at.Outcomes {
		if outcome.Variant == variant {
			filtered = append(filtered, outcome)
		}
	}
	return filtered
}

// SortByTime sorts outcomes by timestamp (earliest first).
func (outcomes ABTestOutcomes) SortByTime() {
	sort.Slice(outcomes, func(i, j int) bool {
		return outcomes[i].Timestamp.Before(outcomes[j].Timestamp)
	})
}

// SortByCost sorts outcomes by cost (lowest first).
func (outcomes ABTestOutcomes) SortByCost() {
	sort.Slice(outcomes, func(i, j int) bool {
		return outcomes[i].Cost < outcomes[j].Cost
	})
}
