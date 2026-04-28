package database

import (
	"database/sql"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"
)

// structInfo holds pre-computed field metadata for a struct type.
// It is populated once per type and cached in scanCache.
type structInfo struct {
	// fields maps column name → struct field index path
	fields map[string][]int

	pkCol    string // column name of the primary key field
	pkIndex  []int  // field index path for PK
	autoID   bool   // true if PK is auto-increment

	autoTimeCols []string
	insertCols   []string
	updateCols   []string

	// relations: maps exported field name → relationDef
	relations map[string]relationDef
}

type relationDef struct {
	kind       string
	table      string
	foreignKey string
	joinTable  string
	joinKey    string
	refKey     string
	fieldIndex []int
	elemType   reflect.Type
}

var scanCache sync.Map // map[reflect.Type]*structInfo

// getStructInfo returns (or builds) the structInfo for the concrete type of v.
func getStructInfo(v any) *structInfo {
	t := reflect.TypeOf(v)
	for t.Kind() == reflect.Ptr || t.Kind() == reflect.Slice {
		t = t.Elem()
	}
	if cached, ok := scanCache.Load(t); ok {
		return cached.(*structInfo)
	}
	info := buildStructInfo(t)
	scanCache.Store(t, info)
	return info
}

func buildStructInfo(t reflect.Type) *structInfo {
	info := &structInfo{
		fields:    make(map[string][]int),
		relations: make(map[string]relationDef),
	}

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		tag := f.Tag.Get("db")
		if tag == "-" {
			continue
		}
		if isRelationTag(tag) {
			rel := parseRelationTag(tag, f, []int{i})
			info.relations[f.Name] = rel
			continue
		}
		colName, opts := parseTag(tag, f.Name)
		info.fields[colName] = []int{i}
		if containsOpt(opts, "primaryKey") {
			info.pkCol = colName
			info.pkIndex = []int{i}
			info.autoID = containsOpt(opts, "autoIncrement")
		}
		if containsOpt(opts, "autoTime") {
			info.autoTimeCols = append(info.autoTimeCols, colName)
		}
	}

	for col := range info.fields {
		if col == info.pkCol && info.autoID {
			continue
		}
		info.insertCols = append(info.insertCols, col)
	}
	for col := range info.fields {
		if col == info.pkCol {
			continue
		}
		info.updateCols = append(info.updateCols, col)
	}
	return info
}

// ─────────────────────────── struct field access ───────────────────

func (si *structInfo) hasAutoID() bool { return si.autoID }

func (si *structInfo) primaryKeyCol() string { return si.pkCol }

func (si *structInfo) setID(v any, id int64) {
	rv := reflect.ValueOf(v)
	for rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	if len(si.pkIndex) == 0 {
		return
	}
	f := rv.FieldByIndex(si.pkIndex)
	switch f.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		f.SetInt(id)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		f.SetUint(uint64(id))
	}
}

// pkValue returns the primary key column name and value from v.
func (si *structInfo) pkValue(v any) (col string, val any) {
	if si.pkCol == "" {
		return "", nil
	}
	rv := reflect.ValueOf(v)
	for rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	return si.pkCol, rv.FieldByIndex(si.pkIndex).Interface()
}

func (si *structInfo) insertable(v any) (cols []string, vals []any) {
	rv := reflect.ValueOf(v)
	for rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	for col, idx := range si.fields {
		if col == si.pkCol && si.autoID {
			continue
		}
		f := rv.FieldByIndex(idx)
		if (col == "created_at" || col == "updated_at") && f.Type() == reflect.TypeOf(time.Time{}) {
			f.Set(reflect.ValueOf(time.Now()))
		}
		cols = append(cols, col)
		vals = append(vals, f.Interface())
	}
	return
}

func (si *structInfo) updatable(v any) (cols []string, vals []any) {
	rv := reflect.ValueOf(v)
	for rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	for col, idx := range si.fields {
		if col == si.pkCol {
			continue
		}
		f := rv.FieldByIndex(idx)
		if col == "updated_at" && f.Type() == reflect.TypeOf(time.Time{}) {
			f.Set(reflect.ValueOf(time.Now()))
		}
		cols = append(cols, col)
		vals = append(vals, f.Interface())
	}
	return
}

// ─────────────────────────── row scanning ─────────────────────────

func scanRow(rows *sql.Rows, dest any) error {
	cols, err := rows.Columns()
	if err != nil {
		return err
	}
	switch d := dest.(type) {
	case *map[string]any:
		return scanRowToMap(rows, cols, d)
	default:
		return scanRowToStruct(rows, cols, d)
	}
}

func scanRowToMap(rows *sql.Rows, cols []string, dest *map[string]any) error {
	ptrs := make([]any, len(cols))
	vals := make([]any, len(cols))
	for i := range ptrs {
		ptrs[i] = &vals[i]
	}
	if err := rows.Scan(ptrs...); err != nil {
		return err
	}
	m := make(map[string]any, len(cols))
	for i, col := range cols {
		m[col] = vals[i]
	}
	*dest = m
	return nil
}

func scanRowToStruct(rows *sql.Rows, cols []string, dest any) error {
	rv := reflect.ValueOf(dest)
	for rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return rows.Scan(dest)
	}
	info := getStructInfo(dest)
	ptrs := make([]any, len(cols))
	for i, col := range cols {
		idx, ok := info.fields[col]
		if !ok {
			var discard any
			ptrs[i] = &discard
			continue
		}
		ptrs[i] = rv.FieldByIndex(idx).Addr().Interface()
	}
	return rows.Scan(ptrs...)
}

func scanRows(rows *sql.Rows, dest any) error {
	cols, err := rows.Columns()
	if err != nil {
		return err
	}
	rv := reflect.ValueOf(dest)
	if rv.Kind() != reflect.Ptr {
		return fmt.Errorf("database: scanRows dest must be a pointer, got %T", dest)
	}
	rv = rv.Elem()
	if rv.Kind() != reflect.Slice {
		return fmt.Errorf("database: scanRows dest must be *[]T, got *%s", rv.Type())
	}
	elemType := rv.Type().Elem()
	isPtr := elemType.Kind() == reflect.Ptr
	if isPtr {
		elemType = elemType.Elem()
	}
	isMap := elemType == reflect.TypeOf(map[string]any{})

	for rows.Next() {
		elem := reflect.New(elemType)
		var scanErr error
		if isMap {
			m := make(map[string]any)
			scanErr = scanRowToMap(rows, cols, &m)
			if isPtr {
				rv.Set(reflect.Append(rv, reflect.ValueOf(&m)))
			} else {
				rv.Set(reflect.Append(rv, reflect.ValueOf(m)))
			}
			if scanErr != nil {
				return scanErr
			}
			continue
		}
		scanErr = scanRowToStruct(rows, cols, elem.Interface())
		if scanErr != nil {
			return scanErr
		}
		if isPtr {
			rv.Set(reflect.Append(rv, elem))
		} else {
			rv.Set(reflect.Append(rv, elem.Elem()))
		}
	}
	return rows.Err()
}

func scanColumn(rows *sql.Rows, dest any) error {
	rv := reflect.ValueOf(dest)
	if rv.Kind() != reflect.Ptr || rv.Elem().Kind() != reflect.Slice {
		return fmt.Errorf("database: Pluck dest must be *[]T")
	}
	slice := rv.Elem()
	elemType := slice.Type().Elem()
	for rows.Next() {
		elem := reflect.New(elemType)
		if err := rows.Scan(elem.Interface()); err != nil {
			return err
		}
		slice.Set(reflect.Append(slice, elem.Elem()))
	}
	return rows.Err()
}

// ─────────────────────────── tag parsing ──────────────────────────

func parseTag(tag, fieldName string) (colName string, opts []string) {
	if tag == "" {
		return toSnake(fieldName), nil
	}
	parts := strings.Split(tag, ",")
	colName = parts[0]
	if colName == "" {
		colName = toSnake(fieldName)
	}
	return colName, parts[1:]
}

func containsOpt(opts []string, opt string) bool {
	for _, o := range opts {
		if o == opt {
			return true
		}
	}
	return false
}

func isRelationTag(tag string) bool {
	return strings.HasPrefix(tag, "has_many:") ||
		strings.HasPrefix(tag, "belongs_to:") ||
		strings.HasPrefix(tag, "many_to_many:")
}

func parseRelationTag(tag string, f reflect.StructField, idx []int) relationDef {
	parts := strings.Split(tag, ",")
	kindTable := strings.SplitN(parts[0], ":", 2)
	kind := kindTable[0]
	table := ""
	if len(kindTable) > 1 {
		table = kindTable[1]
	}
	rel := relationDef{kind: kind, table: table, fieldIndex: idx}
	for _, part := range parts[1:] {
		kv := strings.SplitN(part, ":", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "foreign_key":
			rel.foreignKey = kv[1]
		case "join_table":
			rel.joinTable = kv[1]
		case "join_key":
			rel.joinKey = kv[1]
		case "ref_key":
			rel.refKey = kv[1]
		}
	}
	ft := f.Type
	if ft.Kind() == reflect.Ptr {
		ft = ft.Elem()
	}
	if ft.Kind() == reflect.Slice {
		rel.elemType = ft.Elem()
		if rel.elemType.Kind() == reflect.Ptr {
			rel.elemType = rel.elemType.Elem()
		}
	} else {
		rel.elemType = ft
	}
	return rel
}

func toSnake(s string) string {
	var out strings.Builder
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				out.WriteByte('_')
			}
			out.WriteRune(r + 32)
		} else {
			out.WriteRune(r)
		}
	}
	return out.String()
}
