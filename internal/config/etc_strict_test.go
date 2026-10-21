package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	ekitconfig "github.com/oekit/core/config"
)

// TestEtcConfigsPassStrictMode 守护 etc/config-*.yaml 的键面。
//
// 为什么要有这道门禁：配置里的**未知键会被静默忽略**——键名拼错（如
// `sample_rate` 写成 `sample_rates`）不报错，配置照常加载，只是那个值
// 永远不生效。仓库里已经踩过一次：上游 tracing.Config 没有 yaml tag，
// yaml.v3 的默认键是「字段名整体小写」，于是 `sample_rate` 从未生效
// （prod 配的 0.1 实际跑 100% 采样）。见 TracingConfig 的注释。
//
// 严格模式把「静默失效」变成「启动即报错」，并给出该层合法键清单与
// 近似拼写建议。代价是：配置里多一个上游不认的键，服务就拒绝启动——
// 这正是这道门禁要的效果（配置错误的暴露点应在启动那一刻）。
func TestEtcConfigsPassStrictMode(t *testing.T) {
	for _, env := range []string{"local", "dev", "prod"} {
		t.Run(env, func(t *testing.T) {
			path := filepath.Join("..", "..", "etc", "config-"+env+".yaml")
			if _, err := os.Stat(path); err != nil {
				t.Fatalf("配置文件不存在: %v", err)
			}
			var c Config
			if err := ekitconfig.LoadFile(path, &c, ekitconfig.WithStrict()); err != nil {
				t.Fatalf("配置文件未通过严格模式校验: %v", err)
			}
		})
	}
}

// TestTracingConfigKeysActuallyBind 钉住 tracing 配置键**确实落到字段上**。
//
// 严格模式只能保证「键是合法的」，不能保证「键接到了你以为的字段」——
// 无 tag 的上游结构体会让合法键落到意料之外的字段上（yaml.v3 默认键是
// 字段名整体小写）。这里逐键断言，确保 sample_rate / service_name 之类
// 不会被无声丢掉。
func TestTracingConfigKeysActuallyBind(t *testing.T) {
	path := filepath.Join("..", "..", "etc", "config-prod.yaml")
	var c Config
	if err := ekitconfig.LoadFile(path, &c, ekitconfig.WithStrict()); err != nil {
		t.Fatalf("加载 prod 配置失败: %v", err)
	}
	if c.Tracing.SampleRate != 0.1 {
		t.Errorf("tracing.sample_rate 未生效: got %v, want 0.1", c.Tracing.SampleRate)
	}
	if c.Tracing.Endpoint == "" {
		t.Error("tracing.endpoint 未绑定")
	}
	if c.Tracing.Insecure {
		t.Error("tracing.insecure 应为 false（prod）")
	}
}

// TestGetTracingConfigConverts 验证本地镜像 → 上游结构的转换不丢字段。
func TestGetTracingConfigConverts(t *testing.T) {
	c := &Config{Tracing: TracingConfig{
		ServiceName: "review-service",
		Endpoint:    "otel-collector:4317",
		Insecure:    true,
		SampleRate:  0.25,
		Protocol:    "http",
		Headers:     map[string]string{"X-Token": "v"},
	}}
	got := c.GetTracingConfig()
	if got == nil {
		t.Fatal("GetTracingConfig() 返回 nil")
	}
	if got.ServiceName != "review-service" || got.Endpoint != "otel-collector:4317" ||
		!got.Insecure || got.SampleRate != 0.25 || got.Protocol != "http" {
		t.Errorf("转换丢字段: %+v", got)
	}
	if got.Headers["X-Token"] != "v" {
		t.Errorf("Headers 未转换: %+v", got.Headers)
	}

	// Endpoint 为空时不应产出 tracing 配置（不建 OTLP 导出器）。
	empty := &Config{}
	if empty.GetTracingConfig() != nil {
		t.Error("Endpoint 为空时应返回 nil")
	}
}

// TestLoadWithEnvRejectsUnknownKey 验证配置含未知键时**返回错误**（启动即失败），
// 而不是静默降级成空配置。
//
// 降级的代价：服务带着空配置继续启动，真正的错误（哪个键写错了）被推迟到
// 更远的地方才暴露——表现为「连不上库」「端口不对」这类与根因无关的症状。
func TestLoadWithEnvRejectsUnknownKey(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "etc"), 0o755); err != nil {
		t.Fatal(err)
	}
	// worker_concurency 是 worker_concurrency 的拼写错误。
	bad := "port: 8081\nreview:\n  worker_concurency: 2\n"
	if err := os.WriteFile(filepath.Join(dir, "etc", "config-badkey.yaml"), []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	chdir(t, dir)

	_, err := LoadWithEnv("badkey")
	if err == nil {
		t.Fatal("expected error for unknown config key, got nil")
	}
	if !strings.Contains(err.Error(), "worker_concurency") {
		t.Errorf("error should name the offending key, got: %v", err)
	}
}

// TestLoadWithEnvMissingFileUsesDefaults 验证配置文件缺失时回落到默认值且**不报错**
// （是否可用由 Validate 决定）。
func TestLoadWithEnvMissingFileUsesDefaults(t *testing.T) {
	chdir(t, t.TempDir())

	cfg, err := LoadWithEnv("nosuchenv")
	if err != nil {
		t.Fatalf("missing config file should not error, got: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config with defaults")
	}
	if cfg.Port != 8080 {
		t.Errorf("default port = %d, want 8080", cfg.Port)
	}
}

// chdir 切到指定目录并在测试结束后切回。
func chdir(t *testing.T, dir string) {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
}
