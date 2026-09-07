package domain

import "context"

// 本文件定义 OCR 管理接口。
//
// 阶段 A+10：从 type alias 升级为正式接口定义。
// ocr 包的 manager（Tester / Versioner / Lister / Checker / Viewer / Upgrader）
// 实现这些接口。

// LLMTester 是 LLM 连通性测试器接口。
type LLMTester interface {
	TestLLM(ctx context.Context) (*LLMTestResult, error)
}

// LLMProviderLister 列出可用 LLM provider。
type LLMProviderLister interface {
	ListLLMProviders(ctx context.Context) ([]LLMProvider, error)
}

// RulesChecker 检查规则文件。
type RulesChecker interface {
	RulesCheck(ctx context.Context, path string) (*RulesCheckResult, error)
}

// OCRVersioner 获取 OCR 版本。
type OCRVersioner interface {
	OCRVersion(ctx context.Context) (*OCRVersion, error)
}

// ViewerManager 管理 OCR viewer 进程。
type ViewerManager interface {
	ViewerStatus(ctx context.Context) (*ViewerStatus, error)
	StartViewer(ctx context.Context, addr string) (*ViewerStatus, error)
	StopViewer(ctx context.Context) (*ViewerStatus, error)
}

// OCRUpgrader 管理 OCR 在线升级。
type OCRUpgrader interface {
	CheckUpdate(ctx context.Context) (*OCRUpdateInfo, error)
	Upgrade(ctx context.Context, targetVersion string) (*OCRUpgradeStatus, error)
	UpgradeStatus(ctx context.Context) (*OCRUpgradeStatus, error)
}

// SessionLister 列出 OCR review session 历史。
type SessionLister interface {
	ListOCRSessions(ctx context.Context) (*OCRSessionListResult, error)
}

// OCRSession 描述一个 OCR review session。
type OCRSession struct {
	SessionID  string `json:"session_id"`
	Status     string `json:"status"`
	Mode       string `json:"mode,omitempty"`
	CreatedAt  string `json:"created_at,omitempty"`
	FilesCount int    `json:"files_count,omitempty"`
}

// OCRSessionListResult 是 ocr session list 的返回。
type OCRSessionListResult struct {
	Sessions []OCRSession `json:"sessions"`
}
