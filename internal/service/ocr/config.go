package ocr

import (
	"context"
	"fmt"
	"strings"

	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
	"github.com/yi-nology/code-review-service/internal/service/review"
	"github.com/yi-nology/code-review-service/internal/service/runner"
	"github.com/oekit/pkg/slicex"
)

// ConfigResolver 合并多来源的 OCR 配置。
//
// 配置优先级（从高到低）:
//  1. overrides — 请求级 / 项目策略级覆盖（review.OCROverrides）
//  2. settings  — 数据库全局配置（models.OCRSettings）
//  3. fallback  — YAML 文件兜底（runner.OpenCodeReviewConfig）
//  4. 内置默认值（如 audience="human"）
type ConfigResolver struct {
	settings settingsSource
	fallback runner.OpenCodeReviewConfig
}

func NewConfigResolver(settings settingsSource, fallback runner.OpenCodeReviewConfig) *ConfigResolver {
	return &ConfigResolver{settings: settings, fallback: fallback}
}

func (r *ConfigResolver) Resolve(overrides review.OCROverrides) (runner.OpenCodeReviewConfig, error) {
	s, err := r.settings.Get(context.Background())
	if err != nil {
		return runner.OpenCodeReviewConfig{}, fmt.Errorf("读取 OCR 配置失败: %w", err)
	}
	cfg := r.buildConfig(s, overrides)
	if err := cfg.Validate(); err != nil {
		return cfg, fmt.Errorf("OCR 配置校验失败: %w", err)
	}
	return cfg, nil
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
	cfg.Concurrency = firstPositive(overrides.Concurrency, s.Concurrency, r.fallback.Concurrency)
	cfg.MaxGitProcs = firstPositive(overrides.MaxGitProcs, s.MaxGitProcs, r.fallback.MaxGitProcs)
	cfg.TimeoutMinutes = firstPositive(overrides.TimeoutMinutes, s.TimeoutMinutes, r.fallback.TimeoutMinutes)
	return cfg
}

func firstNonBlank(values ...string) string {
	return slicex.First(func(s string) bool { return strings.TrimSpace(s) != "" }, values...)
}

func firstPositive(values ...int) int {
	return slicex.First(func(v int) bool { return v > 0 }, values...)
}
