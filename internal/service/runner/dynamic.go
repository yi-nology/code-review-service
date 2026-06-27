package runner

import (
	"context"
	"fmt"
	"strings"

	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"
)

type ocrSettingsSource interface {
	Get(ctx context.Context) (*models.OCRSettings, error)
}

type toolPackageSource interface {
	List(ctx context.Context) ([]models.OCRToolPackage, error)
}

type DynamicOpenCodeReviewRunner struct {
	fallback     *OpenCodeReviewRunner
	settings     ocrSettingsSource
	toolPackages toolPackageSource
}

func NewDynamicOpenCodeReviewRunner(fallback *OpenCodeReviewRunner, settings ocrSettingsSource, toolPackages ...toolPackageSource) *DynamicOpenCodeReviewRunner {
	r := &DynamicOpenCodeReviewRunner{fallback: fallback, settings: settings}
	if len(toolPackages) > 0 {
		r.toolPackages = toolPackages[0]
	}
	return r
}

func (r *DynamicOpenCodeReviewRunner) Run(ctx context.Context, workspace domain.Workspace) (domain.Result, error) {
	s, err := r.settings.Get(ctx)
	if err != nil {
		return domain.Result{}, fmt.Errorf("读取 OCR 配置失败: %w", err)
	}
	enabled := s.Enabled
	if workspace.OCR.Enabled != nil {
		enabled = *workspace.OCR.Enabled
	}
	if !enabled {
		return domain.Result{}, fmt.Errorf("代码评审未启用，请先在 OCR 配置页面启用并测试模型连接")
	}

	cfg := r.buildConfig(s, workspace.OCR)
	ocr := &OpenCodeReviewRunner{config: cfg}
	return ocr.Run(ctx, workspace)
}

func (r *DynamicOpenCodeReviewRunner) Preview(ctx context.Context, workspace domain.Workspace) (*domain.PreviewResult, error) {
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

func (r *DynamicOpenCodeReviewRunner) buildConfig(s *models.OCRSettings, overrides domain.OCROverrides) OpenCodeReviewConfig {
	return BuildConfig(s, overrides, r.fallback.config, r.toolPackages)
}
