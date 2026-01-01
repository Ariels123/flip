package channels

import (
	"strings"
	"testing"
	"time"
)

func TestEmailChannel_FormatMessage(t *testing.T) {
	email := NewEmailChannel("localhost", 25, "from@test.com", []string{"to@test.com"})

	alert := &mockAlert{
		alertName:   "disk_space_critical",
		severity:    "critical",
		message:     "Auxiliary DB exceeds size threshold",
		metricValue: 52.3,
		threshold:   50.0,
		firedAt:     time.Now(),
		metadata: map[string]interface{}{
			"disk_path": "/data/auxiliary.db",
		},
	}

	body := email.formatMessage(alert)

	// Verify alert name in body
	if !strings.Contains(body, "disk_space_critical") {
		t.Error("Body should contain alert name")
	}

	// Verify severity
	if !strings.Contains(body, "CRITICAL") {
		t.Error("Body should contain severity")
	}

	// Verify message
	if !strings.Contains(body, "Auxiliary DB exceeds size threshold") {
		t.Error("Body should contain alert message")
	}

	// Verify values
	if !strings.Contains(body, "52.30") {
		t.Error("Body should contain metric value")
	}
	if !strings.Contains(body, "50") {
		t.Error("Body should contain threshold")
	}

	// Verify metadata
	if !strings.Contains(body, "disk_path") {
		t.Error("Body should contain metadata key")
	}
	if !strings.Contains(body, "/data/auxiliary.db") {
		t.Error("Body should contain metadata value")
	}

	// Verify footer
	if !strings.Contains(body, "automated alert from FLIP2") {
		t.Error("Body should contain footer")
	}
}

func TestFormatEmailMessage(t *testing.T) {
	from := "flip2@localhost"
	to := []string{"admin@localhost", "ops@localhost"}
	subject := "[FLIP2] CRITICAL: test_alert"
	body := "Test body"

	msg := formatEmailMessage(from, to, subject, body)

	// Verify headers
	if !strings.Contains(msg, "From: flip2@localhost") {
		t.Error("Message should contain From header")
	}
	if !strings.Contains(msg, "To: admin@localhost, ops@localhost") {
		t.Error("Message should contain To header with all recipients")
	}
	if !strings.Contains(msg, "Subject: [FLIP2] CRITICAL: test_alert") {
		t.Error("Message should contain Subject header")
	}
	if !strings.Contains(msg, "MIME-Version: 1.0") {
		t.Error("Message should contain MIME-Version header")
	}
	if !strings.Contains(msg, "Content-Type: text/plain; charset=utf-8") {
		t.Error("Message should contain Content-Type header")
	}

	// Verify body
	if !strings.Contains(msg, "Test body") {
		t.Error("Message should contain body")
	}

	// Verify header/body separator
	if !strings.Contains(msg, "\r\n\r\n") {
		t.Error("Message should have proper header/body separator")
	}
}

func TestEmailChannel_NoRecipients(t *testing.T) {
	email := NewEmailChannel("localhost", 25, "from@test.com", []string{})

	alert := &mockAlert{
		alertName: "test",
		severity:  "info",
		message:   "Test",
		firedAt:   time.Now(),
	}

	err := email.Send(alert)
	if err == nil {
		t.Error("Expected error when no recipients configured")
	}
	if !strings.Contains(err.Error(), "no email recipients") {
		t.Errorf("Expected 'no email recipients' error, got: %v", err)
	}
}

func TestEmailChannel_NoSMTPHost(t *testing.T) {
	email := NewEmailChannel("", 25, "from@test.com", []string{"to@test.com"})

	alert := &mockAlert{
		alertName: "test",
		severity:  "info",
		message:   "Test",
		firedAt:   time.Now(),
	}

	err := email.Send(alert)
	if err == nil {
		t.Error("Expected error when SMTP host is empty")
	}
	if !strings.Contains(err.Error(), "smtp host not configured") {
		t.Errorf("Expected 'smtp host not configured' error, got: %v", err)
	}
}

func TestFormatEmailValue(t *testing.T) {
	tests := []struct {
		input    float64
		expected string
	}{
		{50.0, "50"},
		{52.3, "52.30"},
		{100.0, "100"},
		{0.5, "0.50"},
		{99.99, "99.99"},
	}

	for _, tc := range tests {
		result := formatEmailValue(tc.input)
		if result != tc.expected {
			t.Errorf("formatEmailValue(%v) = %s, expected %s", tc.input, result, tc.expected)
		}
	}
}

func TestEmailChannel_WarningAlert(t *testing.T) {
	email := NewEmailChannel("localhost", 25, "from@test.com", []string{"to@test.com"})

	alert := &mockAlert{
		alertName:   "high_memory_usage",
		severity:    "warning",
		message:     "Memory usage is high",
		metricValue: 450.0,
		threshold:   400.0,
		firedAt:     time.Now(),
	}

	body := email.formatMessage(alert)

	if !strings.Contains(body, "WARNING") {
		t.Error("Warning alert should contain WARNING in severity")
	}
}

func TestEmailChannel_InfoAlert(t *testing.T) {
	email := NewEmailChannel("localhost", 25, "from@test.com", []string{"to@test.com"})

	alert := &mockAlert{
		alertName:   "info_alert",
		severity:    "info",
		message:     "Informational message",
		metricValue: 10.0,
		threshold:   5.0,
		firedAt:     time.Now(),
	}

	body := email.formatMessage(alert)

	if !strings.Contains(body, "INFO") {
		t.Error("Info alert should contain INFO in severity")
	}
}
