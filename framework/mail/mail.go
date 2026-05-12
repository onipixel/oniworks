// Package mail provides a fluent email API backed by go-mail (SMTP).
// HTML templates are rendered from Go's html/template package.
// Attachments, CC/BCC, and inline images are supported.
package mail

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	gomail "github.com/wneessen/go-mail"

	"github.com/onipixel/oniworks/framework/config"
)

// Transport is the delivery backend used by a Mailer.
type Transport interface {
	Send(msg *Message) error
}

// Config holds mail settings.
type Config struct {
	Driver     string // "smtp" | "log"  (default: "smtp")
	Host       string
	Port       int
	Username   string
	Password   string
	FromName   string
	FromAddr   string
	Encryption string // "tls", "starttls", "none"  (default: "tls")
	Timeout    time.Duration
}

// Mailer manages message construction and delivery.
type Mailer struct {
	cfg       Config
	transport Transport
	tmpls     *template.Template
	tmplMu    sync.RWMutex
	logger    *slog.Logger
}

// New creates a Mailer. Templates are loaded lazily via LoadTemplates.
func New(cfg Config, logger ...*slog.Logger) *Mailer {
	log := slog.Default()
	if len(logger) > 0 && logger[0] != nil {
		log = logger[0]
	}
	if cfg.Encryption == "" {
		cfg.Encryption = "tls"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 15 * time.Second
	}
	if cfg.Driver == "" {
		cfg.Driver = "smtp"
	}

	var t Transport
	switch strings.ToLower(cfg.Driver) {
	case "log":
		t = &LogTransport{path: "storage/logs/mail.log"}
	default:
		t = &SMTPTransport{cfg: cfg}
	}

	return &Mailer{cfg: cfg, transport: t, logger: log}
}

// NewFromConfig creates a Mailer from oni.Config, reading all mail.* keys.
func NewFromConfig(cfg *config.Config, logger ...*slog.Logger) *Mailer {
	return New(Config{
		Driver:     cfg.String("mail.driver", "smtp"),
		Host:       cfg.String("mail.host", "localhost"),
		Port:       cfg.Int("mail.port", 1025),
		Username:   cfg.String("mail.username", ""),
		Password:   cfg.String("mail.password", ""),
		FromAddr:   cfg.String("mail.from", "noreply@app.com"),
		FromName:   cfg.String("mail.from_name", "App"),
		Encryption: cfg.String("mail.encryption", "none"),
	}, logger...)
}

// ─────────────────────────── Transports ──────────────────────────

// SMTPTransport delivers via SMTP using go-mail.
type SMTPTransport struct{ cfg Config }

func (s *SMTPTransport) Send(msg *Message) error {
	gm := gomail.NewMsg()

	from := msg.from
	if from == "" {
		from = s.cfg.FromAddr
	}
	if err := gm.FromFormat(s.cfg.FromName, from); err != nil {
		return fmt.Errorf("mail: set from: %w", err)
	}
	if err := gm.To(msg.to...); err != nil {
		return fmt.Errorf("mail: set to: %w", err)
	}
	if len(msg.cc) > 0 {
		if err := gm.Cc(msg.cc...); err != nil {
			return fmt.Errorf("mail: set cc: %w", err)
		}
	}
	if len(msg.bcc) > 0 {
		if err := gm.Bcc(msg.bcc...); err != nil {
			return fmt.Errorf("mail: set bcc: %w", err)
		}
	}
	gm.Subject(msg.subject)

	if msg.html != "" && msg.text != "" {
		gm.SetBodyString(gomail.TypeTextPlain, msg.text)
		gm.AddAlternativeString(gomail.TypeTextHTML, msg.html)
	} else if msg.html != "" {
		gm.SetBodyString(gomail.TypeTextHTML, msg.html)
	} else if msg.text != "" {
		gm.SetBodyString(gomail.TypeTextPlain, msg.text)
	}

	for _, a := range msg.attachments {
		ct := mime.TypeByExtension(filepath.Ext(a.name))
		if ct == "" {
			ct = "application/octet-stream"
		}
		if a.inline {
			gm.EmbedReader(a.name, a.reader, gomail.WithFileEncoding(gomail.EncodingB64))
		} else {
			gm.AttachReader(a.name, a.reader, gomail.WithFileEncoding(gomail.EncodingB64))
		}
		_ = ct
	}
	for k, v := range msg.headers {
		gm.SetGenHeader(gomail.Header(k), v)
	}

	client, err := dialSMTP(s.cfg)
	if err != nil {
		return err
	}
	if err := client.DialAndSend(gm); err != nil {
		return fmt.Errorf("mail: send: %w", err)
	}
	return nil
}

// LogTransport writes emails to a log file instead of sending them.
type LogTransport struct{ path string }

func (l *LogTransport) Send(msg *Message) error {
	if err := os.MkdirAll(filepath.Dir(l.path), 0o755); err != nil {
		return fmt.Errorf("mail: log transport mkdir: %w", err)
	}
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("mail: log transport open: %w", err)
	}
	defer f.Close()

	fmt.Fprintf(f, "-------------------- %s --------------------\n", time.Now().Format(time.RFC1123))
	fmt.Fprintf(f, "To:      %s\n", strings.Join(msg.to, ", "))
	fmt.Fprintf(f, "Subject: %s\n", msg.subject)
	if msg.html != "" {
		fmt.Fprintf(f, "Body (HTML):\n%s\n", msg.html)
	} else {
		fmt.Fprintf(f, "Body:\n%s\n", msg.text)
	}
	fmt.Fprintln(f)
	return nil
}

// LoadTemplates parses all *.html files in dir into the template set.
// Template names are relative paths, e.g. "welcome.html".
func (m *Mailer) LoadTemplates(dir string) error {
	pattern := filepath.Join(dir, "*.html")
	t, err := template.ParseGlob(pattern)
	if err != nil {
		return fmt.Errorf("mail: load templates %s: %w", pattern, err)
	}
	m.tmplMu.Lock()
	m.tmpls = t
	m.tmplMu.Unlock()
	return nil
}

// ─────────────────────────── Message builder ──────────────────────

// Message is a fluent email message.
type Message struct {
	mailer  *Mailer
	to      []string
	cc      []string
	bcc     []string
	from    string
	subject string
	html    string
	text    string
	headers map[string]string
	attachments []attachment
	ctx     context.Context
}

type attachment struct {
	name   string
	reader io.Reader
	inline bool
}

// To sets the recipient address(es).
func (m *Message) To(addrs ...string) *Message {
	m.to = append(m.to, addrs...)
	return m
}

// CC adds CC recipients.
func (m *Message) CC(addrs ...string) *Message {
	m.cc = append(m.cc, addrs...)
	return m
}

// BCC adds BCC recipients.
func (m *Message) BCC(addrs ...string) *Message {
	m.bcc = append(m.bcc, addrs...)
	return m
}

// From overrides the default sender address.
func (m *Message) From(addr string) *Message {
	m.from = addr
	return m
}

// Subject sets the email subject.
func (m *Message) Subject(s string) *Message {
	m.subject = s
	return m
}

// HTML sets a raw HTML body.
func (m *Message) HTML(body string) *Message {
	m.html = body
	return m
}

// Text sets a plain-text body.
func (m *Message) Text(body string) *Message {
	m.text = body
	return m
}

// View renders a named HTML template with data as the HTML body.
//
//	mailer.NewMessage(ctx).To("user@example.com").Subject("Welcome!").
//	    View("welcome.html", map[string]any{"Name": "Alice"}).
//	    Send()
func (m *Message) View(tmplName string, data any) *Message {
	m.mailer.tmplMu.RLock()
	t := m.mailer.tmpls
	m.mailer.tmplMu.RUnlock()

	if t == nil {
		m.html = fmt.Sprintf("<p>Template engine not loaded. Template: %s</p>", tmplName)
		return m
	}
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, tmplName, data); err != nil {
		m.html = fmt.Sprintf("<p>Template error: %v</p>", err)
		return m
	}
	m.html = buf.String()
	return m
}

// Attach adds a file attachment by path.
func (m *Message) Attach(path string) *Message {
	f, err := os.Open(path)
	if err != nil {
		return m
	}
	m.attachments = append(m.attachments, attachment{
		name:   filepath.Base(path),
		reader: f,
	})
	return m
}

// AttachReader adds an attachment from an io.Reader.
func (m *Message) AttachReader(name string, r io.Reader) *Message {
	m.attachments = append(m.attachments, attachment{name: name, reader: r})
	return m
}

// Header adds a custom email header.
func (m *Message) Header(key, value string) *Message {
	if m.headers == nil {
		m.headers = make(map[string]string)
	}
	m.headers[key] = value
	return m
}

// Send delivers the message via the configured transport.
func (m *Message) Send() error {
	if len(m.to) == 0 {
		return fmt.Errorf("mail: no recipients")
	}
	if err := m.mailer.transport.Send(m); err != nil {
		return err
	}
	m.mailer.logger.Debug("mail: sent", "to", strings.Join(m.to, ", "), "subject", m.subject)
	return nil
}

// ─────────────────────────── Mailer helpers ──────────────────────

// NewMessage creates a fluent message builder for ctx.
func (m *Mailer) NewMessage(ctx context.Context) *Message {
	return &Message{mailer: m, ctx: ctx}
}

func dialSMTP(cfg Config) (*gomail.Client, error) {
	opts := []gomail.Option{
		gomail.WithPort(cfg.Port),
		gomail.WithSMTPAuth(gomail.SMTPAuthPlain),
		gomail.WithUsername(cfg.Username),
		gomail.WithPassword(cfg.Password),
		gomail.WithTimeout(cfg.Timeout),
	}

	switch strings.ToLower(cfg.Encryption) {
	case "tls":
		opts = append(opts, gomail.WithSSL())
	case "starttls":
		opts = append(opts, gomail.WithTLSPolicy(gomail.TLSMandatory))
	case "none":
		opts = append(opts, gomail.WithTLSPolicy(gomail.NoTLS))
	default:
		opts = append(opts, gomail.WithSSL())
	}

	c, err := gomail.NewClient(cfg.Host, opts...)
	if err != nil {
		return nil, fmt.Errorf("mail: create client: %w", err)
	}
	return c, nil
}

// Send is a convenience one-liner.
//
//	mail.Send(ctx, mailer, "user@example.com", "Welcome!", "welcome.html", data)
func Send(ctx context.Context, m *Mailer, to, subject, tmpl string, data any) error {
	return m.NewMessage(ctx).To(to).Subject(subject).View(tmpl, data).Send()
}
