package alerts

import (
	"context"
	"fmt"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// AlertStore handles alert persistence
type AlertStore interface {
	SaveAlert(ctx context.Context, alert *Alert) error
	GetAlert(ctx context.Context, id string) (*Alert, error)
	GetActiveAlerts(ctx context.Context) ([]*Alert, error)
	GetAlertsByName(ctx context.Context, name string) ([]*Alert, error)
	UpdateAlert(ctx context.Context, alert *Alert) error
	DeleteOldAlerts(ctx context.Context, before time.Time) error
}

// PBAlertStore implements AlertStore using PocketBase
type PBAlertStore struct {
	app        *pocketbase.PocketBase
	collection string
}

// NewPBAlertStore creates a new PocketBase-backed alert store
func NewPBAlertStore(app *pocketbase.PocketBase) *PBAlertStore {
	return &PBAlertStore{
		app:        app,
		collection: "alerts",
	}
}

// SaveAlert saves a new alert to PocketBase
func (s *PBAlertStore) SaveAlert(ctx context.Context, alert *Alert) error {
	collection, err := s.app.FindCollectionByNameOrId(s.collection)
	if err != nil {
		return fmt.Errorf("alerts collection not found: %w", err)
	}

	pbRecord := core.NewRecord(collection)
	pbRecord.Set("alert_name", alert.AlertName)
	pbRecord.Set("severity", string(alert.Severity))
	pbRecord.Set("message", alert.Message)
	pbRecord.Set("metric_value", alert.MetricValue)
	pbRecord.Set("threshold", alert.Threshold)
	pbRecord.Set("state", string(alert.State))
	pbRecord.Set("fired_at", alert.FiredAt)
	if alert.ResolvedAt != nil {
		pbRecord.Set("resolved_at", *alert.ResolvedAt)
	}
	pbRecord.Set("notification_sent", alert.NotificationSent)
	if alert.Metadata != nil {
		pbRecord.Set("metadata", alert.Metadata)
	}

	if err := s.app.Save(pbRecord); err != nil {
		return fmt.Errorf("failed to save alert: %w", err)
	}

	// Update the alert ID with the generated record ID
	alert.ID = pbRecord.Id

	return nil
}

// GetAlert retrieves a single alert by ID
func (s *PBAlertStore) GetAlert(ctx context.Context, id string) (*Alert, error) {
	collection, err := s.app.FindCollectionByNameOrId(s.collection)
	if err != nil {
		return nil, fmt.Errorf("alerts collection not found: %w", err)
	}

	record, err := s.app.FindRecordById(collection.Id, id)
	if err != nil {
		return nil, fmt.Errorf("alert not found: %w", err)
	}

	return s.recordToAlert(record), nil
}

// GetActiveAlerts retrieves all currently firing alerts
func (s *PBAlertStore) GetActiveAlerts(ctx context.Context) ([]*Alert, error) {
	collection, err := s.app.FindCollectionByNameOrId(s.collection)
	if err != nil {
		return nil, fmt.Errorf("alerts collection not found: %w", err)
	}

	filter := "state = 'firing'"
	records, err := s.app.FindRecordsByFilter(
		collection.Id,
		filter,
		"-fired_at", // Sort by most recent first
		1000,        // Max 1000 active alerts
		0,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query active alerts: %w", err)
	}

	alerts := make([]*Alert, 0, len(records))
	for _, record := range records {
		alerts = append(alerts, s.recordToAlert(record))
	}

	return alerts, nil
}

// GetAlertsByName retrieves all alerts for a specific alert name
func (s *PBAlertStore) GetAlertsByName(ctx context.Context, name string) ([]*Alert, error) {
	collection, err := s.app.FindCollectionByNameOrId(s.collection)
	if err != nil {
		return nil, fmt.Errorf("alerts collection not found: %w", err)
	}

	filter := fmt.Sprintf("alert_name = '%s'", name)
	records, err := s.app.FindRecordsByFilter(
		collection.Id,
		filter,
		"-fired_at", // Sort by most recent first
		100,         // Max 100 records per alert name
		0,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query alerts by name: %w", err)
	}

	alerts := make([]*Alert, 0, len(records))
	for _, record := range records {
		alerts = append(alerts, s.recordToAlert(record))
	}

	return alerts, nil
}

// UpdateAlert updates an existing alert
func (s *PBAlertStore) UpdateAlert(ctx context.Context, alert *Alert) error {
	if alert.ID == "" {
		return fmt.Errorf("alert ID is required for update")
	}

	collection, err := s.app.FindCollectionByNameOrId(s.collection)
	if err != nil {
		return fmt.Errorf("alerts collection not found: %w", err)
	}

	record, err := s.app.FindRecordById(collection.Id, alert.ID)
	if err != nil {
		return fmt.Errorf("alert not found: %w", err)
	}

	record.Set("state", string(alert.State))
	record.Set("notification_sent", alert.NotificationSent)
	if alert.ResolvedAt != nil {
		record.Set("resolved_at", *alert.ResolvedAt)
	}
	if alert.Metadata != nil {
		record.Set("metadata", alert.Metadata)
	}

	if err := s.app.Save(record); err != nil {
		return fmt.Errorf("failed to update alert: %w", err)
	}

	return nil
}

// DeleteOldAlerts removes resolved alerts older than the specified time
func (s *PBAlertStore) DeleteOldAlerts(ctx context.Context, before time.Time) error {
	collection, err := s.app.FindCollectionByNameOrId(s.collection)
	if err != nil {
		return fmt.Errorf("alerts collection not found: %w", err)
	}

	filter := fmt.Sprintf("state = 'resolved' && resolved_at < '%s'",
		before.Format(time.RFC3339))

	records, err := s.app.FindRecordsByFilter(
		collection.Id,
		filter,
		"",
		1000, // Delete in batches of 1000
		0,
	)
	if err != nil {
		return fmt.Errorf("failed to query old alerts: %w", err)
	}

	// Delete each record
	for _, record := range records {
		if err := s.app.Delete(record); err != nil {
			return fmt.Errorf("failed to delete alert %s: %w", record.Id, err)
		}
	}

	return nil
}

// recordToAlert converts a PocketBase record to an Alert struct
func (s *PBAlertStore) recordToAlert(record *core.Record) *Alert {
	alert := &Alert{
		ID:               record.Id,
		AlertName:        record.GetString("alert_name"),
		Severity:         Severity(record.GetString("severity")),
		Message:          record.GetString("message"),
		MetricValue:      record.GetFloat("metric_value"),
		Threshold:        record.GetFloat("threshold"),
		State:            State(record.GetString("state")),
		FiredAt:          record.GetDateTime("fired_at").Time(),
		NotificationSent: record.GetBool("notification_sent"),
	}

	// Handle optional resolved_at field
	if resolvedAt := record.GetDateTime("resolved_at"); !resolvedAt.Time().IsZero() {
		t := resolvedAt.Time()
		alert.ResolvedAt = &t
	}

	// Handle optional metadata field
	if metadata := record.Get("metadata"); metadata != nil {
		if m, ok := metadata.(map[string]interface{}); ok {
			alert.Metadata = m
		}
	}

	return alert
}
