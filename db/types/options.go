package types

import (
	"fmt"

	"gorm.io/gorm"
)

type Option func(db *gorm.DB) *gorm.DB

func WithFilter(field string, value ...interface{}) Option {
	return func(db *gorm.DB) *gorm.DB {
		if len(value) == 0 {
			return db
		}

		if len(value) == 1 {
			return db.Where(fmt.Sprintf("%s = ?", field), value)
		}

		return db.Where(fmt.Sprintf("%s IN ?", field), value)
	}
}

func InAscOrder(fields ...string) Option {
	return func(db *gorm.DB) *gorm.DB {
		for _, field := range fields {
			db = db.Order(fmt.Sprintf("%s ASC", field))
		}

		return db
	}
}

func InDescOrder(fields ...string) Option {
	return func(db *gorm.DB) *gorm.DB {
		for _, field := range fields {
			db = db.Order(fmt.Sprintf("%s DESC", field))
		}

		return db
	}
}
