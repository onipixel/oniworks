// Package migrations provides the database migration runner and schema builder for OniWorks.
package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"
)

// ─────────────────────────── Global registry ──────────────────────

var (
	registryMu sync.Mutex
	registry   []namedMigration
)

// Register adds a migration to the global application registry.
// Call this from init() in each migration file so it is auto-discovered.
//
//	func init() {
//	    migrations.Register("20240101000000_create_users_table", &CreateUsersTable{})
//	}
func Register(name string, m Migration) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry = append(registry, namedMigration{name: name, m: m})
}

// LoadRegistry copies all globally registered migrations into this Migrator.
// Call this in main.go after importing migration packages as side-effects.
func (mg *Migrator) LoadRegistry() {
	registryMu.Lock()
	defer registryMu.Unlock()
	mg.migrations = append(mg.migrations, registry...)
}

// Migration is the interface every migration struct must implement.
// Up and Down receive a Schema builder — they queue DDL statements by
// calling schema.Create, schema.Drop, schema.Raw, etc.  The actual SQL
// is executed (and errors surfaced) by the Migrator, not the migration.
type Migration interface {
	Up(s *Schema)
	Down(s *Schema)
}

// namedMigration pairs a timestamp-prefixed name with its Migration implementation.
type namedMigration struct {
	name string
	m    Migration
}

// Migrator runs, rolls back, and reports the status of migrations against a database.
type Migrator struct {
	db         *sql.DB
	driver     string // "postgres" or "mysql"
	migrations []namedMigration
	logger     *slog.Logger
	tableName  string
}

// New creates a Migrator.
func New(db *sql.DB, driver string) *Migrator {
	return &Migrator{
		db:        db,
		driver:    driver,
		logger:    slog.Default(),
		tableName: "oni_migrations",
	}
}

// Register adds a migration to the Migrator's list.
//
//	m.Register("2024_01_01_000000_create_users_table", &CreateUsersTable{})
func (m *Migrator) Register(name string, migration Migration) *Migrator {
	m.migrations = append(m.migrations, namedMigration{name: name, m: migration})
	return m
}

// Migrate runs all pending migrations in ascending timestamp order.
func (m *Migrator) Migrate(ctx context.Context) error {
	if err := m.ensureTable(ctx); err != nil {
		return err
	}
	ran, err := m.getRan(ctx)
	if err != nil {
		return err
	}

	m.sortMigrations()
	batch := m.nextBatch(ctx)
	ranCount := 0

	for _, nm := range m.migrations {
		if ran[nm.name] {
			continue
		}
		m.logger.Info("migrating", "name", nm.name)
		s := newSchema(m.db, m.driver)
		nm.m.Up(s)
		if err := s.execute(ctx); err != nil {
			return fmt.Errorf("migrations: %s: %w", nm.name, err)
		}
		if err := m.recordRan(ctx, nm.name, batch); err != nil {
			return err
		}
		m.logger.Info("migrated", "name", nm.name)
		ranCount++
	}

	if ranCount == 0 {
		m.logger.Info("nothing to migrate")
	}
	return nil
}

// Rollback rolls back the last batch of migrations.
func (m *Migrator) Rollback(ctx context.Context) error {
	if err := m.ensureTable(ctx); err != nil {
		return err
	}
	batch, err := m.lastBatch(ctx)
	if err != nil {
		return err
	}
	if batch == 0 {
		m.logger.Info("nothing to rollback")
		return nil
	}
	names, err := m.getBatch(ctx, batch)
	if err != nil {
		return err
	}

	// Rollback in reverse order
	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	for _, name := range names {
		nm, ok := m.findByName(name)
		if !ok {
			m.logger.Warn("migration not found — skipping rollback", "name", name)
			continue
		}
		m.logger.Info("rolling back", "name", name)
		s := newSchema(m.db, m.driver)
		nm.m.Down(s)
		if err := s.execute(ctx); err != nil {
			return fmt.Errorf("migrations: rollback %s: %w", name, err)
		}
		if err := m.deleteRan(ctx, name); err != nil {
			return err
		}
		m.logger.Info("rolled back", "name", name)
	}
	return nil
}

// Fresh drops all tables and re-runs all migrations from scratch.
func (m *Migrator) Fresh(ctx context.Context) error {
	m.logger.Warn("dropping all tables (migrate:fresh)")
	if err := m.dropAll(ctx); err != nil {
		return err
	}
	return m.Migrate(ctx)
}

// Status prints the status of all registered migrations.
func (m *Migrator) Status(ctx context.Context) ([]MigrationStatus, error) {
	if err := m.ensureTable(ctx); err != nil {
		return nil, err
	}
	ran, err := m.getRanWithBatch(ctx)
	if err != nil {
		return nil, err
	}
	m.sortMigrations()

	var result []MigrationStatus
	for _, nm := range m.migrations {
		info := ran[nm.name]
		result = append(result, MigrationStatus{
			Name:    nm.name,
			Ran:     info.ran,
			Batch:   info.batch,
			RanAt:   info.ranAt,
		})
	}
	return result, nil
}

// MigrationStatus represents one migration's current state.
type MigrationStatus struct {
	Name  string
	Ran   bool
	Batch int
	RanAt time.Time
}

// ─────────────────────────── internal helpers ──────────────────────

func (m *Migrator) ensureTable(ctx context.Context) error {
	var q string
	switch m.driver {
	case "postgres":
		q = fmt.Sprintf(`CREATE TABLE IF NOT EXISTS "%s" (
			id BIGSERIAL PRIMARY KEY,
			migration VARCHAR(255) NOT NULL,
			batch INTEGER NOT NULL,
			ran_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`, m.tableName)
	default:
		q = fmt.Sprintf("CREATE TABLE IF NOT EXISTS `%s` (id BIGINT AUTO_INCREMENT PRIMARY KEY, migration VARCHAR(255) NOT NULL, batch INT NOT NULL, ran_at DATETIME NOT NULL DEFAULT NOW()) ENGINE=InnoDB", m.tableName)
	}
	_, err := m.db.ExecContext(ctx, q)
	return err
}

type ranInfo struct {
	ran   bool
	batch int
	ranAt time.Time
}

func (m *Migrator) getRan(ctx context.Context) (map[string]bool, error) {
	info, err := m.getRanWithBatch(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[string]bool, len(info))
	for k, v := range info {
		out[k] = v.ran
	}
	return out, nil
}

func (m *Migrator) getRanWithBatch(ctx context.Context) (map[string]ranInfo, error) {
	q := fmt.Sprintf("SELECT migration, batch, ran_at FROM %s", m.quotedTable())
	rows, err := m.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[string]ranInfo)
	for rows.Next() {
		var name string
		var batch int
		var ranAt time.Time
		if err := rows.Scan(&name, &batch, &ranAt); err != nil {
			return nil, err
		}
		result[name] = ranInfo{ran: true, batch: batch, ranAt: ranAt}
	}
	return result, rows.Err()
}

func (m *Migrator) getBatch(ctx context.Context, batch int) ([]string, error) {
	q := fmt.Sprintf("SELECT migration FROM %s WHERE batch = %d ORDER BY migration ASC", m.quotedTable(), batch)
	rows, err := m.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

func (m *Migrator) nextBatch(ctx context.Context) int {
	b, _ := m.lastBatch(ctx)
	return b + 1
}

func (m *Migrator) lastBatch(ctx context.Context) (int, error) {
	q := fmt.Sprintf("SELECT COALESCE(MAX(batch),0) FROM %s", m.quotedTable())
	row := m.db.QueryRowContext(ctx, q)
	var b int
	return b, row.Scan(&b)
}

func (m *Migrator) recordRan(ctx context.Context, name string, batch int) error {
	q := fmt.Sprintf("INSERT INTO %s (migration, batch, ran_at) VALUES (?, ?, ?)", m.quotedTable())
	if m.driver == "postgres" {
		q = fmt.Sprintf(`INSERT INTO %s (migration, batch, ran_at) VALUES ($1, $2, $3)`, m.quotedTable())
	}
	_, err := m.db.ExecContext(ctx, q, name, batch, time.Now())
	return err
}

func (m *Migrator) deleteRan(ctx context.Context, name string) error {
	q := fmt.Sprintf("DELETE FROM %s WHERE migration = ?", m.quotedTable())
	if m.driver == "postgres" {
		q = fmt.Sprintf("DELETE FROM %s WHERE migration = $1", m.quotedTable())
	}
	_, err := m.db.ExecContext(ctx, q, name)
	return err
}

func (m *Migrator) dropAll(ctx context.Context) error {
	var q string
	if m.driver == "postgres" {
		q = "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"
	} else {
		// For MySQL: get all tables and drop them
		rows, err := m.db.QueryContext(ctx, "SHOW TABLES")
		if err != nil {
			return err
		}
		var tables []string
		for rows.Next() {
			var t string
			_ = rows.Scan(&t)
			tables = append(tables, t)
		}
		rows.Close()
		if len(tables) == 0 {
			return nil
		}
		_, err = m.db.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS=0")
		if err != nil {
			return err
		}
		for _, t := range tables {
			if _, err := m.db.ExecContext(ctx, "DROP TABLE IF EXISTS `"+t+"`"); err != nil {
				return err
			}
		}
		_, err = m.db.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS=1")
		return err
	}
	_, err := m.db.ExecContext(ctx, q)
	return err
}

func (m *Migrator) quotedTable() string {
	if m.driver == "postgres" {
		return `"` + m.tableName + `"`
	}
	return "`" + m.tableName + "`"
}

func (m *Migrator) sortMigrations() {
	sort.Slice(m.migrations, func(i, j int) bool {
		return m.migrations[i].name < m.migrations[j].name
	})
}

func (m *Migrator) findByName(name string) (namedMigration, bool) {
	for _, nm := range m.migrations {
		if nm.name == name {
			return nm, true
		}
	}
	return namedMigration{}, false
}
