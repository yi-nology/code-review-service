package review

import "errors"

var (
	ErrUnsupportedProvider = errors.New("unsupported provider")
	ErrIdempotencyConflict = errors.New("idempotency key was already used with a different request")
)
