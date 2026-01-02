package errors

import (
	"sync"
	"time"
)

// ErrorMetrics tracks error counts and statistics with thread-safe operations.
// It maintains counts by ErrorCode and ErrorCategory, along with timestamps
// for time-series analysis.
type ErrorMetrics struct {
	// mu protects all maps below
	mu sync.RWMutex

	// errorCounts maps ErrorCode to count of occurrences
	errorCounts map[ErrorCode]int64

	// categoryCounts maps ErrorCategory to count of occurrences
	categoryCounts map[ErrorCategory]int64

	// timestamps records when errors occurred for time-series analysis
	timestamps []time.Time
}

// NewErrorMetrics creates a new error metrics tracker.
func NewErrorMetrics() *ErrorMetrics {
	return &ErrorMetrics{
		errorCounts:    make(map[ErrorCode]int64),
		categoryCounts: make(map[ErrorCategory]int64),
		timestamps:     make([]time.Time, 0),
	}
}

// RecordError increments counters for the given error.
// This method is thread-safe and records both the error code count,
// the category count, and the timestamp for time-series analysis.
func (em *ErrorMetrics) RecordError(err *ExecutionError) {
	if err == nil {
		return
	}

	em.mu.Lock()
	defer em.mu.Unlock()

	// Increment error code count
	em.errorCounts[err.Code]++

	// Increment category count
	category := err.Code.Category()
	em.categoryCounts[category]++

	// Record timestamp for time-series analysis
	em.timestamps = append(em.timestamps, time.Now())
}

// GetErrorCounts returns a snapshot of error counts by ErrorCode.
// The returned map is a copy and safe to modify.
// This method is thread-safe using read lock.
func (em *ErrorMetrics) GetErrorCounts() map[ErrorCode]int64 {
	em.mu.RLock()
	defer em.mu.RUnlock()

	// Create a copy of the map to avoid external modifications
	counts := make(map[ErrorCode]int64, len(em.errorCounts))
	for code, count := range em.errorCounts {
		counts[code] = count
	}
	return counts
}

// GetCategoryCounts returns a snapshot of error counts by ErrorCategory.
// The returned map is a copy and safe to modify.
// This method is thread-safe using read lock.
func (em *ErrorMetrics) GetCategoryCounts() map[ErrorCategory]int64 {
	em.mu.RLock()
	defer em.mu.RUnlock()

	// Create a copy of the map to avoid external modifications
	counts := make(map[ErrorCategory]int64, len(em.categoryCounts))
	for category, count := range em.categoryCounts {
		counts[category] = count
	}
	return counts
}

// GetTimestamps returns a snapshot of all recorded error timestamps.
// The returned slice is a copy and safe to modify.
// This method is thread-safe using read lock.
func (em *ErrorMetrics) GetTimestamps() []time.Time {
	em.mu.RLock()
	defer em.mu.RUnlock()

	// Create a copy of the timestamps slice to avoid external modifications
	timestamps := make([]time.Time, len(em.timestamps))
	copy(timestamps, em.timestamps)
	return timestamps
}

// GetTotalErrorCount returns the total number of errors recorded.
// This method is thread-safe using read lock.
func (em *ErrorMetrics) GetTotalErrorCount() int64 {
	em.mu.RLock()
	defer em.mu.RUnlock()

	var total int64
	for _, count := range em.errorCounts {
		total += count
	}
	return total
}

// GetErrorCountByCode returns the count for a specific error code.
// Returns 0 if the code has not been recorded.
// This method is thread-safe using read lock.
func (em *ErrorMetrics) GetErrorCountByCode(code ErrorCode) int64 {
	em.mu.RLock()
	defer em.mu.RUnlock()

	return em.errorCounts[code]
}

// GetErrorCountByCategory returns the count for a specific error category.
// Returns 0 if the category has not been recorded.
// This method is thread-safe using read lock.
func (em *ErrorMetrics) GetErrorCountByCategory(category ErrorCategory) int64 {
	em.mu.RLock()
	defer em.mu.RUnlock()

	return em.categoryCounts[category]
}

// Reset clears all recorded error metrics.
// This method is thread-safe using write lock.
// Use this to reset metrics for a new monitoring period.
func (em *ErrorMetrics) Reset() {
	em.mu.Lock()
	defer em.mu.Unlock()

	em.errorCounts = make(map[ErrorCode]int64)
	em.categoryCounts = make(map[ErrorCategory]int64)
	em.timestamps = make([]time.Time, 0)
}

// GetTimestampsInRange returns timestamps within the specified time range.
// This is useful for time-series analysis and trend detection.
// The returned slice is a copy and safe to modify.
// This method is thread-safe using read lock.
func (em *ErrorMetrics) GetTimestampsInRange(start, end time.Time) []time.Time {
	em.mu.RLock()
	defer em.mu.RUnlock()

	var filtered []time.Time
	for _, ts := range em.timestamps {
		if ts.After(start) && ts.Before(end) {
			filtered = append(filtered, ts)
		}
	}
	return filtered
}

// GetErrorCountSince returns the total count of errors since the given time.
// This is useful for calculating error rates over time windows.
// This method is thread-safe using read lock.
func (em *ErrorMetrics) GetErrorCountSince(since time.Time) int64 {
	em.mu.RLock()
	defer em.mu.RUnlock()

	var count int64
	for _, ts := range em.timestamps {
		if ts.After(since) {
			count++
		}
	}
	return count
}
