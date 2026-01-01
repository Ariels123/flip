package alerts

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadRules(t *testing.T) {
	// Create a temporary test config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test_alerts.yaml")

	validConfig := `
alerts:
  - name: test_alert
    description: Test alert
    metric: db_size_mb
    threshold: 100
    operator: ">"
    severity: warning
    enabled: true
    channels:
      - slack

notification_channels:
  slack:
    enabled: true
    webhook_url: "https://example.com/webhook"
    default_channel: "#test"
  email:
    enabled: false
    smtp_host: "smtp.example.com"
    smtp_port: 587
    from: "test@example.com"
    to:
      - "admin@example.com"

evaluation:
  interval: 60s
  retention: 7d
`

	if err := os.WriteFile(configPath, []byte(validConfig), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Test loading valid config
	rules, err := LoadRules(configPath)
	if err != nil {
		t.Fatalf("LoadRules failed: %v", err)
	}

	if len(rules.Alerts) != 1 {
		t.Errorf("Expected 1 alert, got %d", len(rules.Alerts))
	}

	if rules.Alerts[0].Name != "test_alert" {
		t.Errorf("Expected alert name 'test_alert', got '%s'", rules.Alerts[0].Name)
	}

	if rules.Evaluation.Interval != "60s" {
		t.Errorf("Expected interval '60s', got '%s'", rules.Evaluation.Interval)
	}
}

func TestRuleValidation(t *testing.T) {
	tests := []struct {
		name        string
		rule        RuleConfig
		expectError bool
	}{
		{
			name: "valid rule",
			rule: RuleConfig{
				Name:      "test",
				Metric:    "db_size_mb",
				Threshold: 100,
				Operator:  ">",
				Severity:  "warning",
				Enabled:   true,
				Channels:  []string{"slack"},
			},
			expectError: false,
		},
		{
			name: "invalid operator",
			rule: RuleConfig{
				Name:      "test",
				Metric:    "db_size_mb",
				Threshold: 100,
				Operator:  "!=",
				Severity:  "warning",
				Enabled:   true,
				Channels:  []string{"slack"},
			},
			expectError: true,
		},
		{
			name: "invalid severity",
			rule: RuleConfig{
				Name:      "test",
				Metric:    "db_size_mb",
				Threshold: 100,
				Operator:  ">",
				Severity:  "super_critical",
				Enabled:   true,
				Channels:  []string{"slack"},
			},
			expectError: true,
		},
		{
			name: "invalid metric",
			rule: RuleConfig{
				Name:      "test",
				Metric:    "invalid_metric",
				Threshold: 100,
				Operator:  ">",
				Severity:  "warning",
				Enabled:   true,
				Channels:  []string{"slack"},
			},
			expectError: true,
		},
		{
			name: "missing name",
			rule: RuleConfig{
				Metric:    "db_size_mb",
				Threshold: 100,
				Operator:  ">",
				Severity:  "warning",
				Enabled:   true,
				Channels:  []string{"slack"},
			},
			expectError: true,
		},
		{
			name: "enabled without channels",
			rule: RuleConfig{
				Name:      "test",
				Metric:    "db_size_mb",
				Threshold: 100,
				Operator:  ">",
				Severity:  "warning",
				Enabled:   true,
				Channels:  []string{},
			},
			expectError: true,
		},
		{
			name: "disabled without channels (ok)",
			rule: RuleConfig{
				Name:      "test",
				Metric:    "db_size_mb",
				Threshold: 100,
				Operator:  ">",
				Severity:  "warning",
				Enabled:   false,
				Channels:  []string{},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.rule.Validate()
			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestEnvVarExpansion(t *testing.T) {
	// Set test environment variable
	os.Setenv("TEST_WEBHOOK", "https://hooks.slack.com/test")
	defer os.Unsetenv("TEST_WEBHOOK")

	input := `webhook_url: "${TEST_WEBHOOK}"`
	expected := `webhook_url: "https://hooks.slack.com/test"`

	result := expandEnvVars(input)
	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}

	// Test missing env var (should keep original)
	input = `webhook_url: "${MISSING_VAR}"`
	expected = `webhook_url: "${MISSING_VAR}"`

	result = expandEnvVars(input)
	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}
}

func TestToRule(t *testing.T) {
	config := RuleConfig{
		Name:        "test_rule",
		Description: "Test description",
		Metric:      "db_size_mb",
		Threshold:   100.5,
		Operator:    ">",
		Severity:    "warning",
		Enabled:     true,
		Channels:    []string{"slack", "email"},
		Actions:     []string{"restart"},
	}

	rule := config.ToRule()

	if rule.Name != config.Name {
		t.Errorf("Name mismatch: expected '%s', got '%s'", config.Name, rule.Name)
	}
	if rule.Threshold != config.Threshold {
		t.Errorf("Threshold mismatch: expected %.2f, got %.2f", config.Threshold, rule.Threshold)
	}
	if string(rule.Severity) != config.Severity {
		t.Errorf("Severity mismatch: expected '%s', got '%s'", config.Severity, rule.Severity)
	}
}
