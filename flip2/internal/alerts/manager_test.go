package alerts

import (
	"context"
	"log/slog"
	"testing"
	"time"
)

// MockAlertStore is a simple in-memory store for testing
type MockAlertStore struct {
	alerts map[string]*Alert
}

func NewMockAlertStore() *MockAlertStore {
	return &MockAlertStore{
		alerts: make(map[string]*Alert),
	}
}

func (m *MockAlertStore) SaveAlert(ctx context.Context, alert *Alert) error {
	if alert.ID == "" {
		alert.ID = "test-" + alert.AlertName + "-" + time.Now().Format("20060102150405")
	}
	m.alerts[alert.ID] = alert
	return nil
}

func (m *MockAlertStore) GetAlert(ctx context.Context, id string) (*Alert, error) {
	return m.alerts[id], nil
}

func (m *MockAlertStore) GetActiveAlerts(ctx context.Context) ([]*Alert, error) {
	active := make([]*Alert, 0)
	for _, alert := range m.alerts {
		if alert.State == StateFiring {
			active = append(active, alert)
		}
	}
	return active, nil
}

func (m *MockAlertStore) GetAlertsByName(ctx context.Context, name string) ([]*Alert, error) {
	results := make([]*Alert, 0)
	for _, alert := range m.alerts {
		if alert.AlertName == name {
			results = append(results, alert)
		}
	}
	return results, nil
}

func (m *MockAlertStore) UpdateAlert(ctx context.Context, alert *Alert) error {
	m.alerts[alert.ID] = alert
	return nil
}

func (m *MockAlertStore) DeleteOldAlerts(ctx context.Context, before time.Time) error {
	for id, alert := range m.alerts {
		if alert.State == StateResolved && alert.ResolvedAt != nil && alert.ResolvedAt.Before(before) {
			delete(m.alerts, id)
		}
	}
	return nil
}

func TestManagerFireAndResolve(t *testing.T) {
	store := NewMockAlertStore()
	logger := slog.Default()
	manager := NewManager(store, logger)

	ctx := context.Background()

	// Create a test rule
	rule := &Rule{
		Name:        "test_alert",
		Description: "Test alert",
		Metric:      "test_metric",
		Threshold:   100.0,
		Operator:    ">",
		Severity:    SeverityWarning,
		Enabled:     true,
	}

	// Fire an alert
	alert, err := manager.Fire(ctx, rule, 150.0, "Test alert fired", nil)
	if err != nil {
		t.Fatalf("Failed to fire alert: %v", err)
	}

	if alert == nil {
		t.Fatal("Alert should not be nil")
	}

	if alert.State != StateFiring {
		t.Errorf("Expected state to be firing, got %s", alert.State)
	}

	// Verify alert is in active alerts
	active, err := manager.GetActive(ctx)
	if err != nil {
		t.Fatalf("Failed to get active alerts: %v", err)
	}

	if len(active) != 1 {
		t.Errorf("Expected 1 active alert, got %d", len(active))
	}

	// Try to fire same alert again (should be deduplicated)
	alert2, err := manager.Fire(ctx, rule, 160.0, "Test alert fired again", nil)
	if err != nil {
		t.Fatalf("Failed to fire alert: %v", err)
	}

	if alert2.ID != alert.ID {
		t.Error("Should have returned existing alert (deduplication)")
	}

	// Resolve the alert
	err = manager.Resolve(ctx, rule.Name)
	if err != nil {
		t.Fatalf("Failed to resolve alert: %v", err)
	}

	// Verify no active alerts
	active, err = manager.GetActive(ctx)
	if err != nil {
		t.Fatalf("Failed to get active alerts: %v", err)
	}

	if len(active) != 0 {
		t.Errorf("Expected 0 active alerts, got %d", len(active))
	}
}

func TestManagerCooldown(t *testing.T) {
	store := NewMockAlertStore()
	logger := slog.Default()
	manager := NewManager(store, logger)

	// Set a short cooldown for testing
	manager.SetCooldownPeriod(1 * time.Second)

	ctx := context.Background()

	rule := &Rule{
		Name:      "cooldown_test",
		Threshold: 100.0,
		Operator:  ">",
		Severity:  SeverityWarning,
		Enabled:   true,
	}

	// Fire alert
	_, err := manager.Fire(ctx, rule, 150.0, "First fire", nil)
	if err != nil {
		t.Fatalf("Failed to fire alert: %v", err)
	}

	// Resolve immediately
	err = manager.Resolve(ctx, rule.Name)
	if err != nil {
		t.Fatalf("Failed to resolve alert: %v", err)
	}

	// Try to fire again immediately (should be in cooldown)
	if manager.ShouldFire(rule.Name) {
		t.Error("Alert should be in cooldown period")
	}

	// Wait for cooldown to expire
	time.Sleep(1100 * time.Millisecond)

	// Should be able to fire now
	if !manager.ShouldFire(rule.Name) {
		t.Error("Alert should be fireable after cooldown")
	}
}

func TestManagerCleanup(t *testing.T) {
	store := NewMockAlertStore()
	logger := slog.Default()
	manager := NewManager(store, logger)

	ctx := context.Background()

	rule := &Rule{
		Name:      "cleanup_test",
		Threshold: 100.0,
		Operator:  ">",
		Severity:  SeverityInfo,
		Enabled:   true,
	}

	// Fire and resolve an alert
	_, err := manager.Fire(ctx, rule, 150.0, "Test", nil)
	if err != nil {
		t.Fatalf("Failed to fire alert: %v", err)
	}

	err = manager.Resolve(ctx, rule.Name)
	if err != nil {
		t.Fatalf("Failed to resolve alert: %v", err)
	}

	// Cleanup alerts older than 1 hour
	err = manager.CleanupOld(ctx, 1*time.Hour)
	if err != nil {
		t.Fatalf("Failed to cleanup: %v", err)
	}

	// Alert should still exist (not old enough)
	alerts, err := store.GetAlertsByName(ctx, rule.Name)
	if err != nil {
		t.Fatalf("Failed to get alerts: %v", err)
	}

	if len(alerts) != 1 {
		t.Errorf("Expected 1 alert, got %d", len(alerts))
	}

	// Cleanup alerts older than 0 seconds (should remove all resolved)
	err = manager.CleanupOld(ctx, 0)
	if err != nil {
		t.Fatalf("Failed to cleanup: %v", err)
	}

	// Alert should be gone now
	alerts, err = store.GetAlertsByName(ctx, rule.Name)
	if err != nil {
		t.Fatalf("Failed to get alerts: %v", err)
	}

	if len(alerts) != 0 {
		t.Errorf("Expected 0 alerts, got %d", len(alerts))
	}
}
