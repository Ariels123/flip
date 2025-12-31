package sync

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Replicator orchestrates bidirectional sync between FLIP2 nodes
type Replicator struct {
	mu           sync.RWMutex
	nodeID       string
	vectorClock  *VectorClock
	peers        map[string]Peer
	store        RecordStore
	resolver     ConflictResolver
	logger       *slog.Logger
	syncHistory  map[string]time.Time // last sync time per peer
}

// NewReplicator creates a new replicator instance
func NewReplicator(nodeID string, store RecordStore, logger *slog.Logger) *Replicator {
	if logger == nil {
		logger = slog.Default()
	}
	return &Replicator{
		nodeID:      nodeID,
		vectorClock: NewVectorClock(nodeID),
		peers:       make(map[string]Peer),
		store:       store,
		resolver:    &LastWriteWinsResolver{},
		logger:      logger,
		syncHistory: make(map[string]time.Time),
	}
}

// AddPeer registers a peer for synchronization
func (r *Replicator) AddPeer(peer Peer) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.peers[peer.ID()] = peer
	r.logger.Info("Added sync peer", "peer_id", peer.ID())
}

// RemovePeer removes a peer from synchronization
func (r *Replicator) RemovePeer(peerID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.peers, peerID)
	r.logger.Info("Removed sync peer", "peer_id", peerID)
}

// GetVectorClock returns the current vector clock state
func (r *Replicator) GetVectorClock() *VectorClock {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.vectorClock.Clone()
}

// OnLocalChange should be called when local data changes
func (r *Replicator) OnLocalChange() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.vectorClock.Increment()
}

// SyncAll synchronizes with all registered peers
func (r *Replicator) SyncAll(ctx context.Context) error {
	r.mu.RLock()
	peers := make([]Peer, 0, len(r.peers))
	for _, p := range r.peers {
		peers = append(peers, p)
	}
	r.mu.RUnlock()

	var lastErr error
	for _, peer := range peers {
		if err := r.Sync(ctx, peer.ID()); err != nil {
			r.logger.Error("Sync failed", "peer_id", peer.ID(), "error", err)
			lastErr = err
		}
	}
	return lastErr
}

// Sync performs bidirectional synchronization with a specific peer
func (r *Replicator) Sync(ctx context.Context, peerID string) error {
	r.mu.RLock()
	peer, ok := r.peers[peerID]
	r.mu.RUnlock()
	if !ok {
		return fmt.Errorf("unknown peer: %s", peerID)
	}

	r.logger.Debug("Starting sync", "peer_id", peerID)

	// Check if peer is reachable
	if !peer.IsReachable(ctx) {
		return fmt.Errorf("peer %s is not reachable", peerID)
	}

	// 1. Get peer's vector clock
	remoteVC, err := peer.GetVectorClock(ctx)
	if err != nil {
		return fmt.Errorf("failed to get peer vector clock: %w", err)
	}

	r.mu.RLock()
	localVC := r.vectorClock.Clone()
	r.mu.RUnlock()

	// 2. Compare clocks to determine what to exchange
	comparison := localVC.Compare(remoteVC)
	r.logger.Debug("Vector clock comparison",
		"peer_id", peerID,
		"result", comparison,
		"local", localVC.Clocks,
		"remote", remoteVC.Clocks)

	// 3. Push local records that peer doesn't have
	if comparison == After || comparison == Concurrent {
		records, err := r.store.GetRecordsSince(remoteVC)
		if err != nil {
			return fmt.Errorf("failed to get local records: %w", err)
		}
		if len(records) > 0 {
			r.logger.Info("Pushing records to peer", "peer_id", peerID, "count", len(records))
			if err := peer.PushRecords(ctx, records); err != nil {
				return fmt.Errorf("failed to push records: %w", err)
			}
		}
	}

	// 4. Fetch remote records we don't have
	if comparison == Before || comparison == Concurrent {
		records, err := peer.FetchRecordsSince(ctx, localVC)
		if err != nil {
			return fmt.Errorf("failed to fetch peer records: %w", err)
		}
		for _, record := range records {
			if err := r.applyRecord(record); err != nil {
				r.logger.Error("Failed to apply record", "record_id", record.ID, "error", err)
			}
		}
		if len(records) > 0 {
			r.logger.Info("Applied records from peer", "peer_id", peerID, "count", len(records))
		}
	}

	// 5. Update our vector clock
	r.mu.Lock()
	r.vectorClock.Update(remoteVC)
	r.syncHistory[peerID] = time.Now()
	r.mu.Unlock()

	r.logger.Debug("Sync completed", "peer_id", peerID)
	return nil
}

// applyRecord applies an incoming record, handling conflicts
func (r *Replicator) applyRecord(record *Record) error {
	// Check for existing record
	existing, err := r.store.GetRecord(record.Collection, record.RecordID)
	if err == nil && existing != nil {
		// Conflict - use resolver
		winner := r.resolver.Resolve(existing, record)
		if winner == existing {
			r.logger.Debug("Kept local record in conflict", "record_id", record.RecordID)
			return nil
		}
	}

	// Apply the record
	if err := r.store.ApplyRecord(record); err != nil {
		return err
	}

	// Update vector clock
	r.mu.Lock()
	r.vectorClock.Update(record.VectorClock)
	r.mu.Unlock()

	return nil
}

// GetPeerCount returns the number of registered peers
func (r *Replicator) GetPeerCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.peers)
}

// GetSyncHistory returns when each peer was last synced
func (r *Replicator) GetSyncHistory() map[string]time.Time {
	r.mu.RLock()
	defer r.mu.RUnlock()
	history := make(map[string]time.Time, len(r.syncHistory))
	for k, v := range r.syncHistory {
		history[k] = v
	}
	return history
}
