// Package expr builds dialect-aware SQL expressions for JSON map columns.
// Results are clause.Expr values that pass directly into GORM calls.
//
// Dialect is read once per call from DATABASE env var.
// Falls back to SQLite if unset or unrecognised.
//
//	DATABASE=sqlite   → json_extract / json_set / json_remove / json_patch
//	DATABASE=postgres → ->> / jsonb_set / - / ||
//
// JSON keys are always passed as bind parameters, never interpolated.
package expr

import (
	"encoding/json"
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
	case "postgres":
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

// sqlitePath returns a JSONPath string that safely handles keys containing
// dots or other special characters, e.g. $."netmaker.asdfasdf"
func sqlitePath(key string) string {
	return fmt.Sprintf(`$."%s"`, key)
}

// keyVar returns the bind value addressing key: the key itself on Postgres,
// a JSONPath on SQLite. Keys are always bound, never interpolated, since they
// can be user-controlled (e.g. usernames that are email addresses).
func keyVar(d Dialect, key string) interface{} {
	if d == DialectPostgres {
		return key
	}
	return sqlitePath(key)
}

// scalarSQL returns the SQL fragment that extracts a scalar text value from
// a JSON column at a key bound by the single placeholder (see keyVar).
//
//	SQLite:   json_extract(col, ?)
//	Postgres: col->>?::text
func scalarSQL(d Dialect, col string) string {
	switch d {
	case DialectPostgres:
		return fmt.Sprintf("%s->>?::text", col)
	default:
		return fmt.Sprintf("json_extract(%s, ?)", col)
	}
}

// ---------------------------------------------------------------------------
// Mutations — pass to db.UpdateColumn("col", expr.Set(...))
// ---------------------------------------------------------------------------

// Set writes value at key inside col.
//
//	db.Model(&u).UpdateColumn("meta", expr.Set("meta", "theme", "dark"))
func Set(col, key string, value interface{}) clause.Expr {
	switch d := CurrentDialect(); d {
	case DialectPostgres:
		return clause.Expr{
			SQL:  fmt.Sprintf("jsonb_set(%s, ARRAY[?::text], to_jsonb(?::text))", col),
			Vars: []interface{}{keyVar(d, key), value},
		}
	default:
		return clause.Expr{
			SQL:  fmt.Sprintf("json_set(%s, ?, ?)", col),
			Vars: []interface{}{keyVar(d, key), value},
		}
	}
}

// Remove deletes one or more keys from col in a single expression.
//
//	db.Model(&u).UpdateColumn("meta", expr.Remove("meta", "theme"))
//	db.Model(&u).UpdateColumn("meta", expr.Remove("meta", "theme", "lang", "score"))
func Remove(col string, keys ...string) clause.Expr {
	d := CurrentDialect()
	vars := make([]interface{}, len(keys))
	for i, k := range keys {
		vars[i] = keyVar(d, k)
	}
	switch d {
	case DialectPostgres:
		// col - ?::text - ?::text (removes top-level keys)
		s := col
		for range keys {
			s += " - ?::text"
		}
		return clause.Expr{SQL: s, Vars: vars}
	default:
		// json_remove(col, ?, ?)
		return clause.Expr{
			SQL:  fmt.Sprintf("json_remove(%s, %s)", col, strings.TrimSuffix(strings.Repeat("?, ", len(keys)), ", ")),
			Vars: vars,
		}
	}
}

// RemoveByValue returns a SET expression that rebuilds col without any entries
// whose value equals targetValue.
//
//	db.Model(&u).UpdateColumn("meta", expr.RemoveByValue("meta", "dark"))
//	db.Model(&u).UpdateColumn("meta", expr.RemoveByValue("meta", "english"))
func RemoveByValue(col, targetValue string) clause.Expr {
	switch CurrentDialect() {
	case DialectPostgres:
		return clause.Expr{
			SQL:  fmt.Sprintf("(SELECT COALESCE(jsonb_object_agg(k, v), '{}'::jsonb) FROM jsonb_each_text(%s) AS t(k, v) WHERE v != ?)", col),
			Vars: []interface{}{targetValue},
		}
	default:
		return clause.Expr{
			SQL:  fmt.Sprintf("(SELECT COALESCE(json_group_object(key, value), '{}') FROM json_each(%s) WHERE value != ?)", col),
			Vars: []interface{}{targetValue},
		}
	}
}

// Merge shallow-merges patch into col, adding new keys and overwriting existing ones.
//
//	db.Model(&u).UpdateColumn("meta", expr.Merge("meta", map[string]any{"theme": "dark", "lang": "en"}))
func Merge(col string, patch map[string]interface{}) clause.Expr {
	data, err := json.Marshal(patch)
	if err != nil {
		// fallback to empty object
		data = []byte("{}")
	}

	switch CurrentDialect() {
	case DialectPostgres:
		return clause.Expr{
			SQL:  fmt.Sprintf("%s || ?::jsonb", col),
			Vars: []interface{}{string(data)},
		}
	default:
		return clause.Expr{
			SQL:  fmt.Sprintf("json_patch(%s, ?)", col),
			Vars: []interface{}{string(data)},
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
	raw := scalarSQL(d, col)

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
		Vars: []interface{}{keyVar(d, key), value},
	}
}

// WhereNull matches rows where key is absent or null.
//
//	db.Where(expr.WhereNull("meta", "deleted_at")).Find(&rows)
func WhereNull(col, key string) clause.Expr {
	d := CurrentDialect()
	return clause.Expr{SQL: fmt.Sprintf("%s IS NULL", scalarSQL(d, col)), Vars: []interface{}{keyVar(d, key)}}
}

// WhereNotNull matches rows where key exists and is not null.
//
//	db.Where(expr.WhereNotNull("meta", "verified")).Find(&rows)
func WhereNotNull(col, key string) clause.Expr {
	d := CurrentDialect()
	return clause.Expr{SQL: fmt.Sprintf("%s IS NOT NULL", scalarSQL(d, col)), Vars: []interface{}{keyVar(d, key)}}
}

// WhereHasKey matches rows where key is present in col, whatever its value
// (including JSON null).
//
//	db.Where(expr.WhereHasKey("meta", "theme")).Find(&rows)
func WhereHasKey(col, key string) clause.Expr {
	switch d := CurrentDialect(); d {
	case DialectPostgres:
		// jsonb_exists is the function form of the `?` operator, which would
		// otherwise clash with GORM's placeholder.
		return clause.Expr{SQL: fmt.Sprintf("jsonb_exists(%s, ?::text)", col), Vars: []interface{}{keyVar(d, key)}}
	default:
		return clause.Expr{SQL: fmt.Sprintf("json_type(%s, ?) IS NOT NULL", col), Vars: []interface{}{keyVar(d, key)}}
	}
}

// WhereHasValue matches rows where any entry in col has the given value.
//
//	db.Where(expr.WhereHasValue("meta", "dark")).Find(&rows)
func WhereHasValue(col, value string) clause.Expr {
	switch CurrentDialect() {
	case DialectPostgres:
		return clause.Expr{
			SQL:  fmt.Sprintf("EXISTS (SELECT 1 FROM jsonb_each_text(%s) WHERE value = ?)", col),
			Vars: []interface{}{value},
		}
	default:
		return clause.Expr{
			SQL:  fmt.Sprintf("EXISTS (SELECT 1 FROM json_each(%s) WHERE value = ?)", col),
			Vars: []interface{}{value},
		}
	}
}
