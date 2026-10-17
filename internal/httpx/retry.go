// Package httpx 提供 review-service 内部 HTTP 工具。
//
// 当前导出:
//   - RetryConfig / DoWithRetry: 简单 HTTP 请求重试 + 退避。
//
// 不导出:
//   - 走 ekit network/httpx 的语义（service discovery / circuit breaker /
//     Prometheus target label 等）。
//   - 严格 JSON 响应语义（2xx 必须 JSON、错误体 rune 安全截断）走
//     agentkit/httpx.DoJSON（argusclient 在用）。本 helper 保留给宽容 2xx
//     语义的调用方（webhook 任意 2xx 即成功，不强制 JSON 响应体）与
//     deprecated ocr.LLMProvider 参考实现——DoJSONWithRetry 对 2xx 空体
//     会按解析失败报错，webhook 语义不容。
package httpx

import (
	"context"
	"math/rand"
	"time"
)

// RetryConfig 配置 DoWithRetry 的重试行为。
//
// 零值是合理默认: 2 次尝试 / 1s base backoff / 250ms jitter。
type RetryConfig struct {
	// MaxAttempts 总尝试次数（包括首次）。<= 0 视为 2。
	MaxAttempts int
	// BaseBackoff 首次重试前的等待时间。<= 0 视为 1s。
	BaseBackoff time.Duration
	// Jitter 在 [0, Jitter) 区间随机扰动, 加在 BaseBackoff 上, 防止雷鸣群。
	// <= 0 视为 250ms。
	Jitter time.Duration
}

func (c *RetryConfig) applyDefaults() {
	if c.MaxAttempts <= 0 {
		c.MaxAttempts = 2
	}
	if c.BaseBackoff <= 0 {
		c.BaseBackoff = time.Second
	}
	if c.Jitter <= 0 {
		c.Jitter = 250 * time.Millisecond
	}
}

// DoWithRetry 调用 fn 最多 cfg.MaxAttempts 次, 两次之间等待
// BaseBackoff + rand(0, Jitter) (ctx 取消时立即返回 ctx.Err())。
//
// fn 返回值:
//   - retry=true,  err=任意:  本次失败, 继续尝试; attempts 用尽时 err 作为最终返回值
//   - retry=false, err=nil:  成功, 立即返回 nil
//   - retry=false, err=non-nil: 永久失败 (如 4xx / 业务错), 立即返回 err 不再重试
//
// 退避时检查 ctx.Done(): 如果 ctx 取消, 立即返回 ctx.Err() 而非继续重试。
//
// 用法 (LLM provider):
//
//	err := httpx.DoWithRetry(ctx, httpx.RetryConfig{}, func(ctx context.Context) (bool, error) {
//	    resp, err := client.Do(req)
//	    if err != nil { return true, err }
//	    defer resp.Body.Close()
//	    if resp.StatusCode >= 500 || resp.StatusCode == 429 { return true, err }
//	    return false, nil
//	})
func DoWithRetry(ctx context.Context, cfg RetryConfig, fn func(ctx context.Context) (retry bool, err error)) error {
	cfg.applyDefaults()
	var lastErr error
	for attempt := 0; attempt < cfg.MaxAttempts; attempt++ {
		if attempt > 0 {
			delay := cfg.BaseBackoff + time.Duration(rand.Int63n(int64(cfg.Jitter)))
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}
		retry, err := fn(ctx)
		if !retry {
			return err
		}
		lastErr = err
	}
	return lastErr
}
