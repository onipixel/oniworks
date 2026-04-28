package database

import (
	"fmt"
	"strings"
)

// ColumnDef is a portable column definition shared between the Postgres and MySQL grammars.
type ColumnDef struct {
	Name       string
	Type       string // "string","text","int","bigint","bool","float","decimal","json","time","uuid","binary"
	PrimaryKey bool
	AutoIncr   bool
	Nullable   bool
	Unique     bool
	Default    string
	Length     int
	FKTable    string
	FKCol      string
	FKOnDelete string
}

// Grammar generates database-dialect-specific SQL fragments and DDL statements.
type Grammar interface {
	Placeholder(n int) string
	QuoteIdent(s string) string
	DropTableSQL(table string) string
	CreateTableSQL(table string, cols []ColumnDef) string
	AddColumnSQL(table string, col ColumnDef) string
	DropColumnSQL(table, col string) string
	AddIndexSQL(table string, cols []string, unique bool) string
}

// ─────────────────────────── PostgreSQL ───────────────────────────

type postgresGrammar struct{}

func (postgresGrammar) Placeholder(n int) string {
	if n <= 0 {
		return "?"
	}
	return fmt.Sprintf("$%d", n)
}

func (postgresGrammar) QuoteIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

func (postgresGrammar) DropTableSQL(table string) string {
	return fmt.Sprintf(`DROP TABLE IF EXISTS "%s" CASCADE`, table)
}

func (postgresGrammar) CreateTableSQL(table string, cols []ColumnDef) string {
	lines := make([]string, 0, len(cols)+1)
	var pks []string
	for _, c := range cols {
		lines = append(lines, "  "+pgColumnDef(c))
		if c.PrimaryKey {
			pks = append(pks, `"`+c.Name+`"`)
		}
	}
	if len(pks) > 0 {
		lines = append(lines, fmt.Sprintf("  PRIMARY KEY (%s)", strings.Join(pks, ", ")))
	}
	return fmt.Sprintf("CREATE TABLE IF NOT EXISTS \"%s\" (\n%s\n)", table, strings.Join(lines, ",\n"))
}

func (postgresGrammar) AddColumnSQL(table string, col ColumnDef) string {
	return fmt.Sprintf(`ALTER TABLE "%s" ADD COLUMN %s`, table, pgColumnDef(col))
}

func (postgresGrammar) DropColumnSQL(table, col string) string {
	return fmt.Sprintf(`ALTER TABLE "%s" DROP COLUMN IF EXISTS "%s"`, table, col)
}

func (postgresGrammar) AddIndexSQL(table string, cols []string, unique bool) string {
	qc := make([]string, len(cols))
	for i, c := range cols {
		qc[i] = `"` + c + `"`
	}
	name := fmt.Sprintf("idx_%s_%s", table, strings.Join(cols, "_"))
	u := ""
	if unique {
		u = "UNIQUE "
	}
	return fmt.Sprintf(`CREATE %sINDEX IF NOT EXISTS "%s" ON "%s" (%s)`, u, name, table, strings.Join(qc, ", "))
}

func pgColumnDef(c ColumnDef) string {
	var sb strings.Builder
	sb.WriteString(`"` + c.Name + `" `)
	if c.AutoIncr {
		sb.WriteString("BIGSERIAL")
	} else {
		sb.WriteString(pgType(c))
	}
	if !c.Nullable {
		sb.WriteString(" NOT NULL")
	}
	if c.Unique {
		sb.WriteString(" UNIQUE")
	}
	if c.Default != "" {
		sb.WriteString(" DEFAULT " + c.Default)
	}
	if c.FKTable != "" {
		sb.WriteString(fmt.Sprintf(` REFERENCES "%s"("%s")`, c.FKTable, c.FKCol))
		if c.FKOnDelete != "" {
			sb.WriteString(" ON DELETE " + c.FKOnDelete)
		}
	}
	return sb.String()
}

func pgType(c ColumnDef) string {
	l := c.Length
	switch c.Type {
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
		return strings.ToUpper(c.Type)
	}
}

// ─────────────────────────── MySQL/MariaDB ────────────────────────

type mysqlGrammar struct{}

func (mysqlGrammar) Placeholder(_ int) string { return "?" }

func (mysqlGrammar) QuoteIdent(s string) string {
	return "`" + strings.ReplaceAll(s, "`", "``") + "`"
}

func (mysqlGrammar) DropTableSQL(table string) string {
	return fmt.Sprintf("DROP TABLE IF EXISTS `%s`", table)
}

func (mysqlGrammar) CreateTableSQL(table string, cols []ColumnDef) string {
	lines := make([]string, 0, len(cols)+1)
	var pks []string
	for _, c := range cols {
		lines = append(lines, "  "+myColumnDef(c))
		if c.PrimaryKey {
			pks = append(pks, "`"+c.Name+"`")
		}
	}
	if len(pks) > 0 {
		lines = append(lines, fmt.Sprintf("  PRIMARY KEY (%s)", strings.Join(pks, ", ")))
	}
	return fmt.Sprintf("CREATE TABLE IF NOT EXISTS `%s` (\n%s\n) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci",
		table, strings.Join(lines, ",\n"))
}

func (mysqlGrammar) AddColumnSQL(table string, col ColumnDef) string {
	return fmt.Sprintf("ALTER TABLE `%s` ADD COLUMN %s", table, myColumnDef(col))
}

func (mysqlGrammar) DropColumnSQL(table, col string) string {
	return fmt.Sprintf("ALTER TABLE `%s` DROP COLUMN `%s`", table, col)
}

func (mysqlGrammar) AddIndexSQL(table string, cols []string, unique bool) string {
	qc := make([]string, len(cols))
	for i, c := range cols {
		qc[i] = "`" + c + "`"
	}
	name := fmt.Sprintf("idx_%s_%s", table, strings.Join(cols, "_"))
	u := ""
	if unique {
		u = "UNIQUE "
	}
	return fmt.Sprintf("CREATE %sINDEX `%s` ON `%s` (%s)", u, name, table, strings.Join(qc, ", "))
}

func myColumnDef(c ColumnDef) string {
	var sb strings.Builder
	sb.WriteString("`" + c.Name + "` ")
	sb.WriteString(myType(c))
	if c.AutoIncr {
		sb.WriteString(" AUTO_INCREMENT")
	}
	if !c.Nullable {
		sb.WriteString(" NOT NULL")
	}
	if c.Unique {
		sb.WriteString(" UNIQUE")
	}
	if c.Default != "" {
		sb.WriteString(" DEFAULT " + c.Default)
	}
	return sb.String()
}

func myType(c ColumnDef) string {
	l := c.Length
	switch c.Type {
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
		return strings.ToUpper(c.Type)
	}
}
