package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/oekit/core/config"
	"github.com/oekit/logx"
	"github.com/oekit/tracing"
)

// Load 加载配置。PR-19 修复：原实现用全局 var env = flag.String(...)，
// 不可测试且 init 顺序敏感。改为显式接收 env 参数。
func Load() *Config {
	return LoadWithEnv("local")
}

// LoadWithEnv 用显式环境名加载配置（便于测试与 main.go 显式控制）。
func LoadWithEnv(env string) *Config {
	cfgFile := fmt.Sprintf("etc/config-%s.yaml", env)
	cfg, err := config.LoadBootstrap[Config](env)
	if err != nil {
		if _, statErr := os.Stat(cfgFile); os.IsNotExist(statErr) {
			logx.Warn("config file not found, using defaults", "file", cfgFile, "error", err)
			cfg = &Config{}
		} else {
			logx.Errorf("failed to parse config file %s: %v", cfgFile, err)
			cfg = &Config{}
		}
	}

	if cfg.Port == 0 {
		cfg.Port = 8080
	}

	applyEnvOverrides(cfg)

	return cfg
}

func applyEnvOverrides(cfg *Config) {
	if v := config.Get("REVIEW_TOKEN"); v != "" {
		cfg.Auth.Token = v
	}
	// 对外基地址：PR 门禁 check 跳转链接的拼接来源（EXTERNAL_BASE_URL 优先于 yaml）。
	if v := config.Get("EXTERNAL_BASE_URL"); v != "" {
		cfg.ExternalBaseURL = v
	}
	// Argus 对接（反馈回流代理）：ARGUS_BOT_URL 非空启用「误报抑制」回流
	if v := config.Get("ARGUS_BOT_URL"); v != "" {
		cfg.Argus.BotURL = v
	}
	if v := config.Get("ARGUS_BOT_TOKEN"); v != "" {
		cfg.Argus.BotToken = v
	}
	// PR-19：AUTH_INSECURE 显式开关，允许 dev 环境空 token 启动
	if v, ok := config.GetBool("AUTH_INSECURE"); ok {
		cfg.Auth.Insecure = v
	}
	if v := config.Get("HTTP_ADDR"); v != "" {
		// PR-19 修复：使用 net.SplitHostPort 明确端口解析，
		// 同时支持 ":8080" / "0.0.0.0:8080" / "8080" 三种形式。
		port, err := parsePort(v)
		if err == nil {
			cfg.Port = port
		} else {
			logx.Warnf("invalid HTTP_ADDR %q: %v", v, err)
		}
	}

	if v := config.Get("DATABASE_URL"); v != "" {
		cfg.Database.DSN = v
	}
	if v, ok := config.GetInt("DATABASE_MAX_OPEN"); ok {
		cfg.Database.MaxOpenConns = v
	}
	if v, ok := config.GetInt("DATABASE_MAX_IDLE"); ok {
		cfg.Database.MaxIdleConns = v
	}
	if v, ok := config.GetBool("DATABASE_AUTO_MIGRATE"); ok {
		cfg.Database.AutoMigrate = &v
	}

	if v, ok := config.GetInt("WORKER_CONCURRENCY"); ok {
		cfg.Review.WorkerConcurrency = v
	}
	if v := config.Get("WORKER_POLL_INTERVAL"); v != "" {
		cfg.Review.WorkerPollInterval = v
	}
	if v := config.Get("WORKER_STALE_AFTER"); v != "" {
		cfg.Review.WorkerStaleAfter = v
	}

	if v := config.Get("NACOS_ADDR"); v != "" {
		cfg.Nacos.Addr = v
	}
	if v := config.Get("NACOS_NAMESPACE"); v != "" {
		cfg.Nacos.NamespaceId = v
	}
	if v := config.Get("NACOS_SERVICE_NAME"); v != "" {
		cfg.Nacos.ServiceName = v
	}
	if v := config.Get("NACOS_USERNAME"); v != "" {
		cfg.Nacos.Username = v
	}
	if v := config.Get("NACOS_PASSWORD"); v != "" {
		cfg.Nacos.Password = v
	}
	if v := config.Get("NACOS_GROUP"); v != "" {
		cfg.Nacos.Group = v
	}
	if cfg.Nacos.Addr != "" && cfg.Nacos.Group == "" {
		cfg.Nacos.Group = "DEFAULT_GROUP"
	}

	if v := config.Get("OTEL_ENDPOINT"); v != "" {
		cfg.Tracing.Endpoint = v
	}

	if v, ok := config.GetInt("RATE_LIMIT_RPS"); ok {
		cfg.RateLimit.RPS = v
	}
	if v, ok := config.GetInt("RATE_LIMIT_BURST"); ok {
		cfg.RateLimit.Burst = v
	}
	if v := config.Get("WEBHOOK_BOT_USERNAME"); v != "" {
		cfg.Webhook.BotUsername = v
	}
}

func (c *Config) GetWorkerPollInterval() time.Duration {
	return config.ParseDuration(c.Review.WorkerPollInterval, time.Second)
}

func (c *Config) GetWorkerStaleAfter() time.Duration {
	return config.ParseDuration(c.Review.WorkerStaleAfter, 15*time.Minute)
}

func (c *Config) GetTracingConfig() *tracing.Config {
	if c.Tracing.Endpoint == "" {
		return nil
	}
	return &c.Tracing
}

func (c *Config) GetDatabaseConfig() *DatabaseConfig {
	return &c.Database
}

// GetRateLimit 返回限流参数，未配置时回退到默认值（100 rps / burst 200）。
func (c *Config) GetRateLimit() (rps, burst int) {
	const (
		defaultRPS   = 100
		defaultBurst = 200
	)
	rps = c.RateLimit.RPS
	if rps <= 0 {
		rps = defaultRPS
	}
	burst = c.RateLimit.Burst
	if burst <= 0 {
		burst = defaultBurst
	}
	return rps, burst
}

// parsePort 支持 ":8080" / "0.0.0.0:8080" / "8080" 三种形式。
// 非法输入返回 error。
func parsePort(addr string) (int, error) {
	if !strings.Contains(addr, ":") {
		return strconv.Atoi(addr)
	}
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(port)
}
