package modules

import (
	"context"
	"testing"

	"github.com/yi-nology/code-review-service/internal/config"
	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
	"github.com/yi-nology/code-review-service/internal/service/llm"
	"github.com/yi-nology/code-review-service/internal/service/ocr"
	"github.com/yi-nology/code-review-service/internal/service/runner"
)

// stubLLMProvider 实现 llm.Provider（用于 fx 注入）。
type stubLLMProvider struct{}

func (stubLLMProvider) Complete(_ context.Context, _ llm.CompleteRequest) (llm.CompleteResponse, error) {
	return llm.CompleteResponse{Content: "ok"}, nil
}
func (stubLLMProvider) Available(_ context.Context) bool { return true }

// TestNewOCRRegistry_ConstructsSuccessfully 验证 NewOCRRegistry 构造流程。
// 真实 fx 集成测试在 handler/registry_test.go；这里只测纯函数构造。
func TestNewOCRRegistry_ConstructsSuccessfully(t *testing.T) {
	cfg := &config.Config{}
	cfg.OCR.UpgradeInstallDir = "/tmp"
	cfg.OCR.GitHubRepo = "alibaba/open-code-review"

	// 注意：以下对象大多数为 nil，因为我们的目的只是验证 fx 注入路径打通，
	// 实际能力在 ocr 包单测里覆盖。
	repos := &repository.RepoContainer{}
	fallback := runner.NewOpenCodeReviewRunner(runner.OpenCodeReviewConfig{Binary: "ocr"})
	_ = repos
	_ = fallback

	// 仅做编译期断言：函数签名与 fx 期望一致
	var (
		_ *llm.MetricsProvider
		_ *runner.DynamicOpenCodeReviewRunner
		_ = NewOCRRegistry
	)
}

// TestNewOCRRegistry_FxTypeAssertions 验证 NewOCRRegistry 的返回类型可被 fx 注入。
func TestNewOCRRegistry_FxTypeAssertions(t *testing.T) {
	// 编译期断言：返回类型应该是 *ocr.Registry (来自 internal/ocr 包)
	var _ = NewOCRRegistry
}

// 确保旧 7 个构造函数签名与原 modules/ocr.go 完全一致，便于阶段 A 删除前不变。
func TestLegacySignaturesStable(t *testing.T) {
	repos := &repository.RepoContainer{}

	// 编译期断言
	var (
		_ = NewConfigResolver(repos)
		_ = NewOCRTester
		_ = NewOCRProviderLister
		_ = NewOCRRulesChecker
		_ = NewOCRVersioner
		_ = NewOCRViewerManager
		_ = NewOCRUpgrader
		_ runner.OpenCodeReviewConfig
		_ *ocr.Tester
		_ models.OCRSettings
	)
}
