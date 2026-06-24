package review

import "github.com/yi-nology/code-review-service/internal/repository"

var (
	ErrUnsupportedProvider = repository.ErrUnsupportedProvider
	ErrIdempotencyConflict = repository.ErrIdempotencyConflict
)
