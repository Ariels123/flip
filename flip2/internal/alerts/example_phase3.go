package alerts

import (
	"context"
	"log/slog"
	"os"
)

// ExamplePhase3Integration demonstrates complete Phase 1+2+3 integration
func ExamplePhase3Integration() {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Load rules from config
	rules, err := LoadRules("config/alerts.yaml")
	if err != nil {
		logger.Error("Failed to load rules", "error", err)
		return
	}

	// Create alert manager (Phase 1)
	// Note: In production, initialize with PocketBase:
	// store := NewPBAlertStore(app)
	// For this example, we'll skip manager creation
	// manager := NewManager(store, logger)

	// Create dispatcher (Phase 3)
	dispatcher, err := NewDispatcher(rules, logger)
	if err != nil {
		logger.Error("Failed to create dispatcher", "error", err)
		return
	}

	// In production, you would:
	// manager.SetDispatcher(dispatcher)
	//
	// Then when alerts fire:
	// alert, err := manager.Fire(ctx, rule, value, message, metadata)
	// The manager will automatically dispatch notifications

	// For this example, demonstrate direct dispatcher usage
	testAlert := &Alert{
		AlertName:   "disk_space_critical",
		Severity:    SeverityCritical,
		Message:     "Auxiliary DB exceeds size threshold",
		MetricValue: 52.3,
		Threshold:   50.0,
		Metadata: map[string]interface{}{
			"db_path": "/data/auxiliary.db",
		},
	}

	// Send notifications directly
	channels := []string{"slack", "email"}
	err = dispatcher.Dispatch(ctx, testAlert, channels)
	if err != nil {
		logger.Error("Failed to dispatch notifications", "error", err)
		return
	}

	logger.Info("Notifications sent successfully")
}

// ExampleDispatcherOnly demonstrates using dispatcher independently
func ExampleDispatcherOnly() {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Load rules
	rules, _ := LoadRules("config/alerts.yaml")

	// Create dispatcher
	dispatcher, _ := NewDispatcher(rules, logger)

	// Create a test alert
	alert := &Alert{
		ID:          "test-123",
		AlertName:   "test_alert",
		Severity:    SeverityCritical,
		Message:     "Test notification",
		MetricValue: 100.0,
		Threshold:   50.0,
	}

	// Send to specific channels
	channels := []string{"slack", "email"}
	err := dispatcher.Dispatch(ctx, alert, channels)
	if err != nil {
		logger.Error("Dispatch failed", "error", err)
	} else {
		logger.Info("Notifications sent successfully")
	}
}

// ExampleConfigurationValidation shows how to validate configuration
func ExampleConfigurationValidation() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Load and validate rules
	rules, err := LoadRules("config/alerts.yaml")
	if err != nil {
		logger.Error("Configuration error", "error", err)
		return
	}

	// Check which channels are enabled
	if rules.NotificationChannels.Slack.Enabled {
		logger.Info("Slack notifications enabled",
			"webhook_configured", rules.NotificationChannels.Slack.WebhookURL != "",
		)
	}

	if rules.NotificationChannels.Email.Enabled {
		logger.Info("Email notifications enabled",
			"smtp_host", rules.NotificationChannels.Email.SMTPHost,
			"recipients", len(rules.NotificationChannels.Email.To),
		)
	}

	// List all enabled alert rules
	for _, ruleConfig := range rules.Alerts {
		if ruleConfig.Enabled {
			logger.Info("Alert rule enabled",
				"name", ruleConfig.Name,
				"channels", ruleConfig.Channels,
				"severity", ruleConfig.Severity,
			)
		}
	}
}
