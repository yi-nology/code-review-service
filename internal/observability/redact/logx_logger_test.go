package redact

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/oekit/logx"
)

// fakeLogger 捕获所有日志输出用于测试断言。
type fakeLogger struct {
	lastMsg     string
	lastKVs     []any
	lastFormat  string
	lastArgs    []any
	ctxSeen     context.Context
}

func (f *fakeLogger) Debug(msg string, kv ...any) { f.lastMsg = msg; f.lastKVs = kv }
func (f *fakeLogger) Info(msg string, kv ...any)  { f.lastMsg = msg; f.lastKVs = kv }
func (f *fakeLogger) Warn(msg string, kv ...any)  { f.lastMsg = msg; f.lastKVs = kv }
func (f *fakeLogger) Error(msg string, kv ...any) { f.lastMsg = msg; f.lastKVs = kv }
func (f *fakeLogger) Debugf(format string, args ...any) {
	f.lastFormat = format; f.lastArgs = args
}
func (f *fakeLogger) Infof(format string, args ...any) {
	f.lastFormat = format; f.lastArgs = args
}
func (f *fakeLogger) Warnf(format string, args ...any) {
	f.lastFormat = format; f.lastArgs = args
}
func (f *fakeLogger) Errorf(format string, args ...any) {
	f.lastFormat = format; f.lastArgs = args
}
func (f *fakeLogger) CtxDebug(ctx context.Context, msg string, kv ...any) {
	f.ctxSeen = ctx; f.lastMsg = msg; f.lastKVs = kv
}
func (f *fakeLogger) CtxInfo(ctx context.Context, msg string, kv ...any) {
	f.ctxSeen = ctx; f.lastMsg = msg; f.lastKVs = kv
}
func (f *fakeLogger) CtxWarn(ctx context.Context, msg string, kv ...any) {
	f.ctxSeen = ctx; f.lastMsg = msg; f.lastKVs = kv
}
func (f *fakeLogger) CtxError(ctx context.Context, msg string, kv ...any) {
	f.ctxSeen = ctx; f.lastMsg = msg; f.lastKVs = kv
}
func (f *fakeLogger) CtxDebugf(ctx context.Context, format string, args ...any) {
	f.ctxSeen = ctx; f.lastFormat = format; f.lastArgs = args
}
func (f *fakeLogger) CtxInfof(ctx context.Context, format string, args ...any) {
	f.ctxSeen = ctx; f.lastFormat = format; f.lastArgs = args
}
func (f *fakeLogger) CtxWarnf(ctx context.Context, format string, args ...any) {
	f.ctxSeen = ctx; f.lastFormat = format; f.lastArgs = args
}
func (f *fakeLogger) CtxErrorf(ctx context.Context, format string, args ...any) {
	f.ctxSeen = ctx; f.lastFormat = format; f.lastArgs = args
}
func (f *fakeLogger) WithCtx(ctx context.Context) logx.Logger { f.ctxSeen = ctx; return f }
func (f *fakeLogger) WithService(name string) logx.Logger   { return f }
func (f *fakeLogger) WithAttrs(attrs ...logx.Attr) logx.Logger { return f }
func (f *fakeLogger) WithGroup(name string) logx.Logger    { return f }
func (f *fakeLogger) SetLevel(level logx.Level)            {}

var _ logx.Logger = (*fakeLogger)(nil)

func TestLogxLogger_RedactsMessage(t *testing.T) {
	fake := &fakeLogger{}
	rl := NewLogxLogger(fake)
	rl.Warn("Authorization: Bearer ghp_secret1234567890abcdefghij")
	if strings.Contains(fake.lastMsg, "ghp_secret") {
		t.Fatalf("expected token in message to be redacted, got %q", fake.lastMsg)
	}
}

func TestLogxLogger_RedactsKVErrorValue(t *testing.T) {
	fake := &fakeLogger{}
	rl := NewLogxLogger(fake)
	rl.Error("request failed", "error", errors.New("upstream 401: Bearer sk-1234567890abcdefghij"))
	// 第二个 kv value 应当被脱敏
	if len(fake.lastKVs) < 2 {
		t.Fatalf("expected at least 2 kv entries, got %d", len(fake.lastKVs))
	}
	if v, ok := fake.lastKVs[1].(string); ok {
		if strings.Contains(v, "sk-1234567890") {
			t.Fatalf("expected error value to be redacted, got %q", v)
		}
	} else {
		t.Fatalf("expected string value, got %T", fake.lastKVs[1])
	}
}

func TestLogxLogger_RedactsInfofArgs(t *testing.T) {
	fake := &fakeLogger{}
	rl := NewLogxLogger(fake)
	rl.Errorf("call failed: %v", errors.New("api_key=secretvalue12345"))
	// Errorf 内部会先 fmt.Sprintf 再脱敏
	if len(fake.lastArgs) > 0 {
		if s, ok := fake.lastArgs[0].(string); ok {
			if strings.Contains(s, "secretvalue12345") {
				t.Fatalf("expected formatted string to be redacted, got %q", s)
			}
		}
	}
}

func TestLogxLogger_NilInnerIsNoop(t *testing.T) {
	rl := NewLogxLogger(nil)
	// 不应 panic
	rl.Info("test", "k", "v")
	rl.Errorf("test %s", "v")
	rl.SetLevel(logx.LevelInfo)
}

func TestLogxLogger_WithCtxPreservesContext(t *testing.T) {
	fake := &fakeLogger{}
	rl := NewLogxLogger(fake)
	ctx := context.WithValue(context.Background(), testKey{}, "x")
	child := rl.WithCtx(ctx)
	child.Info("hello")
	if fake.ctxSeen != ctx {
		t.Fatalf("expected ctx to be propagated")
	}
}

type testKey struct{}
