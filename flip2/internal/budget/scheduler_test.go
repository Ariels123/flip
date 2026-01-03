package budget

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestSchedulerStartStop(t *testing.T) {
	store := NewMockStore()
	tracker := New(store, nil)
	scheduler := NewScheduler(tracker, nil, 100*time.Millisecond)

	ctx := context.Background()

	// Start scheduler
	err := scheduler.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}

	if !scheduler.IsRunning() {
		t.Error("Scheduler should be running after Start()")
	}

	// Starting again should be a no-op
	err = scheduler.Start(ctx)
	if err != nil {
		t.Fatalf("Second start should not error: %v", err)
	}

	// Stop scheduler
	scheduler.Stop()

	if scheduler.IsRunning() {
		t.Error("Scheduler should not be running after Stop()")
	}

	// Stopping again should be a no-op
	scheduler.Stop()
}

func TestSchedulerPeriodicCheck(t *testing.T) {
	store := NewMockStore()
	tracker := New(store, nil)
	ctx := context.Background()

	// Create a budget that needs reset
	budget, _ := tracker.CreateBudget(ctx, "Test Budget", ScopeGlobal, "global", 100.0, ResetDaily)

	// Set the budget to need reset
	store.mu.Lock()
	store.budgets[budget.ID].NextResetAt = time.Now().Add(-1 * time.Hour)
	store.budgets[budget.ID].ConsumedUSD = 50.0
	store.mu.Unlock()

	// Create scheduler with short interval
	scheduler := NewScheduler(tracker, nil, 50*time.Millisecond)

	// Start scheduler
	scheduler.Start(ctx)
	defer scheduler.Stop()

	// Wait for at least one check cycle
	time.Sleep(100 * time.Millisecond)

	// Verify budget was reset
	updated, _ := tracker.GetBudget(ctx, budget.ID)
	if updated.ConsumedUSD != 0 {
		t.Errorf("Expected budget to be reset, consumed is %f", updated.ConsumedUSD)
	}
}

func TestSchedulerRunOnce(t *testing.T) {
	store := NewMockStore()
	tracker := New(store, nil)
	scheduler := NewScheduler(tracker, nil, time.Hour)
	ctx := context.Background()

	// Create a budget that needs reset
	budget, _ := tracker.CreateBudget(ctx, "Test Budget", ScopeGlobal, "global", 100.0, ResetDaily)

	// Set the budget to need reset
	store.mu.Lock()
	store.budgets[budget.ID].NextResetAt = time.Now().Add(-1 * time.Hour)
	store.budgets[budget.ID].ConsumedUSD = 75.0
	store.mu.Unlock()

	// Run once without starting the scheduler
	err := scheduler.RunOnce(ctx)
	if err != nil {
		t.Fatalf("RunOnce failed: %v", err)
	}

	// Scheduler should not be running
	if scheduler.IsRunning() {
		t.Error("Scheduler should not be running after RunOnce()")
	}

	// Budget should be reset
	updated, _ := tracker.GetBudget(ctx, budget.ID)
	if updated.ConsumedUSD != 0 {
		t.Errorf("Expected budget to be reset after RunOnce, consumed is %f", updated.ConsumedUSD)
	}
}

func TestSchedulerContextCancellation(t *testing.T) {
	store := NewMockStore()
	tracker := New(store, nil)
	scheduler := NewScheduler(tracker, nil, 50*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())

	// Start scheduler
	scheduler.Start(ctx)

	// Verify it's running
	if !scheduler.IsRunning() {
		t.Fatal("Scheduler should be running")
	}

	// Cancel context
	cancel()

	// Give it time to stop
	time.Sleep(100 * time.Millisecond)

	// Clean up
	scheduler.Stop()
}

func TestAlertCallback(t *testing.T) {
	store := NewMockStore()
	tracker := New(store, nil)
	ctx := context.Background()

	var alertsMu sync.Mutex
	alerts := make([]struct {
		budget    *Budget
		alertType string
	}, 0)

	// Set up alert callback
	tracker.SetAlertCallback(func(budget *Budget, alertType string) {
		alertsMu.Lock()
		defer alertsMu.Unlock()
		alerts = append(alerts, struct {
			budget    *Budget
			alertType string
		}{budget, alertType})
	})

	// Create budget
	tracker.CreateBudget(ctx, "Test Budget", ScopeGlobal, "global", 100.0, ResetDaily)

	// Trigger warning (80% threshold)
	tracker.RecordSpend(ctx, "agent-1", "model-1", "", 85.0, 1000, 500)

	// Wait for async callback
	time.Sleep(50 * time.Millisecond)

	alertsMu.Lock()
	if len(alerts) != 1 {
		t.Errorf("Expected 1 alert, got %d", len(alerts))
	} else if alerts[0].alertType != AlertTypeBudgetWarning {
		t.Errorf("Expected warning alert, got %s", alerts[0].alertType)
	}
	alertsMu.Unlock()

	// Trigger exhausted
	tracker.RecordSpend(ctx, "agent-1", "model-1", "", 20.0, 1000, 500)

	// Wait for async callback
	time.Sleep(50 * time.Millisecond)

	alertsMu.Lock()
	if len(alerts) != 2 {
		t.Errorf("Expected 2 alerts, got %d", len(alerts))
	} else if alerts[1].alertType != AlertTypeBudgetExhausted {
		t.Errorf("Expected exhausted alert, got %s", alerts[1].alertType)
	}
	alertsMu.Unlock()
}

func TestAlertCallbackNotSetDoesNotPanic(t *testing.T) {
	store := NewMockStore()
	tracker := New(store, nil)
	ctx := context.Background()

	// Create budget without setting callback
	tracker.CreateBudget(ctx, "Test Budget", ScopeGlobal, "global", 100.0, ResetDaily)

	// This should not panic even though no callback is set
	tracker.RecordSpend(ctx, "agent-1", "model-1", "", 85.0, 1000, 500)

	// Wait to ensure no async panic
	time.Sleep(50 * time.Millisecond)
}

func TestSchedulerDefaultInterval(t *testing.T) {
	store := NewMockStore()
	tracker := New(store, nil)

	// Create with zero interval - should use default
	scheduler := NewScheduler(tracker, nil, 0)

	// We can't easily check the interval, but we can verify it doesn't panic
	ctx := context.Background()
	scheduler.Start(ctx)
	time.Sleep(10 * time.Millisecond)
	scheduler.Stop()
}
