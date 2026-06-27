package config

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/oekit/core"
	"github.com/oekit/logx"
	"github.com/oekit/tracing"
)

var env = flag.String("env", "local", "environment: local, dev, prod")

func Load() *Config {
	flag.Parse()

	cfgFile := fmt.Sprintf("etc/config-%s.yaml", *env)
	cfg, err := config.LoadBootstrap[Config](*env)
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
	if v := config.Get("HTTP_ADDR"); v != "" {
		if p, err := strconv.Atoi(strings.TrimPrefix(v, ":")); err == nil {
			cfg.Port = p
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

	if cfg.Git.Platforms == nil {
		cfg.Git.Platforms = make(map[string]GitPlatformConfig)
	}
	if token := config.Get("GITHUB_TOKEN"); token != "" {
		github := cfg.Git.Platforms["github"]
		github.Token = token
		github.Enabled = true
		if github.BaseURL == "" {
			github.BaseURL = "https://api.github.com"
		}
		cfg.Git.Platforms["github"] = github
	}
	if token := config.Get("GITLAB_TOKEN"); token != "" {
		gitlab := cfg.Git.Platforms["gitlab"]
		gitlab.Token = token
		gitlab.Enabled = true
		if gitlab.BaseURL == "" {
			gitlab.BaseURL = "https://gitlab.com/api/v4"
		}
		cfg.Git.Platforms["gitlab"] = gitlab
	}
	if token := config.Get("GITEA_TOKEN"); token != "" {
		gitea := cfg.Git.Platforms["gitea"]
		gitea.Token = token
		gitea.Enabled = true
		if gitea.BaseURL == "" {
			gitea.BaseURL = config.Get("GITEA_BASE_URL")
		}
		cfg.Git.Platforms["gitea"] = gitea
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

	if v, ok := config.GetBool("OCR_ENABLED"); ok {
		cfg.OCR.Enabled = v
	}
	if v := config.Get("OCR_BINARY"); v != "" {
		cfg.OCR.Binary = v
	}
	if v := config.Get("OCR_COMMAND"); v != "" {
		cfg.OCR.Command = v
	}

	if v := config.Get("POLICY_FAIL_ON"); v != "" {
		cfg.Policy.FailOn = v
	}
	if v := config.Get("POLICY_IGNORE_PATHS"); v != "" {
		cfg.Policy.IgnorePaths = strings.Split(v, ",")
	}
	if v, ok := config.GetInt("POLICY_MAX_COMMENTS"); ok {
		cfg.Policy.MaxComments = v
	}

	if v := config.Get("NACOS_ADDR"); v != "" {
		cfg.Nacos.Addr = v
	}
	if v := config.Get("NACOS_NAMESPACE"); v != "" {
		cfg.Nacos.NamespaceId = v
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
