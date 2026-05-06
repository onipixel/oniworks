// Package database provides a high-performance query builder, struct scanner,
// lifecycle hooks, and explicit eager relationship loading for OniWorks.
//
// Design principles:
//   - Query builder is lazy — nothing executes until a terminal method is called
//   - Reflection cache: struct field mapping is computed once per type, never per-request
//   - No lazy loading: relationships are never populated automatically (no hidden queries)
//   - Batch eager loading: db.Load(&users, "Posts") fires WHERE user_id IN (...) — no N+1
//   - Context-everywhere: all queries accept context.Context
package database

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"sync/atomic"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
)

// Driver identifies the database backend.
type Driver string

const (
	DriverPostgres Driver = "postgres"
	DriverMySQL    Driver = "mysql"
)

// Config holds all database connection parameters.
type Config struct {
	Driver   Driver
	Host     string
	Port     int
	Name     string
	User     string
	Password string
	SSLMode  string

	// Pool settings
	MaxOpen     int
	MaxIdle     int
	MaxLifetime time.Duration

	// PostgreSQL-specific: use pgxpool directly for best performance
	PgxConfig *pgxpool.Config
}

// DefaultConfig returns a localhost PostgreSQL config suitable for development.
func DefaultConfig() Config {
	return Config{
		Driver:      DriverPostgres,
		Host:        "127.0.0.1",
		Port:        5432,
		User:        "postgres",
		SSLMode:     "disable",
		MaxOpen:     25,
		MaxIdle:     5,
		MaxLifetime: 5 * time.Minute,
	}
}

// DB is the central database handle. It wraps *sql.DB and provides the
// query builder API. Create one DB per database connection and share it.
type DB struct {
	sqlDB    *sql.DB
	driver   Driver
	grammar  Grammar
	logger   *slog.Logger
	logLevel slog.Level

	// txDepth is non-zero inside a Transaction call (supports nested savepoints)
	txDepth int
	tx      *sql.Tx
}

// Grammar and ColumnDef are defined in grammar.go in this package.

// Open connects to the database using cfg and returns a *DB.
func Open(cfg Config) (*DB, error) {
	var (
		sqlDB   *sql.DB
		grammar Grammar
		err     error
	)

	switch cfg.Driver {
	case DriverPostgres:
		dsn := buildPostgresDSN(cfg)
		// Use pgxpool for native Postgres performance, then wrap for database/sql compat
		if cfg.PgxConfig != nil {
			pool, e := pgxpool.New(context.Background(), cfg.PgxConfig.ConnString())
			if e != nil {
				return nil, fmt.Errorf("database: pgxpool: %w", e)
			}
			sqlDB = stdlib.OpenDBFromPool(pool)
		} else {
			sqlDB, err = sql.Open("pgx", dsn)
		}
		grammar = &postgresGrammar{}

	case DriverMySQL:
		dsn := buildMySQLDSN(cfg)
		sqlDB, err = sql.Open("mysql", dsn)
		grammar = &mysqlGrammar{}

	default:
		return nil, fmt.Errorf("database: unsupported driver %q", cfg.Driver)
	}

	if err != nil {
		return nil, fmt.Errorf("database: open: %w", err)
	}

	sqlDB.SetMaxOpenConns(cfg.MaxOpen)
	sqlDB.SetMaxIdleConns(cfg.MaxIdle)
	sqlDB.SetConnMaxLifetime(cfg.MaxLifetime)

	db := &DB{
		sqlDB:    sqlDB,
		driver:   cfg.Driver,
		grammar:  grammar,
		logger:   slog.Default(),
		logLevel: slog.LevelDebug,
	}
	return db, nil
}

// MustOpen is like Open but panics on error.
func MustOpen(cfg Config) *DB {
	db, err := Open(cfg)
	if err != nil {
		panic(err)
	}
	return db
}

// Ping verifies the connection is alive.
func (db *DB) Ping(ctx context.Context) error {
	return db.sqlDB.PingContext(ctx)
}

// Close closes the underlying connection pool.
func (db *DB) Close() error { return db.sqlDB.Close() }

// Driver returns the database driver type.
func (db *DB) Driver() Driver { return db.driver }

// WithLogger sets a custom logger.
func (db *DB) WithLogger(l *slog.Logger) *DB {
	db.logger = l
	return db
}

// SetLogLevel sets the minimum log level for query logging (default: slog.LevelDebug).
func (db *DB) SetLogLevel(level slog.Level) { db.logLevel = level }

// SQLDB exposes the underlying *sql.DB for advanced use (migrations, raw exec, etc.).
func (db *DB) SQLDB() *sql.DB { return db.sqlDB }

// Table returns a new Builder for the given table name.
//
//	db.Table("users").Where("active = ?", true).All(&users)
func (db *DB) Table(table string) *Builder {
	b := builderPool.Get().(*Builder)
	b.reset()
	b.db = db
	b.table = table
	b.ctx = context.Background()
	return b
}

// Raw returns a Builder for a raw SQL query.
//
//	db.Raw("SELECT COUNT(*) FROM users WHERE active = ?", true).Scan(&count)
func (db *DB) Raw(sql string, args ...any) *Builder {
	b := builderPool.Get().(*Builder)
	b.reset()
	b.db = db
	b.rawSQL = sql
	b.rawArgs = args
	b.ctx = context.Background()
	return b
}

// Transaction executes fn inside a database transaction. If fn returns nil,
// the transaction is committed; otherwise it is rolled back.
// Nested calls create savepoints so inner failures can be rolled back independently.
func (db *DB) Transaction(fn func(tx *DB) error) error {
	return db.TransactionContext(context.Background(), fn)
}

// TransactionContext is like Transaction but accepts a context.
func (db *DB) TransactionContext(ctx context.Context, fn func(tx *DB) error) error {
	if db.tx != nil {
		// Already inside a transaction — use savepoint for nesting
		savept := fmt.Sprintf("sp_%d", db.txDepth)
		if _, err := db.tx.ExecContext(ctx, "SAVEPOINT "+savept); err != nil {
			return fmt.Errorf("database: savepoint: %w", err)
		}
		txDB := &DB{sqlDB: db.sqlDB, tx: db.tx, driver: db.driver, grammar: db.grammar, txDepth: db.txDepth + 1, logger: db.logger}
		if err := fn(txDB); err != nil {
			_, _ = db.tx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT "+savept)
			return err
		}
		_, err := db.tx.ExecContext(ctx, "RELEASE SAVEPOINT "+savept)
		return err
	}

	tx, err := db.sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("database: begin transaction: %w", err)
	}
	txDB := &DB{sqlDB: db.sqlDB, tx: tx, driver: db.driver, grammar: db.grammar, txDepth: 1, logger: db.logger}

	var fnErr error
	func() {
		defer func() {
			if r := recover(); r != nil {
				fnErr = fmt.Errorf("database: transaction panic: %v", r)
			}
		}()
		fnErr = fn(txDB)
	}()
	if fnErr != nil {
		_ = tx.Rollback()
		return fnErr
	}
	return tx.Commit()
}

// execContext runs a raw SQL statement, using the active transaction if present.
func (db *DB) execContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	db.log(ctx, query, args)
	if db.tx != nil {
		return db.tx.ExecContext(ctx, query, args...)
	}
	return db.sqlDB.ExecContext(ctx, query, args...)
}

// queryContext runs a raw SQL query, using the active transaction if present.
func (db *DB) queryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	db.log(ctx, query, args)
	if db.tx != nil {
		return db.tx.QueryContext(ctx, query, args...)
	}
	return db.sqlDB.QueryContext(ctx, query, args...)
}

func (db *DB) log(ctx context.Context, query string, args []any) {
	if db.logger != nil {
		db.logger.Log(ctx, db.logLevel, "sql", "query", query, "args", args)
	}
}

// ─────────────────────────── DSN builders ─────────────────────────

func buildPostgresDSN(c Config) string {
	sslMode := c.SSLMode
	if sslMode == "" {
		sslMode = "disable"
	}
	host := c.Host
	if host == "" {
		host = os.Getenv("DB_HOST")
	}
	port := c.Port
	if port == 0 {
		port = 5432
	}
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		host, port, c.User, c.Password, c.Name, sslMode)
}

func buildMySQLDSN(c Config) string {
	host := c.Host
	if host == "" {
		host = "127.0.0.1"
	}
	port := c.Port
	if port == 0 {
		port = 3306
	}
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&multiStatements=true",
		c.User, c.Password, host, port, c.Name)
}

// ─────────────────────────── Load (eager) ─────────────────────────

// Load populates the named relationship(s) on a slice of model pointers.
// It fires exactly one batch query per relationship — never N individual queries.
//
//	db.Load(&users, "Posts")         // SELECT * FROM posts WHERE user_id IN (1,2,3)
//	db.Load(&users, "Posts", "Role") // two batch queries
func (db *DB) Load(dest any, relations ...string) error {
	return db.LoadContext(context.Background(), dest, relations...)
}

// LoadContext is like Load but with context.
func (db *DB) LoadContext(ctx context.Context, dest any, relations ...string) error {
	for _, rel := range relations {
		if err := loadRelation(ctx, db, dest, rel); err != nil {
			return fmt.Errorf("database: Load %q: %w", rel, err)
		}
	}
	return nil
}

// ─────────────────────────── global pool ──────────────────────────

// globalDB is the application-level default DB instance (optional convenience).
var globalDB atomic.Pointer[DB]

// SetDefault sets the package-level default DB used by top-level Table() / Raw() calls.
func SetDefault(db *DB) { globalDB.Store(db) }

// Default returns the package-level default DB, or panics if not set.
func Default() *DB {
	db := globalDB.Load()
	if db == nil {
		panic("database: default DB not set — call database.SetDefault(db) during application boot")
	}
	return db
}

// Table is a package-level shorthand: database.Table("users").Where(...)
func Table(table string) *Builder { return Default().Table(table) }

// Raw is a package-level shorthand: database.Raw("SELECT ...", args...)
func Raw(query string, args ...any) *Builder { return Default().Raw(query, args...) }
