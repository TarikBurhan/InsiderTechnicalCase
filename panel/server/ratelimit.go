package server

import (
	"context"
	"encoding/json"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"notifier/panel/metrics"
	"notifier/panel/response"
	"notifier/pkg/correlation"
)

// RateLimitConfig sets the per-client token buckets of the API.
//
// Two tiers protect the database: "write" requests (POST/PUT/DELETE under
// /api/, e.g. creating notifications, which validate audiences with real
// queries) get a tighter budget than "read" requests (GET under /api/ and
// the WebSocket upgrade). Health, readiness, Prometheus metrics and Swagger
// are not limited.
type RateLimitConfig struct {
	Enabled    bool
	ReadRPS    float64
	ReadBurst  int
	WriteRPS   float64
	WriteBurst int
	// TrustProxy uses the first X-Forwarded-For address as the client key.
	// Only enable it behind a proxy that sets the header.
	TrustProxy bool
	// IdleTTL removes buckets of clients that stayed quiet this long.
	IdleTTL time.Duration
}

const (
	tierRead  = "read"
	tierWrite = "write"
)

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// RateLimiter is an in-memory per-client token bucket limiter. Each Panel
// instance keeps its own buckets.
type RateLimiter struct {
	cfg RateLimitConfig

	mu       sync.Mutex
	visitors map[string]*visitor
}

// NewRateLimiter creates a limiter.
func NewRateLimiter(cfg RateLimitConfig) *RateLimiter {
	if cfg.IdleTTL <= 0 {
		cfg.IdleTTL = 5 * time.Minute
	}
	return &RateLimiter{cfg: cfg, visitors: map[string]*visitor{}}
}

// tierOf returns the limit tier of a request, or "" when it is not limited.
func tierOf(r *http.Request) string {
	switch {
	case strings.HasPrefix(r.URL.Path, "/api/"):
		switch r.Method {
		case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
			return tierWrite
		}
		return tierRead
	case strings.HasPrefix(r.URL.Path, "/ws/"):
		return tierRead
	}
	return ""
}

func (l *RateLimiter) clientKey(r *http.Request) string {
	if l.cfg.TrustProxy {
		if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
			if ip := strings.TrimSpace(strings.Split(fwd, ",")[0]); ip != "" {
				return ip
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (l *RateLimiter) limiterFor(client, tier string) (*rate.Limiter, float64, int) {
	rps, burst := l.cfg.ReadRPS, l.cfg.ReadBurst
	if tier == tierWrite {
		rps, burst = l.cfg.WriteRPS, l.cfg.WriteBurst
	}
	key := tier + "|" + client
	now := time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()
	v, ok := l.visitors[key]
	if !ok {
		v = &visitor{limiter: rate.NewLimiter(rate.Limit(rps), burst)}
		l.visitors[key] = v
	}
	v.lastSeen = now
	return v.limiter, rps, burst
}

// Middleware rejects requests over the budget with 429 Too Many Requests.
func (l *RateLimiter) Middleware(next http.Handler) http.Handler {
	if !l.cfg.Enabled {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tier := tierOf(r)
		if tier == "" {
			next.ServeHTTP(w, r)
			return
		}
		limiter, rps, burst := l.limiterFor(l.clientKey(r), tier)
		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(burst))

		res := limiter.Reserve()
		if delay := res.Delay(); delay > 0 {
			res.Cancel() // do not consume a token for a rejected request
			retryAfter := int(math.Ceil(delay.Seconds()))
			if retryAfter < 1 {
				retryAfter = 1
			}
			metrics.RateLimited.WithLabelValues(tier).Inc()
			w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_ = json.NewEncoder(w).Encode(response.ErrorResponse{
				Error: "rate_limited",
				Message: "too many " + tier + " requests (limit " + strconv.FormatFloat(rps, 'f', -1, 64) +
					"/s, burst " + strconv.Itoa(burst) + "); retry after " + strconv.Itoa(retryAfter) + "s",
				CorrelationID: correlation.FromContext(r.Context()),
			})
			return
		}
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(int(math.Max(0, limiter.Tokens()))))
		next.ServeHTTP(w, r)
	})
}

// RunJanitor removes idle client buckets until ctx is cancelled.
func (l *RateLimiter) RunJanitor(ctx context.Context) {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			l.evictIdle(time.Now())
		}
	}
}

func (l *RateLimiter) evictIdle(now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for k, v := range l.visitors {
		if now.Sub(v.lastSeen) > l.cfg.IdleTTL {
			delete(l.visitors, k)
		}
	}
}

func (l *RateLimiter) size() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.visitors)
}
