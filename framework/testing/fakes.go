package testing

import (
	"sync"
	stdtesting "testing"
	"time"
)

// ─────────────────────────── FakeMail ─────────────────────────────

// FakeMail records outgoing emails for use in tests.
// Inject it in place of a real mail.Mailer to avoid actual SMTP.
type FakeMail struct {
	mu   sync.RWMutex
	Sent []FakeMessage
}

// FakeMessage holds a recorded email.
type FakeMessage struct {
	To      []string
	Subject string
	HTML    string
	Text    string
	SentAt  time.Time
}

// Record adds a message to the sent list.
func (f *FakeMail) Record(msg FakeMessage) {
	msg.SentAt = time.Now()
	f.mu.Lock()
	f.Sent = append(f.Sent, msg)
	f.mu.Unlock()
}

// Count returns the number of emails sent.
func (f *FakeMail) Count() int {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return len(f.Sent)
}

// Last returns the most recently recorded message.
func (f *FakeMail) Last() (FakeMessage, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if len(f.Sent) == 0 {
		return FakeMessage{}, false
	}
	return f.Sent[len(f.Sent)-1], true
}

// AssertSent fails the test if no email was sent to the given address.
func (f *FakeMail) AssertSent(t *stdtesting.T, to string) {
	t.Helper()
	f.mu.RLock()
	defer f.mu.RUnlock()
	for _, m := range f.Sent {
		for _, r := range m.To {
			if r == to {
				return
			}
		}
	}
	t.Errorf("fake mail: no email sent to %q", to)
}

// AssertNotSent fails the test if any email was sent to the given address.
func (f *FakeMail) AssertNotSent(t *stdtesting.T, to string) {
	t.Helper()
	f.mu.RLock()
	defer f.mu.RUnlock()
	for _, m := range f.Sent {
		for _, r := range m.To {
			if r == to {
				t.Errorf("fake mail: unexpected email sent to %q", to)
				return
			}
		}
	}
}

// Reset clears all recorded messages.
func (f *FakeMail) Reset() {
	f.mu.Lock()
	f.Sent = f.Sent[:0]
	f.mu.Unlock()
}

// ─────────────────────────── FakeQueue ────────────────────────────

// FakeQueue records dispatched jobs for use in tests.
type FakeQueue struct {
	mu   sync.RWMutex
	Jobs []FakeJob
}

// FakeJob records a dispatched job.
type FakeJob struct {
	Class   string
	Queue   string
	Payload any
	Delay   time.Duration
}

// Record adds a job to the fake queue.
func (fq *FakeQueue) Record(job FakeJob) {
	fq.mu.Lock()
	fq.Jobs = append(fq.Jobs, job)
	fq.mu.Unlock()
}

// Count returns the number of dispatched jobs.
func (fq *FakeQueue) Count() int {
	fq.mu.RLock()
	defer fq.mu.RUnlock()
	return len(fq.Jobs)
}

// AssertDispatched fails if no job of the given class was dispatched.
func (fq *FakeQueue) AssertDispatched(t *stdtesting.T, class string) {
	t.Helper()
	fq.mu.RLock()
	defer fq.mu.RUnlock()
	for _, j := range fq.Jobs {
		if j.Class == class {
			return
		}
	}
	t.Errorf("fake queue: job %q was not dispatched", class)
}

// AssertNotDispatched fails if a job of the given class was dispatched.
func (fq *FakeQueue) AssertNotDispatched(t *stdtesting.T, class string) {
	t.Helper()
	fq.mu.RLock()
	defer fq.mu.RUnlock()
	for _, j := range fq.Jobs {
		if j.Class == class {
			t.Errorf("fake queue: job %q was unexpectedly dispatched", class)
			return
		}
	}
}

// Reset clears all recorded jobs.
func (fq *FakeQueue) Reset() {
	fq.mu.Lock()
	fq.Jobs = fq.Jobs[:0]
	fq.mu.Unlock()
}

// ─────────────────────────── FakeStorage ──────────────────────────

// FakeStorage is an in-memory storage.Disk for use in tests.
type FakeStorage struct {
	mu      sync.RWMutex
	Objects map[string][]byte
}

// NewFakeStorage creates an empty FakeStorage.
func NewFakeStorage() *FakeStorage {
	return &FakeStorage{Objects: make(map[string][]byte)}
}

// Has reports whether the given path has been stored.
func (fs *FakeStorage) Has(path string) bool {
	fs.mu.RLock()
	defer fs.mu.RUnlock()
	_, ok := fs.Objects[path]
	return ok
}

// Get returns the stored bytes for path.
func (fs *FakeStorage) Get(path string) ([]byte, bool) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()
	b, ok := fs.Objects[path]
	return b, ok
}

// AssertStored fails if path was not stored.
func (fs *FakeStorage) AssertStored(t *stdtesting.T, path string) {
	t.Helper()
	if !fs.Has(path) {
		t.Errorf("fake storage: %q was not stored", path)
	}
}

// AssertNotStored fails if path was stored.
func (fs *FakeStorage) AssertNotStored(t *stdtesting.T, path string) {
	t.Helper()
	if fs.Has(path) {
		t.Errorf("fake storage: %q was unexpectedly stored", path)
	}
}

// Reset removes all stored objects.
func (fs *FakeStorage) Reset() {
	fs.mu.Lock()
	fs.Objects = make(map[string][]byte)
	fs.mu.Unlock()
}
