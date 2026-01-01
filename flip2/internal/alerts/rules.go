package alerts

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// RuleSet contains all loaded alert rules
type RuleSet struct {
	Alerts              []RuleConfig         `yaml:"alerts"`
	NotificationChannels NotificationConfig   `yaml:"notification_channels"`
	Evaluation          EvaluationConfig     `yaml:"evaluation"`
}

// RuleConfig represents a single alert rule from configuration
type RuleConfig struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Metric      string   `yaml:"metric"`
	Threshold   float64  `yaml:"threshold"`
	Operator    string   `yaml:"operator"`
	Severity    string   `yaml:"severity"`
	Enabled     bool     `yaml:"enabled"`
	Channels    []string `yaml:"channels"`
	Actions     []string `yaml:"actions,omitempty"`
}

// NotificationConfig holds notification channel settings
type NotificationConfig struct {
	Slack SlackConfig `yaml:"slack"`
	Email EmailConfig `yaml:"email"`
}

// SlackConfig contains Slack webhook configuration
type SlackConfig struct {
	Enabled        bool   `yaml:"enabled"`
	WebhookURL     string `yaml:"webhook_url"`
	DefaultChannel string `yaml:"default_channel"`
}

// EmailConfig contains SMTP email configuration
type EmailConfig struct {
	Enabled  bool     `yaml:"enabled"`
	SMTPHost string   `yaml:"smtp_host"`
	SMTPPort int      `yaml:"smtp_port"`
	From     string   `yaml:"from"`
	To       []string `yaml:"to"`
}

// EvaluationConfig contains rule evaluation settings
type EvaluationConfig struct {
	Interval  string `yaml:"interval"`
	Retention string `yaml:"retention"`
}

var (
	// validOperators are the supported comparison operators
	validOperators = map[string]bool{
		">":  true,
		"<":  true,
		">=": true,
		"<=": true,
		"==": true,
	}

	// validSeverities are the supported alert severity levels
	validSeverities = map[string]bool{
		"info":     true,
		"warning":  true,
		"critical": true,
	}

	// validMetrics are the supported metric names
	validMetrics = map[string]bool{
		"db_size_mb":            true,
		"error_rate_percent":    true,
		"sync_failed_count_1h":  true,
		"health_check_failed":   true,
		"memory_mb":             true,
		"cost_today_usd":        true,
	}

	// envVarPattern matches ${VAR_NAME} for environment variable expansion
	envVarPattern = regexp.MustCompile(`\$\{([^}]+)\}`)
)

// LoadRules loads alert rules from YAML file
func LoadRules(path string) (*RuleSet, error) {
	// Read file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read rules file: %w", err)
	}

	// Expand environment variables
	expandedData := expandEnvVars(string(data))

	// Parse YAML
	var ruleSet RuleSet
	if err := yaml.Unmarshal([]byte(expandedData), &ruleSet); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Validate rules
	if err := ruleSet.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	return &ruleSet, nil
}

// Validate checks if the rule set is valid
func (r *RuleSet) Validate() error {
	if len(r.Alerts) == 0 {
		return fmt.Errorf("no alerts defined in configuration")
	}

	// Validate each rule
	for i, rule := range r.Alerts {
		if err := rule.Validate(); err != nil {
			return fmt.Errorf("alert[%d] '%s': %w", i, rule.Name, err)
		}
	}

	// Validate evaluation config
	if r.Evaluation.Interval == "" {
		return fmt.Errorf("evaluation.interval is required")
	}
	if r.Evaluation.Retention == "" {
		return fmt.Errorf("evaluation.retention is required")
	}

	return nil
}

// Validate checks if a single rule is valid
func (rc *RuleConfig) Validate() error {
	// Check required fields
	if rc.Name == "" {
		return fmt.Errorf("name is required")
	}
	if rc.Metric == "" {
		return fmt.Errorf("metric is required")
	}
	if rc.Operator == "" {
		return fmt.Errorf("operator is required")
	}
	if rc.Severity == "" {
		return fmt.Errorf("severity is required")
	}

	// Validate operator
	if !validOperators[rc.Operator] {
		return fmt.Errorf("invalid operator '%s' (must be one of: >, <, >=, <=, ==)", rc.Operator)
	}

	// Validate severity
	if !validSeverities[rc.Severity] {
		return fmt.Errorf("invalid severity '%s' (must be one of: info, warning, critical)", rc.Severity)
	}

	// Validate metric name
	if !validMetrics[rc.Metric] {
		return fmt.Errorf("unknown metric '%s' (must be one of: db_size_mb, error_rate_percent, sync_failed_count_1h, health_check_failed, memory_mb, cost_today_usd)", rc.Metric)
	}

	// Validate channels if rule is enabled
	if rc.Enabled && len(rc.Channels) == 0 {
		return fmt.Errorf("at least one notification channel is required for enabled alerts")
	}

	return nil
}

// ToRule converts RuleConfig to Rule type
func (rc *RuleConfig) ToRule() *Rule {
	return &Rule{
		Name:        rc.Name,
		Description: rc.Description,
		Metric:      rc.Metric,
		Threshold:   rc.Threshold,
		Operator:    rc.Operator,
		Severity:    Severity(rc.Severity),
		Enabled:     rc.Enabled,
		Channels:    rc.Channels,
		Actions:     rc.Actions,
	}
}

// expandEnvVars expands ${VAR} environment variables in the input string
func expandEnvVars(input string) string {
	return envVarPattern.ReplaceAllStringFunc(input, func(match string) string {
		// Extract variable name from ${VAR}
		varName := strings.TrimSuffix(strings.TrimPrefix(match, "${"), "}")

		// Get environment variable value
		if value := os.Getenv(varName); value != "" {
			return value
		}

		// Return original if not found (allows for optional vars)
		return match
	})
}
