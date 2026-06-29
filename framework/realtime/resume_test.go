package realtime

import (
	"strconv"
	"testing"
	"time"
)

func ev(id string) *Event { return &Event{ID: id, Type: "msg", Ts: time.Now().Unix()} }

// TestSinceReplaysAfterMarker verifies the normal case: only events after the
// client's last_event_id are replayed.
func TestSinceReplaysAfterMarker(t *testing.T) {
	eb := NewEventBuffer(100, time.Minute)
	for i := 1; i <= 5; i++ {
		eb.Push("chat", ev(strconv.Itoa(i)))
	}
	got := eb.Since("chat", "3")
	if len(got) != 2 || got[0].ID != "4" || got[1].ID != "5" {
		t.Fatalf("expected events 4,5 after marker 3, got %v", ids(got))
	}
}

// TestSinceReplaysAllWhenEmpty verifies "" returns the whole buffer.
func TestSinceReplaysAllWhenEmpty(t *testing.T) {
	eb := NewEventBuffer(100, time.Minute)
	for i := 1; i <= 3; i++ {
		eb.Push("chat", ev(strconv.Itoa(i)))
	}
	if got := eb.Since("chat", ""); len(got) != 3 {
		t.Fatalf("empty marker should replay all 3, got %v", ids(got))
	}
}

// TestSinceReplaysWindowWhenMarkerAgedOut is the silent-loss regression: when
// the client's last_event_id has been evicted from the ring, the buffer must
// replay the whole window, not send nothing.
func TestSinceReplaysWindowWhenMarkerAgedOut(t *testing.T) {
	eb := NewEventBuffer(3, time.Minute) // tiny ring
	for i := 1; i <= 6; i++ {            // 1,2,3 evicted; 4,5,6 remain
		eb.Push("chat", ev(strconv.Itoa(i)))
	}
	got := eb.Since("chat", "2") // "2" has aged out
	if len(got) != 3 {
		t.Fatalf("aged-out marker should replay the whole window (3 events), got %d: %v", len(got), ids(got))
	}
}

func ids(evs []*Event) []string {
	out := make([]string, len(evs))
	for i, e := range evs {
		out[i] = e.ID
	}
	return out
}
