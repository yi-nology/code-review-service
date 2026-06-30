// Package adapter 提供 ocr.Runner 与 legacy runner 包之间的桥接。
//
// 阶段 A：ocr.Runner 持有 Config 但 exec 仍走 legacy runner.OpenCodeReviewRunner。
// 本文件提供适配器，让 legacy runner 接受 ocr.Config 而非自己的 OpenCodeReviewConfig。
//
// 阶段 A+3 删掉（runner 包整体被吸收）。
package adapter

import (
	"context"

	"github.com/yi-nology/code-review-service/internal/ocr"
	runnerpkg "github.com/yi-nology/code-review-service/internal/service/runner"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"
)

// LegacyRunner 桥接 *runner.OpenCodeReviewRunner 到 ocr.legacyRunner 接口。
// Run/Preview/Scan 方法从 ocr.Config 重建 runner.OpenCodeReviewConfig 然后调用底层 runner。
//
// 阶段 A 兼容层：cfg 暂未真正传入底层（底层仍用自身 config），等阶段 A+3 替换为
// 直接用 cfg 构造子进程。
type LegacyRunner struct {
	inner *runnerpkg.OpenCodeReviewRunner
	dyn   *runnerpkg.DynamicOpenCodeReviewRunner
}

// RunWithConfig 桥接到底层 runner.Run。
func (l *LegacyRunner) RunWithConfig(ctx context.Context, workspace domain.Workspace, _ ocr.Config) (domain.Result, error) {
	if l.inner != nil {
		return l.inner.Run(ctx, workspace)
	}
	if l.dyn != nil {
		return l.dyn.Run(ctx, workspace)
	}
	return domain.Result{}, nil
}

func (l *LegacyRunner) PreviewWithConfig(ctx context.Context, workspace domain.Workspace, _ ocr.Config) (*domain.PreviewResult, error) {
	if l.dyn != nil {
		return l.dyn.Preview(ctx, workspace)
	}
	return nil, nil
}

func (l *LegacyRunner) ScanWithConfig(ctx context.Context, workspace domain.Workspace, req domain.ScanRequest, _ ocr.Config) (*domain.ScanResult, error) {
	if l.dyn != nil {
		return l.dyn.Scan(ctx, workspace, req)
	}
	if l.inner != nil {
		return l.inner.Scan(ctx, workspace, req)
	}
	return nil, nil
}

// WrapOpenCodeReviewRunner 把 *runner.OpenCodeReviewRunner 包成 ocr.legacyRunner。
func WrapOpenCodeReviewRunner(r *runnerpkg.OpenCodeReviewRunner) *LegacyRunner {
	return &LegacyRunner{inner: r}
}

// WrapDynamicOpenCodeReviewRunner 把 *runner.DynamicOpenCodeReviewRunner 包成 ocr.legacyRunner。
func WrapDynamicOpenCodeReviewRunner(r *runnerpkg.DynamicOpenCodeReviewRunner) *LegacyRunner {
	return &LegacyRunner{dyn: r}
}