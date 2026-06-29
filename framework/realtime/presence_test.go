package realtime

import (
	"testing"

	"github.com/onipixel/oniworks/framework/memory"
)

// TestToPresenceInfoFromMap is the cross-node regression: presence entries that
// arrive from another node as map[string]any (JSON) must still decode, or
// remote members are silently dropped from Members/Count.
func TestToPresenceInfoFromMap(t *testing.T) {
	// Simulate a JSON-decoded entry from a remote node.
	remote := map[string]any{
		"user_id": float64(42), // JSON numbers are float64
		"conn_id": "conn-remote",
		"meta":    map[string]any{"name": "alice"},
	}
	pi, ok := toPresenceInfo(remote)
	if !ok {
		t.Fatal("remote map entry should decode to PresenceInfo")
	}
	if pi.UserID != 42 || pi.ConnID != "conn-remote" {
		t.Fatalf("decoded fields wrong: %+v", pi)
	}
}

// TestPresenceMembersCountsLocalAndMap verifies Members counts both concrete and
// map-shaped entries living in the store.
func TestPresenceMembersCountsLocalAndMap(t *testing.T) {
	mem := memory.New(memory.Options{})
	pm := newPresenceManager(mem)

	// Local concrete entry.
	pm.Join("room.1", PresenceInfo{UserID: 1, ConnID: "local"})
	// Simulate a remote entry stored as a map (as the Redis adapter would).
	mem.Set(presenceKey("room.1", "remote"), map[string]any{
		"user_id": float64(2), "conn_id": "remote",
	}, presenceTTL)

	if got := pm.Count("room.1"); got != 2 {
		t.Fatalf("Count = %d, want 2 (local + remote)", got)
	}
}

// TestLeaveAllAcrossChannels verifies LeaveAll removes a connection's presence
// in every channel (depends on the mid-pattern glob fix).
func TestLeaveAllAcrossChannels(t *testing.T) {
	mem := memory.New(memory.Options{})
	pm := newPresenceManager(mem)

	pm.Join("room.1", PresenceInfo{UserID: 1, ConnID: "c1"})
	pm.Join("room.2", PresenceInfo{UserID: 1, ConnID: "c1"})
	pm.Join("room.1", PresenceInfo{UserID: 2, ConnID: "c2"})

	pm.LeaveAll("c1")

	if got := pm.Count("room.1"); got != 1 {
		t.Fatalf("room.1 count = %d, want 1 (c2 remains)", got)
	}
	if got := pm.Count("room.2"); got != 0 {
		t.Fatalf("room.2 count = %d, want 0 (c1 left)", got)
	}
}
