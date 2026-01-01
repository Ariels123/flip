package costtracker

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

var (
	// validAgentID matches alphanumeric, hyphens, and underscores only
	validAgentID = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	// validModel matches alphanumeric, hyphens, underscores, dots, and slashes (for model names like "gpt-4" or "claude-3.5")
	validModel = regexp.MustCompile(`^[a-zA-Z0-9._/-]+$`)
)

// PBStore implements CostStore using PocketBase
type PBStore struct {
	app        *pocketbase.PocketBase
	collection string
}

// NewPBStore creates a new PocketBase-backed cost store
func NewPBStore(app *pocketbase.PocketBase) *PBStore {
	return &PBStore{
		app:        app,
		collection: "costs",
	}
}

// SaveCost saves a cost record to PocketBase
func (s *PBStore) SaveCost(ctx context.Context, record CostRecord) error {
	// Validate inputs to prevent SQL injection and data corruption
	if !validAgentID.MatchString(record.AgentID) {
		return fmt.Errorf("invalid agent_id format: %q (must be alphanumeric with hyphens/underscores)", record.AgentID)
	}
	if !validModel.MatchString(record.Model) {
		return fmt.Errorf("invalid model format: %q (must be alphanumeric with hyphens/underscores/dots/slashes)", record.Model)
	}
	// Validate task_id if present (same pattern as agent_id)
	if record.TaskID != "" && !validAgentID.MatchString(record.TaskID) {
		return fmt.Errorf("invalid task_id format: %q (must be alphanumeric with hyphens/underscores)", record.TaskID)
	}

	collection, err := s.app.FindCollectionByNameOrId(s.collection)
	if err != nil {
		return fmt.Errorf("costs collection not found: %w", err)
	}

	pbRecord := core.NewRecord(collection)
	pbRecord.Set("agent_id", record.AgentID)
	pbRecord.Set("model", record.Model)
	pbRecord.Set("input_tokens", record.InputTokens)
	pbRecord.Set("output_tokens", record.OutputTokens)
	pbRecord.Set("cost_usd", record.CostUSD)
	pbRecord.Set("task_id", record.TaskID)
	pbRecord.Set("timestamp", record.Timestamp)

	if err := s.app.Save(pbRecord); err != nil {
		return fmt.Errorf("failed to save cost record: %w", err)
	}

	return nil
}

// GetCostsByAgent retrieves costs for a specific agent in a date range
func (s *PBStore) GetCostsByAgent(ctx context.Context, agentID string, start, end time.Time) ([]CostRecord, error) {
	// Validate agent ID to prevent SQL injection
	if !validAgentID.MatchString(agentID) {
		return nil, fmt.Errorf("invalid agent_id format: %q (must be alphanumeric with hyphens/underscores)", agentID)
	}

	filter := fmt.Sprintf("agent_id = '%s' && timestamp >= '%s' && timestamp < '%s'",
		agentID,
		start.Format(time.RFC3339),
		end.Format(time.RFC3339),
	)
	return s.queryCosts(filter, 1000)
}

// GetCostsByModel retrieves costs for a specific model in a date range
func (s *PBStore) GetCostsByModel(ctx context.Context, model string, start, end time.Time) ([]CostRecord, error) {
	// Validate model name to prevent SQL injection
	if !validModel.MatchString(model) {
		return nil, fmt.Errorf("invalid model format: %q (must be alphanumeric with hyphens/underscores/dots/slashes)", model)
	}

	filter := fmt.Sprintf("model = '%s' && timestamp >= '%s' && timestamp < '%s'",
		model,
		start.Format(time.RFC3339),
		end.Format(time.RFC3339),
	)
	return s.queryCosts(filter, 1000)
}

// GetCostsByDateRange retrieves all costs in a date range
func (s *PBStore) GetCostsByDateRange(ctx context.Context, start, end time.Time) ([]CostRecord, error) {
	filter := fmt.Sprintf("timestamp >= '%s' && timestamp < '%s'",
		start.Format(time.RFC3339),
		end.Format(time.RFC3339),
	)
	return s.queryCosts(filter, 1000)
}

// GetSummary returns aggregated cost summary for a date range
func (s *PBStore) GetSummary(ctx context.Context, start, end time.Time) (CostSummary, error) {
	records, err := s.GetCostsByDateRange(ctx, start, end)
	if err != nil {
		return CostSummary{}, err
	}

	summary := CostSummary{
		RecordCount: len(records),
	}

	for _, r := range records {
		summary.TotalCostUSD += r.CostUSD
		summary.TotalInputTokens += r.InputTokens
		summary.TotalOutputTokens += r.OutputTokens
	}

	return summary, nil
}

// queryCosts executes a filter query and returns cost records
func (s *PBStore) queryCosts(filter string, limit int) ([]CostRecord, error) {
	collection, err := s.app.FindCollectionByNameOrId(s.collection)
	if err != nil {
		return nil, fmt.Errorf("costs collection not found: %w", err)
	}

	records, err := s.app.FindRecordsByFilter(
		collection.Id,
		filter,
		"-timestamp", // Sort by timestamp descending
		limit,
		0,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query costs: %w", err)
	}

	costs := make([]CostRecord, 0, len(records))
	for _, record := range records {
		cost := CostRecord{
			ID:           record.Id,
			AgentID:      record.GetString("agent_id"),
			Model:        record.GetString("model"),
			InputTokens:  record.GetInt("input_tokens"),
			OutputTokens: record.GetInt("output_tokens"),
			CostUSD:      record.GetFloat("cost_usd"),
			TaskID:       record.GetString("task_id"),
			Timestamp:    record.GetDateTime("timestamp").Time(),
		}
		costs = append(costs, cost)
	}

	return costs, nil
}
