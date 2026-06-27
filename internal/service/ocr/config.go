package ocr

import (
	"context"
	"fmt"

	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"
	"github.com/yi-nology/code-review-service/internal/service/runner"
)

// ConfigResolver 合并多来源的 OCR 配置。
//
// 配置优先级（从高到低）:
//  1. overrides — 请求级 / 项目策略级覆盖（domain.OCROverrides）
//  2. settings  — 数据库全局配置（models.OCRSettings）
//  3. fallback  — YAML 文件兜底（runner.OpenCodeReviewConfig）
//  4. 内置默认值（如 audience="human"）
//
// 实际合并逻辑委托给 runner.BuildConfig（单一实现，含 reasoning 解析、
// 遥测、上下文窗口、默认工具包回退），消除此前与 runner 的重复。
type ConfigResolver struct {
	settings     settingsSource
	fallback     runner.OpenCodeReviewConfig
	toolPackages runner.ToolPackageLister
}

func NewConfigResolver(settings settingsSource, fallback runner.OpenCodeReviewConfig, toolPackages runner.ToolPackageLister) *ConfigResolver {
	return &ConfigResolver{settings: settings, fallback: fallback, toolPackages: toolPackages}
}

func (r *ConfigResolver) Resolve(overrides domain.OCROverrides) (runner.OpenCodeReviewConfig, error) {
	s, err := r.settings.Get(context.Background())
	if err != nil {
		return runner.OpenCodeReviewConfig{}, fmt.Errorf("读取 OCR 配置失败: %w", err)
	}
	cfg := runner.BuildConfig(s, overrides, r.fallback, r.toolPackages)
	if err := cfg.Validate(); err != nil {
		return cfg, fmt.Errorf("OCR 配置校验失败: %w", err)
	}
	return cfg, nil
}

func (r *ConfigResolver) ResolveFromSettings(s *models.OCRSettings, overrides domain.OCROverrides) runner.OpenCodeReviewConfig {
	return runner.BuildConfig(s, overrides, r.fallback, r.toolPackages)
}
