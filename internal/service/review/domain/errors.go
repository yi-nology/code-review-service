package domain

import "errors"

// 领域级哨兵错误：表达业务约束，与 repo 层（数据层）错误解耦。
//
// PR-13 修复：原 repoerr 包同时承载 repo 错误（NotFound/Conflict/InvalidState）
// 和领域错误（UnsupportedProvider/IdempotencyConflict），导致消费方对错误的
// 「该走哪一层处理」产生歧义。本文件把领域错误归属明确化。
var (
	// ErrUnsupportedProvider 当 Provider 名（如 "github" / "gitlab"）不在
	// 已知平台列表中时返回。Handler 应当翻译为 400/422。
	ErrUnsupportedProvider = errors.New("unsupported provider")

	// ErrIdempotencyConflict 当同一 Idempotency-Key 携带不同 fingerprint 的
	// 请求时返回，提示 client 之前的请求已成功完成但参数不一致。Handler
	// 应当翻译为 409 Conflict。
	ErrIdempotencyConflict = errors.New("idempotency conflict")
)
