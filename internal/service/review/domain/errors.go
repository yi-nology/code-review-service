package domain

import "github.com/yi-nology/code-review-service/internal/repository"

// 领域错误：re-export repo 层的哨兵错误，供 service/handler 用 domain.ErrX 引用。
var (
	ErrUnsupportedProvider = repository.ErrUnsupportedProvider
	ErrIdempotencyConflict = repository.ErrIdempotencyConflict
)
