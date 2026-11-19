// Package service implements the Panel's business rules on top of the
// repositories: validation, audience resolution, idempotency, cancellation.
package service

import (
	"errors"
	"fmt"

	"notifier/panel/repository"
)

// ErrNotFound is returned when a resource does not exist.
var ErrNotFound = repository.ErrNotFound

// ErrConflict is returned when an operation is not allowed in the current state.
var ErrConflict = errors.New("conflict")

func conflictf(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrConflict, fmt.Sprintf(format, args...))
}
