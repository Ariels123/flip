package alerts

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

const (
	// DefaultCooldownPeriod is the minimum time between firing the same alert
	DefaultCooldownPeriod = 5 * time.Minute
)

// Manager orchestrates alert lifecycle
type Manager struct {
	store           AlertStore
	dispatcher      *Dispatcher
	logger          *slog.Logger
	cooldownPeriod  time.Duration

	// In-memory cache of active alerts for deduplication
	mu           sync.RWMutex
	activeAlerts map[string]*Alert     // alert_name -> Alert
	lastFired    map[string]time.Time  // alert_name -> last fired time
}

// NewManager creates a new alert manager
func NewManager(store AlertStore, logger *slog.Logger) *Manager {
	if logger == nil {
		logger = slog.Default()
	}

	m := &Manager{
		store:          store,
		logger:         logger.WithGroup("alerts"),
		cooldownPeriod: DefaultCooldownPeriod,
		activeAlerts:   make(map[string]*Alert),
		lastFired:      make(map[string]time.Time),
	}

	// Load active alerts from store on startup
	ctx := context.Background()
	if err := m.loadActiveAlerts(ctx); err != nil {
		logger.Error("Failed to load active alerts on startup", "error", err)
	}

	return m
}

// Fire creates and stores a new alert
func (m *Manager) Fire(ctx context.Context, rule *Rule, value float64, message string, metadata map[string]interface{}) (*Alert, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if alert is already firing
	if existing, exists := m.activeAlerts[rule.Name]; exists {
		m.logger.Debug("Alert already firing, skipping duplicate",
			"alert_name", rule.Name,
			"existing_id", existing.ID,
		)
		return existing, nil
	}

	// Check cooldown period
	if lastFired, exists := m.lastFired[rule.Name]; exists {
		if time.Since(lastFired) < m.cooldownPeriod {
			m.logger.Debug("Alert in cooldown period, skipping",
				"alert_name", rule.Name,
				"last_fired", lastFired,
				"cooldown_remaining", m.cooldownPeriod-time.Since(lastFired),
			)
			return nil, nil
		}
	}

	// Create new alert
	alert := &Alert{
		AlertName:        rule.Name,
		Severity:         rule.Severity,
		Message:          message,
		MetricValue:      value,
		Threshold:        rule.Threshold,
		State:            StateFiring,
		FiredAt:          time.Now(),
		NotificationSent: false,
		Metadata:         metadata,
	}

	// Save to store
	if err := m.store.SaveAlert(ctx, alert); err != nil {
		m.logger.Error("Failed to save alert",
			"alert_name", rule.Name,
			"error", err,
		)
		return nil, fmt.Errorf("failed to save alert: %w", err)
	}

	// Update in-memory cache
	m.activeAlerts[rule.Name] = alert
	m.lastFired[rule.Name] = alert.FiredAt

	m.logger.Info("Alert fired",
		"alert_name", rule.Name,
		"severity", rule.Severity,
		"metric_value", value,
		"threshold", rule.Threshold,
		"message", message,
	)

	// Send notifications if dispatcher is configured
	if m.dispatcher != nil && len(rule.Channels) > 0 {
		if err := m.dispatcher.Dispatch(ctx, alert, rule.Channels); err != nil {
			m.logger.Error("Failed to dispatch notifications",
				"alert_name", rule.Name,
				"error", err,
			)
			// Don't fail the whole operation - alert is still stored
		} else {
			// Mark notification as sent
			alert.NotificationSent = true
			if err := m.store.UpdateAlert(ctx, alert); err != nil {
				m.logger.Error("Failed to mark notification sent",
					"alert_id", alert.ID,
					"error", err,
				)
			}
		}
	}

	return alert, nil
}

// Resolve marks an alert as resolved
func (m *Manager) Resolve(ctx context.Context, alertName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if alert is currently firing
	alert, exists := m.activeAlerts[alertName]
	if !exists {
		m.logger.Debug("Alert not firing, nothing to resolve",
			"alert_name", alertName,
		)
		return nil
	}

	// Update alert state
	now := time.Now()
	alert.State = StateResolved
	alert.ResolvedAt = &now

	// Update in store
	if err := m.store.UpdateAlert(ctx, alert); err != nil {
		m.logger.Error("Failed to update alert state",
			"alert_name", alertName,
			"error", err,
		)
		return fmt.Errorf("failed to update alert: %w", err)
	}

	// Remove from active alerts
	delete(m.activeAlerts, alertName)

	m.logger.Info("Alert resolved",
		"alert_name", alertName,
		"duration", now.Sub(alert.FiredAt),
	)

	return nil
}

// GetActive returns currently firing alerts
func (m *Manager) GetActive(ctx context.Context) ([]*Alert, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	alerts := make([]*Alert, 0, len(m.activeAlerts))
	for _, alert := range m.activeAlerts {
		alerts = append(alerts, alert)
	}

	return alerts, nil
}

// ShouldFire checks if alert should fire (deduplication + cooldown)
func (m *Manager) ShouldFire(ruleName string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Check if already firing
	if _, exists := m.activeAlerts[ruleName]; exists {
		return false
	}

	// Check cooldown period
	if lastFired, exists := m.lastFired[ruleName]; exists {
		if time.Since(lastFired) < m.cooldownPeriod {
			return false
		}
	}

	return true
}

// CleanupOld removes resolved alerts older than retention period
func (m *Manager) CleanupOld(ctx context.Context, retention time.Duration) error {
	cutoff := time.Now().Add(-retention)

	if err := m.store.DeleteOldAlerts(ctx, cutoff); err != nil {
		m.logger.Error("Failed to cleanup old alerts",
			"cutoff", cutoff,
			"error", err,
		)
		return fmt.Errorf("failed to cleanup old alerts: %w", err)
	}

	m.logger.Info("Cleaned up old alerts",
		"cutoff", cutoff,
		"retention", retention,
	)

	return nil
}

// SetCooldownPeriod sets the cooldown period for alert deduplication
func (m *Manager) SetCooldownPeriod(period time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cooldownPeriod = period
}

// SetDispatcher sets the notification dispatcher for sending alerts
func (m *Manager) SetDispatcher(dispatcher *Dispatcher) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dispatcher = dispatcher
}

// loadActiveAlerts loads currently firing alerts from the store into memory
func (m *Manager) loadActiveAlerts(ctx context.Context) error {
	alerts, err := m.store.GetActiveAlerts(ctx)
	if err != nil {
		return fmt.Errorf("failed to load active alerts: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, alert := range alerts {
		m.activeAlerts[alert.AlertName] = alert
		m.lastFired[alert.AlertName] = alert.FiredAt
	}

	m.logger.Info("Loaded active alerts from store",
		"count", len(alerts),
	)

	return nil
}

// MarkNotificationSent marks an alert as having its notification sent
func (m *Manager) MarkNotificationSent(ctx context.Context, alertID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Find the alert in active alerts
	var alert *Alert
	for _, a := range m.activeAlerts {
		if a.ID == alertID {
			alert = a
			break
		}
	}

	if alert == nil {
		return fmt.Errorf("alert not found: %s", alertID)
	}

	// Update notification status
	alert.NotificationSent = true

	// Update in store
	if err := m.store.UpdateAlert(ctx, alert); err != nil {
		m.logger.Error("Failed to mark notification sent",
			"alert_id", alertID,
			"error", err,
		)
		return fmt.Errorf("failed to update alert: %w", err)
	}

	m.logger.Debug("Marked notification sent",
		"alert_id", alertID,
		"alert_name", alert.AlertName,
	)

	return nil
}
