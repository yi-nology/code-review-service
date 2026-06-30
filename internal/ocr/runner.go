package ocr

import (
	"context"
	"fmt"

	"github.com/yi-nology/code-review-service/internal/service/review/domain"
)

// Runner 是 open-code-review 子进程调用入口。
//
// 阶段 A 设计：本类型是 runner.OpenCodeReviewRunner 的薄壳 wrapper，
// 持有 Config（来自 settings + fallback + overrides 合并），
// 实现 domain.Runner / domain.PreviewRunner / domain.ScanRunner 三接口。
//
// 实现细节（subprocess / textparse / scan）当前仍委托给 legacy runner 包；
// 阶段 A+2 起逐步把 exec/parse 逻辑内联到 Runner 内部。
type Runner struct {
	cfg    Config
	legacy legacyRunner // nil if no legacy fallback
}

// legacyRunner 是为避免循环依赖引入的内部接口，
// 满足该接口的有 *internal/service/runner.OpenCodeReviewRunner / DynamicOpenCodeReviewRunner。
type legacyRunner interface {
	RunWithConfig(ctx context.Context, workspace domain.Workspace, cfg Config) (domain.Result, error)
	PreviewWithConfig(ctx context.Context, workspace domain.Workspace, cfg Config) (*domain.PreviewResult, error)
	ScanWithConfig(ctx context.Context, workspace domain.Workspace, req domain.ScanRequest, cfg Config) (*domain.ScanResult, error)
}

// NewRunner 从 Config 构造 Runner。
// 阶段 A：仅持有 cfg；exec 委托给 legacy。
func NewRunner(cfg Config) *Runner {
	return &Runner{cfg: cfg}
}

// NewRunnerWithLegacy 构造带 legacy 委托的 Runner。
// 阶段 A 兼容期使用，阶段 A+2 删除。
func NewRunnerWithLegacy(cfg Config, legacy legacyRunner) *Runner {
	return &Runner{cfg: cfg, legacy: legacy}
}

// Cfg 返回当前 Config（用于外部读取）。
func (r *Runner) Cfg() Config { return r.cfg }

// Refresh 替换 Config（settings 变更后调用）。
// 阶段 A+2 引入 Refresh(ctx) 异步刷新。
func (r *Runner) Refresh(cfg Config) {
	r.cfg = cfg
}

// Run 实现 domain.Runner。
func (r *Runner) Run(ctx context.Context, workspace domain.Workspace) (domain.Result, error) {
	if r.legacy != nil {
		return r.legacy.RunWithConfig(ctx, workspace, r.cfg)
	}
	return domain.Result{}, fmt.Errorf("ocr.Runner: legacy runner not configured (阶段 A+2 内联后可用)")
}

// Preview 实现 domain.PreviewRunner。
func (r *Runner) Preview(ctx context.Context, workspace domain.Workspace) (*domain.PreviewResult, error) {
	if r.legacy != nil {
		return r.legacy.PreviewWithConfig(ctx, workspace, r.cfg)
	}
	return nil, fmt.Errorf("ocr.Runner: legacy runner not configured")
}

// Scan 实现 domain.ScanRunner。
func (r *Runner) Scan(ctx context.Context, workspace domain.Workspace, req domain.ScanRequest) (*domain.ScanResult, error) {
	if r.legacy != nil {
		return r.legacy.ScanWithConfig(ctx, workspace, req, r.cfg)
	}
	return nil, fmt.Errorf("ocr.Runner: legacy runner not configured")
}

// Previewer 包装 Runner 提供 domain.PreviewRunner 接口。
func (r *Runner) Previewer() domain.PreviewRunner { return r }

// Scanner 包装 Runner 提供 domain.ScanRunner 接口。
func (r *Runner) Scanner() domain.ScanRunner { return r }

// 编译期断言
var (
	_ domain.Runner        = (*Runner)(nil)
	_ domain.PreviewRunner = (*Runner)(nil)
	_ domain.ScanRunner    = (*Runner)(nil)
)