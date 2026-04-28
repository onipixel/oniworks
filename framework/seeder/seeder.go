// Package seeder provides the database seeder system for OniWorks.
package seeder

import (
	"context"
	"fmt"
	"log/slog"
)

// Seeder is the interface all seeder structs must implement.
type Seeder interface {
	// Run populates the database with seed data.
	Run(ctx context.Context, db DB) error
}

// DB is a minimal interface the seeder uses to execute queries.
// The real *database.DB satisfies this.
type DB interface {
	// Table returns a query builder for the given table.
	Table(name string) interface {
		Insert(dest any) error
		Where(clause string, args ...any) interface {
			Delete() error
		}
	}
}

// Runner manages and executes seeders.
type Runner struct {
	seeders []namedSeeder
	logger  *slog.Logger
}

type namedSeeder struct {
	name string
	s    Seeder
}

// New creates a Seeder Runner.
func New() *Runner {
	return &Runner{logger: slog.Default()}
}

// Register adds a seeder to the runner.
func (r *Runner) Register(name string, s Seeder) *Runner {
	r.seeders = append(r.seeders, namedSeeder{name: name, s: s})
	return r
}

// Run runs all registered seeders in registration order.
func (r *Runner) Run(ctx context.Context, db DB) error {
	for _, ns := range r.seeders {
		r.logger.Info("seeding", "name", ns.name)
		if err := ns.s.Run(ctx, db); err != nil {
			return fmt.Errorf("seeder: %s: %w", ns.name, err)
		}
		r.logger.Info("seeded", "name", ns.name)
	}
	return nil
}

// RunOne runs a single seeder by name.
func (r *Runner) RunOne(ctx context.Context, db DB, name string) error {
	for _, ns := range r.seeders {
		if ns.name == name {
			r.logger.Info("seeding", "name", ns.name)
			if err := ns.s.Run(ctx, db); err != nil {
				return fmt.Errorf("seeder: %s: %w", name, err)
			}
			r.logger.Info("seeded", "name", ns.name)
			return nil
		}
	}
	return fmt.Errorf("seeder: %q not registered", name)
}
