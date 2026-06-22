package ocr

import (
	"context"
	"fmt"

	"github.com/yi-nology/code-review-service/internal/pkg/sliceutil"
	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
	"github.com/yi-nology/code-review-service/internal/service/review"
	"github.com/yi-nology/code-review-service/internal/service/runner"
)

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
	return r.buildConfig(s, overrides), nil
}

func (r *ConfigResolver) ResolveFromSettings(s *models.OCRSettings, overrides review.OCROverrides) runner.OpenCodeReviewConfig {
	return r.buildConfig(s, overrides)
}

func (r *ConfigResolver) buildConfig(s *models.OCRSettings, overrides review.OCROverrides) runner.OpenCodeReviewConfig {
	cfg := runner.OpenCodeReviewConfig{
		Binary:          sliceutil.FirstNonBlank(s.Binary, r.fallback.Binary),
		Audience:        sliceutil.FirstNonBlank(overrides.Audience, s.Audience, r.fallback.Audience, "human"),
		RulePath:        r.fallback.RulePath,
		DefaultSeverity: sliceutil.FirstNonBlank(overrides.DefaultSeverity, s.DefaultSeverity, r.fallback.DefaultSeverity, string(review.SeverityMajor)),
		URL:             sliceutil.FirstNonBlank(overrides.URL, s.URL),
		Model:           sliceutil.FirstNonBlank(overrides.Model, s.ModelName),
		AuthToken:       s.AuthToken,
		AuthHeader:      s.AuthHeader,
		UseAnthropic:    overrides.UseAnthropic || s.UseAnthropic,
		ExtraBody:       sliceutil.FirstNonBlank(overrides.ExtraBody, string(s.ExtraBody)),
		Language:        sliceutil.FirstNonBlank(overrides.Language, s.Language),
		MaxTools:        sliceutil.FirstPositive(overrides.MaxTools, s.MaxTools),
		Background:      sliceutil.FirstNonBlank(overrides.Background, s.Background),
		ToolsPath:       sliceutil.FirstNonBlank(overrides.ToolsPath, s.ToolsPath),
	}
	cfg.Concurrency = sliceutil.FirstPositiveOr(overrides.Concurrency, s.Concurrency, r.fallback.Concurrency)
	cfg.MaxGitProcs = sliceutil.FirstPositiveOr(overrides.MaxGitProcs, s.MaxGitProcs, r.fallback.MaxGitProcs)
	cfg.TimeoutMinutes = sliceutil.FirstPositiveOr(overrides.TimeoutMinutes, s.TimeoutMinutes, r.fallback.TimeoutMinutes)
	return cfg
}
