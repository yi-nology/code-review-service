package domain

import (
	"context"

	"github.com/yi-nology/code-review-service/internal/domain/notification"
)

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

// Notifier 在评审事件发生时推送通知（站内 + 外部 webhook）。
// 实现见 internal/service/notification。executor 用它做 fire-and-forget 通知。
type Notifier interface {
	Notify(ctx context.Context, event notification.Event) error
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

// Delegator 暴露 OCR v1.7.11+ 的 delegate 子命令（host-agent 模式）。
//
// 与 Runner/PreviewRunner 不同，delegate 不调 LLM，而是返回文件列表 + 解析后的规则，
// 让外部 AI agent（Claude Code / Cursor / Codex 等）用自己的智能执行 review。
// review-service 是无人值守 gateway，主路径仍走 Runner；delegate 仅供
// 调用方需要「让外部 agent 接管」时使用。
type Delegator interface {
	// DelegatePreview 返回将参与评审的文件列表 + mode/from/to 元数据。
	// 等价于 `ocr delegate preview`。
	DelegatePreview(ctx context.Context, workspace Workspace) (*DelegatePreviewResult, error)
	// DelegateRule 返回按内容分组的解析规则（system/project + custom）。
	// 等价于 `ocr delegate rule <paths...>`。
	DelegateRule(ctx context.Context, workspace Workspace, paths []string) (*DelegateRuleResult, error)
}

// DelegatePreviewResult 是 ocr delegate preview 的解析结果。
//
// RawOutput 保留 OCR 原始文本输出（带 markdown 格式）；
// Files 是从输出里提取的文件路径列表（方便程序化使用）。
type DelegatePreviewResult struct {
	Mode        string   // range / workspace / commit
	From        string   // diff from ref
	To          string   // diff to ref
	MergeBase   string   // merge-base commit SHA
	TotalInsert int      // 总新增行
	TotalDelete int      // 总删除行
	Files       []string // 参与评审的文件路径
	RawOutput   string   // OCR 原始输出（markdown）
}

// DelegateRuleResult 是 ocr delegate rule 的解析结果。
type DelegateRuleResult struct {
	RawOutput string // OCR 原始 markdown 输出（规则按 group 分段）
}
