package db

import (
	"context"
	"errors"
	"net/http"
	"time"

	"gorm.io/gorm"
)

type ctxKey string

const dbCtxKey ctxKey = "db"
const isTxCtxKey ctxKey = "isTx"
const started ctxKey = "started"

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

	return db.Transaction(func(tx *gorm.DB) error {
		for _, model := range models {
			err = tx.AutoMigrate(model)
			if err != nil {
				return err
			}
		}

		return nil
	})
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
	return context.WithValue(ctx, dbCtxKey, db)
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
	db, ok := ctx.Value(dbCtxKey).(*gorm.DB)
	if !ok {
		panic(ErrDBNotFound)
	}

	return db
}

func SetPagination(ctx context.Context, page, pageSize int) context.Context {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}
	db := FromContext(ctx)
	offset := (page - 1) * pageSize
	return context.WithValue(ctx, dbCtxKey, db.Offset(offset).Limit(pageSize))
}

// BeginTx returns a context with a new transaction.
// If the context already has a db connection instance,
// it uses that instance. Otherwise, it uses the
// connection initialized in the package.
//
// Ensure InitializeDB has been called before using
// this function.
//
// To commit or rollback the transaction, call the
// CommitTx and RollbackTx functions with the context
// returned by this function.
func BeginTx(ctx context.Context) context.Context {
	// if the context already has a transaction,
	// don't begin a new one.
	_, ok := ctx.Value(isTxCtxKey).(bool)
	if ok {
		// *this* function's caller did not start the transaction.
		return context.WithValue(ctx, started, false)
	}

	dbInCtx, ok := ctx.Value(dbCtxKey).(*gorm.DB)
	if !ok {
		// begin a new transaction.
		ctx = context.WithValue(ctx, dbCtxKey, db.Begin())
		// mark the context as containing a transaction
		// handle.
		ctx = context.WithValue(ctx, isTxCtxKey, true)
		// record that *this* function's caller started the
		// transaction.
		return context.WithValue(ctx, started, true)
	}

	// begin a new transaction.
	ctx = context.WithValue(ctx, dbCtxKey, dbInCtx.Begin())
	// mark the context as containing a transaction
	// handle.
	ctx = context.WithValue(ctx, isTxCtxKey, true)
	// record that *this* function's caller started the
	// transaction.
	return context.WithValue(ctx, started, true)
}

// CommitTx commits a transaction started by BeginTx.
//
// This function does nothing if the context does not
// contain a transaction. It also does nothing if the
// commit is not done by the original caller of BeginTx.
func CommitTx(ctx context.Context) {
	isTxStarter, ok := ctx.Value(started).(bool)
	if !ok {
		return
	}

	if !isTxStarter {
		// *this* function's caller did not start the transaction,
		// so we don't need to commit yet.
		return
	}

	_, ok = ctx.Value(isTxCtxKey).(bool)
	if !ok {
		// this should never happen, because the `started`
		// key was set after the `isTx` key and it exists.
		return
	}

	tx, ok := ctx.Value(dbCtxKey).(*gorm.DB)
	if !ok {
		// this should never happen, because the `isTx`
		// key was set after the `db` key and it exists.
		return
	}

	tx.Commit()
}

// RollbackTx rolls back a transaction started by BeginTx.
//
// This function does nothing if the context does not
// contain a transaction. It also does nothing if the
// rollback is not done by the original caller of BeginTx.
func RollbackTx(ctx context.Context) {
	isTxStarter, ok := ctx.Value(started).(bool)
	if !ok {
		return
	}

	if !isTxStarter {
		// *this* function's caller did not start the transaction,
		// so we don't need to rollback yet.
		return
	}

	_, ok = ctx.Value(isTxCtxKey).(bool)
	if !ok {
		// this should never happen, because the `started`
		// key was set after the `isTx` key and it exists.
		return
	}

	tx, ok := ctx.Value(dbCtxKey).(*gorm.DB)
	if !ok {
		// this should never happen, because the `isTx`
		// key was set after the `db` key and it exists.
		return
	}

	tx.Rollback()
}

// CloseDB close a connection to the database
// (if one exists). It panics if any error
// occurs.
func CloseDB() {
	if db == nil {
		return
	}

	sqlDB, err := db.DB()
	if err != nil {
		panic(err)
	}

	err = sqlDB.Close()
	if err != nil {
		panic(err)
	}

	db = nil
}
