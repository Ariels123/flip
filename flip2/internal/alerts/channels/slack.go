package channels

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Alert interface represents the minimal alert data needed for notifications
type Alert interface {
	GetAlertName() string
	GetSeverity() string
	GetMessage() string
	GetMetricValue() float64
	GetThreshold() float64
	GetFiredAt() time.Time
	GetMetadata() map[string]interface{}
}

// SlackChannel sends alerts to Slack via webhook
type SlackChannel struct {
	webhookURL     string
	defaultChannel string
	httpClient     *http.Client
}

// NewSlackChannel creates a new Slack notification channel
func NewSlackChannel(webhookURL, defaultChannel string) *SlackChannel {
	return &SlackChannel{
		webhookURL:     webhookURL,
		defaultChannel: defaultChannel,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Send publishes alert to Slack
func (s *SlackChannel) Send(alert Alert) error {
	if s.webhookURL == "" {
		return fmt.Errorf("slack webhook URL not configured")
	}

	message := s.formatMessage(alert)

	payload, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal slack message: %w", err)
	}

	resp, err := s.httpClient.Post(s.webhookURL, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("slack webhook request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("slack webhook returned status %d", resp.StatusCode)
	}

	return nil
}

// formatMessage creates Slack message payload with attachments
func (s *SlackChannel) formatMessage(alert Alert) map[string]interface{} {
	severity := alert.GetSeverity()

	// Choose emoji based on severity
	emoji := "ℹ️"
	color := "good" // green
	switch severity {
	case "warning":
		emoji = "⚠️"
		color = "warning" // yellow
	case "critical":
		emoji = "🚨"
		color = "danger" // red
	}

	// Create main text
	text := fmt.Sprintf("%s %s: %s", emoji, severity, alert.GetAlertName())

	// Create attachment fields
	fields := []map[string]interface{}{
		{
			"title": "Message",
			"value": alert.GetMessage(),
			"short": false,
		},
		{
			"title": "Current Value",
			"value": formatValue(alert.GetMetricValue()),
			"short": true,
		},
		{
			"title": "Threshold",
			"value": formatValue(alert.GetThreshold()),
			"short": true,
		},
		{
			"title": "Fired At",
			"value": alert.GetFiredAt().Format("2006-01-02 15:04:05"),
			"short": false,
		},
	}

	// Add metadata fields if present
	metadata := alert.GetMetadata()
	if len(metadata) > 0 {
		for key, value := range metadata {
			fields = append(fields, map[string]interface{}{
				"title": key,
				"value": fmt.Sprintf("%v", value),
				"short": true,
			})
		}
	}

	// Create Slack message structure
	return map[string]interface{}{
		"text": text,
		"attachments": []map[string]interface{}{
			{
				"color":  color,
				"fields": fields,
			},
		},
	}
}

// formatValue formats a float64 value for display
func formatValue(value float64) string {
	// For whole numbers, don't show decimal places
	if value == float64(int64(value)) {
		return fmt.Sprintf("%.0f", value)
	}
	// For fractional numbers, show up to 2 decimal places
	return fmt.Sprintf("%.2f", value)
}
