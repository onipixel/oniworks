// Package memory provides the OniWorks distributed in-memory database.
// It is the state layer of the framework: sessions, presence, rate limits,
// pub/sub events, and realtime state. It is NOT a replacement for PostgreSQL.
package memory

import (
	"sync"
	"sync/atomic"
)

// VectorClock implements a simple Lamport-style vector clock for last-write-wins
// conflict resolution across nodes. Each node maintains its own counter and
// increments it on every write.
type VectorClock struct {
	mu      sync.RWMutex
	nodeID  string
	clocks  map[string]uint64 // nodeID → logical time
	localTS uint64            // atomic counter for this node
}

// NewVectorClock creates a VectorClock for the given node ID.
func NewVectorClock(nodeID string) *VectorClock {
	return &VectorClock{
		nodeID: nodeID,
		clocks: map[string]uint64{nodeID: 0},
	}
}

// Tick increments this node's local time and returns a new ClockValue.
// Call before every local write.
func (vc *VectorClock) Tick() ClockValue {
	ts := atomic.AddUint64(&vc.localTS, 1)
	vc.mu.Lock()
	vc.clocks[vc.nodeID] = ts
	snap := vc.snapshot()
	vc.mu.Unlock()
	return ClockValue{nodeID: vc.nodeID, ts: ts, vector: snap}
}

// Merge updates this clock with a received clock value.
// Uses max(local[node], received[node]) for each node.
func (vc *VectorClock) Merge(received ClockValue) {
	vc.mu.Lock()
	defer vc.mu.Unlock()
	for node, t := range received.vector {
		if current, ok := vc.clocks[node]; !ok || t > current {
			vc.clocks[node] = t
		}
	}
	// Advance our own clock if received is ahead
	if received.vector[vc.nodeID] > vc.clocks[vc.nodeID] {
		atomic.StoreUint64(&vc.localTS, received.vector[vc.nodeID])
		vc.clocks[vc.nodeID] = received.vector[vc.nodeID]
	}
}

func (vc *VectorClock) snapshot() map[string]uint64 {
	snap := make(map[string]uint64, len(vc.clocks))
	for k, v := range vc.clocks {
		snap[k] = v
	}
	return snap
}

// ClockValue is an immutable snapshot of a vector clock at a point in time.
type ClockValue struct {
	nodeID string
	ts     uint64
	vector map[string]uint64
}

// After reports whether cv happened strictly after other (causal ordering).
func (cv ClockValue) After(other ClockValue) bool {
	// If our logical timestamp for our own node is greater → cv is newer
	myTime := cv.vector[cv.nodeID]
	otherTime := other.vector[cv.nodeID]
	if myTime != otherTime {
		return myTime > otherTime
	}
	// Tie-break: wall time embedded in ts (high bits) if same node
	return cv.ts > other.ts
}

// Equal reports whether two clock values are concurrent (neither is after the other).
func (cv ClockValue) Equal(other ClockValue) bool {
	return !cv.After(other) && !other.After(cv)
}
