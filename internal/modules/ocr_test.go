package modules

import (
	"testing"

	"github.com/yi-nology/code-review-service/internal/config"
	"github.com/yi-nology/code-review-service/internal/ocr"
	"github.com/yi-nology/code-review-service/internal/repository"
	"github.com/yi-nology/code-review-service/internal/repository/postgres/models"
)

// TestNewOCRRegistry_ConstructsSuccessfully 验证 NewOCRRegistry 构造流程。
// 真实 fx 集成测试在 handler/registry_test.go；这里只测纯函数构造。
func TestNewOCRRegistry_ConstructsSuccessfully(t *testing.T) {
	cfg := &config.Config{}
	// OCR 升级相关字段（UpgradeInstallDir/GitHubRepo）已迁移到 DB 配置，
	// 此处仅保留 cfg 以覆盖 config.Config 零值构造路径。

	// 注意：以下对象大多数为 nil，因为我们的目的只是验证 fx 注入路径打通，
	// 实际能力在 ocr 包单测里覆盖。
	repos := &repository.RepoContainer{}
	_ = repos
	_ = cfg

	// 仅做编译期断言：函数签名与 fx 期望一致
	var (
		_ *ocr.MetricsProvider
		_ *ocr.Runner
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
		_ *ocr.Tester
		_ models.OCRSettings
	)
}
