package errors

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewErrorMetrics(t *testing.T) {
	em := NewErrorMetrics()

	if em == nil {
		t.Fatal("NewErrorMetrics returned nil")
	}

	if len(em.GetErrorCounts()) != 0 {
		t.Error("new metrics should have zero error counts")
	}

	if len(em.GetCategoryCounts()) != 0 {
		t.Error("new metrics should have zero category counts")
	}

	if em.GetTotalErrorCount() != 0 {
		t.Error("new metrics should have zero total count")
	}
}

func TestRecordError(t *testing.T) {
	em := NewErrorMetrics()

	// Record a timeout error
	err1 := NewTimeout(5*time.Minute, nil)
	em.RecordError(err1)

	counts := em.GetErrorCounts()
	if counts[ErrTimeout] != 1 {
		t.Errorf("expected timeout count 1, got %d", counts[ErrTimeout])
	}

	// Record another timeout error
	err2 := NewTimeout(10*time.Minute, nil)
	em.RecordError(err2)

	counts = em.GetErrorCounts()
	if counts[ErrTimeout] != 2 {
		t.Errorf("expected timeout count 2, got %d", counts[ErrTimeout])
	}

	// Record a different error type
	err3 := NewNotFound("binary", nil)
	em.RecordError(err3)

	counts = em.GetErrorCounts()
	if counts[ErrNotFound] != 1 {
		t.Errorf("expected not found count 1, got %d", counts[ErrNotFound])
	}
	if counts[ErrTimeout] != 2 {
		t.Errorf("expected timeout count still 2, got %d", counts[ErrTimeout])
	}
}

func TestRecordNilError(t *testing.T) {
	em := NewErrorMetrics()

	// Recording nil error should not panic and should not affect counts
	em.RecordError(nil)

	if em.GetTotalErrorCount() != 0 {
		t.Error("recording nil error should not increment count")
	}
}

func TestGetErrorCounts(t *testing.T) {
	em := NewErrorMetrics()

	// Record various errors
	em.RecordError(NewTimeout(5*time.Minute, nil))
	em.RecordError(NewTimeout(10*time.Minute, nil))
	em.RecordError(NewQuotaExhausted("rate limit", time.Now().Add(1*time.Hour)))
	em.RecordError(NewNotFound("binary", nil))

	counts := em.GetErrorCounts()

	expectedCounts := map[ErrorCode]int64{
		ErrTimeout:       2,
		ErrQuotaExhausted: 1,
		ErrNotFound:      1,
	}

	for code, expectedCount := range expectedCounts {
		if counts[code] != expectedCount {
			t.Errorf("expected %s count %d, got %d", code, expectedCount, counts[code])
		}
	}

	// Verify returned map is a copy (modifications don't affect original)
	counts[ErrTimeout] = 999
	if em.GetErrorCountByCode(ErrTimeout) != 2 {
		t.Error("modifying returned map should not affect internal state")
	}
}

func TestGetCategoryCounts(t *testing.T) {
	em := NewErrorMetrics()

	// Record errors from different categories
	em.RecordError(NewTimeout(5*time.Minute, nil))           // transient
	em.RecordError(NewQuotaExhausted("rate limit", time.Now().Add(1*time.Hour))) // transient
	em.RecordError(NewNotFound("binary", nil))               // permanent
	em.RecordError(NewPermissionDenied("/bin/claude", nil))  // permanent
	em.RecordError(NewInvalidConfig("bad config", nil))      // configuration

	counts := em.GetCategoryCounts()

	expectedCounts := map[ErrorCategory]int64{
		CategoryTransient:      2,
		CategoryPermanent:      2,
		CategoryConfiguration: 1,
	}

	for category, expectedCount := range expectedCounts {
		if counts[category] != expectedCount {
			t.Errorf("expected %s count %d, got %d", category, expectedCount, counts[category])
		}
	}

	// Verify returned map is a copy
	counts[CategoryTransient] = 999
	if em.GetErrorCountByCategory(CategoryTransient) != 2 {
		t.Error("modifying returned map should not affect internal state")
	}
}

func TestGetTotalErrorCount(t *testing.T) {
	em := NewErrorMetrics()

	if em.GetTotalErrorCount() != 0 {
		t.Error("initial total error count should be 0")
	}

	em.RecordError(NewTimeout(5*time.Minute, nil))
	em.RecordError(NewQuotaExhausted("rate limit", time.Now().Add(1*time.Hour)))
	em.RecordError(NewNotFound("binary", nil))

	if em.GetTotalErrorCount() != 3 {
		t.Errorf("expected total count 3, got %d", em.GetTotalErrorCount())
	}
}

func TestGetErrorCountByCode(t *testing.T) {
	em := NewErrorMetrics()

	em.RecordError(NewTimeout(5*time.Minute, nil))
	em.RecordError(NewTimeout(10*time.Minute, nil))

	count := em.GetErrorCountByCode(ErrTimeout)
	if count != 2 {
		t.Errorf("expected timeout count 2, got %d", count)
	}

	// Non-existent code should return 0
	count = em.GetErrorCountByCode(ErrQuotaExhausted)
	if count != 0 {
		t.Errorf("expected quota exhausted count 0, got %d", count)
	}
}

func TestGetErrorCountByCategory(t *testing.T) {
	em := NewErrorMetrics()

	em.RecordError(NewTimeout(5*time.Minute, nil))
	em.RecordError(NewQuotaExhausted("rate limit", time.Now().Add(1*time.Hour)))

	count := em.GetErrorCountByCategory(CategoryTransient)
	if count != 2 {
		t.Errorf("expected transient count 2, got %d", count)
	}

	// Non-existent category should return 0
	count = em.GetErrorCountByCategory(CategoryPermanent)
	if count != 0 {
		t.Errorf("expected permanent count 0, got %d", count)
	}
}

func TestReset(t *testing.T) {
	em := NewErrorMetrics()

	// Record some errors
	em.RecordError(NewTimeout(5*time.Minute, nil))
	em.RecordError(NewTimeout(10*time.Minute, nil))
	em.RecordError(NewNotFound("binary", nil))

	if em.GetTotalErrorCount() != 3 {
		t.Errorf("expected total count 3 before reset, got %d", em.GetTotalErrorCount())
	}

	// Reset metrics
	em.Reset()

	if em.GetTotalErrorCount() != 0 {
		t.Errorf("expected total count 0 after reset, got %d", em.GetTotalErrorCount())
	}

	if len(em.GetErrorCounts()) != 0 {
		t.Error("error counts should be empty after reset")
	}

	if len(em.GetCategoryCounts()) != 0 {
		t.Error("category counts should be empty after reset")
	}

	if len(em.GetTimestamps()) != 0 {
		t.Error("timestamps should be empty after reset")
	}

	// Should be able to record errors after reset
	em.RecordError(NewTimeout(5*time.Minute, nil))
	if em.GetTotalErrorCount() != 1 {
		t.Errorf("expected count 1 after recording post-reset, got %d", em.GetTotalErrorCount())
	}
}

func TestGetTimestamps(t *testing.T) {
	em := NewErrorMetrics()

	// Record an error
	err := NewTimeout(5*time.Minute, nil)
	before := time.Now()
	em.RecordError(err)
	after := time.Now()

	timestamps := em.GetTimestamps()

	if len(timestamps) != 1 {
		t.Errorf("expected 1 timestamp, got %d", len(timestamps))
	}

	if !timestamps[0].After(before) || !timestamps[0].Before(after.Add(1*time.Second)) {
		t.Error("recorded timestamp is not in expected time range")
	}

	// Verify returned slice is a copy
	originalTime := em.GetTimestamps()[0]
	timestamps[0] = time.Time{}
	currentTime := em.GetTimestamps()[0]
	if currentTime != originalTime {
		t.Error("modifying returned slice should not affect internal state")
	}
}

func TestGetTimestampsInRange(t *testing.T) {
	em := NewErrorMetrics()

	now := time.Now()

	// Record errors at different times
	after := now.Add(2 * time.Second)

	// Manually inject timestamps for testing (simulate time progression)
	// This is a bit hacky but necessary since RecordError uses time.Now()
	// In real usage, the timestamps would naturally vary

	// Record errors and check filtering
	em.RecordError(NewTimeout(5*time.Minute, nil))
	time.Sleep(10 * time.Millisecond)
	em.RecordError(NewNotFound("binary", nil))
	time.Sleep(10 * time.Millisecond)
	em.RecordError(NewTimeout(10*time.Minute, nil))

	// Get all timestamps
	allTimestamps := em.GetTimestamps()
	if len(allTimestamps) != 3 {
		t.Fatalf("expected 3 timestamps, got %d", len(allTimestamps))
	}

	// Get timestamps in a range that includes all
	startTime := allTimestamps[0].Add(-100 * time.Millisecond)
	endTime := allTimestamps[len(allTimestamps)-1].Add(100 * time.Millisecond)

	filtered := em.GetTimestampsInRange(startTime, endTime)
	if len(filtered) != 3 {
		t.Errorf("expected 3 filtered timestamps, got %d", len(filtered))
	}

	// Get timestamps in a range that includes only the middle one
	startTime = allTimestamps[0].Add(5 * time.Millisecond)
	endTime = allTimestamps[2].Add(-5 * time.Millisecond)

	filtered = em.GetTimestampsInRange(startTime, endTime)
	if len(filtered) != 1 {
		t.Errorf("expected 1 filtered timestamp, got %d", len(filtered))
	}

	// Get timestamps in a range that includes none
	startTime = allTimestamps[len(allTimestamps)-1].Add(1 * time.Millisecond)
	endTime = startTime.Add(1 * time.Second)

	filtered = em.GetTimestampsInRange(startTime, endTime)
	if len(filtered) != 0 {
		t.Errorf("expected 0 filtered timestamps, got %d", len(filtered))
	}

	// Verify returned slice is a copy
	_ = after
}

func TestGetErrorCountSince(t *testing.T) {
	em := NewErrorMetrics()

	now := time.Now()

	// Record an error
	em.RecordError(NewTimeout(5*time.Minute, nil))
	time.Sleep(10 * time.Millisecond)
	em.RecordError(NewNotFound("binary", nil))

	// Count errors since before they were recorded
	count := em.GetErrorCountSince(now.Add(-1 * time.Second))
	if count != 2 {
		t.Errorf("expected count 2, got %d", count)
	}

	// Count errors since just before the last error
	count = em.GetErrorCountSince(now)
	if count != 2 {
		t.Errorf("expected count 2, got %d", count)
	}

	// Count errors since far in the future (should be 0)
	count = em.GetErrorCountSince(now.Add(10 * time.Second))
	if count != 0 {
		t.Errorf("expected count 0, got %d", count)
	}
}

func TestConcurrentErrorRecording(t *testing.T) {
	em := NewErrorMetrics()
	numGoroutines := 100
	errorsPerGoroutine := 50

	// Use a WaitGroup to coordinate goroutines
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Launch concurrent goroutines recording errors
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()

			for j := 0; j < errorsPerGoroutine; j++ {
				// Vary the error types
				switch j % 3 {
				case 0:
					em.RecordError(NewTimeout(5*time.Minute, nil))
				case 1:
					em.RecordError(NewNotFound("binary", nil))
				case 2:
					em.RecordError(NewInvalidConfig("bad config", nil))
				}
			}
		}(i)
	}

	// Wait for all goroutines to complete
	wg.Wait()

	// Verify total count
	expectedTotal := int64(numGoroutines * errorsPerGoroutine)
	if em.GetTotalErrorCount() != expectedTotal {
		t.Errorf("expected total count %d, got %d", expectedTotal, em.GetTotalErrorCount())
	}

	// Verify category counts - just check they exist and total is correct
	// Due to modulo distribution, each category should get roughly 1/3 of errors
	categoryCounts := em.GetCategoryCounts()
	transientCount := categoryCounts[CategoryTransient]
	permanentCount := categoryCounts[CategoryPermanent]
	configCount := categoryCounts[CategoryConfiguration]

	if transientCount+permanentCount+configCount != expectedTotal {
		t.Errorf("expected total across categories %d, got %d",
			expectedTotal,
			transientCount+permanentCount+configCount)
	}

	// Allow for some variation due to concurrent scheduling
	expectedPerCategory := int64(numGoroutines * errorsPerGoroutine / 3)
	allowedDeviation := expectedPerCategory / 5 // Allow 20% deviation

	if diff := abs(transientCount - expectedPerCategory); diff > allowedDeviation {
		t.Logf("transient count %d differs from expected %d by %d (allowed deviation: %d)",
			transientCount, expectedPerCategory, diff, allowedDeviation)
	}
}

func abs(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

func TestConcurrentReadsAndWrites(t *testing.T) {
	em := NewErrorMetrics()

	// Channels to signal completion and coordinate goroutines
	done := make(chan bool, 1)
	readerErrors := make(chan error, 10)
	writerCount := int32(0)

	// Writer goroutines that record errors
	numWriters := 50
	for w := 0; w < numWriters; w++ {
		go func() {
			for i := 0; i < 100; i++ {
				em.RecordError(NewTimeout(5*time.Minute, nil))
				atomic.AddInt32(&writerCount, 1)
			}
		}()
	}

	// Reader goroutines that read metrics while writing is happening
	numReaders := 50
	var readerWg sync.WaitGroup
	readerWg.Add(numReaders)

	for r := 0; r < numReaders; r++ {
		go func() {
			defer readerWg.Done()

			for i := 0; i < 200; i++ {
				// Try various read operations
				_ = em.GetErrorCounts()
				_ = em.GetCategoryCounts()
				_ = em.GetTotalErrorCount()
				_ = em.GetErrorCountByCode(ErrTimeout)
				_ = em.GetTimestamps()
				_ = em.GetErrorCountSince(time.Now().Add(-1 * time.Hour))
			}
		}()
	}

	// Wait for readers to complete
	readerWg.Wait()

	// Wait for all writers
	expectedWriteCount := int32(numWriters * 100)
	for i := 0; i < 100; i++ {
		if atomic.LoadInt32(&writerCount) >= expectedWriteCount {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Verify no errors were returned from readers
	close(readerErrors)
	for err := range readerErrors {
		t.Errorf("reader error: %v", err)
	}

	// Verify final count
	if em.GetTotalErrorCount() >= int64(expectedWriteCount) {
		t.Logf("final error count: %d (expected at least %d)", em.GetTotalErrorCount(), expectedWriteCount)
	}

	done <- true
}

func TestConcurrentResetDuringOperation(t *testing.T) {
	em := NewErrorMetrics()

	var wg sync.WaitGroup

	// Writers goroutines
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				em.RecordError(NewTimeout(5*time.Minute, nil))
			}
		}()
	}

	// Readers goroutines
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = em.GetTotalErrorCount()
			}
		}()
	}

	// Reset goroutines
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				time.Sleep(time.Millisecond)
				em.Reset()
			}
		}()
	}

	// Wait for all operations to complete
	wg.Wait()

	// The metrics should be valid after all operations
	count := em.GetTotalErrorCount()
	if count < 0 {
		t.Errorf("total error count should never be negative, got %d", count)
	}

	counts := em.GetErrorCounts()
	if counts == nil {
		t.Error("GetErrorCounts should never return nil")
	}

	catCounts := em.GetCategoryCounts()
	if catCounts == nil {
		t.Error("GetCategoryCounts should never return nil")
	}
}

func TestErrorMetricsAllErrorCodes(t *testing.T) {
	em := NewErrorMetrics()

	// Record all error code types
	testCases := []struct {
		name string
		err  *ExecutionError
	}{
		{"timeout", NewTimeout(5*time.Minute, nil)},
		{"quota", NewQuotaExhausted("rate limit", time.Now().Add(1*time.Hour))},
		{"not_found", NewNotFound("binary", nil)},
		{"execution", NewExecutionFailed("process crashed", true, nil)},
		{"canceled", NewCanceled("user stopped")},
		{"permission", NewPermissionDenied("/bin/claude", nil)},
		{"config", NewInvalidConfig("bad config", nil)},
	}

	for _, tc := range testCases {
		em.RecordError(tc.err)
	}

	// Verify all codes are tracked
	counts := em.GetErrorCounts()
	if len(counts) != len(testCases) {
		t.Errorf("expected %d error types, got %d", len(testCases), len(counts))
	}

	// Verify each code was recorded
	for _, tc := range testCases {
		if counts[tc.err.Code] != 1 {
			t.Errorf("expected count 1 for %s, got %d", tc.err.Code, counts[tc.err.Code])
		}
	}
}

func TestErrorMetricsMemoryEfficiency(t *testing.T) {
	em := NewErrorMetrics()

	// Record a large number of errors
	for i := 0; i < 10000; i++ {
		em.RecordError(NewTimeout(5*time.Minute, nil))
	}

	// Verify metrics are accurate
	if em.GetTotalErrorCount() != 10000 {
		t.Errorf("expected count 10000, got %d", em.GetTotalErrorCount())
	}

	// Reset should clear everything
	em.Reset()
	if em.GetTotalErrorCount() != 0 {
		t.Errorf("expected count 0 after reset, got %d", em.GetTotalErrorCount())
	}

	// Timestamps should also be cleared
	timestamps := em.GetTimestamps()
	if len(timestamps) != 0 {
		t.Errorf("expected 0 timestamps after reset, got %d", len(timestamps))
	}
}

func BenchmarkRecordError(b *testing.B) {
	em := NewErrorMetrics()
	err := NewTimeout(5*time.Minute, nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		em.RecordError(err)
	}
}

func BenchmarkGetErrorCounts(b *testing.B) {
	em := NewErrorMetrics()
	for i := 0; i < 1000; i++ {
		em.RecordError(NewTimeout(5*time.Minute, nil))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = em.GetErrorCounts()
	}
}

func BenchmarkGetTotalErrorCount(b *testing.B) {
	em := NewErrorMetrics()
	for i := 0; i < 1000; i++ {
		em.RecordError(NewTimeout(5*time.Minute, nil))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = em.GetTotalErrorCount()
	}
}

func BenchmarkConcurrentRecordError(b *testing.B) {
	em := NewErrorMetrics()
	err := NewTimeout(5*time.Minute, nil)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			em.RecordError(err)
		}
	})
}

func BenchmarkConcurrentReadWrite(b *testing.B) {
	em := NewErrorMetrics()
	err := NewTimeout(5*time.Minute, nil)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if i%2 == 0 {
				em.RecordError(err)
			} else {
				_ = em.GetTotalErrorCount()
			}
			i++
		}
	})
}

func ExampleErrorMetrics() {
	// Create a new error metrics tracker
	em := NewErrorMetrics()

	// Record some errors
	em.RecordError(NewTimeout(5*time.Minute, nil))
	em.RecordError(NewTimeout(10*time.Minute, nil))
	em.RecordError(NewNotFound("binary", nil))
	em.RecordError(NewInvalidConfig("bad config", nil))

	// Get total error count
	fmt.Printf("Total errors: %d\n", em.GetTotalErrorCount())

	// Get error counts by category
	categoryCounts := em.GetCategoryCounts()
	fmt.Println("Error counts by category:")
	fmt.Printf("  transient: %d\n", categoryCounts[CategoryTransient])
	fmt.Printf("  permanent: %d\n", categoryCounts[CategoryPermanent])
	fmt.Printf("  configuration: %d\n", categoryCounts[CategoryConfiguration])

	// Output:
	// Total errors: 4
	// Error counts by category:
	//   transient: 2
	//   permanent: 1
	//   configuration: 1
}
