package redact

import (
	"context"
	"fmt"

	"github.com/oekit/logx"
)

// LogxLogger 是一个 logx.Logger 包装器，自动对所有字符串值调用 String() 脱敏。
//
// 用法（在 main.go / init.go 启动早期）：
//
//	logx.SetDefault(redact.NewLogxLogger(logx.Default()))
//
// 这样所有 logx.Info / logx.Warn / logx.Errorf("...: %v", err) 中的 err.Error()
// 以及结构化键值对都会被自动脱敏，无需逐处手动调用 redact.String。
type LogxLogger struct {
	inner logx.Logger
}

// NewLogxLogger 包装一个底层 logx.Logger。
// 当 inner 为 nil 时，返回的 Logger 退化为 noop（不输出任何日志）。
func NewLogxLogger(inner logx.Logger) *LogxLogger {
	return &LogxLogger{inner: inner}
}

func (l *LogxLogger) Debug(msg string, keysAndValues ...any) {
	if l.inner == nil {
		return
	}
	l.inner.Debug(String(msg), redactAnySlice(keysAndValues)...)
}
func (l *LogxLogger) Info(msg string, keysAndValues ...any) {
	if l.inner == nil {
		return
	}
	l.inner.Info(String(msg), redactAnySlice(keysAndValues)...)
}
func (l *LogxLogger) Warn(msg string, keysAndValues ...any) {
	if l.inner == nil {
		return
	}
	l.inner.Warn(String(msg), redactAnySlice(keysAndValues)...)
}
func (l *LogxLogger) Error(msg string, keysAndValues ...any) {
	if l.inner == nil {
		return
	}
	l.inner.Error(String(msg), redactAnySlice(keysAndValues)...)
}

func (l *LogxLogger) Debugf(format string, args ...any) {
	if l.inner == nil {
		return
	}
	l.inner.Debugf("%s", String(sprintfArgs(format, args)))
}
func (l *LogxLogger) Infof(format string, args ...any) {
	if l.inner == nil {
		return
	}
	l.inner.Infof("%s", String(sprintfArgs(format, args)))
}
func (l *LogxLogger) Warnf(format string, args ...any) {
	if l.inner == nil {
		return
	}
	l.inner.Warnf("%s", String(sprintfArgs(format, args)))
}
func (l *LogxLogger) Errorf(format string, args ...any) {
	if l.inner == nil {
		return
	}
	l.inner.Errorf("%s", String(sprintfArgs(format, args)))
}

func (l *LogxLogger) CtxDebug(ctx context.Context, msg string, keysAndValues ...any) {
	if l.inner == nil {
		return
	}
	l.inner.CtxDebug(ctx, String(msg), redactAnySlice(keysAndValues)...)
}
func (l *LogxLogger) CtxInfo(ctx context.Context, msg string, keysAndValues ...any) {
	if l.inner == nil {
		return
	}
	l.inner.CtxInfo(ctx, String(msg), redactAnySlice(keysAndValues)...)
}
func (l *LogxLogger) CtxWarn(ctx context.Context, msg string, keysAndValues ...any) {
	if l.inner == nil {
		return
	}
	l.inner.CtxWarn(ctx, String(msg), redactAnySlice(keysAndValues)...)
}
func (l *LogxLogger) CtxError(ctx context.Context, msg string, keysAndValues ...any) {
	if l.inner == nil {
		return
	}
	l.inner.CtxError(ctx, String(msg), redactAnySlice(keysAndValues)...)
}

func (l *LogxLogger) CtxDebugf(ctx context.Context, format string, args ...any) {
	if l.inner == nil {
		return
	}
	l.inner.CtxDebugf(ctx, "%s", String(sprintfArgs(format, args)))
}
func (l *LogxLogger) CtxInfof(ctx context.Context, format string, args ...any) {
	if l.inner == nil {
		return
	}
	l.inner.CtxInfof(ctx, "%s", String(sprintfArgs(format, args)))
}
func (l *LogxLogger) CtxWarnf(ctx context.Context, format string, args ...any) {
	if l.inner == nil {
		return
	}
	l.inner.CtxWarnf(ctx, "%s", String(sprintfArgs(format, args)))
}
func (l *LogxLogger) CtxErrorf(ctx context.Context, format string, args ...any) {
	if l.inner == nil {
		return
	}
	l.inner.CtxErrorf(ctx, "%s", String(sprintfArgs(format, args)))
}

func (l *LogxLogger) WithCtx(ctx context.Context) logx.Logger {
	if l.inner == nil {
		return l
	}
	return &LogxLogger{inner: l.inner.WithCtx(ctx)}
}

func (l *LogxLogger) WithService(name string) logx.Logger {
	if l.inner == nil {
		return l
	}
	return &LogxLogger{inner: l.inner.WithService(name)}
}

func (l *LogxLogger) WithAttrs(attrs ...logx.Attr) logx.Logger {
	if l.inner == nil {
		return l
	}
	return &LogxLogger{inner: l.inner.WithAttrs(attrs...)}
}

func (l *LogxLogger) WithGroup(name string) logx.Logger {
	if l.inner == nil {
		return l
	}
	return &LogxLogger{inner: l.inner.WithGroup(name)}
}

func (l *LogxLogger) SetLevel(level logx.Level) {
	if l.inner == nil {
		return
	}
	l.inner.SetLevel(level)
}

// redactAnySlice 接受奇数个 kv 对，递归对 value 做脱敏。
// 期望 keysAndValues 是 [k1, v1, k2, v2, ...] 形式。
func redactAnySlice(kv []any) []any {
	if len(kv) == 0 {
		return kv
	}
	out := make([]any, 0, len(kv))
	for i := 0; i < len(kv); i++ {
		if i%2 == 0 {
			// key
			out = append(out, kv[i])
			continue
		}
		// value
		out = append(out, redactValue(kv[i]))
	}
	return out
}

func redactValue(v any) any {
	switch x := v.(type) {
	case string:
		return String(x)
	case error:
		return String(x.Error())
	default:
		return v
	}
}

// sprintfArgs 模拟 fmt.Sprintf 但接受预格式化的字符串。
// 用于 Errorf(format, args) 场景：先按 fmt 规则格式化，再脱敏。
func sprintfArgs(format string, args []any) string {
	return fmt.Sprintf(format, args...)
}
