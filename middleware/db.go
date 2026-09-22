package middleware

import (
	"net/http"

	"github.com/gravitl/netmaker/db"
)

// DB injects the database connection into the request context.
func DB(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(db.WithContext(r.Context())))
	})
}
