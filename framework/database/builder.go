package database

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// Map is a convenience alias used in Insert/Update calls.
type Map = map[string]any

// builderPool recycles Builder allocations to reduce GC pressure.
var builderPool = sync.Pool{
	New: func() any { return &Builder{} },
}

// Builder is a lazy, fluent SQL query builder.
// Nothing is executed until a terminal method (First, All, Count, Insert, Update, etc.) is called.
type Builder struct {
	db    *DB
	ctx   context.Context
	table string

	// WHERE
	wheres    []whereClause
	orWheres  []whereClause

	// SELECT
	selects []string

	// ORDER / LIMIT / OFFSET
	orderBys []string
	limit    int
	offset   int

	// JOIN
	joins []string

	// GROUP / HAVING
	groupBys []string
	having   string
	havingArgs []any

	// WITH (eager load at query time)
	withs []string

	// Raw query mode
	rawSQL  string
	rawArgs []any

	// Soft delete support
	withTrashed bool
}

type whereClause struct {
	clause string
	args   []any
	or     bool
}

// reset returns a Builder to its zero state before pool re-use.
func (b *Builder) reset() {
	b.db = nil
	b.ctx = nil
	b.table = ""
	b.wheres = b.wheres[:0]
	b.orWheres = b.orWheres[:0]
	b.selects = b.selects[:0]
	b.orderBys = b.orderBys[:0]
	b.limit = 0
	b.offset = 0
	b.joins = b.joins[:0]
	b.groupBys = b.groupBys[:0]
	b.having = ""
	b.havingArgs = b.havingArgs[:0]
	b.withs = b.withs[:0]
	b.rawSQL = ""
	b.rawArgs = nil
	b.withTrashed = false
}

// release returns the builder to the pool. Call defer b.release() in terminal methods.
func (b *Builder) release() { builderPool.Put(b) }

// ─────────────────────────── Builder API ──────────────────────────

// Ctx sets the context for this query.
func (b *Builder) Ctx(ctx context.Context) *Builder { b.ctx = ctx; return b }

// Select specifies which columns to retrieve.
//
//	db.Table("users").Select("id", "email").All(&users)
func (b *Builder) Select(cols ...string) *Builder { b.selects = append(b.selects, cols...); return b }

// Where adds a WHERE condition (AND-joined).
//
//	db.Table("users").Where("active = ?", true).Where("role = ?", "admin")
func (b *Builder) Where(clause string, args ...any) *Builder {
	b.wheres = append(b.wheres, whereClause{clause: clause, args: args})
	return b
}

// OrWhere adds an OR WHERE condition.
func (b *Builder) OrWhere(clause string, args ...any) *Builder {
	b.wheres = append(b.wheres, whereClause{clause: clause, args: args, or: true})
	return b
}

// WhereIn adds a WHERE column IN (...) clause.
func (b *Builder) WhereIn(col string, values ...any) *Builder {
	if len(values) == 0 {
		return b.Where("1=0") // no results if empty
	}
	placeholders := make([]string, len(values))
	for i := range values {
		placeholders[i] = "?"
	}
	return b.Where(fmt.Sprintf("%s IN (%s)", b.db.grammar.QuoteIdent(col), strings.Join(placeholders, ",")), values...)
}

// WhereNotIn adds a WHERE column NOT IN (...) clause.
func (b *Builder) WhereNotIn(col string, values ...any) *Builder {
	if len(values) == 0 {
		return b
	}
	placeholders := make([]string, len(values))
	for i := range values {
		placeholders[i] = "?"
	}
	return b.Where(fmt.Sprintf("%s NOT IN (%s)", b.db.grammar.QuoteIdent(col), strings.Join(placeholders, ",")), values...)
}

// WhereNull adds WHERE column IS NULL.
func (b *Builder) WhereNull(col string) *Builder {
	return b.Where(b.db.grammar.QuoteIdent(col) + " IS NULL")
}

// WhereNotNull adds WHERE column IS NOT NULL.
func (b *Builder) WhereNotNull(col string) *Builder {
	return b.Where(b.db.grammar.QuoteIdent(col) + " IS NOT NULL")
}

// OrderBy adds an ORDER BY clause.
//
//	db.Table("users").OrderBy("created_at DESC").OrderBy("name ASC")
func (b *Builder) OrderBy(clause string) *Builder { b.orderBys = append(b.orderBys, clause); return b }

// Limit sets the maximum number of results.
func (b *Builder) Limit(n int) *Builder { b.limit = n; return b }

// Offset sets the number of results to skip.
func (b *Builder) Offset(n int) *Builder { b.offset = n; return b }

// Join adds an INNER JOIN.
func (b *Builder) Join(clause string) *Builder { b.joins = append(b.joins, "JOIN "+clause); return b }

// LeftJoin adds a LEFT JOIN.
func (b *Builder) LeftJoin(clause string) *Builder {
	b.joins = append(b.joins, "LEFT JOIN "+clause)
	return b
}

// GroupBy adds a GROUP BY clause.
func (b *Builder) GroupBy(cols ...string) *Builder { b.groupBys = append(b.groupBys, cols...); return b }

// Having adds a HAVING clause.
func (b *Builder) Having(clause string, args ...any) *Builder {
	b.having = clause
	b.havingArgs = args
	return b
}

// With eager-loads the named relationships after the main query executes.
//
//	db.Table("users").With("Posts", "Role").All(&users)
func (b *Builder) With(relations ...string) *Builder {
	b.withs = append(b.withs, relations...)
	return b
}

// WithTrashed includes soft-deleted rows (does not add "deleted_at IS NULL").
func (b *Builder) WithTrashed() *Builder { b.withTrashed = true; return b }

// ─────────────────────────── Terminal methods ──────────────────────

// First executes the query and scans a single row into dest.
// Returns ErrNotFound if no row matches.
func (b *Builder) First(dest any) error {
	defer b.release()
	b.limit = 1
	rawQ, rawA := b.buildSelect()
	query, args := b.normalizePlaceholders(rawQ, rawA...)
	rows, err := b.db.queryContext(b.ctx, query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return err
		}
		return ErrNotFound
	}
	if err := scanRow(rows, dest); err != nil {
		return err
	}
	callHook(dest, hookAfterFind, b.db)
	if len(b.withs) > 0 {
		_ = b.db.LoadContext(b.ctx, dest, b.withs...)
	}
	return rows.Close()
}

// All executes the query and scans all rows into dest (must be a pointer to a slice).
func (b *Builder) All(dest any) error {
	defer b.release()
	rawQ, rawA := b.buildSelect()
	query, args := b.normalizePlaceholders(rawQ, rawA...)
	rows, err := b.db.queryContext(b.ctx, query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	if err := scanRows(rows, dest); err != nil {
		return err
	}
	callSliceHook(dest, hookAfterFind, b.db)
	if len(b.withs) > 0 {
		_ = b.db.LoadContext(b.ctx, dest, b.withs...)
	}
	return rows.Close()
}

// Count executes a COUNT(*) query and returns the result.
func (b *Builder) Count() (int64, error) {
	defer b.release()
	b.selects = []string{"COUNT(*)"}
	rawQ, rawA := b.buildSelect()
	query, args := b.normalizePlaceholders(rawQ, rawA...)
	rows, err := b.db.queryContext(b.ctx, query, args...)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	if !rows.Next() {
		return 0, rows.Err()
	}
	var count int64
	return count, rows.Scan(&count)
}

// Exists reports whether any row matches the query.
func (b *Builder) Exists() (bool, error) {
	count, err := b.Count()
	return count > 0, err
}

// Pluck retrieves a single column as a []T slice.
//
//	var emails []string
//	db.Table("users").Pluck("email", &emails)
func (b *Builder) Pluck(col string, dest any) error {
	defer b.release()
	b.selects = []string{col}
	rawQ, rawA := b.buildSelect()
	query, args := b.normalizePlaceholders(rawQ, rawA...)
	rows, err := b.db.queryContext(b.ctx, query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	return scanColumn(rows, dest)
}

// Insert inserts dest (a model pointer) and calls BeforeCreate / AfterCreate hooks.
func (b *Builder) Insert(dest any) error {
	defer b.release()
	if err := callHook(dest, hookBeforeCreate, b.db); err != nil {
		return err
	}
	info := getStructInfo(dest)
	cols, vals := info.insertable(dest)
	if len(cols) == 0 {
		return fmt.Errorf("database: Insert: no columns to insert")
	}
	query, args := b.buildInsert(cols, vals)
	query, args = b.normalizePlaceholders(query, args...)

	if info.hasAutoID() {
		var id int64
		var err error
		if b.db.driver == DriverPostgres {
			pkCol := info.primaryKeyCol()
			row := b.db.sqlDB.QueryRowContext(b.ctx, query+" RETURNING \""+pkCol+"\"", args...)
			err = row.Scan(&id)
		} else {
			res, e := b.db.execContext(b.ctx, query, args...)
			if e != nil {
				return e
			}
			id, err = res.LastInsertId()
		}
		if err != nil {
			return err
		}
		info.setID(dest, id)
	} else {
		if _, err := b.db.execContext(b.ctx, query, args...); err != nil {
			return err
		}
	}

	return callHook(dest, hookAfterCreate, b.db)
}

// Save updates all fields of dest, calling BeforeSave / AfterSave hooks.
func (b *Builder) Save(dest any) error {
	defer b.release()
	if err := callHook(dest, hookBeforeSave, b.db); err != nil {
		return err
	}
	info := getStructInfo(dest)
	pkCol, pkVal := info.pkValue(dest)
	if pkCol == "" {
		return fmt.Errorf("database: Save: no primary key defined on %T", dest)
	}
	cols, vals := info.updatable(dest)
	if len(cols) == 0 {
		return fmt.Errorf("database: Save: no columns to update")
	}
	b.wheres = append(b.wheres, whereClause{clause: pkCol + " = ?", args: []any{pkVal}})
	query, args := b.buildUpdate(cols, vals)
	query, args = b.normalizePlaceholders(query, args...)
	if _, err := b.db.execContext(b.ctx, query, args...); err != nil {
		return err
	}
	return callHook(dest, hookAfterSave, b.db)
}

// Update updates specific columns for rows matching the WHERE clause.
//
//	db.Table("users").Where("id = ?", 1).Update(database.Map{"name": "Alice"})
func (b *Builder) Update(data Map) error {
	defer b.release()
	cols := make([]string, 0, len(data))
	vals := make([]any, 0, len(data))
	for k, v := range data {
		cols = append(cols, k)
		vals = append(vals, v)
	}
	query, args := b.buildUpdate(cols, vals)
	query, args = b.normalizePlaceholders(query, args...)
	_, err := b.db.execContext(b.ctx, query, args...)
	return err
}

// Delete removes rows matching the WHERE clause.
//
//	db.Table("users").Where("id = ?", id).Delete()
func (b *Builder) Delete() error {
	defer b.release()
	query, args := b.buildDelete()
	query, args = b.normalizePlaceholders(query, args...)
	_, err := b.db.execContext(b.ctx, query, args...)
	return err
}

// Scan executes a raw query and scans a single scalar value.
func (b *Builder) Scan(dest any) error {
	defer b.release()
	var query string
	var args []any
	if b.rawSQL != "" {
		query, args = b.rawSQL, b.rawArgs
	} else {
		query, args = b.buildSelect()
	}
	query, args = b.normalizePlaceholders(query, args...)
	rows, err := b.db.queryContext(b.ctx, query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	if !rows.Next() {
		return rows.Err()
	}
	return rows.Scan(dest)
}

// ─────────────────────────── SQL assembly ──────────────────────────

func (b *Builder) buildSelect() (string, []any) {
	if b.rawSQL != "" {
		return b.rawSQL, b.rawArgs
	}

	var sb strings.Builder
	var args []any

	// SELECT
	sb.WriteString("SELECT ")
	if len(b.selects) > 0 {
		sb.WriteString(strings.Join(b.selects, ", "))
	} else {
		sb.WriteString("*")
	}

	// FROM
	sb.WriteString(" FROM ")
	sb.WriteString(b.db.grammar.QuoteIdent(b.table))

	// JOINs
	for _, j := range b.joins {
		sb.WriteString(" ")
		sb.WriteString(j)
	}

	// WHERE
	where, wArgs := b.buildWhere()
	if where != "" {
		sb.WriteString(" WHERE ")
		sb.WriteString(where)
		args = append(args, wArgs...)
	}

	// GROUP BY
	if len(b.groupBys) > 0 {
		sb.WriteString(" GROUP BY ")
		sb.WriteString(strings.Join(b.groupBys, ", "))
	}

	// HAVING
	if b.having != "" {
		sb.WriteString(" HAVING ")
		sb.WriteString(b.having)
		args = append(args, b.havingArgs...)
	}

	// ORDER BY
	if len(b.orderBys) > 0 {
		sb.WriteString(" ORDER BY ")
		sb.WriteString(strings.Join(b.orderBys, ", "))
	}

	// LIMIT / OFFSET
	if b.limit > 0 {
		sb.WriteString(fmt.Sprintf(" LIMIT %d", b.limit))
	}
	if b.offset > 0 {
		sb.WriteString(fmt.Sprintf(" OFFSET %d", b.offset))
	}

	return sb.String(), args
}

func (b *Builder) buildInsert(cols []string, vals []any) (string, []any) {
	quotedCols := make([]string, len(cols))
	placeholders := make([]string, len(cols))
	for i, c := range cols {
		quotedCols[i] = b.db.grammar.QuoteIdent(c)
		placeholders[i] = "?"
	}
	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		b.db.grammar.QuoteIdent(b.table),
		strings.Join(quotedCols, ", "),
		strings.Join(placeholders, ", "),
	)
	return query, vals
}

func (b *Builder) buildUpdate(cols []string, vals []any) (string, []any) {
	setClauses := make([]string, len(cols))
	for i, c := range cols {
		setClauses[i] = b.db.grammar.QuoteIdent(c) + " = ?"
	}
	where, wArgs := b.buildWhere()
	args := append(vals, wArgs...)
	query := fmt.Sprintf("UPDATE %s SET %s",
		b.db.grammar.QuoteIdent(b.table),
		strings.Join(setClauses, ", "),
	)
	if where != "" {
		query += " WHERE " + where
	}
	return query, args
}

func (b *Builder) buildDelete() (string, []any) {
	where, wArgs := b.buildWhere()
	query := "DELETE FROM " + b.db.grammar.QuoteIdent(b.table)
	if where != "" {
		query += " WHERE " + where
	}
	return query, wArgs
}

func (b *Builder) buildWhere() (string, []any) {
	if len(b.wheres) == 0 {
		return "", nil
	}
	var parts []string
	var args []any
	for i, w := range b.wheres {
		clause := "(" + w.clause + ")"
		if i > 0 {
			if w.or {
				clause = "OR " + clause
			} else {
				clause = "AND " + clause
			}
		}
		parts = append(parts, clause)
		args = append(args, w.args...)
	}
	return strings.Join(parts, " "), args
}

// normalizePlaceholders converts "?" placeholders to database-native ones
// ($1, $2, ... for Postgres; "?" stays for MySQL).
func (b *Builder) normalizePlaceholders(query string, args ...any) (string, []any) {
	if b.db.driver != DriverPostgres {
		return query, args
	}
	var sb strings.Builder
	n := 0
	for _, ch := range query {
		if ch == '?' {
			n++
			sb.WriteString(fmt.Sprintf("$%d", n))
		} else {
			sb.WriteRune(ch)
		}
	}
	return sb.String(), args
}

// ErrNotFound is returned by First when no matching row is found.
var ErrNotFound = fmt.Errorf("database: record not found")
