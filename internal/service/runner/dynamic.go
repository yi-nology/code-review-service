package runner

import (
	"context"
	"fmt"
	"strings"

	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
	"github.com/yi-nology/code-review-service/internal/service/review"
)

type ocrSettingsSource interface {
	Get(ctx context.Context) (*models.OCRSettings, error)
}

type globalRulesSource interface {
	Get(ctx context.Context) (*models.GlobalRules, error)
}

type DynamicOpenCodeReviewRunner struct {
	fallback    *OpenCodeReviewRunner
	settings    ocrSettingsSource
	globalRules globalRulesSource
}

func NewDynamicOpenCodeReviewRunner(fallback *OpenCodeReviewRunner, settings ocrSettingsSource, globalRules globalRulesSource) *DynamicOpenCodeReviewRunner {
	return &DynamicOpenCodeReviewRunner{fallback: fallback, settings: settings, globalRules: globalRules}
}

func (r *DynamicOpenCodeReviewRunner) Run(ctx context.Context, workspace review.Workspace) (review.Result, error) {
	s, err := r.settings.Get(ctx)
	if err != nil {
		return review.Result{}, fmt.Errorf("读取 OCR 配置失败: %w", err)
	}
	enabled := s.Enabled
	if workspace.OCR.Enabled != nil {
		enabled = *workspace.OCR.Enabled
	}
	if !enabled {
		return review.Result{}, fmt.Errorf("代码评审未启用，请先在 OCR 配置页面启用并测试模型连接")
	}

	cfg := r.buildConfig(s, workspace.OCR)
	ocr := &OpenCodeReviewRunner{config: cfg}
	return ocr.Run(ctx, workspace)
}

func (r *DynamicOpenCodeReviewRunner) Preview(ctx context.Context, workspace review.Workspace) (*review.PreviewResult, error) {
	s, err := r.settings.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("读取 OCR 配置失败: %w", err)
	}
	if !s.Enabled {
		return nil, fmt.Errorf("代码评审未启用，请先在 OCR 配置页面启用并测试模型连接")
	}

	cfg := r.buildConfig(s, workspace.OCR)
	if strings.TrimSpace(workspace.Dir) == "" {
		return nil, fmt.Errorf("需要已检出的工作区")
	}

	return runPreview(ctx, cfg, workspace)
}

func (r *DynamicOpenCodeReviewRunner) buildConfig(s *models.OCRSettings, overrides review.OCROverrides) OpenCodeReviewConfig {
	cfg := OpenCodeReviewConfig{
		Binary:                  firstNonBlank(s.Binary, r.fallback.config.Binary),
		Audience:                firstNonBlank(overrides.Audience, s.Audience, r.fallback.config.Audience, "human"),
		RulePath:                r.fallback.config.RulePath,
		DefaultSeverity:         firstNonBlank(overrides.DefaultSeverity, s.DefaultSeverity, r.fallback.config.DefaultSeverity, string(review.SeverityMajor)),
		URL:                     firstNonBlank(overrides.URL, s.URL),
		Model:                   firstNonBlank(overrides.Model, s.ModelName),
		AuthToken:               s.AuthToken,
		AuthHeader:              s.AuthHeader,
		UseAnthropic:            overrides.UseAnthropic || s.UseAnthropic,
		ExtraBody:               firstNonBlank(overrides.ExtraBody, string(s.ExtraBody)),
		Language:                firstNonBlank(overrides.Language, s.Language),
		MaxTools:                firstPositive(overrides.MaxTools, s.MaxTools),
		Background:              firstNonBlank(overrides.Background, s.Background),
		ToolsPath:               firstNonBlank(overrides.ToolsPath, s.ToolsPath),
		TelemetryEnabled:        s.TelemetryEnabled,
		TelemetryExporter:       s.TelemetryExporter,
		TelemetryOTLPEndpoint:   s.TelemetryOTLPEndpoint,
		TelemetryContentLogging: s.TelemetryContentLogging,
	}
	if overrides.Concurrency > 0 {
		cfg.Concurrency = overrides.Concurrency
	} else if s.Concurrency > 0 {
		cfg.Concurrency = s.Concurrency
	} else {
		cfg.Concurrency = r.fallback.config.Concurrency
	}
	if overrides.MaxGitProcs > 0 {
		cfg.MaxGitProcs = overrides.MaxGitProcs
	} else if s.MaxGitProcs > 0 {
		cfg.MaxGitProcs = s.MaxGitProcs
	} else {
		cfg.MaxGitProcs = r.fallback.config.MaxGitProcs
	}
	if overrides.TimeoutMinutes > 0 {
		cfg.TimeoutMinutes = overrides.TimeoutMinutes
	} else if s.TimeoutMinutes > 0 {
		cfg.TimeoutMinutes = s.TimeoutMinutes
	} else {
		cfg.TimeoutMinutes = r.fallback.config.TimeoutMinutes
	}
	return cfg
}
