package clickhouse

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/gravitl/netmaker/servercfg"
)

type ctxKey string

const clickhouseCtxKey ctxKey = "clickhouse"

var ch clickhouse.Conn

var ErrConnNotFound = errors.New("no connection in context")

//go:embed initdb.d/02_create_flows_table.sql
var createFlowsTableScript string

func Initialize() error {
	if ch != nil {
		return nil
	}

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

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err = chConn.Exec(ctx, createFlowsTableScript)
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
// Returns the connection and an error if one does not exist.
func FromContext(ctx context.Context) (clickhouse.Conn, error) {
	ch, ok := ctx.Value(clickhouseCtxKey).(clickhouse.Conn)
	if !ok {
		return nil, ErrConnNotFound
	}

	return ch, nil
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
