package budget

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// PBStore implements BudgetStore using PocketBase
type PBStore struct {
	app        *pocketbase.PocketBase
	budgetsCol string
	consumpCol string
}

// NewPBStore creates a new PocketBase-backed budget store
func NewPBStore(app *pocketbase.PocketBase) *PBStore {
	return &PBStore{
		app:        app,
		budgetsCol: "budgets",
		consumpCol: "budget_consumptions",
	}
}

// CreateBudget creates a new budget
func (s *PBStore) CreateBudget(ctx context.Context, budget *Budget) error {
	collection, err := s.app.FindCollectionByNameOrId(s.budgetsCol)
	if err != nil {
		return fmt.Errorf("budgets collection not found: %w", err)
	}

	record := core.NewRecord(collection)
	s.mapBudgetToRecord(budget, record)

	if err := s.app.Save(record); err != nil {
		return fmt.Errorf("failed to save budget: %w", err)
	}

	budget.ID = record.Id
	return nil
}

// GetBudget retrieves a budget by ID
func (s *PBStore) GetBudget(ctx context.Context, id string) (*Budget, error) {
	record, err := s.app.FindRecordById(s.budgetsCol, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get budget: %w", err)
	}

	return s.mapRecordToBudget(record), nil
}

// GetBudgetByScope retrieves a budget by scope and scopeID
func (s *PBStore) GetBudgetByScope(ctx context.Context, scope BudgetScope, scopeID string) (*Budget, error) {
	records, err := s.app.FindRecordsByFilter(
		s.budgetsCol,
		fmt.Sprintf("scope = '%s' && scope_id = '%s'", scope, scopeID),
		"-created",
		1,
		0,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find budget by scope: %w", err)
	}

	if len(records) == 0 {
		return nil, nil // Not found
	}

	return s.mapRecordToBudget(records[0]), nil
}

// ListBudgets returns all configured budgets
func (s *PBStore) ListBudgets(ctx context.Context) ([]*Budget, error) {
	records, err := s.app.FindRecordsByFilter(
		s.budgetsCol,
		"",
		"-created",
		1000,
		0,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list budgets: %w", err)
	}

	budgets := make([]*Budget, len(records))
	for i, r := range records {
		budgets[i] = s.mapRecordToBudget(r)
	}
	return budgets, nil
}

// UpdateBudget updates an existing budget
func (s *PBStore) UpdateBudget(ctx context.Context, budget *Budget) error {
	record, err := s.app.FindRecordById(s.budgetsCol, budget.ID)
	if err != nil {
		return fmt.Errorf("failed to find budget to update: %w", err)
	}

	s.mapBudgetToRecord(budget, record)

	if err := s.app.Save(record); err != nil {
		return fmt.Errorf("failed to update budget: %w", err)
	}

	return nil
}

// DeleteBudget deletes a budget
func (s *PBStore) DeleteBudget(ctx context.Context, id string) error {
	record, err := s.app.FindRecordById(s.budgetsCol, id)
	if err != nil {
		return fmt.Errorf("failed to find budget to delete: %w", err)
	}

	if err := s.app.Delete(record); err != nil {
		return fmt.Errorf("failed to delete budget: %w", err)
	}

	return nil
}

// RecordConsumption records a single consumption event
func (s *PBStore) RecordConsumption(ctx context.Context, consumption *BudgetConsumption) error {
	collection, err := s.app.FindCollectionByNameOrId(s.consumpCol)
	if err != nil {
		return fmt.Errorf("budget_consumptions collection not found: %w", err)
	}

	record := core.NewRecord(collection)
	record.Set("budget_id", consumption.BudgetID)
	record.Set("agent_id", consumption.AgentID)
	record.Set("model", consumption.Model)
	record.Set("task_id", consumption.TaskID)
	record.Set("amount_usd", consumption.AmountUSD)
	record.Set("input_tokens", consumption.InputTokens)
	record.Set("output_tokens", consumption.OutputTokens)
	record.Set("timestamp", consumption.Timestamp)
	record.Set("period_start", consumption.PeriodStart)

	if err := s.app.Save(record); err != nil {
		return fmt.Errorf("failed to save consumption: %w", err)
	}

	consumption.ID = record.Id
	return nil
}

// GetConsumptions retrieves consumption records
func (s *PBStore) GetConsumptions(ctx context.Context, budgetID string, start, end time.Time) ([]*BudgetConsumption, error) {
	filter := fmt.Sprintf("budget_id = '%s' && timestamp >= '%s' && timestamp < '%s'",
		budgetID,
		start.Format(time.RFC3339),
		end.Format(time.RFC3339),
	)

	records, err := s.app.FindRecordsByFilter(
		s.consumpCol,
		filter,
		"-timestamp",
		1000,
		0,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get consumptions: %w", err)
	}

	consumptions := make([]*BudgetConsumption, len(records))
	for i, r := range records {
		consumptions[i] = &BudgetConsumption{
			ID:           r.Id,
			BudgetID:     r.GetString("budget_id"),
			AgentID:      r.GetString("agent_id"),
			Model:        r.GetString("model"),
			TaskID:       r.GetString("task_id"),
			AmountUSD:    r.GetFloat("amount_usd"),
			InputTokens:  r.GetInt("input_tokens"),
			OutputTokens: r.GetInt("output_tokens"),
			Timestamp:    r.GetDateTime("timestamp").Time(),
			PeriodStart:  r.GetDateTime("period_start").Time(),
		}
	}

	return consumptions, nil
}

// GetTotalConsumed calculates total consumption in a range
func (s *PBStore) GetTotalConsumed(ctx context.Context, budgetID string, start, end time.Time) (float64, error) {
	// For efficiency, we should use aggregation if possible. PocketBase Go API supports it via DB().Select().
	// But sticking to FindRecords for simplicity and consistency with other modules first.
	consumptions, err := s.GetConsumptions(ctx, budgetID, start, end)
	if err != nil {
		return 0, err
	}

	var total float64
	for _, c := range consumptions {
		total += c.AmountUSD
	}
	return total, nil
}

// GetTopConsumersByAgent aggregates consumption by agent
func (s *PBStore) GetTopConsumersByAgent(ctx context.Context, budgetID string, start, end time.Time, limit int) ([]ConsumerSummary, error) {
	consumptions, err := s.GetConsumptions(ctx, budgetID, start, end)
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

	summaries := make([]ConsumerSummary, 0, len(totals))
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

	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].ConsumedUSD > summaries[j].ConsumedUSD
	})

	if limit > 0 && len(summaries) > limit {
		summaries = summaries[:limit]
	}

	return summaries, nil
}

// GetTopConsumersByModel aggregates consumption by model
func (s *PBStore) GetTopConsumersByModel(ctx context.Context, budgetID string, start, end time.Time, limit int) ([]ConsumerSummary, error) {
	consumptions, err := s.GetConsumptions(ctx, budgetID, start, end)
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

	summaries := make([]ConsumerSummary, 0, len(totals))
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

	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].ConsumedUSD > summaries[j].ConsumedUSD
	})

	if limit > 0 && len(summaries) > limit {
		summaries = summaries[:limit]
	}

	return summaries, nil
}

func (s *PBStore) mapBudgetToRecord(budget *Budget, record *core.Record) {
	record.Set("name", budget.Name)
	record.Set("scope", string(budget.Scope))
	record.Set("scope_id", budget.ScopeID)
	record.Set("limit_usd", budget.LimitUSD)
	record.Set("reset_period", string(budget.ResetPeriod))
	record.Set("warning_pct", budget.WarningPct)
	record.Set("consumed_usd", budget.ConsumedUSD)
	record.Set("status", string(budget.Status))
	record.Set("last_reset_at", budget.LastResetAt)
	record.Set("next_reset_at", budget.NextResetAt)
	record.Set("enabled", budget.Enabled)
}

func (s *PBStore) mapRecordToBudget(record *core.Record) *Budget {
	return &Budget{
		ID:          record.Id,
		Name:        record.GetString("name"),
		Scope:       BudgetScope(record.GetString("scope")),
		ScopeID:     record.GetString("scope_id"),
		LimitUSD:    record.GetFloat("limit_usd"),
		ResetPeriod: ResetPeriod(record.GetString("reset_period")),
		WarningPct:  record.GetFloat("warning_pct"),
		ConsumedUSD: record.GetFloat("consumed_usd"),
		Status:      BudgetStatus(record.GetString("status")),
		LastResetAt: record.GetDateTime("last_reset_at").Time(),
		NextResetAt: record.GetDateTime("next_reset_at").Time(),
		CreatedAt:   record.GetDateTime("created").Time(),
		UpdatedAt:   record.GetDateTime("updated").Time(),
		Enabled:     record.GetBool("enabled"),
	}
}
