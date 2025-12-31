package sync

import (
	"fmt"
	"sync"
)

// MemoryStore implements RecordStore with in-memory storage
// This is a simple implementation for testing sync; production would use PocketBase
type MemoryStore struct {
	mu      sync.RWMutex
	records map[string]*Record // key: collection:recordID
}

// NewMemoryStore creates a new in-memory record store
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		records: make(map[string]*Record),
	}
}

// makeKey creates a unique key from collection and recordID
func makeKey(collection, recordID string) string {
	return collection + ":" + recordID
}

// GetRecordsSince returns records newer than the given vector clock
func (s *MemoryStore) GetRecordsSince(since *VectorClock) ([]*Record, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Record
	for _, record := range s.records {
		if since == nil {
			// No filter - return all
			result = append(result, record)
			continue
		}

		// Check if this record is newer than the since clock
		comparison := record.VectorClock.Compare(since)
		if comparison == After || comparison == Concurrent {
			result = append(result, record)
		}
	}
	return result, nil
}

// ApplyRecord stores an incoming record
func (s *MemoryStore) ApplyRecord(record *Record) error {
	if record == nil {
		return fmt.Errorf("cannot apply nil record")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	key := makeKey(record.Collection, record.RecordID)
	s.records[key] = record
	return nil
}

// GetRecord retrieves a specific record
func (s *MemoryStore) GetRecord(collection, recordID string) (*Record, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := makeKey(collection, recordID)
	record, ok := s.records[key]
	if !ok {
		return nil, fmt.Errorf("record not found: %s", key)
	}
	return record, nil
}

// Count returns the number of records in the store
func (s *MemoryStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.records)
}
