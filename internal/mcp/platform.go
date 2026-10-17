package mcp

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/yi-nology/code-review-service/internal/app"
	"github.com/yi-nology/code-review-service/internal/service/review/domain"

	"github.com/mark3labs/mcp-go/mcp"
	provider_sdk "github.com/yi-nology/go-git-platform/provider"
)

// Tier 2 平台辅助 tool：直接调 GitProviderManager（go-git-platform），不走 OCR。
// 用途：minicode / Claude Desktop 在 IDE 里帮用户选 repo、PR、branch 时用。
//
// 跟 Tier 1 评审的区别：
//   - 响应快（秒级），无副作用（只读）。
//   - 不落库,纯代理 git 平台 API。
//   - 错误主要来自网络/平台限流 → 上游错误,可重试。

// 平台参数规整：跟 create_review 一样 lowercase + trim。
// 抽出来 4 个 tool 共用。

// list_provider_repos：列某 platform 下的 repos（onboarding 阶段用）。
func RegisterListProviderRepos(s *Server, git app.PlatformReader) {
	s.AddTool(listProviderReposTool(), listProviderReposHandler(git))
}

func listProviderReposHandler(git app.PlatformReader) func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		provider, err := requireString(args, "provider")
		if err != nil {
			return NewToolErrorf(ErrCategoryInvalidParam, "missing_provider", "传 provider", "%v", err), nil
		}
		if !git.HasProvider(normalizeProvider(provider)) {
			return NewToolErrorf(ErrCategoryUnsupported, "provider_not_configured",
				"先在 review-service web console 配置 platform (Settings → Git Platforms)",
				"platform %q 未配置", provider), nil
		}

		page, perPage := pageArgs(args, 30, 100)

		repos, err := git.ListRepos(ctx, normalizeProvider(provider), provider_sdk.ListRepoOptions{
			Page:    page,
			PerPage: perPage,
		})
		if err != nil {
			return classifyPlatformError(err), nil
		}
		return JSONResult(map[string]any{
			"provider": provider,
			"page":     page,
			"per_page": perPage,
			"repos":    repos,
		})
	}
}

// list_pull_requests：列某 repo 的 PR/MR。
func RegisterListPullRequests(s *Server, git app.GitProviderManager) {
	s.AddTool(listPullRequestsTool(), listPullRequestsHandler(git))
}

func listPullRequestsHandler(git app.GitProviderManager) func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		provider, err := requireString(args, "provider")
		if err != nil {
			return NewToolErrorf(ErrCategoryInvalidParam, "missing_provider", "传 provider", "%v", err), nil
		}
		owner, err := requireString(args, "owner")
		if err != nil {
			return NewToolErrorf(ErrCategoryInvalidParam, "missing_owner", "传 owner", "%v", err), nil
		}
		repo, err := requireString(args, "repo")
		if err != nil {
			return NewToolErrorf(ErrCategoryInvalidParam, "missing_repo", "传 repo", "%v", err), nil
		}

		page, perPage := pageArgs(args, 20, 100)
		state := strings.ToLower(strings.TrimSpace(optionalString(args, "state")))
		if state == "" {
			state = "open"
		}

		crs, total, err := git.ListCRs(ctx, normalizeProvider(provider), provider_sdk.ListCROptions{
			Owner:   owner,
			Repo:    repo,
			State:   provider_sdk.CRState(state),
			Page:    page,
			PerPage: perPage,
		})
		if err != nil {
			return classifyPlatformError(err), nil
		}
		return JSONResult(map[string]any{
			"provider": provider,
			"owner":    owner,
			"repo":     repo,
			"state":    state,
			"page":     page,
			"per_page": perPage,
			"total":    total,
			"prs":      crs,
		})
	}
}

// get_branch_sha：查某分支的 HEAD commit SHA。
//
// 用途：用户在 IDE 里说「帮我评审下 main 最新的代码」,LLM 调这个拿 head_sha,
// 然后拼 create_review 入参。
func RegisterGetBranchSHA(s *Server, git app.GitProviderManager) {
	s.AddTool(getBranchSHATool(), getBranchSHAHandler(git))
}

func getBranchSHAHandler(git app.GitProviderManager) func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		provider, err := requireString(args, "provider")
		if err != nil {
			return NewToolErrorf(ErrCategoryInvalidParam, "missing_provider", "传 provider", "%v", err), nil
		}
		owner, err := requireString(args, "owner")
		if err != nil {
			return NewToolErrorf(ErrCategoryInvalidParam, "missing_owner", "传 owner", "%v", err), nil
		}
		repo, err := requireString(args, "repo")
		if err != nil {
			return NewToolErrorf(ErrCategoryInvalidParam, "missing_repo", "传 repo", "%v", err), nil
		}
		branch, err := requireString(args, "branch")
		if err != nil {
			return NewToolErrorf(ErrCategoryInvalidParam, "missing_branch", "传 branch", "%v", err), nil
		}

		sha, err := git.GetBranchSHA(ctx, normalizeProvider(provider), owner, repo, branch)
		if err != nil {
			return classifyPlatformError(err), nil
		}
		return JSONResult(map[string]any{
			"provider": provider,
			"owner":    owner,
			"repo":     repo,
			"branch":   branch,
			"sha":      sha,
		})
	}
}

// get_pull_request：查单个 PR/MR 详情。
func RegisterGetPullRequest(s *Server, git app.GitProviderManager) {
	s.AddTool(getPullRequestTool(), getPullRequestHandler(git))
}

func getPullRequestHandler(git app.GitProviderManager) func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		provider, err := requireString(args, "provider")
		if err != nil {
			return NewToolErrorf(ErrCategoryInvalidParam, "missing_provider", "传 provider", "%v", err), nil
		}
		owner, err := requireString(args, "owner")
		if err != nil {
			return NewToolErrorf(ErrCategoryInvalidParam, "missing_owner", "传 owner", "%v", err), nil
		}
		repo, err := requireString(args, "repo")
		if err != nil {
			return NewToolErrorf(ErrCategoryInvalidParam, "missing_repo", "传 repo", "%v", err), nil
		}
		numF, ok := args["number"].(float64)
		if !ok || numF <= 0 {
			return NewToolErrorf(ErrCategoryInvalidParam, "missing_number", "传 number (PR/MR 数字编号)", "missing or invalid number"), nil
		}
		number := strconv.FormatInt(int64(numF), 10)

		cr, err := git.GetCR(ctx, normalizeProvider(provider), owner, repo, number)
		if err != nil {
			return classifyPlatformError(err), nil
		}
		return JSONResult(cr)
	}
}

// classifyPlatformError 把 git 平台 API 错误翻译到 MCP 错误。
//
// 跟 classifyCreateError 区别：
//   - 平台错误主要来自网络 / 限流 / 平台不支持,大多数归 upstream_error
//     （retryable=true），LLM 可重试。
//   - 未配置的 platform 跟 ErrUnsupportedProvider 一致，归 unsupported。
func classifyPlatformError(err error) *mcp.CallToolResult {
	if err == nil {
		return nil
	}
	if errors.Is(err, domain.ErrUnsupportedProvider) || strings.Contains(err.Error(), "unsupported provider") {
		return NewToolError(ErrCategoryUnsupported, "unsupported_provider",
			err.Error(),
			"先在 web console 配置 platform (Settings → Git Platforms)", false)
	}
	// 其他归上游错误（网络/限流），可重试。
	return NewToolErrorf(ErrCategoryUpstream, "platform_error",
		"重试;如持续失败检查平台 token 是否过期",
		"%v", err)
}
