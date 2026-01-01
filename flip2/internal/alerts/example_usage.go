package alerts

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"
)

// Example demonstrates how to use the alert manager
func Example() {
	// This is example code showing how Phase 2+ will use the alert manager
	// Not meant to be run directly, but as reference for integration

	// Step 1: Create logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	// Step 2: Create store (requires PocketBase app)
	// store := NewPBAlertStore(app)

	// For this example, use mock store
	store := &ExampleMockAlertStore{alerts: make(map[string]*Alert)}

	// Step 3: Create manager
	manager := NewManager(store, logger)

	// Step 4: Define a rule (normally loaded from alerts.yaml)
	rule := &Rule{
		Name:        "high_error_rate",
		Description: "Error rate exceeds acceptable threshold",
		Metric:      "error_rate_percent",
		Threshold:   5.0,
		Operator:    ">",
		Severity:    SeverityWarning,
		Enabled:     true,
		Channels:    []string{"slack"},
	}

	ctx := context.Background()

	// Step 5: Check if metric exceeds threshold
	currentErrorRate := 7.5

	if currentErrorRate > rule.Threshold {
		// Step 6: Check if we should fire (deduplication)
		if manager.ShouldFire(rule.Name) {
			// Step 7: Fire the alert
			message := fmt.Sprintf("Error rate is %.1f%% (threshold: %.1f%%)",
				currentErrorRate, rule.Threshold)

			metadata := map[string]interface{}{
				"error_count":   75,
				"total_signals": 1000,
			}

			alert, err := manager.Fire(ctx, rule, currentErrorRate, message, metadata)
			if err != nil {
				logger.Error("Failed to fire alert", "error", err)
				return
			}

			logger.Info("Alert fired successfully",
				"alert_id", alert.ID,
				"alert_name", alert.AlertName,
			)
		}
	}

	// Step 8: Later, when metric returns to normal
	currentErrorRate = 3.0

	if currentErrorRate <= rule.Threshold {
		// Resolve the alert
		err := manager.Resolve(ctx, rule.Name)
		if err != nil {
			logger.Error("Failed to resolve alert", "error", err)
			return
		}

		logger.Info("Alert resolved", "alert_name", rule.Name)
	}

	// Step 9: Query active alerts (for dashboard)
	activeAlerts, err := manager.GetActive(ctx)
	if err != nil {
		logger.Error("Failed to get active alerts", "error", err)
		return
	}

	logger.Info("Active alerts", "count", len(activeAlerts))

	// Step 10: Periodic cleanup (run daily)
	// manager.CleanupOld(ctx, 7*24*time.Hour) // Keep 7 days
}

// ExampleMockAlertStore is used for the example documentation
type ExampleMockAlertStore struct {
	alerts map[string]*Alert
}

func (m *ExampleMockAlertStore) SaveAlert(ctx context.Context, alert *Alert) error {
	alert.ID = "example-id-123"
	m.alerts[alert.ID] = alert
	return nil
}

func (m *ExampleMockAlertStore) GetAlert(ctx context.Context, id string) (*Alert, error) {
	return m.alerts[id], nil
}

func (m *ExampleMockAlertStore) GetActiveAlerts(ctx context.Context) ([]*Alert, error) {
	active := make([]*Alert, 0)
	for _, alert := range m.alerts {
		if alert.State == StateFiring {
			active = append(active, alert)
		}
	}
	return active, nil
}

func (m *ExampleMockAlertStore) GetAlertsByName(ctx context.Context, name string) ([]*Alert, error) {
	return nil, nil
}

func (m *ExampleMockAlertStore) UpdateAlert(ctx context.Context, alert *Alert) error {
	m.alerts[alert.ID] = alert
	return nil
}

func (m *ExampleMockAlertStore) DeleteOldAlerts(ctx context.Context, before time.Time) error {
	return nil
}
