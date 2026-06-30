package domain

// 本文件保留旧 OCR 接口的 type alias，阶段 A+7 完整迁移后删除。

import "context"

// LLMTester 旧接口保留为 alias（阶段 A：ocr 包新 Tester 已实现）。
type LLMTester = ocrLLMTester

// LLMProviderLister 旧接口保留为 alias。
type LLMProviderLister = ocrLLMProviderLister

// RulesChecker 旧接口保留为 alias。
type RulesChecker = ocrRulesChecker

// OCRVersioner 旧接口保留为 alias。
type OCRVersioner = ocrOCRVersioner

// ViewerManager 旧接口保留为 alias。
type ViewerManager = ocrViewerManager

// OCRUpgrader 旧接口保留为 alias。
type OCRUpgrader = ocrOCRUpgrader

// 以下定义是为兼容编译而保留（实际类型由 internal/ocr 包定义）。
// 阶段 A+7 完整迁移后会改为正式的 interface 声明。

type ocrLLMTester interface {
	TestLLM(ctx context.Context) (*LLMTestResult, error)
}
type ocrLLMProviderLister interface {
	ListLLMProviders(ctx context.Context) ([]LLMProvider, error)
}
type ocrRulesChecker interface {
	RulesCheck(ctx context.Context, path string) (*RulesCheckResult, error)
}
type ocrOCRVersioner interface {
	OCRVersion(ctx context.Context) (*OCRVersion, error)
}
type ocrViewerManager interface {
	ViewerStatus(ctx context.Context) (*ViewerStatus, error)
	StartViewer(ctx context.Context, addr string) (*ViewerStatus, error)
	StopViewer(ctx context.Context) (*ViewerStatus, error)
}
type ocrOCRUpgrader interface {
	CheckUpdate(ctx context.Context) (*OCRUpdateInfo, error)
	Upgrade(ctx context.Context, targetVersion string) (*OCRUpgradeStatus, error)
	UpgradeStatus(ctx context.Context) (*OCRUpgradeStatus, error)
}