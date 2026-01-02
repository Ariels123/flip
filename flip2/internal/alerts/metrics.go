package alerts

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/pocketbase/pocketbase"
)

// MetricProvider supplies metric values for alert evaluation
type MetricProvider interface {
	GetDBSizeMB(ctx context.Context) (float64, error)
	GetErrorRatePercent(ctx context.Context) (float64, error)
	GetSyncFailedCount1H(ctx context.Context) (int, error)
	GetMemoryMB(ctx context.Context) (float64, error)
	GetCostTodayUSD(ctx context.Context) (float64, error)
	GetHealthCheckFailed(ctx context.Context) (int, error)
}

// PBMetricProvider implements MetricProvider using PocketBase data
type PBMetricProvider struct {
	app      *pocketbase.PocketBase
	dataPath string
}

// NewPBMetricProvider creates a new PocketBase-backed metric provider
func NewPBMetricProvider(app *pocketbase.PocketBase, dataPath string) *PBMetricProvider {
	return &PBMetricProvider{
		app:      app,
		dataPath: dataPath,
	}
}

// GetDBSizeMB returns size of auxiliary.db in megabytes
func (p *PBMetricProvider) GetDBSizeMB(ctx context.Context) (float64, error) {
	path := filepath.Join(p.dataPath, "auxiliary.db")
	info, err := os.Stat(path)
	if err != nil {
		// If file doesn't exist, return 0 (not an error condition)
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to stat auxiliary.db: %w", err)
	}

	sizeMB := float64(info.Size()) / (1024 * 1024)
	return sizeMB, nil
}

// GetErrorRatePercent calculates error rate from signals (last hour)
func (p *PBMetricProvider) GetErrorRatePercent(ctx context.Context) (float64, error) {
	collection, err := p.app.FindCollectionByNameOrId("signals")
	if err != nil {
		return 0, fmt.Errorf("signals collection not found: %w", err)
	}

	// Calculate time range (last hour)
	oneHourAgo := time.Now().Add(-1 * time.Hour)
	timeFilter := fmt.Sprintf("@created >= '%s'", oneHourAgo.Format(time.RFC3339))

	// Count total signals in last hour
	totalRecords, err := p.app.FindRecordsByFilter(
		collection.Id,
		timeFilter,
		"",
		10000, // Max signals to consider
		0,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to query total signals: %w", err)
	}

	totalCount := len(totalRecords)
	if totalCount == 0 {
		return 0, nil // No signals, 0% error rate
	}

	// Count error signals in last hour
	errorFilter := fmt.Sprintf("@created >= '%s' && level = 'error'", oneHourAgo.Format(time.RFC3339))
	errorRecords, err := p.app.FindRecordsByFilter(
		collection.Id,
		errorFilter,
		"",
		10000,
		0,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to query error signals: %w", err)
	}

	errorCount := len(errorRecords)
	errorRate := (float64(errorCount) / float64(totalCount)) * 100.0

	return errorRate, nil
}

// GetSyncFailedCount1H counts sync failures in the last hour
func (p *PBMetricProvider) GetSyncFailedCount1H(ctx context.Context) (int, error) {
	// If app is nil, return 0 (used in tests)
	if p.app == nil {
		return 0, nil
	}

	// Check if sync_status collection exists
	collection, err := p.app.FindCollectionByNameOrId("sync_status")
	if err != nil {
		// Collection doesn't exist yet (Windows not deployed)
		return 0, nil
	}

	// Count failed syncs in last hour
	oneHourAgo := time.Now().Add(-1 * time.Hour)
	filter := fmt.Sprintf("@created >= '%s' && status = 'failed'", oneHourAgo.Format(time.RFC3339))

	records, err := p.app.FindRecordsByFilter(
		collection.Id,
		filter,
		"",
		1000,
		0,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to query sync failures: %w", err)
	}

	return len(records), nil
}

// GetMemoryMB returns current process memory usage in megabytes
func (p *PBMetricProvider) GetMemoryMB(ctx context.Context) (float64, error) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Return allocated memory in MB
	memoryMB := float64(m.Alloc) / (1024 * 1024)
	return memoryMB, nil
}

// GetCostTodayUSD sums costs from costs collection for today
func (p *PBMetricProvider) GetCostTodayUSD(ctx context.Context) (float64, error) {
	collection, err := p.app.FindCollectionByNameOrId("costs")
	if err != nil {
		return 0, fmt.Errorf("costs collection not found: %w", err)
	}

	// Calculate today's date range (midnight to now)
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	// Query costs for today
	filter := fmt.Sprintf("timestamp >= '%s'", todayStart.Format(time.RFC3339))
	records, err := p.app.FindRecordsByFilter(
		collection.Id,
		filter,
		"",
		10000, // Max cost records for one day
		0,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to query costs: %w", err)
	}

	// Sum up cost_usd field
	var totalCost float64
	for _, record := range records {
		cost := record.GetFloat("cost_usd")
		totalCost += cost
	}

	return totalCost, nil
}

// GetHealthCheckFailed returns 1 if health check failed, 0 otherwise
func (p *PBMetricProvider) GetHealthCheckFailed(ctx context.Context) (int, error) {
	// This will be implemented in Phase 5 when health checks are added
	// For now, return 0 (healthy)
	return 0, nil
}
