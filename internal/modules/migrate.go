package modules

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/yi-nology/code-review-service/internal/repository"

	"github.com/oekit/logx"
	"go.uber.org/fx"
)

var MigrateModule = fx.Options(
	fx.Invoke(MigrateConfigFile),
)

// MigrateConfigFile 在 fx 启动时将 ~/.opencodereview/config.json 同步到数据库。
func MigrateConfigFile(lc fx.Lifecycle, repos *repository.RepoContainer) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			runMigrate(ctx, repos)
			return nil
		},
	})
}

func runMigrate(ctx context.Context, repos *repository.RepoContainer) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	home, _ := os.UserHomeDir()
	filePath := filepath.Join(home, ".opencodereview", "config.json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			logx.Infof("migrate: %s 不存在，跳过同步", filePath)
			return
		}
		logx.Warnf("migrate: read %s failed: %v", filePath, err)
		return
	}

	var cfg struct {
		Provider string `json:"provider,omitempty"`
		LLM      *struct {
			URL          string          `json:"url,omitempty"`
			Model        string          `json:"model,omitempty"`
			AuthToken    string          `json:"auth_token,omitempty"`
			AuthHeader   string          `json:"auth_header,omitempty"`
			UseAnthropic bool            `json:"use_anthropic"`
			ExtraBody    json.RawMessage `json:"extra_body,omitempty"`
		} `json:"llm,omitempty"`
		Language string `json:"language,omitempty"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		logx.Warnf("migrate: parse %s failed: %v", filePath, err)
		return
	}
	if cfg.LLM == nil || (cfg.LLM.URL == "" && cfg.LLM.Model == "") {
		logx.Infof("migrate: config has no LLM section, skip")
		return
	}

	existing, err := repos.OCRSettings.Get(ctx)
	if err != nil {
		logx.Warnf("migrate: read DB settings failed: %v", err)
		return
	}
	if existing.URL != "" {
		logx.Infof("migrate: DB already has URL, skip")
		return
	}

	existing.Provider = cfg.Provider
	existing.URL = cfg.LLM.URL
	existing.ModelName = cfg.LLM.Model
	existing.AuthToken = cfg.LLM.AuthToken
	if cfg.LLM.AuthHeader != "" {
		existing.AuthHeader = cfg.LLM.AuthHeader
	}
	existing.UseAnthropic = cfg.LLM.UseAnthropic
	if len(cfg.LLM.ExtraBody) > 0 {
		existing.ExtraBody = json.RawMessage(cfg.LLM.ExtraBody)
	}
	if cfg.Language != "" {
		existing.Language = cfg.Language
	}

	if err := repos.OCRSettings.Save(ctx, existing); err != nil {
		logx.Warnf("migrate: save to DB failed: %v", err)
		return
	}
	logx.Infof("migrate: synced %s to DB (provider=%s, model=%s)", filePath, cfg.Provider, cfg.LLM.Model)
}