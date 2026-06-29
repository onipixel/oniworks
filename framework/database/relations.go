package database

import (
	"context"
	"fmt"
	"reflect"
	"strings"
)

// loadRelation loads one named relationship on dest (pointer to struct or slice of structs).
// It fires exactly one batch SQL query using WHERE foreign_key IN (...).
func loadRelation(ctx context.Context, db *DB, dest any, relationName string) error {
	rv := reflect.ValueOf(dest)
	for rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}

	// Determine the parent elem type
	var elemType reflect.Type
	isSlice := rv.Kind() == reflect.Slice
	if isSlice {
		elemType = rv.Type().Elem()
		if elemType.Kind() == reflect.Ptr {
			elemType = elemType.Elem()
		}
	} else {
		elemType = rv.Type()
	}

	info := getStructInfo(reflect.New(elemType).Interface())
	rel, ok := info.relations[relationName]
	if !ok {
		return fmt.Errorf("database: relation %q not found on %s", relationName, elemType.Name())
	}

	switch rel.kind {
	case "has_many":
		return loadHasMany(ctx, db, rv, isSlice, rel, info)
	case "belongs_to":
		return loadBelongsTo(ctx, db, rv, isSlice, rel, info)
	case "many_to_many":
		return loadManyToMany(ctx, db, rv, isSlice, rel, info)
	default:
		return fmt.Errorf("database: unknown relation kind %q", rel.kind)
	}
}

// loadHasMany: parent has many related rows (e.g. User has many Posts).
// Query: SELECT * FROM posts WHERE user_id IN (1,2,3,...)
func loadHasMany(ctx context.Context, db *DB, rv reflect.Value, isSlice bool, rel relationDef, info *structInfo) error {
	// Collect parent PKs
	parentPKs := collectPKs(rv, isSlice, info)
	if len(parentPKs) == 0 {
		return nil
	}

	placeholders, args := inPlaceholders(parentPKs)
	query := fmt.Sprintf("SELECT * FROM %s WHERE %s IN (%s)",
		db.grammar.QuoteIdent(rel.table),
		db.grammar.QuoteIdent(rel.foreignKey),
		placeholders,
	)
	query = normalizePH(db, query, len(args))

	rows, err := db.queryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	// Build a map from FK value → []relatedRow
	related, err := buildFKMap(rows, rel.foreignKey, rel.elemType)
	if err != nil {
		return err
	}

	// Assign related rows back to parent structs
	assignHasMany(rv, isSlice, info, rel, related)
	return nil
}

// loadBelongsTo: parent owns a FK pointing to related (e.g. Post belongs to User).
// Query: SELECT * FROM roles WHERE id IN (1,2,3,...) [one query, not N]
func loadBelongsTo(ctx context.Context, db *DB, rv reflect.Value, isSlice bool, rel relationDef, info *structInfo) error {
	fkIdx, ok := info.fields[rel.foreignKey]
	if !ok {
		return fmt.Errorf("database: BelongsTo: foreign key %q not found on parent", rel.foreignKey)
	}

	// Collect FK values
	var fkVals []any
	fkSet := make(map[any]bool)
	iter := sliceOrOne(rv, isSlice)
	for _, elem := range iter {
		fkVal := elem.FieldByIndex(fkIdx).Interface()
		if !fkSet[fkVal] {
			fkSet[fkVal] = true
			fkVals = append(fkVals, fkVal)
		}
	}
	if len(fkVals) == 0 {
		return nil
	}

	relInfo := getStructInfo(reflect.New(rel.elemType).Interface())
	pkCol := relInfo.pkCol

	placeholders, args := inPlaceholders(fkVals)
	query := fmt.Sprintf("SELECT * FROM %s WHERE %s IN (%s)",
		db.grammar.QuoteIdent(rel.table),
		db.grammar.QuoteIdent(pkCol),
		placeholders,
	)
	query = normalizePH(db, query, len(args))

	rows, err := db.queryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	related, err := buildPKMap(rows, pkCol, rel.elemType)
	if err != nil {
		return err
	}

	// Assign back
	assignBelongsTo(rv, isSlice, info, rel, fkIdx, related)
	return nil
}

// loadManyToMany: via join table (e.g. Post many-to-many Tags).
func loadManyToMany(ctx context.Context, db *DB, rv reflect.Value, isSlice bool, rel relationDef, info *structInfo) error {
	parentPKs := collectPKs(rv, isSlice, info)
	if len(parentPKs) == 0 {
		return nil
	}

	placeholders, args := inPlaceholders(parentPKs)
	query := fmt.Sprintf(
		"SELECT jt.%s, r.* FROM %s jt JOIN %s r ON jt.%s = r.id WHERE jt.%s IN (%s)",
		db.grammar.QuoteIdent(rel.joinKey),
		db.grammar.QuoteIdent(rel.joinTable),
		db.grammar.QuoteIdent(rel.table),
		db.grammar.QuoteIdent(rel.refKey),
		db.grammar.QuoteIdent(rel.joinKey),
		placeholders,
	)
	query = normalizePH(db, query, len(args))

	rows, err := db.queryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	related, err := buildFKMap(rows, rel.joinKey, rel.elemType)
	if err != nil {
		return err
	}

	assignHasMany(rv, isSlice, info, rel, related)
	return nil
}

// ─────────────────────────── helpers ──────────────────────────────

func collectPKs(rv reflect.Value, isSlice bool, info *structInfo) []any {
	var pks []any
	pkSet := make(map[any]bool)
	for _, elem := range sliceOrOne(rv, isSlice) {
		pk := elem.FieldByIndex(info.pkIndex).Interface()
		if !pkSet[pk] {
			pkSet[pk] = true
			pks = append(pks, pk)
		}
	}
	return pks
}

func sliceOrOne(rv reflect.Value, isSlice bool) []reflect.Value {
	if !isSlice {
		return []reflect.Value{rv}
	}
	elems := make([]reflect.Value, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		e := rv.Index(i)
		if e.Kind() == reflect.Ptr {
			e = e.Elem()
		}
		elems[i] = e
	}
	return elems
}

func inPlaceholders(vals []any) (string, []any) {
	ph := make([]string, len(vals))
	for i := range vals {
		ph[i] = "?"
	}
	return strings.Join(ph, ", "), vals
}

func normalizePH(db *DB, query string, _ int) string {
	if db.driver != DriverPostgres {
		return query
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
	return sb.String()
}

func buildFKMap(rows interface{ Next() bool; Columns() ([]string, error); Scan(...any) error; Err() error; Close() error }, fkCol string, elemType reflect.Type) (map[any][]reflect.Value, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	result := make(map[any][]reflect.Value)
	for rows.Next() {
		elem := reflect.New(elemType).Elem()
		info := getStructInfo(reflect.New(elemType).Interface())
		ptrs := make([]any, len(cols))
		for i, col := range cols {
			idx, ok := info.fields[col]
			if !ok {
				var discard any
				ptrs[i] = &discard
				continue
			}
			ptrs[i] = elem.FieldByIndex(idx).Addr().Interface()
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		// Get FK value
		fkIdx, ok := info.fields[fkCol]
		var fkVal any
		if ok {
			fkVal = elem.FieldByIndex(fkIdx).Interface()
		}
		result[fkVal] = append(result[fkVal], elem)
	}
	return result, rows.Err()
}

func buildPKMap(rows interface{ Next() bool; Columns() ([]string, error); Scan(...any) error; Err() error; Close() error }, pkCol string, elemType reflect.Type) (map[any]reflect.Value, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	result := make(map[any]reflect.Value)
	info := getStructInfo(reflect.New(elemType).Interface())
	for rows.Next() {
		elem := reflect.New(elemType).Elem()
		ptrs := make([]any, len(cols))
		for i, col := range cols {
			idx, ok := info.fields[col]
			if !ok {
				var discard any
				ptrs[i] = &discard
				continue
			}
			ptrs[i] = elem.FieldByIndex(idx).Addr().Interface()
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		pkIdx := info.fields[pkCol]
		pkVal := elem.FieldByIndex(pkIdx).Interface()
		result[pkVal] = elem
	}
	return result, rows.Err()
}

func assignHasMany(rv reflect.Value, isSlice bool, info *structInfo, rel relationDef, related map[any][]reflect.Value) {
	for _, elem := range sliceOrOne(rv, isSlice) {
		pk := elem.FieldByIndex(info.pkIndex).Interface()
		rows, ok := related[pk]
		if !ok {
			continue
		}
		// Use the relation's own field index — not a (table,kind) lookup, which
		// collides when a model has two relations to the same table.
		relField := elem.FieldByIndex(rel.fieldIndex)
		if !relField.IsValid() || !relField.CanSet() {
			continue
		}
		sliceVal := reflect.MakeSlice(relField.Type(), 0, len(rows))
		for _, r := range rows {
			sliceVal = reflect.Append(sliceVal, r)
		}
		relField.Set(sliceVal)
	}
}

func assignBelongsTo(rv reflect.Value, isSlice bool, info *structInfo, rel relationDef, fkIdx []int, related map[any]reflect.Value) {
	for _, elem := range sliceOrOne(rv, isSlice) {
		fkVal := elem.FieldByIndex(fkIdx).Interface()
		relRow, ok := related[fkVal]
		if !ok {
			continue
		}
		relField := elem.FieldByIndex(rel.fieldIndex)
		if !relField.IsValid() || !relField.CanSet() {
			continue
		}
		if relField.Kind() == reflect.Ptr {
			ptr := reflect.New(relRow.Type())
			ptr.Elem().Set(relRow)
			relField.Set(ptr)
		} else {
			relField.Set(relRow)
		}
	}
}

