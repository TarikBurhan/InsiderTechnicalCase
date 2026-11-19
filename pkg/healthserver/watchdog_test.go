package healthserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestWatchdogLoops(t *testing.T) {
	now := time.Now()
	w := NewWatchdog(time.Minute)
	w.now = func() time.Time { return now }
	w.Loop("poller", 2*time.Second) // stuck after 30s (minimum)

	if err := w.Check(context.Background()); err != nil {
		t.Fatalf("fresh loop must be healthy: %v", err)
	}
	now = now.Add(31 * time.Second)
	err := w.Check(context.Background())
	if err == nil || !strings.Contains(err.Error(), "loop poller") {
		t.Fatalf("expected stuck loop, got %v", err)
	}
	w.Beat("poller")
	if err := w.Check(context.Background()); err != nil {
		t.Fatalf("beat must reset the loop: %v", err)
	}
}

func TestWatchdogJobs(t *testing.T) {
	now := time.Now()
	w := NewWatchdog(5 * time.Minute)
	w.now = func() time.Time { return now }
	w.JobStarted("n1", "notification")
	now = now.Add(4 * time.Minute)
	w.JobProgress("n1")
	now = now.Add(4 * time.Minute)
	if err := w.Check(context.Background()); err != nil {
		t.Fatalf("progressing job must be healthy: %v", err)
	}
	now = now.Add(2 * time.Minute)
	if err := w.Check(context.Background()); err == nil || !strings.Contains(err.Error(), "notification n1") {
		t.Fatalf("expected stalled job, got %v", err)
	}
	w.JobDone("n1")
	if err := w.Check(context.Background()); err != nil {
		t.Fatalf("finished job must not be reported: %v", err)
	}
}

func TestNilWatchdogIsSafe(t *testing.T) {
	var w *Watchdog
	w.Beat("x")
	w.JobStarted("a", "b")
	w.JobProgress("a")
	w.JobDone("a")
}

func TestLivenessHandlerReportsFailedChecks(t *testing.T) {
	ok := LivenessHandler("svc", nil)
	rec := httptest.NewRecorder()
	ok(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	failing := LivenessHandler("svc", map[string]Check{"loops": func(context.Context) error { return context.DeadlineExceeded }})
	rec = httptest.NewRecorder()
	failing(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}
