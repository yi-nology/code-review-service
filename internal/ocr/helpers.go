package ocr

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/oekit/pkg/slicex"
)

// 工具函数：从 runner 包原样迁移，供 config.go 使用。

// errConfig 是 Config 校验错误的工厂函数。
func errConfig(format string, args ...any) error {
	return fmt.Errorf("ocr config: "+format, args...)
}

// firstNonBlank 返回第一个非空白字符串（兼容 runner.firstNonBlank）。
func firstNonBlank(values ...string) string {
	return slicex.First(func(s string) bool { return strings.TrimSpace(s) != "" }, values...)
}

// firstPositive 返回第一个正整数（兼容 runner.firstPositive）。
func firstPositive(values ...int) int {
	return slicex.First(func(v int) bool { return v > 0 }, values...)
}

// formatIntEnv 格式化环境变量。
func formatIntEnv(key string, v int) string {
	return key + "=" + strconv.Itoa(v)
}

// backgroundCtx 返回与请求 ctx 解耦、但带超时的 ctx（用于 tools 查询等不应被请求取消打断的场景）。
// 此前返回裸 context.Background() 无 deadline，DB/LLM 查询可能无限期堆积。
// 15s 足够覆盖 tools 配置查询；ctx 到期时回收 timer，避免 lostcancel。
func backgroundCtx() context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	go func() {
		<-ctx.Done()
		cancel()
	}()
	return ctx
}

// ResolveExtraBody 已在 reasoning.go 中定义并实现（阶段 A15 part 9 完整化）。
