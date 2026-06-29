// Package backup provides database backup and restore utilities.
// Currently supports PostgreSQL (via pg_dump/psql) and MySQL (via mysqldump/mysql).
// Backups are compressed with gzip and can be stored on any storage.Disk.
package backup

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Driver represents a database dialect.
type Driver string

const (
	DriverPostgres Driver = "postgres"
	DriverMySQL    Driver = "mysql"
)

// Config holds database connection info for the backup tool.
type Config struct {
	Driver   Driver
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	// BackupDir is where backup files are stored locally.
	BackupDir string
	Logger    *slog.Logger
}

// Manager handles backup and restore operations.
type Manager struct {
	cfg    Config
	logger *slog.Logger
}

// New creates a backup Manager.
func New(cfg Config) *Manager {
	log := cfg.Logger
	if log == nil {
		log = slog.Default()
	}
	if cfg.BackupDir == "" {
		cfg.BackupDir = "./backups"
	}
	if cfg.Port == 0 {
		if cfg.Driver == DriverMySQL {
			cfg.Port = 3306
		} else {
			cfg.Port = 5432
		}
	}
	return &Manager{cfg: cfg, logger: log}
}

// Backup creates a compressed backup and returns the file path.
func (m *Manager) Backup(ctx context.Context) (string, error) {
	if err := os.MkdirAll(m.cfg.BackupDir, 0o750); err != nil {
		return "", fmt.Errorf("backup: mkdir: %w", err)
	}

	ts := time.Now().UTC().Format("20060102-150405")
	filename := fmt.Sprintf("%s-%s.sql.gz", m.cfg.DBName, ts)
	path := filepath.Join(m.cfg.BackupDir, filename)

	f, err := os.Create(path)
	if err != nil {
		return "", fmt.Errorf("backup: create file: %w", err)
	}
	defer f.Close()

	gz := gzip.NewWriter(f)
	defer gz.Close()

	var cmd *exec.Cmd
	switch m.cfg.Driver {
	case DriverPostgres:
		cmd = m.pgDumpCmd(ctx)
	case DriverMySQL:
		cmd = m.mysqlDumpCmd(ctx)
	default:
		return "", fmt.Errorf("backup: unsupported driver %q", m.cfg.Driver)
	}

	cmd.Stdout = gz
	cmd.Stderr = logWriter{log: m.logger}

	m.logger.Info("backup: starting", "db", m.cfg.DBName, "file", path)
	if err := cmd.Run(); err != nil {
		_ = os.Remove(path)
		return "", fmt.Errorf("backup: dump failed: %w", err)
	}

	if err := gz.Close(); err != nil {
		return "", fmt.Errorf("backup: compress: %w", err)
	}

	fi, _ := os.Stat(path)
	m.logger.Info("backup: complete", "file", path, "size", fi.Size())
	return path, nil
}

// Restore restores a database from a .sql.gz file.
func (m *Manager) Restore(ctx context.Context, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("restore: open %q: %w", path, err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("restore: decompress: %w", err)
	}
	defer gz.Close()

	var cmd *exec.Cmd
	switch m.cfg.Driver {
	case DriverPostgres:
		cmd = m.psqlCmd(ctx)
	case DriverMySQL:
		cmd = m.mysqlCmd(ctx)
	default:
		return fmt.Errorf("restore: unsupported driver %q", m.cfg.Driver)
	}

	cmd.Stdin = gz
	cmd.Stderr = logWriter{log: m.logger}

	m.logger.Info("restore: starting", "db", m.cfg.DBName, "file", path)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("restore: failed: %w", err)
	}
	m.logger.Info("restore: complete", "db", m.cfg.DBName)
	return nil
}

// List returns all backup files in the backup directory, newest first.
func (m *Manager) List() ([]BackupFile, error) {
	entries, err := os.ReadDir(m.cfg.BackupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var files []BackupFile
	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql.gz") {
			continue
		}
		info, _ := e.Info()
		files = append(files, BackupFile{
			Name: e.Name(),
			Path: filepath.Join(m.cfg.BackupDir, e.Name()),
			Size: info.Size(),
			At:   info.ModTime(),
		})
	}
	return files, nil
}

// BackupFile describes a backup on disk.
type BackupFile struct {
	Name string
	Path string
	Size int64
	At   time.Time
}

// ─────────────────────────── Command builders ─────────────────────

func (m *Manager) pgDumpCmd(ctx context.Context) *exec.Cmd {
	args := []string{
		"-h", m.cfg.Host,
		"-p", fmt.Sprintf("%d", m.cfg.Port),
		"-U", m.cfg.User,
		"-d", m.cfg.DBName,
		"--no-password",
	}
	cmd := exec.CommandContext(ctx, "pg_dump", args...)
	cmd.Env = append(os.Environ(), "PGPASSWORD="+m.cfg.Password)
	return cmd
}

func (m *Manager) psqlCmd(ctx context.Context) *exec.Cmd {
	args := []string{
		"-h", m.cfg.Host,
		"-p", fmt.Sprintf("%d", m.cfg.Port),
		"-U", m.cfg.User,
		"-d", m.cfg.DBName,
		"--no-password",
	}
	cmd := exec.CommandContext(ctx, "psql", args...)
	cmd.Env = append(os.Environ(), "PGPASSWORD="+m.cfg.Password)
	return cmd
}

func (m *Manager) mysqlDumpCmd(ctx context.Context) *exec.Cmd {
	args := []string{
		fmt.Sprintf("-h%s", m.cfg.Host),
		fmt.Sprintf("-P%d", m.cfg.Port),
		fmt.Sprintf("-u%s", m.cfg.User),
		m.cfg.DBName,
	}
	cmd := exec.CommandContext(ctx, "mysqldump", args...)
	// Pass the password via MYSQL_PWD env instead of "-p<pw>" on argv, where it
	// would be visible to any local user via `ps`/proc.
	cmd.Env = append(os.Environ(), "MYSQL_PWD="+m.cfg.Password)
	return cmd
}

func (m *Manager) mysqlCmd(ctx context.Context) *exec.Cmd {
	args := []string{
		fmt.Sprintf("-h%s", m.cfg.Host),
		fmt.Sprintf("-P%d", m.cfg.Port),
		fmt.Sprintf("-u%s", m.cfg.User),
		m.cfg.DBName,
	}
	cmd := exec.CommandContext(ctx, "mysql", args...)
	cmd.Env = append(os.Environ(), "MYSQL_PWD="+m.cfg.Password)
	return cmd
}

// ─────────────────────────── Log writer ───────────────────────────

type logWriter struct{ log *slog.Logger }

func (lw logWriter) Write(p []byte) (int, error) {
	for _, line := range strings.Split(strings.TrimRight(string(p), "\n"), "\n") {
		if line != "" {
			lw.log.Warn("backup/db: " + line)
		}
	}
	return len(p), nil
}

// ─────────────────────────── Storage upload helpers ───────────────

// Uploader is an interface for pushing a backup to remote storage.
type Uploader interface {
	Put(ctx context.Context, path string, r io.Reader, opts ...any) error
}

// Upload compresses and streams a dump directly to a remote disk.
// This is more memory-efficient than writing to disk first.
func Upload(ctx context.Context, m *Manager, disk interface {
	Put(ctx context.Context, path string, r io.Reader) error
}, remotePath string) error {
	pr, pw := io.Pipe()

	errCh := make(chan error, 1)
	go func() {
		gz := gzip.NewWriter(pw)
		var cmd *exec.Cmd
		switch m.cfg.Driver {
		case DriverPostgres:
			cmd = m.pgDumpCmd(ctx)
		case DriverMySQL:
			cmd = m.mysqlDumpCmd(ctx)
		default:
			_ = pw.CloseWithError(fmt.Errorf("unsupported driver"))
			errCh <- fmt.Errorf("unsupported driver")
			return
		}
		cmd.Stdout = gz
		cmd.Stderr = logWriter{log: m.logger}
		if err := cmd.Run(); err != nil {
			_ = pw.CloseWithError(err)
			errCh <- err
			return
		}
		_ = gz.Close()
		_ = pw.Close()
		errCh <- nil
	}()

	if err := disk.Put(ctx, remotePath, pr); err != nil {
		return err
	}
	return <-errCh
}
