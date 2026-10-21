package repoerr

import (
	"errors"
	"testing"

	"github.com/yi-nology/code-review-service/internal/domain/review"
)

func TestErrorsAreDistinct(t *testing.T) {
	errs := []error{ErrNotFound, ErrConflict, ErrInvalidState, domain.ErrIdempotencyConflict, domain.ErrUnsupportedProvider}
	for i, e1 := range errs {
		for j, e2 := range errs {
			if i == j {
				continue
			}
			if errors.Is(e1, e2) {
				t.Errorf("errors at index %d and %d should be distinct, but errors.Is returned true", i, j)
			}
		}
	}
}

func TestErrorsAreComparable(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"ErrNotFound", ErrNotFound},
		{"ErrConflict", ErrConflict},
		{"ErrInvalidState", ErrInvalidState},
		{"ErrIdempotencyConflict", domain.ErrIdempotencyConflict},
		{"ErrUnsupportedProvider", domain.ErrUnsupportedProvider},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wrapped := errors.Join(tt.err, errors.New("context"))
			if !errors.Is(wrapped, tt.err) {
				t.Errorf("errors.Is should find %s in wrapped error", tt.name)
			}
		})
	}
}
