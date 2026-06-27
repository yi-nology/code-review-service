package domain

import "context"

// Providers 提供 git 平台 Provider 的查找。
type Providers interface {
	Get(name string) (Provider, bool)
}

// Provider 准备一个评审工作区（克隆、解析 PR/SHA、构建 diff）。
type Provider interface {
	Prepare(context.Context, Request) (Workspace, error)
}

// Runner 执行评审（子进程或直接调用），返回原始结果。
type Runner interface {
	Run(context.Context, Workspace) (Result, error)
}

// WritebackPublisher 将评审结果回写到 git 平台。
type WritebackPublisher interface {
	PublishWithDiff(ctx context.Context, req Request, result Result, diff string) (*WritebackResult, error)
}

type WritebackResult struct {
	Published int
	Failed    int
	Skipped   bool
}

type PreviewRunner interface {
	Preview(ctx context.Context, workspace Workspace) (*PreviewResult, error)
}

type ScanRunner interface {
	Scan(ctx context.Context, workspace Workspace, req ScanRequest) (*ScanResult, error)
}

type LLMTester interface {
	TestLLM(ctx context.Context) (*LLMTestResult, error)
}

type LLMProviderLister interface {
	ListLLMProviders(ctx context.Context) ([]LLMProvider, error)
}

type RulesChecker interface {
	RulesCheck(ctx context.Context, path string) (*RulesCheckResult, error)
}

type OCRVersioner interface {
	OCRVersion(ctx context.Context) (*OCRVersion, error)
}

type ViewerManager interface {
	ViewerStatus(ctx context.Context) (*ViewerStatus, error)
	StartViewer(ctx context.Context, addr string) (*ViewerStatus, error)
	StopViewer(ctx context.Context) (*ViewerStatus, error)
}

type OCRUpgrader interface {
	CheckUpdate(ctx context.Context) (*OCRUpdateInfo, error)
	Upgrade(ctx context.Context, targetVersion string) (*OCRUpgradeStatus, error)
	UpgradeStatus(ctx context.Context) (*OCRUpgradeStatus, error)
}
