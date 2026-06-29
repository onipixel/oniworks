package migrations

import (
	"context"
	"database/sql"
	"fmt"
)

// Schema is the migration schema builder. It collects DDL statements and executes them
// atomically when execute() is called.
type Schema struct {
	db     *sql.DB
	driver string
	stmts  []string
}

func newSchema(db *sql.DB, driver string) *Schema {
	return &Schema{db: db, driver: driver}
}

// Create creates a new table using the fluent Table builder.
//
//	s.Create("users", func(t *Table) {
//	    t.ID()
//	    t.String("email", 255).Unique()
//	    t.Timestamps()
//	})
func (s *Schema) Create(table string, fn func(*Table)) {
	t := newTable(table, s.driver)
	fn(t)
	s.stmts = append(s.stmts, t.createSQL())
	s.stmts = append(s.stmts, t.indexSQL()...)
}

// Table modifies an existing table.
func (s *Schema) Table(table string, fn func(*TableModifier)) {
	tm := &TableModifier{table: table, driver: s.driver}
	fn(tm)
	s.stmts = append(s.stmts, tm.stmts...)
}

// Drop drops a table (no IF EXISTS — use DropIfExists for safety).
func (s *Schema) Drop(table string) {
	s.stmts = append(s.stmts, s.dropSQL(table, false))
}

// DropIfExists drops a table only if it exists.
func (s *Schema) DropIfExists(table string) {
	s.stmts = append(s.stmts, s.dropSQL(table, true))
}

// Raw adds a raw SQL statement to the execution queue.
func (s *Schema) Raw(sql string) {
	s.stmts = append(s.stmts, sql)
}

// RenameTable renames a table.
func (s *Schema) RenameTable(from, to string) {
	if s.driver == "postgres" {
		s.stmts = append(s.stmts, fmt.Sprintf(`ALTER TABLE "%s" RENAME TO "%s"`, from, to))
	} else {
		s.stmts = append(s.stmts, fmt.Sprintf("RENAME TABLE `%s` TO `%s`", from, to))
	}
}

// HasTable reports whether a table exists (runs immediately, not queued).
func (s *Schema) HasTable(ctx context.Context, table string) (bool, error) {
	var q string
	if s.driver == "postgres" {
		q = "SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_name = $1)"
	} else {
		q = "SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_name = ? AND table_schema = DATABASE())"
	}
	var exists bool
	row := s.db.QueryRowContext(ctx, q, table)
	return exists, row.Scan(&exists)
}

// Statements returns the SQL statements this schema would execute. The Migrator
// uses it to run a whole batch inside one transaction instead of committing
// each migration independently.
func (s *Schema) Statements() []string { return s.stmts }

func (s *Schema) execute(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	for _, stmt := range s.stmts {
		if stmt == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migrations: %w\nSQL: %s", err, stmt)
		}
	}
	return tx.Commit()
}

func (s *Schema) dropSQL(table string, ifExists bool) string {
	if s.driver == "postgres" {
		if ifExists {
			return fmt.Sprintf(`DROP TABLE IF EXISTS "%s" CASCADE`, table)
		}
		return fmt.Sprintf(`DROP TABLE "%s" CASCADE`, table)
	}
	if ifExists {
		return fmt.Sprintf("DROP TABLE IF EXISTS `%s`", table)
	}
	return fmt.Sprintf("DROP TABLE `%s`", table)
}

// TableModifier adds/drops columns and indexes on an existing table.
type TableModifier struct {
	table  string
	driver string
	stmts  []string
}

func (tm *TableModifier) AddColumn(col *Column) {
	tm.stmts = append(tm.stmts, tm.addColSQL(col))
}

func (tm *TableModifier) DropColumn(name string) {
	if tm.driver == "postgres" {
		tm.stmts = append(tm.stmts, fmt.Sprintf(`ALTER TABLE "%s" DROP COLUMN IF EXISTS "%s"`, tm.table, name))
	} else {
		tm.stmts = append(tm.stmts, fmt.Sprintf("ALTER TABLE `%s` DROP COLUMN `%s`", tm.table, name))
	}
}

func (tm *TableModifier) AddIndex(cols ...string) {
	tm.stmts = append(tm.stmts, tm.indexSQL(cols, false))
}

func (tm *TableModifier) AddUniqueIndex(cols ...string) {
	tm.stmts = append(tm.stmts, tm.indexSQL(cols, true))
}

func (tm *TableModifier) addColSQL(col *Column) string {
	t := newTable(tm.table, tm.driver)
	t.cols = append(t.cols, col)
	stmts := t.createSQL() // this gives CREATE TABLE which is wrong; we need ADD COLUMN
	_ = stmts
	if tm.driver == "postgres" {
		return fmt.Sprintf(`ALTER TABLE "%s" ADD COLUMN %s`, tm.table, col.toPostgresSQL())
	}
	return fmt.Sprintf("ALTER TABLE `%s` ADD COLUMN %s", tm.table, col.toMySQLSQL())
}

func (tm *TableModifier) indexSQL(cols []string, unique bool) string {
	name := fmt.Sprintf("idx_%s_%s", tm.table, joinCols(cols, "_"))
	u := ""
	if unique {
		u = "UNIQUE "
	}
	if tm.driver == "postgres" {
		qc := make([]string, len(cols))
		for i, c := range cols {
			qc[i] = `"` + c + `"`
		}
		return fmt.Sprintf(`CREATE %sINDEX IF NOT EXISTS "%s" ON "%s" (%s)`, u, name, tm.table, joinCols(qc, ", "))
	}
	qc := make([]string, len(cols))
	for i, c := range cols {
		qc[i] = "`" + c + "`"
	}
	return fmt.Sprintf("CREATE %sINDEX `%s` ON `%s` (%s)", u, name, tm.table, joinCols(qc, ", "))
}

func joinCols(cols []string, sep string) string {
	result := ""
	for i, c := range cols {
		if i > 0 {
			result += sep
		}
		result += c
	}
	return result
}
