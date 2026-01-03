// Package budget provides budget consumption tracking with reset logic and reporting
// for the FLIP2 multi-agent system. It tracks spending against configurable budgets
// at agent, model, and global levels with automatic periodic resets.
package budget

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// ResetPeriod defines how often a budget resets
type ResetPeriod string

const (
	ResetHourly  ResetPeriod = "hourly"
	ResetDaily   ResetPeriod = "daily"
	ResetWeekly  ResetPeriod = "weekly"
	ResetMonthly ResetPeriod = "monthly"
)

// BudgetScope defines the scope of a budget (agent, model, or global)
type BudgetScope string

const (
	ScopeAgent  BudgetScope = "agent"
	ScopeModel  BudgetScope = "model"
	ScopeGlobal BudgetScope = "global"
)

// BudgetStatus represents the current status of a budget
type BudgetStatus string

const (
	StatusActive    BudgetStatus = "active"
	StatusWarning   BudgetStatus = "warning"   // >80% consumed
	StatusExhausted BudgetStatus = "exhausted" // 100% consumed
	StatusDisabled  BudgetStatus = "disabled"
)

// Budget represents a configured budget with limits and reset schedule
type Budget struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	Scope        BudgetScope  `json:"scope"`
	ScopeID      string       `json:"scope_id"`       // agent_id, model name, or "global"
	LimitUSD     float64      `json:"limit_usd"`      // Budget limit in USD
	ResetPeriod  ResetPeriod  `json:"reset_period"`   // hourly, daily, weekly, monthly
	WarningPct   float64      `json:"warning_pct"`    // Warning threshold (default 80%)
	ConsumedUSD  float64      `json:"consumed_usd"`   // Current period consumption
	Status       BudgetStatus `json:"status"`         // Current status
	LastResetAt  time.Time    `json:"last_reset_at"`  // Last reset timestamp
	NextResetAt  time.Time    `json:"next_reset_at"`  // Next scheduled reset
	CreatedAt    time.Time    `json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`
	Enabled      bool         `json:"enabled"`
}

// BudgetConsumption records a single consumption event
type BudgetConsumption struct {
	ID          string    `json:"id"`
	BudgetID    string    `json:"budget_id"`
	AgentID     string    `json:"agent_id"`
	Model       string    `json:"model"`
	TaskID      string    `json:"task_id,omitempty"`
	AmountUSD   float64   `json:"amount_usd"`
	InputTokens int       `json:"input_tokens"`
	OutputTokens int      `json:"output_tokens"`
	Timestamp   time.Time `json:"timestamp"`
	PeriodStart time.Time `json:"period_start"` // Budget period this belongs to
}

// BudgetReport provides a summary of budget usage
type BudgetReport struct {
	Budget          *Budget              `json:"budget"`
	TotalConsumed   float64              `json:"total_consumed"`
	RemainingUSD    float64              `json:"remaining_usd"`
	ConsumedPct     float64              `json:"consumed_pct"`
	ConsumptionRate float64              `json:"consumption_rate_per_hour"` // USD per hour
	ProjectedUsage  float64              `json:"projected_usage"`           // Projected at current rate
	TopConsumers    []ConsumerSummary    `json:"top_consumers"`             // Top consumers this period
	RecentActivity  []BudgetConsumption  `json:"recent_activity"`           // Recent consumption events
	GeneratedAt     time.Time            `json:"generated_at"`
}

// ConsumerSummary summarizes consumption by a single entity
type ConsumerSummary struct {
	ID          string  `json:"id"`           // agent_id or model name
	Type        string  `json:"type"`         // "agent" or "model"
	ConsumedUSD float64 `json:"consumed_usd"`
	Percentage  float64 `json:"percentage"`   // Percentage of total budget
	EventCount  int     `json:"event_count"`
}

// BudgetStore defines the interface for budget persistence
type BudgetStore interface {
	// Budget CRUD
	CreateBudget(ctx context.Context, budget *Budget) error
	GetBudget(ctx context.Context, id string) (*Budget, error)
	GetBudgetByScope(ctx context.Context, scope BudgetScope, scopeID string) (*Budget, error)
	ListBudgets(ctx context.Context) ([]*Budget, error)
	UpdateBudget(ctx context.Context, budget *Budget) error
	DeleteBudget(ctx context.Context, id string) error

	// Consumption tracking
	RecordConsumption(ctx context.Context, consumption *BudgetConsumption) error
	GetConsumptions(ctx context.Context, budgetID string, start, end time.Time) ([]*BudgetConsumption, error)
	GetTotalConsumed(ctx context.Context, budgetID string, start, end time.Time) (float64, error)

	// Aggregations
	GetTopConsumersByAgent(ctx context.Context, budgetID string, start, end time.Time, limit int) ([]ConsumerSummary, error)
	GetTopConsumersByModel(ctx context.Context, budgetID string, start, end time.Time, limit int) ([]ConsumerSummary, error)
}

// BudgetAlertCallback is called when budget thresholds are crossed
type BudgetAlertCallback func(budget *Budget, alertType string)

// Tracker manages budget tracking, consumption, and resets
type Tracker struct {
	store         BudgetStore
	logger        *slog.Logger
	mu            sync.RWMutex
	budgets       map[string]*Budget // Cache of active budgets
	alertCallback BudgetAlertCallback
}

// New creates a new budget tracker
func New(store BudgetStore, logger *slog.Logger) *Tracker {
	if logger == nil {
		logger = slog.Default()
	}
	return &Tracker{
		store:   store,
		logger:  logger.WithGroup("budget"),
		budgets: make(map[string]*Budget),
	}
}

// SetAlertCallback sets the callback function for budget alerts
func (t *Tracker) SetAlertCallback(callback BudgetAlertCallback) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.alertCallback = callback
}

// Alert types for budget notifications
const (
	AlertTypeBudgetWarning   = "budget_warning"
	AlertTypeBudgetExhausted = "budget_exhausted"
	AlertTypeBudgetReset     = "budget_reset"
)

// CreateBudget creates a new budget
func (t *Tracker) CreateBudget(ctx context.Context, name string, scope BudgetScope, scopeID string, limitUSD float64, period ResetPeriod) (*Budget, error) {
	now := time.Now()

	budget := &Budget{
		Name:        name,
		Scope:       scope,
		ScopeID:     scopeID,
		LimitUSD:    limitUSD,
		ResetPeriod: period,
		WarningPct:  80.0, // Default 80% warning threshold
		ConsumedUSD: 0,
		Status:      StatusActive,
		LastResetAt: now,
		NextResetAt: calculateNextReset(now, period),
		CreatedAt:   now,
		UpdatedAt:   now,
		Enabled:     true,
	}

	if err := t.store.CreateBudget(ctx, budget); err != nil {
		return nil, fmt.Errorf("failed to create budget: %w", err)
	}

	// Cache the budget
	t.mu.Lock()
	t.budgets[budget.ID] = budget
	t.mu.Unlock()

	t.logger.Info("Budget created",
		"id", budget.ID,
		"name", name,
		"scope", scope,
		"scope_id", scopeID,
		"limit_usd", limitUSD,
		"period", period,
	)

	return budget, nil
}

// RecordSpend records a spending event against applicable budgets
func (t *Tracker) RecordSpend(ctx context.Context, agentID, model, taskID string, amountUSD float64, inputTokens, outputTokens int) error {
	now := time.Now()

	// Find all applicable budget IDs (agent-specific, model-specific, and global)
	budgetIDs := make([]string, 0)

	// Check agent budget
	if agentBudget, err := t.store.GetBudgetByScope(ctx, ScopeAgent, agentID); err == nil && agentBudget != nil && agentBudget.Enabled {
		budgetIDs = append(budgetIDs, agentBudget.ID)
	}

	// Check model budget
	if modelBudget, err := t.store.GetBudgetByScope(ctx, ScopeModel, model); err == nil && modelBudget != nil && modelBudget.Enabled {
		budgetIDs = append(budgetIDs, modelBudget.ID)
	}

	// Check global budget
	if globalBudget, err := t.store.GetBudgetByScope(ctx, ScopeGlobal, "global"); err == nil && globalBudget != nil && globalBudget.Enabled {
		budgetIDs = append(budgetIDs, globalBudget.ID)
	}

	// Record consumption for each applicable budget with proper locking
	for _, budgetID := range budgetIDs {
		if err := t.recordSpendForBudget(ctx, budgetID, agentID, model, taskID, amountUSD, inputTokens, outputTokens, now); err != nil {
			t.logger.Error("Failed to record spend for budget", "budget_id", budgetID, "error", err)
		}
	}

	return nil
}

// recordSpendForBudget records spending for a single budget with proper locking
func (t *Tracker) recordSpendForBudget(ctx context.Context, budgetID, agentID, model, taskID string, amountUSD float64, inputTokens, outputTokens int, now time.Time) error {
	// Lock for the entire read-modify-write cycle to prevent race conditions
	t.mu.Lock()
	defer t.mu.Unlock()

	// Get fresh budget state under lock
	budget, err := t.store.GetBudget(ctx, budgetID)
	if err != nil || budget == nil || !budget.Enabled {
		return nil
	}

	// Check if budget needs reset
	if now.After(budget.NextResetAt) {
		if err := t.resetBudgetLocked(ctx, budget); err != nil {
			t.logger.Error("Failed to reset budget", "budget_id", budget.ID, "error", err)
		}
	}

	consumption := &BudgetConsumption{
		BudgetID:     budget.ID,
		AgentID:      agentID,
		Model:        model,
		TaskID:       taskID,
		AmountUSD:    amountUSD,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		Timestamp:    now,
		PeriodStart:  budget.LastResetAt,
	}

	if err := t.store.RecordConsumption(ctx, consumption); err != nil {
		return fmt.Errorf("failed to record consumption: %w", err)
	}

	// Update budget consumed amount
	budget.ConsumedUSD += amountUSD
	budget.UpdatedAt = now
	budget.Status = t.calculateStatus(budget)

	if err := t.store.UpdateBudget(ctx, budget); err != nil {
		return fmt.Errorf("failed to update budget: %w", err)
	}

	// Update cache
	t.budgets[budget.ID] = budget

	// Log warnings and trigger alerts for high consumption
	if budget.Status == StatusWarning {
		t.logger.Warn("Budget warning threshold exceeded",
			"budget_id", budget.ID,
			"budget_name", budget.Name,
			"consumed_usd", budget.ConsumedUSD,
			"limit_usd", budget.LimitUSD,
			"consumed_pct", (budget.ConsumedUSD/budget.LimitUSD)*100,
		)
		t.triggerAlert(budget, AlertTypeBudgetWarning)
	} else if budget.Status == StatusExhausted {
		t.logger.Error("Budget exhausted",
			"budget_id", budget.ID,
			"budget_name", budget.Name,
			"consumed_usd", budget.ConsumedUSD,
			"limit_usd", budget.LimitUSD,
		)
		t.triggerAlert(budget, AlertTypeBudgetExhausted)
	}

	return nil
}

// triggerAlert safely triggers the alert callback if configured
func (t *Tracker) triggerAlert(budget *Budget, alertType string) {
	// Note: we don't hold the main lock while calling the callback to prevent deadlocks
	// The callback is set once at startup, so this is safe
	if t.alertCallback != nil {
		// Create a copy for the callback to avoid race conditions
		budgetCopy := *budget
		go t.alertCallback(&budgetCopy, alertType)
	}
}

// CheckBudget checks if spending is allowed for the given agent/model
func (t *Tracker) CheckBudget(ctx context.Context, agentID, model string, proposedAmountUSD float64) (bool, string, error) {
	now := time.Now()

	// Check agent budget
	if agentBudget, err := t.store.GetBudgetByScope(ctx, ScopeAgent, agentID); err == nil && agentBudget != nil && agentBudget.Enabled {
		consumed := agentBudget.ConsumedUSD
		// If budget period has expired, it will be reset on next spend, so treat it as 0
		if now.After(agentBudget.NextResetAt) {
			consumed = 0
		}

		if consumed >= agentBudget.LimitUSD {
			return false, fmt.Sprintf("agent budget exhausted: %s", agentBudget.Name), nil
		}
		if consumed+proposedAmountUSD > agentBudget.LimitUSD {
			return false, fmt.Sprintf("proposed spend would exceed agent budget: %.4f + %.4f > %.4f",
				consumed, proposedAmountUSD, agentBudget.LimitUSD), nil
		}
	}

	// Check model budget
	if modelBudget, err := t.store.GetBudgetByScope(ctx, ScopeModel, model); err == nil && modelBudget != nil && modelBudget.Enabled {
		consumed := modelBudget.ConsumedUSD
		// If budget period has expired, treat as 0
		if now.After(modelBudget.NextResetAt) {
			consumed = 0
		}

		if consumed >= modelBudget.LimitUSD {
			return false, fmt.Sprintf("model budget exhausted: %s", modelBudget.Name), nil
		}
		if consumed+proposedAmountUSD > modelBudget.LimitUSD {
			return false, fmt.Sprintf("proposed spend would exceed model budget: %.4f + %.4f > %.4f",
				consumed, proposedAmountUSD, modelBudget.LimitUSD), nil
		}
	}

	// Check global budget
	if globalBudget, err := t.store.GetBudgetByScope(ctx, ScopeGlobal, "global"); err == nil && globalBudget != nil && globalBudget.Enabled {
		consumed := globalBudget.ConsumedUSD
		// If budget period has expired, treat as 0
		if now.After(globalBudget.NextResetAt) {
			consumed = 0
		}

		if consumed >= globalBudget.LimitUSD {
			return false, "global budget exhausted", nil
		}
		if consumed+proposedAmountUSD > globalBudget.LimitUSD {
			return false, fmt.Sprintf("proposed spend would exceed global budget: %.4f + %.4f > %.4f",
				consumed, proposedAmountUSD, globalBudget.LimitUSD), nil
		}
	}

	return true, "", nil
}

// GetBudget retrieves a budget by ID
func (t *Tracker) GetBudget(ctx context.Context, id string) (*Budget, error) {
	return t.store.GetBudget(ctx, id)
}

// ListBudgets returns all configured budgets
func (t *Tracker) ListBudgets(ctx context.Context) ([]*Budget, error) {
	return t.store.ListBudgets(ctx)
}

// GetReport generates a comprehensive budget report
func (t *Tracker) GetReport(ctx context.Context, budgetID string) (*BudgetReport, error) {
	budget, err := t.store.GetBudget(ctx, budgetID)
	if err != nil {
		return nil, fmt.Errorf("budget not found: %w", err)
	}

	now := time.Now()

	// Get consumption for current period
	consumptions, err := t.store.GetConsumptions(ctx, budgetID, budget.LastResetAt, now)
	if err != nil {
		return nil, fmt.Errorf("failed to get consumptions: %w", err)
	}

	// Calculate total consumed
	totalConsumed, err := t.store.GetTotalConsumed(ctx, budgetID, budget.LastResetAt, now)
	if err != nil {
		return nil, fmt.Errorf("failed to get total consumed: %w", err)
	}

	// Get top consumers by agent
	topAgents, _ := t.store.GetTopConsumersByAgent(ctx, budgetID, budget.LastResetAt, now, 5)
	topModels, _ := t.store.GetTopConsumersByModel(ctx, budgetID, budget.LastResetAt, now, 5)

	// Combine top consumers
	topConsumers := make([]ConsumerSummary, 0, len(topAgents)+len(topModels))
	topConsumers = append(topConsumers, topAgents...)
	topConsumers = append(topConsumers, topModels...)

	// Calculate consumption rate (USD per hour)
	periodDuration := now.Sub(budget.LastResetAt)
	var consumptionRate float64
	if periodDuration.Hours() > 0 {
		consumptionRate = totalConsumed / periodDuration.Hours()
	}

	// Project usage for the full period
	fullPeriodHours := budget.NextResetAt.Sub(budget.LastResetAt).Hours()
	projectedUsage := consumptionRate * fullPeriodHours

	// Get recent activity (last 10)
	recentActivity := make([]BudgetConsumption, 0)
	if len(consumptions) > 10 {
		for i := len(consumptions) - 10; i < len(consumptions); i++ {
			recentActivity = append(recentActivity, *consumptions[i])
		}
	} else {
		for _, c := range consumptions {
			recentActivity = append(recentActivity, *c)
		}
	}

	return &BudgetReport{
		Budget:          budget,
		TotalConsumed:   totalConsumed,
		RemainingUSD:    budget.LimitUSD - totalConsumed,
		ConsumedPct:     (totalConsumed / budget.LimitUSD) * 100,
		ConsumptionRate: consumptionRate,
		ProjectedUsage:  projectedUsage,
		TopConsumers:    topConsumers,
		RecentActivity:  recentActivity,
		GeneratedAt:     now,
	}, nil
}

// ResetBudget manually resets a budget
func (t *Tracker) ResetBudget(ctx context.Context, budgetID string) error {
	budget, err := t.store.GetBudget(ctx, budgetID)
	if err != nil {
		return fmt.Errorf("budget not found: %w", err)
	}

	return t.resetBudget(ctx, budget)
}

// CheckAndResetBudgets checks all budgets and resets those past their reset time
func (t *Tracker) CheckAndResetBudgets(ctx context.Context) error {
	budgets, err := t.store.ListBudgets(ctx)
	if err != nil {
		return fmt.Errorf("failed to list budgets: %w", err)
	}

	now := time.Now()
	resetCount := 0

	for _, budget := range budgets {
		if budget.Enabled && now.After(budget.NextResetAt) {
			if err := t.resetBudget(ctx, budget); err != nil {
				t.logger.Error("Failed to reset budget",
					"budget_id", budget.ID,
					"budget_name", budget.Name,
					"error", err,
				)
			} else {
				resetCount++
			}
		}
	}

	if resetCount > 0 {
		t.logger.Info("Budget reset check completed", "reset_count", resetCount)
	}

	return nil
}

// DisableBudget disables a budget
func (t *Tracker) DisableBudget(ctx context.Context, budgetID string) error {
	budget, err := t.store.GetBudget(ctx, budgetID)
	if err != nil {
		return fmt.Errorf("budget not found: %w", err)
	}

	budget.Enabled = false
	budget.Status = StatusDisabled
	budget.UpdatedAt = time.Now()

	if err := t.store.UpdateBudget(ctx, budget); err != nil {
		return fmt.Errorf("failed to disable budget: %w", err)
	}

	t.mu.Lock()
	t.budgets[budget.ID] = budget
	t.mu.Unlock()

	t.logger.Info("Budget disabled", "budget_id", budget.ID, "budget_name", budget.Name)
	return nil
}

// EnableBudget enables a budget
func (t *Tracker) EnableBudget(ctx context.Context, budgetID string) error {
	budget, err := t.store.GetBudget(ctx, budgetID)
	if err != nil {
		return fmt.Errorf("budget not found: %w", err)
	}

	budget.Enabled = true
	budget.Status = t.calculateStatus(budget)
	budget.UpdatedAt = time.Now()

	if err := t.store.UpdateBudget(ctx, budget); err != nil {
		return fmt.Errorf("failed to enable budget: %w", err)
	}

	t.mu.Lock()
	t.budgets[budget.ID] = budget
	t.mu.Unlock()

	t.logger.Info("Budget enabled", "budget_id", budget.ID, "budget_name", budget.Name)
	return nil
}

// UpdateLimit updates the budget limit
func (t *Tracker) UpdateLimit(ctx context.Context, budgetID string, newLimitUSD float64) error {
	budget, err := t.store.GetBudget(ctx, budgetID)
	if err != nil {
		return fmt.Errorf("budget not found: %w", err)
	}

	oldLimit := budget.LimitUSD
	budget.LimitUSD = newLimitUSD
	budget.Status = t.calculateStatus(budget)
	budget.UpdatedAt = time.Now()

	if err := t.store.UpdateBudget(ctx, budget); err != nil {
		return fmt.Errorf("failed to update budget limit: %w", err)
	}

	t.mu.Lock()
	t.budgets[budget.ID] = budget
	t.mu.Unlock()

	t.logger.Info("Budget limit updated",
		"budget_id", budget.ID,
		"budget_name", budget.Name,
		"old_limit", oldLimit,
		"new_limit", newLimitUSD,
	)
	return nil
}

// resetBudget resets a budget to start a new period
func (t *Tracker) resetBudget(ctx context.Context, budget *Budget) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.resetBudgetLocked(ctx, budget)
}

// resetBudgetLocked resets a budget without acquiring the lock (caller must hold lock)
func (t *Tracker) resetBudgetLocked(ctx context.Context, budget *Budget) error {
	now := time.Now()

	previousConsumed := budget.ConsumedUSD

	budget.ConsumedUSD = 0
	budget.Status = StatusActive
	budget.LastResetAt = now
	budget.NextResetAt = calculateNextReset(now, budget.ResetPeriod)
	budget.UpdatedAt = now

	if err := t.store.UpdateBudget(ctx, budget); err != nil {
		return fmt.Errorf("failed to reset budget: %w", err)
	}

	t.budgets[budget.ID] = budget

	t.logger.Info("Budget reset",
		"budget_id", budget.ID,
		"budget_name", budget.Name,
		"previous_consumed", previousConsumed,
		"next_reset", budget.NextResetAt,
	)

	return nil
}

// calculateStatus determines the budget status based on consumption
func (t *Tracker) calculateStatus(budget *Budget) BudgetStatus {
	if !budget.Enabled {
		return StatusDisabled
	}

	if budget.ConsumedUSD >= budget.LimitUSD {
		return StatusExhausted
	}

	consumedPct := (budget.ConsumedUSD / budget.LimitUSD) * 100
	if consumedPct >= budget.WarningPct {
		return StatusWarning
	}

	return StatusActive
}

// calculateNextReset calculates the next reset time based on period
func calculateNextReset(from time.Time, period ResetPeriod) time.Time {
	switch period {
	case ResetHourly:
		return from.Add(time.Hour).Truncate(time.Hour)
	case ResetDaily:
		return time.Date(from.Year(), from.Month(), from.Day()+1, 0, 0, 0, 0, from.Location())
	case ResetWeekly:
		// Next Monday at midnight
		daysUntilMonday := (8 - int(from.Weekday())) % 7
		if daysUntilMonday == 0 {
			daysUntilMonday = 7
		}
		return time.Date(from.Year(), from.Month(), from.Day()+daysUntilMonday, 0, 0, 0, 0, from.Location())
	case ResetMonthly:
		// First day of next month at midnight
		return time.Date(from.Year(), from.Month()+1, 1, 0, 0, 0, 0, from.Location())
	default:
		return from.Add(24 * time.Hour) // Default to daily
	}
}

// String methods for types
func (p ResetPeriod) String() string {
	return string(p)
}

func (s BudgetScope) String() string {
	return string(s)
}

func (s BudgetStatus) String() string {
	return string(s)
}

// IsValid validates the reset period
func (p ResetPeriod) IsValid() bool {
	switch p {
	case ResetHourly, ResetDaily, ResetWeekly, ResetMonthly:
		return true
	default:
		return false
	}
}

// IsValid validates the budget scope
func (s BudgetScope) IsValid() bool {
	switch s {
	case ScopeAgent, ScopeModel, ScopeGlobal:
		return true
	default:
		return false
	}
}
