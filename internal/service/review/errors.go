package review

import "errors"

var (
	ErrUnsupportedProvider = errors.New("unsupported provider")
	ErrIdempotencyConflict = errors.New("idempotency key was already used with a different request")
	ErrIdempotencyKeyTooLong = errors.New("Idempotency-Key must be 200 characters or fewer")
	ErrReviewNotFound = errors.New("review not found")
)
