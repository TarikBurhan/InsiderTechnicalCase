package timezone

import (
	"testing"
	"time"
)

func TestDrift(t *testing.T) {
	if d := Drift(time.Now().Add(-2 * time.Second)); d < 2*time.Second || d > 3*time.Second {
		t.Fatalf("unexpected drift %s", d)
	}
	if d := Drift(time.Now().Add(2 * time.Second)); d < time.Second {
		t.Fatalf("future times must give a positive drift, got %s", d)
	}
}

func TestHolderName(t *testing.T) {
	h := NewHolder()
	if h.Name() != "" || h.RestartRequired() {
		t.Fatal("new holder must be empty")
	}
}
