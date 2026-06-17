package ocr

import (
	"context"
	"fmt"
	"strings"

	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
	"github.com/yi-nology/code-review-service/internal/service/review"
	"github.com/yi-nology/code-review-service/internal/service/runner"
)

type ConfigResolver struct {
	settings SettingsSource
	fallback runner.OpenCodeReviewConfig
}

func NewConfigResolver(settings SettingsSource, fallback runner.OpenCodeReviewConfig) *ConfigResolver {
	return &ConfigResolver{settings: settings, fallback: fallback}
}

func (r *ConfigResolver) Resolve(overrides review.OCROverrides) (runner.OpenCodeReviewConfig, error) {
	s, err := r.settings.Get(context.Background())
	if err != nil {
		return runner.OpenCodeReviewConfig{}, fmt.Errorf("读取 OCR 配置失败: %w", err)
	}
	return r.buildConfig(s, overrides), nil
}

func (r *ConfigResolver) ResolveFromSettings(s *models.OCRSettings, overrides review.OCROverrides) runner.OpenCodeReviewConfig {
	return r.buildConfig(s, overrides)
}

func (r *ConfigResolver) buildConfig(s *models.OCRSettings, overrides review.OCROverrides) runner.OpenCodeReviewConfig {
	cfg := runner.OpenCodeReviewConfig{
		Binary:          firstNonBlank(s.Binary, r.fallback.Binary),
		Audience:        firstNonBlank(overrides.Audience, s.Audience, r.fallback.Audience, "human"),
		RulePath:        r.fallback.RulePath,
		DefaultSeverity: firstNonBlank(overrides.DefaultSeverity, s.DefaultSeverity, r.fallback.DefaultSeverity, string(review.SeverityMajor)),
		URL:             firstNonBlank(overrides.URL, s.URL),
		Model:           firstNonBlank(overrides.Model, s.ModelName),
		AuthToken:       s.AuthToken,
		AuthHeader:      s.AuthHeader,
		UseAnthropic:    overrides.UseAnthropic || s.UseAnthropic,
		ExtraBody:       firstNonBlank(overrides.ExtraBody, string(s.ExtraBody)),
		Language:        firstNonBlank(overrides.Language, s.Language),
		MaxTools:        firstPositive(overrides.MaxTools, s.MaxTools),
		Background:      firstNonBlank(overrides.Background, s.Background),
		ToolsPath:       firstNonBlank(overrides.ToolsPath, s.ToolsPath),
	}
	if overrides.Concurrency > 0 {
		cfg.Concurrency = overrides.Concurrency
	} else if s.Concurrency > 0 {
		cfg.Concurrency = s.Concurrency
	} else {
		cfg.Concurrency = r.fallback.Concurrency
	}
	if overrides.MaxGitProcs > 0 {
		cfg.MaxGitProcs = overrides.MaxGitProcs
	} else if s.MaxGitProcs > 0 {
		cfg.MaxGitProcs = s.MaxGitProcs
	} else {
		cfg.MaxGitProcs = r.fallback.MaxGitProcs
	}
	if overrides.TimeoutMinutes > 0 {
		cfg.TimeoutMinutes = overrides.TimeoutMinutes
	} else if s.TimeoutMinutes > 0 {
		cfg.TimeoutMinutes = s.TimeoutMinutes
	} else {
		cfg.TimeoutMinutes = r.fallback.TimeoutMinutes
	}
	return cfg
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func firstPositive(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}
