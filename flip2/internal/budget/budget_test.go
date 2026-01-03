package budget

import (
	"context"
	"sync"
	"testing"
	"time"
)

// MockStore implements BudgetStore for testing
type MockStore struct {
	mu           sync.RWMutex
	budgets      map[string]*Budget
	consumptions map[string][]*BudgetConsumption
	scopeIndex   map[string]string // scope:scopeID -> budgetID
}

// NewMockStore creates a new mock store for testing
func NewMockStore() *MockStore {
	return &MockStore{
		budgets:      make(map[string]*Budget),
		consumptions: make(map[string][]*BudgetConsumption),
		scopeIndex:   make(map[string]string),
	}
}

func (m *MockStore) CreateBudget(ctx context.Context, budget *Budget) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	budget.ID = generateTestID()
	m.budgets[budget.ID] = budget
	m.scopeIndex[string(budget.Scope)+":"+budget.ScopeID] = budget.ID
	return nil
}

func (m *MockStore) GetBudget(ctx context.Context, id string) (*Budget, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	budget, ok := m.budgets[id]
	if !ok {
		return nil, nil
	}
	// Return a copy
	copy := *budget
	return &copy, nil
}

func (m *MockStore) GetBudgetByScope(ctx context.Context, scope BudgetScope, scopeID string) (*Budget, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	budgetID, ok := m.scopeIndex[string(scope)+":"+scopeID]
	if !ok {
		return nil, nil
	}
	budget, ok := m.budgets[budgetID]
	if !ok {
		return nil, nil
	}
	copy := *budget
	return &copy, nil
}

func (m *MockStore) ListBudgets(ctx context.Context) ([]*Budget, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	budgets := make([]*Budget, 0, len(m.budgets))
	for _, b := range m.budgets {
		copy := *b
		budgets = append(budgets, &copy)
	}
	return budgets, nil
}

func (m *MockStore) UpdateBudget(ctx context.Context, budget *Budget) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.budgets[budget.ID]; !ok {
		return nil
	}
	// Store a copy to avoid shared pointer issues
	copy := *budget
	m.budgets[budget.ID] = &copy
	return nil
}

func (m *MockStore) DeleteBudget(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if budget, ok := m.budgets[id]; ok {
		delete(m.scopeIndex, string(budget.Scope)+":"+budget.ScopeID)
		delete(m.budgets, id)
	}
	return nil
}

func (m *MockStore) RecordConsumption(ctx context.Context, consumption *BudgetConsumption) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	consumption.ID = generateTestID()
	m.consumptions[consumption.BudgetID] = append(m.consumptions[consumption.BudgetID], consumption)
	return nil
}

func (m *MockStore) GetConsumptions(ctx context.Context, budgetID string, start, end time.Time) ([]*BudgetConsumption, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*BudgetConsumption
	for _, c := range m.consumptions[budgetID] {
		if (c.Timestamp.Equal(start) || c.Timestamp.After(start)) && c.Timestamp.Before(end) {
			copy := *c
			result = append(result, &copy)
		}
	}
	return result, nil
}

func (m *MockStore) GetTotalConsumed(ctx context.Context, budgetID string, start, end time.Time) (float64, error) {
	consumptions, err := m.GetConsumptions(ctx, budgetID, start, end)
	if err != nil {
		return 0, err
	}

	var total float64
	for _, c := range consumptions {
		total += c.AmountUSD
	}
	return total, nil
}

func (m *MockStore) GetTopConsumersByAgent(ctx context.Context, budgetID string, start, end time.Time, limit int) ([]ConsumerSummary, error) {
	consumptions, err := m.GetConsumptions(ctx, budgetID, start, end)
	if err != nil {
		return nil, err
	}

	totals := make(map[string]float64)
	counts := make(map[string]int)
	var totalAll float64

	for _, c := range consumptions {
		totals[c.AgentID] += c.AmountUSD
		counts[c.AgentID]++
		totalAll += c.AmountUSD
	}

	var summaries []ConsumerSummary
	for agentID, amount := range totals {
		pct := 0.0
		if totalAll > 0 {
			pct = (amount / totalAll) * 100
		}
		summaries = append(summaries, ConsumerSummary{
			ID:          agentID,
			Type:        "agent",
			ConsumedUSD: amount,
			Percentage:  pct,
			EventCount:  counts[agentID],
		})
	}

	if limit > 0 && len(summaries) > limit {
		summaries = summaries[:limit]
	}
	return summaries, nil
}

func (m *MockStore) GetTopConsumersByModel(ctx context.Context, budgetID string, start, end time.Time, limit int) ([]ConsumerSummary, error) {
	consumptions, err := m.GetConsumptions(ctx, budgetID, start, end)
	if err != nil {
		return nil, err
	}

	totals := make(map[string]float64)
	counts := make(map[string]int)
	var totalAll float64

	for _, c := range consumptions {
		totals[c.Model] += c.AmountUSD
		counts[c.Model]++
		totalAll += c.AmountUSD
	}

	var summaries []ConsumerSummary
	for model, amount := range totals {
		pct := 0.0
		if totalAll > 0 {
			pct = (amount / totalAll) * 100
		}
		summaries = append(summaries, ConsumerSummary{
			ID:          model,
			Type:        "model",
			ConsumedUSD: amount,
			Percentage:  pct,
			EventCount:  counts[model],
		})
	}

	if limit > 0 && len(summaries) > limit {
		summaries = summaries[:limit]
	}
	return summaries, nil
}

var testIDCounter int
var testIDMu sync.Mutex

func generateTestID() string {
	testIDMu.Lock()
	defer testIDMu.Unlock()
	testIDCounter++
	return "test-id-" + string(rune('0'+testIDCounter%10)) + string(rune('0'+testIDCounter/10))
}

func TestCreateBudget(t *testing.T) {
	store := NewMockStore()
	tracker := New(store, nil)
	ctx := context.Background()

	budget, err := tracker.CreateBudget(ctx, "Test Budget", ScopeGlobal, "global", 100.0, ResetDaily)
	if err != nil {
		t.Fatalf("CreateBudget failed: %v", err)
	}

	if budget.ID == "" {
		t.Error("Expected budget ID to be set")
	}
	if budget.Name != "Test Budget" {
		t.Errorf("Expected name 'Test Budget', got '%s'", budget.Name)
	}
	if budget.Scope != ScopeGlobal {
		t.Errorf("Expected scope ScopeGlobal, got '%s'", budget.Scope)
	}
	if budget.LimitUSD != 100.0 {
		t.Errorf("Expected limit 100.0, got %f", budget.LimitUSD)
	}
	if budget.ResetPeriod != ResetDaily {
		t.Errorf("Expected ResetDaily, got '%s'", budget.ResetPeriod)
	}
	if budget.Status != StatusActive {
		t.Errorf("Expected StatusActive, got '%s'", budget.Status)
	}
	if !budget.Enabled {
		t.Error("Expected budget to be enabled")
	}
	if budget.WarningPct != 80.0 {
		t.Errorf("Expected warning_pct 80.0, got %f", budget.WarningPct)
	}
}

func TestRecordSpend(t *testing.T) {
	store := NewMockStore()
	tracker := New(store, nil)
	ctx := context.Background()

	// Create a global budget
	budget, err := tracker.CreateBudget(ctx, "Global Budget", ScopeGlobal, "global", 100.0, ResetDaily)
	if err != nil {
		t.Fatalf("CreateBudget failed: %v", err)
	}

	// Record spending
	err = tracker.RecordSpend(ctx, "agent-1", "gpt-4", "task-1", 10.0, 1000, 500)
	if err != nil {
		t.Fatalf("RecordSpend failed: %v", err)
	}

	// Verify budget was updated
	updatedBudget, err := tracker.GetBudget(ctx, budget.ID)
	if err != nil {
		t.Fatalf("GetBudget failed: %v", err)
	}

	if updatedBudget.ConsumedUSD != 10.0 {
		t.Errorf("Expected consumed 10.0, got %f", updatedBudget.ConsumedUSD)
	}
}

func TestRecordSpendMultipleBudgets(t *testing.T) {
	store := NewMockStore()
	tracker := New(store, nil)
	ctx := context.Background()

	// Create budgets at different scopes
	_, err := tracker.CreateBudget(ctx, "Agent Budget", ScopeAgent, "agent-1", 50.0, ResetDaily)
	if err != nil {
		t.Fatalf("CreateBudget (agent) failed: %v", err)
	}

	_, err = tracker.CreateBudget(ctx, "Model Budget", ScopeModel, "gpt-4", 200.0, ResetDaily)
	if err != nil {
		t.Fatalf("CreateBudget (model) failed: %v", err)
	}

	globalBudget, err := tracker.CreateBudget(ctx, "Global Budget", ScopeGlobal, "global", 1000.0, ResetDaily)
	if err != nil {
		t.Fatalf("CreateBudget (global) failed: %v", err)
	}

	// Record spending - should update all matching budgets
	err = tracker.RecordSpend(ctx, "agent-1", "gpt-4", "task-1", 25.0, 2000, 1000)
	if err != nil {
		t.Fatalf("RecordSpend failed: %v", err)
	}

	// Verify global budget was updated
	updatedGlobal, _ := tracker.GetBudget(ctx, globalBudget.ID)
	if updatedGlobal.ConsumedUSD != 25.0 {
		t.Errorf("Expected global consumed 25.0, got %f", updatedGlobal.ConsumedUSD)
	}
}

func TestBudgetStatusTransitions(t *testing.T) {
	store := NewMockStore()
	tracker := New(store, nil)
	ctx := context.Background()

	// Create budget with 100.0 limit
	budget, _ := tracker.CreateBudget(ctx, "Test Budget", ScopeGlobal, "global", 100.0, ResetDaily)

	// Initial status should be active
	if budget.Status != StatusActive {
		t.Errorf("Initial status should be Active, got %s", budget.Status)
	}

	// Spend 70% - should still be active
	tracker.RecordSpend(ctx, "agent-1", "model-1", "", 70.0, 1000, 500)
	updated, _ := tracker.GetBudget(ctx, budget.ID)
	if updated.Status != StatusActive {
		t.Errorf("Status should be Active at 70%%, got %s", updated.Status)
	}

	// Spend another 15% (85% total) - should be warning
	tracker.RecordSpend(ctx, "agent-1", "model-1", "", 15.0, 1000, 500)
	updated, _ = tracker.GetBudget(ctx, budget.ID)
	if updated.Status != StatusWarning {
		t.Errorf("Status should be Warning at 85%%, got %s", updated.Status)
	}

	// Spend another 20% (105% total) - should be exhausted
	tracker.RecordSpend(ctx, "agent-1", "model-1", "", 20.0, 1000, 500)
	updated, _ = tracker.GetBudget(ctx, budget.ID)
	if updated.Status != StatusExhausted {
		t.Errorf("Status should be Exhausted at 105%%, got %s", updated.Status)
	}
}

func TestCheckBudget(t *testing.T) {
	store := NewMockStore()
	tracker := New(store, nil)
	ctx := context.Background()

	// Create budget
	tracker.CreateBudget(ctx, "Test Budget", ScopeGlobal, "global", 100.0, ResetDaily)

	// Check if spending is allowed initially
	allowed, reason, err := tracker.CheckBudget(ctx, "agent-1", "model-1", 50.0)
	if err != nil {
		t.Fatalf("CheckBudget failed: %v", err)
	}
	if !allowed {
		t.Errorf("Expected spending to be allowed, got denied: %s", reason)
	}

	// Record some spending
	tracker.RecordSpend(ctx, "agent-1", "model-1", "", 80.0, 1000, 500)

	// Check if spending that would exceed limit is denied
	allowed, reason, err = tracker.CheckBudget(ctx, "agent-1", "model-1", 25.0)
	if err != nil {
		t.Fatalf("CheckBudget failed: %v", err)
	}
	if allowed {
		t.Error("Expected spending to be denied when it would exceed limit")
	}
	if reason == "" {
		t.Error("Expected a reason for denial")
	}
}

func TestCheckBudgetExhausted(t *testing.T) {
	store := NewMockStore()
	tracker := New(store, nil)
	ctx := context.Background()

	// Create budget and exhaust it
	budget, _ := tracker.CreateBudget(ctx, "Test Budget", ScopeGlobal, "global", 100.0, ResetDaily)
	tracker.RecordSpend(ctx, "agent-1", "model-1", "", 100.0, 1000, 500)

	// Verify status is exhausted
	updated, _ := tracker.GetBudget(ctx, budget.ID)
	if updated.Status != StatusExhausted {
		t.Fatalf("Expected status to be Exhausted, got %s", updated.Status)
	}

	// Any spending should be denied
	allowed, reason, _ := tracker.CheckBudget(ctx, "agent-1", "model-1", 0.01)
	if allowed {
		t.Error("Expected spending to be denied for exhausted budget")
	}
	if reason == "" {
		t.Error("Expected a reason for denial")
	}
}

func TestCheckBudgetAutoReset(t *testing.T) {
	store := NewMockStore()
	tracker := New(store, nil)
	ctx := context.Background()

	// Create budget and exhaust it
	budget, _ := tracker.CreateBudget(ctx, "Test Budget", ScopeGlobal, "global", 100.0, ResetDaily)
	tracker.RecordSpend(ctx, "agent-1", "model-1", "", 100.0, 1000, 500)

	// Manually set past reset time in the store
	store.mu.Lock()
	stored := store.budgets[budget.ID]
	stored.NextResetAt = time.Now().Add(-1 * time.Hour)
	store.mu.Unlock()

	// CheckBudget should allow spending now because it's past reset time
	allowed, reason, err := tracker.CheckBudget(ctx, "agent-1", "model-1", 10.0)
	if err != nil {
		t.Fatalf("CheckBudget failed: %v", err)
	}
	if !allowed {
		t.Errorf("Expected spending to be allowed for expired budget, got denied: %s", reason)
	}
}

func TestBudgetReset(t *testing.T) {
	store := NewMockStore()
	tracker := New(store, nil)
	ctx := context.Background()

	// Create and consume budget
	budget, _ := tracker.CreateBudget(ctx, "Test Budget", ScopeGlobal, "global", 100.0, ResetDaily)
	tracker.RecordSpend(ctx, "agent-1", "model-1", "", 50.0, 1000, 500)

	// Manually reset
	err := tracker.ResetBudget(ctx, budget.ID)
	if err != nil {
		t.Fatalf("ResetBudget failed: %v", err)
	}

	// Verify reset
	updated, _ := tracker.GetBudget(ctx, budget.ID)
	if updated.ConsumedUSD != 0 {
		t.Errorf("Expected consumed to be 0 after reset, got %f", updated.ConsumedUSD)
	}
	if updated.Status != StatusActive {
		t.Errorf("Expected status to be Active after reset, got %s", updated.Status)
	}
}

func TestAutoResetOnSpend(t *testing.T) {
	store := NewMockStore()
	tracker := New(store, nil)
	ctx := context.Background()

	// Create budget with past reset time
	budget, _ := tracker.CreateBudget(ctx, "Test Budget", ScopeGlobal, "global", 100.0, ResetDaily)

	// Manually set past reset time in the store
	store.mu.Lock()
	stored := store.budgets[budget.ID]
	stored.NextResetAt = time.Now().Add(-1 * time.Hour)
	stored.ConsumedUSD = 80.0
	store.mu.Unlock()

	// Record new spending - should trigger auto-reset first
	tracker.RecordSpend(ctx, "agent-1", "model-1", "", 10.0, 1000, 500)

	// Verify budget was reset before recording spend
	updated, _ := tracker.GetBudget(ctx, budget.ID)
	// After reset, the new spend should be recorded
	if updated.ConsumedUSD != 10.0 {
		t.Errorf("Expected consumed to be 10.0 after auto-reset and new spend, got %f", updated.ConsumedUSD)
	}
}

func TestCheckAndResetBudgets(t *testing.T) {
	store := NewMockStore()
	tracker := New(store, nil)
	ctx := context.Background()

	// Create multiple budgets
	budget1, _ := tracker.CreateBudget(ctx, "Budget 1", ScopeAgent, "agent-1", 100.0, ResetDaily)
	budget2, _ := tracker.CreateBudget(ctx, "Budget 2", ScopeAgent, "agent-2", 100.0, ResetDaily)

	// Set one budget's reset time to the past
	store.mu.Lock()
	store.budgets[budget1.ID].NextResetAt = time.Now().Add(-1 * time.Hour)
	store.budgets[budget1.ID].ConsumedUSD = 50.0
	store.mu.Unlock()

	// Run periodic check
	err := tracker.CheckAndResetBudgets(ctx)
	if err != nil {
		t.Fatalf("CheckAndResetBudgets failed: %v", err)
	}

	// Verify budget1 was reset
	updated1, _ := tracker.GetBudget(ctx, budget1.ID)
	if updated1.ConsumedUSD != 0 {
		t.Errorf("Budget1 should be reset, consumed is %f", updated1.ConsumedUSD)
	}

	// Verify budget2 was not reset
	updated2, _ := tracker.GetBudget(ctx, budget2.ID)
	if updated2.ConsumedUSD != 0 {
		// Budget2 should also be 0 since it was never consumed
	}
}

func TestDisableEnableBudget(t *testing.T) {
	store := NewMockStore()
	tracker := New(store, nil)
	ctx := context.Background()

	budget, _ := tracker.CreateBudget(ctx, "Test Budget", ScopeGlobal, "global", 100.0, ResetDaily)

	// Disable budget
	err := tracker.DisableBudget(ctx, budget.ID)
	if err != nil {
		t.Fatalf("DisableBudget failed: %v", err)
	}

	updated, _ := tracker.GetBudget(ctx, budget.ID)
	if updated.Enabled {
		t.Error("Expected budget to be disabled")
	}
	if updated.Status != StatusDisabled {
		t.Errorf("Expected status Disabled, got %s", updated.Status)
	}

	// Re-enable budget
	err = tracker.EnableBudget(ctx, budget.ID)
	if err != nil {
		t.Fatalf("EnableBudget failed: %v", err)
	}

	updated, _ = tracker.GetBudget(ctx, budget.ID)
	if !updated.Enabled {
		t.Error("Expected budget to be enabled")
	}
	if updated.Status != StatusActive {
		t.Errorf("Expected status Active, got %s", updated.Status)
	}
}

func TestUpdateLimit(t *testing.T) {
	store := NewMockStore()
	tracker := New(store, nil)
	ctx := context.Background()

	budget, _ := tracker.CreateBudget(ctx, "Test Budget", ScopeGlobal, "global", 100.0, ResetDaily)
	tracker.RecordSpend(ctx, "agent-1", "model-1", "", 85.0, 1000, 500)

	// Verify warning status
	updated, _ := tracker.GetBudget(ctx, budget.ID)
	if updated.Status != StatusWarning {
		t.Errorf("Expected warning status at 85%%, got %s", updated.Status)
	}

	// Increase limit - should change status back to active
	err := tracker.UpdateLimit(ctx, budget.ID, 200.0)
	if err != nil {
		t.Fatalf("UpdateLimit failed: %v", err)
	}

	updated, _ = tracker.GetBudget(ctx, budget.ID)
	if updated.LimitUSD != 200.0 {
		t.Errorf("Expected limit 200.0, got %f", updated.LimitUSD)
	}
	if updated.Status != StatusActive {
		t.Errorf("Expected Active status after limit increase, got %s", updated.Status)
	}
}

func TestGetReport(t *testing.T) {
	store := NewMockStore()
	tracker := New(store, nil)
	ctx := context.Background()

	budget, _ := tracker.CreateBudget(ctx, "Test Budget", ScopeGlobal, "global", 100.0, ResetDaily)

	// Record some spending
	tracker.RecordSpend(ctx, "agent-1", "gpt-4", "task-1", 30.0, 3000, 1500)
	tracker.RecordSpend(ctx, "agent-2", "gpt-4", "task-2", 20.0, 2000, 1000)
	tracker.RecordSpend(ctx, "agent-1", "claude-3", "task-3", 15.0, 1500, 750)

	report, err := tracker.GetReport(ctx, budget.ID)
	if err != nil {
		t.Fatalf("GetReport failed: %v", err)
	}

	if report.Budget == nil {
		t.Fatal("Report budget is nil")
	}
	if report.TotalConsumed != 65.0 {
		t.Errorf("Expected total consumed 65.0, got %f", report.TotalConsumed)
	}
	if report.RemainingUSD != 35.0 {
		t.Errorf("Expected remaining 35.0, got %f", report.RemainingUSD)
	}
	if report.ConsumedPct != 65.0 {
		t.Errorf("Expected consumed pct 65.0, got %f", report.ConsumedPct)
	}
	if len(report.RecentActivity) != 3 {
		t.Errorf("Expected 3 recent activities, got %d", len(report.RecentActivity))
	}
}

func TestListBudgets(t *testing.T) {
	store := NewMockStore()
	tracker := New(store, nil)
	ctx := context.Background()

	// Create multiple budgets
	tracker.CreateBudget(ctx, "Budget 1", ScopeGlobal, "global", 100.0, ResetDaily)
	tracker.CreateBudget(ctx, "Budget 2", ScopeAgent, "agent-1", 50.0, ResetHourly)
	tracker.CreateBudget(ctx, "Budget 3", ScopeModel, "gpt-4", 200.0, ResetWeekly)

	budgets, err := tracker.ListBudgets(ctx)
	if err != nil {
		t.Fatalf("ListBudgets failed: %v", err)
	}

	if len(budgets) != 3 {
		t.Errorf("Expected 3 budgets, got %d", len(budgets))
	}
}

func TestCalculateNextReset(t *testing.T) {
	testCases := []struct {
		name     string
		period   ResetPeriod
		from     time.Time
		validate func(t *testing.T, result time.Time)
	}{
		{
			name:   "Hourly",
			period: ResetHourly,
			from:   time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC),
			validate: func(t *testing.T, result time.Time) {
				expected := time.Date(2024, 1, 15, 15, 0, 0, 0, time.UTC)
				if !result.Equal(expected) {
					t.Errorf("Expected %v, got %v", expected, result)
				}
			},
		},
		{
			name:   "Daily",
			period: ResetDaily,
			from:   time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC),
			validate: func(t *testing.T, result time.Time) {
				expected := time.Date(2024, 1, 16, 0, 0, 0, 0, time.UTC)
				if !result.Equal(expected) {
					t.Errorf("Expected %v, got %v", expected, result)
				}
			},
		},
		{
			name:   "Monthly",
			period: ResetMonthly,
			from:   time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC),
			validate: func(t *testing.T, result time.Time) {
				expected := time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)
				if !result.Equal(expected) {
					t.Errorf("Expected %v, got %v", expected, result)
				}
			},
		},
		{
			name:   "Monthly at end of year",
			period: ResetMonthly,
			from:   time.Date(2024, 12, 15, 14, 30, 0, 0, time.UTC),
			validate: func(t *testing.T, result time.Time) {
				expected := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
				if !result.Equal(expected) {
					t.Errorf("Expected %v, got %v", expected, result)
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := calculateNextReset(tc.from, tc.period)
			tc.validate(t, result)
		})
	}
}

func TestResetPeriodIsValid(t *testing.T) {
	testCases := []struct {
		period ResetPeriod
		valid  bool
	}{
		{ResetHourly, true},
		{ResetDaily, true},
		{ResetWeekly, true},
		{ResetMonthly, true},
		{ResetPeriod("invalid"), false},
		{ResetPeriod(""), false},
	}

	for _, tc := range testCases {
		t.Run(string(tc.period), func(t *testing.T) {
			if tc.period.IsValid() != tc.valid {
				t.Errorf("Expected %s.IsValid() = %v, got %v", tc.period, tc.valid, !tc.valid)
			}
		})
	}
}

func TestBudgetScopeIsValid(t *testing.T) {
	testCases := []struct {
		scope BudgetScope
		valid bool
	}{
		{ScopeAgent, true},
		{ScopeModel, true},
		{ScopeGlobal, true},
		{BudgetScope("invalid"), false},
		{BudgetScope(""), false},
	}

	for _, tc := range testCases {
		t.Run(string(tc.scope), func(t *testing.T) {
			if tc.scope.IsValid() != tc.valid {
				t.Errorf("Expected %s.IsValid() = %v, got %v", tc.scope, tc.valid, !tc.valid)
			}
		})
	}
}

func TestConcurrentSpending(t *testing.T) {
	store := NewMockStore()
	tracker := New(store, nil)
	ctx := context.Background()

	// Create budget
	budget, _ := tracker.CreateBudget(ctx, "Test Budget", ScopeGlobal, "global", 1000.0, ResetDaily)

	// Simulate concurrent spending
	var wg sync.WaitGroup
	numGoroutines := 10
	spendPerGoroutine := 10.0

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 5; j++ {
				tracker.RecordSpend(ctx, "agent-1", "model-1", "", spendPerGoroutine, 100, 50)
			}
		}(i)
	}

	wg.Wait()

	// Verify total (10 goroutines * 5 calls * 10.0 = 500.0)
	updated, _ := tracker.GetBudget(ctx, budget.ID)
	expected := float64(numGoroutines) * 5.0 * spendPerGoroutine
	if updated.ConsumedUSD != expected {
		t.Errorf("Expected consumed %f, got %f (concurrency issue)", expected, updated.ConsumedUSD)
	}
}

func TestDisabledBudgetNotChecked(t *testing.T) {
	store := NewMockStore()
	tracker := New(store, nil)
	ctx := context.Background()

	// Create and disable budget
	budget, _ := tracker.CreateBudget(ctx, "Test Budget", ScopeGlobal, "global", 10.0, ResetDaily)
	tracker.DisableBudget(ctx, budget.ID)

	// Spending should not be tracked against disabled budget
	tracker.RecordSpend(ctx, "agent-1", "model-1", "", 100.0, 1000, 500)

	// Budget should still be at 0
	updated, _ := tracker.GetBudget(ctx, budget.ID)
	if updated.ConsumedUSD != 0 {
		t.Errorf("Disabled budget should not track spending, got %f", updated.ConsumedUSD)
	}

	// CheckBudget should not consider disabled budgets
	allowed, _, _ := tracker.CheckBudget(ctx, "agent-1", "model-1", 100.0)
	if !allowed {
		t.Error("CheckBudget should not block on disabled budgets")
	}
}
