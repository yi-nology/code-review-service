package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/yi-nology/go-git-platform/provider"
)

// commitstatus.go：commit status 的**平台词表**与**状态机**适配。
//
// # 一、为什么不能把内部状态词直接丢给 SDK
//
// 我们的内部词表是 success / failed / pending / error，而各平台的 commit status
// 词表并不相同，SDK 各后端的映射口径也不一致：
//
//	平台      平台词表                                SDK 映射口径
//	github    success/failure/pending/error          原样透传
//	gitlab    pending/running/success/failed/...    认 success/failed/pending/running，
//	                                                 其余（含 error）**兜底成 pending**
//	gitee     check run: success/failure/neutral     认 success/failure/error/pending，
//	                                                 其余（含 failed）**兜底成 neutral**
//	gitcode   success/failure/pending/error          原样透传
//
// 那个「兜底」不是安全降级，是**静默改写语义**：
//   - GitLab 上评审失败（error）被写成 pending → 门禁显示黄色「进行中」，
//     终态永远不落，合并门禁既不绿也不红。
//   - Gitee 上评审未过（failed）被写成 neutral → 灰色「无结论」，同样丢信号。
//   - GitHub 原样透传，"failed" 不是 GitHub 的合法词（它要 failure）→ 422。
//
// 实测（GitLab 实例，2026-10-21）：`state=error` 被 GitLab 拒为
// `state does not have a valid value`；而我们发出去的其实是 SDK 兜底后的 pending，
// 于是「评审开始 pending」→「评审失败 pending」= 重复写 pending，GitLab 报
// `Cannot transition status via :enqueue from :pending`（400）——**终态写不进去**。
//
// 所以这里在**平台适配层**先把词表对齐，交给 SDK 的是它认识的状态词，
// 兜底分支不会再被触发。
//
// # 二、重复写同一个非终态是 GitLab 的非法转移
//
// 实测 GitLab 的 commit status 状态机（同名 context、同一 sha）：
//
//	pending → pending   ✗ 400 Cannot transition status via :enqueue from :pending
//	running → running   ✗ 400 Cannot transition status via :run from :running
//	pending → running   ✓
//	pending → success/failed ✓
//	success/failed → 任意 ✓（终态可重复写、可回退到非终态）
//
// 即：**只有「重复写同一个非终态」会被拒**。写非终态前先读一次当前值，
// 已是同一状态就直接跳过——既是幂等，也免得把「本来就已经是进行中」报成失败。
//
// 读失败不阻断：照常写，让写路径自己报错（status 是辅助信号，不该让评审失败）。

// platformCommitState 把内部状态词翻译成该平台 SDK 认识的状态词。
//
// 未识别的平台按原样透传：宁可让平台自己拒（错误信息直指平台口径），
// 也不猜一个可能不存在的词。
func platformCommitState(platform, state string) string {
	normalized := strings.ToLower(strings.TrimSpace(state))
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case "gitlab":
		// GitLab 词表：pending / running / success / failed / canceled（无 error）。
		switch normalized {
		case "error":
			return "failed"
		}
	case "gitee":
		// Gitee check run 结论：success / failure / neutral（SDK 认 failure）。
		switch normalized {
		case "error", "failed":
			return "failure"
		}
	case "github", "gitcode":
		// GitHub 形状的词表：success / failure / pending / error。
		switch normalized {
		case "failed":
			return "failure"
		}
	default:
		return state
	}
	return state
}

// CreateCommitStatus 写一条 commit status。
//
// 平台适配（词表对齐 + 重复写幂等）见 commitstatus.go 的说明。
func (s *GitProviderService) CreateCommitStatus(ctx context.Context, platform, owner, repo, sha string, opts provider.CommitStatusOptions) error {
	p, err := s.GetProvider(platform)
	if err != nil {
		return err
	}
	// SDK v0.48 起 CommitStatusManager 是可选能力接口（如 Gitee 无 commit-status
	// 端点），不再挂在 Provider 上，需断言。
	csm, ok := p.(provider.CommitStatusManager)
	if !ok {
		return fmt.Errorf("platform %s does not support commit statuses", platform)
	}

	// 词表对齐：内部词 → 平台词（否则 SDK 的兜底分支会静默改写语义）。
	opts.State = platformCommitState(platform, opts.State)

	// 非终态重复写是 GitLab 的非法转移 → 先读当前值，已是同态就跳过。
	if !provider.CommitStatusState(provider.NormalizeCommitStatusState(opts.State)).Terminal() {
		if commitStatusAlreadySame(ctx, csm, owner, repo, sha, opts) {
			return nil
		}
	}

	return csm.CreateCommitStatus(ctx, owner, repo, sha, opts)
}

// commitStatusAlreadySame 判断该 (sha, context) 上当前状态是否已经是目标状态。
//
// 只看**同名 context** 的状态：不同 context 是相互独立的 check，
// GitLab 的转移校验也按 context 走。
func commitStatusAlreadySame(ctx context.Context, csm provider.CommitStatusManager, owner, repo, sha string, opts provider.CommitStatusOptions) bool {
	statuses, err := csm.ListCommitStatuses(ctx, owner, repo, sha)
	if err != nil {
		// 读失败不阻断写入：写路径自己会给出真实错误。
		return false
	}
	target := provider.NormalizeCommitStatusState(opts.State)
	for _, st := range statuses {
		if st.Context != opts.Context {
			continue
		}
		return provider.NormalizeCommitStatusState(string(st.State)) == target
	}
	return false
}
