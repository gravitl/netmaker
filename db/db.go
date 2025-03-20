package db

import (
	"context"
	"errors"
	"gorm.io/gorm"
	"net/http"
	"time"
)

var db *gorm.DB

var ErrDBNotFound = errors.New("no db instance in context")

// InitializeDB initializes a connection to the
// database (if not already done) and ensures it
// has the latest schema.
func InitializeDB(models ...interface{}) error {
	if db != nil {
		return nil
	}

	connector, err := newConnector()
	if err != nil {
		return err
	}

	// DB / LIFE ADVICE: try 5 times before giving up.
	for i := 0; i < 5; i++ {
		db, err = connector.connect()
		if err == nil {
			break
		}

		// wait 2s if you have the time.
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		return err
	}

	return db.AutoMigrate(models...)
}

// WithContext returns a new context with the db
// connection instance.
//
// Ensure InitializeDB has been called before using
// this function.
//
// To extract the db connection use the FromContext
// function.
func WithContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, "db", db)
}

// Middleware to auto-inject the db connection instance
// in a request's context.
//
// Ensure InitializeDB has been called before using this
// middleware.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(WithContext(r.Context())))
	})
}

// FromContext extracts the db connection instance from
// the given context.
//
// The function panics, if a connection does not exist.
func FromContext(ctx context.Context) *gorm.DB {
	db, ok := ctx.Value("db").(*gorm.DB)
	if !ok {
		panic(ErrDBNotFound)
	}

	return db
}

// BeginTx returns a context with a new transaction.
// If the context already has a db connection instance,
// it uses that instance. Otherwise, it uses the
// connection initialized in the package.
//
// Ensure InitializeDB has been called before using
// this function.
func BeginTx(ctx context.Context) context.Context {
	dbInCtx, ok := ctx.Value("db").(*gorm.DB)
	if !ok {
		return context.WithValue(ctx, "db", db.Begin())
	}

	return context.WithValue(ctx, "db", dbInCtx.Begin())
}
