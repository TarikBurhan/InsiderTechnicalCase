package request

import (
	"encoding/json"
	"testing"
	"time"
)

func TestParseFlexibleTime(t *testing.T) {
	istanbul, err := time.LoadLocation("Europe/Istanbul")
	if err != nil {
		t.Skip("tzdata not available")
	}
	prev := time.Local
	time.Local = istanbul
	defer func() { time.Local = prev }()

	withOffset, err := ParseFlexibleTime("2026-11-20T09:00:00Z")
	if err != nil || !withOffset.Equal(time.Date(2026, 11, 20, 9, 0, 0, 0, time.UTC)) {
		t.Fatalf("RFC3339 parse failed: %v %v", withOffset, err)
	}
	local, err := ParseFlexibleTime("2026-11-20T09:00:00")
	if err != nil {
		t.Fatal(err)
	}
	// 09:00 in Istanbul (UTC+3) is 06:00 UTC.
	if !local.Equal(time.Date(2026, 11, 20, 6, 0, 0, 0, time.UTC)) {
		t.Fatalf("local time must be read in the account zone, got %s", local.UTC())
	}
	if _, err := ParseFlexibleTime("20/11/2026"); err == nil {
		t.Fatal("expected error for unsupported format")
	}
}

func TestFlexibleTimeJSON(t *testing.T) {
	var req struct {
		At *FlexibleTime `json:"at"`
	}
	if err := json.Unmarshal([]byte(`{"at":"2026-11-20 09:30"}`), &req); err != nil || req.At == nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if err := json.Unmarshal([]byte(`{"at":"tomorrow"}`), &req); err == nil {
		t.Fatal("expected error")
	}
}
