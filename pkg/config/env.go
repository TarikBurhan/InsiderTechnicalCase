// Package config contains small helpers for reading typed values from the
// environment. Every service builds its own Config struct on top of these.
package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// String returns the env value for key or def when it is unset/empty.
func String(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return def
}

// Int returns the env value for key parsed as int or def.
func Int(key string, def int) int {
	v := String(key, "")
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

// Float returns the env value for key parsed as float64 or def.
func Float(key string, def float64) float64 {
	v := String(key, "")
	if v == "" {
		return def
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return def
	}
	return f
}

// Bool returns the env value for key parsed as bool or def.
func Bool(key string, def bool) bool {
	v := String(key, "")
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}

// Duration returns the env value for key parsed with time.ParseDuration or def.
func Duration(key string, def time.Duration) time.Duration {
	v := String(key, "")
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}

// List returns a comma separated env value as a trimmed slice.
func List(key string, def []string) []string {
	v := String(key, "")
	if v == "" {
		return def
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
