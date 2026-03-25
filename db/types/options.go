package types

import (
	"fmt"

	"gorm.io/gorm"
)

type Option func(db *gorm.DB) *gorm.DB

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
			return db.Where(fmt.Sprintf("%s = ?", db.Statement.Quote(field)), value[0])
		}

		return db.Where(fmt.Sprintf("%s IN ?", field), value)
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
