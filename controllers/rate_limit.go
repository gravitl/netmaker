package controller

import (
	"context"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/gravitl/netmaker/logic"
	"golang.org/x/time/rate"
)

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type RateLimiter struct {
	visitors      map[string]*visitor
	lockoutUntil  map[string]time.Time
	mu            sync.Mutex

	rate            rate.Limit
	burst           int
	lockoutDuration time.Duration
}

func NewRateLimiter(ctx context.Context, r rate.Limit, b int, lockout time.Duration) *RateLimiter {
	rl := &RateLimiter{
		visitors:        make(map[string]*visitor),
		lockoutUntil:    make(map[string]time.Time),
		rate:            r,
		burst:           b,
		lockoutDuration: lockout,
	}

	// cleanup goroutine
	go rl.cleanupVisitors(ctx)

	return rl
}

// allowAuthenticate returns whether the request may proceed; if false, retryAfterSecs is a
// positive value suitable for the Retry-After header.
func (rl *RateLimiter) allowAuthenticate(ip string) (ok bool, retryAfterSecs int) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if until, active := rl.lockoutUntil[ip]; active {
		if time.Now().Before(until) {
			s := int(time.Until(until).Round(time.Second).Seconds())
			if s < 1 {
				s = 1
			}
			return false, s
		}
		delete(rl.lockoutUntil, ip)
	}

	v, exists := rl.visitors[ip]
	if !exists {
		limiter := rate.NewLimiter(rl.rate, rl.burst)
		rl.visitors[ip] = &visitor{
			limiter:  limiter,
			lastSeen: time.Now(),
		}
		v = rl.visitors[ip]
	} else {
		v.lastSeen = time.Now()
	}

	if !v.limiter.Allow() {
		rl.lockoutUntil[ip] = time.Now().Add(rl.lockoutDuration)
		s := int(rl.lockoutDuration.Seconds())
		if s < 1 {
			s = 1
		}
		return false, s
	}
	return true, 0
}

func (rl *RateLimiter) cleanupVisitors(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			rl.mu.Lock()
			now := time.Now()
			for ip, v := range rl.visitors {
				if time.Since(v.lastSeen) > 10*time.Minute {
					delete(rl.visitors, ip)
				}
			}
			for ip, until := range rl.lockoutUntil {
				if !now.Before(until) {
					delete(rl.lockoutUntil, ip)
				}
			}
			rl.mu.Unlock()
		case <-ctx.Done():
			return
		}
	}
}

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		route, err := mux.CurrentRoute(r).GetPathTemplate()
		if err != nil {
			logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
			return
		}

		if r.Method == http.MethodPost && route == "/api/users/adm/authenticate" {
			ip := clientIP(r)

			ok, retryAfter := rl.allowAuthenticate(ip)
			if !ok {
				w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
				http.Error(w, "too many requests", http.StatusTooManyRequests)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[len(parts)-1])
	}

	if xrip := r.Header.Get("X-Real-IP"); xrip != "" {
		return strings.TrimSpace(xrip)
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	return host
}
