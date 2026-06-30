package config

import (
	"fmt"
	"strings"
)

// CurrentEnv returns the environment selected by the caller.
// PR-19 修复：原实现依赖全局 var env，删除后改为参数化。
// main.go 通过 LoadWithEnv 显式传入；测试可注入任意值。
func CurrentEnv() string {
	return currentEnv
}

// SetCurrentEnv 注入当前环境名（仅 main 启动时调用一次）。
func SetCurrentEnv(env string) {
	currentEnv = strings.TrimSpace(env)
}

var currentEnv = "local"

// Validate checks runtime configuration that should fail fast before fx starts.
func (c *Config) Validate(environment string) error {
	var problems []string

	if c.Port <= 0 || c.Port > 65535 {
		problems = append(problems, fmt.Sprintf("port must be between 1 and 65535, got %d", c.Port))
	}
	dsn := strings.TrimSpace(c.Database.DSN)
	if dsn == "" {
		problems = append(problems, "DATABASE_URL/database.dsn is required")
	}
	if strings.Contains(dsn, "${") {
		problems = append(problems, "DATABASE_URL/database.dsn contains an unresolved placeholder")
	}
	if c.Database.MaxIdleConns < 0 {
		problems = append(problems, "database.max_idle_conns must not be negative")
	}
	if c.Database.MaxOpenConns < 0 {
		problems = append(problems, "database.max_open_conns must not be negative")
	}
	if c.Database.MaxOpenConns > 0 && c.Database.MaxIdleConns > c.Database.MaxOpenConns {
		problems = append(problems, "database.max_idle_conns must not exceed database.max_open_conns")
	}

	if strings.EqualFold(strings.TrimSpace(environment), "prod") {
		token := strings.TrimSpace(c.Auth.Token)
		if token == "" {
			problems = append(problems, "REVIEW_TOKEN/auth.token is required in prod")
		}
		if strings.Contains(token, "${") {
			problems = append(problems, "REVIEW_TOKEN/auth.token contains an unresolved placeholder")
		}
		if token == "dev-token" {
			problems = append(problems, "REVIEW_TOKEN/auth.token must not use dev-token in prod")
		}
		// PR-19 修复：prod 严禁 AUTH_INSECURE=true，
		// 防止"忘配 token 全开放"部署事故。
		if c.Auth.Insecure {
			problems = append(problems, "AUTH_INSECURE must not be true in prod")
		}
	} else {
		// dev / local：空 token 启动需显式 AUTH_INSECURE=true
		if strings.TrimSpace(c.Auth.Token) == "" && !c.Auth.Insecure {
			problems = append(problems,
				"REVIEW_TOKEN/auth.token is empty in "+environment+
					"; set AUTH_INSECURE=true to allow insecure startup (dev only)")
		}
	}

	if len(problems) > 0 {
		return fmt.Errorf("invalid config: %s", strings.Join(problems, "; "))
	}
	return nil
}
