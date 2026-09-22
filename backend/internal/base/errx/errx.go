// Package errx holds the small set of errors shared across the repository,
// service and handler layers, so each layer can wrap, match and map them
// without importing the layer below just to name a sentinel.
//
// Errors that only mean something to one feature (an import's schema
// mismatch, say) stay with that feature.
package errx

import (
	"errors"
	"fmt"
)

var (
	// ErrNotFound is returned when a single-row lookup matches nothing.
	ErrNotFound = errors.New("not found")
	// ErrConflict is returned when a write collides with existing data
	// (unique or foreign-key violation) — for an import, another upload
	// changed the assets table between validation and commit.
	ErrConflict = errors.New("conflict with existing data")
)

// ConflictError is a request that cannot be applied to the current data
// (deleting an asset that still has children, say). Unlike ErrConflict, which
// means "someone else changed the data, retry", the message is safe to show
// and retrying will not help until the caller changes something.
type ConflictError struct{ Msg string }

func (e *ConflictError) Error() string { return e.Msg }

// Conflict returns a *ConflictError with a formatted message.
func Conflict(format string, args ...any) *ConflictError {
	return &ConflictError{Msg: fmt.Sprintf(format, args...)}
}

// InputError is a client-correctable problem with a request parameter.
type InputError struct{ Msg string }

func (e *InputError) Error() string { return e.Msg }

// InvalidInput returns an *InputError with a formatted message.
func InvalidInput(format string, args ...any) *InputError {
	return &InputError{Msg: fmt.Sprintf(format, args...)}
}
