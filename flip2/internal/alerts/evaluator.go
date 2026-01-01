package alerts

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// Evaluator runs periodic rule evaluation
type Evaluator struct {
	manager  *Manager
	rules    *RuleSet
	metrics  MetricProvider
	logger   *slog.Logger
	interval time.Duration
	stop     chan struct{}
	running  bool
}

// NewEvaluator creates a new rule evaluator
func NewEvaluator(manager *Manager, rules *RuleSet, metrics MetricProvider, logger *slog.Logger) (*Evaluator, error) {
	if logger == nil {
		logger = slog.Default()
	}

	// Parse evaluation interval
	interval, err := time.ParseDuration(rules.Evaluation.Interval)
	if err != nil {
		return nil, fmt.Errorf("invalid evaluation interval '%s': %w", rules.Evaluation.Interval, err)
	}

	return &Evaluator{
		manager:  manager,
		rules:    rules,
		metrics:  metrics,
		logger:   logger.WithGroup("evaluator"),
		interval: interval,
		stop:     make(chan struct{}),
		running:  false,
	}, nil
}

// Start begins the evaluation loop
func (e *Evaluator) Start() {
	if e.running {
		e.logger.Warn("Evaluator already running")
		return
	}

	e.running = true
	e.logger.Info("Starting alert evaluator",
		"interval", e.interval,
		"rules_count", len(e.rules.Alerts),
	)

	go e.run()
}

// Stop halts the evaluation loop
func (e *Evaluator) Stop() {
	if !e.running {
		return
	}

	e.logger.Info("Stopping alert evaluator")
	close(e.stop)
	e.running = false
}

// run is the main evaluation loop
func (e *Evaluator) run() {
	ticker := time.NewTicker(e.interval)
	defer ticker.Stop()

	// Run initial evaluation immediately
	ctx := context.Background()
	if err := e.EvaluateOnce(ctx); err != nil {
		e.logger.Error("Initial evaluation failed", "error", err)
	}

	for {
		select {
		case <-ticker.C:
			ctx := context.Background()
			if err := e.EvaluateOnce(ctx); err != nil {
				e.logger.Error("Evaluation cycle failed", "error", err)
			}

		case <-e.stop:
			e.logger.Info("Evaluation loop stopped")
			return
		}
	}
}

// EvaluateOnce runs one evaluation cycle
func (e *Evaluator) EvaluateOnce(ctx context.Context) error {
	e.logger.Debug("Starting evaluation cycle")

	var errCount int

	// Evaluate each rule
	for _, rule := range e.rules.Alerts {
		// Skip disabled rules
		if !rule.Enabled {
			continue
		}

		if err := e.evaluateRule(ctx, &rule); err != nil {
			e.logger.Error("Failed to evaluate rule",
				"rule_name", rule.Name,
				"error", err,
			)
			errCount++
			continue
		}
	}

	e.logger.Info("Evaluation cycle completed",
		"errors", errCount,
		"alerts_checked", len(e.rules.Alerts),
	)

	return nil
}

// evaluateRule checks a single rule against metrics
func (e *Evaluator) evaluateRule(ctx context.Context, rule *RuleConfig) error {
	// Get metric value
	value, err := e.getMetricValue(ctx, rule.Metric)
	if err != nil {
		return fmt.Errorf("failed to get metric '%s': %w", rule.Metric, err)
	}

	// Compare against threshold
	breached := compareValue(value, rule.Threshold, rule.Operator)

	e.logger.Debug("Evaluated rule",
		"rule_name", rule.Name,
		"metric", rule.Metric,
		"value", value,
		"threshold", rule.Threshold,
		"operator", rule.Operator,
		"breached", breached,
	)

	// Handle state transitions
	if breached {
		// Threshold breached - should we fire?
		if e.manager.ShouldFire(rule.Name) {
			// Fire new alert
			message := fmt.Sprintf("%s: %s (value: %.2f, threshold: %.2f)",
				rule.Severity,
				rule.Description,
				value,
				rule.Threshold,
			)

			metadata := map[string]interface{}{
				"metric":      rule.Metric,
				"operator":    rule.Operator,
				"channels":    rule.Channels,
				"actions":     rule.Actions,
			}

			alert, err := e.manager.Fire(ctx, rule.ToRule(), value, message, metadata)
			if err != nil {
				return fmt.Errorf("failed to fire alert: %w", err)
			}

			if alert != nil {
				e.logger.Warn("Alert FIRED",
					"alert_name", rule.Name,
					"severity", rule.Severity,
					"value", value,
					"threshold", rule.Threshold,
				)
			}
		}
	} else {
		// Threshold not breached - should we resolve?
		// Check if currently firing
		activeAlerts, err := e.manager.GetActive(ctx)
		if err != nil {
			return fmt.Errorf("failed to get active alerts: %w", err)
		}

		// Find if this alert is currently firing
		var isFiring bool
		for _, alert := range activeAlerts {
			if alert.AlertName == rule.Name {
				isFiring = true
				break
			}
		}

		if isFiring {
			// Resolve the alert
			if err := e.manager.Resolve(ctx, rule.Name); err != nil {
				return fmt.Errorf("failed to resolve alert: %w", err)
			}

			e.logger.Info("Alert RESOLVED",
				"alert_name", rule.Name,
				"value", value,
				"threshold", rule.Threshold,
			)
		}
	}

	return nil
}

// getMetricValue retrieves the current value for a metric
func (e *Evaluator) getMetricValue(ctx context.Context, metricName string) (float64, error) {
	switch metricName {
	case "db_size_mb":
		return e.metrics.GetDBSizeMB(ctx)

	case "error_rate_percent":
		return e.metrics.GetErrorRatePercent(ctx)

	case "sync_failed_count_1h":
		count, err := e.metrics.GetSyncFailedCount1H(ctx)
		return float64(count), err

	case "health_check_failed":
		count, err := e.metrics.GetHealthCheckFailed(ctx)
		return float64(count), err

	case "memory_mb":
		return e.metrics.GetMemoryMB(ctx)

	case "cost_today_usd":
		return e.metrics.GetCostTodayUSD(ctx)

	default:
		return 0, fmt.Errorf("unknown metric: %s", metricName)
	}
}

// compareValue checks if value meets threshold based on operator
func compareValue(value, threshold float64, operator string) bool {
	switch operator {
	case ">":
		return value > threshold
	case "<":
		return value < threshold
	case ">=":
		return value >= threshold
	case "<=":
		return value <= threshold
	case "==":
		return value == threshold
	default:
		// Should never happen if validation worked
		return false
	}
}
