package repoerr

import "errors"

// 仓库层哨兵错误：表达数据访问约束，由 repository 实现返回。
// 业务领域错误（UnsupportedProvider / IdempotencyConflict）应放在
// internal/service/review/domain 包，由 service 层包装或直接返回。
var (
	ErrNotFound     = errors.New("record not found")
	ErrConflict     = errors.New("record conflict")
	ErrInvalidState = errors.New("invalid state")
)
