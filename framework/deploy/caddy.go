// Package deploy manages Caddy as an automatic TLS reverse proxy.
// Caddy is started as a managed subprocess with a generated Caddyfile.
// Auto-TLS via Let's Encrypt is enabled by default when a domain is provided.
package deploy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"text/template"
	"time"
)

// Config holds the Caddy reverse-proxy configuration.
type Config struct {
	// Domain is the public domain name, e.g. "example.com".
	// When set, Caddy enables automatic HTTPS via Let's Encrypt.
	Domain string

	// Email is the ACME account email for Let's Encrypt notifications.
	Email string

	// AppAddr is the address your Go app listens on (Caddy proxies to this).
	// Default: "localhost:8080"
	AppAddr string

	// CaddyBin is the path to the caddy binary (default: "caddy" in PATH).
	CaddyBin string

	// DataDir is where Caddy stores TLS certificates and state.
	// Default: "./data/caddy"
	DataDir string

	// AdminAddr is Caddy's admin API address.
	// Default: "localhost:2019"
	AdminAddr string

	// ExtraDirectives are appended verbatim to the site block.
	ExtraDirectives string

	// Logger defaults to slog.Default().
	Logger *slog.Logger
}

// Manager controls a Caddy subprocess.
type Manager struct {
	cfg    Config
	cmd    *exec.Cmd
	mu     sync.Mutex
	logger *slog.Logger
}

// NewManager creates a Manager. Call Start to launch Caddy.
func NewManager(cfg Config) *Manager {
	if cfg.AppAddr == "" {
		cfg.AppAddr = "localhost:8080"
	}
	if cfg.CaddyBin == "" {
		cfg.CaddyBin = "caddy"
	}
	if cfg.DataDir == "" {
		cfg.DataDir = "./data/caddy"
	}
	if cfg.AdminAddr == "" {
		cfg.AdminAddr = "localhost:2019"
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &Manager{cfg: cfg, logger: cfg.Logger}
}

// Start generates a Caddyfile and launches the Caddy process.
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := os.MkdirAll(m.cfg.DataDir, 0o755); err != nil {
		return fmt.Errorf("deploy: mkdir data: %w", err)
	}

	caddyfile, err := m.writeCaddyfile()
	if err != nil {
		return err
	}

	bin, err := exec.LookPath(m.cfg.CaddyBin)
	if err != nil {
		return fmt.Errorf("deploy: caddy binary not found (%q): %w", m.cfg.CaddyBin, err)
	}

	// #nosec G204 — CaddyBin is from trusted config
	cmd := exec.CommandContext(ctx, bin, "run", "--config", caddyfile, "--adapter", "caddyfile")
	cmd.Env = append(os.Environ(),
		"HOME="+m.cfg.DataDir,
		"XDG_DATA_HOME="+m.cfg.DataDir,
	)
	cmd.Stdout = logWriter{log: m.logger, level: slog.LevelInfo, prefix: "caddy: "}
	cmd.Stderr = logWriter{log: m.logger, level: slog.LevelWarn, prefix: "caddy: "}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("deploy: start caddy: %w", err)
	}
	m.cmd = cmd
	m.logger.Info("deploy: caddy started", "pid", cmd.Process.Pid, "config", caddyfile)
	return nil
}

// Stop gracefully shuts down the Caddy process.
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cmd == nil || m.cmd.Process == nil {
		return nil
	}
	if err := m.cmd.Process.Signal(os.Interrupt); err != nil {
		_ = m.cmd.Process.Kill()
	}
	return m.cmd.Wait()
}

// Reload triggers a configuration reload via the Caddy admin API (zero-downtime).
func (m *Manager) Reload(ctx context.Context) error {
	caddyfile, err := m.writeCaddyfile()
	if err != nil {
		return err
	}

	data, err := os.ReadFile(caddyfile)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("http://%s/load", m.cfg.AdminAddr)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url,
		bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "text/caddyfile")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("deploy: reload: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("deploy: reload failed (%d): %s", resp.StatusCode, body)
	}
	m.logger.Info("deploy: caddy reloaded")
	return nil
}

// ─────────────────────────── Caddyfile generation ─────────────────

var caddyfileTemplate = template.Must(template.New("caddyfile").Parse(`
{
	admin {{.AdminAddr}}
	{{- if .Email}}
	email {{.Email}}
	{{- end}}
	storage file_system {
		root {{.DataDir}}
	}
}

{{.SiteAddr}} {
	reverse_proxy {{.AppAddr}}

	{{- if .ExtraDirectives}}
	{{.ExtraDirectives}}
	{{- end}}

	log {
		output stdout
		format json
	}
}
`))

type caddyfileData struct {
	AdminAddr       string
	Email           string
	DataDir         string
	SiteAddr        string
	AppAddr         string
	ExtraDirectives string
}

func (m *Manager) writeCaddyfile() (string, error) {
	siteAddr := m.cfg.Domain
	if siteAddr == "" {
		// Local dev: plain HTTP
		siteAddr = ":80"
	}

	data := caddyfileData{
		AdminAddr:       m.cfg.AdminAddr,
		Email:           m.cfg.Email,
		DataDir:         filepath.ToSlash(m.cfg.DataDir),
		SiteAddr:        siteAddr,
		AppAddr:         m.cfg.AppAddr,
		ExtraDirectives: m.cfg.ExtraDirectives,
	}

	var buf bytes.Buffer
	if err := caddyfileTemplate.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("deploy: generate caddyfile: %w", err)
	}

	path := filepath.Join(m.cfg.DataDir, "Caddyfile")
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		return "", fmt.Errorf("deploy: write caddyfile: %w", err)
	}
	return path, nil
}

// GenerateCaddyJSON generates a Caddy JSON config (useful for advanced scenarios).
func (m *Manager) GenerateCaddyJSON() ([]byte, error) {
	apps := map[string]any{
		"http": map[string]any{
			"servers": map[string]any{
				"srv0": map[string]any{
					"listen": []string{":443"},
					"routes": []map[string]any{
						{
							"handle": []map[string]any{
								{
									"handler": "reverse_proxy",
									"upstreams": []map[string]any{
										{"dial": m.cfg.AppAddr},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	if m.cfg.Domain != "" {
		apps["tls"] = map[string]any{
			"automation": map[string]any{
				"policies": []map[string]any{
					{
						"subjects": []string{m.cfg.Domain},
						"issuers": []map[string]any{
							{
								"module": "acme",
								"email":  m.cfg.Email,
							},
						},
					},
				},
			},
		}
	}

	return json.MarshalIndent(map[string]any{"apps": apps}, "", "  ")
}

// ─────────────────────────── Log writer ───────────────────────────

type logWriter struct {
	log    *slog.Logger
	level  slog.Level
	prefix string
}

func (lw logWriter) Write(p []byte) (int, error) {
	lines := strings.Split(strings.TrimRight(string(p), "\n"), "\n")
	for _, line := range lines {
		if line != "" {
			lw.log.Log(context.Background(), lw.level, lw.prefix+line)
		}
	}
	return len(p), nil
}
