package memory

import (
	"sync"
	"testing"
	"time"
)

// TestPubSubDoubleCancelSafe verifies the cancel function is idempotent — under
// the old code a second call panicked with "close of closed channel".
func TestPubSubDoubleCancelSafe(t *testing.T) {
	ps := newPubSub()
	cancel := ps.subscribe("topic", func(string, any) {})
	cancel()
	cancel() // must not panic
}

// TestPubSubPublishCancelRace drives concurrent publish and unsubscribe on the
// same topics. Under the old code a publish could send on a buffer that cancel
// had just closed, panicking the process. This must complete cleanly.
func TestPubSubPublishCancelRace(t *testing.T) {
	ps := newPubSub()

	done := make(chan struct{})
	var wg sync.WaitGroup

	// Continuous publishers.
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				default:
					ps.publish("orders.new", "payload")
				}
			}
		}()
	}

	// Churning subscribers that subscribe then immediately cancel.
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 500; j++ {
				cancel := ps.subscribe("orders.*", func(string, any) {})
				cancel()
			}
		}()
	}

	time.Sleep(50 * time.Millisecond)
	close(done)
	wg.Wait()
}

// TestPubSubDeliversWhileOpen sanity-checks that delivery still works after the
// close-safety changes.
func TestPubSubDeliversWhileOpen(t *testing.T) {
	ps := newPubSub()
	got := make(chan any, 1)
	cancel := ps.subscribe("chat.room1", func(_ string, p any) { got <- p })
	defer cancel()

	ps.publish("chat.room1", "hello")
	select {
	case p := <-got:
		if p != "hello" {
			t.Fatalf("got %v, want hello", p)
		}
	case <-time.After(time.Second):
		t.Fatal("message was not delivered")
	}
}
