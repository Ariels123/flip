package channels

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// mockAlert implements the Alert interface for testing
type mockAlert struct {
	alertName   string
	severity    string
	message     string
	metricValue float64
	threshold   float64
	firedAt     time.Time
	metadata    map[string]interface{}
}

func (m *mockAlert) GetAlertName() string                  { return m.alertName }
func (m *mockAlert) GetSeverity() string                   { return m.severity }
func (m *mockAlert) GetMessage() string                    { return m.message }
func (m *mockAlert) GetMetricValue() float64               { return m.metricValue }
func (m *mockAlert) GetThreshold() float64                 { return m.threshold }
func (m *mockAlert) GetFiredAt() time.Time                 { return m.firedAt }
func (m *mockAlert) GetMetadata() map[string]interface{}   { return m.metadata }

func TestSlackChannel_Send(t *testing.T) {
	// Create a test server to receive webhook
	var receivedPayload map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}

		if err := json.NewDecoder(r.Body).Decode(&receivedPayload); err != nil {
			t.Errorf("Failed to decode payload: %v", err)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create Slack channel with test server URL
	slack := NewSlackChannel(server.URL, "#test-channel")

	// Create test alert
	alert := &mockAlert{
		alertName:   "test_alert",
		severity:    "critical",
		message:     "Test alert message",
		metricValue: 52.3,
		threshold:   50.0,
		firedAt:     time.Now(),
		metadata: map[string]interface{}{
			"test_key": "test_value",
		},
	}

	// Send alert
	if err := slack.Send(alert); err != nil {
		t.Fatalf("Failed to send alert: %v", err)
	}

	// Verify payload structure
	if receivedPayload == nil {
		t.Fatal("No payload received")
	}

	// Check text field
	if text, ok := receivedPayload["text"].(string); !ok || text == "" {
		t.Errorf("Missing or empty text field")
	}

	// Check attachments
	attachments, ok := receivedPayload["attachments"].([]interface{})
	if !ok || len(attachments) == 0 {
		t.Fatal("Missing or empty attachments")
	}

	// Verify first attachment has fields
	attachment := attachments[0].(map[string]interface{})
	if fields, ok := attachment["fields"].([]interface{}); !ok || len(fields) == 0 {
		t.Errorf("Missing or empty fields in attachment")
	}

	// Verify color is set
	if color, ok := attachment["color"].(string); !ok || color != "danger" {
		t.Errorf("Expected color 'danger' for critical alert, got %v", color)
	}
}

func TestSlackChannel_SendWarning(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]interface{}
		json.NewDecoder(r.Body).Decode(&payload)

		attachments := payload["attachments"].([]interface{})
		attachment := attachments[0].(map[string]interface{})

		// Verify warning color
		if color := attachment["color"].(string); color != "warning" {
			t.Errorf("Expected color 'warning', got %s", color)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	slack := NewSlackChannel(server.URL, "#test")

	alert := &mockAlert{
		alertName:   "warning_alert",
		severity:    "warning",
		message:     "Warning message",
		metricValue: 35.0,
		threshold:   30.0,
		firedAt:     time.Now(),
	}

	if err := slack.Send(alert); err != nil {
		t.Fatalf("Failed to send warning alert: %v", err)
	}
}

func TestSlackChannel_NoWebhookURL(t *testing.T) {
	slack := NewSlackChannel("", "#test")

	alert := &mockAlert{
		alertName: "test",
		severity:  "info",
		message:   "Test",
		firedAt:   time.Now(),
	}

	err := slack.Send(alert)
	if err == nil {
		t.Error("Expected error when webhook URL is empty")
	}
}

func TestSlackChannel_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	slack := NewSlackChannel(server.URL, "#test")

	alert := &mockAlert{
		alertName: "test",
		severity:  "info",
		message:   "Test",
		firedAt:   time.Now(),
	}

	err := slack.Send(alert)
	if err == nil {
		t.Error("Expected error when server returns 500")
	}
}

func TestFormatValue(t *testing.T) {
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
		result := formatValue(tc.input)
		if result != tc.expected {
			t.Errorf("formatValue(%v) = %s, expected %s", tc.input, result, tc.expected)
		}
	}
}
