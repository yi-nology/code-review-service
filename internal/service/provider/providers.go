package provider

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/yi-nology/code-review-service/internal/service/review/domain"
	"github.com/oekit/logx"
)

// DefaultMaxDiffBytes 是单次评审 diff 的默认字节上限（5 MiB）。
// 装配层未从 DB 读到有效值时用此兜底。
// 注意：repository/postgres/models.OCRSettings 的 gorm default tag（5242880）
// 是同一值的字面量镜像（model 包不能反向依赖 service 层），改动时需同步。
const DefaultMaxDiffBytes int64 = 5 * 1024 * 1024

type GitProviderProviders struct {
	svc          *GitProviderService
	maxDiffBytes int64
}

func NewGitProviderProviders(svc *GitProviderService, maxDiffBytes int64) *GitProviderProviders {
	return &GitProviderProviders{svc: svc, maxDiffBytes: maxDiffBytes}
}

func (p *GitProviderProviders) Get(name string) (domain.Provider, bool) {
	if !p.svc.HasProvider(name) {
		return nil, false
	}
	return &gitProviderAdapter{svc: p.svc, platform: name, maxDiffBytes: p.maxDiffBytes}, true
}

type gitProviderAdapter struct {
	svc          *GitProviderService
	platform     string
	maxDiffBytes int64
}

func (a *gitProviderAdapter) Prepare(ctx context.Context, req domain.Request) (domain.Workspace, error) {
	owner, repoName, err := ParseRepoPath(req.Repo)
	if err != nil {
		return domain.Workspace{}, fmt.Errorf("invalid repo path %q: %w", req.Repo, err)
	}

	var diff string
	var prNumber int
	var targetBranch, sourceBranch, headSHAFromAPI, baseSHAFromAPI string
	hasPR := req.PullRequestID != "" || req.MergeRequestIID != ""
	number := req.PullRequestID
	if number == "" {
		number = req.MergeRequestIID
	}

	if hasPR {
		var err error
		prNumber, err = strconv.Atoi(number)
		if err != nil || prNumber <= 0 {
			return domain.Workspace{}, fmt.Errorf("invalid PR/MR number %q", number)
		}

		diff, err = a.svc.GetCRDiffString(ctx, a.platform, owner, repoName, number)
		if err != nil {
			// Gerrit 等无 SDK 后端的平台，API diff 获取会失败。
			// 此时 diff 留空，由 cloneAndResolvePR 克隆后由 OCR 引擎从 git 计算。
			if a.platform == "gerrit" {
				logx.CtxWarnf(ctx, "gerrit: API diff fetch skipped (no SDK backend), will compute from git: %v", err)
			} else {
				return domain.Workspace{}, fmt.Errorf("get diff via API: %w", err)
			}
		}

		cr, crErr := a.svc.GetCR(ctx, a.platform, owner, repoName, number)
		if crErr == nil && cr != nil {
			targetBranch = cr.TargetBranch
			sourceBranch = cr.SourceBranch
			headSHAFromAPI = cr.HeadSHA
			baseSHAFromAPI = cr.BaseSHA
		}
	}

	if a.maxDiffBytes > 0 && int64(len(diff)) > a.maxDiffBytes {
		return domain.Workspace{}, fmt.Errorf("diff size %d bytes exceeds limit of %d bytes", len(diff), a.maxDiffBytes)
	}

	cloneURL := a.svc.BuildCloneURL(a.platform, owner, repoName)
	// 出站防护：owner/repo 来自 webhook 载荷（外部可控），
	// 未校验就出站可能把 git token 送到攻击者的 host 上。详见 vcs.CloneURLGuard。
	if err := a.svc.ValidateCloneURL(a.platform, owner, repoName, cloneURL); err != nil {
		return domain.Workspace{}, fmt.Errorf("clone 目标校验失败: %w", err)
	}
	token := a.svc.GetToken(a.platform)
	skipTLS := a.svc.GetSkipTLS(a.platform)

	var dir, baseSHA, headSHA string
	if req.CommitSHA != "" {
		// 单 commit 评审模式（OCR v1.9.5+ --commit）：只需克隆并检出 commit，diff 由 OCR binary 内部处理。
		dir, err = cloneAndResolveCommit(ctx, cloneURL, token, skipTLS, req.CommitSHA)
	} else if hasPR {
		dir, baseSHA, headSHA, err = cloneAndResolvePR(
			ctx, cloneURL, token, skipTLS, prNumber, a.platform, sourceBranch, targetBranch, headSHAFromAPI, baseSHAFromAPI,
		)
	} else {
		var shaDiff string
		dir, baseSHA, headSHA, shaDiff, err = cloneAndResolveSHA(ctx, cloneURL, token, skipTLS, req.BaseSHA, req.HeadSHA, a.maxDiffBytes)
		if err == nil && shaDiff != "" {
			diff = shaDiff
		}
	}
	if err != nil {
		return domain.Workspace{}, fmt.Errorf("clone repository: %w", err)
	}

	// OriginalHeadSHA 必须是 PR/MR 的真实 head SHA（来自平台 API），而非 req.HeadSHA。
	// 当仅凭 pull_request_id 创建 review 时，req.HeadSHA 为空；即使 cloneAndResolvePR 为了
	// 给 reviewer 提供完整文件上下文而构建了合成提交（restoreDeletedFiles / ApplyProjectScope），
	// 导致 workspace.HeadSHA 偏离真实 head，回写阶段的 stale 检查仍需要用真实 head 与平台
	// 当前 head 比对。否则 publishWriteback (executor/service.go:410) 的 OriginalHeadSHA 覆盖
	// 会被跳过，writeback 误判为 stale 而丢弃评论。
	originalHeadSHA := req.HeadSHA
	if headSHAFromAPI != "" {
		originalHeadSHA = headSHAFromAPI
	}

	return domain.Workspace{
		Provider:        a.platform,
		Repo:            strings.TrimPrefix(req.Repo, "/"),
		BaseSHA:         baseSHA,
		HeadSHA:         headSHA,
		OriginalHeadSHA: originalHeadSHA,
		CommitSHA:       req.CommitSHA,
		Diff:            diff,
		Dir:             dir,
	}, nil
}
