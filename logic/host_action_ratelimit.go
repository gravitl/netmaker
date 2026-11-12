package logic

import (
	"sync"

	"golang.org/x/time/rate"
)

// Per-host limiters for chatty host control-plane actions.
var (
	signalHostLimiters  sync.Map // hostID -> *rate.Limiter
	autoRelayMeLimiters sync.Map // hostID -> *rate.Limiter
)

const (
	// ConnNegotiation signals: burst then ~2/sec sustained per host.
	signalHostRate  = rate.Limit(2)
	signalHostBurst = 10
	// auto_relay_me POSTs should be rare after the first success.
	autoRelayMeRate  = rate.Limit(1)
	autoRelayMeBurst = 3
)

func hostActionLimiter(m *sync.Map, hostID string, r rate.Limit, burst int) *rate.Limiter {
	if v, ok := m.Load(hostID); ok {
		return v.(*rate.Limiter)
	}
	lim := rate.NewLimiter(r, burst)
	actual, _ := m.LoadOrStore(hostID, lim)
	return actual.(*rate.Limiter)
}

// AllowSignalHost reports whether this host may emit another ConnNegotiation signal.
func AllowSignalHost(hostID string) bool {
	if hostID == "" {
		return true
	}
	return hostActionLimiter(&signalHostLimiters, hostID, signalHostRate, signalHostBurst).Allow()
}

// AllowAutoRelayMe reports whether this host may call auto_relay_me.
func AllowAutoRelayMe(hostID string) bool {
	if hostID == "" {
		return true
	}
	return hostActionLimiter(&autoRelayMeLimiters, hostID, autoRelayMeRate, autoRelayMeBurst).Allow()
}
