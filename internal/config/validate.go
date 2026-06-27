package config

import (
	"fmt"
	"strings"
)

// CurrentEnv returns the environment selected by the -env flag.
func CurrentEnv() string {
	if env == nil {
		return "local"
	}
	return strings.TrimSpace(*env)
}

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
	}

	if len(problems) > 0 {
		return fmt.Errorf("invalid config: %s", strings.Join(problems, "; "))
	}
	return nil
}
