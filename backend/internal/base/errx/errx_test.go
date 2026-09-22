package errx

import (
	"errors"
	"fmt"
	"testing"
)

func TestInvalidInput(t *testing.T) {
	tests := []struct {
		name   string
		format string
		args   []any
		want   string
	}{
		{"no args", "query parameter q is required", nil, "query parameter q is required"},
		{"formatted", "limit must be between 1 and %d", []any{200}, "limit must be between 1 and 200"},
		{"quoted value", "unknown asset type %q", []any{"PYLON"}, `unknown asset type "PYLON"`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := InvalidInput(tc.format, tc.args...)
			if err.Msg != tc.want || err.Error() != tc.want {
				t.Errorf("Msg = %q, Error() = %q, want %q", err.Msg, err.Error(), tc.want)
			}
		})
	}
}

func TestInputError_FoundThroughWrapping(t *testing.T) {
	wrapped := fmt.Errorf("search: %w", InvalidInput("bad %s", "input"))
	var ie *InputError
	if !errors.As(wrapped, &ie) || ie.Msg != "bad input" {
		t.Errorf("errors.As(%v) = %v, want *InputError{bad input}", wrapped, ie)
	}
	if errors.As(fmt.Errorf("plain"), &ie) {
		t.Error("a plain error must not match *InputError")
	}
}

func TestSentinels_MatchThroughWrapping(t *testing.T) {
	for name, sentinel := range map[string]error{"ErrNotFound": ErrNotFound, "ErrConflict": ErrConflict} {
		wrapped := fmt.Errorf("layer: %w", fmt.Errorf("deeper: %w", sentinel))
		if !errors.Is(wrapped, sentinel) {
			t.Errorf("%s: errors.Is failed through wrapping", name)
		}
	}
	if errors.Is(ErrNotFound, ErrConflict) {
		t.Error("ErrNotFound and ErrConflict must be distinct")
	}
}
