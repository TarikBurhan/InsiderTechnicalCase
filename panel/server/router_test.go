package server

import (
	"io"
	"log/slog"
	"testing"
)

func TestNewRouterRegistersWithoutConflicts(t *testing.T) {
	NewRouter(Handlers{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
}
