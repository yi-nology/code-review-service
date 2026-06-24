package repository

import "github.com/yi-nology/code-review-service/internal/repository/repoerr"

// Re-export repo errors for convenience
var (
	ErrNotFound            = repoerr.ErrNotFound
	ErrConflict            = repoerr.ErrConflict
	ErrInvalidState        = repoerr.ErrInvalidState
	ErrIdempotencyConflict = repoerr.ErrIdempotencyConflict
	ErrUnsupportedProvider = repoerr.ErrUnsupportedProvider
)
