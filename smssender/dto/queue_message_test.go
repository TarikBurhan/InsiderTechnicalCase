package dto

import "testing"

func TestAttemptFor(t *testing.T) {
	m := QueueMessage{Attempt: 3}
	if got := m.AttemptFor(QueueRecipient{Attempt: 2}); got != 2 {
		t.Fatalf("recipient attempt must win, got %d", got)
	}
	if got := m.AttemptFor(QueueRecipient{}); got != 3 {
		t.Fatalf("batch attempt expected, got %d", got)
	}
	if got := (QueueMessage{}).AttemptFor(QueueRecipient{}); got != 1 {
		t.Fatalf("default attempt must be 1, got %d", got)
	}
}
