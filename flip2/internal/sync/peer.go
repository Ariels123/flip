package sync

import (
	"context"
	"time"
)

// Record represents a sync record with vector clock metadata
type Record struct {
	ID           string       `json:"id"`
	Collection   string       `json:"collection"`
	RecordID     string       `json:"record_id"`
	Operation    string       `json:"operation"` // create, update, delete
	Data         []byte       `json:"data"`
	VectorClock  *VectorClock `json:"vector_clock"`
	Timestamp    time.Time    `json:"timestamp"`
	OriginNodeID string       `json:"origin_node_id"`
}

// Peer defines the interface for communicating with remote FLIP2 nodes
type Peer interface {
	// ID returns the peer's unique identifier
	ID() string

	// GetVectorClock fetches the peer's current vector clock
	GetVectorClock(ctx context.Context) (*VectorClock, error)

	// PushRecords sends records to the peer
	PushRecords(ctx context.Context, records []*Record) error

	// FetchRecordsSince retrieves records newer than the given vector clock
	FetchRecordsSince(ctx context.Context, since *VectorClock) ([]*Record, error)

	// IsReachable checks if the peer is accessible
	IsReachable(ctx context.Context) bool
}

// RecordStore defines the interface for persisting sync records
type RecordStore interface {
	// GetRecordsSince returns records newer than the given vector clock
	GetRecordsSince(since *VectorClock) ([]*Record, error)

	// ApplyRecord stores an incoming record (handles conflicts)
	ApplyRecord(record *Record) error

	// GetRecord retrieves a specific record
	GetRecord(collection, recordID string) (*Record, error)
}

// ConflictResolver handles conflicts between concurrent updates
type ConflictResolver interface {
	// Resolve determines which record should win in a conflict
	// Returns the winning record
	Resolve(local, remote *Record) *Record
}

// LastWriteWinsResolver implements conflict resolution using timestamps
type LastWriteWinsResolver struct{}

// Resolve picks the record with the later timestamp
func (r *LastWriteWinsResolver) Resolve(local, remote *Record) *Record {
	if remote.Timestamp.After(local.Timestamp) {
		return remote
	}
	return local
}
