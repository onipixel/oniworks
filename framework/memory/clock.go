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
	return ClockValue{NodeID: vc.nodeID, TS: ts, Vector: snap}
}

// Merge updates this clock with a received clock value.
// Uses max(local[node], received[node]) for each node.
func (vc *VectorClock) Merge(received ClockValue) {
	vc.mu.Lock()
	defer vc.mu.Unlock()
	for node, t := range received.Vector {
		if current, ok := vc.clocks[node]; !ok || t > current {
			vc.clocks[node] = t
		}
	}
	// Advance our own clock if received is ahead
	if received.Vector[vc.nodeID] > vc.clocks[vc.nodeID] {
		atomic.StoreUint64(&vc.localTS, received.Vector[vc.nodeID])
		vc.clocks[vc.nodeID] = received.Vector[vc.nodeID]
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
//
// Fields are EXPORTED so the value survives gob encoding across the gossip
// transport — with unexported fields the vector was silently dropped on the
// wire, collapsing last-write-wins to "always overwrite".
type ClockValue struct {
	NodeID string
	TS     uint64
	Vector map[string]uint64
}

// After reports whether cv should win a last-write-wins conflict against other.
//
// It first does a proper vector-dominance comparison across the union of all
// node IDs: if cv causally dominates other (cv[n] >= other[n] for every n and
// strictly greater for at least one), cv is newer. If neither dominates the
// writes are concurrent, and we break the tie deterministically (by TS, then
// NodeID) so every node converges on the same winner.
func (cv ClockValue) After(other ClockValue) bool {
	cvDom, otherDom := false, false
	seen := make(map[string]struct{}, len(cv.Vector)+len(other.Vector))
	for n := range cv.Vector {
		seen[n] = struct{}{}
	}
	for n := range other.Vector {
		seen[n] = struct{}{}
	}
	for n := range seen {
		a, b := cv.Vector[n], other.Vector[n]
		if a > b {
			cvDom = true
		}
		if b > a {
			otherDom = true
		}
	}
	switch {
	case cvDom && !otherDom:
		return true // cv causally after other
	case otherDom && !cvDom:
		return false // other causally after cv
	default:
		// Concurrent (or identical): deterministic tie-break.
		if cv.TS != other.TS {
			return cv.TS > other.TS
		}
		return cv.NodeID > other.NodeID
	}
}

// Equal reports whether two clock values are concurrent (neither is after the other).
func (cv ClockValue) Equal(other ClockValue) bool {
	return !cv.After(other) && !other.After(cv)
}
