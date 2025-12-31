package sync

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// PBStore implements RecordStore using PocketBase as the backend
// This enables real synchronization of signals between nodes
type PBStore struct {
	mu     sync.RWMutex
	pb     *pocketbase.PocketBase
	logger *slog.Logger
	nodeID string
}

// NewPBStore creates a new PocketBase-backed record store
func NewPBStore(pb *pocketbase.PocketBase, nodeID string, logger *slog.Logger) *PBStore {
	if logger == nil {
		logger = slog.Default()
	}
	return &PBStore{
		pb:     pb,
		nodeID: nodeID,
		logger: logger,
	}
}

// GetRecordsSince returns records from the signals collection newer than the given vector clock
func (s *PBStore) GetRecordsSince(since *VectorClock) ([]*Record, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Get the signals collection
	collection, err := s.pb.FindCollectionByNameOrId("signals")
	if err != nil {
		return nil, fmt.Errorf("signals collection not found: %w", err)
	}

	// Query all signals (we'll filter by vector clock in memory)
	// Use empty sort to avoid issues with collections missing autodate fields
	records, err := s.pb.FindRecordsByFilter(
		collection.Id,
		"1=1", // Get all records
		"", // No sort - some collections may not have autodate fields
		1000, // Limit
		0,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query signals: %w", err)
	}

	var result []*Record
	for _, pbRecord := range records {
		// Convert PocketBase record to sync Record
		syncRecord, err := s.pbRecordToSyncRecord(pbRecord)
		if err != nil {
			s.logger.Warn("Failed to convert record", "id", pbRecord.Id, "error", err)
			continue
		}

		// Filter by vector clock if provided
		if since != nil {
			comparison := syncRecord.VectorClock.Compare(since)
			if comparison != After && comparison != Concurrent {
				continue // Skip records that aren't newer
			}
		}

		result = append(result, syncRecord)
	}

	s.logger.Debug("GetRecordsSince", "since", since, "found", len(result))
	return result, nil
}

// ApplyRecord applies an incoming record to the signals collection
func (s *PBStore) ApplyRecord(record *Record) error {
	if record == nil {
		return fmt.Errorf("cannot apply nil record")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Only handle signals collection for now
	if record.Collection != "signals" {
		s.logger.Debug("Skipping non-signals collection", "collection", record.Collection)
		return nil
	}

	// Parse the data
	var signalData map[string]interface{}
	if err := json.Unmarshal(record.Data, &signalData); err != nil {
		return fmt.Errorf("failed to unmarshal record data: %w", err)
	}

	// Get or create the signals collection
	collection, err := s.pb.FindCollectionByNameOrId("signals")
	if err != nil {
		return fmt.Errorf("signals collection not found: %w", err)
	}

	// Check if record exists (by signal_id)
	signalID, _ := signalData["signal_id"].(string)
	if signalID == "" {
		signalID = record.RecordID
	}

	existingRecords, err := s.pb.FindRecordsByFilter(
		collection.Id,
		fmt.Sprintf("signal_id = '%s'", signalID),
		"",
		1,
		0,
	)

	switch record.Operation {
	case "delete":
		if len(existingRecords) > 0 {
			if err := s.pb.Delete(existingRecords[0]); err != nil {
				return fmt.Errorf("failed to delete record: %w", err)
			}
			s.logger.Info("Deleted synced record", "signal_id", signalID)
		}
		return nil

	case "create", "update", "":
		var pbRecord *core.Record

		if len(existingRecords) > 0 {
			// Update existing record
			pbRecord = existingRecords[0]
		} else {
			// Create new record - must set Id for PocketBase v0.34+
			pbRecord = core.NewRecord(collection)
			// Use the sync record's ID if available, otherwise generate one
			if record.ID != "" {
				pbRecord.Id = record.ID
			} else if record.RecordID != "" {
				// Use RecordID (signal_id) as the PocketBase record ID
				pbRecord.Id = record.RecordID
			}
		}

		// Set fields from signal data
		if v, ok := signalData["signal_id"]; ok {
			pbRecord.Set("signal_id", v)
		}
		if v, ok := signalData["from_agent"]; ok {
			pbRecord.Set("from_agent", v)
		}
		if v, ok := signalData["to_agent"]; ok {
			pbRecord.Set("to_agent", v)
		}
		if v, ok := signalData["signal_type"]; ok {
			pbRecord.Set("signal_type", v)
		}
		if v, ok := signalData["priority"]; ok {
			pbRecord.Set("priority", v)
		}
		if v, ok := signalData["content"]; ok {
			pbRecord.Set("content", v)
		}
		if v, ok := signalData["read"]; ok {
			pbRecord.Set("read", v)
		}
		if v, ok := signalData["read_at"]; ok {
			pbRecord.Set("read_at", v)
		}

		// Store sync metadata
		vcJSON, _ := json.Marshal(record.VectorClock)
		pbRecord.Set("sync_vector_clock", string(vcJSON))
		pbRecord.Set("sync_origin", record.OriginNodeID)
		pbRecord.Set("sync_timestamp", record.Timestamp)

		if err := s.pb.Save(pbRecord); err != nil {
			return fmt.Errorf("failed to save record: %w", err)
		}

		s.logger.Info("Applied synced record",
			"signal_id", signalID,
			"operation", record.Operation,
			"origin", record.OriginNodeID)
		return nil

	default:
		return fmt.Errorf("unknown operation: %s", record.Operation)
	}
}

// GetRecord retrieves a specific record from the signals collection
func (s *PBStore) GetRecord(collection, recordID string) (*Record, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if collection != "signals" {
		return nil, fmt.Errorf("unsupported collection: %s", collection)
	}

	coll, err := s.pb.FindCollectionByNameOrId("signals")
	if err != nil {
		return nil, fmt.Errorf("signals collection not found: %w", err)
	}

	records, err := s.pb.FindRecordsByFilter(
		coll.Id,
		fmt.Sprintf("signal_id = '%s'", recordID),
		"",
		1,
		0,
	)
	if err != nil || len(records) == 0 {
		return nil, fmt.Errorf("record not found: %s", recordID)
	}

	return s.pbRecordToSyncRecord(records[0])
}

// pbRecordToSyncRecord converts a PocketBase record to a sync Record
func (s *PBStore) pbRecordToSyncRecord(pbRecord *core.Record) (*Record, error) {
	// Extract signal data
	signalData := map[string]interface{}{
		"signal_id":   pbRecord.GetString("signal_id"),
		"from_agent":  pbRecord.GetString("from_agent"),
		"to_agent":    pbRecord.GetString("to_agent"),
		"signal_type": pbRecord.GetString("signal_type"),
		"priority":    pbRecord.GetString("priority"),
		"content":     pbRecord.GetString("content"),
		"read":        pbRecord.GetBool("read"),
		"read_at":     pbRecord.GetString("read_at"),
	}

	data, err := json.Marshal(signalData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal signal data: %w", err)
	}

	// Get or create vector clock
	var vc *VectorClock
	vcStr := pbRecord.GetString("sync_vector_clock")
	if vcStr != "" {
		vc = &VectorClock{}
		if err := json.Unmarshal([]byte(vcStr), vc); err != nil {
			// Create new vector clock on parse error
			vc = NewVectorClock(s.nodeID)
		}
	} else {
		// Create vector clock from created timestamp
		vc = NewVectorClock(s.nodeID)
		vc.Increment()
	}

	// Get origin node
	origin := pbRecord.GetString("sync_origin")
	if origin == "" {
		origin = s.nodeID
	}

	// Get timestamp
	timestamp := pbRecord.GetDateTime("sync_timestamp").Time()
	if timestamp.IsZero() {
		timestamp = pbRecord.GetDateTime("created").Time()
	}
	if timestamp.IsZero() {
		timestamp = time.Now()
	}

	return &Record{
		ID:           pbRecord.Id,
		Collection:   "signals",
		RecordID:     pbRecord.GetString("signal_id"),
		Operation:    "update", // Existing records are updates
		Data:         data,
		VectorClock:  vc,
		Timestamp:    timestamp,
		OriginNodeID: origin,
	}, nil
}

// Count returns the number of signals in the store
func (s *PBStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	collection, err := s.pb.FindCollectionByNameOrId("signals")
	if err != nil {
		return 0
	}

	records, err := s.pb.FindRecordsByFilter(collection.Id, "1=1", "", 0, 0)
	if err != nil {
		return 0
	}

	return len(records)
}
