package alerts

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"
)

// mockChannel is a test channel that records send calls
type mockChannel struct {
	sendCalled bool
	sendError  error
	sentAlerts []*Alert
}

func (m *mockChannel) Send(alert *Alert) error {
	m.sendCalled = true
	m.sentAlerts = append(m.sentAlerts, alert)
	return m.sendError
}

func TestNewDispatcher(t *testing.T) {
	rules := &RuleSet{
		NotificationChannels: NotificationConfig{
			Slack: SlackConfig{
				Enabled:        true,
				WebhookURL:     "https://hooks.slack.com/test",
				DefaultChannel: "#alerts",
			},
			Email: EmailConfig{
				Enabled:  true,
				SMTPHost: "localhost",
				SMTPPort: 25,
				From:     "flip2@localhost",
				To:       []string{"admin@localhost"},
			},
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	dispatcher, err := NewDispatcher(rules, logger)

	if err != nil {
		t.Fatalf("Failed to create dispatcher: %v", err)
	}

	if dispatcher.slackChannel == nil {
		t.Error("Slack channel should be initialized")
	}

	if dispatcher.emailChannel == nil {
		t.Error("Email channel should be initialized")
	}
}

func TestNewDispatcher_DisabledChannels(t *testing.T) {
	rules := &RuleSet{
		NotificationChannels: NotificationConfig{
			Slack: SlackConfig{Enabled: false},
			Email: EmailConfig{Enabled: false},
		},
	}

	dispatcher, err := NewDispatcher(rules, nil)

	if err != nil {
		t.Fatalf("Failed to create dispatcher: %v", err)
	}

	if dispatcher.slackChannel != nil {
		t.Error("Slack channel should not be initialized when disabled")
	}

	if dispatcher.emailChannel != nil {
		t.Error("Email channel should not be initialized when disabled")
	}
}

func TestDispatcher_NoChannels(t *testing.T) {
	rules := &RuleSet{
		NotificationChannels: NotificationConfig{
			Slack: SlackConfig{Enabled: false},
			Email: EmailConfig{Enabled: false},
		},
	}

	dispatcher, _ := NewDispatcher(rules, nil)

	alert := &Alert{
		AlertName: "test",
		Severity:  SeverityCritical,
		Message:   "Test alert",
		FiredAt:   time.Now(),
	}

	ctx := context.Background()
	err := dispatcher.Dispatch(ctx, alert, []string{})

	if err != nil {
		t.Errorf("Dispatch should not error with no channels: %v", err)
	}
}

func TestDispatcher_NilAlert(t *testing.T) {
	rules := &RuleSet{
		NotificationChannels: NotificationConfig{},
	}

	dispatcher, _ := NewDispatcher(rules, nil)

	ctx := context.Background()
	err := dispatcher.Dispatch(ctx, nil, []string{"slack"})

	if err == nil {
		t.Error("Expected error when dispatching nil alert")
	}
}

func TestDispatcher_UnknownChannel(t *testing.T) {
	rules := &RuleSet{
		NotificationChannels: NotificationConfig{
			Slack: SlackConfig{Enabled: false},
			Email: EmailConfig{Enabled: false},
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	dispatcher, _ := NewDispatcher(rules, logger)

	alert := &Alert{
		AlertName: "test",
		Severity:  SeverityCritical,
		Message:   "Test alert",
		FiredAt:   time.Now(),
	}

	ctx := context.Background()
	err := dispatcher.Dispatch(ctx, alert, []string{"unknown_channel"})

	// Should not error, just log warning
	if err != nil {
		t.Errorf("Dispatch should not error for unknown channel: %v", err)
	}
}

func TestDispatcher_ChannelNotConfigured(t *testing.T) {
	rules := &RuleSet{
		NotificationChannels: NotificationConfig{
			Slack: SlackConfig{Enabled: false},
			Email: EmailConfig{Enabled: false},
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	dispatcher, _ := NewDispatcher(rules, logger)

	alert := &Alert{
		AlertName: "test",
		Severity:  SeverityCritical,
		Message:   "Test alert",
		FiredAt:   time.Now(),
	}

	ctx := context.Background()

	// Try to send to slack when it's not configured
	err := dispatcher.Dispatch(ctx, alert, []string{"slack"})

	// Should not error, just log warning
	if err != nil {
		t.Errorf("Dispatch should not error for unconfigured channel: %v", err)
	}
}

func TestDispatchResolution(t *testing.T) {
	rules := &RuleSet{
		NotificationChannels: NotificationConfig{
			Slack: SlackConfig{Enabled: false},
		},
	}

	dispatcher, _ := NewDispatcher(rules, nil)

	alert := &Alert{
		AlertName: "test",
		Severity:  SeverityCritical,
		Message:   "Test alert",
		FiredAt:   time.Now(),
	}

	ctx := context.Background()
	err := dispatcher.DispatchResolution(ctx, alert, []string{})

	if err != nil {
		t.Errorf("DispatchResolution should not error: %v", err)
	}
}

func TestDispatchResolution_NilAlert(t *testing.T) {
	rules := &RuleSet{
		NotificationChannels: NotificationConfig{},
	}

	dispatcher, _ := NewDispatcher(rules, nil)

	ctx := context.Background()
	err := dispatcher.DispatchResolution(ctx, nil, []string{"slack"})

	if err == nil {
		t.Error("Expected error when dispatching resolution for nil alert")
	}
}
