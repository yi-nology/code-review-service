package ocr

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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
// 15s 足够覆盖 tools 配置查询；用 context.AfterFunc 在 ctx 到期时回收 timer，
// 避免 lostcancel 且无需起额外 goroutine。
func backgroundCtx() context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	context.AfterFunc(ctx, cancel)
	return ctx
}

// ResolveExtraBody 已在 reasoning.go 中定义并实现（阶段 A15 part 9 完整化）。

// safeBackgroundFile 校验 OCR --background-file 路径是否安全（OCR v1.7.6+）。
//
// 安全策略（防路径穿越，对应 docs/CODE_REVIEW_POLICY.md 的敏感输入校验要求）：
//   - 空路径：返回 ("", false)，调用方据此跳过 --background-file。
//   - 解析为绝对路径后必须严格位于 workspaceDir 之下（filepath.Rel 不允许 ".." 前缀）。
//   - 文件必须存在且不是目录（避免把 OCR 引向意外目标）。
//
// 校验失败返回 ("", false)，调用方应记日志并继续评审（不阻断）。
func safeBackgroundFile(workspaceDir, raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}
	ws, err := filepath.Abs(filepath.Clean(workspaceDir))
	if err != nil || ws == "" {
		return "", false
	}
	abs := raw
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(ws, abs)
	}
	abs, err = filepath.Abs(filepath.Clean(abs))
	if err != nil {
		return "", false
	}
	rel, err := filepath.Rel(ws, abs)
	if err != nil || rel == "" || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	info, err := os.Stat(abs)
	if err != nil || info.IsDir() {
		return "", false
	}
	return abs, true
}
