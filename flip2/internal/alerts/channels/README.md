# FLIP2 Alert Notification Channels

This package implements notification channels for the FLIP2 alerting system.

## Overview

The `channels` package provides pluggable notification backends for delivering alerts to operators. Currently supports:

- **Slack** - Webhook-based Slack notifications
- **Email** - SMTP email notifications

## Architecture

### Interface-Based Design

To avoid import cycles, this package defines a minimal `Alert` interface:

```go
type Alert interface {
    GetAlertName() string
    GetSeverity() string
    GetMessage() string
    GetMetricValue() float64
    GetThreshold() float64
    GetFiredAt() time.Time
    GetMetadata() map[string]interface{}
}
```

The parent `alerts.Alert` type implements this interface via getter methods.

## Channels

### Slack Channel

**Features:**
- Webhook-based (no API token required)
- Rich message formatting with attachments
- Color-coded by severity
- Includes all alert details
- 10-second HTTP timeout

**Configuration:**
```yaml
notification_channels:
  slack:
    enabled: true
    webhook_url: "${SLACK_WEBHOOK_URL}"
    default_channel: "#flip2-alerts"
```

**Message Format:**
```json
{
  "text": "🚨 critical: disk_space_critical",
  "attachments": [{
    "color": "danger",
    "fields": [
      {"title": "Message", "value": "Auxiliary DB exceeds size threshold"},
      {"title": "Current Value", "value": "52.30"},
      {"title": "Threshold", "value": "50"}
    ]
  }]
}
```

**Usage:**
```go
slack := channels.NewSlackChannel(webhookURL, "#alerts")
err := slack.Send(alert)
```

### Email Channel

**Features:**
- Standard SMTP
- Plain text format
- RFC-compliant headers
- Multiple recipients
- Metadata included

**Configuration:**
```yaml
notification_channels:
  email:
    enabled: true
    smtp_host: "localhost"
    smtp_port: 25
    from: "flip2@localhost"
    to:
      - "admin@localhost"
```

**Message Format:**
```
FLIP2 Alert: disk_space_critical
Severity: CRITICAL

Message: Auxiliary DB exceeds size threshold

Current Value: 52.30
Threshold: 50

Fired At: Tue, 31 Dec 2025 12:00:00 EST

---
This is an automated alert from FLIP2.
```

**Usage:**
```go
email := channels.NewEmailChannel(smtpHost, smtpPort, from, to)
err := email.Send(alert)
```

## Testing

### Unit Tests

All channels have comprehensive unit tests using mock servers:

```bash
go test ./internal/alerts/channels/... -v
```

**Slack Tests:**
- Uses `httptest.NewServer` for webhook mocking
- Verifies JSON payload structure
- Tests color coding by severity
- Tests error handling

**Email Tests:**
- Tests message formatting
- Tests header structure
- Tests error validation
- No actual SMTP calls

### Integration Testing

For manual integration testing:

```bash
# Set environment variables
export SLACK_WEBHOOK_URL="https://hooks.slack.com/services/YOUR/WEBHOOK"
export SMTP_HOST="localhost"

# Run local SMTP server (mailhog)
docker run -d -p 1025:1025 -p 8025:8025 mailhog/mailhog

# Send test notification
go run cmd/test-alert/main.go
```

## Error Handling

Both channels handle errors gracefully:

- **Network failures** - Return error, logged by dispatcher
- **Configuration errors** - Return error at send time
- **Timeouts** - 10s for Slack, configurable for SMTP
- **Invalid responses** - Return descriptive errors

Errors don't crash the system - they're logged and the alert is still stored in the database.

## Adding New Channels

To add a new notification channel:

1. Create new file `internal/alerts/channels/newchannel.go`
2. Implement a struct with `Send(Alert) error` method
3. Update `dispatcher.go` to initialize and route to new channel
4. Add configuration to `alerts.yaml`
5. Add tests

Example:

```go
package channels

type NewChannel struct {
    config string
}

func NewNewChannel(config string) *NewChannel {
    return &NewChannel{config: config}
}

func (n *NewChannel) Send(alert Alert) error {
    // Implementation
    return nil
}
```

## Performance

- **Slack**: ~100ms per notification (HTTP POST)
- **Email**: ~50-500ms depending on SMTP server
- **Overhead**: < 1ms routing logic

Notifications are sent sequentially per alert.

## Security

- Webhook URLs stored in environment variables
- SMTP authentication supported (field exists)
- No secrets in configuration files
- HTTPS webhooks recommended

## License

Part of FLIP2 project.
