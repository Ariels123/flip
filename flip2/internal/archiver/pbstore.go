package archiver

import (
	"context"
	"fmt"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// PBStore implements SignalStore using PocketBase as the backend.
type PBStore struct {
	app               *pocketbase.PocketBase
	signalsCollection string
	archiveCollection string
}

// NewPBStore creates a new PocketBase-backed signal store.
func NewPBStore(app *pocketbase.PocketBase) *PBStore {
	return &PBStore{
		app:               app,
		signalsCollection: "signals",
		archiveCollection: "signals_archive",
	}
}

// NewPBStoreWithCollections creates a store with custom collection names.
func NewPBStoreWithCollections(app *pocketbase.PocketBase, signalsCol, archiveCol string) *PBStore {
	return &PBStore{
		app:               app,
		signalsCollection: signalsCol,
		archiveCollection: archiveCol,
	}
}

// GetSignals retrieves signals matching the filter.
func (s *PBStore) GetSignals(ctx context.Context, filter string, limit int) ([]Signal, error) {
	collection, err := s.app.FindCollectionByNameOrId(s.signalsCollection)
	if err != nil {
		return nil, fmt.Errorf("signals collection not found: %w", err)
	}

	records, err := s.app.FindRecordsByFilter(
		collection.Id,
		filter,
		"-created",
		limit,
		0,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch signals: %w", err)
	}

	signals := make([]Signal, 0, len(records))
	for _, record := range records {
		sig := recordToSignal(record)
		signals = append(signals, sig)
	}

	return signals, nil
}

// DeleteSignal removes a signal by ID.
func (s *PBStore) DeleteSignal(ctx context.Context, id string) error {
	record, err := s.app.FindRecordById(s.signalsCollection, id)
	if err != nil {
		return fmt.Errorf("signal not found: %w", err)
	}

	if err := s.app.Delete(record); err != nil {
		return fmt.Errorf("failed to delete signal: %w", err)
	}

	return nil
}

// CreateArchiveRecord creates a record in the archive collection.
func (s *PBStore) CreateArchiveRecord(ctx context.Context, signal ArchivedSignal) error {
	collection, err := s.app.FindCollectionByNameOrId(s.archiveCollection)
	if err != nil {
		// Try to create the archive collection if it doesn't exist
		if createErr := s.ensureArchiveCollection(); createErr != nil {
			return fmt.Errorf("archive collection not found and could not create: %w", err)
		}
		collection, err = s.app.FindCollectionByNameOrId(s.archiveCollection)
		if err != nil {
			return fmt.Errorf("archive collection still not found: %w", err)
		}
	}

	record := core.NewRecord(collection)
	record.Set("signal_id", signal.SignalID)
	record.Set("from_agent", signal.FromAgent)
	record.Set("to_agent", signal.ToAgent)
	record.Set("signal_type", signal.SignalType)
	record.Set("priority", signal.Priority)
	record.Set("content", signal.Content)
	record.Set("read", signal.Read)
	record.Set("read_at", signal.ReadAt)
	record.Set("archived_at", signal.ArchivedAt)
	record.Set("archived_by", signal.ArchivedBy)
	record.Set("original_collection", signal.OriginalCollection)
	record.Set("reason", signal.Reason)

	if err := s.app.Save(record); err != nil {
		return fmt.Errorf("failed to create archive record: %w", err)
	}

	return nil
}

// GetArchiveSignals retrieves signals from the archive collection.
func (s *PBStore) GetArchiveSignals(ctx context.Context, filter string, limit int) ([]ArchivedSignal, error) {
	collection, err := s.app.FindCollectionByNameOrId(s.archiveCollection)
	if err != nil {
		// Archive collection might not exist yet, return empty
		return []ArchivedSignal{}, nil
	}

	records, err := s.app.FindRecordsByFilter(
		collection.Id,
		filter,
		"-archived_at",
		limit,
		0,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch archive signals: %w", err)
	}

	signals := make([]ArchivedSignal, 0, len(records))
	for _, record := range records {
		sig := recordToArchivedSignal(record)
		signals = append(signals, sig)
	}

	return signals, nil
}

// DeleteArchiveRecord removes a record from the archive collection.
func (s *PBStore) DeleteArchiveRecord(ctx context.Context, id string) error {
	record, err := s.app.FindRecordById(s.archiveCollection, id)
	if err != nil {
		return fmt.Errorf("archive record not found: %w", err)
	}

	if err := s.app.Delete(record); err != nil {
		return fmt.Errorf("failed to delete archive record: %w", err)
	}

	return nil
}

// ensureArchiveCollection creates the signals_archive collection if it doesn't exist.
func (s *PBStore) ensureArchiveCollection() error {
	// Check if collection already exists
	_, err := s.app.FindCollectionByNameOrId(s.archiveCollection)
	if err == nil {
		return nil // Already exists
	}

	// Create the archive collection
	collection := core.NewBaseCollection(s.archiveCollection)

	// Add fields matching ArchivedSignal
	collection.Fields.Add(&core.TextField{Name: "signal_id", Required: true})
	collection.Fields.Add(&core.TextField{Name: "from_agent", Required: true})
	collection.Fields.Add(&core.TextField{Name: "to_agent", Required: true})
	collection.Fields.Add(&core.TextField{Name: "signal_type"})
	collection.Fields.Add(&core.TextField{Name: "priority"})
	collection.Fields.Add(&core.TextField{Name: "content"})
	collection.Fields.Add(&core.BoolField{Name: "read"})
	collection.Fields.Add(&core.TextField{Name: "read_at"})
	collection.Fields.Add(&core.DateField{Name: "archived_at"})
	collection.Fields.Add(&core.TextField{Name: "archived_by"})
	collection.Fields.Add(&core.TextField{Name: "original_collection"})
	collection.Fields.Add(&core.TextField{Name: "reason"})

	// Set public access rules
	emptyRule := ""
	collection.ListRule = &emptyRule
	collection.ViewRule = &emptyRule
	collection.CreateRule = &emptyRule
	collection.UpdateRule = &emptyRule
	collection.DeleteRule = &emptyRule

	// Add indexes
	collection.AddIndex("idx_archive_signal_id", false, "signal_id", "")
	collection.AddIndex("idx_archive_from_agent", false, "from_agent", "")
	collection.AddIndex("idx_archive_archived_at", false, "archived_at", "")

	if err := s.app.Save(collection); err != nil {
		return fmt.Errorf("failed to create archive collection: %w", err)
	}

	return nil
}

// Helper functions to convert PocketBase records to Signal types

func recordToSignal(record *core.Record) Signal {
	return Signal{
		ID:         record.Id,
		SignalID:   record.GetString("signal_id"),
		FromAgent:  record.GetString("from_agent"),
		ToAgent:    record.GetString("to_agent"),
		SignalType: record.GetString("signal_type"),
		Priority:   record.GetString("priority"),
		Content:    record.GetString("content"),
		Read:       record.GetBool("read"),
		ReadAt:     record.GetString("read_at"),
		Created:    record.GetDateTime("created").Time(),
		Updated:    record.GetDateTime("updated").Time(),
	}
}

func recordToArchivedSignal(record *core.Record) ArchivedSignal {
	archivedAt := record.GetDateTime("archived_at").Time()
	if archivedAt.IsZero() {
		archivedAt = time.Now()
	}

	return ArchivedSignal{
		Signal: Signal{
			ID:         record.Id,
			SignalID:   record.GetString("signal_id"),
			FromAgent:  record.GetString("from_agent"),
			ToAgent:    record.GetString("to_agent"),
			SignalType: record.GetString("signal_type"),
			Priority:   record.GetString("priority"),
			Content:    record.GetString("content"),
			Read:       record.GetBool("read"),
			ReadAt:     record.GetString("read_at"),
			Created:    record.GetDateTime("created").Time(),
			Updated:    record.GetDateTime("updated").Time(),
		},
		ArchivedAt:         archivedAt,
		ArchivedBy:         record.GetString("archived_by"),
		OriginalCollection: record.GetString("original_collection"),
		Reason:             record.GetString("reason"),
	}
}
