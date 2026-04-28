package migrations

import (
	"fmt"
	"strings"
)

// Table is the fluent builder for CREATE TABLE statements.
type Table struct {
	name    string
	driver  string
	cols    []*Column
	indexes []tableIndex
}

type tableIndex struct {
	cols   []string
	unique bool
}

func newTable(name, driver string) *Table {
	return &Table{name: name, driver: driver}
}

// ─────────────────────────── Column shortcuts ──────────────────────

// ID adds a "id" BIGINT primary key (auto-increment for MySQL, BIGSERIAL for Postgres).
func (t *Table) ID() *Column { return t.col("id", "bigint").PK().AutoInc() }

// UUID adds a UUID primary key column named "id".
func (t *Table) UUID() *Column { return t.col("id", "uuid").PK() }

// String adds a VARCHAR column.
func (t *Table) String(name string, length ...int) *Column {
	l := 255
	if len(length) > 0 {
		l = length[0]
	}
	return t.col(name, "string").Len(l)
}

// Text adds a TEXT column.
func (t *Table) Text(name string) *Column { return t.col(name, "text") }

// Integer adds an INT column.
func (t *Table) Integer(name string) *Column { return t.col(name, "int") }

// BigInteger adds a BIGINT column.
func (t *Table) BigInteger(name string) *Column { return t.col(name, "bigint") }

// Boolean adds a BOOL/TINYINT(1) column.
func (t *Table) Boolean(name string) *Column { return t.col(name, "bool") }

// Float adds a DOUBLE/FLOAT column.
func (t *Table) Float(name string) *Column { return t.col(name, "float") }

// Decimal adds a DECIMAL(10,2) column.
func (t *Table) Decimal(name string) *Column { return t.col(name, "decimal") }

// JSON adds a JSON/JSONB column.
func (t *Table) JSON(name string) *Column { return t.col(name, "json") }

// Timestamp adds a TIMESTAMP/TIMESTAMPTZ column.
func (t *Table) Timestamp(name string) *Column { return t.col(name, "time") }

// Date adds a DATE column.
func (t *Table) Date(name string) *Column { return t.col(name, "date") }

// Binary adds a BYTEA/BLOB column.
func (t *Table) Binary(name string) *Column { return t.col(name, "binary") }

// Timestamps adds "created_at" and "updated_at" columns (NOT NULL, set automatically).
func (t *Table) Timestamps() {
	t.col("created_at", "time").NotNullable()
	t.col("updated_at", "time").NotNullable()
}

// SoftDeletes adds a nullable "deleted_at" column for soft-delete support.
func (t *Table) SoftDeletes() {
	t.col("deleted_at", "time").Nullable()
}

// ForeignKey adds a BIGINT column with an optional FK constraint.
//
//	t.ForeignKey("user_id", "users", "id").OnDelete("CASCADE")
func (t *Table) ForeignKey(name, refTable, refCol string) *Column {
	col := t.col(name, "bigint")
	col.fkTable = refTable
	col.fkCol = refCol
	return col
}

// Index creates a composite index on the given columns.
func (t *Table) Index(cols ...string) {
	t.indexes = append(t.indexes, tableIndex{cols: cols, unique: false})
}

// UniqueIndex creates a unique composite index.
func (t *Table) UniqueIndex(cols ...string) {
	t.indexes = append(t.indexes, tableIndex{cols: cols, unique: true})
}

// ─────────────────────────── SQL generation ────────────────────────

func (t *Table) createSQL() string {
	lines := make([]string, 0, len(t.cols)+2)
	var pks []string

	for _, c := range t.cols {
		var line string
		if t.driver == "postgres" {
			line = "  " + c.toPostgresSQL()
		} else {
			line = "  " + c.toMySQLSQL()
		}
		lines = append(lines, line)
		if c.pk {
			if t.driver == "postgres" {
				pks = append(pks, `"`+c.name+`"`)
			} else {
				pks = append(pks, "`"+c.name+"`")
			}
		}
	}

	if len(pks) > 0 {
		lines = append(lines, fmt.Sprintf("  PRIMARY KEY (%s)", strings.Join(pks, ", ")))
	}

	if t.driver == "postgres" {
		return fmt.Sprintf("CREATE TABLE IF NOT EXISTS \"%s\" (\n%s\n)", t.name, strings.Join(lines, ",\n"))
	}
	return fmt.Sprintf("CREATE TABLE IF NOT EXISTS `%s` (\n%s\n) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci", t.name, strings.Join(lines, ",\n"))
}

func (t *Table) indexSQL() []string {
	stmts := make([]string, 0, len(t.indexes))
	for _, idx := range t.indexes {
		name := fmt.Sprintf("idx_%s_%s", t.name, strings.Join(idx.cols, "_"))
		u := ""
		if idx.unique {
			u = "UNIQUE "
		}
		if t.driver == "postgres" {
			qc := make([]string, len(idx.cols))
			for i, c := range idx.cols {
				qc[i] = `"` + c + `"`
			}
			stmts = append(stmts, fmt.Sprintf(`CREATE %sINDEX IF NOT EXISTS "%s" ON "%s" (%s)`, u, name, t.name, strings.Join(qc, ", ")))
		} else {
			qc := make([]string, len(idx.cols))
			for i, c := range idx.cols {
				qc[i] = "`" + c + "`"
			}
			stmts = append(stmts, fmt.Sprintf("CREATE %sINDEX `%s` ON `%s` (%s)", u, name, t.name, strings.Join(qc, ", ")))
		}
	}
	return stmts
}

func (t *Table) col(name, kind string) *Column {
	c := &Column{name: name, kind: kind}
	t.cols = append(t.cols, c)
	return c
}

// Column represents one table column in a migration.
type Column struct {
	name       string
	kind       string
	pk         bool
	autoIncr   bool
	nullable   bool
	unique     bool
	def        string
	length     int
	fkTable    string
	fkCol      string
	fkOnDelete string
}

// PK marks this column as the primary key.
func (c *Column) PK() *Column { c.pk = true; return c }

// AutoInc marks this column as auto-increment.
func (c *Column) AutoInc() *Column { c.autoIncr = true; return c }

// Nullable allows NULL values.
func (c *Column) Nullable() *Column { c.nullable = true; return c }

// NotNullable disallows NULL values (default).
func (c *Column) NotNullable() *Column { c.nullable = false; return c }

// Unique adds a unique constraint.
func (c *Column) Unique() *Column { c.unique = true; return c }

// Default sets a default value.
func (c *Column) Default(v any) *Column {
	switch val := v.(type) {
	case string:
		c.def = "'" + val + "'"
	case bool:
		if val {
			c.def = "TRUE"
		} else {
			c.def = "FALSE"
		}
	default:
		c.def = fmt.Sprintf("%v", val)
	}
	return c
}

// Len sets the column length (for VARCHAR).
func (c *Column) Len(n int) *Column { c.length = n; return c }

// OnDelete sets the FK ON DELETE action ("CASCADE", "SET NULL", "RESTRICT").
func (c *Column) OnDelete(action string) *Column { c.fkOnDelete = action; return c }

func (c *Column) toPostgresSQL() string {
	var sb strings.Builder
	sb.WriteString(`"` + c.name + `" `)
	if c.autoIncr {
		sb.WriteString("BIGSERIAL")
	} else {
		sb.WriteString(pgColType(c))
	}
	if !c.nullable {
		sb.WriteString(" NOT NULL")
	}
	if c.unique {
		sb.WriteString(" UNIQUE")
	}
	if c.def != "" {
		sb.WriteString(" DEFAULT " + c.def)
	}
	if c.fkTable != "" {
		sb.WriteString(fmt.Sprintf(` REFERENCES "%s"("%s")`, c.fkTable, c.fkCol))
		if c.fkOnDelete != "" {
			sb.WriteString(" ON DELETE " + c.fkOnDelete)
		}
	}
	return sb.String()
}

func (c *Column) toMySQLSQL() string {
	var sb strings.Builder
	sb.WriteString("`" + c.name + "` ")
	sb.WriteString(myColType(c))
	if c.autoIncr {
		sb.WriteString(" AUTO_INCREMENT")
	}
	if !c.nullable {
		sb.WriteString(" NOT NULL")
	}
	if c.unique {
		sb.WriteString(" UNIQUE")
	}
	if c.def != "" {
		sb.WriteString(" DEFAULT " + c.def)
	}
	return sb.String()
}

func pgColType(c *Column) string {
	l := c.length
	switch c.kind {
	case "string":
		if l <= 0 {
			l = 255
		}
		return fmt.Sprintf("VARCHAR(%d)", l)
	case "text":
		return "TEXT"
	case "int":
		return "INTEGER"
	case "bigint":
		return "BIGINT"
	case "bool":
		return "BOOLEAN"
	case "float":
		return "DOUBLE PRECISION"
	case "decimal":
		return "DECIMAL(10,2)"
	case "json", "jsonb":
		return "JSONB"
	case "time", "timestamp":
		return "TIMESTAMPTZ"
	case "date":
		return "DATE"
	case "uuid":
		return "UUID"
	case "binary", "blob":
		return "BYTEA"
	default:
		return strings.ToUpper(c.kind)
	}
}

func myColType(c *Column) string {
	l := c.length
	switch c.kind {
	case "string":
		if l <= 0 {
			l = 255
		}
		return fmt.Sprintf("VARCHAR(%d)", l)
	case "text":
		return "TEXT"
	case "int":
		return "INT"
	case "bigint":
		return "BIGINT"
	case "bool":
		return "TINYINT(1)"
	case "float":
		return "DOUBLE"
	case "decimal":
		return "DECIMAL(10,2)"
	case "json", "jsonb":
		return "JSON"
	case "time", "timestamp":
		return "DATETIME"
	case "date":
		return "DATE"
	case "uuid":
		return "VARCHAR(36)"
	case "binary", "blob":
		return "BLOB"
	default:
		return strings.ToUpper(c.kind)
	}
}
