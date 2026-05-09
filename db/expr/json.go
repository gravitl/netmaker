// Package expr builds dialect-aware SQL expressions for JSON map columns.
// Results are clause.Expr values that pass directly into GORM calls.
//
// Dialect is read once per call from DATABASE env var.
// Falls back to SQLite if unset or unrecognised.
//
//	DATABASE=sqlite   → json_extract / json_set / json_remove / json_patch
//	DATABASE=postgres → ->> / jsonb_set / #- / ||
package expr

import (
	"fmt"
	"os"
	"strings"

	"gorm.io/gorm/clause"
)

// Dialect is the underlying database engine.
type Dialect string

const (
	DialectSQLite   Dialect = "sqlite"
	DialectPostgres Dialect = "postgres"
)

// CurrentDialect reads the DATABASE env var to determine the active dialect.
func CurrentDialect() Dialect {
	switch strings.ToLower(os.Getenv("DATABASE")) {
	case "postgres", "postgresql", "pg":
		return DialectPostgres
	default:
		return DialectSQLite
	}
}

// Op is a SQL comparison operator.
type Op string

const (
	Eq  Op = "="
	Neq Op = "!="
	Gt  Op = ">"
	Lt  Op = "<"
	Gte Op = ">="
	Lte Op = "<="
)

// scalarSQL returns the SQL fragment that extracts a scalar text value from
// a JSON column at key.
//
//	SQLite:   json_extract(col, '$.key')
//	Postgres: col->>'key'
func scalarSQL(d Dialect, col, key string) string {
	switch d {
	case DialectPostgres:
		return fmt.Sprintf("%s->>'%s'", col, key)
	default:
		return fmt.Sprintf("json_extract(%s, '$.%s')", col, key)
	}
}

// ---------------------------------------------------------------------------
// Mutations — pass to db.UpdateColumn("col", expr.Set(...))
// ---------------------------------------------------------------------------

// Set writes value at key inside col.
//
//	db.Model(&u).UpdateColumn("meta", expr.Set("meta", "theme", "dark"))
func Set(col, key string, value interface{}) clause.Expr {
	switch CurrentDialect() {
	case DialectPostgres:
		return clause.Expr{
			SQL:  fmt.Sprintf("jsonb_set(%s, '{%s}', to_jsonb(?::text))", col, key),
			Vars: []interface{}{value},
		}
	default:
		return clause.Expr{
			SQL:  fmt.Sprintf("json_set(%s, '$.%s', ?)", col, key),
			Vars: []interface{}{value},
		}
	}
}

// Remove deletes one or more keys from col in a single expression.
//
//	db.Model(&u).UpdateColumn("meta", expr.Remove("meta", "theme"))
//	db.Model(&u).UpdateColumn("meta", expr.Remove("meta", "theme", "lang", "score"))
func Remove(col string, keys ...string) clause.Expr {
	switch CurrentDialect() {
	case DialectPostgres:
		// col #- '{a}' #- '{b}' #- '{c}'
		s := col
		for _, k := range keys {
			s = fmt.Sprintf("%s #- '{%s}'", s, k)
		}
		return clause.Expr{SQL: s}
	default:
		// json_remove(col, '$.a', '$.b', '$.c')
		paths := make([]string, len(keys))
		for i, k := range keys {
			paths[i] = fmt.Sprintf("'$.%s'", k)
		}
		return clause.Expr{SQL: fmt.Sprintf("json_remove(%s, %s)", col, strings.Join(paths, ", "))}
	}
}

// Merge shallow-merges patch into col, adding new keys and overwriting existing ones.
//
//	db.Model(&u).UpdateColumn("meta", expr.Merge("meta", map[string]any{"theme": "dark", "lang": "en"}))
func Merge(col string, patch map[string]interface{}) clause.Expr {
	switch CurrentDialect() {
	case DialectPostgres:
		return clause.Expr{
			SQL:  fmt.Sprintf("%s || ?::jsonb", col),
			Vars: []interface{}{patch},
		}
	default:
		return clause.Expr{
			SQL:  fmt.Sprintf("json_patch(%s, ?)", col),
			Vars: []interface{}{patch},
		}
	}
}

// ---------------------------------------------------------------------------
// Queries — pass to db.Where(expr.Where(...))
// ---------------------------------------------------------------------------

// Where compares the value at key in col using op.
// Numeric ops (Gt, Lt, Gte, Lte) cast the extracted value to a number first,
// so comparisons work correctly instead of falling back to string ordering.
//
//	db.Where(expr.Where("meta", "theme",  expr.Eq,  "dark")).Find(&rows)
//	db.Where(expr.Where("meta", "score",  expr.Gt,  100)).Find(&rows)
//	db.Where(expr.Where("meta", "rating", expr.Lte, 4.5)).Find(&rows)
func Where(col, key string, op Op, value interface{}) clause.Expr {
	d := CurrentDialect()
	raw := scalarSQL(d, col, key)

	if op == Gt || op == Lt || op == Gte || op == Lte {
		switch d {
		case DialectPostgres:
			raw = fmt.Sprintf("(%s)::numeric", raw)
		default:
			raw = fmt.Sprintf("CAST(%s AS REAL)", raw)
		}
	}

	return clause.Expr{
		SQL:  fmt.Sprintf("%s %s ?", raw, op),
		Vars: []interface{}{value},
	}
}

// WhereNull matches rows where key is absent or null.
//
//	db.Where(expr.WhereNull("meta", "deleted_at")).Find(&rows)
func WhereNull(col, key string) clause.Expr {
	return clause.Expr{SQL: fmt.Sprintf("%s IS NULL", scalarSQL(CurrentDialect(), col, key))}
}

// WhereNotNull matches rows where key exists and is not null.
//
//	db.Where(expr.WhereNotNull("meta", "verified")).Find(&rows)
func WhereNotNull(col, key string) clause.Expr {
	return clause.Expr{SQL: fmt.Sprintf("%s IS NOT NULL", scalarSQL(CurrentDialect(), col, key))}
}
