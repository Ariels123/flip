package costtracker

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// CostRecord represents a single LLM API cost record
type CostRecord struct {
	ID           string    `json:"id"`
	AgentID      string    `json:"agent_id"`
	Model        string    `json:"model"`
	InputTokens  int       `json:"input_tokens"`
	OutputTokens int       `json:"output_tokens"`
	CostUSD      float64   `json:"cost_usd"`
	TaskID       string    `json:"task_id,omitempty"`
	Timestamp    time.Time `json:"timestamp"`
}

// CostSummary aggregates costs by various dimensions
type CostSummary struct {
	TotalCostUSD    float64 `json:"total_cost_usd"`
	TotalInputTokens int    `json:"total_input_tokens"`
	TotalOutputTokens int   `json:"total_output_tokens"`
	RecordCount      int    `json:"record_count"`
}

// CostStore defines the interface for cost persistence
type CostStore interface {
	SaveCost(ctx context.Context, record CostRecord) error
	GetCostsByAgent(ctx context.Context, agentID string, start, end time.Time) ([]CostRecord, error)
	GetCostsByModel(ctx context.Context, model string, start, end time.Time) ([]CostRecord, error)
	GetCostsByDateRange(ctx context.Context, start, end time.Time) ([]CostRecord, error)
	GetSummary(ctx context.Context, start, end time.Time) (CostSummary, error)
}

// Tracker tracks and aggregates LLM API costs
type Tracker struct {
	store  CostStore
	logger *slog.Logger
}

// New creates a new cost tracker
func New(store CostStore, logger *slog.Logger) *Tracker {
	if logger == nil {
		logger = slog.Default()
	}
	return &Tracker{
		store:  store,
		logger: logger.WithGroup("costtracker"),
	}
}

// RecordCost records a cost entry
func (t *Tracker) RecordCost(ctx context.Context, agentID, model, taskID string, inputTokens, outputTokens int, costUSD float64) error {
	record := CostRecord{
		AgentID:      agentID,
		Model:        model,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		CostUSD:      costUSD,
		TaskID:       taskID,
		Timestamp:    time.Now(),
	}

	if err := t.store.SaveCost(ctx, record); err != nil {
		t.logger.Error("Failed to record cost",
			"agent_id", agentID,
			"model", model,
			"cost_usd", costUSD,
			"error", err,
		)
		return fmt.Errorf("failed to save cost record: %w", err)
	}

	t.logger.Debug("Cost recorded",
		"agent_id", agentID,
		"model", model,
		"input_tokens", inputTokens,
		"output_tokens", outputTokens,
		"cost_usd", costUSD,
	)

	return nil
}

// GetDailyCosts returns costs for a specific day
func (t *Tracker) GetDailyCosts(ctx context.Context, date time.Time) ([]CostRecord, error) {
	start := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	end := start.Add(24 * time.Hour)
	return t.store.GetCostsByDateRange(ctx, start, end)
}

// GetMonthlyCosts returns costs for a specific month
func (t *Tracker) GetMonthlyCosts(ctx context.Context, year int, month time.Month) ([]CostRecord, error) {
	start := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	return t.store.GetCostsByDateRange(ctx, start, end)
}

// GetAgentCosts returns costs for a specific agent in a date range
func (t *Tracker) GetAgentCosts(ctx context.Context, agentID string, start, end time.Time) ([]CostRecord, error) {
	return t.store.GetCostsByAgent(ctx, agentID, start, end)
}

// GetModelCosts returns costs for a specific model in a date range
func (t *Tracker) GetModelCosts(ctx context.Context, model string, start, end time.Time) ([]CostRecord, error) {
	return t.store.GetCostsByModel(ctx, model, start, end)
}

// GetSummary returns aggregated cost summary for a date range
func (t *Tracker) GetSummary(ctx context.Context, start, end time.Time) (CostSummary, error) {
	return t.store.GetSummary(ctx, start, end)
}
