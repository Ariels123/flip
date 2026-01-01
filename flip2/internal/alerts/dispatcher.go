package alerts

import (
	"context"
	"fmt"
	"log/slog"

	"flip2/internal/alerts/channels"
)

// Channel represents a notification channel interface
type Channel interface {
	Send(alert *Alert) error
}

// Dispatcher routes alerts to notification channels
type Dispatcher struct {
	slackChannel *channels.SlackChannel
	emailChannel *channels.EmailChannel
	logger       *slog.Logger
}

// NewDispatcher creates a new notification dispatcher
func NewDispatcher(rules *RuleSet, logger *slog.Logger) (*Dispatcher, error) {
	if logger == nil {
		logger = slog.Default()
	}

	d := &Dispatcher{
		logger: logger.WithGroup("dispatcher"),
	}

	// Initialize Slack channel if configured
	if rules.NotificationChannels.Slack.Enabled {
		d.slackChannel = channels.NewSlackChannel(
			rules.NotificationChannels.Slack.WebhookURL,
			rules.NotificationChannels.Slack.DefaultChannel,
		)
		d.logger.Info("Slack channel initialized",
			"webhook_configured", rules.NotificationChannels.Slack.WebhookURL != "",
			"default_channel", rules.NotificationChannels.Slack.DefaultChannel,
		)
	} else {
		d.logger.Info("Slack channel disabled")
	}

	// Initialize Email channel if configured
	if rules.NotificationChannels.Email.Enabled {
		d.emailChannel = channels.NewEmailChannel(
			rules.NotificationChannels.Email.SMTPHost,
			rules.NotificationChannels.Email.SMTPPort,
			rules.NotificationChannels.Email.From,
			rules.NotificationChannels.Email.To,
		)
		d.logger.Info("Email channel initialized",
			"smtp_host", rules.NotificationChannels.Email.SMTPHost,
			"smtp_port", rules.NotificationChannels.Email.SMTPPort,
			"recipients", len(rules.NotificationChannels.Email.To),
		)
	} else {
		d.logger.Info("Email channel disabled")
	}

	return d, nil
}

// Dispatch sends alert to configured channels
func (d *Dispatcher) Dispatch(ctx context.Context, alert *Alert, channelNames []string) error {
	if alert == nil {
		return fmt.Errorf("alert is nil")
	}

	if len(channelNames) == 0 {
		d.logger.Warn("No channels specified for alert", "alert_name", alert.AlertName)
		return nil
	}

	var errors []error
	successCount := 0

	for _, channelName := range channelNames {
		var err error

		switch channelName {
		case "slack":
			if d.slackChannel != nil {
				err = d.slackChannel.Send(alert)
			} else {
				d.logger.Warn("Slack channel not configured", "alert_name", alert.AlertName)
				continue
			}

		case "email":
			if d.emailChannel != nil {
				err = d.emailChannel.Send(alert)
			} else {
				d.logger.Warn("Email channel not configured", "alert_name", alert.AlertName)
				continue
			}

		default:
			d.logger.Warn("Unknown channel", "channel", channelName, "alert_name", alert.AlertName)
			continue
		}

		// Log result
		if err != nil {
			d.logger.Error("Failed to send notification",
				"channel", channelName,
				"alert_name", alert.AlertName,
				"severity", alert.Severity,
				"error", err,
			)
			errors = append(errors, fmt.Errorf("%s: %w", channelName, err))
		} else {
			d.logger.Info("Notification sent successfully",
				"channel", channelName,
				"alert_name", alert.AlertName,
				"severity", alert.Severity,
			)
			successCount++
		}
	}

	// Return error if all channels failed
	if len(errors) > 0 {
		if successCount == 0 {
			return fmt.Errorf("all notifications failed: %v", errors)
		}
		// Partial success - log but don't fail
		d.logger.Warn("Some notifications failed",
			"total", len(channelNames),
			"success", successCount,
			"failed", len(errors),
			"errors", errors,
		)
	}

	return nil
}

// DispatchResolution sends alert resolution notification to configured channels
func (d *Dispatcher) DispatchResolution(ctx context.Context, alert *Alert, channelNames []string) error {
	if alert == nil {
		return fmt.Errorf("alert is nil")
	}

	// Create a copy of the alert with modified message for resolution
	resolvedAlert := *alert
	resolvedAlert.Message = fmt.Sprintf("✅ RESOLVED: %s", alert.Message)

	return d.Dispatch(ctx, &resolvedAlert, channelNames)
}
