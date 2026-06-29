package memory

import (
	"bytes"
	"encoding/gob"
	"testing"
)

// TestClockSurvivesGob is the wire-loss regression: ClockValue must round-trip
// through gob (the gossip transport) without dropping its vector — exported
// fields. Previously unexported fields were silently lost.
func TestClockSurvivesGob(t *testing.T) {
	orig := ClockValue{NodeID: "node-a", TS: 5, Vector: map[string]uint64{"node-a": 5, "node-b": 2}}

	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(orig); err != nil {
		t.Fatalf("encode: %v", err)
	}
	var got ClockValue
	if err := gob.NewDecoder(&buf).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.NodeID != "node-a" || got.TS != 5 {
		t.Fatalf("scalar fields lost: %+v", got)
	}
	if got.Vector["node-a"] != 5 || got.Vector["node-b"] != 2 {
		t.Fatalf("vector lost over the wire: %+v", got.Vector)
	}
}

// TestClockCausalOrdering verifies vector-dominance comparison: a clock that
// dominates another is correctly ordered After it, regardless of node identity.
func TestClockCausalOrdering(t *testing.T) {
	older := ClockValue{NodeID: "b", TS: 1, Vector: map[string]uint64{"a": 1, "b": 1}}
	newer := ClockValue{NodeID: "a", TS: 2, Vector: map[string]uint64{"a": 2, "b": 1}}

	if !newer.After(older) {
		t.Fatal("newer (dominates) should be After older")
	}
	if older.After(newer) {
		t.Fatal("older must not be After newer")
	}
}

// TestClockConcurrentDeterministic verifies concurrent writes resolve to the
// SAME winner from either node's perspective (convergence).
func TestClockConcurrentDeterministic(t *testing.T) {
	// Concurrent: a advanced its own lane, b advanced its own lane.
	x := ClockValue{NodeID: "a", TS: 1, Vector: map[string]uint64{"a": 1}}
	y := ClockValue{NodeID: "b", TS: 1, Vector: map[string]uint64{"b": 1}}

	// Exactly one direction must win, and it must be stable.
	if x.After(y) == y.After(x) {
		t.Fatalf("concurrent clocks must have a single deterministic winner (x.After=%v y.After=%v)", x.After(y), y.After(x))
	}
}

// TestClockEqualIdentical verifies identical clocks are Equal (concurrent).
func TestClockEqualIdentical(t *testing.T) {
	a := ClockValue{NodeID: "a", TS: 3, Vector: map[string]uint64{"a": 3}}
	b := ClockValue{NodeID: "a", TS: 3, Vector: map[string]uint64{"a": 3}}
	if !a.Equal(b) {
		t.Fatal("identical clocks should be Equal")
	}
}

// TestVectorClockMergeAndTick exercises the live VectorClock.
func TestVectorClockMergeAndTick(t *testing.T) {
	vc := NewVectorClock("a")
	c1 := vc.Tick()
	if c1.Vector["a"] != 1 {
		t.Fatalf("tick should advance lane a to 1, got %v", c1.Vector)
	}
	vc.Merge(ClockValue{NodeID: "b", TS: 9, Vector: map[string]uint64{"b": 9}})
	c2 := vc.Tick()
	if c2.Vector["b"] != 9 {
		t.Fatalf("merge should retain b=9, got %v", c2.Vector)
	}
	if c2.Vector["a"] != 2 {
		t.Fatalf("tick after merge should advance a to 2, got %v", c2.Vector)
	}
}
