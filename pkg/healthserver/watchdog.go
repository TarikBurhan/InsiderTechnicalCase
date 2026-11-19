package healthserver

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// minLoopSilence is the shortest silence after which a loop counts as stuck.
const minLoopSilence = 30 * time.Second

// Watchdog detects background loops (polling, retry scheduler, ...) that
// stopped ticking and jobs that stopped making progress. It is used as a
// liveness check: a stuck service reports itself unhealthy on /health.
type Watchdog struct {
	mu         sync.Mutex
	loops      map[string]*loopState
	jobs       map[string]*jobState
	jobStallAt time.Duration
	now        func() time.Time
}

type loopState struct {
	maxSilence time.Duration
	last       time.Time
}

type jobState struct {
	desc string
	last time.Time
}

// NewWatchdog creates a watchdog. A job without progress for jobStallAfter
// is reported as stalled (0 disables job tracking).
func NewWatchdog(jobStallAfter time.Duration) *Watchdog {
	return &Watchdog{loops: map[string]*loopState{}, jobs: map[string]*jobState{}, jobStallAt: jobStallAfter, now: time.Now}
}

// Loop registers a loop expected to tick every interval. It counts as stuck
// after 3 intervals (at least 30s) without a Beat.
func (w *Watchdog) Loop(name string, interval time.Duration) {
	if w == nil {
		return
	}
	silence := 3 * interval
	if silence < minLoopSilence {
		silence = minLoopSilence
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.loops[name] = &loopState{maxSilence: silence, last: w.now()}
}

// Beat records that a loop completed an iteration.
func (w *Watchdog) Beat(name string) {
	if w == nil {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if l, ok := w.loops[name]; ok {
		l.last = w.now()
	}
}

// JobStarted starts tracking a job.
func (w *Watchdog) JobStarted(id, desc string) {
	if w == nil || w.jobStallAt <= 0 {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.jobs[id] = &jobState{desc: desc, last: w.now()}
}

// JobProgress records progress of a job.
func (w *Watchdog) JobProgress(id string) {
	if w == nil {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if j, ok := w.jobs[id]; ok {
		j.last = w.now()
	}
}

// JobDone stops tracking a job.
func (w *Watchdog) JobDone(id string) {
	if w == nil {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.jobs, id)
}

// Check returns an error listing every stuck loop and stalled job.
func (w *Watchdog) Check(_ context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	now := w.now()
	var problems []string
	for name, l := range w.loops {
		if silent := now.Sub(l.last); silent > l.maxSilence {
			problems = append(problems, fmt.Sprintf("loop %s has not ticked for %s", name, silent.Round(time.Second)))
		}
	}
	for id, j := range w.jobs {
		if idle := now.Sub(j.last); idle > w.jobStallAt {
			problems = append(problems, fmt.Sprintf("%s %s made no progress for %s", j.desc, id, idle.Round(time.Second)))
		}
	}
	if len(problems) == 0 {
		return nil
	}
	sort.Strings(problems)
	return errors.New(strings.Join(problems, "; "))
}
