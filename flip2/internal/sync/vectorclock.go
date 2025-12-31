// Package sync implements distributed synchronization for FLIP2
package sync

import (
	"encoding/json"
	"sync"
)

// CompareResult represents the result of comparing two vector clocks
type CompareResult int

const (
	Before     CompareResult = -1 // This clock happened before other
	Equal      CompareResult = 0  // Clocks are equal
	After      CompareResult = 1  // This clock happened after other
	Concurrent CompareResult = 2  // Clocks are concurrent (conflict)
)

// VectorClock implements a Lamport vector clock for causality tracking
type VectorClock struct {
	mu     sync.RWMutex
	NodeID string           `json:"node_id"`
	Clocks map[string]int64 `json:"clocks"`
}

// NewVectorClock creates a new vector clock for the given node
func NewVectorClock(nodeID string) *VectorClock {
	return &VectorClock{
		NodeID: nodeID,
		Clocks: make(map[string]int64),
	}
}

// Increment advances the local clock
func (vc *VectorClock) Increment() {
	vc.mu.Lock()
	defer vc.mu.Unlock()
	vc.Clocks[vc.NodeID]++
}

// Get returns the clock value for a specific node
func (vc *VectorClock) Get(nodeID string) int64 {
	vc.mu.RLock()
	defer vc.mu.RUnlock()
	return vc.Clocks[nodeID]
}

// GetAll returns a copy of all clock values
func (vc *VectorClock) GetAll() map[string]int64 {
	vc.mu.RLock()
	defer vc.mu.RUnlock()

	result := make(map[string]int64, len(vc.Clocks))
	for k, v := range vc.Clocks {
		result[k] = v
	}
	return result
}

// Update merges another vector clock into this one (takes max of each)
func (vc *VectorClock) Update(other *VectorClock) {
	if other == nil {
		return
	}
	vc.mu.Lock()
	defer vc.mu.Unlock()
	other.mu.RLock()
	defer other.mu.RUnlock()

	for nodeID, clock := range other.Clocks {
		if clock > vc.Clocks[nodeID] {
			vc.Clocks[nodeID] = clock
		}
	}
}

// Compare determines the ordering relationship with another clock
func (vc *VectorClock) Compare(other *VectorClock) CompareResult {
	if other == nil {
		return After
	}
	vc.mu.RLock()
	defer vc.mu.RUnlock()
	other.mu.RLock()
	defer other.mu.RUnlock()

	less := false
	greater := false

	// Check all keys in both clocks
	allKeys := make(map[string]bool)
	for k := range vc.Clocks {
		allKeys[k] = true
	}
	for k := range other.Clocks {
		allKeys[k] = true
	}

	for nodeID := range allKeys {
		v1 := vc.Clocks[nodeID]
		v2 := other.Clocks[nodeID]
		if v1 < v2 {
			less = true
		}
		if v1 > v2 {
			greater = true
		}
	}

	if less && greater {
		return Concurrent
	}
	if less {
		return Before
	}
	if greater {
		return After
	}
	return Equal
}

// Clone creates a deep copy of the vector clock
func (vc *VectorClock) Clone() *VectorClock {
	vc.mu.RLock()
	defer vc.mu.RUnlock()

	clone := &VectorClock{
		NodeID: vc.NodeID,
		Clocks: make(map[string]int64, len(vc.Clocks)),
	}
	for k, v := range vc.Clocks {
		clone.Clocks[k] = v
	}
	return clone
}

// MarshalJSON implements json.Marshaler
func (vc *VectorClock) MarshalJSON() ([]byte, error) {
	vc.mu.RLock()
	defer vc.mu.RUnlock()
	return json.Marshal(struct {
		NodeID string           `json:"node_id"`
		Clocks map[string]int64 `json:"clocks"`
	}{
		NodeID: vc.NodeID,
		Clocks: vc.Clocks,
	})
}

// UnmarshalJSON implements json.Unmarshaler
func (vc *VectorClock) UnmarshalJSON(data []byte) error {
	var tmp struct {
		NodeID string           `json:"node_id"`
		Clocks map[string]int64 `json:"clocks"`
	}
	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}
	vc.NodeID = tmp.NodeID
	vc.Clocks = tmp.Clocks
	if vc.Clocks == nil {
		vc.Clocks = make(map[string]int64)
	}
	return nil
}
