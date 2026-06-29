package database

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"
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
	wheres []whereClause

	// SELECT
	selects []string

	// ORDER / LIMIT / OFFSET
	orderBys []string
	limit    int
	offset   int

	// JOIN
	joins []string

	// GROUP / HAVING
	groupBys   []string
	having     string
	havingArgs []any

	// WITH (eager load at query time)
	withs []string

	// Raw query mode
	rawSQL  string
	rawArgs []any

	// Soft delete support
	withTrashed bool
	softDelete  bool

	// err holds the first error produced by a chained builder method (e.g. an
	// unsafe identifier passed to Select/OrderBy/GroupBy). Terminal methods
	// return it before touching the database, so invalid input fails closed.
	err error
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
	b.softDelete = false
	b.err = nil
}

// setErr records the first error encountered while building the query.
func (b *Builder) setErr(format string, args ...any) {
	if b.err == nil {
		b.err = fmt.Errorf(format, args...)
	}
}

// release returns the builder to the pool. Call defer b.release() in terminal methods.
func (b *Builder) release() { builderPool.Put(b) }

// ─────────────────────────── Builder API ──────────────────────────

// Ctx sets the context for this query.
func (b *Builder) Ctx(ctx context.Context) *Builder { b.ctx = ctx; return b }

// Select specifies which columns to retrieve. Each argument must be a column
// reference — "col", "table.col", "col AS alias", "*", or "table.*" — and is
// validated and quoted to prevent SQL injection. For aggregate or computed
// expressions (e.g. "COUNT(*) AS n"), use SelectRaw instead.
//
//	db.Table("users").Select("id", "email").All(&users)
func (b *Builder) Select(cols ...string) *Builder {
	for _, c := range cols {
		q, ok := b.quoteSelectExpr(c)
		if !ok {
			b.setErr("database: Select: invalid or unsafe column expression %q (use SelectRaw for raw SQL)", c)
			continue
		}
		b.selects = append(b.selects, q)
	}
	return b
}

// SelectRaw adds a raw, unescaped SELECT expression. The caller is responsible
// for ensuring the expression is safe — never pass user input directly.
//
//	db.Table("posts").SelectRaw("COUNT(*) AS post_count")
func (b *Builder) SelectRaw(expr string) *Builder { b.selects = append(b.selects, expr); return b }

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

// WhereRaw adds a raw WHERE clause without any quoting or escaping.
// Use this for complex conditions or when you need full control over the SQL.
//
//	db.Table("users").WhereRaw("1=1").Delete() // delete all rows
func (b *Builder) WhereRaw(clause string, args ...any) *Builder {
	return b.Where(clause, args...)
}

// OrderBy adds an ORDER BY clause. The clause is a comma-separated list of
// "[table.]column [ASC|DESC] [NULLS FIRST|NULLS LAST]" terms; column
// identifiers are validated and quoted to prevent SQL injection, and the
// direction/NULLS keywords are checked against an allow-list. Anything outside
// that grammar (function calls, arithmetic, etc.) is rejected — use OrderByRaw
// for trusted raw expressions.
//
//	db.Table("users").OrderBy("created_at DESC").OrderBy("name ASC")
//	db.Table("messages").OrderBy("last_message_at DESC NULLS LAST")
func (b *Builder) OrderBy(clause string) *Builder {
	q, ok := b.sanitizeOrderBy(clause)
	if !ok {
		b.setErr("database: OrderBy: invalid or unsafe order expression %q (use OrderByRaw for raw SQL)", clause)
		return b
	}
	b.orderBys = append(b.orderBys, q)
	return b
}

// OrderByRaw adds a raw, unescaped ORDER BY expression. The caller is
// responsible for safety — never pass user input directly.
func (b *Builder) OrderByRaw(clause string) *Builder { b.orderBys = append(b.orderBys, clause); return b }

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

// GroupBy adds a GROUP BY clause. Each argument must be a "[table.]column"
// reference and is validated and quoted to prevent SQL injection. Use
// GroupByRaw for trusted raw expressions.
func (b *Builder) GroupBy(cols ...string) *Builder {
	for _, c := range cols {
		q, ok := b.quoteColumnRef(c)
		if !ok {
			b.setErr("database: GroupBy: invalid or unsafe column %q (use GroupByRaw for raw SQL)", c)
			continue
		}
		b.groupBys = append(b.groupBys, q)
	}
	return b
}

// GroupByRaw adds a raw, unescaped GROUP BY expression. The caller is
// responsible for safety — never pass user input directly.
func (b *Builder) GroupByRaw(expr string) *Builder { b.groupBys = append(b.groupBys, expr); return b }

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

// SoftDelete tells the builder this table uses soft deletes (deleted_at column).
// Automatically adds "deleted_at IS NULL" to WHERE unless WithTrashed() is called.
func (b *Builder) SoftDelete() *Builder { b.softDelete = true; return b }

// ─────────────────────────── Terminal methods ──────────────────────

// First executes the query and scans a single row into dest.
// Returns ErrNotFound if no row matches.
func (b *Builder) First(dest any) error {
	defer b.release()
	if b.err != nil {
		return b.err
	}
	b.limit = 1
	rawQ, rawA := b.buildSelect()
	query, args := b.normalizePlaceholders(rawQ, rawA...)
	rows, err := b.db.queryContext(b.ctx, query, args...)
	if err != nil {
		return err
	}
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return err
		}
		_ = rows.Close()
		return ErrNotFound
	}
	if err := scanRow(rows, dest); err != nil {
		_ = rows.Close()
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
	if b.err != nil {
		return b.err
	}
	rawQ, rawA := b.buildSelect()
	query, args := b.normalizePlaceholders(rawQ, rawA...)
	rows, err := b.db.queryContext(b.ctx, query, args...)
	if err != nil {
		return err
	}
	if err := scanRows(rows, dest); err != nil {
		_ = rows.Close()
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
	if b.err != nil {
		return 0, b.err
	}
	rawQ, rawA := b.buildCount()
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

// Page holds paginated query results.
type Page[T any] struct {
	Items       []T   `json:"items"`
	Total       int64 `json:"total"`
	PerPage     int   `json:"per_page"`
	CurrentPage int   `json:"current_page"`
	LastPage    int   `json:"last_page"`
	From        int64 `json:"from"`
	To          int64 `json:"to"`
}

// Paginate executes a COUNT and a SELECT with LIMIT/OFFSET and returns a Page.
// page is 1-based.
func (b *Builder) Paginate(page, perPage int, dest any) (*Page[any], error) {
	if b.err != nil {
		err := b.err
		b.release()
		return nil, err
	}
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 15
	}

	// Clone builder state for the count query. The COUNT must use the SAME
	// filters as the data query — including soft-delete and GROUP BY/HAVING —
	// or Total will not match the rows actually returned.
	countB := builderPool.Get().(*Builder)
	countB.reset()
	countB.db = b.db
	countB.ctx = b.ctx
	countB.table = b.table
	countB.wheres = append(countB.wheres, b.wheres...)
	countB.joins = append(countB.joins, b.joins...)
	countB.groupBys = append(countB.groupBys, b.groupBys...)
	countB.having = b.having
	countB.havingArgs = append(countB.havingArgs, b.havingArgs...)
	countB.withTrashed = b.withTrashed
	countB.softDelete = b.softDelete

	total, err := countB.Count()
	if err != nil {
		b.release()
		return nil, err
	}

	b.limit = perPage
	b.offset = (page - 1) * perPage

	if err := b.All(dest); err != nil {
		return nil, err
	}

	lastPage := int(total) / perPage
	if int(total)%perPage != 0 {
		lastPage++
	}
	if lastPage < 1 {
		lastPage = 1
	}

	from := int64((page-1)*perPage + 1)
	to := int64(page * perPage)
	if to > total {
		to = total
	}
	if total == 0 {
		from = 0
		to = 0
	}

	return &Page[any]{
		Items:       sliceToAny(dest),
		Total:       total,
		PerPage:     perPage,
		CurrentPage: page,
		LastPage:    lastPage,
		From:        from,
		To:          to,
	}, nil
}

// sliceToAny copies the elements of a *[]T (or []T) destination into a []any so
// the paginated rows are available on Page.Items as well as in dest.
func sliceToAny(dest any) []any {
	rv := reflect.ValueOf(dest)
	for rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Slice {
		return nil
	}
	out := make([]any, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		out[i] = rv.Index(i).Interface()
	}
	return out
}

// Pluck retrieves a single column as a []T slice.
//
//	var emails []string
//	db.Table("users").Pluck("email", &emails)
func (b *Builder) Pluck(col string, dest any) error {
	defer b.release()
	if b.err != nil {
		return b.err
	}
	q, ok := b.quoteColumnRef(col)
	if !ok {
		return fmt.Errorf("database: Pluck: invalid or unsafe column %q", col)
	}
	b.selects = []string{q}
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
// dest must be a struct pointer; for map[string]any use InsertMap instead.
func (b *Builder) Insert(dest any) error {
	defer b.release()
	if _, ok := dest.(map[string]any); ok {
		return fmt.Errorf("database: Insert: maps are not supported; use InsertMap instead")
	}
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

// InsertMap inserts a row from a plain map[string]any. Column order is sorted
// for determinism. Hooks are not called. "created_at" and "updated_at" are
// injected as UTC RFC3339 strings when not already present in the map.
func (b *Builder) InsertMap(data map[string]any) error {
	defer b.release()
	now := time.Now().UTC().Format(time.RFC3339)
	if _, ok := data["created_at"]; !ok {
		data["created_at"] = now
	}
	if _, ok := data["updated_at"]; !ok {
		data["updated_at"] = now
	}
	cols := make([]string, 0, len(data))
	for k := range data {
		cols = append(cols, k)
	}
	sort.Strings(cols)
	vals := make([]any, len(cols))
	for i, k := range cols {
		vals[i] = data[k]
	}
	query, args := b.buildInsert(cols, vals)
	query, args = b.normalizePlaceholders(query, args...)
	_, err := b.db.execContext(b.ctx, query, args...)
	return err
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
	if b.err != nil {
		return b.err
	}
	if len(b.wheres) == 0 && b.rawSQL == "" {
		return fmt.Errorf("database: Update called without WHERE clause — use WhereRaw(\"1=1\") to update all rows")
	}
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
	if b.err != nil {
		return b.err
	}
	if len(b.wheres) == 0 && b.rawSQL == "" {
		return fmt.Errorf("database: Delete called without WHERE clause — use WhereRaw(\"1=1\") to delete all rows")
	}
	query, args := b.buildDeleteStmt()
	query, args = b.normalizePlaceholders(query, args...)
	_, err := b.db.execContext(b.ctx, query, args...)
	return err
}

// buildDeleteStmt returns the statement Delete will execute. On a soft-delete
// builder it is an UPDATE stamping deleted_at; otherwise a real DELETE. Use
// ForceDelete to bypass soft delete.
func (b *Builder) buildDeleteStmt() (string, []any) {
	if b.softDelete {
		now := time.Now().UTC().Format(time.RFC3339)
		return b.buildUpdate([]string{"deleted_at"}, []any{now})
	}
	return b.buildDelete()
}

// ForceDelete permanently removes rows even on a soft-delete builder.
//
//	db.Table("posts").SoftDelete().Where("id = ?", id).ForceDelete()
func (b *Builder) ForceDelete() error {
	b.softDelete = false
	return b.Delete()
}

// Exec executes a raw write query (INSERT, UPDATE, DELETE) that returns no rows.
// Intended for use with Raw() for statements like INSERT ... ON CONFLICT DO NOTHING.
func (b *Builder) Exec() error {
	defer b.release()
	query, args := b.rawSQL, b.rawArgs
	if query == "" {
		query, args = b.buildDelete()
	}
	query, args = b.normalizePlaceholders(query, args...)
	_, err := b.db.execContext(b.ctx, query, args...)
	return err
}

// Scan executes a raw query and scans a single scalar value.
func (b *Builder) Scan(dest any) error {
	defer b.release()
	if b.err != nil {
		return b.err
	}
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

// buildCount builds the COUNT query for the current builder state. A grouped
// query is counted by wrapping it in a subquery (counting the number of
// groups); ordering and paging are stripped from the inner query since they
// don't affect the count.
func (b *Builder) buildCount() (string, []any) {
	if len(b.groupBys) > 0 {
		b.selects = []string{"1"}
		b.orderBys = b.orderBys[:0]
		b.limit, b.offset = 0, 0
		inner, args := b.buildSelect()
		return "SELECT COUNT(*) FROM (" + inner + ") AS _oni_count", args
	}
	b.selects = []string{"COUNT(*)"}
	return b.buildSelect()
}

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
	sdClause := ""
	if b.softDelete && !b.withTrashed {
		sdClause = b.db.grammar.QuoteIdent("deleted_at") + " IS NULL"
	}
	if where != "" || sdClause != "" {
		sb.WriteString(" WHERE ")
		if sdClause != "" && where != "" {
			sb.WriteString(sdClause + " AND (" + where + ")")
		} else if sdClause != "" {
			sb.WriteString(sdClause)
		} else {
			sb.WriteString(where)
		}
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

// ─────────────────────── Identifier sanitization ───────────────────
//
// Select/OrderBy/GroupBy accept caller-supplied column references that are
// interpolated directly into SQL (parameter placeholders cannot stand in for
// identifiers). To keep the "injection-safe" guarantee, these helpers validate
// every identifier against a strict grammar and quote it via the dialect's
// QuoteIdent. Anything that does not match is rejected so the builder fails
// closed rather than emitting attacker-controlled SQL.

// isSafeIdentSegment reports whether s is a single bare identifier segment:
// a letter or underscore followed by letters, digits, or underscores.
func isSafeIdentSegment(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		switch {
		case r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z'):
		case i > 0 && r >= '0' && r <= '9':
		default:
			return false
		}
	}
	return true
}

// quoteColumnRef validates and quotes a column reference: "col", "table.col",
// "*", or "table.*". Returns ("", false) if the reference is unsafe.
func (b *Builder) quoteColumnRef(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if s == "*" {
		return "*", true
	}
	parts := strings.Split(s, ".")
	if len(parts) > 2 {
		return "", false
	}
	out := make([]string, len(parts))
	for i, p := range parts {
		p = strings.TrimSpace(p)
		if p == "*" && i == len(parts)-1 {
			out[i] = "*"
			continue
		}
		if !isSafeIdentSegment(p) {
			return "", false
		}
		out[i] = b.db.grammar.QuoteIdent(p)
	}
	return strings.Join(out, "."), true
}

// quoteSelectExpr validates and quotes a SELECT column expression, allowing an
// optional alias: "col", "table.col", "col alias", or "col AS alias".
func (b *Builder) quoteSelectExpr(s string) (string, bool) {
	toks := strings.Fields(strings.TrimSpace(s))
	switch len(toks) {
	case 1:
		return b.quoteColumnRef(toks[0])
	case 2: // "col alias"
		col, ok := b.quoteColumnRef(toks[0])
		if !ok || !isSafeIdentSegment(toks[1]) {
			return "", false
		}
		return col + " AS " + b.db.grammar.QuoteIdent(toks[1]), true
	case 3: // "col AS alias"
		if !strings.EqualFold(toks[1], "AS") {
			return "", false
		}
		col, ok := b.quoteColumnRef(toks[0])
		if !ok || !isSafeIdentSegment(toks[2]) {
			return "", false
		}
		return col + " AS " + b.db.grammar.QuoteIdent(toks[2]), true
	}
	return "", false
}

// sanitizeOrderBy validates and quotes a comma-separated ORDER BY clause of
// "[table.]column [ASC|DESC] [NULLS FIRST|NULLS LAST]" terms.
func (b *Builder) sanitizeOrderBy(clause string) (string, bool) {
	parts := strings.Split(clause, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		toks := strings.Fields(part)
		if len(toks) == 0 {
			return "", false
		}
		col, ok := b.quoteColumnRef(toks[0])
		if !ok {
			return "", false
		}
		seg := col
		idx := 1
		if idx < len(toks) { // optional direction
			d := strings.ToUpper(toks[idx])
			if d != "ASC" && d != "DESC" {
				return "", false
			}
			seg += " " + d
			idx++
		}
		if idx < len(toks) { // optional NULLS FIRST|LAST
			if idx+1 >= len(toks) || !strings.EqualFold(toks[idx], "NULLS") {
				return "", false
			}
			nl := strings.ToUpper(toks[idx+1])
			if nl != "FIRST" && nl != "LAST" {
				return "", false
			}
			seg += " NULLS " + nl
			idx += 2
		}
		if idx != len(toks) { // leftover tokens → unsafe
			return "", false
		}
		out = append(out, seg)
	}
	return strings.Join(out, ", "), true
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
