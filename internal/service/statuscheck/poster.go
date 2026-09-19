// Package statuscheck 实现 CI Status Check：评审结束后向 Git 平台写 commit status，
// 让 review-service 成为 PR 合并的事实质量门禁。
//
// 复用 GitProviderService.CreateCommitStatus（SDK 已封装各平台），不需要裸 HTTP。
// gitcode/gitee 平台未实现该接口（ErrNotImplemented），静默降级跳过。
package statuscheck

import (
	"context"
	"errors"
	"fmt"

	"github.com/yi-nology/code-review-service/internal/service/review/domain"
	"github.com/yi-nology/code-review-service/internal/strutil"
	"github.com/yi-nology/code-review-service/internal/vcs"
	"github.com/oekit/logx"

	sdk "github.com/yi-nology/git-platform-sdk/provider"
)

// Poster 向 Git 平台写 commit status。
//
// 依赖按消费方收窄为 StatusWriter 窄接口（*provider.GitProviderService 结构化
// 满足），statuscheck 不再依赖 internal/service/provider 具体类型。
type Poster struct {
	gitSvc StatusWriter
}

// StatusWriter 只声明写 commit status 所需的方法。
type StatusWriter interface {
	CreateCommitStatus(ctx context.Context, platform, owner, repo, sha string, opts sdk.CommitStatusOptions) error
}

func NewPoster(gitSvc StatusWriter) *Poster {
	return &Poster{gitSvc: gitSvc}
}

// 状态常量（git 平台约定：success / failed / pending / error）。
// 真源在 review/domain（ApplyWeakGate 等门禁决策逻辑所在）；
// 此处保留包内别名，既有调用点（eino/argus 路径）不改。
const (
	StateSuccess = domain.StateSuccess
	StateFailed  = domain.StateFailed
	StatePending = domain.StatePending
	StateError   = domain.StateError
)

// ContextName 是 PR 上显示的 check 名称，固定值便于在分支保护规则里勾选为必需 check。
const ContextName = "review-service/gate"

// Post 写一条 commit status。SHA 必须是真实 head（用 workspace.OriginalHeadSHA）。
// 失败仅记日志，不阻断主流程（status 是辅助信号，不应让评审因写 status 失败而失败）。
func (p *Poster) Post(ctx context.Context, platform, repoPath, sha, state, description, reviewID string) {
	if p == nil || p.gitSvc == nil || sha == "" || repoPath == "" {
		return
	}
	owner, repoName, err := vcs.ParseRepoPath(repoPath)
	if err != nil {
		logx.CtxWarnf(ctx, "[status-check] parse repo path %q failed: %v", repoPath, err)
		return
	}
	opts := sdk.CommitStatusOptions{
		State:       state,
		Context:     ContextName,
		Description: strutil.Truncate(description, 140), // GitHub description 上限 140 字符
	}
	// TargetURL 指向 review 详情页（前端路由 /reviews/:id），便于从 PR check 跳转。
	// 完整 URL 需要服务外部地址，这里先放相对路径；若配置了 ExternalURL 可拼接。
	if reviewID != "" {
		opts.TargetURL = fmt.Sprintf("/reviews/%s", reviewID)
	}

	if err := p.gitSvc.CreateCommitStatus(ctx, platform, owner, repoName, sha, opts); err != nil {
		// gitcode/gitee 未实现，静默降级。
		if errors.Is(err, sdk.ErrNotImplemented) {
			logx.CtxInfof(ctx, "[status-check] platform %s does not support commit status, skipping", platform)
			return
		}
		logx.CtxWarnf(ctx, "[status-check] post %s status to %s/%s@%s failed: %v",
			state, owner, repoName, sha[:min(7, len(sha))], err)
	}
}
