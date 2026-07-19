package ocr

import (
	"os"
	"path/filepath"
	"testing"
)

// TestSafeBackgroundFile 校验 OCR v1.7.6+ --background-file 的路径穿越防御（docs/CODE_REVIEW_POLICY.md 敏感输入校验）。
func TestSafeBackgroundFile(t *testing.T) {
	ws := t.TempDir()
	// 在 workspace 内创建一个合法 markdown
	inside := filepath.Join(ws, "docs", "ctx.md")
	if err := os.MkdirAll(filepath.Dir(inside), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(inside, []byte("# ctx"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	// 在 workspace 外创建一个文件
	outside := filepath.Join(t.TempDir(), "secret.md")
	if err := os.WriteFile(outside, []byte("secret"), 0o644); err != nil {
		t.Fatalf("write outside: %v", err)
	}

	tests := []struct {
		name      string
		raw       string
		wantOk    bool
		wantExact string // 期望返回的路径（精确匹配）；为空时只看 wantOk
	}{
		{name: "empty", raw: "", wantOk: false},
		{name: "whitespace", raw: "   ", wantOk: false},
		{name: "relative inside", raw: "docs/ctx.md", wantOk: true, wantExact: inside},
		{name: "relative nested traversal", raw: "../secret.md", wantOk: false},
		{name: "absolute inside", raw: inside, wantOk: true, wantExact: inside},
		{name: "absolute outside", raw: outside, wantOk: false},
		{name: "absolute via symlink-like escape", raw: filepath.Join(ws, "..", filepath.Base(filepath.Dir(outside)), filepath.Base(outside)), wantOk: false},
		{name: "nonexistent", raw: "does-not-exist.md", wantOk: false},
		{name: "directory not file", raw: "docs", wantOk: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := safeBackgroundFile(ws, tt.raw)
			if ok != tt.wantOk {
				t.Fatalf("safeBackgroundFile(%q) ok = %v, want %v (got=%q)", tt.raw, ok, tt.wantOk, got)
			}
			if ok && tt.wantExact != "" && got != tt.wantExact {
				t.Fatalf("safeBackgroundFile(%q) = %q, want %q", tt.raw, got, tt.wantExact)
			}
		})
	}
}

// TestAppendEnv_Protocol 验证 OCR_LLM_PROTOCOL 优先级（OCR v1.7.10+）。
//
// 行为契约：
//   - Protocol 非空时只设 OCR_LLM_PROTOCOL，不设 OCR_USE_ANTHROPIC（避免冲突）。
//   - Protocol 空时维持旧 OCR_USE_ANTHROPIC=true/false 行为（兼容 v1.7.5 及更早 binary）。
func TestAppendEnv_Protocol(t *testing.T) {
	envValue := func(env []string, key string) (string, bool) {
		prefix := key + "="
		for _, e := range env {
			if len(e) >= len(prefix) && e[:len(prefix)] == prefix {
				return e[len(prefix):], true
			}
		}
		return "", false
	}

	t.Run("protocol set takes precedence", func(t *testing.T) {
		cfg := Config{Protocol: "openai-responses", UseAnthropic: true}
		env := cfg.AppendEnv(nil)
		if v, ok := envValue(env, "OCR_LLM_PROTOCOL"); !ok || v != "openai-responses" {
			t.Fatalf("OCR_LLM_PROTOCOL = %q (%v), want openai-responses", v, ok)
		}
		if _, ok := envValue(env, "OCR_USE_ANTHROPIC"); ok {
			t.Fatalf("OCR_USE_ANTHROPIC 不应被设置（Protocol 优先），got env=%v", env)
		}
	})

	t.Run("empty protocol falls back to use_anthropic=true", func(t *testing.T) {
		cfg := Config{UseAnthropic: true}
		env := cfg.AppendEnv(nil)
		if v, ok := envValue(env, "OCR_USE_ANTHROPIC"); !ok || v != "true" {
			t.Fatalf("OCR_USE_ANTHROPIC = %q (%v), want true", v, ok)
		}
		if _, ok := envValue(env, "OCR_LLM_PROTOCOL"); ok {
			t.Fatalf("OCR_LLM_PROTOCOL 不应被设置")
		}
	})

	t.Run("empty protocol falls back to use_anthropic=false", func(t *testing.T) {
		cfg := Config{UseAnthropic: false}
		env := cfg.AppendEnv(nil)
		if v, ok := envValue(env, "OCR_USE_ANTHROPIC"); !ok || v != "false" {
			t.Fatalf("OCR_USE_ANTHROPIC = %q (%v), want false", v, ok)
		}
	})
}

// TestBuildConfig_ProtocolMerge 验证 overrides.Protocol 优先于 settings.Protocol。
func TestBuildConfig_ProtocolMerge(t *testing.T) {
	t.Run("override wins", func(t *testing.T) {
		cfg := BuildConfig(
			&SettingsData{Protocol: "anthropic"},
			OCROverrides{Protocol: "openai-responses"},
			Defaults(), nil,
		)
		if cfg.Protocol != "openai-responses" {
			t.Fatalf("cfg.Protocol = %q, want openai-responses (override)", cfg.Protocol)
		}
	})
	t.Run("settings used when override empty", func(t *testing.T) {
		cfg := BuildConfig(
			&SettingsData{Protocol: "anthropic"},
			OCROverrides{},
			Defaults(), nil,
		)
		if cfg.Protocol != "anthropic" {
			t.Fatalf("cfg.Protocol = %q, want anthropic (settings)", cfg.Protocol)
		}
	})
	t.Run("background_file from settings", func(t *testing.T) {
		cfg := BuildConfig(
			&SettingsData{BackgroundFile: "/ws/docs/ctx.md"},
			OCROverrides{},
			Defaults(), nil,
		)
		if cfg.BackgroundFile != "/ws/docs/ctx.md" {
			t.Fatalf("cfg.BackgroundFile = %q, want /ws/docs/ctx.md", cfg.BackgroundFile)
		}
	})
}
