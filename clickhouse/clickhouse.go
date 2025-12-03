package clickhouse

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"net/http"
	"sync"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/gravitl/netmaker/servercfg"
)

type ctxKey string

const clickhouseCtxKey ctxKey = "clickhouse"

var initializeOnce sync.Once

var ch clickhouse.Conn

var ErrConnNotFound = errors.New("no connection in context")

//go:embed initdb.d/01_create_database.sql
var createDBScript string

//go:embed initdb.d/02_create_flows_table.sql
var createFlowsTableScript string

func Initialize() error {
	defer func() {
		fmt.Println("COMPLETED CLICKHOUSE INITIALIZATION")
	}()

	config := servercfg.GetClickHouseConfig()
	chConn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{fmt.Sprintf("%s:%d", config.Host, config.Port)},
		Auth: clickhouse.Auth{
			Database: config.Database,
			Username: config.Username,
			Password: config.Password,
		},
	})
	if err != nil {
		return err
	}

	err = chConn.Exec(context.Background(), createDBScript)
	if err != nil {
		return err
	}

	err = chConn.Exec(context.Background(), createFlowsTableScript)
	if err != nil {
		return err
	}

	ch = chConn
	return nil
}

// WithContext returns a new context with the clickhouse
// connection instance.
//
// Ensure Initialize has been called before using
// this function.
//
// To extract the clickhouse connection use the FromContext
// function.
func WithContext(ctx context.Context) context.Context {
	initializeOnce.Do(func() {
		err := Initialize()
		if err != nil {
			panic(fmt.Errorf("failed to initialize clickhouse connection: %w", err))
		}
	})

	return context.WithValue(ctx, clickhouseCtxKey, ch)
}

// Middleware to auto-inject the clickhouse connection instance
// in a request's context.
//
// Ensure Initialize has been called before using this
// middleware.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(WithContext(r.Context())))
	})
}

// FromContext extracts the clickhouse connection instance from
// the given context.
//
// The function panics, if a connection does not exist.
func FromContext(ctx context.Context) clickhouse.Conn {
	ch, ok := ctx.Value(clickhouseCtxKey).(clickhouse.Conn)
	if !ok {
		panic(ErrConnNotFound)
	}

	return ch
}

// Close closes a connection to the clickhouse database
// (if one exists). It panics if any error
// occurs.
func Close() {
	if ch == nil {
		return
	}

	err := ch.Close()
	if err != nil {
		panic(err)
	}

	ch = nil
}
