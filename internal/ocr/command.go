package ocr

import (
	"context"

	ocrinternal "github.com/yi-nology/code-review-service/internal/ocr/internal"
)

// Public aliases to internal/ocr/internal（让 ocr.Run / ocr.CommandSpec 等可被外部使用）。
// 阶段 A+3 完整化：把这些函数迁移到 ocr 包顶层，去掉 internal/ 子包。

// CommandSpec 描述一次外部命令调用。
type CommandSpec = ocrinternal.CommandSpec

// CommandResult 包含命令的完整结果。
type CommandResult = ocrinternal.CommandResult

// InternalRun 是统一的 exec 入口。
func InternalRun(ctx context.Context, spec CommandSpec) (*CommandResult, error) {
	return ocrinternal.Run(ctx, spec)
}

// Run 是简化的 exec 调用（直接传 binary + args + timeout）。
func Run(ctx context.Context, binary string, args ...string) (stdout []byte, stderr []byte, err error) {
	res, err := ocrinternal.Run(ctx, CommandSpec{
		Binary:  binary,
		Args:    args,
		Timeout: 0,
	})
	if res == nil {
		return nil, nil, err
	}
	return res.Stdout, res.Stderr, err
}
