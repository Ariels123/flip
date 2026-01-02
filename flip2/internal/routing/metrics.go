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
// ROUTING METRICS AND COST TRACKING
// ================================================================================

// TaskMetrics tracks execution metrics for a single task type.
type TaskMetrics struct {
	// TaskType is the category of task being tracked.
	TaskType TaskType

	// Count is the total number of tasks of this type executed.
	Count int

	// TotalCostUSD is the sum of all costs for tasks of this type.
	TotalCostUSD float64

	// TotalDurationMS is the sum of all execution durations (in milliseconds).
	TotalDurationMS int64

	// LastExecutedAt is the timestamp of the most recent task execution.
	LastExecutedAt time.Time

	// FirstExecutedAt is the timestamp of the first task execution.
	FirstExecutedAt time.Time

	// MinCostUSD is the minimum cost for a single task of this type.
	MinCostUSD float64

	// MaxCostUSD is the maximum cost for a single task of this type.
	MaxCostUSD float64

	// MinDurationMS is the minimum duration for a task of this type.
	MinDurationMS int64

	// MaxDurationMS is the maximum duration for a task of this type.
	MaxDurationMS int64
}

// AverageCostUSD returns the average cost per task of this type.
func (m *TaskMetrics) AverageCostUSD() float64 {
	if m.Count == 0 {
		return 0.0
	}
	return m.TotalCostUSD / float64(m.Count)
}

// AverageDurationMS returns the average duration per task of this type.
func (m *TaskMetrics) AverageDurationMS() int64 {
	if m.Count == 0 {
		return 0
	}
	return m.TotalDurationMS / int64(m.Count)
}

// RoutingMetrics tracks execution metrics and costs across all task types.
// It uses sync.RWMutex to enable concurrent access from multiple goroutines.
type RoutingMetrics struct {
	mu sync.RWMutex

	// ByTaskType maps task types to their corresponding metrics.
	ByTaskType map[TaskType]*TaskMetrics

	// TotalCostUSD is the sum of all costs across all tasks.
	TotalCostUSD float64

	// TotalTasksExecuted is the total number of tasks executed.
	TotalTasksExecuted int

	// TotalDurationMS is the sum of all execution durations across all tasks.
	TotalDurationMS int64

	// StartedAt is the timestamp when metrics recording started.
	StartedAt time.Time
}

// NewRoutingMetrics creates a new RoutingMetrics instance.
func NewRoutingMetrics() *RoutingMetrics {
	return &RoutingMetrics{
		ByTaskType: make(map[TaskType]*TaskMetrics),
		StartedAt:  time.Now(),
	}
}

// RecordTaskExecution records the execution of a task with its cost and duration.
// This method is safe for concurrent access.
//
// Parameters:
//   - taskType: The category of the task being executed
//   - costUSD: The cost incurred for executing this task (in USD)
//   - durationMS: The duration of task execution (in milliseconds)
func (rm *RoutingMetrics) RecordTaskExecution(taskType TaskType, costUSD float64, durationMS int64) {
	if !taskType.IsValid() {
		// Silently ignore invalid task types
		return
	}

	rm.mu.Lock()
	defer rm.mu.Unlock()

	// Update or create task metrics
	metrics, exists := rm.ByTaskType[taskType]
	if !exists {
		metrics = &TaskMetrics{
			TaskType:        taskType,
			MinCostUSD:      costUSD,
			MaxCostUSD:      costUSD,
			MinDurationMS:   durationMS,
			MaxDurationMS:   durationMS,
			FirstExecutedAt: time.Now(),
		}
		rm.ByTaskType[taskType] = metrics
	}

	// Update metrics
	metrics.Count++
	metrics.TotalCostUSD += costUSD
	metrics.TotalDurationMS += durationMS
	metrics.LastExecutedAt = time.Now()

	// Update min/max costs
	if costUSD < metrics.MinCostUSD {
		metrics.MinCostUSD = costUSD
	}
	if costUSD > metrics.MaxCostUSD {
		metrics.MaxCostUSD = costUSD
	}

	// Update min/max durations
	if durationMS < metrics.MinDurationMS {
		metrics.MinDurationMS = durationMS
	}
	if durationMS > metrics.MaxDurationMS {
		metrics.MaxDurationMS = durationMS
	}

	// Update global metrics
	rm.TotalTasksExecuted++
	rm.TotalCostUSD += costUSD
	rm.TotalDurationMS += durationMS
}

// GetMetrics returns a copy of the metrics for a specific task type.
// Returns nil if no metrics exist for that task type.
func (rm *RoutingMetrics) GetMetrics(taskType TaskType) *TaskMetrics {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	metrics, exists := rm.ByTaskType[taskType]
	if !exists {
		return nil
	}

	// Return a copy to prevent external modification
	copy := *metrics
	return &copy
}

// GenerateReport generates a comprehensive report of all task metrics and costs.
// The report includes:
// - Total costs and task counts
// - Per-task-type breakdown with averages
// - Rankings by cost and frequency
// - Time period summary
func (rm *RoutingMetrics) GenerateReport() string {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	var report strings.Builder

	// Header
	report.WriteString("================================================================================\n")
	report.WriteString("ROUTING METRICS REPORT\n")
	report.WriteString("================================================================================\n\n")

	// Summary section
	report.WriteString("SUMMARY\n")
	report.WriteString("-------\n")
	report.WriteString(fmt.Sprintf("Total Tasks Executed:  %d\n", rm.TotalTasksExecuted))
	report.WriteString(fmt.Sprintf("Total Cost:            $%.4f USD\n", rm.TotalCostUSD))
	report.WriteString(fmt.Sprintf("Total Duration:        %s\n", formatDuration(rm.TotalDurationMS)))
	report.WriteString(fmt.Sprintf("Average Cost/Task:     $%.6f USD\n", rm.averageCostPerTask()))
	report.WriteString(fmt.Sprintf("Average Duration/Task: %d ms\n", rm.averageDurationPerTask()))
	report.WriteString(fmt.Sprintf("Reporting Period:      %s - now\n", rm.StartedAt.Format("2006-01-02 15:04:05")))
	report.WriteString("\n")

	// Task type breakdown
	report.WriteString("TASK TYPE BREAKDOWN\n")
	report.WriteString("-------------------\n")

	// Sort task types by cost (descending) for better readability
	sortedMetrics := rm.getSortedMetrics()
	if len(sortedMetrics) == 0 {
		report.WriteString("No tasks recorded.\n")
	} else {
		// Header row
		report.WriteString(fmt.Sprintf("%-20s | %5s | %10s | %8s | %10s | %8s\n",
			"Task Type", "Count", "Total Cost", "Avg Cost", "Total Dur", "Avg Dur"))
		report.WriteString(strings.Repeat("-", 90) + "\n")

		// Data rows
		for _, metrics := range sortedMetrics {
			report.WriteString(fmt.Sprintf("%-20s | %5d | $%8.4f | $%6.6f | %7dms | %5d ms\n",
				string(metrics.TaskType),
				metrics.Count,
				metrics.TotalCostUSD,
				metrics.AverageCostUSD(),
				metrics.TotalDurationMS,
				metrics.AverageDurationMS(),
			))
		}
	}

	report.WriteString("\n")

	// Top tasks by cost
	report.WriteString("TOP 5 TASKS BY COST\n")
	report.WriteString("-------------------\n")
	if len(sortedMetrics) == 0 {
		report.WriteString("No tasks recorded.\n")
	} else {
		count := 5
		if len(sortedMetrics) < 5 {
			count = len(sortedMetrics)
		}
		for i := 0; i < count; i++ {
			metrics := sortedMetrics[i]
			report.WriteString(fmt.Sprintf("%d. %s: $%.4f (%d tasks)\n",
				i+1,
				metrics.TaskType,
				metrics.TotalCostUSD,
				metrics.Count,
			))
		}
	}

	report.WriteString("\n")

	// Detailed metrics per task type
	report.WriteString("DETAILED METRICS\n")
	report.WriteString("----------------\n")

	for _, metrics := range sortedMetrics {
		report.WriteString(fmt.Sprintf("\n%s:\n", metrics.TaskType))
		report.WriteString(fmt.Sprintf("  Count:              %d\n", metrics.Count))
		report.WriteString(fmt.Sprintf("  Total Cost:         $%.4f USD\n", metrics.TotalCostUSD))
		report.WriteString(fmt.Sprintf("  Average Cost:       $%.6f USD\n", metrics.AverageCostUSD()))
		report.WriteString(fmt.Sprintf("  Min Cost:           $%.6f USD\n", metrics.MinCostUSD))
		report.WriteString(fmt.Sprintf("  Max Cost:           $%.6f USD\n", metrics.MaxCostUSD))
		report.WriteString(fmt.Sprintf("  Total Duration:     %s\n", formatDuration(metrics.TotalDurationMS)))
		report.WriteString(fmt.Sprintf("  Average Duration:   %d ms\n", metrics.AverageDurationMS()))
		report.WriteString(fmt.Sprintf("  Min Duration:       %d ms\n", metrics.MinDurationMS))
		report.WriteString(fmt.Sprintf("  Max Duration:       %d ms\n", metrics.MaxDurationMS))
		report.WriteString(fmt.Sprintf("  First Executed:     %s\n", metrics.FirstExecutedAt.Format("2006-01-02 15:04:05")))
		report.WriteString(fmt.Sprintf("  Last Executed:      %s\n", metrics.LastExecutedAt.Format("2006-01-02 15:04:05")))
	}

	report.WriteString("\n")
	report.WriteString("================================================================================\n")

	return report.String()
}

// getSortedMetrics returns all metrics sorted by total cost (descending).
func (rm *RoutingMetrics) getSortedMetrics() []*TaskMetrics {
	metrics := make([]*TaskMetrics, 0, len(rm.ByTaskType))
	for _, m := range rm.ByTaskType {
		metrics = append(metrics, m)
	}

	// Sort by total cost (descending)
	sort.Slice(metrics, func(i, j int) bool {
		return metrics[i].TotalCostUSD > metrics[j].TotalCostUSD
	})

	return metrics
}

// averageCostPerTask returns the average cost across all tasks.
func (rm *RoutingMetrics) averageCostPerTask() float64 {
	if rm.TotalTasksExecuted == 0 {
		return 0.0
	}
	return rm.TotalCostUSD / float64(rm.TotalTasksExecuted)
}

// averageDurationPerTask returns the average duration across all tasks.
func (rm *RoutingMetrics) averageDurationPerTask() int64 {
	if rm.TotalTasksExecuted == 0 {
		return 0
	}
	return rm.TotalDurationMS / int64(rm.TotalTasksExecuted)
}

// ClearMetrics resets all metrics. Useful for starting a new tracking period.
func (rm *RoutingMetrics) ClearMetrics() {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	rm.ByTaskType = make(map[TaskType]*TaskMetrics)
	rm.TotalCostUSD = 0.0
	rm.TotalTasksExecuted = 0
	rm.TotalDurationMS = 0
	rm.StartedAt = time.Now()
}

// GetTotalCost returns the total cost across all tasks.
func (rm *RoutingMetrics) GetTotalCost() float64 {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	return rm.TotalCostUSD
}

// GetTotalTasksExecuted returns the total number of tasks executed.
func (rm *RoutingMetrics) GetTotalTasksExecuted() int {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	return rm.TotalTasksExecuted
}

// GetTaskCount returns the count of executed tasks for a specific task type.
func (rm *RoutingMetrics) GetTaskCount(taskType TaskType) int {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	if metrics, exists := rm.ByTaskType[taskType]; exists {
		return metrics.Count
	}
	return 0
}

// formatDuration converts milliseconds to a human-readable format.
func formatDuration(ms int64) string {
	if ms < 1000 {
		return fmt.Sprintf("%d ms", ms)
	}
	seconds := ms / 1000
	if seconds < 60 {
		return fmt.Sprintf("%d.%d s", seconds, (ms%1000)/100)
	}
	minutes := seconds / 60
	secs := seconds % 60
	return fmt.Sprintf("%d m %d s", minutes, secs)
}
