package request

import (
	"encoding/json"
	"fmt"
	"time"
)

// localLayouts are accepted without an offset and interpreted in the account
// time zone (time.Local is set to it at startup).
var localLayouts = []string{"2006-01-02T15:04:05", "2006-01-02 15:04:05", "2006-01-02T15:04", "2006-01-02 15:04"}

// ParseFlexibleTime parses RFC3339 (with offset) or a local date-time
// without offset, which is read in the account time zone.
func ParseFlexibleTime(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t, nil
	}
	for _, layout := range localLayouts {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid time %q: use RFC3339 (2026-11-20T09:00:00+03:00) or local time (2026-11-20T09:00:00)", s)
}

// FlexibleTime is a JSON time that accepts the formats of ParseFlexibleTime.
type FlexibleTime struct {
	time.Time
}

// UnmarshalJSON implements json.Unmarshaler.
func (t *FlexibleTime) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("time must be a string")
	}
	parsed, err := ParseFlexibleTime(s)
	if err != nil {
		return err
	}
	t.Time = parsed
	return nil
}

// Ptr returns the time as a pointer (nil when t is nil).
func (t *FlexibleTime) Ptr() *time.Time {
	if t == nil {
		return nil
	}
	v := t.Time
	return &v
}
