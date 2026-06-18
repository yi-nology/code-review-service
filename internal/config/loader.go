package config

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/oekit/core"
	"github.com/oekit/tracing"
)

var env = flag.String("env", "local", "environment: local, dev, prod")

func Load() *Config {
	flag.Parse()

	cfg, err := config.LoadBootstrap[Config](*env)
	if err != nil {
		configFile := fmt.Sprintf("etc/config-%s.yaml", *env)
		slog.Warn("config file not found, using defaults", "file", configFile, "error", err)
		cfg = &Config{}
	}

	if cfg.Port == 0 {
		cfg.Port = 8080
	}

	applyEnvOverrides(cfg)

	return cfg
}

func envString(key string) string   { return os.Getenv(key) }
func envInt(key string) (int, bool) { v, err := strconv.Atoi(os.Getenv(key)); return v, err == nil }
func envBool(key string) (bool, bool) {
	v, err := strconv.ParseBool(os.Getenv(key))
	return v, err == nil
}

func applyEnvOverrides(cfg *Config) {
	if v := envString("REVIEW_TOKEN"); v != "" {
		cfg.Auth.Token = v
	}
	if v := envString("HTTP_ADDR"); v != "" {
		if p, err := strconv.Atoi(strings.TrimPrefix(v, ":")); err == nil {
			cfg.Port = p
		}
	}

	if v := envString("DATABASE_URL"); v != "" {
		cfg.Database.DSN = v
	}
	if v, ok := envInt("DATABASE_MAX_OPEN"); ok {
		cfg.Database.MaxOpenConns = v
	}
	if v, ok := envInt("DATABASE_MAX_IDLE"); ok {
		cfg.Database.MaxIdleConns = v
	}

	if cfg.Git.Platforms == nil {
		cfg.Git.Platforms = make(map[string]GitPlatformConfig)
	}
	if token := envString("GITHUB_TOKEN"); token != "" {
		github := cfg.Git.Platforms["github"]
		github.Token = token
		github.Enabled = true
		if github.BaseURL == "" {
			github.BaseURL = "https://api.github.com"
		}
		cfg.Git.Platforms["github"] = github
	}
	if token := envString("GITLAB_TOKEN"); token != "" {
		gitlab := cfg.Git.Platforms["gitlab"]
		gitlab.Token = token
		gitlab.Enabled = true
		if gitlab.BaseURL == "" {
			gitlab.BaseURL = "https://gitlab.com/api/v4"
		}
		cfg.Git.Platforms["gitlab"] = gitlab
	}
	if token := envString("GITEA_TOKEN"); token != "" {
		gitea := cfg.Git.Platforms["gitea"]
		gitea.Token = token
		gitea.Enabled = true
		if gitea.BaseURL == "" {
			gitea.BaseURL = envString("GITEA_BASE_URL")
		}
		cfg.Git.Platforms["gitea"] = gitea
	}

	if v, ok := envInt("WORKER_CONCURRENCY"); ok {
		cfg.Review.WorkerConcurrency = v
	}
	if v := envString("WORKER_POLL_INTERVAL"); v != "" {
		cfg.Review.WorkerPollInterval = v
	}
	if v := envString("WORKER_STALE_AFTER"); v != "" {
		cfg.Review.WorkerStaleAfter = v
	}

	if v, ok := envBool("OCR_ENABLED"); ok {
		cfg.OCR.Enabled = v
	}
	if v := envString("OCR_BINARY"); v != "" {
		cfg.OCR.Binary = v
	}
	if v := envString("OCR_COMMAND"); v != "" {
		cfg.OCR.Command = v
	}

	if v := envString("POLICY_FAIL_ON"); v != "" {
		cfg.Policy.FailOn = v
	}
	if v := envString("POLICY_IGNORE_PATHS"); v != "" {
		cfg.Policy.IgnorePaths = strings.Split(v, ",")
	}
	if v, ok := envInt("POLICY_MAX_COMMENTS"); ok {
		cfg.Policy.MaxComments = v
	}

	if v := envString("NACOS_ADDR"); v != "" {
		cfg.Nacos.Addr = v
	}
	if v := envString("NACOS_NAMESPACE"); v != "" {
		cfg.Nacos.NamespaceId = v
	}

	if v := envString("OTEL_ENDPOINT"); v != "" {
		cfg.Tracing.Endpoint = v
	}
}

func parseDuration(value string, fallback time.Duration) time.Duration {
	if value == "" {
		return fallback
	}
	d, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return d
}

func (c *Config) GetWorkerPollInterval() time.Duration {
	return parseDuration(c.Review.WorkerPollInterval, time.Second)
}

func (c *Config) GetWorkerStaleAfter() time.Duration {
	return parseDuration(c.Review.WorkerStaleAfter, 15*time.Minute)
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
