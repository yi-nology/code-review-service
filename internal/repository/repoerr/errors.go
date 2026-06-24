package repoerr

import "errors"

var (
	ErrNotFound            = errors.New("record not found")
	ErrConflict            = errors.New("record conflict")
	ErrInvalidState        = errors.New("invalid state")
	ErrIdempotencyConflict = errors.New("idempotency conflict")
	ErrUnsupportedProvider = errors.New("unsupported provider")
)
