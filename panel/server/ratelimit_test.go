package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newTestLimiter(trustProxy bool) http.Handler {
	l := NewRateLimiter(RateLimitConfig{Enabled: true, ReadRPS: 1, ReadBurst: 3, WriteRPS: 1, WriteBurst: 2, TrustProxy: trustProxy})
	return l.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
}

func do(h http.Handler, method, path, ip string, headers map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, nil)
	r.RemoteAddr = ip + ":12345"
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestRateLimitWriteTier(t *testing.T) {
	h := newTestLimiter(false)
	for i := 0; i < 2; i++ {
		if c := do(h, http.MethodPost, "/api/v1/notifications", "10.0.0.1", nil).Code; c != http.StatusOK {
			t.Fatalf("request %d within burst got %d", i, c)
		}
	}
	w := do(h, http.MethodPost, "/api/v1/notifications", "10.0.0.1", nil)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", w.Code)
	}
	if w.Header().Get("Retry-After") == "" || w.Header().Get("X-RateLimit-Remaining") != "0" {
		t.Fatalf("missing rate limit headers: %v", w.Header())
	}
	// Reads have their own budget.
	if c := do(h, http.MethodGet, "/api/v1/notifications", "10.0.0.1", nil).Code; c != http.StatusOK {
		t.Fatalf("read tier must be independent, got %d", c)
	}
	// Other clients are not affected.
	if c := do(h, http.MethodPost, "/api/v1/notifications", "10.0.0.2", nil).Code; c != http.StatusOK {
		t.Fatalf("other client must not be limited, got %d", c)
	}
}

func TestRateLimitExemptPaths(t *testing.T) {
	h := newTestLimiter(false)
	for i := 0; i < 20; i++ {
		for _, p := range []string{"/health", "/ready", "/metrics", "/swagger"} {
			if c := do(h, http.MethodGet, p, "10.0.0.3", nil).Code; c != http.StatusOK {
				t.Fatalf("%s must not be rate limited, got %d", p, c)
			}
		}
	}
}

func TestRateLimitRefills(t *testing.T) {
	l := NewRateLimiter(RateLimitConfig{Enabled: true, ReadRPS: 50, ReadBurst: 1, WriteRPS: 1, WriteBurst: 1})
	h := l.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	if do(h, http.MethodGet, "/api/v1/settings", "10.0.0.4", nil).Code != http.StatusOK {
		t.Fatal("first request must pass")
	}
	if do(h, http.MethodGet, "/api/v1/settings", "10.0.0.4", nil).Code != http.StatusTooManyRequests {
		t.Fatal("second immediate request must be limited")
	}
	time.Sleep(40 * time.Millisecond) // 50 rps -> a token every 20ms
	if c := do(h, http.MethodGet, "/api/v1/settings", "10.0.0.4", nil).Code; c != http.StatusOK {
		t.Fatalf("bucket must refill, got %d", c)
	}
}

func TestRateLimitTrustProxy(t *testing.T) {
	h := newTestLimiter(true)
	hdr := map[string]string{"X-Forwarded-For": "203.0.113.7, 10.0.0.1"}
	for i := 0; i < 2; i++ {
		do(h, http.MethodPost, "/api/v1/notifications", "10.0.0.1", hdr)
	}
	if c := do(h, http.MethodPost, "/api/v1/notifications", "10.0.0.1", hdr).Code; c != http.StatusTooManyRequests {
		t.Fatalf("forwarded client must be limited, got %d", c)
	}
	// Same proxy, different real client.
	if c := do(h, http.MethodPost, "/api/v1/notifications", "10.0.0.1", map[string]string{"X-Forwarded-For": "198.51.100.9"}).Code; c != http.StatusOK {
		t.Fatalf("different forwarded client must pass, got %d", c)
	}
}

func TestRateLimitDisabledAndJanitor(t *testing.T) {
	l := NewRateLimiter(RateLimitConfig{Enabled: false})
	h := l.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	for i := 0; i < 50; i++ {
		if do(h, http.MethodPost, "/api/v1/notifications", "10.0.0.5", nil).Code != http.StatusOK {
			t.Fatal("disabled limiter must pass everything")
		}
	}

	l = NewRateLimiter(RateLimitConfig{Enabled: true, ReadRPS: 1, ReadBurst: 1, WriteRPS: 1, WriteBurst: 1, IdleTTL: time.Minute})
	l.limiterFor("10.0.0.6", tierRead)
	l.evictIdle(time.Now().Add(2 * time.Minute))
	if l.size() != 0 {
		t.Fatal("idle visitors must be evicted")
	}
}
