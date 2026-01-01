package alerts

import "time"

// Severity represents alert severity levels
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

// State represents alert lifecycle state
type State string

const (
	StatePending  State = "pending"
	StateFiring   State = "firing"
	StateResolved State = "resolved"
)

// Alert represents an active or historical alert
type Alert struct {
	ID               string                 `json:"id"`
	AlertName        string                 `json:"alert_name"`
	Severity         Severity               `json:"severity"`
	Message          string                 `json:"message"`
	MetricValue      float64                `json:"metric_value"`
	Threshold        float64                `json:"threshold"`
	State            State                  `json:"state"`
	FiredAt          time.Time              `json:"fired_at"`
	ResolvedAt       *time.Time             `json:"resolved_at,omitempty"`
	NotificationSent bool                   `json:"notification_sent"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
}

// Getter methods for channels.Alert interface compatibility
func (a *Alert) GetAlertName() string                  { return a.AlertName }
func (a *Alert) GetSeverity() string                   { return string(a.Severity) }
func (a *Alert) GetMessage() string                    { return a.Message }
func (a *Alert) GetMetricValue() float64               { return a.MetricValue }
func (a *Alert) GetThreshold() float64                 { return a.Threshold }
func (a *Alert) GetFiredAt() time.Time                 { return a.FiredAt }
func (a *Alert) GetMetadata() map[string]interface{} {
	if a.Metadata == nil {
		return make(map[string]interface{})
	}
	return a.Metadata
}

// Rule defines an alert condition
type Rule struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Metric      string   `yaml:"metric"`
	Threshold   float64  `yaml:"threshold"`
	Operator    string   `yaml:"operator"` // ">", "<", ">=", "<=", "=="
	Severity    Severity `yaml:"severity"`
	Enabled     bool     `yaml:"enabled"`
	Channels    []string `yaml:"channels"`
	Actions     []string `yaml:"actions"`
}
