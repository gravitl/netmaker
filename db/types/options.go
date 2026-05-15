package types

import (
	"fmt"
	"strings"

	"github.com/gravitl/netmaker/db/expr"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Option func(db *gorm.DB) *gorm.DB

func WithPreloads(associations ...string) Option {
	return func(db *gorm.DB) *gorm.DB {
		for _, association := range associations {
			db = db.Preload(association)
		}
		return db
	}
}

func WithAllPreloads() Option {
	return func(db *gorm.DB) *gorm.DB {
		return db.Preload(clause.Associations)
	}
}

// WithJoin joins the given association and optionally scopes the join with conditions.
// Conditions are applied only to the join clause, not the outer query.
func WithJoin(association string, conditions ...Option) Option {
	return func(db *gorm.DB) *gorm.DB {
		// NewDB: true creates a fresh *gorm.DB with no inherited clauses (no WHERE, ORDER BY, etc.)
		// This ensures conditions passed here are scoped only to the JOIN, not the outer query.
		condDB := db.Session(&gorm.Session{NewDB: true})
		for _, condition := range conditions {
			condDB = condition(condDB)
		}
		return db.Joins(association, condDB)
	}
}

func WithPagination(page, pageSize int) Option {
	return func(db *gorm.DB) *gorm.DB {
		if page < 1 {
			page = 1
		}

		if pageSize < 1 || pageSize > 100 {
			pageSize = 10
		}

		offset := (page - 1) * pageSize
		return db.Offset(offset).Limit(pageSize)
	}
}

// WithFilter applies a WHERE clause for the given column.
// IMPORTANT: `field` MUST be a trusted, hardcoded column name.
// NEVER pass user-supplied strings as `field`.
func WithFilter(field string, value ...interface{}) Option {
	return func(db *gorm.DB) *gorm.DB {
		if len(value) == 0 {
			return db
		}

		if len(value) == 1 {
			if value[0] == nil {
				return db.Where(fmt.Sprintf("%s IS NULL", db.Statement.Quote(field)))
			}
			return db.Where(fmt.Sprintf("%s = ?", db.Statement.Quote(field)), value[0])
		}

		return db.Where(fmt.Sprintf("%s IN ?", field), value)
	}
}

// WithNotFilter applies a WHERE NOT clause for the given column.
// IMPORTANT: `field` MUST be a trusted, hardcoded column name.
// NEVER pass user-supplied strings as `field`.
func WithNotFilter(field string, value ...interface{}) Option {
	return func(db *gorm.DB) *gorm.DB {
		if len(value) == 0 {
			return db
		}
		if len(value) == 1 {
			if value[0] == nil {
				return db.Where(fmt.Sprintf("%s IS NOT NULL", db.Statement.Quote(field)))
			}
			return db.Where(fmt.Sprintf("%s != ?", db.Statement.Quote(field)), value[0])
		}
		return db.Where(fmt.Sprintf("%s NOT IN ?", field), value)
	}
}

// WithSearchQuery applies a WHERE clause searching `q` across multiple text fields using OR.
// Uses LOWER() for case-insensitive matching across SQLite and PostgreSQL.
// IMPORTANT: `fields` MUST be trusted, hardcoded column names.
// NEVER pass user-supplied strings as `fields`.
func WithSearchQuery(q string, fields ...interface{}) Option {
	return func(db *gorm.DB) *gorm.DB {
		if q == "" || len(fields) == 0 {
			return db
		}
		clauses := make([]string, len(fields))
		args := make([]interface{}, len(fields))
		for i, f := range fields {
			switch v := f.(type) {
			case string:
				clauses[i] = fmt.Sprintf("LOWER(CAST(%s AS TEXT)) LIKE ?", db.Statement.Quote(v))
			case expr.SearchField:
				quoted := db.Statement.Quote(v.Field)
				if v.Formatter != nil {
					clauses[i] = v.Formatter(quoted) + " LIKE ?"
				} else {
					clauses[i] = fmt.Sprintf("LOWER(CAST(%s AS TEXT)) LIKE ?", quoted)
				}
			}
			args[i] = "%" + strings.ToLower(q) + "%"
		}
		return db.Where(strings.Join(clauses, " OR "), args...)
	}
}

func InAscOrder(fields ...string) Option {
	return func(db *gorm.DB) *gorm.DB {
		for _, field := range fields {
			db = db.Order(fmt.Sprintf("%s ASC", db.Statement.Quote(field)))
		}

		return db
	}
}

func InDescOrder(fields ...string) Option {
	return func(db *gorm.DB) *gorm.DB {
		for _, field := range fields {
			db = db.Order(fmt.Sprintf("%s DESC", db.Statement.Quote(field)))
		}

		return db
	}
}
