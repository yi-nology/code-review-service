package domain

import "context"

// 本文件是评审核心接口（review.go 风格），不含 OCR 管理接口。
// OCR 管理接口（LLMTester/Lister/Checker/Versioner/Viewer/Upgrader）
// 见 ocr_aliases.go，由 internal/ocr 包实现。

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
	Published   int
	Failed      int
	Skipped     bool
	Stale       bool
	ExpectedSHA string
	ActualSHA   string
}

type PreviewRunner interface {
	Preview(ctx context.Context, workspace Workspace) (*PreviewResult, error)
}

type ScanRunner interface {
	Scan(ctx context.Context, workspace Workspace, req ScanRequest) (*ScanResult, error)
}