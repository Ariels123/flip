package alerts

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"
)

// mockMetricProvider implements MetricProvider for testing
type mockMetricProvider struct {
	dbSize       float64
	errorRate    float64
	syncFailed   int
	memory       float64
	cost         float64
	healthFailed int
}

func (m *mockMetricProvider) GetDBSizeMB(ctx context.Context) (float64, error) {
	return m.dbSize, nil
}

func (m *mockMetricProvider) GetErrorRatePercent(ctx context.Context) (float64, error) {
	return m.errorRate, nil
}

func (m *mockMetricProvider) GetSyncFailedCount1H(ctx context.Context) (int, error) {
	return m.syncFailed, nil
}

func (m *mockMetricProvider) GetMemoryMB(ctx context.Context) (float64, error) {
	return m.memory, nil
}

func (m *mockMetricProvider) GetCostTodayUSD(ctx context.Context) (float64, error) {
	return m.cost, nil
}

func (m *mockMetricProvider) GetHealthCheckFailed(ctx context.Context) (int, error) {
	return m.healthFailed, nil
}

func TestCompareValue(t *testing.T) {
	tests := []struct {
		name      string
		value     float64
		threshold float64
		operator  string
		expected  bool
	}{
		{"greater than - true", 10.0, 5.0, ">", true},
		{"greater than - false", 5.0, 10.0, ">", false},
		{"greater than - equal", 10.0, 10.0, ">", false},

		{"less than - true", 5.0, 10.0, "<", true},
		{"less than - false", 10.0, 5.0, "<", false},
		{"less than - equal", 10.0, 10.0, "<", false},

		{"greater or equal - greater", 10.0, 5.0, ">=", true},
		{"greater or equal - equal", 10.0, 10.0, ">=", true},
		{"greater or equal - less", 5.0, 10.0, ">=", false},

		{"less or equal - less", 5.0, 10.0, "<=", true},
		{"less or equal - equal", 10.0, 10.0, "<=", true},
		{"less or equal - greater", 10.0, 5.0, "<=", false},

		{"equal - true", 10.0, 10.0, "==", true},
		{"equal - false", 10.0, 5.0, "==", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := compareValue(tt.value, tt.threshold, tt.operator)
			if result != tt.expected {
				t.Errorf("compareValue(%v, %v, %s) = %v, expected %v",
					tt.value, tt.threshold, tt.operator, result, tt.expected)
			}
		})
	}
}

func TestEvaluatorCreation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	store := NewMockAlertStore()
	manager := NewManager(store, logger)
	metrics := &mockMetricProvider{}

	rules := &RuleSet{
		Alerts: []RuleConfig{
			{
				Name:      "test_alert",
				Metric:    "db_size_mb",
				Threshold: 100,
				Operator:  ">",
				Severity:  "warning",
				Enabled:   true,
				Channels:  []string{"slack"},
			},
		},
		Evaluation: EvaluationConfig{
			Interval:  "60s",
			Retention: "7d",
		},
	}

	evaluator, err := NewEvaluator(manager, rules, metrics, logger)
	if err != nil {
		t.Fatalf("NewEvaluator failed: %v", err)
	}

	if evaluator.interval != 60*time.Second {
		t.Errorf("Expected interval 60s, got %v", evaluator.interval)
	}
}

func TestEvaluatorInvalidInterval(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	store := NewMockAlertStore()
	manager := NewManager(store, logger)
	metrics := &mockMetricProvider{}

	rules := &RuleSet{
		Evaluation: EvaluationConfig{
			Interval:  "invalid",
			Retention: "7d",
		},
	}

	_, err := NewEvaluator(manager, rules, metrics, logger)
	if err == nil {
		t.Error("Expected error for invalid interval, got nil")
	}
}

func TestEvaluateOnce(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	store := NewMockAlertStore()
	manager := NewManager(store, logger)

	// Set metric to exceed threshold
	metrics := &mockMetricProvider{
		dbSize: 120.0, // Above threshold of 100
	}

	rules := &RuleSet{
		Alerts: []RuleConfig{
			{
				Name:        "test_db_alert",
				Description: "DB size test",
				Metric:      "db_size_mb",
				Threshold:   100.0,
				Operator:    ">",
				Severity:    "warning",
				Enabled:     true,
				Channels:    []string{"slack"},
			},
		},
		Evaluation: EvaluationConfig{
			Interval:  "60s",
			Retention: "7d",
		},
	}

	evaluator, err := NewEvaluator(manager, rules, metrics, logger)
	if err != nil {
		t.Fatalf("NewEvaluator failed: %v", err)
	}

	// Run evaluation
	ctx := context.Background()
	if err := evaluator.EvaluateOnce(ctx); err != nil {
		t.Fatalf("EvaluateOnce failed: %v", err)
	}

	// Check that alert was fired
	activeAlerts, err := manager.GetActive(ctx)
	if err != nil {
		t.Fatalf("GetActive failed: %v", err)
	}

	if len(activeAlerts) != 1 {
		t.Errorf("Expected 1 active alert, got %d", len(activeAlerts))
	}

	if len(activeAlerts) > 0 && activeAlerts[0].AlertName != "test_db_alert" {
		t.Errorf("Expected alert name 'test_db_alert', got '%s'", activeAlerts[0].AlertName)
	}
}

func TestEvaluateOnceResolution(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	store := NewMockAlertStore()
	manager := NewManager(store, logger)

	// Start with metric above threshold
	metrics := &mockMetricProvider{
		memory: 600.0, // Above threshold of 500
	}

	rules := &RuleSet{
		Alerts: []RuleConfig{
			{
				Name:        "test_memory_alert",
				Description: "Memory usage test",
				Metric:      "memory_mb",
				Threshold:   500.0,
				Operator:    ">",
				Severity:    "warning",
				Enabled:     true,
				Channels:    []string{"slack"},
			},
		},
		Evaluation: EvaluationConfig{
			Interval:  "60s",
			Retention: "7d",
		},
	}

	evaluator, err := NewEvaluator(manager, rules, metrics, logger)
	if err != nil {
		t.Fatalf("NewEvaluator failed: %v", err)
	}

	ctx := context.Background()

	// First evaluation - should fire
	if err := evaluator.EvaluateOnce(ctx); err != nil {
		t.Fatalf("First EvaluateOnce failed: %v", err)
	}

	activeAlerts, err := manager.GetActive(ctx)
	if err != nil {
		t.Fatalf("GetActive failed: %v", err)
	}

	if len(activeAlerts) != 1 {
		t.Fatalf("Expected 1 active alert after first eval, got %d", len(activeAlerts))
	}

	// Reduce metric below threshold
	metrics.memory = 400.0

	// Second evaluation - should resolve
	if err := evaluator.EvaluateOnce(ctx); err != nil {
		t.Fatalf("Second EvaluateOnce failed: %v", err)
	}

	activeAlerts, err = manager.GetActive(ctx)
	if err != nil {
		t.Fatalf("GetActive failed after resolution: %v", err)
	}

	if len(activeAlerts) != 0 {
		t.Errorf("Expected 0 active alerts after resolution, got %d", len(activeAlerts))
	}
}

func TestEvaluateDisabledRules(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	store := NewMockAlertStore()
	manager := NewManager(store, logger)

	metrics := &mockMetricProvider{
		cost: 20.0, // Above threshold of 10
	}

	rules := &RuleSet{
		Alerts: []RuleConfig{
			{
				Name:        "disabled_alert",
				Description: "This alert is disabled",
				Metric:      "cost_today_usd",
				Threshold:   10.0,
				Operator:    ">",
				Severity:    "warning",
				Enabled:     false, // DISABLED
				Channels:    []string{}, // Empty is ok for disabled
			},
		},
		Evaluation: EvaluationConfig{
			Interval:  "60s",
			Retention: "7d",
		},
	}

	evaluator, err := NewEvaluator(manager, rules, metrics, logger)
	if err != nil {
		t.Fatalf("NewEvaluator failed: %v", err)
	}

	ctx := context.Background()
	if err := evaluator.EvaluateOnce(ctx); err != nil {
		t.Fatalf("EvaluateOnce failed: %v", err)
	}

	// Disabled rule should not fire even if threshold breached
	activeAlerts, err := manager.GetActive(ctx)
	if err != nil {
		t.Fatalf("GetActive failed: %v", err)
	}

	if len(activeAlerts) != 0 {
		t.Errorf("Expected 0 active alerts for disabled rule, got %d", len(activeAlerts))
	}
}
