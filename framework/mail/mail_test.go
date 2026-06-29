package mail

import (
	"context"
	"strings"
	"testing"

	"github.com/onipixel/oniworks/framework/config"
)

// captureTransport records the last message instead of sending it.
type captureTransport struct{ last *Message }

func (c *captureTransport) Send(m *Message) error {
	c.last = m
	return nil
}

func newTestMailer() (*Mailer, *captureTransport) {
	cap := &captureTransport{}
	m := New(Config{Driver: "log", FromAddr: "noreply@app.test"})
	m.transport = cap
	return m, cap
}

// TestMessageBuildAndSend verifies the fluent builder sets fields and the
// transport receives them.
func TestMessageBuildAndSend(t *testing.T) {
	m, cap := newTestMailer()
	err := m.NewMessage(context.Background()).
		To("user@example.com").
		Subject("Welcome").
		HTML("<p>hi</p>").
		Text("hi").
		Send()
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if cap.last == nil {
		t.Fatal("transport received no message")
	}
	if len(cap.last.to) != 1 || cap.last.to[0] != "user@example.com" {
		t.Errorf("to = %v", cap.last.to)
	}
	if cap.last.subject != "Welcome" || cap.last.html != "<p>hi</p>" || cap.last.text != "hi" {
		t.Errorf("fields not set: %+v", cap.last)
	}
}

// TestSendRequiresRecipient verifies sending with no recipients errors.
func TestSendRequiresRecipient(t *testing.T) {
	m, _ := newTestMailer()
	if err := m.NewMessage(context.Background()).Subject("x").Send(); err == nil {
		t.Fatal("expected error sending with no recipients")
	}
}

// TestViewWithoutTemplatesFallsBack verifies View degrades gracefully when no
// templates are loaded (no panic, fallback body).
func TestViewWithoutTemplatesFallsBack(t *testing.T) {
	m, cap := newTestMailer()
	_ = m.NewMessage(context.Background()).
		To("a@b.com").Subject("s").View("welcome.html", map[string]any{"Name": "Al"}).Send()
	if !strings.Contains(cap.last.html, "welcome.html") {
		t.Errorf("expected fallback referencing the template, got %q", cap.last.html)
	}
}

// TestNewDefaultsToTLS verifies the secure default encryption.
func TestNewDefaultsToTLS(t *testing.T) {
	m := New(Config{})
	if m.cfg.Encryption != "tls" {
		t.Fatalf("New default encryption = %q, want tls", m.cfg.Encryption)
	}
}

// TestNewFromConfigDefaultsToTLS is the cleartext-default regression: a config
// without mail.encryption must NOT fall back to "none" (plaintext AUTH).
func TestNewFromConfigDefaultsToTLS(t *testing.T) {
	cfg := config.New()
	cfg.Set("mail.driver", "smtp")
	cfg.Set("mail.host", "smtp.example.com")
	// deliberately leave mail.encryption unset

	m := NewFromConfig(cfg)
	if m.cfg.Encryption != "tls" {
		t.Fatalf("NewFromConfig default encryption = %q, want tls (no cleartext fallback)", m.cfg.Encryption)
	}
}
